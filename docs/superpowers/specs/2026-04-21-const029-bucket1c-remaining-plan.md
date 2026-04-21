# CONST-029 Bucket-1c Remaining Tractable Sites — Plan (2026-04-21)

**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-2.5
**Source inventory:** `docs/development/REMAINING_WORK_2026-04-21.md` §Bucket-1c (non-executed subset)
**Status:** PLAN-ONLY. No code changes in this document's commit.
**Executed in-session:** MemoryService, ConcurrencyAlertManager,
ContextManager (see Phase-1.A commits).

## Common pattern

These sites are mechanically tractable (safe.Store / safe.Slice swap
works) but each carries one of:
- High test-coupling or high caller volume (50+ direct field / method
  accesses across source + tests + handlers).
- A latent bug whose fix should accompany the migration.
- A compound state machine that merits Pattern Zeta analysis rather
  than a plain primitive swap.

Each is scheduled as a dedicated session. Budget estimates below.

---

## Site 1: FreeProviderAdapter (bonus race-fix)

### File

`internal/verifier/adapters/free_adapter.go:68`

### Current shape

```go
// FreeProviderAdapter handles verification for free providers
// (Zen, OpenRouter :free models)
type FreeProviderAdapter struct {
    verifierSvc *verifier.VerificationService
    config      *FreeAdapterConfig
    httpClient  *http.Client

    zenProvider    *zen.ZenProvider
    zenCLIProvider *zen.ZenCLIProvider

    // Cached verification results
    mu             sync.RWMutex
    verifiedModels map[string]*verifier.UnifiedModel
    lastVerified   map[string]time.Time
    healthStatus   map[string]bool

    // Models that failed direct API verification
    failedAPIModels map[string]error
}
```

Readers (`GetVerifiedModels`, `IsModelVerified`, `GetHealthStatus`,
`GetFailedAPIModels`, `IsModelUsingCLIFacade`, `GetCLIFacadeModels`)
all take `fa.mu.RLock()`. Writers (inside `VerifyZenProvider`,
`VerifyOpenRouterFreeModels`) take a **call-local** `modelsMu
sync.Mutex` declared at the top of each method — NOT `fa.mu`.

### Latent race — detail

`VerifyZenProvider` (line 126) declares:

```go
var wg sync.WaitGroup
var modelsMu sync.Mutex          // ← LOCAL mutex
sem := make(chan struct{}, fa.config.MaxConcurrentVerifications)
```

and then each goroutine writes to the shared-on-adapter maps under
that local mutex:

```go
modelsMu.Lock()
fa.verifiedModels[mID] = model
fa.lastVerified[mID] = time.Now()
modelsMu.Unlock()
```

`VerifyOpenRouterFreeModels` (line 786) repeats the pattern with a
**different** local `modelsMu`. Meanwhile `GetVerifiedModels` et al.
hold only `fa.mu.RLock()`. The local mutex does not serialise against
`fa.mu.RLock()` — so:

1. Concurrent `VerifyZenProvider` + `VerifyOpenRouterFreeModels`
   calls race on `fa.verifiedModels` / `fa.lastVerified` (two
   different local mutexes, same map).
2. Any reader (`GetVerifiedModels`, etc.) holding `fa.mu.RLock()` can
   observe a partial map write from either verify call.

The race has been present since the adapter was introduced and is
almost certainly masked by the fact that only one verify path runs at
startup. It is a ticking bomb under the new HelixQA concurrency
regimes.

### Touch-point census

| File | Pattern | Count |
|------|---------|-------|
| `internal/verifier/adapters/free_adapter.go` | `fa.verifiedModels \| fa.lastVerified \| fa.healthStatus \| fa.failedAPIModels \| fa.mu.` | 29 |
| `internal/verifier/adapters/free_adapter_test.go` | same | not measured in this pass (all access is via public methods — tests are insulated) |

Public getters (`GetVerifiedModels`, `IsModelVerified`, `GetHealthStatus`,
`GetFailedAPIModels`, `IsModelUsingCLIFacade`, `GetCLIFacadeModels`)
are the only external entry points — test coupling to private fields
is effectively zero.

### Decision

Migrate each of `verifiedModels`, `lastVerified`, `healthStatus`,
`failedAPIModels` to its own `safe.Store[string, V]`. Remove **both**
`fa.mu` and every per-call `modelsMu`. The race disappears because
`safe.Store`'s atomic `Set` / `Update` / `Range` provide the missing
cross-goroutine serialisation.

### Regression test

Add `TestFreeProviderAdapter_ConcurrentVerify_NoRace` to
`free_adapter_test.go`:

```go
func TestFreeProviderAdapter_ConcurrentVerify_NoRace(t *testing.T) {
    // run under `go test -race ./internal/verifier/adapters/...`
    fa := NewFreeProviderAdapter(nil, DefaultFreeAdapterConfig())
    ctx := context.Background()

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _, _ = fa.VerifyZenProvider(ctx)
        }()
        wg.Add(1)
        go func() {
            defer wg.Done()
            _ = fa.GetVerifiedModels()
            _ = fa.GetHealthStatus()
            _ = fa.GetFailedAPIModels()
        }()
    }
    wg.Wait()

    // If -race did not flag, the final state must at least be
    // internally consistent: every key in verifiedModels must have
    // a corresponding entry in lastVerified.
    models := fa.GetVerifiedModels()
    for id := range models {
        if !fa.IsModelVerified(id) {
            t.Fatalf("inconsistent state: %s in map but IsModelVerified=false", id)
        }
    }
}
```

### Session-budget estimate

~2.5h (migration 1h + race fix + regression test 1.5h).

---

## Site 2: ProviderRegistry

### File

`internal/services/provider_registry.go:72` (2255 LOC file)

### Current shape

```go
type ProviderRegistry struct {
    providers             map[string]llm.LLMProvider
    circuitBreakers       map[string]*CircuitBreaker
    concurrencySemaphores map[string]*semaphore.Weighted
    providerConfigs       map[string]*ProviderConfig
    providerHealth        map[string]*ProviderVerificationResult
    activeRequests        map[string]*int64
    config                *RegistryConfig
    ensemble              *EnsembleService
    requestService        *RequestService
    memory                *MemoryService
    discovery             *ProviderDiscovery
    scoreAdapter          *LLMsVerifierScoreAdapter
    startupVerifier      *verifier.StartupVerifier
    mu                    sync.RWMutex
    drainTimeout          time.Duration
    autoDiscovery         bool
    initSemaphore         *semaphore.Weighted
    initOnce              map[string]*sync.Once
}
```

**7 maps** protected by one `mu` (the 6 listed in the task brief plus
`initOnce`).

### Touch-point census

| Location | Pattern | Count |
|----------|---------|-------|
| `provider_registry.go` | `r.(providers\|circuitBreakers\|concurrencySemaphores\|providerConfigs\|providerHealth\|activeRequests\|initOnce)\|r.mu.` | 133 |
| `provider_registry_test.go` + `provider_registry_unit_test.go` | same | 0 (tests use public API) |
| `internal/handlers/*.go` (non-test) referencing the registry type | file count | 7 |
| `internal/services/*.go` (non-test) referencing the registry type | file count | 8 |

The 0 in tests is important: tests are insulated by the public
method surface. The 133 touches in the source file are all
mu-protected accesses that need mechanical rewrite.

### Decision

Each of the 7 maps → its own `safe.Store`. Drop `mu` entirely. The
maps have **no cross-map invariants** — they are independent indices
keyed by provider name. The migration is mechanically clean; the
risk is caller volume (15 external files) rather than correctness.

### Migration sketch

```go
type ProviderRegistry struct {
    providers             *safe.Store[string, llm.LLMProvider]
    circuitBreakers       *safe.Store[string, *CircuitBreaker]
    concurrencySemaphores *safe.Store[string, *semaphore.Weighted]
    providerConfigs       *safe.Store[string, *ProviderConfig]
    providerHealth        *safe.Store[string, *ProviderVerificationResult]
    activeRequests        *safe.Store[string, *int64]
    initOnce              *safe.Store[string, *sync.Once]
    // unchanged scalar fields…
}
```

Every `r.mu.Lock()/RLock()` block collapses to a single `.Get` /
`.Set` / `.Range` call. Methods like `GetAllProviders` become one
`.Range` closure.

### Caller review checklist

No call-site change is required if the public method surface is
preserved — but the diff should be walked against:

- `internal/handlers/provider_management.go`, `vision_handler.go`,
  `mcp.go`, `acp_handler.go`, `openai_compatible.go`,
  `verification_handler.go`, `protocol_sse.go` (7 handler files).
- `internal/services/debate_*`, `ensemble*`, `request_service.go`,
  `memory_service.go` (8 service files).
- `internal/verifier/*` consumers of `ProviderRegistry`.

### Session-budget estimate

~3h (migration 1h, caller audit + handler test pass 2h).

---

## Site 3: DebateTeamConfig

### File

`internal/services/debate_team_config.go:304` (1624 LOC file)

### Current shape

```go
type DebateTeamConfig struct {
    mu               sync.RWMutex
    members          map[DebateTeamPosition]*DebateTeamMember
    verifiedLLMs     []*VerifiedLLM // sorted by score
    providerRegistry *ProviderRegistry
    discovery        *ProviderDiscovery
    startupVerifier  *verifier.StartupVerifier
    logger           *logrus.Logger
}
```

### Touch-point census

| Location | Pattern | Count |
|----------|---------|-------|
| `debate_team_config.go` | `dtc.(members\|verifiedLLMs\|providerRegistry\|discovery\|startupVerifier)\|dtc.mu.` | 86 |
| `debate_team_config.go` | `dtc.members\b` (subset) | 14 |
| `debate_team_config.go` | `dtc.verifiedLLMs\b` (subset) | 35 |
| `debate_team_config_test.go` | same top pattern | 26 |
| `debate_team_config_test.go` | `.members\|.verifiedLLMs` direct reach | 0 (tests use public API) |

### Decision

- `verifiedLLMs []*VerifiedLLM` → `safe.Slice[*VerifiedLLM]`.
- `members map[DebateTeamPosition]*DebateTeamMember` → `safe.Store`.

Drop `dtc.mu`. `InitializeTeam` and friends replace their
`dtc.mu.Lock()/Unlock()` blocks with appropriate `safe.Store.Range`
and `safe.Slice` bulk operations (`Replace`, `Snapshot`).

### Migration sketch

```go
type DebateTeamConfig struct {
    members          *safe.Store[DebateTeamPosition, *DebateTeamMember]
    verifiedLLMs     *safe.Slice[*VerifiedLLM]
    providerRegistry *ProviderRegistry
    discovery        *ProviderDiscovery
    startupVerifier  *verifier.StartupVerifier
    logger           *logrus.Logger
}
```

`InitializeTeam` — currently ~60 lines under one big `Lock()` —
splits into a compute phase (unlocked) that produces the sorted
slice, then a single `verifiedLLMs.Replace(sorted)` plus
`members.Clear(); for pos, m := range prepared { members.Set(pos, m) }`.

### Test impact

86 + 26 = 112 references to audit, but since tests hit public API
only, the rewrite is concentrated in the source file.
`InitializeTeam`'s atomicity contract must be preserved — callers
expect "either the whole team is there or nothing is" — so the
Replace + sequential Set pattern must be wrapped in a documented
"prepare then publish" block.

### Session-budget estimate

~2.5h.

---

## Site 4: CodeGraph

### File

`internal/knowledge/code_graph.go:124`

### Current shape

```go
type CodeGraph struct {
    config        CodeGraphConfig
    nodes         map[string]*CodeNode
    edges         map[string]*CodeEdge
    nodesByType   map[NodeType][]*CodeNode
    edgesBySource map[string][]*CodeEdge
    edgesByTarget map[string][]*CodeEdge
    embedder      EmbeddingGenerator
    mu            sync.RWMutex
    logger        *logrus.Logger
}
```

### Touch-point census

| Location | Pattern | Count |
|----------|---------|-------|
| `code_graph.go` | `g.(nodes\|edges\|nodesByType\|edgesBySource\|edgesByTarget)\|g.mu.` | 64 |
| `knowledge_test.go` | `CodeGraph\|AddNode\|AddEdge\|FindNodesByType\|…` | 281 |
| `knowledge_extended_test.go` | same | 97 |
| Tests direct-reach into private fields | - | 0 (public API only) |

378 method-call sites in tests — all public surface, so the internal
refactor is insulated.

### Cross-invariants

- `nodes` and `nodesByType` must stay consistent: every entry in
  `nodes` must appear once under its Type bucket in `nodesByType`.
- `edges`, `edgesBySource`, `edgesByTarget` must stay consistent:
  every entry in `edges` appears once in each of the two indices
  keyed by its endpoint.

`AddNode` updates `nodes` + `nodesByType` inside one critical
section. `AddEdge` updates `edges` + `edgesBySource` +
`edgesByTarget` inside one critical section. Readers
(`FindNodesByType`, `GetEdgesBySource`, `GetEdgesByTarget`,
`QueryRelated`) rely on seeing both sides of the pair consistently.

### Decision

A plain safe.Store-per-map swap **breaks the invariant**: a reader
calling `nodesByType.Get(t)` could observe a bucket that references a
node which is no longer in `nodes` (reader saw the nodesByType write
but not the paired nodes write, or vice-versa).

**Two-option choice, plan the cheaper first:**

**Option A (recommended — Pattern Zeta, narrow):** keep a small
`idxMu sync.RWMutex` that guards ONLY the paired index updates.
Migrate the per-map storage to `safe.Store` for lock-free reads of
individual keys, but require `idxMu.RLock()` for any multi-index
read (FindNodesByType, GetEdgesBySource) and `idxMu.Lock()` for any
multi-index write (AddNode, AddEdge, RemoveNode, RemoveEdge). This
is a genuine invariant lock, not an "I forgot to remove the old
mutex" lock.

**Option B (more work — state-pointer):** bundle
`(nodes, nodesByType)` behind a single `atomic.Pointer[*nodeIndex]`
and `(edges, edgesBySource, edgesByTarget)` behind a single
`atomic.Pointer[*edgeIndex]`. Writes copy-modify-swap. Readers load
once and walk a consistent snapshot. Appropriate if read-heavy
(likely true for the knowledge graph) and write volume is modest.

Final decision to be made during the session after measuring
read:write ratio in `QueryRelated` and friends; **default to Option
A** since its regression risk is lower.

### Migration sketch (Option A)

```go
type CodeGraph struct {
    config        CodeGraphConfig
    nodes         *safe.Store[string, *CodeNode]
    edges         *safe.Store[string, *CodeEdge]
    idxMu         sync.RWMutex                         // guards paired indices
    nodesByType   map[NodeType][]*CodeNode             // protected by idxMu
    edgesBySource map[string][]*CodeEdge               // protected by idxMu
    edgesByTarget map[string][]*CodeEdge               // protected by idxMu
    embedder      EmbeddingGenerator
    logger        *logrus.Logger
}
```

The `idxMu` stays on the Bucket-1 allowlist with a pointer to the
invariant comment — it is NOT a forgotten lock.

### Session-budget estimate

~3h (decision measurement 30m, Option-A migration 1.5h, test pass 1h).

---

## Site 5: InstancePool (Pattern Zeta candidate)

### File

`internal/clis/pool.go:13` (474 LOC file)

### Current shape

```go
type InstancePool struct {
    agentType AgentType

    minIdle        int
    maxIdle        int
    maxActive      int
    maxLifetime    time.Duration
    acquireTimeout time.Duration

    idle   []*AgentInstance
    idleCh chan *AgentInstance
    active map[string]*AgentInstance

    factory func() (*AgentInstance, error)

    hits   uint64
    misses uint64
    evicts uint64

    placeholderSeq uint64

    mu     sync.RWMutex
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}
```

### Compound state

`mu` genuinely protects a **three-step transition** in `Acquire`:

1. Remove instance from `idle` slice.
2. Insert instance into `active` map.
3. (On miss path) reserve a placeholder key in `active`, run
   `factory()` outside the lock, then swap placeholder for real key.

`idleCh` is a secondary channel index into `idle`; the two can drift
out of sync and the code compensates under the lock. `placeholderSeq`
is an atomic counter producing unique reservation keys — the
placeholder-key state machine is the subtle bit.

### Touch-point census

| Location | Pattern | Count |
|----------|---------|-------|
| `pool.go` | `p.(idle\|active\|idleCh)\|p.mu.` | 76 |
| `pool_test.go` | same pattern | 0 |
| `pool_test.go` | public-method calls (`NewInstancePool\|Acquire\|…`) | 17 |

### Decision

**Pattern Zeta: keep `mu`.** A safe.Store swap would force the
three-step transition ("drop from idle → insert to active → resolve
placeholder") to split across two independent stores, breaking the
invariant that a concurrent `Acquire` must see a consistent
active-count when deciding whether to create a new instance.

What IS safe and IS in scope:

- `hits`, `misses`, `evicts` are already `uint64` with atomic
  helpers. Leave them alone.
- `placeholderSeq` is already atomic. Leave it alone.
- Add a **documentation block** above `mu` explaining exactly which
  invariant it protects (the three-step state transition) and link
  to the concurrency playbook's Pattern Zeta section.
- Add the site to the concurrency-audit allowlist with a
  Pattern-Zeta annotation, not a "TODO migrate" annotation.

### Migration sketch (minimal)

```go
// mu guards a compound state transition across three fields:
//   (idle slice, active map, idleCh channel) + placeholder-key
//   reservation in `active`.
// A safe.Store swap per field would break the invariant that
// concurrent Acquire callers see a consistent active-count when
// deciding whether the pool is exhausted. This is an explicit
// Pattern-Zeta site — see docs/development/concurrency-playbook.md
// §Pattern Zeta.
mu sync.RWMutex
```

### Session-budget estimate

~2h (documentation 30m + allowlist annotation + re-run concurrency
audit + write up Pattern-Zeta entry in the playbook if missing 90m).

---

## Site 6: WorkerPool

### File

`internal/ensemble/background/worker_pool.go:33`

### Current shape

```go
type WorkerPool struct {
    db     *sql.DB
    logger *log.Logger

    size              int
    queueSize         int
    maxPendingResults int64

    taskQueue chan *clis.Task
    resultQueue chan *TaskResult

    pendingResults sync.Map // already migrated
    pendingCount   int64    // atomic

    workers []*Worker
    instanceAssignments map[string]string // taskID -> instanceID

    tasksSubmitted uint64
    tasksCompleted uint64
    tasksFailed    uint64
    tasksCancelled uint64
    tasksRejected  uint64

    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
    mu     sync.RWMutex

    running bool
}
```

### Touch-point census

| Location | Pattern | Count |
|----------|---------|-------|
| `worker_pool.go` | `wp.(instanceAssignments\|workers\|taskQueue\|resultQueue\|pendingResults\|running)\|wp.mu.` | 30 |
| `worker_pool_test.go` | same | 0 |
| `worker_pool_phase3_test.go` | same | 0 |
| `worker_pool_soak_test.go` | same | 0 |

### Decision

- `instanceAssignments map[string]string` → `safe.Store[string, string]`.
- `running bool` → `atomic.Bool`.
- `workers []*Worker` → leave as-is (only mutated during
  Start/Stop, read on shutdown; keep a narrow `startMu sync.Mutex`
  if needed for Start/Stop serialisation, or fold into `running`
  transition using `atomic.Bool.CompareAndSwap`).

Drop `mu`. The channels (`taskQueue`, `resultQueue`) are already
thread-safe. `pendingResults` is already a `sync.Map` with a
separate atomic count.

### Migration sketch

```go
type WorkerPool struct {
    db     *sql.DB
    logger *log.Logger

    size              int
    queueSize         int
    maxPendingResults int64

    taskQueue   chan *clis.Task
    resultQueue chan *TaskResult

    pendingResults sync.Map
    pendingCount   int64

    workers             []*Worker // mutated only inside Start() / Stop()
    instanceAssignments *safe.Store[string, string]

    tasksSubmitted uint64
    tasksCompleted uint64
    tasksFailed    uint64
    tasksCancelled uint64
    tasksRejected  uint64

    ctx     context.Context
    cancel  context.CancelFunc
    wg      sync.WaitGroup
    running atomic.Bool
}
```

`Start()` uses `running.CompareAndSwap(false, true)` to detect double
Start. `Stop()` symmetric.

### Session-budget estimate

~1h.

---

## Aggregate session-budget

| Site | Hours |
|------|-------|
| FreeProviderAdapter | 2.5 |
| ProviderRegistry | 3 |
| DebateTeamConfig | 2.5 |
| CodeGraph | 3 |
| InstancePool | 2 |
| WorkerPool | 1 |
| **Total** | **14** |

## Execution order recommendation

1. **WorkerPool** first — quickest win, sets no precedent others
   depend on, tests have zero direct-field coupling.
2. **FreeProviderAdapter** — fixes a real race while migrating.
3. **DebateTeamConfig** — moderate size, public-API-only test
   surface.
4. **ProviderRegistry** — biggest caller blast radius; schedule when
   a handler test pass is possible in the same session.
5. **CodeGraph** — structural decision (Option A vs Option B) needed;
   measure before you cut.
6. **InstancePool** — last, because it is the Pattern-Zeta documented
   exception rather than a migration, and benefits from the playbook
   updates that the other sessions may trigger.

## Cross-reference

- Parent design spec: `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`.
- Concurrency playbook: `docs/development/concurrency-playbook.md`.
- Allowlist: `scripts/concurrency-audit-allowlist.txt`.
- Remaining-work inventory: `docs/development/REMAINING_WORK_2026-04-21.md`.
