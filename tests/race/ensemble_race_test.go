// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"sync"
	"sync/atomic"
	"testing"

	"dev.helix.agent/internal/llm"
)

// TestEnsemble_ConcurrentSetGetMaxConcurrent tests concurrent reads and writes
// to the ensembleMaxConcurrent atomic variable via the public API.
func TestEnsemble_ConcurrentSetGetMaxConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if id%2 == 0 {
					llm.SetMaxConcurrentProviders(id + 1)
				} else {
					_ = llm.GetMaxConcurrentProviders()
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestEnsemble_ConcurrentAtomicCounter exercises an atomic int64 counter
// pattern identical to ensembleMaxConcurrent, verifying no races from
// interleaved Store/Load under high concurrency.
func TestEnsemble_ConcurrentAtomicCounter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	var counter atomic.Int64
	counter.Store(int64(llm.DefaultMaxConcurrentProviders))

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if j%3 == 0 {
					counter.Store(int64(id%10 + 1))
				} else {
					_ = counter.Load()
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestCircuitBreakerManager_ConcurrentRegisterAndStats tests the manager-level
// concurrent map access patterns used during ensemble provider management.
func TestCircuitBreakerManager_ConcurrentRegisterAndStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	mgr := llm.NewDefaultCircuitBreakerManager()

	// Pre-register several providers so readers have something to read.
	for i := 0; i < 5; i++ {
		_ = mgr.Register("provider-static", nil)
	}

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			switch id % 4 {
			case 0:
				mgr.Register("provider-dynamic", nil)
			case 1:
				mgr.Unregister("provider-dynamic")
			case 2:
				_ = mgr.GetAllStats()
			case 3:
				_ = mgr.GetAvailableProviders()
			}
		}(i)
	}

	wg.Wait()
}
