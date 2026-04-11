package bigdata

import (
	"context"
	"fmt"
	"testing"
	"time"

	"dev.helix.agent/internal/services"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- DebateServiceWrapper Tests ---

func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return logger
}

func TestDebateServiceWrapper_NewDebateServiceWrapper_ReturnsValidInstance(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	assert.NotNil(t, wrapper)
	assert.Nil(t, wrapper.debateService)
	assert.Nil(t, wrapper.debateIntegration)
	assert.Nil(t, wrapper.analyticsIntegration)
	assert.Nil(t, wrapper.entityIntegration)
	assert.Equal(t, logger, wrapper.logger)
	assert.False(t, wrapper.enableBigData)
}

func TestDebateServiceWrapper_NewDebateServiceWrapper_WithBigDataEnabled(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, true)

	assert.NotNil(t, wrapper)
	assert.True(t, wrapper.enableBigData)
}

func TestDebateServiceWrapper_RecordProviderCall_BigDataDisabled(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)
	ctx := context.Background()

	// Should not panic when big data is disabled
	wrapper.RecordProviderCall(
		ctx, "claude", "claude-3", "req-001",
		100*time.Millisecond, 500, true, "",
	)
}

func TestDebateServiceWrapper_RecordProviderCall_BigDataEnabledNilAnalytics(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, true)
	ctx := context.Background()

	// Should not panic when analyticsIntegration is nil
	wrapper.RecordProviderCall(
		ctx, "deepseek", "deepseek-v3", "req-002",
		200*time.Millisecond, 1000, false, "timeout",
	)
}

func TestDebateServiceWrapper_RecordDebateRound_BigDataDisabled(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)
	ctx := context.Background()

	// Should not panic when big data is disabled
	wrapper.RecordDebateRound(
		ctx, "debate-001", "gemini", "gemini-pro",
		1, 500*time.Millisecond, 200, 0.85,
	)
}

func TestDebateServiceWrapper_RecordDebateRound_BigDataEnabledNilAnalytics(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, true)
	ctx := context.Background()

	// Should not panic when analyticsIntegration is nil
	wrapper.RecordDebateRound(
		ctx, "debate-002", "mistral", "mistral-large",
		2, 300*time.Millisecond, 150, 0.92,
	)
}

func TestDebateServiceWrapper_ExtractProviderFromWinner_WithSlash(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	tests := []struct {
		name     string
		winner   string
		expected string
	}{
		{"provider/model", "claude/claude-3-opus", "claude"},
		{"deepseek format", "deepseek/deepseek-v3", "deepseek"},
		{"no slash", "claude", "claude"},
		{"empty string", "", ""},
		{"slash at end", "provider/", "provider"},
		{"multiple slashes", "a/b/c", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := wrapper.extractProviderFromWinner(tt.winner)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDebateServiceWrapper_ExtractModelFromWinner_WithSlash(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	tests := []struct {
		name     string
		winner   string
		expected string
	}{
		{"provider/model", "claude/claude-3-opus", "claude-3-opus"},
		{"deepseek format", "deepseek/deepseek-v3", "deepseek-v3"},
		{"no slash", "claude", ""},
		{"empty string", "", ""},
		{"slash at end", "provider/", ""},
		{"multiple slashes", "a/b/c", "b/c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := wrapper.extractModelFromWinner(tt.winner)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDebateServiceWrapper_DetermineOutcome_Error(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	result := &services.DebateResult{
		ErrorMessage: "something went wrong",
		Success:      false,
	}
	assert.Equal(t, "error", wrapper.determineOutcome(result))
}

func TestDebateServiceWrapper_DetermineOutcome_Abandoned(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	result := &services.DebateResult{
		ErrorMessage: "",
		Success:      false,
	}
	assert.Equal(t, "abandoned", wrapper.determineOutcome(result))
}

func TestDebateServiceWrapper_DetermineOutcome_Successful(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	result := &services.DebateResult{
		ErrorMessage: "",
		Success:      true,
	}
	assert.Equal(t, "successful", wrapper.determineOutcome(result))
}

func TestDebateServiceWrapper_CalculateTotalTokens_Empty(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	total := wrapper.calculateTotalTokens(nil)
	assert.Equal(t, 0, total)
}

func TestDebateServiceWrapper_CalculateTotalTokens_WithParticipants(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	participants := []services.ParticipantResponse{
		{Response: "This is a response of about forty characters."}, // ~11 tokens
		{Response: "Short"}, // ~1 token
		{Response: "Another response that has some content in it."}, // ~11 tokens
	}

	total := wrapper.calculateTotalTokens(participants)
	// Each response length / 4
	expected := len(participants[0].Response)/4 +
		len(participants[1].Response)/4 +
		len(participants[2].Response)/4
	assert.Equal(t, expected, total)
}

func TestDebateServiceWrapper_ConvertParticipants_Empty(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	result := wrapper.convertParticipants(nil, "")
	assert.Len(t, result, 0)
}

func TestDebateServiceWrapper_ConvertParticipants_WithData(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	participants := []services.ParticipantResponse{
		{
			ParticipantName: "claude-advocate",
			LLMProvider:     "claude",
			LLMModel:        "claude-3-opus",
			Role:            "advocate",
			ResponseTime:    1500 * time.Millisecond,
			Confidence:      0.92,
			Metadata: map[string]any{
				"tokens_used": 500,
			},
		},
		{
			ParticipantName: "deepseek-critic",
			LLMProvider:     "deepseek",
			LLMModel:        "deepseek-v3",
			Role:            "critic",
			ResponseTime:    2000 * time.Millisecond,
			Confidence:      0.88,
			Metadata: map[string]any{
				"tokens_used": 600.0, // float64 from JSON
			},
		},
	}

	result := wrapper.convertParticipants(participants, "claude-advocate")
	require.Len(t, result, 2)

	assert.Equal(t, "claude", result[0].Provider)
	assert.Equal(t, "claude-3-opus", result[0].Model)
	assert.Equal(t, "advocate", result[0].Position)
	assert.Equal(t, 1500, result[0].ResponseTime)
	assert.Equal(t, 500, result[0].TokensUsed)
	assert.InDelta(t, 0.92, result[0].Confidence, 0.001)
	assert.True(t, result[0].Won)

	assert.Equal(t, "deepseek", result[1].Provider)
	assert.Equal(t, 600, result[1].TokensUsed) // float64 converted to int
	assert.False(t, result[1].Won)
}

func TestDebateServiceWrapper_ConvertParticipants_NoMetadataTokens(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	participants := []services.ParticipantResponse{
		{
			ParticipantName: "test",
			LLMProvider:     "test-provider",
			LLMModel:        "test-model",
			Role:            "advocate",
			ResponseTime:    100 * time.Millisecond,
			Confidence:      0.5,
			Metadata:        nil,
		},
	}

	result := wrapper.convertParticipants(participants, "nonexistent")
	require.Len(t, result, 1)
	assert.Equal(t, 0, result[0].TokensUsed)
	assert.False(t, result[0].Won)
}

func TestDebateServiceWrapper_ConvertParticipants_EmptyWinnerName(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	participants := []services.ParticipantResponse{
		{
			ParticipantName: "test",
			LLMProvider:     "provider",
			LLMModel:        "model",
			Role:            "advocate",
			ResponseTime:    100 * time.Millisecond,
		},
	}

	result := wrapper.convertParticipants(participants, "")
	require.Len(t, result, 1)
	assert.False(t, result[0].Won) // No winner when empty string
}

func TestDebateServiceWrapper_RunDebate_NilDebateService(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	// bigData disabled: passthrough to debateService.ConductDebate which is nil
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)
	ctx := context.Background()

	config := &services.DebateConfig{
		Topic:    "test",
		Metadata: map[string]any{},
	}

	// debateService is nil, should panic
	assert.Panics(t, func() {
		_, _ = wrapper.RunDebate(ctx, config)
	})
}

func TestDebateServiceWrapper_DetermineOutcome_ErrorTakesPrecedence(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, false)

	// When both ErrorMessage is set and Success is true, error takes precedence
	result := &services.DebateResult{
		ErrorMessage: "partial error",
		Success:      true,
	}
	assert.Equal(t, "error", wrapper.determineOutcome(result))
}

// --- RecordProviderCall with real analytics integration ---

func TestDebateServiceWrapper_RecordProviderCall_WithAnalytics(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	analytics := NewAnalyticsIntegration(broker, logger, true)

	wrapper := NewDebateServiceWrapper(nil, nil, analytics, nil, logger, true)
	ctx := context.Background()

	wrapper.RecordProviderCall(
		ctx, "claude", "claude-3-opus", "req-001",
		150*time.Millisecond, 800, true, "",
	)

	// Verify that the analytics event was published
	msgs := broker.getPublished()
	assert.Len(t, msgs, 1)
	assert.Equal(t, "helixagent.analytics.providers", msgs[0].topic)
}

func TestDebateServiceWrapper_RecordProviderCall_WithAnalyticsFailure(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	analytics := NewAnalyticsIntegration(broker, logger, true)

	wrapper := NewDebateServiceWrapper(nil, nil, analytics, nil, logger, true)
	ctx := context.Background()

	wrapper.RecordProviderCall(
		ctx, "deepseek", "deepseek-v3", "req-fail",
		5*time.Second, 0, false, "timeout",
	)

	msgs := broker.getPublished()
	assert.Len(t, msgs, 1)
}

func TestDebateServiceWrapper_RecordProviderCall_WithAnalyticsBrokerError(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("broker down")
	analytics := NewAnalyticsIntegration(broker, logger, true)

	wrapper := NewDebateServiceWrapper(nil, nil, analytics, nil, logger, true)
	ctx := context.Background()

	// Should not panic even if broker publish fails
	wrapper.RecordProviderCall(
		ctx, "gemini", "gemini-pro", "req-err",
		100*time.Millisecond, 200, true, "",
	)
}

// --- RecordDebateRound with real analytics integration ---

func TestDebateServiceWrapper_RecordDebateRound_WithAnalytics(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	analytics := NewAnalyticsIntegration(broker, logger, true)

	wrapper := NewDebateServiceWrapper(nil, nil, analytics, nil, logger, true)
	ctx := context.Background()

	wrapper.RecordDebateRound(
		ctx, "debate-001", "claude", "claude-3-opus",
		1, 500*time.Millisecond, 200, 0.85,
	)

	msgs := broker.getPublished()
	assert.Len(t, msgs, 1)
	assert.Equal(t, "helixagent.analytics.providers", msgs[0].topic)
}

func TestDebateServiceWrapper_RecordDebateRound_WithAnalyticsBrokerError(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("publish error")
	analytics := NewAnalyticsIntegration(broker, logger, true)

	wrapper := NewDebateServiceWrapper(nil, nil, analytics, nil, logger, true)
	ctx := context.Background()

	// Should not panic
	wrapper.RecordDebateRound(
		ctx, "debate-err", "mistral", "mistral-large",
		2, 300*time.Millisecond, 150, 0.92,
	)
}

// --- publishDebateCompletion tests ---

func TestDebateServiceWrapper_PublishDebateCompletion_Success(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	debateIntegration := NewDebateIntegration(nil, broker, logger)
	analyticsIntegration := NewAnalyticsIntegration(broker, logger, true)

	wrapper := NewDebateServiceWrapper(
		nil, debateIntegration, analyticsIntegration, nil, logger, true,
	)

	config := &services.DebateConfig{
		Topic: "test debate",
		Metadata: map[string]any{
			"conversation_id": "conv-123",
			"user_id":         "user-456",
			"session_id":      "sess-789",
		},
	}

	result := &services.DebateResult{
		DebateID:    "debate-001",
		TotalRounds: 3,
		Success:     true,
		StartTime:   time.Now().Add(-5 * time.Second),
		EndTime:     time.Now(),
		Consensus: &services.ConsensusResult{
			FinalPosition: "claude/claude-3-opus",
			Confidence:    0.95,
			VotingSummary: services.VotingSummary{
				Winner: "claude-advocate",
			},
		},
		Participants: []services.ParticipantResponse{
			{
				ParticipantName: "claude-advocate",
				LLMProvider:     "claude",
				LLMModel:        "claude-3-opus",
				Role:            "advocate",
				Response:        "I advocate for this position because it is well supported.",
				ResponseTime:    1500 * time.Millisecond,
				Confidence:      0.95,
			},
			{
				ParticipantName: "deepseek-critic",
				LLMProvider:     "deepseek",
				LLMModel:        "deepseek-v3",
				Role:            "critic",
				Response:        "I disagree with the position taken.",
				ResponseTime:    2000 * time.Millisecond,
				Confidence:      0.82,
			},
		},
	}

	duration := 5 * time.Second

	// Call publishDebateCompletion directly (it's a private method)
	wrapper.publishDebateCompletion(context.Background(), config, result, duration)

	// Should have published debate completion + debate metrics
	msgs := broker.getPublished()
	assert.GreaterOrEqual(t, len(msgs), 2)
}

func TestDebateServiceWrapper_PublishDebateCompletion_NoConsensus(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	debateIntegration := NewDebateIntegration(nil, broker, logger)

	wrapper := NewDebateServiceWrapper(
		nil, debateIntegration, nil, nil, logger, true,
	)

	config := &services.DebateConfig{
		Topic:    "test debate",
		Metadata: map[string]any{},
	}

	result := &services.DebateResult{
		DebateID:     "debate-no-consensus",
		TotalRounds:  3,
		Success:      false,
		ErrorMessage: "",
		StartTime:    time.Now(),
		EndTime:      time.Now(),
	}

	wrapper.publishDebateCompletion(context.Background(), config, result, 1*time.Second)

	msgs := broker.getPublished()
	assert.GreaterOrEqual(t, len(msgs), 1)
}

func TestDebateServiceWrapper_PublishDebateCompletion_WithError(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	debateIntegration := NewDebateIntegration(nil, broker, logger)

	wrapper := NewDebateServiceWrapper(
		nil, debateIntegration, nil, nil, logger, true,
	)

	config := &services.DebateConfig{
		Topic:    "test debate",
		Metadata: map[string]any{},
	}

	result := &services.DebateResult{
		DebateID:     "debate-err",
		TotalRounds:  1,
		Success:      false,
		ErrorMessage: "provider timeout",
		StartTime:    time.Now(),
		EndTime:      time.Now(),
	}

	wrapper.publishDebateCompletion(context.Background(), config, result, 1*time.Second)

	msgs := broker.getPublished()
	assert.GreaterOrEqual(t, len(msgs), 1)
}

func TestDebateServiceWrapper_PublishDebateCompletion_NoMetadata(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	debateIntegration := NewDebateIntegration(nil, broker, logger)

	wrapper := NewDebateServiceWrapper(
		nil, debateIntegration, nil, nil, logger, true,
	)

	config := &services.DebateConfig{
		Topic:    "test debate",
		Metadata: nil,
	}

	result := &services.DebateResult{
		DebateID:  "debate-no-meta",
		Success:   true,
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	// Should not panic with nil metadata
	wrapper.publishDebateCompletion(context.Background(), config, result, 1*time.Second)
}

func TestDebateServiceWrapper_PublishDebateCompletion_BrokerError(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("kafka unavailable")
	debateIntegration := NewDebateIntegration(nil, broker, logger)

	wrapper := NewDebateServiceWrapper(
		nil, debateIntegration, nil, nil, logger, true,
	)

	config := &services.DebateConfig{
		Topic:    "test debate",
		Metadata: map[string]any{},
	}

	result := &services.DebateResult{
		DebateID:  "debate-broker-err",
		Success:   true,
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	// Should not panic despite broker error
	wrapper.publishDebateCompletion(context.Background(), config, result, 1*time.Second)
}

func TestDebateServiceWrapper_RunDebate_BigDataEnabledNoConversationID(t *testing.T) {
	t.Parallel()
	// When bigData enabled but no conversation_id in metadata, it should skip
	// context loading and proceed to ConductDebate.
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, true)
	ctx := context.Background()

	config := &services.DebateConfig{
		Topic:    "test",
		Metadata: map[string]any{"user_id": "user-1"},
	}

	// debateService is nil, so ConductDebate should panic
	assert.Panics(t, func() {
		_, _ = wrapper.RunDebate(ctx, config)
	})
}

func TestDebateServiceWrapper_RunDebate_BigDataEnabledNilMetadata(t *testing.T) {
	t.Parallel()
	// When metadata is nil, the type assertion should return empty string
	// for conversation_id. ConductDebate will be called.
	logger := newTestLogger()
	wrapper := NewDebateServiceWrapper(nil, nil, nil, nil, logger, true)
	ctx := context.Background()

	config := &services.DebateConfig{
		Topic:    "test",
		Metadata: nil, // nil metadata
	}

	// Should panic on ConductDebate since debateService is nil,
	// but metadata extraction should not panic
	assert.Panics(t, func() {
		_, _ = wrapper.RunDebate(ctx, config)
	})
}

func TestDebateServiceWrapper_RunDebate_BigDataEnabledWithConversationID(t *testing.T) {
	t.Parallel()
	// When bigData enabled and conversation_id exists, it tries GetConversationContext.
	// With nil debateIntegration, this will panic since it calls
	// debateIntegration.GetConversationContext which dereferences nil.
	logger := newTestLogger()
	broker := newMockBroker()
	debateIntegration := NewDebateIntegration(nil, broker, logger)
	wrapper := NewDebateServiceWrapper(nil, debateIntegration, nil, nil, logger, true)
	ctx := context.Background()

	config := &services.DebateConfig{
		Topic: "test",
		Metadata: map[string]any{
			"conversation_id": "conv-test",
		},
	}

	// GetConversationContext will panic because infiniteContext is nil
	// But the code has a Warn log and continues - let's check
	// Actually, looking at the code: debateIntegration.GetConversationContext
	// dereferences infiniteContext directly, which panics.
	assert.Panics(t, func() {
		_, _ = wrapper.RunDebate(ctx, config)
	})
}

func TestDebateServiceWrapper_RunDebate_BigDataEnabledGetContextFails(t *testing.T) {
	t.Parallel()
	// Use a real InfiniteContextEngine so GetConversationContext doesn't panic
	// but instead returns an error (Kafka unavailable). RunDebate should log
	// a warning and continue to ConductDebate (which panics because debateService is nil).
	logger := newTestLogger()
	broker := newMockBroker()

	integration := newTestIntegrationWithInfiniteContext()
	infiniteCtx := integration.GetInfiniteContext()
	require.NotNil(t, infiniteCtx)

	debateIntegration := NewDebateIntegration(infiniteCtx, broker, logger)
	wrapper := NewDebateServiceWrapper(nil, debateIntegration, nil, nil, logger, true)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	config := &services.DebateConfig{
		Topic: "test topic",
		Metadata: map[string]any{
			"conversation_id": "conv-that-wont-be-found",
		},
	}

	// GetConversationContext will fail (Kafka not available), RunDebate logs warning
	// and proceeds to ConductDebate which panics (debateService is nil)
	assert.Panics(t, func() {
		_, _ = wrapper.RunDebate(ctx, config)
	})
}

func TestDebateServiceWrapper_RunDebate_ConductDebateReturnsError(t *testing.T) {
	t.Parallel()
	// Use a real DebateService without provider registry so ConductDebate
	// returns an error (not a panic). This tests the error path at lines 76-77.
	logger := newTestLogger()
	debateService := services.NewDebateService(logger)
	wrapper := NewDebateServiceWrapper(debateService, nil, nil, nil, logger, false)

	config := &services.DebateConfig{
		Topic:    "test debate",
		Metadata: map[string]any{},
	}

	result, err := wrapper.RunDebate(context.Background(), config)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "provider registry is required")
}

func TestDebateServiceWrapper_RunDebate_BigDataEnabled_ConductDebateError(t *testing.T) {
	t.Parallel()
	// With bigData enabled but no conversation_id, it skips context loading
	// and proceeds to ConductDebate which returns an error.
	logger := newTestLogger()
	debateService := services.NewDebateService(logger)
	wrapper := NewDebateServiceWrapper(debateService, nil, nil, nil, logger, true)

	config := &services.DebateConfig{
		Topic:    "test debate",
		Metadata: map[string]any{"user_id": "u1"},
	}

	result, err := wrapper.RunDebate(context.Background(), config)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "provider registry is required")
}

func TestDebateServiceWrapper_RunDebate_BigDataEnabled_ContextFailsThenDebateError(t *testing.T) {
	t.Parallel()
	// With bigData enabled, conversation_id set, context fetch fails (Kafka),
	// then ConductDebate returns error (no provider registry). This tests
	// the full "fail gracefully + continue" path without panics.
	logger := newTestLogger()
	broker := newMockBroker()
	debateService := services.NewDebateService(logger)

	integration := newTestIntegrationWithInfiniteContext()
	infiniteCtx := integration.GetInfiniteContext()
	require.NotNil(t, infiniteCtx)

	debateIntegration := NewDebateIntegration(infiniteCtx, broker, logger)
	analyticsIntegration := NewAnalyticsIntegration(broker, logger, true)
	wrapper := NewDebateServiceWrapper(debateService, debateIntegration, analyticsIntegration, nil, logger, true)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	config := &services.DebateConfig{
		Topic: "context fail test",
		Metadata: map[string]any{
			"conversation_id": "conv-nonexistent",
		},
	}

	result, err := wrapper.RunDebate(ctx, config)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "provider registry is required")
}

func TestDebateServiceWrapper_PublishDebateCompletion_ConsensusWithVotingWinner(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()
	broker := newMockBroker()
	debateIntegration := NewDebateIntegration(nil, broker, logger)
	analyticsIntegration := NewAnalyticsIntegration(broker, logger, true)

	wrapper := NewDebateServiceWrapper(
		nil, debateIntegration, analyticsIntegration, nil, logger, true,
	)

	config := &services.DebateConfig{
		Topic: "AI safety",
		Metadata: map[string]any{
			"conversation_id": "conv-winner",
			"user_id":         "user-winner",
			"session_id":      "sess-winner",
		},
	}

	result := &services.DebateResult{
		DebateID:    "debate-winner",
		TotalRounds: 5,
		Success:     true,
		StartTime:   time.Now().Add(-10 * time.Second),
		EndTime:     time.Now(),
		Consensus: &services.ConsensusResult{
			FinalPosition: "safety first",
			Confidence:    0.92,
			VotingSummary: services.VotingSummary{
				Winner: "claude-advocate",
			},
		},
		Participants: []services.ParticipantResponse{
			{
				ParticipantName: "claude-advocate",
				LLMProvider:     "claude",
				LLMModel:        "claude-3-opus",
				Role:            "advocate",
				Response:        "Safety is paramount.",
				ResponseTime:    1000 * time.Millisecond,
				Confidence:      0.95,
				Metadata: map[string]any{
					"tokens_used": 500,
				},
			},
			{
				ParticipantName: "deepseek-critic",
				LLMProvider:     "deepseek",
				LLMModel:        "deepseek-v3",
				Role:            "critic",
				Response:        "But progress matters too.",
				ResponseTime:    1200 * time.Millisecond,
				Confidence:      0.85,
				Metadata: map[string]any{
					"tokens_used": 400.0, // float64
				},
			},
		},
	}

	wrapper.publishDebateCompletion(context.Background(), config, result, 10*time.Second)

	msgs := broker.getPublished()
	// Should have published at least debate completion + analytics
	assert.GreaterOrEqual(t, len(msgs), 2)
}
