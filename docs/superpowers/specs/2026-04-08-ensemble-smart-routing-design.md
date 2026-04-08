# Ensemble Smart Routing: Debate for Decisions, Direct for Actions

**Date:** 2026-04-08
**Status:** Approved

## Problem

The AI Debate Ensemble routes ALL requests through multi-round debate, including simple tool-calling requests from OpenCode. Debate participants (cloud LLMs) can't execute tools locally — they respond "I can't see your codebase" instead of calling `read_file`/`list_directory`. OpenCode expects the model to return `tool_calls` which it then executes locally.

## Solution

Two-layer routing that sends tool-calling/simple requests to a direct provider (which returns proper `tool_calls`) and complex development requests to the debate ensemble.

## Architecture

### Request Routing (2 layers)

```
Request arrives at /v1/chat/completions
  │
  ├─ Layer 1 (deterministic): tools[] present in request?
  │   YES → DIRECT provider path
  │   NO  → Layer 2
  │
  ├─ Layer 2 (LLM-based): EnhancedIntentClassifier
  │   conversation/question/analysis → DIRECT provider
  │   creation/debugging/fixing/refactoring → DEBATE ensemble
  │   classification fails → DEBATE ensemble (safe default)
  │
  └─ Routes to either DIRECT or DEBATE
```

### Direct Provider Selection

```
USE_HELIX_LLM=true:
  1. Try HelixLLM (thinker.local:8443)
  2. On failure (400/timeout/garbled) → fallback to highest-scored cloud provider

USE_HELIX_LLM=false:
  1. Use highest-scored verified provider from LLMsVerifier
```

### Direct Provider Path

- Send full request WITH tools to selected provider
- Provider returns `tool_calls` → pass through to OpenCode (OpenCode executes locally)
- Provider returns content → pass through directly
- Streaming supported via CompleteStream
- No tool execution by HelixAgent — stateless per request

### Debate Path (unchanged)

- Standard multi-round debate for complex development requests
- Tools passed to participants (future: participants may use tools)
- Multi-perspective consensus for architectural/design decisions

## Files Changed

| File | Change |
|------|--------|
| `internal/handlers/openai_compatible.go` | Add routing logic in `processWithEnsemble()` — check tools presence, classify intent, route to direct or debate |
| `internal/services/debate_service.go` | No changes (debate path unchanged) |
| `challenges/scripts/cli_agent_ensemble_routing_challenge.sh` | New: routing combination tests × 48 agents |
| `challenges/scripts/cli_agent_tool_execution_challenge.sh` | New: tool calling end-to-end × 48 agents |

## Testing Matrix

| Scenario | Expected Route | Validation |
|----------|---------------|------------|
| Request WITH tools | Direct provider | Returns tool_calls, not debate |
| "Hello!" (no tools) | Direct (conversation) | Clean response, fast |
| "Refactor the auth module" (no tools) | Debate | Multi-perspective response |
| Tool request + HelixLLM failure | Fallback to cloud | No 400, clean response |
| 3-message conversation with tools | Direct, all messages | No errors, no garbled output |
| Classification failure | Debate (safe default) | Graceful degradation |
| All 48 CLI agents × each scenario | Correct routing | Zero false positives |

## Key Constraints

- **No tool execution by HelixAgent** — OpenCode handles tool execution loop
- **No infinite loops** — Stateless per request, OpenCode manages conversation
- **HelixLLM local-first** — When enabled, always try local before cloud
- **Debate preserved** — Complex requests still get multi-model consensus
- **Zero hardcoded patterns** — Layer 2 uses LLM-based classification
