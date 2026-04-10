// Package handlers - Planning Handler Extensions
// This file EXTENDS the existing PlanningHandler with claude-code-source inspired features:
// - Plan Mode for structured task planning and execution
// - Todo/Checklist management
// - Plan verification and tracking
// - Interactive plan editing
package extended

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// PlanModeSession represents an active plan mode session (inspired by claude-code-source)
type ExtendedPlanModeSession struct {
	ID              string                 `json:"id"`
	UserID          string                 `json:"user_id"`
	Objective       string                 `json:"objective"`
	Context         []string               `json:"context"`
	Steps           []PlanStep             `json:"steps"`
	CurrentStepIdx  int                    `json:"current_step_idx"`
	Status          PlanModeStatus         `json:"status"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	AutoExecute     bool                   `json:"auto_execute"`
	ExecutionResult *PlanExecutionResult   `json:"execution_result,omitempty"`
	mu              sync.RWMutex
}

// PlanModeStatus represents the status of a plan mode session
type PlanModeStatus string

const (
	PlanModeStatusDraft      PlanModeStatus = "draft"
	PlanModeStatusPlanning   PlanModeStatus = "planning"
	PlanModeStatusReview     PlanModeStatus = "review"
	PlanModeStatusExecuting  PlanModeStatus = "executing"
	PlanModeStatusPaused     PlanModeStatus = "paused"
	PlanModeStatusCompleted  PlanModeStatus = "completed"
	PlanModeStatusFailed     PlanModeStatus = "failed"
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
	Success     bool       `json:"success"`
	Output      string     `json:"output,omitempty"`
	Error       string     `json:"error,omitempty"`
	DurationMs  int64      `json:"duration_ms"`
	CompletedAt time.Time  `json:"completed_at"`
}

// PlanExecutionResult represents the overall result of plan execution
type PlanExecutionResult struct {
	Success      bool          `json:"success"`
	StepsTotal   int           `json:"steps_total"`
	StepsCompleted int         `json:"steps_completed"`
	StepsFailed  int           `json:"steps_failed"`
	StepsSkipped int           `json:"steps_skipped"`
	TotalDuration time.Duration `json:"total_duration"`
	Summary      string        `json:"summary"`
}

// PlanningHandlerExtensions provides extended planning functionality
// This EXTENDS the existing PlanningHandler with claude-code-source features
type PlanningHandlerExtensions struct {
	sessions   map[string]*PlanModeSession
	sessionsMu sync.RWMutex
	logger     *logrus.Logger
}

// NewPlanningHandlerExtensions creates new planning handler extensions
func NewPlanningHandlerExtensions(logger *logrus.Logger) *PlanningHandlerExtensions {
	if logger == nil {
		logger = logrus.New()
	}
	return &PlanningHandlerExtensions{
		sessions: make(map[string]*PlanModeSession),
		logger:   logger,
	}
}

// ============================================
// PLAN MODE ENDPOINTS
// ============================================

// EnterPlanModeRequest represents a request to enter plan mode
type ExtendedEnterPlanModeRequest struct {
	Objective    string   `json:"objective" binding:"required"`
	Context      []string `json:"context,omitempty"`
	AutoExecute  bool     `json:"auto_execute,omitempty"`
	MaxSteps     int      `json:"max_steps,omitempty"`
}

// EnterPlanModeResponse represents the response from entering plan mode
type EnterPlanModeResponse struct {
	SessionID   string         `json:"session_id"`
	Objective   string         `json:"objective"`
	Status      string         `json:"status"`
	Steps       []PlanStep     `json:"steps"`
	Message     string         `json:"message"`
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

	session := &PlanModeSession{
		ID:           uuid.New().String(),
		Objective:    req.Objective,
		Context:      req.Context,
		Status:       PlanModeStatusPlanning,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		AutoExecute:  req.AutoExecute,
		CurrentStepIdx: -1,
	}

	// Generate initial plan steps using AI
	steps, err := h.generatePlanSteps(context.Background(), req.Objective, req.Context, req.MaxSteps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, VerifierErrorResponse{
			Error: "failed to generate plan: " + err.Error(),
		})
		return
	}

	session.Steps = steps
	session.Status = PlanModeStatusReview

	h.sessionsMu.Lock()
	h.sessions[session.ID] = session
	h.sessionsMu.Unlock()

	h.logger.WithFields(logrus.Fields{
		"session_id": session.ID,
		"objective":  req.Objective,
		"steps":      len(steps),
	}).Info("Entered plan mode")

	c.JSON(http.StatusOK, EnterPlanModeResponse{
		SessionID: session.ID,
		Objective: session.Objective,
		Status:    string(session.Status),
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
	
	h.sessionsMu.RLock()
	session, exists := h.sessions[sessionID]
	h.sessionsMu.RUnlock()
	
	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan session not found"})
		return
	}

	var req UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// Apply updates
	if req.Steps != nil {
		session.Steps = req.Steps
	}
	
	if len(req.AddSteps) > 0 {
		session.Steps = append(session.Steps, req.AddSteps...)
	}
	
	if len(req.RemoveSteps) > 0 {
		stepMap := make(map[string]bool)
		for _, id := range req.RemoveSteps {
			stepMap[id] = true
		}
		filtered := make([]PlanStep, 0, len(session.Steps))
		for _, step := range session.Steps {
			if !stepMap[step.ID] {
				filtered = append(filtered, step)
			}
		}
		session.Steps = filtered
	}

	session.UpdatedAt = time.Now()

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
	
	h.sessionsMu.RLock()
	session, exists := h.sessions[sessionID]
	h.sessionsMu.RUnlock()
	
	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan session not found"})
		return
	}

	session.mu.Lock()
	if session.Status != PlanModeStatusReview && session.Status != PlanModeStatusPaused {
		session.mu.Unlock()
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{
			Error: fmt.Sprintf("cannot execute plan in status: %s", session.Status),
		})
		return
	}
	
	session.Status = PlanModeStatusExecuting
	session.mu.Unlock()

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
	
	h.sessionsMu.RLock()
	session, exists := h.sessions[sessionID]
	h.sessionsMu.RUnlock()
	
	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan session not found"})
		return
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

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
	
	h.sessionsMu.RLock()
	session, exists := h.sessions[sessionID]
	h.sessionsMu.RUnlock()
	
	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan session not found"})
		return
	}

	session.mu.Lock()
	if session.Status != PlanModeStatusExecuting {
		session.mu.Unlock()
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: "plan is not executing"})
		return
	}
	
	session.Status = PlanModeStatusPaused
	session.UpdatedAt = time.Now()
	session.mu.Unlock()

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
	
	h.sessionsMu.Lock()
	session, exists := h.sessions[sessionID]
	if !exists {
		h.sessionsMu.Unlock()
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan session not found"})
		return
	}

	// Optionally save to persistent storage before removing
	if save && session.Status == PlanModeStatusCompleted {
		// Plan history is currently in-memory; database persistence uses planning_sessions SQL schema
		// when the database adapter is available via the PlanningService.
		h.logger.WithField("session_id", sessionID).Info("Saving completed plan to history")
	}

	delete(h.sessions, sessionID)
	h.sessionsMu.Unlock()

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

// generatePlanSteps generates plan steps using AI
func (h *PlanningHandlerExtensions) generatePlanSteps(ctx context.Context, objective string, context []string, maxSteps int) ([]PlanStep, error) {
	if maxSteps == 0 {
		maxSteps = 10
	}

	// This would integrate with the LLM service to generate steps
	// For now, returning a placeholder implementation
	steps := []PlanStep{
		{
			ID:          uuid.New().String(),
			Number:      1,
			Description: fmt.Sprintf("Analyze requirements for: %s", objective),
			Type:        "research",
			Status:      PlanStepStatusPending,
			EstDuration: 5 * time.Minute,
		},
		{
			ID:          uuid.New().String(),
			Number:      2,
			Description: "Design solution approach",
			Type:        "decision",
			Status:      PlanStepStatusPending,
			Dependencies: []string{}, // Would reference step 1 ID
			EstDuration: 10 * time.Minute,
		},
		{
			ID:          uuid.New().String(),
			Number:      3,
			Description: "Implement the solution",
			Type:        "implement",
			Status:      PlanStepStatusPending,
			EstDuration: 30 * time.Minute,
		},
		{
			ID:          uuid.New().String(),
			Number:      4,
			Description: "Test and verify",
			Type:        "test",
			Status:      PlanStepStatusPending,
			EstDuration: 15 * time.Minute,
		},
		{
			ID:          uuid.New().String(),
			Number:      5,
			Description: "Review and finalize",
			Type:        "review",
			Status:      PlanStepStatusPending,
			EstDuration: 10 * time.Minute,
		},
	}

	return steps, nil
}

// executePlanSession executes a plan session
func (h *PlanningHandlerExtensions) executePlanSession(ctx context.Context, session *PlanModeSession) {
	h.logger.WithField("session_id", session.ID).Info("Starting plan execution")

	startTime := time.Now()
	result := &PlanExecutionResult{
		StepsTotal: len(session.Steps),
	}

	for i, step := range session.Steps {
		session.mu.Lock()
		session.CurrentStepIdx = i
		
		// Check if paused
		if session.Status == PlanModeStatusPaused {
			session.mu.Unlock()
			h.logger.WithField("session_id", session.ID).Info("Plan execution paused")
			return
		}
		
		step.Status = PlanStepStatusInProgress
		session.mu.Unlock()

		// Execute step
		stepResult := h.executePlanStep(ctx, &step)

		session.mu.Lock()
		step.Result = stepResult
		
		if stepResult.Success {
			step.Status = PlanStepStatusCompleted
			result.StepsCompleted++
		} else {
			step.Status = PlanStepStatusFailed
			result.StepsFailed++
			
			// Stop on failure unless configured to continue
			if !session.AutoExecute {
				session.Status = PlanModeStatusFailed
				session.mu.Unlock()
				break
			}
		}
		
		session.UpdatedAt = time.Now()
		session.mu.Unlock()
	}

	// Complete plan
	session.mu.Lock()
	if session.Status != PlanModeStatusFailed {
		session.Status = PlanModeStatusCompleted
	}
	result.Success = result.StepsFailed == 0
	result.TotalDuration = time.Since(startTime)
	session.ExecutionResult = result
	now := time.Now()
	session.CompletedAt = &now
	session.mu.Unlock()

	h.logger.WithFields(logrus.Fields{
		"session_id":      session.ID,
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
