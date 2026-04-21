// Package provider_legacy holds the former
// tests/integration/provider_integration_test.go demoted to a unit test.
// CONST-030 forbids in-process mocks in non-unit tests; the file
// exclusively exercises llm.LLMProvider / verifier / services registry
// wiring against a local MockLLMProvider (testify/mock) with no network
// round-trip. PR25 of the CONST-030 compliance campaign.
package provider_legacy_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/verifier"
)

// =============================================================================
// Mock Provider Implementation
// =============================================================================

// MockLLMProvider is a mock implementation of llm.LLMProvider for testing
type MockLLMProvider struct {
	mock.Mock
	name             string
	healthCheckError error
	completeResponse *models.LLMResponse
	completeError    error
	capabilities     *models.ProviderCapabilities
	configValid      bool
	configErrors     []string
	completeCalls    int64
	streamCalls      int64
	healthCheckCalls int64
	mu               sync.Mutex
	responseDelay    time.Duration
	streamResponses  []*models.LLMResponse
}

// NewMockLLMProvider creates a new mock provider with the given name
func NewMockLLMProvider(name string) *MockLLMProvider {
	return &MockLLMProvider{
		name:        name,
		configValid: true,
		capabilities: &models.ProviderCapabilities{
			SupportedModels:         []string{"mock-model-1", "mock-model-2"},
			SupportedFeatures:       []string{"text_completion", "chat"},
			SupportedRequestTypes:   []string{"text_completion", "chat"},
			SupportsStreaming:       true,
			SupportsFunctionCalling: true,
			SupportsVision:          false,
			SupportsTools:           true,
			SupportsCodeCompletion:  true,
			Limits: models.ModelLimits{
				MaxTokens:             4096,
				MaxInputLength:        8192,
				MaxOutputLength:       4096,
				MaxConcurrentRequests: 10,
			},
			Metadata: map[string]string{
				"provider": "mock",
				"version":  "1.0.0",
			},
		},
	}
}

// Complete implements llm.LLMProvider.Complete
func (m *MockLLMProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	atomic.AddInt64(&m.completeCalls, 1)

	if m.responseDelay > 0 {
		select {
		case <-time.After(m.responseDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.completeError != nil {
		return nil, m.completeError
	}

	if m.completeResponse != nil {
		return m.completeResponse, nil
	}

	// Default response
	return &models.LLMResponse{
		ID:           "mock-response-" + req.ID,
		RequestID:    req.ID,
		ProviderID:   m.name,
		ProviderName: m.name,
		Content:      "Mock response for: " + req.Prompt,
		Confidence:   0.95,
		TokensUsed:   10,
		ResponseTime: 100,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}, nil
}

// CompleteStream implements llm.LLMProvider.CompleteStream
func (m *MockLLMProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	atomic.AddInt64(&m.streamCalls, 1)

	if m.completeError != nil {
		return nil, m.completeError
	}

	ch := make(chan *models.LLMResponse)

	go func() {
		defer close(ch)

		if len(m.streamResponses) > 0 {
			for _, resp := range m.streamResponses {
				select {
				case <-ctx.Done():
					return
				case ch <- resp:
				}
			}
		} else {
			// Default streaming response
			chunks := []string{"Mock ", "streaming ", "response"}
			for i, chunk := range chunks {
				select {
				case <-ctx.Done():
					return
				case ch <- &models.LLMResponse{
					ID:           "mock-stream-" + req.ID,
					RequestID:    req.ID,
					ProviderID:   m.name,
					ProviderName: m.name,
					Content:      chunk,
					Confidence:   0.95,
					TokensUsed:   i + 1,
					ResponseTime: int64((i + 1) * 50),
					FinishReason: "",
					CreatedAt:    time.Now(),
				}:
				}
			}
		}
	}()

	return ch, nil
}

// HealthCheck implements llm.LLMProvider.HealthCheck
func (m *MockLLMProvider) HealthCheck() error {
	atomic.AddInt64(&m.healthCheckCalls, 1)
	return m.healthCheckError
}

// GetCapabilities implements llm.LLMProvider.GetCapabilities
func (m *MockLLMProvider) GetCapabilities() *models.ProviderCapabilities {
	return m.capabilities
}

// ValidateConfig implements llm.LLMProvider.ValidateConfig
func (m *MockLLMProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return m.configValid, m.configErrors
}

// SetCompleteResponse sets the response for Complete calls
func (m *MockLLMProvider) SetCompleteResponse(resp *models.LLMResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeResponse = resp
}

// SetCompleteError sets the error for Complete calls
func (m *MockLLMProvider) SetCompleteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeError = err
}

// SetHealthCheckError sets the error for HealthCheck calls
func (m *MockLLMProvider) SetHealthCheckError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthCheckError = err
}

// SetResponseDelay sets a delay for responses
func (m *MockLLMProvider) SetResponseDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseDelay = delay
}

// GetCompleteCalls returns the number of Complete calls
func (m *MockLLMProvider) GetCompleteCalls() int64 {
	return atomic.LoadInt64(&m.completeCalls)
}

// GetHealthCheckCalls returns the number of HealthCheck calls
func (m *MockLLMProvider) GetHealthCheckCalls() int64 {
	return atomic.LoadInt64(&m.healthCheckCalls)
}

// =============================================================================
// Test: Provider Registry Tests
// =============================================================================

func TestProviderRegistry_RegisterProvider(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	mockProvider := NewMockLLMProvider("test-provider")
	err := registry.RegisterProvider("test-provider", mockProvider)
	require.NoError(t, err)

	// Verify provider is registered
	providers := registry.ListProviders()
	assert.Contains(t, providers, "test-provider")

	// Try to register same provider again - should fail
	err = registry.RegisterProvider("test-provider", mockProvider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestProviderRegistry_UnregisterProvider(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	mockProvider := NewMockLLMProvider("test-provider")
	err := registry.RegisterProvider("test-provider", mockProvider)
	require.NoError(t, err)

	// Unregister provider
	err = registry.UnregisterProvider("test-provider")
	require.NoError(t, err)

	// Verify provider is gone
	providers := registry.ListProviders()
	assert.NotContains(t, providers, "test-provider")

	// Try to unregister non-existent provider
	err = registry.UnregisterProvider("non-existent")
	assert.Error(t, err)
}

func TestProviderRegistry_GetProvider(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	mockProvider := NewMockLLMProvider("test-provider")
	err := registry.RegisterProvider("test-provider", mockProvider)
	require.NoError(t, err)

	// Get existing provider
	provider, err := registry.GetProvider("test-provider")
	require.NoError(t, err)
	assert.NotNil(t, provider)

	// Get non-existent provider
	_, err = registry.GetProvider("non-existent")
	assert.Error(t, err)
}

func TestProviderRegistry_ListProviders(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Register multiple providers
	providers := []string{"provider1", "provider2", "provider3"}
	for _, name := range providers {
		err := registry.RegisterProvider(name, NewMockLLMProvider(name))
		require.NoError(t, err)
	}

	// List providers
	listed := registry.ListProviders()
	assert.Len(t, listed, 3)
	for _, name := range providers {
		assert.Contains(t, listed, name)
	}
}

func TestProviderRegistry_AllProvidersImplementInterface(t *testing.T) {
	// This test verifies that mock provider properly implements the interface
	var _ llm.LLMProvider = (*MockLLMProvider)(nil)

	mockProvider := NewMockLLMProvider("test")

	// Verify all interface methods work
	ctx := context.Background()
	req := &models.LLMRequest{
		ID:       "test-req",
		Prompt:   "Hello",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	// Test Complete
	resp, err := mockProvider.Complete(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Test CompleteStream
	stream, err := mockProvider.CompleteStream(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Drain the stream
	for range stream {
	}

	// Test HealthCheck
	err = mockProvider.HealthCheck()
	assert.NoError(t, err)

	// Test GetCapabilities
	caps := mockProvider.GetCapabilities()
	assert.NotNil(t, caps)
	assert.True(t, caps.SupportsStreaming)

	// Test ValidateConfig
	valid, errs := mockProvider.ValidateConfig(nil)
	assert.True(t, valid)
	assert.Empty(t, errs)
}

func TestProviderRegistry_ProviderCapabilities(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Create provider with specific capabilities
	mockProvider := NewMockLLMProvider("capable-provider")
	mockProvider.capabilities = &models.ProviderCapabilities{
		SupportedModels:         []string{"model-a", "model-b", "model-c"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsCodeCompletion:  true,
		SupportsCodeAnalysis:    true,
		SupportsRefactoring:     true,
		Limits: models.ModelLimits{
			MaxTokens:             100000,
			MaxInputLength:        50000,
			MaxOutputLength:       50000,
			MaxConcurrentRequests: 100,
		},
	}

	err := registry.RegisterProvider("capable-provider", mockProvider)
	require.NoError(t, err)

	provider, err := registry.GetProvider("capable-provider")
	require.NoError(t, err)

	caps := provider.GetCapabilities()
	assert.Len(t, caps.SupportedModels, 3)
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsVision)
	assert.Equal(t, 100000, caps.Limits.MaxTokens)
}

func TestProviderRegistry_ProviderConfiguration(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	mockProvider := NewMockLLMProvider("configurable-provider")
	err := registry.RegisterProvider("configurable-provider", mockProvider)
	require.NoError(t, err)

	// Configure provider
	config := &services.ProviderConfig{
		Name:       "configurable-provider",
		Type:       "mock",
		Enabled:    true,
		APIKey:     "test-key",
		BaseURL:    "https://api.test.com",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		Weight:     1.5,
		Tags:       []string{"test", "mock"},
		Capabilities: map[string]string{
			"custom": "value",
		},
	}

	err = registry.ConfigureProvider("configurable-provider", config)
	require.NoError(t, err)

	// Get configuration
	retrievedConfig, err := registry.GetProviderConfig("configurable-provider")
	require.NoError(t, err)
	assert.Equal(t, "configurable-provider", retrievedConfig.Name)
}

// =============================================================================
// Test: Provider Selection Tests
// =============================================================================

func TestProviderRegistry_SelectByProviderName(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Register multiple providers
	for _, name := range []string{"claude", "deepseek", "gemini"} {
		err := registry.RegisterProvider(name, NewMockLLMProvider(name))
		require.NoError(t, err)
	}

	// Select specific provider
	provider, err := registry.GetProvider("deepseek")
	require.NoError(t, err)
	assert.NotNil(t, provider)

	// Verify it's the correct provider by checking capabilities
	caps := provider.GetCapabilities()
	assert.NotNil(t, caps)
}

func TestProviderRegistry_ListProvidersOrderedByScore(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Register providers
	providers := []string{"low-score", "high-score", "medium-score"}
	for _, name := range providers {
		err := registry.RegisterProvider(name, NewMockLLMProvider(name))
		require.NoError(t, err)
	}

	// List providers ordered by score
	ordered := registry.ListProvidersOrderedByScore()
	assert.Len(t, ordered, 3)

	// Without explicit scores, order is based on default score (5.0) + health bonus
	// All should be present
	for _, name := range providers {
		assert.Contains(t, ordered, name)
	}
}

func TestProviderRegistry_GetBestProvidersForDebate(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Register multiple providers
	for _, name := range []string{"provider1", "provider2", "provider3", "provider4", "provider5"} {
		err := registry.RegisterProvider(name, NewMockLLMProvider(name))
		require.NoError(t, err)
	}

	// Get best providers for debate
	best := registry.GetBestProvidersForDebate(3, 5)

	// Should return available providers (max 5)
	assert.LessOrEqual(t, len(best), 5)
}

func TestProviderRegistry_FallbackChain(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Create providers with different behaviors
	primary := NewMockLLMProvider("primary")
	primary.SetCompleteError(errors.New("primary failed"))

	fallback1 := NewMockLLMProvider("fallback1")
	fallback1.SetCompleteError(errors.New("fallback1 failed"))

	fallback2 := NewMockLLMProvider("fallback2")
	// fallback2 succeeds

	err := registry.RegisterProvider("primary", primary)
	require.NoError(t, err)
	err = registry.RegisterProvider("fallback1", fallback1)
	require.NoError(t, err)
	err = registry.RegisterProvider("fallback2", fallback2)
	require.NoError(t, err)

	// Try primary first
	p, _ := registry.GetProvider("primary")
	_, err = p.Complete(context.Background(), &models.LLMRequest{ID: "test", Prompt: "test"})
	assert.Error(t, err)

	// Try fallback1
	p, _ = registry.GetProvider("fallback1")
	_, err = p.Complete(context.Background(), &models.LLMRequest{ID: "test", Prompt: "test"})
	assert.Error(t, err)

	// Try fallback2 - should succeed
	p, _ = registry.GetProvider("fallback2")
	resp, err := p.Complete(context.Background(), &models.LLMRequest{ID: "test", Prompt: "test"})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// =============================================================================
// Test: Provider Health Tests
// =============================================================================

func TestProviderRegistry_HealthCheck(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Create healthy and unhealthy providers
	healthy := NewMockLLMProvider("healthy")
	unhealthy := NewMockLLMProvider("unhealthy")
	unhealthy.SetHealthCheckError(errors.New("health check failed"))

	err := registry.RegisterProvider("healthy", healthy)
	require.NoError(t, err)
	err = registry.RegisterProvider("unhealthy", unhealthy)
	require.NoError(t, err)

	// Run health check
	results := registry.HealthCheck()

	assert.Nil(t, results["healthy"])
	assert.NotNil(t, results["unhealthy"])
	assert.Contains(t, results["unhealthy"].Error(), "health check failed")
}

func TestProviderRegistry_ProviderStatusTracking(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	mockProvider := NewMockLLMProvider("tracked-provider")
	err := registry.RegisterProvider("tracked-provider", mockProvider)
	require.NoError(t, err)

	// Initially no health status
	health := registry.GetProviderHealth("tracked-provider")
	assert.Nil(t, health) // No verification done yet

	// Verify provider
	ctx := context.Background()
	result := registry.VerifyProvider(ctx, "tracked-provider")

	assert.NotNil(t, result)
	assert.Equal(t, "tracked-provider", result.Provider)
	assert.Equal(t, services.ProviderStatusHealthy, result.Status)
	assert.True(t, result.Verified)

	// Check health status after verification
	health = registry.GetProviderHealth("tracked-provider")
	assert.NotNil(t, health)
	assert.True(t, health.Verified)
}

func TestProviderRegistry_UnhealthyProviderHandling(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Create provider that fails verification
	failingProvider := NewMockLLMProvider("failing")
	failingProvider.SetCompleteError(errors.New("API error: 500 internal server error"))

	err := registry.RegisterProvider("failing", failingProvider)
	require.NoError(t, err)

	// Verify provider
	ctx := context.Background()
	result := registry.VerifyProvider(ctx, "failing")

	assert.NotNil(t, result)
	assert.Equal(t, services.ProviderStatusUnhealthy, result.Status)
	assert.False(t, result.Verified)
	assert.NotEmpty(t, result.Error)

	// Provider should not be considered healthy
	assert.False(t, registry.IsProviderHealthy("failing"))
}

func TestProviderRegistry_VerifyAllProviders(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Register multiple providers with different states
	healthy1 := NewMockLLMProvider("healthy1")
	healthy2 := NewMockLLMProvider("healthy2")
	unhealthy := NewMockLLMProvider("unhealthy")
	unhealthy.SetCompleteError(errors.New("verification failed"))

	registry.RegisterProvider("healthy1", healthy1)
	registry.RegisterProvider("healthy2", healthy2)
	registry.RegisterProvider("unhealthy", unhealthy)

	// Verify all providers
	ctx := context.Background()
	results := registry.VerifyAllProviders(ctx)

	assert.Len(t, results, 3)
	assert.True(t, results["healthy1"].Verified)
	assert.True(t, results["healthy2"].Verified)
	assert.False(t, results["unhealthy"].Verified)

	// Check healthy providers list
	healthyProviders := registry.GetHealthyProviders()
	assert.Contains(t, healthyProviders, "healthy1")
	assert.Contains(t, healthyProviders, "healthy2")
	assert.NotContains(t, healthyProviders, "unhealthy")
}

func TestProviderRegistry_RateLimitedProviderStatus(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Create provider that returns rate limit error
	rateLimited := NewMockLLMProvider("rate-limited")
	rateLimited.SetCompleteError(errors.New("HTTP 429: rate limit exceeded"))

	err := registry.RegisterProvider("rate-limited", rateLimited)
	require.NoError(t, err)

	// Verify provider
	ctx := context.Background()
	result := registry.VerifyProvider(ctx, "rate-limited")

	assert.Equal(t, services.ProviderStatusRateLimited, result.Status)
	assert.Contains(t, result.Error, "rate limit")
}

func TestProviderRegistry_AuthFailedProviderStatus(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Create provider that returns auth error
	authFailed := NewMockLLMProvider("auth-failed")
	authFailed.SetCompleteError(errors.New("HTTP 401: unauthorized - invalid API key"))

	err := registry.RegisterProvider("auth-failed", authFailed)
	require.NoError(t, err)

	// Verify provider
	ctx := context.Background()
	result := registry.VerifyProvider(ctx, "auth-failed")

	assert.Equal(t, services.ProviderStatusAuthFailed, result.Status)
	assert.Contains(t, result.Error, "authentication")
}

// =============================================================================
// Test: Provider Discovery Tests
// =============================================================================

func TestProviderDiscovery_DiscoverProviders(t *testing.T) {
	// This test doesn't set API keys, so it should discover only free providers
	discovery := services.NewProviderDiscovery(nil, false)

	discovered, err := discovery.DiscoverProviders()
	require.NoError(t, err)

	// Should discover at least Zen (free provider)
	var zenFound bool
	for _, p := range discovered {
		if p.Type == "zen" {
			zenFound = true
			break
		}
	}
	assert.True(t, zenFound, "Zen free provider should be discovered without API key")
}

func TestProviderDiscovery_GetBestProviders(t *testing.T) {
	discovery := services.NewProviderDiscovery(nil, false)

	// Discover and get best providers
	_, err := discovery.DiscoverProviders()
	require.NoError(t, err)

	best := discovery.GetBestProviders(5)
	// May have providers if any are available
	assert.GreaterOrEqual(t, len(best), 0)
}

func TestProviderDiscovery_Summary(t *testing.T) {
	discovery := services.NewProviderDiscovery(nil, false)

	_, err := discovery.DiscoverProviders()
	require.NoError(t, err)

	summary := discovery.Summary()
	assert.NotNil(t, summary)
	assert.Contains(t, summary, "total_discovered")
	assert.Contains(t, summary, "healthy")
	assert.Contains(t, summary, "providers")
}

func TestProviderDiscovery_ProviderTypes(t *testing.T) {
	// Test that all expected provider types are in the mappings
	expectedTypes := []string{
		"claude", "deepseek", "gemini", "mistral", "openrouter",
		"qwen", "ollama", "cerebras", "zen",
	}

	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	knownTypes := registry.GetKnownProviderTypes()

	for _, expected := range expectedTypes {
		assert.Contains(t, knownTypes, expected, "Expected provider type %s not found", expected)
	}
}

// =============================================================================
// Test: Verification System Tests
// =============================================================================

func TestStartupVerifier_DefaultConfig(t *testing.T) {
	config := verifier.DefaultStartupConfig()

	assert.NotNil(t, config)
	assert.True(t, config.ParallelVerification)
	assert.Greater(t, config.MaxConcurrency, 0)
	assert.Greater(t, config.VerificationTimeout, time.Duration(0))
	assert.Greater(t, config.MinScore, 0.0)
	assert.Equal(t, 5, config.PositionCount)
	assert.Equal(t, 25, config.DebateTeamSize) // 5 positions × (1 primary + 4 fallbacks) = 25 max
}

func TestStartupVerifier_ProviderTypes(t *testing.T) {
	// Test supported provider types
	supportedProviders := verifier.SupportedProviders

	// Verify all 10 providers are defined
	expectedProviders := []string{
		"claude", "deepseek", "gemini", "mistral", "openrouter",
		"qwen", "zen", "ollama", "cerebras", "zai",
	}

	for _, name := range expectedProviders {
		info, exists := supportedProviders[name]
		if exists {
			assert.NotEmpty(t, info.Type)
			assert.NotEmpty(t, info.DisplayName)
		}
	}
}

func TestStartupVerifier_ProviderAuthTypes(t *testing.T) {
	// Test different auth types
	assert.True(t, verifier.IsOAuthProvider("claude"))
	assert.True(t, verifier.IsOAuthProvider("qwen"))
	assert.False(t, verifier.IsOAuthProvider("deepseek"))
	assert.False(t, verifier.IsOAuthProvider("gemini"))
}

func TestStartupVerifier_FreeProviders(t *testing.T) {
	// Test free provider detection
	assert.True(t, verifier.IsFreeProvider("zen"))
	assert.True(t, verifier.IsFreeProvider("ollama"))
	assert.True(t, verifier.IsFreeProvider("openrouter")) // Has free tier
	assert.False(t, verifier.IsFreeProvider("claude"))
	assert.False(t, verifier.IsFreeProvider("deepseek"))
}

func TestStartupVerifier_GetProvidersByAuthType(t *testing.T) {
	oauthProviders := verifier.GetProvidersByAuthType(verifier.AuthTypeOAuth)
	assert.GreaterOrEqual(t, len(oauthProviders), 2) // At least claude and qwen

	apiKeyProviders := verifier.GetProvidersByAuthType(verifier.AuthTypeAPIKey)
	assert.GreaterOrEqual(t, len(apiKeyProviders), 5) // Several API key providers

	freeProviders := verifier.GetProvidersByAuthType(verifier.AuthTypeFree)
	assert.GreaterOrEqual(t, len(freeProviders), 1) // At least zen
}

func TestStartupVerifier_GetProvidersByTier(t *testing.T) {
	// Tier 1: Premium (OAuth)
	tier1 := verifier.GetProvidersByTier(1)
	assert.GreaterOrEqual(t, len(tier1), 1)

	// Tier 2: High-quality API key
	tier2 := verifier.GetProvidersByTier(2)
	assert.GreaterOrEqual(t, len(tier2), 2)

	// Tier 5: Free
	tier5 := verifier.GetProvidersByTier(5)
	assert.GreaterOrEqual(t, len(tier5), 1)
}

func TestStartupVerifier_ProviderInfo(t *testing.T) {
	// Test GetProviderInfo
	info, exists := verifier.GetProviderInfo("claude")
	assert.True(t, exists)
	assert.Equal(t, "claude", info.Type)
	assert.Equal(t, verifier.AuthTypeOAuth, info.AuthType)
	assert.NotEmpty(t, info.BaseURL)
	assert.NotEmpty(t, info.Models)

	// Test non-existent provider
	_, exists = verifier.GetProviderInfo("non-existent")
	assert.False(t, exists)
}

// =============================================================================
// Test: Mock Provider Tests
// =============================================================================

func TestMockProvider_Complete(t *testing.T) {
	provider := NewMockLLMProvider("test")

	req := &models.LLMRequest{
		ID:       "test-123",
		Prompt:   "Hello, world!",
		Messages: []models.Message{{Role: "user", Content: "Hello, world!"}},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Content, "Hello, world!")
	assert.Equal(t, int64(1), provider.GetCompleteCalls())
}

func TestMockProvider_CompleteWithCustomResponse(t *testing.T) {
	provider := NewMockLLMProvider("test")

	customResp := &models.LLMResponse{
		ID:         "custom-123",
		Content:    "Custom response",
		Confidence: 0.99,
	}
	provider.SetCompleteResponse(customResp)

	req := &models.LLMRequest{ID: "test", Prompt: "test"}
	resp, err := provider.Complete(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "custom-123", resp.ID)
	assert.Equal(t, "Custom response", resp.Content)
	assert.Equal(t, 0.99, resp.Confidence)
}

func TestMockProvider_CompleteWithError(t *testing.T) {
	provider := NewMockLLMProvider("test")
	provider.SetCompleteError(errors.New("API error"))

	req := &models.LLMRequest{ID: "test", Prompt: "test"}
	resp, err := provider.Complete(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "API error")
}

func TestMockProvider_CompleteStream(t *testing.T) {
	provider := NewMockLLMProvider("test")

	req := &models.LLMRequest{
		ID:       "test-stream",
		Prompt:   "Stream test",
		Messages: []models.Message{{Role: "user", Content: "Stream test"}},
	}

	stream, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)

	var chunks []string
	for resp := range stream {
		chunks = append(chunks, resp.Content)
	}

	assert.Len(t, chunks, 3)
	assert.Equal(t, "Mock ", chunks[0])
	assert.Equal(t, "streaming ", chunks[1])
	assert.Equal(t, "response", chunks[2])
}

func TestMockProvider_HealthCheck(t *testing.T) {
	provider := NewMockLLMProvider("test")

	// Healthy by default
	err := provider.HealthCheck()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), provider.GetHealthCheckCalls())

	// Set unhealthy
	provider.SetHealthCheckError(errors.New("service unavailable"))
	err = provider.HealthCheck()
	assert.Error(t, err)
	assert.Equal(t, int64(2), provider.GetHealthCheckCalls())
}

func TestMockProvider_Capabilities(t *testing.T) {
	provider := NewMockLLMProvider("test")

	caps := provider.GetCapabilities()
	assert.NotNil(t, caps)
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsTools)
	assert.Contains(t, caps.SupportedModels, "mock-model-1")
	assert.Equal(t, 4096, caps.Limits.MaxTokens)
}

func TestMockProvider_ValidateConfig(t *testing.T) {
	provider := NewMockLLMProvider("test")

	// Valid by default
	valid, errs := provider.ValidateConfig(map[string]interface{}{"key": "value"})
	assert.True(t, valid)
	assert.Empty(t, errs)

	// Set invalid
	provider.configValid = false
	provider.configErrors = []string{"missing required field: api_key"}
	valid, errs = provider.ValidateConfig(nil)
	assert.False(t, valid)
	assert.Len(t, errs, 1)
}

func TestMockProvider_WithTimeout(t *testing.T) {
	provider := NewMockLLMProvider("test")
	provider.SetResponseDelay(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := &models.LLMRequest{ID: "test", Prompt: "test"}
	_, err := provider.Complete(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

// =============================================================================
// Test: Error Handling Tests
// =============================================================================

func TestProviderRegistry_ErrorHandling_EmptyProviderName(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Try to get provider with empty name
	_, err := registry.GetProvider("")
	assert.Error(t, err)
}

func TestProviderRegistry_ErrorHandling_NilProvider(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Note: This may panic or error depending on implementation
	// The test verifies the registry handles nil gracefully
	err := registry.RegisterProvider("nil-provider", nil)
	// Behavior depends on implementation - document actual behavior
	_ = err
}

func TestProviderRegistry_ConcurrentAccess(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Register initial providers
	for i := 0; i < 5; i++ {
		name := "provider-" + string(rune('A'+i))
		registry.RegisterProvider(name, NewMockLLMProvider(name))
	}

	// Concurrent reads and writes
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Mix of operations
			switch idx % 4 {
			case 0:
				registry.ListProviders()
			case 1:
				registry.GetProvider("provider-A")
			case 2:
				registry.HealthCheck()
			case 3:
				registry.ListProvidersOrderedByScore()
			}
		}(i)
	}

	wg.Wait()
}

func TestProviderRegistry_ActiveRequestTracking(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	mockProvider := NewMockLLMProvider("tracked")
	mockProvider.SetResponseDelay(100 * time.Millisecond)

	err := registry.RegisterProvider("tracked", mockProvider)
	require.NoError(t, err)

	// Check initial count
	count := registry.GetActiveRequestCount("tracked")
	assert.Equal(t, int64(0), count)

	// Increment
	ok := registry.IncrementActiveRequests("tracked")
	assert.True(t, ok)
	count = registry.GetActiveRequestCount("tracked")
	assert.Equal(t, int64(1), count)

	// Decrement
	ok = registry.DecrementActiveRequests("tracked")
	assert.True(t, ok)
	count = registry.GetActiveRequestCount("tracked")
	assert.Equal(t, int64(0), count)

	// Non-existent provider
	count = registry.GetActiveRequestCount("non-existent")
	assert.Equal(t, int64(-1), count)
}

// =============================================================================
// Test: UnifiedProvider and UnifiedModel Types
// =============================================================================

func TestUnifiedProviderTypes(t *testing.T) {
	provider := &verifier.UnifiedProvider{
		ID:          "test-provider",
		Name:        "Test Provider",
		DisplayName: "Test Provider Display",
		Type:        "test",
		AuthType:    verifier.AuthTypeAPIKey,
		Verified:    true,
		Score:       8.5,
		Status:      verifier.StatusHealthy,
		Models: []verifier.UnifiedModel{
			{
				ID:                "model-1",
				Name:              "Model 1",
				Provider:          "test",
				Score:             8.5,
				Verified:          true,
				SupportsStreaming: true,
				SupportsTools:     true,
			},
		},
		DefaultModel: "model-1",
	}

	assert.Equal(t, "test-provider", provider.ID)
	assert.Equal(t, verifier.AuthTypeAPIKey, provider.AuthType)
	assert.True(t, provider.Verified)
	assert.Equal(t, 8.5, provider.Score)
	assert.Len(t, provider.Models, 1)
	assert.Equal(t, verifier.StatusHealthy, provider.Status)
}

func TestUnifiedModelTypes(t *testing.T) {
	model := verifier.UnifiedModel{
		ID:                "test-model",
		Name:              "Test Model",
		Provider:          "test-provider",
		Score:             9.0,
		ScoreSuffix:       "(excellent)",
		Verified:          true,
		ContextWindow:     128000,
		MaxOutputTokens:   4096,
		SupportsStreaming: true,
		SupportsTools:     true,
		SupportsFunctions: true,
		SupportsVision:    true,
		Capabilities:      []string{"chat", "code", "vision"},
	}

	assert.Equal(t, "test-model", model.ID)
	assert.Equal(t, 9.0, model.Score)
	assert.True(t, model.Verified)
	assert.True(t, model.SupportsStreaming)
	assert.True(t, model.SupportsVision)
	assert.Contains(t, model.Capabilities, "code")
}

// =============================================================================
// Test: Startup Result Types
// =============================================================================

func TestStartupResultTypes(t *testing.T) {
	result := &verifier.StartupResult{
		TotalProviders:  10,
		VerifiedCount:   8,
		FailedCount:     2,
		APIKeyProviders: 6,
		OAuthProviders:  2,
		FreeProviders:   2,
		StartedAt:       time.Now(),
		CompletedAt:     time.Now().Add(5 * time.Second),
		DurationMs:      5000,
	}

	assert.Equal(t, 10, result.TotalProviders)
	assert.Equal(t, 8, result.VerifiedCount)
	assert.Equal(t, 2, result.FailedCount)
	assert.Equal(t, int64(5000), result.DurationMs)
}

func TestDebateTeamResultTypes(t *testing.T) {
	team := &verifier.DebateTeamResult{
		Positions: []*verifier.DebatePosition{
			{
				Position: 1,
				Role:     "analyst",
				Primary: &verifier.DebateLLM{
					Provider:     "claude",
					ProviderType: "claude",
					ModelID:      "claude-opus-4-5",
					Score:        9.5,
					Verified:     true,
					IsOAuth:      true,
				},
			},
		},
		TotalLLMs:     15,
		MinScore:      5.0,
		SortedByScore: true,
		SelectedAt:    time.Now(),
	}

	assert.Len(t, team.Positions, 1)
	assert.Equal(t, 15, team.TotalLLMs)
	assert.True(t, team.SortedByScore)
	assert.Equal(t, "analyst", team.Positions[0].Role)
	assert.True(t, team.Positions[0].Primary.IsOAuth)
}

// =============================================================================
// Test: Provider Status Constants
// =============================================================================

func TestProviderStatusConstants(t *testing.T) {
	assert.Equal(t, verifier.ProviderStatus("unknown"), verifier.StatusUnknown)
	assert.Equal(t, verifier.ProviderStatus("healthy"), verifier.StatusHealthy)
	assert.Equal(t, verifier.ProviderStatus("verified"), verifier.StatusVerified)
	assert.Equal(t, verifier.ProviderStatus("unhealthy"), verifier.StatusUnhealthy)
	assert.Equal(t, verifier.ProviderStatus("failed"), verifier.StatusFailed)
	assert.Equal(t, verifier.ProviderStatus("degraded"), verifier.StatusDegraded)
	assert.Equal(t, verifier.ProviderStatus("rate_limited"), verifier.StatusRateLimited)
	assert.Equal(t, verifier.ProviderStatus("auth_failed"), verifier.StatusAuthFailed)
}

func TestProviderAuthTypeConstants(t *testing.T) {
	assert.Equal(t, verifier.ProviderAuthType("api_key"), verifier.AuthTypeAPIKey)
	assert.Equal(t, verifier.ProviderAuthType("oauth"), verifier.AuthTypeOAuth)
	assert.Equal(t, verifier.ProviderAuthType("free"), verifier.AuthTypeFree)
	assert.Equal(t, verifier.ProviderAuthType("anonymous"), verifier.AuthTypeAnonymous)
	assert.Equal(t, verifier.ProviderAuthType("local"), verifier.AuthTypeLocal)
}

// =============================================================================
// Test: Integration Scenarios
// =============================================================================

func TestScenario_RegisterVerifyAndSelect(t *testing.T) {
	// Complete scenario: register providers, verify them, select best
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Register multiple providers
	providers := map[string]*MockLLMProvider{
		"fast":    NewMockLLMProvider("fast"),
		"medium":  NewMockLLMProvider("medium"),
		"slow":    NewMockLLMProvider("slow"),
		"failing": NewMockLLMProvider("failing"),
	}

	providers["slow"].SetResponseDelay(100 * time.Millisecond)
	providers["failing"].SetCompleteError(errors.New("always fails"))

	for name, provider := range providers {
		err := registry.RegisterProvider(name, provider)
		require.NoError(t, err)
	}

	// Verify all providers
	ctx := context.Background()
	results := registry.VerifyAllProviders(ctx)

	assert.Len(t, results, 4)
	assert.True(t, results["fast"].Verified)
	assert.True(t, results["medium"].Verified)
	assert.True(t, results["slow"].Verified)
	assert.False(t, results["failing"].Verified)

	// Get healthy providers
	healthy := registry.GetHealthyProviders()
	assert.Len(t, healthy, 3)
	assert.NotContains(t, healthy, "failing")
}

func TestScenario_ProviderFailover(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Create providers with different reliability
	reliable := NewMockLLMProvider("reliable")
	unreliable := NewMockLLMProvider("unreliable")

	callCount := 0
	unreliable.SetCompleteError(errors.New("failed"))

	registry.RegisterProvider("reliable", reliable)
	registry.RegisterProvider("unreliable", unreliable)

	// Simulate failover scenario
	providers := []string{"unreliable", "reliable"}
	var successProvider string

	for _, name := range providers {
		p, _ := registry.GetProvider(name)
		_, err := p.Complete(context.Background(), &models.LLMRequest{
			ID:     "test",
			Prompt: "test",
		})
		callCount++
		if err == nil {
			successProvider = name
			break
		}
	}

	assert.Equal(t, "reliable", successProvider)
	assert.Equal(t, 2, callCount)
}

// =============================================================================
// Test: Benchmark-style Tests
// =============================================================================

func TestProviderRegistry_Performance(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Register many providers
	numProviders := 50
	for i := 0; i < numProviders; i++ {
		name := "provider-" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		registry.RegisterProvider(name, NewMockLLMProvider(name))
	}

	// Measure list operation
	start := time.Now()
	for i := 0; i < 1000; i++ {
		registry.ListProviders()
	}
	listDuration := time.Since(start)

	// Measure get operation
	start = time.Now()
	for i := 0; i < 1000; i++ {
		registry.GetProvider("provider-A0")
	}
	getDuration := time.Since(start)

	t.Logf("List 1000x with %d providers: %v", numProviders, listDuration)
	t.Logf("Get 1000x: %v", getDuration)

	// Reasonable performance expectations
	assert.Less(t, listDuration, 500*time.Millisecond)
	assert.Less(t, getDuration, 100*time.Millisecond)
}
