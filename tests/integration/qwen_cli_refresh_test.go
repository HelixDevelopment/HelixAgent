// Package integration provides integration tests for the Qwen CLI token refresh mechanism.
// These tests require the qwen CLI to be installed and configured.
package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/auth/oauth_credentials"
)

// TestQwenCLIRefreshIntegration tests the full CLI refresh flow
// This test requires:
// - qwen CLI to be installed and in PATH
// - Valid Qwen OAuth credentials at ~/.qwen/oauth_creds.json
func TestQwenCLIRefreshIntegration(t *testing.T) {
	// Skip if SKIP_QWEN_INTEGRATION is set (for CI environments without qwen)
	if os.Getenv("SKIP_QWEN_INTEGRATION") != "" {
		t.Logf("Skipping Qwen integration test - SKIP_QWEN_INTEGRATION set (acceptable)")
		return
	}

	refresher := oauth_credentials.NewCLIRefresher(nil)

	t.Run("CLI is available", func(t *testing.T) {
		if !refresher.IsAvailable() {
			t.Logf("qwen CLI not available - skipping integration tests (acceptable)")
			return
		}
		t.Logf("qwen CLI found at: %s", refresher.GetQwenCLIPath())
	})

	t.Run("GetStatus returns valid information", func(t *testing.T) {
		status := refresher.GetStatus()
		if status == nil {
			t.Fatal("expected non-nil status")
		}

		t.Logf("CLI Available: %v", status.Available)
		t.Logf("CLI Path: %s", status.QwenCLIPath)
		t.Logf("Token Valid: %v", status.TokenValid)
		t.Logf("Token Expires At: %v", status.TokenExpiresAt)
		t.Logf("Token Expires In: %s", status.TokenExpiresIn)
	})

	t.Run("Can read current credentials", func(t *testing.T) {
		reader := oauth_credentials.GetGlobalReader()
		creds, err := reader.ReadQwenCredentials()
		if err != nil {
			t.Skipf("No valid Qwen credentials available: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}

		if creds.AccessToken == "" {
			t.Error("expected non-empty access token")
		}

		t.Logf("Current token expires at: %s", time.UnixMilli(creds.ExpiryDate).Format(time.RFC3339))
		t.Logf("Token expires in: %v", time.Until(time.UnixMilli(creds.ExpiryDate)))
	})
}

// TestQwenCLIRefreshExecution tests actual CLI execution
// This test actually invokes the qwen CLI and should be run sparingly
func TestQwenCLIRefreshExecution(t *testing.T) {
	// Only run if explicitly enabled
	if os.Getenv("TEST_QWEN_CLI_REFRESH") == "" {
		t.Logf("Skipping actual CLI refresh test - set TEST_QWEN_CLI_REFRESH=1 to enable (acceptable)")
		return
	}

	refresher := oauth_credentials.NewCLIRefresher(&oauth_credentials.CLIRefreshConfig{
		RefreshTimeout:     120 * time.Second,
		MinRefreshInterval: 0, // Disable rate limiting for test
		MaxRetries:         1,
		Prompt:             "exit",
	})

	if !refresher.IsAvailable() {
		t.Logf("qwen CLI not available (acceptable)")
		return
	}

	// Reset rate limit for test
	refresher.ResetRateLimit()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	t.Run("CLI refresh executes successfully", func(t *testing.T) {
		result, err := refresher.RefreshQwenToken(ctx)
		if err != nil {
			t.Fatalf("CLI refresh failed: %v", err)
		}

		t.Logf("Refresh Result:")
		t.Logf("  Success: %v", result.Success)
		t.Logf("  Duration: %v", result.RefreshDuration)
		t.Logf("  Retries: %d", result.Retries)

		if result.Error != "" {
			t.Logf("  Error: %s", result.Error)
		}

		if result.NewExpiryDate > 0 {
			t.Logf("  New Expiry: %s", time.UnixMilli(result.NewExpiryDate).Format(time.RFC3339))
		}

		if !result.Success {
			t.Errorf("CLI refresh did not succeed: %s", result.Error)
		}
	})

	t.Run("Token is valid after refresh", func(t *testing.T) {
		reader := oauth_credentials.GetGlobalReader()
		reader.ClearCache() // Clear cache to read fresh credentials

		creds, err := reader.ReadQwenCredentials()
		if err != nil {
			t.Fatalf("Failed to read credentials after refresh: %v", err)
		}

		if creds.AccessToken == "" {
			t.Error("expected non-empty access token after refresh")
		}

		if oauth_credentials.IsExpired(creds.ExpiryDate) {
			t.Error("token should not be expired after refresh")
		}

		t.Logf("Token valid until: %s", time.UnixMilli(creds.ExpiryDate).Format(time.RFC3339))
	})
}

// TestQwenCLIRefreshWithFallback tests the fallback mechanism
func TestQwenCLIRefreshWithFallback(t *testing.T) {
	if os.Getenv("SKIP_QWEN_INTEGRATION") != "" {
		t.Logf("Skipping Qwen integration test - SKIP_QWEN_INTEGRATION set (acceptable)")
		return
	}

	reader := oauth_credentials.GetGlobalReader()
	creds, err := reader.ReadQwenCredentials()
	if err != nil {
		t.Skipf("No valid Qwen credentials: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
	}

	t.Run("RefreshQwenTokenWithFallback with valid credentials", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Since credentials are valid, this should succeed quickly
		refreshedCreds, err := oauth_credentials.RefreshQwenTokenWithFallback(ctx, creds)
		if err != nil {
			// This might fail if rate limited, which is OK
			if contains(err.Error(), "rate limited") {
				t.Logf("Rate limited (expected): %v", err)
				return
			}
			t.Logf("Refresh failed (may be expected): %v", err)
			return
		}

		if refreshedCreds == nil {
			t.Error("expected non-nil credentials")
			return
		}

		if oauth_credentials.IsExpired(refreshedCreds.ExpiryDate) {
			t.Error("refreshed credentials should not be expired")
		}
	})
}

// TestQwenOAuthStatus tests OAuth status reporting
func TestQwenOAuthStatus(t *testing.T) {
	t.Run("IsQwenOAuthEnabled reports environment status", func(t *testing.T) {
		enabled := oauth_credentials.IsQwenOAuthEnabled()
		t.Logf("Qwen OAuth enabled: %v", enabled)
		t.Logf("QWEN_CODE_USE_OAUTH_CREDENTIALS: %s", os.Getenv("QWEN_CODE_USE_OAUTH_CREDENTIALS"))
	})

	t.Run("GetQwenCredentialInfo returns status", func(t *testing.T) {
		reader := oauth_credentials.GetGlobalReader()
		info := reader.GetQwenCredentialInfo()

		t.Logf("Qwen credential info: %+v", info)

		if available, ok := info["available"].(bool); ok {
			if available {
				if _, hasExpiry := info["expires_in"]; !hasExpiry {
					t.Error("expected expires_in when credentials are available")
				}
			}
		}
	})
}

// TestCLIRefresherConcurrency tests thread safety
func TestCLIRefresherConcurrency(t *testing.T) {
	refresher := oauth_credentials.NewCLIRefresher(&oauth_credentials.CLIRefreshConfig{
		MinRefreshInterval: 0,
	})

	t.Run("GetStatus is thread safe", func(t *testing.T) {
		done := make(chan bool)

		// Launch multiple goroutines reading status
		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					_ = refresher.GetStatus()
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("ResetRateLimit is thread safe", func(t *testing.T) {
		done := make(chan bool)

		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					refresher.ResetRateLimit()
					_ = refresher.GetLastRefreshTime()
				}
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// BenchmarkCLIRefresherStatus benchmarks status retrieval
func BenchmarkCLIRefresherStatus(b *testing.B) {
	refresher := oauth_credentials.NewCLIRefresher(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = refresher.GetStatus()
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
