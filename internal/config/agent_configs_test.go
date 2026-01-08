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
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")

	assert.Equal(t, "http://localhost:7061/v1", gen.baseURL)
	assert.Equal(t, "test-key", gen.apiKey)
	assert.Equal(t, "helixagent-debate", gen.model)
	assert.Equal(t, 120, gen.timeout)
	assert.Equal(t, 8192, gen.maxTokens)
}

func TestConfigGenerator_SetTimeout(t *testing.T) {
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "model")
	gen.SetTimeout(300)
	assert.Equal(t, 300, gen.timeout)
}

func TestConfigGenerator_SetMaxTokens(t *testing.T) {
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "model")
	gen.SetMaxTokens(4096)
	assert.Equal(t, 4096, gen.maxTokens)
}

func TestConfigGenerator_Validate_Success(t *testing.T) {
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	err := gen.validate()
	assert.NoError(t, err)
}

func TestConfigGenerator_Validate_EmptyBaseURL(t *testing.T) {
	gen := NewConfigGenerator("", "test-key", "model")
	err := gen.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "baseURL is required")
}

func TestConfigGenerator_Validate_EmptyModel(t *testing.T) {
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "")
	err := gen.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model is required")
}

func TestConfigGenerator_Validate_InvalidURL(t *testing.T) {
	gen := NewConfigGenerator("://invalid", "test-key", "model")
	err := gen.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid baseURL")
}

// ========================================
// OpenCode Config Generation Tests
// ========================================

func TestConfigGenerator_GenerateOpenCodeConfig(t *testing.T) {
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	gen.SetTimeout(180)

	config, err := gen.GenerateOpenCodeConfig()
	require.NoError(t, err)

	assert.Equal(t, "https://opencode.ai/config.json", config.Schema)
	assert.Contains(t, config.Provider, "helixagent")

	provider := config.Provider["helixagent"]
	assert.Equal(t, "@ai-sdk/openai-compatible", provider.NPM)
	assert.Equal(t, "http://localhost:7061/v1", provider.Options.BaseURL)
	assert.Equal(t, "test-key", provider.Options.APIKey)
	assert.Equal(t, 180000, provider.Options.Timeout) // Converted to ms
}

func TestConfigGenerator_GenerateOpenCodeConfig_JSON(t *testing.T) {
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")

	jsonData, err := gen.GenerateJSON(AgentTypeOpenCode)
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// Verify structure
	assert.Contains(t, parsed, "$schema")
	assert.Contains(t, parsed, "provider")

	providers := parsed["provider"].(map[string]interface{})
	assert.Contains(t, providers, "helixagent")
}

// ========================================
// Crush Config Generation Tests
// ========================================

func TestConfigGenerator_GenerateCrushConfig(t *testing.T) {
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
	v := NewConfigValidator()
	assert.NotNil(t, v)
}

// ========================================
// OpenCode Validation Tests
// ========================================

func TestConfigValidator_ValidateOpenCodeConfig_Valid(t *testing.T) {
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		Provider: map[string]OpenCodeProvider{
			"helixagent": {
				NPM: "@ai-sdk/openai-compatible",
				Options: OpenCodeProviderOptions{
					BaseURL: "http://localhost:7061/v1",
					APIKey:  "test-key",
					Timeout: 120000,
				},
			},
		},
	}

	result := v.ValidateOpenCodeConfig(config)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestConfigValidator_ValidateOpenCodeConfig_MissingProvider(t *testing.T) {
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Provider: map[string]OpenCodeProvider{},
	}

	result := v.ValidateOpenCodeConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "provider section is required")
}

func TestConfigValidator_ValidateOpenCodeConfig_MissingBaseURL(t *testing.T) {
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Provider: map[string]OpenCodeProvider{
			"test": {
				Options: OpenCodeProviderOptions{
					BaseURL: "",
				},
			},
		},
	}

	result := v.ValidateOpenCodeConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "baseURL is required")
}

func TestConfigValidator_ValidateOpenCodeConfig_InvalidMCPType(t *testing.T) {
	v := NewConfigValidator()

	config := &OpenCodeConfig{
		Provider: map[string]OpenCodeProvider{
			"test": {
				Options: OpenCodeProviderOptions{
					BaseURL: "http://localhost:7061",
				},
			},
		},
		MCP: map[string]OpenCodeMCP{
			"server1": {
				Type: "invalid",
			},
		},
	}

	result := v.ValidateOpenCodeConfig(config)
	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Errors, ","), "type must be 'local' or 'remote'")
}

// ========================================
// Crush Validation Tests
// ========================================

func TestConfigValidator_ValidateCrushConfig_Valid(t *testing.T) {
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
	v := NewConfigValidator()

	jsonData := `{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"helixagent": {
				"npm": "@ai-sdk/openai-compatible",
				"options": {
					"baseURL": "http://localhost:7061/v1",
					"apiKey": "test-key"
				}
			}
		}
	}`

	result, err := v.ValidateJSON(AgentTypeOpenCode, []byte(jsonData))
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

func TestConfigValidator_ValidateJSON_Crush(t *testing.T) {
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
	v := NewConfigValidator()

	jsonData := `{invalid json}`

	_, err := v.ValidateJSON(AgentTypeOpenCode, []byte(jsonData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestConfigValidator_ValidateJSON_UnsupportedAgent(t *testing.T) {
	v := NewConfigValidator()

	_, err := v.ValidateJSON("unknown", []byte(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported agent type")
}

// ========================================
// ValidationResult Tests
// ========================================

func TestValidationResult_String_Valid(t *testing.T) {
	result := &ValidationResult{Valid: true}
	str := result.String()
	assert.Contains(t, str, "Configuration is valid")
}

func TestValidationResult_String_Invalid(t *testing.T) {
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
	gen := NewConfigGenerator("http://localhost:7061/v1", "test-key", "helixagent-debate")
	v := NewConfigValidator()

	agents := []AgentType{AgentTypeOpenCode, AgentTypeCrush, AgentTypeHelixCode}

	for _, agent := range agents {
		t.Run(string(agent), func(t *testing.T) {
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
		Provider: map[string]OpenCodeProvider{
			"helixagent": {
				Options: OpenCodeProviderOptions{
					BaseURL: "http://localhost:7061/v1",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.ValidateOpenCodeConfig(config)
	}
}
