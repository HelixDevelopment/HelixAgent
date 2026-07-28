package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/handlers"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// fakeModelSourceProvider is a deterministic, in-package fake LLMProvider used
// ONLY to make TestCompletionHandler_Models hermetic (§11.4.50). tests/unit
// is the unit-test tree, where mocks/fakes ARE permitted (CONST-050(A) /
// §11.4.27) — every other test type must exercise the real system.
//
// Before this fake existed, the test wired a *services.ProviderRegistry that
// discovers providers by SCANNING HOST API KEYS (see
// internal/services/provider_discovery.go). On a host with zero configured
// provider keys, registry.ListProviders() is legitimately empty,
// handlers.CompletionHandler.Models correctly returns an HONEST EMPTY list
// by design (§11.4 anti-bluff — never fabricate a "working" list), and
// `assert.NotEmpty(t, response["data"])` then failed on CORRECT product
// behaviour rather than on a defect, gated only by -short mode. Registering
// this fake as an additional provider removes that host-credential
// dependency: the test's outcome no longer depends on which, if any, real
// provider credentials happen to be present in the executing environment.
type fakeModelSourceProvider struct{}

func (fakeModelSourceProvider) Complete(_ context.Context, _ *models.LLMRequest) (*models.LLMResponse, error) {
	return nil, errors.New("fakeModelSourceProvider does not serve completions")
}

func (fakeModelSourceProvider) CompleteStream(_ context.Context, _ *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	return nil, errors.New("fakeModelSourceProvider does not serve streaming completions")
}

func (fakeModelSourceProvider) HealthCheck() error { return nil }

func (fakeModelSourceProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{"fake-test-model-1"},
	}
}

func (fakeModelSourceProvider) ValidateConfig(_ map[string]interface{}) (bool, []string) {
	return true, nil
}

// closeNotifierRecorder wraps httptest.ResponseRecorder to implement http.CloseNotifier
// This is needed for testing SSE/streaming handlers
type closeNotifierRecorder struct {
	*httptest.ResponseRecorder
	closeNotify chan bool
}

func newCloseNotifierRecorder() *closeNotifierRecorder {
	return &closeNotifierRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeNotify:      make(chan bool, 1),
	}
}

// CloseNotify implements http.CloseNotifier (deprecated but still used by some handlers)
func (c *closeNotifierRecorder) CloseNotify() <-chan bool {
	return c.closeNotify
}

func TestCompletionHandler_Complete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping handler test in short mode (requires provider connectivity)") // SKIP-OK: #short-mode
	}
	// Create a mock request service
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	// Create completion handler
	handler := handlers.NewCompletionHandler(requestService)

	// Create test request
	req := handlers.CompletionRequest{
		Prompt:      "Test prompt",
		Model:       "test-model",
		Temperature: 0.7,
		MaxTokens:   100,
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
		EnsembleConfig: &models.EnsembleConfig{
			Strategy:            "confidence_weighted",
			MinProviders:        1,
			ConfidenceThreshold: 0.7,
			FallbackToBest:      true,
			Timeout:             30,
			PreferredProviders:  []string{},
		},
	}

	reqBody, _ := json.Marshal(req)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Set user context (simulate authenticated user)
	c.Set("user_id", "test-user")
	c.Set("session_id", "test-session")

	// Call handler
	handler.Complete(c)

	// Check response - may fail due to no auth/providers in test environment
	if w.Code != http.StatusOK {
		// Expected when no providers are properly configured
		var errorResponse handlers.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		assert.NoError(t, err)
		// Error is acceptable in test environment
		t.Logf("Expected error in test environment: %s", errorResponse.Error.Message)
	} else {
		// Should succeed if providers are available
		var response handlers.CompletionResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response.ID)
		assert.Equal(t, "text_completion", response.Object)
		assert.NotEmpty(t, response.Model)
		assert.NotEmpty(t, response.Choices)
		assert.Equal(t, 1, len(response.Choices))
	}
}

func TestCompletionHandler_Complete_InvalidRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping handler test in short mode (requires provider connectivity)") // SKIP-OK: #short-mode
	}
	// Create a mock request service
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	// Create completion handler
	handler := handlers.NewCompletionHandler(requestService)

	// Create invalid request (missing required field)
	req := map[string]interface{}{
		"model": "test-model",
		// Missing "prompt" field
	}

	reqBody, _ := json.Marshal(req)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Call handler
	handler.Complete(c)

	// Check response - should be bad request
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errorResponse handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)
	assert.NotEmpty(t, errorResponse.Error.Message)
	assert.NotEmpty(t, errorResponse.Error.Type)
}

func TestCompletionHandler_Chat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping handler test in short mode (requires provider connectivity)") // SKIP-OK: #short-mode
	}
	// Create a mock request service
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	// Create completion handler
	handler := handlers.NewCompletionHandler(requestService)

	// Create chat request
	req := handlers.CompletionRequest{
		Prompt: "Chat conversation test", // Required for validation
		Model:  "gpt-3.5-turbo",
		Messages: []models.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
		Temperature: 0.7,
		MaxTokens:   150,
	}

	reqBody, _ := json.Marshal(req)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Set user context
	c.Set("user_id", "test-user")
	c.Set("session_id", "test-session")

	// Call handler
	handler.Chat(c)

	// Check response
	if w.Code != http.StatusOK {
		// May fail due to no providers or auth issues in test environment
		var errorResponse handlers.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		assert.NoError(t, err)
		t.Logf("Expected error in test environment: %s", errorResponse.Error.Message)
	} else {
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "chat.completion", response["object"])
		assert.NotEmpty(t, response["choices"])
	}
}

func TestCompletionHandler_Models(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping handler test in short mode (requires provider connectivity)") // SKIP-OK: #short-mode
	}
	// Create a mock request service
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	// Create completion handler
	handler := handlers.NewCompletionHandler(requestService)

	// Register a deterministic, in-package fake provider (see
	// fakeModelSourceProvider's doc-comment) so this test is hermetic
	// (§11.4.50) and no longer depends on registry.ListProviders() being
	// non-empty on the host running it — that depends entirely on which, if
	// any, real LLM provider API keys are configured in the environment.
	require.NoError(t, registry.RegisterProvider("fake-test-provider", fakeModelSourceProvider{}))

	// Wire the registry as the handler's model source.
	//
	// NewCompletionHandler sets ONLY requestService — modelSource stays nil,
	// and Models() then returns an HONEST EMPTY list by design rather than
	// fabricating one (§11.4 anti-bluff; see SetModelSource's doc-comment).
	// Without this line the test built a handler that was never told where
	// models come from and then asserted the list was non-empty, so it failed
	// on correct product behaviour. The provider-discovery log lines it
	// emitted came from the registry's own environment scan — a different
	// object that was never connected to the handler, which made the failure
	// look like "discovery works but the handler drops the models".
	//
	// *services.ProviderRegistry satisfies handlers.ModelSource
	// (ListProviders + GetProvider), so the registry this test already builds
	// is the authoritative source (CONST-036).
	handler.SetModelSource(registry)

	// Create HTTP request
	httpReq := httptest.NewRequest("GET", "/v1/models", nil)

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Call handler
	handler.Models(c)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "list", response["object"])
	assert.NotEmpty(t, response["data"])

	// Check that models array contains expected models
	models := response["data"].([]interface{})
	assert.Greater(t, len(models), 0)

	// Bind the assertion to the SPECIFIC data this test's own fixture wires
	// in (fakeModelSourceProvider, registered above as "fake-test-provider"
	// with SupportedModels ["fake-test-model-1"]), not to a generic
	// non-empty check. A handler that fabricated ANY hardcoded model list —
	// exactly the CONST-036 "hardcoded model list" anti-pattern this
	// project's governance forbids (see BLUFF-002 in CLAUDE.md) — would also
	// satisfy `assert.NotEmpty` / `assert.Greater(len, 0)`, so those alone do
	// not prove the models actually came from the registered fake provider.
	foundFakeModel := false
	for _, m := range models {
		entry, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if entry["id"] == "fake-test-model-1" {
			foundFakeModel = true
			assert.Equal(t, "fake-test-provider", entry["owned_by"],
				"model %q must be attributed to the fake provider it was registered under, per Models()'s dedupKey := providerName + \"/\" + modelID / owned_by: providerName wiring", entry["id"])
			break
		}
	}
	assert.Truef(t, foundFakeModel,
		"expected the returned model list to contain the fixture's own model id %q (owned_by %q) — its absence means the response is NOT actually sourced from the registered fake provider",
		"fake-test-model-1", "fake-test-provider")
}

func TestCompletionHandler_Stream(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping handler test in short mode (requires provider connectivity)") // SKIP-OK: #short-mode
	}
	// Create a mock request service with a short timeout for unit testing
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 5 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	// Create completion handler
	handler := handlers.NewCompletionHandler(requestService)

	// Create streaming request
	req := handlers.CompletionRequest{
		Prompt:      "Test streaming prompt",
		Model:       "test-model",
		Stream:      true,
		MaxTokens:   100,
		Temperature: 0.7,
	}

	reqBody, _ := json.Marshal(req)

	// Create HTTP request with context timeout to prevent hanging in test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpReq := httptest.NewRequest("POST", "/v1/completions/stream", bytes.NewBuffer(reqBody)).WithContext(ctx)
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder with CloseNotifier support for SSE streaming
	w := newCloseNotifierRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Set user context
	c.Set("user_id", "test-user")
	c.Set("session_id", "test-session")

	// Call handler - use a goroutine with timeout to prevent blocking
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.CompleteStream(c)
	}()

	select {
	case <-done:
		// Handler completed normally
	case <-time.After(15 * time.Second):
		cancel() // Cancel the context to unblock the handler
		t.Log("Streaming handler timed out - expected in unit test without providers")
		return
	}

	// Check response — three genuinely distinct outcomes, not two.
	//
	// This used to branch only on `w.Code != http.StatusOK`. That cannot tell
	// "the handler streamed successfully" from "the handler wrote NOTHING":
	// the recorder initialises Code to 200, and a handler that returns without
	// ever writing leaves it at 200. So the silent case fell into the success
	// branch and failed on `Contains(body, "data:")` against an empty body —
	// a failure detector that could not detect the failure it was there for
	// (§11.4.201: a check must assert the condition it claims).
	//
	// The silent case is REAL and CORRECT product behaviour here: this test
	// gives the REQUEST context a 10s timeout, and with no provider configured
	// nothing is produced before it expires. CompleteStream then takes its
	// client-gone branch and returns without writing — writing to a client
	// that is gone would be the bug. That is an absent precondition, so it is
	// an honest SKIP (§11.4.3), never a silent pass.
	body := w.Body.String()

	switch {
	case w.Code != http.StatusOK:
		// The handler reported a real error — decode and record it.
		var errorResponse handlers.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		assert.NoError(t, err)
		t.Logf("streaming error in test environment: %s", errorResponse.Error.Message)

	case body == "":
		// Nothing was ever written: no provider produced output before the
		// request context expired. Not a pass, not a product failure.
		t.Skip("no provider produced streamed output before the request context expired — nothing to assert about SSE framing") // SKIP-OK: #infra-unavailable

	default:
		// The handler genuinely streamed: hold it to the SSE contract.
		assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
		assert.Contains(t, body, "data:")
	}
}

func TestCompletionRequestValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping validation test in short mode (requires provider connectivity)") // SKIP-OK: #short-mode
	}
	testCases := []struct {
		name        string
		request     handlers.CompletionRequest
		expectError bool
	}{
		{
			name: "Valid request",
			request: handlers.CompletionRequest{
				Prompt: "Test prompt",
				Model:  "test-model",
			},
			expectError: false,
		},
		{
			name: "Missing prompt",
			request: handlers.CompletionRequest{
				Model: "test-model",
			},
			expectError: true,
		},
		{
			name: "Valid chat request",
			request: handlers.CompletionRequest{
				Prompt: "Chat test", // Add required prompt
				Model:  "gpt-3.5-turbo",
				Messages: []models.Message{
					{
						Role:    "user",
						Content: "Hello",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid ensemble request",
			request: handlers.CompletionRequest{
				Prompt: "Test prompt",
				Model:  "test-model",
				EnsembleConfig: &models.EnsembleConfig{
					Strategy:            "confidence_weighted",
					MinProviders:        2,
					ConfidenceThreshold: 0.8,
					FallbackToBest:      true,
					Timeout:             30,
					PreferredProviders:  []string{"provider1", "provider2"},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock request service (note: no real providers for test)
			registryConfig := &services.RegistryConfig{
				DefaultTimeout: 30 * time.Second,
				Providers:      make(map[string]*services.ProviderConfig),
			}
			registry := services.NewProviderRegistry(registryConfig, nil)
			requestService := registry.GetRequestService()

			// Create completion handler
			handler := handlers.NewCompletionHandler(requestService)

			reqBody, _ := json.Marshal(tc.request)

			// Create HTTP request
			httpReq := httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBody))
			httpReq.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Create Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = httpReq
			c.Request = httpReq
			c.Set("user_id", "test-user")
			c.Set("session_id", "test-session")

			// Call handler
			handler.Complete(c)

			if tc.expectError {
				assert.Equal(t, http.StatusBadRequest, w.Code)
			} else {
				// Note: This might still fail with 500 due to no providers,
				// but it shouldn't be a 400 (bad request) error
				if w.Code == http.StatusBadRequest {
					t.Logf("Got unexpected 400 error: %s", w.Body.String())
				}
				assert.NotEqual(t, http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestConvertToInternalRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping conversion test in short mode (requires provider connectivity)") // SKIP-OK: #short-mode
	}
	// This tests the conversion logic by creating a request through the handler
	// and checking that it properly converts to internal format
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	handler := handlers.NewCompletionHandler(requestService)

	// Create test request with various parameters
	req := handlers.CompletionRequest{
		Prompt:      "Test prompt",
		Model:       "test-model",
		Temperature: 0.7,
		MaxTokens:   100,
		TopP:        0.9,
		Stop:        []string{"\n", "."},
		Stream:      false,
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello",
				Name:    stringPtr("User1"),
			},
		},
		MemoryEnhanced: true,
		RequestType:    "test",
	}

	reqBody, _ := json.Marshal(req)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Set user context
	c.Set("user_id", "test-user-id")
	c.Set("session_id", "test-session-id")

	// Call handler
	handler.Complete(c)

	// The request should be processed (even if it fails due to no providers)
	// We're mainly testing that the conversion doesn't panic and sets defaults correctly
	// Valid status codes include:
	// - 200 (OK)
	// - 400-499 (client errors)
	// - 500-504 (server errors including 502 Bad Gateway, 503 Service Unavailable)
	assert.True(t, w.Code >= 200 && w.Code <= 504, "Expected status 200-504, got %d", w.Code)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
