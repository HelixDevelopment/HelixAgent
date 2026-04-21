package service_interactions_test

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServiceInteractions tests interactions between different services
func TestServiceInteractions(t *testing.T) {
	// Integration test — no external deps required

	t.Run("RequestService with ProviderRegistry integration", func(t *testing.T) {
		// Create services directly to avoid provider registry registering real providers
		ensembleService := services.NewEnsembleService("confidence_weighted", 30*time.Second)
		requestService := services.NewRequestService("weighted", ensembleService, nil)

		// Register mock providers
		mockProvider1 := &MockProvider{
			name:          "mock-provider-1",
			response:      "Response from mock provider 1",
			shouldSucceed: true,
			confidence:    0.9,
		}

		mockProvider2 := &MockProvider{
			name:          "mock-provider-2",
			response:      "Response from mock provider 2",
			shouldSucceed: true,
			confidence:    0.8,
		}

		// Register providers through the request service
		requestService.RegisterProvider(mockProvider1.name, mockProvider1)
		requestService.RegisterProvider(mockProvider2.name, mockProvider2)

		// Test getting all providers
		providers := requestService.GetProviders()
		assert.Len(t, providers, 2)
		assert.Contains(t, providers, "mock-provider-1")
		assert.Contains(t, providers, "mock-provider-2")

		// Test getting provider health
		health := requestService.GetAllProviderHealth()
		assert.Len(t, health, 2)
		assert.Contains(t, health, "mock-provider-1")
		assert.Contains(t, health, "mock-provider-2")

		// Test removing a provider
		requestService.RemoveProvider("mock-provider-1")
		providers = requestService.GetProviders()
		assert.Len(t, providers, 1)
		assert.Contains(t, providers, "mock-provider-2")
		assert.NotContains(t, providers, "mock-provider-1")
	})

	t.Run("EnsembleService with multiple providers", func(t *testing.T) {
		// Create ensemble service
		ensembleService := services.NewEnsembleService("confidence_weighted", 5*time.Second)

		// Register multiple mock providers
		providers := []*MockProvider{
			{
				name:          "provider-a",
				response:      "Response from provider A",
				responseDelay: 50 * time.Millisecond,
				shouldSucceed: true,
				confidence:    0.9,
			},
			{
				name:          "provider-b",
				response:      "Response from provider B",
				responseDelay: 30 * time.Millisecond,
				shouldSucceed: true,
				confidence:    0.8,
			},
			{
				name:          "provider-c",
				response:      "Response from provider C",
				responseDelay: 70 * time.Millisecond,
				shouldSucceed: true,
				confidence:    0.7,
			},
		}

		for _, provider := range providers {
			ensembleService.RegisterProvider(provider.name, provider)
		}

		// Test ensemble with multiple providers
		request := &models.LLMRequest{
			Prompt: "Test prompt for ensemble",
			ModelParams: models.ModelParameters{
				Model:       "ensemble-test",
				MaxTokens:   100,
				Temperature: 0.7,
			},
			EnsembleConfig: &models.EnsembleConfig{
				Strategy:            "confidence_weighted",
				MinProviders:        2,
				ConfidenceThreshold: 0.5,
				FallbackToBest:      true,
				Timeout:             5,
				PreferredProviders:  []string{"provider-a", "provider-b"},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		result, err := ensembleService.RunEnsemble(ctx, request)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Selected)
		assert.Contains(t, []string{
			"Response from provider A",
			"Response from provider B",
			"Response from provider C",
		}, result.Selected.Content)
	})

	t.Run("MemoryService with ContextManager integration", func(t *testing.T) {
		// Create context manager
		contextManager := services.NewContextManager(100)

		// Add context entries
		contextEntries := []*services.ContextEntry{
			{
				ID:       "memory-context-1",
				Type:     "memory",
				Source:   "test",
				Content:  "User prefers dark mode interface",
				Priority: 7,
			},
			{
				ID:       "memory-context-2",
				Type:     "memory",
				Source:   "test",
				Content:  "User is working on Go project",
				Priority: 8,
			},
		}

		for _, entry := range contextEntries {
			err := contextManager.AddEntry(entry)
			require.NoError(t, err)
		}

		// Build context for a request
		context, err := contextManager.BuildContext("chat", 500)
		require.NoError(t, err)
		assert.NotEmpty(t, context)

		// Verify memory-related context is included
		hasMemoryContext := false
		for _, entry := range context {
			if entry.Type == "memory" {
				hasMemoryContext = true
				break
			}
		}
		assert.True(t, hasMemoryContext, "Should include memory context entries")

		// Test cache integration
		testData := map[string]interface{}{
			"user_id":    "test-user-123",
			"preference": "dark_mode",
		}
		contextManager.CacheResult("user-prefs", testData, 5*time.Minute)

		cached, found := contextManager.GetCachedResult("user-prefs")
		assert.True(t, found)
		assert.Equal(t, testData, cached)
	})

	t.Run("End-to-end service chain", func(t *testing.T) {
		// Create services directly to avoid provider registry registering real providers
		ensembleService := services.NewEnsembleService("confidence_weighted", 30*time.Second)
		requestService := services.NewRequestService("weighted", ensembleService, nil)

		// Register a mock provider
		mockProvider := &MockProvider{
			name:          "e2e-provider",
			response:      "Integrated response from provider chain",
			shouldSucceed: true,
			confidence:    0.95,
		}

		// Register provider through request service
		requestService.RegisterProvider(mockProvider.name, mockProvider)

		// Test the full chain
		ctx := context.Background()

		// Get available providers
		providers := requestService.GetProviders()
		assert.Contains(t, providers, "e2e-provider")

		// Get provider health
		health := requestService.GetAllProviderHealth()
		assert.Contains(t, health, "e2e-provider")

		// Test processing a request
		request := &models.LLMRequest{
			Prompt: "Test end-to-end request",
			ModelParams: models.ModelParameters{
				Model:       "test-model",
				MaxTokens:   100,
				Temperature: 0.7,
			},
		}

		response, err := requestService.ProcessRequest(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "Integrated response from provider chain", response.Content)
		assert.Equal(t, "e2e-provider", response.ProviderName)
	})
}

// MockProvider implements the LLMProvider interface for testing
type MockProvider struct {
	name          string
	response      string
	responseDelay time.Duration
	shouldSucceed bool
	confidence    float64
	errorMessage  string
}

func (m *MockProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.responseDelay > 0 {
		select {
		case <-time.After(m.responseDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if !m.shouldSucceed {
		return nil, assert.AnError
	}

	return &models.LLMResponse{
		ID:           "mock-response-" + m.name,
		Content:      m.response,
		Confidence:   m.confidence,
		TokensUsed:   50,
		ResponseTime: int64(m.responseDelay / time.Millisecond),
		FinishReason: "stop",
		ProviderID:   m.name,
		ProviderName: m.name,
	}, nil
}

func (m *MockProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if !m.shouldSucceed {
		ch := make(chan *models.LLMResponse)
		close(ch)
		return ch, assert.AnError
	}

	ch := make(chan *models.LLMResponse)
	go func() {
		defer close(ch)

		// Send response in chunks for streaming
		chunks := []string{m.response}
		for _, chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			case ch <- &models.LLMResponse{
				ID:           "mock-stream-response-" + m.name,
				Content:      chunk,
				Confidence:   m.confidence,
				TokensUsed:   len(chunk) / 4,
				ResponseTime: int64(m.responseDelay / time.Millisecond),
				FinishReason: "stop",
				ProviderID:   m.name,
				ProviderName: m.name,
			}:
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	return ch, nil
}
