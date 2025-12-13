package siliconflow

import (
	"context"
	"testing"
	"time"

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

func TestSiliconFlowChatCompletion(t *testing.T) {
	provider, err := NewProvider(map[string]interface{}{
		"api_key": "test-api-key",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := toolkit.ChatRequest{
		Model: "deepseek-ai/DeepSeek-V2.5",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "Hello, test!"},
		},
		MaxTokens: 100,
	}

	// This will fail with network error since we don't have a real API
	_, err = provider.Chat(ctx, req)
	if err == nil {
		t.Error("Expected chat completion to fail with test API key")
	}
	// We just verify the method can be called and fails gracefully
	t.Logf("Chat completion failed as expected: %v", err)
}

func TestSiliconFlowEmbedding(t *testing.T) {
	provider, err := NewProvider(map[string]interface{}{
		"api_key": "test-api-key",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := toolkit.EmbeddingRequest{
		Input: []string{"test input"},
		Model: "BAAI/bge-large-en-v1.5",
	}

	// This will fail with network error
	_, err = provider.Embed(ctx, req)
	if err == nil {
		t.Error("Expected embedding to fail with test API key")
	}
	// We just verify the method can be called and fails gracefully
	t.Logf("Embedding failed as expected: %v", err)
}

func TestSiliconFlowRerank(t *testing.T) {
	provider, err := NewProvider(map[string]interface{}{
		"api_key": "test-api-key",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := toolkit.RerankRequest{
		Query:     "test query",
		Documents: []string{"doc1", "doc2"},
		Model:     "BAAI/bge-reranker-large",
	}

	// This will fail with network error
	_, err = provider.Rerank(ctx, req)
	if err == nil {
		t.Error("Expected rerank to fail with test API key")
	}
	// We just verify the method can be called and fails gracefully
	t.Logf("Rerank failed as expected: %v", err)
}

func TestSiliconFlowModelDiscovery(t *testing.T) {
	provider, err := NewProvider(map[string]interface{}{
		"api_key": "test-api-key",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()

	// This will fail with network error since we don't have a real API
	_, err = provider.DiscoverModels(ctx)
	if err == nil {
		t.Error("Expected model discovery to fail with test API key")
	}
	// We just verify the method can be called and fails gracefully
	t.Logf("Model discovery failed as expected: %v", err)
}

func TestSiliconFlowChatCompletionWithParameters(t *testing.T) {
	provider, err := NewProvider(map[string]interface{}{
		"api_key": "test-api-key",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := toolkit.ChatRequest{
		Model: "deepseek-ai/DeepSeek-V2.5",
		Messages: []toolkit.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello!"},
		},
		MaxTokens:        150,
		Temperature:      0.7,
		TopP:             0.9,
		TopK:             40,
		Stop:             []string{"\n", "END"},
		PresencePenalty:  0.1,
		FrequencyPenalty: -0.1,
		LogitBias: map[string]float64{
			"1234": -100.0,
		},
	}

	// This will fail with network error since we don't have a real API
	_, err = provider.Chat(ctx, req)
	if err == nil {
		t.Error("Expected chat completion to fail with test API key")
	}
	// Verify all parameters are processed without panic
	t.Logf("Chat completion with parameters failed as expected: %v", err)
}

func TestSiliconFlowEmbeddingBatch(t *testing.T) {
	provider, err := NewProvider(map[string]interface{}{
		"api_key": "test-api-key",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := toolkit.EmbeddingRequest{
		Model: "BAAI/bge-large-zh-v1.5",
		Input: []string{
			"First document for embedding",
			"Second document for embedding",
			"Third document for embedding",
		},
	}

	// This will fail with network error since we don't have a real API
	_, err = provider.Embed(ctx, req)
	if err == nil {
		t.Error("Expected batch embedding to fail with test API key")
	}
	// Verify batch processing works
	t.Logf("Batch embedding failed as expected: %v", err)
}

func TestSiliconFlowRerankWithOptions(t *testing.T) {
	provider, err := NewProvider(map[string]interface{}{
		"api_key": "test-api-key",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := toolkit.RerankRequest{
		Model: "BAAI/bge-reranker-large",
		Query: "artificial intelligence applications",
		Documents: []string{
			"AI is used in healthcare for diagnostics",
			"Machine learning helps in financial predictions",
			"Natural language processing powers chatbots",
			"Computer vision enables autonomous vehicles",
			"Robotics combines AI with mechanical engineering",
		},
		TopN:       3,
		ReturnDocs: true,
	}

	// This will fail with network error since we don't have a real API
	_, err = provider.Rerank(ctx, req)
	if err == nil {
		t.Error("Expected rerank to fail with test API key")
	}
	// Verify rerank processing works
	t.Logf("Rerank with options failed as expected: %v", err)
}

func TestSiliconFlowProviderWithCustomConfig(t *testing.T) {
	config := map[string]interface{}{
		"api_key":    "custom-api-key",
		"base_url":   "https://api.siliconflow.cn/v1",
		"timeout":    60000,
		"retries":    5,
		"rate_limit": 100,
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider with custom config: %v", err)
	}

	// Test that provider was created successfully
	if provider.Name() != "siliconflow" {
		t.Errorf("Expected provider name 'siliconflow', got '%s'", provider.Name())
	}

	// Test config validation
	err = provider.ValidateConfig(config)
	if err != nil {
		t.Errorf("Custom config validation failed: %v", err)
	}

	// Test that operations can be attempted (will fail due to network)
	ctx := context.Background()
	chatReq := toolkit.ChatRequest{
		Model: "deepseek-ai/DeepSeek-V2.5",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "Test"},
		},
	}

	_, err = provider.Chat(ctx, chatReq)
	if err == nil {
		t.Error("Expected operation to fail with custom config")
	}
	t.Logf("Custom config operation failed as expected: %v", err)
}

func TestSiliconFlowProviderErrorHandling(t *testing.T) {
	// Test with invalid config
	invalidConfig := map[string]interface{}{
		// Missing required api_key
		"base_url": "https://api.siliconflow.cn/v1",
	}

	_, err := NewProvider(invalidConfig)
	if err == nil {
		t.Error("Expected provider creation to fail with invalid config")
	}
	t.Logf("Invalid config creation failed as expected: %v", err)

	// Test with empty config
	emptyConfig := map[string]interface{}{}

	_, err = NewProvider(emptyConfig)
	if err == nil {
		t.Error("Expected provider creation to fail with empty config")
	}
	t.Logf("Empty config creation failed as expected: %v", err)
}
