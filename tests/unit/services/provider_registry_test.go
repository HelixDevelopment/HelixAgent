package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

// mockLLMProvider implements llm.LLMProvider for testing
type mockLLMProvider struct {
	name         string
	shouldFail   bool
	healthError  error
	capabilities *models.ProviderCapabilities
}

func (m *mockLLMProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.shouldFail {
		return nil, assert.AnError
	}
	return &models.LLMResponse{
		ID:           "mock-response-" + req.ID,
		RequestID:    req.ID,
		ProviderID:   m.name,
		ProviderName: m.name,
		Content:      "Mock response",
		Confidence:   0.9,
		TokensUsed:   100,
		ResponseTime: 100,
		FinishReason: "stop",
		Metadata:     map[string]any{},
		CreatedAt:    time.Now(),
	}, nil
}

func (m *mockLLMProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	responseChan := make(chan *models.LLMResponse, 1)

	go func() {
		defer close(responseChan)
		if m.shouldFail {
			return
		}
		response := &models.LLMResponse{
			ID:           "mock-stream-" + req.ID,
			RequestID:    req.ID,
			ProviderID:   m.name,
			ProviderName: m.name,
			Content:      "Mock streaming response",
			Confidence:   0.9,
			TokensUsed:   50,
			ResponseTime: 50,
			FinishReason: "stop",
			Metadata:     map[string]any{},
			CreatedAt:    time.Now(),
		}
		responseChan <- response
	}()

	if m.shouldFail {
		return responseChan, assert.AnError
	}
	return responseChan, nil
}

func (m *mockLLMProvider) HealthCheck() error {
	return m.healthError
}

func (m *mockLLMProvider) GetCapabilities() *models.ProviderCapabilities {
	if m.capabilities != nil {
		return m.capabilities
	}
	return &models.ProviderCapabilities{
		SupportedModels:       []string{"mock-model"},
		SupportedFeatures:     []string{"completion"},
		SupportedRequestTypes: []string{"text_completion"},
		Metadata:              map[string]string{"mock": "true"},
	}
}

func (m *mockLLMProvider) ValidateConfig(config map[string]any) (bool, []string) {
	return true, nil
}

func TestProviderRegistry_NewProviderRegistry(t *testing.T) {
	// Test with nil config (should use defaults)
	registry := services.NewProviderRegistry(nil, nil)
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.GetEnsembleService())
	assert.NotNil(t, registry.GetRequestService())

	// Test with custom config
	config := &services.RegistryConfig{
		DefaultTimeout: 60 * time.Second,
		MaxRetries:     5,
		HealthCheck: services.HealthCheckConfig{
			Enabled:          true,
			Interval:         30 * time.Second,
			Timeout:          5 * time.Second,
			FailureThreshold: 2,
		},
		Providers: make(map[string]*services.ProviderConfig),
	}

	registry = services.NewProviderRegistryWithoutAutoDiscovery(config, nil)
	assert.NotNil(t, registry)
}

func TestProviderRegistry_RegisterProvider(t *testing.T) {
	registry := services.NewProviderRegistry(nil, nil)

	// Create mock provider
	mockProvider := &mockLLMProvider{
		name: "test-provider",
	}

	// Register provider
	err := registry.RegisterProvider("test-provider", mockProvider)
	assert.NoError(t, err)

	// Try to register same provider again (should fail)
	err = registry.RegisterProvider("test-provider", mockProvider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Verify provider is in list
	providers := registry.ListProviders()
	assert.Contains(t, providers, "test-provider")
}

func TestProviderRegistry_GetProvider(t *testing.T) {
	registry := services.NewProviderRegistry(nil, nil)

	// Create mock provider
	mockProvider := &mockLLMProvider{
		name: "test-provider",
	}

	// Register provider
	err := registry.RegisterProvider("test-provider", mockProvider)
	require.NoError(t, err)

	// Get provider (may be wrapped in circuit breaker)
	provider, err := registry.GetProvider("test-provider")
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	// Provider may be wrapped in a circuit breaker, so we check it implements the interface
	assert.Implements(t, (*llm.LLMProvider)(nil), provider)

	// Try to get non-existent provider
	provider, err = registry.GetProvider("non-existent")
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "not found")
}

func TestProviderRegistry_UnregisterProvider(t *testing.T) {
	registry := services.NewProviderRegistry(nil, nil)

	// Create mock provider
	mockProvider := &mockLLMProvider{
		name: "test-provider",
	}

	// Register provider
	err := registry.RegisterProvider("test-provider", mockProvider)
	require.NoError(t, err)

	// Verify provider is registered
	providers := registry.ListProviders()
	assert.Contains(t, providers, "test-provider")

	// Unregister provider
	err = registry.UnregisterProvider("test-provider")
	assert.NoError(t, err)

	// Verify provider is removed
	providers = registry.ListProviders()
	assert.NotContains(t, providers, "test-provider")

	// Try to unregister non-existent provider
	err = registry.UnregisterProvider("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProviderRegistry_ListProviders(t *testing.T) {
	// Create config with no default providers
	config := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
		Providers:      make(map[string]*services.ProviderConfig), // Empty providers
	}

	registry := services.NewProviderRegistryWithoutAutoDiscovery(config, nil)

	// Initially should be empty (no API keys set in test environment)
	providers := registry.ListProviders()
	initialCount := len(providers)

	// Register additional providers
	mock1 := &mockLLMProvider{name: "provider-1"}
	mock2 := &mockLLMProvider{name: "provider-2"}
	mock3 := &mockLLMProvider{name: "provider-3"}

	require.NoError(t, registry.RegisterProvider("provider-1", mock1))
	require.NoError(t, registry.RegisterProvider("provider-2", mock2))
	require.NoError(t, registry.RegisterProvider("provider-3", mock3))

	// List providers - should have initial + custom providers
	providers = registry.ListProviders()
	expectedCount := initialCount + 3
	assert.Len(t, providers, expectedCount)

	// Check custom providers are present
	assert.Contains(t, providers, "provider-1")
	assert.Contains(t, providers, "provider-2")
	assert.Contains(t, providers, "provider-3")
}

func TestProviderRegistry_HealthCheck(t *testing.T) {
	// Create config with no default providers
	config := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
		Providers:      make(map[string]*services.ProviderConfig), // Empty providers
	}

	registry := services.NewProviderRegistryWithoutAutoDiscovery(config, nil)

	// Register healthy provider
	healthyProvider := &mockLLMProvider{
		name:        "healthy",
		healthError: nil,
	}

	// Register failing provider
	failingProvider := &mockLLMProvider{
		name:        "failing",
		healthError: assert.AnError,
	}

	require.NoError(t, registry.RegisterProvider("healthy", healthyProvider))
	require.NoError(t, registry.RegisterProvider("failing", failingProvider))

	// Run health check
	results := registry.HealthCheck()

	// Check our custom providers are in results
	assert.Contains(t, results, "healthy")
	assert.Contains(t, results, "failing")

	// Check health status
	assert.NoError(t, results["healthy"])
	assert.Error(t, results["failing"])

	// Debug: print actual results
	t.Logf("Health check results: %v", results)
}

func TestProviderRegistry_ConfigureProvider(t *testing.T) {
	registry := services.NewProviderRegistry(nil, nil)

	// Create mock provider
	mockProvider := &mockLLMProvider{
		name: "test-provider",
	}

	// Register provider
	err := registry.RegisterProvider("test-provider", mockProvider)
	require.NoError(t, err)

	// Configure provider (enable)
	config := &services.ProviderConfig{
		Name:    "test-provider",
		Enabled: true,
	}

	err = registry.ConfigureProvider("test-provider", config)
	assert.NoError(t, err)

	// Configure provider (disable - should unregister)
	config.Enabled = false
	err = registry.ConfigureProvider("test-provider", config)
	assert.NoError(t, err)

	// Verify provider is unregistered
	_, err = registry.GetProvider("test-provider")
	assert.Error(t, err)

	// Try to configure non-existent provider
	err = registry.ConfigureProvider("non-existent", config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProviderRegistry_GetProviderConfig(t *testing.T) {
	registry := services.NewProviderRegistry(nil, nil)

	// Create mock provider
	mockProvider := &mockLLMProvider{
		name: "test-provider",
	}

	// Register provider
	err := registry.RegisterProvider("test-provider", mockProvider)
	require.NoError(t, err)

	// Get provider config
	config, err := registry.GetProviderConfig("test-provider")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "test-provider", config.Name)
	assert.True(t, config.Enabled)

	// Try to get config for non-existent provider
	config, err = registry.GetProviderConfig("non-existent")
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "not found")
}

func TestProviderRegistry_DefaultProviders(t *testing.T) {
	// Test that default providers are registered
	config := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
		Providers: map[string]*services.ProviderConfig{
			"deepseek": {
				Name:    "deepseek",
				Type:    "deepseek",
				Enabled: true,
				Models: []services.ModelConfig{{
					ID:      "deepseek-coder",
					Name:    "DeepSeek Coder",
					Enabled: true,
					Weight:  1.0,
				}},
			},
			"claude": {
				Name:    "claude",
				Type:    "claude",
				Enabled: true,
				Models: []services.ModelConfig{{
					ID:      "claude-3-sonnet-20240229",
					Name:    "Claude 3 Sonnet",
					Enabled: true,
					Weight:  1.0,
				}},
			},
		},
	}

	registry := services.NewProviderRegistryWithoutAutoDiscovery(config, nil)

	// Check that providers are registered
	// Note: In test environment, actual providers won't be created due to missing API keys
	// But the registry should still be initialized
	assert.NotNil(t, registry)
}

func TestLoadRegistryConfigFromAppConfig(t *testing.T) {
	// Test loading config from app config
	// This is a simplified test since we don't have the actual config structure
	config := services.LoadRegistryConfigFromAppConfig(nil)
	assert.NotNil(t, config)
	assert.Equal(t, 30*time.Second, config.DefaultTimeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.NotNil(t, config.Providers)
	assert.NotNil(t, config.Ensemble)
	assert.NotNil(t, config.Routing)
}
