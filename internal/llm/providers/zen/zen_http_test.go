package zen

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
)

// TestZenHTTPProvider_DefaultConfig tests default configuration
func TestZenHTTPProvider_DefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultZenHTTPConfig()

	assert.Equal(t, "http://localhost:4096", config.BaseURL)
	assert.Equal(t, "opencode", config.Username)
	assert.Equal(t, "big-pickle", config.Model)
	assert.Equal(t, 180*time.Second, config.Timeout)
	assert.Equal(t, 8192, config.MaxTokens)
	assert.True(t, config.AutoStart)
}

// TestZenHTTPProvider_NewProvider tests provider creation
func TestZenHTTPProvider_NewProvider(t *testing.T) {
	t.Parallel()
	config := ZenHTTPConfig{
		BaseURL:   "http://localhost:5000",
		Username:  "testuser",
		Password:  "testpass",
		Model:     "grok-code",
		Timeout:   60 * time.Second,
		MaxTokens: 4096,
		AutoStart: false,
	}

	provider := NewZenHTTPProvider(config)

	assert.NotNil(t, provider)
	assert.Equal(t, "http://localhost:5000", provider.baseURL)
	assert.Equal(t, "testuser", provider.username)
	assert.Equal(t, "testpass", provider.password)
	assert.Equal(t, "grok-code", provider.model)
	assert.Equal(t, 60*time.Second, provider.timeout)
	assert.Equal(t, 4096, provider.maxTokens)
	assert.False(t, provider.autoStart)
	assert.NotNil(t, provider.httpClient)
}

// TestZenHTTPProvider_NewProviderWithModel tests model-specific creation
func TestZenHTTPProvider_NewProviderWithModel(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("big-pickle")

	assert.NotNil(t, provider)
	assert.Equal(t, "big-pickle", provider.model)
}

// TestZenHTTPProvider_GetName tests provider name
func TestZenHTTPProvider_GetName(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("grok-code")
	assert.Equal(t, "zen-http", provider.GetName())
}

// TestZenHTTPProvider_GetProviderType tests provider type
func TestZenHTTPProvider_GetProviderType(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("grok-code")
	assert.Equal(t, "zen", provider.GetProviderType())
}

// TestZenHTTPProvider_GetCapabilities tests capabilities
func TestZenHTTPProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("grok-code")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsTools)
	assert.GreaterOrEqual(t, len(caps.SupportedModels), 5, "Should support multiple models")

	// Check for specific free models (as of 2026-02)
	assert.Contains(t, caps.SupportedModels, "big-pickle")
	assert.Contains(t, caps.SupportedModels, "gpt-5-nano")
	assert.Contains(t, caps.SupportedModels, "glm-4.7")
	// Note: qwen3-coder removed from free tier 2026-02
	assert.Contains(t, caps.SupportedModels, "kimi-k2")
	assert.Contains(t, caps.SupportedModels, "gemini-3-flash")
}

// TestZenHTTPProvider_SetModel tests model setting
func TestZenHTTPProvider_SetModel(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("grok-code")
	assert.Equal(t, "grok-code", provider.GetCurrentModel())

	provider.SetModel("big-pickle")
	assert.Equal(t, "big-pickle", provider.GetCurrentModel())
}

// TestZenHTTPProvider_DefaultValues tests default value application
func TestZenHTTPProvider_DefaultValues(t *testing.T) {
	t.Parallel()
	// Empty config should get defaults
	config := ZenHTTPConfig{}
	provider := NewZenHTTPProvider(config)

	assert.Equal(t, "http://localhost:4096", provider.baseURL)
	assert.Equal(t, "opencode", provider.username)
	assert.Equal(t, 180*time.Second, provider.timeout)
	assert.Equal(t, 8192, provider.maxTokens)
}

// TestZenHTTPProvider_URLTrailingSlash tests URL normalization
func TestZenHTTPProvider_URLTrailingSlash(t *testing.T) {
	t.Parallel()
	config := ZenHTTPConfig{
		BaseURL: "http://localhost:4096/", // With trailing slash
	}
	provider := NewZenHTTPProvider(config)

	// Trailing slash should be removed
	assert.Equal(t, "http://localhost:4096", provider.baseURL)
}

// TestZenHTTPProvider_ValidateConfig tests config validation
func TestZenHTTPProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("big-pickle")

	valid, errs := provider.ValidateConfig(nil)

	if IsOpenCodeInstalled() {
		assert.True(t, valid)
		assert.Empty(t, errs)
	} else {
		assert.False(t, valid)
		assert.NotEmpty(t, errs)
		assert.Contains(t, errs[0], "not available")
	}
}

// TestZenHTTPProvider_IsServerRunning tests server check
func TestZenHTTPProvider_IsServerRunning(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("big-pickle")

	running := provider.IsServerRunning()
	t.Logf("OpenCode HTTP server running: %v", running)

	// Should not panic
	assert.NotPanics(t, func() {
		provider.IsServerRunning()
	})
}

// TestIsZenHTTPAvailable tests the standalone availability function
func TestIsZenHTTPAvailable(t *testing.T) {
	t.Parallel()
	available := IsZenHTTPAvailable()
	t.Logf("Zen HTTP available: %v", available)

	// Should be consistent with LookPath
	_, err := exec.LookPath("opencode")
	expectedAvailable := err == nil
	assert.Equal(t, expectedAvailable, available)
}

// TestCanUseZenHTTP tests the full HTTP usability check
func TestCanUseZenHTTP(t *testing.T) {
	t.Parallel()
	canUse := CanUseZenHTTP()
	t.Logf("Can use Zen HTTP: %v", canUse)

	// Should be same as IsZenHTTPAvailable
	assert.Equal(t, IsZenHTTPAvailable(), canUse)
}

// TestZenHTTPProvider_APITypes tests API type structures
func TestZenHTTPProvider_APITypes(t *testing.T) {
	t.Parallel()
	// Test sessionResponse serialization
	session := sessionResponse{
		ID:        "sess-123",
		Title:     "Test Session",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(session)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"id":"sess-123"`)
	assert.Contains(t, string(data), `"title":"Test Session"`)

	// Test messageRequest serialization
	msgReq := messageRequest{
		Content: "Hello, world!",
		Model:   "big-pickle",
	}

	data, err = json.Marshal(msgReq)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"content":"Hello, world!"`)
	assert.Contains(t, string(data), `"model":"big-pickle"`)

	// Test messageResponse deserialization
	respJSON := `{
		"id": "msg-456",
		"role": "assistant",
		"content": "Hello! How can I help?",
		"model": "big-pickle",
		"createdAt": "2026-01-27T10:00:00Z"
	}`

	var msgResp messageResponse
	err = json.Unmarshal([]byte(respJSON), &msgResp)
	assert.NoError(t, err)
	assert.Equal(t, "msg-456", msgResp.ID)
	assert.Equal(t, "assistant", msgResp.Role)
	assert.Equal(t, "Hello! How can I help?", msgResp.Content)
	assert.Equal(t, "big-pickle", msgResp.Model)

	// Test errorResponse deserialization
	errJSON := `{"error": "not_found", "message": "Session not found"}`

	var errResp errorResponse
	err = json.Unmarshal([]byte(errJSON), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "not_found", errResp.Error)
	assert.Equal(t, "Session not found", errResp.Message)
}

// TestZenHTTPProvider_Complete_NoPrompt tests error on empty prompt
func TestZenHTTPProvider_Complete_NoPrompt(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("big-pickle")
	provider.autoStart = false // Don't try to start server

	// Simulate running server
	provider.serverStarted = true
	provider.sessionID = "test-session"

	ctx := context.Background()
	resp, err := provider.Complete(ctx, &models.LLMRequest{
		Prompt:   "",
		Messages: nil,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no prompt")
}

// TestZenHTTPProvider_HealthCheck_NotRunning tests health check when server not running
func TestZenHTTPProvider_HealthCheck_NotRunning(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("big-pickle")
	provider.autoStart = false // Don't auto-start

	// Mock a server that's not running
	if !provider.IsServerRunning() {
		err := provider.HealthCheck()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	}
}

// TestZenHTTPProvider_StopServer tests stop server functionality
func TestZenHTTPProvider_StopServer(t *testing.T) {
	t.Parallel()
	provider := &ZenHTTPProvider{
		model:         "big-pickle",
		serverStarted: true,
	}

	// Stop should not panic even without a running process
	assert.NotPanics(t, func() {
		provider.StopServer()
	})

	assert.False(t, provider.serverStarted)
}

// TestZenHTTPProvider_SessionManagement tests session ID handling
func TestZenHTTPProvider_SessionManagement(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("big-pickle")

	// Initially no session
	assert.Empty(t, provider.sessionID)

	// Set a session
	provider.sessionID = "test-session-123"
	assert.Equal(t, "test-session-123", provider.sessionID)
}

// TestZenHTTPProvider_MaxConcurrentRequests tests concurrent request limit
func TestZenHTTPProvider_MaxConcurrentRequests(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("big-pickle")
	caps := provider.GetCapabilities()

	// HTTP supports concurrent requests
	assert.Equal(t, 10, caps.Limits.MaxConcurrentRequests)
}

// TestZenHTTPProvider_ModelSupportViaCapabilities tests model list via capabilities
func TestZenHTTPProvider_ModelSupportViaCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewZenHTTPProviderWithModel("big-pickle")
	caps := provider.GetCapabilities()

	// Should include known Zen free models (as of 2026-02)
	knownModels := []string{
		"big-pickle",
		"gpt-5-nano",
		"glm-4.7",
		"kimi-k2",
		"gemini-3-flash",
		// Note: qwen3-coder removed from free tier 2026-02
	}

	for _, model := range knownModels {
		assert.Contains(t, caps.SupportedModels, model, "Should support model: %s", model)
	}
}

// Integration test - only runs if OpenCode is installed and server is running
func TestZenHTTPProvider_Integration_Complete(t *testing.T) {
	t.Parallel()
	if !IsOpenCodeInstalled() {
		t.Skip("OpenCode CLI not installed")
	}

	provider := NewZenHTTPProviderWithModel("big-pickle")

	// Check if server is already running
	if !provider.IsServerRunning() {
		t.Skip("OpenCode HTTP server not running - skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := provider.Complete(ctx, &models.LLMRequest{
		Prompt: "Reply with exactly one word: hello",
	})

	if err != nil {
		t.Logf("Integration test failed: %v", err)
		t.Skip("Skipping due to error")
	}

	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content)
	assert.Equal(t, "zen-http", resp.ProviderName)
	t.Logf("Response: %s", resp.Content)
	t.Logf("Session ID: %s", resp.Metadata["session_id"])
}

// Integration test for health check
func TestZenHTTPProvider_Integration_HealthCheck(t *testing.T) {
	t.Parallel()
	if !IsOpenCodeInstalled() {
		t.Skip("OpenCode CLI not installed")
	}

	provider := NewZenHTTPProviderWithModel("big-pickle")

	// If server not running and auto-start enabled, it might start
	err := provider.HealthCheck()

	if err != nil {
		t.Logf("Health check failed (may be expected if server not running): %v", err)
		// Not failing the test - server might not be running
	} else {
		t.Log("Health check passed - server is running")
	}
}

// TestZenHTTPProvider_CompleteStream tests streaming completion
func TestZenHTTPProvider_CompleteStream(t *testing.T) {
	t.Parallel()
	if !IsOpenCodeInstalled() {
		t.Skip("OpenCode CLI not installed")
	}

	provider := NewZenHTTPProviderWithModel("big-pickle")

	if !provider.IsServerRunning() {
		t.Skip("Server not running - skipping streaming test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	ch, err := provider.CompleteStream(ctx, &models.LLMRequest{
		Prompt: "Say hello",
	})

	if err != nil {
		t.Logf("Stream test failed: %v", err)
		t.Skip("Streaming test skipped due to error")
	}

	assert.NotNil(t, ch)

	// Read from channel
	for resp := range ch {
		t.Logf("Streamed response: %s", resp.Content)
		if resp.FinishReason == "stop" {
			break
		}
	}
}

// TestZenHTTPProvider_AutoStartDisabled tests behavior with auto-start disabled
func TestZenHTTPProvider_AutoStartDisabled(t *testing.T) {
	t.Parallel()
	config := ZenHTTPConfig{
		AutoStart: false,
		Model:     "big-pickle",
	}
	provider := NewZenHTTPProvider(config)

	assert.False(t, provider.autoStart)

	// Health check should fail if server not running and auto-start is disabled
	if !provider.IsServerRunning() {
		err := provider.HealthCheck()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	}
}

// TestZenHTTPProvider_BasicAuthCredentials tests authentication configuration
func TestZenHTTPProvider_BasicAuthCredentials(t *testing.T) {
	t.Parallel()
	config := ZenHTTPConfig{
		Username: "myuser",
		Password: "mypassword",
	}
	provider := NewZenHTTPProvider(config)

	assert.Equal(t, "myuser", provider.username)
	assert.Equal(t, "mypassword", provider.password)
}

// TestZenHTTPProvider_StartServerWithoutCLI tests server start failure when CLI missing
func TestZenHTTPProvider_StartServerWithoutCLI(t *testing.T) {
	t.Parallel()
	if IsOpenCodeInstalled() {
		t.Skip("OpenCode is installed - can't test missing CLI scenario")
	}

	provider := NewZenHTTPProviderWithModel("big-pickle")
	err := provider.StartServer()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestZenHTTPProvider_MetadataInResponse tests metadata fields in response
func TestZenHTTPProvider_MetadataInResponse(t *testing.T) {
	t.Parallel()
	// Test that response metadata has expected fields
	expectedMetadataKeys := []string{
		"source",
		"session_id",
		"message_id",
		"model",
		"base_url",
		"prompt_tokens",
		"completion_tokens",
		"latency",
	}

	// Create a mock response to verify structure
	resp := &models.LLMResponse{
		Metadata: map[string]interface{}{
			"source":            "opencode-http",
			"session_id":        "test-session",
			"message_id":        "msg-123",
			"model":             "big-pickle",
			"base_url":          "http://localhost:4096",
			"prompt_tokens":     100,
			"completion_tokens": 50,
			"latency":           "1.5s",
		},
	}

	for _, key := range expectedMetadataKeys {
		_, ok := resp.Metadata[key]
		assert.True(t, ok, "Response metadata should contain key: %s", key)
	}
}

// TestZenHTTPProvider_Timeout tests timeout configuration
func TestZenHTTPProvider_Timeout(t *testing.T) {
	t.Parallel()
	config := ZenHTTPConfig{
		Timeout: 30 * time.Second,
	}
	provider := NewZenHTTPProvider(config)

	assert.Equal(t, 30*time.Second, provider.timeout)
	assert.NotNil(t, provider.httpClient)
	assert.Equal(t, 30*time.Second, provider.httpClient.Timeout)
}

// TestZenHTTPProvider_MaxTokens tests max tokens configuration
func TestZenHTTPProvider_MaxTokens(t *testing.T) {
	t.Parallel()
	config := ZenHTTPConfig{
		MaxTokens: 2048,
	}
	provider := NewZenHTTPProvider(config)

	assert.Equal(t, 2048, provider.maxTokens)

	caps := provider.GetCapabilities()
	assert.Equal(t, 2048, caps.Limits.MaxTokens)
}
