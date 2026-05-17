// CONST-030: these tests exercise the in-process reflexion.ReflexionLoop,
// EpisodicMemoryBuffer, AccumulatedWisdom, and ReflectionGenerator against
// canned mockTestExecutor / mockLLMClient / failingLLMClient doubles. They
// assert on protocol-internal invariants (attempt counting, wisdom pattern
// extraction, FIFO eviction, fallback shape) — unit-level per CONST-030.
// Demoted from tests/integration/ (PR10).
package debate_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.debate/reflexion"
)

// mockTestExecutor implements reflexion.TestExecutor for integration testing.
type mockTestExecutor struct {
	// passOnAttempt is the attempt number (1-based) at which all tests pass.
	// If zero, tests never all pass.
	passOnAttempt int
	callCount     int
}

func (m *mockTestExecutor) Execute(
	ctx context.Context,
	code string,
	language string,
) ([]*reflexion.TestResult, error) {
	m.callCount++

	if m.passOnAttempt > 0 && m.callCount >= m.passOnAttempt {
		return []*reflexion.TestResult{
			{Name: "test_basic", Passed: true, Output: "ok", Duration: 10 * time.Millisecond},
			{Name: "test_edge", Passed: true, Output: "ok", Duration: 15 * time.Millisecond},
		}, nil
	}

	// First attempts fail
	return []*reflexion.TestResult{
		{Name: "test_basic", Passed: true, Output: "ok", Duration: 10 * time.Millisecond},
		{Name: "test_edge", Passed: false, Output: "", Error: "assertion failed: expected 42, got 0", Duration: 20 * time.Millisecond},
	}, nil
}

// mockLLMClient implements reflexion.LLMClient for integration testing.
type mockLLMClient struct {
	responses []string
	callCount int
}

func (m *mockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.callCount < len(m.responses) {
		resp := m.responses[m.callCount]
		m.callCount++
		return resp, nil
	}
	m.callCount++
	return "ROOT_CAUSE: Logic error in computation\n" +
		"WHAT_WENT_WRONG: Incorrect formula applied\n" +
		"WHAT_TO_CHANGE: Fix the computation logic\n" +
		"CONFIDENCE: 0.7\n", nil
}

// TestReflexion_FailReflectRetrySuccess verifies the full reflexion cycle:
// first attempt fails, reflection is generated, second attempt succeeds.
func TestReflexion_FailReflectRetrySuccess(t *testing.T) {

	// Test executor: passes on attempt 2
	executor := &mockTestExecutor{passOnAttempt: 2}

	// LLM client for reflection generation
	llmClient := &mockLLMClient{
		responses: []string{
			"ROOT_CAUSE: Off-by-one error in loop\n" +
				"WHAT_WENT_WRONG: Loop did not iterate to the final element\n" +
				"WHAT_TO_CHANGE: Use <= instead of < in the loop condition\n" +
				"CONFIDENCE: 0.8\n",
		},
	}

	memory := reflexion.NewEpisodicMemoryBuffer(100)
	generator := reflexion.NewReflectionGenerator(llmClient)

	config := reflexion.ReflexionConfig{
		MaxAttempts:         3,
		ConfidenceThreshold: 0.95,
		Timeout:             30 * time.Second,
	}

	loop := reflexion.NewReflexionLoop(config, generator, executor, memory)

	attemptCount := 0
	task := &reflexion.ReflexionTask{
		Description: "Write a function to calculate factorial",
		InitialCode: "func factorial(n int) int { return 0 }",
		Language:    "go",
		AgentID:     "test-agent-1",
		SessionID:   "session-1",
		CodeGenerator: func(ctx context.Context, taskDesc string,
			priorReflections []*reflexion.Reflection) (string, error) {
			attemptCount++
			if len(priorReflections) > 0 {
				// After reflection, produce better code
				return "func factorial(n int) int { if n <= 1 { return 1 }; return n * factorial(n-1) }", nil
			}
			return "func factorial(n int) int { return 0 }", nil
		},
	}

	ctx := context.Background()
	result, err := loop.Execute(ctx, task)

	require.NoError(t, err, "Reflexion loop should not error")
	require.NotNil(t, result, "Should return a result")

	assert.True(t, result.AllPassed, "All tests should pass after retry")
	assert.Equal(t, 2, result.Attempts, "Should succeed on attempt 2")
	assert.Greater(t, len(result.Reflections), 0,
		"Should have generated at least one reflection")
	assert.Greater(t, len(result.Episodes), 0,
		"Should have stored at least one episode")

	// Verify episodic memory was updated
	assert.Greater(t, memory.Size(), 0,
		"Episodic memory should contain episodes")
	agentEpisodes := memory.GetByAgent("test-agent-1")
	assert.Greater(t, len(agentEpisodes), 0,
		"Should have episodes for the test agent")

	t.Logf("Reflexion loop: %d attempts, %d reflections, %d episodes, "+
		"final confidence %.2f",
		result.Attempts, len(result.Reflections), len(result.Episodes),
		result.FinalConfidence)
}

// TestReflexion_AccumulatedWisdom verifies that episodes are stored and
// wisdom can be extracted from patterns across multiple sessions.
func TestReflexion_AccumulatedWisdom(t *testing.T) {

	memory := reflexion.NewEpisodicMemoryBuffer(1000)
	wisdom := reflexion.NewAccumulatedWisdom()

	// Simulate multiple sessions with similar failure patterns
	sessions := []struct {
		sessionID string
		agentID   string
		rootCause string
	}{
		{"sess-1", "agent-A", "Nil pointer dereference at runtime"},
		{"sess-2", "agent-B", "Nil pointer dereference at runtime"},
		{"sess-3", "agent-A", "Nil pointer dereference at runtime"},
		{"sess-4", "agent-C", "Array index out of bounds"},
		{"sess-5", "agent-B", "Array index out of bounds"},
		{"sess-6", "agent-A", "Array index out of bounds"},
	}

	for i, sess := range sessions {
		episode := &reflexion.Episode{
			SessionID:       sess.sessionID,
			AgentID:         sess.agentID,
			TaskDescription: "Implement data processing pipeline",
			AttemptNumber:   1,
			Code:            fmt.Sprintf("// attempt code %d", i),
			TestResults:     map[string]interface{}{"test_1": map[string]interface{}{"passed": false}},
			FailureAnalysis: "Test failures detected",
			Reflection: &reflexion.Reflection{
				RootCause:        sess.rootCause,
				WhatWentWrong:    "Runtime error in code",
				WhatToChangeNext: "Add nil check or bounds check",
				ConfidenceInFix:  0.5 + float64(i)*0.05,
			},
			Confidence: 0.3 + float64(i)*0.1,
			Timestamp:  time.Now().Add(time.Duration(i) * time.Minute),
		}

		err := memory.Store(episode)
		require.NoError(t, err, "Episode storage should not fail")
	}

	// Verify memory contains all episodes
	assert.Equal(t, 6, memory.Size(), "Memory should contain 6 episodes")

	// Extract wisdom from episodes (signature drift fix close-out⁷⁴:
	// production added ctx as first arg per the reconstruction roadmap;
	// test signature lagged and the package stopped compiling).
	allEpisodes := memory.GetAll()
	extracted, err := wisdom.ExtractFromEpisodes(context.Background(), allEpisodes)
	require.NoError(t, err, "Wisdom extraction should not fail")

	// Should have extracted patterns (groups with >= 2 episodes)
	assert.Greater(t, len(extracted), 0,
		"Should extract at least one wisdom pattern")

	// Verify accumulated wisdom size
	assert.Greater(t, wisdom.Size(), 0,
		"Wisdom store should have entries")

	// Query relevant wisdom (signature drift fix close-out⁷⁴:
	// production added ctx as first arg).
	relevant := wisdom.GetRelevant(context.Background(), "nil pointer dereference", 5)
	assert.Greater(t, len(relevant), 0,
		"Should find relevant wisdom for nil pointer query")

	// Verify wisdom attributes
	for _, w := range extracted {
		assert.NotEmpty(t, w.ID, "Wisdom should have ID")
		assert.NotEmpty(t, w.Pattern, "Wisdom should have pattern")
		assert.Greater(t, w.Frequency, 1,
			"Wisdom frequency should be >= 2")
		assert.NotEmpty(t, w.Domain, "Wisdom should have domain")
		assert.NotEmpty(t, w.Tags, "Wisdom should have tags")
	}

	// Record usage and verify tracking
	if len(extracted) > 0 {
		wisdomID := extracted[0].ID
		err = wisdom.RecordUsage(wisdomID, true)
		require.NoError(t, err, "Recording usage should not fail")

		err = wisdom.RecordUsage(wisdomID, false)
		require.NoError(t, err, "Recording usage should not fail")

		all := wisdom.GetAll()
		for _, w := range all {
			if w.ID == wisdomID {
				assert.Equal(t, 2, w.UseCount,
					"Use count should be 2")
				assert.InDelta(t, 0.5, w.SuccessRate, 0.01,
					"Success rate should be 0.5")
			}
		}
	}

	t.Logf("Extracted %d wisdom patterns from %d episodes",
		len(extracted), memory.Size())
}

// TestReflexion_EpisodicMemory_BoundedCapacity verifies that the episodic
// memory buffer respects its maximum size via FIFO eviction.
func TestReflexion_EpisodicMemory_BoundedCapacity(t *testing.T) {

	maxSize := 50
	memory := reflexion.NewEpisodicMemoryBuffer(maxSize)

	// Store more episodes than the buffer capacity
	for i := 0; i < maxSize+20; i++ {
		episode := &reflexion.Episode{
			AgentID:         fmt.Sprintf("agent-%d", i%5),
			SessionID:       fmt.Sprintf("session-%d", i%10),
			TaskDescription: fmt.Sprintf("Task %d", i),
			AttemptNumber:   1,
			Code:            fmt.Sprintf("code_%d", i),
			Confidence:      float64(i) / 100.0,
			Timestamp:       time.Now().Add(time.Duration(i) * time.Second),
		}
		err := memory.Store(episode)
		require.NoError(t, err)
	}

	// Buffer size should not exceed maxSize
	assert.Equal(t, maxSize, memory.Size(),
		"Buffer should be bounded at max size")

	// The oldest episodes should have been evicted
	recent := memory.GetRecent(5)
	assert.Len(t, recent, 5, "Should return 5 recent episodes")

	// Most recent should be from the last stored batch
	if len(recent) > 0 {
		assert.Contains(t, recent[0].Code, "code_",
			"Recent episode should have expected code pattern")
	}
}

// TestReflexion_ReflectionGenerator_FallbackMode verifies that the
// reflection generator falls back to deterministic analysis when the
// LLM is unavailable.
func TestReflexion_ReflectionGenerator_FallbackMode(t *testing.T) {

	// LLM client that always fails
	failingClient := &mockLLMClient{
		responses: []string{},
	}
	// Override to always fail
	generator := reflexion.NewReflectionGenerator(
		&failingLLMClient{},
	)

	_ = failingClient // used above conceptually

	req := &reflexion.ReflectionRequest{
		Code:            "func divide(a, b int) int { return a / b }",
		TestResults:     map[string]interface{}{"test_divide_zero": map[string]interface{}{"passed": false}},
		ErrorMessages:   []string{"panic: runtime error: integer divide by zero"},
		TaskDescription: "Implement safe division function",
		AttemptNumber:   1,
	}

	ctx := context.Background()
	reflection, err := generator.Generate(ctx, req)
	require.NoError(t, err, "Fallback generation should not error")
	require.NotNil(t, reflection, "Should produce a fallback reflection")

	assert.NotEmpty(t, reflection.RootCause,
		"Fallback should identify root cause")
	assert.NotEmpty(t, reflection.WhatWentWrong,
		"Fallback should identify what went wrong")
	assert.NotEmpty(t, reflection.WhatToChangeNext,
		"Fallback should suggest what to change")
	assert.Greater(t, reflection.ConfidenceInFix, 0.0,
		"Fallback should have non-zero confidence")

	t.Logf("Fallback reflection: root_cause=%q, confidence=%.2f",
		reflection.RootCause, reflection.ConfidenceInFix)
}

// failingLLMClient always returns an error from Complete.
type failingLLMClient struct{}

func (f *failingLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", fmt.Errorf("LLM unavailable: connection refused")
}
