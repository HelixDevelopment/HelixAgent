# HelixAgent Comprehensive Completion — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve all unfinished, broken, disconnected, and undocumented components across the entire HelixAgent project — achieving 100% test coverage, complete documentation, automated security scanning, and maximum performance optimization.

**Architecture:** 8 parallel workstreams in 3 dependency tiers. Tier 1 (WS1-WS4) runs immediately with no dependencies. Tier 2 (WS5-WS7) starts after its Tier 1 dependency completes. Tier 3 (WS8) starts after all Tier 2 workstreams complete.

**Tech Stack:** Go 1.24+, Gin, PostgreSQL/pgx, Redis, Docker/Podman, Prometheus, OpenTelemetry, gosec, Snyk, SonarQube, testify, quic-go, brotli

---

## Dependency Graph

```
TIER 1 (start immediately)
├── WS1: Dead Code & Router Cleanup
├── WS2: Git & Build Infrastructure Fixes
├── WS3: Documentation Completion
└── WS4: Security Scanning Automation

TIER 2 (after Tier 1 dependency)
├── WS5: Test Coverage Maximization ──► depends on WS1
├── WS6: Performance & Lazy Loading ──► depends on WS1
└── WS7: Monitoring & Metrics Tests ──► depends on WS2

TIER 3 (after all Tier 2)
└── WS8: Website, Courses & Manuals ──► depends on WS3, WS5, WS6, WS7
```

---

## Chunk 1: WS1 — Dead Code & Router Cleanup

### Task 1.1: Connect BackgroundTaskHandler to Router

**Files:**
- Modify: `internal/router/router.go`
- Reference: `internal/handlers/background_task_handler.go` (1056 lines, 21 methods)

- [ ] **Step 1: Read current router structure**

Read `internal/router/router.go` to understand the dependency injection pattern used for existing handlers.

- [ ] **Step 2: Add BackgroundTaskHandler instantiation in router.go**

In `internal/router/router.go`, after the existing handler instantiations (around line 860 where debate handler is created), add:

```go
// Background Tasks
backgroundTaskHandler := handlers.NewBackgroundTaskHandler(
    nil, // repository - will be set when background module is available
    nil, // queue
    nil, // workerPool
    nil, // resourceMonitor
    nil, // stuckDetector
    nil, // notificationHub
    nil, // sseManager
    nil, // wsServer
    nil, // webhookDispatcher
    nil, // pollingStore
    logger,
)
```

- [ ] **Step 3: Register BackgroundTaskHandler routes**

Add route group after existing routes:

```go
// Background Tasks API
tasks := v1.Group("/tasks")
{
    tasks.POST("", backgroundTaskHandler.CreateTask)
    tasks.GET("", backgroundTaskHandler.ListTasks)
    tasks.GET("/:id", backgroundTaskHandler.GetTask)
    tasks.GET("/:id/status", backgroundTaskHandler.GetTaskStatus)
    tasks.GET("/:id/logs", backgroundTaskHandler.GetTaskLogs)
    tasks.GET("/:id/resources", backgroundTaskHandler.GetTaskResources)
    tasks.GET("/:id/events", backgroundTaskHandler.GetTaskEvents)
    tasks.POST("/:id/pause", backgroundTaskHandler.PauseTask)
    tasks.POST("/:id/resume", backgroundTaskHandler.ResumeTask)
    tasks.POST("/:id/cancel", backgroundTaskHandler.CancelTask)
    tasks.DELETE("/:id", backgroundTaskHandler.DeleteTask)
    tasks.GET("/queue/stats", backgroundTaskHandler.GetQueueStats)
    tasks.GET("/events", backgroundTaskHandler.PollEvents)
    tasks.POST("/webhooks", backgroundTaskHandler.RegisterWebhook)
    tasks.GET("/webhooks", backgroundTaskHandler.ListWebhooks)
    tasks.DELETE("/webhooks/:id", backgroundTaskHandler.DeleteWebhook)
    tasks.POST("/:id/analyze", backgroundTaskHandler.AnalyzeTask)
    tasks.GET("/:id/ws", backgroundTaskHandler.HandleWebSocket)
}
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./cmd/helixagent/`
Expected: Clean compilation with no errors

- [ ] **Step 5: Commit**

```bash
git add internal/router/router.go
git commit -m "feat(router): connect BackgroundTaskHandler to /v1/tasks routes"
```

### Task 1.2: Connect DiscoveryHandler to Router

**Files:**
- Modify: `internal/router/router.go`
- Reference: `internal/handlers/discovery_handler.go` (380+ lines, 6 methods)

- [ ] **Step 1: Add DiscoveryHandler instantiation**

After model metadata handler initialization (around line 263), add:

```go
// Discovery
discoveryHandler := handlers.NewDiscoveryHandler(nil) // ModelDiscoveryService set at runtime
```

- [ ] **Step 2: Register routes under /v1/discovery**

```go
// Model Discovery API
discovery := v1.Group("/discovery")
{
    discovery.GET("/models", discoveryHandler.GetDiscoveredModels)
    discovery.GET("/models/selected", discoveryHandler.GetSelectedModels)
    discovery.GET("/stats", discoveryHandler.GetDiscoveryStats)
    discovery.POST("/trigger", discoveryHandler.TriggerDiscovery)
    discovery.GET("/ensemble", discoveryHandler.GetEnsembleModels)
    discovery.GET("/debate-model", discoveryHandler.GetModelForDebate)
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./cmd/helixagent/`

- [ ] **Step 4: Commit**

```bash
git add internal/router/router.go
git commit -m "feat(router): connect DiscoveryHandler to /v1/discovery routes"
```

### Task 1.3: Connect ScoringHandler to Router

**Files:**
- Modify: `internal/router/router.go`
- Reference: `internal/handlers/scoring_handler.go` (519 lines, 9 methods)

- [ ] **Step 1: Add ScoringHandler instantiation**

```go
scoringHandler := handlers.NewScoringHandler(nil) // ScoringService set at runtime
```

- [ ] **Step 2: Register routes under /v1/scoring**

```go
// Model Scoring API
scoring := v1.Group("/scoring")
{
    scoring.GET("/model/:name", scoringHandler.GetModelScore)
    scoring.POST("/batch", scoringHandler.BatchCalculateScores)
    scoring.GET("/top", scoringHandler.GetTopModels)
    scoring.GET("/range", scoringHandler.GetModelsByScoreRange)
    scoring.GET("/weights", scoringHandler.GetScoringWeights)
    scoring.PUT("/weights", scoringHandler.UpdateScoringWeights)
    scoring.GET("/model/:name/detail", scoringHandler.GetModelNameWithScore)
    scoring.POST("/cache/invalidate", scoringHandler.InvalidateCache)
    scoring.POST("/compare", scoringHandler.CompareModels)
}
```

- [ ] **Step 3: Verify and commit**

```bash
go build ./cmd/helixagent/
git add internal/router/router.go
git commit -m "feat(router): connect ScoringHandler to /v1/scoring routes"
```

### Task 1.4: Connect VerificationHandler to Router

**Files:**
- Modify: `internal/router/router.go`
- Reference: `internal/handlers/verification_handler.go` (452 lines, 8 methods)

- [ ] **Step 1: Add VerificationHandler instantiation**

```go
verificationHandler := handlers.NewVerificationHandler(nil, nil, nil, nil)
```

- [ ] **Step 2: Register routes under /v1/verification**

```go
// Verification API
verification := v1.Group("/verification")
{
    verification.POST("/model", verificationHandler.VerifyModel)
    verification.POST("/batch", verificationHandler.BatchVerify)
    verification.GET("/status", verificationHandler.GetVerificationStatus)
    verification.GET("/models", verificationHandler.GetVerifiedModels)
    verification.POST("/model/:name/reverify", verificationHandler.ReVerifyModel)
    verification.GET("/tests", verificationHandler.GetVerificationTests)
    verification.GET("/health", verificationHandler.GetVerificationHealth)
    verification.POST("/code-visibility", verificationHandler.TestCodeVisibility)
}
```

- [ ] **Step 3: Verify and commit**

```bash
go build ./cmd/helixagent/
git add internal/router/router.go
git commit -m "feat(router): connect VerificationHandler to /v1/verification routes"
```

### Task 1.5: Connect HealthHandler to Router

**Files:**
- Modify: `internal/router/router.go`
- Reference: `internal/handlers/health_handler.go` (466 lines, 12 methods)

- [ ] **Step 1: Add HealthHandler instantiation**

```go
healthHandler := handlers.NewHealthHandler(nil) // HealthService set at runtime
```

- [ ] **Step 2: Register routes under /v1/health**

```go
// Provider Health API
health := v1.Group("/health")
{
    health.GET("/providers", healthHandler.GetAllProvidersHealth)
    health.GET("/providers/healthy", healthHandler.GetHealthyProviders)
    health.GET("/providers/fastest", healthHandler.GetFastestProvider)
    health.GET("/provider/:name", healthHandler.GetProviderHealth)
    health.GET("/provider/:name/latency", healthHandler.GetProviderLatency)
    health.GET("/provider/:name/available", healthHandler.IsProviderAvailable)
    health.GET("/circuit-breakers", healthHandler.GetCircuitBreakerStatus)
    health.POST("/provider/:name/success", healthHandler.RecordSuccess)
    health.POST("/provider/:name/failure", healthHandler.RecordFailure)
    health.POST("/provider", healthHandler.AddProvider)
    health.DELETE("/provider/:name", healthHandler.RemoveProvider)
    health.GET("/status", healthHandler.GetHealthServiceStatus)
}
```

- [ ] **Step 3: Verify and commit**

```bash
go build ./cmd/helixagent/
git add internal/router/router.go
git commit -m "feat(router): connect HealthHandler to /v1/health routes"
```

### Task 1.6: Remove Dead Handlers (CogneeHandler duplicate, GraphQLHandler, OpenRouterModelsHandler)

**Files:**
- Delete: `internal/handlers/cognee.go` (60 lines — superseded by `cognee_api.go`)
- Delete: `internal/handlers/graphql_handler.go` (404 lines — feature disabled, never connected)
- Delete: `internal/handlers/openrouter_models.go` (328 lines — model listing via UnifiedHandler)
- Delete associated test files if they only test the removed handlers

- [ ] **Step 1: Verify cognee.go is the duplicate (not cognee_api.go)**

Confirm that `NewCogneeAPIHandler` (used in router.go line 302) is defined in a different file from `NewCogneeHandler`.

- [ ] **Step 2: Remove dead handler files**

```bash
rm internal/handlers/cognee.go
rm internal/handlers/graphql_handler.go
rm internal/handlers/openrouter_models.go
```

- [ ] **Step 3: Remove associated test files**

```bash
rm internal/handlers/cognee_test.go
rm internal/handlers/graphql_handler_test.go
rm internal/handlers/openrouter_models_test.go
```

- [ ] **Step 4: Remove GraphQL resolver tests if they only test GraphQL handler**

Check `internal/handlers/graphql_resolver_test.go` — if it only tests GraphQL handler, remove it.

- [ ] **Step 5: Verify compilation after removal**

Run: `go build ./...`
Expected: Clean build. If any imports break, fix them.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(handlers): remove dead CogneeHandler, GraphQLHandler, OpenRouterModelsHandler"
```

### Task 1.7: Clean Up Repository Interfaces

**Files:**
- Modify or Delete: `internal/repository/repository.go` (97 lines, 6 unused interfaces)

- [ ] **Step 1: Search for imports of internal/repository**

```bash
grep -r "internal/repository" --include="*.go" | grep -v "_test.go" | grep -v "vendor/"
```

- [ ] **Step 2: If no production code imports it, delete the file**

```bash
rm internal/repository/repository.go
```

- [ ] **Step 3: If test files import it, update them to use concrete types from internal/database/**

- [ ] **Step 4: Verify and commit**

```bash
go build ./...
git add -A
git commit -m "refactor(repository): remove unused repository interfaces"
```

### Task 1.8: Create Router Completeness Challenge

**Files:**
- Create: `challenges/scripts/router_completeness_challenge.sh`

- [ ] **Step 1: Write challenge script**

Create a challenge that verifies every handler in `internal/handlers/` is either:
(a) registered in `router.go`, or
(b) explicitly marked as a helper/internal handler

- [ ] **Step 2: Test challenge execution**

```bash
bash challenges/scripts/router_completeness_challenge.sh
```

- [ ] **Step 3: Commit**

```bash
git add challenges/scripts/router_completeness_challenge.sh
git commit -m "test(challenges): add router completeness validation challenge"
```

---

## Chunk 2: WS2 — Git & Build Infrastructure Fixes

### Task 2.1: Convert HTTPS Submodule URLs to SSH

**Files:**
- Modify: `.gitmodules`

- [ ] **Step 1: Convert all HTTPS URLs to SSH**

Use sed to replace all `https://github.com/` with `git@github.com:` in `.gitmodules`:

```bash
sed -i 's|url = https://github.com/\(.*\)\.git|url = git@github.com:\1.git|g' .gitmodules
```

- [ ] **Step 2: Verify the changes**

```bash
grep -n "https://" .gitmodules
```
Expected: No HTTPS URLs remaining.

- [ ] **Step 3: Sync submodule URLs**

```bash
git submodule sync
```

- [ ] **Step 4: Commit**

```bash
git add .gitmodules
git commit -m "fix(git): convert all submodule URLs from HTTPS to SSH per Constitution"
```

### Task 2.2: Fix BackgroundTasks Submodule URL

**Files:**
- Modify: `.gitmodules`

- [ ] **Step 1: Fix the relative URL**

Find the line with `url = ./BackgroundTasks` and replace with proper SSH URL:

```bash
sed -i 's|url = \./BackgroundTasks|url = git@github.com:vasic-digital/BackgroundTasks.git|g' .gitmodules
```

- [ ] **Step 2: Sync and commit**

```bash
git submodule sync
git add .gitmodules
git commit -m "fix(git): fix BackgroundTasks submodule to use proper SSH URL"
```

### Task 2.3: Add Resource Limits to All Makefile Test Targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Define resource limit variables at top of Makefile**

Add after existing variable definitions:

```makefile
# Resource limits per Constitution Rule 15 (30-40% host resources)
RESOURCE_PREFIX := nice -n 19 ionice -c 3
GO_TEST_FLAGS := -p 1
export GOMAXPROCS := 2
```

- [ ] **Step 2: Update test-unit target (line ~384)**

Change from:
```makefile
test-unit:
	@echo "Running unit tests..."
	go test -v ./internal/... -short
```
To:
```makefile
test-unit:
	@echo "Running unit tests (resource-limited)..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./internal/... -short
```

- [ ] **Step 3: Update all remaining test targets similarly**

Apply the `$(RESOURCE_PREFIX)` prefix and `$(GO_TEST_FLAGS)` to: `test`, `test-coverage`, `test-coverage-100`, `test-e2e`, `test-security`, `test-stress`, `test-chaos`, `test-bench`, `test-race`, `test-automation`.

- [ ] **Step 4: Verify Makefile syntax**

```bash
make -n test-unit
```
Expected: Shows the command with nice/ionice/GOMAXPROCS.

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "fix(build): add mandatory resource limits to all test targets per Constitution Rule 15"
```

### Task 2.4: Create Initial Schema Migration (001)

**Files:**
- Create: `scripts/migrations/001_initial_schema.sql`
- Reference: `scripts/run-integration-tests.sh` (lines 168-317 contain the full schema)

- [ ] **Step 1: Extract schema from integration test runner**

Copy the SQL schema from `scripts/run-integration-tests.sh` lines 168-317 into a standalone migration file.

- [ ] **Step 2: Write migration file**

Create `scripts/migrations/001_initial_schema.sql` with proper migration header and all CREATE TABLE statements.

- [ ] **Step 3: Commit**

```bash
git add scripts/migrations/001_initial_schema.sql
git commit -m "feat(db): add initial schema migration 001"
```

### Task 2.5: Fix SonarQube Configuration Mismatches

**Files:**
- Modify: `docker/security/sonarqube/sonar-project.properties`
- Modify: `docker/security/sonarqube/docker-compose.yml`

- [ ] **Step 1: Sync version to 1.3.0**

In `docker/security/sonarqube/sonar-project.properties`, change `sonar.projectVersion=1.0.0` to `sonar.projectVersion=1.3.0`.

- [ ] **Step 2: Fix test report path**

Change `sonar.go.tests.reportPaths=test-report.out` to `sonar.go.tests.reportPaths=test-report.json`.

- [ ] **Step 3: Pin Docker image versions**

In `docker/security/sonarqube/docker-compose.yml`:
- Change `sonarqube:community` to `sonarqube:10.7-community`
- Change `sonarsource/sonar-scanner-cli:latest` to `sonarsource/sonar-scanner-cli:5.0`

- [ ] **Step 4: Commit**

```bash
git add docker/security/sonarqube/
git commit -m "fix(security): sync SonarQube config versions and pin Docker images"
```

### Task 2.6: Create Resource Limits Challenge

**Files:**
- Create: `challenges/scripts/resource_limits_challenge.sh`

- [ ] **Step 1: Write challenge that validates all test targets have resource limits**

Script should grep Makefile for test targets and verify each contains `RESOURCE_PREFIX` or `GOMAXPROCS`.

- [ ] **Step 2: Commit**

```bash
git add challenges/scripts/resource_limits_challenge.sh
git commit -m "test(challenges): add resource limits validation challenge"
```

---

## Chunk 3: WS3 — Documentation Completion

### Task 3.1: Expand SelfImprove README

**Files:**
- Modify: `SelfImprove/README.md` (currently 6 lines)
- Reference: `SelfImprove/selfimprove/` (7 Go files: feedback.go, integration.go, optimizer.go, reward.go, types.go + tests)

- [ ] **Step 1: Read all Go source files in the module**

Read `feedback.go`, `integration.go`, `optimizer.go`, `reward.go`, `types.go` to understand the API.

- [ ] **Step 2: Write comprehensive README (150+ lines)**

Cover: module overview, architecture, package structure, API reference (all exported types/functions), usage examples, configuration, testing instructions, integration with HelixAgent.

- [ ] **Step 3: Commit**

```bash
git add SelfImprove/README.md
git commit -m "docs(selfimprove): expand README with comprehensive API documentation"
```

### Task 3.2: Expand Benchmark README

**Files:**
- Modify: `Benchmark/README.md` (currently 6 lines)
- Reference: `Benchmark/benchmark/` (5 Go files: integration.go, runner.go, types.go + tests)

- [ ] **Step 1-3: Same pattern as Task 3.1**

### Task 3.3: Expand Agentic README

**Files:**
- Modify: `Agentic/README.md` (currently 19 lines)
- Reference: `Agentic/agentic/` (2 Go files: workflow.go + test)

### Task 3.4: Expand LLMOps README

**Files:**
- Modify: `LLMOps/README.md` (currently 11 lines)
- Reference: `LLMOps/llmops/` (9 Go files: evaluator.go, experiments.go, integration.go, prompts.go, types.go + tests)

### Task 3.5: Expand Planning README

**Files:**
- Modify: `Planning/README.md` (currently 40 lines)
- Reference: `Planning/planning/` (6 Go files: hiplan.go, mcts.go, tree_of_thoughts.go + tests)

### Task 3.6: Expand RAG, Streaming, Security, Optimization, Plugins, Memory READMEs

**Files:**
- Modify: `RAG/README.md` (41 lines → 100+)
- Modify: `Streaming/README.md` (39 lines → 100+)
- Modify: `security/README.md` (39 lines → 100+)
- Modify: `Optimization/README.md` (36 lines → 100+)
- Modify: `Plugins/README.md` (35 lines → 100+)
- Modify: `Memory/README.md` (38 lines → 100+)

- [ ] **Step 1: Read each module's pkg/ directory to understand exported API**
- [ ] **Step 2: Expand each README with: architecture, API reference, usage examples, testing**
- [ ] **Step 3: Commit each module separately**

### Task 3.7: Create SDK Documentation

**Files:**
- Create: `sdk/android/README.md`
- Create: `sdk/ios/README.md`
- Create: `sdk/cli/README.md`
- Reference: `sdk/android/SuperAgent.kt` (42KB), `sdk/ios/SuperAgent.swift` (41KB), `sdk/cli/helixagent-cli.js` (13KB)

- [ ] **Step 1: Read each SDK source file to understand the API surface**
- [ ] **Step 2: Write comprehensive README for each (installation, usage, API reference, examples)**
- [ ] **Step 3: Commit**

```bash
git add sdk/android/README.md sdk/ios/README.md sdk/cli/README.md
git commit -m "docs(sdk): add comprehensive README for Android, iOS, and CLI SDKs"
```

### Task 3.8: Add Debate Subdirectory READMEs

**Files:**
- Create: `internal/debate/audit/README.md`
- Create: `internal/debate/evaluation/README.md`
- Create: `internal/debate/gates/README.md`
- Create: `internal/debate/reflexion/README.md`
- Create: `internal/debate/testing/README.md`
- Create: `internal/debate/tools/README.md`

- [ ] **Step 1: Read Go files in each subdirectory**
- [ ] **Step 2: Write README documenting: purpose, exported types, usage within debate system**
- [ ] **Step 3: Commit**

```bash
git add internal/debate/*/README.md
git commit -m "docs(debate): add README documentation for 6 debate subsystem packages"
```

### Task 3.9: Synchronize CLAUDE.md, AGENTS.md, CONSTITUTION.md

**Files:**
- Modify: `CLAUDE.md`
- Modify: `AGENTS.md`
- Modify: `CONSTITUTION.md`

- [ ] **Step 1: Diff all three files to identify discrepancies**
- [ ] **Step 2: Add any new routes, handlers, or features to all three**
- [ ] **Step 3: Update the "Protocol Endpoints" section with new endpoints from WS1**
- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md AGENTS.md CONSTITUTION.md
git commit -m "docs: synchronize CLAUDE.md, AGENTS.md, and CONSTITUTION.md"
```

### Task 3.10: Create Documentation Completeness Challenge

**Files:**
- Create: `challenges/scripts/documentation_completeness_challenge.sh`

- [ ] **Step 1: Write challenge that validates every module has README.md, CLAUDE.md, AGENTS.md**
- [ ] **Step 2: Validate all SDK directories have README.md**
- [ ] **Step 3: Validate all debate subdirectories have README.md**
- [ ] **Step 4: Commit**

---

## Chunk 4: WS4 — Security Scanning Automation

### Task 4.1: Create Automated Snyk Scanning Challenge

**Files:**
- Create: `challenges/scripts/snyk_automated_scanning_challenge.sh`
- Reference: `docker/security/snyk/docker-compose.yml`, `docker/security/snyk/Dockerfile`

- [ ] **Step 1: Write challenge script**

Script should:
1. Check that Snyk compose file exists and is valid
2. Build the Snyk scanner container
3. Run `snyk test` via container against the project
4. Parse results for critical/high vulnerabilities
5. Assert no critical vulnerabilities exist
6. Record metrics (total issues, severity breakdown)

- [ ] **Step 2: Test locally**

```bash
bash challenges/scripts/snyk_automated_scanning_challenge.sh
```

- [ ] **Step 3: Commit**

```bash
git add challenges/scripts/snyk_automated_scanning_challenge.sh
git commit -m "test(challenges): add automated Snyk scanning challenge"
```

### Task 4.2: Create Automated SonarQube Scanning Challenge

**Files:**
- Create: `challenges/scripts/sonarqube_automated_scanning_challenge.sh`
- Reference: `docker/security/sonarqube/docker-compose.yml`

- [ ] **Step 1: Write challenge script**

Script should:
1. Start SonarQube via compose (wait for healthy)
2. Run sonar-scanner against the project
3. Query SonarQube API for quality gate status
4. Assert quality gate passes
5. Record metrics (bugs, vulnerabilities, code smells, coverage)

- [ ] **Step 2: Commit**

```bash
git add challenges/scripts/sonarqube_automated_scanning_challenge.sh
git commit -m "test(challenges): add automated SonarQube scanning challenge"
```

### Task 4.3: Create Security Scanning Integration Tests

**Files:**
- Create: `tests/security/snyk_integration_test.go`
- Create: `tests/security/sonarqube_integration_test.go`

- [ ] **Step 1: Write Go integration tests that validate scanning infrastructure**

```go
func TestSnyk_ComposeFileExists(t *testing.T) {
    _, err := os.Stat("docker/security/snyk/docker-compose.yml")
    require.NoError(t, err)
}

func TestSnyk_DockerfileValid(t *testing.T) {
    _, err := os.Stat("docker/security/snyk/Dockerfile")
    require.NoError(t, err)
}

func TestSonarQube_ConfigurationSync(t *testing.T) {
    // Verify root and docker sonar configs have matching versions
}
```

- [ ] **Step 2: Commit**

```bash
git add tests/security/snyk_integration_test.go tests/security/sonarqube_integration_test.go
git commit -m "test(security): add Snyk and SonarQube integration tests"
```

---

## Chunk 5: WS5 — Test Coverage Maximization

### Task 5.1: Add Go Native Fuzzing Tests

**Files:**
- Create: `tests/fuzz/json_parsing_fuzz_test.go`
- Create: `tests/fuzz/tool_schema_fuzz_test.go`
- Create: `tests/fuzz/protocol_parsing_fuzz_test.go`
- Create: `tests/fuzz/input_validation_fuzz_test.go`

- [ ] **Step 1: Write JSON parsing fuzz test**

```go
package fuzz

import (
    "encoding/json"
    "testing"
)

func FuzzJSONParsing(f *testing.F) {
    // Seed corpus
    f.Add([]byte(`{"messages":[{"role":"user","content":"hello"}]}`))
    f.Add([]byte(`{"model":"gpt-4","temperature":0.7}`))
    f.Add([]byte(`{}`))
    f.Add([]byte(`[]`))

    f.Fuzz(func(t *testing.T, data []byte) {
        var req map[string]interface{}
        // Should not panic on any input
        _ = json.Unmarshal(data, &req)
    })
}
```

- [ ] **Step 2: Write tool schema fuzz test**

Test that `internal/tools/schema.go` handles malformed tool definitions without panicking.

- [ ] **Step 3: Write protocol parsing fuzz test**

Test MCP JSON-RPC, ACP, LSP message parsing with random inputs.

- [ ] **Step 4: Write input validation fuzz test**

Test all HTTP handler input parsing paths.

- [ ] **Step 5: Run fuzz tests briefly to verify they work**

```bash
go test -fuzz=FuzzJSONParsing -fuzztime=10s ./tests/fuzz/
```

- [ ] **Step 6: Commit**

```bash
git add tests/fuzz/
git commit -m "test(fuzz): add Go native fuzzing tests for JSON, tools, protocols, input validation"
```

### Task 5.2: Add pprof Memory Profiling Challenge

**Files:**
- Create: `challenges/scripts/pprof_memory_profiling_challenge.sh`
- Create: `tests/performance/pprof_leak_detection_test.go`

- [ ] **Step 1: Write pprof challenge script**

Script should:
1. Start HelixAgent with `EnablePProf=true`
2. Send load (100 requests)
3. Capture heap profile via `/debug/pprof/heap`
4. Capture goroutine profile via `/debug/pprof/goroutine`
5. Assert goroutine count is bounded
6. Assert heap allocation growth is linear (not exponential)

- [ ] **Step 2: Write Go profiling test**

```go
func TestMemoryLeak_GoroutineCount(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping profiling test in short mode")
    }
    runtime.GOMAXPROCS(2)

    baseline := runtime.NumGoroutine()

    // Simulate work cycles
    for i := 0; i < 100; i++ {
        // Create and complete operations
    }

    runtime.GC()
    time.Sleep(200 * time.Millisecond)

    current := runtime.NumGoroutine()
    leaked := current - baseline
    assert.LessOrEqual(t, leaked, 5,
        "goroutine count should return near baseline")
}
```

- [ ] **Step 3: Commit**

```bash
git add challenges/scripts/pprof_memory_profiling_challenge.sh tests/performance/pprof_leak_detection_test.go
git commit -m "test(perf): add pprof memory profiling challenge and leak detection tests"
```

### Task 5.3: Add Coverage Gate Challenge

**Files:**
- Create: `challenges/scripts/coverage_gate_challenge.sh`

- [ ] **Step 1: Write challenge that enforces minimum coverage thresholds**

Script should:
1. Run `go test -coverprofile=coverage.out ./internal/...`
2. Parse coverage percentage per package
3. Assert each package >= 80% coverage
4. Assert overall coverage >= 85%
5. Report packages below threshold

- [ ] **Step 2: Commit**

```bash
git add challenges/scripts/coverage_gate_challenge.sh
git commit -m "test(challenges): add coverage gate validation challenge (80% per package)"
```

### Task 5.4: Expand Stress Tests for New Handlers

**Files:**
- Create: `tests/stress/background_tasks_stress_test.go`
- Create: `tests/stress/discovery_stress_test.go`
- Create: `tests/stress/scoring_stress_test.go`
- Create: `tests/stress/verification_stress_test.go`
- Create: `tests/stress/health_handler_stress_test.go`

- [ ] **Step 1: Write stress tests following existing pattern**

Each test should:
- Set `runtime.GOMAXPROCS(2)`
- Skip in short mode
- Use 200+ goroutines with atomic counters
- Include deadlock detection timeout
- Report operation counts

- [ ] **Step 2: Run and verify**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -v -p 1 -run TestBackground ./tests/stress/ -timeout 120s
```

- [ ] **Step 3: Commit**

```bash
git add tests/stress/
git commit -m "test(stress): add stress tests for newly connected handlers"
```

### Task 5.5: Add Chaos Engineering Tests

**Files:**
- Create: `tests/chaos/provider_failure_chaos_test.go`
- Create: `tests/chaos/network_partition_test.go`

- [ ] **Step 1: Write chaos tests that simulate provider failures**

```go
func TestChaos_AllProvidersDown(t *testing.T) {
    // Verify system degrades gracefully when all providers fail
}

func TestChaos_IntermittentFailures(t *testing.T) {
    // 50% of requests to mock provider fail randomly
    // Verify circuit breaker activates and fallback works
}
```

- [ ] **Step 2: Commit**

```bash
git add tests/chaos/
git commit -m "test(chaos): add chaos engineering tests for provider failures"
```

---

## Chunk 6: WS6 — Performance & Lazy Loading

### Task 6.1: Audit and Convert init() Functions

**Files:**
- Modify: Various files in `internal/` that use `func init()`

- [ ] **Step 1: Find all init() functions**

```bash
grep -rn "func init()" internal/ --include="*.go" | grep -v "_test.go" | grep -v "vendor/"
```

- [ ] **Step 2: For each init(), evaluate if it can be converted to lazy initialization**

Candidates for conversion:
- Package-level variable initialization that isn't needed at import time
- Registration of providers/handlers that could be deferred
- Configuration parsing that could be lazy

- [ ] **Step 3: Convert each candidate to sync.Once pattern**

```go
var (
    instance *MyType
    once     sync.Once
)

func getInstance() *MyType {
    once.Do(func() {
        instance = &MyType{/* ... */}
    })
    return instance
}
```

- [ ] **Step 4: Verify no behavioral changes**

```bash
go test ./internal/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/
git commit -m "perf: convert eligible init() functions to lazy sync.Once initialization"
```

### Task 6.2: Add Semaphore-Based Concurrency Limiting

**Files:**
- Modify: `internal/services/ensemble.go`
- Modify: `internal/services/provider_registry.go`

- [ ] **Step 1: Verify existing semaphore usage**

The provider_registry already uses `semaphore.Weighted`. Verify ensemble.go also limits concurrent provider calls.

- [ ] **Step 2: If ensemble.go lacks limiting, add semaphore**

```go
import "golang.org/x/sync/semaphore"

var ensembleSem = semaphore.NewWeighted(int64(runtime.NumCPU() * 2))
```

- [ ] **Step 3: Wrap concurrent provider calls with semaphore acquire/release**

- [ ] **Step 4: Add benchmark comparing throughput before/after**

- [ ] **Step 5: Commit**

```bash
git add internal/services/
git commit -m "perf: add semaphore-based concurrency limiting to ensemble orchestration"
```

### Task 6.3: Create Performance Benchmark Suite

**Files:**
- Create: `tests/performance/lazy_loading_benchmark_test.go`
- Create: `tests/performance/semaphore_benchmark_test.go`

- [ ] **Step 1: Write benchmark comparing eager vs lazy initialization**

```go
func BenchmarkProvider_EagerInit(b *testing.B) { /* ... */ }
func BenchmarkProvider_LazyInit(b *testing.B) { /* ... */ }
```

- [ ] **Step 2: Write benchmark for semaphore overhead**

- [ ] **Step 3: Commit**

```bash
git add tests/performance/
git commit -m "test(bench): add lazy loading and semaphore performance benchmarks"
```

---

## Chunk 7: WS7 — Monitoring & Metrics Tests

### Task 7.1: Expand Monitoring Test Suite

**Files:**
- Create: `tests/monitoring/circuit_breaker_transitions_test.go`
- Create: `tests/monitoring/provider_latency_tracking_test.go`
- Create: `tests/monitoring/cache_hit_ratio_test.go`
- Create: `tests/monitoring/database_query_performance_test.go`

- [ ] **Step 1: Write circuit breaker state transition monitoring test**

Follow existing pattern from `tests/monitoring/metrics_collection_test.go`:

```go
func TestMonitoring_CircuitBreaker_StateTransitions(t *testing.T) {
    registry := prometheus.NewRegistry()
    // Create counter for state transitions
    // Trigger closed→open→half-open→closed cycle
    // Assert metrics correctly track each transition
}
```

- [ ] **Step 2: Write provider latency tracking test**
- [ ] **Step 3: Write cache hit ratio monitoring test**
- [ ] **Step 4: Write database query performance test**
- [ ] **Step 5: Commit**

```bash
git add tests/monitoring/
git commit -m "test(monitoring): add circuit breaker, latency, cache, and DB monitoring tests"
```

### Task 7.2: Create Monitoring Dashboard Challenge

**Files:**
- Create: `challenges/scripts/monitoring_dashboard_challenge.sh`

- [ ] **Step 1: Write challenge that validates full monitoring stack**

Script should:
1. Verify Prometheus config exists and is valid
2. Verify Grafana dashboards exist
3. Start monitoring stack via compose
4. Query Prometheus for expected metrics
5. Validate metric names and labels
6. Check alerting rules are configured

- [ ] **Step 2: Commit**

```bash
git add challenges/scripts/monitoring_dashboard_challenge.sh
git commit -m "test(challenges): add monitoring dashboard validation challenge"
```

---

## Chunk 8: WS8 — Website, Video Courses & User Manuals

### Task 8.1: Update Existing User Manuals

**Files:**
- Modify: `Website/user-manuals/04-api-reference.md` (add new endpoints from WS1)
- Modify: `Website/user-manuals/08-troubleshooting.md` (add new handler troubleshooting)
- Modify: `Website/user-manuals/17-security-scanning-guide.md` (add automated scanning from WS4)
- Modify: `Website/user-manuals/18-performance-monitoring.md` (add new monitoring from WS7)
- Modify: `Website/user-manuals/20-testing-strategies.md` (add fuzzing from WS5)

- [ ] **Step 1: Update API reference with new endpoints**

Add sections for: `/v1/tasks/*`, `/v1/discovery/*`, `/v1/scoring/*`, `/v1/verification/*`, `/v1/health/*`

- [ ] **Step 2: Update security scanning guide with automated Snyk/SonarQube**
- [ ] **Step 3: Update performance monitoring with new benchmarks**
- [ ] **Step 4: Update testing strategies with fuzzing tests**
- [ ] **Step 5: Commit**

```bash
git add Website/user-manuals/
git commit -m "docs(manuals): update user manuals with new endpoints, scanning, monitoring, fuzzing"
```

### Task 8.2: Create New User Manuals

**Files:**
- Create: `Website/user-manuals/31-fuzz-testing-guide.md`
- Create: `Website/user-manuals/32-automated-security-scanning.md`
- Create: `Website/user-manuals/33-performance-optimization-guide.md`

- [ ] **Step 1: Write fuzz testing guide (installation, writing fuzz tests, running, interpreting)**
- [ ] **Step 2: Write automated security scanning guide (Snyk + SonarQube containerized workflow)**
- [ ] **Step 3: Write performance optimization guide (lazy loading, semaphores, benchmarking)**
- [ ] **Step 4: Commit**

```bash
git add Website/user-manuals/
git commit -m "docs(manuals): add fuzz testing, security scanning, and performance optimization guides"
```

### Task 8.3: Extend Video Courses

**Files:**
- Create: `Website/video-courses/video-course-62-router-completeness.md`
- Create: `Website/video-courses/video-course-63-automated-security-scanning.md`
- Create: `Website/video-courses/video-course-64-fuzz-testing-mastery.md`
- Create: `Website/video-courses/video-course-65-lazy-loading-patterns.md`
- Modify: `Website/video-courses/video-course-55-security-scanning-pipeline.md` (update with automated execution)

- [ ] **Step 1: Write new course outlines following existing format**

Each course needs: Course Overview, Duration, Level, Prerequisites, Learning Objectives, Modules (4-6), Hands-On Labs, Assessment, Resources.

- [ ] **Step 2: Update course 55 with automated Snyk/SonarQube content**
- [ ] **Step 3: Commit**

```bash
git add Website/video-courses/
git commit -m "docs(courses): add 4 new video courses and update security scanning course"
```

### Task 8.4: Update Architecture Diagrams

**Files:**
- Modify: `docs/diagrams/src/` (PlantUML source files)

- [ ] **Step 1: Update router diagram to show new handler connections**
- [ ] **Step 2: Update security scanning diagram with automated pipeline**
- [ ] **Step 3: Regenerate SVG/PNG outputs**
- [ ] **Step 4: Commit**

```bash
git add docs/diagrams/
git commit -m "docs(diagrams): update architecture diagrams with new routes and scanning pipeline"
```

### Task 8.5: Final Documentation Synchronization

**Files:**
- Modify: `CLAUDE.md` (add new endpoints, challenges, test types)
- Modify: `AGENTS.md` (sync with CLAUDE.md changes)
- Modify: `CONSTITUTION.md` (sync with any new rules)
- Modify: `docs/MODULES.md` (update module catalog)

- [ ] **Step 1: Add all new endpoints to CLAUDE.md Protocol Endpoints section**
- [ ] **Step 2: Add all new challenges to CLAUDE.md Challenges section**
- [ ] **Step 3: Add fuzzing test type to CLAUDE.md Testing section**
- [ ] **Step 4: Sync AGENTS.md and CONSTITUTION.md**
- [ ] **Step 5: Update docs/MODULES.md with any module changes**
- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md AGENTS.md CONSTITUTION.md docs/MODULES.md
git commit -m "docs: final synchronization of CLAUDE.md, AGENTS.md, CONSTITUTION.md, MODULES.md"
```

### Task 8.6: Website Content Update

**Files:**
- Modify: Website HTML/content pages

- [ ] **Step 1: Update feature listing pages with new API endpoints**
- [ ] **Step 2: Update documentation index with new manuals and courses**
- [ ] **Step 3: Rebuild website**

```bash
cd Website && bash build.sh
```

- [ ] **Step 4: Commit**

```bash
git add Website/
git commit -m "docs(website): update website content with all new features and documentation"
```

---

## Execution Order Summary

**Tier 1 (parallel, start immediately):**
- WS1: Tasks 1.1→1.8 (sequential within WS)
- WS2: Tasks 2.1→2.6 (sequential within WS)
- WS3: Tasks 3.1→3.10 (can parallelize 3.1-3.6 and 3.7-3.8)
- WS4: Tasks 4.1→4.3 (sequential within WS)

**Tier 2 (parallel, after respective Tier 1 dependency):**
- WS5: Tasks 5.1→5.5 (after WS1 complete)
- WS6: Tasks 6.1→6.3 (after WS1 complete)
- WS7: Tasks 7.1→7.2 (after WS2 complete)

**Tier 3 (after all Tier 2):**
- WS8: Tasks 8.1→8.6 (sequential within WS)

**Total: 8 workstreams, 38 tasks, ~261 individual steps**
