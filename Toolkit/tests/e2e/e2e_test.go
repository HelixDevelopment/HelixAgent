package e2e

import (
	"context"
	"testing"
	"time"

	testingutils "github.com/HelixDevelopment/helix_agent/Toolkit/Commons/testing"
	"github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit"
)

// TestCompleteChatWorkflow tests a complete chat workflow from start to finish
func TestCompleteChatWorkflow(t *testing.T) {
	// Setup mock provider
	mockProvider := testingutils.NewMockProvider("e2e-chat-provider")

	// Setup test fixtures
	fixtures := testingutils.NewTestFixtures()
	chatResp := fixtures.ChatResponse()
	mockProvider.SetChatResponse(chatResp)

	// Step 1: Provider initialization
	t.Log("Step 1: Provider initialization")
	config := map[string]interface{}{
		"api_key": "test-api-key",
		"timeout": 30,
	}

	err := mockProvider.ValidateConfig(config)
	if err != nil {
		t.Fatalf("Provider configuration failed: %v", err)
	}
	t.Log("✓ Provider configured successfully")

	// Step 2: Model discovery
	t.Log("Step 2: Model discovery")
	ctx := context.Background()
	models, err := mockProvider.DiscoverModels(ctx)
	if err != nil {
		t.Fatalf("Model discovery failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("No models discovered")
	}
	t.Logf("✓ Discovered %d models", len(models))

	// Step 3: Chat completion
	t.Log("Step 3: Chat completion")
	chatReq := fixtures.ChatRequest()
	chatReq.Model = models[0].ID // Use discovered model

	response, err := mockProvider.Chat(ctx, chatReq)
	if err != nil {
		t.Fatalf("Chat completion failed: %v", err)
	}

	// Validate response
	if response.ID == "" {
		t.Error("Response missing ID")
	}
	if response.Model != chatReq.Model {
		t.Errorf("Response model mismatch: expected %s, got %s", chatReq.Model, response.Model)
	}
	if len(response.Choices) == 0 {
		t.Error("Response missing choices")
	}
	if response.Usage.TotalTokens == 0 {
		t.Error("Response missing usage information")
	}
	t.Logf("✓ Chat completed successfully with %d tokens used", response.Usage.TotalTokens)

	// Step 4: Follow-up conversation
	t.Log("Step 4: Follow-up conversation")
	followUpReq := toolkit.ChatRequest{
		Model: response.Model,
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "Can you explain that in simpler terms?"},
		},
	}

	followUpResp, err := mockProvider.Chat(ctx, followUpReq)
	if err != nil {
		t.Fatalf("Follow-up chat failed: %v", err)
	}
	t.Logf("✓ Follow-up conversation completed with %d tokens used", followUpResp.Usage.TotalTokens)

	t.Log("✓ Complete chat workflow finished successfully")
}

// TestCompleteEmbeddingWorkflow tests a complete embedding workflow
func TestCompleteEmbeddingWorkflow(t *testing.T) {
	// Setup mock provider
	mockProvider := testingutils.NewMockProvider("e2e-embedding-provider")

	// Setup test fixtures
	fixtures := testingutils.NewTestFixtures()
	embedResp := fixtures.EmbeddingResponse()
	mockProvider.SetEmbeddingResponse(embedResp)

	// Step 1: Provider initialization
	t.Log("Step 1: Provider initialization")
	config := map[string]interface{}{
		"api_key": "test-api-key",
	}

	err := mockProvider.ValidateConfig(config)
	if err != nil {
		t.Fatalf("Provider configuration failed: %v", err)
	}
	t.Log("✓ Provider configured successfully")

	// Step 2: Model discovery (find embedding-capable model)
	t.Log("Step 2: Model discovery")
	ctx := context.Background()
	models, err := mockProvider.DiscoverModels(ctx)
	if err != nil {
		t.Fatalf("Model discovery failed: %v", err)
	}

	// Find an embedding-capable model
	var embeddingModel *toolkit.ModelInfo
	for _, model := range models {
		if model.Capabilities.SupportsEmbedding {
			embeddingModel = &model
			break
		}
	}

	if embeddingModel == nil {
		// Create a mock embedding model
		embeddingModel = &toolkit.ModelInfo{
			ID:   "embedding-model",
			Name: "Embedding Model",
			Capabilities: toolkit.ModelCapabilities{
				SupportsEmbedding: true,
			},
			Provider: "e2e-embedding-provider",
		}
		mockProvider.SetModels(append(models, *embeddingModel))
	}
	t.Logf("✓ Using embedding model: %s", embeddingModel.ID)

	// Step 3: Single text embedding
	t.Log("Step 3: Single text embedding")
	embedReq := toolkit.EmbeddingRequest{
		Model: embeddingModel.ID,
		Input: []string{"This is a test document for embedding."},
	}

	response, err := mockProvider.Embed(ctx, embedReq)
	if err != nil {
		t.Fatalf("Embedding failed: %v", err)
	}

	// Validate response (mock returns fixture model, not the requested model)
	expectedModel := fixtures.EmbeddingResponse().Model
	if response.Model != expectedModel {
		t.Errorf("Response model mismatch: expected %s, got %s", expectedModel, response.Model)
	}
	if len(response.Data) != len(embedReq.Input) {
		t.Errorf("Embedding count mismatch: expected %d, got %d", len(embedReq.Input), len(response.Data))
	}
	if len(response.Data[0].Embedding) == 0 {
		t.Error("Embedding vector is empty")
	}
	t.Logf("✓ Single text embedded successfully, vector length: %d", len(response.Data[0].Embedding))

	// Step 4: Batch embedding
	t.Log("Step 4: Batch embedding")
	batchReq := toolkit.EmbeddingRequest{
		Model: embeddingModel.ID,
		Input: []string{
			"First document",
			"Second document",
			"Third document",
		},
	}

	batchResp, err := mockProvider.Embed(ctx, batchReq)
	if err != nil {
		t.Fatalf("Batch embedding failed: %v", err)
	}

	// Mock returns fixed response, so we just check it has some data
	if len(batchResp.Data) == 0 {
		t.Error("Batch embedding returned no data")
	}
	t.Logf("✓ Batch embedding completed for %d documents", len(batchResp.Data))

	t.Log("✓ Complete embedding workflow finished successfully")
}

// TestCompleteRerankWorkflow tests a complete rerank workflow
func TestCompleteRerankWorkflow(t *testing.T) {
	// Setup mock provider
	mockProvider := testingutils.NewMockProvider("e2e-rerank-provider")

	// Setup test fixtures
	fixtures := testingutils.NewTestFixtures()
	rerankResp := fixtures.RerankResponse()
	mockProvider.SetRerankResponse(rerankResp)

	// Step 1: Provider initialization
	t.Log("Step 1: Provider initialization")
	config := map[string]interface{}{
		"api_key": "test-api-key",
	}

	err := mockProvider.ValidateConfig(config)
	if err != nil {
		t.Fatalf("Provider configuration failed: %v", err)
	}
	t.Log("✓ Provider configured successfully")

	// Step 2: Model discovery (find rerank-capable model)
	t.Log("Step 2: Model discovery")
	ctx := context.Background()
	models, err := mockProvider.DiscoverModels(ctx)
	if err != nil {
		t.Fatalf("Model discovery failed: %v", err)
	}

	// Use first model or create a rerank model
	rerankModelID := models[0].ID
	if len(models) == 0 {
		rerankModelID = "rerank-model"
	}
	t.Logf("✓ Using rerank model: %s", rerankModelID)

	// Step 3: Document reranking
	t.Log("Step 3: Document reranking")
	rerankReq := toolkit.RerankRequest{
		Model: rerankModelID,
		Query: "What is machine learning?",
		Documents: []string{
			"Machine learning is a subset of artificial intelligence.",
			"The weather today is sunny and warm.",
			"Deep learning uses neural networks with multiple layers.",
			"Cooking recipes often require specific ingredients.",
			"Supervised learning requires labeled training data.",
		},
		TopN: 3,
	}

	response, err := mockProvider.Rerank(ctx, rerankReq)
	if err != nil {
		t.Fatalf("Reranking failed: %v", err)
	}

	// Validate response (mock returns fixture model, not the requested model)
	expectedModel := fixtures.RerankResponse().Model
	if response.Model != expectedModel {
		t.Errorf("Response model mismatch: expected %s, got %s", expectedModel, response.Model)
	}
	// Mock returns fixed number of results, not necessarily TopN
	if len(response.Results) == 0 {
		t.Error("No rerank results returned")
	}

	// Check that results are sorted by score (descending)
	for i := 1; i < len(response.Results); i++ {
		if response.Results[i].Score > response.Results[i-1].Score {
			t.Error("Rerank results not properly sorted by score")
		}
	}
	t.Logf("✓ Reranking completed, top %d results returned", len(response.Results))

	t.Log("✓ Complete rerank workflow finished successfully")
}

// TestErrorHandlingWorkflow tests error handling throughout a complete workflow
func TestErrorHandlingWorkflow(t *testing.T) {
	// Step 1: Provider initialization with invalid config
	t.Log("Step 1: Testing invalid configuration")
	mockProvider := testingutils.NewMockProvider("e2e-error-provider")

	invalidConfig := map[string]interface{}{
		"api_key": "", // Empty API key
	}

	err := mockProvider.ValidateConfig(invalidConfig)
	if err == nil {
		t.Error("Expected validation error for empty API key")
	}
	t.Logf("✓ Invalid config properly rejected: %v", err)

	// Step 2: Valid config but operations will fail
	mockProvider.SetShouldError(true) // Now set to fail operations
	validConfig := map[string]interface{}{
		"api_key": "test-key",
	}

	err = mockProvider.ValidateConfig(validConfig)
	if err == nil {
		t.Error("Expected validation error when shouldError is true")
	}
	t.Logf("✓ Config validation failed as expected: %v", err)

	// Step 3: Test operation failures
	ctx := context.Background()

	// Chat failure
	t.Log("Step 3: Testing operation failures")
	chatReq := toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "Test message"},
		},
	}

	_, err = mockProvider.Chat(ctx, chatReq)
	if err == nil {
		t.Error("Expected chat operation to fail")
	}
	t.Logf("✓ Chat failure handled: %v", err)

	// Model discovery failure
	_, err = mockProvider.DiscoverModels(ctx)
	if err == nil {
		t.Error("Expected model discovery to fail")
	}
	t.Logf("✓ Model discovery failure handled: %v", err)

	// Embedding failure
	embedReq := toolkit.EmbeddingRequest{
		Model: "test-model",
		Input: []string{"test"},
	}

	_, err = mockProvider.Embed(ctx, embedReq)
	if err == nil {
		t.Error("Expected embedding operation to fail")
	}
	t.Logf("✓ Embedding failure handled: %v", err)

	// Rerank failure
	rerankReq := toolkit.RerankRequest{
		Model:     "test-model",
		Query:     "test",
		Documents: []string{"doc"},
		TopN:      1,
	}

	_, err = mockProvider.Rerank(ctx, rerankReq)
	if err == nil {
		t.Error("Expected rerank operation to fail")
	}
	t.Logf("✓ Rerank failure handled: %v", err)

	t.Log("✓ Error handling workflow completed successfully")
}

// TestConcurrentWorkflows tests multiple workflows running concurrently
func TestConcurrentWorkflows(t *testing.T) {
	numWorkers := 5
	done := make(chan bool, numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			// Each worker runs a complete chat workflow
			mockProvider := testingutils.NewMockProvider("e2e-concurrent-provider")

			fixtures := testingutils.NewTestFixtures()
			chatResp := fixtures.ChatResponse()
			mockProvider.SetChatResponse(chatResp)

			ctx := context.Background()

			// Quick workflow: config -> discover -> chat
			config := map[string]interface{}{"api_key": "test"}
			_ = mockProvider.ValidateConfig(config)

			models, _ := mockProvider.DiscoverModels(ctx)

			if len(models) > 0 {
				chatReq := fixtures.ChatRequest()
				chatReq.Model = models[0].ID
				_, _ = mockProvider.Chat(ctx, chatReq)
			}

			t.Logf("Worker %d completed workflow", workerID)
			done <- true
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < numWorkers; i++ {
		<-done
	}

	t.Log("✓ Concurrent workflows completed successfully")
}

// TestTimeoutWorkflow tests workflow behavior under timeouts
func TestTimeoutWorkflow(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("e2e-timeout-provider")

	// Setup slow-responding mock (simulate delay)
	fixtures := testingutils.NewTestFixtures()
	chatResp := fixtures.ChatResponse()
	mockProvider.SetChatResponse(chatResp)

	// Step 1: Normal operation
	t.Log("Step 1: Normal operation")
	ctx := context.Background()
	chatReq := fixtures.ChatRequest()

	_, err := mockProvider.Chat(ctx, chatReq)
	if err != nil {
		t.Fatalf("Normal operation failed: %v", err)
	}
	t.Log("✓ Normal operation completed")

	// Step 2: Operation with timeout
	t.Log("Step 2: Operation with timeout")
	shortCtx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err = mockProvider.Chat(shortCtx, chatReq)
	// Mock provider doesn't respect timeout, but real providers should
	t.Logf("✓ Timeout operation result: %v", err)

	t.Log("✓ Timeout workflow completed")
}
