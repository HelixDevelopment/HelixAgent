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

// TestNewProtocolSSEHandler tests handler creation
func TestNewProtocolSSEHandler(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.clients)
	assert.Equal(t, logger, handler.logger)
}

// TestProtocolSSEHandler_RegisterSSERoutes tests route registration
func TestProtocolSSEHandler_RegisterSSERoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	router := gin.New()
	group := router.Group("/v1")
	handler.RegisterSSERoutes(group)

	// Verify routes are registered by checking the router
	routes := router.Routes()

	expectedRoutes := map[string]string{
		"/v1/mcp":        "GET",
		"/v1/acp":        "GET",
		"/v1/lsp":        "GET",
		"/v1/embeddings": "GET",
		"/v1/vision":     "GET",
		"/v1/cognee":     "GET",
	}

	// Check GET routes
	for path, method := range expectedRoutes {
		found := false
		for _, route := range routes {
			if route.Path == path && route.Method == method {
				found = true
				break
			}
		}
		assert.True(t, found, "Route %s %s should be registered", method, path)
	}

	// Check POST routes
	expectedPOSTRoutes := []string{"/v1/mcp", "/v1/acp", "/v1/lsp", "/v1/embeddings", "/v1/vision", "/v1/cognee"}
	for _, path := range expectedPOSTRoutes {
		found := false
		for _, route := range routes {
			if route.Path == path && route.Method == "POST" {
				found = true
				break
			}
		}
		assert.True(t, found, "Route POST %s should be registered", path)
	}
}

// TestProtocolSSEHandler_HandleMCPMessage_Initialize tests the initialize method
func TestProtocolSSEHandler_HandleMCPMessage_Initialize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMCPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Equal(t, float64(1), response.ID)
	assert.Nil(t, response.Error)

	result := response.Result.(map[string]interface{})
	assert.Equal(t, MCPProtocolVersion, result["protocolVersion"])
	assert.NotNil(t, result["serverInfo"])
	assert.NotNil(t, result["capabilities"])

	serverInfo := result["serverInfo"].(map[string]interface{})
	assert.Equal(t, "helixagent-mcp", serverInfo["name"])
	assert.Equal(t, "1.0.0", serverInfo["version"])
}

// TestProtocolSSEHandler_HandleMCPMessage_ToolsList tests the tools/list method
func TestProtocolSSEHandler_HandleMCPMessage_ToolsList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMCPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Nil(t, response.Error)

	result := response.Result.(map[string]interface{})
	tools := result["tools"].([]interface{})
	assert.GreaterOrEqual(t, len(tools), 3, "MCP should have at least 3 tools")
}

// TestProtocolSSEHandler_HandleMCPMessage_ToolsCall tests the tools/call method
func TestProtocolSSEHandler_HandleMCPMessage_ToolsCall(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "mcp_get_capabilities", "arguments": {}}`),
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMCPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Nil(t, response.Error)
	assert.NotNil(t, response.Result)
}

// TestProtocolSSEHandler_HandleMCPMessage_Ping tests the ping method
func TestProtocolSSEHandler_HandleMCPMessage_Ping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "ping",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMCPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Nil(t, response.Error)
}

// TestProtocolSSEHandler_HandleMCPMessage_UnknownMethod tests unknown method error
func TestProtocolSSEHandler_HandleMCPMessage_UnknownMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "unknown/method",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMCPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32601, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Method not found")
}

// TestProtocolSSEHandler_HandleMCPMessage_ParseError tests JSON parse error
func TestProtocolSSEHandler_HandleMCPMessage_ParseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMCPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32700, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Parse error")
}

// TestProtocolSSEHandler_HandleACPMessage_Initialize tests ACP initialize
func TestProtocolSSEHandler_HandleACPMessage_Initialize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleACPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	result := response.Result.(map[string]interface{})
	serverInfo := result["serverInfo"].(map[string]interface{})
	assert.Equal(t, "helixagent-acp", serverInfo["name"])
}

// TestProtocolSSEHandler_HandleLSPMessage_Initialize tests LSP initialize
func TestProtocolSSEHandler_HandleLSPMessage_Initialize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/lsp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleLSPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	result := response.Result.(map[string]interface{})
	serverInfo := result["serverInfo"].(map[string]interface{})
	assert.Equal(t, "helixagent-lsp", serverInfo["name"])
}

// TestProtocolSSEHandler_HandleEmbeddingsMessage_Initialize tests Embeddings initialize
func TestProtocolSSEHandler_HandleEmbeddingsMessage_Initialize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/embeddings", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleEmbeddingsMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	result := response.Result.(map[string]interface{})
	serverInfo := result["serverInfo"].(map[string]interface{})
	assert.Equal(t, "helixagent-embeddings", serverInfo["name"])
}

// TestProtocolSSEHandler_HandleVisionMessage_Initialize tests Vision initialize
func TestProtocolSSEHandler_HandleVisionMessage_Initialize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/vision", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleVisionMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	result := response.Result.(map[string]interface{})
	serverInfo := result["serverInfo"].(map[string]interface{})
	assert.Equal(t, "helixagent-vision", serverInfo["name"])
}

// TestProtocolSSEHandler_HandleCogneeMessage_Initialize tests Cognee initialize
func TestProtocolSSEHandler_HandleCogneeMessage_Initialize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/cognee", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleCogneeMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	result := response.Result.(map[string]interface{})
	serverInfo := result["serverInfo"].(map[string]interface{})
	assert.Equal(t, "helixagent-cognee", serverInfo["name"])
}

// TestProtocolSSEHandler_GetMCPCapabilities tests MCP capabilities
func TestProtocolSSEHandler_GetMCPCapabilities(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	caps := handler.getMCPCapabilities()

	assert.NotNil(t, caps)
	assert.NotNil(t, caps.Tools)
	assert.NotNil(t, caps.Prompts)
	assert.NotNil(t, caps.Resources)
	assert.True(t, caps.Tools.ListChanged)
	assert.True(t, caps.Resources.Subscribe)
}

// TestProtocolSSEHandler_GetACPCapabilities tests ACP capabilities
func TestProtocolSSEHandler_GetACPCapabilities(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	caps := handler.getACPCapabilities()

	assert.NotNil(t, caps)
	assert.NotNil(t, caps.Tools)
	assert.True(t, caps.Tools.ListChanged)
}

// TestProtocolSSEHandler_GetLSPCapabilities tests LSP capabilities
func TestProtocolSSEHandler_GetLSPCapabilities(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	caps := handler.getLSPCapabilities()

	assert.NotNil(t, caps)
	assert.NotNil(t, caps.Tools)
	assert.NotNil(t, caps.Resources)
}

// TestProtocolSSEHandler_GetMCPTools tests MCP tools list
func TestProtocolSSEHandler_GetMCPTools(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	tools := handler.getMCPTools()

	assert.GreaterOrEqual(t, len(tools), 3)

	// Check for expected tools
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["mcp_list_providers"])
	assert.True(t, toolNames["mcp_get_capabilities"])
	assert.True(t, toolNames["mcp_execute_tool"])
}

// TestProtocolSSEHandler_GetACPTools tests ACP tools list
func TestProtocolSSEHandler_GetACPTools(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	tools := handler.getACPTools()

	assert.GreaterOrEqual(t, len(tools), 2)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["acp_send_message"])
	assert.True(t, toolNames["acp_list_agents"])
}

// TestProtocolSSEHandler_GetLSPTools tests LSP tools list
func TestProtocolSSEHandler_GetLSPTools(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	tools := handler.getLSPTools()

	assert.GreaterOrEqual(t, len(tools), 4)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["lsp_get_diagnostics"])
	assert.True(t, toolNames["lsp_go_to_definition"])
	assert.True(t, toolNames["lsp_find_references"])
	assert.True(t, toolNames["lsp_list_servers"])
}

// TestProtocolSSEHandler_GetEmbeddingsTools tests Embeddings tools list
func TestProtocolSSEHandler_GetEmbeddingsTools(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	tools := handler.getEmbeddingsTools()

	assert.GreaterOrEqual(t, len(tools), 2)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["embeddings_generate"])
	assert.True(t, toolNames["embeddings_search"])
}

// TestProtocolSSEHandler_GetVisionTools tests Vision tools list
func TestProtocolSSEHandler_GetVisionTools(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	tools := handler.getVisionTools()

	assert.GreaterOrEqual(t, len(tools), 2)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["vision_analyze_image"])
	assert.True(t, toolNames["vision_ocr"])
}

// TestProtocolSSEHandler_GetCogneeTools tests Cognee tools list
func TestProtocolSSEHandler_GetCogneeTools(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	tools := handler.getCogneeTools()

	assert.GreaterOrEqual(t, len(tools), 3)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["cognee_add"])
	assert.True(t, toolNames["cognee_search"])
	assert.True(t, toolNames["cognee_visualize"])
}

// TestProtocolSSEHandler_ExecuteMCPTool tests MCP tool execution
func TestProtocolSSEHandler_ExecuteMCPTool(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	result, err := handler.executeMCPTool("mcp_get_capabilities", nil)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "tools")
}

// TestProtocolSSEHandler_ExecuteMCPTool_ListProviders tests listing providers
func TestProtocolSSEHandler_ExecuteMCPTool_ListProviders(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	result, err := handler.executeMCPTool("mcp_list_providers", nil)

	assert.NoError(t, err)
	// With nil mcpHandler, should return empty array
	assert.Equal(t, "[]", result)
}

// TestProtocolSSEHandler_ExecuteMCPTool_UnknownTool tests unknown tool error
func TestProtocolSSEHandler_ExecuteMCPTool_UnknownTool(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	_, err := handler.executeMCPTool("unknown_tool", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown MCP tool")
}

// TestProtocolSSEHandler_ExecuteACPTool tests ACP tool execution
func TestProtocolSSEHandler_ExecuteACPTool(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	result, err := handler.executeACPTool("acp_list_agents", nil)

	assert.NoError(t, err)
	assert.Contains(t, result, "default")
}

// TestProtocolSSEHandler_ExecuteACPTool_SendMessage tests ACP send message
func TestProtocolSSEHandler_ExecuteACPTool_SendMessage(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	args := map[string]interface{}{
		"agent_id": "test-agent",
		"message":  "Hello, agent!",
	}

	result, err := handler.executeACPTool("acp_send_message", args)

	assert.NoError(t, err)
	assert.Contains(t, result, "test-agent")
	assert.Contains(t, result, "Hello, agent!")
}

// TestProtocolSSEHandler_ExecuteLSPTool tests LSP tool execution
func TestProtocolSSEHandler_ExecuteLSPTool(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	result, err := handler.executeLSPTool("lsp_list_servers", nil)

	assert.NoError(t, err)
	assert.Contains(t, result, "gopls")
}

// TestProtocolSSEHandler_ExecuteLSPTool_GetDiagnostics tests LSP diagnostics
func TestProtocolSSEHandler_ExecuteLSPTool_GetDiagnostics(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	args := map[string]interface{}{
		"file_path": "/test/file.go",
	}

	result, err := handler.executeLSPTool("lsp_get_diagnostics", args)

	assert.NoError(t, err)
	assert.Contains(t, result, "/test/file.go")
	assert.Contains(t, result, "No issues found")
}

// TestProtocolSSEHandler_ExecuteEmbeddingsTool tests Embeddings tool execution
func TestProtocolSSEHandler_ExecuteEmbeddingsTool(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	args := map[string]interface{}{
		"text": "Hello, world!",
	}

	result, err := handler.executeEmbeddingsTool("embeddings_generate", args)

	assert.NoError(t, err)
	assert.Contains(t, result, "Generated embedding")
}

// TestProtocolSSEHandler_ExecuteEmbeddingsTool_Search tests Embeddings search
func TestProtocolSSEHandler_ExecuteEmbeddingsTool_Search(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	args := map[string]interface{}{
		"query": "test query",
	}

	result, err := handler.executeEmbeddingsTool("embeddings_search", args)

	assert.NoError(t, err)
	assert.Contains(t, result, "test query")
}

// TestProtocolSSEHandler_ExecuteVisionTool tests Vision tool execution
func TestProtocolSSEHandler_ExecuteVisionTool(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	args := map[string]interface{}{
		"image_url": "https://example.com/image.png",
	}

	result, err := handler.executeVisionTool("vision_analyze_image", args)

	assert.NoError(t, err)
	assert.Contains(t, result, "https://example.com/image.png")
}

// TestProtocolSSEHandler_ExecuteVisionTool_OCR tests Vision OCR
func TestProtocolSSEHandler_ExecuteVisionTool_OCR(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	args := map[string]interface{}{
		"image_url": "https://example.com/document.png",
	}

	result, err := handler.executeVisionTool("vision_ocr", args)

	assert.NoError(t, err)
	assert.Contains(t, result, "OCR result")
}

// TestProtocolSSEHandler_ExecuteCogneeTool tests Cognee tool execution
func TestProtocolSSEHandler_ExecuteCogneeTool(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	args := map[string]interface{}{
		"content": "Test content for knowledge graph",
	}

	result, err := handler.executeCogneeTool("cognee_add", args)

	assert.NoError(t, err)
	assert.Contains(t, result, "Added content")
}

// TestProtocolSSEHandler_ExecuteCogneeTool_Search tests Cognee search
func TestProtocolSSEHandler_ExecuteCogneeTool_Search(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	args := map[string]interface{}{
		"query": "test search",
	}

	result, err := handler.executeCogneeTool("cognee_search", args)

	assert.NoError(t, err)
	assert.Contains(t, result, "test search")
}

// TestProtocolSSEHandler_ExecuteCogneeTool_Visualize tests Cognee visualize
func TestProtocolSSEHandler_ExecuteCogneeTool_Visualize(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	result, err := handler.executeCogneeTool("cognee_visualize", nil)

	assert.NoError(t, err)
	assert.Contains(t, result, "visualization generated")
}

// TestProtocolSSEHandler_HandlePromptsList tests prompts/list method
func TestProtocolSSEHandler_HandlePromptsList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "prompts/list",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMCPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Nil(t, response.Error)
	result := response.Result.(map[string]interface{})
	assert.NotNil(t, result["prompts"])
}

// TestProtocolSSEHandler_HandleResourcesList tests resources/list method
func TestProtocolSSEHandler_HandleResourcesList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "resources/list",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMCPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Nil(t, response.Error)
	result := response.Result.(map[string]interface{})
	assert.NotNil(t, result["resources"])
}

// TestProtocolSSEHandler_HandleInitialized tests initialized notification
func TestProtocolSSEHandler_HandleInitialized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "initialized", // No ID for notification
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMCPMessage(c)

	// Notifications return no content - Gin uses Status(204) but the response recorder may show 200 or 204
	// The important thing is that no error is returned
	assert.True(t, w.Code == http.StatusNoContent || w.Code == http.StatusOK, "Should return 204 or 200 for notification")
}

// TestProtocolSSEHandler_ToolsCall_InvalidParams tests tools/call with invalid params
func TestProtocolSSEHandler_ToolsCall_InvalidParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  json.RawMessage(`"invalid"`), // Not an object
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleMCPMessage(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotNil(t, response.Error)
	assert.Equal(t, -32602, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Invalid params")
}

// TestProtocolSSEHandler_ExecuteToolForProtocol_UnknownProtocol tests unknown protocol error
func TestProtocolSSEHandler_ExecuteToolForProtocol_UnknownProtocol(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	_, err := handler.executeToolForProtocol(nil, "unknown", "tool", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown protocol")
}

// TestProtocolSSEHandler_GetCapabilitiesForProtocol tests capabilities retrieval for all protocols
func TestProtocolSSEHandler_GetCapabilitiesForProtocol(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	protocols := []string{"mcp", "acp", "lsp", "embeddings", "vision", "cognee", "unknown"}

	for _, protocol := range protocols {
		caps := handler.getCapabilitiesForProtocol(protocol)
		assert.NotNil(t, caps, "Capabilities for %s should not be nil", protocol)
	}
}

// TestProtocolSSEHandler_GetToolsForProtocol tests tools retrieval for all protocols
func TestProtocolSSEHandler_GetToolsForProtocol(t *testing.T) {
	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	testCases := []struct {
		protocol     string
		expectedMin  int
	}{
		{"mcp", 3},
		{"acp", 2},
		{"lsp", 4},
		{"embeddings", 2},
		{"vision", 2},
		{"cognee", 3},
		{"unknown", 0},
	}

	for _, tc := range testCases {
		tools := handler.getToolsForProtocol(tc.protocol)
		assert.GreaterOrEqual(t, len(tools), tc.expectedMin, "Tools for %s should have at least %d tools", tc.protocol, tc.expectedMin)
	}
}

// TestJSONRPCMessage_Serialization tests JSON-RPC message serialization
func TestJSONRPCMessage_Serialization(t *testing.T) {
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
		Params:  json.RawMessage(`{"key": "value"}`),
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded JSONRPCMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Equal(t, float64(1), decoded.ID)
	assert.Equal(t, "test", decoded.Method)
}

// TestJSONRPCError_Serialization tests JSON-RPC error serialization
func TestJSONRPCError_Serialization(t *testing.T) {
	rpcErr := JSONRPCError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    "Additional error info",
	}

	data, err := json.Marshal(rpcErr)
	require.NoError(t, err)

	var decoded JSONRPCError
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, -32600, decoded.Code)
	assert.Equal(t, "Invalid Request", decoded.Message)
	assert.Equal(t, "Additional error info", decoded.Data)
}

// TestMCPTool_Serialization tests MCP tool serialization
func TestMCPTool_Serialization(t *testing.T) {
	tool := MCPTool{
		Name:        "test_tool",
		Description: "Test tool description",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	data, err := json.Marshal(tool)
	require.NoError(t, err)

	var decoded MCPTool
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "test_tool", decoded.Name)
	assert.Equal(t, "Test tool description", decoded.Description)
	assert.NotNil(t, decoded.InputSchema)
}

// TestMCPCapabilities_Structure tests MCP capabilities structure
func TestMCPCapabilities_Structure(t *testing.T) {
	caps := MCPCapabilities{
		Tools: &MCPToolsCapability{
			ListChanged: true,
		},
		Prompts: &MCPPromptsCapability{
			ListChanged: true,
		},
		Resources: &MCPResourcesCapability{
			Subscribe:   true,
			ListChanged: true,
		},
	}

	data, err := json.Marshal(caps)
	require.NoError(t, err)

	var decoded MCPCapabilities
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.NotNil(t, decoded.Tools)
	assert.True(t, decoded.Tools.ListChanged)
	assert.NotNil(t, decoded.Resources)
	assert.True(t, decoded.Resources.Subscribe)
}

// TestMCPServerInfo_Structure tests MCP server info structure
func TestMCPServerInfo_Structure(t *testing.T) {
	info := MCPServerInfo{
		Name:            "helixagent-mcp",
		Version:         "1.0.0",
		ProtocolVersion: MCPProtocolVersion,
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded MCPServerInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "helixagent-mcp", decoded.Name)
	assert.Equal(t, "1.0.0", decoded.Version)
	assert.Equal(t, MCPProtocolVersion, decoded.ProtocolVersion)
}

// TestProtocolSSEHandler_AllProtocols_ToolsCallExecution tests tools/call for all protocols
func TestProtocolSSEHandler_AllProtocols_ToolsCallExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	testCases := []struct {
		protocol   string
		handlerFn  func(*gin.Context)
		tool       string
		args       map[string]interface{}
	}{
		{"mcp", handler.HandleMCPMessage, "mcp_get_capabilities", nil},
		{"acp", handler.HandleACPMessage, "acp_list_agents", nil},
		{"lsp", handler.HandleLSPMessage, "lsp_list_servers", nil},
		{"embeddings", handler.HandleEmbeddingsMessage, "embeddings_generate", map[string]interface{}{"text": "test"}},
		{"vision", handler.HandleVisionMessage, "vision_ocr", map[string]interface{}{"image_url": "http://test.com/img.png"}},
		{"cognee", handler.HandleCogneeMessage, "cognee_visualize", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.protocol, func(t *testing.T) {
			params := map[string]interface{}{
				"name":      tc.tool,
				"arguments": tc.args,
			}
			paramsJSON, _ := json.Marshal(params)

			msg := JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
				Params:  paramsJSON,
			}

			reqBytes, _ := json.Marshal(msg)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/"+tc.protocol, bytes.NewBuffer(reqBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			tc.handlerFn(c)

			assert.Equal(t, http.StatusOK, w.Code)

			var response JSONRPCMessage
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Nil(t, response.Error, "Protocol %s tools/call should succeed", tc.protocol)
			assert.NotNil(t, response.Result, "Protocol %s tools/call should return result", tc.protocol)
		})
	}
}

// TestProtocolSSEHandler_ConcurrentAccess tests concurrent access to the handler
func TestProtocolSSEHandler_ConcurrentAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, logger)

	// Test concurrent tool list requests
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			msg := JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      id,
				Method:  "tools/list",
			}

			reqBytes, _ := json.Marshal(msg)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/mcp", bytes.NewBuffer(reqBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.HandleMCPMessage(c)

			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
