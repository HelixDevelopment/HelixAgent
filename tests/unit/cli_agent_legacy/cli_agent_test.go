// Previously tests/integration/cli_agent_integration_test.go. CONST-030
// forbids mocks in non-unit tests; this file exclusively tests CLI-agent
// registry wiring, protocol parsing, and request/response shaping through
// local MockMCPTransport / MockLSPTransport fakes and in-process
// httptest.NewRecorder. PR26 of the CONST-030 compliance campaign.
package cli_agent_legacy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	"dev.helix.agent/internal/agents"
	"dev.helix.agent/internal/services"
	"digital.vasic.toolschema"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TEST CONSTANTS AND TYPES
// =============================================================================

// All 48 expected CLI agents
var expectedAgents = []string{
	"OpenCode", "Crush", "HelixCode", "Kiro",
	"Aider", "ClaudeCode", "Cline", "CodenameGoose",
	"DeepSeekCLI", "Forge", "GeminiCLI", "GPTEngineer",
	"KiloCode", "MistralCode", "OllamaCode", "Plandex",
	"QwenCode", "AmazonQ",
}

// ProtocolTestCase defines a test case for protocol support
type ProtocolTestCase struct {
	Name           string
	Protocol       string
	ExpectedAgents int
	SampleAgents   []string
}

// ToolTestCase defines a test case for tool support
type ToolTestCase struct {
	Name           string
	Tool           string
	MinExpected    int
	RequiredAgents []string
}

// AgentFormatTestCase defines a test case for agent-specific formats
type AgentFormatTestCase struct {
	Agent        string
	APIPattern   string
	ConfigFormat string
	SampleTools  []string
}

// MockMCPTransport implements MCPTransport for testing
type MockMCPTransport struct {
	Connected     bool
	LastMessage   interface{}
	ResponseQueue []interface{}
	SendError     error
	ReceiveError  error
}

func (m *MockMCPTransport) Send(ctx context.Context, message interface{}) error {
	if m.SendError != nil {
		return m.SendError
	}
	m.LastMessage = message
	return nil
}

func (m *MockMCPTransport) Receive(ctx context.Context) (interface{}, error) {
	if m.ReceiveError != nil {
		return nil, m.ReceiveError
	}
	if len(m.ResponseQueue) > 0 {
		resp := m.ResponseQueue[0]
		m.ResponseQueue = m.ResponseQueue[1:]
		return resp, nil
	}
	return map[string]interface{}{"jsonrpc": "2.0", "result": map[string]interface{}{}}, nil
}

func (m *MockMCPTransport) Close() error {
	m.Connected = false
	return nil
}

func (m *MockMCPTransport) IsConnected() bool {
	return m.Connected
}

// MockLSPTransport implements LSPTransport for testing
type MockLSPTransport struct {
	Connected     bool
	LastMessage   interface{}
	ResponseQueue []interface{}
}

func (m *MockLSPTransport) Send(ctx context.Context, message interface{}) error {
	m.LastMessage = message
	return nil
}

func (m *MockLSPTransport) Receive(ctx context.Context) (interface{}, error) {
	if len(m.ResponseQueue) > 0 {
		resp := m.ResponseQueue[0]
		m.ResponseQueue = m.ResponseQueue[1:]
		return resp, nil
	}
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"result": map[string]interface{}{
			"capabilities": map[string]interface{}{
				"completionProvider": true,
				"hoverProvider":      true,
			},
		},
	}, nil
}

func (m *MockLSPTransport) Close() error {
	m.Connected = false
	return nil
}

func (m *MockLSPTransport) IsConnected() bool {
	return m.Connected
}

// =============================================================================
// AGENT REGISTRY TESTS
// =============================================================================

// TestAgentRegistry_AllAgentsRegistered verifies all 48 CLI agents are registered
func TestAgentRegistry_AllAgentsRegistered(t *testing.T) {
	t.Run("Verify_48_Agents_Registered", func(t *testing.T) {
		registeredAgents := agents.GetAllAgents()
		assert.Equal(t, 48, len(registeredAgents), "Should have exactly 48 CLI agents registered")
	})

	t.Run("Verify_All_Expected_Agents_Present", func(t *testing.T) {
		for _, expectedName := range expectedAgents {
			agent, found := agents.GetAgent(expectedName)
			assert.True(t, found, "Agent %s should be registered", expectedName)
			if found {
				assert.Equal(t, expectedName, agent.Name, "Agent name should match")
			}
		}
	})

	t.Run("Verify_Agent_Names_Match", func(t *testing.T) {
		agentNames := agents.GetAgentNames()
		assert.Equal(t, 48, len(agentNames), "Should return 48 agent names")

		// Sort both for comparison
		sortedExpected := make([]string, len(expectedAgents))
		copy(sortedExpected, expectedAgents)
		sort.Strings(sortedExpected)
		sort.Strings(agentNames)

		for i, name := range sortedExpected {
			assert.Contains(t, agentNames, name, "Expected agent %s should be in registry", name)
			if i < len(agentNames) {
				// Verify the agent is retrievable
				_, found := agents.GetAgent(name)
				assert.True(t, found, "Agent %s should be retrievable", name)
			}
		}
	})
}

// TestAgentRegistry_ConfigurationsValid verifies all agent configurations are valid
func TestAgentRegistry_ConfigurationsValid(t *testing.T) {
	for _, agentName := range expectedAgents {
		t.Run(agentName, func(t *testing.T) {
			agent, found := agents.GetAgent(agentName)
			require.True(t, found, "Agent %s should be registered", agentName)

			// Required fields
			assert.NotEmpty(t, agent.Name, "Agent name should not be empty")
			assert.NotEmpty(t, agent.Description, "Agent description should not be empty")
			assert.NotEmpty(t, agent.Language, "Agent language should not be empty")
			assert.NotEmpty(t, agent.ConfigFormat, "Agent config format should not be empty")
			assert.NotEmpty(t, agent.APIPattern, "Agent API pattern should not be empty")
			assert.NotEmpty(t, agent.EntryPoint, "Agent entry point should not be empty")

			// Should have at least some features
			assert.NotEmpty(t, agent.Features, "Agent should have at least one feature")

			// Should have tool support
			assert.NotEmpty(t, agent.ToolSupport, "Agent should have tool support")

			// Should have protocol support
			assert.NotEmpty(t, agent.Protocols, "Agent should have at least one protocol")

			// Should have config location
			assert.NotEmpty(t, agent.ConfigLocation, "Agent should have config location")

			// System prompt should exist
			assert.NotEmpty(t, agent.SystemPrompt, "Agent should have a system prompt")
		})
	}
}

// TestAgentRegistry_ToolMappings verifies agent tool mappings
func TestAgentRegistry_ToolMappings(t *testing.T) {
	// Define minimum tool requirements per agent category
	coreTools := []string{"Bash", "Read", "Write"}

	for _, agentName := range expectedAgents {
		t.Run(agentName+"_Has_Core_Tools", func(t *testing.T) {
			agent, found := agents.GetAgent(agentName)
			require.True(t, found)

			for _, tool := range coreTools {
				assert.Contains(t, agent.ToolSupport, tool,
					"Agent %s should support core tool %s", agentName, tool)
			}
		})
	}

	t.Run("Advanced_Agents_Have_Extended_Tools", func(t *testing.T) {
		// Agents with extended tool support
		advancedAgents := map[string][]string{
			"KiloCode":   {"PR", "Issue", "Workflow", "Symbols", "References", "Definition"},
			"HelixCode":  {"Task"},
			"Kiro":       {"PR", "Issue", "Workflow"},
			"ClaudeCode": {"Task"},
			"Cline":      {"WebFetch", "Symbols", "References", "Definition"},
			"AmazonQ":    {"WebFetch", "Task"},
		}

		for agentName, expectedTools := range advancedAgents {
			agent, found := agents.GetAgent(agentName)
			require.True(t, found, "Agent %s should exist", agentName)

			for _, tool := range expectedTools {
				assert.Contains(t, agent.ToolSupport, tool,
					"Advanced agent %s should support %s", agentName, tool)
			}
		}
	})
}

// TestAgentRegistry_CaseInsensitiveLookup verifies case-insensitive agent lookup
func TestAgentRegistry_CaseInsensitiveLookup(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"opencode", "OpenCode"},
		{"OPENCODE", "OpenCode"},
		{"OpenCode", "OpenCode"},
		{"claudecode", "ClaudeCode"},
		{"CLAUDECODE", "ClaudeCode"},
		{"amazonq", "AmazonQ"},
		{"kilocode", "KiloCode"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			agent, found := agents.GetAgent(tc.input)
			assert.True(t, found, "Should find agent with input %s", tc.input)
			if found {
				assert.Equal(t, tc.expected, agent.Name, "Agent name should be %s", tc.expected)
			}
		})
	}
}

// =============================================================================
// PROTOCOL SUPPORT TESTS
// =============================================================================

// TestProtocolSupport_MCP tests MCP protocol handling
func TestProtocolSupport_MCP(t *testing.T) {
	t.Run("Agents_With_MCP_Support", func(t *testing.T) {
		mcpAgents := agents.GetAgentsByProtocol("MCP")
		assert.GreaterOrEqual(t, len(mcpAgents), 6, "At least 6 agents should support MCP")

		// Verify specific agents support MCP
		expectedMCPAgents := []string{"OpenCode", "ClaudeCode", "Cline", "Kiro", "KiloCode", "AmazonQ"}
		for _, name := range expectedMCPAgents {
			found := false
			for _, agent := range mcpAgents {
				if agent.Name == name {
					found = true
					break
				}
			}
			assert.True(t, found, "Agent %s should support MCP protocol", name)
		}
	})

	t.Run("MCP_Client_Creation", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		mcpClient := services.NewMCPClient(logger)
		assert.NotNil(t, mcpClient, "MCP client should be created")

		// Verify initial state
		servers := mcpClient.ListServers()
		assert.Empty(t, servers, "No servers should be connected initially")
	})

	t.Run("MCP_Protocol_Request_Structure", func(t *testing.T) {
		// Verify MCP request structure
		request := services.MCPRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/list",
			Params:  map[string]interface{}{},
		}

		assert.Equal(t, "2.0", request.JSONRPC)
		assert.Equal(t, "tools/list", request.Method)
	})

	t.Run("MCP_Tool_Call_Structure", func(t *testing.T) {
		toolCall := services.MCPToolCall{
			Name: "Bash",
			Arguments: map[string]interface{}{
				"command":     "ls -la",
				"description": "List files",
			},
		}

		assert.Equal(t, "Bash", toolCall.Name)
		assert.NotNil(t, toolCall.Arguments)
	})
}

// TestProtocolSupport_LSP tests LSP protocol handling
func TestProtocolSupport_LSP(t *testing.T) {
	t.Run("LSP_Client_Creation", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		lspClient := services.NewLSPClient(logger)
		assert.NotNil(t, lspClient, "LSP client should be created")

		// Verify initial state
		servers := lspClient.ListServers()
		assert.Empty(t, servers, "No servers should be connected initially")
	})

	t.Run("LSP_Manager_Creation", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		lspManager := services.NewLSPManager(nil, nil, logger)
		assert.NotNil(t, lspManager, "LSP manager should be created")

		// Verify default servers are configured
		ctx := context.Background()
		servers, err := lspManager.ListLSPServers(ctx)
		assert.NoError(t, err)
		assert.Greater(t, len(servers), 0, "Should have default LSP server configurations")

		// Check for expected servers
		serverIDs := make([]string, len(servers))
		for i, s := range servers {
			serverIDs[i] = s.ID
		}
		assert.Contains(t, serverIDs, "gopls", "Should have Go LSP server configured")
	})

	t.Run("LSP_Request_Validation", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		lspManager := services.NewLSPManager(nil, nil, logger)
		ctx := context.Background()

		// Test validation
		validRequest := services.LSPRequest{
			ServerID: "gopls",
			Method:   "textDocument/hover",
			Params:   map[string]interface{}{},
		}

		err := lspManager.ValidateLSPRequest(ctx, validRequest)
		assert.NoError(t, err, "Valid LSP request should pass validation")

		// Test invalid request - missing server ID
		invalidRequest := services.LSPRequest{
			Method: "textDocument/hover",
		}
		err = lspManager.ValidateLSPRequest(ctx, invalidRequest)
		assert.Error(t, err, "Request without server ID should fail validation")

		// Test invalid request - missing method
		invalidRequest2 := services.LSPRequest{
			ServerID: "gopls",
		}
		err = lspManager.ValidateLSPRequest(ctx, invalidRequest2)
		assert.Error(t, err, "Request without method should fail validation")
	})
}

// TestProtocolSupport_ACP tests ACP protocol handling
func TestProtocolSupport_ACP(t *testing.T) {
	t.Run("Agents_With_ACP_Support", func(t *testing.T) {
		// HelixCode should support ACP
		agent, found := agents.GetAgent("HelixCode")
		require.True(t, found)
		assert.Contains(t, agent.Protocols, "ACP", "HelixCode should support ACP")
	})

	t.Run("ACP_Manager_Creation", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		acpManager := services.NewACPManager(nil, nil, logger)
		assert.NotNil(t, acpManager, "ACP manager should be created")

		// Verify initial state
		ctx := context.Background()
		servers, err := acpManager.ListACPServers(ctx)
		assert.NoError(t, err)
		assert.Empty(t, servers, "No ACP servers should be configured initially")
	})

	t.Run("ACP_Request_Validation", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		acpManager := services.NewACPManager(nil, nil, logger)

		// Register a test server first
		testServer := &services.ACPServer{
			ID:      "test-server",
			Name:    "Test ACP Server",
			URL:     "http://localhost:8080",
			Enabled: true,
		}
		err := acpManager.RegisterServer(testServer)
		assert.NoError(t, err)

		ctx := context.Background()

		// Test valid request
		validRequest := services.ACPRequest{
			ServerID:   "test-server",
			Action:     "execute",
			Parameters: map[string]interface{}{"command": "test"},
		}
		err = acpManager.ValidateACPRequest(ctx, validRequest)
		assert.NoError(t, err, "Valid ACP request should pass validation")

		// Test invalid request - missing server ID
		invalidRequest := services.ACPRequest{
			Action: "execute",
		}
		err = acpManager.ValidateACPRequest(ctx, invalidRequest)
		assert.Error(t, err, "Request without server ID should fail validation")
	})

	t.Run("ACP_Protocol_Request_Structure", func(t *testing.T) {
		request := services.ACPProtocolRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "agent/execute",
			Params:  map[string]interface{}{"task": "analyze"},
		}

		assert.Equal(t, "2.0", request.JSONRPC)
		assert.Equal(t, "agent/execute", request.Method)
	})
}

// TestProtocolSupport_OpenAI tests OpenAI protocol support
func TestProtocolSupport_OpenAI(t *testing.T) {
	t.Run("Agents_With_OpenAI_Protocol", func(t *testing.T) {
		openAIAgents := agents.GetAgentsByProtocol("OpenAI")
		assert.GreaterOrEqual(t, len(openAIAgents), 10, "At least 10 agents should support OpenAI protocol")

		// Verify expected agents
		expectedOpenAI := []string{"OpenCode", "Crush", "HelixCode", "Kiro", "Cline", "GPTEngineer", "KiloCode", "Plandex"}
		for _, name := range expectedOpenAI {
			found := false
			for _, agent := range openAIAgents {
				if agent.Name == name {
					found = true
					break
				}
			}
			assert.True(t, found, "Agent %s should support OpenAI protocol", name)
		}
	})

	t.Run("OpenAI_Compatible_API_Pattern", func(t *testing.T) {
		// Agents with OpenAI-compatible API pattern
		compatibleAgents := []string{"OpenCode", "Crush", "HelixCode", "Kiro", "Cline", "Plandex"}

		for _, name := range compatibleAgents {
			agent, found := agents.GetAgent(name)
			require.True(t, found)
			assert.Contains(t, agent.APIPattern, "OpenAI", "Agent %s should have OpenAI-compatible API pattern", name)
		}
	})
}

// =============================================================================
// AGENT REQUEST FLOW TESTS
// =============================================================================

// TestAgentRequestFlow_RequestParsing tests request parsing for different agent formats
func TestAgentRequestFlow_RequestParsing(t *testing.T) {
	t.Run("Standard_OpenAI_Format", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "system", "content": "You are a coding assistant."},
				{"role": "user", "content": "Hello"},
			},
			"max_tokens": 100,
		}

		jsonData, err := json.Marshal(reqBody)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "helixagent-debate", parsed["model"])
		messages, ok := parsed["messages"].([]interface{})
		require.True(t, ok)
		assert.Len(t, messages, 2)
	})

	t.Run("Streaming_Request_Format", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Test"},
			},
			"stream":     true,
			"max_tokens": 50,
		}

		jsonData, err := json.Marshal(reqBody)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err)

		assert.True(t, parsed["stream"].(bool), "Stream should be true")
	})

	t.Run("Tool_Call_Request_Format", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "List files"},
			},
			"tools": []map[string]interface{}{
				{
					"type": "function",
					"function": map[string]interface{}{
						"name":        "Bash",
						"description": "Execute shell commands",
						"parameters": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"command": map[string]string{"type": "string"},
							},
						},
					},
				},
			},
			"tool_choice": "auto",
		}

		jsonData, err := json.Marshal(reqBody)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err)

		toolsList, ok := parsed["tools"].([]interface{})
		require.True(t, ok)
		assert.Len(t, toolsList, 1)
	})
}

// TestAgentRequestFlow_ToolCallExtraction tests tool call extraction
func TestAgentRequestFlow_ToolCallExtraction(t *testing.T) {
	t.Run("Extract_Bash_Tool_Call", func(t *testing.T) {
		toolCall := map[string]interface{}{
			"id":   "call_123",
			"type": "function",
			"function": map[string]interface{}{
				"name":      "Bash",
				"arguments": `{"command": "ls -la", "description": "List files"}`,
			},
		}

		fn, ok := toolCall["function"].(map[string]interface{})
		require.True(t, ok)

		assert.Equal(t, "Bash", fn["name"])

		var args map[string]interface{}
		err := json.Unmarshal([]byte(fn["arguments"].(string)), &args)
		require.NoError(t, err)

		assert.Equal(t, "ls -la", args["command"])
		assert.Equal(t, "List files", args["description"])
	})

	t.Run("Extract_Read_Tool_Call", func(t *testing.T) {
		toolCall := map[string]interface{}{
			"id":   "call_456",
			"type": "function",
			"function": map[string]interface{}{
				"name":      "Read",
				"arguments": `{"file_path": "/tmp/test.txt"}`,
			},
		}

		fn := toolCall["function"].(map[string]interface{})
		assert.Equal(t, "Read", fn["name"])

		var args map[string]interface{}
		err := json.Unmarshal([]byte(fn["arguments"].(string)), &args)
		require.NoError(t, err)

		assert.Equal(t, "/tmp/test.txt", args["file_path"])
	})

	t.Run("Extract_Edit_Tool_Call", func(t *testing.T) {
		toolCall := map[string]interface{}{
			"id":   "call_789",
			"type": "function",
			"function": map[string]interface{}{
				"name":      "Edit",
				"arguments": `{"file_path": "/tmp/test.txt", "old_string": "foo", "new_string": "bar"}`,
			},
		}

		fn := toolCall["function"].(map[string]interface{})
		assert.Equal(t, "Edit", fn["name"])

		var args map[string]interface{}
		err := json.Unmarshal([]byte(fn["arguments"].(string)), &args)
		require.NoError(t, err)

		assert.Equal(t, "/tmp/test.txt", args["file_path"])
		assert.Equal(t, "foo", args["old_string"])
		assert.Equal(t, "bar", args["new_string"])
	})
}

// TestAgentRequestFlow_ResponseFormatting tests response formatting per agent type
func TestAgentRequestFlow_ResponseFormatting(t *testing.T) {
	t.Run("Standard_Chat_Response", func(t *testing.T) {
		response := map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "helixagent-debate",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello, how can I help you?",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		}

		jsonData, err := json.Marshal(response)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "chat.completion", parsed["object"])
		choices := parsed["choices"].([]interface{})
		assert.Len(t, choices, 1)
	})

	t.Run("Streaming_Response_Chunk", func(t *testing.T) {
		chunk := map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   "helixagent-debate",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": "Hello",
					},
					"finish_reason": nil,
				},
			},
		}

		jsonData, err := json.Marshal(chunk)
		require.NoError(t, err)
		assert.Contains(t, string(jsonData), "chat.completion.chunk")
	})

	t.Run("Tool_Call_Response", func(t *testing.T) {
		response := map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "helixagent-debate",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": nil,
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_abc",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "Bash",
									"arguments": `{"command": "ls", "description": "List files"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		}

		jsonData, err := json.Marshal(response)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err)

		choices := parsed["choices"].([]interface{})
		choice := choices[0].(map[string]interface{})
		assert.Equal(t, "tool_calls", choice["finish_reason"])
	})
}

// =============================================================================
// AGENT-SPECIFIC FORMAT TESTS
// =============================================================================

// TestAgentSpecificFormats_OpenCode tests OpenCode agent format
func TestAgentSpecificFormats_OpenCode(t *testing.T) {
	agent, found := agents.GetAgent("OpenCode")
	require.True(t, found)

	t.Run("OpenCode_Configuration", func(t *testing.T) {
		assert.Equal(t, "OpenCode", agent.Name)
		assert.Equal(t, "Go", agent.Language)
		assert.Equal(t, "JSON", agent.ConfigFormat)
		assert.Contains(t, agent.APIPattern, "OpenAI")
		assert.Contains(t, agent.ConfigLocation, "opencode.json")
	})

	t.Run("OpenCode_Protocol_Support", func(t *testing.T) {
		assert.Contains(t, agent.Protocols, "OpenAI")
		assert.Contains(t, agent.Protocols, "MCP")
	})

	t.Run("OpenCode_Tool_Support", func(t *testing.T) {
		expectedTools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Test", "Lint"}
		for _, tool := range expectedTools {
			assert.Contains(t, agent.ToolSupport, tool,
				"OpenCode should support %s tool", tool)
		}
	})

	t.Run("OpenCode_Request_Format", func(t *testing.T) {
		// OpenCode uses standard OpenAI format
		reqBody := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "system", "content": agent.SystemPrompt},
				{"role": "user", "content": "Help me write a function"},
			},
		}

		jsonData, err := json.Marshal(reqBody)
		require.NoError(t, err)
		assert.Contains(t, string(jsonData), agent.SystemPrompt)
	})
}

// TestAgentSpecificFormats_ClaudeCode tests ClaudeCode agent format
func TestAgentSpecificFormats_ClaudeCode(t *testing.T) {
	agent, found := agents.GetAgent("ClaudeCode")
	require.True(t, found)

	t.Run("ClaudeCode_Configuration", func(t *testing.T) {
		assert.Equal(t, "ClaudeCode", agent.Name)
		assert.Equal(t, "TypeScript", agent.Language)
		assert.Equal(t, "JSON", agent.ConfigFormat)
		assert.Equal(t, "Anthropic", agent.APIPattern)
		assert.Equal(t, "claude", agent.EntryPoint)
	})

	t.Run("ClaudeCode_Protocol_Support", func(t *testing.T) {
		assert.Contains(t, agent.Protocols, "Anthropic")
		assert.Contains(t, agent.Protocols, "MCP")
	})

	t.Run("ClaudeCode_Tool_Support", func(t *testing.T) {
		expectedTools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Task"}
		for _, tool := range expectedTools {
			assert.Contains(t, agent.ToolSupport, tool,
				"ClaudeCode should support %s tool", tool)
		}
	})

	t.Run("ClaudeCode_Features", func(t *testing.T) {
		expectedFeatures := []string{"codebase-understanding", "git-workflow", "plugin-system", "github-integration"}
		for _, feature := range expectedFeatures {
			assert.Contains(t, agent.Features, feature,
				"ClaudeCode should have feature %s", feature)
		}
	})
}

// TestAgentSpecificFormats_Aider tests Aider agent format
func TestAgentSpecificFormats_Aider(t *testing.T) {
	agent, found := agents.GetAgent("Aider")
	require.True(t, found)

	t.Run("Aider_Configuration", func(t *testing.T) {
		assert.Equal(t, "Aider", agent.Name)
		assert.Equal(t, "Python", agent.Language)
		assert.Equal(t, "TOML", agent.ConfigFormat)
		assert.Equal(t, "Multi-provider", agent.APIPattern)
		assert.Contains(t, agent.ConfigLocation, ".aider")
	})

	t.Run("Aider_Multi_Protocol_Support", func(t *testing.T) {
		assert.Contains(t, agent.Protocols, "OpenAI")
		assert.Contains(t, agent.Protocols, "Anthropic")
		assert.Contains(t, agent.Protocols, "DeepSeek")
		assert.Contains(t, agent.Protocols, "Gemini")
	})

	t.Run("Aider_EnvVars", func(t *testing.T) {
		assert.NotNil(t, agent.EnvVars)
		_, hasModel := agent.EnvVars["AIDER_MODEL"]
		assert.True(t, hasModel, "Aider should have AIDER_MODEL env var")
	})
}

// TestAgentSpecificFormats_KiloCode tests KiloCode agent format
func TestAgentSpecificFormats_KiloCode(t *testing.T) {
	agent, found := agents.GetAgent("KiloCode")
	require.True(t, found)

	t.Run("KiloCode_Configuration", func(t *testing.T) {
		assert.Equal(t, "KiloCode", agent.Name)
		assert.Equal(t, "TypeScript", agent.Language)
		assert.Equal(t, "Multi-provider", agent.APIPattern)
	})

	t.Run("KiloCode_Extensive_Protocol_Support", func(t *testing.T) {
		expectedProtocols := []string{"OpenAI", "Anthropic", "OpenRouter", "AWS", "GCP", "Azure", "MCP"}
		for _, protocol := range expectedProtocols {
			assert.Contains(t, agent.Protocols, protocol,
				"KiloCode should support %s protocol", protocol)
		}
	})

	t.Run("KiloCode_Full_Tool_Support", func(t *testing.T) {
		// KiloCode should have the most comprehensive tool support
		expectedTools := []string{
			"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git",
			"Test", "Lint", "Diff", "TreeView", "FileInfo",
			"Symbols", "References", "Definition",
			"PR", "Issue", "Workflow",
		}
		for _, tool := range expectedTools {
			assert.Contains(t, agent.ToolSupport, tool,
				"KiloCode should support %s tool", tool)
		}
	})
}

// TestAgentSpecificFormats_AmazonQ tests AmazonQ agent format
func TestAgentSpecificFormats_AmazonQ(t *testing.T) {
	agent, found := agents.GetAgent("AmazonQ")
	require.True(t, found)

	t.Run("AmazonQ_Configuration", func(t *testing.T) {
		assert.Equal(t, "AmazonQ", agent.Name)
		assert.Equal(t, "Rust", agent.Language)
		assert.Equal(t, "AWS", agent.APIPattern)
		assert.Equal(t, "q", agent.EntryPoint)
	})

	t.Run("AmazonQ_Protocol_Support", func(t *testing.T) {
		assert.Contains(t, agent.Protocols, "AWS")
		assert.Contains(t, agent.Protocols, "MCP")
	})

	t.Run("AmazonQ_EnvVars", func(t *testing.T) {
		assert.NotNil(t, agent.EnvVars)
		_, hasRegion := agent.EnvVars["AWS_REGION"]
		assert.True(t, hasRegion, "AmazonQ should have AWS_REGION env var")
		_, hasProfile := agent.EnvVars["AWS_PROFILE"]
		assert.True(t, hasProfile, "AmazonQ should have AWS_PROFILE env var")
	})
}

// =============================================================================
// TOOL SCHEMA REGISTRY TESTS
// =============================================================================

// TestToolSchemaRegistry_AllToolsRegistered verifies tool schema registry
func TestToolSchemaRegistry_AllToolsRegistered(t *testing.T) {
	t.Run("Core_Tools_Registered", func(t *testing.T) {
		coreTools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "WebFetch", "WebSearch", "Task"}
		for _, toolName := range coreTools {
			schema, found := tools.GetToolSchema(toolName)
			assert.True(t, found, "Core tool %s should be registered", toolName)
			if found {
				assert.NotEmpty(t, schema.RequiredFields, "Tool %s should have required fields", toolName)
			}
		}
	})

	t.Run("Version_Control_Tools_Registered", func(t *testing.T) {
		vcTools := []string{"Git", "Diff"}
		for _, toolName := range vcTools {
			schema, found := tools.GetToolSchema(toolName)
			assert.True(t, found, "Version control tool %s should be registered", toolName)
			if found {
				assert.Equal(t, tools.CategoryVersionControl, schema.Category)
			}
		}
	})

	t.Run("Code_Intelligence_Tools_Registered", func(t *testing.T) {
		ciTools := []string{"Symbols", "References", "Definition"}
		for _, toolName := range ciTools {
			schema, found := tools.GetToolSchema(toolName)
			assert.True(t, found, "Code intelligence tool %s should be registered", toolName)
			if found {
				assert.Equal(t, tools.CategoryCodeIntel, schema.Category)
			}
		}
	})

	t.Run("Workflow_Tools_Registered", func(t *testing.T) {
		wfTools := []string{"PR", "Issue", "Workflow"}
		for _, toolName := range wfTools {
			schema, found := tools.GetToolSchema(toolName)
			assert.True(t, found, "Workflow tool %s should be registered", toolName)
			if found {
				assert.Equal(t, tools.CategoryWorkflow, schema.Category)
			}
		}
	})
}

// TestToolSchemaRegistry_ToolValidation verifies tool argument validation
func TestToolSchemaRegistry_ToolValidation(t *testing.T) {
	t.Run("Valid_Bash_Args", func(t *testing.T) {
		args := map[string]interface{}{
			"command":     "ls -la",
			"description": "List files",
		}
		err := tools.ValidateToolArgs("Bash", args)
		assert.NoError(t, err)
	})

	t.Run("Invalid_Bash_Args_Missing_Command", func(t *testing.T) {
		args := map[string]interface{}{
			"description": "List files",
		}
		err := tools.ValidateToolArgs("Bash", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command")
	})

	t.Run("Invalid_Bash_Args_Empty_Command", func(t *testing.T) {
		args := map[string]interface{}{
			"command":     "",
			"description": "List files",
		}
		err := tools.ValidateToolArgs("Bash", args)
		assert.Error(t, err)
	})

	t.Run("Valid_Read_Args", func(t *testing.T) {
		args := map[string]interface{}{
			"file_path": "/tmp/test.txt",
		}
		err := tools.ValidateToolArgs("Read", args)
		assert.NoError(t, err)
	})

	t.Run("Valid_Edit_Args", func(t *testing.T) {
		args := map[string]interface{}{
			"file_path":  "/tmp/test.txt",
			"old_string": "foo",
			"new_string": "bar",
		}
		err := tools.ValidateToolArgs("Edit", args)
		assert.NoError(t, err)
	})

	t.Run("Invalid_Edit_Args_Missing_Fields", func(t *testing.T) {
		args := map[string]interface{}{
			"file_path": "/tmp/test.txt",
		}
		err := tools.ValidateToolArgs("Edit", args)
		assert.Error(t, err)
	})

	t.Run("Unknown_Tool", func(t *testing.T) {
		args := map[string]interface{}{}
		err := tools.ValidateToolArgs("NonExistentTool", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})
}

// TestToolSchemaRegistry_OpenAIDefinitions verifies OpenAI tool definition generation
func TestToolSchemaRegistry_OpenAIDefinitions(t *testing.T) {
	t.Run("Generate_All_Definitions", func(t *testing.T) {
		definitions := tools.GenerateAllToolDefinitions()
		assert.Greater(t, len(definitions), 0, "Should generate tool definitions")

		// Verify structure
		for _, def := range definitions {
			assert.Equal(t, "function", def["type"])
			fn, ok := def["function"].(map[string]interface{})
			require.True(t, ok)
			assert.NotEmpty(t, fn["name"])
			assert.NotEmpty(t, fn["description"])
		}
	})

	t.Run("Generate_Bash_Definition", func(t *testing.T) {
		schema, found := tools.GetToolSchema("Bash")
		require.True(t, found)

		def := tools.GenerateOpenAIToolDefinition(schema)
		assert.Equal(t, "function", def["type"])

		fn := def["function"].(map[string]interface{})
		assert.Equal(t, "Bash", fn["name"])
		assert.Contains(t, fn["description"].(string), "shell")

		params := fn["parameters"].(map[string]interface{})
		props := params["properties"].(map[string]interface{})
		assert.Contains(t, props, "command")
		assert.Contains(t, props, "description")
	})
}

// =============================================================================
// HTTP HANDLER INTEGRATION TESTS
// =============================================================================

// TestHTTPHandlerIntegration_AgentsEndpoint tests the agents endpoint
func TestHTTPHandlerIntegration_AgentsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("List_All_Agents", func(t *testing.T) {
		router := gin.New()
		router.GET("/v1/agents", func(c *gin.Context) {
			allAgents := agents.GetAllAgents()
			response := map[string]interface{}{
				"agents": allAgents,
				"count":  len(allAgents),
			}
			c.JSON(http.StatusOK, response)
		})

		req, _ := http.NewRequest("GET", "/v1/agents", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		count := int(response["count"].(float64))
		assert.Equal(t, 48, count)
	})

	t.Run("Get_Specific_Agent", func(t *testing.T) {
		router := gin.New()
		router.GET("/v1/agents/:name", func(c *gin.Context) {
			name := c.Param("name")
			agent, found := agents.GetAgent(name)
			if !found {
				c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
				return
			}
			c.JSON(http.StatusOK, agent)
		})

		req, _ := http.NewRequest("GET", "/v1/agents/OpenCode", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response agents.CLIAgent
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "OpenCode", response.Name)
	})

	t.Run("Get_Nonexistent_Agent", func(t *testing.T) {
		router := gin.New()
		router.GET("/v1/agents/:name", func(c *gin.Context) {
			name := c.Param("name")
			agent, found := agents.GetAgent(name)
			if !found {
				c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
				return
			}
			c.JSON(http.StatusOK, agent)
		})

		req, _ := http.NewRequest("GET", "/v1/agents/NonExistent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Get_Agents_By_Protocol", func(t *testing.T) {
		router := gin.New()
		router.GET("/v1/agents/protocol/:protocol", func(c *gin.Context) {
			protocol := c.Param("protocol")
			filteredAgents := agents.GetAgentsByProtocol(protocol)
			c.JSON(http.StatusOK, gin.H{
				"agents": filteredAgents,
				"count":  len(filteredAgents),
			})
		})

		req, _ := http.NewRequest("GET", "/v1/agents/protocol/MCP", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		count := int(response["count"].(float64))
		assert.GreaterOrEqual(t, count, 6, "At least 6 agents should support MCP")
	})

	t.Run("Get_Agents_By_Tool", func(t *testing.T) {
		router := gin.New()
		router.GET("/v1/agents/tool/:tool", func(c *gin.Context) {
			tool := c.Param("tool")
			filteredAgents := agents.GetAgentsByTool(tool)
			c.JSON(http.StatusOK, gin.H{
				"agents": filteredAgents,
				"count":  len(filteredAgents),
			})
		})

		req, _ := http.NewRequest("GET", "/v1/agents/tool/Bash", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		count := int(response["count"].(float64))
		assert.GreaterOrEqual(t, count, 44, "Most agents should support Bash")
	})
}

// =============================================================================
// LIVE SERVER INTEGRATION TESTS (Requires Running HelixAgent)
// =============================================================================

// TestLiveServer_AgentCompatibility tests agent compatibility with live server
func TestLiveServer_AgentCompatibility(t *testing.T) {
	if !cliAgentServerAvailable(t) {
		return
	}

	baseURL := cliAgentTestBaseURL()
	client := &http.Client{Timeout: 60 * time.Second}

	for _, agentName := range expectedAgents {
		t.Run(agentName+"_Request", func(t *testing.T) {
			agent, found := agents.GetAgent(agentName)
			require.True(t, found)

			reqBody := map[string]interface{}{
				"model": "helixagent-debate",
				"messages": []map[string]string{
					{"role": "system", "content": agent.SystemPrompt},
					{"role": "user", "content": "Say 'OK'"},
				},
				"max_tokens": 20,
			}

			jsonBody, err := json.Marshal(reqBody)
			require.NoError(t, err)

			resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if resp.StatusCode == 200 {
				var apiResp map[string]interface{}
				err = json.Unmarshal(body, &apiResp)
				if err == nil {
					choices, ok := apiResp["choices"].([]interface{})
					if ok && len(choices) > 0 {
						t.Logf("%s: Response received successfully", agentName)
					}
				}
			} else if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
				t.Logf("%s: Provider temporarily unavailable (status %d)", agentName, resp.StatusCode)
			}
		})
	}
}

// TestLiveServer_ProtocolEndpoints tests protocol-specific endpoints
func TestLiveServer_ProtocolEndpoints(t *testing.T) {
	if !cliAgentServerAvailable(t) {
		return
	}

	baseURL := cliAgentTestBaseURL()
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("MCP_Endpoint", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/v1/mcp")
		if err != nil {
			t.Logf("MCP endpoint not accessible: %v", err)
			return
		}
		defer resp.Body.Close()

		// 404 is acceptable if not implemented, 200/405 means endpoint exists
		t.Logf("MCP endpoint status: %d", resp.StatusCode)
	})

	t.Run("LSP_Endpoint", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/v1/lsp")
		if err != nil {
			t.Logf("LSP endpoint not accessible: %v", err)
			return
		}
		defer resp.Body.Close()

		t.Logf("LSP endpoint status: %d", resp.StatusCode)
	})

	t.Run("ACP_Endpoint", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/v1/acp")
		if err != nil {
			t.Logf("ACP endpoint not accessible: %v", err)
			return
		}
		defer resp.Body.Close()

		t.Logf("ACP endpoint status: %d", resp.StatusCode)
	})
}

// TestLiveServer_ToolCallFlow tests complete tool call flow
func TestLiveServer_ToolCallFlow(t *testing.T) {
	if !cliAgentServerAvailable(t) {
		return
	}

	baseURL := cliAgentTestBaseURL()
	client := &http.Client{Timeout: 90 * time.Second}

	t.Run("Tool_Enabled_Request", func(t *testing.T) {
		toolDefs := []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "Bash",
					"description": "Execute shell commands",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"command":     map[string]string{"type": "string", "description": "Command to execute"},
							"description": map[string]string{"type": "string", "description": "Description"},
						},
						"required": []string{"command", "description"},
					},
				},
			},
		}

		reqBody := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "system", "content": "You are a coding assistant with tool access."},
				{"role": "user", "content": "What is 2+2? Just answer the number."},
			},
			"tools":      toolDefs,
			"max_tokens": 50,
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		if resp.StatusCode == 200 {
			t.Logf("Tool-enabled request succeeded")
		} else if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			t.Logf("Provider temporarily unavailable (status %d)", resp.StatusCode)
		} else {
			t.Logf("Response status: %d, body: %s", resp.StatusCode, cliAgentTruncateString(string(body), 200))
		}
	})
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// cliAgentServerAvailable checks if the HelixAgent server is available for CLI agent tests
func cliAgentServerAvailable(t *testing.T) bool {
	t.Helper()

	if os.Getenv("HELIXAGENT_INTEGRATION_TESTS") != "1" {
		t.Logf("HELIXAGENT_INTEGRATION_TESTS not set - skipping live server test")
		return false
	}

	baseURL := cliAgentTestBaseURL()
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(baseURL + "/health")
	if err != nil || resp.StatusCode != 200 {
		t.Logf("HelixAgent not running at %s", baseURL)
		return false
	}
	resp.Body.Close()
	return true
}

// cliAgentTestBaseURL returns the base URL for CLI agent testing
func cliAgentTestBaseURL() string {
	if url := os.Getenv("HELIXAGENT_URL"); url != "" {
		return url
	}
	return "http://localhost:7061"
}

// cliAgentTruncateString truncates a string to the specified length
func cliAgentTruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

// BenchmarkAgentLookup benchmarks agent lookup performance
func BenchmarkAgentLookup(b *testing.B) {
	b.Run("Direct_Lookup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			agents.GetAgent("OpenCode")
		}
	})

	b.Run("Case_Insensitive_Lookup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			agents.GetAgent("opencode")
		}
	})

	b.Run("GetAllAgents", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			agents.GetAllAgents()
		}
	})

	b.Run("GetAgentsByProtocol", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			agents.GetAgentsByProtocol("MCP")
		}
	})

	b.Run("GetAgentsByTool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			agents.GetAgentsByTool("Bash")
		}
	})
}

// BenchmarkToolValidation benchmarks tool validation performance
func BenchmarkToolValidation(b *testing.B) {
	args := map[string]interface{}{
		"command":     "ls -la",
		"description": "List files",
	}

	b.Run("Bash_Validation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tools.ValidateToolArgs("Bash", args)
		}
	})

	b.Run("Schema_Lookup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tools.GetToolSchema("Bash")
		}
	})
}
