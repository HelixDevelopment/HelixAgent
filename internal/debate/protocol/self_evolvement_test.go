package protocol

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSelfEvolvementLLM is a mock LLM client for self-evolvement tests.
type mockSelfEvolvementLLM struct {
	responses []string
	callIdx   int
}

func (m *mockSelfEvolvementLLM) Complete(
	ctx context.Context, prompt string,
) (string, error) {
	if m.callIdx < len(m.responses) {
		resp := m.responses[m.callIdx]
		m.callIdx++
		return resp, nil
	}
	return "ACTUAL: expected output", nil
}

// ==========================================================================
// DefaultSelfEvolvementConfig
// ==========================================================================

func TestDefaultSelfEvolvementConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultSelfEvolvementConfig()

	assert.True(t, cfg.Enabled, "default config should be enabled")
	assert.Equal(t, 2, cfg.MaxIterations,
		"default max iterations should be 2")
	assert.Equal(t, 2*time.Minute, cfg.Timeout,
		"default timeout should be 2 minutes")
}

// ==========================================================================
// NewSelfEvolvementPhase
// ==========================================================================

func TestNewSelfEvolvementPhase(t *testing.T) {
	t.Parallel()
	t.Run("with valid config", func(t *testing.T) {
		cfg := DefaultSelfEvolvementConfig()
		mock := &mockSelfEvolvementLLM{}

		phase := NewSelfEvolvementPhase(cfg, mock)

		require.NotNil(t, phase)
		assert.Equal(t, 2, phase.config.MaxIterations)
		assert.Equal(t, 2*time.Minute, phase.config.Timeout)
		assert.True(t, phase.config.Enabled)
	})

	t.Run("zero max iterations defaults to 2", func(t *testing.T) {
			t.Parallel()
		cfg := SelfEvolvementConfig{
			Enabled:       true,
			MaxIterations: 0,
			Timeout:       time.Minute,
		}
		phase := NewSelfEvolvementPhase(cfg, &mockSelfEvolvementLLM{})

		assert.Equal(t, 2, phase.config.MaxIterations)
	})

	t.Run("negative max iterations defaults to 2", func(t *testing.T) {
			t.Parallel()
		cfg := SelfEvolvementConfig{
			Enabled:       true,
			MaxIterations: -1,
			Timeout:       time.Minute,
		}
		phase := NewSelfEvolvementPhase(cfg, &mockSelfEvolvementLLM{})

		assert.Equal(t, 2, phase.config.MaxIterations)
	})

	t.Run("zero timeout defaults to 2 minutes", func(t *testing.T) {
			t.Parallel()
		cfg := SelfEvolvementConfig{
			Enabled:       true,
			MaxIterations: 3,
			Timeout:       0,
		}
		phase := NewSelfEvolvementPhase(cfg, &mockSelfEvolvementLLM{})

		assert.Equal(t, 2*time.Minute, phase.config.Timeout)
	})
}

// ==========================================================================
// Execute — disabled (Skipped)
// ==========================================================================

func TestSelfEvolvementPhase_Execute_Disabled(t *testing.T) {
	t.Parallel()
	cfg := SelfEvolvementConfig{
		Enabled:       false,
		MaxIterations: 2,
		Timeout:       time.Minute,
	}
	mock := &mockSelfEvolvementLLM{}
	phase := NewSelfEvolvementPhase(cfg, mock)

	result, err := phase.Execute(
		context.Background(),
		"agent-1",
		"def add(a, b): return a + b",
		"implement addition",
		"python",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Skipped,
		"should be skipped when disabled")
	assert.Equal(t, "def add(a, b): return a + b",
		result.FinalSolution)
	assert.Equal(t, 0, result.Iterations)
	assert.Equal(t, float64(0), result.FinalPassRate)
	assert.Equal(t, 0, mock.callIdx,
		"LLM should not be called when disabled")
}

// ==========================================================================
// Execute — all tests pass on first iteration
// ==========================================================================

func TestSelfEvolvementPhase_Execute_AllPassFirst(t *testing.T) {
	t.Parallel()
	cfg := SelfEvolvementConfig{
		Enabled:       true,
		MaxIterations: 3,
		Timeout:       30 * time.Second,
	}

	// Response 1: generateSelfTests returns tests
	// Response 2: executeSelfTests — LLM evaluates test 1 (matches expected)
	// Response 3: executeSelfTests — LLM evaluates test 2 (matches expected)
	mock := &mockSelfEvolvementLLM{
		responses: []string{
			// generateSelfTests
			"TEST: test_basic_add\n" +
				"INPUT: add(2, 3)\n" +
				"EXPECTED: 5\n" +
				"---\n" +
				"TEST: test_negative_add\n" +
				"INPUT: add(-1, 1)\n" +
				"EXPECTED: 0\n",
			// executeSelfTests — test 1 evaluation
			"5",
			// executeSelfTests — test 2 evaluation
			"0",
		},
	}

	phase := NewSelfEvolvementPhase(cfg, mock)

	result, err := phase.Execute(
		context.Background(),
		"agent-1",
		"def add(a, b): return a + b",
		"implement addition function",
		"python",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Skipped)
	assert.Equal(t, 1, result.Iterations,
		"should complete in 1 iteration when all tests pass")
	assert.InDelta(t, 1.0, result.FinalPassRate, 1e-9,
		"all tests should pass")
	assert.Empty(t, result.Improvements,
		"no improvements needed when all pass")
	assert.Equal(t, "def add(a, b): return a + b",
		result.FinalSolution,
		"solution should be unchanged")
}

// ==========================================================================
// Execute — refine after failure
// ==========================================================================

func TestSelfEvolvementPhase_Execute_RefineAfterFailure(t *testing.T) {
	t.Parallel()
	cfg := SelfEvolvementConfig{
		Enabled:       true,
		MaxIterations: 2,
		Timeout:       30 * time.Second,
	}

	// Iteration 0:
	//   generateSelfTests -> 2 tests
	//   executeSelfTests -> test 1 passes, test 2 fails
	//   refineSolution -> improved solution
	// Iteration 1:
	//   generateSelfTests -> 2 tests again
	//   executeSelfTests -> both pass now
	mock := &mockSelfEvolvementLLM{
		responses: []string{
			// Iter 0: generateSelfTests
			"TEST: test_positive\nINPUT: abs(5)\nEXPECTED: 5\n---\n" +
				"TEST: test_negative\nINPUT: abs(-3)\nEXPECTED: 3\n",
			// Iter 0: executeSelfTests test 1 — correct
			"5",
			// Iter 0: executeSelfTests test 2 — wrong
			"-3",
			// Iter 0: refineSolution
			"SOLUTION:\ndef abs(x): return x if x >= 0 else -x\n" +
				"IMPROVEMENT:\nFixed negative number handling",
			// Iter 1: generateSelfTests
			"TEST: test_positive\nINPUT: abs(5)\nEXPECTED: 5\n---\n" +
				"TEST: test_negative\nINPUT: abs(-3)\nEXPECTED: 3\n",
			// Iter 1: executeSelfTests test 1 — correct
			"5",
			// Iter 1: executeSelfTests test 2 — now correct
			"3",
		},
	}

	phase := NewSelfEvolvementPhase(cfg, mock)

	result, err := phase.Execute(
		context.Background(),
		"agent-1",
		"def abs(x): return x",
		"implement absolute value",
		"python",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Skipped)
	assert.Equal(t, 2, result.Iterations)
	assert.InDelta(t, 1.0, result.FinalPassRate, 1e-9)
	assert.NotEmpty(t, result.Improvements)
	assert.Contains(t, result.FinalSolution,
		"def abs(x): return x if x >= 0 else -x")
}

// ==========================================================================
// parseSelfTests
// ==========================================================================

func TestSelfEvolvementPhase_ParseSelfTests(t *testing.T) {
	t.Parallel()
	phase := NewSelfEvolvementPhase(
		DefaultSelfEvolvementConfig(), nil,
	)

	tests := []struct {
		name          string
		response      string
		expectedCount int
		checkFirst    *SelfTestResult
	}{
		{
			name: "two valid tests",
			response: "TEST: test_add\nINPUT: add(1,2)\nEXPECTED: 3\n" +
				"---\n" +
				"TEST: test_sub\nINPUT: sub(5,3)\nEXPECTED: 2\n",
			expectedCount: 2,
			checkFirst: &SelfTestResult{
				TestName: "test_add",
				Input:    "add(1,2)",
				Expected: "3",
			},
		},
		{
			name:          "empty response",
			response:      "",
			expectedCount: 0,
		},
		{
			name: "missing expected value is skipped",
			response: "TEST: test_incomplete\nINPUT: foo()\n---\n" +
				"TEST: test_complete\nINPUT: bar()\nEXPECTED: ok\n",
			expectedCount: 1,
			checkFirst: &SelfTestResult{
				TestName: "test_complete",
				Input:    "bar()",
				Expected: "ok",
			},
		},
		{
			name: "missing test name is skipped",
			response: "INPUT: baz()\nEXPECTED: result\n---\n" +
				"TEST: valid_test\nINPUT: x()\nEXPECTED: y\n",
			expectedCount: 1,
			checkFirst: &SelfTestResult{
				TestName: "valid_test",
				Input:    "x()",
				Expected: "y",
			},
		},
		{
			name: "single test without separator",
			response: "TEST: single_test\n" +
				"INPUT: compute(42)\n" +
				"EXPECTED: 84\n",
			expectedCount: 1,
			checkFirst: &SelfTestResult{
				TestName: "single_test",
				Input:    "compute(42)",
				Expected: "84",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			results := phase.parseSelfTests(tc.response)

			assert.Len(t, results, tc.expectedCount)
			if tc.checkFirst != nil && tc.expectedCount > 0 {
				assert.Equal(t, tc.checkFirst.TestName,
					results[0].TestName)
				assert.Equal(t, tc.checkFirst.Input,
					results[0].Input)
				assert.Equal(t, tc.checkFirst.Expected,
					results[0].Expected)
			}
		})
	}
}

// ==========================================================================
// parseRefinedSolution
// ==========================================================================

func TestSelfEvolvementPhase_ParseRefinedSolution(t *testing.T) {
	t.Parallel()
	phase := NewSelfEvolvementPhase(
		DefaultSelfEvolvementConfig(), nil,
	)

	tests := []struct {
		name                string
		response            string
		expectedSolution    string
		expectedImprovement string
	}{
		{
			name: "both markers present",
			response: "SOLUTION:\ndef add(a, b): return a + b\n" +
				"IMPROVEMENT:\nFixed return type",
			expectedSolution:    "def add(a, b): return a + b",
			expectedImprovement: "Fixed return type",
		},
		{
			name:                "only solution marker",
			response:            "SOLUTION:\ndef mul(a, b): return a * b",
			expectedSolution:    "def mul(a, b): return a * b",
			expectedImprovement: "",
		},
		{
			name:                "no markers treats entire response as solution",
			response:            "def div(a, b): return a / b",
			expectedSolution:    "def div(a, b): return a / b",
			expectedImprovement: "",
		},
		{
			name: "improvement before solution (unusual order)",
			response: "IMPROVEMENT:\nReordered logic\n" +
				"SOLUTION:\ndef fixed(): pass",
			expectedSolution:    "def fixed(): pass",
			expectedImprovement: "Reordered logic",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			solution, improvement := phase.parseRefinedSolution(
				tc.response,
			)
			assert.Equal(t, tc.expectedSolution, solution)
			assert.Equal(t, tc.expectedImprovement, improvement)
		})
	}
}
