package bigdata

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NewAnalyticsIntegration tests ---

func TestAnalyticsIntegration_New_Enabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	logger := newTestLogger()

	ai := NewAnalyticsIntegration(broker, logger, true)
	require.NotNil(t, ai)
	assert.True(t, ai.enabled)
	assert.Equal(t, broker, ai.kafkaBroker)
	assert.Equal(t, logger, ai.logger)
}

func TestAnalyticsIntegration_New_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	logger := newTestLogger()

	ai := NewAnalyticsIntegration(broker, logger, false)
	require.NotNil(t, ai)
	assert.False(t, ai.enabled)
}

// --- PublishProviderMetrics tests ---

func TestAnalyticsIntegration_PublishProviderMetrics_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	metrics := &ProviderMetrics{
		Provider:       "claude",
		Model:          "claude-3-opus",
		RequestID:      "req-123",
		Timestamp:      time.Now(),
		ResponseTimeMs: 250.5,
		TokensUsed:     1500,
		Success:        true,
	}

	err := ai.PublishProviderMetrics(context.Background(), metrics)
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.analytics.providers", published[0].topic)
	assert.Equal(t, "provider.metrics", published[0].message.Type)
	assert.Equal(t, "provider.metrics", published[0].message.Headers["event_type"])
}

func TestAnalyticsIntegration_PublishProviderMetrics_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), false)

	metrics := &ProviderMetrics{
		Provider:  "claude",
		Model:     "claude-3-opus",
		Timestamp: time.Now(),
	}

	err := ai.PublishProviderMetrics(context.Background(), metrics)
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestAnalyticsIntegration_PublishProviderMetrics_BrokerError(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("connection refused")
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	metrics := &ProviderMetrics{
		Provider:  "deepseek",
		Model:     "deepseek-v2",
		Timestamp: time.Now(),
	}

	err := ai.PublishProviderMetrics(context.Background(), metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kafka publish failed")
}

// --- PublishDebateMetrics tests ---

func TestAnalyticsIntegration_PublishDebateMetrics_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	metrics := &DebateMetrics{
		DebateID:         "debate-456",
		Topic:            "climate change",
		Timestamp:        time.Now(),
		TotalRounds:      3,
		TotalDurationMs:  5000.0,
		ParticipantCount: 5,
		Winner:           "claude",
		WinnerProvider:   "claude",
		WinnerModel:      "claude-3-opus",
		Confidence:       0.92,
		TotalTokens:      10000,
		Outcome:          "successful",
	}

	err := ai.PublishDebateMetrics(context.Background(), metrics)
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.analytics.debates", published[0].topic)
	assert.Equal(t, "debate.metrics", published[0].message.Type)
}

func TestAnalyticsIntegration_PublishDebateMetrics_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), false)

	metrics := &DebateMetrics{
		DebateID:  "debate-789",
		Timestamp: time.Now(),
	}

	err := ai.PublishDebateMetrics(context.Background(), metrics)
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestAnalyticsIntegration_PublishDebateMetrics_BrokerError(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("topic not found")
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	metrics := &DebateMetrics{
		DebateID:  "debate-err",
		Timestamp: time.Now(),
		Outcome:   "error",
	}

	err := ai.PublishDebateMetrics(context.Background(), metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kafka publish failed")
}

// --- PublishConversationMetrics tests ---

func TestAnalyticsIntegration_PublishConversationMetrics_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	metrics := &ConversationMetrics{
		ConversationID:   "conv-001",
		UserID:           "user-42",
		SessionID:        "sess-99",
		Timestamp:        time.Now(),
		MessageCount:     25,
		EntityCount:      8,
		TotalTokens:      5000,
		Compressed:       true,
		CompressionRatio: 0.65,
		DebateCount:      2,
	}

	err := ai.PublishConversationMetrics(context.Background(), metrics)
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.analytics.conversations", published[0].topic)
	assert.Equal(t, "conversation.metrics", published[0].message.Type)
}

func TestAnalyticsIntegration_PublishConversationMetrics_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), false)

	metrics := &ConversationMetrics{
		ConversationID: "conv-disabled",
		Timestamp:      time.Now(),
	}

	err := ai.PublishConversationMetrics(context.Background(), metrics)
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

// --- RecordProviderRequest tests ---

func TestAnalyticsIntegration_RecordProviderRequest_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	err := ai.RecordProviderRequest(
		context.Background(),
		"gemini", "gemini-pro", "req-gp-1",
		200*time.Millisecond,
		800,
		true,
		"",
	)
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.analytics.providers", published[0].topic)
}

func TestAnalyticsIntegration_RecordProviderRequest_Failure(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	err := ai.RecordProviderRequest(
		context.Background(),
		"openrouter", "model-x", "req-fail-1",
		5*time.Second,
		0,
		false,
		"timeout",
	)
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
}

func TestAnalyticsIntegration_RecordProviderRequest_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), false)

	err := ai.RecordProviderRequest(
		context.Background(),
		"gemini", "gemini-pro", "req-dis",
		100*time.Millisecond, 100, true, "",
	)
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

// --- RecordDebateRound tests ---

func TestAnalyticsIntegration_RecordDebateRound_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	err := ai.RecordDebateRound(
		context.Background(),
		"debate-100", "mistral", "mistral-large",
		2,
		350*time.Millisecond,
		1200,
		0.87,
	)
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.analytics.providers", published[0].topic)
}

func TestAnalyticsIntegration_RecordDebateRound_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), false)

	err := ai.RecordDebateRound(
		context.Background(),
		"debate-dis", "mistral", "mistral-large",
		1, 100*time.Millisecond, 500, 0.5,
	)
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

// --- RecordDebateCompletion tests ---

func TestAnalyticsIntegration_RecordDebateCompletion_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	err := ai.RecordDebateCompletion(
		context.Background(),
		"debate-200", "AI safety",
		5,
		10*time.Second,
		3,
		"claude", "claude", "claude-3-opus",
		0.95,
		15000,
		"successful",
	)
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.analytics.debates", published[0].topic)
}

func TestAnalyticsIntegration_RecordDebateCompletion_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), false)

	err := ai.RecordDebateCompletion(
		context.Background(),
		"debate-dis", "topic", 1, time.Second, 2,
		"w", "wp", "wm", 0.5, 100, "abandoned",
	)
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestAnalyticsIntegration_RecordDebateCompletion_BrokerError(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("broker down")
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	err := ai.RecordDebateCompletion(
		context.Background(),
		"debate-err", "topic", 1, time.Second, 2,
		"w", "wp", "wm", 0.5, 100, "error",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kafka publish failed")
}

// --- RecordConversation tests ---

func TestAnalyticsIntegration_RecordConversation_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	err := ai.RecordConversation(
		context.Background(),
		"conv-300", "user-1", "sess-1",
		50, 12, 8000, true, 0.6, 3,
	)
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.analytics.conversations", published[0].topic)
}

func TestAnalyticsIntegration_RecordConversation_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), false)

	err := ai.RecordConversation(
		context.Background(),
		"conv-dis", "u", "s", 1, 0, 100, false, 0.0, 0,
	)
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestAnalyticsIntegration_RecordConversation_BrokerError(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("timeout")
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	err := ai.RecordConversation(
		context.Background(),
		"conv-err", "u", "s", 1, 0, 100, false, 0.0, 0,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kafka publish failed")
}

// --- BatchPublishProviderMetrics tests ---

func TestAnalyticsIntegration_BatchPublishProviderMetrics_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	batch := []*ProviderMetrics{
		{
			Provider:  "claude",
			Model:     "claude-3-opus",
			Timestamp: time.Now(),
			Success:   true,
		},
		{
			Provider:  "gemini",
			Model:     "gemini-pro",
			Timestamp: time.Now(),
			Success:   true,
		},
		{
			Provider:  "deepseek",
			Model:     "deepseek-v2",
			Timestamp: time.Now(),
			Success:   false,
			ErrorType: "rate_limit",
		},
	}

	err := ai.BatchPublishProviderMetrics(context.Background(), batch)
	assert.NoError(t, err)

	published := broker.getPublished()
	assert.Len(t, published, 3)
}

func TestAnalyticsIntegration_BatchPublishProviderMetrics_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), false)

	batch := []*ProviderMetrics{
		{Provider: "claude", Timestamp: time.Now()},
	}

	err := ai.BatchPublishProviderMetrics(context.Background(), batch)
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestAnalyticsIntegration_BatchPublishProviderMetrics_EmptyBatch(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	err := ai.BatchPublishProviderMetrics(context.Background(), []*ProviderMetrics{})
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestAnalyticsIntegration_BatchPublishProviderMetrics_BrokerErrorContinues(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("intermittent error")
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	batch := []*ProviderMetrics{
		{Provider: "claude", Timestamp: time.Now()},
		{Provider: "gemini", Timestamp: time.Now()},
	}

	// BatchPublishProviderMetrics always returns nil, logging errors
	err := ai.BatchPublishProviderMetrics(context.Background(), batch)
	assert.NoError(t, err)
}

func TestAnalyticsIntegration_BatchPublishProviderMetrics_NilBatch(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ai := NewAnalyticsIntegration(broker, newTestLogger(), true)

	err := ai.BatchPublishProviderMetrics(context.Background(), nil)
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}
