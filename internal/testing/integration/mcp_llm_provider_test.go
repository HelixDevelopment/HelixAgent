// Package integration provides integration tests for MCP servers with all LLM providers.
// These tests verify that MCP tools work correctly with each LLM provider.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/testutil"
)

// LLMProviderConfig represents an LLM provider configuration
type LLMProviderConfig struct {
	Name    string
	EnvKey  string
	Model   string
	IsOAuth bool
	IsFree  bool
}

// SupportedLLMProviders defines all supported LLM providers
var SupportedLLMProviders = []LLMProviderConfig{
	{Name: "claude", EnvKey: "CLAUDE_API_KEY", Model: "claude-3-5-sonnet-20241022", IsOAuth: true},
	{Name: "deepseek", EnvKey: "DEEPSEEK_API_KEY", Model: "deepseek-chat"},
	{Name: "gemini", EnvKey: "GEMINI_API_KEY", Model: "gemini-2.0-flash-exp"},
	{Name: "mistral", EnvKey: "MISTRAL_API_KEY", Model: "mistral-large-latest"},
	{Name: "openrouter", EnvKey: "OPENROUTER_API_KEY", Model: "openrouter/auto"},
	{Name: "qwen", EnvKey: "QWEN_API_KEY", Model: "qwen-turbo", IsOAuth: true},
	{Name: "zai", EnvKey: "ZAI_API_KEY", Model: "zai-chat"},
	{Name: "zen", EnvKey: "OPENCODE_API_KEY", Model: "gpt-4o-mini", IsFree: true},
	{Name: "cerebras", EnvKey: "CEREBRAS_API_KEY", Model: "llama3.1-8b"},
	{Name: "ollama", EnvKey: "", Model: "llama3.1:8b"},
}

// LLMClient provides communication with HelixAgent LLM API
type LLMClient struct {
	baseURL    string
	httpClient *http.Client
}

// CompletionRequest represents an LLM completion request
type CompletionRequest struct {
	Provider    string          `json:"provider,omitempty"`
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       []Tool          `json:"tools,omitempty"`
	MCPContext  *MCPToolContext `json:"mcp_context,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Tool represents a tool definition
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef represents a function definition
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// MCPToolContext provides MCP context for LLM requests
type MCPToolContext struct {
	AvailableServers []MCPServerInfo `json:"available_servers"`
	ToolResults      []MCPToolResult `json:"tool_results,omitempty"`
}

// MCPServerInfo represents MCP server information
type MCPServerInfo struct {
	Name  string   `json:"name"`
	Port  int      `json:"port"`
	Tools []string `json:"tools"`
}

// MCPToolResult represents an MCP tool execution result
type MCPToolResult struct {
	Server string      `json:"server"`
	Tool   string      `json:"tool"`
	Result interface{} `json:"result"`
}

// CompletionResponse represents an LLM completion response
type CompletionResponse struct {
	ID      string    `json:"id"`
	Model   string    `json:"model"`
	Choices []Choice  `json:"choices"`
	Usage   *Usage    `json:"usage,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// APIError represents an API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewLLMClient creates a new LLM client
func NewLLMClient(baseURL string) *LLMClient {
	return &LLMClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Complete sends a completion request
func (c *LLMClient) Complete(req *CompletionRequest) (*CompletionResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result CompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListProviders lists available LLM providers
func (c *LLMClient) ListProviders() ([]string, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v1/providers")
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list providers failed with status %d: %s", resp.StatusCode, string(body))
	}

	// GET /v1/providers returns HelixAgent's registered provider set. Every
	// entry in that list IS an available provider — the registry only lists
	// providers it constructed successfully — and each entry carries the
	// provider's capabilities rather than a status flag.
	//
	// This previously filtered on a `status` field equal to "available" or
	// "verified". That field has never existed on this endpoint: per-provider
	// verification state is served separately by /v1/providers/verification
	// (CONST-036/037, LLMsVerifier as the source of truth). The filter
	// therefore discarded every provider unconditionally, so the caller's
	// NotEmpty assertion could never hold. Decode the real contract instead,
	// and treat a nameless entry as the malformed payload it would be.
	var result struct {
		Providers []struct {
			Name            string   `json:"name"`
			SupportedModels []string `json:"supported_models"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	providers := make([]string, 0, len(result.Providers))
	for i, p := range result.Providers {
		if p.Name == "" {
			return nil, fmt.Errorf("malformed /v1/providers payload: entry %d has an empty name", i)
		}
		providers = append(providers, p.Name)
	}

	return providers, nil
}

// TestLLMProviderDiscovery tests that LLM providers are discoverable
func TestLLMProviderDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode") // SKIP-OK: #short-mode
	}
	client := NewLLMClient(helixAgentBaseURL(t))

	providers, err := client.ListProviders()
	require.NoError(t, err, "Failed to list providers")

	assert.NotEmpty(t, providers, "Should have at least one provider")
	t.Logf("Found %d providers: %v", len(providers), providers)
}

// TestLLMProviderCompletion tests each LLM provider with a simple completion
func TestLLMProviderCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode") // SKIP-OK: #short-mode
	}
	client := NewLLMClient(helixAgentBaseURL(t))

	for _, provider := range SupportedLLMProviders {
		t.Run(provider.Name, func(t *testing.T) {
			req := &CompletionRequest{
				Provider: provider.Name,
				Model:    provider.Model,
				Messages: []Message{
					{Role: "user", Content: "Say 'hello' and nothing else."},
				},
				Temperature: 0.1,
				MaxTokens:   50,
			}

			resp, err := client.Complete(req)
			if err != nil {
				t.Skipf("Provider %s not available: %v (SKIP-OK: #infra-unavailable)", provider.Name, err)
				return
			}

			if resp.Error != nil {
				t.Skipf("Provider returned error: %s (SKIP-OK: #unmarked-skip-needs-ticket)", resp.Error.Message)
				return
			}

			require.NotEmpty(t, resp.Choices, "Should have choices")
			assert.NotEmpty(t, resp.Choices[0].Message.Content, "Should have content")

			t.Logf("Provider %s response: %s", provider.Name, resp.Choices[0].Message.Content)
		})
	}
}

// TestMCPContextWithLLMProvider tests providing MCP context to LLM providers
func TestMCPContextWithLLMProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MCP integration test in -short mode") // SKIP-OK: #short-mode
	}
	// First, collect MCP tool results
	mcpContext := &MCPToolContext{
		AvailableServers: make([]MCPServerInfo, 0),
		ToolResults:      make([]MCPToolResult, 0),
	}

	// Test time server
	testutil.RequireExternalService(t, "time", "localhost", "9103")
	timeClient, err := NewMCPClient(9103, 10*time.Second)
	require.NoError(t, err)

	err = timeClient.Initialize()
	require.NoError(t, err)

	tools, _ := timeClient.ListTools()
	mcpContext.AvailableServers = append(mcpContext.AvailableServers, MCPServerInfo{
		Name:  "time",
		Port:  9103,
		Tools: tools,
	})

	resp, err := timeClient.CallTool("get_current_time", map[string]interface{}{"timezone": "UTC"})
	if err == nil && resp.Error == nil {
		var result interface{}
		_ = json.Unmarshal(resp.Result, &result)
		mcpContext.ToolResults = append(mcpContext.ToolResults, MCPToolResult{
			Server: "time",
			Tool:   "get_current_time",
			Result: result,
		})
	}
	_ = timeClient.Close()

	// Now test with each LLM provider
	llmClient := NewLLMClient(helixAgentBaseURL(t))

	for _, provider := range SupportedLLMProviders {
		t.Run(provider.Name, func(t *testing.T) {
			req := &CompletionRequest{
				Provider: provider.Name,
				Model:    provider.Model,
				Messages: []Message{
					{
						Role: "system",
						Content: fmt.Sprintf("You have access to MCP tools. Available servers: %v. "+
							"Tool results: %v", mcpContext.AvailableServers, mcpContext.ToolResults),
					},
					{Role: "user", Content: "What time is it according to the time MCP server?"},
				},
				Temperature: 0.1,
				MaxTokens:   200,
				MCPContext:  mcpContext,
			}

			llmResp, err := llmClient.Complete(req)
			if err != nil {
				t.Skipf("Provider %s not available: %v (SKIP-OK: #infra-unavailable)", provider.Name, err)
				return
			}

			if llmResp.Error != nil {
				t.Skipf("Provider error: %s (SKIP-OK: #unmarked-skip-needs-ticket)", llmResp.Error.Message)
				return
			}

			assert.NotEmpty(t, llmResp.Choices, "Should have response")
			t.Logf("Provider %s MCP context response: %s",
				provider.Name, llmResp.Choices[0].Message.Content)
		})
	}
}

// TestLLMToolCalling tests LLM providers with tool calling capability
func TestLLMToolCalling(t *testing.T) {
	llmClient := NewLLMClient(helixAgentBaseURL(t))

	// Define tools that mirror MCP tools
	tools := []Tool{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_current_time",
				Description: "Get the current time in a specified timezone",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"timezone": map[string]interface{}{
							"type":        "string",
							"description": "The timezone (e.g., UTC, America/New_York)",
						},
					},
					"required": []string{"timezone"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_directory",
				Description: "List files in a directory",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The directory path to list",
						},
					},
					"required": []string{"path"},
				},
			},
		},
	}

	for _, provider := range SupportedLLMProviders {
		t.Run(provider.Name, func(t *testing.T) {
			req := &CompletionRequest{
				Provider: provider.Name,
				Model:    provider.Model,
				Messages: []Message{
					{Role: "user", Content: "What time is it in UTC?"},
				},
				Tools:       tools,
				Temperature: 0.1,
				MaxTokens:   200,
			}

			resp, err := llmClient.Complete(req)
			if err != nil {
				t.Skipf("Provider %s not available: %v (SKIP-OK: #infra-unavailable)", provider.Name, err)
				return
			}

			if resp.Error != nil {
				t.Skipf("Provider error: %s (SKIP-OK: #unmarked-skip-needs-ticket)", resp.Error.Message)
				return
			}

			t.Logf("Provider %s tool-enabled response: %v", provider.Name, resp.Choices)
		})
	}
}

// TestAllMCPServersWithAllProviders tests all MCP servers with all LLM providers
func TestAllMCPServersWithAllProviders(t *testing.T) {
	ctx := context.Background()

	// Collect MCP context from all servers
	mcpContext := &MCPToolContext{
		AvailableServers: make([]MCPServerInfo, 0),
		ToolResults:      make([]MCPToolResult, 0),
	}

	mcpServers := []struct {
		name string
		port int
		tool string
		args map[string]interface{}
	}{
		{name: "time", port: 9103, tool: "get_current_time", args: map[string]interface{}{"timezone": "UTC"}},
		{name: "filesystem", port: 9104, tool: "list_directory", args: map[string]interface{}{"path": "/tmp"}},
		{name: "memory", port: 9105, tool: "create_entities", args: map[string]interface{}{
			"entities": []map[string]interface{}{
				{"name": "integration-test", "entityType": "test", "observations": []string{"test observation"}},
			},
		}},
	}

	connectedServers := 0
	for _, srv := range mcpServers {
		func() {
			client, err := NewMCPClient(srv.port, 10*time.Second)
			if err != nil {
				return
			}
			defer func() { _ = client.Close() }()

			if err := client.Initialize(); err != nil {
				return
			}

			tools, _ := client.ListTools()
			mcpContext.AvailableServers = append(mcpContext.AvailableServers, MCPServerInfo{
				Name:  srv.name,
				Port:  srv.port,
				Tools: tools,
			})

			resp, err := client.CallTool(srv.tool, srv.args)
			if err == nil && resp.Error == nil {
				var result interface{}
				_ = json.Unmarshal(resp.Result, &result)
				mcpContext.ToolResults = append(mcpContext.ToolResults, MCPToolResult{
					Server: srv.name,
					Tool:   srv.tool,
					Result: result,
				})
			}
			connectedServers++
		}()
	}

	if connectedServers == 0 {
		t.Skip("No MCP servers available") // SKIP-OK: #legacy-untriaged
		return
	}

	t.Logf("Connected to %d MCP servers, collected %d tool results",
		connectedServers, len(mcpContext.ToolResults))

	_ = ctx // For cancellation in real implementation

	// Test with each LLM provider
	llmClient := NewLLMClient(helixAgentBaseURL(t))

	for _, provider := range SupportedLLMProviders {
		t.Run(provider.Name, func(t *testing.T) {
			req := &CompletionRequest{
				Provider: provider.Name,
				Model:    provider.Model,
				Messages: []Message{
					{
						Role: "system",
						Content: fmt.Sprintf("You have MCP tool results available. "+
							"Servers: %d, Tool results: %d",
							len(mcpContext.AvailableServers), len(mcpContext.ToolResults)),
					},
					{Role: "user", Content: "Summarize the available MCP tools and their results."},
				},
				Temperature: 0.3,
				MaxTokens:   500,
				MCPContext:  mcpContext,
			}

			resp, err := llmClient.Complete(req)
			if err != nil {
				t.Skipf("Provider %s not available: %v (SKIP-OK: #infra-unavailable)", provider.Name, err)
				return
			}

			if resp.Error != nil {
				t.Skipf("Provider error: %s (SKIP-OK: #unmarked-skip-needs-ticket)", resp.Error.Message)
				return
			}

			assert.NotEmpty(t, resp.Choices, "Should have response")
			t.Logf("Provider %s MCP summary: %s",
				provider.Name, resp.Choices[0].Message.Content[:min(200, len(resp.Choices[0].Message.Content))])
		})
	}
}

// BenchmarkLLMCompletion benchmarks LLM completion with MCP context
func BenchmarkLLMCompletion(b *testing.B) {
	base, probed := resolveHelixAgent()
	if base == "" {
		b.Skipf("HelixAgent provider API not reachable; probed %v", probed)
	}
	llmClient := NewLLMClient(base)

	req := &CompletionRequest{
		Model: "auto",
		Messages: []Message{
			{Role: "user", Content: "Say 'test'."},
		},
		Temperature: 0.1,
		MaxTokens:   10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := llmClient.Complete(req)
		if err != nil {
			b.Skipf("LLM not available: %v", err)
			return
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
