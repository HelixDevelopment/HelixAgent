# Concurrency Playbook — Structural Safety Over Review Discipline

**Status:** Mandatory for new code. Migration of existing sites tracked separately.
**Authority:** Supersedes any ad-hoc `sync.Mutex + bare map/slice` pattern in shared state.
**Origin:** 2026-04-18 — architectural response to recurring Pattern-A race bugs (BUGFIXES #29, #30, #34–#38).

## The Problem

Every `sync.Mutex + map[K]V` pair in a struct is a contract enforced by human discipline: "remember to take the lock before every read, every write, every iteration." That contract scales badly:

- New methods added later forget the lock (caught only by `-race` in CI, if a test happens to exercise the race).
- Existing methods take the wrong lock (BUGFIX #38 — `calculateDiversityBonus` held `providerScoresMu` while touching `cache`).
- Returned pointers/slices expose internals for unlocked external mutation (BUGFIX #29).
- Iteration holds a read lock while the callback blocks/calls back in, deadlocking the struct.

We have shipped 18+ fixes against this pattern. Each fix is correct; the pattern that demands fixes is wrong.

## The Rule (CONST-029)

**Any struct field that is a mutable collection (map, slice, channel-map, etc.) and is accessed concurrently MUST use a `safe.Store[K,V]` or `safe.Slice[T]` from `digital.vasic.concurrency/pkg/safe`. Bare `sync.Mutex + map`/`sync.Mutex + slice` combinations are prohibited for new code.**

Rationale: the bare-mutex pattern is a review-caught bug class; the primitives make it a structurally impossible bug class. Forgetting to take the lock becomes a compile error — there is no lock to take.

## The Primitives

Import path: `digital.vasic.concurrency/pkg/safe`
Source: [`Concurrency/pkg/safe/`](../../Concurrency/pkg/safe/)

### `safe.Store[K comparable, V any]`

Generic concurrent-safe key-value container. Zero-value ready.

| Operation                          | Semantics                                                      |
|------------------------------------|----------------------------------------------------------------|
| `Get(k) (V, bool)`                 | Read-locked lookup                                             |
| `Put(k, v)`                        | Write-locked insert/overwrite                                  |
| `PutIfAbsent(k, v) (V, bool)`      | Atomic insert-if-missing — no race between Has and Put         |
| `Update(k, fn) `                   | Atomic read-modify-write-or-delete via callback                |
| `Delete(k) (V, bool)`              | Returns prior value + presence                                 |
| `Has(k) bool`, `Len() int`         | Read-locked metadata                                           |
| `Snapshot() map[K]V`               | Point-in-time copy — safe to iterate without the lock          |
| `Keys() []K`, `Values() []V`       | Point-in-time copies                                           |
| `Range(fn)`                        | Locked iteration with early exit — callback MUST NOT call back |
| `Clear()`                          | Empties the store                                              |

### `safe.Slice[T any]`

Generic concurrent-safe slice.

| Operation                              | Semantics                                           |
|----------------------------------------|-----------------------------------------------------|
| `Append(v)`, `AppendAll(vs...)`        | Write-locked append                                 |
| `At(i) (T, bool)`                      | Read-locked indexed access                          |
| `Len() int`                            | Read-locked length                                  |
| `Snapshot() []T`                       | Point-in-time copy                                  |
| `Find(pred) (T, bool)`                 | Read-locked first-match                             |
| `FindIndex(pred) int`                  | Read-locked index-of-first-match                    |
| `UpdateAt(pred, fn) bool`              | Atomic match-update-replace                         |
| `Delete(pred) (T, bool)`               | Write-locked first-match removal                    |
| `Range(fn)`                            | Locked iteration with early exit                    |
| `Replace(next)`                        | Atomic wholesale swap, input is defensively copied  |
| `Clear()`                              | Empties the slice                                   |

**Key invariant shared by both types:** the internal map/slice is **never** exposed. No method returns `map[K]V` or `[]T` by reference. `Snapshot` returns a copy. There is no `Raw()`, `Internal()`, `Map()`, or `Slice()` method — guarded by compile-time test assertions.

## Usage Patterns

### Atomic read-modify-write

Use `Update` / `UpdateAt` — do not read, modify, write-back in three calls.

```go
// WRONG — classic check-then-act race
if v, ok := s.Get("k"); ok {
    v.Count++
    s.Put("k", v)          // someone else may have written between Get and Put
}

// RIGHT — atomic under a single write lock
s.Update("k", func(cur Entry, ok bool) (Entry, bool) {
    if !ok { return Entry{}, false }   // not present, don't create
    cur.Count++
    return cur, true                   // keep
})
```

### Iteration + mutation

Never mutate during `Range` — the lock is held and callbacks that loop back in will deadlock. Snapshot first, iterate the copy, then apply mutations.

```go
// WRONG — Range holds RLock; Update inside tries to take Lock → deadlock
s.Range(func(k string, v Entry) bool {
    if v.Expired { s.Delete(k) }       // DEADLOCK
    return true
})

// RIGHT — snapshot, iterate the copy, mutate outside the lock
for k, v := range s.Snapshot() {
    if v.Expired { s.Delete(k) }       // independent lock acquisition, safe
}
```

### Pointer values (advanced)

Storing `*T` in `Store[K, *T]` protects only the map's integrity — it does not protect mutations of `*T` itself. Two callers reading the same pointer and mutating fields race. Options:

1. **Preferred:** store `T` by value. Mutations go through `Update`, which copies through the callback.
2. **Acceptable:** store `*T` + give each `T` its own mutex. Document the discipline inline.
3. **Avoid:** store `*T` and rely on "callers take the outer mutex too" — this is the pattern we are leaving behind.

## What NOT To Do

**Do not** expose the internal collection. The following methods are prohibited on concurrent-safe types:
- `Map() map[K]V`
- `Slice() []T`
- `Raw() map[K]V`
- `Internal()` returning any collection by reference

**Do not** nest a `safe.Store` inside a struct that also has its own outer mutex — that re-introduces the discipline problem at a higher layer. Pick one, and if you need multi-field atomicity, use `Update`'s callback to batch.

**Do not** use `sync.Map` as a replacement. `sync.Map` is fast for disjoint-key workloads but has no `Update`, no typed `V`, no iteration-with-early-exit semantics, and its shape encourages the same lock-forgetting pattern for cross-key invariants.

## Migration Order

Pattern-A sites identified for migration (priority order):

| Priority | Site                           | File                                                    |
|----------|--------------------------------|---------------------------------------------------------|
| 1        | `SessionHandler`               | `internal/handlers/session.go`                          |
| 1        | `EnhancedScoringService`       | `internal/verifier/enhanced_scoring.go`                 |
| 2        | `InstancePool`                 | `internal/clis/pool.go`                                 |
| 2        | `ProviderRegistry`             | `internal/services/provider_registry.go`                |
| 2        | `Broker`                       | `internal/messaging/inmemory/broker.go`                 |
| 3        | `AdaptiveWorkerPool`           | `internal/background/worker_pool.go`                    |
| 3        | `CacheService` (`userKeys`)    | `internal/cache/cache_service.go`                       |
| 3        | `AgentTeam`, `Task`            | `internal/handlers/extended/ensemble.go`                |
| 4        | `Kairos`, `Cursor`, `Windsurf`, `Kodu` | `internal/agents/...`, `internal/clis/agents/...` |

Each migration is its own PR with: (a) the code change, (b) paired race test that would have caught the pre-migration bug, (c) BUGFIXES.md entry referencing the structural fix.

## Enforcement

Two audit scripts run under `make ci-validate-all`:

- [`scripts/concurrency-audit.sh`](../../scripts/concurrency-audit.sh) — flags struct definitions combining `sync.Mutex|RWMutex` with `map[...]...` or `[]...` fields, except packages on the migration allowlist.
- [`scripts/test-parallel-audit.sh`](../../scripts/test-parallel-audit.sh) — flags tests with `t.Parallel()` that close over mutable package-level state.

New code failing either audit fails CI. Existing allowlisted packages migrate per the table above and come off the allowlist.

## Why Not Just Rely On `-race`?

The race detector is probabilistic. It reports races *that happened during this run*. It cannot prove absence of races — only presence. We have shipped fixes for 18+ races in this codebase; each was caught on a different run, in a different test, under a different load profile. Some required `-count=10` to surface.

The race detector is valuable as a backstop. It is not a sufficient strategy. Structural unreachability is.

## References

- Primitives: `Concurrency/pkg/safe/{store,slice}.go` with 10× race-clean coverage
- Prior-pattern fixes: `docs/issues/fixed/BUGFIXES.md` entries #29, #30, #34, #35, #36, #37, #38
- Constitution: CONST-029 (forthcoming synchronisation to `CLAUDE.md` and `AGENTS.md`)
