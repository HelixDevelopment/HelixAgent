package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// ConfigGenerator Tests
// ========================================

func TestNewConfigGenerator(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")

	assert.Equal(t, "http://localhost:7061/v1", gen.baseURL)
	assert.Equal(t, "test-key", gen.apiKey)
	assert.Equal(t, "helixagent-debate", gen.model)
	assert.Equal(t, 120, gen.timeout)
	assert.Equal(t, 8192, gen.maxTokens)
}

func TestConfigGenerator_SetTimeout(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "model")
	gen.SetTimeout(300)
	assert.Equal(t, 300, gen.timeout)
}

func TestConfigGenerator_SetMaxTokens(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "model")
	gen.SetMaxTokens(4096)
	assert.Equal(t, 4096, gen.maxTokens)
}

func TestConfigGenerator_Validate_Success(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	err := gen.validate()
	assert.NoError(t, err)
}

func TestConfigGenerator_Validate_EmptyBaseURL(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("", "test-key", "model")
	err := gen.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "baseURL is required")
}

func TestConfigGenerator_Validate_EmptyModel(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "")
	err := gen.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model is required")
}

func TestConfigGenerator_Validate_InvalidURL(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("://invalid", "test-key", "model")
	err := gen.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid baseURL")
}

// ========================================
// OpenCode Config Generation Tests
// ========================================

func TestConfigGenerator_GenerateOpenCodeConfig(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	gen.SetTimeout(180)

	config, err := gen.GenerateOpenCodeConfig()
	require.NoError(t, err)

	// New schema uses "providers" (plural) with "local" provider
	assert.Contains(t, config.Providers, "local")
	provider := config.Providers["local"]
	assert.Equal(t, "test-key", provider.APIKey)

	// Check agents section (new schema uses "agents" plural)
	assert.Contains(t, config.Agents, "coder")
	coderAgent := config.Agents["coder"]
	assert.Equal(t, "local.helixagent-debate", coderAgent.Model)
}

func TestConfigGenerator_GenerateOpenCodeConfig_JSON(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")

	jsonData, err := gen.GenerateJSON(AgentTypeOpenCode)
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// Verify structure - new schema uses "providers" (plural), "agents", "mcpServers"
	assert.Contains(t, parsed, "providers")
	assert.Contains(t, parsed, "agents")

	providers := parsed["providers"].(map[string]interface{})
	assert.Contains(t, providers, "local")
}

// ========================================
// Crush Config Generation Tests
// ========================================

func TestConfigGenerator_GenerateCrushConfig(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	gen.SetMaxTokens(4096)

	config, err := gen.GenerateCrushConfig()
	require.NoError(t, err)

	assert.Equal(t, "https://charm.land/crush.json", config.Schema)
	assert.Contains(t, config.Providers, "helixagent")

	provider := config.Providers["helixagent"]
	assert.Equal(t, "openai-compat", provider.Type)
	assert.Equal(t, "http://localhost:7061/v1", provider.BaseURL)
	assert.Equal(t, "test-key", provider.APIKey)
	assert.Len(t, provider.Models, 1)
	assert.Equal(t, "helixagent-debate", provider.Models[0].ID)
	assert.Equal(t, 4096, provider.Models[0].DefaultMaxTokens)
}

func TestConfigGenerator_GenerateCrushConfig_JSON(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")

	jsonData, err := gen.GenerateJSON(AgentTypeCrush)
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// Verify structure
	assert.Contains(t, parsed, "$schema")
	assert.Contains(t, parsed, "providers")

	providers := parsed["providers"].(map[string]interface{})
	assert.Contains(t, providers, "helixagent")

	helixagent := providers["helixagent"].(map[string]interface{})
	assert.Equal(t, "openai-compat", helixagent["type"])
}

// ========================================
// HelixCode Config Generation Tests
// ========================================

func TestConfigGenerator_GenerateHelixCodeConfig(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	gen.SetTimeout(300).SetMaxTokens(16384)

	config, err := gen.GenerateHelixCodeConfig()
	require.NoError(t, err)

	assert.Contains(t, config.Providers, "helixagent")

	provider := config.Providers["helixagent"]
	assert.Equal(t, "openai-compatible", provider.Type)
	assert.Equal(t, "http://localhost:7061/v1", provider.BaseURL)
	assert.Equal(t, "test-key", provider.APIKey)
	assert.Equal(t, "helixagent-debate", provider.Model)
	assert.Equal(t, 16384, provider.MaxTokens)
	assert.Equal(t, 300, provider.Timeout)

	assert.Equal(t, "helixagent", config.Settings.DefaultProvider)
	assert.True(t, config.Settings.StreamingEnabled)
}

func TestConfigGenerator_GenerateHelixCodeConfig_JSON(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")

	jsonData, err := gen.GenerateJSON(AgentTypeHelixCode)
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// Verify structure
	assert.Contains(t, parsed, "providers")
	assert.Contains(t, parsed, "settings")
}

// ========================================
// GenerateConfig Generic Tests
// ========================================

func TestConfigGenerator_GenerateConfig_AllTypes(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")

	tests := []struct {
		agentType AgentType
		valid     bool
	}{
		{AgentTypeOpenCode, true},
		{AgentTypeCrush, true},
		{AgentTypeHelixCode, true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.agentType), func(t *testing.T) {
			t.Parallel()
			config, err := gen.GenerateConfig(tt.agentType)
			if tt.valid {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			} else {
				assert.Error(t, err)
				assert.Nil(t, config)
			}
		})
	}
}

// ========================================
// ConfigValidator Tests
// ========================================

func TestNewConfigValidator(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()
	assert.NotNil(t, v)
}

// ========================================
// OpenCode Validation Tests
// ========================================

func TestConfigValidator_ValidateOpenCodeConfig_Valid(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Providers: map[string]OpenCodeProvider{
			"local": {
				APIKey: "test-key",
			},
		},
		Agents: map[string]OpenCodeAgent{
			"coder": {
				Model:     "local.helixagent-debate",
				MaxTokens: 8192,
			},
		},
	}

	result := v.ValidateOpenCodeConfig(config)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestConfigValidator_ValidateOpenCodeConfig_MissingProvider(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Providers: map[string]OpenCodeProvider{},
	}

	result := v.ValidateOpenCodeConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "providers section is required")
}

func TestConfigValidator_ValidateOpenCodeConfig_MissingAgentModel(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Providers: map[string]OpenCodeProvider{
			"local": {
				APIKey: "test-key",
			},
		},
		Agents: map[string]OpenCodeAgent{
			"coder": {
				Model: "", // Empty model should fail validation
			},
		},
	}

	result := v.ValidateOpenCodeConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "model is required")
}

func TestConfigValidator_ValidateOpenCodeConfig_InvalidMCPType(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Providers: map[string]OpenCodeProvider{
			"local": {
				APIKey: "test-key",
			},
		},
		MCPServers: map[string]OpenCodeMCPServer{
			"server1": {
				Type: "invalid", // Should be "stdio" or "sse"
			},
		},
	}

	result := v.ValidateOpenCodeConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "type must be 'stdio' or 'sse'")
}

// ========================================
// Crush Validation Tests
// ========================================

func TestConfigValidator_ValidateCrushConfig_Valid(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &CrushConfig{
		Schema: "https://charm.land/crush.json",
		Providers: map[string]CrushProvider{
			"helixagent": {
				Type:    "openai-compat",
				BaseURL: "http://localhost:7061/v1",
				APIKey:  "test-key",
				Models: []CrushModel{
					{ID: "helixagent-debate"},
				},
			},
		},
	}

	result := v.ValidateCrushConfig(config)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestConfigValidator_ValidateCrushConfig_InvalidType(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &CrushConfig{
		Providers: map[string]CrushProvider{
			"test": {
				Type: "invalid-type",
			},
		},
	}

	result := v.ValidateCrushConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "invalid type")
}

func TestConfigValidator_ValidateCrushConfig_OpenAICompatMissingBaseURL(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &CrushConfig{
		Providers: map[string]CrushProvider{
			"test": {
				Type: "openai-compat",
				// Missing BaseURL
			},
		},
	}

	result := v.ValidateCrushConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "base_url is required for openai-compat")
}

func TestConfigValidator_ValidateCrushConfig_MCPValidation(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	tests := []struct {
		name        string
		mcpType     string
		command     string
		url         string
		expectValid bool
	}{
		{"stdio_valid", "stdio", "npx", "", true},
		{"stdio_missing_command", "stdio", "", "", false},
		{"http_valid", "http", "", "http://localhost:7061", true},
		{"http_missing_url", "http", "", "", false},
		{"sse_valid", "sse", "", "http://localhost:7061/sse", true},
		{"sse_missing_url", "sse", "", "", false},
		{"invalid_type", "invalid", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := &CrushConfig{
				Providers: map[string]CrushProvider{
					"test": {Type: "openai", BaseURL: "http://test"},
				},
				MCP: map[string]CrushMCP{
					"server": {
						Type:    tt.mcpType,
						Command: tt.command,
						URL:     tt.url,
					},
				},
			}

			result := v.ValidateCrushConfig(config)
			if tt.expectValid {
				assert.True(t, result.Valid, "Expected valid config for %s", tt.name)
			} else {
				assert.False(t, result.Valid, "Expected invalid config for %s", tt.name)
			}
		})
	}
}

// ========================================
// HelixCode Validation Tests
// ========================================

func TestConfigValidator_ValidateHelixCodeConfig_Valid(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &HelixCodeConfig{
		Providers: map[string]HelixCodeProvider{
			"helixagent": {
				Type:    "openai-compatible",
				BaseURL: "http://localhost:7061/v1",
				Model:   "helixagent-debate",
				Timeout: 120,
			},
		},
		Settings: HelixCodeSettings{
			DefaultProvider: "helixagent",
		},
	}

	result := v.ValidateHelixCodeConfig(config)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestConfigValidator_ValidateHelixCodeConfig_MissingBaseURL(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &HelixCodeConfig{
		Providers: map[string]HelixCodeProvider{
			"test": {
				Model: "model",
				// Missing BaseURL
			},
		},
	}

	result := v.ValidateHelixCodeConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "base_url is required")
}

func TestConfigValidator_ValidateHelixCodeConfig_MissingModel(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &HelixCodeConfig{
		Providers: map[string]HelixCodeProvider{
			"test": {
				BaseURL: "http://localhost:7061",
				// Missing Model
			},
		},
	}

	result := v.ValidateHelixCodeConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "model is required")
}

func TestConfigValidator_ValidateHelixCodeConfig_InvalidDefaultProvider(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &HelixCodeConfig{
		Providers: map[string]HelixCodeProvider{
			"test": {
				BaseURL: "http://localhost:7061",
				Model:   "model",
			},
		},
		Settings: HelixCodeSettings{
			DefaultProvider: "nonexistent",
		},
	}

	result := v.ValidateHelixCodeConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "not found in providers")
}

// ========================================
// ValidateJSON Tests
// ========================================

func TestConfigValidator_ValidateJSON_OpenCode(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	jsonData := `{
		"providers": {
			"local": {
				"apiKey": "test-key"
			}
		},
		"agents": {
			"coder": {
				"model": "local.helixagent-debate",
				"maxTokens": 8192
			}
		}
	}`

	result, err := v.ValidateJSON(AgentTypeOpenCode, []byte(jsonData))
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

func TestConfigValidator_ValidateJSON_Crush(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	jsonData := `{
		"$schema": "https://charm.land/crush.json",
		"providers": {
			"helixagent": {
				"type": "openai-compat",
				"base_url": "http://localhost:7061/v1",
				"models": [{"id": "helixagent-debate"}]
			}
		}
	}`

	result, err := v.ValidateJSON(AgentTypeCrush, []byte(jsonData))
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

func TestConfigValidator_ValidateJSON_HelixCode(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	jsonData := `{
		"providers": {
			"helixagent": {
				"type": "openai-compatible",
				"base_url": "http://localhost:7061/v1",
				"model": "helixagent-debate"
			}
		}
	}`

	result, err := v.ValidateJSON(AgentTypeHelixCode, []byte(jsonData))
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

func TestConfigValidator_ValidateJSON_InvalidJSON(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	jsonData := `{invalid json}`

	_, err := v.ValidateJSON(AgentTypeOpenCode, []byte(jsonData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestConfigValidator_ValidateJSON_UnsupportedAgent(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	_, err := v.ValidateJSON("unknown", []byte(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported agent type")
}

// ========================================
// ValidationResult Tests
// ========================================

func TestValidationResult_String_Valid(t *testing.T) {
	t.Parallel()
	result := &ValidationResult{Valid: true}
	str := result.String()
	assert.Contains(t, str, "Configuration is valid")
}

func TestValidationResult_String_Invalid(t *testing.T) {
	t.Parallel()
	result := &ValidationResult{
		Valid:    false,
		Errors:   []string{"error1", "error2"},
		Warnings: []string{"warning1"},
	}
	str := result.String()
	assert.Contains(t, str, "Configuration is INVALID")
	assert.Contains(t, str, "error1")
	assert.Contains(t, str, "error2")
	assert.Contains(t, str, "warning1")
}

// ========================================
// Integration Tests
// ========================================

func TestConfigGenerator_GenerateAndValidate_AllAgents(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	v := NewConfigValidator()

	agents := []AgentType{AgentTypeOpenCode, AgentTypeCrush, AgentTypeHelixCode}

	for _, agent := range agents {
		t.Run(string(agent), func(t *testing.T) {
			t.Parallel()
			// Generate config
			jsonData, err := gen.GenerateJSON(agent)
			require.NoError(t, err)

			// Validate generated config
			result, err := v.ValidateJSON(agent, jsonData)
			require.NoError(t, err)
			assert.True(t, result.Valid, "Generated config should be valid: %v", result.Errors)
		})
	}
}

// ========================================
// Benchmark Tests
// ========================================

func BenchmarkConfigGenerator_GenerateOpenCodeConfig(b *testing.B) {
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.GenerateOpenCodeConfig()
	}
}

func BenchmarkConfigGenerator_GenerateCrushConfig(b *testing.B) {
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.GenerateCrushConfig()
	}
}

func BenchmarkConfigGenerator_GenerateHelixCodeConfig(b *testing.B) {
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.GenerateHelixCodeConfig()
	}
}

func BenchmarkConfigValidator_ValidateOpenCodeConfig(b *testing.B) {
	v := NewConfigValidator()
	config := &OpenCodeConfig{
		Providers: map[string]OpenCodeProvider{
			"local": {
				APIKey: "test-key",
			},
		},
		Agents: map[string]OpenCodeAgent{
			"coder": {
				Model:     "local.helixagent-debate",
				MaxTokens: 8192,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.ValidateOpenCodeConfig(config)
	}
}

// ========================================
// OpenCode v1.1.30+ Schema Tests
// The new OpenCode schema uses:
// - "providers" (not "provider") - map of provider configs
// - "agents" (not "agent") - map with keys: coder, task, title, summarizer
// - "mcpServers" (not "mcp") - map of MCP server configs
// - Model format is "provider.model-name" (e.g., local.helixagent-debate)
// ========================================

func TestOpenCodeConfig_AgentsSection_HasRequiredAgents(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	config, err := gen.GenerateOpenCodeConfig()
	require.NoError(t, err)

	// Agents section must exist with standard agent names
	require.NotNil(t, config.Agents, "Agents section must be present")
	require.NotEmpty(t, config.Agents, "Agents section must have entries")

	// Check for required agents
	coderAgent, exists := config.Agents["coder"]
	assert.True(t, exists, "Agents section must have 'coder' entry")
	assert.NotEmpty(t, coderAgent.Model, "Agent 'coder' must have a model")
}

func TestOpenCodeConfig_AgentsSection_ModelFormat(t *testing.T) {
	t.Parallel()
	// Verify the model format is "provider.model-name"
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	config, err := gen.GenerateOpenCodeConfig()
	require.NoError(t, err)

	coderAgent := config.Agents["coder"]
	assert.Equal(t, "local.helixagent-debate", coderAgent.Model,
		"Agent model must be in format 'provider.model-name'")
	assert.Contains(t, coderAgent.Model, ".",
		"Agent model must contain '.' separator")
}

func TestOpenCodeConfig_AgentsSection_JSONStructure(t *testing.T) {
	t.Parallel()
	// This test verifies the JSON structure is correct at the raw JSON level
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	jsonData, err := gen.GenerateJSON(AgentTypeOpenCode)
	require.NoError(t, err)

	// Parse as raw JSON to verify structure
	var rawJSON map[string]interface{}
	err = json.Unmarshal(jsonData, &rawJSON)
	require.NoError(t, err)

	// Check agents section exists (plural, not singular)
	agentsSection, ok := rawJSON["agents"]
	require.True(t, ok, "JSON must have 'agents' key (plural)")
	require.NotNil(t, agentsSection, "Agents section must not be nil")

	// Agents section must be a map
	agentsMap, ok := agentsSection.(map[string]interface{})
	require.True(t, ok, "Agents section must be a JSON object")
	require.NotEmpty(t, agentsMap, "Agents section must have at least one entry")

	// Check for standard agent names
	_, hasCoder := agentsMap["coder"]
	assert.True(t, hasCoder, "Agents section should have 'coder' entry")
}

func TestOpenCodeConfig_AgentsSection_Validation_MissingModel(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Providers: map[string]OpenCodeProvider{
			"local": {
				APIKey: "test-key",
			},
		},
		Agents: map[string]OpenCodeAgent{
			"coder": {
				Model: "", // Empty model should fail validation
			},
		},
	}

	result := v.ValidateOpenCodeConfig(config)
	assert.False(t, result.Valid, "Config with empty agent model should be invalid")
	assert.Contains(t, strings.Join(result.Errors, ","), "model is required")
}

func TestOpenCodeConfig_AgentsSection_Validation_InvalidModelFormat(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Providers: map[string]OpenCodeProvider{
			"local": {
				APIKey: "test-key",
			},
		},
		Agents: map[string]OpenCodeAgent{
			"coder": {
				Model: "model-without-provider", // Missing provider. prefix should warn
			},
		},
	}

	result := v.ValidateOpenCodeConfig(config)
	// Should be valid but with warning about format
	assert.True(t, result.Valid)
	assert.Contains(t, strings.Join(result.Warnings, ","), "should be in format 'provider.model-name'")
}

func TestOpenCodeConfig_AgentsSection_CompleteWorkingConfig(t *testing.T) {
	t.Parallel()
	// Test a complete configuration that matches what works with OpenCode v1.1.30+
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Providers: map[string]OpenCodeProvider{
			"local": {
				APIKey: "helixagent-local",
			},
		},
		Agents: map[string]OpenCodeAgent{
			"coder": {
				Model:     "local.helixagent-debate",
				MaxTokens: 8192,
			},
			"task": {
				Model:     "local.helixagent-debate",
				MaxTokens: 4096,
			},
			"title": {
				Model:     "local.helixagent-debate",
				MaxTokens: 80,
			},
			"summarizer": {
				Model:     "local.helixagent-debate",
				MaxTokens: 4096,
			},
		},
		ContextPaths: []string{"CLAUDE.md"},
	}

	result := v.ValidateOpenCodeConfig(config)
	assert.True(t, result.Valid, "Complete working config should be valid: %v", result.Errors)
	assert.Empty(t, result.Errors, "Complete working config should have no errors")
}

func TestOpenCodeConfig_MCPServersSection_Structure(t *testing.T) {
	t.Parallel()
	// Verify the MCPServers section is properly structured
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	config, err := gen.GenerateOpenCodeConfig()
	require.NoError(t, err)

	require.NotNil(t, config.MCPServers, "MCPServers section must exist")
	assert.NotEmpty(t, config.MCPServers, "MCPServers section must have entries")

	// Check that a stdio MCP server has required fields
	if fsMCP, exists := config.MCPServers["filesystem"]; exists {
		assert.NotEmpty(t, fsMCP.Command, "stdio MCP must have command")
	}

	// Check that an SSE MCP server has required fields
	if helixMCP, exists := config.MCPServers["helixagent"]; exists {
		assert.Equal(t, "sse", helixMCP.Type, "HelixAgent MCP must be SSE type")
		assert.NotEmpty(t, helixMCP.URL, "SSE MCP must have URL")
	}
}

func TestOpenCodeConfig_ProvidersSection_LocalProvider(t *testing.T) {
	t.Parallel()
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	config, err := gen.GenerateOpenCodeConfig()
	require.NoError(t, err)

	provider, exists := config.Providers["local"]
	assert.True(t, exists, "Providers must have 'local' entry")
	assert.NotEmpty(t, provider.APIKey, "Local provider must have API key set")
}

func TestOpenCodeConfig_ValidateJSON_WithNewSchema(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator()

	// Valid config with new v1.1.30+ schema
	validJSON := `{
		"providers": {
			"local": {
				"apiKey": "test-key"
			}
		},
		"agents": {
			"coder": {
				"model": "local.helixagent-debate",
				"maxTokens": 8192
			}
		}
	}`

	result, err := v.ValidateJSON(AgentTypeOpenCode, []byte(validJSON))
	require.NoError(t, err)
	assert.True(t, result.Valid, "Config with new schema should be valid: %v", result.Errors)
}

func TestOpenCodeConfig_GeneratedConfigCanBeReParsed(t *testing.T) {
	t.Parallel()
	// Test that generated config can be serialized and deserialized without loss
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	v := NewConfigValidator()

	// Generate config
	originalConfig, err := gen.GenerateOpenCodeConfig()
	require.NoError(t, err)

	// Serialize to JSON
	jsonData, err := json.MarshalIndent(originalConfig, "", "  ")
	require.NoError(t, err)

	// Deserialize back
	var parsedConfig OpenCodeConfig
	err = json.Unmarshal(jsonData, &parsedConfig)
	require.NoError(t, err)

	// Validate parsed config
	result := v.ValidateOpenCodeConfig(&parsedConfig)
	assert.True(t, result.Valid, "Re-parsed config should be valid: %v", result.Errors)

	// Verify agents section survived round-trip
	assert.NotEmpty(t, parsedConfig.Agents, "Agents section must survive round-trip")
	coderAgent, exists := parsedConfig.Agents["coder"]
	assert.True(t, exists, "coder agent must survive round-trip")
	assert.Equal(t, "local.helixagent-debate", coderAgent.Model,
		"Agent model must survive round-trip")
}
