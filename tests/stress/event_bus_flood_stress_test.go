//go:build stress
// +build stress

package stress

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	events "dev.helix.agent/internal/adapters"
)

// TestEventBusFlood_HighThroughput publishes events at a high rate with
// 100 buffered subscribers and asserts no events are dropped for buffered
// subscribers. The bus buffer must be large enough to absorb the burst,
// and all subscribers must drain to completion.
func TestEventBusFlood_HighThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		subscribers    = 100
		publisherCount = 10
		eventsEach     = 200 // 10 publishers × 200 = 2000 total events
		totalEvents    = publisherCount * eventsEach
		bufferSize     = totalEvents * 2 // generous buffer so no drops for buffered subs
	)

	bus := events.NewEventBus(&events.BusConfig{
		BufferSize:     bufferSize,
		PublishTimeout: 100 * time.Millisecond,
	})

	goroutinesBefore := runtime.NumGoroutine()

	// Set up 100 buffered subscribers — all share the same event type.
	// Each subscriber drains in a background goroutine.
	type subResult struct {
		received int64
	}
	results := make([]subResult, subscribers)
	var subWg sync.WaitGroup

	for i := 0; i < subscribers; i++ {
		ch := bus.Subscribe(events.EventCacheHit)
		subWg.Add(1)
		go func(idx int, c <-chan *events.Event) {
			defer subWg.Done()
			for range c {
				atomic.AddInt64(&results[idx].received, 1)
			}
		}(i, ch)
	}

	var (
		wg            sync.WaitGroup
		publishPanics int64
	)
	start := make(chan struct{})

	// High-rate publishers: all fire concurrently after start.
	for p := 0; p < publisherCount; p++ {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&publishPanics, 1)
				}
			}()
			<-start
			for j := 0; j < eventsEach; j++ {
				bus.Publish(events.NewEvent(
					events.EventCacheHit,
					fmt.Sprintf("flood-publisher-%d", pid),
					j,
				))
			}
		}(p)
	}

	close(start)

	// Wait for all publishers to finish.
	publishDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(publishDone)
	}()

	select {
	case <-publishDone:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: event bus flood publishers timed out")
	}

	// Close bus to flush all subscriber goroutines.
	_ = bus.Close()
	subWg.Wait()

	assert.Zero(t, publishPanics, "no panics during high-throughput flood publish")

	// Numeric assertion: EventsPublished must equal total published events.
	metrics := bus.Metrics()
	assert.Equal(t, int64(totalEvents), metrics.EventsPublished,
		"all %d events must be counted as published", totalEvents)

	// For buffered subscribers: EventsDropped should be zero — the buffer
	// was sized to absorb the full burst.
	assert.Zero(t, metrics.EventsDropped,
		"no events should be dropped with adequately sized subscriber buffers")

	// Goroutine leak check
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20, "no goroutine leak after event bus flood")

	// Log per-subscriber receipt totals for diagnostics.
	var totalReceived int64
	for i := range results {
		totalReceived += results[i].received
	}
	t.Logf("EventBus flood: published=%d, delivered=%d, dropped=%d, total_received_across_subs=%d",
		metrics.EventsPublished, metrics.EventsDelivered, metrics.EventsDropped, totalReceived)
}
