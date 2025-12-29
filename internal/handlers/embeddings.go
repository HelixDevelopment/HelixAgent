package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/services"
)

// EmbeddingHandler handles embedding and vector search HTTP endpoints
type EmbeddingHandler struct {
	embeddingManager *services.EmbeddingManager
	log              *logrus.Logger
}

// NewEmbeddingHandler creates a new embedding handler
func NewEmbeddingHandler(embeddingManager *services.EmbeddingManager, log *logrus.Logger) *EmbeddingHandler {
	return &EmbeddingHandler{
		embeddingManager: embeddingManager,
		log:              log,
	}
}

// GenerateEmbeddings handles POST /v1/embeddings/generate
func (h *EmbeddingHandler) GenerateEmbeddings(c *gin.Context) {
	var req services.EmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind embedding request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.embeddingManager.GenerateEmbeddings(c.Request.Context(), req)
	if err != nil {
		h.log.WithError(err).Error("Failed to generate embeddings")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// VectorSearch handles POST /v1/embeddings/search
func (h *EmbeddingHandler) VectorSearch(c *gin.Context) {
	var req services.VectorSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind vector search request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.embeddingManager.VectorSearch(c.Request.Context(), req)
	if err != nil {
		h.log.WithError(err).Error("Failed to perform vector search")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// IndexDocument handles POST /v1/embeddings/index
func (h *EmbeddingHandler) IndexDocument(c *gin.Context) {
	var req struct {
		ID       string                 `json:"id"`
		Title    string                 `json:"title"`
		Content  string                 `json:"content"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind index request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.embeddingManager.IndexDocument(c.Request.Context(), req.ID, req.Title, req.Content, req.Metadata)
	if err != nil {
		h.log.WithError(err).Error("Failed to index document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Document indexed successfully"})
}

// BatchIndexDocuments handles POST /v1/embeddings/batch-index
func (h *EmbeddingHandler) BatchIndexDocuments(c *gin.Context) {
	var req struct {
		Documents []map[string]interface{} `json:"documents"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind batch index request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.embeddingManager.BatchIndexDocuments(c.Request.Context(), req.Documents)
	if err != nil {
		h.log.WithError(err).Error("Failed to batch index documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Batch indexing completed"})
}

// GetEmbeddingStats handles GET /v1/embeddings/stats
func (h *EmbeddingHandler) GetEmbeddingStats(c *gin.Context) {
	stats, err := h.embeddingManager.GetEmbeddingStats(c.Request.Context())
	if err != nil {
		h.log.WithError(err).Error("Failed to get embedding stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ConfigureProvider handles PUT /v1/embeddings/provider
func (h *EmbeddingHandler) ConfigureProvider(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind provider configuration request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.embeddingManager.ConfigureVectorProvider(c.Request.Context(), req.Provider)
	if err != nil {
		h.log.WithError(err).Error("Failed to configure vector provider")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vector provider configured successfully", "provider": req.Provider})
}

// SimilaritySearch handles POST /v1/embeddings/similarity
func (h *EmbeddingHandler) SimilaritySearch(c *gin.Context) {
	var req services.VectorSearchRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind similarity search request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.embeddingManager.VectorSearch(c.Request.Context(), req)
	if err != nil {
		h.log.WithError(err).Error("Failed to perform similarity search")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
