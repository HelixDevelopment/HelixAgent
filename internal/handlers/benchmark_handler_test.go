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

	"dev.helix.agent/internal/benchmark"
)

func setupBenchmarkHandler() (*BenchmarkHandler, *gin.Engine) {
	system := benchmark.NewBenchmarkSystem(nil, nil)
	// Initialize with nil provider — runner will be functional but
	// LLM calls return "no provider available" which is fine for handler tests.
	_ = system.Initialize(nil)

	h := NewBenchmarkHandler(system)
	r := gin.New()

	api := r.Group("/v1")
	RegisterBenchmarkRoutes(api, h)

	return h, r
}

func setupBenchmarkHandlerUninit() (*BenchmarkHandler, *gin.Engine) {
	// System without Initialize — runner is nil
	system := benchmark.NewBenchmarkSystem(nil, nil)

	h := NewBenchmarkHandler(system)
	r := gin.New()

	api := r.Group("/v1")
	RegisterBenchmarkRoutes(api, h)

	return h, r
}

// waitForBenchmarkComplete polls the results endpoint until the run
// reaches a terminal status or the timeout elapses.
func waitForBenchmarkComplete(r *gin.Engine, runID string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/benchmark/results/"+runID, nil)
		r.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			var resp BenchmarkRunResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
				if resp.Status == benchmark.BenchmarkStatusCompleted ||
					resp.Status == benchmark.BenchmarkStatusFailed ||
					resp.Status == benchmark.BenchmarkStatusCancelled {
					return
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// --- constructor ---

func TestBenchmarkHandler_NewBenchmarkHandler(t *testing.T) {
	t.Parallel()
	system := benchmark.NewBenchmarkSystem(nil, nil)
	h := NewBenchmarkHandler(system)

	assert.NotNil(t, h)
	assert.Equal(t, system, h.system)
}

func TestBenchmarkHandler_NewBenchmarkHandler_NilSystem(t *testing.T) {
	t.Parallel()
	h := NewBenchmarkHandler(nil)

	assert.NotNil(t, h)
	assert.Nil(t, h.system)
}

// --- StartBenchmark ---

func TestBenchmarkHandler_StartBenchmark_Success(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	reqBody := StartBenchmarkRequest{
		BenchmarkType: benchmark.BenchmarkTypeSWEBench,
		ProviderName:  "test-provider",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/benchmark/run", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp BenchmarkRunResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, benchmark.BenchmarkTypeSWEBench, resp.BenchmarkType)
	assert.Equal(t, "test-provider", resp.ProviderName)
	assert.Equal(t, benchmark.BenchmarkStatusRunning, resp.Status)

	// Wait for the background goroutine to complete to avoid race detector
	// complaints from the runner's executeRun goroutine.
	waitForBenchmarkComplete(r, resp.ID, 5*time.Second)
}

func TestBenchmarkHandler_StartBenchmark_DefaultName(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	reqBody := StartBenchmarkRequest{
		BenchmarkType: benchmark.BenchmarkTypeHumanEval,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/benchmark/run", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp BenchmarkRunResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "humaneval benchmark", resp.Name)
	assert.Equal(t, "default", resp.ProviderName)

	waitForBenchmarkComplete(r, resp.ID, 5*time.Second)
}

func TestBenchmarkHandler_StartBenchmark_WithConfig(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	reqBody := StartBenchmarkRequest{
		BenchmarkType: benchmark.BenchmarkTypeMMLU,
		ProviderName:  "openai",
		ModelName:     "gpt-4",
		Config: &benchmark.BenchmarkConfig{
			MaxTasks:    5,
			Concurrency: 2,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/benchmark/run", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp BenchmarkRunResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "openai", resp.ProviderName)
	assert.Equal(t, "gpt-4", resp.ModelName)

	waitForBenchmarkComplete(r, resp.ID, 5*time.Second)
}

func TestBenchmarkHandler_StartBenchmark_BadRequest_MissingType(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	body := []byte(`{"provider_name":"test"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/benchmark/run", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBenchmarkHandler_StartBenchmark_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST", "/v1/benchmark/run",
		bytes.NewBuffer([]byte(`{bad json}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBenchmarkHandler_StartBenchmark_UninitializedRunner(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandlerUninit()

	reqBody := StartBenchmarkRequest{
		BenchmarkType: benchmark.BenchmarkTypeSWEBench,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/benchmark/run", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "not initialized")
}

// --- ListBenchmarkResults ---

func TestBenchmarkHandler_ListBenchmarkResults_Empty(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/benchmark/results", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListBenchmarkRunsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Total)
	assert.Empty(t, resp.Runs)
}

func TestBenchmarkHandler_ListBenchmarkResults_WithData(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	// Create and start a run
	reqBody := StartBenchmarkRequest{
		BenchmarkType: benchmark.BenchmarkTypeSWEBench,
		ProviderName:  "test",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/benchmark/run", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created BenchmarkRunResponse
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	// Wait for the background goroutine to finish before listing
	waitForBenchmarkComplete(r, created.ID, 5*time.Second)

	// List results
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/benchmark/results", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListBenchmarkRunsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Total)
	assert.Len(t, resp.Runs, 1)
}

func TestBenchmarkHandler_ListBenchmarkResults_FilterByType(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	// Create a swe-bench run
	reqBody := StartBenchmarkRequest{
		BenchmarkType: benchmark.BenchmarkTypeSWEBench,
		ProviderName:  "test",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/benchmark/run", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created BenchmarkRunResponse
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	// Wait for background goroutine
	waitForBenchmarkComplete(r, created.ID, 5*time.Second)

	// Filter by humaneval (should be empty)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/benchmark/results?benchmark_type=humaneval", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListBenchmarkRunsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total)

	// Filter by swe-bench (should have 1)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/benchmark/results?benchmark_type=swe-bench", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
}

func TestBenchmarkHandler_ListBenchmarkResults_FilterByProvider(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	// Create a run
	reqBody := StartBenchmarkRequest{
		BenchmarkType: benchmark.BenchmarkTypeSWEBench,
		ProviderName:  "openai",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/benchmark/run", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created BenchmarkRunResponse
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	// Wait for background goroutine
	waitForBenchmarkComplete(r, created.ID, 5*time.Second)

	// Filter by nonexistent provider
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/benchmark/results?provider_name=anthropic", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListBenchmarkRunsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total)
}

func TestBenchmarkHandler_ListBenchmarkResults_UninitializedRunner(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandlerUninit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/benchmark/results", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- GetBenchmarkResult ---

func TestBenchmarkHandler_GetBenchmarkResult_Success(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	// Create a run
	reqBody := StartBenchmarkRequest{
		Name:          "get-test",
		BenchmarkType: benchmark.BenchmarkTypeSWEBench,
		ProviderName:  "test",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/benchmark/run", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created BenchmarkRunResponse
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	// Wait for background goroutine
	waitForBenchmarkComplete(r, created.ID, 5*time.Second)

	// Get by ID
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/benchmark/results/"+created.ID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp BenchmarkRunResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, created.ID, resp.ID)
	assert.Equal(t, "get-test", resp.Name)
	assert.Equal(t, benchmark.BenchmarkTypeSWEBench, resp.BenchmarkType)
}

func TestBenchmarkHandler_GetBenchmarkResult_NotFound(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/benchmark/results/nonexistent-id", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "not found")
}

func TestBenchmarkHandler_GetBenchmarkResult_UninitializedRunner(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandlerUninit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/benchmark/results/some-id", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Route registration ---

func TestBenchmarkHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	system := benchmark.NewBenchmarkSystem(nil, nil)
	h := NewBenchmarkHandler(system)
	r := gin.New()
	api := r.Group("/v1")

	// Should not panic
	RegisterBenchmarkRoutes(api, h)

	// Unknown route returns 404
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/benchmark/unknown", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestBenchmarkHandler_ContentType(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/benchmark/results", nil)
	r.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

// --- Integration: create and retrieve ---

func TestBenchmarkHandler_CreateAndRetrieve_Integration(t *testing.T) {
	t.Parallel()
	_, r := setupBenchmarkHandler()

	// Create run
	createReq := StartBenchmarkRequest{
		Name:          "integration-benchmark",
		BenchmarkType: benchmark.BenchmarkTypeHumanEval,
		ProviderName:  "openai",
		ModelName:     "gpt-4",
	}
	body, _ := json.Marshal(createReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/benchmark/run", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created BenchmarkRunResponse
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)

	// Wait for the background goroutine to finish
	waitForBenchmarkComplete(r, created.ID, 5*time.Second)

	// Get by ID
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/benchmark/results/"+created.ID, nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var fetched BenchmarkRunResponse
	err = json.Unmarshal(w.Body.Bytes(), &fetched)
	require.NoError(t, err)

	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "integration-benchmark", fetched.Name)
	assert.Equal(t, benchmark.BenchmarkTypeHumanEval, fetched.BenchmarkType)
	assert.Equal(t, "openai", fetched.ProviderName)
	assert.Equal(t, "gpt-4", fetched.ModelName)

	// List (should contain it)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/benchmark/results", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var listed ListBenchmarkRunsResponse
	err = json.Unmarshal(w.Body.Bytes(), &listed)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, listed.Total, 1)
}

// --- Response type field tests ---

func TestBenchmarkHandler_BenchmarkRunResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := BenchmarkRunResponse{
		ID:            "run-1",
		Name:          "Test Run",
		BenchmarkType: benchmark.BenchmarkTypeSWEBench,
		ProviderName:  "openai",
		ModelName:     "gpt-4",
		Status:        benchmark.BenchmarkStatusCompleted,
		CreatedAt:     "2024-01-01T00:00:00Z",
	}

	assert.Equal(t, "run-1", resp.ID)
	assert.Equal(t, "Test Run", resp.Name)
	assert.Equal(t, benchmark.BenchmarkTypeSWEBench, resp.BenchmarkType)
	assert.Equal(t, benchmark.BenchmarkStatusCompleted, resp.Status)
}

func TestBenchmarkHandler_ListBenchmarkRunsResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := ListBenchmarkRunsResponse{
		Runs:  []BenchmarkRunResponse{{ID: "r1"}, {ID: "r2"}},
		Total: 2,
	}

	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Runs, 2)
}
