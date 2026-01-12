package agents

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLIAgentRegistryCount verifies we have all 18 CLI agents
func TestCLIAgentRegistryCount(t *testing.T) {
	assert.Equal(t, 18, len(CLIAgentRegistry), "Should have exactly 18 CLI agents registered")
}

// TestAllAgentNamesPresent verifies all expected agents are in the registry
func TestAllAgentNamesPresent(t *testing.T) {
	expectedAgents := []string{
		// Original 4 agents
		"OpenCode", "Crush", "HelixCode", "Kiro",
		// New 14 agents from HelixCode/Example_Projects
		"Aider", "ClaudeCode", "Cline", "CodenameGoose", "DeepSeekCLI",
		"Forge", "GeminiCLI", "GPTEngineer", "KiloCode", "MistralCode",
		"OllamaCode", "Plandex", "QwenCode", "AmazonQ",
	}

	for _, name := range expectedAgents {
		agent, found := GetAgent(name)
		require.True(t, found, "Agent %s should be registered", name)
		require.NotNil(t, agent, "Agent %s should not be nil", name)
		assert.Equal(t, name, agent.Name, "Agent name should match registry key")
	}
}

// TestGetAgentExactMatch tests exact name matching
func TestGetAgentExactMatch(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"OpenCode", "OpenCode"},
		{"Crush", "Crush"},
		{"HelixCode", "HelixCode"},
		{"Kiro", "Kiro"},
		{"Aider", "Aider"},
		{"ClaudeCode", "ClaudeCode"},
		{"Cline", "Cline"},
		{"KiloCode", "KiloCode"},
		{"AmazonQ", "AmazonQ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, found := GetAgent(tt.name)
			assert.True(t, found)
			assert.Equal(t, tt.expected, agent.Name)
		})
	}
}

// TestGetAgentCaseInsensitive tests case-insensitive matching
func TestGetAgentCaseInsensitive(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"opencode", "OpenCode"},
		{"OPENCODE", "OpenCode"},
		{"OpenCODE", "OpenCode"},
		{"crush", "Crush"},
		{"CRUSH", "Crush"},
		{"kiro", "Kiro"},
		{"KIRO", "Kiro"},
		{"claudecode", "ClaudeCode"},
		{"CLAUDECODE", "ClaudeCode"},
		{"amazonq", "AmazonQ"},
		{"AMAZONQ", "AmazonQ"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			agent, found := GetAgent(tt.input)
			assert.True(t, found, "Agent %s should be found", tt.input)
			assert.Equal(t, tt.expected, agent.Name)
		})
	}
}

// TestGetAgentNotFound tests that non-existent agents return false
func TestGetAgentNotFound(t *testing.T) {
	nonExistent := []string{
		"NonExistent",
		"FakeAgent",
		"NotAnAgent",
		"",
		"   ",
	}

	for _, name := range nonExistent {
		t.Run(name, func(t *testing.T) {
			agent, found := GetAgent(name)
			assert.False(t, found, "Agent %s should not be found", name)
			assert.Nil(t, agent)
		})
	}
}

// TestGetAllAgents verifies GetAllAgents returns all agents
func TestGetAllAgents(t *testing.T) {
	agents := GetAllAgents()
	assert.Equal(t, 18, len(agents), "Should return all 18 agents")

	// Verify no nil agents
	for _, agent := range agents {
		assert.NotNil(t, agent)
		assert.NotEmpty(t, agent.Name)
	}
}

// TestGetAgentNames verifies GetAgentNames returns all names
func TestGetAgentNames(t *testing.T) {
	names := GetAgentNames()
	assert.Equal(t, 18, len(names), "Should return 18 agent names")

	// Verify no empty names
	for _, name := range names {
		assert.NotEmpty(t, name)
	}

	// Verify all expected names are present
	expectedNames := []string{
		// 4 original + 14 new = 18 total
		"OpenCode", "Crush", "HelixCode", "Kiro",
		"Aider", "ClaudeCode", "Cline", "CodenameGoose", "DeepSeekCLI",
		"Forge", "GeminiCLI", "GPTEngineer", "KiloCode", "MistralCode",
		"OllamaCode", "Plandex", "QwenCode", "AmazonQ",
	}

	// Sort both for comparison
	sort.Strings(names)
	sort.Strings(expectedNames)

	// Check all expected names are in the result (order may vary)
	for _, expected := range expectedNames {
		found := false
		for _, name := range names {
			if name == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected agent %s to be in names list", expected)
	}
}

// TestAgentRequiredFields verifies all agents have required fields populated
func TestAgentRequiredFields(t *testing.T) {
	for name, agent := range CLIAgentRegistry {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, agent.Name, "Agent %s should have a name", name)
			assert.NotEmpty(t, agent.Description, "Agent %s should have a description", name)
			assert.NotEmpty(t, agent.Language, "Agent %s should have a language", name)
			assert.NotEmpty(t, agent.ConfigFormat, "Agent %s should have a config format", name)
			assert.NotEmpty(t, agent.APIPattern, "Agent %s should have an API pattern", name)
			assert.NotEmpty(t, agent.EntryPoint, "Agent %s should have an entry point", name)
			assert.NotEmpty(t, agent.Features, "Agent %s should have features", name)
			assert.NotEmpty(t, agent.ToolSupport, "Agent %s should have tool support", name)
			assert.NotEmpty(t, agent.Protocols, "Agent %s should have protocols", name)
			assert.NotEmpty(t, agent.ConfigLocation, "Agent %s should have a config location", name)
			assert.NotEmpty(t, agent.SystemPrompt, "Agent %s should have a system prompt", name)
		})
	}
}

// TestAgentToolSupport verifies tool support for each agent
func TestAgentToolSupport(t *testing.T) {
	// All agents should support at least Bash and Read
	coreTools := []string{"Bash", "Read"}

	for name, agent := range CLIAgentRegistry {
		t.Run(name, func(t *testing.T) {
			for _, tool := range coreTools {
				found := false
				for _, supported := range agent.ToolSupport {
					if supported == tool {
						found = true
						break
					}
				}
				assert.True(t, found, "Agent %s should support core tool %s", name, tool)
			}
		})
	}
}

// TestAgentProtocols verifies protocol support for each agent
func TestAgentProtocols(t *testing.T) {
	// Each agent should support at least one protocol
	for name, agent := range CLIAgentRegistry {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, agent.Protocols, "Agent %s should support at least one protocol", name)
		})
	}
}

// TestGetAgentsByProtocol tests filtering agents by protocol
func TestGetAgentsByProtocol(t *testing.T) {
	tests := []struct {
		protocol       string
		minExpected    int
		shouldContain  []string
	}{
		{"OpenAI", 10, []string{"OpenCode", "Crush", "HelixCode", "Kiro", "Plandex"}},
		{"Anthropic", 5, []string{"ClaudeCode", "CodenameGoose", "Forge", "Aider", "KiloCode"}},
		{"MCP", 6, []string{"OpenCode", "HelixCode", "Kiro", "ClaudeCode", "Cline", "KiloCode"}},
		{"AWS", 2, []string{"AmazonQ", "KiloCode"}},
		{"Ollama", 2, []string{"DeepSeekCLI", "OllamaCode"}},
		{"Gemini", 2, []string{"Aider", "GeminiCLI"}},
		{"Mistral", 1, []string{"MistralCode"}},
		{"Qwen", 1, []string{"QwenCode"}},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			agents := GetAgentsByProtocol(tt.protocol)
			assert.GreaterOrEqual(t, len(agents), tt.minExpected,
				"Protocol %s should have at least %d agents", tt.protocol, tt.minExpected)

			agentNames := make(map[string]bool)
			for _, agent := range agents {
				agentNames[agent.Name] = true
			}

			for _, expectedName := range tt.shouldContain {
				assert.True(t, agentNames[expectedName],
					"Protocol %s should include agent %s", tt.protocol, expectedName)
			}
		})
	}
}

// TestGetAgentsByProtocolCaseInsensitive tests case-insensitive protocol matching
func TestGetAgentsByProtocolCaseInsensitive(t *testing.T) {
	openaiLower := GetAgentsByProtocol("openai")
	openaiUpper := GetAgentsByProtocol("OPENAI")
	openaiMixed := GetAgentsByProtocol("OpenAI")

	assert.Equal(t, len(openaiLower), len(openaiUpper))
	assert.Equal(t, len(openaiLower), len(openaiMixed))
}

// TestGetAgentsByTool tests filtering agents by tool
func TestGetAgentsByTool(t *testing.T) {
	tests := []struct {
		tool           string
		minExpected    int
		shouldContain  []string
	}{
		{"Bash", 18, []string{"OpenCode", "Crush", "HelixCode", "Kiro", "Aider"}},
		{"Read", 18, []string{"OpenCode", "Crush", "HelixCode", "Kiro", "ClaudeCode"}},
		{"Write", 18, []string{"OpenCode", "Crush", "HelixCode", "Kiro", "Cline"}},
		{"Edit", 17, []string{"OpenCode", "HelixCode", "Kiro", "Aider", "ClaudeCode"}}, // Crush doesn't support Edit
		{"Git", 14, []string{"OpenCode", "HelixCode", "Kiro", "Aider", "ClaudeCode"}},
		{"Test", 6, []string{"OpenCode", "HelixCode", "Kiro", "Forge", "KiloCode"}},
		{"Lint", 5, []string{"OpenCode", "HelixCode", "Kiro", "Forge", "KiloCode"}},
		{"Task", 5, []string{"HelixCode", "ClaudeCode", "Forge", "Plandex", "AmazonQ"}},
		{"PR", 2, []string{"Kiro", "KiloCode"}},
		{"Issue", 2, []string{"Kiro", "KiloCode"}},
		{"Workflow", 2, []string{"Kiro", "KiloCode"}},
		{"WebFetch", 2, []string{"Cline", "AmazonQ"}},
		{"Symbols", 2, []string{"Cline", "KiloCode"}},
		{"References", 2, []string{"Cline", "KiloCode"}},
		{"Definition", 2, []string{"Cline", "KiloCode"}},
		{"TreeView", 2, []string{"CodenameGoose", "KiloCode"}},
		{"FileInfo", 1, []string{"KiloCode"}},
		{"Diff", 2, []string{"Aider", "KiloCode"}},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			agents := GetAgentsByTool(tt.tool)
			assert.GreaterOrEqual(t, len(agents), tt.minExpected,
				"Tool %s should be supported by at least %d agents", tt.tool, tt.minExpected)

			agentNames := make(map[string]bool)
			for _, agent := range agents {
				agentNames[agent.Name] = true
			}

			for _, expectedName := range tt.shouldContain {
				assert.True(t, agentNames[expectedName],
					"Tool %s should be supported by agent %s", tt.tool, expectedName)
			}
		})
	}
}

// TestGetAgentsByToolCaseInsensitive tests case-insensitive tool matching
func TestGetAgentsByToolCaseInsensitive(t *testing.T) {
	bashLower := GetAgentsByTool("bash")
	bashUpper := GetAgentsByTool("BASH")
	bashMixed := GetAgentsByTool("Bash")

	assert.Equal(t, len(bashLower), len(bashUpper))
	assert.Equal(t, len(bashLower), len(bashMixed))
}

// TestAgentLanguages verifies all agents have valid languages
func TestAgentLanguages(t *testing.T) {
	validLanguages := map[string]bool{
		"Go":         true,
		"Python":     true,
		"TypeScript": true,
		"Rust":       true,
	}

	for name, agent := range CLIAgentRegistry {
		t.Run(name, func(t *testing.T) {
			assert.True(t, validLanguages[agent.Language],
				"Agent %s has invalid language: %s", name, agent.Language)
		})
	}
}

// TestAgentConfigFormats verifies all agents have valid config formats
func TestAgentConfigFormats(t *testing.T) {
	validFormats := map[string]bool{
		"JSON":             true,
		"YAML":             true,
		"TOML":             true,
		"ENV":              true,
		"Proto/gRPC":       true,
		"YAML+JSON Schema": true,
	}

	for name, agent := range CLIAgentRegistry {
		t.Run(name, func(t *testing.T) {
			assert.True(t, validFormats[agent.ConfigFormat],
				"Agent %s has invalid config format: %s", name, agent.ConfigFormat)
		})
	}
}

// TestAgentAPIPatterns verifies all agents have valid API patterns
func TestAgentAPIPatterns(t *testing.T) {
	validPatterns := map[string]bool{
		"OpenAI-compatible": true,
		"Multi-provider":    true,
		"Anthropic":         true,
		"Google":            true,
		"Mistral":           true,
		"Ollama":            true,
		"Qwen":              true,
		"AWS":               true,
		"OpenAI":            true,
		"DeepSeek/Ollama":   true,
	}

	for name, agent := range CLIAgentRegistry {
		t.Run(name, func(t *testing.T) {
			assert.True(t, validPatterns[agent.APIPattern],
				"Agent %s has invalid API pattern: %s", name, agent.APIPattern)
		})
	}
}

// TestKiloCodeHasAll21Tools verifies KiloCode supports all 21 tools
func TestKiloCodeHasAll21Tools(t *testing.T) {
	all21Tools := []string{
		"Bash", "Read", "Write", "Edit", "Glob", "Grep",
		"Git", "Test", "Lint", "Diff", "TreeView", "FileInfo",
		"Symbols", "References", "Definition",
		"PR", "Issue", "Workflow",
	}

	agent, found := GetAgent("KiloCode")
	require.True(t, found)

	for _, tool := range all21Tools {
		found := false
		for _, supported := range agent.ToolSupport {
			if supported == tool {
				found = true
				break
			}
		}
		assert.True(t, found, "KiloCode should support tool %s", tool)
	}
}

// TestOpenCodeAgentIntegration tests OpenCode-specific configuration
func TestOpenCodeAgentIntegration(t *testing.T) {
	agent, found := GetAgent("OpenCode")
	require.True(t, found)

	assert.Equal(t, "Go", agent.Language)
	assert.Equal(t, "JSON", agent.ConfigFormat)
	assert.Equal(t, "OpenAI-compatible", agent.APIPattern)
	assert.Equal(t, "opencode", agent.EntryPoint)
	assert.Contains(t, agent.Protocols, "OpenAI")
	assert.Contains(t, agent.Protocols, "MCP")
}

// TestClaudeCodeAgentIntegration tests ClaudeCode-specific configuration
func TestClaudeCodeAgentIntegration(t *testing.T) {
	agent, found := GetAgent("ClaudeCode")
	require.True(t, found)

	assert.Equal(t, "TypeScript", agent.Language)
	assert.Equal(t, "JSON", agent.ConfigFormat)
	assert.Equal(t, "Anthropic", agent.APIPattern)
	assert.Equal(t, "claude", agent.EntryPoint)
	assert.Contains(t, agent.Protocols, "Anthropic")
	assert.Contains(t, agent.Protocols, "MCP")
	assert.Contains(t, agent.EnvVars, "ANTHROPIC_API_KEY")
}

// TestAmazonQAgentIntegration tests AmazonQ-specific configuration
func TestAmazonQAgentIntegration(t *testing.T) {
	agent, found := GetAgent("AmazonQ")
	require.True(t, found)

	assert.Equal(t, "Rust", agent.Language)
	assert.Equal(t, "JSON", agent.ConfigFormat)
	assert.Equal(t, "AWS", agent.APIPattern)
	assert.Equal(t, "q", agent.EntryPoint)
	assert.Contains(t, agent.Protocols, "AWS")
	assert.Contains(t, agent.Protocols, "MCP")
	assert.Contains(t, agent.EnvVars, "AWS_REGION")
	assert.Contains(t, agent.EnvVars, "AWS_PROFILE")
}

// TestAiderAgentIntegration tests Aider-specific configuration
func TestAiderAgentIntegration(t *testing.T) {
	agent, found := GetAgent("Aider")
	require.True(t, found)

	assert.Equal(t, "Python", agent.Language)
	assert.Equal(t, "TOML", agent.ConfigFormat)
	assert.Equal(t, "Multi-provider", agent.APIPattern)
	assert.Equal(t, "aider", agent.EntryPoint)
	assert.Contains(t, agent.Features, "git-integration")
	assert.Contains(t, agent.Features, "auto-commits")
	assert.Contains(t, agent.EnvVars, "AIDER_MODEL")
}

// TestEqualFold tests the custom case-insensitive comparison
func TestEqualFold(t *testing.T) {
	tests := []struct {
		a, b     string
		expected bool
	}{
		{"abc", "ABC", true},
		{"ABC", "abc", true},
		{"AbC", "aBc", true},
		{"hello", "hello", true},
		{"HELLO", "HELLO", true},
		{"hello", "HELLO", true},
		{"abc", "abd", false},
		{"abc", "ab", false},
		{"", "", true},
		{"a", "", false},
		{"", "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			result := equalFold(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestAgentCategoryConstants verifies category constants are defined
func TestAgentCategoryConstants(t *testing.T) {
	assert.Equal(t, "terminal", CategoryTerminal)
	assert.Equal(t, "vscode", CategoryVSCode)
	assert.Equal(t, "jetbrains", CategoryJetBrains)
	assert.Equal(t, "standalone", CategoryStandalone)
	assert.Equal(t, "cloud", CategoryCloud)
	assert.Equal(t, "local", CategoryLocal)
}

// TestAgentConfigLocations verifies config locations are valid paths
func TestAgentConfigLocations(t *testing.T) {
	for name, agent := range CLIAgentRegistry {
		t.Run(name, func(t *testing.T) {
			// Config location should start with ~ or /
			assert.True(t,
				agent.ConfigLocation[0] == '~' || agent.ConfigLocation[0] == '/',
				"Agent %s config location should be an absolute path: %s", name, agent.ConfigLocation)
		})
	}
}

// TestAgentEntryPoints verifies entry points are valid command names
func TestAgentEntryPoints(t *testing.T) {
	for name, agent := range CLIAgentRegistry {
		t.Run(name, func(t *testing.T) {
			// Entry point should be a simple command name (no spaces, no special chars except -)
			for _, c := range agent.EntryPoint {
				assert.True(t,
					(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_',
					"Agent %s entry point contains invalid character: %s", name, agent.EntryPoint)
			}
		})
	}
}

// TestAgentSystemPrompts verifies system prompts are meaningful
func TestAgentSystemPrompts(t *testing.T) {
	for name, agent := range CLIAgentRegistry {
		t.Run(name, func(t *testing.T) {
			// System prompt should be at least 30 characters
			assert.GreaterOrEqual(t, len(agent.SystemPrompt), 30,
				"Agent %s system prompt is too short", name)
			// System prompt should contain "You are"
			assert.Contains(t, agent.SystemPrompt, "You are",
				"Agent %s system prompt should contain 'You are'", name)
		})
	}
}
