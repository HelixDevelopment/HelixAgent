// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/notifications"
)

// TestSSEManager_ConcurrentRegisterUnregister tests concurrent client registration
// and unregistration on the SSEManager. Both clientsMu (RWMutex) and
// globalClientsMu (RWMutex) are exercised simultaneously.
func TestSSEManager_ConcurrentRegisterUnregister(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := notifications.DefaultSSEConfig()
	cfg.HeartbeatInterval = time.Hour // very long interval so ticker never fires during test
	mgr := notifications.NewSSEManager(cfg, logger)
	defer mgr.Stop() //nolint:errcheck

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			taskID := fmt.Sprintf("task-%d", id%5) // shared task IDs create contention
			ch := make(chan []byte, 10)

			_ = mgr.RegisterClient(taskID, ch)
			_ = mgr.UnregisterClient(taskID, ch)
		}(i)
	}

	wg.Wait()
}

// TestSSEManager_ConcurrentGlobalClients tests concurrent global client
// register/unregister operations.
func TestSSEManager_ConcurrentGlobalClients(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := notifications.DefaultSSEConfig()
	cfg.HeartbeatInterval = time.Hour
	mgr := notifications.NewSSEManager(cfg, logger)
	defer mgr.Stop() //nolint:errcheck

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ch := make(chan []byte, 10)
			_ = mgr.RegisterGlobalClient(ch)
			_ = mgr.UnregisterGlobalClient(ch)
		}()
	}

	wg.Wait()
}

// TestSSEManager_ConcurrentIPTracking tests concurrent RegisterClientWithIP
// calls, which exercise the ipConnsMu Mutex.
func TestSSEManager_ConcurrentIPTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := notifications.DefaultSSEConfig()
	cfg.HeartbeatInterval = time.Hour
	cfg.MaxConnsPerIP = 100 // high cap so goroutines are not rejected
	mgr := notifications.NewSSEManager(cfg, logger)
	defer mgr.Stop() //nolint:errcheck

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ip := fmt.Sprintf("192.168.1.%d", id%10) // shared IPs create contention
			ch := make(chan []byte, 10)
			taskID := fmt.Sprintf("task-ip-%d", id)

			_ = mgr.RegisterClientWithIP(taskID, ip, ch)
			_ = mgr.UnregisterClient(taskID, ch)
		}(i)
	}

	wg.Wait()
}
