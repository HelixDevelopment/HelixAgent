# Session 2026-04-24 — evening — .env loading root-cause investigation

Companion to `SESSION_2026-04-24.md` + `SESSION_2026-04-24_late.md`.

## What was investigated

User asserted "Provider API keys shall be valid, all of it!" after seeing 17 of
25 providers returning 401 during the full-challenge-sweep. Investigation
separated three distinct issues:

1. **godotenv.Load ignored existing-but-empty shell env vars** → switched to Overload.
2. **.env uses `$ApiKey_Cerebras` (no braces) and godotenv's bare-$VAR parser
   is non-greedy on mixed-case identifiers** → parsed as `$A` + literal
   `piKey_Cerebras` → 14-char garbage sent as Bearer token → 401.
3. **Debate orchestrator hangs on chat completions** → separate bug, Issue #43.

## How Issue #42 was cracked

Added a diagnostic log in `cerebras.HealthCheck` that emitted
`sha256(p.apiKey)[:12] + len(p.apiKey)` + (when short) `p.apiKey[:5]` at the
exact moment the `Authorization: Bearer …` header was set. Compared the hash
against `/proc/<pid>/environ`'s CEREBRAS_API_KEY.

| Measurement | Value |
|---|---|
| /proc/<pid>/environ sha12 | `25725bc03015` (52 chars, real `csk-` key) |
| Helixagent in-process sha12 | `2a6e80911324` (14 chars, prefix `piKey`) |

The `piKey` prefix revealed partial expansion of `$ApiKey_Cerebras`. Fix
applied in `cmd/helixagent/main.go` — two-pass env load: first godotenv, then
a second pass that re-reads the raw file and applies `os.ExpandEnv` (Go's
greedy expander) to every value containing `$`.

## Results

Same session, fresh boot, identical keys, nothing else changed:

| Metric | Before | After |
|---|---|---|
| Process env CEREBRAS_API_KEY byte-matches .env.bak ApiKey_Cerebras | no | **yes** |
| cerebras.HealthCheck sha12 == process env sha12 | no (14 vs 52) | **yes (both 25725bc03015)** |
| Cerebras API response | 401 "Wrong API Key" | **HTTP 200** |
| Healthy providers | 8/25 | **14/24** |
| Providers flipped ✗ → ✓ | — | cerebras, chutes, groq, hyperbolic, mistral, zai |

## Commits landed

Main `HelixAgent`, pushed to github / githubhelixdevelopment / gitlab / upstream:

```
c5f85c52 fix(env): second-pass ExpandEnv so bare $VAR refs in .env actually resolve
f570bd85 fix(env): godotenv.Overload + .env.bak chain so API keys actually load
75116486 fix(demos): runtime-agnostic docker/podman + skip-on-missing-deps for pkg/api
3741b8bc fix(challenges): modernize 185 scripts' HELIXAGENT_PORT default 7061 → 8100
```

## Residual state

- **10 providers still unhealthy** — triaged in `docs/issues/fixed/BUGFIXES.md` "Direct-auth status" table.
  Of those: COHERE (rate-limited — will self-heal), FIREWORKS (412 format issue),
  GITHUB-MODELS (PAT scope), DEEPSEEK (helixagent-URL-mismatch), SILICONFLOW (OpenRouter
  routing), plus 4 genuinely-expired keys (REPLICATE, KIMI, UPSTAGE, CLOUDFLARE) that need
  operator rotation.

- **Issue #43 (OPEN)** — chat completions hang 30-90s and return
  `"Comprehensive debate completed with 0 rounds"` placeholder response.
  NEW orchestrator times out; DebateService fallback runs 0 rounds with
  quality_score=0.0 failing the 0.85 gate. Needs its own investigation
  (timeout tuning + quality-gate interaction).

## Honest confidence at end of this chapter

What's now demonstrated:
- Env loading is correct end-to-end (hash match verified).
- 14 providers pass their startup health check with real API calls.
- Direct API calls using the loaded keys return 200.

What's still NOT working:
- `/v1/chat/completions` via the debate orchestrator — hangs 30-90s, returns
  placeholder. Blocks the full challenge sweep from completing meaningfully
  against the real system.

What's expected to keep drifting:
- Operator key rotation for the 4 genuinely-expired keys.
- Rate-limited keys (Cohere) — self-heal as the quota window resets.
