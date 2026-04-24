package mem0_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TEST SUITE: Mem0 Memory Integration with AI Debate Group (unit-level)
// =============================================================================
// These tests verify that Mem0 Memory enhancement wraps LLM providers
// correctly and that ensemble responses propagate Mem0 metadata. They
// exercise in-process contracts only and do NOT require a live HelixAgent
// instance — CONST-030 compliant via Pattern-4 demote-to-unit.
// The live-integration portion (formerly `TestMem0LiveIntegration`) lives in
// `tests/integration/mem0_ensemble_live_test.go` and hits :8100 directly.
// =============================================================================

// MockBaseLLMProvider is a basic mock LLM provider for unit testing the
// Mem0 enhancement wrapper and ensemble interaction.
type MockBaseLLMProvider struct {
	name            string
	completeFunc    func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
	streamFunc      func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error)
	healthCheckFunc func() error
}

func (m *MockBaseLLMProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return &models.LLMResponse{
		ID:      "mock-response",
		Content: "Mock response from " + m.name,
	}, nil
}

func (m *MockBaseLLMProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		resp, _ := m.Complete(ctx, req)
		ch <- resp
	}()
	return ch, nil
}

func (m *MockBaseLLMProvider) HealthCheck() error {
	if m.healthCheckFunc != nil {
		return m.healthCheckFunc()
	}
	return nil
}

func (m *MockBaseLLMProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportsStreaming: true,
		Metadata:          make(map[string]string),
	}
}

func (m *MockBaseLLMProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

// TestMem0EnhancedProviderBasics verifies basic Mem0 provider enhancement
func TestMem0EnhancedProviderBasics(t *testing.T) {
	t.Run("Mem0EnhancedProvider wraps provider correctly", func(t *testing.T) {
		mockProvider := &MockBaseLLMProvider{
			name: "test-provider",
			completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return &models.LLMResponse{
					ID:         "resp-1",
					Content:    "Test response",
					Confidence: 0.9,
				}, nil
			},
		}

		// Create enhanced provider (without actual Mem0 Memory service for unit test)
		enhanced := services.NewCogneeEnhancedProvider("test", mockProvider, nil, nil)

		// Verify capabilities show Mem0 enhancement
		caps := enhanced.GetCapabilities()
		assert.NotNil(t, caps)
		assert.Contains(t, caps.Metadata, "cognee_enhanced")
		assert.Equal(t, "true", caps.Metadata["cognee_enhanced"])
		assert.Contains(t, caps.SupportedFeatures, "cognee_memory")
		assert.Contains(t, caps.SupportedFeatures, "knowledge_graph")
	})

	t.Run("Mem0EnhancedProvider Complete adds metadata", func(t *testing.T) {
		mockProvider := &MockBaseLLMProvider{
			name: "test-provider",
			completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return &models.LLMResponse{
					ID:         "resp-1",
					Content:    "Test response",
					Confidence: 0.9,
				}, nil
			},
		}

		enhanced := services.NewCogneeEnhancedProvider("test", mockProvider, nil, nil)

		req := &models.LLMRequest{ID: "test-1", Prompt: "Test"}
		resp, err := enhanced.Complete(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, resp)

		// Response should have Mem0 metadata
		assert.NotNil(t, resp.Metadata)
		// Note: cognee_enhanced will be false since we don't have a real Mem0 Memory service
		_, hasMem0Enhanced := resp.Metadata["cognee_enhanced"]
		assert.True(t, hasMem0Enhanced, "Response should have mem0_enhanced metadata")
	})

	t.Run("Enhanced provider returns underlying provider", func(t *testing.T) {
		mockProvider := &MockBaseLLMProvider{name: "underlying"}
		enhanced := services.NewCogneeEnhancedProvider("test", mockProvider, nil, nil)

		underlying := enhanced.GetUnderlyingProvider()
		assert.NotNil(t, underlying)
		assert.Equal(t, mockProvider, underlying)
	})

	t.Run("Enhanced provider config is retrievable", func(t *testing.T) {
		mockProvider := &MockBaseLLMProvider{name: "test"}
		enhanced := services.NewCogneeEnhancedProvider("test", mockProvider, nil, nil)

		config := enhanced.GetConfig()
		assert.NotNil(t, config)
		assert.True(t, config.EnhanceBeforeRequest)
		assert.True(t, config.StoreAfterResponse)
	})
}

// TestMem0ProviderStatsTracking verifies statistics tracking
func TestMem0ProviderStatsTracking(t *testing.T) {
	t.Run("Provider tracks request statistics", func(t *testing.T) {
		mockProvider := &MockBaseLLMProvider{
			name: "stats-test",
			completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return &models.LLMResponse{
					ID:      "resp",
					Content: "Response",
				}, nil
			},
		}

		enhanced := services.NewCogneeEnhancedProvider("stats-test", mockProvider, nil, nil)

		// Make several requests
		for i := 0; i < 5; i++ {
			req := &models.LLMRequest{ID: fmt.Sprintf("req-%d", i), Prompt: "Test"}
			_, err := enhanced.Complete(context.Background(), req)
			require.NoError(t, err)
		}

		stats := enhanced.GetStats()
		assert.Equal(t, int64(5), stats.TotalRequests, "Should track total requests")
	})

	t.Run("Stats track enhancement errors", func(t *testing.T) {
		mockProvider := &MockBaseLLMProvider{
			name: "error-test",
			completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return &models.LLMResponse{Content: "OK"}, nil
			},
		}

		enhanced := services.NewCogneeEnhancedProvider("error-test", mockProvider, nil, nil)
		stats := enhanced.GetStats()

		// Without Mem0 Memory service, we shouldn't have enhancement errors
		// (enhancement is skipped when service is nil)
		assert.Equal(t, int64(0), stats.EnhancementErrors)
	})
}

// TestMem0HealthCheck verifies health check behavior
func TestMem0HealthCheck(t *testing.T) {
	t.Run("Health check delegates to underlying provider", func(t *testing.T) {
		healthCheckCalled := false
		mockProvider := &MockBaseLLMProvider{
			name: "health-test",
			healthCheckFunc: func() error {
				healthCheckCalled = true
				return nil
			},
		}

		enhanced := services.NewCogneeEnhancedProvider("health-test", mockProvider, nil, nil)
		err := enhanced.HealthCheck()

		assert.NoError(t, err)
		assert.True(t, healthCheckCalled, "Should call underlying provider's health check")
	})
}

// TestMem0StreamingEnhancement tests streaming with Mem0 Memory
func TestMem0StreamingEnhancement(t *testing.T) {
	t.Run("Streaming response works through enhanced provider", func(t *testing.T) {
		mockProvider := &MockBaseLLMProvider{
			name: "stream-test",
			streamFunc: func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
				ch := make(chan *models.LLMResponse, 3)
				go func() {
					defer close(ch)
					ch <- &models.LLMResponse{Content: "Hello "}
					ch <- &models.LLMResponse{Content: "World"}
					ch <- &models.LLMResponse{Content: "!", FinishReason: "stop"}
				}()
				return ch, nil
			},
		}

		enhanced := services.NewCogneeEnhancedProvider("stream-test", mockProvider, nil, nil)

		req := &models.LLMRequest{ID: "stream-1", Prompt: "Test"}
		stream, err := enhanced.CompleteStream(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, stream)

		var content string
		for resp := range stream {
			content += resp.Content
		}

		assert.Equal(t, "Hello World!", content)
	})
}

// TestAllEnsembleProvidersHaveMem0Capabilities verifies all providers show Mem0 features
func TestAllEnsembleProvidersHaveMem0Capabilities(t *testing.T) {
	t.Run("All providers in ensemble show Mem0 capabilities", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		// Create multiple enhanced providers
		providers := []string{"deepseek", "mistral", "cerebras"}
		for _, name := range providers {
			mockProvider := &MockBaseLLMProvider{name: name}
			enhanced := services.NewCogneeEnhancedProvider(name, mockProvider, nil, nil)

			// Verify capabilities before registration
			caps := enhanced.GetCapabilities()
			assert.Contains(t, caps.Metadata, "cognee_enhanced",
				"Provider %s should have mem0_enhanced metadata", name)
			assert.Contains(t, caps.SupportedFeatures, "cognee_memory",
				"Provider %s should support mem0_memory", name)
			assert.Contains(t, caps.SupportedFeatures, "knowledge_graph",
				"Provider %s should support knowledge_graph", name)

			// Register with ensemble using adapter
			ensemble.RegisterProvider(name, &Mem0ProviderAdapter{enhanced})
		}

		// Verify all providers are registered
		registeredProviders := ensemble.GetProviders()
		assert.Equal(t, len(providers), len(registeredProviders))
	})
}

// TestMem0ConfigValidation tests configuration behavior
func TestMem0ConfigValidation(t *testing.T) {
	t.Run("Default config has sensible values", func(t *testing.T) {
		mockProvider := &MockBaseLLMProvider{name: "config-test"}
		enhanced := services.NewCogneeEnhancedProvider("test", mockProvider, nil, nil)

		config := enhanced.GetConfig()

		assert.True(t, config.EnhanceBeforeRequest, "Should enhance by default")
		assert.True(t, config.StoreAfterResponse, "Should store by default")
		assert.True(t, config.AutoCognifyResponses, "Should auto-cognify by default")
		assert.True(t, config.EnableGraphReasoning, "Graph reasoning should be enabled")
		assert.True(t, config.EnableCodeIntelligence, "Code intelligence should be enabled")
		assert.Greater(t, config.MaxContextInjection, 0, "Should have max context")
		assert.Greater(t, config.RelevanceThreshold, 0.0, "Should have relevance threshold")
		assert.NotEmpty(t, config.DefaultDataset, "Should have default dataset")
	})

	t.Run("Custom config is applied", func(t *testing.T) {
		mockProvider := &MockBaseLLMProvider{name: "custom-config"}
		customConfig := &services.CogneeProviderConfig{
			EnhanceBeforeRequest: false,
			StoreAfterResponse:   false,
			MaxContextInjection:  500,
			RelevanceThreshold:   0.9,
			DefaultDataset:       "custom",
		}

		enhanced := services.NewCogneeEnhancedProviderWithConfig(
			"test", mockProvider, nil, customConfig, nil)

		config := enhanced.GetConfig()

		assert.False(t, config.EnhanceBeforeRequest)
		assert.False(t, config.StoreAfterResponse)
		assert.Equal(t, 500, config.MaxContextInjection)
		assert.Equal(t, 0.9, config.RelevanceThreshold)
		assert.Equal(t, "custom", config.DefaultDataset)
	})
}

// TestEnsembleWithMem0Metadata tests that ensemble responses include Mem0 metadata
func TestEnsembleWithMem0Metadata(t *testing.T) {
	t.Run("Ensemble collects Mem0 metadata from providers", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		var responseCount int32

		providers := []string{"mem0-p1", "mem0-p2"}
		for _, name := range providers {
			providerName := name
			mockProvider := &MockBaseLLMProvider{
				name: providerName,
				completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
					atomic.AddInt32(&responseCount, 1)
					return &models.LLMResponse{
						ID:           fmt.Sprintf("resp-%s", providerName),
						Content:      fmt.Sprintf("Response from %s", providerName),
						ProviderName: providerName,
						Confidence:   0.85,
						Metadata: map[string]interface{}{
							"mem0_enhanced": true,
							"mem0_stored":   true,
						},
					}, nil
				},
			}

			enhanced := services.NewCogneeEnhancedProvider(providerName, mockProvider, nil, nil)
			ensemble.RegisterProvider(providerName, &Mem0ProviderAdapter{enhanced})
		}

		req := &models.LLMRequest{ID: "metadata-test", Prompt: "Test"}
		result, err := ensemble.RunEnsemble(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify all providers were called
		assert.Equal(t, int32(2), atomic.LoadInt32(&responseCount))

		// Verify we got responses
		assert.Equal(t, 2, len(result.Responses))

		// Verify selected response has metadata
		require.NotNil(t, result.Selected)
		require.NotNil(t, result.Selected.Metadata)
	})
}

// =============================================================================
// Mem0 Provider Adapter
// =============================================================================

// Mem0ProviderAdapter adapts CogneeEnhancedProvider to services.LLMProvider
// for use in Mem0 Memory ensemble unit tests
type Mem0ProviderAdapter struct {
	provider *services.CogneeEnhancedProvider
}

func (a *Mem0ProviderAdapter) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	return a.provider.Complete(ctx, req)
}

func (a *Mem0ProviderAdapter) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	return a.provider.CompleteStream(ctx, req)
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkMem0EnhancedProvider(b *testing.B) {
	mockProvider := &MockBaseLLMProvider{
		name: "bench",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{
				ID:      "resp",
				Content: "Response",
			}, nil
		},
	}

	enhanced := services.NewCogneeEnhancedProvider("bench", mockProvider, nil, nil)
	req := &models.LLMRequest{ID: "bench", Prompt: "Test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enhanced.Complete(context.Background(), req)
	}
}

func BenchmarkMem0EnsembleParallel(b *testing.B) {
	ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("bench-provider-%d", i)
		mockProvider := &MockBaseLLMProvider{
			name: name,
			completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return &models.LLMResponse{
					ID:         "resp",
					Content:    "Response",
					Confidence: 0.9,
				}, nil
			},
		}
		enhanced := services.NewCogneeEnhancedProvider(name, mockProvider, nil, nil)
		ensemble.RegisterProvider(name, &Mem0ProviderAdapter{enhanced})
	}

	req := &models.LLMRequest{ID: "bench", Prompt: "Test"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ensemble.RunEnsemble(context.Background(), req)
		}
	})
}
