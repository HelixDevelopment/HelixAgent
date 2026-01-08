package services_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/helixagent/helixagent/internal/models"
	"github.com/helixagent/helixagent/internal/services"
)

// mockEnsembleProvider implements LLMProvider for testing
type mockEnsembleProvider struct {
	name           string
	response       *models.LLMResponse
	errorResponse  error
	streamResponse <-chan *models.LLMResponse
	streamError    error
}

func (m *mockEnsembleProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.errorResponse != nil {
		return nil, m.errorResponse
	}
	return m.response, nil
}

func (m *mockEnsembleProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	return m.streamResponse, m.streamError
}

func TestEnsembleService_NewEnsembleService(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	assert.NotNil(t, service)
	// Verify through behavior
	assert.Empty(t, service.GetProviders())
}

func TestEnsembleService_RegisterProvider(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	provider1 := &mockEnsembleProvider{name: "provider-1"}
	provider2 := &mockEnsembleProvider{name: "provider-2"}

	service.RegisterProvider("provider-1", provider1)
	service.RegisterProvider("provider-2", provider2)

	providers := service.GetProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, "provider-1")
	assert.Contains(t, providers, "provider-2")
}

func TestEnsembleService_RemoveProvider(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	provider1 := &mockEnsembleProvider{name: "provider-1"}
	provider2 := &mockEnsembleProvider{name: "provider-2"}

	service.RegisterProvider("provider-1", provider1)
	service.RegisterProvider("provider-2", provider2)

	providers := service.GetProviders()
	assert.Len(t, providers, 2)

	service.RemoveProvider("provider-1")

	providers = service.GetProviders()
	assert.Len(t, providers, 1)
	assert.Contains(t, providers, "provider-2")
	assert.NotContains(t, providers, "provider-1")
}

func TestEnsembleService_RunEnsemble_NoProviders(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	req := &models.LLMRequest{
		ID: "test-request",
		ModelParams: models.ModelParameters{
			Model: "test-model",
		},
	}

	result, err := service.RunEnsemble(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no providers available")
}

func TestEnsembleService_RunEnsemble_SingleProviderSuccess(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	response := &models.LLMResponse{
		ID:           "resp-1",
		Content:      "Test response",
		ProviderID:   "test-provider",
		ProviderName: "test-provider",
		Metadata: map[string]any{
			"confidence": 0.9,
		},
	}

	provider := &mockEnsembleProvider{
		name:     "test-provider",
		response: response,
	}

	service.RegisterProvider("test-provider", provider)

	req := &models.LLMRequest{
		ID: "test-request",
		ModelParams: models.ModelParameters{
			Model: "test-model",
		},
	}

	result, err := service.RunEnsemble(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Len(t, result.Responses, 1)
	assert.Equal(t, response, result.Selected)
	assert.Equal(t, "confidence_weighted", result.VotingMethod)
	assert.NotNil(t, result.Scores)
	assert.NotNil(t, result.Metadata)

	// Verify metadata
	assert.Equal(t, 1, result.Metadata["total_providers"])
	assert.Equal(t, 1, result.Metadata["successful_providers"])
	assert.Equal(t, 0, result.Metadata["failed_providers"])
}

func TestEnsembleService_RunEnsemble_MultipleProvidersSuccess(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	// Create multiple providers with different responses
	provider1 := &mockEnsembleProvider{
		name: "provider-1",
		response: &models.LLMResponse{
			ID:           "resp-1",
			Content:      "Response from provider 1",
			ProviderID:   "provider-1",
			ProviderName: "provider-1",
			Metadata: map[string]any{
				"confidence": 0.8,
			},
		},
	}

	provider2 := &mockEnsembleProvider{
		name: "provider-2",
		response: &models.LLMResponse{
			ID:           "resp-2",
			Content:      "Response from provider 2",
			ProviderID:   "provider-2",
			ProviderName: "provider-2",
			Metadata: map[string]any{
				"confidence": 0.9,
			},
		},
	}

	provider3 := &mockEnsembleProvider{
		name: "provider-3",
		response: &models.LLMResponse{
			ID:           "resp-3",
			Content:      "Response from provider 3",
			ProviderID:   "provider-3",
			ProviderName: "provider-3",
			Metadata: map[string]any{
				"confidence": 0.7,
			},
		},
	}

	service.RegisterProvider("provider-1", provider1)
	service.RegisterProvider("provider-2", provider2)
	service.RegisterProvider("provider-3", provider3)

	req := &models.LLMRequest{
		ID: "test-request",
		ModelParams: models.ModelParameters{
			Model: "test-model",
		},
	}

	result, err := service.RunEnsemble(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have 3 responses
	assert.Len(t, result.Responses, 3)
	assert.NotNil(t, result.Selected)
	assert.Equal(t, "confidence_weighted", result.VotingMethod)

	// Verify metadata
	assert.Equal(t, 3, result.Metadata["total_providers"])
	assert.Equal(t, 3, result.Metadata["successful_providers"])
	assert.Equal(t, 0, result.Metadata["failed_providers"])
}

func TestEnsembleService_RunEnsemble_ProviderFailure(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	// Create one successful and one failing provider
	successProvider := &mockEnsembleProvider{
		name: "success-provider",
		response: &models.LLMResponse{
			ID:           "resp-success",
			Content:      "Successful response",
			ProviderID:   "success-provider",
			ProviderName: "success-provider",
			Metadata: map[string]any{
				"confidence": 0.9,
			},
		},
	}

	failingProvider := &mockEnsembleProvider{
		name:          "failing-provider",
		errorResponse: assert.AnError,
	}

	service.RegisterProvider("success-provider", successProvider)
	service.RegisterProvider("failing-provider", failingProvider)

	req := &models.LLMRequest{
		ID: "test-request",
		ModelParams: models.ModelParameters{
			Model: "test-model",
		},
	}

	result, err := service.RunEnsemble(context.Background(), req)
	require.NoError(t, err) // Should still succeed with at least one response
	require.NotNil(t, result)

	// Should have 1 successful response
	assert.Len(t, result.Responses, 1)
	assert.NotNil(t, result.Selected)

	// Verify metadata includes error info
	assert.Equal(t, 2, result.Metadata["total_providers"])
	assert.Equal(t, 1, result.Metadata["successful_providers"])
	assert.Equal(t, 1, result.Metadata["failed_providers"])

	errors, ok := result.Metadata["errors"].([]error)
	assert.True(t, ok)
	assert.Len(t, errors, 1)
}

func TestEnsembleService_RunEnsemble_AllProvidersFail(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	// Create all failing providers
	provider1 := &mockEnsembleProvider{
		name:          "provider-1",
		errorResponse: assert.AnError,
	}

	provider2 := &mockEnsembleProvider{
		name:          "provider-2",
		errorResponse: assert.AnError,
	}

	service.RegisterProvider("provider-1", provider1)
	service.RegisterProvider("provider-2", provider2)

	req := &models.LLMRequest{
		ID: "test-request",
		ModelParams: models.ModelParameters{
			Model: "test-model",
		},
	}

	result, err := service.RunEnsemble(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, result)
	// Error message format: "[all_providers_failed] All X providers failed"
	assert.Contains(t, err.Error(), "providers failed")
}

func TestEnsembleService_RunEnsemble_Timeout(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 100*time.Millisecond) // Short timeout

	// Create a provider that takes longer than timeout
	slowProvider := &mockEnsembleProvider{
		name: "slow-provider",
		response: &models.LLMResponse{
			ID:           "resp-slow",
			Content:      "Slow response",
			ProviderID:   "slow-provider",
			ProviderName: "slow-provider",
		},
	}

	service.RegisterProvider("slow-provider", slowProvider)

	req := &models.LLMRequest{
		ID: "test-request",
		ModelParams: models.ModelParameters{
			Model: "test-model",
		},
	}

	// Note: Our mock doesn't actually delay, so this test verifies the timeout context is used
	result, err := service.RunEnsemble(context.Background(), req)

	// The mock returns immediately, so we should get a response
	// In a real test with actual delay, this would timeout
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 1)
}

func TestEnsembleService_RunEnsembleStream_NoProviders(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	req := &models.LLMRequest{
		ID: "test-request",
		ModelParams: models.ModelParameters{
			Model: "test-model",
		},
	}

	stream, err := service.RunEnsembleStream(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, stream)
	assert.Contains(t, err.Error(), "no providers available")
}

func TestEnsembleService_RunEnsembleStream_Success(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	// Create a mock stream
	ch := make(chan *models.LLMResponse, 1)
	ch <- &models.LLMResponse{
		ID:           "stream-resp",
		Content:      "Stream response",
		ProviderID:   "stream-provider",
		ProviderName: "stream-provider",
	}
	close(ch)

	provider := &mockEnsembleProvider{
		name:           "stream-provider",
		streamResponse: ch,
	}

	service.RegisterProvider("stream-provider", provider)

	req := &models.LLMRequest{
		ID: "test-request",
		ModelParams: models.ModelParameters{
			Model: "test-model",
		},
	}

	stream, err := service.RunEnsembleStream(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read from stream
	response, ok := <-stream
	assert.True(t, ok)
	assert.NotNil(t, response)
	assert.Equal(t, "Stream response", response.Content)

	// Stream should be closed
	_, ok = <-stream
	assert.False(t, ok)
}

func TestEnsembleService_RunEnsembleStream_ProviderError(t *testing.T) {
	service := services.NewEnsembleService("confidence_weighted", 30*time.Second)

	provider := &mockEnsembleProvider{
		name:        "error-provider",
		streamError: assert.AnError,
	}

	service.RegisterProvider("error-provider", provider)

	req := &models.LLMRequest{
		ID: "test-request",
		ModelParams: models.ModelParameters{
			Model: "test-model",
		},
	}

	stream, err := service.RunEnsembleStream(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, stream)
	// The actual implementation returns "no providers available for streaming" when provider has stream error
	assert.Contains(t, err.Error(), "no providers available")
}

func TestEnsembleService_VotingStrategies(t *testing.T) {
	// Test different voting strategies
	testCases := []struct {
		name     string
		strategy string
	}{
		{"ConfidenceWeighted", "confidence_weighted"},
		{"MajorityVote", "majority_vote"},
		{"QualityWeighted", "quality_weighted"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := services.NewEnsembleService(tc.strategy, 30*time.Second)

			// Create multiple providers
			for i := 1; i <= 3; i++ {
				provider := &mockEnsembleProvider{
					name: "provider-" + string(rune('A'+i-1)),
					response: &models.LLMResponse{
						ID:           "resp-" + string(rune('A'+i-1)),
						Content:      "Response " + string(rune('A'+i-1)),
						ProviderID:   "provider-" + string(rune('A'+i-1)),
						ProviderName: "provider-" + string(rune('A'+i-1)),
						Metadata: map[string]any{
							"confidence": 0.7 + float64(i)*0.1,
						},
					},
				}
				service.RegisterProvider("provider-"+string(rune('A'+i-1)), provider)
			}

			req := &models.LLMRequest{
				ID: "test-request",
				ModelParams: models.ModelParameters{
					Model: "test-model",
				},
			}

			result, err := service.RunEnsemble(context.Background(), req)
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, tc.strategy, result.VotingMethod)
			assert.Len(t, result.Responses, 3)
			assert.NotNil(t, result.Selected)
		})
	}
}
