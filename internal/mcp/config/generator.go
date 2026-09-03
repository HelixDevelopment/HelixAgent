// Package config provides MCP configuration generation for CLI agents
package config

import (
	"fmt"
	"os"
	"strings"
)

// MCPServerConfig represents an MCP server configuration
type MCPServerConfig struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Enabled     bool              `json:"enabled"`
}

// MCPConfigGenerator generates validated MCP configurations
type MCPConfigGenerator struct {
	homeDir   string
	helixHome string
	baseURL   string
	envVars   map[string]string
}

// NewMCPConfigGenerator creates a new MCP config generator
func NewMCPConfigGenerator(baseURL string) *MCPConfigGenerator {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home"
	}

	helixHome := os.Getenv("HELIXAGENT_HOME")
	if helixHome == "" {
		helixHome = homeDir + "/.helixagent"
	}

	g := &MCPConfigGenerator{
		homeDir:   homeDir,
		helixHome: helixHome,
		baseURL:   baseURL,
		envVars:   make(map[string]string),
	}

	// Load environment variables
	g.loadEnvVars()

	return g
}

// loadEnvVars loads environment variables from .env and environment
func (g *MCPConfigGenerator) loadEnvVars() {
	// Load from .env file
	if data, err := os.ReadFile(".env"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				g.envVars[key] = value
			}
		}
	}

	// Also load from environment (overrides .env)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			g.envVars[parts[0]] = parts[1]
		}
	}
}

// hasEnvVar checks if an environment variable is set and non-empty
func (g *MCPConfigGenerator) hasEnvVar(name string) bool {
	val, ok := g.envVars[name]
	return ok && val != ""
}

// getEnvOrDefault returns the environment variable value or a default
func (g *MCPConfigGenerator) getEnvOrDefault(name, defaultVal string) string {
	if val, ok := g.envVars[name]; ok && val != "" {
		return val
	}
	return defaultVal
}

// GenerateOpenCodeMCPs generates MCP configurations for OpenCode
// Only includes MCPs that have all required dependencies
func (g *MCPConfigGenerator) GenerateOpenCodeMCPs() map[string]MCPServerConfig {
	mcps := make(map[string]MCPServerConfig)

	// ==========================================================================
	// Category 1: HelixAgent (Always enabled - core functionality)
	// ==========================================================================
	mcps["helixagent"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"node", g.helixHome + "/plugins/mcp-server/dist/index.js", "--endpoint", g.baseURL},
		Enabled: true,
	}

	// ==========================================================================
	// Category 2: Core MCPs (No API keys required - always work)
	// ==========================================================================
	mcps["filesystem"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", g.homeDir},
		Enabled: true,
	}
	mcps["fetch"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "mcp-fetch-server"},
		Enabled: true,
	}
	mcps["memory"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "@modelcontextprotocol/server-memory"},
		Enabled: true,
	}
	mcps["time"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "@theo.foobar/mcp-time"},
		Enabled: true,
	}
	mcps["git"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "mcp-git"},
		Enabled: true,
	}
	mcps["sequential-thinking"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "@modelcontextprotocol/server-sequential-thinking"},
		Enabled: true,
	}
	mcps["everything"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "@modelcontextprotocol/server-everything"},
		Enabled: true,
	}

	// ==========================================================================
	// Category 3: Database MCPs (Local services)
	// ==========================================================================
	mcps["sqlite"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "mcp-server-sqlite-npx", "/tmp/helixagent.db"},
		Enabled: true,
	}

	// PostgreSQL - enabled if POSTGRES_URL is set or service is running
	// Build URL from components to avoid hardcoding credentials
	postgresURL := g.getEnvOrDefault("POSTGRES_URL", fmt.Sprintf("postgresql://%s:%s@%s:%s/%s",
		g.getEnvOrDefault("POSTGRES_USER", "helixagent"),
		os.Getenv("POSTGRES_PASSWORD"),
		g.getEnvOrDefault("POSTGRES_HOST", "localhost"),
		g.getEnvOrDefault("POSTGRES_PORT", "15432"),
		g.getEnvOrDefault("POSTGRES_DB", "helixagent_db"),
	))
	mcps["postgres"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "@modelcontextprotocol/server-postgres"},
		Environment: map[string]string{
			"POSTGRES_URL": postgresURL,
		},
		Enabled: true,
	}

	// Redis - enabled if REDIS_URL is set
	if g.hasEnvVar("REDIS_URL") || g.hasEnvVar("REDIS_HOST") {
		redisURL := g.getEnvOrDefault("REDIS_URL", fmt.Sprintf("redis://:%s@%s:%s",
			os.Getenv("REDIS_PASSWORD"),
			g.getEnvOrDefault("REDIS_HOST", "localhost"),
			g.getEnvOrDefault("REDIS_PORT", "16379"),
		))
		mcps["redis"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-redis"},
			Environment: map[string]string{
				"REDIS_URL": redisURL,
			},
			Enabled: true,
		}
	}

	// MongoDB - enabled if MONGODB_URI is set
	if g.hasEnvVar("MONGODB_URI") || g.hasEnvVar("MONGODB_HOST") {
		mongoURI := g.getEnvOrDefault("MONGODB_URI", fmt.Sprintf("mongodb://%s:%s@%s:%s/%s?authSource=admin",
			g.getEnvOrDefault("MONGODB_USER", "helixagent"),
			os.Getenv("MONGODB_PASSWORD"),
			g.getEnvOrDefault("MONGODB_HOST", "localhost"),
			g.getEnvOrDefault("MONGODB_PORT", "27017"),
			g.getEnvOrDefault("MONGODB_DB", "helixagent"),
		))
		mcps["mongodb"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "mongodb-mcp-server"},
			Environment: map[string]string{
				"MONGODB_URI": mongoURI,
			},
			Enabled: true,
		}
	}

	// ==========================================================================
	// Category 4: Vector Database MCPs
	// ==========================================================================

	// Qdrant - enabled if QDRANT_URL is set
	if g.hasEnvVar("QDRANT_URL") || g.hasEnvVar("QDRANT_HOST") {
		qdrantURL := g.getEnvOrDefault("QDRANT_URL", "http://localhost:6333")
		mcps["qdrant"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-qdrant"},
			Environment: map[string]string{
				"QDRANT_URL": qdrantURL,
			},
			Enabled: true,
		}
	}

	// Chroma - enabled if CHROMA_URL is set
	if g.hasEnvVar("CHROMA_URL") || g.hasEnvVar("CHROMA_HOST") {
		chromaURL := g.getEnvOrDefault("CHROMA_URL", "http://localhost:8000")
		mcps["chroma"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-chroma"},
			Environment: map[string]string{
				"CHROMA_URL": chromaURL,
			},
			Enabled: true,
		}
	}

	// ==========================================================================
	// Category 5: DevOps MCPs (Local tools)
	// ==========================================================================
	mcps["docker"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "mcp-server-docker"},
		Enabled: true,
	}
	mcps["puppeteer"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "@modelcontextprotocol/server-puppeteer"},
		Enabled: true,
	}
	mcps["playwright"] = MCPServerConfig{
		Type:    "local",
		Command: []string{"npx", "-y", "@playwright/mcp"},
		Enabled: true,
	}

	// Kubernetes - enabled if KUBECONFIG is set
	if g.hasEnvVar("KUBECONFIG") {
		mcps["kubernetes"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-kubernetes"},
			Environment: g.expandEnvMap(map[string]string{
				"KUBECONFIG": "{env:KUBECONFIG}",
			}),
			Enabled: true,
		}
	}

	// ==========================================================================
	// Category 5: Conditional MCPs (Only if API keys are available)
	// ==========================================================================

	// GitHub - enabled if GITHUB_TOKEN is set
	if g.hasEnvVar("GITHUB_TOKEN") {
		mcps["github"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"},
			Environment: g.expandEnvMap(map[string]string{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
				"GITHUB_PERSONAL_ACCESS_TOKEN": "{env:GITHUB_TOKEN}",
			}),
			Enabled: true,
		}
	}

	// GitLab - enabled if GITLAB_TOKEN is set
	if g.hasEnvVar("GITLAB_TOKEN") {
		mcps["gitlab"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-gitlab"},
			Environment: g.expandEnvMap(map[string]string{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
				"GITLAB_PERSONAL_ACCESS_TOKEN": "{env:GITLAB_TOKEN}",
			}),
			Enabled: true,
		}
	}

	// Brave Search - enabled if BRAVE_API_KEY is set
	if g.hasEnvVar("BRAVE_API_KEY") {
		mcps["brave-search"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-brave-search"},
			Environment: g.expandEnvMap(map[string]string{
				"BRAVE_API_KEY": "{env:BRAVE_API_KEY}",
			}),
			Enabled: true,
		}
	}

	// Slack - enabled if SLACK_BOT_TOKEN is set
	if g.hasEnvVar("SLACK_BOT_TOKEN") {
		mcps["slack"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-slack"},
			Environment: g.expandEnvMap(map[string]string{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
				"SLACK_BOT_TOKEN": "{env:SLACK_BOT_TOKEN}",
				"SLACK_TEAM_ID":   "{env:SLACK_TEAM_ID}",
			}),
			Enabled: true,
		}
	}

	// Notion - enabled if NOTION_API_KEY is set
	if g.hasEnvVar("NOTION_API_KEY") {
		mcps["notion"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "@notionhq/notion-mcp-server"},
			Environment: g.expandEnvMap(map[string]string{
				"NOTION_API_KEY": "{env:NOTION_API_KEY}",
			}),
			Enabled: true,
		}
	}

	// Sentry - enabled if SENTRY_AUTH_TOKEN is set
	if g.hasEnvVar("SENTRY_AUTH_TOKEN") {
		mcps["sentry"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "@sentry/mcp-server"},
			Environment: g.expandEnvMap(map[string]string{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
				"SENTRY_AUTH_TOKEN": "{env:SENTRY_AUTH_TOKEN}",
				"SENTRY_ORG":        "{env:SENTRY_ORG}",
			}),
			Enabled: true,
		}
	}

	// Cloudflare - enabled if CLOUDFLARE_API_TOKEN is set
	if g.hasEnvVar("CLOUDFLARE_API_TOKEN") {
		mcps["cloudflare"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "@cloudflare/mcp-server-cloudflare"},
			Environment: g.expandEnvMap(map[string]string{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
				"CLOUDFLARE_API_TOKEN": "{env:CLOUDFLARE_API_TOKEN}",
			}),
			Enabled: true,
		}
	}

	// Discord - enabled if DISCORD_TOKEN is set
	if g.hasEnvVar("DISCORD_TOKEN") {
		mcps["discord"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-discord"},
			Environment: g.expandEnvMap(map[string]string{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
				"DISCORD_TOKEN": "{env:DISCORD_TOKEN}",
			}),
			Enabled: true,
		}
	}

	// Exa - enabled if EXA_API_KEY is set
	if g.hasEnvVar("EXA_API_KEY") {
		mcps["exa"] = MCPServerConfig{
			Type:    "local",
			Command: []string{"npx", "-y", "exa-mcp-server"},
			Environment: g.expandEnvMap(map[string]string{
				"EXA_API_KEY": "{env:EXA_API_KEY}",
			}),
			Enabled: true,
		}
	}

	return mcps
}

// expandEnvValue expands {env:VAR_NAME} placeholders with actual environment variable values
func (g *MCPConfigGenerator) expandEnvValue(value string) string {
	if strings.HasPrefix(value, "{env:") && strings.HasSuffix(value, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(value, "{env:"), "}")
		if val, ok := g.envVars[envVar]; ok && val != "" {
			return val
		}
	}
	return value
}

// expandEnvMap expands all {env:...} placeholders in an environment map
func (g *MCPConfigGenerator) expandEnvMap(env map[string]string) map[string]string {
	if env == nil {
		return nil
	}
	expanded := make(map[string]string, len(env))
	for k, v := range env {
		expanded[k] = g.expandEnvValue(v)
	}
	return expanded
}

// GetEnabledMCPCount returns the count of enabled MCPs
func (g *MCPConfigGenerator) GetEnabledMCPCount() int {
	mcps := g.GenerateOpenCodeMCPs()
	count := 0
	for _, mcp := range mcps {
		if mcp.Enabled {
			count++
		}
	}
	return count
}

// GetMCPSummary returns a summary of enabled/disabled MCPs
func (g *MCPConfigGenerator) GetMCPSummary() map[string]interface{} {
	mcps := g.GenerateOpenCodeMCPs()

	enabled := []string{}
	for name, mcp := range mcps {
		if mcp.Enabled {
			enabled = append(enabled, name)
		}
	}

	return map[string]interface{}{
		"total_enabled": len(enabled),
		"enabled_mcps":  enabled,
		"categories": map[string]int{
			"core":     8, // filesystem, fetch, memory, time, git, sequential-thinking, everything, helixagent
			"database": 2, // sqlite, postgres
			"devops":   2, // docker, puppeteer
		},
	}
}
