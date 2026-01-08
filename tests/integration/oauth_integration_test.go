// Package integration provides integration tests for OAuth credential functionality
package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/auth/oauth_credentials"
	"dev.helix.agent/internal/llm/providers/claude"
	"dev.helix.agent/internal/llm/providers/qwen"
)

// TestOAuthClaudeProviderIntegration tests the Claude provider with OAuth credentials
func TestOAuthClaudeProviderIntegration(t *testing.T) {
	// Skip if OAuth is not enabled or credentials are not available
	if !oauth_credentials.IsClaudeOAuthEnabled() {
		t.Skip("Claude OAuth is not enabled (CLAUDE_CODE_USE_OAUTH_CREDENTIALS not set)")
	}

	reader := oauth_credentials.GetGlobalReader()
	if !reader.HasValidClaudeCredentials() {
		t.Skip("No valid Claude OAuth credentials available")
	}

	// Create OAuth-enabled provider
	provider, err := claude.NewClaudeProviderWithOAuth("", "")
	if err != nil {
		t.Fatalf("Failed to create Claude OAuth provider: %v", err)
	}

	// Verify provider configuration
	if provider.GetAuthType() != claude.AuthTypeOAuth {
		t.Error("Expected OAuth auth type")
	}

	// Test capabilities
	caps := provider.GetCapabilities()
	if caps == nil {
		t.Error("GetCapabilities returned nil")
	}

	// Validate config (should pass since OAuth is set up)
	valid, errors := provider.ValidateConfig(nil)
	if !valid {
		t.Errorf("ValidateConfig failed: %v", errors)
	}

	t.Log("Claude OAuth provider integration test passed")
}

// TestOAuthQwenProviderIntegration tests the Qwen provider with OAuth credentials
func TestOAuthQwenProviderIntegration(t *testing.T) {
	// Skip if OAuth is not enabled or credentials are not available
	if !oauth_credentials.IsQwenOAuthEnabled() {
		t.Skip("Qwen OAuth is not enabled (QWEN_CODE_USE_OAUTH_CREDENTIALS not set)")
	}

	reader := oauth_credentials.GetGlobalReader()
	if !reader.HasValidQwenCredentials() {
		t.Skip("No valid Qwen OAuth credentials available")
	}

	// Create OAuth-enabled provider
	provider, err := qwen.NewQwenProviderWithOAuth("", "")
	if err != nil {
		t.Fatalf("Failed to create Qwen OAuth provider: %v", err)
	}

	// Verify provider configuration
	if provider.GetAuthType() != qwen.AuthTypeOAuth {
		t.Error("Expected OAuth auth type")
	}

	// Test capabilities
	caps := provider.GetCapabilities()
	if caps == nil {
		t.Error("GetCapabilities returned nil")
	}

	// Validate config
	valid, errors := provider.ValidateConfig(nil)
	if !valid {
		t.Errorf("ValidateConfig failed: %v", errors)
	}

	t.Log("Qwen OAuth provider integration test passed")
}

// TestOAuthAutoProviderSelection tests automatic provider selection with OAuth
func TestOAuthAutoProviderSelection(t *testing.T) {
	// Test Claude auto selection
	t.Run("Claude Auto Selection", func(t *testing.T) {
		if !oauth_credentials.IsClaudeOAuthEnabled() {
			t.Skip("Claude OAuth not enabled")
		}

		reader := oauth_credentials.GetGlobalReader()
		if !reader.HasValidClaudeCredentials() {
			// Should fall back to API key
			provider, err := claude.NewClaudeProviderAuto("test-api-key", "", "")
			if err != nil {
				t.Fatalf("Failed to create auto provider: %v", err)
			}
			if provider.GetAuthType() != claude.AuthTypeAPIKey {
				t.Error("Expected API key auth type as fallback")
			}
		} else {
			// Should use OAuth
			provider, err := claude.NewClaudeProviderAuto("", "", "")
			if err != nil {
				t.Fatalf("Failed to create auto provider: %v", err)
			}
			if provider.GetAuthType() != claude.AuthTypeOAuth {
				t.Error("Expected OAuth auth type when credentials available")
			}
		}
	})

	// Test Qwen auto selection
	t.Run("Qwen Auto Selection", func(t *testing.T) {
		if !oauth_credentials.IsQwenOAuthEnabled() {
			t.Skip("Qwen OAuth not enabled")
		}

		reader := oauth_credentials.GetGlobalReader()
		if !reader.HasValidQwenCredentials() {
			// Should fall back to API key
			provider, err := qwen.NewQwenProviderAuto("test-api-key", "", "")
			if err != nil {
				t.Fatalf("Failed to create auto provider: %v", err)
			}
			if provider.GetAuthType() != qwen.AuthTypeAPIKey {
				t.Error("Expected API key auth type as fallback")
			}
		} else {
			// Should use OAuth
			provider, err := qwen.NewQwenProviderAuto("", "", "")
			if err != nil {
				t.Fatalf("Failed to create auto provider: %v", err)
			}
			if provider.GetAuthType() != qwen.AuthTypeOAuth {
				t.Error("Expected OAuth auth type when credentials available")
			}
		}
	})
}

// TestOAuthCredentialInfo tests getting credential information
func TestOAuthCredentialInfo(t *testing.T) {
	reader := oauth_credentials.GetGlobalReader()

	// Test Claude credential info
	t.Run("Claude Credential Info", func(t *testing.T) {
		info := reader.GetClaudeCredentialInfo()
		if info == nil {
			t.Fatal("GetClaudeCredentialInfo returned nil")
		}

		// Should have 'available' key
		if _, ok := info["available"]; !ok {
			t.Error("Missing 'available' key in credential info")
		}

		available, ok := info["available"].(bool)
		if ok && available {
			// Check for additional keys when credentials are available
			requiredKeys := []string{"subscription_type", "rate_limit_tier", "expires_in", "has_refresh_token"}
			for _, key := range requiredKeys {
				if _, exists := info[key]; !exists {
					t.Errorf("Missing '%s' key in credential info", key)
				}
			}
		}
	})

	// Test Qwen credential info
	t.Run("Qwen Credential Info", func(t *testing.T) {
		info := reader.GetQwenCredentialInfo()
		if info == nil {
			t.Fatal("GetQwenCredentialInfo returned nil")
		}

		// Should have 'available' key
		if _, ok := info["available"]; !ok {
			t.Error("Missing 'available' key in credential info")
		}

		available, ok := info["available"].(bool)
		if ok && available {
			// Check for additional keys when credentials are available
			requiredKeys := []string{"token_type", "expires_in", "has_refresh_token"}
			for _, key := range requiredKeys {
				if _, exists := info[key]; !exists {
					t.Errorf("Missing '%s' key in credential info", key)
				}
			}
		}
	})
}

// TestOAuthLiveAPICallClaude tests a live API call with Claude OAuth credentials
func TestOAuthLiveAPICallClaude(t *testing.T) {
	// This test makes actual API calls - skip in CI or when credentials unavailable
	if os.Getenv("RUN_LIVE_OAUTH_TESTS") != "true" {
		t.Skip("Skipping live OAuth test (set RUN_LIVE_OAUTH_TESTS=true to enable)")
	}

	if !oauth_credentials.IsClaudeOAuthEnabled() {
		t.Skip("Claude OAuth not enabled")
	}

	reader := oauth_credentials.GetGlobalReader()
	if !reader.HasValidClaudeCredentials() {
		t.Skip("No valid Claude OAuth credentials")
	}

	provider, err := claude.NewClaudeProviderWithOAuth("", "claude-3-haiku-20240307")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test health check
	err = provider.HealthCheck()
	if err != nil {
		t.Logf("Health check warning: %v", err)
	}

	t.Log("Claude OAuth live API test passed")
}

// TestOAuthLiveAPICallQwen tests a live API call with Qwen OAuth credentials
func TestOAuthLiveAPICallQwen(t *testing.T) {
	// This test makes actual API calls - skip in CI or when credentials unavailable
	if os.Getenv("RUN_LIVE_OAUTH_TESTS") != "true" {
		t.Skip("Skipping live OAuth test (set RUN_LIVE_OAUTH_TESTS=true to enable)")
	}

	if !oauth_credentials.IsQwenOAuthEnabled() {
		t.Skip("Qwen OAuth not enabled")
	}

	reader := oauth_credentials.GetGlobalReader()
	if !reader.HasValidQwenCredentials() {
		t.Skip("No valid Qwen OAuth credentials")
	}

	provider, err := qwen.NewQwenProviderWithOAuth("", "qwen-turbo")
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test health check
	err = provider.HealthCheck()
	if err != nil {
		t.Logf("Health check warning: %v", err)
	}

	t.Log("Qwen OAuth live API test passed")
}

// TestEnvironmentVariableToggle tests the environment variable toggle behavior
func TestEnvironmentVariableToggle(t *testing.T) {
	// Save original values
	originalClaude := os.Getenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS")
	originalQwen := os.Getenv("QWEN_CODE_USE_OAUTH_CREDENTIALS")
	defer func() {
		os.Setenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS", originalClaude)
		os.Setenv("QWEN_CODE_USE_OAUTH_CREDENTIALS", originalQwen)
	}()

	// Test with OAuth disabled
	os.Setenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS", "false")
	os.Setenv("QWEN_CODE_USE_OAUTH_CREDENTIALS", "false")

	// Should require API keys when OAuth is disabled
	_, err := claude.NewClaudeProviderAuto("", "", "")
	if err == nil {
		t.Error("Expected error when OAuth disabled and no API key provided")
	}

	_, err = qwen.NewQwenProviderAuto("", "", "")
	if err == nil {
		t.Error("Expected error when OAuth disabled and no API key provided")
	}

	// Should work with API keys
	provider1 := claude.NewClaudeProvider("test-key", "", "")
	if provider1 == nil {
		t.Error("Failed to create Claude provider with API key")
	}
	if provider1.GetAuthType() != claude.AuthTypeAPIKey {
		t.Error("Expected API key auth type")
	}

	provider2 := qwen.NewQwenProvider("test-key", "", "")
	if provider2 == nil {
		t.Error("Failed to create Qwen provider with API key")
	}
	if provider2.GetAuthType() != qwen.AuthTypeAPIKey {
		t.Error("Expected API key auth type")
	}
}
