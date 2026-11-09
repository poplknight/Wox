package info

import (
	"context"
	"time"
)

// collectSnapshot reads the current machine and folds counter deltas into rates.
func collectSnapshot(ctx context.Context, state *sampleState, now time.Time) infoSnapshot {
	snap := readPlatform(ctx, state)
	readHostIdentity(&snap)
	snap.Net, snap.NetKnown = applyNetRates(state, snap.Counters, now)
	return snap
}
