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

## Next steps

1. Restart binary to activate cooldown + amber runtime fix.
2. Wait 1 h, re-run this report's "Verification of code fixes" grep.
3. Address Stage-3 carryover findings (#20-#27) per user priority.
