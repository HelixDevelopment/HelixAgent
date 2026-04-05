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
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupBrowserRouter creates a BrowserHandler with nil manager (tests
// exercise JSON-binding / validation paths that run before Execute).
func setupBrowserRouter() (*BrowserHandler, *gin.Engine) {
	handler := NewBrowserHandler(nil)
	router := gin.New()
	handler.RegisterRoutes(router)
	return handler, router
}

// --- Constructor -------------------------------------------------------

func TestNewBrowserHandler(t *testing.T) {
	h := NewBrowserHandler(nil)
	assert.NotNil(t, h)
}

// --- Navigate ----------------------------------------------------------

func TestBrowserHandler_Navigate_InvalidJSON(t *testing.T) {
	_, router := setupBrowserRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/browser/navigate", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBrowserHandler_Navigate_MissingURL(t *testing.T) {
	_, router := setupBrowserRouter()

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/v1/browser/navigate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "URL")
}

// --- Click -------------------------------------------------------------

func TestBrowserHandler_Click_InvalidJSON(t *testing.T) {
	_, router := setupBrowserRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/browser/click", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBrowserHandler_Click_MissingSelector(t *testing.T) {
	_, router := setupBrowserRouter()

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/v1/browser/click", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "Selector")
}

// --- Type --------------------------------------------------------------

func TestBrowserHandler_Type_InvalidJSON(t *testing.T) {
	_, router := setupBrowserRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/browser/type", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBrowserHandler_Type_MissingFields(t *testing.T) {
	tests := []struct {
		name    string
		body    TypeRequest
		errText string
	}{
		{
			name:    "missing_selector",
			body:    TypeRequest{Text: "hello"},
			errText: "Selector",
		},
		{
			name:    "missing_text",
			body:    TypeRequest{Selector: "#input"},
			errText: "Text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, router := setupBrowserRouter()

			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/v1/browser/type", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Contains(t, resp["error"], tt.errText)
		})
	}
}

// --- Screenshot --------------------------------------------------------

func TestBrowserHandler_Screenshot_InvalidJSON(t *testing.T) {
	_, router := setupBrowserRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/browser/screenshot", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Extract -----------------------------------------------------------

func TestBrowserHandler_Extract_InvalidJSON(t *testing.T) {
	_, router := setupBrowserRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/browser/extract", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBrowserHandler_Extract_MissingSelector(t *testing.T) {
	_, router := setupBrowserRouter()

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/v1/browser/extract", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "Selector")
}

// --- Evaluate ----------------------------------------------------------

func TestBrowserHandler_Evaluate_InvalidJSON(t *testing.T) {
	_, router := setupBrowserRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/browser/evaluate", bytes.NewBufferString("nope"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBrowserHandler_Evaluate_MissingScript(t *testing.T) {
	_, router := setupBrowserRouter()

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/v1/browser/evaluate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "Script")
}

// --- Request type JSON round-trips -------------------------------------

func TestNavigateRequest_JSONRoundTrip(t *testing.T) {
	orig := NavigateRequest{
		URL:     "https://example.com",
		WaitFor: "#main",
		Timeout: 10,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded NavigateRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestClickRequest_JSONRoundTrip(t *testing.T) {
	orig := ClickRequest{Selector: "#btn", Button: "left"}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded ClickRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestTypeRequest_JSONRoundTrip(t *testing.T) {
	orig := TypeRequest{Selector: "#input", Text: "hello", Clear: true}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded TypeRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestScreenshotRequest_JSONRoundTrip(t *testing.T) {
	orig := ScreenshotRequest{Selector: "#hero", FullPage: true}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded ScreenshotRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestExtractRequest_JSONRoundTrip(t *testing.T) {
	orig := ExtractRequest{Selector: "h1", Type: "text", Attribute: ""}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded ExtractRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestEvaluateRequest_JSONRoundTrip(t *testing.T) {
	orig := EvaluateRequest{Script: "return 1+1"}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded EvaluateRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestNavigateResponse_JSONRoundTrip(t *testing.T) {
	orig := NavigateResponse{Success: true, URL: "https://example.com", Title: "Example"}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded NavigateResponse
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestExtractResponse_JSONRoundTrip(t *testing.T) {
	orig := ExtractResponse{Content: "Hello World", Selector: "h1"}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded ExtractResponse
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

// --- RegisterRoutes verification ---------------------------------------

func TestBrowserHandler_RegisterRoutes(t *testing.T) {
	handler := NewBrowserHandler(nil)
	router := gin.New()
	handler.RegisterRoutes(router)

	routes := router.Routes()
	expected := map[string]string{
		"POST:/v1/browser/navigate":   "",
		"POST:/v1/browser/click":      "",
		"POST:/v1/browser/type":       "",
		"POST:/v1/browser/screenshot": "",
		"POST:/v1/browser/extract":    "",
		"POST:/v1/browser/evaluate":   "",
	}

	for _, r := range routes {
		key := r.Method + ":" + r.Path
		delete(expected, key)
	}

	assert.Empty(t, expected, "some expected routes were not registered: %v", expected)
}
