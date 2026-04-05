package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dev.helix.agent/internal/embeddings/models"
	"dev.helix.agent/internal/rag"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEmbeddingModelForHandler implements models.EmbeddingModel for testing
type MockEmbeddingModelForHandler struct {
	dim int
}

func (m *MockEmbeddingModelForHandler) Encode(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		embedding := make([]float32, m.dim)
		for j := range embedding {
			embedding[j] = float32(i+1) * 0.1 / float32(j+1)
		}
		result[i] = embedding
	}
	return result, nil
}

func (m *MockEmbeddingModelForHandler) EncodeSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := m.Encode(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

func (m *MockEmbeddingModelForHandler) Dimensions() int {
	return m.dim
}

func (m *MockEmbeddingModelForHandler) Name() string {
	return "mock-embedding"
}

func (m *MockEmbeddingModelForHandler) MaxTokens() int {
	return 8192
}

func (m *MockEmbeddingModelForHandler) Provider() string {
	return "mock"
}

func (m *MockEmbeddingModelForHandler) Health(ctx context.Context) error {
	return nil
}

func (m *MockEmbeddingModelForHandler) Close() error {
	return nil
}

func createTestRAGHandler() *RAGHandler {
	embeddingConfig := models.RegistryConfig{
		FallbackChain: []string{"mock"},
	}
	embeddingRegistry := models.NewEmbeddingModelRegistry(embeddingConfig)
	_ = embeddingRegistry.Register("mock", &MockEmbeddingModelForHandler{dim: 384})

	pipelineConfig := rag.PipelineConfig{
		VectorDBType:   rag.VectorDBChroma,
		CollectionName: "test_collection",
		EmbeddingModel: "mock",
		ChunkingConfig: rag.DefaultChunkingConfig(),
	}

	pipeline := rag.NewPipeline(pipelineConfig, embeddingRegistry)

	return NewRAGHandler(RAGHandlerConfig{
		Pipeline:          pipeline,
		EmbeddingRegistry: embeddingRegistry,
		Logger:            logrus.New(),
	})
}

func setupRAGRouter(handler *RAGHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	v1 := router.Group("/v1/rag")
	{
		v1.GET("/health", handler.Health)
		v1.GET("/stats", handler.Stats)
		v1.POST("/documents", handler.IngestDocument)
		v1.POST("/documents/batch", handler.IngestDocuments)
		v1.DELETE("/documents/:id", handler.DeleteDocument)
		v1.POST("/search", handler.Search)
		v1.POST("/search/hybrid", handler.HybridSearch)
		v1.POST("/search/expanded", handler.SearchWithExpansion)
		v1.POST("/rerank", handler.ReRank)
		v1.POST("/compress", handler.CompressContext)
		v1.POST("/expand", handler.ExpandQuery)
		v1.POST("/chunk", handler.ChunkDocument)
	}

	return router
}

func TestNewRAGHandler(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.pipeline)
	assert.NotNil(t, handler.advancedRAG)
	assert.NotNil(t, handler.logger)
}

func TestNewRAGHandler_NilPipeline(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{
		Pipeline: nil,
		Logger:   logrus.New(),
	})

	assert.NotNil(t, handler)
	assert.Nil(t, handler.pipeline)
	assert.Nil(t, handler.advancedRAG)
}

func TestRAGHandler_Health(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/rag/health", nil)
	router.ServeHTTP(w, req)

	// Pipeline is not connected, so health returns unhealthy
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", response["status"])
}

func TestRAGHandler_Health_NilPipeline(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{Logger: logrus.New()})
	router := setupRAGRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/rag/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestRAGHandler_ChunkDocument(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := ChunkDocumentRequest{
		Content: "This is a test document. It has multiple sentences. Each sentence will be chunked.",
		Metadata: map[string]interface{}{
			"source": "test",
		},
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/chunk", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "chunks")
	assert.Contains(t, response, "count")
}

func TestRAGHandler_ChunkDocument_MissingContent(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := map[string]interface{}{
		"metadata": map[string]interface{}{"source": "test"},
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/chunk", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRAGHandler_ExpandQuery(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := ExpandQueryRequest{
		Query: "create function",
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/expand", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "query")
	assert.Contains(t, response, "expansions")
	assert.Contains(t, response, "count")
	assert.Equal(t, "create function", response["query"])
}

func TestRAGHandler_ExpandQuery_MissingQuery(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := map[string]interface{}{}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/expand", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRAGHandler_IngestDocument(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := IngestDocumentRequest{
		ID:      "test-doc-1",
		Content: "This is a test document for RAG ingestion.",
		Source:  "test",
		Metadata: map[string]interface{}{
			"category": "test",
		},
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/documents", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Will fail because pipeline is not connected to a real vector DB
	// but should not return 400 (bad request)
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestRAGHandler_IngestDocuments_Batch(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := IngestDocumentsRequest{
		Documents: []IngestDocumentRequest{
			{ID: "doc1", Content: "First document content", Source: "test"},
			{ID: "doc2", Content: "Second document content", Source: "test"},
		},
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/documents/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Will fail because pipeline is not connected
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestRAGHandler_Search(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := SearchRequest{
		Query: "test query",
		TopK:  5,
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Will fail because pipeline is not connected
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestRAGHandler_Search_MissingQuery(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := map[string]interface{}{
		"top_k": 5,
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRAGHandler_HybridSearch(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := HybridSearchRequest{
		Query:         "test query",
		TopK:          5,
		VectorWeight:  0.7,
		KeywordWeight: 0.3,
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/search/hybrid", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Will fail because pipeline is not connected
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestRAGHandler_SearchWithExpansion(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := ExpandedSearchRequest{
		Query: "create function",
		TopK:  5,
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/search/expanded", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Will fail because pipeline is not connected
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestRAGHandler_ReRank(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := ReRankRequest{
		Query: "test query",
		Results: []rag.PipelineSearchResult{
			{Chunk: rag.PipelineChunk{ID: "1", Content: "Test content"}, Score: 0.8},
		},
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/rerank", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "query")
	assert.Contains(t, response, "results")
	assert.Contains(t, response, "count")
}

func TestRAGHandler_CompressContext(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	requestBody := CompressRequest{
		Query: "test query",
		Results: []rag.PipelineSearchResult{
			{
				Chunk: rag.PipelineChunk{
					ID:      "1",
					Content: "This is test content that will be compressed. It has multiple sentences for testing.",
				},
				Score: 0.8,
			},
		},
	}

	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/compress", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "original_length")
	assert.Contains(t, response, "compressed_length")
	assert.Contains(t, response, "content")
	assert.Contains(t, response, "compression_ratio")
}

func TestRAGHandler_DeleteDocument(t *testing.T) {
	t.Parallel()
	handler := createTestRAGHandler()
	router := setupRAGRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/rag/documents/test-doc-1", nil)
	router.ServeHTTP(w, req)

	// Will fail because pipeline is not connected
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestRAGHandler_Stats_NilPipeline(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{Logger: logrus.New()})
	router := setupRAGRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/rag/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestRAGHandler_ExpandQuery_NilAdvancedRAG(t *testing.T) {
	t.Parallel()
	handler := NewRAGHandler(RAGHandlerConfig{Logger: logrus.New()})
	router := setupRAGRouter(handler)

	requestBody := ExpandQueryRequest{Query: "test"}
	body, _ := json.Marshal(requestBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/rag/expand", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
