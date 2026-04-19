# Pattern-A Zero-Tolerance Migration — Design Spec

**Date:** 2026-04-19
**Status:** Approved (user: milos85vasic.2nd@gmail.com)
**Predecessor:** CONST-029 foundation (commits `c56b710` Concurrency / `faa0976e` HelixAgent)
**Rule:** CONST-029 (Concurrent-Safe Containers)
**Author/approver:** Claude Opus 4.7 + user

## 1. Context and goal

CONST-029 retired the `sync.Mutex + bare map/slice` discipline-by-convention pattern by landing `digital.vasic.concurrency/pkg/safe.Store[K,V]` and `safe.Slice[T]`, an audit gate (`scripts/concurrency-audit.sh`), and a playbook. The audit seeded an allowlist of 254 pre-existing Pattern-A sites; the gate prevents regression but does not migrate existing code.

**Goal:** zero-tolerance drain of the allowlist. Every one of the 254 entries is either migrated to `safe.*`, refactored so it no longer matches Pattern-A, or (for test-file mocks) handled by a scope refinement accompanied by test-code migration. The final state is: `scripts/concurrency-audit-allowlist.txt` deleted, `scripts/concurrency-audit.sh` reduced to an assertion of zero hits, and no Pattern-A struct anywhere in HelixAgent production or test code.

**Non-goals (for this spec):** migrating the 38–39 other HelixAgent submodules' internal Pattern-A sites. Those get the rule propagated and their own audit wired up, but their internal migrations are tracked separately per submodule.

## 2. Allowlist decomposition (measured)

```
total                           254
  test-file mocks                26   (*_test.go)
  production sites              228
    single-collection           129   (clean safe.Store / safe.Slice migration)
    two-collection               60   (Tier-1/2 independent, or Tier-3 joint)
    three-or-more collections    26   (Tier-3 joint, possibly refactor)
    parser edge-cases           ~13   (inspect manually)
  package density top-3
    services/                    49
    security/                     8
    verifier/                     7
```

## 3. Scope policy

**Zero-tolerance drain.** Every allowlist entry must leave. Sites that cannot migrate under the existing three tiers get an audit-matcher refinement that narrows the matcher rather than adding per-site exceptions (no "justified residuals" list).

**Test-file mocks.** The 26 `*_test.go` entries are migrated in a final pass that also teaches the audit matcher to count test files. Scope refinement (skipping `_test.go`) only activates AFTER all 26 are migrated; otherwise shrinking scope would mask real debt.

**Third-party read-only submodules** (`cli_agents/`, `MCP/`) are excluded per CLAUDE.md Rule 10. Submodule-internal migrations happen in the submodule's own repo.

## 4. Chosen approach: package-by-package manual migration (Approach 1)

Reasoning: the codebase has subtle invariants (compound check-then-act operations, cross-method lock discipline, goroutine-lifecycle coupling) that an AST codemod cannot reliably preserve. Hand-migration gives each site human attention. Effort cost (8–12 work-sessions) is accepted as the price of correctness.

Rejected alternatives:
- **Codemod-driven migration (Approach 2):** tempting for the 129 single-collection sites but risks preserving the race-prone call-site patterns mid-transform. Would require a post-codemod hand-audit of every migrated site anyway — net saving was marginal.
- **Primitives-expansion first (Approach 3):** adding `Tx` or `ValueStore` to the `safe` package before we have proof the existing primitives are insufficient is YAGNI. Tier 3 (see §6) shows `safe.Store[string, innerState]` with a constant key already covers joint-atomicity cleanly.

## 5. Migration unit of work (single commit)

Each allowlist entry = one migration = one commit. Fixed 7-step shape:

1. **Identify** — pick next entry from allowlist (`file:line:StructName`).
2. **Read invariants** — open the file and existing tests; enumerate methods touching the collection, compound operations, returned references to internals.
3. **Rewrite struct** — replace `sync.RWMutex + map[K]V` with `*safe.Store[K,V]`; initialize in constructor.
4. **Rewrite methods** — simple reads/writes → `Store.Get/Put/Delete`; compound operations → `Store.Update` callbacks; iteration → `Store.Snapshot` + range over copy.
5. **Fix call sites** — usually same-package; callers that previously also took the struct's mutex become simpler.
6. **Remove allowlist entry** — delete that one line from `scripts/concurrency-audit-allowlist.txt`.
7. **Verify** — `GOMAXPROCS=2 nice -n 19 go test -race -count=3 ./internal/<pkg>/...` and `./scripts/concurrency-audit.sh` both green.

Commit message format: `migrate(<pkg>): <StructName> → safe.Store/Slice (CONST-029)`. Per-commit scope enables `git revert <sha>` rollback.

## 6. Joint-atomicity handling (the hardest design choice)

86 of 228 production sites have ≥2 collections. Three migration tiers:

### Tier 1 — Independent collections
Two collections that never need joint atomicity (verified by enumerating every method: no method touches both). Each becomes its own `safe.Store` / `safe.Slice`. No outer mutex. This is the default assumption; falsify by reading before reaching for Tier 3.

### Tier 2 — Consistent read across collections, writes are single-collection
A compound read needs a consistent view of both, but writes only touch one collection at a time. Use two independent `safe.Store`s; the compound read calls `Snapshot()` on each. Read consistency is eventually-consistent by a few microseconds — acceptable for cache/metrics, document explicitly in the method comment.

### Tier 3 — Genuine joint atomicity (writes touch multiple collections)
Collapse joint state into an inner struct:

```go
type invalidationState struct {
    tagIndex map[string]map[string]struct{}
    keyTags  map[string][]string
}

type TagBasedInvalidation struct {
    state   *safe.Store[string, invalidationState]  // always keyed "_"
    metrics *InvalidationMetrics
}

const _stateKey = "_"

func (i *TagBasedInvalidation) AddTag(key string, tags ...string) {
    i.state.Update(_stateKey, func(s invalidationState, _ bool) (invalidationState, bool) {
        s.keyTags[key] = append(s.keyTags[key], tags...)
        for _, tag := range tags {
            if s.tagIndex[tag] == nil {
                s.tagIndex[tag] = make(map[string]struct{})
            }
            s.tagIndex[tag][key] = struct{}{}
        }
        return s, true
    })
}
```

The Store's write lock covers the entire callback; the callback sees a mutation-in-place view of the inner maps. Fully atomic, no outer mutex.

**Discipline contract for Tier 3:** callbacks MUST NOT leak inner map references outside the callback body. Each Tier 3 migration adds a stress test that runs concurrent `Update` calls and asserts invariants (e.g., for tag invalidation: `tagIndex` and `keyTags` stay mutually consistent).

### Escape hatch (target: zero uses)
If profiling shows a Tier 3 site copies a prohibitively large struct on every Update, keep bare `sync.Mutex` with all collection fields as `safe.*` types. The current audit matcher already passes such structs (no bare `map[...]` / `[]...` next to a mutex). Every escape-hatch use is flagged in BUGFIXES.md and listed in a "justified exceptions" section of the playbook with the profiling data. Strong preference for refactoring the state shape instead.

## 7. Package execution order (risk-adjusted)

| Order | Package                         | Sites | Rationale                                                                         |
|------:|---------------------------------|------:|-----------------------------------------------------------------------------------|
| 1     | `internal/services/`            |    49 | Highest density; contains EnhancedScoring (BUGFIX #38) + debate/provider registry |
| 2     | `internal/handlers/`            |     6 | SessionHandler (BUGFIX #29), canonical Pattern-A site                             |
| 3     | `internal/verifier/`            |     7 | EnhancedScoringService — tests Tier-1/2 vs Tier-3 correctness                     |
| 4     | `internal/security/`            |     8 | Security-critical; exercises the pattern under load-bearing invariants            |
| 5     | `internal/cache/`               |     6 | TagBasedInvalidation — canonical Tier-3 site; validates the pattern               |
| 6     | `internal/messaging/*`, `inmemory/` | 11 | Broker (4 maps) — hardest joint-atomicity case                                    |
| 7     | Remaining packages              |  ~115 | Mechanical bulk after patterns established — ~20 distinct packages                |
| 8     | Test-file mocks (`_test.go`)    |    26 | Last; audit matcher extended to include test files in the same commit             |

**"Ordering groups," not packages.** Row 7 aggregates ~20 distinct packages (streaming, notifications, plugins, adapters, mcp, llmops, agentic, bigdata, clis, agents, background, formatters, observability, etc.). Each still gets its own commit(s); they are grouped in the ordering table because the work on them is mechanical after earlier patterns are established.

**Parser edge-cases (~13 sites).** The audit matcher missed a cardinality reading for ~13 sites (struct body spans >40 lines, or comment placement confuses the awk state machine). These sites ARE in the allowlist; they just weren't categorized into Tier 1/2/3 by the upfront scan. They will be categorized during per-package work (steps 1–2 of the migration unit) and handled by whichever tier fits.

**Hard stop after ordering group 1.** Once `services/` is drained (~49 sites), pause for a review:
- Did any tests regress?
- Did patterns (Tier 1/2/3) generalize cleanly?
- Are commit sizes manageable? Do we need to sub-batch future packages?

If all green, remaining packages are routine batch-work. If not, revisit design before committing more.

## 8. Test strategy (three gates per migration commit)

### Gate 1 — Behavior preservation
All existing tests in the touched package pass unchanged:
```
GOMAXPROCS=2 nice -n 19 go test -race -count=3 ./internal/<pkg>/...
```
A failing pre-existing test means the migration changed behavior. Either revise the migration or update the test with an explicit callout in the commit message.

### Gate 2 — Concurrency proof
Each migration commit either cites an existing stress test that exercises concurrent access, or adds one. Tier-3 commits MUST add an invariant test. Stress tests run `-count=10` under `-race` to make the race detector probabilistic-safe.

Budget exception: ~20% of sites have trivial surfaces where a stress test would be busywork; commit message notes "no new test — behavior preserved." This is bounded; if we hit >30% of sites in a package qualifying for the exception, we revisit.

### Gate 3 — Audit + allowlist consistency
`./scripts/concurrency-audit.sh` passes with allowlist shrunk by exactly the number of sites in this commit. A counter check catches drift (e.g., a site renamed mid-migration; double-removal; accidental skip).

### BUGFIXES linkage (CONST-028)
Migrations that uncover an actual latent race (not just a pattern violation) get a BUGFIXES entry with the paired stress test as verification artifact. Pure-refactor migrations do not — that would be noise.

### Project-completion gate
When allowlist hits zero:
1. Full-codebase race suite: `GOMAXPROCS=2 nice -n 19 go test -race -count=10 ./internal/...`
2. Full challenge master suite (39 challenges): all PASSED.
3. Delete `scripts/concurrency-audit-allowlist.txt`.
4. Simplify `scripts/concurrency-audit.sh` to unconditional fail-on-hit (remove `--update-allowlist` flag).
5. Extend audit matcher to include `_test.go` files (enabled only after all 26 test-mock sites are migrated).
6. Final commit message announces CONST-029 fully discharged.

## 9. Cross-submodule propagation (mandatory constraint)

Per user directive: CONST-029 is propagated to every submodule's `CLAUDE.md`, `AGENTS.md`, and `Constitution` (where present). The Concurrency submodule is the canonical home of the primitives and playbook.

### Propagation plan

1. **Concurrency submodule (canonical).**
   - Add `Concurrency/docs/concurrency-playbook.md` (mirror of HelixAgent's current playbook, made self-contained).
   - Add CONST-029 section to `Concurrency/CLAUDE.md` and `Concurrency/AGENTS.md`.
   - Commit and push to github + gitlab.

2. **HelixAgent main (existing).**
   - `docs/development/concurrency-playbook.md` keeps its content but gains a "Canonical source" note pointing to `Concurrency/docs/concurrency-playbook.md`. Both remain valid; the HelixAgent copy is repo-local ergonomics, Concurrency copy is authoritative.

3. **Other submodules (38–39 of 41, excluding `cli_agents/`, `MCP/`, any other read-only third-party).** For each submodule:
   - Add a concise CONST-029 section to `CLAUDE.md` and `AGENTS.md` with the rule text and a link to `Concurrency/docs/concurrency-playbook.md`.
   - If the submodule has its own `Constitution.md` / `CONSTITUTION.md`, add an entry there too.
   - Run `scripts/concurrency-audit.sh` (adapted) on the submodule. If hits exist, seed a local allowlist.
   - Wire the audit into whatever lint/validate target the submodule has (`make lint`, `make test`, `make ci-validate-all`, etc.).
   - Commit + push to github + gitlab.
   - Bump the submodule pointer in HelixAgent main; commit + push to all 4 HelixAgent remotes.

4. **Submodule internal migrations.** Per-submodule Pattern-A migrations happen in that submodule's repo with its own allowlist. Scope is tracked separately; not blocking for the HelixAgent main-repo drain.

### Read-only submodules (skip)
`cli_agents/`, `MCP/`, and any other submodule whose `CLAUDE.md` declares read-only status (CLAUDE.md Rule 10). Skipped entirely — no commits, no propagation, no audit. Verified in the first propagation pass by grep for "read-only" / "third-party" markers in each submodule's top-level docs.

## 10. Commit and push cadence (mandatory constraint)

Per user directive: never leave work uncommitted at the end of a batch; never leave commits unpushed overnight.

### After every successful migration commit
- **Main repo:** push to `github`, `gitlab`, `githubhelixdevelopment`, `upstream` — all 4 remotes, in parallel.
- **If a submodule was touched:**
  - Push to the submodule's `github` + `gitlab` remotes in parallel.
  - Bump the submodule pointer in HelixAgent main as a separate follow-up commit.
  - Push HelixAgent main to all 4 remotes.

### Session-end discipline
Before ending any work session on this migration:
- `git status --short` shows no unpushed commits in any repo.
- No uncommitted changes except deliberately in-flight work documented in a session log.

### Failure handling
If a push fails (network, auth, upstream rejected):
- Retry once after 30 seconds.
- If still failing, note in session log; do not continue migration work until the push lands.
- Never `--force` push without explicit user approval.

## 11. Effort estimate

- **HelixAgent site migrations (254 sites, 8 ordering groups covering ~25 distinct packages, Approach 1):** 8–12 work-sessions.
- **Cross-submodule propagation (38–39 submodules, doc-only):** 1–2 work-sessions.
- **Submodule-internal Pattern-A migrations (unknown until audit pass):** scope TBD after the propagation pass; each submodule gets its own time estimate.

Milestones:
1. Package 1 (services/) drained → stop, review.
2. Packages 2–5 drained → midpoint check.
3. Packages 6–8 drained → allowlist zero, project gates run.
4. Cross-submodule propagation complete.
5. CONST-029 fully discharged.

## 12. Risks and mitigations

| Risk                                                       | Mitigation                                                                           |
|------------------------------------------------------------|--------------------------------------------------------------------------------------|
| Tier-3 pattern has a subtle bug we haven't found yet       | Package 1 hard-stop includes Tier-3 review; first Tier-3 commit has exhaustive test |
| Large packages stall (services/ = 49 sites)                | Sub-batch by sub-directory; stop after 20 if session window closing                 |
| A migration exposes an unrelated latent bug                | BUGFIXES entry, paired test, separate fix commit; migration commit stays scoped     |
| `-race` finds intermittent failures only under `-count=10` | Standard per-migration cadence is `-count=3`; project-completion gate is `-count=10`|
| Submodule audit surfaces hundreds of new sites             | Each submodule gets its own allowlist; internal migration scope is not blocking     |
| Push cadence slows iteration                               | Parallel push via `run_in_background`; accepted cost for upstream redundancy        |

## 13. Approval

- [x] Scope policy (zero-tolerance drain) — approved
- [x] Chosen approach (Approach 1, package-by-package manual) — approved
- [x] Migration unit of work — approved
- [x] Package ordering and batching — approved
- [x] Joint-atomicity handling (Tier 1/2/3) — approved
- [x] Test strategy (three gates) — approved
- [x] Audit mechanics + cross-submodule propagation + commit cadence — approved

Spec complete. Ready to transition to implementation planning.
