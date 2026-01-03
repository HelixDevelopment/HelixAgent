package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/superagent/superagent/internal/verifier"
)

// DiscoveryHandler handles model discovery HTTP requests
type DiscoveryHandler struct {
	discoveryService *verifier.ModelDiscoveryService
}

// NewDiscoveryHandler creates a new discovery handler
func NewDiscoveryHandler(ds *verifier.ModelDiscoveryService) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryService: ds,
	}
}

// DiscoveredModelResponse represents a discovered model response
type DiscoveredModelResponse struct {
	ModelID       string   `json:"model_id"`
	ModelName     string   `json:"model_name"`
	Provider      string   `json:"provider"`
	Verified      bool     `json:"verified"`
	CodeVisible   bool     `json:"code_visible"`
	OverallScore  float64  `json:"overall_score"`
	ScoreSuffix   string   `json:"score_suffix"`
	DiscoveredAt  string   `json:"discovered_at"`
	Capabilities  []string `json:"capabilities,omitempty"`
}

// GetDiscoveredModels godoc
// @Summary Get all discovered models
// @Description Get all models discovered from configured providers
// @Tags discovery
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/verifier/discovery/models [get]
func (h *DiscoveryHandler) GetDiscoveredModels(c *gin.Context) {
	models := h.discoveryService.GetDiscoveredModels()

	response := make([]DiscoveredModelResponse, len(models))
	for i, m := range models {
		response[i] = DiscoveredModelResponse{
			ModelID:      m.ModelID,
			ModelName:    m.ModelName,
			Provider:     m.Provider,
			Verified:     m.Verified,
			CodeVisible:  m.CodeVisible,
			OverallScore: m.OverallScore,
			ScoreSuffix:  m.ScoreSuffix,
			DiscoveredAt: m.DiscoveredAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"models": response,
		"total":  len(response),
	})
}

// SelectedModelResponse represents a selected model response
type SelectedModelResponse struct {
	ModelID      string  `json:"model_id"`
	ModelName    string  `json:"model_name"`
	Provider     string  `json:"provider"`
	OverallScore float64 `json:"overall_score"`
	ScoreSuffix  string  `json:"score_suffix"`
	Rank         int     `json:"rank"`
	VoteWeight   float64 `json:"vote_weight"`
	CodeVisible  bool    `json:"code_visible"`
	SelectedAt   string  `json:"selected_at"`
}

// GetSelectedModels godoc
// @Summary Get selected models for AI debate
// @Description Get the top-scoring models selected for AI debate ensemble
// @Tags discovery
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/verifier/discovery/selected [get]
func (h *DiscoveryHandler) GetSelectedModels(c *gin.Context) {
	models := h.discoveryService.GetSelectedModels()

	response := make([]SelectedModelResponse, len(models))
	for i, m := range models {
		response[i] = SelectedModelResponse{
			ModelID:      m.ModelID,
			ModelName:    m.ModelName,
			Provider:     m.Provider,
			OverallScore: m.OverallScore,
			ScoreSuffix:  m.ScoreSuffix,
			Rank:         m.Rank,
			VoteWeight:   m.VoteWeight,
			CodeVisible:  m.CodeVisible,
			SelectedAt:   m.SelectedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"models":      response,
		"total":       len(response),
		"description": "These models are automatically selected for AI debate ensemble based on verification and scoring",
	})
}

// GetDiscoveryStats godoc
// @Summary Get discovery statistics
// @Description Get statistics about model discovery, verification, and selection
// @Tags discovery
// @Produce json
// @Success 200 {object} verifier.DiscoveryStats
// @Router /api/v1/verifier/discovery/stats [get]
func (h *DiscoveryHandler) GetDiscoveryStats(c *gin.Context) {
	stats := h.discoveryService.GetDiscoveryStats()

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
		"description": map[string]string{
			"total_discovered":   "Total models discovered from all providers",
			"total_verified":     "Models that passed verification (including code visibility test)",
			"total_selected":     "Models selected for AI debate ensemble",
			"code_visible_count": "Models that passed the 'Do you see my code?' test",
			"average_score":      "Average score of selected models (0-10 scale)",
			"by_provider":        "Breakdown of discovered models by provider",
		},
	})
}

// TriggerDiscoveryRequest represents a trigger discovery request
type TriggerDiscoveryRequest struct {
	Providers []struct {
		Name    string `json:"name" binding:"required"`
		APIKey  string `json:"api_key" binding:"required"`
		BaseURL string `json:"base_url,omitempty"`
	} `json:"providers" binding:"required"`
}

// TriggerDiscovery godoc
// @Summary Trigger model discovery
// @Description Manually trigger model discovery for specified providers
// @Tags discovery
// @Accept json
// @Produce json
// @Param request body TriggerDiscoveryRequest true "Providers to discover from"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/verifier/discovery/trigger [post]
func (h *DiscoveryHandler) TriggerDiscovery(c *gin.Context) {
	var req TriggerDiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	credentials := make([]verifier.ProviderCredentials, len(req.Providers))
	for i, p := range req.Providers {
		credentials[i] = verifier.ProviderCredentials{
			ProviderName: p.Name,
			APIKey:       p.APIKey,
			BaseURL:      p.BaseURL,
		}
	}

	// Trigger discovery in background
	go func() {
		h.discoveryService.Start(credentials)
	}()

	c.JSON(http.StatusOK, gin.H{
		"message":   "Discovery triggered",
		"providers": len(req.Providers),
		"status":    "Discovery running in background. Check /discovery/stats for progress.",
	})
}

// GetEnsembleModels godoc
// @Summary Get models for AI debate ensemble
// @Description Get the models currently used for AI debate with their vote weights
// @Tags discovery
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/verifier/discovery/ensemble [get]
func (h *DiscoveryHandler) GetEnsembleModels(c *gin.Context) {
	models := h.discoveryService.GetSelectedModels()

	type EnsembleModel struct {
		ModelID          string  `json:"model_id"`
		DisplayName      string  `json:"display_name"`
		Provider         string  `json:"provider"`
		Score            float64 `json:"score"`
		VoteWeight       float64 `json:"vote_weight"`
		VoteWeightPct    string  `json:"vote_weight_pct"`
		CodeVisible      bool    `json:"code_visible"`
		RecommendedFor   []string `json:"recommended_for"`
	}

	var totalWeight float64
	for _, m := range models {
		totalWeight += m.VoteWeight
	}

	ensemble := make([]EnsembleModel, len(models))
	for i, m := range models {
		weightPct := 0.0
		if totalWeight > 0 {
			weightPct = (m.VoteWeight / totalWeight) * 100
		}

		ensemble[i] = EnsembleModel{
			ModelID:       m.ModelID,
			DisplayName:   m.ModelName + " " + m.ScoreSuffix,
			Provider:      m.Provider,
			Score:         m.OverallScore,
			VoteWeight:    m.VoteWeight,
			VoteWeightPct: formatPercent(weightPct),
			CodeVisible:   m.CodeVisible,
			RecommendedFor: getRecommendations(m.OverallScore),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ensemble": ensemble,
		"total_models": len(ensemble),
		"total_vote_weight": totalWeight,
		"description": "These models participate in AI debate. Votes are weighted by verification score.",
		"how_it_works": []string{
			"1. User query is sent to all ensemble models",
			"2. Models provide initial responses",
			"3. Models critique each other's responses",
			"4. Models update their positions based on valid critiques",
			"5. Weighted voting determines consensus",
			"6. Best answer is synthesized from consensus",
		},
	})
}

// GetModelForDebate godoc
// @Summary Get a specific model for debate
// @Description Get details of a specific model if it's selected for AI debate
// @Tags discovery
// @Produce json
// @Param model_id path string true "Model ID"
// @Success 200 {object} SelectedModelResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/verifier/discovery/ensemble/{model_id} [get]
func (h *DiscoveryHandler) GetModelForDebate(c *gin.Context) {
	modelID := c.Param("model_id")

	model, found := h.discoveryService.GetModelForDebate(modelID)
	if !found {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Model not in ensemble: " + modelID})
		return
	}

	c.JSON(http.StatusOK, SelectedModelResponse{
		ModelID:      model.ModelID,
		ModelName:    model.ModelName,
		Provider:     model.Provider,
		OverallScore: model.OverallScore,
		ScoreSuffix:  model.ScoreSuffix,
		Rank:         model.Rank,
		VoteWeight:   model.VoteWeight,
		CodeVisible:  model.CodeVisible,
		SelectedAt:   model.SelectedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// RegisterDiscoveryRoutes registers discovery routes
func RegisterDiscoveryRoutes(r *gin.RouterGroup, h *DiscoveryHandler) {
	discovery := r.Group("/verifier/discovery")
	{
		discovery.GET("/models", h.GetDiscoveredModels)
		discovery.GET("/selected", h.GetSelectedModels)
		discovery.GET("/stats", h.GetDiscoveryStats)
		discovery.POST("/trigger", h.TriggerDiscovery)
		discovery.GET("/ensemble", h.GetEnsembleModels)
		discovery.GET("/ensemble/:model_id", h.GetModelForDebate)
	}
}

// Helper functions
func formatPercent(p float64) string {
	if p >= 10 {
		return fmt.Sprintf("%.1f%%", p)
	}
	return fmt.Sprintf("%.2f%%", p)
}

func getRecommendations(score float64) []string {
	if score >= 9.0 {
		return []string{"complex reasoning", "code generation", "creative writing", "analysis"}
	}
	if score >= 8.0 {
		return []string{"general tasks", "summarization", "Q&A", "code review"}
	}
	if score >= 7.0 {
		return []string{"simple tasks", "quick responses", "basic Q&A"}
	}
	return []string{"fallback", "high-volume tasks"}
}

