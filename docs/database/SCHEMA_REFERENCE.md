# Database Schema Reference

HelixAgent uses PostgreSQL 15+ with a schema covering users, LLM providers, request/response logging, background tasks, AI debate, knowledge management, and protocol support.

## Schema Overview

| Domain | Tables | SQL File |
|--------|--------|----------|
| Users & Sessions | `users`, `user_sessions` | `sql/schema/users_sessions.sql` |
| LLM Providers | `llm_providers`, `models_metadata`, `model_benchmarks` | `sql/schema/llm_providers.sql` |
| Requests & Responses | `llm_requests`, `llm_responses` | `sql/schema/requests_responses.sql` |
| Background Tasks | `background_tasks`, `background_tasks_dead_letter`, `task_execution_history`, `task_resource_snapshots`, `webhook_deliveries` | `sql/schema/background_tasks.sql` |
| AI Debate (Logs) | `debate_logs` | `sql/schema/debate_system.sql` |
| Debate Persistence | `debate_sessions`, `debate_turns`, `code_versions` | `sql/schema/debate_sessions.sql`, `sql/schema/debate_turns.sql`, `sql/schema/code_versions.sql` |
| Knowledge | `cognee_memories` | `sql/schema/cognee_memories.sql` |
| Protocols | `mcp_servers`, `lsp_servers`, `acp_servers`, `embedding_config`, `protocol_cache`, `protocol_metrics` | `sql/schema/protocol_support.sql` |
| Agentic Workflows | `agentic_workflows`, `agentic_workflow_nodes`, `agentic_workflow_edges` | `sql/schema/agentic_workflows.sql` |
| LLMOps | `llmops_experiments`, `llmops_evaluations`, `llmops_prompt_versions` | `sql/schema/llmops_experiments.sql` |
| Planning | `planning_sessions`, `planning_hiplan_milestones`, `planning_mcts_nodes` | `sql/schema/planning_sessions.sql` |
| CLI Agents | `cli_agent_instances`, `cli_agent_tasks`, `repo_maps`, `repo_symbols`, `git_operations`, `diff_applications`, `terminal_sessions`, `tool_use_log`, `project_memory`, `browser_sessions`, `sandbox_environments`, `task_plans` | `sql/001_cli_agents_fusion.sql` |
| Performance & Security | `feature_flags`, `performance_baselines`, `security_scan_history`, `benchmark_runs` | `sql/002_performance_and_security.sql` |
| Vector Documents | `vector_documents` | `sql/schema/complete_schema.sql` |
| Analytics (ClickHouse) | `debate_metrics`, `conversation_metrics`, `provider_performance`, `llm_response_latency`, `entity_extraction_metrics`, `memory_operations`, `debate_winners`, `system_health`, `api_requests` | `sql/schema/clickhouse_analytics.sql` |
| Cross-Session Learning | `cross_session_*` tables | `sql/schema/cross_session_learning.sql` |
| Streaming Analytics | `streaming_*` tables | `sql/schema/streaming_analytics.sql` |
| Distributed Memory | `distributed_*` tables | `sql/schema/distributed_memory.sql` |
| Conversation Context | `conversation_*` tables | `sql/schema/conversation_context.sql` |
| Performance | Indexes, materialized views | `sql/schema/indexes_views.sql` |

**Consolidated reference**: `sql/schema/complete_schema.sql` contains all tables with comprehensive comments.

**Relationships**: `sql/schema/relationships.sql` documents all foreign key constraints and entity relationships.

## Core Tables

### Users & Sessions

```sql
-- Users table: authentication and profile
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        VARCHAR(255) UNIQUE NOT NULL,
    email           VARCHAR(255) UNIQUE NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    role            VARCHAR(50) DEFAULT 'user',
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- User sessions: JWT token tracking
CREATE TABLE user_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      VARCHAR(255) NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);
```

### LLM Providers

```sql
-- Provider registry with scoring
CREATE TABLE llm_providers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) UNIQUE NOT NULL,
    display_name    VARCHAR(255),
    provider_type   VARCHAR(50) NOT NULL,    -- "api_key", "oauth", "free"
    is_enabled      BOOLEAN DEFAULT true,
    verification_score DECIMAL(4,2),          -- LLMsVerifier score (0-10)
    last_verified   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Models available per provider
CREATE TABLE llm_models (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id     UUID NOT NULL REFERENCES llm_providers(id),
    model_name      VARCHAR(255) NOT NULL,
    capabilities    JSONB DEFAULT '{}',
    max_tokens      INTEGER,
    cost_per_token  DECIMAL(10,8),
    is_active       BOOLEAN DEFAULT true
);
```

### Request/Response Logging

```sql
-- All LLM API requests
CREATE TABLE llm_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID REFERENCES users(id),
    provider_id     UUID REFERENCES llm_providers(id),
    model_name      VARCHAR(255),
    prompt_tokens   INTEGER,
    request_body    JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- LLM API responses
CREATE TABLE llm_responses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      UUID NOT NULL REFERENCES llm_requests(id),
    completion_tokens INTEGER,
    total_tokens    INTEGER,
    response_body   JSONB,
    latency_ms      INTEGER,
    status          VARCHAR(50),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);
```

### Background Tasks

```sql
-- Async task queue
CREATE TABLE background_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_type       VARCHAR(100) NOT NULL,
    payload         JSONB NOT NULL,
    status          VARCHAR(50) DEFAULT 'pending',  -- pending/queued/running/completed/failed/stuck/cancelled
    priority        INTEGER DEFAULT 0,
    max_retries     INTEGER DEFAULT 3,
    retry_count     INTEGER DEFAULT 0,
    scheduled_at    TIMESTAMPTZ,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    error_message   TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Dead letter queue for permanently failed tasks
CREATE TABLE dead_letter_queue (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id         UUID NOT NULL REFERENCES background_tasks(id),
    failure_reason  TEXT,
    original_payload JSONB,
    moved_at        TIMESTAMPTZ DEFAULT NOW()
);
```

### AI Debate System

```sql
-- Debate logs: append-only log of participant actions (no FKs, string-based IDs)
CREATE TABLE debate_logs (
    id              SERIAL PRIMARY KEY,
    debate_id       VARCHAR(255) NOT NULL,
    session_id      VARCHAR(255) NOT NULL,
    participant_id  INTEGER,
    participant_identifier VARCHAR(255),
    participant_name VARCHAR(255),
    role            VARCHAR(100),
    provider        VARCHAR(100),
    model           VARCHAR(255),
    round           INTEGER,
    action          VARCHAR(100),
    response_time_ms BIGINT,
    quality_score   DECIMAL(5,4),
    tokens_used     INTEGER,
    content_length  INTEGER,
    error_message   TEXT,
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    expires_at      TIMESTAMPTZ
);

-- Debate sessions: lifecycle tracking with full metadata for replay/recovery
CREATE TABLE debate_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    debate_id       VARCHAR(255) NOT NULL,
    topic           TEXT NOT NULL,
    status          VARCHAR(50) NOT NULL DEFAULT 'pending',
    topology_type   VARCHAR(50),
    coordination_protocol VARCHAR(50),
    config          JSONB DEFAULT '{}',
    initiated_by    VARCHAR(255),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    total_rounds    INTEGER DEFAULT 0,
    final_consensus_score DECIMAL(5,4),
    outcome         JSONB DEFAULT '{}',
    metadata        JSONB DEFAULT '{}'
);

-- Debate turns: individual agent actions within rounds and phases
CREATE TABLE debate_turns (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID NOT NULL REFERENCES debate_sessions(id) ON DELETE CASCADE,
    round           INTEGER NOT NULL,
    phase           VARCHAR(50) NOT NULL,
    agent_id        VARCHAR(255) NOT NULL,
    agent_role      VARCHAR(100),
    provider        VARCHAR(100),
    model           VARCHAR(255),
    content         TEXT,
    confidence      DECIMAL(5,4),
    tool_calls      JSONB DEFAULT '[]',
    test_results    JSONB DEFAULT '{}',
    reflections     JSONB DEFAULT '[]',
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    response_time_ms INTEGER
);

-- Code versions: snapshots at debate milestones
CREATE TABLE code_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID NOT NULL REFERENCES debate_sessions(id) ON DELETE CASCADE,
    turn_id         UUID REFERENCES debate_turns(id) ON DELETE SET NULL,
    language        VARCHAR(50),
    code            TEXT NOT NULL,
    version_number  INTEGER NOT NULL,
    quality_score   DECIMAL(5,4),
    test_pass_rate  DECIMAL(5,4),
    metrics         JSONB DEFAULT '{}',
    diff_from_previous TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (session_id, version_number)
);
```

## Relationships

Key foreign key relationships:

- `user_sessions.user_id` → `users.id` (CASCADE)
- `models_metadata.provider_id` → `llm_providers.id` (CASCADE)
- `model_benchmarks.model_id` → `models_metadata.model_id` (CASCADE)
- `llm_requests.user_id` → `users.id` (CASCADE)
- `llm_requests.session_id` → `user_sessions.id` (CASCADE)
- `llm_responses.request_id` → `llm_requests.id` (CASCADE)
- `llm_responses.provider_id` → `llm_providers.id` (SET NULL)
- `cognee_memories.session_id` → `user_sessions.id` (CASCADE)
- `background_tasks.parent_task_id` → `background_tasks.id` (SET NULL)
- `task_execution_history.task_id` → `background_tasks.id` (CASCADE)
- `task_resource_snapshots.task_id` → `background_tasks.id` (CASCADE)
- `webhook_deliveries.task_id` → `background_tasks.id` (SET NULL)
- `debate_turns.session_id` → `debate_sessions.id` (CASCADE)
- `code_versions.session_id` → `debate_sessions.id` (CASCADE)
- `code_versions.turn_id` → `debate_turns.id` (SET NULL)
- `agentic_workflow_nodes.workflow_id` → `agentic_workflows.id` (CASCADE)
- `agentic_workflow_edges.workflow_id` → `agentic_workflows.id` (CASCADE)
- `planning_hiplan_milestones.session_id` → `planning_sessions.id` (CASCADE)
- `planning_mcts_nodes.session_id` → `planning_sessions.id` (CASCADE)

**Logical references (no FK constraint):**
- `debate_sessions.debate_id` -- string-based link to `debate_logs.debate_id`
- `debate_logs.session_id` -- string-based session reference
- `protocol_metrics.server_id` -- logical reference to `mcp_servers.id`, `lsp_servers.id`, or `acp_servers.id`

See `sql/schema/relationships.sql` for the complete relationship documentation.

## ER Diagram

Visual entity-relationship diagrams:

- `docs/diagrams/src/database-er.mmd` - Mermaid ER diagram
- `docs/diagrams/src/database-er.puml` - PlantUML class diagram

Generate:
```bash
./scripts/generate-diagrams.sh
```

## Migrations

Migrations are in `migrations/` directory (files 001-014). The consolidated schema in `sql/schema/complete_schema.sql` represents the current state after all migrations.

## SQL Files

| File | Description |
|------|-------------|
| `sql/schema/complete_schema.sql` | Full consolidated schema |
| `sql/schema/users_sessions.sql` | Users and session tables |
| `sql/schema/llm_providers.sql` | LLM provider and model tables |
| `sql/schema/requests_responses.sql` | Request/response logging |
| `sql/schema/background_tasks.sql` | Task queue, dead letter, execution history, resource snapshots, webhooks |
| `sql/schema/debate_system.sql` | AI debate logs (append-only) |
| `sql/schema/debate_sessions.sql` | Debate session lifecycle tracking |
| `sql/schema/debate_turns.sql` | Debate turn-level state for replay |
| `sql/schema/code_versions.sql` | Code snapshots at debate milestones |
| `sql/schema/cognee_memories.sql` | Knowledge management |
| `sql/schema/protocol_support.sql` | MCP/ACP/LSP/embedding/cache/metrics tables |
| `sql/schema/agentic_workflows.sql` | Agentic workflow orchestration |
| `sql/schema/llmops_experiments.sql` | LLMOps experiments, evaluations, prompts |
| `sql/schema/planning_sessions.sql` | HiPlan, MCTS, Tree of Thoughts planning |
| `sql/schema/clickhouse_analytics.sql` | ClickHouse time-series analytics |
| `sql/schema/streaming_analytics.sql` | Streaming analytics tables |
| `sql/schema/distributed_memory.sql` | Distributed memory tables |
| `sql/schema/conversation_context.sql` | Conversation context tables |
| `sql/schema/cross_session_learning.sql` | Cross-session learning tables |
| `sql/schema/indexes_views.sql` | Performance indexes and materialized views |
| `sql/schema/relationships.sql` | FK constraints documentation |
| `sql/001_cli_agents_fusion.sql` | CLI agent instances, tasks, repo maps, git ops, diffs, tools, memory |
| `sql/002_performance_and_security.sql` | Feature flags, performance baselines, security scans, benchmark runs |
