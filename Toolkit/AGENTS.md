# AGENTS.md - Toolkit Module

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

## Overview

Toolkit is a Go library for building AI-powered applications with multi-provider support,
specialized agents, and reusable infrastructure utilities. It provides unified interfaces for
chat completion, embeddings, reranking, and model discovery across AI providers, plus shared
packages for authentication, configuration, HTTP communication, rate limiting, error handling,
response parsing, and testing.

## Key Files

### Core (`pkg/toolkit/`)

- `toolkit.go` -- `Toolkit` struct: provider and agent registry with register/get/list operations
- `interfaces.go` -- Core interfaces (`Provider`, `Agent`), request/response types (`ChatRequest`, `ChatResponse`, `EmbeddingRequest`, `EmbeddingResponse`, `RerankRequest`, `RerankResponse`), model types (`ModelInfo`, `ModelCapabilities`, `ModelCategory`), `ProviderFactoryRegistry`

### Agents (`pkg/toolkit/agents/`)

- `generic.go` -- `GenericAgent`: general-purpose AI assistant, delegates to provider `Chat()`, configurable model/temperature/max_tokens
- `codereview.go` -- `CodeReviewAgent`: specialized code review with structured analysis (security, performance, maintainability, bugs, best practices)

### Common Utilities (`pkg/toolkit/common/`)

- `discovery/discovery.go` -- `DiscoveryService`: model discovery with caching (5-min TTL), filtering by criteria, sorting; `BaseDiscovery`, `DefaultCategoryInferrer`
- `http/client.go` -- `Client`: HTTP client with retry logic (exponential backoff), auth headers, `Get`/`Post`/`Put`/`Delete`/`DoRequest` methods
- `ratelimit/ratelimit.go` -- `TokenBucket`, `SlidingWindowLimiter`, `PerKeyLimiter`, `CircuitBreaker`, `RateLimiter` interface, HTTP `Middleware`

### Commons Libraries (`Commons/`)

- `auth/auth.go` -- `AuthManager` (API key + OAuth2 token refresh), `OAuth2Refresher` (client credentials/refresh token grants), `AuthInterceptor`, auth `Middleware` (HTTP transport wrapper)
- `config/config.go` -- `Config` map type with typed getters, `Validator` with rules (`Required`, `OneOf`, `MinLength`), `ProviderConfig`, env var loading
- `discovery/discovery.go` -- Generic model discovery framework: `CapabilityInferrer`, `CategoryInferrer`, `ModelFormatter` interfaces; `BaseDiscovery`, `DefaultCapabilityInferrer`, `DefaultCategoryInferrer`, `DefaultModelFormatter`
- `errors/errors.go` -- Error types (`ProviderError`, `APIError`, `RateLimitError`, `AuthenticationError`, `NetworkError`, `TimeoutError`, `ValidationError`), `ErrorHandler`, retryability checks
- `http/client.go` -- Advanced `Client` with rate limiting (`TokenBucket`), request/response interceptors, retry with exponential backoff
- `ratelimit/ratelimit.go` -- Simple `TokenBucket` rate limiter used by `Commons/http`
- `response/response.go` -- `JSONParser`, `StreamingParser` (SSE), `ErrorDetector`, `ResponseValidator`, `PaginationParser`, `ChunkedParser`, `ResponseBuilder`
- `testing/testing.go` -- `MockHTTPClient`, `MockProvider`, `TestServer`, `TestFixtures` (sample requests/responses), assertion helpers (`AssertChatResponse`, `AssertEmbeddingResponse`, `AssertRerankResponse`)

### Providers (`Providers/`)

- `Chutes/chutes.go` -- Chutes provider implementing `Provider` interface; auto-registers via `init()`
- `Chutes/builder.go` -- `ConfigBuilder` with `Build`/`Validate`/`Merge`; `Config` struct (api_key, base_url, timeout, retries, rate_limit)
- `Chutes/client.go` -- Chutes API client: `ChatCompletion`, `CreateEmbeddings`, `CreateRerank`, `GetModels`
- `Chutes/discovery.go` -- Model discovery for Chutes
- `SiliconFlow/siliconflow.go` -- SiliconFlow provider implementing `Provider` interface; auto-registers via `init()`
- `SiliconFlow/builder.go` -- `ConfigBuilder` with `Build`/`Validate`/`Merge`; `Config` struct
- `SiliconFlow/client.go` -- SiliconFlow API client: `ChatCompletion`, `CreateEmbeddings`, `CreateRerank`
- `SiliconFlow/discovery.go` -- Model discovery for SiliconFlow

### CLI (`cmd/toolkit/`)

- `main.go` -- CLI entry point with cobra: `version`, `test`, `chat`, `agent` commands; imports providers via blank imports for auto-registration

### Tests (`tests/`)

- `integration/framework.go` -- Integration test framework
- `integration/provider_integration_test.go` -- Provider integration tests
- `e2e/e2e_test.go` -- End-to-end tests
- `performance/benchmark_test.go` -- Benchmark tests
- `security/security_test.go` -- Security tests
- `chaos/chaos_test.go` -- Chaos engineering tests

## Exported Types Summary

### interfaces.go

- `Provider` -- Interface: `Name()`, `Chat()`, `Embed()`, `Rerank()`, `DiscoverModels()`, `ValidateConfig()`
- `Agent` -- Interface: `Name()`, `Execute()`, `ValidateConfig()`, `Capabilities()`
- `ChatRequest`, `ChatResponse` -- Chat completion request/response
- `EmbeddingRequest`, `EmbeddingResponse` -- Embedding request/response
- `RerankRequest`, `RerankResponse`, `RerankResult` -- Reranking types
- `Message`, `ChatMessage`, `Choice`, `ChatChoice`, `Usage`, `Embedding`, `EmbeddingData`, `RerankData` -- Supporting types
- `ModelInfo`, `ModelCapabilities`, `ModelCategory` -- Model metadata
- `ProviderFactory`, `ProviderFactoryRegistry` -- Factory pattern
- `RegisterProviderFactory()`, `CreateProvider()`, `ListProviders()` -- Global registry functions

### toolkit.go

- `Toolkit` -- Main struct: `RegisterProvider()`, `GetProvider()`, `RegisterAgent()`, `GetAgent()`, `ListProviders()`, `ListAgents()`

### agents/generic.go

- `GenericAgent` -- `NewGenericAgent(name, description, provider)`, `Execute()`, `ValidateConfig()`, `Capabilities()`, `SetConfig()`, `GetConfig()`

### agents/codereview.go

- `CodeReviewAgent` -- `NewCodeReviewAgent(name, provider)`, `Execute()`, `ValidateConfig()`, `Capabilities()`, `SetConfig()`, `GetConfig()`

## Integration with HelixAgent

The Toolkit module is referenced by the main HelixAgent project as a submodule. It provides
the foundational `Provider` and `Agent` interfaces and common utilities used across the
HelixAgent ecosystem for provider communication, configuration management, authentication,
rate limiting, and testing infrastructure.

## Development Standards

- All code must compile and pass `go vet ./...`
- Tests must use table-driven style with `testify`
- No mocks outside unit tests
- Run `make fmt vet lint` before committing
- Follow Conventional Commits: `feat(toolkit): ...`, `fix(toolkit): ...`
