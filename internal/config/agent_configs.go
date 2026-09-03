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

// AgentConfig holds configuration for HelixAgent to work with different AI agents
type AgentConfig struct {
	BaseURL   string            `json:"base_url"`
	APIKey    string            `json:"api_key,omitempty"`
	Model     string            `json:"model"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timeout   int               `json:"timeout,omitempty"` // in seconds
	MaxTokens int               `json:"max_tokens,omitempty"`
	AgentType AgentType         `json:"agent_type"`
}

// OpenCodeConfig represents configuration for OpenCode (v1.1.30+)
// Based on actual OpenCode source: internal/config/config.go
// Key differences from old schema:
// - "providers" (not "provider") - map of provider configs
// - "agents" (not "agent") - map with keys: coder, task, title, summarizer
// - "mcpServers" (not "mcp") - map of MCP server configs
// - Config file must be named .opencode.json (with leading dot)
// - Uses LOCAL_ENDPOINT env var for local provider base URL
type OpenCodeConfig struct {
	Providers    map[string]OpenCodeProvider  `json:"providers,omitempty"`
	Agents       map[string]OpenCodeAgent     `json:"agents,omitempty"`
	MCPServers   map[string]OpenCodeMCPServer `json:"mcpServers,omitempty"`
	ContextPaths []string                     `json:"contextPaths,omitempty"`
	TUI          *OpenCodeTUI                 `json:"tui,omitempty"`
}

// OpenCodeProvider represents a provider configuration in OpenCode
// Supported providers: local, anthropic, openai, gemini, groq, openrouter, xai, bedrock, azure, vertexai, copilot
type OpenCodeProvider struct {
	APIKey   string `json:"apiKey,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// OpenCodeAgent represents an agent configuration in OpenCode
// Valid agent names: coder, task, title, summarizer
type OpenCodeAgent struct {
	Model           string `json:"model"`                     // Format: provider.model-name (e.g., local.helixagent-debate)
	MaxTokens       int64  `json:"maxTokens,omitempty"`       // Maximum output tokens
	ReasoningEffort string `json:"reasoningEffort,omitempty"` // low, medium, high (for models that support reasoning)
}

// OpenCodeMCPServer represents MCP server configuration in OpenCode
// Type can be "stdio" (default) or "sse"
type OpenCodeMCPServer struct {
	Command string            `json:"command,omitempty"` // Required for stdio type
	Args    []string          `json:"args,omitempty"`
	Env     []string          `json:"env,omitempty"`  // Array of "KEY=VALUE" strings, NOT a map
	Type    string            `json:"type,omitempty"` // "stdio" or "sse"
	URL     string            `json:"url,omitempty"`  // Required for sse type
	Headers map[string]string `json:"headers,omitempty"`
}

// OpenCodeTUI represents TUI configuration
type OpenCodeTUI struct {
	Theme string `json:"theme,omitempty"` // opencode, catppuccin, dracula, etc.
}

// CrushConfig represents configuration for Crush
// Schema: https://charm.land/crush.json
type CrushConfig struct {
	Schema    string                   `json:"$schema,omitempty"`
	Providers map[string]CrushProvider `json:"providers"`
	MCP       map[string]CrushMCP      `json:"mcp,omitempty"`
}

// CrushProvider represents a provider configuration in Crush
type CrushProvider struct {
	Type    string            `json:"type"` // openai, openai-compat, anthropic
	BaseURL string            `json:"base_url,omitempty"`
	APIKey  string            `json:"api_key,omitempty"`
	Models  []CrushModel      `json:"models,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// CrushModel represents a model in Crush configuration
type CrushModel struct {
	ID               string `json:"id"`
	Name             string `json:"name,omitempty"`
	ContextWindow    int    `json:"context_window,omitempty"`
	DefaultMaxTokens int    `json:"default_max_tokens,omitempty"`
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
	Schema    string                       `json:"$schema,omitempty"`
	Providers map[string]HelixCodeProvider `json:"providers"`
	Settings  HelixCodeSettings            `json:"settings,omitempty"`
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
	DefaultProvider  string `json:"default_provider,omitempty"`
	StreamingEnabled bool   `json:"streaming_enabled,omitempty"`
	AutoSave         bool   `json:"auto_save,omitempty"`
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

// GenerateOpenCodeConfig generates OpenCode configuration (v1.1.30+ schema)
// IMPORTANT: OpenCode uses LOCAL_ENDPOINT env var for the "local" provider
// User must set: LOCAL_ENDPOINT=http://localhost:8100 (or their HelixAgent URL)
// Config file must be named .opencode.json (with leading dot)
func (g *ConfigGenerator) GenerateOpenCodeConfig() (*OpenCodeConfig, error) {
	if err := g.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Use "local" provider which reads LOCAL_ENDPOINT env var
	// Model format is "local.{model-name}" where model-name comes from /v1/models
	modelID := fmt.Sprintf("local.%s", g.model)

	config := &OpenCodeConfig{
		Providers: map[string]OpenCodeProvider{
			"local": {
				APIKey: g.apiKey, // Can be any value for local provider
			},
		},
		Agents: map[string]OpenCodeAgent{
			"coder": {
				Model:     modelID,
				MaxTokens: int64(g.maxTokens),
			},
			"task": {
				Model:     modelID,
				MaxTokens: int64(g.maxTokens / 2),
			},
			"title": {
				Model:     modelID,
				MaxTokens: 80,
			},
			"summarizer": {
				Model:     modelID,
				MaxTokens: int64(g.maxTokens / 2),
			},
		},
		MCPServers: g.generateMCPServers(),
		ContextPaths: []string{
			"CLAUDE.md",
			"CLAUDE.local.md",
			"opencode.md",
			".github/copilot-instructions.md",
		},
		TUI: &OpenCodeTUI{
			Theme: "opencode",
		},
	}

	return config, nil
}

// generateMCPServers creates the MCP server configurations for OpenCode
func (g *ConfigGenerator) generateMCPServers() map[string]OpenCodeMCPServer {
	// Parse base URL to get host for SSE MCPs
	baseURL := g.baseURL
	if baseURL == "" {
		baseURL = "http://localhost:8100"
	}

	return map[string]OpenCodeMCPServer{
		// Anthropic Official MCPs
		"filesystem": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/home"},
		},
		"fetch": {
			Command: "npx",
			Args:    []string{"-y", "mcp-fetch-server"},
		},
		"memory": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-memory"},
		},
		"time": {
			Command: "npx",
			Args:    []string{"-y", "@theo.foobar/mcp-time"},
		},
		"git": {
			Command: "npx",
			Args:    []string{"-y", "mcp-git"},
		},
		"sqlite": {
			Command: "npx",
			Args:    []string{"-y", "mcp-server-sqlite-npx", "/tmp/helixagent.db"},
		},
		"postgres": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost:8101/helixagent"},
		},
		"puppeteer": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-puppeteer"},
		},
		"brave-search": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-brave-search"},
			Env:     []string{"BRAVE_API_KEY=${BRAVE_API_KEY}"},
		},
		"google-maps": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-google-maps"},
			Env:     []string{"GOOGLE_MAPS_API_KEY=${GOOGLE_MAPS_API_KEY}"},
		},
		"slack": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-slack"},
			Env:     []string{"SLACK_BOT_TOKEN=${SLACK_BOT_TOKEN}", "SLACK_TEAM_ID=${SLACK_TEAM_ID}"},
		},
		"github": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-github"},
			Env:     []string{"GITHUB_TOKEN=${GITHUB_TOKEN}"},
		},
		"gitlab": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-gitlab"},
			Env:     []string{"GITLAB_TOKEN=${GITLAB_TOKEN}"},
		},
		"sequential-thinking": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
		},
		"everart": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-everart"},
			Env:     []string{"EVERART_API_KEY=${EVERART_API_KEY}"},
		},
		"exa": {
			Command: "npx",
			Args:    []string{"-y", "exa-mcp-server"},
			Env:     []string{"EXA_API_KEY=${EXA_API_KEY}"},
		},
		"sentry": {
			Command: "npx",
			Args:    []string{"-y", "@sentry/mcp-server"},
			Env:     []string{"SENTRY_AUTH_TOKEN=${SENTRY_AUTH_TOKEN}", "SENTRY_ORG=${SENTRY_ORG}"},
		},
		// Restored 2026-09-03 — each replaces a name ce9c4eb9 removed as a
		// verified npm 404. Every replacement was re-verified live against
		// registry.npmjs.org with its declared repository URL confirmed to
		// resolve; per-package version / adoption / publisher-class evidence
		// is recorded on the annotated entries in
		// cmd/helixagent/main.go's buildOpenCodeMCPServersFiltered.
		// The env names below are the ones each package actually reads.
		"linear": {
			Command: "npx",
			Args:    []string{"-y", "@tacticlaunch/mcp-linear"},
			Env:     []string{"LINEAR_API_TOKEN=${LINEAR_API_TOKEN}"},
		},
		"jira": {
			Command: "npx",
			Args:    []string{"-y", "@aashari/mcp-server-atlassian-jira"},
			Env: []string{
				"ATLASSIAN_SITE_NAME=${ATLASSIAN_SITE_NAME}",
				"ATLASSIAN_USER_EMAIL=${ATLASSIAN_USER_EMAIL}",
				"ATLASSIAN_API_TOKEN=${ATLASSIAN_API_TOKEN}",
			},
		},
		"asana": {
			Command: "npx",
			Args:    []string{"-y", "@roychri/mcp-server-asana"},
			Env:     []string{"ASANA_ACCESS_TOKEN=${ASANA_ACCESS_TOKEN}"},
		},
		"google-drive": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-gdrive"},
			Env:     []string{"GDRIVE_CREDENTIALS_PATH=${GDRIVE_CREDENTIALS_PATH}"},
		},
		"notion": {
			Command: "npx",
			Args:    []string{"-y", "@notionhq/notion-mcp-server"},
			Env:     []string{"OPENAI_API_KEY=${OPENAI_API_KEY}"},
		},
		"figma": {
			Command: "npx",
			Args:    []string{"-y", "figma-developer-mcp"},
			Env:     []string{"FIGMA_API_KEY=${FIGMA_API_KEY}"},
		},
		"aws-kb-retrieval": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-aws-kb-retrieval"},
			Env:     []string{"AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}", "AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}"},
		},
		// HelixAgent SSE MCPs
		"helixagent": {
			Type: "sse",
			URL:  baseURL + "/v1/mcp/sse",
		},
		"helixagent-debate": {
			Type: "sse",
			URL:  baseURL + "/v1/mcp/debate/sse",
		},
		"helixagent-rag": {
			Type: "sse",
			URL:  baseURL + "/v1/mcp/rag/sse",
		},
		"helixagent-memory": {
			Type: "sse",
			URL:  baseURL + "/v1/mcp/memory/sse",
		},
		"kubernetes": {
			Command: "npx",
			Args:    []string{"-y", "mcp-server-kubernetes"},
			Env:     []string{"KUBECONFIG=${KUBECONFIG}"},
		},
		"redis": {
			Command: "npx",
			Args:    []string{"-y", "mcp-server-redis"},
			Env:     []string{"REDIS_URL=redis://localhost:6379"},
		},
		"mongodb": {
			Command: "npx",
			Args:    []string{"-y", "mongodb-mcp-server"},
			Env:     []string{"MONGODB_URI=mongodb://localhost:27017"},
		},
		"elasticsearch": {
			Command: "npx",
			Args:    []string{"-y", "mcp-server-elasticsearch"},
			Env:     []string{"ELASTICSEARCH_URL=http://localhost:9200"},
		},
		"qdrant": {
			Command: "npx",
			Args:    []string{"-y", "mcp-server-qdrant"},
			Env:     []string{"QDRANT_URL=http://localhost:6333"},
		},
		"chroma": {
			Command: "npx",
			Args:    []string{"-y", "mcp-server-chroma"},
			Env:     []string{"CHROMA_URL=http://localhost:8001"},
		},
		"aws-s3": {
			Command: "npx",
			Args:    []string{"-y", "mcp-server-s3"},
			Env:     []string{"AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}", "AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}"},
		},
	}
}

// GenerateCrushConfig generates Crush configuration
func (g *ConfigGenerator) GenerateCrushConfig() (*CrushConfig, error) {
	if err := g.validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	config := &CrushConfig{
		Schema: "https://charm.land/crush.json",
		Providers: map[string]CrushProvider{
			"helixagent": {
				Type:    "openai-compat",
				BaseURL: g.baseURL,
				APIKey:  g.apiKey,
				Models: []CrushModel{
					{
						ID:               g.model,
						Name:             "HelixAgent AI Debate Ensemble",
						ContextWindow:    128000,
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
			"helixagent": {
				Type:      "openai-compatible",
				BaseURL:   g.baseURL,
				APIKey:    g.apiKey,
				Model:     g.model,
				MaxTokens: g.maxTokens,
				Timeout:   g.timeout,
			},
		},
		Settings: HelixCodeSettings{
			DefaultProvider:  "helixagent",
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

// ValidateOpenCodeConfig validates OpenCode configuration (v1.1.30+ schema)
func (v *ConfigValidator) ValidateOpenCodeConfig(config *OpenCodeConfig) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Check providers section
	if len(config.Providers) == 0 {
		result.addError("providers section is required")
	}

	// Validate providers - note: for "local" provider, API key can be any value
	// and baseURL comes from LOCAL_ENDPOINT env var
	validProviders := map[string]bool{
		"local": true, "anthropic": true, "openai": true, "gemini": true,
		"groq": true, "openrouter": true, "xai": true, "bedrock": true,
		"azure": true, "vertexai": true, "copilot": true,
	}
	for name := range config.Providers {
		if !validProviders[name] {
			result.addWarning(fmt.Sprintf("provider '%s': unknown provider type", name))
		}
	}

	// Validate agents section
	validAgentNames := map[string]bool{"coder": true, "task": true, "title": true, "summarizer": true}
	for agentName, agent := range config.Agents {
		if !validAgentNames[agentName] {
			result.addWarning(fmt.Sprintf("agent '%s': unknown agent name (valid: coder, task, title, summarizer)", agentName))
		}
		if agent.Model == "" {
			result.addError(fmt.Sprintf("agent '%s': model is required", agentName))
		} else {
			// Validate model format (should be provider.model)
			if !strings.Contains(agent.Model, ".") {
				result.addWarning(fmt.Sprintf("agent '%s': model should be in format 'provider.model-name'", agentName))
			}
		}
		if agent.MaxTokens <= 0 {
			result.addWarning(fmt.Sprintf("agent '%s': maxTokens should be positive", agentName))
		}
	}

	// Validate MCP servers
	for name, mcp := range config.MCPServers {
		mcpType := mcp.Type
		if mcpType == "" {
			mcpType = "stdio" // Default type
		}

		if mcpType != "stdio" && mcpType != "sse" {
			result.addError(fmt.Sprintf("mcpServers '%s': type must be 'stdio' or 'sse'", name))
		}

		if mcpType == "stdio" && mcp.Command == "" {
			result.addError(fmt.Sprintf("mcpServers '%s': command is required for stdio type", name))
		}

		if mcpType == "sse" && mcp.URL == "" {
			result.addError(fmt.Sprintf("mcpServers '%s': url is required for sse type", name))
		}

		if mcpType == "sse" && mcp.URL != "" {
			if _, err := url.Parse(mcp.URL); err != nil {
				result.addError(fmt.Sprintf("mcpServers '%s': invalid url: %v", name, err))
			}
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
