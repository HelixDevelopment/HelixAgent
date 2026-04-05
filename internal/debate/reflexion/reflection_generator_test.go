package reflexion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLMClient implements the LLMClient interface for testing.
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Complete(
	_ context.Context,
	_ string,
) (string, error) {
	return m.response, m.err
}

func TestNewReflectionGenerator(t *testing.T) {
	t.Parallel()
	client := &mockLLMClient{response: "ok"}
	gen := NewReflectionGenerator(client)
	require.NotNil(t, gen)
	assert.Equal(t, client, gen.llmClient)
}

func TestReflectionGenerator_Generate_WithMockLLM(t *testing.T) {
	t.Parallel()
	llmResponse := `ROOT_CAUSE: Off-by-one error in loop boundary
WHAT_WENT_WRONG: The loop iterates one extra time causing index out of bounds
WHAT_TO_CHANGE: Use < instead of <= in the loop condition
CONFIDENCE: 0.85`

	client := &mockLLMClient{response: llmResponse}
	gen := NewReflectionGenerator(client)

	req := &ReflectionRequest{
		Code:            "for i := 0; i <= len(arr); i++ {}",
		TestResults:     map[string]interface{}{"test1": "fail"},
		ErrorMessages:   []string{"index out of range"},
		TaskDescription: "implement array traversal",
		AttemptNumber:   1,
	}

	reflection, err := gen.Generate(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, reflection)

	assert.Equal(t, "Off-by-one error in loop boundary", reflection.RootCause)
	assert.Equal(t,
		"The loop iterates one extra time causing index out of bounds",
		reflection.WhatWentWrong,
	)
	assert.Equal(t,
		"Use < instead of <= in the loop condition",
		reflection.WhatToChangeNext,
	)
	assert.InDelta(t, 0.85, reflection.ConfidenceInFix, 0.001)
	assert.False(t, reflection.GeneratedAt.IsZero())
}

func TestReflectionGenerator_Generate_NilRequest(t *testing.T) {
	t.Parallel()
	client := &mockLLMClient{}
	gen := NewReflectionGenerator(client)

	reflection, err := gen.Generate(context.Background(), nil)
	assert.Nil(t, reflection)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be nil")
}

func TestReflectionGenerator_Generate_Fallback(t *testing.T) {
	t.Parallel()
	// LLM returns an error; generator should fall back to deterministic.
	client := &mockLLMClient{
		response: "",
		err:      errors.New("service unavailable"),
	}
	gen := NewReflectionGenerator(client)

	req := &ReflectionRequest{
		Code:            "func broken() {}",
		TestResults:     map[string]interface{}{"test1": "fail"},
		ErrorMessages:   []string{"compile error: syntax error"},
		TaskDescription: "fix compilation",
		AttemptNumber:   1,
	}

	reflection, err := gen.Generate(context.Background(), req)
	require.NoError(t, err, "fallback should not return error")
	require.NotNil(t, reflection)

	// Fallback should detect "compile"/"syntax" keywords.
	assert.Contains(t, reflection.RootCause, "ompilation")
	assert.Contains(t, reflection.WhatWentWrong, "syntax")
	assert.InDelta(t, 0.6, reflection.ConfidenceInFix, 0.01)
}

func TestReflectionGenerator_Generate_FallbackOnUnparseableResponse(t *testing.T) {
	t.Parallel()
	// LLM returns something that cannot be parsed.
	client := &mockLLMClient{
		response: "I'm not sure what happened. Let me think...",
		err:      nil,
	}
	gen := NewReflectionGenerator(client)

	req := &ReflectionRequest{
		Code:            "func broken() {}",
		TestResults:     map[string]interface{}{},
		ErrorMessages:   []string{"assertion failed: expected 42, got 0"},
		TaskDescription: "fix logic",
		AttemptNumber:   1,
	}

	reflection, err := gen.Generate(context.Background(), req)
	require.NoError(t, err, "fallback should be used for unparseable response")
	require.NotNil(t, reflection)
	// Fallback should detect "assert"/"expected"/"got" keywords.
	assert.NotEmpty(t, reflection.RootCause)
}

func TestReflectionGenerator_ParseReflectionResponse(t *testing.T) {
	t.Parallel()
	gen := NewReflectionGenerator(&mockLLMClient{})

	tests := []struct {
		name       string
		response   string
		wantErr    bool
		errContain string
		rootCause  string
		confidence float64
	}{
		{
			name: "valid complete response",
			response: `ROOT_CAUSE: missing nil check
WHAT_WENT_WRONG: nil pointer dereference
WHAT_TO_CHANGE: add guard clause
CONFIDENCE: 0.75`,
			wantErr:    false,
			rootCause:  "missing nil check",
			confidence: 0.75,
		},
		{
			name: "valid with extra whitespace",
			response: `
  ROOT_CAUSE: whitespace issue
  WHAT_WENT_WRONG: extra spaces
  WHAT_TO_CHANGE: trim input
  CONFIDENCE: 0.5
`,
			wantErr:    false,
			rootCause:  "whitespace issue",
			confidence: 0.5,
		},
		{
			name: "missing ROOT_CAUSE",
			response: `WHAT_WENT_WRONG: something
WHAT_TO_CHANGE: fix it
CONFIDENCE: 0.5`,
			wantErr:    true,
			errContain: "ROOT_CAUSE",
		},
		{
			name: "missing CONFIDENCE",
			response: `ROOT_CAUSE: issue
WHAT_WENT_WRONG: something
WHAT_TO_CHANGE: fix it`,
			wantErr:    true,
			errContain: "CONFIDENCE",
		},
		{
			name: "invalid confidence value",
			response: `ROOT_CAUSE: issue
WHAT_WENT_WRONG: something
WHAT_TO_CHANGE: fix it
CONFIDENCE: not_a_number`,
			wantErr:    true,
			errContain: "failed to parse confidence",
		},
		{
			name:       "empty response",
			response:   "",
			wantErr:    true,
			errContain: "missing required fields",
		},
		{
			name: "missing multiple fields",
			response: `ROOT_CAUSE: only this
CONFIDENCE: 0.5`,
			wantErr:    true,
			errContain: "WHAT_WENT_WRONG",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			reflection, err := gen.parseReflectionResponse(tc.response)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errContain != "" {
					assert.Contains(t, err.Error(), tc.errContain)
				}
				assert.Nil(t, reflection)
			} else {
				require.NoError(t, err)
				require.NotNil(t, reflection)
				assert.Equal(t, tc.rootCause, reflection.RootCause)
				assert.InDelta(t, tc.confidence, reflection.ConfidenceInFix, 0.001)
				assert.False(t, reflection.GeneratedAt.IsZero())
			}
		})
	}
}

func TestReflectionGenerator_BuildReflectionPrompt(t *testing.T) {
	t.Parallel()
	gen := NewReflectionGenerator(&mockLLMClient{})

	t.Run("basic prompt", func(t *testing.T) {
			t.Parallel()
		req := &ReflectionRequest{
			Code:            "func add(a, b int) int { return a - b }",
			TestResults:     map[string]interface{}{"test_add": "failed"},
			ErrorMessages:   []string{"expected 5, got -1"},
			TaskDescription: "implement addition",
			AttemptNumber:   2,
		}

		prompt := gen.buildReflectionPrompt(req)

		assert.Contains(t, prompt, "ROOT_CAUSE:")
		assert.Contains(t, prompt, "WHAT_WENT_WRONG:")
		assert.Contains(t, prompt, "WHAT_TO_CHANGE:")
		assert.Contains(t, prompt, "CONFIDENCE:")
		assert.Contains(t, prompt, "implement addition")
		assert.Contains(t, prompt, "Attempt: 2")
		assert.Contains(t, prompt, "func add")
		assert.Contains(t, prompt, "expected 5, got -1")
		assert.Contains(t, prompt, "Prior Reflections: None")
	})

	t.Run("prompt with prior reflections", func(t *testing.T) {
			t.Parallel()
		req := &ReflectionRequest{
			Code:            "func foo() {}",
			TestResults:     map[string]interface{}{},
			ErrorMessages:   []string{"error"},
			TaskDescription: "fix foo",
			AttemptNumber:   3,
			PriorReflections: []*Reflection{
				{
					RootCause:        "wrong return",
					WhatWentWrong:    "returned nil",
					WhatToChangeNext: "return value",
					ConfidenceInFix:  0.5,
					GeneratedAt:      time.Now(),
				},
			},
		}

		prompt := gen.buildReflectionPrompt(req)

		assert.Contains(t, prompt, "Prior Reflections:")
		assert.Contains(t, prompt, "Reflection 1")
		assert.Contains(t, prompt, "wrong return")
		assert.NotContains(t, prompt, "Prior Reflections: None")
	})
}

func TestReflectionGenerator_GenerateFallbackReflection(t *testing.T) {
	t.Parallel()
	gen := NewReflectionGenerator(&mockLLMClient{})

	tests := []struct {
		name              string
		errorMessages     []string
		expectedRootCause string
		expectedConf      float64
	}{
		{
			name:              "syntax error",
			errorMessages:     []string{"compile error: unexpected token"},
			expectedRootCause: "Compilation or syntax error in the code",
			expectedConf:      0.6,
		},
		{
			name:              "test assertion failure",
			errorMessages:     []string{"assert failed: expected 10, got 5"},
			expectedRootCause: "Test assertion failure due to incorrect logic",
			expectedConf:      0.5,
		},
		{
			name:              "timeout",
			errorMessages:     []string{"context deadline exceeded"},
			expectedRootCause: "Code execution exceeded the time limit",
			expectedConf:      0.5,
		},
		{
			name:              "nil pointer",
			errorMessages:     []string{"nil pointer dereference"},
			expectedRootCause: "Null/nil pointer dereference at runtime",
			expectedConf:      0.6,
		},
		{
			name:              "index out of range",
			errorMessages:     []string{"index out of range [5] with length 3"},
			expectedRootCause: "Array or slice index out of bounds",
			expectedConf:      0.6,
		},
		{
			name:              "permission denied",
			errorMessages:     []string{"permission denied: cannot write"},
			expectedRootCause: "Permission or authorization error",
			expectedConf:      0.4,
		},
		{
			name:              "import error",
			errorMessages:     []string{"missing import for package foo/bar"},
			expectedRootCause: "Missing or incorrect import/dependency",
			expectedConf:      0.6,
		},
		{
			name:              "type mismatch",
			errorMessages:     []string{"incompatible type conversion"},
			expectedRootCause: "Type mismatch or invalid type conversion",
			expectedConf:      0.5,
		},
		{
			name:              "deadlock",
			errorMessages:     []string{"all goroutines are asleep - deadlock!"},
			expectedRootCause: "Concurrency issue such as deadlock or race condition",
			expectedConf:      0.4,
		},
		{
			name:              "out of memory",
			errorMessages:     []string{"out of memory: allocation failed"},
			expectedRootCause: "Excessive memory usage or allocation failure",
			expectedConf:      0.4,
		},
		{
			name:              "unknown error",
			errorMessages:     []string{"something weird happened"},
			expectedRootCause: "Unknown error in code attempt",
			expectedConf:      0.3,
		},
		{
			name:              "no errors",
			errorMessages:     nil,
			expectedRootCause: "Unknown error in code attempt",
			expectedConf:      0.3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			req := &ReflectionRequest{
				Code:            "func test() {}",
				ErrorMessages:   tc.errorMessages,
				TaskDescription: "test task",
				AttemptNumber:   1,
			}

			reflection := gen.generateFallbackReflection(req)
			require.NotNil(t, reflection)
			assert.Equal(t, tc.expectedRootCause, reflection.RootCause)
			assert.InDelta(t, tc.expectedConf, reflection.ConfidenceInFix, 0.01)
			assert.False(t, reflection.GeneratedAt.IsZero())
		})
	}

	t.Run("with prior reflections reduces confidence", func(t *testing.T) {
			t.Parallel()
		prior := &Reflection{
			RootCause:        "previous cause",
			WhatWentWrong:    "previous wrong",
			WhatToChangeNext: "previous fix",
			ConfidenceInFix:  0.5,
			GeneratedAt:      time.Now(),
		}

		req := &ReflectionRequest{
			Code:             "func test() {}",
			ErrorMessages:    []string{"compile error: syntax issue"},
			TaskDescription:  "test task",
			AttemptNumber:    2,
			PriorReflections: []*Reflection{prior},
		}

		reflection := gen.generateFallbackReflection(req)
		require.NotNil(t, reflection)
		// 0.6 * 0.9 = 0.54 for syntax error with prior reflection.
		assert.InDelta(t, 0.54, reflection.ConfidenceInFix, 0.01)
		assert.Contains(t, reflection.WhatToChangeNext, "Previous fix")
		assert.Contains(t, reflection.WhatToChangeNext, "previous fix")
	})
}
