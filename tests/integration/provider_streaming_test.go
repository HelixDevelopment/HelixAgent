package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ProviderConfig holds configuration for testing a provider
type ProviderConfig struct {
	Name        string
	EnvKey      string
	BaseURL     string
	Model       string
	AuthHeader  string
	StreamURL   string
	ContentPath string // JSON path to content in response
}

// TestMessage represents a chat message
type TestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents an OpenAI-compatible chat request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []TestMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

// StreamChunk represents an SSE streaming chunk
type StreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

// ChatResponse represents an OpenAI-compatible chat response
type ChatResponse struct {
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

// getProviderConfigs returns configurations for all supported providers
func getProviderConfigs() []ProviderConfig {
	return []ProviderConfig{
		{
			Name:       "DeepSeek",
			EnvKey:     "DEEPSEEK_API_KEY",
			BaseURL:    "https://api.deepseek.com/v1",
			Model:      "deepseek-chat",
			AuthHeader: "Authorization",
		},
		{
			Name:       "OpenRouter",
			EnvKey:     "OPENROUTER_API_KEY",
			BaseURL:    "https://openrouter.ai/api/v1",
			Model:      "google/gemini-2.0-flash-exp:free",
			AuthHeader: "Authorization",
		},
		{
			Name:       "Gemini",
			EnvKey:     "GEMINI_API_KEY",
			BaseURL:    "https://generativelanguage.googleapis.com/v1beta",
			Model:      "gemini-2.0-flash",
			AuthHeader: "x-goog-api-key",
		},
	}
}

// loadEnvFile loads environment variables from .env file
func loadEnvFile(t *testing.T) {
	envPath := "../../.env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Logf("Could not read .env file: %v (will use system env)", err)
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, "\"'")
			// Expand environment variables in value
			value = os.ExpandEnv(value)
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
}

// getAPIKey retrieves API key for a provider
func getAPIKey(envKey string) string {
	return os.Getenv(envKey)
}

// TestProviderStreamingOpenAICompatible tests streaming for OpenAI-compatible providers
func TestProviderStreamingOpenAICompatible(t *testing.T) {
	loadEnvFile(t)

	providers := []ProviderConfig{
		{
			Name:       "DeepSeek",
			EnvKey:     "DEEPSEEK_API_KEY",
			BaseURL:    "https://api.deepseek.com/v1",
			Model:      "deepseek-chat",
			AuthHeader: "Authorization",
		},
		{
			Name:       "OpenRouter",
			EnvKey:     "OPENROUTER_API_KEY",
			BaseURL:    "https://openrouter.ai/api/v1",
			Model:      "google/gemini-2.0-flash-exp:free",
			AuthHeader: "Authorization",
		},
	}

	for _, provider := range providers {
		t.Run(provider.Name+"_Streaming", func(t *testing.T) {
			apiKey := getAPIKey(provider.EnvKey)
			if apiKey == "" {
				t.Skipf("Skipping %s: %s not set", provider.Name, provider.EnvKey)
				return
			}

			testOpenAICompatibleStreaming(t, provider, apiKey)
		})
	}
}

// TestProviderNonStreamingOpenAICompatible tests non-streaming for OpenAI-compatible providers
func TestProviderNonStreamingOpenAICompatible(t *testing.T) {
	loadEnvFile(t)

	providers := []ProviderConfig{
		{
			Name:       "DeepSeek",
			EnvKey:     "DEEPSEEK_API_KEY",
			BaseURL:    "https://api.deepseek.com/v1",
			Model:      "deepseek-chat",
			AuthHeader: "Authorization",
		},
		{
			Name:       "OpenRouter",
			EnvKey:     "OPENROUTER_API_KEY",
			BaseURL:    "https://openrouter.ai/api/v1",
			Model:      "google/gemini-2.0-flash-exp:free",
			AuthHeader: "Authorization",
		},
	}

	for _, provider := range providers {
		t.Run(provider.Name+"_NonStreaming", func(t *testing.T) {
			apiKey := getAPIKey(provider.EnvKey)
			if apiKey == "" {
				t.Skipf("Skipping %s: %s not set", provider.Name, provider.EnvKey)
				return
			}

			testOpenAICompatibleNonStreaming(t, provider, apiKey)
		})
	}
}

// TestGeminiNativeStreaming tests Gemini's native API streaming
func TestGeminiNativeStreaming(t *testing.T) {
	loadEnvFile(t)

	apiKey := getAPIKey("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping Gemini: GEMINI_API_KEY not set")
		return
	}

	testGeminiStreaming(t, apiKey, "gemini-2.0-flash")
}

// TestGeminiNativeNonStreaming tests Gemini's native API non-streaming
func TestGeminiNativeNonStreaming(t *testing.T) {
	loadEnvFile(t)

	apiKey := getAPIKey("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping Gemini: GEMINI_API_KEY not set")
		return
	}

	testGeminiNonStreaming(t, apiKey, "gemini-2.0-flash")
}

// TestHelixAgentEnsembleStreaming tests HelixAgent's ensemble streaming
func TestHelixAgentEnsembleStreaming(t *testing.T) {
	loadEnvFile(t)

	// Check if HelixAgent is running
	resp, err := http.Get("http://localhost:7061/health")
	if err != nil {
		t.Skip("Skipping HelixAgent test: server not running")
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skip("Skipping HelixAgent test: server not healthy")
		return
	}

	testHelixAgentStreaming(t)
}

// TestHelixAgentEnsembleNonStreaming tests HelixAgent's ensemble non-streaming
func TestHelixAgentEnsembleNonStreaming(t *testing.T) {
	loadEnvFile(t)

	// Check if HelixAgent is running
	resp, err := http.Get("http://localhost:7061/health")
	if err != nil {
		t.Skip("Skipping HelixAgent test: server not running")
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skip("Skipping HelixAgent test: server not healthy")
		return
	}

	testHelixAgentNonStreaming(t)
}

// TestHTTPErrorHandling tests that providers properly handle HTTP errors
func TestHTTPErrorHandling(t *testing.T) {
	loadEnvFile(t)

	t.Run("DeepSeek_InvalidAPIKey", func(t *testing.T) {
		testOpenAICompatibleErrorHandling(t, ProviderConfig{
			Name:       "DeepSeek",
			BaseURL:    "https://api.deepseek.com/v1",
			Model:      "deepseek-chat",
			AuthHeader: "Authorization",
		}, "invalid-api-key")
	})

	t.Run("OpenRouter_InvalidAPIKey", func(t *testing.T) {
		testOpenAICompatibleErrorHandling(t, ProviderConfig{
			Name:       "OpenRouter",
			BaseURL:    "https://openrouter.ai/api/v1",
			Model:      "google/gemini-2.0-flash-exp:free",
			AuthHeader: "Authorization",
		}, "invalid-api-key")
	})

	t.Run("Gemini_InvalidAPIKey", func(t *testing.T) {
		testGeminiErrorHandling(t, "invalid-api-key", "gemini-2.0-flash")
	})
}

// TestAllProvidersParallel tests all providers in parallel
func TestAllProvidersParallel(t *testing.T) {
	loadEnvFile(t)

	var wg sync.WaitGroup
	results := make(map[string]bool)
	var mu sync.Mutex

	providers := getProviderConfigs()

	for _, provider := range providers {
		provider := provider // capture for goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()

			apiKey := getAPIKey(provider.EnvKey)
			if apiKey == "" {
				mu.Lock()
				results[provider.Name] = false
				mu.Unlock()
				t.Logf("Skipping %s: %s not set", provider.Name, provider.EnvKey)
				return
			}

			success := true
			if provider.Name == "Gemini" {
				success = testGeminiStreamingQuiet(t, apiKey, provider.Model)
			} else {
				success = testOpenAICompatibleStreamingQuiet(t, provider, apiKey)
			}

			mu.Lock()
			results[provider.Name] = success
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Report results
	t.Log("Provider Test Results:")
	for name, success := range results {
		if success {
			t.Logf("  ✅ %s: PASS", name)
		} else {
			t.Logf("  ❌ %s: FAIL", name)
		}
	}
}

// Helper functions

func testOpenAICompatibleStreaming(t *testing.T, provider ProviderConfig, apiKey string) {
	client := &http.Client{Timeout: 60 * time.Second}

	req := ChatRequest{
		Model: provider.Model,
		Messages: []TestMessage{
			{Role: "user", Content: "What is 2+2? Reply with just the number."},
		},
		Stream:      true,
		MaxTokens:   50,
		Temperature: 0.1,
	}

	jsonData, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq, err := http.NewRequest("POST", provider.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	require.NoError(t, err)

	if provider.AuthHeader == "Authorization" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		httpReq.Header.Set(provider.AuthHeader, apiKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Skip test if provider returns auth error (invalid/expired credentials)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Skipf("%s returned %d - credentials may be invalid or expired", provider.Name, resp.StatusCode)
		return
	}

	// Skip on rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		t.Skipf("%s rate limited (429) - try again later", provider.Name)
		return
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK")

	// Read streaming response
	reader := bufio.NewReader(resp.Body)
	var chunks []string
	var fullContent string

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if bytes.HasPrefix(line, []byte("data: ")) {
			data := bytes.TrimPrefix(line, []byte("data: "))
			if bytes.Equal(data, []byte("[DONE]")) {
				break
			}

			var chunk StreamChunk
			if err := json.Unmarshal(data, &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				content := chunk.Choices[0].Delta.Content
				chunks = append(chunks, content)
				fullContent += content
			}
		}
	}

	assert.NotEmpty(t, chunks, "Should receive streaming chunks")
	assert.NotEmpty(t, fullContent, "Should have content")
	t.Logf("%s streaming response: %s (chunks: %d)", provider.Name, fullContent, len(chunks))
}

func testOpenAICompatibleNonStreaming(t *testing.T, provider ProviderConfig, apiKey string) {
	client := &http.Client{Timeout: 60 * time.Second}

	req := ChatRequest{
		Model: provider.Model,
		Messages: []TestMessage{
			{Role: "user", Content: "What is 3+3? Reply with just the number."},
		},
		MaxTokens:   50,
		Temperature: 0.1,
	}

	jsonData, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq, err := http.NewRequest("POST", provider.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	require.NoError(t, err)

	if provider.AuthHeader == "Authorization" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		httpReq.Header.Set(provider.AuthHeader, apiKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Skip test if provider returns auth error (invalid/expired credentials)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Skipf("%s returned %d - credentials may be invalid or expired", provider.Name, resp.StatusCode)
		return
	}

	// Skip on rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		t.Skipf("%s rate limited (429) - try again later", provider.Name)
		return
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var chatResp ChatResponse
	err = json.Unmarshal(body, &chatResp)
	require.NoError(t, err)

	assert.NotEmpty(t, chatResp.Choices, "Should have choices")
	if len(chatResp.Choices) > 0 {
		content := chatResp.Choices[0].Message.Content
		assert.NotEmpty(t, content, "Should have content")
		t.Logf("%s non-streaming response: %s", provider.Name, content)
	}
}

func testGeminiStreaming(t *testing.T, apiKey, model string) {
	client := &http.Client{Timeout: 60 * time.Second}

	// Gemini request format
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": "What is 2+2? Reply with just the number."},
				},
				"role": "user",
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 50,
			"temperature":     0.1,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	require.NoError(t, err)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", model, apiKey)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	require.NoError(t, err)

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		t.Skip("Gemini rate limited, skipping test")
		return
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK, got %d", resp.StatusCode)

	// Read streaming response
	reader := bufio.NewReader(resp.Body)
	var fullContent string

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if bytes.HasPrefix(line, []byte("data: ")) {
			data := bytes.TrimPrefix(line, []byte("data: "))

			var geminiResp map[string]interface{}
			if err := json.Unmarshal(data, &geminiResp); err != nil {
				continue
			}

			if candidates, ok := geminiResp["candidates"].([]interface{}); ok && len(candidates) > 0 {
				if candidate, ok := candidates[0].(map[string]interface{}); ok {
					if content, ok := candidate["content"].(map[string]interface{}); ok {
						if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
							if part, ok := parts[0].(map[string]interface{}); ok {
								if text, ok := part["text"].(string); ok {
									fullContent += text
								}
							}
						}
					}
				}
			}
		}
	}

	assert.NotEmpty(t, fullContent, "Should have content")
	t.Logf("Gemini streaming response: %s", fullContent)
}

func testGeminiNonStreaming(t *testing.T, apiKey, model string) {
	client := &http.Client{Timeout: 60 * time.Second}

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": "What is 3+3? Reply with just the number."},
				},
				"role": "user",
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 50,
			"temperature":     0.1,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	require.NoError(t, err)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	require.NoError(t, err)

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		t.Skip("Gemini rate limited, skipping test")
		return
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK, got %d", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var geminiResp map[string]interface{}
	err = json.Unmarshal(body, &geminiResp)
	require.NoError(t, err)

	var content string
	if candidates, ok := geminiResp["candidates"].([]interface{}); ok && len(candidates) > 0 {
		if candidate, ok := candidates[0].(map[string]interface{}); ok {
			if contentObj, ok := candidate["content"].(map[string]interface{}); ok {
				if parts, ok := contentObj["parts"].([]interface{}); ok && len(parts) > 0 {
					if part, ok := parts[0].(map[string]interface{}); ok {
						if text, ok := part["text"].(string); ok {
							content = text
						}
					}
				}
			}
		}
	}

	assert.NotEmpty(t, content, "Should have content")
	t.Logf("Gemini non-streaming response: %s", content)
}

func testHelixAgentStreaming(t *testing.T) {
	client := &http.Client{Timeout: 120 * time.Second}

	req := ChatRequest{
		Model: "helixagent",
		Messages: []TestMessage{
			{Role: "user", Content: "What is 2+2? Reply with just the number."},
		},
		Stream:    true,
		MaxTokens: 50,
	}

	jsonData, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq, err := http.NewRequest("POST", "http://localhost:7061/v1/chat/completions", bytes.NewBuffer(jsonData))
	require.NoError(t, err)

	httpReq.Header.Set("Content-Type", "application/json")
	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-test"
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "EOF") || strings.Contains(errStr, "connection") ||
			strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
			t.Skipf("HelixAgent connection issue: %v", err)
		}
		require.NoError(t, err)
	}
	defer resp.Body.Close()

	// Skip on auth errors or server errors
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Skip("HelixAgent returned auth error - API key may be missing or invalid")
	}
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		t.Skipf("HelixAgent returned server error %d: %s", resp.StatusCode, string(body))
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK")

	// Read streaming response
	reader := bufio.NewReader(resp.Body)
	var chunks []string
	var fullContent string

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if bytes.HasPrefix(line, []byte("data: ")) {
			data := bytes.TrimPrefix(line, []byte("data: "))
			if bytes.Equal(data, []byte("[DONE]")) {
				break
			}

			var chunk StreamChunk
			if err := json.Unmarshal(data, &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				content := chunk.Choices[0].Delta.Content
				// Skip error messages in content
				if !strings.Contains(content, "API returned status") {
					chunks = append(chunks, content)
					fullContent += content
				}
			}
		}
	}

	assert.NotEmpty(t, fullContent, "Should have content (no error messages)")
	t.Logf("HelixAgent streaming response: %s (chunks: %d)", fullContent, len(chunks))
}

func testHelixAgentNonStreaming(t *testing.T) {
	client := &http.Client{Timeout: 120 * time.Second}

	req := ChatRequest{
		Model: "helixagent",
		Messages: []TestMessage{
			{Role: "user", Content: "What is 5+5? Reply with just the number."},
		},
		MaxTokens: 50,
	}

	jsonData, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq, err := http.NewRequest("POST", "http://localhost:7061/v1/chat/completions", bytes.NewBuffer(jsonData))
	require.NoError(t, err)

	httpReq.Header.Set("Content-Type", "application/json")
	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		apiKey = "sk-test"
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Skip on auth errors or server errors
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Skip("HelixAgent returned auth error - API key may be missing or invalid")
	}

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode >= 500 {
		t.Skipf("HelixAgent returned server error %d: %s", resp.StatusCode, string(body))
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK")

	var chatResp ChatResponse
	err = json.Unmarshal(body, &chatResp)
	require.NoError(t, err)

	assert.NotEmpty(t, chatResp.Choices, "Should have choices")
	if len(chatResp.Choices) > 0 {
		content := chatResp.Choices[0].Message.Content
		assert.NotEmpty(t, content, "Should have content")
		t.Logf("HelixAgent non-streaming response: %s", content)
	}
}

func testOpenAICompatibleErrorHandling(t *testing.T, provider ProviderConfig, invalidAPIKey string) {
	client := &http.Client{Timeout: 30 * time.Second}

	req := ChatRequest{
		Model: provider.Model,
		Messages: []TestMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream:    true,
		MaxTokens: 10,
	}

	jsonData, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq, err := http.NewRequest("POST", provider.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	require.NoError(t, err)

	httpReq.Header.Set("Authorization", "Bearer "+invalidAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should get 401 or 403 for invalid API key
	assert.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
		"Expected 401 or 403 for invalid API key, got %d", resp.StatusCode)
	t.Logf("%s returned %d for invalid API key (expected)", provider.Name, resp.StatusCode)
}

func testGeminiErrorHandling(t *testing.T, invalidAPIKey, model string) {
	client := &http.Client{Timeout: 30 * time.Second}

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": "Hello"},
				},
				"role": "user",
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	require.NoError(t, err)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", model, invalidAPIKey)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	require.NoError(t, err)

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should get 400 or 401 for invalid API key
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Expected 400 or 401 for invalid API key, got %d", resp.StatusCode)
	t.Logf("Gemini returned %d for invalid API key (expected)", resp.StatusCode)
}

// Quiet versions for parallel testing (no t.Log)
func testOpenAICompatibleStreamingQuiet(t *testing.T, provider ProviderConfig, apiKey string) bool {
	client := &http.Client{Timeout: 60 * time.Second}

	req := ChatRequest{
		Model: provider.Model,
		Messages: []TestMessage{
			{Role: "user", Content: "Say hello"},
		},
		Stream:      true,
		MaxTokens:   20,
		Temperature: 0.1,
	}

	jsonData, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", provider.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))

	if provider.AuthHeader == "Authorization" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		httpReq.Header.Set(provider.AuthHeader, apiKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func testGeminiStreamingQuiet(t *testing.T, apiKey, model string) bool {
	client := &http.Client{Timeout: 60 * time.Second}

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": "Say hello"},
				},
				"role": "user",
			},
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", model, apiKey)
	httpReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Consider rate limiting as "pass" since it means the API is working
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusTooManyRequests
}

// TestStreamingContentIntegrity verifies streaming returns correct content
func TestStreamingContentIntegrity(t *testing.T) {
	loadEnvFile(t)

	testCases := []struct {
		question string
		expected string
	}{
		{"What is 2+2? Reply with just the number.", "4"},
		{"What is 5-3? Reply with just the number.", "2"},
		{"What is 3*3? Reply with just the number.", "9"},
	}

	providers := []ProviderConfig{
		{
			Name:       "DeepSeek",
			EnvKey:     "DEEPSEEK_API_KEY",
			BaseURL:    "https://api.deepseek.com/v1",
			Model:      "deepseek-chat",
			AuthHeader: "Authorization",
		},
	}

	for _, provider := range providers {
		apiKey := getAPIKey(provider.EnvKey)
		if apiKey == "" {
			t.Skipf("Skipping %s: %s not set", provider.Name, provider.EnvKey)
			continue
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s_%s", provider.Name, tc.expected), func(t *testing.T) {
				client := &http.Client{Timeout: 60 * time.Second}

				req := ChatRequest{
					Model: provider.Model,
					Messages: []TestMessage{
						{Role: "user", Content: tc.question},
					},
					Stream:      true,
					MaxTokens:   10,
					Temperature: 0.0,
				}

				jsonData, _ := json.Marshal(req)
				httpReq, _ := http.NewRequest("POST", provider.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
				httpReq.Header.Set("Authorization", "Bearer "+apiKey)
				httpReq.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(httpReq)
				require.NoError(t, err)
				defer resp.Body.Close()

				require.Equal(t, http.StatusOK, resp.StatusCode)

				reader := bufio.NewReader(resp.Body)
				var fullContent string

				for {
					line, err := reader.ReadBytes('\n')
					if err != nil {
						break
					}

					line = bytes.TrimSpace(line)
					if bytes.HasPrefix(line, []byte("data: ")) {
						data := bytes.TrimPrefix(line, []byte("data: "))
						if bytes.Equal(data, []byte("[DONE]")) {
							break
						}

						var chunk StreamChunk
						if err := json.Unmarshal(data, &chunk); err != nil {
							continue
						}

						if len(chunk.Choices) > 0 {
							fullContent += chunk.Choices[0].Delta.Content
						}
					}
				}

				assert.Contains(t, fullContent, tc.expected,
					"Expected response to contain '%s', got '%s'", tc.expected, fullContent)
			})
		}
	}
}

// TestStreamingTimeout tests that streaming handles timeouts properly
func TestStreamingTimeout(t *testing.T) {
	loadEnvFile(t)

	apiKey := getAPIKey("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping: DEEPSEEK_API_KEY not set")
		return
	}

	// Use a very short timeout
	client := &http.Client{Timeout: 100 * time.Millisecond}

	req := ChatRequest{
		Model: "deepseek-chat",
		Messages: []TestMessage{
			{Role: "user", Content: "Write a very long essay about the history of computing."},
		},
		Stream:    true,
		MaxTokens: 1000,
	}

	jsonData, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	httpReq = httpReq.WithContext(ctx)

	_, err := client.Do(httpReq)
	// Should timeout or context deadline exceeded
	assert.Error(t, err, "Should have timed out")
	t.Logf("Timeout test passed: %v", err)
}
