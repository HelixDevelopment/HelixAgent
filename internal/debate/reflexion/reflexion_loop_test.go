package reflexion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTestExecutor implements the TestExecutor interface for testing.
type mockTestExecutor struct {
	// calls tracks how many times Execute was called.
	calls   int
	results [][]*TestResult
	errs    []error
}

func (m *mockTestExecutor) Execute(
	_ context.Context,
	_ string,
	_ string,
) ([]*TestResult, error) {
	idx := m.calls
	m.calls++

	if idx < len(m.errs) && m.errs[idx] != nil {
		return nil, m.errs[idx]
	}
	if idx < len(m.results) {
		return m.results[idx], nil
	}
	// Default: all pass.
	return []*TestResult{{Name: "default", Passed: true}}, nil
}

func TestDefaultReflexionConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultReflexionConfig()

	assert.Equal(t, 3, cfg.MaxAttempts)
	assert.InDelta(t, 0.95, cfg.ConfidenceThreshold, 0.001)
	assert.Equal(t, 5*time.Minute, cfg.Timeout)
}

func TestNewReflexionLoop(t *testing.T) {
	t.Parallel()
	t.Run("with valid config", func(t *testing.T) {
		cfg := ReflexionConfig{
			MaxAttempts:         5,
			ConfidenceThreshold: 0.9,
			Timeout:             10 * time.Minute,
		}
		gen := NewReflectionGenerator(&mockLLMClient{})
		exec := &mockTestExecutor{}
		mem := NewEpisodicMemoryBuffer(100)

		loop := NewReflexionLoop(cfg, gen, exec, mem)
		require.NotNil(t, loop)
		assert.Equal(t, 5, loop.config.MaxAttempts)
		assert.InDelta(t, 0.9, loop.config.ConfidenceThreshold, 0.001)
	})

	t.Run("defaults applied for zero values", func(t *testing.T) {
			t.Parallel()
		cfg := ReflexionConfig{} // all zeroes
		loop := NewReflexionLoop(cfg, nil, nil, nil)
		require.NotNil(t, loop)
		assert.Equal(t, DefaultReflexionConfig().MaxAttempts, loop.config.MaxAttempts)
		assert.InDelta(t,
			DefaultReflexionConfig().ConfidenceThreshold,
			loop.config.ConfidenceThreshold,
			0.001,
		)
		assert.Equal(t, DefaultReflexionConfig().Timeout, loop.config.Timeout)
	})
}

func TestReflexionLoop_Execute_NilTask(t *testing.T) {
	t.Parallel()
	loop := NewReflexionLoop(DefaultReflexionConfig(), nil, &mockTestExecutor{}, nil)

	result, err := loop.Execute(context.Background(), nil)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task must not be nil")
}

func TestReflexionLoop_Execute_NilExecutor(t *testing.T) {
	t.Parallel()
	loop := NewReflexionLoop(DefaultReflexionConfig(), nil, nil, nil)

	task := &ReflexionTask{
		Description: "test",
		InitialCode: "code",
		Language:    "go",
		AgentID:     "agent-1",
	}
	result, err := loop.Execute(context.Background(), task)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test executor must not be nil")
}

func TestReflexionLoop_Execute_AllPassFirstAttempt(t *testing.T) {
	t.Parallel()
	executor := &mockTestExecutor{
		results: [][]*TestResult{
			{
				{Name: "TestAdd", Passed: true, Output: "ok"},
				{Name: "TestSub", Passed: true, Output: "ok"},
			},
		},
	}

	gen := NewReflectionGenerator(&mockLLMClient{})
	mem := NewEpisodicMemoryBuffer(100)
	loop := NewReflexionLoop(DefaultReflexionConfig(), gen, executor, mem)

	task := &ReflexionTask{
		Description: "implement arithmetic",
		InitialCode: "func add(a, b int) int { return a + b }",
		Language:    "go",
		AgentID:     "agent-1",
		SessionID:   "session-1",
	}

	result, err := loop.Execute(context.Background(), task)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.AllPassed)
	assert.Equal(t, 1, result.Attempts)
	assert.InDelta(t, 1.0, result.FinalConfidence, 0.001)
	assert.Equal(t, task.InitialCode, result.FinalCode)
	assert.Empty(t, result.Reflections)
	assert.Empty(t, result.Episodes)
	assert.Len(t, result.TestResults, 2)
	assert.True(t, result.Duration > 0)
}

func TestReflexionLoop_Execute_FailThenPass(t *testing.T) {
	t.Parallel()
	executor := &mockTestExecutor{
		results: [][]*TestResult{
			// First attempt: one test fails.
			{
				{Name: "TestAdd", Passed: true, Output: "ok"},
				{Name: "TestEdge", Passed: false, Error: "expected 0, got -1"},
			},
			// Second attempt: all pass.
			{
				{Name: "TestAdd", Passed: true, Output: "ok"},
				{Name: "TestEdge", Passed: true, Output: "ok"},
			},
		},
	}

	llmResponse := `ROOT_CAUSE: Edge case not handled
WHAT_WENT_WRONG: Negative numbers not supported
WHAT_TO_CHANGE: Add check for negative input
CONFIDENCE: 0.7`

	gen := NewReflectionGenerator(&mockLLMClient{response: llmResponse})
	mem := NewEpisodicMemoryBuffer(100)

	callCount := 0
	codeGen := func(
		_ context.Context,
		_ string,
		_ []*Reflection,
	) (string, error) {
		callCount++
		return "func add(a, b int) int { if a < 0 { a = 0 }; return a + b }", nil
	}

	loop := NewReflexionLoop(
		ReflexionConfig{
			MaxAttempts:         5,
			ConfidenceThreshold: 0.95,
			Timeout:             30 * time.Second,
		},
		gen,
		executor,
		mem,
	)

	task := &ReflexionTask{
		Description:   "implement addition with edge cases",
		InitialCode:   "func add(a, b int) int { return a + b }",
		Language:      "go",
		AgentID:       "agent-1",
		SessionID:     "session-1",
		CodeGenerator: codeGen,
	}

	result, err := loop.Execute(context.Background(), task)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.AllPassed)
	assert.Equal(t, 2, result.Attempts)
	assert.Len(t, result.Reflections, 1)
	assert.Len(t, result.Episodes, 1)
	assert.Equal(t, 1, callCount, "code generator called once")
	assert.Equal(t, 1, mem.Size(), "episode stored in memory")
}

func TestReflexionLoop_Execute_MaxAttempts(t *testing.T) {
	t.Parallel()
	// All attempts fail.
	failResult := []*TestResult{
		{Name: "TestMain", Passed: false, Error: "assertion mismatch"},
	}

	executor := &mockTestExecutor{
		results: [][]*TestResult{
			failResult,
			failResult,
			failResult,
		},
	}

	llmResponse := `ROOT_CAUSE: Logic error
WHAT_WENT_WRONG: Wrong computation
WHAT_TO_CHANGE: Fix the algorithm
CONFIDENCE: 0.4`

	gen := NewReflectionGenerator(&mockLLMClient{response: llmResponse})
	mem := NewEpisodicMemoryBuffer(100)

	codeGenCalls := 0
	codeGen := func(
		_ context.Context,
		_ string,
		reflections []*Reflection,
	) (string, error) {
		codeGenCalls++
		return "still broken code", nil
	}

	loop := NewReflexionLoop(
		ReflexionConfig{
			MaxAttempts:         3,
			ConfidenceThreshold: 0.95,
			Timeout:             30 * time.Second,
		},
		gen,
		executor,
		mem,
	)

	task := &ReflexionTask{
		Description:   "fix broken function",
		InitialCode:   "broken code",
		Language:      "go",
		AgentID:       "agent-1",
		SessionID:     "session-1",
		CodeGenerator: codeGen,
	}

	result, err := loop.Execute(context.Background(), task)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.AllPassed)
	assert.Equal(t, 3, result.Attempts)
	assert.Len(t, result.Reflections, 3)
	assert.Len(t, result.Episodes, 3)
	// Code generator called for attempts 2 and 3 (after failures on 1 and 2).
	assert.Equal(t, 2, codeGenCalls)
	assert.Equal(t, 3, mem.Size())
}

func TestReflexionLoop_Execute_NoCodeGenerator(t *testing.T) {
	t.Parallel()
	// No initial code AND no code generator: should error.
	executor := &mockTestExecutor{}
	loop := NewReflexionLoop(DefaultReflexionConfig(), nil, executor, nil)

	task := &ReflexionTask{
		Description: "something",
		InitialCode: "", // empty
		Language:    "go",
		AgentID:     "agent-1",
		// CodeGenerator is nil
	}

	result, err := loop.Execute(context.Background(), task)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "neither initial code nor code generator")
}

func TestReflexionLoop_Execute_NoCodeGenerator_WithInitialCode(t *testing.T) {
	t.Parallel()
	// Has initial code but no code generator. Tests fail: should return
	// without error (cannot improve, just reports result).
	executor := &mockTestExecutor{
		results: [][]*TestResult{
			{
				{Name: "TestMain", Passed: false, Error: "wrong output"},
			},
		},
	}

	loop := NewReflexionLoop(DefaultReflexionConfig(), nil, executor, nil)

	task := &ReflexionTask{
		Description: "test static code",
		InitialCode: "func main() {}",
		Language:    "go",
		AgentID:     "agent-1",
	}

	result, err := loop.Execute(context.Background(), task)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.AllPassed)
	assert.Equal(t, 1, result.Attempts)
}

func TestReflexionLoop_Execute_CodeGeneratorError(t *testing.T) {
	t.Parallel()
	// First test fails, then code generator fails.
	executor := &mockTestExecutor{
		results: [][]*TestResult{
			{{Name: "TestMain", Passed: false, Error: "fail"}},
		},
	}

	gen := NewReflectionGenerator(&mockLLMClient{
		response: `ROOT_CAUSE: err
WHAT_WENT_WRONG: err
WHAT_TO_CHANGE: fix
CONFIDENCE: 0.3`,
	})

	codeGen := func(
		_ context.Context,
		_ string,
		_ []*Reflection,
	) (string, error) {
		return "", errors.New("code generation service down")
	}

	loop := NewReflexionLoop(
		ReflexionConfig{
			MaxAttempts:         3,
			ConfidenceThreshold: 0.95,
			Timeout:             30 * time.Second,
		},
		gen,
		executor,
		nil,
	)

	task := &ReflexionTask{
		Description:   "test code gen failure",
		InitialCode:   "bad code",
		Language:      "go",
		AgentID:       "agent-1",
		CodeGenerator: codeGen,
	}

	result, err := loop.Execute(context.Background(), task)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code generation failed")
}

func TestReflexionLoop_Execute_TestExecutorError(t *testing.T) {
	t.Parallel()
	executor := &mockTestExecutor{
		errs: []error{errors.New("executor crashed")},
	}

	loop := NewReflexionLoop(DefaultReflexionConfig(), nil, executor, nil)

	task := &ReflexionTask{
		Description: "test exec error",
		InitialCode: "some code",
		Language:    "go",
		AgentID:     "agent-1",
	}

	result, err := loop.Execute(context.Background(), task)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test execution failed")
}

func TestReflexionLoop_Execute_InitialCodeGeneration(t *testing.T) {
	t.Parallel()
	// No initial code but code generator provided.
	executor := &mockTestExecutor{
		results: [][]*TestResult{
			{{Name: "TestMain", Passed: true}},
		},
	}

	codeGen := func(
		_ context.Context,
		task string,
		reflections []*Reflection,
	) (string, error) {
		assert.Nil(t, reflections, "first call has no reflections")
		return "generated code", nil
	}

	loop := NewReflexionLoop(DefaultReflexionConfig(), nil, executor, nil)

	task := &ReflexionTask{
		Description:   "generate from scratch",
		InitialCode:   "",
		Language:      "go",
		AgentID:       "agent-1",
		CodeGenerator: codeGen,
	}

	result, err := loop.Execute(context.Background(), task)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.AllPassed)
	assert.Equal(t, "generated code", result.FinalCode)
}

func TestReflexionLoop_Execute_InitialCodeGenerationError(t *testing.T) {
	t.Parallel()
	codeGen := func(
		_ context.Context,
		_ string,
		_ []*Reflection,
	) (string, error) {
		return "", errors.New("cannot generate")
	}

	loop := NewReflexionLoop(DefaultReflexionConfig(), nil, &mockTestExecutor{}, nil)

	task := &ReflexionTask{
		Description:   "generate from scratch",
		InitialCode:   "",
		Language:      "go",
		AgentID:       "agent-1",
		CodeGenerator: codeGen,
	}

	result, err := loop.Execute(context.Background(), task)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initial code generation failed")
}
