package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Valid top-level keys per OpenCode.ai official schema
// Source: https://opencode.ai/config.json
var ValidTopLevelKeys = map[string]bool{
	"$schema":      true,
	"plugin":       true,
	"enterprise":   true,
	"instructions": true,
	"provider":     true,
	"mcp":          true,
	"tools":        true,
	"agent":        true,
	"command":      true,
	"keybinds":     true,
	"username":     true,
	"share":        true,
	"permission":   true,
	"compaction":   true,
	"mode":         true,
	"autoshare":    true,
}

// Config represents OpenCode configuration (matching LLMsVerifier types)
// ONLY these top-level keys are valid per LLMsVerifier validator:
// $schema, plugin, enterprise, instructions, provider, mcp, tools, agent,
// command, keybinds, username, share, permission, compaction, mode, autoshare
type Config struct {
	Schema       string                    `json:"$schema,omitempty"`
	Plugin       []string                  `json:"plugin,omitempty"`
	Enterprise   *EnterpriseConfig         `json:"enterprise,omitempty"`
	Instructions []string                  `json:"instructions,omitempty"`
	Provider     map[string]ProviderConfig `json:"provider,omitempty"`
	Mcp          map[string]McpConfig      `json:"mcp,omitempty"`
	Tools        map[string]bool           `json:"tools,omitempty"`
	Agent        map[string]AgentConfig    `json:"agent,omitempty"`
	Command      map[string]CommandConfig  `json:"command,omitempty"`
	Keybinds     *KeybindsConfig           `json:"keybinds,omitempty"`
	Username     string                    `json:"username,omitempty"`
	Share        interface{}               `json:"share,omitempty"`
	Permission   map[string]string         `json:"permission,omitempty"`
	Compaction   *CompactionConfig         `json:"compaction,omitempty"`
	Mode         map[string]interface{}    `json:"mode,omitempty"`
	Autoshare    interface{}               `json:"autoshare,omitempty"`
}

type EnterpriseConfig struct {
	URL string `json:"url,omitempty"`
}

type ProviderConfig struct {
	Npm     string                 `json:"npm,omitempty"`
	Name    string                 `json:"name,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
	Models  map[string]ModelConfig `json:"models,omitempty"`
}

type ModelConfig struct {
	Name           string   `json:"name,omitempty"`
	MaxTokens      int      `json:"maxTokens,omitempty"`
	CostPer1MIn    float64  `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut   float64  `json:"cost_per_1m_out,omitempty"`
	SupportsBrotli bool     `json:"supports_brotli,omitempty"`
	// Enhanced capabilities
	Attachments    bool     `json:"attachments,omitempty"`
	Reasoning      bool     `json:"reasoning,omitempty"`
	Vision         bool     `json:"vision,omitempty"`
	ImageInput     bool     `json:"imageInput,omitempty"`
	ImageOutput    bool     `json:"imageOutput,omitempty"`
	OCR            bool     `json:"ocr,omitempty"`
	PDF            bool     `json:"pdf,omitempty"`
	Audio          bool     `json:"audio,omitempty"`
	Video          bool     `json:"video,omitempty"`
	Streaming      bool     `json:"streaming,omitempty"`
	FunctionCalls  bool     `json:"functionCalls,omitempty"`
	ToolUse        bool     `json:"toolUse,omitempty"`
	Embeddings     bool     `json:"embeddings,omitempty"`
	CodeExecution  bool     `json:"codeExecution,omitempty"`
	WebBrowsing    bool     `json:"webBrowsing,omitempty"`
	FileUpload     bool     `json:"fileUpload,omitempty"`
	NoFileLimit    bool     `json:"noFileLimit,omitempty"`
	// Protocol support
	MCP            bool     `json:"mcp,omitempty"`
	ACP            bool     `json:"acp,omitempty"`
	LSP            bool     `json:"lsp,omitempty"`
	// Fallback configuration
	Fallbacks      []string `json:"fallbacks,omitempty"`
}

type McpConfig struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Timeout     *int              `json:"timeout,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type AgentConfig struct {
	Model       string          `json:"model,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Prompt      string          `json:"prompt,omitempty"`
	Tools       map[string]bool `json:"tools,omitempty"`
	Disable     *bool           `json:"disable,omitempty"`
	Description string          `json:"description,omitempty"`
	Mode        string          `json:"mode,omitempty"`
	Color       string          `json:"color,omitempty"`
	MaxSteps    *int            `json:"maxSteps,omitempty"`
}

type CommandConfig struct {
	Template    string `json:"template"`
	Description string `json:"description,omitempty"`
	Agent       string `json:"agent,omitempty"`
	Model       string `json:"model,omitempty"`
}

type KeybindsConfig struct {
	Leader   string `json:"leader,omitempty"`
	AppExit  string `json:"app_exit,omitempty"`
}

type CompactionConfig struct {
	Auto  *bool `json:"auto,omitempty"`
	Prune *bool `json:"prune,omitempty"`
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

// GenerateHelixAgentConfig creates an OpenCode configuration for HelixAgent
// Uses ONLY valid top-level keys from LLMsVerifier's schema validation
// Includes all protocol support: MCP, ACP, LSP, Embeddings, Vision, and more
func GenerateHelixAgentConfig(host string, port int, debateMembers []DebateGroupMember) *Config {
	baseURL := fmt.Sprintf("http://%s:%d/v1", host, port)

	enabled := true
	temperature := 0.7
	timeout := 60000 // 60 seconds in milliseconds - required for MCP SSE connections

	// Build fallback list from debate members sorted by score
	var fallbacks []string
	for _, member := range debateMembers {
		fallbacks = append(fallbacks, fmt.Sprintf("%s/%s", member.Provider, member.Model))
	}

	config := &Config{
		Schema:   "https://opencode.ai/config.json",
		Username: "HelixAgent AI Ensemble",
		Instructions: []string{
			"You are connected to HelixAgent, a Virtual LLM Provider that exposes ONE model backed by an AI debate ensemble.",
			"The helixagent-debate model combines responses from multiple top-performing LLMs through confidence-weighted voting.",
			"All underlying models have been verified through real API calls by LLMsVerifier.",
			"This ensemble supports: MCP, ACP, LSP protocols, embeddings, vision/OCR, file handling, and all generative capabilities.",
			"Protocol requests use automatic fallback to the next strongest LLM if the primary fails.",
		},
		// Provider configuration (REQUIRED per LLMsVerifier validator)
		Provider: map[string]ProviderConfig{
			"helixagent": {
				Npm:  "@ai-sdk/openai-compatible",
				Name: "HelixAgent AI Debate Ensemble",
				Options: map[string]interface{}{
					"apiKey":  os.Getenv("HELIXAGENT_API_KEY"),
					"baseURL": baseURL,
					"timeout": 600000,
				},
				Models: map[string]ModelConfig{
					"helixagent-debate": {
						Name:          "HelixAgent Debate Ensemble",
						MaxTokens:     128000,
						// Full capabilities - combined from all 15 LLMs in debate team
						Attachments:   true,
						Reasoning:     true,
						Vision:        true,
						ImageInput:    true,
						ImageOutput:   true,
						OCR:           true,
						PDF:           true,
						Audio:         true,
						Video:         true,
						Streaming:     true,
						FunctionCalls: true,
						ToolUse:       true,
						Embeddings:    true,
						CodeExecution: true,
						WebBrowsing:   true,
						FileUpload:    true,
						NoFileLimit:   true,
						// Protocol support
						MCP:           true,
						ACP:           true,
						LSP:           true,
						// Fallback chain (sorted by LLMsVerifier score)
						Fallbacks:     fallbacks,
					},
				},
			},
		},
		// MCP servers - comprehensive protocol support
		Mcp: map[string]McpConfig{
			// HelixAgent native protocol endpoints
			"helixagent-mcp": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/mcp", host, port),
				Enabled: &enabled,
				Timeout: &timeout,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-acp": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/acp", host, port),
				Enabled: &enabled,
				Timeout: &timeout,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-lsp": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/lsp", host, port),
				Enabled: &enabled,
				Timeout: &timeout,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-embeddings": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/embeddings", host, port),
				Enabled: &enabled,
				Timeout: &timeout,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-vision": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/vision", host, port),
				Enabled: &enabled,
				Timeout: &timeout,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-cognee": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/cognee", host, port),
				Enabled: &enabled,
				Timeout: &timeout,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			// Standard MCP servers
			"filesystem": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "/"},
				Enabled: &enabled,
			},
			"github": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"},
				Enabled: &enabled,
				Environment: map[string]string{
					"GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN"),
				},
			},
			"memory": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-memory"},
				Enabled: &enabled,
			},
			"fetch": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-fetch"},
				Enabled: &enabled,
			},
			"puppeteer": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-puppeteer"},
				Enabled: &enabled,
			},
			"sqlite": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-sqlite"},
				Enabled: &enabled,
			},
		},
		// Agent configurations (must have model or prompt per LLMsVerifier)
		Agent: map[string]AgentConfig{
			"default": {
				Model:       "helixagent/helixagent-debate",
				Temperature: &temperature,
				Prompt:      "You are HelixAgent, an AI ensemble that combines the intelligence of multiple top-performing LLMs through debate and consensus. You have full access to MCP, ACP, LSP protocols, embeddings, vision/OCR, and all generative capabilities.",
				Description: "HelixAgent AI Debate Ensemble - 15 verified LLMs with protocol support and fallback chain",
				Tools: map[string]bool{
					"read":       true,
					"write":      true,
					"bash":       true,
					"glob":       true,
					"grep":       true,
					"webfetch":   true,
					"edit":       true,
					"mcp":        true,
					"embeddings": true,
					"vision":     true,
				},
			},
			"code-reviewer": {
				Model:       "helixagent/helixagent-debate",
				Prompt:      "You are a code reviewer with access to LSP protocol for code intelligence. Analyze code for bugs, security issues, and improvements.",
				Description: "Code review agent with LSP support",
				Tools: map[string]bool{
					"read":  true,
					"write": false,
					"bash":  false,
					"lsp":   true,
				},
			},
			"embeddings-agent": {
				Model:       "helixagent/helixagent-debate",
				Prompt:      "You are an embeddings specialist. Generate and work with vector embeddings for semantic search and similarity.",
				Description: "Embeddings specialist agent",
				Tools: map[string]bool{
					"read":       true,
					"embeddings": true,
				},
			},
			"vision-agent": {
				Model:       "helixagent/helixagent-debate",
				Prompt:      "You are a vision specialist. Analyze images, perform OCR, and understand visual content.",
				Description: "Vision and OCR specialist agent",
				Tools: map[string]bool{
					"read":   true,
					"vision": true,
					"ocr":    true,
				},
			},
		},
		// Permission model - only valid keys: read, edit, bash, webfetch
		Permission: map[string]string{
			"read":     "allow",
			"edit":     "ask",
			"bash":     "ask",
			"webfetch": "allow",
		},
		// Tools configuration - boolean values
		Tools: map[string]bool{
			"Read":      true,
			"Write":     true,
			"Bash":      true,
			"Glob":      true,
			"Grep":      true,
			"Edit":      true,
			"WebFetch":  true,
			"Task":      true,
			"TodoWrite": true,
		},
		// Compaction settings
		Compaction: &CompactionConfig{
			Auto:  &enabled,
			Prune: &enabled,
		},
	}

	return config
}

// ValidateConfig validates an OpenCode configuration
func ValidateConfig(config *Config) *ValidationResult {
	result := &ValidationResult{Valid: true, Errors: []ValidationError{}, Warnings: []string{}}

	// Must have provider
	if config.Provider == nil || len(config.Provider) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "provider",
			Message: "at least one provider must be configured",
		})
	}

	// Validate each provider
	for name, provider := range config.Provider {
		if provider.Options == nil || len(provider.Options) == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("provider.%s.options", name),
				Message: "provider must have options configured",
			})
		}
	}

	// Validate MCP servers
	for name, mcp := range config.Mcp {
		if mcp.Type != "local" && mcp.Type != "remote" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp.%s.type", name),
				Message: "type must be 'local' or 'remote'",
			})
		}
		if mcp.Type == "local" && len(mcp.Command) == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp.%s.command", name),
				Message: "command is required for local MCP servers",
			})
		}
		if mcp.Type == "remote" && mcp.URL == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp.%s.url", name),
				Message: "url is required for remote MCP servers",
			})
		}
	}

	// Validate agents
	for name, agent := range config.Agent {
		if agent.Model == "" && agent.Prompt == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("agent.%s", name),
				Message: "agent must have either model or prompt configured",
			})
		}
	}

	return result
}

// ValidateTopLevelKeys checks for invalid top-level keys
func ValidateTopLevelKeys(configJSON []byte) *ValidationResult {
	result := &ValidationResult{Valid: true, Errors: []ValidationError{}, Warnings: []string{}}

	var rawConfig map[string]interface{}
	if err := json.Unmarshal(configJSON, &rawConfig); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "",
			Message: fmt.Sprintf("invalid JSON: %v", err),
		})
		return result
	}

	// Check for invalid top-level keys
	var invalidKeys []string
	for key := range rawConfig {
		if !ValidTopLevelKeys[key] {
			invalidKeys = append(invalidKeys, key)
		}
	}

	if len(invalidKeys) > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "",
			Message: fmt.Sprintf("invalid top-level keys: %v", invalidKeys),
		})
	}

	return result
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config, path string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// LoadAndValidateConfig loads and validates an existing config file
func LoadAndValidateConfig(path string) (*Config, *ValidationResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config: %w", err)
	}

	// First validate top-level keys
	topLevelResult := ValidateTopLevelKeys(data)
	if !topLevelResult.Valid {
		return nil, topLevelResult, nil
	}

	// Parse config
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate structure
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
		fmt.Println("OPENCODE CONFIGURATION VALIDATION (LLMsVerifier)")
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
			fmt.Printf("Providers configured: %d\n", len(config.Provider))
			fmt.Printf("MCP servers configured: %d\n", len(config.Mcp))
			fmt.Printf("Agents configured: %d\n", len(config.Agent))
		} else {
			fmt.Println("CONFIGURATION HAS ERRORS:")
			fmt.Println()
			for _, err := range result.Errors {
				fmt.Printf("  - [%s] %s\n", err.Field, err.Message)
			}
			os.Exit(1)
		}
		fmt.Println()
		fmt.Println("=" + repeatString("=", 69))
		return
	}

	// Generation mode
	if outputPath == "" {
		// Default to user's Downloads folder
		homeDir, _ := os.UserHomeDir()
		outputPath = filepath.Join(homeDir, "Downloads", "opencode-helix-agent.json")
	}

	fmt.Println("=" + repeatString("=", 69))
	fmt.Println("HELIXAGENT OPENCODE CONFIGURATION GENERATOR")
	fmt.Println("Using LLMsVerifier validation implementation")
	fmt.Println("=" + repeatString("=", 69))
	fmt.Println()

	// Generate config
	config := GenerateHelixAgentConfig(host, port, nil)

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

	// Validate the saved file (to ensure JSON is valid)
	_, finalResult, err := LoadAndValidateConfig(outputPath)
	if err != nil {
		fmt.Printf("Error validating saved config: %v\n", err)
		os.Exit(1)
	}

	if !finalResult.Valid {
		fmt.Println("ERROR: Saved config failed validation:")
		for _, err := range finalResult.Errors {
			fmt.Printf("  - [%s] %s\n", err.Field, err.Message)
		}
		os.Exit(1)
	}

	fmt.Println("Configuration generated and validated successfully!")
	fmt.Println()
	fmt.Printf("Output: %s\n", outputPath)
	fmt.Printf("Host: %s\n", host)
	fmt.Printf("Port: %d\n", port)
	fmt.Printf("Schema: https://opencode.ai/config.json\n")
	fmt.Printf("Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()
	fmt.Println("Configuration includes:")
	fmt.Printf("  - Providers: %d\n", len(config.Provider))
	fmt.Printf("  - MCP servers: %d\n", len(config.Mcp))
	fmt.Printf("  - Agents: %d\n", len(config.Agent))
	fmt.Printf("  - Commands: %d\n", len(config.Command))
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
