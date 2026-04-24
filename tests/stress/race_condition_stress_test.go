package stress

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRaceConditionPatterns tests patterns that commonly cause race conditions
// Run with: go test -race -run TestRaceConditionPatterns ./tests/stress/
func TestRaceConditionPatterns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	t.Run("AtomicCounterConsistency", func(t *testing.T) {
		var counter int64
		iterations := 10000

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					atomic.AddInt64(&counter, 1)
				}
			}()
		}

		wg.Wait()

		expected := int64(100 * iterations)
		assert.Equal(t, expected, counter, "Atomic counter should be exact")
	})

	t.Run("SyncMapConcurrency", func(t *testing.T) {
		var m sync.Map
		var writeOps, readOps int64

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		// Concurrent writers
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				counter := 0
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}
					m.Store(id, counter)
					counter++
					atomic.AddInt64(&writeOps, 1)
				}
			}(i)
		}

		// Concurrent readers
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
					m.Load(id % 20)
					atomic.AddInt64(&readOps, 1)
				}
			}(i)
		}

		// Concurrent deleters
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}
					m.Delete(id)
					time.Sleep(time.Millisecond)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Write ops: %d, Read ops: %d", writeOps, readOps)
		assert.Greater(t, writeOps, int64(100), "Should complete many writes")
		assert.Greater(t, readOps, int64(100), "Should complete many reads")
	})

	t.Run("OnceValueConsistency", func(t *testing.T) {
		// Test that sync.Once guarantees single initialization across goroutines
		var initCount int64
		var once sync.Once
		var value int

		var wg sync.WaitGroup
		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				once.Do(func() {
					atomic.AddInt64(&initCount, 1)
					value = 42
				})
				assert.Equal(t, 42, value, "Value should always be 42 after Once.Do")
			}()
		}

		wg.Wait()

		assert.Equal(t, int64(1), initCount, "Init should run exactly once")
	})

	t.Run("MutexProtectedSlice", func(t *testing.T) {
		var mu sync.Mutex
		data := make([]int, 0, 1000)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					mu.Lock()
					data = append(data, val)
					mu.Unlock()
				}
			}(i)
		}

		wg.Wait()

		assert.Equal(t, 1000, len(data), "All appends should be accounted for")
	})

	t.Run("CondBroadcastWakeup", func(t *testing.T) {
		var mu sync.Mutex
		cond := sync.NewCond(&mu)
		ready := false
		var wokenUp int64

		var wg sync.WaitGroup

		// Waiters
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				mu.Lock()
				for !ready {
					cond.Wait()
				}
				atomic.AddInt64(&wokenUp, 1)
				mu.Unlock()
			}()
		}

		// Give waiters time to start
		time.Sleep(50 * time.Millisecond)

		// Broadcast
		mu.Lock()
		ready = true
		mu.Unlock()
		cond.Broadcast()

		wg.Wait()

		assert.Equal(t, int64(50), wokenUp, "All waiters should be woken up by Broadcast")
	})

	t.Run("WaitGroupReuse", func(t *testing.T) {
		// Test that WaitGroup can be safely reused after Wait returns
		var totalOps int64

		for round := 0; round < 100; round++ {
			var wg sync.WaitGroup
			for i := 0; i < 50; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					atomic.AddInt64(&totalOps, 1)
				}()
			}
			wg.Wait()
		}

		assert.Equal(t, int64(5000), totalOps, "All operations should complete")
	})

	t.Run("ChannelFanOutFanIn", func(t *testing.T) {
		// Fan-out fan-in pattern — common source of races if implemented wrong
		input := make(chan int, 100)
		results := make(chan int, 100)

		// Fan out to workers
		var wg sync.WaitGroup
		numWorkers := 10
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for val := range input {
					results <- val * 2
				}
			}()
		}

		// Send work
		numItems := 1000
		go func() {
			for i := 0; i < numItems; i++ {
				input <- i
			}
			close(input)
		}()

		// Close results after workers done
		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect results
		var count int
		for range results {
			count++
		}

		assert.Equal(t, numItems, count, "All items should be processed through fan-out/fan-in")
	})
}
