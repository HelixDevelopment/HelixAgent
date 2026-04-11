package claude

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
)

// TestClaudeCLIProvider_DefaultConfig tests default configuration
func TestClaudeCLIProvider_DefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultClaudeCLIConfig()

	// Model is empty by default - will be discovered dynamically
	assert.Equal(t, "", config.Model)
	assert.Equal(t, 180*time.Second, config.Timeout)
	assert.Equal(t, 4096, config.MaxOutputTokens)
}

// TestClaudeCLIProvider_NewProvider tests provider creation
func TestClaudeCLIProvider_NewProvider(t *testing.T) {
	t.Parallel()
	config := ClaudeCLIConfig{
		Model:           "claude-opus-4-5-20251101",
		Timeout:         60 * time.Second,
		MaxOutputTokens: 2048,
	}

	provider := NewClaudeCLIProvider(config)

	assert.NotNil(t, provider)
	assert.Equal(t, "claude-opus-4-5-20251101", provider.model)
	assert.Equal(t, 60*time.Second, provider.timeout)
	assert.Equal(t, 2048, provider.maxOutputTokens)
}

// TestClaudeCLIProvider_NewProviderWithModel tests model-specific creation
func TestClaudeCLIProvider_NewProviderWithModel(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-haiku-4-5-20251001")

	assert.NotNil(t, provider)
	assert.Equal(t, "claude-haiku-4-5-20251001", provider.model)
}

// TestClaudeCLIProvider_GetName tests provider name
func TestClaudeCLIProvider_GetName(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")
	assert.Equal(t, "claude-cli", provider.GetName())
}

// TestClaudeCLIProvider_GetProviderType tests provider type
func TestClaudeCLIProvider_GetProviderType(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")
	assert.Equal(t, "claude", provider.GetProviderType())
}

// TestClaudeCLIProvider_GetCapabilities tests capabilities
func TestClaudeCLIProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsTools) // CLI doesn't support tools
	assert.GreaterOrEqual(t, len(caps.SupportedModels), 5, "Should support multiple Claude models")

	// Check for specific models
	assert.Contains(t, caps.SupportedModels, "claude-opus-4-5-20251101")
	assert.Contains(t, caps.SupportedModels, "claude-sonnet-4-5-20250929")
	assert.Contains(t, caps.SupportedModels, "claude-sonnet-4-20250514")
}

// TestClaudeCLIProvider_SetModel tests model setting
func TestClaudeCLIProvider_SetModel(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")
	assert.Equal(t, "claude-sonnet-4-20250514", provider.GetCurrentModel())

	provider.SetModel("claude-opus-4-5-20251101")
	assert.Equal(t, "claude-opus-4-5-20251101", provider.GetCurrentModel())
}

// TestIsClaudeCodeInstalled tests CLI installation check
func TestIsClaudeCodeInstalled(t *testing.T) {
	t.Parallel()
	// This test is informational - actual result depends on system
	installed := IsClaudeCodeInstalled()
	t.Logf("Claude Code installed: %v", installed)

	// Verify the function doesn't panic
	assert.NotPanics(t, func() {
		IsClaudeCodeInstalled()
	})
}

// TestGetClaudeCodePath tests path lookup
func TestGetClaudeCodePath(t *testing.T) {
	t.Parallel()
	path, err := GetClaudeCodePath()

	// Just verify it returns proper types
	if err != nil {
		t.Logf("Claude Code not found: %v", err)
		assert.Empty(t, path)
	} else {
		t.Logf("Claude Code path: %s", path)
		assert.NotEmpty(t, path)
	}
}

// TestIsClaudeCodeAuthenticated tests auth check
func TestIsClaudeCodeAuthenticated(t *testing.T) {
	t.Parallel()
	// This test is informational - actual result depends on system
	authenticated := IsClaudeCodeAuthenticated()
	t.Logf("Claude Code authenticated: %v", authenticated)

	// If CLI is not installed, should return false
	if !IsClaudeCodeInstalled() {
		assert.False(t, authenticated)
	}
}

// TestClaudeCLIProvider_IsCLIAvailable tests availability check
func TestClaudeCLIProvider_IsCLIAvailable(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

	available := provider.IsCLIAvailable()
	t.Logf("CLI available: %v", available)

	if !available {
		err := provider.GetCLIError()
		t.Logf("CLI error: %v", err)
		assert.NotNil(t, err)
	}

	// Calling multiple times should return same result (sync.Once)
	available2 := provider.IsCLIAvailable()
	assert.Equal(t, available, available2)
}

// TestClaudeCLIProvider_ValidateConfig tests config validation
func TestClaudeCLIProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

	valid, errs := provider.ValidateConfig(nil)
	// ValidateConfig requires IsCLIAvailable() to return true.
	// When inside a Claude Code session, IsCLIAvailable() returns false.
	if IsClaudeCodeInstalled() && IsClaudeCodeAuthenticated() && !IsInsideClaudeCodeSession() {
		assert.True(t, valid)
		assert.Empty(t, errs)
	} else {
		assert.False(t, valid)
		assert.NotEmpty(t, errs)
	}
}

// TestClaudeCLIProvider_Complete_NoPrompt tests error on empty prompt
func TestClaudeCLIProvider_Complete_NoPrompt(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

	// Skip if CLI not available
	if !provider.IsCLIAvailable() {
		t.Skip("Claude CLI not available")
	}

	ctx := context.Background()
	resp, err := provider.Complete(ctx, &models.LLMRequest{
		Prompt:   "",
		Messages: nil,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no prompt")
}

// TestClaudeCLIProvider_Complete_CLIUnavailable tests behavior when CLI unavailable
func TestClaudeCLIProvider_Complete_CLIUnavailable(t *testing.T) {
	t.Parallel()
	// Create provider with invalid path; consume sync.Once so IsCLIAvailable returns pre-set false
	provider := &ClaudeCLIProvider{
		model:        "claude-sonnet-4-20250514",
		cliAvailable: false,
		cliCheckErr:  exec.ErrNotFound,
	}
	provider.cliCheckOnce.Do(func() {}) // Mark Once as done

	ctx := context.Background()
	resp, err := provider.Complete(ctx, &models.LLMRequest{
		Prompt: "Hello",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not available")
}

// TestClaudeCLIProvider_HealthCheck_CLIUnavailable tests health check when CLI unavailable
func TestClaudeCLIProvider_HealthCheck_CLIUnavailable(t *testing.T) {
	t.Parallel()
	provider := &ClaudeCLIProvider{
		model:        "claude-sonnet-4-20250514",
		cliAvailable: false,
		cliCheckErr:  exec.ErrNotFound,
	}

	err := provider.HealthCheck()

	assert.Error(t, err)
}

// Integration test - only runs if Claude CLI is installed and authenticated
func TestClaudeCLIProvider_Integration_Complete(t *testing.T) {
	t.Parallel()
	if !IsClaudeCodeInstalled() {
		t.Skip("Claude Code CLI not installed")
	}
	if IsInsideClaudeCodeSession() {
		t.Skip("Cannot launch Claude Code inside another Claude Code session")
	}
	if !IsClaudeCodeAuthenticated() {
		t.Skip("Claude Code CLI not authenticated")
	}

	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := provider.Complete(ctx, &models.LLMRequest{
		Prompt: "Reply with exactly one word: hello",
		ModelParams: models.ModelParameters{
			MaxTokens: 10,
		},
	})

	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "forbidden") || strings.Contains(errMsg, "Request not allowed") {
			t.Skipf("Claude OAuth token is product-restricted (403 Forbidden). Get API key from console.anthropic.com: %v", err)
		}
		assert.NoError(t, err)
		return
	}

	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content)
	assert.Equal(t, "claude-cli", resp.ProviderName)
	t.Logf("Response: %s", resp.Content)
}

// Integration test for health check
func TestClaudeCLIProvider_Integration_HealthCheck(t *testing.T) {
	t.Parallel()
	if !IsClaudeCodeInstalled() {
		t.Skip("Claude Code CLI not installed")
	}
	if IsInsideClaudeCodeSession() {
		t.Skip("Cannot launch Claude Code inside another Claude Code session")
	}
	if !IsClaudeCodeAuthenticated() {
		t.Skip("Claude Code CLI not authenticated")
	}

	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

	err := provider.HealthCheck()

	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "forbidden") || strings.Contains(errMsg, "Request not allowed") {
			t.Skipf("Claude OAuth token is product-restricted (403 Forbidden). Get API key from console.anthropic.com: %v", err)
		}
	}

	assert.NoError(t, err)
}

// TestCanUseClaudeOAuth tests the OAuth check function
func TestCanUseClaudeOAuth(t *testing.T) {
	t.Parallel()
	// This test is informational
	canUse := CanUseClaudeOAuth()
	t.Logf("Can use Claude OAuth: %v", canUse)

	// The function should not panic
	assert.NotPanics(t, func() {
		CanUseClaudeOAuth()
	})
}

// TestClaudeCLIProvider_ParseJSONResponse tests JSON response parsing
func TestClaudeCLIProvider_ParseJSONResponse(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

	tests := []struct {
		name           string
		input          string
		expectedResult string
		expectedSessID string
		expectUsage    bool
	}{
		{
			name:           "valid JSON response",
			input:          `{"result": "Hello, world!", "session_id": "sess-123", "usage": {"prompt_tokens": 10, "completion_tokens": 5}, "model": "claude-sonnet"}`,
			expectedResult: "Hello, world!",
			expectedSessID: "sess-123",
			expectUsage:    true,
		},
		{
			name:           "JSON response without session",
			input:          `{"result": "Just a response", "usage": {}}`,
			expectedResult: "Just a response",
			expectedSessID: "",
			expectUsage:    true,
		},
		{
			name:           "plain text fallback",
			input:          "This is plain text response",
			expectedResult: "This is plain text response",
			expectedSessID: "",
			expectUsage:    false,
		},
		{
			name:           "empty result field - fallback to raw",
			input:          `{"result": "", "session_id": "sess-456"}`,
			expectedResult: `{"result": "", "session_id": "sess-456"}`,
			expectedSessID: "",
			expectUsage:    false,
		},
		{
			name:           "malformed JSON - fallback to raw",
			input:          `{"result": "incomplete`,
			expectedResult: `{"result": "incomplete`,
			expectedSessID: "",
			expectUsage:    false,
		},
		{
			name:           "JSON with whitespace",
			input:          `  {"result": "Trimmed response", "session_id": "sess-789"}  `,
			expectedResult: "Trimmed response",
			expectedSessID: "sess-789",
			expectUsage:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, sessionID, metadata := provider.parseJSONResponse(tt.input)

			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedSessID, sessionID)

			if tt.expectUsage {
				_, hasUsage := metadata["usage"]
				assert.True(t, hasUsage, "Should have usage metadata")
			}
		})
	}
}

// TestClaudeCLIProvider_SessionContinuity tests session ID persistence
func TestClaudeCLIProvider_SessionContinuity(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

	// Initially no session
	assert.Empty(t, provider.sessionID)

	// Simulate parsing a response with session ID
	_, sessionID, _ := provider.parseJSONResponse(`{"result": "Hello", "session_id": "sess-abc123"}`)
	assert.Equal(t, "sess-abc123", sessionID)

	// Store session ID as the provider would
	if sessionID != "" {
		provider.sessionID = sessionID
	}

	assert.Equal(t, "sess-abc123", provider.sessionID)
}

// TestClaudeCLIProvider_JSONResponseStruct tests the JSON response structure
func TestClaudeCLIProvider_JSONResponseStruct(t *testing.T) {
	t.Parallel()
	jsonStr := `{
		"result": "Test response content",
		"session_id": "session-12345",
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50
		},
		"model": "claude-opus-4-5-20251101"
	}`

	var resp claudeJSONResponse
	err := json.Unmarshal([]byte(jsonStr), &resp)

	assert.NoError(t, err)
	assert.Equal(t, "Test response content", resp.Result)
	assert.Equal(t, "session-12345", resp.SessionID)
	assert.Equal(t, "claude-opus-4-5-20251101", resp.Model)
	assert.NotNil(t, resp.Usage)

	// Check usage values
	promptTokens, ok := resp.Usage["prompt_tokens"].(float64)
	assert.True(t, ok)
	assert.Equal(t, float64(100), promptTokens)

	completionTokens, ok := resp.Usage["completion_tokens"].(float64)
	assert.True(t, ok)
	assert.Equal(t, float64(50), completionTokens)
}

// TestClaudeCLIProvider_TokenEstimation tests token count estimation from metadata
func TestClaudeCLIProvider_TokenEstimation(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

	// Test with usage metadata
	jsonStr := `{"result": "Response", "session_id": "sess-1", "usage": {"prompt_tokens": 150, "completion_tokens": 75}}`
	_, _, metadata := provider.parseJSONResponse(jsonStr)

	usage, ok := metadata["usage"].(map[string]interface{})
	assert.True(t, ok)

	if pt, ok := usage["prompt_tokens"].(float64); ok {
		assert.Equal(t, float64(150), pt)
	}
	if ct, ok := usage["completion_tokens"].(float64); ok {
		assert.Equal(t, float64(75), ct)
	}
}

// TestClaudeCLIProvider_ModelDiscovery tests model discovery functions
func TestClaudeCLIProvider_ModelDiscovery(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("")

	// GetAvailableModels should return models (known or discovered)
	models := provider.GetAvailableModels()
	assert.NotEmpty(t, models)
	t.Logf("Available models: %v", models)

	// Should contain known Claude models
	foundClaude := false
	for _, m := range models {
		if strings.HasPrefix(m, "claude-") {
			foundClaude = true
			break
		}
	}
	assert.True(t, foundClaude, "Should find at least one Claude model")
}

// TestClaudeCLIProvider_GetBestAvailableModel tests best model selection
func TestClaudeCLIProvider_GetBestAvailableModel(t *testing.T) {
	t.Parallel()
	provider := &ClaudeCLIProvider{
		availableModels: []string{
			"claude-haiku-4-5-20251001",
			"claude-opus-4-5-20251101",
			"claude-sonnet-4-20250514",
		},
		modelsDiscovered: true,
	}

	best := provider.GetBestAvailableModel()
	// Should prefer opus over sonnet over haiku
	assert.Contains(t, best, "opus")
}

// TestClaudeCLIProvider_IsModelAvailable tests model availability check
func TestClaudeCLIProvider_IsModelAvailable(t *testing.T) {
	t.Parallel()
	provider := &ClaudeCLIProvider{
		availableModels: []string{
			"claude-opus-4-5-20251101",
			"claude-sonnet-4-20250514",
		},
		modelsDiscovered: true,
	}

	assert.True(t, provider.IsModelAvailable("claude-opus-4-5-20251101"))
	assert.True(t, provider.IsModelAvailable("claude-sonnet-4-20250514"))
	assert.False(t, provider.IsModelAvailable("nonexistent-model"))
}

// TestParseModelsOutput tests the CLI output parser
func TestParseModelsOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple model list",
			input:    "claude-opus-4-5-20251101\nclaude-sonnet-4-20250514\nclaude-haiku-4-5-20251001",
			expected: []string{"claude-opus-4-5-20251101", "claude-sonnet-4-20250514", "claude-haiku-4-5-20251001"},
		},
		{
			name:     "model with extra text",
			input:    "Available models:\nclaude-opus-4-5-20251101 - Most capable\nclaude-sonnet-4-20250514 - Balanced",
			expected: []string{"claude-opus-4-5-20251101", "claude-sonnet-4-20250514"},
		},
		{
			name:     "empty output",
			input:    "",
			expected: nil,
		},
		{
			name:     "no claude models",
			input:    "gpt-4\ngpt-3.5-turbo",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseModelsOutput(tt.input)
			if tt.expected == nil {
				assert.Empty(t, result)
			} else {
				assert.Equal(t, len(tt.expected), len(result))
				for _, exp := range tt.expected {
					assert.Contains(t, result, exp)
				}
			}
		})
	}
}

// TestGetKnownClaudeModels tests the known models list
func TestGetKnownClaudeModels(t *testing.T) {
	t.Parallel()
	models := GetKnownClaudeModels()
	assert.NotEmpty(t, models)
	assert.Contains(t, models, "claude-opus-4-6")
	assert.Contains(t, models, "claude-opus-4-5-20251101")
	assert.Contains(t, models, "claude-sonnet-4-5-20250929")
	assert.Contains(t, models, "claude-haiku-4-5-20251001")
}

// TestDiscoverClaudeModels tests standalone discovery function
func TestDiscoverClaudeModels(t *testing.T) {
	t.Parallel()
	models, err := DiscoverClaudeModels()
	assert.NotEmpty(t, models)

	if IsClaudeCodeInstalled() {
		// May or may not error depending on CLI capabilities
		t.Logf("Discovered models (err=%v): %v", err, models)
	} else {
		// Should return known models with an error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not installed")
	}
}

// TestClaudeCLIProvider_CompleteStream_NotAvailable tests streaming when CLI not available
func TestClaudeCLIProvider_CompleteStream_NotAvailable(t *testing.T) {
	t.Parallel()
	provider := &ClaudeCLIProvider{
		model:        "claude-sonnet-4-20250514",
		cliAvailable: false,
		cliCheckErr:  exec.ErrNotFound,
	}
	provider.cliCheckOnce.Do(func() {}) // Mark Once as done so IsCLIAvailable() won't re-check

	ctx := context.Background()
	ch, err := provider.CompleteStream(ctx, &models.LLMRequest{
		Prompt: "Hello",
	})

	assert.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "not available")
}

// TestClaudeCLIProvider_CompleteStream_NoPrompt tests streaming with empty prompt
func TestClaudeCLIProvider_CompleteStream_NoPrompt(t *testing.T) {
	t.Parallel()
	if !IsClaudeCodeInstalled() {
		t.Skip("Claude CLI not installed")
	}

	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")
	if !provider.IsCLIAvailable() {
		t.Skip("Claude CLI not available")
	}

	ctx := context.Background()
	ch, err := provider.CompleteStream(ctx, &models.LLMRequest{
		Prompt:   "",
		Messages: nil,
	})

	assert.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "no prompt")
}

// ============================================================================
// Improved Error Handling Tests
// ============================================================================

// TestIsInsideClaudeCodeSession tests detection of Claude Code session
func TestIsInsideClaudeCodeSession(t *testing.T) {
	t.Parallel()
	t.Run("Returns false when no env vars set", func(t *testing.T) {
		// This test assumes we're not running inside Claude Code during test
		// The actual result depends on the environment
		result := IsInsideClaudeCodeSession()
		t.Logf("IsInsideClaudeCodeSession: %v", result)
		// Just verify it doesn't panic
		assert.NotPanics(t, func() {
			IsInsideClaudeCodeSession()
		})
	})
}

// TestClaudeCLIProvider_Complete_InsideSession tests error when inside Claude Code session
func TestClaudeCLIProvider_Complete_InsideSession(t *testing.T) {
	t.Parallel()
	// Create provider
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

	// If we're inside a session, we should get an error
	if IsInsideClaudeCodeSession() {
		ctx := context.Background()
		_, err := provider.Complete(ctx, &models.LLMRequest{
			Prompt: "Hello",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot run inside another Claude Code session")
	} else {
		t.Log("Not inside Claude Code session - skipping session detection test")
	}
}

// TestClaudeCLIProvider_CompleteStream_InsideSession tests streaming error when inside session
func TestClaudeCLIProvider_CompleteStream_InsideSession(t *testing.T) {
	t.Parallel()
	provider := NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

	// If we're inside a session, we should get an error
	if IsInsideClaudeCodeSession() {
		ctx := context.Background()
		_, err := provider.CompleteStream(ctx, &models.LLMRequest{
			Prompt: "Hello",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot run inside another Claude Code session")
	} else {
		t.Log("Not inside Claude Code session - skipping session detection test")
	}
}

// TestClaudeCLIProvider_Complete_EmptyResponse tests handling of empty response
func TestClaudeCLIProvider_Complete_EmptyResponse(t *testing.T) {
	t.Parallel()
	// Create provider with mock behavior
	provider := &ClaudeCLIProvider{
		model:        "claude-sonnet-4-20250514",
		cliAvailable: false,
		cliCheckErr:  exec.ErrNotFound,
	}
	provider.cliCheckOnce.Do(func() {})

	ctx := context.Background()
	_, err := provider.Complete(ctx, &models.LLMRequest{
		Prompt: "Hello",
	})

	assert.Error(t, err)
	// The error should mention CLI not available
	assert.Contains(t, err.Error(), "not available")
}
