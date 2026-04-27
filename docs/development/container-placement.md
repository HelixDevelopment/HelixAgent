# Container Placement & Partitioned Distribution

Authoritative description of how HelixAgent distributes containers
across remote hosts WITHOUT duplication. Pairs with
`docs/development/container-resource-policy.md` (per-service resource
budgets).

## TL;DR

- The orchestrator (`bin/helixagent` running on the local host) is the
  **single source of truth** for placement. No external script touches
  container state per CONST-031.
- Each containerized service runs on **EXACTLY ONE** host across the
  CONTAINERS_REMOTE_HOST_N_* set. No replication. Duplication =
  divergent writes = data corruption.
- Services with `depends_on` form a **co-location group** and are
  always placed on the same host. (Docker Compose `depends_on` is
  intra-host; cross-host `depends_on` does not work.)
- Placement is computed at boot via the existing scheduler in
  `Containers/pkg/scheduler` (default strategy:
  `StrategyResourceAware`), using live host probes from
  `Containers/pkg/remote.Prober`.
- The plan for each compose file is persisted to
  `<project>/.placement-plan-<compose>.json` for operator audit and
  for the verification Challenge.

## Architecture

```
+------------------+
|  bin/helixagent  |  ── boot ──┐
| (orchestrator)   |             │
+------------------+             ▼
       │             internal/placement
       │                 ParseCompose      → ContainerRequirements
       │                 PlanCompose       → PlacementPlan
       │                 EmitPerHostCompose → per-host compose subset
       │
       │             Containers/pkg/scheduler  (existing, unchanged)
       │             Containers/pkg/remote     (existing, unchanged)
       │             Containers/pkg/serviceregistry (existing, unchanged)
       ▼
+----------+   +----------+   +----------+
| host A   |   | host B   |   | host C   |
| podman   |   | docker   |   | …        |
| services |   | services |   | services |
+----------+   +----------+   +----------+
       ▲             ▲              ▲
       └─────────────┴──────────────┘
            SVC_<X>_HOST env vars
            tell internal/config
            which host owns each
            service at runtime
```

## Files in this fix

| File | Purpose |
|------|---------|
| `internal/placement/parser.go` | Parse a docker-compose file → `[]scheduler.ContainerRequirements`. Reads `deploy.resources.{limits,reservations}` (with `${VAR:-default}` env-var unwrap), `depends_on` (list + map forms), `profiles`, GPU device reservations. Computes co-location groups via union-find over the `depends_on` graph. |
| `internal/placement/emitter.go` | Given a source compose + a list of services, write a filtered compose containing ONLY those services (preserving networks, volumes, top-level keys). |
| `internal/placement/planner.go` | High-level `PlanCompose(ctx, file, profile, hostManager, opts...) → *Plan`. Groups requirements by co-location label, schedules each group as one atomic unit, returns `[]HostAssignment`. |
| `internal/placement/persist.go` | `WritePlanJSON` / `ReadPlanJSON` for the per-compose plan record. |
| `cmd/helixagent/placement_deploy.go` | `deployComposePartitioned` — used by `cmd/helixagent/main.go` instead of `RemoteComposeUp` for partitioned distribution. Stages per-host filtered composes inside the project tree so build contexts (`context: ../..`) resolve correctly. Sets `SVC_<SERVICE>_HOST` env vars after each successful host deploy. |
| `internal/adapters/containers/adapter.go` | Adds `HostManager()` getter and `DeployComposeToHost(ctx, hostName, file, profile)` wrapper around the existing private `deployComposeToHost`. |
| `challenges/scripts/partitioned_distribution_challenge.sh` | Reproduction guard (CONST-032). Asserts every container appears on exactly one host. |
| `internal/placement/*_test.go` | Unit invariants: co-location preservation, no duplicates, every service placed. Uses a fake executor so the scheduler runs without SSH. |

## Co-location groups

Services that depend on each other must be co-located. Examples in the
current codebase:

- **`cognee` group** (in `docker-compose.yml`):
  `cognee` → depends on `postgres`, `redis`, `chromadb`. The four
  services always land on the same host.
- **MCP backend pairs** (in `docker/mcp/docker-compose.mcp-servers.yml`):
  `mcp-redis` + `mcp-redis-backend`, `mcp-mongodb` + `mcp-mongodb-backend`,
  `mcp-qdrant` + `mcp-qdrant-backend` — each pair is one group.

Co-location is automatically derived from `depends_on`. No manual
grouping config.

## Adding a new service

Same as before:

1. Add the service block to the appropriate compose file.
2. Set `deploy.resources.limits` + `reservations` per the tier table
   in `container-resource-policy.md`.
3. If it depends on other services, declare `depends_on:` — placement
   will follow.
4. If it requires a GPU, declare it via:
   ```yaml
   deploy:
     resources:
       reservations:
         devices:
           - driver: nvidia
             count: 1
             capabilities: [gpu]
   ```
   The parser turns this into a `scheduler.GPURequirement` and the
   scheduler's `StrategyGPUAffinity` will steer placement.

## Adding a new host

Append a `CONTAINERS_REMOTE_HOST_N_*` block to `Containers/.env`. The
loader stops at the first absent `_NAME`, so N can scale freely (1..100
per CONST-031). The scheduler picks up the new host on the next boot.

## Strategy selection

Default is `scheduler.StrategyResourceAware` — picks the host with the
most available memory + CPU after subtracting current placements. Other
strategies in `Containers/pkg/scheduler` are also available
(`StrategyRoundRobin`, `StrategySpread`, `StrategyBinPack`,
`StrategyAffinity`, `StrategyGPUAffinity`). Switch via the `opts`
parameter in `PlanCompose`.

## Why we partition rather than replicate

Replicated deployment (the previous behaviour) put a copy of postgres,
redis, chromadb, and cognee on every remote host. Each cognee instance
wrote to its local postgres, so the four hosts had four independent
postgres databases that diverged. Reads through the gateway returned
data from whichever host responded first. This is **data corruption
under any meaningful workload** and was the trigger for partitioning
(BUGFIXES.md Issue #52).

## Verification

```bash
# After a fresh boot:
challenges/scripts/partitioned_distribution_challenge.sh
# Asserts: every helixagent-* container exists on EXACTLY ONE host.

# Unit tests (no docker / SSH required):
go test -count=1 ./internal/placement/...
# Asserts: co-location, no duplicates, every service placed.

# Inspect the plan persisted by the orchestrator:
cat .placement-plan-docker-compose.yml.json     | jq
cat .placement-plan-docker-compose.mcp-servers.yml.json | jq
```

## Future work (out of scope for Issue #52)

- **Volume migration when a service moves hosts.** Today, moving postgres
  from host A → host B = data loss. Add an opt-in volume rsync pass so a
  rebalance preserves state.
- **Live rebalancing.** `Containers/pkg/distribution.Distributor.Rebalance`
  exists but isn't wired in; could trigger on host degraded events.
- **Operator-overridable per-service pinning** via env var
  `HELIXAGENT_PLACE_<SERVICE>=<host>` for emergencies. Today the
  scheduler's choice is final.
