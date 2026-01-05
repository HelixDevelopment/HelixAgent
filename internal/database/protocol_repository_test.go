package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helper Functions for Protocol Repository
// =============================================================================

func setupProtocolTestDB(t *testing.T) (*pgxpool.Pool, *ProtocolRepository) {
	ctx := context.Background()
	connString := "postgres://superagent:secret@localhost:5432/superagent_db?sslmode=disable"

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
		return nil, nil
	}

	repo := NewProtocolRepository(pool)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: database connection failed: %v", err)
		pool.Close()
		return nil, nil
	}

	return pool, repo
}

func cleanupProtocolTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()

	// Clean up test data
	_, err := pool.Exec(ctx, "DELETE FROM mcp_servers WHERE name LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup mcp_servers: %v", err)
	}

	_, err = pool.Exec(ctx, "DELETE FROM lsp_servers WHERE name LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup lsp_servers: %v", err)
	}

	_, err = pool.Exec(ctx, "DELETE FROM acp_servers WHERE name LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup acp_servers: %v", err)
	}

	_, err = pool.Exec(ctx, "DELETE FROM protocol_cache WHERE cache_key LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup protocol_cache: %v", err)
	}

	_, err = pool.Exec(ctx, "DELETE FROM protocol_metrics WHERE operation LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup protocol_metrics: %v", err)
	}
}

func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func createTestMCPServer() *MCPServer {
	return &MCPServer{
		ID:       "test-mcp-" + time.Now().Format("20060102150405.000"),
		Name:     "test-mcp-server-" + time.Now().Format("20060102150405"),
		Type:     "local",
		Command:  stringPtr("npx @modelcontextprotocol/server"),
		Enabled:  true,
		Tools:    json.RawMessage(`["read", "write", "execute"]`),
		LastSync: time.Now(),
	}
}

func createTestLSPServer() *LSPServer {
	return &LSPServer{
		ID:           "test-lsp-" + time.Now().Format("20060102150405.000"),
		Name:         "test-lsp-server-" + time.Now().Format("20060102150405"),
		Language:     "go",
		Command:      "gopls",
		Enabled:      true,
		Workspace:    "/tmp/workspace",
		Capabilities: json.RawMessage(`{"hover": true, "completion": true}`),
		LastSync:     time.Now(),
	}
}

func createTestACPServer() *ACPServer {
	return &ACPServer{
		ID:       "test-acp-" + time.Now().Format("20060102150405.000"),
		Name:     "test-acp-server-" + time.Now().Format("20060102150405"),
		Type:     "remote",
		URL:      stringPtr("https://acp.example.com"),
		Enabled:  true,
		Tools:    json.RawMessage(`["analyze", "suggest"]`),
		LastSync: time.Now(),
	}
}

// =============================================================================
// MCP Server Integration Tests
// =============================================================================

func TestProtocolRepository_CreateMCPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestMCPServer()
		err := repo.CreateMCPServer(ctx, server)
		assert.NoError(t, err)
	})

	t.Run("WithRemoteType", func(t *testing.T) {
		server := createTestMCPServer()
		server.ID = "test-mcp-remote-" + time.Now().Format("20060102150405.000")
		server.Name = "test-mcp-remote-" + time.Now().Format("20060102150405")
		server.Type = "remote"
		server.Command = nil
		server.URL = stringPtr("https://mcp.example.com")
		err := repo.CreateMCPServer(ctx, server)
		assert.NoError(t, err)
	})
}

func TestProtocolRepository_GetMCPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestMCPServer()
		err := repo.CreateMCPServer(ctx, server)
		require.NoError(t, err)

		fetched, err := repo.GetMCPServer(ctx, server.ID)
		assert.NoError(t, err)
		assert.Equal(t, server.ID, fetched.ID)
		assert.Equal(t, server.Name, fetched.Name)
		assert.Equal(t, server.Type, fetched.Type)
		assert.Equal(t, server.Enabled, fetched.Enabled)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetMCPServer(ctx, "non-existent-id")
		assert.Error(t, err)
	})
}

func TestProtocolRepository_ListMCPServers(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("ListAll", func(t *testing.T) {
		// Create test servers
		for i := 0; i < 3; i++ {
			server := createTestMCPServer()
			server.ID = "test-mcp-list-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			server.Name = "test-mcp-list-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			err := repo.CreateMCPServer(ctx, server)
			require.NoError(t, err)
		}

		servers, err := repo.ListMCPServers(ctx, false)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(servers), 3)
	})

	t.Run("ListEnabledOnly", func(t *testing.T) {
		// Create enabled and disabled servers
		enabledServer := createTestMCPServer()
		enabledServer.ID = "test-mcp-enabled-" + time.Now().Format("20060102150405.000")
		enabledServer.Name = "test-mcp-enabled-" + time.Now().Format("20060102150405")
		enabledServer.Enabled = true
		err := repo.CreateMCPServer(ctx, enabledServer)
		require.NoError(t, err)

		disabledServer := createTestMCPServer()
		disabledServer.ID = "test-mcp-disabled-" + time.Now().Format("20060102150405.001")
		disabledServer.Name = "test-mcp-disabled-" + time.Now().Format("20060102150405.001")
		disabledServer.Enabled = false
		err = repo.CreateMCPServer(ctx, disabledServer)
		require.NoError(t, err)

		servers, err := repo.ListMCPServers(ctx, true)
		assert.NoError(t, err)

		for _, s := range servers {
			assert.True(t, s.Enabled)
		}
	})
}

func TestProtocolRepository_UpdateMCPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestMCPServer()
		err := repo.CreateMCPServer(ctx, server)
		require.NoError(t, err)

		server.Name = "test-mcp-updated-" + time.Now().Format("20060102150405")
		server.Enabled = false
		server.Tools = json.RawMessage(`["new-tool"]`)

		err = repo.UpdateMCPServer(ctx, server)
		assert.NoError(t, err)

		fetched, err := repo.GetMCPServer(ctx, server.ID)
		assert.NoError(t, err)
		assert.Equal(t, server.Name, fetched.Name)
		assert.False(t, fetched.Enabled)
	})
}

func TestProtocolRepository_DeleteMCPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestMCPServer()
		err := repo.CreateMCPServer(ctx, server)
		require.NoError(t, err)

		err = repo.DeleteMCPServer(ctx, server.ID)
		assert.NoError(t, err)

		_, err = repo.GetMCPServer(ctx, server.ID)
		assert.Error(t, err)
	})
}

// =============================================================================
// LSP Server Integration Tests
// =============================================================================

func TestProtocolRepository_CreateLSPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestLSPServer()
		err := repo.CreateLSPServer(ctx, server)
		assert.NoError(t, err)
	})

	t.Run("DifferentLanguages", func(t *testing.T) {
		languages := []string{"python", "rust", "typescript", "java"}
		for _, lang := range languages {
			server := createTestLSPServer()
			server.ID = "test-lsp-" + lang + "-" + time.Now().Format("20060102150405.000")
			server.Name = "test-lsp-" + lang + "-" + time.Now().Format("20060102150405")
			server.Language = lang
			err := repo.CreateLSPServer(ctx, server)
			assert.NoError(t, err)
		}
	})
}

func TestProtocolRepository_GetLSPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestLSPServer()
		err := repo.CreateLSPServer(ctx, server)
		require.NoError(t, err)

		fetched, err := repo.GetLSPServer(ctx, server.ID)
		assert.NoError(t, err)
		assert.Equal(t, server.ID, fetched.ID)
		assert.Equal(t, server.Name, fetched.Name)
		assert.Equal(t, server.Language, fetched.Language)
		assert.Equal(t, server.Command, fetched.Command)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetLSPServer(ctx, "non-existent-id")
		assert.Error(t, err)
	})
}

func TestProtocolRepository_ListLSPServers(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("ListAll", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			server := createTestLSPServer()
			server.ID = "test-lsp-list-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			server.Name = "test-lsp-list-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			err := repo.CreateLSPServer(ctx, server)
			require.NoError(t, err)
		}

		servers, err := repo.ListLSPServers(ctx, false)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(servers), 3)
	})

	t.Run("ListEnabledOnly", func(t *testing.T) {
		enabledServer := createTestLSPServer()
		enabledServer.ID = "test-lsp-enabled-" + time.Now().Format("20060102150405.000")
		enabledServer.Name = "test-lsp-enabled-" + time.Now().Format("20060102150405")
		enabledServer.Enabled = true
		err := repo.CreateLSPServer(ctx, enabledServer)
		require.NoError(t, err)

		disabledServer := createTestLSPServer()
		disabledServer.ID = "test-lsp-disabled-" + time.Now().Format("20060102150405.001")
		disabledServer.Name = "test-lsp-disabled-" + time.Now().Format("20060102150405.001")
		disabledServer.Enabled = false
		err = repo.CreateLSPServer(ctx, disabledServer)
		require.NoError(t, err)

		servers, err := repo.ListLSPServers(ctx, true)
		assert.NoError(t, err)

		for _, s := range servers {
			assert.True(t, s.Enabled)
		}
	})
}

func TestProtocolRepository_UpdateLSPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestLSPServer()
		err := repo.CreateLSPServer(ctx, server)
		require.NoError(t, err)

		server.Name = "test-lsp-updated-" + time.Now().Format("20060102150405")
		server.Workspace = "/updated/workspace"
		server.Capabilities = json.RawMessage(`{"hover": false, "completion": true, "diagnostics": true}`)

		err = repo.UpdateLSPServer(ctx, server)
		assert.NoError(t, err)

		fetched, err := repo.GetLSPServer(ctx, server.ID)
		assert.NoError(t, err)
		assert.Equal(t, server.Name, fetched.Name)
		assert.Equal(t, "/updated/workspace", fetched.Workspace)
	})
}

func TestProtocolRepository_DeleteLSPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestLSPServer()
		err := repo.CreateLSPServer(ctx, server)
		require.NoError(t, err)

		err = repo.DeleteLSPServer(ctx, server.ID)
		assert.NoError(t, err)

		_, err = repo.GetLSPServer(ctx, server.ID)
		assert.Error(t, err)
	})
}

// =============================================================================
// ACP Server Integration Tests
// =============================================================================

func TestProtocolRepository_CreateACPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestACPServer()
		err := repo.CreateACPServer(ctx, server)
		assert.NoError(t, err)
	})

	t.Run("WithLocalType", func(t *testing.T) {
		server := createTestACPServer()
		server.ID = "test-acp-local-" + time.Now().Format("20060102150405.000")
		server.Name = "test-acp-local-" + time.Now().Format("20060102150405")
		server.Type = "local"
		server.URL = nil
		err := repo.CreateACPServer(ctx, server)
		assert.NoError(t, err)
	})
}

func TestProtocolRepository_GetACPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestACPServer()
		err := repo.CreateACPServer(ctx, server)
		require.NoError(t, err)

		fetched, err := repo.GetACPServer(ctx, server.ID)
		assert.NoError(t, err)
		assert.Equal(t, server.ID, fetched.ID)
		assert.Equal(t, server.Name, fetched.Name)
		assert.Equal(t, server.Type, fetched.Type)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetACPServer(ctx, "non-existent-id")
		assert.Error(t, err)
	})
}

func TestProtocolRepository_ListACPServers(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("ListAll", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			server := createTestACPServer()
			server.ID = "test-acp-list-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			server.Name = "test-acp-list-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			err := repo.CreateACPServer(ctx, server)
			require.NoError(t, err)
		}

		servers, err := repo.ListACPServers(ctx, false)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(servers), 3)
	})

	t.Run("ListEnabledOnly", func(t *testing.T) {
		enabledServer := createTestACPServer()
		enabledServer.ID = "test-acp-enabled-" + time.Now().Format("20060102150405.000")
		enabledServer.Name = "test-acp-enabled-" + time.Now().Format("20060102150405")
		enabledServer.Enabled = true
		err := repo.CreateACPServer(ctx, enabledServer)
		require.NoError(t, err)

		disabledServer := createTestACPServer()
		disabledServer.ID = "test-acp-disabled-" + time.Now().Format("20060102150405.001")
		disabledServer.Name = "test-acp-disabled-" + time.Now().Format("20060102150405.001")
		disabledServer.Enabled = false
		err = repo.CreateACPServer(ctx, disabledServer)
		require.NoError(t, err)

		servers, err := repo.ListACPServers(ctx, true)
		assert.NoError(t, err)

		for _, s := range servers {
			assert.True(t, s.Enabled)
		}
	})
}

func TestProtocolRepository_UpdateACPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestACPServer()
		err := repo.CreateACPServer(ctx, server)
		require.NoError(t, err)

		server.Name = "test-acp-updated-" + time.Now().Format("20060102150405")
		server.Enabled = false
		server.Tools = json.RawMessage(`["updated-tool"]`)

		err = repo.UpdateACPServer(ctx, server)
		assert.NoError(t, err)

		fetched, err := repo.GetACPServer(ctx, server.ID)
		assert.NoError(t, err)
		assert.Equal(t, server.Name, fetched.Name)
		assert.False(t, fetched.Enabled)
	})
}

func TestProtocolRepository_DeleteACPServer(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		server := createTestACPServer()
		err := repo.CreateACPServer(ctx, server)
		require.NoError(t, err)

		err = repo.DeleteACPServer(ctx, server.ID)
		assert.NoError(t, err)

		_, err = repo.GetACPServer(ctx, server.ID)
		assert.Error(t, err)
	})
}

// =============================================================================
// Protocol Cache Integration Tests
// =============================================================================

func TestProtocolRepository_SetCache(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		key := "test-cache-" + time.Now().Format("20060102150405")
		data := json.RawMessage(`{"key": "value"}`)
		ttl := 1 * time.Hour

		err := repo.SetCache(ctx, key, data, ttl)
		assert.NoError(t, err)

		cached, err := repo.GetCache(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, key, cached.CacheKey)
		assert.JSONEq(t, string(data), string(cached.CacheData))
	})

	t.Run("Upsert", func(t *testing.T) {
		key := "test-cache-upsert-" + time.Now().Format("20060102150405")
		data1 := json.RawMessage(`{"version": 1}`)
		data2 := json.RawMessage(`{"version": 2}`)
		ttl := 1 * time.Hour

		err := repo.SetCache(ctx, key, data1, ttl)
		require.NoError(t, err)

		err = repo.SetCache(ctx, key, data2, ttl)
		assert.NoError(t, err)

		cached, err := repo.GetCache(ctx, key)
		assert.NoError(t, err)
		assert.JSONEq(t, string(data2), string(cached.CacheData))
	})
}

func TestProtocolRepository_GetCache(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		key := "test-cache-get-" + time.Now().Format("20060102150405")
		data := json.RawMessage(`{"test": true}`)
		ttl := 1 * time.Hour

		err := repo.SetCache(ctx, key, data, ttl)
		require.NoError(t, err)

		cached, err := repo.GetCache(ctx, key)
		assert.NoError(t, err)
		assert.NotNil(t, cached)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetCache(ctx, "non-existent-cache-key")
		assert.Error(t, err)
	})

	t.Run("Expired", func(t *testing.T) {
		key := "test-cache-expired-" + time.Now().Format("20060102150405")
		data := json.RawMessage(`{"expired": true}`)
		ttl := -1 * time.Hour // Already expired

		err := repo.SetCache(ctx, key, data, ttl)
		require.NoError(t, err)

		_, err = repo.GetCache(ctx, key)
		assert.Error(t, err)
	})
}

func TestProtocolRepository_DeleteCache(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		key := "test-cache-delete-" + time.Now().Format("20060102150405")
		data := json.RawMessage(`{"delete": true}`)
		ttl := 1 * time.Hour

		err := repo.SetCache(ctx, key, data, ttl)
		require.NoError(t, err)

		err = repo.DeleteCache(ctx, key)
		assert.NoError(t, err)

		_, err = repo.GetCache(ctx, key)
		assert.Error(t, err)
	})
}

func TestProtocolRepository_ClearExpiredCache(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create expired cache entries
		for i := 0; i < 3; i++ {
			key := "test-cache-expired-clear-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			data := json.RawMessage(`{"expired": true}`)
			ttl := -1 * time.Hour // Already expired

			err := repo.SetCache(ctx, key, data, ttl)
			require.NoError(t, err)
		}

		deleted, err := repo.ClearExpiredCache(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, int64(3))
	})
}

// =============================================================================
// Protocol Metrics Integration Tests
// =============================================================================

func TestProtocolRepository_RecordMetric(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		metric := &ProtocolMetrics{
			ProtocolType: "mcp",
			ServerID:     stringPtr("test-server-1"),
			Operation:    "test-operation-" + time.Now().Format("20060102150405"),
			Status:       "success",
			DurationMs:   intPtr(150),
			Metadata:     json.RawMessage(`{"request_id": "abc123"}`),
		}

		err := repo.RecordMetric(ctx, metric)
		assert.NoError(t, err)
	})

	t.Run("WithError", func(t *testing.T) {
		errMsg := "connection timeout"
		metric := &ProtocolMetrics{
			ProtocolType: "lsp",
			ServerID:     stringPtr("test-server-2"),
			Operation:    "test-operation-error-" + time.Now().Format("20060102150405"),
			Status:       "error",
			DurationMs:   intPtr(5000),
			ErrorMessage: &errMsg,
			Metadata:     json.RawMessage(`{}`),
		}

		err := repo.RecordMetric(ctx, metric)
		assert.NoError(t, err)
	})

	t.Run("WithoutServerID", func(t *testing.T) {
		metric := &ProtocolMetrics{
			ProtocolType: "embedding",
			ServerID:     nil,
			Operation:    "test-operation-no-server-" + time.Now().Format("20060102150405"),
			Status:       "success",
			DurationMs:   intPtr(50),
			Metadata:     json.RawMessage(`{}`),
		}

		err := repo.RecordMetric(ctx, metric)
		assert.NoError(t, err)
	})
}

func TestProtocolRepository_GetMetrics(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create test metrics
		for i := 0; i < 5; i++ {
			metric := &ProtocolMetrics{
				ProtocolType: "mcp",
				Operation:    "test-get-metrics-" + time.Now().Format("20060102150405.000000"),
				Status:       "success",
				DurationMs:   intPtr(100 + i*10),
				Metadata:     json.RawMessage(`{}`),
			}
			err := repo.RecordMetric(ctx, metric)
			require.NoError(t, err)
		}

		since := time.Now().Add(-1 * time.Hour)
		metrics, err := repo.GetMetrics(ctx, "mcp", since, 10)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(metrics), 5)
	})

	t.Run("WithLimit", func(t *testing.T) {
		since := time.Now().Add(-1 * time.Hour)
		metrics, err := repo.GetMetrics(ctx, "", since, 2)
		assert.NoError(t, err)
		assert.LessOrEqual(t, len(metrics), 2)
	})

	t.Run("FilterByProtocolType", func(t *testing.T) {
		// Create metrics for different protocols
		for _, protocol := range []string{"mcp", "lsp", "acp"} {
			metric := &ProtocolMetrics{
				ProtocolType: protocol,
				Operation:    "test-filter-" + protocol + "-" + time.Now().Format("20060102150405.000"),
				Status:       "success",
				Metadata:     json.RawMessage(`{}`),
			}
			err := repo.RecordMetric(ctx, metric)
			require.NoError(t, err)
		}

		since := time.Now().Add(-1 * time.Hour)
		metrics, err := repo.GetMetrics(ctx, "lsp", since, 100)
		assert.NoError(t, err)

		for _, m := range metrics {
			assert.Equal(t, "lsp", m.ProtocolType)
		}
	})
}

func TestProtocolRepository_GetMetricsSummary(t *testing.T) {
	pool, repo := setupProtocolTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProtocolTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create test metrics with different statuses
		statuses := []string{"success", "success", "success", "error", "timeout"}
		for i, status := range statuses {
			metric := &ProtocolMetrics{
				ProtocolType: "acp",
				Operation:    "test-summary-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i)),
				Status:       status,
				DurationMs:   intPtr(100 + i*50),
				Metadata:     json.RawMessage(`{}`),
			}
			err := repo.RecordMetric(ctx, metric)
			require.NoError(t, err)
		}

		since := time.Now().Add(-1 * time.Hour)
		summary, err := repo.GetMetricsSummary(ctx, "acp", since)
		assert.NoError(t, err)
		assert.NotNil(t, summary)
		assert.Contains(t, summary, "total_operations")
		assert.Contains(t, summary, "successful")
		assert.Contains(t, summary, "errors")
		assert.Contains(t, summary, "timeouts")
		assert.Contains(t, summary, "success_rate")
	})
}

// =============================================================================
// Unit Tests (No Database Required)
// =============================================================================

func TestNewProtocolRepository(t *testing.T) {
	t.Run("CreatesRepositoryWithNilPool", func(t *testing.T) {
		repo := NewProtocolRepository(nil)
		assert.NotNil(t, repo)
	})
}

func TestMCPServer_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullServer", func(t *testing.T) {
		server := &MCPServer{
			ID:        "mcp-1",
			Name:      "test-server",
			Type:      "local",
			Command:   stringPtr("npx mcp-server"),
			URL:       nil,
			Enabled:   true,
			Tools:     json.RawMessage(`["read", "write"]`),
			LastSync:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		jsonBytes, err := json.Marshal(server)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "test-server")
		assert.Contains(t, string(jsonBytes), "local")

		var decoded MCPServer
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, server.Name, decoded.Name)
	})

	t.Run("SerializesRemoteServer", func(t *testing.T) {
		server := &MCPServer{
			ID:      "mcp-2",
			Name:    "remote-server",
			Type:    "remote",
			Command: nil,
			URL:     stringPtr("https://mcp.example.com"),
			Enabled: true,
			Tools:   json.RawMessage(`[]`),
		}

		jsonBytes, err := json.Marshal(server)
		require.NoError(t, err)

		var decoded MCPServer
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "remote", decoded.Type)
		assert.NotNil(t, decoded.URL)
	})
}

func TestLSPServer_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullServer", func(t *testing.T) {
		server := &LSPServer{
			ID:           "lsp-1",
			Name:         "gopls",
			Language:     "go",
			Command:      "gopls serve",
			Enabled:      true,
			Workspace:    "/home/user/project",
			Capabilities: json.RawMessage(`{"hover": true}`),
			LastSync:     time.Now(),
		}

		jsonBytes, err := json.Marshal(server)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "gopls")
		assert.Contains(t, string(jsonBytes), "go")

		var decoded LSPServer
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, server.Language, decoded.Language)
	})
}

func TestACPServer_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullServer", func(t *testing.T) {
		server := &ACPServer{
			ID:       "acp-1",
			Name:     "acp-server",
			Type:     "remote",
			URL:      stringPtr("https://acp.example.com"),
			Enabled:  true,
			Tools:    json.RawMessage(`["analyze"]`),
			LastSync: time.Now(),
		}

		jsonBytes, err := json.Marshal(server)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "acp-server")

		var decoded ACPServer
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, server.Name, decoded.Name)
	})
}

func TestEmbeddingConfig_JSONSerialization(t *testing.T) {
	t.Run("SerializesConfig", func(t *testing.T) {
		cfg := &EmbeddingConfig{
			ID:          1,
			Provider:    "openai",
			Model:       "text-embedding-ada-002",
			Dimension:   1536,
			APIEndpoint: stringPtr("https://api.openai.com/v1/embeddings"),
			APIKey:      stringPtr("secret-key"),
		}

		jsonBytes, err := json.Marshal(cfg)
		require.NoError(t, err)

		// API key should be omitted in JSON
		assert.NotContains(t, string(jsonBytes), "secret-key")
		assert.Contains(t, string(jsonBytes), "openai")
		assert.Contains(t, string(jsonBytes), "1536")
	})
}

func TestProtocolCache_JSONSerialization(t *testing.T) {
	t.Run("SerializesCache", func(t *testing.T) {
		cache := &ProtocolCache{
			CacheKey:  "test-key",
			CacheData: json.RawMessage(`{"cached": true}`),
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		jsonBytes, err := json.Marshal(cache)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "test-key")

		var decoded ProtocolCache
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, cache.CacheKey, decoded.CacheKey)
	})
}

func TestProtocolMetrics_JSONSerialization(t *testing.T) {
	t.Run("SerializesMetrics", func(t *testing.T) {
		errMsg := "test error"
		metric := &ProtocolMetrics{
			ID:           1,
			ProtocolType: "mcp",
			ServerID:     stringPtr("server-1"),
			Operation:    "execute",
			Status:       "error",
			DurationMs:   intPtr(500),
			ErrorMessage: &errMsg,
			Metadata:     json.RawMessage(`{"attempts": 3}`),
		}

		jsonBytes, err := json.Marshal(metric)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "mcp")
		assert.Contains(t, string(jsonBytes), "execute")
		assert.Contains(t, string(jsonBytes), "error")

		var decoded ProtocolMetrics
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, metric.ProtocolType, decoded.ProtocolType)
	})

	t.Run("SerializesWithNilOptionalFields", func(t *testing.T) {
		metric := &ProtocolMetrics{
			ProtocolType: "lsp",
			Operation:    "hover",
			Status:       "success",
			ServerID:     nil,
			DurationMs:   nil,
			ErrorMessage: nil,
			Metadata:     json.RawMessage(`{}`),
		}

		jsonBytes, err := json.Marshal(metric)
		require.NoError(t, err)

		var decoded ProtocolMetrics
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Nil(t, decoded.ServerID)
		assert.Nil(t, decoded.DurationMs)
	})
}

func TestProtocolMetrics_StatusValues(t *testing.T) {
	statuses := []string{"success", "error", "timeout"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			metric := &ProtocolMetrics{
				Status: status,
			}
			assert.Equal(t, status, metric.Status)
		})
	}
}

func TestMCPServer_TypeValues(t *testing.T) {
	types := []string{"local", "remote"}

	for _, serverType := range types {
		t.Run(serverType, func(t *testing.T) {
			server := &MCPServer{
				Type: serverType,
			}
			assert.Equal(t, serverType, server.Type)
		})
	}
}

func TestLSPServer_LanguageValues(t *testing.T) {
	languages := []string{"go", "python", "rust", "typescript", "java", "c", "cpp"}

	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			server := &LSPServer{
				Language: lang,
			}
			assert.Equal(t, lang, server.Language)
		})
	}
}

func TestCreateTestHelpers(t *testing.T) {
	t.Run("CreateTestMCPServer", func(t *testing.T) {
		server := createTestMCPServer()
		assert.NotEmpty(t, server.ID)
		assert.NotEmpty(t, server.Name)
		assert.NotEmpty(t, server.Type)
		assert.True(t, server.Enabled)
		assert.NotNil(t, server.Tools)
	})

	t.Run("CreateTestLSPServer", func(t *testing.T) {
		server := createTestLSPServer()
		assert.NotEmpty(t, server.ID)
		assert.NotEmpty(t, server.Name)
		assert.NotEmpty(t, server.Language)
		assert.NotEmpty(t, server.Command)
		assert.True(t, server.Enabled)
	})

	t.Run("CreateTestACPServer", func(t *testing.T) {
		server := createTestACPServer()
		assert.NotEmpty(t, server.ID)
		assert.NotEmpty(t, server.Name)
		assert.NotEmpty(t, server.Type)
		assert.True(t, server.Enabled)
	})
}

func TestStringPtr(t *testing.T) {
	t.Run("ReturnsPointer", func(t *testing.T) {
		s := "test"
		ptr := stringPtr(s)
		assert.NotNil(t, ptr)
		assert.Equal(t, s, *ptr)
	})

	t.Run("EmptyString", func(t *testing.T) {
		ptr := stringPtr("")
		assert.NotNil(t, ptr)
		assert.Equal(t, "", *ptr)
	})
}

func TestIntPtr(t *testing.T) {
	t.Run("ReturnsPointer", func(t *testing.T) {
		i := 42
		ptr := intPtr(i)
		assert.NotNil(t, ptr)
		assert.Equal(t, i, *ptr)
	})

	t.Run("ZeroValue", func(t *testing.T) {
		ptr := intPtr(0)
		assert.NotNil(t, ptr)
		assert.Equal(t, 0, *ptr)
	})
}
