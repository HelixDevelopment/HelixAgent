package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/superagent/superagent/internal/services"
)

// MockUnifiedProtocolManager mocks the UnifiedProtocolManager for testing
type MockUnifiedProtocolManager struct {
	mock.Mock
}

func (m *MockUnifiedProtocolManager) ExecuteRequest(ctx context.Context, req services.UnifiedProtocolRequest) (services.UnifiedProtocolResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(services.UnifiedProtocolResponse), args.Error(1)
}

func (m *MockUnifiedProtocolManager) ListServers(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockUnifiedProtocolManager) GetMetrics(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockUnifiedProtocolManager) RefreshAll(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockUnifiedProtocolManager) ConfigureProtocols(ctx context.Context, config map[string]interface{}) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func TestProtocolHandler_ExecuteProtocolRequest(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	req := services.UnifiedProtocolRequest{
		ProtocolType: "mcp",
		ServerID:     "test-server",
		ToolName:     "test-tool",
		Arguments:    map[string]interface{}{"arg1": "value1"},
	}

	expectedResponse := services.UnifiedProtocolResponse{
		Success:   true,
		Result:    "Tool executed successfully",
		Protocol:  "mcp",
		Timestamp: time.Now(),
	}

	mockManager.On("ExecuteRequest", mock.Anything, req).Return(expectedResponse, nil)

	// Create HTTP request
	requestBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/protocols/execute", bytes.NewBuffer(requestBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Test
	handler.ExecuteProtocolRequest(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response services.UnifiedProtocolResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "mcp", response.Protocol)
	assert.NotNil(t, response.Result)

	mockManager.AssertExpectations(t)
}

func TestProtocolHandler_ListProtocolServers(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	expectedServers := map[string]interface{}{
		"mcp": []map[string]interface{}{
			{"id": "server1", "name": "MCP Server 1"},
		},
		"acp": []map[string]interface{}{
			{"id": "server2", "name": "ACP Server 1"},
		},
	}

	mockManager.On("ListServers", mock.Anything).Return(expectedServers, nil)

	// Create HTTP request
	httpReq := httptest.NewRequest("GET", "/protocols/servers", nil)

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Test
	handler.ListProtocolServers(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "mcp")
	assert.Contains(t, response, "acp")

	mockManager.AssertExpectations(t)
}

func TestProtocolHandler_GetProtocolMetrics(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	expectedMetrics := map[string]interface{}{
		"totalRequests":     150,
		"activeConnections": 5,
		"cacheHitRate":      0.85,
	}

	mockManager.On("GetMetrics", mock.Anything).Return(expectedMetrics, nil)

	// Create HTTP request
	httpReq := httptest.NewRequest("GET", "/protocols/metrics", nil)

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Test
	handler.GetProtocolMetrics(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(150), response["totalRequests"])
	assert.Equal(t, float64(5), response["activeConnections"])
	assert.Equal(t, 0.85, response["cacheHitRate"])

	mockManager.AssertExpectations(t)
}

func TestProtocolHandler_RefreshProtocolServers(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	mockManager.On("RefreshAll", mock.Anything).Return(nil)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/protocols/refresh", nil)

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Test
	handler.RefreshProtocolServers(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "message")
	assert.Contains(t, response["message"].(string), "refreshed")

	mockManager.AssertExpectations(t)
}

func TestProtocolHandler_ConfigureProtocols(t *testing.T) {
	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	config := map[string]interface{}{
		"mcp": map[string]interface{}{
			"enabled": true,
			"servers": []interface{}{"server1", "server2"},
		},
		"acp": map[string]interface{}{
			"enabled": true,
		},
	}

	mockManager.On("ConfigureProtocols", mock.Anything, config).Return(nil)

	// Create HTTP request
	requestBody, _ := json.Marshal(config)
	httpReq := httptest.NewRequest("POST", "/protocols/configure", bytes.NewBuffer(requestBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Test
	handler.ConfigureProtocols(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "message")
	assert.Contains(t, response["message"].(string), "configured")

	mockManager.AssertExpectations(t)
}

func TestNewProtocolHandler(t *testing.T) {
	logger := logrus.New()
	mockManager := &MockUnifiedProtocolManager{}

	handler := NewProtocolHandler(mockManager, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, mockManager, handler.protocolService)
	assert.Equal(t, logger, handler.log)
}

func TestProtocolHandler_ExecuteProtocolRequest_InvalidJSON(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	// Create HTTP request with invalid JSON
	httpReq := httptest.NewRequest("POST", "/protocols/execute", bytes.NewBufferString("invalid json"))
	httpReq.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	handler.ExecuteProtocolRequest(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProtocolHandler_ExecuteProtocolRequest_ServiceError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	req := services.UnifiedProtocolRequest{
		ProtocolType: "mcp",
		ServerID:     "test-server",
		ToolName:     "test-tool",
	}

	mockManager.On("ExecuteRequest", mock.Anything, req).Return(services.UnifiedProtocolResponse{}, assert.AnError)

	requestBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/protocols/execute", bytes.NewBuffer(requestBody))
	httpReq.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	handler.ExecuteProtocolRequest(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockManager.AssertExpectations(t)
}

func TestProtocolHandler_ListProtocolServers_Error(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	mockManager.On("ListServers", mock.Anything).Return(map[string]interface{}{}, assert.AnError)

	httpReq := httptest.NewRequest("GET", "/protocols/servers", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	handler.ListProtocolServers(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockManager.AssertExpectations(t)
}

func TestProtocolHandler_GetProtocolMetrics_Error(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	mockManager.On("GetMetrics", mock.Anything).Return(map[string]interface{}{}, assert.AnError)

	httpReq := httptest.NewRequest("GET", "/protocols/metrics", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	handler.GetProtocolMetrics(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockManager.AssertExpectations(t)
}

func TestProtocolHandler_RefreshProtocolServers_Error(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	mockManager.On("RefreshAll", mock.Anything).Return(assert.AnError)

	httpReq := httptest.NewRequest("POST", "/protocols/refresh", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	handler.RefreshProtocolServers(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockManager.AssertExpectations(t)
}

func TestProtocolHandler_ConfigureProtocols_InvalidJSON(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	httpReq := httptest.NewRequest("POST", "/protocols/configure", bytes.NewBufferString("{invalid"))
	httpReq.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	handler.ConfigureProtocols(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProtocolHandler_ConfigureProtocols_ServiceError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	mockManager := &MockUnifiedProtocolManager{}
	handler := NewProtocolHandler(mockManager, logger)

	config := map[string]interface{}{"enabled": true}

	mockManager.On("ConfigureProtocols", mock.Anything, config).Return(assert.AnError)

	requestBody, _ := json.Marshal(config)
	httpReq := httptest.NewRequest("POST", "/protocols/configure", bytes.NewBuffer(requestBody))
	httpReq.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	handler.ConfigureProtocols(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockManager.AssertExpectations(t)
}
