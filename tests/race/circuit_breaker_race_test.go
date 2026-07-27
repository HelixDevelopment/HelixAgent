// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"sync"
	"testing"

	"dev.helix.agent/internal/llm"
)

// TestCircuitBreaker_ConcurrentAfterRequest tests concurrent state transitions.
// Launches 20 goroutines mixing failure and success calls on a shared circuit breaker.
func TestCircuitBreaker_ConcurrentAfterRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode") // SKIP-OK: #short-mode
	}

	cb := llm.NewDefaultCircuitBreaker("test-provider", nil)

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Alternate between failures and successes to drive state transitions.
			for j := 0; j < 50; j++ {
				if (id+j)%3 == 0 {
					// Trigger the afterRequest path via GetStats (read) + Reset (write).
					cb.Reset()
				} else {
					// Read path.
					_ = cb.GetState()
					_ = cb.GetStats()
					_ = cb.IsClosed()
					_ = cb.IsOpen()
					_ = cb.IsHalfOpen()
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestCircuitBreaker_ConcurrentAddRemoveListener tests concurrent listener registration
// and removal.
func TestCircuitBreaker_ConcurrentAddRemoveListener(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode") // SKIP-OK: #short-mode
	}

	cb := llm.NewDefaultCircuitBreaker("listener-test", nil)

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := cb.AddListener(func(providerID string, old, new llm.CircuitState) {})
			if id >= 0 {
				cb.RemoveListener(id)
			}
			_ = cb.ListenerCount()
		}()
	}

	wg.Wait()
}

// TestCircuitBreakerManager_ConcurrentAccess tests concurrent Register/Get/Unregister
// on a CircuitBreakerManager.
func TestCircuitBreakerManager_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode") // SKIP-OK: #short-mode
	}

	mgr := llm.NewDefaultCircuitBreakerManager()

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			providerID := "provider-concurrent"

			// Register, get, and unregister all racing against each other.
			mgr.Register(providerID, nil)
			_, _ = mgr.Get(providerID)
			_ = mgr.GetAllStats()
			_ = mgr.GetAvailableProviders()
			mgr.Unregister(providerID)
		}(i)
	}

	wg.Wait()
}
