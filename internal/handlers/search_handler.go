// Package handlers provides HTTP handlers
package handlers

import (
	"net/http"
	"time"

	"dev.helix.agent/internal/search"
	"github.com/gin-gonic/gin"
)

// SearchHandler handles search-related HTTP requests
type SearchHandler struct {
	searcher search.Searcher
	indexer  search.Indexer
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(searcher search.Searcher, indexer search.Indexer) *SearchHandler {
	return &SearchHandler{
		searcher: searcher,
		indexer:  indexer,
	}
}

// SemanticSearchRequest represents a semantic search query
type SemanticSearchRequest struct {
	Query       string                 `json:"query" binding:"required"`
	Language    string                 `json:"language,omitempty"`
	FilePattern string                 `json:"file_pattern,omitempty"`
	TopK        int                    `json:"top_k" binding:"max=50"`
	MinScore    float32                `json:"min_score,omitempty"`
	Filters     map[string]interface{} `json:"filters,omitempty"`
}

// SearchResponse represents search results
type SearchResponse struct {
	Results    []search.SearchResult `json:"results"`
	TotalFound int                   `json:"total_found"`
	QueryTime  int64                 `json:"query_time_ms"`
}

// IndexResponse represents indexing result
type IndexResponse struct {
	FilesIndexed  int           `json:"files_indexed"`
	ChunksCreated int           `json:"chunks_created"`
	Duration      time.Duration `json:"duration_ms"`
	Errors        []string      `json:"errors,omitempty"`
}

// SemanticSearch performs semantic code search
func (h *SearchHandler) SemanticSearch(c *gin.Context) {
	var req SemanticSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start := time.Now()

	opts := search.SearchOptions{
		TopK:           req.TopK,
		MinScore:       req.MinScore,
		Filters:        req.Filters,
		IncludeContent: true,
	}

	if opts.TopK == 0 {
		opts.TopK = 10
	}

	// Add language filter if specified
	if req.Language != "" {
		opts.Filters["language"] = req.Language
	}

	results, err := h.searcher.Search(c.Request.Context(), req.Query, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := SearchResponse{
		Results:    results,
		TotalFound: len(results),
		QueryTime:  time.Since(start).Milliseconds(),
	}

	c.JSON(http.StatusOK, resp)
}

// TriggerIndex triggers a full reindex
func (h *SearchHandler) TriggerIndex(c *gin.Context) {
	result, err := h.indexer.Index(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert errors to strings
	errorStrings := make([]string, len(result.Errors))
	for i, err := range result.Errors {
		errorStrings[i] = err.Error()
	}

	resp := IndexResponse{
		FilesIndexed: result.FilesIndexed,
		Duration:     result.Duration,
		Errors:       errorStrings,
	}

	c.JSON(http.StatusOK, resp)
}

// RegisterRoutes registers the search routes
func (h *SearchHandler) RegisterRoutes(router *gin.Engine) {
	v1 := router.Group("/v1/search")
	{
		v1.POST("/semantic", h.SemanticSearch)
		v1.POST("/index", h.TriggerIndex)
	}
}
