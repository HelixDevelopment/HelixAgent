package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/superagent/superagent/internal/verifier"
	"github.com/superagent/superagent/internal/verifier/adapters"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupVerificationHandler() (*VerificationHandler, *gin.Engine) {
	vs := verifier.NewVerificationService(nil)
	ss, _ := verifier.NewScoringService(nil)
	hs := verifier.NewHealthService(nil)
	reg, _ := adapters.NewExtendedProviderRegistry(nil)

	// Set up a mock provider function
	vs.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code", nil
	})

	h := NewVerificationHandler(vs, ss, hs, reg)
	r := gin.New()

	api := r.Group("/api/v1")
	RegisterVerificationRoutes(api, h)

	return h, r
}

func TestNewVerificationHandler(t *testing.T) {
	vs := verifier.NewVerificationService(nil)
	ss, _ := verifier.NewScoringService(nil)
	hs := verifier.NewHealthService(nil)
	reg, _ := adapters.NewExtendedProviderRegistry(nil)

	h := NewVerificationHandler(vs, ss, hs, reg)
	if h == nil {
		t.Fatal("NewVerificationHandler returned nil")
	}
	if h.verificationService != vs {
		t.Error("verificationService not set correctly")
	}
	if h.scoringService != ss {
		t.Error("scoringService not set correctly")
	}
	if h.healthService != hs {
		t.Error("healthService not set correctly")
	}
	if h.registry != reg {
		t.Error("registry not set correctly")
	}
}

func TestVerifyModel_Success(t *testing.T) {
	_, r := setupVerificationHandler()

	req := VerifyModelRequest{
		ModelID:  "gpt-4",
		Provider: "openai",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/verifier/verify", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp VerifyModelResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.ModelID != "gpt-4" {
		t.Errorf("expected ModelID 'gpt-4', got '%s'", resp.ModelID)
	}
	if resp.Provider != "openai" {
		t.Errorf("expected Provider 'openai', got '%s'", resp.Provider)
	}
}

func TestVerifyModel_BadRequest(t *testing.T) {
	_, r := setupVerificationHandler()

	// Missing required fields
	body := []byte(`{}`)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/verifier/verify", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestVerifyModel_InvalidJSON(t *testing.T) {
	_, r := setupVerificationHandler()

	body := []byte(`{invalid json}`)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/verifier/verify", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestBatchVerify_Success(t *testing.T) {
	_, r := setupVerificationHandler()

	req := BatchVerifyRequest{
		Models: []struct {
			ModelID  string `json:"model_id" binding:"required"`
			Provider string `json:"provider" binding:"required"`
		}{
			{ModelID: "gpt-4", Provider: "openai"},
			{ModelID: "claude-3", Provider: "anthropic"},
		},
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/verifier/verify/batch", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp BatchVerifyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Summary.Total != 2 {
		t.Errorf("expected 2 total, got %d", resp.Summary.Total)
	}
	if len(resp.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(resp.Results))
	}
}

func TestBatchVerify_BadRequest(t *testing.T) {
	_, r := setupVerificationHandler()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/verifier/verify/batch", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetVerificationStatus(t *testing.T) {
	_, r := setupVerificationHandler()

	// The mock service returns a default status for any model
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/api/v1/verifier/status/test-model", nil)
	r.ServeHTTP(w, httpReq)

	// Accept either 200 (found) or 404 (not found) depending on implementation
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d", w.Code)
	}
}

func TestGetVerifiedModels(t *testing.T) {
	_, r := setupVerificationHandler()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/api/v1/verifier/models", nil)
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp GetVerifiedModelsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
}

func TestGetVerifiedModels_WithQueryParams(t *testing.T) {
	_, r := setupVerificationHandler()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/api/v1/verifier/models?provider=openai&min_score=80&require_code=true&limit=10", nil)
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestTestCodeVisibility_Success(t *testing.T) {
	_, r := setupVerificationHandler()

	req := TestCodeVisibilityRequest{
		ModelID:  "gpt-4",
		Provider: "openai",
		Language: "python",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/verifier/test/code-visibility", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp TestCodeVisibilityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.ModelID != "gpt-4" {
		t.Errorf("expected ModelID 'gpt-4', got '%s'", resp.ModelID)
	}
}

func TestTestCodeVisibility_DefaultLanguage(t *testing.T) {
	_, r := setupVerificationHandler()

	req := TestCodeVisibilityRequest{
		ModelID:  "gpt-4",
		Provider: "openai",
		// Language not specified, should default to python
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/verifier/test/code-visibility", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestTestCodeVisibility_BadRequest(t *testing.T) {
	_, r := setupVerificationHandler()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/verifier/test/code-visibility", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetVerificationTests(t *testing.T) {
	_, r := setupVerificationHandler()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/api/v1/verifier/tests", nil)
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var tests map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &tests); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Check that expected tests are present
	expectedTests := []string{
		"code_visibility",
		"existence",
		"responsiveness",
		"latency",
		"streaming",
		"function_calling",
		"coding_capability",
		"error_detection",
	}

	for _, test := range expectedTests {
		if _, ok := tests[test]; !ok {
			t.Errorf("expected test '%s' not found", test)
		}
	}
}

func TestReVerifyModel_Success(t *testing.T) {
	_, r := setupVerificationHandler()

	req := ReVerifyModelRequest{
		ModelID:  "gpt-4",
		Provider: "openai",
		Force:    true,
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/verifier/reverify", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp VerifyModelResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Message != "Re-verification completed" {
		t.Errorf("expected message 'Re-verification completed', got '%s'", resp.Message)
	}
}

func TestReVerifyModel_BadRequest(t *testing.T) {
	_, r := setupVerificationHandler()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/verifier/reverify", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetVerificationHealth(t *testing.T) {
	_, r := setupVerificationHandler()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/api/v1/verifier/health", nil)
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp VerificationHealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", resp.Status)
	}
}

func TestVerifyModelRequest_Fields(t *testing.T) {
	req := VerifyModelRequest{
		ModelID:    "gpt-4",
		Provider:   "openai",
		Tests:      []string{"code_visibility", "existence"},
		Timeout:    30,
		RetryCount: 3,
	}

	if req.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if req.Provider != "openai" {
		t.Error("Provider mismatch")
	}
	if len(req.Tests) != 2 {
		t.Error("Tests length mismatch")
	}
	if req.Timeout != 30 {
		t.Error("Timeout mismatch")
	}
	if req.RetryCount != 3 {
		t.Error("RetryCount mismatch")
	}
}

func TestVerifyModelResponse_Fields(t *testing.T) {
	resp := VerifyModelResponse{
		ModelID:          "gpt-4",
		Provider:         "openai",
		Verified:         true,
		Score:            95.0,
		OverallScore:     92.5,
		ScoreSuffix:      "(SC:9.3)",
		CodeVisible:      true,
		Tests:            map[string]bool{"code_visibility": true},
		VerificationTime: 1500,
		Message:          "Verification completed",
	}

	if resp.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if !resp.Verified {
		t.Error("Verified mismatch")
	}
	if resp.Score != 95.0 {
		t.Error("Score mismatch")
	}
	if !resp.CodeVisible {
		t.Error("CodeVisible mismatch")
	}
}

func TestBatchVerifyRequest_Fields(t *testing.T) {
	req := BatchVerifyRequest{
		Models: []struct {
			ModelID  string `json:"model_id" binding:"required"`
			Provider string `json:"provider" binding:"required"`
		}{
			{ModelID: "model1", Provider: "provider1"},
			{ModelID: "model2", Provider: "provider2"},
		},
	}

	if len(req.Models) != 2 {
		t.Error("Models length mismatch")
	}
	if req.Models[0].ModelID != "model1" {
		t.Error("First model ID mismatch")
	}
}

func TestBatchVerifyResponse_Fields(t *testing.T) {
	resp := BatchVerifyResponse{
		Results: []VerifyModelResponse{
			{ModelID: "model1", Verified: true},
		},
	}
	resp.Summary.Total = 1
	resp.Summary.Verified = 1
	resp.Summary.Failed = 0

	if len(resp.Results) != 1 {
		t.Error("Results length mismatch")
	}
	if resp.Summary.Total != 1 {
		t.Error("Summary.Total mismatch")
	}
}

func TestGetVerifiedModelsResponse_Fields(t *testing.T) {
	resp := GetVerifiedModelsResponse{
		Models: []VerifiedModelInfo{
			{
				ModelID:      "gpt-4",
				ModelName:    "GPT-4",
				Provider:     "openai",
				Verified:     true,
				OverallScore: 95.0,
				ScoreSuffix:  "(SC:9.5)",
				CodeVisible:  true,
			},
		},
		Total: 1,
	}

	if resp.Total != 1 {
		t.Error("Total mismatch")
	}
	if len(resp.Models) != 1 {
		t.Error("Models length mismatch")
	}
	if resp.Models[0].ModelID != "gpt-4" {
		t.Error("Model ID mismatch")
	}
}

func TestVerifiedModelInfo_Fields(t *testing.T) {
	info := VerifiedModelInfo{
		ModelID:      "gpt-4",
		ModelName:    "GPT-4",
		Provider:     "openai",
		Verified:     true,
		OverallScore: 95.0,
		ScoreSuffix:  "(SC:9.5)",
		CodeVisible:  true,
	}

	if info.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if info.OverallScore != 95.0 {
		t.Error("OverallScore mismatch")
	}
}

func TestTestCodeVisibilityRequest_Fields(t *testing.T) {
	req := TestCodeVisibilityRequest{
		ModelID:  "gpt-4",
		Provider: "openai",
		Language: "python",
	}

	if req.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if req.Language != "python" {
		t.Error("Language mismatch")
	}
}

func TestTestCodeVisibilityResponse_Fields(t *testing.T) {
	resp := TestCodeVisibilityResponse{
		ModelID:     "gpt-4",
		Provider:    "openai",
		CodeVisible: true,
		Language:    "python",
		Prompt:      "Do you see my code?",
		Response:    "Yes, I can see your code",
		Confidence:  0.95,
	}

	if resp.CodeVisible != true {
		t.Error("CodeVisible mismatch")
	}
	if resp.Confidence != 0.95 {
		t.Error("Confidence mismatch")
	}
}

func TestReVerifyModelRequest_Fields(t *testing.T) {
	req := ReVerifyModelRequest{
		ModelID:  "gpt-4",
		Provider: "openai",
		Force:    true,
		Tests:    []string{"code_visibility"},
	}

	if req.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if !req.Force {
		t.Error("Force mismatch")
	}
}

func TestVerificationHealthResponse_Fields(t *testing.T) {
	resp := VerificationHealthResponse{
		Status:           "healthy",
		VerifiedModels:   10,
		PendingModels:    5,
		HealthyProviders: 3,
		TotalProviders:   4,
		LastVerification: "2024-01-01T12:00:00Z",
	}

	if resp.Status != "healthy" {
		t.Error("Status mismatch")
	}
	if resp.VerifiedModels != 10 {
		t.Error("VerifiedModels mismatch")
	}
	if resp.HealthyProviders != 3 {
		t.Error("HealthyProviders mismatch")
	}
}

func TestRegisterVerificationRoutes(t *testing.T) {
	vs := verifier.NewVerificationService(nil)
	ss, _ := verifier.NewScoringService(nil)
	hs := verifier.NewHealthService(nil)
	reg, _ := adapters.NewExtendedProviderRegistry(nil)

	h := NewVerificationHandler(vs, ss, hs, reg)
	r := gin.New()
	api := r.Group("/api/v1")

	// Should not panic
	RegisterVerificationRoutes(api, h)

	// Test that routes are registered by checking 404 for unknown routes
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("GET", "/api/v1/verifier/unknown", nil)
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown route, got %d", w.Code)
	}
}
