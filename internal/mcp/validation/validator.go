// Package validation provides MCP server validation and verification
package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/adapters/containers"
)

// MCPRequirement defines what an MCP server needs to work
type MCPRequirement struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"` // "local" or "remote"
	Package        string   `json:"package,omitempty"`
	Command        []string `json:"command,omitempty"`
	URL            string   `json:"url,omitempty"`
	RequiredEnvs   []string `json:"required_envs,omitempty"`
	OptionalEnvs   []string `json:"optional_envs,omitempty"`
	LocalServices  []string `json:"local_services,omitempty"` // e.g., "postgresql", "redis"
	Description    string   `json:"description"`
	Category       string   `json:"category"`
	CanWorkLocally bool     `json:"can_work_locally"` // Can work without external API
	Enabled        bool     `json:"enabled"`
	Priority       int      `json:"priority"` // Higher = more important
}

// MCPValidationResult contains the result of validating an MCP server
type MCPValidationResult struct {
	Name            string    `json:"name"`
	Status          string    `json:"status"` // "works", "disabled", "failed", "missing_deps"
	CanEnable       bool      `json:"can_enable"`
	MissingEnvVars  []string  `json:"missing_env_vars,omitempty"`
	MissingServices []string  `json:"missing_services,omitempty"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	ResponseTimeMs  int64     `json:"response_time_ms,omitempty"`
	TestedAt        time.Time `json:"tested_at"`
	Category        string    `json:"category"`
	Reason          string    `json:"reason,omitempty"`
}

// MCPValidationReport contains the full validation report
type MCPValidationReport struct {
	GeneratedAt     time.Time                       `json:"generated_at"`
	TotalMCPs       int                             `json:"total_mcps"`
	WorkingMCPs     int                             `json:"working_mcps"`
	DisabledMCPs    int                             `json:"disabled_mcps"`
	FailedMCPs      int                             `json:"failed_mcps"`
	Results         map[string]*MCPValidationResult `json:"results"`
	EnabledMCPList  []string                        `json:"enabled_mcp_list"`
	DisabledMCPList []string                        `json:"disabled_mcp_list"`
	Summary         string                          `json:"summary"`
}

// MCPValidator validates MCP servers.
//
// Concurrency model (CONST-029): requirements and envCache are
// written only during NewMCPValidator (loadRequirements /
// loadEnvCache) and read lock-free afterwards — they are effectively
// immutable, so the bare-map form is safe without a mutex. results
// is populated concurrently by the fan-out of validateMCP
// goroutines and migrates to *safe.Store. No mutex survives because
// the three maps have no cross-key invariants.
type MCPValidator struct {
	requirements     map[string]*MCPRequirement
	results          *safe.Store[string, *MCPValidationResult]
	envCache         map[string]string
	timeout          time.Duration
	ContainerAdapter *containers.Adapter
}

// NewMCPValidator creates a new MCP validator
func NewMCPValidator() *MCPValidator {
	v := &MCPValidator{
		requirements: make(map[string]*MCPRequirement),
		results:      safe.NewStore[string, *MCPValidationResult](),
		envCache:     make(map[string]string),
		timeout:      30 * time.Second,
	}
	v.loadRequirements()
	v.loadEnvCache()
	return v
}

// loadEnvCache loads environment variables into cache
func (v *MCPValidator) loadEnvCache() {
	// Load from .env file if exists
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
				value := strings.TrimSpace(parts[1])
				// Remove quotes if present
				value = strings.Trim(value, "\"'")
				v.envCache[key] = value
			}
		}
	}

	// Also load from environment
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			v.envCache[parts[0]] = parts[1]
		}
	}
}

// hasEnvVar checks if an environment variable is set and non-empty
func (v *MCPValidator) hasEnvVar(name string) bool {
	if val, ok := v.envCache[name]; ok && val != "" {
		return true
	}
	return false
}

// loadRequirements loads all MCP server requirements
func (v *MCPValidator) loadRequirements() {
	// ============================================================================
	// Category: HelixAgent (Always work if HelixAgent is running)
	// ============================================================================
	v.requirements["helixagent"] = &MCPRequirement{
		Name:           "helixagent",
		Type:           "local",
		Command:        []string{"node", "${HELIXAGENT_HOME}/plugins/mcp-server/dist/index.js", "--endpoint", "http://localhost:8100"},
		Description:    "HelixAgent MCP plugin - provides unified access to HelixAgent APIs",
		Category:       "helixagent",
		CanWorkLocally: true,
		LocalServices:  []string{"helixagent"},
		Enabled:        true,
		Priority:       100,
	}

	// ============================================================================
	// Category: Core (No API keys required, always work)
	// ============================================================================
	v.requirements["filesystem"] = &MCPRequirement{
		Name:           "filesystem",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-filesystem",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "${HOME}"},
		Description:    "File system access - read, write, list files",
		Category:       "core",
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       95,
	}
	v.requirements["fetch"] = &MCPRequirement{
		Name:           "fetch",
		Type:           "local",
		Package:        "mcp-fetch-server",
		Command:        []string{"npx", "-y", "mcp-fetch-server"},
		Description:    "HTTP fetch - make web requests",
		Category:       "core",
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       95,
	}
	v.requirements["memory"] = &MCPRequirement{
		Name:           "memory",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-memory",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-memory"},
		Description:    "In-memory key-value storage",
		Category:       "core",
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       90,
	}
	v.requirements["time"] = &MCPRequirement{
		Name:           "time",
		Type:           "local",
		Package:        "@theo.foobar/mcp-time",
		Command:        []string{"npx", "-y", "@theo.foobar/mcp-time"},
		Description:    "Time and timezone utilities",
		Category:       "core",
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       90,
	}
	v.requirements["git"] = &MCPRequirement{
		Name:           "git",
		Type:           "local",
		Package:        "mcp-git",
		Command:        []string{"npx", "-y", "mcp-git"},
		Description:    "Git repository operations",
		Category:       "core",
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       90,
	}
	v.requirements["sequential-thinking"] = &MCPRequirement{
		Name:           "sequential-thinking",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-sequential-thinking",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-sequential-thinking"},
		Description:    "Sequential thinking and reasoning",
		Category:       "core",
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       85,
	}
	v.requirements["everything"] = &MCPRequirement{
		Name:           "everything",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-everything",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-everything"},
		Description:    "Test MCP server with all features",
		Category:       "core",
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       50,
	}

	// ============================================================================
	// Category: Database (Local services required)
	// ============================================================================
	v.requirements["sqlite"] = &MCPRequirement{
		Name:           "sqlite",
		Type:           "local",
		Package:        "mcp-server-sqlite-npx",
		Command:        []string{"npx", "-y", "mcp-server-sqlite-npx", "/tmp/helixagent.db"},
		Description:    "SQLite database operations",
		Category:       "database",
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       85,
	}
	v.requirements["postgres"] = &MCPRequirement{
		Name:           "postgres",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-postgres",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-postgres"},
		RequiredEnvs:   []string{"POSTGRES_URL"},
		Description:    "PostgreSQL database operations",
		Category:       "database",
		LocalServices:  []string{"postgresql"},
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       80,
	}
	// Fixed 2026-09-03. `mcp-server-redis` is an npm SECURITY HOLDING package
	// (resolves, installs nothing). Real package:
	// @modelcontextprotocol/server-redis, which reads the URL from
	// process.argv[2] — so it is passed positionally and RequiredEnvs is now
	// empty rather than naming a variable the server never reads.
	v.requirements["redis"] = &MCPRequirement{
		Name:           "redis",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-redis",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-redis", "redis://localhost:6379"},
		Description:    "Redis cache operations",
		Category:       "database",
		LocalServices:  []string{"redis"},
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       75,
	}
	v.requirements["mongodb"] = &MCPRequirement{
		Name:           "mongodb",
		Type:           "local",
		Package:        "mongodb-mcp-server",
		Command:        []string{"npx", "-y", "mongodb-mcp-server"},
		RequiredEnvs:   []string{"MONGODB_URI"},
		Description:    "MongoDB database operations",
		Category:       "database",
		LocalServices:  []string{"mongodb"},
		CanWorkLocally: true,
		Enabled:        false, // Disabled by default - needs MongoDB
		Priority:       70,
	}

	// ============================================================================
	// Category: DevOps (Local tools required)
	// ============================================================================
	v.requirements["docker"] = &MCPRequirement{
		Name:           "docker",
		Type:           "local",
		Package:        "mcp-server-docker",
		Command:        []string{"npx", "-y", "mcp-server-docker"},
		Description:    "Docker container management",
		Category:       "devops",
		LocalServices:  []string{"docker"},
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       80,
	}
	v.requirements["kubernetes"] = &MCPRequirement{
		Name:           "kubernetes",
		Type:           "local",
		Package:        "mcp-server-kubernetes",
		Command:        []string{"npx", "-y", "mcp-server-kubernetes"},
		RequiredEnvs:   []string{"KUBECONFIG"},
		Description:    "Kubernetes cluster management",
		Category:       "devops",
		LocalServices:  []string{"kubernetes"},
		CanWorkLocally: true,
		Enabled:        false, // Disabled by default - needs kubectl configured
		Priority:       70,
	}
	v.requirements["puppeteer"] = &MCPRequirement{
		Name:           "puppeteer",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-puppeteer",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-puppeteer"},
		Description:    "Browser automation with Puppeteer",
		Category:       "devops",
		CanWorkLocally: true,
		Enabled:        true,
		Priority:       75,
	}

	// ============================================================================
	// Category: Development (API keys required)
	// ============================================================================
	v.requirements["github"] = &MCPRequirement{
		Name:           "github",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-github",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-github"},
		RequiredEnvs:   []string{"GITHUB_TOKEN"},
		Description:    "GitHub API operations",
		Category:       "development",
		CanWorkLocally: false,
		Enabled:        true, // Will be enabled if GITHUB_TOKEN exists
		Priority:       85,
	}
	v.requirements["gitlab"] = &MCPRequirement{
		Name:           "gitlab",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-gitlab",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-gitlab"},
		RequiredEnvs:   []string{"GITLAB_TOKEN"},
		Description:    "GitLab API operations",
		Category:       "development",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no GITLAB_TOKEN
		Priority:       80,
	}
	v.requirements["sentry"] = &MCPRequirement{
		Name:           "sentry",
		Type:           "local",
		Package:        "@sentry/mcp-server",
		Command:        []string{"npx", "-y", "@sentry/mcp-server"},
		RequiredEnvs:   []string{"SENTRY_AUTH_TOKEN", "SENTRY_ORG"},
		Description:    "Sentry error tracking",
		Category:       "development",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no SENTRY keys
		Priority:       70,
	}

	// ============================================================================
	// Category: Communication (API keys required)
	// ============================================================================
	v.requirements["slack"] = &MCPRequirement{
		Name:           "slack",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-slack",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-slack"},
		RequiredEnvs:   []string{"SLACK_BOT_TOKEN", "SLACK_TEAM_ID"},
		Description:    "Slack workspace integration",
		Category:       "communication",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no Slack keys
		Priority:       75,
	}
	v.requirements["discord"] = &MCPRequirement{
		Name:           "discord",
		Type:           "local",
		Package:        "mcp-server-discord",
		Command:        []string{"npx", "-y", "mcp-server-discord"},
		RequiredEnvs:   []string{"DISCORD_TOKEN"},
		Description:    "Discord bot integration",
		Category:       "communication",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no Discord token
		Priority:       70,
	}

	// ============================================================================
	// Category: Search (API keys required)
	// ============================================================================
	v.requirements["brave-search"] = &MCPRequirement{
		Name:           "brave-search",
		Type:           "local",
		Package:        "@modelcontextprotocol/server-brave-search",
		Command:        []string{"npx", "-y", "@modelcontextprotocol/server-brave-search"},
		RequiredEnvs:   []string{"BRAVE_API_KEY"},
		Description:    "Brave Search API",
		Category:       "search",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no Brave API key
		Priority:       80,
	}
	v.requirements["exa"] = &MCPRequirement{
		Name:           "exa",
		Type:           "local",
		Package:        "exa-mcp-server",
		Command:        []string{"npx", "-y", "exa-mcp-server"},
		RequiredEnvs:   []string{"EXA_API_KEY"},
		Description:    "Exa AI search",
		Category:       "search",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no Exa API key
		Priority:       70,
	}

	// ============================================================================
	// Category: Productivity (API keys required)
	// ============================================================================
	// Restored 2026-09-03. ce9c4eb9 removed this because
	// @modelcontextprotocol/server-linear 404s, and correctly refused
	// `linear-mcp-server` whose declared repository 404s. Replacement verified
	// live: @tacticlaunch/mcp-linear v1.4.3, repo
	// github.com/tacticlaunch/mcp-linear -> HTTP 200, ~4856 downloads/week,
	// community publisher. RequiredEnvs names LINEAR_API_TOKEN — the variable
	// the package actually reads, not the removed entry's LINEAR_API_KEY.
	v.requirements["linear"] = &MCPRequirement{
		Name:           "linear",
		Type:           "local",
		Package:        "@tacticlaunch/mcp-linear",
		Command:        []string{"npx", "-y", "@tacticlaunch/mcp-linear"},
		RequiredEnvs:   []string{"LINEAR_API_TOKEN"},
		Description:    "Linear issue tracking",
		Category:       "productivity",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no Linear API token
		Priority:       75,
	}
	v.requirements["notion"] = &MCPRequirement{
		Name:           "notion",
		Type:           "local",
		Package:        "@notionhq/notion-mcp-server",
		Command:        []string{"npx", "-y", "@notionhq/notion-mcp-server"},
		RequiredEnvs:   []string{"NOTION_API_KEY"},
		Description:    "Notion workspace integration",
		Category:       "productivity",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no Notion API key
		Priority:       75,
	}
	v.requirements["todoist"] = &MCPRequirement{
		Name:           "todoist",
		Type:           "local",
		Package:        "@doist/todoist-mcp",
		Command:        []string{"npx", "-y", "@doist/todoist-mcp"},
		RequiredEnvs:   []string{"TODOIST_API_TOKEN"},
		Description:    "Todoist task management",
		Category:       "productivity",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no Todoist API key
		Priority:       70,
	}
	v.requirements["figma"] = &MCPRequirement{
		Name:           "figma",
		Type:           "local",
		Package:        "figma-developer-mcp",
		Command:        []string{"npx", "-y", "figma-developer-mcp"},
		RequiredEnvs:   []string{"FIGMA_API_KEY"},
		Description:    "Figma design platform",
		Category:       "productivity",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no Figma API key
		Priority:       70,
	}

	// ============================================================================
	// Category: Cloud (API keys required)
	// ============================================================================
	v.requirements["cloudflare"] = &MCPRequirement{
		Name:           "cloudflare",
		Type:           "local",
		Package:        "@cloudflare/mcp-server-cloudflare",
		Command:        []string{"npx", "-y", "@cloudflare/mcp-server-cloudflare"},
		RequiredEnvs:   []string{"CLOUDFLARE_API_TOKEN"},
		Description:    "Cloudflare services",
		Category:       "cloud",
		CanWorkLocally: false,
		Enabled:        false, // Disabled - no Cloudflare API token
		Priority:       70,
	}
}

// ValidateAll validates all MCP servers
func (v *MCPValidator) ValidateAll(ctx context.Context) *MCPValidationReport {
	report := &MCPValidationReport{
		GeneratedAt: time.Now(),
		Results:     make(map[string]*MCPValidationResult),
	}

	var wg sync.WaitGroup
	resultChan := make(chan *MCPValidationResult, len(v.requirements))

	for name, req := range v.requirements {
		wg.Add(1)
		go func(name string, req *MCPRequirement) {
			defer wg.Done()
			result := v.validateMCP(ctx, name, req)
			resultChan <- result
		}(name, req)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		v.results.Put(result.Name, result)
		report.Results[result.Name] = result

		if result.CanEnable {
			report.WorkingMCPs++
			report.EnabledMCPList = append(report.EnabledMCPList, result.Name)
		} else if result.Status == "disabled" {
			report.DisabledMCPs++
			report.DisabledMCPList = append(report.DisabledMCPList, result.Name)
		} else {
			report.FailedMCPs++
			report.DisabledMCPList = append(report.DisabledMCPList, result.Name)
		}
		report.TotalMCPs++
	}

	report.Summary = fmt.Sprintf(
		"MCP Validation Complete: %d total, %d working, %d disabled, %d failed",
		report.TotalMCPs, report.WorkingMCPs, report.DisabledMCPs, report.FailedMCPs,
	)

	return report
}

// validateMCP validates a single MCP server
func (v *MCPValidator) validateMCP(ctx context.Context, name string, req *MCPRequirement) *MCPValidationResult {
	result := &MCPValidationResult{
		Name:     name,
		TestedAt: time.Now(),
		Category: req.Category,
	}

	// Check required environment variables
	var missingEnvs []string
	for _, env := range req.RequiredEnvs {
		if !v.hasEnvVar(env) {
			missingEnvs = append(missingEnvs, env)
		}
	}

	if len(missingEnvs) > 0 {
		result.Status = "disabled"
		result.CanEnable = false
		result.MissingEnvVars = missingEnvs
		result.Reason = fmt.Sprintf("Missing required environment variables: %s", strings.Join(missingEnvs, ", "))
		return result
	}

	// Check local services
	var missingServices []string
	for _, service := range req.LocalServices {
		if !v.checkLocalService(service) {
			missingServices = append(missingServices, service)
		}
	}

	if len(missingServices) > 0 && !req.CanWorkLocally {
		result.Status = "missing_deps"
		result.CanEnable = false
		result.MissingServices = missingServices
		result.Reason = fmt.Sprintf("Missing local services: %s", strings.Join(missingServices, ", "))
		return result
	}

	// If all checks pass, mark as working
	result.Status = "works"
	result.CanEnable = true
	result.Reason = "All requirements satisfied"

	return result
}

// checkLocalService checks if a local service is running
func (v *MCPValidator) checkLocalService(service string) bool {
	switch service {
	case "helixagent":
		resp, err := http.Get("http://localhost:8100/health")
		if err != nil {
			return false
		}
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode == http.StatusOK
	case "postgresql":
		// Check if PostgreSQL is running on configured port
		_, err := exec.Command("pg_isready", "-h", "localhost", "-p", "15432").Output()
		return err == nil
	case "redis":
		_, err := exec.Command("redis-cli", "-p", "16379", "ping").Output()
		return err == nil
	case "docker":
		if v.ContainerAdapter != nil {
			return v.ContainerAdapter.RuntimeAvailable(context.Background())
		}
		return false
	case "kubernetes":
		_, err := exec.Command("kubectl", "cluster-info").Output()
		return err == nil
	default:
		return true // Unknown service, assume available
	}
}

// GetEnabledMCPs returns list of MCPs that should be enabled
func (v *MCPValidator) GetEnabledMCPs() []string {
	var enabled []string
	v.results.Range(func(name string, result *MCPValidationResult) bool {
		if result.CanEnable {
			enabled = append(enabled, name)
		}
		return true
	})
	return enabled
}

// GetMCPConfig returns the MCP configuration for enabled MCPs
func (v *MCPValidator) GetMCPConfig(name string) *MCPRequirement {
	return v.requirements[name]
}

// GenerateReport generates a human-readable report
func (v *MCPValidator) GenerateReport() string {
	results := v.results.Snapshot()

	var sb strings.Builder
	sb.WriteString("=" + strings.Repeat("=", 78) + "\n")
	sb.WriteString("MCP VALIDATION REPORT\n")
	sb.WriteString("=" + strings.Repeat("=", 78) + "\n\n")

	// Count by status
	working := 0
	disabled := 0
	failed := 0

	for _, result := range results {
		switch result.Status {
		case "works":
			working++
		case "disabled":
			disabled++
		default:
			failed++
		}
	}

	sb.WriteString(fmt.Sprintf("Summary: %d total, %d working, %d disabled, %d failed\n\n",
		len(results), working, disabled, failed))

	// Working MCPs
	sb.WriteString("WORKING MCPs (Enabled):\n")
	sb.WriteString("-" + strings.Repeat("-", 78) + "\n")
	for name, result := range results {
		if result.Status == "works" {
			req := v.requirements[name]
			sb.WriteString(fmt.Sprintf("  ✓ %-25s [%s] %s\n", name, req.Category, req.Description))
		}
	}

	// Disabled MCPs
	sb.WriteString("\nDISABLED MCPs (Missing Requirements):\n")
	sb.WriteString("-" + strings.Repeat("-", 78) + "\n")
	for name, result := range results {
		if result.Status == "disabled" || result.Status == "missing_deps" {
			req := v.requirements[name]
			sb.WriteString(fmt.Sprintf("  ✗ %-25s [%s] %s\n", name, req.Category, result.Reason))
		}
	}

	sb.WriteString("\n" + strings.Repeat("=", 79) + "\n")

	return sb.String()
}

// ToJSON returns the validation results as JSON
func (v *MCPValidator) ToJSON() ([]byte, error) {
	results := v.results.Snapshot()
	report := &MCPValidationReport{
		GeneratedAt: time.Now(),
		TotalMCPs:   len(results),
		Results:     results,
	}

	for _, result := range results {
		if result.CanEnable {
			report.WorkingMCPs++
			report.EnabledMCPList = append(report.EnabledMCPList, result.Name)
		} else {
			report.DisabledMCPs++
			report.DisabledMCPList = append(report.DisabledMCPList, result.Name)
		}
	}

	return json.MarshalIndent(report, "", "  ")
}
