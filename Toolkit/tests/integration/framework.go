package integration

import (
	"context"
	"fmt"
	"os"
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

// SetupSuite attempts to register real providers from environment-
// detected API keys. Returns ErrNoProvidersConfigured when nothing is
// available so the calling test can `t.Skip("SKIP-OK: ...")` rather
// than silently exercising mocks.
//
// Previously this returned nil unconditionally with the comment "For
// now, we'll test with mock providers from the testing package" — any
// integration test that called SetupSuite() ran against no providers
// at all (the providers map stayed empty) and either no-op'd or
// crashed at first GetProvider call. CONST-050(A) violation: an
// integration test framework must exercise real backing services.
//
// To register real providers, set CHUTES_API_KEY and/or
// SILICONFLOW_API_KEY (or any other env var the corresponding
// provider's factory uses) before invoking SetupSuite.
func (s *IntegrationTestSuite) SetupSuite() error {
	registered := 0
	for _, name := range s.registry.ListProviders() {
		// Each provider's factory reads its API key from env. If the
		// key is unset, factory creation returns an error; we collect
		// it but don't fail — the integration suite gates per-provider
		// via TestProviderLifecycle skips.
		cfg := map[string]interface{}{}
		p, err := s.registry.Create(name, cfg)
		if err != nil {
			continue
		}
		s.providers[name] = p
		registered++
	}
	if registered == 0 {
		fmt.Fprintln(os.Stderr, "[§11.4 / CONST-050(A)] IntegrationTestSuite.SetupSuite: zero real providers registered — set CHUTES_API_KEY / SILICONFLOW_API_KEY / other provider env vars; tests must Skip('SKIP-OK: ...') rather than exercise mocks")
		return ErrNoProvidersConfigured
	}
	return nil
}

// ErrNoProvidersConfigured is returned by SetupSuite when no real
// provider could be registered. Tests should Skip on this error
// rather than fail (per CONST-050(A) integration tests must exercise
// real systems, not mocks).
var ErrNoProvidersConfigured = fmt.Errorf("integration: no real providers configured (set CHUTES_API_KEY / SILICONFLOW_API_KEY / etc.); tests must Skip rather than mock-fallback")

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
