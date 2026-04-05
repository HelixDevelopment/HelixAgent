package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/services"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func createTestConcurrencyAlertManager() *services.ConcurrencyAlertManager {
	config := services.ConcurrencyAlertManagerConfig{
		Enabled:         false, // Disable alert processing for tests
		DefaultCooldown: time.Minute,
		CleanupInterval: time.Hour,
		MaxAlertAge:     24 * time.Hour,
		EnableLogging:   false,
		EnableWebhook:   false,
		WebhookTimeout:  5 * time.Second,
	}
	return services.NewConcurrencyAlertManager(config, logrus.New())
}

// TestNewMonitoringHandler tests handler creation
func TestNewMonitoringHandler(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	assert.NotNil(t, handler)
	assert.Nil(t, handler.circuitBreakerMonitor)
	assert.Nil(t, handler.oauthTokenMonitor)
	assert.Nil(t, handler.providerHealthMonitor)
	assert.Nil(t, handler.fallbackChainValidator)
	assert.Nil(t, handler.concurrencyAlertManager)
}

// TestMonitoringHandler_GetOverallStatus_AllNil tests when all monitors are nil
func TestMonitoringHandler_GetOverallStatus_AllNil(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/monitoring/status", nil)

	handler.GetOverallStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response OverallMonitoringStatus
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Healthy)
	assert.Nil(t, response.CircuitBreakers)
	assert.Nil(t, response.OAuthTokens)
	assert.Nil(t, response.ProviderHealth)
	assert.Nil(t, response.FallbackChain)
	assert.Nil(t, response.ConcurrencyAlerts)
}

// TestMonitoringHandler_GetCircuitBreakerStatus_NilMonitor tests when circuit breaker monitor is nil
func TestMonitoringHandler_GetCircuitBreakerStatus_NilMonitor(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/monitoring/circuit-breakers", nil)

	handler.GetCircuitBreakerStatus(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Circuit breaker monitor not available")
}

// TestMonitoringHandler_ResetCircuitBreaker_NilMonitor tests reset with nil monitor
func TestMonitoringHandler_ResetCircuitBreaker_NilMonitor(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "provider", Value: "test-provider"}}
	c.Request = httptest.NewRequest("POST", "/v1/monitoring/circuit-breakers/test-provider/reset", nil)

	handler.ResetCircuitBreaker(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Circuit breaker monitor not available")
}

// TestMonitoringHandler_ResetAllCircuitBreakers_NilMonitor tests reset all with nil monitor
func TestMonitoringHandler_ResetAllCircuitBreakers_NilMonitor(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/monitoring/circuit-breakers/reset-all", nil)

	handler.ResetAllCircuitBreakers(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Circuit breaker monitor not available")
}

// TestMonitoringHandler_GetOAuthTokenStatus_NilMonitor tests OAuth with nil monitor
func TestMonitoringHandler_GetOAuthTokenStatus_NilMonitor(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/monitoring/oauth-tokens", nil)

	handler.GetOAuthTokenStatus(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "OAuth token monitor not available")
}

// TestMonitoringHandler_RefreshOAuthToken_NilMonitor tests OAuth refresh with nil monitor
func TestMonitoringHandler_RefreshOAuthToken_NilMonitor(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "provider", Value: "claude"}}
	c.Request = httptest.NewRequest("POST", "/v1/monitoring/oauth-tokens/claude/refresh", nil)

	handler.RefreshOAuthToken(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "OAuth token monitor not available")
}

// TestMonitoringHandler_GetProviderHealthStatus_NilMonitor tests provider health with nil monitor
func TestMonitoringHandler_GetProviderHealthStatus_NilMonitor(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/monitoring/provider-health", nil)

	handler.GetProviderHealthStatus(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Provider health monitor not available")
}

// TestMonitoringHandler_ForceHealthCheck_NilMonitor tests force health check with nil monitor
func TestMonitoringHandler_ForceHealthCheck_NilMonitor(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/monitoring/provider-health/check", nil)

	handler.ForceHealthCheck(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Provider health monitor not available")
}

// TestMonitoringHandler_ForceProviderHealthCheck_NilMonitor tests provider health check with nil monitor
func TestMonitoringHandler_ForceProviderHealthCheck_NilMonitor(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "provider", Value: "deepseek"}}
	c.Request = httptest.NewRequest("POST", "/v1/monitoring/provider-health/deepseek/check", nil)

	handler.ForceProviderHealthCheck(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Provider health monitor not available")
}

// TestMonitoringHandler_GetFallbackChainStatus_NilValidator tests fallback status with nil validator
func TestMonitoringHandler_GetFallbackChainStatus_NilValidator(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/monitoring/fallback-chain", nil)

	handler.GetFallbackChainStatus(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Fallback chain validator not available")
}

// TestMonitoringHandler_ValidateFallbackChain_NilValidator tests validate with nil validator
func TestMonitoringHandler_ValidateFallbackChain_NilValidator(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/monitoring/fallback-chain/validate", nil)

	handler.ValidateFallbackChain(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Fallback chain validator not available")
}

// TestMonitoringHandler_RegisterRoutes tests route registration
func TestMonitoringHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	router := gin.New()
	group := router.Group("/v1")
	handler.RegisterRoutes(group)

	// Test that routes are registered by making requests
	routes := router.Routes()
	expectedPaths := []string{
		"/v1/monitoring/status",
		"/v1/monitoring/circuit-breakers",
		"/v1/monitoring/circuit-breakers/:provider/reset",
		"/v1/monitoring/circuit-breakers/reset-all",
		"/v1/monitoring/oauth-tokens",
		"/v1/monitoring/oauth-tokens/:provider/refresh",
		"/v1/monitoring/provider-health",
		"/v1/monitoring/provider-health/check",
		"/v1/monitoring/provider-health/:provider/check",
		"/v1/monitoring/fallback-chain",
		"/v1/monitoring/fallback-chain/validate",
		// Concurrency monitoring routes
		"/v1/monitoring/concurrency",
		"/v1/monitoring/concurrency/alerts",
		"/v1/monitoring/concurrency/alerts/dead-letter",
		"/v1/monitoring/concurrency/alerts/dead-letter/:key/retry",
		"/v1/monitoring/concurrency/alerts/retry-queue",
		"/v1/monitoring/concurrency/alerts/retry-queue/:key/cancel",
		"/v1/monitoring/concurrency/:provider/reset-tracking",
		"/v1/monitoring/concurrency/reset-all-tracking",
	}

	registeredPaths := make(map[string]bool)
	for _, route := range routes {
		registeredPaths[route.Path] = true
	}

	for _, path := range expectedPaths {
		assert.True(t, registeredPaths[path], "Route %s should be registered", path)
	}
}

// TestOverallMonitoringStatus_Struct tests the struct fields
func TestOverallMonitoringStatus_Struct(t *testing.T) {
	t.Parallel()
	status := OverallMonitoringStatus{
		Healthy: true,
	}

	assert.True(t, status.Healthy)
	assert.Nil(t, status.CircuitBreakers)
	assert.Nil(t, status.OAuthTokens)
	assert.Nil(t, status.ProviderHealth)
	assert.Nil(t, status.FallbackChain)
}

// TestMonitoringHandler_MultipleProviderEndpoints tests different provider parameter values
func TestMonitoringHandler_MultipleProviderEndpoints(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)
	providers := []string{"claude", "qwen", "deepseek", "gemini", "ollama", "openrouter"}

	for _, provider := range providers {
		t.Run("Reset_"+provider, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "provider", Value: provider}}
			c.Request = httptest.NewRequest("POST", "/v1/monitoring/circuit-breakers/"+provider+"/reset", nil)

			handler.ResetCircuitBreaker(c)

			// All should fail with 503 since monitor is nil
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})

		t.Run("OAuthRefresh_"+provider, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "provider", Value: provider}}
			c.Request = httptest.NewRequest("POST", "/v1/monitoring/oauth-tokens/"+provider+"/refresh", nil)

			handler.RefreshOAuthToken(c)

			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})

		t.Run("HealthCheck_"+provider, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "provider", Value: provider}}
			c.Request = httptest.NewRequest("POST", "/v1/monitoring/provider-health/"+provider+"/check", nil)

			handler.ForceProviderHealthCheck(c)

			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}
}

// TestMonitoringHandler_EmptyProviderParam tests empty provider parameter
func TestMonitoringHandler_EmptyProviderParam(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	tests := []struct {
		name   string
		method func(c *gin.Context)
		path   string
	}{
		{
			name:   "ResetCircuitBreaker",
			method: handler.ResetCircuitBreaker,
			path:   "/v1/monitoring/circuit-breakers//reset",
		},
		{
			name:   "RefreshOAuthToken",
			method: handler.RefreshOAuthToken,
			path:   "/v1/monitoring/oauth-tokens//refresh",
		},
		{
			name:   "ForceProviderHealthCheck",
			method: handler.ForceProviderHealthCheck,
			path:   "/v1/monitoring/provider-health//check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "provider", Value: ""}}
			c.Request = httptest.NewRequest("POST", tt.path, nil)

			tt.method(c)

			// Should return 503 (monitor not available) even with empty provider
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}
}

// TestMonitoringHandler_ResponseFormats tests that all responses are valid JSON
func TestMonitoringHandler_ResponseFormats(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	endpoints := []struct {
		name   string
		method func(c *gin.Context)
		setup  func(c *gin.Context)
	}{
		{
			name:   "GetOverallStatus",
			method: handler.GetOverallStatus,
			setup:  func(c *gin.Context) {},
		},
		{
			name:   "GetCircuitBreakerStatus",
			method: handler.GetCircuitBreakerStatus,
			setup:  func(c *gin.Context) {},
		},
		{
			name:   "ResetCircuitBreaker",
			method: handler.ResetCircuitBreaker,
			setup: func(c *gin.Context) {
				c.Params = gin.Params{{Key: "provider", Value: "test"}}
			},
		},
		{
			name:   "ResetAllCircuitBreakers",
			method: handler.ResetAllCircuitBreakers,
			setup:  func(c *gin.Context) {},
		},
		{
			name:   "GetOAuthTokenStatus",
			method: handler.GetOAuthTokenStatus,
			setup:  func(c *gin.Context) {},
		},
		{
			name:   "RefreshOAuthToken",
			method: handler.RefreshOAuthToken,
			setup: func(c *gin.Context) {
				c.Params = gin.Params{{Key: "provider", Value: "test"}}
			},
		},
		{
			name:   "GetProviderHealthStatus",
			method: handler.GetProviderHealthStatus,
			setup:  func(c *gin.Context) {},
		},
		{
			name:   "ForceHealthCheck",
			method: handler.ForceHealthCheck,
			setup:  func(c *gin.Context) {},
		},
		{
			name:   "ForceProviderHealthCheck",
			method: handler.ForceProviderHealthCheck,
			setup: func(c *gin.Context) {
				c.Params = gin.Params{{Key: "provider", Value: "test"}}
			},
		},
		{
			name:   "GetFallbackChainStatus",
			method: handler.GetFallbackChainStatus,
			setup:  func(c *gin.Context) {},
		},
		{
			name:   "ValidateFallbackChain",
			method: handler.ValidateFallbackChain,
			setup:  func(c *gin.Context) {},
		},
	}

	for _, ep := range endpoints {
		t.Run(ep.name+"_ValidJSON", func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/test", nil)
			ep.setup(c)

			ep.method(c)

			// Response should be valid JSON
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err, "Response should be valid JSON")
		})
	}
}

func TestMonitoringHandler_GetDeadLetterAlerts_NilManager(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/monitoring/concurrency/alerts/dead-letter", nil)

	handler.GetDeadLetterAlerts(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Concurrency alert manager not available")
}

func TestMonitoringHandler_GetDeadLetterAlerts_WithManager(t *testing.T) {
	t.Parallel()
	manager := createTestConcurrencyAlertManager()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, manager)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/monitoring/concurrency/alerts/dead-letter", nil)

	handler.GetDeadLetterAlerts(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Empty(t, response) // Should be empty since no alerts in dead letter queue
}

func TestMonitoringHandler_RetryDeadLetterAlert_NilManager(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "test:123"}}
	c.Request = httptest.NewRequest("POST", "/v1/monitoring/concurrency/alerts/dead-letter/test:123/retry", nil)

	handler.RetryDeadLetterAlert(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Concurrency alert manager not available")
}

func TestMonitoringHandler_RetryDeadLetterAlert_WithManager(t *testing.T) {
	t.Parallel()
	manager := createTestConcurrencyAlertManager()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, manager)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "test:123"}}
	c.Request = httptest.NewRequest("POST", "/v1/monitoring/concurrency/alerts/dead-letter/test:123/retry", nil)

	handler.RetryDeadLetterAlert(c)

	assert.Equal(t, http.StatusNotFound, w.Code) // Alert not found in dead letter queue

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Alert not found in dead letter queue")
}

func TestMonitoringHandler_GetRetryQueueAlerts_NilManager(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/monitoring/concurrency/alerts/retry-queue", nil)

	handler.GetRetryQueueAlerts(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Concurrency alert manager not available")
}

func TestMonitoringHandler_GetRetryQueueAlerts_WithManager(t *testing.T) {
	t.Parallel()
	manager := createTestConcurrencyAlertManager()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, manager)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/monitoring/concurrency/alerts/retry-queue", nil)

	handler.GetRetryQueueAlerts(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Empty(t, response) // Should be empty since no alerts in retry queue
}

func TestMonitoringHandler_CancelRetryAttempt_NilManager(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "test:123"}}
	c.Request = httptest.NewRequest("POST", "/v1/monitoring/concurrency/alerts/retry-queue/test:123/cancel", nil)

	handler.CancelRetryAttempt(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Concurrency alert manager not available")
}

func TestMonitoringHandler_CancelRetryAttempt_WithManager(t *testing.T) {
	t.Parallel()
	manager := createTestConcurrencyAlertManager()
	handler := NewMonitoringHandler(nil, nil, nil, nil, nil, manager)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: "test:123"}}
	c.Request = httptest.NewRequest("POST", "/v1/monitoring/concurrency/alerts/retry-queue/test:123/cancel", nil)

	handler.CancelRetryAttempt(c)

	assert.Equal(t, http.StatusNotFound, w.Code) // Alert not found in retry queue

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Alert not found in retry queue")
}
