# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

## Project Overview

HelixAgent is an AI-powered ensemble LLM service written in Go that combines responses from multiple language models using intelligent aggregation strategies. It provides OpenAI-compatible APIs and supports 43 LLM providers (Claude, Chutes, DeepSeek, Gemini, Mistral, OpenRouter, Qwen, ZAI, Zen, Cerebras, Ollama, AI21, Anthropic, Cohere, Fireworks, GitHub Models, Groq, HuggingFace, OpenAI, Perplexity, Replicate, Together, Venice, xAI, Junie, Cloudflare, Codestral, Hyperbolic, Kilo, Kimi, KimiCode, Modal, Nia, NLPCloud, Novita, Nvidia, PublicAI, SambaNova, Sarvam, SiliconFlow, Upstage, VulaVula, Zhipu) with **dynamic provider selection** based on LLMsVerifier verification scores.

**Module**: `dev.helix.agent` (Go 1.25.3)

Subprojects: **Toolkit** (`Toolkit/`) — Go library for AI apps. **LLMsVerifier** (`LLMsVerifier/`) — provider accuracy verification. Plus **41 extracted modules** (see [Extracted Modules](#extracted-modules-submodules) below) covering containers, challenges, concurrency, observability, auth, storage, streaming, security, vector databases, embeddings, database, cache, messaging, formatters, MCP, RAG, memory, optimization, plugins, event bus, agentic workflows, LLM operations, self-improvement, planning algorithms, benchmarking, background tasks, build checks, conversation context, debate orchestration, LLM provider interface, data models, skill registry, tool schemas, **HelixMemory** (unified cognitive memory engine), **HelixSpecifier** (spec-driven development fusion engine), documentation processing, QA orchestration, CLI agent management, computer vision, provider verification, and containerized MCP servers.

## Mandatory Development Standards

**These rules are NON-NEGOTIABLE and MUST be followed for every component, service, or feature.**

1. **100% Test Coverage** — Every component MUST have unit, integration, E2E, automation, security/penetration, and benchmark tests. No false positives. Mocks/stubs ONLY in unit tests; all other tests use real data and live services.
2. **Challenge Coverage** — Every component MUST have Challenge scripts (`./challenges/scripts/`) validating real-life use cases. No false success — validate actual behavior, not return codes.
3. **Containerization** — All services MUST run in containers (Docker/Podman/K8s). Must support local default execution AND remote configuration. Auto boot-up before HelixAgent is ready. Remote services need API-based health checks.
3a. **Centralized Container Management** — ALL container operations (runtime detection, compose up/down, health checks, remote distribution) MUST go through the Containers module (`digital.vasic.containers`) via `internal/adapters/containers/adapter.go`. No direct `exec.Command` to `docker`/`podman` in production code. The adapter delegates to the Containers module when available, with fallback to direct commands only in adapter internals.
3b. **MANDATORY Container Orchestration Flow (CRITICAL)** — This is the ONLY acceptable container orchestration flow. All tests and challenges MUST follow this pattern:
   - **Step 1**: HelixAgent boots and initializes Containers module adapter
   - **Step 2**: Adapter reads `Containers/.env` file (NOT project root `.env`)
   - **Step 3**: Based on `Containers/.env` content:
     - `CONTAINERS_REMOTE_ENABLED=true` → ALL containers distributed to remote host(s) via `CONTAINERS_REMOTE_HOST_*` vars. NO local containers started.
     - `CONTAINERS_REMOTE_ENABLED=false` or missing → ALL containers start locally
   - **Step 4**: Health checks performed against configured endpoints (local or remote)
   - **Step 5**: Required services failing health check cause boot failure in strict mode
   - **Rules**: NO manual container starts, NO mixed mode, tests use `tests/precondition/containers_boot_test.go`, challenges verify container placement based on `Containers/.env`
   - **Key Files**: `Containers/.env` (orchestration), `internal/config/config.go:isContainersRemoteEnabled()`, `internal/services/boot_manager.go`, `tests/precondition/containers_boot_test.go`
4. **Configuration via HelixAgent Only** — CLI agent config export uses only HelixAgent + LLMsVerifier's unified generator (`pkg/cliagents/`). No third-party scripts.
5. **Real Data** — Beyond unit tests, all components MUST use actual API calls, real databases, live services. No simulated success. Fallback chains tested with actual failures.
6. **Health & Observability** — Every service MUST expose health endpoints. Circuit breakers for all external deps. Prometheus/OpenTelemetry integration. Status via `/v1/monitoring/status`.
7. **Documentation & Quality** — Follow existing patterns. Update CLAUDE.md, AGENTS.md, relevant docs. Pass `make fmt vet lint security-scan`. Conventional Commits: `<type>(<scope>): <description>`.
8. **Validation Before Release** — Pass `make ci-validate-all`, `./challenges/scripts/run_all_challenges.sh`, `make test-with-infra`, and benchmark/stress tests.
9. **No Mocks or Stubs in Production** — Mocks, stubs, fakes, placeholder classes, TODO implementations STRICTLY FORBIDDEN in production code. All production code must be fully functional with real integrations. Only unit tests may use mocks/stubs.
10. **Third-Party Submodules** — `cli_agents/` and `MCP/` are read-only third-party deps; NEVER commit/push changes. Only project-owned submodules (LLMsVerifier, formatters) may be updated. Use `git submodule update --remote`.
11. **Container-Based Builds** — ALL release builds MUST be performed inside Docker/Podman containers for reproducibility. Use `make release` / `make release-all`. Version info injected via `-ldflags -X`.
11a. **MANDATORY Container Rebuild** — **All running containers on local host or remote distributed machines MUST be rebuilt and redeployed if code was changed affecting any of them!** After any code changes to services, handlers, MCPs, formatters, or any containerized component: (1) Rebuild affected images with `make docker-build` or `make container-build`, (2) Restart containers with `make docker-run` or `make container-start`, (3) If using remote distribution, re-run distribution with `CONTAINERS_REMOTE_ENABLED=true`. Failure to rebuild containers after code changes will result in outdated code running in production.
12. **Infrastructure Before Tests** — ALL infrastructure containers (PostgreSQL, Redis, Mock LLM) MUST be running before executing tests or challenges. Use `make test-infra-start` or `make test-infra-direct-start` (Podman fallback with `--userns=host`). Tests and challenges that require infrastructure WILL FAIL without running containers.
13. **Comprehensive Verification** — Every fix MUST be verified from all angles: runtime testing (actual HTTP requests), compile verification, code structure checks, npm/dependency existence checks, backward compatibility, and no false positives in tests or challenges. Grep-only validation is NEVER sufficient.
14. **HTTP/3 (QUIC) with Brotli Compression** — ALL HTTP communication MUST use HTTP/3 (QUIC) as primary transport with Brotli compression. HTTP/2 ONLY as fallback when HTTP/3 is unavailable. Compression: Brotli (primary) → gzip (fallback). All HTTP clients and servers MUST prefer HTTP/3. Use `quic-go/quic-go` for transport, `andybalholm/brotli` for compression.
15. **Resource Limits for Tests & Challenges (CRITICAL)** — ALL test and challenge execution MUST be strictly limited to 30-40% of host system resources. Use `GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`, `-p 1` for go test. Container limits required. The host runs mission-critical processes — exceeding limits has caused system crashes and forced resets.

## Git Rules

- **SSH ONLY for ALL Git operations** — **MANDATORY: NEVER use HTTPS for any Git service operations.** All cloning, fetching, pushing, and submodule operations MUST use SSH URLs (`git@github.com:org/repo.git`). HTTPS is STRICTLY FORBIDDEN even for public repositories. SSH keys are already configured on all Git services (GitHub, GitLab, etc.).
- **Branch naming**: `feat/`, `fix/`, `chore/`, `docs/`, `refactor/`, `test/` + short description
- **Commits**: Conventional Commits — `feat(llm): add ensemble voting strategy`
- **Always run `make fmt vet lint` before committing**

## Code Style

- Standard Go conventions ([Effective Go](https://go.dev/doc/effective_go)), `gofmt` formatting
- Imports grouped: stdlib, third-party, internal (blank line separated). Use `goimports`.
- Line length ≤ 100 chars (readability first)
- Naming: `camelCase` private, `PascalCase` exported, `UPPER_SNAKE_CASE` constants, acronyms all-caps (`HTTP`, `URL`, `ID`)
- Receivers: 1-2 letters (`s` for service, `c` for client)
- Errors: always check, wrap with `fmt.Errorf("...: %w", err)`, `defer` for cleanup
- Interfaces: small/focused, accept interfaces return structs
- Concurrency: `context.Context` always, `sync.Mutex`/`sync.RWMutex` for shared data
- Tests: table-driven, `testify`, naming `Test<Struct>_<Method>_<Scenario>`

## Build & Run

```bash
make build                # Build binary
make build-debug          # Build with debug symbols
make run                  # Run locally
make run-dev              # Development mode (GIN_MODE=debug)
make docker-build         # Build Docker image
docker-compose up -d      # Start full stack
```

### Release Builds

All release builds MUST be performed inside Docker/Podman containers for reproducibility.

```bash
make release              # Build helixagent for all platforms
make release-all          # Build ALL 7 apps for all platforms
make release-<app>        # Build a specific app (helixagent, api, grpc-server, ...)
make release-force        # Force rebuild all (ignore change detection)
make release-info         # Show version codes and source hashes
make release-clean        # Clean release artifacts (keep version data)
make release-builder-image # Build the builder container image
```

## Testing

**IMPORTANT:** Infrastructure containers (PostgreSQL, Redis, Mock LLM) MUST be running before executing tests or challenges. Start them with `make test-infra-start` (or `make test-infra-direct-start` for Podman rootless fallback).

```bash
make test                 # All tests (auto-detects infra)
make test-unit            # Unit tests (./internal/... -short)
make test-integration     # Integration tests (./tests/integration)
make test-e2e             # E2E tests (./tests/e2e)
make test-security        # Security tests (./tests/security)
make test-stress          # Stress tests (./tests/stress)
make test-chaos           # Challenge tests (./tests/challenge)
make test-bench           # Benchmarks
make test-fuzz            # Fuzz tests (corpus replay)
make test-race            # Race detection
make test-coverage        # Coverage with HTML report
```

Single test: `go test -v -run TestName ./path/to/package`

With infrastructure:
```bash
make test-infra-start     # Start PostgreSQL, Redis, Mock LLM containers
DB_HOST=localhost DB_PORT=15432 DB_USER=helixagent DB_PASSWORD=helixagent123 DB_NAME=helixagent_db \
REDIS_HOST=localhost REDIS_PORT=16379 REDIS_PASSWORD=helixagent123 \
go test -v -run TestName ./path/to/package
make test-infra-stop      # Stop containers
make test-with-infra      # All tests with Docker infra
```

## Code Quality & CI

```bash
make fmt                  # go fmt
make vet                  # go vet
make lint                 # golangci-lint
make security-scan        # gosec
make install-deps         # Install dev tools
make ci-validate-all      # All validation checks
make ci-pre-commit        # Pre-commit (fmt, vet, fallback)
make ci-pre-push          # Pre-push (includes unit tests)
```

## Infrastructure & Monitoring

```bash
make infra-start          # Start ALL infra (auto-detects Docker/Podman)
make infra-stop / restart / status
make infra-core           # Core: PostgreSQL, Redis, ChromaDB, Cognee
make infra-mcp / lsp / rag
make monitoring-status / circuit-breakers / provider-health / fallback-chain
make monitoring-reset-circuits / force-health-check
```

## Architecture

### Entry Points (7 apps)
- `cmd/helixagent/` — Main app | `cmd/api/` — API server | `cmd/grpc-server/` — gRPC
- `cmd/cognee-mock/` — Cognee mock server | `cmd/sanity-check/` — Sanity checker
- `cmd/mcp-bridge/` — MCP bridge | `cmd/generate-constitution/` — Constitution generator

### Core Packages (`internal/`)
- `llm/providers/` — 43 dedicated LLM providers (ai21, anthropic, cerebras, chutes, claude, cloudflare, codestral, cohere, deepseek, fireworks, gemini (unified: API+CLI+ACP), githubmodels, groq, huggingface, hyperbolic, junie, kilo, kimi, kimicode, mistral, modal, nia, nlpcloud, novita, nvidia, ollama, openai, openrouter, perplexity, publicai, qwen, replicate, sambanova, sarvam, siliconflow, together, upstage, venice, vulavula, xai, zai, zen, zhipu) + generic OpenAI-compatible provider
- `llm/providers/generic/` — Generic OpenAI-compatible provider for verification of providers without dedicated implementations
- `llm/discovery/` — 3-tier dynamic model discovery (Provider API → models.dev → hardcoded fallback)
- `llm/ensemble.go` — Ensemble orchestration
- `services/` — Business logic: provider_registry, ensemble, debate_service, debate_team_config, llm_intent_classifier, context_manager, mcp_client, lsp_manager, plugin_system
- `handlers/` — HTTP handlers (BackgroundTask, Discovery, Scoring, Verification, Health + core) | `middleware/` — Auth, rate limiting, CORS
- `handlers/agentic_handler.go` — Agentic workflow endpoints | `handlers/planning_handler.go` — AI planning endpoints
- `handlers/llmops_handler.go` — LLM operations endpoints | `handlers/benchmark_handler.go` — Benchmarking endpoints
- `background/` — Task queue, worker pool, resource monitor | `notifications/` — SSE, WebSocket, Webhooks
- `cache/` — Redis + in-memory | `database/` — PostgreSQL/pgx | `models/` — Data models/enums
- `debate/` — Orchestrator framework: agents, topology, protocol, voting, cognitive, knowledge, reflexion, gates, evaluation, audit, tools (13 packages)
- `formatters/` — 32+ code formatters, REST API, middleware executor
- `tools/` — Tool schema registry (21 tools) | `agents/` — CLI agent registry (48 agents)
- `embedding/` — 6 providers (OpenAI, Cohere, Voyage, Jina, Google, Bedrock)
- `vectordb/` — Qdrant, Pinecone, Milvus, pgvector
- `mcp/adapters/` — 45+ MCP adapters | `mcp/config/` — Container config generator
- `rag/` — Hybrid retrieval | `memory/` — Mem0-style with entity graphs
- `routing/semantic/` — Embedding similarity routing
- `agentic/` — Graph-based workflow orchestration
- `security/` — Red team framework, guardrails, PII detection
- `observability/` — OpenTelemetry, Jaeger, Zipkin, Langfuse
- `bigdata/` — Infinite context, distributed memory, knowledge graph streaming
- `optimization/` — gptcache, outlines, streaming, sglang, llamaindex, langchain
- `verifier/` — Startup verification orchestrator and adapters
- `challenges/` — HelixAgent-specific challenge implementations (plugin, infra bridge, shell adapter, 22 Go-native userflow challenges with dependency graph)
- `adapters/` — Bridge layer connecting internal types to extracted modules (20+ adapter files with 75+ tests)
- `adapters/observability/` — OpenTelemetry integration adapter | `adapters/events/` — EventBus integration adapter | `adapters/http/` — HTTP/3 client pool adapter | `adapters/helixqa/` — HelixQA autonomous QA pipeline adapter

### Extracted Modules (submodules)

Each module is an independent Go module with its own go.mod, tests, CLAUDE.md, AGENTS.md, README.md, and docs/. All use `replace` directives in the root go.mod for local development. See `docs/MODULES.md` for the full catalog.

**Foundation (Phase 1 — zero dependencies):**
- **EventBus** (`EventBus/`, `digital.vasic.eventbus`) — Pub/sub event system: bus, event types, filtering, middleware chain. 4 packages.
- **Concurrency** (`Concurrency/`, `digital.vasic.concurrency`) — Worker pools, priority queues, rate limiters (token bucket/sliding window), circuit breakers, semaphores, resource monitoring. 6 packages.
- **Observability** (`Observability/`, `digital.vasic.observability`) — OpenTelemetry tracing, Prometheus metrics, structured logging, health checks, ClickHouse analytics. 5 packages.
- **Auth** (`Auth/`, `digital.vasic.auth`) — JWT, API key, OAuth authentication; HTTP middleware; token management. 5 packages.
- **Storage** (`Storage/`, `digital.vasic.storage`) — Object storage abstraction: S3/MinIO, local filesystem, cloud providers. 4 packages.
- **Streaming** (`Streaming/`, `digital.vasic.streaming`) — SSE, WebSocket, gRPC streaming, webhooks, HTTP client, transport abstraction. 6 packages.

**Infrastructure (Phase 2 — zero module dependencies, complex):**
- **Security** (`Security/`, `digital.vasic.security`) — Guardrails engine, PII detection/redaction, content filtering, policy enforcement, vulnerability scanning. 5 packages.
- **VectorDB** (`VectorDB/`, `digital.vasic.vectordb`) — Unified vector store: Qdrant, Pinecone, Milvus, pgvector adapters; similarity search, collection management. 5 packages.
- **Embeddings** (`Embeddings/`, `digital.vasic.embeddings`) — 6 embedding providers (OpenAI, Cohere, Voyage, Jina, Google, Bedrock); batch embedding. 7 packages.
- **Database** (`Database/`, `digital.vasic.database`) — PostgreSQL (pgx), SQLite, connection pooling, migrations, repository pattern, query builder. 7 packages.
- **Cache** (`Cache/`, `digital.vasic.cache`) — Redis + in-memory caching, distributed cache, TTL policies, cache warming. 5 packages.

**Services (Phase 3):**
- **Messaging** (`Messaging/`, `digital.vasic.messaging`) — Kafka + RabbitMQ: unified broker, producer/consumer, dead letter queues, retry policies. 5 packages.
- **Formatters** (`Formatters/`, `digital.vasic.formatters`) — Code formatter framework: native/service/built-in formatters, registry, executor, caching. 6 packages.
- **MCP** (`MCP_Module/`, `digital.vasic.mcp`) — Model Context Protocol: adapter framework, client/server, config generation, registry, JSON-RPC protocol. 6 packages.

**Integration (Phase 4):**
- **RAG** (`RAG/`, `digital.vasic.rag`) — Retrieval-Augmented Generation: chunking, retrieval, reranking, hybrid search, pipeline composition. 5 packages.
- **Memory** (`Memory/`, `digital.vasic.memory`) — Mem0-style memory: entity graph, semantic search, memory scopes, consolidation. 4 packages.
- **Optimization** (`Optimization/`, `digital.vasic.optimization`) — GPT-Cache, Outlines structured output, streaming optimization, SGLang, prompt optimization. 6 packages.
- **Plugins** (`Plugins/`, `digital.vasic.plugins`) — Plugin system: interface + lifecycle, registry, dynamic loading, sandboxing, structured output parsing. 5 packages.

**AI/ML (Phase 5):**
- **Agentic** (`Agentic/`, `digital.vasic.agentic`) — Graph-based agentic workflow orchestration: multi-step execution, conditional branching, state management. 1 package.
- **LLMOps** (`LLMOps/`, `digital.vasic.llmops`) — LLM operations: continuous evaluation, A/B experiment management, dataset management, prompt versioning. 1 package (5 files).
- **SelfImprove** (`SelfImprove/`, `digital.vasic.selfimprove`) — AI self-improvement: reward modelling, RLHF feedback integration, optimizer, dimension-weighted scoring. 1 package (5 files).
- **Planning** (`Planning/`, `digital.vasic.planning`) — AI planning algorithms: hierarchical planning (HiPlan), Monte Carlo Tree Search (MCTS), Tree of Thoughts. 1 package (3 files).
- **Benchmark** (`Benchmark/`, `digital.vasic.benchmark`) — LLM benchmarking: SWE-bench, HumanEval, MMLU and custom benchmarks; leaderboard, provider comparison. 1 package (3 files).

**Cognitive (Phase 6 — unified memory fusion):**
- **HelixMemory** (`HelixMemory/`, `digital.vasic.helixmemory`) — **DEFAULT** unified cognitive memory engine for HelixAgent and AI debate ensemble. Orchestrates Mem0 (facts), Cognee (knowledge graphs), Letta (stateful agent runtime), Graphiti (temporal graph) through 3-stage fusion pipeline. 12 power features, circuit breakers, Prometheus metrics, infra bridge. Active by default; opt out with `-tags nohelixmemory`. 12+ packages.

**Specification (Phase 7 — spec-driven development):**
- **HelixSpecifier** (`HelixSpecifier/`, `digital.vasic.helixspecifier`) — Spec-Driven Development Fusion Engine: 3-pillar architecture (SpecKit 7-phase SDD, Superpowers TDD/subagents, GSD milestones), adaptive ceremony scaling, effort classification, CLI agent adapters, 10 power features, spec memory, DebateFunc injection for real multi-LLM debate, 3-pillar fusion (SpecKit + Superpowers + GSD), intent classifier. 27 packages (21 core + 6 test suites). 835+ tests (unit, integration, E2E, security, stress, benchmark, automation). Active by default; opt out with `-tags nohelixspecifier`.

**Core Abstractions (Phase 8 — shared types and interfaces):**
- **LLMProvider** (`LLMProvider/`, `digital.vasic.llmprovider`) — Core LLM provider interface and utilities: circuit breakers, health monitoring, retry logic, lazy loading patterns.
- **Models** (`Models/`, `digital.vasic.models`) — Core data types and structures for AI/LLM applications, agent systems, and related services.
- **ToolSchema** (`ToolSchema/`, `digital.vasic.toolschema`) — Tool schema definition, validation, and execution: unified interface for AI agent tool systems with safety and validation.
- **SkillRegistry** (`SkillRegistry/`, `digital.vasic.skillregistry`) — CLI agent skill registration and management: skill definitions, capabilities, configuration templates, multi-agent coordination.
- **BackgroundTasks** (`BackgroundTasks/`, `digital.vasic.background`) — Background task processing: persistence, resource monitoring, stuck detection, event publishing.
- **ConversationContext** (`ConversationContext/`, `digital.vasic.conversation`) — Conversation context management: compression, infinite context via event sourcing, Kafka replay, LLM-based summarization.
- **DebateOrchestrator** (`DebateOrchestrator/`, `digital.vasic.debate`) — Multi-agent AI debate framework: consensus-building, Reflexion, adversarial dynamics, dehallucination, self-evolvement, 8-phase protocol.
- **BuildCheck** (`BuildCheck/`, `digital.vasic.buildcheck`) — Content-based change detection for container image builds: SHA256 hash comparison, rebuild determination.

**Pre-existing:**
- **Containers** (`Containers/`, `digital.vasic.containers`) — Generic container orchestration: runtime abstraction (Docker/Podman/K8s), health checking, compose orchestration, lifecycle management. 12 packages.
- **Challenges** (`Challenges/`, `digital.vasic.challenges`) — Generic challenge framework: assertion engine (19 evaluators), registry, runner, reporting, monitoring, metrics, plugin system v2.0.0, userflow testing (21 adapters across 8 interfaces: browser/mobile/desktop/API/gRPC/WebSocket/build), Panoptic vision/recorder/testgen/error-analyzer adapters, AI test generation challenges. 16 packages.
- **DocProcessor** (`DocProcessor/`, `digital.vasic.docprocessor`) — Documentation processing, feature map extraction, and coverage tracking: parser, feature map indexing, coverage reports, diff detection. 6 packages.
- **HelixQA** (`HelixQA/`, `digital.vasic.helixqa`) — QA orchestration framework: autonomous LLM-driven testing, crash detection, evidence collection, ticket generation, quality monitoring, multi-device parallel QA, credential discovery, Playwright web testing, vision analysis. 22 packages.
- **LLMOrchestrator** (`LLMOrchestrator/`, `digital.vasic.llmorchestrator`) — CLI agent management with hybrid pipe+file protocol: lifecycle management, circuit breakers, agent registry, request routing. 6 packages.
- **VisionEngine** (`VisionEngine/`, `digital.vasic.visionengine`) — Computer vision and UI analysis: NavigationGraph, LLM vision providers (GPT-4V, Gemini Vision, Claude), UI element detection, remote vision pool (Ollama/llama.cpp multi-instance). 6 packages.
- **LLMsVerifier** (`LLMsVerifier/`, `digital.vasic.llmsverifier`) — Provider accuracy verification, 5-component scoring pipeline, 3-tier subscription detection, unified CLI agent config generator (48 agents). 10+ packages.
- **MCP-Servers** (`MCP-Servers/`, collection) — 60+ containerized MCP server implementations: filesystem, memory, sequential-thinking, database, vision, embeddings, RAG, formatters, monitoring.

### Key Interfaces
- `LLMProvider` — Provider contract (Complete, CompleteStream, HealthCheck, GetCapabilities, ValidateConfig)
- `VotingStrategy` — Ensemble voting | `CacheInterface` — Cache abstraction
- `PluginRegistry`/`PluginLoader` — Plugin system | `TaskExecutor`/`TaskQueue` — Background tasks
- `Formatter` — Code formatter interface | Vector stores: `Connect`, `Upsert`, `Search`, `Delete`, `Get`

### Goroutine Lifecycle Safety
All HTTP handlers with background goroutines implement graceful shutdown via `sync.WaitGroup` lifecycle tracking. Key pattern: `WaitGroup.Add(1)` before goroutine launch, `defer WaitGroup.Done()` inside the goroutine, `Shutdown()`/`Stop()` calls `cancel()` + `WaitGroup.Wait()`. This pattern prevents goroutine leaks and ensures all background work completes before process exit. Applied across SSE handlers, cache invalidation, model refresh, debate log tracking, and ACP shutdown. Diagram: `docs/diagrams/src/goroutine-lifecycle.puml`.

### Release Build System
- **Version Package**: `internal/version/` — single source of truth, set via `-ldflags -X` at build time
- **Container Builds**: All release builds run inside `helixagent-builder` container (golang:1.24-alpine)
- **Change Detection**: SHA256 hash of source files; skips build when unchanged. Version codes auto-increment per app.
- **7 Apps**: helixagent, api, grpc-server, cognee-mock, sanity-check, mcp-bridge, generate-constitution
- **5 Platforms**: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- **Output**: `releases/<app>/<os>-<arch>/<version_code>/<binary>` + `build-info.json`, `latest` symlink
- Key files: `VERSION`, `internal/version/version.go`, `scripts/build/`, `docker/build/Dockerfile.builder`
- Docs: `docs/development/RELEASE_BUILD_GUIDE.md`

### Architectural Patterns
- **Provider Registry**: Unified multi-provider interface with credential management
- **Ensemble Strategy**: Confidence-weighted voting, majority vote, parallel execution
- **AI Debate**: Multi-round debate, 5 positions × 5 LLMs = 25 total, multi-pass validation (Initial → Validation → Polish → Final)
- **Debate Orchestrator**: Multi-topology (mesh/star/chain/tree), 8-phase protocol (Dehallucination → SelfEvolvement → Proposal → Critique → Review → Optimization → Adversarial → Convergence), cross-debate learning, auto-fallback to legacy
- **Debate Reflexion Framework**: Episodic memory buffer, verbal reflection generator, retry-and-learn loop, accumulated wisdom for cross-session learning
- **Debate Adversarial Dynamics**: Red/Blue team multi-round attack-defend cycles with structured reports
- **Debate Approval Gates**: Configurable human-in-the-loop gates with REST API (approve/reject/gates endpoints), disabled by default
- **Debate Voting Methods**: 6 methods — Weighted (MiniMax), Majority, Borda Count, Condorcet (cycle detection + Borda fallback), Plurality, Unanimous + AutoSelectMethod
- **Debate Persistence**: PostgreSQL tables (debate_sessions, debate_turns, code_versions) with full repositories
- **Debate Provenance & Audit**: Full reproducibility tracking with 14 event types, session summaries, JSON export
- **Debate Benchmark Bridge**: SWE-bench/HumanEval/MMLU evaluation with static code analysis for 5 metrics
- **Debate CI/CD Hooks**: Configurable validation pipelines per phase (tests, linting, static analysis, security scan)
- **Debate Git Worktree Tool**: Isolated session version control with snapshot commits and diff generation
- **Debate Performance Optimizer**: Parallel LLM execution with semaphore limiting, response caching with TTL, smart fallback chain traversal, early termination on consensus detection, and comprehensive stats tracking. Key file: `internal/services/debate_performance_optimizer.go`
- **SpecKit Auto-Activation**: 7-phase development flow (Constitution → Specify → Clarify → Plan → Tasks → Analyze → Implement) triggered automatically for large changes/refactoring based on work granularity detection (5 levels: single action, small creation, big creation, whole functionality, refactoring). Phase caching for resumption. Key files: `internal/services/speckit_orchestrator.go`, `enhanced_intent_classifier.go`, `debate_service_speckit_e2e_test.go`
- **Constitution Management**: Auto-update Constitution on project changes (new modules, documentation changes, structure changes). Background watcher monitors filesystem. Key files: `internal/services/constitution_watcher.go`, `constitution_manager.go`, `documentation_sync.go`
- **Circuit Breaker**: Fault tolerance for provider failures
- **Semantic Intent Detection**: LLM-based classification (zero hardcoding), pattern-based fallback
- **Fallback Error Reporting**: Categorized errors (rate_limit, timeout, auth, connection, unavailable, overloaded) in streamed responses
- **Dynamic Model Discovery**: 3-tier model discovery for all providers — Tier 1: query provider's `/v1/models` API, Tier 2: query models.dev catalog, Tier 3: hardcoded fallback. Thread-safe caching with 1-hour TTL. Custom response parsers for non-OpenAI formats (Gemini, Ollama, Cohere, Replicate, ZAI). Key package: `internal/llm/discovery/`

## Startup Verification Pipeline

LLMsVerifier is the **single source of truth**. On startup: discover providers → verify in parallel (8-test pipeline) → score (5 weighted components) → rank → select debate team → start server.

**Provider types**: API Key (DeepSeek, Gemini, Mistral, OpenRouter, ZAI, Cerebras), OAuth (Claude, Qwen), Free (Zen, OpenRouter :free)

**Scoring weights**: ResponseSpeed 25%, CostEffectiveness 25%, ModelEfficiency 20%, Capability 20%, Recency 10%. OAuth +0.5 bonus. Free: 6.0-7.0. Min score: 5.0.

Key files: `internal/verifier/startup.go`, `provider_types.go`, `adapters/oauth_adapter.go`, `adapters/free_adapter.go`

**Subscription Detection**: 3-tier dynamic detection (API → rate limit headers → static). Subscription types: `free`, `free_credits`, `free_tier`, `pay_as_you_go`, `monthly`, `enterprise`. Per-provider auth mechanism configs (Bearer, `x-api-key`, `x-goog-api-key`, anonymous). Rate limit header parsing for 6+ providers. Key files: `internal/verifier/subscription_types.go`, `subscription_detector.go`, `provider_access.go`, `rate_limit_headers.go`

## Provider Access Mechanisms

OAuth/free providers use CLI proxies when direct API access is restricted:
- **Claude**: `claude -p --output-format json` (session continuity) — `internal/llm/providers/claude/claude_cli.go`
- **Qwen**: ACP via `qwen --acp` (JSON-RPC 2.0), fallback CLI — `internal/llm/providers/qwen/qwen_acp.go`
- **Zen**: HTTP server `opencode serve :4096`, fallback CLI — `internal/llm/providers/zen/zen_http.go`
- **Gemini**: `gemini -p --output-format json` (headless mode), ACP via `gemini --experimental-acp` — `internal/llm/providers/gemini/gemini_cli.go`
- **Junie**: CLI mode with `--output-format json` and ACP mode via `junie --acp` — `internal/llm/providers/junie/junie_cli.go`

Triggered when: `*_USE_OAUTH_CREDENTIALS=true` + no API key, or no `OPENCODE_API_KEY` for Zen, or `JUNIE_API_KEY` for Junie, or no `GEMINI_API_KEY` for Gemini CLI/ACP.

**OAuth limitation**: CLI OAuth tokens are product-restricted (cannot use for general API). Get proper API keys from console.anthropic.com / dashscope.aliyuncs.com, or use CLI proxy.

## Configuration

Env vars in `.env.example`: `PORT`, `GIN_MODE`, `JWT_SECRET`, `DB_*`, `REDIS_*`, `*_API_KEY` for each provider, `*_USE_OAUTH_CREDENTIALS`, `COGNEE_ENABLED` (off by default; Mem0 is primary memory), `CONSTITUTION_WATCHER_ENABLED` (Constitution auto-update), `CONSTITUTION_WATCHER_CHECK_INTERVAL` (default: 5m).

Service overrides: `SVC_<SERVICE>_<FIELD>` (e.g., `SVC_POSTGRESQL_HOST`, `SVC_REDIS_REMOTE=true`). Config files: `configs/development.yaml`, `configs/production.yaml`.

BigData components configured via `BIGDATA_ENABLE_*` env vars. Missing deps (Neo4j, ClickHouse, Kafka) gracefully degrade. Key file: `internal/bigdata/integration.go`.

**SpecKit Configuration**: Auto-activation threshold configured via `WorkGranularity` detection. Triggered for `GranularityBigCreation`, `GranularityWholeFunctionality`, `GranularityRefactoring`. Phase caching enabled by default, stored in `.speckit/cache/`.

## Adding a New LLM Provider

1. Create `internal/llm/providers/<name>/<name>.go` implementing `LLMProvider`
2. Add tool support if applicable (`SupportsTools: true` in GetCapabilities)
3. Register in `internal/services/provider_registry.go`
4. Add env vars to `.env.example`, tests in `internal/llm/providers/<name>/<name>_test.go`

## Tool Schema

All parameters use **snake_case**. Key files: `internal/tools/schema.go`, `internal/tools/handler.go`.

## CLI Agents (48)

Registry: `internal/agents/registry.go`. Generate configs: `./bin/helixagent --generate-agent-config=<name>`. All agents include formatters config. Config generation via LLMsVerifier's `pkg/cliagents/`.

### CLI Agent Config Rules (MANDATORY)

1. **Config filenames**: `opencode.json` (WITHOUT leading dot), `crush.json`, etc. OpenCode v1.2.6+ does NOT recognize `.opencode.json` (with dot).
2. **No env var syntax in API keys**: CLI agents do NOT support `{env:VAR_NAME}` syntax. Generated configs for installation MUST contain the real API key value from `.env`.
3. **Two config versions**: Repository examples in `configs/cli-agents/` use `<YOUR_HELIXAGENT_API_KEY>` as placeholder. Installed configs (e.g., `~/.config/opencode/opencode.json`) use real API key values.
4. **Config locations**: OpenCode: `~/.config/opencode/opencode.json`. Crush: `~/.config/crush/crush.json`. Both use `http://localhost:7061/v1` as provider base URL.
5. **Model ID format**: Provider-qualified model references use `helixagent/helixagent-debate` format (provider-id/model-id).
6. **15+ MCP servers**: ALL 48 CLI agents MUST ship with at least 15 MCP servers: 6 HelixAgent remote (mcp, acp, lsp, embeddings, vision, cognee), 3 extended (rag, formatters, monitoring), 6 local npx (filesystem, memory, sequential-thinking, everything, puppeteer, sqlite), 3 free remote (context7, deepwiki, cloudflare-docs).
7. **10+ Plugins**: ALL agents MUST include HelixAgent plugins: helixagent-mcp, helixagent-lsp, helixagent-acp, helixagent-embeddings, helixagent-vision, helixagent-rag, helixagent-formatters, helixagent-debate, helixagent-memory, helixagent-monitoring.
8. **Extensions**: ALL agents MUST include enabled LSP, ACP, Embeddings, RAG, and 8+ Skills (code-review, code-format, semantic-search, vision-analysis, memory-recall, rag-retrieval, lsp-diagnostics, agent-communication).
9. **No hardcoding**: All config values come from the generator system (`LLMsVerifier/llm-verifier/pkg/cliagents/`). No hardcoded values or placeholders in exported configs.
10. **Challenge**: `./challenges/scripts/cli_agent_config_challenge.sh` validates all 48 agents have required features.

## Code Formatters

32+ formatters (11 native, 14 service, 7 built-in) for 19 languages. REST API: `POST /v1/format`, `GET /v1/formatters`. Service formatters in Docker (ports 9210-9300). Core: `internal/formatters/` (interface, registry, executor, cache, system). Native providers: `internal/formatters/providers/native/`. AI debate integration: `internal/services/debate_formatter_integration.go`.

## MCP Adapters

45+ adapters in `internal/mcp/adapters/`. 65+ containerized MCP servers (ports 9101-9999, zero npx). Container config: `internal/mcp/config/generator_container.go`. Compose: `docker/mcp/docker-compose.mcp-full.yml`.

## CI/CD Container Build System

Five-phase CI/CD system running **all builds, tests, and artifact generation inside Docker/Podman containers**. See `docs/CI_BUILD_GUIDE.md` for full documentation.

```bash
make ci-all              # All five phases + report aggregation
make ci-go               # Phase 1: Go builds + all tests + integration services
make ci-mobile           # Phase 2: Flutter/RN + Robolectric + Android emulator E2E
make ci-web              # Phase 3: Angular + Website + JS SDK + Playwright + Lighthouse
make ci-desktop          # Phase 4: Electron/Tauri desktop apps
make ci-integration      # Phase 5: Full-stack integration tests
make ci-report           # Aggregate reports into summary.html + results.json
make ci-build-images     # Build all CI container images
make ci-clean            # Remove CI containers, networks, volumes
CI_RESOURCE_LIMIT=medium make ci-all  # Medium resource limits (default: low)
```

**Compose file:** `docker-compose.ci.yml` with profiles: `go-ci`, `mobile-ci`, `web-ci`, `desktop-ci`, `integration`, `report`, `infra`.

**Integration services** (started automatically): PostgreSQL, Redis, Mock LLM, OAuth Mock, ChromaDB, Qdrant, Kafka, RabbitMQ, MinIO.

**Signing:** Default Android keystore at `keys/android/debug.keystore`. Release signing via `CI_ANDROID_RELEASE_KEYSTORE` env var.

**Reports:** `reports/summary.html` (dashboard), `reports/results.json` (machine-readable), `reports/<phase>/` (per-phase details).

**False positive prevention:** 6-layer validation (exit codes, test counts, coverage gates, artifact integrity, integration liveness, report cross-validation). Thresholds: `ci/thresholds.json`.

**Resource control:** `CI_RESOURCE_LIMIT` env var (`low`=30%, `medium`=50%, `high`=70% of host resources). Default: `low`.

## Challenges

**IMPORTANT:** Infrastructure containers MUST be running before executing challenges. Start with `make test-infra-start` or `make test-infra-direct-start`.

```bash
./challenges/scripts/run_all_challenges.sh                       # All challenges
./challenges/scripts/release_build_challenge.sh                  # 25 tests
./challenges/scripts/unified_verification_challenge.sh           # 15 tests
./challenges/scripts/llms_reevaluation_challenge.sh              # 26 tests
./challenges/scripts/debate_team_dynamic_selection_challenge.sh  # 12 tests
./challenges/scripts/semantic_intent_challenge.sh                # 19 tests
./challenges/scripts/fallback_mechanism_challenge.sh             # 17 tests
./challenges/scripts/integration_providers_challenge.sh          # 47 tests
./challenges/scripts/all_agents_e2e_challenge.sh                 # 102 tests
./challenges/scripts/full_system_boot_challenge.sh               # 53 tests
./challenges/scripts/cli_proxy_challenge.sh                      # 50 tests
./challenges/scripts/grpc_service_challenge.sh                   # 9 tests
./challenges/scripts/bigdata_comprehensive_challenge.sh          # 23 tests
./challenges/scripts/memory_system_challenge.sh                  # 14 tests
./challenges/scripts/mem0_migration_challenge.sh                 # Mem0 migration verification
./challenges/scripts/security_scanning_challenge.sh              # 10 tests
./challenges/scripts/constitution_watcher_challenge.sh           # 12 tests
./challenges/scripts/speckit_auto_activation_challenge.sh        # 15 tests
./challenges/scripts/verification_failure_reasons_challenge.sh   # 15 tests
./challenges/scripts/subscription_detection_challenge.sh        # 20 tests
./challenges/scripts/provider_comprehensive_challenge.sh        # 40 tests
./challenges/scripts/provider_url_consistency_challenge.sh      # 20 tests
./challenges/scripts/cli_agent_config_challenge.sh              # 60 tests
./challenges/scripts/debate_reflexion_challenge.sh              # 12 tests
./challenges/scripts/debate_adversarial_dynamics_challenge.sh   # 10 tests
./challenges/scripts/debate_tree_topology_challenge.sh          # 10 tests
./challenges/scripts/debate_dehallucination_challenge.sh        # 10 tests
./challenges/scripts/debate_self_evolvement_challenge.sh        # 10 tests
./challenges/scripts/debate_condorcet_voting_challenge.sh       # 10 tests
./challenges/scripts/debate_approval_gate_challenge.sh          # 12 tests
./challenges/scripts/debate_persistence_challenge.sh            # 13 tests
./challenges/scripts/debate_benchmark_integration_challenge.sh  # 10 tests
./challenges/scripts/debate_provenance_audit_challenge.sh       # 12 tests
./challenges/scripts/debate_deadlock_detection_challenge.sh     # 8 tests
./challenges/scripts/debate_git_integration_challenge.sh        # 11 tests
./challenges/scripts/debate_orchestrator_challenge.sh           # 61 tests
./challenges/scripts/debate_performance_optimizer_challenge.sh   # 36 tests
./challenges/scripts/helixmemory_challenge.sh                   # 80+ tests
./challenges/scripts/helixspecifier_challenge.sh                # 138 tests
./challenges/scripts/userflow_comprehensive_challenge.sh        # 30+ tests (shell)
./challenges/scripts/ci_container_build_challenge.sh            # 87 tests
./challenges/scripts/goroutine_lifecycle_challenge.sh           # Goroutine lifecycle validation
./challenges/scripts/adapter_coverage_challenge.sh              # Adapter test coverage validation
./challenges/scripts/race_condition_challenge.sh                # Race condition detection
./challenges/scripts/router_completeness_challenge.sh           # Router handler registration validation
./challenges/scripts/resource_limits_challenge.sh               # Test resource limit enforcement
./challenges/scripts/documentation_completeness_challenge.sh    # Documentation sync validation
./challenges/scripts/snyk_automated_scanning_challenge.sh       # 38 tests - Snyk containerized scanning
./challenges/scripts/sonarqube_automated_scanning_challenge.sh  # 45 tests - SonarQube containerized scanning
./challenges/scripts/pprof_memory_profiling_challenge.sh        # Memory leak detection via pprof
./challenges/scripts/coverage_gate_challenge.sh                 # Coverage threshold enforcement
./challenges/scripts/lazy_loading_validation_challenge.sh       # sync.Once lazy loading validation
./challenges/scripts/monitoring_dashboard_challenge.sh          # Prometheus metrics and dashboard validation
./challenges/scripts/dead_code_elimination_challenge.sh        # 15 tests
./challenges/scripts/concurrency_safety_comprehensive_challenge.sh  # 20 tests
./challenges/scripts/new_endpoints_challenge.sh                # 12 tests
./challenges/scripts/helixqa_integration_challenge.sh          # 50 tests
./challenges/scripts/concurrency_fixes_validation_challenge.sh     # 15 tests
./challenges/scripts/security_scan_validation_challenge.sh         # 10 tests
./challenges/scripts/documentation_completeness_validation_challenge.sh  # 12 tests
./challenges/scripts/output_pipeline_challenge.sh                       # 11 tests
./challenges/scripts/container_lazy_loading_challenge.sh                # 13 tests
./challenges/scripts/ensemble_handler_challenge.sh                      # 16 tests
./challenges/scripts/browser_handler_challenge.sh                       # 11 tests
./challenges/scripts/search_handler_challenge.sh                        # 13 tests
./challenges/scripts/fuzz_test_validation_challenge.sh                  # 16 tests
./challenges/scripts/feature_flag_challenge.sh                          # 13 tests
./challenges/scripts/env_var_completeness_challenge.sh                  # 22 tests
./challenges/scripts/safety_regression_challenge.sh                     # 16 tests
./challenges/scripts/test_type_completeness_challenge.sh                # 21 tests
./challenges/scripts/performance_baseline_challenge.sh                  # 16 tests
./challenges/scripts/stress_resilience_challenge.sh                     # 21 tests
./challenges/scripts/security_scan_results_challenge.sh                 # 17 tests
./challenges/scripts/grafana_dashboard_content_challenge.sh             # 7 tests
./challenges/scripts/lazy_loading_comprehensive_challenge.sh            # 16 tests
./challenges/scripts/brotli_compression_challenge.sh                    # 11 tests
# Go-native userflow challenges (22): run with --run-challenges=userflow
```

## LLMsVerifier

```bash
make verifier-init / verifier-build / verifier-test
make verifier-verify MODEL=gpt-4 PROVIDER=openai
```

## Protocol Endpoints

MCP `/v1/mcp` | ACP `/v1/acp` | LSP `/v1/lsp` | Embeddings `/v1/embeddings` | Vision `/v1/vision` | Cognee `/v1/cognee` (optional) | Startup `/v1/startup/verification` | BigData `/v1/bigdata/health` | Tasks `/v1/tasks` | Discovery `/v1/discovery` | Scoring `/v1/scoring` | Verification `/v1/verification` | Health `/v1/health` | Agentic `/v1/agentic/workflows` | Planning `/v1/planning/{hiplan,mcts,tot}` | LLMOps `/v1/llmops/{experiments,evaluate,prompts}` | Benchmark `/v1/benchmark/{run,results}` | QA `/v1/qa/{sessions,findings,platforms,discover}` | Ensemble `/v1/ensemble/{sessions,teams}` | Completion `/v1/completion/*` | GraphQL `/v1/graphql` (feature-flagged, `GRAPHQL_ENABLED=true`)

Fallback: routes to strongest LLM by score, falls back on failure.

## Technology Stack

Gin v1.12.0, PostgreSQL 15 (pgx/v5), Redis 7, testify v1.11.1, Prometheus/Grafana, OpenTelemetry. Supports Docker and Podman (`./scripts/container-runtime.sh`).

## Unified Service Management

`BootManager` (`internal/services/boot_manager.go`): groups services by compose file, starts via `docker compose up -d`, health checks all. `HealthChecker` (`internal/services/health_checker.go`): TCP/HTTP checks with retries. Required services (PostgreSQL, Redis, ChromaDB) fail boot on health failure. Remote services: health check only. SQL schemas: `sql/schema/`.

**Container Adapter**: `internal/adapters/containers/adapter.go` centralizes all container operations through the Containers module (`digital.vasic.containers`). Key variable: `globalContainerAdapter` in `cmd/helixagent/main.go`. The adapter auto-detects container runtime, sets up compose orchestrator, and optionally initializes remote distribution from `CONTAINERS_REMOTE_*` env vars. BootManager and infrastructure functions delegate to the adapter when available. Challenge: `./challenges/scripts/container_centralization_challenge.sh`.

**Constitution Management**: `ConstitutionWatcher` (`internal/services/constitution_watcher.go`) monitors project changes and auto-updates Constitution. Triggers: new modules extracted (go.mod detection), documentation changes (AGENTS.md/CLAUDE.md), project structure changes (new top-level directories), test coverage drops. Runs as background service with configurable check interval (default: 5 minutes). Auto-syncs updates to documentation files via `DocumentationSync`. Enable with `CONSTITUTION_WATCHER_ENABLED=true`.


<!-- BEGIN_CONSTITUTION -->
# Project Constitution

**Version:** 1.2.0 | **Updated:** 2026-02-21 15:45

Constitution with 26 rules (26 mandatory) across categories: Quality: 2, Safety: 1, Security: 1, Performance: 2, Containerization: 3, Configuration: 1, Testing: 4, Documentation: 2, Principles: 2, Stability: 1, Observability: 1, GitOps: 2, CI/CD: 1, Architecture: 1, Networking: 1, Resource Management: 1

## Mandatory Principles

**All development MUST adhere to these non-negotiable principles:**

### Architecture

**Comprehensive Decoupling** (Priority: 1)
- Identify all parts and functionalities that can be extracted as separate modules (libraries) and reused in various projects. Perform additional work to make each module fully decoupled and independent. Each module must be a separate project with its own CLAUDE.md, AGENTS.md, README.md, docs/, tests, and challenges.

### Testing

**100% Test Coverage** (Priority: 1)
- Every component MUST have 100% test coverage across ALL test types: unit, integration, E2E, security, stress, chaos, automation, and benchmark tests. No false positives. Use real data and live services (mocks only in unit tests).

**Comprehensive Challenges** (Priority: 1)
- Every component MUST have Challenge scripts validating real-life use cases. No false success - validate actual behavior, not return codes.

**Stress and Integration Tests** (Priority: 2)
- Introduce comprehensive stress and integration tests validating that the system is responsive and not possible to overload or break.

**Infrastructure Before Tests** (Priority: 1)
- ALL infrastructure containers (PostgreSQL, Redis, Mock LLM) MUST be running before executing tests or challenges. Use `make test-infra-start` or `make test-infra-direct-start` (Podman fallback with `--userns=host`). Tests and challenges that require infrastructure WILL FAIL without running containers.

### Documentation

**Complete Documentation** (Priority: 1)
- Every module and feature MUST have complete documentation: README.md, CLAUDE.md, AGENTS.md, user guides, step-by-step manuals, video courses, diagrams, SQL definitions, and website content. No component can remain undocumented.

**Documentation Synchronization** (Priority: 1)
- Anything added to Constitution MUST be present in AGENTS.md and CLAUDE.md, and vice versa. Keep all three synchronized.

### Quality

**No Broken Components** (Priority: 1)
- No module, application, library, or test can remain broken, disabled, or incomplete. Everything must be fully functional and operational.

**No Dead Code** (Priority: 1)
- Identify and remove all 'dead code' - features or functionalities left unconnected with the system. Perform comprehensive research and cleanup.

### Safety

**Memory Safety** (Priority: 1)
- Perform comprehensive research for memory leaks, deadlocks, and race conditions. Apply safety fixes and improvements to prevent these issues.

### Security

**Security Scanning** (Priority: 1)
- Execute Snyk and SonarQube scanning. Analyze findings in depth and resolve everything. Ensure scanning infrastructure is accessible via containerization (Docker/Podman).

### Performance

**Monitoring and Metrics** (Priority: 2)
- Create tests that run and perform monitoring and metrics collection. Use collected data for proper optimizations.

**Lazy Loading and Non-Blocking** (Priority: 2)
- Implement lazy loading and lazy initialization wherever possible. Introduce semaphore mechanisms and non-blocking mechanisms to ensure flawless responsiveness.

### Principles

**Software Principles** (Priority: 2)
- Apply all software principles: KISS, DRY, SOLID, YAGNI, etc. Ensure code is clean, maintainable, and follows best practices.

**Design Patterns** (Priority: 2)
- Use appropriate design patterns: Proxy, Facade, Factory, Abstract Factory, Observer, Mediator, Strategy, etc. Apply patterns where they add value.

### Stability

**Rock-Solid Changes** (Priority: 1)
- All changes must be safe, non-error-prone, and MUST NOT BREAK any existing working functionality. Ensure backward compatibility unless explicitly breaking.

### Containerization

**⚠️ CRITICAL WARNING - READ THIS FIRST ⚠️**

**ALL container orchestration is handled AUTOMATICALLY by the HelixAgent binary during boot. NEVER manipulate containers manually.**

**FORBIDDEN ACTIONS:**
- ❌ `docker start|stop|restart|rm` commands
- ❌ `podman start|stop|restart|rm` commands  
- ❌ `docker-compose up|down` commands
- ❌ `podman-compose up|down` commands
- ❌ `make test-infra-start` or similar Makefile targets
- ❌ Manual SSH to remote hosts for container deployment

**ONLY ACCEPTABLE WORKFLOW:**
1. `make build` - Build the HelixAgent binary
2. `./bin/helixagent` - Run it (ALL container orchestration happens automatically)
3. The binary reads `Containers/.env` and orchestrates everything

**Full Containerization** (Priority: 2)
- All services MUST run in containers (Docker/Podman/K8s). Support local default execution AND remote configuration. Services must auto-boot before HelixAgent is ready.

**Mandatory Container Orchestration Flow** (Priority: 1)
- This is the ONLY acceptable container orchestration flow. All tests and challenges MUST follow this pattern:
- **Step 1**: HelixAgent boots and initializes Containers module adapter
- **Step 2**: Adapter reads `Containers/.env` file (NOT project root `.env`)
- **Step 3**: Based on `Containers/.env` content: `CONTAINERS_REMOTE_ENABLED=true` → ALL containers to remote host(s) via `CONTAINERS_REMOTE_HOST_*` vars, NO local containers; `CONTAINERS_REMOTE_ENABLED=false` or missing → ALL containers locally
- **Step 4**: Health checks performed against configured endpoints (local or remote)
- **Step 5**: Required services failing health check cause boot failure in strict mode
- **Rules**: NO manual container starts, NO mixed mode, tests use `tests/precondition/containers_boot_test.go`, challenges verify container placement based on `Containers/.env`
- **Key Files**: `Containers/.env` (orchestration), `internal/config/config.go:isContainersRemoteEnabled()`, `internal/services/boot_manager.go`, `tests/precondition/containers_boot_test.go`

**Container-Based Builds** (Priority: 1)
- ALL release builds MUST be performed inside Docker/Podman containers for reproducibility. Use `make release` / `make release-all`. Version info injected via `-ldflags -X`. No release binaries should be built directly on the host unless container build is unavailable.

### Configuration

**Unified Configuration** (Priority: 1)
- **CLI agent configs MUST ONLY be generated using the HelixAgent binary** (`./bin/helixagent --generate-agent-config=<agent>` or `go run ./cmd/helixagent --generate-agent-config=<agent>`).
- **NEVER create, write, or modify CLI agent config files manually or via scripts.** The HelixAgent binary is the sole authority for config generation.
- Config generation uses LLMsVerifier's unified generator (`pkg/cliagents/`). No third-party scripts or manual edits.
- This ensures schema compliance, API key injection, MCP endpoint consistency, and validation for all 48 supported CLI agents.

**Non-Interactive Execution** (Priority: 1)
- **ALL commands MUST be fully non-interactive and automatable via command pipelines.**
- **NEVER prompt for passwords, passphrases, or any user input interactively.**
- SSH connections MUST use key-based authentication with SSH agent (`ssh-add`) or password provided via environment variables/sshpass.
- Container distribution to remote hosts MUST be fully automated through the Containers module's SSH executor with pre-configured credentials.
- All secrets (API keys, passwords, SSH keys) MUST be provided via environment variables or `.env` files, never via interactive prompts.

### Networking

**HTTP/3 (QUIC) with Brotli Compression** (Priority: 1)
- ALL HTTP communication MUST use HTTP/3 (QUIC) as primary transport with Brotli compression. HTTP/2 only as fallback. Compression priority: Brotli → gzip. All HTTP clients and servers MUST prefer HTTP/3. Use `quic-go/quic-go` for transport and `andybalholm/brotli` for compression.

### Resource Management

**Test and Challenge Resource Limits** (Priority: 1)
- ALL test and challenge execution MUST be strictly limited to 30-40% of host system resources. Use GOMAXPROCS=2, nice -n 19, ionice -c 3, and -p 1 for go test. Container limits required. Host machine runs mission-critical processes; exceeding limits has caused system crashes.

### Observability

**Health and Monitoring** (Priority: 2)
- Every service MUST expose health endpoints. Circuit breakers for all external dependencies. Prometheus/OpenTelemetry integration.

### GitOps

**GitSpec Compliance** (Priority: 2)
- Follow GitSpec constitution and all constraints from AGENTS.md and CLAUDE.md.

**SSH Only for Git Operations** (Priority: 1)
- **MANDATORY: NEVER use HTTPS for any Git service operations.** All cloning, fetching, pushing, and submodule operations MUST use SSH URLs (`git@github.com:org/repo.git`). HTTPS is STRICTLY FORBIDDEN even for public repositories. SSH keys are already configured on all Git services (GitHub, GitLab, etc.).

### CI/CD

**Manual CI/CD Only - NO Automated Pipelines or Git Hooks** (Priority: 1)
- **NO GitHub Actions, GitLab CI/CD, Jenkins, CircleCI, Travis CI, or any automated pipeline** may exist in this repository!
- **NO Git hooks (pre-commit, pre-push, post-commit, etc.)** may be installed or configured
- **NO `.github/workflows/` directory** allowed
- **NO `.gitlab-ci.yml` file** allowed  
- **NO Jenkinsfile, .travis.yml, .circleci, or any other CI configuration** allowed
- All builds, tests, and quality checks must be executed manually only via Makefile targets
- This rule is permanent and non-negotiable

---

*This Constitution is automatically synchronized with AGENTS.md, CLAUDE.md, and CONSTITUTION.json.*

<!-- END_CONSTITUTION -->


