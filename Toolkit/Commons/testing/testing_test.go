package testing

import (
	"context"
	"net/http"
	"testing"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
)

func TestNewMockHTTPClient(t *testing.T) {
	client := NewMockHTTPClient()

	if client.responses == nil {
		t.Error("Expected responses map to be initialized")
	}
}

func TestMockHTTPClient_AddResponse(t *testing.T) {
	client := NewMockHTTPClient()

	response := &MockResponse{
		StatusCode: 200,
		Body:       map[string]string{"message": "success"},
		Headers:    map[string]string{"Content-Type": "application/json"},
	}

	client.AddResponse("GET", "https://api.example.com/test", response)

	key := "GET https://api.example.com/test"
	if client.responses[key] != response {
		t.Error("Expected response to be stored")
	}
}

func TestMockHTTPClient_Do_Success(t *testing.T) {
	client := NewMockHTTPClient()

	response := &MockResponse{
		StatusCode: 200,
		Body:       map[string]string{"message": "success"},
		Headers:    map[string]string{"Content-Type": "application/json"},
	}

	client.AddResponse("GET", "https://api.example.com/test", response)

	req, _ := http.NewRequest("GET", "https://api.example.com/test", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type header, got %s", resp.Header.Get("Content-Type"))
	}
}

func TestMockHTTPClient_Do_NotFound(t *testing.T) {
	client := NewMockHTTPClient()

	req, _ := http.NewRequest("GET", "https://api.example.com/missing", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.StatusCode != 404 {
		t.Errorf("Expected status code 404, got %d", resp.StatusCode)
	}
}

func TestNewTestServer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := NewTestServer(handler)

	if server.server == nil {
		t.Error("Expected server to be initialized")
	}

	if server.URL() == "" {
		t.Error("Expected non-empty URL")
	}

	server.Close()
}

func TestNewMockProvider(t *testing.T) {
	provider := NewMockProvider("test-provider")

	if provider.name != "test-provider" {
		t.Errorf("Expected name 'test-provider', got %s", provider.name)
	}

	if len(provider.models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(provider.models))
	}

	if provider.models[0].ID != "test-model" {
		t.Errorf("Expected model ID 'test-model', got %s", provider.models[0].ID)
	}
}

func TestMockProvider_Name(t *testing.T) {
	provider := NewMockProvider("test-provider")

	if provider.Name() != "test-provider" {
		t.Errorf("Expected name 'test-provider', got %s", provider.Name())
	}
}

func TestMockProvider_Chat(t *testing.T) {
	provider := NewMockProvider("test-provider")

	expectedResp := toolkit.ChatResponse{
		ID:    "test-chat-id",
		Model: "test-model",
	}

	provider.SetChatResponse(expectedResp)

	req := toolkit.ChatRequest{Model: "test-model"}
	resp, err := provider.Chat(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.ID != expectedResp.ID {
		t.Errorf("Expected ID %s, got %s", expectedResp.ID, resp.ID)
	}
}

func TestMockProvider_Chat_Error(t *testing.T) {
	provider := NewMockProvider("test-provider")
	provider.SetShouldError(true)

	req := toolkit.ChatRequest{Model: "test-model"}
	_, err := provider.Chat(context.Background(), req)

	if err == nil {
		t.Error("Expected error")
	}

	if err.Error() != "mock chat error" {
		t.Errorf("Expected 'mock chat error', got %s", err.Error())
	}
}

func TestMockProvider_Embed(t *testing.T) {
	provider := NewMockProvider("test-provider")

	expectedResp := toolkit.EmbeddingResponse{
		Model: "test-embedding-model",
		Data: []toolkit.EmbeddingData{
			{Embedding: []float64{0.1, 0.2, 0.3}},
		},
	}

	provider.SetEmbeddingResponse(expectedResp)

	req := toolkit.EmbeddingRequest{Model: "test-embedding-model"}
	resp, err := provider.Embed(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Model != expectedResp.Model {
		t.Errorf("Expected model %s, got %s", expectedResp.Model, resp.Model)
	}
}

func TestMockProvider_Embed_Error(t *testing.T) {
	provider := NewMockProvider("test-provider")
	provider.SetShouldError(true)

	req := toolkit.EmbeddingRequest{Model: "test-embedding-model"}
	_, err := provider.Embed(context.Background(), req)

	if err == nil {
		t.Error("Expected error")
	}

	if err.Error() != "mock embed error" {
		t.Errorf("Expected 'mock embed error', got %s", err.Error())
	}
}

func TestMockProvider_Rerank(t *testing.T) {
	provider := NewMockProvider("test-provider")

	expectedResp := toolkit.RerankResponse{
		Model: "test-rerank-model",
		Results: []toolkit.RerankResult{
			{Index: 0, Score: 0.9},
		},
	}

	provider.SetRerankResponse(expectedResp)

	req := toolkit.RerankRequest{Model: "test-rerank-model"}
	resp, err := provider.Rerank(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Model != expectedResp.Model {
		t.Errorf("Expected model %s, got %s", expectedResp.Model, resp.Model)
	}
}

func TestMockProvider_Rerank_Error(t *testing.T) {
	provider := NewMockProvider("test-provider")
	provider.SetShouldError(true)

	req := toolkit.RerankRequest{Model: "test-rerank-model"}
	_, err := provider.Rerank(context.Background(), req)

	if err == nil {
		t.Error("Expected error")
	}

	if err.Error() != "mock rerank error" {
		t.Errorf("Expected 'mock rerank error', got %s", err.Error())
	}
}

func TestMockProvider_DiscoverModels(t *testing.T) {
	provider := NewMockProvider("test-provider")

	models, err := provider.DiscoverModels(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}

	if models[0].ID != "test-model" {
		t.Errorf("Expected model ID 'test-model', got %s", models[0].ID)
	}
}

func TestMockProvider_DiscoverModels_Error(t *testing.T) {
	provider := NewMockProvider("test-provider")
	provider.SetShouldError(true)

	_, err := provider.DiscoverModels(context.Background())

	if err == nil {
		t.Error("Expected error")
	}

	if err.Error() != "mock discover error" {
		t.Errorf("Expected 'mock discover error', got %s", err.Error())
	}
}

func TestMockProvider_ValidateConfig(t *testing.T) {
	provider := NewMockProvider("test-provider")

	err := provider.ValidateConfig(map[string]interface{}{"key": "value"})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestMockProvider_ValidateConfig_Error(t *testing.T) {
	provider := NewMockProvider("test-provider")
	provider.SetShouldError(true)

	err := provider.ValidateConfig(map[string]interface{}{"key": "value"})

	if err == nil {
		t.Error("Expected error")
	}

	if err.Error() != "mock validation error" {
		t.Errorf("Expected 'mock validation error', got %s", err.Error())
	}
}

func TestTestError_Error(t *testing.T) {
	err := &TestError{Message: "test error message"}

	if err.Error() != "test error message" {
		t.Errorf("Expected 'test error message', got %s", err.Error())
	}
}

func TestNewTestFixtures(t *testing.T) {
	fixtures := NewTestFixtures()

	if fixtures == nil {
		t.Error("Expected non-nil fixtures")
	}
}

func TestTestFixtures_ChatRequest(t *testing.T) {
	fixtures := NewTestFixtures()
	req := fixtures.ChatRequest()

	if req.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got %s", req.Model)
	}

	if len(req.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(req.Messages))
	}

	if req.Messages[0].Content != "Hello, world!" {
		t.Errorf("Expected message 'Hello, world!', got %s", req.Messages[0].Content)
	}
}

func TestTestFixtures_ChatResponse(t *testing.T) {
	fixtures := NewTestFixtures()
	resp := fixtures.ChatResponse()

	if resp.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", resp.ID)
	}

	if resp.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got %s", resp.Model)
	}

	if len(resp.Choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(resp.Choices))
	}
}

func TestTestFixtures_EmbeddingRequest(t *testing.T) {
	fixtures := NewTestFixtures()
	req := fixtures.EmbeddingRequest()

	if req.Model != "test-embedding-model" {
		t.Errorf("Expected model 'test-embedding-model', got %s", req.Model)
	}

	if len(req.Input) != 1 {
		t.Errorf("Expected 1 input, got %d", len(req.Input))
	}
}

func TestTestFixtures_EmbeddingResponse(t *testing.T) {
	fixtures := NewTestFixtures()
	resp := fixtures.EmbeddingResponse()

	if resp.Model != "test-embedding-model" {
		t.Errorf("Expected model 'test-embedding-model', got %s", resp.Model)
	}

	if len(resp.Data) != 1 {
		t.Errorf("Expected 1 data item, got %d", len(resp.Data))
	}
}

func TestTestFixtures_RerankRequest(t *testing.T) {
	fixtures := NewTestFixtures()
	req := fixtures.RerankRequest()

	if req.Model != "test-rerank-model" {
		t.Errorf("Expected model 'test-rerank-model', got %s", req.Model)
	}

	if req.TopN != 2 {
		t.Errorf("Expected TopN 2, got %d", req.TopN)
	}
}

func TestTestFixtures_RerankResponse(t *testing.T) {
	fixtures := NewTestFixtures()
	resp := fixtures.RerankResponse()

	if resp.Model != "test-rerank-model" {
		t.Errorf("Expected model 'test-rerank-model', got %s", resp.Model)
	}

	if len(resp.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(resp.Results))
	}
}

func TestTestFixtures_ModelInfo(t *testing.T) {
	fixtures := NewTestFixtures()
	model := fixtures.ModelInfo()

	if model.ID != "test-model" {
		t.Errorf("Expected ID 'test-model', got %s", model.ID)
	}

	if !model.Capabilities.SupportsChat {
		t.Error("Expected SupportsChat to be true")
	}

	if model.Capabilities.ContextWindow != 4096 {
		t.Errorf("Expected context window 4096, got %d", model.Capabilities.ContextWindow)
	}
}

func TestAssertChatResponse(t *testing.T) {
	fixtures := NewTestFixtures()

	actual := fixtures.ChatResponse()
	expected := fixtures.ChatResponse()

	// This should not panic or fail
	AssertChatResponse(t, actual, expected)
}

func TestAssertEmbeddingResponse(t *testing.T) {
	fixtures := NewTestFixtures()

	actual := fixtures.EmbeddingResponse()
	expected := fixtures.EmbeddingResponse()

	// This should not panic or fail
	AssertEmbeddingResponse(t, actual, expected)
}

func TestAssertRerankResponse(t *testing.T) {
	fixtures := NewTestFixtures()

	actual := fixtures.RerankResponse()
	expected := fixtures.RerankResponse()

	// This should not panic or fail
	AssertRerankResponse(t, actual, expected)
}

func TestMockProvider_SetModels(t *testing.T) {
	provider := NewMockProvider("test-provider")

	customModels := []toolkit.ModelInfo{
		{
			ID:   "custom-model-1",
			Name: "Custom Model 1",
		},
		{
			ID:   "custom-model-2",
			Name: "Custom Model 2",
		},
	}

	provider.SetModels(customModels)

	models, err := provider.DiscoverModels(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	if models[0].ID != "custom-model-1" {
		t.Errorf("Expected first model ID 'custom-model-1', got %s", models[0].ID)
	}

	if models[1].Name != "Custom Model 2" {
		t.Errorf("Expected second model name 'Custom Model 2', got %s", models[1].Name)
	}
}

func TestMockProvider_ValidateConfig_EmptyAPIKey(t *testing.T) {
	provider := NewMockProvider("test-provider")

	// Test the specific case where API key is empty string
	err := provider.ValidateConfig(map[string]interface{}{
		"api_key": "",
	})

	if err == nil {
		t.Error("Expected error for empty API key")
	}

	if err.Error() != "api_key cannot be empty" {
		t.Errorf("Expected 'api_key cannot be empty', got %s", err.Error())
	}
}
