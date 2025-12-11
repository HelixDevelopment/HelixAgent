package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestOpenRouterModelsHandler_HandleModels tests models endpoint
func TestOpenRouterModelsHandler_HandleModels(t *testing.T) {
	handler := NewOpenRouterModelsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/openrouter/models", nil)

	handler.HandleModels(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "object")
	assert.Contains(t, body, "data")
	assert.Contains(t, body, "openrouter/anthropic/claude-3.5-sonnet")
	assert.Contains(t, body, "openrouter/openai/gpt-4o")
	assert.Contains(t, body, "openrouter/google/gemini-pro-1.5")
}

// TestOpenRouterModelsHandler_HandleModels_WithAuth tests models endpoint with auth
func TestOpenRouterModelsHandler_HandleModels_WithAuth(t *testing.T) {
	handler := NewOpenRouterModelsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/openrouter/models", nil)
	c.Request.Header.Set("Authorization", "Bearer test-key")

	handler.HandleModels(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "object")
	assert.Contains(t, body, "data")
	assert.Contains(t, body, "openrouter/anthropic/claude-3.5-sonnet-20241022")
}

// TestOpenRouterModelsHandler_HandleModelMetadata tests model metadata endpoint
func TestOpenRouterModelsHandler_HandleModelMetadata(t *testing.T) {
	handler := NewOpenRouterModelsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/openrouter/models/test-model", nil)

	// Set the model parameter in the Gin context
	c.Params = []gin.Param{{Key: "model", Value: "test-model"}}

	handler.HandleModelMetadata(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "id")
	assert.Contains(t, body, "test-model")
	assert.Contains(t, body, "object")
	assert.Contains(t, body, "model")
	assert.Contains(t, body, "capabilities")
	assert.Contains(t, body, "pricing")
	assert.Contains(t, body, "limits")
}

// TestOpenRouterModelsHandler_HandleModelMetadata_NoModel tests model metadata endpoint without model
func TestOpenRouterModelsHandler_HandleModelMetadata_NoModel(t *testing.T) {
	handler := NewOpenRouterModelsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/openrouter/models/", nil)

	handler.HandleModelMetadata(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "model parameter is required")
}

// TestOpenRouterModelsHandler_HandleProviderHealth tests provider health endpoint
func TestOpenRouterModelsHandler_HandleProviderHealth(t *testing.T) {
	handler := NewOpenRouterModelsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/openrouter/health", nil)
	c.Request.Header.Set("Authorization", "Bearer test-key")

	handler.HandleProviderHealth(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "status")
	assert.Contains(t, body, "healthy")
	assert.Contains(t, body, "provider")
	assert.Contains(t, body, "openrouter")
	assert.Contains(t, body, "rate_limits")
}

// TestOpenRouterModelsHandler_HandleProviderHealth_NoAuth tests provider health endpoint without auth
func TestOpenRouterModelsHandler_HandleProviderHealth_NoAuth(t *testing.T) {
	handler := NewOpenRouterModelsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/openrouter/health", nil)

	handler.HandleProviderHealth(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "OpenRouter API key required for health check")
}

// TestOpenRouterModelsHandler_HandleUsageStats tests usage stats endpoint
func TestOpenRouterModelsHandler_HandleUsageStats(t *testing.T) {
	handler := NewOpenRouterModelsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/openrouter/usage", nil)

	handler.HandleUsageStats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "provider")
	assert.Contains(t, body, "openrouter")
	assert.Contains(t, body, "requests_total")
	assert.Contains(t, body, "success_rate")
	assert.Contains(t, body, "models_usage")
	assert.Contains(t, body, "daily_usage")
	assert.Contains(t, body, "monthly_usage")
}

// TestNewOpenRouterModelsHandler tests handler creation
func TestNewOpenRouterModelsHandler(t *testing.T) {
	handler := NewOpenRouterModelsHandler()

	assert.NotNil(t, handler)
}
