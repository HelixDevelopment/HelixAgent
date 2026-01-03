package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/superagent/superagent/internal/verifier"
)

// HealthHandler handles health monitoring HTTP requests
type HealthHandler struct {
	healthService *verifier.HealthService
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(hs *verifier.HealthService) *HealthHandler {
	return &HealthHandler{
		healthService: hs,
	}
}

// ProviderHealthResponse represents provider health response
type ProviderHealthResponse struct {
	ProviderID    string  `json:"provider_id"`
	ProviderName  string  `json:"provider_name"`
	Healthy       bool    `json:"healthy"`
	CircuitState  string  `json:"circuit_state"`
	FailureCount  int     `json:"failure_count"`
	SuccessCount  int     `json:"success_count"`
	AvgResponseMs int64   `json:"avg_response_ms"`
	UptimePercent float64 `json:"uptime_percent"`
	LastSuccessAt string  `json:"last_success_at,omitempty"`
	LastFailureAt string  `json:"last_failure_at,omitempty"`
	LastCheckedAt string  `json:"last_checked_at"`
}

// GetProviderHealth godoc
// @Summary Get health status for a provider
// @Description Get detailed health status for a specific provider
// @Tags health
// @Produce json
// @Param provider_id path string true "Provider ID"
// @Success 200 {object} ProviderHealthResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/verifier/health/providers/{provider_id} [get]
func (h *HealthHandler) GetProviderHealth(c *gin.Context) {
	providerID := c.Param("provider_id")

	health, err := h.healthService.GetProviderHealth(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: err.Error()})
		return
	}

	resp := ProviderHealthResponse{
		ProviderID:    health.ProviderID,
		ProviderName:  health.ProviderName,
		Healthy:       health.Healthy,
		CircuitState:  health.CircuitState,
		FailureCount:  health.FailureCount,
		SuccessCount:  health.SuccessCount,
		AvgResponseMs: health.AvgResponseMs,
		UptimePercent: health.UptimePercent,
		LastCheckedAt: health.LastCheckedAt.Format("2006-01-02T15:04:05Z"),
	}

	if !health.LastSuccessAt.IsZero() {
		resp.LastSuccessAt = health.LastSuccessAt.Format("2006-01-02T15:04:05Z")
	}
	if !health.LastFailureAt.IsZero() {
		resp.LastFailureAt = health.LastFailureAt.Format("2006-01-02T15:04:05Z")
	}

	c.JSON(http.StatusOK, resp)
}

// GetAllProvidersHealthResponse represents all providers health response
type GetAllProvidersHealthResponse struct {
	Providers      []ProviderHealthResponse `json:"providers"`
	Total          int                      `json:"total"`
	HealthyCount   int                      `json:"healthy_count"`
	UnhealthyCount int                      `json:"unhealthy_count"`
	Summary        HealthSummary            `json:"summary"`
}

// HealthSummary represents health summary
type HealthSummary struct {
	OverallHealthy        bool    `json:"overall_healthy"`
	AverageResponseMs     int64   `json:"average_response_ms"`
	AverageUptimePercent  float64 `json:"average_uptime_percent"`
	ProvidersWithOpenCircuit int  `json:"providers_with_open_circuit"`
}

// GetAllProvidersHealth godoc
// @Summary Get health status for all providers
// @Description Get health status for all registered providers
// @Tags health
// @Produce json
// @Success 200 {object} GetAllProvidersHealthResponse
// @Router /api/v1/verifier/health/providers [get]
func (h *HealthHandler) GetAllProvidersHealth(c *gin.Context) {
	providers := h.healthService.GetAllProviderHealth()

	resp := GetAllProvidersHealthResponse{
		Providers: make([]ProviderHealthResponse, len(providers)),
		Total:     len(providers),
	}

	var totalResponseMs int64
	var totalUptime float64
	var openCircuits int

	for i, p := range providers {
		resp.Providers[i] = ProviderHealthResponse{
			ProviderID:    p.ProviderID,
			ProviderName:  p.ProviderName,
			Healthy:       p.Healthy,
			CircuitState:  p.CircuitState,
			FailureCount:  p.FailureCount,
			SuccessCount:  p.SuccessCount,
			AvgResponseMs: p.AvgResponseMs,
			UptimePercent: p.UptimePercent,
			LastCheckedAt: p.LastCheckedAt.Format("2006-01-02T15:04:05Z"),
		}

		if !p.LastSuccessAt.IsZero() {
			resp.Providers[i].LastSuccessAt = p.LastSuccessAt.Format("2006-01-02T15:04:05Z")
		}
		if !p.LastFailureAt.IsZero() {
			resp.Providers[i].LastFailureAt = p.LastFailureAt.Format("2006-01-02T15:04:05Z")
		}

		if p.Healthy {
			resp.HealthyCount++
		} else {
			resp.UnhealthyCount++
		}

		if p.CircuitState == "open" {
			openCircuits++
		}

		totalResponseMs += p.AvgResponseMs
		totalUptime += p.UptimePercent
	}

	// Calculate summary
	if len(providers) > 0 {
		resp.Summary.AverageResponseMs = totalResponseMs / int64(len(providers))
		resp.Summary.AverageUptimePercent = totalUptime / float64(len(providers))
	}
	resp.Summary.OverallHealthy = resp.UnhealthyCount == 0
	resp.Summary.ProvidersWithOpenCircuit = openCircuits

	c.JSON(http.StatusOK, resp)
}

// GetHealthyProvidersResponse represents healthy providers response
type GetHealthyProvidersResponse struct {
	Providers []string `json:"providers"`
	Count     int      `json:"count"`
}

// GetHealthyProviders godoc
// @Summary Get list of healthy providers
// @Description Get a list of all currently healthy provider IDs
// @Tags health
// @Produce json
// @Success 200 {object} GetHealthyProvidersResponse
// @Router /api/v1/verifier/health/healthy [get]
func (h *HealthHandler) GetHealthyProviders(c *gin.Context) {
	providers := h.healthService.GetHealthyProviders()

	c.JSON(http.StatusOK, GetHealthyProvidersResponse{
		Providers: providers,
		Count:     len(providers),
	})
}

// GetFastestProviderRequest represents a fastest provider request
type GetFastestProviderRequest struct {
	Providers []string `json:"providers" binding:"required"`
}

// GetFastestProviderResponse represents fastest provider response
type GetFastestProviderResponse struct {
	ProviderID    string `json:"provider_id"`
	AvgResponseMs int64  `json:"avg_response_ms"`
}

// GetFastestProvider godoc
// @Summary Get fastest provider from list
// @Description Get the fastest available provider from a list of provider IDs
// @Tags health
// @Accept json
// @Produce json
// @Param request body GetFastestProviderRequest true "List of providers to check"
// @Success 200 {object} GetFastestProviderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/verifier/health/fastest [post]
func (h *HealthHandler) GetFastestProvider(c *gin.Context) {
	var req GetFastestProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	providerID, err := h.healthService.GetFastestProvider(req.Providers)
	if err != nil {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: err.Error()})
		return
	}

	latency, _ := h.healthService.GetProviderLatency(providerID)

	c.JSON(http.StatusOK, GetFastestProviderResponse{
		ProviderID:    providerID,
		AvgResponseMs: latency,
	})
}

// CircuitBreakerResponse represents circuit breaker status response
type CircuitBreakerResponse struct {
	ProviderID   string `json:"provider_id"`
	State        string `json:"state"`
	IsAvailable  bool   `json:"is_available"`
	FailureCount int    `json:"failure_count"`
	SuccessCount int    `json:"success_count"`
}

// GetCircuitBreakerStatus godoc
// @Summary Get circuit breaker status
// @Description Get circuit breaker status for a specific provider
// @Tags health
// @Produce json
// @Param provider_id path string true "Provider ID"
// @Success 200 {object} CircuitBreakerResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/verifier/health/circuit/{provider_id} [get]
func (h *HealthHandler) GetCircuitBreakerStatus(c *gin.Context) {
	providerID := c.Param("provider_id")

	cb := h.healthService.GetCircuitBreaker(providerID)
	if cb == nil {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: "provider not found"})
		return
	}

	c.JSON(http.StatusOK, CircuitBreakerResponse{
		ProviderID:  providerID,
		State:       cb.State().String(),
		IsAvailable: cb.IsAvailable(),
	})
}

// RecordSuccessRequest represents a success recording request
type RecordSuccessRequest struct {
	ProviderID string `json:"provider_id" binding:"required"`
}

// RecordSuccess godoc
// @Summary Record provider success
// @Description Manually record a successful operation for a provider
// @Tags health
// @Accept json
// @Produce json
// @Param request body RecordSuccessRequest true "Provider ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/verifier/health/record/success [post]
func (h *HealthHandler) RecordSuccess(c *gin.Context) {
	var req RecordSuccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	h.healthService.RecordSuccess(req.ProviderID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Success recorded",
		"provider_id": req.ProviderID,
	})
}

// RecordFailureRequest represents a failure recording request
type RecordFailureRequest struct {
	ProviderID string `json:"provider_id" binding:"required"`
}

// RecordFailure godoc
// @Summary Record provider failure
// @Description Manually record a failed operation for a provider
// @Tags health
// @Accept json
// @Produce json
// @Param request body RecordFailureRequest true "Provider ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/verifier/health/record/failure [post]
func (h *HealthHandler) RecordFailure(c *gin.Context) {
	var req RecordFailureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	h.healthService.RecordFailure(req.ProviderID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Failure recorded",
		"provider_id": req.ProviderID,
	})
}

// HealthAddProviderRequest represents an add provider request for health monitoring
type HealthAddProviderRequest struct {
	ProviderID   string `json:"provider_id" binding:"required"`
	ProviderName string `json:"provider_name" binding:"required"`
}

// AddProvider godoc
// @Summary Add provider to health monitoring
// @Description Add a new provider to health monitoring
// @Tags health
// @Accept json
// @Produce json
// @Param request body HealthAddProviderRequest true "Provider details"
// @Success 200 {object} map[string]string
// @Failure 400 {object} VerifierErrorResponse
// @Router /api/v1/verifier/health/providers [post]
func (h *HealthHandler) AddProvider(c *gin.Context) {
	var req HealthAddProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	h.healthService.AddProvider(req.ProviderID, req.ProviderName)

	c.JSON(http.StatusOK, gin.H{
		"message":       "Provider added to health monitoring",
		"provider_id":   req.ProviderID,
		"provider_name": req.ProviderName,
	})
}

// RemoveProvider godoc
// @Summary Remove provider from health monitoring
// @Description Remove a provider from health monitoring
// @Tags health
// @Produce json
// @Param provider_id path string true "Provider ID"
// @Success 200 {object} map[string]string
// @Router /api/v1/verifier/health/providers/{provider_id} [delete]
func (h *HealthHandler) RemoveProvider(c *gin.Context) {
	providerID := c.Param("provider_id")

	h.healthService.RemoveProvider(providerID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Provider removed from health monitoring",
		"provider_id": providerID,
	})
}

// IsProviderAvailable godoc
// @Summary Check if provider is available
// @Description Check if a specific provider is available
// @Tags health
// @Produce json
// @Param provider_id path string true "Provider ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/verifier/health/available/{provider_id} [get]
func (h *HealthHandler) IsProviderAvailable(c *gin.Context) {
	providerID := c.Param("provider_id")

	available := h.healthService.IsProviderAvailable(providerID)

	c.JSON(http.StatusOK, gin.H{
		"provider_id": providerID,
		"available":   available,
	})
}

// LatencyStatsResponse represents latency statistics response
type LatencyStatsResponse struct {
	ProviderID    string  `json:"provider_id"`
	AvgLatencyMs  int64   `json:"avg_latency_ms"`
	MinLatencyMs  int64   `json:"min_latency_ms"`
	MaxLatencyMs  int64   `json:"max_latency_ms"`
	P50LatencyMs  int64   `json:"p50_latency_ms"`
	P95LatencyMs  int64   `json:"p95_latency_ms"`
	P99LatencyMs  int64   `json:"p99_latency_ms"`
}

// GetProviderLatency godoc
// @Summary Get provider latency
// @Description Get latency statistics for a specific provider
// @Tags health
// @Produce json
// @Param provider_id path string true "Provider ID"
// @Success 200 {object} LatencyStatsResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/verifier/health/latency/{provider_id} [get]
func (h *HealthHandler) GetProviderLatency(c *gin.Context) {
	providerID := c.Param("provider_id")

	latency, err := h.healthService.GetProviderLatency(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: err.Error()})
		return
	}

	// Basic latency stats (would be extended with actual percentile tracking)
	c.JSON(http.StatusOK, LatencyStatsResponse{
		ProviderID:   providerID,
		AvgLatencyMs: latency,
	})
}

// HealthServiceStatusResponse represents health service status
type HealthServiceStatusResponse struct {
	Running           bool   `json:"running"`
	CheckInterval     string `json:"check_interval"`
	TotalProviders    int    `json:"total_providers"`
	MonitoringStarted string `json:"monitoring_started,omitempty"`
}

// GetHealthServiceStatus godoc
// @Summary Get health service status
// @Description Get the status of the health monitoring service
// @Tags health
// @Produce json
// @Success 200 {object} HealthServiceStatusResponse
// @Router /api/v1/verifier/health/status [get]
func (h *HealthHandler) GetHealthServiceStatus(c *gin.Context) {
	providers := h.healthService.GetAllProviderHealth()

	c.JSON(http.StatusOK, HealthServiceStatusResponse{
		Running:        true,
		CheckInterval:  "30s",
		TotalProviders: len(providers),
	})
}

// RegisterHealthRoutes registers health monitoring routes
func RegisterHealthRoutes(r *gin.RouterGroup, h *HealthHandler) {
	health := r.Group("/verifier/health")
	{
		health.GET("/status", h.GetHealthServiceStatus)
		health.GET("/providers", h.GetAllProvidersHealth)
		health.GET("/providers/:provider_id", h.GetProviderHealth)
		health.POST("/providers", h.AddProvider)
		health.DELETE("/providers/:provider_id", h.RemoveProvider)
		health.GET("/healthy", h.GetHealthyProviders)
		health.POST("/fastest", h.GetFastestProvider)
		health.GET("/circuit/:provider_id", h.GetCircuitBreakerStatus)
		health.GET("/available/:provider_id", h.IsProviderAvailable)
		health.GET("/latency/:provider_id", h.GetProviderLatency)
		health.POST("/record/success", h.RecordSuccess)
		health.POST("/record/failure", h.RecordFailure)
	}
}
