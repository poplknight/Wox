//go:build linux

package info

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func warmHardwareProfile(context.Context) {}

func readPlatform(ctx context.Context, state *sampleState) infoSnapshot {
	snap := infoSnapshot{}
	if percent, ok := percentAfterSample(ctx, state, linuxCPUTicks); ok {
		snap.CPU = percent
		snap.CPUKnown = true
	}
	if used, total, ok := linuxMemory(); ok {
		snap.MemUsed = used
		snap.MemTotal = total
		snap.MemKnown = true
		snap.MemoryText = formatBytes(total)
	}
	snap.Model, snap.Chip = linuxProduct()
	snap.OSName = linuxOSName()
	if boot, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(boot))
		if len(fields) > 0 {
			if seconds, err := strconv.ParseFloat(fields[0], 64); err == nil {
				snap.Uptime = time.Duration(seconds * float64(time.Second))
				snap.UptimeKnown = true
			}
		}
	}
	if battery, known, status, ok := linuxPower(); ok {
		snap.PowerOK = true
		snap.BatteryKnown = known
		snap.BatteryPercent = battery
		snap.PowerStatus = status
	}
	snap.CycleCount = linuxPowerFile("cycle_count")
	snap.Disks = linuxDisks()
	snap.Counters = linuxNetCounters()
	snap.TopCPU, snap.TopMem = readTopProcesses(ctx)
	return snap
}

func linuxCPUTicks() (uint64, uint64, bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	fields := strings.Fields(strings.SplitN(string(data), "\n", 2)[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, false
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return idle, total, total > 0
}

func linuxMemory() (uint64, uint64, bool) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	info := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		info[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}
	total := info["MemTotal"]
	available := info["MemAvailable"]
	if total == 0 || available > total {
		return 0, 0, false
	}
	return total - available, total, true
}

func linuxProduct() (string, string) {
	vendor := linuxFileLine("/sys/class/dmi/id/sys_vendor")
	product := linuxFileLine("/sys/class/dmi/id/product_name")
	cpu := linuxCPUModel()
	return strings.TrimSpace(vendor + " " + product), cpu
}

func linuxCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(key) == "model name" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func linuxOSName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	var name, version string
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		switch key {
		case "PRETTY_NAME":
			return value
		case "NAME":
			name = value
		case "VERSION":
			version = value
		}
	}
	return strings.TrimSpace(name + " " + version)
}

func linuxPower() (int, bool, string, bool) {
	paths, err := filepath.Glob("/sys/class/power_supply/BAT*")
	if err != nil || len(paths) == 0 {
		return 0, false, "", false
	}
	capacityText := linuxFileLine(paths[0] + "/capacity")
	capacity, err := strconv.Atoi(capacityText)
	if err != nil {
		return 0, false, linuxPowerStatus(linuxFileLine(paths[0] + "/status")), true
	}
	return capacity, true, linuxPowerStatus(linuxFileLine(paths[0] + "/status")), true
}

func linuxPowerStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "charging":
		return powerStatusCharging
	case "discharging":
		return powerStatusDischarging
	case "full":
		return powerStatusCharged
	case "not charging":
		return powerStatusAC
	default:
		return ""
	}
}

func linuxPowerFile(name string) string {
	paths, err := filepath.Glob("/sys/class/power_supply/BAT*")
	if err != nil || len(paths) == 0 {
		return ""
	}
	return linuxFileLine(paths[0] + "/" + name)
}

func linuxDisks() []diskUsage {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}
	disks := []diskUsage{}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || seen[fields[1]] || !linuxRealFilesystem(fields[2]) {
			continue
		}
		used, total, ok := diskStat(fields[1])
		if !ok {
			continue
		}
		seen[fields[1]] = true
		disks = append(disks, diskUsage{Name: fields[1], Used: used, Total: total})
	}
	return disks
}

func linuxRealFilesystem(kind string) bool {
	switch kind {
	case "ext4", "ext3", "xfs", "btrfs", "zfs", "ntfs", "ntfs3", "vfat", "exfat", "fuseblk", "f2fs":
		return true
	default:
		return false
	}
}

func diskStat(path string) (uint64, uint64, bool) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil || stat.Blocks == 0 || stat.Bsize <= 0 {
		return 0, 0, false
	}
	bsize := uint64(stat.Bsize)
	total := stat.Blocks * bsize
	free := uint64(stat.Bavail) * bsize
	if free > total {
		return 0, 0, false
	}
	return total - free, total, true
}

func linuxNetCounters() []netCounter {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil
	}
	counters := []netCounter{}
	for _, line := range strings.Split(string(data), "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		fields := strings.Fields(rest)
		if name == "" || len(fields) < 9 {
			continue
		}
		rx, rxErr := strconv.ParseUint(fields[0], 10, 64)
		tx, txErr := strconv.ParseUint(fields[8], 10, 64)
		if rxErr != nil || txErr != nil {
			continue
		}
		counters = append(counters, netCounter{Name: name, Rx: rx, Tx: tx})
	}
	return counters
}

func linuxFileLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
