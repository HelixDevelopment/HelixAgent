package services

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"golang.org/x/sync/semaphore"

	"dev.helix.agent/internal/models"
)

// maxParallelOperations limits the number of concurrent operations in
// ExecuteParallelOperations to avoid unbounded goroutine growth.
// Capped at 2*NumCPU or 16, whichever is smaller.
var maxParallelOperations = int64(min(2*runtime.NumCPU(), 16))

// IntegrationOrchestrator coordinates MCP, LSP, and tool execution.
//
// Concurrency: workflows is a safe.Store so registration, retrieval,
// listing, and deletion need no external synchronisation. Each Workflow
// value, once in the store, is mutated only by the single goroutine
// currently executing it (executeWorkflow holds the only reference
// during execution); publication before execution happens-before the
// workflow pointer becomes visible via Put.
type IntegrationOrchestrator struct {
	mcpManager       *MCPManager
	lspClient        *LSPClient
	toolRegistry     *ToolRegistry
	contextManager   *ContextManager
	providerRegistry *ProviderRegistry
	workflows        *safe.Store[string, *Workflow]
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
		workflows:      safe.NewStore[string, *Workflow](),
	}
}

// SetProviderRegistry sets the LLM provider registry for LLM workflow steps
func (io *IntegrationOrchestrator) SetProviderRegistry(registry *ProviderRegistry) {
	io.providerRegistry = registry
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

	// Register workflow in the map before execution
	io.workflows.Put(workflow.ID, workflow)

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
func (io *IntegrationOrchestrator) ExecuteToolChain(ctx context.Context, toolChain []IntegrationToolExecution) (map[string]interface{}, error) {
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

	// Register workflow in the map before execution
	io.workflows.Put(workflow.ID, workflow)

	if err := io.executeWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	return workflow.Results, nil
}

// ExecuteParallelOperations executes multiple operations in parallel.
// Concurrency is bounded by maxParallelOperations to prevent unbounded
// goroutine growth when the caller supplies a large operation list.
func (io *IntegrationOrchestrator) ExecuteParallelOperations(ctx context.Context, operations []Operation) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	errors := make([]error, 0)

	sem := semaphore.NewWeighted(maxParallelOperations)

	var wg sync.WaitGroup
	resultChan := make(chan OperationResult, len(operations))

	for _, op := range operations {
		// Acquire a semaphore slot before launching the goroutine so that
		// we never have more than maxParallelOperations in flight.
		if err := sem.Acquire(ctx, 1); err != nil {
			// Context cancelled or expired — stop scheduling.
			break
		}
		wg.Add(1)
		go func(operation Operation) {
			defer wg.Done()
			defer sem.Release(1)
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

	// Mutex to protect shared state accessed by goroutines
	var mu sync.Mutex

	for {
		mu.Lock()
		completedCount := len(completed)
		mu.Unlock()

		if completedCount >= len(workflow.Steps) {
			break
		}

		// Find steps that can be executed
		mu.Lock()
		executable := io.findExecutableSteps(workflow.Steps, dependencyGraph, completed, running)
		mu.Unlock()

		if len(executable) == 0 {
			// Check for circular dependencies or deadlocks
			mu.Lock()
			runningCount := len(running)
			mu.Unlock()

			if runningCount > 0 {
				// Wait for running steps to complete
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("workflow deadlock or circular dependency detected")
		}

		// Execute steps in parallel, bounded by maxParallelOperations
		stepSem := semaphore.NewWeighted(maxParallelOperations)
		var wg sync.WaitGroup
		for _, step := range executable {
			if err := stepSem.Acquire(ctx, 1); err != nil {
				break // context cancelled
			}
			wg.Add(1)
			mu.Lock()
			running[step.ID] = true
			mu.Unlock()

			go func(s WorkflowStep) {
				defer wg.Done()
				defer stepSem.Release(1)
				defer func() {
					mu.Lock()
					delete(running, s.ID)
					mu.Unlock()
				}()

				result, err := io.executeStep(ctx, &s)
				mu.Lock()
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
				mu.Unlock()
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
		//nolint:errcheck // parameter validation ensures correct type
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

	// Check if provider registry is available
	if io.providerRegistry == nil {
		return nil, fmt.Errorf("LLM provider registry not configured")
	}

	// Get provider name (default to first available)
	//nolint:errcheck // parameter validation ensures correct type
	providerName, _ := step.Parameters["provider"].(string)
	if providerName == "" {
		providers := io.providerRegistry.ListProviders()
		if len(providers) == 0 {
			return nil, fmt.Errorf("no LLM providers available")
		}
		providerName = providers[0]
	}

	provider, err := io.providerRegistry.GetProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", providerName, err)
	}

	// Build the LLM request
	llmReq, err := io.buildLLMRequest(step)
	if err != nil {
		return nil, err
	}

	switch operation {
	case "complete":
		return provider.Complete(ctx, llmReq)

	case "stream":
		// Get the streaming channel from provider
		streamChan, err := provider.CompleteStream(ctx, llmReq)
		if err != nil {
			return nil, fmt.Errorf("failed to start streaming: %w", err)
		}

		// Collect all streamed responses into a single result
		var fullContent string
		var lastResponse *models.LLMResponse

		for response := range streamChan {
			if response != nil {
				fullContent += response.Content
				lastResponse = response
			}
		}

		// Return combined response
		if lastResponse != nil {
			lastResponse.Content = fullContent
			return lastResponse, nil
		}

		return &models.LLMResponse{
			ID:        fmt.Sprintf("workflow-%s-%d", step.ID, time.Now().Unix()),
			RequestID: llmReq.ID,
			Content:   fullContent,
			CreatedAt: time.Now(),
		}, nil

	default:
		return nil, fmt.Errorf("unknown LLM operation: %s", operation)
	}
}

// buildLLMRequest constructs an LLMRequest from workflow step parameters
func (io *IntegrationOrchestrator) buildLLMRequest(step *WorkflowStep) (*models.LLMRequest, error) {
	//nolint:errcheck // parameter validation ensures correct type
	prompt, _ := step.Parameters["prompt"].(string)
	//nolint:errcheck // parameter validation ensures correct type
	model, _ := step.Parameters["model"].(string)
	if model == "" {
		model = "default"
	}

	llmReq := &models.LLMRequest{
		ID:     fmt.Sprintf("workflow-%s-%d", step.ID, time.Now().Unix()),
		Prompt: prompt,
		ModelParams: models.ModelParameters{
			Model: model,
		},
		CreatedAt: time.Now(),
	}

	// Add temperature if provided
	if temp, ok := step.Parameters["temperature"].(float64); ok {
		llmReq.ModelParams.Temperature = temp
	}

	// Add max_tokens if provided
	if maxTokens, ok := step.Parameters["max_tokens"].(int); ok {
		llmReq.ModelParams.MaxTokens = maxTokens
	} else if maxTokensFloat, ok := step.Parameters["max_tokens"].(float64); ok {
		llmReq.ModelParams.MaxTokens = int(maxTokensFloat)
	}

	// Add messages if provided
	if messages, ok := step.Parameters["messages"].([]interface{}); ok {
		llmMessages := make([]models.Message, 0, len(messages))
		for _, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				//nolint:errcheck // schema validation ensures correct type
				role, _ := msgMap["role"].(string)
				//nolint:errcheck // schema validation ensures correct type
				content, _ := msgMap["content"].(string)
				llmMessages = append(llmMessages, models.Message{
					Role:    role,
					Content: content,
				})
			}
		}
		llmReq.Messages = llmMessages
	}

	return llmReq, nil
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

// GetWorkflow returns a workflow by ID, or nil if not found.
func (io *IntegrationOrchestrator) GetWorkflow(id string) *Workflow {
	wf, _ := io.workflows.Get(id)
	return wf
}

// ListWorkflows returns the IDs of all registered workflows.
func (io *IntegrationOrchestrator) ListWorkflows() []string {
	return io.workflows.Keys()
}

// DeleteWorkflow removes a workflow by ID and returns true if it existed.
func (io *IntegrationOrchestrator) DeleteWorkflow(id string) bool {
	_, existed := io.workflows.Delete(id)
	return existed
}

// Data structures

// IntegrationToolExecution represents a tool execution request for the
// IntegrationOrchestrator workflow engine.
type IntegrationToolExecution struct {
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
