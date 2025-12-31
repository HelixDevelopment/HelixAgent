package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDebateLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Silence logs in tests
	return log
}

func TestNewDebateService(t *testing.T) {
	logger := newTestDebateLogger()
	ds := NewDebateService(logger)
	require.NotNil(t, ds)
	assert.Equal(t, logger, ds.logger)
}

func TestDebateService_ConductDebate_Basic(t *testing.T) {
	logger := newTestDebateLogger()
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
	assert.Equal(t, 3, result.RoundsConducted)
	assert.True(t, result.Success)
	assert.Equal(t, 0.85, result.QualityScore)
	assert.Equal(t, 0.87, result.FinalScore)
	assert.NotEmpty(t, result.SessionID)
	assert.NotNil(t, result.Metadata)
}

func TestDebateService_ConductDebate_WithParticipants(t *testing.T) {
	logger := newTestDebateLogger()
	ds := NewDebateService(logger)

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

	// Verify participants
	assert.Len(t, result.Participants, 3)

	for i, participant := range result.Participants {
		assert.Equal(t, config.Participants[i].ParticipantID, participant.ParticipantID)
		assert.Equal(t, config.Participants[i].Name, participant.ParticipantName)
		assert.Equal(t, config.Participants[i].Role, participant.Role)
		assert.Equal(t, config.Participants[i].LLMProvider, participant.LLMProvider)
		assert.Equal(t, config.Participants[i].LLMModel, participant.LLMModel)
		assert.Contains(t, participant.Response, config.Participants[i].Name)
		assert.Contains(t, participant.Content, config.Participants[i].Name)
		assert.Equal(t, 0.9, participant.Confidence)
		assert.Equal(t, 0.85, participant.QualityScore)
		assert.Equal(t, 5*time.Second, participant.ResponseTime)
		assert.Equal(t, 1, participant.Round)
		assert.Equal(t, 1, participant.RoundNumber)
	}
}

func TestDebateService_ConductDebate_WithConsensus(t *testing.T) {
	logger := newTestDebateLogger()
	ds := NewDebateService(logger)

	config := &DebateConfig{
		DebateID:     "test-debate-3",
		Topic:        "Consensus Test",
		MaxRounds:    2,
		Timeout:      5 * time.Second,
		Participants: []ParticipantConfig{{ParticipantID: "p1", Name: "Agent", Role: "debater"}},
		EnableCognee: false,
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Consensus)

	assert.True(t, result.Consensus.Reached)
	assert.True(t, result.Consensus.Achieved)
	assert.Equal(t, 0.85, result.Consensus.Confidence)
	assert.Equal(t, 0.85, result.Consensus.ConsensusLevel)
	assert.Equal(t, 0.85, result.Consensus.AgreementLevel)
	assert.Equal(t, 0.85, result.Consensus.AgreementScore)
	assert.Equal(t, "Agreement reached", result.Consensus.FinalPosition)
	assert.Equal(t, []string{"Point 1", "Point 2"}, result.Consensus.KeyPoints)
	assert.Empty(t, result.Consensus.Disagreements)
	assert.Equal(t, "Consensus summary", result.Consensus.Summary)
	assert.Equal(t, 0.85, result.Consensus.QualityScore)
}

func TestDebateService_ConductDebate_WithCogneeEnabled(t *testing.T) {
	logger := newTestDebateLogger()
	ds := NewDebateService(logger)

	config := &DebateConfig{
		DebateID:     "test-debate-cognee",
		Topic:        "AI Ethics",
		MaxRounds:    1,
		Timeout:      5 * time.Second,
		Participants: []ParticipantConfig{{ParticipantID: "p1", Name: "Agent", Role: "debater"}},
		EnableCognee: true, // Enable Cognee enhancement
	}

	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify Cognee enhancement
	assert.True(t, result.CogneeEnhanced)
	require.NotNil(t, result.CogneeInsights)

	insights := result.CogneeInsights
	assert.Equal(t, "debate-insights", insights.DatasetName)
	assert.Contains(t, insights.SemanticAnalysis.MainThemes, "AI Ethics")
	assert.Contains(t, insights.SemanticAnalysis.MainThemes, "debate")
	assert.Equal(t, 0.85, insights.SemanticAnalysis.CoherenceScore)

	// Verify entity extraction
	require.Len(t, insights.EntityExtraction, 1)
	assert.Equal(t, "AI Ethics", insights.EntityExtraction[0].Text)
	assert.Equal(t, "TOPIC", insights.EntityExtraction[0].Type)
	assert.Equal(t, 1.0, insights.EntityExtraction[0].Confidence)

	// Verify sentiment analysis
	assert.Equal(t, "neutral", insights.SentimentAnalysis.OverallSentiment)
	assert.Equal(t, 0.7, insights.SentimentAnalysis.SentimentScore)

	// Verify knowledge graph
	assert.Contains(t, insights.KnowledgeGraph.CentralConcepts, "AI Ethics")

	// Verify recommendations
	assert.Len(t, insights.Recommendations, 3)
	assert.Contains(t, insights.Recommendations, "Consider diverse perspectives")

	// Verify quality metrics
	require.NotNil(t, insights.QualityMetrics)
	assert.Equal(t, 0.9, insights.QualityMetrics.Coherence)
	assert.Equal(t, 0.85, insights.QualityMetrics.Relevance)
	assert.Equal(t, 0.88, insights.QualityMetrics.Accuracy)
	assert.Equal(t, 0.87, insights.QualityMetrics.Completeness)
	assert.Equal(t, 0.87, insights.QualityMetrics.OverallScore)

	// Verify topic modeling
	assert.Equal(t, 0.9, insights.TopicModeling["AI Ethics"])

	// Verify scores
	assert.Equal(t, 0.85, insights.CoherenceScore)
	assert.Equal(t, 0.82, insights.RelevanceScore)
	assert.Equal(t, 0.75, insights.InnovationScore)
}

func TestDebateService_ConductDebate_WithEmptyParticipants(t *testing.T) {
	logger := newTestDebateLogger()
	ds := NewDebateService(logger)

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
	logger := newTestDebateLogger()
	ds := NewDebateService(logger)

	timeout := 15 * time.Second
	config := &DebateConfig{
		DebateID:     "test-debate-timing",
		Topic:        "Timing Test",
		MaxRounds:    4,
		Timeout:      timeout,
		Participants: []ParticipantConfig{{ParticipantID: "p1", Name: "Agent", Role: "debater"}},
		EnableCognee: false,
	}

	startTime := time.Now()
	result, err := ds.ConductDebate(context.Background(), config)
	require.NoError(t, err)

	// Verify timing
	assert.Equal(t, timeout, result.Duration)
	assert.True(t, result.StartTime.After(startTime.Add(-time.Second)) || result.StartTime.Equal(startTime))
	assert.True(t, result.EndTime.After(result.StartTime) || result.EndTime.Equal(result.StartTime.Add(timeout)))
}
