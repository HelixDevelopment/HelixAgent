package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CrushConfig represents the Crush configuration format
// Schema: https://charm.land/crush.json
type CrushConfig struct {
	Schema    string                   `json:"$schema,omitempty"`
	Providers map[string]CrushProvider `json:"providers"`
	MCP       map[string]CrushMCP      `json:"mcp,omitempty"`
	LSP       map[string]CrushLSP      `json:"lsp,omitempty"`
	Options   CrushOptions             `json:"options,omitempty"`
	Keybinds  map[string]string        `json:"keybinds,omitempty"`
}

// CrushProvider represents a provider configuration in Crush
type CrushProvider struct {
	Name     string            `json:"name,omitempty"`
	Type     string            `json:"type"` // openai, openai-compat, anthropic
	BaseURL  string            `json:"base_url,omitempty"`
	APIKey   string            `json:"api_key,omitempty"`
	Models   []CrushModel      `json:"models,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Timeout  int               `json:"timeout,omitempty"`
}

// CrushModel represents a model in Crush configuration
type CrushModel struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name,omitempty"`
	ContextWindow       int      `json:"context_window,omitempty"`
	DefaultMaxTokens    int      `json:"default_max_tokens,omitempty"`
	CostPer1MIn         float64  `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut        float64  `json:"cost_per_1m_out,omitempty"`
	CostPer1MInCached   float64  `json:"cost_per_1m_in_cached,omitempty"`
	CostPer1MOutCached  float64  `json:"cost_per_1m_out_cached,omitempty"`
	// Capabilities
	SupportsVision      bool     `json:"supports_vision,omitempty"`
	SupportsAttachments bool     `json:"supports_attachments,omitempty"`
	SupportsFunctions   bool     `json:"supports_functions,omitempty"`
	SupportsStreaming   bool     `json:"supports_streaming,omitempty"`
	// Fallbacks
	Fallbacks           []string `json:"fallbacks,omitempty"`
}

// CrushMCP represents MCP server configuration in Crush
type CrushMCP struct {
	Type     string            `json:"type"` // stdio, http, sse
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	URL      string            `json:"url,omitempty"`
	Timeout  int               `json:"timeout,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Enabled  bool              `json:"enabled"`
	Disabled bool              `json:"disabled,omitempty"`
}

// CrushLSP represents LSP server configuration in Crush
type CrushLSP struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Enabled bool     `json:"enabled"`
}

// CrushOptions represents global options in Crush
type CrushOptions struct {
	DisableProviderAutoUpdate bool   `json:"disable_provider_auto_update,omitempty"`
	DefaultProvider           string `json:"default_provider,omitempty"`
	StreamingEnabled          bool   `json:"streaming_enabled,omitempty"`
	AutoSave                  bool   `json:"auto_save,omitempty"`
	Theme                     string `json:"theme,omitempty"`
}

// DebateGroupMember represents a member of the AI debate group
type DebateGroupMember struct {
	Provider  string   `json:"provider"`
	Model     string   `json:"model"`
	Score     float64  `json:"score"`
	Fallbacks []string `json:"fallbacks,omitempty"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult holds validation results
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors"`
	Warnings []string          `json:"warnings"`
}

// GenerateHelixAgentCrushConfig creates a Crush configuration for HelixAgent
func GenerateHelixAgentCrushConfig(host string, port int, debateMembers []DebateGroupMember) *CrushConfig {
	baseURL := fmt.Sprintf("http://%s:%d/v1", host, port)

	// Build fallback list from debate members sorted by score
	var fallbacks []string
	for _, member := range debateMembers {
		fallbacks = append(fallbacks, fmt.Sprintf("%s/%s", member.Provider, member.Model))
	}

	config := &CrushConfig{
		Schema: "https://charm.land/crush.json",
		Providers: map[string]CrushProvider{
			"helixagent": {
				Name:    "HelixAgent AI Debate Ensemble",
				Type:    "openai-compat",
				BaseURL: baseURL,
				APIKey:  os.Getenv("HELIXAGENT_API_KEY"),
				Timeout: 600,
				Models: []CrushModel{
					{
						ID:                  "helixagent-debate",
						Name:                "HelixAgent AI Debate Ensemble",
						ContextWindow:       128000,
						DefaultMaxTokens:    8192,
						SupportsVision:      true,
						SupportsAttachments: true,
						SupportsFunctions:   true,
						SupportsStreaming:   true,
						Fallbacks:           fallbacks,
					},
				},
				Headers: map[string]string{
					"X-Client": "Crush",
				},
			},
		},
		MCP: map[string]CrushMCP{
			// HelixAgent protocol endpoints
			"helixagent-mcp": {
				Type:    "http",
				URL:     fmt.Sprintf("http://%s:%d/v1/mcp", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-acp": {
				Type:    "http",
				URL:     fmt.Sprintf("http://%s:%d/v1/acp", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-lsp": {
				Type:    "http",
				URL:     fmt.Sprintf("http://%s:%d/v1/lsp", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-embeddings": {
				Type:    "http",
				URL:     fmt.Sprintf("http://%s:%d/v1/embeddings", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-vision": {
				Type:    "http",
				URL:     fmt.Sprintf("http://%s:%d/v1/vision", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-cognee": {
				Type:    "http",
				URL:     fmt.Sprintf("http://%s:%d/v1/cognee", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			// Standard MCP servers
			"filesystem": {
				Type:    "stdio",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/"},
				Enabled: true,
			},
			"github": {
				Type:    "stdio",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-github"},
				Enabled: true,
			},
			"memory": {
				Type:    "stdio",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-memory"},
				Enabled: true,
			},
			"fetch": {
				Type:    "stdio",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-fetch"},
				Enabled: true,
			},
		},
		LSP: map[string]CrushLSP{
			"go": {
				Command: "gopls",
				Enabled: true,
			},
			"typescript": {
				Command: "typescript-language-server",
				Args:    []string{"--stdio"},
				Enabled: true,
			},
			"python": {
				Command: "pylsp",
				Enabled: true,
			},
			"rust": {
				Command: "rust-analyzer",
				Enabled: true,
			},
		},
		Options: CrushOptions{
			DisableProviderAutoUpdate: false,
			DefaultProvider:           "helixagent",
			StreamingEnabled:          true,
			AutoSave:                  true,
			Theme:                     "default",
		},
		Keybinds: map[string]string{
			"submit":    "ctrl+enter",
			"cancel":    "escape",
			"clear":     "ctrl+l",
			"newline":   "shift+enter",
			"copy":      "ctrl+c",
			"paste":     "ctrl+v",
			"undo":      "ctrl+z",
		},
	}

	return config
}

// ValidateConfig validates a Crush configuration
func ValidateConfig(config *CrushConfig) *ValidationResult {
	result := &ValidationResult{Valid: true, Errors: []ValidationError{}, Warnings: []string{}}

	// Must have providers
	if config.Providers == nil || len(config.Providers) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "providers",
			Message: "at least one provider must be configured",
		})
	}

	// Validate providers
	validTypes := map[string]bool{"openai": true, "openai-compat": true, "anthropic": true, "google": true}
	for name, provider := range config.Providers {
		if !validTypes[provider.Type] {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("providers.%s.type", name),
				Message: fmt.Sprintf("invalid type '%s', must be one of: openai, openai-compat, anthropic, google", provider.Type),
			})
		}

		if provider.Type == "openai-compat" && provider.BaseURL == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("providers.%s.base_url", name),
				Message: "base_url is required for openai-compat type",
			})
		}

		if len(provider.Models) == 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("provider '%s' has no models defined", name))
		}

		for i, model := range provider.Models {
			if model.ID == "" {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("providers.%s.models[%d].id", name, i),
					Message: "model id is required",
				})
			}
		}
	}

	// Validate MCP servers
	validMCPTypes := map[string]bool{"stdio": true, "http": true, "sse": true}
	for name, mcp := range config.MCP {
		if !validMCPTypes[mcp.Type] {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp.%s.type", name),
				Message: fmt.Sprintf("invalid type '%s', must be one of: stdio, http, sse", mcp.Type),
			})
		}

		if mcp.Type == "stdio" && mcp.Command == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp.%s.command", name),
				Message: "command is required for stdio type",
			})
		}

		if (mcp.Type == "http" || mcp.Type == "sse") && mcp.URL == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp.%s.url", name),
				Message: fmt.Sprintf("url is required for %s type", mcp.Type),
			})
		}
	}

	// Validate LSP servers
	for name, lsp := range config.LSP {
		if lsp.Command == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("lsp.%s.command", name),
				Message: "command is required for LSP server",
			})
		}
	}

	return result
}

// SaveConfig saves configuration to file
func SaveConfig(config *CrushConfig, path string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// LoadAndValidateConfig loads and validates an existing config file
func LoadAndValidateConfig(path string) (*CrushConfig, *ValidationResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config CrushConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}

	result := ValidateConfig(&config)
	return &config, result, nil
}

func main() {
	var (
		host       string
		port       int
		outputPath string
		validate   string
	)

	flag.StringVar(&host, "host", "localhost", "HelixAgent host")
	flag.IntVar(&port, "port", 7061, "HelixAgent port")
	flag.StringVar(&outputPath, "output", "", "Output path for generated config")
	flag.StringVar(&validate, "validate", "", "Path to config file to validate")
	flag.Parse()

	// Validation mode
	if validate != "" {
		fmt.Println("=" + repeatString("=", 69))
		fmt.Println("CRUSH CONFIGURATION VALIDATION")
		fmt.Println("=" + repeatString("=", 69))
		fmt.Println()

		config, result, err := LoadAndValidateConfig(validate)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		if result.Valid {
			fmt.Println("CONFIGURATION IS VALID")
			fmt.Println()
			fmt.Printf("Providers configured: %d\n", len(config.Providers))
			fmt.Printf("MCP servers configured: %d\n", len(config.MCP))
			fmt.Printf("LSP servers configured: %d\n", len(config.LSP))
		} else {
			fmt.Println("CONFIGURATION HAS ERRORS:")
			fmt.Println()
			for _, err := range result.Errors {
				fmt.Printf("  - [%s] %s\n", err.Field, err.Message)
			}
			os.Exit(1)
		}

		if len(result.Warnings) > 0 {
			fmt.Println("\nWarnings:")
			for _, warn := range result.Warnings {
				fmt.Printf("  - %s\n", warn)
			}
		}
		fmt.Println()
		fmt.Println("=" + repeatString("=", 69))
		return
	}

	// Generation mode
	if outputPath == "" {
		homeDir, _ := os.UserHomeDir()
		outputPath = filepath.Join(homeDir, "Downloads", "crush-helix-agent.json")
	}

	fmt.Println("=" + repeatString("=", 69))
	fmt.Println("HELIXAGENT CRUSH CONFIGURATION GENERATOR")
	fmt.Println("=" + repeatString("=", 69))
	fmt.Println()

	// Generate config
	config := GenerateHelixAgentCrushConfig(host, port, nil)

	// Validate before saving
	result := ValidateConfig(config)
	if !result.Valid {
		fmt.Println("ERROR: Generated config failed validation:")
		for _, err := range result.Errors {
			fmt.Printf("  - [%s] %s\n", err.Field, err.Message)
		}
		os.Exit(1)
	}

	// Save config
	if err := SaveConfig(config, outputPath); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Configuration generated and validated successfully!")
	fmt.Println()
	fmt.Printf("Output: %s\n", outputPath)
	fmt.Printf("Host: %s\n", host)
	fmt.Printf("Port: %d\n", port)
	fmt.Printf("Schema: https://charm.land/crush.json\n")
	fmt.Printf("Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()
	fmt.Println("Configuration includes:")
	fmt.Printf("  - Providers: %d\n", len(config.Providers))
	fmt.Printf("  - MCP servers: %d\n", len(config.MCP))
	fmt.Printf("  - LSP servers: %d\n", len(config.LSP))
	fmt.Println()
	fmt.Println("=" + repeatString("=", 69))
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
