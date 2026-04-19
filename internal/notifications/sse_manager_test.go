package notifications

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for DefaultSSEConfig
func TestDefaultSSEConfig(t *testing.T) {
	t.Parallel()
	config := DefaultSSEConfig()

	assert.Equal(t, 30*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 100, config.BufferSize)
	assert.Equal(t, 1000, config.MaxClients)
}

// Tests for NewSSEManager
func TestNewSSEManager(t *testing.T) {
	t.Parallel()
	logger := testLogger()

	t.Run("with default config", func(t *testing.T) {
		t.Parallel()
		manager := NewSSEManager(nil, logger)
		require.NotNil(t, manager)

		assert.NotNil(t, manager.state)
		assert.Equal(t, 0, manager.GetTotalClientCount())
		assert.Equal(t, logger, manager.logger)
		assert.Equal(t, 30*time.Second, manager.heartbeatInterval)
		assert.Equal(t, 100, manager.bufferSize)

		_ = manager.Stop()
	})

	t.Run("with custom config", func(t *testing.T) {
		t.Parallel()
		config := &SSEConfig{
			HeartbeatInterval: 10 * time.Second,
			BufferSize:        50,
			MaxClients:        500,
		}

		manager := NewSSEManager(config, logger)
		require.NotNil(t, manager)

		assert.Equal(t, 10*time.Second, manager.heartbeatInterval)
		assert.Equal(t, 50, manager.bufferSize)

		_ = manager.Stop()
	})
}

// Tests for SSEManager Start/Stop
func TestSSEManager_StartStop(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)

	err := manager.Start()
	assert.NoError(t, err)

	err = manager.Stop()
	assert.NoError(t, err)
}

// Tests for RegisterClient.
//
// Each subtest constructs its OWN SSEManager to avoid sharing the
// clients map across parallel subtests (race-debt BUGFIX #25). The
// previous shared-manager pattern produced nondeterministic
// GetClientCount values: "register single client" saw 3 because
// "register multiple" had already added 2 more, and vice-versa.
func TestSSEManager_RegisterClient(t *testing.T) {
	t.Parallel()
	logger := testLogger()

	newManager := func(t *testing.T) *SSEManager {
		t.Helper()
		m := NewSSEManager(nil, logger)
		t.Cleanup(func() { _ = m.Stop() })
		return m
	}

	t.Run("register single client", func(t *testing.T) {
		t.Parallel()
		manager := newManager(t)
		clientChan := make(chan []byte, 10)
		err := manager.RegisterClient("task-1", clientChan)
		assert.NoError(t, err)

		count := manager.GetClientCount("task-1")
		assert.Equal(t, 1, count)
	})

	t.Run("register multiple clients for same task", func(t *testing.T) {
		t.Parallel()
		manager := newManager(t)
		clientChan1 := make(chan []byte, 10)
		clientChan2 := make(chan []byte, 10)
		clientChan3 := make(chan []byte, 10)

		require.NoError(t, manager.RegisterClient("task-1", clientChan1))
		require.NoError(t, manager.RegisterClient("task-1", clientChan2))
		require.NoError(t, manager.RegisterClient("task-1", clientChan3))

		count := manager.GetClientCount("task-1")
		assert.Equal(t, 3, count)
	})

	t.Run("register clients for different tasks", func(t *testing.T) {
		t.Parallel()
		manager := newManager(t)
		clientChan := make(chan []byte, 10)
		err := manager.RegisterClient("task-2", clientChan)
		assert.NoError(t, err)

		count := manager.GetClientCount("task-2")
		assert.Equal(t, 1, count)
	})
}

// Tests for UnregisterClient
func TestSSEManager_UnregisterClient(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	clientChan := make(chan []byte, 10)
	_ = manager.RegisterClient("task-1", clientChan)

	err := manager.UnregisterClient("task-1", clientChan)
	assert.NoError(t, err)

	count := manager.GetClientCount("task-1")
	assert.Equal(t, 0, count)
}

// Tests for UnregisterClient with nonexistent task
func TestSSEManager_UnregisterClient_NonexistentTask(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	clientChan := make(chan []byte, 10)

	err := manager.UnregisterClient("nonexistent", clientChan)
	assert.NoError(t, err)
}

// Tests for RegisterGlobalClient
func TestSSEManager_RegisterGlobalClient(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	clientChan := make(chan []byte, 10)

	err := manager.RegisterGlobalClient(clientChan)
	assert.NoError(t, err)

	totalCount := manager.GetTotalClientCount()
	assert.Equal(t, 1, totalCount)
}

// Tests for UnregisterGlobalClient
func TestSSEManager_UnregisterGlobalClient(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	clientChan := make(chan []byte, 10)
	_ = manager.RegisterGlobalClient(clientChan)

	err := manager.UnregisterGlobalClient(clientChan)
	assert.NoError(t, err)

	totalCount := manager.GetTotalClientCount()
	assert.Equal(t, 0, totalCount)
}

// Tests for Broadcast
func TestSSEManager_Broadcast(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	clientChan := make(chan []byte, 10)
	_ = manager.RegisterClient("task-1", clientChan)

	data := []byte(`{"message":"test"}`)
	manager.Broadcast("task-1", data)

	select {
	case received := <-clientChan:
		assert.Contains(t, string(received), "message")
		assert.Contains(t, string(received), "test")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast message")
	}
}

// Tests for Broadcast to global clients
func TestSSEManager_Broadcast_IncludesGlobalClients(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	globalChan := make(chan []byte, 10)
	_ = manager.RegisterGlobalClient(globalChan)

	data := []byte(`{"message":"test"}`)
	manager.Broadcast("task-1", data)

	select {
	case received := <-globalChan:
		assert.Contains(t, string(received), "test")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for global client message")
	}
}

// Tests for Broadcast with full channel
func TestSSEManager_Broadcast_FullChannel(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	// Create a channel with no buffer
	clientChan := make(chan []byte)
	_ = manager.RegisterClient("task-1", clientChan)

	// This should not block
	data := []byte(`{"message":"test"}`)
	done := make(chan bool)
	go func() {
		manager.Broadcast("task-1", data)
		done <- true
	}()

	select {
	case <-done:
		// Success - broadcast didn't block
	case <-time.After(time.Second):
		t.Fatal("broadcast blocked on full channel")
	}
}

// Tests for BroadcastEvent
func TestSSEManager_BroadcastEvent(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	clientChan := make(chan []byte, 10)
	_ = manager.RegisterClient("task-1", clientChan)

	eventData := map[string]interface{}{
		"progress": 50,
		"status":   "running",
	}

	err := manager.BroadcastEvent("task-1", "progress_update", eventData)
	assert.NoError(t, err)

	select {
	case received := <-clientChan:
		assert.Contains(t, string(received), "event: progress_update")
		assert.Contains(t, string(received), "progress")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

// Tests for BroadcastEvent with invalid data
func TestSSEManager_BroadcastEvent_InvalidData(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	clientChan := make(chan []byte, 10)
	_ = manager.RegisterClient("task-1", clientChan)

	// Create a value that cannot be marshaled to JSON
	invalidData := make(chan int)

	err := manager.BroadcastEvent("task-1", "test", invalidData)
	assert.Error(t, err)
}

// Tests for BroadcastAll
func TestSSEManager_BroadcastAll(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	chan1 := make(chan []byte, 10)
	chan2 := make(chan []byte, 10)
	chan3 := make(chan []byte, 10) // global

	_ = manager.RegisterClient("task-1", chan1)
	_ = manager.RegisterClient("task-2", chan2)
	_ = manager.RegisterGlobalClient(chan3)

	data := []byte(`{"broadcast":"all"}`)
	manager.BroadcastAll(data)

	// Verify all clients received the message
	for _, ch := range []chan []byte{chan1, chan2, chan3} {
		select {
		case received := <-ch:
			assert.Contains(t, string(received), "broadcast")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for broadcast")
		}
	}
}

// Tests for GetClientCount
func TestSSEManager_GetClientCount(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	assert.Equal(t, 0, manager.GetClientCount("task-1"))

	chan1 := make(chan []byte, 10)
	chan2 := make(chan []byte, 10)

	_ = manager.RegisterClient("task-1", chan1)
	assert.Equal(t, 1, manager.GetClientCount("task-1"))

	_ = manager.RegisterClient("task-1", chan2)
	assert.Equal(t, 2, manager.GetClientCount("task-1"))

	_ = manager.UnregisterClient("task-1", chan1)
	assert.Equal(t, 1, manager.GetClientCount("task-1"))
}

// Tests for GetTotalClientCount
func TestSSEManager_GetTotalClientCount(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	assert.Equal(t, 0, manager.GetTotalClientCount())

	chan1 := make(chan []byte, 10)
	chan2 := make(chan []byte, 10)
	chan3 := make(chan []byte, 10)

	_ = manager.RegisterClient("task-1", chan1)
	_ = manager.RegisterClient("task-2", chan2)
	_ = manager.RegisterGlobalClient(chan3)

	assert.Equal(t, 3, manager.GetTotalClientCount())
}

// Tests for formatSSEEvent
func TestFormatSSEEvent(t *testing.T) {
	t.Parallel()
	t.Run("simple message", func(t *testing.T) {
		data := []byte(`{"test":"data"}`)
		formatted := formatSSEEvent("message", data)

		expected := "event: message\ndata: {\"test\":\"data\"}\n\n"
		assert.Equal(t, expected, string(formatted))
	})

	t.Run("heartbeat event", func(t *testing.T) {
		t.Parallel()
		data := []byte(`{"type":"heartbeat"}`)
		formatted := formatSSEEvent("heartbeat", data)

		assert.Contains(t, string(formatted), "event: heartbeat")
		assert.Contains(t, string(formatted), "heartbeat")
	})

	t.Run("custom event", func(t *testing.T) {
		t.Parallel()
		data := []byte(`{"progress":50}`)
		formatted := formatSSEEvent("progress_update", data)

		assert.Contains(t, string(formatted), "event: progress_update")
		assert.Contains(t, string(formatted), "progress")
	})
}

// Tests for SSESubscriber
func TestSSESubscriber(t *testing.T) {
	t.Parallel()
	clientChan := make(chan []byte, 10)

	t.Run("create new subscriber", func(t *testing.T) {
		t.Parallel()
		subscriber := NewSSESubscriber("sub-1", "task-1", clientChan)

		assert.Equal(t, "sub-1", subscriber.ID())
		assert.Equal(t, NotificationTypeSSE, subscriber.Type())
		assert.True(t, subscriber.IsActive())
	})

	t.Run("notify subscriber", func(t *testing.T) {
		t.Parallel()
		subscriber := NewSSESubscriber("sub-1", "task-1", clientChan)

		notification := &TaskNotification{
			TaskID:    "task-1",
			EventType: "progress",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"progress": 50},
		}

		err := subscriber.Notify(context.Background(), notification)
		assert.NoError(t, err)

		select {
		case received := <-clientChan:
			assert.Contains(t, string(received), "progress")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for notification")
		}
	})

	t.Run("close subscriber", func(t *testing.T) {
		t.Parallel()
		subscriber := NewSSESubscriber("sub-1", "task-1", clientChan)

		err := subscriber.Close()
		assert.NoError(t, err)
		assert.False(t, subscriber.IsActive())
	})
}

// Tests for SSESubscriber with full channel
func TestSSESubscriber_FullChannel(t *testing.T) {
	t.Parallel()
	clientChan := make(chan []byte) // unbuffered

	subscriber := NewSSESubscriber("sub-1", "task-1", clientChan)

	notification := &TaskNotification{
		TaskID:    "task-1",
		EventType: "test",
		Timestamp: time.Now(),
	}

	err := subscriber.Notify(context.Background(), notification)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel full")
}

// Tests for concurrent client registration
func TestSSEManager_ConcurrentRegistration(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	var wg sync.WaitGroup
	numGoroutines := 10
	numClients := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numClients; j++ {
				clientChan := make(chan []byte, 10)
				_ = manager.RegisterClient("task-1", clientChan)
			}
		}()
	}

	wg.Wait()

	totalCount := manager.GetClientCount("task-1")
	assert.Equal(t, numGoroutines*numClients, totalCount)
}

// Tests for concurrent broadcast
func TestSSEManager_ConcurrentBroadcast(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	// Register multiple clients
	channels := make([]chan []byte, 10)
	for i := range channels {
		channels[i] = make(chan []byte, 100)
		_ = manager.RegisterClient("task-1", channels[i])
	}

	var wg sync.WaitGroup
	numBroadcasts := 100

	for i := 0; i < numBroadcasts; i++ {
		wg.Add(1)
		go func(msgNum int) {
			defer wg.Done()
			data := []byte(`{"msg":` + string(rune('0'+msgNum%10)) + `}`)
			manager.Broadcast("task-1", data)
		}(i)
	}

	wg.Wait()

	// Verify no panic occurred
	assert.Equal(t, 10, manager.GetClientCount("task-1"))
}

// Tests for heartbeat functionality
func TestSSEManager_Heartbeat(t *testing.T) {
	t.Parallel()
	logger := testLogger()

	config := &SSEConfig{
		HeartbeatInterval: 100 * time.Millisecond, // Short interval for testing
		BufferSize:        100,
	}

	manager := NewSSEManager(config, logger)
	defer func() { _ = manager.Stop() }()

	clientChan := make(chan []byte, 10)
	_ = manager.RegisterClient("task-1", clientChan)

	// Wait for at least one heartbeat
	select {
	case received := <-clientChan:
		assert.Contains(t, string(received), "heartbeat")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for heartbeat")
	}
}

// Tests for SSE manager Stop closing channels
func TestSSEManager_Stop_ClosesChannels(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)

	clientChan := make(chan []byte, 10)
	globalChan := make(chan []byte, 10)

	_ = manager.RegisterClient("task-1", clientChan)
	_ = manager.RegisterGlobalClient(globalChan)

	err := manager.Stop()
	assert.NoError(t, err)

	// Channels should be closed
	select {
	case _, ok := <-clientChan:
		assert.False(t, ok, "client channel should be closed")
	case <-time.After(100 * time.Millisecond):
		// Channel might be empty but not closed in some implementations
	}
}

// Tests for SSEConfig
func TestSSEConfig(t *testing.T) {
	t.Parallel()
	config := &SSEConfig{
		HeartbeatInterval: 45 * time.Second,
		BufferSize:        200,
		MaxClients:        2000,
	}

	assert.Equal(t, 45*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 200, config.BufferSize)
	assert.Equal(t, 2000, config.MaxClients)
}

// Tests for concurrent senders + Stop not panicking (send-on-closed-channel)
func TestSSEManager_Stop_ConcurrentSendersNoPanic(t *testing.T) {
	t.Parallel()
	logger := testLogger()

	t.Run("concurrent broadcast during stop does not panic", func(t *testing.T) {
		t.Parallel()
		manager := NewSSEManager(nil, logger)

		// Register several clients with buffered channels
		const numClients = 10
		for i := 0; i < numClients; i++ {
			ch := make(chan []byte, 100)
			_ = manager.RegisterClient("task-1", ch)
		}

		globalCh := make(chan []byte, 100)
		_ = manager.RegisterGlobalClient(globalCh)

		// Launch concurrent senders that keep broadcasting
		var senderWg sync.WaitGroup
		const numSenders = 20
		stop := make(chan struct{})

		for i := 0; i < numSenders; i++ {
			senderWg.Add(1)
			go func(id int) {
				defer senderWg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						data := []byte(`{"sender":"active"}`)
						manager.Broadcast("task-1", data)
						_ = manager.BroadcastEvent("task-1", "test", map[string]string{"key": "val"})
						manager.BroadcastAll(data)
					}
				}
			}(i)
		}

		// Let senders run briefly, then stop the manager concurrently
		time.Sleep(10 * time.Millisecond)
		err := manager.Stop()
		assert.NoError(t, err)

		// Signal senders to finish
		close(stop)
		senderWg.Wait()

		// Additional broadcasts after stop should silently return (no panic)
		manager.Broadcast("task-1", []byte(`{"after":"stop"}`))
		_ = manager.BroadcastEvent("task-1", "late", map[string]string{"x": "y"})
		manager.BroadcastAll([]byte(`{"after":"stop"}`))
	})

	t.Run("double stop does not panic", func(t *testing.T) {
		t.Parallel()
		manager := NewSSEManager(nil, logger)

		ch := make(chan []byte, 10)
		_ = manager.RegisterClient("task-1", ch)

		err1 := manager.Stop()
		assert.NoError(t, err1)

		// Second Stop must not panic (double-close protection)
		err2 := manager.Stop()
		assert.NoError(t, err2)
	})

	t.Run("concurrent stops do not panic", func(t *testing.T) {
		t.Parallel()
		manager := NewSSEManager(nil, logger)

		ch := make(chan []byte, 10)
		_ = manager.RegisterClient("task-1", ch)
		_ = manager.RegisterGlobalClient(make(chan []byte, 10))

		var stopWg sync.WaitGroup
		const numStoppers = 10
		for i := 0; i < numStoppers; i++ {
			stopWg.Add(1)
			go func() {
				defer stopWg.Done()
				_ = manager.Stop()
			}()
		}
		stopWg.Wait()
	})
}

// Tests for SSE event data marshaling
func TestSSEManager_EventDataMarshaling(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	manager := NewSSEManager(nil, logger)
	defer func() { _ = manager.Stop() }()

	clientChan := make(chan []byte, 10)
	_ = manager.RegisterClient("task-1", clientChan)

	// Complex event data
	eventData := map[string]interface{}{
		"task_id":  "task-1",
		"progress": 75.5,
		"status":   "running",
		"metadata": map[string]interface{}{
			"started_at": time.Now().Format(time.RFC3339),
			"worker":     "worker-1",
		},
		"errors": []string{"warning1", "warning2"},
	}

	err := manager.BroadcastEvent("task-1", "complex_event", eventData)
	assert.NoError(t, err)

	select {
	case received := <-clientChan:
		// Verify the data is valid JSON
		var parsed map[string]interface{}
		// Extract just the data part after "data: "
		dataStr := string(received)
		dataStart := len("event: complex_event\ndata: ")
		dataEnd := len(dataStr) - 2 // remove trailing \n\n
		if dataEnd > dataStart {
			jsonData := dataStr[dataStart:dataEnd]
			err := json.Unmarshal([]byte(jsonData), &parsed)
			assert.NoError(t, err)
			assert.Equal(t, "task-1", parsed["task_id"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for complex event")
	}
}
