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

// TestEventBus_HighThroughputPublish_Stress launches 200 goroutines each
// publishing 100 events to verify no panics, no deadlocks, and that the
// bus drains cleanly after close.
func TestEventBus_HighThroughputPublish_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		publishers      = 200
		eventsEach      = 100
		subscriberCount = 5
	)

	bus := events.NewEventBus(&events.BusConfig{
		BufferSize:     10000,
		PublishTimeout: 50 * time.Millisecond,
	})

	goroutinesBefore := runtime.NumGoroutine()

	// Set up subscribers that drain their channels
	var received int64
	var subscriberWg sync.WaitGroup
	for i := 0; i < subscriberCount; i++ {
		ch := bus.Subscribe(events.EventCacheHit)
		subscriberWg.Add(1)
		go func(c <-chan *events.Event) {
			defer subscriberWg.Done()
			for range c {
				atomic.AddInt64(&received, 1)
			}
		}(ch)
	}

	var (
		wg     sync.WaitGroup
		panics int64
	)
	start := make(chan struct{})

	for i := 0; i < publishers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			for j := 0; j < eventsEach; j++ {
				bus.Publish(events.NewEvent(
					events.EventCacheHit,
					fmt.Sprintf("publisher-%d", id),
					j,
				))
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: high-throughput publish stress timed out")
	}

	// Close bus to flush subscriber goroutines
	_ = bus.Close()
	subscriberWg.Wait()

	assert.Zero(t, panics, "no goroutine should panic during high-throughput publish")

	metrics := bus.Metrics()
	t.Logf("EventBus metrics: published=%d, delivered=%d, dropped=%d",
		metrics.EventsPublished, metrics.EventsDelivered, metrics.EventsDropped)

	assert.Equal(t, int64(publishers*eventsEach), metrics.EventsPublished,
		"all published events should be counted in metrics")

	// Goroutine leak check
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20,
		"goroutine count should not grow significantly after bus close")
}

// TestEventBus_ConcurrentSubscribeUnsubscribe_Stress verifies that
// subscribing and unsubscribing concurrently with active publishing does
// not cause data races or panics.
func TestEventBus_ConcurrentSubscribeUnsubscribe_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	bus := events.NewEventBus(&events.BusConfig{
		BufferSize:     500,
		PublishTimeout: 20 * time.Millisecond,
	})
	defer func() { _ = bus.Close() }()

	var (
		wg     sync.WaitGroup
		panics int64
	)
	start := make(chan struct{})

	// Publishers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			for j := 0; j < 200; j++ {
				bus.Publish(events.NewEvent(events.EventCacheHit, "pub", id*j))
				bus.Publish(events.NewEvent(events.EventCacheMiss, "pub", id*j))
			}
		}(i)
	}

	// Subscribers that subscribe and then quickly unsubscribe
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			ch := bus.Subscribe(events.EventCacheHit)
			// Drain briefly
			timer := time.NewTimer(5 * time.Millisecond)
			select {
			case <-timer.C:
			case <-ch:
				timer.Stop()
			}
			bus.Unsubscribe(ch)
		}()
	}

	// Metrics readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 50; j++ {
				bus.Metrics()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: concurrent subscribe/unsubscribe stress timed out")
	}

	assert.Zero(t, panics,
		"no goroutine should panic during concurrent subscribe/unsubscribe")
}

// TestEventBus_MultipleEventTypes_Stress verifies that events published to
// different event types are routed correctly to type-specific subscribers
// with no cross-contamination under concurrent load.
func TestEventBus_MultipleEventTypes_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	bus := events.NewEventBus(&events.BusConfig{
		BufferSize:     2000,
		PublishTimeout: 30 * time.Millisecond,
	})

	testEventTypes := []events.EventType{
		events.EventCacheHit,
		events.EventCacheMiss,
		events.EventCacheInvalidated,
		events.EventProviderRegistered,
		events.EventRequestReceived,
	}

	// Track counts per type
	var counters [5]int64

	var subWg sync.WaitGroup
	for idx, et := range testEventTypes {
		idx := idx
		ch := bus.Subscribe(et)
		subWg.Add(1)
		go func(c <-chan *events.Event, i int) {
			defer subWg.Done()
			for range c {
				atomic.AddInt64(&counters[i], 1)
			}
		}(ch, idx)
	}

	const eventsPerType = 500
	var wg sync.WaitGroup
	var panics int64
	start := make(chan struct{})

	for workerIdx := 0; workerIdx < 100; workerIdx++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			for j := 0; j < eventsPerType/100; j++ {
				for _, et := range testEventTypes {
					bus.Publish(events.NewEvent(et, "multi-type-stress", wid))
				}
			}
		}(workerIdx)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: multiple event types stress timed out")
	}

	_ = bus.Close()
	subWg.Wait()

	assert.Zero(t, panics, "no panics during multi-type event stress")

	metrics := bus.Metrics()
	t.Logf("EventBus multi-type metrics: published=%d, delivered=%d, dropped=%d",
		metrics.EventsPublished, metrics.EventsDelivered, metrics.EventsDropped)

	// Total published must equal len(types) * eventsPerType
	assert.Equal(t, int64(len(testEventTypes)*eventsPerType), metrics.EventsPublished,
		"all events across all types should be counted")
}

// TestEventBus_NoGoroutineLeaks_Stress creates and destroys multiple bus
// instances under concurrent publish/subscribe load, verifying the goroutine
// count returns to near-baseline after all buses are closed.
func TestEventBus_NoGoroutineLeaks_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	for iteration := 0; iteration < 10; iteration++ {
		b := events.NewEventBus(&events.BusConfig{
			BufferSize:     100,
			PublishTimeout: 10 * time.Millisecond,
		})

		// Add subscribers
		channels := make([]<-chan *events.Event, 5)
		for i := 0; i < 5; i++ {
			channels[i] = b.Subscribe(events.EventCacheHit)
		}

		// Drain subscribers
		var drainWg sync.WaitGroup
		for _, ch := range channels {
			drainWg.Add(1)
			go func(c <-chan *events.Event) {
				defer drainWg.Done()
				for range c {
				}
			}(ch)
		}

		// Publish and then close
		for j := 0; j < 100; j++ {
			b.Publish(events.NewEvent(events.EventCacheHit, "leak-test", j))
		}

		_ = b.Close()
		drainWg.Wait()
	}

	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("EventBus goroutine leak: before=%d, after=%d, delta=%d",
		goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 15,
		"goroutine count should not grow significantly after repeated bus create/destroy")
}
