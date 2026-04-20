// Package planning provides Hierarchical Planning (HiPlan) implementation
// for managing complex multi-step tasks with global milestones and local hints.
package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
)

// MilestoneState represents the state of a milestone
type MilestoneState string

const (
	MilestoneStatePending    MilestoneState = "pending"
	MilestoneStateInProgress MilestoneState = "in_progress"
	MilestoneStateCompleted  MilestoneState = "completed"
	MilestoneStateFailed     MilestoneState = "failed"
	MilestoneStateSkipped    MilestoneState = "skipped"
)

// Milestone represents a high-level goal in the hierarchical plan
type Milestone struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	State        MilestoneState         `json:"state"`
	Priority     int                    `json:"priority"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Steps        []*PlanStep            `json:"steps,omitempty"`
	Hints        []string               `json:"hints,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Progress     float64                `json:"progress"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	Error        string                 `json:"error,omitempty"`
}

// PlanStepState represents the state of a plan step
type PlanStepState string

const (
	PlanStepStatePending    PlanStepState = "pending"
	PlanStepStateInProgress PlanStepState = "in_progress"
	PlanStepStateCompleted  PlanStepState = "completed"
	PlanStepStateFailed     PlanStepState = "failed"
)

// PlanStep represents a low-level action within a milestone
type PlanStep struct {
	ID          string                 `json:"id"`
	MilestoneID string                 `json:"milestone_id"`
	Action      string                 `json:"action"`
	Description string                 `json:"description"`
	State       PlanStepState          `json:"state"`
	Hints       []string               `json:"hints,omitempty"`
	Inputs      map[string]interface{} `json:"inputs,omitempty"`
	Outputs     map[string]interface{} `json:"outputs,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// HiPlanConfig holds configuration for Hierarchical Planning
type HiPlanConfig struct {
	// MaxMilestones is the maximum number of milestones
	MaxMilestones int `json:"max_milestones"`
	// MaxStepsPerMilestone is the maximum steps per milestone
	MaxStepsPerMilestone int `json:"max_steps_per_milestone"`
	// EnableParallelMilestones allows parallel milestone execution
	EnableParallelMilestones bool `json:"enable_parallel_milestones"`
	// MaxParallelMilestones is the max parallel milestones
	MaxParallelMilestones int `json:"max_parallel_milestones"`
	// EnableAdaptivePlanning allows runtime plan modification
	EnableAdaptivePlanning bool `json:"enable_adaptive_planning"`
	// RetryFailedSteps enables automatic retry
	RetryFailedSteps bool `json:"retry_failed_steps"`
	// MaxRetries is the max retries per step
	MaxRetries int `json:"max_retries"`
	// Timeout is the overall planning timeout
	Timeout time.Duration `json:"timeout"`
	// StepTimeout is the timeout per step
	StepTimeout time.Duration `json:"step_timeout"`
}

// DefaultHiPlanConfig returns default configuration
func DefaultHiPlanConfig() HiPlanConfig {
	return HiPlanConfig{
		MaxMilestones:            20,
		MaxStepsPerMilestone:     50,
		EnableParallelMilestones: true,
		MaxParallelMilestones:    3,
		EnableAdaptivePlanning:   true,
		RetryFailedSteps:         true,
		MaxRetries:               3,
		Timeout:                  30 * time.Minute,
		StepTimeout:              5 * time.Minute,
	}
}

// MilestoneGenerator generates milestones from a goal
type MilestoneGenerator interface {
	// GenerateMilestones creates milestones for a goal
	GenerateMilestones(ctx context.Context, goal string) ([]*Milestone, error)
	// GenerateSteps creates steps for a milestone
	GenerateSteps(ctx context.Context, milestone *Milestone) ([]*PlanStep, error)
	// GenerateHints generates contextual hints for a step
	GenerateHints(ctx context.Context, step *PlanStep, context string) ([]string, error)
}

// StepExecutor executes plan steps
type StepExecutor interface {
	// Execute executes a plan step
	Execute(ctx context.Context, step *PlanStep, hints []string) (*StepResult, error)
	// Validate validates step outputs
	Validate(ctx context.Context, step *PlanStep, result *StepResult) error
}

// StepResult holds the result of executing a step
type StepResult struct {
	Success  bool                   `json:"success"`
	Outputs  map[string]interface{} `json:"outputs,omitempty"`
	Logs     []string               `json:"logs,omitempty"`
	Duration time.Duration          `json:"duration"`
	Error    string                 `json:"error,omitempty"`
}

// HierarchicalPlan represents a complete hierarchical plan
type HierarchicalPlan struct {
	ID          string                 `json:"id"`
	Goal        string                 `json:"goal"`
	Milestones  []*Milestone           `json:"milestones"`
	State       string                 `json:"state"`
	Progress    float64                `json:"progress"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// HiPlan implements Hierarchical Planning.
//
// Concurrency model (CONST-029): milestoneLibrary → *safe.Store;
// currentPlan → atomic.Pointer[HierarchicalPlan]. mu dropped.
type HiPlan struct {
	config           HiPlanConfig
	generator        MilestoneGenerator
	executor         StepExecutor
	currentPlan      atomic.Pointer[HierarchicalPlan]
	milestoneLibrary *safe.Store[string, *Milestone]
	logger           *logrus.Logger
}

// NewHiPlan creates a new HiPlan instance
func NewHiPlan(config HiPlanConfig, generator MilestoneGenerator, executor StepExecutor, logger *logrus.Logger) *HiPlan {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)
	}

	return &HiPlan{
		config:           config,
		generator:        generator,
		executor:         executor,
		milestoneLibrary: safe.NewStore[string, *Milestone](),
		logger:           logger,
	}
}

// CreatePlan creates a hierarchical plan for a goal
func (h *HiPlan) CreatePlan(ctx context.Context, goal string) (*HierarchicalPlan, error) {
	ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	h.logger.Infof("Creating plan for goal: %s", goal)

	// Generate milestones
	milestones, err := h.generator.GenerateMilestones(ctx, goal)
	if err != nil {
		return nil, fmt.Errorf("failed to generate milestones: %w", err)
	}

	if len(milestones) > h.config.MaxMilestones {
		milestones = milestones[:h.config.MaxMilestones]
	}

	// Generate steps for each milestone
	for _, milestone := range milestones {
		steps, err := h.generator.GenerateSteps(ctx, milestone)
		if err != nil {
			h.logger.Warnf("Failed to generate steps for milestone %s: %v", milestone.ID, err)
			continue
		}

		if len(steps) > h.config.MaxStepsPerMilestone {
			steps = steps[:h.config.MaxStepsPerMilestone]
		}

		milestone.Steps = steps
	}

	plan := &HierarchicalPlan{
		ID:         fmt.Sprintf("plan-%d", time.Now().UnixNano()),
		Goal:       goal,
		Milestones: milestones,
		State:      "created",
		CreatedAt:  time.Now(),
		Metadata:   make(map[string]interface{}),
	}

	h.currentPlan.Store(plan)

	return plan, nil
}

// ExecutePlan executes a hierarchical plan
func (h *HiPlan) ExecutePlan(ctx context.Context, plan *HierarchicalPlan) (*PlanResult, error) {
	ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	startTime := time.Now()
	plan.State = "executing"
	now := time.Now()
	plan.StartedAt = &now

	result := &PlanResult{
		PlanID:           plan.ID,
		MilestoneResults: make([]*MilestoneResult, 0),
		StartedAt:        startTime,
	}

	// Sort milestones by priority
	sortedMilestones := h.sortMilestonesByDependencies(plan.Milestones)

	// Execute milestones
	if h.config.EnableParallelMilestones {
		result.MilestoneResults = h.executeParallelMilestones(ctx, sortedMilestones)
	} else {
		result.MilestoneResults = h.executeSequentialMilestones(ctx, sortedMilestones)
	}

	// Calculate overall result
	completedCount := 0
	failedCount := 0
	for _, mr := range result.MilestoneResults {
		if mr.Success {
			completedCount++
		} else {
			failedCount++
		}
	}

	result.Success = failedCount == 0
	result.CompletedMilestones = completedCount
	result.FailedMilestones = failedCount
	result.Duration = time.Since(startTime)

	// Update plan state
	endTime := time.Now()
	plan.CompletedAt = &endTime
	if result.Success {
		plan.State = "completed"
		plan.Progress = 1.0
	} else {
		plan.State = "failed"
		plan.Progress = float64(completedCount) / float64(len(plan.Milestones))
	}

	return result, nil
}

// sortMilestonesByDependencies performs topological sort
func (h *HiPlan) sortMilestonesByDependencies(milestones []*Milestone) []*Milestone {
	// Build dependency graph
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for _, m := range milestones {
		if _, exists := inDegree[m.ID]; !exists {
			inDegree[m.ID] = 0
		}
		for _, dep := range m.Dependencies {
			dependents[dep] = append(dependents[dep], m.ID)
			inDegree[m.ID]++
		}
	}

	// Find nodes with no dependencies
	queue := make([]string, 0)
	for _, m := range milestones {
		if inDegree[m.ID] == 0 {
			queue = append(queue, m.ID)
		}
	}

	// Topological sort
	sorted := make([]*Milestone, 0, len(milestones))
	milestoneMap := make(map[string]*Milestone)
	for _, m := range milestones {
		milestoneMap[m.ID] = m
	}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		if m, exists := milestoneMap[id]; exists {
			sorted = append(sorted, m)
		}

		for _, depID := range dependents[id] {
			inDegree[depID]--
			if inDegree[depID] == 0 {
				queue = append(queue, depID)
			}
		}
	}

	// Add remaining milestones (if any cycles)
	for _, m := range milestones {
		found := false
		for _, s := range sorted {
			if s.ID == m.ID {
				found = true
				break
			}
		}
		if !found {
			sorted = append(sorted, m)
		}
	}

	return sorted
}

// executeSequentialMilestones executes milestones sequentially
func (h *HiPlan) executeSequentialMilestones(ctx context.Context, milestones []*Milestone) []*MilestoneResult {
	results := make([]*MilestoneResult, 0, len(milestones))

	for _, milestone := range milestones {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		result := h.executeMilestone(ctx, milestone)
		results = append(results, result)

		// Stop on failure if not adaptive
		if !result.Success && !h.config.EnableAdaptivePlanning {
			break
		}
	}

	return results
}

// executeParallelMilestones executes milestones in parallel
func (h *HiPlan) executeParallelMilestones(ctx context.Context, milestones []*Milestone) []*MilestoneResult {
	results := make([]*MilestoneResult, len(milestones))
	completedDeps := make(map[string]bool)
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, h.config.MaxParallelMilestones)

	for i, milestone := range milestones {
		// Check dependencies
		canExecute := true
		for _, dep := range milestone.Dependencies {
			mu.Lock()
			if !completedDeps[dep] {
				canExecute = false
			}
			mu.Unlock()
		}

		if !canExecute {
			// Execute sequentially if dependencies not met
			result := h.executeMilestone(ctx, milestone)
			results[i] = result
			if result.Success {
				mu.Lock()
				completedDeps[milestone.ID] = true
				mu.Unlock()
			}
			continue
		}

		wg.Add(1)
		go func(idx int, m *Milestone) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := h.executeMilestone(ctx, m)
			results[idx] = result

			if result.Success {
				mu.Lock()
				completedDeps[m.ID] = true
				mu.Unlock()
			}
		}(i, milestone)
	}

	wg.Wait()
	return results
}

// executeMilestone executes a single milestone
func (h *HiPlan) executeMilestone(ctx context.Context, milestone *Milestone) *MilestoneResult {
	startTime := time.Now()
	milestone.State = MilestoneStateInProgress
	now := time.Now()
	milestone.StartedAt = &now

	result := &MilestoneResult{
		MilestoneID: milestone.ID,
		StepResults: make([]*StepResult, 0),
		StartedAt:   startTime,
	}

	completedSteps := 0
	for _, step := range milestone.Steps {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err().Error()
			milestone.State = MilestoneStateFailed
			return result
		default:
		}

		// Generate contextual hints
		hints, err := h.generator.GenerateHints(ctx, step, h.buildContext(milestone, step))
		if err != nil {
			h.logger.Warnf("Failed to generate hints: %v", err)
			hints = milestone.Hints
		}
		step.Hints = hints

		// Execute step with retries
		var stepResult *StepResult
		var execErr error

		for retry := 0; retry <= h.config.MaxRetries; retry++ {
			stepCtx, cancel := context.WithTimeout(ctx, h.config.StepTimeout)
			stepResult, execErr = h.executor.Execute(stepCtx, step, hints)
			cancel()

			if execErr == nil && stepResult.Success {
				break
			}

			if !h.config.RetryFailedSteps || retry == h.config.MaxRetries {
				break
			}

			h.logger.Warnf("Step %s failed, retrying (%d/%d)", step.ID, retry+1, h.config.MaxRetries)
		}

		if stepResult == nil {
			stepResult = &StepResult{
				Success: false,
				Error:   "execution failed",
			}
		}

		result.StepResults = append(result.StepResults, stepResult)
		step.State = PlanStepStateCompleted
		if stepResult.Success {
			completedSteps++
			step.Outputs = stepResult.Outputs
		} else {
			step.State = PlanStepStateFailed
			step.Error = stepResult.Error
			if !h.config.EnableAdaptivePlanning {
				break
			}
		}

		// Update progress
		milestone.Progress = float64(completedSteps) / float64(len(milestone.Steps))
	}

	result.Duration = time.Since(startTime)
	result.Success = completedSteps == len(milestone.Steps)

	endTime := time.Now()
	milestone.CompletedAt = &endTime
	if result.Success {
		milestone.State = MilestoneStateCompleted
	} else {
		milestone.State = MilestoneStateFailed
	}

	return result
}

// ExecuteStep executes a single step with hints and retries
func (h *HiPlan) ExecuteStep(ctx context.Context, step *PlanStep, hints []string) (*StepResult, error) {
	// Execute step with retries
	var stepResult *StepResult
	var execErr error

	for retry := 0; retry <= h.config.MaxRetries; retry++ {
		stepCtx, cancel := context.WithTimeout(ctx, h.config.StepTimeout)
		stepResult, execErr = h.executor.Execute(stepCtx, step, hints)
		cancel()

		if execErr == nil && stepResult != nil && stepResult.Success {
			break
		}

		if !h.config.RetryFailedSteps || retry == h.config.MaxRetries {
			break
		}

		h.logger.Warnf("Step %s failed, retrying (%d/%d)", step.ID, retry+1, h.config.MaxRetries)
	}

	if stepResult == nil {
		stepResult = &StepResult{
			Success: false,
			Error:   "execution failed",
		}
		if execErr != nil {
			stepResult.Error = execErr.Error()
		}
	}

	return stepResult, execErr
}

// buildContext builds context string for hint generation
func (h *HiPlan) buildContext(milestone *Milestone, step *PlanStep) string {
	return fmt.Sprintf("Milestone: %s\nDescription: %s\nStep: %s\nAction: %s",
		milestone.Name, milestone.Description, step.ID, step.Action)
}

// MilestoneResult holds the result of executing a milestone
type MilestoneResult struct {
	MilestoneID string        `json:"milestone_id"`
	Success     bool          `json:"success"`
	StepResults []*StepResult `json:"step_results"`
	Duration    time.Duration `json:"duration"`
	StartedAt   time.Time     `json:"started_at"`
	Error       string        `json:"error,omitempty"`
}

// PlanResult holds the result of executing a plan
type PlanResult struct {
	PlanID              string             `json:"plan_id"`
	Success             bool               `json:"success"`
	MilestoneResults    []*MilestoneResult `json:"milestone_results"`
	CompletedMilestones int                `json:"completed_milestones"`
	FailedMilestones    int                `json:"failed_milestones"`
	Duration            time.Duration      `json:"duration"`
	StartedAt           time.Time          `json:"started_at"`
}

// MarshalJSON implements custom JSON marshaling
func (r *PlanResult) MarshalJSON() ([]byte, error) {
	type Alias PlanResult
	return json.Marshal(&struct {
		*Alias
		DurationMs int64 `json:"duration_ms"`
	}{
		Alias:      (*Alias)(r),
		DurationMs: r.Duration.Milliseconds(),
	})
}

// LLMMilestoneGenerator implements MilestoneGenerator using an LLM
type LLMMilestoneGenerator struct {
	generateFunc func(ctx context.Context, prompt string) (string, error)
	logger       *logrus.Logger
}

// NewLLMMilestoneGenerator creates a new LLM-based milestone generator
func NewLLMMilestoneGenerator(generateFunc func(ctx context.Context, prompt string) (string, error), logger *logrus.Logger) *LLMMilestoneGenerator {
	return &LLMMilestoneGenerator{
		generateFunc: generateFunc,
		logger:       logger,
	}
}

// GenerateMilestones generates milestones for a goal
func (g *LLMMilestoneGenerator) GenerateMilestones(ctx context.Context, goal string) ([]*Milestone, error) {
	prompt := fmt.Sprintf(`Create a hierarchical plan for the following goal:
"%s"

Generate 3-5 high-level milestones that need to be achieved.
For each milestone, provide:
1. A clear name
2. A description
3. Any dependencies on other milestones (by number)

Format as numbered list with details.`, goal)

	response, err := g.generateFunc(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse milestones from response
	milestones := make([]*Milestone, 0)
	lines := splitLines(response)

	for i, line := range lines {
		if len(line) == 0 {
			continue
		}

		milestone := &Milestone{
			ID:       fmt.Sprintf("milestone-%d", i),
			Name:     line,
			State:    MilestoneStatePending,
			Priority: i,
			Metadata: make(map[string]interface{}),
		}
		milestones = append(milestones, milestone)
	}

	return milestones, nil
}

// GenerateSteps generates steps for a milestone
func (g *LLMMilestoneGenerator) GenerateSteps(ctx context.Context, milestone *Milestone) ([]*PlanStep, error) {
	prompt := fmt.Sprintf(`For the milestone: "%s"
Description: %s

Generate 3-7 specific action steps to complete this milestone.
Each step should be:
1. Concrete and actionable
2. Independently executable
3. Verifiable for completion

Format as numbered list.`, milestone.Name, milestone.Description)

	response, err := g.generateFunc(ctx, prompt)
	if err != nil {
		return nil, err
	}

	steps := make([]*PlanStep, 0)
	lines := splitLines(response)

	for i, line := range lines {
		if len(line) == 0 {
			continue
		}

		step := &PlanStep{
			ID:          fmt.Sprintf("%s-step-%d", milestone.ID, i),
			MilestoneID: milestone.ID,
			Action:      line,
			State:       PlanStepStatePending,
			Inputs:      make(map[string]interface{}),
			Outputs:     make(map[string]interface{}),
		}
		steps = append(steps, step)
	}

	return steps, nil
}

// GenerateHints generates contextual hints for a step
func (g *LLMMilestoneGenerator) GenerateHints(ctx context.Context, step *PlanStep, context string) ([]string, error) {
	prompt := fmt.Sprintf(`Given this context:
%s

Generate 2-3 specific hints or tips to help execute this step successfully.
Focus on:
- Edge cases to consider
- Best practices
- Common pitfalls to avoid

Format as bullet points.`, context)

	response, err := g.generateFunc(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return splitLines(response), nil
}

// AddToLibrary adds a milestone to the library for reuse
func (h *HiPlan) AddToLibrary(milestone *Milestone) {
	h.milestoneLibrary.Put(milestone.ID, milestone)
}

// GetFromLibrary retrieves a milestone from the library
func (h *HiPlan) GetFromLibrary(id string) (*Milestone, bool) {
	return h.milestoneLibrary.Get(id)
}

// GetCurrentPlan returns the current plan
func (h *HiPlan) GetCurrentPlan() *HierarchicalPlan {
	return h.currentPlan.Load()
}
