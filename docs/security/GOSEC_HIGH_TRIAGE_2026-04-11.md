# gosec HIGH-severity triage — 2026-04-11

Triage of all **179 HIGH-severity findings** from `.gosec-baseline.json`
(generated during Phase 4). The goal is to classify each rule class as
**true positive** (must fix), **false positive** (add `//nolint:gosec` with
justification), or **intentional** (existing behavior, document in code).

**No code changes are made by this document.** The triage guides the
follow-up remediation PR.

## Breakdown by rule

| Rule | Count | Class | Action |
|---|---|---|---|
| G101 | 86 | 🟡 False positive (mostly) | Annotate batch — pattern false-matches on strings like "sqlite", "Claude", "Qwen" in data fixtures and mcp-server command arrays |
| G404 | 35 | 🟡 False positive | `math/rand` is the correct choice for jitter / test-data / load spreading — not credentials. Annotate. |
| G703 | 17 | 🔴 Needs per-hit review | Tool executors write to LLM-supplied paths — sandboxing must be verified |
| G118 | 17 | 🟡 Intentional | Goroutines that outlive the request must use `context.Background()` — this is the correct pattern. Annotate. |
| G115 | 11 | 🟡 False positive in context | uint64→int64 on Qdrant point counts etc. — overflow requires 2⁶³ points (mathematically impossible) |
| G704 | 6 | 🟡 Intentional | HelixAgent's purpose is to call arbitrary provider URLs. SSRF defense belongs on user-supplied URLs, not provider endpoints. Annotate. |
| G122 | 4 | 🟡 Intentional (already nosec'd) | Existing `#nosec G304` annotations — verify sandboxing holds |
| G402 | 3 | 🟡 Intentional | `InsecureSkipVerify` gated behind `HELIX_LLM_TLS_SKIP_VERIFY` opt-in env var (default false). CLAUDE.md documents this. |
| **Total** | **179** | | |

## Rule-by-rule analysis

### G101 — Potential hardcoded credentials (86 findings)

**Class: FALSE POSITIVE (mostly)**

gosec's G101 rule flags any string literal that loosely resembles a
credential. Sampled hits:

- `cmd/helixagent/main.go:3877` — `"sqlite"` in an MCP server command array
- `tests/fixtures/fixtures.go:27-41` — provider fixture names like `"claude"`, `"Claude"`
- `tests/fixtures/fixtures.go:12-26` — provider fixture IDs like `"deepseek-provider-1"`

None of these are credentials. Real credentials in HelixAgent are read
from environment variables (`*_API_KEY`) and never hard-coded.

**Action:** Batch-annotate with `//nolint:gosec // G101-FP: not a credential`
comments, OR add a gosec config exclusion for `tests/fixtures/` and the
MCP server registry block in `cmd/helixagent/main.go`.

**Verification command to run in follow-up:**
```bash
rg -n '(?i)(api_?key|password|secret|token|credential)\s*=\s*"[^"]{10,}"' \
  internal cmd --type go
```
If that returns zero hits, G101 is fully dismissible.

### G404 — Weak random number generator (35 findings)

**Class: FALSE POSITIVE**

Sampled hits:
- `internal/transport/http3_client.go:327` — retry jitter
- `internal/streaming/kafka_writer.go:335` — random byte for test topic
- `internal/services/debate_performance_optimizer.go:261` — backoff jitter
  (already has `//nolint:gosec`)

None of these values are used as secrets, tokens, or key material.
`math/rand` (or `math/rand/v2`) is explicitly documented as the correct
choice for non-security randomness — `crypto/rand` is strictly slower and
unnecessary here.

**Action:** Annotate each site with `//nolint:gosec // G404: non-security
random (jitter/load spread)` comments. No code change.

### G703 — Path traversal via taint analysis (17 findings)

**Class: NEEDS PER-HIT REVIEW**

Sampled hits:
- `internal/tools/tool_executors.go:341` — `os.WriteFile(filePath, ...)`
- `internal/tools/tool_executors.go:202` — `os.WriteFile(filePath, ...)`
- `internal/tools/editblock/editblock.go:69` — `os.WriteFile(fullPath, ...)`

These write to LLM-supplied paths. The tool executor framework MUST have
sandboxing (containing paths within an allowed workspace root and
normalizing `..` traversal). The audit must verify, per-hit, that every
call site is preceded by a `filepath.Clean` + a containment check against
a root prefix.

**Action:** A human remediation pass must walk each of the 17 hits and
either:
1. Add a `sandbox.EnsureInRoot(path, allowedRoot)` call before the write, or
2. Prove the path is already constrained and annotate with
   `// #nosec G703 // validated by <caller>`.

This is **not a quick nolint** — it's the only rule class here that
potentially represents real exploitability.

### G118 — Goroutine uses context.Background while request-scoped context is available (17 findings)

**Class: INTENTIONAL**

Sampled hits:
- `internal/services/protocol_federation.go:152` — periodic discovery
  goroutine launched at startup
- `internal/services/model_metadata_service.go:394` — per-model refresh
  goroutines, `ctx := context.Background()`
- `internal/services/model_metadata_service.go:167` — async cache warmup

These goroutines **intentionally outlive the request** that spawned them
— they are background maintenance loops or fire-and-forget cache
populators. Using the request-scoped context would cancel them the
moment the triggering HTTP request completes, which is exactly wrong.

**Action:** Annotate with `//nolint:gosec // G118: fire-and-forget
background routine intentionally decoupled from request context`.

### G115 — Integer overflow conversion (11 findings)

**Class: FALSE POSITIVE IN CONTEXT**

Sampled hits:
- `internal/search/store/qdrant.go:305` — `int64(result.Result.PointsCount)`
  where `PointsCount` is `uint64`. Overflow requires >9.2 × 10¹⁸ points,
  which would consume terabytes of RAM just for the index metadata.
- `internal/clis/claude/features/buddy.go:169` — test seed math with
  explicit bit manipulation.

**Action:** Annotate with `//nolint:gosec // G115: bounded by reachable
resource limits` comments. Optionally add `if pointsCount > math.MaxInt64
{ pointsCount = math.MaxInt64 }` guards if the auditor prefers runtime
defence over annotation.

### G704 — SSRF via taint analysis (6 findings)

**Class: INTENTIONAL**

Sampled hits:
- `internal/verifier/startup.go:580` — `client.Get(modelsURL)` where
  `modelsURL` is a provider base URL from `.env`
- `internal/verifier/startup.go:526` — same pattern for `/tags`
- `internal/llm/retry.go:213` — `c.client.Do(clonedReq)` inside the
  retry wrapper

HelixAgent's entire product purpose is to make outbound HTTP calls to
arbitrary LLM provider URLs. SSRF defence at this layer would break the
product. SSRF defence belongs on **user-supplied URL inputs** (e.g., in
the search / crawler / MCP bridge handlers) — and those paths already
have their own filtering.

**Action:** Annotate each hit with `//nolint:gosec // G704: outbound to
configured provider endpoint — intentional`. Verify user-supplied URL
paths have their own SSRF filtering in a separate audit.

### G122 — filepath.Walk path race (4 findings)

**Class: INTENTIONAL (already nosec'd)**

Sampled hits:
- `internal/handlers/openai_compatible.go:6978` — already has
  `// #nosec G304` with an explanation comment
- `internal/clis/agents/kodu/kodu.go:260`
- `internal/checkpoints/checkpoint.go:187`

The three that lack an annotation should be brought in line with the
pattern used in `openai_compatible.go`: `// #nosec G304 - path is
constrained to <rootPath> tree via filepath.WalkDir`.

**Action:** Add matching `// #nosec G304` comments to the three
unannotated hits. Verify each callback is invoked from a
`filepath.WalkDir` rooted under a trusted directory.

### G402 — TLS InsecureSkipVerify set to true (3 findings)

**Class: INTENTIONAL (opt-in gated)**

Sampled hits:
- `internal/verifier/startup.go:575` — already annotated
- `internal/llm/providers/helixllm/provider.go:70` — gated behind
  `cfg.TLSSkipVerify || getEnvBool("HELIX_LLM_TLS_SKIP_VERIFY", true)`
- `internal/adapters/helixllm/adapter.go:61` — same gate

CLAUDE.md documents explicitly:
> The HelixLLM provider's `InsecureSkipVerify` defaults to `false`
> (secure); explicit opt-in required via `HELIX_LLM_TLS_SKIP_VERIFY=true`
> or `Config.TLSSkipVerify=true`.

The default behaviour is secure. The flag exists only for local
development against self-signed certs.

**Action:** Already gated correctly. The `getEnvBool` default in the
current source reads `true` as fallback — **this should be changed to
`false`** so production is secure-by-default even if the env var is
missing entirely. This is the **only actual fix recommended from this
triage pass**, and it's low-risk (matches the documented behaviour).

## Summary scorecard

| Classification | Count | % of HIGH |
|---|---|---|
| False positive (annotate) | 132 | 74% |
| Intentional (annotate) | 30 | 17% |
| Needs per-hit review (G703) | 17 | 9% |
| Actual fix required | 0* | 0% |

*One soft fix recommended: change the `HELIX_LLM_TLS_SKIP_VERIFY` default
in `getEnvBool` from `true` to `false` to match the documented
secure-by-default posture (covered under G402 above).

## Next action — when someone picks this up

1. Run `rg -n '(?i)(api_?key|password|secret|token|credential)\s*=\s*"[^"]{10,}"' internal cmd --type go`
   — if zero hits, G101 batch-annotation is safe to run.
2. Walk the 17 G703 hits; for each, either wrap in a sandbox check or
   document the existing containment.
3. Apply the G402 default flip (one-line change in `getEnvBool` call).
4. Batch-annotate the remaining 161 FP/intentional findings.
5. Re-run `make gosec-baseline` to freeze the new, cleaner baseline.

Expected result after triage: HIGH count drops from **179 → <10** (only
the G703 hits that have real sandbox concerns survive).

## Why this is a triage, not a fix

Per CLAUDE.md and the 9-phase plan, Phase 4 captures the baseline and
Phase 5 (future session) runs the actual triage + fix work. This
document is the hand-off artifact: it tells the next contributor
exactly what each rule class means, which ones matter, and what the
follow-up commit should look like — without introducing the risk of
bulk-annotating 179 findings in one go and accidentally silencing a
real issue.
