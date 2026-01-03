package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/superagent/superagent/internal/services"
)

// DebateHandler handles debate API endpoints
type DebateHandler struct {
	debateService  *services.DebateService
	advancedDebate *services.AdvancedDebateService
	logger         *logrus.Logger

	// In-memory storage for active debates (in production, use database)
	activeDebates map[string]*debateState
	mu            sync.RWMutex
}

type debateState struct {
	Config    *services.DebateConfig `json:"config"`
	Status    string                 `json:"status"`
	Result    *services.DebateResult `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
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

// CreateDebateRequest represents the request to create a debate
type CreateDebateRequest struct {
	DebateID     string                       `json:"debate_id,omitempty"`
	Topic        string                       `json:"topic" binding:"required"`
	Participants []ParticipantConfigRequest   `json:"participants" binding:"required,min=2"`
	MaxRounds    int                          `json:"max_rounds,omitempty"`
	Timeout      int                          `json:"timeout,omitempty"` // seconds
	Strategy     string                       `json:"strategy,omitempty"`
	EnableCognee bool                         `json:"enable_cognee,omitempty"`
	Metadata     map[string]any               `json:"metadata,omitempty"`
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

	// Store debate state
	h.mu.Lock()
	h.activeDebates[debateID] = &debateState{
		Config:    config,
		Status:    "pending",
		StartTime: time.Now(),
	}
	h.mu.Unlock()

	// Start debate asynchronously
	go h.runDebate(debateID, config)

	c.JSON(http.StatusAccepted, gin.H{
		"debate_id":  debateID,
		"status":     "pending",
		"topic":      req.Topic,
		"max_rounds": maxRounds,
		"timeout":    timeout.Seconds(),
		"participants": len(participants),
		"created_at": time.Now().Unix(),
		"message":    "Debate started. Use GET /v1/debates/" + debateID + " to check status.",
	})
}

// runDebate executes the debate asynchronously
func (h *DebateHandler) runDebate(debateID string, config *services.DebateConfig) {
	h.mu.Lock()
	if state, exists := h.activeDebates[debateID]; exists {
		state.Status = "running"
	}
	h.mu.Unlock()

	// Conduct the debate
	var result *services.DebateResult
	var err error

	if h.advancedDebate != nil {
		result, err = h.advancedDebate.ConductAdvancedDebate(nil, config)
	} else if h.debateService != nil {
		result, err = h.debateService.ConductDebate(nil, config)
	} else {
		err = &debateError{message: "no debate service configured"}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if state, exists := h.activeDebates[debateID]; exists {
		now := time.Now()
		state.EndTime = &now

		if err != nil {
			state.Status = "failed"
			state.Error = err.Error()
			h.logger.WithError(err).WithField("debate_id", debateID).Error("Debate failed")
		} else {
			state.Status = "completed"
			state.Result = result
			h.logger.WithField("debate_id", debateID).Info("Debate completed successfully")
		}
	}
}

type debateError struct {
	message string
}

func (e *debateError) Error() string {
	return e.message
}

// GetDebate handles GET /v1/debates/:id
func (h *DebateHandler) GetDebate(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.RLock()
	state, exists := h.activeDebates[debateID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}

	response := gin.H{
		"debate_id":  debateID,
		"status":     state.Status,
		"topic":      state.Config.Topic,
		"max_rounds": state.Config.MaxRounds,
		"start_time": state.StartTime.Unix(),
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

	c.JSON(http.StatusOK, response)
}

// GetDebateStatus handles GET /v1/debates/:id/status
func (h *DebateHandler) GetDebateStatus(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.RLock()
	state, exists := h.activeDebates[debateID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}

	status := gin.H{
		"debate_id":  debateID,
		"status":     state.Status,
		"start_time": state.StartTime.Unix(),
	}

	if state.EndTime != nil {
		status["end_time"] = state.EndTime.Unix()
		status["duration_seconds"] = state.EndTime.Sub(state.StartTime).Seconds()
	}

	if state.Error != "" {
		status["error"] = state.Error
	}

	// Add progress info if running
	if state.Status == "running" && state.Config != nil {
		status["max_rounds"] = state.Config.MaxRounds
		status["timeout_seconds"] = state.Config.Timeout.Seconds()
	}

	c.JSON(http.StatusOK, status)
}

// GetDebateResults handles GET /v1/debates/:id/results
func (h *DebateHandler) GetDebateResults(c *gin.Context) {
	debateID := c.Param("id")

	h.mu.RLock()
	state, exists := h.activeDebates[debateID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Debate not found",
				"debate_id": debateID,
			},
		})
		return
	}

	if state.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Debate has not completed yet",
				"status":  state.Status,
			},
		})
		return
	}

	if state.Result == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Debate completed but no results available",
			},
		})
		return
	}

	c.JSON(http.StatusOK, state.Result)
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
				"message": "Debate not found",
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

// RegisterRoutes registers debate routes on a router group
func (h *DebateHandler) RegisterRoutes(rg *gin.RouterGroup) {
	debates := rg.Group("/debates")
	{
		debates.POST("", h.CreateDebate)
		debates.GET("", h.ListDebates)
		debates.GET("/:id", h.GetDebate)
		debates.GET("/:id/status", h.GetDebateStatus)
		debates.GET("/:id/results", h.GetDebateResults)
		debates.DELETE("/:id", h.DeleteDebate)
	}
}
