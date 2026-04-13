# HelixLLM Multi-Provider Fallback Chain Design

**Date:** 2026-04-13
**Status:** Draft
**Scope:** HelixLLM submodule — switch from local-only llama.cpp to multi-provider fallback with free cloud models as primary

## Problem

HelixLLM currently uses a local Qwen3-8B model on llama.cpp (CPU) as its primary provider. Response times are ~60s, and complex multi-step tasks (e.g., `/init` in OpenCode) time out. Free cloud providers (Chutes, OpenRouter, HuggingFace, Nvidia, Cerebras, SambaNova, Together) offer faster, stronger models at no cost within rate limits.

## Solution

Add a FallbackChain layer between Gateway and Brain that:
1. Auto-discovers free models from all providers with API keys
2. Scores and ranks them via LLMsVerifier (hybrid: import packages at startup + poll API for live updates)
3. Routes requests through the ranked chain with reactive (429 failover) + proactive (rate limit header parsing) rotation
4. Keeps llama.cpp as the guaranteed last-resort fallback
5. Syncs persistent memories to HelixAgent's HelixMemory module

All existing infrastructure (RAG hook, ToolManager, RequestOrchestrator, sliding window, tool sanitization) remains untouched.

## Decisions

| Question | Decision |
|---|---|
| LLMsVerifier integration | Hybrid — import scoring packages at startup, poll HTTP API for live updates (5m interval) |
| Model discovery | Auto-discover all free models from all providers with API keys in .env |
| Rate limit handling | Reactive (immediate failover on 429) + proactive (parse rate limit headers, skip providers approaching limits) |
| HelixMemory integration | Adapter pattern — session memory stays in HelixLLM's MemoryManager, persistent memories sync to HelixMemory |
| Testing scope | Phased — this spec covers fallback chain + comprehensive tests + challenges + core docs. CLI agent tests and broader docs deferred |
| Architecture | FallbackChain layer between Gateway and Brain (Approach B) |

## Architecture

### Request Flow

```
OpenCode CLI Agent
    |
    v
HelixAgent /v1/chat/completions (smart routing)
    |
    v
HelixLLM Gateway (auth -> RAG hook -> ToolManager -> RequestOrchestrator)
    |
    v
FallbackChain.CompleteStream(req)
    |
    +-- Check rate limits: skip providers where ShouldSkip() == true
    +-- Check circuit breakers: skip providers where circuit is open
    |
    v
    Entry #1: openrouter/deepseek-chat-v3:free (score=87.3)
    |   +-- SUCCESS -> parse rate limit headers -> return response
    |   +-- 429 -> mark exhausted, set cooldown -> advance
    |
    v
    Entry #2: chutes/Qwen3-235B-A22B (score=82.1)
    |   +-- SUCCESS -> parse rate limit headers -> return response
    |   +-- 5xx -> circuit breaker increment -> advance
    |
    v
    Entry #N (last): llamacpp/qwen2.5-coder-3b (score=local, always present)
    |   +-- SUCCESS or FINAL ERROR
    |
    v
Response -> Gateway -> HelixAgent -> OpenCode CLI Agent
```

### Key Invariants

1. LlamaCpp is always last -- guaranteed local fallback even if all cloud providers are down
2. A single request tries at most `len(entries)` providers -- no infinite retry loops
3. Rate limit headers are parsed from every response, even successful ones
4. The request itself is never modified between retries -- same `InternalChatRequest` goes to each provider
5. Streaming requests get the same fallback treatment -- if a stream fails mid-response, the chain does NOT retry (partial responses cannot be unsent); only pre-stream failures trigger fallback
6. All existing infrastructure (RAG hook, ToolManager, Orchestrator, sliding window, tool sanitization) remains untouched -- FallbackChain sits below Gateway, above Brain

## Section 1: New Provider Implementations

Seven new providers in `internal/brain/`, all implementing the existing `Provider` interface:

```go
type Provider interface {
    Complete(ctx context.Context, req *types.InternalChatRequest) (*types.InternalChatResponse, error)
    CompleteStream(ctx context.Context, req *types.InternalChatRequest) (<-chan types.StreamChunk, error)
    Models() []string
    Name() string
    Available() bool
}
```

### Provider Table

| Provider | File | Base URL | Auth Header | Model Discovery |
|---|---|---|---|---|
| Chutes | `chutes_provider.go` | `https://llm.chutes.ai/v1` | `Bearer` + `HELIX_LLM_CHUTES_KEY` | `GET /v1/models` |
| OpenRouter | `openrouter_provider.go` | `https://openrouter.ai/api/v1` | `Bearer` + `HELIX_LLM_OPENROUTER_KEY` | `GET /v1/models`, filter `:free` suffix |
| HuggingFace | `huggingface_provider.go` | `https://router.huggingface.co/v1` | `Bearer` + `HELIX_LLM_HUGGINGFACE_KEY` | `GET /v1/models`, filter by `pipeline_tag: text-generation` and free inference status |
| Nvidia NIM | `nvidia_provider.go` | `https://integrate.api.nvidia.com/v1` | `Bearer` + `HELIX_LLM_NVIDIA_KEY` | `GET /v1/models` |
| Cerebras | `cerebras_provider.go` | `https://api.cerebras.ai/v1` | `Bearer` + `HELIX_LLM_CEREBRAS_KEY` | `GET /v1/models` |
| SambaNova | `sambanova_provider.go` | `https://api.sambanova.ai/v1` | `Bearer` + `HELIX_LLM_SAMBANOVA_KEY` | `GET /v1/models` |
| Together | `together_provider.go` | `https://api.together.xyz/v1` | `Bearer` + `HELIX_LLM_TOGETHER_KEY` | `GET /v1/models` |

### Shared Base: OpenAICompatibleProvider

All seven providers are OpenAI-compatible (POST `/v1/chat/completions` with JSON body). A shared `OpenAICompatibleProvider` base in `openai_compat_provider.go` handles:

- HTTP client setup with TLS, 5-minute timeout
- Request marshaling to OpenAI format
- Response parsing from OpenAI format
- Auth header injection (configurable: Bearer, x-api-key, custom)
- Model discovery via `GET /v1/models` with provider-specific filtering
- Rate limit header extraction from responses

Each concrete provider embeds `OpenAICompatibleProvider` and only specifies: name, base URL, auth header format, and model-discovery filter logic (e.g., OpenRouter filters for `:free` suffix).

### Registration

In `Brain.New()`, each provider registers if its config key is non-empty (same pattern as existing OpenAI/Anthropic). The `Config` struct gets 7 new key fields:

```go
type Config struct {
    // ... existing fields ...
    ChutesKey      string // HELIX_LLM_CHUTES_KEY
    OpenRouterKey  string // HELIX_LLM_OPENROUTER_KEY
    HuggingFaceKey string // HELIX_LLM_HUGGINGFACE_KEY
    NvidiaKey      string // HELIX_LLM_NVIDIA_KEY
    CerebrasKey    string // HELIX_LLM_CEREBRAS_KEY
    SambaNovaKey   string // HELIX_LLM_SAMBANOVA_KEY
    TogetherKey    string // HELIX_LLM_TOGETHER_KEY
}
```

## Section 2: FallbackChain Package

New package `internal/fallback/` sits between Gateway and Brain.

### Chain (orchestrator)

```go
type Chain struct {
    brain       *brain.Brain
    entries     []ChainEntry
    rateLimiter *RateLimitTracker
    scorer      *ScorerBridge
    mu          sync.RWMutex
}
```

- `Complete(ctx, req)` -- iterates entries in order. On success, returns. On 429/quota error, marks entry exhausted, advances to next. On other errors (5xx, timeout), skips with circuit breaker. LlamaCpp entry is always last.
- `CompleteStream(ctx, req)` -- same fallback logic for streaming.
- `Refresh()` -- called periodically, re-scores providers via LLMsVerifier, reorders entries (llama.cpp stays last).

### ChainEntry (provider wrapper with state)

```go
type ChainEntry struct {
    ProviderName   string
    ModelID        string
    Score          float64
    Status         EntryStatus       // active | exhausted | circuit_open
    RateLimit      *RateLimitState
    CooldownUntil  time.Time
    CircuitBreaker *CircuitBreaker
}
```

### Ordering Logic

At startup and on each `Refresh()`:
1. Collect all available providers from Brain
2. Score each via LLMsVerifier (model capability + cost + speed)
3. Sort descending by score
4. Filter out providers where `ShouldSkip()` is true (approaching rate limit)
5. Append llama.cpp at the end regardless of score

### Gateway Wiring

`HandleChatCompletions` receives `*fallback.Chain` instead of `*brain.Brain`. The Chain's `Complete`/`CompleteStream` methods match Brain's call pattern, so the change is a single type swap in the handler function parameter.

## Section 3: LLMsVerifier Integration (ScorerBridge)

New file `internal/fallback/scorer_bridge.go`. Hybrid approach -- imports LLMsVerifier Go packages at compile time for startup scoring, polls HTTP API for live updates.

### ScorerBridge Struct

```go
type ScorerBridge struct {
    verifierURL     string
    httpClient      *http.Client
    localScorer     *scoring.Scorer
    pricingDet      *pricing.PricingDetector
    refreshInterval time.Duration    // default 5m
    stopCh          chan struct{}
    wg              sync.WaitGroup
}
```

### Startup Flow (synchronous, blocks before serving)

1. Brain registers all providers with API keys
2. For each provider, call `Brain.Models()` to get available models
3. Import `llm-verifier/scoring` -- call `scoring.CalculateScore(model, provider)` for each model
4. Import `llm-verifier/enhanced/pricing` -- call `pricing.IsFree(model)` to filter free-only models
5. Build initial `[]ChainEntry` sorted by score, llama.cpp appended last
6. Log the ranked chain: `[1] openrouter/deepseek-chat-v3:free (score=87.3) [2] chutes/Qwen3-235B (score=82.1) ... [last] llamacpp/qwen2.5-coder-3b (score=local)`

### Live Refresh (background goroutine, every 5 minutes)

1. `GET {VERIFIER_API_URL}/api/models?free=true` -- fetch current scored models
2. `GET {VERIFIER_API_URL}/api/scores` -- fetch latest verification scores
3. Merge with local rate-limit state
4. Reorder chain entries under write lock
5. Log any rank changes: `provider X moved from #3 to #1`

### Graceful Degradation

If LLMsVerifier is unreachable at startup, fall back to a static scoring heuristic (prefer providers with known-strong free models: OpenRouter > Chutes > HuggingFace > Nvidia > others > llama.cpp). If live refresh fails, keep last-known scores and log a warning.

### Config

- `HELIX_LLM_VERIFIER_URL` -- defaults to `http://localhost:7061`
- `HELIX_LLM_SCORE_REFRESH_INTERVAL` -- defaults to `5m`

## Section 4: Rate Limit Tracking

New file `internal/fallback/rate_limit.go`.

### Reactive (on 429)

When a provider returns HTTP 429 or a body containing `"rate_limit"`, `"quota_exceeded"`, or `"tokens_exhausted"`:
1. Parse `retry-after` header -> set `CooldownUntil` to `now + retry-after`
2. If no `retry-after`, use exponential backoff: 60s -> 120s -> 240s (max 15 min), reset on success
3. Set `EntryStatus = exhausted`
4. Chain immediately advances to next entry and retries the same request
5. Log: `provider X rate limited, cooling down until Y, falling to provider Z`

### Proactive (header parsing)

After every successful response, parse rate limit headers:
- `x-ratelimit-remaining` / `x-ratelimit-remaining-tokens` / `x-ratelimit-remaining-requests`
- `x-ratelimit-reset` (Unix timestamp or seconds)
- Provider-specific variants (OpenRouter uses `x-ratelimit-remaining-credits`)

Store in `RateLimitState`:

```go
type RateLimitState struct {
    RemainingRequests int
    RemainingTokens   int
    ResetAt           time.Time
    LastUpdated       time.Time
}
```

`ShouldSkip()` returns true when: `RemainingRequests < 5` OR `RemainingTokens < 1000` (configurable thresholds via `HELIX_LLM_RATELIMIT_MIN_REQUESTS` and `HELIX_LLM_RATELIMIT_MIN_TOKENS`).

When proactively skipped, entry stays `active` (not `exhausted`) -- it is deprioritized in ordering, not removed.

### Cooldown Recovery

Background goroutine checks every 30s for entries where `CooldownUntil < now`. Resets those entries to `active`, resets backoff counter. Entry re-enters the chain at its score-based position.

### Circuit Breaker (non-rate-limit errors)

For 5xx responses, timeouts, and connection failures:
- 3 consecutive failures -> circuit opens for 2 minutes
- Half-open: allows 1 probe request, success -> close, failure -> reopen for 4 minutes
- Independent from rate limit tracking -- a provider can be rate-limited but circuit-healthy

### RateLimitTracker Struct

```go
type RateLimitTracker struct {
    states map[string]*RateLimitState
    mu     sync.RWMutex
}
```

## Section 5: HelixMemory Adapter

New file `internal/fallback/memory_adapter.go`. Lightweight bridge -- HelixLLM's existing 3-tier MemoryManager handles session memory, the adapter syncs important persistent memories to HelixAgent's HelixMemory service.

### What Gets Synced (write to HelixMemory)

- User preferences learned during sessions (e.g., coding style, language preferences)
- Cross-session learnings from the ReAct agent (patterns that worked, patterns that failed)
- Entity relationships discovered during RAG-augmented conversations
- Triggered by MemoryManager's episodic memory write -- if the memory is flagged `persistent: true`

### What Gets Recalled (read from HelixMemory)

- At session start, fetch user's persistent memories to seed the system prompt
- During RAG augmentation, include relevant HelixMemory entities alongside codebase context

### MemoryAdapter Struct

```go
type MemoryAdapter struct {
    helixMemoryURL string
    httpClient     *http.Client
    enabled        bool              // HELIX_LLM_MEMORY_SYNC_ENABLED
    syncInterval   time.Duration     // batch sync every 30s
    pending        []MemoryEntry
    mu             sync.Mutex
    stopCh         chan struct{}
    wg             sync.WaitGroup
}
```

### Integration Point

Hooks into HelixLLM's existing `MemoryManager.Store()` path. When a memory is stored with `persistent: true`, it is also added to the adapter's pending buffer. A background goroutine flushes the buffer every 30s via `POST /v1/memory/entities`.

### Graceful Degradation

If HelixMemory is unreachable, pending entries accumulate up to a 1000-entry cap (oldest dropped), adapter logs a warning, session continues normally. No request-path blocking.

### Config

- `HELIX_LLM_MEMORY_SYNC_ENABLED=true`
- `HELIX_LLM_MEMORY_URL=http://localhost:7061`

## Section 6: Testing & Challenges

Per Constitution: unit, integration, E2E, security, stress, benchmark tests + challenge scripts. All with resource limits (`GOMAXPROCS=2`, `nice -n 19`, `-p 1`).

### Unit Tests

| File | Tests |
|---|---|
| `internal/brain/openai_compat_provider_test.go` | HTTP request building, auth headers, model discovery parsing, error handling for each of 7 providers |
| `internal/fallback/chain_test.go` | Ordering logic, fallback on 429, fallback on 5xx, llama.cpp always last, all-exhausted behavior, concurrent access |
| `internal/fallback/rate_limit_test.go` | Header parsing (6+ formats), ShouldSkip thresholds, cooldown recovery, exponential backoff, proactive skip |
| `internal/fallback/scorer_bridge_test.go` | Startup scoring, live refresh, graceful degradation, reordering under lock |
| `internal/fallback/circuit_breaker_test.go` | Open/close/half-open transitions, probe success/failure, independence from rate limiting |
| `internal/fallback/memory_adapter_test.go` | Buffer batching, flush on interval, cap overflow, graceful degradation |

### Integration Tests (`tests/integration/`)

- `fallback_chain_integration_test.go` -- real HTTP calls to providers with API keys, verify actual model responses, verify fallback when one provider is down
- `scorer_bridge_integration_test.go` -- real LLMsVerifier scoring against live providers
- `memory_sync_integration_test.go` -- real HelixMemory write/read cycle

### E2E Tests (`tests/e2e/`)

- `multi_provider_e2e_test.go` -- full Gateway -> FallbackChain -> Brain -> Provider flow, verify response quality from free models
- `rate_limit_rotation_e2e_test.go` -- simulate rate limiting (mock 429 on primary), verify rotation to secondary

### Security Tests (`tests/security/`)

- API key not leaked in logs or error responses
- TLS verification on all provider connections
- Rate limit header injection resistance

### Stress Tests (`tests/stress/`)

- 100 concurrent requests through the fallback chain
- All providers rate-limited simultaneously -- verify llama.cpp fallback under load

### Benchmark Tests (`tests/benchmark/`)

- Latency comparison: free cloud vs. local llama.cpp
- Fallback chain overhead measurement (routing + scoring adds < 5ms)

### Challenge Scripts (`challenges/scripts/`)

- `multi_provider_fallback_challenge.sh` -- boots HelixLLM, verifies all free providers discovered, sends requests, verifies fallback rotation works, validates rate limit tracking
- `helixllm_memory_sync_challenge.sh` -- verifies HelixMemory adapter stores and retrieves persistent memories

## New Files Summary

All paths relative to `HelixLLM/`:

```
internal/brain/openai_compat_provider.go        # Shared OpenAI-compatible base
internal/brain/openai_compat_provider_test.go
internal/brain/chutes_provider.go                # Chutes provider
internal/brain/openrouter_provider.go            # OpenRouter provider (free filter)
internal/brain/huggingface_provider.go           # HuggingFace provider
internal/brain/nvidia_provider.go                # Nvidia NIM provider
internal/brain/cerebras_provider.go              # Cerebras provider
internal/brain/sambanova_provider.go             # SambaNova provider
internal/brain/together_provider.go              # Together provider
internal/fallback/chain.go                       # FallbackChain orchestrator
internal/fallback/chain_test.go
internal/fallback/chain_entry.go                 # ChainEntry type + EntryStatus
internal/fallback/rate_limit.go                  # RateLimitTracker + RateLimitState
internal/fallback/rate_limit_test.go
internal/fallback/circuit_breaker.go             # CircuitBreaker (open/close/half-open)
internal/fallback/circuit_breaker_test.go
internal/fallback/scorer_bridge.go               # LLMsVerifier hybrid integration
internal/fallback/scorer_bridge_test.go
internal/fallback/memory_adapter.go              # HelixMemory sync adapter
internal/fallback/memory_adapter_test.go
tests/integration/fallback_chain_integration_test.go
tests/integration/scorer_bridge_integration_test.go
tests/integration/memory_sync_integration_test.go
tests/e2e/multi_provider_e2e_test.go
tests/e2e/rate_limit_rotation_e2e_test.go
tests/security/fallback_security_test.go
tests/stress/fallback_stress_test.go
tests/benchmark/fallback_benchmark_test.go
challenges/scripts/multi_provider_fallback_challenge.sh
challenges/scripts/helixllm_memory_sync_challenge.sh
```

## Modified Files

```
internal/brain/brain.go          # Add 7 new provider registrations in New()
internal/brain/config.go         # Add 7 new key fields to Config
internal/shared/config/config.go # Load 7 new env vars
internal/gateway/openai.go       # Swap *brain.Brain param for *fallback.Chain
cmd/helixllm/main.go             # Initialize FallbackChain, wire between Brain and Gateway
.env.example                     # Add 7 new provider key env vars + fallback config vars
```

## Out of Scope (deferred to follow-up spec)

- Tests for all 50+ CLI agents against the new fallback chain
- Website content, video courses, step-by-step manuals
- Full diagram updates (architecture, sequence diagrams)
- Broader documentation updates beyond core README/CLAUDE.md changes
