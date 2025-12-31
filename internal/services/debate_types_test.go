package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDebateResult_Validate(t *testing.T) {
	now := time.Now()

	t.Run("valid debate result", func(t *testing.T) {
		result := &DebateResult{
			DebateID:    "debate-123",
			StartTime:   now,
			TotalRounds: 3,
			Participants: []ParticipantResponse{
				{ParticipantID: "p1", ParticipantName: "Participant 1"},
				{ParticipantID: "p2", ParticipantName: "Participant 2"},
			},
		}

		err := result.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing debate_id", func(t *testing.T) {
		result := &DebateResult{
			StartTime:   now,
			TotalRounds: 3,
			Participants: []ParticipantResponse{
				{ParticipantID: "p1"},
				{ParticipantID: "p2"},
			},
		}

		err := result.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "debate_id is required")
	})

	t.Run("missing start_time", func(t *testing.T) {
		result := &DebateResult{
			DebateID:    "debate-123",
			TotalRounds: 3,
			Participants: []ParticipantResponse{
				{ParticipantID: "p1"},
				{ParticipantID: "p2"},
			},
		}

		err := result.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "start_time is required")
	})

	t.Run("invalid total_rounds", func(t *testing.T) {
		result := &DebateResult{
			DebateID:    "debate-123",
			StartTime:   now,
			TotalRounds: 0,
			Participants: []ParticipantResponse{
				{ParticipantID: "p1"},
				{ParticipantID: "p2"},
			},
		}

		err := result.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "total_rounds must be at least 1")
	})

	t.Run("insufficient participants", func(t *testing.T) {
		result := &DebateResult{
			DebateID:    "debate-123",
			StartTime:   now,
			TotalRounds: 3,
			Participants: []ParticipantResponse{
				{ParticipantID: "p1"},
			},
		}

		err := result.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2 participants required")
	})
}

func TestConsensusResult_Validate(t *testing.T) {
	now := time.Now()

	t.Run("valid consensus result", func(t *testing.T) {
		result := &ConsensusResult{
			Timestamp:  now,
			Confidence: 0.85,
		}

		err := result.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing timestamp", func(t *testing.T) {
		result := &ConsensusResult{
			Confidence: 0.85,
		}

		err := result.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timestamp is required")
	})

	t.Run("confidence below 0", func(t *testing.T) {
		result := &ConsensusResult{
			Timestamp:  now,
			Confidence: -0.1,
		}

		err := result.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "confidence must be between 0 and 1")
	})

	t.Run("confidence above 1", func(t *testing.T) {
		result := &ConsensusResult{
			Timestamp:  now,
			Confidence: 1.5,
		}

		err := result.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "confidence must be between 0 and 1")
	})

	t.Run("confidence at boundaries", func(t *testing.T) {
		result := &ConsensusResult{
			Timestamp:  now,
			Confidence: 0.0,
		}
		assert.NoError(t, result.Validate())

		result.Confidence = 1.0
		assert.NoError(t, result.Validate())
	})
}

func TestParticipantResponse_Validate(t *testing.T) {
	now := time.Now()

	t.Run("valid participant response", func(t *testing.T) {
		response := &ParticipantResponse{
			ParticipantID:   "participant-123",
			ParticipantName: "Test Participant",
			Round:           1,
			Response:        "This is my response",
			Timestamp:       now,
		}

		err := response.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing participant_id", func(t *testing.T) {
		response := &ParticipantResponse{
			ParticipantName: "Test Participant",
			Round:           1,
			Response:        "Response",
			Timestamp:       now,
		}

		err := response.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "participant_id is required")
	})

	t.Run("missing participant_name", func(t *testing.T) {
		response := &ParticipantResponse{
			ParticipantID: "participant-123",
			Round:         1,
			Response:      "Response",
			Timestamp:     now,
		}

		err := response.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "participant_name is required")
	})

	t.Run("invalid round", func(t *testing.T) {
		response := &ParticipantResponse{
			ParticipantID:   "participant-123",
			ParticipantName: "Test Participant",
			Round:           0,
			Response:        "Response",
			Timestamp:       now,
		}

		err := response.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "round must be at least 1")
	})

	t.Run("missing response", func(t *testing.T) {
		response := &ParticipantResponse{
			ParticipantID:   "participant-123",
			ParticipantName: "Test Participant",
			Round:           1,
			Response:        "",
			Timestamp:       now,
		}

		err := response.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "response is required")
	})

	t.Run("missing timestamp", func(t *testing.T) {
		response := &ParticipantResponse{
			ParticipantID:   "participant-123",
			ParticipantName: "Test Participant",
			Round:           1,
			Response:        "Response",
		}

		err := response.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timestamp is required")
	})
}

func TestDebateResult_Structure(t *testing.T) {
	now := time.Now()
	result := &DebateResult{
		DebateID:        "debate-123",
		SessionID:       "session-456",
		Topic:           "AI Ethics",
		StartTime:       now,
		EndTime:         now.Add(time.Hour),
		Duration:        time.Hour,
		TotalRounds:     3,
		RoundsConducted: 3,
		Participants:    []ParticipantResponse{},
		QualityScore:    0.85,
		FinalScore:      0.90,
		Success:         true,
		FallbackUsed:    false,
		CogneeEnhanced:  true,
		MemoryUsed:      true,
	}

	assert.Equal(t, "debate-123", result.DebateID)
	assert.Equal(t, "session-456", result.SessionID)
	assert.Equal(t, "AI Ethics", result.Topic)
	assert.Equal(t, 3, result.TotalRounds)
	assert.Equal(t, 0.85, result.QualityScore)
	assert.True(t, result.Success)
	assert.True(t, result.CogneeEnhanced)
}

func TestConsensusResult_Structure(t *testing.T) {
	now := time.Now()
	result := &ConsensusResult{
		Reached:        true,
		Achieved:       true,
		Confidence:     0.85,
		ConsensusLevel: 0.90,
		AgreementLevel: 0.88,
		AgreementScore: 0.87,
		FinalPosition:  "AI should be regulated",
		KeyPoints:      []string{"Point 1", "Point 2"},
		Disagreements:  []string{"Disagreement 1"},
		Summary:        "Summary of consensus",
		VotingSummary: VotingSummary{
			Strategy:         "majority",
			TotalVotes:       5,
			VoteDistribution: map[string]int{"yes": 3, "no": 2},
			Winner:           "yes",
			Margin:           0.2,
		},
		Timestamp:    now,
		QualityScore: 0.9,
	}

	assert.True(t, result.Reached)
	assert.True(t, result.Achieved)
	assert.Equal(t, 0.85, result.Confidence)
	assert.Len(t, result.KeyPoints, 2)
	assert.Equal(t, "yes", result.VotingSummary.Winner)
}

func TestParticipantResponse_Structure(t *testing.T) {
	now := time.Now()
	response := &ParticipantResponse{
		ParticipantID:   "p-123",
		ParticipantName: "Claude",
		Role:            "debater",
		Round:           1,
		RoundNumber:     1,
		Response:        "My argument is...",
		Content:         "Content",
		Confidence:      0.95,
		QualityScore:    0.90,
		ResponseTime:    500 * time.Millisecond,
		LLMProvider:     "anthropic",
		LLMModel:        "claude-3-opus",
		LLMName:         "Claude",
		CogneeEnhanced:  true,
		Timestamp:       now,
	}

	assert.Equal(t, "p-123", response.ParticipantID)
	assert.Equal(t, "Claude", response.ParticipantName)
	assert.Equal(t, "debater", response.Role)
	assert.Equal(t, 0.95, response.Confidence)
	assert.Equal(t, "anthropic", response.LLMProvider)
	assert.True(t, response.CogneeEnhanced)
}

func TestCogneeInsights_Structure(t *testing.T) {
	insights := &CogneeInsights{
		DatasetName:     "debate-dataset",
		EnhancementTime: 2 * time.Second,
		SemanticAnalysis: SemanticAnalysis{
			SimilarityMatrix: [][]float64{{1.0, 0.8}, {0.8, 1.0}},
			Clusters:         []Cluster{{ID: "c1", Members: []string{"m1", "m2"}, Centroid: "m1"}},
			MainThemes:       []string{"theme1", "theme2"},
			CoherenceScore:   0.85,
		},
		EntityExtraction: []Entity{
			{Text: "AI", Type: "technology", Confidence: 0.9},
		},
		SentimentAnalysis: SentimentAnalysis{
			OverallSentiment: "positive",
			SentimentScore:   0.7,
		},
		KnowledgeGraph: KnowledgeGraph{
			Nodes:           []Node{{ID: "n1", Label: "AI", Type: "concept"}},
			Edges:           []Edge{{Source: "n1", Target: "n2", Type: "relates", Weight: 0.8}},
			CentralConcepts: []string{"AI"},
		},
		Recommendations: []string{"rec1", "rec2"},
		CoherenceScore:  0.88,
		RelevanceScore:  0.92,
		InnovationScore: 0.75,
	}

	assert.Equal(t, "debate-dataset", insights.DatasetName)
	assert.Len(t, insights.EntityExtraction, 1)
	assert.Equal(t, "positive", insights.SentimentAnalysis.OverallSentiment)
	assert.Len(t, insights.KnowledgeGraph.CentralConcepts, 1)
}

func TestCogneeAnalysis_Structure(t *testing.T) {
	analysis := &CogneeAnalysis{
		Enhanced:         true,
		OriginalResponse: "Original response",
		EnhancedResponse: "Enhanced response",
		Sentiment:        "positive",
		Entities:         []string{"entity1", "entity2"},
		KeyPhrases:       []string{"phrase1", "phrase2"},
		Confidence:       0.88,
		ProcessingTime:   100 * time.Millisecond,
	}

	assert.True(t, analysis.Enhanced)
	assert.Equal(t, "positive", analysis.Sentiment)
	assert.Len(t, analysis.Entities, 2)
	assert.Equal(t, 0.88, analysis.Confidence)
}

func TestVotingSummary_Structure(t *testing.T) {
	summary := VotingSummary{
		Strategy:         "confidence_weighted",
		TotalVotes:       10,
		VoteDistribution: map[string]int{"option1": 6, "option2": 4},
		Winner:           "option1",
		Margin:           0.2,
	}

	assert.Equal(t, "confidence_weighted", summary.Strategy)
	assert.Equal(t, 10, summary.TotalVotes)
	assert.Equal(t, "option1", summary.Winner)
}

func TestQualityMetrics_Structure(t *testing.T) {
	metrics := &QualityMetrics{
		Coherence:    0.85,
		Relevance:    0.90,
		Accuracy:     0.88,
		Completeness: 0.82,
		OverallScore: 0.86,
	}

	assert.Equal(t, 0.85, metrics.Coherence)
	assert.Equal(t, 0.90, metrics.Relevance)
	assert.Equal(t, 0.88, metrics.Accuracy)
	assert.Equal(t, 0.86, metrics.OverallScore)
}

func TestEntity_Structure(t *testing.T) {
	entity := Entity{
		Text:       "Artificial Intelligence",
		Type:       "technology",
		Confidence: 0.95,
	}

	assert.Equal(t, "Artificial Intelligence", entity.Text)
	assert.Equal(t, "technology", entity.Type)
	assert.Equal(t, 0.95, entity.Confidence)
}

func TestCluster_Structure(t *testing.T) {
	cluster := Cluster{
		ID:       "cluster-1",
		Members:  []string{"member1", "member2", "member3"},
		Centroid: "member1",
	}

	assert.Equal(t, "cluster-1", cluster.ID)
	assert.Len(t, cluster.Members, 3)
	assert.Equal(t, "member1", cluster.Centroid)
}

func TestSentimentByRound_Structure(t *testing.T) {
	sentiment := SentimentByRound{
		Round:     1,
		Sentiment: "positive",
		Score:     0.75,
	}

	assert.Equal(t, 1, sentiment.Round)
	assert.Equal(t, "positive", sentiment.Sentiment)
	assert.Equal(t, 0.75, sentiment.Score)
}

func TestNode_Structure(t *testing.T) {
	node := Node{
		ID:    "node-1",
		Label: "Concept",
		Type:  "concept",
		Properties: map[string]any{
			"importance": 0.9,
			"category":   "main",
		},
	}

	assert.Equal(t, "node-1", node.ID)
	assert.Equal(t, "Concept", node.Label)
	assert.Equal(t, 0.9, node.Properties["importance"])
}

func TestEdge_Structure(t *testing.T) {
	edge := Edge{
		Source: "node-1",
		Target: "node-2",
		Type:   "relates_to",
		Weight: 0.8,
	}

	assert.Equal(t, "node-1", edge.Source)
	assert.Equal(t, "node-2", edge.Target)
	assert.Equal(t, "relates_to", edge.Type)
	assert.Equal(t, 0.8, edge.Weight)
}
