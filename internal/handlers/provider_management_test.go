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
	"github.com/superagent/superagent/internal/services"
)

func newTestProviderLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	return logger
}

func TestNewProviderManagementHandler(t *testing.T) {
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.providerRegistry)
	assert.NotNil(t, handler.log)
}

func TestProviderManagementHandler_AddProvider_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	t.Run("returns error for missing required fields", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Missing required fields
		c.Request = httptest.NewRequest("POST", "/v1/providers", bytes.NewReader([]byte(`{}`)))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.AddProvider(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest("POST", "/v1/providers", bytes.NewReader([]byte(`{invalid json}`)))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.AddProvider(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for invalid provider type", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := AddProviderRequest{
			Name:    "test-provider",
			Type:    "invalid-type",
			APIKey:  "test-key",
			BaseURL: "https://api.example.com",
			Model:   "test-model",
		}
		jsonBody, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/v1/providers", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.AddProvider(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "invalid provider type")
		assert.NotNil(t, response["valid_types"])
	})

	t.Run("accepts valid provider types", func(t *testing.T) {
		validTypes := []string{"deepseek", "claude", "gemini", "qwen", "zai", "ollama", "openrouter"}

		for _, providerType := range validTypes {
			t.Run(providerType, func(t *testing.T) {
				// Create new handler for each test to avoid conflicts
				localRegistry := services.NewProviderRegistry(nil, nil)
				localHandler := NewProviderManagementHandler(localRegistry, logger)

				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)

				reqBody := AddProviderRequest{
					Name:    "test-" + providerType,
					Type:    providerType,
					APIKey:  "test-key",
					BaseURL: "https://api.example.com",
					Model:   "test-model",
					Weight:  1.0,
					Enabled: true,
				}
				jsonBody, _ := json.Marshal(reqBody)
				c.Request = httptest.NewRequest("POST", "/v1/providers", bytes.NewReader(jsonBody))
				c.Request.Header.Set("Content-Type", "application/json")

				localHandler.AddProvider(c)

				// Should not be BadRequest (invalid type)
				// May be InternalServerError if provider creation fails
				// but should NOT be BadRequest for invalid type
				assert.NotEqual(t, http.StatusBadRequest, w.Code, "Provider type %s should be valid", providerType)
			})
		}
	})
}

func TestProviderManagementHandler_GetProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	t.Run("returns 404 for non-existent provider", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
		c.Request = httptest.NewRequest("GET", "/v1/providers/non-existent", nil)

		handler.GetProvider(c)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "provider not found")
	})
}

func TestProviderManagementHandler_UpdateProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	t.Run("returns 404 for non-existent provider", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}

		reqBody := UpdateProviderRequest{
			Name: "updated-name",
		}
		jsonBody, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("PUT", "/v1/providers/non-existent", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.UpdateProvider(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "test-provider"}}
		c.Request = httptest.NewRequest("PUT", "/v1/providers/test-provider", bytes.NewReader([]byte(`{invalid}`)))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.UpdateProvider(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestProviderManagementHandler_DeleteProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	t.Run("returns 404 for non-existent provider", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
		c.Request = httptest.NewRequest("DELETE", "/v1/providers/non-existent", nil)

		handler.DeleteProvider(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 404 for non-existent provider with force", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
		c.Request = httptest.NewRequest("DELETE", "/v1/providers/non-existent?force=true", nil)

		handler.DeleteProvider(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAddProviderRequest_Struct(t *testing.T) {
	req := AddProviderRequest{
		Name:    "test-provider",
		Type:    "deepseek",
		APIKey:  "sk-test-key",
		BaseURL: "https://api.example.com",
		Model:   "deepseek-chat",
		Weight:  1.5,
		Enabled: true,
		Config: map[string]interface{}{
			"temperature": 0.7,
			"max_tokens":  1000,
		},
	}

	assert.Equal(t, "test-provider", req.Name)
	assert.Equal(t, "deepseek", req.Type)
	assert.Equal(t, "sk-test-key", req.APIKey)
	assert.Equal(t, "https://api.example.com", req.BaseURL)
	assert.Equal(t, "deepseek-chat", req.Model)
	assert.Equal(t, 1.5, req.Weight)
	assert.True(t, req.Enabled)
	assert.NotNil(t, req.Config)
}

func TestUpdateProviderRequest_Struct(t *testing.T) {
	enabled := true
	req := UpdateProviderRequest{
		Name:    "updated-provider",
		APIKey:  "new-api-key",
		BaseURL: "https://new-api.example.com",
		Model:   "new-model",
		Weight:  2.0,
		Enabled: &enabled,
		Config: map[string]interface{}{
			"temperature": 0.5,
		},
	}

	assert.Equal(t, "updated-provider", req.Name)
	assert.Equal(t, "new-api-key", req.APIKey)
	assert.Equal(t, "https://new-api.example.com", req.BaseURL)
	assert.Equal(t, "new-model", req.Model)
	assert.Equal(t, 2.0, req.Weight)
	assert.NotNil(t, req.Enabled)
	assert.True(t, *req.Enabled)
}

func TestProviderResponse_Struct(t *testing.T) {
	resp := ProviderResponse{
		Success:  true,
		Message:  "Provider created successfully",
		Provider: nil,
	}

	assert.True(t, resp.Success)
	assert.Equal(t, "Provider created successfully", resp.Message)
	assert.Nil(t, resp.Provider)
}
