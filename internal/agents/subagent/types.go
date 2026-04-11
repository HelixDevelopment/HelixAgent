// Package subagent implements a sub-agent system inspired by Snow CLI
// Sub-agents run in isolated contexts to preserve main workflow tokens
package subagent

import (
	"context"
	"time"
)

// ToolCall represents a tool invocation from an agent
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// SubAgentType represents the type of sub-agent
type SubAgentType string

const (
	// ExploreAgent searches and analyzes code
	ExploreAgent SubAgentType = "explore"
	// PlanAgent creates comprehensive plans
	PlanAgent SubAgentType = "plan"
	// GeneralAgent handles batch operations
	GeneralAgent SubAgentType = "general"
	// CustomAgent user-defined sub-agent
	CustomAgent SubAgentType = "custom"
)

// SubAgentStatus represents the current state of a sub-agent
type SubAgentStatus string

const (
	StatusIdle      SubAgentStatus = "idle"
	StatusRunning   SubAgentStatus = "running"
	StatusCompleted SubAgentStatus = "completed"
	StatusFailed    SubAgentStatus = "failed"
	StatusCancelled SubAgentStatus = "cancelled"
)

// TaskStatus represents task execution status
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// Config holds the manager configuration
type Config struct {
	ProviderType string
	APIKey       string
	Logger       interface{} // zap.Logger or similar
}

// ProfileConfig holds the configuration for creating an agent profile
type ProfileConfig struct {
	Name        string
	Model       string
	MaxTokens   int
	Temperature float64
	Tools       []string
}

// Task represents a task to be executed by a sub-agent
type Task struct {
	Description string
	MaxSteps    int
}

// TaskResult contains the result of a task execution
type TaskResult struct {
	Result ExploreResult
	Error  error
}

// ExploreResult contains discoveries from an exploration task
type ExploreResult struct {
	Discoveries   []string
	FilesExamined []string
}

// PlanInput contains input for creating a plan
type PlanInput struct {
	Objective   string
	Discoveries []string
	Constraints []string
}

// PlanResult contains the created plan
type PlanResult struct {
	Steps         []PlanStep
	FilesToCreate []string
	FilesToModify []string
}

// PlanStep represents a single step in a plan
type PlanStep struct {
	Description string
	Priority    string
}

// ImplementationResult contains the result of implementing a plan
type ImplementationResult struct {
	FilesWritten     []string
	CommandsExecuted []string
	Error            string
}

// Agent is an interface for executable agents
type Agent interface {
	Execute(ctx context.Context, task Task) (ExploreResult, error)
	CreatePlan(ctx context.Context, input PlanInput) (PlanResult, error)
	ExecutePlan(ctx context.Context, plan PlanResult) (ImplementationResult, error)
}

// SubAgent represents a specialized sub-agent
// Based on Snow CLI's sub-agent architecture
type SubAgent struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        SubAgentType           `json:"type"`
	Description string                 `json:"description"`
	Role        string                 `json:"role"`
	Config      SubAgentConfig         `json:"config"`
	Status      SubAgentStatus         `json:"status"`
	Tools       []string               `json:"tools"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// SubAgentConfig holds sub-agent configuration
type SubAgentConfig struct {
	// Profile is the API configuration profile to use
	Profile string `json:"profile,omitempty"`

	// Model overrides the default model
	Model string `json:"model,omitempty"`

	// MaxTokens for the sub-agent
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature for generation
	Temperature float64 `json:"temperature,omitempty"`

	// EnableThinking for reasoning models
	EnableThinking bool `json:"enable_thinking,omitempty"`

	// ThinkingBudget for extended thinking
	ThinkingBudget int `json:"thinking_budget,omitempty"`
}

// SubAgentTask represents a task assigned to a sub-agent
type SubAgentTask struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	Type        SubAgentType           `json:"type"`
	Prompt      string                 `json:"prompt"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Status      TaskStatus             `json:"status"`
	Result      *SubAgentTaskResult    `json:"result,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// TaskResult contains the result of a sub-agent task
type SubAgentTaskResult struct {
	Content   string                 `json:"content"`
	ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
	Usage     *TokenUsage            `json:"usage,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// SubAgentManager manages sub-agents and their execution
type SubAgentManager interface {
	// Create creates a new sub-agent
	Create(ctx context.Context, config SubAgentConfig) (*SubAgent, error)

	// Get retrieves a sub-agent by ID
	Get(ctx context.Context, id string) (*SubAgent, error)

	// List returns all sub-agents
	List(ctx context.Context) ([]*SubAgent, error)

	// Update updates a sub-agent configuration
	Update(ctx context.Context, id string, config SubAgentConfig) error

	// Delete removes a sub-agent
	Delete(ctx context.Context, id string) error

	// Execute runs a task on a sub-agent
	Execute(ctx context.Context, agentID string, task SubAgentTask) (*SubAgentTaskResult, error)

	// ExecuteAsync runs a task asynchronously
	ExecuteAsync(ctx context.Context, agentID string, task SubAgentTask) (string, error)

	// GetTask retrieves task status and result
	GetTask(ctx context.Context, taskID string) (*SubAgentTask, error)

	// CancelTask cancels a running task
	CancelTask(ctx context.Context, taskID string) error

	// SendMessage sends a message to a running sub-agent
	SendMessage(ctx context.Context, agentID string, message string) error

	// CreateAgent creates an agent with the given profile (high-level API)
	CreateAgent(ctx context.Context, agentType string, profile ProfileConfig) (Agent, error)

	// Shutdown cleans up resources
	Shutdown(ctx context.Context) error
}

// DefaultSubAgents returns the built-in system sub-agents
// Based on Snow CLI's built-in agents
func DefaultSubAgents() []*SubAgent {
	return []*SubAgent{
		{
			ID:          "explore",
			Name:        "Explore Agent",
			Type:        ExploreAgent,
			Description: "An exploration agent for searching code functionality, focusing on locating code positions",
			Role: `You are an exploration agent specialized in code search and analysis.
Your task is to:
1. Search for code patterns, functions, and definitions
2. Analyze code structure and dependencies
3. Locate specific implementations
4. Provide file paths and line numbers

Be precise and thorough in your exploration.`,
			Config: SubAgentConfig{
				MaxTokens:   4096,
				Temperature: 0.3,
			},
			Tools: []string{
				"filesystem-read",
				"codebase-search",
				"ace-find_definition",
				"ace-semantic_search",
			},
		},
		{
			ID:          "plan",
			Name:        "Plan Agent",
			Type:        PlanAgent,
			Description: "A planning agent for developing comprehensive coding plans and guidance",
			Role: `You are a planning agent specialized in creating detailed implementation plans.
Your task is to:
1. Break down complex features into steps
2. Identify dependencies and prerequisites
3. Create implementation roadmaps
4. Provide actionable guidance

Think step-by-step and provide clear, actionable plans.`,
			Config: SubAgentConfig{
				MaxTokens:      8192,
				Temperature:    0.5,
				EnableThinking: true,
				ThinkingBudget: 4000,
			},
			Tools: []string{
				"filesystem-read",
				"codebase-search",
			},
		},
		{
			ID:          "general",
			Name:        "General Purpose Agent",
			Type:        GeneralAgent,
			Description: "A general-purpose agent for batch operations and systematic refactoring",
			Role: `You are a general-purpose coding agent.
Your task is to:
1. Handle batch modifications across multiple files
2. Perform systematic refactoring
3. Implement straightforward features
4. Complete singular but multi-file tasks

Work efficiently and maintain code quality.`,
			Config: SubAgentConfig{
				MaxTokens:   4096,
				Temperature: 0.4,
			},
			Tools: []string{
				"filesystem-read",
				"filesystem-edit",
				"filesystem-create",
				"terminal-execute",
			},
		},
	}
}

// LLM types for sub-agent operations

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolDefinition represents a tool definition
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model          string           `json:"model"`
	Messages       []Message        `json:"messages"`
	MaxTokens      int              `json:"max_tokens"`
	Temperature    float64          `json:"temperature"`
	Tools          []ToolDefinition `json:"tools,omitempty"`
	EnableThinking bool             `json:"enable_thinking,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     TokenUsage `json:"usage"`
}

// AgentResult represents the result from an agent execution
type AgentResult struct {
	Content   string                 `json:"content"`
	ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
	Usage     *TokenUsage            `json:"usage,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
