// Package testing provides utilities for testing AI providers.
package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
)

// MockHTTPClient is a mock HTTP client for testing.
type MockHTTPClient struct {
	responses map[string]*MockResponse
}

// MockResponse represents a mock HTTP response.
type MockResponse struct {
	StatusCode int
	Body       interface{}
	Headers    map[string]string
}

// NewMockHTTPClient creates a new mock HTTP client.
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		responses: make(map[string]*MockResponse),
	}
}

// AddResponse adds a mock response for a given URL and method.
func (m *MockHTTPClient) AddResponse(method, url string, response *MockResponse) {
	key := method + " " + url
	m.responses[key] = response
}

// Do simulates an HTTP request.
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	key := req.Method + " " + req.URL.String()
	mockResp, exists := m.responses[key]
	if !exists {
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(bytes.NewReader([]byte("Not found"))),
		}, nil
	}

	var body io.ReadCloser
	if mockResp.Body != nil {
		bodyBytes, err := json.Marshal(mockResp.Body)
		if err != nil {
			return nil, err
		}
		body = io.NopCloser(bytes.NewReader(bodyBytes))
	} else {
		body = io.NopCloser(bytes.NewReader([]byte{}))
	}

	resp := &http.Response{
		StatusCode: mockResp.StatusCode,
		Body:       body,
		Header:     make(http.Header),
	}

	for k, v := range mockResp.Headers {
		resp.Header.Set(k, v)
	}

	return resp, nil
}

// TestServer creates a test HTTP server.
type TestServer struct {
	server *httptest.Server
}

// NewTestServer creates a new test server.
func NewTestServer(handler http.Handler) *TestServer {
	server := httptest.NewServer(handler)
	return &TestServer{server: server}
}

// URL returns the test server URL.
func (ts *TestServer) URL() string {
	return ts.server.URL
}

// Close closes the test server.
func (ts *TestServer) Close() {
	ts.server.Close()
}

// MockProvider is a mock provider for testing.
type MockProvider struct {
	name        string
	chatResp    toolkit.ChatResponse
	embedResp   toolkit.EmbeddingResponse
	rerankResp  toolkit.RerankResponse
	models      []toolkit.ModelInfo
	shouldError bool
}

// NewMockProvider creates a new mock provider.
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name: name,
		models: []toolkit.ModelInfo{
			{
				ID:   "test-model",
				Name: "Test Model",
				Capabilities: toolkit.ModelCapabilities{
					SupportsChat: true,
				},
				Provider: name,
			},
		},
	}
}

// SetChatResponse sets the mock chat response.
func (mp *MockProvider) SetChatResponse(resp toolkit.ChatResponse) {
	mp.chatResp = resp
}

// SetEmbeddingResponse sets the mock embedding response.
func (mp *MockProvider) SetEmbeddingResponse(resp toolkit.EmbeddingResponse) {
	mp.embedResp = resp
}

// SetRerankResponse sets the mock rerank response.
func (mp *MockProvider) SetRerankResponse(resp toolkit.RerankResponse) {
	mp.rerankResp = resp
}

// SetModels sets the mock models.
func (mp *MockProvider) SetModels(models []toolkit.ModelInfo) {
	mp.models = models
}

// SetShouldError sets whether the provider should return errors.
func (mp *MockProvider) SetShouldError(shouldError bool) {
	mp.shouldError = shouldError
}

// Name returns the provider name.
func (mp *MockProvider) Name() string {
	return mp.name
}

// Chat performs a chat completion request.
func (mp *MockProvider) Chat(ctx context.Context, req toolkit.ChatRequest) (toolkit.ChatResponse, error) {
	if mp.shouldError {
		return toolkit.ChatResponse{}, &TestError{Message: "mock chat error"}
	}
	return mp.chatResp, nil
}

// Embed performs an embedding request.
func (mp *MockProvider) Embed(ctx context.Context, req toolkit.EmbeddingRequest) (toolkit.EmbeddingResponse, error) {
	if mp.shouldError {
		return toolkit.EmbeddingResponse{}, &TestError{Message: "mock embed error"}
	}
	return mp.embedResp, nil
}

// Rerank performs a rerank request.
func (mp *MockProvider) Rerank(ctx context.Context, req toolkit.RerankRequest) (toolkit.RerankResponse, error) {
	if mp.shouldError {
		return toolkit.RerankResponse{}, &TestError{Message: "mock rerank error"}
	}
	return mp.rerankResp, nil
}

// DiscoverModels discovers available models.
func (mp *MockProvider) DiscoverModels(ctx context.Context) ([]toolkit.ModelInfo, error) {
	if mp.shouldError {
		return nil, &TestError{Message: "mock discover error"}
	}
	return mp.models, nil
}

// ValidateConfig validates the provider configuration.
func (mp *MockProvider) ValidateConfig(config map[string]interface{}) error {
	if mp.shouldError {
		return &TestError{Message: "mock validation error"}
	}
	return nil
}

// TestError represents a test error.
type TestError struct {
	Message string
}

func (e *TestError) Error() string {
	return e.Message
}

// TestFixtures provides common test fixtures.
type TestFixtures struct{}

// NewTestFixtures creates new test fixtures.
func NewTestFixtures() *TestFixtures {
	return &TestFixtures{}
}

// ChatRequest returns a sample chat request.
func (tf *TestFixtures) ChatRequest() toolkit.ChatRequest {
	return toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{
				Role:    "user",
				Content: "Hello, world!",
			},
		},
		MaxTokens: 100,
	}
}

// ChatResponse returns a sample chat response.
func (tf *TestFixtures) ChatResponse() toolkit.ChatResponse {
	return toolkit.ChatResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "test-model",
		Choices: []toolkit.Choice{
			{
				Index: 0,
				Message: toolkit.ChatMessage{
					Role:    "assistant",
					Content: "Hello! How can I help you?",
				},
				FinishReason: "stop",
			},
		},
		Usage: toolkit.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}
}

// EmbeddingRequest returns a sample embedding request.
func (tf *TestFixtures) EmbeddingRequest() toolkit.EmbeddingRequest {
	return toolkit.EmbeddingRequest{
		Model: "test-embedding-model",
		Input: []string{"Hello, world!"},
	}
}

// EmbeddingResponse returns a sample embedding response.
func (tf *TestFixtures) EmbeddingResponse() toolkit.EmbeddingResponse {
	return toolkit.EmbeddingResponse{
		Object: "list",
		Data: []toolkit.EmbeddingData{
			{
				Object:    "embedding",
				Embedding: []float64{0.1, 0.2, 0.3},
				Index:     0,
			},
		},
		Model: "test-embedding-model",
		Usage: toolkit.Usage{
			PromptTokens:     5,
			CompletionTokens: 0,
			TotalTokens:      5,
		},
	}
}

// RerankRequest returns a sample rerank request.
func (tf *TestFixtures) RerankRequest() toolkit.RerankRequest {
	return toolkit.RerankRequest{
		Model:     "test-rerank-model",
		Query:     "test query",
		Documents: []string{"doc1", "doc2"},
		TopN:      2,
	}
}

// RerankResponse returns a sample rerank response.
func (tf *TestFixtures) RerankResponse() toolkit.RerankResponse {
	return toolkit.RerankResponse{
		Object: "list",
		Model:  "test-rerank-model",
		Results: []toolkit.RerankResult{
			{
				Index:    0,
				Score:    0.9,
				Document: "doc1",
			},
			{
				Index:    1,
				Score:    0.8,
				Document: "doc2",
			},
		},
	}
}

// ModelInfo returns a sample model info.
func (tf *TestFixtures) ModelInfo() toolkit.ModelInfo {
	return toolkit.ModelInfo{
		ID:   "test-model",
		Name: "Test Model",
		Capabilities: toolkit.ModelCapabilities{
			SupportsChat:      true,
			SupportsEmbedding: false,
			ContextWindow:     4096,
			MaxTokens:         2048,
		},
		Provider:    "test-provider",
		Description: "A test model",
	}
}

// AssertChatResponse asserts that a chat response matches expected values.
func AssertChatResponse(t *testing.T, actual, expected toolkit.ChatResponse) {
	t.Helper()

	if actual.ID != expected.ID {
		t.Errorf("ID mismatch: got %s, want %s", actual.ID, expected.ID)
	}
	if actual.Model != expected.Model {
		t.Errorf("Model mismatch: got %s, want %s", actual.Model, expected.Model)
	}
	if len(actual.Choices) != len(expected.Choices) {
		t.Errorf("Choices length mismatch: got %d, want %d", len(actual.Choices), len(expected.Choices))
	}
	// Add more assertions as needed
}

// AssertEmbeddingResponse asserts that an embedding response matches expected values.
func AssertEmbeddingResponse(t *testing.T, actual, expected toolkit.EmbeddingResponse) {
	t.Helper()

	if actual.Model != expected.Model {
		t.Errorf("Model mismatch: got %s, want %s", actual.Model, expected.Model)
	}
	if len(actual.Data) != len(expected.Data) {
		t.Errorf("Data length mismatch: got %d, want %d", len(actual.Data), len(expected.Data))
	}
	// Add more assertions as needed
}

// AssertRerankResponse asserts that a rerank response matches expected values.
func AssertRerankResponse(t *testing.T, actual, expected toolkit.RerankResponse) {
	t.Helper()

	if actual.Model != expected.Model {
		t.Errorf("Model mismatch: got %s, want %s", actual.Model, expected.Model)
	}
	if len(actual.Results) != len(expected.Results) {
		t.Errorf("Results length mismatch: got %d, want %d", len(actual.Results), len(expected.Results))
	}
	// Add more assertions as needed
}
