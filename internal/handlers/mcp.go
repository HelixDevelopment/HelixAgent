package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

// MCPHandler implements Model Context Protocol endpoints
type MCPHandler struct {
	providerRegistry *services.ProviderRegistry
	mcpManager       *services.MCPManager
	config           *config.MCPConfig
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(registry *services.ProviderRegistry, config *config.MCPConfig) *MCPHandler {
	return &MCPHandler{
		providerRegistry: registry,
		mcpManager:       services.NewMCPManager(),
		config:           config,
	}
}

// MCPCapabilities returns MCP capabilities from all providers and MCP servers
func (h *MCPHandler) MCPCapabilities(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MCP is not enabled",
		})
		return
	}

	capabilities := make(map[string]interface{})

	// Get capabilities from LLM providers
	providers := h.providerRegistry.ListProviders()
	for _, providerName := range providers {
		if provider, err := h.providerRegistry.GetProvider(providerName); err == nil {
			providerCaps := provider.GetCapabilities()
			if h.config.ExposeAllTools {
				// Get tools from provider
				tools := h.getProviderTools(providerName)
				capabilities[providerName] = map[string]interface{}{
					"capabilities": providerCaps,
					"tools":        tools,
				}
			} else {
				capabilities[providerName] = providerCaps
			}
		}
	}

	// Get capabilities from MCP servers
	mcpTools := h.mcpManager.ListTools()
	mcpCapabilities := make(map[string]interface{})
	for _, tool := range mcpTools {
		mcpCapabilities[tool.Name] = map[string]interface{}{
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
			"server":      tool.Server.Name,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"version": "2024-11-05",
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
		"providers":   capabilities,
		"mcp_servers": mcpCapabilities,
	})
}

// MCPTools lists all available tools from all providers and MCP servers
func (h *MCPHandler) MCPTools(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MCP is not enabled",
		})
		return
	}

	var allTools []interface{}

	// Get tools from LLM providers
	providers := h.providerRegistry.ListProviders()
	for _, providerName := range providers {
		tools := h.getProviderTools(providerName)
		if h.config.UnifiedToolNamespace {
			// Prefix tool names with provider
			for _, tool := range tools {
				if toolMap, ok := tool.(map[string]interface{}); ok {
					if name, ok := toolMap["name"].(string); ok {
						toolMap["name"] = fmt.Sprintf("%s_%s", providerName, name)
					}
				}
			}
		}
		allTools = append(allTools, tools...)
	}

	// Get tools from MCP servers
	mcpTools := h.mcpManager.ListTools()
	for _, tool := range mcpTools {
		toolInfo := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
			"type":        "mcp_server",
			"server":      tool.Server.Name,
		}

		if h.config.UnifiedToolNamespace {
			toolInfo["name"] = fmt.Sprintf("mcp_%s_%s", tool.Server.Name, tool.Name)
		}

		allTools = append(allTools, toolInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": allTools,
	})
}

// MCPToolsCall executes a tool call
func (h *MCPHandler) MCPToolsCall(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MCP is not enabled",
		})
		return
	}

	var request struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Check if this is an MCP server tool call
	if _, err := h.mcpManager.GetTool(request.Name); err == nil {
		// Execute MCP server tool
		result, err := h.mcpManager.CallTool(c.Request.Context(), request.Name, request.Arguments)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("MCP tool call failed: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": fmt.Sprintf("%v", result),
				},
			},
			"isError":  false,
			"toolType": "mcp_server",
		})
		return
	}

	// Extract provider name if using unified namespace
	providerName := "superagent-ensemble"
	toolName := request.Name

	if h.config.UnifiedToolNamespace {
		// Parse tool name to extract provider
		if strings.Contains(request.Name, "_") {
			parts := strings.SplitN(request.Name, "_", 2)
			if len(parts) == 2 {
				providerName = parts[0]
				toolName = parts[1]
			}
		}
	}

	// Create LLM request for tool call
	req := &models.LLMRequest{
		ID:     fmt.Sprintf("mcp-%d", time.Now().Unix()),
		Prompt: fmt.Sprintf("Execute tool: %s with args: %v", toolName, request.Arguments),
		Messages: []models.Message{
			{
				Role:    "user",
				Content: fmt.Sprintf("Please execute the tool '%s' with these arguments: %v", toolName, request.Arguments),
				ToolCalls: map[string]interface{}{
					"name":      toolName,
					"arguments": request.Arguments,
				},
			},
		},
		ModelParams: models.ModelParameters{
			Model: providerName,
		},
		Status: "pending",
	}

	// Execute tool call using ensemble
	ensembleService := h.providerRegistry.GetEnsembleService()
	if ensembleService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ensemble service not available",
		})
		return
	}

	result, err := ensembleService.RunEnsemble(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Tool call failed: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": result.Selected.Content,
			},
		},
		"isError":  false,
		"toolType": "llm_provider",
	})
}

// MCPPrompts lists available prompts
func (h *MCPHandler) MCPPrompts(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MCP is not enabled",
		})
		return
	}

	// Return standard prompts that all providers can use
	prompts := []interface{}{
		map[string]interface{}{
			"name":        "summarize",
			"description": "Summarize the given content",
			"arguments": []interface{}{
				map[string]interface{}{
					"name":        "content",
					"description": "Content to summarize",
					"required":    true,
				},
			},
		},
		map[string]interface{}{
			"name":        "analyze",
			"description": "Analyze the given content",
			"arguments": []interface{}{
				map[string]interface{}{
					"name":        "content",
					"description": "Content to analyze",
					"required":    true,
				},
				map[string]interface{}{
					"name":        "aspect",
					"description": "Aspect to focus on",
					"required":    false,
				},
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"prompts": prompts,
	})
}

// MCPResources lists available resources
func (h *MCPHandler) MCPResources(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "MCP is not enabled",
		})
		return
	}

	resources := []interface{}{
		map[string]interface{}{
			"uri":         "superagent://providers",
			"name":        "Available Providers",
			"description": "List of all available LLM providers",
			"mimeType":    "application/json",
		},
		map[string]interface{}{
			"uri":         "superagent://models",
			"name":        "Available Models",
			"description": "List of all available models from all providers",
			"mimeType":    "application/json",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"resources": resources,
	})
}

// RegisterMCPServer registers an MCP server with the handler
func (h *MCPHandler) RegisterMCPServer(serverConfig map[string]interface{}) error {
	return h.mcpManager.RegisterServer(serverConfig)
}

// GetMCPManager returns the MCP manager for external access
func (h *MCPHandler) GetMCPManager() *services.MCPManager {
	return h.mcpManager
}

// Helper functions
func (h *MCPHandler) getProviderTools(providerName string) []interface{} {
	// Get provider capabilities and return appropriate tools
	if provider, err := h.providerRegistry.GetProvider(providerName); err == nil {
		caps := provider.GetCapabilities()
		tools := []interface{}{}

		// Add tools based on capabilities
		if caps.SupportsTools {
			tools = append(tools, map[string]interface{}{
				"name":        "execute_code",
				"description": "Execute code in various languages",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"language": map[string]interface{}{
							"type":        "string",
							"description": "Programming language",
						},
						"code": map[string]interface{}{
							"type":        "string",
							"description": "Code to execute",
						},
					},
					"required": []string{"language", "code"},
				},
			})
		}

		if caps.SupportsSearch {
			tools = append(tools, map[string]interface{}{
				"name":        "web_search",
				"description": "Search the web for information",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query",
						},
					},
					"required": []string{"query"},
				},
			})
		}

		if caps.SupportsReasoning {
			tools = append(tools, map[string]interface{}{
				"name":        "reasoning",
				"description": "Step-by-step reasoning and analysis",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"problem": map[string]interface{}{
							"type":        "string",
							"description": "Problem to analyze",
						},
						"method": map[string]interface{}{
							"type":        "string",
							"description": "Reasoning method (chain_of_thought, step_by_step, etc.)",
						},
					},
					"required": []string{"problem"},
				},
			})
		}

		return tools
	}
	return []interface{}{}
}

func findUnderscoreIndex(s string) int {
	for i, char := range s {
		if char == '_' && i > 0 && i < len(s)-1 {
			return i
		}
	}
	return -1
}
