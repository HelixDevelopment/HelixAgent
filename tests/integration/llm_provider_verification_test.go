// Package integration provides comprehensive LLM provider tests
// These tests verify that all configured LLM providers are working correctly
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// VerifyProviderConfig holds test configuration for each provider (verification tests)
type VerifyProviderConfig struct {
	Name       string
	BaseURL    string
	Model      string
	EnvKeyName string
	MaxTimeout time.Duration
}

// VerifyChatRequest represents an OpenAI-compatible chat request (verification tests)
type VerifyChatRequest struct {
	Model     string          `json:"model"`
	Messages  []VerifyMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
}

// VerifyMessage represents a chat message (verification tests)
type VerifyMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// VerifyChatResponse represents a chat completion response (verification tests)
type VerifyChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Choices []VerifyChoice `json:"choices"`
	Error   *VerifyError   `json:"error,omitempty"`
}

// VerifyChoice represents a response choice (verification tests)
type VerifyChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// VerifyError represents an API error (verification tests)
type VerifyError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// TestLLMProviders_AllProviders tests all configured LLM providers
func TestLLMProviders_AllProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LLM provider tests in short mode")
	}

	providers := []VerifyProviderConfig{
		{
			Name:       "DeepSeek",
			BaseURL:    "https://api.deepseek.com/v1/chat/completions",
			Model:      "deepseek-chat",
			EnvKeyName: "DEEPSEEK_API_KEY",
			MaxTimeout: 30 * time.Second,
		},
		{
			Name:       "Mistral",
			BaseURL:    "https://api.mistral.ai/v1/chat/completions",
			Model:      "mistral-large-latest",
			EnvKeyName: "MISTRAL_API_KEY",
			MaxTimeout: 30 * time.Second,
		},
		{
			Name:       "Cerebras",
			BaseURL:    "https://api.cerebras.ai/v1/chat/completions",
			Model:      "llama-3.3-70b",
			EnvKeyName: "CEREBRAS_API_KEY",
			MaxTimeout: 30 * time.Second,
		},
	}

	for _, provider := range providers {
		t.Run(provider.Name, func(t *testing.T) {
			testVerifyProvider(t, provider)
		})
	}
}

// TestLLMProviders_DeepSeek specifically tests DeepSeek API
func TestLLMProviders_DeepSeek(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping DeepSeek test in short mode")
	}

	provider := VerifyProviderConfig{
		Name:       "DeepSeek",
		BaseURL:    "https://api.deepseek.com/v1/chat/completions",
		Model:      "deepseek-chat",
		EnvKeyName: "DEEPSEEK_API_KEY",
		MaxTimeout: 30 * time.Second,
	}

	testVerifyProvider(t, provider)
}

// TestLLMProviders_Mistral specifically tests Mistral API
func TestLLMProviders_Mistral(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Mistral test in short mode")
	}

	provider := VerifyProviderConfig{
		Name:       "Mistral",
		BaseURL:    "https://api.mistral.ai/v1/chat/completions",
		Model:      "mistral-large-latest",
		EnvKeyName: "MISTRAL_API_KEY",
		MaxTimeout: 30 * time.Second,
	}

	testVerifyProvider(t, provider)
}

// TestLLMProviders_Cerebras specifically tests Cerebras API
func TestLLMProviders_Cerebras(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Cerebras test in short mode")
	}

	provider := VerifyProviderConfig{
		Name:       "Cerebras",
		BaseURL:    "https://api.cerebras.ai/v1/chat/completions",
		Model:      "llama-3.3-70b",
		EnvKeyName: "CEREBRAS_API_KEY",
		MaxTimeout: 30 * time.Second,
	}

	testVerifyProvider(t, provider)
}

// TestLLMProviders_ResponseTime verifies providers respond within acceptable time
func TestLLMProviders_ResponseTime(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping response time test in short mode")
	}

	providers := []VerifyProviderConfig{
		{
			Name:       "DeepSeek",
			BaseURL:    "https://api.deepseek.com/v1/chat/completions",
			Model:      "deepseek-chat",
			EnvKeyName: "DEEPSEEK_API_KEY",
			MaxTimeout: 10 * time.Second, // Strict timeout for performance
		},
		{
			Name:       "Mistral",
			BaseURL:    "https://api.mistral.ai/v1/chat/completions",
			Model:      "mistral-large-latest",
			EnvKeyName: "MISTRAL_API_KEY",
			MaxTimeout: 10 * time.Second,
		},
		{
			Name:       "Cerebras",
			BaseURL:    "https://api.cerebras.ai/v1/chat/completions",
			Model:      "llama-3.3-70b",
			EnvKeyName: "CEREBRAS_API_KEY",
			MaxTimeout: 10 * time.Second,
		},
	}

	for _, provider := range providers {
		t.Run(provider.Name+"_ResponseTime", func(t *testing.T) {
			apiKey := os.Getenv(provider.EnvKeyName)
			if apiKey == "" {
				t.Skipf("Skipping %s: %s not set", provider.Name, provider.EnvKeyName)
			}

			start := time.Now()
			_, err := makeVerifyRequest(provider, apiKey, "Hi")
			elapsed := time.Since(start)

			if err != nil {
				t.Errorf("%s request failed: %v", provider.Name, err)
				return
			}

			if elapsed > provider.MaxTimeout {
				t.Errorf("%s response time %v exceeds max %v", provider.Name, elapsed, provider.MaxTimeout)
			} else {
				t.Logf("%s responded in %v", provider.Name, elapsed)
			}
		})
	}
}

// TestLLMProviders_AuthValidation verifies API key validation
func TestLLMProviders_AuthValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping auth validation test in short mode")
	}

	providers := []VerifyProviderConfig{
		{Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1/chat/completions", Model: "deepseek-chat", MaxTimeout: 10 * time.Second},
		{Name: "Mistral", BaseURL: "https://api.mistral.ai/v1/chat/completions", Model: "mistral-large-latest", MaxTimeout: 10 * time.Second},
		{Name: "Cerebras", BaseURL: "https://api.cerebras.ai/v1/chat/completions", Model: "llama-3.3-70b", MaxTimeout: 10 * time.Second},
	}

	for _, provider := range providers {
		t.Run(provider.Name+"_InvalidAuth", func(t *testing.T) {
			resp, err := makeVerifyRequest(provider, "invalid-api-key", "Hi")
			if err == nil && resp.Error == nil {
				t.Errorf("%s should reject invalid API key", provider.Name)
			}
		})
	}
}

// TestHelixAgent_Endpoint tests the HelixAgent chat completions endpoint
func TestHelixAgent_Endpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HelixAgent endpoint test in short mode")
	}

	helixagentURL := os.Getenv("HELIXAGENT_URL")
	if helixagentURL == "" {
		helixagentURL = "http://localhost:7061"
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2af3d6cd8c0bdf57e221bbf7771fa06bda93cc8866807cc85211f58d1a"
	}

	provider := VerifyProviderConfig{
		Name:       "HelixAgent",
		BaseURL:    helixagentURL + "/v1/chat/completions",
		Model:      "helix-agent",
		MaxTimeout: 60 * time.Second,
	}

	start := time.Now()
	resp, err := makeVerifyRequest(provider, apiKey, "What is 2+2?")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("HelixAgent request failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("HelixAgent returned error: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("HelixAgent returned no choices")
	}

	t.Logf("HelixAgent responded in %v: %s", elapsed, resp.Choices[0].Message.Content[:verifyMin(100, len(resp.Choices[0].Message.Content))])
}

// TestHelixAgent_Health tests the HelixAgent health endpoint
func TestHelixAgent_Health(t *testing.T) {
	helixagentURL := os.Getenv("HELIXAGENT_URL")
	if helixagentURL == "" {
		helixagentURL = "http://localhost:7061"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(helixagentURL + "/health")
	if err != nil {
		// Skip test if server is not running (connection refused)
		t.Skipf("Skipping integration test - HelixAgent server not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Health check returned status %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	status, ok := health["status"].(string)
	if !ok || status != "healthy" {
		t.Fatalf("Unexpected health status: %v", health)
	}

	t.Log("HelixAgent health check passed")
}

// testVerifyProvider is a helper function to test a single provider
func testVerifyProvider(t *testing.T, provider VerifyProviderConfig) {
	apiKey := os.Getenv(provider.EnvKeyName)
	if apiKey == "" {
		t.Skipf("Skipping %s: %s not set", provider.Name, provider.EnvKeyName)
	}

	resp, err := makeVerifyRequest(provider, apiKey, "Say hello in one word")
	if err != nil {
		t.Fatalf("%s request failed: %v", provider.Name, err)
	}

	if resp.Error != nil {
		t.Fatalf("%s API error: %s (code: %s)", provider.Name, resp.Error.Message, resp.Error.Code)
	}

	if len(resp.Choices) == 0 {
		t.Fatalf("%s returned no choices", provider.Name)
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		t.Fatalf("%s returned empty content", provider.Name)
	}

	t.Logf("%s response: %s", provider.Name, content[:verifyMin(50, len(content))])
}

// makeVerifyRequest makes a chat completion request to a provider
func makeVerifyRequest(provider VerifyProviderConfig, apiKey string, prompt string) (*VerifyChatResponse, error) {
	reqBody := VerifyChatRequest{
		Model: provider.Model,
		Messages: []VerifyMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 50,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", provider.BaseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: provider.MaxTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Return error for 4xx/5xx HTTP status codes (authentication failures, etc.)
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var chatResp VerifyChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	return &chatResp, nil
}

func verifyMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
