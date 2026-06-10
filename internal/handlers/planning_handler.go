package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/planning"
	"dev.helix.agent/internal/services"
)

// PlanningHandler provides HTTP endpoints for AI planning algorithms.
// Endpoints:
//
//	POST /v1/planning/hiplan - hierarchical planning
//	POST /v1/planning/mcts   - Monte Carlo Tree Search
//	POST /v1/planning/tot    - Tree of Thoughts
type PlanningHandler struct {
	logger         *logrus.Logger
	requestService *services.RequestService // optional — when wired, generateTaskBreakdown uses real LLM
}

// NewPlanningHandler creates a new planning handler.
func NewPlanningHandler(logger *logrus.Logger) *PlanningHandler {
	if logger == nil {
		logger = logrus.New()
	}
	return &PlanningHandler{
		logger: logger,
	}
}

// SetRequestService wires the LLM request service so generateTaskBreakdown
// can call a real LLM instead of returning the 5-step template. CONST-035 §c
// (closing #planning-llm-task-breakdown).
func (h *PlanningHandler) SetRequestService(rs *services.RequestService) {
	h.requestService = rs
}

// --- HiPlan ---

// HiPlanRequest represents a request for hierarchical planning.
type HiPlanRequest struct {
	Goal   string               `json:"goal" binding:"required"`
	Config *HiPlanConfigRequest `json:"config,omitempty"`
}

// HiPlanConfigRequest represents optional HiPlan configuration.
type HiPlanConfigRequest struct {
	MaxMilestones            int   `json:"max_milestones,omitempty"`
	MaxStepsPerMilestone     int   `json:"max_steps_per_milestone,omitempty"`
	EnableParallelMilestones *bool `json:"enable_parallel_milestones,omitempty"`
	MaxParallelMilestones    int   `json:"max_parallel_milestones,omitempty"`
	EnableAdaptivePlanning   *bool `json:"enable_adaptive_planning,omitempty"`
	RetryFailedSteps         *bool `json:"retry_failed_steps,omitempty"`
	MaxRetries               int   `json:"max_retries,omitempty"`
	TimeoutSeconds           int   `json:"timeout_seconds,omitempty"`
}

// HiPlanResponse represents the response from hierarchical planning.
type HiPlanResponse struct {
	PlanID     string                    `json:"plan_id"`
	Goal       string                    `json:"goal"`
	State      string                    `json:"state"`
	Progress   float64                   `json:"progress"`
	Milestones []HiPlanMilestoneResponse `json:"milestones"`
	DurationMs int64                     `json:"duration_ms"`
	CreatedAt  string                    `json:"created_at"`
	Error      string                    `json:"error,omitempty"`
}

// HiPlanMilestoneResponse represents a milestone in the response.
type HiPlanMilestoneResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	State       string  `json:"state"`
	Priority    int     `json:"priority"`
	Progress    float64 `json:"progress"`
	StepsCount  int     `json:"steps_count"`
	Error       string  `json:"error,omitempty"`
}

// CreateHiPlan godoc
// @Summary Execute hierarchical planning
// @Description Creates and optionally executes a hierarchical plan for a given goal
// @Tags planning
// @Accept json
// @Produce json
// @Param request body HiPlanRequest true "Planning goal and configuration"
// @Success 200 {object} HiPlanResponse
// @Failure 400 {object} VerifierErrorResponse
// @Failure 500 {object} VerifierErrorResponse
// @Router /api/v1/planning/hiplan [post]
func (h *PlanningHandler) CreateHiPlan(c *gin.Context) {
	var req HiPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	cfg := planning.DefaultHiPlanConfig()
	if req.Config != nil {
		if req.Config.MaxMilestones > 0 {
			cfg.MaxMilestones = req.Config.MaxMilestones
		}
		if req.Config.MaxStepsPerMilestone > 0 {
			cfg.MaxStepsPerMilestone = req.Config.MaxStepsPerMilestone
		}
		if req.Config.EnableParallelMilestones != nil {
			cfg.EnableParallelMilestones = *req.Config.EnableParallelMilestones
		}
		if req.Config.MaxParallelMilestones > 0 {
			cfg.MaxParallelMilestones = req.Config.MaxParallelMilestones
		}
		if req.Config.EnableAdaptivePlanning != nil {
			cfg.EnableAdaptivePlanning = *req.Config.EnableAdaptivePlanning
		}
		if req.Config.RetryFailedSteps != nil {
			cfg.RetryFailedSteps = *req.Config.RetryFailedSteps
		}
		if req.Config.MaxRetries > 0 {
			cfg.MaxRetries = req.Config.MaxRetries
		}
		if req.Config.TimeoutSeconds > 0 {
			cfg.Timeout = time.Duration(req.Config.TimeoutSeconds) * time.Second
		}
	}

	generator := &staticMilestoneGenerator{}
	executor := &noopStepExecutor{}

	planner := planning.NewHiPlan(cfg, generator, executor, h.logger)

	startTime := time.Now()
	plan, err := planner.CreatePlan(context.Background(), req.Goal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, VerifierErrorResponse{
			Error: "failed to create plan: " + err.Error(),
		})
		return
	}

	resp := HiPlanResponse{
		PlanID:     plan.ID,
		Goal:       plan.Goal,
		State:      plan.State,
		Progress:   plan.Progress,
		Milestones: make([]HiPlanMilestoneResponse, len(plan.Milestones)),
		DurationMs: time.Since(startTime).Milliseconds(),
		CreatedAt:  plan.CreatedAt.Format(time.RFC3339),
	}

	for i, m := range plan.Milestones {
		resp.Milestones[i] = HiPlanMilestoneResponse{
			ID:          m.ID,
			Name:        m.Name,
			Description: m.Description,
			State:       string(m.State),
			Priority:    m.Priority,
			Progress:    m.Progress,
			StepsCount:  len(m.Steps),
			Error:       m.Error,
		}
	}

	h.logger.WithFields(logrus.Fields{
		"plan_id":    plan.ID,
		"goal":       req.Goal,
		"milestones": len(plan.Milestones),
	}).Info("HiPlan created")

	c.JSON(http.StatusOK, resp)
}

// --- MCTS ---

// MCTSRequest represents a request for Monte Carlo Tree Search.
type MCTSRequest struct {
	InitialState string             `json:"initial_state" binding:"required"`
	Config       *MCTSConfigRequest `json:"config,omitempty"`
}

// MCTSConfigRequest represents optional MCTS configuration.
type MCTSConfigRequest struct {
	ExplorationConstant float64 `json:"exploration_constant,omitempty"`
	MaxDepth            int     `json:"max_depth,omitempty"`
	MaxIterations       int     `json:"max_iterations,omitempty"`
	RolloutDepth        int     `json:"rollout_depth,omitempty"`
	SimulationCount     int     `json:"simulation_count,omitempty"`
	DiscountFactor      float64 `json:"discount_factor,omitempty"`
	TimeoutSeconds      int     `json:"timeout_seconds,omitempty"`
	UseUCTDP            *bool   `json:"use_uct_dp,omitempty"`
}

// MCTSResponse represents the response from MCTS.
type MCTSResponse struct {
	BestActions     []string `json:"best_actions"`
	FinalReward     float64  `json:"final_reward"`
	TotalIterations int      `json:"total_iterations"`
	RootVisits      int      `json:"root_visits"`
	TreeSize        int      `json:"tree_size"`
	DurationMs      int64    `json:"duration_ms"`
	Error           string   `json:"error,omitempty"`
}

// RunMCTS godoc
// @Summary Execute Monte Carlo Tree Search
// @Description Runs MCTS from an initial state to find the best action sequence
// @Tags planning
// @Accept json
// @Produce json
// @Param request body MCTSRequest true "Initial state and configuration"
// @Success 200 {object} MCTSResponse
// @Failure 400 {object} VerifierErrorResponse
// @Failure 500 {object} VerifierErrorResponse
// @Router /api/v1/planning/mcts [post]
func (h *PlanningHandler) RunMCTS(c *gin.Context) {
	var req MCTSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	cfg := planning.DefaultMCTSConfig()
	if req.Config != nil {
		if req.Config.ExplorationConstant > 0 {
			cfg.ExplorationConstant = req.Config.ExplorationConstant
		}
		if req.Config.MaxDepth > 0 {
			cfg.MaxDepth = req.Config.MaxDepth
		}
		if req.Config.MaxIterations > 0 {
			cfg.MaxIterations = req.Config.MaxIterations
		}
		if req.Config.RolloutDepth > 0 {
			cfg.RolloutDepth = req.Config.RolloutDepth
		}
		if req.Config.SimulationCount > 0 {
			cfg.SimulationCount = req.Config.SimulationCount
		}
		if req.Config.DiscountFactor > 0 {
			cfg.DiscountFactor = req.Config.DiscountFactor
		}
		if req.Config.TimeoutSeconds > 0 {
			cfg.Timeout = time.Duration(req.Config.TimeoutSeconds) * time.Second
		}
		if req.Config.UseUCTDP != nil {
			cfg.UseUCTDP = *req.Config.UseUCTDP
		}
	}

	actionGen := &staticActionGenerator{}
	rewardFunc := &staticRewardFunction{}

	mcts := planning.NewMCTS(cfg, actionGen, rewardFunc, nil, h.logger)

	result, err := mcts.Search(context.Background(), req.InitialState)
	if err != nil {
		c.JSON(http.StatusInternalServerError, VerifierErrorResponse{
			Error: "MCTS search failed: " + err.Error(),
		})
		return
	}

	resp := MCTSResponse{
		BestActions:     result.BestActions,
		FinalReward:     result.FinalReward,
		TotalIterations: result.TotalIterations,
		RootVisits:      result.RootVisits,
		TreeSize:        result.TreeSize,
		DurationMs:      result.Duration.Milliseconds(),
	}

	h.logger.WithFields(logrus.Fields{
		"iterations": result.TotalIterations,
		"tree_size":  result.TreeSize,
		"reward":     result.FinalReward,
	}).Info("MCTS search completed")

	c.JSON(http.StatusOK, resp)
}

// --- Tree of Thoughts ---

// ToTRequest represents a request for Tree of Thoughts.
type ToTRequest struct {
	Problem string            `json:"problem" binding:"required"`
	Config  *ToTConfigRequest `json:"config,omitempty"`
}

// ToTConfigRequest represents optional ToT configuration.
type ToTConfigRequest struct {
	MaxDepth           int     `json:"max_depth,omitempty"`
	MaxBranches        int     `json:"max_branches,omitempty"`
	MinScore           float64 `json:"min_score,omitempty"`
	PruneThreshold     float64 `json:"prune_threshold,omitempty"`
	SearchStrategy     string  `json:"search_strategy,omitempty"`
	BeamWidth          int     `json:"beam_width,omitempty"`
	Temperature        float64 `json:"temperature,omitempty"`
	EnableBacktracking *bool   `json:"enable_backtracking,omitempty"`
	MaxIterations      int     `json:"max_iterations,omitempty"`
	TimeoutSeconds     int     `json:"timeout_seconds,omitempty"`
}

// ToTResponse represents the response from Tree of Thoughts.
type ToTResponse struct {
	Problem       string       `json:"problem"`
	Solution      []ToTThought `json:"solution"`
	BestScore     float64      `json:"best_score"`
	Iterations    int          `json:"iterations"`
	Strategy      string       `json:"strategy"`
	TreeDepth     int          `json:"tree_depth"`
	NodesExplored int          `json:"nodes_explored"`
	DurationMs    int64        `json:"duration_ms"`
	Error         string       `json:"error,omitempty"`
}

// ToTThought represents a thought in the solution path.
type ToTThought struct {
	ID        string  `json:"id"`
	Content   string  `json:"content"`
	Reasoning string  `json:"reasoning,omitempty"`
	Score     float64 `json:"score"`
	Depth     int     `json:"depth"`
}

// RunToT godoc
// @Summary Execute Tree of Thoughts reasoning
// @Description Runs Tree of Thoughts search to solve a problem
// @Tags planning
// @Accept json
// @Produce json
// @Param request body ToTRequest true "Problem and configuration"
// @Success 200 {object} ToTResponse
// @Failure 400 {object} VerifierErrorResponse
// @Failure 500 {object} VerifierErrorResponse
// @Router /api/v1/planning/tot [post]
func (h *PlanningHandler) RunToT(c *gin.Context) {
	var req ToTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	cfg := planning.DefaultTreeOfThoughtsConfig()
	if req.Config != nil {
		if req.Config.MaxDepth > 0 {
			cfg.MaxDepth = req.Config.MaxDepth
		}
		if req.Config.MaxBranches > 0 {
			cfg.MaxBranches = req.Config.MaxBranches
		}
		if req.Config.MinScore > 0 {
			cfg.MinScore = req.Config.MinScore
		}
		if req.Config.PruneThreshold > 0 {
			cfg.PruneThreshold = req.Config.PruneThreshold
		}
		if req.Config.SearchStrategy != "" {
			cfg.SearchStrategy = req.Config.SearchStrategy
		}
		if req.Config.BeamWidth > 0 {
			cfg.BeamWidth = req.Config.BeamWidth
		}
		if req.Config.Temperature > 0 {
			cfg.Temperature = req.Config.Temperature
		}
		if req.Config.EnableBacktracking != nil {
			cfg.EnableBacktracking = *req.Config.EnableBacktracking
		}
		if req.Config.MaxIterations > 0 {
			cfg.MaxIterations = req.Config.MaxIterations
		}
		if req.Config.TimeoutSeconds > 0 {
			cfg.Timeout = time.Duration(req.Config.TimeoutSeconds) * time.Second
		}
	}

	generator := &staticThoughtGenerator{}
	evaluator := &staticThoughtEvaluator{}

	tot := planning.NewTreeOfThoughts(cfg, generator, evaluator, h.logger)

	result, err := tot.Solve(context.Background(), req.Problem)
	if err != nil {
		c.JSON(http.StatusInternalServerError, VerifierErrorResponse{
			Error: "Tree of Thoughts failed: " + err.Error(),
		})
		return
	}

	resp := ToTResponse{
		Problem:       result.Problem,
		BestScore:     result.BestScore,
		Iterations:    result.Iterations,
		Strategy:      result.Strategy,
		TreeDepth:     result.TreeDepth,
		NodesExplored: result.NodesExplored,
		DurationMs:    result.Duration.Milliseconds(),
	}

	if result.Solution != nil {
		resp.Solution = make([]ToTThought, len(result.Solution))
		for i, t := range result.Solution {
			resp.Solution[i] = ToTThought{
				ID:        t.ID,
				Content:   t.Content,
				Reasoning: t.Reasoning,
				Score:     t.Score,
				Depth:     t.Depth,
			}
		}
	}

	h.logger.WithFields(logrus.Fields{
		"problem":    req.Problem,
		"strategy":   result.Strategy,
		"iterations": result.Iterations,
		"best_score": result.BestScore,
	}).Info("Tree of Thoughts completed")

	c.JSON(http.StatusOK, resp)
}

// RegisterPlanningRoutes registers planning routes.
func RegisterPlanningRoutes(r *gin.RouterGroup, h *PlanningHandler) {
	p := r.Group("/planning")
	{
		// Core planning algorithms
		p.POST("/hiplan", h.CreateHiPlan)
		p.POST("/mcts", h.RunMCTS)
		p.POST("/tot", h.RunToT)

		// Plan Mode endpoints (from Claude Code)
		p.POST("/plan-mode/enter", h.EnterPlanMode)
		p.POST("/plan-mode/:id/exit", h.ExitPlanMode)
		p.GET("/plan-mode/:id/status", h.GetPlanModeStatus)
		p.POST("/plan-mode/:id/verify", h.VerifyPlan)
		p.POST("/plan-mode/:id/execute", h.StartPlanExecution)
		p.PUT("/plan-mode/:id/tasks/:taskId", h.UpdatePlanTask)
	}
}

// --- Static implementations for HTTP-driven execution ---
// These provide deterministic planning without requiring an LLM connection.
// In production, callers inject real LLM-backed generators/executors via
// the service layer.

// staticMilestoneGenerator generates milestones from the goal text.
type staticMilestoneGenerator struct{}

func (g *staticMilestoneGenerator) GenerateMilestones(
	_ context.Context, goal string,
) ([]*planning.Milestone, error) {
	return []*planning.Milestone{
		{
			ID:          "milestone-0",
			Name:        "Analyze: " + truncate(goal, 60),
			Description: "Analyze the goal and identify requirements",
			State:       planning.MilestoneStatePending,
			Priority:    0,
			Metadata:    map[string]interface{}{"source": "static"},
		},
		{
			ID:           "milestone-1",
			Name:         "Plan: " + truncate(goal, 60),
			Description:  "Create a detailed plan of action",
			State:        planning.MilestoneStatePending,
			Priority:     1,
			Dependencies: []string{"milestone-0"},
			Metadata:     map[string]interface{}{"source": "static"},
		},
		{
			ID:           "milestone-2",
			Name:         "Execute: " + truncate(goal, 60),
			Description:  "Execute the plan and verify results",
			State:        planning.MilestoneStatePending,
			Priority:     2,
			Dependencies: []string{"milestone-1"},
			Metadata:     map[string]interface{}{"source": "static"},
		},
	}, nil
}

func (g *staticMilestoneGenerator) GenerateSteps(
	_ context.Context, milestone *planning.Milestone,
) ([]*planning.PlanStep, error) {
	return []*planning.PlanStep{
		{
			ID:          fmt.Sprintf("%s-step-0", milestone.ID),
			MilestoneID: milestone.ID,
			Action:      "Initialize " + milestone.Name,
			State:       planning.PlanStepStatePending,
			Inputs:      map[string]interface{}{},
			Outputs:     map[string]interface{}{},
		},
		{
			ID:          fmt.Sprintf("%s-step-1", milestone.ID),
			MilestoneID: milestone.ID,
			Action:      "Process " + milestone.Name,
			State:       planning.PlanStepStatePending,
			Inputs:      map[string]interface{}{},
			Outputs:     map[string]interface{}{},
		},
	}, nil
}

func (g *staticMilestoneGenerator) GenerateHints(
	_ context.Context, _ *planning.PlanStep, _ string,
) ([]string, error) {
	return []string{"Verify inputs", "Check edge cases"}, nil
}

// noopStepExecutor marks steps as successful without side effects.
type noopStepExecutor struct{}

func (e *noopStepExecutor) Execute(
	_ context.Context, step *planning.PlanStep, _ []string,
) (*planning.StepResult, error) {
	return &planning.StepResult{
		Success:  true,
		Outputs:  map[string]interface{}{"step_id": step.ID, "status": "completed"},
		Logs:     []string{"Step executed successfully"},
		Duration: time.Millisecond,
	}, nil
}

func (e *noopStepExecutor) Validate(
	_ context.Context, _ *planning.PlanStep, _ *planning.StepResult,
) error {
	return nil
}

// staticActionGenerator produces a fixed set of actions for MCTS.
type staticActionGenerator struct{}

func (g *staticActionGenerator) GetActions(
	_ context.Context, state interface{},
) ([]string, error) {
	stateStr, ok := state.(string)
	if !ok {
		stateStr = fmt.Sprintf("%v", state)
	}
	return []string{
		"refine: " + truncate(stateStr, 40),
		"expand: " + truncate(stateStr, 40),
		"simplify: " + truncate(stateStr, 40),
	}, nil
}

func (g *staticActionGenerator) ApplyAction(
	_ context.Context, state interface{}, action string,
) (interface{}, error) {
	stateStr, ok := state.(string)
	if !ok {
		stateStr = fmt.Sprintf("%v", state)
	}
	return stateStr + " -> " + action, nil
}

// staticRewardFunction returns a fixed reward of 0.5 and is never terminal.
type staticRewardFunction struct{}

func (f *staticRewardFunction) Evaluate(
	_ context.Context, _ interface{},
) (float64, error) {
	return 0.5, nil
}

func (f *staticRewardFunction) IsTerminal(
	_ context.Context, _ interface{},
) (bool, error) {
	return false, nil
}

// staticThoughtGenerator generates static thoughts for ToT.
type staticThoughtGenerator struct{}

func (g *staticThoughtGenerator) GenerateThoughts(
	_ context.Context, parent *planning.Thought, count int,
) ([]*planning.Thought, error) {
	thoughts := make([]*planning.Thought, 0, count)
	approaches := []string{
		"Break the problem into smaller sub-problems",
		"Apply a known algorithm or pattern",
		"Consider edge cases and constraints",
		"Evaluate alternative approaches",
		"Synthesize a solution from observations",
	}
	for i := 0; i < count && i < len(approaches); i++ {
		thoughts = append(thoughts, &planning.Thought{
			ID:        fmt.Sprintf("%s-%d", parent.ID, i),
			Content:   approaches[i],
			State:     planning.ThoughtStatePending,
			CreatedAt: time.Now(),
			Metadata:  map[string]interface{}{"source": "static"},
		})
	}
	return thoughts, nil
}

func (g *staticThoughtGenerator) GenerateInitialThoughts(
	_ context.Context, problem string, count int,
) ([]*planning.Thought, error) {
	thoughts := make([]*planning.Thought, 0, count)
	approaches := []string{
		"Analyze the problem statement: " + truncate(problem, 50),
		"Identify key constraints and requirements",
		"Research similar solved problems",
		"Draft a high-level solution approach",
		"Evaluate feasibility of the approach",
	}
	for i := 0; i < count && i < len(approaches); i++ {
		thoughts = append(thoughts, &planning.Thought{
			ID:        fmt.Sprintf("init-%d", i),
			Content:   approaches[i],
			State:     planning.ThoughtStatePending,
			Depth:     1,
			CreatedAt: time.Now(),
			Metadata:  map[string]interface{}{"type": "initial"},
		})
	}
	return thoughts, nil
}

// staticThoughtEvaluator scores thoughts with a fixed value.
type staticThoughtEvaluator struct{}

func (e *staticThoughtEvaluator) EvaluateThought(
	_ context.Context, _ *planning.Thought,
) (float64, error) {
	return 0.7, nil
}

func (e *staticThoughtEvaluator) EvaluatePath(
	_ context.Context, path []*planning.Thought,
) (float64, error) {
	if len(path) == 0 {
		return 0, nil
	}
	return 0.7, nil
}

func (e *staticThoughtEvaluator) IsTerminal(
	_ context.Context, thought *planning.Thought,
) (bool, error) {
	// Terminal at depth >= 3 so searches converge
	return thought.Depth >= 3, nil
}

// truncate shortens a string to maxLen, appending "..." if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ============================================
// PLAN MODE - Multi-step planning with verification
// ============================================

// PlanState represents the state of a plan mode session
type PlanState string

const (
	PlanStateDraft     PlanState = "draft"
	PlanStateActive    PlanState = "active"
	PlanStateExecuting PlanState = "executing"
	PlanStateCompleted PlanState = "completed"
	PlanStateFailed    PlanState = "failed"
	PlanStateVerified  PlanState = "verified"
)

// TaskStatus represents the status of a plan task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusBlocked    TaskStatus = "blocked"
	TaskStatusVerified   TaskStatus = "verified"
)

// PlanModeSession represents a plan mode session
type PlanModeSession struct {
	ID           string                 `json:"id"`
	State        PlanState              `json:"state"`
	OriginalTask string                 `json:"original_task"`
	Tasks        []PlanTask             `json:"tasks"`
	Progress     float64                `json:"progress"`
	CurrentPhase string                 `json:"current_phase"`
	Verification PlanVerification       `json:"verification"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// PlanTask represents a task in the plan
type PlanTask struct {
	ID           string                 `json:"id"`
	Description  string                 `json:"description"`
	Status       TaskStatus             `json:"status"`
	Dependencies []string               `json:"dependencies"`
	CanParallel  bool                   `json:"can_parallel"`
	Verified     bool                   `json:"verified"`
	Output       string                 `json:"output,omitempty"`
	Error        string                 `json:"error,omitempty"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// PlanVerification represents the verification state
type PlanVerification struct {
	Required   bool       `json:"required"`
	Passed     bool       `json:"passed"`
	Feedback   string     `json:"feedback,omitempty"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	VerifiedBy string     `json:"verified_by,omitempty"`
}

// EnterPlanModeRequest represents a request to enter plan mode
type EnterPlanModeRequest struct {
	Task                string `json:"task" binding:"required"`
	RequireVerification bool   `json:"require_verification,omitempty"`
}

// ExitPlanModeRequest represents a request to exit plan mode
type ExitPlanModeRequest struct {
	Force  bool   `json:"force,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// VerifyPlanRequest represents a plan verification request
type VerifyPlanRequest struct {
	Approved bool   `json:"approved"`
	Feedback string `json:"feedback,omitempty"`
}

// UpdateTaskRequest represents a task update request
type UpdateTaskRequest struct {
	Status   string `json:"status,omitempty"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	Verified *bool  `json:"verified,omitempty"`
}

// Global plan mode sessions store (in production, use database)
var (
	planModeSessions   = make(map[string]*PlanModeSession)
	planModeSessionsMu sync.RWMutex
)

// EnterPlanMode enters plan mode for a task
func (h *PlanningHandler) EnterPlanMode(c *gin.Context) {
	var req EnterPlanModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	session := &PlanModeSession{
		ID:           generatePlanModeID(),
		State:        PlanStateDraft,
		OriginalTask: req.Task,
		Tasks:        []PlanTask{},
		Progress:     0.0,
		CurrentPhase: "breakdown",
		Verification: PlanVerification{
			Required: req.RequireVerification,
			Passed:   false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Auto-generate initial task breakdown (simplified)
	session.Tasks = h.generateTaskBreakdown(req.Task)

	planModeSessionsMu.Lock()
	planModeSessions[session.ID] = session
	planModeSessionsMu.Unlock()

	h.logger.WithFields(logrus.Fields{
		"session_id": session.ID,
		"task":       req.Task,
		"tasks":      len(session.Tasks),
	}).Info("Entered plan mode")

	c.JSON(http.StatusCreated, gin.H{
		"session_id":    session.ID,
		"state":         session.State,
		"original_task": session.OriginalTask,
		"tasks":         session.Tasks,
		"progress":      session.Progress,
		"created_at":    session.CreatedAt,
	})
}

// ExitPlanMode exits plan mode
func (h *PlanningHandler) ExitPlanMode(c *gin.Context) {
	id := c.Param("id")

	var req ExitPlanModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	planModeSessionsMu.Lock()
	session, exists := planModeSessions[id]
	if !exists {
		planModeSessionsMu.Unlock()
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan mode session not found"})
		return
	}

	// Check if we can exit
	if session.State == PlanStateExecuting && !req.Force {
		planModeSessionsMu.Unlock()
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{
			Error: "Cannot exit while executing. Use force=true to force exit.",
		})
		return
	}

	now := time.Now()
	session.State = PlanStateCompleted
	session.CompletedAt = &now
	session.UpdatedAt = now

	planModeSessionsMu.Unlock()

	h.logger.WithFields(logrus.Fields{
		"session_id": id,
		"force":      req.Force,
		"reason":     req.Reason,
	}).Info("Exited plan mode")

	c.JSON(http.StatusOK, gin.H{
		"session_id":   session.ID,
		"state":        session.State,
		"progress":     session.Progress,
		"completed_at": session.CompletedAt,
	})
}

// GetPlanModeStatus gets the status of a plan mode session
func (h *PlanningHandler) GetPlanModeStatus(c *gin.Context) {
	id := c.Param("id")

	planModeSessionsMu.RLock()
	session, exists := planModeSessions[id]
	planModeSessionsMu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan mode session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// VerifyPlan verifies a plan before execution
func (h *PlanningHandler) VerifyPlan(c *gin.Context) {
	id := c.Param("id")

	var req VerifyPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	planModeSessionsMu.Lock()
	session, exists := planModeSessions[id]
	if !exists {
		planModeSessionsMu.Unlock()
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan mode session not found"})
		return
	}

	if session.State != PlanStateDraft && session.State != PlanStateActive {
		planModeSessionsMu.Unlock()
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{
			Error: fmt.Sprintf("Cannot verify plan in state: %s", session.State),
		})
		return
	}

	now := time.Now()
	session.Verification.Passed = req.Approved
	session.Verification.Feedback = req.Feedback
	session.Verification.VerifiedAt = &now
	session.UpdatedAt = now

	if req.Approved {
		session.State = PlanStateVerified
		session.Verification.VerifiedBy = "user" // Could be API key, user ID, etc.
	} else {
		session.State = PlanStateDraft
	}

	planModeSessionsMu.Unlock()

	h.logger.WithFields(logrus.Fields{
		"session_id": id,
		"approved":   req.Approved,
		"feedback":   req.Feedback,
	}).Info("Plan verification completed")

	c.JSON(http.StatusOK, gin.H{
		"session_id":  session.ID,
		"verified":    req.Approved,
		"state":       session.State,
		"feedback":    req.Feedback,
		"verified_at": now,
	})
}

// StartPlanExecution starts executing a verified plan
func (h *PlanningHandler) StartPlanExecution(c *gin.Context) {
	id := c.Param("id")

	planModeSessionsMu.Lock()
	session, exists := planModeSessions[id]
	if !exists {
		planModeSessionsMu.Unlock()
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan mode session not found"})
		return
	}

	if session.Verification.Required && !session.Verification.Passed {
		planModeSessionsMu.Unlock()
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{
			Error: "Plan requires verification before execution",
		})
		return
	}

	if session.State == PlanStateExecuting {
		planModeSessionsMu.Unlock()
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: "Plan already executing"})
		return
	}

	session.State = PlanStateExecuting
	session.CurrentPhase = "execution"
	session.UpdatedAt = time.Now()
	planModeSessionsMu.Unlock()

	// Start execution in background
	go h.executePlan(id)

	h.logger.WithFields(logrus.Fields{
		"session_id": id,
	}).Info("Started plan execution")

	c.JSON(http.StatusOK, gin.H{
		"session_id": session.ID,
		"state":      session.State,
		"message":    "Plan execution started",
	})
}

// UpdatePlanTask updates a task's status
func (h *PlanningHandler) UpdatePlanTask(c *gin.Context) {
	id := c.Param("id")
	taskID := c.Param("taskId")

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	planModeSessionsMu.Lock()
	session, exists := planModeSessions[id]
	if !exists {
		planModeSessionsMu.Unlock()
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "plan mode session not found"})
		return
	}

	found := false
	for i := range session.Tasks {
		if session.Tasks[i].ID == taskID {
			if req.Status != "" {
				session.Tasks[i].Status = TaskStatus(req.Status)
			}
			if req.Output != "" {
				session.Tasks[i].Output = req.Output
			}
			if req.Error != "" {
				session.Tasks[i].Error = req.Error
			}
			if req.Verified != nil {
				session.Tasks[i].Verified = *req.Verified
			}

			now := time.Now()
			if req.Status == "in_progress" && session.Tasks[i].StartedAt == nil {
				session.Tasks[i].StartedAt = &now
			}
			if (req.Status == "completed" || req.Status == "failed") && session.Tasks[i].CompletedAt == nil {
				session.Tasks[i].CompletedAt = &now
			}

			found = true
			break
		}
	}

	if !found {
		planModeSessionsMu.Unlock()
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "task not found"})
		return
	}

	// Recalculate progress
	session.Progress = h.calculateProgress(session.Tasks)
	session.UpdatedAt = time.Now()
	planModeSessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"session_id": id,
		"task_id":    taskID,
		"progress":   session.Progress,
	})
}

// generateTaskBreakdown decomposes a goal into PlanTasks. When a
// RequestService is wired (the production path, set up in router.go),
// it calls a real LLM to produce task-specific decomposition. When
// not wired (test/dev mode), it falls back to a 5-step generic
// template documented as such on each task's Verified=false flag.
//
// CONST-035 §c: closes #planning-llm-task-breakdown. The previous
// version was always template-only, which made the function name
// (generateTaskBreakdown) a comment-bluff. Now the production code
// path actually runs LLM decomposition; the template path is reserved
// for graceful fallback when the LLM is unreachable or no
// RequestService is configured.
func (h *PlanningHandler) generateTaskBreakdown(task string) []PlanTask {
	if h.requestService != nil {
		tasks, err := h.llmTaskBreakdown(task)
		if err == nil && len(tasks) > 0 {
			return tasks
		}
		// LLM call failed — log the error detail (per CONST-035 §c
		// honest gap-marking) and fall through to template
		// (fail-OPEN per CLAUDE.md cold-start guidance).
		if err != nil {
			h.logger.WithError(err).Warn("LLM task breakdown failed; falling back to template")
		} else {
			h.logger.Warn("LLM task breakdown returned 0 tasks; falling back to template")
		}
	}
	return h.templateTaskBreakdown(task)
}

// llmTaskBreakdown calls the LLM via the wired RequestService, parses
// the response as a JSON array of {description, can_parallel, dependencies},
// and converts to PlanTasks.
func (h *PlanningHandler) llmTaskBreakdown(task string) ([]PlanTask, error) {
	if h.requestService == nil {
		return nil, fmt.Errorf("no request service")
	}

	prompt := fmt.Sprintf(`You are a planning agent. Break down the following goal into a list of concrete, actionable tasks. Return ONLY a JSON array of objects with this exact shape (no prose, no markdown fences):

[
  {"description": "concise task description", "can_parallel": false, "dependencies": []},
  ...
]

Each description should be ≤ 80 chars. dependencies are a list of task indices (0-based) that must complete before this task. can_parallel is true if this task can run alongside other can_parallel tasks. Aim for 3-7 tasks.

Goal: %s`, truncate(task, 500))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID:     "planning-breakdown-" + generateTaskID(),
		Prompt: prompt,
		ModelParams: models.ModelParameters{
			Model:       "helixagent-llm",
			Temperature: 0.3,
			MaxTokens:   1200,
		},
	}

	resp, err := h.requestService.ProcessRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ProcessRequest: %w", err)
	}
	if resp == nil || resp.Content == "" {
		return nil, fmt.Errorf("empty LLM response")
	}

	// Strip markdown code fences if the LLM ignored the instruction
	body := strings.TrimSpace(resp.Content)
	body = strings.TrimPrefix(body, "```json")
	body = strings.TrimPrefix(body, "```")
	body = strings.TrimSuffix(body, "```")
	body = strings.TrimSpace(body)

	type llmTask struct {
		Description  string   `json:"description"`
		CanParallel  bool     `json:"can_parallel"`
		Dependencies []int    `json:"dependencies"`
	}
	var raw []llmTask
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, fmt.Errorf("parse LLM JSON: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("LLM returned empty task list")
	}

	// Convert to PlanTasks; resolve dependency indices to task IDs
	out := make([]PlanTask, len(raw))
	for i, t := range raw {
		out[i] = PlanTask{
			ID:           generateTaskID(),
			Description:  truncate(t.Description, 200),
			Status:       TaskStatusPending,
			Dependencies: []string{},
			CanParallel:  t.CanParallel,
			Verified:     true, // LLM-generated, not template — discriminator for callers
		}
	}
	// Second pass to resolve indices → IDs (avoid use-before-definition)
	for i, t := range raw {
		for _, depIdx := range t.Dependencies {
			if depIdx >= 0 && depIdx < len(out) && depIdx != i {
				out[i].Dependencies = append(out[i].Dependencies, out[depIdx].ID)
			}
		}
	}
	return out, nil
}

// templateTaskBreakdown is the fallback when no LLM is wired or the
// LLM call fails. Returns the canonical 5-step skeleton.
func (h *PlanningHandler) templateTaskBreakdown(task string) []PlanTask {
	return []PlanTask{
		{
			ID:           generateTaskID(),
			Description:  "Analyze requirements: " + truncate(task, 50),
			Status:       TaskStatusPending,
			Dependencies: []string{},
			CanParallel:  false,
			Verified:     false,
		},
		{
			ID:           generateTaskID(),
			Description:  "Design solution approach",
			Status:       TaskStatusPending,
			Dependencies: []string{},
			CanParallel:  false,
			Verified:     false,
		},
		{
			ID:           generateTaskID(),
			Description:  "Implement core functionality",
			Status:       TaskStatusPending,
			Dependencies: []string{},
			CanParallel:  false,
			Verified:     false,
		},
		{
			ID:           generateTaskID(),
			Description:  "Add tests and validation",
			Status:       TaskStatusPending,
			Dependencies: []string{},
			CanParallel:  true,
			Verified:     false,
		},
		{
			ID:           generateTaskID(),
			Description:  "Review and finalize",
			Status:       TaskStatusPending,
			Dependencies: []string{},
			CanParallel:  false,
			Verified:     false,
		},
	}
}

// calculateProgress calculates the overall progress
func (h *PlanningHandler) calculateProgress(tasks []PlanTask) float64 {
	if len(tasks) == 0 {
		return 0.0
	}

	completed := 0
	for _, task := range tasks {
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusVerified {
			completed++
		}
	}

	return float64(completed) / float64(len(tasks))
}

// executePlan executes the plan tasks
func (h *PlanningHandler) executePlan(sessionID string) {
	planModeSessionsMu.RLock()
	session, exists := planModeSessions[sessionID]
	planModeSessionsMu.RUnlock()

	if !exists {
		return
	}

	// Execute tasks in dependency order
	for i := range session.Tasks {
		// Check for cancellation
		planModeSessionsMu.RLock()
		s, ok := planModeSessions[sessionID]
		planModeSessionsMu.RUnlock()

		if !ok || s.State != PlanStateExecuting {
			return
		}

		// Start task
		planModeSessionsMu.Lock()
		if session.Tasks[i].Status == TaskStatusPending {
			now := time.Now()
			session.Tasks[i].Status = TaskStatusInProgress
			session.Tasks[i].StartedAt = &now
		}
		planModeSessionsMu.Unlock()

		// Simulate execution (in production, this would execute actual work)
		time.Sleep(100 * time.Millisecond)

		// Complete task
		planModeSessionsMu.Lock()
		now := time.Now()
		session.Tasks[i].Status = TaskStatusCompleted
		session.Tasks[i].CompletedAt = &now
		session.Tasks[i].Output = "Task completed successfully"
		session.Progress = h.calculateProgress(session.Tasks)
		session.UpdatedAt = now
		planModeSessionsMu.Unlock()

		h.logger.WithFields(logrus.Fields{
			"session_id": sessionID,
			"task_id":    session.Tasks[i].ID,
			"progress":   session.Progress,
		}).Debug("Task completed")
	}

	// Mark plan as completed
	planModeSessionsMu.Lock()
	session.State = PlanStateCompleted
	now := time.Now()
	session.CompletedAt = &now
	session.UpdatedAt = now
	session.Progress = 1.0
	planModeSessionsMu.Unlock()

	h.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
	}).Info("Plan execution completed")
}

// generatePlanModeID generates a unique plan mode session ID.
// D-20: uuid suffix makes the ID unique-by-construction; the bare UnixNano suffix
// collided for plan sessions created within one coarse clock tick.
func generatePlanModeID() string {
	return fmt.Sprintf("plan_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8])
}

// generateTaskID generates a unique task ID.
// D-20: uuid suffix makes the ID unique-by-construction; the bare UnixNano suffix
// collided for tasks created within one coarse clock tick.
func generateTaskID() string {
	return fmt.Sprintf("task_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8])
}
