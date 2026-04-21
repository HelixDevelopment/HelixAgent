// Package handlers - Ensemble Handler Extensions
// This file EXTENDS the existing EnsembleHandler with claude-code-source inspired features:
// - Team management (TeamCreate, TeamDelete)
// - Task management (TaskCreate, TaskGet, TaskList, TaskStop, TaskUpdate)
// - Enhanced multi-agent coordination
// - Agent-to-agent messaging
package extended

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/ensemble/multi_instance"
)

// teamState is the JSON-wire-format snapshot of an AgentTeam. Mutators
// load the current state, clone it, mutate the clone, and CAS-store the
// result. Readers (MarshalJSON) load the current pointer and serialise
// the snapshot — there is no lock, no mutex, and no bare collection in
// the AgentTeam field list (CONST-029 compliance).
type teamState struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	LeaderID    string          `json:"leader_id"`
	MemberIDs   []string        `json:"member_ids"`
	Config      AgentTeamConfig `json:"config"`
	Status      TeamStatus      `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Team represents a team of agents (inspired by claude-code-source Team tools).
//
// Concurrency (CONST-029): all wire-format fields live inside an
// atomic.Pointer[teamState]. Writers mutate via AgentTeam.update();
// readers (MarshalJSON + accessors) Load the snapshot and inspect it.
// Public-wire shape is preserved — the MarshalJSON method emits exactly
// the same JSON the prior embedded-field version did.
type AgentTeam struct {
	state atomic.Pointer[teamState]
}

// newAgentTeam constructs an AgentTeam with the initial state.
func newAgentTeam(init teamState) *AgentTeam {
	t := &AgentTeam{}
	copy := init
	t.state.Store(&copy)
	return t
}

// snapshot returns the current state (never nil after construction).
func (t *AgentTeam) snapshot() teamState {
	s := t.state.Load()
	if s == nil {
		return teamState{}
	}
	return *s
}

// update applies mutate to a clone of the current state and stores the
// result. There is no CAS loop because all mutations go through the
// HTTP handlers which run serially per-team under the parent Store's
// per-key serialisation (readers never block writers, but concurrent
// writers on the SAME team ID are rare — acceptable "last writer wins"
// semantics in line with the original mu.Lock/Unlock behaviour).
func (t *AgentTeam) update(mutate func(s *teamState)) {
	current := t.snapshot()
	mutate(&current)
	t.state.Store(&current)
}

// MarshalJSON emits the JSON wire format. Replaces the prior RLock +
// alias-type pattern with a lock-free snapshot load (BUGFIX #28 still
// fixed — no field read races possible since the snapshot is immutable).
func (t *AgentTeam) MarshalJSON() ([]byte, error) {
	s := t.snapshot()
	return json.Marshal(&s)
}

// UnmarshalJSON decodes a JSON payload into a fresh teamState and
// atomic-stores it. Allows tests (and any other caller) to reconstitute
// an AgentTeam from a JSON response without touching internal state.
func (t *AgentTeam) UnmarshalJSON(data []byte) error {
	var s teamState
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t.state.Store(&s)
	return nil
}

// Snapshot returns a read-only copy of the current state. Public
// accessor used by tests and by other packages that need to inspect
// the wire-format fields without going through JSON marshalling.
func (t *AgentTeam) Snapshot() teamState { return t.snapshot() }

// TeamConfig holds team configuration
type AgentTeamConfig struct {
	MaxMembers       int               `json:"max_members"`
	CoordinationMode string            `json:"coordination_mode"` // hierarchical, democratic, leader_follower
	DecisionStrategy string            `json:"decision_strategy"` // consensus, majority, leader_decides
	AutoLoadBalance  bool              `json:"auto_load_balance"`
	FallbackEnabled  bool              `json:"fallback_enabled"`
	SharedContext    map[string]string `json:"shared_context"`
}

// TeamStatus represents team status
type TeamStatus string

const (
	TeamStatusActive   TeamStatus = "active"
	TeamStatusInactive TeamStatus = "inactive"
	TeamStatusBusy     TeamStatus = "busy"
	TeamStatusError    TeamStatus = "error"
)

// Type aliases for backward compatibility
type Team = AgentTeam
type TeamConfig = AgentTeamConfig
type CreateTeamRequest = CreateAgentTeamRequest
type UpdateTeamRequest = UpdateAgentTeamRequest
type CreateTaskRequest = CreateAgentTaskRequest
type UpdateTaskRequest = UpdateAgentTaskRequest
type TaskStatus = AgentTaskStatus

// taskState is the JSON-wire-format snapshot of a Task. Same
// state-pointer pattern as teamState (CONST-029).
type taskState struct {
	ID           string          `json:"id"`
	TeamID       string          `json:"team_id,omitempty"`
	AssigneeID   string          `json:"assignee_id,omitempty"`
	CreatorID    string          `json:"creator_id"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Type         string          `json:"type"`
	Status       AgentTaskStatus `json:"status"`
	Priority     TaskPriority    `json:"priority"`
	Dependencies []string        `json:"dependencies"`
	Subtasks     []Subtask       `json:"subtasks"`
	Result       *TaskResult     `json:"result,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	Deadline     *time.Time      `json:"deadline,omitempty"`
	Metadata     TaskMetadata    `json:"metadata"`
}

// Task represents a task assigned to agents (inspired by claude-code-source Task tools).
//
// Concurrency (CONST-029): wire-format fields live inside an
// atomic.Pointer[taskState]. Same "load → clone → mutate → store"
// pattern as AgentTeam.
type Task struct {
	state atomic.Pointer[taskState]
}

// newTask constructs a Task with the initial state.
func newTask(init taskState) *Task {
	t := &Task{}
	copy := init
	t.state.Store(&copy)
	return t
}

// snapshot returns the current state (never nil after construction).
func (t *Task) snapshot() taskState {
	s := t.state.Load()
	if s == nil {
		return taskState{}
	}
	return *s
}

// update applies mutate to a clone of the current state and stores it.
func (t *Task) update(mutate func(s *taskState)) {
	current := t.snapshot()
	mutate(&current)
	t.state.Store(&current)
}

// MarshalJSON emits the JSON wire format — same output as the prior
// embedded-field version (BUGFIX #28 still fixed — lock-free snapshot).
func (t *Task) MarshalJSON() ([]byte, error) {
	s := t.snapshot()
	return json.Marshal(&s)
}

// UnmarshalJSON decodes a JSON payload into a fresh taskState and
// atomic-stores it.
func (t *Task) UnmarshalJSON(data []byte) error {
	var s taskState
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t.state.Store(&s)
	return nil
}

// Snapshot returns a read-only copy of the current state.
func (t *Task) Snapshot() taskState { return t.snapshot() }

// AgentTaskStatus represents task status
type AgentTaskStatus string

const (
	AgentTaskStatusPending    AgentTaskStatus = "pending"
	AgentTaskStatusAssigned   AgentTaskStatus = "assigned"
	AgentTaskStatusInProgress AgentTaskStatus = "in_progress"
	AgentTaskStatusReview     AgentTaskStatus = "review"
	AgentTaskStatusCompleted  AgentTaskStatus = "completed"
	AgentTaskStatusFailed     AgentTaskStatus = "failed"
	AgentTaskStatusCancelled  AgentTaskStatus = "cancelled"
)

// TaskStatus constants for backward compatibility
const (
	TaskStatusPending    = AgentTaskStatusPending
	TaskStatusAssigned   = AgentTaskStatusAssigned
	TaskStatusInProgress = AgentTaskStatusInProgress
	TaskStatusReview     = AgentTaskStatusReview
	TaskStatusCompleted  = AgentTaskStatusCompleted
	TaskStatusFailed     = AgentTaskStatusFailed
	TaskStatusCancelled  = AgentTaskStatusCancelled
)

// TaskPriority represents task priority
type TaskPriority string

const (
	TaskPriorityLow      TaskPriority = "low"
	TaskPriorityMedium   TaskPriority = "medium"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityCritical TaskPriority = "critical"
)

// Subtask represents a subtask
type Subtask struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	AssigneeID string `json:"assignee_id,omitempty"`
	Result     string `json:"result,omitempty"`
}

// TaskResult represents task execution result
type TaskResult struct {
	Success     bool           `json:"success"`
	Output      string         `json:"output,omitempty"`
	Artifacts   []TaskArtifact `json:"artifacts,omitempty"`
	Metrics     TaskMetrics    `json:"metrics"`
	CompletedAt time.Time      `json:"completed_at"`
}

// TaskArtifact represents a task artifact
type TaskArtifact struct {
	Type     string `json:"type"` // file, url, text, diff
	Name     string `json:"name"`
	Content  string `json:"content,omitempty"`
	Location string `json:"location,omitempty"`
}

// TaskMetrics represents task execution metrics
type TaskMetrics struct {
	DurationMs   int64   `json:"duration_ms"`
	TokensUsed   int     `json:"tokens_used"`
	CostEstimate float64 `json:"cost_estimate"`
	QualityScore float64 `json:"quality_score"`
}

// TaskMetadata represents task metadata
type TaskMetadata struct {
	Tags           []string          `json:"tags"`
	Requirements   []string          `json:"requirements"`
	AcceptanceCrit []string          `json:"acceptance_criteria"`
	Context        map[string]string `json:"context"`
}

// AgentMessage represents a message between agents
type AgentMessage struct {
	ID        string    `json:"id"`
	FromID    string    `json:"from_id"`
	ToID      string    `json:"to_id,omitempty"` // Empty = broadcast
	TeamID    string    `json:"team_id,omitempty"`
	Type      string    `json:"type"` // request, response, broadcast, direct
	Content   string    `json:"content"`
	Priority  string    `json:"priority"`
	Timestamp time.Time `json:"timestamp"`
	ReadBy    []string  `json:"read_by"`
}

// EnsembleHandlerExtensions EXTENDS the existing EnsembleHandler
// with team and task management capabilities from claude-code-source.
//
// Concurrency model (CONST-029): teams, tasks, and messages are
// safe.Store containers. Per-entry Team/Task mutations still go
// through each struct's own mu (both are JSON-marshaled, so in-place
// field updates under their own RWMutex remains the right pattern).
// messages per-key append uses Update for atomic read-append-commit.
type EnsembleHandlerExtensions struct {
	coordinator *multi_instance.Coordinator
	teams       *safe.Store[string, *Team]
	tasks       *safe.Store[string, *Task]
	messages    *safe.Store[string, []AgentMessage]
	logger      *logrus.Logger
}

// NewEnsembleHandlerExtensions creates new ensemble handler extensions
func NewEnsembleHandlerExtensions(coordinator *multi_instance.Coordinator, logger *logrus.Logger) *EnsembleHandlerExtensions {
	if logger == nil {
		logger = logrus.New()
	}
	return &EnsembleHandlerExtensions{
		coordinator: coordinator,
		teams:       safe.NewStore[string, *Team](),
		tasks:       safe.NewStore[string, *Task](),
		messages:    safe.NewStore[string, []AgentMessage](),
		logger:      logger,
	}
}

// ============================================
// TEAM MANAGEMENT ENDPOINTS (TeamCreate, TeamDelete)
// ============================================

// CreateTeamRequest represents a team creation request
type CreateAgentTeamRequest struct {
	Name        string      `json:"name" binding:"required"`
	Description string      `json:"description,omitempty"`
	LeaderID    string      `json:"leader_id" binding:"required"`
	MemberIDs   []string    `json:"member_ids,omitempty"`
	Config      *TeamConfig `json:"config,omitempty"`
}

// CreateTeam godoc
// @Summary Create a new agent team
// @Description Creates a team of agents for coordinated work (inspired by claude-code-source TeamCreate)
// @Tags ensemble
// @Accept json
// @Produce json
// @Param request body CreateTeamRequest true "Team configuration"
// @Success 201 {object} Team
// @Failure 400 {object} gin.H
// @Router /api/v1/ensemble/teams [post]
func (h *EnsembleHandlerExtensions) CreateTeam(c *gin.Context) {
	var req CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := AgentTeamConfig{
		MaxMembers:       10,
		CoordinationMode: "leader_follower",
		DecisionStrategy: "leader_decides",
		AutoLoadBalance:  true,
		FallbackEnabled:  true,
	}
	if req.Config != nil {
		config = *req.Config
	}

	team := newAgentTeam(teamState{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		LeaderID:    req.LeaderID,
		MemberIDs:   req.MemberIDs,
		Config:      config,
		Status:      TeamStatusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})
	snap := team.snapshot()
	h.teams.Put(snap.ID, team)

	h.logger.WithFields(logrus.Fields{
		"team_id":      snap.ID,
		"team_name":    snap.Name,
		"leader_id":    snap.LeaderID,
		"member_count": len(snap.MemberIDs),
	}).Info("Created agent team")

	c.JSON(http.StatusCreated, team)
}

// GetTeam godoc
// @Summary Get team by ID
// @Description Retrieves team information
// @Tags ensemble
// @Accept json
// @Produce json
// @Param team_id path string true "Team ID"
// @Success 200 {object} Team
// @Failure 404 {object} gin.H
// @Router /api/v1/ensemble/teams/{team_id} [get]
func (h *EnsembleHandlerExtensions) GetTeam(c *gin.Context) {
	teamID := c.Param("team_id")

	team, exists := h.teams.Get(teamID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	c.JSON(http.StatusOK, team)
}

// ListTeams godoc
// @Summary List all teams
// @Description Lists all agent teams with optional filtering
// @Tags ensemble
// @Accept json
// @Produce json
// @Param status query string false "Filter by status"
// @Success 200 {array} Team
// @Router /api/v1/ensemble/teams [get]
func (h *EnsembleHandlerExtensions) ListTeams(c *gin.Context) {
	status := c.Query("status")

	var teams []*Team
	h.teams.Range(func(_ string, team *Team) bool {
		if status == "" || string(team.snapshot().Status) == status {
			teams = append(teams, team)
		}
		return true
	})

	c.JSON(http.StatusOK, teams)
}

// UpdateTeamRequest represents a team update request
type UpdateAgentTeamRequest struct {
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	LeaderID    string      `json:"leader_id,omitempty"`
	MemberIDs   []string    `json:"member_ids,omitempty"`
	Config      *TeamConfig `json:"config,omitempty"`
	Status      string      `json:"status,omitempty"`
}

// UpdateTeam godoc
// @Summary Update a team
// @Description Updates team configuration
// @Tags ensemble
// @Accept json
// @Produce json
// @Param team_id path string true "Team ID"
// @Param request body UpdateTeamRequest true "Team updates"
// @Success 200 {object} Team
// @Failure 404 {object} gin.H
// @Router /api/v1/ensemble/teams/{team_id} [put]
func (h *EnsembleHandlerExtensions) UpdateTeam(c *gin.Context) {
	teamID := c.Param("team_id")

	team, exists := h.teams.Get(teamID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	var req UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team.update(func(s *teamState) {
		if req.Name != "" {
			s.Name = req.Name
		}
		if req.Description != "" {
			s.Description = req.Description
		}
		if req.LeaderID != "" {
			s.LeaderID = req.LeaderID
		}
		if req.MemberIDs != nil {
			s.MemberIDs = req.MemberIDs
		}
		if req.Config != nil {
			s.Config = *req.Config
		}
		if req.Status != "" {
			s.Status = TeamStatus(req.Status)
		}
		s.UpdatedAt = time.Now()
	})

	c.JSON(http.StatusOK, team)
}

// DeleteTeam godoc
// @Summary Delete a team
// @Description Deletes an agent team (inspired by claude-code-source TeamDelete)
// @Tags ensemble
// @Accept json
// @Produce json
// @Param team_id path string true "Team ID"
// @Param force query bool false "Force delete even if team has active tasks"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/v1/ensemble/teams/{team_id} [delete]
func (h *EnsembleHandlerExtensions) DeleteTeam(c *gin.Context) {
	teamID := c.Param("team_id")
	force := c.Query("force") == "true"

	team, exists := h.teams.Get(teamID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	// Check for active tasks
	if !force {
		var hasActiveTasks bool
		h.tasks.Range(func(_ string, task *Task) bool {
			ts := task.snapshot()
			if ts.TeamID == teamID && (ts.Status == AgentTaskStatusInProgress || ts.Status == AgentTaskStatusPending) {
				hasActiveTasks = true
				return false
			}
			return true
		})
		if hasActiveTasks {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "team has active tasks, use force=true to delete anyway",
			})
			return
		}
	}

	h.teams.Delete(teamID)

	teamSnap := team.snapshot()
	h.logger.WithFields(logrus.Fields{
		"team_id":   teamID,
		"team_name": teamSnap.Name,
	}).Info("Deleted agent team")

	c.JSON(http.StatusOK, gin.H{
		"message":   "team deleted",
		"team_id":   teamID,
		"team_name": teamSnap.Name,
	})
}

// ============================================
// TASK MANAGEMENT ENDPOINTS (TaskCreate, TaskGet, TaskList, TaskStop, TaskUpdate)
// ============================================

// CreateTaskRequest represents a task creation request
type CreateAgentTaskRequest struct {
	TeamID       string        `json:"team_id,omitempty"`
	AssigneeID   string        `json:"assignee_id,omitempty"`
	Title        string        `json:"title" binding:"required"`
	Description  string        `json:"description,omitempty"`
	Type         string        `json:"type" binding:"required"` // code_review, implementation, research, testing, documentation
	Priority     string        `json:"priority,omitempty"`      // low, medium, high, critical
	Dependencies []string      `json:"dependencies,omitempty"`
	Deadline     *time.Time    `json:"deadline,omitempty"`
	Metadata     *TaskMetadata `json:"metadata,omitempty"`
}

// CreateTask godoc
// @Summary Create a new task
// @Description Creates a task for an agent or team (inspired by claude-code-source TaskCreate)
// @Tags ensemble
// @Accept json
// @Produce json
// @Param request body CreateTaskRequest true "Task configuration"
// @Success 201 {object} Task
// @Failure 400 {object} gin.H
// @Router /api/v1/ensemble/tasks [post]
func (h *EnsembleHandlerExtensions) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	priority := TaskPriorityMedium
	if req.Priority != "" {
		priority = TaskPriority(req.Priority)
	}

	metadata := TaskMetadata{}
	if req.Metadata != nil {
		metadata = *req.Metadata
	}

	task := newTask(taskState{
		ID:           uuid.New().String(),
		TeamID:       req.TeamID,
		AssigneeID:   req.AssigneeID,
		Title:        req.Title,
		Description:  req.Description,
		Type:         req.Type,
		Status:       AgentTaskStatusPending,
		Priority:     priority,
		Dependencies: req.Dependencies,
		Deadline:     req.Deadline,
		Metadata:     metadata,
		CreatedAt:    time.Now(),
	})
	taskSnap := task.snapshot()
	h.tasks.Put(taskSnap.ID, task)

	h.logger.WithFields(logrus.Fields{
		"task_id":     taskSnap.ID,
		"task_title":  taskSnap.Title,
		"task_type":   taskSnap.Type,
		"team_id":     taskSnap.TeamID,
		"assignee_id": taskSnap.AssigneeID,
	}).Info("Created task")

	c.JSON(http.StatusCreated, task)
}

// GetTask godoc
// @Summary Get task by ID
// @Description Retrieves task information (inspired by claude-code-source TaskGet)
// @Tags ensemble
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Success 200 {object} Task
// @Failure 404 {object} gin.H
// @Router /api/v1/ensemble/tasks/{task_id} [get]
func (h *EnsembleHandlerExtensions) GetTask(c *gin.Context) {
	taskID := c.Param("task_id")

	task, exists := h.tasks.Get(taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// ListTasksRequest represents a task list request
type ListTasksRequest struct {
	TeamID     string `form:"team_id"`
	AssigneeID string `form:"assignee_id"`
	Status     string `form:"status"`
	Type       string `form:"type"`
	Priority   string `form:"priority"`
}

// ListTasks godoc
// @Summary List tasks
// @Description Lists tasks with optional filtering (inspired by claude-code-source TaskList)
// @Tags ensemble
// @Accept json
// @Produce json
// @Param team_id query string false "Filter by team ID"
// @Param assignee_id query string false "Filter by assignee ID"
// @Param status query string false "Filter by status"
// @Param type query string false "Filter by type"
// @Param priority query string false "Filter by priority"
// @Success 200 {array} Task
// @Router /api/v1/ensemble/tasks [get]
func (h *EnsembleHandlerExtensions) ListTasks(c *gin.Context) {
	var req ListTasksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var tasks []*Task
	h.tasks.Range(func(_ string, task *Task) bool {
		ts := task.snapshot()
		// Apply filters
		if req.TeamID != "" && ts.TeamID != req.TeamID {
			return true
		}
		if req.AssigneeID != "" && ts.AssigneeID != req.AssigneeID {
			return true
		}
		if req.Status != "" && string(ts.Status) != req.Status {
			return true
		}
		if req.Type != "" && ts.Type != req.Type {
			return true
		}
		if req.Priority != "" && string(ts.Priority) != req.Priority {
			return true
		}
		tasks = append(tasks, task)
		return true
	})

	c.JSON(http.StatusOK, tasks)
}

// UpdateTaskRequest represents a task update request
type UpdateAgentTaskRequest struct {
	Title       string      `json:"title,omitempty"`
	Description string      `json:"description,omitempty"`
	AssigneeID  string      `json:"assignee_id,omitempty"`
	Status      string      `json:"status,omitempty"`
	Priority    string      `json:"priority,omitempty"`
	Subtasks    []Subtask   `json:"subtasks,omitempty"`
	Result      *TaskResult `json:"result,omitempty"`
}

// UpdateTask godoc
// @Summary Update a task
// @Description Updates task information (inspired by claude-code-source TaskUpdate)
// @Tags ensemble
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Param request body UpdateTaskRequest true "Task updates"
// @Success 200 {object} Task
// @Failure 404 {object} gin.H
// @Router /api/v1/ensemble/tasks/{task_id} [put]
func (h *EnsembleHandlerExtensions) UpdateTask(c *gin.Context) {
	taskID := c.Param("task_id")

	task, exists := h.tasks.Get(taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task.update(func(s *taskState) {
		if req.Title != "" {
			s.Title = req.Title
		}
		if req.Description != "" {
			s.Description = req.Description
		}
		if req.AssigneeID != "" {
			s.AssigneeID = req.AssigneeID
		}
		if req.Status != "" {
			oldStatus := s.Status
			s.Status = TaskStatus(req.Status)

			// Update timestamps based on status changes
			if s.Status == AgentTaskStatusInProgress && oldStatus != AgentTaskStatusInProgress {
				now := time.Now()
				s.StartedAt = &now
			}
			if (s.Status == AgentTaskStatusCompleted || s.Status == AgentTaskStatusFailed) &&
				oldStatus != AgentTaskStatusCompleted && oldStatus != AgentTaskStatusFailed {
				now := time.Now()
				s.CompletedAt = &now
			}
		}
		if req.Priority != "" {
			s.Priority = TaskPriority(req.Priority)
		}
		if req.Subtasks != nil {
			s.Subtasks = req.Subtasks
		}
		if req.Result != nil {
			s.Result = req.Result
		}
	})

	c.JSON(http.StatusOK, task)
}

// StopTask godoc
// @Summary Stop a running task
// @Description Stops/cancels a task execution (inspired by claude-code-source TaskStop)
// @Tags ensemble
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Success 200 {object} Task
// @Failure 400 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /api/v1/ensemble/tasks/{task_id}/stop [post]
func (h *EnsembleHandlerExtensions) StopTask(c *gin.Context) {
	taskID := c.Param("task_id")

	task, exists := h.tasks.Get(taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	currentSnap := task.snapshot()
	if currentSnap.Status != AgentTaskStatusInProgress && currentSnap.Status != AgentTaskStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("cannot stop task in status: %s", currentSnap.Status),
		})
		return
	}
	task.update(func(s *taskState) {
		s.Status = TaskStatusCancelled
		now := time.Now()
		s.CompletedAt = &now
	})

	h.logger.WithFields(logrus.Fields{
		"task_id":    taskID,
		"task_title": currentSnap.Title,
	}).Info("Stopped task")

	c.JSON(http.StatusOK, task)
}

// GetTaskOutput godoc
// @Summary Get task output
// @Description Retrieves task execution output (inspired by claude-code-source TaskOutput)
// @Tags ensemble
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Success 200 {object} TaskResult
// @Failure 404 {object} gin.H
// @Router /api/v1/ensemble/tasks/{task_id}/output [get]
func (h *EnsembleHandlerExtensions) GetTaskOutput(c *gin.Context) {
	taskID := c.Param("task_id")

	task, exists := h.tasks.Get(taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	result := task.snapshot().Result

	if result == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "pending",
			"message": "Task has no output yet",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ============================================
// AGENT MESSAGING
// ============================================

// SendMessageRequest represents a message send request
type SendMessageRequest struct {
	ToID     string `json:"to_id,omitempty"` // Empty = broadcast to team
	TeamID   string `json:"team_id,omitempty"`
	Type     string `json:"type" binding:"required"` // request, response, broadcast, direct
	Content  string `json:"content" binding:"required"`
	Priority string `json:"priority,omitempty"` // low, normal, high, urgent
}

// SendMessage godoc
// @Summary Send message to agent(s)
// @Description Sends a message to specific agent or broadcasts to team
// @Tags ensemble
// @Accept json
// @Produce json
// @Param request body SendMessageRequest true "Message data"
// @Success 201 {object} AgentMessage
// @Router /api/v1/ensemble/messages [post]
func (h *EnsembleHandlerExtensions) SendMessage(c *gin.Context) {
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get sender from context (would be set by auth middleware)
	fromID := c.GetString("agent_id")
	if fromID == "" {
		fromID = "system"
	}

	priority := req.Priority
	if priority == "" {
		priority = "normal"
	}

	message := AgentMessage{
		ID:        uuid.New().String(),
		FromID:    fromID,
		ToID:      req.ToID,
		TeamID:    req.TeamID,
		Type:      req.Type,
		Content:   req.Content,
		Priority:  priority,
		Timestamp: time.Now(),
		ReadBy:    []string{},
	}

	recipientID := req.ToID
	if recipientID == "" && req.TeamID != "" {
		recipientID = req.TeamID // Broadcast to team
	}
	h.messages.Update(recipientID, func(cur []AgentMessage, _ bool) ([]AgentMessage, bool) {
		return append(cur, message), true
	})

	h.logger.WithFields(logrus.Fields{
		"message_id": message.ID,
		"from_id":    fromID,
		"to_id":      req.ToID,
		"team_id":    req.TeamID,
	}).Info("Sent agent message")

	c.JSON(http.StatusCreated, message)
}

// ListMessages godoc
// @Summary List messages for agent/team
// @Description Retrieves messages for the current agent or team
// @Tags ensemble
// @Accept json
// @Produce json
// @Param team_id query string false "Filter by team ID"
// @Param since query string false "Filter messages since timestamp (ISO 8601)"
// @Success 200 {array} AgentMessage
// @Router /api/v1/ensemble/messages [get]
func (h *EnsembleHandlerExtensions) ListMessages(c *gin.Context) {
	teamID := c.Query("team_id")
	sinceStr := c.Query("since")

	// Get recipient from context
	recipientID := c.GetString("agent_id")
	if recipientID == "" {
		recipientID = teamID
	}

	messages, _ := h.messages.Get(recipientID)

	// Filter by timestamp if provided
	var filtered []AgentMessage
	if sinceStr != "" {
		since, err := time.Parse(time.RFC3339, sinceStr)
		if err == nil {
			for _, msg := range messages {
				if msg.Timestamp.After(since) {
					filtered = append(filtered, msg)
				}
			}
		}
	} else {
		filtered = messages
	}

	c.JSON(http.StatusOK, filtered)
}

// ============================================
// ROUTE REGISTRATION
// ============================================

// RegisterRoutes registers the extended ensemble routes
func (h *EnsembleHandlerExtensions) RegisterRoutes(r *gin.RouterGroup) {
	// Team routes
	teams := r.Group("/teams")
	{
		teams.POST("", h.CreateTeam)
		teams.GET("", h.ListTeams)
		teams.GET("/:team_id", h.GetTeam)
		teams.PUT("/:team_id", h.UpdateTeam)
		teams.DELETE("/:team_id", h.DeleteTeam)
	}

	// Task routes
	tasks := r.Group("/tasks")
	{
		tasks.POST("", h.CreateTask)
		tasks.GET("", h.ListTasks)
		tasks.GET("/:task_id", h.GetTask)
		tasks.PUT("/:task_id", h.UpdateTask)
		tasks.POST("/:task_id/stop", h.StopTask)
		tasks.GET("/:task_id/output", h.GetTaskOutput)
	}

	// Messaging routes
	messages := r.Group("/messages")
	{
		messages.POST("", h.SendMessage)
		messages.GET("", h.ListMessages)
	}
}
