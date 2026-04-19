package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/sirupsen/logrus"

	"digital.vasic.concurrency/pkg/safe"
)

// ============================================================================
// Multi-Pass Validation System for AI Debate Ensemble
// ============================================================================
// Phases:
//   1. INITIAL RESPONSE   - Each participant provides their initial perspective
//   2. VALIDATION         - Cross-validation and fact-checking of responses
//   3. POLISH & IMPROVE   - Refinement based on validation feedback
//   4. FINAL CONCLUSION   - Synthesized consensus with confidence scores
// ============================================================================

// ValidationPhase represents a phase in the multi-pass validation process
type ValidationPhase string

const (
	// PhaseInitialResponse is the first phase where participants provide initial responses
	PhaseInitialResponse ValidationPhase = "initial_response"
	// PhaseValidation is the second phase where responses are cross-validated
	PhaseValidation ValidationPhase = "validation"
	// PhasePolishImprove is the third phase where responses are refined
	PhasePolishImprove ValidationPhase = "polish_improve"
	// PhaseFinalConclusion is the final phase with synthesized consensus
	PhaseFinalConclusion ValidationPhase = "final_conclusion"
)

// PhaseInfo provides detailed information about each validation phase
type PhaseInfo struct {
	Phase       ValidationPhase `json:"phase"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Icon        string          `json:"icon"`
	Order       int             `json:"order"`
}

// ValidationPhases returns all phases in order
func ValidationPhases() []PhaseInfo {
	return []PhaseInfo{
		{
			Phase:       PhaseInitialResponse,
			Name:        "INITIAL RESPONSE",
			Description: "Each AI participant provides their initial perspective on the topic",
			Icon:        "🔍",
			Order:       1,
		},
		{
			Phase:       PhaseValidation,
			Name:        "VALIDATION",
			Description: "Cross-validation of responses to identify accuracy and completeness",
			Icon:        "✓",
			Order:       2,
		},
		{
			Phase:       PhasePolishImprove,
			Name:        "POLISH & IMPROVE",
			Description: "Refinement and improvement based on validation feedback",
			Icon:        "✨",
			Order:       3,
		},
		{
			Phase:       PhaseFinalConclusion,
			Name:        "FINAL CONCLUSION",
			Description: "Synthesized consensus with confidence scores",
			Icon:        "📜",
			Order:       4,
		},
	}
}

// GetPhaseInfo returns information about a specific phase
func GetPhaseInfo(phase ValidationPhase) *PhaseInfo {
	for _, info := range ValidationPhases() {
		if info.Phase == phase {
			return &info
		}
	}
	return nil
}

// ValidationConfig configures the multi-pass validation process
type ValidationConfig struct {
	EnableValidation    bool          `json:"enable_validation"`
	EnablePolish        bool          `json:"enable_polish"`
	ValidationTimeout   time.Duration `json:"validation_timeout"`
	PolishTimeout       time.Duration `json:"polish_timeout"`
	MinConfidenceToSkip float64       `json:"min_confidence_to_skip"` // Skip polish if initial confidence is high
	MaxValidationRounds int           `json:"max_validation_rounds"`
	ParallelValidation  bool          `json:"parallel_validation"`
	ShowPhaseIndicators bool          `json:"show_phase_indicators"`
	VerbosePhaseHeaders bool          `json:"verbose_phase_headers"`
}

// DefaultValidationConfig returns default validation configuration
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		EnableValidation:    true,
		EnablePolish:        true,
		ValidationTimeout:   30 * time.Second,
		PolishTimeout:       20 * time.Second,
		MinConfidenceToSkip: 0.95,
		MaxValidationRounds: 2,
		ParallelValidation:  true,
		ShowPhaseIndicators: true,
		VerbosePhaseHeaders: true,
	}
}

// ValidationResult represents the result of validating a response
type ValidationResult struct {
	ParticipantID     string            `json:"participant_id"`
	OriginalResponse  string            `json:"original_response"`
	ValidationScore   float64           `json:"validation_score"`
	FactualAccuracy   float64           `json:"factual_accuracy"`
	Completeness      float64           `json:"completeness"`
	Coherence         float64           `json:"coherence"`
	Issues            []ValidationIssue `json:"issues"`
	Suggestions       []string          `json:"suggestions"`
	ValidatorID       string            `json:"validator_id"`
	ValidatorProvider string            `json:"validator_provider"`
	ValidatorModel    string            `json:"validator_model"`
	Timestamp         time.Time         `json:"timestamp"`
	Duration          time.Duration     `json:"duration"`
}

// ValidationIssue represents an issue found during validation
type ValidationIssue struct {
	Type        IssueType          `json:"type"`
	Severity    ValidationSeverity `json:"severity"`
	Description string             `json:"description"`
	Location    string             `json:"location,omitempty"`
	Suggestion  string             `json:"suggestion,omitempty"`
}

// IssueType represents the type of validation issue
type IssueType string

const (
	IssueFactualError    IssueType = "factual_error"
	IssueIncomplete      IssueType = "incomplete"
	IssueUnclear         IssueType = "unclear"
	IssueContradiction   IssueType = "contradiction"
	IssueMissingContext  IssueType = "missing_context"
	IssueOverGeneralized IssueType = "over_generalized"
	IssueOutOfScope      IssueType = "out_of_scope"
)

// ValidationSeverity represents the severity of a validation issue
type ValidationSeverity string

const (
	ValidationSeverityCritical ValidationSeverity = "critical"
	ValidationSeverityMajor    ValidationSeverity = "major"
	ValidationSeverityMinor    ValidationSeverity = "minor"
	ValidationSeverityInfo     ValidationSeverity = "info"
)

// PolishResult represents the result of polishing a response
type PolishResult struct {
	ParticipantID    string            `json:"participant_id"`
	OriginalResponse string            `json:"original_response"`
	PolishedResponse string            `json:"polished_response"`
	ImprovementScore float64           `json:"improvement_score"`
	ChangesSummary   []string          `json:"changes_summary"`
	PolisherID       string            `json:"polisher_id"`
	PolisherProvider string            `json:"polisher_provider"`
	PolisherModel    string            `json:"polisher_model"`
	ValidationIssues []ValidationIssue `json:"validation_issues_addressed"`
	Timestamp        time.Time         `json:"timestamp"`
	Duration         time.Duration     `json:"duration"`
}

// PhaseResult represents the result of a complete validation phase
type PhaseResult struct {
	Phase        ValidationPhase       `json:"phase"`
	StartTime    time.Time             `json:"start_time"`
	EndTime      time.Time             `json:"end_time"`
	Duration     time.Duration         `json:"duration"`
	Responses    []ParticipantResponse `json:"responses"`
	Validations  []ValidationResult    `json:"validations,omitempty"`
	Polishes     []PolishResult        `json:"polishes,omitempty"`
	PhaseScore   float64               `json:"phase_score"`
	PhaseSummary string                `json:"phase_summary"`
}

// MultiPassResult represents the complete multi-pass validation result
type MultiPassResult struct {
	DebateID           string                 `json:"debate_id"`
	Topic              string                 `json:"topic"`
	Config             *ValidationConfig      `json:"config"`
	Phases             []*PhaseResult         `json:"phases"`
	FinalConsensus     *ConsensusResult       `json:"final_consensus"`
	FinalResponse      string                 `json:"final_response"`
	TotalDuration      time.Duration          `json:"total_duration"`
	OverallConfidence  float64                `json:"overall_confidence"`
	QualityImprovement float64                `json:"quality_improvement"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// MultiPassValidator provides multi-pass validation for debate responses.
// Concurrent-safe by construction (CONST-029): phaseCallbacks is a safe.Store;
// config is an atomic.Pointer.
type MultiPassValidator struct {
	debateService  *DebateService
	logger         *logrus.Logger
	config         atomic.Pointer[ValidationConfig]
	phaseCallbacks *safe.Store[ValidationPhase, func(*PhaseResult)]
}

// NewMultiPassValidator creates a new multi-pass validator
func NewMultiPassValidator(debateService *DebateService, logger *logrus.Logger) *MultiPassValidator {
	mpv := &MultiPassValidator{
		debateService:  debateService,
		logger:         logger,
		phaseCallbacks: safe.NewStore[ValidationPhase, func(*PhaseResult)](),
	}
	mpv.config.Store(DefaultValidationConfig())
	return mpv
}

// SetConfig sets the validation configuration
func (mpv *MultiPassValidator) SetConfig(config *ValidationConfig) {
	mpv.config.Store(config)
}

// GetConfig returns the current validation configuration
func (mpv *MultiPassValidator) GetConfig() *ValidationConfig {
	return mpv.config.Load()
}

// SetPhaseCallback sets a callback for a specific phase
func (mpv *MultiPassValidator) SetPhaseCallback(phase ValidationPhase, callback func(*PhaseResult)) {
	mpv.phaseCallbacks.Put(phase, callback)
}

// notifyPhaseCallback notifies the callback for a phase
func (mpv *MultiPassValidator) notifyPhaseCallback(phase ValidationPhase, result *PhaseResult) {
	if callback, ok := mpv.phaseCallbacks.Get(phase); ok && callback != nil {
		callback(result)
	}
}

// ValidateAndImprove performs multi-pass validation and improvement on debate responses
func (mpv *MultiPassValidator) ValidateAndImprove(
	ctx context.Context,
	debateResult *DebateResult,
) (*MultiPassResult, error) {
	startTime := time.Now()

	mpv.logger.WithFields(logrus.Fields{
		"debate_id":     debateResult.DebateID,
		"topic":         debateResult.Topic,
		"responses":     len(debateResult.AllResponses),
		"enable_polish": mpv.config.Load().EnablePolish,
	}).Info("Starting multi-pass validation")

	result := &MultiPassResult{
		DebateID: debateResult.DebateID,
		Topic:    debateResult.Topic,
		Config:   mpv.config.Load(),
		Phases:   make([]*PhaseResult, 0, 4),
		Metadata: make(map[string]interface{}),
	}

	// Phase 1: Initial Response (already collected from debate)
	phase1 := mpv.createInitialPhase(debateResult)
	result.Phases = append(result.Phases, phase1)
	mpv.notifyPhaseCallback(PhaseInitialResponse, phase1)

	// Calculate initial quality
	initialQuality := mpv.calculatePhaseQuality(phase1.Responses)

	// Phase 2: Validation
	if mpv.config.Load().EnableValidation {
		phase2, err := mpv.runValidationPhase(ctx, phase1.Responses, debateResult.Topic)
		if err != nil {
			mpv.logger.WithError(err).Warn("Validation phase failed, continuing with initial responses")
		} else {
			result.Phases = append(result.Phases, phase2)
			mpv.notifyPhaseCallback(PhaseValidation, phase2)
		}
	}

	// Phase 3: Polish & Improve
	var polishedResponses []ParticipantResponse
	if mpv.config.Load().EnablePolish {
		// Only polish if initial confidence is below threshold
		avgConfidence := mpv.calculateAverageConfidence(phase1.Responses)
		if avgConfidence < mpv.config.Load().MinConfidenceToSkip {
			lastPhase := result.Phases[len(result.Phases)-1]
			phase3, err := mpv.runPolishPhase(ctx, lastPhase.Responses, debateResult.Topic)
			if err != nil {
				mpv.logger.WithError(err).Warn("Polish phase failed, using validated responses")
			} else {
				result.Phases = append(result.Phases, phase3)
				mpv.notifyPhaseCallback(PhasePolishImprove, phase3)
				polishedResponses = phase3.Responses
			}
		} else {
			mpv.logger.WithField("confidence", avgConfidence).Info("Skipping polish phase - confidence above threshold")
		}
	}

	// Phase 4: Final Conclusion
	finalResponses := phase1.Responses
	if len(polishedResponses) > 0 {
		finalResponses = polishedResponses
	} else if len(result.Phases) > 1 {
		finalResponses = result.Phases[len(result.Phases)-1].Responses
	}

	phase4, err := mpv.runFinalConclusionPhase(ctx, finalResponses, debateResult.Topic)
	if err != nil {
		mpv.logger.WithError(err).Warn("Final conclusion phase failed")
	} else {
		result.Phases = append(result.Phases, phase4)
		mpv.notifyPhaseCallback(PhaseFinalConclusion, phase4)
	}

	// Calculate final results
	result.TotalDuration = time.Since(startTime)
	result.FinalConsensus = debateResult.Consensus

	// Calculate quality improvement
	finalQuality := mpv.calculatePhaseQuality(finalResponses)
	if initialQuality > 0 {
		result.QualityImprovement = (finalQuality - initialQuality) / initialQuality * 100
	}

	// Calculate overall confidence
	result.OverallConfidence = mpv.calculateAverageConfidence(finalResponses)

	// Generate final response
	result.FinalResponse = mpv.generateFinalResponse(finalResponses, debateResult.Topic)

	// Add metadata
	result.Metadata["initial_quality"] = initialQuality
	result.Metadata["final_quality"] = finalQuality
	result.Metadata["total_phases"] = len(result.Phases)

	mpv.logger.WithFields(logrus.Fields{
		"debate_id":           debateResult.DebateID,
		"total_duration":      result.TotalDuration,
		"quality_improvement": result.QualityImprovement,
		"overall_confidence":  result.OverallConfidence,
		"phases_completed":    len(result.Phases),
	}).Info("Multi-pass validation completed")

	return result, nil
}

// createInitialPhase creates the initial phase result from debate responses
func (mpv *MultiPassValidator) createInitialPhase(debateResult *DebateResult) *PhaseResult {
	return &PhaseResult{
		Phase:        PhaseInitialResponse,
		StartTime:    debateResult.StartTime,
		EndTime:      debateResult.EndTime,
		Duration:     debateResult.Duration,
		Responses:    debateResult.AllResponses,
		PhaseScore:   debateResult.QualityScore,
		PhaseSummary: "Initial responses collected from all debate participants",
	}
}

// runValidationPhase runs the validation phase
func (mpv *MultiPassValidator) runValidationPhase(
	ctx context.Context,
	responses []ParticipantResponse,
	topic string,
) (*PhaseResult, error) {
	startTime := time.Now()

	mpv.logger.Info("Starting validation phase")

	validations := make([]ValidationResult, 0, len(responses))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Create validation context with timeout
	validationCtx, cancel := context.WithTimeout(ctx, mpv.config.Load().ValidationTimeout)
	defer cancel()

	for _, resp := range responses {
		if mpv.config.Load().ParallelValidation {
			wg.Add(1)
			go func(r ParticipantResponse) {
				defer wg.Done()
				validation := mpv.validateResponse(validationCtx, r, responses, topic)
				mu.Lock()
				validations = append(validations, validation)
				mu.Unlock()
			}(resp)
		} else {
			validation := mpv.validateResponse(validationCtx, resp, responses, topic)
			validations = append(validations, validation)
		}
	}

	if mpv.config.Load().ParallelValidation {
		wg.Wait()
	}

	// Calculate phase score
	totalScore := 0.0
	for _, v := range validations {
		totalScore += v.ValidationScore
	}
	phaseScore := totalScore / float64(len(validations))

	return &PhaseResult{
		Phase:        PhaseValidation,
		StartTime:    startTime,
		EndTime:      time.Now(),
		Duration:     time.Since(startTime),
		Responses:    responses,
		Validations:  validations,
		PhaseScore:   phaseScore,
		PhaseSummary: fmt.Sprintf("Validated %d responses with average score %.2f", len(validations), phaseScore),
	}, nil
}

// validateResponse validates a single response against all other responses
func (mpv *MultiPassValidator) validateResponse(
	ctx context.Context,
	response ParticipantResponse,
	allResponses []ParticipantResponse,
	topic string,
) ValidationResult {
	startTime := time.Now()

	// Build validation context from other responses
	var otherPerspectives strings.Builder
	for _, r := range allResponses {
		if r.ParticipantID != response.ParticipantID {
			otherPerspectives.WriteString(fmt.Sprintf("- %s (%s): %s\n",
				r.ParticipantName, r.Role, truncateForValidation(r.Content, 200)))
		}
	}

	// Create validation prompt
	validationPrompt := fmt.Sprintf(`You are a validation expert. Analyze this response for factual accuracy, completeness, and coherence.

TOPIC: %s

RESPONSE TO VALIDATE:
%s

OTHER PERSPECTIVES:
%s

Evaluate the response on these criteria (0.0 to 1.0):
1. Factual Accuracy: Is the information correct and verifiable?
2. Completeness: Does it address the topic comprehensively?
3. Coherence: Is it well-structured and logical?

Identify any issues and provide improvement suggestions.

Respond in this exact format:
FACTUAL_ACCURACY: [score]
COMPLETENESS: [score]
COHERENCE: [score]
ISSUES: [list any issues, one per line]
SUGGESTIONS: [list improvement suggestions, one per line]`,
		topic, response.Content, otherPerspectives.String())

	// Try to get validation from LLM
	var factualAccuracy, completeness, coherence float64 = 0.7, 0.7, 0.7
	var issues []ValidationIssue
	var suggestions []string
	var validatorProvider, validatorModel string

	if mpv.debateService != nil && mpv.debateService.providerRegistry != nil {
		// Try to use Claude for validation (highest quality)
		providers := []string{"claude", "deepseek", "gemini", "qwen"}
		for _, provName := range providers {
			provider, err := mpv.debateService.providerRegistry.GetProvider(provName)
			if err != nil {
				continue
			}

			llmReq := &models.LLMRequest{
				Prompt: validationPrompt,
				Messages: []models.Message{
					{Role: "user", Content: validationPrompt},
				},
				ModelParams: models.ModelParameters{
					Temperature: 0.3, // Lower temperature for validation
					MaxTokens:   1000,
				},
			}

			llmResp, err := provider.Complete(ctx, llmReq)
			if err != nil {
				continue
			}

			// Parse validation response
			factualAccuracy, completeness, coherence, issues, suggestions = parseValidationResponse(llmResp.Content)
			validatorProvider = provName
			validatorModel = provName + "-model"
			break
		}
	}

	// Fallback to heuristic validation if LLM unavailable
	if validatorProvider == "" {
		factualAccuracy, completeness, coherence, issues, suggestions = heuristicValidation(response.Content, topic)
		validatorProvider = "heuristic"
		validatorModel = "rule-based"
	}

	validationScore := (factualAccuracy + completeness + coherence) / 3.0

	return ValidationResult{
		ParticipantID:     response.ParticipantID,
		OriginalResponse:  response.Content,
		ValidationScore:   validationScore,
		FactualAccuracy:   factualAccuracy,
		Completeness:      completeness,
		Coherence:         coherence,
		Issues:            issues,
		Suggestions:       suggestions,
		ValidatorID:       fmt.Sprintf("validator-%d", time.Now().UnixNano()),
		ValidatorProvider: validatorProvider,
		ValidatorModel:    validatorModel,
		Timestamp:         startTime,
		Duration:          time.Since(startTime),
	}
}

// runPolishPhase runs the polish and improvement phase
func (mpv *MultiPassValidator) runPolishPhase(
	ctx context.Context,
	responses []ParticipantResponse,
	topic string,
) (*PhaseResult, error) {
	startTime := time.Now()

	mpv.logger.Info("Starting polish & improvement phase")

	polishes := make([]PolishResult, 0, len(responses))
	polishedResponses := make([]ParticipantResponse, 0, len(responses))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Create polish context with timeout
	polishCtx, cancel := context.WithTimeout(ctx, mpv.config.Load().PolishTimeout)
	defer cancel()

	for _, resp := range responses {
		if mpv.config.Load().ParallelValidation {
			wg.Add(1)
			go func(r ParticipantResponse) {
				defer wg.Done()
				polish, polishedResp := mpv.polishResponse(polishCtx, r, topic)
				mu.Lock()
				polishes = append(polishes, polish)
				polishedResponses = append(polishedResponses, polishedResp)
				mu.Unlock()
			}(resp)
		} else {
			polish, polishedResp := mpv.polishResponse(polishCtx, resp, topic)
			polishes = append(polishes, polish)
			polishedResponses = append(polishedResponses, polishedResp)
		}
	}

	if mpv.config.Load().ParallelValidation {
		wg.Wait()
	}

	// Calculate phase score
	totalImprovement := 0.0
	for _, p := range polishes {
		totalImprovement += p.ImprovementScore
	}
	avgImprovement := totalImprovement / float64(len(polishes))

	return &PhaseResult{
		Phase:        PhasePolishImprove,
		StartTime:    startTime,
		EndTime:      time.Now(),
		Duration:     time.Since(startTime),
		Responses:    polishedResponses,
		Polishes:     polishes,
		PhaseScore:   avgImprovement,
		PhaseSummary: fmt.Sprintf("Polished %d responses with average improvement %.2f%%", len(polishes), avgImprovement*100),
	}, nil
}

// polishResponse polishes and improves a single response
func (mpv *MultiPassValidator) polishResponse(
	ctx context.Context,
	response ParticipantResponse,
	topic string,
) (PolishResult, ParticipantResponse) {
	startTime := time.Now()

	polishPrompt := fmt.Sprintf(`You are an expert editor. Improve this response while preserving its core message and intent.

TOPIC: %s

ORIGINAL RESPONSE:
%s

Instructions:
1. Improve clarity and conciseness
2. Ensure logical flow and structure
3. Add specific examples or evidence where helpful
4. Remove redundancies and filler content
5. Maintain the original perspective and conclusions

Provide the improved response directly, followed by a summary of changes.

IMPROVED RESPONSE:`, topic, response.Content)

	var polishedContent string = response.Content
	var changesSummary []string
	var polisherProvider, polisherModel string

	if mpv.debateService != nil && mpv.debateService.providerRegistry != nil {
		providers := []string{"claude", "deepseek", "gemini", "qwen"}
		for _, provName := range providers {
			provider, err := mpv.debateService.providerRegistry.GetProvider(provName)
			if err != nil {
				continue
			}

			llmReq := &models.LLMRequest{
				Prompt: polishPrompt,
				Messages: []models.Message{
					{Role: "user", Content: polishPrompt},
				},
				ModelParams: models.ModelParameters{
					Temperature: 0.5,
					MaxTokens:   1500,
				},
			}

			llmResp, err := provider.Complete(ctx, llmReq)
			if err != nil {
				continue
			}

			polishedContent, changesSummary = parsePolishResponse(llmResp.Content)
			polisherProvider = provName
			polisherModel = provName + "-model"
			break
		}
	}

	// Calculate improvement score
	improvementScore := calculateImprovementScore(response.Content, polishedContent)

	polishResult := PolishResult{
		ParticipantID:    response.ParticipantID,
		OriginalResponse: response.Content,
		PolishedResponse: polishedContent,
		ImprovementScore: improvementScore,
		ChangesSummary:   changesSummary,
		PolisherID:       fmt.Sprintf("polisher-%d", time.Now().UnixNano()),
		PolisherProvider: polisherProvider,
		PolisherModel:    polisherModel,
		Timestamp:        startTime,
		Duration:         time.Since(startTime),
	}

	// Create polished response
	polishedResponse := response
	polishedResponse.Content = polishedContent
	polishedResponse.Response = polishedContent
	if polishedResponse.Metadata == nil {
		polishedResponse.Metadata = make(map[string]interface{})
	}
	polishedResponse.Metadata["polished"] = true
	polishedResponse.Metadata["improvement_score"] = improvementScore

	return polishResult, polishedResponse
}

// runFinalConclusionPhase runs the final conclusion phase
func (mpv *MultiPassValidator) runFinalConclusionPhase(
	ctx context.Context,
	responses []ParticipantResponse,
	topic string,
) (*PhaseResult, error) {
	startTime := time.Now()

	mpv.logger.Info("Starting final conclusion phase")

	// Synthesize all responses into a final conclusion
	var allContent strings.Builder
	for _, r := range responses {
		allContent.WriteString(fmt.Sprintf("[%s - %s]: %s\n\n", r.ParticipantName, r.Role, r.Content))
	}

	synthesisPrompt := fmt.Sprintf(`You are a synthesis expert. Create a comprehensive final conclusion from these debate perspectives.

TOPIC: %s

ALL PERSPECTIVES:
%s

Create a unified final conclusion that:
1. Identifies the core consensus points
2. Acknowledges any valid dissenting views
3. Provides a clear, actionable conclusion
4. Rates overall confidence (0.0 to 1.0)

Format your response as:
CONCLUSION:
[Your synthesized conclusion]

CONSENSUS_POINTS:
[Key points of agreement]

CONFIDENCE: [score]`, topic, allContent.String())

	finalConclusion := mpv.generateFinalResponse(responses, topic)
	var confidence float64 = 0.8

	if mpv.debateService != nil && mpv.debateService.providerRegistry != nil {
		providers := []string{"claude", "deepseek", "gemini", "qwen"}
		for _, provName := range providers {
			provider, err := mpv.debateService.providerRegistry.GetProvider(provName)
			if err != nil {
				continue
			}

			llmReq := &models.LLMRequest{
				Prompt: synthesisPrompt,
				Messages: []models.Message{
					{Role: "user", Content: synthesisPrompt},
				},
				ModelParams: models.ModelParameters{
					Temperature: 0.3,
					MaxTokens:   2000,
				},
			}

			llmResp, err := provider.Complete(ctx, llmReq)
			if err != nil {
				continue
			}

			finalConclusion, confidence = parseSynthesisResponse(llmResp.Content)
			break
		}
	}

	// Create final response as a synthetic participant
	finalResponse := ParticipantResponse{
		ParticipantID:   "final-synthesis",
		ParticipantName: "CONSENSUS",
		Role:            "synthesis",
		Content:         finalConclusion,
		Response:        finalConclusion,
		Confidence:      confidence,
		QualityScore:    confidence,
		Timestamp:       time.Now(),
		Metadata: map[string]interface{}{
			"is_final_synthesis": true,
			"source_responses":   len(responses),
		},
	}

	return &PhaseResult{
		Phase:        PhaseFinalConclusion,
		StartTime:    startTime,
		EndTime:      time.Now(),
		Duration:     time.Since(startTime),
		Responses:    []ParticipantResponse{finalResponse},
		PhaseScore:   confidence,
		PhaseSummary: fmt.Sprintf("Final consensus reached with %.0f%% confidence", confidence*100),
	}, nil
}

// generateFinalResponse generates the final synthesized response
func (mpv *MultiPassValidator) generateFinalResponse(responses []ParticipantResponse, topic string) string {
	if len(responses) == 0 {
		return "No responses to synthesize."
	}

	// Simple synthesis: combine best points from all responses
	var synthesis strings.Builder
	synthesis.WriteString(fmt.Sprintf("After careful analysis of %d perspectives on \"%s\", ", len(responses), truncateForValidation(topic, 50)))
	synthesis.WriteString("the following conclusions were reached:\n\n")

	// Extract key points from each response
	for i, r := range responses {
		if i < 3 { // Limit to top 3 responses
			// Take first sentence or first 200 chars
			content := r.Content
			if idx := strings.Index(content, "."); idx > 0 && idx < 200 {
				content = content[:idx+1]
			} else if len(content) > 200 {
				content = content[:200] + "..."
			}
			synthesis.WriteString(fmt.Sprintf("- %s\n", content))
		}
	}

	return synthesis.String()
}

// calculatePhaseQuality calculates the quality score for a phase
func (mpv *MultiPassValidator) calculatePhaseQuality(responses []ParticipantResponse) float64 {
	if len(responses) == 0 {
		return 0.0
	}

	total := 0.0
	for _, r := range responses {
		total += r.QualityScore
	}
	return total / float64(len(responses))
}

// calculateAverageConfidence calculates average confidence across responses
func (mpv *MultiPassValidator) calculateAverageConfidence(responses []ParticipantResponse) float64 {
	if len(responses) == 0 {
		return 0.0
	}

	total := 0.0
	for _, r := range responses {
		total += r.Confidence
	}
	return total / float64(len(responses))
}

// Helper functions

func truncateForValidation(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func parseValidationResponse(content string) (float64, float64, float64, []ValidationIssue, []string) {
	// Default values
	factual, complete, coherent := 0.7, 0.7, 0.7
	var issues []ValidationIssue
	var suggestions []string

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FACTUAL_ACCURACY:") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "FACTUAL_ACCURACY:"), "%f", &factual) //nolint:errcheck
		} else if strings.HasPrefix(line, "COMPLETENESS:") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "COMPLETENESS:"), "%f", &complete) //nolint:errcheck
		} else if strings.HasPrefix(line, "COHERENCE:") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "COHERENCE:"), "%f", &coherent) //nolint:errcheck
		}
	}

	return factual, complete, coherent, issues, suggestions
}

func parsePolishResponse(content string) (string, []string) {
	// Extract improved response
	polished := content
	if idx := strings.Index(content, "IMPROVED RESPONSE:"); idx >= 0 {
		polished = strings.TrimSpace(content[idx+len("IMPROVED RESPONSE:"):])
	}

	// Extract changes summary
	var changes []string
	if idx := strings.Index(polished, "CHANGES:"); idx >= 0 {
		changesText := polished[idx+len("CHANGES:"):]
		polished = strings.TrimSpace(polished[:idx])
		for _, line := range strings.Split(changesText, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && (strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*")) {
				changes = append(changes, strings.TrimPrefix(strings.TrimPrefix(line, "-"), "*"))
			}
		}
	}

	return polished, changes
}

func parseSynthesisResponse(content string) (string, float64) {
	conclusion := content
	confidence := 0.8

	if idx := strings.Index(content, "CONCLUSION:"); idx >= 0 {
		rest := content[idx+len("CONCLUSION:"):]
		if confIdx := strings.Index(rest, "CONFIDENCE:"); confIdx >= 0 {
			conclusion = strings.TrimSpace(rest[:confIdx])
			_, _ = fmt.Sscanf(strings.TrimPrefix(rest[confIdx:], "CONFIDENCE:"), "%f", &confidence) //nolint:errcheck
		} else {
			conclusion = strings.TrimSpace(rest)
		}
	}

	return conclusion, confidence
}

func heuristicValidation(content, topic string) (float64, float64, float64, []ValidationIssue, []string) {
	// Simple heuristic validation based on content analysis
	factual := 0.7
	complete := 0.7
	coherent := 0.7
	var issues []ValidationIssue
	var suggestions []string

	// Check content length
	if len(content) < 50 {
		complete = 0.4
		issues = append(issues, ValidationIssue{
			Type:        IssueIncomplete,
			Severity:    ValidationSeverityMajor,
			Description: "Response is too brief",
			Suggestion:  "Expand with more details and examples",
		})
	} else if len(content) > 2000 {
		coherent = 0.6
		suggestions = append(suggestions, "Consider condensing the response for clarity")
	}

	// Check for topic relevance
	topicWords := strings.Fields(strings.ToLower(topic))
	contentLower := strings.ToLower(content)
	matchCount := 0
	for _, word := range topicWords {
		if len(word) > 3 && strings.Contains(contentLower, word) {
			matchCount++
		}
	}
	if len(topicWords) > 0 && matchCount < len(topicWords)/2 {
		factual = 0.5
		issues = append(issues, ValidationIssue{
			Type:        IssueOutOfScope,
			Severity:    ValidationSeverityMinor,
			Description: "Response may not fully address the topic",
		})
	}

	return factual, complete, coherent, issues, suggestions
}

func calculateImprovementScore(original, polished string) float64 {
	if original == polished {
		return 0.0
	}

	// Simple improvement detection based on length and structure changes
	lenDiff := float64(len(polished)-len(original)) / float64(len(original)+1)

	// Check for structural improvements
	originalSentences := strings.Count(original, ".")
	polishedSentences := strings.Count(polished, ".")
	structureChange := float64(polishedSentences-originalSentences) / float64(originalSentences+1)

	// Normalize to 0-1 range
	improvement := 0.5 + (lenDiff * 0.25) + (structureChange * 0.25)
	if improvement < 0 {
		improvement = 0.1
	} else if improvement > 1 {
		improvement = 1.0
	}

	return improvement
}

// ============================================================================
// Phase Rendering Functions
// ============================================================================

// FormatPhaseHeader returns a formatted header for a validation phase
func FormatPhaseHeader(phase ValidationPhase, verbose bool) string {
	info := GetPhaseInfo(phase)
	if info == nil {
		return ""
	}

	if verbose {
		return fmt.Sprintf(`
%s
%s PHASE %d: %s %s
%s
%s
%s

`,
			strings.Repeat("═", 70),
			info.Icon,
			info.Order,
			info.Name,
			info.Icon,
			strings.Repeat("═", 70),
			info.Description,
			strings.Repeat("─", 70))
	}

	return fmt.Sprintf("\n%s PHASE %d: %s %s\n%s\n",
		info.Icon, info.Order, info.Name, info.Icon, strings.Repeat("─", 50))
}

// FormatPhaseFooter returns a formatted footer for a validation phase
func FormatPhaseFooter(phase ValidationPhase, result *PhaseResult, verbose bool) string {
	info := GetPhaseInfo(phase)
	if info == nil {
		return ""
	}

	if verbose {
		return fmt.Sprintf(`
%s
Phase %d Complete | Duration: %v | Score: %.2f
%s: %s
%s

`,
			strings.Repeat("─", 70),
			info.Order,
			result.Duration.Round(time.Millisecond),
			result.PhaseScore,
			info.Name,
			result.PhaseSummary,
			strings.Repeat("═", 70))
	}

	return fmt.Sprintf("[Phase %d Complete: %.2f score]\n", info.Order, result.PhaseScore)
}

// FormatMultiPassOutput formats the complete multi-pass validation output
func FormatMultiPassOutput(result *MultiPassResult) string {
	var sb strings.Builder

	// Header
	sb.WriteString(`
╔══════════════════════════════════════════════════════════════════════════════╗
║              🎭 HELIXAGENT AI DEBATE ENSEMBLE - MULTI-PASS VALIDATION 🎭      ║
╠══════════════════════════════════════════════════════════════════════════════╣
║  Four-phase validation ensures the highest quality consensus response.        ║
╚══════════════════════════════════════════════════════════════════════════════╝
`)

	sb.WriteString(fmt.Sprintf("\n📋 TOPIC: %s\n\n", truncateForValidation(result.Topic, 60)))

	// Phases
	for _, phase := range result.Phases {
		sb.WriteString(FormatPhaseHeader(phase.Phase, true))

		// Show phase-specific content
		switch phase.Phase {
		case PhaseInitialResponse:
			for _, r := range phase.Responses {
				sb.WriteString(fmt.Sprintf("[%s] %s:\n",
					strings.ToUpper(r.Role[:1]), r.ParticipantName))
				sb.WriteString(fmt.Sprintf("    %s\n\n", truncateForValidation(r.Content, 200)))
			}
		case PhaseValidation:
			for _, v := range phase.Validations {
				sb.WriteString(fmt.Sprintf("Validation Score: %.2f (Accuracy: %.2f, Complete: %.2f, Coherent: %.2f)\n",
					v.ValidationScore, v.FactualAccuracy, v.Completeness, v.Coherence))
			}
		case PhasePolishImprove:
			for _, p := range phase.Polishes {
				sb.WriteString(fmt.Sprintf("Improvement: %.0f%% | Changes: %d\n",
					p.ImprovementScore*100, len(p.ChangesSummary)))
			}
		case PhaseFinalConclusion:
			if len(phase.Responses) > 0 {
				sb.WriteString(fmt.Sprintf("\n%s\n", phase.Responses[0].Content))
			}
		}

		sb.WriteString(FormatPhaseFooter(phase.Phase, phase, true))
	}

	// Final Summary
	sb.WriteString(`
╔══════════════════════════════════════════════════════════════════════════════╗
║                           📜 FINAL SUMMARY 📜                                 ║
╠══════════════════════════════════════════════════════════════════════════════╣
`)
	sb.WriteString(fmt.Sprintf("║  Overall Confidence: %.0f%%\n", result.OverallConfidence*100))
	sb.WriteString(fmt.Sprintf("║  Quality Improvement: %.1f%%\n", result.QualityImprovement))
	sb.WriteString(fmt.Sprintf("║  Total Duration: %v\n", result.TotalDuration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("║  Phases Completed: %d/4\n", len(result.Phases)))
	sb.WriteString(`╚══════════════════════════════════════════════════════════════════════════════╝

`)
	sb.WriteString("───────────────────────────────────────────────────────────────────────────────\n")
	sb.WriteString("         ✨ Powered by HelixAgent AI Debate Ensemble - Multi-Pass Validation ✨\n")
	sb.WriteString("───────────────────────────────────────────────────────────────────────────────\n")

	return sb.String()
}
