# Feature Flags

**Date:** 2026-03-30
**Status:** Active

## Overview

HelixAgent uses environment variable-based feature flags to control optional subsystems, debug endpoints, and experimental features. This document is the authoritative reference for all feature flags, their defaults, and when to change them.

---

## Flag Reference

### ENABLE_PPROF

**Purpose:** Controls whether Go's `net/http/pprof` debug profiling endpoints are registered on the HTTP router.

**Default:** `false` (disabled)

**Values:** `true` or `false`

**When enabled, exposes:**
- `/debug/pprof/` -- profiling index
- `/debug/pprof/heap` -- heap memory profile
- `/debug/pprof/goroutine` -- goroutine stack dumps
- `/debug/pprof/profile` -- CPU profile (30s by default)
- `/debug/pprof/trace` -- execution trace

**When to enable:**
- Diagnosing memory leaks or goroutine leaks in development/staging
- Collecting CPU profiles for performance optimization
- Running the `pprof_memory_profiling_challenge.sh` challenge

**When to disable:**
- Production deployments (pprof endpoints expose internal state and can be used for DoS via CPU profile requests)
- Any environment exposed to untrusted networks

**Key file:** `internal/router/router.go` (line ~176)

**Validation:** `tests/monitoring/pprof_endpoint_test.go` verifies that pprof routes are NOT accessible when the flag is unset.

---

### GOMAXPROCS

**Purpose:** Controls the maximum number of OS threads that can execute Go code simultaneously. This is a Go runtime setting, not HelixAgent-specific, but it has first-class importance in this project.

**Default:** Number of CPU cores (Go runtime default)

**Recommended for tests:** `2`

**When to change:**
- **Tests and challenges:** MUST be set to `2` per Constitution resource limit requirements
- **Production (resource-constrained):** Set to match the number of allocated CPU cores
- **Production (dedicated host):** Leave unset to use all available cores
- **Benchmarks:** Set to `2` for reproducible results across machines

**Impact:**
- Controls parallelism in lazy initialization (semaphore limits)
- Controls concurrent provider verification at startup
- Controls worker pool sizing in some components
- Directly affects benchmark numbers (higher GOMAXPROCS = different throughput)

**Usage:**
```bash
GOMAXPROCS=2 ./bin/helixagent        # Limit to 2 threads
GOMAXPROCS=2 go test ./...           # Limit tests to 2 threads
```

---

### COGNEE_ENABLED

**Purpose:** Enables the Cognee knowledge graph integration as an optional memory backend alongside the primary Mem0 memory system.

**Default:** `false` (disabled)

**Values:** `true` or `false`

**When to enable:**
- When a Cognee server is available and configured
- When knowledge graph capabilities are needed beyond what Mem0 provides
- When running Cognee-specific integration tests or challenges

**When to disable:**
- Default operation (Mem0 is the primary memory system)
- When no Cognee infrastructure is available
- In CI-like environments without Cognee containers

**Related variables:**
- `COGNEE_API_URL` -- Cognee server endpoint
- `COGNEE_API_KEY` -- Cognee authentication key

**Key file:** `internal/config/config.go`

**Notes:** Cognee is complementary to Mem0, not a replacement. Mem0 handles fact storage; Cognee provides knowledge graph traversal. HelixMemory's fusion pipeline can use both.

---

### CONSTITUTION_WATCHER_ENABLED

**Purpose:** Enables the background Constitution watcher service that automatically detects project changes (new modules, documentation updates, structure changes) and regenerates the Constitution.

**Default:** `false` (disabled)

**Values:** `true` or `false`

**Related variables:**
- `CONSTITUTION_WATCHER_CHECK_INTERVAL` -- How often to check for changes (default: `5m`)

**When to enable:**
- During active development when modules are being extracted or restructured
- When documentation synchronization between CLAUDE.md, AGENTS.md, and Constitution must stay current automatically

**When to disable:**
- Production deployments (Constitution should be stable)
- During benchmarking (background goroutine overhead)
- When Constitution updates should be deliberate, not automatic

**Key files:**
- `internal/services/constitution_watcher.go` -- watcher implementation
- `internal/services/constitution_manager.go` -- Constitution generation
- `internal/services/documentation_sync.go` -- CLAUDE.md/AGENTS.md sync

**Validation:** `./challenges/scripts/constitution_watcher_challenge.sh` (12 tests)

---

### GRAPHQL_ENABLED

**Purpose:** Enables the GraphQL API endpoint at `/v1/graphql`. This is a feature-flagged experimental endpoint.

**Default:** `false` (disabled)

**Values:** `true` or `false`

**When to enable:**
- When GraphQL API access is needed by frontend clients or integrations
- During development of GraphQL schema extensions

**When to disable:**
- Default operation (REST API is the primary interface)
- Production environments that do not need GraphQL

**Key file:** `internal/router/router.go` (line ~1278), `internal/graphql/schema.go`

---

### BIGDATA_ENABLE_* Family

**Purpose:** Controls individual BigData subsystem components. Each can be enabled or disabled independently.

| Flag | Default | Component |
|------|---------|-----------|
| `BIGDATA_ENABLE_INFINITE_CONTEXT` | `true` | Infinite context window via event sourcing |
| `BIGDATA_ENABLE_DISTRIBUTED_MEMORY` | `false` | Distributed memory synchronization |
| `BIGDATA_ENABLE_KNOWLEDGE_GRAPH` | `false` | Knowledge graph streaming pipeline |
| `BIGDATA_ENABLE_ANALYTICS` | `false` | ClickHouse analytics integration |
| `BIGDATA_ENABLE_CROSS_LEARNING` | `true` | Cross-session learning from debates |

**When to change:**
- Enable `DISTRIBUTED_MEMORY` when running a multi-node HelixAgent cluster
- Enable `KNOWLEDGE_GRAPH` when Neo4j is available
- Enable `ANALYTICS` when ClickHouse is available
- Disable `INFINITE_CONTEXT` or `CROSS_LEARNING` only if their overhead is measurable and problematic

**Key file:** `internal/config/config.go`, `internal/bigdata/integration.go`

**Graceful degradation:** Missing dependencies (Neo4j, ClickHouse, Kafka) cause the corresponding component to log a warning and remain inactive rather than failing the boot.

---

### Build Tags as Feature Flags

Some features are controlled at compile time via Go build tags rather than environment variables:

| Build Tag | Effect |
|-----------|--------|
| `nohelixmemory` | Exclude HelixMemory module entirely |
| `nohelixspecifier` | Exclude HelixSpecifier module entirely |

**Usage:**
```bash
go build -tags nohelixmemory ./cmd/helixagent/
```

These are useful for reducing binary size or avoiding dependencies that are not needed in a specific deployment.

---

## Summary Table

| Flag | Default | Type | Requires Infra |
|------|---------|------|---------------|
| `ENABLE_PPROF` | `false` | Runtime | No |
| `GOMAXPROCS` | All cores | Runtime | No |
| `COGNEE_ENABLED` | `false` | Runtime | Cognee server |
| `CONSTITUTION_WATCHER_ENABLED` | `false` | Runtime | No |
| `GRAPHQL_ENABLED` | `false` | Runtime | No |
| `BIGDATA_ENABLE_INFINITE_CONTEXT` | `true` | Runtime | No |
| `BIGDATA_ENABLE_DISTRIBUTED_MEMORY` | `false` | Runtime | Kafka |
| `BIGDATA_ENABLE_KNOWLEDGE_GRAPH` | `false` | Runtime | Neo4j |
| `BIGDATA_ENABLE_ANALYTICS` | `false` | Runtime | ClickHouse |
| `BIGDATA_ENABLE_CROSS_LEARNING` | `true` | Runtime | No |
| `nohelixmemory` | Not set | Build tag | N/A |
| `nohelixspecifier` | Not set | Build tag | N/A |

---

## Cross-References

- Full configuration reference: `docs/configuration/README.md`
- Environment variables: `.env.example`
- BigData architecture: `docs/bigdata/`
- HelixMemory setup: `docs/HELIXMEMORY_SETUP.md`
- Constitution watcher: `docs/development/DEAD_CODE_AUDIT.md` (env var cleanup)
