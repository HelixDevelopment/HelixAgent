package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/google/uuid"
)

// Manager implements SubAgentManager interface.
//
// Concurrent-safe by construction (CONST-029):
//   - agents/tasks/agentInstances are safe.Stores with no joint-atomicity
//     requirement across them; each Store mutation is independently
//     atomic. Field mutations on *SubAgent and *SubAgentTask values
//     route through Update callbacks (Pattern Beta).
type Manager struct {
	config Config

	agents         *safe.Store[string, *SubAgent]
	tasks          *safe.Store[string, *SubAgentTask]
	agentInstances *safe.Store[string, *agentInstance]

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
		agents:         safe.NewStore[string, *SubAgent](),
		tasks:          safe.NewStore[string, *SubAgentTask](),
		agentInstances: safe.NewStore[string, *agentInstance](),
		shutdown:       make(chan struct{}),
	}
}

// Create creates a new sub-agent
func (m *Manager) Create(ctx context.Context, config SubAgentConfig) (*SubAgent, error) {
	agent := &SubAgent{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("agent-%s", config.Profile),
		Type:      CustomAgent,
		Config:    config,
		Status:    StatusIdle,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.agents.Put(agent.ID, agent)
	return agent, nil
}

// Get retrieves a sub-agent by ID
func (m *Manager) Get(ctx context.Context, id string) (*SubAgent, error) {
	agent, exists := m.agents.Get(id)
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", id)
	}
	return agent, nil
}

// List returns all sub-agents
func (m *Manager) List(ctx context.Context) ([]*SubAgent, error) {
	return m.agents.Values(), nil
}

// Update updates a sub-agent configuration
func (m *Manager) Update(ctx context.Context, id string, config SubAgentConfig) error {
	var notFound bool
	m.agents.Update(id, func(agent *SubAgent, ok bool) (*SubAgent, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		agent.Config = config
		agent.UpdatedAt = time.Now()
		return agent, true
	})
	if notFound {
		return fmt.Errorf("agent not found: %s", id)
	}
	return nil
}

// Delete removes a sub-agent
func (m *Manager) Delete(ctx context.Context, id string) error {
	if _, ok := m.agents.Delete(id); !ok {
		return fmt.Errorf("agent not found: %s", id)
	}
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

	m.tasks.Put(task.ID, &task)

	// Update agent status
	m.agents.Update(agentID, func(a *SubAgent, ok bool) (*SubAgent, bool) {
		if !ok {
			return nil, false
		}
		a.Status = StatusRunning
		a.UpdatedAt = time.Now()
		return a, true
	})

	// Execute the task
	result := m.executeTask(ctx, agent, &task)

	// Update task with result
	completedAt := time.Now()
	m.tasks.Update(task.ID, func(existing *SubAgentTask, ok bool) (*SubAgentTask, bool) {
		if !ok {
			return nil, false
		}
		existing.Result = result
		existing.Status = TaskCompleted
		existing.CompletedAt = &completedAt
		return existing, true
	})

	// Update agent status
	m.agents.Update(agentID, func(a *SubAgent, ok bool) (*SubAgent, bool) {
		if !ok {
			return nil, false
		}
		a.Status = StatusIdle
		a.UpdatedAt = time.Now()
		return a, true
	})

	return result, nil
}

// ExecuteAsync runs a task asynchronously
func (m *Manager) ExecuteAsync(ctx context.Context, agentID string, task SubAgentTask) (string, error) {
	task.ID = uuid.New().String()
	task.AgentID = agentID
	task.Status = TaskPending
	task.CreatedAt = time.Now()

	m.tasks.Put(task.ID, &task)

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
	task, exists := m.tasks.Get(taskID)
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

// CancelTask cancels a running task
func (m *Manager) CancelTask(ctx context.Context, taskID string) error {
	var notFound, notRunning bool
	m.tasks.Update(taskID, func(task *SubAgentTask, ok bool) (*SubAgentTask, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		if task.Status != TaskRunning {
			notRunning = true
			return task, true
		}
		task.Status = TaskCancelled
		return task, true
	})
	if notFound {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if notRunning {
		return fmt.Errorf("task is not running: %s", taskID)
	}
	return nil
}

// SendMessage sends a message to a running sub-agent
func (m *Manager) SendMessage(ctx context.Context, agentID string, message string) error {
	instance, exists := m.agentInstances.Get(agentID)
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

	m.agents.Put(subAgent.ID, subAgent)

	// Create the agent instance
	instance := &agentInstance{
		agent:       subAgent,
		profile:     profile,
		messageChan: make(chan string, 10),
	}

	m.agentInstances.Put(subAgent.ID, instance)

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
	m.agentInstances.Range(func(_ string, instance *agentInstance) bool {
		if instance.cancelFunc != nil {
			instance.cancelFunc()
		}
		close(instance.messageChan)
		return true
	})

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
