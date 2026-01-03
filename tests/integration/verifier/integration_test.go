package verifier_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/superagent/superagent/internal/handlers"
	"github.com/superagent/superagent/internal/verifier"
	"github.com/superagent/superagent/internal/verifier/adapters"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *verifier.VerificationService, *verifier.ScoringService, *verifier.HealthService) {
	gin.SetMode(gin.TestMode)

	cfg := verifier.DefaultConfig()
	cfg.Verification.VerificationTimeout = 5 * time.Second

	verificationService := verifier.NewVerificationService(cfg)
	require.NotNil(t, verificationService)

	scoringService, err := verifier.NewScoringService(cfg)
	require.NoError(t, err)

	healthService := verifier.NewHealthService(cfg)

	registryCfg := &adapters.ExtendedRegistryConfig{}
	registry, err := adapters.NewExtendedProviderRegistry(registryCfg)
	require.NoError(t, err)

	verificationHandler := handlers.NewVerificationHandler(verificationService, scoringService, healthService, registry)
	scoringHandler := handlers.NewScoringHandler(scoringService)
	healthHandler := handlers.NewHealthHandler(healthService)

	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterVerificationRoutes(api, verificationHandler)
	handlers.RegisterScoringRoutes(api, scoringHandler)
	handlers.RegisterHealthRoutes(api, healthHandler)

	return router, verificationService, scoringService, healthService
}

func TestVerificationEndpoint_VerifyModel(t *testing.T) {
	router, service, _, _ := setupTestRouter(t)

	service.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your Python code.", nil
	})

	req := httptest.NewRequest("POST", "/api/v1/verifier/verify", strings.NewReader(`{
		"model_id": "gpt-4",
		"provider": "openai"
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.VerifyModelResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", response.ModelID)
	assert.Equal(t, "openai", response.Provider)
}

func TestVerificationEndpoint_BatchVerify(t *testing.T) {
	router, service, _, _ := setupTestRouter(t)

	service.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I see your code.", nil
	})

	req := httptest.NewRequest("POST", "/api/v1/verifier/verify/batch", strings.NewReader(`{
		"models": [
			{"model_id": "gpt-4", "provider": "openai"},
			{"model_id": "claude-3", "provider": "anthropic"}
		]
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.BatchVerifyResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Results, 2)
	assert.Equal(t, 2, response.Summary.Total)
}

func TestVerificationEndpoint_GetVerificationTests(t *testing.T) {
	router, _, _, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/v1/verifier/tests", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var tests map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &tests)
	require.NoError(t, err)

	assert.Contains(t, tests, "code_visibility")
	assert.Contains(t, tests, "existence")
	assert.Contains(t, tests, "streaming")
}

func TestScoringEndpoint_GetModelScore(t *testing.T) {
	router, _, _, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/v1/verifier/scores/gpt-4", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.GetModelScoreResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", response.ModelID)
	assert.GreaterOrEqual(t, response.OverallScore, 0.0)
	assert.LessOrEqual(t, response.OverallScore, 10.0)
}

func TestScoringEndpoint_GetTopModels(t *testing.T) {
	router, _, _, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/v1/verifier/scores/top?limit=5", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.GetTopModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(response.Models), 0)
}

func TestScoringEndpoint_GetScoringWeights(t *testing.T) {
	router, _, _, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/v1/verifier/scores/weights", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.ScoringWeightsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.IsValid)
	assert.InDelta(t, 1.0, response.Total, 0.01)
}

func TestScoringEndpoint_UpdateScoringWeights(t *testing.T) {
	router, _, _, _ := setupTestRouter(t)

	req := httptest.NewRequest("PUT", "/api/v1/verifier/scores/weights", strings.NewReader(`{
		"response_speed": 0.30,
		"model_efficiency": 0.20,
		"cost_effectiveness": 0.20,
		"capability": 0.20,
		"recency": 0.10
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.ScoringWeightsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 0.30, response.Weights.ResponseSpeed)
}

func TestScoringEndpoint_CompareModels(t *testing.T) {
	router, _, _, _ := setupTestRouter(t)

	req := httptest.NewRequest("POST", "/api/v1/verifier/scores/compare", strings.NewReader(`{
		"model_ids": ["gpt-4", "claude-3"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.CompareModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Models, 2)
	assert.NotEmpty(t, response.Winner)
}

func TestHealthEndpoint_GetAllProvidersHealth(t *testing.T) {
	router, _, _, healthService := setupTestRouter(t)

	healthService.AddProvider("openai-1", "openai")
	healthService.AddProvider("anthropic-1", "anthropic")

	req := httptest.NewRequest("GET", "/api/v1/verifier/health/providers", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.GetAllProvidersHealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 2, response.Total)
	assert.Len(t, response.Providers, 2)
}

func TestHealthEndpoint_GetHealthyProviders(t *testing.T) {
	router, _, _, healthService := setupTestRouter(t)

	healthService.AddProvider("openai-1", "openai")

	req := httptest.NewRequest("GET", "/api/v1/verifier/health/healthy", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.GetHealthyProvidersResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, response.Count, 0)
}

func TestHealthEndpoint_AddRemoveProvider(t *testing.T) {
	router, _, _, _ := setupTestRouter(t)

	// Add provider
	addReq := httptest.NewRequest("POST", "/api/v1/verifier/health/providers", strings.NewReader(`{
		"provider_id": "test-1",
		"provider_name": "test"
	}`))
	addReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, addReq)
	assert.Equal(t, http.StatusOK, w.Code)

	// Get provider
	getReq := httptest.NewRequest("GET", "/api/v1/verifier/health/providers/test-1", nil)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, getReq)
	assert.Equal(t, http.StatusOK, w.Code)

	// Remove provider
	delReq := httptest.NewRequest("DELETE", "/api/v1/verifier/health/providers/test-1", nil)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, delReq)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthEndpoint_GetFastestProvider(t *testing.T) {
	router, _, _, healthService := setupTestRouter(t)

	healthService.AddProvider("fast-1", "fast")
	healthService.AddProvider("slow-1", "slow")

	req := httptest.NewRequest("POST", "/api/v1/verifier/health/fastest", strings.NewReader(`{
		"providers": ["fast-1", "slow-1"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.GetFastestProviderResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response.ProviderID)
}

func TestHealthEndpoint_RecordSuccessFailure(t *testing.T) {
	router, _, _, healthService := setupTestRouter(t)

	healthService.AddProvider("test-1", "test")

	// Record success
	successReq := httptest.NewRequest("POST", "/api/v1/verifier/health/record/success", strings.NewReader(`{
		"provider_id": "test-1"
	}`))
	successReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, successReq)
	assert.Equal(t, http.StatusOK, w.Code)

	// Record failure
	failureReq := httptest.NewRequest("POST", "/api/v1/verifier/health/record/failure", strings.NewReader(`{
		"provider_id": "test-1"
	}`))
	failureReq.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	router.ServeHTTP(w, failureReq)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthEndpoint_IsProviderAvailable(t *testing.T) {
	router, _, _, healthService := setupTestRouter(t)

	healthService.AddProvider("test-1", "test")

	req := httptest.NewRequest("GET", "/api/v1/verifier/health/available/test-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["available"].(bool))
}

func TestVerificationHealth(t *testing.T) {
	router, _, _, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/v1/verifier/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response handlers.VerificationHealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
}
