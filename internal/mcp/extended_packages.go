// Package mcp provides extended MCP server package definitions.
package mcp

import "os"

// ExtendedMCPPackages defines all MCP packages including new integrations.
// These extend the StandardMCPPackages with additional servers for:
// - Vector databases (Chroma, Qdrant)
// - Design tools (Figma, Miro)
// - Image generation (Stable Diffusion, Replicate)
// - Development tools (Git, Time, Sequential Thinking)
// - LSP bridges
var ExtendedMCPPackages = []MCPPackage{
	// ==========================================================================
	// CORE REFERENCE SERVERS (Anthropic Official)
	// ==========================================================================
	{
		Name:        "filesystem",
		NPM:         "@modelcontextprotocol/server-filesystem",
		Description: "MCP server for secure filesystem operations with configurable access controls",
		Category:    CategoryCore,
	},
	{
		Name:        "github",
		NPM:         "@modelcontextprotocol/server-github",
		Description: "MCP server for GitHub API operations",
		Category:    CategoryCore,
	},
	{
		Name:        "memory",
		NPM:         "@modelcontextprotocol/server-memory",
		Description: "MCP server for knowledge-graph-based persistent memory system",
		Category:    CategoryCore,
	},
	{
		Name:        "fetch",
		NPM:         "mcp-fetch-server",
		Description: "MCP server for web content fetching and conversion",
		Category:    CategoryCore,
	},
	{
		Name:        "puppeteer",
		NPM:         "@modelcontextprotocol/server-puppeteer",
		Description: "MCP server for browser automation with Puppeteer",
		Category:    CategoryCore,
	},
	{
		Name:        "sqlite",
		NPM:         "mcp-server-sqlite",
		Description: "MCP server for SQLite database operations",
		Category:    CategoryCore,
	},
	{
		Name:        "git",
		NPM:         "mcp-git",
		Description: "MCP server for Git repository operations",
		Category:    CategoryCore,
	},
	{
		Name:        "time",
		NPM:         "@theo.foobar/mcp-time",
		Description: "MCP server for time and timezone conversion",
		Category:    CategoryCore,
	},
	{
		Name:        "sequential-thinking",
		NPM:         "@modelcontextprotocol/server-sequential-thinking",
		Description: "MCP server for dynamic and reflective problem-solving",
		Category:    CategoryCore,
	},
	{
		Name:        "everything",
		NPM:         "@modelcontextprotocol/server-everything",
		Description: "Reference/test server with prompts, resources, and tools",
		Category:    CategoryCore,
	},

	// ==========================================================================
	// VECTOR DATABASE SERVERS
	// ==========================================================================
	{
		Name:        "chroma",
		NPM:         "mcp-server-chroma",
		Description: "MCP server for ChromaDB vector database operations",
		Category:    CategoryVectorDB,
		RequiresEnv: []string{"CHROMA_URL"},
	},
	{
		Name:        "qdrant",
		NPM:         "mcp-server-qdrant",
		Description: "MCP server for Qdrant vector database operations",
		Category:    CategoryVectorDB,
		RequiresEnv: []string{"QDRANT_URL"},
	},
	{
		Name:        "weaviate",
		NPM:         "mcp-server-weaviate",
		Description: "MCP server for Weaviate vector database operations",
		Category:    CategoryVectorDB,
		RequiresEnv: []string{"WEAVIATE_URL"},
	},
	{
		Name:        "pinecone",
		NPM:         "mcp-server-pinecone",
		Description: "MCP server for Pinecone vector database operations",
		Category:    CategoryVectorDB,
		RequiresEnv: []string{"PINECONE_API_KEY"},
	},

	// ==========================================================================
	// DESIGN & UI SERVERS
	// ==========================================================================
	// Fixed 2026-09-03. `mcp-server-figma` RESOLVES (HTTP 200) but is an npm
	// SECURITY HOLDING package: single version 0.0.1-security, description
	// "security holding package", repository "npm/security-holder" — a name
	// npm seized. Every existence check passed it while it installed nothing.
	// Replacement verified live: figma-developer-mcp v0.13.2 (Framelink),
	// repo github.com/GLips/Figma-Context-MCP -> HTTP 200, ~81586
	// downloads/week, community publisher. It reads FIGMA_API_KEY, not the
	// FIGMA_ACCESS_TOKEN this entry declared.
	{
		Name:        "figma",
		NPM:         "figma-developer-mcp",
		Description: "MCP server for Figma design operations",
		Category:    CategoryDesign,
		RequiresEnv: []string{"FIGMA_API_KEY"},
	},

	// Restored 2026-09-03. ce9c4eb9 removed "miro" because `mcp-miro` 404s.
	// Replacement verified live: @k-jarzyna/mcp-miro v1.0.11 (2026-08-18),
	// repo github.com/k-jarzyna/mcp-miro -> HTTP 200, ~229 downloads/week,
	// community publisher (k-jarzyna; npm scope matches the repo owner).
	// Adoption is low - the resolving repository is what carries provenance
	// here, not the download count. Env is MIRO_ACCESS_TOKEN per the package's
	// own README, NOT the MIRO_OAUTH_TOKEN the removed entry declared.
	{
		Name:        "miro",
		NPM:         "@k-jarzyna/mcp-miro",
		Description: "MCP server for Miro whiteboard operations",
		Category:    CategoryDesign,
		RequiresEnv: []string{"MIRO_ACCESS_TOKEN"},
	},

	// ==========================================================================
	// IMAGE GENERATION SERVERS
	// ==========================================================================

	// Restored 2026-09-03. ce9c4eb9 removed "replicate" because
	// `mcp-server-replicate` 404s. Replacement verified live: replicate-mcp
	// v0.9.0, 90 published versions, ~2000 downloads/week, published by
	// Replicate themselves (maintainer `replicatebot`, homepage
	// replicate.com/docs/reference/mcp -> HTTP 200) - VENDOR, not community.
	// The npm README is empty, so the invocation `npx -y replicate-mcp` and
	// the REPLICATE_API_TOKEN env come from Replicate's own docs page rather
	// than from the package; provenance rests on the vendor maintainer +
	// vendor-domain homepage, as the package declares no `repository` field.
	{
		Name:        "replicate",
		NPM:         "replicate-mcp",
		Description: "MCP server for Replicate model inference and image generation",
		Category:    CategoryImage,
		RequiresEnv: []string{"REPLICATE_API_TOKEN"},
	},

	// ==========================================================================
	// DEVELOPMENT TOOL SERVERS
	// ==========================================================================
	// Fixed 2026-09-03. `mcp-server-postgres` RESOLVES but is an npm SECURITY
	// HOLDING package (single version 0.0.1-security). Notably it is EXACTLY
	// the `bin` name of the real scoped package below — the seized bare name
	// was a squat on the binary name, which is why it looked plausible.
	// Replacement: @modelcontextprotocol/server-postgres v0.6.2, ~100745
	// downloads/week, MCP-org scope sharing the control package's maintainers
	// — VENDOR, though ARCHIVED upstream (its source is vendored in-tree at
	// external/mcp-servers/servers-archived/src/postgres, whose package.json
	// declares `bin: {"mcp-server-postgres": ...}`, confirming the mapping).
	//
	// It takes the connection URL POSITIONALLY (index.ts: process.argv.slice(2))
	// and reads NO env var — the POSTGRES_URL this entry declared was never
	// consulted by the package. The catalogue records only the package name;
	// the positional URL is supplied by the generators.
	{
		Name:        "postgres",
		NPM:         "@modelcontextprotocol/server-postgres",
		Description: "MCP server for PostgreSQL database operations (connection URL passed positionally)",
		Category:    CategoryDev,
	},
	{
		Name:        "mongodb",
		NPM:         "mongodb-mcp-server",
		Description: "MCP server for MongoDB database operations",
		Category:    CategoryDev,
		RequiresEnv: []string{"MONGODB_URL"},
	},
	// Fixed 2026-09-03. `mcp-server-redis` RESOLVES but is an npm SECURITY
	// HOLDING package (single version 0.0.1-security), and like the postgres
	// case is exactly the `bin` name of the real scoped package.
	// Replacement: @modelcontextprotocol/server-redis v2025.4.25, ~2165
	// downloads/week, MCP-org scope — VENDOR, ARCHIVED upstream (source
	// vendored in-tree at external/mcp-servers/servers-archived/src/redis).
	//
	// It takes the Redis URL POSITIONALLY — src/index.ts line 13 is literally
	// `const REDIS_URL = process.argv[2] || "redis://localhost:6379"`, so the
	// REDIS_URL env var this entry declared is IGNORED by the package. Every
	// config in this repo was setting an env var the server never reads.
	{
		Name:        "redis",
		NPM:         "@modelcontextprotocol/server-redis",
		Description: "MCP server for Redis operations (connection URL passed positionally)",
		Category:    CategoryDev,
	},
	{
		Name:        "docker",
		NPM:         "mcp-server-docker",
		Description: "MCP server for Docker container operations",
		Category:    CategoryDev,
	},
	{
		Name:        "kubernetes",
		NPM:         "mcp-server-kubernetes",
		Description: "MCP server for Kubernetes operations",
		Category:    CategoryDev,
		RequiresEnv: []string{"KUBECONFIG"},
	},

	// ==========================================================================
	// SEARCH & WEB SERVERS
	// ==========================================================================
	{
		Name:        "brave-search",
		NPM:         "mcp-server-brave-search",
		Description: "MCP server for Brave Search API",
		Category:    CategorySearch,
		RequiresEnv: []string{"BRAVE_API_KEY"},
	},
	{
		Name:        "tavily",
		NPM:         "tavily-mcp",
		Description: "MCP server for Tavily search API",
		Category:    CategorySearch,
		RequiresEnv: []string{"TAVILY_API_KEY"},
	},

	// Restored 2026-09-03. ce9c4eb9 removed "duckduckgo" because
	// `mcp-server-duckduckgo` 404s. Replacement verified live:
	// duckduckgo-mcp-server v0.1.2, repo github.com/zhsama/duckduckgo-mcp-server
	// -> HTTP 200, ~3445 downloads/week, community publisher (zhsama).
	// Invocation is a bare `npx -y duckduckgo-mcp-server` per the package's
	// own README config block; the API needs no credential.
	{
		Name:        "duckduckgo",
		NPM:         "duckduckgo-mcp-server",
		Description: "MCP server for DuckDuckGo search",
		Category:    CategorySearch,
	},

	// ==========================================================================
	// CLOUD STORAGE SERVERS
	// ==========================================================================
	{
		Name:        "s3",
		NPM:         "mcp-server-s3",
		Description: "MCP server for AWS S3 operations",
		Category:    CategoryCloud,
		RequiresEnv: []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"},
	},

	// Restored 2026-09-03. ce9c4eb9 removed "gcs" because `mcp-server-gcs`
	// 404s. Replacement verified live: @google-cloud/storage-mcp v0.6.0,
	// repo github.com/googleapis/gcloud-mcp -> HTTP 200, ~20919 downloads/week,
	// published by Google themselves (maintainers google-wombot / google-admin)
	// - VENDOR, not community. Auth is Application Default Credentials, so the
	// declared env is GOOGLE_CLOUD_PROJECT (the package's own variable), not
	// the GOOGLE_APPLICATION_CREDENTIALS the removed entry named.
	{
		Name:        "gcs",
		NPM:         "@google-cloud/storage-mcp",
		Description: "MCP server for Google Cloud Storage operations",
		Category:    CategoryCloud,
		RequiresEnv: []string{"GOOGLE_CLOUD_PROJECT"},
	},

	// Restored 2026-09-03. ce9c4eb9 removed "google-drive" because
	// `mcp-server-google-drive` 404s. Replacement verified live:
	// @modelcontextprotocol/server-gdrive v2025.1.14, ~6700 downloads/week,
	// published under the MCP org scope by the same maintainer set as the
	// control package @modelcontextprotocol/server-everything - VENDOR.
	// It declares no `repository` field; provenance is the scope + shared
	// maintainers + the archived source vendored in-tree at
	// external/mcp-servers/servers-archived/src/gdrive (whose package.json
	// names this package). ARCHIVED upstream - it installs and runs, but is
	// no longer maintained. Env GDRIVE_CREDENTIALS_PATH per that README.
	{
		Name:        "google-drive",
		NPM:         "@modelcontextprotocol/server-gdrive",
		Description: "MCP server for Google Drive operations (archived upstream)",
		Category:    CategoryCloud,
		RequiresEnv: []string{"GDRIVE_CREDENTIALS_PATH"},
	},
}

// MCPPackageCategory represents a category of MCP packages
type MCPPackageCategory string

const (
	CategoryCore     MCPPackageCategory = "core"
	CategoryVectorDB MCPPackageCategory = "vectordb"
	CategoryDesign   MCPPackageCategory = "design"
	CategoryImage    MCPPackageCategory = "image"
	CategoryDev      MCPPackageCategory = "dev"
	CategorySearch   MCPPackageCategory = "search"
	CategoryCloud    MCPPackageCategory = "cloud"
)

// MCPPackageExtended extends MCPPackage with additional metadata
type MCPPackageExtended struct {
	MCPPackage
	Category    MCPPackageCategory
	RequiresEnv []string
	Optional    bool
}

// GetPackagesByCategory returns all packages in a specific category
func GetPackagesByCategory(category MCPPackageCategory) []MCPPackage {
	var result []MCPPackage
	for _, pkg := range ExtendedMCPPackages {
		if pkg.Category == category {
			result = append(result, pkg)
		}
	}
	return result
}

// GetCorePackages returns all core MCP packages
func GetCorePackages() []MCPPackage {
	return GetPackagesByCategory(CategoryCore)
}

// GetVectorDBPackages returns all vector database MCP packages
func GetVectorDBPackages() []MCPPackage {
	return GetPackagesByCategory(CategoryVectorDB)
}

// GetDesignPackages returns all design tool MCP packages
func GetDesignPackages() []MCPPackage {
	return GetPackagesByCategory(CategoryDesign)
}

// GetImagePackages returns all image generation MCP packages
func GetImagePackages() []MCPPackage {
	return GetPackagesByCategory(CategoryImage)
}

// GetAllExtendedPackages returns all extended MCP packages
func GetAllExtendedPackages() []MCPPackage {
	return ExtendedMCPPackages
}

// FilterAvailablePackages filters packages based on available environment variables
func FilterAvailablePackages(packages []MCPPackage) []MCPPackage {
	var available []MCPPackage
	for _, pkg := range packages {
		if len(pkg.RequiresEnv) == 0 {
			available = append(available, pkg)
			continue
		}

		// Check if all required env vars are set
		allSet := true
		for _, envVar := range pkg.RequiresEnv {
			if os.Getenv(envVar) == "" {
				allSet = false
				break
			}
		}
		if allSet {
			available = append(available, pkg)
		}
	}
	return available
}
