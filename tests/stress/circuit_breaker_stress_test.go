package stress

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/services"
)

// TestCircuitBreakerCascadeStress tests circuit breaker behavior under cascading failures
func TestCircuitBreakerCascadeStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	t.Run("ConcurrentTrips", func(t *testing.T) {
		cb := services.NewCircuitBreaker(5, 2, 100*time.Millisecond)

		var wg sync.WaitGroup
		var failures int64
		var blocked int64

		// Hammer the circuit breaker with concurrent failures
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					err := cb.Call(func() error {
						return fmt.Errorf("simulated failure")
					})
					if err != nil {
						if err.Error() == "circuit breaker is open" {
							atomic.AddInt64(&blocked, 1)
						} else {
							atomic.AddInt64(&failures, 1)
						}
					}
				}
			}()
		}

		wg.Wait()

		state := cb.GetState()
		t.Logf("Circuit state after %d failures (%d blocked): %s", failures, blocked, state)
		assert.Equal(t, services.StateOpen, state, "Circuit should be open after cascading failures")
	})

	t.Run("RecoveryUnderLoad", func(t *testing.T) {
		cb := services.NewCircuitBreaker(3, 2, 50*time.Millisecond)

		// Trip the circuit
		for i := 0; i < 5; i++ {
			_ = cb.Call(func() error {
				return fmt.Errorf("failure")
			})
		}
		require.Equal(t, services.StateOpen, cb.GetState())

		// Wait for recovery timeout
		time.Sleep(60 * time.Millisecond)

		// Send concurrent success signals
		var wg sync.WaitGroup
		var successes int64

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := cb.Call(func() error {
					return nil // success
				})
				if err == nil {
					atomic.AddInt64(&successes, 1)
				}
			}()
		}

		wg.Wait()
		t.Logf("Successes during recovery: %d, final state: %s", successes, cb.GetState())
		assert.Greater(t, successes, int64(0), "Should allow some requests through during recovery")
	})

	t.Run("MultipleProviderCascade", func(t *testing.T) {
		providerNames := []string{"provider-a", "provider-b", "provider-c", "provider-d", "provider-e"}
		breakers := make(map[string]*services.CircuitBreaker)

		for _, p := range providerNames {
			_ = p
			breakers[p] = services.NewCircuitBreaker(3, 2, 200*time.Millisecond)
		}

		var wg sync.WaitGroup
		var totalBlocked int64

		// Simulate cascade: when one provider fails, load shifts to others
		for round := 0; round < 5; round++ {
			failingProvider := providerNames[round%len(providerNames)]

			for _, p := range providerNames {
				wg.Add(1)
				go func(name string, shouldFail bool) {
					defer wg.Done()
					cb := breakers[name]

					for i := 0; i < 10; i++ {
						err := cb.Call(func() error {
							if shouldFail {
								return fmt.Errorf("provider %s failed", name)
							}
							return nil
						})
						if err != nil && err.Error() == "circuit breaker is open" {
							atomic.AddInt64(&totalBlocked, 1)
						}
					}
				}(p, p == failingProvider)
			}
		}

		wg.Wait()
		t.Logf("Total blocked requests across cascade: %d", totalBlocked)

		openCircuits := 0
		for _, cb := range breakers {
			if cb.GetState() == services.StateOpen {
				openCircuits++
			}
		}
		t.Logf("Open circuits after cascade: %d/%d", openCircuits, len(providerNames))
	})

	t.Run("RapidStateTransitions", func(t *testing.T) {
		cb := services.NewCircuitBreaker(2, 1, 10*time.Millisecond)

		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var transitions int64

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				lastState := cb.GetState()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}

					if id%2 == 0 {
						_ = cb.Call(func() error {
							return fmt.Errorf("fail")
						})
					} else {
						_ = cb.Call(func() error {
							return nil
						})
					}

					currentState := cb.GetState()
					if currentState != lastState {
						atomic.AddInt64(&transitions, 1)
						lastState = currentState
					}
					time.Sleep(time.Millisecond)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("State transitions observed: %d", transitions)
		assert.Greater(t, transitions, int64(0), "Should observe state transitions")
	})
}
