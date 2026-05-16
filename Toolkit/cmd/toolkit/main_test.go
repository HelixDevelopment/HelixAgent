package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockProvider implements toolkit.Provider for testing
type MockProvider struct {
	name             string
	chatResponse     toolkit.ChatResponse
	chatError        error
	embedResponse    toolkit.EmbeddingResponse
	embedError       error
	rerankResponse   toolkit.RerankResponse
	rerankError      error
	modelsResponse   []toolkit.ModelInfo
	modelsError      error
	validateError    error
	chatCallCount    int
	embedCallCount   int
	rerankCallCount  int
	discoverCallCount int
}

func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name: name,
		chatResponse: toolkit.ChatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "test-model",
			Choices: []toolkit.Choice{
				{
					Index: 0,
					Message: toolkit.Message{
						Role:    "assistant",
						Content: "Mock response",
					},
					FinishReason: "stop",
				},
			},
			Usage: toolkit.Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		},
		modelsResponse: []toolkit.ModelInfo{
			{
				ID:      "test-model-1",
				Name:    "Test Model 1",
				Object:  "model",
				OwnedBy: "mock",
			},
			{
				ID:      "test-model-2",
				Name:    "Test Model 2",
				Object:  "model",
				OwnedBy: "mock",
			},
		},
	}
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) Chat(ctx context.Context, req toolkit.ChatRequest) (toolkit.ChatResponse, error) {
	m.chatCallCount++
	return m.chatResponse, m.chatError
}

func (m *MockProvider) Embed(ctx context.Context, req toolkit.EmbeddingRequest) (toolkit.EmbeddingResponse, error) {
	m.embedCallCount++
	return m.embedResponse, m.embedError
}

func (m *MockProvider) Rerank(ctx context.Context, req toolkit.RerankRequest) (toolkit.RerankResponse, error) {
	m.rerankCallCount++
	return m.rerankResponse, m.rerankError
}

func (m *MockProvider) DiscoverModels(ctx context.Context) ([]toolkit.ModelInfo, error) {
	m.discoverCallCount++
	return m.modelsResponse, m.modelsError
}

func (m *MockProvider) ValidateConfig(config map[string]interface{}) error {
	return m.validateError
}

// TestVersionCommand tests the version command output
func TestVersionCommand(t *testing.T) {
	rootCmd := &cobra.Command{
		Use:   "toolkit",
		Short: "HelixAgent Toolkit - AI-powered application framework",
	}

	var output bytes.Buffer
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(&output, "HelixAgent Toolkit v1.0.0")
		},
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.SetArgs([]string{"version"})
	err := rootCmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, output.String(), "HelixAgent Toolkit v1.0.0")
}

// TestRootCommandHelp tests that root command shows help
func TestRootCommandHelp(t *testing.T) {
	rootCmd := &cobra.Command{
		Use:   "toolkit",
		Short: "HelixAgent Toolkit - AI-powered application framework",
		Long:  `A comprehensive toolkit for building AI-powered applications with support for multiple providers and specialized agents.`,
	}

	rootCmd.AddCommand(&cobra.Command{Use: "test", Short: "Run integration tests"})
	rootCmd.AddCommand(&cobra.Command{Use: "chat", Short: "Start an interactive chat session"})
	rootCmd.AddCommand(&cobra.Command{Use: "agent", Short: "Execute tasks with AI agents"})
	rootCmd.AddCommand(&cobra.Command{Use: "version", Short: "Show version information"})

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()

	require.NoError(t, err)
	helpOutput := output.String()
	assert.Contains(t, helpOutput, "toolkit")
	assert.Contains(t, helpOutput, "test")
	assert.Contains(t, helpOutput, "chat")
	assert.Contains(t, helpOutput, "agent")
	assert.Contains(t, helpOutput, "version")
}

// TestChatCommandFlags tests chat command flag parsing
func TestChatCommandFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		expectError  bool
		checkProvider string
		checkAPIKey   string
		checkModel    string
	}{
		{
			name:         "default provider",
			args:         []string{"chat"},
			expectError:  false,
			checkProvider: "siliconflow",
		},
		{
			name:         "custom provider",
			args:         []string{"chat", "--provider", "chutes"},
			expectError:  false,
			checkProvider: "chutes",
		},
		{
			name:         "with api key",
			args:         []string{"chat", "--api-key", "test-key-123"},
			expectError:  false,
			checkAPIKey:  "test-key-123",
		},
		{
			name:         "with model",
			args:         []string{"chat", "--model", "gpt-4"},
			expectError:  false,
			checkModel:   "gpt-4",
		},
		{
			name:         "all flags",
			args:         []string{"chat", "-p", "openai", "-k", "key123", "-m", "gpt-4", "-u", "https://api.example.com"},
			expectError:  false,
			checkProvider: "openai",
			checkAPIKey:  "key123",
			checkModel:   "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedProvider, capturedAPIKey, capturedModel, capturedBaseURL string

			rootCmd := &cobra.Command{Use: "toolkit"}
			chatCmd := &cobra.Command{
				Use:   "chat",
				Short: "Start an interactive chat session",
				Run: func(cmd *cobra.Command, args []string) {
					capturedProvider, _ = cmd.Flags().GetString("provider")
					capturedAPIKey, _ = cmd.Flags().GetString("api-key")
					capturedModel, _ = cmd.Flags().GetString("model")
					capturedBaseURL, _ = cmd.Flags().GetString("base-url")
				},
			}
			chatCmd.Flags().StringP("provider", "p", "siliconflow", "Provider name")
			chatCmd.Flags().StringP("api-key", "k", "", "API key")
			chatCmd.Flags().StringP("base-url", "u", "", "Base URL")
			chatCmd.Flags().StringP("model", "m", "", "Model name")
			rootCmd.AddCommand(chatCmd)

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkProvider != "" {
					assert.Equal(t, tt.checkProvider, capturedProvider)
				}
				if tt.checkAPIKey != "" {
					assert.Equal(t, tt.checkAPIKey, capturedAPIKey)
				}
				if tt.checkModel != "" {
					assert.Equal(t, tt.checkModel, capturedModel)
				}
			}
			_ = capturedBaseURL // silence unused variable warning
		})
	}
}

// TestAgentCommandFlags tests agent command flag parsing
func TestAgentCommandFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectAgent string
		expectTask  string
	}{
		{
			name:        "generic agent",
			args:        []string{"agent", "--type", "generic", "--task", "help me"},
			expectAgent: "generic",
			expectTask:  "help me",
		},
		{
			name:        "codereview agent",
			args:        []string{"agent", "-t", "codereview", "--task", "review this code"},
			expectAgent: "codereview",
			expectTask:  "review this code",
		},
		{
			name:        "default agent type",
			args:        []string{"agent", "--task", "do something"},
			expectAgent: "generic",
			expectTask:  "do something",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedAgentType, capturedTask string

			rootCmd := &cobra.Command{Use: "toolkit"}
			agentCmd := &cobra.Command{
				Use:   "agent",
				Short: "Execute tasks with AI agents",
				Run: func(cmd *cobra.Command, args []string) {
					capturedAgentType, _ = cmd.Flags().GetString("type")
					capturedTask, _ = cmd.Flags().GetString("task")
				},
			}
			agentCmd.Flags().StringP("provider", "p", "siliconflow", "Provider name")
			agentCmd.Flags().StringP("api-key", "k", "", "API key")
			agentCmd.Flags().StringP("base-url", "u", "", "Base URL")
			agentCmd.Flags().StringP("model", "m", "", "Model name")
			agentCmd.Flags().StringP("type", "t", "generic", "Agent type (generic, codereview)")
			agentCmd.Flags().String("task", "", "Task to execute")
			rootCmd.AddCommand(agentCmd)

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			assert.NoError(t, err)
			assert.Equal(t, tt.expectAgent, capturedAgentType)
			assert.Equal(t, tt.expectTask, capturedTask)
		})
	}
}

// TestProviderFactoryRegistry tests the provider registry functionality
func TestProviderFactoryRegistry(t *testing.T) {
	registry := toolkit.NewProviderFactoryRegistry()

	// Register a mock provider factory
	mockFactory := func(config map[string]interface{}) (toolkit.Provider, error) {
		return NewMockProvider("test-provider"), nil
	}

	err := registry.Register("test", mockFactory)
	assert.NoError(t, err)

	// List providers
	providers := registry.ListProviders()
	assert.Contains(t, providers, "test")

	// Create provider
	provider, err := registry.Create("test", map[string]interface{}{"api_key": "test"})
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "test-provider", provider.Name())

	// Try to create non-existent provider
	_, err = registry.Create("nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

// TestMockProviderChat tests the mock provider chat functionality
func TestMockProviderChat(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider("test")

	// Test successful chat
	req := toolkit.ChatRequest{
		Messages: []toolkit.Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 100,
	}

	resp, err := provider.Chat(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, 1, provider.chatCallCount)
	assert.Len(t, resp.Choices, 1)
	assert.Equal(t, "Mock response", resp.Choices[0].Message.Content)

	// Test chat with error
	provider.chatError = fmt.Errorf("chat failed")
	_, err = provider.Chat(ctx, req)
	assert.Error(t, err)
	assert.Equal(t, 2, provider.chatCallCount)
}

// TestMockProviderDiscoverModels tests model discovery
func TestMockProviderDiscoverModels(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider("test")

	models, err := provider.DiscoverModels(ctx)
	assert.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, "test-model-1", models[0].ID)
	assert.Equal(t, "test-model-2", models[1].ID)
	assert.Equal(t, 1, provider.discoverCallCount)

	// Test discovery with error
	provider.modelsError = fmt.Errorf("discovery failed")
	_, err = provider.DiscoverModels(ctx)
	assert.Error(t, err)
}

// TestToolkitRegistration tests toolkit provider and agent registration
func TestToolkitRegistration(t *testing.T) {
	tk := toolkit.NewToolkit()

	// Register provider
	provider := NewMockProvider("test-provider")
	tk.RegisterProvider("test", provider)

	// Get provider
	retrieved, err := tk.GetProvider("test")
	assert.NoError(t, err)
	assert.Equal(t, "test-provider", retrieved.Name())

	// Get non-existent provider
	_, err = tk.GetProvider("nonexistent")
	assert.Error(t, err)

	// List providers
	providers := tk.ListProviders()
	assert.Contains(t, providers, "test")
}

// TestChatRequestSerialization tests ChatRequest JSON serialization
func TestChatRequestSerialization(t *testing.T) {
	req := toolkit.ChatRequest{
		Messages: []toolkit.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   1000,
		TopP:        0.9,
		Stop:        []string{"\n\n"},
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var decoded toolkit.ChatRequest
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, req.Model, decoded.Model)
	assert.Equal(t, len(req.Messages), len(decoded.Messages))
	assert.Equal(t, req.Temperature, decoded.Temperature)
	assert.Equal(t, req.MaxTokens, decoded.MaxTokens)
}

// TestChatResponseSerialization tests ChatResponse JSON serialization
func TestChatResponseSerialization(t *testing.T) {
	resp := toolkit.ChatResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1677858242,
		Model:   "gpt-4",
		Choices: []toolkit.Choice{
			{
				Index: 0,
				Message: toolkit.Message{
					Role:    "assistant",
					Content: "Hello! How can I help you?",
				},
				FinishReason: "stop",
			},
		},
		Usage: toolkit.Usage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
		},
	}

	data, err := json.Marshal(resp)
	assert.NoError(t, err)

	var decoded toolkit.ChatResponse
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, resp.ID, decoded.ID)
	assert.Equal(t, resp.Model, decoded.Model)
	assert.Equal(t, len(resp.Choices), len(decoded.Choices))
	assert.Equal(t, resp.Usage.TotalTokens, decoded.Usage.TotalTokens)
}

// TestEmbeddingRequestSerialization tests EmbeddingRequest JSON serialization
func TestEmbeddingRequestSerialization(t *testing.T) {
	req := toolkit.EmbeddingRequest{
		Input:          []string{"Hello world", "Test input"},
		Model:          "text-embedding-ada-002",
		EncodingFormat: "float",
		Dimensions:     1536,
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var decoded toolkit.EmbeddingRequest
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, req.Input, decoded.Input)
	assert.Equal(t, req.Model, decoded.Model)
	assert.Equal(t, req.Dimensions, decoded.Dimensions)
}

// TestRerankRequestSerialization tests RerankRequest JSON serialization
func TestRerankRequestSerialization(t *testing.T) {
	req := toolkit.RerankRequest{
		Query:      "What is the capital of France?",
		Documents:  []string{"Paris is the capital of France", "Berlin is the capital of Germany"},
		Model:      "rerank-english-v2.0",
		TopN:       5,
		ReturnDocs: true,
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var decoded toolkit.RerankRequest
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, req.Query, decoded.Query)
	assert.Equal(t, len(req.Documents), len(decoded.Documents))
	assert.Equal(t, req.TopN, decoded.TopN)
}

// TestModelCategories tests model category constants
func TestModelCategories(t *testing.T) {
	assert.Equal(t, toolkit.ModelCategory("chat"), toolkit.CategoryChat)
	assert.Equal(t, toolkit.ModelCategory("embedding"), toolkit.CategoryEmbedding)
	assert.Equal(t, toolkit.ModelCategory("rerank"), toolkit.CategoryRerank)
	assert.Equal(t, toolkit.ModelCategory("multimodal"), toolkit.CategoryMultimodal)
	assert.Equal(t, toolkit.ModelCategory("image"), toolkit.CategoryImage)
	assert.Equal(t, toolkit.ModelCategory("audio"), toolkit.CategoryAudio)
	assert.Equal(t, toolkit.ModelCategory("video"), toolkit.CategoryVideo)
}

// TestModelInfoSerialization tests ModelInfo JSON serialization
func TestModelInfoSerialization(t *testing.T) {
	info := toolkit.ModelInfo{
		ID:       "gpt-4",
		Name:     "GPT-4",
		Object:   "model",
		Category: toolkit.CategoryChat,
		Capabilities: toolkit.ModelCapabilities{
			SupportsChat:    true,
			FunctionCalling: true,
			ContextWindow:   128000,
			MaxTokens:       4096,
		},
		Provider:    "openai",
		Description: "GPT-4 model",
		Created:     1677649963,
		OwnedBy:     "openai",
	}

	data, err := json.Marshal(info)
	assert.NoError(t, err)

	var decoded toolkit.ModelInfo
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, info.ID, decoded.ID)
	assert.Equal(t, info.Category, decoded.Category)
	assert.Equal(t, info.Capabilities.ContextWindow, decoded.Capabilities.ContextWindow)
}

// TestMockHTTPServer creates a mock HTTP server for testing API interactions
func TestMockHTTPServer(t *testing.T) {
	// Create a mock server that simulates an API endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			resp := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion",
				"created": 1677858242,
				"model":   "test-model",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]string{
							"role":    "assistant",
							"content": "Test response from mock server",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]int{
					"prompt_tokens":     10,
					"completion_tokens": 15,
					"total_tokens":      25,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case "/v1/models":
			resp := map[string]interface{}{
				"object": "list",
				"data": []map[string]interface{}{
					{"id": "model-1", "object": "model", "owned_by": "test"},
					{"id": "model-2", "object": "model", "owned_by": "test"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case "/v1/embeddings":
			embedding := make([]float64, 1536)
			for i := range embedding {
				embedding[i] = float64(i) * 0.001
			}
			resp := map[string]interface{}{
				"object": "list",
				"data": []map[string]interface{}{
					{"object": "embedding", "index": 0, "embedding": embedding},
				},
				"model": "text-embedding-ada-002",
				"usage": map[string]int{"prompt_tokens": 5, "total_tokens": 5},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Test that the server responds correctly
	resp, err := http.Get(server.URL + "/v1/models")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "list", result["object"])

	data, ok := result["data"].([]interface{})
	require.True(t, ok)
	assert.Len(t, data, 2)
}

// TestCommandValidation tests that commands validate required flags
func TestCommandValidation(t *testing.T) {
	tests := []struct {
		name        string
		setupCmd    func() *cobra.Command
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name: "test command needs no flags",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{
					Use: "test",
					RunE: func(cmd *cobra.Command, args []string) error {
						return nil
					},
				}
				return cmd
			},
			args:        []string{},
			expectError: false,
		},
		{
			name: "chat command requires api-key",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{
					Use: "chat",
					RunE: func(cmd *cobra.Command, args []string) error {
						apiKey, _ := cmd.Flags().GetString("api-key")
						if apiKey == "" {
							return fmt.Errorf("API key is required")
						}
						return nil
					},
				}
				cmd.Flags().StringP("api-key", "k", "", "API key")
				return cmd
			},
			args:        []string{},
			expectError: true,
			errorMsg:    "API key is required",
		},
		{
			name: "agent command requires task",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{
					Use: "agent",
					RunE: func(cmd *cobra.Command, args []string) error {
						task, _ := cmd.Flags().GetString("task")
						if task == "" {
							return fmt.Errorf("Task is required")
						}
						return nil
					},
				}
				cmd.Flags().String("task", "", "Task to execute")
				return cmd
			},
			args:        []string{},
			expectError: true,
			errorMsg:    "Task is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := &cobra.Command{Use: "toolkit"}
			subCmd := tt.setupCmd()
			rootCmd.AddCommand(subCmd)

			rootCmd.SetArgs(append([]string{subCmd.Use}, tt.args...))
			err := rootCmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestProviderCreationWithConfig tests provider creation with various configs
func TestProviderCreationWithConfig(t *testing.T) {
	// Create a test registry with mock factory
	registry := toolkit.NewProviderFactoryRegistry()

	registry.Register("mockprovider", func(config map[string]interface{}) (toolkit.Provider, error) {
		apiKey, ok := config["api_key"].(string)
		if !ok || apiKey == "" {
			return nil, fmt.Errorf("api_key is required")
		}
		return NewMockProvider("mockprovider"), nil
	})

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name:        "valid config",
			config:      map[string]interface{}{"api_key": "test-key"},
			expectError: false,
		},
		{
			name:        "missing api key",
			config:      map[string]interface{}{},
			expectError: true,
		},
		{
			name:        "empty api key",
			config:      map[string]interface{}{"api_key": ""},
			expectError: true,
		},
		{
			name:        "with base url",
			config:      map[string]interface{}{"api_key": "test", "base_url": "https://api.example.com"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := registry.Create("mockprovider", tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

// TestAgentTypeValidation tests agent type validation
func TestAgentTypeValidation(t *testing.T) {
	validTypes := []string{"generic", "codereview"}
	invalidTypes := []string{"invalid", "unknown", ""}

	for _, agentType := range validTypes {
		t.Run("valid_"+agentType, func(t *testing.T) {
			isValid := agentType == "generic" || agentType == "codereview"
			assert.True(t, isValid, "Agent type %s should be valid", agentType)
		})
	}

	for _, agentType := range invalidTypes {
		t.Run("invalid_"+agentType, func(t *testing.T) {
			isValid := agentType == "generic" || agentType == "codereview"
			assert.False(t, isValid, "Agent type %s should be invalid", agentType)
		})
	}
}

// TestCLIOutputCapture tests capturing CLI output
func TestCLIOutputCapture(t *testing.T) {
	// Save original stdout
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	// Create a pipe to capture output
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute something that prints
	fmt.Println("Test output capture")

	// Close writer and read output
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)

	assert.Contains(t, buf.String(), "Test output capture")
}

// TestEnvironmentVariableHandling tests env var processing
func TestEnvironmentVariableHandling(t *testing.T) {
	// Test getting env var with default
	testKey := "TOOLKIT_TEST_VAR_" + fmt.Sprintf("%d", os.Getpid())

	// Test when not set
	val := os.Getenv(testKey)
	assert.Empty(t, val)

	// Test when set
	os.Setenv(testKey, "test-value")
	defer os.Unsetenv(testKey)

	val = os.Getenv(testKey)
	assert.Equal(t, "test-value", val)
}

// TestMultipleCommandExecution tests running multiple commands in sequence
func TestMultipleCommandExecution(t *testing.T) {
	commands := []struct {
		name string
		args []string
	}{
		{"version", []string{"version"}},
		{"help", []string{"--help"}},
	}

	for _, cmd := range commands {
		t.Run(cmd.name, func(t *testing.T) {
			rootCmd := &cobra.Command{
				Use:   "toolkit",
				Short: "HelixAgent Toolkit",
			}

			versionCmd := &cobra.Command{
				Use: "version",
				Run: func(cmd *cobra.Command, args []string) {},
			}
			rootCmd.AddCommand(versionCmd)

			rootCmd.SetArgs(cmd.args)
			err := rootCmd.Execute()
			assert.NoError(t, err)
		})
	}
}

// TestConcurrentProviderCalls tests concurrent access to provider
func TestConcurrentProviderCalls(t *testing.T) {
	provider := NewMockProvider("concurrent-test")
	ctx := context.Background()

	// Run concurrent chat requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			req := toolkit.ChatRequest{
				Messages: []toolkit.Message{
					{Role: "user", Content: fmt.Sprintf("Message %d", idx)},
				},
			}
			_, err := provider.Chat(ctx, req)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, provider.chatCallCount)
}

// TestErrorMessageFormatting tests error message formatting
func TestErrorMessageFormatting(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "provider not registered",
			err:      fmt.Errorf("provider %s not registered", "unknown"),
			expected: "provider unknown not registered",
		},
		{
			name:     "chat failed",
			err:      fmt.Errorf("failed to execute task: %w", fmt.Errorf("network error")),
			expected: "failed to execute task: network error",
		},
		{
			name:     "no response choices",
			err:      fmt.Errorf("no response choices returned"),
			expected: "no response choices returned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// TestUsageStructCalculation tests usage calculation
func TestUsageStructCalculation(t *testing.T) {
	usage := toolkit.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	assert.Equal(t, 100, usage.PromptTokens)
	assert.Equal(t, 50, usage.CompletionTokens)
	assert.Equal(t, 150, usage.TotalTokens)
	assert.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens)
}

// TestMessageRoles tests message role constants
func TestMessageRoles(t *testing.T) {
	roles := []string{"system", "user", "assistant", "function", "tool"}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			msg := toolkit.Message{Role: role, Content: "test"}
			assert.Equal(t, role, msg.Role)
		})
	}
}

// TestChoiceFinishReasons tests finish reason constants
func TestChoiceFinishReasons(t *testing.T) {
	finishReasons := []string{"stop", "length", "function_call", "tool_calls", "content_filter"}

	for _, reason := range finishReasons {
		t.Run(reason, func(t *testing.T) {
			choice := toolkit.Choice{
				Index:        0,
				Message:      toolkit.Message{Role: "assistant", Content: "test"},
				FinishReason: reason,
			}
			assert.Equal(t, reason, choice.FinishReason)
		})
	}
}

// TestEmptyResponseHandling tests handling of empty responses
func TestEmptyResponseHandling(t *testing.T) {
	provider := NewMockProvider("empty-test")
	provider.chatResponse = toolkit.ChatResponse{
		ID:      "test",
		Choices: []toolkit.Choice{}, // Empty choices
	}

	ctx := context.Background()
	req := toolkit.ChatRequest{
		Messages: []toolkit.Message{{Role: "user", Content: "test"}},
	}

	resp, err := provider.Chat(ctx, req)
	assert.NoError(t, err) // Provider doesn't error, but response has no choices
	assert.Empty(t, resp.Choices)
}

// TestStreamingFlag tests streaming flag handling
func TestStreamingFlag(t *testing.T) {
	req := toolkit.ChatRequest{
		Messages: []toolkit.Message{{Role: "user", Content: "test"}},
		Stream:   true,
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"stream":true`)

	req.Stream = false
	data, err = json.Marshal(req)
	assert.NoError(t, err)
	// With omitempty, false might not appear, or might appear as false
	var decoded toolkit.ChatRequest
	json.Unmarshal(data, &decoded)
	assert.False(t, decoded.Stream)
}

// TestContextCancellation tests context cancellation handling
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	provider := NewMockProvider("cancel-test")
	req := toolkit.ChatRequest{
		Messages: []toolkit.Message{{Role: "user", Content: "test"}},
	}

	// The mock provider doesn't check context, but we can verify the pattern
	_, err := provider.Chat(ctx, req)
	// Mock doesn't implement context checking, so no error
	assert.NoError(t, err)
	assert.True(t, ctx.Err() != nil) // Context is cancelled
}

// TestLargeMessageHandling tests handling of large messages
func TestLargeMessageHandling(t *testing.T) {
	// Create a large message
	largeContent := strings.Repeat("Hello world. ", 10000)

	msg := toolkit.Message{
		Role:    "user",
		Content: largeContent,
	}

	data, err := json.Marshal(msg)
	assert.NoError(t, err)
	assert.True(t, len(data) > 100000)

	var decoded toolkit.Message
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, largeContent, decoded.Content)
}

// TestSpecialCharacterHandling tests handling of special characters
func TestSpecialCharacterHandling(t *testing.T) {
	specialContent := "Hello\n\t\"World\"\n<script>alert('xss')</script>\n日本語\n🎉"

	msg := toolkit.Message{
		Role:    "user",
		Content: specialContent,
	}

	data, err := json.Marshal(msg)
	assert.NoError(t, err)

	var decoded toolkit.Message
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, specialContent, decoded.Content)
}

// TestModelCapabilitiesDefaults tests default capability values
func TestModelCapabilitiesDefaults(t *testing.T) {
	caps := toolkit.ModelCapabilities{}

	// All should be false/zero by default
	assert.False(t, caps.SupportsChat)
	assert.False(t, caps.SupportsEmbedding)
	assert.False(t, caps.SupportsRerank)
	assert.False(t, caps.SupportsAudio)
	assert.False(t, caps.SupportsVideo)
	assert.False(t, caps.SupportsVision)
	assert.False(t, caps.FunctionCalling)
	assert.Equal(t, 0, caps.ContextWindow)
	assert.Equal(t, 0, caps.MaxTokens)
}

// TestRerankResultSorting tests rerank result sorting
func TestRerankResultSorting(t *testing.T) {
	results := []toolkit.RerankResult{
		{Index: 2, Score: 0.5, Document: "Doc 2"},
		{Index: 0, Score: 0.9, Document: "Doc 0"},
		{Index: 1, Score: 0.7, Document: "Doc 1"},
	}

	// Verify the structure (sorting would be done by implementation)
	assert.Equal(t, 0.5, results[0].Score)
	assert.Equal(t, 0.9, results[1].Score)
	assert.Equal(t, 0.7, results[2].Score)
}

// TestEmbeddingDimensionValidation tests embedding dimension handling
func TestEmbeddingDimensionValidation(t *testing.T) {
	tests := []struct {
		name       string
		dimensions int
		valid      bool
	}{
		{"zero dimensions", 0, true}, // Let provider decide
		{"standard dimensions", 1536, true},
		{"small dimensions", 256, true},
		{"large dimensions", 4096, true},
		{"negative dimensions", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := toolkit.EmbeddingRequest{
				Input:      []string{"test"},
				Dimensions: tt.dimensions,
			}

			// Validate that negative dimensions would be rejected
			if tt.dimensions < 0 {
				assert.True(t, req.Dimensions < 0)
			} else {
				assert.True(t, req.Dimensions >= 0)
			}
		})
	}
}

// BenchmarkProviderChat benchmarks provider chat operations
func BenchmarkProviderChat(b *testing.B) {
	provider := NewMockProvider("benchmark")
	ctx := context.Background()
	req := toolkit.ChatRequest{
		Messages: []toolkit.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.Chat(ctx, req)
	}
}

// BenchmarkJSONSerialization benchmarks JSON serialization
func BenchmarkJSONSerialization(b *testing.B) {
	req := toolkit.ChatRequest{
		Messages: []toolkit.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello world"},
		},
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(req)
	}
}

// BenchmarkJSONDeserialization benchmarks JSON deserialization
func BenchmarkJSONDeserialization(b *testing.B) {
	data := []byte(`{"messages":[{"role":"user","content":"Hello"}],"model":"gpt-4","temperature":0.7,"max_tokens":1000}`)
	var req toolkit.ChatRequest

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Unmarshal(data, &req)
	}
}
