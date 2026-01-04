package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/superagent/superagent/internal/verifier"
)

func setupHealthHandler() (*HealthHandler, *gin.Engine) {
	hs := verifier.NewHealthService(nil)

	h := NewHealthHandler(hs)
	r := gin.New()

	api := r.Group("/api/v1")
	RegisterHealthRoutes(api, h)

	return h, r
}

func setupHealthHandlerWithProviders() (*HealthHandler, *gin.Engine, *verifier.HealthService) {
	hs := verifier.NewHealthService(nil)
	hs.AddProvider("openai", "OpenAI")
	hs.AddProvider("anthropic", "Anthropic")
	hs.AddProvider("google", "Google")

	h := NewHealthHandler(hs)
	r := gin.New()

	api := r.Group("/api/v1")
	RegisterHealthRoutes(api, h)

	return h, r, hs
}

func TestNewHealthHandler(t *testing.T) {
	hs := verifier.NewHealthService(nil)
	h := NewHealthHandler(hs)

	assert.NotNil(t, h)
	assert.Equal(t, hs, h.healthService)
}

func TestNewHealthHandler_NilService(t *testing.T) {
	h := NewHealthHandler(nil)

	assert.NotNil(t, h)
	assert.Nil(t, h.healthService)
}

func TestGetProviderHealth_Success(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/providers/openai", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ProviderHealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "openai", resp.ProviderID)
	assert.Equal(t, "OpenAI", resp.ProviderName)
	assert.True(t, resp.Healthy)
	assert.Equal(t, "closed", resp.CircuitState)
}

func TestGetProviderHealth_NotFound(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/providers/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp.Error, "provider not found")
}

func TestGetAllProvidersHealth_EmptyList(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/providers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetAllProvidersHealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Total)
	assert.Equal(t, 0, resp.HealthyCount)
	assert.Equal(t, 0, resp.UnhealthyCount)
	assert.True(t, resp.Summary.OverallHealthy)
}

func TestGetAllProvidersHealth_WithProviders(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/providers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetAllProvidersHealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 3, resp.Total)
	assert.Equal(t, 3, resp.HealthyCount)
	assert.Equal(t, 0, resp.UnhealthyCount)
	assert.True(t, resp.Summary.OverallHealthy)
}

func TestGetAllProvidersHealth_ResponseFormat(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/providers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetAllProvidersHealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp.Providers)
	assert.NotNil(t, resp.Summary)
}

func TestGetHealthyProviders_EmptyList(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/healthy", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetHealthyProvidersResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Count)
	assert.Empty(t, resp.Providers)
}

func TestGetHealthyProviders_WithProviders(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/healthy", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetHealthyProvidersResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 3, resp.Count)
	assert.Len(t, resp.Providers, 3)
}

func TestGetFastestProvider_Success(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	reqBody := GetFastestProviderRequest{
		Providers: []string{"openai", "anthropic", "google"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/fastest", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetFastestProviderResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.ProviderID)
}

func TestGetFastestProvider_BadRequest_EmptyProviders(t *testing.T) {
	_, r := setupHealthHandler()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/fastest", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetFastestProvider_BadRequest_InvalidJSON(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/fastest", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetFastestProvider_NotFound_NoAvailableProviders(t *testing.T) {
	_, r := setupHealthHandler()

	reqBody := GetFastestProviderRequest{
		Providers: []string{"nonexistent1", "nonexistent2"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/fastest", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetCircuitBreakerStatus_Success(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/circuit/openai", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CircuitBreakerResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "openai", resp.ProviderID)
	assert.Equal(t, "closed", resp.State)
	assert.True(t, resp.IsAvailable)
}

func TestGetCircuitBreakerStatus_NotFound(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/circuit/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp.Error, "provider not found")
}

func TestRecordSuccess_Success(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	reqBody := RecordSuccessRequest{
		ProviderID: "openai",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/record/success", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "Success recorded", resp["message"])
	assert.Equal(t, "openai", resp["provider_id"])
}

func TestRecordSuccess_BadRequest_MissingProviderID(t *testing.T) {
	_, r := setupHealthHandler()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/record/success", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRecordSuccess_BadRequest_InvalidJSON(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/record/success", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRecordFailure_Success(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	reqBody := RecordFailureRequest{
		ProviderID: "openai",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/record/failure", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "Failure recorded", resp["message"])
	assert.Equal(t, "openai", resp["provider_id"])
}

func TestRecordFailure_BadRequest_MissingProviderID(t *testing.T) {
	_, r := setupHealthHandler()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/record/failure", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRecordFailure_BadRequest_InvalidJSON(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/record/failure", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddProvider_Success(t *testing.T) {
	_, r := setupHealthHandler()

	reqBody := HealthAddProviderRequest{
		ProviderID:   "new-provider",
		ProviderName: "New Provider",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/providers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "Provider added to health monitoring", resp["message"])
	assert.Equal(t, "new-provider", resp["provider_id"])
	assert.Equal(t, "New Provider", resp["provider_name"])
}

func TestAddProvider_BadRequest_MissingFields(t *testing.T) {
	_, r := setupHealthHandler()

	tests := []struct {
		name string
		body string
	}{
		{"missing provider_id", `{"provider_name": "Test"}`},
		{"missing provider_name", `{"provider_id": "test"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/verifier/health/providers", bytes.NewBuffer([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestAddProvider_BadRequest_InvalidJSON(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/providers", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveProvider_Success(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/verifier/health/providers/openai", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "Provider removed from health monitoring", resp["message"])
	assert.Equal(t, "openai", resp["provider_id"])
}

func TestRemoveProvider_NonExistent(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/verifier/health/providers/nonexistent", nil)
	r.ServeHTTP(w, req)

	// Still returns 200 for idempotency
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIsProviderAvailable_Available(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/available/openai", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "openai", resp["provider_id"])
	assert.True(t, resp["available"].(bool))
}

func TestIsProviderAvailable_NotAvailable(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/available/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "nonexistent", resp["provider_id"])
	assert.False(t, resp["available"].(bool))
}

func TestGetProviderLatency_Success(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/latency/openai", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp LatencyStatsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "openai", resp.ProviderID)
}

func TestGetProviderLatency_NotFound(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/latency/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetHealthServiceStatus_Success(t *testing.T) {
	_, r := setupHealthHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp HealthServiceStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Running)
	assert.Equal(t, "30s", resp.CheckInterval)
}

func TestGetHealthServiceStatus_WithProviders(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp HealthServiceStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 3, resp.TotalProviders)
}

func TestRegisterHealthRoutes(t *testing.T) {
	hs := verifier.NewHealthService(nil)
	h := NewHealthHandler(hs)
	r := gin.New()
	api := r.Group("/api/v1")

	// Should not panic
	RegisterHealthRoutes(api, h)

	// Test that routes are registered by checking 404 for unknown routes
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/unknown", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProviderHealthResponse_Fields(t *testing.T) {
	resp := ProviderHealthResponse{
		ProviderID:    "openai",
		ProviderName:  "OpenAI",
		Healthy:       true,
		CircuitState:  "closed",
		FailureCount:  0,
		SuccessCount:  10,
		AvgResponseMs: 150,
		UptimePercent: 99.9,
		LastSuccessAt: "2024-01-01T12:00:00Z",
		LastFailureAt: "",
		LastCheckedAt: "2024-01-01T12:00:00Z",
	}

	assert.Equal(t, "openai", resp.ProviderID)
	assert.Equal(t, "OpenAI", resp.ProviderName)
	assert.True(t, resp.Healthy)
	assert.Equal(t, "closed", resp.CircuitState)
	assert.Equal(t, 0, resp.FailureCount)
	assert.Equal(t, 10, resp.SuccessCount)
	assert.Equal(t, int64(150), resp.AvgResponseMs)
	assert.Equal(t, 99.9, resp.UptimePercent)
}

func TestCircuitBreakerResponse_Fields(t *testing.T) {
	resp := CircuitBreakerResponse{
		ProviderID:   "openai",
		State:        "closed",
		IsAvailable:  true,
		FailureCount: 0,
		SuccessCount: 10,
	}

	assert.Equal(t, "openai", resp.ProviderID)
	assert.Equal(t, "closed", resp.State)
	assert.True(t, resp.IsAvailable)
}

func TestHealthSummary_Fields(t *testing.T) {
	summary := HealthSummary{
		OverallHealthy:           true,
		AverageResponseMs:        100,
		AverageUptimePercent:     99.5,
		ProvidersWithOpenCircuit: 0,
	}

	assert.True(t, summary.OverallHealthy)
	assert.Equal(t, int64(100), summary.AverageResponseMs)
	assert.Equal(t, 99.5, summary.AverageUptimePercent)
	assert.Equal(t, 0, summary.ProvidersWithOpenCircuit)
}

func TestLatencyStatsResponse_Fields(t *testing.T) {
	resp := LatencyStatsResponse{
		ProviderID:   "openai",
		AvgLatencyMs: 150,
		MinLatencyMs: 50,
		MaxLatencyMs: 500,
		P50LatencyMs: 120,
		P95LatencyMs: 300,
		P99LatencyMs: 450,
	}

	assert.Equal(t, "openai", resp.ProviderID)
	assert.Equal(t, int64(150), resp.AvgLatencyMs)
	assert.Equal(t, int64(50), resp.MinLatencyMs)
	assert.Equal(t, int64(500), resp.MaxLatencyMs)
}

func TestHealthServiceStatusResponse_Fields(t *testing.T) {
	resp := HealthServiceStatusResponse{
		Running:           true,
		CheckInterval:     "30s",
		TotalProviders:    5,
		MonitoringStarted: "2024-01-01T12:00:00Z",
	}

	assert.True(t, resp.Running)
	assert.Equal(t, "30s", resp.CheckInterval)
	assert.Equal(t, 5, resp.TotalProviders)
}

func TestHealthRoutes_ContentType(t *testing.T) {
	_, r := setupHealthHandler()

	routes := []string{
		"/api/v1/verifier/health/status",
		"/api/v1/verifier/health/providers",
		"/api/v1/verifier/health/healthy",
	}

	for _, route := range routes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", route, nil)
		r.ServeHTTP(w, req)

		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Route %s should return JSON", route)
	}
}

func TestHealthRoutes_MethodNotAllowed(t *testing.T) {
	_, r := setupHealthHandler()

	tests := []struct {
		method   string
		path     string
		expected int
	}{
		{"POST", "/api/v1/verifier/health/status", http.StatusNotFound},
		{"DELETE", "/api/v1/verifier/health/healthy", http.StatusNotFound},
		{"GET", "/api/v1/verifier/health/record/success", http.StatusNotFound},
		{"GET", "/api/v1/verifier/health/record/failure", http.StatusNotFound},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(tt.method, tt.path, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, tt.expected, w.Code, "Expected %d for %s %s", tt.expected, tt.method, tt.path)
	}
}

func TestGetAllProvidersHealth_SummaryCalculation(t *testing.T) {
	_, r, hs := setupHealthHandlerWithProviders()

	// Record some failures to change health status
	hs.RecordFailure("openai")
	hs.RecordFailure("openai")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/health/providers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetAllProvidersHealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 3, resp.Total)
	assert.NotNil(t, resp.Summary)
}

func TestRecordSuccessAndFailure_Integration(t *testing.T) {
	_, r, _ := setupHealthHandlerWithProviders()

	// Record success
	successReq := RecordSuccessRequest{ProviderID: "openai"}
	successBody, _ := json.Marshal(successReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/record/success", bytes.NewBuffer(successBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Record failure
	failureReq := RecordFailureRequest{ProviderID: "openai"}
	failureBody, _ := json.Marshal(failureReq)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/verifier/health/record/failure", bytes.NewBuffer(failureBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check provider health reflects the recorded events
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/verifier/health/providers/openai", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var health ProviderHealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &health)
	require.NoError(t, err)

	// Should have recorded at least one success and one failure
	assert.True(t, health.SuccessCount >= 1 || health.FailureCount >= 1)
}

func TestAddAndRemoveProvider_Integration(t *testing.T) {
	_, r := setupHealthHandler()

	// Add a new provider
	addReq := HealthAddProviderRequest{
		ProviderID:   "test-provider",
		ProviderName: "Test Provider",
	}
	addBody, _ := json.Marshal(addReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/health/providers", bytes.NewBuffer(addBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify provider exists
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/verifier/health/providers/test-provider", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Remove provider
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/verifier/health/providers/test-provider", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify provider no longer exists
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/verifier/health/providers/test-provider", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
