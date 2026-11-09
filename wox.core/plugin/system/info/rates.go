package info

import (
	"context"
	"time"
)

// netRates converts cumulative interface counters into per-second rates.
// The first sample only records a baseline, so callers can show a calculating state.
func netRates(previous []netCounter, previousAt, now time.Time, current []netCounter) ([]nicRate, bool) {
	rates := make([]nicRate, 0, len(current))
	elapsed := now.Sub(previousAt).Seconds()
	known := !previousAt.IsZero() && elapsed >= 0.2
	prior := map[string]netCounter{}
	for _, counter := range previous {
		prior[counter.Name] = counter
	}
	for _, counter := range current {
		rate := nicRate{Name: counter.Name}
		old, ok := prior[counter.Name]
		if known && ok && counter.Rx >= old.Rx && counter.Tx >= old.Tx {
			rate.Down = float64(counter.Rx-old.Rx) / elapsed
			rate.Up = float64(counter.Tx-old.Tx) / elapsed
			rate.Known = true
		}
		rates = append(rates, rate)
	}
	anyKnown := false
	for _, rate := range rates {
		if rate.Known {
			anyKnown = true
			break
		}
	}
	return rates, anyKnown
}

// applyNetRates stores the latest counters and returns rates against the previous sample.
func applyNetRates(state *sampleState, counters []netCounter, now time.Time) ([]nicRate, bool) {
	if state == nil {
		return nil, false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	rates, known := netRates(state.net, state.netAt, now, counters)
	state.net = append([]netCounter(nil), counters...)
	state.netAt = now
	return rates, known
}

// percentAfterSample returns CPU usage, taking a short second sample when no baseline exists yet.
func percentAfterSample(ctx context.Context, state *sampleState, read func() (uint64, uint64, bool)) (float64, bool) {
	idle, total, ok := read()
	if !ok {
		return 0, false
	}
	if percent, known := cpuPercentFromSamples(state, idle, total); known {
		return percent, true
	}
	timer := time.NewTimer(150 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return 0, false
	case <-timer.C:
	}
	idle, total, ok = read()
	if !ok {
		return 0, false
	}
	return cpuPercentFromSamples(state, idle, total)
}

// cpuPercentFromSamples returns usage between two cumulative idle/total readings.
func cpuPercentFromSamples(state *sampleState, idle, total uint64) (float64, bool) {
	if state == nil || total == 0 {
		return 0, false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	previousIdle, previousTotal, valid := state.cpuIdle, state.cpuTotal, state.cpuValid
	state.cpuIdle, state.cpuTotal, state.cpuValid = idle, total, true
	if !valid || total <= previousTotal || idle > total || previousIdle > idle {
		return 0, false
	}
	totalDelta := total - previousTotal
	idleDelta := idle - previousIdle
	if idleDelta > totalDelta || totalDelta == 0 {
		return 0, false
	}
	return 100 * float64(totalDelta-idleDelta) / float64(totalDelta), true
}
