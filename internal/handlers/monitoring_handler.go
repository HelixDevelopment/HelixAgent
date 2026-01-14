package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"dev.helix.agent/internal/services"
)

// MonitoringHandler handles monitoring API endpoints
type MonitoringHandler struct {
	circuitBreakerMonitor *services.CircuitBreakerMonitor
	oauthTokenMonitor     *services.OAuthTokenMonitor
	providerHealthMonitor *services.ProviderHealthMonitor
	fallbackChainValidator *services.FallbackChainValidator
}

// NewMonitoringHandler creates a new monitoring handler
func NewMonitoringHandler(
	cbm *services.CircuitBreakerMonitor,
	otm *services.OAuthTokenMonitor,
	phm *services.ProviderHealthMonitor,
	fcv *services.FallbackChainValidator,
) *MonitoringHandler {
	return &MonitoringHandler{
		circuitBreakerMonitor: cbm,
		oauthTokenMonitor:     otm,
		providerHealthMonitor: phm,
		fallbackChainValidator: fcv,
	}
}

// RegisterRoutes registers the monitoring routes
func (h *MonitoringHandler) RegisterRoutes(router *gin.RouterGroup) {
	monitoring := router.Group("/monitoring")
	{
		// Overall status
		monitoring.GET("/status", h.GetOverallStatus)

		// Circuit breaker endpoints
		monitoring.GET("/circuit-breakers", h.GetCircuitBreakerStatus)
		monitoring.POST("/circuit-breakers/:provider/reset", h.ResetCircuitBreaker)
		monitoring.POST("/circuit-breakers/reset-all", h.ResetAllCircuitBreakers)

		// OAuth token endpoints
		monitoring.GET("/oauth-tokens", h.GetOAuthTokenStatus)
		monitoring.POST("/oauth-tokens/:provider/refresh", h.RefreshOAuthToken)

		// Provider health endpoints
		monitoring.GET("/provider-health", h.GetProviderHealthStatus)
		monitoring.POST("/provider-health/check", h.ForceHealthCheck)
		monitoring.POST("/provider-health/:provider/check", h.ForceProviderHealthCheck)

		// Fallback chain endpoints
		monitoring.GET("/fallback-chain", h.GetFallbackChainStatus)
		monitoring.POST("/fallback-chain/validate", h.ValidateFallbackChain)
	}
}

// OverallMonitoringStatus represents the overall system monitoring status
type OverallMonitoringStatus struct {
	Healthy           bool                               `json:"healthy"`
	CircuitBreakers   *services.CircuitBreakerStatus     `json:"circuit_breakers,omitempty"`
	OAuthTokens       *services.OAuthTokenStatus         `json:"oauth_tokens,omitempty"`
	ProviderHealth    *services.ProviderHealthOverallStatus `json:"provider_health,omitempty"`
	FallbackChain     *services.FallbackChainStatus      `json:"fallback_chain,omitempty"`
}

// GetOverallStatus returns the overall monitoring status
// @Summary Get overall monitoring status
// @Description Returns combined status of all monitoring components
// @Tags monitoring
// @Produce json
// @Success 200 {object} OverallMonitoringStatus
// @Router /v1/monitoring/status [get]
func (h *MonitoringHandler) GetOverallStatus(c *gin.Context) {
	status := OverallMonitoringStatus{
		Healthy: true,
	}

	// Get circuit breaker status
	if h.circuitBreakerMonitor != nil {
		cbStatus := h.circuitBreakerMonitor.GetStatus()
		status.CircuitBreakers = &cbStatus
		if !cbStatus.Healthy {
			status.Healthy = false
		}
	}

	// Get OAuth token status
	if h.oauthTokenMonitor != nil {
		otStatus := h.oauthTokenMonitor.GetStatus()
		status.OAuthTokens = &otStatus
		if !otStatus.Healthy {
			status.Healthy = false
		}
	}

	// Get provider health status
	if h.providerHealthMonitor != nil {
		phStatus := h.providerHealthMonitor.GetStatus()
		status.ProviderHealth = &phStatus
		if !phStatus.Healthy {
			status.Healthy = false
		}
	}

	// Get fallback chain status
	if h.fallbackChainValidator != nil {
		fcStatus := h.fallbackChainValidator.GetStatus()
		status.FallbackChain = &fcStatus
		if fcStatus.Validated && !fcStatus.Valid {
			status.Healthy = false
		}
	}

	c.JSON(http.StatusOK, status)
}

// GetCircuitBreakerStatus returns circuit breaker status
// @Summary Get circuit breaker status
// @Description Returns status of all circuit breakers
// @Tags monitoring
// @Produce json
// @Success 200 {object} services.CircuitBreakerStatus
// @Router /v1/monitoring/circuit-breakers [get]
func (h *MonitoringHandler) GetCircuitBreakerStatus(c *gin.Context) {
	if h.circuitBreakerMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Circuit breaker monitor not available",
		})
		return
	}

	status := h.circuitBreakerMonitor.GetStatus()
	c.JSON(http.StatusOK, status)
}

// ResetCircuitBreaker resets a specific circuit breaker
// @Summary Reset circuit breaker
// @Description Resets a specific provider's circuit breaker
// @Tags monitoring
// @Param provider path string true "Provider ID"
// @Produce json
// @Success 200 {object} map[string]string
// @Router /v1/monitoring/circuit-breakers/{provider}/reset [post]
func (h *MonitoringHandler) ResetCircuitBreaker(c *gin.Context) {
	if h.circuitBreakerMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Circuit breaker monitor not available",
		})
		return
	}

	provider := c.Param("provider")
	err := h.circuitBreakerMonitor.ResetCircuitBreaker(provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Circuit breaker reset successfully",
		"provider": provider,
	})
}

// ResetAllCircuitBreakers resets all circuit breakers
// @Summary Reset all circuit breakers
// @Description Resets all circuit breakers to closed state
// @Tags monitoring
// @Produce json
// @Success 200 {object} map[string]string
// @Router /v1/monitoring/circuit-breakers/reset-all [post]
func (h *MonitoringHandler) ResetAllCircuitBreakers(c *gin.Context) {
	if h.circuitBreakerMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Circuit breaker monitor not available",
		})
		return
	}

	h.circuitBreakerMonitor.ResetAllCircuitBreakers()
	c.JSON(http.StatusOK, gin.H{
		"message": "All circuit breakers reset successfully",
	})
}

// GetOAuthTokenStatus returns OAuth token status
// @Summary Get OAuth token status
// @Description Returns status of all OAuth tokens
// @Tags monitoring
// @Produce json
// @Success 200 {object} services.OAuthTokenStatus
// @Router /v1/monitoring/oauth-tokens [get]
func (h *MonitoringHandler) GetOAuthTokenStatus(c *gin.Context) {
	if h.oauthTokenMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "OAuth token monitor not available",
		})
		return
	}

	status := h.oauthTokenMonitor.GetStatus()
	c.JSON(http.StatusOK, status)
}

// RefreshOAuthToken attempts to refresh an OAuth token
// @Summary Refresh OAuth token
// @Description Attempts to refresh a specific provider's OAuth token
// @Tags monitoring
// @Param provider path string true "Provider (claude or qwen)"
// @Produce json
// @Success 200 {object} map[string]string
// @Router /v1/monitoring/oauth-tokens/{provider}/refresh [post]
func (h *MonitoringHandler) RefreshOAuthToken(c *gin.Context) {
	if h.oauthTokenMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "OAuth token monitor not available",
		})
		return
	}

	provider := c.Param("provider")
	err := h.oauthTokenMonitor.RefreshToken(provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Token refresh initiated",
		"provider": provider,
	})
}

// GetProviderHealthStatus returns provider health status
// @Summary Get provider health status
// @Description Returns health status of all providers
// @Tags monitoring
// @Produce json
// @Success 200 {object} services.ProviderHealthOverallStatus
// @Router /v1/monitoring/provider-health [get]
func (h *MonitoringHandler) GetProviderHealthStatus(c *gin.Context) {
	if h.providerHealthMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Provider health monitor not available",
		})
		return
	}

	status := h.providerHealthMonitor.GetStatus()
	c.JSON(http.StatusOK, status)
}

// ForceHealthCheck forces an immediate health check of all providers
// @Summary Force health check
// @Description Forces immediate health check of all providers
// @Tags monitoring
// @Produce json
// @Success 200 {object} services.ProviderHealthOverallStatus
// @Router /v1/monitoring/provider-health/check [post]
func (h *MonitoringHandler) ForceHealthCheck(c *gin.Context) {
	if h.providerHealthMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Provider health monitor not available",
		})
		return
	}

	h.providerHealthMonitor.ForceCheck(c.Request.Context())
	status := h.providerHealthMonitor.GetStatus()
	c.JSON(http.StatusOK, status)
}

// ForceProviderHealthCheck forces a health check for a specific provider
// @Summary Force provider health check
// @Description Forces immediate health check of a specific provider
// @Tags monitoring
// @Param provider path string true "Provider ID"
// @Produce json
// @Success 200 {object} services.ProviderHealthStatus
// @Router /v1/monitoring/provider-health/{provider}/check [post]
func (h *MonitoringHandler) ForceProviderHealthCheck(c *gin.Context) {
	if h.providerHealthMonitor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Provider health monitor not available",
		})
		return
	}

	provider := c.Param("provider")
	h.providerHealthMonitor.ForceCheckProvider(c.Request.Context(), provider)

	status, exists := h.providerHealthMonitor.GetProviderStatus(provider)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Provider not found",
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetFallbackChainStatus returns fallback chain validation status
// @Summary Get fallback chain status
// @Description Returns status of fallback chain validation
// @Tags monitoring
// @Produce json
// @Success 200 {object} services.FallbackChainValidationResult
// @Router /v1/monitoring/fallback-chain [get]
func (h *MonitoringHandler) GetFallbackChainStatus(c *gin.Context) {
	if h.fallbackChainValidator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Fallback chain validator not available",
		})
		return
	}

	result := h.fallbackChainValidator.GetLastValidation()
	if result == nil {
		// Perform validation if not done yet
		result = h.fallbackChainValidator.Validate()
	}

	c.JSON(http.StatusOK, result)
}

// ValidateFallbackChain forces a new fallback chain validation
// @Summary Validate fallback chain
// @Description Forces a new fallback chain validation
// @Tags monitoring
// @Produce json
// @Success 200 {object} services.FallbackChainValidationResult
// @Router /v1/monitoring/fallback-chain/validate [post]
func (h *MonitoringHandler) ValidateFallbackChain(c *gin.Context) {
	if h.fallbackChainValidator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Fallback chain validator not available",
		})
		return
	}

	result := h.fallbackChainValidator.Validate()
	c.JSON(http.StatusOK, result)
}
