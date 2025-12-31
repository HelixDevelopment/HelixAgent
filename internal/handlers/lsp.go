package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/services"
)

// LSPHandler handles LSP (Language Server Protocol) requests
type LSPHandler struct {
	lspService *services.LSPManager
	log        *logrus.Logger
}

// NewLSPHandler creates a new LSP handler
func NewLSPHandler(lspService *services.LSPManager, log *logrus.Logger) *LSPHandler {
	return &LSPHandler{
		lspService: lspService,
		log:        log,
	}
}

// ListLSPServers handles GET /v1/lsp/servers
func (h *LSPHandler) ListLSPServers(c *gin.Context) {
	servers, err := h.lspService.ListLSPServers(c.Request.Context())
	if err != nil {
		h.log.WithError(err).Error("Failed to list LSP servers")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, servers)
}

// ExecuteLSPRequest handles POST /v1/lsp/execute
func (h *LSPHandler) ExecuteLSPRequest(c *gin.Context) {
	var req services.UnifiedProtocolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind LSP request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.ServerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serverId is required"})
		return
	}

	if req.ToolName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "toolName (operation) is required"})
		return
	}

	h.log.WithFields(logrus.Fields{
		"serverId":  req.ServerID,
		"operation": req.ToolName,
	}).Info("Executing LSP request")

	// Execute the LSP operation based on tool name
	ctx := c.Request.Context()
	var result interface{}
	var err error

	switch req.ToolName {
	case "completion":
		// Extract parameters for completion
		uri, _ := req.Arguments["uri"].(string)
		line, _ := req.Arguments["line"].(float64)
		character, _ := req.Arguments["character"].(float64)
		text, _ := req.Arguments["text"].(string)

		if uri == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required for completion"})
			return
		}

		// Check if LSP service is available
		if h.lspService == nil {
			result = map[string]interface{}{"message": "LSP service not configured", "uri": uri}
		} else {
			position := services.LSPPosition{Line: int(line), Character: int(character)}
			result, err = h.lspService.GetCompletion(ctx, req.ServerID, text, uri, position)
		}

	case "hover":
		// Extract parameters for hover
		uri, _ := req.Arguments["uri"].(string)
		line, _ := req.Arguments["line"].(float64)
		character, _ := req.Arguments["character"].(float64)

		if uri == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required for hover"})
			return
		}

		if h.lspService == nil {
			result = map[string]interface{}{"message": "LSP service not configured", "uri": uri}
		} else {
			result, err = h.lspService.GetHover(ctx, req.ServerID, uri, int(line), int(character))
		}

	case "definition":
		// Extract parameters for definition
		uri, _ := req.Arguments["uri"].(string)
		line, _ := req.Arguments["line"].(float64)
		character, _ := req.Arguments["character"].(float64)

		if uri == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required for definition"})
			return
		}

		if h.lspService == nil {
			result = map[string]interface{}{"message": "LSP service not configured", "uri": uri}
		} else {
			result, err = h.lspService.GetDefinition(ctx, req.ServerID, uri, int(line), int(character))
		}

	case "references":
		// Extract parameters for references
		uri, _ := req.Arguments["uri"].(string)
		line, _ := req.Arguments["line"].(float64)
		character, _ := req.Arguments["character"].(float64)

		if uri == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required for references"})
			return
		}

		if h.lspService == nil {
			result = map[string]interface{}{"message": "LSP service not configured", "uri": uri}
		} else {
			result, err = h.lspService.GetReferences(ctx, req.ServerID, uri, int(line), int(character))
		}

	case "diagnostics":
		// Get diagnostics for a file
		uri, _ := req.Arguments["uri"].(string)

		if uri == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required for diagnostics"})
			return
		}

		if h.lspService == nil {
			result = map[string]interface{}{"message": "LSP service not configured", "uri": uri}
		} else {
			result, err = h.lspService.GetDiagnostics(ctx, req.ServerID, uri)
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":              "unsupported operation",
			"operation":          req.ToolName,
			"supportedOperations": []string{"completion", "hover", "definition", "references", "diagnostics"},
		})
		return
	}

	if err != nil {
		h.log.WithFields(logrus.Fields{
			"serverId":  req.ServerID,
			"operation": req.ToolName,
			"error":     err.Error(),
		}).Error("LSP operation failed")

		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"error":     err.Error(),
			"serverId":  req.ServerID,
			"operation": req.ToolName,
		})
		return
	}

	h.log.WithFields(logrus.Fields{
		"serverId":  req.ServerID,
		"operation": req.ToolName,
	}).Info("LSP operation completed successfully")

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"serverId":  req.ServerID,
		"operation": req.ToolName,
		"result":    result,
	})
}

// SyncLSPServer handles POST /v1/lsp/servers/:id/sync
func (h *LSPHandler) SyncLSPServer(c *gin.Context) {
	serverID := c.Param("id")

	if err := h.lspService.SyncLSPServer(c.Request.Context(), serverID); err != nil {
		h.log.WithFields(logrus.Fields{
			"serverId": serverID,
			"error":    err.Error(),
		}).Error("Failed to sync LSP server")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    err.Error(),
			"serverId": serverID,
		})
		return
	}

	h.log.WithField("serverId", serverID).Info("LSP server sync completed")

	c.JSON(http.StatusOK, gin.H{
		"message":  "LSP server synced successfully",
		"serverId": serverID,
		"syncedAt": c.Request.Context().Value("time"),
	})
}

// GetLSPStats handles GET /v1/lsp/stats
func (h *LSPHandler) GetLSPStats(c *gin.Context) {
	stats, err := h.lspService.GetLSPStats(c.Request.Context())
	if err != nil {
		h.log.WithError(err).Error("Failed to get LSP stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
