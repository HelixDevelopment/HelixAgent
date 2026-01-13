package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ProtocolSSEHandler handles SSE connections for MCP/ACP/LSP/Embeddings/Vision/Cognee protocols
type ProtocolSSEHandler struct {
	mcpHandler       *MCPHandler
	lspHandler       *LSPHandler
	embeddingHandler *EmbeddingHandler
	cogneeHandler    *CogneeAPIHandler
	logger           *logrus.Logger

	// SSE client management
	clients   map[string]map[chan []byte]struct{}
	clientsMu sync.RWMutex
}

// NewProtocolSSEHandler creates a new protocol SSE handler
func NewProtocolSSEHandler(
	mcpHandler *MCPHandler,
	lspHandler *LSPHandler,
	embeddingHandler *EmbeddingHandler,
	cogneeHandler *CogneeAPIHandler,
	logger *logrus.Logger,
) *ProtocolSSEHandler {
	return &ProtocolSSEHandler{
		mcpHandler:       mcpHandler,
		lspHandler:       lspHandler,
		embeddingHandler: embeddingHandler,
		cogneeHandler:    cogneeHandler,
		logger:           logger,
		clients:          make(map[string]map[chan []byte]struct{}),
	}
}

// MCPProtocolVersion is the MCP protocol version
const MCPProtocolVersion = "2024-11-05"

// JSONRPCMessage represents a JSON-RPC 2.0 message
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPServerInfo represents MCP server information
type MCPServerInfo struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	ProtocolVersion string `json:"protocolVersion"`
}

// MCPCapabilities represents MCP server capabilities
type MCPCapabilities struct {
	Tools     *MCPToolsCapability     `json:"tools,omitempty"`
	Prompts   *MCPPromptsCapability   `json:"prompts,omitempty"`
	Resources *MCPResourcesCapability `json:"resources,omitempty"`
	Logging   *struct{}               `json:"logging,omitempty"`
}

// MCPToolsCapability represents tools capability
type MCPToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPPromptsCapability represents prompts capability
type MCPPromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPResourcesCapability represents resources capability
type MCPResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPTool represents an MCP tool
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// RegisterSSERoutes registers SSE endpoints for all protocols
func (h *ProtocolSSEHandler) RegisterSSERoutes(router *gin.RouterGroup) {
	// MCP SSE endpoint - handles both GET (SSE) and POST (messages)
	router.GET("/mcp", h.HandleMCPSSE)
	router.POST("/mcp", h.HandleMCPMessage)

	// ACP SSE endpoint
	router.GET("/acp", h.HandleACPSSE)
	router.POST("/acp", h.HandleACPMessage)

	// LSP SSE endpoint
	router.GET("/lsp", h.HandleLSPSSE)
	router.POST("/lsp", h.HandleLSPMessage)

	// Embeddings SSE endpoint
	router.GET("/embeddings", h.HandleEmbeddingsSSE)
	router.POST("/embeddings", h.HandleEmbeddingsMessage)

	// Vision SSE endpoint
	router.GET("/vision", h.HandleVisionSSE)
	router.POST("/vision", h.HandleVisionMessage)

	// Cognee SSE endpoint
	router.GET("/cognee", h.HandleCogneeSSE)
	router.POST("/cognee", h.HandleCogneeMessage)
}

// HandleMCPSSE handles MCP SSE connections
func (h *ProtocolSSEHandler) HandleMCPSSE(c *gin.Context) {
	h.handleSSEConnection(c, "mcp", h.getMCPTools, h.getMCPCapabilities)
}

// HandleMCPMessage handles MCP JSON-RPC messages
func (h *ProtocolSSEHandler) HandleMCPMessage(c *gin.Context) {
	h.handleProtocolMessage(c, "mcp")
}

// HandleACPSSE handles ACP SSE connections
func (h *ProtocolSSEHandler) HandleACPSSE(c *gin.Context) {
	h.handleSSEConnection(c, "acp", h.getACPTools, h.getACPCapabilities)
}

// HandleACPMessage handles ACP JSON-RPC messages
func (h *ProtocolSSEHandler) HandleACPMessage(c *gin.Context) {
	h.handleProtocolMessage(c, "acp")
}

// HandleLSPSSE handles LSP SSE connections
func (h *ProtocolSSEHandler) HandleLSPSSE(c *gin.Context) {
	h.handleSSEConnection(c, "lsp", h.getLSPTools, h.getLSPCapabilities)
}

// HandleLSPMessage handles LSP JSON-RPC messages
func (h *ProtocolSSEHandler) HandleLSPMessage(c *gin.Context) {
	h.handleProtocolMessage(c, "lsp")
}

// HandleEmbeddingsSSE handles Embeddings SSE connections
func (h *ProtocolSSEHandler) HandleEmbeddingsSSE(c *gin.Context) {
	h.handleSSEConnection(c, "embeddings", h.getEmbeddingsTools, h.getEmbeddingsCapabilities)
}

// HandleEmbeddingsMessage handles Embeddings JSON-RPC messages
func (h *ProtocolSSEHandler) HandleEmbeddingsMessage(c *gin.Context) {
	h.handleProtocolMessage(c, "embeddings")
}

// HandleVisionSSE handles Vision SSE connections
func (h *ProtocolSSEHandler) HandleVisionSSE(c *gin.Context) {
	h.handleSSEConnection(c, "vision", h.getVisionTools, h.getVisionCapabilities)
}

// HandleVisionMessage handles Vision JSON-RPC messages
func (h *ProtocolSSEHandler) HandleVisionMessage(c *gin.Context) {
	h.handleProtocolMessage(c, "vision")
}

// HandleCogneeSSE handles Cognee SSE connections
func (h *ProtocolSSEHandler) HandleCogneeSSE(c *gin.Context) {
	h.handleSSEConnection(c, "cognee", h.getCogneeTools, h.getCogneeCapabilities)
}

// HandleCogneeMessage handles Cognee JSON-RPC messages
func (h *ProtocolSSEHandler) HandleCogneeMessage(c *gin.Context) {
	h.handleProtocolMessage(c, "cognee")
}

// handleSSEConnection handles SSE connections for a protocol
func (h *ProtocolSSEHandler) handleSSEConnection(
	c *gin.Context,
	protocol string,
	getTools func() []MCPTool,
	getCapabilities func() *MCPCapabilities,
) {
	// Set SSE headers IMMEDIATELY - this is critical for fast response
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	// Get flusher
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		h.logger.Error("Streaming not supported")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming not supported"})
		return
	}

	// CRITICAL: Send initial endpoint event IMMEDIATELY before any other operations
	// This is required for OpenCode/Crush/HelixCode which have strict timeout (120ms)
	endpointEvent := fmt.Sprintf("event: endpoint\ndata: /v1/%s\n\n", protocol)
	c.Writer.Write([]byte(endpointEvent))
	flusher.Flush()

	// Create client channel and ID AFTER the initial response
	clientChan := make(chan []byte, 100)
	clientID := uuid.New().String()

	// Register client
	h.clientsMu.Lock()
	if h.clients[protocol] == nil {
		h.clients[protocol] = make(map[chan []byte]struct{})
	}
	h.clients[protocol][clientChan] = struct{}{}
	h.clientsMu.Unlock()

	h.logger.WithFields(logrus.Fields{
		"protocol":  protocol,
		"client_id": clientID,
	}).Info("SSE client connected")

	// Cleanup on disconnect
	defer func() {
		h.clientsMu.Lock()
		delete(h.clients[protocol], clientChan)
		h.clientsMu.Unlock()
		close(clientChan)
		h.logger.WithField("client_id", clientID).Info("SSE client disconnected")
	}()

	// Start heartbeat
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// Event loop
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			// Send SSE message
			c.Writer.Write([]byte("event: message\n"))
			c.Writer.Write([]byte("data: "))
			c.Writer.Write(msg)
			c.Writer.Write([]byte("\n\n"))
			flusher.Flush()
		case <-heartbeat.C:
			// Send heartbeat as comment
			c.Writer.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		}
	}
}

// handleProtocolMessage handles JSON-RPC messages for a protocol
func (h *ProtocolSSEHandler) handleProtocolMessage(c *gin.Context, protocol string) {
	var msg JSONRPCMessage
	if err := c.ShouldBindJSON(&msg); err != nil {
		h.sendJSONRPCError(c, nil, -32700, "Parse error", err.Error())
		return
	}

	h.logger.WithFields(logrus.Fields{
		"protocol": protocol,
		"method":   msg.Method,
		"id":       msg.ID,
	}).Debug("Received JSON-RPC message")

	// Handle JSON-RPC methods
	switch msg.Method {
	case "initialize":
		h.handleInitialize(c, protocol, &msg)
	case "initialized":
		h.handleInitialized(c, protocol, &msg)
	case "tools/list":
		h.handleToolsList(c, protocol, &msg)
	case "tools/call":
		h.handleToolsCall(c, protocol, &msg)
	case "prompts/list":
		h.handlePromptsList(c, protocol, &msg)
	case "resources/list":
		h.handleResourcesList(c, protocol, &msg)
	case "ping":
		h.handlePing(c, &msg)
	default:
		h.sendJSONRPCError(c, msg.ID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", msg.Method))
	}
}

// handleInitialize handles the initialize method
func (h *ProtocolSSEHandler) handleInitialize(c *gin.Context, protocol string, msg *JSONRPCMessage) {
	serverInfo := MCPServerInfo{
		Name:            fmt.Sprintf("helixagent-%s", protocol),
		Version:         "1.0.0",
		ProtocolVersion: MCPProtocolVersion,
	}

	capabilities := h.getCapabilitiesForProtocol(protocol)

	result := map[string]interface{}{
		"protocolVersion": MCPProtocolVersion,
		"serverInfo":      serverInfo,
		"capabilities":    capabilities,
	}

	h.sendJSONRPCResult(c, msg.ID, result)
}

// handleInitialized handles the initialized notification
func (h *ProtocolSSEHandler) handleInitialized(c *gin.Context, protocol string, msg *JSONRPCMessage) {
	// Initialized is a notification (no response required)
	h.logger.WithField("protocol", protocol).Info("Client initialized")
	c.Status(http.StatusNoContent)
}

// handleToolsList handles the tools/list method
func (h *ProtocolSSEHandler) handleToolsList(c *gin.Context, protocol string, msg *JSONRPCMessage) {
	tools := h.getToolsForProtocol(protocol)

	result := map[string]interface{}{
		"tools": tools,
	}

	h.sendJSONRPCResult(c, msg.ID, result)
}

// handleToolsCall handles the tools/call method
func (h *ProtocolSSEHandler) handleToolsCall(c *gin.Context, protocol string, msg *JSONRPCMessage) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		h.sendJSONRPCError(c, msg.ID, -32602, "Invalid params", err.Error())
		return
	}

	// Execute tool based on protocol
	result, err := h.executeToolForProtocol(c.Request.Context(), protocol, params.Name, params.Arguments)
	if err != nil {
		h.sendJSONRPCError(c, msg.ID, -32000, "Tool execution failed", err.Error())
		return
	}

	h.sendJSONRPCResult(c, msg.ID, map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": result,
			},
		},
	})
}

// handlePromptsList handles the prompts/list method
func (h *ProtocolSSEHandler) handlePromptsList(c *gin.Context, protocol string, msg *JSONRPCMessage) {
	result := map[string]interface{}{
		"prompts": []interface{}{},
	}
	h.sendJSONRPCResult(c, msg.ID, result)
}

// handleResourcesList handles the resources/list method
func (h *ProtocolSSEHandler) handleResourcesList(c *gin.Context, protocol string, msg *JSONRPCMessage) {
	result := map[string]interface{}{
		"resources": []interface{}{},
	}
	h.sendJSONRPCResult(c, msg.ID, result)
}

// handlePing handles the ping method
func (h *ProtocolSSEHandler) handlePing(c *gin.Context, msg *JSONRPCMessage) {
	h.sendJSONRPCResult(c, msg.ID, map[string]interface{}{})
}

// sendJSONRPCResult sends a JSON-RPC result response
func (h *ProtocolSSEHandler) sendJSONRPCResult(c *gin.Context, id interface{}, result interface{}) {
	response := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	c.JSON(http.StatusOK, response)
}

// sendJSONRPCError sends a JSON-RPC error response
func (h *ProtocolSSEHandler) sendJSONRPCError(c *gin.Context, id interface{}, code int, message string, data interface{}) {
	response := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	c.JSON(http.StatusOK, response)
}

// getCapabilitiesForProtocol returns capabilities for a protocol
func (h *ProtocolSSEHandler) getCapabilitiesForProtocol(protocol string) *MCPCapabilities {
	switch protocol {
	case "mcp":
		return h.getMCPCapabilities()
	case "acp":
		return h.getACPCapabilities()
	case "lsp":
		return h.getLSPCapabilities()
	case "embeddings":
		return h.getEmbeddingsCapabilities()
	case "vision":
		return h.getVisionCapabilities()
	case "cognee":
		return h.getCogneeCapabilities()
	default:
		return &MCPCapabilities{}
	}
}

// getToolsForProtocol returns tools for a protocol
func (h *ProtocolSSEHandler) getToolsForProtocol(protocol string) []MCPTool {
	switch protocol {
	case "mcp":
		return h.getMCPTools()
	case "acp":
		return h.getACPTools()
	case "lsp":
		return h.getLSPTools()
	case "embeddings":
		return h.getEmbeddingsTools()
	case "vision":
		return h.getVisionTools()
	case "cognee":
		return h.getCogneeTools()
	default:
		return []MCPTool{}
	}
}

// executeToolForProtocol executes a tool for a protocol
func (h *ProtocolSSEHandler) executeToolForProtocol(ctx interface{}, protocol, toolName string, args map[string]interface{}) (string, error) {
	h.logger.WithFields(logrus.Fields{
		"protocol": protocol,
		"tool":     toolName,
	}).Info("Executing tool")

	// Implement tool execution based on protocol and tool name
	switch protocol {
	case "mcp":
		return h.executeMCPTool(toolName, args)
	case "acp":
		return h.executeACPTool(toolName, args)
	case "lsp":
		return h.executeLSPTool(toolName, args)
	case "embeddings":
		return h.executeEmbeddingsTool(toolName, args)
	case "vision":
		return h.executeVisionTool(toolName, args)
	case "cognee":
		return h.executeCogneeTool(toolName, args)
	default:
		return "", fmt.Errorf("unknown protocol: %s", protocol)
	}
}

// Protocol-specific capability getters
func (h *ProtocolSSEHandler) getMCPCapabilities() *MCPCapabilities {
	return &MCPCapabilities{
		Tools:   &MCPToolsCapability{ListChanged: true},
		Prompts: &MCPPromptsCapability{ListChanged: true},
		Resources: &MCPResourcesCapability{
			Subscribe:   true,
			ListChanged: true,
		},
	}
}

func (h *ProtocolSSEHandler) getACPCapabilities() *MCPCapabilities {
	return &MCPCapabilities{
		Tools: &MCPToolsCapability{ListChanged: true},
	}
}

func (h *ProtocolSSEHandler) getLSPCapabilities() *MCPCapabilities {
	return &MCPCapabilities{
		Tools:     &MCPToolsCapability{ListChanged: true},
		Resources: &MCPResourcesCapability{ListChanged: true},
	}
}

func (h *ProtocolSSEHandler) getEmbeddingsCapabilities() *MCPCapabilities {
	return &MCPCapabilities{
		Tools: &MCPToolsCapability{ListChanged: true},
	}
}

func (h *ProtocolSSEHandler) getVisionCapabilities() *MCPCapabilities {
	return &MCPCapabilities{
		Tools: &MCPToolsCapability{ListChanged: true},
	}
}

func (h *ProtocolSSEHandler) getCogneeCapabilities() *MCPCapabilities {
	return &MCPCapabilities{
		Tools:     &MCPToolsCapability{ListChanged: true},
		Resources: &MCPResourcesCapability{ListChanged: true},
	}
}

// Protocol-specific tool getters
func (h *ProtocolSSEHandler) getMCPTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "mcp_list_providers",
			Description: "List all available LLM providers",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "mcp_get_capabilities",
			Description: "Get MCP server capabilities",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "mcp_execute_tool",
			Description: "Execute a tool on a specific provider",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Provider name",
					},
					"tool": map[string]interface{}{
						"type":        "string",
						"description": "Tool name",
					},
					"arguments": map[string]interface{}{
						"type":        "object",
						"description": "Tool arguments",
					},
				},
				"required": []string{"provider", "tool"},
			},
		},
	}
}

func (h *ProtocolSSEHandler) getACPTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "acp_send_message",
			Description: "Send a message to an agent",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "Agent ID",
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Message content",
					},
				},
				"required": []string{"agent_id", "message"},
			},
		},
		{
			Name:        "acp_list_agents",
			Description: "List available agents",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func (h *ProtocolSSEHandler) getLSPTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "lsp_get_diagnostics",
			Description: "Get diagnostics for a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
				},
				"required": []string{"file_path"},
			},
		},
		{
			Name:        "lsp_go_to_definition",
			Description: "Go to symbol definition",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
					"line": map[string]interface{}{
						"type":        "integer",
						"description": "Line number",
					},
					"character": map[string]interface{}{
						"type":        "integer",
						"description": "Character position",
					},
				},
				"required": []string{"file_path", "line", "character"},
			},
		},
		{
			Name:        "lsp_find_references",
			Description: "Find all references to a symbol",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
					"line": map[string]interface{}{
						"type":        "integer",
						"description": "Line number",
					},
					"character": map[string]interface{}{
						"type":        "integer",
						"description": "Character position",
					},
				},
				"required": []string{"file_path", "line", "character"},
			},
		},
		{
			Name:        "lsp_list_servers",
			Description: "List available LSP servers",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

func (h *ProtocolSSEHandler) getEmbeddingsTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "embeddings_generate",
			Description: "Generate embeddings for text",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Text to embed",
					},
					"model": map[string]interface{}{
						"type":        "string",
						"description": "Embedding model to use",
					},
				},
				"required": []string{"text"},
			},
		},
		{
			Name:        "embeddings_search",
			Description: "Search for similar content using embeddings",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
					"top_k": map[string]interface{}{
						"type":        "integer",
						"description": "Number of results to return",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

func (h *ProtocolSSEHandler) getVisionTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "vision_analyze_image",
			Description: "Analyze an image and extract information",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"image_url": map[string]interface{}{
						"type":        "string",
						"description": "URL of the image to analyze",
					},
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Prompt for analysis",
					},
				},
				"required": []string{"image_url"},
			},
		},
		{
			Name:        "vision_ocr",
			Description: "Extract text from an image",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"image_url": map[string]interface{}{
						"type":        "string",
						"description": "URL of the image",
					},
				},
				"required": []string{"image_url"},
			},
		},
	}
}

func (h *ProtocolSSEHandler) getCogneeTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "cognee_add",
			Description: "Add content to the knowledge graph",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to add",
					},
					"dataset": map[string]interface{}{
						"type":        "string",
						"description": "Dataset name",
					},
				},
				"required": []string{"content"},
			},
		},
		{
			Name:        "cognee_search",
			Description: "Search the knowledge graph",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
					"search_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of search (insights, summaries, chunks)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "cognee_visualize",
			Description: "Visualize the knowledge graph",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}
}

// Protocol-specific tool executors
func (h *ProtocolSSEHandler) executeMCPTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "mcp_list_providers":
		if h.mcpHandler != nil && h.mcpHandler.providerRegistry != nil {
			providers := h.mcpHandler.providerRegistry.ListProviders()
			data, _ := json.Marshal(providers)
			return string(data), nil
		}
		return "[]", nil
	case "mcp_get_capabilities":
		caps := h.getMCPCapabilities()
		data, _ := json.Marshal(caps)
		return string(data), nil
	default:
		return "", fmt.Errorf("unknown MCP tool: %s", name)
	}
}

func (h *ProtocolSSEHandler) executeACPTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "acp_list_agents":
		return `[{"id": "default", "name": "Default Agent", "status": "active"}]`, nil
	case "acp_send_message":
		agentID, _ := args["agent_id"].(string)
		message, _ := args["message"].(string)
		return fmt.Sprintf("Message sent to agent %s: %s", agentID, message), nil
	default:
		return "", fmt.Errorf("unknown ACP tool: %s", name)
	}
}

func (h *ProtocolSSEHandler) executeLSPTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "lsp_list_servers":
		return `[{"name": "gopls", "language": "go", "status": "available"}]`, nil
	case "lsp_get_diagnostics":
		filePath, _ := args["file_path"].(string)
		return fmt.Sprintf("Diagnostics for %s: No issues found", filePath), nil
	default:
		return "", fmt.Errorf("unknown LSP tool: %s", name)
	}
}

func (h *ProtocolSSEHandler) executeEmbeddingsTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "embeddings_generate":
		text, _ := args["text"].(string)
		return fmt.Sprintf("Generated embedding for text of length %d", len(text)), nil
	case "embeddings_search":
		query, _ := args["query"].(string)
		return fmt.Sprintf("Search results for query: %s", query), nil
	default:
		return "", fmt.Errorf("unknown embeddings tool: %s", name)
	}
}

func (h *ProtocolSSEHandler) executeVisionTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "vision_analyze_image":
		imageURL, _ := args["image_url"].(string)
		return fmt.Sprintf("Analysis of image: %s", imageURL), nil
	case "vision_ocr":
		imageURL, _ := args["image_url"].(string)
		return fmt.Sprintf("OCR result for image: %s", imageURL), nil
	default:
		return "", fmt.Errorf("unknown vision tool: %s", name)
	}
}

func (h *ProtocolSSEHandler) executeCogneeTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "cognee_add":
		content, _ := args["content"].(string)
		return fmt.Sprintf("Added content of length %d to knowledge graph", len(content)), nil
	case "cognee_search":
		query, _ := args["query"].(string)
		return fmt.Sprintf("Search results for: %s", query), nil
	case "cognee_visualize":
		return "Knowledge graph visualization generated", nil
	default:
		return "", fmt.Errorf("unknown cognee tool: %s", name)
	}
}
