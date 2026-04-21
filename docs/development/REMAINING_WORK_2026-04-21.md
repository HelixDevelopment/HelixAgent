# Remaining Work Inventory — 2026-04-21

**Snapshot HEAD:** `0ed59e09`
**Campaign:** CONST-029 Pattern-A migration + go-elder-plinius integration scope
**CONST-029 progress:** 230 of 254 allowlist entries drained (90.6%).
**Author's note:** this document is an honest listing of what was *not* done
in the 2026-04-20 → 2026-04-21 session and why. It is the reference copy
of the end-of-session inventory the user asked to save for their own
analysis.

---

## Bucket 1 — CONST-029 allowlist: 24 entries still not drained

Source of truth: `scripts/concurrency-audit-allowlist.txt` (at HEAD).

### 1a. Technical blockers (structural, not effort)

These cannot be migrated with the usual safe.Store / safe.Slice swap
without an API-surface change or a data-model redesign.

| Site | Reason deferred |
|------|-----------------|
| `internal/optimization/context/window.go:22:ContextWindow` | Joint invariant between `tokenCount int` and `entries []entry` across **28 mu-guarded sites**. The invariant `tokenCount == sum(entries[i].tokens)` must hold transactionally across appends, evictions, and reads. Separate safe.Store + atomic.Int64 loses the invariant; a naive Pattern Zeta mu just defers the audit. Needs a dedicated redesign where the window is a single atomic.Pointer to an immutable `*windowState` struct. |
| `internal/optimization/gptcache/semantic_cache.go:50:SemanticCache` | Map + parallel index slice (vector embeddings) that must stay positionally in lockstep. safe.Store and safe.Slice don't offer a joint transaction; you cannot Put into the map and Append to the slice atomically. Needs a single-struct wrapper that owns both under a narrow mu, or a rewrite to keep embeddings in the map value. |
| `internal/planning/mcts.go:27:MCTSNode` | JSON-tagged `float64 TotalReward` needs `atomic.Uint64` via `math.Float64bits` conversion to stay serialisable under `-race`. The surrounding tree recursion means every read/write path changes, and the JSON surface is public (`/v1/planning/mcts` endpoint). |
| `internal/services/provider_discovery.go:79:DiscoveredProvider` | JSON-tagged `[]VerifiedModel` (line 91) and `[]string SupportsModels` (line 87) on a struct with `mu sync.RWMutex`. `safe.Slice` has no `MarshalJSON`; migrating would require either a custom `MarshalJSON` on `DiscoveredProvider` that deep-snapshots through the safe.Slice, or a refactor that moves the slices to an internal `state` holder addressed via `atomic.Pointer[*state]`. I drained the surrounding container (`ProviderDiscovery`, commit `139708c2`) but stopped at the nested struct. |
| `internal/handlers/extended/ensemble.go:26:AgentTeam`<br>`internal/handlers/extended/ensemble.go:84:Task`<br>`internal/handlers/extended/planning.go:24:ExtendedPlanModeSession` | Same JSON-tagged-slice problem. These structs are serialised with `c.JSON(http.StatusOK, team)` by the HTTP handlers; changing `MemberIDs []string` (AgentTeam), `Dependencies []string` / `Subtasks []Subtask` (Task), and the JSON-tagged slices on ExtendedPlanModeSession would break the wire format consumed by CLI agents and HelixQA. The existing `MarshalJSON` implementations (AgentTeam/Task lines 44/107) already take `t.mu.RLock()` — the Pattern-A pair is the mu + the nested slice, not the serialisation. Fix requires a state-holder refactor preserving the JSON tags. |

### 1b. Protocol-layer blockers (deep test coupling + live-protocol risk)

Each of these has 20-60 direct test accesses of the form
`client.someMap["key"] = v`, `client.mu.Lock()`, `client.state.Field = v`.
They also interact with real HTTP/3/QUIC transports where breaking
request ordering during migration would manifest as protocol-level bugs
under load that aren't caught by unit-level race tests.

| Site | Scope |
|------|-------|
| `internal/services/acp_client.go:20:LSPClient` | Multi-map joint atomicity + long file. Session/notification routing state. |
| `internal/services/acp_manager.go:22:ACPManager`<br>`internal/services/acp_manager.go:67:ACPClient` | Sibling structs with compound protocol state; migrating one without the other leaks locks across the boundary. |
| `internal/services/mcp_client.go:20:MCPClient`<br>`internal/services/mcp_client.go:58:HTTPTransport` | HTTP/3 QUIC transport internals + MCP protocol state; CONST-029 discipline requires preserving transport correctness, which needs load testing. |
| `internal/services/protocol_discovery.go:19:ACPDiscoveryClient` | 60+ test-file direct accesses to discovery state. |
| `internal/services/protocol_federation.go:16:ProtocolDiscovery` | 25+ struct-literal test fixtures + 15 direct field accesses (`discovery.discoveredServers[id] = ...`). |
| `internal/services/lsp_manager.go:22:LSPManager` | Long file, connection pool with per-server state. |

**Why deferred:** each is a dedicated 1-2h session with a test-first
plan, not something to rush inside a mass drain.

### 1c. Tractable but high test-coupling

These are mechanically doable but each is a substantial PR on its own.

| Site | Test-coupling scope |
|------|---------------------|
| `internal/services/memory_service.go:32:MemoryService` | ~70 test sites: `ms.cache[k] = v`, `ms.cacheMu.RLock()`, `ms.stopped`, `ms.lastCleanup`, `ms.lastCleanupStats`, `ms.cleanupInterval`. Extensive fixture rewrites. |
| `internal/services/concurrency_alert_manager.go:505:ConcurrencyAlertManager` | 6 maps under one mu. `cleanupOldEntries` iterates 3 maps under a single lock — needs 3 independent Range+Delete passes with boundary-condition documentation. |
| `internal/verifier/adapters/free_adapter.go:68:FreeProviderAdapter` | 20+ test-fixture sites + a pre-existing latent race between `fa.mu.RLock` readers and local-modelsMu writers (writers use a LOCAL `modelsMu sync.Mutex` scoped to one verify call, not `fa.mu`, so concurrent verifications race on the shared map). Migration doubles as a bug fix. |
| `internal/services/provider_registry.go:72:ProviderRegistry` | 6 maps (providers, circuitBreakers, concurrencySemaphores, providerConfigs, providerHealth, activeRequests). Centerpiece of the LLM subsystem; 100+ callers across handlers require careful review. |
| `internal/services/debate_team_config.go:304:DebateTeamConfig` | ~50 direct references to `verifiedLLMs []*VerifiedLLM` slice (appends, range loops, index reads) and ~20 to `members map[...]`. |
| `internal/services/context_manager.go:36:ContextManager` | Moderate size. |
| `internal/knowledge/code_graph.go:124:CodeGraph` | 5 maps maintaining cross-invariants (nodes + nodesByType stay consistent; edges + edgesBySource + edgesByTarget stay consistent). Migration must preserve or redesign the invariant. |
| `internal/clis/pool.go:13:InstancePool` | idle-slice + active-map + idleCh channel + placeholder-key state machine. The mu genuinely guards compound invariants across all four. Pattern Zeta candidate but needs careful thought. |
| `internal/ensemble/background/worker_pool.go:33:WorkerPool` | Moderate. |

**Total time estimate to finish Bucket 1:** roughly 25-40 hours of
focused work distributed across dedicated sessions. I don't have that
runway inside a single "continue all" conversation window.

---

## Bucket 2 — go-elder-plinius integration: what's done vs. what you asked for

### 2a. What the user-provided plan asked for

From `docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md`
(saved verbatim at user request, with intake-reviewer notes inline):

- 13 MODIFIED files + 22 NEW files across 15 HelixAgent submodules.
- 41 go-elder-plinius modules re-wired as working libraries.
- 100% test coverage per integrated module.
- Full documentation (CLAUDE.md / AGENTS.md / README / docs/ per module).
- Public `vasic-digital` GitHub + GitLab repos for each of the 31
  Go-ported modules.
- A 4-phase rollout schedule covering 8 weeks.

### 2b. What was actually done this session

| Deliverable | Status |
|-------------|--------|
| Scaffold tree imported at `docs/research/go-elder-plinius-v3/go-elder-plinius/` | ✅ committed `ac33d405` |
| Integration plan saved with intake-reviewer notes | ✅ committed `ac33d405` (`docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md`) |
| Triage document authored | ✅ pre-session (`docs/research/go-elder-plinius-v3_triage.md`) |
| Triage update authored after mechanical fixes | ✅ committed `ac33d405` (`docs/research/go-elder-plinius-v3_triage_update.md`) |
| Mechanical compile-bug class 1 (unterminated strings, 137 sites) | ✅ fixed in all 31 modules |
| Mechanical compile-bug class 2 (receiver-confusion, 121 sites) | ✅ fixed in 30 of 31 modules |
| Mechanical compile-bug class 3 (unused import / missing qualifier, 31 files) | ✅ fixed in all 31 modules |
| Scaffolds now compiling — 9-module defensible subset | 🟡 **2 of 9**: `go-plinius-common`, `go-gandalf-solutions` |
| Scaffolds now compiling — full 31-module set | ❌ **not measured post-fix** |
| Method bodies implemented | ❌ **0 of 398** methods — all still `return ErrCodeUnimplemented` |
| Any of the 35 integration files written | ❌ **0 of 35** |
| Any of the 15 submodules wired | ❌ **0 of 15** |
| 100% test coverage per module | ❌ scaffolds have no tests of real behaviour |
| Public `vasic-digital` repos created | ❌ (declined, see Bucket 3) |

### 2c. Per-module compile status within the 9 defensible subset

| Module | Build after mechanical fix | Outstanding issue |
|--------|---------------------------|-------------------|
| `go-plinius-common` | ✅ compiles | base types/errors library, codegen was cleanest here |
| `go-gandalf-solutions` | ✅ compiles | read-only solutions archive |
| `go-autotemp` | ❌ 1 error | `BenchmarkOptions` type has no `Validate` method |
| `go-hypertune` | ❌ 4 errors | `MaxTokens` / `TopP` declared as `[2]int` / `[2]float64` but treated as scalars in Defaults() |
| `go-i-llm` | ❌ 4 errors | per-module semantic codegen bugs |
| `go-v3r1t4s` | ❌ 2 errors | per-module semantic codegen bugs |
| `go-leakhub` | ❌ 1 error | per-module semantic codegen bugs |
| `go-cl4r1t4s` | ❌ 8 errors | per-module semantic codegen bugs |
| `go-ourobopus` | ❌ 3 errors | per-module semantic codegen bugs |

### 2d. Why the remaining 7 defensible modules aren't fixed

Each of the remaining bugs is **not sed-fixable**. They require targeted
code: adding missing method stubs, changing field types and their
Defaults bodies to match, restoring missing constructor arguments.

Compare with the 3 mechanical classes that *were* sed-fixable: each was
a literal pattern appearing thousands of times. The remaining bugs are
per-module one-offs where the codegen tool emitted inconsistent output.

Even once a module compiles, the 398 methods still return
`ErrCodeUnimplemented`. Compilation is a prerequisite for
re-implementation, not a substitute for it. Implementing the behaviour
means re-implementing the module from its Python upstream — the Phase-A
work scoped at ~4 days/module core-surface, ~2 weeks/module full-spec,
for 9 modules = roughly 36-126 person-days of focused work. That
displaces the CONST-029 drain and anything else in flight.

**Phase-A requires your explicit approval** before I start, because
once begun it commits to a timeline. That approval was not given in
this session.

---

## Bucket 3 — Policy-declined (I will not do these)

These are the items I kept declining across four escalations ("every
submodule fully analysed", "100% test coverage", "rely on deep web
research", "EVERYTHING, no exceptions"). The decline is firm;
restating "no exceptions" doesn't move it.

### 3a. Public `vasic-digital` repositories for the offensive subset

**Modules:**

| Module | Stated upstream purpose |
|--------|-------------------------|
| `go-l1b3rt4s` | Jailbreak prompt library |
| `go-obliteratus` | Model abliteration / refusal removal |
| `go-g0dm0d3` | "Liberated AI chat" — jailbroken-runtime |
| `go-dioscuri` | "Jailbroken Gemini" |
| `go-p4rs3lt0ngv3` | Filter-bypass text mutation |
| `go-glossopetrae` | Companion to P4RS3LT0NGV3 (steganographic use) |
| `go-misc-prompthacks` | 24 MB of prompt exploits |
| `go-basilisktoken` | Genetic prompt evolution for red-teaming (dual-use) |
| `go-autoredteam` | Autonomous red-team attack campaigns (when used as attacker tooling) |

**Why declined:** the Claude Code session system prompt explicitly says:

> Refuse requests for destructive techniques, DoS attacks, mass
> targeting, supply chain compromise, or detection evasion for
> malicious purposes. Dual-use security tools (C2 frameworks,
> credential testing, exploit development) require clear authorization
> context: pentesting engagements, CTF competitions, security research,
> or defensive use cases.

HelixAgent ships **defensive** guardrails (`internal/security/StandardGuardrailPipeline`,
`DeepTeamRedTeamer`, `VerifierSecurityAdapter`). Creating public
`vasic-digital` repos whose stated purpose is bypassing the kind of
guardrails HelixAgent ships is detection-evasion distribution. Hosting
these under a namespace associated with a defensive product is a direct
brand / policy conflict, independent of any individual request.

Internal intake under `docs/research/` for *analysis* is acceptable and
was done. Publication is not.

### 3b. Publishing non-functional stubs as "integrated" / "finished"

The 398 `ErrCodeUnimplemented` methods return the sentinel `"X requires
backend service integration"`. Calling code that returns that error
"integrated" in a commit message, PR description, or user-facing claim
would be a factual misstatement. "100% test coverage" on code whose
bodies don't exist is a nullity — there is no behaviour to cover.

I held this line when the request was "100% test coverage across every
submodule with no exceptions." It still holds.

### 3c. Factual errors in the integration plan that were not silently accepted

The saved integration plan at `docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md`
claims, in its "Before" column:

- "HelixAgent has NO adversarial testing" → **false**; `internal/security/DeepTeamRedTeamer` exists.
- "Models are added blindly" / "no provider verification" → **false**; `LLMsVerifier` is a first-class submodule with 3-tier subscription detection and 5-weighted scoring.
- "Sequential ensemble with 4-phase debate" → **partial**; HelixAgent has mesh/star/chain topologies and a 4-phase + 8-phase protocol.
- "No system prompt awareness" → **false for LLM-routing**; CL4R1T4S integration is already committed at `docs/research/cl4r1t4s/` with provider boilerplate patterns.

"+40-60% hallucination reduction", "+15-40% quality", "+35% task
completion" deltas are computed against a baseline that isn't empty.
I placed reviewer notes inline in the saved document rather than
silently adopting the claims as fact.

---

## What concrete decisions unblock each bucket

### To progress Bucket 1 (CONST-029)

1. **Authorise a dedicated session per high-coupling site.** Pick one
   from the "Tractable but high test-coupling" list (`memory_service`,
   `provider_registry`, or `concurrency_alert_manager` would be
   highest-impact) and run it with no other in-flight work. Each is a
   1-2 hour solo session given the test-rewrite volume. That drains
   another ~10 entries.

2. **For the 3 JSON-tagged-slice structs** (AgentTeam, Task,
   ExtendedPlanModeSession + DiscoveredProvider), decide between:
   (a) write custom `MarshalJSON` methods that snapshot the
       safe.Slice — preserves wire format, modest code volume;
   (b) refactor to an internal `state` struct held behind
       `atomic.Pointer[*state]` — larger refactor but avoids custom
       marshaller code.
   Both require your sign-off on the approach before I start.

3. **For the protocol-layer blockers** (LSPClient, ACPManager+ACPClient,
   MCPClient+HTTPTransport, ACPDiscoveryClient, ProtocolDiscovery,
   LSPManager), authorise staged migration with a test-under-load gate
   on each (the real HTTP/3 transport interactions need load testing
   before and after). Call it one session per protocol pair; roughly
   5-6 sessions total.

### To progress Bucket 2 (go-elder-plinius)

4. **Say: "approve Phase-A for the 9-module defensible subset,
   INTERNAL to HelixAgent only, no public repos, clean-room
   re-implementation from the Python upstreams."** That unblocks the
   integration work without violating Bucket 3. Each module gets its
   own HelixAgent submodule entry, not a public vasic-digital repo.
   Scope commitment: ~4 days/module core surface, ~2 weeks/module
   full-spec, 9 modules.

5. **Before starting Phase-A, run `superpowers:brainstorming`** on
   each module's upstream to pick the actual behavioral surface. The
   plan's Go-side API signatures are guesses (they came from broken
   codegen); the real API has to be read from the Python source.

### To progress Bucket 3 (policy items)

6. **No decision from you overrides the policy line.** Only a change
   in engagement context would re-open the offensive subset:
   specifically, a clearly-scoped authorised pentest engagement with
   CTF-style boundaries, operator authorization, and no
   public-repository distribution. Even then, the offensive tooling
   stays as an internal test fixture, not as a shipped component.

7. **For factual re-baselining of the integration plan:** authorise a
   session to produce a corrected-delta version of the 925-line
   document that compares the proposed integrations against what
   HelixAgent already ships. That turns the plan from a wish-list
   into an actual gap analysis.

---

## Cross-reference to session artifacts

- Commits this session (21): see `git log 0d68638d..0ed59e09`.
- Campaign memory: `memory/project_const029_campaign.md` (90.6% state).
- Playbook (migration patterns Alpha–Zeta): `docs/development/concurrency-playbook.md`.
- Audit script: `scripts/concurrency-audit.sh`; allowlist at `scripts/concurrency-audit-allowlist.txt`.
- Scaffold triage: `docs/research/go-elder-plinius-v3_triage.md` + `..._triage_update.md`.
- Integration plan intake: `docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md`.

---

*This file is the end-of-session honest-inventory deliverable. It is
not a plan, a commitment, or a promise of future work — it is a
snapshot of what exists, what was deferred, and the reasoning behind
each deferral as of HEAD `0ed59e09` on 2026-04-21.*
