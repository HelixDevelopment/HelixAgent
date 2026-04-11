package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAgent implements Agent interface for testing
type MockAgent struct {
	responses map[string]interface{}
	shouldErr bool
}

func NewMockAgent() *MockAgent {
	return &MockAgent{
		responses: make(map[string]interface{}),
	}
}

func (m *MockAgent) WithResponse(taskID string, response interface{}) *MockAgent {
	m.responses[taskID] = response
	return m
}

func (m *MockAgent) WithError(shouldErr bool) *MockAgent {
	m.shouldErr = shouldErr
	return m
}

func (m *MockAgent) Execute(ctx context.Context, task Task) (interface{}, error) {
	if m.shouldErr {
		return nil, assert.AnError
	}

	if response, ok := m.responses[task.ID]; ok {
		return response, nil
	}

	// Default response
	return "default response", nil
}

func (m *MockAgent) Metrics() map[string]float64 {
	return map[string]float64{
		"tokens_used": 100,
		"latency_ms":  50,
	}
}

func TestNewEvaluator(t *testing.T) {
	logger := logrus.New()
	ev := NewEvaluator(logger)

	require.NotNil(t, ev)
	assert.NotNil(t, ev.logger)
	assert.Empty(t, ev.tasks)
	assert.Empty(t, ev.results)
}

func TestEvaluator_WithOutput(t *testing.T) {
	ev := NewEvaluator(nil)
	ev.WithOutput("/tmp/results.json")

	assert.Equal(t, "/tmp/results.json", ev.outputPath)
}

func TestEvaluator_AddTask(t *testing.T) {
	ev := NewEvaluator(nil)

	task := Task{
		ID:   "test-1",
		Name: "Test Task",
	}

	ev.AddTask(task)
	assert.Len(t, ev.tasks, 1)
	assert.Equal(t, "test-1", ev.tasks[0].ID)
}

func TestEvaluator_AddTask_DefaultTimeout(t *testing.T) {
	ev := NewEvaluator(nil)

	task := Task{
		ID: "test-1",
	}

	ev.AddTask(task)
	assert.Equal(t, 5*time.Minute, ev.tasks[0].Timeout)
}

func TestEvaluator_LoadTasksFromFile(t *testing.T) {
	tempDir := t.TempDir()
	tasksFile := filepath.Join(tempDir, "tasks.json")

	tasks := []Task{
		{ID: "task-1", Name: "Task 1", Type: "code"},
		{ID: "task-2", Name: "Task 2", Type: "test"},
	}

	data, _ := json.Marshal(tasks)
	require.NoError(t, os.WriteFile(tasksFile, data, 0644))

	ev := NewEvaluator(nil)
	err := ev.LoadTasksFromFile(tasksFile)

	require.NoError(t, err)
	assert.Len(t, ev.tasks, 2)
}

func TestEvaluator_LoadTasksFromFile_NotFound(t *testing.T) {
	ev := NewEvaluator(nil)
	err := ev.LoadTasksFromFile("/nonexistent/tasks.json")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read tasks file")
}

func TestEvaluator_SaveResults(t *testing.T) {
	tempDir := t.TempDir()
	resultsFile := filepath.Join(tempDir, "results.json")

	ev := NewEvaluator(nil)
	ev.results = []Result{
		{TaskID: "task-1", Success: true, Score: 1.0},
		{TaskID: "task-2", Success: false, Score: 0.5},
	}

	err := ev.SaveResults(resultsFile)
	require.NoError(t, err)

	// Verify file exists and content
	data, err := os.ReadFile(resultsFile)
	require.NoError(t, err)

	var results []Result
	err = json.Unmarshal(data, &results)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestEvaluator_Run(t *testing.T) {
	ev := NewEvaluator(nil)

	ev.AddTask(Task{
		ID:       "task-1",
		Type:     "code",
		Expected: "expected output",
	})

	agent := NewMockAgent().
		WithResponse("task-1", "expected output")

	ctx := context.Background()
	summary, err := ev.Run(ctx, agent)

	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, 1, summary.TotalTasks)
	assert.Equal(t, 1, summary.Passed)
	assert.Equal(t, 0, summary.Failed)
	assert.Equal(t, 1.0, summary.SuccessRate)
}

func TestEvaluator_Run_NoTasks(t *testing.T) {
	ev := NewEvaluator(nil)
	agent := NewMockAgent()
	ctx := context.Background()

	_, err := ev.Run(ctx, agent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tasks")
}

func TestEvaluator_Run_WithError(t *testing.T) {
	ev := NewEvaluator(nil)

	ev.AddTask(Task{
		ID:   "task-1",
		Type: "code",
	})

	agent := NewMockAgent().WithError(true)

	ctx := context.Background()
	summary, err := ev.Run(ctx, agent)

	require.NoError(t, err)
	assert.Equal(t, 0, summary.Passed)
	assert.Equal(t, 1, summary.Failed)
}

func TestEvaluator_RunParallel(t *testing.T) {
	ev := NewEvaluator(nil)

	// Add multiple tasks
	for i := 0; i < 5; i++ {
		ev.AddTask(Task{
			ID:       fmt.Sprintf("task-%d", i),
			Type:     "code",
			Expected: "output",
		})
	}

	agent := NewMockAgent()
	for i := 0; i < 5; i++ {
		agent.WithResponse(fmt.Sprintf("task-%d", i), "output")
	}

	ctx := context.Background()
	summary, err := ev.RunParallel(ctx, agent, 2)

	require.NoError(t, err)
	assert.Equal(t, 5, summary.TotalTasks)
}

func TestEvaluator_compareOutput(t *testing.T) {
	ev := NewEvaluator(nil)

	// Exact match
	assert.Equal(t, 1.0, ev.compareOutput("hello", "hello"))

	// Mismatch
	assert.Less(t, ev.compareOutput("hello", "world"), 1.0)

	// Nil handling
	assert.Equal(t, 1.0, ev.compareOutput(nil, nil))
	assert.Equal(t, 0.0, ev.compareOutput(nil, "value"))
	assert.Equal(t, 0.0, ev.compareOutput("value", nil))
}

func TestStringSimilarity(t *testing.T) {
	// Exact match
	assert.Equal(t, 1.0, stringSimilarity("hello world", "hello world"))

	// Empty strings
	assert.Equal(t, 1.0, stringSimilarity("", ""))
	assert.Equal(t, 0.0, stringSimilarity("", "hello"))

	// Partial match
	similarity := stringSimilarity("hello world", "hello there")
	assert.Greater(t, similarity, 0.0)
	assert.Less(t, similarity, 1.0)
}

func TestTokenize(t *testing.T) {
	words := tokenize("hello world")
	assert.True(t, words["hello"])
	assert.True(t, words["world"])
	assert.False(t, words["missing"])
}

func TestSummary_String(t *testing.T) {
	summary := &Summary{
		TotalTasks:      10,
		Passed:          8,
		Failed:          2,
		SuccessRate:     0.8,
		AverageScore:    0.85,
		AverageDuration: time.Second,
	}

	str := summary.String()
	assert.Contains(t, str, "Total Tasks: 10")
	assert.Contains(t, str, "Passed: 8")
	assert.Contains(t, str, "Success Rate: 80.00%")
}

func TestEvaluator_FilterTasks(t *testing.T) {
	ev := NewEvaluator(nil)

	ev.AddTask(Task{ID: "1", Tags: []string{"code", "go"}})
	ev.AddTask(Task{ID: "2", Tags: []string{"test", "go"}})
	ev.AddTask(Task{ID: "3", Tags: []string{"docs"}})

	filtered := ev.FilterTasks("go")
	assert.Len(t, filtered, 2)

	filtered = ev.FilterTasks("code")
	assert.Len(t, filtered, 1)

	filtered = ev.FilterTasks()
	assert.Len(t, filtered, 3)
}

func TestHasAnyTag(t *testing.T) {
	assert.True(t, hasAnyTag([]string{"a", "b", "c"}, []string{"b"}))
	assert.True(t, hasAnyTag([]string{"a", "b", "c"}, []string{"x", "b"}))
	assert.False(t, hasAnyTag([]string{"a", "b", "c"}, []string{"x", "y"}))
	assert.False(t, hasAnyTag([]string{}, []string{"a"}))
}

func TestEvaluator_GetTasksByType(t *testing.T) {
	ev := NewEvaluator(nil)

	ev.AddTask(Task{ID: "1", Type: "code"})
	ev.AddTask(Task{ID: "2", Type: "test"})
	ev.AddTask(Task{ID: "3", Type: "code"})

	codeTasks := ev.GetTasksByType("code")
	assert.Len(t, codeTasks, 2)

	testTasks := ev.GetTasksByType("test")
	assert.Len(t, testTasks, 1)
}

func TestEvaluator_GetTasksByDifficulty(t *testing.T) {
	ev := NewEvaluator(nil)

	ev.AddTask(Task{ID: "1", Difficulty: "easy"})
	ev.AddTask(Task{ID: "2", Difficulty: "hard"})
	ev.AddTask(Task{ID: "3", Difficulty: "easy"})

	easyTasks := ev.GetTasksByDifficulty("easy")
	assert.Len(t, easyTasks, 2)
}

func TestEvaluator_LoadTasksFromDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create task files
	tasks1 := []Task{{ID: "task-1", Type: "code"}}
	tasks2 := []Task{{ID: "task-2", Type: "test"}}

	data1, _ := json.Marshal(tasks1)
	data2, _ := json.Marshal(tasks2)

	os.WriteFile(filepath.Join(tempDir, "tasks1.json"), data1, 0644)
	os.WriteFile(filepath.Join(tempDir, "tasks2.json"), data2, 0644)
	os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("not json"), 0644)

	ev := NewEvaluator(nil)
	err := ev.LoadTasksFromDir(tempDir)

	require.NoError(t, err)
	assert.Len(t, ev.tasks, 2)
}

func TestEvaluator_Clear(t *testing.T) {
	ev := NewEvaluator(nil)

	ev.AddTask(Task{ID: "1"})
	ev.results = []Result{{TaskID: "1"}}

	ev.Clear()

	assert.Empty(t, ev.tasks)
	assert.Empty(t, ev.results)
}

func TestEvaluator_GetResults(t *testing.T) {
	ev := NewEvaluator(nil)
	ev.results = []Result{{TaskID: "1", Success: true}}

	results := ev.GetResults()
	assert.Len(t, results, 1)
}

func TestComparison(t *testing.T) {
	ev1 := NewEvaluator(nil)
	ev1.results = []Result{
		{Score: 1.0, Success: true},
		{Score: 0.5, Success: false},
	}

	ev2 := NewEvaluator(nil)
	ev2.results = []Result{
		{Score: 0.8, Success: true},
		{Score: 0.4, Success: false},
	}

	comparison := ev1.Compare(ev2)

	assert.NotNil(t, comparison.This)
	assert.NotNil(t, comparison.Other)
	assert.Greater(t, comparison.ScoreImprovement, 0.0)
}

// Need fmt import
func init() {}
