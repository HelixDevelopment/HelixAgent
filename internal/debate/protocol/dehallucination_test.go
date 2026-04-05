package protocol

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDehallucationLLM is a mock LLM client for dehallucination tests.
type mockDehallucationLLM struct {
	responses []string
	callIdx   int
}

func (m *mockDehallucationLLM) Complete(
	ctx context.Context, prompt string,
) (string, error) {
	if m.callIdx < len(m.responses) {
		resp := m.responses[m.callIdx]
		m.callIdx++
		return resp, nil
	}
	return "CONFIDENCE: 0.95", nil
}

// ==========================================================================
// DefaultDehallucationConfig
// ==========================================================================

func TestDefaultDehallucationConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultDehallucationConfig()

	assert.True(t, cfg.Enabled, "default config should be enabled")
	assert.Equal(t, 3, cfg.MaxClarificationRounds,
		"default max rounds should be 3")
	assert.InDelta(t, 0.9, cfg.ConfidenceThreshold, 1e-9,
		"default confidence threshold should be 0.9")
}

// ==========================================================================
// NewDehallucationPhase
// ==========================================================================

func TestNewDehallucationPhase(t *testing.T) {
	t.Parallel()
	cfg := DefaultDehallucationConfig()
	mock := &mockDehallucationLLM{}

	phase := NewDehallucationPhase(cfg, mock)

	require.NotNil(t, phase)
	assert.Equal(t, cfg.MaxClarificationRounds,
		phase.config.MaxClarificationRounds)
	assert.Equal(t, cfg.ConfidenceThreshold,
		phase.config.ConfidenceThreshold)
	assert.True(t, phase.config.Enabled)
}

// ==========================================================================
// Execute — disabled (Skipped)
// ==========================================================================

func TestDehallucationPhase_Execute_Disabled(t *testing.T) {
	t.Parallel()
	cfg := DehallucationConfig{
		Enabled:                false,
		MaxClarificationRounds: 3,
		ConfidenceThreshold:    0.9,
	}
	mock := &mockDehallucationLLM{}
	phase := NewDehallucationPhase(cfg, mock)

	result, err := phase.Execute(
		context.Background(), "build a REST API", nil,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Skipped,
		"should be skipped when disabled")
	assert.Equal(t, "build a REST API", result.ClarifiedTask,
		"clarified task should be the original task")
	assert.Equal(t, 0, result.ClarificationRounds)
	assert.Equal(t, 0, mock.callIdx,
		"LLM should not be called when disabled")
}

// ==========================================================================
// Execute — with mock LLM (clarifications then answers)
// ==========================================================================

func TestDehallucationPhase_Execute_WithMockLLM(t *testing.T) {
	t.Parallel()
	cfg := DehallucationConfig{
		Enabled:                true,
		MaxClarificationRounds: 3,
		ConfidenceThreshold:    0.85,
	}

	// Response 1: initial clarification generation (low confidence)
	// Response 2: answer from domain expert (moderate confidence)
	// Response 3: second round clarification (high confidence => break)
	mock := &mockDehallucationLLM{
		responses: []string{
			// Round 0 — generateClarifications (initial)
			"QUESTION: What language?\nCATEGORY: requirements\nPRIORITY: 4\nCONFIDENCE: 0.4",
			// Round 0 — answerClarifications
			"ANSWER: Use Go\nREMAINING_AMBIGUITIES: framework choice\nOVERALL_CONFIDENCE: 0.6",
			// Round 1 — generateClarifications (with priorAnswers)
			"QUESTION: Which framework?\nCATEGORY: constraints\nPRIORITY: 3\nCONFIDENCE: 0.9",
		},
	}

	phase := NewDehallucationPhase(cfg, mock)

	result, err := phase.Execute(
		context.Background(), "build an API server", nil,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Skipped)
	assert.GreaterOrEqual(t, result.ClarificationRounds, 1)
	assert.GreaterOrEqual(t, result.FinalConfidence, 0.6)
	assert.NotEmpty(t, result.Clarifications)
	assert.NotEmpty(t, result.Responses)
	assert.Contains(t, result.ClarifiedTask, "build an API server")
}

// ==========================================================================
// Execute — high initial confidence (Skipped)
// ==========================================================================

func TestDehallucationPhase_Execute_HighInitialConfidence(t *testing.T) {
	t.Parallel()
	cfg := DehallucationConfig{
		Enabled:                true,
		MaxClarificationRounds: 3,
		ConfidenceThreshold:    0.8,
	}

	// LLM immediately reports high confidence, no questions needed
	mock := &mockDehallucationLLM{
		responses: []string{
			"CONFIDENCE: 0.95",
		},
	}

	phase := NewDehallucationPhase(cfg, mock)

	result, err := phase.Execute(
		context.Background(), "print hello world", nil,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Skipped,
		"should be skipped when initial confidence is above threshold")
	assert.Equal(t, "print hello world", result.ClarifiedTask)
	assert.InDelta(t, 0.95, result.FinalConfidence, 1e-9)
	assert.Equal(t, 1, mock.callIdx,
		"only one LLM call should be made for initial assessment")
}

// ==========================================================================
// parseClarifications
// ==========================================================================

func TestDehallucationPhase_ParseClarifications(t *testing.T) {
	t.Parallel()
	phase := NewDehallucationPhase(DefaultDehallucationConfig(), nil)

	tests := []struct {
		name          string
		response      string
		expectedCount int
		expectedConf  float64
		checkQuestion string
		checkCategory string
		checkPriority int
	}{
		{
			name: "single question with all fields",
			response: "QUESTION: What is the target OS?\n" +
				"CATEGORY: constraints\n" +
				"PRIORITY: 5\n" +
				"CONFIDENCE: 0.6",
			expectedCount: 1,
			expectedConf:  0.6,
			checkQuestion: "What is the target OS?",
			checkCategory: "constraints",
			checkPriority: 5,
		},
		{
			name: "multiple questions",
			response: "QUESTION: What language?\n" +
				"CATEGORY: requirements\n" +
				"PRIORITY: 4\n" +
				"QUESTION: What framework?\n" +
				"CATEGORY: integration\n" +
				"PRIORITY: 3\n" +
				"CONFIDENCE: 0.5",
			expectedCount: 2,
			expectedConf:  0.5,
			checkQuestion: "What language?",
			checkCategory: "requirements",
			checkPriority: 4,
		},
		{
			name: "question without priority uses default",
			response: "QUESTION: Is caching needed?\n" +
				"CATEGORY: performance\n" +
				"CONFIDENCE: 0.7",
			expectedCount: 1,
			expectedConf:  0.7,
			checkQuestion: "Is caching needed?",
			checkCategory: "performance",
			checkPriority: 3, // default
		},
		{
			name: "invalid priority falls back to 3",
			response: "QUESTION: How many users?\n" +
				"CATEGORY: requirements\n" +
				"PRIORITY: 99\n" +
				"CONFIDENCE: 0.3",
			expectedCount: 1,
			expectedConf:  0.3,
			checkQuestion: "How many users?",
			checkCategory: "requirements",
			checkPriority: 3,
		},
		{
			name:          "empty response",
			response:      "",
			expectedCount: 0,
			expectedConf:  0.0,
		},
		{
			name: "unrecognized category normalized to requirements",
			response: "QUESTION: Any preferences?\n" +
				"CATEGORY: misc\n" +
				"PRIORITY: 2\n" +
				"CONFIDENCE: 0.5",
			expectedCount: 1,
			expectedConf:  0.5,
			checkQuestion: "Any preferences?",
			checkCategory: "requirements",
			checkPriority: 2,
		},
		{
			name: "edge_cases alias",
			response: "QUESTION: Boundary?\n" +
				"CATEGORY: edge cases\n" +
				"PRIORITY: 3\n" +
				"CONFIDENCE: 0.4",
			expectedCount: 1,
			expectedConf:  0.4,
			checkQuestion: "Boundary?",
			checkCategory: "edge_cases",
			checkPriority: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			questions, conf := phase.parseClarifications(tc.response)

			assert.Len(t, questions, tc.expectedCount)
			assert.InDelta(t, tc.expectedConf, conf, 1e-9)

			if tc.expectedCount > 0 {
				assert.Equal(t, tc.checkQuestion, questions[0].Question)
				assert.Equal(t, tc.checkCategory, questions[0].Category)
				assert.Equal(t, tc.checkPriority, questions[0].Priority)
			}
		})
	}
}

// ==========================================================================
// parseAnswer
// ==========================================================================

func TestDehallucationPhase_ParseAnswer(t *testing.T) {
	t.Parallel()
	phase := NewDehallucationPhase(DefaultDehallucationConfig(), nil)

	tests := []struct {
		name               string
		response           string
		expectedAnswer     string
		expectedConf       float64
		expectedAmbigCount int
	}{
		{
			name: "structured answer with ambiguities",
			response: "ANSWER: Use PostgreSQL for persistence\n" +
				"REMAINING_AMBIGUITIES: schema design; indexing strategy\n" +
				"OVERALL_CONFIDENCE: 0.75",
			expectedAnswer:     "Use PostgreSQL for persistence",
			expectedConf:       0.75,
			expectedAmbigCount: 2,
		},
		{
			name: "multiple answers joined",
			response: "ANSWER: Use Go\n" +
				"ANSWER: Use Gin framework\n" +
				"REMAINING_AMBIGUITIES: none\n" +
				"OVERALL_CONFIDENCE: 0.9",
			expectedAnswer:     "Use Go | Use Gin framework",
			expectedConf:       0.9,
			expectedAmbigCount: 0,
		},
		{
			name:               "unstructured fallback",
			response:           "I think we should use Redis for caching.",
			expectedAnswer:     "I think we should use Redis for caching.",
			expectedConf:       0.5, // default
			expectedAmbigCount: 0,
		},
		{
			name: "N/A ambiguities treated as empty",
			response: "ANSWER: Use REST\n" +
				"REMAINING_AMBIGUITIES: N/A\n" +
				"OVERALL_CONFIDENCE: 0.85",
			expectedAnswer:     "Use REST",
			expectedConf:       0.85,
			expectedAmbigCount: 0,
		},
		{
			name: "None ambiguities treated as empty",
			response: "ANSWER: Use HTTP/3\n" +
				"REMAINING_AMBIGUITIES: None\n" +
				"OVERALL_CONFIDENCE: 0.88",
			expectedAnswer:     "Use HTTP/3",
			expectedConf:       0.88,
			expectedAmbigCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			answer := phase.parseAnswer(tc.response)

			require.NotNil(t, answer)
			assert.Equal(t, tc.expectedAnswer, answer.Answer)
			assert.InDelta(t, tc.expectedConf, answer.Confidence, 1e-9)
			assert.Len(t, answer.RemainingAmbiguities, tc.expectedAmbigCount)
		})
	}
}

// ==========================================================================
// buildClarifiedTask
// ==========================================================================

func TestDehallucationPhase_BuildClarifiedTask(t *testing.T) {
	t.Parallel()
	phase := NewDehallucationPhase(DefaultDehallucationConfig(), nil)

	t.Run("no responses returns original task", func(t *testing.T) {
			t.Parallel()
		result := phase.buildClarifiedTask("original task", nil)
		assert.Equal(t, "original task", result)
	})

	t.Run("empty responses returns original task", func(t *testing.T) {
			t.Parallel()
		result := phase.buildClarifiedTask(
			"original task", []ClarificationResponse{},
		)
		assert.Equal(t, "original task", result)
	})

	t.Run("responses appended to task", func(t *testing.T) {
			t.Parallel()
		responses := []ClarificationResponse{
			{
				Answer:     "Use Go with Gin",
				Confidence: 0.8,
			},
			{
				Answer:               "Add Redis caching",
				Confidence:           0.85,
				RemainingAmbiguities: []string{"cache eviction policy"},
			},
		}

		result := phase.buildClarifiedTask("build API", responses)

		assert.Contains(t, result, "build API")
		assert.Contains(t, result, "--- Clarifications ---")
		assert.Contains(t, result, "Round 1 Clarification:")
		assert.Contains(t, result, "Use Go with Gin")
		assert.Contains(t, result, "Round 2 Clarification:")
		assert.Contains(t, result, "Add Redis caching")
		assert.Contains(t, result, "cache eviction policy")
	})
}
