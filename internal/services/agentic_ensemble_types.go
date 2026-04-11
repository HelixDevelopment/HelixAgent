package services

import "time"

// AgenticMode represents the operating mode for the agentic ensemble.
type AgenticMode int

const (
	// AgenticModeReason performs reasoning-only without tool execution.
	AgenticModeReason AgenticMode = 0

	// AgenticModeExecute performs reasoning with tool execution.
	AgenticModeExecute AgenticMode = 1
)

// String returns the string representation of the AgenticMode.
func (m AgenticMode) String() string {
	switch m {
	case AgenticModeReason:
		return "reason"
	case AgenticModeExecute:
		return "execute"
	default:
		return "unknown"
	}
}

// AgenticTaskStatus represents the lifecycle status of an agentic task.
type AgenticTaskStatus int

const (
	// AgenticTaskPending indicates the task is waiting to be executed.
	AgenticTaskPending AgenticTaskStatus = 0

	// AgenticTaskRunning indicates the task is currently being executed.
	AgenticTaskRunning AgenticTaskStatus = 1

	// AgenticTaskCompleted indicates the task has completed successfully.
	AgenticTaskCompleted AgenticTaskStatus = 2

	// AgenticTaskFailed indicates the task has failed.
	AgenticTaskFailed AgenticTaskStatus = 3
)

// String returns the string representation of the AgenticTaskStatus.
func (s AgenticTaskStatus) String() string {
	switch s {
	case AgenticTaskPending:
		return "pending"
	case AgenticTaskRunning:
		return "running"
	case AgenticTaskCompleted:
		return "completed"
	case AgenticTaskFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// AgenticTask represents a decomposed unit of work within the ensemble.
type AgenticTask struct {
	ID               string            `json:"id"`
	Description      string            `json:"description"`
	Dependencies     []string          `json:"dependencies,omitempty"`
	ToolRequirements []string          `json:"tool_requirements,omitempty"`
	Priority         int               `json:"priority"`
	EstimatedSteps   int               `json:"estimated_steps"`
	Status           AgenticTaskStatus `json:"status"`
}

// AgenticResult holds the outcome of a single agent's task execution.
type AgenticResult struct {
	TaskID    string                 `json:"task_id"`
	AgentID   string                 `json:"agent_id"`
	Content   string                 `json:"content"`
	ToolCalls []AgenticToolExecution `json:"tool_calls,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Error     error                  `json:"-"`
}

// AgenticToolExecution captures a single tool invocation and its result
// within the agentic ensemble pipeline.
type AgenticToolExecution struct {
	Protocol  string        `json:"protocol"`
	Operation string        `json:"operation"`
	Input     interface{}   `json:"input,omitempty"`
	Output    interface{}   `json:"output,omitempty"`
	Duration  time.Duration `json:"duration"`
	Error     error         `json:"-"`
}

// AgenticEnsembleConfig holds all configuration for the agentic ensemble.
type AgenticEnsembleConfig struct {
	MaxConcurrentAgents       int           `json:"max_concurrent_agents"`
	MaxIterationsPerAgent     int           `json:"max_iterations_per_agent"`
	MaxToolIterationsPerPhase int           `json:"max_tool_iterations_per_phase"`
	AgentTimeout              time.Duration `json:"agent_timeout"`
	GlobalTimeout             time.Duration `json:"global_timeout"`
	ToolIterationTimeout      time.Duration `json:"tool_iteration_timeout"`
	EnableVision              bool          `json:"enable_vision"`
	EnableMemory              bool          `json:"enable_memory"`
	EnableExecution           bool          `json:"enable_execution"`
}

// DefaultAgenticEnsembleConfig returns the default configuration for the
// agentic ensemble with production-ready defaults.
func DefaultAgenticEnsembleConfig() AgenticEnsembleConfig {
	return AgenticEnsembleConfig{
		MaxConcurrentAgents:       5,
		MaxIterationsPerAgent:     20,
		MaxToolIterationsPerPhase: 5,
		AgentTimeout:              5 * time.Minute,
		GlobalTimeout:             15 * time.Minute,
		ToolIterationTimeout:      30 * time.Second,
		EnableVision:              true,
		EnableMemory:              true,
		EnableExecution:           true,
	}
}

// ToolInvocationSummary summarizes tool usage by protocol.
type ToolInvocationSummary struct {
	Protocol string `json:"protocol"`
	Count    int    `json:"count"`
}

// AgenticMetadata captures metadata about an agentic ensemble response.
type AgenticMetadata struct {
	Mode            string                  `json:"mode"`
	StagesCompleted []string                `json:"stages_completed"`
	AgentsSpawned   int                     `json:"agents_spawned"`
	TasksCompleted  int                     `json:"tasks_completed"`
	ToolsInvoked    []ToolInvocationSummary `json:"tools_invoked,omitempty"`
	TotalDurationMs int64                   `json:"total_duration_ms"`
	ProvenanceID    string                  `json:"provenance_id"`
}
