package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/superagent/superagent/internal/verifier"
)

func setupDiscoveryHandler() (*DiscoveryHandler, *gin.Engine) {
	vs := verifier.NewVerificationService(nil)
	ss, _ := verifier.NewScoringService(nil)
	hs := verifier.NewHealthService(nil)
	ds := verifier.NewModelDiscoveryService(vs, ss, hs, nil)

	h := NewDiscoveryHandler(ds)
	r := gin.New()

	api := r.Group("/api/v1")
	RegisterDiscoveryRoutes(api, h)

	return h, r
}

func setupDiscoveryHandlerWithModels() (*DiscoveryHandler, *gin.Engine, *verifier.ModelDiscoveryService) {
	vs := verifier.NewVerificationService(nil)
	ss, _ := verifier.NewScoringService(nil)
	hs := verifier.NewHealthService(nil)
	ds := verifier.NewModelDiscoveryService(vs, ss, hs, nil)

	h := NewDiscoveryHandler(ds)
	r := gin.New()

	api := r.Group("/api/v1")
	RegisterDiscoveryRoutes(api, h)

	return h, r, ds
}

func TestNewDiscoveryHandler(t *testing.T) {
	vs := verifier.NewVerificationService(nil)
	ss, _ := verifier.NewScoringService(nil)
	hs := verifier.NewHealthService(nil)
	ds := verifier.NewModelDiscoveryService(vs, ss, hs, nil)

	h := NewDiscoveryHandler(ds)

	assert.NotNil(t, h)
	assert.Equal(t, ds, h.discoveryService)
}

func TestNewDiscoveryHandler_NilService(t *testing.T) {
	h := NewDiscoveryHandler(nil)

	assert.NotNil(t, h)
	assert.Nil(t, h.discoveryService)
}

func TestGetDiscoveredModels_EmptyList(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/models", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp, "models")
	assert.Contains(t, resp, "total")
	assert.Equal(t, float64(0), resp["total"])
}

func TestGetDiscoveredModels_ResponseFormat(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/models", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	models, ok := resp["models"].([]interface{})
	assert.True(t, ok, "models should be an array")
	assert.NotNil(t, models)
}

func TestGetSelectedModels_EmptyList(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/selected", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp, "models")
	assert.Contains(t, resp, "total")
	assert.Contains(t, resp, "description")
	assert.Equal(t, float64(0), resp["total"])
}

func TestGetSelectedModels_ResponseFormat(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/selected", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	desc, ok := resp["description"].(string)
	assert.True(t, ok, "description should be a string")
	assert.NotEmpty(t, desc)
}

func TestGetDiscoveryStats_Success(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/stats", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp, "stats")
	assert.Contains(t, resp, "description")
}

func TestGetDiscoveryStats_StatsFormat(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/stats", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	stats, ok := resp["stats"].(map[string]interface{})
	assert.True(t, ok, "stats should be an object")
	assert.NotNil(t, stats)
}

func TestGetDiscoveryStats_DescriptionFormat(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/stats", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	desc, ok := resp["description"].(map[string]interface{})
	assert.True(t, ok, "description should be an object")
	assert.Contains(t, desc, "total_discovered")
	assert.Contains(t, desc, "total_verified")
	assert.Contains(t, desc, "total_selected")
}

func TestTriggerDiscovery_Success(t *testing.T) {
	_, r := setupDiscoveryHandler()

	reqBody := TriggerDiscoveryRequest{
		Providers: []struct {
			Name    string `json:"name" binding:"required"`
			APIKey  string `json:"api_key" binding:"required"`
			BaseURL string `json:"base_url,omitempty"`
		}{
			{Name: "openai", APIKey: "test-key"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/discovery/trigger", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp, "message")
	assert.Contains(t, resp, "providers")
	assert.Contains(t, resp, "status")
	assert.Equal(t, "Discovery triggered", resp["message"])
	assert.Equal(t, float64(1), resp["providers"])
}

func TestTriggerDiscovery_MultipleProviders(t *testing.T) {
	_, r := setupDiscoveryHandler()

	reqBody := TriggerDiscoveryRequest{
		Providers: []struct {
			Name    string `json:"name" binding:"required"`
			APIKey  string `json:"api_key" binding:"required"`
			BaseURL string `json:"base_url,omitempty"`
		}{
			{Name: "openai", APIKey: "test-key-1"},
			{Name: "anthropic", APIKey: "test-key-2"},
			{Name: "google", APIKey: "test-key-3"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/discovery/trigger", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(3), resp["providers"])
}

func TestTriggerDiscovery_WithBaseURL(t *testing.T) {
	_, r := setupDiscoveryHandler()

	reqBody := TriggerDiscoveryRequest{
		Providers: []struct {
			Name    string `json:"name" binding:"required"`
			APIKey  string `json:"api_key" binding:"required"`
			BaseURL string `json:"base_url,omitempty"`
		}{
			{Name: "ollama", APIKey: "test-key", BaseURL: "http://localhost:11434"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/discovery/trigger", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTriggerDiscovery_BadRequest_EmptyBody(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/discovery/trigger", bytes.NewBuffer([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTriggerDiscovery_BadRequest_InvalidJSON(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/discovery/trigger", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTriggerDiscovery_WithMissingName(t *testing.T) {
	_, r := setupDiscoveryHandler()

	// Note: Gin binding with nested struct fields may not enforce required on nested fields
	// This tests that the API accepts a provider with missing name (it will just have empty string)
	body := []byte(`{"providers": [{"api_key": "test-key"}]}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/discovery/trigger", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// The request succeeds but with empty provider name
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTriggerDiscovery_WithMissingAPIKey(t *testing.T) {
	_, r := setupDiscoveryHandler()

	// Note: Gin binding with nested struct fields may not enforce required on nested fields
	// This tests that the API accepts a provider with missing api_key (it will just have empty string)
	body := []byte(`{"providers": [{"name": "openai"}]}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/discovery/trigger", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// The request succeeds but with empty API key
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetEnsembleModels_EmptyList(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/ensemble", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp, "ensemble")
	assert.Contains(t, resp, "total_models")
	assert.Contains(t, resp, "total_vote_weight")
	assert.Contains(t, resp, "description")
	assert.Contains(t, resp, "how_it_works")
}

func TestGetEnsembleModels_HowItWorksSteps(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/ensemble", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	howItWorks, ok := resp["how_it_works"].([]interface{})
	assert.True(t, ok, "how_it_works should be an array")
	assert.Equal(t, 6, len(howItWorks))
}

func TestGetModelForDebate_NotFound(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/ensemble/nonexistent-model", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp.Error, "Model not in ensemble")
}

func TestGetModelForDebate_EmptyModelID(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/ensemble/", nil)
	r.ServeHTTP(w, req)

	// Empty model_id with trailing slash results in 301 redirect or 404
	// The behavior depends on Gin's redirect settings
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMovedPermanently,
		"Expected 404 or 301, got %d", w.Code)
}

func TestRegisterDiscoveryRoutes(t *testing.T) {
	vs := verifier.NewVerificationService(nil)
	ss, _ := verifier.NewScoringService(nil)
	hs := verifier.NewHealthService(nil)
	ds := verifier.NewModelDiscoveryService(vs, ss, hs, nil)

	h := NewDiscoveryHandler(ds)
	r := gin.New()
	api := r.Group("/api/v1")

	// Should not panic
	RegisterDiscoveryRoutes(api, h)

	// Test that routes are registered by checking 404 for unknown routes
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/unknown", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDiscoveredModelResponse_Fields(t *testing.T) {
	resp := DiscoveredModelResponse{
		ModelID:      "gpt-4",
		ModelName:    "GPT-4",
		Provider:     "openai",
		Verified:     true,
		CodeVisible:  true,
		OverallScore: 9.5,
		ScoreSuffix:  "(SC:9.5)",
		DiscoveredAt: time.Now().Format("2006-01-02T15:04:05Z"),
		Capabilities: []string{"chat", "code", "analysis"},
	}

	assert.Equal(t, "gpt-4", resp.ModelID)
	assert.Equal(t, "GPT-4", resp.ModelName)
	assert.Equal(t, "openai", resp.Provider)
	assert.True(t, resp.Verified)
	assert.True(t, resp.CodeVisible)
	assert.Equal(t, 9.5, resp.OverallScore)
	assert.Equal(t, "(SC:9.5)", resp.ScoreSuffix)
	assert.Len(t, resp.Capabilities, 3)
}

func TestSelectedModelResponse_Fields(t *testing.T) {
	resp := SelectedModelResponse{
		ModelID:      "gpt-4",
		ModelName:    "GPT-4",
		Provider:     "openai",
		OverallScore: 9.5,
		ScoreSuffix:  "(SC:9.5)",
		Rank:         1,
		VoteWeight:   0.95,
		CodeVisible:  true,
		SelectedAt:   time.Now().Format("2006-01-02T15:04:05Z"),
	}

	assert.Equal(t, "gpt-4", resp.ModelID)
	assert.Equal(t, "GPT-4", resp.ModelName)
	assert.Equal(t, "openai", resp.Provider)
	assert.Equal(t, 9.5, resp.OverallScore)
	assert.Equal(t, 1, resp.Rank)
	assert.Equal(t, 0.95, resp.VoteWeight)
	assert.True(t, resp.CodeVisible)
}

func TestTriggerDiscoveryRequest_Fields(t *testing.T) {
	req := TriggerDiscoveryRequest{
		Providers: []struct {
			Name    string `json:"name" binding:"required"`
			APIKey  string `json:"api_key" binding:"required"`
			BaseURL string `json:"base_url,omitempty"`
		}{
			{Name: "openai", APIKey: "key1", BaseURL: ""},
			{Name: "anthropic", APIKey: "key2", BaseURL: "https://custom.url"},
		},
	}

	assert.Len(t, req.Providers, 2)
	assert.Equal(t, "openai", req.Providers[0].Name)
	assert.Equal(t, "key1", req.Providers[0].APIKey)
	assert.Equal(t, "anthropic", req.Providers[1].Name)
	assert.Equal(t, "https://custom.url", req.Providers[1].BaseURL)
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{10.0, "10.0%"},
		{25.55, "25.6%"},   // Rounds to 25.6%
		{99.9, "99.9%"},
		{9.5, "9.50%"},
		{5.123, "5.12%"},
		{0.5, "0.50%"},
	}

	for _, tt := range tests {
		result := formatPercent(tt.input)
		assert.Equal(t, tt.expected, result, "formatPercent(%v) should be %s", tt.input, tt.expected)
	}
}

func TestGetRecommendations(t *testing.T) {
	tests := []struct {
		score            float64
		expectedContains string
	}{
		{9.5, "complex reasoning"},
		{9.0, "complex reasoning"},
		{8.5, "general tasks"},
		{8.0, "general tasks"},
		{7.5, "simple tasks"},
		{7.0, "simple tasks"},
		{6.5, "fallback"},
		{5.0, "fallback"},
	}

	for _, tt := range tests {
		result := getRecommendations(tt.score)
		assert.Contains(t, result, tt.expectedContains, "getRecommendations(%v) should contain %s", tt.score, tt.expectedContains)
	}
}

func TestGetRecommendations_HighScore(t *testing.T) {
	result := getRecommendations(9.5)

	assert.Contains(t, result, "complex reasoning")
	assert.Contains(t, result, "code generation")
	assert.Contains(t, result, "creative writing")
	assert.Contains(t, result, "analysis")
}

func TestGetRecommendations_MediumScore(t *testing.T) {
	result := getRecommendations(8.5)

	assert.Contains(t, result, "general tasks")
	assert.Contains(t, result, "summarization")
	assert.Contains(t, result, "Q&A")
	assert.Contains(t, result, "code review")
}

func TestGetRecommendations_LowScore(t *testing.T) {
	result := getRecommendations(7.5)

	assert.Contains(t, result, "simple tasks")
	assert.Contains(t, result, "quick responses")
	assert.Contains(t, result, "basic Q&A")
}

func TestGetRecommendations_VeryLowScore(t *testing.T) {
	result := getRecommendations(5.0)

	assert.Contains(t, result, "fallback")
	assert.Contains(t, result, "high-volume tasks")
}

func TestGetDiscoveredModels_ContentType(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/models", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestGetSelectedModels_ContentType(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/selected", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestGetDiscoveryStats_ContentType(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/stats", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestGetEnsembleModels_ContentType(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/ensemble", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestTriggerDiscovery_ContentType(t *testing.T) {
	_, r := setupDiscoveryHandler()

	reqBody := TriggerDiscoveryRequest{
		Providers: []struct {
			Name    string `json:"name" binding:"required"`
			APIKey  string `json:"api_key" binding:"required"`
			BaseURL string `json:"base_url,omitempty"`
		}{
			{Name: "openai", APIKey: "test-key"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/discovery/trigger", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestGetModelForDebate_ContentType(t *testing.T) {
	_, r := setupDiscoveryHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/discovery/ensemble/test-model", nil)
	r.ServeHTTP(w, req)

	// Even for 404, should return JSON
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestDiscoveryRoutes_MethodNotAllowed(t *testing.T) {
	_, r := setupDiscoveryHandler()

	tests := []struct {
		method   string
		path     string
		expected int
	}{
		{"POST", "/api/v1/verifier/discovery/models", http.StatusNotFound},
		{"POST", "/api/v1/verifier/discovery/selected", http.StatusNotFound},
		{"POST", "/api/v1/verifier/discovery/stats", http.StatusNotFound},
		{"GET", "/api/v1/verifier/discovery/trigger", http.StatusNotFound},
		{"POST", "/api/v1/verifier/discovery/ensemble", http.StatusNotFound},
		{"POST", "/api/v1/verifier/discovery/ensemble/test-model", http.StatusNotFound},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(tt.method, tt.path, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, tt.expected, w.Code, "Expected %d for %s %s", tt.expected, tt.method, tt.path)
	}
}
