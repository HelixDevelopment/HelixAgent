package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock LLM Provider for Ensemble Tests
// =============================================================================

type ensembleUnitTestMockProvider struct {
	name       string
	response   *models.LLMResponse
	err        error
	delay      time.Duration
	streamResp []*models.LLMResponse
	streamErr  error
	callCount  int
}

func newEnsembleMockProvider(name string, content string, confidence float64) *ensembleUnitTestMockProvider {
	return &ensembleUnitTestMockProvider{
		name: name,
		response: &models.LLMResponse{
			ID:           name + "-resp",
			Content:      content,
			Confidence:   confidence,
			ProviderID:   name,
			ProviderName: name,
			TokensUsed:   100,
			ResponseTime: 500,
			FinishReason: "stop",
		},
	}
}

func (m *ensembleUnitTestMockProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	m.callCount++

	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	if m.err != nil {
		return nil, m.err
	}

	if m.response == nil {
		return nil, fmt.Errorf("mock provider error")
	}

	resp := *m.response
	resp.ID = req.ID + "-response"
	return &resp, nil
}

func (m *ensembleUnitTestMockProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}

	ch := make(chan *models.LLMResponse, len(m.streamResp))
	go func() {
		defer close(ch)
		for _, resp := range m.streamResp {
			select {
			case <-ctx.Done():
				return
			case ch <- resp:
			}
		}
	}()
	return ch, nil
}

var _ LLMProvider = (*ensembleUnitTestMockProvider)(nil)

// =============================================================================
// Ensemble Service Creation Tests
// =============================================================================

func TestEnsembleUnit_NewEnsembleService(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		timeout  time.Duration
	}{
		{"confidence_weighted", "confidence_weighted", 30 * time.Second},
		{"majority_vote", "majority_vote", 60 * time.Second},
		{"quality_weighted", "quality_weighted", 45 * time.Second},
		{"unknown_strategy", "unknown", 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewEnsembleService(tt.strategy, tt.timeout)

			require.NotNil(t, service)
			assert.Equal(t, tt.strategy, service.strategy)
			assert.Equal(t, tt.timeout, service.timeout)
			assert.NotNil(t, service.providers)
			assert.Equal(t, 0, service.providers.Len())
		})
	}
}

// =============================================================================
// Provider Registration Tests
// =============================================================================

func TestEnsembleUnit_RegisterProvider(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	t.Run("register single provider", func(t *testing.T) {
		provider := newEnsembleMockProvider("test-provider", "test response", 0.9)
		service.RegisterProvider("test-provider", provider)

		providers := service.GetProviders()
		assert.Len(t, providers, 1)
		assert.Contains(t, providers, "test-provider")
	})

	t.Run("register multiple providers", func(t *testing.T) {
		service2 := NewEnsembleService("confidence_weighted", 30*time.Second)

		service2.RegisterProvider("provider1", newEnsembleMockProvider("provider1", "response1", 0.9))
		service2.RegisterProvider("provider2", newEnsembleMockProvider("provider2", "response2", 0.8))
		service2.RegisterProvider("provider3", newEnsembleMockProvider("provider3", "response3", 0.85))

		providers := service2.GetProviders()
		assert.Len(t, providers, 3)
	})
}

func TestEnsembleUnit_RemoveProvider(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	service.RegisterProvider("provider1", newEnsembleMockProvider("provider1", "response1", 0.9))
	service.RegisterProvider("provider2", newEnsembleMockProvider("provider2", "response2", 0.8))

	assert.Len(t, service.GetProviders(), 2)

	service.RemoveProvider("provider1")

	assert.Len(t, service.GetProviders(), 1)
	assert.Contains(t, service.GetProviders(), "provider2")
	assert.NotContains(t, service.GetProviders(), "provider1")
}

func TestEnsembleUnit_RemoveNonExistentProvider(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	// Should not panic
	service.RemoveProvider("non-existent")
	assert.Empty(t, service.GetProviders())
}

func TestEnsembleUnit_GetProviders(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	t.Run("empty providers", func(t *testing.T) {
		providers := service.GetProviders()
		assert.Empty(t, providers)
		assert.NotNil(t, providers)
	})

	t.Run("with providers", func(t *testing.T) {
		service.RegisterProvider("p1", newEnsembleMockProvider("p1", "r1", 0.9))
		service.RegisterProvider("p2", newEnsembleMockProvider("p2", "r2", 0.8))

		providers := service.GetProviders()
		assert.Len(t, providers, 2)
		assert.Contains(t, providers, "p1")
		assert.Contains(t, providers, "p2")
	})
}

// =============================================================================
// SetScoreProvider Tests
// =============================================================================

func TestEnsembleUnit_SetScoreProvider(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	// Create a mock score provider
	mockScoreProvider := &mockLLMsVerifierScoreProvider{
		scores: map[string]float64{
			"claude":   9.5,
			"deepseek": 8.5,
		},
	}

	service.SetScoreProvider(mockScoreProvider)

	assert.NotNil(t, service.scoreProvider)
}

// Mock LLMsVerifierScoreProvider for testing
type mockLLMsVerifierScoreProvider struct {
	scores map[string]float64
}

func (m *mockLLMsVerifierScoreProvider) GetProviderScore(provider string) (float64, bool) {
	score, ok := m.scores[provider]
	return score, ok
}

func (m *mockLLMsVerifierScoreProvider) GetModelScore(modelID string) (float64, bool) {
	return 0, false
}

func (m *mockLLMsVerifierScoreProvider) RefreshScores(ctx context.Context) error {
	return nil
}

var _ LLMsVerifierScoreProvider = (*mockLLMsVerifierScoreProvider)(nil)

// =============================================================================
// RunEnsemble Tests
// =============================================================================

func TestEnsembleUnit_RunEnsemble_NoProviders(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)
	ctx := context.Background()

	result, err := service.RunEnsemble(ctx, &models.LLMRequest{
		Prompt: "test",
	})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no providers available")
}

func TestEnsembleUnit_RunEnsemble_SingleProvider(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)
	ctx := context.Background()

	service.RegisterProvider("provider1", newEnsembleMockProvider("provider1", "test response", 0.9))

	result, err := service.RunEnsemble(ctx, &models.LLMRequest{
		Prompt: "test prompt",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 1)
	assert.NotNil(t, result.Selected)
	assert.Equal(t, "test response", result.Selected.Content)
	assert.Equal(t, "confidence_weighted", result.VotingMethod)
	assert.NotNil(t, result.Scores)
	assert.NotNil(t, result.Metadata)
}

func TestEnsembleUnit_RunEnsemble_MultipleProviders(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)
	ctx := context.Background()

	service.RegisterProvider("provider1", newEnsembleMockProvider("provider1", "response1", 0.7))
	service.RegisterProvider("provider2", newEnsembleMockProvider("provider2", "response2", 0.9))
	service.RegisterProvider("provider3", newEnsembleMockProvider("provider3", "response3", 0.8))

	result, err := service.RunEnsemble(ctx, &models.LLMRequest{
		Prompt: "test",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 3)
	assert.NotNil(t, result.Selected)
	assert.Equal(t, "confidence_weighted", result.VotingMethod)
}

func TestEnsembleUnit_RunEnsemble_ProviderError(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)
	ctx := context.Background()

	goodProvider := newEnsembleMockProvider("good", "good response", 0.9)
	badProvider := &ensembleUnitTestMockProvider{
		name: "bad",
		err:  errors.New("provider failed"),
	}

	service.RegisterProvider("good", goodProvider)
	service.RegisterProvider("bad", badProvider)

	result, err := service.RunEnsemble(ctx, &models.LLMRequest{
		Prompt: "test",
	})

	// Should still succeed if at least one provider works
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 1)
	assert.Equal(t, "good response", result.Selected.Content)
	assert.NotNil(t, result.Metadata["errors"])
}

func TestEnsembleUnit_RunEnsemble_AllProvidersFail(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)
	ctx := context.Background()

	badProvider1 := &ensembleUnitTestMockProvider{
		name: "bad1",
		err:  errors.New("provider 1 failed"),
	}
	badProvider2 := &ensembleUnitTestMockProvider{
		name: "bad2",
		err:  errors.New("provider 2 failed"),
	}

	service.RegisterProvider("bad1", badProvider1)
	service.RegisterProvider("bad2", badProvider2)

	result, err := service.RunEnsemble(ctx, &models.LLMRequest{
		Prompt: "test",
	})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "providers failed")
}

func TestEnsembleUnit_RunEnsemble_Timeout(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 100*time.Millisecond)
	ctx := context.Background()

	slowProvider := &ensembleUnitTestMockProvider{
		name:  "slow",
		delay: 200 * time.Millisecond,
		response: &models.LLMResponse{
			Content:    "slow response",
			Confidence: 0.9,
		},
	}

	service.RegisterProvider("slow", slowProvider)

	result, err := service.RunEnsemble(ctx, &models.LLMRequest{
		Prompt: "test",
	})

	// Should timeout or handle gracefully
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestEnsembleUnit_RunEnsemble_NilResponse(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)
	ctx := context.Background()

	nilProvider := &ensembleUnitTestMockProvider{
		name:     "nil",
		response: nil,
		err:      nil,
	}

	service.RegisterProvider("nil", nilProvider)

	result, err := service.RunEnsemble(ctx, &models.LLMRequest{
		Prompt: "test",
	})

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestEnsembleUnit_RunEnsemble_WithPreferredProviders(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)
	ctx := context.Background()

	service.RegisterProvider("provider1", newEnsembleMockProvider("provider1", "response1", 0.7))
	service.RegisterProvider("provider2", newEnsembleMockProvider("provider2", "response2", 0.9))
	service.RegisterProvider("provider3", newEnsembleMockProvider("provider3", "response3", 0.8))

	result, err := service.RunEnsemble(ctx, &models.LLMRequest{
		Prompt: "test",
		EnsembleConfig: &models.EnsembleConfig{
			PreferredProviders: []string{"provider2", "provider1"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	// Should filter to preferred providers
	assert.LessOrEqual(t, len(result.Responses), 3)
}

func TestEnsembleUnit_RunEnsemble_NoSuitableProviders(t *testing.T) {

	service := NewEnsembleService("confidence_weighted", 30*time.Second)
	ctx := context.Background()

	service.RegisterProvider("provider1", newEnsembleMockProvider("provider1", "response1", 0.7))

	result, err := service.RunEnsemble(ctx, &models.LLMRequest{
		Prompt: "test",
		EnsembleConfig: &models.EnsembleConfig{
			PreferredProviders: []string{"non-existent"},
		},
	})

	// Should error since no suitable providers match
	assert.Error(t, err)
	assert.Nil(t, result)
}

// =============================================================================
// Filter Providers Tests
// =============================================================================

func TestEnsembleUnit_FilterProviders(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	providers := map[string]LLMProvider{
		"provider1": newEnsembleMockProvider("provider1", "r1", 0.9),
		"provider2": newEnsembleMockProvider("provider2", "r2", 0.8),
		"provider3": newEnsembleMockProvider("provider3", "r3", 0.7),
	}

	t.Run("no preferred providers", func(t *testing.T) {
		filtered := service.filterProviders(providers, &models.LLMRequest{})
		assert.Len(t, filtered, 3)
	})

	t.Run("with preferred providers", func(t *testing.T) {
		filtered := service.filterProviders(providers, &models.LLMRequest{
			EnsembleConfig: &models.EnsembleConfig{
				PreferredProviders: []string{"provider1", "provider2"},
			},
		})
		assert.Len(t, filtered, 2)
		assert.Contains(t, filtered, "provider1")
		assert.Contains(t, filtered, "provider2")
	})

	t.Run("minimum providers", func(t *testing.T) {
		filtered := service.filterProviders(providers, &models.LLMRequest{
			EnsembleConfig: &models.EnsembleConfig{
				PreferredProviders: []string{"provider1"},
				MinProviders:       2,
			},
		})
		assert.GreaterOrEqual(t, len(filtered), 2)
	})
}

// =============================================================================
// GetSortedProviderNames Tests
// =============================================================================

func TestEnsembleUnit_GetSortedProviderNames(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	providers := map[string]LLMProvider{
		"claude":       newEnsembleMockProvider("claude", "r1", 0.9),
		"deepseek":     newEnsembleMockProvider("deepseek", "r2", 0.8),
		"claude-oauth": newEnsembleMockProvider("claude-oauth", "r3", 0.95),
	}

	// Set up score provider
	mockScoreProvider := &mockLLMsVerifierScoreProvider{
		scores: map[string]float64{
			"claude":   9.5,
			"deepseek": 8.5,
		},
	}
	service.SetScoreProvider(mockScoreProvider)

	sorted := service.getSortedProviderNames(providers)

	assert.Len(t, sorted, 3)
	// OAuth providers should come first
	assert.Equal(t, "claude-oauth", sorted[0])
}

func TestEnsembleUnit_GetSortedProviderNames_Empty(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	providers := map[string]LLMProvider{}

	sorted := service.getSortedProviderNames(providers)

	assert.Empty(t, sorted)
}

// =============================================================================
// Voting Strategy Tests
// =============================================================================

func TestEnsembleUnit_Vote_ConfidenceWeighted(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	responses := []*models.LLMResponse{
		{ID: "r1", Content: "response1", Confidence: 0.7, ProviderName: "p1"},
		{ID: "r2", Content: "response2", Confidence: 0.9, ProviderName: "p2"},
		{ID: "r3", Content: "response3", Confidence: 0.8, ProviderName: "p3"},
	}

	selected, scores, err := service.vote(responses, &models.LLMRequest{})

	require.NoError(t, err)
	assert.NotNil(t, selected)
	assert.Equal(t, "r2", selected.ID) // Highest confidence
	assert.NotNil(t, scores)
	assert.True(t, selected.Selected)
}

func TestEnsembleUnit_Vote_MajorityVote(t *testing.T) {
	service := NewEnsembleService("majority_vote", 30*time.Second)

	responses := []*models.LLMResponse{
		{ID: "r1", Content: "common response", Confidence: 0.7, ProviderName: "p1"},
		{ID: "r2", Content: "common response", Confidence: 0.8, ProviderName: "p2"},
		{ID: "r3", Content: "different response", Confidence: 0.9, ProviderName: "p3"},
	}

	selected, _, err := service.vote(responses, &models.LLMRequest{})

	require.NoError(t, err)
	assert.NotNil(t, selected)
}

func TestEnsembleUnit_Vote_QualityWeighted(t *testing.T) {
	service := NewEnsembleService("quality_weighted", 30*time.Second)

	responses := []*models.LLMResponse{
		{ID: "r1", Content: strings.Repeat("x", 200), Confidence: 0.7, ProviderName: "p1", TokensUsed: 50, ResponseTime: 500, FinishReason: "stop"},
		{ID: "r2", Content: strings.Repeat("y", 500), Confidence: 0.8, ProviderName: "p2", TokensUsed: 100, ResponseTime: 600, FinishReason: "stop"},
	}

	selected, _, err := service.vote(responses, &models.LLMRequest{})

	require.NoError(t, err)
	assert.NotNil(t, selected)
}

func TestEnsembleUnit_Vote_NoResponses(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	responses := []*models.LLMResponse{}

	selected, scores, err := service.vote(responses, &models.LLMRequest{})

	assert.Error(t, err)
	assert.Nil(t, selected)
	assert.Nil(t, scores)
}

func TestEnsembleUnit_Vote_DefaultStrategy(t *testing.T) {
	service := NewEnsembleService("unknown_strategy", 30*time.Second)

	responses := []*models.LLMResponse{
		{ID: "r1", Content: "response1", Confidence: 0.8, ProviderName: "p1"},
	}

	selected, _, err := service.vote(responses, &models.LLMRequest{})

	require.NoError(t, err)
	assert.NotNil(t, selected)
}

// =============================================================================
// ConfidenceWeightedStrategy Tests
// =============================================================================

func TestEnsembleUnit_ConfidenceWeightedStrategy_Vote(t *testing.T) {
	strategy := &ConfidenceWeightedStrategy{}

	responses := []*models.LLMResponse{
		{ID: "r1", Content: "response1", Confidence: 0.6, ProviderName: "p1", TokensUsed: 100, ResponseTime: 500, FinishReason: "stop"},
		{ID: "r2", Content: "response2", Confidence: 0.9, ProviderName: "p2", TokensUsed: 120, ResponseTime: 400, FinishReason: "stop"},
	}

	selected, _, err := strategy.Vote(responses, &models.LLMRequest{})

	require.NoError(t, err)
	assert.NotNil(t, selected)
	assert.Equal(t, "r2", selected.ID)
}

func TestEnsembleUnit_ConfidenceWeightedStrategy_Vote_NoResponses(t *testing.T) {
	strategy := &ConfidenceWeightedStrategy{}

	responses := []*models.LLMResponse{}

	selected, scores, err := strategy.Vote(responses, &models.LLMRequest{})

	assert.Error(t, err)
	assert.Nil(t, selected)
	assert.Nil(t, scores)
}

func TestEnsembleUnit_ConfidenceWeightedStrategy_Vote_WithPreferredProviders(t *testing.T) {
	strategy := &ConfidenceWeightedStrategy{}

	responses := []*models.LLMResponse{
		{ID: "r1", Content: "response1", Confidence: 0.9, ProviderName: "preferred"},
		{ID: "r2", Content: "response2", Confidence: 0.8, ProviderName: "other"},
	}

	selected, scores, err := strategy.Vote(responses, &models.LLMRequest{
		EnsembleConfig: &models.EnsembleConfig{
			PreferredProviders: []string{"preferred"},
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, selected)
	assert.NotNil(t, scores)
	// Preferred provider should have higher score
	assert.Greater(t, scores["r1"], scores["r2"])
}

func TestEnsembleUnit_ConfidenceWeightedStrategy_ApplyQualityWeights(t *testing.T) {
	strategy := &ConfidenceWeightedStrategy{}

	tests := []struct {
		name        string
		resp        *models.LLMResponse
		baseScore   float64
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "optimal length",
			resp:        &models.LLMResponse{Content: strings.Repeat("x", 200), ResponseTime: 500, TokensUsed: 50, FinishReason: "stop"},
			baseScore:   1.0,
			expectedMin: 1.0,
			expectedMax: 2.0,
		},
		{
			name:        "too long",
			resp:        &models.LLMResponse{Content: strings.Repeat("x", 3000), ResponseTime: 15000, TokensUsed: 1000, FinishReason: "length"},
			baseScore:   1.0,
			expectedMin: 0.5,
			expectedMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := strategy.applyQualityWeights(tt.resp, tt.baseScore)
			assert.GreaterOrEqual(t, score, tt.expectedMin)
			assert.LessOrEqual(t, score, tt.expectedMax)
		})
	}
}

// =============================================================================
// MajorityVoteStrategy Tests
// =============================================================================

func TestEnsembleUnit_MajorityVoteStrategy_Vote(t *testing.T) {
	strategy := &MajorityVoteStrategy{}

	responses := []*models.LLMResponse{
		{ID: "r1", Content: "common answer", Confidence: 0.8},
		{ID: "r2", Content: "common answer", Confidence: 0.9},
		{ID: "r3", Content: "different answer", Confidence: 0.7},
	}

	selected, _, err := strategy.Vote(responses, &models.LLMRequest{})

	require.NoError(t, err)
	assert.NotNil(t, selected)
}

func TestEnsembleUnit_MajorityVoteStrategy_Vote_NoResponses(t *testing.T) {
	strategy := &MajorityVoteStrategy{}

	responses := []*models.LLMResponse{}

	selected, _, err := strategy.Vote(responses, &models.LLMRequest{})

	assert.Error(t, err)
	assert.Nil(t, selected)
}

func TestEnsembleUnit_MajorityVoteStrategy_Vote_SingleResponse(t *testing.T) {
	strategy := &MajorityVoteStrategy{}

	responses := []*models.LLMResponse{
		{ID: "r1", Content: "only response", Confidence: 0.8},
	}

	selected, _, err := strategy.Vote(responses, &models.LLMRequest{})

	require.NoError(t, err)
	assert.NotNil(t, selected)
	assert.Equal(t, "r1", selected.ID)
}

// =============================================================================
// QualityWeightedStrategy Tests
// =============================================================================

func TestEnsembleUnit_QualityWeightedStrategy_Vote(t *testing.T) {
	strategy := &QualityWeightedStrategy{}

	responses := []*models.LLMResponse{
		{ID: "r1", Content: strings.Repeat("x", 200), Confidence: 0.7, ResponseTime: 500, TokensUsed: 50, FinishReason: "stop"},
		{ID: "r2", Content: strings.Repeat("y", 500), Confidence: 0.8, ResponseTime: 600, TokensUsed: 100, FinishReason: "stop"},
	}

	selected, _, err := strategy.Vote(responses, &models.LLMRequest{})

	require.NoError(t, err)
	assert.NotNil(t, selected)
}

func TestEnsembleUnit_QualityWeightedStrategy_Vote_NoResponses(t *testing.T) {
	strategy := &QualityWeightedStrategy{}

	responses := []*models.LLMResponse{}

	selected, _, err := strategy.Vote(responses, &models.LLMRequest{})

	assert.Error(t, err)
	assert.Nil(t, selected)
}

func TestEnsembleUnit_QualityWeightedStrategy_CalculateQualityScore(t *testing.T) {
	strategy := &QualityWeightedStrategy{}

	tests := []struct {
		name     string
		resp     *models.LLMResponse
		minScore float64
		maxScore float64
	}{
		{
			name: "high quality response",
			resp: &models.LLMResponse{
				Content:      strings.Repeat("x", 300),
				Confidence:   0.95,
				ResponseTime: 500,
				TokensUsed:   80,
				FinishReason: "stop",
			},
			minScore: 0.5,
			maxScore: 1.0,
		},
		{
			name: "low quality response",
			resp: &models.LLMResponse{
				Content:      "short",
				Confidence:   0.5,
				ResponseTime: 15000,
				TokensUsed:   200,
				FinishReason: "content_filter",
			},
			minScore: 0.0,
			maxScore: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := strategy.calculateQualityScore(tt.resp)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore)
		})
	}
}

// =============================================================================
// Legacy ProcessEnsemble Tests
// =============================================================================

func TestEnsembleUnit_ProcessEnsemble_NoEnsembleConfig(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	responses := []*models.LLMResponse{
		{ID: "r1", Content: "response1", Confidence: 0.8},
		{ID: "r2", Content: "response2", Confidence: 0.9},
	}

	selected, err := service.ProcessEnsemble(context.Background(), &models.LLMRequest{}, responses)

	require.NoError(t, err)
	assert.NotNil(t, selected)
	assert.Equal(t, "r2", selected.ID) // Highest confidence
}

func TestEnsembleUnit_ProcessEnsemble_ConfidenceWeighted(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	responses := []*models.LLMResponse{
		{ID: "r1", Content: "response1", Confidence: 0.7},
		{ID: "r2", Content: "response2", Confidence: 0.95},
	}

	selected, err := service.ProcessEnsemble(context.Background(), &models.LLMRequest{
		EnsembleConfig: &models.EnsembleConfig{
			Strategy:            "confidence_weighted",
			ConfidenceThreshold: 0.9,
			FallbackToBest:      true,
		},
	}, responses)

	require.NoError(t, err)
	assert.NotNil(t, selected)
	assert.True(t, selected.Selected)
}

func TestEnsembleUnit_ProcessEnsemble_MajorityVoting(t *testing.T) {
	service := NewEnsembleService("majority_vote", 30*time.Second)

	responses := []*models.LLMResponse{
		{ID: "r1", Content: "common answer", Confidence: 0.7},
		{ID: "r2", Content: "common answer", Confidence: 0.8},
		{ID: "r3", Content: "different", Confidence: 0.9},
	}

	selected, err := service.ProcessEnsemble(context.Background(), &models.LLMRequest{
		EnsembleConfig: &models.EnsembleConfig{
			Strategy:       "majority_vote",
			FallbackToBest: true,
		},
	}, responses)

	require.NoError(t, err)
	assert.NotNil(t, selected)
}

func TestEnsembleUnit_ProcessEnsemble_EmptyResponses(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	responses := []*models.LLMResponse{}

	selected, err := service.ProcessEnsemble(context.Background(), &models.LLMRequest{}, responses)

	require.NoError(t, err)
	assert.Nil(t, selected)
}

// =============================================================================
// Legacy Voting Method Tests
// =============================================================================

func TestEnsembleUnit_ConfidenceWeightedVoting(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	tests := []struct {
		name       string
		responses  []*models.LLMResponse
		config     *models.EnsembleConfig
		expectedID string
	}{
		{
			name: "above threshold",
			responses: []*models.LLMResponse{
				{ID: "r1", Confidence: 0.95},
				{ID: "r2", Confidence: 0.8},
			},
			config: &models.EnsembleConfig{
				ConfidenceThreshold: 0.9,
				FallbackToBest:      true,
			},
			expectedID: "r1",
		},
		{
			name: "below threshold with fallback",
			responses: []*models.LLMResponse{
				{ID: "r1", Confidence: 0.7},
				{ID: "r2", Confidence: 0.8},
			},
			config: &models.EnsembleConfig{
				ConfidenceThreshold: 0.9,
				FallbackToBest:      true,
			},
			expectedID: "r2",
		},
		{
			name: "below threshold without fallback",
			responses: []*models.LLMResponse{
				{ID: "r1", Confidence: 0.7},
			},
			config: &models.EnsembleConfig{
				ConfidenceThreshold: 0.9,
				FallbackToBest:      false,
			},
			expectedID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := service.confidenceWeightedVoting(tt.responses, tt.config)
			if tt.expectedID == "" {
				assert.Nil(t, selected)
			} else {
				assert.NotNil(t, selected)
				assert.Equal(t, tt.expectedID, selected.ID)
			}
		})
	}
}

func TestEnsembleUnit_MajorityVoting(t *testing.T) {
	service := NewEnsembleService("majority_vote", 30*time.Second)

	tests := []struct {
		name      string
		responses []*models.LLMResponse
		config    *models.EnsembleConfig
	}{
		{
			name: "true majority",
			responses: []*models.LLMResponse{
				{ID: "r1", Content: "answer A", Confidence: 0.8},
				{ID: "r2", Content: "answer A", Confidence: 0.7},
				{ID: "r3", Content: "answer B", Confidence: 0.9},
			},
			config: &models.EnsembleConfig{FallbackToBest: true},
		},
		{
			name: "no majority with fallback",
			responses: []*models.LLMResponse{
				{ID: "r1", Content: "A", Confidence: 0.9},
				{ID: "r2", Content: "B", Confidence: 0.8},
				{ID: "r3", Content: "C", Confidence: 0.7},
			},
			config: &models.EnsembleConfig{FallbackToBest: true},
		},
		{
			name: "single response",
			responses: []*models.LLMResponse{
				{ID: "r1", Content: "only", Confidence: 0.8},
			},
			config: &models.EnsembleConfig{},
		},
		{
			name:      "empty responses",
			responses: []*models.LLMResponse{},
			config:    &models.EnsembleConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := service.majorityVoting(tt.responses, tt.config)
			if len(tt.responses) == 0 {
				assert.Nil(t, selected)
			} else {
				assert.NotNil(t, selected)
			}
		})
	}
}

func TestEnsembleUnit_SelectBestResponse(t *testing.T) {
	service := NewEnsembleService("confidence_weighted", 30*time.Second)

	tests := []struct {
		name       string
		responses  []*models.LLMResponse
		expectedID string
	}{
		{
			name: "multiple responses",
			responses: []*models.LLMResponse{
				{ID: "r1", Confidence: 0.7},
				{ID: "r2", Confidence: 0.9},
				{ID: "r3", Confidence: 0.8},
			},
			expectedID: "r2",
		},
		{
			name: "single response",
			responses: []*models.LLMResponse{
				{ID: "r1", Confidence: 0.8},
			},
			expectedID: "r1",
		},
		{
			name:       "empty responses",
			responses:  []*models.LLMResponse{},
			expectedID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := service.selectBestResponse(tt.responses)
			if tt.expectedID == "" {
				assert.Nil(t, selected)
			} else {
				assert.NotNil(t, selected)
				assert.Equal(t, tt.expectedID, selected.ID)
				assert.True(t, selected.Selected)
			}
		})
	}
}
