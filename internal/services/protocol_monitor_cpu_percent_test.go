package services

import (
	"math"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSystemMetricsCollector_ComputeCPUPercent_IdleExceedsTotalNoUnderflow is
// the §11.4.115 RED->GREEN polarity-switch regression guard for the CPU%
// underflow defect in (*systemMetricsCollector).computeCPUPercent.
//
// metricsCollectorInstance.prevCPUStats is a PACKAGE-GLOBAL MUTABLE struct
// shared by every concurrent (*ProtocolMonitor).metricsCollector() goroutine
// (one per ProtocolMonitor instance, each on its own 30s ticker calling
// collectRealSystemMetrics -> collectCPUPercent -> computeCPUPercent). The
// pre-fix body has NO mutex guarding this read-modify-write, even though
// sibling ProtocolMetrics methods in this file already serialise their
// shared-state mutations via mu.Lock()/mu.Unlock() (see e.g. lines 183,
// 245, 255, 265, 294, 683). Under concurrent scrapes the
// read-then-write of prevCPUStats can interleave/tear, and even under
// correct synchronisation a torn or otherwise-adversarial sample pair where
// idle grows faster than total makes the uint64 subtraction
// `totalDelta-idleDelta` wrap around to a huge value, so
// `100.0*float64(wrapped)/float64(totalDelta)` reports a wildly wrong
// (typically far above 100%) CPU usage instead of a sane, clamped result.
//
//   - RED_MODE=1 (run against the PRE-FIX computeCPUPercent, with no
//     idleDelta > totalDelta guard): asserts the computed percentage for a
//     synthetic pair where idle grew faster than total is absurd (outside
//     the sane [0,100] range).
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts the
//     FIXED computeCPUPercent produces a sane, clamped-to-[0,100] result for
//     the identical synthetic pair.
func TestSystemMetricsCollector_ComputeCPUPercent_IdleExceedsTotalNoUnderflow(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	// A synthetic pair where idle grows faster than total -- exactly the
	// case where an unguarded uint64 subtraction (totalDelta - idleDelta)
	// wraps around to a value near 2^64.
	prev := cpuStats{idle: 100, total: 1000}
	current := cpuStats{idle: 5000, total: 1010} // idleDelta(4900) > totalDelta(10)

	c := &systemMetricsCollector{}
	// Prime with the first sample: computeCPUPercent's "not yet
	// initialized" branch always returns 0 and records prev -- this is not
	// part of the defect under test, it is how the real collector always
	// starts.
	require.Equal(t, 0.0, c.computeCPUPercent(prev))

	got := c.computeCPUPercent(current)

	if redMode {
		absurd := got < 0 || got > 100
		require.Truef(t, absurd,
			"RED_MODE=1: expected the pre-fix unguarded computation to produce an absurd (outside "+
				"[0,100]) CPU percentage for idleDelta(%d) > totalDelta(%d); got %.4f -- defect did not "+
				"reproduce under this forcing, this is a FINDING not evidence of a fix",
			current.idle-prev.idle, current.total-prev.total, got)
		t.Logf("RED_MODE=1: reproduced the CPU%% underflow -- computeCPUPercent(prev=%+v, current=%+v) = %.4f",
			prev, current, got)
		return
	}

	require.GreaterOrEqualf(t, got, 0.0,
		"RED_MODE=0 (GREEN guard): CPU percentage must be clamped to a sane [0,100] range even when "+
			"idleDelta(%d) > totalDelta(%d); got %.4f",
		current.idle-prev.idle, current.total-prev.total, got)
	require.LessOrEqualf(t, got, 100.0,
		"RED_MODE=0 (GREEN guard): CPU percentage must be clamped to a sane [0,100] range even when "+
			"idleDelta(%d) > totalDelta(%d); got %.4f",
		current.idle-prev.idle, current.total-prev.total, got)
	t.Logf("idle-exceeds-total case OK: computeCPUPercent(prev=%+v, current=%+v) = %.4f (clamped, no underflow)",
		prev, current, got)
}

// TestSystemMetricsCollector_ComputeCPUPercent_ZeroTotalDeltaNoGarbage covers
// the totalDelta == 0 case (repeated sample / stalled /proc/stat counter):
// dividing by zero on an unguarded float64 division silently produces
// +Inf/NaN rather than panicking, which is exactly the kind of
// silent-garbage result the fix must prevent regardless of RED_MODE (the
// pre-fix code already special-cased totalDelta == 0, so this test is a
// standing GREEN guard confirming the fix did not regress it).
func TestSystemMetricsCollector_ComputeCPUPercent_ZeroTotalDeltaNoGarbage(t *testing.T) {
	c := &systemMetricsCollector{}
	same := cpuStats{idle: 500, total: 1000}
	require.Equal(t, 0.0, c.computeCPUPercent(same))

	got := c.computeCPUPercent(same) // totalDelta == 0
	require.Falsef(t, math.IsNaN(got), "computeCPUPercent must not return NaN on totalDelta==0; got %v", got)
	require.Falsef(t, math.IsInf(got, 0), "computeCPUPercent must not return +/-Inf on totalDelta==0; got %v", got)
	require.GreaterOrEqualf(t, got, 0.0, "computeCPUPercent must not return a negative garbage value on totalDelta==0; got %v", got)
	require.LessOrEqualf(t, got, 100.0, "computeCPUPercent must not return an out-of-range garbage value on totalDelta==0; got %v", got)
}

// TestSystemMetricsCollector_ComputeCPUPercent_ConcurrentNoRace hammers a
// single collector from many goroutines simultaneously (run this test with
// `go test -race`) to prove the mutex genuinely serialises the
// read-modify-write of prevCPUStats/initialized. Against the PRE-FIX
// (unlocked) body, `go test -race` reliably reports a DATA RACE for this
// test; against the fixed body it does not.
func TestSystemMetricsCollector_ComputeCPUPercent_ConcurrentNoRace(t *testing.T) {
	c := &systemMetricsCollector{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				stats := cpuStats{
					idle:  uint64(i*1000 + j),
					total: uint64(i*2000 + j*2 + 1),
				}
				got := c.computeCPUPercent(stats)
				if got < 0 || got > 100 {
					t.Errorf("computeCPUPercent produced an out-of-range result under concurrency: %v", got)
				}
			}
		}(i)
	}
	wg.Wait()
}
