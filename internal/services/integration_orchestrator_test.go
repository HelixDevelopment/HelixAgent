package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIntegrationOrchestrator(t *testing.T) {
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)
	require.NotNil(t, io)
	assert.NotNil(t, io.workflows)
	assert.Nil(t, io.mcpManager)
	assert.Nil(t, io.lspClient)
	assert.Nil(t, io.toolRegistry)
	assert.Nil(t, io.contextManager)
}

func TestIntegrationOrchestrator_SetProviderRegistry(t *testing.T) {
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)
	assert.Nil(t, io.providerRegistry)

	// Create a mock provider registry
	pr := &ProviderRegistry{}
	io.SetProviderRegistry(pr)
	assert.Equal(t, pr, io.providerRegistry)
}

func TestIntegrationOrchestrator_buildDependencyGraph(t *testing.T) {
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	t.Run("empty steps", func(t *testing.T) {
		graph := io.buildDependencyGraph([]WorkflowStep{})
		assert.Empty(t, graph)
	})

	t.Run("single step no dependencies", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "Step 1", Type: "tool"},
		}
		graph := io.buildDependencyGraph(steps)
		assert.Len(t, graph, 1)
		assert.Empty(t, graph["step1"])
	})

	t.Run("linear dependencies", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "Step 1", Type: "tool"},
			{ID: "step2", Name: "Step 2", Type: "tool", DependsOn: []string{"step1"}},
			{ID: "step3", Name: "Step 3", Type: "tool", DependsOn: []string{"step2"}},
		}
		graph := io.buildDependencyGraph(steps)
		assert.Len(t, graph, 3)
		assert.Empty(t, graph["step1"])
		assert.Equal(t, []string{"step1"}, graph["step2"])
		assert.Equal(t, []string{"step2"}, graph["step3"])
	})

	t.Run("multiple dependencies", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "Step 1", Type: "tool"},
			{ID: "step2", Name: "Step 2", Type: "tool"},
			{ID: "step3", Name: "Step 3", Type: "tool", DependsOn: []string{"step1", "step2"}},
		}
		graph := io.buildDependencyGraph(steps)
		assert.Len(t, graph, 3)
		assert.Empty(t, graph["step1"])
		assert.Empty(t, graph["step2"])
		assert.Len(t, graph["step3"], 2)
		assert.Contains(t, graph["step3"], "step1")
		assert.Contains(t, graph["step3"], "step2")
	})
}

func TestIntegrationOrchestrator_hasCycles(t *testing.T) {
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	t.Run("no cycles - empty graph", func(t *testing.T) {
		graph := map[string][]string{}
		assert.False(t, io.hasCycles(graph))
	})

	t.Run("no cycles - linear", func(t *testing.T) {
		graph := map[string][]string{
			"step1": {},
			"step2": {"step1"},
			"step3": {"step2"},
		}
		assert.False(t, io.hasCycles(graph))
	})

	t.Run("no cycles - diamond", func(t *testing.T) {
		graph := map[string][]string{
			"step1": {},
			"step2": {"step1"},
			"step3": {"step1"},
			"step4": {"step2", "step3"},
		}
		assert.False(t, io.hasCycles(graph))
	})

	t.Run("has cycle - self loop", func(t *testing.T) {
		graph := map[string][]string{
			"step1": {"step1"},
		}
		assert.True(t, io.hasCycles(graph))
	})

	t.Run("has cycle - two nodes", func(t *testing.T) {
		graph := map[string][]string{
			"step1": {"step2"},
			"step2": {"step1"},
		}
		assert.True(t, io.hasCycles(graph))
	})

	t.Run("has cycle - three nodes", func(t *testing.T) {
		graph := map[string][]string{
			"step1": {"step2"},
			"step2": {"step3"},
			"step3": {"step1"},
		}
		assert.True(t, io.hasCycles(graph))
	})
}

func TestIntegrationOrchestrator_findExecutableSteps(t *testing.T) {
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	t.Run("all steps executable - no dependencies", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "Step 1"},
			{ID: "step2", Name: "Step 2"},
		}
		graph := map[string][]string{
			"step1": {},
			"step2": {},
		}
		completed := map[string]bool{}
		running := map[string]bool{}

		executable := io.findExecutableSteps(steps, graph, completed, running)
		assert.Len(t, executable, 2)
	})

	t.Run("one step executable - dependencies not met", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "Step 1"},
			{ID: "step2", Name: "Step 2", DependsOn: []string{"step1"}},
		}
		graph := map[string][]string{
			"step1": {},
			"step2": {"step1"},
		}
		completed := map[string]bool{}
		running := map[string]bool{}

		executable := io.findExecutableSteps(steps, graph, completed, running)
		assert.Len(t, executable, 1)
		assert.Equal(t, "step1", executable[0].ID)
	})

	t.Run("second step executable - dependency completed", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "Step 1"},
			{ID: "step2", Name: "Step 2", DependsOn: []string{"step1"}},
		}
		graph := map[string][]string{
			"step1": {},
			"step2": {"step1"},
		}
		completed := map[string]bool{"step1": true}
		running := map[string]bool{}

		executable := io.findExecutableSteps(steps, graph, completed, running)
		assert.Len(t, executable, 1)
		assert.Equal(t, "step2", executable[0].ID)
	})

	t.Run("no executable - all completed", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "Step 1"},
			{ID: "step2", Name: "Step 2"},
		}
		graph := map[string][]string{
			"step1": {},
			"step2": {},
		}
		completed := map[string]bool{"step1": true, "step2": true}
		running := map[string]bool{}

		executable := io.findExecutableSteps(steps, graph, completed, running)
		assert.Empty(t, executable)
	})

	t.Run("skip running steps", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "Step 1"},
			{ID: "step2", Name: "Step 2"},
		}
		graph := map[string][]string{
			"step1": {},
			"step2": {},
		}
		completed := map[string]bool{}
		running := map[string]bool{"step1": true}

		executable := io.findExecutableSteps(steps, graph, completed, running)
		assert.Len(t, executable, 1)
		assert.Equal(t, "step2", executable[0].ID)
	})

	t.Run("multiple dependencies must all be complete", func(t *testing.T) {
		steps := []WorkflowStep{
			{ID: "step1", Name: "Step 1"},
			{ID: "step2", Name: "Step 2"},
			{ID: "step3", Name: "Step 3", DependsOn: []string{"step1", "step2"}},
		}
		graph := map[string][]string{
			"step1": {},
			"step2": {},
			"step3": {"step1", "step2"},
		}
		completed := map[string]bool{"step1": true}
		running := map[string]bool{}

		executable := io.findExecutableSteps(steps, graph, completed, running)
		assert.Len(t, executable, 1)
		assert.Equal(t, "step2", executable[0].ID)

		// Complete step2, now step3 should be executable
		completed["step2"] = true
		executable = io.findExecutableSteps(steps, graph, completed, running)
		assert.Len(t, executable, 1)
		assert.Equal(t, "step3", executable[0].ID)
	})
}

func TestWorkflowStep_Fields(t *testing.T) {
	step := WorkflowStep{
		ID:         "test-step",
		Name:       "Test Step",
		Type:       "tool",
		Parameters: map[string]any{"key": "value"},
		DependsOn:  []string{"step1", "step2"},
		Status:     "pending",
		MaxRetries: 3,
	}

	assert.Equal(t, "test-step", step.ID)
	assert.Equal(t, "Test Step", step.Name)
	assert.Equal(t, "tool", step.Type)
	assert.Equal(t, "value", step.Parameters["key"])
	assert.Equal(t, []string{"step1", "step2"}, step.DependsOn)
	assert.Equal(t, "pending", step.Status)
	assert.Equal(t, 3, step.MaxRetries)
	assert.Nil(t, step.StartTime)
	assert.Nil(t, step.EndTime)
	assert.Equal(t, 0, step.RetryCount)
}

func TestWorkflow_Fields(t *testing.T) {
	workflow := &Workflow{
		ID:          "test-workflow",
		Name:        "Test Workflow",
		Description: "A test workflow",
		Status:      "pending",
		Results:     make(map[string]any),
		Errors:      []error{},
	}

	assert.Equal(t, "test-workflow", workflow.ID)
	assert.Equal(t, "Test Workflow", workflow.Name)
	assert.Equal(t, "A test workflow", workflow.Description)
	assert.Equal(t, "pending", workflow.Status)
	assert.NotNil(t, workflow.Results)
	assert.Empty(t, workflow.Errors)
}

func TestToolExecution_Fields(t *testing.T) {
	te := ToolExecution{
		ToolName:   "test-tool",
		Parameters: map[string]any{"param1": "value1"},
		DependsOn:  []string{"dep1"},
		MaxRetries: 2,
	}

	assert.Equal(t, "test-tool", te.ToolName)
	assert.Equal(t, "value1", te.Parameters["param1"])
	assert.Equal(t, []string{"dep1"}, te.DependsOn)
	assert.Equal(t, 2, te.MaxRetries)
}

func TestOperation_Fields(t *testing.T) {
	op := Operation{
		ID:         "op-1",
		Type:       "lsp",
		Name:       "Initialize",
		Parameters: map[string]interface{}{"filePath": "/path/to/file"},
	}

	assert.Equal(t, "op-1", op.ID)
	assert.Equal(t, "lsp", op.Type)
	assert.Equal(t, "Initialize", op.Name)
	assert.Equal(t, "/path/to/file", op.Parameters["filePath"])
}

func TestOperationResult_Fields(t *testing.T) {
	result := OperationResult{
		ID:   "op-1",
		Data: map[string]string{"key": "value"},
	}

	assert.Equal(t, "op-1", result.ID)
	assert.NotNil(t, result.Data)
	assert.Nil(t, result.Error)
}

func TestIntegrationOrchestrator_buildLLMRequest(t *testing.T) {
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	t.Run("basic request", func(t *testing.T) {
		step := &WorkflowStep{
			ID: "step1",
			Parameters: map[string]any{
				"prompt": "Test prompt",
			},
		}
		req, err := io.buildLLMRequest(step)
		require.NoError(t, err)
		assert.Equal(t, "Test prompt", req.Prompt)
		assert.Equal(t, "default", req.ModelParams.Model)
	})

	t.Run("with model specified", func(t *testing.T) {
		step := &WorkflowStep{
			ID: "step1",
			Parameters: map[string]any{
				"prompt": "Test prompt",
				"model":  "gpt-4",
			},
		}
		req, err := io.buildLLMRequest(step)
		require.NoError(t, err)
		assert.Equal(t, "gpt-4", req.ModelParams.Model)
	})

	t.Run("with temperature", func(t *testing.T) {
		step := &WorkflowStep{
			ID: "step1",
			Parameters: map[string]any{
				"prompt":      "Test prompt",
				"temperature": 0.7,
			},
		}
		req, err := io.buildLLMRequest(step)
		require.NoError(t, err)
		assert.Equal(t, 0.7, req.ModelParams.Temperature)
	})

	t.Run("with max_tokens as int", func(t *testing.T) {
		step := &WorkflowStep{
			ID: "step1",
			Parameters: map[string]any{
				"prompt":     "Test prompt",
				"max_tokens": 1000,
			},
		}
		req, err := io.buildLLMRequest(step)
		require.NoError(t, err)
		assert.Equal(t, 1000, req.ModelParams.MaxTokens)
	})

	t.Run("with max_tokens as float64", func(t *testing.T) {
		step := &WorkflowStep{
			ID: "step1",
			Parameters: map[string]any{
				"prompt":     "Test prompt",
				"max_tokens": 1500.0, // JSON numbers come as float64
			},
		}
		req, err := io.buildLLMRequest(step)
		require.NoError(t, err)
		assert.Equal(t, 1500, req.ModelParams.MaxTokens)
	})

	t.Run("with messages", func(t *testing.T) {
		step := &WorkflowStep{
			ID: "step1",
			Parameters: map[string]any{
				"prompt": "Test prompt",
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
					map[string]interface{}{
						"role":    "assistant",
						"content": "Hi there!",
					},
				},
			},
		}
		req, err := io.buildLLMRequest(step)
		require.NoError(t, err)
		require.Len(t, req.Messages, 2)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "Hello", req.Messages[0].Content)
		assert.Equal(t, "assistant", req.Messages[1].Role)
		assert.Equal(t, "Hi there!", req.Messages[1].Content)
	})
}

func TestIntegrationOrchestrator_hasCyclesUtil(t *testing.T) {
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	t.Run("simple path no cycle", func(t *testing.T) {
		graph := map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {},
		}
		visited := map[string]bool{}
		recStack := map[string]bool{}

		result := io.hasCyclesUtil("a", graph, visited, recStack)
		assert.False(t, result)
	})

	t.Run("back edge creates cycle", func(t *testing.T) {
		graph := map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"a"}, // Back edge to a
		}
		visited := map[string]bool{}
		recStack := map[string]bool{}

		result := io.hasCyclesUtil("a", graph, visited, recStack)
		assert.True(t, result)
	})
}
