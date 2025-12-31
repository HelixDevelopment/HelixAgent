package services

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newACPManagerTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewACPManager(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)

	require.NotNil(t, manager)
	assert.Nil(t, manager.repo)
	assert.Nil(t, manager.cache)
	assert.NotNil(t, manager.log)
}

func TestACPManager_ListACPServers(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	servers, err := manager.ListACPServers(ctx)
	require.NoError(t, err)
	require.NotNil(t, servers)
	require.NotEmpty(t, servers)

	// Check the default server
	assert.Equal(t, "opencode-1", servers[0].ID)
	assert.Equal(t, "OpenCode Agent", servers[0].Name)
	assert.True(t, servers[0].Enabled)
}

func TestACPManager_GetACPServer(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("get existing server", func(t *testing.T) {
		server, err := manager.GetACPServer(ctx, "opencode-1")
		require.NoError(t, err)
		require.NotNil(t, server)
		assert.Equal(t, "opencode-1", server.ID)
		assert.Equal(t, "OpenCode Agent", server.Name)
		assert.True(t, server.Enabled)
	})

	t.Run("get non-existent server", func(t *testing.T) {
		server, err := manager.GetACPServer(ctx, "non-existent")
		assert.Error(t, err)
		assert.Nil(t, server)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestACPManager_ExecuteACPAction(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("execute valid action", func(t *testing.T) {
		req := ACPRequest{
			ServerID:   "opencode-1",
			Action:     "code_execution",
			Parameters: map[string]interface{}{"language": "go", "code": "fmt.Println()"},
		}
		response, err := manager.ExecuteACPAction(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.True(t, response.Success)
		assert.NotNil(t, response.Data)
		assert.False(t, response.Timestamp.IsZero())
	})

	t.Run("execute on non-existent server", func(t *testing.T) {
		req := ACPRequest{
			ServerID: "non-existent",
			Action:   "test",
		}
		response, err := manager.ExecuteACPAction(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid server ID")
	})
}

func TestACPManager_ValidateACPRequest(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("valid request", func(t *testing.T) {
		req := ACPRequest{
			ServerID: "opencode-1",
			Action:   "code_execution",
		}
		err := manager.ValidateACPRequest(ctx, req)
		assert.NoError(t, err)
	})

	t.Run("missing server ID", func(t *testing.T) {
		req := ACPRequest{
			ServerID: "",
			Action:   "test",
		}
		err := manager.ValidateACPRequest(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server ID is required")
	})

	t.Run("missing action", func(t *testing.T) {
		req := ACPRequest{
			ServerID: "opencode-1",
			Action:   "",
		}
		err := manager.ValidateACPRequest(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "action is required")
	})

	t.Run("non-existent server", func(t *testing.T) {
		req := ACPRequest{
			ServerID: "non-existent",
			Action:   "test",
		}
		err := manager.ValidateACPRequest(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid server ID")
	})
}

func TestACPManager_SyncACPServer(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	// SyncACPServer currently just logs
	err := manager.SyncACPServer(ctx, "opencode-1")
	assert.NoError(t, err)
}

func TestACPManager_GetACPStats(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	stats, err := manager.GetACPStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Contains(t, stats, "totalServers")
	assert.Contains(t, stats, "enabledServers")
	assert.Contains(t, stats, "totalCapabilities")
	assert.Contains(t, stats, "lastSync")

	totalServers := stats["totalServers"].(int)
	assert.Greater(t, totalServers, 0)
}

// Test ACP types
func TestACPServer_Structure(t *testing.T) {
	server := ACPServer{
		ID:      "test-server",
		Name:    "Test Server",
		URL:     "ws://localhost:8080/agent",
		Enabled: true,
		Version: "1.0.0",
		Capabilities: []ACPCapability{
			{Name: "test", Description: "Test capability"},
		},
	}

	assert.Equal(t, "test-server", server.ID)
	assert.Equal(t, "Test Server", server.Name)
	assert.True(t, server.Enabled)
	assert.Len(t, server.Capabilities, 1)
}

func TestACPCapability_Structure(t *testing.T) {
	cap := ACPCapability{
		Name:        "code_execution",
		Description: "Execute code and return results",
		Parameters: map[string]interface{}{
			"language": "string",
			"code":     "string",
		},
	}

	assert.Equal(t, "code_execution", cap.Name)
	assert.Equal(t, "Execute code and return results", cap.Description)
	assert.NotNil(t, cap.Parameters)
}

func TestACPRequest_Structure(t *testing.T) {
	req := ACPRequest{
		ServerID: "opencode-1",
		Action:   "code_execution",
		Parameters: map[string]interface{}{
			"language": "go",
			"code":     "fmt.Println()",
		},
	}

	assert.Equal(t, "opencode-1", req.ServerID)
	assert.Equal(t, "code_execution", req.Action)
	assert.NotNil(t, req.Parameters)
}

func TestACPResponse_Structure(t *testing.T) {
	resp := ACPResponse{
		Success: true,
		Data:    map[string]string{"result": "success"},
		Error:   "",
	}

	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data)
	assert.Empty(t, resp.Error)
}

// Benchmarks
func BenchmarkACPManager_ListACPServers(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ListACPServers(ctx)
	}
}

func BenchmarkACPManager_GetACPServer(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GetACPServer(ctx, "opencode-1")
	}
}

func BenchmarkACPManager_GetACPStats(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GetACPStats(ctx)
	}
}
