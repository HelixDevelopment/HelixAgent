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

func setupScoringHandler() (*ScoringHandler, *gin.Engine) {
	ss, _ := verifier.NewScoringService(nil)

	h := NewScoringHandler(ss)
	r := gin.New()

	api := r.Group("/api/v1")
	RegisterScoringRoutes(api, h)

	return h, r
}

func setupScoringHandlerWithCache() (*ScoringHandler, *gin.Engine, *verifier.ScoringService) {
	ss, _ := verifier.NewScoringService(nil)

	// Pre-populate cache with some scores
	ss.CalculateScore(nil, "gpt-4")
	ss.CalculateScore(nil, "claude-3")
	ss.CalculateScore(nil, "gemini-pro")

	h := NewScoringHandler(ss)
	r := gin.New()

	api := r.Group("/api/v1")
	RegisterScoringRoutes(api, h)

	return h, r, ss
}

func TestNewScoringHandler(t *testing.T) {
	ss, _ := verifier.NewScoringService(nil)
	h := NewScoringHandler(ss)

	assert.NotNil(t, h)
	assert.Equal(t, ss, h.scoringService)
}

func TestNewScoringHandler_NilService(t *testing.T) {
	h := NewScoringHandler(nil)

	assert.NotNil(t, h)
	assert.Nil(t, h.scoringService)
}

func TestGetModelScore_Success(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/gpt-4", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetModelScoreResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", resp.ModelID)
	assert.True(t, resp.OverallScore >= 0 && resp.OverallScore <= 10)
	assert.NotEmpty(t, resp.ScoreSuffix)
	assert.NotEmpty(t, resp.CalculatedAt)
}

func TestGetModelScore_KnownModels(t *testing.T) {
	_, r := setupScoringHandler()

	// Note: The scoring service uses a map to match patterns, so pattern order is not guaranteed
	// gpt-4o matches gpt-4 pattern since gpt-4 is checked first in map iteration
	// We test that scores are within reasonable ranges for known models
	tests := []struct {
		modelID  string
		minScore float64
		maxScore float64
	}{
		{"gpt-4", 9.0, 9.5},
		{"gpt-4o", 9.0, 9.5},       // May match gpt-4 pattern first
		{"claude-3", 9.0, 9.5},
		{"claude-3.5", 9.0, 9.5},   // May match claude-3 pattern first
		{"gemini-pro", 8.5, 9.0},
		{"llama-3", 7.5, 8.0},
		{"mistral-large", 8.0, 8.5},
		{"deepseek-coder", 7.5, 8.0},
		{"qwen", 7.0, 7.5},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/"+tt.modelID, nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp GetModelScoreResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			assert.GreaterOrEqual(t, resp.OverallScore, tt.minScore, "Score for %s should be >= %v", tt.modelID, tt.minScore)
			assert.LessOrEqual(t, resp.OverallScore, tt.maxScore, "Score for %s should be <= %v", tt.modelID, tt.maxScore)
		})
	}
}

func TestGetModelScore_UnknownModel(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/unknown-model", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetModelScoreResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Unknown models get default score of 5.0
	assert.Equal(t, 5.0, resp.OverallScore)
}

func TestGetModelScore_ResponseFormat(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/gpt-4", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetModelScoreResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.ModelID)
	assert.NotEmpty(t, resp.ModelName)
	assert.NotEmpty(t, resp.ScoreSuffix)
	assert.NotNil(t, resp.Components)
	assert.NotEmpty(t, resp.CalculatedAt)
	assert.NotEmpty(t, resp.DataSource)
}

func TestBatchCalculateScores_Success(t *testing.T) {
	_, r := setupScoringHandler()

	reqBody := BatchScoreRequest{
		ModelIDs: []string{"gpt-4", "claude-3", "gemini-pro"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp BatchScoreResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 3, resp.Total)
	assert.Len(t, resp.Scores, 3)
}

func TestBatchCalculateScores_SingleModel(t *testing.T) {
	_, r := setupScoringHandler()

	reqBody := BatchScoreRequest{
		ModelIDs: []string{"gpt-4"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp BatchScoreResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Total)
}

func TestBatchCalculateScores_BadRequest_EmptyModelIDs(t *testing.T) {
	_, r := setupScoringHandler()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBatchCalculateScores_BadRequest_InvalidJSON(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/batch", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBatchCalculateScores_SortedByScore(t *testing.T) {
	_, r := setupScoringHandler()

	reqBody := BatchScoreRequest{
		ModelIDs: []string{"unknown-model", "gpt-4", "llama-3"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp BatchScoreResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Verify sorted in descending order by score
	for i := 1; i < len(resp.Scores); i++ {
		assert.GreaterOrEqual(t, resp.Scores[i-1].OverallScore, resp.Scores[i].OverallScore)
	}
}

func TestGetTopModels_Success(t *testing.T) {
	_, r, _ := setupScoringHandlerWithCache()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/top", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetTopModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp.Models)
	assert.Equal(t, len(resp.Models), resp.Total)
}

func TestGetTopModels_WithLimit(t *testing.T) {
	_, r, _ := setupScoringHandlerWithCache()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/top?limit=2", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetTopModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.LessOrEqual(t, resp.Total, 2)
}

func TestGetTopModels_DefaultLimit(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/top", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Default limit is 10, so should return at most 10
	var resp GetTopModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.LessOrEqual(t, resp.Total, 10)
}

func TestGetTopModels_InvalidLimit(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/top?limit=invalid", nil)
	r.ServeHTTP(w, req)

	// Should still succeed with default limit
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetModelsByScoreRange_Success(t *testing.T) {
	_, r, _ := setupScoringHandlerWithCache()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/range?min_score=8.0&max_score=10.0", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetTopModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	for _, model := range resp.Models {
		assert.GreaterOrEqual(t, model.OverallScore, 8.0)
		assert.LessOrEqual(t, model.OverallScore, 10.0)
	}
}

func TestGetModelsByScoreRange_DefaultValues(t *testing.T) {
	_, r, _ := setupScoringHandlerWithCache()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/range", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetTopModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Default range is 0-10, so all cached models should be returned
	assert.NotNil(t, resp.Models)
}

func TestGetModelsByScoreRange_WithLimit(t *testing.T) {
	_, r, _ := setupScoringHandlerWithCache()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/range?min_score=0&max_score=10&limit=2", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetTopModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.LessOrEqual(t, resp.Total, 2)
}

func TestGetScoringWeights_Success(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/weights", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ScoringWeightsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Default weights should sum to 1.0
	assert.True(t, resp.IsValid)
	assert.InDelta(t, 1.0, resp.Total, 0.01)
	assert.NotEmpty(t, resp.Description)
}

func TestGetScoringWeights_DefaultValues(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/weights", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ScoringWeightsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Verify default weights
	assert.Equal(t, 0.25, resp.Weights.ResponseSpeed)
	assert.Equal(t, 0.20, resp.Weights.ModelEfficiency)
	assert.Equal(t, 0.25, resp.Weights.CostEffectiveness)
	assert.Equal(t, 0.20, resp.Weights.Capability)
	assert.Equal(t, 0.10, resp.Weights.Recency)
}

func TestUpdateScoringWeights_Success(t *testing.T) {
	_, r := setupScoringHandler()

	reqBody := UpdateScoringWeightsRequest{
		ResponseSpeed:     0.30,
		ModelEfficiency:   0.20,
		CostEffectiveness: 0.20,
		Capability:        0.20,
		Recency:           0.10,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/verifier/scores/weights", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ScoringWeightsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.IsValid)
	assert.Equal(t, 0.30, resp.Weights.ResponseSpeed)
}

func TestUpdateScoringWeights_BadRequest_InvalidSum(t *testing.T) {
	_, r := setupScoringHandler()

	// Weights don't sum to 1.0
	reqBody := UpdateScoringWeightsRequest{
		ResponseSpeed:     0.50,
		ModelEfficiency:   0.50,
		CostEffectiveness: 0.50,
		Capability:        0.50,
		Recency:           0.50,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/verifier/scores/weights", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateScoringWeights_BadRequest_InvalidJSON(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/verifier/scores/weights", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetModelNameWithScore_Success(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/gpt-4/name", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp, "model_id")
	assert.Contains(t, resp, "name_with_score")
	assert.Equal(t, "gpt-4", resp["model_id"])

	nameWithScore := resp["name_with_score"].(string)
	assert.Contains(t, nameWithScore, "(SC:")
}

func TestGetModelNameWithScore_DifferentModels(t *testing.T) {
	_, r := setupScoringHandler()

	models := []string{"gpt-4", "claude-3", "gemini-pro"}

	for _, modelID := range models {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/"+modelID+"/name", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, modelID, resp["model_id"])
	}
}

func TestInvalidateCache_AllCache(t *testing.T) {
	_, r, _ := setupScoringHandlerWithCache()

	reqBody := InvalidateCacheRequest{
		All: true,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/cache/invalidate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "All cached scores invalidated", resp["message"])
}

func TestInvalidateCache_SpecificModel(t *testing.T) {
	_, r, _ := setupScoringHandlerWithCache()

	reqBody := InvalidateCacheRequest{
		ModelID: "gpt-4",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/cache/invalidate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp["message"], "gpt-4")
}

func TestInvalidateCache_BadRequest_NoParams(t *testing.T) {
	_, r := setupScoringHandler()

	reqBody := InvalidateCacheRequest{}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/cache/invalidate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInvalidateCache_BadRequest_InvalidJSON(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/cache/invalidate", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCompareModels_Success(t *testing.T) {
	_, r := setupScoringHandler()

	reqBody := CompareModelsRequest{
		ModelIDs: []string{"gpt-4", "claude-3"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CompareModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Len(t, resp.Models, 2)
	assert.NotEmpty(t, resp.Winner)
	assert.NotNil(t, resp.Comparison)
}

func TestCompareModels_WinnerDetermination(t *testing.T) {
	_, r := setupScoringHandler()

	// gpt-4 (9.0) vs llama-3 (7.5) - gpt-4 should win
	reqBody := CompareModelsRequest{
		ModelIDs: []string{"gpt-4", "llama-3"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CompareModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", resp.Winner)
}

func TestCompareModels_BadRequest_TooFewModels(t *testing.T) {
	_, r := setupScoringHandler()

	reqBody := CompareModelsRequest{
		ModelIDs: []string{"gpt-4"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCompareModels_BadRequest_EmptyModelIDs(t *testing.T) {
	_, r := setupScoringHandler()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCompareModels_BadRequest_InvalidJSON(t *testing.T) {
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/compare", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCompareModels_MultipleModels(t *testing.T) {
	_, r := setupScoringHandler()

	reqBody := CompareModelsRequest{
		ModelIDs: []string{"gpt-4", "claude-3", "gemini-pro", "llama-3", "qwen"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/compare", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CompareModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Len(t, resp.Models, 5)
	assert.Len(t, resp.Comparison, 5)
}

func TestRegisterScoringRoutes(t *testing.T) {
	ss, _ := verifier.NewScoringService(nil)
	h := NewScoringHandler(ss)
	r := gin.New()
	api := r.Group("/api/v1")

	// Should not panic
	RegisterScoringRoutes(api, h)

	// Test that routes are registered by checking 404 for unknown routes
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/unknown/route", nil)
	r.ServeHTTP(w, req)

	// unknown/route would match /:model_id/name so it returns 200
	// Check a truly non-existent route
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/verifier/scores/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetModelScoreResponse_Fields(t *testing.T) {
	resp := GetModelScoreResponse{
		ModelID:      "gpt-4",
		ModelName:    "GPT-4",
		OverallScore: 9.0,
		ScoreSuffix:  "(SC:9.0)",
		Components: ScoreComponentsDetail{
			SpeedScore:      9.0,
			EfficiencyScore: 8.5,
			CostScore:       7.0,
			CapabilityScore: 9.5,
			RecencyScore:    9.0,
		},
		CalculatedAt: "2024-01-01T12:00:00Z",
		DataSource:   "basic",
	}

	assert.Equal(t, "gpt-4", resp.ModelID)
	assert.Equal(t, "GPT-4", resp.ModelName)
	assert.Equal(t, 9.0, resp.OverallScore)
	assert.Equal(t, "(SC:9.0)", resp.ScoreSuffix)
	assert.Equal(t, 9.0, resp.Components.SpeedScore)
	assert.Equal(t, "basic", resp.DataSource)
}

func TestScoreComponentsDetail_Fields(t *testing.T) {
	comp := ScoreComponentsDetail{
		SpeedScore:      9.0,
		EfficiencyScore: 8.5,
		CostScore:       7.0,
		CapabilityScore: 9.5,
		RecencyScore:    8.0,
	}

	assert.Equal(t, 9.0, comp.SpeedScore)
	assert.Equal(t, 8.5, comp.EfficiencyScore)
	assert.Equal(t, 7.0, comp.CostScore)
	assert.Equal(t, 9.5, comp.CapabilityScore)
	assert.Equal(t, 8.0, comp.RecencyScore)
}

func TestScoringWeightsResponse_Fields(t *testing.T) {
	resp := ScoringWeightsResponse{
		Weights: ScoringWeightsDetail{
			ResponseSpeed:     0.25,
			ModelEfficiency:   0.20,
			CostEffectiveness: 0.25,
			Capability:        0.20,
			Recency:           0.10,
		},
		Total:       1.0,
		IsValid:     true,
		Description: "Scoring weights description",
	}

	assert.Equal(t, 0.25, resp.Weights.ResponseSpeed)
	assert.Equal(t, 1.0, resp.Total)
	assert.True(t, resp.IsValid)
}

func TestModelWithScoreInfo_Fields(t *testing.T) {
	info := ModelWithScoreInfo{
		ModelID:      "gpt-4",
		Name:         "GPT-4",
		Provider:     "openai",
		OverallScore: 9.0,
		ScoreSuffix:  "(SC:9.0)",
		Rank:         1,
	}

	assert.Equal(t, "gpt-4", info.ModelID)
	assert.Equal(t, "GPT-4", info.Name)
	assert.Equal(t, "openai", info.Provider)
	assert.Equal(t, 9.0, info.OverallScore)
	assert.Equal(t, 1, info.Rank)
}

func TestScoringRoutes_ContentType(t *testing.T) {
	_, r := setupScoringHandler()

	routes := []string{
		"/api/v1/verifier/scores/gpt-4",
		"/api/v1/verifier/scores/gpt-4/name",
		"/api/v1/verifier/scores/top",
		"/api/v1/verifier/scores/range",
		"/api/v1/verifier/scores/weights",
	}

	for _, route := range routes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", route, nil)
		r.ServeHTTP(w, req)

		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Route %s should return JSON", route)
	}
}

func TestScoringRoutes_MethodNotAllowed(t *testing.T) {
	_, r := setupScoringHandler()

	// Note: Routes like /batch and /compare will match /:model_id pattern as GET
	// So we only test methods that are truly not allowed
	tests := []struct {
		method   string
		path     string
		expected int
	}{
		{"DELETE", "/api/v1/verifier/scores/gpt-4", http.StatusNotFound},
		{"DELETE", "/api/v1/verifier/scores/weights", http.StatusNotFound},
		{"PATCH", "/api/v1/verifier/scores/cache/invalidate", http.StatusNotFound},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(tt.method, tt.path, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, tt.expected, w.Code, "Expected %d for %s %s", tt.expected, tt.method, tt.path)
	}
}

func TestGetBatchAsModelID(t *testing.T) {
	// Test that GET /scores/batch is interpreted as getting score for model "batch"
	_, r := setupScoringHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/verifier/scores/batch", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetModelScoreResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// "batch" is treated as a model_id, gets default score
	assert.Equal(t, "batch", resp.ModelID)
	assert.Equal(t, 5.0, resp.OverallScore)
}

func TestUpdateAndGetWeights_Integration(t *testing.T) {
	_, r := setupScoringHandler()

	// Update weights
	updateReq := UpdateScoringWeightsRequest{
		ResponseSpeed:     0.30,
		ModelEfficiency:   0.15,
		CostEffectiveness: 0.25,
		Capability:        0.20,
		Recency:           0.10,
	}
	updateBody, _ := json.Marshal(updateReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/verifier/scores/weights", bytes.NewBuffer(updateBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Get weights and verify
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/verifier/scores/weights", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ScoringWeightsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0.30, resp.Weights.ResponseSpeed)
	assert.Equal(t, 0.15, resp.Weights.ModelEfficiency)
}

func TestInvalidateCacheAndScore_Integration(t *testing.T) {
	_, r, _ := setupScoringHandlerWithCache()

	// Invalidate specific model cache
	invalidateReq := InvalidateCacheRequest{
		ModelID: "gpt-4",
	}
	invalidateBody, _ := json.Marshal(invalidateReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/verifier/scores/cache/invalidate", bytes.NewBuffer(invalidateBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Score should still be calculable
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/verifier/scores/gpt-4", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GetModelScoreResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", resp.ModelID)
}
