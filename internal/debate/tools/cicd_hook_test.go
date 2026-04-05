package tools

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// CI/CD Hook Tests
// =============================================================================

func TestDefaultHookConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultHookConfig()

	assert.True(t, cfg.Enabled)
	assert.False(t, cfg.FailFast)
	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.NotNil(t, cfg.HookPoints)

	// Post-proposal should have run_linter.
	pp, ok := cfg.HookPoints[HookPostProposal]
	require.True(t, ok)
	assert.Contains(t, pp, ActionRunLinter)

	// Post-optimization should have run_tests and static_analysis.
	po, ok := cfg.HookPoints[HookPostOptimization]
	require.True(t, ok)
	assert.Contains(t, po, ActionRunTests)
	assert.Contains(t, po, ActionStaticAnalysis)

	// Post-adversarial should have security_scan.
	pa, ok := cfg.HookPoints[HookPostAdversarial]
	require.True(t, ok)
	assert.Contains(t, pa, ActionSecurityScan)

	// Post-convergence should have all four.
	pc, ok := cfg.HookPoints[HookPostConvergence]
	require.True(t, ok)
	assert.Len(t, pc, 4)
}

func TestNewCICDHook(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	cfg := DefaultHookConfig()

	hook := NewCICDHook(cfg, executor)
	require.NotNil(t, hook)
	assert.True(t, hook.IsEnabled())
	assert.Equal(t, executor, hook.executor)
}

func TestCICDHook_Configure(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	hook := NewCICDHook(HookConfig{}, executor)

	// Initially no hook points.
	hooks := hook.GetConfiguredHooks()
	assert.Empty(t, hooks)

	// Configure a new hook point.
	hook.Configure(HookPreDebate, []HookAction{ActionRunTests, ActionRunLinter})

	hooks = hook.GetConfiguredHooks()
	require.Contains(t, hooks, HookPreDebate)
	assert.Len(t, hooks[HookPreDebate], 2)
	assert.Equal(t, ActionRunTests, hooks[HookPreDebate][0])
	assert.Equal(t, ActionRunLinter, hooks[HookPreDebate][1])
}

func TestCICDHook_Configure_Overwrite(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	hook := NewCICDHook(DefaultHookConfig(), executor)

	// Overwrite post-proposal to use security scan only.
	hook.Configure(HookPostProposal, []HookAction{ActionSecurityScan})

	hooks := hook.GetConfiguredHooks()
	require.Contains(t, hooks, HookPostProposal)
	assert.Len(t, hooks[HookPostProposal], 1)
	assert.Equal(t, ActionSecurityScan, hooks[HookPostProposal][0])
}

func TestCICDHook_Execute_Disabled(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	cfg := DefaultHookConfig()
	cfg.Enabled = false

	hook := NewCICDHook(cfg, executor)

	result, err := hook.Execute(
		context.Background(), HookPostProposal, "some code", "go",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.AllPassed)
	assert.Empty(t, result.Actions)
}

func TestCICDHook_Execute_NoHooksConfigured(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	cfg := HookConfig{
		Enabled:    true,
		HookPoints: map[HookPoint][]HookAction{},
		Timeout:    30 * time.Second,
	}

	hook := NewCICDHook(cfg, executor)

	result, err := hook.Execute(
		context.Background(), HookPostProposal, "code", "go",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.AllPassed)
	assert.Empty(t, result.Actions)
}

func TestCICDHook_Execute_WithActions(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()

	// Simple Go code that should pass linting and has test patterns.
	goCode := `package main

import "fmt"

// Hello returns a greeting.
func Hello(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

func TestHello(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Hello, World!", Hello("World"))
}
`

	cfg := HookConfig{
		Enabled: true,
		HookPoints: map[HookPoint][]HookAction{
			HookPostProposal: {ActionRunTests, ActionRunLinter},
		},
		Timeout: 30 * time.Second,
	}

	hook := NewCICDHook(cfg, executor)
	ctx := context.Background()

	result, err := hook.Execute(ctx, HookPostProposal, goCode, "go")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, HookPostProposal, result.HookPoint)
	assert.Len(t, result.Actions, 2)

	// Tests should be detected (func TestHello, assert.).
	testResult, ok := result.Actions[ActionRunTests]
	require.True(t, ok)
	assert.True(t, testResult.Passed)

	// Linter result should exist.
	lintResult, ok := result.Actions[ActionRunLinter]
	require.True(t, ok)
	assert.NotNil(t, lintResult)
}

func TestCICDHook_Execute_FailFast(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()

	// Code with no test patterns should fail run_tests.
	badCode := "x := 1"

	cfg := HookConfig{
		Enabled:  true,
		FailFast: true,
		HookPoints: map[HookPoint][]HookAction{
			HookPostDebate: {
				ActionRunTests,
				ActionRunLinter,
				ActionSecurityScan,
			},
		},
		Timeout: 30 * time.Second,
	}

	hook := NewCICDHook(cfg, executor)
	result, err := hook.Execute(
		context.Background(), HookPostDebate, badCode, "go",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.AllPassed)

	// With fail-fast, only the first action (run_tests) should be present
	// since it fails.
	assert.Len(t, result.Actions, 1)
	assert.Contains(t, result.Actions, ActionRunTests)
}

func TestCICDHook_EnableDisable(t *testing.T) {
	t.Parallel()
	hook := NewCICDHook(DefaultHookConfig(), NewDefaultActionExecutor())

	assert.True(t, hook.IsEnabled())

	hook.Disable()
	assert.False(t, hook.IsEnabled())

	hook.Enable()
	assert.True(t, hook.IsEnabled())
}

func TestCICDHook_GetConfiguredHooks_ReturnsCopy(t *testing.T) {
	t.Parallel()
	hook := NewCICDHook(DefaultHookConfig(), NewDefaultActionExecutor())

	hooks1 := hook.GetConfiguredHooks()
	hooks2 := hook.GetConfiguredHooks()

	// Mutating the returned map should not affect the original.
	delete(hooks1, HookPostProposal)
	_, ok := hooks2[HookPostProposal]
	assert.True(t, ok, "returned map should be a copy")
}

// =============================================================================
// DefaultActionExecutor Tests
// =============================================================================

func TestDefaultActionExecutor_RunTests(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	ctx := context.Background()

	tests := []struct {
		name     string
		code     string
		language string
		passed   bool
	}{
		{
			name:     "go code with test function",
			code:     "func TestSomething(t *testing.T) {\n\tt.Fatal(\"fail\")\n}",
			language: "go",
			passed:   true,
		},
		{
			name:     "go code with assert",
			code:     "assert.Equal(t, expected, actual)",
			language: "go",
			passed:   true,
		},
		{
			name:     "python code with test",
			code:     "def test_something():\n    assert True",
			language: "python",
			passed:   true,
		},
		{
			name:     "javascript code with describe",
			code:     "describe('suite', () => {\n  it('works', () => { expect(true).toBe(true); });\n});",
			language: "javascript",
			passed:   true,
		},
		{
			name:     "no test patterns",
			code:     "func main() { println(1) }",
			language: "go",
			passed:   false,
		},
		{
			name:     "unknown language with test keyword",
			code:     "test something\nassert result",
			language: "ruby",
			passed:   true,
		},
		{
			name:     "unknown language without test keyword",
			code:     "x = 1 + 2",
			language: "ruby",
			passed:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			result, err := executor.Execute(ctx, ActionRunTests, tc.code, tc.language)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, ActionRunTests, result.Action)
			assert.Equal(t, tc.passed, result.Passed)
		})
	}
}

func TestDefaultActionExecutor_RunLinter(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	ctx := context.Background()

	tests := []struct {
		name     string
		code     string
		language string
		passed   bool
	}{
		{
			name:     "clean code",
			code:     "func hello() {\n\treturn\n}",
			language: "go",
			passed:   true,
		},
		{
			name:     "trailing whitespace",
			code:     "func hello() { \n\treturn\n}",
			language: "go",
			passed:   false,
		},
		{
			name: "line too long",
			code: "func hello() {\n\t" +
				"x := \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
				"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"\n}",
			language: "go",
			passed:   false,
		},
		{
			name:     "go fmt.Println in non-test code",
			code:     "func hello() {\n\tfmt.Println(\"debug\")\n}",
			language: "go",
			passed:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			result, err := executor.Execute(ctx, ActionRunLinter, tc.code, tc.language)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, ActionRunLinter, result.Action)
			assert.Equal(t, tc.passed, result.Passed, "output: %s", result.Output)
		})
	}
}

func TestDefaultActionExecutor_SecurityScan(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	ctx := context.Background()

	tests := []struct {
		name     string
		code     string
		language string
		passed   bool
	}{
		{
			name:     "clean code",
			code:     "func hello() string {\n\treturn \"hi\"\n}",
			language: "go",
			passed:   true,
		},
		{
			name:     "go exec.Command without nosec",
			code:     "func run() {\n\texec.Command(\"ls\", \"-la\").Run()\n}",
			language: "go",
			passed:   false,
		},
		{
			name:     "dangerous dynamic execution pattern",
			code:     "result = " + dangerousEvalPattern + "user_input)",
			language: "python",
			passed:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			result, err := executor.Execute(ctx, ActionSecurityScan, tc.code, tc.language)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, ActionSecurityScan, result.Action)
			assert.Equal(t, tc.passed, result.Passed, "output: %s", result.Output)
		})
	}
}

func TestDefaultActionExecutor_StaticAnalysis(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	ctx := context.Background()

	// Simple code with low nesting.
	simpleCode := "func hello() {\n\treturn\n}\n"
	result, err := executor.Execute(
		ctx, ActionStaticAnalysis, simpleCode, "go",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, ActionStaticAnalysis, result.Action)
	assert.True(t, result.Passed)
}

func TestDefaultActionExecutor_Benchmarks(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	ctx := context.Background()

	result, err := executor.Execute(
		ctx, ActionRunBenchmarks, "code", "go",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Passed)
	assert.Contains(t, result.Output, "no performance issues found")
}

func TestDefaultActionExecutor_CustomScript(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	ctx := context.Background()

	result, err := executor.Execute(
		ctx, ActionCustomScript, "code", "go",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Passed)
	assert.Contains(t, result.Output, "custom validation checks passed")
}

func TestDefaultActionExecutor_UnknownAction(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()
	ctx := context.Background()

	result, err := executor.Execute(
		ctx, HookAction("unknown_action"), "code", "go",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Output, "unknown action")
}

func TestDefaultActionExecutor_CancelledContext(t *testing.T) {
	t.Parallel()
	executor := NewDefaultActionExecutor()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	result, err := executor.Execute(ctx, ActionRunTests, "code", "go")
	assert.Error(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkDefaultHookConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultHookConfig()
	}
}

func BenchmarkNewCICDHook(b *testing.B) {
	cfg := DefaultHookConfig()
	executor := NewDefaultActionExecutor()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewCICDHook(cfg, executor)
	}
}

func BenchmarkCICDHook_GetConfiguredHooks(b *testing.B) {
	hook := NewCICDHook(DefaultHookConfig(), NewDefaultActionExecutor())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hook.GetConfiguredHooks()
	}
}

func BenchmarkDefaultActionExecutor_Execute_RunTests(b *testing.B) {
	executor := NewDefaultActionExecutor()
	ctx := context.Background()
	code := "func TestSomething(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.Execute(ctx, ActionRunTests, code, "go") //nolint:errcheck
	}
}

func BenchmarkDefaultActionExecutor_Execute_SecurityScan(b *testing.B) {
	executor := NewDefaultActionExecutor()
	ctx := context.Background()
	code := "func hello() string {\n\treturn \"hi\"\n}"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.Execute(ctx, ActionSecurityScan, code, "go") //nolint:errcheck
	}
}
