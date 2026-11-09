//go:build windows

package info

import (
	"sort"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	psapi                    = windows.NewLazySystemDLL("psapi.dll")
	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
)

type processMemoryCounters struct {
	cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// readWindowsProcesses ranks processes by CPU since the previous sample and by current working set.
func readWindowsProcesses(state *sampleState) (cpu []procUse, memory []procUse) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, nil
	}
	defer windows.CloseHandle(snapshot)

	_, total, systemOK := windowsCPUTicks()
	var systemDelta uint64
	previousTimes := map[uint32]uint64{}
	if state != nil {
		state.mu.Lock()
		if systemOK && state.procValid && total > state.procWall {
			systemDelta = total - state.procWall
		}
		for pid, ticks := range state.procTimes {
			previousTimes[pid] = ticks
		}
		state.mu.Unlock()
	}

	nextTimes := map[uint32]uint64{}
	processes := []procUse{}
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		pid := entry.ProcessID
		if pid == 0 {
			continue
		}
		name := windows.UTF16ToString(entry.ExeFile[:])
		ticks, rss, ok := windowsProcessUsage(pid)
		if !ok {
			continue
		}
		nextTimes[pid] = ticks
		proc := procUse{Name: name, RSS: rss}
		if previous, found := previousTimes[pid]; found && systemDelta > 0 && ticks >= previous {
			proc.CPU = 100 * float64(ticks-previous) / float64(systemDelta)
		}
		processes = append(processes, proc)
	}
	if state != nil && systemOK {
		state.mu.Lock()
		state.procTimes = nextTimes
		state.procWall = total
		state.procValid = true
		state.mu.Unlock()
	}

	memory = append([]procUse(nil), processes...)
	sort.SliceStable(memory, func(i, j int) bool { return memory[i].RSS > memory[j].RSS })
	cpu = append([]procUse(nil), processes...)
	sort.SliceStable(cpu, func(i, j int) bool { return cpu[i].CPU > cpu[j].CPU })
	if systemDelta == 0 {
		cpu = nil
	}
	return cpu, memory
}

func windowsProcessUsage(pid uint32) (uint64, uint64, bool) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return 0, 0, false
	}
	defer windows.CloseHandle(handle)

	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return 0, 0, false
	}
	kernelTicks := uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)
	userTicks := uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)
	ticks := kernelTicks + userTicks
	counters := processMemoryCounters{cb: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	ret, _, _ := procGetProcessMemoryInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&counters)), uintptr(counters.cb))
	if ret == 0 {
		return ticks, 0, true
	}
	return ticks, uint64(counters.WorkingSetSize), true
}
