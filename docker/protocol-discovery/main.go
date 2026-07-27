// Protocol Discovery Service with MCP Tool Search Technology
// Central registry for all MCP, LSP, ACP, Embedding servers
// ALL MCP integrations MUST use MCP Tool Search technology
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// MCP TOOL SEARCH TYPES (Matching HelixAgent internal/tools/schema.go)
// =============================================================================

// ToolSchema defines the schema for a tool
type ToolSchema struct {
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	RequiredFields []string         `json:"required_fields"`
	OptionalFields []string         `json:"optional_fields,omitempty"`
	Aliases        []string         `json:"aliases,omitempty"`
	Category       string           `json:"category"`
	Parameters     map[string]Param `json:"parameters"`
}

// Param defines a parameter for a tool
type Param struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// ToolSearchResult represents a search result with relevance score
type ToolSearchResult struct {
	Tool      *ToolSchema `json:"tool"`
	Score     float64     `json:"score"`
	MatchType string      `json:"match_type"`
}

// SearchOptions configures the tool search behavior
type SearchOptions struct {
	Query         string   `json:"query"`
	Categories    []string `json:"categories,omitempty"`
	IncludeParams bool     `json:"include_params,omitempty"`
	FuzzyMatch    bool     `json:"fuzzy_match,omitempty"`
	MaxResults    int      `json:"max_results,omitempty"`
	MinScore      float64  `json:"min_score,omitempty"`
}

// =============================================================================
// PROTOCOL SERVER TYPES
// =============================================================================

// ProtocolServer represents a registered protocol server with MCP Tool Search
type ProtocolServer struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"` // mcp, lsp, acp, embedding, rag
	URL          string            `json:"url"`
	Status       string            `json:"status"` // healthy, unhealthy, unknown
	Version      string            `json:"version,omitempty"`
	Description  string            `json:"description,omitempty"`
	Tools        []*ToolSchema     `json:"tools,omitempty"` // MCP Tool Search compatible tools
	Capabilities []string          `json:"capabilities,omitempty"`
	LastCheck    time.Time         `json:"last_check"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	// MCP Tool Search fields
	Category  string   `json:"category,omitempty"`
	Aliases   []string `json:"aliases,omitempty"`
	AuthType  string   `json:"auth_type,omitempty"`
	Official  bool     `json:"official,omitempty"`
	Supported bool     `json:"supported,omitempty"`
}

// ServerSearchResult represents a server search result using MCP Tool Search
type ServerSearchResult struct {
	Server    ProtocolServer `json:"server"`
	Score     float64        `json:"score"`
	MatchType string         `json:"match_type"` // name, description, category, tool
}

// DiscoveryResponse represents the discovery API response
type DiscoveryResponse struct {
	TotalServers     int              `json:"total_servers"`
	MCPServers       []ProtocolServer `json:"mcp_servers"`
	LSPServers       []ProtocolServer `json:"lsp_servers"`
	ACPServers       []ProtocolServer `json:"acp_servers"`
	EmbeddingServers []ProtocolServer `json:"embedding_servers"`
	RAGServers       []ProtocolServer `json:"rag_servers"`
	Timestamp        time.Time        `json:"timestamp"`
}

// Registry holds all registered servers
type Registry struct {
	servers map[string]ProtocolServer
	mu      sync.RWMutex
}

var registry = &Registry{
	servers: make(map[string]ProtocolServer),
}

// =============================================================================
// MCP TOOL SEARCH IMPLEMENTATION
// =============================================================================

// SearchServers searches for servers using MCP Tool Search technology
func (r *Registry) SearchServers(opts SearchOptions) []ServerSearchResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if opts.MaxResults <= 0 {
		opts.MaxResults = 50
	}
	if opts.MinScore <= 0 {
		opts.MinScore = 0.1
	}

	query := strings.ToLower(opts.Query)
	var results []ServerSearchResult

	for _, server := range r.servers {
		// Filter by category if specified
		if len(opts.Categories) > 0 {
			found := false
			for _, cat := range opts.Categories {
				if strings.EqualFold(server.Type, cat) || strings.EqualFold(server.Category, cat) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		score, matchType := calculateServerScore(&server, query, opts)
		if score >= opts.MinScore {
			results = append(results, ServerSearchResult{
				Server:    server,
				Score:     score,
				MatchType: matchType,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}

	return results
}

// calculateServerScore calculates relevance score for MCP Tool Search
func calculateServerScore(server *ProtocolServer, query string, opts SearchOptions) (float64, string) {
	if query == "" {
		return 1.0, "all"
	}

	var maxScore float64
	var matchType string

	// Exact name match - highest score
	if strings.EqualFold(server.Name, query) {
		return 1.0, "name_exact"
	}

	// Name contains query
	nameLower := strings.ToLower(server.Name)
	if strings.Contains(nameLower, query) {
		score := 0.9 * (float64(len(query)) / float64(len(nameLower)))
		if score > maxScore {
			maxScore = score
			matchType = "name"
		}
	}

	// Check aliases
	for _, alias := range server.Aliases {
		aliasLower := strings.ToLower(alias)
		if strings.EqualFold(alias, query) {
			return 0.95, "alias_exact"
		}
		if strings.Contains(aliasLower, query) {
			score := 0.85 * (float64(len(query)) / float64(len(aliasLower)))
			if score > maxScore {
				maxScore = score
				matchType = "alias"
			}
		}
	}

	// Description match
	descLower := strings.ToLower(server.Description)
	if strings.Contains(descLower, query) {
		words := strings.Fields(query)
		matchedWords := 0
		for _, word := range words {
			if strings.Contains(descLower, word) {
				matchedWords++
			}
		}
		score := 0.7 * (float64(matchedWords) / float64(len(words)))
		if score > maxScore {
			maxScore = score
			matchType = "description"
		}
	}

	// Category match
	if strings.Contains(strings.ToLower(server.Category), query) ||
		strings.Contains(strings.ToLower(server.Type), query) {
		score := 0.6
		if score > maxScore {
			maxScore = score
			matchType = "category"
		}
	}

	// Tool name/description match (MCP Tool Search)
	for _, tool := range server.Tools {
		toolNameLower := strings.ToLower(tool.Name)
		if strings.Contains(toolNameLower, query) {
			score := 0.75 * (float64(len(query)) / float64(len(toolNameLower)))
			if score > maxScore {
				maxScore = score
				matchType = "tool_name"
			}
		}

		toolDescLower := strings.ToLower(tool.Description)
		if strings.Contains(toolDescLower, query) {
			score := 0.65
			if score > maxScore {
				maxScore = score
				matchType = "tool_description"
			}
		}
	}

	// Capability match
	for _, cap := range server.Capabilities {
		if strings.Contains(strings.ToLower(cap), query) {
			score := 0.55
			if score > maxScore {
				maxScore = score
				matchType = "capability"
			}
		}
	}

	// Fuzzy match using Levenshtein distance
	if opts.FuzzyMatch && maxScore < opts.MinScore {
		nameDist := levenshteinDistance(query, nameLower)
		maxLen := math.Max(float64(len(query)), float64(len(nameLower)))
		similarity := 1.0 - (float64(nameDist) / maxLen)
		if similarity > 0.6 {
			score := similarity * 0.5
			if score > maxScore {
				maxScore = score
				matchType = "fuzzy"
			}
		}
	}

	return maxScore, matchType
}

// levenshteinDistance calculates edit distance for fuzzy matching
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,
				min(matrix[i][j-1]+1, matrix[i-1][j-1]+cost),
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// PRE-DEFINED MCP SERVERS WITH TOOL SEARCH COMPATIBLE TOOLS
// =============================================================================

var predefinedServers = []ProtocolServer{
	// MCP Core Servers
	{
		Name: "helixagent-mcp", Type: "mcp", URL: "http://helixagent-mcp:9100",
		Description: "HelixAgent MCP Server - Central access to all HelixAgent features",
		Category:    "core", Official: true, Supported: true,
		Capabilities: []string{"debate", "ensemble", "task", "rag", "memory", "providers"},
		Tools: []*ToolSchema{
			{Name: "helix_debate", Description: "Start AI debate with ensemble", Category: "ai", RequiredFields: []string{"topic"}},
			{Name: "helix_ensemble", Description: "Query AI ensemble", Category: "ai", RequiredFields: []string{"prompt"}},
			{Name: "helix_task", Description: "Create background task", Category: "workflow", RequiredFields: []string{"command"}},
			{Name: "helix_rag", Description: "RAG retrieval", Category: "rag", RequiredFields: []string{"query"}},
			{Name: "helix_memory", Description: "Memory operations", Category: "memory", RequiredFields: []string{"operation"}},
		},
	},
	{
		Name: "mcp-manager", Type: "mcp", URL: "http://mcp-manager:9000",
		Description: "MCP Server Manager - Orchestrates all MCP servers",
		Category:    "core", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "list_servers", Description: "List all MCP servers", Category: "management"},
			{Name: "server_status", Description: "Get server status", Category: "management", RequiredFields: []string{"server_name"}},
			{Name: "restart_server", Description: "Restart an MCP server", Category: "management", RequiredFields: []string{"server_name"}},
		},
	},
	{
		Name: "mcp-filesystem", Type: "mcp", URL: "http://mcp-filesystem:3000",
		Description: "Secure file operations",
		Category:    "filesystem", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "read_file", Description: "Read contents of a file", Category: "filesystem", RequiredFields: []string{"path"}, Aliases: []string{"read"}},
			{Name: "write_file", Description: "Write content to a file", Category: "filesystem", RequiredFields: []string{"path", "content"}, Aliases: []string{"write"}},
			{Name: "list_directory", Description: "List files in a directory", Category: "filesystem", RequiredFields: []string{"path"}, Aliases: []string{"ls", "dir"}},
			{Name: "search_files", Description: "Search for files", Category: "filesystem", RequiredFields: []string{"pattern"}, Aliases: []string{"find", "glob"}},
			{Name: "get_file_info", Description: "Get file metadata", Category: "filesystem", RequiredFields: []string{"path"}},
		},
	},
	{
		Name: "mcp-git", Type: "mcp", URL: "http://mcp-git:3000",
		Description: "Git repository operations",
		Category:    "version_control", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "git_clone", Description: "Clone a repository", Category: "version_control", RequiredFields: []string{"url"}},
			{Name: "git_commit", Description: "Commit changes", Category: "version_control", RequiredFields: []string{"message"}},
			{Name: "git_push", Description: "Push to remote", Category: "version_control"},
			{Name: "git_pull", Description: "Pull from remote", Category: "version_control"},
			{Name: "git_status", Description: "Show working tree status", Category: "version_control"},
			{Name: "git_diff", Description: "Show changes", Category: "version_control"},
			{Name: "git_log", Description: "Show commit history", Category: "version_control"},
			{Name: "git_branch", Description: "Branch operations", Category: "version_control"},
		},
	},
	{
		Name: "mcp-memory", Type: "mcp", URL: "http://mcp-memory:3000",
		Description: "Knowledge graph memory with persistent storage",
		Category:    "memory", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "store_memory", Description: "Store information in memory", Category: "memory", RequiredFields: []string{"key", "value"}},
			{Name: "retrieve_memory", Description: "Retrieve information from memory", Category: "memory", RequiredFields: []string{"key"}},
			{Name: "search_memory", Description: "Search memory for relevant information", Category: "memory", RequiredFields: []string{"query"}},
			{Name: "delete_memory", Description: "Delete memory entry", Category: "memory", RequiredFields: []string{"key"}},
			{Name: "list_memories", Description: "List all memory entries", Category: "memory"},
		},
	},
	{
		Name: "mcp-fetch-server", Type: "mcp", URL: "http://mcp-fetch-server:3000",
		Description: "Web content fetching and processing",
		Category:    "web", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "fetch_url", Description: "Fetch content from URL", Category: "web", RequiredFields: []string{"url"}},
			{Name: "extract_text", Description: "Extract text from HTML", Category: "web", RequiredFields: []string{"url"}},
			{Name: "download_file", Description: "Download file from URL", Category: "web", RequiredFields: []string{"url", "path"}},
			{Name: "fetch_json", Description: "Fetch and parse JSON", Category: "web", RequiredFields: []string{"url"}},
		},
	},
	{
		Name: "mcp-time", Type: "mcp", URL: "http://mcp-time:3000",
		Description: "Timezone and time operations",
		Category:    "utility", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "get_time", Description: "Get current time", Category: "utility"},
			{Name: "convert_timezone", Description: "Convert time between timezones", Category: "utility", RequiredFields: []string{"time", "from_tz", "to_tz"}},
			{Name: "format_date", Description: "Format date/time", Category: "utility", RequiredFields: []string{"timestamp", "format"}},
			{Name: "parse_date", Description: "Parse date string", Category: "utility", RequiredFields: []string{"date_string"}},
		},
	},
	{
		Name: "mcp-sqlite", Type: "mcp", URL: "http://mcp-sqlite:3000",
		Description: "SQLite database operations",
		Category:    "database", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "query", Description: "Execute SQL query", Category: "database", RequiredFields: []string{"sql"}},
			{Name: "execute", Description: "Execute SQL statement", Category: "database", RequiredFields: []string{"sql"}},
			{Name: "create_table", Description: "Create database table", Category: "database", RequiredFields: []string{"table_name", "columns"}},
			{Name: "list_tables", Description: "List all tables", Category: "database"},
			{Name: "describe_table", Description: "Get table schema", Category: "database", RequiredFields: []string{"table_name"}},
		},
	},
	{
		Name: "mcp-postgres", Type: "mcp", URL: "http://mcp-postgres:3000",
		Description: "PostgreSQL database operations",
		Category:    "database", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "pg_query", Description: "Execute PostgreSQL query", Category: "database", RequiredFields: []string{"sql"}},
			{Name: "pg_execute", Description: "Execute PostgreSQL statement", Category: "database", RequiredFields: []string{"sql"}},
			{Name: "pg_list_tables", Description: "List all tables", Category: "database"},
			{Name: "pg_describe", Description: "Describe table structure", Category: "database", RequiredFields: []string{"table_name"}},
		},
	},
	{
		Name: "mcp-puppeteer", Type: "mcp", URL: "http://mcp-puppeteer:3000",
		Description: "Browser automation with Puppeteer",
		Category:    "automation", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "navigate", Description: "Navigate to URL", Category: "automation", RequiredFields: []string{"url"}},
			{Name: "click", Description: "Click element", Category: "automation", RequiredFields: []string{"selector"}},
			{Name: "type", Description: "Type text into element", Category: "automation", RequiredFields: []string{"selector", "text"}},
			{Name: "screenshot", Description: "Take screenshot", Category: "automation"},
			{Name: "evaluate", Description: "Execute JavaScript", Category: "automation", RequiredFields: []string{"script"}},
			{Name: "wait_for", Description: "Wait for element/condition", Category: "automation", RequiredFields: []string{"selector"}},
		},
	},
	{
		Name: "mcp-sequential-thinking", Type: "mcp", URL: "http://mcp-sequential-thinking:3000",
		Description: "Step-by-step reasoning and problem solving",
		Category:    "reasoning", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "think_step", Description: "Process one reasoning step", Category: "reasoning", RequiredFields: []string{"thought"}},
			{Name: "analyze_problem", Description: "Analyze a problem", Category: "reasoning", RequiredFields: []string{"problem"}},
			{Name: "generate_plan", Description: "Generate solution plan", Category: "reasoning", RequiredFields: []string{"goal"}},
		},
	},
	// MCP Integration Servers
	{
		Name: "mcp-ai-experiment-logger", Type: "mcp", URL: "http://mcp-ai-experiment-logger:3000",
		Description: "AI experiment logging and tracking",
		Category:    "ai", Supported: true,
		Tools: []*ToolSchema{
			{Name: "log_experiment", Description: "Log an AI experiment", Category: "ai", RequiredFields: []string{"name", "parameters"}},
			{Name: "search_experiments", Description: "Search past experiments", Category: "ai", RequiredFields: []string{"query"}},
			{Name: "generate_report", Description: "Generate experiment report", Category: "ai", RequiredFields: []string{"experiment_id"}},
			{Name: "compare_experiments", Description: "Compare two experiments", Category: "ai", RequiredFields: []string{"exp_id_1", "exp_id_2"}},
		},
	},
	{
		Name: "mcp-api-debugger", Type: "mcp", URL: "http://mcp-api-debugger:3000",
		Description: "API debugging and testing",
		Category:    "development", Supported: true,
		Tools: []*ToolSchema{
			{Name: "debug_request", Description: "Debug HTTP request", Category: "development", RequiredFields: []string{"url", "method"}},
			{Name: "analyze_response", Description: "Analyze HTTP response", Category: "development", RequiredFields: []string{"response"}},
			{Name: "test_endpoint", Description: "Test API endpoint", Category: "development", RequiredFields: []string{"url"}},
			{Name: "mock_response", Description: "Create mock response", Category: "development", RequiredFields: []string{"schema"}},
		},
	},
	{
		Name: "mcp-workflow", Type: "mcp", URL: "http://mcp-workflow:3000",
		Description: "Workflow orchestration",
		Category:    "workflow", Supported: true,
		Tools: []*ToolSchema{
			{Name: "create_workflow", Description: "Create new workflow", Category: "workflow", RequiredFields: []string{"name", "steps"}},
			{Name: "execute_workflow", Description: "Execute workflow", Category: "workflow", RequiredFields: []string{"workflow_id"}},
			{Name: "get_workflow_status", Description: "Get workflow status", Category: "workflow", RequiredFields: []string{"workflow_id"}},
			{Name: "list_workflows", Description: "List all workflows", Category: "workflow"},
		},
	},
	// MCP External Service Adapters
	{
		Name: "mcp-slack", Type: "mcp", URL: "http://mcp-slack:3000",
		Description: "Slack workspace messaging",
		Category:    "communication", AuthType: "token", Supported: true,
		Tools: []*ToolSchema{
			{Name: "send_message", Description: "Send Slack message", Category: "communication", RequiredFields: []string{"channel", "text"}},
			{Name: "list_channels", Description: "List Slack channels", Category: "communication"},
			{Name: "search_messages", Description: "Search Slack messages", Category: "communication", RequiredFields: []string{"query"}},
			{Name: "upload_file", Description: "Upload file to Slack", Category: "communication", RequiredFields: []string{"channel", "file"}},
		},
	},
	{
		Name: "mcp-github", Type: "mcp", URL: "http://mcp-github:3000",
		Description: "GitHub repositories and issues",
		Category:    "version_control", AuthType: "token", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "create_issue", Description: "Create GitHub issue", Category: "version_control", RequiredFields: []string{"repo", "title"}},
			{Name: "create_pr", Description: "Create pull request", Category: "version_control", RequiredFields: []string{"repo", "title", "head", "base"}},
			{Name: "list_repos", Description: "List repositories", Category: "version_control"},
			{Name: "search_code", Description: "Search code in GitHub", Category: "version_control", RequiredFields: []string{"query"}},
			{Name: "get_file", Description: "Get file contents", Category: "version_control", RequiredFields: []string{"repo", "path"}},
		},
	},
	{
		Name: "mcp-linear", Type: "mcp", URL: "http://mcp-linear:3000",
		Description: "Linear issue tracking",
		Category:    "productivity", AuthType: "api_key", Supported: true,
		Tools: []*ToolSchema{
			{Name: "create_issue", Description: "Create Linear issue", Category: "productivity", RequiredFields: []string{"title", "team_id"}},
			{Name: "update_issue", Description: "Update Linear issue", Category: "productivity", RequiredFields: []string{"issue_id"}},
			{Name: "list_issues", Description: "List Linear issues", Category: "productivity"},
			{Name: "search_issues", Description: "Search Linear issues", Category: "productivity", RequiredFields: []string{"query"}},
		},
	},
	{
		Name: "mcp-notion", Type: "mcp", URL: "http://mcp-notion:3000",
		Description: "Notion workspace management",
		Category:    "productivity", AuthType: "api_key", Supported: true,
		Tools: []*ToolSchema{
			{Name: "create_page", Description: "Create Notion page", Category: "productivity", RequiredFields: []string{"parent_id", "title"}},
			{Name: "update_page", Description: "Update Notion page", Category: "productivity", RequiredFields: []string{"page_id"}},
			{Name: "search", Description: "Search Notion", Category: "productivity", RequiredFields: []string{"query"}},
			{Name: "query_database", Description: "Query Notion database", Category: "productivity", RequiredFields: []string{"database_id"}},
		},
	},
	{
		Name: "mcp-brave-search", Type: "mcp", URL: "http://mcp-brave-search:3000",
		Description: "Brave Search web search",
		Category:    "search", AuthType: "api_key", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "web_search", Description: "Search the web", Category: "search", RequiredFields: []string{"query"}},
			{Name: "local_search", Description: "Search local businesses", Category: "search", RequiredFields: []string{"query", "location"}},
			{Name: "news_search", Description: "Search news articles", Category: "search", RequiredFields: []string{"query"}},
			{Name: "image_search", Description: "Search images", Category: "search", RequiredFields: []string{"query"}},
		},
	},
	{
		Name: "mcp-sentry", Type: "mcp", URL: "http://mcp-sentry:3000",
		Description: "Sentry error tracking",
		Category:    "analytics", AuthType: "api_key", Official: true, Supported: true,
		Tools: []*ToolSchema{
			{Name: "list_issues", Description: "List Sentry issues", Category: "analytics", RequiredFields: []string{"project"}},
			{Name: "get_issue", Description: "Get issue details", Category: "analytics", RequiredFields: []string{"issue_id"}},
			{Name: "resolve_issue", Description: "Resolve an issue", Category: "analytics", RequiredFields: []string{"issue_id"}},
			{Name: "get_events", Description: "Get error events", Category: "analytics", RequiredFields: []string{"issue_id"}},
		},
	},
	// MCP Vector/AI Servers
	{
		Name: "mcp-chroma", Type: "mcp", URL: "http://mcp-chroma:3000",
		Description: "ChromaDB vector operations",
		Category:    "vector", Supported: true,
		Tools: []*ToolSchema{
			{Name: "add", Description: "Add vectors to collection", Category: "vector", RequiredFields: []string{"collection", "documents"}},
			{Name: "query", Description: "Query vectors", Category: "vector", RequiredFields: []string{"collection", "query_text"}},
			{Name: "delete", Description: "Delete vectors", Category: "vector", RequiredFields: []string{"collection", "ids"}},
			{Name: "update", Description: "Update vectors", Category: "vector", RequiredFields: []string{"collection", "ids", "documents"}},
		},
	},
	{
		Name: "mcp-qdrant", Type: "mcp", URL: "http://mcp-qdrant:3000",
		Description: "Qdrant vector operations",
		Category:    "vector", Supported: true,
		Tools: []*ToolSchema{
			{Name: "upsert", Description: "Upsert vectors", Category: "vector", RequiredFields: []string{"collection", "points"}},
			{Name: "search", Description: "Search vectors", Category: "vector", RequiredFields: []string{"collection", "vector"}},
			{Name: "delete", Description: "Delete vectors", Category: "vector", RequiredFields: []string{"collection", "ids"}},
			{Name: "scroll", Description: "Scroll through vectors", Category: "vector", RequiredFields: []string{"collection"}},
		},
	},
	{
		Name: "mcp-huggingface", Type: "mcp", URL: "http://mcp-huggingface:3000",
		Description: "HuggingFace models and datasets",
		Category:    "ai", AuthType: "api_key", Supported: true,
		Tools: []*ToolSchema{
			{Name: "list_models", Description: "List HuggingFace models", Category: "ai"},
			{Name: "inference", Description: "Run model inference", Category: "ai", RequiredFields: []string{"model", "inputs"}},
			{Name: "download_model", Description: "Download model", Category: "ai", RequiredFields: []string{"model_id"}},
			{Name: "list_datasets", Description: "List datasets", Category: "ai"},
		},
	},
	{
		Name: "mcp-replicate", Type: "mcp", URL: "http://mcp-replicate:3000",
		Description: "Replicate ML model hosting",
		Category:    "ai", AuthType: "api_key", Supported: true,
		Tools: []*ToolSchema{
			{Name: "run_model", Description: "Run a model", Category: "ai", RequiredFields: []string{"model", "input"}},
			{Name: "list_models", Description: "List available models", Category: "ai"},
			{Name: "get_prediction", Description: "Get prediction status", Category: "ai", RequiredFields: []string{"prediction_id"}},
		},
	},
	// LSP Servers
	{
		Name: "lsp-ai", Type: "lsp", URL: "http://lsp-ai:5000",
		Description: "AI-powered language server",
		Category:    "code_intelligence", Supported: true,
		Capabilities: []string{"completion", "chat", "code_actions", "diagnostics"},
	},
	{
		Name: "lsp-multi", Type: "lsp", URL: "http://lsp-multi:5001",
		Description: "Multi-language LSP (Go, Rust, Python, TypeScript, C++, Java)",
		Category:    "code_intelligence", Supported: true,
		Capabilities: []string{"completion", "diagnostics", "formatting", "references", "hover"},
	},
	{
		Name: "lsp-manager", Type: "lsp", URL: "http://lsp-manager:5100",
		Description: "LSP Server Manager",
		Category:    "code_intelligence", Supported: true,
	},
	// ACP Servers
	{
		Name: "acp-manager", Type: "acp", URL: "http://acp-manager:9200",
		Description: "Agent Communication Protocol manager",
		Category:    "agent", Supported: true,
		Capabilities: []string{"agent_discovery", "agent_routing", "message_passing"},
	},
	// Embedding Servers
	{
		Name: "embedding-sentence-transformers", Type: "embedding", URL: "http://embedding-sentence-transformers:8016",
		Description: "Sentence Transformers embeddings",
		Category:    "embedding", Supported: true,
		Capabilities: []string{"encode", "batch_encode"},
	},
	{
		Name: "embedding-bge-m3", Type: "embedding", URL: "http://embedding-bge-m3:8017",
		Description: "BGE-M3 multilingual embeddings",
		Category:    "embedding", Supported: true,
		Capabilities: []string{"encode", "batch_encode", "multilingual"},
	},
	// RAG Servers
	{
		Name: "rag-manager", Type: "rag", URL: "http://rag-manager:8030",
		Description: "RAG Pipeline Manager",
		Category:    "rag", Supported: true,
		Capabilities: []string{"retrieve", "rerank", "generate"},
	},
	{
		Name: "rag-reranker", Type: "rag", URL: "http://rag-reranker:8021",
		Description: "Cross-encoder reranking",
		Category:    "rag", Supported: true,
		Capabilities: []string{"rerank", "score"},
	},
	{
		Name: "qdrant", Type: "rag", URL: "http://qdrant:6333",
		Description: "Qdrant vector database",
		Category:    "vector", Supported: true,
		Capabilities: []string{"upsert", "search", "filter"},
	},
}

func init() {
	for _, server := range predefinedServers {
		server.Status = "unknown"
		server.LastCheck = time.Now()
		registry.servers[server.Name] = server
	}
}

// =============================================================================
// HTTP HANDLERS WITH MCP TOOL SEARCH
// =============================================================================

func handleHealth(w http.ResponseWriter, r *http.Request) {
	registry.mu.RLock()
	healthy := 0
	for _, server := range registry.servers {
		if server.Status == "healthy" {
			healthy++
		}
	}
	registry.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"servers": map[string]int{
			"total":   len(registry.servers),
			"healthy": healthy,
		},
	})
}

// handleDiscovery returns all registered servers
func handleDiscovery(w http.ResponseWriter, r *http.Request) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	response := DiscoveryResponse{
		Timestamp: time.Now(),
	}

	for _, server := range registry.servers {
		switch server.Type {
		case "mcp":
			response.MCPServers = append(response.MCPServers, server)
		case "lsp":
			response.LSPServers = append(response.LSPServers, server)
		case "acp":
			response.ACPServers = append(response.ACPServers, server)
		case "embedding":
			response.EmbeddingServers = append(response.EmbeddingServers, server)
		case "rag":
			response.RAGServers = append(response.RAGServers, server)
		}
	}

	response.TotalServers = len(response.MCPServers) + len(response.LSPServers) +
		len(response.ACPServers) + len(response.EmbeddingServers) + len(response.RAGServers)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSearch handles MCP Tool Search queries
func handleSearch(w http.ResponseWriter, r *http.Request) {
	var opts SearchOptions

	if r.Method == "GET" {
		opts.Query = r.URL.Query().Get("q")
		if opts.Query == "" {
			opts.Query = r.URL.Query().Get("query")
		}
		if categories := r.URL.Query().Get("categories"); categories != "" {
			opts.Categories = strings.Split(categories, ",")
		}
		opts.FuzzyMatch = r.URL.Query().Get("fuzzy") == "true"
	} else {
		if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	results := registry.SearchServers(opts)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":     opts.Query,
		"count":     len(results),
		"results":   results,
		"timestamp": time.Now(),
	})
}

// handleMCPServers returns MCP servers with MCP Tool Search
func handleMCPServers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		query = r.URL.Query().Get("query")
	}

	registry.mu.RLock()
	defer registry.mu.RUnlock()

	var servers []ProtocolServer
	if query != "" {
		// Use MCP Tool Search
		results := registry.SearchServers(SearchOptions{
			Query:      query,
			Categories: []string{"mcp"},
			FuzzyMatch: true,
		})
		for _, result := range results {
			if result.Server.Type == "mcp" {
				servers = append(servers, result.Server)
			}
		}
	} else {
		for _, server := range registry.servers {
			if server.Type == "mcp" {
				servers = append(servers, server)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":     len(servers),
		"servers":   servers,
		"timestamp": time.Now(),
	})
}

// handleToolSearch handles tool-specific MCP Tool Search
func handleToolSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		query = r.URL.Query().Get("query")
	}

	if query == "" {
		http.Error(w, "query parameter is required", http.StatusBadRequest)
		return
	}

	registry.mu.RLock()
	defer registry.mu.RUnlock()

	type ToolResult struct {
		ServerName string      `json:"server_name"`
		ServerURL  string      `json:"server_url"`
		Tool       *ToolSchema `json:"tool"`
		Score      float64     `json:"score"`
		MatchType  string      `json:"match_type"`
	}

	var results []ToolResult
	queryLower := strings.ToLower(query)

	for _, server := range registry.servers {
		for _, tool := range server.Tools {
			score, matchType := calculateToolScore(tool, queryLower)
			if score >= 0.1 {
				results = append(results, ToolResult{
					ServerName: server.Name,
					ServerURL:  server.URL,
					Tool:       tool,
					Score:      score,
					MatchType:  matchType,
				})
			}
		}
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit to 50
	if len(results) > 50 {
		results = results[:50]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":     query,
		"count":     len(results),
		"results":   results,
		"timestamp": time.Now(),
	})
}

func calculateToolScore(tool *ToolSchema, query string) (float64, string) {
	var maxScore float64
	var matchType string

	// Exact name match
	if strings.EqualFold(tool.Name, query) {
		return 1.0, "name_exact"
	}

	// Name contains query
	nameLower := strings.ToLower(tool.Name)
	if strings.Contains(nameLower, query) {
		score := 0.9 * (float64(len(query)) / float64(len(nameLower)))
		if score > maxScore {
			maxScore = score
			matchType = "name"
		}
	}

	// Alias match
	for _, alias := range tool.Aliases {
		if strings.EqualFold(alias, query) {
			return 0.95, "alias_exact"
		}
		if strings.Contains(strings.ToLower(alias), query) {
			score := 0.85
			if score > maxScore {
				maxScore = score
				matchType = "alias"
			}
		}
	}

	// Description match
	if strings.Contains(strings.ToLower(tool.Description), query) {
		score := 0.7
		if score > maxScore {
			maxScore = score
			matchType = "description"
		}
	}

	// Category match
	if strings.Contains(strings.ToLower(tool.Category), query) {
		score := 0.6
		if score > maxScore {
			maxScore = score
			matchType = "category"
		}
	}

	return maxScore, matchType
}

// handleLSPServers returns LSP servers
func handleLSPServers(w http.ResponseWriter, r *http.Request) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	var servers []ProtocolServer
	for _, server := range registry.servers {
		if server.Type == "lsp" {
			servers = append(servers, server)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":     len(servers),
		"servers":   servers,
		"timestamp": time.Now(),
	})
}

// handleACPServers returns ACP servers
func handleACPServers(w http.ResponseWriter, r *http.Request) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	var servers []ProtocolServer
	for _, server := range registry.servers {
		if server.Type == "acp" {
			servers = append(servers, server)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":     len(servers),
		"servers":   servers,
		"timestamp": time.Now(),
	})
}

// handleEmbeddingServers returns embedding servers
func handleEmbeddingServers(w http.ResponseWriter, r *http.Request) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	var servers []ProtocolServer
	for _, server := range registry.servers {
		if server.Type == "embedding" {
			servers = append(servers, server)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":     len(servers),
		"servers":   servers,
		"timestamp": time.Now(),
	})
}

// handleRAGServers returns RAG servers
func handleRAGServers(w http.ResponseWriter, r *http.Request) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	var servers []ProtocolServer
	for _, server := range registry.servers {
		if server.Type == "rag" {
			servers = append(servers, server)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":     len(servers),
		"servers":   servers,
		"timestamp": time.Now(),
	})
}

// handleRegister allows dynamic registration of servers
func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var server ProtocolServer
	if err := json.NewDecoder(r.Body).Decode(&server); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	server.Status = "unknown"
	server.LastCheck = time.Now()

	registry.mu.Lock()
	registry.servers[server.Name] = server
	registry.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Server %s registered", server.Name),
		"server":  server,
	})
}

// healthChecker periodically checks all servers
func healthChecker() {
	interval := 60 * time.Second
	ticker := time.NewTicker(interval)
	for range ticker.C {
		registry.mu.Lock()
		for name, server := range registry.servers {
			checkServerHealth(&server)
			registry.servers[name] = server
		}
		registry.mu.Unlock()
	}
}

func checkServerHealth(server *ProtocolServer) {
	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := server.URL + "/health"

	resp, err := client.Get(healthURL)
	if err != nil {
		server.Status = "unhealthy"
		server.LastCheck = time.Now()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		server.Status = "healthy"
	} else {
		server.Status = "unhealthy"
	}
	server.LastCheck = time.Now()
}

func main() {
	port := os.Getenv("DISCOVERY_PORT")
	if port == "" {
		port = "9300"
	}

	// Start background health checker
	go healthChecker()

	// API routes
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/v1/discovery", handleDiscovery)
	http.HandleFunc("/v1/discovery/mcp", handleMCPServers)
	http.HandleFunc("/v1/discovery/lsp", handleLSPServers)
	http.HandleFunc("/v1/discovery/acp", handleACPServers)
	http.HandleFunc("/v1/discovery/embedding", handleEmbeddingServers)
	http.HandleFunc("/v1/discovery/rag", handleRAGServers)
	http.HandleFunc("/v1/register", handleRegister)

	// MCP Tool Search endpoints
	http.HandleFunc("/v1/search", handleSearch)
	http.HandleFunc("/v1/search/tools", handleToolSearch)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":    "HelixAgent Protocol Discovery Service",
			"version": "1.0.0",
			"features": []string{
				"MCP Tool Search Technology",
				"Fuzzy Matching",
				"Category Filtering",
				"Dynamic Registration",
			},
			"endpoints": []string{
				"/health",
				"/v1/discovery",
				"/v1/discovery/mcp",
				"/v1/discovery/lsp",
				"/v1/discovery/acp",
				"/v1/discovery/embedding",
				"/v1/discovery/rag",
				"/v1/search",
				"/v1/search/tools",
				"/v1/register",
			},
		})
	})

	log.Printf("Protocol Discovery Service with MCP Tool Search starting on port %s", port)
	log.Printf("Registered %d protocol servers with %d tools", len(predefinedServers), countTools())

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func countTools() int {
	count := 0
	for _, server := range predefinedServers {
		count += len(server.Tools)
	}
	return count
}
