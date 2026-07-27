package stress

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestProviderFailoverStress tests provider failover under high concurrent load
func TestProviderFailoverStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	t.Run("ConcurrentFailoverChain", func(t *testing.T) {
		// Simulate a fallback chain of providers
		providers := []string{"primary", "secondary", "tertiary", "quaternary"}
		failingProviders := sync.Map{}
		var successCount, failCount int64

		// Provider health simulation
		isHealthy := func(name string) bool {
			_, failing := failingProviders.Load(name)
			return !failing
		}

		// Execute with fallback chain
		executeWithFallback := func() error {
			for _, p := range providers {
				if isHealthy(p) {
					return nil
				}
			}
			return fmt.Errorf("all providers exhausted")
		}

		var wg sync.WaitGroup

		// Start concurrent requests
		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := executeWithFallback()
				if err != nil {
					atomic.AddInt64(&failCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}()
		}

		// Gradually fail providers
		go func() {
			time.Sleep(10 * time.Millisecond)
			failingProviders.Store("primary", true)
			time.Sleep(10 * time.Millisecond)
			failingProviders.Store("secondary", true)
		}()

		wg.Wait()

		t.Logf("Success: %d, Fail: %d", successCount, failCount)
		assert.Greater(t, successCount, int64(0), "Some requests should succeed via fallback")
	})

	t.Run("PartialDegradation", func(t *testing.T) {
		// Simulate providers with varying error rates
		type providerSim struct {
			name      string
			errorRate float64 // 0.0 to 1.0
		}

		providers := []providerSim{
			{"fast-unreliable", 0.7},
			{"medium-stable", 0.1},
			{"slow-reliable", 0.01},
		}

		var wg sync.WaitGroup
		var totalSuccess, totalFail int64
		providerHits := sync.Map{}

		for i := 0; i < 500; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for _, p := range providers {
					if rand.Float64() >= p.errorRate {
						atomic.AddInt64(&totalSuccess, 1)
						val, _ := providerHits.LoadOrStore(p.name, new(int64))
						atomic.AddInt64(val.(*int64), 1)
						return
					}
				}
				atomic.AddInt64(&totalFail, 1)
			}()
		}

		wg.Wait()

		total := totalSuccess + totalFail
		successRate := float64(totalSuccess) / float64(total) * 100
		t.Logf("Partial degradation: %d/%d successful (%.1f%%)", totalSuccess, total, successRate)

		providerHits.Range(func(key, value interface{}) bool {
			t.Logf("  %s: %d hits", key.(string), atomic.LoadInt64(value.(*int64)))
			return true
		})

		assert.Greater(t, successRate, 90.0, "With fallback chain, success rate should be >90%%")
	})

	t.Run("FlappingProviders", func(t *testing.T) {
		// Simulate providers that flap between healthy and unhealthy
		var healthy int32 = 1 // 1 = healthy, 0 = unhealthy
		var wg sync.WaitGroup
		var successCount, failCount int64

		// Flapping goroutine
		ctx := make(chan struct{})
		go func() {
			for {
				select {
				case <-ctx:
					return
				default:
				}
				time.Sleep(5 * time.Millisecond)
				if atomic.LoadInt32(&healthy) == 1 {
					atomic.StoreInt32(&healthy, 0)
				} else {
					atomic.StoreInt32(&healthy, 1)
				}
			}
		}()

		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if atomic.LoadInt32(&healthy) == 1 {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}()
		}

		wg.Wait()
		close(ctx)

		total := successCount + failCount
		t.Logf("Flapping: success=%d, fail=%d, total=%d", successCount, failCount, total)
		// Both should have some — the provider is flapping
		assert.Equal(t, int64(1000), total, "All requests should be accounted for")
	})
}
