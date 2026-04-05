package evaluation

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Benchmark Bridge Tests
// =============================================================================

func TestNewBenchmarkBridge(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	require.NotNil(t, bridge)
	assert.NotNil(t, bridge.evaluators)

	// The custom evaluator should be registered by default.
	_, ok := bridge.evaluators[BenchmarkCustom]
	assert.True(t, ok)
}

// mockEvaluator is a test evaluator that returns a fixed score.
type mockEvaluator struct {
	score float64
	err   error
}

func (m *mockEvaluator) Evaluate(
	ctx context.Context,
	solution string,
	problem *BenchmarkProblem,
) (*EvaluationScore, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &EvaluationScore{
		BenchmarkType: problem.Type,
		Score:         m.score,
		Details: map[string]float64{
			"mock_metric": m.score,
		},
		Metadata: map[string]interface{}{
			"mock": true,
		},
	}, nil
}

func TestBenchmarkBridge_RegisterEvaluator(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()

	eval := &mockEvaluator{score: 0.95}
	bridge.RegisterEvaluator(BenchmarkSWEBench, eval)

	// Verify it was registered.
	bridge.mu.RLock()
	registered, ok := bridge.evaluators[BenchmarkSWEBench]
	bridge.mu.RUnlock()
	assert.True(t, ok)
	assert.Equal(t, eval, registered)
}

func TestBenchmarkBridge_RegisterEvaluator_Overwrite(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()

	eval1 := &mockEvaluator{score: 0.5}
	eval2 := &mockEvaluator{score: 0.9}

	bridge.RegisterEvaluator(BenchmarkHumanEval, eval1)
	bridge.RegisterEvaluator(BenchmarkHumanEval, eval2)

	bridge.mu.RLock()
	registered := bridge.evaluators[BenchmarkHumanEval]
	bridge.mu.RUnlock()
	assert.Equal(t, eval2, registered)
}

func TestBenchmarkBridge_EvaluateDebateResult_CustomEvaluator(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	ctx := context.Background()

	result := &DebateResultForEval{
		ID:    "debate-1",
		Topic: "Implement a binary search function",
		FinalSolution: `package search

import "fmt"

// BinarySearch performs binary search on a sorted slice.
// It returns the index or -1 if not found.
func BinarySearch(arr []int, target int) int {
	if len(arr) == 0 {
		return -1
	}
	left, right := 0, len(arr)-1
	for left <= right {
		mid := left + (right-left)/2
		if arr[mid] == target {
			return mid
		} else if arr[mid] < target {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	return -1
}

func TestBinarySearch(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 2, BinarySearch([]int{1, 2, 3, 4, 5}, 3))
}
`,
		Consensus: 0.85,
		Language:  "go",
	}

	score, err := bridge.EvaluateDebateResult(ctx, result, BenchmarkCustom)
	require.NoError(t, err)
	require.NotNil(t, score)

	assert.Equal(t, BenchmarkCustom, score.BenchmarkType)
	assert.Greater(t, score.Score, 0.0)
	assert.LessOrEqual(t, score.Score, 1.0)

	// Metadata should contain debate context.
	assert.Equal(t, "debate-1", score.Metadata["debate_id"])
	assert.Equal(t, 0.85, score.Metadata["consensus"])

	// Details should have the five custom metrics.
	assert.Contains(t, score.Details, "correctness")
	assert.Contains(t, score.Details, "maintainability")
	assert.Contains(t, score.Details, "performance")
	assert.Contains(t, score.Details, "security")
	assert.Contains(t, score.Details, "test_coverage")

	// Verify individual metric ranges.
	for metric, val := range score.Details {
		assert.GreaterOrEqual(t, val, 0.0, "metric %s below 0", metric)
		assert.LessOrEqual(t, val, 1.0, "metric %s above 1", metric)
	}
}

func TestBenchmarkBridge_EvaluateDebateResult_CustomEvaluatorRegistered(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	ctx := context.Background()

	mock := &mockEvaluator{score: 0.88}
	bridge.RegisterEvaluator(BenchmarkSWEBench, mock)

	result := &DebateResultForEval{
		ID:            "debate-2",
		Topic:         "Fix bug X",
		FinalSolution: "some fix",
		Consensus:     0.9,
		Language:      "python",
	}

	score, err := bridge.EvaluateDebateResult(ctx, result, BenchmarkSWEBench)
	require.NoError(t, err)
	require.NotNil(t, score)

	assert.Equal(t, BenchmarkSWEBench, score.BenchmarkType)
	assert.Equal(t, 0.88, score.Details["mock_metric"])
	assert.Equal(t, "debate-2", score.Metadata["debate_id"])
}

func TestBenchmarkBridge_EvaluateDebateResult_FallbackToCustom(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	ctx := context.Background()

	// BenchmarkMMLU is not registered, should fall back to custom.
	result := &DebateResultForEval{
		ID:            "debate-3",
		Topic:         "Answer trivia",
		FinalSolution: "The answer is 42",
		Consensus:     0.7,
		Language:      "go",
	}

	score, err := bridge.EvaluateDebateResult(ctx, result, BenchmarkMMLU)
	require.NoError(t, err)
	require.NotNil(t, score)

	// Should use the custom evaluator (BenchmarkCustom type).
	assert.Equal(t, BenchmarkCustom, score.BenchmarkType)
}

func TestBenchmarkBridge_EvaluateDebateResult_NilResult(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	ctx := context.Background()

	score, err := bridge.EvaluateDebateResult(ctx, nil, BenchmarkCustom)
	assert.Error(t, err)
	assert.Nil(t, score)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestBenchmarkBridge_EvaluateDebateResult_EvaluatorError(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	ctx := context.Background()

	mock := &mockEvaluator{err: fmt.Errorf("evaluation engine failure")}
	bridge.RegisterEvaluator(BenchmarkHumanEval, mock)

	result := &DebateResultForEval{
		ID:            "debate-4",
		Topic:         "Generate code",
		FinalSolution: "code here",
		Language:      "go",
	}

	score, err := bridge.EvaluateDebateResult(ctx, result, BenchmarkHumanEval)
	assert.Error(t, err)
	assert.Nil(t, score)
	assert.Contains(t, err.Error(), "evaluation failed")
}

// =============================================================================
// CalculateCustomMetrics Tests
// =============================================================================

func TestBenchmarkBridge_CalculateCustomMetrics_GoCode(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()

	goCode := `package main

import (
	"fmt"
	"strings"
)

// Greet returns a greeting message for the given name.
// It validates input and returns an error for empty names.
func Greet(name string) (string, error) {
	if len(name) == 0 {
		return "", fmt.Errorf("name cannot be empty")
	}
	name = strings.TrimSpace(name)
	return fmt.Sprintf("Hello, %s!", name), nil
}

func TestGreet(t *testing.T) {
	t.Parallel()
	result, err := Greet("World")
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World!", result)
}
`
	metrics, err := bridge.CalculateCustomMetrics(goCode, "go")
	require.NoError(t, err)
	require.NotNil(t, metrics)

	assert.Contains(t, metrics, "correctness")
	assert.Contains(t, metrics, "maintainability")
	assert.Contains(t, metrics, "performance")
	assert.Contains(t, metrics, "security")
	assert.Contains(t, metrics, "test_coverage")

	// Correctness should be reasonably high (error handling + return + imports).
	assert.Greater(t, metrics["correctness"], 0.5)

	// Test coverage should be > 0 (has test function and assert).
	assert.Greater(t, metrics["test_coverage"], 0.0)

	// All metrics must be in [0, 1].
	for name, val := range metrics {
		assert.GreaterOrEqual(t, val, 0.0, "%s below 0", name)
		assert.LessOrEqual(t, val, 1.0, "%s above 1", name)
	}
}

func TestBenchmarkBridge_CalculateCustomMetrics_PythonCode(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()

	pythonCode := `# Module for greeting utilities
import os

def greet(name):
    """Greet a person by name."""
    if not name:
        raise ValueError("name cannot be empty")
    try:
        return f"Hello, {name}!"
    except Exception as e:
        raise RuntimeError(f"greeting failed: {e}")

def test_greet():
    assert greet("World") == "Hello, World!"
`

	metrics, err := bridge.CalculateCustomMetrics(pythonCode, "python")
	require.NoError(t, err)
	require.NotNil(t, metrics)

	assert.Greater(t, metrics["correctness"], 0.5)
	assert.Greater(t, metrics["test_coverage"], 0.0)
	assert.Greater(t, metrics["security"], 0.0)
}

func TestBenchmarkBridge_CalculateCustomMetrics_EmptyCode(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()

	metrics, err := bridge.CalculateCustomMetrics("", "go")
	require.NoError(t, err)
	require.NotNil(t, metrics)

	assert.Equal(t, 0.0, metrics["correctness"])
	assert.Equal(t, 0.0, metrics["maintainability"])
	assert.Equal(t, 0.0, metrics["performance"])
	assert.Equal(t, 0.0, metrics["security"])
	assert.Equal(t, 0.0, metrics["test_coverage"])
}

func TestBenchmarkBridge_CalculateCustomMetrics_SecurityIssues(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()

	insecureCode := `package main

import (
	"fmt"
	"os"
)

func query(userInput string) {
	sql := fmt.Sprintf("SELECT * FROM users WHERE name = '%s'", userInput)
	fmt.Println(sql)
}
`
	metrics, err := bridge.CalculateCustomMetrics(insecureCode, "go")
	require.NoError(t, err)

	// Security score should be lower due to SQL injection pattern.
	assert.Less(t, metrics["security"], 0.7)
}

// =============================================================================
// DebateBenchmarkSuite Tests
// =============================================================================

func TestDebateBenchmarkSuite_AddProblem(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	suite := NewDebateBenchmarkSuite(bridge)

	problem := &BenchmarkProblem{
		ID:          "prob-1",
		Type:        BenchmarkCustom,
		Description: "Implement sorting",
		Language:    "go",
		Difficulty:  "easy",
	}

	suite.AddProblem(problem)

	problems := suite.GetProblems()
	require.Len(t, problems, 1)
	assert.Equal(t, "prob-1", problems[0].ID)
}

func TestDebateBenchmarkSuite_AddProblem_Multiple(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	suite := NewDebateBenchmarkSuite(bridge)

	for i := 0; i < 5; i++ {
		suite.AddProblem(&BenchmarkProblem{
			ID:          fmt.Sprintf("prob-%d", i),
			Type:        BenchmarkCustom,
			Description: fmt.Sprintf("Problem %d", i),
			Language:    "go",
			Difficulty:  "medium",
		})
	}

	problems := suite.GetProblems()
	assert.Len(t, problems, 5)
}

func TestDebateBenchmarkSuite_GetProblems_ReturnsCopy(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	suite := NewDebateBenchmarkSuite(bridge)

	suite.AddProblem(&BenchmarkProblem{
		ID: "prob-1", Type: BenchmarkCustom,
	})

	p1 := suite.GetProblems()
	p2 := suite.GetProblems()

	// Mutating the first should not affect the second.
	p1[0] = nil
	require.NotNil(t, p2[0])
}

func TestDebateBenchmarkSuite_Run(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	suite := NewDebateBenchmarkSuite(bridge)

	suite.AddProblem(&BenchmarkProblem{
		ID:          "prob-1",
		Type:        BenchmarkCustom,
		Description: "Implement a function",
		Language:    "go",
		Difficulty:  "easy",
	})
	suite.AddProblem(&BenchmarkProblem{
		ID:          "prob-2",
		Type:        BenchmarkCustom,
		Description: "Implement another function",
		Language:    "go",
		Difficulty:  "medium",
	})

	solveFn := func(
		ctx context.Context,
		problem *BenchmarkProblem,
	) (*DebateResultForEval, error) {
		return &DebateResultForEval{
			ID:    problem.ID,
			Topic: problem.Description,
			FinalSolution: `package main

import "fmt"

// Solution function.
func Solution() string {
	if err := validate(); err != nil {
		return ""
	}
	return "ok"
}

func validate() error {
	return nil
}
`,
			Consensus: 0.8,
			Language:  problem.Language,
		}, nil
	}

	ctx := context.Background()
	scores, err := suite.Run(ctx, solveFn)
	require.NoError(t, err)
	require.Len(t, scores, 2)

	for _, score := range scores {
		assert.Equal(t, BenchmarkCustom, score.BenchmarkType)
		assert.Greater(t, score.Score, 0.0)
		assert.LessOrEqual(t, score.Score, 1.0)
	}
}

func TestDebateBenchmarkSuite_Run_EmptyProblems(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	suite := NewDebateBenchmarkSuite(bridge)

	solveFn := func(
		ctx context.Context,
		problem *BenchmarkProblem,
	) (*DebateResultForEval, error) {
		return nil, fmt.Errorf("should not be called")
	}

	scores, err := suite.Run(context.Background(), solveFn)
	require.NoError(t, err)
	assert.Empty(t, scores)
}

func TestDebateBenchmarkSuite_Run_SolveError(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	suite := NewDebateBenchmarkSuite(bridge)

	suite.AddProblem(&BenchmarkProblem{
		ID:   "prob-1",
		Type: BenchmarkCustom,
	})

	solveFn := func(
		ctx context.Context,
		problem *BenchmarkProblem,
	) (*DebateResultForEval, error) {
		return nil, fmt.Errorf("solver crashed")
	}

	scores, err := suite.Run(context.Background(), solveFn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "solve failed")
	assert.Empty(t, scores)
}

func TestDebateBenchmarkSuite_Run_NilResultSkipped(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	suite := NewDebateBenchmarkSuite(bridge)

	suite.AddProblem(&BenchmarkProblem{
		ID: "prob-1", Type: BenchmarkCustom, Language: "go",
	})
	suite.AddProblem(&BenchmarkProblem{
		ID: "prob-2", Type: BenchmarkCustom, Language: "go",
	})

	callCount := 0
	solveFn := func(
		ctx context.Context,
		problem *BenchmarkProblem,
	) (*DebateResultForEval, error) {
		callCount++
		if problem.ID == "prob-1" {
			return nil, nil // nil result should be skipped.
		}
		return &DebateResultForEval{
			ID:            problem.ID,
			FinalSolution: "func Hello() { return }",
			Language:      "go",
		}, nil
	}

	scores, err := suite.Run(context.Background(), solveFn)
	require.NoError(t, err)
	assert.Len(t, scores, 1) // Only prob-2 produced a score.
	assert.Equal(t, 2, callCount)
}

func TestDebateBenchmarkSuite_Run_ContextCancelled(t *testing.T) {
	t.Parallel()
	bridge := NewBenchmarkBridge()
	suite := NewDebateBenchmarkSuite(bridge)

	suite.AddProblem(&BenchmarkProblem{
		ID: "prob-1", Type: BenchmarkCustom, Language: "go",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	solveFn := func(
		ctx context.Context,
		problem *BenchmarkProblem,
	) (*DebateResultForEval, error) {
		return nil, fmt.Errorf("should not be called")
	}

	scores, err := suite.Run(ctx, solveFn)
	assert.Error(t, err)
	assert.Empty(t, scores)
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestClampScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    float64
		expected float64
	}{
		{0.5, 0.5},
		{0.0, 0.0},
		{1.0, 1.0},
		{-0.5, 0.0},
		{1.5, 1.0},
		{-100.0, 0.0},
		{100.0, 1.0},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("clamp(%f)", tc.input), func(t *testing.T) {
				t.Parallel()
			assert.Equal(t, tc.expected, clampScore(tc.input))
		})
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkBenchmarkBridge_CalculateCustomMetrics(b *testing.B) {
	bridge := NewBenchmarkBridge()
	solution := `package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

// Authenticator handles user authentication.
type Authenticator struct {
	users map[string]string
}

// NewAuthenticator creates a new Authenticator.
func NewAuthenticator() *Authenticator {
	return &Authenticator{
		users: make(map[string]string),
	}
}

// Register registers a new user with validation.
func (a *Authenticator) Register(username, password string) error {
	if strings.TrimSpace(username) == "" {
		return errors.New("username cannot be empty")
	}
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters, got %d", len(password))
	}
	if _, exists := a.users[username]; exists {
		return fmt.Errorf("user %s already exists", username)
	}
	a.users[username] = password
	return nil
}

// Authenticate validates credentials.
func (a *Authenticator) Authenticate(username, password string) (bool, error) {
	if username == "" || password == "" {
		return false, errors.New("username and password required")
	}
	stored, exists := a.users[username]
	if !exists {
		return false, nil
	}
	return stored == password, nil
}

// GenerateToken creates a random token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bridge.CalculateCustomMetrics(solution, "go")
	}
}

func BenchmarkBenchmarkBridge_EvaluateDebateResult(b *testing.B) {
	bridge := NewBenchmarkBridge()
	result := &DebateResultForEval{
		ID:            "debate-1",
		Topic:         "Implement authentication service",
		FinalSolution: "package auth\n\nfunc Authenticate(user, pass string) bool {\n\tif user == \"\" {\n\t\treturn false\n\t}\n\treturn true\n}\n",
		Consensus:     0.85,
		Language:      "go",
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bridge.EvaluateDebateResult(ctx, result, BenchmarkCustom)
	}
}

func BenchmarkCalculateCorrectness(b *testing.B) {
	solution := `package main

import "fmt"

func main() {
	result, err := process()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(result)
}

func process() (string, error) {
	return "done", nil
}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateCorrectness(solution, "go")
	}
}

func BenchmarkCalculateMaintainability(b *testing.B) {
	solution := `package main

import "fmt"

// Processor handles data processing.
type Processor struct {
	data []string
}

// NewProcessor creates a new Processor.
func NewProcessor() *Processor {
	return &Processor{
		data: make([]string, 0),
	}
}

// Process processes input data.
func (p *Processor) Process(input string) string {
	// Validate input
	if input == "" {
		return ""
	}
	// Transform and return
	result := fmt.Sprintf("processed: %s", input)
	p.data = append(p.data, result)
	return result
}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateMaintainability(solution, "go")
	}
}

func BenchmarkCalculateSecurity(b *testing.B) {
	solution := `package main

import (
	"crypto/tls"
	"database/sql"
	"fmt"
)

func queryUser(db *sql.DB, id string) error {
	if id == "" {
		return fmt.Errorf("id required")
	}
	row := db.QueryRow("SELECT * FROM users WHERE id = ?", id)
	_ = row
	return nil
}

func getTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateSecurity(solution, "go")
	}
}
