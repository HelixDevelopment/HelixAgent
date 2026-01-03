package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockUnifiedProtocolManager mocks the UnifiedProtocolManager for testing
type MockUnifiedProtocolManager struct {
	mock.Mock
}

func (m *MockUnifiedProtocolManager) ExecuteRequest(ctx context.Context, req UnifiedProtocolRequest) (UnifiedProtocolResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(UnifiedProtocolResponse), args.Error(1)
}

func (m *MockUnifiedProtocolManager) ListServers(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockUnifiedProtocolManager) GetMetrics(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockUnifiedProtocolManager) RefreshAll(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockUnifiedProtocolManager) ConfigureProtocols(ctx context.Context, config map[string]interface{}) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

// TestUnifiedProtocolManager_NewUnifiedProtocolManager tests creating a new unified protocol manager
func TestUnifiedProtocolManager_NewUnifiedProtocolManager(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress logs in tests

	// Test
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Assert
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.mcpManager)
	assert.NotNil(t, manager.lspManager)
	assert.NotNil(t, manager.acpManager)
	assert.NotNil(t, manager.embeddingManager)
	assert.NotNil(t, manager.log)
}

// TestUnifiedProtocolManager_ExecuteRequest_MCP tests MCP request execution
// Note: This is an integration test that requires a real MCP server
func TestUnifiedProtocolManager_ExecuteRequest_MCP(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Create a test API key
	security := manager.GetSecurity()
	testKey, err := security.CreateAPIKey("test-key", "test", []string{"mcp:*", "acp:*", "embedding:*", "lsp:*"})
	assert.NoError(t, err)

	// Try to connect a test MCP server - skip if no real server is available
	// This test requires a real MCP-compatible server to be running
	err = manager.mcpManager.ConnectServer(context.Background(), "test-server", "Test Server", "echo", []string{"test"})
	if err != nil {
		t.Skipf("Skipping MCP integration test - no MCP server available: %v", err)
	}

	req := UnifiedProtocolRequest{
		ProtocolType: "mcp",
		ServerID:     "test-server",
		ToolName:     "test-tool",
		Arguments:    map[string]interface{}{"arg1": "value1"},
	}

	// Create context with API key
	ctx := context.WithValue(context.Background(), "api_key", testKey.Key)

	// Test
	response, err := manager.ExecuteRequest(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "mcp", response.Protocol)
	assert.NotNil(t, response.Result)
}

// TestUnifiedProtocolManager_ExecuteRequest_ACP tests ACP request execution
func TestUnifiedProtocolManager_ExecuteRequest_ACP(t *testing.T) {
	// Create a mock ACP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ACPProtocolResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  map[string]interface{}{"status": "success", "data": "test-result"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Register a mock ACP server (accessing private field since we're in same package)
	err := manager.acpManager.RegisterServer(&ACPServer{
		ID:      "opencode-1",
		Name:    "Test ACP Server",
		URL:     mockServer.URL,
		Enabled: true,
	})
	require.NoError(t, err)

	// Create a test API key
	security := manager.GetSecurity()
	testKey, err := security.CreateAPIKey("test-key", "test", []string{"mcp:*", "acp:*", "embedding:*", "lsp:*"})
	assert.NoError(t, err)

	req := UnifiedProtocolRequest{
		ProtocolType: "acp",
		ServerID:     "opencode-1",
		ToolName:     "test-action",
		Arguments:    map[string]interface{}{"arg1": "value1"},
	}

	// Create context with API key
	ctx := context.WithValue(context.Background(), "api_key", testKey.Key)

	// Test
	response, err := manager.ExecuteRequest(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "acp", response.Protocol)
	assert.NotNil(t, response.Result)
}

// TestUnifiedProtocolManager_ExecuteRequest_Embedding tests embedding request execution
func TestUnifiedProtocolManager_ExecuteRequest_Embedding(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Create a test API key
	security := manager.GetSecurity()
	testKey, err := security.CreateAPIKey("test-key", "test", []string{"mcp:*", "acp:*", "embedding:*", "lsp:*"})
	assert.NoError(t, err)

	req := UnifiedProtocolRequest{
		ProtocolType: "embedding",
		Arguments:    map[string]interface{}{"text": "test text"},
	}

	// Create context with API key
	ctx := context.WithValue(context.Background(), "api_key", testKey.Key)

	// Test
	response, err := manager.ExecuteRequest(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "embedding", response.Protocol)
	assert.NotNil(t, response.Result)
}

// TestUnifiedProtocolManager_ExecuteRequest_UnsupportedProtocol tests unsupported protocol
func TestUnifiedProtocolManager_ExecuteRequest_UnsupportedProtocol(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Create a test API key
	security := manager.GetSecurity()
	testKey, err := security.CreateAPIKey("test-key", "test", []string{"mcp:*", "acp:*", "embedding:*", "lsp:*"})
	assert.NoError(t, err)

	req := UnifiedProtocolRequest{
		ProtocolType: "unsupported",
	}

	// Create context with API key
	ctx := context.WithValue(context.Background(), "api_key", testKey.Key)

	// Test
	response, err := manager.ExecuteRequest(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.False(t, response.Success)
	assert.Equal(t, "insufficient permissions for unsupported:execute", response.Error)
}

// TestUnifiedProtocolManager_ListServers tests listing all servers
func TestUnifiedProtocolManager_ListServers(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Test
	servers, err := manager.ListServers(context.Background())

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, servers)
	assert.Contains(t, servers, "mcp")
	assert.Contains(t, servers, "acp")
	assert.Contains(t, servers, "lsp")
	assert.Contains(t, servers, "embedding")
}

// TestUnifiedProtocolManager_GetMetrics tests getting metrics
func TestUnifiedProtocolManager_GetMetrics(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Test
	metrics, err := manager.GetMetrics(context.Background())

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Contains(t, metrics, "overall")
	assert.Contains(t, metrics, "mcp")
	assert.Contains(t, metrics, "acp")
	assert.Contains(t, metrics, "lsp")
	assert.Contains(t, metrics, "embedding")
}

// TestUnifiedProtocolManager_RefreshAll tests refreshing all protocols
func TestUnifiedProtocolManager_RefreshAll(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Test
	err := manager.RefreshAll(context.Background())

	// Assert
	assert.NoError(t, err)
}

// TestUnifiedProtocolManager_ConfigureProtocols tests protocol configuration
func TestUnifiedProtocolManager_ConfigureProtocols(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	config := map[string]interface{}{
		"mcp": map[string]interface{}{
			"enabled": true,
			"servers": []string{"server1", "server2"},
		},
		"acp": map[string]interface{}{
			"enabled": true,
		},
	}

	// Test
	err := manager.ConfigureProtocols(context.Background(), config)

	// Assert
	assert.NoError(t, err)
}

// TestUnifiedProtocolManager_ExecuteRequest_NoAPIKey tests execution without API key
func TestUnifiedProtocolManager_ExecuteRequest_NoAPIKey(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	req := UnifiedProtocolRequest{
		ProtocolType: "mcp",
		ServerID:     "test-server",
		ToolName:     "test-tool",
		Arguments:    map[string]interface{}{},
	}

	// Context without API key
	ctx := context.Background()

	response, err := manager.ExecuteRequest(ctx, req)

	assert.Error(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "API key required")
}

// TestUnifiedProtocolManager_ExecuteRequest_RateLimitExceeded tests rate limiting
func TestUnifiedProtocolManager_ExecuteRequest_RateLimitExceeded(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Create API key
	security := manager.GetSecurity()
	testKey, err := security.CreateAPIKey("rate-limit-test", "test", []string{"mcp:*"})
	require.NoError(t, err)

	// Set very low rate limit
	manager.rateLimiter = NewRateLimiter(1) // Only 1 request allowed

	req := UnifiedProtocolRequest{
		ProtocolType: "mcp",
		ServerID:     "test-server",
		ToolName:     "test-tool",
		Arguments:    map[string]interface{}{},
	}

	ctx := context.WithValue(context.Background(), "api_key", testKey.Key)

	// First request should succeed (or fail for other reasons)
	_, _ = manager.ExecuteRequest(ctx, req)

	// Subsequent request should hit rate limit
	response, err := manager.ExecuteRequest(ctx, req)

	assert.Error(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "Rate limit exceeded")
}

// TestUnifiedProtocolManager_ExecuteRequest_LSP tests LSP request execution
func TestUnifiedProtocolManager_ExecuteRequest_LSP(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Create API key with LSP permissions
	security := manager.GetSecurity()
	testKey, err := security.CreateAPIKey("lsp-test", "test", []string{"lsp:*"})
	require.NoError(t, err)

	req := UnifiedProtocolRequest{
		ProtocolType: "lsp",
		ServerID:     "test-server",
		ToolName:     "completion",
		Arguments: map[string]interface{}{
			"fileURI": "file:///test.go",
			"line":    10,
			"column":  5,
		},
	}

	ctx := context.WithValue(context.Background(), "api_key", testKey.Key)

	// Execute request - may fail due to no actual server, but should hit the LSP branch
	response, _ := manager.ExecuteRequest(ctx, req)

	// At minimum, protocol should be set
	assert.Equal(t, "lsp", response.Protocol)
}

// TestUnifiedProtocolManager_GetSecurity tests security getter
func TestUnifiedProtocolManager_GetSecurity(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	security := manager.GetSecurity()
	assert.NotNil(t, security)
}

// TestUnifiedProtocolManager_GetMonitor tests monitor getter
func TestUnifiedProtocolManager_GetMonitor(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	monitor := manager.GetMonitor()
	assert.NotNil(t, monitor)
}

// TestUnifiedProtocolManager_ExecuteRequest_SecurityValidationFailed tests security validation failure
func TestUnifiedProtocolManager_ExecuteRequest_SecurityValidationFailed(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Create an API key with limited permissions (no mcp:* permission)
	security := manager.GetSecurity()
	testKey, err := security.CreateAPIKey("limited-key", "test", []string{"lsp:*"}) // Only LSP access
	assert.NoError(t, err)

	req := UnifiedProtocolRequest{
		ProtocolType: "mcp", // Trying to access MCP without permission
		ServerID:     "test-server",
		ToolName:     "test-tool",
		Arguments:    map[string]interface{}{},
	}

	ctx := context.WithValue(context.Background(), "api_key", testKey.Key)

	response, err := manager.ExecuteRequest(ctx, req)

	assert.Error(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "insufficient permissions")
}

// TestUnifiedProtocolManager_ExecuteRequest_EmbeddingMissingText tests embedding with missing text
func TestUnifiedProtocolManager_ExecuteRequest_EmbeddingMissingText(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	security := manager.GetSecurity()
	testKey, err := security.CreateAPIKey("test-key", "test", []string{"embedding:*"})
	assert.NoError(t, err)

	req := UnifiedProtocolRequest{
		ProtocolType: "embedding",
		ServerID:     "embedding-server",
		ToolName:     "generate",
		Arguments:    map[string]interface{}{}, // Missing "text" argument
	}

	ctx := context.WithValue(context.Background(), "api_key", testKey.Key)

	response, err := manager.ExecuteRequest(ctx, req)

	assert.Error(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, err.Error(), "text argument is required")
}

// TestUnifiedProtocolManager_ExecuteRequest_EmbeddingWrongTextType tests embedding with wrong text type
func TestUnifiedProtocolManager_ExecuteRequest_EmbeddingWrongTextType(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	security := manager.GetSecurity()
	testKey, err := security.CreateAPIKey("test-key", "test", []string{"embedding:*"})
	assert.NoError(t, err)

	req := UnifiedProtocolRequest{
		ProtocolType: "embedding",
		ServerID:     "embedding-server",
		ToolName:     "generate",
		Arguments:    map[string]interface{}{"text": 12345}, // Wrong type - should be string
	}

	ctx := context.WithValue(context.Background(), "api_key", testKey.Key)

	response, err := manager.ExecuteRequest(ctx, req)

	assert.Error(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, err.Error(), "text argument is required")
}

// TestUnifiedProtocolManager_RecordMetrics tests the recordMetrics private method through execute
func TestUnifiedProtocolManager_RecordMetrics_Extended(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// recordMetrics is private but we can test through ExecuteRequest
	// which calls recordMetrics internally
	security := manager.GetSecurity()
	testKey, _ := security.CreateAPIKey("test-key", "test", []string{"embedding:*"})

	req := UnifiedProtocolRequest{
		ProtocolType: "embedding",
		ServerID:     "embedding-server",
		ToolName:     "generate",
		Arguments:    map[string]interface{}{"text": "test text"},
	}

	ctx := context.WithValue(context.Background(), "api_key", testKey.Key)

	// Execute request - this will internally call recordMetrics
	response, err := manager.ExecuteRequest(ctx, req)

	// Verify response
	assert.NoError(t, err)
	assert.True(t, response.Success)
}

// TestExtractAPIKeyFromContext_Extended tests additional extractAPIKeyFromContext scenarios
func TestExtractAPIKeyFromContext_Extended(t *testing.T) {
	t.Run("extracts valid API key from context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "api_key", "test-api-key-123")
		apiKey := extractAPIKeyFromContext(ctx)
		assert.Equal(t, "test-api-key-123", apiKey)
	})

	t.Run("returns empty string when no API key in context", func(t *testing.T) {
		ctx := context.Background()
		apiKey := extractAPIKeyFromContext(ctx)
		assert.Equal(t, "", apiKey)
	})

	t.Run("returns empty string when API key has wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "api_key", 12345)
		apiKey := extractAPIKeyFromContext(ctx)
		assert.Equal(t, "", apiKey)
	})
}

// TestUnifiedProtocolManager_GetACP_Extended tests the GetACP getter
func TestUnifiedProtocolManager_GetACP_Extended(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	acp := manager.GetACP()
	assert.NotNil(t, acp)
}
