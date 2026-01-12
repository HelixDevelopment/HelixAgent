package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// KiloCodeConfig represents the Kilo Code configuration format
// Kilo Code is a VS Code-based AI coding assistant with custom provider support
type KiloCodeConfig struct {
	Schema      string                     `json:"$schema,omitempty"`
	Version     string                     `json:"version,omitempty"`
	Providers   map[string]KiloProvider    `json:"providers"`
	Agents      map[string]KiloAgent       `json:"agents,omitempty"`
	MCP         map[string]KiloMCP         `json:"mcp,omitempty"`
	Tools       map[string]bool            `json:"tools,omitempty"`
	Settings    KiloSettings               `json:"settings,omitempty"`
	Permissions KiloPermissions            `json:"permissions,omitempty"`
	Shortcuts   map[string]string          `json:"shortcuts,omitempty"`
}

// KiloProvider represents a provider configuration in Kilo Code
type KiloProvider struct {
	Type        string            `json:"type"` // openai-compatible, anthropic, azure, google
	Name        string            `json:"name,omitempty"`
	BaseURL     string            `json:"baseUrl,omitempty"`
	APIKey      string            `json:"apiKey,omitempty"`
	Models      []KiloModel       `json:"models,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Timeout     int               `json:"timeout,omitempty"`
	MaxRetries  int               `json:"maxRetries,omitempty"`
}

// KiloModel represents a model in Kilo Code configuration
type KiloModel struct {
	ID              string            `json:"id"`
	Name            string            `json:"name,omitempty"`
	Description     string            `json:"description,omitempty"`
	MaxTokens       int               `json:"maxTokens,omitempty"`
	ContextWindow   int               `json:"contextWindow,omitempty"`
	// Pricing
	CostPer1MIn     float64           `json:"costPer1MIn,omitempty"`
	CostPer1MOut    float64           `json:"costPer1MOut,omitempty"`
	// Capabilities
	Capabilities    KiloCapabilities  `json:"capabilities,omitempty"`
	// Fallbacks
	Fallbacks       []string          `json:"fallbacks,omitempty"`
}

// KiloCapabilities represents model capabilities
type KiloCapabilities struct {
	Vision          bool `json:"vision,omitempty"`
	ImageInput      bool `json:"imageInput,omitempty"`
	ImageOutput     bool `json:"imageOutput,omitempty"`
	OCR             bool `json:"ocr,omitempty"`
	PDF             bool `json:"pdf,omitempty"`
	Audio           bool `json:"audio,omitempty"`
	Video           bool `json:"video,omitempty"`
	Streaming       bool `json:"streaming,omitempty"`
	FunctionCalls   bool `json:"functionCalls,omitempty"`
	ToolUse         bool `json:"toolUse,omitempty"`
	Embeddings      bool `json:"embeddings,omitempty"`
	CodeExecution   bool `json:"codeExecution,omitempty"`
	FileUpload      bool `json:"fileUpload,omitempty"`
	Reasoning       bool `json:"reasoning,omitempty"`
	// Protocols
	MCP             bool `json:"mcp,omitempty"`
	ACP             bool `json:"acp,omitempty"`
	LSP             bool `json:"lsp,omitempty"`
}

// KiloAgent represents an agent configuration in Kilo Code
type KiloAgent struct {
	Model           string          `json:"model"`
	Provider        string          `json:"provider,omitempty"`
	SystemPrompt    string          `json:"systemPrompt,omitempty"`
	Description     string          `json:"description,omitempty"`
	Temperature     float64         `json:"temperature,omitempty"`
	MaxTokens       int             `json:"maxTokens,omitempty"`
	Tools           map[string]bool `json:"tools,omitempty"`
	Mode            string          `json:"mode,omitempty"` // code, chat, review
	Color           string          `json:"color,omitempty"`
}

// KiloMCP represents MCP server configuration in Kilo Code
type KiloMCP struct {
	Type        string            `json:"type"` // local, remote, stdio
	Command     []string          `json:"command,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Enabled     bool              `json:"enabled"`
	Timeout     int               `json:"timeout,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

// KiloSettings represents global settings for Kilo Code
type KiloSettings struct {
	DefaultProvider     string `json:"defaultProvider,omitempty"`
	DefaultModel        string `json:"defaultModel,omitempty"`
	DefaultAgent        string `json:"defaultAgent,omitempty"`
	StreamingEnabled    bool   `json:"streamingEnabled,omitempty"`
	AutoSave            bool   `json:"autoSave,omitempty"`
	CompactionEnabled   bool   `json:"compactionEnabled,omitempty"`
	PruneEnabled        bool   `json:"pruneEnabled,omitempty"`
	Theme               string `json:"theme,omitempty"`
	Language            string `json:"language,omitempty"`
	TelemetryEnabled    bool   `json:"telemetryEnabled,omitempty"`
}

// KiloPermissions represents permission settings
type KiloPermissions struct {
	Read      string `json:"read,omitempty"`      // allow, ask, deny
	Write     string `json:"write,omitempty"`
	Edit      string `json:"edit,omitempty"`
	Bash      string `json:"bash,omitempty"`
	WebFetch  string `json:"webFetch,omitempty"`
	MCP       string `json:"mcp,omitempty"`
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

// GenerateHelixAgentKiloConfig creates a Kilo Code configuration for HelixAgent
func GenerateHelixAgentKiloConfig(host string, port int, debateMembers []DebateGroupMember) *KiloCodeConfig {
	baseURL := fmt.Sprintf("http://%s:%d/v1", host, port)

	// Build fallback list from debate members sorted by score
	var fallbacks []string
	for _, member := range debateMembers {
		fallbacks = append(fallbacks, fmt.Sprintf("%s/%s", member.Provider, member.Model))
	}

	config := &KiloCodeConfig{
		Schema:  "https://kilocode.dev/config-schema.json",
		Version: "1.0",
		Providers: map[string]KiloProvider{
			"helixagent": {
				Type:    "openai-compatible",
				Name:    "HelixAgent AI Debate Ensemble",
				BaseURL: baseURL,
				APIKey:  os.Getenv("HELIXAGENT_API_KEY"),
				Timeout: 600000,
				MaxRetries: 3,
				Models: []KiloModel{
					{
						ID:            "helixagent-debate",
						Name:          "HelixAgent AI Debate Ensemble",
						Description:   "Virtual LLM backed by 15 verified AI models through debate and consensus",
						MaxTokens:     8192,
						ContextWindow: 128000,
						Capabilities: KiloCapabilities{
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
							FileUpload:    true,
							Reasoning:     true,
							MCP:           true,
							ACP:           true,
							LSP:           true,
						},
						Fallbacks: fallbacks,
					},
				},
				Headers: map[string]string{
					"X-Client": "KiloCode",
				},
			},
		},
		Agents: map[string]KiloAgent{
			"default": {
				Model:        "helixagent-debate",
				Provider:     "helixagent",
				SystemPrompt: "You are HelixAgent, an AI ensemble that combines the intelligence of multiple top-performing LLMs through debate and consensus. You have full access to MCP, ACP, LSP protocols, embeddings, vision/OCR, and all generative capabilities.",
				Description:  "HelixAgent AI Debate Ensemble - 15 verified LLMs with protocol support",
				Temperature:  0.7,
				MaxTokens:    8192,
				Mode:         "code",
				Tools: map[string]bool{
					"read":       true,
					"write":      true,
					"edit":       true,
					"bash":       true,
					"glob":       true,
					"grep":       true,
					"webfetch":   true,
					"mcp":        true,
					"embeddings": true,
					"vision":     true,
				},
			},
			"code-reviewer": {
				Model:        "helixagent-debate",
				Provider:     "helixagent",
				SystemPrompt: "You are an expert code reviewer with access to LSP protocol for code intelligence. Analyze code for bugs, security issues, performance problems, and improvements.",
				Description:  "Code review specialist with LSP support",
				Temperature:  0.3,
				Mode:         "review",
				Tools: map[string]bool{
					"read": true,
					"lsp":  true,
					"grep": true,
				},
			},
			"embeddings-specialist": {
				Model:        "helixagent-debate",
				Provider:     "helixagent",
				SystemPrompt: "You are an embeddings specialist. Generate and work with vector embeddings for semantic search and similarity matching.",
				Description:  "Embeddings and semantic search specialist",
				Temperature:  0.5,
				Mode:         "chat",
				Tools: map[string]bool{
					"read":       true,
					"embeddings": true,
				},
			},
			"vision-analyst": {
				Model:        "helixagent-debate",
				Provider:     "helixagent",
				SystemPrompt: "You are a vision analysis specialist. Analyze images, perform OCR, and understand visual content in detail.",
				Description:  "Vision and OCR analysis specialist",
				Temperature:  0.5,
				Mode:         "chat",
				Tools: map[string]bool{
					"read":   true,
					"vision": true,
				},
			},
		},
		MCP: map[string]KiloMCP{
			// HelixAgent protocol endpoints
			"helixagent-mcp": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/mcp", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-acp": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/acp", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-lsp": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/lsp", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-embeddings": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/embeddings", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-vision": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/vision", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			"helixagent-cognee": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/cognee", host, port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{
					"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY"),
				},
			},
			// Standard MCP servers
			"filesystem": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "/"},
				Enabled: true,
			},
			"github": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"},
				Enabled: true,
				Environment: map[string]string{
					"GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN"),
				},
			},
			"memory": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-memory"},
				Enabled: true,
			},
			"fetch": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-fetch"},
				Enabled: true,
			},
			"puppeteer": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-puppeteer"},
				Enabled: true,
			},
			"sqlite": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-sqlite"},
				Enabled: true,
			},
		},
		Tools: map[string]bool{
			"Read":       true,
			"Write":      true,
			"Edit":       true,
			"Bash":       true,
			"Glob":       true,
			"Grep":       true,
			"WebFetch":   true,
			"Task":       true,
			"TodoWrite":  true,
			"MCP":        true,
			"Embeddings": true,
			"Vision":     true,
		},
		Settings: KiloSettings{
			DefaultProvider:   "helixagent",
			DefaultModel:      "helixagent-debate",
			DefaultAgent:      "default",
			StreamingEnabled:  true,
			AutoSave:          true,
			CompactionEnabled: true,
			PruneEnabled:      true,
			Theme:             "default",
			Language:          "en",
			TelemetryEnabled:  false,
		},
		Permissions: KiloPermissions{
			Read:     "allow",
			Write:    "ask",
			Edit:     "ask",
			Bash:     "ask",
			WebFetch: "allow",
			MCP:      "allow",
		},
		Shortcuts: map[string]string{
			"submit":     "ctrl+enter",
			"cancel":     "escape",
			"clear":      "ctrl+l",
			"newConvo":   "ctrl+shift+n",
			"switchMode": "ctrl+shift+m",
		},
	}

	return config
}

// ValidateConfig validates a Kilo Code configuration
func ValidateConfig(config *KiloCodeConfig) *ValidationResult {
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
	validTypes := map[string]bool{"openai-compatible": true, "anthropic": true, "azure": true, "google": true}
	for name, provider := range config.Providers {
		if !validTypes[provider.Type] {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("providers.%s.type", name),
				Message: fmt.Sprintf("invalid type '%s', must be one of: openai-compatible, anthropic, azure, google", provider.Type),
			})
		}

		if provider.Type == "openai-compatible" && provider.BaseURL == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("providers.%s.baseUrl", name),
				Message: "baseUrl is required for openai-compatible type",
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

	// Validate agents
	for name, agent := range config.Agents {
		if agent.Model == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("agents.%s.model", name),
				Message: "agent model is required",
			})
		}
	}

	// Validate MCP servers
	validMCPTypes := map[string]bool{"local": true, "remote": true, "stdio": true}
	for name, mcp := range config.MCP {
		if !validMCPTypes[mcp.Type] {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp.%s.type", name),
				Message: fmt.Sprintf("invalid type '%s', must be one of: local, remote, stdio", mcp.Type),
			})
		}

		if (mcp.Type == "local" || mcp.Type == "stdio") && len(mcp.Command) == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp.%s.command", name),
				Message: fmt.Sprintf("command is required for %s type", mcp.Type),
			})
		}

		if mcp.Type == "remote" && mcp.URL == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mcp.%s.url", name),
				Message: "url is required for remote type",
			})
		}
	}

	// Validate permissions
	validPerms := map[string]bool{"allow": true, "ask": true, "deny": true}
	if config.Permissions.Read != "" && !validPerms[config.Permissions.Read] {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "permissions.read",
			Message: "must be one of: allow, ask, deny",
		})
	}
	if config.Permissions.Write != "" && !validPerms[config.Permissions.Write] {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "permissions.write",
			Message: "must be one of: allow, ask, deny",
		})
	}
	if config.Permissions.Bash != "" && !validPerms[config.Permissions.Bash] {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "permissions.bash",
			Message: "must be one of: allow, ask, deny",
		})
	}

	return result
}

// SaveConfig saves configuration to file
func SaveConfig(config *KiloCodeConfig, path string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// LoadAndValidateConfig loads and validates an existing config file
func LoadAndValidateConfig(path string) (*KiloCodeConfig, *ValidationResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config KiloCodeConfig
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
		fmt.Println("KILO CODE CONFIGURATION VALIDATION")
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
			fmt.Printf("Agents configured: %d\n", len(config.Agents))
			fmt.Printf("MCP servers configured: %d\n", len(config.MCP))
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
		outputPath = filepath.Join(homeDir, "Downloads", "kilocode-helix-agent.json")
	}

	fmt.Println("=" + repeatString("=", 69))
	fmt.Println("HELIXAGENT KILO CODE CONFIGURATION GENERATOR")
	fmt.Println("=" + repeatString("=", 69))
	fmt.Println()

	// Generate config
	config := GenerateHelixAgentKiloConfig(host, port, nil)

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
	fmt.Printf("Schema: https://kilocode.dev/config-schema.json\n")
	fmt.Printf("Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()
	fmt.Println("Configuration includes:")
	fmt.Printf("  - Providers: %d\n", len(config.Providers))
	fmt.Printf("  - Agents: %d\n", len(config.Agents))
	fmt.Printf("  - MCP servers: %d\n", len(config.MCP))
	fmt.Printf("  - Tools: %d\n", len(config.Tools))
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
