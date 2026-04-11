package clis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AgentType is an alias for CLIAgentType for backward compatibility
type AgentType = CLIAgentType

// CLIAgentType represents the type of CLI agent
type CLIAgentType string

const (
	TypeAider          CLIAgentType = "aider"
	TypeClaudeCode     CLIAgentType = "claude_code"
	TypeCodex          CLIAgentType = "codex"
	TypeCline          CLIAgentType = "cline"
	TypeOpenHands      CLIAgentType = "openhands"
	TypeKiro           CLIAgentType = "kiro"
	TypeContinue       CLIAgentType = "continue"
	TypeGoose          CLIAgentType = "goose"
	TypeForge          CLIAgentType = "forge"
	TypePlandex        CLIAgentType = "plandex"
	TypeHelixAgent     CLIAgentType = "helixagent"
	TypeSupermaven     CLIAgentType = "supermaven"
	TypeCursor         CLIAgentType = "cursor"
	TypeWindsurf       CLIAgentType = "windsurf"
	TypeAugment        CLIAgentType = "augment"
	TypeSourcegraph    CLIAgentType = "sourcegraph"
	TypeCodeium        CLIAgentType = "codeium"
	TypeTabnine        CLIAgentType = "tabnine"
	TypeCodeGPT        CLIAgentType = "codegpt"
	TypeTwin           CLIAgentType = "twin"
	TypeDevin          CLIAgentType = "devin"
	TypeDevika         CLIAgentType = "devika"
	TypeSWEAgent       CLIAgentType = "swe_agent"
	TypeGPTPilot       CLIAgentType = "gpt_pilot"
	TypeMetamorph      CLIAgentType = "metamorph"
	TypeJunie          CLIAgentType = "junie"
	TypeAmazonQ        CLIAgentType = "amazon_q"
	TypeGitHubCopilot  CLIAgentType = "github_copilot"
	TypeJetBrainsAI    CLIAgentType = "jetbrains_ai"
	TypeCodeGemma      CLIAgentType = "codegemma"
	TypeStarCoder      CLIAgentType = "starcoder"
	TypeQwenCoder      CLIAgentType = "qwencoder"
	TypeMistralCode    CLIAgentType = "mistralcode"
	TypeGeminiAssist   CLIAgentType = "gemini_assist"
	TypeCodey          CLIAgentType = "codey"
	TypeLlamaCode      CLIAgentType = "llama_code"
	TypeDeepSeekCoder  CLIAgentType = "deepseek_coder"
	TypeWizardCoder    CLIAgentType = "wizard_coder"
	TypePhind          CLIAgentType = "phind"
	TypeCody           CLIAgentType = "cody"
	TypeCursorSh       CLIAgentType = "cursorsh"
	TypeTrae           CLIAgentType = "trae"
	TypeBlackbox       CLIAgentType = "blackbox"
	TypeLovable        CLIAgentType = "lovable"
	TypeV0             CLIAgentType = "v0"
	TypeTempo          CLIAgentType = "tempo"
	TypeBolt           CLIAgentType = "bolt"
	TypeReplitAgent    CLIAgentType = "replit_agent"
	TypeIDX            CLIAgentType = "idx"
	TypeFirebaseStudio CLIAgentType = "firebase_studio"
	TypeCascade        CLIAgentType = "cascade"
)

// InstanceStatus represents the status of a CLI agent instance
type InstanceStatus string

const (
	StatusIdle        InstanceStatus = "idle"
	StatusRunning     InstanceStatus = "running"
	StatusPaused      InstanceStatus = "paused"
	StatusError       InstanceStatus = "error"
	StatusStopped     InstanceStatus = "stopped"
	StatusCreating    InstanceStatus = "creating"
	StatusActive      InstanceStatus = "active"
	StatusTerminating InstanceStatus = "terminating"
	StatusTerminated  InstanceStatus = "terminated"
	StatusFailed      InstanceStatus = "failed"
	StatusBackground  InstanceStatus = "background"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskPending         TaskStatus = "pending"
	TaskStatusPending   TaskStatus = "pending" // Alias for compatibility
	TaskRunning         TaskStatus = "running"
	TaskStatusRunning   TaskStatus = "running" // Alias for compatibility
	TaskCompleted       TaskStatus = "completed"
	TaskStatusCompleted TaskStatus = "completed" // Alias for compatibility
	TaskFailed          TaskStatus = "failed"
	TaskStatusFailed    TaskStatus = "failed" // Alias for compatibility
	TaskCancelled       TaskStatus = "cancelled"
	TaskStatusCancelled TaskStatus = "cancelled" // Alias for compatibility
	TaskRetrying        TaskStatus = "retrying"
	TaskStatusAssigned  TaskStatus = "assigned" // Additional status
	TaskStatusExpired   TaskStatus = "expired"  // Additional status
)

// EventType represents the type of event
type EventType string

const (
	EventInstanceCreated EventType = "instance_created"
	EventInstanceStarted EventType = "instance_started"
	EventInstanceStopped EventType = "instance_stopped"
	EventInstanceError   EventType = "instance_error"
	EventTaskSubmitted   EventType = "task_submitted"
	EventTaskCompleted   EventType = "task_completed"
	EventTaskFailed      EventType = "task_failed"
	EventHeartbeat       EventType = "heartbeat"
	EventTypeStatus      EventType = "status"   // For status events
	EventTypeProgress    EventType = "progress" // For progress events
	EventTypeError       EventType = "error"    // For error events
)

// Event represents an event in the system
type Event struct {
	ID         uuid.UUID              `json:"id"`
	Type       EventType              `json:"type"`
	InstanceID uuid.UUID              `json:"instance_id"`
	Topic      string                 `json:"topic,omitempty"`
	Source     string                 `json:"source,omitempty"`
	Payload    map[string]interface{} `json:"payload"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AgentInstance represents a CLI agent instance
type AgentInstance struct {
	ID                string                 `json:"id"`
	Type              CLIAgentType           `json:"type"`
	Name              string                 `json:"name"`
	Status            InstanceStatus         `json:"status"`
	Health            HealthStatus           `json:"health"`
	Config            InstanceConfig         `json:"config"`
	Provider          string                 `json:"provider"`
	Resources         ResourceLimits         `json:"resources"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	StartedAt         *time.Time             `json:"started_at,omitempty"`
	SessionID         string                 `json:"session_id,omitempty"`
	TaskID            string                 `json:"task_id,omitempty"`
	RequestsProcessed uint64                 `json:"requests_processed"`
	TotalExecTimeMs   uint64                 `json:"total_exec_time_ms"`
	ErrorsCount       uint64                 `json:"errors_count"`
	State             map[string]interface{} `json:"state,omitempty"`
	HealthDetails     map[string]interface{} `json:"health_details,omitempty"`
	LastHealthCheck   *time.Time             `json:"last_health_check,omitempty"`
	RequestCh         chan *Request          `json:"-"`
	ResponseCh        chan *Response         `json:"-"`
	EventCh           chan *Event            `json:"-"`
}

// CanAcceptWork returns true if the instance can accept new work
func (a *AgentInstance) CanAcceptWork() bool {
	return (a.Status == StatusIdle || a.Status == StatusActive) && a.Health == HealthHealthy
}

// IsActive returns true if the instance is active
func (a *AgentInstance) IsActive() bool {
	return a.Status == StatusActive || a.Status == StatusIdle || a.Status == StatusBackground || a.Status == StatusRunning
}

// IsHealthy returns true if the instance is healthy
func (a *AgentInstance) IsHealthy() bool {
	return a.Health == HealthHealthy
}

// HealthStatus represents the health of an instance
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
	HealthDegraded  HealthStatus = "degraded"
)

// ResourceLimits defines resource constraints for an instance
type ResourceLimits struct {
	MaxMemoryMB   int `json:"max_memory_mb"`
	MaxCPUPercent int `json:"max_cpu_percent"`
	MaxDiskMB     int `json:"max_disk_mb"`
}

// ProviderConfig holds provider configuration
type ProviderConfig struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	APIKey string `json:"-"` // Never serialize
}

// Request type constants
const (
	RequestTypeExecute = "execute"
	RequestTypeQuery   = "query"
	RequestTypeHealth  = "health"
	RequestTypeCancel  = "cancel"
)

// Task type constants
const (
	TaskTypeGitOperation  = "git_operation"
	TaskTypeCodeAnalysis  = "code_analysis"
	TaskTypeDocumentation = "documentation"
	TaskTypeTesting       = "testing"
	TaskTypeLinting       = "linting"
	TaskTypeBuild         = "build"
	TaskTypeDeploy        = "deploy"
	TaskTypeCodeReview    = "code_review"
)

// Request represents a request to an agent instance
type Request struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Payload   interface{}   `json:"payload"`
	Timestamp time.Time     `json:"timestamp"`
	Timeout   time.Duration `json:"timeout,omitempty"`
}

// ErrorDetail represents detailed error information
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Response represents a response from an agent instance
type Response struct {
	ID        string        `json:"id"`
	RequestID string        `json:"request_id"`
	Success   bool          `json:"success"`
	Payload   interface{}   `json:"payload,omitempty"`
	Result    interface{}   `json:"result,omitempty"`
	Error     *ErrorDetail  `json:"error,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// CLIAgentInstance represents a CLI agent instance
type CLIAgentInstance struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	Type          CLIAgentType    `json:"type" db:"type"`
	Status        InstanceStatus  `json:"status" db:"status"`
	Config        InstanceConfig  `json:"config" db:"config"`
	SessionID     *uuid.UUID      `json:"session_id,omitempty" db:"session_id"`
	UserID        *uuid.UUID      `json:"user_id,omitempty" db:"user_id"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
	LastHeartbeat *time.Time      `json:"last_heartbeat,omitempty" db:"last_heartbeat"`
	Metadata      json.RawMessage `json:"metadata,omitempty" db:"metadata"`
}

// InstanceConfig holds configuration for a CLI agent instance
type InstanceConfig struct {
	// Common settings
	WorkingDir  string            `json:"working_dir,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	MaxRetries  int               `json:"max_retries,omitempty"`

	// Provider settings
	ProviderType string `json:"provider_type,omitempty"`
	Model        string `json:"model,omitempty"`
	APIKey       string `json:"-"` // Never serialize

	// Resource limits
	MaxMemoryMB   int `json:"max_memory_mb,omitempty"`
	MaxCPUPercent int `json:"max_cpu_percent,omitempty"`

	// Health check settings
	HealthCheckInterval time.Duration `json:"health_check_interval,omitempty"`

	// Feature toggles
	EnableGit     bool `json:"enable_git,omitempty"`
	EnableBrowser bool `json:"enable_browser,omitempty"`
	EnableSandbox bool `json:"enable_sandbox,omitempty"`
	AutoApprove   bool `json:"auto_approve,omitempty"`

	// Type-specific settings
	AiderConfig  *AiderConfig      `json:"aider_config,omitempty"`
	ClaudeConfig *ClaudeCodeConfig `json:"claude_config,omitempty"`
	CodexConfig  *CodexConfig      `json:"codex_config,omitempty"`
	ClineConfig  *ClineConfig      `json:"cline_config,omitempty"`
}

// AiderConfig holds Aider-specific configuration
type AiderConfig struct {
	MapTokens         int      `json:"map_tokens,omitempty"`
	EnableRepoMap     bool     `json:"enable_repo_map,omitempty"`
	EnableDiffEditing bool     `json:"enable_diff_editing,omitempty"`
	CommitAttribution string   `json:"commit_attribution,omitempty"`
	IgnoredFiles      []string `json:"ignored_files,omitempty"`
}

// ClaudeCodeConfig holds Claude Code-specific configuration
type ClaudeCodeConfig struct {
	EnableToolUse    bool     `json:"enable_tool_use,omitempty"`
	EnableAutoMode   bool     `json:"enable_auto_mode,omitempty"`
	ApprovalRequired []string `json:"approval_required,omitempty"`
	MaxToolCalls     int      `json:"max_tool_calls,omitempty"`
}

// CodexConfig holds Codex-specific configuration
type CodexConfig struct {
	EnableInterpreter bool              `json:"enable_interpreter,omitempty"`
	EnableReasoning   bool              `json:"enable_reasoning,omitempty"`
	ReasoningEffort   string            `json:"reasoning_effort,omitempty"`
	SandboxConfig     map[string]string `json:"sandbox_config,omitempty"`
}

// ClineConfig holds Cline-specific configuration
type ClineConfig struct {
	EnableBrowser     bool   `json:"enable_browser,omitempty"`
	EnableComputerUse bool   `json:"enable_computer_use,omitempty"`
	BrowserViewport   string `json:"browser_viewport,omitempty"`
	MaxAutonomySteps  int    `json:"max_autonomy_steps,omitempty"`
}

// Task is an alias for CLIAgentTask for backward compatibility
type Task = CLIAgentTask

// CLIAgentTask represents a task for a CLI agent
type CLIAgentTask struct {
	ID              string          `json:"id" db:"id"`
	InstanceID      uuid.UUID       `json:"instance_id" db:"instance_id"`
	ParentTaskID    *uuid.UUID      `json:"parent_task_id,omitempty" db:"parent_task_id"`
	Type            string          `json:"type" db:"type"`
	Name            string          `json:"name,omitempty" db:"name"`
	Status          TaskStatus      `json:"status" db:"status"`
	Priority        int             `json:"priority" db:"priority"`
	Input           json.RawMessage `json:"input" db:"input"`
	Output          json.RawMessage `json:"output,omitempty" db:"output"`
	Payload         interface{}     `json:"payload,omitempty" db:"payload"`
	Error           *string         `json:"error,omitempty" db:"error"`
	StartedAt       *time.Time      `json:"started_at,omitempty" db:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	DurationMs      *int            `json:"duration_ms,omitempty" db:"duration_ms"`
	RetryCount      int             `json:"retry_count" db:"retry_count"`
	MaxRetries      int             `json:"max_retries" db:"max_retries"`
	ProgressPercent int             `json:"progress_percent,omitempty" db:"progress_percent"`
}

// TaskInput is a generic interface for task inputs
type TaskInput interface {
	Validate() error
}

// TaskOutput is a generic interface for task outputs
type TaskOutput interface {
	IsSuccess() bool
}

// RepoMapInput for repo map generation tasks
type RepoMapInput struct {
	Query          string   `json:"query"`
	MentionedFiles []string `json:"mentioned_files,omitempty"`
	MapTokens      int      `json:"map_tokens,omitempty"`
}

func (r RepoMapInput) Validate() error {
	if r.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

// RepoMapOutput for repo map generation results
type RepoMapOutput struct {
	Symbols    []Symbol   `json:"symbols"`
	Files      []FileInfo `json:"files"`
	TokenCount int        `json:"token_count"`
	Success    bool       `json:"success"`
	Error      string     `json:"error,omitempty"`
}

func (r RepoMapOutput) IsSuccess() bool {
	return r.Success
}

// Symbol represents a code symbol
type Symbol struct {
	Name           string  `json:"name"`
	Type           string  `json:"type"` // function, class, method, variable, etc.
	FilePath       string  `json:"file_path"`
	LineStart      int     `json:"line_start"`
	LineEnd        int     `json:"line_end"`
	Signature      string  `json:"signature,omitempty"`
	Documentation  string  `json:"documentation,omitempty"`
	RelevanceScore float64 `json:"relevance_score"`
}

// FileInfo represents file information
type FileInfo struct {
	Path         string `json:"path"`
	Language     string `json:"language"`
	Size         int64  `json:"size"`
	LastModified int64  `json:"last_modified"`
}

// DiffApplyInput for diff application tasks
type DiffApplyInput struct {
	FilePath        string `json:"file_path"`
	SearchBlock     string `json:"search_block"`
	ReplaceBlock    string `json:"replace_block"`
	CreateIfMissing bool   `json:"create_if_missing,omitempty"`
}

func (d DiffApplyInput) Validate() error {
	if d.FilePath == "" {
		return fmt.Errorf("file_path is required")
	}
	if d.SearchBlock == "" && !d.CreateIfMissing {
		return fmt.Errorf("search_block is required unless create_if_missing is true")
	}
	return nil
}

// DiffApplyOutput for diff application results
type DiffApplyOutput struct {
	Success         bool   `json:"success"`
	FilePath        string `json:"file_path"`
	OriginalContent string `json:"original_content,omitempty"`
	NewContent      string `json:"new_content,omitempty"`
	DiffContent     string `json:"diff_content,omitempty"`
	Error           string `json:"error,omitempty"`
}

func (d DiffApplyOutput) IsSuccess() bool {
	return d.Success
}

// GitCommitInput for git commit tasks
type GitCommitInput struct {
	Message     string   `json:"message"`
	Files       []string `json:"files,omitempty"`
	Attribution string   `json:"attribution,omitempty"`
	AllowEmpty  bool     `json:"allow_empty,omitempty"`
}

func (g GitCommitInput) Validate() error {
	if g.Message == "" {
		return fmt.Errorf("commit message is required")
	}
	return nil
}

// GitCommitOutput for git commit results
type GitCommitOutput struct {
	Success      bool     `json:"success"`
	CommitHash   string   `json:"commit_hash,omitempty"`
	FilesChanged []string `json:"files_changed,omitempty"`
	Error        string   `json:"error,omitempty"`
}

func (g GitCommitOutput) IsSuccess() bool {
	return g.Success
}

// ToolUseInput for tool use tasks
type ToolUseInput struct {
	ToolName         string                 `json:"tool_name"`
	Arguments        map[string]interface{} `json:"arguments"`
	RequiresApproval bool                   `json:"requires_approval"`
}

func (t ToolUseInput) Validate() error {
	if t.ToolName == "" {
		return fmt.Errorf("tool_name is required")
	}
	return nil
}

// ToolUseOutput for tool use results
type ToolUseOutput struct {
	Success    bool                   `json:"success"`
	ToolName   string                 `json:"tool_name"`
	Result     map[string]interface{} `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	DurationMs int                    `json:"duration_ms"`
}

func (t ToolUseOutput) IsSuccess() bool {
	return t.Success
}

// InstanceManagerInterface defines the interface for managing CLI agent instances
type InstanceManagerInterface interface {
	Create(ctx context.Context, agentType CLIAgentType, config InstanceConfig) (*AgentInstance, error)
	Get(ctx context.Context, id uuid.UUID) (*AgentInstance, error)
	List(ctx context.Context, filter InstanceFilter) ([]*AgentInstance, error)
	Update(ctx context.Context, id uuid.UUID, config InstanceConfig) (*AgentInstance, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Start(ctx context.Context, id uuid.UUID) error
	Stop(ctx context.Context, id uuid.UUID) error
	Heartbeat(ctx context.Context, id uuid.UUID) error
}

// InstanceFilter for listing instances
type InstanceFilter struct {
	Type      *CLIAgentType
	Status    *InstanceStatus
	UserID    *uuid.UUID
	SessionID *uuid.UUID
}

// TaskManager manages CLI agent tasks
type TaskManager interface {
	Create(ctx context.Context, instanceID uuid.UUID, taskType string, input TaskInput, priority int) (*CLIAgentTask, error)
	Get(ctx context.Context, id uuid.UUID) (*CLIAgentTask, error)
	List(ctx context.Context, filter TaskFilter) ([]*CLIAgentTask, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status TaskStatus, output TaskOutput, err error) error
	Cancel(ctx context.Context, id uuid.UUID) error
	Retry(ctx context.Context, id uuid.UUID) (*CLIAgentTask, error)
}

// TaskFilter for listing tasks
type TaskFilter struct {
	InstanceID *uuid.UUID
	Status     *TaskStatus
	Type       *string
}

// CLIAgent defines the interface that all CLI agent implementations must satisfy
type CLIAgent interface {
	// Lifecycle
	Initialize(ctx context.Context, config InstanceConfig) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Destroy(ctx context.Context) error

	// Capabilities
	GetCapabilities() Capabilities

	// Task execution
	ExecuteTask(ctx context.Context, task *CLIAgentTask) (TaskOutput, error)

	// Health check
	HealthCheck(ctx context.Context) error
}

// Capabilities represents what a CLI agent can do
type Capabilities struct {
	SupportsRepoMap     bool     `json:"supports_repo_map"`
	SupportsGitOps      bool     `json:"supports_git_ops"`
	SupportsDiffEditing bool     `json:"supports_diff_editing"`
	SupportsToolUse     bool     `json:"supports_tool_use"`
	SupportsBrowser     bool     `json:"supports_browser"`
	SupportsSandbox     bool     `json:"supports_sandbox"`
	SupportsInterpreter bool     `json:"supports_interpreter"`
	SupportedTools      []string `json:"supported_tools,omitempty"`
}

// DefaultCapabilities returns default capabilities for each agent type
func DefaultCapabilities(agentType CLIAgentType) Capabilities {
	switch agentType {
	case TypeAider:
		return Capabilities{
			SupportsRepoMap:     true,
			SupportsGitOps:      true,
			SupportsDiffEditing: true,
			SupportsToolUse:     false,
			SupportsBrowser:     false,
			SupportsSandbox:     false,
			SupportsInterpreter: false,
		}
	case TypeClaudeCode:
		return Capabilities{
			SupportsRepoMap:     false,
			SupportsGitOps:      true,
			SupportsDiffEditing: false,
			SupportsToolUse:     true,
			SupportsBrowser:     false,
			SupportsSandbox:     false,
			SupportsInterpreter: false,
			SupportedTools:      []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"},
		}
	case TypeCodex:
		return Capabilities{
			SupportsRepoMap:     false,
			SupportsGitOps:      false,
			SupportsDiffEditing: false,
			SupportsToolUse:     false,
			SupportsBrowser:     false,
			SupportsSandbox:     true,
			SupportsInterpreter: true,
		}
	case TypeCline:
		return Capabilities{
			SupportsRepoMap:     false,
			SupportsGitOps:      false,
			SupportsDiffEditing: false,
			SupportsToolUse:     true,
			SupportsBrowser:     true,
			SupportsSandbox:     false,
			SupportsInterpreter: false,
			SupportedTools:      []string{"Browser", "ComputerUse", "Bash"},
		}
	case TypeOpenHands:
		return Capabilities{
			SupportsRepoMap:     false,
			SupportsGitOps:      true,
			SupportsDiffEditing: false,
			SupportsToolUse:     true,
			SupportsBrowser:     false,
			SupportsSandbox:     true,
			SupportsInterpreter: true,
		}
	default:
		return Capabilities{}
	}
}

// DefaultInstanceConfig returns default instance configuration
func DefaultInstanceConfig(agentType CLIAgentType) InstanceConfig {
	return InstanceConfig{
		WorkingDir:   "/tmp/clis",
		Timeout:      5 * time.Minute,
		MaxRetries:   3,
		ProviderType: "default",
		Environment:  make(map[string]string),
	}
}

// HealthCheckResult represents a health check result
type HealthCheckResult struct {
	Healthy   bool                   `json:"healthy"`
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details"`
	Timestamp time.Time              `json:"timestamp"`
	CheckedAt time.Time              `json:"checked_at"`
	Checks    map[string]bool        `json:"checks"`
}

// MessageType represents the type of message
type MessageType string

const (
	MessageTypeStatus   MessageType = "status"
	MessageTypeEvent    MessageType = "event"
	MessageTypeCommand  MessageType = "command"
	MessageTypeResponse MessageType = "response"
)

// Message represents a message in the system
type Message struct {
	ID        string      `json:"id"`
	SessionID string      `json:"session_id"`
	Type      MessageType `json:"type"`
	SourceID  string      `json:"source_id"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}
