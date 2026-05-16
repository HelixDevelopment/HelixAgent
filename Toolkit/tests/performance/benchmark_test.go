package performance

import (
	"context"
	"testing"

	testingutils "github.com/HelixDevelopment/helix_agent/Toolkit/Commons/testing"
	"github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit"
)

// BenchmarkChat benchmarks chat completion performance
func BenchmarkChat(b *testing.B) {
	mockProvider := testingutils.NewMockProvider("benchmark-provider")

	// Set up mock responses
	fixtures := testingutils.NewTestFixtures()
	chatResp := fixtures.ChatResponse()
	mockProvider.SetChatResponse(chatResp)

	ctx := context.Background()
	req := fixtures.ChatRequest()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mockProvider.Chat(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkChatParallel benchmarks chat completion performance with parallel execution
func BenchmarkChatParallel(b *testing.B) {
	mockProvider := testingutils.NewMockProvider("benchmark-provider")

	fixtures := testingutils.NewTestFixtures()
	chatResp := fixtures.ChatResponse()
	mockProvider.SetChatResponse(chatResp)

	ctx := context.Background()
	req := fixtures.ChatRequest()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := mockProvider.Chat(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkEmbedding benchmarks embedding performance
func BenchmarkEmbedding(b *testing.B) {
	mockProvider := testingutils.NewMockProvider("benchmark-provider")

	fixtures := testingutils.NewTestFixtures()
	embedResp := fixtures.EmbeddingResponse()
	mockProvider.SetEmbeddingResponse(embedResp)

	ctx := context.Background()
	req := fixtures.EmbeddingRequest()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mockProvider.Embed(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEmbeddingParallel benchmarks embedding performance with parallel execution
func BenchmarkEmbeddingParallel(b *testing.B) {
	mockProvider := testingutils.NewMockProvider("benchmark-provider")

	fixtures := testingutils.NewTestFixtures()
	embedResp := fixtures.EmbeddingResponse()
	mockProvider.SetEmbeddingResponse(embedResp)

	ctx := context.Background()
	req := fixtures.EmbeddingRequest()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := mockProvider.Embed(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkRerank benchmarks rerank performance
func BenchmarkRerank(b *testing.B) {
	mockProvider := testingutils.NewMockProvider("benchmark-provider")

	fixtures := testingutils.NewTestFixtures()
	rerankResp := fixtures.RerankResponse()
	mockProvider.SetRerankResponse(rerankResp)

	ctx := context.Background()
	req := fixtures.RerankRequest()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mockProvider.Rerank(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRerankParallel benchmarks rerank performance with parallel execution
func BenchmarkRerankParallel(b *testing.B) {
	mockProvider := testingutils.NewMockProvider("benchmark-provider")

	fixtures := testingutils.NewTestFixtures()
	rerankResp := fixtures.RerankResponse()
	mockProvider.SetRerankResponse(rerankResp)

	ctx := context.Background()
	req := fixtures.RerankRequest()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := mockProvider.Rerank(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkModelDiscovery benchmarks model discovery performance
func BenchmarkModelDiscovery(b *testing.B) {
	mockProvider := testingutils.NewMockProvider("benchmark-provider")

	fixtures := testingutils.NewTestFixtures()

	// Set up multiple models
	models := []toolkit.ModelInfo{
		fixtures.ModelInfo(),
		{
			ID:   "model-2",
			Name: "Model 2",
			Capabilities: toolkit.ModelCapabilities{
				SupportsChat:      true,
				SupportsEmbedding: true,
				ContextWindow:     8192,
				MaxTokens:         4096,
			},
			Provider:    "benchmark-provider",
			Description: "Another test model",
		},
	}
	mockProvider.SetModels(models)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mockProvider.DiscoverModels(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConfigValidation benchmarks config validation performance
func BenchmarkConfigValidation(b *testing.B) {
	mockProvider := testingutils.NewMockProvider("benchmark-provider")

	config := map[string]interface{}{
		"api_key":     "test-key",
		"model":       "test-model",
		"temperature": 0.7,
		"max_tokens":  100,
		"timeout":     30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := mockProvider.ValidateConfig(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}
