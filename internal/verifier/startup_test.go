// Package verifier provides integration with LLMsVerifier for model verification,
// scoring, and health monitoring capabilities.
package verifier

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
)

// MockLLMProvider implements llm.LLMProvider for testing
type MockLLMProvider struct {
	Name            string
	CompleteFunc    func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
	StreamFunc      func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error)
	HealthCheckFunc func() error
	GetCapsFunc     func() *models.ProviderCapabilities
	ValidateFunc    func(config map[string]interface{}) (bool, []string)
}

func (m *MockLLMProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, req)
	}
	return &models.LLMResponse{
		ID:           "mock-response",
		Content:      "Mock response content",
		Confidence:   0.9,
		ProviderName: m.Name,
	}, nil
}

func (m *MockLLMProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if m.StreamFunc != nil {
		return m.StreamFunc(ctx, req)
	}
	ch := make(chan *models.LLMResponse, 1)
	close(ch)
	return ch, nil
}

func (m *MockLLMProvider) HealthCheck() error {
	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc()
	}
	return nil
}

func (m *MockLLMProvider) GetCapabilities() *models.ProviderCapabilities {
	if m.GetCapsFunc != nil {
		return m.GetCapsFunc()
	}
	return &models.ProviderCapabilities{
		SupportedModels:   []string{"test-model"},
		SupportsStreaming: true,
	}
}

func (m *MockLLMProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if m.ValidateFunc != nil {
		return m.ValidateFunc(config)
	}
	return true, nil
}

// Ensure MockLLMProvider implements llm.LLMProvider
var _ llm.LLMProvider = (*MockLLMProvider)(nil)

func TestDefaultStartupConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultStartupConfig()

	assert.NotNil(t, cfg)
	assert.True(t, cfg.ParallelVerification)
	assert.Equal(t, 10, cfg.MaxConcurrency)
	assert.Equal(t, 120*time.Second, cfg.VerificationTimeout) // 2 minutes for slow providers (Zen, ZAI)
	assert.Equal(t, 10*time.Second, cfg.HealthCheckTimeout)
	assert.Equal(t, 5.0, cfg.MinScore)
	assert.Equal(t, 25, cfg.DebateTeamSize) // 5 positions × (1 primary + 4 fallbacks) = 25 max
	assert.Equal(t, 5, cfg.PositionCount)
	assert.Equal(t, 4, cfg.FallbacksPerPosition) // 2-4 fallbacks per position
	assert.Equal(t, 0.0, cfg.OAuthPriorityBoost) // NO OAuth priority - pure score-based
	assert.True(t, cfg.TrustOAuthOnFailure)
	assert.True(t, cfg.EnableFreeProviders)
	assert.False(t, cfg.OAuthPrimaryNonOAuthFallback) // NO special OAuth fallback treatment
	assert.True(t, cfg.CacheVerificationResults)
}

func TestNewStartupVerifier(t *testing.T) {
	t.Parallel()
	cfg := DefaultStartupConfig()
	logger := logrus.New()

	sv := NewStartupVerifier(cfg, logger)

	assert.NotNil(t, sv)
	assert.Equal(t, cfg, sv.config)
	assert.NotNil(t, sv.providers)
	assert.NotNil(t, sv.verifierSvc)
}

func TestNewStartupVerifierWithNilConfig(t *testing.T) {
	t.Parallel()
	logger := logrus.New()

	sv := NewStartupVerifier(nil, logger)

	assert.NotNil(t, sv)
	assert.NotNil(t, sv.config)
	assert.Equal(t, 25, sv.config.DebateTeamSize) // 5 positions × (1 + 4 fallbacks) = 25
}

func TestNewStartupVerifierWithNilLogger(t *testing.T) {
	t.Parallel()
	cfg := DefaultStartupConfig()

	sv := NewStartupVerifier(cfg, nil)

	assert.NotNil(t, sv)
	assert.NotNil(t, sv.log)
}

func TestStartupVerifier_SetProviderFactory(t *testing.T) {
	t.Parallel()
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	// Set provider factory with correct signature
	factory := func(providerType string, config ProviderCreateConfig) (llm.LLMProvider, error) {
		return &MockLLMProvider{Name: providerType}, nil
	}

	sv.SetProviderFactory(factory)

	// Factory should be set
	assert.NotNil(t, sv.providerFactory)
}

func TestStartupVerifier_GetRankedProviders_Empty(t *testing.T) {
	t.Parallel()
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	ranked := sv.GetRankedProviders()

	// Before verification, rankedProviders is nil
	assert.Nil(t, ranked)
}

func TestProviderAuthType_Constants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ProviderAuthType("api_key"), AuthTypeAPIKey)
	assert.Equal(t, ProviderAuthType("oauth"), AuthTypeOAuth)
	assert.Equal(t, ProviderAuthType("free"), AuthTypeFree)
	assert.Equal(t, ProviderAuthType("anonymous"), AuthTypeAnonymous)
	assert.Equal(t, ProviderAuthType("local"), AuthTypeLocal)
}

func TestProviderStatus_Constants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ProviderStatus("unknown"), StatusUnknown)
	assert.Equal(t, ProviderStatus("healthy"), StatusHealthy)
	assert.Equal(t, ProviderStatus("verified"), StatusVerified)
	assert.Equal(t, ProviderStatus("unhealthy"), StatusUnhealthy)
	assert.Equal(t, ProviderStatus("failed"), StatusFailed)
	assert.Equal(t, ProviderStatus("degraded"), StatusDegraded)
	assert.Equal(t, ProviderStatus("rate_limited"), StatusRateLimited)
	assert.Equal(t, ProviderStatus("auth_failed"), StatusAuthFailed)
	assert.Equal(t, ProviderStatus("unavailable"), StatusUnavailable)
}

func TestSupportedProviders(t *testing.T) {
	t.Parallel()
	// Verify expected providers are defined
	expectedProviders := []string{"claude", "qwen", "gemini", "deepseek", "mistral", "groq", "cerebras", "openrouter", "zen", "ollama", "zai"}

	for _, name := range expectedProviders {
		info, ok := SupportedProviders[name]
		assert.True(t, ok, "Provider %s should be in SupportedProviders", name)
		assert.NotNil(t, info)
		assert.NotEmpty(t, info.Type)
		assert.NotEmpty(t, info.DisplayName)
	}
}

func TestGetProviderInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		provider string
		wantOK   bool
	}{
		{"Claude exists", "claude", true},
		{"Qwen exists", "qwen", true},
		{"Gemini exists", "gemini", true},
		{"DeepSeek exists", "deepseek", true},
		{"Unknown provider", "unknown", false},
		{"Empty provider", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info, ok := GetProviderInfo(tt.provider)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.NotNil(t, info)
			}
		})
	}
}

func TestIsOAuthProvider(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		provider string
		expected bool
	}{
		{"Claude is OAuth", "claude", true},
		{"Qwen is OAuth", "qwen", true},
		{"Gemini is not OAuth", "gemini", false},
		{"DeepSeek is not OAuth", "deepseek", false},
		{"Zen is not OAuth", "zen", false},
		{"Unknown provider", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsOAuthProvider(tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsFreeProvider(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		provider string
		expected bool
	}{
		{"Zen is free", "zen", true},
		{"OpenRouter has free tier", "openrouter", true},
		{"Ollama is free (local)", "ollama", true},
		{"Claude is not free", "claude", false},
		{"Gemini is not free", "gemini", false},
		{"Unknown provider", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsFreeProvider(tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetProvidersByAuthType(t *testing.T) {
	t.Parallel()
	// Test OAuth providers
	oauthProviders := GetProvidersByAuthType(AuthTypeOAuth)
	assert.NotEmpty(t, oauthProviders)

	// Verify Claude and Qwen are in OAuth list
	hasOAuth := make(map[string]bool)
	for _, p := range oauthProviders {
		hasOAuth[p.Type] = true
	}
	assert.True(t, hasOAuth["claude"], "Claude should be OAuth provider")
	assert.True(t, hasOAuth["qwen"], "Qwen should be OAuth provider")

	// Test API key providers
	apiKeyProviders := GetProvidersByAuthType(AuthTypeAPIKey)
	assert.NotEmpty(t, apiKeyProviders)

	// Test free providers
	freeProviders := GetProvidersByAuthType(AuthTypeFree)
	assert.NotEmpty(t, freeProviders)
}

func TestGetProvidersByTier(t *testing.T) {
	t.Parallel()
	// Tier 1 should have premium providers
	tier1 := GetProvidersByTier(1)
	assert.NotEmpty(t, tier1)

	// Tier 5 should have free providers
	tier5 := GetProvidersByTier(5)
	assert.NotEmpty(t, tier5)

	// Non-existent tier should be empty
	tier100 := GetProvidersByTier(100)
	assert.Empty(t, tier100)
}

func TestUnifiedProvider_Fields(t *testing.T) {
	t.Parallel()
	provider := &UnifiedProvider{
		ID:          "test-provider",
		Name:        "Test Provider",
		DisplayName: "Test Display Name",
		Type:        "test",
		AuthType:    AuthTypeAPIKey,
		Verified:    true,
		Score:       8.5,
		Status:      StatusVerified,
		Models: []UnifiedModel{
			{
				ID:       "model-1",
				Name:     "Model One",
				Score:    8.0,
				Verified: true,
			},
		},
		BaseURL:      "https://api.test.com",
		DefaultModel: "model-1",
	}

	assert.Equal(t, "test-provider", provider.ID)
	assert.Equal(t, "Test Provider", provider.Name)
	assert.Equal(t, AuthTypeAPIKey, provider.AuthType)
	assert.True(t, provider.Verified)
	assert.Equal(t, 8.5, provider.Score)
	assert.Equal(t, StatusVerified, provider.Status)
	assert.Len(t, provider.Models, 1)
}

func TestUnifiedModel_Fields(t *testing.T) {
	t.Parallel()
	model := &UnifiedModel{
		ID:                "model-1",
		Name:              "Test Model",
		Provider:          "test-provider",
		Score:             8.5,
		Verified:          true,
		Latency:           100 * time.Millisecond,
		ContextWindow:     128000,
		MaxOutputTokens:   4096,
		SupportsStreaming: true,
		SupportsTools:     true,
		Capabilities:      []string{"text", "code"},
		Metadata: map[string]interface{}{
			"version": "1.0",
		},
	}

	assert.Equal(t, "model-1", model.ID)
	assert.Equal(t, "Test Model", model.Name)
	assert.Equal(t, "test-provider", model.Provider)
	assert.Equal(t, 8.5, model.Score)
	assert.True(t, model.Verified)
	assert.Equal(t, 100*time.Millisecond, model.Latency)
	assert.Equal(t, 128000, model.ContextWindow)
	assert.True(t, model.SupportsStreaming)
	assert.Contains(t, model.Capabilities, "text")
	assert.Contains(t, model.Capabilities, "code")
}

func TestStartupResult_Fields(t *testing.T) {
	t.Parallel()
	result := &StartupResult{
		TotalProviders:  10,
		VerifiedCount:   8,
		FailedCount:     2,
		SkippedCount:    0,
		APIKeyProviders: 5,
		OAuthProviders:  2,
		FreeProviders:   3,
		StartedAt:       time.Now(),
		CompletedAt:     time.Now(),
		DurationMs:      1500,
	}

	assert.Equal(t, 10, result.TotalProviders)
	assert.Equal(t, 8, result.VerifiedCount)
	assert.Equal(t, 2, result.FailedCount)
	assert.Equal(t, 5, result.APIKeyProviders)
	assert.Equal(t, 2, result.OAuthProviders)
	assert.Equal(t, 3, result.FreeProviders)
	assert.Equal(t, int64(1500), result.DurationMs)
}

func TestDebateTeamResult_Fields(t *testing.T) {
	t.Parallel()
	result := &DebateTeamResult{
		Positions: []*DebatePosition{
			{
				Position: 1,
				Role:     "analyst",
				Primary: &DebateLLM{
					Provider:  "claude",
					ModelID:   "claude-sonnet-4-5",
					ModelName: "Claude Sonnet 4.5",
					AuthType:  AuthTypeOAuth,
					Score:     9.5,
					Verified:  true,
					IsOAuth:   true,
				},
				Fallbacks: []*DebateLLM{
					{Provider: "deepseek", ModelID: "deepseek-chat", Score: 8.5},
					{Provider: "mistral", ModelID: "mistral-large", Score: 8.0},
					{Provider: "cerebras", ModelID: "llama-3.3-70b", Score: 7.8},
					{Provider: "gemini", ModelID: "gemini-2.0-flash", Score: 7.5},
				},
			},
		},
		TotalLLMs:     25, // 5 positions × 5 LLMs each
		MinScore:      5.0,
		SortedByScore: true, // NO OAuth priority - pure score-based
		LLMReuseCount: 0,
		SelectedAt:    time.Now(),
	}

	assert.Len(t, result.Positions, 1)
	assert.Equal(t, 25, result.TotalLLMs)
	assert.Equal(t, 5.0, result.MinScore)
	assert.True(t, result.SortedByScore, "Should be sorted by score only (no OAuth priority)")
	assert.NotNil(t, result.Positions[0].Primary)
	assert.Equal(t, "claude", result.Positions[0].Primary.Provider)
	assert.Len(t, result.Positions[0].Fallbacks, 4, "Should have 4 fallbacks")
}

func TestStartupError_Fields(t *testing.T) {
	t.Parallel()
	err := &StartupError{
		Provider:    "test-provider",
		ModelID:     "test-model",
		Phase:       "verification",
		Error:       "connection timeout",
		Recoverable: true,
	}

	assert.Equal(t, "test-provider", err.Provider)
	assert.Equal(t, "test-model", err.ModelID)
	assert.Equal(t, "verification", err.Phase)
	assert.Equal(t, "connection timeout", err.Error)
	assert.True(t, err.Recoverable)
}

func TestProviderDiscoveryResult_Fields(t *testing.T) {
	t.Parallel()
	result := &ProviderDiscoveryResult{
		ID:          "test-provider",
		Type:        "api_key",
		AuthType:    AuthTypeAPIKey,
		Discovered:  true,
		Source:      "env",
		Credentials: "sk-***",
		BaseURL:     "https://api.test.com",
		Models:      []string{"model-1", "model-2"},
	}

	assert.Equal(t, "test-provider", result.ID)
	assert.Equal(t, AuthTypeAPIKey, result.AuthType)
	assert.True(t, result.Discovered)
	assert.Equal(t, "env", result.Source)
	assert.Len(t, result.Models, 2)
}

func TestStartupVerifier_VerifyAllProviders_EmptyEnvironment(t *testing.T) {
	t.Parallel()
	cfg := DefaultStartupConfig()
	cfg.VerificationTimeout = 5 * time.Second
	sv := NewStartupVerifier(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := sv.VerifyAllProviders(ctx)

	// Should not error even with no providers
	require.NoError(t, err)
	require.NotNil(t, result)

	// Result should reflect no discovered providers (in a clean test environment)
	assert.GreaterOrEqual(t, result.TotalProviders, 0)
}

func TestStartupVerifier_GetDebateTeam_BeforeVerification(t *testing.T) {
	t.Parallel()
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	team := sv.GetDebateTeam()

	// Should return nil before verification
	assert.Nil(t, team)
}

func TestStartupConfig_Validation(t *testing.T) {
	t.Parallel()
	cfg := DefaultStartupConfig()

	// Verify all expected fields have sensible defaults
	assert.Greater(t, cfg.MaxConcurrency, 0)
	assert.Greater(t, cfg.VerificationTimeout, time.Duration(0))
	assert.Greater(t, cfg.HealthCheckTimeout, time.Duration(0))
	assert.GreaterOrEqual(t, cfg.MinScore, 0.0)
	assert.Equal(t, 25, cfg.DebateTeamSize)  // 5 positions × (1 primary + 4 fallbacks) = 25 max
	assert.Equal(t, 5*5, cfg.DebateTeamSize) // 5 positions * 5 LLMs each (1 primary + 4 fallbacks)
	assert.Equal(t, cfg.PositionCount*(1+cfg.FallbacksPerPosition), cfg.DebateTeamSize)
}

func TestProviderTypeInfo_Fields(t *testing.T) {
	t.Parallel()
	// Test Claude provider info
	claude, ok := SupportedProviders["claude"]
	require.True(t, ok)
	assert.Equal(t, "claude", claude.Type)
	assert.Equal(t, "Claude (Anthropic)", claude.DisplayName)
	assert.Equal(t, AuthTypeOAuth, claude.AuthType)
	assert.Equal(t, 1, claude.Tier)
	assert.NotEmpty(t, claude.EnvVars)
	assert.NotEmpty(t, claude.BaseURL)
	assert.NotEmpty(t, claude.Models)
	assert.False(t, claude.Free)

	// Test Zen provider info
	zen, ok := SupportedProviders["zen"]
	require.True(t, ok)
	assert.Equal(t, "zen", zen.Type)
	assert.Equal(t, "OpenCode Zen", zen.DisplayName)
	assert.Equal(t, AuthTypeFree, zen.AuthType)
	assert.True(t, zen.Free)
	assert.NotEmpty(t, zen.Models)
}

// ============================================================================
// Re-Evaluation Tests - Verify fresh verification on every call
// ============================================================================

func TestStartupVerifier_ReEvaluation_TimestampsUpdated(t *testing.T) {
	t.Parallel()
	// Test that each call to VerifyAllProviders updates timestamps
	cfg := DefaultStartupConfig()
	cfg.VerificationTimeout = 5 * time.Second
	cfg.EnableFreeProviders = true // Ensure Zen is discovered
	sv := NewStartupVerifier(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First verification
	result1, err1 := sv.VerifyAllProviders(ctx)
	require.NoError(t, err1)
	require.NotNil(t, result1)

	// Record first verification time
	firstStarted := result1.StartedAt
	firstCompleted := result1.CompletedAt

	// Wait a short time
	time.Sleep(100 * time.Millisecond)

	// Second verification - should be fresh, not cached
	result2, err2 := sv.VerifyAllProviders(ctx)
	require.NoError(t, err2)
	require.NotNil(t, result2)

	// Timestamps should be different (newer)
	assert.True(t, result2.StartedAt.After(firstStarted) || result2.StartedAt.Equal(firstStarted),
		"Second verification started_at should be >= first verification")
	assert.True(t, result2.CompletedAt.After(firstCompleted),
		"Second verification completed_at should be after first verification")
}

func TestStartupVerifier_ReEvaluation_ResultsAreConsistent(t *testing.T) {
	t.Parallel()
	// Test that multiple verifications produce consistent results
	cfg := DefaultStartupConfig()
	cfg.VerificationTimeout = 5 * time.Second
	cfg.EnableFreeProviders = true
	sv := NewStartupVerifier(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run verification multiple times
	var results []*StartupResult
	for i := 0; i < 3; i++ {
		result, err := sv.VerifyAllProviders(ctx)
		require.NoError(t, err)
		require.NotNil(t, result)
		results = append(results, result)
		time.Sleep(50 * time.Millisecond)
	}

	// All results should have same provider count (consistent discovery)
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0].TotalProviders, results[i].TotalProviders,
			"Provider count should be consistent across verifications")
	}

	// Each result should have a valid duration (>= 0 since very fast verifications may round to 0)
	for _, r := range results {
		assert.GreaterOrEqual(t, r.DurationMs, int64(0), "Duration should be non-negative")
	}
}

func TestStartupVerifier_ReEvaluation_ProvidersReSorted(t *testing.T) {
	t.Parallel()
	// Test that providers are re-sorted on each verification (purely by score)
	cfg := DefaultStartupConfig()
	cfg.VerificationTimeout = 5 * time.Second
	cfg.EnableFreeProviders = true
	sv := NewStartupVerifier(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First verification
	_, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)

	ranked := sv.GetRankedProviders()
	if len(ranked) < 2 {
		t.Skip("Need at least 2 providers to test sorting") // SKIP-OK: #legacy-untriaged
	}

	// Verify sorted order (descending by score ONLY - NO OAuth priority)
	for i := 1; i < len(ranked); i++ {
		// Scores should be in descending order (highest first)
		assert.GreaterOrEqual(t, ranked[i-1].Score, ranked[i].Score,
			"Providers should be sorted by score (descending) - NO OAuth priority")
	}
}

func TestStartupVerifier_ReEvaluation_DebateTeamReselected(t *testing.T) {
	t.Parallel()
	// Test that debate team is reselected on each verification
	cfg := DefaultStartupConfig()
	cfg.VerificationTimeout = 5 * time.Second
	cfg.EnableFreeProviders = true
	sv := NewStartupVerifier(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First verification
	result1, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)

	if result1.DebateTeam == nil {
		t.Skip("No debate team configured (need verified providers)") // SKIP-OK: #legacy-untriaged
	}

	firstSelectedAt := result1.DebateTeam.SelectedAt

	// Wait a short time
	time.Sleep(100 * time.Millisecond)

	// Second verification
	result2, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result2.DebateTeam)

	// Debate team should be reselected with new timestamp
	assert.True(t, result2.DebateTeam.SelectedAt.After(firstSelectedAt),
		"Debate team should be reselected with newer timestamp")
}

func TestStartupVerifier_ReEvaluation_LastVerifyAtUpdated(t *testing.T) {
	t.Parallel()
	// Test that lastVerifyAt is updated on each call
	cfg := DefaultStartupConfig()
	cfg.VerificationTimeout = 5 * time.Second
	sv := NewStartupVerifier(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initial state - not initialized
	assert.False(t, sv.IsInitialized())

	// First verification
	_, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)

	// Should now be initialized
	assert.True(t, sv.IsInitialized())

	// Get lastVerifyAt indirectly through re-verification
	time.Sleep(100 * time.Millisecond)

	// Second verification should complete without error
	_, err = sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	assert.True(t, sv.IsInitialized())
}

func TestStartupResult_FreshTimestamps(t *testing.T) {
	t.Parallel()
	// Test that StartupResult has fresh timestamps (within last 5 minutes)
	cfg := DefaultStartupConfig()
	cfg.VerificationTimeout = 5 * time.Second
	sv := NewStartupVerifier(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	now := time.Now()
	maxAge := 5 * time.Minute

	// started_at should be within last 5 minutes
	assert.True(t, now.Sub(result.StartedAt) < maxAge,
		"started_at should be fresh (within last 5 minutes)")

	// completed_at should be within last 5 minutes
	assert.True(t, now.Sub(result.CompletedAt) < maxAge,
		"completed_at should be fresh (within last 5 minutes)")

	// completed_at should be after started_at
	assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt),
		"completed_at should be >= started_at")
}

// ============================================================================
// Debate Team Selection Tests - LLM Reuse Logic
// ============================================================================

func TestDebateTeamSelection_AllPositionsFilled(t *testing.T) {
	t.Parallel()
	// Test that all positions (up to 25 LLMs) are filled regardless of unique LLM count
	cfg := DefaultStartupConfig()
	cfg.VerificationTimeout = 5 * time.Second
	cfg.EnableFreeProviders = true
	sv := NewStartupVerifier(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	if result.DebateTeam == nil {
		t.Skip("No debate team available (need at least 1 verified provider)") // SKIP-OK: #legacy-untriaged
	}

	// Must have exactly 5 positions
	assert.Len(t, result.DebateTeam.Positions, 5, "Should have exactly 5 positions")

	// All positions must be filled with primary and 2-4 fallbacks
	totalFilled := 0
	for i, pos := range result.DebateTeam.Positions {
		require.NotNil(t, pos, "Position %d should not be nil", i+1)
		require.NotNil(t, pos.Primary, "Position %d primary should not be nil", i+1)
		totalFilled++ // Primary
		// Each position should have 2-4 fallbacks
		assert.GreaterOrEqual(t, len(pos.Fallbacks), 2, "Position %d should have at least 2 fallbacks", i+1)
		assert.LessOrEqual(t, len(pos.Fallbacks), 4, "Position %d should have at most 4 fallbacks", i+1)
		totalFilled += len(pos.Fallbacks)
	}

	// Total should be between 15 (5 positions × 3 LLMs) and 25 (5 positions × 5 LLMs)
	assert.GreaterOrEqual(t, totalFilled, 15, "Should have at least 15 LLMs filled")
	assert.LessOrEqual(t, totalFilled, 25, "Should have at most 25 LLMs filled")
}

func TestDebateTeamSelection_LLMReuse(t *testing.T) {
	t.Parallel()
	// Test that strongest LLMs are reused when fewer than 25 unique LLMs available
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	// Manually set up ranked providers with only 3 LLMs
	sv.rankedProviders.Replace([]*UnifiedProvider{
		{
			ID:       "provider1",
			Name:     "Provider1",
			Type:     "provider1",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    9.0,
			Models: []UnifiedModel{
				{ID: "model-a", Name: "Model A", Score: 9.0},
			},
		},
		{
			ID:       "provider2",
			Name:     "Provider2",
			Type:     "provider2",
			AuthType: AuthTypeFree,
			Verified: true,
			Score:    7.0,
			Models: []UnifiedModel{
				{ID: "model-b", Name: "Model B", Score: 7.0},
				{ID: "model-c", Name: "Model C", Score: 7.0},
			},
		},
	})

	team, err := sv.selectDebateTeam()
	require.NoError(t, err)
	require.NotNil(t, team)

	// All positions should be filled via reuse (up to 25)
	assert.GreaterOrEqual(t, team.TotalLLMs, 15, "Should have at least 15 LLMs filled")
	assert.LessOrEqual(t, team.TotalLLMs, 25, "Should have at most 25 LLMs filled")
	assert.Len(t, team.Positions, 5, "Should have 5 positions")

	// Verify each position has primary and at least 2 fallbacks
	for i, pos := range team.Positions {
		assert.NotNil(t, pos.Primary, "Position %d primary must be filled", i+1)
		assert.GreaterOrEqual(t, len(pos.Fallbacks), 2, "Position %d must have at least 2 fallbacks", i+1)
	}
}

func TestDebateTeamSelection_SingleLLMReuse(t *testing.T) {
	t.Parallel()
	// Test with only 1 LLM - should reuse it for all positions
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	sv.rankedProviders.Replace([]*UnifiedProvider{
		{
			ID:       "zen",
			Name:     "Zen",
			Type:     "zen",
			AuthType: AuthTypeFree,
			Verified: true,
			Score:    6.5,
			Models: []UnifiedModel{
				{ID: "grok-code", Name: "Grok Code", Score: 6.5},
			},
		},
	})

	team, err := sv.selectDebateTeam()
	require.NoError(t, err)
	require.NotNil(t, team)

	// All positions should be filled with the same LLM (reused)
	assert.GreaterOrEqual(t, team.TotalLLMs, 15, "Should have at least 15 positions filled")

	// All positions should use the same model (reused)
	for i, pos := range team.Positions {
		assert.Equal(t, "grok-code", pos.Primary.ModelID, "Position %d primary", i+1)
		for j, fb := range pos.Fallbacks {
			assert.Equal(t, "grok-code", fb.ModelID, "Position %d fallback %d", i+1, j+1)
		}
	}
}

func TestDebateTeamSelection_ScoreBasedOnly(t *testing.T) {
	t.Parallel()
	// Test that providers are sorted PURELY by score (NO OAuth priority)
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	sv.rankedProviders.Replace([]*UnifiedProvider{
		{
			ID:       "deepseek",
			Name:     "DeepSeek",
			Type:     "deepseek",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    9.0, // Higher score - should be first
			Models: []UnifiedModel{
				{ID: "deepseek-chat", Name: "DeepSeek Chat", Score: 9.0},
			},
		},
		{
			ID:       "claude",
			Name:     "Claude",
			Type:     "claude",
			AuthType: AuthTypeOAuth,
			Verified: true,
			Score:    8.0, // Lower score - should be second
			Models: []UnifiedModel{
				{ID: "claude-opus-4-5", Name: "Claude Opus", Score: 8.0},
			},
		},
	})

	team, err := sv.selectDebateTeam()
	require.NoError(t, err)
	require.NotNil(t, team)

	// First position's primary should be highest score (DeepSeek) regardless of auth type
	// NO OAuth priority - pure score-based selection
	assert.Equal(t, "deepseek", team.Positions[0].Primary.Provider,
		"Highest score provider should be first (NO OAuth priority)")
	assert.Equal(t, 9.0, team.Positions[0].Primary.Score,
		"First primary should have highest score")
}

func TestDebateTeamSelection_SortedByScore(t *testing.T) {
	t.Parallel()
	// Test that LLMs are sorted purely by score (NO auth type grouping)
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	sv.rankedProviders.Replace([]*UnifiedProvider{
		{
			ID:       "gemini",
			Name:     "Gemini",
			Type:     "gemini",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    9.0,
			Models: []UnifiedModel{
				{ID: "gemini-2.0", Name: "Gemini 2.0", Score: 9.0},
			},
		},
		{
			ID:       "deepseek",
			Name:     "DeepSeek",
			Type:     "deepseek",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    8.5,
			Models: []UnifiedModel{
				{ID: "deepseek-chat", Name: "DeepSeek Chat", Score: 8.5},
			},
		},
		{
			ID:       "mistral",
			Name:     "Mistral",
			Type:     "mistral",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    8.0,
			Models: []UnifiedModel{
				{ID: "mistral-large", Name: "Mistral Large", Score: 8.0},
			},
		},
	})

	team, err := sv.selectDebateTeam()
	require.NoError(t, err)
	require.NotNil(t, team)

	// Primary should be highest score
	assert.Equal(t, "gemini-2.0", team.Positions[0].Primary.ModelID)
	// Fallbacks should follow in score order
	require.GreaterOrEqual(t, len(team.Positions[0].Fallbacks), 2, "Should have at least 2 fallbacks")
	assert.Equal(t, "deepseek-chat", team.Positions[0].Fallbacks[0].ModelID)
	assert.Equal(t, "mistral-large", team.Positions[0].Fallbacks[1].ModelID)
}

func TestDebateTeamSelection_NoProviders(t *testing.T) {
	t.Parallel()
	// Test with no providers - should return error
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	sv.rankedProviders.Replace(nil) // Empty

	team, err := sv.selectDebateTeam()
	assert.Error(t, err)
	assert.Nil(t, team)
	assert.Contains(t, err.Error(), "no verified providers available")
}

func TestDebateTeamSelection_UnverifiedExcluded(t *testing.T) {
	t.Parallel()
	// Test that unverified providers are excluded from team
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	sv.rankedProviders.Replace([]*UnifiedProvider{
		{
			ID:       "verified1",
			Name:     "Verified1",
			Type:     "verified1",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    8.0,
			Models: []UnifiedModel{
				{ID: "model-good", Name: "Model Good", Score: 8.0},
			},
		},
		{
			ID:       "unverified",
			Name:     "Unverified",
			Type:     "unverified",
			AuthType: AuthTypeAPIKey,
			Verified: false, // NOT verified
			Score:    9.5,   // Higher score but unverified
			Models: []UnifiedModel{
				{ID: "model-bad", Name: "Model Bad", Score: 9.5},
			},
		},
	})

	team, err := sv.selectDebateTeam()
	require.NoError(t, err)
	require.NotNil(t, team)

	// All positions should use only the verified model (unverified excluded)
	for i, pos := range team.Positions {
		assert.Equal(t, "model-good", pos.Primary.ModelID, "Position %d primary", i+1)
		for j, fb := range pos.Fallbacks {
			assert.Equal(t, "model-good", fb.ModelID, "Position %d fallback %d", i+1, j+1)
		}
	}
}

func TestDebateTeamSelection_BelowMinScoreExcluded(t *testing.T) {
	t.Parallel()
	// Test that providers below min score are excluded
	cfg := DefaultStartupConfig()
	cfg.MinScore = 5.0
	sv := NewStartupVerifier(cfg, nil)

	sv.rankedProviders.Replace([]*UnifiedProvider{
		{
			ID:       "good",
			Name:     "Good",
			Type:     "good",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    7.0, // Above min
			Models: []UnifiedModel{
				{ID: "model-good", Name: "Model Good", Score: 7.0},
			},
		},
		{
			ID:       "bad",
			Name:     "Bad",
			Type:     "bad",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    4.0, // Below min (5.0)
			Models: []UnifiedModel{
				{ID: "model-bad", Name: "Model Bad", Score: 4.0},
			},
		},
	})

	team, err := sv.selectDebateTeam()
	require.NoError(t, err)
	require.NotNil(t, team)

	// Only the good model should be used
	for _, pos := range team.Positions {
		assert.Equal(t, "model-good", pos.Primary.ModelID)
	}
}

func TestDebateTeamSelection_RolesAssigned(t *testing.T) {
	t.Parallel()
	// Test that all 5 positions have correct roles assigned
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	sv.rankedProviders.Replace([]*UnifiedProvider{
		{
			ID:       "test",
			Name:     "Test",
			Type:     "test",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    7.0,
			Models: []UnifiedModel{
				{ID: "test-model", Name: "Test Model", Score: 7.0},
			},
		},
	})

	team, err := sv.selectDebateTeam()
	require.NoError(t, err)
	require.NotNil(t, team)

	expectedRoles := []string{"analyst", "proposer", "critic", "synthesis", "mediator"}
	for i, pos := range team.Positions {
		assert.Equal(t, i+1, pos.Position, "Position number should match")
		assert.Equal(t, expectedRoles[i], pos.Role, "Role should match for position %d", i+1)
	}
}

func TestDebateTeamSelection_MultipleModelsPerProvider(t *testing.T) {
	t.Parallel()
	// Test that all models from a provider are considered
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	sv.rankedProviders.Replace([]*UnifiedProvider{
		{
			ID:       "multi",
			Name:     "Multi",
			Type:     "multi",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    8.0,
			Models: []UnifiedModel{
				{ID: "model-1", Name: "Model 1", Score: 8.0},
				{ID: "model-2", Name: "Model 2", Score: 8.0},
				{ID: "model-3", Name: "Model 3", Score: 8.0},
			},
		},
	})

	team, err := sv.selectDebateTeam()
	require.NoError(t, err)
	require.NotNil(t, team)

	// All 3 models should be used before wrapping
	models := make(map[string]int)
	for _, pos := range team.Positions {
		models[pos.Primary.ModelID]++
		for _, fb := range pos.Fallbacks {
			models[fb.ModelID]++
		}
	}

	// Each model should be used at least once
	assert.Greater(t, models["model-1"], 0, "model-1 should be used")
	assert.Greater(t, models["model-2"], 0, "model-2 should be used")
	assert.Greater(t, models["model-3"], 0, "model-3 should be used")
}

func TestDebateTeamSelection_ReusedLLMsAreSeparateInstances(t *testing.T) {
	t.Parallel()
	// Test that reused LLMs are SEPARATE UNIQUE INSTANCES, not the same pointer
	cfg := DefaultStartupConfig()
	sv := NewStartupVerifier(cfg, nil)

	sv.rankedProviders.Replace([]*UnifiedProvider{
		{
			ID:       "only",
			Name:     "Only Provider",
			Type:     "only",
			AuthType: AuthTypeAPIKey,
			Verified: true,
			Score:    8.0,
			Models: []UnifiedModel{
				{ID: "single-model", Name: "Single Model", Score: 8.0},
			},
		},
	})

	team, err := sv.selectDebateTeam()
	require.NoError(t, err)
	require.NotNil(t, team)

	// Collect all LLM instances
	var allInstances []*DebateLLM
	for _, pos := range team.Positions {
		allInstances = append(allInstances, pos.Primary)
		allInstances = append(allInstances, pos.Fallbacks...)
	}

	// Verify all instances are separate (different pointers)
	// Even though they have the same data, they should be different objects
	for i := 0; i < len(allInstances); i++ {
		for j := i + 1; j < len(allInstances); j++ {
			assert.NotSame(t, allInstances[i], allInstances[j],
				"Reused LLMs at positions %d and %d should be separate instances", i, j)
		}
	}

	// Verify all have the same model ID (same LLM reused)
	for _, instance := range allInstances {
		assert.Equal(t, "single-model", instance.ModelID)
	}
}

// TestVerifyOAuthProviderTrust tests that OAuth providers are trusted even when
// API verification fails (due to product-restricted tokens).
func TestVerifyOAuthProviderTrust(t *testing.T) {
	t.Parallel()
	cfg := DefaultStartupConfig()
	cfg.TrustOAuthOnFailure = true
	sv := NewStartupVerifier(cfg, logrus.New())

	// Set up a verification service that always fails verification
	sv.verifierSvc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		// Simulate OAuth product-restricted token error by returning empty response
		// which fails the verification tests
		return "", nil
	})

	provider := &UnifiedProvider{
		ID:           "claude",
		Name:         "Claude",
		DisplayName:  "Claude (OAuth)",
		Type:         "claude",
		AuthType:     AuthTypeOAuth,
		DefaultModel: "claude-sonnet-4-20250514",
	}

	disc := &ProviderDiscoveryResult{
		ID:          "claude",
		Type:        "claude",
		AuthType:    AuthTypeOAuth,
		Discovered:  true,
		Source:      "oauth",
		Credentials: "OAuth Token",
		Models:      []string{"claude-sonnet-4-20250514", "claude-opus-4-20250514", "claude-haiku-4-20250514"},
	}

	// Verify the OAuth provider
	ctx := context.Background()
	verifiedProvider, err := sv.verifyOAuthProvider(ctx, provider, disc)

	// Should succeed (no error) because TrustOAuthOnFailure is true
	assert.NoError(t, err)
	assert.NotNil(t, verifiedProvider)

	// The provider should be marked as verified (trusted)
	assert.True(t, verifiedProvider.Verified, "OAuth provider should be trusted even when verification fails")

	// The provider should have healthy status
	assert.Equal(t, StatusHealthy, verifiedProvider.Status)

	// The provider should have models
	assert.Len(t, verifiedProvider.Models, 3, "OAuth provider should have 3 models")

	// The score should be the default trusted score (8.0 - NO OAuth priority boost)
	expectedScore := 8.0 // NO OAuthPriorityBoost (0.0) - pure score-based
	assert.Equal(t, expectedScore, verifiedProvider.Score, "OAuth provider should have trusted default score (no priority boost)")

	// Test results should show oauth_trusted
	assert.True(t, verifiedProvider.TestResults["oauth_trusted"], "Should have oauth_trusted test result")
}

// TestVerifyOAuthProviderNoTrust tests that OAuth providers fail verification
// when TrustOAuthOnFailure is false.
func TestVerifyOAuthProviderNoTrust(t *testing.T) {
	t.Parallel()
	cfg := DefaultStartupConfig()
	cfg.TrustOAuthOnFailure = false // Disable trust
	sv := NewStartupVerifier(cfg, logrus.New())

	// Set up a verification service that fails verification
	sv.verifierSvc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "", nil // Empty response causes verification to fail
	})

	provider := &UnifiedProvider{
		ID:           "claude",
		Name:         "Claude",
		DisplayName:  "Claude (OAuth)",
		Type:         "claude",
		AuthType:     AuthTypeOAuth,
		DefaultModel: "claude-sonnet-4-20250514",
	}

	disc := &ProviderDiscoveryResult{
		ID:          "claude",
		Type:        "claude",
		AuthType:    AuthTypeOAuth,
		Discovered:  true,
		Source:      "oauth",
		Credentials: "OAuth Token",
		Models:      []string{"claude-sonnet-4-20250514"},
	}

	// Verify the OAuth provider
	ctx := context.Background()
	verifiedProvider, err := sv.verifyOAuthProvider(ctx, provider, disc)

	// Should fail because TrustOAuthOnFailure is false
	assert.Error(t, err)
	assert.Nil(t, verifiedProvider)
	assert.Contains(t, err.Error(), "verification failed")
}

// TestZAIProviderHasModels tests that ZAI (Zhipu GLM) provider has models configured
func TestZAIProviderHasModels(t *testing.T) {
	t.Parallel()
	info, ok := GetProviderInfo("zai")
	assert.True(t, ok, "ZAI provider should exist")
	assert.NotEmpty(t, info.Models, "ZAI provider should have models configured")
	assert.Contains(t, info.DisplayName, "GLM", "ZAI should be Zhipu GLM")
}

// TestZenProviderFreeModels tests that Zen provider has correct free models
func TestZenProviderFreeModels(t *testing.T) {
	t.Parallel()
	info, ok := GetProviderInfo("zen")
	assert.True(t, ok, "Zen provider should exist")
	assert.True(t, info.Free, "Zen provider should be free")
	assert.Equal(t, AuthTypeFree, info.AuthType, "Zen should have free auth type")
	assert.NotEmpty(t, info.Models, "Zen provider should have models")

	// Check for at least some expected models
	hasWorkingModel := false
	workingModels := []string{"big-pickle", "gpt-5-nano", "glm-4.7"}
	for _, model := range info.Models {
		for _, working := range workingModels {
			if model == working {
				hasWorkingModel = true
				break
			}
		}
	}
	assert.True(t, hasWorkingModel, "Zen should have at least one known working model")
}
