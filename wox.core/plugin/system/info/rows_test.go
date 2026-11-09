package info

import (
	"strings"
	"testing"
	"time"
)

func TestInfoRowsKeepStableCategories(t *testing.T) {
	snap := infoSnapshot{
		Model: "MacBook Air", Chip: "Apple M4", MemoryText: "16 GB", OSName: "Sequoia 15.7.9",
		CPU: 23, CPUKnown: true,
		MemUsed: 12 << 30, MemTotal: 16 << 30, MemKnown: true,
		Addresses: []nicAddress{{Name: "en0", IPv4: []string{"192.168.1.20", "100.1.1.1"}}},
		Counters:  []netCounter{{Name: "en0"}},
		Net:       []nicRate{{Name: "en0", Down: 1500, Up: 256, Known: true}},
		NetKnown:  true,
		PowerOK:   true, BatteryKnown: true, BatteryPercent: 78, PowerStatus: powerStatusCharging, Remaining: "3:12",
		Disks:    []diskUsage{{Name: "/", Used: 100 << 30, Total: 200 << 30}, {Name: "/System/Volumes/Data", Used: 120 << 30, Total: 460 << 30}},
		UserName: "tal",
	}
	rows := infoRows(snap, "", "", func(key string) string { return key })
	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Identity)
		if row.Drill != row.Identity {
			t.Fatalf("%s should drill into itself", row.Identity)
		}
	}
	want := []string{commandMachine, commandIP, commandCPU, commandMemory, commandNet, commandPower, commandDisk, commandUser}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("categories = %s", strings.Join(got, ","))
	}
	if rows[0].Subtitle != "MacBook Air · Apple M4 · 16 GB · Sequoia 15.7.9" {
		t.Fatalf("machine summary = %s", rows[0].Subtitle)
	}
	if rows[1].Subtitle != "192.168.1.20" {
		t.Fatalf("ip summary = %s", rows[1].Subtitle)
	}
}

func TestInfoRowsOmitPowerWhenUnavailable(t *testing.T) {
	snap := infoSnapshot{Model: "Desktop", CPU: 10, CPUKnown: true, UserName: "tal"}
	for _, row := range infoRows(snap, "", "", func(key string) string { return key }) {
		if row.Identity == commandPower {
			t.Fatal("power must stay out of the list when the machine has no power reading")
		}
	}
}

func TestMachineDetailsCopyValues(t *testing.T) {
	snap := infoSnapshot{Model: "MacBook Air", Chip: "Apple M4", UserName: "tal", Uptime: 3*time.Hour + 4*time.Minute, UptimeKnown: true}
	rows := infoRows(snap, commandMachine, "", func(key string) string {
		switch key {
		case "plugin_info_duration_hours":
			return "%dh %dm"
		default:
			return key
		}
	})
	if len(rows) < 2 || rows[0].Title != "MacBook Air" || rows[0].Copy != "MacBook Air" || rows[0].Drill != "" {
		t.Fatalf("model row = %+v", rows[0])
	}
	found := false
	for _, row := range rows {
		if row.Identity == "machine:uptime" && row.Copy == "3h 4m" {
			found = true
		}
	}
	if !found {
		t.Fatal("uptime row was not copied as a duration")
	}
}

func TestUserDetailCopiesName(t *testing.T) {
	rows := infoRows(infoSnapshot{UserName: "tal", Home: "/Users/tal"}, commandUser, "", func(key string) string { return key })
	if len(rows) != 2 || rows[0].Title != "tal" || rows[0].Copy != "tal" {
		t.Fatalf("user rows = %+v", rows)
	}
}

func TestParsePMSetAndMacOSLabel(t *testing.T) {
	percent, known, status, remaining, ok := parsePMSet("Now drawing from 'Battery Power'\n -InternalBattery-0 78%; discharging; 3:12 remaining")
	if !ok || !known || percent != 78 || status != powerStatusDischarging || remaining != "3:12" {
		t.Fatalf("pmset = %d %v %s %s %v", percent, known, status, remaining, ok)
	}
	if macOSLabel("15.7.9") != "Sequoia 15.7.9" {
		t.Fatal("macos label")
	}
}

func TestNetRatesNeedABaseline(t *testing.T) {
	current := []netCounter{{Name: "en0", Rx: 2000, Tx: 1000}}
	if _, known := netRates(nil, time.Time{}, time.Now(), current); known {
		t.Fatal("first sample cannot produce a rate")
	}
	rates, known := netRates(current, time.Now().Add(-time.Second), time.Now(), []netCounter{{Name: "en0", Rx: 4048, Tx: 2024}})
	if !known || len(rates) != 1 || rates[0].Down < 2000 || rates[0].Up < 1000 {
		t.Fatalf("rates = %+v known=%v", rates, known)
	}
}

func TestSplitInfoQueryKeepsPartialCategorySearch(t *testing.T) {
	command, search := splitInfoQuery("", "cpu ")
	if command != commandCPU || search != "" {
		t.Fatalf("detail query = %s %q", command, search)
	}
	command, search = splitInfoQuery("", "c")
	if command != "" || search != "c" {
		t.Fatalf("partial query = %s %q", command, search)
	}
}

func TestFormatBytesUsesBinaryUnits(t *testing.T) {
	if got := formatBytes(16 << 30); got != "16 GB" {
		t.Fatalf("formatBytes = %s", got)
	}
}
