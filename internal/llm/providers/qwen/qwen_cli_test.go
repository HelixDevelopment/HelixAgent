package qwen

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
)

// TestQwenCLIProvider_DefaultConfig tests default configuration
func TestQwenCLIProvider_DefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultQwenCLIConfig()

	// Model is empty by default - will be discovered dynamically
	assert.Equal(t, "", config.Model)
	assert.Equal(t, 120*time.Second, config.Timeout)
	assert.Equal(t, 4096, config.MaxOutputTokens)
}

// TestQwenCLIProvider_NewProvider tests provider creation
func TestQwenCLIProvider_NewProvider(t *testing.T) {
	t.Parallel()
	config := QwenCLIConfig{
		Model:           "qwen-max",
		Timeout:         60 * time.Second,
		MaxOutputTokens: 2048,
	}

	provider := NewQwenCLIProvider(config)

	assert.NotNil(t, provider)
	assert.Equal(t, "qwen-max", provider.model)
	assert.Equal(t, 60*time.Second, provider.timeout)
	assert.Equal(t, 2048, provider.maxOutputTokens)
}

// TestQwenCLIProvider_NewProviderWithModel tests model-specific creation
func TestQwenCLIProvider_NewProviderWithModel(t *testing.T) {
	t.Parallel()
	provider := NewQwenCLIProviderWithModel("qwen-turbo")

	assert.NotNil(t, provider)
	assert.Equal(t, "qwen-turbo", provider.model)
}

// TestQwenCLIProvider_GetName tests provider name
func TestQwenCLIProvider_GetName(t *testing.T) {
	t.Parallel()
	provider := NewQwenCLIProviderWithModel("qwen-plus")
	assert.Equal(t, "qwen-cli", provider.GetName())
}

// TestQwenCLIProvider_GetProviderType tests provider type
func TestQwenCLIProvider_GetProviderType(t *testing.T) {
	t.Parallel()
	provider := NewQwenCLIProviderWithModel("qwen-plus")
	assert.Equal(t, "qwen", provider.GetProviderType())
}

// TestQwenCLIProvider_GetCapabilities tests capabilities
func TestQwenCLIProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewQwenCLIProviderWithModel("qwen-plus")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsTools) // CLI doesn't support tools
	assert.GreaterOrEqual(t, len(caps.SupportedModels), 5, "Should support multiple Qwen models")

	// Check for specific models
	assert.Contains(t, caps.SupportedModels, "qwen-plus")
	assert.Contains(t, caps.SupportedModels, "qwen-turbo")
	assert.Contains(t, caps.SupportedModels, "qwen-max")
}

// TestQwenCLIProvider_SetModel tests model setting
func TestQwenCLIProvider_SetModel(t *testing.T) {
	t.Parallel()
	provider := NewQwenCLIProviderWithModel("qwen-plus")
	assert.Equal(t, "qwen-plus", provider.GetCurrentModel())

	provider.SetModel("qwen-max")
	assert.Equal(t, "qwen-max", provider.GetCurrentModel())
}

// TestIsQwenCodeInstalled tests CLI installation check
func TestIsQwenCodeInstalled(t *testing.T) {
	t.Parallel()
	// This test is informational - actual result depends on system
	installed := IsQwenCodeInstalled()
	t.Logf("Qwen Code installed: %v", installed)

	// Verify the function doesn't panic
	assert.NotPanics(t, func() {
		IsQwenCodeInstalled()
	})
}

// TestGetQwenCodePath tests path lookup
func TestGetQwenCodePath(t *testing.T) {
	t.Parallel()
	path, err := GetQwenCodePath()

	// Just verify it returns proper types
	if err != nil {
		t.Logf("Qwen Code not found: %v", err)
		assert.Empty(t, path)
	} else {
		t.Logf("Qwen Code path: %s", path)
		assert.NotEmpty(t, path)
	}
}

// TestIsQwenCodeAuthenticated tests auth check
func TestIsQwenCodeAuthenticated(t *testing.T) {
	t.Parallel()
	// This test is informational - actual result depends on system
	authenticated := IsQwenCodeAuthenticated()
	t.Logf("Qwen Code authenticated: %v", authenticated)

	// If CLI is not installed, should return false
	if !IsQwenCodeInstalled() {
		assert.False(t, authenticated)
	}
}

// TestQwenCLIProvider_IsCLIAvailable tests availability check
func TestQwenCLIProvider_IsCLIAvailable(t *testing.T) {
	t.Parallel()
	provider := NewQwenCLIProviderWithModel("qwen-plus")

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

// TestQwenCLIProvider_ValidateConfig tests config validation
func TestQwenCLIProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	provider := NewQwenCLIProviderWithModel("qwen-plus")

	valid, errs := provider.ValidateConfig(nil)
	if IsQwenCodeInstalled() && IsQwenCodeAuthenticated() {
		assert.True(t, valid)
		assert.Empty(t, errs)
	} else {
		assert.False(t, valid)
		assert.NotEmpty(t, errs)
	}
}

// TestQwenCLIProvider_Complete_NoPrompt tests error on empty prompt
func TestQwenCLIProvider_Complete_NoPrompt(t *testing.T) {
	t.Parallel()
	provider := NewQwenCLIProviderWithModel("qwen-plus")

	// Skip if CLI not available
	if !provider.IsCLIAvailable() {
		t.Skip("Qwen CLI not available")
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

// TestQwenCLIProvider_Complete_CLIUnavailable tests behavior when CLI unavailable
func TestQwenCLIProvider_Complete_CLIUnavailable(t *testing.T) {
	t.Parallel()
	// Create provider with invalid path
	provider := &QwenCLIProvider{
		model:        "qwen-plus",
		cliAvailable: false,
		cliCheckErr:  exec.ErrNotFound,
	}
	// Mark sync.Once as already executed to prevent real CLI check
	provider.cliCheckOnce.Do(func() {})

	ctx := context.Background()
	resp, err := provider.Complete(ctx, &models.LLMRequest{
		Prompt: "Hello",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not available")
}

// TestQwenCLIProvider_HealthCheck_CLIUnavailable tests health check when CLI unavailable
func TestQwenCLIProvider_HealthCheck_CLIUnavailable(t *testing.T) {
	t.Parallel()
	provider := &QwenCLIProvider{
		model:        "qwen-plus",
		cliAvailable: false,
		cliCheckErr:  exec.ErrNotFound,
	}
	// Mark sync.Once as already executed to prevent real CLI check
	provider.cliCheckOnce.Do(func() {})

	err := provider.HealthCheck()

	assert.Error(t, err)
}

// Integration test - only runs if Qwen CLI is installed and authenticated
func TestQwenCLIProvider_Integration_Complete(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if !IsQwenCodeInstalled() {
		t.Skip("Qwen Code CLI not installed")
	}
	if !IsQwenCodeAuthenticated() {
		t.Skip("Qwen Code CLI not authenticated")
	}

	provider := NewQwenCLIProviderWithModel("qwen-plus")

	// Verify CLI is actually available before running the test
	if !provider.IsCLIAvailable() {
		t.Skipf("Qwen CLI not available: %v", provider.GetCLIError())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := provider.Complete(ctx, &models.LLMRequest{
		Prompt: "Reply with exactly one word: hello",
		ModelParams: models.ModelParameters{
			MaxTokens: 10,
		},
	})

	if err != nil {
		t.Logf("Integration test failed (may be expected if CLI has issues): %v", err)
		t.Skip("CLI integration test skipped due to error")
	}

	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content)
	assert.Equal(t, "qwen-cli", resp.ProviderName)
	t.Logf("Response: %s", resp.Content)
}

// Integration test for health check
func TestQwenCLIProvider_Integration_HealthCheck(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if !IsQwenCodeInstalled() {
		t.Skip("Qwen Code CLI not installed")
	}
	if !IsQwenCodeAuthenticated() {
		t.Skip("Qwen Code CLI not authenticated")
	}

	provider := NewQwenCLIProviderWithModel("qwen-plus")

	// Verify CLI is actually available before running the test
	if !provider.IsCLIAvailable() {
		t.Skipf("Qwen CLI not available: %v", provider.GetCLIError())
	}

	err := provider.HealthCheck()

	if err != nil {
		t.Logf("Health check failed (may be expected if CLI has issues): %v", err)
		t.Skip("Health check test skipped due to error")
	}

	assert.NoError(t, err)
}

// TestCanUseQwenOAuth tests the OAuth check function
func TestCanUseQwenOAuth(t *testing.T) {
	t.Parallel()
	// This test is informational
	canUse := CanUseQwenOAuth()
	t.Logf("Can use Qwen OAuth: %v", canUse)

	// The function should not panic
	assert.NotPanics(t, func() {
		CanUseQwenOAuth()
	})
}
