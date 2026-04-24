// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/mcp"
)

// TestMCPConnectionPool_ConcurrentRegisterServer tests concurrent server
// registration on MCPConnectionPool. The pool uses sync.RWMutex internally —
// this test drives concurrent writes to the connections map.
func TestMCPConnectionPool_ConcurrentRegisterServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	pool := mcp.NewConnectionPool(nil, mcp.DefaultPoolConfig(), logger)

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Each goroutine registers its own server — unique names avoid the
			// "already registered" error while still racing on the map write.
			cfg := mcp.MCPServerConfig{
				Name:    fmt.Sprintf("test-server-%d", id),
				Type:    mcp.MCPServerTypeRemote,
				URL:     fmt.Sprintf("http://localhost:%d", 9000+id),
				Timeout: 5 * time.Second,
				Enabled: true,
			}
			_ = pool.RegisterServer(cfg)
		}(i)
	}

	wg.Wait()
}

// TestMCPConnectionPool_ConcurrentRegisterAndGetStatus tests concurrent
// registration and status reads to exercise the RWMutex read/write split.
func TestMCPConnectionPool_ConcurrentRegisterAndGetStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	pool := mcp.NewConnectionPool(nil, mcp.DefaultPoolConfig(), logger)

	// Pre-register a server so readers have something to read.
	_ = pool.RegisterServer(mcp.MCPServerConfig{
		Name:    "static-server",
		Type:    mcp.MCPServerTypeRemote,
		URL:     "http://localhost:9999",
		Timeout: 5 * time.Second,
		Enabled: true,
	})

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if id%2 == 0 {
				// Writer: register a new server.
				cfg := mcp.MCPServerConfig{
					Name:    fmt.Sprintf("dynamic-server-%d", id),
					Type:    mcp.MCPServerTypeRemote,
					URL:     fmt.Sprintf("http://localhost:%d", 8000+id),
					Timeout: 5 * time.Second,
					Enabled: true,
				}
				_ = pool.RegisterServer(cfg)
			} else {
				// Reader: get status for the static server.
				_, _ = pool.GetServerStatus("static-server")
				_ = pool.GetMetrics()
				_ = pool.ListServers()
			}
		}(i)
	}

	wg.Wait()
}
