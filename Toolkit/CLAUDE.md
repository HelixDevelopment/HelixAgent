# CLAUDE.md - Toolkit Module

## Universal Mandatory Constraints

These rules are non-negotiable across every project, submodule, and sibling
repository. They are derived from the HelixAgent root `CLAUDE.md`. Each
project MUST surface them in its own `CLAUDE.md`, `AGENTS.md`, and
`CONSTITUTION.md`. Project-specific addenda are welcome but cannot weaken
or override these.

### Hard Stops (permanent, non-negotiable)

1. **NO CI/CD pipelines.** No `.github/workflows/`, `.gitlab-ci.yml`,
   `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated pipeline.
   No Git hooks either. All builds and tests run manually or via Makefile/
   script targets.
2. **NO HTTPS for Git.** SSH URLs only (`git@github.com:…`,
   `git@gitlab.com:…`, etc.) for clones, fetches, pushes, and submodule
   updates. Including for public repos. SSH keys are configured on every
   service.
3. **NO manual container commands.** Container orchestration is owned by
   the project's binary/orchestrator (e.g. `make build` → `./bin/<app>`).
   Direct `docker`/`podman start|stop|rm` and `docker-compose up|down`
   are prohibited as workflows. The orchestrator reads its configured
   `.env` and brings up everything.

### Mandatory Development Standards

1. **100% Test Coverage.** Every component MUST have unit, integration,
   E2E, automation, security/penetration, and benchmark tests. No false
   positives. Mocks/stubs ONLY in unit tests; all other test types use
   real data and live services.
2. **Challenge Coverage.** Every component MUST have Challenge scripts
   (`./challenges/scripts/`) validating real-life use cases. No false
   success — validate actual behavior, not return codes.
3. **Real Data.** Beyond unit tests, all components MUST use actual API
   calls, real databases, live services. No simulated success. Fallback
   chains tested with actual failures.
4. **Health & Observability.** Every service MUST expose health
   endpoints. Circuit breakers for all external dependencies. Prometheus
   / OpenTelemetry integration where applicable.
5. **Documentation & Quality.** Update `CLAUDE.md`, `AGENTS.md`, and
   relevant docs alongside code changes. Pass language-appropriate
   format/lint/security gates. Conventional Commits:
   `<type>(<scope>): <description>`.
6. **Validation Before Release.** Pass the project's full validation
   suite (`make ci-validate-all`-equivalent) plus all challenges
   (`./challenges/scripts/run_all_challenges.sh`).
7. **No Mocks or Stubs in Production.** Mocks, stubs, fakes, placeholder
   classes, TODO implementations are STRICTLY FORBIDDEN in production
   code. All production code is fully functional with real integrations.
   Only unit tests may use mocks/stubs.
8. **Comprehensive Verification.** Every fix MUST be verified from all
   angles: runtime testing (actual HTTP requests / real CLI invocations),
   compile verification, code structure checks, dependency existence
   checks, backward compatibility, and no false positives in tests or
   challenges. Grep-only validation is NEVER sufficient.
9. **Resource Limits for Tests & Challenges (CRITICAL).** ALL test and
   challenge execution MUST be strictly limited to 30-40% of host system
   resources. Use `GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`, `-p 1`
   for `go test`. Container limits required. The host runs
   mission-critical processes — exceeding limits causes system crashes.
10. **Bugfix Documentation.** All bug fixes MUST be documented in
    `docs/issues/fixed/BUGFIXES.md` (or the project's equivalent) with
    root cause analysis, affected files, fix description, and a link to
    the verification test/challenge.
11. **Real Infrastructure for All Non-Unit Tests.** Mocks/fakes/stubs/
    placeholders MAY be used ONLY in unit tests (files ending `_test.go`
    run under `go test -short`, equivalent for other languages). ALL
    other test types — integration, E2E, functional, security, stress,
    chaos, challenge, benchmark, runtime verification — MUST execute
    against the REAL running system with REAL containers, REAL
    databases, REAL services, and REAL HTTP calls. Non-unit tests that
    cannot connect to real services MUST skip (not fail).
12. **Reproduction-Before-Fix (CONST-032 — MANDATORY).** Every reported
    error, defect, or unexpected behavior MUST be reproduced by a
    Challenge script BEFORE any fix is attempted. Sequence:
    (1) Write the Challenge first. (2) Run it; confirm fail (it
    reproduces the bug). (3) Then write the fix. (4) Re-run; confirm
    pass. (5) Commit Challenge + fix together. The Challenge becomes
    the regression guard for that bug forever.
13. **Concurrent-Safe Containers (Go-specific, where applicable).** Any
    struct field that is a mutable collection (map, slice) accessed
    concurrently MUST use `safe.Store[K,V]` / `safe.Slice[T]` from
    `digital.vasic.concurrency/pkg/safe` (or the project's equivalent
    primitives). Bare `sync.Mutex + map/slice` combinations are
    prohibited for new code.

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

## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

```bash
# Multi-provider Toolkit SDK: chat / embed / rerank through unified Provider
cd Toolkit && GOMAXPROCS=2 nice -n 19 go test -count=1 -race -v ./pkg/toolkit ./Providers/...
```
Expect: PASS; a sample Chat call completes against whichever Provider's API key is set (Chutes, SiliconFlow, or compatible OpenAI-shape endpoint). Without keys the provider-specific tests skip.


## Overview

`github.com/HelixDevelopment/helix_agent/Toolkit` is a Go library for building AI-powered applications
with multi-provider support, specialized agents, and common infrastructure utilities. It provides
unified interfaces for chat completion, embeddings, reranking, and model discovery across multiple
AI providers, plus reusable packages for authentication, configuration, HTTP clients, rate limiting,
error handling, response parsing, and testing.

**Module**: `github.com/HelixDevelopment/helix_agent/Toolkit` (Go 1.24+)

## Build & Test

```bash
make build                # Build all packages
make build-cli            # Build the CLI tool (bin/toolkit)
make build-all            # Build for linux/amd64, darwin/amd64, darwin/arm64, windows/amd64
make test                 # Run all tests
make test-unit            # Unit tests only (-short)
make test-integration     # Integration tests (tests/integration/)
make test-e2e             # End-to-end tests (tests/e2e/)
make test-performance     # Benchmarks (tests/performance/)
make test-security        # Security tests (tests/security/)
make test-chaos           # Chaos tests (tests/chaos/)
make test-coverage        # Coverage with HTML report
make test-fuzz            # Fuzz tests (10s)
make fmt                  # go fmt
make vet                  # go vet
make lint                 # golangci-lint
make security-scan        # gosec
make bench                # Benchmarks with memory profiling
```

Single test: `go test -v -run TestName ./path/to/package`

## Package Structure

| Package | Purpose |
|---------|---------|
| `cmd/toolkit` | CLI entry point: test, chat, agent, version commands (cobra) |
| `pkg/toolkit` | Core types: `Toolkit`, `Provider`, `Agent` interfaces, request/response types, provider factory registry |
| `pkg/toolkit/agents` | Built-in agents: `GenericAgent`, `CodeReviewAgent` |
| `pkg/toolkit/common/discovery` | Model discovery service with caching, filtering, sorting |
| `pkg/toolkit/common/http` | HTTP client with retry logic, exponential backoff, auth headers |
| `pkg/toolkit/common/ratelimit` | Token bucket, sliding window, per-key limiters, circuit breaker, HTTP middleware |
| `Commons/auth` | Authentication management: API key, OAuth2 token refresh, HTTP interceptor/middleware |
| `Commons/config` | Configuration map with typed getters, validation rules, env var loading, `ProviderConfig` |
| `Commons/discovery` | Generic model discovery framework: capability/category inference, model formatting |
| `Commons/errors` | Standardized error types: `ProviderError`, `APIError`, `RateLimitError`, `AuthenticationError`, `NetworkError`, `TimeoutError`, `ValidationError` with retryability checks |
| `Commons/http` | Advanced HTTP client with rate limiting, request/response interceptors, retry with backoff |
| `Commons/ratelimit` | Token bucket rate limiter (simple variant used by `Commons/http`) |
| `Commons/response` | Response parsing: JSON, SSE streaming, pagination, chunked, error detection, validation |
| `Commons/testing` | Test utilities: `MockHTTPClient`, `MockProvider`, `TestServer`, `TestFixtures`, assertion helpers |
| `Providers/Chutes` | Chutes provider: client, config builder, model discovery |
| `Providers/SiliconFlow` | SiliconFlow provider: client, config builder, model discovery |
| `tests/integration` | Integration test framework and provider tests |
| `tests/e2e` | End-to-end tests |
| `tests/performance` | Benchmark tests |
| `tests/security` | Security tests |
| `tests/chaos` | Chaos engineering tests |

## Key Types

### Core Interfaces (`pkg/toolkit`)

- `Provider` -- AI provider contract: `Name()`, `Chat()`, `Embed()`, `Rerank()`, `DiscoverModels()`, `ValidateConfig()`
- `Agent` -- AI agent contract: `Name()`, `Execute()`, `ValidateConfig()`, `Capabilities()`
- `ProviderFactory` -- Function type `func(config map[string]interface{}) (Provider, error)`
- `ProviderFactoryRegistry` -- Registry for provider factories with `Register()`, `Create()`, `ListProviders()`

### Request/Response Types (`pkg/toolkit`)

- `ChatRequest` / `ChatResponse` -- Chat completion with messages, model, temperature, max tokens, stop, penalties, logit bias
- `EmbeddingRequest` / `EmbeddingResponse` -- Text embeddings with encoding format and dimensions
- `RerankRequest` / `RerankResponse` -- Document reranking with query, documents, top-N
- `ModelInfo` / `ModelCapabilities` / `ModelCategory` -- Model metadata and capability flags (chat, embedding, rerank, audio, video, vision, function calling, context window)
- `Message` / `Choice` / `Usage` / `Embedding` / `RerankData` / `RerankResult` -- Supporting types

### Agents (`pkg/toolkit/agents`)

- `GenericAgent` -- General-purpose AI assistant with configurable model, temperature, max tokens
- `CodeReviewAgent` -- Specialized code review agent analyzing security, performance, maintainability, best practices, bugs

### Rate Limiting (`pkg/toolkit/common/ratelimit`)

- `TokenBucket` / `TokenBucketConfig` -- Token bucket rate limiter with `Allow()` and `Wait(ctx)`
- `SlidingWindowLimiter` -- Sliding window rate limiter
- `PerKeyLimiter` -- Per-key rate limiting (per IP, per user) with cleanup
- `CircuitBreaker` / `CircuitBreakerConfig` -- Circuit breaker with closed/open/half-open states
- `RateLimiter` -- Common interface: `Allow() bool`, `Wait(ctx) error`
- `Middleware` -- HTTP middleware for rate limiting with `Handler()` and `WaitHandler()`

### Authentication (`Commons/auth`)

- `AuthManager` -- Manages API key and OAuth2 token auth with thread-safe refresh
- `TokenRefresher` / `OAuth2Refresher` -- Token refresh interface and OAuth2 implementation (client credentials, refresh token grants)
- `AuthInterceptor` -- HTTP request interceptor adding auth headers
- `Middleware` -- HTTP client wrapper adding auth transport

### Configuration (`Commons/config`)

- `Config` -- Generic `map[string]interface{}` with typed getters (`GetString`, `GetInt`, `GetBool`, `GetFloat` with defaults)
- `Validator` / `ValidateFunc` -- Validation with `Required()`, `OneOf()`, `MinLength()` rules
- `ProviderConfig` -- Common provider config struct (API key, base URL, timeout, retries, rate limit)
- `LoadFromEnv()` / `LoadProviderConfigFromEnv()` -- Environment variable loading with prefix filtering

### Errors (`Commons/errors`)

- `ProviderError` -- Provider-specific with code, status, details
- `APIError` -- Parsed API error response
- `RateLimitError` -- Rate limit with retry-after
- `AuthenticationError` -- Auth failures
- `NetworkError` -- Network errors with unwrap
- `TimeoutError` -- Operation timeouts
- `ValidationError` -- Field validation
- `ErrorHandler` -- HTTP error handling and classification
- `IsRetryable()` / `IsRateLimit()` / `IsAuth()` / `GetRetryAfter()` -- Error type checking utilities

### Response Parsing (`Commons/response`)

- `JSONParser` -- JSON response/bytes parsing
- `StreamingParser` -- SSE streaming with data/error callbacks
- `ErrorDetector` -- HTTP status and JSON error detection
- `ResponseValidator` -- Required field validation
- `PaginationParser` -- Paginated response handling with next-page detection
- `ChunkedParser` -- Chunked response reading
- `ResponseBuilder` -- Response construction and sanitization

## Providers

Providers self-register via `init()` using `toolkit.RegisterProviderFactory()`. Import the provider
package with a blank import to auto-register:

```go
import (
    _ "github.com/HelixDevelopment/helix_agent/Toolkit/Providers/Chutes"
    _ "github.com/HelixDevelopment/helix_agent/Toolkit/Providers/SiliconFlow"
)
```

Each provider package contains:
- `<name>.go` -- Provider implementation (`Name`, `Chat`, `Embed`, `Rerank`, `DiscoverModels`, `ValidateConfig`)
- `builder.go` -- Config builder with `Build()`, `Validate()`, `Merge()`
- `client.go` -- API client for HTTP communication
- `discovery.go` -- Model discovery from provider API

## CLI Tool

The CLI (`cmd/toolkit`) provides four commands via cobra:

- `toolkit version` -- Show version
- `toolkit test` -- Run integration tests (provider creation, model discovery)
- `toolkit chat --provider <name> --api-key <key> [--model <model>] [--base-url <url>]` -- Interactive chat
- `toolkit agent --type <generic|codereview> --task <task> --api-key <key> [--provider <name>] [--model <model>]` -- Agent task execution

## Mandatory Development Standards

- 100% test coverage across unit, integration, E2E, security, chaos, performance, and fuzz tests
- No mocks outside unit tests -- all other tests use real implementations
- Challenges must validate real-life use cases, not just return codes
- Follow Conventional Commits: `feat(toolkit): ...`, `fix(toolkit): ...`
- Run `make fmt vet lint` before committing

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none |
| Downstream (these import this module) | none |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
