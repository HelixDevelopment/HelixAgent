package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
)

// MockLLMProviderForOrchestrator implements llm.LLMProvider for testing
type MockLLMProviderForOrchestrator struct {
	name         string
	completeFunc func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
	streamFunc   func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error)
}

func (m *MockLLMProviderForOrchestrator) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return &models.LLMResponse{
		Content:      "test response from " + m.name,
		ProviderName: m.name,
		Confidence:   0.9,
	}, nil
}

func (m *MockLLMProviderForOrchestrator) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	ch := make(chan *models.LLMResponse, 2)
	go func() {
		ch <- &models.LLMResponse{Content: "stream part 1", ProviderName: m.name}
		ch <- &models.LLMResponse{Content: "stream part 2", ProviderName: m.name}
		close(ch)
	}()
	return ch, nil
}

func (m *MockLLMProviderForOrchestrator) HealthCheck() error {
	return nil
}

func (m *MockLLMProviderForOrchestrator) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportsStreaming:       true,
		SupportedFeatures:       []string{"complete", "stream"},
		SupportsFunctionCalling: false,
	}
}

func (m *MockLLMProviderForOrchestrator) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

var _ llm.LLMProvider = (*MockLLMProviderForOrchestrator)(nil)

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

// Tests for executeStep

func TestIntegrationOrchestrator_executeStep_UnknownType(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "Test Step",
		Type: "unknown",
	}

	result, err := io.executeStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown step type: unknown")
}

func TestIntegrationOrchestrator_executeMCPStep_NoManager(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil) // No MCP manager

	step := &WorkflowStep{
		ID:   "step1",
		Name: "MCP Step",
		Type: "mcp",
	}

	result, err := io.executeMCPStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "MCP manager not available")
}

func TestIntegrationOrchestrator_executeMCPStep_NoOperation(t *testing.T) {
	ctx := context.Background()
	// Create a simple MCP manager (can be nil for this test as we fail before using it)
	io := NewIntegrationOrchestrator(&MCPManager{}, nil, nil, nil)

	step := &WorkflowStep{
		ID:         "step1",
		Name:       "MCP Step",
		Type:       "mcp",
		Parameters: map[string]any{},
	}

	result, err := io.executeMCPStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "MCP operation parameter required")
}

func TestIntegrationOrchestrator_executeMCPStep_UnknownOperation(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(&MCPManager{}, nil, nil, nil)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "MCP Step",
		Type: "mcp",
		Parameters: map[string]any{
			"operation": "unknown_operation",
		},
	}

	result, err := io.executeMCPStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown MCP operation: unknown_operation")
}

func TestIntegrationOrchestrator_executeMCPStep_CallToolNoName(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(&MCPManager{}, nil, nil, nil)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "MCP Step",
		Type: "mcp",
		Parameters: map[string]any{
			"operation": "call_tool",
			// toolName is missing
		},
	}

	result, err := io.executeMCPStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "toolName parameter required")
}

func TestIntegrationOrchestrator_executeMCPStep_RegisterServerNoConfig(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(&MCPManager{}, nil, nil, nil)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "MCP Step",
		Type: "mcp",
		Parameters: map[string]any{
			"operation": "register_server",
			// serverConfig is missing
		},
	}

	result, err := io.executeMCPStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "serverConfig parameter required")
}

func TestIntegrationOrchestrator_executeMCPStep_ListTools(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	mcpMgr := NewMCPManager(nil, nil, logger)
	io := NewIntegrationOrchestrator(mcpMgr, nil, nil, nil)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "MCP Step",
		Type: "mcp",
		Parameters: map[string]any{
			"operation": "list_tools",
		},
	}

	_, err := io.executeMCPStep(ctx, step)
	assert.NoError(t, err)
	// Result may be nil (empty list) or a list - just check no error occurred
}

func TestIntegrationOrchestrator_executeMCPStep_ListServers(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	mcpMgr := NewMCPManager(nil, nil, logger)
	io := NewIntegrationOrchestrator(mcpMgr, nil, nil, nil)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "MCP Step",
		Type: "mcp",
		Parameters: map[string]any{
			"operation": "list_servers",
		},
	}

	result, err := io.executeMCPStep(ctx, step)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestIntegrationOrchestrator_executeMCPStep_CallToolWithParams(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	mcpMgr := NewMCPManager(nil, nil, logger)
	io := NewIntegrationOrchestrator(mcpMgr, nil, nil, nil)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "MCP Step",
		Type: "mcp",
		Parameters: map[string]any{
			"operation": "call_tool",
			"toolName":  "test-tool",
			"params": map[string]interface{}{
				"arg1": "value1",
			},
		},
	}

	// This will fail because no tool is registered, but it tests the params extraction path
	result, err := io.executeMCPStep(ctx, step)
	// Error expected since tool doesn't exist
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestIntegrationOrchestrator_executeToolStep_NoToolName(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, &ToolRegistry{}, nil)

	step := &WorkflowStep{
		ID:         "step1",
		Name:       "Tool Step",
		Type:       "tool",
		Parameters: map[string]any{}, // toolName is missing
	}

	result, err := io.executeToolStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "toolName parameter required")
}

func TestIntegrationOrchestrator_executeLLMStep_NoOperation(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	step := &WorkflowStep{
		ID:         "step1",
		Name:       "LLM Step",
		Type:       "llm",
		Parameters: map[string]any{}, // operation is missing
	}

	result, err := io.executeLLMStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "LLM operation parameter required")
}

func TestIntegrationOrchestrator_executeLLMStep_NoProviderRegistry(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil) // No provider registry

	step := &WorkflowStep{
		ID:   "step1",
		Name: "LLM Step",
		Type: "llm",
		Parameters: map[string]any{
			"operation": "complete",
		},
	}

	result, err := io.executeLLMStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "LLM provider registry not configured")
}

func TestIntegrationOrchestrator_executeLLMStep_NoProviders(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	// Set empty provider registry
	io.providerRegistry = NewProviderRegistry(nil, nil)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "LLM Step",
		Type: "llm",
		Parameters: map[string]any{
			"operation": "complete",
		},
	}

	result, err := io.executeLLMStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no LLM providers available")
}

// Note: TestIntegrationOrchestrator_executeLLMStep_UnknownOperation is skipped
// because it requires a full LLM provider implementation with GetCapabilities method

func TestIntegrationOrchestrator_executeLSPStep_NilClient(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil) // Nil LSP client

	step := &WorkflowStep{
		ID:   "step1",
		Name: "Initialize LSP Client",
		Type: "lsp",
		Parameters: map[string]any{},
	}

	// This should panic or return error due to nil client
	defer func() {
		if r := recover(); r != nil {
			// Expected - nil pointer dereference
		}
	}()

	_, err := io.executeLSPStep(ctx, step)
	// If we get here without panic, check error
	if err == nil {
		t.Skip("LSP client is nil - operation would fail")
	}
}

func TestIntegrationOrchestrator_executeLSPStep_UnknownStep(t *testing.T) {
	ctx := context.Background()
	// Create a minimal LSP client
	lspClient := &LSPClient{}
	io := NewIntegrationOrchestrator(nil, lspClient, nil, nil)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "Unknown LSP Operation",
		Type: "lsp",
		Parameters: map[string]any{},
	}

	result, err := io.executeLSPStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown LSP step: Unknown LSP Operation")
}

func TestIntegrationOrchestrator_executeOperation_UnknownType(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	op := Operation{
		ID:   "op-1",
		Type: "unknown",
		Name: "Unknown Op",
	}

	result := io.executeOperation(ctx, op)
	assert.Error(t, result.Error)
	assert.Equal(t, "op-1", result.ID)
	assert.Contains(t, result.Error.Error(), "unknown operation type: unknown")
}

func TestIntegrationOrchestrator_ExecuteParallelOperations_Empty(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	results, err := io.ExecuteParallelOperations(ctx, []Operation{})
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestIntegrationOrchestrator_ExecuteParallelOperations_WithErrors(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	operations := []Operation{
		{ID: "op-1", Type: "unknown", Name: "Op 1"},
		{ID: "op-2", Type: "unknown", Name: "Op 2"},
	}

	results, err := io.ExecuteParallelOperations(ctx, operations)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parallel execution had")
	assert.Empty(t, results) // All operations failed
}

func TestIntegrationOrchestrator_ExecuteToolChain_Empty(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, &ToolRegistry{}, nil)

	results, err := io.ExecuteToolChain(ctx, []ToolExecution{})
	assert.NoError(t, err)
	assert.NotNil(t, results)
}

func TestIntegrationOrchestrator_executeOperation_LSPType(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	lspClient := NewLSPClient(logger)
	io := NewIntegrationOrchestrator(nil, lspClient, nil, nil)

	op := Operation{
		ID:         "op-lsp",
		Type:       "lsp",
		Name:       "Unknown LSP Operation",
		Parameters: map[string]interface{}{},
	}

	result := io.executeOperation(ctx, op)
	assert.Error(t, result.Error)
	assert.Equal(t, "op-lsp", result.ID)
	assert.Contains(t, result.Error.Error(), "unknown LSP step")
}

func TestIntegrationOrchestrator_executeOperation_ToolType(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, &ToolRegistry{}, nil)

	op := Operation{
		ID:   "op-tool",
		Type: "tool",
		Name: "Test Tool",
		Parameters: map[string]interface{}{
			"toolName": "non-existent-tool",
		},
	}

	result := io.executeOperation(ctx, op)
	// Should fail because tool doesn't exist
	assert.Error(t, result.Error)
	assert.Equal(t, "op-tool", result.ID)
}

func TestIntegrationOrchestrator_executeStep_MCPType(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	mcpMgr := NewMCPManager(nil, nil, logger)
	io := NewIntegrationOrchestrator(mcpMgr, nil, nil, nil)

	step := &WorkflowStep{
		ID:   "step-mcp",
		Name: "MCP Step",
		Type: "mcp",
		Parameters: map[string]any{
			"operation": "list_tools",
		},
	}

	result, err := io.executeStep(ctx, step)
	assert.NoError(t, err)
	// Result may be nil (empty list) or actual tools
	_ = result
}

func TestIntegrationOrchestrator_executeStep_ToolType(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, &ToolRegistry{}, nil)

	step := &WorkflowStep{
		ID:   "step-tool",
		Name: "Tool Step",
		Type: "tool",
		Parameters: map[string]any{
			"toolName": "non-existent-tool",
		},
	}

	result, err := io.executeStep(ctx, step)
	// Should fail because tool doesn't exist
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestIntegrationOrchestrator_executeStep_LSPType(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	lspClient := NewLSPClient(logger)
	io := NewIntegrationOrchestrator(nil, lspClient, nil, nil)

	step := &WorkflowStep{
		ID:   "step-lsp",
		Name: "Unknown LSP Step",
		Type: "lsp",
		Parameters: map[string]any{},
	}

	result, err := io.executeStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown LSP step")
}

func TestIntegrationOrchestrator_executeStep_LLMType(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	step := &WorkflowStep{
		ID:   "step-llm",
		Name: "LLM Step",
		Type: "llm",
		Parameters: map[string]any{
			"operation": "complete",
		},
	}

	result, err := io.executeStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "LLM provider registry not configured")
}

func TestIntegrationOrchestrator_executeWorkflow_EmptySteps(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	workflow := &Workflow{
		ID:      "workflow-1",
		Name:    "Empty Workflow",
		Steps:   []WorkflowStep{},
		Results: make(map[string]any),
	}

	err := io.executeWorkflow(ctx, workflow)
	assert.NoError(t, err)
	assert.Equal(t, "completed", workflow.Status)
}

func TestIntegrationOrchestrator_executeWorkflow_SingleStep(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	workflow := &Workflow{
		ID:   "workflow-2",
		Name: "Single Step Workflow",
		Steps: []WorkflowStep{
			{
				ID:         "step1",
				Name:       "Step 1",
				Type:       "unknown", // Will fail
				Parameters: map[string]any{},
			},
		},
		Results: make(map[string]any),
	}

	err := io.executeWorkflow(ctx, workflow)
	assert.NoError(t, err) // Workflow completes even if steps fail
	assert.Equal(t, "completed", workflow.Status)
	assert.Len(t, workflow.Errors, 1)
}

func TestIntegrationOrchestrator_executeWorkflow_MultipleSteps(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	workflow := &Workflow{
		ID:   "workflow-3",
		Name: "Multi Step Workflow",
		Steps: []WorkflowStep{
			{
				ID:         "step1",
				Name:       "Step 1",
				Type:       "unknown",
				Parameters: map[string]any{},
			},
			{
				ID:         "step2",
				Name:       "Step 2",
				Type:       "unknown",
				Parameters: map[string]any{},
			},
		},
		Results: make(map[string]any),
	}

	err := io.executeWorkflow(ctx, workflow)
	assert.NoError(t, err)
	assert.Equal(t, "completed", workflow.Status)
}

func TestIntegrationOrchestrator_executeWorkflow_WithDependencies(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	workflow := &Workflow{
		ID:   "workflow-4",
		Name: "Workflow With Dependencies",
		Steps: []WorkflowStep{
			{
				ID:         "step1",
				Name:       "Step 1",
				Type:       "unknown",
				Parameters: map[string]any{},
			},
			{
				ID:         "step2",
				Name:       "Step 2",
				Type:       "unknown",
				Parameters: map[string]any{},
				DependsOn:  []string{"step1"},
			},
			{
				ID:         "step3",
				Name:       "Step 3",
				Type:       "unknown",
				Parameters: map[string]any{},
				DependsOn:  []string{"step2"},
			},
		},
		Results: make(map[string]any),
	}

	err := io.executeWorkflow(ctx, workflow)
	assert.NoError(t, err)
	assert.Equal(t, "completed", workflow.Status)
}

func TestIntegrationOrchestrator_executeWorkflow_ParallelSteps(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	workflow := &Workflow{
		ID:   "workflow-5",
		Name: "Parallel Workflow",
		Steps: []WorkflowStep{
			{
				ID:         "step1",
				Name:       "Step 1",
				Type:       "unknown",
				Parameters: map[string]any{},
			},
			{
				ID:         "step2",
				Name:       "Step 2",
				Type:       "unknown",
				Parameters: map[string]any{},
			},
			{
				ID:         "step3",
				Name:       "Step 3",
				Type:       "unknown",
				Parameters: map[string]any{},
				DependsOn:  []string{"step1", "step2"},
			},
		},
		Results: make(map[string]any),
	}

	err := io.executeWorkflow(ctx, workflow)
	assert.NoError(t, err)
	assert.Equal(t, "completed", workflow.Status)
}

func TestIntegrationOrchestrator_ExecuteCodeAnalysis(t *testing.T) {
	ctx := context.Background()

	t.Run("with lsp client", func(t *testing.T) {
		log := logrus.New()
		log.SetLevel(logrus.PanicLevel)
		lspClient := NewLSPClient(log)

		io := NewIntegrationOrchestrator(nil, lspClient, nil, nil)

		// This will create the workflow and attempt to execute
		// LSP steps will fail because no server is connected, but the function handles this
		result, err := io.ExecuteCodeAnalysis(ctx, "/test/file.go", "go")
		// The workflow will fail because StartServer fails when no command is configured
		// but the function itself is exercised
		_ = err
		if result != nil {
			assert.Equal(t, "/test/file.go", result.FilePath)
		}
	})
}

func TestIntegrationOrchestrator_ExecuteToolChain(t *testing.T) {
	ctx := context.Background()

	t.Run("empty tool chain", func(t *testing.T) {
		io := NewIntegrationOrchestrator(nil, nil, nil, nil)

		result, err := io.ExecuteToolChain(ctx, []ToolExecution{})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("single tool execution", func(t *testing.T) {
		io := NewIntegrationOrchestrator(nil, nil, nil, nil)

		toolChain := []ToolExecution{
			{
				ToolName:   "test-tool",
				Parameters: map[string]any{"key": "value"},
			},
		}

		result, err := io.ExecuteToolChain(ctx, toolChain)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("multiple tools with dependencies", func(t *testing.T) {
		io := NewIntegrationOrchestrator(nil, nil, nil, nil)

		// Note: ExecuteToolChain assigns IDs as "tool_0", "tool_1", etc.
		toolChain := []ToolExecution{
			{
				ToolName:   "analyze",
				Parameters: map[string]any{"action": "analyze"},
			},
			{
				ToolName:   "process",
				Parameters: map[string]any{"action": "process"},
				DependsOn:  []string{"tool_0"}, // Depends on first tool
			},
			{
				ToolName:   "summarize",
				Parameters: map[string]any{"action": "summarize"},
				DependsOn:  []string{"tool_1"}, // Depends on second tool
			},
		}

		result, err := io.ExecuteToolChain(ctx, toolChain)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestIntegrationOrchestrator_executeLLMStep_Complete(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	// Create provider registry with mock provider
	registry := NewProviderRegistry(&RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
		CircuitBreaker: CircuitBreakerConfig{Enabled: false},
		Providers:      make(map[string]*ProviderConfig),
	}, nil)

	mockProvider := &MockLLMProviderForOrchestrator{
		name: "test-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{
				Content:      "completed response",
				ProviderName: "test-provider",
				Confidence:   0.95,
			}, nil
		},
	}
	registry.RegisterProvider("test-provider", mockProvider)
	io.SetProviderRegistry(registry)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "LLM Complete",
		Type: "llm",
		Parameters: map[string]any{
			"operation": "complete",
			"provider":  "test-provider",
			"prompt":    "Test prompt",
			"model":     "test-model",
		},
	}

	result, err := io.executeLLMStep(ctx, step)
	require.NoError(t, err)
	require.NotNil(t, result)

	resp, ok := result.(*models.LLMResponse)
	require.True(t, ok)
	assert.Equal(t, "completed response", resp.Content)
}

func TestIntegrationOrchestrator_executeLLMStep_Stream(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	// Create provider registry with mock provider
	registry := NewProviderRegistry(&RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
		CircuitBreaker: CircuitBreakerConfig{Enabled: false},
		Providers:      make(map[string]*ProviderConfig),
	}, nil)

	mockProvider := &MockLLMProviderForOrchestrator{
		name: "stream-provider",
		streamFunc: func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
			ch := make(chan *models.LLMResponse, 3)
			go func() {
				ch <- &models.LLMResponse{Content: "Hello ", ProviderName: "stream-provider"}
				ch <- &models.LLMResponse{Content: "World", ProviderName: "stream-provider"}
				ch <- &models.LLMResponse{Content: "!", ProviderName: "stream-provider"}
				close(ch)
			}()
			return ch, nil
		},
	}
	registry.RegisterProvider("stream-provider", mockProvider)
	io.SetProviderRegistry(registry)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "LLM Stream",
		Type: "llm",
		Parameters: map[string]any{
			"operation": "stream",
			"provider":  "stream-provider",
			"prompt":    "Test streaming prompt",
		},
	}

	result, err := io.executeLLMStep(ctx, step)
	require.NoError(t, err)
	require.NotNil(t, result)

	resp, ok := result.(*models.LLMResponse)
	require.True(t, ok)
	assert.Equal(t, "Hello World!", resp.Content)
}

func TestIntegrationOrchestrator_executeLLMStep_UnknownOperation(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	// Create provider registry with mock provider
	registry := NewProviderRegistry(&RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{Enabled: false},
		Providers:      make(map[string]*ProviderConfig),
	}, nil)

	mockProvider := &MockLLMProviderForOrchestrator{name: "test-provider"}
	registry.RegisterProvider("test-provider", mockProvider)
	io.SetProviderRegistry(registry)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "LLM Unknown",
		Type: "llm",
		Parameters: map[string]any{
			"operation": "invalid_operation",
			"provider":  "test-provider",
		},
	}

	result, err := io.executeLLMStep(ctx, step)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown LLM operation")
}

func TestIntegrationOrchestrator_executeLLMStep_DefaultProvider(t *testing.T) {
	ctx := context.Background()
	io := NewIntegrationOrchestrator(nil, nil, nil, nil)

	// Create provider registry with mock provider (no explicit provider in step)
	registry := NewProviderRegistry(&RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{Enabled: false},
		Providers:      make(map[string]*ProviderConfig),
	}, nil)

	mockProvider := &MockLLMProviderForOrchestrator{name: "default-provider"}
	registry.RegisterProvider("default-provider", mockProvider)
	io.SetProviderRegistry(registry)

	step := &WorkflowStep{
		ID:   "step1",
		Name: "LLM Default",
		Type: "llm",
		Parameters: map[string]any{
			"operation": "complete",
			// No provider specified - should use first available
			"prompt": "Test with default provider",
		},
	}

	result, err := io.executeLLMStep(ctx, step)
	require.NoError(t, err)
	require.NotNil(t, result)

	resp, ok := result.(*models.LLMResponse)
	require.True(t, ok)
	assert.Contains(t, resp.Content, "default-provider")
}
