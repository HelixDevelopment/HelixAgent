package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"digital.vasic.llmsverifier/pkg/cliagents"
)

// DebateGroupMember represents a member of the AI debate group
type DebateGroupMember struct {
	Provider  string   `json:"provider"`
	Model     string   `json:"model"`
	Score     float64  `json:"score"`
	Fallbacks []string `json:"fallbacks,omitempty"`
}

// UnifiedCLIGenerator wraps LLMsVerifier's unified generator with HelixAgent-specific functionality
type UnifiedCLIGenerator struct {
	generator     *cliagents.UnifiedGenerator
	host          string
	port          int
	outputDir     string
	debateMembers []DebateGroupMember
}

// NewUnifiedCLIGenerator creates a new unified generator using LLMsVerifier
func NewUnifiedCLIGenerator(host string, port int, outputDir string) *UnifiedCLIGenerator {
	config := &cliagents.GeneratorConfig{
		HelixAgentHost: host,
		HelixAgentPort: port,
		OutputDir:      outputDir,
		IncludeScores:  true,
		// ForHost, not DefaultMCPServers(): the no-argument wrapper is
		// literally DefaultMCPServersForHost("localhost", 8100), so it ignored
		// this constructor's own host/port arguments and emitted the nine
		// HelixAgent service URLs on a hardcoded port — the same defect the
		// shipped binary's generators carried, in the one place a -host/-port
		// flag made it look configurable.
		MCPServers: cliagents.DefaultMCPServersForHost(host, port),
	}

	return &UnifiedCLIGenerator{
		generator: cliagents.NewUnifiedGenerator(config),
		host:      host,
		port:      port,
		outputDir: outputDir,
	}
}

// SetDebateMembers sets the debate group members for fallback configuration
func (g *UnifiedCLIGenerator) SetDebateMembers(members []DebateGroupMember) {
	g.debateMembers = members
}

// GenerationResult holds the result of config generation
type GenerationResult struct {
	AgentType   cliagents.AgentType `json:"agent_type"`
	OutputPath  string              `json:"output_path"`
	Success     bool                `json:"success"`
	Error       string              `json:"error,omitempty"`
	ConfigStats ConfigStats         `json:"stats,omitempty"`
	Validated   bool                `json:"validated,omitempty"`
	Warnings    []string            `json:"warnings,omitempty"`
}

// ConfigStats holds statistics about generated config
type ConfigStats struct {
	Providers  int `json:"providers"`
	Agents     int `json:"agents,omitempty"`
	MCPServers int `json:"mcp_servers,omitempty"`
	LSPServers int `json:"lsp_servers,omitempty"`
	Tools      int `json:"tools,omitempty"`
}

// GenerateAll generates configurations for all supported CLI agents
func (g *UnifiedCLIGenerator) GenerateAll() []GenerationResult {
	ctx := context.Background()
	results, _ := g.generator.GenerateAll(ctx)

	var genResults []GenerationResult
	for _, result := range results {
		genResult := g.convertResult(result)
		if genResult.Success {
			// Save the config
			if err := g.saveConfig(result); err != nil {
				genResult.Success = false
				genResult.Error = fmt.Sprintf("failed to save config: %v", err)
			}
		}
		genResults = append(genResults, genResult)
	}

	return genResults
}

// Generate generates configuration for a specific CLI agent
func (g *UnifiedCLIGenerator) Generate(agentType cliagents.AgentType) GenerationResult {
	ctx := context.Background()
	result, err := g.generator.Generate(ctx, agentType)
	if err != nil {
		return GenerationResult{
			AgentType: agentType,
			Success:   false,
			Error:     err.Error(),
		}
	}

	genResult := g.convertResult(result)
	if genResult.Success {
		// Save the config
		if err := g.saveConfig(result); err != nil {
			genResult.Success = false
			genResult.Error = fmt.Sprintf("failed to save config: %v", err)
		}
	}

	return genResult
}

// convertResult converts LLMsVerifier result to our format
func (g *UnifiedCLIGenerator) convertResult(result *cliagents.GenerationResult) GenerationResult {
	genResult := GenerationResult{
		AgentType: result.AgentType,
		Success:   result.Success,
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			genResult.Error = result.Errors[0]
		}
		return genResult
	}

	// Extract stats based on config type
	genResult.ConfigStats = g.extractStats(result)

	// Check validation
	if result.ValidationResult != nil {
		genResult.Validated = result.ValidationResult.Valid
		genResult.Warnings = result.ValidationResult.Warnings
		if !result.ValidationResult.Valid && len(result.ValidationResult.Errors) > 0 {
			genResult.Error = result.ValidationResult.Errors[0]
			genResult.Success = false
		}
	}

	return genResult
}

// extractStats extracts configuration statistics from the result
func (g *UnifiedCLIGenerator) extractStats(result *cliagents.GenerationResult) ConfigStats {
	stats := ConfigStats{
		Providers: 1, // We always have the helixagent provider
	}

	switch cfg := result.Config.(type) {
	case *cliagents.OpenCodeConfig:
		stats.MCPServers = len(cfg.MCPServers)
		stats.Agents = len(cfg.Agent)
		stats.Tools = len(cfg.Tools)
	case *cliagents.CrushConfig:
		stats.MCPServers = len(cfg.MCP)
		stats.Agents = 0
	case *cliagents.KiloCodeConfig:
		stats.MCPServers = len(cfg.MCP)
		stats.Agents = len(cfg.Agents)
	case *cliagents.HelixCodeConfig:
		stats.MCPServers = len(cfg.MCP)
		stats.Agents = len(cfg.Agents)
		stats.Tools = countHelixCodeTools(cfg.Tools)
	case *cliagents.GenericAgentConfig:
		stats.MCPServers = len(cfg.MCP)
	}

	return stats
}

// countHelixCodeTools counts enabled tools
func countHelixCodeTools(tools cliagents.HelixCodeTools) int {
	count := 0
	if tools.FileSystem {
		count++
	}
	if tools.Terminal {
		count++
	}
	if tools.Browser {
		count++
	}
	if tools.Search {
		count++
	}
	if tools.Git {
		count++
	}
	if tools.MCP {
		count++
	}
	if tools.LSP {
		count++
	}
	if tools.Embeddings {
		count++
	}
	if tools.Vision {
		count++
	}
	return count
}

// saveConfig saves the configuration to disk
func (g *UnifiedCLIGenerator) saveConfig(result *cliagents.GenerationResult) error {
	if result.Config == nil {
		return fmt.Errorf("no configuration to save")
	}

	// Create output directory if needed
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Get the schema for filename extension
	schema, _ := g.generator.GetSchema(result.AgentType)

	// Always use unique filename based on agent type to prevent overwrites
	// Format: {agent-type}-helixagent.{extension}
	ext := ".json"
	if schema != nil && schema.ConfigFileName != "" {
		// Get extension from schema config file name
		if filepath.Ext(schema.ConfigFileName) != "" {
			ext = filepath.Ext(schema.ConfigFileName)
		}
	}
	filename := fmt.Sprintf("%s-helixagent%s", result.AgentType, ext)

	outputPath := filepath.Join(g.outputDir, filename)

	// Marshal and save
	data, err := json.MarshalIndent(result.Config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	result.ConfigPath = outputPath
	return nil
}

// ListSupportedAgents returns all supported agent types
func (g *UnifiedCLIGenerator) ListSupportedAgents() []cliagents.AgentType {
	return g.generator.ListSupportedAgents()
}

// GetAllSchemas returns schemas for all agents
func (g *UnifiedCLIGenerator) GetAllSchemas() map[cliagents.AgentType]*cliagents.AgentSchema {
	return g.generator.GetAllSchemas()
}

// Validate validates a configuration for a specific agent
func (g *UnifiedCLIGenerator) Validate(agentType cliagents.AgentType, config any) (*cliagents.ValidationResult, error) {
	return g.generator.Validate(agentType, config)
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
		validate   string
	)

	flag.StringVar(&host, "host", "localhost", "HelixAgent host")
	flag.IntVar(&port, "port", 7061, "HelixAgent port")
	flag.StringVar(&outputDir, "output-dir", "", "Output directory for generated configs")
	flag.StringVar(&agentType, "agent", "all", "Agent type to generate (opencode, crush, kilocode, helixcode, aider, continue, cursor, cline, windsurf, zed, neovim-ai, vscode-ai, intellij-ai, claude-code, qwen-code, github-copilot, or all)")
	flag.BoolVar(&listAgents, "list", false, "List supported agent types and their schemas")
	flag.StringVar(&validate, "validate", "", "Validate existing config file")
	flag.Parse()

	if listAgents {
		fmt.Println("Supported CLI Agent Types (via LLMsVerifier):")
		fmt.Println("-" + repeatString("-", 68))

		generator := NewUnifiedCLIGenerator(host, port, "")
		schemas := generator.GetAllSchemas()

		for agentType, schema := range schemas {
			fmt.Printf("  %-15s : %s\n", agentType, schema.Description)
			fmt.Printf("                  Config: %s\n", schema.ConfigFileName)
			fmt.Printf("                  Dir: %s\n", schema.DefaultConfigDir)
			fmt.Println()
		}

		fmt.Printf("\nTotal: %d CLI agents supported\n", len(schemas))
		return
	}

	if validate != "" {
		fmt.Printf("Validating config file: %s\n", validate)
		validateConfigFile(validate)
		return
	}

	if outputDir == "" {
		homeDir, _ := os.UserHomeDir()
		outputDir = filepath.Join(homeDir, "Downloads", "helixagent-cli-configs")
	}

	fmt.Println("=" + repeatString("=", 69))
	fmt.Println("HELIXAGENT UNIFIED CLI AGENT CONFIGURATION GENERATOR")
	fmt.Println("  Powered by LLMsVerifier")
	fmt.Println("=" + repeatString("=", 69))
	fmt.Println()

	generator := NewUnifiedCLIGenerator(host, port, outputDir)

	var results []GenerationResult

	if agentType == "all" {
		fmt.Println("Generating configurations for ALL 16 supported CLI agents...")
		fmt.Println()
		results = generator.GenerateAll()
	} else {
		fmt.Printf("Generating configuration for: %s\n", agentType)
		fmt.Println()
		result := generator.Generate(cliagents.AgentType(agentType))
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
			if result.Validated {
				fmt.Printf("      Validated: %s\n", "PASS")
			}
			if len(result.Warnings) > 0 {
				fmt.Printf("      Warnings: %v\n", result.Warnings)
			}
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

// validateConfigFile validates an existing configuration file
func validateConfigFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	// Try to detect agent type and validate
	generator := cliagents.NewUnifiedGenerator(nil)

	// Try each agent type
	for _, agentType := range cliagents.SupportedAgents {
		result, _ := generator.Validate(cliagents.AgentType(agentType), config)
		if result != nil && result.Valid {
			fmt.Printf("Valid configuration for: %s\n", agentType)
			return
		}
	}

	// If no valid agent type found, try OpenCode specifically (most common)
	result, _ := generator.Validate(cliagents.AgentOpenCode, config)
	if result != nil {
		if result.Valid {
			fmt.Println("Valid OpenCode configuration")
		} else {
			fmt.Println("Validation errors:")
			for _, err := range result.Errors {
				fmt.Printf("  - %s\n", err)
			}
			os.Exit(1)
		}
	}
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
