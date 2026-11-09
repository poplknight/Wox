//go:build darwin

package info

/*
#include <mach/mach.h>
#include <stdint.h>

static int woxInfoReadCPUTicks(uint64_t *idle, uint64_t *total) {
	host_cpu_load_info_data_t info;
	mach_msg_type_number_t count = HOST_CPU_LOAD_INFO_COUNT;
	if (host_statistics(mach_host_self(), HOST_CPU_LOAD_INFO, (host_info_t)&info, &count) != KERN_SUCCESS) {
		return 1;
	}
	uint64_t nextTotal = 0;
	for (int i = 0; i < CPU_STATE_MAX; i++) {
		nextTotal += info.cpu_ticks[i];
	}
	*idle = info.cpu_ticks[CPU_STATE_IDLE];
	*total = nextTotal;
	return nextTotal == 0;
}

static int woxInfoReadMemory(uint64_t *used, uint64_t *total) {
	host_t host = mach_host_self();
	host_basic_info_data_t basic;
	mach_msg_type_number_t basicCount = HOST_BASIC_INFO_COUNT;
	if (host_info(host, HOST_BASIC_INFO, (host_info_t)&basic, &basicCount) != KERN_SUCCESS) {
		return 1;
	}
	vm_size_t pageSize = 0;
	if (host_page_size(host, &pageSize) != KERN_SUCCESS || pageSize == 0 || basic.max_mem == 0) {
		return 1;
	}
	vm_statistics64_data_t stats;
	mach_msg_type_number_t statsCount = HOST_VM_INFO64_COUNT;
	if (host_statistics64(host, HOST_VM_INFO64, (host_info64_t)&stats, &statsCount) != KERN_SUCCESS) {
		return 1;
	}
	uint64_t totalPages = basic.max_mem / pageSize;
	uint64_t available = stats.free_count + stats.inactive_count + stats.purgeable_count;
	if (available > totalPages) {
		available = totalPages;
	}
	*total = basic.max_mem;
	*used = (totalPages - available) * pageSize;
	return 0;
}
*/
import "C"
import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

var (
	hardwareMu     sync.Mutex
	cachedHardware hardwareProfile
)

// warmHardwareProfile reads the marketing model and display once, without blocking the first query.
func warmHardwareProfile(ctx context.Context) {
	output, err := exec.CommandContext(ctx, "/usr/sbin/system_profiler", "SPHardwareDataType", "SPDisplaysDataType", "SPPowerDataType", "-detailLevel", "mini").Output()
	if err != nil {
		return
	}
	profile := parseProfiler(string(output))
	hardwareMu.Lock()
	cachedHardware = profile
	hardwareMu.Unlock()
}

func currentHardwareProfile() hardwareProfile {
	hardwareMu.Lock()
	defer hardwareMu.Unlock()
	return cachedHardware
}

func readPlatform(ctx context.Context, state *sampleState) infoSnapshot {
	snap := infoSnapshot{}
	if percent, ok := percentAfterSample(ctx, state, darwinCPUTicks); ok {
		snap.CPU = percent
		snap.CPUKnown = true
	}
	if used, total, ok := darwinMemory(); ok {
		snap.MemUsed = used
		snap.MemTotal = total
		snap.MemKnown = true
		if snap.MemoryText == "" {
			snap.MemoryText = formatBytes(total)
		}
	}
	snap.Model = sysctlString("hw.model")
	snap.Chip = sysctlString("machdep.cpu.brand_string")
	if version, err := exec.CommandContext(ctx, "/usr/bin/sw_vers", "-productVersion").Output(); err == nil {
		snap.OSName = macOSLabel(strings.TrimSpace(string(version)))
	}
	if boot, err := unix.SysctlTimeval("kern.boottime"); err == nil {
		snap.Uptime = time.Since(time.Unix(boot.Sec, int64(boot.Usec)*1000))
		snap.UptimeKnown = snap.Uptime > 0
	}
	profile := currentHardwareProfile()
	if profile.Model != "" {
		snap.Model = profile.Model
	}
	if profile.Chip != "" {
		snap.Chip = profile.Chip
	}
	if profile.Memory != "" {
		snap.MemoryText = profile.Memory
	}
	snap.Serial = profile.Serial
	snap.Display = profile.Display
	snap.CycleCount = profile.Cycles
	snap.Health = profile.Health
	if battery, known, status, remaining, ok := darwinPower(ctx); ok {
		snap.PowerOK = true
		snap.BatteryKnown = known
		snap.BatteryPercent = battery
		snap.PowerStatus = status
		snap.Remaining = remaining
	}
	snap.Disks = darwinDisks()
	snap.Counters = darwinNetCounters(ctx)
	snap.TopCPU, snap.TopMem = readTopProcesses(ctx)
	return snap
}

func darwinCPUTicks() (uint64, uint64, bool) {
	var idle C.uint64_t
	var total C.uint64_t
	if C.woxInfoReadCPUTicks(&idle, &total) != 0 {
		return 0, 0, false
	}
	return uint64(idle), uint64(total), true
}

func darwinMemory() (uint64, uint64, bool) {
	var used C.uint64_t
	var total C.uint64_t
	if C.woxInfoReadMemory(&used, &total) != 0 || total == 0 {
		return 0, 0, false
	}
	return uint64(used), uint64(total), true
}

func sysctlString(name string) string {
	value, err := unix.Sysctl(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func darwinPower(ctx context.Context) (int, bool, string, string, bool) {
	output, err := exec.CommandContext(ctx, "/usr/bin/pmset", "-g", "batt").Output()
	if err != nil {
		return 0, false, "", "", false
	}
	return parsePMSet(string(output))
}

func darwinDisks() []diskUsage {
	paths := []string{"/", "/System/Volumes/Data"}
	entries, err := exec.Command("/bin/ls", "-1", "/Volumes").Output()
	if err == nil {
		for _, name := range strings.Split(strings.TrimSpace(string(entries)), "\n") {
			name = strings.TrimSpace(name)
			if name == "" || name == "Macintosh HD" {
				continue
			}
			paths = append(paths, "/Volumes/"+name)
		}
	}
	disks := make([]diskUsage, 0, len(paths))
	seen := map[uint64]bool{}
	for _, path := range paths {
		used, total, ok := diskStat(path)
		if !ok || seen[total] {
			continue
		}
		seen[total] = true
		disks = append(disks, diskUsage{Name: path, Used: used, Total: total})
	}
	return disks
}

func diskStat(path string) (uint64, uint64, bool) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil || stat.Blocks == 0 || stat.Bsize == 0 {
		return 0, 0, false
	}
	bsize := uint64(stat.Bsize)
	total := stat.Blocks * bsize
	free := stat.Bavail * bsize
	if free > total {
		return 0, 0, false
	}
	return total - free, total, true
}

func darwinNetCounters(ctx context.Context) []netCounter {
	output, err := exec.CommandContext(ctx, "/usr/sbin/netstat", "-ibn").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) == 0 {
		return nil
	}
	header := strings.Fields(lines[0])
	rxIndex, txIndex := -1, -1
	for index, name := range header {
		switch name {
		case "Ibytes":
			rxIndex = index
		case "Obytes":
			txIndex = index
		}
	}
	if rxIndex < 0 || txIndex < 0 {
		return nil
	}
	counters := map[string]netCounter{}
	order := []string{}
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) <= txIndex || len(fields) <= rxIndex {
			continue
		}
		name := fields[0]
		if name == "" || counters[name].Name != "" {
			continue
		}
		rx, rxErr := strconv.ParseUint(fields[rxIndex], 10, 64)
		tx, txErr := strconv.ParseUint(fields[txIndex], 10, 64)
		if rxErr != nil || txErr != nil {
			continue
		}
		counters[name] = netCounter{Name: name, Rx: rx, Tx: tx}
		order = append(order, name)
	}
	result := make([]netCounter, 0, len(order))
	for _, name := range order {
		result = append(result, counters[name])
	}
	return result
}
