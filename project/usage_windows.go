package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	getSystemTimes = kernel.NewProc("GetSystemTimes")
	globalMemory   = kernel.NewProc("GlobalMemoryStatusEx")
	pdh            = systemDLL("pdh.dll")
	pdhOpen        = pdh.NewProc("PdhOpenQueryW")
	pdhClose       = pdh.NewProc("PdhCloseQuery")
	pdhAddEnglish  = pdh.NewProc("PdhAddEnglishCounterW")
	pdhAdd         = pdh.NewProc("PdhAddCounterW")
	pdhRemove      = pdh.NewProc("PdhRemoveCounter")
	pdhInfo        = pdh.NewProc("PdhGetCounterInfoW")
	pdhExpand      = pdh.NewProc("PdhExpandWildCardPathW")
	pdhCollect     = pdh.NewProc("PdhCollectQueryData")
	pdhFormat      = pdh.NewProc("PdhGetFormattedCounterValue")
)

const (
	pdhMoreData        = 0x800007D2
	pdhRefreshCounters = 4
	pdhFormatDouble    = 0x200
	pdhFormatNoCap     = 0x8000
)

type memoryStatus struct {
	size, load uint32
	memory     [7]uint64
}

func readMemory() usageValue {
	status := memoryStatus{size: uint32(unsafe.Sizeof(memoryStatus{}))}
	ok, _, _ := globalMemory.Call(uintptr(unsafe.Pointer(&status)))
	return usageValue{int(status.load), ok != 0 && status.load <= 100}
}

func readCPUTimes() (cpuTimes, bool) {
	var idle, kernel, user syscall.Filetime
	ok, _, _ := getSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	// FILETIME is an unsigned 64-bit count of 100 ns units.
	ticks := func(t syscall.Filetime) uint64 { return uint64(t.HighDateTime)<<32 | uint64(t.LowDateTime) }
	return cpuTimes{ticks(idle), ticks(kernel), ticks(user)}, ok != 0
}

type gpuCounter struct {
	handle uintptr
	engine string
}

type gpuSampler struct {
	query    uintptr
	wildcard string
	counters map[string]gpuCounter
	rescan   time.Time
}

type pdhCounterInfo struct {
	length, kind, version, status uint32
	scale, defaultScale           int32
	userData, queryData           uintptr
	fullPath                      *uint16
}

func (s *gpuSampler) open() bool {
	var query uintptr
	code, _, _ := pdhOpen.Call(0, 0, uintptr(unsafe.Pointer(&query)))
	if code != 0 {
		return false
	}

	var counter uintptr
	code, _, _ = pdhAddEnglish.Call(query,
		uintptr(unsafe.Pointer(textPointer(`\GPU Engine(*)\Utilization Percentage`))),
		0, uintptr(unsafe.Pointer(&counter)))
	if code != 0 {
		pdhClose.Call(query)
		return false
	}
	path := localizedWildcard(counter)
	pdhRemove.Call(counter)
	if path == "" {
		pdhClose.Call(query)
		return false
	}
	s.query = query
	s.wildcard = path
	s.counters = make(map[string]gpuCounter)
	return true
}

func localizedWildcard(counter uintptr) string {
	var size uint32
	code, _, _ := pdhInfo.Call(counter, 0, uintptr(unsafe.Pointer(&size)), 0)
	if code != pdhMoreData || size < uint32(unsafe.Sizeof(pdhCounterInfo{})) || size > 1<<20 {
		return ""
	}
	buffer := make([]byte, size)
	code, _, _ = pdhInfo.Call(counter, 0, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&buffer[0])))
	if code != 0 {
		return ""
	}
	path := (*pdhCounterInfo)(unsafe.Pointer(&buffer[0])).fullPath
	start := uintptr(unsafe.Pointer(&buffer[0]))
	at := uintptr(unsafe.Pointer(path))
	if at < start || at >= start+uintptr(len(buffer)) || at%2 != 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice(path, (start+uintptr(len(buffer))-at)/2))
}

func (s *gpuSampler) close() {
	if s.query != 0 {
		pdhClose.Call(s.query)
		s.query = 0
	}
}

func (s *gpuSampler) updateCounters() {
	var size uint32
	code, _, _ := pdhExpand.Call(0, uintptr(unsafe.Pointer(textPointer(s.wildcard))), 0,
		uintptr(unsafe.Pointer(&size)), pdhRefreshCounters)
	if code != pdhMoreData || size < 2 || size > 1<<20 {
		return
	}
	buffer := make([]uint16, size)
	code, _, _ = pdhExpand.Call(0, uintptr(unsafe.Pointer(textPointer(s.wildcard))),
		uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 0)
	if code != 0 || size > uint32(len(buffer)) {
		return
	}
	current := make(map[string]bool)
	start := 0
	for i, char := range buffer[:size] {
		if char != 0 {
			continue
		}
		if i == start {
			break
		}
		path := syscall.UTF16ToString(buffer[start:i])
		start = i + 1
		engine := gpuEngineKey(path)
		if engine == "" || current[path] {
			continue
		}
		current[path] = true
		if _, exists := s.counters[path]; exists {
			continue
		}
		var handle uintptr
		code, _, _ := pdhAdd.Call(s.query, uintptr(unsafe.Pointer(textPointer(path))), 0,
			uintptr(unsafe.Pointer(&handle)))
		if code == 0 {
			s.counters[path] = gpuCounter{handle, engine}
		}
	}
	for path, entry := range s.counters {
		if !current[path] {
			pdhRemove.Call(entry.handle)
			delete(s.counters, path)
		}
	}
}

type pdhDouble struct {
	status uint32
	_      uint32
	value  float64
}

func (s *gpuSampler) sample(now time.Time) usageValue {
	if !now.Before(s.rescan) {
		s.rescan = now.Add(5 * time.Second)
		if s.query == 0 && !s.open() {
			return usageValue{}
		}
		s.updateCounters()
	}
	if len(s.counters) == 0 {
		return usageValue{}
	}
	code, _, _ := pdhCollect.Call(s.query)
	if code != 0 {
		return usageValue{}
	}
	engines := make(engineUsage)
	for _, counter := range s.counters {
		var result pdhDouble
		code, _, _ := pdhFormat.Call(counter.handle, pdhFormatDouble|pdhFormatNoCap,
			0, uintptr(unsafe.Pointer(&result)))
		if code == 0 && (result.status == 0 || result.status == 1) {
			engines.add(counter.engine, result.value)
		}
	}
	value, valid := engines.busiest()
	return usageValue{value, valid}
}

func monitorUsage(stop <-chan struct{}, events chan<- usageSnapshot) {
	var gpu gpuSampler
	defer gpu.close()
	var previous cpuTimes
	var hasPrevious bool
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		var snapshot usageSnapshot
		now, valid := readCPUTimes()
		if valid && hasPrevious {
			value, ok := cpuUsage(previous, now)
			snapshot[0] = usageValue{value, ok}
		}
		previous, hasPrevious = now, valid
		snapshot[1] = readMemory()
		snapshot[2] = gpu.sample(time.Now())
		select {
		case events <- snapshot:
		case <-stop:
			return
		}
		select {
		case <-ticker.C:
		case <-stop:
			return
		}
	}
}
