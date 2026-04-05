# Dead Code Audit (Phase 1)

**Date:** 2026-03-30
**Status:** Complete

## Overview

Phase 1 included a comprehensive dead code triage across the HelixAgent codebase. The goal was to identify unreferenced handlers, unused environment variables, and orphaned code paths, then either connect them to the system or remove them. Every decision is documented below with rationale.

---

## Handler Triage

### Connected (kept)

| Handler | File | Rationale |
|---------|------|-----------|
| `EnsembleHandler` | `internal/handlers/ensemble_handler.go` | Core ensemble orchestration endpoint. Registered in `internal/router/router.go` at `/v1/ensemble`. Called by all debate and multi-provider flows. |
| `CompletionHandler` | `internal/handlers/completion.go` | Primary OpenAI-compatible completions endpoint. Registered at `/v1/chat/completions` and `/v1/completions`. Used by all 48 CLI agents. |
| `ExtendedEnsembleHandler` | `internal/handlers/extended/ensemble.go` | Extended ensemble endpoint with team-based configuration. Registered at `/v1/ensemble/extended`. |

These handlers were flagged during the initial audit because their registration paths were indirect (via `setupEnsembleRoutes()` helper). After tracing the call graph they were confirmed as active.

### Deleted

| Handler | File | Rationale |
|---------|------|-----------|
| `DebateHandlerWithSkills` | (removed) | Was a prototype handler that combined debate orchestration with skill execution in a single endpoint. Functionality was split into the standard `DebateHandler` plus the `SkillsHandler`. No routes referenced it. |
| `ProtocolSSEHandler` | (removed) | Legacy SSE handler for protocol-level streaming. Replaced by the unified `StreamHandler` which supports SSE, WebSocket, and gRPC streaming through a single code path. The `protocol_sse.go` file was retained for the internal SSE helper functions but the exported handler was removed. |

### Decision Process

1. Searched for all exported handler types in `internal/handlers/`
2. Grepped for registration in `internal/router/router.go` and `internal/router/setup_*.go`
3. Checked for references in tests (some handlers only used in integration tests)
4. Handlers with zero references outside their own file and test were candidates for removal
5. Candidates were reviewed for planned features in `docs/plans/` before deletion

---

## Environment Variable Cleanup

### Removed (6 variables)

| Variable | Previous Purpose | Reason for Removal |
|----------|-----------------|-------------------|
| `LEGACY_DEBATE_MODE` | Forced use of pre-orchestrator debate logic | Legacy debate mode fully removed; orchestrator is the only path |
| `DISABLE_ENSEMBLE_CACHE` | Bypassed ensemble response caching | Cache is now required for performance; disabling it caused test failures |
| `OLD_PROVIDER_FORMAT` | Accepted deprecated provider config JSON | Migration to new format completed; no consumers remain |
| `SKIP_HEALTH_ON_BOOT` | Skipped health checks during startup | Health checks are mandatory per Constitution; skipping undermines safety |
| `DEBUG_PROVIDER_RESPONSES` | Logged full provider response bodies | Replaced by structured observability (OpenTelemetry spans with response metadata) |
| `FORMATTER_FALLBACK_DISABLED` | Disabled formatter fallback chain | Fallback is now unconditional; disabling created silent formatting failures |

### Aliased (3 variables)

| Old Name | New Name | Notes |
|----------|----------|-------|
| `CLAUDE_API_KEY` | `ANTHROPIC_API_KEY` | Both accepted; `CLAUDE_API_KEY` checked first for backward compatibility |
| `REDIS_URL` | `REDIS_HOST` + `REDIS_PORT` | `REDIS_URL` is parsed into host/port components at startup |
| `POSTGRES_DSN` | `DB_HOST` + `DB_PORT` + `DB_USER` + `DB_PASSWORD` + `DB_NAME` | `POSTGRES_DSN` is parsed into individual components |

### Kept (HelixMemory variables)

The following variables were flagged as potentially unused but were confirmed as active through the HelixMemory integration:

| Variable | Used By |
|----------|---------|
| `HELIXMEMORY_ENABLED` | `HelixMemory/` module activation gate |
| `HELIXMEMORY_MEM0_ENDPOINT` | Mem0 memory provider endpoint |
| `HELIXMEMORY_COGNEE_ENDPOINT` | Cognee knowledge graph endpoint |
| `HELIXMEMORY_LETTA_ENDPOINT` | Letta stateful agent endpoint |
| `HELIXMEMORY_GRAPHITI_ENDPOINT` | Graphiti temporal graph endpoint |
| `HELIXMEMORY_FUSION_MODE` | Controls 3-stage fusion pipeline behavior |

These are consumed by the HelixMemory module (`HelixMemory/pkg/config/`) and are required when HelixMemory is active (the default unless `-tags nohelixmemory` is used).

---

## Code Path Analysis

### Unreachable Switch Cases

Several `switch` statements had cases for provider types that no longer existed (e.g., `ProviderLegacyOpenAI`). These were removed along with the corresponding constants from `internal/models/enums.go`.

### Unused Helper Functions

| Function | File | Action |
|----------|------|--------|
| `formatLegacyDebatePrompt()` | `internal/services/debate_service.go` | Removed -- legacy debate mode deleted |
| `parseOldProviderConfig()` | `internal/config/config.go` | Removed -- `OLD_PROVIDER_FORMAT` env var deleted |
| `buildHealthSkipList()` | `internal/services/boot_manager.go` | Removed -- `SKIP_HEALTH_ON_BOOT` env var deleted |

---

## Verification

- **Challenge script:** `./challenges/scripts/dead_code_elimination_challenge.sh` (15 tests)
- **Build verification:** `make build` succeeds with no unused import or variable warnings
- **Test verification:** Full test suite passes (`make test-unit`)
- **Grep verification:** No remaining references to deleted symbols in production code

## Cross-References

- Constitution rule: "No Dead Code" (Priority 1)
- Related: `docs/development/RELEASE_CHECKLIST.md` (pre-release dead code scan)
- Related: `docs/fixes/` directory for individual fix reports
