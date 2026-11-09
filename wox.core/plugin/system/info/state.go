package info

import (
	"sync"
	"time"
)

// sampleState keeps the previous counters needed to turn cumulative values into rates.
type sampleState struct {
	mu        sync.Mutex
	cpuIdle   uint64
	cpuTotal  uint64
	cpuValid  bool
	net       []netCounter
	netAt     time.Time
	procTimes map[uint32]uint64
	procWall  uint64
	procValid bool
}
