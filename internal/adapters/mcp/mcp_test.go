package mcp_test

import (
	"context"
	"testing"
	"time"

	adapter "dev.helix.agent/internal/adapters/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mock MCP Adapter for RegistryAdapter tests
// ============================================================================

type mockMCPAdapter struct {
	name string
}

func (m *mockMCPAdapter) Name() string                        { return m.name }
func (m *mockMCPAdapter) Config() map[string]interface{}      { return nil }
func (m *mockMCPAdapter) Start(_ context.Context) error       { return nil }
func (m *mockMCPAdapter) Stop(_ context.Context) error        { return nil }
func (m *mockMCPAdapter) HealthCheck(_ context.Context) error { return nil }

// ============================================================================
// RegistryAdapter Tests
// ============================================================================

func TestNewRegistryAdapter(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	require.NotNil(t, r)
}

func TestRegistryAdapter_RegisterAndGet(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	mock := &mockMCPAdapter{name: "test-adapter"}

	err := r.Register(mock)
	require.NoError(t, err)

	got, ok := r.Get("test-adapter")
	assert.True(t, ok)
	assert.NotNil(t, got)
	assert.Equal(t, "test-adapter", got.Name())
}

func TestRegistryAdapter_Register_DuplicateName(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	mock1 := &mockMCPAdapter{name: "dup"}
	mock2 := &mockMCPAdapter{name: "dup"}

	err := r.Register(mock1)
	require.NoError(t, err)

	// Second registration with same name should fail
	err = r.Register(mock2)
	assert.Error(t, err)
}

func TestRegistryAdapter_Get_NotFound(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	got, ok := r.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestRegistryAdapter_List(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	list := r.List()
	assert.NotNil(t, list)
	assert.Empty(t, list)

	r.Register(&mockMCPAdapter{name: "a1"})
	r.Register(&mockMCPAdapter{name: "a2"})

	list = r.List()
	assert.Len(t, list, 2)
}

func TestRegistryAdapter_Unregister(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	r.Register(&mockMCPAdapter{name: "to-remove"})

	err := r.Unregister("to-remove")
	require.NoError(t, err)

	_, ok := r.Get("to-remove")
	assert.False(t, ok)
}

func TestRegistryAdapter_Unregister_NotFound(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	err := r.Unregister("nonexistent")
	assert.Error(t, err)
}

func TestRegistryAdapter_StartAll_Empty(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	err := r.StartAll(context.Background())
	assert.NoError(t, err)
}

func TestRegistryAdapter_StartAll_WithAdapters(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	r.Register(&mockMCPAdapter{name: "s1"})
	r.Register(&mockMCPAdapter{name: "s2"})

	err := r.StartAll(context.Background())
	assert.NoError(t, err)
}

func TestRegistryAdapter_StopAll(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	r.Register(&mockMCPAdapter{name: "s1"})

	err := r.StartAll(context.Background())
	require.NoError(t, err)

	err = r.StopAll(context.Background())
	assert.NoError(t, err)
}

func TestRegistryAdapter_HealthCheckAll_Empty(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	results := r.HealthCheckAll(context.Background())
	assert.NotNil(t, results)
	assert.Empty(t, results)
}

func TestRegistryAdapter_HealthCheckAll_WithAdapters(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	r.Register(&mockMCPAdapter{name: "h1"})
	r.Register(&mockMCPAdapter{name: "h2"})

	results := r.HealthCheckAll(context.Background())
	assert.NotNil(t, results)
	// Both adapters return nil error
	for _, err := range results {
		assert.NoError(t, err)
	}
}

// ============================================================================
// ServerConfig.ToExternalConfig Tests
// ============================================================================

func TestServerConfig_ToExternalConfig_Basic(t *testing.T) {
	t.Parallel()
	cfg := &adapter.ServerConfig{
		Name:    "my-mcp-server",
		Command: []string{"npx", "-y", "my-server"},
		Env:     map[string]string{"API_KEY": "test"},
		Timeout: 30 * time.Second,
	}

	extCfg := cfg.ToExternalConfig()
	assert.Equal(t, "my-mcp-server", extCfg.ClientName)
	assert.Equal(t, "npx", extCfg.ServerCommand)
	assert.Equal(t, []string{"-y", "my-server"}, extCfg.ServerArgs)
	assert.Equal(t, "test", extCfg.ServerEnv["API_KEY"])
	assert.Equal(t, 30*time.Second, extCfg.Timeout)
}

func TestServerConfig_ToExternalConfig_EmptyCommand(t *testing.T) {
	t.Parallel()
	cfg := &adapter.ServerConfig{
		Name: "empty-cmd",
	}
	extCfg := cfg.ToExternalConfig()
	assert.Equal(t, "empty-cmd", extCfg.ClientName)
	assert.Empty(t, extCfg.ServerCommand)
}

func TestServerConfig_ToExternalConfig_SingleCommand(t *testing.T) {
	t.Parallel()
	cfg := &adapter.ServerConfig{
		Name:    "single",
		Command: []string{"myserver"},
	}
	extCfg := cfg.ToExternalConfig()
	assert.Equal(t, "myserver", extCfg.ServerCommand)
	assert.Nil(t, extCfg.ServerArgs)
}

func TestServerConfig_ToExternalConfig_ZeroTimeout(t *testing.T) {
	t.Parallel()
	cfg := &adapter.ServerConfig{
		Name:    "no-timeout",
		Timeout: 0,
	}
	extCfg := cfg.ToExternalConfig()
	// Zero timeout should keep the default
	assert.NotEqual(t, 0, extCfg.Timeout)
}

// ============================================================================
// DefaultClientConfig Tests
// ============================================================================

func TestDefaultClientConfig(t *testing.T) {
	t.Parallel()
	cfg := adapter.DefaultClientConfig()
	// Should return a valid non-zero config
	assert.NotEmpty(t, cfg.Transport)
}

// ============================================================================
// Transport constants
// ============================================================================

func TestTransportConstants(t *testing.T) {
	t.Parallel()
	// Ensure constants are accessible and distinct
	assert.NotEqual(t, adapter.TransportStdio, adapter.TransportHTTP)
}

// ============================================================================
// Type aliases (compile-time verification)
// ============================================================================

func TestTypeAliases(t *testing.T) {
	t.Parallel()
	// Verify type aliases compile - create nil instances of each alias type
	var _ adapter.Tool
	var _ adapter.ToolResult
	var _ adapter.Resource
	var _ adapter.Prompt
	var _ adapter.Request
	var _ adapter.Response

	t.Log("All MCP type aliases are valid")
}
