package toolkit

import (
	"context"
	"testing"
)

func TestNewProviderFactoryRegistry(t *testing.T) {
	registry := NewProviderFactoryRegistry()
	if registry == nil {
		t.Fatal("NewProviderFactoryRegistry returned nil")
	}
	if registry.factories == nil {
		t.Fatal("factories map not initialized")
	}
}

func TestProviderFactoryRegistry_Register(t *testing.T) {
	registry := NewProviderFactoryRegistry()

	// Mock factory function
	factory := func(config map[string]interface{}) (Provider, error) {
		return nil, nil
	}

	err := registry.Register("test-provider", factory)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Check if registered
	if _, exists := registry.factories["test-provider"]; !exists {
		t.Fatal("Factory not registered")
	}
}

func TestProviderFactoryRegistry_Create(t *testing.T) {
	registry := NewProviderFactoryRegistry()

	// Mock provider
	mockProvider := &mockProvider{}

	// Mock factory function
	factory := func(config map[string]interface{}) (Provider, error) {
		return mockProvider, nil
	}

	// Register factory
	registry.Register("test-provider", factory)

	// Test successful creation
	provider, err := registry.Create("test-provider", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if provider == nil {
		t.Fatal("Created provider is nil")
	}

	// Test unregistered provider
	_, err = registry.Create("non-existent", map[string]interface{}{})
	if err == nil {
		t.Fatal("Expected error for non-existent provider")
	}
	expectedErr := "provider non-existent not registered"
	if err.Error() != expectedErr {
		t.Fatalf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestProviderFactoryRegistry_ListProviders(t *testing.T) {
	registry := NewProviderFactoryRegistry()

	// Initially empty
	providers := registry.ListProviders()
	if len(providers) != 0 {
		t.Fatalf("Expected 0 providers, got %d", len(providers))
	}

	// Mock factory
	factory := func(config map[string]interface{}) (Provider, error) {
		return nil, nil
	}

	// Register multiple providers
	registry.Register("provider1", factory)
	registry.Register("provider2", factory)

	providers = registry.ListProviders()
	if len(providers) != 2 {
		t.Fatalf("Expected 2 providers, got %d", len(providers))
	}

	// Check if both are present
	found := make(map[string]bool)
	for _, p := range providers {
		found[p] = true
	}
	if !found["provider1"] || !found["provider2"] {
		t.Fatal("Not all providers listed")
	}
}

// mockProvider implements Provider interface for testing
type mockProvider struct{}

func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, nil
}

func (m *mockProvider) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	return EmbeddingResponse{}, nil
}

func (m *mockProvider) Rerank(ctx context.Context, req RerankRequest) (RerankResponse, error) {
	return RerankResponse{}, nil
}

func (m *mockProvider) DiscoverModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{}, nil
}

func (m *mockProvider) ValidateConfig(config map[string]interface{}) error {
	return nil
}
