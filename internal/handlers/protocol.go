package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/services"
)

// ProtocolHandler handles unified protocol requests
type ProtocolHandler struct {
	protocolService services.ProtocolManagerInterface
	log             *logrus.Logger
}

// NewProtocolHandler creates a new protocol handler
func NewProtocolHandler(protocolService services.ProtocolManagerInterface, log *logrus.Logger) *ProtocolHandler {
	return &ProtocolHandler{
		protocolService: protocolService,
		log:             log,
	}
}

// ExecuteProtocolRequest handles POST /v1/protocols/execute
func (h *ProtocolHandler) ExecuteProtocolRequest(c *gin.Context) {
	var req services.UnifiedProtocolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind protocol request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.protocolService.ExecuteRequest(c.Request.Context(), req)
	if err != nil {
		h.log.WithError(err).Error("Failed to execute protocol request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListProtocolServers handles GET /v1/protocols/servers
func (h *ProtocolHandler) ListProtocolServers(c *gin.Context) {
	servers, err := h.protocolService.ListServers(c.Request.Context())
	if err != nil {
		h.log.WithError(err).Error("Failed to list protocol servers")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, servers)
}

// GetProtocolMetrics handles GET /v1/protocols/metrics
func (h *ProtocolHandler) GetProtocolMetrics(c *gin.Context) {
	metrics, err := h.protocolService.GetMetrics(c.Request.Context())
	if err != nil {
		h.log.WithError(err).Error("Failed to get protocol metrics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// RefreshProtocolServers handles POST /v1/protocols/refresh
func (h *ProtocolHandler) RefreshProtocolServers(c *gin.Context) {
	err := h.protocolService.RefreshAll(c.Request.Context())
	if err != nil {
		h.log.WithError(err).Error("Failed to refresh protocol servers")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Protocol servers refreshed successfully"})
}

// ConfigureProtocols handles POST /v1/protocols/configure
func (h *ProtocolHandler) ConfigureProtocols(c *gin.Context) {
	var config map[string]interface{}
	if err := c.ShouldBindJSON(&config); err != nil {
		h.log.WithError(err).Error("Failed to bind protocol configuration")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.protocolService.ConfigureProtocols(c.Request.Context(), config)
	if err != nil {
		h.log.WithError(err).Error("Failed to configure protocols")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Protocols configured successfully"})
}
