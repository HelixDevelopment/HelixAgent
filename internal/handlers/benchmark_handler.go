package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"dev.helix.agent/internal/benchmark"
)

// BenchmarkHandler provides HTTP endpoints for LLM benchmarking.
// Endpoints:
//
//	POST /v1/benchmark/run          - start a benchmark suite
//	GET  /v1/benchmark/results      - list benchmark results
//	GET  /v1/benchmark/results/:id  - get specific result
type BenchmarkHandler struct {
	system *benchmark.BenchmarkSystem
}

// NewBenchmarkHandler creates a new benchmark handler
func NewBenchmarkHandler(system *benchmark.BenchmarkSystem) *BenchmarkHandler {
	return &BenchmarkHandler{
		system: system,
	}
}

// checkSystem returns false and writes a 503 response if the benchmark system
// has not been initialised.
func (h *BenchmarkHandler) checkSystem(c *gin.Context) bool {
	if h.system == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "Benchmark system not initialized",
		})
		return false
	}
	return true
}

// --- request / response types ---

// StartBenchmarkRequest represents a request to start a benchmark run
type StartBenchmarkRequest struct {
	Name          string                     `json:"name,omitempty"`
	BenchmarkType benchmark.BenchmarkType    `json:"benchmark_type" binding:"required"`
	ProviderName  string                     `json:"provider_name,omitempty"`
	ModelName     string                     `json:"model_name,omitempty"`
	Config        *benchmark.BenchmarkConfig `json:"config,omitempty"`
}

// BenchmarkRunResponse represents a benchmark run in API responses
type BenchmarkRunResponse struct {
	ID            string                      `json:"id"`
	Name          string                      `json:"name"`
	Description   string                      `json:"description,omitempty"`
	BenchmarkType benchmark.BenchmarkType     `json:"benchmark_type"`
	ProviderName  string                      `json:"provider_name"`
	ModelName     string                      `json:"model_name,omitempty"`
	Status        benchmark.BenchmarkStatus   `json:"status"`
	Summary       *benchmark.BenchmarkSummary `json:"summary,omitempty"`
	CreatedAt     string                      `json:"created_at"`
}

// ListBenchmarkRunsResponse represents the list benchmark runs response
type ListBenchmarkRunsResponse struct {
	Runs  []BenchmarkRunResponse `json:"runs"`
	Total int                    `json:"total"`
}

// --- handlers ---

// StartBenchmark godoc
// @Summary Start a benchmark suite
// @Description Create and start a new benchmark run for a specific benchmark type
// @Tags benchmark
// @Accept json
// @Produce json
// @Param request body StartBenchmarkRequest true "Benchmark run details"
// @Success 201 {object} BenchmarkRunResponse
// @Failure 400 {object} VerifierErrorResponse
// @Failure 500 {object} VerifierErrorResponse
// @Router /v1/benchmark/run [post]
func (h *BenchmarkHandler) StartBenchmark(c *gin.Context) {
	if !h.checkSystem(c) {
		return
	}
	var req StartBenchmarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	runner := h.system.GetRunner()
	if runner == nil {
		c.JSON(
			http.StatusInternalServerError,
			VerifierErrorResponse{Error: "benchmark runner not initialized"},
		)
		return
	}

	config := req.Config
	if config == nil {
		config = benchmark.DefaultBenchmarkConfig()
	}

	run := &benchmark.BenchmarkRun{
		Name:          req.Name,
		BenchmarkType: req.BenchmarkType,
		ProviderName:  req.ProviderName,
		ModelName:     req.ModelName,
		Config:        config,
	}

	if run.Name == "" {
		run.Name = string(req.BenchmarkType) + " benchmark"
	}
	if run.ProviderName == "" {
		run.ProviderName = "default"
	}

	if err := runner.CreateRun(c.Request.Context(), run); err != nil {
		c.JSON(http.StatusBadRequest, VerifierErrorResponse{Error: err.Error()})
		return
	}

	// Snapshot response fields before StartRun, which launches a background
	// goroutine that mutates the run struct concurrently.
	resp := BenchmarkRunResponse{
		ID:            run.ID,
		Name:          run.Name,
		Description:   run.Description,
		BenchmarkType: run.BenchmarkType,
		ProviderName:  run.ProviderName,
		ModelName:     run.ModelName,
		Status:        run.Status,
		CreatedAt:     run.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if err := runner.StartRun(c.Request.Context(), run.ID); err != nil {
		// CONST-035 §c: "benchmark not found" / "invalid benchmark_type"
		// is bad-input, not a server error. Returning 500 for client-side
		// fault triggers SDK retry storms — route to 400.
		errMsg := err.Error()
		status := http.StatusInternalServerError
		if strings.Contains(errMsg, "not found") ||
			strings.Contains(errMsg, "invalid") ||
			strings.Contains(errMsg, "unsupported") {
			status = http.StatusBadRequest
		}
		c.JSON(status, VerifierErrorResponse{Error: errMsg})
		return
	}

	// StartRun succeeded so the run is now running.
	resp.Status = benchmark.BenchmarkStatusRunning

	c.JSON(http.StatusCreated, resp)
}

// ListBenchmarkResults godoc
// @Summary List benchmark results
// @Description List all benchmark run results, optionally filtered
// @Tags benchmark
// @Produce json
// @Param benchmark_type query string false "Filter by benchmark type"
// @Param provider_name query string false "Filter by provider name"
// @Param status query string false "Filter by status"
// @Success 200 {object} ListBenchmarkRunsResponse
// @Failure 500 {object} VerifierErrorResponse
// @Router /v1/benchmark/results [get]
func (h *BenchmarkHandler) ListBenchmarkResults(c *gin.Context) {

	if !h.checkSystem(c) {
		return
	}
	runner := h.system.GetRunner()
	if runner == nil {
		c.JSON(
			http.StatusInternalServerError,
			VerifierErrorResponse{Error: "benchmark runner not initialized"},
		)
		return
	}

	filter := &benchmark.RunFilter{
		BenchmarkType: benchmark.BenchmarkType(c.Query("benchmark_type")),
		ProviderName:  c.Query("provider_name"),
		Status:        benchmark.BenchmarkStatus(c.Query("status")),
	}

	runs, err := runner.ListRuns(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, VerifierErrorResponse{Error: err.Error()})
		return
	}

	resp := ListBenchmarkRunsResponse{
		Runs:  make([]BenchmarkRunResponse, len(runs)),
		Total: len(runs),
	}
	for i, run := range runs {
		resp.Runs[i] = benchmarkRunToResponse(run)
	}

	c.JSON(http.StatusOK, resp)
}

// GetBenchmarkResult godoc
// @Summary Get specific benchmark result
// @Description Get details of a specific benchmark run by ID
// @Tags benchmark
// @Produce json
// @Param id path string true "Benchmark run ID"
// @Success 200 {object} BenchmarkRunResponse
// @Failure 404 {object} VerifierErrorResponse
// @Failure 500 {object} VerifierErrorResponse
// @Router /v1/benchmark/results/{id} [get]
func (h *BenchmarkHandler) GetBenchmarkResult(c *gin.Context) {

	if !h.checkSystem(c) {
		return
	}
	runner := h.system.GetRunner()
	if runner == nil {
		c.JSON(
			http.StatusInternalServerError,
			VerifierErrorResponse{Error: "benchmark runner not initialized"},
		)
		return
	}

	id := c.Param("id")

	run, err := runner.GetRun(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, VerifierErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, benchmarkRunToResponse(run))
}

// --- helpers ---

func benchmarkRunToResponse(run *benchmark.BenchmarkRun) BenchmarkRunResponse {
	return BenchmarkRunResponse{
		ID:            run.ID,
		Name:          run.Name,
		Description:   run.Description,
		BenchmarkType: run.BenchmarkType,
		ProviderName:  run.ProviderName,
		ModelName:     run.ModelName,
		Status:        run.Status,
		Summary:       run.Summary,
		CreatedAt:     run.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// RegisterBenchmarkRoutes registers benchmark routes
func RegisterBenchmarkRoutes(r *gin.RouterGroup, h *BenchmarkHandler) {
	bm := r.Group("/benchmark")
	{
		bm.POST("/run", h.StartBenchmark)
		bm.GET("/results", h.ListBenchmarkResults)
		bm.GET("/results/:id", h.GetBenchmarkResult)
	}
}
