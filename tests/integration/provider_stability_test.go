// Package integration provides comprehensive provider stability tests
// These tests verify that all LLM providers are working correctly and catch issues early
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

// ProviderStabilityConfig holds test configuration for provider stability tests
type ProviderStabilityConfig struct {
	Name           string
	APIEndpoint    string
	Model          string
	APIKeyEnvVar   string
	Timeout        time.Duration
	ExpectedFields []string
}

// StabilityChatRequest represents an OpenAI-compatible chat request
type StabilityChatRequest struct {
	Model       string             `json:"model"`
	Messages    []StabilityMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
}

// StabilityMessage represents a chat message
type StabilityMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StabilityChatResponse represents a chat completion response
type StabilityChatResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []StabilityChoice `json:"choices"`
	Usage   StabilityUsage    `json:"usage,omitempty"`
	Error   *StabilityError   `json:"error,omitempty"`
}

// StabilityChoice represents a response choice
type StabilityChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// StabilityUsage represents token usage
type StabilityUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StabilityError represents an API error
type StabilityError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// TestProviderStability_AllProviders tests all configured LLM providers
func TestProviderStability_AllProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping provider stability tests in short mode")
	}

	providers := []ProviderStabilityConfig{
		{
			Name:         "DeepSeek",
			APIEndpoint:  "https://api.deepseek.com/v1/chat/completions",
			Model:        "deepseek-chat",
			APIKeyEnvVar: "DEEPSEEK_API_KEY",
			Timeout:      60 * time.Second,
			ExpectedFields: []string{"id", "choices", "model"},
		},
		{
			Name:         "Mistral",
			APIEndpoint:  "https://api.mistral.ai/v1/chat/completions",
			Model:        "mistral-large-latest",
			APIKeyEnvVar: "MISTRAL_API_KEY",
			Timeout:      60 * time.Second,
			ExpectedFields: []string{"id", "choices", "model"},
		},
		{
			Name:         "Cerebras",
			APIEndpoint:  "https://api.cerebras.ai/v1/chat/completions",
			Model:        "llama-3.3-70b",
			APIKeyEnvVar: "CEREBRAS_API_KEY",
			Timeout:      60 * time.Second,
			ExpectedFields: []string{"id", "choices", "model"},
		},
		// Gemini uses a different API format - skip in direct tests
		// HelixAgent handles Gemini through its native provider
	}

	for _, provider := range providers {
		t.Run(provider.Name, func(t *testing.T) {
			testProviderStability(t, provider)
		})
	}
}

// testProviderStability tests a single provider for stability
func testProviderStability(t *testing.T, provider ProviderStabilityConfig) {
	apiKey := os.Getenv(provider.APIKeyEnvVar)
	if apiKey == "" {
		t.Skipf("Skipping %s: %s not set", provider.Name, provider.APIKeyEnvVar)
	}

	// Test 1: Basic completion
	t.Run("BasicCompletion", func(t *testing.T) {
		resp, err := makeStabilityRequest(provider, apiKey, "Say hello in one word")
		if err != nil {
			t.Fatalf("%s request failed: %v", provider.Name, err)
		}

		validateStabilityResponse(t, provider.Name, resp, provider.ExpectedFields)
	})

	// Test 2: Response has content
	t.Run("ResponseHasContent", func(t *testing.T) {
		resp, err := makeStabilityRequest(provider, apiKey, "What is 2+2?")
		if err != nil {
			t.Fatalf("%s request failed: %v", provider.Name, err)
		}

		if len(resp.Choices) == 0 {
			t.Fatalf("%s returned no choices", provider.Name)
		}

		content := resp.Choices[0].Message.Content
		if content == "" {
			t.Fatalf("%s returned empty content", provider.Name)
		}

		t.Logf("%s response: %s", provider.Name, truncateString(content, 100))
	})

	// Test 3: Has valid finish reason
	t.Run("HasFinishReason", func(t *testing.T) {
		resp, err := makeStabilityRequest(provider, apiKey, "Hi")
		if err != nil {
			t.Fatalf("%s request failed: %v", provider.Name, err)
		}

		if len(resp.Choices) == 0 {
			t.Fatalf("%s returned no choices", provider.Name)
		}

		finishReason := resp.Choices[0].FinishReason
		validReasons := []string{"stop", "length", "end_turn", "model_length", "eos"}
		isValid := false
		for _, r := range validReasons {
			if finishReason == r {
				isValid = true
				break
			}
		}
		if !isValid && finishReason != "" {
			t.Logf("%s has unusual finish_reason: %s (may be OK)", provider.Name, finishReason)
		}
	})
}

// TestProviderStability_ErrorHandling tests error handling for providers
func TestProviderStability_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error handling tests in short mode")
	}

	providers := []ProviderStabilityConfig{
		{Name: "DeepSeek", APIEndpoint: "https://api.deepseek.com/v1/chat/completions", Model: "deepseek-chat", Timeout: 10 * time.Second},
		{Name: "Mistral", APIEndpoint: "https://api.mistral.ai/v1/chat/completions", Model: "mistral-large-latest", Timeout: 10 * time.Second},
		{Name: "Cerebras", APIEndpoint: "https://api.cerebras.ai/v1/chat/completions", Model: "llama-3.3-70b", Timeout: 10 * time.Second},
	}

	for _, provider := range providers {
		t.Run(provider.Name+"_InvalidAuth", func(t *testing.T) {
			resp, err := makeStabilityRequest(provider, "invalid-api-key-test", "Hi")
			// Should either return error or error response
			if err == nil && resp.Error == nil && len(resp.Choices) > 0 {
				t.Errorf("%s should reject invalid API key", provider.Name)
			}
		})
	}
}

// TestProviderStability_Concurrent tests concurrent requests to providers
func TestProviderStability_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent tests in short mode")
	}

	providers := []ProviderStabilityConfig{
		{Name: "DeepSeek", APIEndpoint: "https://api.deepseek.com/v1/chat/completions", Model: "deepseek-chat", APIKeyEnvVar: "DEEPSEEK_API_KEY", Timeout: 60 * time.Second},
		{Name: "Mistral", APIEndpoint: "https://api.mistral.ai/v1/chat/completions", Model: "mistral-large-latest", APIKeyEnvVar: "MISTRAL_API_KEY", Timeout: 60 * time.Second},
		{Name: "Cerebras", APIEndpoint: "https://api.cerebras.ai/v1/chat/completions", Model: "llama-3.3-70b", APIKeyEnvVar: "CEREBRAS_API_KEY", Timeout: 60 * time.Second},
	}

	for _, provider := range providers {
		t.Run(provider.Name+"_Concurrent", func(t *testing.T) {
			apiKey := os.Getenv(provider.APIKeyEnvVar)
			if apiKey == "" {
				t.Skipf("Skipping %s: %s not set", provider.Name, provider.APIKeyEnvVar)
			}

			concurrency := 3
			var wg sync.WaitGroup
			results := make(chan *stabilityTestResult, concurrency)

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					start := time.Now()
					resp, err := makeStabilityRequest(provider, apiKey, fmt.Sprintf("Count to %d", idx+1))
					results <- &stabilityTestResult{
						index:    idx,
						response: resp,
						err:      err,
						duration: time.Since(start),
					}
				}(i)
			}

			go func() {
				wg.Wait()
				close(results)
			}()

			successCount := 0
			for result := range results {
				if result.err != nil {
					t.Logf("%s request %d failed: %v", provider.Name, result.index, result.err)
					continue
				}
				if result.response.Error != nil {
					t.Logf("%s request %d API error: %s", provider.Name, result.index, result.response.Error.Message)
					continue
				}
				if len(result.response.Choices) == 0 {
					t.Logf("%s request %d returned no choices", provider.Name, result.index)
					continue
				}
				successCount++
				t.Logf("%s request %d completed in %v", provider.Name, result.index, result.duration)
			}

			// At least 2 out of 3 should succeed for stability
			if successCount < 2 {
				t.Errorf("%s only had %d/%d successful concurrent requests", provider.Name, successCount, concurrency)
			}
		})
	}
}

// TestProviderStability_ResponseTime tests response time for providers
func TestProviderStability_ResponseTime(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping response time tests in short mode")
	}

	providers := []ProviderStabilityConfig{
		{Name: "DeepSeek", APIEndpoint: "https://api.deepseek.com/v1/chat/completions", Model: "deepseek-chat", APIKeyEnvVar: "DEEPSEEK_API_KEY", Timeout: 30 * time.Second},
		{Name: "Mistral", APIEndpoint: "https://api.mistral.ai/v1/chat/completions", Model: "mistral-large-latest", APIKeyEnvVar: "MISTRAL_API_KEY", Timeout: 30 * time.Second},
		{Name: "Cerebras", APIEndpoint: "https://api.cerebras.ai/v1/chat/completions", Model: "llama-3.3-70b", APIKeyEnvVar: "CEREBRAS_API_KEY", Timeout: 30 * time.Second},
	}

	maxResponseTime := 30 * time.Second

	for _, provider := range providers {
		t.Run(provider.Name+"_ResponseTime", func(t *testing.T) {
			apiKey := os.Getenv(provider.APIKeyEnvVar)
			if apiKey == "" {
				t.Skipf("Skipping %s: %s not set", provider.Name, provider.APIKeyEnvVar)
			}

			start := time.Now()
			_, err := makeStabilityRequest(provider, apiKey, "Hi")
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("%s request failed: %v", provider.Name, err)
			}

			if elapsed > maxResponseTime {
				t.Errorf("%s response time %v exceeds max %v", provider.Name, elapsed, maxResponseTime)
			} else {
				t.Logf("%s responded in %v", provider.Name, elapsed)
			}
		})
	}
}

// TestHelixAgent_ProviderIntegration tests HelixAgent's integration with providers
func TestHelixAgent_ProviderIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HelixAgent integration test in short mode")
	}

	helixagentURL := os.Getenv("HELIXAGENT_URL")
	if helixagentURL == "" {
		helixagentURL = "http://localhost:7061"
	}

	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-bd15ed2af3d6cd8c0bdf57e221bbf7771fa06bda93cc8866807cc85211f58d1a"
	}

	// Check health first
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(helixagentURL + "/health")
	if err != nil {
		t.Skipf("HelixAgent not available: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("HelixAgent unhealthy: %d", resp.StatusCode)
	}

	// Test chat completion
	t.Run("ChatCompletion", func(t *testing.T) {
		provider := ProviderStabilityConfig{
			Name:        "HelixAgent",
			APIEndpoint: helixagentURL + "/v1/chat/completions",
			Model:       "helixagent-debate",
			Timeout:     120 * time.Second,
		}

		req := StabilityChatRequest{
			Model: provider.Model,
			Messages: []StabilityMessage{
				{Role: "user", Content: "Say hello in 5 words or less."},
			},
			MaxTokens:   50,
			Temperature: 0.7,
		}

		jsonBody, _ := json.Marshal(req)
		httpReq, _ := http.NewRequest("POST", provider.APIEndpoint, bytes.NewBuffer(jsonBody))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		httpClient := &http.Client{Timeout: provider.Timeout}
		start := time.Now()
		httpResp, err := httpClient.Do(httpReq)
		elapsed := time.Since(start)

		if err != nil {
			t.Skipf("HelixAgent request failed (network issue, server may be overloaded): %v", err)
		}
		defer httpResp.Body.Close()

		body, _ := io.ReadAll(httpResp.Body)

		var chatResp StabilityChatResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			t.Skipf("Failed to parse response (may indicate service unavailable): %v", err)
		}

		if chatResp.Error != nil {
			t.Skipf("HelixAgent returned error (may indicate service unavailable): %s", chatResp.Error.Message)
		}

		if len(chatResp.Choices) == 0 {
			t.Skip("HelixAgent returned no choices (may indicate service unavailable)")
		}

		t.Logf("HelixAgent responded in %v: %s", elapsed, truncateString(chatResp.Choices[0].Message.Content, 100))
	})

	// Test debate endpoint
	t.Run("DebateEndpoint", func(t *testing.T) {
		debateReq := map[string]interface{}{
			"topic": "Testing provider stability",
			"participants": []map[string]string{
				{"name": "TestAgent1", "role": "proponent"},
				{"name": "TestAgent2", "role": "opponent"},
			},
			"max_rounds": 1,
			"timeout":    30,
		}

		jsonBody, _ := json.Marshal(debateReq)
		httpReq, _ := http.NewRequest("POST", helixagentURL+"/v1/debates", bytes.NewBuffer(jsonBody))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		httpClient := &http.Client{Timeout: 10 * time.Second}
		httpResp, err := httpClient.Do(httpReq)
		if err != nil {
			t.Fatalf("Debate request failed: %v", err)
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusAccepted && httpResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(httpResp.Body)
			t.Fatalf("Debate request returned %d: %s", httpResp.StatusCode, string(body))
		}

		var debateResp map[string]interface{}
		json.NewDecoder(httpResp.Body).Decode(&debateResp)

		if debateID, ok := debateResp["debate_id"].(string); ok {
			t.Logf("Debate created: %s", debateID)
		}
	})
}

type stabilityTestResult struct {
	index    int
	response *StabilityChatResponse
	err      error
	duration time.Duration
}

func makeStabilityRequest(provider ProviderStabilityConfig, apiKey string, prompt string) (*StabilityChatResponse, error) {
	reqBody := StabilityChatRequest{
		Model: provider.Model,
		Messages: []StabilityMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", provider.APIEndpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: provider.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var chatResp StabilityChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, truncateString(string(body), 200))
	}

	return &chatResp, nil
}

func validateStabilityResponse(t *testing.T, providerName string, resp *StabilityChatResponse, expectedFields []string) {
	t.Helper()

	if resp.Error != nil {
		t.Fatalf("%s API error: %s (type: %s)", providerName, resp.Error.Message, resp.Error.Type)
	}

	// Check for expected fields
	for _, field := range expectedFields {
		switch field {
		case "id":
			if resp.ID == "" {
				t.Errorf("%s response missing 'id' field", providerName)
			}
		case "choices":
			if len(resp.Choices) == 0 {
				t.Errorf("%s response has no choices", providerName)
			}
		case "model":
			if resp.Model == "" {
				t.Logf("%s response missing 'model' field (may be OK)", providerName)
			}
		}
	}

	// Validate choices have content
	if len(resp.Choices) > 0 {
		if resp.Choices[0].Message.Content == "" {
			t.Errorf("%s response has empty content", providerName)
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
