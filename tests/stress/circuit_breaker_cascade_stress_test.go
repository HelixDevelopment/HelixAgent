//go:build stress
// +build stress

package stress

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"digital.vasic.concurrency/pkg/breaker"
)

// TestCircuitBreakerCascade verifies that sequential failures trip each
// circuit breaker independently, and that recovery cascades correctly once
// the timeout elapses. Tests the numeric thresholds:
//   - All N breakers transition to Open after MaxFailures failures each.
//   - After recovery timeout, all breakers transition to HalfOpen.
//   - Successful probe calls close each breaker independently.
func TestCircuitBreakerCascade(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		breakerCount   = 10
		maxFailures    = 5
		recoveryTimeout = 200 * time.Millisecond
	)

	// Create N independent circuit breakers — one per "provider".
	breakers := make([]*breaker.CircuitBreaker, breakerCount)
	for i := 0; i < breakerCount; i++ {
		breakers[i] = breaker.New(&breaker.Config{
			MaxFailures:      maxFailures,
			Timeout:          recoveryTimeout,
			HalfOpenRequests: 1,
		})
	}

	goroutinesBefore := runtime.NumGoroutine()

	// --- Phase 1: Trip each breaker with sequential failures ---
	// Each breaker receives exactly MaxFailures failures to guarantee it opens.
	var totalFailures int64
	for i, cb := range breakers {
		for f := 0; f < maxFailures; f++ {
			err := cb.Execute(func() error {
				return fmt.Errorf("provider-%d: simulated failure %d", i, f)
			})
			if err != nil {
				atomic.AddInt64(&totalFailures, 1)
			}
		}
	}

	// Verify: all breakers must be Open after their respective failure storms.
	openCount := 0
	for i, cb := range breakers {
		state := cb.State()
		if state == breaker.Open {
			openCount++
		} else {
			t.Logf("Breaker %d unexpectedly in state %s after %d failures", i, state, maxFailures)
		}
	}
	assert.Equal(t, breakerCount, openCount,
		"all %d circuit breakers must be Open after %d sequential failures each",
		breakerCount, maxFailures)

	t.Logf("Phase 1 complete: %d/%d breakers opened, totalFailures=%d",
		openCount, breakerCount, totalFailures)

	// --- Phase 2: Wait for recovery timeout ---
	// After the timeout, each breaker transitions to HalfOpen on the next call.
	time.Sleep(recoveryTimeout + 50*time.Millisecond)

	// --- Phase 3: Probe recovery — successful calls close each breaker ---
	var closedAfterRecovery int64
	var panics int64

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt64(&panics, 1)
			}
		}()

		for i, cb := range breakers {
			// One successful probe call should close the breaker (HalfOpenRequests=1).
			err := cb.Execute(func() error {
				return nil // success
			})
			state := cb.State()
			if err == nil && state == breaker.Closed {
				atomic.AddInt64(&closedAfterRecovery, 1)
			} else {
				t.Logf("Breaker %d: probe err=%v, state=%s", i, err, state)
			}
		}
	}()

	// Deadlock detection for recovery phase.
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("DEADLOCK DETECTED: circuit breaker cascade recovery timed out")
	}

	assert.Zero(t, panics, "no panics during circuit breaker cascade recovery")

	// Numeric threshold: all breakers must recover to Closed after one successful probe.
	assert.Equal(t, int64(breakerCount), closedAfterRecovery,
		"all %d circuit breakers must recover to Closed after successful probe (got %d)",
		breakerCount, closedAfterRecovery)

	t.Logf("Phase 3 complete: %d/%d breakers recovered to Closed",
		closedAfterRecovery, breakerCount)

	// --- Phase 4: Verify breakers are independently closed (not cross-contaminated) ---
	finalOpenCount := 0
	for _, cb := range breakers {
		if cb.State() != breaker.Closed {
			finalOpenCount++
		}
	}
	assert.Zero(t, finalOpenCount,
		"all circuit breakers must be Closed after cascade recovery (still open: %d)", finalOpenCount)

	// Goroutine leak check.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 10, "no goroutine leak during circuit breaker cascade test")

	t.Logf("Circuit breaker cascade: breakers=%d, opened=%d, recovered=%d, panics=%d",
		breakerCount, openCount, closedAfterRecovery, panics)
}
