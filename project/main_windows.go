package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	user               = systemDLL("user32.dll")
	gdi                = systemDLL("gdi32.dll")
	initCommonControls = systemDLL("comctl32.dll").NewProc("InitCommonControlsEx")
	getModuleHandle    = kernel.NewProc("GetModuleHandleW")
	registerClass      = user.NewProc("RegisterClassW")
	createWindow       = user.NewProc("CreateWindowExW")
	defaultWindowProc  = user.NewProc("DefWindowProcW")
	loadCursor         = user.NewProc("LoadCursorW")
	showWindow         = user.NewProc("ShowWindow")
	getMessage         = user.NewProc("GetMessageW")
	translateMessage   = user.NewProc("TranslateMessage")
	dispatchMessage    = user.NewProc("DispatchMessageW")
	isDialogMessage    = user.NewProc("IsDialogMessageW")
	postQuit           = user.NewProc("PostQuitMessage")
	destroyWindow      = user.NewProc("DestroyWindow")
	enableWindow       = user.NewProc("EnableWindow")
	setWindowText      = user.NewProc("SetWindowTextW")
	getWindowText      = user.NewProc("GetWindowTextW")
	getTextLength      = user.NewProc("GetWindowTextLengthW")
	sendMessage        = user.NewProc("SendMessageW")
	setTimer           = user.NewProc("SetTimer")
	setFocus           = user.NewProc("SetFocus")
	messageBox         = user.NewProc("MessageBoxW")
	adjustWindowRect   = user.NewProc("AdjustWindowRectEx")
	stockObject        = gdi.NewProc("GetStockObject")
	createFont         = gdi.NewProc("CreateFontW")
	deleteObject       = gdi.NewProc("DeleteObject")
)

const (
	wmDestroy      = 2
	wmClose        = 0x10
	wmSetFont      = 0x30
	wmCommand      = 0x111
	wmTimer        = 0x113
	wsChildVisible = 0x50000000
	wsTabStop      = 0x10000
	windowStyle    = 0x00CA0000
	windowExStyle  = 0x10000
	centerDefault  = 0x80000000
	buttonStart    = 101
	buttonStop     = 102
)

type windowClass struct {
	style                              uint32
	callback                           uintptr
	classExtra, windowExtra            int32
	instance, icon, cursor, background uintptr
	menu, name                         *uint16
}

type point struct{ x, y int32 }
type rectangle struct{ left, top, right, bottom int32 }
type winMessage struct {
	window         uintptr
	message        uint32
	wParam, lParam uintptr
	time           uint32
	pos            point
	private        uint32
}

type usageMeter struct {
	bar, value uintptr
}

type appWindow struct {
	window, instance                         uintptr
	minutes, cap, clock, status, start, stop uintptr
	bodyFont, titleFont, clockFont           uintptr
	events                                   chan update
	usageEvents                              chan usageSnapshot
	meters                                   [3]usageMeter
	input                                    io.WriteCloser
	deadline                                 time.Time
	running, restoring, stopping, closing    bool
	outcome                                  string
	failed                                   bool
}

func errorBox(window uintptr, message string) {
	messageBox.Call(window, uintptr(unsafe.Pointer(textPointer(message))),
		uintptr(unsafe.Pointer(textPointer("Quiet Session"))), 0x10)
}

func labelText(window uintptr, s string) {
	setWindowText.Call(window, uintptr(unsafe.Pointer(textPointer(s))))
}

func controlText(window uintptr) string {
	n, _, _ := getTextLength.Call(window)
	buffer := make([]uint16, n+1)
	getWindowText.Call(window, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return syscall.UTF16ToString(buffer)
}

func font(height uintptr, weight uintptr, face string) uintptr {
	// CreateFont expects a negative logical height for the character itself.
	size := -int(height)
	result, _, _ := createFont.Call(uintptr(size), 0, 0, 0, weight,
		0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(textPointer(face))))
	return result
}

func (a *appWindow) control(class, title string, style, extended, x, y, width, height, id, typeface uintptr) uintptr {
	h, _, _ := createWindow.Call(extended, uintptr(unsafe.Pointer(textPointer(class))),
		uintptr(unsafe.Pointer(textPointer(title))), wsChildVisible|style, x, y, width, height,
		a.window, id, a.instance, 0)
	if h == 0 {
		panic("Windows 창의 구성 요소를 시작할 수 없습니다")
	}
	sendMessage.Call(h, wmSetFont, typeface, 1)
	return h
}

func (a *appWindow) label(title string, x, y, width, height, typeface uintptr) uintptr {
	return a.control("STATIC", title, 0, 0, x, y, width, height, 0, typeface)
}

func (a *appWindow) createControls() {
	a.bodyFont = font(15, 400, "Malgun Gothic")
	a.titleFont = font(26, 700, "Malgun Gothic")
	a.clockFont = font(38, 700, "Segoe UI")
	if a.bodyFont == 0 {
		a.bodyFont, _, _ = stockObject.Call(17)
	}
	if a.titleFont == 0 {
		a.titleFont = a.bodyFont
	}
	if a.clockFont == 0 {
		a.clockFont = a.bodyFont
	}
	a.label("조용한 모드", 26, 24, 390, 42, a.titleFont)
	a.label("선택한 시간 동안 CPU 성능을 제한해 발열을 줄입니다.", 28, 72, 390, 28, a.bodyFont)
	a.label("사용 시간 (분)", 28, 116, 174, 25, a.bodyFont)
	a.label("최대 프로세서 상태", 224, 116, 190, 25, a.bodyFont)
	a.minutes = a.control("EDIT", "50", wsTabStop|0x2000|0x80, 0x200, 28, 145, 174, 30, 0, a.bodyFont)
	sendMessage.Call(a.minutes, 0xC5, 3, 0)
	a.cap = a.control("COMBOBOX", "", wsTabStop|3|0x200000, 0, 224, 145, 190, 128, 0, a.bodyFont)
	for _, item := range []string{"60%", "75%", "85%"} {
		sendMessage.Call(a.cap, 0x143, 0, uintptr(unsafe.Pointer(textPointer(item))))
	}
	sendMessage.Call(a.cap, 0x14E, 1, 0)
	a.clock = a.label("–:–", 26, 193, 390, 55, a.clockFont)
	a.status = a.label("대기 중 · 팬 속도는 노트북이 자동 조절합니다.", 28, 253, 386, 65, a.bodyFont)
	a.createMeters()
	a.start = a.control("BUTTON", "시작", wsTabStop|1, 0, 28, 421, 234, 42, buttonStart, a.bodyFont)
	a.stop = a.control("BUTTON", "지금 종료", wsTabStop, 0, 270, 421, 144, 42, buttonStop, a.bodyFont)
	enableWindow.Call(a.stop, 0)
}

func (a *appWindow) createMeters() {
	for i, name := range []string{"CPU", "RAM", "GPU"} {
		y := uintptr(324 + i*25)
		a.label(name, 28, y, 50, 20, a.bodyFont)
		bar := a.control("msctls_progress32", "", 1, 0, 82, y+5, 272, 10, 0, a.bodyFont)
		label := a.control("STATIC", "—", 2, 0, 358, y, 56, 21, 0, a.bodyFont)
		a.meters[i] = usageMeter{bar, label}
	}
}

func (a *appWindow) refreshMeters() {
	select {
	case snapshot := <-a.usageEvents:
		for i, meter := range a.meters {
			value := snapshot[i]
			text := "—"
			if value.valid {
				text = strconv.Itoa(value.percentage) + "%"
			} else if i == 2 {
				text = "측정 불가"
			}
			sendMessage.Call(meter.bar, 0x402, uintptr(value.percentage), 0)
			labelText(meter.value, text)
		}
	default:
	}
}

func (a *appWindow) begin() {
	if a.running {
		return
	}
	minutes, err := strconv.Atoi(strings.TrimSpace(controlText(a.minutes)))
	if err != nil || minutes < 1 || minutes > 180 {
		labelText(a.status, "사용 시간에 1~180 사이의 정수를 입력하세요.")
		setFocus.Call(a.minutes)
		return
	}
	index, _, _ := sendMessage.Call(a.cap, 0x147, 0, 0)
	caps := []string{"60", "75", "85"}
	if index >= uintptr(len(caps)) {
		labelText(a.status, "최대 프로세서 상태를 선택하세요.")
		return
	}
	a.launch(strconv.Itoa(minutes), caps[index])
}

func (a *appWindow) launch(arguments ...string) {
	executable, err := os.Executable()
	if err != nil {
		labelText(a.status, err.Error())
		return
	}
	command := exec.Command(executable, append([]string{"--worker"}, arguments...)...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	input, err := command.StdinPipe()
	if err != nil {
		labelText(a.status, err.Error())
		return
	}
	output, err := command.StdoutPipe()
	if err != nil {
		input.Close()
		labelText(a.status, err.Error())
		return
	}
	if err := command.Start(); err != nil {
		output.Close()
		input.Close()
		labelText(a.status, "전원 제어를 시작할 수 없습니다. "+err.Error())
		return
	}
	a.running, a.restoring, a.stopping = true, len(arguments) == 0, false
	a.outcome, a.failed = "", false
	a.input = input
	for _, control := range []uintptr{a.start, a.minutes, a.cap} {
		enableWindow.Call(control, 0)
	}
	if !a.restoring {
		enableWindow.Call(a.stop, 1)
	}
	labelText(a.status, "전원 설정을 확인하는 중입니다.")
	go func() {
		decoder := json.NewDecoder(output)
		for {
			var u update
			if err := decoder.Decode(&u); err != nil {
				output.Close()
				break
			}
			a.events <- u
		}
		_ = command.Wait()
		a.events <- update{Event: "exit"}
	}()
}

func (a *appWindow) end() {
	if a.running && !a.restoring && !a.stopping {
		a.stopping = true
		a.input.Close()
		enableWindow.Call(a.stop, 0)
		labelText(a.status, "전원 설정을 복구하는 중입니다.")
	}
}

func (a *appWindow) refresh() {
	for {
		select {
		case event := <-a.events:
			switch event.Event {
			case "started":
				a.deadline = event.EndsAt
				if !a.stopping {
					labelText(a.status, "조용한 모드 사용 중")
				}
			case "done", "error":
				a.outcome, a.failed = event.Message, event.Event == "error"
			case "exit":
				a.input.Close()
				a.running = false
				a.deadline = time.Time{}
				labelText(a.clock, "–:–")
				if a.outcome == "" {
					a.outcome = "전원 제어가 중단됐습니다. 다시 실행해 복구를 시도하세요."
					a.failed = true
				}
				labelText(a.status, a.outcome)
				for _, control := range []uintptr{a.start, a.minutes, a.cap} {
					enableWindow.Call(control, 1)
				}
				enableWindow.Call(a.stop, 0)
				if a.closing {
					if a.failed {
						errorBox(a.window, a.outcome)
					}
					destroyWindow.Call(a.window)
					return
				}
			}
		default:
			if !a.deadline.IsZero() {
				remaining := max(0, time.Until(a.deadline))
				seconds := int((remaining + time.Second - 1) / time.Second)
				labelText(a.clock, fmt.Sprintf("%02d:%02d", seconds/60, seconds%60))
			}
			return
		}
	}
}

func (a *appWindow) handle(window, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmCommand:
		switch wParam & 0xffff {
		case buttonStart:
			a.begin()
			return 0
		case buttonStop:
			a.end()
			return 0
		}
	case wmTimer:
		a.refreshMeters()
		a.refresh()
		return 0
	case wmClose:
		if a.running {
			a.closing = true
			a.end()
		} else {
			destroyWindow.Call(window)
		}
		return 0
	case wmDestroy:
		postQuit.Call(0)
		return 0
	}
	result, _, _ := defaultWindowProc.Call(window, message, wParam, lParam)
	return result
}

func showApp() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	a := &appWindow{events: make(chan update, 4), usageEvents: make(chan usageSnapshot, 1)}
	controls := [2]uint32{8, 0x20}
	if ok, _, err := initCommonControls.Call(uintptr(unsafe.Pointer(&controls))); ok == 0 {
		return err
	}
	a.instance, _, _ = getModuleHandle.Call(0)
	name := textPointer("QuietSessionWindow")
	cursor, _, _ := loadCursor.Call(0, 32512)
	class := windowClass{
		callback: syscall.NewCallback(a.handle), instance: a.instance,
		cursor: cursor, background: 6, name: name,
	}
	if atom, _, err := registerClass.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		return err
	}
	rect := rectangle{right: 442, bottom: 487}
	adjustWindowRect.Call(uintptr(unsafe.Pointer(&rect)), windowStyle, 0, windowExStyle)
	window, _, err := createWindow.Call(windowExStyle, uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(textPointer("Quiet Session"))), windowStyle,
		centerDefault, centerDefault, uintptr(rect.right-rect.left), uintptr(rect.bottom-rect.top),
		0, 0, a.instance, 0)
	if window == 0 {
		return err
	}
	a.window = window
	a.createControls()
	if timer, _, err := setTimer.Call(window, 1, 200, 0); timer == 0 {
		destroyWindow.Call(window)
		return err
	}
	stopUsage := make(chan struct{})
	defer close(stopUsage)
	go monitorUsage(stopUsage, a.usageEvents)
	showWindow.Call(window, 5)
	setFocus.Call(a.minutes)
	if _, err := os.Stat(journalPath()); err == nil {
		a.launch()
	}
	var message winMessage
	for {
		result, _, err := getMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		switch int32(result) {
		case 0:
			for _, handle := range []uintptr{a.bodyFont, a.titleFont, a.clockFont} {
				deleteObject.Call(handle)
			}
			return nil
		case -1:
			return err
		}
		if handled, _, _ := isDialogMessage.Call(window, uintptr(unsafe.Pointer(&message))); handled == 0 {
			translateMessage.Call(uintptr(unsafe.Pointer(&message)))
			dispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
		}
	}
}

func runWorker(arguments []string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if os.Getenv("LOCALAPPDATA") == "" {
		return errors.New("이 Windows 계정의 앱 데이터 폴더를 찾을 수 없습니다")
	}
	unlock, err := lockWorker()
	if err != nil {
		return err
	}
	defer unlock()
	p, path := windowsPower{}, journalPath()
	if err := recoverPlan(p, path); err != nil {
		return fmt.Errorf("기존 전원 계획을 복구하지 못했습니다.\n%w", err)
	}
	encoder := json.NewEncoder(os.Stdout)
	if len(arguments) == 0 {
		return encoder.Encode(update{Event: "done", Message: "기존 전원 계획 복구를 완료했습니다."})
	}
	if len(arguments) != 2 {
		return errors.New("사용 시간과 최대 프로세서 상태가 필요합니다")
	}
	minutes, err := strconv.Atoi(arguments[0])
	if err != nil || minutes < 1 || minutes > 180 {
		return errors.New("사용 시간은 1~180분 사이여야 합니다")
	}
	cap, err := strconv.Atoi(arguments[1])
	if err != nil || (cap != 60 && cap != 75 && cap != 85) {
		return errors.New("최대 프로세서 상태는 60%, 75%, 85% 중 하나여야 합니다")
	}
	stop := monitorWindow(os.Stdin)
	message, err := session(p, path, time.Duration(minutes)*time.Minute, uint32(cap), stop, func(u update) {
		_ = encoder.Encode(u)
	})
	if err != nil {
		return err
	}
	return encoder.Encode(update{Event: "done", Message: message})
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "--worker" {
		if err := runWorker(os.Args[2:]); err != nil {
			_ = json.NewEncoder(os.Stdout).Encode(update{Event: "error", Message: err.Error()})
			os.Exit(1)
		}
		return
	}
	if err := showApp(); err != nil {
		errorBox(0, "Quiet Session을 시작할 수 없습니다.\n"+err.Error())
	}
}
