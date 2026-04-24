package stress

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMemoryLeakDetection tests for memory leaks in common allocation patterns
func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	t.Run("MapGrowthBounded", func(t *testing.T) {
		// Verify maps don't grow unbounded when entries are added and removed
		var baseline runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&baseline)

		m := make(map[int][]byte)

		// Add and remove entries repeatedly
		for round := 0; round < 100; round++ {
			for i := 0; i < 1000; i++ {
				m[i] = make([]byte, 256)
			}
			for i := 0; i < 1000; i++ {
				delete(m, i)
			}
		}

		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		liveHeapMB := float64(after.HeapInuse) / 1024 / 1024
		t.Logf("Live heap after map churn: %.2f MB", liveHeapMB)
		assert.Less(t, liveHeapMB, 100.0, "Map churn should not cause unbounded memory growth")
	})

	t.Run("SliceAppendBounded", func(t *testing.T) {
		// Verify slice append patterns don't leak
		var baseline runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&baseline)

		for round := 0; round < 100; round++ {
			s := make([]byte, 0, 1024)
			for i := 0; i < 10000; i++ {
				s = append(s, byte(i%256))
			}
			_ = s
		}

		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		liveHeapMB := float64(after.HeapInuse) / 1024 / 1024
		t.Logf("Live heap after slice append: %.2f MB", liveHeapMB)
		assert.Less(t, liveHeapMB, 50.0, "Slice append should not leak memory")
	})

	t.Run("GoroutineLocalAllocations", func(t *testing.T) {
		// Verify goroutine-local allocations are cleaned up
		var baseline runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&baseline)

		var wg sync.WaitGroup
		for batch := 0; batch < 10; batch++ {
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// Allocate goroutine-local data
					data := make([]byte, 4096)
					for j := range data {
						data[j] = byte(j % 256)
					}
					time.Sleep(time.Millisecond)
				}()
			}
			wg.Wait()
		}

		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		runtime.GC()

		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		liveHeapMB := float64(after.HeapInuse) / 1024 / 1024
		t.Logf("Live heap after goroutine allocations: %.2f MB", liveHeapMB)
		assert.Less(t, liveHeapMB, 50.0, "Goroutine-local allocations should be GC'd")
	})

	t.Run("ChannelBufferMemory", func(t *testing.T) {
		// Verify channel buffers are properly reclaimed
		var baseline runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&baseline)

		for round := 0; round < 50; round++ {
			ch := make(chan []byte, 100)

			// Fill channel with data
			for i := 0; i < 100; i++ {
				ch <- make([]byte, 1024)
			}

			// Drain channel
			for i := 0; i < 100; i++ {
				<-ch
			}
			close(ch)
		}

		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		liveHeapMB := float64(after.HeapInuse) / 1024 / 1024
		t.Logf("Live heap after channel buffer cycles: %.2f MB", liveHeapMB)
		assert.Less(t, liveHeapMB, 50.0, "Channel buffers should be reclaimed")
	})

	t.Run("ContextValueMemory", func(t *testing.T) {
		// Verify context value chains don't cause memory leaks
		var baseline runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&baseline)

		type ctxKey int

		for round := 0; round < 1000; round++ {
			ctx := context.Background()
			// Build a chain of context values
			for i := 0; i < 50; i++ {
				ctx = context.WithValue(ctx, ctxKey(i), make([]byte, 128))
			}
			// Create cancellable children
			_, cancel := context.WithCancel(ctx)
			cancel()
		}

		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		liveHeapMB := float64(after.HeapInuse) / 1024 / 1024
		t.Logf("Live heap after context value chains: %.2f MB", liveHeapMB)
		assert.Less(t, liveHeapMB, 50.0, "Context value chains should be GC'd after cancel")
	})

	t.Run("ConcurrentMapWithMutex", func(t *testing.T) {
		// Verify concurrent map access with proper synchronization doesn't leak
		var baseline runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&baseline)

		var mu sync.RWMutex
		m := make(map[string][]byte)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		// Writers
		for i := 0; i < 10; i++ {
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
					key := string(rune('A' + id))
					mu.Lock()
					m[key] = make([]byte, 256)
					mu.Unlock()
					counter++
					if counter%100 == 0 {
						// Periodic cleanup
						mu.Lock()
						delete(m, key)
						mu.Unlock()
					}
				}
			}(i)
		}

		// Readers
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}
					key := string(rune('A' + id%10))
					mu.RLock()
					_ = m[key]
					mu.RUnlock()
				}
			}(i)
		}

		wg.Wait()

		// Clear map
		m = nil

		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		liveHeapMB := float64(after.HeapInuse) / 1024 / 1024
		t.Logf("Live heap after concurrent map operations: %.2f MB", liveHeapMB)
		assert.Less(t, liveHeapMB, 100.0, "Concurrent map should not leak memory")
	})
}
