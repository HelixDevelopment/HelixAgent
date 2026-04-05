//go:build stress
// +build stress

package stress

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestStreamingBackpressure verifies that a slow consumer on a channel does
// not cause the producer to OOM. The producer applies backpressure (drops or
// blocks) rather than accumulating unbounded in-flight messages. Heap growth
// is measured before and after to enforce a hard memory ceiling.
func TestStreamingBackpressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		bufferSize      = 64           // small channel buffer to trigger backpressure
		produceDuration = 5 * time.Second
		// OOM threshold: live heap must not exceed 100 MB during the test.
		maxHeapMB = 100.0
	)

	// Force GC and capture initial heap.
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	baselineHeapMB := float64(baseline.HeapInuse) / 1024 / 1024

	ch := make(chan []byte, bufferSize)

	var (
		produced int64
		consumed int64
		dropped  int64
	)

	// Producer: generates 4 KB payloads as fast as possible.
	// Uses non-blocking send so backpressure causes drops rather than OOM.
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		deadline := time.Now().Add(produceDuration)
		for time.Now().Before(deadline) {
			payload := make([]byte, 4*1024) // 4 KB per message
			select {
			case ch <- payload:
				atomic.AddInt64(&produced, 1)
			default:
				// Buffer full — apply backpressure by dropping.
				atomic.AddInt64(&dropped, 1)
			}
			// Tiny yield to avoid spinning a single CPU core at 100%.
			runtime.Gosched()
		}
		close(ch)
	}()

	// Slow consumer: reads from channel with deliberate delay to force backpressure.
	var consumerWg sync.WaitGroup
	consumerWg.Add(1)
	go func() {
		defer consumerWg.Done()
		for msg := range ch {
			_ = msg // consume without retaining
			atomic.AddInt64(&consumed, 1)
			// Slow consumer: 500µs per message.
			time.Sleep(500 * time.Microsecond)
		}
	}()

	// Periodic heap monitor: sample HeapInuse during the test.
	var peakHeapMB float64
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-producerDone:
				return
			case <-ticker.C:
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				heapMB := float64(ms.HeapInuse) / 1024 / 1024
				if heapMB > peakHeapMB {
					peakHeapMB = heapMB
				}
			}
		}
	}()

	// Wait for producer to finish (deadlock detection: 15s).
	select {
	case <-producerDone:
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: streaming backpressure producer did not finish within 15s")
	}

	<-monitorDone

	// Wait for consumer to drain remaining buffered messages.
	consumerDone := make(chan struct{})
	go func() {
		consumerWg.Wait()
		close(consumerDone)
	}()

	select {
	case <-consumerDone:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: slow consumer did not drain after producer closed channel")
	}

	// Final GC + heap measurement.
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	var final runtime.MemStats
	runtime.ReadMemStats(&final)
	finalHeapMB := float64(final.HeapInuse) / 1024 / 1024

	t.Logf("Streaming backpressure: produced=%d, consumed=%d, dropped=%d",
		produced, consumed, dropped)
	t.Logf("Heap: baseline=%.2f MB, peak=%.2f MB, final=%.2f MB",
		baselineHeapMB, peakHeapMB, finalHeapMB)

	// Key assertions:

	// 1. Producer must have produced some messages.
	assert.Greater(t, produced, int64(0), "producer must have sent at least some messages")

	// 2. Consumer must have received at least some messages.
	assert.Greater(t, consumed, int64(0), "consumer must have received at least some messages")

	// 3. Backpressure must have been active (drops > 0 confirms channel was full).
	assert.Greater(t, dropped, int64(0),
		"backpressure must have caused drops — slow consumer should have filled the buffer")

	// 4. OOM prevention: peak heap must stay below maxHeapMB.
	assert.Less(t, peakHeapMB, maxHeapMB,
		"peak heap must stay below %.0f MB — slow consumer must not cause OOM (peak=%.2f MB)",
		maxHeapMB, peakHeapMB)

	// 5. Final heap returns to near-baseline (no persistent retention).
	assert.Less(t, finalHeapMB, baselineHeapMB+50.0,
		"final heap must return close to baseline after consumer drains (final=%.2f MB, baseline=%.2f MB)",
		finalHeapMB, baselineHeapMB)
}
