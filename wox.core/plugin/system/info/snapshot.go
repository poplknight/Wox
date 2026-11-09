package info

import "time"

// infoSnapshot is the local machine state used to build one query.
type infoSnapshot struct {
	Model       string
	Chip        string
	MemoryText  string
	OSName      string
	Hostname    string
	Serial      string
	Display     string
	Uptime      time.Duration
	UptimeKnown bool

	PowerOK        bool
	BatteryPercent int
	BatteryKnown   bool
	PowerStatus    string
	Remaining      string
	CycleCount     string
	Health         string

	CPU      float64
	CPUKnown bool

	MemUsed  uint64
	MemTotal uint64
	MemKnown bool

	Addresses   []nicAddress
	PublicIPv4  string
	PublicIPv6  string
	PublicState string

	Disks []diskUsage

	Counters []netCounter
	Net      []nicRate
	NetKnown bool

	UserName string
	FullName string
	Home     string

	TopCPU []procUse
	TopMem []procUse
}

type nicAddress struct {
	Name      string
	IPv4      []string
	IPv6      []string
	LinkLocal []string
}

type diskUsage struct {
	Name  string
	Used  uint64
	Total uint64
}

type netCounter struct {
	Name string
	Rx   uint64
	Tx   uint64
}

type nicRate struct {
	Name  string
	Down  float64
	Up    float64
	Known bool
}

type procUse struct {
	Name string
	CPU  float64
	RSS  uint64
}
