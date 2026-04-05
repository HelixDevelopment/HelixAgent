package mcp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMCPConnectionPool_ConcurrentGetClose(t *testing.T) {
	t.Parallel()
	pool := NewConnectionPool(nil, &MCPPoolConfig{
		MaxConnections:    20,
		ConnectionTimeout: 5 * time.Second,
		IdleTimeout:       1 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		RetryAttempts:     1,
		RetryDelay:        10 * time.Millisecond,
	}, nil)

	// Register several servers (they won't actually connect, but that's fine
	// for exercising the race between Get and Close)
	for i := 0; i < 5; i++ {
		name := "server-" + string(rune('a'+i))
		err := pool.RegisterServer(MCPServerConfig{
			Name:    name,
			Type:    MCPServerTypeRemote,
			URL:     "http://localhost:19999",
			Enabled: true,
			Timeout: 1 * time.Second,
		})
		assert.NoError(t, err)
	}

	servers := pool.ListServers()

	const goroutines = 20
	var wg sync.WaitGroup
	ctx := context.Background()

	// Half the goroutines do GetConnection, other half do Close/RegisterServer
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if id%2 == 0 {
				// Try to get connections (may fail due to pool closing or
				// unreachable server — errors are expected)
				for _, name := range servers {
					_, _ = pool.GetConnection(ctx, name)
				}
			} else {
				// Close the pool concurrently
				_ = pool.Close()
			}
		}(i)
	}

	wg.Wait()

	// After all goroutines finish, pool must be closed
	assert.True(t, pool.closed.Load(), "pool should be closed after concurrent Close calls")

	// RegisterServer must fail on a closed pool
	err := pool.RegisterServer(MCPServerConfig{
		Name:    "after-close",
		Type:    MCPServerTypeRemote,
		URL:     "http://localhost:19999",
		Enabled: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pool is closed")
}
