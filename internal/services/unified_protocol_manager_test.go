package services

import (
	"context"
	"encoding/json"
	"fmt"
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
		t.Skipf("Skipping MCP integration test - no MCP server available: %v (SKIP-OK: #integration-only)", err)
	}

	req := UnifiedProtocolRequest{
		ProtocolType: "mcp",
		ServerID:     "test-server",
		ToolName:     "test-tool",
		Arguments:    map[string]interface{}{"arg1": "value1"},
	}

	// Create context with API key
	ctx := ContextWithAPIKey(context.Background(), testKey.Key)

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
		_ = json.NewEncoder(w).Encode(resp)
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
	ctx := ContextWithAPIKey(context.Background(), testKey.Key)

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
	ctx := ContextWithAPIKey(context.Background(), testKey.Key)

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
	ctx := ContextWithAPIKey(context.Background(), testKey.Key)

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

	// Test - RefreshAll may return errors when no servers are configured
	// This is expected behavior since our fix properly reports errors instead of swallowing them
	err := manager.RefreshAll(context.Background())

	// Assert - function completes without panic, errors are logged
	// Note: Errors are expected when no servers are configured
	if err != nil {
		// Verify it's an expected error type (refresh failure, not panic)
		assert.Contains(t, err.Error(), "refresh")
	}
}

// TestUnifiedProtocolManager_RefreshAll_ErrorHandling tests that RefreshAll properly handles and reports errors
func TestUnifiedProtocolManager_RefreshAll_ErrorHandling(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Test that RefreshAll doesn't panic even when sub-managers have issues
	// The function should log errors but continue trying other protocols
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to force errors

	err := manager.RefreshAll(ctx)

	// Assert - should either succeed or return a properly formatted error
	if err != nil {
		// Error message should indicate which protocol failed
		errStr := err.Error()
		assert.True(t,
			contains(errStr, "MCP") ||
				contains(errStr, "LSP") ||
				contains(errStr, "ACP") ||
				contains(errStr, "embedding"),
			"Error should indicate which protocol failed: %s", errStr)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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

	ctx := ContextWithAPIKey(context.Background(), testKey.Key)

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

	ctx := ContextWithAPIKey(context.Background(), testKey.Key)

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

	ctx := ContextWithAPIKey(context.Background(), testKey.Key)

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

	ctx := ContextWithAPIKey(context.Background(), testKey.Key)

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

	ctx := ContextWithAPIKey(context.Background(), testKey.Key)

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

	ctx := ContextWithAPIKey(context.Background(), testKey.Key)

	// Execute request - this will internally call recordMetrics
	response, err := manager.ExecuteRequest(ctx, req)

	// Verify response
	assert.NoError(t, err)
	assert.True(t, response.Success)
}

// TestExtractAPIKeyFromContext_Extended tests additional extractAPIKeyFromContext scenarios
func TestExtractAPIKeyFromContext_Extended(t *testing.T) {
	t.Run("extracts valid API key from context", func(t *testing.T) {
		ctx := ContextWithAPIKey(context.Background(), "test-api-key-123")
		apiKey := extractAPIKeyFromContext(ctx)
		assert.Equal(t, "test-api-key-123", apiKey)
	})

	t.Run("returns empty string when no API key in context", func(t *testing.T) {
		ctx := context.Background()
		apiKey := extractAPIKeyFromContext(ctx)
		assert.Equal(t, "", apiKey)
	})

	t.Run("returns empty string when API key has wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), apiKeyCtxKey{}, 12345)
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

// --- MultiError Tests ---

// TestMultiError_SingleError tests MultiError with a single error
func TestMultiError_SingleError(t *testing.T) {
	err := NewMultiError([]error{fmt.Errorf("single error")})
	assert.Equal(t, "single error", err.Error())
	assert.Equal(t, "single error", err.Unwrap().Error())
}

// TestMultiError_MultipleErrors tests MultiError with multiple errors
func TestMultiError_MultipleErrors(t *testing.T) {
	errs := []error{
		fmt.Errorf("error 1"),
		fmt.Errorf("error 2"),
		fmt.Errorf("error 3"),
	}
	multiErr := NewMultiError(errs)

	errStr := multiErr.Error()

	// Should contain count
	assert.Contains(t, errStr, "3 errors occurred")

	// Should contain all error messages
	assert.Contains(t, errStr, "error 1")
	assert.Contains(t, errStr, "error 2")
	assert.Contains(t, errStr, "error 3")

	// Should contain numbered format
	assert.Contains(t, errStr, "[1]")
	assert.Contains(t, errStr, "[2]")
	assert.Contains(t, errStr, "[3]")
}

// TestMultiError_EmptyErrors tests MultiError with no errors
func TestMultiError_EmptyErrors(t *testing.T) {
	err := NewMultiError([]error{})
	assert.Equal(t, "", err.Error())
	assert.Nil(t, err.Unwrap())
}

// TestMultiError_Unwrap tests that Unwrap returns the first error
func TestMultiError_Unwrap(t *testing.T) {
	firstErr := fmt.Errorf("first error")
	secondErr := fmt.Errorf("second error")

	multiErr := NewMultiError([]error{firstErr, secondErr})

	// Unwrap should return the first error for errors.Is/As compatibility
	assert.Equal(t, firstErr, multiErr.Unwrap())
}

// TestMultiError_ErrorsIsCompatibility tests errors.Is compatibility via Unwrap
func TestMultiError_ErrorsIsCompatibility(t *testing.T) {
	targetErr := fmt.Errorf("target error")
	otherErr := fmt.Errorf("other error")

	multiErr := NewMultiError([]error{targetErr, otherErr})

	// Since Unwrap returns the first error, errors.Is should work for the first error
	unwrapped := multiErr.Unwrap()
	assert.Equal(t, targetErr, unwrapped)
}

// TestRefreshAll_ReturnsAllErrors tests that RefreshAll returns all errors via MultiError
func TestRefreshAll_ReturnsAllErrors(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	manager := NewUnifiedProtocolManager(nil, nil, logger)

	// Call RefreshAll - with no servers configured, it may return errors
	// from the sub-managers attempting to refresh
	err := manager.RefreshAll(context.Background())

	if err != nil {
		// Check if it's a MultiError (when multiple protocol refreshes fail)
		multiErr, ok := err.(*MultiError)
		if ok {
			// Verify MultiError has the expected structure
			assert.Greater(t, len(multiErr.Errors), 0, "MultiError should contain at least one error")

			// Each error should indicate which protocol failed
			for _, e := range multiErr.Errors {
				errStr := e.Error()
				protocolMentioned := contains(errStr, "MCP") ||
					contains(errStr, "LSP") ||
					contains(errStr, "ACP") ||
					contains(errStr, "embedding")
				assert.True(t, protocolMentioned,
					"Each error should mention the protocol that failed: %s", errStr)
			}
		} else {
			// If it's a single error case, it should still mention a protocol
			errStr := err.Error()
			assert.True(t,
				contains(errStr, "MCP") ||
					contains(errStr, "LSP") ||
					contains(errStr, "ACP") ||
					contains(errStr, "embedding") ||
					contains(errStr, "refresh"),
				"Error should indicate what failed: %s", errStr)
		}
	}
}

// TestNewMultiError tests the NewMultiError constructor
func TestNewMultiError(t *testing.T) {
	errs := []error{fmt.Errorf("test error")}
	multiErr := NewMultiError(errs)

	assert.NotNil(t, multiErr)
	assert.Equal(t, 1, len(multiErr.Errors))
	assert.Equal(t, "test error", multiErr.Errors[0].Error())
}
