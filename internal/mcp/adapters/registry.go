// Package adapters provides the MCP server adapter registry.
// This file registers all available MCP server adapters for HelixAgent.
package adapters

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"digital.vasic.concurrency/pkg/safe"
)

// ServerInfo contains information about an MCP server.
type ServerInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
}

// ToolDefinition defines an MCP tool.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in a tool result.
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

// MCPAdapter defines the interface for MCP server adapters.
type MCPAdapter interface {
	GetServerInfo() ServerInfo
	ListTools() []ToolDefinition
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error)
}

// AdapterCategory categorizes MCP adapters.
type AdapterCategory string

const (
	CategoryDatabase       AdapterCategory = "database"
	CategoryStorage        AdapterCategory = "storage"
	CategoryVersionControl AdapterCategory = "version_control"
	CategoryProductivity   AdapterCategory = "productivity"
	CategoryCommunication  AdapterCategory = "communication"
	CategorySearch         AdapterCategory = "search"
	CategoryAutomation     AdapterCategory = "automation"
	CategoryInfrastructure AdapterCategory = "infrastructure"
	CategoryAnalytics      AdapterCategory = "analytics"
	CategoryAI             AdapterCategory = "ai"
	CategoryUtility        AdapterCategory = "utility"
	CategoryDesign         AdapterCategory = "design"
	CategoryCollaboration  AdapterCategory = "collaboration"
)

// AdapterMetadata contains metadata about an adapter.
type AdapterMetadata struct {
	Name        string          `json:"name"`
	Category    AdapterCategory `json:"category"`
	Description string          `json:"description"`
	AuthType    string          `json:"authType"` // "api_key", "oauth2", "none", "token"
	DocsURL     string          `json:"docsUrl"`
	Official    bool            `json:"official"`  // Official MCP server
	Supported   bool            `json:"supported"` // Fully supported by HelixAgent
}

// AdapterFactory is a function that creates an MCPAdapter on demand.
// It is called at most once per adapter, on first access.
type AdapterFactory func() (MCPAdapter, error)

// lazyAdapter holds a factory and its sync.Once for deferred initialization.
type lazyAdapter struct {
	factory AdapterFactory
	once    sync.Once
	adapter MCPAdapter
	initErr error
}

// get returns the initialized adapter, calling the factory on first access.
func (la *lazyAdapter) get() (MCPAdapter, error) {
	la.once.Do(func() {
		la.adapter, la.initErr = la.factory()
	})
	return la.adapter, la.initErr
}

// AdapterRegistry manages MCP adapters.
//
// Concurrency model (CONST-029): adapters/lazyAdapters/metadata → 3×
// *safe.Store. mu dropped — each method is a single Put/Get/Delete or
// a Range/Snapshot iteration. lazyAdapter retains its own sync.Once
// for factory-fires-once semantics (not Pattern-A).
type AdapterRegistry struct {
	adapters     *safe.Store[string, MCPAdapter]
	lazyAdapters *safe.Store[string, *lazyAdapter]
	metadata     *safe.Store[string, AdapterMetadata]
}

// NewAdapterRegistry creates a new adapter registry.
func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{
		adapters:     safe.NewStore[string, MCPAdapter](),
		lazyAdapters: safe.NewStore[string, *lazyAdapter](),
		metadata:     safe.NewStore[string, AdapterMetadata](),
	}
}

// Register registers an adapter eagerly (immediately available).
func (r *AdapterRegistry) Register(name string, adapter MCPAdapter, metadata AdapterMetadata) {
	r.adapters.Put(name, adapter)
	r.metadata.Put(name, metadata)
	// Remove any lazy registration for the same name to avoid confusion
	r.lazyAdapters.Delete(name)
}

// RegisterLazy registers an adapter factory for deferred initialization.
// The factory is called at most once, when the adapter is first accessed
// via Get or CallTool. This avoids initialization overhead for adapters
// that may never be used during a session.
func (r *AdapterRegistry) RegisterLazy(name string, factory AdapterFactory, metadata AdapterMetadata) {
	r.lazyAdapters.Put(name, &lazyAdapter{factory: factory})
	r.metadata.Put(name, metadata)
	// Remove any eager registration for the same name
	r.adapters.Delete(name)
}

// Get retrieves an adapter by name. For lazily registered adapters,
// this triggers initialization on the first call.
func (r *AdapterRegistry) Get(name string) (MCPAdapter, bool) {
	// Check eagerly registered adapters first
	if adapter, ok := r.adapters.Get(name); ok {
		return adapter, true
	}
	// Check lazily registered adapters
	la, ok := r.lazyAdapters.Get(name)
	if !ok {
		return nil, false
	}
	adapter, err := la.get()
	if err != nil {
		return nil, false
	}
	return adapter, true
}

// GetMetadata retrieves adapter metadata.
func (r *AdapterRegistry) GetMetadata(name string) (AdapterMetadata, bool) {
	return r.metadata.Get(name)
}

// List returns all registered adapter names (both eager and lazy).
func (r *AdapterRegistry) List() []string {
	seen := make(map[string]struct{}, r.adapters.Len()+r.lazyAdapters.Len())
	names := make([]string, 0, r.adapters.Len()+r.lazyAdapters.Len())
	r.adapters.Range(func(name string, _ MCPAdapter) bool {
		names = append(names, name)
		seen[name] = struct{}{}
		return true
	})
	r.lazyAdapters.Range(func(name string, _ *lazyAdapter) bool {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
		}
		return true
	})
	return names
}

// ListByCategory returns adapters in a category.
func (r *AdapterRegistry) ListByCategory(category AdapterCategory) []string {
	var names []string
	r.metadata.Range(func(name string, meta AdapterMetadata) bool {
		if meta.Category == category {
			names = append(names, name)
		}
		return true
	})
	return names
}

// ListAll returns all adapter metadata.
func (r *AdapterRegistry) ListAll() []AdapterMetadata {
	result := make([]AdapterMetadata, 0, r.metadata.Len())
	r.metadata.Range(func(_ string, meta AdapterMetadata) bool {
		result = append(result, meta)
		return true
	})
	return result
}

// CallTool calls a tool on an adapter.
func (r *AdapterRegistry) CallTool(ctx context.Context, adapterName, toolName string, args map[string]interface{}) (*ToolResult, error) {
	adapter, ok := r.Get(adapterName)
	if !ok {
		return nil, fmt.Errorf("adapter not found: %s", adapterName)
	}
	return adapter.CallTool(ctx, toolName, args)
}

// AvailableAdapters lists all available MCP server adapters with their metadata.
var AvailableAdapters = []AdapterMetadata{
	// Database
	{Name: "postgresql", Category: CategoryDatabase, Description: "PostgreSQL database operations", AuthType: "connection_string", DocsURL: "https://github.com/modelcontextprotocol/servers/tree/main/src/postgres", Official: true, Supported: true},
	{Name: "sqlite", Category: CategoryDatabase, Description: "SQLite database operations", AuthType: "file_path", DocsURL: "https://github.com/modelcontextprotocol/servers/tree/main/src/sqlite", Official: true, Supported: true},
	{Name: "mongodb", Category: CategoryDatabase, Description: "MongoDB NoSQL database", AuthType: "connection_string", DocsURL: "https://github.com/modelcontextprotocol/servers", Official: false, Supported: true},
	{Name: "redis", Category: CategoryDatabase, Description: "Redis cache and data store", AuthType: "connection_string", DocsURL: "https://github.com/modelcontextprotocol/servers", Official: false, Supported: true},
	{Name: "neon", Category: CategoryDatabase, Description: "Neon serverless Postgres", AuthType: "api_key", DocsURL: "https://neon.tech/docs", Official: false, Supported: true},
	{Name: "supabase", Category: CategoryDatabase, Description: "Supabase backend-as-a-service", AuthType: "api_key", DocsURL: "https://supabase.com/docs", Official: false, Supported: true},

	// Storage
	{Name: "aws-s3", Category: CategoryStorage, Description: "AWS S3 object storage", AuthType: "api_key", DocsURL: "https://docs.aws.amazon.com/s3/", Official: false, Supported: true},
	{Name: "google-drive", Category: CategoryStorage, Description: "Google Drive file storage", AuthType: "oauth2", DocsURL: "https://developers.google.com/drive", Official: false, Supported: true},
	{Name: "dropbox", Category: CategoryStorage, Description: "Dropbox file storage", AuthType: "oauth2", DocsURL: "https://www.dropbox.com/developers", Official: false, Supported: true},

	// Version Control
	{Name: "github", Category: CategoryVersionControl, Description: "GitHub repositories and issues", AuthType: "token", DocsURL: "https://github.com/modelcontextprotocol/servers/tree/main/src/github", Official: true, Supported: true},
	{Name: "gitlab", Category: CategoryVersionControl, Description: "GitLab repositories and CI/CD", AuthType: "token", DocsURL: "https://docs.gitlab.com/ee/api/", Official: false, Supported: true},
	{Name: "bitbucket", Category: CategoryVersionControl, Description: "Bitbucket repositories", AuthType: "token", DocsURL: "https://developer.atlassian.com/bitbucket/api/2/reference/", Official: false, Supported: true},

	// Productivity
	{Name: "notion", Category: CategoryProductivity, Description: "Notion workspace management", AuthType: "api_key", DocsURL: "https://developers.notion.com/", Official: false, Supported: true},
	{Name: "linear", Category: CategoryProductivity, Description: "Linear issue tracking", AuthType: "api_key", DocsURL: "https://developers.linear.app/", Official: false, Supported: true},
	{Name: "asana", Category: CategoryProductivity, Description: "Asana project and task management", AuthType: "token", DocsURL: "https://developers.asana.com/", Official: false, Supported: true},
	{Name: "jira", Category: CategoryProductivity, Description: "Jira issue tracking and project management", AuthType: "api_key", DocsURL: "https://developer.atlassian.com/cloud/jira/platform/rest/v3/", Official: false, Supported: true},
	{Name: "todoist", Category: CategoryProductivity, Description: "Todoist task management", AuthType: "api_key", DocsURL: "https://developer.todoist.com/", Official: false, Supported: true},
	{Name: "obsidian", Category: CategoryProductivity, Description: "Obsidian note-taking", AuthType: "local", DocsURL: "https://obsidian.md/", Official: false, Supported: true},

	// Communication
	{Name: "slack", Category: CategoryCommunication, Description: "Slack workspace messaging", AuthType: "token", DocsURL: "https://api.slack.com/", Official: true, Supported: true},
	{Name: "discord", Category: CategoryCommunication, Description: "Discord server management", AuthType: "token", DocsURL: "https://discord.com/developers/docs", Official: false, Supported: true},
	{Name: "email", Category: CategoryCommunication, Description: "Email sending and reading", AuthType: "oauth2", DocsURL: "", Official: false, Supported: true},

	// Search
	{Name: "brave-search", Category: CategorySearch, Description: "Brave Search web search", AuthType: "api_key", DocsURL: "https://brave.com/search/api/", Official: true, Supported: true},
	{Name: "exa", Category: CategorySearch, Description: "Exa semantic search", AuthType: "api_key", DocsURL: "https://docs.exa.ai/", Official: false, Supported: true},
	{Name: "google-search", Category: CategorySearch, Description: "Google Custom Search", AuthType: "api_key", DocsURL: "https://developers.google.com/custom-search", Official: false, Supported: true},

	// Automation
	{Name: "puppeteer", Category: CategoryAutomation, Description: "Browser automation", AuthType: "none", DocsURL: "https://pptr.dev/", Official: true, Supported: true},
	{Name: "browserbase", Category: CategoryAutomation, Description: "Cloud browser automation", AuthType: "api_key", DocsURL: "https://docs.browserbase.com/", Official: false, Supported: true},
	{Name: "playwright", Category: CategoryAutomation, Description: "Cross-browser automation", AuthType: "none", DocsURL: "https://playwright.dev/", Official: false, Supported: true},

	// Infrastructure
	{Name: "docker", Category: CategoryInfrastructure, Description: "Docker container management", AuthType: "socket", DocsURL: "https://docs.docker.com/engine/api/", Official: false, Supported: true},
	{Name: "kubernetes", Category: CategoryInfrastructure, Description: "Kubernetes cluster management", AuthType: "kubeconfig", DocsURL: "https://kubernetes.io/docs/reference/using-api/", Official: false, Supported: true},
	{Name: "cloudflare", Category: CategoryInfrastructure, Description: "Cloudflare Workers and KV", AuthType: "api_key", DocsURL: "https://developers.cloudflare.com/", Official: false, Supported: true},
	{Name: "vercel", Category: CategoryInfrastructure, Description: "Vercel deployments", AuthType: "token", DocsURL: "https://vercel.com/docs/rest-api", Official: false, Supported: true},

	// Analytics & Observability
	{Name: "sentry", Category: CategoryAnalytics, Description: "Error tracking and monitoring", AuthType: "api_key", DocsURL: "https://docs.sentry.io/api/", Official: true, Supported: true},
	{Name: "axiom", Category: CategoryAnalytics, Description: "Log management and observability", AuthType: "api_key", DocsURL: "https://axiom.co/docs", Official: false, Supported: true},
	{Name: "datadog", Category: CategoryAnalytics, Description: "Monitoring and analytics", AuthType: "api_key", DocsURL: "https://docs.datadoghq.com/api/", Official: false, Supported: true},

	// AI & ML
	{Name: "everart", Category: CategoryAI, Description: "AI image generation", AuthType: "api_key", DocsURL: "https://everart.ai/", Official: true, Supported: true},
	{Name: "replicate", Category: CategoryAI, Description: "ML model hosting", AuthType: "api_key", DocsURL: "https://replicate.com/docs", Official: false, Supported: true},
	{Name: "huggingface", Category: CategoryAI, Description: "HuggingFace models and datasets", AuthType: "api_key", DocsURL: "https://huggingface.co/docs/api-inference", Official: false, Supported: true},

	// Utility
	{Name: "fetch", Category: CategoryUtility, Description: "HTTP requests and web fetching", AuthType: "none", DocsURL: "https://github.com/modelcontextprotocol/servers/tree/main/src/fetch", Official: true, Supported: true},
	{Name: "time", Category: CategoryUtility, Description: "Time and timezone utilities", AuthType: "none", DocsURL: "", Official: true, Supported: true},
	{Name: "memory", Category: CategoryUtility, Description: "Persistent memory storage", AuthType: "none", DocsURL: "https://github.com/modelcontextprotocol/servers/tree/main/src/memory", Official: true, Supported: true},
	{Name: "filesystem", Category: CategoryUtility, Description: "File system operations", AuthType: "none", DocsURL: "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem", Official: true, Supported: true},
	{Name: "sequential-thinking", Category: CategoryUtility, Description: "Step-by-step reasoning", AuthType: "none", DocsURL: "", Official: true, Supported: true},
	{Name: "e2b", Category: CategoryUtility, Description: "Code sandbox execution", AuthType: "api_key", DocsURL: "https://e2b.dev/docs", Official: false, Supported: true},

	// Payments & Commerce
	{Name: "stripe", Category: CategoryProductivity, Description: "Payment processing", AuthType: "api_key", DocsURL: "https://stripe.com/docs/api", Official: false, Supported: true},
	{Name: "shopify", Category: CategoryProductivity, Description: "E-commerce platform", AuthType: "api_key", DocsURL: "https://shopify.dev/docs/api", Official: false, Supported: true},

	// Maps & Location
	{Name: "google-maps", Category: CategoryUtility, Description: "Google Maps and Places", AuthType: "api_key", DocsURL: "https://developers.google.com/maps", Official: true, Supported: true},

	// Design & UI
	{Name: "figma", Category: CategoryDesign, Description: "Figma design integration for reading and modifying design files", AuthType: "token", DocsURL: "https://www.figma.com/developers/api", Official: false, Supported: true},
	{Name: "svgmaker", Category: CategoryDesign, Description: "AI-powered SVG generation, editing, and optimization", AuthType: "api_key", DocsURL: "https://svgmaker.io/docs", Official: false, Supported: true},
	{Name: "stable-diffusion", Category: CategoryAI, Description: "Stable Diffusion WebUI integration for AI image generation", AuthType: "none", DocsURL: "https://github.com/AUTOMATIC1111/stable-diffusion-webui", Official: false, Supported: true},

	// Collaboration
	{Name: "miro", Category: CategoryCollaboration, Description: "Miro whiteboard collaboration for visual planning", AuthType: "oauth2", DocsURL: "https://developers.miro.com/docs", Official: false, Supported: true},
}

var (
	defaultRegistry     *AdapterRegistry
	defaultRegistryOnce sync.Once

	// DefaultRegistry is the default adapter registry.
	// Prefer GetDefaultRegistry() for new code.
	DefaultRegistry *AdapterRegistry
)

func init() {
	// Eagerly populate DefaultRegistry for backward compatibility.
	// The registry only copies static metadata — no I/O or heavy work.
	GetDefaultRegistry()
}

// GetDefaultRegistry returns the singleton default adapter registry,
// populating it with all available adapters on the first call.
// This uses sync.Once for thread-safe lazy initialization — the registry
// is only created when first accessed, not at package load time.
func GetDefaultRegistry() *AdapterRegistry {
	defaultRegistryOnce.Do(func() {
		r := NewAdapterRegistry()
		for _, meta := range AvailableAdapters {
			r.metadata.Put(meta.Name, meta)
		}
		defaultRegistry = r
		// Keep the deprecated package-level var in sync for backward compatibility.
		DefaultRegistry = r
	})
	return defaultRegistry
}

// InitializeDefaultRegistry populates the default registry with all available adapters.
// Deprecated: GetDefaultRegistry() is now initialized lazily via sync.Once.
// This function is retained for backward compatibility.
func InitializeDefaultRegistry() {
	_ = GetDefaultRegistry()
}

// GetAdapterCount returns the total number of available adapters.
func GetAdapterCount() int {
	return len(AvailableAdapters)
}

// GetSupportedAdapters returns adapters that are fully supported.
func GetSupportedAdapters() []AdapterMetadata {
	var supported []AdapterMetadata
	for _, meta := range AvailableAdapters {
		if meta.Supported {
			supported = append(supported, meta)
		}
	}
	return supported
}

// GetOfficialAdapters returns official MCP adapters.
func GetOfficialAdapters() []AdapterMetadata {
	var official []AdapterMetadata
	for _, meta := range AvailableAdapters {
		if meta.Official {
			official = append(official, meta)
		}
	}
	return official
}

// AdapterSearchResult represents a search result with relevance score
type AdapterSearchResult struct {
	Adapter   AdapterMetadata `json:"adapter"`
	Score     float64         `json:"score"`
	MatchType string          `json:"match_type"` // "name", "description", "category"
}

// AdapterSearchOptions configures the adapter search behavior
type AdapterSearchOptions struct {
	Query      string            `json:"query"`
	Categories []AdapterCategory `json:"categories,omitempty"`
	AuthTypes  []string          `json:"auth_types,omitempty"`
	Official   *bool             `json:"official,omitempty"`
	Supported  *bool             `json:"supported,omitempty"`
	MaxResults int               `json:"max_results,omitempty"`
	MinScore   float64           `json:"min_score,omitempty"`
}

// Search searches adapters with the given options
func (r *AdapterRegistry) Search(opts AdapterSearchOptions) []AdapterSearchResult {
	if opts.MaxResults <= 0 {
		opts.MaxResults = 50
	}
	if opts.MinScore <= 0 {
		opts.MinScore = 0.1
	}

	query := strings.ToLower(opts.Query)
	var results []AdapterSearchResult

	for _, meta := range AvailableAdapters {
		// Apply filters
		if !matchesAdapterFilters(meta, opts) {
			continue
		}

		score, matchType := calculateAdapterScore(meta, query)
		if score >= opts.MinScore {
			results = append(results, AdapterSearchResult{
				Adapter:   meta,
				Score:     score,
				MatchType: matchType,
			})
		}
	}

	// Sort by score descending
	sortAdapterResults(results)

	// Limit results
	if len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}

	return results
}

// matchesAdapterFilters checks if adapter matches filter criteria
func matchesAdapterFilters(meta AdapterMetadata, opts AdapterSearchOptions) bool {
	// Filter by category
	if len(opts.Categories) > 0 {
		found := false
		for _, cat := range opts.Categories {
			if meta.Category == cat {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by auth type
	if len(opts.AuthTypes) > 0 {
		found := false
		for _, authType := range opts.AuthTypes {
			if strings.EqualFold(meta.AuthType, authType) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by official status
	if opts.Official != nil && meta.Official != *opts.Official {
		return false
	}

	// Filter by supported status
	if opts.Supported != nil && meta.Supported != *opts.Supported {
		return false
	}

	return true
}

// calculateAdapterScore calculates relevance score for an adapter
func calculateAdapterScore(meta AdapterMetadata, query string) (float64, string) {
	if query == "" {
		return 1.0, "all"
	}

	var maxScore float64
	var matchType string

	// Exact name match
	if strings.EqualFold(meta.Name, query) {
		return 1.0, "name"
	}

	// Name contains query
	nameLower := strings.ToLower(meta.Name)
	if strings.Contains(nameLower, query) {
		score := 0.9 * (float64(len(query)) / float64(len(nameLower)))
		if score > maxScore {
			maxScore = score
			matchType = "name"
		}
	}

	// Description match
	descLower := strings.ToLower(meta.Description)
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
	if strings.Contains(strings.ToLower(string(meta.Category)), query) {
		score := 0.6
		if score > maxScore {
			maxScore = score
			matchType = "category"
		}
	}

	return maxScore, matchType
}

// sortAdapterResults sorts results by score descending
func sortAdapterResults(results []AdapterSearchResult) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// SearchByCapability searches adapters by capability keywords
func (r *AdapterRegistry) SearchByCapability(capability string) []AdapterMetadata {
	capLower := strings.ToLower(capability)
	var results []AdapterMetadata

	for _, meta := range AvailableAdapters {
		// Check description for capability keywords
		if strings.Contains(strings.ToLower(meta.Description), capLower) {
			results = append(results, meta)
			continue
		}

		// Check category name
		if strings.Contains(strings.ToLower(string(meta.Category)), capLower) {
			results = append(results, meta)
		}
	}

	return results
}

// GetAdapterSuggestions returns adapter suggestions based on partial input
func (r *AdapterRegistry) GetAdapterSuggestions(prefix string, maxSuggestions int) []AdapterMetadata {
	if maxSuggestions <= 0 {
		maxSuggestions = 10
	}

	prefixLower := strings.ToLower(prefix)
	var suggestions []AdapterMetadata

	for _, meta := range AvailableAdapters {
		if strings.HasPrefix(strings.ToLower(meta.Name), prefixLower) {
			suggestions = append(suggestions, meta)
			if len(suggestions) >= maxSuggestions {
				break
			}
		}
	}

	return suggestions
}

// GetAllCategories returns all unique adapter categories
func GetAllCategories() []AdapterCategory {
	return []AdapterCategory{
		CategoryDatabase,
		CategoryStorage,
		CategoryVersionControl,
		CategoryProductivity,
		CategoryCommunication,
		CategorySearch,
		CategoryAutomation,
		CategoryInfrastructure,
		CategoryAnalytics,
		CategoryAI,
		CategoryUtility,
		CategoryDesign,
		CategoryCollaboration,
	}
}

// GetAllAuthTypes returns all unique auth types
func GetAllAuthTypes() []string {
	authTypes := make(map[string]bool)
	for _, meta := range AvailableAdapters {
		authTypes[meta.AuthType] = true
	}

	var result []string
	for authType := range authTypes {
		result = append(result, authType)
	}
	return result
}
