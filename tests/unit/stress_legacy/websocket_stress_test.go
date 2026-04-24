// Previously tests/stress/... — demoted to unit package under CONST-030.
// Stress tests of in-process concurrency primitives (semaphore / pool /
// cache saturation, circuit-breaker storms, goroutine growth) that
// exercise local mock providers and invokers. Keeping the mocks in the
// unit tree per the audit's Pattern-4 disposition for stress tests.
// PR33 of the CONST-030 compliance campaign.
package stress_legacy_test

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/notifications"
)

// mockWSClient implements notifications.WebSocketClientInterface for
// stress testing without real WebSocket connections.
type mockWSClient struct {
	id       string
	messages [][]byte
	mu       sync.Mutex
	closed   bool
	sendErr  error
}

func newMockWSClient(id string) *mockWSClient {
	return &mockWSClient{
		id:       id,
		messages: make([][]byte, 0, 64),
	}
}

func (m *mockWSClient) Send(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("client %s is closed", m.id)
	}
	if m.sendErr != nil {
		return m.sendErr
	}
	m.messages = append(m.messages, data)
	return nil
}

func (m *mockWSClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockWSClient) ID() string {
	return m.id
}

func (m *mockWSClient) messageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *mockWSClient) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// TestStress_WebSocket_ConcurrentRegistrations registers 100 mock clients
// concurrently to the WebSocket server and verifies no panics, deadlocks,
// or lost registrations occur.
func TestStress_WebSocket_ConcurrentRegistrations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := notifications.DefaultWebSocketConfig()
	server := notifications.NewWebSocketServer(config, logger)
	require.NotNil(t, server)
	require.NoError(t, server.Start())
	defer server.Stop()

	const clientCount = 100

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var panicCount, regErrors int64
	clients := make([]*mockWSClient, clientCount)

	start := make(chan struct{})

	for i := 0; i < clientCount; i++ {
		clients[i] = newMockWSClient(fmt.Sprintf("ws-client-%d", i))
		wg.Add(1)
		go func(id int, client *mockWSClient) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			taskID := fmt.Sprintf("task-%d", id%10)
			if err := server.RegisterClient(taskID, client); err != nil {
				atomic.AddInt64(&regErrors, 1)
			}
		}(i, clients[i])
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
		t.Fatal("DEADLOCK DETECTED: WebSocket concurrent registrations timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	totalCount := server.GetTotalClientCount()

	assert.Zero(t, panicCount, "no panics during concurrent client registrations")
	assert.Zero(t, regErrors, "all client registrations should succeed")
	assert.Equal(t, clientCount, totalCount,
		"all 100 clients should be registered")
	assert.Less(t, leaked, 15,
		"goroutines should not leak from client registrations")
	t.Logf("WebSocket registration stress: registered=%d, errors=%d, "+
		"panics=%d, goroutine_leak=%d",
		totalCount, regErrors, panicCount, leaked)
}

// TestStress_WebSocket_RapidConnectDisconnect simulates rapid connect and
// disconnect cycles of 100 clients to verify that the server correctly
// cleans up resources and does not leak goroutines or memory.
func TestStress_WebSocket_RapidConnectDisconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := notifications.DefaultWebSocketConfig()
	server := notifications.NewWebSocketServer(config, logger)
	require.NotNil(t, server)
	require.NoError(t, server.Start())
	defer server.Stop()

	const clientCount = 100
	const cyclesPerClient = 5

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	var wg sync.WaitGroup
	var panicCount int64
	var connectOps, disconnectOps int64

	start := make(chan struct{})

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			taskID := fmt.Sprintf("rapid-task-%d", id%20)

			for cycle := 0; cycle < cyclesPerClient; cycle++ {
				clientID := fmt.Sprintf("rapid-client-%d-%d", id, cycle)
				client := newMockWSClient(clientID)

				// Register
				if err := server.RegisterClient(taskID, client); err == nil {
					atomic.AddInt64(&connectOps, 1)
				}

				// Unregister
				if err := server.UnregisterClient(taskID, clientID); err == nil {
					atomic.AddInt64(&disconnectOps, 1)
				}
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
		t.Fatal("DEADLOCK DETECTED: WebSocket rapid connect/disconnect timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)
	heapGrowthMB := float64(memAfter.HeapInuse-memBefore.HeapInuse) / 1024 / 1024

	// After all connect/disconnect cycles, total count should be 0
	totalCount := server.GetTotalClientCount()

	assert.Zero(t, panicCount, "no panics during rapid connect/disconnect")
	assert.Equal(t, int64(clientCount*cyclesPerClient), connectOps,
		"all connect operations should succeed")
	assert.Equal(t, int64(clientCount*cyclesPerClient), disconnectOps,
		"all disconnect operations should succeed")
	assert.Equal(t, 0, totalCount,
		"all clients should be unregistered after disconnect cycles")
	assert.Less(t, leaked, 15,
		"goroutines should not leak from rapid connect/disconnect")
	assert.Less(t, heapGrowthMB, 100.0,
		"heap should not grow unboundedly after connect/disconnect cycling")
	t.Logf("WebSocket rapid connect/disconnect: connects=%d, disconnects=%d, "+
		"remaining=%d, panics=%d, goroutine_leak=%d, heap_growth=%.2fMB",
		connectOps, disconnectOps, totalCount, panicCount, leaked, heapGrowthMB)
}

// TestStress_WebSocket_BroadcastUnderLoad registers 100 clients across
// multiple tasks and then broadcasts messages concurrently to verify
// that the broadcast mechanism handles high fan-out without panics
// or dropped messages.
func TestStress_WebSocket_BroadcastUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := notifications.DefaultWebSocketConfig()
	server := notifications.NewWebSocketServer(config, logger)
	require.NotNil(t, server)
	require.NoError(t, server.Start())
	defer server.Stop()

	const clientsPerTask = 10
	const taskCount = 10
	const broadcastCount = 20

	// Register clients
	allClients := make(map[string][]*mockWSClient)
	for taskIdx := 0; taskIdx < taskCount; taskIdx++ {
		taskID := fmt.Sprintf("broadcast-task-%d", taskIdx)
		for clientIdx := 0; clientIdx < clientsPerTask; clientIdx++ {
			client := newMockWSClient(
				fmt.Sprintf("bc-client-%d-%d", taskIdx, clientIdx))
			err := server.RegisterClient(taskID, client)
			require.NoError(t, err)
			allClients[taskID] = append(allClients[taskID], client)
		}
	}

	totalRegistered := server.GetTotalClientCount()
	assert.Equal(t, taskCount*clientsPerTask, totalRegistered,
		"all clients should be registered before broadcast")

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var panicCount int64
	var broadcastOps int64

	start := make(chan struct{})

	// Broadcast to all tasks concurrently
	for taskIdx := 0; taskIdx < taskCount; taskIdx++ {
		wg.Add(1)
		go func(tIdx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			taskID := fmt.Sprintf("broadcast-task-%d", tIdx)
			for j := 0; j < broadcastCount; j++ {
				msg, _ := json.Marshal(map[string]interface{}{
					"type":    "update",
					"task_id": taskID,
					"seq":     j,
				})
				server.Broadcast(taskID, msg)
				atomic.AddInt64(&broadcastOps, 1)
			}
		}(taskIdx)
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
		t.Fatal("DEADLOCK DETECTED: WebSocket broadcast under load timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	// Verify messages were received
	var totalMessages int64
	for _, clients := range allClients {
		for _, client := range clients {
			totalMessages += int64(client.messageCount())
		}
	}

	expectedBroadcasts := int64(taskCount * broadcastCount)
	expectedMessages := int64(taskCount * clientsPerTask * broadcastCount)

	assert.Zero(t, panicCount, "no panics during broadcast under load")
	assert.Equal(t, expectedBroadcasts, broadcastOps,
		"all broadcast operations should complete")
	assert.Equal(t, expectedMessages, totalMessages,
		"all clients should receive all broadcast messages")
	assert.Less(t, leaked, 15,
		"goroutines should not leak from broadcast operations")
	t.Logf("WebSocket broadcast stress: broadcasts=%d, total_messages=%d, "+
		"panics=%d, goroutine_leak=%d",
		broadcastOps, totalMessages, panicCount, leaked)
}

// TestStress_WebSocket_BroadcastAllUnderLoad tests BroadcastAll which sends
// to every connected client regardless of task, verifying correctness
// under concurrent sends.
func TestStress_WebSocket_BroadcastAllUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := notifications.DefaultWebSocketConfig()
	server := notifications.NewWebSocketServer(config, logger)
	require.NotNil(t, server)
	require.NoError(t, server.Start())
	defer server.Stop()

	// Register clients: some task-specific, some global
	const taskClientCount = 50
	const globalClientCount = 50

	var taskClients []*mockWSClient
	for i := 0; i < taskClientCount; i++ {
		client := newMockWSClient(fmt.Sprintf("task-bc-all-%d", i))
		taskID := fmt.Sprintf("bca-task-%d", i%5)
		err := server.RegisterClient(taskID, client)
		require.NoError(t, err)
		taskClients = append(taskClients, client)
	}

	var globalClients []*mockWSClient
	for i := 0; i < globalClientCount; i++ {
		client := newMockWSClient(fmt.Sprintf("global-bc-all-%d", i))
		err := server.RegisterGlobalClient(client)
		require.NoError(t, err)
		globalClients = append(globalClients, client)
	}

	const broadcastCount = 10
	const senderCount = 10

	var wg sync.WaitGroup
	var panicCount int64

	start := make(chan struct{})

	for i := 0; i < senderCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < broadcastCount; j++ {
				msg := []byte(fmt.Sprintf(`{"sender":%d,"seq":%d}`, id, j))
				server.BroadcastAll(msg)
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
		t.Fatal("DEADLOCK DETECTED: WebSocket BroadcastAll under load timed out")
	}

	assert.Zero(t, panicCount, "no panics during BroadcastAll under load")

	// Global clients should receive all broadcasts
	for _, client := range globalClients {
		count := client.messageCount()
		assert.Equal(t, senderCount*broadcastCount, count,
			"global client %s should receive all BroadcastAll messages", client.ID())
	}

	t.Logf("WebSocket BroadcastAll stress: senders=%d, broadcasts_per_sender=%d, "+
		"task_clients=%d, global_clients=%d, panics=%d",
		senderCount, broadcastCount, taskClientCount, globalClientCount, panicCount)
}

// TestStress_WebSocket_MixedOperations exercises register, unregister,
// broadcast, and client count queries all happening concurrently to
// detect lock ordering issues or deadlocks.
func TestStress_WebSocket_MixedOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := notifications.DefaultWebSocketConfig()
	server := notifications.NewWebSocketServer(config, logger)
	require.NotNil(t, server)
	require.NoError(t, server.Start())
	defer server.Stop()

	const goroutineCount = 100

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var panicCount int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			taskID := fmt.Sprintf("mixed-task-%d", id%10)

			switch id % 5 {
			case 0: // Register task clients
				for j := 0; j < 5; j++ {
					client := newMockWSClient(
						fmt.Sprintf("mixed-reg-%d-%d", id, j))
					_ = server.RegisterClient(taskID, client)
				}
			case 1: // Register global clients
				for j := 0; j < 5; j++ {
					client := newMockWSClient(
						fmt.Sprintf("mixed-global-%d-%d", id, j))
					_ = server.RegisterGlobalClient(client)
				}
			case 2: // Broadcast
				for j := 0; j < 10; j++ {
					msg := []byte(fmt.Sprintf(`{"id":%d,"j":%d}`, id, j))
					server.Broadcast(taskID, msg)
				}
			case 3: // BroadcastAll
				for j := 0; j < 10; j++ {
					msg := []byte(fmt.Sprintf(`{"all":%d,"j":%d}`, id, j))
					server.BroadcastAll(msg)
				}
			case 4: // Query counts
				for j := 0; j < 20; j++ {
					_ = server.GetClientCount(taskID)
					_ = server.GetTotalClientCount()
				}
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
		t.Fatal("DEADLOCK DETECTED: WebSocket mixed operations timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panicCount,
		"no panics during mixed WebSocket operations")
	assert.Less(t, leaked, 15,
		"goroutines should not leak from mixed WebSocket operations")
	t.Logf("WebSocket mixed ops stress: goroutines=%d, panics=%d, "+
		"goroutine_leak=%d, final_client_count=%d",
		goroutineCount, panicCount, leaked, server.GetTotalClientCount())
}
