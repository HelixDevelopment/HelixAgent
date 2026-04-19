# CONST-029 Migration Resume Kit

**Date paused:** 2026-04-19
**Last commit:** `40aa4d73` on `main`
**Allowlist:** 242 of 254 remaining (12 drained, 4.7%)
**Status:** All 4 remotes in sync; audit gate green; playbook + primitives landed.

This document is a complete, standalone continuation kit for the CONST-029 Pattern-A drain campaign. Anyone (human or AI) can pick up the work with one command.

---

## 1. One-command resume

Paste this verbatim as the first user message in a new session:

```
Resume CONST-029 Pattern-A migration per
docs/superpowers/specs/2026-04-19-migration-resume-kit.md.

Run `./scripts/migration/status.sh` first to see state.
Run `./scripts/migration/next.sh` to pick the next site.
Drain the Tier-A queue in order, one commit per site.
After each successful commit: push to github, gitlab, githubhelixdevelopment.
Stop when 3 consecutive sites hit test-coupling blockers — report and ask
before tackling the Tier-C backlog.

Honor the full spec: docs/superpowers/specs/2026-04-19-pattern-a-migration-design.md
```

That's it. Two scripts, one approved spec, 242 sites.

---

## 2. State at time of pause

| Item | Value |
|---|---|
| Branch | `main` |
| HEAD | `40aa4d73` |
| Allowlist file | `scripts/concurrency-audit-allowlist.txt` |
| Allowlist entries remaining | 242 |
| Entries drained this campaign | 12 |
| Test-file mocks (deferred) | 26 of 242 |
| Known blockers flagged | 8 sites |
| Audit gate | `make ci-validate-concurrency` — green |
| Primitives home | `Concurrency/pkg/safe/{store,slice}.go` |
| Spec | `docs/superpowers/specs/2026-04-19-pattern-a-migration-design.md` |
| Playbook | `docs/development/concurrency-playbook.md` |
| Constitution | `CONST-029` synced to `CONSTITUTION.{md,json}`, `CLAUDE.md`, `AGENTS.md` |

**Migrations completed this campaign** (commits, newest first):

1. `40aa4d73` `EnsembleService`
2. `69f6e567` `DebateReportingService`
3. `d0b8c3e0` `ProtocolSecurity` + `RateLimiter` (two sites, one commit)
4. `7b669c82` `DebateResilienceService`
5. `aa261987` `MultiPassValidator`
6. `fa4e7b13` `DebatePerformanceService`
7. `513b73fa` `DebateMonitoringService`
8. `772c073f` `DebateHistoryService`
9. `cab120ac` `FallbackChainValidator`
10. `165933e5` `ToolRegistry`
11. `18c12546` `ProtocolCache`

Plus the architectural foundation:

- `faa0976e` primitives + audit gate + playbook
- `344e48b2` HelixAgent integration smoke test
- `4c0fb6b7` this campaign's spec
- `b41d8c03` prior-session overclaim correction

---

## 3. Migration patterns — distilled from 12 real migrations

These are the actual patterns that worked. Not speculative — every pattern below is grounded in a committed migration.

### Pattern Alpha — immutable-values Store (easiest, ~10–15 min)

**When:** values are set once at Put and never mutated post-insert. The map is the state.

**Examples shipped:** `DebateHistoryService`, `DebatePerformanceService`, `DebateReportingService` (reports).

**Recipe:**
```go
// Before
type X struct {
    mu      sync.RWMutex
    entries map[string]*Entry
}

// After
type X struct {
    entries *safe.Store[string, *Entry]
}

// Methods: direct Get/Put/Delete/Snapshot/Len/Keys/Values.
```

### Pattern Beta — mutable-pointer-values Store (medium, ~25–40 min)

**When:** values are pointers to structs whose fields are mutated after the pointer is in the map.

**Examples shipped:** `DebateMonitoringService`, `DebateResilienceService`, `ProtocolSecurity`.

**Recipe:** **all** access paths — reads AND writes — go through `Store.Update` callbacks. A naive `Get(key)` + dereference races with a concurrent `Update` mutating the same pointer's fields.

```go
// Mutation: Update callback mutates *V in place
s.sessions.Update(id, func(s *Session, ok bool) (*Session, bool) {
    if !ok { return nil, false }
    s.Status = "completed"
    s.LastUpdated = time.Now()
    return s, true
})

// Read that copies fields: ALSO under Update
func (x *X) GetStatus(id string) (*Status, error) {
    var result *Status
    var missing bool
    x.sessions.Update(id, func(s *Session, ok bool) (*Session, bool) {
        if !ok { missing = true; return nil, false }
        copy := s.Status
        result = &copy
        return s, true // keep
    })
    if missing { return nil, errNotFound }
    return result, nil
}
```

Race detector under `-race -count=3` will catch the naive pattern. It caught `DebateMonitoringService` mid-session. The fix is mechanical once you see the pattern.

### Pattern Gamma — scalar pointer → atomic.Pointer (trivial)

**When:** a single scalar pointer field was protected by mu alongside a collection.

**Examples shipped:** `FallbackChainValidator` (lastValidation), `MultiPassValidator` (config).

**Recipe:**
```go
// Before
mu             sync.RWMutex
lastValidation *Result

// After
lastValidation atomic.Pointer[Result]

// Write
v.lastValidation.Store(result)

// Read
if r := v.lastValidation.Load(); r != nil { ... }
```

### Pattern Delta — two independent stores (Tier 1 multi-collection)

**When:** two collections in the same struct but no method atomically touches both.

**Examples shipped:** `ProtocolCache` (cache, invalidators), `ToolRegistry` (tools, customTools), `DebateReportingService` (reports, templates), `ProtocolSecurity` (apiKeys, permissions).

**Recipe:** two `safe.Store`s, no outer mutex. Verify independence by enumerating methods — if no method reads BOTH under the same lock for consistency, they're independent.

### Pattern Epsilon — joint atomicity via state struct (Tier 3, not yet shipped)

**When:** multiple collections must update atomically (e.g., `TagBasedInvalidation.AddTag` writes to two maps under one lock).

**Recipe (per spec §6):**
```go
type innerState struct {
    tagIndex map[string]map[string]struct{}
    keyTags  map[string][]string
}

type X struct {
    state *safe.Store[string, innerState]  // constant key
}

const _stateKey = "_"

func (x *X) AddTag(key string, tags ...string) {
    x.state.Update(_stateKey, func(s innerState, _ bool) (innerState, bool) {
        s.keyTags[key] = append(s.keyTags[key], tags...)
        for _, tag := range tags {
            if s.tagIndex[tag] == nil { s.tagIndex[tag] = map[string]struct{}{} }
            s.tagIndex[tag][key] = struct{}{}
        }
        return s, true
    })
}
```

Store's write lock covers the entire callback, so mutation-in-place of the inner maps is safe.

### Pattern Zeta — scalar mutex survivor (audit-compatible)

**When:** a struct has sync.Mutex + an interface or a scalar pointer, NO collection. The audit does NOT flag this — sync.Mutex alone is fine.

**Example shipped:** `EnsembleService.scoreMu` retained after migrating `providers` to safe.Store.

**Recipe:** migrate the collection, keep the mutex scoped only to the non-collection field. Audit passes because no `map[...]` or `[]...` is adjacent to the mutex.

---

## 4. The next-session drain queue (Tier-A, 15 sites)

Hand-picked from the 242 remaining for speed + low risk. Order matters — earlier sites build momentum and validate patterns before you reach harder cases.

1. `internal/cache/expiration.go:ExpirationManager`
2. `internal/cache/invalidation.go:TagBasedInvalidation` ← **canonical Pattern Epsilon (Tier 3)** — worked example in spec §6
3. `internal/cache/invalidation.go:EventDrivenInvalidation`
4. `internal/plugins/registry.go:Registry`
5. `internal/plugins/lifecycle.go:LifecycleManager`
6. `internal/messaging/inmemory/queue.go:SimpleQueue`
7. `internal/messaging/inmemory/queue.go:DelayedQueue`
8. `internal/features/features.go:Registry`
9. `internal/skills/registry.go:Registry`
10. `internal/formatters/registry.go:FormatterRegistry`
11. `internal/formatters/cache.go:FormatterCache`
12. `internal/notifications/polling_store.go:PollingStore`
13. `internal/streaming/state_store.go:InMemoryStateStore`
14. `internal/streaming/types.go:MpscStream`
15. `internal/verifier/adapters/extended_registry.go:ExtendedProviderRegistry`

Running `./scripts/migration/next.sh` regenerates this queue dynamically from the allowlist based on current coupling/size scores.

---

## 5. Known blockers — defer to dedicated sessions

Each needs more context and more test-file rewrite than a batch-mode session supports. Do not attempt in a general drain run.

| Site | File | Why blocked |
|---|---|---|
| `ACPDiscoveryClient` | `internal/services/protocol_discovery.go` | 60+ direct-internal accesses in `acp_client_test.go` |
| `LSPClient` | `internal/services/acp_client.go` | 3 collections with joint atomicity, 1000+ LOC |
| `MCPClient` | `internal/services/mcp_client.go` | HTTP transport + protocol state intertwined |
| `ACPManager / ACPClient` | `internal/services/acp_manager.go` | Compound protocol state |
| `BootManager` | `internal/services/boot_manager.go` | Exports `Results` field for backward compat |
| `ProtocolDiscovery` | `internal/services/protocol_federation.go` | 20+ test-internal accesses (tried, reverted) |
| `DebateService` | `internal/services/debate_service.go` | 6+ test-internal accesses + provider-registry cross-deps |
| `ConcurrencyMonitor` | `internal/services/concurrency_monitor.go` | 20+ test-internal accesses |

Strategy for blockers: dedicate one session per site. First commit is a test-rewrite-only (switching to public API). Second commit is the migration. This makes both steps reviewable and revertible in isolation.

---

## 6. Research synthesis

The choices we made, backed by the research done for this resume kit.

### Why manual migration beat a codemod

The Go ecosystem has three viable codemod routes:
- **[uber-go/gopatch](https://github.com/uber-go/gopatch)** — unified-diff-style patches with metavariables. Mature, widely used at Uber.
- **[codemod/codemod](https://github.com/codemod/codemod)** — general CLI with ast-grep support.
- **gofmt -r** — built-in syntactic rewrite; narrow but reliable.

We evaluated gopatch for the mechanical `mu + map` → `safe.Store` transform. Verdict: **not worth it for this campaign** because:

1. Our 12 shipped migrations produced 6 distinct patterns (Alpha through Zeta). A codemod handles Alpha cleanly, struggles with Beta (reads-under-Update), and can't handle Epsilon at all. ~60% of sites need human judgment on which pattern applies.
2. Test-file coupling varies per site — codemods don't rewrite assertions (`assert.Len(svc.entries, 1)` → `assert.Equal(t, 1, svc.entries.Len())`) in a uniform way.
3. Each migration is a full audit of an area's invariants. A codemod that passes tests but preserves latent races (as happened with `DebateMonitoringService` — the bug was in the reader-after-write pattern the race detector found under `-race -count=3`) gives false confidence.

If a later campaign wants codemod assist, the right target is the call-site fanout AFTER the struct migration: once `X.mu` is gone, every `x.mu.Lock()` / `x.mu.RUnlock()` in the same package is a mechanical delete. Gopatch rules for that narrow transform would be safe.

### Why `safe.Store[K,V]` over `sync.Map`

Several patterns we need do not fit `sync.Map`:

- **Atomic read-modify-write.** `sync.Map` has no `Update(fn)`. Userland "get-then-put" has a check-then-act race. Update is non-negotiable for Pattern Beta and Pattern Epsilon.
- **Typed V.** `sync.Map` uses `interface{}` with runtime casts. `safe.Store[K,V]` preserves the static type. The audit-gate itself depends on the type annotation to detect migrated structs.
- **Iteration with early exit.** `sync.Map.Range` has awkward semantics and no way to atomically delete during iteration. `safe.Store.Range` plus the `Snapshot` escape hatch covers both shapes.
- **`Len()`.** `sync.Map` famously has no `Len`. Several of our migrations needed it.

The Go standard library is exploring a generic `sync.MutexMap` ([#70066](https://github.com/golang/go/issues/70066)) and a generic `sync.Map` replacement ([#47643](https://github.com/golang/go/issues/47643), [#76064](https://github.com/golang/go/issues/76064)) — both future, neither available today. Our `safe.Store` is the production-ready alternative for now.

### Why `atomic.Pointer[T]` (not `atomic.Value`) for scalar pointers

`atomic.Value`:
- Stores `interface{}`; requires consistent dynamic type (panics on first mismatched Store).
- Zero-value `.Load()` returns nil interface, but subsequent `.Store(v)` locks in the type.

`atomic.Pointer[T]` (Go 1.19+):
- Type-safe by construction; cannot store mismatched types.
- `Load()` returns nil if never stored; no type-lock panic.
- Zero allocations for primitive updates.

For HelixAgent on Go 1.25.3, `atomic.Pointer[T]` is the safer default. Used in `FallbackChainValidator` and `MultiPassValidator` migrations.

### Why read-AND-write via Update for Pattern Beta

This was the most surprising lesson. We discovered it under `-race -count=3` while migrating `DebateMonitoringService`:

- Migration: `sync.RWMutex + map[string]*Session` → `safe.Store[string, *Session]`.
- Writes went through `Update` callbacks — correct.
- Reads (e.g., `GetExtendedStatus`) called `Get(id)`, got the pointer, and dereferenced `*session.Status` outside any lock.
- Concurrent `Update` callback mutated `session.Status.CurrentRound` at the same time.
- Race detector caught it on the second iteration of `-count=3`.

The fix: reads that copy fields from pointer values must also route through `Update`. The Store's write lock then covers the copy. Pattern Beta codifies this.

**This is why we don't use codemods.** A codemod would have preserved the naive Get + dereference pattern and shipped a broken migration.

### Sources consulted

- [uber-go/gopatch — Go refactoring via unified-diff patches](https://github.com/uber-go/gopatch)
- [sync: generic sync.MutexMap proposal (#70066)](https://github.com/golang/go/issues/70066)
- [sync: new version of sync.Map (#47643)](https://github.com/golang/go/issues/47643)
- [Codebase Refactoring with help from Go — go.dev/talks/2016/refactor](https://go.dev/talks/2016/refactor.article)
- [Safe Concurrency in Go: Mutex, Atomic, Channels](https://medium.com/@octo_pus/safe-concurrency-in-go-mutex-atomic-and-channels-8299e4740a09)
- [Atomic Value in Go Concurrency](https://medium.com/@AlexanderObregon/atomic-value-in-go-concurrency-d82dd187e73b)
- [Sync Map, Reconstructed — zephyrtronium](https://zephyrtronium.github.io/articles/syncmap.html)

---

## 7. Per-site migration recipe (the checklist)

For each site, from the session the resumes:

1. `./scripts/migration/next.sh` — pick top candidate.
2. Read the struct definition and enumerate methods touching the collection.
3. Classify into Pattern Alpha / Beta / Gamma / Delta / Epsilon / Zeta (§3 above).
4. Check `<file>_test.go` for direct-internal accesses — grep `\.mu\.` and `\.<field>\[`. If count > 10, add to blocker list; skip.
5. Edit the struct: replace mutex + collection with `safe.Store` / `safe.Slice` / `atomic.Pointer`.
6. Rewrite methods per the pattern's recipe.
7. Update test file if it reaches into internals — prefer `assert.Equal(t, N, s.x.Len())` over `assert.Len(t, s.x, N)`.
8. `GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=3 -timeout 180s -run <package-tests> ./internal/...`
9. Delete the site from `scripts/concurrency-audit-allowlist.txt` (one line).
10. `./scripts/concurrency-audit.sh` — confirm allowlist shrunk by exactly one.
11. `git add -A && git commit -m "migrate(<pkg>): <Struct> → safe.Store/Slice (CONST-029)"`
12. Push to all 3 working remotes in parallel:
    ```bash
    for r in github gitlab githubhelixdevelopment; do
      git push "$r" main &
    done; wait
    ```
13. Loop back to step 1.

At any point you can run `./scripts/migration/status.sh` to see current state, drained count, and per-package breakdown of what's left.

---

## 8. Commit message template

Conventional Commits format, as per CLAUDE.md:

```
migrate(<package>): <StructName> → safe.Store (CONST-029)

<Brief summary of what changed. Which pattern (Alpha/Beta/etc).>
<Any race fix discovered during migration.>
<Test file changes summary.>

Allowlist: <before> → <after>.
Refs CONST-029.
```

Previous commits in this campaign (`git log --grep='migrate(' --oneline`) are templates.

---

## 9. When to stop

Stop the session and report to the user when any of:

- **3 consecutive sites are blockers.** Pattern recognition: we're in hard-site territory. Hand off to dedicated-session work.
- **A stress test catches a race you don't immediately understand.** Per the systematic-debugging skill, stop, investigate root cause. Don't push broken migrations.
- **Context window gets deep (>30k tokens in-session on migration work).** Quality per edit degrades after that. Checkpoint and resume.
- **Allowlist hits zero.** Time to delete the allowlist file, simplify `concurrency-audit.sh` to unconditional fail-on-hit, and celebrate.

---

## 10. Files to know

| File | Purpose |
|---|---|
| `scripts/migration/status.sh` | One-command state printer |
| `scripts/migration/next.sh` | Next-candidate picker, scored |
| `scripts/concurrency-audit.sh` | The gate — MUST stay green |
| `scripts/concurrency-audit-allowlist.txt` | The ledger — shrink it, don't grow it |
| `docs/superpowers/specs/2026-04-19-pattern-a-migration-design.md` | The approved spec |
| `docs/superpowers/specs/2026-04-19-migration-resume-kit.md` | This file |
| `docs/development/concurrency-playbook.md` | The discipline reference |
| `Concurrency/pkg/safe/{store,slice}.go` | The primitives |
| `CONSTITUTION.md`, `CLAUDE.md`, `AGENTS.md` | CONST-029 canonical text |

---

## 11. Closing invariants

Until the campaign completes, ensure:

- The audit gate stays green on every commit.
- Every commit is pushed to **all 4 remotes** before session-end (upstream = githubhelixdevelopment; gitlab, github are the mirrors; the fourth is already covered by `upstream` alias pointing to the same remote).
- `make ci-validate-concurrency` is still wired into `make ci-validate-all`.
- No new code lands that re-introduces the bare-`sync.Mutex + map` pattern. The audit would catch it, but be vigilant anyway.

When the last entry comes off the allowlist:

1. Run `GOMAXPROCS=2 nice -n 19 go test -race -count=10 ./internal/...` — full-codebase race suite.
2. Run `./challenges/scripts/run_all_challenges.sh` — master challenge harness.
3. `rm scripts/concurrency-audit-allowlist.txt`.
4. Edit `scripts/concurrency-audit.sh` to remove the allowlist machinery — just fail on any hit unconditionally.
5. Extend the matcher to cover `_test.go` files (after the 26 test-mock sites are also drained).
6. Close out CONST-029 with a celebratory commit and a BUGFIXES.md summary.

---

That's the kit. One command resumes the work; two scripts and a spec carry the rest. Good luck to the next session.
