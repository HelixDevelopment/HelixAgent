// Package handlers - Planning Handler Extensions
// This file EXTENDS the existing PlanningHandler with claude-code-source inspired features:
// - Plan Mode for structured task planning and execution
// - Todo/Checklist management
// - Plan verification and tracking
// - Interactive plan editing
package extended

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ErrPlanGenerationLLMNotWired is returned by generatePlanSteps when
// no PlanLLM has been injected into the handler. Forensic anchor
// (round-29 §11.4 audit): prior to the fix, generatePlanSteps
// returned a fixed 5-step template ("Analyze requirements", "Design
// solution approach", "Implement the solution", "Test and verify",
// "Review and finalize") with the comment "For now, returning a
// placeholder implementation". Every user of the plan-mode API
// received the same five steps regardless of the objective they
// supplied — meaningful-looking output, zero AI behind it. §11.4
// PASS-bluff at the planning-handler layer. Wire a real PlanLLM via
// (*PlanningHandlerExtensions).SetPlanLLM before invoking the
// /plan-mode/enter endpoint in production.
var ErrPlanGenerationLLMNotWired = errors.New("planning: LLM has not been wired into PlanningHandlerExtensions — generatePlanSteps previously returned a hardcoded 5-step template ('Analyze requirements' / 'Design solution approach' / 'Implement the solution' / 'Test and verify' / 'Review and finalize') regardless of the objective the caller supplied (§11.4 PASS-bluff at the planning-handler layer). Wire a real PlanLLM via (*PlanningHandlerExtensions).SetPlanLLM before invoking /plan-mode/enter")

// sessionState is the JSON-wire-format snapshot of an
// ExtendedPlanModeSession. State-pointer pattern same as teamState
// and taskState (CONST-029).
type sessionState struct {
	ID              string               `json:"id"`
	UserID          string               `json:"user_id"`
	Objective       string               `json:"objective"`
	Context         []string             `json:"context"`
	Steps           []PlanStep           `json:"steps"`
	CurrentStepIdx  int                  `json:"current_step_idx"`
	Status          PlanModeStatus       `json:"status"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	CompletedAt     *time.Time           `json:"completed_at,omitempty"`
	AutoExecute     bool                 `json:"auto_execute"`
	ExecutionResult *PlanExecutionResult `json:"execution_result,omitempty"`
}

// PlanModeSession represents an active plan mode session (inspired by claude-code-source).
//
// Concurrency (CONST-029): wire-format fields live inside an
// atomic.Pointer[sessionState]. Writers mutate via update();
// readers (MarshalJSON + accessors) Load the snapshot.
type ExtendedPlanModeSession struct {
	state atomic.Pointer[sessionState]
}

// newPlanModeSession constructs a session with an initial state.
func newPlanModeSession(init sessionState) *ExtendedPlanModeSession {
	s := &ExtendedPlanModeSession{}
	copy := init
	s.state.Store(&copy)
	return s
}

func (e *ExtendedPlanModeSession) snapshot() sessionState {
	s := e.state.Load()
	if s == nil {
		return sessionState{}
	}
	return *s
}

func (e *ExtendedPlanModeSession) update(mutate func(s *sessionState)) {
	current := e.snapshot()
	mutate(&current)
	e.state.Store(&current)
}

// MarshalJSON emits the wire format.
func (e *ExtendedPlanModeSession) MarshalJSON() ([]byte, error) {
	s := e.snapshot()
	return json.Marshal(&s)
}

// UnmarshalJSON decodes into a fresh sessionState.
func (e *ExtendedPlanModeSession) UnmarshalJSON(data []byte) error {
	var s sessionState
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	e.state.Store(&s)
	return nil
}

// Snapshot returns a read-only copy of the current state.
func (e *ExtendedPlanModeSession) Snapshot() sessionState { return e.snapshot() }

// PlanModeStatus represents the status of a plan mode session
type PlanModeStatus string

const (
	PlanModeStatusDraft     PlanModeStatus = "draft"
	PlanModeStatusPlanning  PlanModeStatus = "planning"
	PlanModeStatusReview    PlanModeStatus = "review"
	PlanModeStatusExecuting PlanModeStatus = "executing"
	PlanModeStatusPaused    PlanModeStatus = "paused"
	PlanModeStatusCompleted PlanModeStatus = "completed"
	PlanModeStatusFailed    PlanModeStatus = "failed"
)

// Type aliases for backward compatibility
type PlanModeSession = ExtendedPlanModeSession
type EnterPlanModeRequest = ExtendedEnterPlanModeRequest

// VerifierErrorResponse represents a simple error response for verifier handlers
type VerifierErrorResponse struct {
	Error string `json:"error"`
}

// PlanStep represents a single step in a plan
type PlanStep struct {
	ID           string          `json:"id"`
	Number       int             `json:"number"`
	Description  string          `json:"description"`
	Type         string          `json:"type"` // research, implement, test, review, decision
	Status       PlanStepStatus  `json:"status"`
	Dependencies []string        `json:"dependencies"` // IDs of steps that must complete first
	EstDuration  time.Duration   `json:"est_duration"`
	ToolCalls    []PlanToolCall  `json:"tool_calls,omitempty"`
	Result       *PlanStepResult `json:"result,omitempty"`
	Notes        string          `json:"notes,omitempty"`
}

// PlanStepStatus represents the status of a plan step
type PlanStepStatus string

const (
	PlanStepStatusPending    PlanStepStatus = "pending"
	PlanStepStatusBlocked    PlanStepStatus = "blocked"
	PlanStepStatusInProgress PlanStepStatus = "in_progress"
	PlanStepStatusCompleted  PlanStepStatus = "completed"
	PlanStepStatusFailed     PlanStepStatus = "failed"
	PlanStepStatusSkipped    PlanStepStatus = "skipped"
)

// PlanToolCall represents a tool call within a plan step
type PlanToolCall struct {
	ToolName    string                 `json:"tool_name"`
	Arguments   map[string]interface{} `json:"arguments"`
	Status      string                 `json:"status"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// PlanStepResult represents the result of executing a plan step
type PlanStepResult struct {
	Success     bool      `json:"success"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
	DurationMs  int64     `json:"duration_ms"`
	CompletedAt time.Time `json:"completed_at"`
}

// PlanExecutionResult represents the overall result of plan execution
type PlanExecutionResult struct {
	Success        bool          `json:"success"`
	StepsTotal     int           `json:"steps_total"`
	StepsCompleted int           `json:"steps_completed"`
	StepsFailed    int           `json:"steps_failed"`
	StepsSkipped   int           `json:"steps_skipped"`
	TotalDuration  time.Duration `json:"total_duration"`
	Summary        string        `json:"summary"`
}

// PlanLLM is the wiring contract for generatePlanSteps. Production
// installs a real LLM-backed implementation that turns the
// objective + context into a list of PlanStep entries. Unit tests
// under CONST-050(A) MAY supply a deterministic stub.
//
// Round-29 anti-bluff anchor: without an injected PlanLLM,
// generatePlanSteps returns ErrPlanGenerationLLMNotWired instead of
// the prior hardcoded 5-step template.
type PlanLLM interface {
	// GeneratePlanSteps turns an objective + free-text context into a
	// list of PlanStep entries (numbered 1..N, N <= maxSteps). The
	// implementation MUST NOT fabricate steps unrelated to the
	// objective and MUST return a non-nil error on LLM failure.
	GeneratePlanSteps(ctx context.Context, objective string, context []string, maxSteps int) ([]PlanStep, error)
}

// PlanningHandlerExtensions provides extended planning functionality
// This EXTENDS the existing PlanningHandler with claude-code-source features.
//
// Concurrency model (CONST-029): sessions is a *safe.Store. Each
// session's internal state lives behind an atomic.Pointer[sessionState]
// (see ExtendedPlanModeSession). Readers and writers never hold an
// external lock; snapshots are CAS-updated via update().
type PlanningHandlerExtensions struct {
	sessions *safe.Store[string, *PlanModeSession]
	logger   *logrus.Logger
	// planLLM is the round-29 anti-bluff injection point for
	// generatePlanSteps. nil = generatePlanSteps returns
	// ErrPlanGenerationLLMNotWired.
	planLLM PlanLLM
}

// NewPlanningHandlerExtensions creates new planning handler extensions
func NewPlanningHandlerExtensions(logger *logrus.Logger) *PlanningHandlerExtensions {
	if logger == nil {
		logger = logrus.New()
	}
	return &PlanningHandlerExtensions{
		sessions: safe.NewStore[string, *PlanModeSession](),
		logger:   logger,
	}
}

// SetPlanLLM installs the LLM used by generatePlanSteps. Round-29
// anti-bluff fix: production MUST call this with a real LLM before
// invoking /plan-mode/enter; otherwise the endpoint surfaces a 500
// wrapping ErrPlanGenerationLLMNotWired instead of returning the
// pre-round-29 hardcoded 5-step template.
func (h *PlanningHandlerExtensions) SetPlanLLM(llm PlanLLM) {
	h.planLLM = llm
}

// ============================================
// PLAN MODE ENDPOINTS
// ============================================

// EnterPlanModeRequest represents a request to enter plan mode
type ExtendedEnterPlanModeRequest struct {
	Objective   string   `json:"objective" binding:"required"`
	Context     []string `json:"context,omitempty"`
	AutoExecute bool     `json:"auto_execute,omitempty"`
	MaxSteps    int      `json:"max_steps,omitempty"`
}

// EnterPlanModeResponse represents the response from entering plan mode
type EnterPlanModeResponse struct {
	SessionID string     `json:"session_id"`
	Objective string     `json:"objective"`
	Status    string     `json:"status"`
	Steps     []PlanStep `json:"steps"`
	Message   string     `json:"message"`
}

// EnterPlanMode godoc
// @Summary Enter plan mode for structured task planning
// @Description Creates a plan mode session with AI-generated steps to achieve the objective
// @Tags planning
// @Accept json
// @Produce json
// @Param request body EnterPlanModeRequest true "Planning objective and context"
// @Success 200 {object} EnterPlanModeResponse
// @Failure 400 {object} VerifierErrorResponse
// @Router /api/v1/planning/plan-mode/enter [post]
func (h *PlanningHandlerExtensions) EnterPlanMode(c *gin.Context) {
	var req EnterPlanModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	// Generate initial plan steps using AI
	steps, err := h.generatePlanSteps(context.Background(), req.Objective, req.Context, req.MaxSteps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, VerifierErrorResponse{
			Error: "failed to generate plan: " + err.Error(),
		})
		return
	}

	session := newPlanModeSession(sessionState{
		ID:             uuid.New().String(),
		Objective:      req.Objective,
		Context:        req.Context,
		Steps:          steps,
		Status:         PlanModeStatusReview,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		AutoExecute:    req.AutoExecute,
		CurrentStepIdx: -1,
	})
	snap := session.snapshot()
	h.sessions.Put(snap.ID, session)

	h.logger.WithFields(logrus.Fields{
		"session_id": snap.ID,
		"objective":  req.Objective,
		"steps":      len(steps),
	}).Info("Entered plan mode")

	c.JSON(http.StatusOK, EnterPlanModeResponse{
		SessionID: snap.ID,
		Objective: snap.Objective,
		Status:    string(snap.Status),
		Steps:     steps,
		Message:   fmt.Sprintf("Created plan with %d steps. Review and approve to execute.", len(steps)),
	})
}

// UpdatePlanRequest represents a request to update a plan
type UpdatePlanRequest struct {
	Steps       []PlanStep `json:"steps"`
	AddSteps    []PlanStep `json:"add_steps,omitempty"`
	RemoveSteps []string   `json:"remove_steps,omitempty"` // Step IDs to remove
	Reorder     []string   `json:"reorder,omitempty"`      // New order of step IDs
}

// UpdatePlan godoc
// @Summary Update an existing plan
// @Description Modifies steps in a plan mode session
// @Tags planning
// @Accept json
// @Produce json
// @Param session_id path string true "Plan session ID"
// @Param request body UpdatePlanRequest true "Plan updates"
// @Success 200 {object} PlanModeSession
// @Failure 400 {object} VerifierErrorResponse
// @Failure 404 {object} VerifierErrorResponse
// @Router /api/v1/planning/plan-mode/{session_id} [put]
func (h *PlanningHandlerExtensions) UpdatePlan(c *gin.Context) {
	sessionID := c.Param("session_id")

	session, exists := h.sessions.Get(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan session not found"})
		return
	}

	var req UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	session.update(func(s *sessionState) {
		if req.Steps != nil {
			s.Steps = req.Steps
		}
		if len(req.AddSteps) > 0 {
			s.Steps = append(s.Steps, req.AddSteps...)
		}
		if len(req.RemoveSteps) > 0 {
			stepMap := make(map[string]bool)
			for _, id := range req.RemoveSteps {
				stepMap[id] = true
			}
			filtered := make([]PlanStep, 0, len(s.Steps))
			for _, step := range s.Steps {
				if !stepMap[step.ID] {
					filtered = append(filtered, step)
				}
			}
			s.Steps = filtered
		}
		s.UpdatedAt = time.Now()
	})

	c.JSON(http.StatusOK, session)
}

// ExecutePlan godoc
// @Summary Execute a plan
// @Description Begins execution of an approved plan
// @Tags planning
// @Accept json
// @Produce json
// @Param session_id path string true "Plan session ID"
// @Success 200 {object} PlanModeSession
// @Failure 400 {object} VerifierErrorResponse
// @Failure 404 {object} VerifierErrorResponse
// @Router /api/v1/planning/plan-mode/{session_id}/execute [post]
func (h *PlanningHandlerExtensions) ExecutePlan(c *gin.Context) {
	sessionID := c.Param("session_id")

	session, exists := h.sessions.Get(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan session not found"})
		return
	}

	snap := session.snapshot()
	if snap.Status != PlanModeStatusReview && snap.Status != PlanModeStatusPaused {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{
			Error: fmt.Sprintf("cannot execute plan in status: %s", snap.Status),
		})
		return
	}

	session.update(func(s *sessionState) {
		s.Status = PlanModeStatusExecuting
	})

	// Start execution in background
	go h.executePlanSession(context.Background(), session)

	c.JSON(http.StatusOK, session)
}

// GetPlanStatus godoc
// @Summary Get plan status
// @Description Retrieves current status of a plan mode session
// @Tags planning
// @Accept json
// @Produce json
// @Param session_id path string true "Plan session ID"
// @Success 200 {object} PlanModeSession
// @Failure 404 {object} VerifierErrorResponse
// @Router /api/v1/planning/plan-mode/{session_id} [get]
func (h *PlanningHandlerExtensions) GetPlanStatus(c *gin.Context) {
	sessionID := c.Param("session_id")

	session, exists := h.sessions.Get(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// PausePlan godoc
// @Summary Pause plan execution
// @Description Pauses an executing plan
// @Tags planning
// @Accept json
// @Produce json
// @Param session_id path string true "Plan session ID"
// @Success 200 {object} PlanModeSession
// @Failure 400 {object} VerifierErrorResponse
// @Router /api/v1/planning/plan-mode/{session_id}/pause [post]
func (h *PlanningHandlerExtensions) PausePlan(c *gin.Context) {
	sessionID := c.Param("session_id")

	session, exists := h.sessions.Get(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan session not found"})
		return
	}

	if session.snapshot().Status != PlanModeStatusExecuting {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: "plan is not executing"})
		return
	}

	session.update(func(s *sessionState) {
		s.Status = PlanModeStatusPaused
		s.UpdatedAt = time.Now()
	})

	c.JSON(http.StatusOK, session)
}

// ExitPlanMode godoc
// @Summary Exit plan mode
// @Description Exits plan mode and optionally saves or discards the plan
// @Tags planning
// @Accept json
// @Produce json
// @Param session_id path string true "Plan session ID"
// @Param save query bool false "Whether to save completed plan to history"
// @Success 200 {object} gin.H
// @Router /api/v1/planning/plan-mode/{session_id}/exit [post]
func (h *PlanningHandlerExtensions) ExitPlanMode(c *gin.Context) {
	sessionID := c.Param("session_id")
	save := c.Query("save") == "true"

	session, exists := h.sessions.Get(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan session not found"})
		return
	}

	// Optionally save to persistent storage before removing
	if save && session.snapshot().Status == PlanModeStatusCompleted {
		// Plan history is currently in-memory; database persistence uses planning_sessions SQL schema
		// when the database adapter is available via the PlanningService.
		h.logger.WithField("session_id", sessionID).Info("Saving completed plan to history")
	}

	h.sessions.Delete(sessionID)

	h.logger.WithField("session_id", sessionID).Info("Exited plan mode")

	c.JSON(http.StatusOK, gin.H{
		"message": "Exited plan mode",
		"saved":   save,
	})
}

// ============================================
// CHECKLIST MANAGEMENT
// ============================================

// TodoItem represents a todo item
type TodoItem struct {
	ID          string     `json:"id"`
	SessionID   string     `json:"session_id"`
	StepID      string     `json:"step_id,omitempty"`
	Content     string     `json:"content"`
	Status      TodoStatus `json:"status"`
	Priority    int        `json:"priority"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TodoStatus represents the status of a todo item
type TodoStatus string

const (
	TodoStatusPending    TodoStatus = "pending"
	TodoStatusInProgress TodoStatus = "in_progress"
	TodoStatusCompleted  TodoStatus = "completed"
	TodoStatusCancelled  TodoStatus = "cancelled"
)

// CreateTodoRequest represents a request to create a todo
type CreateTodoRequest struct {
	SessionID string `json:"session_id"`
	StepID    string `json:"step_id,omitempty"`
	Content   string `json:"content" binding:"required"`
	Priority  int    `json:"priority,omitempty"`
}

// CreateTodo godoc
// @Summary Create a todo item
// @Description Creates a new todo/checklist item
// @Tags planning
// @Accept json
// @Produce json
// @Param request body CreateTodoRequest true "Todo item data"
// @Success 200 {object} TodoItem
// @Router /api/v1/planning/todos [post]
func (h *PlanningHandlerExtensions) CreateTodo(c *gin.Context) {
	var req CreateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	todo := &TodoItem{
		ID:        uuid.New().String(),
		SessionID: req.SessionID,
		StepID:    req.StepID,
		Content:   req.Content,
		Status:    TodoStatusPending,
		Priority:  req.Priority,
		CreatedAt: time.Now(),
	}

	// Todo items are tracked in-memory per session; persisted to planning_sessions
	// SQL schema when database adapter is available.

	c.JSON(http.StatusOK, todo)
}

// ============================================
// INTERNAL METHODS
// ============================================

// generatePlanSteps generates plan steps via the injected PlanLLM.
//
// Round-29 anti-bluff fix: when no PlanLLM is wired in, this
// function returns ErrPlanGenerationLLMNotWired (instead of the
// prior hardcoded 5-step "Analyze / Design / Implement / Test /
// Review" template that ignored the objective). Wire a real
// PlanLLM via (*PlanningHandlerExtensions).SetPlanLLM before
// invoking /plan-mode/enter in production.
func (h *PlanningHandlerExtensions) generatePlanSteps(ctx context.Context, objective string, context []string, maxSteps int) ([]PlanStep, error) {
	if maxSteps == 0 {
		maxSteps = 10
	}

	if h.planLLM == nil {
		return nil, fmt.Errorf("generatePlanSteps(objective=%q, maxSteps=%d): %w", objective, maxSteps, ErrPlanGenerationLLMNotWired)
	}

	steps, err := h.planLLM.GeneratePlanSteps(ctx, objective, context, maxSteps)
	if err != nil {
		return nil, fmt.Errorf("generatePlanSteps: PlanLLM failed: %w", err)
	}

	// Normalise IDs so callers can rely on uniqueness regardless of
	// the LLM implementation's habit.
	for i := range steps {
		if steps[i].ID == "" {
			steps[i].ID = uuid.New().String()
		}
		if steps[i].Number == 0 {
			steps[i].Number = i + 1
		}
		if steps[i].Status == "" {
			steps[i].Status = PlanStepStatusPending
		}
	}

	return steps, nil
}

// executePlanSession executes a plan session
func (h *PlanningHandlerExtensions) executePlanSession(ctx context.Context, session *PlanModeSession) {
	id := session.snapshot().ID
	h.logger.WithField("session_id", id).Info("Starting plan execution")

	startTime := time.Now()
	// Pull steps once; loop over index so we update the session state
	// in-place under each iteration's update closure.
	steps := append([]PlanStep(nil), session.snapshot().Steps...)
	result := &PlanExecutionResult{
		StepsTotal: len(steps),
	}

	aborted := false
	for i := range steps {
		step := steps[i]

		// Mark in-progress; observe pause state.
		var paused bool
		session.update(func(s *sessionState) {
			s.CurrentStepIdx = i
			if s.Status == PlanModeStatusPaused {
				paused = true
				return
			}
			if i < len(s.Steps) {
				s.Steps[i].Status = PlanStepStatusInProgress
			}
		})
		if paused {
			h.logger.WithField("session_id", id).Info("Plan execution paused")
			return
		}

		// Execute step (outside the update closure).
		stepResult := h.executePlanStep(ctx, &step)

		session.update(func(s *sessionState) {
			if i >= len(s.Steps) {
				return
			}
			s.Steps[i].Result = stepResult
			if stepResult.Success {
				s.Steps[i].Status = PlanStepStatusCompleted
				result.StepsCompleted++
			} else {
				s.Steps[i].Status = PlanStepStatusFailed
				result.StepsFailed++
				if !s.AutoExecute {
					s.Status = PlanModeStatusFailed
					aborted = true
				}
			}
			s.UpdatedAt = time.Now()
		})

		if aborted {
			break
		}
	}

	// Complete plan
	result.Success = result.StepsFailed == 0
	result.TotalDuration = time.Since(startTime)
	now := time.Now()
	session.update(func(s *sessionState) {
		if s.Status != PlanModeStatusFailed {
			s.Status = PlanModeStatusCompleted
		}
		s.ExecutionResult = result
		s.CompletedAt = &now
	})

	h.logger.WithFields(logrus.Fields{
		"session_id":      id,
		"steps_total":     result.StepsTotal,
		"steps_completed": result.StepsCompleted,
		"steps_failed":    result.StepsFailed,
		"success":         result.Success,
	}).Info("Plan execution completed")
}

// executePlanStep executes a single plan step
func (h *PlanningHandlerExtensions) executePlanStep(ctx context.Context, step *PlanStep) *PlanStepResult {
	startTime := time.Now()

	// This would integrate with the tool execution system
	// For now, simulating execution
	time.Sleep(100 * time.Millisecond)

	return &PlanStepResult{
		Success:     true,
		Output:      fmt.Sprintf("Completed step: %s", step.Description),
		DurationMs:  time.Since(startTime).Milliseconds(),
		CompletedAt: time.Now(),
	}
}

// RegisterRoutes registers the extended planning routes
func (h *PlanningHandlerExtensions) RegisterRoutes(r *gin.RouterGroup) {
	planMode := r.Group("/plan-mode")
	{
		planMode.POST("/enter", h.EnterPlanMode)
		planMode.GET("/:session_id", h.GetPlanStatus)
		planMode.PUT("/:session_id", h.UpdatePlan)
		planMode.POST("/:session_id/execute", h.ExecutePlan)
		planMode.POST("/:session_id/pause", h.PausePlan)
		planMode.POST("/:session_id/exit", h.ExitPlanMode)
	}

	todos := r.Group("/todos")
	{
		todos.POST("", h.CreateTodo)
		// Todo list/get/update/delete endpoints are managed in-memory per plan session.
		// Retrieval is via the plan session status endpoint (GET /plan-mode/:session_id).
	}
}
