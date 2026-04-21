# CONST-029 Protocol-Layer Per-Site Plan (2026-04-21)

**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-3
**Source inventory:** `docs/development/REMAINING_WORK_2026-04-21.md` §Bucket-1b
**Status:** PLAN-ONLY. No code changes in this document's commit.

## Why these are different from Bucket-1a/1c

Each site owns transport or protocol state that interacts with live
HTTP/3/QUIC sessions. Breaking request ordering or session affinity
during migration would manifest as protocol-level bugs under load
that unit-level race tests don't catch.

Although none of the eight target files import `quic-go` directly
(grep confirms zero hits under `internal/services/`), their state is
consumed by handlers served through the router that IS wrapped by the
QUIC listener (`internal/router/router.go`). The protocol sessions
themselves — WebSocket, stdio JSON-RPC, LSP framing — terminate inside
these structs. A partial migration that leaves a hybrid
`sync.RWMutex + safe.Store` pattern can reorder reads across session
boundaries; this is the exact failure mode that load tests are built
to catch.

## Common gate: test-under-load

Every protocol-layer migration MUST:
1. Run `./tests/load/` suite BEFORE the migration, capture baseline.
2. Perform the migration.
3. Run the same suite AFTER. Pass criteria: no p99 regression >10%;
   no new errors; no session-drop rate increase.
4. If #3 fails, revert; schedule a separate debug session.

The load suite exposes four tests in a single file
(`tests/load/load_test.go`, 324 lines, inspected 2026-04-21):

| Test | Line | Purpose |
|------|------|---------|
| `TestLoad_SustainedConstantRate` | 95 | Baseline p99 under constant RPS |
| `TestLoad_SpikeTraffic` | 152 | Abrupt 10× traffic spike |
| `TestLoad_SoakTest` | 226 | Long-duration leak detection |
| `TestLoad_GoroutineLeakDetection` | 282 | Count deltas around requests |

The suite is endpoint-agnostic (the 324-line file contains no literal
`/v1/lsp`, `/v1/acp`, or `/v1/mcp` string). Per-site gates therefore
add a targeted stress file in addition to the generic suite; these
complementary files live under `tests/stress/` and `tests/integration/`
and are cited per-site below.

Loads to run per site are noted below.

## Site 1: LSPClient (internal/services/acp_client.go:20)

### Current shape

```go
// LSPClient implements a real Language Server Protocol client
type LSPClient struct {
    servers      map[string]*LSPServerConnection
    capabilities map[string]*LSPCapabilities
    diagnostics  map[string][]*ACPDiagnostic // URI -> diagnostics
    messageID    atomic.Int64
    mu           sync.RWMutex
    logger       *logrus.Logger
}
```

### Protocol state inventory

- `servers`: serverID → live `LSPServerConnection` (owns transport,
  capabilities, per-URI file info). Touched on every request/response.
- `capabilities`: serverID → `LSPCapabilities` (JSON-decoded once per
  session, then read-mostly).
- `diagnostics`: URI → slice of `ACPDiagnostic` (server-pushed via
  `textDocument/publishDiagnostics`; grows unbounded across a session).
- `messageID`: already `atomic.Int64` — no migration needed.

### Test-coupling census

- Test file: `internal/services/acp_client_test.go`
- Direct field accesses (pattern
  `\.servers\[|\.capabilities\[|\.diagnostics\[|\.mu\.|\.messageID`):
  **119** matches. Highest in the Bucket.
- `LSPClient{…}` struct literals across `internal/`: 4 files
  (`internal/services/integration_orchestrator_test.go`,
  `internal/clis/continueagent/lsp_client.go` — NB: different package,
  same type name collision,
  `internal/testing/lsp/functional_test.go`,
  `internal/services/acp_client.go` itself).

### Migration staging

1. Inventory direct field accesses in tests, plan rewrite. 119 hits is
   high — convert tests to use getter methods first, in its own commit.
2. Migrate scalar counters → atomics FIRST. Already done
   (`messageID atomic.Int64`); no work here.
3. Migrate pure-state maps (lookup tables) → `safe.Store`. Start with
   `capabilities` — read-mostly after init, safest conversion.
4. Migrate session-routing maps LAST — `servers` and `diagnostics`
   carry transport affinity and server-push ordering respectively.
5. Remove `mu` once all three maps are in `safe.Store`.

### Test-under-load gate

Generic: `tests/load/load_test.go` (all four tests).
Targeted: `tests/integration/protocol_comprehensive_integration_test.go`
exercises LSP traffic end-to-end. No dedicated stress file exists
under `tests/stress/` for LSP (absence noted).

### Session-budget estimate

~2h.

---

## Site 2-3: ACPManager + ACPClient (paired)

### Why paired

Sibling structs in `internal/services/acp_manager.go`:
`ACPManager` embeds a `*ACPClient` (field `client` on line 27). Both
types have their own mutex: `ACPManager.serversMu` and
`ACPClient.wsConnsMu`. A lock ordering between them is implicit
in the call graph; migrating one to `safe.Store` without the other
leaves a half-locked object graph where cross-struct invariants
(e.g. "server present ⇒ ws conn exists") are no longer enforceable
in one atomic region.

### Current shapes

```go
// ACPManager handles ACP (Agent Client Protocol) operations
type ACPManager struct {
    repo       *database.ModelMetadataRepository
    cache      CacheInterface
    log        *logrus.Logger
    config     *config.ACPConfig
    client     *ACPClient
    servers    map[string]*ACPServer
    serversMu  sync.RWMutex
    httpClient *http.Client
}
```

```go
// ACPClient handles HTTP and WebSocket communication with ACP servers
type ACPClient struct {
    httpClient *http.Client
    wsDialer   *websocket.Dialer
    wsConns    map[string]*websocket.Conn
    wsConnsMu  sync.RWMutex
    timeout    time.Duration
    maxRetries int
    log        *logrus.Logger
}
```

### Test-coupling census

- Test file: `internal/services/acp_manager_test.go`
- Direct field accesses (pattern
  `\.servers\[|\.wsConns\[|\.mu\.|\.serversMu\.|\.wsConnsMu\.`):
  **10** matches. Low-coupling; most tests treat these as black boxes.
- `ACPManager{…}` struct literals across `internal/`: 1 file
  (only `internal/services/acp_manager.go` itself — no test
  literal fixtures).
- `ACPClient{…}` struct literals across `internal/`: 3 files
  (`internal/testing/acp/functional_test.go`,
  `internal/services/acp_manager.go`, `internal/testing/acp/README.md`
  — the README is a doc-only reference, not a Go literal).

### Staging (same 5-step template as Site 1)

**Per struct — ACPManager:**
1. Inventory test accesses (10 hits, trivially rewritable).
2. No scalar counters present (no step-2 work).
3. Migrate `servers` → `safe.Store[string, *ACPServer]`.
4. No session-routing distinct from `servers`.
5. Remove `serversMu`.

**Per struct — ACPClient:**
1. Inventory test accesses + 1 struct-literal fixture in
   `internal/testing/acp/functional_test.go`.
2. No new scalar counters.
3. Migrate `wsConns` → `safe.Store[string, *websocket.Conn]`.
   NB: websocket.Conn is not concurrency-safe for concurrent writes
   itself; `safe.Store` only protects the map, not the conn. Document
   this in the migration commit message.
4. No distinct routing maps.
5. Remove `wsConnsMu`.

### Test-under-load gate

Generic: `tests/load/load_test.go`.
Targeted: `tests/integration/protocol_comprehensive_integration_test.go`.
Special check: WebSocket session churn — the spike test must not
produce stale wsConns entries (confirm via goroutine leak detection
test after migration).

### Session-budget estimate

~2h (paired in one session).

---

## Site 4-5: MCPClient + HTTPTransport (paired)

### Why paired

`HTTPTransport` is a struct inside `mcp_client.go` specifically for
MCPClient's HTTP layer (defined line 58 in the same file). Migrating
`MCPClient` without `HTTPTransport` leaves a split-brain lock pattern:
the connection-registry lock lives on `MCPClient.mu`, the
per-transport lock lives on `HTTPTransport.mu`, and a single logical
request crosses both. Converting only one yields an object graph where
the protected fields are half `safe.Store`, half `mutex+map`, and
callers can no longer reason about which primitive they are
interacting with when they hold a connection pointer.

### Current shapes

```go
// MCPClient implements a real MCP (Model Context Protocol) client
type MCPClient struct {
    servers   map[string]*MCPServerConnection
    tools     map[string]*MCPTool
    messageID atomic.Int64
    mu        sync.RWMutex
    logger    *logrus.Logger
}
```

```go
// HTTPTransport implements MCP transport over HTTP
type HTTPTransport struct {
    baseURL      string
    headers      map[string]string
    connected    bool
    mu           sync.Mutex
    client       *http.Client
    responseData []byte
}
```

### HTTP/3 transport surface

Specific concerns about:
- **Inflight request maps**: `MCPClient.servers` holds the session
  registry; each `MCPServerConnection` carries a `Transport` interface
  whose concrete type may be `HTTPTransport` or `StdioTransport`.
- **Session-ID → response-channel routing**: `MCPClient.messageID` is
  the JSON-RPC monotonic counter; response correlation happens inside
  the transport, not here. No routing map migration needed on
  MCPClient; the message→channel affinity is inside
  `MCPServerConnection` (not on the allowlist).
- **Stream-reuse / connection pooling**: `HTTPTransport.client` is the
  `*http.Client`, shared across calls. The `responseData` byte slice
  plus `connected` bool under `mu` form a per-transport critical
  region. Migration replaces the bool with an `atomic.Bool` and the
  slice with a single-slot `safe.Store` or (preferred) a channel of
  `[]byte` to preserve ordering semantics.

### Staging

**Per struct — MCPClient:**
1. Inventory test accesses (0 hits — zero matches for
   `\.servers\[|\.tools\[|\.mu\.|\.messageID` in
   `internal/services/mcp_client_test.go`).
2. `messageID` already `atomic.Int64`; no work.
3. Migrate `servers` → `safe.Store[string, *MCPServerConnection]`.
4. Migrate `tools` → `safe.Store[string, *MCPTool]`.
5. Remove `mu`.

**Per struct — HTTPTransport:**
1. No dedicated test file; struct is exercised indirectly via
   `MCPClient` tests and `internal/testing/mcp/functional_test.go`.
2. Replace `connected bool` with `atomic.Bool`.
3. Replace `responseData []byte` with a channel-based or
   single-slot `safe.Store` pattern (concurrency-playbook §"single
   mutable field" — pick the simpler primitive).
4. No maps to route.
5. Remove `mu`.

### Test-under-load gate

Required generic: `tests/load/load_test.go`.
Required targeted:
- `tests/stress/mcp_adapter_stress_test.go`
- `tests/integration/mcp_comprehensive_integration_test.go`
- `tests/integration/mcp_sse_endpoint_test.go` (SSE + streaming)
Special check: MCP requests have specific timing characteristics
(SSE + streaming) — `mcp_sse_endpoint_test.go` is the critical
load-under-stream case. Any p99 regression or out-of-order SSE frame
fails the gate.

### Session-budget estimate

~3h (paired; transport complexity).

---

## Site 6: ACPDiscoveryClient (internal/services/protocol_discovery.go:19)

### Current shape

```go
// ACPDiscoveryClient implements a real Agent Client Protocol client for discovery
type ACPDiscoveryClient struct {
    agents    map[string]*ACPAgentConnection
    messageID atomic.Int64
    mu        sync.RWMutex
    logger    *logrus.Logger
}
```

### Test-coupling census

- Test file: `internal/services/protocol_discovery_test.go`
- Direct field accesses (pattern
  `\.agents\[|\.mu\.|\.messageID`): **42** matches.
- `ACPDiscoveryClient{…}` struct literals across `internal/`: 1 file
  (`internal/services/protocol_discovery.go` — no test-side
  literal fixtures).

### Migration staging

1. Inventory 42 test accesses; convert to getter methods where the
   test is asserting map contents, in a preparatory commit.
2. `messageID` already `atomic.Int64`.
3. Migrate `agents` → `safe.Store[string, *ACPAgentConnection]`.
4. No session-routing maps beyond `agents`.
5. Remove `mu`.

### Test-under-load gate

Generic: `tests/load/load_test.go`.
Targeted:
`tests/integration/protocol_comprehensive_integration_test.go`.

### Session-budget estimate

~2h.

---

## Site 7: ProtocolDiscovery (internal/services/protocol_federation.go:16)

### Current shape

```go
// ProtocolDiscovery provides automatic discovery of protocol servers
type ProtocolDiscovery struct {
    discoveredServers map[string]*DiscoveredServer
    discoveryMethods  []DiscoveryMethod
    mu                sync.RWMutex
    logger            *logrus.Logger
    stopChan          chan struct{}
}
```

### Test-coupling census

- Test file: `internal/services/protocol_federation_test.go`
- Direct field accesses (pattern
  `\.discoveredServers\[|\.discoveryMethods|\.mu\.|\.stopChan`):
  **16** matches.
- `ProtocolDiscovery{…}` struct literals across `internal/`: 2 files
  (`internal/services/protocol_federation.go` and
  `internal/services/protocol_federation_test.go` — the test file
  does use struct-literal fixtures, which constrains the migration to
  keep field names stable until those fixtures are rewritten).

### Migration staging

1. Rewrite the fixture sites first (2 file touches) — this
   preparatory commit is a hard prerequisite.
2. No scalar counters on this struct.
3. Migrate `discoveredServers` → `safe.Store[string, *DiscoveredServer]`.
4. `discoveryMethods []DiscoveryMethod` is append-mostly with rare
   reads. Migrate to `safe.Slice[DiscoveryMethod]` (per playbook).
5. Remove `mu`. `stopChan` does not need changes (channels are already
   concurrency-safe).

### Test-under-load gate

Generic: `tests/load/load_test.go`.
Targeted:
`tests/integration/protocol_comprehensive_integration_test.go`.

### Session-budget estimate

~2h.

---

## Site 8: LSPManager (internal/services/lsp_manager.go:22)

### Current shape

```go
// LSPManager handles LSP (Language Server Protocol) operations
type LSPManager struct {
    repo        *database.ModelMetadataRepository
    cache       CacheInterface
    log         *logrus.Logger
    connections map[string]*LSPConnection
    servers     map[string]*LSPServer
    mu          sync.RWMutex
    messageID   int64
    config      *LSPConfig
}
```

### Connection-pool specifics

- `connections`: connID → `*LSPConnection` (per-request-pipeline state;
  lifetime bounded by request).
- `servers`: serverID → `*LSPServer` (long-lived, one per configured
  language server in `LSPConfig.ServerConfigs`).
- Per-server lifecycle: `NewLSPConnection(server, …)` creates a
  connection bound to a server. Removing a `servers` entry while
  a `connections` entry still references it would produce a dangling
  pointer in the connection's internal `*LSPServer` field. The
  migration must preserve atomic "both maps or neither" semantics
  around server removal — i.e. use a shared `safe.Store` pair
  reconciled at the call-site, not two independent `safe.Store`s
  updated in sequence.
- `messageID int64` is a bare int64 (NOT atomic) — incrementing
  concurrently is a pre-existing race that the migration will fix.

### Migration staging

1. Inventory 9 test accesses in `internal/services/lsp_manager_test.go`.
   Low coupling; rewrite is trivial.
2. Migrate `messageID int64` → `atomic.Int64`. This is itself a bug
   fix (pre-existing race). Commit separately.
3. Migrate `connections` → `safe.Store[string, *LSPConnection]`.
4. Migrate `servers` → `safe.Store[string, *LSPServer]`. Document the
   removal invariant (see Connection-pool specifics).
5. Remove `mu`.

### Test-under-load gate

Generic: `tests/load/load_test.go`.
Targeted:
`tests/integration/protocol_comprehensive_integration_test.go`.

### Session-budget estimate

~2.5h.

---

## Aggregate budget

| Site | Hours |
|------|-------|
| LSPClient | 2 |
| ACPManager+ACPClient (paired) | 2 |
| MCPClient+HTTPTransport (paired) | 3 |
| ACPDiscoveryClient | 2 |
| ProtocolDiscovery | 2 |
| LSPManager | 2.5 |
| **Total** | **13.5** |

## Execution order recommendation

1. **ACPDiscoveryClient** — pure discovery state, lowest transport risk.
2. **ProtocolDiscovery** — similar, establishes patterns.
3. **LSPClient** — standalone client state (high test coupling, but
   tests are already written; just pre-commit rewrite is on the path).
4. **LSPManager** — consumes the migrated LSPClient; also fixes
   pre-existing `messageID` race.
5. **ACPManager+ACPClient** — paired.
6. **MCPClient+HTTPTransport** — paired, highest transport risk,
   last. SSE + streaming is the hardest load-under-stream scenario in
   the codebase.

## Cross-reference

- Parent design spec:
  `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`.
- Concurrency playbook: `docs/development/concurrency-playbook.md`.
- Allowlist: `scripts/concurrency-audit-allowlist.txt` — lines 15, 16,
  17, 21, 22, 23, 25, 26 correspond to the 8 rows in this plan.
- Load suite root: `tests/load/load_test.go` (4 tests, 324 lines).
- Stress suite: `tests/stress/mcp_adapter_stress_test.go` (only
  MCP-specific stress file in the repo).
- Targeted integration coverage:
  `tests/integration/protocol_comprehensive_integration_test.go`,
  `tests/integration/mcp_comprehensive_integration_test.go`,
  `tests/integration/mcp_sse_endpoint_test.go`.
