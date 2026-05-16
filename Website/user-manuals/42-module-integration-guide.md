# User Manual 42: Module Integration Guide

## Overview

HelixAgent comprises 41 extracted modules plus the core application. This guide explains how the modules connect, the boot sequence, configuration matrix, and inter-module communication patterns.

## Prerequisites

- Familiarity with HelixAgent architecture (see [User Manual 01](01-getting-started.md))
- All submodules initialized: `git submodule update --init --recursive`
- Go 1.25+ (modules use `replace` directives in the root `go.mod` for local development)

## Module Dependency Overview

Modules are organized in 8 phases. Each phase depends only on earlier phases, forming a strict dependency DAG:

### Phase 1: Foundation (Zero Dependencies)

| Module | Path | Purpose |
|--------|------|---------|
| EventBus | `EventBus/` | Pub/sub event system, filtering, middleware |
| Concurrency | `Concurrency/` | Worker pools, rate limiters, circuit breakers, semaphores |
| Observability | `Observability/` | OpenTelemetry tracing, Prometheus metrics, logging |
| Auth | `Auth/` | JWT, API key, OAuth authentication |
| Storage | `Storage/` | Object storage abstraction (S3, local, cloud) |
| Streaming | `Streaming/` | SSE, WebSocket, gRPC streaming, webhooks |

### Phase 2: Infrastructure (Zero Module Dependencies)

| Module | Path | Purpose |
|--------|------|---------|
| Security | `security/` | Guardrails, PII detection, content filtering |
| VectorDB | `VectorDB/` | Qdrant, Pinecone, Milvus, pgvector |
| Embeddings | `Embeddings/` | 6 embedding providers |
| Database | `Database/` | PostgreSQL, SQLite, connection pooling, migrations |
| Cache | `Cache/` | Redis + in-memory caching |

### Phase 3: Services

| Module | Path | Purpose |
|--------|------|---------|
| Messaging | `Messaging/` | Kafka, RabbitMQ, dead letter queues |
| Formatters | `Formatters/` | 32+ code formatters, registry, executor |
| MCP | `MCP_Module/` | Model Context Protocol adapters and servers |

### Phase 4: Integration

| Module | Path | Purpose |
|--------|------|---------|
| RAG | `RAG/` | Retrieval-Augmented Generation pipeline |
| Memory | `Memory/` | Mem0-style memory with entity graphs |
| Optimization | `Optimization/` | GPT-Cache, structured output, prompt optimization |
| Plugins | `Plugins/` | Plugin system, dynamic loading, sandboxing |

### Phase 5: AI/ML

| Module | Path | Purpose |
|--------|------|---------|
| Agentic | `Agentic/` | Graph-based agentic workflows |
| LLMOps | `LLMOps/` | A/B experiments, evaluation, prompt versioning |
| SelfImprove | `SelfImprove/` | Reward modelling, RLHF feedback |
| Planning | `Planning/` | HiPlan, MCTS, Tree of Thoughts |
| Benchmark | `Benchmark/` | SWE-bench, HumanEval, MMLU benchmarks |

### Phase 6: Cognitive

| Module | Path | Purpose |
|--------|------|---------|
| HelixMemory | `HelixMemory/` | Unified cognitive memory (Mem0 + Cognee + Letta + Graphiti) |

### Phase 7: Specification

| Module | Path | Purpose |
|--------|------|---------|
| HelixSpecifier | `HelixSpecifier/` | Spec-driven development fusion engine |

### Phase 8: Core Abstractions

| Module | Path | Purpose |
|--------|------|---------|
| LLMProvider | `LLMProvider/` | Provider interface, circuit breakers, health monitoring |
| Models | `Models/` | Shared data types for AI/LLM systems |
| ToolSchema | `ToolSchema/` | Tool schema definition and validation |
| SkillRegistry | `SkillRegistry/` | CLI agent skill registration |
| BackgroundTasks | `BackgroundTasks/` | Task persistence, resource monitoring |
| ConversationContext | `ConversationContext/` | Conversation compression, infinite context |
| DebateOrchestrator | `DebateOrchestrator/` | Multi-agent debate framework |
| BuildCheck | `BuildCheck/` | Content-based change detection for builds |

### Pre-Existing Modules

| Module | Path | Purpose |
|--------|------|---------|
| Containers | `containers/` | Docker/Podman/K8s orchestration |
| Challenges | `challenges/` | Test framework, assertions, userflow testing |
| Toolkit | `Toolkit/` | Go utility library for AI apps |
| LLMsVerifier | `LLMsVerifier/` | Provider accuracy verification and scoring |
| DocProcessor | `DocProcessor/` | Documentation processing and feature extraction |
| HelixQA | `helix_qa/` | QA orchestration framework |
| LLMOrchestrator | `LLMOrchestrator/` | Headless CLI agent management |
| VisionEngine | `VisionEngine/` | Computer vision and LLM Vision |

## Boot Sequence

HelixAgent follows a strict initialization order on startup:

1. **Configuration** -- Load `.env`, `configs/*.yaml`, and `containers/.env`
2. **Container Adapter** -- Initialize `internal/adapters/containers/adapter.go`, detect runtime (Docker/Podman)
3. **Container Orchestration** -- Start all infrastructure containers (local or remote per `containers/.env`)
4. **Health Checks** -- Verify PostgreSQL, Redis, ChromaDB, and required services are reachable
5. **Database Migrations** -- Apply SQL schemas from `sql/schema/`
6. **Provider Discovery** -- 3-tier model discovery (Provider API, models.dev, hardcoded fallback)
7. **LLMsVerifier Pipeline** -- Verify providers in parallel (8-test pipeline), score and rank
8. **Debate Team Selection** -- Select top providers for the AI debate ensemble
9. **Module Initialization** -- Initialize adapters for all active modules (lazy loading where possible)
10. **HTTP Server** -- Start Gin server with middleware (auth, rate limiting, CORS, compression)
11. **Background Services** -- Start Constitution watcher, cache warming, metric exporters

## Configuration Matrix

Modules are configured through environment variables and the adapter layer:

| Module | Config Source | Key Variables |
|--------|-------------|---------------|
| Database | `.env` | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` |
| Cache | `.env` | `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` |
| Containers | `containers/.env` | `CONTAINERS_REMOTE_ENABLED`, `CONTAINERS_REMOTE_HOST_*` |
| Observability | `.env` | `OTEL_EXPORTER_*`, `JAEGER_ENDPOINT` |
| Auth | `.env` | `JWT_SECRET`, `API_KEY_*` |
| LLM Providers | `.env` | `*_API_KEY`, `*_USE_OAUTH_CREDENTIALS` |
| HelixMemory | `.env` | Active by default; opt out with `-tags nohelixmemory` |
| HelixSpecifier | `.env` | Active by default; opt out with `-tags nohelixspecifier` |
| BigData | `.env` | `BIGDATA_ENABLE_*` (graceful degradation if deps missing) |
| MCP | `.env` + compose | 45+ adapters, ports 9101-9999 |

Service-level overrides use the `SVC_<SERVICE>_<FIELD>` pattern (e.g., `SVC_POSTGRESQL_HOST`, `SVC_REDIS_REMOTE=true`).

## Inter-Module Communication

Modules communicate through 4 mechanisms:

### 1. Adapter Layer

The `internal/adapters/` directory bridges internal types to extracted module interfaces. Each adapter converts between HelixAgent's internal types and the module's public API:

```
internal/adapters/
  containers/adapter.go     -- Container orchestration
  observability/adapter.go  -- OpenTelemetry integration
  events/adapter.go         -- EventBus integration
  http/adapter.go           -- HTTP/3 client pool
  ... (20+ adapter files, 75+ tests)
```

### 2. EventBus (Asynchronous)

Modules publish and subscribe to events for decoupled communication:

- Provider health changes trigger circuit breaker updates
- Cache invalidation events propagate across components
- Debate phase transitions emit audit trail events

### 3. Direct Interface Injection

Higher-phase modules accept lower-phase interfaces via constructor injection:

- RAG receives VectorDB and Embeddings interfaces
- HelixMemory receives Database, Cache, and EventBus interfaces
- DebateOrchestrator receives LLMProvider and Memory interfaces

### 4. HTTP/gRPC (Cross-Process)

Containerized services communicate over HTTP/3 (QUIC) with Brotli compression:

- MCP servers (ports 9101-9999)
- Formatter service endpoints (ports 9210-9300)
- Monitoring and health endpoints

## Validation

Verify module integration with:

```bash
# Build to confirm all modules compile together
make build

# Run adapter coverage validation
./challenges/scripts/adapter_coverage_challenge.sh

# Run full system boot to verify initialization order
./challenges/scripts/full_system_boot_challenge.sh

# Validate all module documentation is synchronized
./challenges/scripts/documentation_completeness_challenge.sh
```

## Troubleshooting

- **"module not found" during build**: Run `git submodule update --init --recursive` and verify `replace` directives in root `go.mod`
- **Boot hangs at container health check**: Check `containers/.env` for correct host/port configuration
- **Adapter test failures**: Ensure the target module's tests pass independently before testing the adapter
- **Circular dependency detected**: Modules must only depend on earlier phases; check `go.mod` for violations

## Related Resources

- [User Manual 01: Getting Started](01-getting-started.md) -- Initial setup
- [User Manual 05: Deployment Guide](05-deployment-guide.md) -- Production deployment
- Architecture docs: `docs/MODULES.md`
- Adapter layer: `internal/adapters/`
