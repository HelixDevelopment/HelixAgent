package stress

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDeadlockDetectionStress tests common deadlock patterns under stress
func TestDeadlockDetectionStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	t.Run("MutexOrderingStress", func(t *testing.T) {
		// Test that consistent mutex ordering prevents deadlocks
		var mu1, mu2 sync.Mutex
		var completed int64

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}
					// Always acquire in same order to avoid deadlock
					mu1.Lock()
					mu2.Lock()
					atomic.AddInt64(&completed, 1)
					mu2.Unlock()
					mu1.Unlock()
				}
			}()
		}

		wg.Wait()

		t.Logf("Completed %d lock cycles without deadlock", completed)
		assert.Greater(t, completed, int64(100), "Should complete many lock cycles")
	})

	t.Run("ChannelDeadlockAvoidance", func(t *testing.T) {
		// Test that select with default prevents channel deadlocks
		var completed int64

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		ch1 := make(chan int, 1)
		ch2 := make(chan int, 1)

		var wg sync.WaitGroup

		// Writer 1: tries ch1 then ch2
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case ch1 <- 1:
					atomic.AddInt64(&completed, 1)
				default:
				}
				select {
				case <-ctx.Done():
					return
				case ch2 <- 2:
					atomic.AddInt64(&completed, 1)
				default:
				}
			}
		}()

		// Reader: drains both channels
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ch1:
				case <-ch2:
				}
			}
		}()

		wg.Wait()

		t.Logf("Completed %d channel operations without deadlock", completed)
		assert.Greater(t, completed, int64(10), "Should complete channel operations")
	})

	t.Run("RWMutexStarvationResistance", func(t *testing.T) {
		// Test that RWMutex doesn't starve writers under heavy read load
		var mu sync.RWMutex
		var readOps, writeOps int64

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		// Many readers
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}
					mu.RLock()
					atomic.AddInt64(&readOps, 1)
					mu.RUnlock()
				}
			}()
		}

		// Few writers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}
					mu.Lock()
					atomic.AddInt64(&writeOps, 1)
					mu.Unlock()
					time.Sleep(100 * time.Microsecond)
				}
			}()
		}

		wg.Wait()

		t.Logf("Read ops: %d, Write ops: %d", readOps, writeOps)
		assert.Greater(t, writeOps, int64(10), "Writers should not be starved")
		assert.Greater(t, readOps, int64(100), "Readers should proceed normally")
	})

	t.Run("NestedContextCancellation", func(t *testing.T) {
		// Test that nested context cancellation completes cleanly
		baseline := runtime.NumGoroutine()
		var completed int64

		for round := 0; round < 50; round++ {
			parent, parentCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)

			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				child, childCancel := context.WithCancel(parent)
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer childCancel()
					select {
					case <-child.Done():
						atomic.AddInt64(&completed, 1)
					}
				}()
			}

			wg.Wait()
			parentCancel()
		}

		time.Sleep(200 * time.Millisecond)
		runtime.GC()

		current := runtime.NumGoroutine()
		leaked := current - baseline
		t.Logf("Nested context: completed=%d, goroutine diff=%d", completed, leaked)
		assert.LessOrEqual(t, leaked, 5, "Should not leak goroutines from nested contexts")
	})

	t.Run("ProducerConsumerDeadlockFree", func(t *testing.T) {
		// Multiple producers and consumers with bounded buffer
		bufferSize := 10
		ch := make(chan int, bufferSize)
		var produced, consumed int64

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		// Producers
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				counter := 0
				for {
					select {
					case <-ctx.Done():
						return
					case ch <- counter:
						counter++
						atomic.AddInt64(&produced, 1)
					}
				}
			}(i)
		}

		// Consumers
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ch:
						atomic.AddInt64(&consumed, 1)
					}
				}
			}()
		}

		wg.Wait()

		t.Logf("Produced: %d, Consumed: %d", produced, consumed)
		assert.Greater(t, produced, int64(100), "Should produce many messages")
		assert.Greater(t, consumed, int64(100), "Should consume many messages")
	})
}
