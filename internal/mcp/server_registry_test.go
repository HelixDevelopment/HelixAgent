package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServerRegistry(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("/tmp/mcp-servers")
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.servers)
	assert.Equal(t, "/tmp/mcp-servers", registry.serversDir)
}

func TestServerRegistry_LoadServers(t *testing.T) {
	t.Parallel()
	// Create a temporary directory with test servers
	tempDir, err := os.MkdirTemp("", "mcp-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a test server directory
	serverDir := filepath.Join(tempDir, "test-server")
	require.NoError(t, os.MkdirAll(serverDir, 0755))

	// Create server.json
	config := `{
		"$schema": "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json",
		"name": "test-server",
		"title": "Test Server",
		"description": "A test MCP server",
		"version": "1.0.0",
		"packages": [
			{
				"registryType": "npm",
				"identifier": "@test/server",
				"version": "1.0.0",
				"transport": {
					"type": "stdio"
				}
			}
		]
	}`
	err = os.WriteFile(filepath.Join(serverDir, "server.json"), []byte(config), 0644)
	require.NoError(t, err)

	// Load servers
	registry := NewServerRegistry(tempDir)
	count, err := registry.LoadServers()
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify server was loaded
	server, ok := registry.Get("test-server")
	assert.True(t, ok)
	assert.Equal(t, "test-server", server.Config.Name)
	assert.Equal(t, "Test Server", server.Config.Title)
	assert.Equal(t, ServerStatusAvailable, server.Status)
}

func TestServerRegistry_LoadServers_Multiple(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "mcp-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	servers := []struct {
		name  string
		title string
	}{
		{"server-a", "Server A"},
		{"server-b", "Server B"},
		{"server-c", "Server C"},
	}

	for _, s := range servers {
		serverDir := filepath.Join(tempDir, s.name)
		require.NoError(t, os.MkdirAll(serverDir, 0755))

		config := `{
			"name": "` + s.name + `",
			"title": "` + s.title + `",
			"version": "1.0.0",
			"packages": []
		}`
		err = os.WriteFile(filepath.Join(serverDir, "server.json"), []byte(config), 0644)
		require.NoError(t, err)
	}

	registry := NewServerRegistry(tempDir)
	count, err := registry.LoadServers()
	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Equal(t, 3, registry.Count())
}

func TestServerRegistry_GetAll(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	// Register test servers
	for i, name := range []string{"server-1", "server-2", "server-3"} {
		server := &MCPServer{
			Config: &ServerConfig{
				Name:  name,
				Title: "Server " + string(rune('A'+i)),
			},
			Enabled: true,
			Status:  ServerStatusAvailable,
		}
		err := registry.Register(server)
		require.NoError(t, err)
	}

	servers := registry.GetAll()
	assert.Len(t, servers, 3)
}

func TestServerRegistry_GetEnabled(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	// Register servers with different enabled states
	servers := []struct {
		name    string
		enabled bool
	}{
		{"server-enabled-1", true},
		{"server-disabled", false},
		{"server-enabled-2", true},
	}

	for _, s := range servers {
		server := &MCPServer{
			Config:  &ServerConfig{Name: s.name},
			Enabled: s.enabled,
			Status:  ServerStatusAvailable,
		}
		_ = registry.Register(server)
	}

	enabled := registry.GetEnabled()
	assert.Len(t, enabled, 2)
}

func TestServerRegistry_GetByStatus(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	// Register servers with different statuses
	statuses := []ServerStatus{
		ServerStatusAvailable,
		ServerStatusRunning,
		ServerStatusAvailable,
		ServerStatusError,
	}

	for i, status := range statuses {
		server := &MCPServer{
			Config:  &ServerConfig{Name: "server-" + string(rune('a'+i))},
			Enabled: true,
			Status:  status,
		}
		_ = registry.Register(server)
	}

	available := registry.GetByStatus(ServerStatusAvailable)
	assert.Len(t, available, 2)

	running := registry.GetByStatus(ServerStatusRunning)
	assert.Len(t, running, 1)

	errors := registry.GetByStatus(ServerStatusError)
	assert.Len(t, errors, 1)
}

func TestServerRegistry_EnableDisable(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	server := &MCPServer{
		Config:  &ServerConfig{Name: "test-server"},
		Enabled: true,
		Status:  ServerStatusAvailable,
	}
	require.NoError(t, registry.Register(server))

	// Disable
	err := registry.Disable("test-server")
	require.NoError(t, err)

	s, ok := registry.Get("test-server")
	require.True(t, ok)
	assert.False(t, s.Enabled)

	// Enable
	err = registry.Enable("test-server")
	require.NoError(t, err)

	s, ok = registry.Get("test-server")
	require.True(t, ok)
	assert.True(t, s.Enabled)
}

func TestServerRegistry_EnableDisable_NotFound(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	err := registry.Enable("nonexistent")
	assert.Error(t, err)

	err = registry.Disable("nonexistent")
	assert.Error(t, err)
}

func TestServerRegistry_UpdateStatus(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	server := &MCPServer{
		Config:  &ServerConfig{Name: "test-server"},
		Enabled: true,
		Status:  ServerStatusAvailable,
	}
	_ = registry.Register(server)

	err := registry.UpdateStatus("test-server", ServerStatusRunning)
	require.NoError(t, err)

	s, _ := registry.Get("test-server")
	assert.Equal(t, ServerStatusRunning, s.Status)
}

func TestServerRegistry_Remove(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	server := &MCPServer{
		Config:  &ServerConfig{Name: "test-server"},
		Enabled: true,
		Status:  ServerStatusAvailable,
	}
	_ = registry.Register(server)

	assert.Equal(t, 1, registry.Count())

	err := registry.Remove("test-server")
	require.NoError(t, err)

	assert.Equal(t, 0, registry.Count())
}

func TestServerRegistry_Remove_NotFound(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	err := registry.Remove("nonexistent")
	assert.Error(t, err)
}

func TestServerRegistry_Stats(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	// Register servers
	servers := []struct {
		name     string
		enabled  bool
		status   ServerStatus
		featured bool
	}{
		{"server-a", true, ServerStatusRunning, true},
		{"server-b", true, ServerStatusAvailable, false},
		{"server-c", false, ServerStatusStopped, false},
	}

	for _, s := range servers {
		var meta *ServerMeta
		if s.featured {
			meta = &ServerMeta{
				Marketplace: &MarketplaceMeta{Featured: true},
			}
		}
		server := &MCPServer{
			Config: &ServerConfig{
				Name:  s.name,
				Title: s.name,
				Meta:  meta,
			},
			Enabled: s.enabled,
			Status:  s.status,
		}
		_ = registry.Register(server)
	}

	stats := registry.Stats()
	assert.Equal(t, 3, stats.TotalServers)
	assert.Equal(t, 2, stats.EnabledCount)
	assert.Equal(t, 1, stats.StatusCounts[ServerStatusRunning])
	assert.Equal(t, 1, stats.StatusCounts[ServerStatusAvailable])
	assert.Equal(t, 1, stats.StatusCounts[ServerStatusStopped])
	assert.Len(t, stats.ServerList, 3)
}

func TestServerRegistry_ToMCPConfig(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	server := &MCPServer{
		Config: &ServerConfig{
			Name: "test-server",
			Packages: []Package{
				{
					RegistryType: "npm",
					Identifier:   "@test/server",
					Version:      "1.0.0",
					Transport:    &Transport{Type: "stdio"},
				},
			},
		},
		Path:    "/path/to/test-server",
		Enabled: true,
		Status:  ServerStatusAvailable,
	}
	_ = registry.Register(server)

	config := registry.ToMCPConfig()
	assert.Contains(t, config, "mcpServers")

	mcpServers := config["mcpServers"].(map[string]interface{})
	assert.Contains(t, mcpServers, "test-server")

	serverConfig := mcpServers["test-server"].(map[string]interface{})
	assert.Equal(t, "npx", serverConfig["command"])
	assert.Contains(t, serverConfig["args"].([]string), "@test/server")
}

func TestServerRegistry_Register_Duplicate(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	server := &MCPServer{
		Config:  &ServerConfig{Name: "test-server"},
		Enabled: true,
	}

	err := registry.Register(server)
	require.NoError(t, err)

	err = registry.Register(server)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestServerRegistry_Register_NoConfig(t *testing.T) {
	t.Parallel()
	registry := NewServerRegistry("")

	server := &MCPServer{
		Config:  nil,
		Enabled: true,
	}

	err := registry.Register(server)
	assert.Error(t, err)
}
