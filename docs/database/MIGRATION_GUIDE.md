# Database Migration Guide

Generated 2026-04-11 (Phase 7). Covers all SQL schema files under
`sql/schema/` and the order in which they should be applied to a fresh
PostgreSQL instance.

## Target engine

- **PostgreSQL 15+** (primary, via `pgx/v5`)
- **ClickHouse 23+** (for `clickhouse_analytics.sql` only — separate instance)

Primary Postgres runs on port `5432` in production and `15432` in the test
infrastructure (`docker-compose.infra.yml` / boot-manager defaults).

## Application order

All files are idempotent (`CREATE ... IF NOT EXISTS` / `CREATE TABLE ... IF
NOT EXISTS` where possible) but must be applied in this dependency order for
a clean first install.

### Tier 1 — Core identity & sessions

| # | File | Purpose |
|---|---|---|
| 1 | `users_sessions.sql` | Users, sessions, API keys — foundation for all foreign keys |
| 2 | `llm_providers.sql` | Provider registry, model metadata, capabilities |
| 3 | `requests_responses.sql` | LLM requests/responses ledger |

### Tier 2 — Feature subsystems

| # | File | Purpose |
|---|---|---|
| 4 | `background_tasks.sql` | Background task queue (used by `internal/background`) |
| 5 | `protocol_support.sql` | MCP, LSP, ACP, Embeddings — protocol state |
| 6 | `cognee_memories.sql` | Cognee memory layer (optional, behind `COGNEE_ENABLED`) |
| 7 | `conversation_context.sql` | Infinite-context engine + Kafka-backed replay |
| 8 | `code_versions.sql` | Git-worktree code version snapshots for debate |

### Tier 3 — AI debate system

| # | File | Purpose |
|---|---|---|
| 9 | `debate_system.sql` | Root tables for the AI debate subsystem |
| 10 | `debate_sessions.sql` | Debate session records |
| 11 | `debate_turns.sql` | Individual debate turns / messages |
| 12 | `cross_session_learning.sql` | Learned patterns + accumulated wisdom |

### Tier 4 — Higher-level features

| # | File | Purpose |
|---|---|---|
| 13 | `agentic_workflows.sql` | Graph-based workflow orchestration state |
| 14 | `planning_sessions.sql` | HiPlan / MCTS / Tree of Thoughts results |
| 15 | `llmops_experiments.sql` | A/B experiments, eval runs, prompt versions |
| 16 | `distributed_memory.sql` | Event sourcing + CRDT multi-node memory |
| 17 | `streaming_analytics.sql` | Kafka Streams real-time conversation analytics |

### Tier 5 — Indexes, views, constraints

| # | File | Purpose |
|---|---|---|
| 18 | `indexes_views.sql` | Performance indexes + materialized views |
| 19 | `relationships.sql` | Cross-table foreign keys + ER documentation |

### Reference bundle

| File | Purpose |
|---|---|
| `complete_schema.sql` | Combined reference dump of the whole schema — apply this for a single-shot install |

### Separate engine

| File | Target | Purpose |
|---|---|---|
| `clickhouse_analytics.sql` | **ClickHouse** | Time-series analytics for debates, conversations, providers |

## Applying the schema

All application happens via the HelixAgent binary during boot — **do not
run `psql` manually unless you are recovering from a corrupted database**.
On boot, `internal/services/boot_manager.go` applies schemas in the Tier
order above. If you must apply manually for an emergency recovery:

```bash
# 1. Verify the database is up (it should have been booted by the binary)
pg_isready -h localhost -p 5432 -U helixagent

# 2. Apply in tier order
for f in \
  sql/schema/users_sessions.sql \
  sql/schema/llm_providers.sql \
  sql/schema/requests_responses.sql \
  sql/schema/background_tasks.sql \
  sql/schema/protocol_support.sql \
  sql/schema/cognee_memories.sql \
  sql/schema/conversation_context.sql \
  sql/schema/code_versions.sql \
  sql/schema/debate_system.sql \
  sql/schema/debate_sessions.sql \
  sql/schema/debate_turns.sql \
  sql/schema/cross_session_learning.sql \
  sql/schema/agentic_workflows.sql \
  sql/schema/planning_sessions.sql \
  sql/schema/llmops_experiments.sql \
  sql/schema/distributed_memory.sql \
  sql/schema/streaming_analytics.sql \
  sql/schema/indexes_views.sql \
  sql/schema/relationships.sql \
; do
  echo "Applying $f"
  PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 -f "$f" || exit 1
done
```

**Do not** run this as part of any CI pipeline or git hook — the project
Constitution forbids both. Manual, human-driven only.

## Rollback policy

The schemas are intentionally additive. There is **no downgrade path** — a
rollback means restoring from a database backup or dropping and re-
initialising the cluster. This is consistent with the note in
`LLMsVerifier/llm-verifier/database/migrations.go:415`
("rolling back this migration is not supported").

## Adding a new schema file

1. Decide which tier it belongs to based on foreign-key dependencies.
2. Insert it between the appropriate existing files in `boot_manager.go`'s
   schema application list.
3. Update this document's Tier tables.
4. Regenerate `complete_schema.sql` if you use it for single-shot installs.
5. Add a migration test under `tests/integration/` that applies the new
   schema against a fresh test-infrastructure container and asserts the
   expected tables exist.
