// Package config provides configuration generation and validation for AI coding agents.
// Supports OpenCode, Crush, and HelixCode configuration formats.
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// AgentType represents the type of AI coding agent
type AgentType string

const (
	AgentTypeOpenCode  AgentType = "opencode"
	AgentTypeCrush     AgentType = "crush"
	AgentTypeHelixCode AgentType = "helixcode"
)

// AgentConfig holds configuration for SuperAgent to work with different AI agents
type AgentConfig struct {
	BaseURL    string            `json:"base_url"`
	APIKey     string            `json:"api_key,omitempty"`
	Model      string            `json:"model"`
	Headers    map[string]string `json:"headers,omitempty"`
	Timeout    int               `json:"timeout,omitempty"` // in seconds
	MaxTokens  int               `json:"max_tokens,omitempty"`
	AgentType  AgentType         `json:"agent_type"`
}

// OpenCodeConfig represents configuration for OpenCode
// Schema: https://opencode.ai/config.json
type OpenCodeConfig struct {
	Schema   string                    `json:"$schema,omitempty"`
	Provider map[string]OpenCodeProvider `json:"provider"`
	MCP      map[string]OpenCodeMCP    `json:"mcp,omitempty"`
}

// OpenCodeProvider represents a provider configuration in OpenCode
type OpenCodeProvider struct {
	NPM     string                 `json:"npm,omitempty"`
	Options OpenCodeProviderOptions `json:"options"`
}

// OpenCodeProviderOptions represents provider options in OpenCode
type OpenCodeProviderOptions struct {
	BaseURL string `json:"baseURL"`
	APIKey  string `json:"apiKey,omitempty"`
	Timeout int    `json:"timeout,omitempty"` // in milliseconds
}

// OpenCodeMCP represents MCP server configuration in OpenCode
type OpenCodeMCP struct {
	Type    string            `json:"type"` // local or remote
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// CrushConfig represents configuration for Crush
// Schema: https://charm.land/crush.json
type CrushConfig struct {
	Schema    string                  `json:"$schema,omitempty"`
	Providers map[string]CrushProvider `json:"providers"`
	MCP       map[string]CrushMCP     `json:"mcp,omitempty"`
}

// CrushProvider represents a provider configuration in Crush
type CrushProvider struct {
	Type     string       `json:"type"` // openai, openai-compat, anthropic
	BaseURL  string       `json:"base_url,omitempty"`
	APIKey   string       `json:"api_key,omitempty"`
	Models   []CrushModel `json:"models,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// CrushModel represents a model in Crush configuration
type CrushModel struct {
	ID              string `json:"id"`
	Name            string `json:"name,omitempty"`
	ContextWindow   int    `json:"context_window,omitempty"`
	DefaultMaxTokens int   `json:"default_max_tokens,omitempty"`
}

// CrushMCP represents MCP server configuration in Crush
type CrushMCP struct {
	Type     string            `json:"type"` // stdio, http, sse
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	URL      string            `json:"url,omitempty"`
	Timeout  int               `json:"timeout,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
}

// HelixCodeConfig represents configuration for HelixCode
type HelixCodeConfig struct {
	Schema    string                     `json:"$schema,omitempty"`
	Providers map[string]HelixCodeProvider `json:"providers"`
	Settings  HelixCodeSettings          `json:"settings,omitempty"`
}

// HelixCodeProvider represents a provider configuration in HelixCode
type HelixCodeProvider struct {
	Type      string            `json:"type"` // openai-compatible
	BaseURL   string            `json:"base_url"`
	APIKey    string            `json:"api_key,omitempty"`
	Model     string            `json:"model"`
	MaxTokens int               `json:"max_tokens,omitempty"`
	Timeout   int               `json:"timeout,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// HelixCodeSettings represents global settings for HelixCode
type HelixCodeSettings struct {
	DefaultProvider string `json:"default_provider,omitempty"`
	StreamingEnabled bool  `json:"streaming_enabled,omitempty"`
	AutoSave        bool   `json:"auto_save,omitempty"`
}

// ConfigGenerator generates configurations for different AI agents
type ConfigGenerator struct {
	baseURL   string
	apiKey    string
	model     string
	timeout   int
	maxTokens int
}

// NewConfigGenerator creates a new configuration generator
func NewConfigGenerator(baseURL, apiKey, model string) *ConfigGenerator {
	return &ConfigGenerator{
		baseURL:   baseURL,
		apiKey:    apiKey,
		model:     model,
		timeout:   120,
		maxTokens: 8192,
	}
}

// SetTimeout sets the timeout in seconds
func (g *ConfigGenerator) SetTimeout(seconds int) *ConfigGenerator {
	g.timeout = seconds
	return g
}

// SetMaxTokens sets the max tokens
func (g *ConfigGenerator) SetMaxTokens(tokens int) *ConfigGenerator {
	g.maxTokens = tokens
	return g
}

// GenerateOpenCodeConfig generates OpenCode configuration
func (g *ConfigGenerator) GenerateOpenCodeConfig() (*OpenCodeConfig, error) {
	if err := g.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	config := &OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		Provider: map[string]OpenCodeProvider{
			"superagent": {
				NPM: "@ai-sdk/openai-compatible",
				Options: OpenCodeProviderOptions{
					BaseURL: g.baseURL,
					APIKey:  g.apiKey,
					Timeout: g.timeout * 1000, // Convert to milliseconds
				},
			},
		},
	}

	return config, nil
}

// GenerateCrushConfig generates Crush configuration
func (g *ConfigGenerator) GenerateCrushConfig() (*CrushConfig, error) {
	if err := g.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	config := &CrushConfig{
		Schema: "https://charm.land/crush.json",
		Providers: map[string]CrushProvider{
			"superagent": {
				Type:    "openai-compat",
				BaseURL: g.baseURL,
				APIKey:  g.apiKey,
				Models: []CrushModel{
					{
						ID:              g.model,
						Name:            "SuperAgent AI Debate Ensemble",
						ContextWindow:   128000,
						DefaultMaxTokens: g.maxTokens,
					},
				},
			},
		},
	}

	return config, nil
}

// GenerateHelixCodeConfig generates HelixCode configuration
func (g *ConfigGenerator) GenerateHelixCodeConfig() (*HelixCodeConfig, error) {
	if err := g.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	config := &HelixCodeConfig{
		Providers: map[string]HelixCodeProvider{
			"superagent": {
				Type:      "openai-compatible",
				BaseURL:   g.baseURL,
				APIKey:    g.apiKey,
				Model:     g.model,
				MaxTokens: g.maxTokens,
				Timeout:   g.timeout,
			},
		},
		Settings: HelixCodeSettings{
			DefaultProvider:  "superagent",
			StreamingEnabled: true,
			AutoSave:         true,
		},
	}

	return config, nil
}

// GenerateConfig generates configuration for the specified agent type
func (g *ConfigGenerator) GenerateConfig(agentType AgentType) (interface{}, error) {
	switch agentType {
	case AgentTypeOpenCode:
		return g.GenerateOpenCodeConfig()
	case AgentTypeCrush:
		return g.GenerateCrushConfig()
	case AgentTypeHelixCode:
		return g.GenerateHelixCodeConfig()
	default:
		return nil, fmt.Errorf("unsupported agent type: %s", agentType)
	}
}

// GenerateJSON generates JSON configuration for the specified agent type
func (g *ConfigGenerator) GenerateJSON(agentType AgentType) ([]byte, error) {
	config, err := g.GenerateConfig(agentType)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(config, "", "  ")
}

// validate validates the generator configuration
func (g *ConfigGenerator) validate() error {
	if g.baseURL == "" {
		return fmt.Errorf("baseURL is required")
	}

	// Validate URL format
	if _, err := url.Parse(g.baseURL); err != nil {
		return fmt.Errorf("invalid baseURL: %w", err)
	}

	if g.model == "" {
		return fmt.Errorf("model is required")
	}

	return nil
}

// ConfigValidator validates configurations for different AI agents
type ConfigValidator struct{}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{}
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateOpenCodeConfig validates OpenCode configuration
func (v *ConfigValidator) ValidateOpenCodeConfig(config *OpenCodeConfig) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Check required fields
	if len(config.Provider) == 0 {
		result.addError("provider section is required")
	}

	for name, provider := range config.Provider {
		if provider.Options.BaseURL == "" {
			result.addError(fmt.Sprintf("provider '%s': baseURL is required", name))
		} else {
			// Validate URL
			if _, err := url.Parse(provider.Options.BaseURL); err != nil {
				result.addError(fmt.Sprintf("provider '%s': invalid baseURL: %v", name, err))
			}
		}

		// Check for recommended fields
		if provider.Options.Timeout == 0 {
			result.addWarning(fmt.Sprintf("provider '%s': timeout not set, using default", name))
		}
	}

	// Validate MCP servers
	for name, mcp := range config.MCP {
		if mcp.Type != "local" && mcp.Type != "remote" {
			result.addError(fmt.Sprintf("mcp '%s': type must be 'local' or 'remote'", name))
		}

		if mcp.Type == "local" && mcp.Command == "" {
			result.addError(fmt.Sprintf("mcp '%s': command is required for local type", name))
		}

		if mcp.Type == "remote" && mcp.URL == "" {
			result.addError(fmt.Sprintf("mcp '%s': url is required for remote type", name))
		}
	}

	return result
}

// ValidateCrushConfig validates Crush configuration
func (v *ConfigValidator) ValidateCrushConfig(config *CrushConfig) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Check required fields
	if len(config.Providers) == 0 {
		result.addError("providers section is required")
	}

	validTypes := map[string]bool{"openai": true, "openai-compat": true, "anthropic": true}

	for name, provider := range config.Providers {
		if !validTypes[provider.Type] {
			result.addError(fmt.Sprintf("provider '%s': invalid type '%s'", name, provider.Type))
		}

		if provider.Type == "openai-compat" && provider.BaseURL == "" {
			result.addError(fmt.Sprintf("provider '%s': base_url is required for openai-compat type", name))
		}

		if len(provider.Models) == 0 {
			result.addWarning(fmt.Sprintf("provider '%s': no models defined", name))
		}

		for i, model := range provider.Models {
			if model.ID == "" {
				result.addError(fmt.Sprintf("provider '%s': model[%d] id is required", name, i))
			}
		}
	}

	// Validate MCP servers
	validMCPTypes := map[string]bool{"stdio": true, "http": true, "sse": true}
	for name, mcp := range config.MCP {
		if !validMCPTypes[mcp.Type] {
			result.addError(fmt.Sprintf("mcp '%s': invalid type '%s'", name, mcp.Type))
		}

		if mcp.Type == "stdio" && mcp.Command == "" {
			result.addError(fmt.Sprintf("mcp '%s': command is required for stdio type", name))
		}

		if (mcp.Type == "http" || mcp.Type == "sse") && mcp.URL == "" {
			result.addError(fmt.Sprintf("mcp '%s': url is required for %s type", name, mcp.Type))
		}
	}

	return result
}

// ValidateHelixCodeConfig validates HelixCode configuration
func (v *ConfigValidator) ValidateHelixCodeConfig(config *HelixCodeConfig) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Check required fields
	if len(config.Providers) == 0 {
		result.addError("providers section is required")
	}

	for name, provider := range config.Providers {
		if provider.BaseURL == "" {
			result.addError(fmt.Sprintf("provider '%s': base_url is required", name))
		} else {
			// Validate URL
			if _, err := url.Parse(provider.BaseURL); err != nil {
				result.addError(fmt.Sprintf("provider '%s': invalid base_url: %v", name, err))
			}
		}

		if provider.Model == "" {
			result.addError(fmt.Sprintf("provider '%s': model is required", name))
		}

		// Check for recommended fields
		if provider.Timeout == 0 {
			result.addWarning(fmt.Sprintf("provider '%s': timeout not set, using default", name))
		}
	}

	// Validate settings
	if config.Settings.DefaultProvider != "" {
		if _, exists := config.Providers[config.Settings.DefaultProvider]; !exists {
			result.addError(fmt.Sprintf("settings.default_provider '%s' not found in providers", config.Settings.DefaultProvider))
		}
	}

	return result
}

// ValidateJSON validates JSON configuration for the specified agent type
func (v *ConfigValidator) ValidateJSON(agentType AgentType, data []byte) (*ValidationResult, error) {
	switch agentType {
	case AgentTypeOpenCode:
		var config OpenCodeConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return v.ValidateOpenCodeConfig(&config), nil

	case AgentTypeCrush:
		var config CrushConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return v.ValidateCrushConfig(&config), nil

	case AgentTypeHelixCode:
		var config HelixCodeConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return v.ValidateHelixCodeConfig(&config), nil

	default:
		return nil, fmt.Errorf("unsupported agent type: %s", agentType)
	}
}

func (r *ValidationResult) addError(msg string) {
	r.Valid = false
	r.Errors = append(r.Errors, msg)
}

func (r *ValidationResult) addWarning(msg string) {
	r.Warnings = append(r.Warnings, msg)
}

// String returns a human-readable validation result
func (r *ValidationResult) String() string {
	var sb strings.Builder

	if r.Valid {
		sb.WriteString("Configuration is valid\n")
	} else {
		sb.WriteString("Configuration is INVALID\n")
	}

	if len(r.Errors) > 0 {
		sb.WriteString("\nErrors:\n")
		for _, err := range r.Errors {
			sb.WriteString("  - " + err + "\n")
		}
	}

	if len(r.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, warn := range r.Warnings {
			sb.WriteString("  - " + warn + "\n")
		}
	}

	return sb.String()
}
