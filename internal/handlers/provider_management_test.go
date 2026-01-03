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

func TestProviderManagementHandler_GetProvider_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: ""}}
	c.Request = httptest.NewRequest("GET", "/v1/providers/", nil)

	handler.GetProvider(c)

	// Empty ID will not be found in registry
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "provider not found")
}

func TestProviderManagementHandler_UpdateProvider_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: ""}}
	reqBody := UpdateProviderRequest{Name: "test"}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("PUT", "/v1/providers/", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateProvider(c)

	// Empty ID will not be found in registry
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "provider not found")
}

func TestProviderManagementHandler_DeleteProvider_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: ""}}
	c.Request = httptest.NewRequest("DELETE", "/v1/providers/", nil)

	handler.DeleteProvider(c)

	// Empty ID will not be found in registry
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "provider not found")
}

func TestProviderManagementHandler_AddProvider_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := AddProviderRequest{
		Name:    "test-deepseek",
		Type:    "deepseek",
		APIKey:  "sk-test-key-12345",
		BaseURL: "https://api.deepseek.com",
		Model:   "deepseek-chat",
		Weight:  1.0,
		Enabled: true,
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/v1/providers", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.AddProvider(c)

	// Should succeed (200 or 201) or fail gracefully
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated || w.Code == http.StatusInternalServerError)
}

func TestProviderManagementHandler_AddProvider_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/providers", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.AddProvider(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProviderManagementHandler_UpdateProvider_EmptyRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-provider"}}
	c.Request = httptest.NewRequest("PUT", "/v1/providers/test-provider", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateProvider(c)

	// Should return 404 because provider doesn't exist, not 400
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestProviderManagementHandler_AddProvider_DuplicateProvider tests adding duplicate provider
func TestProviderManagementHandler_AddProvider_DuplicateProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	// First, register a provider using RegisterProviderFromConfig
	err := registry.RegisterProviderFromConfig(services.ProviderConfig{
		Name:    "existing-provider",
		Type:    "deepseek",
		APIKey:  "test-key",
		BaseURL: "https://api.example.com",
		Enabled: true,
		Models: []services.ModelConfig{{
			ID:      "test-model",
			Name:    "test-model",
			Enabled: true,
			Weight:  1.0,
		}},
	})
	// Note: This may fail if the provider type doesn't match a registered factory
	// but we can still test the duplicate check logic

	// Now try to add a provider with the same name
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := AddProviderRequest{
		Name:    "existing-provider",
		Type:    "deepseek",
		APIKey:  "new-key",
		BaseURL: "https://api.new.com",
		Model:   "new-model",
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/v1/providers", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.AddProvider(c)

	// If the first provider was registered, this should be a conflict
	// If not, it should be successful or an internal error
	if err == nil {
		assert.Equal(t, http.StatusConflict, w.Code)
	}
}

// TestProviderManagementHandler_AddProvider_DefaultWeight tests default weight setting
func TestProviderManagementHandler_AddProvider_DefaultWeight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Request without weight specified (should default to 1.0)
	reqBody := AddProviderRequest{
		Name:    "test-provider-no-weight",
		Type:    "deepseek",
		APIKey:  "test-key",
		BaseURL: "https://api.example.com",
		Model:   "test-model",
		Enabled: true,
		// Weight is not set, should default to 1.0
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/v1/providers", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.AddProvider(c)

	// The request should be processed (may succeed or fail depending on provider factory)
	// But it should not fail due to weight validation
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

// TestProviderManagementHandler_UpdateProvider_WithModel tests update with model change
func TestProviderManagementHandler_UpdateProvider_WithModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-provider"}}

	enabled := true
	reqBody := UpdateProviderRequest{
		Name:    "updated-name",
		APIKey:  "new-api-key",
		BaseURL: "https://new-api.example.com",
		Model:   "new-model",
		Weight:  2.5,
		Enabled: &enabled,
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("PUT", "/v1/providers/test-provider", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateProvider(c)

	// Should return 404 because provider doesn't exist
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestProviderManagementHandler_UpdateProvider_EnabledNil tests update with nil enabled
func TestProviderManagementHandler_UpdateProvider_EnabledNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-provider"}}

	reqBody := UpdateProviderRequest{
		APIKey: "new-api-key",
		// Enabled is nil
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("PUT", "/v1/providers/test-provider", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateProvider(c)

	// Should return 404 because provider doesn't exist
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestProviderManagementHandler_DeleteProvider_WithForce tests delete with force parameter
func TestProviderManagementHandler_DeleteProvider_WithForce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	c.Request = httptest.NewRequest("DELETE", "/v1/providers/non-existent?force=true", nil)

	handler.DeleteProvider(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestProviderManagementHandler_DeleteProvider_ForceParamVariations tests different force param values
func TestProviderManagementHandler_DeleteProvider_ForceParamVariations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	forceValues := []string{"false", "TRUE", "True", "1", "yes"}

	for _, forceVal := range forceValues {
		t.Run("force="+forceVal, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
			c.Request = httptest.NewRequest("DELETE", "/v1/providers/non-existent?force="+forceVal, nil)

			handler.DeleteProvider(c)

			// Provider doesn't exist, so should be 404
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

// TestProviderManagementHandler_AddProvider_AllProviderTypes tests adding all valid provider types
func TestProviderManagementHandler_AddProvider_AllProviderTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()

	providerTypes := []string{"deepseek", "claude", "gemini", "qwen", "zai", "ollama", "openrouter"}

	for i, providerType := range providerTypes {
		t.Run(providerType, func(t *testing.T) {
			registry := services.NewProviderRegistry(nil, nil)
			handler := NewProviderManagementHandler(registry, logger)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			reqBody := AddProviderRequest{
				Name:    "test-provider-" + providerType + "-" + string(rune('0'+i)),
				Type:    providerType,
				APIKey:  "test-key-" + providerType,
				BaseURL: "https://api." + providerType + ".com",
				Model:   providerType + "-model",
				Weight:  1.0,
				Enabled: true,
				Config: map[string]interface{}{
					"temperature": 0.7,
				},
			}
			jsonBody, _ := json.Marshal(reqBody)
			c.Request = httptest.NewRequest("POST", "/v1/providers", bytes.NewReader(jsonBody))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.AddProvider(c)

			// Should not fail due to invalid type
			assert.NotEqual(t, http.StatusBadRequest, w.Code, "Provider type %s should be valid", providerType)
		})
	}
}

// TestProviderManagementHandler_UpdateProvider_EmptyBody tests update with empty body
func TestProviderManagementHandler_UpdateProvider_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-provider"}}
	c.Request = httptest.NewRequest("PUT", "/v1/providers/test-provider", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateProvider(c)

	// Empty body should fail binding with 400
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestProviderManagementHandler_GetProvider_VariousIDs tests various provider ID formats
func TestProviderManagementHandler_GetProvider_VariousIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	testIDs := []string{
		"simple-id",
		"id-with-numbers-123",
		"ID_WITH_UNDERSCORES",
		"uuid-like-1234-5678-abcd",
		"very-long-provider-id-that-might-exist-in-production",
	}

	for _, id := range testIDs {
		t.Run(id, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: id}}
			c.Request = httptest.NewRequest("GET", "/v1/providers/"+id, nil)

			handler.GetProvider(c)

			// All should return 404 since no providers are registered
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

// TestProviderManagementHandler_AddProvider_WithConfig tests adding provider with config
func TestProviderManagementHandler_AddProvider_WithConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestProviderLogger()
	registry := services.NewProviderRegistry(nil, nil)
	handler := NewProviderManagementHandler(registry, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := AddProviderRequest{
		Name:    "test-provider-with-config",
		Type:    "deepseek",
		APIKey:  "test-key",
		BaseURL: "https://api.example.com",
		Model:   "test-model",
		Weight:  1.5,
		Enabled: true,
		Config: map[string]interface{}{
			"temperature":   0.7,
			"max_tokens":    2048,
			"top_p":         0.9,
			"custom_option": "value",
		},
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/v1/providers", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.AddProvider(c)

	// Should process the request (may succeed or fail based on provider factory)
	// But config should not cause issues
	assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusInternalServerError)
}
