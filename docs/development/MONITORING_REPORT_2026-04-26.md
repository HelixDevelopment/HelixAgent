# Live Monitoring Report — 2026-04-26

## Scope

Stage 4 of the CLI agent integration cycle: monitor `./bin/helixagent` (PID
629070, started 03:01 local), local + remote container fleet
(thinker.local, amber.local), and host resources for warnings/errors over
~10h of organic activity.

## Method

- Read `/tmp/helixagent-run/boot.log` (last 8000 lines, 1.25 MB total).
- Survey local containers (`podman ps`) and remote (`ssh thinker/amber.local
  podman ps`).
- Sample API end-to-end with `curl /v1/chat/completions`.
- Categorize errors / warnings by frequency.
- Apply systematic-debugging Phase 1 to each category before proposing fixes.

## Baseline numbers

- Binary RSS: 105 MB.
- Host load 1m: 0.86 (62 GB / 5.5 GB used).
- API: `/health` returns `healthy`; `/v1/models` lists `helixagent-debate`
  and `helixagent-ensemble`; live chat completion returns content in ~5 s.
- No panics, no fatals, no crashes.

## Findings & dispositions

### Code-fixable

| # | Finding | Disposition | Commit |
|---|---|---|---|
| 26 | Crush registry path `~/.config/crush/config.json` (CLI didn't read it) → must be `crush.json` | **FIXED** in `internal/agents/registry.go` | `18d7af2e` |
| 28 | Intent classifier hammers Cerebras after 429/quota; 166× warnings + 42× spec-auto-activation cousin | **FIXED** with per-provider 5-min cooldown in `llm_intent_classifier.go` + 2 new unit tests | `bb64dcd3` |
| 32 | Orchestrator logs "SUCCEEDED" before empty-content check, then logs "returned empty content" 24× | **FIXED** by reordering log statements; SUCCEEDED only fires when content non-empty + carries `content_len` field | `6af55414` |
| 38 | `Containers/.env` declares amber.local as `podman` runtime but amber has `/usr/bin/docker` | **FIXED** in `Containers/.env` (gitignored, no commit needed) | local |

### Operator / credential action

These are NOT code bugs — they need credential rotation by the user.

| # | Finding | Action |
|---|---|---|
| 36 | Stale 401/403 API keys: `groq`, `github-models`, `replicate`, `upstage`, `codestral` | Rotate keys or unset `*_API_KEY` env vars to suppress these providers |
| 33 | Providers with all-models-rejected (no verified models): `baseten`, `junie`, `cloudflare`, `inference`, `huggingface` | Mix of #36 + verification budget; rotate keys first, then revisit |
| 40 | 25× "Provider has failed 3 consecutive health checks" (cohere 429, fireworks 412, cloudflare 400) | Downstream of #36 + tier-limited free quotas. Will quiet itself after key rotation + classifier cooldown reduces hot-loop traffic. |

### Architectural follow-ups

| # | Finding | Recommendation |
|---|---|---|
| 30 | "[CODE PATH] NEW orchestrator FAILED, trying debate service" 48× — all "context deadline exceeded" | Mostly downstream of #28; expect to taper after restart with cooldown active. Re-evaluate after 1h post-restart. If still > 5/h, port the cooldown pattern into the orchestrator's per-participant call path. |
| 34 | "Model verification budget exhausted" 12× — 30 s `VerificationTimeout` too tight for providers with 6-7 models | Acceptable trade-off (the warning IS the call to action). Operators can raise per-provider via env vars. |
| 35 | "All 0 fallbacks failed" — empty fallback list should not be reachable | Trace originating call path; likely a config issue where a model has no scored alternatives. Low priority — only 8 events in 10h. |

### Stage-3 carryover (CLI agent integration findings, prior session)

Tracked here for completeness, not re-investigated in this monitoring window.

| # | Finding | Status |
|---|---|---|
| 20 | Add Anthropic-compatible `/v1/messages` endpoint for Claude CLI | Open |
| 21 | Add Google `generateContent` endpoint for Gemini CLI | Open |
| 22 | Document Qwen OAuth bypass for `--openai-base-url` | Open |
| 23 | Document Crush requires TTY (use `crush run`) | Open |
| 24 | Fix `helixcode` config generator to include JWT_SECRET | Patched in user config; generator still misses it |
| 25 | Document Copilot doesn't honor base URL (GitHub-managed auth) | Open |
| 27 | Wrapper scripts for protocol-translating agents | Open |
| 39 | Local container fleet empty (everything on thinker.local) | Verify if intentional vs. distributor bug |

## Verification of code fixes

Fix takes effect on next binary restart. Pre-restart baselines:
- 166 LLM intent classification failures in 10h (~17/h)
- 24 orchestrator empty-content events in 10h (~2.4/h)

Post-restart success criterion (1h sample):
- Intent classification failures: < 5/h (≥ 95% reduction; provider hits 429
  once per 5 min instead of every chat call)
- Orchestrator log lines: SUCCEEDED only paired with non-empty content
  (verifiable by grep: zero "SUCCEEDED" lines followed by "returned empty content")

## Container fleet snapshot

- **localhost**: 0 containers running (Containers/.env distribution → remote
  hosts)
- **thinker.local**: postgres + redis + chromadb (3 containers, all healthy,
  17h uptime)
- **amber.local**: 0 containers (was blocked by runtime mismatch — Finding
  #38, fixed; will populate on next binary restart)

## Findings discovered during restart (13:01-13:06)

| # | Finding | Disposition |
|---|---|---|
| 41 | HTTP shutdown deadline too tight; binary exited `level=fatal` instead of clean (forcing a `context deadline exceeded` on `HTTP server shutdown`) during graceful SIGTERM | Open. Increase shutdown grace period in `cmd/helixagent/main.go`. Low-priority cosmetic. |
| 42 | Intent classifier sends full user message (9635 chars observed) to Cerebras → `context_length_exceeded` (400). Cooldown introduced in #28 only handles 429s; this is a 400. | **FIXED** — `maxClassifierUserMessageChars=4000` in `llm_intent_classifier.go:buildIntentClassificationPrompt` + truncation marker + 1 unit test. Commit `408c9c15`. |
| 43 | **USER-VISIBLE PAIN**: binary takes ~2:18 to bind port 8100 because remote-distribution SCPs 12 build contexts to thinker.local sequentially on every restart, blocking server start. Liveness probe binds 8111 immediately, but CLI agents talk to 8100 and see "Cannot connect" with retry loop until distribution finishes. Reproduced 2026-04-26 13:02:46 → 13:06:03. | Open, **high priority**. Two complementary changes: (a) hash-skip in `internal/adapters/containers/adapter.go:copyBuildContexts` — read remote checksum, only copy on diff; (b) make `BootAll` non-blocking — start HTTP server first, run distribution async with status endpoint surfacing partial readiness. Today's early-bind 8111 probe was the first half of (b); now extend it to 8100. |

## Findings discovered during continued monitoring (13:07+)

| # | Finding | Disposition |
|---|---|---|
| 44 | `qwen-oauth` provider keeps invoking the qwen CLI every health-check cycle even though the CLI returns "Qwen OAuth free tier was discontinued on 2026-04-15" — an unrecoverable user-action-required error. | **FIXED** — sync.RWMutex-protected `terminalErr` + `isQwenTerminalAuthError` matcher in `qwen_cli.go`; HealthCheck short-circuits after first terminal failure. 2 new tests. Commit `fb95624b`. |
| 45 | `junie` provider keeps invoking the junie CLI every cycle despite "403 Forbidden: No active JetBrains AI subscription found." | **FIXED** — same sticky-disable pattern in `junie_cli.go`. 2 new tests. Commit `fb95624b`. |

## Stage-3 carryover progress (this monitoring turn)

| # | Finding | Disposition |
|---|---|---|
| 20 | Add Anthropic-compatible `/v1/messages` endpoint for Claude CLI | **FIXED (non-streaming MVP)** — `internal/handlers/anthropic_compatible.go` translates Anthropic↔OpenAI request/response, delegates to ChatCompletions in-process via capturing ResponseWriter. Streaming + tools return clear 400s for now. 6 tests. Commit `f6fdd26d`. |
| 21 | Add Google `generateContent` endpoint for Gemini CLI | Open. Same translator pattern as #20; lower priority because Gemini CLI bypasses `--openai-base-url` regardless. |
| 22 | Document Qwen OAuth bypass for `--openai-base-url` | **DOC** in `docs/cli-agents/14-protocol-mismatches.md`. Commit `047b0c87`. |
| 23 | Document Crush requires TTY (use `crush run`) | **DOC** in same file. Commit `047b0c87`. |
| 24 | Fix helixcode config generator to include JWT_SECRET | **FIXED** — generator now emits `auth.jwt_secret` from `JWT_SECRET` env (or `HELIXAGENT_JWT_SECRET` fallback). Verified end-to-end with real `--generate-agent-config=helixcode` invocation against rebuilt binary. LLMsVerifier commit `f0047ca0`, vendor refreshed, parent bump commit `9d9dec38`. |
| 25 | Document Copilot doesn't honor base URL | **DOC** in same file. Commit `047b0c87`. |
| 26 | Fix Crush registry path | **FIXED** in `internal/agents/registry.go`. Commit `18d7af2e`. |
| 27 | Wrapper scripts for protocol-translating agents | Possibly redundant with #20/#21 done — re-evaluate after #21 lands. |

## Next steps

1. ~~Restart binary to activate cooldown + amber runtime fix.~~ DONE (13:02:46 → 13:06:03 ready).
2. Restart binary again to activate `/v1/messages` route + the truncation, sticky-disable, and JWT-generator fixes. **Pending user OK** — current binary is in active use.
3. Address Finding #43 (slow boot — user-visible pain). Two-part architectural fix.
4. Optional: Finding #21 (Google generateContent translator) using the same pattern as #20.
5. Optional: Finding #41 (shutdown grace — cosmetic, ~5 min fix).
