package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// SpecKitPhase represents a phase in the SpecKit development flow
type SpecKitPhase string

const (
	PhaseConstitution SpecKitPhase = "constitution"
	PhaseSpecify      SpecKitPhase = "specify"
	PhaseClarify      SpecKitPhase = "clarify"
	PhasePlan         SpecKitPhase = "plan"
	PhaseTasks        SpecKitPhase = "tasks"
	PhaseAnalyze      SpecKitPhase = "analyze"
	PhaseImplement    SpecKitPhase = "implement"
)

// SpecKitPhaseResult contains the result of a single SpecKit phase
type SpecKitPhaseResult struct {
	Phase        SpecKitPhase           `json:"phase"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time"`
	Duration     time.Duration          `json:"duration"`
	Success      bool                   `json:"success"`
	Output       string                 `json:"output"`
	Artifact     string                 `json:"artifact"` // Main output artifact
	Artifacts    map[string]interface{} `json:"artifacts,omitempty"`
	DebateID     string                 `json:"debate_id,omitempty"`
	QualityScore float64                `json:"quality_score,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Cached       bool                   `json:"cached,omitempty"` // Was this result loaded from cache?
}

// SpecKitFlowResult contains the complete result of a SpecKit flow
type SpecKitFlowResult struct {
	FlowID              string                         `json:"flow_id"`
	StartTime           time.Time                      `json:"start_time"`
	EndTime             time.Time                      `json:"end_time"`
	Duration            time.Duration                  `json:"duration"`
	Success             bool                           `json:"success"`
	PhaseResults        []SpecKitPhaseResult           `json:"phase_results"`
	Phases              map[string]*SpecKitPhaseResult `json:"phases"` // Quick lookup by phase name
	Constitution        *Constitution                  `json:"constitution,omitempty"`
	FinalArtifact       string                         `json:"final_artifact"` // Implementation output
	OverallQualityScore float64                        `json:"overall_quality_score"`
	Metadata            map[string]interface{}         `json:"metadata,omitempty"`
	ResumedFromCache    bool                           `json:"resumed_from_cache,omitempty"` // Was flow resumed from cache?
	ResumedFromPhase    SpecKitPhase                   `json:"resumed_from_phase,omitempty"` // Which phase was resumed from
}

// SpecKitOrchestrator orchestrates the SpecKit development flow
type SpecKitOrchestrator struct {
	debateService       *DebateService
	constitutionManager *ConstitutionManager
	documentationSync   *DocumentationSync
	logger              *logrus.Logger
	projectRoot         string
	phaseDebateRounds   map[SpecKitPhase]int
	phaseTimeouts       map[SpecKitPhase]time.Duration
	cacheDir            string // Directory for caching phase results
	enableCaching       bool   // Enable phase caching for resumption
}

// NewSpecKitOrchestrator creates a new SpecKit orchestrator
func NewSpecKitOrchestrator(
	debateService *DebateService,
	constitutionManager *ConstitutionManager,
	documentationSync *DocumentationSync,
	logger *logrus.Logger,
	projectRoot string,
) *SpecKitOrchestrator {
	cacheDir := filepath.Join(projectRoot, ".speckit", "cache")

	return &SpecKitOrchestrator{
		debateService:       debateService,
		constitutionManager: constitutionManager,
		documentationSync:   documentationSync,
		logger:              logger,
		projectRoot:         projectRoot,
		cacheDir:            cacheDir,
		enableCaching:       true, // Enable by default
		phaseDebateRounds: map[SpecKitPhase]int{
			PhaseConstitution: 5, // Deep analysis for Constitution
			PhaseSpecify:      3, // Specification debate
			PhaseClarify:      3, // Clarification debate
			PhasePlan:         4, // Planning debate
			PhaseTasks:        2, // Task breakdown
			PhaseAnalyze:      4, // Analysis debate
			PhaseImplement:    6, // Implementation debate
		},
		phaseTimeouts: map[SpecKitPhase]time.Duration{
			PhaseConstitution: 15 * time.Minute,
			PhaseSpecify:      10 * time.Minute,
			PhaseClarify:      10 * time.Minute,
			PhasePlan:         12 * time.Minute,
			PhaseTasks:        8 * time.Minute,
			PhaseAnalyze:      12 * time.Minute,
			PhaseImplement:    20 * time.Minute,
		},
	}
}

// ExecuteFlow executes the complete SpecKit flow
func (so *SpecKitOrchestrator) ExecuteFlow(ctx context.Context, userRequest string, intentResult *EnhancedIntentResult) (*SpecKitFlowResult, error) {
	flowID := fmt.Sprintf("speckit-%d", time.Now().UnixNano())
	startTime := time.Now()

	so.logger.WithFields(logrus.Fields{
		"flow_id":     flowID,
		"granularity": intentResult.Granularity,
		"action_type": intentResult.ActionType,
	}).Info("[SpecKit] Starting SpecKit flow")

	result := &SpecKitFlowResult{
		FlowID:       flowID,
		StartTime:    startTime,
		Success:      false,
		PhaseResults: []SpecKitPhaseResult{},
		Metadata: map[string]interface{}{
			"user_request":  userRequest,
			"intent_result": intentResult,
		},
	}

	// Phase 1: Constitution
	constitutionResult, err := so.executeConstitutionPhase(ctx, userRequest, intentResult)
	result.PhaseResults = append(result.PhaseResults, *constitutionResult)
	if err != nil {
		result.Success = false
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("constitution phase failed: %w", err)
	}
	if err := so.savePhaseToCache(flowID, PhaseConstitution, constitutionResult); err != nil {
		so.logger.WithError(err).Warn("[SpecKit] Failed to cache phase result")
	}
	//nolint:errcheck // artifact type guaranteed by phase execution
	result.Constitution = constitutionResult.Artifacts["constitution"].(*Constitution)

	// Phase 2: Specify
	specifyResult, err := so.executeSpecifyPhase(ctx, userRequest, result.Constitution)
	result.PhaseResults = append(result.PhaseResults, *specifyResult)
	if err != nil {
		result.Success = false
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("specify phase failed: %w", err)
	}
	if err := so.savePhaseToCache(flowID, PhaseSpecify, specifyResult); err != nil {
		so.logger.WithError(err).Warn("[SpecKit] Failed to cache phase result")
	}

	// Phase 3: Clarify
	clarifyResult, err := so.executeClarifyPhase(ctx, userRequest, result.Constitution, specifyResult)
	result.PhaseResults = append(result.PhaseResults, *clarifyResult)
	if err != nil {
		result.Success = false
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("clarify phase failed: %w", err)
	}
	if err := so.savePhaseToCache(flowID, PhaseClarify, clarifyResult); err != nil {
		so.logger.WithError(err).Warn("[SpecKit] Failed to cache phase result")
	}

	// Phase 4: Plan
	planResult, err := so.executePlanPhase(ctx, userRequest, result.Constitution, clarifyResult)
	result.PhaseResults = append(result.PhaseResults, *planResult)
	if err != nil {
		result.Success = false
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("plan phase failed: %w", err)
	}
	if err := so.savePhaseToCache(flowID, PhasePlan, planResult); err != nil {
		so.logger.WithError(err).Warn("[SpecKit] Failed to cache phase result")
	}

	// Phase 5: Tasks
	tasksResult, err := so.executeTasksPhase(ctx, planResult)
	result.PhaseResults = append(result.PhaseResults, *tasksResult)
	if err != nil {
		result.Success = false
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("tasks phase failed: %w", err)
	}
	if err := so.savePhaseToCache(flowID, PhaseTasks, tasksResult); err != nil {
		so.logger.WithError(err).Warn("[SpecKit] Failed to cache phase result")
	}

	// Phase 6: Analyze
	analyzeResult, err := so.executeAnalyzePhase(ctx, result.Constitution, tasksResult)
	result.PhaseResults = append(result.PhaseResults, *analyzeResult)
	if err != nil {
		result.Success = false
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("analyze phase failed: %w", err)
	}
	if err := so.savePhaseToCache(flowID, PhaseAnalyze, analyzeResult); err != nil {
		so.logger.WithError(err).Warn("[SpecKit] Failed to cache phase result")
	}

	// Phase 7: Implement
	implementResult, err := so.executeImplementPhase(ctx, result.Constitution, analyzeResult, tasksResult)
	result.PhaseResults = append(result.PhaseResults, *implementResult)
	if err != nil {
		result.Success = false
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("implement phase failed: %w", err)
	}
	if err := so.savePhaseToCache(flowID, PhaseImplement, implementResult); err != nil {
		so.logger.WithError(err).Warn("[SpecKit] Failed to cache phase result")
	}

	result.Success = true
	result.EndTime = time.Now()
	result.Duration = time.Since(startTime)

	if err := so.saveFlowToCache(result); err != nil {
		so.logger.WithError(err).Warn("[SpecKit] Failed to cache flow result")
	}

	so.logger.WithFields(logrus.Fields{
		"flow_id":  flowID,
		"duration": result.Duration,
		"success":  result.Success,
	}).Info("[SpecKit] Completed SpecKit flow")

	return result, nil
}

// executeConstitutionPhase executes the Constitution phase
func (so *SpecKitOrchestrator) executeConstitutionPhase(ctx context.Context, userRequest string, intentResult *EnhancedIntentResult) (*SpecKitPhaseResult, error) {
	phaseResult := &SpecKitPhaseResult{
		Phase:     PhaseConstitution,
		StartTime: time.Now(),
		Success:   false,
	}

	so.logger.Info("[SpecKit:Constitution] Starting Constitution phase")

	// Load existing Constitution or create new one
	constitution, err := so.constitutionManager.LoadOrCreateConstitution(ctx, so.projectRoot)
	if err != nil {
		phaseResult.Error = fmt.Sprintf("failed to load/create constitution: %v", err)
		phaseResult.EndTime = time.Now()
		phaseResult.Duration = time.Since(phaseResult.StartTime)
		return phaseResult, err
	}

	// Build debate topic for Constitution analysis
	topic := so.buildConstitutionDebateTopic(userRequest, intentResult, constitution)

	// Execute debate for Constitution
	debateCtx, cancel := context.WithTimeout(ctx, so.phaseTimeouts[PhaseConstitution])
	defer cancel()

	debateConfig := &DebateConfig{
		Topic:     topic,
		MaxRounds: so.phaseDebateRounds[PhaseConstitution],
		Metadata: map[string]any{
			"phase":        string(PhaseConstitution),
			"user_request": userRequest,
		},
	}

	debateResult, err := so.debateService.ConductDebate(debateCtx, debateConfig)
	if err != nil {
		phaseResult.Error = fmt.Sprintf("debate failed: %v", err)
		phaseResult.EndTime = time.Now()
		phaseResult.Duration = time.Since(phaseResult.StartTime)
		return phaseResult, err
	}

	// Update Constitution based on debate results
	updatedConstitution, err := so.constitutionManager.UpdateConstitutionFromDebate(constitution, debateResult, userRequest)
	if err != nil {
		phaseResult.Error = fmt.Sprintf("failed to update constitution: %v", err)
		phaseResult.EndTime = time.Now()
		phaseResult.Duration = time.Since(phaseResult.StartTime)
		return phaseResult, err
	}

	// Save updated Constitution
	if err := so.constitutionManager.SaveConstitution(so.projectRoot, updatedConstitution); err != nil {
		phaseResult.Error = fmt.Sprintf("failed to save constitution: %v", err)
		phaseResult.EndTime = time.Now()
		phaseResult.Duration = time.Since(phaseResult.StartTime)
		return phaseResult, err
	}

	// Sync with documentation
	if err := so.documentationSync.SyncConstitutionToDocumentation(so.projectRoot, updatedConstitution); err != nil {
		so.logger.Warnf("[SpecKit:Constitution] Failed to sync documentation: %v", err)
	}

	phaseResult.Success = true
	phaseResult.Output = extractBestResponse(debateResult)
	phaseResult.DebateID = debateResult.DebateID
	phaseResult.QualityScore = debateResult.QualityScore
	phaseResult.Artifacts = map[string]interface{}{
		"constitution": updatedConstitution,
		"debate":       debateResult,
	}
	phaseResult.EndTime = time.Now()
	phaseResult.Duration = time.Since(phaseResult.StartTime)

	so.logger.WithFields(logrus.Fields{
		"duration":      phaseResult.Duration,
		"quality_score": phaseResult.QualityScore,
	}).Info("[SpecKit:Constitution] Completed Constitution phase")

	return phaseResult, nil
}

// executeSpecifyPhase executes the Specify phase
func (so *SpecKitOrchestrator) executeSpecifyPhase(ctx context.Context, userRequest string, constitution *Constitution) (*SpecKitPhaseResult, error) {
	phaseResult := &SpecKitPhaseResult{
		Phase:     PhaseSpecify,
		StartTime: time.Now(),
		Success:   false,
	}

	so.logger.Info("[SpecKit:Specify] Starting Specify phase")

	// Build specification debate topic
	topic := fmt.Sprintf(`Based on the user request and project Constitution, create a detailed specification:

User Request: %s

Constitution Summary:
%s

Create a comprehensive specification that:
1. Defines all functional requirements
2. Defines all non-functional requirements (performance, security, scalability)
3. Identifies all affected components and modules
4. Specifies interfaces and APIs
5. Defines data structures and models
6. Lists dependencies and constraints
7. Specifies testing requirements (all test types)
8. Defines documentation requirements
9. Ensures compliance with Constitution principles

Output a structured specification document.`, userRequest, constitution.Summary)

	// Execute debate
	debateCtx, cancel := context.WithTimeout(ctx, so.phaseTimeouts[PhaseSpecify])
	defer cancel()

	debateConfig := &DebateConfig{
		Topic:     topic,
		MaxRounds: so.phaseDebateRounds[PhaseSpecify],
		Metadata: map[string]any{
			"phase": string(PhaseSpecify),
		},
	}

	debateResult, err := so.debateService.ConductDebate(debateCtx, debateConfig)
	if err != nil {
		phaseResult.Error = fmt.Sprintf("debate failed: %v", err)
		phaseResult.EndTime = time.Now()
		phaseResult.Duration = time.Since(phaseResult.StartTime)
		return phaseResult, err
	}

	phaseResult.Success = true
	phaseResult.Output = extractBestResponse(debateResult)
	phaseResult.DebateID = debateResult.DebateID
	phaseResult.QualityScore = debateResult.QualityScore
	phaseResult.Artifacts = map[string]interface{}{
		"specification": extractBestResponse(debateResult),
		"debate":        debateResult,
	}
	phaseResult.EndTime = time.Now()
	phaseResult.Duration = time.Since(phaseResult.StartTime)

	so.logger.WithFields(logrus.Fields{
		"duration":      phaseResult.Duration,
		"quality_score": phaseResult.QualityScore,
	}).Info("[SpecKit:Specify] Completed Specify phase")

	return phaseResult, nil
}

// executeClarifyPhase executes the Clarify phase
func (so *SpecKitOrchestrator) executeClarifyPhase(ctx context.Context, userRequest string, constitution *Constitution, specifyResult *SpecKitPhaseResult) (*SpecKitPhaseResult, error) {
	phaseResult := &SpecKitPhaseResult{
		Phase:     PhaseClarify,
		StartTime: time.Now(),
		Success:   false,
	}

	so.logger.Info("[SpecKit:Clarify] Starting Clarify phase")

	//nolint:errcheck // artifact type guaranteed by phase execution
	specification := specifyResult.Artifacts["specification"].(string)

	topic := fmt.Sprintf(`Review and clarify the specification to ensure completeness and remove ambiguities:

Specification:
%s

Clarification tasks:
1. Identify any ambiguous requirements
2. Resolve conflicts or contradictions
3. Fill in missing details
4. Validate against Constitution principles
5. Ensure all edge cases are covered
6. Clarify technical approach and architecture
7. Validate that all mandatory Constitution points are addressed
8. Ensure 100%% test coverage strategy is defined
9. Verify documentation requirements are complete

Output a clarified and validated specification.`, specification)

	// Execute debate
	debateCtx, cancel := context.WithTimeout(ctx, so.phaseTimeouts[PhaseClarify])
	defer cancel()

	debateConfig := &DebateConfig{
		Topic:     topic,
		MaxRounds: so.phaseDebateRounds[PhaseClarify],
		Metadata: map[string]any{
			"phase": string(PhaseClarify),
		},
	}

	debateResult, err := so.debateService.ConductDebate(debateCtx, debateConfig)
	if err != nil {
		phaseResult.Error = fmt.Sprintf("debate failed: %v", err)
		phaseResult.EndTime = time.Now()
		phaseResult.Duration = time.Since(phaseResult.StartTime)
		return phaseResult, err
	}

	phaseResult.Success = true
	phaseResult.Output = extractBestResponse(debateResult)
	phaseResult.DebateID = debateResult.DebateID
	phaseResult.QualityScore = debateResult.QualityScore
	phaseResult.Artifacts = map[string]interface{}{
		"clarified_spec": extractBestResponse(debateResult),
		"debate":         debateResult,
	}
	phaseResult.EndTime = time.Now()
	phaseResult.Duration = time.Since(phaseResult.StartTime)

	so.logger.WithFields(logrus.Fields{
		"duration":      phaseResult.Duration,
		"quality_score": phaseResult.QualityScore,
	}).Info("[SpecKit:Clarify] Completed Clarify phase")

	return phaseResult, nil
}

// executePlanPhase executes the Plan phase
func (so *SpecKitOrchestrator) executePlanPhase(ctx context.Context, userRequest string, constitution *Constitution, clarifyResult *SpecKitPhaseResult) (*SpecKitPhaseResult, error) {
	phaseResult := &SpecKitPhaseResult{
		Phase:     PhasePlan,
		StartTime: time.Now(),
		Success:   false,
	}

	so.logger.Info("[SpecKit:Plan] Starting Plan phase")

	//nolint:errcheck // artifact type guaranteed by phase execution
	clarifiedSpec := clarifyResult.Artifacts["clarified_spec"].(string)

	topic := fmt.Sprintf(`Create a comprehensive implementation plan based on the clarified specification:

Clarified Specification:
%s

Planning requirements:
1. Divide work into logical phases
2. Identify all components to be created/modified
3. Define dependencies between components
4. Specify testing strategy for each component (unit, integration, E2E, security, stress, chaos, automation, benchmark)
5. Define challenge scripts for each component
6. Plan documentation deliverables (README, CLAUDE.md, AGENTS.md, user guides, manuals, video courses, diagrams, SQL definitions, website)
7. Identify potential risks and mitigation strategies
8. Estimate complexity and effort for each phase
9. Define checkpoints for progress tracking
10. Plan for module extraction and decoupling (if applicable)
11. Plan for monitoring and metrics
12. Plan for Snyk/SonarQube scanning integration

Output a detailed, phased implementation plan with tasks, dependencies, and deliverables.`, clarifiedSpec)

	// Execute debate
	debateCtx, cancel := context.WithTimeout(ctx, so.phaseTimeouts[PhasePlan])
	defer cancel()

	debateConfig := &DebateConfig{
		Topic:     topic,
		MaxRounds: so.phaseDebateRounds[PhasePlan],
		Metadata: map[string]any{
			"phase": string(PhasePlan),
		},
	}

	debateResult, err := so.debateService.ConductDebate(debateCtx, debateConfig)
	if err != nil {
		phaseResult.Error = fmt.Sprintf("debate failed: %v", err)
		phaseResult.EndTime = time.Now()
		phaseResult.Duration = time.Since(phaseResult.StartTime)
		return phaseResult, err
	}

	phaseResult.Success = true
	phaseResult.Output = extractBestResponse(debateResult)
	phaseResult.DebateID = debateResult.DebateID
	phaseResult.QualityScore = debateResult.QualityScore
	phaseResult.Artifacts = map[string]interface{}{
		"plan":   extractBestResponse(debateResult),
		"debate": debateResult,
	}
	phaseResult.EndTime = time.Now()
	phaseResult.Duration = time.Since(phaseResult.StartTime)

	so.logger.WithFields(logrus.Fields{
		"duration":      phaseResult.Duration,
		"quality_score": phaseResult.QualityScore,
	}).Info("[SpecKit:Plan] Completed Plan phase")

	return phaseResult, nil
}

// executeTasksPhase executes the Tasks phase
func (so *SpecKitOrchestrator) executeTasksPhase(ctx context.Context, planResult *SpecKitPhaseResult) (*SpecKitPhaseResult, error) {
	phaseResult := &SpecKitPhaseResult{
		Phase:     PhaseTasks,
		StartTime: time.Now(),
		Success:   false,
	}

	so.logger.Info("[SpecKit:Tasks] Starting Tasks phase")

	//nolint:errcheck // artifact type guaranteed by phase execution
	plan := planResult.Artifacts["plan"].(string)

	topic := fmt.Sprintf(`Break down the implementation plan into discrete, actionable tasks:

Implementation Plan:
%s

Task breakdown requirements:
1. Create atomic tasks (each task should be completable independently)
2. Assign priority to each task (Critical, High, Medium, Low)
3. Estimate effort for each task
4. Define dependencies between tasks
5. Specify acceptance criteria for each task
6. Define testing requirements per task
7. Define documentation requirements per task
8. Group tasks by phase
9. Create a task execution order that respects dependencies
10. Include checkpoint tasks for progress tracking

Output a structured task list with all details in JSON format.`, plan)

	// Execute debate
	debateCtx, cancel := context.WithTimeout(ctx, so.phaseTimeouts[PhaseTasks])
	defer cancel()

	debateConfig := &DebateConfig{
		Topic:     topic,
		MaxRounds: so.phaseDebateRounds[PhaseTasks],
		Metadata: map[string]any{
			"phase": string(PhaseTasks),
		},
	}

	debateResult, err := so.debateService.ConductDebate(debateCtx, debateConfig)
	if err != nil {
		phaseResult.Error = fmt.Sprintf("debate failed: %v", err)
		phaseResult.EndTime = time.Now()
		phaseResult.Duration = time.Since(phaseResult.StartTime)
		return phaseResult, err
	}

	// Try to parse tasks as JSON
	var tasks []map[string]interface{}
	taskJSON := so.extractJSON(extractBestResponse(debateResult))
	if err := json.Unmarshal([]byte(taskJSON), &tasks); err != nil {
		so.logger.Warnf("[SpecKit:Tasks] Could not parse tasks as JSON: %v", err)
		// Still succeed, just store as string
	}

	phaseResult.Success = true
	phaseResult.Output = extractBestResponse(debateResult)
	phaseResult.DebateID = debateResult.DebateID
	phaseResult.QualityScore = debateResult.QualityScore
	phaseResult.Artifacts = map[string]interface{}{
		"tasks":     tasks,
		"tasks_raw": extractBestResponse(debateResult),
		"debate":    debateResult,
	}
	phaseResult.EndTime = time.Now()
	phaseResult.Duration = time.Since(phaseResult.StartTime)

	so.logger.WithFields(logrus.Fields{
		"duration":      phaseResult.Duration,
		"quality_score": phaseResult.QualityScore,
		"task_count":    len(tasks),
	}).Info("[SpecKit:Tasks] Completed Tasks phase")

	return phaseResult, nil
}

// executeAnalyzePhase executes the Analyze phase
func (so *SpecKitOrchestrator) executeAnalyzePhase(ctx context.Context, constitution *Constitution, tasksResult *SpecKitPhaseResult) (*SpecKitPhaseResult, error) {
	phaseResult := &SpecKitPhaseResult{
		Phase:     PhaseAnalyze,
		StartTime: time.Now(),
		Success:   false,
	}

	so.logger.Info("[SpecKit:Analyze] Starting Analyze phase")

	//nolint:errcheck // artifact type guaranteed by phase execution
	tasksRaw := tasksResult.Artifacts["tasks_raw"].(string)

	topic := fmt.Sprintf(`Perform comprehensive analysis before implementation:

Tasks:
%s

Constitution Requirements:
%s

Analysis requirements:
1. Review current codebase structure and identify affected files
2. Analyze potential impact on existing functionality
3. Identify risks (breaking changes, performance, security, memory leaks, race conditions, deadlocks)
4. Validate compliance with Constitution principles
5. Identify opportunities for module extraction and decoupling
6. Analyze testing strategy completeness
7. Review documentation requirements
8. Identify potential design patterns to apply
9. Analyze lazy loading and non-blocking opportunities
10. Validate monitoring and metrics strategy
11. Identify Snyk/SonarQube scanning requirements
12. Ensure no dead code will be introduced

Output a comprehensive analysis report with findings and recommendations.`, tasksRaw, constitution.Summary)

	// Execute debate
	debateCtx, cancel := context.WithTimeout(ctx, so.phaseTimeouts[PhaseAnalyze])
	defer cancel()

	debateConfig := &DebateConfig{
		Topic:     topic,
		MaxRounds: so.phaseDebateRounds[PhaseAnalyze],
		Metadata: map[string]any{
			"phase": string(PhaseAnalyze),
		},
	}

	debateResult, err := so.debateService.ConductDebate(debateCtx, debateConfig)
	if err != nil {
		phaseResult.Error = fmt.Sprintf("debate failed: %v", err)
		phaseResult.EndTime = time.Now()
		phaseResult.Duration = time.Since(phaseResult.StartTime)
		return phaseResult, err
	}

	phaseResult.Success = true
	phaseResult.Output = extractBestResponse(debateResult)
	phaseResult.DebateID = debateResult.DebateID
	phaseResult.QualityScore = debateResult.QualityScore
	phaseResult.Artifacts = map[string]interface{}{
		"analysis": extractBestResponse(debateResult),
		"debate":   debateResult,
	}
	phaseResult.EndTime = time.Now()
	phaseResult.Duration = time.Since(phaseResult.StartTime)

	so.logger.WithFields(logrus.Fields{
		"duration":      phaseResult.Duration,
		"quality_score": phaseResult.QualityScore,
	}).Info("[SpecKit:Analyze] Completed Analyze phase")

	return phaseResult, nil
}

// executeImplementPhase executes the Implement phase
func (so *SpecKitOrchestrator) executeImplementPhase(ctx context.Context, constitution *Constitution, analyzeResult *SpecKitPhaseResult, tasksResult *SpecKitPhaseResult) (*SpecKitPhaseResult, error) {
	phaseResult := &SpecKitPhaseResult{
		Phase:     PhaseImplement,
		StartTime: time.Now(),
		Success:   false,
	}

	so.logger.Info("[SpecKit:Implement] Starting Implement phase")

	//nolint:errcheck // artifact type guaranteed by phase execution
	analysis := analyzeResult.Artifacts["analysis"].(string)
	//nolint:errcheck // artifact type guaranteed by phase execution
	tasksRaw := tasksResult.Artifacts["tasks_raw"].(string)

	topic := fmt.Sprintf(`Execute implementation based on analysis and tasks:

Analysis Report:
%s

Tasks:
%s

Constitution Requirements:
%s

Implementation requirements:
1. Implement all tasks in dependency order
2. Follow all Constitution principles
3. Apply identified design patterns
4. Implement comprehensive error handling
5. Add logging and monitoring
6. Implement lazy loading and non-blocking where identified
7. Add semaphore mechanisms where needed
8. Ensure rock-solid implementation (no breaking changes)
9. Implement 100%% test coverage (all test types: unit, integration, E2E, security, stress, chaos, automation, benchmark)
10. Create comprehensive challenge scripts
11. Add complete documentation (code comments, README, CLAUDE.md, AGENTS.md, user guides, manuals, diagrams, SQL definitions)
12. Ensure no dead code
13. Run Snyk/SonarQube scanning and fix all findings
14. Validate memory safety (no leaks, no deadlocks, no race conditions)
15. Ensure flawless responsiveness

Output implementation details including:
- Files created/modified
- Code changes summary
- Tests added
- Challenges created
- Documentation updated
- Verification steps
- Any issues encountered and resolutions`, analysis, tasksRaw, constitution.Summary)

	// Execute debate with extended rounds for implementation
	debateCtx, cancel := context.WithTimeout(ctx, so.phaseTimeouts[PhaseImplement])
	defer cancel()

	debateConfig := &DebateConfig{
		Topic:     topic,
		MaxRounds: so.phaseDebateRounds[PhaseImplement],
		Metadata: map[string]any{
			"phase": string(PhaseImplement),
		},
	}

	debateResult, err := so.debateService.ConductDebate(debateCtx, debateConfig)
	if err != nil {
		phaseResult.Error = fmt.Sprintf("debate failed: %v", err)
		phaseResult.EndTime = time.Now()
		phaseResult.Duration = time.Since(phaseResult.StartTime)
		return phaseResult, err
	}

	phaseResult.Success = true
	phaseResult.Output = extractBestResponse(debateResult)
	phaseResult.DebateID = debateResult.DebateID
	phaseResult.QualityScore = debateResult.QualityScore
	phaseResult.Artifacts = map[string]interface{}{
		"implementation": extractBestResponse(debateResult),
		"debate":         debateResult,
	}
	phaseResult.EndTime = time.Now()
	phaseResult.Duration = time.Since(phaseResult.StartTime)

	so.logger.WithFields(logrus.Fields{
		"duration":      phaseResult.Duration,
		"quality_score": phaseResult.QualityScore,
	}).Info("[SpecKit:Implement] Completed Implement phase")

	return phaseResult, nil
}

// buildConstitutionDebateTopic builds the debate topic for Constitution phase
func (so *SpecKitOrchestrator) buildConstitutionDebateTopic(userRequest string, intentResult *EnhancedIntentResult, existingConstitution *Constitution) string {
	existingRules := "None (creating new Constitution)"
	if len(existingConstitution.Rules) > 0 {
		existingRules = fmt.Sprintf("%d existing rules", len(existingConstitution.Rules))
	}

	return fmt.Sprintf(`Analyze the user request and create or update the project Constitution:

User Request: %s

Request Classification:
- Granularity: %s (score: %.2f)
- Action Type: %s (score: %.2f)
- Estimated Scope: %s

Existing Constitution: %s

Review all project documentation:
- AGENTS.md: %s
- CLAUDE.md: %s
- Technical docs in docs/
- Existing AI request documents in docs/requests/

Constitution Tasks:
1. Analyze if existing Constitution adequately covers this request
2. Identify any new principles needed for this specific work
3. Ensure all mandatory Constitution points are included:
   - Comprehensive decoupling and module extraction
   - Each module: separate project with CLAUDE.md, AGENTS.md, README.md, docs/, 100%% tests, challenges
   - All software principles (KISS, DRY, SOLID, etc.)
   - Design patterns (proxy, facade, factory, observer, mediator, etc.)
   - 100%% test coverage (ALL test types: unit, integration, E2E, security, stress, chaos, automation, benchmark)
   - Comprehensive challenges for real-world validation
   - Complete documentation (user guides, manuals, video courses, diagrams, SQL definitions, website)
   - No broken/disabled modules
   - No dead code
   - Memory safety (no leaks, deadlocks, race conditions)
   - Snyk/SonarQube scanning with issue resolution
   - Monitoring and metrics
   - Lazy loading, lazy initialization, semaphores, non-blocking mechanisms
   - Rock-solid changes (no breaking existing functionality)
   - GitSpec constitution compliance
   - NO GitHub Actions (manual CI/CD only)
   - Sync with AGENTS.md and CLAUDE.md

Output the complete Constitution including existing rules and any new/updated rules.`,
		userRequest,
		intentResult.Granularity,
		intentResult.GranularityScore,
		intentResult.ActionType,
		intentResult.ActionTypeScore,
		intentResult.EstimatedScope,
		existingRules,
		filepath.Join(so.projectRoot, "AGENTS.md"),
		filepath.Join(so.projectRoot, "CLAUDE.md"),
	)
}

// extractJSON extracts JSON from a response that might contain markdown
func (so *SpecKitOrchestrator) extractJSON(content string) string {
	content = strings.TrimSpace(content)

	// Check for ```json ... ```
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + len("```json")
		end := strings.LastIndex(content, "```")
		if end > start {
			content = content[start:end]
		}
	} else if strings.Contains(content, "```") {
		start := strings.Index(content, "```") + len("```")
		end := strings.LastIndex(content, "```")
		if end > start {
			content = content[start:end]
		}
	}

	return strings.TrimSpace(content)
}

// GetPhaseResult retrieves a specific phase result from a flow result
func GetPhaseResult(flowResult *SpecKitFlowResult, phase SpecKitPhase) *SpecKitPhaseResult {
	for _, phaseResult := range flowResult.PhaseResults {
		if phaseResult.Phase == phase {
			return &phaseResult
		}
	}
	return nil
}

// extractBestResponse extracts the best response content from a debate result
func extractBestResponse(debateResult *DebateResult) string {
	if debateResult == nil {
		return ""
	}

	// Try BestResponse first
	if debateResult.BestResponse != nil && debateResult.BestResponse.Content != "" {
		return debateResult.BestResponse.Content
	}

	// Fallback to first participant if available
	if len(debateResult.Participants) > 0 && debateResult.Participants[0].Content != "" {
		return debateResult.Participants[0].Content
	}

	// Last resort: try AllResponses
	if len(debateResult.AllResponses) > 0 && debateResult.AllResponses[0].Content != "" {
		return debateResult.AllResponses[0].Content
	}

	return ""
}

// Phase Caching and Resumption Methods

// savePhaseToCache saves a phase result to the cache
func (so *SpecKitOrchestrator) savePhaseToCache(flowID string, phase SpecKitPhase, result *SpecKitPhaseResult) error {
	if !so.enableCaching {
		return nil
	}

	// Create cache directory if it doesn't exist
	cacheFilePath := filepath.Join(so.cacheDir, flowID)
	if err := ensureDir(cacheFilePath); err != nil {
		so.logger.WithError(err).Warn("[SpecKit Cache] Failed to create cache directory")
		return err
	}

	// Save phase result as JSON
	phaseFile := filepath.Join(cacheFilePath, fmt.Sprintf("%s.json", phase))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal phase result: %w", err)
	}

	if err := writeFile(phaseFile, data); err != nil {
		return fmt.Errorf("failed to write phase cache: %w", err)
	}

	so.logger.WithFields(logrus.Fields{
		"flow_id": flowID,
		"phase":   phase,
		"file":    phaseFile,
	}).Debug("[SpecKit Cache] Phase result cached")

	return nil
}

// loadPhaseFromCache loads a cached phase result
func (so *SpecKitOrchestrator) loadPhaseFromCache(flowID string, phase SpecKitPhase) (*SpecKitPhaseResult, error) {
	if !so.enableCaching {
		return nil, fmt.Errorf("caching disabled")
	}

	phaseFile := filepath.Join(so.cacheDir, flowID, fmt.Sprintf("%s.json", phase))
	data, err := readFile(phaseFile)
	if err != nil {
		return nil, fmt.Errorf("phase cache not found: %w", err)
	}

	var result SpecKitPhaseResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal phase cache: %w", err)
	}

	result.Cached = true

	so.logger.WithFields(logrus.Fields{
		"flow_id": flowID,
		"phase":   phase,
	}).Info("[SpecKit Cache] Loaded phase from cache")

	return &result, nil
}

// saveFlowToCache saves the complete flow result to cache
func (so *SpecKitOrchestrator) saveFlowToCache(result *SpecKitFlowResult) error {
	if !so.enableCaching {
		return nil
	}

	cacheFilePath := filepath.Join(so.cacheDir, result.FlowID)
	if err := ensureDir(cacheFilePath); err != nil {
		return err
	}

	flowFile := filepath.Join(cacheFilePath, "flow.json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal flow result: %w", err)
	}

	if err := writeFile(flowFile, data); err != nil {
		return fmt.Errorf("failed to write flow cache: %w", err)
	}

	so.logger.WithFields(logrus.Fields{
		"flow_id":       result.FlowID,
		"phases_cached": len(result.PhaseResults),
	}).Debug("[SpecKit Cache] Flow result cached")

	return nil
}

// loadFlowFromCache loads a complete flow result from cache
func (so *SpecKitOrchestrator) loadFlowFromCache(flowID string) (*SpecKitFlowResult, error) {
	if !so.enableCaching {
		return nil, fmt.Errorf("caching disabled")
	}

	flowFile := filepath.Join(so.cacheDir, flowID, "flow.json")
	data, err := readFile(flowFile)
	if err != nil {
		return nil, fmt.Errorf("flow cache not found: %w", err)
	}

	var result SpecKitFlowResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow cache: %w", err)
	}

	result.ResumedFromCache = true

	so.logger.WithFields(logrus.Fields{
		"flow_id":       flowID,
		"phases_loaded": len(result.PhaseResults),
		"last_phase":    result.PhaseResults[len(result.PhaseResults)-1].Phase,
	}).Info("[SpecKit Cache] Loaded flow from cache")

	return &result, nil
}

// clearFlowCache clears all cached data for a flow
func (so *SpecKitOrchestrator) clearFlowCache(flowID string) error {
	if !so.enableCaching {
		return nil
	}

	cacheFilePath := filepath.Join(so.cacheDir, flowID)
	if err := removeDir(cacheFilePath); err != nil {
		return fmt.Errorf("failed to clear flow cache: %w", err)
	}

	so.logger.WithField("flow_id", flowID).Debug("[SpecKit Cache] Cleared flow cache")
	return nil
}

// resumeFlow resumes a SpecKit flow from the last cached phase
func (so *SpecKitOrchestrator) resumeFlow(ctx context.Context, flowID string, userRequest string, intentResult *EnhancedIntentResult) (*SpecKitFlowResult, error) {
	// Try to load cached flow
	cachedFlow, err := so.loadFlowFromCache(flowID)
	if err != nil {
		return nil, fmt.Errorf("cannot resume flow: %w", err)
	}

	if cachedFlow.Success {
		so.logger.Info("[SpecKit Resumption] Flow already completed, returning cached result")
		return cachedFlow, nil
	}

	// Determine where to resume from
	lastPhase := cachedFlow.PhaseResults[len(cachedFlow.PhaseResults)-1].Phase
	so.logger.WithFields(logrus.Fields{
		"flow_id":    flowID,
		"last_phase": lastPhase,
	}).Info("[SpecKit Resumption] Resuming flow from last completed phase")

	// Determine remaining phases
	allPhases := []SpecKitPhase{
		PhaseConstitution, PhaseSpecify, PhaseClarify,
		PhasePlan, PhaseTasks, PhaseAnalyze, PhaseImplement,
	}

	// Find index of last completed phase
	lastPhaseIndex := -1
	for i, phase := range allPhases {
		if phase == lastPhase {
			lastPhaseIndex = i
			break
		}
	}

	if lastPhaseIndex == -1 {
		return nil, fmt.Errorf("unknown last phase: %s", lastPhase)
	}

	// If all phases complete, return cached flow
	if lastPhaseIndex == len(allPhases)-1 {
		so.logger.Info("[SpecKit Resumption] All phases already completed")
		cachedFlow.ResumedFromPhase = lastPhase
		return cachedFlow, nil
	}

	// Execute remaining phases
	remainingPhases := allPhases[lastPhaseIndex+1:]
	so.logger.WithFields(logrus.Fields{
		"remaining_phases": len(remainingPhases),
		"phases":           remainingPhases,
	}).Info("[SpecKit Resumption] Executing remaining phases")

	// Extract context from cached phases
	constitution := cachedFlow.Constitution
	var specifyResult, clarifyResult, planResult, tasksResult, analyzeResult *SpecKitPhaseResult

	// Build context from cached phases
	cachedFlow.Phases = make(map[string]*SpecKitPhaseResult)
	for i := range cachedFlow.PhaseResults {
		phase := &cachedFlow.PhaseResults[i]
		cachedFlow.Phases[string(phase.Phase)] = phase

		switch phase.Phase {
		case PhaseSpecify:
			specifyResult = phase
		case PhaseClarify:
			clarifyResult = phase
		case PhasePlan:
			planResult = phase
		case PhaseTasks:
			tasksResult = phase
		case PhaseAnalyze:
			analyzeResult = phase
		}
	}

	// Execute remaining phases
	for _, phase := range remainingPhases {
		var phaseResult *SpecKitPhaseResult
		var err error

		switch phase {
		case PhaseConstitution:
			// Should never resume from before Constitution
			return nil, fmt.Errorf("cannot resume before Constitution phase")

		case PhaseSpecify:
			phaseResult, err = so.executeSpecifyPhase(ctx, userRequest, constitution)

		case PhaseClarify:
			phaseResult, err = so.executeClarifyPhase(ctx, userRequest, constitution, specifyResult)

		case PhasePlan:
			phaseResult, err = so.executePlanPhase(ctx, userRequest, constitution, clarifyResult)

		case PhaseTasks:
			phaseResult, err = so.executeTasksPhase(ctx, planResult)

		case PhaseAnalyze:
			phaseResult, err = so.executeAnalyzePhase(ctx, constitution, tasksResult)

		case PhaseImplement:
			phaseResult, err = so.executeImplementPhase(ctx, constitution, analyzeResult, tasksResult)
		}

		if err != nil {
			so.logger.WithError(err).WithField("phase", phase).Error("[SpecKit Resumption] Phase execution failed")
			cachedFlow.Success = false
			cachedFlow.EndTime = time.Now()
			cachedFlow.Duration = cachedFlow.EndTime.Sub(cachedFlow.StartTime)

			// Save partial progress
			if saveErr := so.saveFlowToCache(cachedFlow); saveErr != nil {
				so.logger.WithError(saveErr).Warn("[SpecKit Resumption] Failed to save partial progress")
			}

			return cachedFlow, fmt.Errorf("phase %s failed during resumption: %w", phase, err)
		}

		// Add result to flow
		cachedFlow.PhaseResults = append(cachedFlow.PhaseResults, *phaseResult)
		cachedFlow.Phases[string(phase)] = phaseResult

		// Update context for next phases
		switch phase {
		case PhaseSpecify:
			specifyResult = phaseResult
		case PhaseClarify:
			clarifyResult = phaseResult
		case PhasePlan:
			planResult = phaseResult
		case PhaseTasks:
			tasksResult = phaseResult
		case PhaseAnalyze:
			analyzeResult = phaseResult
		}

		// Save progress after each phase
		if err := so.savePhaseToCache(flowID, phase, phaseResult); err != nil {
			so.logger.WithError(err).Warn("[SpecKit Resumption] Failed to cache phase result")
		}

		so.logger.WithField("phase", phase).Info("[SpecKit Resumption] Phase completed")
	}

	// Mark flow as complete
	cachedFlow.Success = true
	cachedFlow.EndTime = time.Now()
	cachedFlow.Duration = cachedFlow.EndTime.Sub(cachedFlow.StartTime)
	cachedFlow.ResumedFromPhase = lastPhase

	// Calculate overall quality score
	totalScore := 0.0
	for _, phaseResult := range cachedFlow.PhaseResults {
		totalScore += phaseResult.QualityScore
	}
	cachedFlow.OverallQualityScore = totalScore / float64(len(cachedFlow.PhaseResults))

	// Extract final artifact (implementation output)
	if implementPhase, ok := cachedFlow.Phases[string(PhaseImplement)]; ok {
		cachedFlow.FinalArtifact = implementPhase.Artifact
	}

	// Save completed flow
	if err := so.saveFlowToCache(cachedFlow); err != nil {
		so.logger.WithError(err).Warn("[SpecKit Resumption] Failed to save completed flow")
	}

	so.logger.WithFields(logrus.Fields{
		"flow_id":         flowID,
		"phases_resumed":  len(remainingPhases),
		"overall_quality": cachedFlow.OverallQualityScore,
	}).Info("[SpecKit Resumption] Flow completed successfully")

	return cachedFlow, nil
}

// Helper functions for file operations

func ensureDir(dir string) error {
	// #nosec G301 -- speckit cache directories use standard 0750 permissions
	return os.MkdirAll(dir, 0750)
}

func writeFile(path string, data []byte) error {
	// #nosec G306 -- speckit phase cache files use standard 0600 permissions
	return os.WriteFile(path, data, 0600)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func removeDir(dir string) error {
	return os.RemoveAll(dir)
}
