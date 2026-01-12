package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CLIAgentType represents supported CLI agent types
type CLIAgentType string

const (
	OpenCodeAgent  CLIAgentType = "opencode"
	CrushAgent     CLIAgentType = "crush"
	KiloCodeAgent  CLIAgentType = "kilocode"
	HelixCodeAgent CLIAgentType = "helixcode"
)

// AllAgents returns all supported agent types
func AllAgents() []CLIAgentType {
	return []CLIAgentType{OpenCodeAgent, CrushAgent, KiloCodeAgent, HelixCodeAgent}
}

// DebateGroupMember represents a member of the AI debate group
type DebateGroupMember struct {
	Provider  string   `json:"provider"`
	Model     string   `json:"model"`
	Score     float64  `json:"score"`
	Fallbacks []string `json:"fallbacks,omitempty"`
}

// GenerationResult holds the result of config generation
type GenerationResult struct {
	AgentType   CLIAgentType `json:"agent_type"`
	OutputPath  string       `json:"output_path"`
	Success     bool         `json:"success"`
	Error       string       `json:"error,omitempty"`
	ConfigStats ConfigStats  `json:"stats,omitempty"`
}

// ConfigStats holds statistics about generated config
type ConfigStats struct {
	Providers   int `json:"providers"`
	Agents      int `json:"agents,omitempty"`
	MCPServers  int `json:"mcp_servers,omitempty"`
	LSPServers  int `json:"lsp_servers,omitempty"`
	Tools       int `json:"tools,omitempty"`
}

// UnifiedGenerator generates configurations for all CLI agents
type UnifiedGenerator struct {
	host          string
	port          int
	outputDir     string
	debateMembers []DebateGroupMember
}

// NewUnifiedGenerator creates a new unified generator
func NewUnifiedGenerator(host string, port int, outputDir string) *UnifiedGenerator {
	return &UnifiedGenerator{
		host:      host,
		port:      port,
		outputDir: outputDir,
	}
}

// SetDebateMembers sets the debate group members for fallback configuration
func (g *UnifiedGenerator) SetDebateMembers(members []DebateGroupMember) {
	g.debateMembers = members
}

// GenerateAll generates configurations for all supported CLI agents
func (g *UnifiedGenerator) GenerateAll() []GenerationResult {
	var results []GenerationResult

	for _, agentType := range AllAgents() {
		result := g.Generate(agentType)
		results = append(results, result)
	}

	return results
}

// Generate generates configuration for a specific CLI agent
func (g *UnifiedGenerator) Generate(agentType CLIAgentType) GenerationResult {
	result := GenerationResult{AgentType: agentType}

	var config interface{}
	var filename string
	var stats ConfigStats

	switch agentType {
	case OpenCodeAgent:
		cfg := g.generateOpenCodeConfig()
		config = cfg
		filename = "opencode-helix-agent.json"
		stats = ConfigStats{
			Providers:  len(cfg.Provider),
			Agents:     len(cfg.Agent),
			MCPServers: len(cfg.Mcp),
			Tools:      len(cfg.Tools),
		}

	case CrushAgent:
		cfg := g.generateCrushConfig()
		config = cfg
		filename = "crush-helix-agent.json"
		stats = ConfigStats{
			Providers:  len(cfg.Providers),
			MCPServers: len(cfg.MCP),
			LSPServers: len(cfg.LSP),
		}

	case KiloCodeAgent:
		cfg := g.generateKiloCodeConfig()
		config = cfg
		filename = "kilocode-helix-agent.json"
		stats = ConfigStats{
			Providers:  len(cfg.Providers),
			Agents:     len(cfg.Agents),
			MCPServers: len(cfg.MCP),
			Tools:      len(cfg.Tools),
		}

	case HelixCodeAgent:
		cfg := g.generateHelixCodeConfig()
		config = cfg
		filename = "helixcode-helix-agent.json"
		stats = ConfigStats{
			Providers: len(cfg.Providers),
		}

	default:
		result.Success = false
		result.Error = fmt.Sprintf("unsupported agent type: %s", agentType)
		return result
	}

	// Create output directory if needed
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to create output directory: %v", err)
		return result
	}

	// Marshal and save
	outputPath := filepath.Join(g.outputDir, filename)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to marshal config: %v", err)
		return result
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to write config: %v", err)
		return result
	}

	result.Success = true
	result.OutputPath = outputPath
	result.ConfigStats = stats
	return result
}

// ========================
// OpenCode Configuration
// ========================

type OpenCodeConfig struct {
	Schema       string                        `json:"$schema,omitempty"`
	Plugin       []string                      `json:"plugin,omitempty"`
	Instructions []string                      `json:"instructions,omitempty"`
	Provider     map[string]OpenCodeProvider   `json:"provider,omitempty"`
	Mcp          map[string]OpenCodeMCP        `json:"mcp,omitempty"`
	Tools        map[string]bool               `json:"tools,omitempty"`
	Agent        map[string]OpenCodeAgentConfig      `json:"agent,omitempty"`
	Permission   map[string]string             `json:"permission,omitempty"`
	Username     string                        `json:"username,omitempty"`
}

type OpenCodeProvider struct {
	Npm     string                       `json:"npm,omitempty"`
	Name    string                       `json:"name,omitempty"`
	Options map[string]interface{}       `json:"options,omitempty"`
	Models  map[string]OpenCodeModel     `json:"models,omitempty"`
}

type OpenCodeModel struct {
	Name          string   `json:"name,omitempty"`
	MaxTokens     int      `json:"maxTokens,omitempty"`
	Attachments   bool     `json:"attachments,omitempty"`
	Reasoning     bool     `json:"reasoning,omitempty"`
	Vision        bool     `json:"vision,omitempty"`
	Streaming     bool     `json:"streaming,omitempty"`
	FunctionCalls bool     `json:"functionCalls,omitempty"`
	Embeddings    bool     `json:"embeddings,omitempty"`
	MCP           bool     `json:"mcp,omitempty"`
	ACP           bool     `json:"acp,omitempty"`
	LSP           bool     `json:"lsp,omitempty"`
	Fallbacks     []string `json:"fallbacks,omitempty"`
}

type OpenCodeMCP struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Timeout     *int              `json:"timeout,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

type OpenCodeAgentConfig struct {
	Model       string          `json:"model,omitempty"`
	Prompt      string          `json:"prompt,omitempty"`
	Description string          `json:"description,omitempty"`
	Tools       map[string]bool `json:"tools,omitempty"`
}

func (g *UnifiedGenerator) generateOpenCodeConfig() *OpenCodeConfig {
	baseURL := fmt.Sprintf("http://%s:%d/v1", g.host, g.port)
	enabled := true
	timeout := 120

	var fallbacks []string
	for _, member := range g.debateMembers {
		fallbacks = append(fallbacks, fmt.Sprintf("%s/%s", member.Provider, member.Model))
	}

	return &OpenCodeConfig{
		Schema:   "https://opencode.ai/config.json",
		Username: "HelixAgent AI Ensemble",
		Instructions: []string{
			"You are connected to HelixAgent, a Virtual LLM Provider that exposes ONE model backed by an AI debate ensemble.",
			"The helixagent-debate model combines responses from multiple top-performing LLMs through confidence-weighted voting.",
		},
		Provider: map[string]OpenCodeProvider{
			"helixagent": {
				Npm:  "@ai-sdk/openai-compatible",
				Name: "HelixAgent AI Debate Ensemble",
				Options: map[string]interface{}{
					"apiKey":  os.Getenv("HELIXAGENT_API_KEY"),
					"baseURL": baseURL,
					"timeout": 600000,
				},
				Models: map[string]OpenCodeModel{
					"helixagent-debate": {
						Name:          "HelixAgent Debate Ensemble",
						MaxTokens:     128000,
						Attachments:   true,
						Reasoning:     true,
						Vision:        true,
						Streaming:     true,
						FunctionCalls: true,
						Embeddings:    true,
						MCP:           true,
						ACP:           true,
						LSP:           true,
						Fallbacks:     fallbacks,
					},
				},
			},
		},
		Mcp: map[string]OpenCodeMCP{
			"helixagent-mcp": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/mcp", g.host, g.port),
				Enabled: &enabled,
				Timeout: &timeout,
				Headers: map[string]string{"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY")},
			},
			"helixagent-embeddings": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/embeddings", g.host, g.port),
				Enabled: &enabled,
				Timeout: &timeout,
				Headers: map[string]string{"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY")},
			},
			"filesystem": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "/"},
				Enabled: &enabled,
			},
			"github": {
				Type:        "local",
				Command:     []string{"npx", "-y", "@modelcontextprotocol/server-github"},
				Enabled:     &enabled,
				Environment: map[string]string{"GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN")},
			},
		},
		Agent: map[string]OpenCodeAgentConfig{
			"default": {
				Model:       "helixagent/helixagent-debate",
				Prompt:      "You are HelixAgent, an AI ensemble combining multiple LLMs through debate and consensus.",
				Description: "HelixAgent AI Debate Ensemble",
				Tools: map[string]bool{
					"read": true, "write": true, "bash": true, "glob": true, "grep": true,
					"webfetch": true, "edit": true, "mcp": true, "embeddings": true,
				},
			},
		},
		Tools: map[string]bool{
			"Read": true, "Write": true, "Bash": true, "Glob": true,
			"Grep": true, "Edit": true, "WebFetch": true, "Task": true,
		},
		Permission: map[string]string{
			"read": "allow", "edit": "ask", "bash": "ask", "webfetch": "allow",
		},
	}
}

// ========================
// Crush Configuration
// ========================

type CrushConfig struct {
	Schema    string                   `json:"$schema,omitempty"`
	Providers map[string]CrushProvider `json:"providers"`
	MCP       map[string]CrushMCP      `json:"mcp,omitempty"`
	LSP       map[string]CrushLSP      `json:"lsp,omitempty"`
	Options   CrushOptions             `json:"options,omitempty"`
}

type CrushProvider struct {
	Name    string       `json:"name,omitempty"`
	Type    string       `json:"type"`
	BaseURL string       `json:"base_url,omitempty"`
	APIKey  string       `json:"api_key,omitempty"`
	Models  []CrushModel `json:"models,omitempty"`
	Timeout int          `json:"timeout,omitempty"`
}

type CrushModel struct {
	ID               string   `json:"id"`
	Name             string   `json:"name,omitempty"`
	ContextWindow    int      `json:"context_window,omitempty"`
	DefaultMaxTokens int      `json:"default_max_tokens,omitempty"`
	Fallbacks        []string `json:"fallbacks,omitempty"`
}

type CrushMCP struct {
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Timeout int               `json:"timeout,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
}

type CrushLSP struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Enabled bool     `json:"enabled"`
}

type CrushOptions struct {
	DefaultProvider           string `json:"default_provider,omitempty"`
	StreamingEnabled          bool   `json:"streaming_enabled,omitempty"`
	DisableProviderAutoUpdate bool   `json:"disable_provider_auto_update,omitempty"`
}

func (g *UnifiedGenerator) generateCrushConfig() *CrushConfig {
	baseURL := fmt.Sprintf("http://%s:%d/v1", g.host, g.port)

	var fallbacks []string
	for _, member := range g.debateMembers {
		fallbacks = append(fallbacks, fmt.Sprintf("%s/%s", member.Provider, member.Model))
	}

	return &CrushConfig{
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
						ID:               "helixagent-debate",
						Name:             "HelixAgent AI Debate Ensemble",
						ContextWindow:    128000,
						DefaultMaxTokens: 8192,
						Fallbacks:        fallbacks,
					},
				},
			},
		},
		MCP: map[string]CrushMCP{
			"helixagent-mcp": {
				Type:    "http",
				URL:     fmt.Sprintf("http://%s:%d/v1/mcp", g.host, g.port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY")},
			},
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
		},
		LSP: map[string]CrushLSP{
			"go":         {Command: "gopls", Enabled: true},
			"typescript": {Command: "typescript-language-server", Args: []string{"--stdio"}, Enabled: true},
			"python":     {Command: "pylsp", Enabled: true},
			"rust":       {Command: "rust-analyzer", Enabled: true},
		},
		Options: CrushOptions{
			DefaultProvider:  "helixagent",
			StreamingEnabled: true,
		},
	}
}

// ========================
// Kilo Code Configuration
// ========================

type KiloCodeConfig struct {
	Schema      string                  `json:"$schema,omitempty"`
	Version     string                  `json:"version,omitempty"`
	Providers   map[string]KiloProvider `json:"providers"`
	Agents      map[string]KiloAgent    `json:"agents,omitempty"`
	MCP         map[string]KiloMCP      `json:"mcp,omitempty"`
	Tools       map[string]bool         `json:"tools,omitempty"`
	Settings    KiloSettings            `json:"settings,omitempty"`
	Permissions KiloPermissions         `json:"permissions,omitempty"`
}

type KiloProvider struct {
	Type    string      `json:"type"`
	Name    string      `json:"name,omitempty"`
	BaseURL string      `json:"baseUrl,omitempty"`
	APIKey  string      `json:"apiKey,omitempty"`
	Models  []KiloModel `json:"models,omitempty"`
	Timeout int         `json:"timeout,omitempty"`
}

type KiloModel struct {
	ID            string            `json:"id"`
	Name          string            `json:"name,omitempty"`
	MaxTokens     int               `json:"maxTokens,omitempty"`
	ContextWindow int               `json:"contextWindow,omitempty"`
	Capabilities  KiloCapabilities  `json:"capabilities,omitempty"`
	Fallbacks     []string          `json:"fallbacks,omitempty"`
}

type KiloCapabilities struct {
	Vision        bool `json:"vision,omitempty"`
	Streaming     bool `json:"streaming,omitempty"`
	FunctionCalls bool `json:"functionCalls,omitempty"`
	Embeddings    bool `json:"embeddings,omitempty"`
	MCP           bool `json:"mcp,omitempty"`
}

type KiloAgent struct {
	Model        string          `json:"model"`
	Provider     string          `json:"provider,omitempty"`
	SystemPrompt string          `json:"systemPrompt,omitempty"`
	Description  string          `json:"description,omitempty"`
	Tools        map[string]bool `json:"tools,omitempty"`
}

type KiloMCP struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Enabled     bool              `json:"enabled"`
	Timeout     int               `json:"timeout,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

type KiloSettings struct {
	DefaultProvider  string `json:"defaultProvider,omitempty"`
	DefaultModel     string `json:"defaultModel,omitempty"`
	StreamingEnabled bool   `json:"streamingEnabled,omitempty"`
	AutoSave         bool   `json:"autoSave,omitempty"`
}

type KiloPermissions struct {
	Read     string `json:"read,omitempty"`
	Write    string `json:"write,omitempty"`
	Bash     string `json:"bash,omitempty"`
	WebFetch string `json:"webFetch,omitempty"`
}

func (g *UnifiedGenerator) generateKiloCodeConfig() *KiloCodeConfig {
	baseURL := fmt.Sprintf("http://%s:%d/v1", g.host, g.port)

	var fallbacks []string
	for _, member := range g.debateMembers {
		fallbacks = append(fallbacks, fmt.Sprintf("%s/%s", member.Provider, member.Model))
	}

	return &KiloCodeConfig{
		Schema:  "https://kilocode.dev/config-schema.json",
		Version: "1.0",
		Providers: map[string]KiloProvider{
			"helixagent": {
				Type:    "openai-compatible",
				Name:    "HelixAgent AI Debate Ensemble",
				BaseURL: baseURL,
				APIKey:  os.Getenv("HELIXAGENT_API_KEY"),
				Timeout: 600000,
				Models: []KiloModel{
					{
						ID:            "helixagent-debate",
						Name:          "HelixAgent AI Debate Ensemble",
						MaxTokens:     8192,
						ContextWindow: 128000,
						Capabilities: KiloCapabilities{
							Vision:        true,
							Streaming:     true,
							FunctionCalls: true,
							Embeddings:    true,
							MCP:           true,
						},
						Fallbacks: fallbacks,
					},
				},
			},
		},
		Agents: map[string]KiloAgent{
			"default": {
				Model:        "helixagent-debate",
				Provider:     "helixagent",
				SystemPrompt: "You are HelixAgent, an AI ensemble combining multiple LLMs through debate and consensus.",
				Description:  "HelixAgent AI Debate Ensemble",
				Tools: map[string]bool{
					"read": true, "write": true, "bash": true, "glob": true,
					"grep": true, "edit": true, "webfetch": true, "mcp": true,
				},
			},
		},
		MCP: map[string]KiloMCP{
			"helixagent-mcp": {
				Type:    "remote",
				URL:     fmt.Sprintf("http://%s:%d/v1/mcp", g.host, g.port),
				Enabled: true,
				Timeout: 120,
				Headers: map[string]string{"Authorization": "Bearer " + os.Getenv("HELIXAGENT_API_KEY")},
			},
			"filesystem": {
				Type:    "local",
				Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "/"},
				Enabled: true,
			},
		},
		Tools: map[string]bool{
			"Read": true, "Write": true, "Bash": true, "Glob": true,
			"Grep": true, "Edit": true, "WebFetch": true, "Task": true,
		},
		Settings: KiloSettings{
			DefaultProvider:  "helixagent",
			DefaultModel:     "helixagent-debate",
			StreamingEnabled: true,
			AutoSave:         true,
		},
		Permissions: KiloPermissions{
			Read: "allow", Write: "ask", Bash: "ask", WebFetch: "allow",
		},
	}
}

// ========================
// HelixCode Configuration
// ========================

type HelixCodeConfig struct {
	Schema    string                      `json:"$schema,omitempty"`
	Providers map[string]HelixCodeProvider `json:"providers"`
	Settings  HelixCodeSettings           `json:"settings,omitempty"`
}

type HelixCodeProvider struct {
	Type      string `json:"type"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key,omitempty"`
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
}

type HelixCodeSettings struct {
	DefaultProvider  string `json:"default_provider,omitempty"`
	StreamingEnabled bool   `json:"streaming_enabled,omitempty"`
	AutoSave         bool   `json:"auto_save,omitempty"`
}

func (g *UnifiedGenerator) generateHelixCodeConfig() *HelixCodeConfig {
	baseURL := fmt.Sprintf("http://%s:%d/v1", g.host, g.port)

	return &HelixCodeConfig{
		Providers: map[string]HelixCodeProvider{
			"helixagent": {
				Type:      "openai-compatible",
				BaseURL:   baseURL,
				APIKey:    os.Getenv("HELIXAGENT_API_KEY"),
				Model:     "helixagent-debate",
				MaxTokens: 8192,
				Timeout:   120,
			},
		},
		Settings: HelixCodeSettings{
			DefaultProvider:  "helixagent",
			StreamingEnabled: true,
			AutoSave:         true,
		},
	}
}

// ========================
// Main
// ========================

func main() {
	var (
		host       string
		port       int
		outputDir  string
		agentType  string
		listAgents bool
	)

	flag.StringVar(&host, "host", "localhost", "HelixAgent host")
	flag.IntVar(&port, "port", 7061, "HelixAgent port")
	flag.StringVar(&outputDir, "output-dir", "", "Output directory for generated configs")
	flag.StringVar(&agentType, "agent", "all", "Agent type to generate (opencode, crush, kilocode, helixcode, or all)")
	flag.BoolVar(&listAgents, "list", false, "List supported agent types")
	flag.Parse()

	if listAgents {
		fmt.Println("Supported CLI Agent Types:")
		fmt.Println("  - opencode   : OpenCode AI (https://opencode.ai)")
		fmt.Println("  - crush      : Crush by Charm (https://charm.land/crush)")
		fmt.Println("  - kilocode   : Kilo Code (VS Code extension)")
		fmt.Println("  - helixcode  : HelixCode (HelixAgent native)")
		fmt.Println("  - all        : Generate all configurations")
		return
	}

	if outputDir == "" {
		homeDir, _ := os.UserHomeDir()
		outputDir = filepath.Join(homeDir, "Downloads", "helixagent-cli-configs")
	}

	fmt.Println("=" + repeatString("=", 69))
	fmt.Println("HELIXAGENT UNIFIED CLI AGENT CONFIGURATION GENERATOR")
	fmt.Println("=" + repeatString("=", 69))
	fmt.Println()

	generator := NewUnifiedGenerator(host, port, outputDir)

	var results []GenerationResult

	if agentType == "all" {
		fmt.Println("Generating configurations for ALL supported CLI agents...")
		fmt.Println()
		results = generator.GenerateAll()
	} else {
		fmt.Printf("Generating configuration for: %s\n", agentType)
		fmt.Println()
		result := generator.Generate(CLIAgentType(agentType))
		results = []GenerationResult{result}
	}

	// Print results
	fmt.Println("Results:")
	fmt.Println("-" + repeatString("-", 68))

	successCount := 0
	for _, result := range results {
		status := "PASS"
		if !result.Success {
			status = "FAIL"
		} else {
			successCount++
		}

		fmt.Printf("[%s] %s\n", status, result.AgentType)
		if result.Success {
			fmt.Printf("      Output: %s\n", result.OutputPath)
			fmt.Printf("      Stats: providers=%d", result.ConfigStats.Providers)
			if result.ConfigStats.Agents > 0 {
				fmt.Printf(", agents=%d", result.ConfigStats.Agents)
			}
			if result.ConfigStats.MCPServers > 0 {
				fmt.Printf(", mcp=%d", result.ConfigStats.MCPServers)
			}
			if result.ConfigStats.LSPServers > 0 {
				fmt.Printf(", lsp=%d", result.ConfigStats.LSPServers)
			}
			if result.ConfigStats.Tools > 0 {
				fmt.Printf(", tools=%d", result.ConfigStats.Tools)
			}
			fmt.Println()
		} else {
			fmt.Printf("      Error: %s\n", result.Error)
		}
	}

	fmt.Println("-" + repeatString("-", 68))
	fmt.Printf("Generated: %d/%d configurations\n", successCount, len(results))
	fmt.Printf("Output directory: %s\n", outputDir)
	fmt.Printf("Timestamp: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()
	fmt.Println("=" + repeatString("=", 69))

	// Exit with error code if any failed
	if successCount != len(results) {
		os.Exit(1)
	}
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
