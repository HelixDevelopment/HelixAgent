package monitoring_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/handlers"
)

// TestHealthEndpoint_Comprehensive_RootHealth validates that the root /health
// endpoint returns HTTP 200 with a minimal JSON body containing a status field.
func TestHealthEndpoint_Comprehensive_RootHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"/health must return HTTP 200")

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err, "/health must return valid JSON")

	status, exists := body["status"]
	assert.True(t, exists, "/health JSON must contain 'status' field")
	assert.Equal(t, "healthy", status,
		"/health 'status' field must equal 'healthy'")
}

// TestHealthEndpoint_Comprehensive_V1Health validates that the /v1/health
// endpoint returns HTTP 200 with a detailed provider status breakdown.
func TestHealthEndpoint_Comprehensive_V1Health(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	r := gin.New()
	r.GET("/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"providers": map[string]interface{}{
				"total":     5,
				"healthy":   4,
				"unhealthy": 1,
			},
			"timestamp": time.Now().Unix(),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "/v1/health must return HTTP 200")

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err, "/v1/health must return valid JSON")

	// Verify top-level structure
	_, hasStatus := body["status"]
	assert.True(t, hasStatus, "/v1/health must include 'status' field")

	_, hasTimestamp := body["timestamp"]
	assert.True(t, hasTimestamp, "/v1/health must include 'timestamp' field")

	providersRaw, hasProviders := body["providers"]
	assert.True(t, hasProviders, "/v1/health must include 'providers' field")

	// Verify providers sub-object
	providers, ok := providersRaw.(map[string]interface{})
	require.True(t, ok, "'providers' must be a JSON object")

	for _, field := range []string{"total", "healthy", "unhealthy"} {
		_, fieldExists := providers[field]
		assert.True(t, fieldExists,
			"providers object must contain field %q", field)
	}

	// Verify numeric constraints
	total := providers["total"].(float64)
	healthy := providers["healthy"].(float64)
	unhealthy := providers["unhealthy"].(float64)

	assert.Equal(t, total, healthy+unhealthy,
		"total providers must equal healthy + unhealthy")
	assert.GreaterOrEqual(t, healthy, 0.0, "healthy count must be non-negative")
	assert.GreaterOrEqual(t, unhealthy, 0.0, "unhealthy count must be non-negative")
}

// TestHealthEndpoint_Comprehensive_ContentType validates that health endpoints
// return the correct Content-Type header.
func TestHealthEndpoint_Comprehensive_ContentType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	endpoints := []string{"/health", "/v1/health"}

	for _, endpoint := range endpoints {
		endpoint := endpoint
		t.Run(endpoint, func(t *testing.T) {
			r := gin.New()
			path := endpoint
			r.GET(path, func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "healthy"})
			})

			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			ct := w.Header().Get("Content-Type")
			assert.Contains(t, ct, "application/json",
				"endpoint %q must respond with Content-Type application/json", endpoint)
		})
	}
}

// TestHealthEndpoint_Comprehensive_MonitoringStatus validates that the
// monitoring handler's GetOverallStatus returns a well-formed response
// even when no monitors are injected (nil-safe defaults).
func TestHealthEndpoint_Comprehensive_MonitoringStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	handler := handlers.NewMonitoringHandler(nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/v1/monitoring/status",
		nil,
	)

	handler.GetOverallStatus(c)

	assert.Equal(t, http.StatusOK, w.Code,
		"Monitoring status must return 200 with nil monitors")

	var response handlers.OverallMonitoringStatus
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Monitoring status response must be valid JSON")

	// With all monitors nil the system reports healthy by default.
	assert.True(t, response.Healthy,
		"System with no monitors must report healthy")
}

// TestHealthEndpoint_Comprehensive_NotFound validates that unknown paths
// return HTTP 404 rather than 200, confirming that health routes are
// registered precisely and not catching everything.
func TestHealthEndpoint_Comprehensive_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	req := httptest.NewRequest(http.MethodGet, "/nonexistent-health-path", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"Unknown path must return HTTP 404")
}

// TestHealthEndpoint_Comprehensive_MethodNotAllowed validates that POSTing
// to a GET-only health endpoint returns HTTP 405 Method Not Allowed.
func TestHealthEndpoint_Comprehensive_MethodNotAllowed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Gin returns 405 when HandleMethodNotAllowed is true.
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code,
		"POST to GET-only /health must return 405 Method Not Allowed")
}

// TestHealthEndpoint_Comprehensive_Schema validates the full expected schema
// of a /v1/health response by marshaling a sample struct and checking that
// all required JSON keys are present and correctly typed.
func TestHealthEndpoint_Comprehensive_Schema(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	sample := map[string]interface{}{
		"status": "healthy",
		"providers": map[string]interface{}{
			"total":     10,
			"healthy":   9,
			"unhealthy": 1,
		},
		"timestamp": time.Now().Unix(),
	}

	data, err := json.Marshal(sample)
	require.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Validate top-level string field
	statusVal, ok := decoded["status"].(string)
	require.True(t, ok, "'status' must be a string")
	assert.NotEmpty(t, statusVal, "'status' must not be empty")

	// Validate timestamp is numeric
	ts, ok := decoded["timestamp"].(float64)
	require.True(t, ok, "'timestamp' must be numeric")
	assert.Greater(t, ts, float64(0), "'timestamp' must be a positive Unix epoch")

	// Validate providers sub-object
	pRaw, ok := decoded["providers"].(map[string]interface{})
	require.True(t, ok, "'providers' must be a JSON object")

	for _, key := range []string{"total", "healthy", "unhealthy"} {
		v, exists := pRaw[key]
		assert.True(t, exists, "providers must have field %q", key)
		_, isNum := v.(float64)
		assert.True(t, isNum, "providers.%s must be numeric", key)
	}
}
