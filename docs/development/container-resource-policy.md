# Container Resource Policy

Authoritative tier table and conventions for every service in
`docker-compose.yml`. The conventions in this document are enforced by:

- `challenges/scripts/compose_resource_limits_challenge.sh` (Challenge,
  CONST-032 reproduction guard for Issue #50)
- `internal/adapters/containers/compose_resources_test.go`
  (`TestComposeResourceInvariants`, runs under `go test ./...`)
- `scripts/normalize-compose-resources.py` (idempotent rewriter — run it
  whenever you add or rename a service)

## TL;DR

1. **One vocabulary**: every service uses `deploy.resources.limits` +
   `deploy.resources.reservations`. The legacy top-level keys
   (`mem_limit`, `memswap_limit`, `pids_limit`) are forbidden — Docker
   Compose v2 rejects the project when both forms collide. See
   BUGFIXES.md Issue #50.
2. **Env-var-driven**: every value is `${SERVICE_FIELD:-default}` so
   resources scale across dev / staging / production without YAML edits.
3. **Reservations always present**: scheduler placement hints matter
   under remote distribution — without them the scheduler has no signal
   on which host has the headroom.
4. **CPUs are quoted strings**: `cpus: "2.00"`, not bare `cpus: 2.00`.
   Compose accepts both, but quoted-string is stable across YAML
   round-trips and makes env-var interpolation work without surprises.

## Canonical service block

```yaml
services:
  postgres:
    image: docker.io/postgres:15-alpine
    container_name: helixagent-postgres
    oom_score_adj: 500            # kernel-priority hint, NOT a resource limit
    # ... env, volumes, healthcheck ...
    deploy:
      resources:
        limits:
          memory: ${POSTGRES_MEM_LIMIT:-4G}
          cpus: "${POSTGRES_CPU_LIMIT:-2.00}"
          pids: 1024
        reservations:
          memory: ${POSTGRES_MEM_RESERVE:-1G}
          cpus: "${POSTGRES_CPU_RESERVE:-0.50}"
    restart: unless-stopped
```

**Forbidden**: `mem_limit`, `memswap_limit`, `pids_limit` at top level
of any service.

## Tier table

The 16 orchestrated services are grouped into four tiers. Dev defaults
are baked into `docker-compose.yml`. Production defaults live in
`docker-compose.scale.yml` (overlay) and `.env.scale.example` (env
reference). Operators bump individual services by setting
`<SERVICE_UPPER>_MEM_LIMIT` etc. ahead of overlay merging.

| Tier   | Services                                                                 | Dev mem | Dev cpu | Prod mem | Prod cpu | pids |
|--------|--------------------------------------------------------------------------|---------|---------|----------|----------|------|
| Tiny   | redis, grafana, mock-llm                                                 | 1G/256M | 0.50/0.10 | 2G/512M | 1.00/0.25 | 1024 |
| Small  | prometheus, langchain-server, llamaindex-server, guidance-server, lmql-server | 2G/512M | 1.00/0.25 | 4G/1G   | 2.00/0.50 | 1024 |
| Medium | postgres, chromadb, memgraph, neo4j, cognee, helixagent, sglang          | 4G/1G   | 2.00/0.50 | 8G/2G   | 4.00/1.00 | 1024–2048 |
| XL     | ollama                                                                   | 12G/4G  | 4.00/1.00 | 24G/8G  | 8.00/2.00 | 2048 |

(`mem` rows show `limit / reservation`. Reservations are scheduler
hints, not enforced minimums.)

### Tier rationale

**Tiny — caches, sidecars, dashboards.** Steady-state memory is
small; pinning at 1–2 G prevents unbounded growth from misbehaving
tenants.

- *redis* — appendonly enabled, but data set is HelixAgent metadata
  (provider state, rate-limit windows). 256–512 MB is plenty.
- *grafana* — render-only, dashboards are stateless on disk.
- *mock-llm* — test double, no model weights, just JSON.

**Small — Python framework servers.** Loading langchain / llamaindex /
guidance / lmql pulls in ~500 MB of Python deps + transformer tokenizers
without weights. 2 G dev / 4 G prod accommodates two concurrent
inference threads. Prometheus is grouped here because its ingest path is
RAM-bound on label cardinality.

**Medium — databases + main gateway + LLM-heavy services.** Working
sets and indexes for Postgres / ChromaDB / Memgraph / Neo4j benefit
from headroom; cognee orchestrates LLM calls plus a knowledge graph,
which in turn pulls in embeddings; the helixagent gateway holds the
service registry, route table, and per-provider state in memory.
sglang serves LLM traffic and benefits from the same budget. neo4j and
helixagent use 2048 pids (more spawned worker threads than peers).

**XL — local LLM serving.** Ollama loads the active model weights
into RAM. 12 G dev fits a quantized 7B model with KV cache; 24 G prod
fits a 13B or two 7Bs in parallel. Bump the env var if you serve 70B.

## Production overlay

```bash
# Layered compose: base (dev defaults) + scale overlay (prod defaults)
docker compose -f docker-compose.yml -f docker-compose.scale.yml up -d
```

`docker-compose.scale.yml` is *only* `deploy.resources` overrides — no
new services, no env changes. Layering is additive; conflicts on
non-resource keys would silently merge per Compose merge rules, which
is why the overlay is restricted to resource fields.

To override a single service for a one-off run without writing a new
overlay:

```bash
POSTGRES_MEM_LIMIT=16G POSTGRES_CPU_LIMIT=8.00 \
  docker compose -f docker-compose.yml -f docker-compose.scale.yml up -d
```

## When to update this policy

1. **Adding a service** — add it to `TIERS` in
   `scripts/normalize-compose-resources.py`, run the script, update the
   table above, add the service name to `required` in
   `compose_resource_limits_challenge.sh` and the test, AND to the prod
   overlay if it's required for distributed runs.
2. **Resizing a tier** — update the dev defaults in the script, run it,
   update the prod overlay, document the rationale in BUGFIXES.md (or a
   tuning note if it's not a fix).
3. **New constraint type** (e.g. block-IO limits) — add it to the Tier
   struct and the canonical block here.

## Why we don't use top-level `mem_limit` anymore

Docker Compose v2 strict mode rejects projects where the same resource
is specified twice with different values. Even when values match, the
dual form is an aliasing trap waiting to happen. Compose v3 deprecates
the top-level keys in favor of `deploy.resources` exclusively. Podman
tolerates the old form but treats it as a synonym (legacy wins),
masking the conflict. We pick the strictest interpretation — Compose
v2 — as the floor, so the same compose file boots on every runtime in
the orchestrator's distribution set.

## Why `oom_score_adj` is preserved

It's a kernel hint (priority for the OOM killer), not a resource
constraint. Compose v2 accepts it alongside `deploy:`. Setting it to
`500` (default+max=1000) makes our containers preferred OOM victims
when the host is under memory pressure — better that postgres restarts
than the host gets reaped.

## Why `pids_limit` was a separate landmine

The fix initially missed it because Compose's "distinct values"
rejection on `pids_limit` only fires when a `deploy:` block is present
on the same service. Pre-fix, services with no `deploy:` had no
conflict; the fix adds `deploy:` to every service, which surfaces the
`pids_limit` conflict on every one of them. Both legacy keys
(`mem_limit`, `pids_limit`) are stripped by the normalizer in a single
pass.
