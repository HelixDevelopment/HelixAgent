package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager implements SubAgentManager interface
type Manager struct {
	config Config

	agents   map[string]*SubAgent
	agentsMu sync.RWMutex

	tasks   map[string]*SubAgentTask
	tasksMu sync.RWMutex

	agentInstances map[string]*agentInstance
	instancesMu    sync.RWMutex

	shutdown chan struct{}
	wg       sync.WaitGroup
}

// agentInstance wraps a sub-agent with execution state
type agentInstance struct {
	agent       *SubAgent
	profile     ProfileConfig
	cancelFunc  context.CancelFunc
	messageChan chan string
}

// NewManager creates a new sub-agent manager
func NewManager(config *Config) *Manager {
	if config == nil {
		config = &Config{}
	}

	return &Manager{
		config:         *config,
		agents:         make(map[string]*SubAgent),
		tasks:          make(map[string]*SubAgentTask),
		agentInstances: make(map[string]*agentInstance),
		shutdown:       make(chan struct{}),
	}
}

// Create creates a new sub-agent
func (m *Manager) Create(ctx context.Context, config SubAgentConfig) (*SubAgent, error) {
	m.agentsMu.Lock()
	defer m.agentsMu.Unlock()

	agent := &SubAgent{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("agent-%s", config.Profile),
		Type:      CustomAgent,
		Config:    config,
		Status:    StatusIdle,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.agents[agent.ID] = agent
	return agent, nil
}

// Get retrieves a sub-agent by ID
func (m *Manager) Get(ctx context.Context, id string) (*SubAgent, error) {
	m.agentsMu.RLock()
	defer m.agentsMu.RUnlock()

	agent, exists := m.agents[id]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", id)
	}
	return agent, nil
}

// List returns all sub-agents
func (m *Manager) List(ctx context.Context) ([]*SubAgent, error) {
	m.agentsMu.RLock()
	defer m.agentsMu.RUnlock()

	agents := make([]*SubAgent, 0, len(m.agents))
	for _, agent := range m.agents {
		agents = append(agents, agent)
	}
	return agents, nil
}

// Update updates a sub-agent configuration
func (m *Manager) Update(ctx context.Context, id string, config SubAgentConfig) error {
	m.agentsMu.Lock()
	defer m.agentsMu.Unlock()

	agent, exists := m.agents[id]
	if !exists {
		return fmt.Errorf("agent not found: %s", id)
	}

	agent.Config = config
	agent.UpdatedAt = time.Now()
	return nil
}

// Delete removes a sub-agent
func (m *Manager) Delete(ctx context.Context, id string) error {
	m.agentsMu.Lock()
	defer m.agentsMu.Unlock()

	if _, exists := m.agents[id]; !exists {
		return fmt.Errorf("agent not found: %s", id)
	}

	delete(m.agents, id)
	return nil
}

// Execute runs a task on a sub-agent
func (m *Manager) Execute(ctx context.Context, agentID string, task SubAgentTask) (*SubAgentTaskResult, error) {
	agent, err := m.Get(ctx, agentID)
	if err != nil {
		return nil, err
	}

	task.ID = uuid.New().String()
	task.AgentID = agentID
	task.Status = TaskRunning
	task.CreatedAt = time.Now()
	now := time.Now()
	task.StartedAt = &now

	m.tasksMu.Lock()
	m.tasks[task.ID] = &task
	m.tasksMu.Unlock()

	// Update agent status
	m.agentsMu.Lock()
	agent.Status = StatusRunning
	agent.UpdatedAt = time.Now()
	m.agentsMu.Unlock()

	// Execute the task
	result := m.executeTask(ctx, agent, &task)

	// Update task with result
	completedAt := time.Now()
	m.tasksMu.Lock()
	if existingTask, exists := m.tasks[task.ID]; exists {
		existingTask.Result = result
		existingTask.Status = TaskCompleted
		existingTask.CompletedAt = &completedAt
	}
	m.tasksMu.Unlock()

	// Update agent status
	m.agentsMu.Lock()
	agent.Status = StatusIdle
	agent.UpdatedAt = time.Now()
	m.agentsMu.Unlock()

	return result, nil
}

// ExecuteAsync runs a task asynchronously
func (m *Manager) ExecuteAsync(ctx context.Context, agentID string, task SubAgentTask) (string, error) {
	task.ID = uuid.New().String()
	task.AgentID = agentID
	task.Status = TaskPending
	task.CreatedAt = time.Now()

	m.tasksMu.Lock()
	m.tasks[task.ID] = &task
	m.tasksMu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		select {
		case <-m.shutdown:
			return
		default:
		}

		_, _ = m.Execute(ctx, agentID, task)
	}()

	return task.ID, nil
}

// GetTask retrieves task status and result
func (m *Manager) GetTask(ctx context.Context, taskID string) (*SubAgentTask, error) {
	m.tasksMu.RLock()
	defer m.tasksMu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

// CancelTask cancels a running task
func (m *Manager) CancelTask(ctx context.Context, taskID string) error {
	m.tasksMu.Lock()
	defer m.tasksMu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status != TaskRunning {
		return fmt.Errorf("task is not running: %s", taskID)
	}

	task.Status = TaskCancelled
	return nil
}

// SendMessage sends a message to a running sub-agent
func (m *Manager) SendMessage(ctx context.Context, agentID string, message string) error {
	m.instancesMu.RLock()
	instance, exists := m.agentInstances[agentID]
	m.instancesMu.RUnlock()

	if !exists {
		return fmt.Errorf("agent instance not found: %s", agentID)
	}

	select {
	case instance.messageChan <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CreateAgent creates an agent with the given profile (high-level API)
func (m *Manager) CreateAgent(ctx context.Context, agentType string, profile ProfileConfig) (Agent, error) {
	m.instancesMu.Lock()
	defer m.instancesMu.Unlock()

	// Create the underlying sub-agent
	config := SubAgentConfig{
		Model:       profile.Model,
		MaxTokens:   profile.MaxTokens,
		Temperature: profile.Temperature,
	}

	agentTypeEnum := SubAgentType(agentType)
	subAgent := &SubAgent{
		ID:        uuid.New().String(),
		Name:      profile.Name,
		Type:      agentTypeEnum,
		Config:    config,
		Status:    StatusIdle,
		Tools:     profile.Tools,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Set role based on agent type
	for _, defaultAgent := range DefaultSubAgents() {
		if defaultAgent.Type == agentTypeEnum {
			subAgent.Role = defaultAgent.Role
			subAgent.Description = defaultAgent.Description
			break
		}
	}

	m.agentsMu.Lock()
	m.agents[subAgent.ID] = subAgent
	m.agentsMu.Unlock()

	// Create the agent instance
	instance := &agentInstance{
		agent:       subAgent,
		profile:     profile,
		messageChan: make(chan string, 10),
	}

	m.agentInstances[subAgent.ID] = instance

	// Return the high-level agent wrapper
	return &agentWrapper{
		manager:   m,
		instance:  instance,
		agentType: agentTypeEnum,
	}, nil
}

// Shutdown cleans up resources
func (m *Manager) Shutdown(ctx context.Context) error {
	close(m.shutdown)

	// Cancel all running agent instances
	m.instancesMu.Lock()
	for _, instance := range m.agentInstances {
		if instance.cancelFunc != nil {
			instance.cancelFunc()
		}
		close(instance.messageChan)
	}
	m.instancesMu.Unlock()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// executeTask executes a task and returns the result
func (m *Manager) executeTask(ctx context.Context, agent *SubAgent, task *SubAgentTask) *SubAgentTaskResult {
	// This is a simulation of task execution
	// In a real implementation, this would:
	// 1. Call the LLM provider
	// 2. Execute tools as needed
	// 3. Track token usage

	result := &SubAgentTaskResult{
		Content: fmt.Sprintf("Task executed by %s agent: %s", agent.Type, task.Prompt),
		Usage: &TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	return result
}

// agentWrapper wraps an agent instance to provide the high-level Agent interface
type agentWrapper struct {
	manager   *Manager
	instance  *agentInstance
	agentType SubAgentType
}

// Execute runs an exploration task
func (a *agentWrapper) Execute(ctx context.Context, task Task) (ExploreResult, error) {
	subAgentTask := SubAgentTask{
		Type:   a.agentType,
		Prompt: task.Description,
	}

	result, err := a.manager.Execute(ctx, a.instance.agent.ID, subAgentTask)
	if err != nil {
		return ExploreResult{}, err
	}

	// Parse the result content as discoveries
	// In a real implementation, this would parse structured output
	exploreResult := ExploreResult{
		Discoveries:   []string{result.Content},
		FilesExamined: []string{},
	}

	return exploreResult, nil
}

// CreatePlan creates a plan based on exploration results
func (a *agentWrapper) CreatePlan(ctx context.Context, input PlanInput) (PlanResult, error) {
	prompt := fmt.Sprintf("Create a plan for: %s\n\nDiscoveries: %v\nConstraints: %v",
		input.Objective, input.Discoveries, input.Constraints)

	subAgentTask := SubAgentTask{
		Type:   PlanAgent,
		Prompt: prompt,
	}

	_, err := a.manager.Execute(ctx, a.instance.agent.ID, subAgentTask)
	if err != nil {
		return PlanResult{}, err
	}

	// Return a mock plan
	return PlanResult{
		Steps: []PlanStep{
			{Description: "Analyze requirements", Priority: "high"},
			{Description: "Design solution", Priority: "high"},
			{Description: "Implement changes", Priority: "medium"},
			{Description: "Test and validate", Priority: "medium"},
		},
		FilesToCreate: []string{},
		FilesToModify: []string{},
	}, nil
}

// ExecutePlan implements the plan
func (a *agentWrapper) ExecutePlan(ctx context.Context, plan PlanResult) (ImplementationResult, error) {
	prompt := fmt.Sprintf("Execute plan with %d steps", len(plan.Steps))

	subAgentTask := SubAgentTask{
		Type:   GeneralAgent,
		Prompt: prompt,
	}

	_, err := a.manager.Execute(ctx, a.instance.agent.ID, subAgentTask)
	if err != nil {
		return ImplementationResult{Error: err.Error()}, err
	}

	return ImplementationResult{
		FilesWritten:     []string{},
		CommandsExecuted: []string{},
	}, nil
}

// Ensure Manager implements SubAgentManager interface
var _ SubAgentManager = (*Manager)(nil)
