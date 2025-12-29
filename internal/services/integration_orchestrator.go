package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// IntegrationOrchestrator coordinates MCP, LSP, and tool execution
type IntegrationOrchestrator struct {
	mcpManager     *MCPManager
	lspClient      *LSPClient
	toolRegistry   *ToolRegistry
	contextManager *ContextManager
	workflows      map[string]*Workflow
	mu             sync.RWMutex
}

// Workflow represents a sequence of integrated operations
type Workflow struct {
	ID          string
	Name        string
	Description string
	Steps       []WorkflowStep
	Status      string // "pending", "running", "completed", "failed"
	Results     map[string]any
	Errors      []error
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	ID         string
	Name       string
	Type       string // "lsp", "mcp", "tool", "llm"
	Parameters map[string]any
	DependsOn  []string // IDs of steps this depends on
	Status     string   // "pending", "running", "completed", "failed"
	Result     any
	Error      error
	StartTime  *time.Time
	EndTime    *time.Time
	RetryCount int
	MaxRetries int
}

// NewIntegrationOrchestrator creates a new orchestrator
func NewIntegrationOrchestrator(mcpManager *MCPManager, lspClient *LSPClient, toolRegistry *ToolRegistry, contextManager *ContextManager) *IntegrationOrchestrator {
	return &IntegrationOrchestrator{
		mcpManager:     mcpManager,
		lspClient:      lspClient,
		toolRegistry:   toolRegistry,
		contextManager: contextManager,
		workflows:      make(map[string]*Workflow),
	}
}

// ExecuteCodeAnalysis performs comprehensive code analysis
func (io *IntegrationOrchestrator) ExecuteCodeAnalysis(ctx context.Context, filePath string, languageID string) (*models.CodeIntelligence, error) {
	log.Printf("Starting code analysis for %s with language %s", filePath, languageID)
	workflow := &Workflow{
		ID:          fmt.Sprintf("analysis-%d", time.Now().Unix()),
		Name:        "Code Analysis",
		Description: "Comprehensive code analysis using LSP and tools",
		Steps: []WorkflowStep{
			{
				ID:         "lsp_init",
				Name:       "Initialize LSP Client",
				Type:       "lsp",
				Parameters: map[string]any{"languageID": languageID, "filePath": filePath},
			},
			{
				ID:         "lsp_intelligence",
				Name:       "Get Code Intelligence",
				Type:       "lsp",
				Parameters: map[string]any{"filePath": filePath},
				DependsOn:  []string{"lsp_init"},
			},
			{
				ID:         "tool_analysis",
				Name:       "Run Analysis Tools",
				Type:       "tool",
				Parameters: map[string]any{"filePath": filePath},
				DependsOn:  []string{"lsp_intelligence"},
			},
		},
		Status:    "pending",
		Results:   make(map[string]interface{}),
		CreatedAt: time.Now(),
	}

	if err := io.executeWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	// Combine results
	intelligence := &models.CodeIntelligence{FilePath: filePath}

	if lspResult, ok := workflow.Results["lsp_intelligence"].(*models.CodeIntelligence); ok {
		intelligence = lspResult
	}

	return intelligence, nil
}

// ExecuteToolChain executes a chain of tools with dependencies
func (io *IntegrationOrchestrator) ExecuteToolChain(ctx context.Context, toolChain []ToolExecution) (map[string]interface{}, error) {
	workflow := &Workflow{
		ID:          fmt.Sprintf("toolchain-%d", time.Now().Unix()),
		Name:        "Tool Chain Execution",
		Description: "Execute a chain of tools with dependencies",
		Status:      "pending",
		Results:     make(map[string]any),
		CreatedAt:   time.Now(),
	}

	// Convert tool executions to workflow steps
	for i, execution := range toolChain {
		step := WorkflowStep{
			ID:         fmt.Sprintf("tool_%d", i),
			Name:       execution.ToolName,
			Type:       "tool",
			Parameters: execution.Parameters,
			DependsOn:  execution.DependsOn,
			MaxRetries: execution.MaxRetries,
		}
		workflow.Steps = append(workflow.Steps, step)
	}

	if err := io.executeWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	return workflow.Results, nil
}

// ExecuteParallelOperations executes multiple operations in parallel
func (io *IntegrationOrchestrator) ExecuteParallelOperations(ctx context.Context, operations []Operation) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	errors := make([]error, 0)

	var wg sync.WaitGroup
	resultChan := make(chan OperationResult, len(operations))

	for _, op := range operations {
		wg.Add(1)
		go func(operation Operation) {
			defer wg.Done()
			result := io.executeOperation(ctx, operation)
			resultChan <- result
		}(op)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if result.Error != nil {
			errors = append(errors, result.Error)
		} else {
			results[result.ID] = result.Data
		}
	}

	if len(errors) > 0 {
		return results, fmt.Errorf("parallel execution had %d errors: %v", len(errors), errors)
	}

	return results, nil
}

// executeWorkflow executes a workflow with proper dependency management
func (io *IntegrationOrchestrator) executeWorkflow(ctx context.Context, workflow *Workflow) error {
	workflow.Status = "running"
	workflow.UpdatedAt = time.Now()

	// Build dependency graph
	dependencyGraph := io.buildDependencyGraph(workflow.Steps)

	// Execute steps in topological order
	completed := make(map[string]bool)
	running := make(map[string]bool)

	for len(completed) < len(workflow.Steps) {
		// Find steps that can be executed
		executable := io.findExecutableSteps(workflow.Steps, dependencyGraph, completed, running)

		if len(executable) == 0 {
			// Check for circular dependencies or deadlocks
			if len(running) > 0 {
				// Wait for running steps to complete
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("workflow deadlock or circular dependency detected")
		}

		// Execute steps in parallel
		var wg sync.WaitGroup
		for _, step := range executable {
			wg.Add(1)
			running[step.ID] = true

			go func(s WorkflowStep) {
				defer wg.Done()
				defer func() { delete(running, s.ID) }()

				result, err := io.executeStep(ctx, &s)
				if err != nil {
					s.Error = err
					s.Status = "failed"
					workflow.Errors = append(workflow.Errors, err)
				} else {
					s.Result = result
					s.Status = "completed"
					workflow.Results[s.ID] = result
				}

				s.EndTime = &time.Time{}
				*s.EndTime = time.Now()
				completed[s.ID] = true
			}(step)
		}

		wg.Wait()
	}

	workflow.Status = "completed"
	workflow.UpdatedAt = time.Now()

	return nil
}

// executeStep executes a single workflow step
func (io *IntegrationOrchestrator) executeStep(ctx context.Context, step *WorkflowStep) (any, error) {
	step.Status = "running"
	now := time.Now()
	step.StartTime = &now

	defer func() {
		step.EndTime = &time.Time{}
		*step.EndTime = time.Now()
	}()

	var result interface{}
	var err error

	switch step.Type {
	case "lsp":
		result, err = io.executeLSPStep(ctx, step)
	case "mcp":
		result, err = io.executeMCPStep(ctx, step)
	case "tool":
		result, err = io.executeToolStep(ctx, step)
	case "llm":
		result, err = io.executeLLMStep(ctx, step)
	default:
		err = fmt.Errorf("unknown step type: %s", step.Type)
	}

	// Retry logic
	for err != nil && step.RetryCount < step.MaxRetries {
		step.RetryCount++
		log.Printf("Retrying step %s (attempt %d)", step.ID, step.RetryCount+1)

		time.Sleep(time.Duration(step.RetryCount) * time.Second)

		switch step.Type {
		case "lsp":
			result, err = io.executeLSPStep(ctx, step)
		case "mcp":
			result, err = io.executeMCPStep(ctx, step)
		case "tool":
			result, err = io.executeToolStep(ctx, step)
		case "llm":
			result, err = io.executeLLMStep(ctx, step)
		}
	}

	return result, err
}

// executeLSPStep executes an LSP-related step
func (io *IntegrationOrchestrator) executeLSPStep(ctx context.Context, step *WorkflowStep) (any, error) {
	switch step.Name {
	case "Initialize LSP Client":
		return nil, io.lspClient.StartServer(ctx)
	case "Get Code Intelligence":
		filePath, _ := step.Parameters["filePath"].(string)
		return io.lspClient.GetCodeIntelligence(ctx, filePath, nil)
	default:
		return nil, fmt.Errorf("unknown LSP step: %s", step.Name)
	}
}

// executeMCPStep executes an MCP-related step
func (io *IntegrationOrchestrator) executeMCPStep(ctx context.Context, step *WorkflowStep) (any, error) {
	if io.mcpManager == nil {
		return nil, fmt.Errorf("MCP manager not available")
	}

	// Extract MCP operation parameters
	operation, ok := step.Parameters["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("MCP operation parameter required")
	}

	switch operation {
	case "list_tools":
		return io.mcpManager.ListTools(), nil

	case "call_tool":
		toolName, ok := step.Parameters["toolName"].(string)
		if !ok {
			return nil, fmt.Errorf("toolName parameter required for call_tool operation")
		}

		params, ok := step.Parameters["params"].(map[string]interface{})
		if !ok {
			params = make(map[string]interface{})
		}

		return io.mcpManager.CallTool(ctx, toolName, params)

	case "list_servers":
		servers, err := io.mcpManager.ListMCPServers(ctx)
		return servers, err

	case "register_server":
		serverConfig, ok := step.Parameters["serverConfig"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("serverConfig parameter required for register_server operation")
		}

		return nil, io.mcpManager.RegisterServer(serverConfig)

	default:
		return nil, fmt.Errorf("unknown MCP operation: %s", operation)
	}
}

// executeToolStep executes a tool-related step
func (io *IntegrationOrchestrator) executeToolStep(ctx context.Context, step *WorkflowStep) (any, error) {
	toolName, ok := step.Parameters["toolName"].(string)
	if !ok {
		return nil, fmt.Errorf("toolName parameter required")
	}

	return io.toolRegistry.ExecuteTool(ctx, toolName, step.Parameters)
}

// executeLLMStep executes an LLM-related step
func (io *IntegrationOrchestrator) executeLLMStep(ctx context.Context, step *WorkflowStep) (any, error) {
	// Extract LLM operation parameters
	operation, ok := step.Parameters["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("LLM operation parameter required")
	}

	switch operation {
	case "complete":
		prompt, ok := step.Parameters["prompt"].(string)
		if !ok {
			return nil, fmt.Errorf("prompt parameter required for complete operation")
		}

		model, _ := step.Parameters["model"].(string)
		if model == "" {
			model = "gpt-3.5-turbo" // default
		}

		// Create LLM request
		llmReq := &models.LLMRequest{
			ID:     fmt.Sprintf("workflow-%s-%d", step.ID, time.Now().Unix()),
			Prompt: prompt,
			ModelParams: models.ModelParameters{
				Model: model,
			},
			CreatedAt: time.Now(),
		}

		// Add messages if provided
		if messages, ok := step.Parameters["messages"].([]interface{}); ok {
			llmMessages := make([]models.Message, 0, len(messages))
			for _, msg := range messages {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					role, _ := msgMap["role"].(string)
					content, _ := msgMap["content"].(string)
					llmMessages = append(llmMessages, models.Message{
						Role:    role,
						Content: content,
					})
				}
			}
			llmReq.Messages = llmMessages
		}

		// For now, we'll need to get a provider registry or direct provider
		// This is a simplified implementation
		return nil, fmt.Errorf("LLM provider integration requires provider registry access")

	case "stream":
		// Similar to complete but for streaming
		return nil, fmt.Errorf("LLM streaming not yet implemented in workflow steps")

	default:
		return nil, fmt.Errorf("unknown LLM operation: %s", operation)
	}
}

// executeOperation executes a single operation
func (io *IntegrationOrchestrator) executeOperation(ctx context.Context, op Operation) OperationResult {
	result := OperationResult{ID: op.ID}

	switch op.Type {
	case "lsp":
		data, err := io.executeLSPStep(ctx, &WorkflowStep{Name: op.Name, Parameters: op.Parameters})
		result.Data = data
		result.Error = err
	case "tool":
		data, err := io.executeToolStep(ctx, &WorkflowStep{Parameters: op.Parameters})
		result.Data = data
		result.Error = err
	default:
		result.Error = fmt.Errorf("unknown operation type: %s", op.Type)
	}

	return result
}

// Helper methods

func (io *IntegrationOrchestrator) buildDependencyGraph(steps []WorkflowStep) map[string][]string {
	graph := make(map[string][]string)
	stepMap := make(map[string]*WorkflowStep)

	for i := range steps {
		stepMap[steps[i].ID] = &steps[i]
		graph[steps[i].ID] = steps[i].DependsOn
	}

	// Detect cycles
	if io.hasCycles(graph) {
		log.Printf("Warning: Cycle detected in workflow dependencies")
		// Could implement cycle breaking logic here
	}

	return graph
}

// hasCycles detects if the dependency graph has cycles
func (io *IntegrationOrchestrator) hasCycles(graph map[string][]string) bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for node := range graph {
		if !visited[node] {
			if io.hasCyclesUtil(node, graph, visited, recStack) {
				return true
			}
		}
	}
	return false
}

// hasCyclesUtil is a utility function for cycle detection
func (io *IntegrationOrchestrator) hasCyclesUtil(node string, graph map[string][]string, visited, recStack map[string]bool) bool {
	visited[node] = true
	recStack[node] = true

	for _, neighbor := range graph[node] {
		if !visited[neighbor] && io.hasCyclesUtil(neighbor, graph, visited, recStack) {
			return true
		} else if recStack[neighbor] {
			return true
		}
	}

	recStack[node] = false
	return false
}

func (io *IntegrationOrchestrator) findExecutableSteps(steps []WorkflowStep, graph map[string][]string, completed, running map[string]bool) []WorkflowStep {
	var executable []WorkflowStep

	for _, step := range steps {
		if completed[step.ID] || running[step.ID] {
			continue
		}

		// Check if all dependencies are completed
		canExecute := true
		for _, dep := range graph[step.ID] {
			if !completed[dep] {
				canExecute = false
				break
			}
		}

		if canExecute {
			executable = append(executable, step)
		}
	}

	return executable
}

// Data structures

// ToolExecution represents a tool execution request
type ToolExecution struct {
	ToolName   string
	Parameters map[string]any
	DependsOn  []string
	MaxRetries int
}

// Operation represents a single operation to execute
type Operation struct {
	ID         string
	Type       string
	Name       string
	Parameters map[string]interface{}
}

// OperationResult represents the result of an operation
type OperationResult struct {
	ID    string
	Data  interface{}
	Error error
}
