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

	"dev.helix.agent/internal/llmops"
)

func setupLLMOpsHandler() (*LLMOpsHandler, *gin.Engine) {
	system := llmops.NewLLMOpsSystem(nil, nil)
	_ = system.Initialize()

	h := NewLLMOpsHandler(system)
	r := gin.New()

	api := r.Group("/v1")
	RegisterLLMOpsRoutes(api, h)

	return h, r
}

func setupLLMOpsHandlerUninit() (*LLMOpsHandler, *gin.Engine) {
	// System without Initialize — all sub-components are nil
	system := llmops.NewLLMOpsSystem(nil, nil)

	h := NewLLMOpsHandler(system)
	r := gin.New()

	api := r.Group("/v1")
	RegisterLLMOpsRoutes(api, h)

	return h, r
}

// --- constructor ---

func TestLLMOpsHandler_NewLLMOpsHandler(t *testing.T) {
	t.Parallel()
	system := llmops.NewLLMOpsSystem(nil, nil)
	h := NewLLMOpsHandler(system)

	assert.NotNil(t, h)
	assert.Equal(t, system, h.system)
}

func TestLLMOpsHandler_NewLLMOpsHandler_NilSystem(t *testing.T) {
	t.Parallel()
	h := NewLLMOpsHandler(nil)

	assert.NotNil(t, h)
	assert.Nil(t, h.system)
}

// --- CreateExperiment ---

func TestLLMOpsHandler_CreateExperiment_Success(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	reqBody := CreateExperimentRequest{
		Name: "test-experiment",
		Variants: []*llmops.Variant{
			{ID: "v1", Name: "Control", IsControl: true},
			{ID: "v2", Name: "Treatment"},
		},
		Metrics:      []string{"quality"},
		TargetMetric: "quality",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/experiments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp ExperimentResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "test-experiment", resp.Name)
	assert.Equal(t, llmops.ExperimentStatusDraft, resp.Status)
	assert.Len(t, resp.Variants, 2)
}

func TestLLMOpsHandler_CreateExperiment_BadRequest_MissingName(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	body := []byte(`{"variants": [{"id":"a","name":"A"},{"id":"b","name":"B"}]}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/experiments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMOpsHandler_CreateExperiment_BadRequest_TooFewVariants(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	body := []byte(`{"name":"exp","variants":[{"id":"a","name":"A"}]}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/experiments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMOpsHandler_CreateExperiment_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST", "/v1/llmops/experiments",
		bytes.NewBuffer([]byte(`{invalid}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMOpsHandler_CreateExperiment_UninitializedManager(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandlerUninit()

	reqBody := CreateExperimentRequest{
		Name: "exp",
		Variants: []*llmops.Variant{
			{ID: "v1", Name: "A"},
			{ID: "v2", Name: "B"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/experiments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "not initialized")
}

// --- ListExperiments ---

func TestLLMOpsHandler_ListExperiments_Empty(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/llmops/experiments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListExperimentsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Total)
	assert.Empty(t, resp.Experiments)
}

func TestLLMOpsHandler_ListExperiments_WithData(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	// Create an experiment first
	reqBody := CreateExperimentRequest{
		Name: "list-test",
		Variants: []*llmops.Variant{
			{ID: "v1", Name: "A", IsControl: true},
			{ID: "v2", Name: "B"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/experiments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Now list
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/llmops/experiments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListExperimentsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Total)
	assert.Len(t, resp.Experiments, 1)
	assert.Equal(t, "list-test", resp.Experiments[0].Name)
}

func TestLLMOpsHandler_ListExperiments_FilterByStatus(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	// Create an experiment (status=draft)
	reqBody := CreateExperimentRequest{
		Name: "filter-test",
		Variants: []*llmops.Variant{
			{ID: "v1", Name: "A", IsControl: true},
			{ID: "v2", Name: "B"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/experiments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Filter by running (should be empty)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/llmops/experiments?status=running", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListExperimentsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total)

	// Filter by draft (should have 1)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/llmops/experiments?status=draft", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
}

func TestLLMOpsHandler_ListExperiments_UninitializedManager(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandlerUninit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/llmops/experiments", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- GetExperiment ---

func TestLLMOpsHandler_GetExperiment_Success(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	// Create experiment
	reqBody := CreateExperimentRequest{
		Name: "get-test",
		Variants: []*llmops.Variant{
			{ID: "v1", Name: "A", IsControl: true},
			{ID: "v2", Name: "B"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/experiments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created ExperimentResponse
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	// Get by ID
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/llmops/experiments/"+created.ID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ExperimentResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, created.ID, resp.ID)
	assert.Equal(t, "get-test", resp.Name)
}

func TestLLMOpsHandler_GetExperiment_NotFound(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/llmops/experiments/nonexistent-id", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "not found")
}

func TestLLMOpsHandler_GetExperiment_UninitializedManager(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandlerUninit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/llmops/experiments/some-id", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- CreateEvaluation ---

func TestLLMOpsHandler_CreateEvaluation_BadRequest_MissingName(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	body := []byte(`{"dataset":"ds1"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/evaluate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMOpsHandler_CreateEvaluation_BadRequest_MissingDataset(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	body := []byte(`{"name":"eval1"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/evaluate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMOpsHandler_CreateEvaluation_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST", "/v1/llmops/evaluate",
		bytes.NewBuffer([]byte(`{bad json}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMOpsHandler_CreateEvaluation_DatasetNotFound(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	reqBody := CreateEvaluationRequest{
		Name:    "eval-run",
		Dataset: "nonexistent-dataset",
		Metrics: []string{"accuracy"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/evaluate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// The evaluator returns an error for missing dataset
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "dataset not found")
}

func TestLLMOpsHandler_CreateEvaluation_UninitializedEvaluator(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandlerUninit()

	reqBody := CreateEvaluationRequest{
		Name:    "eval",
		Dataset: "ds",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/evaluate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- ListPrompts ---

func TestLLMOpsHandler_ListPrompts_Empty(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/llmops/prompts", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListPromptsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Total)
	assert.Empty(t, resp.Prompts)
}

func TestLLMOpsHandler_ListPrompts_WithData(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	// Create a prompt
	createBody := CreatePromptVersionRequest{
		Name:    "greeting",
		Version: "1.0.0",
		Content: "Hello {{name}}!",
	}
	body, _ := json.Marshal(createBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/prompts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// List prompts
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/llmops/prompts", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListPromptsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Total)
	assert.Len(t, resp.Prompts, 1)
	assert.Equal(t, "greeting", resp.Prompts[0].Name)
}

func TestLLMOpsHandler_ListPrompts_UninitializedRegistry(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandlerUninit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/llmops/prompts", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- CreatePromptVersion ---

func TestLLMOpsHandler_CreatePromptVersion_Success(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	reqBody := CreatePromptVersionRequest{
		Name:        "summarize",
		Version:     "1.0.0",
		Content:     "Summarize: {{text}}",
		Author:      "test-author",
		Description: "A summarization prompt",
		Tags:        []string{"summarization"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/prompts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp PromptVersionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "summarize", resp.Name)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Equal(t, "Summarize: {{text}}", resp.Content)
	assert.Equal(t, "test-author", resp.Author)
	assert.True(t, resp.IsActive) // first version is auto-activated
}

func TestLLMOpsHandler_CreatePromptVersion_BadRequest_MissingName(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	body := []byte(`{"version":"1.0.0","content":"hello"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/prompts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMOpsHandler_CreatePromptVersion_BadRequest_MissingVersion(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	body := []byte(`{"name":"test","content":"hello"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/prompts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMOpsHandler_CreatePromptVersion_BadRequest_MissingContent(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	body := []byte(`{"name":"test","version":"1.0.0"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/prompts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMOpsHandler_CreatePromptVersion_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST", "/v1/llmops/prompts",
		bytes.NewBuffer([]byte(`{bad}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMOpsHandler_CreatePromptVersion_DuplicateVersion(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	reqBody := CreatePromptVersionRequest{
		Name:    "dup-test",
		Version: "1.0.0",
		Content: "Hello",
	}
	body, _ := json.Marshal(reqBody)

	// First creation succeeds
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/prompts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Second creation with same name/version fails
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/v1/llmops/prompts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "already exists")
}

func TestLLMOpsHandler_CreatePromptVersion_UninitializedRegistry(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandlerUninit()

	reqBody := CreatePromptVersionRequest{
		Name:    "test",
		Version: "1.0.0",
		Content: "hello",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/prompts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Route registration ---

func TestLLMOpsHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	system := llmops.NewLLMOpsSystem(nil, nil)
	h := NewLLMOpsHandler(system)
	r := gin.New()
	api := r.Group("/v1")

	// Should not panic
	RegisterLLMOpsRoutes(api, h)

	// Unknown route returns 404
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/llmops/unknown", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestLLMOpsHandler_ContentType(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	routes := []string{
		"/v1/llmops/experiments",
		"/v1/llmops/prompts",
	}

	for _, route := range routes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", route, nil)
		r.ServeHTTP(w, req)

		assert.Contains(t, w.Header().Get("Content-Type"), "application/json",
			"Route %s should return JSON", route)
	}
}

// --- Integration: create experiment then get ---

func TestLLMOpsHandler_CreateAndGetExperiment_Integration(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	// Create
	createReq := CreateExperimentRequest{
		Name:        "integration-exp",
		Description: "Integration test experiment",
		Variants: []*llmops.Variant{
			{ID: "ctrl", Name: "Control", IsControl: true},
			{ID: "treat", Name: "Treatment"},
		},
		Metrics:      []string{"quality", "latency"},
		TargetMetric: "quality",
	}
	body, _ := json.Marshal(createReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/experiments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created ExperimentResponse
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	// Get
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/llmops/experiments/"+created.ID, nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var fetched ExperimentResponse
	err = json.Unmarshal(w.Body.Bytes(), &fetched)
	require.NoError(t, err)

	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "integration-exp", fetched.Name)
	assert.Equal(t, "Integration test experiment", fetched.Description)
	assert.Equal(t, llmops.ExperimentStatusDraft, fetched.Status)

	// List
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/llmops/experiments", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var listed ListExperimentsResponse
	err = json.Unmarshal(w.Body.Bytes(), &listed)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, listed.Total, 1)
}

// --- Integration: create and list prompts ---

func TestLLMOpsHandler_CreateAndListPrompts_Integration(t *testing.T) {
	t.Parallel()
	_, r := setupLLMOpsHandler()

	// Create prompt v1
	p1 := CreatePromptVersionRequest{
		Name:    "qa-prompt",
		Version: "1.0.0",
		Content: "Answer: {{question}}",
		Author:  "tester",
	}
	body, _ := json.Marshal(p1)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/llmops/prompts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Create prompt v2
	p2 := CreatePromptVersionRequest{
		Name:    "qa-prompt",
		Version: "2.0.0",
		Content: "Please answer: {{question}}",
		Author:  "tester",
	}
	body, _ = json.Marshal(p2)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/v1/llmops/prompts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// List all
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/llmops/prompts", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListPromptsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 2, resp.Total)
}

// --- Response type field tests ---

func TestLLMOpsHandler_ExperimentResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := ExperimentResponse{
		ID:           "exp-1",
		Name:         "Test",
		Status:       llmops.ExperimentStatusDraft,
		Metrics:      []string{"quality"},
		TargetMetric: "quality",
		CreatedAt:    "2024-01-01T00:00:00Z",
		UpdatedAt:    "2024-01-01T00:00:00Z",
	}

	assert.Equal(t, "exp-1", resp.ID)
	assert.Equal(t, "Test", resp.Name)
	assert.Equal(t, llmops.ExperimentStatusDraft, resp.Status)
}

func TestLLMOpsHandler_PromptVersionResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := PromptVersionResponse{
		ID:       "p-1",
		Name:     "test-prompt",
		Version:  "1.0.0",
		Content:  "Hello",
		IsActive: true,
	}

	assert.Equal(t, "p-1", resp.ID)
	assert.Equal(t, "test-prompt", resp.Name)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.True(t, resp.IsActive)
}

func TestLLMOpsHandler_EvaluationRunResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := EvaluationRunResponse{
		ID:      "eval-1",
		Name:    "eval-run",
		Dataset: "ds1",
		Status:  llmops.EvaluationStatusPending,
	}

	assert.Equal(t, "eval-1", resp.ID)
	assert.Equal(t, "eval-run", resp.Name)
	assert.Equal(t, llmops.EvaluationStatusPending, resp.Status)
}
