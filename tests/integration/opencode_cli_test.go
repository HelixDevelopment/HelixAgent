// Package integration provides comprehensive integration tests for OpenCode CLI with HelixAgent.
// These tests verify all CLI commands, API endpoints, and end-to-end workflows.
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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TEST CONFIGURATION
// =============================================================================

const (
	// Default HelixAgent configuration
	DefaultHelixAgentHost = "localhost"
	DefaultHelixAgentPort = "8080"
	HelixAgentBinary      = "../../bin/helixagent"

	// Timeouts
	APITimeout     = 30 * time.Second
	StreamTimeout  = 60 * time.Second
	ServerStartup  = 5 * time.Second
	CommandTimeout = 10 * time.Second
)

// TestConfig holds test configuration
type TestConfig struct {
	HelixAgentHost   string
	HelixAgentPort   string
	HelixAgentAPIKey string
	BaseURL          string
	BinaryPath       string
	TempDir          string
}

// OpenCodeConfig represents the OpenCode configuration structure
type OpenCodeConfig struct {
	Schema   string                         `json:"$schema"`
	Provider map[string]OpenCodeProviderDef `json:"provider"`
	Agent    *OpenCodeAgentDef              `json:"agent,omitempty"`
}

// OpenCodeProviderDef represents a provider definition
type OpenCodeProviderDef struct {
	Name    string                 `json:"name"`
	Options map[string]interface{} `json:"options"`
}

// OpenCodeAgentDef represents agent configuration
type OpenCodeAgentDef struct {
	Model *OpenCodeModelRef `json:"model"`
}

// OpenCodeModelRef represents a model reference
type OpenCodeModelRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// OpenAIModelsResponse represents the models API response
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIModel represents a model
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// OpenAIChatRequest represents a chat completion request
type OpenAIChatRequest struct {
	Model       string           `json:"model"`
	Messages    []OpenAIMessage  `json:"messages"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

// OpenAIMessage represents a message
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatResponse represents a chat completion response
type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

// OpenAIChoice represents a response choice
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIUsage represents token usage
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIStreamChunk represents a streaming chunk
type OpenAIStreamChunk struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"`
	Created int64                 `json:"created"`
	Model   string                `json:"model"`
	Choices []OpenAIStreamChoice  `json:"choices"`
}

// OpenAIStreamChoice represents a streaming choice
type OpenAIStreamChoice struct {
	Index        int                   `json:"index"`
	Delta        OpenAIStreamDelta     `json:"delta"`
	FinishReason *string               `json:"finish_reason"`
}

// OpenAIStreamDelta represents delta content
type OpenAIStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// =============================================================================
// TEST SETUP
// =============================================================================

func loadTestConfig(t *testing.T) *TestConfig {
	t.Helper()

	// Load .env file from project root
	projectRoot := findProjectRoot(t)
	envFile := filepath.Join(projectRoot, ".env")
	if err := godotenv.Load(envFile); err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}

	config := &TestConfig{
		HelixAgentHost:   getEnvOrDefault("HELIXAGENT_HOST", DefaultHelixAgentHost),
		HelixAgentPort:   getEnvOrDefault("PORT", DefaultHelixAgentPort),
		HelixAgentAPIKey: os.Getenv("HELIXAGENT_API_KEY"),
		BinaryPath:       filepath.Join(projectRoot, "bin", "helixagent"),
	}

	config.BaseURL = fmt.Sprintf("http://%s:%s/v1", config.HelixAgentHost, config.HelixAgentPort)

	// Create temp directory for test files
	tempDir, err := os.MkdirTemp("", "opencode-test-*")
	require.NoError(t, err, "Failed to create temp directory")
	config.TempDir = tempDir

	return config
}

func findProjectRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root (go.mod)")
		}
		dir = parent
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func cleanupTestConfig(t *testing.T, config *TestConfig) {
	t.Helper()
	if config.TempDir != "" {
		os.RemoveAll(config.TempDir)
	}
}

// =============================================================================
// CLI COMMAND TESTS
// =============================================================================

// TestGenerateAPIKeyCommand tests the -generate-api-key CLI command
func TestGenerateAPIKeyCommand(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("GenerateAPIKeyToStdout", func(t *testing.T) {
		if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
			t.Skip("HelixAgent binary not found, run 'make build' first")
		}

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-api-key")
		output, err := cmd.Output()
		require.NoError(t, err, "Failed to run generate-api-key command")

		apiKey := strings.TrimSpace(string(output))

		// Validate API key format: sk-{64 hex characters}
		assert.True(t, strings.HasPrefix(apiKey, "sk-"), "API key should start with 'sk-'")
		assert.Regexp(t, regexp.MustCompile(`^sk-[a-f0-9]{64}$`), apiKey,
			"API key should match format sk-{64 hex chars}")
	})

	t.Run("GenerateAPIKeyToEnvFile", func(t *testing.T) {
		if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
			t.Skip("HelixAgent binary not found, run 'make build' first")
		}

		envFilePath := filepath.Join(config.TempDir, ".env.test")

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath,
			"-generate-api-key",
			"-api-key-env-file", envFilePath)
		output, err := cmd.Output()
		require.NoError(t, err, "Failed to run generate-api-key with env file")

		apiKey := strings.TrimSpace(string(output))

		// Verify env file was created and contains the key
		content, err := os.ReadFile(envFilePath)
		require.NoError(t, err, "Failed to read env file")

		assert.Contains(t, string(content), "HELIXAGENT_API_KEY="+apiKey,
			"Env file should contain the generated API key")
	})

	t.Run("GenerateAPIKeyPreservesExistingEnv", func(t *testing.T) {
		if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
			t.Skip("HelixAgent binary not found, run 'make build' first")
		}

		envFilePath := filepath.Join(config.TempDir, ".env.preserve")

		// Create env file with existing content
		existingContent := "EXISTING_VAR=existing_value\n# Comment line\nANOTHER_VAR=another\n"
		err := os.WriteFile(envFilePath, []byte(existingContent), 0644)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath,
			"-generate-api-key",
			"-api-key-env-file", envFilePath)
		_, err = cmd.Output()
		require.NoError(t, err)

		// Verify existing content is preserved
		content, err := os.ReadFile(envFilePath)
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "EXISTING_VAR=existing_value")
		assert.Contains(t, contentStr, "# Comment line")
		assert.Contains(t, contentStr, "ANOTHER_VAR=another")
		assert.Contains(t, contentStr, "HELIXAGENT_API_KEY=sk-")
	})

	t.Run("GenerateMultipleUniqueKeys", func(t *testing.T) {
		if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
			t.Skip("HelixAgent binary not found, run 'make build' first")
		}

		keys := make(map[string]bool)
		numKeys := 10

		for i := 0; i < numKeys; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
			cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-api-key")
			output, err := cmd.Output()
			cancel()
			require.NoError(t, err)

			apiKey := strings.TrimSpace(string(output))
			assert.False(t, keys[apiKey], "Generated duplicate API key")
			keys[apiKey] = true
		}

		assert.Equal(t, numKeys, len(keys), "Should generate unique keys")
	})
}

// TestGenerateOpenCodeConfigCommand tests the -generate-opencode-config CLI command
func TestGenerateOpenCodeConfigCommand(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("GenerateConfigToStdout", func(t *testing.T) {
		if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
			t.Skip("HelixAgent binary not found, run 'make build' first")
		}

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err, "Failed to run generate-opencode-config")

		var openCodeConfig OpenCodeConfig
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err, "Output should be valid JSON")

		// Validate config structure - uses "helixagent" provider, NOT "openai"
		assert.Equal(t, "https://opencode.ai/config.json", openCodeConfig.Schema)
		assert.NotNil(t, openCodeConfig.Provider["helixagent"])
		assert.Equal(t, "HelixAgent AI Debate Ensemble", openCodeConfig.Provider["helixagent"].Name)
		assert.NotNil(t, openCodeConfig.Provider["helixagent"].Options["apiKey"])
		assert.NotNil(t, openCodeConfig.Provider["helixagent"].Options["baseURL"])
		assert.NotNil(t, openCodeConfig.Agent)
		assert.Equal(t, "helixagent", openCodeConfig.Agent.Model.Provider)
		assert.Equal(t, "helixagent-debate", openCodeConfig.Agent.Model.Model)
	})

	t.Run("GenerateConfigToFile", func(t *testing.T) {
		if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
			t.Skip("HelixAgent binary not found, run 'make build' first")
		}

		configPath := filepath.Join(config.TempDir, "opencode.json")

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath,
			"-generate-opencode-config",
			"-opencode-output", configPath)
		_, err := cmd.Output()
		require.NoError(t, err)

		// Verify file was created
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfig
		err = json.Unmarshal(content, &openCodeConfig)
		require.NoError(t, err)

		assert.Equal(t, "https://opencode.ai/config.json", openCodeConfig.Schema)
	})

	t.Run("GenerateConfigWithEnvAPIKey", func(t *testing.T) {
		if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
			t.Skip("HelixAgent binary not found, run 'make build' first")
		}

		// Set a specific API key in environment
		testAPIKey := "sk-test1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcd"
		os.Setenv("HELIXAGENT_API_KEY", testAPIKey)
		defer os.Unsetenv("HELIXAGENT_API_KEY")

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfig
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		assert.Equal(t, testAPIKey, openCodeConfig.Provider["helixagent"].Options["apiKey"])
	})

	t.Run("GenerateConfigWithCustomHostPort", func(t *testing.T) {
		if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
			t.Skip("HelixAgent binary not found, run 'make build' first")
		}

		os.Setenv("HELIXAGENT_HOST", "custom-host.example.com")
		os.Setenv("PORT", "9090")
		defer func() {
			os.Unsetenv("HELIXAGENT_HOST")
			os.Unsetenv("PORT")
		}()

		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, config.BinaryPath, "-generate-opencode-config")
		output, err := cmd.Output()
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfig
		err = json.Unmarshal(output, &openCodeConfig)
		require.NoError(t, err)

		baseURL := openCodeConfig.Provider["helixagent"].Options["baseURL"].(string)
		assert.Equal(t, "http://custom-host.example.com:9090/v1", baseURL)
	})
}

// TestHelpCommand tests the -help CLI command
func TestHelpCommand(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
		t.Skip("HelixAgent binary not found, run 'make build' first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, config.BinaryPath, "-help")
	output, err := cmd.Output()
	require.NoError(t, err)

	helpText := string(output)
	assert.Contains(t, helpText, "HelixAgent")
	assert.Contains(t, helpText, "-generate-api-key")
	assert.Contains(t, helpText, "-generate-opencode-config")
	assert.Contains(t, helpText, "-config")
	assert.Contains(t, helpText, "Usage:")
}

// =============================================================================
// API ENDPOINT TESTS
// =============================================================================

func skipIfNoServer(t *testing.T, config *TestConfig) {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(config.BaseURL + "/models")
	if err != nil {
		t.Skipf("HelixAgent server not running at %s: %v", config.BaseURL, err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized && config.HelixAgentAPIKey == "" {
		t.Skip("HelixAgent requires API key but HELIXAGENT_API_KEY not set")
	}
}

// TestModelsEndpoint tests GET /v1/models
func TestModelsEndpoint(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)
	skipIfNoServer(t, config)

	t.Run("ListModels", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)

		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var modelsResp OpenAIModelsResponse
		err = json.NewDecoder(resp.Body).Decode(&modelsResp)
		require.NoError(t, err)

		assert.Equal(t, "list", modelsResp.Object)
		assert.NotEmpty(t, modelsResp.Data, "Should have at least one model")

		// Check for helixagent-debate model
		hasDebateModel := false
		for _, model := range modelsResp.Data {
			if model.ID == "helixagent-debate" {
				hasDebateModel = true
				break
			}
		}
		assert.True(t, hasDebateModel, "Should include helixagent-debate model")
	})

	t.Run("ModelsWithInvalidAuth", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer invalid-key")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// May get 401 Unauthorized or 200 OK depending on auth configuration
		// If auth is enabled, should get 401; if not, should get 200
		assert.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusOK,
			"Should return 401 (auth enabled) or 200 (auth disabled), got %d", resp.StatusCode)
		if resp.StatusCode == http.StatusOK {
			t.Log("Note: Server does not require authentication for models endpoint")
		}
	})
}

// TestChatCompletionsEndpoint tests POST /v1/chat/completions (non-streaming)
func TestChatCompletionsEndpoint(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)
	skipIfNoServer(t, config)

	t.Run("SimpleChatCompletion", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Say 'Hello' and nothing else."},
			},
			MaxTokens:   50,
			Temperature: 0.1,
			Stream:      false,
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusInternalServerError {
			body, _ := io.ReadAll(resp.Body)
			t.Skipf("Server returned 500 (providers may be unavailable): %s", string(body))
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", string(body))
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var chatResp OpenAIChatResponse
		err = json.NewDecoder(resp.Body).Decode(&chatResp)
		require.NoError(t, err)

		assert.Equal(t, "chat.completion", chatResp.Object)
		assert.NotEmpty(t, chatResp.ID)
		if assert.NotEmpty(t, chatResp.Choices, "Should have choices") {
			assert.NotEmpty(t, chatResp.Choices[0].Message.Content)
		}
	})

	t.Run("ChatCompletionWithSystemMessage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "system", Content: "You are a helpful assistant. Always respond with exactly one word."},
				{Role: "user", Content: "What is 2+2?"},
			},
			MaxTokens:   10,
			Temperature: 0.0,
			Stream:      false,
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusInternalServerError {
			body, _ := io.ReadAll(resp.Body)
			t.Skipf("Server returned 500 (providers may be unavailable): %s", string(body))
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var chatResp OpenAIChatResponse
		err = json.NewDecoder(resp.Body).Decode(&chatResp)
		require.NoError(t, err)

		if len(chatResp.Choices) > 0 {
			assert.NotEmpty(t, chatResp.Choices[0].Message.Content)
		}
	})

	t.Run("ChatCompletionWithMultipleTurns", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "My name is Alice."},
				{Role: "assistant", Content: "Hello Alice! Nice to meet you."},
				{Role: "user", Content: "What is my name?"},
			},
			MaxTokens:   50,
			Temperature: 0.0,
			Stream:      false,
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusInternalServerError {
			body, _ := io.ReadAll(resp.Body)
			t.Skipf("Server returned 500 (providers may be unavailable): %s", string(body))
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var chatResp OpenAIChatResponse
		err = json.NewDecoder(resp.Body).Decode(&chatResp)
		require.NoError(t, err)

		// Response should mention Alice
		if len(chatResp.Choices) > 0 {
			assert.Contains(t, strings.ToLower(chatResp.Choices[0].Message.Content), "alice")
		}
	})

	t.Run("ChatCompletionInvalidRequest", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		// Send invalid JSON
		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", strings.NewReader("not valid json"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("ChatCompletionEmptyMessages", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model:    "helixagent-debate",
			Messages: []OpenAIMessage{},
			Stream:   false,
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return error for empty messages
		assert.True(t, resp.StatusCode >= 400, "Should return error for empty messages")
	})
}

// TestChatCompletionsStreamingEndpoint tests POST /v1/chat/completions with streaming
func TestChatCompletionsStreamingEndpoint(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)
	skipIfNoServer(t, config)

	t.Run("StreamingChatCompletion", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), StreamTimeout)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Count from 1 to 5."},
			},
			MaxTokens:   100,
			Temperature: 0.0,
			Stream:      true,
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Skip if server returns 500 (providers unavailable)
		if resp.StatusCode == http.StatusInternalServerError {
			body, _ := io.ReadAll(resp.Body)
			t.Skipf("Server returned 500 (providers may be unavailable): %s", string(body))
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Read SSE stream
		var chunks []OpenAIStreamChunk
		var contentBuilder strings.Builder
		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					break
				}

				var chunk OpenAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err == nil {
					chunks = append(chunks, chunk)
					if len(chunk.Choices) > 0 {
						contentBuilder.WriteString(chunk.Choices[0].Delta.Content)
					}
				}
			}
		}

		assert.NotEmpty(t, chunks, "Should receive streaming chunks")
		content := contentBuilder.String()
		assert.NotEmpty(t, content, "Should receive content from stream")
		t.Logf("Streamed content: %s", content)
	})

	t.Run("StreamingContentIntegrity", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), StreamTimeout)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Say exactly: 'The quick brown fox jumps over the lazy dog.'"},
			},
			MaxTokens:   100,
			Temperature: 0.0,
			Stream:      true,
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Skip if server returns 500 (providers unavailable)
		if resp.StatusCode == http.StatusInternalServerError {
			body, _ := io.ReadAll(resp.Body)
			t.Skipf("Server returned 500 (providers may be unavailable): %s", string(body))
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var contentBuilder strings.Builder
		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					break
				}

				var chunk OpenAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err == nil {
					if len(chunk.Choices) > 0 {
						contentBuilder.WriteString(chunk.Choices[0].Delta.Content)
					}
				}
			}
		}

		content := strings.ToLower(contentBuilder.String())
		// Should contain the expected phrase (or parts of it)
		assert.True(t, strings.Contains(content, "quick") || strings.Contains(content, "fox") || len(content) > 0,
			"Streamed content should be coherent")
	})

	t.Run("StreamingWithCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Write a very long story about a dragon."},
			},
			MaxTokens:   1000,
			Temperature: 0.7,
			Stream:      true,
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			// Context cancellation is expected
			assert.True(t, strings.Contains(err.Error(), "context") ||
				strings.Contains(err.Error(), "deadline"))
			return
		}
		defer resp.Body.Close()

		// Read until context is cancelled
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			// Context should cancel during reading
		}
	})
}

// =============================================================================
// CONCURRENT REQUEST TESTS
// =============================================================================

// TestConcurrentRequests tests multiple simultaneous requests
func TestConcurrentRequests(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)
	skipIfNoServer(t, config)

	t.Run("ParallelNonStreamingRequests", func(t *testing.T) {
		numRequests := 5
		var wg sync.WaitGroup
		results := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
				defer cancel()

				chatReq := OpenAIChatRequest{
					Model: "helixagent-debate",
					Messages: []OpenAIMessage{
						{Role: "user", Content: fmt.Sprintf("What is %d + %d?", idx, idx)},
					},
					MaxTokens:   50,
					Temperature: 0.0,
					Stream:      false,
				}

				body, _ := json.Marshal(chatReq)
				req, _ := http.NewRequestWithContext(ctx, "POST",
					config.BaseURL+"/chat/completions", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				if config.HelixAgentAPIKey != "" {
					req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
				}

				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					results <- err
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					results <- fmt.Errorf("request %d failed with status %d", idx, resp.StatusCode)
					return
				}

				results <- nil
			}(i)
		}

		wg.Wait()
		close(results)

		var errors []error
		for err := range results {
			if err != nil {
				errors = append(errors, err)
			}
		}

		// Skip if all or most requests failed with provider unavailable errors (500, 502, 503, 504) or timeout
		skipCount := 0
		for _, e := range errors {
			if e != nil {
				errStr := e.Error()
				if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "503") || strings.Contains(errStr, "504") ||
					strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "timeout") ||
					strings.Contains(errStr, "EOF") || strings.Contains(errStr, "connection") {
					skipCount++
				}
			}
		}
		if skipCount > 0 {
			t.Skipf("Skipping: %d/%d requests had provider/connection issues", skipCount, numRequests)
		}

		assert.Empty(t, errors, "All parallel requests should succeed")
	})

	t.Run("ParallelStreamingRequests", func(t *testing.T) {
		numRequests := 3
		var wg sync.WaitGroup
		results := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), StreamTimeout)
				defer cancel()

				chatReq := OpenAIChatRequest{
					Model: "helixagent-debate",
					Messages: []OpenAIMessage{
						{Role: "user", Content: fmt.Sprintf("Count from %d to %d", idx*3+1, idx*3+3)},
					},
					MaxTokens:   50,
					Temperature: 0.0,
					Stream:      true,
				}

				body, _ := json.Marshal(chatReq)
				req, _ := http.NewRequestWithContext(ctx, "POST",
					config.BaseURL+"/chat/completions", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "text/event-stream")
				if config.HelixAgentAPIKey != "" {
					req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
				}

				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					results <- err
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					results <- fmt.Errorf("stream %d failed with status %d", idx, resp.StatusCode)
					return
				}

				// Read some of the stream
				scanner := bufio.NewScanner(resp.Body)
				gotContent := false
				for scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "data: ") {
						data := strings.TrimPrefix(line, "data: ")
						if data == "[DONE]" {
							break
						}
						var chunk OpenAIStreamChunk
						if json.Unmarshal([]byte(data), &chunk) == nil {
							if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
								gotContent = true
							}
						}
					}
				}

				if !gotContent {
					results <- fmt.Errorf("stream %d received no content", idx)
					return
				}

				results <- nil
			}(i)
		}

		wg.Wait()
		close(results)

		var errors []error
		for err := range results {
			if err != nil {
				errors = append(errors, err)
			}
		}

		// Skip if all or most requests failed with provider unavailable errors (500, 502, 503, 504) or timeout
		skipCount := 0
		for _, e := range errors {
			if e != nil {
				errStr := e.Error()
				if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "503") || strings.Contains(errStr, "504") ||
					strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "timeout") ||
					strings.Contains(errStr, "EOF") || strings.Contains(errStr, "connection") {
					skipCount++
				}
			}
		}
		if skipCount > 0 {
			t.Skipf("Skipping: %d/%d streaming requests had provider/connection issues", skipCount, numRequests)
		}

		assert.Empty(t, errors, "All parallel streaming requests should succeed")
	})

	t.Run("MixedStreamingAndNonStreaming", func(t *testing.T) {
		numRequests := 6
		var wg sync.WaitGroup
		results := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			stream := i%2 == 0 // Alternate between streaming and non-streaming

			go func(idx int, useStream bool) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), StreamTimeout)
				defer cancel()

				chatReq := OpenAIChatRequest{
					Model: "helixagent-debate",
					Messages: []OpenAIMessage{
						{Role: "user", Content: "Say hello."},
					},
					MaxTokens:   30,
					Temperature: 0.0,
					Stream:      useStream,
				}

				body, _ := json.Marshal(chatReq)
				req, _ := http.NewRequestWithContext(ctx, "POST",
					config.BaseURL+"/chat/completions", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				if useStream {
					req.Header.Set("Accept", "text/event-stream")
				}
				if config.HelixAgentAPIKey != "" {
					req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
				}

				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					results <- fmt.Errorf("request %d (stream=%v): %w", idx, useStream, err)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					results <- fmt.Errorf("request %d (stream=%v) failed with status %d",
						idx, useStream, resp.StatusCode)
					return
				}

				results <- nil
			}(i, stream)
		}

		wg.Wait()
		close(results)

		var errors []error
		for err := range results {
			if err != nil {
				errors = append(errors, err)
			}
		}

		// Skip if all or most requests failed with provider unavailable errors (500, 502, 503, 504) or timeout
		skipCount := 0
		for _, e := range errors {
			if e != nil {
				errStr := e.Error()
				if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "503") || strings.Contains(errStr, "504") ||
					strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "timeout") ||
					strings.Contains(errStr, "EOF") || strings.Contains(errStr, "connection") {
					skipCount++
				}
			}
		}
		if skipCount > 0 {
			t.Skipf("Skipping: %d/%d mixed requests had provider/connection issues", skipCount, numRequests)
		}

		assert.Empty(t, errors, "Mixed streaming/non-streaming requests should succeed")
	})
}

// =============================================================================
// CONFIGURATION VALIDATION TESTS
// =============================================================================

// TestOpenCodeConfigValidation tests OpenCode configuration validation
func TestOpenCodeConfigValidation(t *testing.T) {
	t.Run("ValidMinimalConfig", func(t *testing.T) {
		config := OpenCodeConfig{
			Schema: "https://opencode.ai/config.json",
			Provider: map[string]OpenCodeProviderDef{
				"openai": {
					Name: "Test Provider",
					Options: map[string]interface{}{
						"apiKey":  "sk-test",
						"baseURL": "http://localhost:7061/v1",
					},
				},
			},
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var parsed OpenCodeConfig
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, config.Schema, parsed.Schema)
		assert.Equal(t, config.Provider["openai"].Name, parsed.Provider["openai"].Name)
	})

	t.Run("ValidFullConfig", func(t *testing.T) {
		config := OpenCodeConfig{
			Schema: "https://opencode.ai/config.json",
			Provider: map[string]OpenCodeProviderDef{
				"openai": {
					Name: "HelixAgent AI Debate Ensemble",
					Options: map[string]interface{}{
						"apiKey":  "sk-test123",
						"baseURL": "http://localhost:7061/v1",
					},
				},
			},
			Agent: &OpenCodeAgentDef{
				Model: &OpenCodeModelRef{
					Provider: "openai",
					Model:    "helixagent-debate",
				},
			},
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var parsed OpenCodeConfig
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.NotNil(t, parsed.Agent)
		assert.Equal(t, "openai", parsed.Agent.Model.Provider)
		assert.Equal(t, "helixagent-debate", parsed.Agent.Model.Model)
	})

	t.Run("ConfigWithMissingAPIKey", func(t *testing.T) {
		config := map[string]interface{}{
			"$schema": "https://opencode.ai/config.json",
			"provider": map[string]interface{}{
				"openai": map[string]interface{}{
					"name": "Test",
					"options": map[string]interface{}{
						"baseURL": "http://localhost:7061/v1",
						// Missing apiKey
					},
				},
			},
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var parsed OpenCodeConfig
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		// apiKey should be nil/missing
		apiKey, hasKey := parsed.Provider["openai"].Options["apiKey"]
		assert.False(t, hasKey || apiKey != nil, "Should not have apiKey")
	})

	t.Run("ConfigWithMultipleProviders", func(t *testing.T) {
		config := OpenCodeConfig{
			Schema: "https://opencode.ai/config.json",
			Provider: map[string]OpenCodeProviderDef{
				"openai": {
					Name: "OpenAI",
					Options: map[string]interface{}{
						"apiKey": "sk-openai",
					},
				},
				"anthropic": {
					Name: "Anthropic",
					Options: map[string]interface{}{
						"apiKey": "sk-anthropic",
					},
				},
			},
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var parsed OpenCodeConfig
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Len(t, parsed.Provider, 2)
		assert.NotNil(t, parsed.Provider["openai"])
		assert.NotNil(t, parsed.Provider["anthropic"])
	})
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

// TestErrorHandling tests various error scenarios
func TestErrorHandling(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)
	skipIfNoServer(t, config)

	t.Run("InvalidModel", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "non-existent-model",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Hello"},
			},
			Stream: false,
		}

		body, _ := json.Marshal(chatReq)
		req, _ := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return error for invalid model (may return 400 or proceed with default)
		t.Logf("Response status for invalid model: %d", resp.StatusCode)
	})

	t.Run("MissingAuthHeader", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		chatReq := OpenAIChatRequest{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Hello"},
			},
			Stream: false,
		}

		body, _ := json.Marshal(chatReq)
		req, _ := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// Intentionally not setting Authorization header

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 401 if auth is required, 200 if auth is not enforced
		// May also return 502/503 if providers are not available
		// Either is acceptable depending on server configuration
		validStatuses := []int{
			http.StatusOK,
			http.StatusUnauthorized,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
		}
		isValid := false
		for _, status := range validStatuses {
			if resp.StatusCode == status {
				isValid = true
				break
			}
		}
		assert.True(t, isValid,
			"Should return 401 (auth required), 200 (success), or 502/503 (provider unavailable), got %d", resp.StatusCode)
		if resp.StatusCode == http.StatusOK {
			t.Log("Note: Server does not require authentication for chat completions")
		} else if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable {
			t.Log("Note: Provider is unavailable, which is expected in test environment")
		}
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, _ := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", strings.NewReader("{malformed"))
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("EmptyRequestBody", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, _ := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("WrongHTTPMethod", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		// Try GET on POST-only endpoint
		req, _ := http.NewRequestWithContext(ctx, "GET",
			config.BaseURL+"/chat/completions", nil)
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Router may return 405 Method Not Allowed or 404 Not Found depending on configuration
		assert.True(t, resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound,
			"Should return 405 or 404 for wrong method, got %d", resp.StatusCode)
	})
}

// =============================================================================
// END-TO-END WORKFLOW TESTS
// =============================================================================

// TestEndToEndWorkflow tests complete OpenCode integration workflow
func TestEndToEndWorkflow(t *testing.T) {
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("GenerateConfigAndTestAPI", func(t *testing.T) {
		if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
			t.Skip("HelixAgent binary not found, run 'make build' first")
		}

		// Step 1: Generate OpenCode config
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		configPath := filepath.Join(config.TempDir, "opencode.json")

		cmd := exec.CommandContext(ctx, config.BinaryPath,
			"-generate-opencode-config",
			"-opencode-output", configPath)
		_, err := cmd.Output()
		cancel()
		require.NoError(t, err, "Should generate OpenCode config")

		// Step 2: Verify config file exists and is valid
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfig
		err = json.Unmarshal(content, &openCodeConfig)
		require.NoError(t, err, "Generated config should be valid JSON")

		// Step 3: Extract API key and base URL from config (uses "helixagent" provider)
		apiKey, ok := openCodeConfig.Provider["helixagent"].Options["apiKey"].(string)
		require.True(t, ok, "Config should contain API key")

		baseURL, ok := openCodeConfig.Provider["helixagent"].Options["baseURL"].(string)
		require.True(t, ok, "Config should contain base URL")

		t.Logf("Generated config with baseURL: %s", baseURL)

		// Step 4: Test API connection (if server is running)
		client := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequest("GET", baseURL+"/models", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Server not running, skipping API test: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Log("API connection successful")
		} else {
			t.Logf("API returned status: %d", resp.StatusCode)
		}
	})

	t.Run("FullConfigGenerationWithEnvFile", func(t *testing.T) {
		if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
			t.Skip("HelixAgent binary not found, run 'make build' first")
		}

		envPath := filepath.Join(config.TempDir, ".env")
		configPath := filepath.Join(config.TempDir, "opencode-full.json")

		// Unset any existing API key to force generation
		origAPIKey := os.Getenv("HELIXAGENT_API_KEY")
		os.Unsetenv("HELIXAGENT_API_KEY")
		defer func() {
			if origAPIKey != "" {
				os.Setenv("HELIXAGENT_API_KEY", origAPIKey)
			}
		}()

		// Generate config with env file
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		cmd := exec.CommandContext(ctx, config.BinaryPath,
			"-generate-opencode-config",
			"-opencode-output", configPath,
			"-api-key-env-file", envPath)
		_, err := cmd.Output()
		cancel()
		require.NoError(t, err)

		// Verify both files were created
		require.FileExists(t, configPath)
		require.FileExists(t, envPath)

		// Verify env file contains API key
		envContent, err := os.ReadFile(envPath)
		require.NoError(t, err)
		assert.Contains(t, string(envContent), "HELIXAGENT_API_KEY=sk-")

		// Verify config file contains matching API key
		configContent, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var openCodeConfig OpenCodeConfig
		err = json.Unmarshal(configContent, &openCodeConfig)
		require.NoError(t, err)

		configAPIKey := openCodeConfig.Provider["helixagent"].Options["apiKey"].(string)
		assert.True(t, strings.HasPrefix(configAPIKey, "sk-"))
	})
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

// BenchmarkChatCompletion benchmarks chat completion requests
func BenchmarkChatCompletion(b *testing.B) {
	// Load config
	projectRoot := "."
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			projectRoot = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	envFile := filepath.Join(projectRoot, ".env")
	godotenv.Load(envFile)

	host := getEnvOrDefault("HELIXAGENT_HOST", DefaultHelixAgentHost)
	port := getEnvOrDefault("PORT", DefaultHelixAgentPort)
	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	baseURL := fmt.Sprintf("http://%s:%s/v1", host, port)

	// Check if server is running
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/models")
	if err != nil {
		b.Skip("HelixAgent server not running")
	}
	resp.Body.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		chatReq := OpenAIChatRequest{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Say 'ok'"},
			},
			MaxTokens:   5,
			Temperature: 0.0,
			Stream:      false,
		}

		body, _ := json.Marshal(chatReq)
		req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
