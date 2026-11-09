//go:build windows

package info

import (
	"context"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

type windowsFileTime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

type windowsMemoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func warmHardwareProfile(context.Context) {}

// readPlatform collects Windows machine info. Power is intentionally omitted.
func readPlatform(ctx context.Context, state *sampleState) infoSnapshot {
	snap := infoSnapshot{}
	if percent, ok := percentAfterSample(ctx, state, windowsCPUTicks); ok {
		snap.CPU = percent
		snap.CPUKnown = true
	}
	if used, total, ok := windowsMemory(); ok {
		snap.MemUsed = used
		snap.MemTotal = total
		snap.MemKnown = true
		snap.MemoryText = formatBytes(total)
	}
	snap.Model = windowsRegistryValue(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, "SystemProductName")
	vendor := windowsRegistryValue(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, "SystemManufacturer")
	if vendor != "" && snap.Model != "" && !strings.Contains(strings.ToLower(snap.Model), strings.ToLower(vendor)) {
		snap.Model = vendor + " " + snap.Model
	}
	snap.Chip = windowsRegistryValue(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString")
	product := windowsRegistryValue(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName")
	display := windowsRegistryValue(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "DisplayVersion")
	snap.OSName = strings.TrimSpace(product + " " + display)
	snap.Disks = windowsDisks()
	snap.Counters = windowsNetCounters()
	snap.TopCPU, snap.TopMem = readWindowsProcesses(state)
	return snap
}

func windowsCPUTicks() (uint64, uint64, bool) {
	var idle, kernel, user windowsFileTime
	ret, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if ret == 0 {
		return 0, 0, false
	}
	idleTicks := windowsFileTimeTicks(idle)
	total := windowsFileTimeTicks(kernel) + windowsFileTimeTicks(user)
	return idleTicks, total, total > idleTicks
}

func windowsFileTimeTicks(fileTime windowsFileTime) uint64 {
	return uint64(fileTime.HighDateTime)<<32 | uint64(fileTime.LowDateTime)
}

func windowsMemory() (uint64, uint64, bool) {
	status := windowsMemoryStatusEx{Length: uint32(unsafe.Sizeof(windowsMemoryStatusEx{}))}
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 || status.TotalPhys == 0 || status.AvailPhys > status.TotalPhys {
		return 0, 0, false
	}
	return status.TotalPhys - status.AvailPhys, status.TotalPhys, true
}

func windowsRegistryValue(root registry.Key, path, name string) string {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func windowsDisks() []diskUsage {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	disks := []diskUsage{}
	for letter := 0; letter < 26; letter++ {
		if mask&(1<<uint(letter)) == 0 {
			continue
		}
		root := string(rune('A'+letter)) + `:\`
		var free, total, avail uint64
		path, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if err := windows.GetDiskFreeSpaceEx(path, &free, &total, &avail); err != nil || total == 0 || avail > total {
			continue
		}
		disks = append(disks, diskUsage{Name: root, Used: total - avail, Total: total})
	}
	return disks
}

func windowsNetCounters() []netCounter {
	ifaces, err := netInterfaces()
	if err != nil {
		return nil
	}
	counters := make([]netCounter, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.Flags&netFlagUp == 0 || iface.Flags&netFlagLoopback != 0 {
			continue
		}
		row := windows.MibIfRow2{InterfaceIndex: uint32(iface.Index)}
		if err := windows.GetIfEntry2Ex(0, &row); err != nil {
			continue
		}
		name := windows.UTF16ToString(row.Alias[:])
		if name == "" {
			name = iface.Name
		}
		counters = append(counters, netCounter{Name: name, Rx: row.InOctets, Tx: row.OutOctets})
	}
	return counters
}
