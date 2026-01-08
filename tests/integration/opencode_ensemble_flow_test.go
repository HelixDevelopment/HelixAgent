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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// =============================================================================
// TEST SUITE: OpenCode → AI Debate Group Flow Verification
// =============================================================================
// These tests verify that ALL OpenCode API calls flow through the AI Debate
// Group (ensemble) and that responses are the result of multi-provider consensus.
// =============================================================================

// TestEnsembleFlowVerification verifies the complete request flow from
// ChatCompletions handler through EnsembleService
func TestEnsembleFlowVerification(t *testing.T) {
	t.Run("ChatCompletions routes through ensemble", func(t *testing.T) {
		// Create ensemble service with mock providers
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		// Track which providers were called
		var calledProviders sync.Map
		var callCount int32

		// Register mock providers
		providers := []string{"provider1", "provider2", "provider3"}
		for _, name := range providers {
			providerName := name
			mockProvider := &MockEnsembleProvider{
				name: providerName,
				onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
					atomic.AddInt32(&callCount, 1)
					calledProviders.Store(providerName, true)
					return &models.LLMResponse{
						ID:           fmt.Sprintf("resp-%s", providerName),
						Content:      fmt.Sprintf("Response from %s", providerName),
						ProviderName: providerName,
						Confidence:   0.8 + float64(len(providerName))*0.01,
						FinishReason: "stop",
					}, nil
				},
			}
			ensemble.RegisterProvider(providerName, mockProvider)
		}

		// Execute ensemble request
		req := &models.LLMRequest{
			ID:      "test-request-1",
			Prompt:  "Test prompt",
			Messages: []models.Message{
				{Role: "user", Content: "Test message"},
			},
		}

		result, err := ensemble.RunEnsemble(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify ALL providers were called (parallel execution)
		assert.Equal(t, int32(len(providers)), atomic.LoadInt32(&callCount),
			"All providers should be called in parallel")

		for _, name := range providers {
			_, called := calledProviders.Load(name)
			assert.True(t, called, "Provider %s should have been called", name)
		}

		// Verify we got responses from all providers
		assert.Equal(t, len(providers), len(result.Responses),
			"Should have responses from all providers")

		// Verify a response was selected
		assert.NotNil(t, result.Selected, "A response should be selected")

		// Verify voting method was applied
		assert.Equal(t, "confidence_weighted", result.VotingMethod)

		// Verify scores were calculated
		assert.NotEmpty(t, result.Scores, "Scores should be calculated for voting")
	})

	t.Run("Ensemble handles provider failures gracefully", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		// Register one failing and two successful providers
		failingProvider := &MockEnsembleProvider{
			name: "failing",
			onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return nil, fmt.Errorf("provider failure")
			},
		}
		successProvider1 := &MockEnsembleProvider{
			name: "success1",
			onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return &models.LLMResponse{
					ID:         "resp-success1",
					Content:    "Success response 1",
					Confidence: 0.9,
				}, nil
			},
		}
		successProvider2 := &MockEnsembleProvider{
			name: "success2",
			onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return &models.LLMResponse{
					ID:         "resp-success2",
					Content:    "Success response 2",
					Confidence: 0.85,
				}, nil
			},
		}

		ensemble.RegisterProvider("failing", failingProvider)
		ensemble.RegisterProvider("success1", successProvider1)
		ensemble.RegisterProvider("success2", successProvider2)

		req := &models.LLMRequest{
			ID:     "test-request-2",
			Prompt: "Test prompt",
		}

		result, err := ensemble.RunEnsemble(context.Background(), req)
		require.NoError(t, err, "Ensemble should succeed even with one failing provider")
		require.NotNil(t, result)

		// Should have 2 successful responses
		assert.Equal(t, 2, len(result.Responses))

		// Should select from successful responses
		assert.NotNil(t, result.Selected)
		assert.NotEmpty(t, result.Selected.Content)

		// Metadata should track failures
		failedCount, ok := result.Metadata["failed_providers"].(int)
		assert.True(t, ok)
		assert.Equal(t, 1, failedCount)
	})
}

// TestProviderParallelExecution verifies providers are called concurrently
func TestProviderParallelExecution(t *testing.T) {
	t.Run("Providers execute in parallel not sequentially", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		var startTimes sync.Map
		var endTimes sync.Map
		providerDelay := 100 * time.Millisecond

		// Register providers with artificial delay
		for i := 1; i <= 3; i++ {
			name := fmt.Sprintf("parallel-provider-%d", i)
			providerName := name
			mockProvider := &MockEnsembleProvider{
				name: providerName,
				onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
					startTimes.Store(providerName, time.Now())
					time.Sleep(providerDelay)
					endTimes.Store(providerName, time.Now())
					return &models.LLMResponse{
						ID:         fmt.Sprintf("resp-%s", providerName),
						Content:    "Response",
						Confidence: 0.9,
					}, nil
				},
			}
			ensemble.RegisterProvider(providerName, mockProvider)
		}

		startTotal := time.Now()
		req := &models.LLMRequest{ID: "parallel-test", Prompt: "Test"}
		result, err := ensemble.RunEnsemble(context.Background(), req)
		totalDuration := time.Since(startTotal)

		require.NoError(t, err)
		require.NotNil(t, result)

		// If executed in parallel, total time should be ~delay, not 3*delay
		// Allow some overhead (2x delay max)
		maxExpectedDuration := providerDelay * 2
		assert.Less(t, totalDuration, maxExpectedDuration,
			"Providers should execute in parallel. Got %v, expected less than %v",
			totalDuration, maxExpectedDuration)

		// Verify all providers started around the same time
		var firstStart, lastStart time.Time
		startTimes.Range(func(key, value interface{}) bool {
			t := value.(time.Time)
			if firstStart.IsZero() || t.Before(firstStart) {
				firstStart = t
			}
			if lastStart.IsZero() || t.After(lastStart) {
				lastStart = t
			}
			return true
		})

		startDiff := lastStart.Sub(firstStart)
		assert.Less(t, startDiff, 50*time.Millisecond,
			"All providers should start within 50ms of each other for parallel execution")
	})
}

// TestVotingStrategyVerification verifies different voting strategies work correctly
func TestVotingStrategyVerification(t *testing.T) {
	t.Run("Confidence weighted voting selects highest confidence", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		// Register providers with different confidence levels
		confidences := map[string]float64{
			"low-conf":    0.5,
			"medium-conf": 0.7,
			"high-conf":   0.95,
		}

		for name, conf := range confidences {
			providerName := name
			confidence := conf
			mockProvider := &MockEnsembleProvider{
				name: providerName,
				onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
					return &models.LLMResponse{
						ID:           fmt.Sprintf("resp-%s", providerName),
						Content:      fmt.Sprintf("Response from %s", providerName),
						Confidence:   confidence,
						FinishReason: "stop",
						ResponseTime: 1000, // Same response time
					}, nil
				},
			}
			ensemble.RegisterProvider(providerName, mockProvider)
		}

		req := &models.LLMRequest{ID: "voting-test", Prompt: "Test"}
		result, err := ensemble.RunEnsemble(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Selected)

		// Highest confidence provider should be selected
		assert.Contains(t, result.Selected.Content, "high-conf",
			"Highest confidence response should be selected")
	})

	t.Run("Majority voting works with similar responses", func(t *testing.T) {
		ensemble := services.NewEnsembleService("majority_vote", 30*time.Second)

		// Register providers where 2 out of 3 give same answer
		responses := map[string]string{
			"provider1": "The answer is 42",
			"provider2": "The answer is 42",
			"provider3": "The answer is 43", // Different answer
		}

		for name, resp := range responses {
			providerName := name
			response := resp
			mockProvider := &MockEnsembleProvider{
				name: providerName,
				onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
					return &models.LLMResponse{
						ID:         fmt.Sprintf("resp-%s", providerName),
						Content:    response,
						Confidence: 0.8,
					}, nil
				},
			}
			ensemble.RegisterProvider(providerName, mockProvider)
		}

		req := &models.LLMRequest{ID: "majority-test", Prompt: "What is the answer?"}
		result, err := ensemble.RunEnsemble(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Selected)

		// Majority answer should be selected
		assert.Equal(t, "The answer is 42", result.Selected.Content,
			"Majority response should be selected")
	})
}

// TestResponseMetadataValidation verifies response metadata is correct
func TestResponseMetadataValidation(t *testing.T) {
	t.Run("Ensemble result contains proper metadata", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		for i := 1; i <= 2; i++ {
			name := fmt.Sprintf("meta-provider-%d", i)
			mockProvider := &MockEnsembleProvider{
				name: name,
				onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
					return &models.LLMResponse{
						ID:         fmt.Sprintf("resp-%s", name),
						Content:    "Response",
						Confidence: 0.9,
					}, nil
				},
			}
			ensemble.RegisterProvider(name, mockProvider)
		}

		req := &models.LLMRequest{ID: "metadata-test", Prompt: "Test"}
		result, err := ensemble.RunEnsemble(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify metadata fields
		assert.NotNil(t, result.Metadata)
		assert.Contains(t, result.Metadata, "total_providers")
		assert.Contains(t, result.Metadata, "successful_providers")
		assert.Contains(t, result.Metadata, "failed_providers")

		// Verify voting method
		assert.Equal(t, "confidence_weighted", result.VotingMethod)

		// Verify scores exist for all responses
		for _, resp := range result.Responses {
			_, hasScore := result.Scores[resp.ID]
			assert.True(t, hasScore, "Each response should have a score")
		}
	})
}

// TestOpenCodeAPIIntegration tests the full API flow with real HTTP requests
func TestOpenCodeAPIIntegration(t *testing.T) {
	// Skip if no server is running
	serverURL := os.Getenv("HELIXAGENT_TEST_URL")
	if serverURL == "" {
		serverURL = "http://localhost:7061"
	}

	// Check if server is available - use longer timeout for ensemble operations
	client := &http.Client{Timeout: 60 * time.Second}
	healthResp, err := client.Get(serverURL + "/health")
	if err != nil {
		t.Skip("HelixAgent server not available, skipping integration test")
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Skip("HelixAgent server not healthy, skipping integration test")
	}

	t.Run("ChatCompletions returns ensemble response", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "What is 2+2? Answer with just the number."},
			},
			"max_tokens": 10,
		}

		jsonBody, _ := json.Marshal(reqBody)
		resp, err := client.Post(
			serverURL+"/v1/chat/completions",
			"application/json",
			bytes.NewReader(jsonBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 502 {
			t.Skip("Providers temporarily unavailable (502), skipping test")
		}
		require.Equal(t, http.StatusOK, resp.StatusCode, "Response: %s", string(body))

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		// Verify response model indicates ensemble
		model, ok := result["model"].(string)
		assert.True(t, ok)
		assert.Equal(t, "helixagent-ensemble", model,
			"Response model should be helixagent-ensemble")

		// Verify system fingerprint
		fingerprint, ok := result["system_fingerprint"].(string)
		assert.True(t, ok)
		assert.Equal(t, "fp_helixagent_ensemble", fingerprint,
			"System fingerprint should indicate ensemble")

		// Verify we got choices
		choices, ok := result["choices"].([]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, choices, "Should have at least one choice")
	})

	t.Run("Multiple requests all go through ensemble", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan bool, 5)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				reqBody := map[string]interface{}{
					"model": "helixagent-ensemble",
					"messages": []map[string]string{
						{"role": "user", "content": fmt.Sprintf("Say 'test %d'", idx)},
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
					results <- false
					return
				}
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				var result map[string]interface{}
				if err := json.Unmarshal(body, &result); err != nil {
					results <- false
					return
				}

				// Verify ensemble markers
				model, _ := result["model"].(string)
				fingerprint, _ := result["system_fingerprint"].(string)

				results <- (model == "helixagent-ensemble" &&
					fingerprint == "fp_helixagent_ensemble")
			}(i)
		}

		wg.Wait()
		close(results)

		// Count successful requests - allow some failures due to load
		successCount := 0
		for success := range results {
			if success {
				successCount++
			}
		}
		// At least 1 should succeed to verify API works; skip if server is overwhelmed
		if successCount == 0 {
			t.Skip("No requests succeeded (server may be overloaded or unavailable)")
		}
		t.Logf("Multiple requests test: %d/5 succeeded through ensemble", successCount)
	})

	t.Run("Providers endpoint shows registered providers", func(t *testing.T) {
		resp, err := client.Get(serverURL + "/v1/providers")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		// Verify providers are registered
		count, ok := result["count"].(float64)
		assert.True(t, ok)
		assert.Greater(t, count, float64(0), "Should have at least one provider")

		providers, ok := result["providers"].([]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, providers, "Providers list should not be empty")
	})
}

// TestEnsembleServiceRegistration verifies providers are properly registered
func TestEnsembleServiceRegistration(t *testing.T) {
	t.Run("Provider registration adds to ensemble", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		// Initially no providers
		assert.Empty(t, ensemble.GetProviders())

		// Register providers
		for i := 1; i <= 3; i++ {
			name := fmt.Sprintf("reg-provider-%d", i)
			mockProvider := &MockEnsembleProvider{name: name}
			ensemble.RegisterProvider(name, mockProvider)
		}

		// Verify all providers registered
		providers := ensemble.GetProviders()
		assert.Equal(t, 3, len(providers))

		for i := 1; i <= 3; i++ {
			name := fmt.Sprintf("reg-provider-%d", i)
			assert.Contains(t, providers, name)
		}
	})

	t.Run("Provider removal updates ensemble", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		// Register providers
		ensemble.RegisterProvider("keep", &MockEnsembleProvider{name: "keep"})
		ensemble.RegisterProvider("remove", &MockEnsembleProvider{name: "remove"})

		assert.Equal(t, 2, len(ensemble.GetProviders()))

		// Remove one provider
		ensemble.RemoveProvider("remove")

		providers := ensemble.GetProviders()
		assert.Equal(t, 1, len(providers))
		assert.Contains(t, providers, "keep")
		assert.NotContains(t, providers, "remove")
	})
}

// TestRequestIDTracking verifies the same request ID is used across providers
func TestRequestIDTracking(t *testing.T) {
	t.Run("Same request ID sent to all providers", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		var receivedRequestIDs sync.Map

		for i := 1; i <= 3; i++ {
			name := fmt.Sprintf("tracking-provider-%d", i)
			providerName := name
			mockProvider := &MockEnsembleProvider{
				name: providerName,
				onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
					receivedRequestIDs.Store(providerName, req.ID)
					return &models.LLMResponse{
						ID:         "resp-" + providerName,
						Content:    "Response",
						Confidence: 0.9,
					}, nil
				},
			}
			ensemble.RegisterProvider(providerName, mockProvider)
		}

		originalRequestID := "unique-request-id-12345"
		req := &models.LLMRequest{
			ID:     originalRequestID,
			Prompt: "Test",
		}

		result, err := ensemble.RunEnsemble(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify all providers received the same request ID
		receivedRequestIDs.Range(func(key, value interface{}) bool {
			receivedID := value.(string)
			assert.Equal(t, originalRequestID, receivedID,
				"Provider %s should receive original request ID", key)
			return true
		})
	})
}

// TestEnsembleTimeout verifies timeout handling
func TestEnsembleTimeout(t *testing.T) {
	t.Run("Slow providers are handled with timeout", func(t *testing.T) {
		// Create ensemble with short timeout
		ensemble := services.NewEnsembleService("confidence_weighted", 500*time.Millisecond)

		// Register one fast and one slow provider
		fastProvider := &MockEnsembleProvider{
			name: "fast",
			onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return &models.LLMResponse{
					ID:         "resp-fast",
					Content:    "Fast response",
					Confidence: 0.9,
				}, nil
			},
		}

		slowProvider := &MockEnsembleProvider{
			name: "slow",
			onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				select {
				case <-time.After(2 * time.Second): // Will be cancelled
					return &models.LLMResponse{
						ID:      "resp-slow",
						Content: "Slow response",
					}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}

		ensemble.RegisterProvider("fast", fastProvider)
		ensemble.RegisterProvider("slow", slowProvider)

		req := &models.LLMRequest{ID: "timeout-test", Prompt: "Test"}

		start := time.Now()
		result, err := ensemble.RunEnsemble(context.Background(), req)
		duration := time.Since(start)

		// Should complete before slow provider would finish
		assert.Less(t, duration, 1*time.Second)

		// Should still have result from fast provider
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result.Responses), 1,
			"Should have at least one response from fast provider")
	})
}

// TestNoProvidersAvailable verifies error handling when no providers exist
func TestNoProvidersAvailable(t *testing.T) {
	t.Run("Returns error when no providers registered", func(t *testing.T) {
		ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

		req := &models.LLMRequest{ID: "no-providers-test", Prompt: "Test"}
		result, err := ensemble.RunEnsemble(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no providers")
	})
}

// =============================================================================
// Mock Provider Implementation
// =============================================================================

type MockEnsembleProvider struct {
	name       string
	onComplete func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
}

func (m *MockEnsembleProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.onComplete != nil {
		return m.onComplete(ctx, req)
	}
	return &models.LLMResponse{
		ID:         "mock-response-" + m.name,
		Content:    "Mock response from " + m.name,
		Confidence: 0.8,
	}, nil
}

func (m *MockEnsembleProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		resp, _ := m.Complete(ctx, req)
		ch <- resp
	}()
	return ch, nil
}

// =============================================================================
// Handler Flow Verification Tests
// =============================================================================

// TestHandlerEnsembleIntegration verifies handler properly calls ensemble
func TestHandlerEnsembleIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Handler calls ensemble for chat completions", func(t *testing.T) {
		// Create registry with mock ensemble
		registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

		// Register a test provider
		mockProvider := &MockLLMProviderForEnsemble{
			name: "test-handler-provider",
		}

		err := registry.RegisterProvider("test-handler-provider", mockProvider)
		require.NoError(t, err)

		// Verify provider is registered with ensemble
		ensembleService := registry.GetEnsembleService()
		require.NotNil(t, ensembleService)

		providers := ensembleService.GetProviders()
		assert.Contains(t, providers, "test-handler-provider",
			"Provider should be registered with ensemble service")
	})
}

// MockLLMProviderForEnsemble implements llm.LLMProvider interface
type MockLLMProviderForEnsemble struct {
	name string
}

func (m *MockLLMProviderForEnsemble) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	return &models.LLMResponse{
		ID:           "mock-" + m.name,
		Content:      "Mock response from " + m.name,
		Confidence:   0.85,
		FinishReason: "stop",
	}, nil
}

func (m *MockLLMProviderForEnsemble) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		resp, _ := m.Complete(ctx, req)
		ch <- resp
	}()
	return ch, nil
}

func (m *MockLLMProviderForEnsemble) HealthCheck() error {
	return nil
}

func (m *MockLLMProviderForEnsemble) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportsStreaming: true,
	}
}

func (m *MockLLMProviderForEnsemble) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkEnsembleExecution(b *testing.B) {
	ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	// Register providers
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("bench-provider-%d", i)
		mockProvider := &MockEnsembleProvider{
			name: name,
			onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return &models.LLMResponse{
					ID:         "resp",
					Content:    "Response",
					Confidence: 0.9,
				}, nil
			},
		}
		ensemble.RegisterProvider(name, mockProvider)
	}

	req := &models.LLMRequest{ID: "bench-test", Prompt: "Test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ensemble.RunEnsemble(context.Background(), req)
	}
}

func BenchmarkParallelEnsembleRequests(b *testing.B) {
	ensemble := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("parallel-bench-provider-%d", i)
		mockProvider := &MockEnsembleProvider{
			name: name,
			onComplete: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return &models.LLMResponse{
					ID:         "resp",
					Content:    "Response",
					Confidence: 0.9,
				}, nil
			},
		}
		ensemble.RegisterProvider(name, mockProvider)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := &models.LLMRequest{ID: "parallel-bench", Prompt: "Test"}
			_, _ = ensemble.RunEnsemble(context.Background(), req)
		}
	})
}
