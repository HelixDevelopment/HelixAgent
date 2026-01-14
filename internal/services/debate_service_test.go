package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
)

// debateMockLLMProvider implements llm.LLMProvider for testing
type debateMockLLMProvider struct {
	name           string
	response       *models.LLMResponse
	err            error
	delay          time.Duration
	callCount      int
	mu             sync.Mutex
	completeFunc   func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
	capabilities   *models.ProviderCapabilities
}

func newDebateMockProvider(name string, response *models.LLMResponse) *debateMockLLMProvider {
	return &debateMockLLMProvider{
		name:     name,
		response: response,
		capabilities: &models.ProviderCapabilities{
			SupportedModels:   []string{"test-model"},
			SupportsStreaming: true,
		},
	}
}

func (m *debateMockLLMProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}

	if m.err != nil {
		return nil, m.err
	}

	resp := *m.response
	resp.ProviderName = m.name
	resp.ProviderID = m.name
	return &resp, nil
}

func (m *debateMockLLMProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		if m.err != nil {
			return
		}
		resp := *m.response
		resp.ProviderName = m.name
		ch <- &resp
	}()
	return ch, nil
}

func (m *debateMockLLMProvider) HealthCheck() error {
	return nil
}

func (m *debateMockLLMProvider) GetCapabilities() *models.ProviderCapabilities {
	return m.capabilities
}

func (m *debateMockLLMProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

func (m *debateMockLLMProvider) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// Ensure debateMockLLMProvider implements llm.LLMProvider
var _ llm.LLMProvider = (*debateMockLLMProvider)(nil)

func newDebateSvcTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Silence logs in tests
	return log
}

// createTestProviderRegistry creates a provider registry with mock providers
func createTestProviderRegistry(providers map[string]*debateMockLLMProvider) *ProviderRegistry {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
	}
	registry := &ProviderRegistry{
		providers:       make(map[string]llm.LLMProvider),
		circuitBreakers: make(map[string]*CircuitBreaker),
		config:          cfg,
	}
	registry.ensemble = NewEnsembleService("confidence_weighted", cfg.DefaultTimeout)
	registry.requestService = NewRequestService("weighted", registry.ensemble, nil)

	for name, provider := range providers {
		registry.providers[name] = provider
	}

	return registry
}

// =============================================================================
// Basic Service Tests
// =============================================================================

func TestNewDebateService(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)
	require.NotNil(t, ds)
	assert.Equal(t, logger, ds.logger)
	assert.Nil(t, ds.providerRegistry)
	assert.Nil(t, ds.cogneeService)
}

func TestNewDebateServiceWithDeps(t *testing.T) {
	logger := newDebateSvcTestLogger()
	registry := createTestProviderRegistry(nil)

	ds := NewDebateServiceWithDeps(logger, registry, nil)
	require.NotNil(t, ds)
	assert.Equal(t, logger, ds.logger)
	assert.Equal(t, registry, ds.providerRegistry)
}

func TestDebateService_SetProviderRegistry(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	registry := createTestProviderRegistry(nil)
	ds.SetProviderRegistry(registry)

	assert.Equal(t, registry, ds.providerRegistry)
}

func TestDebateService_SetCogneeService(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	// Note: CogneeService requires a config, so we just check nil handling
	ds.SetCogneeService(nil)
	assert.Nil(t, ds.cogneeService)
}

// =============================================================================
// Provider Registry Validation Tests
// =============================================================================

func TestDebateService_ConductDebate_RequiresProviderRegistry(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	config := &DebateConfig{
		DebateID:  "test-debate-1",
		Topic:     "Test Topic",
		MaxRounds: 3,
		Timeout:   10 * time.Second,
		Participants: []ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Agent 1",
				Role:          "proposer",
				LLMProvider:   "openai",
				LLMModel:      "gpt-4",
			},
		},
		EnableCognee: false,
	}

	// Without provider registry, should return an error
	result, err := ds.ConductDebate(context.Background(), config)
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "provider registry is required")
}

func TestDebateService_ConductDebate_Basic(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Create mock providers for the participants
	mockProvider1 := newDebateMockProvider("openai", &models.LLMResponse{
		Content:      "This is my position on the test topic. First, I believe we should consider the key aspects. Therefore, my conclusion supports this view.",
		Confidence:   0.85,
		TokensUsed:   100,
		FinishReason: "stop",
	})
	mockProvider2 := newDebateMockProvider("anthropic", &models.LLMResponse{
		Content:      "I have a different perspective. Although there are valid points, I think we should also consider alternatives. Moreover, the long-term implications matter.",
		Confidence:   0.82,
		TokensUsed:   110,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"openai":    mockProvider1,
		"anthropic": mockProvider2,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "test-debate-1",
		Topic:     "Test Topic",
		MaxRounds: 3,
		Timeout:   10 * time.Second,
		Participants: []ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Agent 1",
				Role:          "proposer",
				LLMProvider:   "openai",
				LLMModel:      "gpt-4",
			},
			{
				ParticipantID: "participant-2",
				Name:          "Agent 2",
				Role:          "opponent",
				LLMProvider:   "anthropic",
				LLMModel:      "claude-3",
			},
		},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "test-debate-1", result.DebateID)
	assert.Equal(t, "Test Topic", result.Topic)
	assert.Equal(t, 3, result.TotalRounds)
	assert.True(t, result.RoundsConducted > 0)
	assert.True(t, result.Success)
	assert.True(t, result.QualityScore > 0)
	assert.True(t, result.FinalScore > 0)
	assert.NotEmpty(t, result.SessionID)
	assert.NotNil(t, result.Metadata)
}

func TestDebateService_ConductDebate_WithParticipants(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Create mock providers for multi-participant debate
	mockProviderOpenAI := newDebateMockProvider("openai", &models.LLMResponse{
		Content:      "First point from the proposer. I believe this is important because of several factors.",
		Confidence:   0.88,
		TokensUsed:   80,
		FinishReason: "stop",
	})
	mockProviderAnthropic := newDebateMockProvider("anthropic", &models.LLMResponse{
		Content:      "Critical analysis shows both strengths and weaknesses. However, we must consider alternatives.",
		Confidence:   0.85,
		TokensUsed:   90,
		FinishReason: "stop",
	})
	mockProviderOllama := newDebateMockProvider("ollama", &models.LLMResponse{
		Content:      "As a mediator, I see valid points on both sides. Therefore, let me summarize the key aspects.",
		Confidence:   0.82,
		TokensUsed:   85,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"openai":    mockProviderOpenAI,
		"anthropic": mockProviderAnthropic,
		"ollama":    mockProviderOllama,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "test-debate-2",
		Topic:     "Multi-participant debate",
		MaxRounds: 5,
		Timeout:   30 * time.Second,
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "First", Role: "proposer", LLMProvider: "openai", LLMModel: "gpt-4"},
			{ParticipantID: "p2", Name: "Second", Role: "critic", LLMProvider: "anthropic", LLMModel: "claude-3"},
			{ParticipantID: "p3", Name: "Third", Role: "mediator", LLMProvider: "ollama", LLMModel: "llama2"},
		},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify participants received responses
	assert.True(t, len(result.Participants) > 0 || len(result.AllResponses) > 0)
	assert.True(t, result.Success)
	assert.True(t, result.QualityScore > 0)
}

func TestDebateService_ConductDebate_WithConsensus(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Create a mock provider
	mockProvider := newDebateMockProvider("debater-provider", &models.LLMResponse{
		Content:      "I present my argument with clear evidence. First, consider the main point. Therefore, the conclusion is well-supported.",
		Confidence:   0.9,
		TokensUsed:   100,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"debater-provider": mockProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:     "test-debate-3",
		Topic:        "Consensus Test",
		MaxRounds:    2,
		Timeout:      5 * time.Second,
		Participants: []ParticipantConfig{{ParticipantID: "p1", Name: "Agent", Role: "debater", LLMProvider: "debater-provider", LLMModel: "test-model"}},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Consensus)

	// Verify consensus was analyzed (values depend on actual response analysis)
	assert.NotEmpty(t, result.Consensus.FinalPosition)
	assert.True(t, result.Consensus.ConsensusLevel >= 0)
	assert.True(t, result.Consensus.AgreementLevel >= 0)
}

func TestDebateService_ConductDebate_WithCogneeEnabled(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Create a mock provider
	mockProvider := newDebateMockProvider("ethics-provider", &models.LLMResponse{
		Content:      "AI Ethics is a crucial topic. First, we must consider fairness. Moreover, transparency is essential. Therefore, responsible AI development is key.",
		Confidence:   0.88,
		TokensUsed:   120,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"ethics-provider": mockProvider,
	})

	// Note: Without actual CogneeService, Cognee insights won't be generated
	// This tests that the debate still works with EnableCognee=true but no CogneeService
	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:     "test-debate-cognee",
		Topic:        "AI Ethics",
		MaxRounds:    1,
		Timeout:      5 * time.Second,
		Participants: []ParticipantConfig{{ParticipantID: "p1", Name: "Agent", Role: "debater", LLMProvider: "ethics-provider", LLMModel: "test-model"}},
		EnableCognee: true, // Enable Cognee enhancement
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Without CogneeService configured, Cognee enhancement won't be applied
	// but debate should still complete successfully
	assert.True(t, result.Success)
	assert.Equal(t, "AI Ethics", result.Topic)
	assert.True(t, len(result.AllResponses) > 0)
}

func TestDebateService_ConductDebate_WithEmptyParticipants(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Create an empty registry (no providers needed for empty participants)
	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:     "test-debate-empty",
		Topic:        "Empty Test",
		MaxRounds:    1,
		Timeout:      5 * time.Second,
		Participants: []ParticipantConfig{}, // No participants
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Empty(t, result.Participants)
	assert.NotNil(t, result.Consensus)
}

func TestDebateService_ConductDebate_DurationAndTiming(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Create a mock provider
	mockProvider := newDebateMockProvider("timing-provider", &models.LLMResponse{
		Content:      "Response for timing test. This is a well-formed response.",
		Confidence:   0.85,
		TokensUsed:   50,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"timing-provider": mockProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	timeout := 15 * time.Second
	config := &DebateConfig{
		DebateID:     "test-debate-timing",
		Topic:        "Timing Test",
		MaxRounds:    4,
		Timeout:      timeout,
		Participants: []ParticipantConfig{{ParticipantID: "p1", Name: "Agent", Role: "debater", LLMProvider: "timing-provider", LLMModel: "test-model"}},
		EnableCognee: false,
	}

	startTime := time.Now()
	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)

	// Verify timing - duration is calculated from actual execution
	assert.True(t, result.Duration > 0)
	assert.True(t, result.StartTime.After(startTime.Add(-time.Second)) || result.StartTime.Equal(startTime))
	assert.True(t, result.EndTime.After(result.StartTime))
}

// =============================================================================
// Real LLM Provider Tests (with mocks)
// =============================================================================

func TestDebateService_ConductRealDebate_SingleRound(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Create mock providers
	mockProvider1 := newDebateMockProvider("provider1", &models.LLMResponse{
		Content:      "This is a well-structured response about the topic. First, let me address the main points. However, there are some considerations to keep in mind. Therefore, my conclusion is that we should proceed carefully.",
		Confidence:   0.85,
		TokensUsed:   150,
		FinishReason: "stop",
	})

	mockProvider2 := newDebateMockProvider("provider2", &models.LLMResponse{
		Content:      "I agree with some points. Although there are challenges, the benefits are significant. Moreover, we should consider the long-term implications.",
		Confidence:   0.8,
		TokensUsed:   120,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"provider1": mockProvider1,
		"provider2": mockProvider2,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "real-debate-1",
		Topic:     "AI in Healthcare",
		MaxRounds: 1,
		Timeout:   30 * time.Second,
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Agent1", Role: "proposer", LLMProvider: "provider1", LLMModel: "test-model"},
			{ParticipantID: "p2", Name: "Agent2", Role: "opponent", LLMProvider: "provider2", LLMModel: "test-model"},
		},
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify real responses were used
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.RoundsConducted)
	assert.Len(t, result.Participants, 2)

	// Verify providers were called
	assert.Equal(t, 1, mockProvider1.getCallCount())
	assert.Equal(t, 1, mockProvider2.getCallCount())

	// Verify quality scores are calculated (not hardcoded)
	assert.NotEqual(t, 0.85, result.QualityScore) // Should be calculated
	assert.True(t, result.QualityScore > 0)
}

func TestDebateService_ConductRealDebate_MultipleRounds(t *testing.T) {
	logger := newDebateSvcTestLogger()

	callCount := 0
	mockProvider := &debateMockLLMProvider{
		name: "provider1",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			callCount++
			return &models.LLMResponse{
				Content:      "Round response with important key points. Furthermore, this is a comprehensive analysis.",
				Confidence:   0.9,
				TokensUsed:   100,
				FinishReason: "stop",
				ProviderName: "provider1",
			}, nil
		},
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"test-model"},
		},
	}

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"provider1": mockProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "real-debate-multi",
		Topic:     "Climate Change Solutions",
		MaxRounds: 3,
		Timeout:   60 * time.Second,
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Agent1", Role: "proposer", LLMProvider: "provider1", LLMModel: "test-model"},
		},
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have 3 rounds
	assert.Equal(t, 3, result.RoundsConducted)
	assert.Equal(t, 3, callCount) // One call per round

	// All responses should be in AllResponses
	assert.Len(t, result.AllResponses, 3)
}

func TestDebateService_ConductRealDebate_ProviderError(t *testing.T) {
	logger := newDebateSvcTestLogger()

	mockProvider := &debateMockLLMProvider{
		name: "failing-provider",
		err:  errors.New("provider failed"),
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"test-model"},
		},
	}

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"failing-provider": mockProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "error-debate",
		Topic:     "Test",
		MaxRounds: 1,
		Timeout:   5 * time.Second,
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Agent", Role: "debater", LLMProvider: "failing-provider", LLMModel: "test-model"},
		},
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err) // Should not error, but have empty responses
	require.NotNil(t, result)

	// No responses due to error
	assert.Empty(t, result.AllResponses)
	assert.False(t, result.Success)
}

func TestDebateService_ConductRealDebate_MixedProviderSuccess(t *testing.T) {
	logger := newDebateSvcTestLogger()

	successProvider := newDebateMockProvider("success", &models.LLMResponse{
		Content:      "This is a successful response with key insights. The main point is important.",
		Confidence:   0.85,
		TokensUsed:   80,
		FinishReason: "stop",
	})

	failingProvider := &debateMockLLMProvider{
		name: "failing",
		err:  errors.New("failed"),
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"test-model"},
		},
	}

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"success": successProvider,
		"failing": failingProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "mixed-debate",
		Topic:     "Mixed Results",
		MaxRounds: 1,
		Timeout:   10 * time.Second,
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Success", Role: "proposer", LLMProvider: "success", LLMModel: "test-model"},
			{ParticipantID: "p2", Name: "Failure", Role: "opponent", LLMProvider: "failing", LLMModel: "test-model"},
		},
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have one successful response
	assert.Len(t, result.AllResponses, 1)
	assert.True(t, result.Success)
	assert.Equal(t, "Success", result.AllResponses[0].ParticipantName)
}

func TestDebateService_ConductRealDebate_Timeout(t *testing.T) {
	logger := newDebateSvcTestLogger()

	slowProvider := &debateMockLLMProvider{
		name:  "slow",
		delay: 5 * time.Second, // Longer than timeout
		response: &models.LLMResponse{
			Content:    "Slow response",
			Confidence: 0.8,
		},
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"test-model"},
		},
	}

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"slow": slowProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "timeout-debate",
		Topic:     "Timeout Test",
		MaxRounds: 1,
		Timeout:   100 * time.Millisecond, // Very short timeout
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Slow", Role: "debater", LLMProvider: "slow", LLMModel: "test-model"},
		},
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should timeout with no responses
	assert.Empty(t, result.AllResponses)
}

func TestDebateService_ConductRealDebate_ProviderNotFound(t *testing.T) {
	logger := newDebateSvcTestLogger()

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "notfound-debate",
		Topic:     "Test",
		MaxRounds: 1,
		Timeout:   5 * time.Second,
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Agent", Role: "debater", LLMProvider: "nonexistent", LLMModel: "test-model"},
		},
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Empty(t, result.AllResponses)
	assert.False(t, result.Success)
}

// =============================================================================
// Quality Score Calculation Tests
// =============================================================================

func TestDebateService_CalculateResponseQuality(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	tests := []struct {
		name     string
		response *models.LLMResponse
		minScore float64
		maxScore float64
	}{
		{
			name: "high quality response",
			response: &models.LLMResponse{
				Content:      "This is a comprehensive response. First, let me explain. However, there are considerations. Therefore, the conclusion is clear. Furthermore, additional points support this view.",
				Confidence:   0.9,
				TokensUsed:   50,
				FinishReason: "stop",
			},
			minScore: 0.6,
			maxScore: 1.0,
		},
		{
			name: "low quality response",
			response: &models.LLMResponse{
				Content:      "Short.",
				Confidence:   0.3,
				TokensUsed:   100,
				FinishReason: "content_filter",
			},
			minScore: 0.1,
			maxScore: 0.5,
		},
		{
			name: "medium quality response",
			response: &models.LLMResponse{
				Content:      "This is a moderate response with some detail. It covers the topic adequately.",
				Confidence:   0.7,
				TokensUsed:   30,
				FinishReason: "stop",
			},
			minScore: 0.4,
			maxScore: 0.8,
		},
		{
			name: "no confidence response",
			response: &models.LLMResponse{
				Content:      "Response without confidence value. This has some content to analyze.",
				Confidence:   0.0,
				TokensUsed:   20,
				FinishReason: "stop",
			},
			minScore: 0.3,
			maxScore: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ds.calculateResponseQuality(tt.response)
			assert.GreaterOrEqual(t, score, tt.minScore, "Score should be >= %f", tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore, "Score should be <= %f", tt.maxScore)
		})
	}
}

func TestDebateService_CalculateQualityScore_EmptyResponses(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	score := ds.calculateQualityScore([]ParticipantResponse{})
	assert.Equal(t, 0.0, score)
}

func TestDebateService_CalculateFinalScore(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	responses := []ParticipantResponse{
		{ParticipantID: "p1", QualityScore: 0.8},
		{ParticipantID: "p2", QualityScore: 0.7},
	}

	consensus := &ConsensusResult{
		AgreementLevel: 0.75,
	}

	score := ds.calculateFinalScore(responses, consensus)
	assert.Greater(t, score, 0.0)
	assert.LessOrEqual(t, score, 1.0)
}

// =============================================================================
// Coherence Analysis Tests
// =============================================================================

func TestDebateService_AnalyzeCoherence(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	tests := []struct {
		name     string
		content  string
		minScore float64
	}{
		{
			name:     "empty content",
			content:  "",
			minScore: 0.0,
		},
		{
			name:     "simple content",
			content:  "Hello world.",
			minScore: 0.4,
		},
		{
			name:     "structured content",
			content:  "First, let me explain. Second, consider this. Third, the conclusion. Therefore, we can see. However, there are alternatives.",
			minScore: 0.7,
		},
		{
			name:     "well structured",
			content:  "First, the main point. However, we must consider. Therefore, my conclusion. In conclusion, the evidence shows. Furthermore, additional support.",
			minScore: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ds.analyzeCoherence(tt.content)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, 1.0)
		})
	}
}

// =============================================================================
// Consensus Analysis Tests
// =============================================================================

func TestDebateService_AnalyzeConsensus_EmptyResponses(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	consensus := ds.analyzeConsensus([]ParticipantResponse{}, "Test Topic")

	assert.False(t, consensus.Reached)
	assert.False(t, consensus.Achieved)
	assert.Equal(t, 0.0, consensus.Confidence)
	assert.Equal(t, "No responses to analyze", consensus.FinalPosition)
}

func TestDebateService_AnalyzeConsensus_SingleResponse(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	responses := []ParticipantResponse{
		{
			ParticipantID:   "p1",
			ParticipantName: "Agent1",
			Content:         "This is a comprehensive response about the topic.",
			QualityScore:    0.8,
		},
	}

	consensus := ds.analyzeConsensus(responses, "Test Topic")

	assert.True(t, consensus.Reached) // Single response is self-consistent
	assert.Equal(t, 1.0, consensus.AgreementLevel)
}

func TestDebateService_AnalyzeConsensus_SimilarResponses(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	responses := []ParticipantResponse{
		{
			ParticipantID:   "p1",
			ParticipantName: "Agent1",
			Content:         "The climate change issue requires immediate action. We need renewable energy.",
			QualityScore:    0.8,
		},
		{
			ParticipantID:   "p2",
			ParticipantName: "Agent2",
			Content:         "Climate change is urgent. Renewable energy is the key action we need to take.",
			QualityScore:    0.85,
		},
	}

	consensus := ds.analyzeConsensus(responses, "Climate Change")

	assert.Greater(t, consensus.AgreementLevel, 0.3) // Some agreement expected
	assert.NotEmpty(t, consensus.Summary)
}

func TestDebateService_AnalyzeConsensus_DifferentResponses(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	responses := []ParticipantResponse{
		{
			ParticipantID:   "p1",
			ParticipantName: "Agent1",
			Content:         "Artificial intelligence will revolutionize healthcare through diagnostics.",
			QualityScore:    0.8,
		},
		{
			ParticipantID:   "p2",
			ParticipantName: "Agent2",
			Content:         "Economic policies should focus on reducing unemployment rates in rural areas.",
			QualityScore:    0.85,
		},
	}

	consensus := ds.analyzeConsensus(responses, "Random Topic")

	// Very different topics should have low agreement
	assert.Less(t, consensus.AgreementLevel, 0.5)
}

func TestDebateService_CalculateAgreementScore(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	// Test with empty responses (< 2 means self-consistent)
	score := ds.calculateAgreementScore([]ParticipantResponse{})
	assert.Equal(t, 1.0, score)

	// Test with single response (returns 1.0 as self-consistent)
	score = ds.calculateAgreementScore([]ParticipantResponse{
		{Content: "Single response"},
	})
	assert.Equal(t, 1.0, score)

	// Test with two identical responses (should be high agreement)
	score = ds.calculateAgreementScore([]ParticipantResponse{
		{Content: "Identical response here"},
		{Content: "Identical response here"},
	})
	assert.Greater(t, score, 0.9)

	// Test with different responses
	score = ds.calculateAgreementScore([]ParticipantResponse{
		{Content: "Apple banana cherry grape"},
		{Content: "Computer mouse keyboard monitor"},
	})
	assert.Less(t, score, 0.3)
}

func TestDebateService_CalculateTextSimilarity(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	tests := []struct {
		name  string
		text1 string
		text2 string
		min   float64
		max   float64
	}{
		{
			name:  "identical texts",
			text1: "hello world",
			text2: "hello world",
			min:   0.9,
			max:   1.0,
		},
		{
			name:  "completely different",
			text1: "apple orange banana",
			text2: "computer keyboard mouse",
			min:   0.0,
			max:   0.1,
		},
		{
			name:  "partial overlap",
			text1: "hello world test",
			text2: "hello universe test",
			min:   0.3,
			max:   0.7,
		},
		{
			name:  "empty first",
			text1: "",
			text2: "hello",
			min:   0.0,
			max:   0.0,
		},
		{
			name:  "empty second",
			text1: "hello",
			text2: "",
			min:   0.0,
			max:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ds.calculateTextSimilarity(tt.text1, tt.text2)
			assert.GreaterOrEqual(t, score, tt.min)
			assert.LessOrEqual(t, score, tt.max)
		})
	}
}

// =============================================================================
// Key Points and Disagreements Tests
// =============================================================================

func TestDebateService_ExtractKeyPoints(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	responses := []ParticipantResponse{
		{
			Content: "The most important aspect is efficiency. The key factor is scalability.",
		},
		{
			Content: "An essential point to consider is security. The main concern is reliability.",
		},
	}

	keyPoints := ds.extractKeyPoints(responses)
	assert.NotEmpty(t, keyPoints)
	assert.LessOrEqual(t, len(keyPoints), 5)
}

func TestDebateService_ExtractKeyPoints_NoIndicators(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	responses := []ParticipantResponse{
		{
			Content: "This is just a regular sentence without any markers.",
		},
	}

	keyPoints := ds.extractKeyPoints(responses)
	// Should extract first sentences as fallback
	assert.NotEmpty(t, keyPoints)
}

func TestDebateService_ExtractDisagreements(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	responses := []ParticipantResponse{
		{
			Content: "I agree with the premise. However, there are issues to consider. I disagree with the conclusion.",
		},
		{
			Content: "On the other hand, we should think differently. But the data shows otherwise.",
		},
	}

	disagreements := ds.extractDisagreements(responses)
	assert.NotEmpty(t, disagreements)
	assert.LessOrEqual(t, len(disagreements), 5)
}

// =============================================================================
// Sentiment Analysis Tests
// =============================================================================

func TestDebateService_AnalyzeSentiment(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "positive content",
			content:  "This is excellent! I agree this is a great and beneficial approach. The advantages are clear.",
			expected: "positive",
		},
		{
			name:     "negative content",
			content:  "This is a bad idea. I disagree strongly. The problem is serious and harmful.",
			expected: "negative",
		},
		{
			name:     "neutral content",
			content:  "Let us consider this from a different perspective. Although there are factors to examine.",
			expected: "neutral",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentiment := ds.analyzeSentiment(tt.content)
			assert.Equal(t, tt.expected, sentiment)
		})
	}
}

func TestDebateService_GetOverallSentiment(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	tests := []struct {
		name      string
		responses []ParticipantResponse
		expected  string
	}{
		{
			name: "mostly positive",
			responses: []ParticipantResponse{
				{Content: "This is great and excellent!"},
				{Content: "I agree, very good approach."},
				{Content: "Some considerations here."},
			},
			expected: "positive",
		},
		{
			name: "mostly negative",
			responses: []ParticipantResponse{
				{Content: "This is a bad problem."},
				{Content: "I disagree, harmful approach."},
				{Content: "Some points here."},
			},
			expected: "negative",
		},
		{
			name: "mixed neutral",
			responses: []ParticipantResponse{
				{Content: "Consider this perspective."},
				{Content: "Although we should examine."},
			},
			expected: "neutral",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentiment := ds.getOverallSentiment(tt.responses)
			assert.Equal(t, tt.expected, sentiment)
		})
	}
}

func TestDebateService_CalculateSentimentScore(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	// Empty responses
	score := ds.calculateSentimentScore([]ParticipantResponse{})
	assert.Equal(t, 0.5, score)

	// Positive responses
	positiveResponses := []ParticipantResponse{
		{Content: "Excellent and great!"},
	}
	score = ds.calculateSentimentScore(positiveResponses)
	assert.Equal(t, 0.8, score)
}

// =============================================================================
// Innovation Score Tests
// =============================================================================

func TestDebateService_CalculateInnovationScore(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	tests := []struct {
		name      string
		responses []ParticipantResponse
		minScore  float64
	}{
		{
			name:      "empty responses",
			responses: []ParticipantResponse{},
			minScore:  0.0,
		},
		{
			name: "innovative content",
			responses: []ParticipantResponse{
				{Content: "This novel approach offers a unique perspective. The innovative solution is creative."},
			},
			minScore: 0.3,
		},
		{
			name: "standard content",
			responses: []ParticipantResponse{
				{Content: "The standard approach is straightforward. We should follow established practices."},
			},
			minScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ds.calculateInnovationScore(tt.responses)
			assert.GreaterOrEqual(t, score, tt.minScore)
		})
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestDebateService_FindBestResponse(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	// Empty responses
	best := ds.findBestResponse([]ParticipantResponse{})
	assert.Nil(t, best)

	// Multiple responses
	responses := []ParticipantResponse{
		{ParticipantID: "p1", QualityScore: 0.7},
		{ParticipantID: "p2", QualityScore: 0.9},
		{ParticipantID: "p3", QualityScore: 0.8},
	}
	best = ds.findBestResponse(responses)
	require.NotNil(t, best)
	assert.Equal(t, "p2", best.ParticipantID)
}

func TestDebateService_GetLatestParticipantResponses(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	allResponses := []ParticipantResponse{
		{ParticipantID: "p1", Round: 1, ParticipantName: "Agent1"},
		{ParticipantID: "p1", Round: 2, ParticipantName: "Agent1"},
		{ParticipantID: "p2", Round: 1, ParticipantName: "Agent2"},
	}

	participants := []ParticipantConfig{
		{ParticipantID: "p1", Name: "Agent1"},
		{ParticipantID: "p2", Name: "Agent2"},
	}

	latest := ds.getLatestParticipantResponses(allResponses, participants)

	require.Len(t, latest, 2)
	assert.Equal(t, 2, latest[0].Round) // p1's latest round
	assert.Equal(t, 1, latest[1].Round) // p2's only round
}

func TestDebateService_GetUniqueProviders(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	responses := []ParticipantResponse{
		{LLMProvider: "openai"},
		{LLMProvider: "anthropic"},
		{LLMProvider: "openai"},
	}

	providers := ds.getUniqueProviders(responses)
	assert.Len(t, providers, 2)
}

func TestDebateService_CalculateAvgResponseTime(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	// Empty responses
	avg := ds.calculateAvgResponseTime([]ParticipantResponse{})
	assert.Equal(t, time.Duration(0), avg)

	// Multiple responses
	responses := []ParticipantResponse{
		{ResponseTime: 2 * time.Second},
		{ResponseTime: 4 * time.Second},
	}
	avg = ds.calculateAvgResponseTime(responses)
	assert.Equal(t, 3*time.Second, avg)
}

func TestDebateService_CheckEarlyConsensus(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	// Less than 2 responses
	result := ds.checkEarlyConsensus([]ParticipantResponse{
		{Content: "Single response"},
	})
	assert.False(t, result)

	// Similar responses (high agreement)
	result = ds.checkEarlyConsensus([]ParticipantResponse{
		{Content: "The solution is clear and effective."},
		{Content: "The solution is clear and effective."},
	})
	assert.True(t, result)

	// Different responses (low agreement)
	result = ds.checkEarlyConsensus([]ParticipantResponse{
		{Content: "Apple banana cherry."},
		{Content: "Computer keyboard mouse."},
	})
	assert.False(t, result)
}

func TestDebateService_GetVoteDistribution(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	responses := []ParticipantResponse{
		{ParticipantName: "Agent1"},
		{ParticipantName: "Agent1"},
		{ParticipantName: "Agent2"},
	}

	dist := ds.getVoteDistribution(responses)
	assert.Equal(t, 2, dist["Agent1"])
	assert.Equal(t, 1, dist["Agent2"])
}

func TestDebateService_GetWinner(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	// Empty responses
	winner := ds.getWinner([]ParticipantResponse{})
	assert.Empty(t, winner)

	// Multiple responses
	responses := []ParticipantResponse{
		{ParticipantName: "Agent1", QualityScore: 0.7},
		{ParticipantName: "Agent2", QualityScore: 0.9},
	}
	winner = ds.getWinner(responses)
	assert.Equal(t, "Agent2", winner)
}

func TestDebateService_GenerateFinalPosition(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	tests := []struct {
		name             string
		responses        []ParticipantResponse
		consensusReached bool
		expected         string
	}{
		{
			name:             "empty responses",
			responses:        []ParticipantResponse{},
			consensusReached: false,
			expected:         "No position established",
		},
		{
			name:             "consensus reached",
			responses:        []ParticipantResponse{{Content: "test"}},
			consensusReached: true,
			expected:         "Consensus reached: Participants found common ground on the key aspects of the topic",
		},
		{
			name:             "no consensus",
			responses:        []ParticipantResponse{{Content: "test"}},
			consensusReached: false,
			expected:         "Discussion ongoing: Multiple perspectives presented with varying viewpoints",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ds.generateFinalPosition(tt.responses, tt.consensusReached)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDebateService_GenerateRecommendations(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	// High quality
	recs := ds.generateRecommendations([]ParticipantResponse{}, 0.8)
	assert.Contains(t, recs, "Consider diverse perspectives")

	// Low quality - should have more recommendations
	recs = ds.generateRecommendations([]ParticipantResponse{}, 0.3)
	assert.Contains(t, recs, "Increase response depth and detail")

	// Few responses
	recs = ds.generateRecommendations([]ParticipantResponse{{}, {}}, 0.8)
	assert.Contains(t, recs, "Consider additional debate rounds")
}

// =============================================================================
// Prompt Building Tests
// =============================================================================

func TestDebateService_BuildSystemPrompt(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	tests := []struct {
		name        string
		participant ParticipantConfig
		contains    []string
	}{
		{
			name:        "proposer role",
			participant: ParticipantConfig{Name: "TestAgent", Role: "proposer"},
			contains:    []string{"TestAgent", "proposer", "presenting and defending"},
		},
		{
			name:        "opponent role",
			participant: ParticipantConfig{Name: "TestAgent", Role: "opponent"},
			contains:    []string{"TestAgent", "opponent", "challenging"},
		},
		{
			name:        "unknown role",
			participant: ParticipantConfig{Name: "TestAgent", Role: "unknown"},
			contains:    []string{"TestAgent", "AI debate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := ds.buildSystemPrompt(tt.participant)
			for _, expected := range tt.contains {
				assert.Contains(t, prompt, expected)
			}
		})
	}
}

func TestDebateService_BuildDebatePrompt(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	participant := ParticipantConfig{Name: "Agent1", Role: "proposer"}

	// First round (no previous responses)
	prompt := ds.buildDebatePrompt("Test Topic", participant, 1, nil)
	assert.Contains(t, prompt, "Test Topic")
	assert.Contains(t, prompt, "ROUND: 1")
	assert.Contains(t, prompt, "opening round")

	// Second round (with previous responses)
	previousResponses := []ParticipantResponse{
		{ParticipantName: "Agent2", Role: "opponent", Round: 1, Content: "Previous response content."},
	}
	prompt = ds.buildDebatePrompt("Test Topic", participant, 2, previousResponses)
	assert.Contains(t, prompt, "ROUND: 2")
	assert.Contains(t, prompt, "PREVIOUS RESPONSES")
	assert.Contains(t, prompt, "Agent2")
	assert.Contains(t, prompt, "Previous response content")
}

// =============================================================================
// Min Helper Function Test
// =============================================================================

func TestMin(t *testing.T) {
	assert.Equal(t, 1, min(1, 2))
	assert.Equal(t, 1, min(2, 1))
	assert.Equal(t, 5, min(5, 5))
	assert.Equal(t, -1, min(-1, 0))
}

// =============================================================================
// Context Cancellation Test
// =============================================================================

func TestDebateService_ConductRealDebate_ContextCancellation(t *testing.T) {
	logger := newDebateSvcTestLogger()

	mockProvider := &debateMockLLMProvider{
		name:  "slow",
		delay: 2 * time.Second,
		response: &models.LLMResponse{
			Content:    "Response",
			Confidence: 0.8,
		},
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"test-model"},
		},
	}

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"slow": mockProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	ctx, cancel := context.WithCancel(context.Background())

	config := &DebateConfig{
		DebateID:  "cancel-debate",
		Topic:     "Cancellation Test",
		MaxRounds: 5,
		Timeout:   30 * time.Second,
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Agent", Role: "debater", LLMProvider: "slow", LLMModel: "test-model"},
		},
	}

	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result, err := ds.ConductDebate(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have early termination
	assert.Less(t, result.RoundsConducted, 5)
}

// =============================================================================
// Concurrent Provider Access Test
// =============================================================================

func TestDebateService_ConductRealDebate_ConcurrentProviders(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Create multiple providers that all respond successfully
	providers := make(map[string]*debateMockLLMProvider)
	for i := 1; i <= 5; i++ {
		name := "provider" + string(rune('0'+i))
		providers[name] = newDebateMockProvider(name, &models.LLMResponse{
			Content:      "Response from provider with important key points.",
			Confidence:   0.8,
			TokensUsed:   50,
			FinishReason: "stop",
		})
	}

	registry := createTestProviderRegistry(providers)
	ds := NewDebateServiceWithDeps(logger, registry, nil)

	participants := make([]ParticipantConfig, 0, 5)
	for i := 1; i <= 5; i++ {
		name := "provider" + string(rune('0'+i))
		participants = append(participants, ParticipantConfig{
			ParticipantID: "p" + string(rune('0'+i)),
			Name:          "Agent" + string(rune('0'+i)),
			Role:          "debater",
			LLMProvider:   name,
			LLMModel:      "test-model",
		})
	}

	config := &DebateConfig{
		DebateID:     "concurrent-debate",
		Topic:        "Concurrent Test",
		MaxRounds:    1,
		Timeout:      30 * time.Second,
		Participants: participants,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// All 5 providers should have responded
	assert.Len(t, result.AllResponses, 5)

	// All providers should have been called once
	for name, provider := range providers {
		assert.Equal(t, 1, provider.getCallCount(), "Provider %s call count", name)
	}
}

// =============================================================================
// Cognee Integration Tests
// =============================================================================

func TestDebateService_AnalyzeWithCognee_NoCogneeService(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	// Without cognee service, should return error
	analysis, err := ds.analyzeWithCognee(context.Background(), "Test content for analysis")

	assert.Error(t, err)
	assert.Nil(t, analysis)
	assert.Contains(t, err.Error(), "cognee service not configured")
}

func TestDebateService_GenerateCogneeInsights_NoCogneeService(t *testing.T) {
	logger := newDebateSvcTestLogger()
	ds := NewDebateService(logger)

	config := &DebateConfig{
		Topic: "Test Topic",
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Agent1"},
		},
	}

	responses := []ParticipantResponse{
		{ParticipantID: "p1", Content: "Test response content"},
	}

	// Without cognee service, should return error
	insights, err := ds.generateCogneeInsights(context.Background(), config, responses)

	assert.Error(t, err)
	assert.Nil(t, insights)
	assert.Contains(t, err.Error(), "cognee service not configured")
}

// =============================================================================
// Fallback Chain Tests - CRITICAL: These tests ensure fallback mechanism works
// =============================================================================

// TestDebateService_FallbackChain_EmptyResponse tests that empty responses trigger fallback
func TestDebateService_FallbackChain_EmptyResponse(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Primary provider returns empty response
	primaryProvider := newDebateMockProvider("primary", &models.LLMResponse{
		Content:      "", // EMPTY - should trigger fallback
		Confidence:   0.0,
		TokensUsed:   0,
		FinishReason: "stop",
	})

	// Fallback provider returns valid response
	fallbackProvider := newDebateMockProvider("fallback", &models.LLMResponse{
		Content:      "This is a valid response from the fallback provider. It contains meaningful content for the debate.",
		Confidence:   0.85,
		TokensUsed:   50,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"primary":  primaryProvider,
		"fallback": fallbackProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "fallback-test-1",
		Topic:     "Test Fallback Mechanism",
		MaxRounds: 1,
		Timeout:   10 * time.Second,
		Participants: []ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Agent 1",
				Role:          "proposer",
				LLMProvider:   "primary",
				LLMModel:      "test-model",
				Fallbacks: []FallbackConfig{
					{Provider: "fallback", Model: "fallback-model"},
				},
			},
		},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify fallback was used
	assert.Len(t, result.AllResponses, 1)
	response := result.AllResponses[0]
	assert.Contains(t, response.Content, "valid response from the fallback")

	// CRITICAL: Verify fallback notice is highlighted in response content
	assert.Contains(t, response.Content, "[FALLBACK ACTIVATED:", "Response should contain fallback notice")
	assert.Contains(t, response.Content, "Primary primary/test-model unavailable", "Should show original provider")
	assert.Contains(t, response.Content, "response from fallback/fallback-model", "Should show fallback provider")

	// Verify metadata shows fallback was used
	if fallbackUsed, ok := response.Metadata["fallback_used"]; ok {
		assert.True(t, fallbackUsed.(bool), "Fallback should be marked as used")
	}

	// Primary should have been called once (empty response)
	assert.Equal(t, 1, primaryProvider.getCallCount(), "Primary provider should be called once")
	// Fallback should have been called once
	assert.Equal(t, 1, fallbackProvider.getCallCount(), "Fallback provider should be called once")
}

// TestDebateService_FallbackChain_WhitespaceOnlyResponse tests whitespace-only responses
func TestDebateService_FallbackChain_WhitespaceOnlyResponse(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Primary returns only whitespace
	primaryProvider := newDebateMockProvider("primary", &models.LLMResponse{
		Content:      "   \n\t  \n  ", // Only whitespace - should trigger fallback
		Confidence:   0.0,
		TokensUsed:   0,
		FinishReason: "stop",
	})

	// Fallback provider returns valid response
	fallbackProvider := newDebateMockProvider("fallback", &models.LLMResponse{
		Content:      "Valid content from fallback after whitespace-only primary response.",
		Confidence:   0.85,
		TokensUsed:   50,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"primary":  primaryProvider,
		"fallback": fallbackProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "whitespace-fallback-test",
		Topic:     "Test Whitespace Fallback",
		MaxRounds: 1,
		Timeout:   10 * time.Second,
		Participants: []ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Agent 1",
				Role:          "proposer",
				LLMProvider:   "primary",
				LLMModel:      "test-model",
				Fallbacks: []FallbackConfig{
					{Provider: "fallback", Model: "fallback-model"},
				},
			},
		},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Len(t, result.AllResponses, 1)
	response := result.AllResponses[0]
	assert.Contains(t, response.Content, "Valid content from fallback")
}

// TestDebateService_FallbackChain_PrimaryError tests error from primary triggers fallback
func TestDebateService_FallbackChain_PrimaryError(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Primary provider returns an error
	primaryProvider := &debateMockLLMProvider{
		name: "primary",
		err:  errors.New("rate limit exceeded"),
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"test-model"},
		},
	}

	// Fallback provider works
	fallbackProvider := newDebateMockProvider("fallback", &models.LLMResponse{
		Content:      "Response from fallback after primary error.",
		Confidence:   0.80,
		TokensUsed:   40,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"primary":  primaryProvider,
		"fallback": fallbackProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "error-fallback-test",
		Topic:     "Test Error Fallback",
		MaxRounds: 1,
		Timeout:   10 * time.Second,
		Participants: []ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Agent 1",
				Role:          "proposer",
				LLMProvider:   "primary",
				LLMModel:      "test-model",
				Fallbacks: []FallbackConfig{
					{Provider: "fallback", Model: "fallback-model"},
				},
			},
		},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Len(t, result.AllResponses, 1)
	response := result.AllResponses[0]
	assert.Contains(t, response.Content, "Response from fallback")
}

// TestDebateService_FallbackChain_MultipleFallbacks tests fallback chain with multiple options
func TestDebateService_FallbackChain_MultipleFallbacks(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Primary fails
	primaryProvider := &debateMockLLMProvider{
		name: "primary",
		err:  errors.New("connection timeout"),
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"test-model"},
		},
	}

	// First fallback also fails
	fallback1Provider := &debateMockLLMProvider{
		name: "fallback1",
		err:  errors.New("service unavailable"),
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"fallback1-model"},
		},
	}

	// Second fallback succeeds
	fallback2Provider := newDebateMockProvider("fallback2", &models.LLMResponse{
		Content:      "Response from second fallback after two failures.",
		Confidence:   0.75,
		TokensUsed:   35,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"primary":   primaryProvider,
		"fallback1": fallback1Provider,
		"fallback2": fallback2Provider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "multi-fallback-test",
		Topic:     "Test Multiple Fallbacks",
		MaxRounds: 1,
		Timeout:   15 * time.Second,
		Participants: []ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Agent 1",
				Role:          "proposer",
				LLMProvider:   "primary",
				LLMModel:      "test-model",
				Fallbacks: []FallbackConfig{
					{Provider: "fallback1", Model: "fallback1-model"},
					{Provider: "fallback2", Model: "fallback2-model"},
				},
			},
		},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Len(t, result.AllResponses, 1)
	response := result.AllResponses[0]
	assert.Contains(t, response.Content, "second fallback")

	// Verify fallback metadata
	if fallbackIndex, ok := response.Metadata["fallback_index"]; ok {
		assert.Equal(t, 2, fallbackIndex.(int), "Should be second fallback")
	}

	// Verify all providers were tried
	assert.Equal(t, 1, primaryProvider.getCallCount(), "Primary should be called once")
	assert.Equal(t, 1, fallback1Provider.getCallCount(), "Fallback1 should be called once")
	assert.Equal(t, 1, fallback2Provider.getCallCount(), "Fallback2 should be called once")
}

// TestDebateService_FallbackChain_AllFallbacksFail tests when all fallbacks fail
func TestDebateService_FallbackChain_AllFallbacksFail(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// All providers fail
	primaryProvider := &debateMockLLMProvider{
		name: "primary",
		err:  errors.New("primary failed"),
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"test-model"},
		},
	}

	fallbackProvider := &debateMockLLMProvider{
		name: "fallback",
		err:  errors.New("fallback also failed"),
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"fallback-model"},
		},
	}

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"primary":  primaryProvider,
		"fallback": fallbackProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "all-fail-test",
		Topic:     "Test All Fallbacks Fail",
		MaxRounds: 1,
		Timeout:   10 * time.Second,
		Participants: []ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Agent 1",
				Role:          "proposer",
				LLMProvider:   "primary",
				LLMModel:      "test-model",
				Fallbacks: []FallbackConfig{
					{Provider: "fallback", Model: "fallback-model"},
				},
			},
		},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	// When all fallbacks fail, debate returns result with Success=false (graceful degradation)
	require.NoError(t, err) // No error returned, but result indicates failure
	require.NotNil(t, result)
	assert.False(t, result.Success, "Debate should report failure when all providers fail")
	assert.Len(t, result.AllResponses, 0, "No responses should be collected when all providers fail")

	// Verify all providers were tried
	assert.Equal(t, 1, primaryProvider.getCallCount(), "Primary should be called once")
	assert.Equal(t, 1, fallbackProvider.getCallCount(), "Fallback should be called once")
}

// TestDebateService_FallbackChain_NoFallbacksConfigured tests behavior without fallbacks
func TestDebateService_FallbackChain_NoFallbacksConfigured(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// Primary returns empty, no fallbacks configured
	primaryProvider := newDebateMockProvider("primary", &models.LLMResponse{
		Content:      "", // Empty response
		Confidence:   0.0,
		TokensUsed:   0,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"primary": primaryProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "no-fallback-test",
		Topic:     "Test No Fallbacks",
		MaxRounds: 1,
		Timeout:   10 * time.Second,
		Participants: []ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Agent 1",
				Role:          "proposer",
				LLMProvider:   "primary",
				LLMModel:      "test-model",
				// No Fallbacks configured
			},
		},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	// Without fallbacks and empty primary response, debate returns Success=false (graceful degradation)
	require.NoError(t, err) // No error returned, but result indicates failure
	require.NotNil(t, result)
	assert.False(t, result.Success, "Debate should report failure when primary fails without fallbacks")
	assert.Len(t, result.AllResponses, 0, "No responses should be collected when primary fails and no fallbacks")
}

// TestDebateService_FallbackChain_MultipleParticipants tests fallback with multiple participants
func TestDebateService_FallbackChain_MultipleParticipants(t *testing.T) {
	logger := newDebateSvcTestLogger()

	// First participant's primary works
	provider1 := newDebateMockProvider("provider1", &models.LLMResponse{
		Content:      "Valid response from first participant.",
		Confidence:   0.90,
		TokensUsed:   30,
		FinishReason: "stop",
	})

	// Second participant's primary fails, fallback works
	provider2Primary := &debateMockLLMProvider{
		name: "provider2",
		err:  errors.New("provider2 temporarily unavailable"),
		capabilities: &models.ProviderCapabilities{
			SupportedModels: []string{"test-model"},
		},
	}

	provider2Fallback := newDebateMockProvider("provider2fallback", &models.LLMResponse{
		Content:      "Response from second participant's fallback.",
		Confidence:   0.85,
		TokensUsed:   40,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"provider1":         provider1,
		"provider2":         provider2Primary,
		"provider2fallback": provider2Fallback,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "multi-participant-fallback-test",
		Topic:     "Test Multiple Participants with Fallback",
		MaxRounds: 1,
		Timeout:   15 * time.Second,
		Participants: []ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Agent 1",
				Role:          "proposer",
				LLMProvider:   "provider1",
				LLMModel:      "model1",
			},
			{
				ParticipantID: "participant-2",
				Name:          "Agent 2",
				Role:          "opponent",
				LLMProvider:   "provider2",
				LLMModel:      "model2",
				Fallbacks: []FallbackConfig{
					{Provider: "provider2fallback", Model: "fallback-model"},
				},
			},
		},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Both participants should have responded
	assert.Len(t, result.AllResponses, 2)
}

// TestDebateService_FallbackChain_FallbackMetadata tests that fallback metadata is properly set
func TestDebateService_FallbackChain_FallbackMetadata(t *testing.T) {
	logger := newDebateSvcTestLogger()

	primaryProvider := newDebateMockProvider("deepseek", &models.LLMResponse{
		Content:      "", // Empty - triggers fallback
		Confidence:   0.0,
		TokensUsed:   0,
		FinishReason: "stop",
	})

	fallbackProvider := newDebateMockProvider("mistral", &models.LLMResponse{
		Content:      "Fallback response with proper metadata tracking.",
		Confidence:   0.88,
		TokensUsed:   45,
		FinishReason: "stop",
	})

	registry := createTestProviderRegistry(map[string]*debateMockLLMProvider{
		"deepseek": primaryProvider,
		"mistral":  fallbackProvider,
	})

	ds := NewDebateServiceWithDeps(logger, registry, nil)

	config := &DebateConfig{
		DebateID:  "metadata-test",
		Topic:     "Test Fallback Metadata",
		MaxRounds: 1,
		Timeout:   10 * time.Second,
		Participants: []ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "The Mediator",
				Role:          "mediator",
				LLMProvider:   "deepseek",
				LLMModel:      "deepseek-chat",
				Fallbacks: []FallbackConfig{
					{Provider: "mistral", Model: "mistral-large"},
				},
			},
		},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Len(t, result.AllResponses, 1)
	response := result.AllResponses[0]

	// Verify fallback metadata
	assert.True(t, response.Metadata["fallback_used"].(bool), "fallback_used should be true")
	assert.Equal(t, 1, response.Metadata["fallback_index"].(int), "fallback_index should be 1")
	assert.Equal(t, "deepseek", response.Metadata["original_provider"].(string), "original_provider should be deepseek")
	assert.Equal(t, "deepseek-chat", response.Metadata["original_model"].(string), "original_model should be deepseek-chat")

	// Verify fallback notice is in the response content for user visibility
	assert.Contains(t, response.Content, "[FALLBACK ACTIVATED:", "Response should highlight fallback usage")
	assert.Contains(t, response.Content, "deepseek/deepseek-chat unavailable", "Should show original provider/model")
	assert.Contains(t, response.Content, "mistral/mistral-large", "Should show fallback provider/model")
}
