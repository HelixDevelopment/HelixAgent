package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/services"
)

// CogneeAPIHandler handles all Cognee API endpoints
type CogneeAPIHandler struct {
	cogneeService *services.CogneeService
	logger        *logrus.Logger
}

// NewCogneeAPIHandler creates a new comprehensive Cognee API handler
func NewCogneeAPIHandler(cogneeService *services.CogneeService, logger *logrus.Logger) *CogneeAPIHandler {
	if logger == nil {
		logger = logrus.New()
	}
	return &CogneeAPIHandler{
		cogneeService: cogneeService,
		logger:        logger,
	}
}

// =====================================================
// HEALTH & STATUS ENDPOINTS
// =====================================================

// Health checks Cognee service health
// GET /v1/cognee/health
func (h *CogneeAPIHandler) Health(c *gin.Context) {
	ctx := c.Request.Context()

	healthy := h.cogneeService.IsHealthy(ctx)
	ready := h.cogneeService.IsReady()

	status := "healthy"
	code := http.StatusOK
	if !healthy {
		status = "unhealthy"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status":  status,
		"healthy": healthy,
		"ready":   ready,
		"config": gin.H{
			"enabled":                  h.cogneeService.GetConfig().Enabled,
			"auto_cognify":             h.cogneeService.GetConfig().AutoCognify,
			"enhance_prompts":          h.cogneeService.GetConfig().EnhancePrompts,
			"temporal_awareness":       h.cogneeService.GetConfig().TemporalAwareness,
			"enable_graph_reasoning":   h.cogneeService.GetConfig().EnableGraphReasoning,
			"enable_code_intelligence": h.cogneeService.GetConfig().EnableCodeIntelligence,
		},
	})
}

// Stats returns Cognee usage statistics
// GET /v1/cognee/stats
func (h *CogneeAPIHandler) Stats(c *gin.Context) {
	stats := h.cogneeService.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"total_memories_stored":     stats.TotalMemoriesStored,
		"total_searches":            stats.TotalSearches,
		"total_cognify_operations":  stats.TotalCognifyOperations,
		"total_insights_queries":    stats.TotalInsightsQueries,
		"total_graph_completions":   stats.TotalGraphCompletions,
		"total_code_processed":      stats.TotalCodeProcessed,
		"total_feedback_received":   stats.TotalFeedbackReceived,
		"average_search_latency_ms": stats.AverageSearchLatency.Milliseconds(),
		"last_activity":             stats.LastActivity,
		"error_count":               stats.ErrorCount,
	})
}

// =====================================================
// MEMORY ENDPOINTS
// =====================================================

// AddMemoryRequest represents a request to add memory
type AddMemoryRequest struct {
	Content     string                 `json:"content" binding:"required"`
	Dataset     string                 `json:"dataset"`
	ContentType string                 `json:"content_type"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// AddMemory adds content to Cognee's memory
// POST /v1/cognee/memory
func (h *CogneeAPIHandler) AddMemory(c *gin.Context) {
	var req AddMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	memory, err := h.cogneeService.AddMemory(ctx, req.Content, req.Dataset, req.ContentType, req.Metadata)
	if err != nil {
		h.logger.WithError(err).Error("Failed to add memory")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"memory": gin.H{
			"id":           memory.ID,
			"vector_id":    memory.VectorID,
			"graph_nodes":  memory.GraphNodes,
			"dataset":      memory.Dataset,
			"content_type": memory.ContentType,
			"created_at":   memory.CreatedAt,
		},
	})
}

// SearchMemoryRequest represents a search request
type SearchMemoryRequest struct {
	Query   string `json:"query" binding:"required"`
	Dataset string `json:"dataset"`
	Limit   int    `json:"limit"`
}

// SearchMemory searches Cognee's memory
// POST /v1/cognee/search
func (h *CogneeAPIHandler) SearchMemory(c *gin.Context) {
	var req SearchMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	result, err := h.cogneeService.SearchMemory(ctx, req.Query, req.Dataset, req.Limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to search memory")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":             result.Query,
		"vector_results":    result.VectorResults,
		"graph_results":     result.GraphResults,
		"insights_results":  result.InsightsResults,
		"graph_completions": result.GraphCompletions,
		"combined_context":  result.CombinedContext,
		"total_results":     result.TotalResults,
		"search_latency_ms": result.SearchLatency.Milliseconds(),
		"relevance_score":   result.RelevanceScore,
	})
}

// =====================================================
// COGNIFY ENDPOINTS
// =====================================================

// CognifyRequest represents a cognify request
type CognifyRequest struct {
	Datasets []string `json:"datasets"`
}

// Cognify processes data into knowledge graphs
// POST /v1/cognee/cognify
func (h *CogneeAPIHandler) Cognify(c *gin.Context) {
	var req CognifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Datasets = []string{} // Use default
	}

	ctx := c.Request.Context()
	if err := h.cogneeService.Cognify(ctx, req.Datasets); err != nil {
		h.logger.WithError(err).Error("Failed to cognify")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Cognify operation completed successfully",
		"datasets": req.Datasets,
	})
}

// =====================================================
// INSIGHTS & GRAPH ENDPOINTS
// =====================================================

// InsightsRequest represents an insights request
type InsightsRequest struct {
	Query    string   `json:"query" binding:"required"`
	Datasets []string `json:"datasets"`
	Limit    int      `json:"limit"`
}

// GetInsights retrieves insights using graph reasoning
// POST /v1/cognee/insights
func (h *CogneeAPIHandler) GetInsights(c *gin.Context) {
	var req InsightsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	insights, err := h.cogneeService.GetInsights(ctx, req.Query, req.Datasets, req.Limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get insights")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":    req.Query,
		"insights": insights,
		"count":    len(insights),
	})
}

// GraphCompletionRequest represents a graph completion request
type GraphCompletionRequest struct {
	Query    string   `json:"query" binding:"required"`
	Datasets []string `json:"datasets"`
	Limit    int      `json:"limit"`
}

// GetGraphCompletion performs LLM-powered graph completion
// POST /v1/cognee/graph/complete
func (h *CogneeAPIHandler) GetGraphCompletion(c *gin.Context) {
	var req GraphCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	completions, err := h.cogneeService.GetGraphCompletion(ctx, req.Query, req.Datasets, req.Limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get graph completion")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":       req.Query,
		"completions": completions,
		"count":       len(completions),
	})
}

// =====================================================
// CODE INTELLIGENCE ENDPOINTS
// =====================================================

// ProcessCodeRequest represents a code processing request
type ProcessCodeRequest struct {
	Code     string `json:"code" binding:"required"`
	Language string `json:"language"`
	Dataset  string `json:"dataset"`
}

// ProcessCode indexes code through Cognee's code pipeline
// POST /v1/cognee/code
func (h *CogneeAPIHandler) ProcessCode(c *gin.Context) {
	var req ProcessCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	result, err := h.cogneeService.ProcessCode(ctx, req.Code, req.Language, req.Dataset)
	if err != nil {
		h.logger.WithError(err).Error("Failed to process code")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"language":    result.Language,
		"summary":     result.Summary,
		"entities":    result.Entities,
		"connections": result.Connections,
	})
}

// =====================================================
// DATASET MANAGEMENT ENDPOINTS
// =====================================================

// CreateDatasetRequest represents a dataset creation request
type CreateDatasetRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// CreateDataset creates a new dataset
// POST /v1/cognee/datasets
func (h *CogneeAPIHandler) CreateDataset(c *gin.Context) {
	var req CreateDatasetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	if err := h.cogneeService.CreateDataset(ctx, req.Name, req.Description, req.Metadata); err != nil {
		h.logger.WithError(err).Error("Failed to create dataset")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Dataset created successfully",
		"name":    req.Name,
	})
}

// ListDatasets retrieves all datasets
// GET /v1/cognee/datasets
func (h *CogneeAPIHandler) ListDatasets(c *gin.Context) {
	ctx := c.Request.Context()
	datasets, err := h.cogneeService.ListDatasets(ctx)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list datasets")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"datasets": datasets,
		"count":    len(datasets),
	})
}

// DeleteDataset removes a dataset
// DELETE /v1/cognee/datasets/:name
func (h *CogneeAPIHandler) DeleteDataset(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dataset name is required"})
		return
	}

	ctx := c.Request.Context()
	if err := h.cogneeService.DeleteDataset(ctx, name); err != nil {
		h.logger.WithError(err).Error("Failed to delete dataset")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Dataset deleted successfully",
		"name":    name,
	})
}

// =====================================================
// GRAPH VISUALIZATION ENDPOINTS
// =====================================================

// VisualizeGraph retrieves graph visualization data
// GET /v1/cognee/graph/visualize
func (h *CogneeAPIHandler) VisualizeGraph(c *gin.Context) {
	dataset := c.Query("dataset")
	format := c.Query("format")

	ctx := c.Request.Context()
	graph, err := h.cogneeService.VisualizeGraph(ctx, dataset, format)
	if err != nil {
		h.logger.WithError(err).Error("Failed to visualize graph")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dataset": dataset,
		"format":  format,
		"graph":   graph,
	})
}

// =====================================================
// FEEDBACK ENDPOINTS
// =====================================================

// FeedbackRequest represents a feedback request
type FeedbackRequest struct {
	QueryID   string  `json:"query_id" binding:"required"`
	Query     string  `json:"query" binding:"required"`
	Response  string  `json:"response" binding:"required"`
	Relevance float64 `json:"relevance"`
	Approved  bool    `json:"approved"`
}

// ProvideFeedback records user feedback for self-improvement
// POST /v1/cognee/feedback
func (h *CogneeAPIHandler) ProvideFeedback(c *gin.Context) {
	var req FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	if err := h.cogneeService.ProvideFeedback(ctx, req.QueryID, req.Query, req.Response, req.Relevance, req.Approved); err != nil {
		h.logger.WithError(err).Error("Failed to record feedback")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Feedback recorded successfully",
		"query_id": req.QueryID,
		"approved": req.Approved,
	})
}

// =====================================================
// CONFIGURATION ENDPOINTS
// =====================================================

// GetConfig returns the current Cognee configuration
// GET /v1/cognee/config
func (h *CogneeAPIHandler) GetConfig(c *gin.Context) {
	config := h.cogneeService.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"enabled":                  config.Enabled,
		"base_url":                 config.BaseURL,
		"auto_cognify":             config.AutoCognify,
		"enhance_prompts":          config.EnhancePrompts,
		"store_responses":          config.StoreResponses,
		"max_context_size":         config.MaxContextSize,
		"relevance_threshold":      config.RelevanceThreshold,
		"temporal_awareness":       config.TemporalAwareness,
		"enable_feedback_loop":     config.EnableFeedbackLoop,
		"enable_graph_reasoning":   config.EnableGraphReasoning,
		"enable_code_intelligence": config.EnableCodeIntelligence,
		"default_search_limit":     config.DefaultSearchLimit,
		"default_dataset":          config.DefaultDataset,
		"search_types":             config.SearchTypes,
		"cache_enabled":            config.CacheEnabled,
		"max_concurrency":          config.MaxConcurrency,
	})
}

// EnsureRunning starts Cognee containers if not running
// POST /v1/cognee/start
func (h *CogneeAPIHandler) EnsureRunning(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.cogneeService.EnsureRunning(ctx); err != nil {
		h.logger.WithError(err).Error("Failed to start Cognee")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cognee services started successfully",
		"healthy": h.cogneeService.IsHealthy(ctx),
		"ready":   h.cogneeService.IsReady(),
	})
}

// =====================================================
// ROUTE REGISTRATION
// =====================================================

// RegisterRoutes registers all Cognee API routes
func (h *CogneeAPIHandler) RegisterRoutes(router *gin.RouterGroup) {
	cognee := router.Group("/cognee")
	{
		// Health & Status
		cognee.GET("/health", h.Health)
		cognee.GET("/stats", h.Stats)
		cognee.GET("/config", h.GetConfig)
		cognee.POST("/start", h.EnsureRunning)

		// Memory Operations
		cognee.POST("/memory", h.AddMemory)
		cognee.POST("/search", h.SearchMemory)

		// Cognify
		cognee.POST("/cognify", h.Cognify)

		// Insights & Graph
		cognee.POST("/insights", h.GetInsights)
		cognee.POST("/graph/complete", h.GetGraphCompletion)
		cognee.GET("/graph/visualize", h.VisualizeGraph)

		// Code Intelligence
		cognee.POST("/code", h.ProcessCode)

		// Dataset Management
		cognee.POST("/datasets", h.CreateDataset)
		cognee.GET("/datasets", h.ListDatasets)
		cognee.DELETE("/datasets/:name", h.DeleteDataset)

		// Feedback
		cognee.POST("/feedback", h.ProvideFeedback)
	}
}

// =====================================================
// HELPER FUNCTIONS
// =====================================================

func getIntParam(c *gin.Context, key string, defaultVal int) int {
	if val := c.Query(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getFloatParam(c *gin.Context, key string, defaultVal float64) float64 {
	if val := c.Query(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}
