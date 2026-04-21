# CONST-029 Structural Blockers — Per-Site Plan (2026-04-21)

**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-2
**Source inventory:** `docs/development/REMAINING_WORK_2026-04-21.md` §Bucket-1a
**Status:** PLAN-ONLY. No code changes in this document's commit.

## Common constraints

All sites in this document share: joint invariant across multiple
fields, OR JSON-tagged slice with live wire-format consumer, OR
both. `safe.Store` + `safe.Slice` swap alone is insufficient — the
blanket swap either loses a transactional invariant or changes the
JSON wire format in a way that cascades into 48 CLI agent configs.

Each site is therefore given a **bespoke decision** (immutable state
pointer, atomic.Uint64 + Float64bits, MarshalJSON snapshot, or
co-located map value) chosen to keep the existing external contract
intact while eliminating the bare mutex.

## Site 1: ContextWindow (internal/optimization/context/window.go:22)

### Current shape

```go
// ContextWindow manages the context window for LLM interactions.
type ContextWindow struct {
    mu           sync.RWMutex
    entries      []ContextEntry
    config       *WindowConfig
    tokenCount   int
    lastAccess   time.Time
    eventHandler WindowEventHandler
}
```

### Lock pattern

Pattern-A: `mu sync.RWMutex` guards the mutable quartet
`entries []ContextEntry`, `tokenCount int`, `lastAccess time.Time`,
and `eventHandler WindowEventHandler`. The joint invariant
`tokenCount == Σ entries[i].TokenCount` is preserved transactionally
across Add / RemoveEntry / UpdateEntry / evictFIFO / evictByPriority
/ ClearExceptPinned. 21 methods take `w.mu.Lock()` or `w.mu.RLock()`.

### Touch-point census

- Source-file `w.(mu|tokenCount|entries|lastAccess|eventHandler)`
  accesses: **79** (via `grep -cE` against `window.go`).
- Test-file `.(mu|tokenCount|entries|lastAccess|eventHandler)`
  accesses in `internal/optimization/context/window_test.go`: **22**.
- External callers creating a `ContextWindow{}` literal outside
  `internal/optimization/context/`: **0** (the package exports
  `NewContextWindow`; consumers go through the constructor).
- Reachable from public endpoints: indirectly via
  `internal/optimization/optimizer.go` (which composes the window
  into the Optimizer facade); no direct public `/v1` surface.

### Decision

**Single `atomic.Pointer[windowState]` pointing at an immutable
`*windowState`.** `eventHandler` is hoisted out to a dedicated
`atomic.Pointer[WindowEventHandler]` (cold-path, no need to
re-allocate state on handler swap). `config *WindowConfig` stays
on the outer struct (constructor-set, read-only).

### Rationale

Separate `safe.Store` (for entries keyed by ID) + `atomic.Int64`
(for tokenCount) loses the joint invariant: a reader observing the
pair mid-flight can see a tokenCount that does not match the
summed entries. The immutable-state CAS-loop preserves
`tokenCount == Σ entries[i].TokenCount` structurally because every
writer produces a fresh `*windowState` that is already consistent
before the `CompareAndSwap`. There is no mutex to forget, because
the state is a single atomic pointer swap. JSON wire format is
preserved (the `[]ContextEntry` and `WindowSnapshot.TokenCount`
fields marshal exactly as before — `Snapshot()` simply becomes
`w.state.Load()` plus a shallow copy).

### Migration sketch

```go
type windowState struct {
    entries    []ContextEntry // treated as immutable after publication
    tokenCount int
    lastAccess time.Time
}

type ContextWindow struct {
    state        atomic.Pointer[windowState]
    config       *WindowConfig             // constructor-set, RO
    eventHandler atomic.Pointer[WindowEventHandler]
}

// Add is a CAS-loop producing a new *windowState.
func (w *ContextWindow) Add(entry ContextEntry) error {
    // ... entry normalization (ID, Timestamp, TokenCount) ...
    for {
        prev := w.state.Load()
        nextTokens := prev.tokenCount + entry.TokenCount
        availableTokens := w.config.MaxTokens - w.config.ReserveTokens

        nextEntries := prev.entries
        if nextTokens > availableTokens {
            evicted, newEntries, ok := w.planEviction(prev, entry.TokenCount)
            if !ok {
                w.emitEvent(EventTypeOverflow, nil)
                return ErrContextOverflow
            }
            nextEntries = newEntries
            nextTokens = prev.tokenCount - evicted + entry.TokenCount
        }

        next := &windowState{
            entries:    append(append(make([]ContextEntry, 0, len(nextEntries)+1),
                         nextEntries...), entry),
            tokenCount: nextTokens,
            lastAccess: time.Now(),
        }
        if w.state.CompareAndSwap(prev, next) {
            w.emitEvent(EventTypeEntryAdded, &entry)
            return nil
        }
    }
}
```

Reader methods (`Get`, `GetMessages`, `TokenCount`, `AvailableTokens`,
`UsageRatio`, `Stats`, `Snapshot`) all collapse to a single
`snap := w.state.Load()` at the top and consult `snap.entries` /
`snap.tokenCount` without any lock.

### Test impact

`internal/optimization/context/window_test.go` (22 field-access
sites) — every direct `w.entries` / `w.tokenCount` read in white-box
assertions becomes `w.state.Load().entries` / `.tokenCount`. Setup
helpers that push entries via exported `Add` / `AddMessage` are
unaffected. The `windowState` type stays package-private, so no
downstream `internal/optimization/optimizer.go` test changes.

### Session-budget estimate

~2h. The 79 source-site rewrites (mostly reader methods collapsing
to a single Load()), plus 22 test white-box fixups, plus a
`planEviction` helper extraction so the CAS body stays readable.

---

## Site 2: SemanticCache (internal/optimization/gptcache/semantic_cache.go:50)

### Current shape

```go
// SemanticCache provides semantic similarity-based caching for LLM queries.
type SemanticCache struct {
    mu sync.RWMutex

    // Storage
    entries    map[string]*CacheEntry // ID -> Entry
    embeddings [][]float64            // Ordered embeddings for similarity search
    entryIDs   []string               // Ordered entry IDs matching embeddings

    // Eviction
    eviction EvictionStrategy

    // Configuration
    config *Config

    // Statistics
    hits            int64
    misses          int64
    totalSimilarity float64
    hitCount        int64

    // Callbacks
    onEvict func(entry *CacheEntry)
}
```

### Lock pattern

Pattern-A: `mu` guards `entries map[string]*CacheEntry` + the
**positionally-paired** parallel slices `embeddings [][]float64`
and `entryIDs []string` (index `i` in both must always refer to
the same entry). The four int64/float64 statistics counters
(`hits`, `misses`, `totalSimilarity`, `hitCount`) also live under
the same mu.

### Touch-point census

- Source-file `c.(mu|entries|embeddings|entryIDs|hits|misses|
  totalSimilarity|hitCount|eviction|onEvict)` accesses: **84**
  (via `grep -cE` against `semantic_cache.go`).
- Test-file equivalent in `internal/optimization/gptcache/
  semantic_cache_test.go`: **1** white-box access (tests are almost
  entirely black-box against exported methods — a major accelerant).
- External consumers: `internal/optimization/optimizer.go` at
  lines 28 (field decl) and 75 (`gptcache.NewSemanticCache(...)` call);
  all access goes through exported `Get` / `Set` / `Stats` / `Clear`.

### Decision

**Primary: co-locate embedding with map value.** Replace the
positional-lockstep trio (`entries` map, `embeddings` slice,
`entryIDs` slice) with a single `*safe.Store[string, cacheEntry]`
where `cacheEntry` carries both the `*CacheEntry` and its
embedding. Similarity search iterates via `Store.Range` building
a transient `[]float64` slice for `FindMostSimilar`. Statistics
counters (`hits`, `misses`, `hitCount`) → `atomic.Int64`;
`totalSimilarity` → `atomic.Uint64` holding `math.Float64bits`
(same pattern as Site 3). `onEvict` callback → `atomic.Pointer`.

**Fallback (if Range-per-Get proves too expensive):** keep the
three fields but wrap in a private `innerState` struct behind a
single `atomic.Pointer`. Re-evaluate after microbenchmark; the
Range cost is O(N) which matches the existing linear scan today,
so the primary path is expected to dominate.

### Rationale

The current `entries` / `embeddings` / `entryIDs` trio has a
structural invariant (`len(embeddings) == len(entryIDs) ==
len(entries)`, and `entries[entryIDs[i]].Embedding == embeddings[i]`)
that `safe.Store` + `safe.Slice` cannot express — the two
containers do not share a transactional boundary. Co-location
collapses the invariant: the entry and its embedding live in the
same map value, so the invariant is "a map value is self-consistent,"
which `safe.Store` enforces. Atomic counters for statistics match
the pattern already approved for Phase-3 hot-path guardrails
(CLAUDE.md §Phase-3 hot-path memory-safety). Wire format is
preserved: `CacheEntry.Embedding []float64 \`json:"embedding"\``
marshal continues to work because the embedding still lives on the
entry itself.

### Migration sketch

```go
type cacheSlot struct {
    entry     *CacheEntry
    embedding []float64 // kept alongside entry for o(1) similarity lookup
}

type SemanticCache struct {
    entries *safe.Store[string, cacheSlot]
    eviction EvictionStrategy    // EvictionStrategy is itself concurrent-safe
    config   *Config

    hits            atomic.Int64
    misses          atomic.Int64
    hitCount        atomic.Int64
    totalSimilarity atomic.Uint64 // math.Float64bits storage

    onEvict atomic.Pointer[func(entry *CacheEntry)]
}

func (c *SemanticCache) GetWithThreshold(
    ctx context.Context, embedding []float64, threshold float64,
) (*CacheHit, error) {
    // Snapshot all slots into a transient slice for FindMostSimilar.
    type indexedSlot struct {
        id    string
        slot  cacheSlot
    }
    var snap []indexedSlot
    var embs [][]float64
    c.entries.Range(func(id string, slot cacheSlot) bool {
        snap = append(snap, indexedSlot{id, slot})
        embs = append(embs, slot.embedding)
        return true
    })
    if len(embs) == 0 {
        c.misses.Add(1)
        return nil, ErrCacheMiss
    }
    bestIdx, bestScore := FindMostSimilar(embedding, embs, c.config.SimilarityMetric)
    if bestIdx < 0 || bestScore < threshold {
        c.misses.Add(1)
        return nil, ErrCacheMiss
    }
    hit := snap[bestIdx]
    // Update entry via Store.Update so access metadata is atomic.
    c.entries.Update(hit.id, func(cur cacheSlot, ok bool) (cacheSlot, bool) {
        if !ok {
            return cur, false
        }
        cur.entry.AccessedAt = time.Now()
        cur.entry.AccessCount++
        return cur, true
    })
    c.eviction.UpdateAccess(hit.id)
    c.hits.Add(1)
    c.hitCount.Add(1)
    // totalSimilarity += bestScore via Float64bits CAS loop
    for {
        prev := c.totalSimilarity.Load()
        next := math.Float64bits(math.Float64frombits(prev) + bestScore)
        if c.totalSimilarity.CompareAndSwap(prev, next) { break }
    }
    return &CacheHit{Entry: hit.slot.entry, Similarity: bestScore}, nil
}
```

### Test impact

`internal/optimization/gptcache/semantic_cache_test.go` has only
**1** white-box field access (negligible), so the migration lands
almost entirely behind the exported API. `optimizer.go` at lines
28/75 requires no change — same constructor + same exported
methods. `eviction_test.go` / `types_test.go` are orthogonal (they
test the eviction strategy and type helpers, not the cache
internals).

### Session-budget estimate

~2.5h. Source is dense (84 touches, multiple write-paths:
`Set`, `SetWithID`, `Remove`, `removeByIDLocked`, `Clear`,
`Invalidate`), but tests are black-box so the risk surface is
contained. The extra 0.5h over Site 1 is for cross-referencing
the four stats counters' Float64bits CAS loop for correctness.

---

## Site 3: MCTSNode (internal/planning/mcts.go:27)

### Current shape

```go
// MCTSNode represents a node in the MCTS tree
type MCTSNode struct {
    ID          string                 `json:"id"`
    ParentID    string                 `json:"parent_id,omitempty"`
    State       interface{}            `json:"state"`
    Action      string                 `json:"action,omitempty"`
    Visits      int                    `json:"visits"`
    TotalReward float64                `json:"total_reward"`
    Children    []*MCTSNode            `json:"children,omitempty"`
    NodeState   MCTSNodeState          `json:"node_state"`
    Depth       int                    `json:"depth"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    CreatedAt   time.Time              `json:"created_at"`
    mu          sync.RWMutex
}
```

### Lock pattern

Pattern-A under tree recursion. `Visits int` and `TotalReward
float64` are mutated from `AddReward`, read from `AverageReward`,
`UCTValue`, `selectBestChild`, and backpropagated concurrently
when `EnableParallel=true` in `MCTSConfig`. Both fields are
JSON-tagged and must serialise via the public
`/v1/planning/mcts` endpoint (see `MCTSResult.MarshalJSON` at
`internal/planning/mcts.go:501`, which delegates to the struct tags
via `Alias`).

### Touch-point census

- Source-file `TotalReward|Visits` accesses in `mcts.go`: **20**
  (via `grep -cE`).
- Source-file node-mu accesses: **31** touches across
  `n.mu|child.mu|current.mu|node.mu`.
- Test-file `TotalReward|\.Visits` accesses:
  - `internal/planning/planning_test.go`: **5**.
  - `internal/planning/planning_extended_test.go`: **6**.
- External references to `MCTSNode` outside
  `internal/planning/`: **0** (public surface is `MCTSResult`,
  which holds `[]*MCTSNode` inside `BestPath` — the JSON wire
  format on `total_reward` and `visits` is therefore
  contract-bound).

### Decision

`TotalReward float64` → `totalReward atomic.Uint64` holding
`math.Float64bits`. `Visits int` → `visits atomic.Int64`. The
rest of the struct becomes a single-writer field set during
`expand()` (children append, NodeState transition) — this is
covered by the existing `n.mu.Lock()` in `expand` where the
writer is unique. `Children`, `NodeState`, `Metadata`, `Depth`,
`State`, `Action`, `ID`, `ParentID`, `CreatedAt` remain plain
fields guarded by the existing `mu` (kept **only** for the
expand-writer path — readers no longer acquire it).

Custom `MarshalJSON` / `UnmarshalJSON` on `MCTSNode` restores
the existing wire format:

```go
func (n *MCTSNode) MarshalJSON() ([]byte, error) {
    type alias MCTSNode
    return json.Marshal(&struct {
        Visits      int64   `json:"visits"`
        TotalReward float64 `json:"total_reward"`
        *alias
    }{
        Visits:      n.visits.Load(),
        TotalReward: n.GetTotalReward(),
        alias:       (*alias)(n),
    })
}
```

**Caveat — residual `mu`:** this site deliberately keeps `mu` for
the expand-writer / children-append path rather than going full
`atomic.Pointer[nodeState]`, because `MCTSNode` is a tree node with
recursive pointers — a CAS swap on a parent's `state` would require
republishing the child set under a new pointer, cascading up to the
root on every expand. The CONST-029 allowlist entry remains for
this one field (with a dedicated justification comment); the
hot-path race (TotalReward / Visits under `-race` in parallel
rollouts) is fully eliminated by the atomics.

### Rationale

JSON surface is public (`MCTSResult.BestPath []*MCTSNode` serialises
via the shared struct tags). Changing field type from `float64` to
`atomic.Uint64` breaks the default marshaller; the custom
`MarshalJSON` method restores the wire contract 1:1. The
`atomic.Uint64 + math.Float64bits` pattern is the standard
lock-free double-accumulator idiom and is already approved for
`totalSimilarity` in Site 2. Keeping `mu` for the expand-writer
path is a conscious scope cut — the hot-path data race
(parallel backpropagation incrementing `TotalReward` / `Visits`)
is what `-race` actually catches.

### Migration sketch

```go
type MCTSNode struct {
    ID          string                 `json:"id"`
    ParentID    string                 `json:"parent_id,omitempty"`
    State       interface{}            `json:"state"`
    Action      string                 `json:"action,omitempty"`
    visits      atomic.Int64           // was: Visits int `json:"visits"`
    totalReward atomic.Uint64          // was: TotalReward float64; math.Float64bits
    Children    []*MCTSNode            `json:"children,omitempty"` // expand-writer only
    NodeState   MCTSNodeState          `json:"node_state"`          // expand-writer only
    Depth       int                    `json:"depth"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    CreatedAt   time.Time              `json:"created_at"`
    mu          sync.Mutex             // retained for expand/children append only
}

func (n *MCTSNode) GetTotalReward() float64 {
    return math.Float64frombits(n.totalReward.Load())
}

func (n *MCTSNode) AddReward(reward float64) {
    n.visits.Add(1)
    for {
        prev := n.totalReward.Load()
        next := math.Float64bits(math.Float64frombits(prev) + reward)
        if n.totalReward.CompareAndSwap(prev, next) { return }
    }
}

func (n *MCTSNode) AverageReward() float64 {
    v := n.visits.Load()
    if v == 0 { return 0 }
    return n.GetTotalReward() / float64(v)
}

func (n *MCTSNode) MarshalJSON() ([]byte, error) {
    type alias MCTSNode
    return json.Marshal(&struct {
        Visits      int64   `json:"visits"`
        TotalReward float64 `json:"total_reward"`
        *alias
    }{
        Visits:      n.visits.Load(),
        TotalReward: n.GetTotalReward(),
        alias:       (*alias)(n),
    })
}
```

### Test impact

`planning_test.go` (5 accesses) and `planning_extended_test.go`
(6 accesses) need white-box reads rewritten:
`node.Visits` → `node.visits.Load()` (or a `Visits()` method if
we want to keep tests cleaner); `node.TotalReward` →
`node.GetTotalReward()`. Since `Visits`/`TotalReward` are no longer
exported fields, either adding thin accessor methods or renaming
the test helpers is the right call — accessor is cleaner and
preserves API parity. Wire-format serialisation tests are
unaffected because `MarshalJSON` replays the public names.

### Session-budget estimate

~2h. Accessor-method extraction + 11 test-file rewrites +
`MarshalJSON` authoring + `-race` verification on a parallel MCTS
search.

---

## Site 4: DiscoveredProvider (internal/services/provider_discovery.go:79)

### Current shape

```go
// DiscoveredProvider represents a provider discovered from environment
type DiscoveredProvider struct {
    mu             sync.RWMutex                 // Protects Status, Verified, VerifiedAt, Error, Score fields
    Name           string                       `json:"name"`
    Type           string                       `json:"type"`
    APIKeyEnvVar   string                       `json:"api_key_env_var"`
    APIKey         string                       `json:"-"` // Hidden in JSON
    BaseURL        string                       `json:"base_url"`
    DefaultModel   string                       `json:"default_model"`
    Provider       llm.LLMProvider              `json:"-"`
    Status         ProviderHealthStatus         `json:"status"`
    Score          float64                      `json:"score"`
    Verified       bool                         `json:"verified"`
    VerifiedAt     time.Time                    `json:"verified_at,omitempty"`
    Error          string                       `json:"error,omitempty"`
    Capabilities   *models.ProviderCapabilities `json:"capabilities,omitempty"`
    SupportsModels []string                     `json:"supported_models,omitempty"`
    VerifiedModels []VerifiedModel              `json:"verified_models,omitempty"`
}
```

### Lock pattern

`mu sync.RWMutex` explicitly documented as guarding `Status`,
`Verified`, `VerifiedAt`, `Error`, `Score`. In practice
`SupportsModels []string` and `VerifiedModels []VerifiedModel`
(both JSON-tagged) are also updated via in-place field writes
during verification runs and read under `c.JSON(...)` over the
`/v1/discovery` / `/v1/scoring` / `/v1/verification` endpoints.
The enclosing `ProviderDiscovery` struct (line 61) has **already
been drained** to `safe.Store` (commit `139708c2`) — only
`DiscoveredProvider` itself still carries Pattern-A.

### Touch-point census

- Source-file `provider\.mu|d\.mu|dp\.mu|p\.mu|.VerifiedModels|
  .SupportsModels|.Verified\b|.VerifiedAt|.Score\b|.Error\b|
  .Status\b` accesses: **84** across `provider_discovery.go`.
- `DiscoveredProvider{` / `&DiscoveredProvider` creation sites: **4**.
- `provider_discovery_test.go` accesses: **4**
  (field-reads on the struct, mostly `p.Verified`/`p.Score`).
- External `DiscoveredProvider` references outside
  `internal/services/provider_discovery`: **0** (search confirmed
  no matches). The type is consumed internally via
  `ProviderDiscovery.providers *safe.Store[string,
  *DiscoveredProvider]` and serialised through the handlers.

### Decision

**Option (a): MarshalJSON-snapshot.** Replace `SupportsModels
[]string` and `VerifiedModels []VerifiedModel` with
`supportsModels *safe.Slice[string]` and `verifiedModels
*safe.Slice[VerifiedModel]`. Replace the scalar cluster
(`Status`, `Verified`, `VerifiedAt`, `Error`, `Score`) with a
single `atomic.Pointer[verificationState]` (immutable struct
carrying all five fields). Add a custom `MarshalJSON` on
`DiscoveredProvider` that reconstructs the public JSON shape.

### Rationale

Wire format is stable and consumed by 48 CLI agent configs
(`configs/cli-agents/`) plus the `/v1/discovery` /
`/v1/scoring` / `/v1/verification` REST endpoints. A custom
marshaller is ~30 lines and keeps the public contract pinned.
A full state-pointer refactor is **not** used here because the
enclosing `ProviderDiscovery` was recently drained
(commit `139708c2`) — re-opening that file to thread
`atomic.Pointer[discoveredProviderState]` through the
`providers *safe.Store` value type would re-introduce the
churn we just retired. Scalar cluster → single
`atomic.Pointer[verificationState]` is the minimum needed to
make the Status/Verified/Score triple transactionally consistent
(a reader must never see `Verified=true` with `Status=unhealthy`,
which the current Pattern-A guarantees and which individual
atomics would not).

### Migration sketch

```go
type verificationState struct {
    Status     ProviderHealthStatus
    Score      float64
    Verified   bool
    VerifiedAt time.Time
    Error      string
}

type DiscoveredProvider struct {
    Name         string          `json:"name"`
    Type         string          `json:"type"`
    APIKeyEnvVar string          `json:"api_key_env_var"`
    APIKey       string          `json:"-"`
    BaseURL      string          `json:"base_url"`
    DefaultModel string          `json:"default_model"`
    Provider     llm.LLMProvider `json:"-"`

    verification   atomic.Pointer[verificationState]
    capabilities   atomic.Pointer[models.ProviderCapabilities]
    supportsModels *safe.Slice[string]
    verifiedModels *safe.Slice[VerifiedModel]
}

func (d *DiscoveredProvider) MarshalJSON() ([]byte, error) {
    vs := d.verification.Load()
    if vs == nil { vs = &verificationState{} }
    caps := d.capabilities.Load()

    return json.Marshal(&struct {
        Name           string                       `json:"name"`
        Type           string                       `json:"type"`
        APIKeyEnvVar   string                       `json:"api_key_env_var"`
        BaseURL        string                       `json:"base_url"`
        DefaultModel   string                       `json:"default_model"`
        Status         ProviderHealthStatus         `json:"status"`
        Score          float64                      `json:"score"`
        Verified       bool                         `json:"verified"`
        VerifiedAt     time.Time                    `json:"verified_at,omitempty"`
        Error          string                       `json:"error,omitempty"`
        Capabilities   *models.ProviderCapabilities `json:"capabilities,omitempty"`
        SupportsModels []string                     `json:"supported_models,omitempty"`
        VerifiedModels []VerifiedModel              `json:"verified_models,omitempty"`
    }{
        Name:           d.Name,
        Type:           d.Type,
        APIKeyEnvVar:   d.APIKeyEnvVar,
        BaseURL:        d.BaseURL,
        DefaultModel:   d.DefaultModel,
        Status:         vs.Status,
        Score:          vs.Score,
        Verified:       vs.Verified,
        VerifiedAt:     vs.VerifiedAt,
        Error:          vs.Error,
        Capabilities:   caps,
        SupportsModels: d.supportsModels.Snapshot(),
        VerifiedModels: d.verifiedModels.Snapshot(),
    })
}
```

### Test impact

`internal/services/provider_discovery_test.go` (4 field-access
sites) — field reads like `p.Verified` and `p.Score` change to
`p.verification.Load().Verified` / `.Score`. Tests that
serialise via `json.Marshal(p)` are unaffected (the custom
`MarshalJSON` preserves the wire shape). HTTP handler tests over
`/v1/discovery` continue to pass because `c.JSON(http.StatusOK,
provider)` calls `MarshalJSON`.

### Session-budget estimate

~1.5h. The heaviest work is refactoring the verification writer
paths (`verifyProvider`, `refreshScores`, etc.) to construct a
fresh `verificationState` and CAS-swap, plus `supportsModels` /
`verifiedModels` migrations to `safe.Slice.Replace` /
`safe.Slice.Append`. Tests are minimal.

---

## Site 5: AgentTeam (internal/handlers/extended/ensemble.go:26)

### Current shape

```go
// Team represents a team of agents (inspired by claude-code-source Team tools)
type AgentTeam struct {
    ID          string          `json:"id"`
    Name        string          `json:"name"`
    Description string          `json:"description,omitempty"`
    LeaderID    string          `json:"leader_id"`
    MemberIDs   []string        `json:"member_ids"`
    Config      AgentTeamConfig `json:"config"`
    Status      TeamStatus      `json:"status"`
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
    mu          sync.RWMutex    `json:"-"`
}

// MarshalJSON takes the read lock before serialising so concurrent
// updates to any team field ... do not race with encoding/json.
func (t *AgentTeam) MarshalJSON() ([]byte, error) {
    t.mu.RLock()
    defer t.mu.RUnlock()
    type teamAlias AgentTeam
    alias := (*teamAlias)(t)
    return json.Marshal(alias)
}
```

### Lock pattern

`mu sync.RWMutex` already pairs with an existing `MarshalJSON`
method that takes `t.mu.RLock()` before serialising — documented
as a BUGFIX #28 fix. The mu is the exact structural blocker
CONST-029 retires.

### Touch-point census

- `ensemble.go` — `team\.mu|task\.mu|team\.Name|team\.MemberIDs|
  team\.Status|task\.Status|task\.Title` accesses: **29**.
- `ensemble.go` — `AgentTeam|\bTask\b` references (type/literal/
  method/MarshalJSON): **40**.
- `internal/handlers/extended/extended_test.go` — `AgentTeam|
  \bTask\b|ExtendedPlanModeSession|PlanModeSession`: **29**.
  HTTP handler tests call `c.JSON(team)` through the test router,
  so wire format matters.

### Decision

**Option (b): state-pointer refactor.** `state atomic.Pointer
[teamState]` carrying all mutable fields. Existing `MarshalJSON`
body simplifies from "RLock + Marshal alias" to "Load + Marshal
snapshot struct."

### Rationale

`AgentTeam` already owns a `MarshalJSON` method. Migrating the
body from mutex-based to state-pointer-based gives a pure
simplification (no new snapshot-assembly logic; the snapshot is
the atomically-loaded state). This is strictly cleaner than
option (a) because option (a) would require the snapshot step
inside MarshalJSON in addition to the atomic fields. The mu
retires in full; no allowlist entry needed.

### Migration sketch

```go
type teamState struct {
    Name        string
    Description string
    LeaderID    string
    MemberIDs   []string
    Config      AgentTeamConfig
    Status      TeamStatus
    UpdatedAt   time.Time
}

type AgentTeam struct {
    ID        string    // immutable after construction
    CreatedAt time.Time // immutable after construction
    state     atomic.Pointer[teamState]
}

func (t *AgentTeam) MarshalJSON() ([]byte, error) {
    s := t.state.Load()
    return json.Marshal(&struct {
        ID          string          `json:"id"`
        Name        string          `json:"name"`
        Description string          `json:"description,omitempty"`
        LeaderID    string          `json:"leader_id"`
        MemberIDs   []string        `json:"member_ids"`
        Config      AgentTeamConfig `json:"config"`
        Status      TeamStatus      `json:"status"`
        CreatedAt   time.Time       `json:"created_at"`
        UpdatedAt   time.Time       `json:"updated_at"`
    }{
        ID: t.ID, Name: s.Name, Description: s.Description,
        LeaderID: s.LeaderID, MemberIDs: s.MemberIDs,
        Config: s.Config, Status: s.Status,
        CreatedAt: t.CreatedAt, UpdatedAt: s.UpdatedAt,
    })
}

// UpdateTeam handler applies all patches via a single CAS-loop:
func (h *EnsembleHandlerExtensions) applyTeamUpdate(
    t *AgentTeam, req UpdateTeamRequest,
) {
    for {
        prev := t.state.Load()
        next := *prev // shallow copy (MemberIDs slice is immutable after publication)
        if req.Name != "" { next.Name = req.Name }
        if req.Description != "" { next.Description = req.Description }
        if req.LeaderID != "" { next.LeaderID = req.LeaderID }
        if req.MemberIDs != nil {
            next.MemberIDs = append([]string(nil), req.MemberIDs...)
        }
        if req.Config != nil { next.Config = *req.Config }
        if req.Status != "" { next.Status = TeamStatus(req.Status) }
        next.UpdatedAt = time.Now()
        if t.state.CompareAndSwap(prev, &next) { return }
    }
}
```

### Test impact

`extended_test.go` (29 accesses covering both `AgentTeam` and
`Task` and the plan session) — HTTP handler tests use the real
gin router, so they exercise `c.JSON(team)` and assert against
the JSON body. The custom `MarshalJSON` preserves the wire shape
1:1, so those assertions keep passing. White-box field reads
like `team.Name` become `team.state.Load().Name` (or a
`team.Snapshot()` helper if we want cleanness).

### Session-budget estimate

~2h. Dense handler update paths (`UpdateTeam`, `DeleteTeam`
active-task check — the latter reads `task.Status` via
`h.tasks.Range` and stays OK under the parallel Task migration).

---

## Site 6: Task (internal/handlers/extended/ensemble.go:84)

### Current shape

```go
// Task represents a task assigned to agents
type Task struct {
    ID           string          `json:"id"`
    TeamID       string          `json:"team_id,omitempty"`
    AssigneeID   string          `json:"assignee_id,omitempty"`
    CreatorID    string          `json:"creator_id"`
    Title        string          `json:"title"`
    Description  string          `json:"description"`
    Type         string          `json:"type"`
    Status       AgentTaskStatus `json:"status"`
    Priority     TaskPriority    `json:"priority"`
    Dependencies []string        `json:"dependencies"`
    Subtasks     []Subtask       `json:"subtasks"`
    Result       *TaskResult     `json:"result,omitempty"`
    CreatedAt    time.Time       `json:"created_at"`
    StartedAt    *time.Time      `json:"started_at,omitempty"`
    CompletedAt  *time.Time      `json:"completed_at,omitempty"`
    Deadline     *time.Time      `json:"deadline,omitempty"`
    Metadata     TaskMetadata    `json:"metadata"`
    mu           sync.RWMutex    `json:"-"`
}

func (t *Task) MarshalJSON() ([]byte, error) {
    t.mu.RLock()
    defer t.mu.RUnlock()
    type taskAlias Task
    alias := (*taskAlias)(t)
    return json.Marshal(alias)
}
```

### Lock pattern

Identical to AgentTeam: `mu sync.RWMutex` paired with an
existing MarshalJSON under RLock. Mutators are
`UpdateTask`, `StopTask`, and `GetTaskOutput` (the last reads
`task.Result` under RLock).

### Touch-point census

- Shared with AgentTeam in `ensemble.go`: **29** mu-related
  accesses, **40** type references — the counts cover both
  structs because they co-inhabit the file.
- `extended_test.go`: **29** shared with AgentTeam and session.

### Decision

**Option (b): state-pointer refactor**, same shape as AgentTeam.
`atomic.Pointer[taskState]` with all mutable fields.

### Rationale

Identical justification to Site 5 — existing MarshalJSON owner,
no cross-entity invariant, clean migration.

### Migration sketch

```go
type taskState struct {
    TeamID       string
    AssigneeID   string
    Title        string
    Description  string
    Type         string
    Status       AgentTaskStatus
    Priority     TaskPriority
    Dependencies []string
    Subtasks     []Subtask
    Result       *TaskResult
    StartedAt    *time.Time
    CompletedAt  *time.Time
    Deadline     *time.Time
    Metadata     TaskMetadata
}

type Task struct {
    ID        string    // immutable after construction
    CreatorID string    // immutable after construction
    CreatedAt time.Time // immutable after construction
    state     atomic.Pointer[taskState]
}

func (t *Task) MarshalJSON() ([]byte, error) {
    s := t.state.Load()
    return json.Marshal(&struct {
        // ... (full field list with json tags) ...
    }{ /* ID from t, everything else from s */ })
}
```

`UpdateTask` + `StopTask` handlers collapse their individual
`task.mu.Lock() / set / Unlock()` blocks into a single CAS loop
building a new `taskState`. The status-transition bookkeeping
(setting `StartedAt` / `CompletedAt` on specific transitions)
stays inside the CAS body.

### Test impact

Shared with Site 5 — `extended_test.go` HTTP-level tests are
wire-contract preserving. White-box task field reads rewritten
to `task.state.Load().Field`.

### Session-budget estimate

~2h. Task has more fields than AgentTeam (13 mutable vs 7) but
the mechanical pattern is identical, so the time is dominated by
the number of handler sites touching it.

---

## Site 7: ExtendedPlanModeSession (internal/handlers/extended/planning.go:24)

### Current shape

```go
type ExtendedPlanModeSession struct {
    ID              string               `json:"id"`
    UserID          string               `json:"user_id"`
    Objective       string               `json:"objective"`
    Context         []string             `json:"context"`
    Steps           []PlanStep           `json:"steps"`
    CurrentStepIdx  int                  `json:"current_step_idx"`
    Status          PlanModeStatus       `json:"status"`
    CreatedAt       time.Time            `json:"created_at"`
    UpdatedAt       time.Time            `json:"updated_at"`
    CompletedAt     *time.Time           `json:"completed_at,omitempty"`
    AutoExecute     bool                 `json:"auto_execute"`
    ExecutionResult *PlanExecutionResult `json:"execution_result,omitempty"`
    mu              sync.RWMutex
}
```

### Lock pattern

`mu sync.RWMutex` guards most mutable fields. Unlike Sites 5/6,
this struct does **not** currently own a `MarshalJSON` — so the
default marshaller races with in-flight updates during a
`GetPlanStatus` handler concurrent with `UpdatePlan` /
`ExecutePlan` / `PausePlan`. The current `GetPlanStatus`
acquires `RLock` around the `c.JSON(session)` call — this is the
same "handler holds mu across serialisation" pattern AgentTeam/
Task already moved past via BUGFIX #28. The CONST-029 migration
must close this gap.

### Touch-point census

- `planning.go` `session\.mu|session\.Status|session\.Steps|
  session\.UpdatedAt|session\.CurrentStepIdx` accesses: **42**.
- `planning.go` `PlanModeSession|ExtendedPlanModeSession` type
  references: **12**.
- `extended_test.go` shared: **29** covering all three
  extended types.

### Decision

**Option (b): state-pointer refactor.** `atomic.Pointer
[sessionState]` plus a new `MarshalJSON` on
`ExtendedPlanModeSession` that replays the wire shape — this
closes the BUGFIX-#28-equivalent serialisation race as a
side-effect of the migration.

### Rationale

Same as Sites 5/6 with one additional win: adding the
`MarshalJSON` method that simply does `s := t.state.Load();
Marshal(...)` kills the handler-holds-mu-across-c.JSON hazard
that Sites 5/6 already closed. Consistency across the three
extended-handler structs.

### Migration sketch

```go
type sessionState struct {
    Objective       string
    Context         []string
    Steps           []PlanStep
    CurrentStepIdx  int
    Status          PlanModeStatus
    UpdatedAt       time.Time
    CompletedAt     *time.Time
    AutoExecute     bool
    ExecutionResult *PlanExecutionResult
}

type ExtendedPlanModeSession struct {
    ID        string    // immutable
    UserID    string    // immutable
    CreatedAt time.Time // immutable
    state     atomic.Pointer[sessionState]
}

func (s *ExtendedPlanModeSession) MarshalJSON() ([]byte, error) {
    st := s.state.Load()
    return json.Marshal(&struct {
        ID              string               `json:"id"`
        UserID          string               `json:"user_id"`
        Objective       string               `json:"objective"`
        Context         []string             `json:"context"`
        Steps           []PlanStep           `json:"steps"`
        CurrentStepIdx  int                  `json:"current_step_idx"`
        Status          PlanModeStatus       `json:"status"`
        CreatedAt       time.Time            `json:"created_at"`
        UpdatedAt       time.Time            `json:"updated_at"`
        CompletedAt     *time.Time           `json:"completed_at,omitempty"`
        AutoExecute     bool                 `json:"auto_execute"`
        ExecutionResult *PlanExecutionResult `json:"execution_result,omitempty"`
    }{
        ID: s.ID, UserID: s.UserID, CreatedAt: s.CreatedAt,
        Objective: st.Objective, Context: st.Context, Steps: st.Steps,
        CurrentStepIdx: st.CurrentStepIdx, Status: st.Status,
        UpdatedAt: st.UpdatedAt, CompletedAt: st.CompletedAt,
        AutoExecute: st.AutoExecute, ExecutionResult: st.ExecutionResult,
    })
}
```

`UpdatePlan`, `ExecutePlan`, `PausePlan`, `executePlanSession`,
`GetPlanStatus`, `ExitPlanMode` handlers all migrate from
`session.mu.Lock()` / field mutation / `Unlock()` to a CAS-loop
building a new `sessionState`. The `executePlanSession`
background goroutine has per-step `mu.Lock/Unlock` pairs that
collapse to per-step CAS loops updating `CurrentStepIdx` /
`UpdatedAt` / per-step `Status` on the step slice (the step
slice is a field on `sessionState`; mutations produce a fresh
slice).

### Test impact

Shared with Sites 5/6 (29 accesses in `extended_test.go`). HTTP
handler tests over `/api/v1/planning/plan-mode/*` exercise the
wire format via the real gin router — the new `MarshalJSON`
preserves it. White-box reads rewritten as per Sites 5/6.

### Session-budget estimate

~2h. Larger state struct (9 mutable fields) + existing
`executePlanSession` background goroutine needs careful CAS-loop
threading per step.

---

## Aggregate session-budget

| Site | Hours |
|------|-------|
| ContextWindow | 2 |
| SemanticCache | 2.5 |
| MCTSNode | 2 |
| DiscoveredProvider | 1.5 |
| AgentTeam | 2 |
| Task | 2 |
| ExtendedPlanModeSession | 2 |
| **Total** | **14** |

## Execution gating

Each site is its own dedicated session. No concurrent drains
across sites — each needs the test-rewrite volume and
`-race` verification to clear before the next. Sites 5/6
(AgentTeam + Task) may be combined into a single session if the
implementer is willing to burn the 4h in one slot (they co-inhabit
`ensemble.go` and share the same test file), but splitting is the
default. Site 3 (MCTSNode) carries a residual `mu` by design;
the allowlist entry for that one field must be retained with the
justification comment from this spec.

## Cross-reference

- Parent design: `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
- CONST-029 campaign memory: `memory/project_const029_campaign.md`
- Concurrency playbook: `docs/development/concurrency-playbook.md`
- Allowlist: `scripts/concurrency-audit-allowlist.txt`
- BUGFIX #28 (handler-holds-mu-across-c.JSON race): resolved for
  AgentTeam/Task, still open for ExtendedPlanModeSession — closed
  as side-effect of Site 7 migration
- Related drained sites (reference patterns): `ProviderDiscovery`
  at commit `139708c2`, `StartupVerifier` at commit `aa39a250`
