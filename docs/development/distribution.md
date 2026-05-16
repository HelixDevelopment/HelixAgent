# Remote Distribution — Mode B (label-based sharding)

How HelixAgent deploys its containerised stack across multiple
remote hosts registered via `containers/.env`.

## Two modes, one mechanism

The adapter (`internal/adapters/containers/adapter.go:RemoteComposeUp`)
iterates every registered remote host and deploys the compose file
to each in turn. Per-host behavior is decided by **one label**:
`deploy_profile`.

| Host has `deploy_profile` label? | Effective compose profile on that host |
|---|---|
| No (unset) | Caller-supplied profile (typically `default`) |
| Yes, e.g. `deploy_profile=storage` | `storage` — overrides caller |

Combined with compose-file-level service tagging, this gives you:

- **Mode A — full-stack replication** (default, no config required).
  No hosts carry a `deploy_profile` label, the compose file has no
  `profiles:` on services, so every host runs every service. Useful
  for HA of stateless services; **not appropriate for singleton
  state** like primary Postgres or Redis.

- **Mode B — label-based sharding**. Each host carries a
  `deploy_profile` label; the compose file tags every service with
  one or more `profiles:`; only matching services land on each host.
  This is the intended steady-state for multi-host deployments.

## Activating Mode B

Mode B requires **both** changes — setting labels alone does
nothing if the compose file doesn't tag services.

### Step 1 — label hosts in `containers/.env`

Extend the `_LABELS` entry for each host with a `deploy_profile` tag
and add your service-group labels:

```ini
CONTAINERS_REMOTE_HOST_1_NAME=thinker
CONTAINERS_REMOTE_HOST_1_LABELS=storage=fast,memory=high,deploy_profile=storage

CONTAINERS_REMOTE_HOST_2_NAME=amber
CONTAINERS_REMOTE_HOST_2_LABELS=memory=high,deploy_profile=compute
```

### Step 2 — tag services in `docker-compose*.yml`

Every service must advertise one or more profiles. Services without
any profile key deploy on *every* invocation regardless of which
`--profile` flag is passed — so tag them explicitly even if the
intent is "runs everywhere".

```yaml
services:
  helixagent-postgres:
    image: postgres:15-alpine
    profiles: [storage]      # only on hosts with deploy_profile=storage
    ports:
      - "${HELIXAGENT_PORT_POSTGRES:-8101}:5432"

  helixagent-redis:
    image: redis:7-alpine
    profiles: [storage]
    # ...

  helixllm:
    image: helixagent_helixllm:latest
    profiles: [compute]      # only on hosts with deploy_profile=compute
    # ...

  mcp-fetch:
    image: mcp_mcp-fetch:latest
    profiles: [compute]
    # ...

  cognee:
    image: helixagent-cognee:latest
    profiles: [storage, compute]  # deploys on both
    # ...
```

### Step 3 — rebuild and relaunch

No code change needed after the first wiring; the adapter picks up
`deploy_profile` from the existing labels map at runtime.

```bash
make build
./bin/helixagent
```

The boot log will show the per-host profile override:

```
host thinker has deploy_profile=storage label; overriding deploy
profile for this host (caller asked "default")
host amber has deploy_profile=compute label; overriding deploy
profile for this host (caller asked "default")
```

## Strict-remote mode (CONST-031)

When `CONTAINERS_REMOTE_ENABLED=true`, `boot_manager.go` skips the
local compose path entirely. Any endpoint whose config forgot to
set `Remote=true` is logged with status `skipped_strict_remote` and
**not** started locally. This prevents the legacy hybrid deployment
(some services on remote hosts, others quietly running locally)
that used to mask remote-deploy failures.

If you actually need a specific service running locally while the
rest is remote, either:

1. Mark the endpoint `Remote=true` and add a compose entry for it
   on one of the remote hosts, or
2. Turn off remote distribution (`CONTAINERS_REMOTE_ENABLED=false`),
   in which case local compose is the whole picture.

## Resilience

- **Continue-on-error**: If one host fails, deployment to the other
  hosts continues. If *any* host succeeded, `RemoteComposeUp`
  returns nil (partial-success warning logged). If *every* host
  failed, it returns an aggregate error.
- **SSH keep-alive**: 30s ping every 10 missed → 5-minute network
  silence tolerance. Added in the Containers submodule — prevents
  long compose builds from dying on routine micro-stalls.
- **Idempotent deploys**: `compose up -d` is idempotent — a second
  run just reconciles state. Re-running after a partial failure is
  safe.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| "remote compose up on X: exit -1" | SSH session dropped during build | Confirm `KeepAlive` option is >0 in `remote.Options`; verify `ServerAliveInterval` on ssh command line via `--debug` |
| One host deploys, other is silent | Using pre-1fe0c83 Containers adapter (only picked `hosts[0]`) | Bump submodule pointer |
| "CONTAINERS_REMOTE_ENABLED=true but these services are not marked ep.Remote" warning | Endpoint config forgot `Remote=true` | Update endpoint or disable remote |
| Mode B labels set but all services run on all hosts | Compose services lack `profiles:` tags | Add `profiles:` to every service |
| Every service deploys to no host | Caller asked for `--profile X` but no services tagged with X | Check profile-tag alignment |
