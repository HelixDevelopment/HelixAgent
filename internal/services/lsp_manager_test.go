package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/database"
)

func newLSPTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

// TestNewLSPManager tests LSP manager creation
func TestNewLSPManager(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	require.NotNil(t, manager)
	assert.Nil(t, manager.repo)
	assert.Nil(t, manager.cache)
	assert.NotNil(t, manager.log)
	assert.NotNil(t, manager.connections)
	assert.NotNil(t, manager.servers)
	assert.NotNil(t, manager.config)
}

// TestNewLSPManagerWithConfig tests LSP manager creation with custom config
func TestNewLSPManagerWithConfig(t *testing.T) {
	log := newLSPTestLogger()

	t.Run("with custom config", func(t *testing.T) {
		config := &LSPConfig{
			ServerConfigs:     make(map[string]LSPServerConfig),
			DefaultWorkspace:  "/custom/workspace",
			RequestTimeout:    60 * time.Second,
			InitTimeout:       20 * time.Second,
			EnableCaching:     false,
			BinarySearchPaths: []string{"/custom/bin"},
		}
		manager := NewLSPManagerWithConfig(nil, nil, log, config)

		require.NotNil(t, manager)
		assert.Equal(t, "/custom/workspace", manager.config.DefaultWorkspace)
		assert.Equal(t, 60*time.Second, manager.config.RequestTimeout)
		assert.False(t, manager.config.EnableCaching)
	})

	t.Run("with nil config uses default", func(t *testing.T) {
		manager := NewLSPManagerWithConfig(nil, nil, log, nil)

		require.NotNil(t, manager)
		assert.Equal(t, "/workspace", manager.config.DefaultWorkspace)
		assert.True(t, manager.config.EnableCaching)
	})

	t.Run("with server config overrides", func(t *testing.T) {
		config := &LSPConfig{
			ServerConfigs: map[string]LSPServerConfig{
				"gopls": {
					Command:      "/custom/gopls",
					Args:         []string{"--debug"},
					WorkspaceDir: "/custom/go/workspace",
					Enabled:      false,
				},
			},
			DefaultWorkspace:  "/workspace",
			RequestTimeout:    30 * time.Second,
			InitTimeout:       10 * time.Second,
			BinarySearchPaths: []string{"/usr/bin"},
		}
		manager := NewLSPManagerWithConfig(nil, nil, log, config)

		require.NotNil(t, manager)
		server, err := manager.GetLSPServer(context.Background(), "gopls")
		require.NoError(t, err)
		assert.Equal(t, "/custom/gopls", server.Command)
		assert.Equal(t, []string{"--debug"}, server.Args)
		assert.Equal(t, "/custom/go/workspace", server.Workspace)
		assert.False(t, server.Enabled)
	})
}

// TestDefaultLSPConfig tests default configuration
func TestDefaultLSPConfig(t *testing.T) {
	config := DefaultLSPConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "/workspace", config.DefaultWorkspace)
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
	assert.Equal(t, 10*time.Second, config.InitTimeout)
	assert.True(t, config.EnableCaching)
	assert.Contains(t, config.BinarySearchPaths, "/usr/bin")
	assert.Contains(t, config.BinarySearchPaths, "/usr/local/bin")

	// Check server configs
	assert.Contains(t, config.ServerConfigs, "gopls")
	assert.Contains(t, config.ServerConfigs, "rust-analyzer")
	assert.Contains(t, config.ServerConfigs, "pylsp")
	assert.Contains(t, config.ServerConfigs, "ts-language-server")
}

// TestLSPManager_ListLSPServers tests listing LSP servers
func TestLSPManager_ListLSPServers(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	servers, err := manager.ListLSPServers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, servers)

	// Verify expected servers
	serverIDs := make(map[string]bool)
	for _, server := range servers {
		serverIDs[server.ID] = true
	}

	assert.True(t, serverIDs["gopls"], "should have gopls server")
	assert.True(t, serverIDs["rust-analyzer"], "should have rust-analyzer server")
	assert.True(t, serverIDs["pylsp"], "should have pylsp server")
	assert.True(t, serverIDs["ts-language-server"], "should have ts-language-server")
}

// TestLSPManager_GetLSPServer tests getting a specific server
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
		assert.NotEmpty(t, server.Capabilities)
	})

	t.Run("get non-existent server", func(t *testing.T) {
		server, err := manager.GetLSPServer(ctx, "non-existent")
		assert.Error(t, err)
		assert.Nil(t, server)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("get all predefined servers", func(t *testing.T) {
		serverIDs := []string{"gopls", "rust-analyzer", "pylsp", "ts-language-server"}
		for _, id := range serverIDs {
			server, err := manager.GetLSPServer(ctx, id)
			require.NoError(t, err, "Failed to get server %s", id)
			require.NotNil(t, server)
			assert.Equal(t, id, server.ID)
		}
	})
}

// TestLSPManager_ExecuteLSPRequest tests executing LSP requests
func TestLSPManager_ExecuteLSPRequest(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("request with unavailable server", func(t *testing.T) {
		req := LSPRequest{
			ServerID: "gopls",
			Method:   "textDocument/completion",
			Params: map[string]interface{}{
				"textDocument": map[string]string{"uri": "file:///test.go"},
			},
		}

		response, err := manager.ExecuteLSPRequest(ctx, req)
		require.NoError(t, err) // Returns response with error, not error itself
		require.NotNil(t, response)
		// Server likely unavailable in test environment
		assert.False(t, response.Timestamp.IsZero())
	})

	t.Run("request with non-existent server", func(t *testing.T) {
		req := LSPRequest{
			ServerID: "non-existent-server",
			Method:   "textDocument/completion",
			Params:   map[string]interface{}{},
		}

		response, err := manager.ExecuteLSPRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "unavailable")
	})
}

// TestLSPManager_GetDiagnostics tests getting diagnostics
func TestLSPManager_GetDiagnostics(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	result, err := manager.GetDiagnostics(ctx, "gopls", "file:///test.go")
	require.NoError(t, err)
	require.NotNil(t, result)

	diagMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "gopls", diagMap["serverId"])
	assert.Equal(t, "file:///test.go", diagMap["fileUri"])
	assert.NotNil(t, diagMap["timestamp"])
}

// TestLSPManager_GetCodeActions tests getting code actions
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
	assert.Equal(t, "file:///test.go", actionsMap["fileUri"])
}

// TestLSPManager_GetCompletion tests getting completions
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
	assert.Equal(t, "file:///test.go", compMap["fileUri"])
}

// TestLSPManager_ValidateLSPRequest tests request validation
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

	t.Run("disabled server", func(t *testing.T) {
		// Create manager with disabled server
		config := &LSPConfig{
			ServerConfigs: map[string]LSPServerConfig{
				"gopls": {
					Command: "gopls",
					Enabled: false,
				},
			},
			DefaultWorkspace:  "/workspace",
			RequestTimeout:    30 * time.Second,
			InitTimeout:       10 * time.Second,
			BinarySearchPaths: []string{"/usr/bin"},
		}
		manager := NewLSPManagerWithConfig(nil, nil, log, config)

		req := LSPRequest{
			ServerID: "gopls",
			Method:   "textDocument/completion",
		}
		err := manager.ValidateLSPRequest(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})
}

// TestLSPManager_GetLSPStats tests getting statistics
func TestLSPManager_GetLSPStats(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	stats, err := manager.GetLSPStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Contains(t, stats, "totalServers")
	assert.Contains(t, stats, "enabledServers")
	assert.Contains(t, stats, "availableServers")
	assert.Contains(t, stats, "connectedServers")
	assert.Contains(t, stats, "totalCapabilities")
	assert.Contains(t, stats, "lastSync")

	totalServers := stats["totalServers"].(int)
	assert.Greater(t, totalServers, 0)

	enabledServers := stats["enabledServers"].(int)
	assert.GreaterOrEqual(t, enabledServers, 0)
	assert.LessOrEqual(t, enabledServers, totalServers)
}

// TestLSPManager_RefreshAllLSPServers tests refreshing all servers
func TestLSPManager_RefreshAllLSPServers(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	err := manager.RefreshAllLSPServers(ctx)
	assert.NoError(t, err)
}

// TestLSPManager_SyncLSPServer tests syncing a server
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

// TestLSPServer_Structure tests LSP server struct
func TestLSPServer_Structure(t *testing.T) {
	now := time.Now()
	server := LSPServer{
		ID:        "test-server",
		Name:      "Test Language Server",
		Language:  "test",
		Command:   "test-lsp",
		Args:      []string{"--debug"},
		Enabled:   true,
		Workspace: "/workspace",
		LastSync:  &now,
		Capabilities: []LSPCapability{
			{Name: "completion", Description: "Code completion"},
			{Name: "hover", Description: "Hover information"},
		},
		BinaryPath: "/usr/bin/test-lsp",
		Available:  true,
	}

	assert.Equal(t, "test-server", server.ID)
	assert.Equal(t, "Test Language Server", server.Name)
	assert.Equal(t, "test", server.Language)
	assert.Equal(t, "test-lsp", server.Command)
	assert.Equal(t, []string{"--debug"}, server.Args)
	assert.True(t, server.Enabled)
	assert.Equal(t, "/workspace", server.Workspace)
	assert.NotNil(t, server.LastSync)
	assert.Len(t, server.Capabilities, 2)
	assert.Equal(t, "/usr/bin/test-lsp", server.BinaryPath)
	assert.True(t, server.Available)
}

// TestLSPCapability_Structure tests LSP capability struct
func TestLSPCapability_Structure(t *testing.T) {
	cap := LSPCapability{
		Name:        "completion",
		Description: "Provides code completion",
	}

	assert.Equal(t, "completion", cap.Name)
	assert.Equal(t, "Provides code completion", cap.Description)
}

// TestLSPRequest_Structure tests LSP request struct
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
	assert.NotNil(t, req.Params)
	assert.Equal(t, "package main", req.Text)
	assert.Equal(t, "file:///test.go", req.FileURI)
	assert.Equal(t, 10, req.Position.Line)
	assert.Equal(t, 5, req.Position.Character)
}

// TestLSPResponse_Structure tests LSP response struct
func TestLSPResponse_Structure(t *testing.T) {
	resp := LSPResponse{
		Success:   true,
		Result:    map[string]string{"message": "success"},
		Error:     "",
		Timestamp: time.Now(),
	}

	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Result)
	assert.Empty(t, resp.Error)
	assert.False(t, resp.Timestamp.IsZero())
}

// TestLSPPosition_Structure tests LSP position struct
func TestLSPPosition_Structure(t *testing.T) {
	pos := LSPPosition{
		Line:      100,
		Character: 25,
	}

	assert.Equal(t, 100, pos.Line)
	assert.Equal(t, 25, pos.Character)
}

// TestLSPJSONRPCRequest_Structure tests JSON-RPC request struct
func TestLSPJSONRPCRequest_Structure(t *testing.T) {
	req := LSPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  map[string]interface{}{"test": "value"},
	}

	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, int64(1), req.ID)
	assert.Equal(t, "initialize", req.Method)
	assert.NotNil(t, req.Params)

	// Test JSON marshaling
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), "jsonrpc")
	assert.Contains(t, string(data), "2.0")
}

// TestLSPJSONRPCResponse_Structure tests JSON-RPC response struct
func TestLSPJSONRPCResponse_Structure(t *testing.T) {
	t.Run("successful response", func(t *testing.T) {
		resp := LSPJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  map[string]interface{}{"data": "test"},
		}

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, int64(1), resp.ID)
		assert.NotNil(t, resp.Result)
		assert.Nil(t, resp.Error)
	})

	t.Run("error response", func(t *testing.T) {
		resp := LSPJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error: &LSPJSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "additional info",
			},
		}

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Nil(t, resp.Result)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, -32600, resp.Error.Code)
		assert.Equal(t, "Invalid Request", resp.Error.Message)
	})
}

// TestLSPJSONRPCNotification_Structure tests JSON-RPC notification struct
func TestLSPJSONRPCNotification_Structure(t *testing.T) {
	notif := LSPJSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  map[string]interface{}{},
	}

	assert.Equal(t, "2.0", notif.JSONRPC)
	assert.Equal(t, "initialized", notif.Method)
	assert.NotNil(t, notif.Params)
}

// TestLSPInitializeParams_Structure tests initialize params struct
func TestLSPInitializeParams_Structure(t *testing.T) {
	params := LSPInitializeParams{
		ProcessID: 12345,
		RootURI:   "file:///workspace",
		Capabilities: LSPClientCapabilities{
			TextDocument: &LSPTextDocumentClientCapabilities{
				Completion: &LSPCompletionClientCapabilities{
					DynamicRegistration: true,
				},
			},
		},
		WorkspaceFolders: []LSPWorkspaceFolder{
			{URI: "file:///workspace", Name: "main"},
		},
	}

	assert.Equal(t, 12345, params.ProcessID)
	assert.Equal(t, "file:///workspace", params.RootURI)
	assert.NotNil(t, params.Capabilities.TextDocument)
	assert.True(t, params.Capabilities.TextDocument.Completion.DynamicRegistration)
	assert.Len(t, params.WorkspaceFolders, 1)
}

// TestLSPClientCapabilities_Structure tests client capabilities structs
func TestLSPClientCapabilities_Structure(t *testing.T) {
	caps := LSPClientCapabilities{
		TextDocument: &LSPTextDocumentClientCapabilities{
			Completion: &LSPCompletionClientCapabilities{
				DynamicRegistration: false,
				CompletionItem: &LSPCompletionItemCapabilities{
					SnippetSupport:          true,
					CommitCharactersSupport: true,
					DocumentationFormat:     []string{"markdown", "plaintext"},
				},
			},
			Hover: &LSPHoverClientCapabilities{
				DynamicRegistration: false,
				ContentFormat:       []string{"markdown"},
			},
			Definition: &LSPDefinitionClientCapabilities{
				DynamicRegistration: false,
				LinkSupport:         true,
			},
			References: &LSPReferencesClientCapabilities{
				DynamicRegistration: false,
			},
			CodeAction: &LSPCodeActionClientCapabilities{
				DynamicRegistration: false,
			},
			Diagnostics: &LSPDiagnosticsClientCapabilities{
				DynamicRegistration: false,
			},
		},
		Workspace: &LSPWorkspaceClientCapabilities{
			ApplyEdit:        true,
			WorkspaceFolders: true,
			Configuration:    true,
		},
	}

	assert.NotNil(t, caps.TextDocument)
	assert.NotNil(t, caps.TextDocument.Completion)
	assert.True(t, caps.TextDocument.Completion.CompletionItem.SnippetSupport)
	assert.NotNil(t, caps.Workspace)
	assert.True(t, caps.Workspace.ApplyEdit)
}

// TestLSPServerCapabilities_Structure tests server capabilities struct
func TestLSPServerCapabilities_Structure(t *testing.T) {
	caps := LSPServerCapabilities{
		CompletionProvider:         true,
		HoverProvider:              true,
		DefinitionProvider:         true,
		ReferencesProvider:         true,
		DiagnosticProvider:         false,
		CodeActionProvider:         true,
		DocumentFormattingProvider: false,
	}

	assert.True(t, caps.CompletionProvider)
	assert.True(t, caps.HoverProvider)
	assert.True(t, caps.DefinitionProvider)
	assert.True(t, caps.ReferencesProvider)
	assert.False(t, caps.DiagnosticProvider)
	assert.True(t, caps.CodeActionProvider)
	assert.False(t, caps.DocumentFormattingProvider)
}

// TestLSPRange_Structure tests LSP range struct
func TestLSPRange_Structure(t *testing.T) {
	r := LSPRange{
		Start: LSPPosition{Line: 10, Character: 5},
		End:   LSPPosition{Line: 10, Character: 15},
	}

	assert.Equal(t, 10, r.Start.Line)
	assert.Equal(t, 5, r.Start.Character)
	assert.Equal(t, 10, r.End.Line)
	assert.Equal(t, 15, r.End.Character)
}

// TestLSPDiagnostic_Structure tests LSP diagnostic struct
func TestLSPDiagnostic_Structure(t *testing.T) {
	diag := LSPDiagnostic{
		Range: LSPRange{
			Start: LSPPosition{Line: 10, Character: 0},
			End:   LSPPosition{Line: 10, Character: 20},
		},
		Severity: 1,
		Code:     "E001",
		Source:   "test-linter",
		Message:  "Test diagnostic message",
	}

	assert.Equal(t, 10, diag.Range.Start.Line)
	assert.Equal(t, 1, diag.Severity)
	assert.Equal(t, "E001", diag.Code)
	assert.Equal(t, "test-linter", diag.Source)
	assert.Equal(t, "Test diagnostic message", diag.Message)
}

// TestLSPTextDocumentItem_Structure tests text document item struct
func TestLSPTextDocumentItem_Structure(t *testing.T) {
	doc := LSPTextDocumentItem{
		URI:        "file:///test.go",
		LanguageID: "go",
		Version:    1,
		Text:       "package main",
	}

	assert.Equal(t, "file:///test.go", doc.URI)
	assert.Equal(t, "go", doc.LanguageID)
	assert.Equal(t, 1, doc.Version)
	assert.Equal(t, "package main", doc.Text)
}

// TestLSPManager_GetHover tests getting hover information
func TestLSPManager_GetHover(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("valid server and uri", func(t *testing.T) {
		result, err := manager.GetHover(ctx, "gopls", "file:///test.go", 10, 5)
		require.NoError(t, err)
		require.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "gopls", resultMap["serverId"])
		assert.Equal(t, "file:///test.go", resultMap["fileUri"])
	})

	t.Run("non-existent server", func(t *testing.T) {
		result, err := manager.GetHover(ctx, "non-existent", "file:///test.go", 10, 5)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("empty uri", func(t *testing.T) {
		result, err := manager.GetHover(ctx, "gopls", "", 10, 5)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "fileURI is required")
	})

	t.Run("different servers", func(t *testing.T) {
		servers := []string{"gopls", "pylsp", "rust-analyzer", "ts-language-server"}
		for _, serverID := range servers {
			result, err := manager.GetHover(ctx, serverID, "file:///main.test", 0, 0)
			require.NoError(t, err, "Failed for server %s", serverID)
			require.NotNil(t, result)
		}
	})
}

// TestLSPManager_GetDefinition tests getting definition location
func TestLSPManager_GetDefinition(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("valid server and uri", func(t *testing.T) {
		result, err := manager.GetDefinition(ctx, "gopls", "file:///test.go", 10, 5)
		require.NoError(t, err)
		require.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "gopls", resultMap["serverId"])
		assert.Equal(t, "file:///test.go", resultMap["fileUri"])
	})

	t.Run("non-existent server", func(t *testing.T) {
		result, err := manager.GetDefinition(ctx, "non-existent", "file:///test.go", 10, 5)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("empty uri", func(t *testing.T) {
		result, err := manager.GetDefinition(ctx, "gopls", "", 10, 5)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "fileURI is required")
	})

	t.Run("rust-analyzer server", func(t *testing.T) {
		result, err := manager.GetDefinition(ctx, "rust-analyzer", "file:///lib.rs", 5, 10)
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// TestLSPManager_GetReferences tests getting references
func TestLSPManager_GetReferences(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("valid server and uri", func(t *testing.T) {
		result, err := manager.GetReferences(ctx, "gopls", "file:///test.go", 10, 5)
		require.NoError(t, err)
		require.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "gopls", resultMap["serverId"])
		assert.Equal(t, "file:///test.go", resultMap["fileUri"])
	})

	t.Run("non-existent server", func(t *testing.T) {
		result, err := manager.GetReferences(ctx, "non-existent", "file:///test.go", 10, 5)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("empty uri", func(t *testing.T) {
		result, err := manager.GetReferences(ctx, "gopls", "", 10, 5)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "fileURI is required")
	})

	t.Run("ts-language-server", func(t *testing.T) {
		result, err := manager.GetReferences(ctx, "ts-language-server", "file:///app.ts", 20, 15)
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// MockCacheWithInvalidate implements CacheInterface with InvalidateByPattern
type MockCacheWithInvalidate struct {
	invalidateError error
	invalidateCalls int
	mu              sync.Mutex
}

func (m *MockCacheWithInvalidate) Get(ctx context.Context, key string) (*database.ModelMetadata, bool, error) {
	return nil, false, nil
}

func (m *MockCacheWithInvalidate) Set(ctx context.Context, key string, value *database.ModelMetadata) error {
	return nil
}

func (m *MockCacheWithInvalidate) Delete(ctx context.Context, key string) error {
	return nil
}

func (m *MockCacheWithInvalidate) GetBulk(ctx context.Context, keys []string) (map[string]*database.ModelMetadata, error) {
	return nil, nil
}

func (m *MockCacheWithInvalidate) SetBulk(ctx context.Context, items map[string]*database.ModelMetadata) error {
	return nil
}

func (m *MockCacheWithInvalidate) Clear(ctx context.Context) error {
	return nil
}

func (m *MockCacheWithInvalidate) Size(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *MockCacheWithInvalidate) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockCacheWithInvalidate) GetProviderModels(ctx context.Context, provider string) ([]*database.ModelMetadata, error) {
	return nil, nil
}

func (m *MockCacheWithInvalidate) SetProviderModels(ctx context.Context, provider string, models []*database.ModelMetadata) error {
	return nil
}

func (m *MockCacheWithInvalidate) DeleteProviderModels(ctx context.Context, provider string) error {
	return nil
}

func (m *MockCacheWithInvalidate) GetByCapability(ctx context.Context, capability string) ([]*database.ModelMetadata, error) {
	return nil, nil
}

func (m *MockCacheWithInvalidate) SetByCapability(ctx context.Context, capability string, models []*database.ModelMetadata) error {
	return nil
}

func (m *MockCacheWithInvalidate) InvalidateByPattern(ctx context.Context, pattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invalidateCalls++
	return m.invalidateError
}

func (m *MockCacheWithInvalidate) GetInvalidateCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.invalidateCalls
}

// TestLSPManager_RefreshAllLSPServers_WithCache tests refresh with cache
func TestLSPManager_RefreshAllLSPServers_WithCache(t *testing.T) {
	log := newLSPTestLogger()
	ctx := context.Background()

	t.Run("with cache that implements InvalidateByPattern", func(t *testing.T) {
		mockCache := &MockCacheWithInvalidate{}
		manager := NewLSPManager(nil, mockCache, log)

		err := manager.RefreshAllLSPServers(ctx)
		require.NoError(t, err)

		// Should have called InvalidateByPattern for each enabled server
		servers, _ := manager.ListLSPServers(ctx)
		enabledCount := 0
		for _, s := range servers {
			if s.Enabled {
				enabledCount++
			}
		}
		assert.Equal(t, enabledCount, mockCache.GetInvalidateCalls())
	})

	t.Run("with cache that fails InvalidateByPattern", func(t *testing.T) {
		mockCache := &MockCacheWithInvalidate{
			invalidateError: errors.New("cache invalidation failed"),
		}
		manager := NewLSPManager(nil, mockCache, log)

		// RefreshAllLSPServers should still succeed even if cache invalidation fails
		err := manager.RefreshAllLSPServers(ctx)
		require.NoError(t, err)
	})

	t.Run("with nil cache", func(t *testing.T) {
		manager := NewLSPManager(nil, nil, log)

		err := manager.RefreshAllLSPServers(ctx)
		require.NoError(t, err)
	})
}

// TestLSPManager_Close tests closing connections
func TestLSPManager_Close(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	t.Run("close non-existent connection", func(t *testing.T) {
		err := manager.Close("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no connection")
	})
}

// TestLSPManager_CloseAll tests closing all connections
func TestLSPManager_CloseAll(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	// Should not error when no connections
	err := manager.CloseAll()
	assert.NoError(t, err)
}

// TestLSPManager_GetConfig tests getting configuration
func TestLSPManager_GetConfig(t *testing.T) {
	log := newLSPTestLogger()
	config := &LSPConfig{
		DefaultWorkspace: "/custom",
		RequestTimeout:   60 * time.Second,
	}
	manager := NewLSPManagerWithConfig(nil, nil, log, config)

	retrievedConfig := manager.GetConfig()
	assert.Equal(t, "/custom", retrievedConfig.DefaultWorkspace)
	assert.Equal(t, 60*time.Second, retrievedConfig.RequestTimeout)
}

// TestLSPManager_AddServer tests adding custom servers
func TestLSPManager_AddServer(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	customServer := &LSPServer{
		ID:        "custom-lsp",
		Name:      "Custom Language Server",
		Language:  "custom",
		Command:   "custom-lsp",
		Enabled:   true,
		Available: true,
	}

	manager.AddServer(customServer)

	server, err := manager.GetLSPServer(ctx, "custom-lsp")
	require.NoError(t, err)
	assert.Equal(t, "custom-lsp", server.ID)
	assert.Equal(t, "Custom Language Server", server.Name)
}

// TestLSPManager_GetConnection tests getting connections
func TestLSPManager_GetConnection(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	// No connection should exist initially
	conn := manager.GetConnection("gopls")
	assert.Nil(t, conn)
}

// TestLSPConnection_IsConnected tests connection status
func TestLSPConnection_IsConnected(t *testing.T) {
	conn := &LSPConnection{
		connected: true,
	}

	assert.True(t, conn.IsConnected())

	conn.connected = false
	assert.False(t, conn.IsConnected())
}

// TestLSPConnection_Close tests closing a connection
func TestLSPConnection_Close(t *testing.T) {
	t.Run("close with nil fields", func(t *testing.T) {
		conn := &LSPConnection{
			connected: true,
		}

		err := conn.Close()
		assert.NoError(t, err)
		assert.False(t, conn.connected)
	})
}

// MockWriteCloser implements io.WriteCloser for testing
type MockWriteCloser struct {
	bytes.Buffer
	closed     bool
	closeError error
}

func (m *MockWriteCloser) Close() error {
	m.closed = true
	return m.closeError
}

// MockReadCloser implements io.ReadCloser for testing
type MockReadCloser struct {
	*bytes.Reader
	closed     bool
	closeError error
}

func (m *MockReadCloser) Close() error {
	m.closed = true
	return m.closeError
}

// TestLSPConnection_Close_WithIO tests closing with IO streams
func TestLSPConnection_Close_WithIO(t *testing.T) {
	stdin := &MockWriteCloser{}
	stdout := &MockReadCloser{Reader: bytes.NewReader([]byte{})}
	stderr := &MockReadCloser{Reader: bytes.NewReader([]byte{})}

	conn := &LSPConnection{
		connected: true,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
	}

	err := conn.Close()
	assert.NoError(t, err)
	assert.False(t, conn.connected)
	assert.True(t, stdin.closed)
	assert.True(t, stdout.closed)
	assert.True(t, stderr.closed)
}

// TestLSPConnection_Close_WithErrors tests closing with errors
func TestLSPConnection_Close_WithErrors(t *testing.T) {
	stdin := &MockWriteCloser{closeError: errors.New("stdin close error")}
	stdout := &MockReadCloser{Reader: bytes.NewReader([]byte{}), closeError: errors.New("stdout close error")}

	conn := &LSPConnection{
		connected: true,
		stdin:     stdin,
		stdout:    stdout,
	}

	err := conn.Close()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "errors during close")
}

// TestLSPManager_SetConnection tests setting connections
func TestLSPManager_SetConnection(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	conn := &LSPConnection{
		ServerID:  "test-server",
		connected: true,
	}

	manager.SetConnection("test-server", conn)

	retrievedConn := manager.GetConnection("test-server")
	assert.NotNil(t, retrievedConn)
	assert.Equal(t, "test-server", retrievedConn.ServerID)
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

func BenchmarkLSPManager_ValidateLSPRequest(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	req := LSPRequest{
		ServerID: "gopls",
		Method:   "textDocument/completion",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.ValidateLSPRequest(ctx, req)
	}
}

// TestLSPServerConfig_Structure tests server config struct
func TestLSPServerConfig_Structure(t *testing.T) {
	config := LSPServerConfig{
		Command:      "/usr/bin/gopls",
		Args:         []string{"serve", "--debug"},
		RootURI:      "file:///workspace",
		WorkspaceDir: "/workspace",
		Enabled:      true,
	}

	assert.Equal(t, "/usr/bin/gopls", config.Command)
	assert.Equal(t, []string{"serve", "--debug"}, config.Args)
	assert.Equal(t, "file:///workspace", config.RootURI)
	assert.Equal(t, "/workspace", config.WorkspaceDir)
	assert.True(t, config.Enabled)
}

// TestLSPConfig_Structure tests config struct
func TestLSPConfig_Structure(t *testing.T) {
	config := LSPConfig{
		ServerConfigs: map[string]LSPServerConfig{
			"test": {Command: "test-lsp"},
		},
		DefaultWorkspace:  "/test/workspace",
		RequestTimeout:    45 * time.Second,
		InitTimeout:       15 * time.Second,
		EnableCaching:     false,
		BinarySearchPaths: []string{"/custom/bin"},
	}

	assert.Len(t, config.ServerConfigs, 1)
	assert.Equal(t, "/test/workspace", config.DefaultWorkspace)
	assert.Equal(t, 45*time.Second, config.RequestTimeout)
	assert.Equal(t, 15*time.Second, config.InitTimeout)
	assert.False(t, config.EnableCaching)
	assert.Contains(t, config.BinarySearchPaths, "/custom/bin")
}

// TestLSPInitializeResult_Structure tests initialize result struct
func TestLSPInitializeResult_Structure(t *testing.T) {
	result := LSPInitializeResult{
		Capabilities: LSPServerCapabilitiesResult{
			CompletionProvider: map[string]interface{}{"triggerCharacters": []string{"."}},
			HoverProvider:      true,
		},
		ServerInfo: &LSPServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		},
	}

	assert.NotNil(t, result.Capabilities.CompletionProvider)
	assert.Equal(t, true, result.Capabilities.HoverProvider)
	assert.NotNil(t, result.ServerInfo)
	assert.Equal(t, "test-server", result.ServerInfo.Name)
	assert.Equal(t, "1.0.0", result.ServerInfo.Version)
}

// TestLSPTextDocumentIdentifier_Structure tests text document identifier
func TestLSPTextDocumentIdentifier_Structure(t *testing.T) {
	id := LSPTextDocumentIdentifier{
		URI: "file:///test.go",
	}

	assert.Equal(t, "file:///test.go", id.URI)

	// Test JSON marshaling
	data, err := json.Marshal(id)
	require.NoError(t, err)
	assert.Contains(t, string(data), "file:///test.go")
}

// TestLSPTextDocumentPositionParams_Structure tests position params
func TestLSPTextDocumentPositionParams_Structure(t *testing.T) {
	params := LSPTextDocumentPositionParams{
		TextDocument: LSPTextDocumentIdentifier{URI: "file:///test.go"},
		Position:     LSPPosition{Line: 10, Character: 5},
	}

	assert.Equal(t, "file:///test.go", params.TextDocument.URI)
	assert.Equal(t, 10, params.Position.Line)
	assert.Equal(t, 5, params.Position.Character)
}

// TestLSPDidOpenTextDocumentParams_Structure tests did open params
func TestLSPDidOpenTextDocumentParams_Structure(t *testing.T) {
	params := LSPDidOpenTextDocumentParams{
		TextDocument: LSPTextDocumentItem{
			URI:        "file:///test.go",
			LanguageID: "go",
			Version:    1,
			Text:       "package main",
		},
	}

	assert.Equal(t, "file:///test.go", params.TextDocument.URI)
	assert.Equal(t, "go", params.TextDocument.LanguageID)
	assert.Equal(t, 1, params.TextDocument.Version)
	assert.Equal(t, "package main", params.TextDocument.Text)
}

// TestLSPWorkspaceFolder_Structure tests workspace folder struct
func TestLSPWorkspaceFolder_Structure(t *testing.T) {
	folder := LSPWorkspaceFolder{
		URI:  "file:///workspace",
		Name: "main",
	}

	assert.Equal(t, "file:///workspace", folder.URI)
	assert.Equal(t, "main", folder.Name)
}

// TestLSPServerCapabilitiesResult_Structure tests server capabilities result
func TestLSPServerCapabilitiesResult_Structure(t *testing.T) {
	caps := LSPServerCapabilitiesResult{
		CompletionProvider:         true,
		HoverProvider:              true,
		DefinitionProvider:         map[string]interface{}{},
		ReferencesProvider:         true,
		DiagnosticProvider:         nil,
		CodeActionProvider:         true,
		DocumentFormattingProvider: false,
		TextDocumentSync:           1,
	}

	assert.Equal(t, true, caps.CompletionProvider)
	assert.Equal(t, true, caps.HoverProvider)
	assert.NotNil(t, caps.DefinitionProvider)
	assert.Nil(t, caps.DiagnosticProvider)
	assert.Equal(t, 1, caps.TextDocumentSync)
}

// TestLSPJSONRPCError_Structure tests JSON-RPC error struct
func TestLSPJSONRPCError_Structure(t *testing.T) {
	err := LSPJSONRPCError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    map[string]string{"detail": "missing field"},
	}

	assert.Equal(t, -32600, err.Code)
	assert.Equal(t, "Invalid Request", err.Message)
	assert.NotNil(t, err.Data)
}

// TestLSPManager_RefreshWithDisabledServers tests refresh skips disabled servers
func TestLSPManager_RefreshWithDisabledServers(t *testing.T) {
	log := newLSPTestLogger()
	config := &LSPConfig{
		ServerConfigs: map[string]LSPServerConfig{
			"gopls": {
				Command: "gopls",
				Enabled: false,
			},
			"pylsp": {
				Command: "pylsp",
				Enabled: true,
			},
		},
		DefaultWorkspace:  "/workspace",
		RequestTimeout:    30 * time.Second,
		InitTimeout:       10 * time.Second,
		BinarySearchPaths: []string{"/usr/bin"},
	}
	manager := NewLSPManagerWithConfig(nil, nil, log, config)
	ctx := context.Background()

	err := manager.RefreshAllLSPServers(ctx)
	assert.NoError(t, err)
}

// TestLSPManager_NextMessageID tests message ID generation
func TestLSPManager_NextMessageID(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	id1 := manager.nextMessageID()
	id2 := manager.nextMessageID()
	id3 := manager.nextMessageID()

	assert.Equal(t, int64(1), id1)
	assert.Equal(t, int64(2), id2)
	assert.Equal(t, int64(3), id3)
}

// TestLSPManager_ConcurrentAccess tests thread safety
func TestLSPManager_ConcurrentAccess(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = manager.ListLSPServers(ctx)
			_, _ = manager.GetLSPServer(ctx, "gopls")
			_, _ = manager.GetLSPStats(ctx)
		}()
	}

	wg.Wait()
}

// TestLSPManager_ServerAvailability tests server availability detection
func TestLSPManager_ServerAvailability(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	servers, err := manager.ListLSPServers(ctx)
	require.NoError(t, err)

	// All servers should have the Available field set based on binary detection
	for _, server := range servers {
		// The Available field should be set (either true or false)
		// We just verify the field exists and the server has proper configuration
		assert.NotEmpty(t, server.ID)
		assert.NotEmpty(t, server.Command)
	}
}

// TestLSPManager_ExecuteWithMockConnection tests execution with mock connection
func TestLSPManager_ExecuteWithMockConnection(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	// Add a test server that's available
	testServer := &LSPServer{
		ID:        "test-mock",
		Name:      "Test Mock Server",
		Language:  "test",
		Command:   "echo", // Use a command that exists
		Enabled:   true,
		Available: false, // But mark as unavailable
	}
	manager.AddServer(testServer)

	// Execute request should handle unavailable server gracefully
	req := LSPRequest{
		ServerID: "test-mock",
		Method:   "test/method",
		Params:   map[string]interface{}{},
	}

	response, err := manager.ExecuteLSPRequest(ctx, req)
	require.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "unavailable")
}

// TestLSPManager_RefreshUpdatesAvailability tests that refresh updates availability
func TestLSPManager_RefreshUpdatesAvailability(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	// Get initial state
	server1, err := manager.GetLSPServer(ctx, "gopls")
	require.NoError(t, err)
	initialAvailable := server1.Available

	// Refresh
	err = manager.SyncLSPServer(ctx, "gopls")
	require.NoError(t, err)

	// Get updated state (availability might be same, but LastSync should be updated)
	server2, err := manager.GetLSPServer(ctx, "gopls")
	require.NoError(t, err)

	// Availability might be the same, but LastSync should be set after refresh
	assert.Equal(t, initialAvailable, server2.Available)
	if server2.LastSync != nil {
		assert.False(t, server2.LastSync.IsZero())
	}
}

// TestLSPManager_RefreshWithDeadConnection tests refresh with dead connections
func TestLSPManager_RefreshWithDeadConnection(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	// Add a dead connection
	deadConn := &LSPConnection{
		ServerID:  "gopls",
		connected: false,
	}
	manager.SetConnection("gopls", deadConn)

	// Refresh should clean up dead connection
	err := manager.SyncLSPServer(ctx, "gopls")
	require.NoError(t, err)

	// Dead connection should be removed
	conn := manager.GetConnection("gopls")
	assert.Nil(t, conn)
}

// TestLSPServerInfo_Structure tests server info struct
func TestLSPServerInfo_Structure(t *testing.T) {
	info := LSPServerInfo{
		Name:    "gopls",
		Version: "0.14.0",
	}

	assert.Equal(t, "gopls", info.Name)
	assert.Equal(t, "0.14.0", info.Version)

	// Test with empty version
	info2 := LSPServerInfo{
		Name: "test-server",
	}
	assert.Equal(t, "test-server", info2.Name)
	assert.Empty(t, info2.Version)
}

// TestLSPManager_GetOrCreateConnection_ServerNotFound tests connection creation errors
func TestLSPManager_GetOrCreateConnection_ServerNotFound(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	// Try to get connection for non-existent server
	req := LSPRequest{
		ServerID: "non-existent",
		Method:   "test/method",
	}

	response, err := manager.ExecuteLSPRequest(ctx, req)
	require.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "not found")
}

// TestLSPManager_GetOrCreateConnection_DisabledServer tests disabled server handling
func TestLSPManager_GetOrCreateConnection_DisabledServer(t *testing.T) {
	log := newLSPTestLogger()
	config := &LSPConfig{
		ServerConfigs: map[string]LSPServerConfig{
			"gopls": {
				Command: "gopls",
				Enabled: false,
			},
		},
		DefaultWorkspace:  "/workspace",
		RequestTimeout:    30 * time.Second,
		InitTimeout:       10 * time.Second,
		BinarySearchPaths: []string{"/usr/bin"},
	}
	manager := NewLSPManagerWithConfig(nil, nil, log, config)
	ctx := context.Background()

	req := LSPRequest{
		ServerID: "gopls",
		Method:   "test/method",
	}

	response, err := manager.ExecuteLSPRequest(ctx, req)
	require.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "not enabled")
}

// TestLSPManager_ContextCancellation tests context cancellation handling
func TestLSPManager_ContextCancellation(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Operations should handle cancelled context gracefully
	_, err := manager.ListLSPServers(ctx)
	// ListLSPServers doesn't use context for its core operation, so it should succeed
	assert.NoError(t, err)
}

// MockLSPServer is a simple mock LSP server for testing
type MockLSPServer struct {
	responses map[string]interface{}
	mu        sync.Mutex
}

func NewMockLSPServer() *MockLSPServer {
	return &MockLSPServer{
		responses: make(map[string]interface{}),
	}
}

func (m *MockLSPServer) SetResponse(method string, response interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[method] = response
}

// TestWriteMessage tests message writing
func TestWriteMessage(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	stdin := &MockWriteCloser{}
	conn := &LSPConnection{
		stdin:     stdin,
		connected: true,
	}

	message := LSPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test/method",
	}

	err := manager.writeMessage(conn, message)
	assert.NoError(t, err)

	// Verify the written content has Content-Length header
	written := stdin.String()
	assert.Contains(t, written, "Content-Length:")
	assert.Contains(t, written, "jsonrpc")
}

// TestReadMessage tests message reading
func TestReadMessage(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	t.Run("valid message", func(t *testing.T) {
		responseJSON := `{"jsonrpc":"2.0","id":1,"result":{"test":"value"}}`
		message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(responseJSON), responseJSON)

		conn := &LSPConnection{
			scanner:   bufio.NewReader(strings.NewReader(message)),
			connected: true,
		}

		response, err := manager.readMessage(conn)
		require.NoError(t, err)
		assert.Equal(t, "2.0", response.JSONRPC)
		assert.Equal(t, int64(1), response.ID)
		assert.NotNil(t, response.Result)
	})

	t.Run("missing content length", func(t *testing.T) {
		message := "\r\n{}"

		conn := &LSPConnection{
			scanner:   bufio.NewReader(strings.NewReader(message)),
			connected: true,
		}

		_, err := manager.readMessage(conn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Content-Length")
	})

	t.Run("invalid json", func(t *testing.T) {
		message := "Content-Length: 5\r\n\r\n{bad}"

		conn := &LSPConnection{
			scanner:   bufio.NewReader(strings.NewReader(message)),
			connected: true,
		}

		_, err := manager.readMessage(conn)
		assert.Error(t, err)
	})
}

// TestSendRequest_ConnectionClosed tests sending on closed connection
func TestSendRequest_ConnectionClosed(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	conn := &LSPConnection{
		connected: false,
	}

	_, err := manager.sendRequest(ctx, conn, "test/method", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection is closed")
}

// TestSendNotification_ConnectionClosed tests notification on closed connection
func TestSendNotification_ConnectionClosed(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	conn := &LSPConnection{
		connected: false,
	}

	err := manager.sendNotification(conn, "test/notification", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection is closed")
}

// Test for fileExists function
func TestFileExists(t *testing.T) {
	// Test with a command that should exist on most systems
	exists := fileExists("go")
	// We don't assert the result since it depends on system configuration
	_ = exists

	// Test with a path that definitely doesn't exist
	exists = fileExists("/definitely/not/a/real/path/binary")
	assert.False(t, exists)
}

// TestFindBinary tests binary finding
func TestFindBinary(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)

	t.Run("absolute path that doesn't exist", func(t *testing.T) {
		path, found := manager.findBinary("/nonexistent/path/binary")
		assert.False(t, found)
		assert.Empty(t, path)
	})

	t.Run("command not in path", func(t *testing.T) {
		path, found := manager.findBinary("definitely-not-a-real-command-xyz")
		assert.False(t, found)
		assert.Empty(t, path)
	})
}

// TestLSPManager_CompleteIntegration tests a complete integration scenario
func TestLSPManager_CompleteIntegration(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	// 1. List servers
	servers, err := manager.ListLSPServers(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, servers)

	// 2. Get specific server
	server, err := manager.GetLSPServer(ctx, "gopls")
	require.NoError(t, err)
	assert.NotNil(t, server)

	// 3. Validate request
	req := LSPRequest{
		ServerID: "gopls",
		Method:   "textDocument/hover",
		FileURI:  "file:///test.go",
		Position: LSPPosition{Line: 10, Character: 5},
	}
	err = manager.ValidateLSPRequest(ctx, req)
	require.NoError(t, err)

	// 4. Execute request (will fail gracefully if server unavailable)
	response, err := manager.ExecuteLSPRequest(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, response)

	// 5. Get stats
	stats, err := manager.GetLSPStats(ctx)
	require.NoError(t, err)
	assert.Contains(t, stats, "totalServers")

	// 6. Refresh
	err = manager.RefreshAllLSPServers(ctx)
	require.NoError(t, err)

	// 7. Cleanup
	err = manager.CloseAll()
	require.NoError(t, err)
}

// TestJSONSerialization tests JSON marshaling/unmarshaling of LSP types
func TestJSONSerialization(t *testing.T) {
	t.Run("LSPRequest", func(t *testing.T) {
		req := LSPRequest{
			ServerID: "gopls",
			Method:   "textDocument/completion",
			Params:   map[string]interface{}{"key": "value"},
			FileURI:  "file:///test.go",
			Position: LSPPosition{Line: 10, Character: 5},
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var decoded LSPRequest
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, req.ServerID, decoded.ServerID)
		assert.Equal(t, req.Method, decoded.Method)
	})

	t.Run("LSPResponse", func(t *testing.T) {
		resp := LSPResponse{
			Success:   true,
			Result:    map[string]string{"data": "test"},
			Timestamp: time.Now(),
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var decoded LSPResponse
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, resp.Success, decoded.Success)
	})

	t.Run("LSPServer", func(t *testing.T) {
		now := time.Now()
		server := LSPServer{
			ID:        "test",
			Name:      "Test Server",
			Language:  "test",
			Command:   "test-lsp",
			Args:      []string{"--debug"},
			Enabled:   true,
			Workspace: "/workspace",
			LastSync:  &now,
			Capabilities: []LSPCapability{
				{Name: "hover", Description: "Hover support"},
			},
			BinaryPath: "/usr/bin/test-lsp",
			Available:  true,
		}

		data, err := json.Marshal(server)
		require.NoError(t, err)

		var decoded LSPServer
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, server.ID, decoded.ID)
		assert.Equal(t, server.Available, decoded.Available)
	})
}

// TestLSPManager_OpenDocument tests the openDocument method
func TestLSPManager_OpenDocument(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	stdin := &MockWriteCloser{}
	conn := &LSPConnection{
		stdin:     stdin,
		connected: true,
	}

	err := manager.openDocument(ctx, conn, "file:///test.go", "go", "package main")
	require.NoError(t, err)

	// Verify the notification was written
	written := stdin.String()
	assert.Contains(t, written, "textDocument/didOpen")
	assert.Contains(t, written, "file:///test.go")
}

// TestLSPManager_InitializeConnection_Integration tests initialization
func TestLSPManager_InitializeConnection_Integration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test")
	}

	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	// This test requires an actual LSP server to be installed
	server, err := manager.GetLSPServer(ctx, "gopls")
	require.NoError(t, err)

	if !server.Available {
		t.Skip("gopls not available")
	}

	// Try to create a connection
	conn, err := manager.getOrCreateConnection(ctx, "gopls")
	if err != nil {
		t.Skipf("Could not create connection: %v", err)
	}

	assert.NotNil(t, conn)
	assert.True(t, conn.initialized)

	// Cleanup
	_ = manager.CloseAll()
}

// TestLSPManager_StartServer_ProcessError tests process start errors
func TestLSPManager_StartServer_ProcessError(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	// Add a server with invalid command
	invalidServer := &LSPServer{
		ID:         "invalid",
		Name:       "Invalid Server",
		Language:   "invalid",
		Command:    "/nonexistent/binary",
		BinaryPath: "/nonexistent/binary",
		Enabled:    true,
		Available:  true, // Mark as available to bypass availability check
	}
	manager.AddServer(invalidServer)

	// Try to execute request - should handle error gracefully
	req := LSPRequest{
		ServerID: "invalid",
		Method:   "test/method",
	}

	response, err := manager.ExecuteLSPRequest(ctx, req)
	require.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "failed to start")
}

// TestExecLookPath tests that we properly wrap exec.LookPath
func TestExecLookPath(t *testing.T) {
	// Test looking up a common command
	_, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go command not in PATH")
	}
}

// TestLSPManager_GracefulDegradation tests graceful degradation scenarios
func TestLSPManager_GracefulDegradation(t *testing.T) {
	log := newLSPTestLogger()
	manager := NewLSPManager(nil, nil, log)
	ctx := context.Background()

	testCases := []struct {
		name     string
		method   func() (interface{}, error)
		checkFn  func(t *testing.T, result interface{})
	}{
		{
			name: "GetDiagnostics",
			method: func() (interface{}, error) {
				return manager.GetDiagnostics(ctx, "gopls", "file:///test.go")
			},
			checkFn: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, m, "diagnostics")
				assert.Contains(t, m, "timestamp")
			},
		},
		{
			name: "GetCodeActions",
			method: func() (interface{}, error) {
				return manager.GetCodeActions(ctx, "gopls", "text", "file:///test.go", LSPPosition{Line: 1, Character: 0})
			},
			checkFn: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, m, "actions")
			},
		},
		{
			name: "GetCompletion",
			method: func() (interface{}, error) {
				return manager.GetCompletion(ctx, "gopls", "fmt.", "file:///test.go", LSPPosition{Line: 1, Character: 4})
			},
			checkFn: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, m, "completions")
			},
		},
		{
			name: "GetHover",
			method: func() (interface{}, error) {
				return manager.GetHover(ctx, "gopls", "file:///test.go", 1, 0)
			},
			checkFn: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, m, "serverId")
			},
		},
		{
			name: "GetDefinition",
			method: func() (interface{}, error) {
				return manager.GetDefinition(ctx, "gopls", "file:///test.go", 1, 0)
			},
			checkFn: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, m, "serverId")
			},
		},
		{
			name: "GetReferences",
			method: func() (interface{}, error) {
				return manager.GetReferences(ctx, "gopls", "file:///test.go", 1, 0)
			},
			checkFn: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, m, "references")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.method()
			require.NoError(t, err)
			tc.checkFn(t, result)
		})
	}
}
