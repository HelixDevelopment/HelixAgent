// Package adapters provides provider adapters for LLMsVerifier integration
package adapters

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultOAuthAdapterConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultOAuthAdapterConfig()

	assert.NotNil(t, cfg)
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

func TestNewOAuthAdapter(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	require.NotNil(t, adapter)
	assert.NotNil(t, adapter.credReader)
	assert.NotNil(t, adapter.config)
	assert.NotNil(t, adapter.log)
	assert.Nil(t, adapter.verifierSvc) // No verifier service provided
}

func TestNewOAuthAdapterWithConfig(t *testing.T) {
	t.Parallel()
	customConfig := &OAuthAdapterConfig{
		RefreshThresholdMins:       5,
		TrustOnVerificationFailure: false,
		DefaultScoreOnFailure:      6.0,
		OAuthPriorityBoost:         1.0,
		VerificationTimeout:        45 * time.Second,
		RefreshTimeout:             90 * time.Second,
		EnableCLIRefresh:           false,
		CLIRefreshRetries:          5,
	}

	adapter := NewOAuthAdapterWithConfig(nil, customConfig, nil)

	require.NotNil(t, adapter)
	assert.Equal(t, 5, adapter.config.RefreshThresholdMins)
	assert.False(t, adapter.config.TrustOnVerificationFailure)
	assert.Equal(t, 6.0, adapter.config.DefaultScoreOnFailure)
	assert.Equal(t, 1.0, adapter.config.OAuthPriorityBoost)
	assert.Equal(t, 45*time.Second, adapter.config.VerificationTimeout)
	assert.Equal(t, 90*time.Second, adapter.config.RefreshTimeout)
	assert.False(t, adapter.config.EnableCLIRefresh)
	assert.Equal(t, 5, adapter.config.CLIRefreshRetries)
}

func TestNewOAuthAdapterWithConfig_NilConfig(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapterWithConfig(nil, nil, nil)

	require.NotNil(t, adapter)
	// Should use default config
	assert.Equal(t, 10, adapter.config.RefreshThresholdMins)
	// IMPORTANT: Default is false - don't trust tokens that can't make API calls
	assert.False(t, adapter.config.TrustOnVerificationFailure)
}

func TestOAuthAdapter_GetClaudeToken_Empty(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	token, expiry := adapter.GetClaudeToken()

	assert.Empty(t, token)
	assert.True(t, expiry.IsZero())
}

func TestOAuthAdapter_GetQwenToken_Empty(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	token, expiry := adapter.GetQwenToken()

	assert.Empty(t, token)
	assert.True(t, expiry.IsZero())
}

func TestOAuthAdapter_IsClaudeTokenValid_Empty(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	valid := adapter.IsClaudeTokenValid()

	assert.False(t, valid)
}

func TestOAuthAdapter_IsQwenTokenValid_Empty(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	valid := adapter.IsQwenTokenValid()

	assert.False(t, valid)
}

func TestOAuthAdapter_IsClaudeTokenValid_WithToken(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	// Set a token manually for testing
	adapter.mu.Lock()
	adapter.claudeToken = "test-token"
	adapter.claudeExpiry = time.Now().Add(1 * time.Hour)
	adapter.mu.Unlock()

	valid := adapter.IsClaudeTokenValid()

	assert.True(t, valid)
}

func TestOAuthAdapter_IsQwenTokenValid_WithToken(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	// Set a token manually for testing
	adapter.mu.Lock()
	adapter.qwenToken = "test-token"
	adapter.qwenExpiry = time.Now().Add(1 * time.Hour)
	adapter.mu.Unlock()

	valid := adapter.IsQwenTokenValid()

	assert.True(t, valid)
}

func TestOAuthAdapter_IsClaudeTokenValid_ExpiredToken(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	// Set an expired token
	adapter.mu.Lock()
	adapter.claudeToken = "test-token"
	adapter.claudeExpiry = time.Now().Add(-1 * time.Hour)
	adapter.mu.Unlock()

	valid := adapter.IsClaudeTokenValid()

	assert.False(t, valid)
}

func TestOAuthAdapter_IsQwenTokenValid_ExpiredToken(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	// Set an expired token
	adapter.mu.Lock()
	adapter.qwenToken = "test-token"
	adapter.qwenExpiry = time.Now().Add(-1 * time.Hour)
	adapter.mu.Unlock()

	valid := adapter.IsQwenTokenValid()

	assert.False(t, valid)
}

func TestOAuthAdapter_GetClaudeToken_WithToken(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	expectedToken := "test-claude-token"
	expectedExpiry := time.Now().Add(1 * time.Hour)

	adapter.mu.Lock()
	adapter.claudeToken = expectedToken
	adapter.claudeExpiry = expectedExpiry
	adapter.mu.Unlock()

	token, expiry := adapter.GetClaudeToken()

	assert.Equal(t, expectedToken, token)
	assert.Equal(t, expectedExpiry, expiry)
}

func TestOAuthAdapter_GetQwenToken_WithToken(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	expectedToken := "test-qwen-token"
	expectedExpiry := time.Now().Add(1 * time.Hour)

	adapter.mu.Lock()
	adapter.qwenToken = expectedToken
	adapter.qwenExpiry = expectedExpiry
	adapter.mu.Unlock()

	token, expiry := adapter.GetQwenToken()

	assert.Equal(t, expectedToken, token)
	assert.Equal(t, expectedExpiry, expiry)
}

func TestOAuthAdapterConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := &OAuthAdapterConfig{
		RefreshThresholdMins:       15,
		TrustOnVerificationFailure: true,
		DefaultScoreOnFailure:      8.0,
		OAuthPriorityBoost:         0.75,
		VerificationTimeout:        20 * time.Second,
		RefreshTimeout:             40 * time.Second,
		EnableCLIRefresh:           true,
		CLIRefreshRetries:          2,
	}

	assert.Equal(t, 15, cfg.RefreshThresholdMins)
	assert.True(t, cfg.TrustOnVerificationFailure)
	assert.Equal(t, 8.0, cfg.DefaultScoreOnFailure)
	assert.Equal(t, 0.75, cfg.OAuthPriorityBoost)
	assert.Equal(t, 20*time.Second, cfg.VerificationTimeout)
	assert.Equal(t, 40*time.Second, cfg.RefreshTimeout)
	assert.True(t, cfg.EnableCLIRefresh)
	assert.Equal(t, 2, cfg.CLIRefreshRetries)
}

func TestOAuthAdapterConfig_ZeroValue(t *testing.T) {
	t.Parallel()
	cfg := &OAuthAdapterConfig{}

	assert.Equal(t, 0, cfg.RefreshThresholdMins)
	assert.False(t, cfg.TrustOnVerificationFailure)
	assert.Equal(t, 0.0, cfg.DefaultScoreOnFailure)
	assert.Equal(t, 0.0, cfg.OAuthPriorityBoost)
	assert.Equal(t, time.Duration(0), cfg.VerificationTimeout)
	assert.Equal(t, time.Duration(0), cfg.RefreshTimeout)
	assert.False(t, cfg.EnableCLIRefresh)
	assert.Equal(t, 0, cfg.CLIRefreshRetries)
}

func TestOAuthAdapter_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	// Simulate concurrent read/write operations
	done := make(chan bool, 4)

	// Writer for Claude token
	go func() {
		for i := 0; i < 100; i++ {
			adapter.mu.Lock()
			adapter.claudeToken = "test-token"
			adapter.claudeExpiry = time.Now().Add(1 * time.Hour)
			adapter.mu.Unlock()
		}
		done <- true
	}()

	// Reader for Claude token
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = adapter.GetClaudeToken()
		}
		done <- true
	}()

	// Writer for Qwen token
	go func() {
		for i := 0; i < 100; i++ {
			adapter.mu.Lock()
			adapter.qwenToken = "test-token"
			adapter.qwenExpiry = time.Now().Add(1 * time.Hour)
			adapter.mu.Unlock()
		}
		done <- true
	}()

	// Reader for Qwen token
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = adapter.GetQwenToken()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}
}

func TestOAuthAdapter_TokenValidityChecks_Concurrent(t *testing.T) {
	t.Parallel()
	adapter := NewOAuthAdapter(nil, nil)

	// Set initial tokens
	adapter.mu.Lock()
	adapter.claudeToken = "claude-token"
	adapter.claudeExpiry = time.Now().Add(1 * time.Hour)
	adapter.qwenToken = "qwen-token"
	adapter.qwenExpiry = time.Now().Add(1 * time.Hour)
	adapter.mu.Unlock()

	done := make(chan bool, 2)

	// Check Claude validity concurrently
	go func() {
		for i := 0; i < 100; i++ {
			_ = adapter.IsClaudeTokenValid()
		}
		done <- true
	}()

	// Check Qwen validity concurrently
	go func() {
		for i := 0; i < 100; i++ {
			_ = adapter.IsQwenTokenValid()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 2; i++ {
		<-done
	}
}
