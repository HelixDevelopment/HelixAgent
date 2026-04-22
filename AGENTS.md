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
a gRPC facade. The system supports 47+ LLM providers, multi-round AI debate
orchestration, MCP (Model Context Protocol) adapters, ACP (Agent Coordination
Protocol), LSP (Language Server Protocol), embeddings, vision, and
containerized infrastructure.

**Module**: `dev.helix.agent`

**Main Binary**: `helixagent` (built from `cmd/helixagent/`). Runs on port **7061**.

**Additional Applications**:
- `api` — Standalone API server (port 8080, demo/development only, NOT production)
- `grpc-server` — gRPC service endpoint implementing `LLMFacade` and `LLMProvider`
- `mcp-bridge` — MCP SSE bridge (port 9000) wrapping stdio MCP servers
- `cognee-mock` — Mock Cognee service for testing
- `sanity-check` — System validation tool
- `generate-constitution` — Constitution file generator
- `audit` — Audit utility

**Monorepo Structure**: The project is a Go monorepo with ~60 submodules,
including `Containers`, `Database`, `Auth`, `Cache`, `Concurrency`, `EventBus`,
`MCP_Module`, `DebateOrchestrator`, `HelixMemory`, `HelixLLM`, `HelixQA`, and
many others. The root `go.mod` uses extensive `replace` directives to wire local
submodules together.

---

## Technology Stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.26 |
| HTTP Framework | Gin |
| gRPC | Protocol Buffers + `google.golang.org/grpc` |
| Database | PostgreSQL (via `pgxpool` / `digital.vasic.database`) |
| Cache | Redis (primary: port 6379, no password) |
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
cmd/                    # Application entry points (7 binaries)
  helixagent/           # Main production server
  api/                  # Standalone demo API
  grpc-server/          # gRPC facade
  mcp-bridge/           # MCP SSE bridge
  cognee-mock/          # Mock Cognee
  sanity-check/         # System validation
  generate-constitution/# Constitution generator

internal/               # Core application code (~50+ packages)
  router/               # Central Gin router setup, middleware, service initialization
  handlers/             # HTTP handlers (~40 files: OpenAI-compatible, debate, MCP, LSP, ACP, embeddings, etc.)
  services/             # Business logic: provider registry, ensemble, debate, caching, monitoring, OAuth
  llm/                  # LLM abstraction layer and 47+ provider implementations
  models/               # Core domain types (LLMRequest, LLMResponse, Message, etc.)
  database/             # PostgreSQL connectivity, migrations (embedded SQL strings)
  config/               # Centralized env-var based configuration
  middleware/           # Auth, compression, concurrency limiting, body size limits
  mcp/                  # MCP server registry, connection pooling, preinstaller, SSE bridge
  adapters/             # Adapters to external submodules (containers, database, auth, memory, messaging)
  features/             # Feature flags (GraphQL, Brotli, HTTP/3, etc.)
  cache/                # Caching layer
  security/             # Guardrails, red-team fixtures, normalization

pkg/                    # Public API packages
  api/                  # Generated protobuf code (llm-facade.pb.go, llm-facade_grpc.pb.go)

tests/                  # Test suites organized by type
  unit/                 # Unit tests by domain
  integration/          # Cross-service integration tests
  e2e/                  # End-to-end workflows
  security/             # Vulnerability scans
  stress/               # Load and saturation tests
  chaos/                # Fault injection
  challenge/            # Chaos/competition tests
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
   - **DebateService** (`internal/services/debate_service.go`) — Core debate
     with `ConductDebate()`, multi-round consensus, fallback chains,
     suspiciously-fast-response detection.
   - **Orchestrator Framework** (`internal/services/debate_integration/`) —
     Advanced 8-phase protocol (Dehallucination → SelfEvolvement → Proposal →
     Critique → Review → Optimization → Adversarial → Convergence) with agent
     pools and topology support (mesh/star/chain/tree).

4. **Database Migrations**: Embedded as SQL strings in `internal/database/db.go`.
   Migration files live in `internal/database/migrations/`.

---

## Build and Test Commands

All builds and tests are run manually or via Makefile targets. There is no CI/CD
automation in this repository (see Mandatory Constraints below).

### Build

```bash
make build              # Build the helixagent binary
make build-debug        # Build with debug symbols
make build-all          # Build all 7 applications
make release            # Full release build with version injection
```

### Quick Test

```bash
make test               # Auto-detects Postgres/Redis; runs full suite or falls back to -short
make test-unit          # go test ./internal/... -short
make test-integration   # Integration tests via scripts/run-integration-tests.sh
make test-e2e           # End-to-end tests
make test-security      # Security test suite
make test-stress        # Load and saturation tests
make test-chaos         # Chaos/competition tests
make test-bench         # go test -bench=. -benchmem ./...
make test-race          # Race detector run
make test-performance   # Performance benchmarks (//go:build performance)
make test-pentest       # Penetration tests (//go:build pentest)
```

### Complete Test Orchestration

```bash
make test-complete      # Runs all 6 test types with full Docker/Podman infra
make test-all           # scripts/run_all_tests.sh
make test-with-full-infra  # Starts Kafka, RabbitMQ, MinIO, etc. then tests (600s timeout)
```

### Test Infrastructure Management

```bash
make ensure-test-infra      # Auto-detects Docker/Podman, starts Postgres + Redis
make test-infra-start       # Start test infrastructure
make test-infra-stop        # Stop test infrastructure
make test-infra-full-start  # Full stack (basic, messaging, bigdata profiles)
```

### Other Useful Targets

```bash
make fmt                # gofmt + goimports
make vet                # go vet ./...
make lint               # golangci-lint run
make security-scan      # Orchestrates Gosec, Trivy, Snyk, SonarQube
make docker-build       # Build Docker image
make podman-build       # Build Podman image
make container-detect   # Detect available container runtime
make ci-validate-all    # Full CI validation including concurrency audit
```

### Resource Limits

All test runs are prefixed with `nice -n 19 ionice -c 3` and use:
- `GO_TEST_FLAGS := -p 1`
- `GOMAXPROCS := 2`

---

## Code Style Guidelines

### Go Conventions
- **Go version**: 1.26
- **Naming**: PascalCase for exported identifiers, camelCase for unexported.
  Constructor functions use `New*` prefix.
- **Context**: `context.Context` is always the first parameter when needed.
- **Errors**: Use `fmt.Errorf("...: %w", err)` for wrapping. Never panic in
  production code — return errors up the stack.
- **Struct tags**: JSON tags are required for serialized structs.
- **Logging**: Use `github.com/sirupsen/logrus` for structured logging.
  Pattern: `log.WithError(err).Warn("...")` or `log.WithFields(...).Info("...")`.
- **Interfaces**: Define close to consumers (e.g., `LLMProvider`, `EmbeddingGenerator`).

### Concurrency (CONST-029)
Any struct field that is a mutable collection (map, slice, channel-map) and is
accessed concurrently **MUST** use `safe.Store[K,V]` or `safe.Slice[T]` from
`digital.vasic.concurrency/pkg/safe`. Bare `sync.Mutex + map` / `sync.Mutex +
slice` is **prohibited for new code**.

- **Migration status**: ~39% drained (100 of 254 initial sites migrated).
- **Allowlist**: `scripts/concurrency-audit-allowlist.txt` tracks remaining sites.
- **Enforcement**: `scripts/concurrency-audit.sh` runs under `make ci-validate-all`.
- **Guide**: `docs/development/concurrency-playbook.md`

### Linting
- **`.golangci.yml`** (root): enables `errcheck`, `govet`, `staticcheck`,
  `unused`, `gosimple`, `ineffassign`, `typecheck`.
- **Skipped directories**: `cli_agents`, `MCP`, `MCP-Servers`, `vendor`.

### Commit Style
Conventional Commits: `type(scope): description`.
Examples: `feat(debate): add mesh topology`, `fix(handlers): correct embedding response format`, `docs(agents): update port architecture`.

---

## Testing Instructions

### Test Categories

| Category | Location | Mocks Allowed | Infrastructure Required |
|----------|----------|---------------|------------------------|
| Unit | `*_test.go` (run with `-short`) | Yes | None |
| Integration | `tests/integration/...` | **NO** | Postgres, Redis, HelixAgent on 7061 |
| E2E | `tests/e2e/...` | **NO** | Full test stack (docker-compose.test.yml) |
| Security | `tests/security/...` | **NO** | Full test stack |
| Stress | `tests/stress/...` | **NO** | Full test stack |
| Chaos/Challenge | `tests/challenge/...`, `tests/chaos/...` | **NO** | Full test stack |
| Performance | `tests/performance/...` | **NO** | Full stack |
| Benchmarks | `*_benchmark_test.go`, `*_bench_test.go` | **NO** | Varies |

### Skipping Strategy

Tests that require infrastructure MUST probe via TCP/HTTP and call `t.Skip()`
when services are unavailable in local development. They MUST NOT skip when
`CI=true` or `FULL_TEST_MODE=true`.

Key helpers in `internal/testutil/`:
- `RequireServer(t)` — skips if `:7061` not reachable
- `RequirePostgres(t)` — skips if Postgres TCP probe fails
- `RequireRedis(t)` — skips if Redis TCP probe fails
- `RequireMockLLM(t)` — skips if mock LLM `:18081` not up
- `RequireInfra(t)` — combo check
- `RequireEnv(t, envVar)` — skips if env var missing or placeholder-like
- `RequireAPIKey(t, provider)` — skips if provider API key missing
- `TestTimeout(t, d)` / `ShortTimeout(t)` / `MediumTimeout(t)` / `LongTimeout(t)`

### Test Infrastructure Stack (docker-compose.test.yml)

| Service | Container Name | External Port |
|---------|---------------|---------------|
| Mock LLM | helixagent-mock-llm | 18081 |
| PostgreSQL | helixagent-postgres | 15432 |
| Redis | helixagent-redis | 16379 |
| Ollama | helixagent-ollama | 11434 |
| HelixAgent | helixagent-app | 8080→7061 |
| Prometheus | helixagent-prometheus | 9090 |
| Grafana | helixagent-grafana | 3000 |

### Test Data
Shared fixtures live in `tests/fixtures/fixtures.go`:
- `MockProviders()`, `MockLLMRequests()`, `MockLLMResponses()`,
  `MockModelParameters(requestType)`, `MockUserSessions()`

### Mock LLM Server
`tests/mock-llm-server/main.go` is a Dockerized deterministic OpenAI-compatible
API. Provider configs in tests can point to it via `CLAUDE_BASE_URL=http://localhost:18081/v1`.

### Running a Single Test Package
```bash
go test -v -short ./internal/handlers/...
go test -v ./tests/integration/...
```

---

## Security Considerations

1. **No CI/CD Pipelines**: Automated pipelines are prohibited. Builds and tests
   are manual or Makefile-driven only.
2. **No Git Hooks**: Pre-commit, pre-push, and post-commit hooks are not
   installed. `.pre-commit-config.yaml` exists for reference only.
3. **Password Hashing**: Argon2id is used for user passwords.
4. **TLS**: TLS 1.3 is configured for HelixLLM and external provider connections.
5. **Guardrails**: `internal/security/guardrails.go` and `normalize.go` handle
   prompt injection, jailbreak, role reversal, and other red-team attack vectors.
   Red-team fixtures are in `internal/security/redteam/fixtures/*.yaml`.
6. **Secrets Scanning**: `detect-secrets` with `.secrets.baseline`.
7. **Security Scanners**: Gosec, Trivy, Snyk, SonarQube, Semgrep, KICS, Grype.
   Run via `make security-scan` or `scripts/security-scan.sh`.
8. **Gosec Baseline**: `.gosec-baseline.json` suppresses known false positives.
9. **Container Security**: Images run as non-root (`helixagent:1001`).
   Kubernetes manifests enforce `runAsNonRoot: true`, `readOnlyRootFilesystem: true`,
   and drop all capabilities.
10. **Non-cryptographic randomness**: Annotated with `#nosec G404` where
    applicable (jitter calculations).

---

## Deployment Processes

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
- `docker/mcp/docker-compose.mcp-full.yml` — 65+ MCP servers (ports 9101–9961)

### Kubernetes
Manifests are in `k8s/`:
- `k8s/base/` — Namespace, Deployment (2 replicas), Service, ConfigMap, HPA, PDB,
  NetworkPolicy, ServiceAccount
- `k8s/staging/` — Staging overlays
- `k8s/production/` — Production overlays (ingress, patches)

### Remote Container Distribution
Remote hosts are registered dynamically via `CONTAINERS_REMOTE_HOST_N_*`
environment variables in `Containers/.env`. N iterates 1..100. Enable with
`CONTAINERS_REMOTE_ENABLED=true`. No host name is hardcoded in source.

Audit configured hosts:
```bash
grep '^CONTAINERS_REMOTE_HOST_' Containers/.env
```

Direct `docker`/`podman` commands and ad-hoc remote hosts outside the `.env`
mechanism are strictly prohibited.

---

## Mandatory Constraints

These constraints are **permanent and non-negotiable**.

### CONST-025: No Mocks Outside Unit Tests

ONLY unit tests may use mocks, stubs, fakes, or placeholder implementations.
Integration tests, functional tests, E2E tests, Challenge tests, and HelixQA
tests MUST ALL execute against the REAL running HelixAgent system (port 7061)
with real containers, real databases, real Redis, and real HTTP calls. All
services and containers MUST be booted and operational before non-unit tests run.
Tests that cannot connect to real services MUST skip (not fail).

### CONST-026: Both Debate Flavors Must Be Tested Comprehensively

HelixAgent has TWO distinct debate implementations that BOTH require integration
tests against the LIVE API (`/v1/debates`):

1. **DebateService** (`internal/services/debate_service.go`) — Core debate with
   `ConductDebate()`, provider registry, suspiciously-fast-response detection,
   multi-round orchestration
2. **Orchestrator Framework** (`internal/services/debate_integration/`) —
   Advanced orchestrator with agent pools, 8-phase protocol, topology support
   (mesh/star/chain/tree)

Tests MUST cover:
- **5-position debates** (minimum viable multi-agent debate)
- **8+ position debates** (large-scale multi-agent debate)
- Error handling, timeout, fallback, and concurrent execution
- Voting methods, consensus detection, quality scoring

### CONST-027: Port and Service Architecture

**HelixAgent runs on port 7061** (NOT 8080). This is non-negotiable.

**Eager Services** (started at boot):
- HelixAgent: port 7061
- HelixLLM: port 8444 (HTTPS/TLS 1.3)
- PostgreSQL: port 5432
- Redis (primary): port 6379, NO password (container: helixagent-redis)
- MCP Bridge: port 9000
- MCP Servers: ports 9101-9803

**Lazy Services** (started on-demand):
- Cognee/HelixMemory: port 8000, ChromaDB: port 8001, Neo4j: ports 7474/7687,
  Qdrant: port 6333

**Redis Architecture**:
- `helixagent-redis` port 6379: NO password — HelixAgent core, streaming, tests
- `helixagent-mcp-redis-backend` port 16379: password `helixagent123` — MCP containers

**API Response Format Contracts** (server returns these, tests MUST match):
- `/v1/embeddings/providers` → providers as objects `{name,model,dimension,enabled}` (NOT strings)
- `/v1/vision/capabilities` → capabilities as objects `{id,name,status}` with status field
- `/v1/acp/agents` → agents as objects `{id,name,status}` with status field
- `/v1/acp/agents/{id}` → uses field `id` (NOT `agent_id`)
- `/v1/acp/execute` → uses field `agent_id`
- Health endpoints: `/v1/vision/health` works, `/v1/acp/health` works,
  `/v1/embeddings/health` returns 404 — use `/v1/embeddings/providers` instead

### CONST-028: Bugfix Documentation

All bug fixes MUST be documented in `docs/issues/fixed/BUGFIXES.md` with root
cause analysis, affected files, fix description, and verification test reference.
Every fix must have a corresponding verification test that proves the issue
cannot regress.

### CONST-029: Concurrent-Safe Containers

Any struct field that is a mutable collection (map, slice, channel-map) and is
accessed concurrently MUST use `safe.Store[K,V]` or `safe.Slice[T]` from
`digital.vasic.concurrency/pkg/safe`. Bare `sync.Mutex + map` / `sync.Mutex +
slice` combinations in shared state are prohibited for new code.

**Rationale:** The bare-mutex pattern is a review-caught bug class; the
primitives make forgetting the lock structurally impossible (there is no lock to
forget). We have shipped 18+ fixes against Pattern-A races (BUGFIXES #29, #30,
#34–#38); each fix was correct but the pattern that demanded fixing was wrong.

**Primitives:** `digital.vasic.concurrency/pkg/safe/{store,slice}.go` — generic,
10× race-clean, internal collection never exposed.

**Discipline and migration table:** `docs/development/concurrency-playbook.md`.

**Enforcement:** `scripts/concurrency-audit.sh` runs under `make ci-validate-all`.
New code failing the audit fails CI. Existing sites migrate per the playbook's
priority order; allowlist is temporary.

### CONST-030: Real Infrastructure for All Non-Unit Tests

Mocks, stubs, fakes, placeholders, and hardcoded data MAY ONLY be used in unit
tests (files ending `_test.go` run under `go test -short`). EVERY other test
type — integration, E2E, functional, security, stress, chaos, challenge,
benchmark, HelixQA, and any runtime verification — MUST execute against the REAL
running HelixAgent system with REAL containers, REAL databases, REAL Redis, REAL
MCP/ACP/LSP services, and REAL HTTP calls.

To enable this: before every non-unit test run, the HelixAgent binary MUST build,
distribute, and boot all containers per the Mandatory Container Orchestration
Flow. Non-unit tests that cannot connect to real services MUST skip (not fail).
Violations are critical infrastructure failures and block merge.

This rule strengthens and supersedes CONST-025.

### CONST-031: Authorized Remote Distribution Hosts

Remote distribution hosts are registered **dynamically** via
`CONTAINERS_REMOTE_HOST_N_*` environment variables in `Containers/.env`. N
iterates 1..100; the loader (`Containers/pkg/envconfig/parser.go`) stops at the
first absent `_NAME`. Adding an Nth host = append six env vars — no code change
required. The `.env` file is the sole source of truth; **no host name is
hardcoded in source, tests, challenges, or other governance docs**.

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

<!-- BEGIN_CONSTITUTION -->
# Project Constitution

**Version:** 1.2.0 | **Updated:** 2026-02-21 15:45

Constitution with 26 rules (26 mandatory) across categories: Quality: 2, Safety: 1, Security: 1, Performance: 2, Containerization: 3, Configuration: 1, Testing: 4, Documentation: 2, Principles: 2, Stability: 1, Observability: 1, GitOps: 2, CI/CD: 1, Architecture: 1, Networking: 1, Resource Management: 1

## Mandatory Principles

**All development MUST adhere to these non-negotiable principles:**

This constitution section is synchronized from CONSTITUTION.md. For the full text of all 26 mandatory rules, see CONSTITUTION.md and CONSTITUTION.json.
<!-- END_CONSTITUTION -->
