package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAdvancedDebateTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Silence logs in tests
	return log
}

func TestAdvancedDebateService_ConductAdvancedDebate(t *testing.T) {
	logger := newAdvancedDebateTestLogger()

	ads := NewAdvancedDebateService(
		NewDebateService(logger),
		NewDebateMonitoringService(logger),
		NewDebatePerformanceService(logger),
		NewDebateHistoryService(logger),
		NewDebateResilienceService(logger),
		NewDebateReportingService(logger),
		NewDebateSecurityService(logger),
		logger,
	)

	config := &DebateConfig{
		DebateID:  "test-advanced-debate-1",
		Topic:     "Advanced Topic",
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

	result, err := ads.ConductAdvancedDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "test-advanced-debate-1", result.DebateID)
	assert.Equal(t, "Advanced Topic", result.Topic)
	assert.True(t, result.Success)

	// Verify report was added to metadata
	require.NotNil(t, result.Metadata)
	assert.Contains(t, result.Metadata, "report")
}

func TestAdvancedDebateService_ConductAdvancedDebate_WithCogneeEnabled(t *testing.T) {
	logger := newAdvancedDebateTestLogger()

	ads := NewAdvancedDebateService(
		NewDebateService(logger),
		NewDebateMonitoringService(logger),
		NewDebatePerformanceService(logger),
		NewDebateHistoryService(logger),
		NewDebateResilienceService(logger),
		NewDebateReportingService(logger),
		NewDebateSecurityService(logger),
		logger,
	)

	config := &DebateConfig{
		DebateID:  "test-advanced-debate-cognee",
		Topic:     "Cognee-Enhanced Debate",
		MaxRounds: 2,
		Timeout:   5 * time.Second,
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Agent", Role: "debater"},
		},
		EnableCognee: true, // Enable Cognee
	}

	result, err := ads.ConductAdvancedDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.CogneeEnhanced)
	assert.NotNil(t, result.CogneeInsights)
}

func TestAdvancedDebateService_ConductAdvancedDebate_MultipleParticipants(t *testing.T) {
	logger := newAdvancedDebateTestLogger()

	ads := NewAdvancedDebateService(
		NewDebateService(logger),
		NewDebateMonitoringService(logger),
		NewDebatePerformanceService(logger),
		NewDebateHistoryService(logger),
		NewDebateResilienceService(logger),
		NewDebateReportingService(logger),
		NewDebateSecurityService(logger),
		logger,
	)

	config := &DebateConfig{
		DebateID:  "test-advanced-debate-multi",
		Topic:     "Multi-participant Discussion",
		MaxRounds: 5,
		Timeout:   30 * time.Second,
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Alice", Role: "proposer"},
			{ParticipantID: "p2", Name: "Bob", Role: "critic"},
			{ParticipantID: "p3", Name: "Charlie", Role: "mediator"},
		},
		EnableCognee: false,
	}

	result, err := ads.ConductAdvancedDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "test-advanced-debate-multi", result.DebateID)
	assert.Len(t, result.Participants, 3)
}

func TestAdvancedDebateService_ConductAdvancedDebate_EmptyParticipants(t *testing.T) {
	logger := newAdvancedDebateTestLogger()

	ads := NewAdvancedDebateService(
		NewDebateService(logger),
		NewDebateMonitoringService(logger),
		NewDebatePerformanceService(logger),
		NewDebateHistoryService(logger),
		NewDebateResilienceService(logger),
		NewDebateReportingService(logger),
		NewDebateSecurityService(logger),
		logger,
	)

	config := &DebateConfig{
		DebateID:     "test-advanced-debate-empty",
		Topic:        "Empty Debate",
		MaxRounds:    1,
		Timeout:      5 * time.Second,
		Participants: []ParticipantConfig{},
		EnableCognee: false,
	}

	result, err := ads.ConductAdvancedDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Empty(t, result.Participants)
}
