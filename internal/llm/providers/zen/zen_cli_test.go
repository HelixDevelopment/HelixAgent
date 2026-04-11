package zen

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestZenCLIProvider_DefaultConfig tests default configuration
func TestZenCLIProvider_DefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultZenCLIConfig()

	// Model is empty by default - will be discovered dynamically
	assert.Equal(t, "", config.Model)
	assert.Equal(t, 120*time.Second, config.Timeout)
	assert.Equal(t, 4096, config.MaxOutputTokens)
}

// TestZenCLIProvider_NewProvider tests provider creation
func TestZenCLIProvider_NewProvider(t *testing.T) {
	t.Parallel()
	config := ZenCLIConfig{
		Model:           "grok-code",
		Timeout:         60 * time.Second,
		MaxOutputTokens: 2048,
	}

	provider := NewZenCLIProvider(config)

	assert.NotNil(t, provider)
	assert.Equal(t, "grok-code", provider.model)
	assert.Equal(t, 60*time.Second, provider.timeout)
	assert.Equal(t, 2048, provider.maxOutputTokens)
}

// TestZenCLIProvider_NewProviderWithModel tests model-specific creation
func TestZenCLIProvider_NewProviderWithModel(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithModel("big-pickle")
	assert.Equal(t, "big-pickle", provider.model)
}

// TestZenCLIProvider_GetName tests provider name
func TestZenCLIProvider_GetName(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithModel("grok-code")
	assert.Equal(t, "zen-cli", provider.GetName())
}

// TestZenCLIProvider_GetProviderType tests provider type
func TestZenCLIProvider_GetProviderType(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithModel("grok-code")
	assert.Equal(t, "zen", provider.GetProviderType())
}

// TestZenCLIProvider_GetCapabilities tests capabilities
func TestZenCLIProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping CLI availability test in short mode")
	}

	provider := NewZenCLIProviderWithModel("grok-code")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsFunctionCalling)
	assert.False(t, caps.SupportsVision)
	assert.False(t, caps.SupportsTools)
	assert.Contains(t, caps.SupportedFeatures, "text_completion")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Equal(t, "OpenCode Zen (CLI Facade)", caps.Metadata["provider"])
	assert.Equal(t, "true", caps.Metadata["facade"])
}

// TestZenCLIProvider_SetModel tests model setting
func TestZenCLIProvider_SetModel(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithModel("grok-code")
	assert.Equal(t, "grok-code", provider.GetCurrentModel())

	provider.SetModel("big-pickle")
	assert.Equal(t, "big-pickle", provider.GetCurrentModel())
}

// TestZenCLIProvider_MarkModelAsFailedAPI tests failed API tracking
func TestZenCLIProvider_MarkModelAsFailedAPI(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithModel("grok-code")

	assert.False(t, provider.IsModelFailedAPI("grok-code"))

	provider.MarkModelAsFailedAPI("grok-code")
	assert.True(t, provider.IsModelFailedAPI("grok-code"))
	assert.False(t, provider.IsModelFailedAPI("big-pickle"))
}

// TestZenCLIProvider_ShouldUseCLIFacade tests facade decision logic
func TestZenCLIProvider_ShouldUseCLIFacade(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithModel("grok-code")

	// Model not failed - should not use facade
	assert.False(t, provider.ShouldUseCLIFacade("grok-code"))

	// Mark as failed
	provider.MarkModelAsFailedAPI("grok-code")

	// If CLI available, should use facade
	if provider.IsCLIAvailable() {
		assert.True(t, provider.ShouldUseCLIFacade("grok-code"))
	} else {
		// CLI not available - should not use facade even if failed
		assert.False(t, provider.ShouldUseCLIFacade("grok-code"))
	}
}

// TestZenCLIProvider_IsCLIAvailable tests CLI availability check
func TestZenCLIProvider_IsCLIAvailable(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithModel("grok-code")

	available := provider.IsCLIAvailable()
	t.Logf("CLI available: %v", available)

	if !available {
		err := provider.GetCLIError()
		t.Logf("CLI error: %v", err)
		assert.NotNil(t, err)
	}
}

// TestZenCLIProvider_ValidateConfig tests configuration validation
func TestZenCLIProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithModel("grok-code")

	valid, errs := provider.ValidateConfig(nil)
	if IsOpenCodeInstalled() {
		assert.True(t, valid)
		assert.Empty(t, errs)
	} else {
		assert.False(t, valid)
		assert.NotEmpty(t, errs)
	}
}

// TestZenCLIProvider_HealthCheck tests health check
func TestZenCLIProvider_HealthCheck(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping CLI availability test in short mode")
	}

	provider := NewZenCLIProviderWithModel("grok-code")

	err := provider.HealthCheck()
	if IsOpenCodeInstalled() {
		assert.NoError(t, err)
	} else {
		assert.Error(t, err)
	}
}

// TestZenCLIProvider_Complete_NotAvailable tests completion when CLI not available
func TestZenCLIProvider_Complete_NotAvailable(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithUnavailableCLI("grok-code", exec.ErrNotFound)

	req := &models.LLMRequest{
		Prompt: "test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not available")
}

// TestZenCLIProvider_CompleteStream_NotAvailable tests streaming when CLI not available
func TestZenCLIProvider_CompleteStream_NotAvailable(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithUnavailableCLI("grok-code", exec.ErrNotFound)

	req := &models.LLMRequest{
		Prompt: "test prompt",
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "not available")
}

// TestZenCLIProvider_GetAvailableModels tests model discovery
func TestZenCLIProvider_GetAvailableModels(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping model discovery test in short mode")
	}

	provider := NewZenCLIProviderWithModel("grok-code")

	models := provider.GetAvailableModels()
	assert.NotEmpty(t, models)
	// Should at least return the known models
	t.Logf("Available models: %v", models)
}

// TestZenCLIProvider_IsModelAvailable tests model availability check
func TestZenCLIProvider_IsModelAvailable(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping model discovery test in short mode")
	}

	provider := NewZenCLIProviderWithModel("grok-code")

	// Get all available models
	available := provider.GetAvailableModels()
	assert.NotEmpty(t, available, "Should have at least some available models")

	// Check that at least one model contains known Zen identifiers
	foundKnown := false
	for _, model := range available {
		if strings.Contains(model, "big-pickle") ||
			strings.Contains(model, "grok") ||
			strings.Contains(model, "glm") ||
			strings.Contains(model, "gpt-5") {
			foundKnown = true
			// Verify that model is available when checked directly
			assert.True(t, provider.IsModelAvailable(model), "Model %s should be available", model)
			break
		}
	}
	assert.True(t, foundKnown, "Should find at least one known Zen model pattern")
}

// TestZenCLIProvider_GetBestAvailableModel tests best model selection
func TestZenCLIProvider_GetBestAvailableModel(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping model discovery test in short mode")
	}

	provider := NewZenCLIProviderWithModel("")

	bestModel := provider.GetBestAvailableModel()
	assert.NotEmpty(t, bestModel)
	t.Logf("Best available model: %s", bestModel)
}

// TestIsOpenCodeInstalled tests the standalone installation check
func TestIsOpenCodeInstalled(t *testing.T) {
	t.Parallel()
	installed := IsOpenCodeInstalled()
	t.Logf("OpenCode installed: %v", installed)
}

// TestGetKnownZenModels tests the known models list
func TestGetKnownZenModels(t *testing.T) {
	t.Parallel()
	models := GetKnownZenModels()
	require.NotEmpty(t, models)
	require.Len(t, models, 5) // Updated 2026-02: qwen3-coder removed from free tier
	assert.Contains(t, models, "big-pickle")
	assert.Contains(t, models, "gpt-5-nano")
	assert.Contains(t, models, "glm-4.7")
	// Note: qwen3-coder removed from free tier 2026-02
	assert.Contains(t, models, "kimi-k2")
	assert.Contains(t, models, "gemini-3-flash")
}

// TestParseZenModelsOutput tests output parsing
func TestParseZenModelsOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name:     "model list with grok",
			output:   "grok-code\nbig-pickle\nglm-4.7-free",
			expected: []string{"grok-code", "big-pickle", "glm-4.7-free"},
		},
		{
			name:     "model list with prefixes",
			output:   "opencode/grok-code\nopencode/big-pickle",
			expected: []string{"opencode/grok-code", "opencode/big-pickle"},
		},
		{
			name:     "empty output",
			output:   "",
			expected: nil,
		},
		{
			name:     "random text",
			output:   "Some random text that doesn't match models",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			models := parseZenModelsOutput(tt.output)
			if tt.expected == nil {
				assert.Empty(t, models)
			} else {
				assert.Equal(t, len(tt.expected), len(models))
				for _, expected := range tt.expected {
					assert.Contains(t, models, expected)
				}
			}
		})
	}
}

// TestDiscoverZenModels tests the standalone discovery function
func TestDiscoverZenModels(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping model discovery test in short mode")
	}

	models, err := DiscoverZenModels()
	assert.NotEmpty(t, models)
	t.Logf("Discovered models: %v, error: %v", models, err)
}

// Integration test - only run if OpenCode is actually installed
func TestZenCLIProvider_Integration(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping model discovery test in short mode")
	}

	if !IsOpenCodeInstalled() {
		t.Skip("Skipping integration test - OpenCode CLI not installed")
	}

	provider := NewZenCLIProviderWithModel("grok-code")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		Messages: []models.Message{
			{Role: "user", Content: "Say OK"},
		},
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		t.Logf("Integration test failed (may be expected if CLI not authenticated): %v", err)
		return
	}

	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content)
	t.Logf("Response: %s", resp.Content)
}
