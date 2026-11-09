package info

import (
	"regexp"
	"strings"
)

var (
	pmsetPercent = regexp.MustCompile(`(\d+)%`)
	pmsetState   = regexp.MustCompile(`%;\s*([^;]+)`)
	pmsetRemain  = regexp.MustCompile(`;\s*(\d+:\d+)\s+remaining`)
)

// parsePMSet reads the live battery summary from `pmset -g batt`.
func parsePMSet(output string) (battery int, batteryKnown bool, status, remaining string, ok bool) {
	text := strings.TrimSpace(output)
	if text == "" {
		return 0, false, "", "", false
	}
	if match := pmsetPercent.FindStringSubmatch(text); len(match) == 2 {
		value := 0
		for _, c := range match[1] {
			value = value*10 + int(c-'0')
		}
		battery = value
		batteryKnown = true
		ok = true
	}
	if match := pmsetState.FindStringSubmatch(text); len(match) == 2 {
		status = pmsetStatus(match[1])
		ok = ok || status != ""
	}
	if match := pmsetRemain.FindStringSubmatch(text); len(match) == 2 {
		remaining = match[1]
	}
	if !batteryKnown && strings.Contains(text, "AC Power") {
		return 0, false, powerStatusAC, "", true
	}
	if status == "" && strings.Contains(text, "AC Power") {
		status = powerStatusAC
	}
	if status == "" && strings.Contains(text, "Battery Power") {
		status = powerStatusBattery
	}
	return battery, batteryKnown, status, remaining, ok
}

func pmsetStatus(raw string) string {
	state := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(state, "discharg"):
		return powerStatusDischarging
	case strings.Contains(state, "charg") && strings.Contains(state, "not"):
		return powerStatusBattery
	case strings.Contains(state, "finish"):
		return powerStatusCharged
	case strings.Contains(state, "charged"):
		return powerStatusCharged
	case strings.Contains(state, "charg"):
		return powerStatusCharging
	default:
		return ""
	}
}

// parseProfiler pulls the hardware lines Wox shows in the machine category.
func parseProfiler(text string) hardwareProfile {
	var profile hardwareProfile
	expectMainResolution := false
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := splitProfilerLine(line)
		if !ok {
			continue
		}
		switch key {
		case "Model Name":
			if profile.Model == "" {
				profile.Model = value
			}
		case "Chip":
			profile.Chip = value
		case "Memory":
			profile.Memory = value
		case "Serial Number (system)", "Serial Number":
			if profile.Serial == "" {
				profile.Serial = value
			}
		case "Main Display":
			expectMainResolution = strings.EqualFold(value, "Yes")
		case "Resolution":
			if profile.Display == "" || expectMainResolution {
				profile.Display = strings.ReplaceAll(value, " x ", " × ")
				expectMainResolution = false
			}
		case "Cycle Count":
			profile.Cycles = value
		case "Condition":
			profile.Health = value
		}
	}
	return profile
}

type hardwareProfile struct {
	Model   string
	Chip    string
	Memory  string
	Serial  string
	Display string
	Cycles  string
	Health  string
}

func splitProfilerLine(line string) (string, string, bool) {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

// macOSLabel turns a product version into the release name users recognize.
func macOSLabel(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	majorText, _, _ := strings.Cut(version, ".")
	major := 0
	for _, c := range majorText {
		if c < '0' || c > '9' {
			return "macOS " + version
		}
		major = major*10 + int(c-'0')
	}
	names := map[int]string{
		11: "Big Sur",
		12: "Monterey",
		13: "Ventura",
		14: "Sonoma",
		15: "Sequoia",
		26: "Tahoe",
	}
	if name, ok := names[major]; ok {
		return name + " " + version
	}
	return "macOS " + version
}
