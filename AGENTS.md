# AGENTS.md

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- **NO Git hooks (pre-commit, pre-push, post-commit, etc.)** may be installed or configured
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

---

# HelixAgent: AI-Powered Ensemble LLM Service

## Project Overview

HelixAgent is a production-ready, AI-powered ensemble LLM service written in Go (1.25+) that aggregates responses from multiple language models to provide the most accurate and reliable outputs. It provides OpenAI-compatible APIs with support for 47+ LLM providers, debate orchestration, MCP adapters, and containerized infrastructure. (Authoritative count: `ls internal/llm/providers/ | grep -v common`.)

**Module**: `dev.helix.agent`

**Main Binary**: `helixagent` (built from `cmd/helixagent/`)

**Additional Applications**:
- `api` - Standalone API server
- `grpc-server` - gRPC service endpoint
- `cognee-mock` - Mock Cognee service for testing
- `sanity-check` - System validation tool
- `mcp-bridge` - MCP protocol bridge
- `generate-constitution` - Constitution file generator

---

## MANDATORY CONSTRAINTS

### CONST-025: No Mocks Outside Unit Tests

ONLY unit tests may use mocks, stubs, fakes, or placeholder implementations. Integration tests, functional tests, E2E tests, Challenge tests, and HelixQA tests MUST ALL execute against the REAL running HelixAgent system (port 7061) with real containers, real databases, real Redis, and real HTTP calls. All services and containers MUST be booted and operational before non-unit tests run. Tests that cannot connect to real services MUST skip (not fail).

### CONST-026: Both Debate Flavors Must Be Tested Comprehensively

HelixAgent has TWO distinct debate implementations that BOTH require integration tests against the LIVE API (`/v1/debates`):

1. **DebateService** (`internal/services/debate_service.go`) — Core debate with `ConductDebate()`, provider registry, suspiciously-fast-response detection, multi-round orchestration
2. **Orchestrator Framework** (`internal/services/debate_integration/`) — Advanced orchestrator with agent pools, 8-phase protocol, topology support (mesh/star/chain/tree)

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
- Cognee/HelixMemory: port 8000, ChromaDB: port 8001, Neo4j: ports 7474/7687, Qdrant: port 6333

**Redis Architecture**:
- `helixagent-redis` port 6379: NO password — HelixAgent core, streaming, tests
- `helixagent-mcp-redis-backend` port 16379: password `helixagent123` — MCP containers

**API Response Format Contracts** (server returns these, tests MUST match):
- `/v1/embeddings/providers` → providers as objects `{name,model,dimension,enabled}` (NOT strings)
- `/v1/vision/capabilities` → capabilities as objects `{id,name,status}` with status field
- `/v1/acp/agents` → agents as objects `{id,name,status}` with status field
- `/v1/acp/agents/{id}` → uses field `id` (NOT `agent_id`)
- `/v1/acp/execute` → uses field `agent_id`
- Health endpoints: `/v1/vision/health` works, `/v1/acp/health` works, `/v1/embeddings/health` returns 404 — use `/v1/embeddings/providers` instead

### CONST-028: Bugfix Documentation

All bug fixes MUST be documented in `docs/issues/fixed/BUGFIXES.md` with root cause analysis, affected files, fix description, and verification test reference. Every fix must have a corresponding verification test that proves the issue cannot regress.

### CONST-029: Concurrent-Safe Containers

Any struct field that is a mutable collection (map, slice, channel-map) and is accessed concurrently MUST use `safe.Store[K,V]` or `safe.Slice[T]` from `digital.vasic.concurrency/pkg/safe`. Bare `sync.Mutex + map` / `sync.Mutex + slice` combinations in shared state are prohibited for new code.

**Rationale:** The bare-mutex pattern is a review-caught bug class; the primitives make forgetting the lock structurally impossible (there is no lock to forget). We have shipped 18+ fixes against Pattern-A races (BUGFIXES #29, #30, #34–#38); each fix was correct but the pattern that demanded fixing was wrong.

**Primitives:** `digital.vasic.concurrency/pkg/safe/{store,slice}.go` — generic, 10× race-clean, internal collection never exposed.

**Discipline and migration table:** `docs/development/concurrency-playbook.md`.

**Enforcement:** `scripts/concurrency-audit.sh` runs under `make ci-validate-all`. New code failing the audit fails CI. Existing sites migrate per the playbook's priority order; allowlist is temporary.

### CONST-030: Real Infrastructure for All Non-Unit Tests

Mocks, stubs, fakes, placeholders, and hardcoded data MAY ONLY be used in unit tests (files ending `_test.go` run under `go test -short`). EVERY other test type — integration, E2E, functional, security, stress, chaos, challenge, benchmark, HelixQA, and any runtime verification — MUST execute against the REAL running HelixAgent system with REAL containers, REAL databases, REAL Redis, REAL MCP/ACP/LSP services, and REAL HTTP calls.

To enable this: before every non-unit test run, the HelixAgent binary MUST build, distribute, and boot all containers per the Mandatory Container Orchestration Flow. Non-unit tests that cannot connect to real services MUST skip (not fail). Violations are critical infrastructure failures and block merge.

This rule strengthens and supersedes CONST-025.

### CONST-031: Authorized Remote Distribution Hosts

Remote distribution hosts are registered **dynamically** via `CONTAINERS_REMOTE_HOST_N_*` environment variables in `Containers/.env`. N iterates 1..100; the loader (`Containers/pkg/envconfig/parser.go`) stops at the first absent `_NAME`. Adding an Nth host = append six env vars — no code change required. The `.env` file is the sole source of truth; **no host name is hardcoded in source, tests, challenges, or other governance docs**.

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

Enable with `CONTAINERS_REMOTE_ENABLED=true`. Every non-unit test run and every production deployment MUST use whichever hosts are currently configured when remote distribution is enabled.

Direct `docker`/`podman` commands, manual container start/stop, and ad-hoc remote hosts outside the `.env` mechanism are strictly prohibited per the Mandatory Container Orchestration Flow.

**Snapshot (2026-04-21)**: configured hosts are `thinker.local` and `amber.local`. Snapshot reflects `.env` state at that date; N scales freely to any number of hosts ≥ 1.
