// Package main provides a mock LLM server for testing purposes.
// This server implements OpenAI-compatible API endpoints for testing without real API keys.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ChatCompletionRequest represents the OpenAI chat completion request format
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse represents the OpenAI chat completion response format
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// EmbeddingRequest represents the OpenAI embedding request format
type EmbeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

// EmbeddingResponse represents the OpenAI embedding response format
type EmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// ModelsResponse represents the models list response
type ModelsResponse struct {
	Object string `json:"object"`
	Data   []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/v1/health", handleHealth)

	// Chat completions (OpenAI-compatible)
	mux.HandleFunc("/v1/chat/completions", handleChatCompletions)
	mux.HandleFunc("/chat/completions", handleChatCompletions)

	// Completions (legacy)
	mux.HandleFunc("/v1/completions", handleCompletions)
	mux.HandleFunc("/completions", handleCompletions)

	// Embeddings
	mux.HandleFunc("/v1/embeddings", handleEmbeddings)
	mux.HandleFunc("/embeddings", handleEmbeddings)

	// Models list
	mux.HandleFunc("/v1/models", handleModels)
	mux.HandleFunc("/models", handleModels)

	// Claude API compatibility
	mux.HandleFunc("/v1/messages", handleClaudeMessages)
	mux.HandleFunc("/messages", handleClaudeMessages)

	// Ollama API compatibility
	mux.HandleFunc("/api/generate", handleOllamaGenerate)
	mux.HandleFunc("/api/chat", handleOllamaChat)
	mux.HandleFunc("/api/tags", handleOllamaTags)

	log.Printf("Mock LLM server starting on port %s", port)
	log.Printf("Endpoints available:")
	log.Printf("  - POST /v1/chat/completions (OpenAI)")
	log.Printf("  - POST /v1/completions (OpenAI legacy)")
	log.Printf("  - POST /v1/embeddings (OpenAI)")
	log.Printf("  - GET  /v1/models (OpenAI)")
	log.Printf("  - POST /v1/messages (Claude)")
	log.Printf("  - POST /api/generate (Ollama)")
	log.Printf("  - POST /api/chat (Ollama)")

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"service":   "mock-llm-server",
		"timestamp": time.Now().Unix(),
	})
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Generate mock response based on input
	lastMessage := ""
	if len(req.Messages) > 0 {
		lastMessage = req.Messages[len(req.Messages)-1].Content
	}

	responseContent := generateMockResponse(lastMessage, req.Model)

	if req.Stream {
		handleStreamingResponse(w, req.Model, responseContent)
		return
	}

	response := ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-mock-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
	}
	response.Choices = []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}{
		{
			Index: 0,
			Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				Role:    "assistant",
				Content: responseContent,
			},
			FinishReason: "stop",
		},
	}
	response.Usage.PromptTokens = len(lastMessage) / 4
	response.Usage.CompletionTokens = len(responseContent) / 4
	response.Usage.TotalTokens = response.Usage.PromptTokens + response.Usage.CompletionTokens

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleStreamingResponse(w http.ResponseWriter, model, content string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Split content into chunks
	words := strings.Fields(content)
	id := fmt.Sprintf("chatcmpl-mock-%d", time.Now().UnixNano())

	for i, word := range words {
		chunk := map[string]interface{}{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]string{
						"content": word + " ",
					},
					"finish_reason": nil,
				},
			},
		}

		if i == len(words)-1 {
			chunk["choices"].([]map[string]interface{})[0]["finish_reason"] = "stop"
		}

		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var req struct {
		Model     string `json:"model"`
		Prompt    string `json:"prompt"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	responseText := generateMockResponse(req.Prompt, req.Model)

	response := map[string]interface{}{
		"id":      fmt.Sprintf("cmpl-mock-%d", time.Now().UnixNano()),
		"object":  "text_completion",
		"created": time.Now().Unix(),
		"model":   req.Model,
		"choices": []map[string]interface{}{
			{
				"text":          responseText,
				"index":         0,
				"logprobs":      nil,
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     len(req.Prompt) / 4,
			"completion_tokens": len(responseText) / 4,
			"total_tokens":      (len(req.Prompt) + len(responseText)) / 4,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var req EmbeddingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Generate mock embedding (1536 dimensions like text-embedding-ada-002)
	embedding := make([]float64, 1536)
	for i := range embedding {
		embedding[i] = float64(i%100) / 100.0
	}

	response := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{
				"object":    "embedding",
				"embedding": embedding,
				"index":     0,
			},
		},
		"model": req.Model,
		"usage": map[string]int{
			"prompt_tokens": len(req.Input) / 4,
			"total_tokens":  len(req.Input) / 4,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	models := []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}{
		{"gpt-4", "model", time.Now().Unix(), "mock-provider"},
		{"gpt-4-turbo", "model", time.Now().Unix(), "mock-provider"},
		{"gpt-3.5-turbo", "model", time.Now().Unix(), "mock-provider"},
		{"claude-3-opus", "model", time.Now().Unix(), "mock-provider"},
		{"claude-3-sonnet", "model", time.Now().Unix(), "mock-provider"},
		{"text-embedding-ada-002", "model", time.Now().Unix(), "mock-provider"},
		{"llama2", "model", time.Now().Unix(), "mock-provider"},
		{"mistral", "model", time.Now().Unix(), "mock-provider"},
	}

	response := map[string]interface{}{
		"object": "list",
		"data":   models,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleClaudeMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var req struct {
		Model     string        `json:"model"`
		Messages  []ChatMessage `json:"messages"`
		MaxTokens int           `json:"max_tokens"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	lastMessage := ""
	if len(req.Messages) > 0 {
		lastMessage = req.Messages[len(req.Messages)-1].Content
	}

	responseContent := generateMockResponse(lastMessage, req.Model)

	response := map[string]interface{}{
		"id":            fmt.Sprintf("msg-mock-%d", time.Now().UnixNano()),
		"type":          "message",
		"role":          "assistant",
		"content":       []map[string]string{{"type": "text", "text": responseContent}},
		"model":         req.Model,
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage": map[string]int{
			"input_tokens":  len(lastMessage) / 4,
			"output_tokens": len(responseContent) / 4,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleOllamaGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var req struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	responseContent := generateMockResponse(req.Prompt, req.Model)

	response := map[string]interface{}{
		"model":                req.Model,
		"created_at":           time.Now().Format(time.RFC3339),
		"response":             responseContent,
		"done":                 true,
		"context":              []int{1, 2, 3},
		"total_duration":       1000000000,
		"load_duration":        500000000,
		"prompt_eval_count":    len(req.Prompt) / 4,
		"prompt_eval_duration": 200000000,
		"eval_count":           len(responseContent) / 4,
		"eval_duration":        300000000,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleOllamaChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var req struct {
		Model    string        `json:"model"`
		Messages []ChatMessage `json:"messages"`
		Stream   bool          `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	lastMessage := ""
	if len(req.Messages) > 0 {
		lastMessage = req.Messages[len(req.Messages)-1].Content
	}

	responseContent := generateMockResponse(lastMessage, req.Model)

	response := map[string]interface{}{
		"model":      req.Model,
		"created_at": time.Now().Format(time.RFC3339),
		"message": map[string]string{
			"role":    "assistant",
			"content": responseContent,
		},
		"done":           true,
		"total_duration": 1000000000,
		"eval_count":     len(responseContent) / 4,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleOllamaTags(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"models": []map[string]interface{}{
			{
				"name":        "llama2:latest",
				"model":       "llama2:latest",
				"modified_at": time.Now().Format(time.RFC3339),
				"size":        3826793472,
				"digest":      "mock-digest",
				"details": map[string]interface{}{
					"parent_model":       "",
					"format":             "gguf",
					"family":             "llama",
					"families":           []string{"llama"},
					"parameter_size":     "7B",
					"quantization_level": "Q4_0",
				},
			},
			{
				"name":        "mistral:latest",
				"model":       "mistral:latest",
				"modified_at": time.Now().Format(time.RFC3339),
				"size":        4109865984,
				"digest":      "mock-digest-2",
				"details": map[string]interface{}{
					"parent_model":       "",
					"format":             "gguf",
					"family":             "mistral",
					"families":           []string{"mistral"},
					"parameter_size":     "7B",
					"quantization_level": "Q4_0",
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func generateMockResponse(prompt, model string) string {
	// Generate contextual mock responses based on input
	prompt = strings.ToLower(prompt)

	if strings.Contains(prompt, "hello") || strings.Contains(prompt, "hi") {
		return "Hello! I'm a mock LLM server for testing purposes. How can I help you today?"
	}

	if strings.Contains(prompt, "test") {
		return "This is a test response from the mock LLM server. The server is working correctly."
	}

	if strings.Contains(prompt, "code") || strings.Contains(prompt, "function") {
		return "Here's a sample function:\n\n```go\nfunc example() string {\n    return \"Hello from mock LLM\"\n}\n```"
	}

	if strings.Contains(prompt, "error") {
		return "I understand you're experiencing an issue. Let me help you troubleshoot this problem."
	}

	if strings.Contains(prompt, "json") {
		return `{"status": "success", "message": "Mock JSON response", "data": {"key": "value"}}`
	}

	// Default response
	return fmt.Sprintf("Mock response from model '%s': I've processed your request. This is a simulated response for testing purposes. Your input was: %s",
		model, truncate(prompt, 100))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
