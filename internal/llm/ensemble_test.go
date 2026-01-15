package llm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ensembleMockProvider is a test implementation of LLMProvider
type ensembleMockProvider struct {
	name       string
	response   *models.LLMResponse
	err        error
	delay      time.Duration
	callCount  int32
}

func (m *ensembleMockProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	atomic.AddInt32(&m.callCount, 1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *ensembleMockProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		if m.delay > 0 {
			time.Sleep(m.delay)
		}
		if m.err == nil && m.response != nil {
			ch <- m.response
		}
	}()
	return ch, m.err
}

func (m *ensembleMockProvider) HealthCheck() error {
	return nil
}

func (m *ensembleMockProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportsStreaming: true,
		SupportsTools:     true,
	}
}

func (m *ensembleMockProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

func (m *ensembleMockProvider) GetCallCount() int32 {
	return atomic.LoadInt32(&m.callCount)
}

// TestRunEnsemble tests the basic RunEnsemble function
func TestRunEnsemble(t *testing.T) {
	t.Run("Returns error with nil request", func(t *testing.T) {
		responses, selected, err := RunEnsemble(nil)
		assert.Error(t, err)
		assert.Nil(t, responses)
		assert.Nil(t, selected)
		assert.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("Returns error with no providers", func(t *testing.T) {
		req := &models.LLMRequest{
			Prompt: "test prompt",
		}
		responses, selected, err := RunEnsemble(req)
		assert.Error(t, err)
		assert.Nil(t, responses)
		assert.Nil(t, selected)
		assert.Contains(t, err.Error(), "no providers configured")
	})
}

// TestRunEnsembleWithProviders tests the ensemble execution with providers
func TestRunEnsembleWithProviders(t *testing.T) {
	t.Run("Returns error with nil request", func(t *testing.T) {
		providers := []LLMProvider{&ensembleMockProvider{}}
		responses, selected, err := RunEnsembleWithProviders(nil, providers)
		assert.Error(t, err)
		assert.Nil(t, responses)
		assert.Nil(t, selected)
	})

	t.Run("Returns error with empty providers", func(t *testing.T) {
		req := &models.LLMRequest{Prompt: "test"}
		responses, selected, err := RunEnsembleWithProviders(req, []LLMProvider{})
		assert.Error(t, err)
		assert.Nil(t, responses)
		assert.Nil(t, selected)
	})

	t.Run("Returns error with nil providers slice", func(t *testing.T) {
		req := &models.LLMRequest{Prompt: "test"}
		responses, selected, err := RunEnsembleWithProviders(req, nil)
		assert.Error(t, err)
		assert.Nil(t, responses)
		assert.Nil(t, selected)
	})

	t.Run("Single provider success", func(t *testing.T) {
		provider := &ensembleMockProvider{
			name: "test-provider",
			response: &models.LLMResponse{
				Content:    "Test response",
				Confidence: 0.9,
			},
		}

		req := &models.LLMRequest{Prompt: "test prompt"}
		responses, selected, err := RunEnsembleWithProviders(req, []LLMProvider{provider})

		require.NoError(t, err)
		assert.Len(t, responses, 1)
		assert.NotNil(t, selected)
		assert.Equal(t, "Test response", selected.Content)
		assert.Equal(t, int32(1), provider.GetCallCount())
	})

	t.Run("Multiple providers parallel execution", func(t *testing.T) {
		providers := []LLMProvider{
			&ensembleMockProvider{
				name: "provider-1",
				response: &models.LLMResponse{
					Content:    "Response 1",
					Confidence: 0.7,
				},
			},
			&ensembleMockProvider{
				name: "provider-2",
				response: &models.LLMResponse{
					Content:    "Response 2",
					Confidence: 0.9,
				},
			},
			&ensembleMockProvider{
				name: "provider-3",
				response: &models.LLMResponse{
					Content:    "Response 3",
					Confidence: 0.8,
				},
			},
		}

		req := &models.LLMRequest{Prompt: "test"}
		responses, selected, err := RunEnsembleWithProviders(req, providers)

		require.NoError(t, err)
		assert.Len(t, responses, 3)
		// Selected should be the one with highest confidence (0.9)
		assert.NotNil(t, selected)
		assert.Equal(t, "Response 2", selected.Content)
		assert.Equal(t, 0.9, selected.Confidence)

		// All providers should be called
		for _, p := range providers {
			mp := p.(*ensembleMockProvider)
			assert.Equal(t, int32(1), mp.GetCallCount())
		}
	})

	t.Run("Selects highest confidence response", func(t *testing.T) {
		providers := []LLMProvider{
			&ensembleMockProvider{
				response: &models.LLMResponse{Content: "Low", Confidence: 0.3},
			},
			&ensembleMockProvider{
				response: &models.LLMResponse{Content: "High", Confidence: 0.95},
			},
			&ensembleMockProvider{
				response: &models.LLMResponse{Content: "Medium", Confidence: 0.6},
			},
		}

		req := &models.LLMRequest{Prompt: "test"}
		_, selected, err := RunEnsembleWithProviders(req, providers)

		require.NoError(t, err)
		assert.Equal(t, "High", selected.Content)
		assert.Equal(t, 0.95, selected.Confidence)
	})

	t.Run("Handles provider errors gracefully", func(t *testing.T) {
		providers := []LLMProvider{
			&ensembleMockProvider{
				response: &models.LLMResponse{Content: "Success", Confidence: 0.8},
			},
			&ensembleMockProvider{
				err: errors.New("provider error"),
			},
		}

		req := &models.LLMRequest{Prompt: "test"}
		responses, selected, err := RunEnsembleWithProviders(req, providers)

		require.NoError(t, err)
		// Only one response (the successful one)
		assert.Len(t, responses, 1)
		assert.NotNil(t, selected)
		assert.Equal(t, "Success", selected.Content)
	})

	t.Run("All providers fail returns empty", func(t *testing.T) {
		providers := []LLMProvider{
			&ensembleMockProvider{err: errors.New("error 1")},
			&ensembleMockProvider{err: errors.New("error 2")},
		}

		req := &models.LLMRequest{Prompt: "test"}
		responses, selected, err := RunEnsembleWithProviders(req, providers)

		// No error returned, but no responses
		assert.NoError(t, err)
		assert.Len(t, responses, 0)
		assert.Nil(t, selected)
	})

	t.Run("Handles nil responses from providers", func(t *testing.T) {
		providers := []LLMProvider{
			&ensembleMockProvider{response: nil},
			&ensembleMockProvider{
				response: &models.LLMResponse{Content: "Valid", Confidence: 0.7},
			},
		}

		req := &models.LLMRequest{Prompt: "test"}
		responses, selected, err := RunEnsembleWithProviders(req, providers)

		require.NoError(t, err)
		assert.Len(t, responses, 1)
		assert.Equal(t, "Valid", selected.Content)
	})

	t.Run("Parallel execution with delays", func(t *testing.T) {
		// Create providers with different delays
		providers := []LLMProvider{
			&ensembleMockProvider{
				delay:    50 * time.Millisecond,
				response: &models.LLMResponse{Content: "Slow", Confidence: 0.6},
			},
			&ensembleMockProvider{
				delay:    10 * time.Millisecond,
				response: &models.LLMResponse{Content: "Fast", Confidence: 0.9},
			},
		}

		req := &models.LLMRequest{Prompt: "test"}
		start := time.Now()
		responses, _, err := RunEnsembleWithProviders(req, providers)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Len(t, responses, 2)
		// Should complete in roughly the time of the slowest provider (parallel execution)
		// Allow some buffer for test environment
		assert.Less(t, duration, 200*time.Millisecond, "Should execute in parallel")
	})

	t.Run("Zero confidence is valid", func(t *testing.T) {
		providers := []LLMProvider{
			&ensembleMockProvider{
				response: &models.LLMResponse{Content: "Zero confidence", Confidence: 0.0},
			},
		}

		req := &models.LLMRequest{Prompt: "test"}
		responses, selected, err := RunEnsembleWithProviders(req, providers)

		require.NoError(t, err)
		assert.Len(t, responses, 1)
		assert.NotNil(t, selected)
		assert.Equal(t, 0.0, selected.Confidence)
	})

	t.Run("Negative confidence handled correctly", func(t *testing.T) {
		providers := []LLMProvider{
			&ensembleMockProvider{
				response: &models.LLMResponse{Content: "Negative", Confidence: -0.5},
			},
			&ensembleMockProvider{
				response: &models.LLMResponse{Content: "Positive", Confidence: 0.5},
			},
		}

		req := &models.LLMRequest{Prompt: "test"}
		_, selected, err := RunEnsembleWithProviders(req, providers)

		require.NoError(t, err)
		// Should select the one with higher confidence
		assert.Equal(t, "Positive", selected.Content)
	})

	t.Run("Equal confidence selects first encountered", func(t *testing.T) {
		providers := []LLMProvider{
			&ensembleMockProvider{
				response: &models.LLMResponse{Content: "First", Confidence: 0.8},
			},
			&ensembleMockProvider{
				response: &models.LLMResponse{Content: "Second", Confidence: 0.8},
			},
		}

		req := &models.LLMRequest{Prompt: "test"}
		_, selected, err := RunEnsembleWithProviders(req, providers)

		require.NoError(t, err)
		assert.NotNil(t, selected)
		// Should be one of them (order not guaranteed due to parallel execution)
		assert.Equal(t, 0.8, selected.Confidence)
	})
}

// TestEnsembleConfig tests the EnsembleConfig struct
func TestEnsembleConfig(t *testing.T) {
	t.Run("Empty config", func(t *testing.T) {
		config := EnsembleConfig{}
		assert.Nil(t, config.Providers)
		assert.Len(t, config.Providers, 0)
	})

	t.Run("Config with providers", func(t *testing.T) {
		providers := []LLMProvider{
			&ensembleMockProvider{name: "p1"},
			&ensembleMockProvider{name: "p2"},
		}
		config := EnsembleConfig{Providers: providers}
		assert.Len(t, config.Providers, 2)
	})
}

// TestEnsembleConcurrency tests concurrent access patterns
func TestEnsembleConcurrency(t *testing.T) {
	t.Run("Multiple concurrent ensemble calls", func(t *testing.T) {
		provider := &ensembleMockProvider{
			response: &models.LLMResponse{Content: "Concurrent", Confidence: 0.8},
			delay:    10 * time.Millisecond,
		}

		var wg sync.WaitGroup
		numCalls := 10

		for i := 0; i < numCalls; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := &models.LLMRequest{Prompt: "concurrent test"}
				_, _, err := RunEnsembleWithProviders(req, []LLMProvider{provider})
				assert.NoError(t, err)
			}()
		}

		wg.Wait()
		assert.Equal(t, int32(numCalls), provider.GetCallCount())
	})
}

// Ensure ensembleMockProvider implements LLMProvider
var _ LLMProvider = (*ensembleMockProvider)(nil)
