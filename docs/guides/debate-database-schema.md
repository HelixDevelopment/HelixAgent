# Debate Database Schema

## Overview

The debate system persists its state across three PostgreSQL tables: `debate_sessions`, `debate_turns`, and `code_versions`. Together they provide full session lifecycle tracking, turn-level replay capability, and code evolution history. The schema files live in `sql/schema/`.

## Entity Relationship

```
debate_sessions (1) ---< (N) debate_turns
debate_sessions (1) ---< (N) code_versions
debate_turns    (1) ---< (0..N) code_versions  (optional link)
```

- A debate session has many turns and many code versions.
- Each turn belongs to exactly one session (CASCADE delete).
- Each code version belongs to exactly one session (CASCADE delete) and optionally references the turn that produced it (SET NULL on delete).

## Table: debate_sessions

**File:** `sql/schema/debate_sessions.sql`

Tracks the complete lifecycle of a debate session from initiation to completion.

### Columns

| Column                  | Type                       | Nullable | Default              | Description                                      |
|-------------------------|----------------------------|----------|----------------------|--------------------------------------------------|
| `id`                    | `UUID`                     | NO       | `gen_random_uuid()`  | Primary key                                      |
| `debate_id`             | `VARCHAR(255)`             | NO       |                      | Links to debate_logs for correlation             |
| `topic`                 | `TEXT`                     | NO       |                      | Debate topic or task description                 |
| `status`                | `VARCHAR(50)`              | NO       | `'pending'`          | Session lifecycle state                          |
| `topology_type`         | `VARCHAR(50)`              | YES      |                      | `graph_mesh`, `star`, `chain`, `tree`            |
| `coordination_protocol` | `VARCHAR(50)`              | YES      |                      | `cpde`, `dpde`, `adaptive`                       |
| `config`                | `JSONB`                    | YES      | `'{}'`               | Max rounds, timeout, consensus threshold, gates  |
| `initiated_by`          | `VARCHAR(255)`             | YES      |                      | Requester identifier                             |
| `created_at`            | `TIMESTAMP WITH TIME ZONE` | YES      | `NOW()`              | Session creation time                            |
| `updated_at`            | `TIMESTAMP WITH TIME ZONE` | YES      | `NOW()`              | Last modification (auto-updated by trigger)      |
| `completed_at`          | `TIMESTAMP WITH TIME ZONE` | YES      |                      | When session reached terminal state              |
| `total_rounds`          | `INTEGER`                  | YES      | `0`                  | Number of debate rounds completed                |
| `final_consensus_score` | `DECIMAL(5,4)`             | YES      |                      | Final consensus level (0.0000 - 1.0000)          |
| `outcome`               | `JSONB`                    | YES      | `'{}'`               | Winner, voting method, confidence, summary       |
| `metadata`              | `JSONB`                    | YES      | `'{}'`               | Audit trail, provenance, extra data              |

### Constraints

- **Primary key:** `id`
- **Check constraint (`chk_debate_sessions_status`):** status must be one of `pending`, `running`, `paused`, `completed`, `failed`, `cancelled`

### Status Lifecycle

```
pending -> running -> completed
pending -> running -> paused -> running -> completed
pending -> running -> failed
pending -> cancelled
```

The `paused` status is used when approval gates are enabled and the debate is waiting for human approval.

### Indexes

| Index                                | Columns / Expression              | Type    | Notes                                |
|--------------------------------------|-----------------------------------|---------|--------------------------------------|
| `idx_debate_sessions_debate_id`      | `debate_id`                       | B-tree  | Debate lookup                        |
| `idx_debate_sessions_status`         | `status`                          | B-tree  | Status filtering                     |
| `idx_debate_sessions_created_at`     | `created_at`                      | B-tree  | Time-range queries                   |
| `idx_debate_sessions_topology`       | `topology_type`                   | B-tree  | Topology analytics                   |
| `idx_debate_sessions_active`         | `status` WHERE IN (pending, running, paused) | Partial | Active session queries    |
| `idx_debate_sessions_metadata`       | `metadata`                        | GIN     | JSONB containment queries            |
| `idx_debate_sessions_config`         | `config`                          | GIN     | JSONB containment queries            |
| `idx_debate_sessions_debate_status`  | `(debate_id, status)`             | B-tree  | Composite lookup                     |

### Trigger

An `BEFORE UPDATE` trigger automatically sets `updated_at = NOW()` on every row change.

## Table: debate_turns

**File:** `sql/schema/debate_turns.sql`

Stores every individual agent action within a debate round. This is the granular record needed for full debate replay, provenance tracking, and failure analysis.

### Columns

| Column            | Type                       | Nullable | Default              | Description                                        |
|-------------------|----------------------------|----------|----------------------|----------------------------------------------------|
| `id`              | `UUID`                     | NO       | `gen_random_uuid()`  | Primary key                                        |
| `session_id`      | `UUID`                     | NO       |                      | Foreign key to `debate_sessions.id` (CASCADE)      |
| `round`           | `INTEGER`                  | NO       |                      | Debate round number (1-based)                      |
| `phase`           | `VARCHAR(50)`              | NO       |                      | Protocol phase                                     |
| `agent_id`        | `VARCHAR(255)`             | NO       |                      | Agent that produced this turn                      |
| `agent_role`      | `VARCHAR(100)`             | YES      |                      | Agent role (proposer, critic, red_team, etc.)      |
| `provider`        | `VARCHAR(100)`             | YES      |                      | LLM provider name                                  |
| `model`           | `VARCHAR(255)`             | YES      |                      | Specific model used                                |
| `content`         | `TEXT`                     | YES      |                      | Response content                                   |
| `confidence`      | `DECIMAL(5,4)`             | YES      |                      | Agent confidence (0.0000 - 1.0000)                 |
| `tool_calls`      | `JSONB`                    | YES      | `'[]'`               | Tool invocations and results                       |
| `test_results`    | `JSONB`                    | YES      | `'{}'`               | Test execution outcomes                            |
| `reflections`     | `JSONB`                    | YES      | `'[]'`               | Reflexion episodic memory entries                  |
| `metadata`        | `JSONB`                    | YES      | `'{}'`               | Additional structured data                         |
| `created_at`      | `TIMESTAMP WITH TIME ZONE` | YES      | `NOW()`              | Turn timestamp                                     |
| `response_time_ms`| `INTEGER`                  | YES      |                      | Response latency in milliseconds                   |

### Constraints

- **Primary key:** `id`
- **Foreign key:** `session_id` REFERENCES `debate_sessions(id)` ON DELETE CASCADE
- **Check constraint (`chk_debate_turns_phase`):** phase must be one of `dehallucination`, `self_evolvement`, `proposal`, `critique`, `review`, `optimization`, `adversarial`, `convergence`

### Indexes

| Index                                    | Columns / Expression              | Type    | Notes                              |
|------------------------------------------|-----------------------------------|---------|------------------------------------|
| `idx_debate_turns_session_id`            | `session_id`                      | B-tree  | Session lookup                     |
| `idx_debate_turns_session_round`         | `(session_id, round)`             | B-tree  | Round-level queries                |
| `idx_debate_turns_phase`                 | `phase`                           | B-tree  | Phase filtering                    |
| `idx_debate_turns_agent`                 | `agent_id`                        | B-tree  | Agent tracking                     |
| `idx_debate_turns_session_round_phase`   | `(session_id, round, phase)`      | B-tree  | Most specific lookup               |
| `idx_debate_turns_created_at`            | `created_at`                      | B-tree  | Time-range queries                 |
| `idx_debate_turns_reflections`           | `reflections` WHERE != '[]'       | GIN     | JSONB queries on non-empty reflections |
| `idx_debate_turns_tool_calls`            | `tool_calls` WHERE != '[]'        | GIN     | JSONB queries on non-empty tool calls  |
| `idx_debate_turns_metadata`              | `metadata`                        | GIN     | JSONB containment queries          |

## Table: code_versions

**File:** `sql/schema/code_versions.sql`

Captures code snapshots at key milestones during a debate, enabling solution comparison, rollback, and quality trend analysis.

### Columns

| Column              | Type                       | Nullable | Default              | Description                                     |
|---------------------|----------------------------|----------|----------------------|-------------------------------------------------|
| `id`                | `UUID`                     | NO       | `gen_random_uuid()`  | Primary key                                     |
| `session_id`        | `UUID`                     | NO       |                      | Foreign key to `debate_sessions.id` (CASCADE)   |
| `turn_id`           | `UUID`                     | YES      |                      | Foreign key to `debate_turns.id` (SET NULL)     |
| `language`          | `VARCHAR(50)`              | YES      |                      | Programming language                            |
| `code`              | `TEXT`                     | NO       |                      | Full code snapshot                              |
| `version_number`    | `INTEGER`                  | NO       |                      | Sequential version within session               |
| `quality_score`     | `DECIMAL(5,4)`             | YES      |                      | Overall quality (0.0000 - 1.0000)               |
| `test_pass_rate`    | `DECIMAL(5,4)`             | YES      |                      | Test pass percentage (0.0000 - 1.0000)          |
| `metrics`           | `JSONB`                    | YES      | `'{}'`               | Maintainability, complexity, security scores    |
| `diff_from_previous`| `TEXT`                     | YES      |                      | Unified diff from prior version                 |
| `created_at`        | `TIMESTAMP WITH TIME ZONE` | YES      | `NOW()`              | Version creation time                           |

### Constraints

- **Primary key:** `id`
- **Foreign key:** `session_id` REFERENCES `debate_sessions(id)` ON DELETE CASCADE
- **Foreign key:** `turn_id` REFERENCES `debate_turns(id)` ON DELETE SET NULL
- **Unique constraint (`uq_code_versions_session_version`):** `(session_id, version_number)` -- ensures sequential version numbers within a session

### Indexes

| Index                                  | Columns / Expression              | Type    | Notes                                |
|----------------------------------------|-----------------------------------|---------|--------------------------------------|
| `idx_code_versions_session_id`         | `session_id`                      | B-tree  | Session lookup                       |
| `idx_code_versions_turn_id`            | `turn_id`                         | B-tree  | Turn lookup                          |
| `idx_code_versions_session_version`    | `(session_id, version_number)`    | B-tree  | Ordered version retrieval            |
| `idx_code_versions_language`           | `language`                        | B-tree  | Language analytics                   |
| `idx_code_versions_quality`            | `quality_score` WHERE NOT NULL    | Partial | High-quality version filtering       |
| `idx_code_versions_test_pass_rate`     | `test_pass_rate` WHERE NOT NULL   | Partial | Test pass rate filtering             |
| `idx_code_versions_metrics`            | `metrics`                         | GIN     | JSONB containment queries            |

## Common Query Patterns

### Get all turns for a debate session, ordered by round and phase

```sql
SELECT *
FROM debate_turns
WHERE session_id = $1
ORDER BY round, phase, created_at;
```

### Get the latest code version for a session

```sql
SELECT *
FROM code_versions
WHERE session_id = $1
ORDER BY version_number DESC
LIMIT 1;
```

### Find active debate sessions

```sql
SELECT *
FROM debate_sessions
WHERE status IN ('pending', 'running', 'paused')
ORDER BY created_at DESC;
```

### Get quality trend for a session

```sql
SELECT version_number, quality_score, test_pass_rate
FROM code_versions
WHERE session_id = $1
ORDER BY version_number;
```

### Find all Reflexion episodes for a session

```sql
SELECT id, agent_id, phase, reflections
FROM debate_turns
WHERE session_id = $1
  AND reflections != '[]'::jsonb
ORDER BY round, phase;
```

### Get adversarial round results

```sql
SELECT *
FROM debate_turns
WHERE session_id = $1
  AND phase = 'adversarial'
ORDER BY round;
```

## Repository API

The Go repository layer provides programmatic interfaces over these tables:

### Debate Persistence Repositories (`internal/database/`)

- **`DebateSessionRepository`** -- CRUD operations for `debate_sessions` (Insert, GetByID, GetByDebateID, UpdateStatus, UpdateOutcome, ListByStatus, ListActive, Delete).
- **`DebateTurnRepository`** -- CRUD operations for `debate_turns` (Insert, GetByID, ListBySession, ListBySessionAndRound, ListBySessionAndPhase, ListByAgent, GetReflections, Delete, DeleteBySession).
- **`CodeVersionRepository`** -- CRUD operations for `code_versions` (Insert, GetByID, GetLatestBySession, ListBySession, GetBySessionAndVersion, GetNextVersionNumber, Delete, DeleteBySession).
- **`DebateLogRepository`** -- Append-only logging for `debate_logs` with retention policy (Insert, GetByDebateID, GetByParticipantIdentifier, CleanupExpiredLogs, StartCleanupWorker).

### Knowledge Repository (`internal/debate/knowledge/`)

- **`ExtractLessons(ctx, result)`** -- Extracts and persists lessons from a completed debate.
- **`SearchLessons(ctx, query, options)`** -- Full-text search over lesson content.
- **`GetDebateHistory(ctx, filter)`** -- Retrieves debate history with filtering by domain, success, consensus, and date range.
- **`GetStatistics(ctx)`** -- Aggregate statistics: total lessons, patterns, strategies, success rates.

## Go Code vs SQL Schema Notes

The Go repository `CreateTable` methods serve as runtime fallbacks for environments where the SQL schema files have not been applied. There are minor type differences between the Go runtime DDL and the authoritative SQL schema files:

| Column | SQL Schema File | Go CreateTable (runtime fallback) | Notes |
|--------|-----------------|-----------------------------------|-------|
| `debate_sessions.id` | `UUID DEFAULT gen_random_uuid()` | `VARCHAR(255)` | Go code provides caller-generated IDs |
| `debate_sessions.topology_type` | `VARCHAR(50)` | `VARCHAR(100)` | Go uses wider column |
| `debate_sessions.coordination_protocol` | `VARCHAR(50)` | `VARCHAR(100)` | Go uses wider column |
| `debate_sessions.outcome` | `JSONB DEFAULT '{}'` | `TEXT` | Go scans as string regardless |
| `debate_sessions.config` | `JSONB DEFAULT '{}'` | `JSONB` (no default) | Minor default difference |

The SQL schema files under `sql/schema/` are the canonical reference. The Go `CreateTable` methods are compatible at runtime because PostgreSQL implicitly handles type coercion between VARCHAR/UUID and TEXT/JSONB for the operations performed by the repository.

## Related Files

- `sql/schema/debate_sessions.sql` -- Session table DDL (authoritative)
- `sql/schema/debate_turns.sql` -- Turn table DDL (authoritative)
- `sql/schema/code_versions.sql` -- Code version table DDL (authoritative)
- `sql/schema/debate_system.sql` -- Debate logs table DDL (authoritative)
- `internal/database/debate_session_repository.go` -- Session repository
- `internal/database/debate_turn_repository.go` -- Turn repository
- `internal/database/code_version_repository.go` -- Code version repository
- `internal/database/debate_log_repository.go` -- Debate log repository
- `internal/debate/knowledge/repository.go` -- Knowledge extraction repository
