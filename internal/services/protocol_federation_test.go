package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFederationTestLogger creates a logger for federation tests
func newFederationTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

// Tests for ProtocolDiscovery

func TestNewProtocolDiscovery(t *testing.T) {
	log := newFederationTestLogger()

	discovery := NewProtocolDiscovery(log)

	require.NotNil(t, discovery)
	assert.NotNil(t, discovery.discoveredServers)
	assert.NotNil(t, discovery.discoveryMethods)
	assert.NotNil(t, discovery.stopChan)
	assert.Equal(t, log, discovery.logger)
	// Should have default discovery methods
	assert.Len(t, discovery.discoveryMethods, 3)
}

func TestProtocolDiscovery_AddDiscoveryMethod(t *testing.T) {
	log := newFederationTestLogger()
	discovery := NewProtocolDiscovery(log)

	initialCount := len(discovery.discoveryMethods)

	// Add a custom discovery method
	mockMethod := &MockDiscoveryMethod{name: "mock"}
	discovery.AddDiscoveryMethod(mockMethod)

	assert.Len(t, discovery.discoveryMethods, initialCount+1)
}

func TestProtocolDiscovery_Start(t *testing.T) {
	log := newFederationTestLogger()
	discovery := NewProtocolDiscovery(log)

	ctx := context.Background()
	err := discovery.Start(ctx)

	require.NoError(t, err)

	// Clean up
	discovery.Stop()
}

func TestProtocolDiscovery_Stop(t *testing.T) {
	log := newFederationTestLogger()
	discovery := NewProtocolDiscovery(log)

	ctx := context.Background()
	_ = discovery.Start(ctx)

	// Should not panic
	discovery.Stop()
}

func TestProtocolDiscovery_DiscoverServers(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		discoveryMethods:  []DiscoveryMethod{},
		stopChan:          make(chan struct{}),
		logger:            log,
	}

	// Add a mock discovery method that returns servers
	mockMethod := &MockDiscoveryMethod{
		name: "mock",
		servers: []*DiscoveredServer{
			{
				ID:       "test-server-1",
				Protocol: "mcp",
				Address:  "127.0.0.1",
				Port:     3000,
				Name:     "Test MCP Server",
				Type:     "mock",
				Status:   StatusOnline,
			},
		},
	}
	discovery.AddDiscoveryMethod(mockMethod)

	ctx := context.Background()
	err := discovery.DiscoverServers(ctx)

	require.NoError(t, err)
	assert.Len(t, discovery.discoveredServers, 1)
}

func TestProtocolDiscovery_DiscoverServers_MethodError(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		discoveryMethods:  []DiscoveryMethod{},
		stopChan:          make(chan struct{}),
		logger:            log,
	}

	// Add a failing mock discovery method
	mockMethod := &MockDiscoveryMethod{
		name: "failing",
		err:  errors.New("discovery failed"),
	}
	discovery.AddDiscoveryMethod(mockMethod)

	// Add a working mock discovery method
	workingMethod := &MockDiscoveryMethod{
		name: "working",
		servers: []*DiscoveredServer{
			{
				ID:       "working-server",
				Protocol: "lsp",
				Address:  "127.0.0.1",
				Port:     6006,
				Name:     "Working Server",
				Type:     "mock",
				Status:   StatusOnline,
			},
		},
	}
	discovery.AddDiscoveryMethod(workingMethod)

	ctx := context.Background()
	err := discovery.DiscoverServers(ctx)

	// Should not return error even if one method fails
	require.NoError(t, err)
	// Should have discovered the working server
	assert.Len(t, discovery.discoveredServers, 1)
}

func TestProtocolDiscovery_GetDiscoveredServers(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		discoveryMethods:  []DiscoveryMethod{},
		stopChan:          make(chan struct{}),
		logger:            log,
	}

	t.Run("empty servers", func(t *testing.T) {
		servers := discovery.GetDiscoveredServers()
		assert.Empty(t, servers)
	})

	t.Run("with servers", func(t *testing.T) {
		discovery.discoveredServers["server1"] = &DiscoveredServer{
			ID:       "server1",
			Protocol: "mcp",
			Address:  "127.0.0.1",
			Port:     3000,
		}
		discovery.discoveredServers["server2"] = &DiscoveredServer{
			ID:       "server2",
			Protocol: "lsp",
			Address:  "127.0.0.1",
			Port:     6006,
		}

		servers := discovery.GetDiscoveredServers()
		assert.Len(t, servers, 2)
	})
}

func TestProtocolDiscovery_GetServersByProtocol(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: map[string]*DiscoveredServer{
			"mcp-1": {ID: "mcp-1", Protocol: "mcp", Address: "127.0.0.1", Port: 3000},
			"mcp-2": {ID: "mcp-2", Protocol: "mcp", Address: "127.0.0.2", Port: 3001},
			"lsp-1": {ID: "lsp-1", Protocol: "lsp", Address: "127.0.0.1", Port: 6006},
			"acp-1": {ID: "acp-1", Protocol: "acp", Address: "127.0.0.1", Port: 7061},
		},
		logger: log,
	}

	t.Run("get mcp servers", func(t *testing.T) {
		servers := discovery.GetServersByProtocol("mcp")
		assert.Len(t, servers, 2)
	})

	t.Run("get lsp servers", func(t *testing.T) {
		servers := discovery.GetServersByProtocol("lsp")
		assert.Len(t, servers, 1)
	})

	t.Run("get non-existent protocol", func(t *testing.T) {
		servers := discovery.GetServersByProtocol("unknown")
		assert.Empty(t, servers)
	})
}

func TestProtocolDiscovery_GetServerByID(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: map[string]*DiscoveredServer{
			"server-1": {ID: "server-1", Protocol: "mcp", Address: "127.0.0.1", Port: 3000},
		},
		logger: log,
	}

	t.Run("get existing server", func(t *testing.T) {
		server, err := discovery.GetServerByID("server-1")
		require.NoError(t, err)
		assert.Equal(t, "server-1", server.ID)
		assert.Equal(t, "mcp", server.Protocol)
	})

	t.Run("get non-existent server", func(t *testing.T) {
		server, err := discovery.GetServerByID("non-existent")
		assert.Error(t, err)
		assert.Nil(t, server)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProtocolDiscovery_RegisterServer(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		logger:            log,
	}

	err := discovery.RegisterServer("mcp", "127.0.0.1", 3000, "My MCP Server")

	require.NoError(t, err)
	assert.Len(t, discovery.discoveredServers, 1)

	// Verify server details
	serverID := "mcp-127.0.0.1-3000"
	server := discovery.discoveredServers[serverID]
	require.NotNil(t, server)
	assert.Equal(t, "mcp", server.Protocol)
	assert.Equal(t, "127.0.0.1", server.Address)
	assert.Equal(t, 3000, server.Port)
	assert.Equal(t, "My MCP Server", server.Name)
	assert.Equal(t, "manual", server.Type)
	assert.Equal(t, StatusOnline, server.Status)
}

func TestProtocolDiscovery_UnregisterServer(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: map[string]*DiscoveredServer{
			"server-1": {ID: "server-1", Protocol: "mcp", Address: "127.0.0.1", Port: 3000},
		},
		logger: log,
	}

	t.Run("unregister existing server", func(t *testing.T) {
		err := discovery.UnregisterServer("server-1")
		require.NoError(t, err)
		assert.Empty(t, discovery.discoveredServers)
	})

	t.Run("unregister non-existent server", func(t *testing.T) {
		err := discovery.UnregisterServer("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProtocolDiscovery_HealthCheck(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: map[string]*DiscoveredServer{
			"server-1": {
				ID:       "server-1",
				Protocol: "mcp",
				Address:  "127.0.0.1",
				Port:     3000,
				Status:   StatusOnline,
			},
		},
		logger: log,
	}

	ctx := context.Background()
	err := discovery.HealthCheck(ctx)

	// Health check should complete without error
	require.NoError(t, err)
}

func TestProtocolDiscovery_addOrUpdateServer(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		logger:            log,
	}

	t.Run("add new server", func(t *testing.T) {
		server := &DiscoveredServer{
			ID:           "new-server",
			Protocol:     "mcp",
			Address:      "127.0.0.1",
			Port:         3000,
			Status:       StatusOnline,
			Capabilities: map[string]interface{}{"streaming": true},
		}

		discovery.addOrUpdateServer(server)

		assert.Len(t, discovery.discoveredServers, 1)
		assert.NotNil(t, discovery.discoveredServers["new-server"])
	})

	t.Run("update existing server", func(t *testing.T) {
		// Update the existing server
		updatedServer := &DiscoveredServer{
			ID:           "new-server",
			Protocol:     "mcp",
			Address:      "127.0.0.1",
			Port:         3000,
			Status:       StatusOffline,
			Capabilities: map[string]interface{}{"streaming": false},
		}

		discovery.addOrUpdateServer(updatedServer)

		// Should still be 1 server
		assert.Len(t, discovery.discoveredServers, 1)
		// Status should be updated
		assert.Equal(t, StatusOffline, discovery.discoveredServers["new-server"].Status)
	})
}

func TestProtocolDiscovery_updateServerStatus(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: map[string]*DiscoveredServer{
			"server-1": {
				ID:       "server-1",
				Protocol: "mcp",
				Status:   StatusOnline,
				LastSeen: time.Now().Add(-1 * time.Hour),
			},
		},
		logger: log,
	}

	beforeUpdate := discovery.discoveredServers["server-1"].LastSeen

	discovery.updateServerStatus("server-1", StatusOffline)

	server := discovery.discoveredServers["server-1"]
	assert.Equal(t, StatusOffline, server.Status)
	assert.True(t, server.LastSeen.After(beforeUpdate))
}

func TestProtocolDiscovery_checkServerHealth(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		logger:            log,
	}

	ctx := context.Background()

	t.Run("check mcp server health", func(t *testing.T) {
		server := &DiscoveredServer{
			Protocol: "mcp",
			Address:  "127.0.0.1",
			Port:     59999, // Non-existent port
		}

		status := discovery.checkServerHealth(ctx, server)
		// Should be offline since no server is listening
		assert.Equal(t, StatusOffline, status)
	})

	t.Run("check lsp server health", func(t *testing.T) {
		server := &DiscoveredServer{
			Protocol: "lsp",
			Address:  "127.0.0.1",
			Port:     59998,
		}

		status := discovery.checkServerHealth(ctx, server)
		assert.Equal(t, StatusOffline, status)
	})

	t.Run("check acp server health", func(t *testing.T) {
		server := &DiscoveredServer{
			Protocol: "acp",
			Address:  "127.0.0.1",
			Port:     59997,
		}

		status := discovery.checkServerHealth(ctx, server)
		assert.Equal(t, StatusOffline, status)
	})

	t.Run("check unknown protocol health", func(t *testing.T) {
		server := &DiscoveredServer{
			Protocol: "unknown",
			Address:  "127.0.0.1",
			Port:     59996,
		}

		status := discovery.checkServerHealth(ctx, server)
		assert.Equal(t, StatusUnknown, status)
	})
}

// Tests for NetworkDiscovery

func TestNetworkDiscovery_Name(t *testing.T) {
	net := &NetworkDiscovery{
		port:     9999,
		protocol: "udp",
		services: make(map[string]*DiscoveredServer),
		logger:   newFederationTestLogger(),
	}

	assert.Equal(t, "network", net.Name())
}

func TestNetworkDiscovery_Start(t *testing.T) {
	net := &NetworkDiscovery{
		port:     9999,
		protocol: "udp",
		services: make(map[string]*DiscoveredServer),
		logger:   newFederationTestLogger(),
	}

	ctx := context.Background()
	err := net.Start(ctx)

	require.NoError(t, err)
}

func TestNetworkDiscovery_Stop(t *testing.T) {
	net := &NetworkDiscovery{
		port:     9999,
		protocol: "udp",
		services: make(map[string]*DiscoveredServer),
		logger:   newFederationTestLogger(),
	}

	err := net.Stop()

	require.NoError(t, err)
}

// Tests for ConfigurationDiscovery

func TestConfigurationDiscovery_Name(t *testing.T) {
	cfg := &ConfigurationDiscovery{
		config:   make(map[string]interface{}),
		services: make(map[string]*DiscoveredServer),
		logger:   newFederationTestLogger(),
	}

	assert.Equal(t, "config", cfg.Name())
}

func TestConfigurationDiscovery_Discover_NoConfig(t *testing.T) {
	cfg := &ConfigurationDiscovery{
		config:   make(map[string]interface{}),
		services: make(map[string]*DiscoveredServer),
		logger:   newFederationTestLogger(),
	}

	ctx := context.Background()
	servers, err := cfg.Discover(ctx)

	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestConfigurationDiscovery_Discover_WithMCPConfig(t *testing.T) {
	cfg := &ConfigurationDiscovery{
		config: map[string]interface{}{
			"mcp": map[string]interface{}{
				"servers": []interface{}{
					map[string]interface{}{
						"name": "test-mcp-server",
					},
				},
			},
		},
		services: make(map[string]*DiscoveredServer),
		logger:   newFederationTestLogger(),
	}

	ctx := context.Background()
	servers, err := cfg.Discover(ctx)

	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "config-mcp-test-mcp-server", servers[0].ID)
	assert.Equal(t, "mcp", servers[0].Protocol)
	assert.Equal(t, "test-mcp-server", servers[0].Name)
	assert.Equal(t, "config", servers[0].Type)
}

func TestConfigurationDiscovery_Start(t *testing.T) {
	cfg := &ConfigurationDiscovery{
		config:   make(map[string]interface{}),
		services: make(map[string]*DiscoveredServer),
		logger:   newFederationTestLogger(),
	}

	ctx := context.Background()
	err := cfg.Start(ctx)

	require.NoError(t, err)
}

func TestConfigurationDiscovery_Stop(t *testing.T) {
	cfg := &ConfigurationDiscovery{
		config:   make(map[string]interface{}),
		services: make(map[string]*DiscoveredServer),
		logger:   newFederationTestLogger(),
	}

	err := cfg.Stop()

	require.NoError(t, err)
}

// Tests for ServerStatus

func TestServerStatus_Values(t *testing.T) {
	assert.Equal(t, ServerStatus(0), StatusUnknown)
	assert.Equal(t, ServerStatus(1), StatusOnline)
	assert.Equal(t, ServerStatus(2), StatusOffline)
	assert.Equal(t, ServerStatus(3), StatusError)
}

// Tests for DiscoveredServer

func TestDiscoveredServer_Structure(t *testing.T) {
	server := &DiscoveredServer{
		ID:       "test-server",
		Protocol: "mcp",
		Address:  "127.0.0.1",
		Port:     3000,
		Name:     "Test Server",
		Type:     "manual",
		Capabilities: map[string]interface{}{
			"streaming": true,
			"tools":     true,
		},
		LastSeen: time.Now(),
		Status:   StatusOnline,
	}

	assert.Equal(t, "test-server", server.ID)
	assert.Equal(t, "mcp", server.Protocol)
	assert.Equal(t, "127.0.0.1", server.Address)
	assert.Equal(t, 3000, server.Port)
	assert.Equal(t, "Test Server", server.Name)
	assert.Equal(t, "manual", server.Type)
	assert.Equal(t, StatusOnline, server.Status)
	assert.True(t, server.Capabilities["streaming"].(bool))
}

// Concurrent access tests

func TestProtocolDiscovery_ConcurrentAccess(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		logger:            log,
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			serverID := "server-" + string(rune(id))

			// Register
			_ = discovery.RegisterServer("mcp", "127.0.0.1", 3000+id, serverID)

			// Get
			_, _ = discovery.GetServerByID(serverID)

			// List
			_ = discovery.GetDiscoveredServers()

			// Unregister
			_ = discovery.UnregisterServer(serverID)
		}(i)
	}

	wg.Wait()
}

func TestProtocolDiscovery_ConcurrentDiscovery(t *testing.T) {
	log := newFederationTestLogger()

	mockMethod := &MockDiscoveryMethod{
		name: "concurrent-mock",
		servers: []*DiscoveredServer{
			{ID: "concurrent-1", Protocol: "mcp", Address: "127.0.0.1", Port: 3000},
		},
	}

	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		discoveryMethods:  []DiscoveryMethod{mockMethod},
		stopChan:          make(chan struct{}),
		logger:            log,
	}

	// Run concurrent discoveries
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			_ = discovery.DiscoverServers(ctx)
		}()
	}

	wg.Wait()
}

// MockDiscoveryMethod implements DiscoveryMethod for testing
type MockDiscoveryMethod struct {
	name     string
	servers  []*DiscoveredServer
	err      error
	started  bool
	stopped  bool
}

func (m *MockDiscoveryMethod) Name() string {
	return m.name
}

func (m *MockDiscoveryMethod) Discover(ctx context.Context) ([]*DiscoveredServer, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.servers, nil
}

func (m *MockDiscoveryMethod) Start(ctx context.Context) error {
	m.started = true
	return nil
}

func (m *MockDiscoveryMethod) Stop() error {
	m.stopped = true
	return nil
}

// Edge case tests

func TestProtocolDiscovery_RegisterServer_SpecialCharacters(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		logger:            log,
	}

	// Register server with special characters in name
	err := discovery.RegisterServer("mcp", "127.0.0.1", 3000, "Server with spaces & special chars!")

	require.NoError(t, err)
	servers := discovery.GetDiscoveredServers()
	require.Len(t, servers, 1)
	assert.Equal(t, "Server with spaces & special chars!", servers[0].Name)
}

func TestProtocolDiscovery_RegisterServer_IPv6(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		logger:            log,
	}

	err := discovery.RegisterServer("mcp", "::1", 3000, "IPv6 Server")

	require.NoError(t, err)
	servers := discovery.GetDiscoveredServers()
	require.Len(t, servers, 1)
	assert.Equal(t, "::1", servers[0].Address)
}

func TestProtocolDiscovery_RegisterServer_AllProtocols(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		logger:            log,
	}

	protocols := []string{"mcp", "lsp", "acp", "custom"}

	for i, proto := range protocols {
		err := discovery.RegisterServer(proto, "127.0.0.1", 3000+i, proto+" server")
		require.NoError(t, err)
	}

	servers := discovery.GetDiscoveredServers()
	assert.Len(t, servers, 4)

	for _, proto := range protocols {
		protoServers := discovery.GetServersByProtocol(proto)
		assert.Len(t, protoServers, 1)
	}
}

// Benchmark tests

func BenchmarkProtocolDiscovery_RegisterServer(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		logger:            log,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = discovery.RegisterServer("mcp", "127.0.0.1", 3000+i, "Benchmark Server")
	}
}

func BenchmarkProtocolDiscovery_GetServersByProtocol(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		logger:            log,
	}

	// Add some servers
	for i := 0; i < 100; i++ {
		protocol := "mcp"
		if i%2 == 0 {
			protocol = "lsp"
		}
		discovery.discoveredServers[string(rune(i))] = &DiscoveredServer{
			ID:       string(rune(i)),
			Protocol: protocol,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = discovery.GetServersByProtocol("mcp")
	}
}

func BenchmarkProtocolDiscovery_GetDiscoveredServers(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		logger:            log,
	}

	// Add 100 servers
	for i := 0; i < 100; i++ {
		discovery.discoveredServers[string(rune(i))] = &DiscoveredServer{
			ID:       string(rune(i)),
			Protocol: "mcp",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = discovery.GetDiscoveredServers()
	}
}

// Table-driven tests

func TestProtocolDiscovery_GetServersByProtocol_TableDriven(t *testing.T) {
	log := newFederationTestLogger()

	tests := []struct {
		name         string
		servers      map[string]*DiscoveredServer
		protocol     string
		expectedLen  int
	}{
		{
			name:         "empty servers",
			servers:      map[string]*DiscoveredServer{},
			protocol:     "mcp",
			expectedLen:  0,
		},
		{
			name: "single matching server",
			servers: map[string]*DiscoveredServer{
				"s1": {ID: "s1", Protocol: "mcp"},
			},
			protocol:    "mcp",
			expectedLen: 1,
		},
		{
			name: "no matching servers",
			servers: map[string]*DiscoveredServer{
				"s1": {ID: "s1", Protocol: "lsp"},
				"s2": {ID: "s2", Protocol: "acp"},
			},
			protocol:    "mcp",
			expectedLen: 0,
		},
		{
			name: "multiple matching servers",
			servers: map[string]*DiscoveredServer{
				"s1": {ID: "s1", Protocol: "mcp"},
				"s2": {ID: "s2", Protocol: "mcp"},
				"s3": {ID: "s3", Protocol: "lsp"},
			},
			protocol:    "mcp",
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discovery := &ProtocolDiscovery{
				discoveredServers: tt.servers,
				logger:            log,
			}

			result := discovery.GetServersByProtocol(tt.protocol)
			assert.Len(t, result, tt.expectedLen)
		})
	}
}

func TestProtocolDiscovery_checkServerHealth_TableDriven(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		logger: log,
	}

	tests := []struct {
		name           string
		server         *DiscoveredServer
		expectedStatus ServerStatus
	}{
		{
			name: "unknown protocol",
			server: &DiscoveredServer{
				Protocol: "unknown",
				Address:  "127.0.0.1",
				Port:     9999,
			},
			expectedStatus: StatusUnknown,
		},
		{
			name: "mcp offline",
			server: &DiscoveredServer{
				Protocol: "mcp",
				Address:  "127.0.0.1",
				Port:     59990,
			},
			expectedStatus: StatusOffline,
		},
		{
			name: "lsp offline",
			server: &DiscoveredServer{
				Protocol: "lsp",
				Address:  "127.0.0.1",
				Port:     59991,
			},
			expectedStatus: StatusOffline,
		},
		{
			name: "acp offline",
			server: &DiscoveredServer{
				Protocol: "acp",
				Address:  "127.0.0.1",
				Port:     59992,
			},
			expectedStatus: StatusOffline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			status := discovery.checkServerHealth(ctx, tt.server)
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

// Tests for default discovery methods initialization

func TestNewProtocolDiscovery_DefaultMethods(t *testing.T) {
	log := newFederationTestLogger()
	discovery := NewProtocolDiscovery(log)

	// Verify default methods are added
	require.Len(t, discovery.discoveryMethods, 3)

	// Check method names
	names := make([]string, 0, 3)
	for _, method := range discovery.discoveryMethods {
		names = append(names, method.Name())
	}

	assert.Contains(t, names, "network")
	assert.Contains(t, names, "dns")
	assert.Contains(t, names, "config")
}

// Test for periodic discovery goroutine behavior

func TestProtocolDiscovery_PeriodicDiscovery_Stop(t *testing.T) {
	log := newFederationTestLogger()
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		discoveryMethods:  []DiscoveryMethod{},
		stopChan:          make(chan struct{}),
		logger:            log,
	}

	// Start discovery
	ctx := context.Background()
	err := discovery.Start(ctx)
	require.NoError(t, err)

	// Wait a bit for goroutine to start
	time.Sleep(10 * time.Millisecond)

	// Stop should not hang
	done := make(chan struct{})
	go func() {
		discovery.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good, stopped successfully
	case <-time.After(1 * time.Second):
		t.Fatal("Stop() timed out")
	}
}

// Tests for error handling in discovery methods

func TestProtocolDiscovery_DiscoverServers_AllMethodsFail(t *testing.T) {
	log := newFederationTestLogger()

	failingMethod1 := &MockDiscoveryMethod{name: "fail1", err: errors.New("fail1")}
	failingMethod2 := &MockDiscoveryMethod{name: "fail2", err: errors.New("fail2")}

	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		discoveryMethods:  []DiscoveryMethod{failingMethod1, failingMethod2},
		stopChan:          make(chan struct{}),
		logger:            log,
	}

	ctx := context.Background()
	err := discovery.DiscoverServers(ctx)

	// Should not return error even if all methods fail
	require.NoError(t, err)
	// But should have no discovered servers
	assert.Empty(t, discovery.discoveredServers)
}

func TestProtocolDiscovery_HealthCheck_ConcurrentServers(t *testing.T) {
	log := newFederationTestLogger()

	// Add multiple servers
	servers := make(map[string]*DiscoveredServer)
	for i := 0; i < 10; i++ {
		id := string(rune('a' + i))
		servers[id] = &DiscoveredServer{
			ID:       id,
			Protocol: "mcp",
			Address:  "127.0.0.1",
			Port:     59900 + i,
			Status:   StatusOnline,
		}
	}

	discovery := &ProtocolDiscovery{
		discoveredServers: servers,
		logger:            log,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Health check should complete for all servers
	err := discovery.HealthCheck(ctx)
	require.NoError(t, err)
}
