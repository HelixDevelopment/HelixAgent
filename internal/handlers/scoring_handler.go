package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/superagent/superagent/internal/verifier"
)

// ScoringHandler handles scoring-related HTTP requests
type ScoringHandler struct {
	scoringService *verifier.ScoringService
}

// NewScoringHandler creates a new scoring handler
func NewScoringHandler(ss *verifier.ScoringService) *ScoringHandler {
	return &ScoringHandler{
		scoringService: ss,
	}
}

// GetModelScoreResponse represents a model score response
type GetModelScoreResponse struct {
	ModelID      string                `json:"model_id"`
	ModelName    string                `json:"model_name"`
	OverallScore float64               `json:"overall_score"`
	ScoreSuffix  string                `json:"score_suffix"`
	Components   ScoreComponentsDetail `json:"components"`
	CalculatedAt string                `json:"calculated_at"`
	DataSource   string                `json:"data_source"`
}

// ScoreComponentsDetail represents detailed score components
type ScoreComponentsDetail struct {
	SpeedScore      float64 `json:"speed_score"`
	EfficiencyScore float64 `json:"efficiency_score"`
	CostScore       float64 `json:"cost_score"`
	CapabilityScore float64 `json:"capability_score"`
	RecencyScore    float64 `json:"recency_score"`
}

// GetModelScore godoc
// @Summary Get score for a model
// @Description Get comprehensive score for a specific model
// @Tags scoring
// @Produce json
// @Param model_id path string true "Model ID"
// @Success 200 {object} GetModelScoreResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/verifier/scores/{model_id} [get]
func (h *ScoringHandler) GetModelScore(c *gin.Context) {
	modelID := c.Param("model_id")

	result, err := h.scoringService.CalculateScore(c.Request.Context(), modelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetModelScoreResponse{
		ModelID:      result.ModelID,
		ModelName:    result.ModelName,
		OverallScore: result.OverallScore,
		ScoreSuffix:  result.ScoreSuffix,
		Components: ScoreComponentsDetail{
			SpeedScore:      result.Components.SpeedScore,
			EfficiencyScore: result.Components.EfficiencyScore,
			CostScore:       result.Components.CostScore,
			CapabilityScore: result.Components.CapabilityScore,
			RecencyScore:    result.Components.RecencyScore,
		},
		CalculatedAt: result.CalculatedAt.Format("2006-01-02T15:04:05Z"),
		DataSource:   result.DataSource,
	})
}

// BatchScoreRequest represents a batch scoring request
type BatchScoreRequest struct {
	ModelIDs []string `json:"model_ids" binding:"required"`
}

// BatchScoreResponse represents a batch scoring response
type BatchScoreResponse struct {
	Scores []GetModelScoreResponse `json:"scores"`
	Total  int                     `json:"total"`
}

// BatchCalculateScores godoc
// @Summary Calculate scores for multiple models
// @Description Calculate comprehensive scores for multiple models in batch
// @Tags scoring
// @Accept json
// @Produce json
// @Param request body BatchScoreRequest true "Batch score request"
// @Success 200 {object} BatchScoreResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/verifier/scores/batch [post]
func (h *ScoringHandler) BatchCalculateScores(c *gin.Context) {
	var req BatchScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	results, err := h.scoringService.BatchCalculateScores(c.Request.Context(), req.ModelIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	resp := BatchScoreResponse{
		Scores: make([]GetModelScoreResponse, len(results)),
		Total:  len(results),
	}

	for i, r := range results {
		resp.Scores[i] = GetModelScoreResponse{
			ModelID:      r.ModelID,
			ModelName:    r.ModelName,
			OverallScore: r.OverallScore,
			ScoreSuffix:  r.ScoreSuffix,
			Components: ScoreComponentsDetail{
				SpeedScore:      r.Components.SpeedScore,
				EfficiencyScore: r.Components.EfficiencyScore,
				CostScore:       r.Components.CostScore,
				CapabilityScore: r.Components.CapabilityScore,
				RecencyScore:    r.Components.RecencyScore,
			},
			CalculatedAt: r.CalculatedAt.Format("2006-01-02T15:04:05Z"),
			DataSource:   r.DataSource,
		}
	}

	c.JSON(http.StatusOK, resp)
}

// GetTopModelsResponse represents top models response
type GetTopModelsResponse struct {
	Models []ModelWithScoreInfo `json:"models"`
	Total  int                  `json:"total"`
}

// ModelWithScoreInfo represents a model with score info
type ModelWithScoreInfo struct {
	ModelID      string  `json:"model_id"`
	Name         string  `json:"name"`
	Provider     string  `json:"provider"`
	OverallScore float64 `json:"overall_score"`
	ScoreSuffix  string  `json:"score_suffix"`
	Rank         int     `json:"rank"`
}

// GetTopModels godoc
// @Summary Get top scoring models
// @Description Get a list of top scoring models
// @Tags scoring
// @Produce json
// @Param limit query int false "Number of models to return (default 10)"
// @Success 200 {object} GetTopModelsResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/verifier/scores/top [get]
func (h *ScoringHandler) GetTopModels(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	models, err := h.scoringService.GetTopModels(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	resp := GetTopModelsResponse{
		Models: make([]ModelWithScoreInfo, len(models)),
		Total:  len(models),
	}

	for i, m := range models {
		resp.Models[i] = ModelWithScoreInfo{
			ModelID:      m.ModelID,
			Name:         m.Name,
			Provider:     m.Provider,
			OverallScore: m.OverallScore,
			ScoreSuffix:  m.ScoreSuffix,
			Rank:         i + 1,
		}
	}

	c.JSON(http.StatusOK, resp)
}

// GetModelsByScoreRangeRequest represents a score range request
type GetModelsByScoreRangeRequest struct {
	MinScore float64 `form:"min_score" binding:"gte=0,lte=10"`
	MaxScore float64 `form:"max_score" binding:"gte=0,lte=10"`
	Limit    int     `form:"limit"`
}

// GetModelsByScoreRange godoc
// @Summary Get models by score range
// @Description Get models within a specific score range
// @Tags scoring
// @Produce json
// @Param min_score query number true "Minimum score"
// @Param max_score query number true "Maximum score"
// @Param limit query int false "Maximum number of results"
// @Success 200 {object} GetTopModelsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/verifier/scores/range [get]
func (h *ScoringHandler) GetModelsByScoreRange(c *gin.Context) {
	minScore := 0.0
	maxScore := 10.0
	limit := 50

	if ms := c.Query("min_score"); ms != "" {
		if parsed, err := strconv.ParseFloat(ms, 64); err == nil {
			minScore = parsed
		}
	}

	if ms := c.Query("max_score"); ms != "" {
		if parsed, err := strconv.ParseFloat(ms, 64); err == nil {
			maxScore = parsed
		}
	}

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	models, err := h.scoringService.GetModelsByScoreRange(c.Request.Context(), minScore, maxScore, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	resp := GetTopModelsResponse{
		Models: make([]ModelWithScoreInfo, len(models)),
		Total:  len(models),
	}

	for i, m := range models {
		resp.Models[i] = ModelWithScoreInfo{
			ModelID:      m.ModelID,
			Name:         m.Name,
			Provider:     m.Provider,
			OverallScore: m.OverallScore,
			ScoreSuffix:  m.ScoreSuffix,
			Rank:         i + 1,
		}
	}

	c.JSON(http.StatusOK, resp)
}

// ScoringWeightsResponse represents scoring weights response
type ScoringWeightsResponse struct {
	Weights       ScoringWeightsDetail `json:"weights"`
	Total         float64              `json:"total"`
	IsValid       bool                 `json:"is_valid"`
	Description   string               `json:"description"`
}

// ScoringWeightsDetail represents detailed scoring weights
type ScoringWeightsDetail struct {
	ResponseSpeed     float64 `json:"response_speed"`
	ModelEfficiency   float64 `json:"model_efficiency"`
	CostEffectiveness float64 `json:"cost_effectiveness"`
	Capability        float64 `json:"capability"`
	Recency           float64 `json:"recency"`
}

// GetScoringWeights godoc
// @Summary Get current scoring weights
// @Description Get the current scoring weights used for model scoring
// @Tags scoring
// @Produce json
// @Success 200 {object} ScoringWeightsResponse
// @Router /api/v1/verifier/scores/weights [get]
func (h *ScoringHandler) GetScoringWeights(c *gin.Context) {
	weights := h.scoringService.GetWeights()

	total := weights.ResponseSpeed + weights.ModelEfficiency +
		weights.CostEffectiveness + weights.Capability + weights.Recency

	c.JSON(http.StatusOK, ScoringWeightsResponse{
		Weights: ScoringWeightsDetail{
			ResponseSpeed:     weights.ResponseSpeed,
			ModelEfficiency:   weights.ModelEfficiency,
			CostEffectiveness: weights.CostEffectiveness,
			Capability:        weights.Capability,
			Recency:           weights.Recency,
		},
		Total:   total,
		IsValid: total >= 0.99 && total <= 1.01,
		Description: "Score = (Speed × 25%) + (Efficiency × 20%) + (Cost × 25%) + (Capability × 20%) + (Recency × 10%)",
	})
}

// UpdateScoringWeightsRequest represents a weights update request
type UpdateScoringWeightsRequest struct {
	ResponseSpeed     float64 `json:"response_speed" binding:"gte=0,lte=1"`
	ModelEfficiency   float64 `json:"model_efficiency" binding:"gte=0,lte=1"`
	CostEffectiveness float64 `json:"cost_effectiveness" binding:"gte=0,lte=1"`
	Capability        float64 `json:"capability" binding:"gte=0,lte=1"`
	Recency           float64 `json:"recency" binding:"gte=0,lte=1"`
}

// UpdateScoringWeights godoc
// @Summary Update scoring weights
// @Description Update the scoring weights used for model scoring (must sum to 1.0)
// @Tags scoring
// @Accept json
// @Produce json
// @Param request body UpdateScoringWeightsRequest true "New weights"
// @Success 200 {object} ScoringWeightsResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/verifier/scores/weights [put]
func (h *ScoringHandler) UpdateScoringWeights(c *gin.Context) {
	var req UpdateScoringWeightsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	weights := &verifier.ScoreWeights{
		ResponseSpeed:     req.ResponseSpeed,
		ModelEfficiency:   req.ModelEfficiency,
		CostEffectiveness: req.CostEffectiveness,
		Capability:        req.Capability,
		Recency:           req.Recency,
	}

	if err := h.scoringService.UpdateWeights(weights); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	total := req.ResponseSpeed + req.ModelEfficiency +
		req.CostEffectiveness + req.Capability + req.Recency

	c.JSON(http.StatusOK, ScoringWeightsResponse{
		Weights: ScoringWeightsDetail{
			ResponseSpeed:     weights.ResponseSpeed,
			ModelEfficiency:   weights.ModelEfficiency,
			CostEffectiveness: weights.CostEffectiveness,
			Capability:        weights.Capability,
			Recency:           weights.Recency,
		},
		Total:   total,
		IsValid: true,
		Description: "Weights updated successfully",
	})
}

// GetModelNameWithScore godoc
// @Summary Get model name with score suffix
// @Description Get the model name appended with its score suffix (e.g., "GPT-4 (SC:9.2)")
// @Tags scoring
// @Produce json
// @Param model_id path string true "Model ID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/verifier/scores/{model_id}/name [get]
func (h *ScoringHandler) GetModelNameWithScore(c *gin.Context) {
	modelID := c.Param("model_id")

	name, err := h.scoringService.GetModelNameWithScore(c.Request.Context(), modelID, modelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"model_id":        modelID,
		"name_with_score": name,
	})
}

// InvalidateCacheRequest represents a cache invalidation request
type InvalidateCacheRequest struct {
	ModelID string `json:"model_id,omitempty"`
	All     bool   `json:"all,omitempty"`
}

// InvalidateCache godoc
// @Summary Invalidate score cache
// @Description Invalidate cached scores for a specific model or all models
// @Tags scoring
// @Accept json
// @Produce json
// @Param request body InvalidateCacheRequest true "Cache invalidation request"
// @Success 200 {object} map[string]string
// @Router /api/v1/verifier/scores/cache/invalidate [post]
func (h *ScoringHandler) InvalidateCache(c *gin.Context) {
	var req InvalidateCacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if req.All {
		h.scoringService.InvalidateAllCache()
		c.JSON(http.StatusOK, gin.H{
			"message": "All cached scores invalidated",
		})
		return
	}

	if req.ModelID != "" {
		h.scoringService.InvalidateCache(req.ModelID)
		c.JSON(http.StatusOK, gin.H{
			"message": "Cache invalidated for model: " + req.ModelID,
		})
		return
	}

	c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Specify model_id or set all=true"})
}

// CompareModelsRequest represents a model comparison request
type CompareModelsRequest struct {
	ModelIDs []string `json:"model_ids" binding:"required,min=2,max=10"`
}

// CompareModelsResponse represents a model comparison response
type CompareModelsResponse struct {
	Models     []GetModelScoreResponse `json:"models"`
	Winner     string                  `json:"winner"`
	Comparison map[string]float64      `json:"comparison"`
}

// CompareModels godoc
// @Summary Compare multiple models
// @Description Compare scores of multiple models side by side
// @Tags scoring
// @Accept json
// @Produce json
// @Param request body CompareModelsRequest true "Models to compare"
// @Success 200 {object} CompareModelsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/verifier/scores/compare [post]
func (h *ScoringHandler) CompareModels(c *gin.Context) {
	var req CompareModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	results, err := h.scoringService.BatchCalculateScores(c.Request.Context(), req.ModelIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	resp := CompareModelsResponse{
		Models:     make([]GetModelScoreResponse, len(results)),
		Comparison: make(map[string]float64),
	}

	var winner string
	var highestScore float64

	for i, r := range results {
		resp.Models[i] = GetModelScoreResponse{
			ModelID:      r.ModelID,
			ModelName:    r.ModelName,
			OverallScore: r.OverallScore,
			ScoreSuffix:  r.ScoreSuffix,
			Components: ScoreComponentsDetail{
				SpeedScore:      r.Components.SpeedScore,
				EfficiencyScore: r.Components.EfficiencyScore,
				CostScore:       r.Components.CostScore,
				CapabilityScore: r.Components.CapabilityScore,
				RecencyScore:    r.Components.RecencyScore,
			},
			CalculatedAt: r.CalculatedAt.Format("2006-01-02T15:04:05Z"),
			DataSource:   r.DataSource,
		}

		resp.Comparison[r.ModelID] = r.OverallScore

		if r.OverallScore > highestScore {
			highestScore = r.OverallScore
			winner = r.ModelID
		}
	}

	resp.Winner = winner

	c.JSON(http.StatusOK, resp)
}

// RegisterScoringRoutes registers scoring routes
func RegisterScoringRoutes(r *gin.RouterGroup, h *ScoringHandler) {
	scores := r.Group("/verifier/scores")
	{
		scores.GET("/:model_id", h.GetModelScore)
		scores.GET("/:model_id/name", h.GetModelNameWithScore)
		scores.POST("/batch", h.BatchCalculateScores)
		scores.GET("/top", h.GetTopModels)
		scores.GET("/range", h.GetModelsByScoreRange)
		scores.GET("/weights", h.GetScoringWeights)
		scores.PUT("/weights", h.UpdateScoringWeights)
		scores.POST("/cache/invalidate", h.InvalidateCache)
		scores.POST("/compare", h.CompareModels)
	}
}
