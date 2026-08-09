// Package ports is the single source of truth for TCP port
// assignments across HelixAgent and its satellite services.
//
// # Motivation
//
// Ports were previously scattered across .env files, yaml configs,
// docker-compose files, handler code, and tests — 1000+ file
// references to raw integer literals. Migrating the whole project
// to a new port band required editing every one.
//
// This package centralizes:
//
//   - Canonical logical identifiers for every port-using service
//     (exported as the Service enum, whose string value doubles
//     as the canonical env var name).
//
//   - The numeric offset for each service within its band.
//
//   - A configurable prefix digit (8 by default, switchable to 9
//     via the HELIXAGENT_PORT_PREFIX env var) which multiplies to
//     form the full port: Prefix()*1000 + offset.
//
//   - Env-var lookup so submodule services launched in containers
//     honor root-provided ports over their own hardcoded defaults.
//
// Band layout (prefix=8)
//
//	81xx   core + eager infrastructure (HelixAgent, databases,
//	       MCP bridge, HelixLLM, lazy data services)
//	82xx   MCP servers (12 tiers: core, database, vector, devops,
//	       browser, communication, productivity, search/AI,
//	       google, monitoring, finance, design)
//	83xx   observability stack (Prometheus, Grafana, ES, Signoz)
//	84xx-89xx  reserved for future expansion
//
// Switching to prefix=9 shifts every port: HelixAgent 8100 → 9100,
// Postgres 8101 → 9101, etc. The band layout is identical, so
// config files that use env-var interpolation (${PORT:-8100})
// continue to work unmodified when only the prefix changes.
//
// Usage
//
//	port := ports.Get(ports.HelixAgentHTTP)    // 8100
//	addr := ports.Addr(ports.HelixAgentHTTP)   // ":8100"
//	url  := ports.HTTPURL(ports.HelixAgentHTTP, "localhost")
//	env  := ports.Export() // map for os/exec or container env
package ports

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Service is a logical identifier for a port-using service.
// Its string value is the canonical env var name; callers can
// override the default for any service by setting the env var.
type Service string

// Core services (band 81xx). Offsets 0-29 are reserved for
// eager infrastructure — main HelixAgent, databases, caches,
// principal LLM/gateway services, and auxiliary data engines.
const (
	// HelixAgentHTTP — main HelixAgent HTTP API (was 7061).
	HelixAgentHTTP Service = "HELIXAGENT_PORT_HTTP"
	// PostgresPrimary — primary PostgreSQL instance (was 5432).
	PostgresPrimary Service = "HELIXAGENT_PORT_POSTGRES"
	// RedisDefault — no-password Redis (was 6379).
	RedisDefault Service = "HELIXAGENT_PORT_REDIS"
	// MCPBridge — MCP gateway (was 9000).
	MCPBridge Service = "HELIXAGENT_PORT_MCP_BRIDGE"
	// MCPRouterAlt — alternative MCP router (was 9100).
	MCPRouterAlt Service = "HELIXAGENT_PORT_MCP_ROUTER_ALT"
	// HelixLLM — HelixLLM gateway with TLS (was 8444).
	HelixLLM Service = "HELIXAGENT_PORT_HELIXLLM"
	// MockLLM — test-only mock LLM server (was 18081).
	MockLLM Service = "HELIXAGENT_PORT_MOCK_LLM"
	// PostgresReplica — Postgres streaming replica (was 5433).
	PostgresReplica Service = "HELIXAGENT_PORT_POSTGRES_REPLICA"
	// PostgresExtra — supplementary Postgres instance (was 5434).
	PostgresExtra Service = "HELIXAGENT_PORT_POSTGRES_EXTRA"
	// PostgresTest — Postgres for tests (was 15432).
	PostgresTest Service = "HELIXAGENT_PORT_POSTGRES_TEST"
	// RedisMCP — password-protected MCP-backend Redis (was 16379).
	RedisMCP Service = "HELIXAGENT_PORT_REDIS_MCP"
	// HelixAgentLiveness — early-bind liveness probe served the moment
	// the binary starts, BEFORE the ~7-min startup verification pipeline.
	// Returns 200 with {status:"starting"|"ready", started_at, elapsed_ms}.
	// Drainage report 2026-04-25 Finding #2: prior to this, no /health
	// existed during the verification window, breaking CLAUDE.md rule #6
	// (every service MUST expose health endpoints).
	HelixAgentLiveness Service = "HELIXAGENT_PORT_LIVENESS"
	// HelixAgentGRPC — the gRPC API served by cmd/grpc-server
	// (LLMFacade + LLMProvider). Was 50051, hardcoded and absent
	// from this registry until 2026-08-09.
	//
	// 50051 is the gRPC ecosystem's conventional port, which makes
	// it the single most contended number a gRPC service can pick:
	// this project's own helixcode-infra-weaviate container
	// publishes it, so the server could not start while HelixAgent's
	// infrastructure was up, and clients dialling that literal
	// reached Weaviate and got Unimplemented for every method.
	// Because the service was not registered here, that collision
	// was undetectable by TestOffsets_NoCollisions — a service the
	// registry does not know cannot be arbitrated against the ones
	// it does.
	HelixAgentGRPC Service = "HELIXAGENT_PORT_GRPC"

	// Lazy / auxiliary data services (offsets 20-29).
	// Cognee — knowledge-graph engine (was 8000, lazy).
	Cognee Service = "HELIXAGENT_PORT_COGNEE"
	// ChromaDB — vector store (was 8001, lazy).
	ChromaDB Service = "HELIXAGENT_PORT_CHROMADB"
	// Qdrant — vector store alt (was 6333, lazy).
	Qdrant Service = "HELIXAGENT_PORT_QDRANT"
	// Neo4jHTTP — Neo4j HTTP interface (was 7474, lazy).
	Neo4jHTTP Service = "HELIXAGENT_PORT_NEO4J_HTTP"
	// Neo4jBolt — Neo4j Bolt protocol (was 7687, lazy).
	Neo4jBolt Service = "HELIXAGENT_PORT_NEO4J_BOLT"
)

// MCP server tiers (band 82xx). Each tier gets a contiguous
// block; gaps within a block are reserved for future servers in
// the same category.
const (
	// Tier 1: core (8200-8209). 10 servers.
	MCPFetch              Service = "HELIXAGENT_PORT_MCP_FETCH"
	MCPGit                Service = "HELIXAGENT_PORT_MCP_GIT"
	MCPTime               Service = "HELIXAGENT_PORT_MCP_TIME"
	MCPFilesystem         Service = "HELIXAGENT_PORT_MCP_FILESYSTEM"
	MCPMemory             Service = "HELIXAGENT_PORT_MCP_MEMORY"
	MCPEverything         Service = "HELIXAGENT_PORT_MCP_EVERYTHING"
	MCPSequentialThinking Service = "HELIXAGENT_PORT_MCP_SEQUENTIAL_THINKING"
	MCPSQLite             Service = "HELIXAGENT_PORT_MCP_SQLITE"
	MCPPuppeteer          Service = "HELIXAGENT_PORT_MCP_PUPPETEER"
	MCPPostgres           Service = "HELIXAGENT_PORT_MCP_POSTGRES"

	// Tier 2: database (8210-8214). 5 servers.
	MCPMongoDB       Service = "HELIXAGENT_PORT_MCP_MONGODB"
	MCPRedis         Service = "HELIXAGENT_PORT_MCP_REDIS"
	MCPMySQL         Service = "HELIXAGENT_PORT_MCP_MYSQL"
	MCPElasticsearch Service = "HELIXAGENT_PORT_MCP_ELASTICSEARCH"
	MCPSupabase      Service = "HELIXAGENT_PORT_MCP_SUPABASE"

	// Tier 3: vector (8215-8218). 4 servers.
	MCPQdrant   Service = "HELIXAGENT_PORT_MCP_QDRANT"
	MCPChroma   Service = "HELIXAGENT_PORT_MCP_CHROMA"
	MCPPinecone Service = "HELIXAGENT_PORT_MCP_PINECONE"
	MCPWeaviate Service = "HELIXAGENT_PORT_MCP_WEAVIATE"

	// Tier 4: devops/infra (8220-8233). 14 servers.
	MCPGitHub     Service = "HELIXAGENT_PORT_MCP_GITHUB"
	MCPGitLab     Service = "HELIXAGENT_PORT_MCP_GITLAB"
	MCPSentry     Service = "HELIXAGENT_PORT_MCP_SENTRY"
	MCPKubernetes Service = "HELIXAGENT_PORT_MCP_KUBERNETES"
	MCPDocker     Service = "HELIXAGENT_PORT_MCP_DOCKER"
	MCPAnsible    Service = "HELIXAGENT_PORT_MCP_ANSIBLE"
	MCPAWS        Service = "HELIXAGENT_PORT_MCP_AWS"
	MCPGCP        Service = "HELIXAGENT_PORT_MCP_GCP"
	MCPHeroku     Service = "HELIXAGENT_PORT_MCP_HEROKU"
	MCPCloudflare Service = "HELIXAGENT_PORT_MCP_CLOUDFLARE"
	MCPVercel     Service = "HELIXAGENT_PORT_MCP_VERCEL"
	MCPWorkers    Service = "HELIXAGENT_PORT_MCP_WORKERS"
	MCPJetbrains  Service = "HELIXAGENT_PORT_MCP_JETBRAINS"
	MCPK8sAlt     Service = "HELIXAGENT_PORT_MCP_K8S_ALT"

	// Tier 5: browser (8234-8237). 4 servers.
	MCPPlaywright  Service = "HELIXAGENT_PORT_MCP_PLAYWRIGHT"
	MCPBrowserbase Service = "HELIXAGENT_PORT_MCP_BROWSERBASE"
	MCPFirecrawl   Service = "HELIXAGENT_PORT_MCP_FIRECRAWL"
	MCPCrawl4AI    Service = "HELIXAGENT_PORT_MCP_CRAWL4AI"

	// Tier 6: communication (8238-8240). 3 servers.
	MCPSlack    Service = "HELIXAGENT_PORT_MCP_SLACK"
	MCPDiscord  Service = "HELIXAGENT_PORT_MCP_DISCORD"
	MCPTelegram Service = "HELIXAGENT_PORT_MCP_TELEGRAM"

	// Tier 7: productivity (8250-8259). 10 servers.
	MCPNotion    Service = "HELIXAGENT_PORT_MCP_NOTION"
	MCPLinear    Service = "HELIXAGENT_PORT_MCP_LINEAR"
	MCPJira      Service = "HELIXAGENT_PORT_MCP_JIRA"
	MCPAsana     Service = "HELIXAGENT_PORT_MCP_ASANA"
	MCPTrello    Service = "HELIXAGENT_PORT_MCP_TRELLO"
	MCPTodoist   Service = "HELIXAGENT_PORT_MCP_TODOIST"
	MCPMonday    Service = "HELIXAGENT_PORT_MCP_MONDAY"
	MCPAirtable  Service = "HELIXAGENT_PORT_MCP_AIRTABLE"
	MCPObsidian  Service = "HELIXAGENT_PORT_MCP_OBSIDIAN"
	MCPAtlassian Service = "HELIXAGENT_PORT_MCP_ATLASSIAN"

	// Tier 8: search/AI (8260-8269). 10 servers.
	MCPBraveSearch Service = "HELIXAGENT_PORT_MCP_BRAVE_SEARCH"
	MCPExa         Service = "HELIXAGENT_PORT_MCP_EXA"
	MCPTavily      Service = "HELIXAGENT_PORT_MCP_TAVILY"
	MCPPerplexity  Service = "HELIXAGENT_PORT_MCP_PERPLEXITY"
	MCPKagi        Service = "HELIXAGENT_PORT_MCP_KAGI"
	MCPOmnisearch  Service = "HELIXAGENT_PORT_MCP_OMNISEARCH"
	MCPContext7    Service = "HELIXAGENT_PORT_MCP_CONTEXT7"
	MCPLlamaIndex  Service = "HELIXAGENT_PORT_MCP_LLAMAINDEX"
	MCPLangchain   Service = "HELIXAGENT_PORT_MCP_LANGCHAIN"
	MCPOpenAI      Service = "HELIXAGENT_PORT_MCP_OPENAI"

	// Tier 9: google (8270-8274). 5 servers.
	MCPGDrive    Service = "HELIXAGENT_PORT_MCP_GDRIVE"
	MCPGCalendar Service = "HELIXAGENT_PORT_MCP_GCALENDAR"
	MCPGMaps     Service = "HELIXAGENT_PORT_MCP_GMAPS"
	MCPYouTube   Service = "HELIXAGENT_PORT_MCP_YOUTUBE"
	MCPGmail     Service = "HELIXAGENT_PORT_MCP_GMAIL"

	// Tier 10: monitoring (8275-8277). 3 servers.
	MCPDatadog    Service = "HELIXAGENT_PORT_MCP_DATADOG"
	MCPGrafana    Service = "HELIXAGENT_PORT_MCP_GRAFANA"
	MCPPrometheus Service = "HELIXAGENT_PORT_MCP_PROMETHEUS"

	// Tier 11: finance/business (8278-8280). 3 servers.
	MCPStripe  Service = "HELIXAGENT_PORT_MCP_STRIPE"
	MCPHubspot Service = "HELIXAGENT_PORT_MCP_HUBSPOT"
	MCPZendesk Service = "HELIXAGENT_PORT_MCP_ZENDESK"

	// Tier 12: design (8281). 1 server.
	MCPFigma Service = "HELIXAGENT_PORT_MCP_FIGMA"
)

// Observability stack (band 83xx).
const (
	ACPManager       Service = "HELIXAGENT_PORT_ACP_MANAGER"
	ElasticsearchAlt Service = "HELIXAGENT_PORT_ELASTICSEARCH_ALT"
	OpenSearch       Service = "HELIXAGENT_PORT_OPENSEARCH"
	SignozOTel       Service = "HELIXAGENT_PORT_SIGNOZ_OTEL"
	Prometheus       Service = "HELIXAGENT_PORT_PROMETHEUS"
	Grafana          Service = "HELIXAGENT_PORT_GRAFANA"
	Jaeger           Service = "HELIXAGENT_PORT_JAEGER"
)

// EnvPrefix selects the leading digit of default ports. Setting
// HELIXAGENT_PORT_PREFIX=9 shifts every port into the 9xxx band.
const EnvPrefix = "HELIXAGENT_PORT_PREFIX"

// DefaultPrefix is the prefix assumed when EnvPrefix is unset.
const DefaultPrefix = 8

// offsets maps each Service to its in-band offset. The final
// default port is Prefix()*1000 + offset.
var offsets = map[Service]int{
	// 81xx — core.
	HelixAgentHTTP:     100,
	PostgresPrimary:    101,
	RedisDefault:       102,
	MCPBridge:          103,
	MCPRouterAlt:       104,
	HelixLLM:           105,
	MockLLM:            106,
	PostgresReplica:    107,
	PostgresExtra:      108,
	PostgresTest:       109,
	RedisMCP:           110,
	HelixAgentLiveness: 111,
	HelixAgentGRPC:     112,
	Cognee:             120,
	ChromaDB:           121,
	Qdrant:             122,
	Neo4jHTTP:          123,
	Neo4jBolt:          124,

	// 82xx — MCP tiers.
	// Tier 1 core: 200-209.
	MCPFetch:              200,
	MCPGit:                201,
	MCPTime:               202,
	MCPFilesystem:         203,
	MCPMemory:             204,
	MCPEverything:         205,
	MCPSequentialThinking: 206,
	MCPSQLite:             207,
	MCPPuppeteer:          208,
	MCPPostgres:           209,
	// Tier 2 database: 210-214.
	MCPMongoDB:       210,
	MCPRedis:         211,
	MCPMySQL:         212,
	MCPElasticsearch: 213,
	MCPSupabase:      214,
	// Tier 3 vector: 215-218.
	MCPQdrant:   215,
	MCPChroma:   216,
	MCPPinecone: 217,
	MCPWeaviate: 218,
	// Tier 4 devops/infra: 220-233.
	MCPGitHub:     220,
	MCPGitLab:     221,
	MCPSentry:     222,
	MCPKubernetes: 223,
	MCPDocker:     224,
	MCPAnsible:    225,
	MCPAWS:        226,
	MCPGCP:        227,
	MCPHeroku:     228,
	MCPCloudflare: 229,
	MCPVercel:     230,
	MCPWorkers:    231,
	MCPJetbrains:  232,
	MCPK8sAlt:     233,
	// Tier 5 browser: 234-237.
	MCPPlaywright:  234,
	MCPBrowserbase: 235,
	MCPFirecrawl:   236,
	MCPCrawl4AI:    237,
	// Tier 6 communication: 238-240.
	MCPSlack:    238,
	MCPDiscord:  239,
	MCPTelegram: 240,
	// Tier 7 productivity: 250-259.
	MCPNotion:    250,
	MCPLinear:    251,
	MCPJira:      252,
	MCPAsana:     253,
	MCPTrello:    254,
	MCPTodoist:   255,
	MCPMonday:    256,
	MCPAirtable:  257,
	MCPObsidian:  258,
	MCPAtlassian: 259,
	// Tier 8 search/AI: 260-269.
	MCPBraveSearch: 260,
	MCPExa:         261,
	MCPTavily:      262,
	MCPPerplexity:  263,
	MCPKagi:        264,
	MCPOmnisearch:  265,
	MCPContext7:    266,
	MCPLlamaIndex:  267,
	MCPLangchain:   268,
	MCPOpenAI:      269,
	// Tier 9 google: 270-274.
	MCPGDrive:    270,
	MCPGCalendar: 271,
	MCPGMaps:     272,
	MCPYouTube:   273,
	MCPGmail:     274,
	// Tier 10 monitoring: 275-277.
	MCPDatadog:    275,
	MCPGrafana:    276,
	MCPPrometheus: 277,
	// Tier 11 finance/business: 278-280.
	MCPStripe:  278,
	MCPHubspot: 279,
	MCPZendesk: 280,
	// Tier 12 design: 281.
	MCPFigma: 281,

	// 83xx — observability.
	ACPManager:       300,
	ElasticsearchAlt: 301,
	OpenSearch:       302,
	SignozOTel:       303,
	Prometheus:       310,
	Grafana:          311,
	Jaeger:           312,
}

var (
	prefixOnce sync.Once
	prefix     int
)

// Prefix returns the active port prefix, derived once from
// HELIXAGENT_PORT_PREFIX (default 8). Valid values: 8 or 9.
// Any other value falls back to DefaultPrefix.
func Prefix() int {
	prefixOnce.Do(func() {
		prefix = DefaultPrefix
		raw := strings.TrimSpace(os.Getenv(EnvPrefix))
		if raw == "" {
			return
		}
		n, err := strconv.Atoi(raw)
		if err != nil || (n != 8 && n != 9) {
			return
		}
		prefix = n
	})
	return prefix
}

// resetPrefixForTest resets the cached prefix. Tests call this
// via PrefixResetForTest when flipping HELIXAGENT_PORT_PREFIX.
func resetPrefixForTest() {
	prefixOnce = sync.Once{}
}

// PrefixResetForTest is exported for tests that need to observe
// a fresh Prefix() reading after manipulating the env var.
// Do not call from production code.
func PrefixResetForTest() { resetPrefixForTest() }

// Get returns the port for svc. If the env var named by svc is
// set and parseable as a uint16, that value wins; otherwise the
// default (Prefix()*1000 + offset) is returned. Panics if svc
// is not registered — that would be a programming error.
func Get(svc Service) int {
	raw := strings.TrimSpace(os.Getenv(string(svc)))
	if raw != "" {
		if n, err := strconv.Atoi(raw); err == nil &&
			n > 0 && n < 65536 {
			return n
		}
	}
	off, ok := offsets[svc]
	if !ok {
		panic(fmt.Sprintf("ports: unregistered service %q", svc))
	}
	return Prefix()*1000 + off
}

// Default returns the default port for svc ignoring any env
// override. Useful for docs/diagnostics.
func Default(svc Service) int {
	off, ok := offsets[svc]
	if !ok {
		panic(fmt.Sprintf("ports: unregistered service %q", svc))
	}
	return Prefix()*1000 + off
}

// Addr returns the canonical listen address ":<port>" for svc,
// ready for http.ListenAndServe and similar.
func Addr(svc Service) string {
	return fmt.Sprintf(":%d", Get(svc))
}

// HostPort returns "<host>:<port>" for svc, suitable for dialing.
func HostPort(svc Service, host string) string {
	return fmt.Sprintf("%s:%d", host, Get(svc))
}

// HTTPURL returns "http://<host>:<port>" for svc.
func HTTPURL(svc Service, host string) string {
	return fmt.Sprintf("http://%s:%d", host, Get(svc))
}

// HTTPSURL returns "https://<host>:<port>" for svc.
func HTTPSURL(svc Service, host string) string {
	return fmt.Sprintf("https://%s:%d", host, Get(svc))
}

// All returns every registered service. Stable ordering is not
// guaranteed — callers that need determinism should sort.
func All() []Service {
	out := make([]Service, 0, len(offsets))
	for s := range offsets {
		out = append(out, s)
	}
	return out
}

// Export returns a map[envVarName]portString for every service,
// suitable for injection into os/exec or container environments.
// Entries reflect the currently active prefix and any env overrides.
func Export() map[string]string {
	out := make(map[string]string, len(offsets)+1)
	out[EnvPrefix] = strconv.Itoa(Prefix())
	for s := range offsets {
		out[string(s)] = strconv.Itoa(Get(s))
	}
	return out
}
