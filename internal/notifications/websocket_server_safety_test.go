package notifications

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWebSocketServer_Concurrent validates that the documented lock ordering
// (clientsMu before globalClientsMu) is correct under concurrent access.
// Running with -race detects any data races or lock-order violations.
func TestWebSocketServer_Concurrent(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	server := NewWebSocketServer(nil, logger)

	const numGoroutines = 50

	// Pre-register a set of task clients and global clients so Broadcast /
	// BroadcastGlobal have real work to do under concurrent access.
	for i := 0; i < 10; i++ {
		client := NewMockWebSocketClient(fmt.Sprintf("task-client-%d", i))
		if err := server.RegisterClient("task-concurrent", client); err != nil {
			t.Fatalf("RegisterClient failed: %v", err)
		}
	}
	for i := 0; i < 10; i++ {
		client := NewMockWebSocketClient(fmt.Sprintf("global-client-%d", i))
		if err := server.RegisterGlobalClient(client); err != nil {
			t.Fatalf("RegisterGlobalClient failed: %v", err)
		}
	}

	payload := []byte(`{"event":"concurrent_safety_check"}`)

	var wg sync.WaitGroup

	// 50 goroutines calling Broadcast (acquires clientsMu then globalClientsMu)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.Broadcast("task-concurrent", payload)
		}()
	}

	// 50 goroutines calling BroadcastGlobal (acquires only globalClientsMu)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.broadcastGlobal(payload)
		}()
	}

	// Wait for all goroutines before stopping.
	wg.Wait()

	// Stop must cleanly drain the WaitGroup and close all clients.
	err := server.Stop()
	assert.NoError(t, err, "Stop() should not return an error after concurrent broadcasts")
}
