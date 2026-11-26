package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel       = syscall.NewLazyDLL("kernel32.dll")
	localFree    = kernel.NewProc("LocalFree")
	createMutex  = kernel.NewProc("CreateMutexW")
	waitFor      = kernel.NewProc("WaitForSingleObject")
	releaseMutex = kernel.NewProc("ReleaseMutex")

	powrprof       = systemDLL("powrprof.dll")
	powerGetActive = powrprof.NewProc("PowerGetActiveScheme")
	powerSetActive = powrprof.NewProc("PowerSetActiveScheme")
	powerDuplicate = powrprof.NewProc("PowerDuplicateScheme")
	powerDelete    = powrprof.NewProc("PowerDeleteScheme")
	powerRead      = []*syscall.LazyProc{
		powrprof.NewProc("PowerReadACValueIndex"),
		powrprof.NewProc("PowerReadDCValueIndex"),
	}
	powerWrite = []*syscall.LazyProc{
		powrprof.NewProc("PowerWriteACValueIndex"),
		powrprof.NewProc("PowerWriteDCValueIndex"),
	}
)

func systemDLL(name string) *syscall.LazyDLL {
	var directory [32768]uint16
	n, _, err := kernel.NewProc("GetSystemDirectoryW").Call(
		uintptr(unsafe.Pointer(&directory[0])), uintptr(len(directory)))
	if n == 0 || n >= uintptr(len(directory)) {
		panic(err)
	}
	return syscall.NewLazyDLL(filepath.Join(syscall.UTF16ToString(directory[:n]), name))
}

type windowsPower struct{}

func powerError(operation string, code uintptr) error {
	if code == 0 {
		return nil
	}
	return fmt.Errorf("%s (Windows 오류 %d)", operation, code)
}

func (windowsPower) active() (planID, error) {
	var address *planID
	code, _, _ := powerGetActive.Call(0, uintptr(unsafe.Pointer(&address)))
	if err := powerError("현재 전원 계획을 확인할 수 없습니다", code); err != nil {
		return planID{}, err
	}
	if address == nil {
		return planID{}, errors.New("Windows에서 현재 전원 계획을 반환하지 않았습니다")
	}
	id := *address
	localFree.Call(uintptr(unsafe.Pointer(address)))
	return id, nil
}

func (windowsPower) activate(id planID) error {
	code, _, _ := powerSetActive.Call(0, uintptr(unsafe.Pointer(&id)))
	return powerError("전원 계획을 활성화할 수 없습니다", code)
}

func (windowsPower) duplicate(source, target planID) error {
	address := &target
	code, _, _ := powerDuplicate.Call(0, uintptr(unsafe.Pointer(&source)), uintptr(unsafe.Pointer(&address)))
	return powerError("전원 계획을 복제할 수 없습니다", code)
}

func (windowsPower) delete(id planID) error {
	code, _, _ := powerDelete.Call(0, uintptr(unsafe.Pointer(&id)))
	if code == 2 || code == 1168 {
		return nil
	}
	return powerError("임시 전원 계획을 삭제할 수 없습니다", code)
}

func (windowsPower) read(id planID, source int, setting planID, optional bool) (uint32, bool, error) {
	var value uint32
	code, _, _ := powerRead[source].Call(0, uintptr(unsafe.Pointer(&id)), uintptr(unsafe.Pointer(&processor)),
		uintptr(unsafe.Pointer(&setting)), uintptr(unsafe.Pointer(&value)))
	if optional && (code == 2 || code == 50 || code == 87 || code == 1168) {
		return 0, false, nil
	}
	return value, code == 0, powerError("CPU 전원 설정을 확인할 수 없습니다", code)
}

func (p windowsPower) write(id planID, source int, setting planID, value uint32) error {
	code, _, _ := powerWrite[source].Call(0, uintptr(unsafe.Pointer(&id)), uintptr(unsafe.Pointer(&processor)),
		uintptr(unsafe.Pointer(&setting)), uintptr(value))
	if err := powerError("CPU 전원 설정을 변경할 수 없습니다", code); err != nil {
		return err
	}
	actual, _, err := p.read(id, source, setting, false)
	if err == nil && actual != value {
		return errors.New("변경한 CPU 전원 설정을 확인할 수 없습니다")
	}
	return err
}

func textPointer(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func lockWorker() (func(), error) {
	name := textPointer("Global\\QuietSession.PowerPlan.v1")
	raw, _, err := createMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if raw == 0 {
		return nil, fmt.Errorf("전원 관리 잠금에 접근할 수 없습니다: %w", err)
	}
	handle := syscall.Handle(raw)
	result, _, _ := waitFor.Call(raw, 0)
	if result != 0 && result != 0x80 {
		syscall.CloseHandle(handle)
		return nil, errors.New("다른 창에서 조용한 모드를 사용하고 있습니다")
	}
	return func() {
		releaseMutex.Call(raw)
		syscall.CloseHandle(handle)
	}, nil
}

func journalPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "QuietSession", "restore.json")
}
