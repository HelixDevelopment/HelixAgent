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

// TestStreamingBackpressureStress tests streaming with slow consumers and fast producers
func TestStreamingBackpressureStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	t.Run("SlowConsumerFastProducer", func(t *testing.T) {
		bufferSize := 100
		ch := make(chan string, bufferSize)
		var produced, consumed, dropped int64

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Fast producer
		go func() {
			defer close(ch)
			for i := 0; ; i++ {
				select {
				case <-ctx.Done():
					return
				case ch <- "data":
					atomic.AddInt64(&produced, 1)
				default:
					// Buffer full, drop
					atomic.AddInt64(&dropped, 1)
				}
			}
		}()

		// Slow consumer
		for msg := range ch {
			_ = msg
			atomic.AddInt64(&consumed, 1)
			time.Sleep(100 * time.Microsecond) // Slow consumption
		}

		t.Logf("Produced: %d, Consumed: %d, Dropped: %d", produced, consumed, dropped)
		assert.Greater(t, consumed, int64(0), "Should consume some messages")
	})

	t.Run("MultipleSlowConsumers", func(t *testing.T) {
		numConsumers := 10
		ch := make(chan int, 1000)
		var totalConsumed int64

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Producer
		go func() {
			defer close(ch)
			for i := 0; ; i++ {
				select {
				case <-ctx.Done():
					return
				case ch <- i:
				}
			}
		}()

		var wg sync.WaitGroup
		perConsumer := make([]int64, numConsumers)

		for i := 0; i < numConsumers; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for range ch {
					perConsumer[id]++
					atomic.AddInt64(&totalConsumed, 1)
					// Variable processing speed
					time.Sleep(time.Duration(id+1) * 50 * time.Microsecond)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Total consumed: %d", totalConsumed)
		for i, count := range perConsumer {
			t.Logf("  Consumer %d: %d messages", i, count)
		}

		assert.Greater(t, totalConsumed, int64(100), "Should consume substantial messages")
	})

	t.Run("BurstTraffic", func(t *testing.T) {
		ch := make(chan struct{}, 50) // Small buffer
		var bursts int64
		var handled int64

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Burst producer: sends bursts of 100, then pauses
		go func() {
			for {
				select {
				case <-ctx.Done():
					close(ch)
					return
				default:
				}

				// Send burst
				for i := 0; i < 100; i++ {
					select {
					case ch <- struct{}{}:
						atomic.AddInt64(&bursts, 1)
					case <-ctx.Done():
						close(ch)
						return
					default:
						// backpressure
					}
				}
				time.Sleep(50 * time.Millisecond) // Pause between bursts
			}
		}()

		// Consumer
		for range ch {
			atomic.AddInt64(&handled, 1)
			time.Sleep(time.Millisecond)
		}

		t.Logf("Burst sent: %d, Handled: %d", bursts, handled)
		assert.Greater(t, handled, int64(0), "Should handle burst traffic")
	})

	t.Run("MemoryUnderBackpressure", func(t *testing.T) {
		var baseline runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&baseline)

		// Create channels to simulate backpressure queues
		numChannels := 20
		channels := make([]chan int, numChannels)
		for i := range channels {
			channels[i] = make(chan int, 50)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		for _, ch := range channels {
			wg.Add(1)
			go func(c chan int) {
				defer wg.Done()
				counter := 0
				for {
					select {
					case <-ctx.Done():
						return
					case c <- counter:
						counter++
					default:
						select {
						case <-c:
						default:
						}
					}
				}
			}(ch)
		}

		wg.Wait()

		for _, ch := range channels {
			close(ch)
			for range ch {
			}
		}

		var final runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&final)

		// Check live heap — after GC, memory should be bounded
		liveHeapMB := float64(final.HeapInuse) / 1024 / 1024
		t.Logf("Live heap after backpressure test: %.2f MB", liveHeapMB)
		assert.Less(t, liveHeapMB, 200.0, "Live heap under backpressure should be bounded")
	})
}
