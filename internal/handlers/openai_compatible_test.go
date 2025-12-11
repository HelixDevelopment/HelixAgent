package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/config"
)

// TestUnifiedHandler_Models tests models endpoint
func TestUnifiedHandler_Models(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)

	handler.Models(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "object")
	assert.Contains(t, body, "data")
	assert.Contains(t, body, "superagent-ensemble")
}

// TestUnifiedHandler_ModelsPublic tests public models endpoint
func TestUnifiedHandler_ModelsPublic(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models/public", nil)

	handler.ModelsPublic(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "object")
	assert.Contains(t, body, "data")
	assert.Contains(t, body, "superagent-ensemble")
}

// TestUnifiedHandler_ChatCompletions_InvalidRequest tests invalid request
func TestUnifiedHandler_ChatCompletions_InvalidRequest(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Invalid JSON
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletions(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_Completions_InvalidRequest tests invalid completions request
func TestUnifiedHandler_Completions_InvalidRequest(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Invalid JSON
	c.Request = httptest.NewRequest("POST", "/v1/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Completions(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_ChatCompletionsStream_InvalidRequest tests invalid stream request
func TestUnifiedHandler_ChatCompletionsStream_InvalidRequest(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Invalid JSON
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletionsStream(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_CompletionsStream_InvalidRequest tests invalid completions stream request
func TestUnifiedHandler_CompletionsStream_InvalidRequest(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Invalid JSON
	c.Request = httptest.NewRequest("POST", "/v1/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CompletionsStream(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestNewUnifiedHandler tests handler creation
func TestNewUnifiedHandler(t *testing.T) {
	cfg := &config.Config{}

	handler := NewUnifiedHandler(nil, cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
	assert.Nil(t, handler.providerRegistry)
}

// TestSendOpenAIError tests error response formatting
func TestSendOpenAIError(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handler.sendOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "Invalid request", "Missing required field")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "invalid_request_error")
	assert.Contains(t, body, "Invalid request")
	assert.Contains(t, body, "Missing required field")
}
