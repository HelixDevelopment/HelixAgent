package siliconflow

import (
	"testing"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
)

func TestNewProvider(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	provider, err := NewProvider(config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if provider == nil {
		t.Error("Expected non-nil provider")
	}

	sfProvider, ok := provider.(*Provider)
	if !ok {
		t.Fatalf("Expected *Provider, got %T", provider)
	}

	if sfProvider.client == nil {
		t.Error("Expected client to be initialized")
	}

	if sfProvider.discovery == nil {
		t.Error("Expected discovery to be initialized")
	}

	if sfProvider.config == nil {
		t.Error("Expected config to be initialized")
	}

	if sfProvider.config.APIKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got %s", sfProvider.config.APIKey)
	}
}

func TestNewProvider_InvalidConfig(t *testing.T) {
	// Test missing API key
	config := map[string]interface{}{}

	_, err := NewProvider(config)

	if err == nil {
		t.Error("Expected error for missing API key")
	}
}

func TestProvider_Name(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	provider, _ := NewProvider(config)

	if provider.Name() != "siliconflow" {
		t.Errorf("Expected name 'siliconflow', got %s", provider.Name())
	}
}

func TestProvider_ValidateConfig(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	provider, _ := NewProvider(config)

	err := provider.ValidateConfig(config)

	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}

	// Test invalid config
	invalidConfig := map[string]interface{}{}
	err = provider.ValidateConfig(invalidConfig)

	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

func TestFactory(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	provider, err := Factory(config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if provider.Name() != "siliconflow" {
		t.Errorf("Expected name 'siliconflow', got %s", provider.Name())
	}
}

func TestRegister(t *testing.T) {
	registry := toolkit.NewProviderFactoryRegistry()

	err := Register(registry)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Registration should succeed without error
}

func TestSetGlobalProviderRegistry(t *testing.T) {
	registry := toolkit.NewProviderFactoryRegistry()

	SetGlobalProviderRegistry(registry)

	if globalProviderRegistry != registry {
		t.Error("Expected global registry to be set")
	}
}

// Note: Chat, Embed, Rerank, and DiscoverModels methods are not easily testable
// without mocking HTTP requests, so they are tested indirectly through integration tests
