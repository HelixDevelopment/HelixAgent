package stress

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGoroutineLeakDetection tests for goroutine leaks in common patterns
func TestGoroutineLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	t.Run("ChannelCleanup", func(t *testing.T) {
		baseline := runtime.NumGoroutine()

		// Create and properly close many channels
		for i := 0; i < 100; i++ {
			ch := make(chan struct{})
			go func() {
				<-ch
			}()
			close(ch)
		}

		// Allow goroutines to finish
		time.Sleep(100 * time.Millisecond)
		runtime.GC()

		current := runtime.NumGoroutine()
		leaked := current - baseline
		t.Logf("Goroutines: baseline=%d, current=%d, diff=%d", baseline, current, leaked)
		assert.LessOrEqual(t, leaked, 5, "Should not leak goroutines from channel operations")
	})

	t.Run("WaitGroupCleanup", func(t *testing.T) {
		baseline := runtime.NumGoroutine()

		for round := 0; round < 50; round++ {
			var wg sync.WaitGroup
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					time.Sleep(time.Millisecond)
				}()
			}
			wg.Wait()
		}

		time.Sleep(100 * time.Millisecond)
		runtime.GC()

		current := runtime.NumGoroutine()
		leaked := current - baseline
		t.Logf("Goroutines: baseline=%d, current=%d, diff=%d", baseline, current, leaked)
		assert.LessOrEqual(t, leaked, 5, "Should not leak goroutines from WaitGroup patterns")
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		baseline := runtime.NumGoroutine()

		for round := 0; round < 100; round++ {
			done := make(chan struct{})
			go func() {
				select {
				case <-done:
					return
				case <-time.After(10 * time.Second):
					return
				}
			}()
			close(done)
		}

		time.Sleep(100 * time.Millisecond)
		runtime.GC()

		current := runtime.NumGoroutine()
		leaked := current - baseline
		t.Logf("Goroutines: baseline=%d, current=%d, diff=%d", baseline, current, leaked)
		assert.LessOrEqual(t, leaked, 5, "Should not leak goroutines from context cancellation")
	})

	t.Run("TimerCleanup", func(t *testing.T) {
		baseline := runtime.NumGoroutine()

		for i := 0; i < 200; i++ {
			timer := time.NewTimer(time.Hour) // Long timer
			timer.Stop()                      // Must stop to prevent leak
		}

		time.Sleep(100 * time.Millisecond)
		runtime.GC()

		current := runtime.NumGoroutine()
		leaked := current - baseline
		t.Logf("Goroutines: baseline=%d, current=%d, diff=%d", baseline, current, leaked)
		assert.LessOrEqual(t, leaked, 5, "Should not leak goroutines from timer operations")
	})

	t.Run("HighGoroutineChurn", func(t *testing.T) {
		baseline := runtime.NumGoroutine()

		// Create and complete many short-lived goroutines
		for batch := 0; batch < 10; batch++ {
			var wg sync.WaitGroup
			for i := 0; i < 500; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// Simulate brief work
					_ = make([]byte, 1024)
				}()
			}
			wg.Wait()
		}

		time.Sleep(200 * time.Millisecond)
		runtime.GC()

		current := runtime.NumGoroutine()
		leaked := current - baseline
		t.Logf("After 5000 goroutines: baseline=%d, current=%d, diff=%d", baseline, current, leaked)
		assert.LessOrEqual(t, leaked, 10, "Should not leak goroutines after high churn")
	})
}
