// Previously tests/stress/... — demoted to unit package under CONST-030.
// Stress tests of in-process concurrency primitives (semaphore / pool /
// cache saturation, circuit-breaker storms, goroutine growth) that
// exercise local mock providers and invokers. Keeping the mocks in the
// unit tree per the audit's Pattern-4 disposition for stress tests.
// PR33 of the CONST-030 compliance campaign.
package stress_legacy_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMCPAdapterStress tests MCP adapter registry under concurrent load
func TestMCPAdapterStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	t.Run("ConcurrentRegistration", func(t *testing.T) {
		registry := &mockAdapterRegistry{
			adapters: make(map[string]bool),
		}

		var wg sync.WaitGroup
		var registered int64

		// Register 1000 adapters concurrently
		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				name := adapterName(id)
				registry.Register(name)
				atomic.AddInt64(&registered, 1)
			}(i)
		}

		wg.Wait()

		count := registry.Count()
		t.Logf("Registered %d adapters, registry has %d", registered, count)
		assert.Equal(t, 1000, count, "All adapters should be registered")
	})

	t.Run("ConcurrentLookupDuringRegistration", func(t *testing.T) {
		registry := &mockAdapterRegistry{
			adapters: make(map[string]bool),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		var lookups, hits, misses int64

		// Writers: register adapters
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; ; j++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					registry.Register(adapterName(id*1000 + j))
					time.Sleep(time.Millisecond)
				}
			}(i)
		}

		// Readers: lookup adapters concurrently
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}
					name := adapterName(id * 100)
					if registry.Exists(name) {
						atomic.AddInt64(&hits, 1)
					} else {
						atomic.AddInt64(&misses, 1)
					}
					atomic.AddInt64(&lookups, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Lookups: %d (hits: %d, misses: %d)", lookups, hits, misses)
		assert.Greater(t, lookups, int64(100), "Should perform many lookups")
	})

	t.Run("ToolExecutionConcurrency", func(t *testing.T) {
		// Simulate concurrent tool executions across multiple adapters
		numAdapters := 10
		numToolsPerAdapter := 5

		var wg sync.WaitGroup
		var totalExecutions int64
		var totalErrors int64

		for a := 0; a < numAdapters; a++ {
			for tool := 0; tool < numToolsPerAdapter; tool++ {
				wg.Add(1)
				go func(adapterID, toolID int) {
					defer wg.Done()
					// Simulate tool execution
					for i := 0; i < 100; i++ {
						time.Sleep(10 * time.Microsecond)
						atomic.AddInt64(&totalExecutions, 1)
					}
				}(a, tool)
			}
		}

		wg.Wait()

		expectedTotal := int64(numAdapters * numToolsPerAdapter * 100)
		t.Logf("Tool executions: %d (expected: %d), errors: %d", totalExecutions, expectedTotal, totalErrors)
		assert.Equal(t, expectedTotal, totalExecutions, "All tool executions should complete")
		assert.Equal(t, int64(0), totalErrors, "No errors expected")
	})

	t.Run("AdapterHealthCheckFlood", func(t *testing.T) {
		numAdapters := 20
		var wg sync.WaitGroup
		var checks int64

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		for i := 0; i < numAdapters; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}
					// Simulate health check
					time.Sleep(100 * time.Microsecond)
					atomic.AddInt64(&checks, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Health checks performed: %d across %d adapters", checks, numAdapters)
		assert.Greater(t, checks, int64(100), "Should perform many health checks")
	})
}

// mockAdapterRegistry is a thread-safe adapter registry for stress testing
type mockAdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[string]bool
}

func (r *mockAdapterRegistry) Register(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[name] = true
}

func (r *mockAdapterRegistry) Exists(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[name]
}

func (r *mockAdapterRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.adapters)
}

func adapterName(id int) string {
	return fmt.Sprintf("adapter-%d", id)
}
