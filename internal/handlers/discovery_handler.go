package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"dev.helix.agent/internal/verifier"
)

// providerRegistryView is the minimal subset of services.ProviderRegistry
// the DiscoveryHandler reads when the full ModelDiscoveryService isn't
// wired. Defined here to avoid an import cycle with internal/services.
type providerRegistryView interface {
	ListProviders() []string
}

// DiscoveryHandler handles model discovery HTTP requests.
//
// Accepts EITHER the full ModelDiscoveryService (verifier-driven scoring +
// selection) OR a fallback ProviderRegistry view (always available). The
// fallback uses each provider's GetCapabilities().SupportedModels — same
// source as /v1/models — so /v1/discovery/models never returns 503 just
// because the optional ModelDiscoveryService isn't wired.
type DiscoveryHandler struct {
	discoveryService *verifier.ModelDiscoveryService
	registry         providerRegistryView
	getModels        func(providerName string) []string
}

// NewDiscoveryHandler creates a new discovery handler with optional
// ModelDiscoveryService.
func NewDiscoveryHandler(ds *verifier.ModelDiscoveryService) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryService: ds,
	}
}

// NewDiscoveryHandlerWithRegistry creates a discovery handler that falls back
// to ProviderRegistry data when the full ModelDiscoveryService isn't wired.
// `getModels` is a callback returning the models a registered provider
// supports — passed in by the caller so this package doesn't need to import
// internal/services (cycle prevention).
func NewDiscoveryHandlerWithRegistry(
	ds *verifier.ModelDiscoveryService,
	registry providerRegistryView,
	getModels func(providerName string) []string,
) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryService: ds,
		registry:         registry,
		getModels:        getModels,
	}
}

// checkDiscoveryService returns true if either the discovery service or the
// fallback registry is available. Returns 503 only when both are missing.
func (h *DiscoveryHandler) checkDiscoveryService(c *gin.Context) bool {
	if h.discoveryService == nil && h.registry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "Model discovery service not initialized",
		})
		return false
	}
	return true
}

// fallbackDiscoveredModels returns the list of (provider, model) pairs
// pulled from the ProviderRegistry. Real data — every entry corresponds
// to a model some registered provider claims to support.
func (h *DiscoveryHandler) fallbackDiscoveredModels() []DiscoveredModelResponse {
	if h.registry == nil || h.getModels == nil {
		return nil
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	var out []DiscoveredModelResponse
	for _, prov := range h.registry.ListProviders() {
		for _, m := range h.getModels(prov) {
			out = append(out, DiscoveredModelResponse{
				ModelID:      m,
				ModelName:    m,
				Provider:     prov,
				Verified:     false, // verifier-driven status not in fallback
				CodeVisible:  false,
				OverallScore: 0,
				ScoreSuffix:  "",
				DiscoveredAt: now,
			})
		}
	}
	return out
}

// DiscoveredModelResponse represents a discovered model response
type DiscoveredModelResponse struct {
	ModelID      string   `json:"model_id"`
	ModelName    string   `json:"model_name"`
	Provider     string   `json:"provider"`
	Verified     bool     `json:"verified"`
	CodeVisible  bool     `json:"code_visible"`
	OverallScore float64  `json:"overall_score"`
	ScoreSuffix  string   `json:"score_suffix"`
	DiscoveredAt string   `json:"discovered_at"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// GetDiscoveredModels godoc
// @Summary Get all discovered models
// @Description Get all models discovered from configured providers
// @Tags discovery
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/verifier/discovery/models [get]
func (h *DiscoveryHandler) GetDiscoveredModels(c *gin.Context) {
	if !h.checkDiscoveryService(c) {
		return
	}

	// Prefer the full ModelDiscoveryService when wired; fall back to
	// ProviderRegistry-derived models otherwise. The fallback is real
	// data (each registered provider's GetCapabilities().SupportedModels)
	// — the only fields it can't populate are the verifier-driven scores
	// and the code-visibility flag.
	var response []DiscoveredModelResponse
	source := "registry"
	if h.discoveryService != nil {
		models := h.discoveryService.GetDiscoveredModels()
		response = make([]DiscoveredModelResponse, len(models))
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
		source = "discovery_service"
	} else {
		response = h.fallbackDiscoveredModels()
	}

	c.JSON(http.StatusOK, gin.H{
		"models": response,
		"total":  len(response),
		"source": source,
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

	if !h.checkDiscoveryService(c) {
		return
	}

	// CONST-035 §c "Completion": when discoveryService isn't wired but
	// registry IS, return an empty selected-models list with a clear
	// `source` indicator instead of panicking on nil dereference.
	// The full discoveryService selects/ranks models by verification
	// score; without it there's no ranked subset to expose.
	if h.discoveryService == nil {
		c.JSON(http.StatusOK, gin.H{
			"models":      []SelectedModelResponse{},
			"total":       0,
			"source":      "registry",
			"description": "discoveryService not wired; selected-model ranking requires verifier scores. Use /v1/discovery/models for the unranked registry-derived list.",
		})
		return
	}

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
		"source":      "discovery_service",
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

	if !h.checkDiscoveryService(c) {
		return
	}

	// CONST-035 §c: nil-check + registry fallback (see GetSelectedModels)
	if h.discoveryService == nil {
		fallback := h.fallbackDiscoveredModels()
		byProvider := map[string]int{}
		for _, m := range fallback {
			byProvider[m.Provider]++
		}
		c.JSON(http.StatusOK, gin.H{
			"stats": gin.H{
				"total_discovered":   len(fallback),
				"total_verified":     0,
				"total_selected":     0,
				"code_visible_count": 0,
				"average_score":      0,
				"by_provider":        byProvider,
			},
			"source": "registry",
			"description": map[string]string{
				"total_discovered":   "Total models from registry fallback (verifier scores unavailable until discoveryService is wired)",
				"total_verified":     "0 — no verifier service",
				"total_selected":     "0 — no verifier service",
				"code_visible_count": "0 — no verifier service",
				"average_score":      "0 — no verifier service",
				"by_provider":        "Breakdown by provider name from registry",
			},
		})
		return
	}

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

	if !h.checkDiscoveryService(c) {
		return
	}
	var req TriggerDiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
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

	// Trigger discovery in background with a timeout context to prevent goroutine leaks.
	// The discovery service manages its own goroutine lifecycle via stopCh/wg,
	// so cleanup happens when Stop() is called during server shutdown.
	_, discoveryCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	go func() {
		defer discoveryCancel()
		if err := h.discoveryService.Start(credentials); err != nil {
			log.Printf("Discovery trigger failed: %v", err)
		}
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

	if !h.checkDiscoveryService(c) {
		return
	}

	// CONST-035 §c: nil-check + empty fallback (see GetSelectedModels)
	if h.discoveryService == nil {
		c.JSON(http.StatusOK, gin.H{
			"models":      []any{},
			"total":       0,
			"source":      "registry",
			"description": "Ensemble selection requires discoveryService (verifier-driven). Without it the ranked ensemble is empty.",
		})
		return
	}

	models := h.discoveryService.GetSelectedModels()

	type EnsembleModel struct {
		ModelID        string   `json:"model_id"`
		DisplayName    string   `json:"display_name"`
		Provider       string   `json:"provider"`
		Score          float64  `json:"score"`
		VoteWeight     float64  `json:"vote_weight"`
		VoteWeightPct  string   `json:"vote_weight_pct"`
		CodeVisible    bool     `json:"code_visible"`
		RecommendedFor []string `json:"recommended_for"`
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
			ModelID:        m.ModelID,
			DisplayName:    m.ModelName + " " + m.ScoreSuffix,
			Provider:       m.Provider,
			Score:          m.OverallScore,
			VoteWeight:     m.VoteWeight,
			VoteWeightPct:  formatPercent(weightPct),
			CodeVisible:    m.CodeVisible,
			RecommendedFor: getRecommendationsForModel(m.ModelID, m.Provider, m.OverallScore, m.CodeVisible),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ensemble":          ensemble,
		"total_models":      len(ensemble),
		"total_vote_weight": totalWeight,
		"description":       "These models participate in AI debate. Votes are weighted by verification score.",
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

	if !h.checkDiscoveryService(c) {
		return
	}

	// CONST-035 §c: nil-check (see GetSelectedModels). Without
	// discoveryService there's no per-model debate metadata available.
	if h.discoveryService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "discoveryService not wired; per-model debate metadata is verifier-driven and unavailable in registry-fallback mode.",
		})
		return
	}

	modelID := c.Param("model_id")

	model, found := h.discoveryService.GetModelForDebate(modelID)
	if !found {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "Model not in ensemble: " + modelID})
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

// getRecommendationsForModel generates dynamic recommendations based on model characteristics
func getRecommendationsForModel(modelID, provider string, score float64, codeVisible bool) []string {
	recommendations := make([]string, 0, 6)

	// Provider-specific recommendations
	providerLower := strings.ToLower(provider)
	switch providerLower {
	case "claude", "anthropic":
		recommendations = append(recommendations, "complex reasoning", "nuanced analysis")
		if codeVisible {
			recommendations = append(recommendations, "code review with explanations")
		}
	case "deepseek":
		recommendations = append(recommendations, "code generation", "technical analysis")
		if codeVisible {
			recommendations = append(recommendations, "algorithm design")
		}
	case "gemini", "google":
		recommendations = append(recommendations, "multimodal tasks", "research synthesis")
	case "qwen", "alibaba":
		recommendations = append(recommendations, "multilingual tasks", "translation")
	case "openrouter":
		recommendations = append(recommendations, "versatile tasks", "model routing")
	case "mistral":
		recommendations = append(recommendations, "European language tasks", "efficient inference")
	case "ollama":
		recommendations = append(recommendations, "local inference", "privacy-sensitive tasks")
	case "cerebras":
		recommendations = append(recommendations, "high-speed inference", "batch processing")
	case "zen", "opencode":
		recommendations = append(recommendations, "cost-effective tasks", "high-volume requests")
	default:
		recommendations = append(recommendations, "general tasks")
	}

	// Score-based additions
	if score >= 9.0 {
		recommendations = append(recommendations, "complex multi-step reasoning")
		if codeVisible {
			recommendations = append(recommendations, "architectural decisions")
		}
	} else if score >= 8.0 {
		recommendations = append(recommendations, "detailed analysis")
		if codeVisible {
			recommendations = append(recommendations, "code optimization")
		}
	} else if score >= 7.0 {
		recommendations = append(recommendations, "summarization", "Q&A")
	} else {
		recommendations = append(recommendations, "fallback scenarios", "simple queries")
	}

	// Model-specific additions based on model ID patterns
	modelIDLower := strings.ToLower(modelID)
	if strings.Contains(modelIDLower, "opus") {
		recommendations = append(recommendations, "long-form content")
	}
	if strings.Contains(modelIDLower, "sonnet") {
		recommendations = append(recommendations, "balanced performance")
	}
	if strings.Contains(modelIDLower, "haiku") {
		recommendations = append(recommendations, "quick responses")
	}
	if strings.Contains(modelIDLower, "coder") || strings.Contains(modelIDLower, "code") {
		recommendations = append(recommendations, "code-specific tasks")
	}
	if strings.Contains(modelIDLower, "flash") || strings.Contains(modelIDLower, "turbo") {
		recommendations = append(recommendations, "low-latency tasks")
	}

	// Ensure uniqueness and limit to 6 recommendations
	seen := make(map[string]bool)
	unique := make([]string, 0, 6)
	for _, r := range recommendations {
		if !seen[r] && len(unique) < 6 {
			seen[r] = true
			unique = append(unique, r)
		}
	}

	return unique
}
