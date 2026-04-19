package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/services/debate_integration"
	"dev.helix.agent/internal/skills"
)

// DebateHandler handles debate API endpoints
type DebateHandler struct {
	debateService           *services.DebateService
	advancedDebate          *services.AdvancedDebateService
	skillsIntegration       *skills.Integration
	orchestratorIntegration *debate_integration.ServiceIntegration
	logger                  *logrus.Logger

	// In-memory storage for active debates (in production, use database)
	activeDebates map[string]*debateState
	mu            sync.RWMutex
}

type debateState struct {
	Config                    *services.DebateConfig     `json:"config"`
	ValidationConfig          *services.ValidationConfig `json:"validation_config,omitempty"`
	EnableMultiPassValidation bool                       `json:"enable_multi_pass_validation"`
	Status                    string                     `json:"status"`
	CurrentPhase              string                     `json:"current_phase,omitempty"`
	Result                    *services.DebateResult     `json:"result,omitempty"`
	MultiPassResult           *services.MultiPassResult  `json:"multi_pass_result,omitempty"`
	Error                     string                     `json:"error,omitempty"`
	StartTime                 time.Time                  `json:"start_time"`
	EndTime                   *time.Time                 `json:"end_time,omitempty"`
	SkillsUsed                *skills.SkillsUsedMetadata `json:"skills_used,omitempty"`
	skillsContext             *skills.RequestContext     // Internal, not JSON-serialized
}

// NewDebateHandler creates a new debate handler
func NewDebateHandler(debateService *services.DebateService, advancedDebate *services.AdvancedDebateService, logger *logrus.Logger) *DebateHandler {
	return &DebateHandler{
		debateService:  debateService,
		advancedDebate: advancedDebate,
		logger:         logger,
		activeDebates:  make(map[string]*debateState),
	}
}

// SetSkillsIntegration sets the Skills integration for the handler
func (h *DebateHandler) SetSkillsIntegration(integration *skills.Integration) {
	h.skillsIntegration = integration
}

// SetOrchestratorIntegration sets the new debate framework integration
func (h *DebateHandler) SetOrchestratorIntegration(integration *debate_integration.ServiceIntegration) {
	h.orchestratorIntegration = integration
}

// CreateDebateRequest represents the request to create a debate
type CreateDebateRequest struct {
	DebateID                  string                     `json:"debate_id,omitempty"`
	Topic                     string                     `json:"topic" binding:"required"`
	Participants              []ParticipantConfigRequest `json:"participants" binding:"required,min=2"`
	MaxRounds                 int                        `json:"max_rounds,omitempty"`
	Timeout                   int                        `json:"timeout,omitempty"` // seconds
	Strategy                  string                     `json:"strategy,omitempty"`
	EnableCognee              bool                       `json:"enable_cognee,omitempty"`
	EnableMultiPassValidation bool                       `json:"enable_multi_pass_validation,omitempty"`
	ValidationConfig          *ValidationConfigRequest   `json:"validation_config,omitempty"`
	TopologyType              string                     `json:"topology_type,omitempty"`         // graph_mesh, star, chain, tree
	CoordinationProtocol      string                     `json:"coordination_protocol,omitempty"` // cpde, dpde, adaptive
	EnableAdversarial         bool                       `json:"enable_adversarial,omitempty"`
	EnableDehallucination     bool                       `json:"enable_dehallucination,omitempty"`
	EnableSelfEvolvement      bool                       `json:"enable_self_evolvement,omitempty"`
	EnableApprovalGates       bool                       `json:"enable_approval_gates,omitempty"`
	VotingMethod              string                     `json:"voting_method,omitempty"` // weighted, majority, borda, condorcet, plurality, unanimous
	MaxReflexionAttempts      int                        `json:"max_reflexion_attempts,omitempty"`
	Metadata                  map[string]any             `json:"metadata,omitempty"`
}

// ValidationConfigRequest configures multi-pass validation
type ValidationConfigRequest struct {
	EnableValidation    bool    `json:"enable_validation"`
	EnablePolish        bool    `json:"enable_polish"`
	ValidationTimeout   int     `json:"validation_timeout,omitempty"` // seconds
	PolishTimeout       int     `json:"polish_timeout,omitempty"`     // seconds
	MinConfidenceToSkip float64 `json:"min_confidence_to_skip,omitempty"`
	MaxValidationRounds int     `json:"max_validation_rounds,omitempty"`
	ShowPhaseIndicators bool    `json:"show_phase_indicators,omitempty"`
}

// ParticipantConfigRequest represents a participant in the request
type ParticipantConfigRequest struct {
	ParticipantID string  `json:"participant_id,omitempty"`
	Name          string  `json:"name" binding:"required"`
	Role          string  `json:"role,omitempty"`
	LLMProvider   string  `json:"llm_provider,omitempty"`
	LLMModel      string  `json:"llm_model,omitempty"`
	MaxRounds     int     `json:"max_rounds,omitempty"`
	Timeout       int     `json:"timeout,omitempty"`
	Weight        float64 `json:"weight,omitempty"`
}

// CreateDebate handles POST /v1/debates
func (h *DebateHandler) CreateDebate(c *gin.Context) {
	var req CreateDebateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Invalid request body",
				"details": err.Error(),
			},
		})
		return
	}

	// Generate debate ID if not provided
	debateID := req.DebateID
	if debateID == "" {
		debateID = "debate-" + uuid.New().String()[:8]
	}

	// Set defaults
	maxRounds := req.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 3
	}

	timeout := time.Duration(req.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	strategy := req.Strategy
	if strategy == "" {
		strategy = "consensus"
	}

	// Convert participants
	participants := make([]services.ParticipantConfig, len(req.Participants))
	for i, p := range req.Participants {
		participantID := p.ParticipantID
		if participantID == "" {
			participantID = "participant-" + uuid.New().String()[:8]
		}

		role := p.Role
		if role == "" {
			if i == 0 {
				role = "proposer"
			} else if i == 1 {
				role = "critic"
			} else {
				role = "debater"
			}
		}

		provider := p.LLMProvider
		if provider == "" {
			provider = "openai"
		}

		model := p.LLMModel
		if model == "" {
			model = "gpt-4"
		}

		weight := p.Weight
		if weight <= 0 {
			weight = 1.0
		}

		participants[i] = services.ParticipantConfig{
			ParticipantID: participantID,
			Name:          p.Name,
			Role:          role,
			LLMProvider:   provider,
			LLMModel:      model,
			MaxRounds:     p.MaxRounds,
			Timeout:       time.Duration(p.Timeout) * time.Second,
			Weight:        weight,
		}
	}

	config := &services.DebateConfig{
		DebateID:     debateID,
		Topic:        req.Topic,
		Participants: participants,
		MaxRounds:    maxRounds,
		Timeout:      timeout,
		Strategy:     strategy,
		EnableCognee: req.EnableCognee,
		Metadata:     req.Metadata,
	}

	// Build validation config if multi-pass validation is enabled
	var validationConfig *services.ValidationConfig
	if req.EnableMultiPassValidation {
		validationConfig = services.DefaultValidationConfig()
		if req.ValidationConfig != nil {
			validationConfig.EnableValidation = req.ValidationConfig.EnableValidation
			validationConfig.EnablePolish = req.ValidationConfig.EnablePolish
			if req.ValidationConfig.ValidationTimeout > 0 {
				validationConfig.ValidationTimeout = time.Duration(req.ValidationConfig.ValidationTimeout) * time.Second
			}
			if req.ValidationConfig.PolishTimeout > 0 {
				validationConfig.PolishTimeout = time.Duration(req.ValidationConfig.PolishTimeout) * time.Second
			}
			if req.ValidationConfig.MinConfidenceToSkip > 0 {
				validationConfig.MinConfidenceToSkip = req.ValidationConfig.MinConfidenceToSkip
			}
			if req.ValidationConfig.MaxValidationRounds > 0 {
				validationConfig.MaxValidationRounds = req.ValidationConfig.MaxValidationRounds
			}
			validationConfig.ShowPhaseIndicators = req.ValidationConfig.ShowPhaseIndicators
		}
	}

	// Process Skills matching if integration is available
	var skillsCtx *skills.RequestContext
	if h.skillsIntegration != nil {
		var err error
		skillsCtx, err = h.skillsIntegration.ProcessRequest(c.Request.Context(), debateID, req.Topic)
		if err != nil {
			h.logger.WithError(err).Warn("Failed to process Skills for debate")
		}
	}

	// Store debate state
	h.mu.Lock()
	h.activeDebates[debateID] = &debateState{
		Config:                    config,
		ValidationConfig:          validationConfig,
		EnableMultiPassValidation: req.EnableMultiPassValidation,
		Status:                    "pending",
		StartTime:                 time.Now(),
		skillsContext:             skillsCtx,
	}
	h.mu.Unlock()

	// Start debate asynchronously
	go h.runDebate(debateID, config, validationConfig)

	c.JSON(http.StatusAccepted, gin.H{
		"debate_id":    debateID,
		"status":       "pending",
		"topic":        req.Topic,
		"max_rounds":   maxRounds,
		"timeout":      timeout.Seconds(),
		"participants": len(participants),
		"created_at":   time.Now().Unix(),
		"message":      "Debate started. Use GET /v1/debates/" + debateID + " to check status.",
	})
}

// runDebate executes the debate asynchronously
func (h *DebateHandler) runDebate(debateID string, config *services.DebateConfig, validationConfig *services.ValidationConfig) {
	h.mu.Lock()
	if state, exists := h.activeDebates[debateID]; exists {
		state.Status = "running"
		if validationConfig != nil {
			state.CurrentPhase = string(services.PhaseInitialResponse)
		}
	}
	h.mu.Unlock()

	// Conduct the debate
	var result *services.DebateResult
	var multiPassResult *services.MultiPassResult
	var err error

	// Use the new debate framework (mandatory - per documentation requirements)
	// The new orchestrator framework implements the full AI debate spec:
	// - 5 positions × 3 LLMs = 15 agents
	// - 8-phase protocol: Dehallucination → SelfEvolvement → Proposal → Critique → Review → Optimization → Adversarial → Convergence
	// - Lesson banking and cross-debate learning
	// - Weighted voting with confidence scoring
	if h.orchestratorIntegration == nil {
		h.logger.WithField("debate_id", debateID).Error("Orchestrator integration not initialized - this is a configuration error")
		err = &debateError{message: "orchestrator integration not initialized"}
	} else {
		// Log agent pool status for debugging
		agentCount := h.orchestratorIntegration.GetOrchestrator().GetAgentPool().Size()
		h.logger.WithFields(logrus.Fields{
			"debate_id":   debateID,
			"agent_count": agentCount,
			"framework":   "new_orchestrator",
		}).Info("Running debate with new orchestrator framework (mandatory)")

		result, err = h.orchestratorIntegration.ConductDebate(context.Background(), config)
		if err != nil {
			h.logger.WithError(err).WithField("debate_id", debateID).Error("New framework debate failed")
			// Don't fall back to legacy - fail explicitly to force proper configuration
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if state, exists := h.activeDebates[debateID]; exists {
		now := time.Now()
		state.EndTime = &now

		// Complete Skills tracking and get usage metadata
		if h.skillsIntegration != nil && state.skillsContext != nil {
			success := err == nil
			errorMsg := ""
			if err != nil {
				errorMsg = err.Error()
			}
			usages := h.skillsIntegration.CompleteRequest(debateID, success, errorMsg)
			if len(usages) > 0 {
				state.SkillsUsed = h.skillsIntegration.BuildSkillsUsedSection(usages)
			}
		}

		if err != nil {
			state.Status = "failed"
			state.Error = err.Error()
			h.logger.WithError(err).WithField("debate_id", debateID).Error("Debate failed")
		} else {
			state.Status = "completed"
			state.Result = result
			state.MultiPassResult = multiPassResult
			if multiPassResult != nil { //nolint:govet
				state.CurrentPhase = string(services.PhaseFinalConclusion)
				h.logger.WithFields(logrus.Fields{
					"debate_id":           debateID,
					"phases_completed":    len(multiPassResult.Phases),
					"overall_confidence":  multiPassResult.OverallConfidence,
					"quality_improvement": multiPassResult.QualityImprovement,
				}).Info("Multi-pass debate completed successfully")
			} else {
				h.logger.WithField("debate_id", debateID).Info("Debate completed successfully")
			}
		}
	}
}

type debateError struct {
	message string
}

func (e *debateError) Error() string {
	return e.message
}

// GetDebate handles GET /v1/debates/:id.
//
// The read lock is held across the entire state-to-response copy
// because runDebate() writes to state.Status / state.CurrentPhase /
// state.EndTime / state.Error / state.Result / state.MultiPassResult /
// state.SkillsUsed concurrently. Previously the lock was released
// right after the map lookup and the subsequent field reads raced
// with runDebate's writes — caught by -race (BUGFIX #27).
func (h *DebateHandler) GetDebate(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.RLock()
	state, exists := h.activeDebates[debateID]
	if !exists {
		h.mu.RUnlock()
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message":   "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}

	// Build the response while still holding RLock. All field reads
	// must happen here; runDebate takes the write lock for state
	// mutations, so this section is race-safe.
	response := gin.H{
		"debate_id":                    debateID,
		"status":                       state.Status,
		"topic":                        state.Config.Topic,
		"max_rounds":                   state.Config.MaxRounds,
		"start_time":                   state.StartTime.Unix(),
		"enable_multi_pass_validation": state.EnableMultiPassValidation,
	}

	if state.CurrentPhase != "" {
		response["current_phase"] = state.CurrentPhase
	}

	if state.EndTime != nil {
		response["end_time"] = state.EndTime.Unix()
		response["duration_seconds"] = state.EndTime.Sub(state.StartTime).Seconds()
	}

	if state.Error != "" {
		response["error"] = state.Error
	}

	if state.Result != nil {
		response["result"] = state.Result
	}

	if state.SkillsUsed != nil {
		response["skills_used"] = state.SkillsUsed
	}

	if state.MultiPassResult != nil {
		response["multi_pass_result"] = gin.H{
			"phases_completed":    len(state.MultiPassResult.Phases),
			"final_response":      state.MultiPassResult.FinalResponse,
			"overall_confidence":  state.MultiPassResult.OverallConfidence,
			"quality_improvement": state.MultiPassResult.QualityImprovement,
			"phases":              state.MultiPassResult.Phases,
		}
	}
	h.mu.RUnlock()

	c.JSON(http.StatusOK, response)
}

// GetDebateStatus handles GET /v1/debates/:id/status.
// Same race-safety pattern as GetDebate above (BUGFIX #27).
func (h *DebateHandler) GetDebateStatus(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.RLock()
	state, exists := h.activeDebates[debateID]
	if !exists {
		h.mu.RUnlock()
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message":   "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}

	status := gin.H{
		"debate_id":                    debateID,
		"status":                       state.Status,
		"start_time":                   state.StartTime.Unix(),
		"enable_multi_pass_validation": state.EnableMultiPassValidation,
	}

	if state.CurrentPhase != "" {
		status["current_phase"] = state.CurrentPhase
	}

	if state.EndTime != nil {
		status["end_time"] = state.EndTime.Unix()
		status["duration_seconds"] = state.EndTime.Sub(state.StartTime).Seconds()
	}

	if state.Error != "" {
		status["error"] = state.Error
	}

	if state.Status == "running" && state.Config != nil {
		status["max_rounds"] = state.Config.MaxRounds
		status["timeout_seconds"] = state.Config.Timeout.Seconds()
		if state.EnableMultiPassValidation {
			status["validation_phases"] = []string{
				string(services.PhaseInitialResponse),
				string(services.PhaseValidation),
				string(services.PhasePolishImprove),
				string(services.PhaseFinalConclusion),
			}
		}
	}

	if state.MultiPassResult != nil {
		status["overall_confidence"] = state.MultiPassResult.OverallConfidence
		status["quality_improvement"] = state.MultiPassResult.QualityImprovement
		status["phases_completed"] = len(state.MultiPassResult.Phases)
	}
	h.mu.RUnlock()

	c.JSON(http.StatusOK, status)
}

// GetDebateResults handles GET /v1/debates/:id/results.
// Same race-safety pattern as GetDebate (BUGFIX #27).
func (h *DebateHandler) GetDebateResults(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.RLock()
	state, exists := h.activeDebates[debateID]
	if !exists {
		h.mu.RUnlock()
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message":   "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}
	// Snapshot the fields we need under the lock; release before
	// responding so we do not hold RLock across c.JSON.
	status := state.Status
	result := state.Result
	h.mu.RUnlock()

	if status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Debate has not completed yet",
				"status":  status,
			},
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Debate completed but no results available",
			},
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ListDebates handles GET /v1/debates
func (h *DebateHandler) ListDebates(c *gin.Context) {
	status := c.Query("status")

	h.mu.RLock()
	defer h.mu.RUnlock()

	debates := make([]gin.H, 0)
	for id, state := range h.activeDebates {
		if status != "" && state.Status != status {
			continue
		}

		debate := gin.H{
			"debate_id":  id,
			"topic":      state.Config.Topic,
			"status":     state.Status,
			"start_time": state.StartTime.Unix(),
		}

		if state.EndTime != nil {
			debate["end_time"] = state.EndTime.Unix()
		}

		debates = append(debates, debate)
	}

	c.JSON(http.StatusOK, gin.H{
		"debates": debates,
		"count":   len(debates),
	})
}

// DeleteDebate handles DELETE /v1/debates/:id
func (h *DebateHandler) DeleteDebate(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.activeDebates[debateID]; !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message":   "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}

	delete(h.activeDebates, debateID)

	c.JSON(http.StatusOK, gin.H{
		"message":   "Debate deleted",
		"debate_id": debateID,
	})
}

// ApproveDebate handles POST /v1/debates/:id/approve
func (h *DebateHandler) ApproveDebate(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.RLock()
	state, exists := h.activeDebates[debateID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message":   "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}

	if state.Status != "paused" && state.Status != "pending_approval" {
		c.JSON(http.StatusConflict, gin.H{
			"error": gin.H{
				"message":   "Debate is not awaiting approval",
				"debate_id": debateID,
				"status":    state.Status,
			},
		})
		return
	}

	h.mu.Lock()
	state.Status = "running"
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message":   "Debate approved and resumed",
		"debate_id": debateID,
		"status":    "running",
	})
}

// RejectDebate handles POST /v1/debates/:id/reject
func (h *DebateHandler) RejectDebate(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.RLock()
	state, exists := h.activeDebates[debateID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message":   "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req) //nolint:errcheck

	h.mu.Lock()
	state.Status = "rejected"
	if req.Reason != "" {
		state.Error = "Rejected: " + req.Reason
	}
	now := time.Now()
	state.EndTime = &now
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message":   "Debate rejected",
		"debate_id": debateID,
		"status":    "rejected",
		"reason":    req.Reason,
	})
}

// GetDebateGates handles GET /v1/debates/:id/gates
func (h *DebateHandler) GetDebateGates(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.RLock()
	state, exists := h.activeDebates[debateID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message":   "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"debate_id":        debateID,
		"status":           state.Status,
		"current_phase":    state.CurrentPhase,
		"gates_enabled":    state.Config != nil,
		"pending_approval": state.Status == "pending_approval" || state.Status == "paused",
	})
}

// GetDebateAudit handles GET /v1/debates/:id/audit
func (h *DebateHandler) GetDebateAudit(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.RLock()
	state, exists := h.activeDebates[debateID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message":   "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"debate_id":     debateID,
		"status":        state.Status,
		"start_time":    state.StartTime,
		"end_time":      state.EndTime,
		"current_phase": state.CurrentPhase,
		"has_result":    state.Result != nil,
	})
}

// RegisterRoutes registers debate routes on a router group
func (h *DebateHandler) RegisterRoutes(rg *gin.RouterGroup) {
	debates := rg.Group("/debates")
	{
		debates.POST("", h.CreateDebate)
		debates.GET("", h.ListDebates)
		debates.GET("/:id", h.GetDebate)
		debates.GET("/:id/status", h.GetDebateStatus)
		debates.GET("/:id/results", h.GetDebateResults)
		debates.POST("/:id/approve", h.ApproveDebate)
		debates.POST("/:id/reject", h.RejectDebate)
		debates.GET("/:id/gates", h.GetDebateGates)
		debates.GET("/:id/audit", h.GetDebateAudit)
		debates.DELETE("/:id", h.DeleteDebate)
	}
}
