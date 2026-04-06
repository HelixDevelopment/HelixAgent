# HelixAgent Database Schema

## Overview

HelixAgent uses PostgreSQL 15 as its primary data store. This document describes the database schema, relationships, and indexing strategies.

## Schema Diagram

```
┌────────────────────┐     ┌────────────────────┐     ┌────────────────────┐
│      users         │     │  user_sessions     │     │   llm_providers    │
├────────────────────┤     ├────────────────────┤     ├────────────────────┤
│ id (PK)            │◀────│ user_id (FK)       │     │ id (PK)            │
│ username           │     │ id (PK)            │     │ name               │
│ email              │     │ session_token      │     │ type               │
│ password_hash      │     │ context (JSONB)    │     │ enabled            │
│ api_key            │     │ status             │     │ weight             │
│ role               │     │ request_count      │     │ health_status      │
│ created_at         │     │ expires_at         │     │ verification_score │
│ updated_at         │     │ created_at         │     │ created_at         │
└────────────────────┘     └────────────────────┘     └────────────────────┘
                                    │                          │
                                    ▼                          ▼
┌────────────────────┐     ┌────────────────────┐     ┌────────────────────┐
│  llm_requests      │     │  llm_responses     │     │  models_metadata   │
├────────────────────┤     ├────────────────────┤     ├────────────────────┤
│ id (PK)            │◀────│ request_id (FK)    │     │ id (PK)            │
│ session_id (FK)    │     │ id (PK)            │     │ model_id           │
│ user_id (FK)       │     │ provider_id (FK)   │     │ provider_id (FK)   │
│ prompt             │     │ content            │     │ context_window     │
│ messages (JSONB)   │     │ confidence         │     │ capabilities       │
│ status             │     │ tokens_used        │     │ benchmark_score    │
│ created_at         │     │ response_time      │     │ created_at         │
└────────────────────┘     │ selected           │     └────────────────────┘
                           └────────────────────┘
                                                      ┌────────────────────┐
┌────────────────────┐     ┌────────────────────┐     │  debate_turns      │
│  debate_logs       │     │  debate_sessions   │◀────├────────────────────┤
├────────────────────┤     ├────────────────────┤     │ id (PK)            │
│ id (PK)            │     │ id (PK)            │     │ session_id (FK)    │
│ debate_id          │     │ debate_id          │     │ round              │
│ session_id         │     │ topic              │     │ phase              │
│ participant_id     │     │ status             │     │ agent_id           │
│ role               │     │ topology_type      │     │ content            │
│ provider           │     │ config (JSONB)     │     │ confidence         │
│ round              │     │ total_rounds       │     │ reflections (JSONB)│
│ action             │     │ consensus_score    │     │ created_at         │
│ quality_score      │     │ outcome (JSONB)    │     └────────────────────┘
│ metadata (JSONB)   │     │ created_at         │              │
│ expires_at         │     └────────────────────┘              │
└────────────────────┘              │                          ▼
                                    │              ┌────────────────────┐
┌────────────────────┐              └─────────────▶│  code_versions     │
│  background_tasks  │                             ├────────────────────┤
├────────────────────┤     ┌────────────────────┐  │ id (PK)            │
│ id (PK)            │◀────│ task_exec_history  │  │ session_id (FK)    │
│ task_type          │     ├────────────────────┤  │ turn_id (FK)       │
│ payload (JSONB)    │     │ task_id (FK)       │  │ code               │
│ status             │     │ event_type         │  │ version_number     │
│ priority           │     │ event_data (JSONB) │  │ quality_score      │
│ max_retries        │     │ created_at         │  │ test_pass_rate     │
│ worker_id          │     └────────────────────┘  │ metrics (JSONB)    │
│ created_at         │                             └────────────────────┘
└────────────────────┘
         │
         ▼
┌────────────────────┐     ┌────────────────────┐
│ webhook_deliveries │     │ vector_documents   │
├────────────────────┤     ├────────────────────┤
│ id (PK)            │     │ id (PK)            │
│ task_id (FK)       │     │ title              │
│ webhook_url        │     │ content            │
│ event_type         │     │ metadata (JSONB)   │
│ payload (JSONB)    │     │ embedding_id       │
│ status             │     │ embedding_provider │
│ attempts           │     │ created_at         │
│ created_at         │     └────────────────────┘
└────────────────────┘
```

## Table Definitions

### users

Stores user account information.

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_role ON users(role);
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Primary key |
| email | VARCHAR(255) | User email (unique) |
| password_hash | VARCHAR(255) | Bcrypt password hash |
| role | VARCHAR(50) | User role (user, admin, service) |
| status | VARCHAR(20) | Account status (active, suspended, deleted) |
| metadata | JSONB | Additional user metadata |

### api_keys

Stores API keys for authentication.

```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    scopes TEXT[] DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    rate_limit INTEGER DEFAULT 1000,
    status VARCHAR(20) NOT NULL DEFAULT 'active'
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_status ON api_keys(status);
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| key_hash | VARCHAR(255) | SHA-256 hash of API key |
| scopes | TEXT[] | Allowed scopes (completions, debates, admin) |
| rate_limit | INTEGER | Requests per minute limit |

### providers

Stores LLM provider configurations.

```sql
CREATE TABLE providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL,
    enabled BOOLEAN DEFAULT true,
    priority INTEGER DEFAULT 5,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    verification_score DECIMAL(4,2) DEFAULT 0.0,
    last_verified TIMESTAMP WITH TIME ZONE,
    health_status VARCHAR(20) DEFAULT 'unknown',
    capabilities JSONB DEFAULT '{}'::jsonb,
    rate_limits JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_providers_name ON providers(name);
CREATE INDEX idx_providers_enabled ON providers(enabled);
CREATE INDEX idx_providers_type ON providers(type);
CREATE INDEX idx_providers_score ON providers(verification_score DESC);
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| type | VARCHAR(50) | Provider type (apikey, oauth, free) |
| verification_score | DECIMAL(4,2) | LLMsVerifier score (0-10) |
| health_status | VARCHAR(20) | Current health (healthy, degraded, unhealthy, unknown) |
| capabilities | JSONB | Provider capabilities (streaming, tools, vision) |

### provider_configs

Stores encrypted provider credentials.

```sql
CREATE TABLE provider_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value TEXT NOT NULL,
    encrypted BOOLEAN DEFAULT true,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(provider_id, key)
);

CREATE INDEX idx_provider_configs_provider ON provider_configs(provider_id);
```

### sessions

Stores user sessions for stateful interactions.

```sql
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES providers(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    last_activity TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb,
    context JSONB DEFAULT '[]'::jsonb
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
CREATE INDEX idx_sessions_last_activity ON sessions(last_activity);
```

### completions

Stores completion requests and responses for auditing.

```sql
CREATE TABLE completions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,
    provider_id UUID NOT NULL REFERENCES providers(id),
    prompt TEXT NOT NULL,
    response TEXT,
    tokens_in INTEGER DEFAULT 0,
    tokens_out INTEGER DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    cached BOOLEAN DEFAULT false,
    error TEXT,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_completions_session ON completions(session_id);
CREATE INDEX idx_completions_provider ON completions(provider_id);
CREATE INDEX idx_completions_created ON completions(created_at DESC);
CREATE INDEX idx_completions_cached ON completions(cached);
```

### background_tasks

Stores background task queue.

```sql
CREATE TABLE background_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    priority INTEGER DEFAULT 5,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    worker_id VARCHAR(100),
    result JSONB,
    scheduled_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_tasks_status ON background_tasks(status);
CREATE INDEX idx_tasks_priority ON background_tasks(priority DESC);
CREATE INDEX idx_tasks_created ON background_tasks(created_at);
CREATE INDEX idx_tasks_type ON background_tasks(type);
CREATE INDEX idx_tasks_scheduled ON background_tasks(scheduled_at) WHERE scheduled_at IS NOT NULL;
CREATE INDEX idx_tasks_pending ON background_tasks(priority DESC, created_at) WHERE status = 'pending';
```

**Task Status Values:**
- `pending` - Task is waiting to be processed
- `queued` - Task has been picked up by a worker
- `running` - Task is currently executing
- `completed` - Task finished successfully
- `failed` - Task failed after all retries
- `stuck` - Task hasn't progressed (detected by stuck detector)
- `cancelled` - Task was cancelled

### task_events

Stores task lifecycle events for real-time notifications.

```sql
CREATE TABLE task_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES background_tasks(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    data JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_task_events_task ON task_events(task_id);
CREATE INDEX idx_task_events_type ON task_events(event_type);
CREATE INDEX idx_task_events_created ON task_events(created_at DESC);
```

**Event Types:**
- `task.created` - Task was submitted
- `task.started` - Task started executing
- `task.progress` - Task progress update
- `task.completed` - Task finished successfully
- `task.failed` - Task failed
- `task.cancelled` - Task was cancelled

### debate_logs

Append-only log of every participant action in every debate round. Uses string-based identifiers for flexibility. Supports time-based retention via `expires_at`.

```sql
CREATE TABLE debate_logs (
    id SERIAL PRIMARY KEY,
    debate_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL,
    participant_id INTEGER,
    participant_identifier VARCHAR(255),
    participant_name VARCHAR(255),
    role VARCHAR(100),
    provider VARCHAR(100),
    model VARCHAR(255),
    round INTEGER,
    action VARCHAR(100),
    response_time_ms BIGINT,
    quality_score DECIMAL(5,4),
    tokens_used INTEGER,
    content_length INTEGER,
    error_message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_debate_logs_debate_id ON debate_logs(debate_id);
CREATE INDEX idx_debate_logs_session_id ON debate_logs(session_id);
CREATE INDEX idx_debate_logs_provider ON debate_logs(provider);
CREATE INDEX idx_debate_logs_model ON debate_logs(model);
CREATE INDEX idx_debate_logs_created_at ON debate_logs(created_at);
CREATE INDEX idx_debate_logs_expires_at ON debate_logs(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_debate_logs_debate_round ON debate_logs(debate_id, round);
CREATE INDEX idx_debate_logs_active ON debate_logs(debate_id) WHERE expires_at IS NULL OR expires_at > NOW();
CREATE INDEX idx_debate_logs_provider_model ON debate_logs(provider, model);
CREATE INDEX idx_debate_logs_metadata ON debate_logs USING GIN (metadata);
```

**Action Values:**
- `response` - Agent response in a round
- `rebuttal` - Counter-argument
- `summary` - Round summary
- `vote` - Voting action
- `synthesis` - Final synthesis

### debate_sessions

Tracks the complete lifecycle of a debate session with full metadata for replay/recovery. Supports pause/resume via status transitions for approval gates.

```sql
CREATE TABLE debate_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    debate_id VARCHAR(255) NOT NULL,
    topic TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    topology_type VARCHAR(50),
    coordination_protocol VARCHAR(50),
    config JSONB DEFAULT '{}',
    initiated_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    total_rounds INTEGER DEFAULT 0,
    final_consensus_score DECIMAL(5,4),
    outcome JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}'
);

ALTER TABLE debate_sessions
    ADD CONSTRAINT chk_debate_sessions_status
    CHECK (status IN ('pending', 'running', 'paused', 'completed', 'failed', 'cancelled'));

CREATE INDEX idx_debate_sessions_debate_id ON debate_sessions(debate_id);
CREATE INDEX idx_debate_sessions_status ON debate_sessions(status);
CREATE INDEX idx_debate_sessions_created_at ON debate_sessions(created_at);
CREATE INDEX idx_debate_sessions_topology ON debate_sessions(topology_type);
CREATE INDEX idx_debate_sessions_active ON debate_sessions(status) WHERE status IN ('pending', 'running', 'paused');
CREATE INDEX idx_debate_sessions_metadata ON debate_sessions USING GIN (metadata);
CREATE INDEX idx_debate_sessions_config ON debate_sessions USING GIN (config);
CREATE INDEX idx_debate_sessions_debate_status ON debate_sessions(debate_id, status);
```

**Debate Session Status Values:**
- `pending` - Session not yet started
- `running` - Debate in progress
- `paused` - Waiting for human approval (approval gates)
- `completed` - Debate finished successfully
- `failed` - Debate failed
- `cancelled` - Debate was cancelled

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| debate_id | VARCHAR(255) | Links to debate_logs for correlation |
| topology_type | VARCHAR(50) | graph_mesh, star, chain, tree |
| coordination_protocol | VARCHAR(50) | cpde, dpde, adaptive |
| config | JSONB | Max rounds, timeout, consensus threshold, gates |
| final_consensus_score | DECIMAL(5,4) | Final consensus level (0.0-1.0) |
| outcome | JSONB | Winner, voting method, confidence, summary |

### debate_turns

Stores every individual agent action within a debate round and phase. Enables full debate replay, provenance tracking, and failure analysis.

```sql
CREATE TABLE debate_turns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES debate_sessions(id) ON DELETE CASCADE,
    round INTEGER NOT NULL,
    phase VARCHAR(50) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    agent_role VARCHAR(100),
    provider VARCHAR(100),
    model VARCHAR(255),
    content TEXT,
    confidence DECIMAL(5,4),
    tool_calls JSONB DEFAULT '[]',
    test_results JSONB DEFAULT '{}',
    reflections JSONB DEFAULT '[]',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    response_time_ms INTEGER
);

ALTER TABLE debate_turns
    ADD CONSTRAINT chk_debate_turns_phase
    CHECK (phase IN (
        'dehallucination', 'self_evolvement', 'proposal', 'critique',
        'review', 'optimization', 'adversarial', 'convergence'
    ));

CREATE INDEX idx_debate_turns_session_id ON debate_turns(session_id);
CREATE INDEX idx_debate_turns_session_round ON debate_turns(session_id, round);
CREATE INDEX idx_debate_turns_phase ON debate_turns(phase);
CREATE INDEX idx_debate_turns_agent ON debate_turns(agent_id);
CREATE INDEX idx_debate_turns_session_round_phase ON debate_turns(session_id, round, phase);
CREATE INDEX idx_debate_turns_created_at ON debate_turns(created_at);
CREATE INDEX idx_debate_turns_reflections ON debate_turns USING GIN (reflections) WHERE reflections != '[]'::jsonb;
CREATE INDEX idx_debate_turns_tool_calls ON debate_turns USING GIN (tool_calls) WHERE tool_calls != '[]'::jsonb;
CREATE INDEX idx_debate_turns_metadata ON debate_turns USING GIN (metadata);
```

**Phase Values (8-phase protocol):**
- `dehallucination` - Fact verification
- `self_evolvement` - Self-improvement
- `proposal` - Initial proposal
- `critique` - Critical review
- `review` - Peer review
- `optimization` - Solution optimization
- `adversarial` - Red/blue team attack-defend
- `convergence` - Final convergence

### code_versions

Captures code snapshots at key milestones during a debate, enabling solution comparison, rollback, and quality trend analysis.

```sql
CREATE TABLE code_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES debate_sessions(id) ON DELETE CASCADE,
    turn_id UUID REFERENCES debate_turns(id) ON DELETE SET NULL,
    language VARCHAR(50),
    code TEXT NOT NULL,
    version_number INTEGER NOT NULL,
    quality_score DECIMAL(5,4),
    test_pass_rate DECIMAL(5,4),
    metrics JSONB DEFAULT '{}',
    diff_from_previous TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uq_code_versions_session_version UNIQUE (session_id, version_number)
);

CREATE INDEX idx_code_versions_session_id ON code_versions(session_id);
CREATE INDEX idx_code_versions_turn_id ON code_versions(turn_id);
CREATE INDEX idx_code_versions_session_version ON code_versions(session_id, version_number);
CREATE INDEX idx_code_versions_language ON code_versions(language);
CREATE INDEX idx_code_versions_quality ON code_versions(quality_score) WHERE quality_score IS NOT NULL;
CREATE INDEX idx_code_versions_test_pass_rate ON code_versions(test_pass_rate) WHERE test_pass_rate IS NOT NULL;
CREATE INDEX idx_code_versions_metrics ON code_versions USING GIN (metrics);
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| session_id | UUID | FK to debate_sessions (CASCADE delete) |
| turn_id | UUID | FK to debate_turns (SET NULL on delete) |
| version_number | INTEGER | Sequential version within session (unique per session) |
| quality_score | DECIMAL(5,4) | Overall quality (0.0-1.0) |
| test_pass_rate | DECIMAL(5,4) | Test pass percentage (0.0-1.0) |
| metrics | JSONB | Maintainability, complexity, security scores |

### webhook_deliveries

Webhook notification delivery tracking with retry support.

```sql
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID REFERENCES background_tasks(id) ON DELETE SET NULL,
    webhook_url TEXT NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    last_attempt_at TIMESTAMP WITH TIME ZONE,
    last_error TEXT,
    response_code INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    delivered_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_webhook_deliveries_task ON webhook_deliveries(task_id);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status) WHERE status != 'delivered';
```

**Webhook Status Values:**
- `pending` - Awaiting delivery
- `delivered` - Successfully delivered
- `failed` - Delivery failed after all retries
- `retrying` - Retrying delivery

### vector_documents

Documents with vector embeddings for semantic search via pgvector.

```sql
CREATE TABLE vector_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    embedding_id UUID,
    embedding_provider VARCHAR(50) DEFAULT 'pgvector',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_vector_documents_embedding_provider ON vector_documents(embedding_provider);
```

### agentic_workflows

Stores workflow definitions, execution state, and results for graph-based agentic orchestration.

```sql
CREATE TABLE agentic_workflows (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    entry_point VARCHAR(64) NOT NULL,
    config JSONB,
    input JSONB,
    result JSONB,
    error TEXT,
    nodes_executed INTEGER DEFAULT 0,
    execution_time_ms BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);
```

### planning_sessions

Stores HiPlan, MCTS, and Tree of Thoughts planning results.

```sql
CREATE TABLE planning_sessions (
    id VARCHAR(64) PRIMARY KEY,
    algorithm VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    input JSONB NOT NULL,
    config JSONB,
    result JSONB,
    error TEXT,
    execution_time_ms BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);
```

### llmops_experiments

Stores A/B experiments, evaluations, and prompt versions for LLM operations.

```sql
CREATE TABLE llmops_experiments (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'created',
    variants JSONB NOT NULL,
    metrics JSONB,
    config JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);
```

### feature_flags

Feature flag management.

```sql
CREATE TABLE feature_flags (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT false,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### benchmark_runs

Benchmark run results for provider comparison.

```sql
CREATE TABLE benchmark_runs (
    id VARCHAR(100) PRIMARY KEY,
    benchmark_type VARCHAR(50) NOT NULL,
    provider_name VARCHAR(100) NOT NULL,
    model_name VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    pass_rate REAL,
    average_score REAL,
    average_latency_ns BIGINT,
    total_tasks INTEGER,
    passed_tasks INTEGER,
    failed_tasks INTEGER,
    config JSONB,
    summary JSONB,
    started_at TIMESTAMP WITH TIME ZONE,
    ended_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### security_scan_history

Security scan result history.

```sql
CREATE TABLE security_scan_history (
    id SERIAL PRIMARY KEY,
    tool_name VARCHAR(100) NOT NULL,
    scan_type VARCHAR(50) NOT NULL,
    findings_critical INTEGER DEFAULT 0,
    findings_high INTEGER DEFAULT 0,
    findings_medium INTEGER DEFAULT 0,
    findings_low INTEGER DEFAULT 0,
    findings_info INTEGER DEFAULT 0,
    scan_duration_ms BIGINT,
    report_path TEXT,
    scanned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### performance_baselines

Performance benchmark baselines for regression detection.

```sql
CREATE TABLE performance_baselines (
    id SERIAL PRIMARY KEY,
    metric_name VARCHAR(255) NOT NULL,
    package_name VARCHAR(255) NOT NULL,
    baseline_ns BIGINT NOT NULL,
    baseline_allocs BIGINT,
    baseline_bytes BIGINT,
    captured_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(metric_name, package_name)
);
```

## Migrations

Migrations are managed using golang-migrate. Files are located in `migrations/`.

```bash
# Run migrations
make migrate-up

# Rollback last migration
make migrate-down

# Create new migration
make migrate-create NAME=add_new_table
```

## Indexes Strategy

### Primary Lookups
- All primary keys use UUID for distributed systems
- Unique constraints on business identifiers (email, name)

### Query Optimization
- Status-based indexes for queue polling
- Composite indexes for frequent queries
- Partial indexes for specific conditions (e.g., pending tasks)

### Timestamp Indexes
- DESC ordering for recent-first queries
- Used for audit trails and history

## Connection Pooling

```yaml
database:
  host: localhost
  port: 5432
  user: helixagent
  password: secret
  name: helixagent_db
  pool:
    max_open: 25
    max_idle: 5
    max_lifetime: 1h
    max_idle_time: 15m
```

## Backup Strategy

```bash
# Daily full backup
pg_dump -Fc helixagent_db > backup_$(date +%Y%m%d).dump

# Point-in-time recovery with WAL archiving
archive_mode = on
archive_command = 'cp %p /archive/%f'
```

---

**Document Version**: 2.0
**Last Updated**: April 6, 2026
**Author**: Generated by Claude Code
