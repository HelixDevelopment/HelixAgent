# Port Registry

Canonical port assignments for HelixAgent and every service it
launches. Lives in `internal/ports/ports.go`; this document is the
human-readable companion.

## Why a registry

Before: 1000+ file references to raw integer ports scattered
across `.env` files, YAML configs, `docker-compose*.yml`, handler
code, tests, and challenge scripts. Moving to a new port band
meant editing every one by hand, with every port migration risking
subtle breakage (timeouts vs. ports, year literals vs. ports, etc.).

Now: a single Go package (`internal/ports`) holds every port as a
derived value of one configurable prefix. Every file imports it and
calls `ports.Get(svc)`, `ports.Addr(svc)`, etc. Switching the whole
system between the 8xxx and 9xxx bands is a single env var.

## Prefix mechanism

| Env var | Accepted values | Default | Effect |
|---|---|---|---|
| `HELIXAGENT_PORT_PREFIX` | `8` or `9` | `8` | First digit of every default port |

- `HELIXAGENT_PORT_PREFIX=8` → ports are `8100 … 8312`.
- `HELIXAGENT_PORT_PREFIX=9` → same layout in the `9xxx` band.
- Any other value (including empty, missing, non-numeric) silently
  falls back to the default (`8`). See `TestPrefix_InvalidValuesFallBackToDefault`.

The prefix is cached after the first read (via `sync.Once`) so
reading many ports is cheap and consistent within a single run.

## Band layout

```
81xx  Core + eager infrastructure         (offsets 100-124)
82xx  MCP servers (12 tiers)              (offsets 200-281)
83xx  Observability stack                 (offsets 300-312)
84xx  reserved
85xx  reserved
86xx  reserved
87xx  reserved
88xx  reserved
89xx  reserved
```

Each band has head-room for ~20× the currently used slots, so
adding a new MCP server tier or auxiliary service does not
require rebalancing.

## Canonical port table (prefix = 8)

### 81xx core (16 in use)

| Old port | New port | Service | Env var |
|---|---|---|---|
| 7061  | **8100** | HelixAgent main HTTP | `HELIXAGENT_PORT_HTTP` |
| 5432  | **8101** | PostgreSQL primary | `HELIXAGENT_PORT_POSTGRES` |
| 6379  | **8102** | Redis (no password) | `HELIXAGENT_PORT_REDIS` |
| 9000  | **8103** | MCP Bridge | `HELIXAGENT_PORT_MCP_BRIDGE` |
| 9100  | **8104** | MCP Router (alt) | `HELIXAGENT_PORT_MCP_ROUTER_ALT` |
| 8444  | **8105** | HelixLLM (TLS) | `HELIXAGENT_PORT_HELIXLLM` |
| 18081 | **8106** | Mock LLM (tests) | `HELIXAGENT_PORT_MOCK_LLM` |
| 5433  | **8107** | PostgreSQL replica | `HELIXAGENT_PORT_POSTGRES_REPLICA` |
| 5434  | **8108** | PostgreSQL extra | `HELIXAGENT_PORT_POSTGRES_EXTRA` |
| 15432 | **8109** | PostgreSQL test | `HELIXAGENT_PORT_POSTGRES_TEST` |
| 16379 | **8110** | Redis MCP backend (pwd) | `HELIXAGENT_PORT_REDIS_MCP` |
| 8000  | **8120** | Cognee (lazy) | `HELIXAGENT_PORT_COGNEE` |
| 8001  | **8121** | ChromaDB (lazy) | `HELIXAGENT_PORT_CHROMADB` |
| 6333  | **8122** | Qdrant (lazy) | `HELIXAGENT_PORT_QDRANT` |
| 7474  | **8123** | Neo4j HTTP (lazy) | `HELIXAGENT_PORT_NEO4J_HTTP` |
| 7687  | **8124** | Neo4j Bolt (lazy) | `HELIXAGENT_PORT_NEO4J_BOLT` |

### 82xx MCP tiers (57 in use; 100 reserved)

| Range | Tier | Count | Env-var prefix |
|---|---|---|---|
| 8200-8209 | T1  core (fetch, git, time, fs, memory, everything, sequential-thinking, sqlite, puppeteer, postgres) | 10 | `HELIXAGENT_PORT_MCP_*` |
| 8210-8214 | T2  database (mongodb, redis, mysql, elasticsearch, supabase) | 5 | `HELIXAGENT_PORT_MCP_*` |
| 8215-8218 | T3  vector (qdrant, chroma, pinecone, weaviate) | 4 | `HELIXAGENT_PORT_MCP_*` |
| 8220-8233 | T4  devops/infra (github, gitlab, sentry, k8s, docker, …) | 14 | `HELIXAGENT_PORT_MCP_*` |
| 8234-8237 | T5  browser (playwright, browserbase, firecrawl, crawl4ai) | 4 | `HELIXAGENT_PORT_MCP_*` |
| 8238-8240 | T6  communication (slack, discord, telegram) | 3 | `HELIXAGENT_PORT_MCP_*` |
| 8250-8259 | T7  productivity (notion, jira, trello, …) | 10 | `HELIXAGENT_PORT_MCP_*` |
| 8260-8269 | T8  search/AI (brave, exa, perplexity, context7, …) | 10 | `HELIXAGENT_PORT_MCP_*` |
| 8270-8274 | T9  google (drive, calendar, maps, youtube, gmail) | 5 | `HELIXAGENT_PORT_MCP_*` |
| 8275-8277 | T10 monitoring (datadog, grafana, prometheus) | 3 | `HELIXAGENT_PORT_MCP_*` |
| 8278-8280 | T11 finance/business (stripe, hubspot, zendesk) | 3 | `HELIXAGENT_PORT_MCP_*` |
| 8281      | T12 design (figma) | 1 | `HELIXAGENT_PORT_MCP_*` |

Full mapping lives in `internal/ports/ports.go`; grep
`HELIXAGENT_PORT_MCP_` to enumerate.

### 83xx observability (7 in use; 40 reserved)

| Old port | New port | Service | Env var |
|---|---|---|---|
| 9200 | **8300** | ACP Manager / Elasticsearch | `HELIXAGENT_PORT_ACP_MANAGER` |
| 9210 | **8301** | Elasticsearch (alt) | `HELIXAGENT_PORT_ELASTICSEARCH_ALT` |
| 9211 | **8302** | OpenSearch / osquery | `HELIXAGENT_PORT_OPENSEARCH` |
| 9220 | **8303** | Signoz OTel | `HELIXAGENT_PORT_SIGNOZ_OTEL` |
| (var) | **8310** | Prometheus | `HELIXAGENT_PORT_PROMETHEUS` |
| (var) | **8311** | Grafana | `HELIXAGENT_PORT_GRAFANA` |
| (var) | **8312** | Jaeger | `HELIXAGENT_PORT_JAEGER` |

## How to use it

### In Go code (root)

```go
import "dev.helix.agent/internal/ports"

srv := &http.Server{
    Addr: ports.Addr(ports.HelixAgentHTTP), // ":8100"
}

// Dialing a sibling:
conn, err := net.Dial("tcp",
    ports.HostPort(ports.PostgresPrimary, "localhost"))
```

### In Go code (submodules)

Submodules must not import `dev.helix.agent/internal/ports` —
that would break their project-agnostic mandate. Instead they
read the canonical env var they expect the root to inject:

```go
port, _ := strconv.Atoi(os.Getenv("HELIXAGENT_PORT_POSTGRES"))
if port == 0 {
    port = 5432 // legacy fallback for standalone dev
}
```

The root binary injects every `HELIXAGENT_PORT_*` env var when it
spawns child processes or passes env into containers (see
`ports.Export()`).

### In docker-compose files

Use the canonical env var with an explicit default. Compose
substitutes from the environment provided by helixagent at boot:

```yaml
services:
  postgres:
    image: postgres:15-alpine
    ports:
      - "${HELIXAGENT_PORT_POSTGRES:-8101}:5432"
```

The `:5432` part is the CONTAINER-internal port and never
changes. Only the HOST-side port is remapped. This keeps Postgres
happy (it still binds 5432 inside the container) while the host
uses the registry-assigned port.

### Switching prefix

```bash
HELIXAGENT_PORT_PREFIX=9 ./bin/helixagent
```

Every derived port shifts from `8xxx` to `9xxx` in one step. No
config-file rewriting needed.

## Invariants (enforced by tests)

- No two services share an offset (`TestOffsets_NoCollisions`).
- Every offset produces a valid 16-bit port at both prefixes
  (`TestOffsets_FitIn16BitAtBothPrefixes`).
- Band discipline: core ≤ 199, MCP 200-281, observability 300-312
  (`TestOffsets_WithinExpectedBands`).
- Every env-var name starts with `HELIXAGENT_PORT_`
  (`TestEnvVarNames_AllStartWithHelixAgentPrefix`).

## Adding a new port

1. Add the `Service` constant in `ports.go` (pick a name ending in
   its canonical env var form).
2. Add its offset to the `offsets` map in the appropriate band.
3. Add a spot-check in `TestGet_DefaultsMatchCanonicalMap`.
4. Update the table in this file.
5. `make test-unit` — the collision and band-discipline tests
   will catch any mistake.
