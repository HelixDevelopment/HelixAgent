// Package adapters provides provider adapters for LLMsVerifier integration
package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/auth/oauth_credentials"
	"dev.helix.agent/internal/verifier"
)

// =====================================================
// OAUTH ADAPTER COMPREHENSIVE TESTS
// =====================================================

func TestDefaultOAuthAdapterConfig_Comprehensive(t *testing.T) {
	t.Parallel()
	cfg := DefaultOAuthAdapterConfig()

	require.NotNil(t, cfg)
	assert.Equal(t, 10, cfg.RefreshThresholdMins)
	// IMPORTANT: Default is false - don't trust tokens that can't make API calls
	assert.False(t, cfg.TrustOnVerificationFailure)
	assert.Equal(t, 7.5, cfg.DefaultScoreOnFailure)
	assert.Equal(t, 0.5, cfg.OAuthPriorityBoost)
	assert.Equal(t, 30*time.Second, cfg.VerificationTimeout)
	assert.Equal(t, 60*time.Second, cfg.RefreshTimeout)
	assert.True(t, cfg.EnableCLIRefresh)
	assert.Equal(t, 3, cfg.CLIRefreshRetries)
}

func TestNewOAuthAdapter_Comprehensive(t *testing.T) {
	t.Parallel()
	t.Run("with nil verifier and logger", func(t *testing.T) {
		adapter := NewOAuthAdapter(nil, nil)

		require.NotNil(t, adapter)
		assert.NotNil(t, adapter.credReader)
		assert.NotNil(t, adapter.log)
		assert.NotNil(t, adapter.config)
		assert.Nil(t, adapter.verifierSvc)
	})

	t.Run("with provided verifier", func(t *testing.T) {
		t.Parallel()
		verifierSvc := verifier.NewVerificationService(&verifier.Config{})
		logger := logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		adapter := NewOAuthAdapter(verifierSvc, logger)

		require.NotNil(t, adapter)
		assert.Equal(t, verifierSvc, adapter.verifierSvc)
		assert.Equal(t, logger, adapter.log)
	})
}

func TestNewOAuthAdapterWithConfig_Comprehensive(t *testing.T) {
	t.Parallel()
	t.Run("with custom config", func(t *testing.T) {
		customConfig := &OAuthAdapterConfig{
			RefreshThresholdMins:       5,
			TrustOnVerificationFailure: false,
			DefaultScoreOnFailure:      8.0,
			OAuthPriorityBoost:         1.0,
			VerificationTimeout:        60 * time.Second,
			RefreshTimeout:             120 * time.Second,
			EnableCLIRefresh:           false,
			CLIRefreshRetries:          5,
		}

		adapter := NewOAuthAdapterWithConfig(nil, customConfig, nil)

		require.NotNil(t, adapter)
		assert.Equal(t, customConfig, adapter.config)
		assert.Equal(t, 5, adapter.config.RefreshThresholdMins)
		assert.False(t, adapter.config.TrustOnVerificationFailure)
		assert.Equal(t, 8.0, adapter.config.DefaultScoreOnFailure)
		assert.Equal(t, 1.0, adapter.config.OAuthPriorityBoost)
	})

	t.Run("with nil config uses default", func(t *testing.T) {
		t.Parallel()
		adapter := NewOAuthAdapterWithConfig(nil, nil, nil)

		require.NotNil(t, adapter)
		assert.NotNil(t, adapter.config)
		assert.Equal(t, DefaultOAuthAdapterConfig().RefreshThresholdMins, adapter.config.RefreshThresholdMins)
	})
}

func TestOAuthAdapter_GetClaudeToken(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)
	testExpiry := time.Now().Add(time.Hour)

	// Set token manually for testing
	adapter.mu.Lock()
	adapter.claudeToken = "test-claude-token"
	adapter.claudeExpiry = testExpiry
	adapter.mu.Unlock()

	token, expiry := adapter.GetClaudeToken()

	assert.Equal(t, "test-claude-token", token)
	assert.Equal(t, testExpiry, expiry)
}

func TestOAuthAdapter_GetQwenToken(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)
	testExpiry := time.Now().Add(time.Hour)

	// Set token manually for testing
	adapter.mu.Lock()
	adapter.qwenToken = "test-qwen-token"
	adapter.qwenExpiry = testExpiry
	adapter.mu.Unlock()

	token, expiry := adapter.GetQwenToken()

	assert.Equal(t, "test-qwen-token", token)
	assert.Equal(t, testExpiry, expiry)
}

func TestOAuthAdapter_IsClaudeTokenValid(t *testing.T) {
	t.Parallel()

	// Per-subtest adapter — the previous shared adapter pattern caused
	// "no token" to fail after "valid token" set claudeToken on the
	// same adapter (race-debt BUGFIX #26).
	newAdapter := func() *OAuthAdapter { return NewOAuthAdapter(nil, nil) }

	t.Run("no token", func(t *testing.T) {
		t.Parallel()
		adapter := newAdapter()
		valid := adapter.IsClaudeTokenValid()
		assert.False(t, valid)
	})

	t.Run("valid token", func(t *testing.T) {
		t.Parallel()
		adapter := newAdapter()
		adapter.mu.Lock()
		adapter.claudeToken = "test-token"
		adapter.claudeExpiry = time.Now().Add(time.Hour)
		adapter.mu.Unlock()

		valid := adapter.IsClaudeTokenValid()
		assert.True(t, valid)
	})

	t.Run("expired token", func(t *testing.T) {
		t.Parallel()
		adapter := newAdapter()
		adapter.mu.Lock()
		adapter.claudeToken = "test-token"
		adapter.claudeExpiry = time.Now().Add(-time.Hour)
		adapter.mu.Unlock()

		valid := adapter.IsClaudeTokenValid()
		assert.False(t, valid)
	})

	t.Run("empty token with future expiry", func(t *testing.T) {
		t.Parallel()
		adapter := newAdapter()
		adapter.mu.Lock()
		adapter.claudeToken = ""
		adapter.claudeExpiry = time.Now().Add(time.Hour)
		adapter.mu.Unlock()

		valid := adapter.IsClaudeTokenValid()
		assert.False(t, valid)
	})
}

func TestOAuthAdapter_IsQwenTokenValid(t *testing.T) {
	t.Parallel()
	// Per-subtest adapter — see BUGFIX #26 for the rationale
	// (parallel subtests must not share a mutable adapter).
	newAdapter := func() *OAuthAdapter { return NewOAuthAdapter(nil, nil) }

	t.Run("no token", func(t *testing.T) {
		t.Parallel()
		adapter := newAdapter()
		valid := adapter.IsQwenTokenValid()
		assert.False(t, valid)
	})

	t.Run("valid token", func(t *testing.T) {
		t.Parallel()
		adapter := newAdapter()
		adapter.mu.Lock()
		adapter.qwenToken = "test-token"
		adapter.qwenExpiry = time.Now().Add(time.Hour)
		adapter.mu.Unlock()

		valid := adapter.IsQwenTokenValid()
		assert.True(t, valid)
	})

	t.Run("expired token", func(t *testing.T) {
		t.Parallel()
		adapter := newAdapter()
		adapter.mu.Lock()
		adapter.qwenToken = "test-token"
		adapter.qwenExpiry = time.Now().Add(-time.Hour)
		adapter.mu.Unlock()

		valid := adapter.IsQwenTokenValid()
		assert.False(t, valid)
	})
}

func TestOAuthAdapter_VerifyClaudeOAuth(t *testing.T) {
	t.Parallel()
	// Create verification service for the adapter
	verifierSvc := verifier.NewVerificationService(&verifier.Config{})
	verifierSvc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code", nil
	})

	adapter := NewOAuthAdapter(verifierSvc, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	provider, err := adapter.VerifyClaudeOAuth(ctx)

	// Test depends on whether credentials exist in the environment
	// If credentials exist, provider should be returned with no error
	// If no credentials, error should be returned
	if err != nil {
		// No credentials - expected in CI/test environments
		assert.Nil(t, provider)
	} else {
		// Credentials exist - verify provider is valid
		assert.NotNil(t, provider)
		assert.Equal(t, "claude", provider.ID)
		assert.Equal(t, verifier.AuthTypeOAuth, provider.AuthType)
	}
}

func TestOAuthAdapter_VerifyQwenOAuth(t *testing.T) {
	t.Parallel()
	// Create verification service for the adapter
	verifierSvc := verifier.NewVerificationService(&verifier.Config{})
	verifierSvc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code", nil
	})

	adapter := NewOAuthAdapter(verifierSvc, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	provider, err := adapter.VerifyQwenOAuth(ctx)

	// Test depends on whether credentials exist in the environment
	// If credentials exist, provider should be returned with no error
	// If no credentials, error should be returned
	if err != nil {
		// No credentials - expected in CI/test environments
		assert.Nil(t, provider)
	} else {
		// Credentials exist - verify provider is valid
		assert.NotNil(t, provider)
		assert.Equal(t, "qwen", provider.ID)
		assert.Equal(t, verifier.AuthTypeOAuth, provider.AuthType)
	}
}

func TestOAuthAdapter_RefreshTokenIfNeeded_Claude(t *testing.T) {
	t.Parallel()
	t.Run("token not near expiry", func(t *testing.T) {
		adapter := NewOAuthAdapter(nil, nil)

		// Set token with long expiry
		adapter.mu.Lock()
		adapter.claudeToken = "test-token"
		adapter.claudeExpiry = time.Now().Add(time.Hour)
		adapter.mu.Unlock()

		ctx := context.Background()
		err := adapter.RefreshTokenIfNeeded(ctx, "claude")

		// Should not error since token is not near expiry
		assert.NoError(t, err)
	})

	t.Run("token near expiry", func(t *testing.T) {
		t.Parallel()
		adapter := NewOAuthAdapter(nil, nil)

		// Set token with near expiry
		adapter.mu.Lock()
		adapter.claudeToken = "test-token"
		adapter.claudeExpiry = time.Now().Add(5 * time.Minute)
		adapter.mu.Unlock()

		ctx := context.Background()
		err := adapter.RefreshTokenIfNeeded(ctx, "claude")

		// May or may not error depending on whether credentials exist
		// The important thing is the function completes without panic
		_ = err
	})

	t.Run("unknown provider type", func(t *testing.T) {
		t.Parallel()
		adapter := NewOAuthAdapter(nil, nil)

		ctx := context.Background()
		err := adapter.RefreshTokenIfNeeded(ctx, "unknown")

		// Should not error for unknown provider type
		assert.NoError(t, err)
	})
}

func TestOAuthAdapter_RefreshTokenIfNeeded_Qwen(t *testing.T) {
	t.Parallel()
	t.Run("token not near expiry", func(t *testing.T) {
		adapter := NewOAuthAdapter(nil, nil)

		// Set token with long expiry
		adapter.mu.Lock()
		adapter.qwenToken = "test-token"
		adapter.qwenExpiry = time.Now().Add(time.Hour)
		adapter.mu.Unlock()

		ctx := context.Background()
		err := adapter.RefreshTokenIfNeeded(ctx, "qwen")

		// Should not error since token is not near expiry
		assert.NoError(t, err)
	})

	t.Run("token near expiry with credentials", func(t *testing.T) {
		t.Parallel()
		// Check if Qwen credentials exist before running test
		credReader := oauth_credentials.NewOAuthCredentialReader()
		_, err := credReader.ReadQwenCredentials()
		if err != nil {
			t.Skip("Qwen credentials not available - skipping refresh test")
		}

		adapter := NewOAuthAdapter(nil, nil)

		// Override config to use shorter threshold for testing
		adapter.config.RefreshThresholdMins = 15

		// Set token with near expiry (less than threshold)
		adapter.mu.Lock()
		adapter.qwenToken = "test-token"
		adapter.qwenExpiry = time.Now().Add(5 * time.Minute)
		adapter.mu.Unlock()

		ctx := context.Background()
		refreshErr := adapter.RefreshTokenIfNeeded(ctx, "qwen")

		// If credentials exist, refresh should succeed or fail with specific error
		// We're just testing that the method doesn't panic
		if refreshErr != nil {
			t.Logf("Refresh returned error (expected if token refresh fails): %v", refreshErr)
		}
	})
}

func TestOAuthAdapter_StartBackgroundRefresh(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	// Set tokens with future expiry to avoid refresh attempts
	adapter.mu.Lock()
	adapter.claudeToken = "test-token"
	adapter.claudeExpiry = time.Now().Add(time.Hour)
	adapter.qwenToken = "test-token"
	adapter.qwenExpiry = time.Now().Add(time.Hour)
	adapter.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start background refresh with short interval
	adapter.StartBackgroundRefresh(ctx, 50*time.Millisecond)

	// Wait for context to be done
	<-ctx.Done()

	// Test passes if no panic occurs
}

func TestOAuthAdapter_ConcurrentAccess_Comprehensive(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	done := make(chan bool, 20)

	// Concurrent reads and writes
	for i := 0; i < 10; i++ {
		go func() {
			adapter.mu.Lock()
			adapter.claudeToken = "test-token"
			adapter.claudeExpiry = time.Now().Add(time.Hour)
			adapter.mu.Unlock()
			done <- true
		}()
		go func() {
			_ = adapter.IsClaudeTokenValid()
			_, _ = adapter.GetClaudeToken()
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestOAuthAdapterConfig_Fields_Comprehensive(t *testing.T) {
	t.Parallel()
	cfg := &OAuthAdapterConfig{
		RefreshThresholdMins:       15,
		TrustOnVerificationFailure: true,
		DefaultScoreOnFailure:      9.0,
		OAuthPriorityBoost:         2.0,
		VerificationTimeout:        45 * time.Second,
		RefreshTimeout:             90 * time.Second,
		EnableCLIRefresh:           false,
		CLIRefreshRetries:          5,
	}

	assert.Equal(t, 15, cfg.RefreshThresholdMins)
	assert.True(t, cfg.TrustOnVerificationFailure)
	assert.Equal(t, 9.0, cfg.DefaultScoreOnFailure)
	assert.Equal(t, 2.0, cfg.OAuthPriorityBoost)
	assert.Equal(t, 45*time.Second, cfg.VerificationTimeout)
	assert.Equal(t, 90*time.Second, cfg.RefreshTimeout)
	assert.False(t, cfg.EnableCLIRefresh)
	assert.Equal(t, 5, cfg.CLIRefreshRetries)
}

func TestOAuthAdapter_TokenState(t *testing.T) {
	t.Parallel()
	// Per-subtest adapter — "initial state" expected empty tokens but
	// the shared adapter was being mutated by "after setting tokens"
	// in parallel (race-debt BUGFIX #26).
	newAdapter := func() *OAuthAdapter { return NewOAuthAdapter(nil, nil) }

	t.Run("initial state", func(t *testing.T) {
		t.Parallel()
		adapter := newAdapter()
		token, expiry := adapter.GetClaudeToken()
		assert.Empty(t, token)
		assert.True(t, expiry.IsZero())

		token, expiry = adapter.GetQwenToken()
		assert.Empty(t, token)
		assert.True(t, expiry.IsZero())
	})

	t.Run("after setting tokens", func(t *testing.T) {
		t.Parallel()
		adapter := newAdapter()
		now := time.Now()
		claudeExpiry := now.Add(time.Hour)
		qwenExpiry := now.Add(2 * time.Hour)

		adapter.mu.Lock()
		adapter.claudeToken = "claude-token-123"
		adapter.claudeExpiry = claudeExpiry
		adapter.qwenToken = "qwen-token-456"
		adapter.qwenExpiry = qwenExpiry
		adapter.mu.Unlock()

		cToken, cExpiry := adapter.GetClaudeToken()
		assert.Equal(t, "claude-token-123", cToken)
		assert.Equal(t, claudeExpiry, cExpiry)

		qToken, qExpiry := adapter.GetQwenToken()
		assert.Equal(t, "qwen-token-456", qToken)
		assert.Equal(t, qwenExpiry, qExpiry)
	})
}

func TestOAuthAdapter_WithVerificationService(t *testing.T) {
	t.Parallel()
	verifierSvc := verifier.NewVerificationService(&verifier.Config{})
	verifierSvc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code", nil
	})

	adapter := NewOAuthAdapter(verifierSvc, nil)

	assert.NotNil(t, adapter.verifierSvc)
	assert.Equal(t, verifierSvc, adapter.verifierSvc)
}

func TestOAuthAdapter_EdgeCases(t *testing.T) {
	t.Parallel()
	t.Run("refresh threshold exactly at boundary", func(t *testing.T) {
		adapter := NewOAuthAdapter(nil, nil)
		adapter.config.RefreshThresholdMins = 10

		// Token expires exactly at threshold
		adapter.mu.Lock()
		adapter.claudeToken = "test-token"
		adapter.claudeExpiry = time.Now().Add(10 * time.Minute)
		adapter.mu.Unlock()

		// Should trigger refresh (time.Until < threshold)
		ctx := context.Background()
		err := adapter.RefreshTokenIfNeeded(ctx, "claude")
		// May or may not error depending on credentials
		// The important thing is the function completes without panic
		_ = err
	})

	t.Run("token expiry in past", func(t *testing.T) {
		t.Parallel()
		adapter := NewOAuthAdapter(nil, nil)

		adapter.mu.Lock()
		adapter.claudeToken = "expired-token"
		adapter.claudeExpiry = time.Now().Add(-time.Hour)
		adapter.mu.Unlock()

		valid := adapter.IsClaudeTokenValid()
		assert.False(t, valid)
	})

	t.Run("zero expiry time", func(t *testing.T) {
		t.Parallel()
		adapter := NewOAuthAdapter(nil, nil)

		adapter.mu.Lock()
		adapter.claudeToken = "test-token"
		adapter.claudeExpiry = time.Time{}
		adapter.mu.Unlock()

		valid := adapter.IsClaudeTokenValid()
		assert.False(t, valid)
	})
}
