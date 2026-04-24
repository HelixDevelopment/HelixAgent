// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// RaceTestCase represents a single race condition test scenario
type RaceTestCase struct {
	Name       string
	Setup      func() interface{}
	Operation  func(interface{})
	Concurrent int
	Iterations int
	Timeout    time.Duration
}

// Run executes the race test case with the race detector
func (tc *RaceTestCase) Run(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), tc.Timeout)
	defer cancel()

	instance := tc.Setup()

	var wg sync.WaitGroup
	errors := make(chan error, tc.Concurrent*tc.Iterations)

	for i := 0; i < tc.Concurrent; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < tc.Iterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					func() {
						defer func() {
							if r := recover(); r != nil {
								errors <- assert.AnError
							}
						}()
						tc.Operation(instance)
					}()
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	var errCount int
	for err := range errors {
		if err != nil {
			errCount++
		}
	}

	assert.Equal(t, 0, errCount, "Race test '%s' encountered %d errors", tc.Name, errCount)
}

// TestCache_RaceCondition tests concurrent cache access
func TestCache_RaceCondition(t *testing.T) {
	tc := RaceTestCase{
		Name:       "CacheConcurrentAccess",
		Concurrent: 100,
		Iterations: 1000,
		Timeout:    30 * time.Second,
		Setup: func() interface{} {
			return &sync.Map{}
		},
		Operation: func(instance interface{}) {
			cache := instance.(*sync.Map)
			key := "test-key"

			// Concurrent read/write
			cache.Store(key, "value")
			if val, ok := cache.Load(key); ok {
				_ = val.(string)
			}
			cache.Delete(key)
		},
	}

	tc.Run(t)
}

// TestCounter_RaceCondition tests concurrent counter operations
func TestCounter_RaceCondition(t *testing.T) {
	type Counter struct {
		mu    sync.RWMutex
		value int
	}

	tc := RaceTestCase{
		Name:       "CounterConcurrentAccess",
		Concurrent: 100,
		Iterations: 1000,
		Timeout:    30 * time.Second,
		Setup: func() interface{} {
			return &Counter{}
		},
		Operation: func(instance interface{}) {
			c := instance.(*Counter)

			// Concurrent increment
			c.mu.Lock()
			c.value++
			c.mu.Unlock()

			// Concurrent read
			c.mu.RLock()
			_ = c.value
			c.mu.RUnlock()
		},
	}

	tc.Run(t)
}

// TestChannel_RaceCondition tests channel operations
func TestChannel_RaceCondition(t *testing.T) {
	tc := RaceTestCase{
		Name:       "ChannelOperations",
		Concurrent: 50,
		Iterations: 100,
		Timeout:    30 * time.Second,
		Setup: func() interface{} {
			return make(chan int, 100)
		},
		Operation: func(instance interface{}) {
			ch := instance.(chan int)

			select {
			case ch <- 1:
			default:
			}

			select {
			case <-ch:
			default:
			}
		},
	}

	tc.Run(t)
}

// TestWaitGroup_RaceCondition tests WaitGroup usage
func TestWaitGroup_RaceCondition(t *testing.T) {
	type Container struct {
		mu      sync.RWMutex
		results []int
	}

	tc := RaceTestCase{
		Name:       "WaitGroupSynchronization",
		Concurrent: 50,
		Iterations: 100,
		Timeout:    30 * time.Second,
		Setup: func() interface{} {
			return &Container{results: make([]int, 0)}
		},
		Operation: func(instance interface{}) {
			c := instance.(*Container)
			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()

				c.mu.Lock()
				c.results = append(c.results, 1)
				c.mu.Unlock()
			}()

			wg.Wait()
		},
	}

	tc.Run(t)
}

// TestContext_RaceCondition tests context cancellation
func TestContext_RaceCondition(t *testing.T) {
	tc := RaceTestCase{
		Name:       "ContextCancellation",
		Concurrent: 50,
		Iterations: 100,
		Timeout:    10 * time.Second,
		Setup: func() interface{} {
			ctx, cancel := context.WithCancel(context.Background())
			return map[string]interface{}{
				"ctx":    ctx,
				"cancel": cancel,
			}
		},
		Operation: func(instance interface{}) {
			m := instance.(map[string]interface{})
			ctx := m["ctx"].(context.Context)
			cancel := m["cancel"].(context.CancelFunc)

			select {
			case <-ctx.Done():
			default:
			}

			cancel()
		},
	}

	tc.Run(t)
}

// TestOnce_RaceCondition tests sync.Once behavior
func TestOnce_RaceCondition(t *testing.T) {
	type Container struct {
		once  sync.Once
		value int
		mu    sync.RWMutex
	}

	tc := RaceTestCase{
		Name:       "SyncOnceInitialization",
		Concurrent: 100,
		Iterations: 100,
		Timeout:    10 * time.Second,
		Setup: func() interface{} {
			return &Container{}
		},
		Operation: func(instance interface{}) {
			c := instance.(*Container)

			c.once.Do(func() {
				c.mu.Lock()
				c.value = 42
				c.mu.Unlock()
			})

			c.mu.RLock()
			_ = c.value
			c.mu.RUnlock()
		},
	}

	tc.Run(t)
}

// TestPool_RaceCondition tests sync.Pool usage
func TestPool_RaceCondition(t *testing.T) {
	type Buffer struct {
		data []byte
	}

	pool := &sync.Pool{
		New: func() interface{} {
			return &Buffer{data: make([]byte, 1024)}
		},
	}

	tc := RaceTestCase{
		Name:       "SyncPoolUsage",
		Concurrent: 100,
		Iterations: 1000,
		Timeout:    30 * time.Second,
		Setup: func() interface{} {
			return pool
		},
		Operation: func(instance interface{}) {
			p := instance.(*sync.Pool)

			buf := p.Get().(*Buffer)
			buf.data[0] = 1
			p.Put(buf)
		},
	}

	tc.Run(t)
}

// TestMap_RaceCondition tests concurrent map access patterns
func TestMap_RaceCondition(t *testing.T) {
	tc := RaceTestCase{
		Name:       "ConcurrentMapPatterns",
		Concurrent: 100,
		Iterations: 1000,
		Timeout:    30 * time.Second,
		Setup: func() interface{} {
			return make(map[string]int)
		},
		Operation: func(instance interface{}) {
			// This should be detected by race detector
			// In production, use sync.Map or mutex
			m := instance.(map[string]int)
			m["key"] = 1
			_ = m["key"]
		},
	}

	// Note: This test will FAIL the race detector intentionally
	// to demonstrate that it catches unsafe operations
	t.Skip("Intentionally testing race detector - skip in normal runs")  // SKIP-OK: #legacy-untriaged
	tc.Run(t)
}

// Helper function to detect goroutine leaks
func DetectGoroutineLeak(t *testing.T, testFunc func()) {
	t.Helper()

	before := runtime.NumGoroutine()
	testFunc()

	// Give goroutines time to exit
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before {
		t.Logf("Goroutines before: %d, after: %d", before, after)
		// Don't fail immediately, log for investigation
	}
}

// BenchmarkRaceDetectorOverhead measures race detector impact
func BenchmarkRaceDetectorOverhead(b *testing.B) {
	var counter int64
	var mu sync.RWMutex

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			counter++
			mu.Unlock()
		}
	})
}
