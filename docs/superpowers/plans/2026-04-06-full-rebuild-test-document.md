# Full System Rebuild, Test, and Documentation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild all 7 apps, boot with remote container distribution, run all tests/challenges at 100% coverage, update all documentation.

**Architecture:** Build all binaries via `go build -mod=mod`, enable `CONTAINERS_REMOTE_ENABLED=true` in `Containers/.env`, boot via `./bin/helixagent` (auto-distributes to `thinker.local`), run complete test suite and all 525 challenges, regenerate all diagrams/docs.

**Tech Stack:** Go 1.25.3, Docker/Podman, PostgreSQL 15, Redis 7, PlantUML, Mermaid

---

### Task 1: Build All 7 Apps

**Files:**
- Modify: `Containers/.env` (enable remote)
- Build targets: `cmd/helixagent`, `cmd/api`, `cmd/grpc-server`, `cmd/cognee-mock`, `cmd/sanity-check`, `cmd/mcp-bridge`, `cmd/generate-constitution`

- [x] **Step 1:** `go mod vendor` to sync vendor directory
- [x] **Step 2:** `make build` for main helixagent binary
- [x] **Step 3:** Build remaining 6 apps individually
- [x] **Step 4:** Verify all 7 binaries exist in `bin/`

### Task 2: Enable Remote Distribution and Boot

- [x] **Step 1:** Set `CONTAINERS_REMOTE_ENABLED=true` in `Containers/.env`
- [x] **Step 2:** Run `./bin/helixagent` to boot and auto-distribute containers
- [x] **Step 3:** Wait for health checks on all services

### Task 3: Run All Tests (100% Coverage)

- [x] **Step 1:** `make test-complete` (all 6 test types with coverage)
- [x] **Step 2:** `make test-no-skip` (verify no unconditionally disabled tests)
- [x] **Step 3:** `make test-coverage-100` (enforce 100% gate)
- [x] **Step 4:** `make test-race` (race condition detection)
- [x] **Step 5:** `make test-bench` (benchmarks)

### Task 4: Run All 525 Challenges

- [x] **Step 1:** `./challenges/scripts/run_all_challenges.sh`

### Task 5: Update Documentation

- [x] **Step 1:** Regenerate diagrams: `./scripts/generate-diagrams.sh`
- [x] **Step 2:** Update API docs, user manuals, SQL schemas
- [x] **Step 3:** Update video course materials
