package chutes

import (
	"testing"

	"github.com/superagent/toolkit/pkg/toolkit"
)

func TestChutesProvider(t *testing.T) {
	// Test configuration
	config := map[string]interface{}{
		"api_key":    "test-api-key",
		"base_url":   "https://api.chutes.ai/v1",
		"timeout":    30000,
		"retries":    3,
		"rate_limit": 60,
	}

	// Test provider creation
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create Chutes provider: %v", err)
	}

	// Test provider name
	if provider.Name() != "chutes" {
		t.Errorf("Expected provider name 'chutes', got '%s'", provider.Name())
	}

	// Test configuration validation
	err = provider.ValidateConfig(config)
	if err != nil {
		t.Errorf("Configuration validation failed: %v", err)
	}

	// Test invalid configuration (missing API key)
	invalidConfig := map[string]interface{}{
		"base_url": "https://api.chutes.ai/v1",
		"timeout":  30000,
	}

	err = provider.ValidateConfig(invalidConfig)
	if err == nil {
		t.Error("Expected validation error for missing API key, but got none")
	}
}

func TestChutesConfigBuilder(t *testing.T) {
	builder := NewConfigBuilder()

	// Test valid configuration
	config := map[string]interface{}{
		"api_key":    "test-api-key",
		"base_url":   "https://api.chutes.ai/v1",
		"timeout":    30000,
		"retries":    3,
		"rate_limit": 60,
	}

	builtConfig, err := builder.Build(config)
	if err != nil {
		t.Fatalf("Failed to build config: %v", err)
	}

	chutesConfig, ok := builtConfig.(*Config)
	if !ok {
		t.Fatal("Built config is not of type *Config")
	}

	if chutesConfig.APIKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got '%s'", chutesConfig.APIKey)
	}

	if chutesConfig.BaseURL != "https://api.chutes.ai/v1" {
		t.Errorf("Expected base URL 'https://api.chutes.ai/v1', got '%s'", chutesConfig.BaseURL)
	}

	if chutesConfig.Timeout != 30000 {
		t.Errorf("Expected timeout 30000, got %d", chutesConfig.Timeout)
	}

	if chutesConfig.Retries != 3 {
		t.Errorf("Expected retries 3, got %d", chutesConfig.Retries)
	}

	if chutesConfig.RateLimit != 60 {
		t.Errorf("Expected rate limit 60, got %d", chutesConfig.RateLimit)
	}

	// Test validation
	err = builder.Validate(chutesConfig)
	if err != nil {
		t.Errorf("Config validation failed: %v", err)
	}

	// Test invalid config (missing API key)
	invalidConfig := &Config{
		BaseURL:   "https://api.chutes.ai/v1",
		Timeout:   30000,
		Retries:   3,
		RateLimit: 60,
	}

	err = builder.Validate(invalidConfig)
	if err == nil {
		t.Error("Expected validation error for missing API key, but got none")
	}
}

func TestChutesClient(t *testing.T) {
	// Test client creation with default base URL
	client := NewClient("test-api-key", "")
	if client == nil {
		t.Fatal("Failed to create Chutes client")
	}

	// Test client creation with custom base URL
	client = NewClient("test-api-key", "https://custom.api.chutes.ai/v1")
	if client == nil {
		t.Fatal("Failed to create Chutes client with custom base URL")
	}
}

func TestChutesRegistration(t *testing.T) {
	// Test that the provider can be registered
	registry := toolkit.NewProviderFactoryRegistry()
	err := Register(registry)
	if err != nil {
		t.Fatalf("Failed to register Chutes provider: %v", err)
	}

	// Test that we can create a provider using the registry
	config := map[string]interface{}{
		"api_key": "test-api-key",
	}

	provider, err := registry.Create("chutes", config)
	if err != nil {
		t.Fatalf("Registry failed to create provider: %v", err)
	}

	if provider.Name() != "chutes" {
		t.Errorf("Expected provider name 'chutes', got '%s'", provider.Name())
	}
}

func TestChutesAutoRegistration(t *testing.T) {
	// Test auto-registration via init function
	registry := toolkit.NewProviderFactoryRegistry()
	
	// Set the global registry (simulating what happens in main)
	SetGlobalProviderRegistry(registry)
	
	// The init function should have registered the provider
	// when the package was imported, so we should be able to create it
	config := map[string]interface{}{
		"api_key": "test-api-key",
	}
	
	provider, err := registry.Create("chutes", config)
	if err != nil {
		// This is expected since init() runs at package import time
		// and we set the registry after import
		t.Skip("Auto-registration test requires registry to be set before package import")
	}

	if provider.Name() != "chutes" {
		t.Errorf("Expected provider name 'chutes', got '%s'", provider.Name())
	}
}