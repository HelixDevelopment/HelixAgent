# HelixAgent Constitution

**Version:** 1.3.0
**Created:** 2026-02-10
**Updated:** 2026-04-16

Constitution with 33 rules (33 mandatory) across categories: Quality: 2, Safety: 1, Security: 1, Performance: 2, Containerization: 4, Configuration: 2, Testing: 8, Documentation: 2, Principles: 2, Stability: 1, Observability: 1, GitOps: 2, CI/CD: 1, Architecture: 1, Networking: 1, Resource Management: 1, Concurrency: 1

## Architecture

### Comprehensive Decoupling **[MANDATORY]** (Priority: 1)

**ID:** CONST-001

Identify all parts and functionalities that can be extracted as separate modules (libraries) and reused in various projects. Perform additional work to make each module fully decoupled and independent. Each module must be a separate project with its own CLAUDE.md, AGENTS.md, README.md, docs/, tests, and challenges.

## Testing

### No Mocks or Stubs in Production **[MANDATORY]** (Priority: 1)

**ID:** CONST-002a

**NO mocks, stubs, placeholder classes, or TODO implementations in production code.** Production code MUST use real implementations only. Mocks and stubs are permitted EXCLUSIVELY in unit test files (`*_test.go`). Integration tests, E2E tests, and all production code MUST use real services, real databases, real API calls, and real data. Placeholder classes, unimplemented interfaces, and stubbed methods are STRICTLY FORBIDDEN in production code.

### 100% Test Coverage **[MANDATORY]** (Priority: 1)

**ID:** CONST-002

Every component MUST have 100% test coverage across ALL test types: unit, integration, E2E, security, stress, chaos, automation, and benchmark tests. No false positives. Use real data and live services (mocks only in unit tests).

### Comprehensive Challenges **[MANDATORY]** (Priority: 1)

**ID:** CONST-003

Every component MUST have Challenge scripts validating real-life use cases. No false success - validate actual behavior, not return codes.

### Stress and Integration Tests **[MANDATORY]** (Priority: 2)

**ID:** CONST-014

Introduce comprehensive stress and integration tests validating that the system is responsive and not possible to overload or break.

### Infrastructure Before Tests **[MANDATORY]** (Priority: 1)

**ID:** CONST-022

ALL infrastructure containers (PostgreSQL, Redis, Mock LLM) MUST be running before executing tests or challenges. Use `make test-infra-start` or `make test-infra-direct-start` (Podman fallback with `--userns=host`). Tests and challenges that require infrastructure WILL FAIL without running containers.

## Documentation

### Complete Documentation **[MANDATORY]** (Priority: 1)

**ID:** CONST-004

Every module and feature MUST have complete documentation: README.md, CLAUDE.md, AGENTS.md, user guides, step-by-step manuals, video courses, diagrams, SQL definitions, and website content. No component can remain undocumented.

### Documentation Synchronization **[MANDATORY]** (Priority: 1)

**ID:** CONST-020

Anything added to Constitution MUST be present in AGENTS.md and CLAUDE.md, and vice versa. Keep all three synchronized.

## Quality

### No Broken Components **[MANDATORY]** (Priority: 1)

**ID:** CONST-005

No module, application, library, or test can remain broken, disabled, or incomplete. Everything must be fully functional and operational.

### No Dead Code **[MANDATORY]** (Priority: 1)

**ID:** CONST-006

Identify and remove all 'dead code' - features or functionalities left unconnected with the system. Perform comprehensive research and cleanup.

## Safety

### Memory Safety **[MANDATORY]** (Priority: 1)

**ID:** CONST-007

Perform comprehensive research for memory leaks, deadlocks, and race conditions. Apply safety fixes and improvements to prevent these issues.

## Security

### Security Scanning **[MANDATORY]** (Priority: 1)

**ID:** CONST-008

Execute Snyk and SonarQube scanning. Analyze findings in depth and resolve everything. Ensure scanning infrastructure is accessible via containerization (Docker/Podman).

## Performance

### Monitoring and Metrics **[MANDATORY]** (Priority: 2)

**ID:** CONST-009

Create tests that run and perform monitoring and metrics collection. Use collected data for proper optimizations.

### Lazy Loading and Non-Blocking **[MANDATORY]** (Priority: 2)

**ID:** CONST-010

Implement lazy loading and lazy initialization wherever possible. Introduce semaphore mechanisms and non-blocking mechanisms to ensure flawless responsiveness.

## Principles

### Software Principles **[MANDATORY]** (Priority: 2)

**ID:** CONST-011

Apply all software principles: KISS, DRY, SOLID, YAGNI, etc. Ensure code is clean, maintainable, and follows best practices.

### Design Patterns **[MANDATORY]** (Priority: 2)

**ID:** CONST-012

Use appropriate design patterns: Proxy, Facade, Factory, Abstract Factory, Observer, Mediator, Strategy, etc. Apply patterns where they add value.

## Stability

### Rock-Solid Changes **[MANDATORY]** (Priority: 1)

**ID:** CONST-013

All changes must be safe, non-error-prone, and MUST NOT BREAK any existing working functionality. Ensure backward compatibility unless explicitly breaking.

## Containerization

### Full Containerization **[MANDATORY]** (Priority: 2)

**ID:** CONST-015

All services MUST run in containers (Docker/Podman/K8s). Support local default execution AND remote configuration. Services must auto-boot before HelixAgent is ready.

### Mandatory Container Orchestration Flow **[MANDATORY]** (Priority: 1)

**ID:** CONST-015a

The ONLY acceptable container orchestration flow: (1) HelixAgent boots and initializes Containers module adapter, (2) Adapter reads Containers/.env file (NOT project root .env), (3) Based on CONTAINERS_REMOTE_ENABLED: true=ALL containers to remote hosts via CONTAINERS_REMOTE_HOST_* vars, false/missing=ALL containers locally, (4) Health checks against configured endpoints, (5) Required services failing health check cause boot failure. Rules: NO manual container starts, NO mixed mode, tests use tests/precondition/containers_boot_test.go, challenges verify container placement. Key files: Containers/.env, internal/config/config.go:isContainersRemoteEnabled(), internal/services/boot_manager.go, tests/precondition/containers_boot_test.go.

### Container-Based Builds **[MANDATORY]** (Priority: 1)

**ID:** CONST-021

ALL release builds MUST be performed inside Docker/Podman containers for reproducibility. Use `make release` / `make release-all`. Version info injected via `-ldflags -X`. No release binaries should be built directly on the host unless container build is unavailable.

### Mandatory Container Rebuild **[MANDATORY]** (Priority: 1)

**ID:** CONST-015b

All running containers on local host or remote distributed machines MUST be rebuilt and redeployed if code was changed affecting any of them. After code changes to services, handlers, MCPs, formatters, or any containerized component: rebuild affected images, restart containers, re-run distribution if using remote hosts.

## Configuration

### Unified Configuration **[MANDATORY]** (Priority: 1)

**ID:** CONST-016

**CLI agent configs MUST ONLY be generated using the HelixAgent binary** (`./bin/helixagent --generate-agent-config=<agent>` or `go run ./cmd/helixagent --generate-agent-config=<agent>`). **NEVER create, write, or modify CLI agent config files manually or via scripts.** The HelixAgent binary is the sole authority for config generation. Config generation uses LLMsVerifier's unified generator (`pkg/cliagents/`). No third-party scripts or manual edits. This ensures schema compliance, API key injection, MCP endpoint consistency, and validation for all 48 supported CLI agents.

### Non-Interactive Execution **[MANDATORY]** (Priority: 1)

**ID:** CONST-016a

ALL commands MUST be fully non-interactive and automatable via command pipelines. NEVER prompt for passwords, passphrases, or any user input interactively. SSH connections MUST use key-based authentication. All secrets MUST be provided via environment variables or .env files, never via interactive prompts.

## Observability

### Health and Monitoring **[MANDATORY]** (Priority: 2)

**ID:** CONST-017

Every service MUST expose health endpoints. Circuit breakers for all external dependencies. Prometheus/OpenTelemetry integration.

## GitOps

### GitSpec Compliance **[MANDATORY]** (Priority: 2)

**ID:** CONST-018

Follow GitSpec constitution and all constraints from AGENTS.md and CLAUDE.md.

### SSH Only for Git Operations **[MANDATORY]** (Priority: 1)

**ID:** CONST-018a

MANDATORY: NEVER use HTTPS for any Git service operations. All cloning, fetching, pushing, and submodule operations MUST use SSH URLs (git@github.com:org/repo.git). HTTPS is STRICTLY FORBIDDEN even for public repositories. SSH keys are already configured on all Git services (GitHub, GitLab, etc.).

## CI/CD

### Manual CI/CD Only **[MANDATORY]** (Priority: 1)

**ID:** CONST-019

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline** may exist in this repository! **NO Git hooks (pre-commit, pre-push, post-commit, etc.)** may be installed or configured. All builds, tests, and quality checks must be executed manually only via Makefile targets. This rule is permanent and non-negotiable.

## Networking

### HTTP/3 (QUIC) with Brotli Compression **[MANDATORY]** (Priority: 1)

**ID:** CONST-023

ALL HTTP communication MUST use HTTP/3 (QUIC) as primary transport with Brotli compression. HTTP/2 only as fallback when HTTP/3 is unavailable. Compression priority: Brotli (primary) then gzip (fallback). All HTTP clients and servers MUST prefer HTTP/3. Use `quic-go/quic-go` for transport and `andybalholm/brotli` for compression.

## Resource Management

### Test and Challenge Resource Limits **[MANDATORY]** (Priority: 1)

**ID:** CONST-024

ALL test and challenge execution MUST be strictly limited to 30-40% of host system resources. Use GOMAXPROCS=2, nice -n 19, ionice -c 3, and -p 1 for go test. Container limits required. Host machine runs mission-critical processes; exceeding limits has caused system crashes and forced resets.

## Testing Constraints

### No Mocks Outside Unit Tests **[MANDATORY]** (Priority: 1)

**ID:** CONST-025

ONLY unit tests (`*_test.go` with `-short` flag or tests that do NOT call live services) may use mocks, stubs, fakes, or placeholder implementations. Integration tests, functional tests, E2E tests, Challenge tests, and HelixQA tests MUST ALL execute against the REAL running HelixAgent system with real containers, real databases, real Redis, and real HTTP calls. NO test that is not a unit test may use `integrationMockProvider`, `debateMockLLMProvider`, or any other mock. All services and containers MUST be booted and operational before non-unit tests run. Tests that cannot connect to real services MUST skip (not fail).

### Both Debate Flavors Must Be Tested **[MANDATORY]** (Priority: 1)

**ID:** CONST-026

HelixAgent has TWO distinct debate implementations:
1. **DebateService** (`internal/services/debate_service.go`) — Core debate with `ConductDebate()`, provider registry, suspiciously-fast-response detection, multi-round orchestration
2. **Orchestrator Framework** (`internal/services/debate_integration/`) — Advanced orchestrator with agent pools, 8-phase protocol, topology support

BOTH flavors MUST have comprehensive integration tests against the LIVE HelixAgent API (`/v1/debates`). Tests MUST cover:
- **5-position debates** (minimum viable multi-agent debate)
- **8+ position debates** (large-scale multi-agent debate)
- Error handling, timeout, fallback, and concurrent execution
- Voting methods, consensus detection, quality scoring

The `IsSuspiciouslyFastResponse` check (100ms / 100 chars threshold) is a production safeguard. Mock providers in unit tests must respect this threshold (latency >= 100ms or content >= 100 chars).

### Port and Service Architecture **[MANDATORY]** (Priority: 1)

**ID:** CONST-027

The following service architecture is MANDATORY. Port assignments are centralized in the canonical registry at `internal/ports/ports.go`; see `docs/development/port-registry.md` for the full table. Default ports live in the **81xx band** under prefix `8` and shift to the **91xx band** under prefix `9` (env var `HELIXAGENT_PORT_PREFIX`). No ad-hoc port allocation — every service that binds a port must have an entry in the registry.

**Eager Services** (started at boot, always running):
- HelixAgent: port **8100** (`HELIXAGENT_PORT_HTTP`)
- PostgreSQL: port **8101** (`HELIXAGENT_PORT_POSTGRES`)
- Redis (primary): port **8102** (`HELIXAGENT_PORT_REDIS`), **NO password** (container: helixagent-redis)
- MCP Bridge: port **8103** (`HELIXAGENT_PORT_MCP_BRIDGE`)
- HelixLLM: port **8105** (`HELIXAGENT_PORT_HELIXLLM`, HTTPS/TLS 1.3)
- Redis MCP backend: port **8110** (`HELIXAGENT_PORT_REDIS_MCP`, password `helixagent123`)
- MCP Servers: ports **8200-8281** (82xx band, 12 tiers; internal port 9000 bridged via socat)

**Lazy Services** (started on-demand):
- Cognee: port **8120** (`HELIXAGENT_PORT_COGNEE`)
- ChromaDB: port **8121** (`HELIXAGENT_PORT_CHROMADB`)
- Qdrant: port **8122** (`HELIXAGENT_PORT_QDRANT`)
- Neo4j: ports **8123** HTTP / **8124** Bolt (`HELIXAGENT_PORT_NEO4J_HTTP`, `HELIXAGENT_PORT_NEO4J_BOLT`)

**Observability** (83xx band): Prometheus **8310**, Grafana **8311**, Jaeger **8312**, ACP Manager **8300**.

**Redis Architecture**:
- `helixagent-redis` on port **8102**: **NO password** — used by HelixAgent core, streaming, and functional tests
- `helixagent-mcp-redis-backend` on port **8110**: password `helixagent123` — used by MCP containers

**Invariants enforced by `internal/ports/ports_test.go` and `challenges/scripts/port_registry_challenge.sh`:** no offset collisions, every port fits in 16 bits at prefixes 8 and 9, band discipline preserved (core ≤199, MCP 200-281, obs 300-312), every env-var name starts with `HELIXAGENT_PORT_`.

**API Response Format Contracts** (server returns these exact formats, tests must match):
- `/v1/embeddings/providers` → `{"providers":[{"name":"...","model":"...","dimension":N,"enabled":bool}]}` (objects, NOT strings)
- `/v1/vision/capabilities` → `{"capabilities":[{"id":"...","name":"...","status":"..."}]}` (objects with status field)
- `/v1/acp/agents` → `{"agents":[{"id":"...","name":"...","status":"..."}]}` (objects with status field)
- `/v1/acp/agents/{id}` → `{"id":"...","name":"...","status":"...","capabilities":[...]}` (uses `id` NOT `agent_id`)
- `/v1/acp/execute` → `{"agent_id":"...","status":"...","result":{...}}` (uses `agent_id`)
- Health endpoints: `/v1/vision/health` ✓, `/v1/acp/health` ✓, `/v1/embeddings/health` ✗ (404 — use `/v1/embeddings/providers` as health check)

### Bugfix Documentation **[MANDATORY]** (Priority: 1)

**ID:** CONST-028

All bug fixes MUST be documented in `docs/issues/fixed/BUGFIXES.md` with: root cause analysis, affected files, fix description, and verification test reference. Fixes without documentation are incomplete.

## Concurrency

### Concurrent-Safe Containers **[MANDATORY]** (Priority: 1)

**ID:** CONST-029

Any struct field that is a mutable collection (map, slice, channel-map) and is accessed concurrently MUST use `safe.Store[K,V]` or `safe.Slice[T]` from `digital.vasic.concurrency/pkg/safe`. Bare `sync.Mutex + map` / `sync.Mutex + slice` combinations in shared state are prohibited for new code.

**Rationale:** The bare-mutex pattern is a review-caught bug class; the primitives make forgetting the lock structurally impossible (there is no lock to forget). We have shipped 18+ fixes against Pattern-A races (BUGFIXES #29, #30, #34–#38); each fix was correct but the pattern that demanded fixing was wrong.

**Primitives:** `digital.vasic.concurrency/pkg/safe/{store,slice}.go` — generic, 10× race-clean, internal collection never exposed (no `Raw()`, `Map()`, `Slice()`, `Internal()` methods).

**Atomic operations:** Use `Store.Update` and `Slice.UpdateAt` callbacks for read-modify-write. Do not compose Get+Put in userland.

**Iteration:** Never mutate inside `Range` (deadlock). Use `Snapshot` + iterate the copy + apply mutations.

**Discipline and migration table:** `docs/development/concurrency-playbook.md`.

**Enforcement:** `scripts/concurrency-audit.sh` runs under `make ci-validate-all`. New code failing the audit fails CI. Existing sites migrate per the playbook's priority order; allowlist is temporary.
