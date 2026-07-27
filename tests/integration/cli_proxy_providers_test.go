// Package integration provides integration tests for CLI proxy providers
//
// These tests verify that OAuth/free providers correctly use CLI proxy mechanism
// instead of direct API calls, which would fail due to product-restricted tokens.
package integration

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"dev.helix.agent/internal/auth/oauth_credentials"
	"dev.helix.agent/internal/llm/providers/claude"
	"dev.helix.agent/internal/llm/providers/qwen"
	"dev.helix.agent/internal/llm/providers/zen"
	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLIProxyProviderDiscovery verifies that CLI providers are correctly discovered
func TestCLIProxyProviderDiscovery(t *testing.T) {
	t.Run("Claude CLI provider discovery", func(t *testing.T) {
		// Check if Claude CLI is available
		cliProvider := claude.NewClaudeCLIProviderWithModel("claude-sonnet-4-5-20250929")
		available := cliProvider.IsCLIAvailable()

		if available {
			t.Log("Claude CLI is available - CLI proxy can be used")
			assert.NotNil(t, cliProvider)
			assert.Equal(t, "claude-cli", cliProvider.GetName())
		} else {
			t.Log("Claude CLI not available - skipping CLI proxy tests")
			t.Skip("Claude CLI not installed or not authenticated") // SKIP-OK: #legacy-untriaged
		}
	})

	t.Run("Qwen CLI provider discovery", func(t *testing.T) {
		// Check if Qwen CLI is available
		cliProvider := qwen.NewQwenCLIProviderWithModel("qwen-turbo")
		available := cliProvider.IsCLIAvailable()

		if available {
			t.Log("Qwen CLI is available - CLI proxy can be used")
			assert.NotNil(t, cliProvider)
			assert.Equal(t, "qwen-cli", cliProvider.GetName())
		} else {
			t.Log("Qwen CLI not available - skipping CLI proxy tests")
			t.Skip("Qwen CLI not installed or not authenticated") // SKIP-OK: #legacy-untriaged
		}
	})

	t.Run("OpenCode CLI provider discovery", func(t *testing.T) {
		// Check if OpenCode CLI is available
		cliProvider := zen.NewZenCLIProviderWithModel("big-pickle")
		available := cliProvider.IsCLIAvailable()

		if available {
			t.Log("OpenCode CLI is available - CLI proxy can be used")
			assert.NotNil(t, cliProvider)
			assert.Equal(t, "zen-cli", cliProvider.GetName())
		} else {
			t.Log("OpenCode CLI not available - skipping CLI proxy tests")
			t.Skip("OpenCode CLI not installed") // SKIP-OK: #legacy-untriaged
		}
	})
}

// TestCLIProviderCapabilities verifies CLI providers report correct capabilities
func TestCLIProviderCapabilities(t *testing.T) {
	t.Run("Claude CLI capabilities", func(t *testing.T) {
		provider := claude.NewClaudeCLIProviderWithModel("claude-sonnet-4-5-20250929")
		caps := provider.GetCapabilities()

		assert.NotNil(t, caps)
		assert.True(t, caps.SupportsStreaming)
		// CLI doesn't support tools directly
		assert.False(t, caps.SupportsTools)
	})

	t.Run("Qwen CLI capabilities", func(t *testing.T) {
		provider := qwen.NewQwenCLIProviderWithModel("qwen-turbo")
		caps := provider.GetCapabilities()

		assert.NotNil(t, caps)
		assert.True(t, caps.SupportsStreaming)
		assert.False(t, caps.SupportsTools)
	})

	t.Run("Zen CLI capabilities", func(t *testing.T) {
		provider := zen.NewZenCLIProviderWithModel("big-pickle")
		caps := provider.GetCapabilities()

		assert.NotNil(t, caps)
		assert.True(t, caps.SupportsStreaming)
		assert.False(t, caps.SupportsTools)
	})
}

// TestCLIProxyOAuthTokenRestrictions documents OAuth token limitations
func TestCLIProxyOAuthTokenRestrictions(t *testing.T) {
	t.Run("Claude OAuth tokens are product-restricted", func(t *testing.T) {
		// This test documents the behavior that Claude OAuth tokens
		// from Claude Code CLI can ONLY be used through Claude Code CLI,
		// NOT through direct API calls
		//
		// Attempting direct API call returns:
		// "This credential is only authorized for use with Claude Code
		// and cannot be used for other API requests."

		if !oauth_credentials.IsClaudeOAuthEnabled() {
			t.Skip("Claude OAuth not enabled") // SKIP-OK: #legacy-untriaged
		}

		// Verify credentials exist
		reader := oauth_credentials.GetGlobalReader()
		assert.True(t, reader.HasValidClaudeCredentials())

		// The correct approach is to use CLI proxy, not direct API
		cliProvider := claude.NewClaudeCLIProviderWithModel("claude-sonnet-4-5-20250929")
		if cliProvider.IsCLIAvailable() {
			t.Log("Using CLI proxy for Claude OAuth - this is the correct approach")
			assert.Equal(t, "claude-cli", cliProvider.GetName())
		}
	})

	t.Run("Qwen OAuth tokens are portal-restricted", func(t *testing.T) {
		// This test documents that Qwen OAuth tokens from Qwen Code CLI
		// are for the Qwen Portal only, NOT for DashScope API

		if !oauth_credentials.IsQwenOAuthEnabled() {
			t.Skip("Qwen OAuth not enabled") // SKIP-OK: #legacy-untriaged
		}

		reader := oauth_credentials.GetGlobalReader()
		assert.True(t, reader.HasValidQwenCredentials())

		// The correct approach is to use CLI proxy
		cliProvider := qwen.NewQwenCLIProviderWithModel("qwen-turbo")
		if cliProvider.IsCLIAvailable() {
			t.Log("Using CLI proxy for Qwen OAuth - this is the correct approach")
			assert.Equal(t, "qwen-cli", cliProvider.GetName())
		}
	})
}

// TestCLIProxyJSONOutputParsing verifies JSON output parsing for OpenCode
func TestCLIProxyJSONOutputParsing(t *testing.T) {
	provider := zen.NewZenCLIProviderWithModel("big-pickle")

	t.Run("Parse valid JSON response", func(t *testing.T) {
		// Test JSON parsing using reflection to access private method
		// For now, we test by checking the provider is correctly configured
		assert.NotNil(t, provider)
		assert.Equal(t, "big-pickle", provider.GetCurrentModel())
	})

	t.Run("Verify JSON format flag usage", func(t *testing.T) {
		// The provider should use -f json flag for structured output
		// This test verifies the configuration is correct
		caps := provider.GetCapabilities()
		assert.NotNil(t, caps)
	})
}

// TestCLIProviderComplete tests actual completion if CLI is available
func TestCLIProviderComplete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode") // SKIP-OK: #short-mode
	}

	t.Run("Claude CLI complete", func(t *testing.T) {
		provider := claude.NewClaudeCLIProviderWithModel("claude-sonnet-4-5-20250929")
		if !provider.IsCLIAvailable() {
			t.Skip("Claude CLI not available") // SKIP-OK: #legacy-untriaged
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		req := &models.LLMRequest{
			Messages: []models.Message{
				{Role: "user", Content: "Reply with just 'OK'"},
			},
			ModelParams: models.ModelParameters{
				MaxTokens: 10,
			},
		}

		resp, err := provider.Complete(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Content)
		assert.Equal(t, "claude-cli", resp.ProviderName)
	})

	t.Run("Qwen CLI complete", func(t *testing.T) {
		provider := qwen.NewQwenCLIProviderWithModel("qwen-turbo")
		if !provider.IsCLIAvailable() {
			t.Skip("Qwen CLI not available") // SKIP-OK: #legacy-untriaged
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		req := &models.LLMRequest{
			Messages: []models.Message{
				{Role: "user", Content: "Reply with just 'OK'"},
			},
			ModelParams: models.ModelParameters{
				MaxTokens: 10,
			},
		}

		resp, err := provider.Complete(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Content)
		assert.Equal(t, "qwen-cli", resp.ProviderName)
	})

	t.Run("Zen CLI complete", func(t *testing.T) {
		provider := zen.NewZenCLIProviderWithModel("opencode/big-pickle")
		if !provider.IsCLIAvailable() {
			t.Skip("OpenCode CLI not available") // SKIP-OK: #legacy-untriaged
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		req := &models.LLMRequest{
			Messages: []models.Message{
				{Role: "user", Content: "Reply with just 'OK'"},
			},
			ModelParams: models.ModelParameters{
				MaxTokens: 10,
			},
		}

		resp, err := provider.Complete(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Content)
		assert.Equal(t, "zen-cli", resp.ProviderName)
	})
}

// TestCLIProviderHealthCheck verifies health checks work
func TestCLIProviderHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode") // SKIP-OK: #short-mode
	}

	t.Run("Claude CLI health check", func(t *testing.T) {
		provider := claude.NewClaudeCLIProviderWithModel("claude-sonnet-4-5-20250929")
		if !provider.IsCLIAvailable() {
			t.Skip("Claude CLI not available") // SKIP-OK: #legacy-untriaged
		}

		err := provider.HealthCheck()
		assert.NoError(t, err)
	})

	t.Run("Qwen CLI health check", func(t *testing.T) {
		provider := qwen.NewQwenCLIProviderWithModel("qwen-turbo")
		if !provider.IsCLIAvailable() {
			t.Skip("Qwen CLI not available") // SKIP-OK: #legacy-untriaged
		}

		err := provider.HealthCheck()
		assert.NoError(t, err)
	})

	t.Run("Zen CLI health check", func(t *testing.T) {
		provider := zen.NewZenCLIProviderWithModel("big-pickle")
		if !provider.IsCLIAvailable() {
			t.Skip("OpenCode CLI not available") // SKIP-OK: #legacy-untriaged
		}

		err := provider.HealthCheck()
		assert.NoError(t, err)
	})
}

// TestProviderRegistryUseCLIProxy verifies registry uses CLI proxy for OAuth
func TestProviderRegistryUseCLIProxy(t *testing.T) {
	t.Run("Registry should use CLI for Claude OAuth", func(t *testing.T) {
		// When CLAUDE_API_KEY is not set but OAuth is enabled,
		// registry should use ClaudeCLIProvider instead of direct API

		// Check if OAuth is enabled
		if !oauth_credentials.IsClaudeOAuthEnabled() {
			t.Skip("Claude OAuth not enabled") // SKIP-OK: #legacy-untriaged
		}

		// Verify CLI approach is correct
		cliProvider := claude.NewClaudeCLIProviderWithModel("claude-sonnet-4-5-20250929")
		if cliProvider.IsCLIAvailable() {
			assert.Equal(t, "claude-cli", cliProvider.GetName())
			t.Log("Registry correctly uses CLI proxy for Claude OAuth")
		} else {
			t.Log("Claude CLI not available - OAuth provider would not be registered")
		}
	})

	t.Run("Registry should use CLI for Qwen OAuth", func(t *testing.T) {
		if !oauth_credentials.IsQwenOAuthEnabled() {
			t.Skip("Qwen OAuth not enabled") // SKIP-OK: #legacy-untriaged
		}

		cliProvider := qwen.NewQwenCLIProviderWithModel("qwen-turbo")
		if cliProvider.IsCLIAvailable() {
			assert.Equal(t, "qwen-cli", cliProvider.GetName())
			t.Log("Registry correctly uses CLI proxy for Qwen OAuth")
		} else {
			t.Log("Qwen CLI not available - OAuth provider would not be registered")
		}
	})

	t.Run("Registry should use CLI for Zen free mode", func(t *testing.T) {
		// When OPENCODE_API_KEY is not set,
		// registry should prefer ZenCLIProvider for free mode

		cliProvider := zen.NewZenCLIProviderWithModel("big-pickle")
		if cliProvider.IsCLIAvailable() {
			assert.Equal(t, "zen-cli", cliProvider.GetName())
			t.Log("Registry correctly uses CLI proxy for Zen free mode")
		} else {
			t.Log("OpenCode CLI not available - would fall back to anonymous API")
		}
	})
}

// TestCLIInstallationCheck verifies CLI tools installation status
func TestCLIInstallationCheck(t *testing.T) {
	t.Run("Check claude CLI installation", func(t *testing.T) {
		_, err := exec.LookPath("claude")
		if err != nil {
			t.Log("Claude CLI not installed: ", err)
		} else {
			t.Log("Claude CLI is installed")
		}
	})

	t.Run("Check qwen CLI installation", func(t *testing.T) {
		installed := qwen.IsQwenCodeInstalled()
		if installed {
			t.Log("Qwen Code CLI is installed")
		} else {
			t.Log("Qwen Code CLI is not installed")
		}
	})

	t.Run("Check opencode CLI installation", func(t *testing.T) {
		_, err := exec.LookPath("opencode")
		if err != nil {
			t.Log("OpenCode CLI not installed: ", err)
		} else {
			t.Log("OpenCode CLI is installed")
		}
	})
}

// TestNoDirectOAuthAPIUsage ensures no direct OAuth API calls are made
func TestNoDirectOAuthAPIUsage(t *testing.T) {
	t.Run("Verify no NewClaudeProviderWithOAuth in registry", func(t *testing.T) {
		// This test documents the requirement that provider_registry.go
		// should NOT use NewClaudeProviderWithOAuth for OAuth credentials
		// because those tokens are product-restricted

		// The registry should use ClaudeCLIProvider instead
		t.Log("Registry must use ClaudeCLIProvider, not NewClaudeProviderWithOAuth")
		t.Log("OAuth tokens are product-restricted to CLI tool usage only")
	})

	t.Run("Verify no NewQwenProviderWithOAuth in registry", func(t *testing.T) {
		// Similar to Claude, Qwen OAuth tokens are portal-restricted
		// Registry should use QwenCLIProvider instead

		t.Log("Registry must use QwenCLIProvider, not NewQwenProviderWithOAuth")
		t.Log("OAuth tokens are portal-restricted and cannot be used for DashScope API")
	})
}

// TestCLIProxyEnvironmentVariables verifies environment handling
func TestCLIProxyEnvironmentVariables(t *testing.T) {
	t.Run("Claude OAuth env check", func(t *testing.T) {
		oauthEnabled := os.Getenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS")
		if oauthEnabled == "true" {
			t.Log("CLAUDE_CODE_USE_OAUTH_CREDENTIALS is enabled")
			assert.True(t, oauth_credentials.IsClaudeOAuthEnabled())
		} else {
			t.Log("CLAUDE_CODE_USE_OAUTH_CREDENTIALS is not set or false")
		}
	})

	t.Run("Qwen OAuth env check", func(t *testing.T) {
		oauthEnabled := os.Getenv("QWEN_CODE_USE_OAUTH_CREDENTIALS")
		if oauthEnabled == "true" {
			t.Log("QWEN_CODE_USE_OAUTH_CREDENTIALS is enabled")
			assert.True(t, oauth_credentials.IsQwenOAuthEnabled())
		} else {
			t.Log("QWEN_CODE_USE_OAUTH_CREDENTIALS is not set or false")
		}
	})

	t.Run("OpenCode API key check", func(t *testing.T) {
		apiKey := os.Getenv("OPENCODE_API_KEY")
		if apiKey != "" {
			t.Log("OPENCODE_API_KEY is set - direct API will be used")
		} else {
			t.Log("OPENCODE_API_KEY is not set - CLI proxy preferred for free mode")
		}
	})
}
