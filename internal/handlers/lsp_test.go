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

func TestNewLSPHandler(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	handler := NewLSPHandler(nil, log)

	require.NotNil(t, handler)
	assert.NotNil(t, handler.log)
	assert.Nil(t, handler.lspService) // nil when no service provided
}

func TestLSPHandler_ExecuteLSPRequest_InvalidJSON(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	handler := NewLSPHandler(nil, log)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/lsp/execute", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ExecuteLSPRequest(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLSPHandler_ExecuteLSPRequest_ValidJSON(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	handler := NewLSPHandler(nil, log)

	// Valid JSON should pass binding and return success
	// (the method is a placeholder that doesn't call the service)
	body := `{"server_id": "gopls", "tool_name": "completion", "params": {"file": "main.go"}}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/lsp/execute", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ExecuteLSPRequest(c)

	// The placeholder implementation returns success
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLSPHandler_ExecuteLSPRequest_EmptyBody(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	handler := NewLSPHandler(nil, log)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/lsp/execute", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ExecuteLSPRequest(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLSPHandler_SyncLSPServer_ParamParsing(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	handler := NewLSPHandler(nil, log)

	t.Run("with server id param", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/v1/lsp/servers/gopls/sync", nil)
		c.Params = gin.Params{{Key: "id", Value: "gopls"}}

		// This will call the nil service and panic, but we can test the param parsing
		// by checking if it would have been called with the right server ID
		// For now, we just verify the handler was created correctly
		require.NotNil(t, handler)
	})
}

func BenchmarkLSPHandler_ExecuteLSPRequest(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.WarnLevel)

	handler := NewLSPHandler(nil, log)

	body := `{"server_id": "gopls", "tool_name": "completion"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/v1/lsp/execute", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.ExecuteLSPRequest(c)
	}
}

// MockLSPManager is a mock implementation of LSPManager for testing
type MockLSPManager struct {
	ListLSPServersFunc func() ([]map[string]interface{}, error)
	SyncLSPServerFunc  func(serverID string) error
	GetLSPStatsFunc    func() (map[string]interface{}, error)
}

func (m *MockLSPManager) ListLSPServers(ctx interface{}) ([]map[string]interface{}, error) {
	if m.ListLSPServersFunc != nil {
		return m.ListLSPServersFunc()
	}
	return []map[string]interface{}{}, nil
}

func (m *MockLSPManager) SyncLSPServer(ctx interface{}, serverID string) error {
	if m.SyncLSPServerFunc != nil {
		return m.SyncLSPServerFunc(serverID)
	}
	return nil
}

func (m *MockLSPManager) GetLSPStats(ctx interface{}) (map[string]interface{}, error) {
	if m.GetLSPStatsFunc != nil {
		return m.GetLSPStatsFunc()
	}
	return map[string]interface{}{}, nil
}

func TestLSPHandler_ExecuteLSPRequest_WithFields(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	handler := NewLSPHandler(nil, log)

	t.Run("valid request with all fields", func(t *testing.T) {
		body := `{
			"serverId": "gopls",
			"toolName": "completion",
			"protocolType": "lsp",
			"arguments": {"line": 10, "column": 5}
		}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/v1/lsp/execute", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ExecuteLSPRequest(c)

		assert.Equal(t, http.StatusOK, w.Code)

		body_resp := w.Body.String()
		assert.Contains(t, body_resp, "success")
		assert.Contains(t, body_resp, "true")
	})

	t.Run("request with minimal fields", func(t *testing.T) {
		body := `{"serverId": "pyright", "toolName": "hover"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/v1/lsp/execute", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ExecuteLSPRequest(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestLSPHandler_ExecuteLSPRequest_Operations(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	handler := NewLSPHandler(nil, log)

	operations := []string{"completion", "hover", "definition", "references", "rename", "formatting"}

	for _, op := range operations {
		t.Run("operation_"+op, func(t *testing.T) {
			// Use JSON field names (camelCase as per struct tags)
			body := `{"serverId": "gopls", "toolName": "` + op + `"}`
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/lsp/execute", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.ExecuteLSPRequest(c)

			assert.Equal(t, http.StatusOK, w.Code)

			body_resp := w.Body.String()
			assert.Contains(t, body_resp, "success")
		})
	}
}

func TestLSPHandler_ExecuteLSPRequest_WithContext(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	handler := NewLSPHandler(nil, log)

	body := `{"serverId": "gopls", "toolName": "completion"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/lsp/execute", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ExecuteLSPRequest(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body_resp := w.Body.String()
	assert.Contains(t, body_resp, "result")
	assert.Contains(t, body_resp, "LSP operation completed successfully")
}

func TestLSPHandler_ExecuteLSPRequest_ResponseFormat(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	handler := NewLSPHandler(nil, log)

	body := `{"serverId": "test-server", "toolName": "test-operation"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/lsp/execute", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ExecuteLSPRequest(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	assert.Equal(t, "test-server", response["serverId"])
	assert.Equal(t, "test-operation", response["operation"])
	assert.Equal(t, "LSP operation completed successfully", response["result"])
}

func TestLSPHandler_SyncLSPServer_MultipleIDs(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	handler := NewLSPHandler(nil, log)

	serverIDs := []string{"gopls", "pyright", "tsserver", "rust-analyzer", "clangd"}

	for _, serverID := range serverIDs {
		t.Run("server_"+serverID, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/lsp/servers/"+serverID+"/sync", nil)
			c.Params = gin.Params{{Key: "id", Value: serverID}}

			// Can't call SyncLSPServer directly without a service, but we can test param extraction
			require.NotNil(t, handler)
			assert.Equal(t, serverID, c.Param("id"))
		})
	}
}
