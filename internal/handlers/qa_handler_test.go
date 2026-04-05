package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	helixqaadapter "dev.helix.agent/internal/adapters/helixqa"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupQAHandler(adapter *helixqaadapter.Adapter) (*QAHandler, *gin.Engine) {
	h := NewQAHandler(adapter)
	r := gin.New()
	api := r.Group("/v1")
	RegisterQARoutes(api, h)
	return h, r
}

func TestQAHandler_New_NilAdapter(t *testing.T) {
	t.Parallel()
	h := NewQAHandler(nil)
	require.NotNil(t, h)
	assert.Nil(t, h.adapter)
}

func TestQAHandler_New_WithAdapter(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	h := NewQAHandler(adapter)
	require.NotNil(t, h)
	assert.NotNil(t, h.adapter)
}

func TestQAHandler_ListPlatforms_NilAdapter(t *testing.T) {
	t.Parallel()
	_, r := setupQAHandler(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/qa/platforms", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "service_unavailable", resp["error"])
}

func TestQAHandler_ListPlatforms_Success(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, r := setupQAHandler(adapter)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/qa/platforms", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	platforms, ok := resp["platforms"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(platforms), 4)
}

func TestQAHandler_ListFindings_NilAdapter(t *testing.T) {
	t.Parallel()
	_, r := setupQAHandler(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/qa/findings", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestQAHandler_ListFindings_StoreNotInit(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, r := setupQAHandler(adapter)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/qa/findings", nil)
	r.ServeHTTP(w, req)

	// Store not initialized returns 500.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestQAHandler_ListFindings_EmptyStore(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	dbPath := t.TempDir() + "/test.db"
	require.NoError(t, adapter.Initialize(dbPath))
	defer adapter.Close()

	_, r := setupQAHandler(adapter)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/qa/findings", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["total"])
}

func TestQAHandler_GetFinding_NilAdapter(t *testing.T) {
	t.Parallel()
	_, r := setupQAHandler(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/qa/findings/HELIX-001", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestQAHandler_UpdateFinding_NilAdapter(t *testing.T) {
	t.Parallel()
	_, r := setupQAHandler(nil)

	body := `{"status":"fixed"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/v1/qa/findings/HELIX-001",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestQAHandler_UpdateFinding_InvalidJSON(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, r := setupQAHandler(adapter)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/v1/qa/findings/HELIX-001",
		bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQAHandler_StartSession_NilAdapter(t *testing.T) {
	t.Parallel()
	_, r := setupQAHandler(nil)

	body := `{"project_root":"/tmp","platforms":["api"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/qa/sessions",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestQAHandler_StartSession_InvalidJSON(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, r := setupQAHandler(adapter)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/qa/sessions",
		bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQAHandler_StartSession_MissingRequired(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, r := setupQAHandler(adapter)

	// Missing platforms field.
	body := `{"project_root":"/tmp"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/qa/sessions",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQAHandler_DiscoverKnowledge_NilAdapter(t *testing.T) {
	t.Parallel()
	_, r := setupQAHandler(nil)

	body := `{"project_root":"/tmp"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/qa/discover",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestQAHandler_DiscoverKnowledge_InvalidJSON(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, r := setupQAHandler(adapter)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/qa/discover",
		bytes.NewBufferString("{bad}"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQAHandler_DiscoverKnowledge_ValidRoot(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, r := setupQAHandler(adapter)

	tmpDir := t.TempDir()
	body, _ := json.Marshal(DiscoverKnowledgeRequest{
		ProjectRoot: tmpDir,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/qa/discover",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp KnowledgeSummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// Empty project has 0 docs and constraints.
	assert.Equal(t, 0, resp.DocsCount)
}

func TestRegisterQARoutes_AllEndpoints(t *testing.T) {
	t.Parallel()
	_, r := setupQAHandler(nil)

	routes := r.Routes()
	paths := make(map[string]bool)
	for _, route := range routes {
		paths[route.Method+" "+route.Path] = true
	}

	assert.True(t, paths["POST /v1/qa/sessions"],
		"POST /v1/qa/sessions should be registered")
	assert.True(t, paths["GET /v1/qa/findings"],
		"GET /v1/qa/findings should be registered")
	assert.True(t, paths["GET /v1/qa/findings/:id"],
		"GET /v1/qa/findings/:id should be registered")
	assert.True(t, paths["PUT /v1/qa/findings/:id"],
		"PUT /v1/qa/findings/:id should be registered")
	assert.True(t, paths["GET /v1/qa/platforms"],
		"GET /v1/qa/platforms should be registered")
	assert.True(t, paths["POST /v1/qa/discover"],
		"POST /v1/qa/discover should be registered")
}
