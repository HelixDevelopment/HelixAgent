package services

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMCPClientTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewMCPClient(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	require.NotNil(t, client)
	assert.NotNil(t, client.logger)
}

func TestMCPToolCallRequest(t *testing.T) {
	req := ToolCallRequest{
		Name: "test_tool",
		Arguments: map[string]interface{}{
			"arg1": "value1",
		},
	}

	assert.Equal(t, "test_tool", req.Name)
	assert.Equal(t, "value1", req.Arguments["arg1"])
}

func TestMCPToolCallResult(t *testing.T) {
	result := ToolCallResult{
		Content: []Content{
			{Type: "text", Text: "test result"},
		},
		IsError: false,
	}

	assert.Equal(t, 1, len(result.Content))
	assert.Equal(t, "text", result.Content[0].Type)
	assert.Equal(t, "test result", result.Content[0].Text)
	assert.False(t, result.IsError)
}

func TestMCPErrorResult(t *testing.T) {
	result := ToolCallResult{
		Content: []Content{
			{Type: "text", Text: "error occurred"},
		},
		IsError: true,
	}

	assert.True(t, result.IsError)
}

func TestJSONRPCRequest(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test/method",
		Params:  map[string]string{"key": "value"},
	}

	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, 1, req.ID)
	assert.Equal(t, "test/method", req.Method)
}

func TestJSONRPCResponse(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  map[string]string{"result": "success"},
	}

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.NotNil(t, resp.Result)
}

func TestJSONRPCErrorResponse(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error: &JSONRPCError{
			Code:    -32601,
			Message: "Method not found",
		},
	}

	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Equal(t, "Method not found", resp.Error.Message)
}

func TestMCPProtocolConstants(t *testing.T) {
	assert.Equal(t, "2024-11-05", MCPProtocolVersion)
	assert.Equal(t, -32700, JSONRPCParseError)
	assert.Equal(t, -32600, JSONRPCInvalidRequest)
	assert.Equal(t, -32601, JSONRPCMethodNotFound)
	assert.Equal(t, -32602, JSONRPCInvalidParams)
	assert.Equal(t, -32603, JSONRPCInternalError)
	assert.Equal(t, -32000, JSONRPCServerError)
}

func TestImplementation(t *testing.T) {
	impl := Implementation{
		Name:    "test-impl",
		Version: "1.0.0",
	}

	assert.Equal(t, "test-impl", impl.Name)
	assert.Equal(t, "1.0.0", impl.Version)
}

func TestContent(t *testing.T) {
	content := Content{
		Type: "text",
		Text: "test content",
	}

	assert.Equal(t, "text", content.Type)
	assert.Equal(t, "test content", content.Text)
}

func TestMCPTool(t *testing.T) {
	tool := &MCPTool{
		Name:        "test_tool",
		Description: "A test tool",
	}

	assert.Equal(t, "test_tool", tool.Name)
	assert.Equal(t, "A test tool", tool.Description)
}
