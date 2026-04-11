package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestNewRAGHandler_Extended tests handler creation with various configs
func TestNewRAGHandler_Extended(t *testing.T) {
	t.Parallel()
	config := RAGHandlerConfig{
		Logger: logrus.New(),
	}
	handler := NewRAGHandler(config)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.logger)
	assert.Nil(t, handler.pipeline)
	assert.Nil(t, handler.advancedRAG)
}

// TestNewRAGHandler_NilLogger tests handler creation with nil logger
func TestNewRAGHandler_NilLogger(t *testing.T) {
	t.Parallel()
	config := RAGHandlerConfig{}
	handler := NewRAGHandler(config)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.logger) // Should create default logger
}

// TestRAGHandler_Health_NilPipeline_Extended tests health endpoint response structure
func TestRAGHandler_Health_NilPipeline_Extended(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/rag/health", nil)

	handler.Health(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "not_configured", response["status"])
	details := response["details"].(map[string]interface{})
	assert.Contains(t, details["error"], "not initialized")
}

// TestRAGHandler_Stats_NilPipeline_Extended tests stats endpoint response structure
func TestRAGHandler_Stats_NilPipeline_Extended(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/rag/stats", nil)

	handler.Stats(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "not initialized")
}

// TestRAGHandler_IngestDocument_NilPipeline tests ingest with nil pipeline
func TestRAGHandler_IngestDocument_NilPipeline(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := IngestDocumentRequest{
		Content: "Test content",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/documents", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.IngestDocument(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_IngestDocument_InvalidJSON tests ingest with invalid JSON
func TestRAGHandler_IngestDocument_InvalidJSON(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/documents", bytes.NewReader([]byte("invalid json")))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.IngestDocument(c)

	// Should return 503 (nil pipeline) rather than 400 since the check happens first
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_IngestDocument_MissingContent tests ingest with missing required field
func TestRAGHandler_IngestDocument_MissingContent(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := map[string]string{"id": "test-id"}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/documents", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.IngestDocument(c)

	// Pipeline check happens first
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_IngestDocuments_NilPipeline tests batch ingest with nil pipeline
func TestRAGHandler_IngestDocuments_NilPipeline(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := IngestDocumentsRequest{
		Documents: []IngestDocumentRequest{
			{Content: "Test 1"},
			{Content: "Test 2"},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/documents/batch", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.IngestDocuments(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_IngestDocuments_EmptyDocuments tests batch ingest with empty documents
func TestRAGHandler_IngestDocuments_EmptyDocuments(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := IngestDocumentsRequest{
		Documents: []IngestDocumentRequest{},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/documents/batch", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.IngestDocuments(c)

	// Pipeline check happens first
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_DeleteDocument_NilPipeline tests delete with nil pipeline
func TestRAGHandler_DeleteDocument_NilPipeline(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-doc-id"}}
	c.Request = httptest.NewRequest("DELETE", "/v1/rag/documents/test-doc-id", nil)

	handler.DeleteDocument(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_DeleteDocument_EmptyID tests delete with empty ID
func TestRAGHandler_DeleteDocument_EmptyID(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: ""}}
	c.Request = httptest.NewRequest("DELETE", "/v1/rag/documents/", nil)

	handler.DeleteDocument(c)

	// Pipeline check happens first
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_Search_NilPipeline tests search with nil pipeline
func TestRAGHandler_Search_NilPipeline(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := SearchRequest{
		Query: "test query",
		TopK:  10,
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/search", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Search(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_Search_DefaultTopK tests search with default TopK
func TestRAGHandler_Search_DefaultTopK(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := SearchRequest{
		Query: "test query",
		// TopK not set, should default to 10
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/search", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Search(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_Search_WithFilters tests search with filters
func TestRAGHandler_Search_WithFilters(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := SearchRequest{
		Query: "test query",
		TopK:  5,
		Filters: map[string]interface{}{
			"source": "docs",
			"type":   "markdown",
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/search", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Search(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_Search_InvalidJSON tests search with invalid JSON
func TestRAGHandler_Search_InvalidJSON(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/search", bytes.NewReader([]byte("invalid")))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Search(c)

	// Pipeline check happens first
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_HybridSearch_NilPipeline tests hybrid search with nil pipeline
func TestRAGHandler_HybridSearch_NilPipeline(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := HybridSearchRequest{
		Query: "test query",
		TopK:  10,
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/search/hybrid", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HybridSearch(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_HybridSearch_WithWeights tests hybrid search with custom weights
func TestRAGHandler_HybridSearch_WithWeights(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := HybridSearchRequest{
		Query:         "test query",
		TopK:          5,
		VectorWeight:  0.7,
		KeywordWeight: 0.3,
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/search/hybrid", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HybridSearch(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_SearchWithExpansion_NilPipeline tests expanded search with nil pipeline
func TestRAGHandler_SearchWithExpansion_NilPipeline(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := ExpandedSearchRequest{
		Query: "test query",
		TopK:  10,
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/search/expanded", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.SearchWithExpansion(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_ReRank_NilAdvancedRAG tests rerank with nil advanced RAG
func TestRAGHandler_ReRank_NilAdvancedRAG(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := ReRankRequest{
		Query:   "test query",
		Results: nil,
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/rerank", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ReRank(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_CompressContext_NilAdvancedRAG tests compression with nil advanced RAG
func TestRAGHandler_CompressContext_NilAdvancedRAG(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := CompressRequest{
		Query:   "test query",
		Results: nil,
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/compress", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CompressContext(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_ExpandQuery_NilAdvancedRAG_Extended tests query expansion response structure
func TestRAGHandler_ExpandQuery_NilAdvancedRAG_Extended(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := ExpandQueryRequest{
		Query: "test query",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/expand", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ExpandQuery(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_ChunkDocument_NilPipeline tests chunking with nil pipeline
func TestRAGHandler_ChunkDocument_NilPipeline(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := ChunkDocumentRequest{
		Content: "Test content for chunking",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/chunk", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChunkDocument(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestRAGHandler_ChunkDocument_WithMetadata tests chunking with metadata
func TestRAGHandler_ChunkDocument_WithMetadata(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	reqBody := ChunkDocumentRequest{
		Content: "Test content for chunking with metadata",
		Metadata: map[string]interface{}{
			"source": "test",
			"author": "unit-test",
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/rag/chunk", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChunkDocument(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestIngestDocumentRequest_Struct tests the struct fields
func TestIngestDocumentRequest_Struct(t *testing.T) {
	t.Parallel()
	req := IngestDocumentRequest{
		ID:       "doc-123",
		Content:  "Test content",
		Metadata: map[string]interface{}{"key": "value"},
		Source:   "unit-test",
	}

	assert.Equal(t, "doc-123", req.ID)
	assert.Equal(t, "Test content", req.Content)
	assert.Equal(t, "unit-test", req.Source)
	assert.NotNil(t, req.Metadata)
}

// TestSearchRequest_Struct tests the struct fields
func TestSearchRequest_Struct(t *testing.T) {
	t.Parallel()
	req := SearchRequest{
		Query:   "search query",
		TopK:    20,
		Filters: map[string]interface{}{"type": "doc"},
	}

	assert.Equal(t, "search query", req.Query)
	assert.Equal(t, 20, req.TopK)
	assert.NotNil(t, req.Filters)
}

// TestHybridSearchRequest_Struct tests the struct fields
func TestHybridSearchRequest_Struct(t *testing.T) {
	t.Parallel()
	req := HybridSearchRequest{
		Query:         "hybrid query",
		TopK:          15,
		VectorWeight:  0.6,
		KeywordWeight: 0.4,
	}

	assert.Equal(t, "hybrid query", req.Query)
	assert.Equal(t, 15, req.TopK)
	assert.Equal(t, 0.6, req.VectorWeight)
	assert.Equal(t, 0.4, req.KeywordWeight)
}

// TestRAGHandlerConfig_Struct tests the config struct
func TestRAGHandlerConfig_Struct(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := RAGHandlerConfig{
		Pipeline: nil,
		Logger:   logger,
	}

	assert.Nil(t, config.Pipeline)
	assert.NotNil(t, config.Logger)
}

// TestRAGHandler_AllEndpoints_InvalidJSON tests all endpoints with invalid JSON
func TestRAGHandler_AllEndpoints_InvalidJSON(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	endpoints := []struct {
		name       string
		method     func(c *gin.Context)
		httpMethod string
		path       string
	}{
		{"IngestDocument", handler.IngestDocument, "POST", "/v1/rag/documents"},
		{"IngestDocuments", handler.IngestDocuments, "POST", "/v1/rag/documents/batch"},
		{"Search", handler.Search, "POST", "/v1/rag/search"},
		{"HybridSearch", handler.HybridSearch, "POST", "/v1/rag/search/hybrid"},
		{"SearchWithExpansion", handler.SearchWithExpansion, "POST", "/v1/rag/search/expanded"},
		{"ReRank", handler.ReRank, "POST", "/v1/rag/rerank"},
		{"CompressContext", handler.CompressContext, "POST", "/v1/rag/compress"},
		{"ExpandQuery", handler.ExpandQuery, "POST", "/v1/rag/expand"},
		{"ChunkDocument", handler.ChunkDocument, "POST", "/v1/rag/chunk"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(ep.httpMethod, ep.path, bytes.NewReader([]byte("{invalid json")))
			c.Request.Header.Set("Content-Type", "application/json")

			ep.method(c)

			// Pipeline/AdvancedRAG nil check happens first, so all return 503
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}
}

// TestRAGHandler_AllEndpoints_EmptyBody tests all POST endpoints with empty body
func TestRAGHandler_AllEndpoints_EmptyBody(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{})

	endpoints := []struct {
		name   string
		method func(c *gin.Context)
		path   string
	}{
		{"IngestDocument", handler.IngestDocument, "/v1/rag/documents"},
		{"IngestDocuments", handler.IngestDocuments, "/v1/rag/documents/batch"},
		{"Search", handler.Search, "/v1/rag/search"},
		{"HybridSearch", handler.HybridSearch, "/v1/rag/search/hybrid"},
		{"SearchWithExpansion", handler.SearchWithExpansion, "/v1/rag/search/expanded"},
		{"ReRank", handler.ReRank, "/v1/rag/rerank"},
		{"CompressContext", handler.CompressContext, "/v1/rag/compress"},
		{"ExpandQuery", handler.ExpandQuery, "/v1/rag/expand"},
		{"ChunkDocument", handler.ChunkDocument, "/v1/rag/chunk"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", ep.path, nil)
			c.Request.Header.Set("Content-Type", "application/json")

			ep.method(c)

			// All return 503 due to nil pipeline/advancedRAG
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}
}
