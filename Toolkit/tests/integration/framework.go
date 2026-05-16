package integration

import (
	"context"
	"testing"
	"time"

	"github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit"
)

// IntegrationTestSuite provides a framework for integration testing
type IntegrationTestSuite struct {
	providers map[string]toolkit.Provider
	registry  *toolkit.ProviderFactoryRegistry
}

// NewIntegrationTestSuite creates a new integration test suite
func NewIntegrationTestSuite() *IntegrationTestSuite {
	return &IntegrationTestSuite{
		providers: make(map[string]toolkit.Provider),
		registry:  toolkit.NewProviderFactoryRegistry(),
	}
}

// SetupSuite initializes the test environment
func (s *IntegrationTestSuite) SetupSuite() error {
	// Register all available providers
	// Note: In a real implementation, this would dynamically discover providers

	// For now, we'll test with mock providers from the testing package
	return nil
}

// RegisterProvider registers a provider for testing
func (s *IntegrationTestSuite) RegisterProvider(name string, provider toolkit.Provider) {
	s.providers[name] = provider
}

// GetProvider returns a registered provider
func (s *IntegrationTestSuite) GetProvider(name string) (toolkit.Provider, bool) {
	provider, exists := s.providers[name]
	return provider, exists
}

// TestProviderLifecycle tests the complete lifecycle of a provider
func (s *IntegrationTestSuite) TestProviderLifecycle(t *testing.T, providerName string) {
	provider, exists := s.GetProvider(providerName)
	if !exists {
		t.Fatalf("Provider %s not registered", providerName)
	}

	// Test provider name
	if provider.Name() != providerName {
		t.Errorf("Expected provider name %s, got %s", providerName, provider.Name())
	}

	// Test config validation
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	err := provider.ValidateConfig(config)
	if err != nil {
		t.Errorf("Config validation failed: %v", err)
	}

	// Test model discovery
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := provider.DiscoverModels(ctx)
	if err != nil {
		t.Errorf("Model discovery failed: %v", err)
	}

	if len(models) == 0 {
		t.Logf("Warning: No models discovered for provider %s", providerName)
	}

	// Test basic chat functionality (if models available)
	if len(models) > 0 {
		model := models[0]
		if model.Capabilities.SupportsChat {
			chatReq := toolkit.ChatRequest{
				Model: model.ID,
				Messages: []toolkit.ChatMessage{
					{Role: "user", Content: "Hello, integration test!"},
				},
				MaxTokens: 50,
			}

			_, err := provider.Chat(ctx, chatReq)
			// Note: This might fail in integration tests without real API keys
			// We just test that the method exists and can be called
			t.Logf("Chat test for provider %s: %v", providerName, err)
		}
	}
}

// TestProviderCompatibility tests that providers implement the interface correctly
func (s *IntegrationTestSuite) TestProviderCompatibility(t *testing.T, provider toolkit.Provider) {
	// Test that all interface methods are implemented
	_ = provider.Name()

	config := map[string]interface{}{}
	_ = provider.ValidateConfig(config)

	ctx := context.Background()
	_, _ = provider.DiscoverModels(ctx)

	// Test method signatures
	chatReq := toolkit.ChatRequest{}
	_, _ = provider.Chat(ctx, chatReq)

	embedReq := toolkit.EmbeddingRequest{}
	_, _ = provider.Embed(ctx, embedReq)

	rerankReq := toolkit.RerankRequest{}
	_, _ = provider.Rerank(ctx, rerankReq)
}

// TestCrossProviderConsistency tests that different providers behave consistently
func (s *IntegrationTestSuite) TestCrossProviderConsistency(t *testing.T) {
	providers := []string{"chutes", "siliconflow"}

	for _, providerName := range providers {
		provider, exists := s.GetProvider(providerName)
		if !exists {
			t.Logf("Provider %s not available for testing", providerName)
			continue
		}

		// Test that all providers can handle basic operations
		ctx := context.Background()

		// All providers should be able to validate basic config
		config := map[string]interface{}{
			"api_key": "test-key",
		}

		err := provider.ValidateConfig(config)
		if err != nil {
			t.Errorf("Provider %s config validation failed: %v", providerName, err)
		}

		// All providers should be able to discover models (may return empty list)
		_, err = provider.DiscoverModels(ctx)
		if err != nil {
			t.Errorf("Provider %s model discovery failed: %v", providerName, err)
		}
	}
}

// TestErrorHandling tests error handling across providers
func (s *IntegrationTestSuite) TestErrorHandling(t *testing.T, provider toolkit.Provider) {
	ctx := context.Background()

	// Test with invalid config
	invalidConfig := map[string]interface{}{}
	err := provider.ValidateConfig(invalidConfig)
	if err == nil {
		t.Logf("Warning: Provider %s accepted invalid config", provider.Name())
	}

	// Test with invalid API key
	invalidChatReq := toolkit.ChatRequest{
		Model: "invalid-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "test"},
		},
	}

	_, err = provider.Chat(ctx, invalidChatReq)
	// Note: This should fail, but we're testing that it fails gracefully
	t.Logf("Error handling test for provider %s: %v", provider.Name(), err)
}

// CleanupSuite cleans up the test environment
func (s *IntegrationTestSuite) CleanupSuite() {
	// Clean up resources
	s.providers = nil
	s.registry = nil
}
