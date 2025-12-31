package services

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLSPTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewLSPManager(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	require.NotNil(t, manager)
	assert.Nil(t, manager.repo)
	assert.Nil(t, manager.cache)
	assert.NotNil(t, manager.log)
}

func TestLSPManager_ListLSPServers(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	servers, err := manager.ListLSPServers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, servers)

	// Verify some expected servers
	serverIDs := make(map[string]bool)
	for _, server := range servers {
		serverIDs[server.ID] = true
	}

	assert.True(t, serverIDs["gopls"], "should have gopls server")
	assert.True(t, serverIDs["rust-analyzer"], "should have rust-analyzer server")
	assert.True(t, serverIDs["pylsp"], "should have pylsp server")
	assert.True(t, serverIDs["ts-language-server"], "should have ts-language-server")
}

func TestLSPManager_GetLSPServer(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("get existing server", func(t *testing.T) {
		server, err := manager.GetLSPServer(ctx, "gopls")
		require.NoError(t, err)
		require.NotNil(t, server)
		assert.Equal(t, "gopls", server.ID)
		assert.Equal(t, "Go Language Server", server.Name)
		assert.Equal(t, "go", server.Language)
		assert.True(t, server.Enabled)
	})

	t.Run("get non-existent server", func(t *testing.T) {
		server, err := manager.GetLSPServer(ctx, "non-existent")
		assert.Error(t, err)
		assert.Nil(t, server)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestLSPManager_ExecuteLSPRequest(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	req := LSPRequest{
		ServerID: "gopls",
		Method:   "textDocument/completion",
		Params: map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///test.go"},
		},
	}

	response, err := manager.ExecuteLSPRequest(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.Success)
	assert.NotNil(t, response.Result)
	assert.False(t, response.Timestamp.IsZero())
}

func TestLSPManager_GetDiagnostics(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	diagnostics, err := manager.GetDiagnostics(ctx, "gopls", "file:///test.go")
	require.NoError(t, err)
	require.NotNil(t, diagnostics)

	diagMap, ok := diagnostics.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "gopls", diagMap["serverId"])
	assert.Equal(t, "file:///test.go", diagMap["fileUri"])
	assert.NotNil(t, diagMap["diagnostics"])
}

func TestLSPManager_GetCodeActions(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	position := LSPPosition{Line: 10, Character: 5}
	actions, err := manager.GetCodeActions(ctx, "gopls", "some code", "file:///test.go", position)
	require.NoError(t, err)
	require.NotNil(t, actions)

	actionsMap, ok := actions.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "gopls", actionsMap["serverId"])
	assert.NotNil(t, actionsMap["actions"])
}

func TestLSPManager_GetCompletion(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	position := LSPPosition{Line: 5, Character: 10}
	completions, err := manager.GetCompletion(ctx, "gopls", "fmt.", "file:///test.go", position)
	require.NoError(t, err)
	require.NotNil(t, completions)

	compMap, ok := completions.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "gopls", compMap["serverId"])
	assert.NotNil(t, compMap["completions"])
}

func TestLSPManager_ValidateLSPRequest(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("valid request", func(t *testing.T) {
		req := LSPRequest{
			ServerID: "gopls",
			Method:   "textDocument/completion",
		}
		err := manager.ValidateLSPRequest(ctx, req)
		assert.NoError(t, err)
	})

	t.Run("missing server ID", func(t *testing.T) {
		req := LSPRequest{
			ServerID: "",
			Method:   "textDocument/completion",
		}
		err := manager.ValidateLSPRequest(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server ID is required")
	})

	t.Run("missing method", func(t *testing.T) {
		req := LSPRequest{
			ServerID: "gopls",
			Method:   "",
		}
		err := manager.ValidateLSPRequest(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "method is required")
	})

	t.Run("non-existent server", func(t *testing.T) {
		req := LSPRequest{
			ServerID: "non-existent-server",
			Method:   "textDocument/completion",
		}
		err := manager.ValidateLSPRequest(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid server ID")
	})
}

func TestLSPManager_GetLSPStats(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	stats, err := manager.GetLSPStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Contains(t, stats, "totalServers")
	assert.Contains(t, stats, "enabledServers")
	assert.Contains(t, stats, "totalCapabilities")
	assert.Contains(t, stats, "lastSync")

	totalServers := stats["totalServers"].(int)
	assert.Greater(t, totalServers, 0)

	enabledServers := stats["enabledServers"].(int)
	assert.GreaterOrEqual(t, enabledServers, 0)
	assert.LessOrEqual(t, enabledServers, totalServers)
}

func TestLSPManager_RefreshAllLSPServers(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	err := manager.RefreshAllLSPServers(ctx)
	assert.NoError(t, err)
}

func TestLSPManager_SyncLSPServer(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("sync existing server", func(t *testing.T) {
		err := manager.SyncLSPServer(ctx, "gopls")
		assert.NoError(t, err)
	})

	t.Run("sync non-existent server", func(t *testing.T) {
		err := manager.SyncLSPServer(ctx, "non-existent")
		assert.Error(t, err)
	})
}

// Test LSP types
func TestLSPServer_Structure(t *testing.T) {
	server := LSPServer{
		ID:        "test-server",
		Name:      "Test Language Server",
		Language:  "test",
		Command:   "test-lsp",
		Enabled:   true,
		Workspace: "/workspace",
		Capabilities: []LSPCapability{
			{Name: "completion", Description: "Code completion"},
			{Name: "hover", Description: "Hover information"},
		},
	}

	assert.Equal(t, "test-server", server.ID)
	assert.Equal(t, "Test Language Server", server.Name)
	assert.Equal(t, "test", server.Language)
	assert.True(t, server.Enabled)
	assert.Len(t, server.Capabilities, 2)
}

func TestLSPCapability_Structure(t *testing.T) {
	cap := LSPCapability{
		Name:        "completion",
		Description: "Provides code completion",
	}

	assert.Equal(t, "completion", cap.Name)
	assert.Equal(t, "Provides code completion", cap.Description)
}

func TestLSPRequest_Structure(t *testing.T) {
	req := LSPRequest{
		ServerID: "gopls",
		Method:   "textDocument/completion",
		Params: map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///test.go"},
		},
		Text:    "package main",
		FileURI: "file:///test.go",
		Position: LSPPosition{
			Line:      10,
			Character: 5,
		},
	}

	assert.Equal(t, "gopls", req.ServerID)
	assert.Equal(t, "textDocument/completion", req.Method)
	assert.Equal(t, 10, req.Position.Line)
	assert.Equal(t, 5, req.Position.Character)
}

func TestLSPResponse_Structure(t *testing.T) {
	resp := LSPResponse{
		Success: true,
		Result:  map[string]string{"message": "success"},
		Error:   "",
	}

	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Result)
	assert.Empty(t, resp.Error)
}

func TestLSPPosition_Structure(t *testing.T) {
	pos := LSPPosition{
		Line:      100,
		Character: 25,
	}

	assert.Equal(t, 100, pos.Line)
	assert.Equal(t, 25, pos.Character)
}

// Benchmarks
func BenchmarkLSPManager_ListLSPServers(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ListLSPServers(ctx)
	}
}

func BenchmarkLSPManager_GetLSPServer(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GetLSPServer(ctx, "gopls")
	}
}

func BenchmarkLSPManager_GetLSPStats(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GetLSPStats(ctx)
	}
}
