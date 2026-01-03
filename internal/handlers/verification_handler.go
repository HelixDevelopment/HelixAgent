package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/superagent/superagent/internal/verifier"
	"github.com/superagent/superagent/internal/verifier/adapters"
)

// VerificationHandler handles verification-related HTTP requests
type VerificationHandler struct {
	verificationService *verifier.VerificationService
	scoringService      *verifier.ScoringService
	healthService       *verifier.HealthService
	registry            *adapters.ExtendedProviderRegistry
}

// NewVerificationHandler creates a new verification handler
func NewVerificationHandler(
	vs *verifier.VerificationService,
	ss *verifier.ScoringService,
	hs *verifier.HealthService,
	reg *adapters.ExtendedProviderRegistry,
) *VerificationHandler {
	return &VerificationHandler{
		verificationService: vs,
		scoringService:      ss,
		healthService:       hs,
		registry:            reg,
	}
}

// VerifyModelRequest represents a model verification request
type VerifyModelRequest struct {
	ModelID    string   `json:"model_id" binding:"required"`
	Provider   string   `json:"provider" binding:"required"`
	Tests      []string `json:"tests,omitempty"`
	Timeout    int      `json:"timeout,omitempty"`
	RetryCount int      `json:"retry_count,omitempty"`
}

// VerifyModelResponse represents a model verification response
type VerifyModelResponse struct {
	ModelID           string          `json:"model_id"`
	Provider          string          `json:"provider"`
	Verified          bool            `json:"verified"`
	Score             float64         `json:"score"`
	OverallScore      float64         `json:"overall_score"`
	ScoreSuffix       string          `json:"score_suffix"`
	CodeVisible       bool            `json:"code_visible"`
	Tests             map[string]bool `json:"tests"`
	VerificationTime  int64           `json:"verification_time_ms"`
	Message           string          `json:"message,omitempty"`
}

// VerifyModel godoc
// @Summary Verify a model
// @Description Verify a specific model using LLMsVerifier including code visibility test
// @Tags verification
// @Accept json
// @Produce json
// @Param request body VerifyModelRequest true "Verification request"
// @Success 200 {object} VerifyModelResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/verifier/verify [post]
func (h *VerificationHandler) VerifyModel(c *gin.Context) {
	var req VerifyModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	result, err := h.verificationService.VerifyModel(c.Request.Context(), req.ModelID, req.Provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, VerifyModelResponse{
		ModelID:          result.ModelID,
		Provider:         result.Provider,
		Verified:         result.Verified,
		Score:            result.Score,
		OverallScore:     result.OverallScore,
		ScoreSuffix:      result.ScoreSuffix,
		CodeVisible:      result.CodeVisible,
		Tests:            result.Tests,
		VerificationTime: result.VerificationTimeMs,
		Message:          result.Message,
	})
}

// BatchVerifyRequest represents a batch verification request
type BatchVerifyRequest struct {
	Models []struct {
		ModelID  string `json:"model_id" binding:"required"`
		Provider string `json:"provider" binding:"required"`
	} `json:"models" binding:"required"`
}

// BatchVerifyResponse represents a batch verification response
type BatchVerifyResponse struct {
	Results []VerifyModelResponse `json:"results"`
	Summary struct {
		Total    int `json:"total"`
		Verified int `json:"verified"`
		Failed   int `json:"failed"`
	} `json:"summary"`
}

// BatchVerify godoc
// @Summary Batch verify models
// @Description Verify multiple models in a single request
// @Tags verification
// @Accept json
// @Produce json
// @Param request body BatchVerifyRequest true "Batch verification request"
// @Success 200 {object} BatchVerifyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/verifier/verify/batch [post]
func (h *VerificationHandler) BatchVerify(c *gin.Context) {
	var req BatchVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Convert to batch request
	batchReqs := make([]*verifier.BatchVerificationRequest, len(req.Models))
	for i, m := range req.Models {
		batchReqs[i] = &verifier.BatchVerificationRequest{
			ModelID:  m.ModelID,
			Provider: m.Provider,
		}
	}

	results, err := h.verificationService.BatchVerify(c.Request.Context(), batchReqs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	resp := BatchVerifyResponse{
		Results: make([]VerifyModelResponse, len(results)),
	}

	for i, r := range results {
		resp.Results[i] = VerifyModelResponse{
			ModelID:          r.ModelID,
			Provider:         r.Provider,
			Verified:         r.Verified,
			Score:            r.Score,
			OverallScore:     r.OverallScore,
			ScoreSuffix:      r.ScoreSuffix,
			CodeVisible:      r.CodeVisible,
			Tests:            r.Tests,
			VerificationTime: r.VerificationTimeMs,
			Message:          r.Message,
		}

		resp.Summary.Total++
		if r.Verified {
			resp.Summary.Verified++
		} else {
			resp.Summary.Failed++
		}
	}

	c.JSON(http.StatusOK, resp)
}

// GetVerificationStatus godoc
// @Summary Get verification status for a model
// @Description Get the current verification status for a specific model
// @Tags verification
// @Produce json
// @Param model_id path string true "Model ID"
// @Success 200 {object} VerifyModelResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/verifier/status/{model_id} [get]
func (h *VerificationHandler) GetVerificationStatus(c *gin.Context) {
	modelID := c.Param("model_id")

	result, err := h.verificationService.GetVerificationStatus(c.Request.Context(), modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, VerifyModelResponse{
		ModelID:          result.ModelID,
		Provider:         result.Provider,
		Verified:         result.Verified,
		Score:            result.Score,
		OverallScore:     result.OverallScore,
		ScoreSuffix:      result.ScoreSuffix,
		CodeVisible:      result.CodeVisible,
		Tests:            result.Tests,
		VerificationTime: result.VerificationTimeMs,
	})
}

// GetVerifiedModelsResponse represents verified models response
type GetVerifiedModelsResponse struct {
	Models []VerifiedModelInfo `json:"models"`
	Total  int                 `json:"total"`
}

// VerifiedModelInfo represents verified model information
type VerifiedModelInfo struct {
	ModelID      string  `json:"model_id"`
	ModelName    string  `json:"model_name"`
	Provider     string  `json:"provider"`
	Verified     bool    `json:"verified"`
	OverallScore float64 `json:"overall_score"`
	ScoreSuffix  string  `json:"score_suffix"`
	CodeVisible  bool    `json:"code_visible"`
}

// GetVerifiedModels godoc
// @Summary Get all verified models
// @Description Get a list of all models that have been verified
// @Tags verification
// @Produce json
// @Param provider query string false "Filter by provider"
// @Param min_score query number false "Minimum score filter"
// @Param require_code query bool false "Require code visibility"
// @Param limit query int false "Maximum number of results"
// @Success 200 {object} GetVerifiedModelsResponse
// @Router /api/v1/verifier/models [get]
func (h *VerificationHandler) GetVerifiedModels(c *gin.Context) {
	provider := c.Query("provider")
	minScore := 0.0
	if ms := c.Query("min_score"); ms != "" {
		// Parse min_score
	}
	requireCode := c.Query("require_code") == "true"
	limit := 100
	if l := c.Query("limit"); l != "" {
		// Parse limit
	}

	models := h.registry.GetTopModels(&adapters.TopModelsRequest{
		Limit:          limit,
		ProviderFilter: []string{provider},
		MinScore:       minScore,
		RequireCode:    requireCode,
	})

	resp := GetVerifiedModelsResponse{
		Models: make([]VerifiedModelInfo, len(models)),
		Total:  len(models),
	}

	for i, m := range models {
		resp.Models[i] = VerifiedModelInfo{
			ModelID:      m.ModelID,
			ModelName:    m.ModelName,
			Provider:     m.ProviderName,
			Verified:     m.Verified,
			OverallScore: m.OverallScore,
			ScoreSuffix:  m.ScoreSuffix,
			CodeVisible:  m.CodeVisible,
		}
	}

	c.JSON(http.StatusOK, resp)
}

// TestCodeVisibilityRequest represents a code visibility test request
type TestCodeVisibilityRequest struct {
	ModelID  string `json:"model_id" binding:"required"`
	Provider string `json:"provider" binding:"required"`
	Language string `json:"language,omitempty"`
}

// TestCodeVisibilityResponse represents a code visibility test response
type TestCodeVisibilityResponse struct {
	ModelID     string `json:"model_id"`
	Provider    string `json:"provider"`
	CodeVisible bool   `json:"code_visible"`
	Language    string `json:"language"`
	Prompt      string `json:"prompt"`
	Response    string `json:"response"`
	Confidence  float64 `json:"confidence"`
}

// TestCodeVisibility godoc
// @Summary Test if model can see code
// @Description Tests if the model can see injected code using "Do you see my code?" verification
// @Tags verification
// @Accept json
// @Produce json
// @Param request body TestCodeVisibilityRequest true "Code visibility test request"
// @Success 200 {object} TestCodeVisibilityResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/verifier/test/code-visibility [post]
func (h *VerificationHandler) TestCodeVisibility(c *gin.Context) {
	var req TestCodeVisibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	language := req.Language
	if language == "" {
		language = "python"
	}

	result, err := h.verificationService.TestCodeVisibility(c.Request.Context(), req.ModelID, req.Provider, language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, TestCodeVisibilityResponse{
		ModelID:     result.ModelID,
		Provider:    result.Provider,
		CodeVisible: result.CodeVisible,
		Language:    result.Language,
		Prompt:      result.Prompt,
		Response:    result.Response,
		Confidence:  result.Confidence,
	})
}

// GetVerificationTests godoc
// @Summary Get available verification tests
// @Description Get a list of all available verification tests
// @Tags verification
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/v1/verifier/tests [get]
func (h *VerificationHandler) GetVerificationTests(c *gin.Context) {
	tests := map[string]string{
		"code_visibility":    "Tests if the model can see injected code",
		"existence":          "Verifies model exists and responds",
		"responsiveness":     "Tests model response time and quality",
		"latency":            "Measures model latency",
		"streaming":          "Tests streaming support",
		"function_calling":   "Tests function calling capability",
		"coding_capability":  "Tests coding ability",
		"error_detection":    "Tests error detection in code",
		"context_handling":   "Tests context window handling",
		"multilingual":       "Tests multilingual support",
		"reasoning":          "Tests reasoning capability",
		"instruction_follow": "Tests instruction following",
	}

	c.JSON(http.StatusOK, tests)
}

// ReVerifyModelRequest represents a re-verification request
type ReVerifyModelRequest struct {
	ModelID  string   `json:"model_id" binding:"required"`
	Provider string   `json:"provider" binding:"required"`
	Force    bool     `json:"force"`
	Tests    []string `json:"tests,omitempty"`
}

// ReVerifyModel godoc
// @Summary Re-verify a model
// @Description Force re-verification of a previously verified model
// @Tags verification
// @Accept json
// @Produce json
// @Param request body ReVerifyModelRequest true "Re-verification request"
// @Success 200 {object} VerifyModelResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/verifier/reverify [post]
func (h *VerificationHandler) ReVerifyModel(c *gin.Context) {
	var req ReVerifyModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Invalidate existing verification
	h.verificationService.InvalidateVerification(req.ModelID)

	// Run new verification
	result, err := h.verificationService.VerifyModel(c.Request.Context(), req.ModelID, req.Provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, VerifyModelResponse{
		ModelID:          result.ModelID,
		Provider:         result.Provider,
		Verified:         result.Verified,
		Score:            result.Score,
		OverallScore:     result.OverallScore,
		ScoreSuffix:      result.ScoreSuffix,
		CodeVisible:      result.CodeVisible,
		Tests:            result.Tests,
		VerificationTime: result.VerificationTimeMs,
		Message:          "Re-verification completed",
	})
}

// VerificationHealthResponse represents verification service health response
type VerificationHealthResponse struct {
	Status           string `json:"status"`
	VerifiedModels   int    `json:"verified_models"`
	PendingModels    int    `json:"pending_models"`
	HealthyProviders int    `json:"healthy_providers"`
	TotalProviders   int    `json:"total_providers"`
	LastVerification string `json:"last_verification,omitempty"`
}

// GetVerificationHealth godoc
// @Summary Get verification service health
// @Description Get the health status of the verification service
// @Tags verification
// @Produce json
// @Success 200 {object} VerificationHealthResponse
// @Router /api/v1/verifier/health [get]
func (h *VerificationHandler) GetVerificationHealth(c *gin.Context) {
	stats := h.verificationService.GetStats()
	healthy := h.registry.GetHealthyProviders()

	c.JSON(http.StatusOK, VerificationHealthResponse{
		Status:           "healthy",
		VerifiedModels:   stats.VerifiedModels,
		PendingModels:    stats.PendingModels,
		HealthyProviders: len(healthy),
		TotalProviders:   stats.TotalProviders,
		LastVerification: stats.LastVerificationAt,
	})
}

// RegisterVerificationRoutes registers verification routes
func RegisterVerificationRoutes(r *gin.RouterGroup, h *VerificationHandler) {
	verifier := r.Group("/verifier")
	{
		verifier.POST("/verify", h.VerifyModel)
		verifier.POST("/verify/batch", h.BatchVerify)
		verifier.GET("/status/:model_id", h.GetVerificationStatus)
		verifier.GET("/models", h.GetVerifiedModels)
		verifier.POST("/test/code-visibility", h.TestCodeVisibility)
		verifier.GET("/tests", h.GetVerificationTests)
		verifier.POST("/reverify", h.ReVerifyModel)
		verifier.GET("/health", h.GetVerificationHealth)
	}
}
