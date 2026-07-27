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

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/notifications"
)

// TestSSE_100ConcurrentClients_ConnectDisconnect simulates 100 concurrent SSE
// clients that each register, receive several broadcasts, and then deregister.
// The test verifies:
//   - Clean client cleanup: zero registered clients after all have disconnected.
//   - Bounded memory: goroutine count does not grow excessively.
//   - No panics during concurrent registration / broadcast / deregistration.
func TestSSE_100ConcurrentClients_ConnectDisconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // suppress debug noise during stress

	// Use a short heartbeat so the manager loop stays active during the test
	// without adding significant overhead.
	cfg := &notifications.SSEConfig{
		HeartbeatInterval: 5 * time.Second,
		BufferSize:        64,
		MaxClients:        200,
	}
	mgr := notifications.NewSSEManager(cfg, logger)
	defer func() { _ = mgr.Stop() }()

	const (
		clients          = 100
		broadcastsPerKey = 10
		taskIDPrefix     = "stress-task-"
	)

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var (
		wg         sync.WaitGroup
		panicCount int64
		registered int64
		received   int64
		start      = make(chan struct{})
	)

	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()

			<-start

			taskID := fmt.Sprintf("%s%d", taskIDPrefix, id%10) // 10 shared task buckets
			ch := make(chan []byte, 32)

			// Register as a task-specific client.
			if err := mgr.RegisterClient(taskID, ch); err != nil {
				return
			}
			atomic.AddInt64(&registered, 1)

			// Drain incoming messages for a short window.
			deadline := time.After(500 * time.Millisecond)
		drain:
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						break drain
					}
					atomic.AddInt64(&received, 1)
				case <-deadline:
					break drain
				}
			}

			// Deregister — this is the cleanup path under test.
			_ = mgr.UnregisterClient(taskID, ch)
		}(i)
	}

	close(start)

	// Concurrently broadcast to all task buckets while clients are active.
	var broadcastWG sync.WaitGroup
	for b := 0; b < broadcastsPerKey; b++ {
		broadcastWG.Add(1)
		go func(seq int) {
			defer broadcastWG.Done()
			for k := 0; k < 10; k++ {
				taskID := fmt.Sprintf("%s%d", taskIDPrefix, k)
				mgr.Broadcast(taskID, []byte(fmt.Sprintf(`{"seq":%d,"task":%d}`, seq, k)))
			}
		}(b)
	}
	broadcastWG.Wait()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: SSE storm test timed out after 30s")
	}

	// After all clients deregistered the manager should hold no task clients.
	remaining := mgr.GetTotalClientCount()
	// Global clients (none registered here) = 0; task clients should also = 0.
	assert.Zero(t, remaining,
		"all SSE clients must be cleaned up after deregistration")

	assert.Zero(t, panicCount, "no goroutine should panic during SSE storm")
	assert.Equal(t, int64(clients), registered, "all clients must register successfully")

	t.Logf("SSE storm: clients=%d registered=%d received=%d panics=%d remainingClients=%d",
		clients, registered, received, panicCount, remaining)

	// Goroutine-leak check.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	assert.Less(t, leaked, 30,
		"goroutine count must not grow excessively after SSE storm")
	t.Logf("Goroutines: before=%d after=%d leaked=%d", goroutinesBefore, goroutinesAfter, leaked)
}

// TestSSE_GlobalClients_Storm exercises the global-client broadcast path with
// 50 concurrent global clients all connecting and disconnecting rapidly while
// broadcasts are in flight.
func TestSSE_GlobalClients_Storm(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	cfg := &notifications.SSEConfig{
		HeartbeatInterval: 10 * time.Second,
		BufferSize:        32,
		MaxClients:        200,
	}
	mgr := notifications.NewSSEManager(cfg, logger)
	defer func() { _ = mgr.Stop() }()

	const globalClients = 50

	var (
		wg         sync.WaitGroup
		panicCount int64
		start      = make(chan struct{})
	)

	for i := 0; i < globalClients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()

			<-start

			ch := make(chan []byte, 16)
			if err := mgr.RegisterGlobalClient(ch); err != nil {
				return
			}

			// Short drain window.
			timer := time.NewTimer(300 * time.Millisecond)
			defer timer.Stop()
		drain:
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						break drain
					}
				case <-timer.C:
					break drain
				}
			}

			_ = mgr.UnregisterGlobalClient(ch)
		}()
	}

	// Broadcaster goroutine runs concurrently with connect/disconnect.
	broadcastDone := make(chan struct{})
	go func() {
		defer close(broadcastDone)
		for i := 0; i < 200; i++ {
			mgr.BroadcastAll([]byte(fmt.Sprintf(`{"seq":%d}`, i)))
			time.Sleep(time.Millisecond)
		}
	}()

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("DEADLOCK: global SSE storm timed out")
	}

	<-broadcastDone

	assert.Zero(t, panicCount, "no panic during global SSE client storm")
	assert.Zero(t, mgr.GetTotalClientCount(), "no global clients must remain after storm")
}

// TestSSE_BroadcastWhileStopping verifies that calling Stop() while an active
// broadcast storm is in progress does not panic or deadlock.
func TestSSE_BroadcastWhileStopping(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	cfg := notifications.DefaultSSEConfig()
	cfg.HeartbeatInterval = 30 * time.Second // don't interfere with test timing
	mgr := notifications.NewSSEManager(cfg, logger)

	// Pre-register a handful of clients.
	const preRegistered = 20
	for i := 0; i < preRegistered; i++ {
		ch := make(chan []byte, 8)
		_ = mgr.RegisterClient(fmt.Sprintf("task-%d", i), ch)
	}

	var panicCount int64

	// Start a broadcast storm in the background.
	var stormWG sync.WaitGroup
	stormWG.Add(1)
	go func() {
		defer stormWG.Done()
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt64(&panicCount, 1)
			}
		}()
		for i := 0; i < 500; i++ {
			mgr.Broadcast(fmt.Sprintf("task-%d", i%preRegistered),
				[]byte(fmt.Sprintf(`{"i":%d}`, i)))
		}
	}()

	// Stop the manager mid-storm.
	time.Sleep(5 * time.Millisecond)
	stopErr := mgr.Stop()

	stormWG.Wait()

	assert.NoError(t, stopErr, "Stop() must not return an error")
	assert.Zero(t, panicCount, "broadcast-while-stopping must not panic")
}
