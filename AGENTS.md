# AGENTS.md

> This file is the authoritative guide for AI coding agents working on HelixAgent.
> It assumes the reader knows nothing about the project.
> Deeper `AGENTS.md` or `CLAUDE.md` files in subdirectories take precedence over
> this root file for files within those subtrees.

---

## Project Overview

HelixAgent is a production-ready, AI-powered ensemble LLM service written in Go
(1.26) that aggregates responses from multiple language models to provide the
most accurate and reliable outputs. It exposes an OpenAI-compatible REST API and
a gRPC facade. The system supports 51 LLM providers, multi-round AI debate
orchestration, MCP (Model Context Protocol) adapters, ACP (Agent Coordination
Protocol), LSP (Language Server Protocol), embeddings, vision, and
containerized infrastructure.

**Module**: `dev.helix.agent`

**Main Binary**: `helixagent` (built from `cmd/helixagent/`). Runs on port **8100** (`HELIXAGENT_PORT_HTTP`).

**Additional Applications**:
- `api` — Standalone API server (port 8080, demo/development only, NOT production)
- `grpc-server` — gRPC service endpoint implementing `LLMFacade` and `LLMProvider`
- `mcp-bridge` — MCP SSE bridge (port 8103, `HELIXAGENT_PORT_MCP_BRIDGE`) wrapping stdio MCP servers
- `cognee-mock` — Mock Cognee service for testing
- `sanity-check` — System validation tool
- `generate-constitution` — Constitution file generator
- `audit` — Audit utility

**Monorepo Structure**: The project is a Go monorepo with ~60 submodules.
The root `go.mod` uses extensive `replace` directives to wire local submodules together.
Key submodules:
- `Containers` - Container orchestration and remote distribution
- `Database` - PostgreSQL connectivity and migrations
- `Auth` - Authentication and JWT handling
- `Cache` - Redis-based caching layer
- `Concurrency` - Safe concurrent data structures (`safe.Store`, `safe.Slice`)
- `EventBus` - Event-driven architecture
- `MCP_Module` - Model Context Protocol implementation
- `DebateOrchestrator` - Multi-agent debate system (two implementations)
- `HelixMemory` - Memory and knowledge graph
- `HelixLLM` - LLM abstraction layer (51 providers)
- `HelixQA` - Quality assurance and testing framework

---

## Technology Stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.26 |
| HTTP Framework | Gin |
| gRPC | Protocol Buffers + `google.golang.org/grpc` |
| Database | PostgreSQL (via `pgxpool` / `digital.vasic.database`) |
| Cache | Redis (primary: port 8102, no password) |
| Observability | Prometheus metrics, Grafana dashboards, structured logging via `sirupsen/logrus` |
| Container Runtime | Docker or Podman (auto-detected) |
| Orchestration | Kubernetes manifests in `k8s/` (base, staging, production overlays) |
| Messaging | Kafka, RabbitMQ (optional, started on-demand) |
| Memory / Vector | Cognee, ChromaDB, Neo4j, Qdrant, Mem0, HelixMemory |
| Object Storage | MinIO (optional) |
| Security | Argon2id password hashing, TLS 1.3, JWT (`golang-jwt/jwt/v5`) |

---

## Code Organization

```
cmd/                    # Application entry points (8 binaries)
  helixagent/           # Main production server
  api/                  # Standalone demo API
  grpc-server/          # gRPC facade
  mcp-bridge/           # MCP SSE bridge
  cognee-mock/          # Mock Cognee
  sanity-check/         # System validation
  generate-constitution/# Constitution generator
  audit/                # Audit utility

internal/               # Core application code (~50+ packages)
  router/               # Central Gin router setup, middleware, service initialization
  handlers/             # HTTP handlers (~40 files: OpenAI-compatible, debate, MCP, LSP, ACP, embeddings, etc.)
  services/             # Business logic: provider registry, ensemble, debate, caching, monitoring, OAuth
  llm/                  # LLM abstraction layer and 51 provider implementations
  models/               # Core domain types (LLMRequest, LLMResponse, Message, etc.)
  database/             # PostgreSQL connectivity, migrations (embedded SQL strings)
  config/               # Centralized env-var based configuration
  middleware/           # Auth, compression, concurrency limiting, body size limits
  mcp/                  # MCP server registry, connection pooling, preinstaller, SSE bridge
  adapters/             # Adapters to external submodules (containers, database, auth, memory, messaging)
  features/             # Feature flags (GraphQL, Brotli, HTTP/3, etc.)
  cache/                # Caching layer
  security/             # Guardrails, red-team fixtures, normalization
  ports/                # Centralized port registry (see CONST-027)

pkg/                    # Public API packages
  api/                  # Generated protobuf code (llm-facade.pb.go, llm-facade_grpc.pb.go)

tests/                  # Test suites organized by type
  unit/                 # Unit tests by domain (mocks allowed, run with `-short`)
  integration/          # Cross-service integration tests (REAL infrastructure required)
  e2e/                  # End-to-end workflows (REAL infrastructure required)
  security/             # Vulnerability scans (REAL infrastructure required)
  stress/               # Load and saturation tests (REAL infrastructure required)
  chaos/                # Fault injection (REAL infrastructure required)
  challenge/            # Chaos/competition tests (REAL infrastructure required)
  performance/          # Performance benchmarks (`//go:build performance`)
  benchmark/            # Benchmark suites
  fixtures/             # Shared test data
  testutils/            # Shared helpers
  mock-llm-server/      # Deterministic mock LLM (Dockerized, OpenAI-compatible)

specs/                  # gRPC / protobuf contracts
k8s/                    # Kubernetes manifests (base, staging, production)
monitoring/             # Prometheus and Grafana configs
docs/                   # Documentation: api/, deployment/, development/, security/, architecture/
scripts/                # ~90+ build, test, deploy, security, and utility scripts
```

### Key Architectural Patterns

1. **Request Flow**: HTTP Request → `internal/handlers` → `internal/services` →
   `ProviderRegistry` / `EnsembleService` / `DebateService` → `internal/llm`
   providers → aggregated `models.LLMResponse`.

2. **Provider Registry**: Lazy initialization with `sync.Once` per provider.
   Auto-discovers providers from environment variables (e.g., `CLAUDE_API_KEY`).
   Wraps each provider with circuit breakers and concurrency semaphores.
   Performs startup verification using `LLMsVerifier` to form a "Debate Team"
   of the best 15 LLMs.

3. **Debate Architecture (Two Implementations)**:
   - **DebateService** (`internal/services/debate_service.go`) — Core debate with
     `ConductDebate()`, provider registry, suspiciously-fast-response detection,
     multi-round orchestration
   - **Orchestrator Framework** (`internal/services/debate_integration/`) —
     Advanced orchestrator with agent pools, 8-phase protocol, topology support
     (mesh/star/chain/tree)

4. **Database Migrations**: Embedded as SQL strings in `internal/database/db.go`.
   Migration files live in `internal/database/migrations/`.

5. **Port and Service Architecture**:
   - **HelixAgent runs on port 8100 by default** (from the canonical port
     registry at `internal/ports/ports.go`).
   - Configurable prefix: all default ports share a leading digit controlled by
     `HELIXAGENT_PORT_PREFIX` (default 8, can flip to 9).
   - Eager services (started at boot): HelixAgent HTTP (8100), PostgreSQL (8101),
     Redis (8102), MCP Bridge (8103), HelixLLM (8105), Redis MCP backend (8110)
   - Lazy services (started on-demand): Cognee (8120), ChromaDB (8121), Qdrant (8122),
     Neo4j (8123/8124)
   - Observability: Prometheus (8310), Grafana (8311), Jaeger (8312), ACP Manager (8300)
   - MCP Servers: 8200-8281 (12 tiers; see `docs/development/port-registry.md`)

---

## Build and Test Commands

All builds and tests are run manually or via Makefile targets. No CI/CD pipelines.

### Build

```bash
make build              # Build helixagent binary
make build-debug        # Build with debug symbols
make build-all          # Build all 7 applications
make release            # Full release build with version injection
```

### Test

```bash
make test               # Auto-detects Postgres/Redis; full suite or falls back to -short
make test-unit          # go test ./internal/... -short (unit tests only)
make test-integration   # Integration tests via scripts/run-integration-tests.sh
make test-e2e           # End-to-end tests (REAL infra)
make test-bench         # go test -bench=. -benchmem ./...
make test-race          # Race detector run
make test-complete      # All 6 test types with full Docker/Podman infra
make ci-validate-all    # Full CI validation including concurrency audit
make ensure-test-infra  # Auto-detect Docker/Podman, start Postgres + Redis
```

### Resource Limits
All test runs use `nice -n 19 ionice -c 3`, `GO_TEST_FLAGS := -p 1`, `GOMAXPROCS := 2`.

### Running a Single Test Package
```bash
go test -v -short ./internal/handlers/...
go test -v ./tests/integration/...
```

---

## Code Style

### Concurrency (CONST-029)
Any mutable collection accessed concurrently **MUST** use `safe.Store[K,V]` or `safe.Slice[T]`
from `digital.vasic.concurrency/pkg/safe`. Bare `sync.Mutex + map`/`slice` is **prohibited for new code**.
Enforced by `scripts/concurrency-audit.sh` under `make ci-validate-all`.

### Linting
`.golangci.yml` enables `errcheck`, `govet`, `staticcheck`, `unused`, `gosimple`, `ineffassign`, `typecheck`.
Skipped directories: `cli_agents`, `MCP`, `MCP-Servers`, `vendor`.

### Commit Style
Conventional Commits: `type(scope): description`.
Examples: `feat(debate): add mesh topology`, `fix(handlers): correct embedding response format`.

---

## Testing

### Test Categories

| Category | Location | Mocks Allowed | Infrastructure |
|----------|----------|---------------|----------------|
| Unit | `*_test.go` (run with `-short`) | Yes | None |
| Integration | `tests/integration/...` | **NO** | Postgres :8101, Redis :8102, HelixAgent :8100 |
| E2E / Security / Stress / Chaos | `tests/{e2e,security,stress,chaos}/...` | **NO** | Full test stack |
| Performance | `tests/performance/...` (`//go:build performance`) | **NO** | Full stack |
| Benchmarks | `*_benchmark_test.go` | **NO** | Varies |

### Skipping Strategy
Tests that require infrastructure MUST probe via TCP/HTTP and call `t.Skip()` when unavailable.
They MUST NOT skip when `CI=true` or `FULL_TEST_MODE=true`.

Key helpers in `internal/testutil/`:
- `RequireServer(t)` / `RequirePostgres(t)` / `RequireRedis(t)` / `RequireMockLLM(t)`
- `RequireInfra(t)` / `RequireEnv(t, envVar)` / `RequireAPIKey(t, provider)`
- `ShortTimeout(t)` / `MediumTimeout(t)` / `LongTimeout(t)`

### Mock LLM Server
`tests/mock-llm-server/main.go` — Dockerized, deterministic, OpenAI-compatible.
Provider configs in tests can point to it via `CLAUDE_BASE_URL=http://localhost:18081/v1`.

### Test Data
Shared fixtures in `tests/fixtures/fixtures.go`: `MockProviders()`, `MockLLMRequests()`, `MockLLMResponses()`, `MockModelParameters(requestType)`, `MockUserSessions()`.

---

## Security / Operations

### Container Runtime
The project supports both Docker and Podman. Auto-detection is performed by
`scripts/container-runtime.sh`. Every Docker target has a Podman equivalent.

### Local Development
```bash
make ensure-test-infra   # Starts Postgres + Redis
make build
./bin/helixagent         # or make run
```

### Docker Compose Stacks
- `docker-compose.yml` — Main production-like stack
- `docker-compose.test.yml` — Test stack
- `docker-compose.integration.yml` — Lightweight integration test infra
- `docker-compose.memory.yml` — HelixMemory stack
- `docker-compose.production.yml` — Production messaging overlay
- `docker-compose.security.yml` — Security scanning stack
- `docker-compose.helixllm.yml` — HelixLLM infrastructure
- `docker/mcp/docker-compose.mcp-full.yml` — 65+ MCP servers (ports 8200-8281, 82xx band; see `docs/development/port-registry.md`)

### Kubernetes
Manifests in `k8s/`:
- `k8s/base/` — Namespace, Deployment (2 replicas), Service, ConfigMap, HPA, PDB, NetworkPolicy, ServiceAccount
- `k8s/staging/` — Staging overlays
- `k8s/production/` — Production overlays (ingress, patches)

### Remote Container Distribution
Remote hosts are registered dynamically via `CONTAINERS_REMOTE_HOST_N_*` env vars in `Containers/.env`.
Enable with `CONTAINERS_REMOTE_ENABLED=true`. Audit configured hosts:
```bash
grep '^CONTAINERS_REMOTE_HOST_' Containers/.env
```

---

## Mandatory Constraints

These constraints are **permanent and non-negotiable**.

### CONST-025: No Mocks Outside Unit Tests
ONLY unit tests may use mocks, stubs, fakes, or placeholder implementations.
Integration, E2E, security, stress, chaos, challenge, benchmark, HelixQA tests MUST execute against the REAL running HelixAgent system with REAL containers, databases, Redis, and HTTP calls.

### CONST-026: Both Debate Flavors Must Be Tested Comprehensively
HelixAgent has TWO distinct debate implementations that BOTH require integration tests against `/v1/debates`:
- **DebateService** (`internal/services/debate_service.go`) — Core debate with `ConductDebate()`, multi-round orchestration
- **Orchestrator Framework** (`internal/services/debate_integration/`) — Advanced orchestrator with agent pools, 8-phase protocol, topology support (mesh/star/chain/tree)

Tests MUST cover 5+ and 8+ position debates, error handling, timeout, fallback, concurrent execution, voting, consensus detection, and quality scoring.

### CONST-027: Port and Service Architecture
**HelixAgent runs on port 8100 by default** (from `internal/ports/ports.go`).

**Configurable prefix:** `HELIXAGENT_PORT_PREFIX` (default 8, can flip to 9).
At prefix=8 → 81xx-83xx; at prefix=9 → 91xx-93xx.

**Eager Services** (started at boot, band 81xx):
- HelixAgent HTTP: 8100 (`HELIXAGENT_PORT_HTTP`)
- PostgreSQL: 8101 (`HELIXAGENT_PORT_POSTGRES`)
- Redis: 8102 (`HELIXAGENT_PORT_REDIS`, no password)
- MCP Bridge: 8103 (`HELIXAGENT_PORT_MCP_BRIDGE`)
- HelixLLM: 8105 (`HELIXAGENT_PORT_HELIXLLM`, HTTPS/TLS 1.3)
- Redis MCP backend: 8110 (`HELIXAGENT_PORT_REDIS_MCP`, password `helixagent123`)

**Lazy Services** (started on-demand, band 81xx):
- Cognee: 8120 (`HELIXAGENT_PORT_COGNEE`)
- ChromaDB: 8121 (`HELIXAGENT_PORT_CHROMADB`)
- Qdrant: 8122 (`HELIXAGENT_PORT_QDRANT`)
- Neo4j HTTP/Bolt: 8123/8124 (`HELIXAGENT_PORT_NEO4J_HTTP`, `HELIXAGENT_PORT_NEO4J_BOLT`)

**Observability** (band 83xx): Prometheus 8310, Grafana 8311, Jaeger 8312, ACP Manager 8300

**Redis Architecture**:
- `helixagent-redis` port 8102: NO password — HelixAgent core, streaming, tests
- `helixagent-mcp-redis-backend` port 8110: password `helixagent123` — MCP containers

**API Response Format Contracts** (server returns these, tests MUST match):
- `/v1/embeddings/providers` → objects `{name,model,dimension,enabled}` (NOT strings)
- `/v1/vision/capabilities` → objects `{id,name,status}` with status field
- `/v1/acp/agents` → objects `{id,name,status}` with status field
- `/v1/acp/agents/{id}` → uses field `id` (NOT `agent_id`)
- `/v1/acp/execute` → uses field `agent_id`
- Health: `/v1/vision/health` works, `/v1/acp/health` works, `/v1/embeddings/health` returns 404 — use `/v1/embeddings/providers`

### CONST-028: Bugfix Documentation
All bug fixes MUST be documented in `docs/issues/fixed/BUGFIXES.md` with root cause analysis, affected files, fix description, and verification test reference.

### CONST-029: Concurrent-Safe Containers
Any mutable collection accessed concurrently **MUST** use `safe.Store[K,V]` or `safe.Slice[T]` from `digital.vasic.concurrency/pkg/safe`. Bare `sync.Mutex + map`/`slice` is **prohibited for new code**. Enforced by `scripts/concurrency-audit.sh`.

### CONST-030: Real Infrastructure for All Non-Unit Tests
Mocks, stubs, fakes, placeholders, and hardcoded data MAY ONLY be used in unit tests (files ending `_test.go` run under `go test -short`).
EVERY other test type MUST execute against the REAL running HelixAgent system with REAL containers, databases, Redis, MCP/ACP/LSP services, and REAL HTTP calls.

### CONST-031: Authorized Remote Distribution Hosts
Remote distribution hosts are registered dynamically via `CONTAINERS_REMOTE_HOST_N_*` env vars in `Containers/.env`.
No host name is hardcoded in source, tests, challenges, or governance docs.

**Per-host env var keys (each N):**

- `CONTAINERS_REMOTE_HOST_N_NAME` (required)
- `CONTAINERS_REMOTE_HOST_N_ADDRESS`
- `CONTAINERS_REMOTE_HOST_N_PORT`
- `CONTAINERS_REMOTE_HOST_N_USER`
- `CONTAINERS_REMOTE_HOST_N_KEY` (or use ssh-agent)
- `CONTAINERS_REMOTE_HOST_N_PASSWORD` (optional; key-based auth preferred)
- `CONTAINERS_REMOTE_HOST_N_RUNTIME` (`docker`/`podman`/`k8s`)
- `CONTAINERS_REMOTE_HOST_N_LABELS` (comma-separated `key=value`)
- `CONTAINERS_REMOTE_HOST_N_GPU_AUTOPROBE` (optional)

Audit the currently configured set:

```bash
grep '^CONTAINERS_REMOTE_HOST_' Containers/.env
```

Enable with `CONTAINERS_REMOTE_ENABLED=true`. Every non-unit test run and every
production deployment MUST use whichever hosts are currently configured when
remote distribution is enabled.

Direct `docker`/`podman` commands, manual container start/stop, and ad-hoc
remote hosts outside the `.env` mechanism are strictly prohibited per the
Mandatory Container Orchestration Flow.

**Snapshot (2026-04-21)**: configured hosts are `thinker.local` and `amber.local`.
Snapshot reflects `.env` state at that date; N scales freely to any number of
hosts ≥ 1.

### CONST-032: Reproduction-Before-Fix

**Mandatory.** Every reported error, defect, or unexpected behavior MUST be reproduced by a Challenge script BEFORE any fix is attempted.

**Sequence:**
1. Write `challenges/scripts/<bug>_challenge.sh` that exercises the exact failing scenario against the running binary. The challenge MUST exit non-zero when the bug is present.
2. Run the Challenge to confirm reproduction. Paste the failing output into the bug ticket.
3. Then write the fix. No code change is permitted before steps 1 and 2 are complete.
4. Re-run the Challenge to confirm the fix. The challenge becomes the regression guard forever.
5. Commit Challenge + fix together.

<!-- BEGIN_CONSTITUTION -->
# Project Constitution

**Version:** 1.2.0 | **Updated:** 2026-02-21 15:45

This constitution section is synchronized from CONSTITUTION.md. For the full text of all 26 mandatory rules, see CONSTITUTION.md and CONSTITUTION.json.
<!-- END_CONSTITUTION -->

---

## Universal Mandatory Constraints

These rules are non-negotiable across HelixAgent and EVERY submodule
or sibling project. They are derived from the root `CLAUDE.md` and have
been cascaded into every project-owned repo's `CLAUDE.md`, `AGENTS.md`,
and `CONSTITUTION.md`. Project-specific addenda are welcome but cannot
weaken or override these.

### Hard Stops (permanent, non-negotiable)

1. **NO CI/CD pipelines.** No `.github/workflows/`, `.gitlab-ci.yml`,
   `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated pipeline.
   No Git hooks either.
2. **NO HTTPS for Git.** SSH URLs only (`git@github.com:…`,
   `git@gitlab.com:…`, etc.) for clones, fetches, pushes, and submodule
   updates — including for public repos.
3. **NO manual container commands.** The HelixAgent binary owns
   container orchestration; `make build` → `./bin/helixagent` is the
   only acceptable workflow.

### Mandatory Development Standards

1. **100% Test Coverage** — unit, integration, E2E, security/penetration,
   benchmark. Mocks/stubs ONLY in unit tests.
2. **Challenge Coverage** — every component MUST have Challenge scripts
   under `./challenges/scripts/` validating real-life use cases.
3. **Real Data** — non-unit tests use actual API calls, real databases,
   live services. No simulated success.
4. **Health & Observability** — every service exposes health endpoints;
   circuit breakers for all external dependencies.
5. **Documentation & Quality** — Conventional Commits, follow patterns,
   keep CLAUDE.md / AGENTS.md current.
6. **Validation Before Release** — pass `make ci-validate-all` plus all
   challenges.
7. **No Mocks or Stubs in Production.**
8. **Comprehensive Verification** — runtime testing (actual HTTP / real
   CLI invocations); grep-only validation is NEVER sufficient.
9. **Resource Limits for Tests & Challenges (CRITICAL)** — strictly
   30-40% of host resources. Use `GOMAXPROCS=2`, `nice -n 19`,
   `ionice -c 3`, `-p 1` for `go test`.
10. **Bugfix Documentation** — every fix in `docs/issues/fixed/BUGFIXES.md`
    with root cause, affected files, fix description, verification test.
11. **Real Infrastructure for All Non-Unit Tests (CONST-030)** — any
    test type beyond `go test -short` MUST execute against the REAL
    running system with REAL containers, REAL databases, REAL services,
    REAL HTTP calls. Tests that cannot connect to real services MUST
    skip (not fail).
12. **Reproduction-Before-Fix (CONST-032 — MANDATORY)** — every reported
    bug MUST be reproduced by a Challenge script BEFORE any fix is
    attempted. Sequence: write Challenge → run, confirm fail → write
    fix → re-run, confirm pass → commit Challenge + fix together. The
    Challenge becomes the regression guard for that bug forever.
13. **Concurrent-Safe Containers (CONST-029)** — mutable shared
    collections MUST use `safe.Store[K,V]` / `safe.Slice[T]` from
    `digital.vasic.concurrency/pkg/safe`. Bare `sync.Mutex + map/slice`
    is prohibited for new code.

### Definition of Done (universal)

A change is NOT done because code compiles and tests pass. "Done"
requires pasted terminal output from a real run, produced in the same
session as the change.

- **No self-certification.** Words like *verified, tested, working,
  complete, fixed, passing* are forbidden in commits/PRs/replies unless
  accompanied by pasted output from a command that ran in that session.
- **Demo before code.** Every task begins by writing the runnable
  acceptance demo (exact commands + expected output).
- **Real system, every time.** Demos run against real artifacts.
- **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `describe.skip`
  without a trailing `SKIP-OK: #<ticket>` comment break validation.
- **Evidence in the PR.** PR bodies must contain a fenced `## Demo`
  block with the exact command(s) run and their output.

<!-- BEGIN host-power-management addendum (CONST-033) -->

## Host Power Management — Hard Ban (CONST-033)

**You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition.** This rule applies to:

- Every shell command you run via the Bash tool.
- Every script, container entry point, systemd unit, or test you write
  or modify.
- Every CLI suggestion, snippet, or example you emit.

**Forbidden invocations** (non-exhaustive — see CONST-033 in
`CONSTITUTION.md` for the full list):

- `systemctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot|kexec`
- `loginctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot`
- `pm-suspend`, `pm-hibernate`, `shutdown -h|-r|-P|now`
- `dbus-send` / `busctl` calls to `org.freedesktop.login1.Manager.Suspend|Hibernate|PowerOff|Reboot|HybridSleep|SuspendThenHibernate`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to anything but `'nothing'` or `'blank'`

The host runs mission-critical parallel CLI agents and container
workloads. Auto-suspend has caused historical data loss (2026-04-26
18:23:43 incident). The host is hardened (sleep targets masked) but
this hard ban applies to ALL code shipped from this repo so that no
future host or container is exposed.

**Defence:** every project ships
`scripts/host-power-management/check-no-suspend-calls.sh` (static
scanner) and
`challenges/scripts/no_suspend_calls_challenge.sh` (challenge wrapper).
Both MUST be wired into the project's CI / `run_all_challenges.sh`.

**Full background:** `docs/HOST_POWER_MANAGEMENT.md` and `CONSTITUTION.md` (CONST-033).

<!-- END host-power-management addendum (CONST-033) -->

