package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/services"
)

// MCPHandler handles Model Context Protocol (MCP) endpoints
type MCPHandler struct {
	config           *config.MCPConfig
	providerRegistry *services.ProviderRegistry
	mcpManager       *services.MCPManager
	logger           *logrus.Logger
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(providerRegistry *services.ProviderRegistry, cfg *config.MCPConfig) *MCPHandler {
	return &MCPHandler{
		config:           cfg,
		providerRegistry: providerRegistry,
		mcpManager:       services.NewMCPManager(nil, nil, logrus.New()),
		logger:           logrus.New(),
	}
}

// MCPCapabilities returns MCP server capabilities
func (h *MCPHandler) MCPCapabilities(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MCP is not enabled",
		})
		return
	}

	capabilities := map[string]interface{}{
		"version": "1.0.0",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": true,
			},
			"prompts": map[string]interface{}{
				"listChanged": true,
			},
			"resources": map[string]interface{}{
				"listChanged": true,
			},
		},
		"providers":   []string{},
		"mcp_servers": []string{},
	}

	if h.providerRegistry != nil {
		providers := h.providerRegistry.ListProviders()
		capabilities["providers"] = providers
	}

	if h.mcpManager != nil {
		servers, err := h.mcpManager.ListMCPServers(c.Request.Context())
		if err == nil {
			serverNames := make([]string, len(servers))
			for i, server := range servers {
				serverNames[i] = server.Name
			}
			capabilities["mcp_servers"] = serverNames
		}
	}

	c.JSON(http.StatusOK, capabilities)
}

// MCPTools returns available MCP tools
func (h *MCPHandler) MCPTools(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MCP is not enabled",
		})
		return
	}

	tools := []map[string]interface{}{}
	c.JSON(http.StatusOK, gin.H{"tools": tools})
}

// MCPToolsCall executes an MCP tool
func (h *MCPHandler) MCPToolsCall(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MCP is not enabled",
		})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request",
		})
		return
	}

	if h.providerRegistry == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Provider registry not available",
		})
		return
	}

	// Process tool call
	name, ok := req["name"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Tool name is required",
		})
		return
	}

	arguments, _ := req["arguments"].(map[string]interface{})

	// Handle unified namespace if enabled
	if h.config.UnifiedToolNamespace {
		if idx := findUnderscoreIndex(name); idx > 0 {
			providerName := name[:idx]
			toolName := name[idx+1:]

			h.logger.WithFields(logrus.Fields{
				"provider": providerName,
				"tool":     toolName,
			}).Info("Executing tool with unified namespace")

			// Try to execute tool on specific provider
			// This is a simplified implementation
			c.JSON(http.StatusOK, gin.H{
				"result":    fmt.Sprintf("Tool %s executed on provider %s", toolName, providerName),
				"arguments": arguments,
			})
			return
		}
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"error": "Invalid tool format or provider not found",
	})
}

// MCPPrompts returns available MCP prompts
func (h *MCPHandler) MCPPrompts(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MCP is not enabled",
		})
		return
	}

	prompts := []map[string]interface{}{
		{
			"name":        "summarize",
			"description": "Summarize text content",
			"arguments": []map[string]interface{}{
				{
					"name":        "text",
					"description": "Text to summarize",
					"required":    true,
				},
			},
		},
		{
			"name":        "analyze",
			"description": "Analyze content for insights",
			"arguments": []map[string]interface{}{
				{
					"name":        "content",
					"description": "Content to analyze",
					"required":    true,
				},
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{"prompts": prompts})
}

// MCPResources returns available MCP resources
func (h *MCPHandler) MCPResources(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MCP is not enabled",
		})
		return
	}

	resources := []map[string]interface{}{
		{
			"uri":         "superagent://providers",
			"name":        "Provider Information",
			"description": "Information about configured LLM providers",
			"mimeType":    "application/json",
		},
		{
			"uri":         "superagent://models",
			"name":        "Model Metadata",
			"description": "Metadata about available LLM models",
			"mimeType":    "application/json",
		},
	}

	c.JSON(http.StatusOK, gin.H{"resources": resources})
}

// GetMCPManager returns the MCP manager instance
func (h *MCPHandler) GetMCPManager() *services.MCPManager {
	return h.mcpManager
}

// RegisterMCPServer registers an MCP server
func (h *MCPHandler) RegisterMCPServer(config map[string]interface{}) error {
	if h.mcpManager == nil {
		return fmt.Errorf("MCP manager not initialized")
	}

	// This is a simplified implementation
	// In a real implementation, this would connect to the actual MCP server
	h.logger.WithField("config", config).Info("Registering MCP server")

	return nil
}

// findUnderscoreIndex finds the first underscore in a string (not at start or end)
func findUnderscoreIndex(s string) int {
	for i, char := range s {
		if char == '_' && i > 0 && i < len(s)-1 {
			return i
		}
	}
	return -1
}
