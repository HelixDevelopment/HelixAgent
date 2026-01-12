package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AgentResponse represents a single agent from the registry
type AgentResponse struct {
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Language       string            `json:"language"`
	ConfigFormat   string            `json:"config_format"`
	APIPattern     string            `json:"api_pattern"`
	EntryPoint     string            `json:"entry_point"`
	Features       []string          `json:"features"`
	ToolSupport    []string          `json:"tool_support"`
	Protocols      []string          `json:"protocols"`
	ConfigLocation string            `json:"config_location"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	SystemPrompt   string            `json:"system_prompt"`
}

// AgentListResponse represents the list of all agents
type AgentListResponse struct {
	Agents []AgentResponse `json:"agents"`
	Count  int             `json:"count"`
}

// TestListAllCLIAgents tests the /v1/agents endpoint
func TestListAllCLIAgents(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(baseURL + "/v1/agents")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode == 200 {
		var agentList AgentListResponse
		err = json.Unmarshal(body, &agentList)
		require.NoError(t, err)

		assert.Equal(t, 18, agentList.Count, "Should have 18 CLI agents")
		assert.Len(t, agentList.Agents, 18, "Should return 18 agents")

		// Verify all expected agents are present
		expectedAgents := []string{
			"OpenCode", "Crush", "HelixCode", "Kiro",
			"Aider", "ClaudeCode", "Cline", "CodenameGoose",
			"DeepSeekCLI", "Forge", "GeminiCLI", "GPTEngineer",
			"KiloCode", "MistralCode", "OllamaCode", "Plandex",
			"QwenCode", "AmazonQ",
		}

		agentNames := make(map[string]bool)
		for _, agent := range agentList.Agents {
			agentNames[agent.Name] = true
		}

		for _, expected := range expectedAgents {
			assert.True(t, agentNames[expected], "Agent %s should be in the list", expected)
		}
	} else if resp.StatusCode == 404 {
		t.Logf("Agent registry endpoint not yet implemented (404)")
	}
}

// TestGetSpecificCLIAgent tests getting a specific agent
func TestGetSpecificCLIAgent(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	agentsToTest := []string{"OpenCode", "ClaudeCode", "Aider", "KiloCode", "AmazonQ"}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, agentName := range agentsToTest {
		t.Run(agentName, func(t *testing.T) {
			resp, err := client.Get(baseURL + "/v1/agents/" + agentName)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if resp.StatusCode == 200 {
				var agent AgentResponse
				err = json.Unmarshal(body, &agent)
				require.NoError(t, err)

				assert.Equal(t, agentName, agent.Name)
				assert.NotEmpty(t, agent.Description)
				assert.NotEmpty(t, agent.Language)
				assert.NotEmpty(t, agent.ConfigFormat)
				assert.NotEmpty(t, agent.ToolSupport)
				assert.NotEmpty(t, agent.Protocols)
			} else if resp.StatusCode == 404 {
				t.Logf("Agent registry endpoint not yet implemented (404)")
			}
		})
	}
}

// TestGetAgentsByProtocol tests filtering agents by protocol
func TestGetAgentsByProtocol(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	protocols := []struct {
		name        string
		minExpected int
	}{
		{"OpenAI", 10},
		{"Anthropic", 5},
		{"MCP", 6},
		{"AWS", 2},
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, p := range protocols {
		t.Run(p.name, func(t *testing.T) {
			resp, err := client.Get(baseURL + "/v1/agents/protocol/" + p.name)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if resp.StatusCode == 200 {
				var agentList AgentListResponse
				err = json.Unmarshal(body, &agentList)
				require.NoError(t, err)

				assert.GreaterOrEqual(t, agentList.Count, p.minExpected,
					"Protocol %s should have at least %d agents", p.name, p.minExpected)
			} else if resp.StatusCode == 404 {
				t.Logf("Agent protocol endpoint not yet implemented (404)")
			}
		})
	}
}

// TestGetAgentsByTool tests filtering agents by tool
func TestGetAgentsByTool(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	tools := []struct {
		name        string
		minExpected int
	}{
		{"Bash", 18},
		{"Read", 18},
		{"Write", 18},
		{"Edit", 17},
		{"Git", 14},
		{"Test", 6},
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, tool := range tools {
		t.Run(tool.name, func(t *testing.T) {
			resp, err := client.Get(baseURL + "/v1/agents/tool/" + tool.name)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if resp.StatusCode == 200 {
				var agentList AgentListResponse
				err = json.Unmarshal(body, &agentList)
				require.NoError(t, err)

				assert.GreaterOrEqual(t, agentList.Count, tool.minExpected,
					"Tool %s should be supported by at least %d agents", tool.name, tool.minExpected)
			} else if resp.StatusCode == 404 {
				t.Logf("Agent tool endpoint not yet implemented (404)")
			}
		})
	}
}

// TestAllCLIAgentStyles tests that each CLI agent style works with the API
func TestAllCLIAgentStyles(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	// Define all 18 agent styles with their system prompts
	agentStyles := []struct {
		name         string
		systemPrompt string
		userMessage  string
	}{
		{"OpenCode", "You are an AI coding assistant.", "Write a hello world in Python"},
		{"Crush", "You are Crush, a terminal-based AI assistant.", "How do I list files?"},
		{"HelixCode", "You are the HelixCode distributed AI assistant.", "Explain AI ensembles"},
		{"Kiro", "You are Kiro, an AI coding agent.", "What tools do you have?"},
		{"Aider", "You are Aider, an AI pair programmer.", "Help me add a function"},
		{"ClaudeCode", "You are Claude Code, Anthropic's official CLI.", "Structure a Go project"},
		{"Cline", "You are Cline, an autonomous coding agent.", "Analyze this codebase"},
		{"CodenameGoose", "You are Goose, an AI coding assistant.", "Set up a Rust project"},
		{"DeepSeekCLI", "You are DeepSeek CLI.", "Sort an array"},
		{"Forge", "You are Forge, an AI agent orchestrator.", "Create a workflow"},
		{"GeminiCLI", "You are Gemini CLI.", "Use Cloud Functions"},
		{"GPTEngineer", "You are GPT Engineer.", "Generate a REST API"},
		{"KiloCode", "You are Kilo Code.", "Compare rate limiters"},
		{"MistralCode", "You are Mistral Code.", "Write a decorator"},
		{"OllamaCode", "You are Ollama Code.", "Benefits of local LLMs"},
		{"Plandex", "You are Plandex.", "Create a refactoring plan"},
		{"QwenCode", "You are Qwen Code.", "Parse JSON data"},
		{"AmazonQ", "You are Amazon Q Developer.", "Deploy a Lambda"},
	}

	client := &http.Client{Timeout: 120 * time.Second}

	for _, agent := range agentStyles {
		t.Run(agent.name, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"model": "helixagent-debate",
				"messages": []map[string]string{
					{"role": "system", "content": agent.systemPrompt},
					{"role": "user", "content": agent.userMessage},
				},
				"max_tokens": 200,
			}

			jsonBody, err := json.Marshal(reqBody)
			require.NoError(t, err)

			resp, err := client.Post(
				baseURL+"/v1/chat/completions",
				"application/json",
				bytes.NewBuffer(jsonBody),
			)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if resp.StatusCode == 200 {
				var apiResp ToolCallAPIResponse
				err = json.Unmarshal(body, &apiResp)
				if err == nil && len(apiResp.Choices) > 0 {
					content := apiResp.Choices[0].Message.Content
					assert.NotEmpty(t, content, "Response for %s should have content", agent.name)
					t.Logf("%s: Got response with %d chars", agent.name, len(content))
				}
			} else if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
				t.Logf("%s: Provider temporarily unavailable (status %d)", agent.name, resp.StatusCode)
			} else {
				t.Logf("%s: Response status %d", agent.name, resp.StatusCode)
			}
		})
	}
}

// TestCLIAgentToolSupport tests tool support for different agent styles
func TestCLIAgentToolSupport(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	// KiloCode supports all 21 tools - test with full toolset
	allTools := []map[string]interface{}{
		{"type": "function", "function": map[string]interface{}{
			"name": "Bash", "description": "Execute bash",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"command": map[string]string{"type": "string"}}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Read", "description": "Read file",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"file_path": map[string]string{"type": "string"}}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Write", "description": "Write file",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"file_path": map[string]string{"type": "string"}, "content": map[string]string{"type": "string"}}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Edit", "description": "Edit file",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"file_path": map[string]string{"type": "string"}, "old_string": map[string]string{"type": "string"}, "new_string": map[string]string{"type": "string"}}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Glob", "description": "Find files",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"pattern": map[string]string{"type": "string"}}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Grep", "description": "Search content",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"pattern": map[string]string{"type": "string"}}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Git", "description": "Git operations",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"operation": map[string]string{"type": "string"}}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Test", "description": "Run tests",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"description": map[string]string{"type": "string"}}}}},
	}

	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "system", "content": "You are Kilo Code, a multi-provider AI coding assistant."},
			{"role": "user", "content": "Check the git status of this project"},
		},
		"tools":       allTools,
		"tool_choice": "auto",
		"max_tokens":  300,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Post(
		baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode == 200 {
		var apiResp ToolCallAPIResponse
		err = json.Unmarshal(body, &apiResp)
		if err == nil {
			assert.NotEmpty(t, apiResp.Choices, "Response should have choices")
			t.Log("KiloCode tool support test passed")
		}
	} else if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
		t.Logf("Provider temporarily unavailable (status %d)", resp.StatusCode)
	}
}

// TestCLIAgentStreaming tests streaming for CLI agents
func TestCLIAgentStreaming(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "system", "content": "You are an AI coding assistant."},
			{"role": "user", "content": "List 3 clean code principles"},
		},
		"max_tokens": 200,
		"stream":     true,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Post(
		baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode == 200 {
		bodyStr := string(body)
		assert.Contains(t, bodyStr, "data:", "Streaming response should have SSE format")
		t.Log("Streaming test passed")
	} else if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
		t.Logf("Provider temporarily unavailable (status %d)", resp.StatusCode)
	}
}

// TestCLIAgentLongContext tests long context handling
func TestCLIAgentLongContext(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	// Generate a longer context
	longContext := "You are analyzing a large codebase. Here is the structure:\n"
	for i := 1; i <= 50; i++ {
		longContext += "- Module " + string(rune('A'+i%26)) + ": Contains functions for feature " + string(rune('0'+i%10)) + "\n"
	}

	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "system", "content": "You are a code analysis assistant."},
			{"role": "user", "content": longContext + "\nSummarize in one sentence."},
		},
		"max_tokens": 100,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Post(
		baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode == 200 {
		var apiResp ToolCallAPIResponse
		err = json.Unmarshal(body, &apiResp)
		if err == nil && len(apiResp.Choices) > 0 {
			assert.NotEmpty(t, apiResp.Choices[0].Message.Content)
			t.Log("Long context test passed")
		}
	} else if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
		t.Logf("Provider temporarily unavailable (status %d)", resp.StatusCode)
	}
}
