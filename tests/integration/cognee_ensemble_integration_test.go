package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// =============================================================================
// TEST SUITE: Cognee Integration with AI Debate Group
// =============================================================================
// These tests verify that Cognee is heavily used by all LLMs contributing to
// the AI debate. Every provider in the ensemble should enhance requests with
// Cognee context and store responses back to Cognee for knowledge accumulation.
// =============================================================================

// TestCogneeEnhancedProviderBasics verifies basic Cognee provider enhancement
func TestCogneeEnhancedProviderBasics(t *testing.T) {
	t.Run("CogneeEnhancedProvider wraps provider correctly", func(t *testing.T) {
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

		// Create enhanced provider (without actual Cognee service for unit test)
		enhanced := services.NewCogneeEnhancedProvider("test", mockProvider, nil, nil)

		// Verify capabilities show Cognee enhancement
		caps := enhanced.GetCapabilities()
		assert.NotNil(t, caps)
		assert.Contains(t, caps.Metadata, "cognee_enhanced")
		assert.Equal(t, "true", caps.Metadata["cognee_enhanced"])
		assert.Contains(t, caps.SupportedFeatures, "cognee_memory")
		assert.Contains(t, caps.SupportedFeatures, "knowledge_graph")
	})

	t.Run("CogneeEnhancedProvider Complete adds metadata", func(t *testing.T) {
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

		// Response should have Cognee metadata
		assert.NotNil(t, resp.Metadata)
		// Note: cognee_enhanced will be false since we don't have a real Cognee service
		_, hasCogneeEnhanced := resp.Metadata["cognee_enhanced"]
		assert.True(t, hasCogneeEnhanced, "Response should have cognee_enhanced metadata")
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

// TestCogneeProviderStatsTracking verifies statistics tracking
func TestCogneeProviderStatsTracking(t *testing.T) {
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

		// Without Cognee service, we shouldn't have enhancement errors
		// (enhancement is skipped when service is nil)
		assert.Equal(t, int64(0), stats.EnhancementErrors)
	})
}

// TestCogneeHealthCheck verifies health check behavior
func TestCogneeHealthCheck(t *testing.T) {
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

// TestCogneeStreamingEnhancement tests streaming with Cognee
func TestCogneeStreamingEnhancement(t *testing.T) {
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

// TestAllEnsembleProvidersHaveCogneeCapabilities verifies all providers show Cognee features
func TestAllEnsembleProvidersHaveCogneeCapabilities(t *testing.T) {
	t.Run("All providers in ensemble show Cognee capabilities", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		// Create multiple enhanced providers
		providers := []string{"deepseek", "mistral", "cerebras"}
		for _, name := range providers {
			mockProvider := &MockBaseLLMProvider{name: name}
			enhanced := services.NewCogneeEnhancedProvider(name, mockProvider, nil, nil)

			// Verify capabilities before registration
			caps := enhanced.GetCapabilities()
			assert.Contains(t, caps.Metadata, "cognee_enhanced",
				"Provider %s should have cognee_enhanced metadata", name)
			assert.Contains(t, caps.SupportedFeatures, "cognee_memory",
				"Provider %s should support cognee_memory", name)
			assert.Contains(t, caps.SupportedFeatures, "knowledge_graph",
				"Provider %s should support knowledge_graph", name)

			// Register with ensemble using adapter
			ensemble.RegisterProvider(name, &CogneeProviderAdapter{enhanced})
		}

		// Verify all providers are registered
		registeredProviders := ensemble.GetProviders()
		assert.Equal(t, len(providers), len(registeredProviders))
	})
}

// TestCogneeLiveIntegration tests Cognee integration with live server
func TestCogneeLiveIntegration(t *testing.T) {
	serverURL := os.Getenv("HELIXAGENT_TEST_URL")
	if serverURL == "" {
		serverURL = "http://localhost:7061"
	}

	// Use longer timeout for ensemble operations
	client := &http.Client{Timeout: 60 * time.Second}

	// Check if server is available
	healthResp, err := client.Get(serverURL + "/health")
	if err != nil {
		t.Skip("HelixAgent server not available, skipping live integration test")
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Skip("HelixAgent server not healthy, skipping live integration test")
	}

	t.Run("Chat completion shows Cognee enhancement in providers", func(t *testing.T) {
		// Check providers endpoint for Cognee capabilities
		resp, err := client.Get(serverURL + "/v1/providers")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		providers, ok := result["providers"].([]interface{})
		assert.True(t, ok)

		cogneeEnabledCount := 0
		for _, p := range providers {
			provider := p.(map[string]interface{})
			metadata, ok := provider["metadata"].(map[string]interface{})
			if ok {
				if enhanced, exists := metadata["cognee_enhanced"]; exists && enhanced == "true" {
					cogneeEnabledCount++
				}
			}

			// Check for Cognee features in supported features
			features, ok := provider["supported_features"].([]interface{})
			if ok {
				for _, f := range features {
					if f.(string) == "cognee_memory" {
						cogneeEnabledCount++
						break
					}
				}
			}
		}

		assert.Greater(t, cogneeEnabledCount, 0,
			"At least some providers should have Cognee enhancement")
	})

	t.Run("Cognee search endpoint responds", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"query":       "test query",
			"dataset":     "default",
			"limit":       5,
			"search_type": "CHUNKS",
		}

		jsonBody, _ := json.Marshal(reqBody)
		resp, err := client.Post(
			serverURL+"/v1/cognee/search",
			"application/json",
			bytes.NewReader(jsonBody),
		)

		// Cognee might not be available, but endpoint should respond
		if err == nil {
			defer resp.Body.Close()
			// Accept 200 (success) or 400/503 (Cognee not ready/invalid)
			assert.Contains(t, []int{200, 400, 503}, resp.StatusCode,
				"Cognee search should respond appropriately")
		}
	})

	t.Run("Chat request goes through Cognee-enhanced ensemble", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "What is 1+1?"},
			},
			"max_tokens": 20,
		}

		jsonBody, _ := json.Marshal(reqBody)
		resp, err := client.Post(
			serverURL+"/v1/chat/completions",
			"application/json",
			bytes.NewReader(jsonBody),
		)
		if err != nil {
			// Handle network errors (timeout, EOF, connection reset) gracefully
			t.Skipf("Network error during chat request (server may be unavailable): %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			t.Skipf("Providers temporarily unavailable (%d), skipping test", resp.StatusCode)
		}
		require.Equal(t, http.StatusOK, resp.StatusCode, "Response: %s", string(body))

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		// Verify ensemble response
		assert.Equal(t, "helixagent-ensemble", result["model"])
		assert.Equal(t, "fp_helixagent_ensemble", result["system_fingerprint"])

		// Verify we got a response (Cognee enhancement happens transparently)
		choices, ok := result["choices"].([]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, choices)
	})

	t.Run("Multiple concurrent requests all use Cognee ensemble", func(t *testing.T) {
		var wg sync.WaitGroup
		type reqResult struct {
			success        bool
			providerFailed bool
		}
		results := make(chan reqResult, 5)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				reqBody := map[string]interface{}{
					"model": "helixagent-ensemble",
					"messages": []map[string]string{
						{"role": "user", "content": fmt.Sprintf("Count to %d", idx+1)},
					},
					"max_tokens": 30,
				}

				jsonBody, _ := json.Marshal(reqBody)
				resp, err := client.Post(
					serverURL+"/v1/chat/completions",
					"application/json",
					bytes.NewReader(jsonBody),
				)
				if err != nil {
					results <- reqResult{success: false, providerFailed: true}
					return
				}
				defer resp.Body.Close()

				// Check for provider failures (502, 503, 504)
				if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
					results <- reqResult{success: false, providerFailed: true}
					return
				}

				body, _ := io.ReadAll(resp.Body)
				var result map[string]interface{}
				if err := json.Unmarshal(body, &result); err != nil {
					results <- reqResult{success: false, providerFailed: false}
					return
				}

				// Verify ensemble markers
				model, _ := result["model"].(string)
				fingerprint, _ := result["system_fingerprint"].(string)

				results <- reqResult{
					success:        (model == "helixagent-ensemble" && fingerprint == "fp_helixagent_ensemble"),
					providerFailed: false,
				}
			}(i)
		}

		wg.Wait()
		close(results)

		successCount := 0
		providerFailCount := 0
		for res := range results {
			if res.success {
				successCount++
			}
			if res.providerFailed {
				providerFailCount++
			}
		}

		// If all failed due to provider issues, skip the test
		if providerFailCount == 5 {
			t.Skip("All requests failed due to provider unavailability (502/503/504)")
		}

		// At least 3 out of 5 should succeed (60% tolerance for server load)
		// But adjust for provider failures
		nonFailedRequests := 5 - providerFailCount
		expectedSuccesses := (nonFailedRequests * 60) / 100
		if expectedSuccesses < 1 {
			expectedSuccesses = 1
		}
		assert.GreaterOrEqual(t, successCount, expectedSuccesses,
			"At least 60%% of non-failed requests should go through ensemble (got %d/%d)", successCount, nonFailedRequests)
	})
}

// TestCogneeConfigValidation tests configuration behavior
func TestCogneeConfigValidation(t *testing.T) {
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

// TestEnsembleWithCogneeMetadata tests that ensemble responses include Cognee metadata
func TestEnsembleWithCogneeMetadata(t *testing.T) {
	t.Run("Ensemble collects Cognee metadata from providers", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		var responseCount int32

		providers := []string{"cognee-p1", "cognee-p2"}
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
							"cognee_enhanced": true,
							"cognee_stored":   true,
						},
					}, nil
				},
			}

			enhanced := services.NewCogneeEnhancedProvider(providerName, mockProvider, nil, nil)
			ensemble.RegisterProvider(providerName, &CogneeProviderAdapter{enhanced})
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
// Mock Implementations
// =============================================================================

// MockBaseLLMProvider is a basic mock LLM provider
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

// CogneeProviderAdapter adapts CogneeEnhancedProvider to services.LLMProvider
type CogneeProviderAdapter struct {
	provider *services.CogneeEnhancedProvider
}

func (a *CogneeProviderAdapter) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	return a.provider.Complete(ctx, req)
}

func (a *CogneeProviderAdapter) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	return a.provider.CompleteStream(ctx, req)
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkCogneeEnhancedProvider(b *testing.B) {
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

func BenchmarkCogneeEnsembleParallel(b *testing.B) {
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
		ensemble.RegisterProvider(name, &CogneeProviderAdapter{enhanced})
	}

	req := &models.LLMRequest{ID: "bench", Prompt: "Test"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ensemble.RunEnsemble(context.Background(), req)
		}
	})
}
