// Package handlers provides HTTP handler tests
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dev.helix.agent/internal/templates"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateHandler_ListTemplates(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Create a temporary template manager
	config := templates.ManagerConfig{
		TemplatesDir: t.TempDir(),
		MaxTemplates: 100,
	}
	manager, err := templates.NewManager(config)
	require.NoError(t, err)

	handler := NewTemplateHandler(manager)
	router := gin.New()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/templates", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListTemplatesResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(resp.Templates), 4) // At least built-in templates
}

func TestTemplateHandler_GetTemplate(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	config := templates.ManagerConfig{
		TemplatesDir: t.TempDir(),
		MaxTemplates: 100,
	}
	manager, err := templates.NewManager(config)
	require.NoError(t, err)

	handler := NewTemplateHandler(manager)
	router := gin.New()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/templates/onboarding", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp templates.ContextTemplate
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "onboarding", resp.Metadata.ID)
}

func TestTemplateHandler_ApplyTemplate(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	config := templates.ManagerConfig{
		TemplatesDir: t.TempDir(),
		MaxTemplates: 100,
	}
	manager, err := templates.NewManager(config)
	require.NoError(t, err)

	handler := NewTemplateHandler(manager)
	router := gin.New()
	handler.RegisterRoutes(router)

	applyReq := ApplyTemplateRequest{
		TemplateID: "onboarding",
		Variables:  map[string]string{},
	}

	body, _ := json.Marshal(applyReq)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/templates/apply", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ApplyTemplateResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Instructions)
}

func TestTemplateHandler_GetTemplate_NotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	config := templates.ManagerConfig{
		TemplatesDir: t.TempDir(),
		MaxTemplates: 100,
	}
	manager, err := templates.NewManager(config)
	require.NoError(t, err)

	handler := NewTemplateHandler(manager)
	router := gin.New()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/templates/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
