package info

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	commandMachine = "machine"
	commandIP      = "ip"
	commandCPU     = "cpu"
	commandMemory  = "mem"
	commandNet     = "net"
	commandPower   = "power"
	commandDisk    = "disk"
	commandUser    = "user"

	topProcessCount = 8

	powerStatusCharging    = "charging"
	powerStatusDischarging = "discharging"
	powerStatusCharged     = "charged"
	powerStatusAC          = "ac"
	powerStatusBattery     = "battery"

	publicIPLoading = "loading"
	publicIPReady   = "ready"
	publicIPFailed  = "failed"
)

// infoText resolves an i18n key to the active language.
type infoText func(key string) string

// infoRow is one launcher result before it is wrapped with actions.
type infoRow struct {
	Identity string
	Title    string
	Subtitle string
	Copy     string
	Drill    string
	Score    int64
}

// infoRows builds the category list or one category's detail rows.
func infoRows(snap infoSnapshot, command, search string, text infoText) []infoRow {
	var rows []infoRow
	if command == "" {
		rows = infoCategories(snap, text)
	} else {
		rows = infoDetails(snap, command, text)
	}
	filtered := make([]infoRow, 0, len(rows))
	for _, row := range rows {
		if infoMatches(search, row) {
			filtered = append(filtered, row)
		}
	}
	for index := range filtered {
		filtered[index].Score = int64(len(filtered)-index) * 10
	}
	return filtered
}

func infoCategories(snap infoSnapshot, text infoText) []infoRow {
	rows := make([]infoRow, 0, 8)
	if summary := machineSummary(snap); summary != "" {
		rows = append(rows, categoryRow(commandMachine, "plugin_info_machine", summary, text))
	}
	if summary := ipSummary(snap); summary != "" {
		rows = append(rows, categoryRow(commandIP, "plugin_info_ip", summary, text))
	}
	if snap.CPUKnown {
		rows = append(rows, categoryRow(commandCPU, "plugin_info_cpu", formatPercent(snap.CPU), text))
	}
	if snap.MemKnown && snap.MemTotal > 0 {
		rows = append(rows, categoryRow(commandMemory, "plugin_info_mem", memorySummary(snap), text))
	}
	if len(snap.Counters) > 0 {
		subtitle := text("plugin_info_calculating")
		if snap.NetKnown {
			subtitle = netSummary(snap)
		}
		rows = append(rows, categoryRow(commandNet, "plugin_info_net", subtitle, text))
	}
	if snap.PowerOK {
		rows = append(rows, categoryRow(commandPower, "plugin_info_power", powerSummary(snap, text), text))
	}
	if len(snap.Disks) > 0 {
		rows = append(rows, categoryRow(commandDisk, "plugin_info_disk", diskSummary(primaryDisk(snap.Disks)), text))
	}
	if snap.UserName != "" {
		rows = append(rows, categoryRow(commandUser, "plugin_info_user", snap.UserName, text))
	}
	return rows
}

func categoryRow(command, titleKey, subtitle string, text infoText) infoRow {
	return infoRow{
		Identity: command,
		Title:    text(titleKey),
		Subtitle: subtitle,
		Copy:     subtitle,
		Drill:    command,
	}
}

func infoDetails(snap infoSnapshot, command string, text infoText) []infoRow {
	switch command {
	case commandMachine:
		return machineDetails(snap, text)
	case commandIP:
		return ipDetails(snap, text)
	case commandCPU:
		return usageDetails("plugin_info_cpu_total", formatPercent(snap.CPU), snap.CPUKnown, snap.TopCPU, true, text)
	case commandMemory:
		return usageDetails("plugin_info_mem_total", memorySummary(snap), snap.MemKnown, snap.TopMem, false, text)
	case commandNet:
		return netDetails(snap, text)
	case commandPower:
		return powerDetails(snap, text)
	case commandDisk:
		return diskDetails(snap, text)
	case commandUser:
		return userDetails(snap, text)
	default:
		return nil
	}
}

func machineDetails(snap infoSnapshot, text infoText) []infoRow {
	rows := []infoRow{}
	appendValue(&rows, "machine:model", text("plugin_info_model"), snap.Model)
	appendValue(&rows, "machine:chip", text("plugin_info_chip"), snap.Chip)
	appendValue(&rows, "machine:memory", text("plugin_info_memory_size"), snap.MemoryText)
	appendValue(&rows, "machine:os", text("plugin_info_os"), snap.OSName)
	appendValue(&rows, "machine:display", text("plugin_info_display"), snap.Display)
	appendValue(&rows, "machine:hostname", text("plugin_info_hostname"), snap.Hostname)
	appendValue(&rows, "machine:serial", text("plugin_info_serial"), snap.Serial)
	if snap.UptimeKnown {
		appendValue(&rows, "machine:uptime", text("plugin_info_uptime"), formatDuration(snap.Uptime, text))
	}
	return rows
}

func ipDetails(snap infoSnapshot, text infoText) []infoRow {
	rows := []infoRow{}
	for _, nic := range snap.Addresses {
		if virtualInterface(nic.Name) {
			continue
		}
		for _, address := range nic.IPv4 {
			appendAddress(&rows, nic.Name, text("plugin_info_ipv4"), address)
		}
		for _, address := range nic.IPv6 {
			appendAddress(&rows, nic.Name, text("plugin_info_ipv6"), address)
		}
		for _, address := range nic.LinkLocal {
			appendAddress(&rows, nic.Name, text("plugin_info_ipv6_link"), address)
		}
	}
	switch snap.PublicState {
	case publicIPReady:
		appendAddress(&rows, text("plugin_info_public_ip"), "IPv4", snap.PublicIPv4)
		appendAddress(&rows, text("plugin_info_public_ip"), "IPv6", snap.PublicIPv6)
	case publicIPFailed:
		rows = append(rows, infoRow{Identity: "ip:public", Title: text("plugin_info_public_ip_failed"), Subtitle: text("plugin_info_public_ip")})
	default:
		rows = append(rows, infoRow{Identity: "ip:public", Title: text("plugin_info_calculating"), Subtitle: text("plugin_info_public_ip")})
	}
	return rows
}

func usageDetails(totalKey, total string, known bool, processes []procUse, cpu bool, text infoText) []infoRow {
	if !known && len(processes) == 0 {
		return nil
	}
	rows := []infoRow{}
	if known && total != "" {
		appendValue(&rows, "usage:total", text(totalKey), total)
	}
	limit := topProcessCount
	if len(processes) < limit {
		limit = len(processes)
	}
	for _, proc := range processes[:limit] {
		value := formatBytes(proc.RSS)
		copy := proc.Name + "  " + value
		if cpu {
			value = formatUsagePercent(proc.CPU)
			copy = fmt.Sprintf("%s  %s", proc.Name, value)
		}
		rows = append(rows, infoRow{
			Identity: "proc:" + proc.Name,
			Title:    proc.Name,
			Subtitle: value,
			Copy:     copy,
		})
	}
	return rows
}

func netDetails(snap infoSnapshot, text infoText) []infoRow {
	rows := []infoRow{}
	if snap.NetKnown {
		appendValue(&rows, "net:total", text("plugin_info_net_total"), netSummary(snap))
	}
	for _, nic := range snap.Net {
		if virtualInterface(nic.Name) {
			continue
		}
		subtitle := text("plugin_info_calculating")
		if nic.Known {
			subtitle = formatTraffic(nic.Down, nic.Up)
		}
		rows = append(rows, infoRow{Identity: "net:" + nic.Name, Title: nic.Name, Subtitle: subtitle, Copy: subtitle})
	}
	return rows
}

func powerDetails(snap infoSnapshot, text infoText) []infoRow {
	if !snap.PowerOK {
		return nil
	}
	rows := []infoRow{}
	if snap.BatteryKnown {
		appendValue(&rows, "power:battery", text("plugin_info_battery"), fmt.Sprintf("%d%%", snap.BatteryPercent))
	}
	appendValue(&rows, "power:source", text("plugin_info_power_source"), powerStatusText(snap.PowerStatus, text))
	appendValue(&rows, "power:remaining", text("plugin_info_remaining"), snap.Remaining)
	appendValue(&rows, "power:cycles", text("plugin_info_cycle_count"), snap.CycleCount)
	appendValue(&rows, "power:health", text("plugin_info_health"), snap.Health)
	return rows
}

func diskDetails(snap infoSnapshot, text infoText) []infoRow {
	rows := make([]infoRow, 0, len(snap.Disks))
	for _, disk := range snap.Disks {
		value := diskSummary(disk)
		rows = append(rows, infoRow{Identity: "disk:" + disk.Name, Title: disk.Name, Subtitle: value, Copy: value})
	}
	_ = text
	return rows
}

func userDetails(snap infoSnapshot, text infoText) []infoRow {
	rows := []infoRow{}
	appendValue(&rows, "user:name", text("plugin_info_username"), snap.UserName)
	appendValue(&rows, "user:full", text("plugin_info_full_name"), snap.FullName)
	appendValue(&rows, "user:home", text("plugin_info_home"), snap.Home)
	return rows
}

func appendValue(rows *[]infoRow, identity, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	*rows = append(*rows, infoRow{Identity: identity, Title: value, Subtitle: label, Copy: value})
}

func appendAddress(rows *[]infoRow, iface, kind, address string) {
	address = strings.TrimSpace(address)
	if address == "" {
		return
	}
	*rows = append(*rows, infoRow{
		Identity: "ip:" + iface + ":" + address,
		Title:    address,
		Subtitle: iface + " · " + kind,
		Copy:     address,
	})
}

// splitInfoQuery treats "info cpu " as the cpu detail view.
// The command is recognized here so a missing category, such as power on Windows, is not declared to the automatic command menu.
func splitInfoQuery(command, search string) (string, string) {
	if strings.TrimSpace(command) != "" {
		return command, search
	}
	if !strings.HasSuffix(search, " ") {
		return "", strings.TrimSpace(search)
	}
	fields := strings.Fields(search)
	if len(fields) == 0 || !knownInfoCommand(fields[0]) {
		return "", strings.TrimSpace(search)
	}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(search), fields[0]))
	return fields[0], rest
}

func knownInfoCommand(command string) bool {
	switch command {
	case commandMachine, commandIP, commandCPU, commandMemory, commandNet, commandPower, commandDisk, commandUser:
		return true
	default:
		return false
	}
}

func infoMatches(search string, row infoRow) bool {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return true
	}
	return strings.Contains(strings.ToLower(row.Identity), search) ||
		strings.Contains(strings.ToLower(row.Title), search) ||
		strings.Contains(strings.ToLower(row.Subtitle), search)
}

func machineSummary(snap infoSnapshot) string {
	return joinParts(snap.Model, snap.Chip, snap.MemoryText, snap.OSName)
}

func memorySummary(snap infoSnapshot) string {
	if !snap.MemKnown || snap.MemTotal == 0 {
		return ""
	}
	percent := 100 * float64(snap.MemUsed) / float64(snap.MemTotal)
	return formatBytes(snap.MemUsed) + " / " + formatBytes(snap.MemTotal) + " · " + formatPercent(percent)
}

func ipSummary(snap infoSnapshot) string {
	var fallback string
	for _, nic := range snap.Addresses {
		if virtualInterface(nic.Name) {
			continue
		}
		for _, address := range nic.IPv4 {
			if fallback == "" {
				fallback = address
			}
			if privateIPv4(address) {
				return address
			}
		}
	}
	if fallback != "" {
		return fallback
	}
	for _, nic := range snap.Addresses {
		if virtualInterface(nic.Name) {
			continue
		}
		if len(nic.IPv6) > 0 {
			return nic.IPv6[0]
		}
	}
	return ""
}

func netSummary(snap infoSnapshot) string {
	var down, up float64
	for _, nic := range snap.Net {
		if virtualInterface(nic.Name) || !nic.Known {
			continue
		}
		down += nic.Down
		up += nic.Up
	}
	return formatTraffic(down, up)
}

func powerSummary(snap infoSnapshot, text infoText) string {
	parts := []string{}
	if snap.BatteryKnown {
		parts = append(parts, fmt.Sprintf("%d%%", snap.BatteryPercent))
	}
	if status := powerStatusText(snap.PowerStatus, text); status != "" {
		parts = append(parts, status)
	}
	if snap.Remaining != "" {
		parts = append(parts, snap.Remaining)
	}
	return joinParts(parts...)
}

func powerStatusText(status string, text infoText) string {
	switch status {
	case powerStatusCharging:
		return text("plugin_info_charging")
	case powerStatusDischarging:
		return text("plugin_info_discharging")
	case powerStatusCharged:
		return text("plugin_info_charged")
	case powerStatusAC:
		return text("plugin_info_ac")
	case powerStatusBattery:
		return text("plugin_info_battery_power")
	default:
		return ""
	}
}

func diskSummary(disk diskUsage) string {
	if disk.Total == 0 {
		return ""
	}
	percent := 100 * float64(disk.Used) / float64(disk.Total)
	return formatBytes(disk.Used) + " / " + formatBytes(disk.Total) + " · " + formatPercent(percent)
}

func primaryDisk(disks []diskUsage) diskUsage {
	var best diskUsage
	for _, disk := range disks {
		if disk.Total > best.Total {
			best = disk
		}
	}
	return best
}

func joinParts(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " · ")
}

func formatUsagePercent(value float64) string {
	if value < 0 {
		value = 0
	}
	return fmt.Sprintf("%.0f%%", value)
}

func formatPercent(value float64) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return fmt.Sprintf("%.0f%%", value)
}

func formatTraffic(down, up float64) string {
	return "↓ " + formatBytes(uint64(down)) + "/s  ↑ " + formatBytes(uint64(up)) + "/s"
}

func formatDuration(duration time.Duration, text infoText) string {
	if duration < 0 {
		duration = 0
	}
	minutes := int(duration.Minutes())
	days := minutes / (24 * 60)
	minutes -= days * 24 * 60
	hours := minutes / 60
	minutes -= hours * 60
	if days > 0 {
		return fmt.Sprintf(text("plugin_info_duration_days"), days, hours)
	}
	return fmt.Sprintf(text("plugin_info_duration_hours"), hours, minutes)
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	size := float64(value) / unit
	exponent := 0
	for size >= unit && exponent < len(units)-1 {
		size /= unit
		exponent++
	}
	suffix := units[exponent]
	rounded := math.Round(size*10) / 10
	if math.Abs(rounded-math.Round(rounded)) < 0.05 {
		return fmt.Sprintf("%.0f %s", rounded, suffix)
	}
	return fmt.Sprintf("%.1f %s", rounded, suffix)
}

func privateIPv4(address string) bool {
	if strings.HasPrefix(address, "10.") || strings.HasPrefix(address, "192.168.") {
		return true
	}
	if !strings.HasPrefix(address, "172.") {
		return false
	}
	parts := strings.Split(address, ".")
	if len(parts) < 2 {
		return false
	}
	second := 0
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return false
		}
		second = second*10 + int(c-'0')
	}
	return second >= 16 && second <= 31
}

func virtualInterface(name string) bool {
	name = strings.ToLower(name)
	switch {
	case name == "lo" || strings.HasPrefix(name, "lo"):
		return true
	case strings.HasPrefix(name, "awdl"), strings.HasPrefix(name, "llw"), strings.HasPrefix(name, "utun"):
		return true
	case strings.HasPrefix(name, "bridge"), strings.HasPrefix(name, "gif"), strings.HasPrefix(name, "stf"):
		return true
	case strings.HasPrefix(name, "ap"), strings.HasPrefix(name, "anpi"), strings.HasPrefix(name, "docker"):
		return true
	case strings.HasPrefix(name, "veth"), strings.HasPrefix(name, "br-"), strings.HasPrefix(name, "cni"):
		return true
	default:
		return false
	}
}
