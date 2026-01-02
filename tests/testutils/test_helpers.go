package testutils

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
)

func init() {
	// Set Gin to test mode by default
	gin.SetMode(gin.TestMode)
}

// TestContext provides a test context with cancel
type TestContext struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

// NewTestContext creates a new test context with timeout
func NewTestContext(timeout time.Duration) *TestContext {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return &TestContext{Ctx: ctx, Cancel: cancel}
}

// Cleanup cancels the context
func (tc *TestContext) Cleanup() {
	tc.Cancel()
}

// TestRouter provides a test router setup
type TestRouter struct {
	Engine *gin.Engine
	T      *testing.T
}

// NewTestRouter creates a new test router
func NewTestRouter(t *testing.T) *TestRouter {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	return &TestRouter{Engine: engine, T: t}
}

// AddRoute adds a route to the test router
func (tr *TestRouter) AddRoute(method, path string, handler gin.HandlerFunc) {
	switch method {
	case http.MethodGet:
		tr.Engine.GET(path, handler)
	case http.MethodPost:
		tr.Engine.POST(path, handler)
	case http.MethodPut:
		tr.Engine.PUT(path, handler)
	case http.MethodDelete:
		tr.Engine.DELETE(path, handler)
	case http.MethodPatch:
		tr.Engine.PATCH(path, handler)
	}
}

// Request performs a request to the test router
func (tr *TestRouter) Request(method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(tr.T, err)
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, path, reqBody)
	require.NoError(tr.T, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	tr.Engine.ServeHTTP(recorder, req)
	return recorder
}

// GET performs a GET request
func (tr *TestRouter) GET(path string, headers map[string]string) *httptest.ResponseRecorder {
	return tr.Request(http.MethodGet, path, nil, headers)
}

// POST performs a POST request
func (tr *TestRouter) POST(path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	return tr.Request(http.MethodPost, path, body, headers)
}

// PUT performs a PUT request
func (tr *TestRouter) PUT(path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	return tr.Request(http.MethodPut, path, body, headers)
}

// DELETE performs a DELETE request
func (tr *TestRouter) DELETE(path string, headers map[string]string) *httptest.ResponseRecorder {
	return tr.Request(http.MethodDelete, path, nil, headers)
}

// AssertStatus asserts the response status code
func (tr *TestRouter) AssertStatus(recorder *httptest.ResponseRecorder, expected int) {
	assert.Equal(tr.T, expected, recorder.Code, "Unexpected status code. Response body: %s", recorder.Body.String())
}

// AssertJSON asserts the response is valid JSON and returns decoded data
func (tr *TestRouter) AssertJSON(recorder *httptest.ResponseRecorder) map[string]interface{} {
	var result map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &result)
	require.NoError(tr.T, err, "Response is not valid JSON: %s", recorder.Body.String())
	return result
}

// AssertJSONArray asserts the response is a valid JSON array
func (tr *TestRouter) AssertJSONArray(recorder *httptest.ResponseRecorder) []interface{} {
	var result []interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &result)
	require.NoError(tr.T, err, "Response is not valid JSON array: %s", recorder.Body.String())
	return result
}

// NewTestConfig creates a minimal test configuration
func NewTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host:      "localhost",
			Port:      "8080",
			JWTSecret: "test-secret-key-for-testing-purposes-only-1234567890",
		},
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "test",
			Password: "test",
			Name:     "test_db",
		},
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
		},
		LLM: config.LLMConfig{
			DefaultTimeout: 30 * time.Second,
			MaxRetries:     3,
			Providers:      make(map[string]config.ProviderConfig),
		},
		ModelsDev: config.ModelsDevConfig{
			Enabled: false,
		},
		MCP: config.MCPConfig{
			Enabled: false,
		},
		Cognee: config.CogneeConfig{
			Enabled: false,
		},
	}
}

// NewTestConfigWithJWTSecret creates a test config with custom JWT secret
func NewTestConfigWithJWTSecret(secret string) *config.Config {
	cfg := NewTestConfig()
	cfg.Server.JWTSecret = secret
	return cfg
}

// NewTestConfigWithProviders creates a test config with providers
func NewTestConfigWithProviders(providers map[string]config.ProviderConfig) *config.Config {
	cfg := NewTestConfig()
	cfg.LLM.Providers = providers
	return cfg
}

// CreateTestLLMRequest creates a test LLM request
func CreateTestLLMRequest(prompt string) *models.LLMRequest {
	return &models.LLMRequest{
		ID:        "test-request-" + time.Now().Format("20060102150405"),
		SessionID: "test-session",
		UserID:    "test-user",
		Prompt:    prompt,
		ModelParams: models.ModelParameters{
			Model:       "test-model",
			Temperature: 0.7,
			MaxTokens:   100,
			TopP:        0.9,
		},
		Status:      "pending",
		CreatedAt:   time.Now(),
		RequestType: "completion",
	}
}

// CreateTestLLMResponse creates a test LLM response
func CreateTestLLMResponse(requestID, content string) *models.LLMResponse {
	return &models.LLMResponse{
		RequestID:      requestID,
		ProviderID:     "test-provider",
		ProviderName:   "test",
		Content:        content,
		Confidence:     0.9,
		TokensUsed:     50,
		ResponseTime:   100,
		FinishReason:   "stop",
		Selected:       true,
		SelectionScore: 0.9,
		CreatedAt:      time.Now(),
	}
}

// CreateTestChatRequest creates a test chat request with messages
func CreateTestChatRequest(messages []models.Message) *models.LLMRequest {
	return &models.LLMRequest{
		ID:        "test-chat-" + time.Now().Format("20060102150405"),
		SessionID: "test-session",
		UserID:    "test-user",
		Messages:  messages,
		ModelParams: models.ModelParameters{
			Model:       "test-model",
			Temperature: 0.7,
			MaxTokens:   100,
		},
		Status:      "pending",
		CreatedAt:   time.Now(),
		RequestType: "chat",
	}
}

// CreateTestMessages creates test chat messages
func CreateTestMessages(userMessage string) []models.Message {
	return []models.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: userMessage},
	}
}

// AssertErrorContains asserts that an error contains a specific message
func AssertErrorContains(t *testing.T, err error, expectedMsg string) {
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedMsg)
}

// AssertNoError asserts that no error occurred
func AssertNoError(t *testing.T, err error) {
	require.NoError(t, err)
}

// WaitForCondition waits for a condition to be true
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, checkInterval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(checkInterval)
	}
	return false
}

// CreateAuthHeaders creates authorization headers for testing
func CreateAuthHeaders(token string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + token,
	}
}

// CreateAPIKeyHeaders creates API key headers for testing
func CreateAPIKeyHeaders(apiKey string) map[string]string {
	return map[string]string{
		"X-API-Key": apiKey,
	}
}

// SkipIfShort skips the test if running in short mode
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
}

// SkipIfNoDatabase skips the test if database is not available
func SkipIfNoDatabase(t *testing.T) {
	// Check if database connection can be established
	// For now, just skip in short mode
	if testing.Short() {
		t.Skip("Skipping test that requires database connection")
	}
}

// SkipIfNoRedis skips the test if Redis is not available
func SkipIfNoRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test that requires Redis connection")
	}
}

// JSONMarshal marshals to JSON and panics on error
func JSONMarshal(t *testing.T, v interface{}) []byte {
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

// JSONUnmarshal unmarshals from JSON and panics on error
func JSONUnmarshal(t *testing.T, data []byte, v interface{}) {
	err := json.Unmarshal(data, v)
	require.NoError(t, err)
}
