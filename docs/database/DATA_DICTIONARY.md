# HelixAgent SQL Data Dictionary

## Overview

PostgreSQL 15 with pgx/v5 driver. Schema managed via embedded Go migrations in `internal/database/db.go` (migrations 001-003) and standalone SQL files under `sql/schema/` (migrations 011-014). Extensions: `uuid-ossp` (UUID generation), `pgvector` (vector similarity search). ClickHouse is used separately for time-series analytics.

Custom ENUM types:
- **task_status**: `pending`, `queued`, `running`, `paused`, `completed`, `failed`, `stuck`, `cancelled`, `dead_letter`
- **task_priority**: `critical`, `high`, `normal`, `low`, `background`

---

## 1. User Management

### users

Registered user accounts. Each user has a unique username, email, and API key. The `role` column controls authorization.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| username | VARCHAR(255) | NO | -- | Unique username |
| email | VARCHAR(255) | NO | -- | Unique email address |
| password_hash | VARCHAR(255) | NO | -- | Bcrypt password hash |
| api_key | VARCHAR(255) | NO | -- | Unique API key for authentication |
| role | VARCHAR(50) | YES | `'user'` | Authorization role (`user`, `admin`) |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification timestamp |

**Unique constraints:** `username`, `email`, `api_key`

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_users_email | email | Lookup by email |
| idx_users_api_key | api_key | Lookup by API key |

---

### user_sessions

Active user sessions with conversation context. Sessions expire after a configurable TTL and track request counts for rate limiting.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| user_id | UUID | YES | -- | FK to `users(id)` |
| session_token | VARCHAR(255) | NO | -- | Unique session token |
| context | JSONB | YES | `'{}'` | Conversation context |
| memory_id | UUID | YES | -- | Logical reference to Cognee memory (external) |
| status | VARCHAR(50) | YES | `'active'` | `active`, `expired`, `revoked` |
| request_count | INTEGER | YES | `0` | Number of requests in this session |
| last_activity | TIMESTAMPTZ | YES | `NOW()` | Last activity timestamp |
| expires_at | TIMESTAMPTZ | NO | -- | Session expiration time |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| user_id | users(id) | CASCADE |

**Unique constraints:** `session_token`

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_user_sessions_user_id | user_id | Sessions by user |
| idx_user_sessions_expires_at | expires_at | Expiration queries |
| idx_user_sessions_session_token | session_token | Token lookup |
| idx_sessions_active | user_id, status, last_activity DESC | Partial: `status = 'active'` |
| idx_sessions_expired | expires_at | Partial: `status = 'active' AND expires_at < NOW()` |

---

## 2. LLM Infrastructure

### llm_providers

Registry of configured LLM providers. Weight controls ensemble voting influence. Health status is updated by periodic probes. Extended by migration 002 with Models.dev columns.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| name | VARCHAR(255) | NO | -- | Unique provider name |
| type | VARCHAR(100) | NO | -- | Provider type classification |
| api_key | VARCHAR(255) | YES | -- | API key for provider authentication |
| base_url | VARCHAR(500) | YES | -- | Provider base URL |
| model | VARCHAR(255) | YES | -- | Default model identifier |
| weight | DECIMAL(5,2) | YES | `1.0` | Ensemble voting weight |
| enabled | BOOLEAN | YES | `TRUE` | Whether provider is active |
| config | JSONB | YES | `'{}'` | Provider-specific configuration |
| health_status | VARCHAR(50) | YES | `'unknown'` | `healthy`, `degraded`, `unhealthy`, `unknown` |
| response_time | BIGINT | YES | `0` | Last response time in milliseconds |
| modelsdev_provider_id | VARCHAR(255) | YES | -- | Models.dev provider identifier |
| total_models | INTEGER | YES | `0` | Total models from Models.dev catalog |
| enabled_models | INTEGER | YES | `0` | Number of enabled models |
| last_models_sync | TIMESTAMPTZ | YES | -- | Last Models.dev sync timestamp |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification timestamp |

**Unique constraints:** `name`

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_llm_providers_name | name | Provider lookup |
| idx_llm_providers_enabled | enabled | Filter active providers |
| idx_providers_healthy_enabled | name, health_status, response_time | Partial: `enabled = TRUE AND health_status = 'healthy'` |
| idx_providers_by_weight | weight DESC, response_time ASC | Partial: `enabled = TRUE` |
| idx_providers_health_check | id, name, health_status, response_time, updated_at | Partial: `enabled = TRUE` |

---

### llm_requests

Every LLM completion request. Carries prompt, message history, model params, and optional ensemble configuration for multi-provider orchestration.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| session_id | UUID | YES | -- | FK to `user_sessions(id)` |
| user_id | UUID | YES | -- | FK to `users(id)` |
| prompt | TEXT | NO | -- | User prompt text |
| messages | JSONB | NO | `'[]'` | OpenAI-format message array |
| model_params | JSONB | NO | `'{}'` | Parameters: temperature, top_p, max_tokens, etc. |
| ensemble_config | JSONB | YES | `NULL` | Ensemble strategy (NULL = single provider) |
| memory_enhanced | BOOLEAN | YES | `FALSE` | Whether Cognee memories were injected |
| memory | JSONB | YES | `'{}'` | Injected memory data |
| status | VARCHAR(50) | YES | `'pending'` | `pending`, `running`, `completed`, `failed` |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| started_at | TIMESTAMPTZ | YES | -- | Execution start time |
| completed_at | TIMESTAMPTZ | YES | -- | Execution completion time |
| request_type | VARCHAR(50) | YES | `'completion'` | `completion`, `chat`, `embedding`, `vision` |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| session_id | user_sessions(id) | CASCADE |
| user_id | users(id) | CASCADE |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_llm_requests_session_id | session_id | Requests by session |
| idx_llm_requests_user_id | user_id | Requests by user |
| idx_llm_requests_status | status | Filter by status |
| idx_requests_session_status | session_id, status, created_at DESC | Partial: `status IN ('pending', 'completed')` |
| idx_requests_recent | created_at DESC | Partial: `created_at > NOW() - INTERVAL '24 hours'` |

---

### llm_responses

Individual provider responses. In ensemble mode, one request generates multiple responses. The `selected` flag marks the ensemble winner.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| request_id | UUID | YES | -- | FK to `llm_requests(id)` |
| provider_id | UUID | YES | -- | FK to `llm_providers(id)` |
| provider_name | VARCHAR(100) | NO | -- | Denormalized provider name for query speed |
| content | TEXT | NO | -- | Response content |
| confidence | DECIMAL(3,2) | NO | `0.0` | Confidence score (0.00-1.00) |
| tokens_used | INTEGER | YES | `0` | Total tokens consumed |
| response_time | BIGINT | YES | `0` | Response latency in milliseconds |
| finish_reason | VARCHAR(50) | YES | `'stop'` | `stop`, `length`, `content_filter`, `tool_calls` |
| metadata | JSONB | YES | `'{}'` | Additional response metadata |
| selected | BOOLEAN | YES | `FALSE` | Ensemble winner flag |
| selection_score | DECIMAL(5,2) | YES | `0.0` | Score used for ensemble selection |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| request_id | llm_requests(id) | CASCADE |
| provider_id | llm_providers(id) | SET NULL |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_llm_responses_request_id | request_id | Responses per request |
| idx_llm_responses_provider_id | provider_id | Responses by provider |
| idx_llm_responses_selected | selected | Find winners |
| idx_responses_selection | request_id, selected, selection_score DESC | Ensemble selection query |
| idx_responses_aggregation | provider_name, created_at DESC | INCLUDE (response_time, tokens_used, confidence) |

---

## 3. Model Metadata

### models_metadata

Model catalog from Models.dev. Stores capabilities, pricing, benchmarks, and protocol support. Extended by migration 003 with protocol columns.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| model_id | VARCHAR(255) | NO | -- | Unique model identifier |
| model_name | VARCHAR(255) | NO | -- | Human-readable model name |
| provider_id | VARCHAR(255) | NO | -- | FK to `llm_providers(id)` |
| provider_name | VARCHAR(255) | NO | -- | Provider display name |
| description | TEXT | YES | -- | Model description |
| context_window | INTEGER | YES | -- | Max context window size (tokens) |
| max_tokens | INTEGER | YES | -- | Max output tokens |
| pricing_input | DECIMAL(10,6) | YES | -- | Input pricing per 1K tokens |
| pricing_output | DECIMAL(10,6) | YES | -- | Output pricing per 1K tokens |
| pricing_currency | VARCHAR(10) | YES | `'USD'` | Pricing currency |
| supports_vision | BOOLEAN | YES | `FALSE` | Vision/image input capability |
| supports_function_calling | BOOLEAN | YES | `FALSE` | Tool/function calling capability |
| supports_streaming | BOOLEAN | YES | `FALSE` | Streaming response capability |
| supports_json_mode | BOOLEAN | YES | `FALSE` | JSON output mode capability |
| supports_image_generation | BOOLEAN | YES | `FALSE` | Image generation capability |
| supports_audio | BOOLEAN | YES | `FALSE` | Audio processing capability |
| supports_code_generation | BOOLEAN | YES | `FALSE` | Specialized code generation |
| supports_reasoning | BOOLEAN | YES | `FALSE` | Chain-of-thought reasoning |
| benchmark_score | DECIMAL(5,2) | YES | -- | Overall benchmark score |
| popularity_score | INTEGER | YES | -- | Popularity ranking |
| reliability_score | DECIMAL(5,2) | YES | -- | Reliability score |
| model_type | VARCHAR(100) | YES | -- | Model type classification |
| model_family | VARCHAR(100) | YES | -- | Model family (e.g., GPT, Claude) |
| version | VARCHAR(50) | YES | -- | Model version |
| tags | JSONB | YES | `'[]'` | Tags array |
| modelsdev_url | TEXT | YES | -- | Models.dev URL |
| modelsdev_id | VARCHAR(255) | YES | -- | Models.dev identifier |
| modelsdev_api_version | VARCHAR(50) | YES | -- | Models.dev API version |
| raw_metadata | JSONB | YES | `'{}'` | Raw metadata from source |
| protocol_support | JSONB | YES | `'[]'` | Supported protocols array |
| mcp_server_id | VARCHAR(255) | YES | -- | Associated MCP server |
| lsp_server_id | VARCHAR(255) | YES | -- | Associated LSP server |
| acp_server_id | VARCHAR(255) | YES | -- | Associated ACP server |
| embedding_provider | VARCHAR(50) | YES | `'pgvector'` | Embedding provider |
| protocol_config | JSONB | YES | `'{}'` | Protocol configuration |
| protocol_last_sync | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last protocol sync |
| last_refreshed_at | TIMESTAMPTZ | NO | -- | Last catalog refresh |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification (auto-trigger) |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| provider_id | llm_providers(id) | CASCADE |

**Unique constraints:** `model_id`

**Triggers:** `update_models_metadata_updated_at` -- auto-sets `updated_at` on UPDATE.

**Indexes:**
| Index | Column(s) | Type | Notes |
|-------|-----------|------|-------|
| idx_models_metadata_provider_id | provider_id | btree | |
| idx_models_metadata_model_type | model_type | btree | |
| idx_models_metadata_last_refreshed | last_refreshed_at | btree | |
| idx_models_metadata_model_family | model_family | btree | |
| idx_models_metadata_benchmark_score | benchmark_score | btree | |
| idx_models_metadata_tags | tags | GIN | JSONB array search |
| idx_models_metadata_protocol_support | protocol_support | GIN | JSONB array search |
| idx_models_by_capabilities | provider_name, supports_streaming, supports_function_calling | btree | Partial: `provider_name IS NOT NULL` |
| idx_models_scored | benchmark_score DESC, reliability_score DESC | btree | Partial: `benchmark_score IS NOT NULL` |

---

### model_benchmarks

Individual benchmark results per model (MMLU, HumanEval, GSM8K, etc.).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| model_id | VARCHAR(255) | NO | -- | FK to `models_metadata(model_id)` |
| benchmark_name | VARCHAR(255) | NO | -- | Benchmark name (e.g., MMLU, HumanEval) |
| benchmark_type | VARCHAR(100) | YES | -- | Category of benchmark |
| score | DECIMAL(10,4) | YES | -- | Raw benchmark score |
| rank | INTEGER | YES | -- | Rank among models |
| normalized_score | DECIMAL(5,2) | YES | -- | Score normalized to 0-100 |
| benchmark_date | DATE | YES | -- | Date benchmark was run |
| metadata | JSONB | YES | `'{}'` | Additional benchmark metadata |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| model_id | models_metadata(model_id) | CASCADE |

**Unique constraints:** `(model_id, benchmark_name)`

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_benchmarks_model_id | model_id | Benchmarks by model |
| idx_benchmarks_type | benchmark_type | Filter by type |
| idx_benchmarks_score | score | Score ordering |

---

### models_refresh_history

Audit log of Models.dev catalog refresh operations. Standalone table with no foreign keys.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| refresh_type | VARCHAR(50) | NO | -- | Type of refresh operation |
| status | VARCHAR(50) | NO | -- | Outcome status |
| models_refreshed | INTEGER | YES | `0` | Number of models successfully refreshed |
| models_failed | INTEGER | YES | `0` | Number of models that failed refresh |
| error_message | TEXT | YES | -- | Error details if failed |
| started_at | TIMESTAMPTZ | YES | `NOW()` | Refresh start time |
| completed_at | TIMESTAMPTZ | YES | -- | Refresh completion time |
| duration_seconds | INTEGER | YES | -- | Total refresh duration |
| metadata | JSONB | YES | `'{}'` | Additional metadata |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_refresh_history_started | started_at | Chronological lookup |
| idx_refresh_history_status | status | Filter by outcome |

---

## 4. Protocol Support

### mcp_servers

MCP (Model Context Protocol) server configuration. Local servers are spawned as subprocesses; remote servers are accessed via HTTP/SSE.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(255) | NO | -- | Primary key (server identifier) |
| name | VARCHAR(255) | NO | -- | Server display name |
| type | VARCHAR(20) | NO | -- | `local` or `remote` (CHECK constraint) |
| command | TEXT | YES | -- | Launch command (local servers) |
| url | TEXT | YES | -- | Endpoint URL (remote servers) |
| enabled | BOOLEAN | NO | `true` | Whether server is active |
| tools | JSONB | NO | `'[]'` | Tool definitions array |
| last_sync | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last synchronization |
| created_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last modification timestamp |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_mcp_servers_enabled | enabled | Filter active servers |
| idx_mcp_active_servers | type, enabled, last_sync DESC | Partial: `enabled = TRUE` |

---

### lsp_servers

LSP (Language Server Protocol) server configuration.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(255) | NO | -- | Primary key (server identifier) |
| name | VARCHAR(255) | NO | -- | Server display name |
| language | VARCHAR(50) | NO | -- | Programming language served |
| command | VARCHAR(500) | NO | -- | Launch command |
| enabled | BOOLEAN | NO | `true` | Whether server is active |
| workspace | VARCHAR(1000) | YES | `'/workspace'` | Workspace root path |
| capabilities | JSONB | NO | `'[]'` | LSP capabilities array |
| last_sync | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last synchronization |
| created_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last modification timestamp |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_lsp_servers_enabled | enabled | Filter active servers |

---

### acp_servers

ACP (Agent Communication Protocol) server configuration.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(255) | NO | -- | Primary key (server identifier) |
| name | VARCHAR(255) | NO | -- | Server display name |
| type | VARCHAR(20) | NO | -- | `local` or `remote` (CHECK constraint) |
| url | TEXT | YES | -- | Endpoint URL |
| enabled | BOOLEAN | NO | `true` | Whether server is active |
| tools | JSONB | NO | `'[]'` | Tool definitions array |
| last_sync | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last synchronization |
| created_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last modification timestamp |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_acp_servers_enabled | enabled | Filter active servers |

---

### embedding_config

Embedding provider configuration for vector operations.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | SERIAL | NO | auto-increment | Primary key |
| provider | VARCHAR(50) | NO | `'pgvector'` | Embedding provider name |
| model | VARCHAR(100) | NO | `'text-embedding-ada-002'` | Embedding model identifier |
| dimension | INTEGER | NO | `1536` | Embedding vector dimension |
| api_endpoint | TEXT | YES | -- | Provider API endpoint |
| api_key | TEXT | YES | -- | Provider API key |
| created_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last modification timestamp |

---

### vector_documents

Documents with vector embeddings for semantic search via pgvector. Defined in `complete_schema.sql` (not in Go migrations).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| title | TEXT | NO | -- | Document title |
| content | TEXT | NO | -- | Document content |
| metadata | JSONB | YES | `'{}'` | Document metadata |
| embedding_id | UUID | YES | -- | External embedding store reference |
| embedding | VECTOR(1536) | YES | -- | Primary embedding (1536-dim) |
| created_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last modification timestamp |
| embedding_provider | VARCHAR(50) | YES | `'pgvector'` | Provider that generated embedding |
| search_vector | VECTOR(1536) | YES | -- | Secondary search embedding |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_vector_documents_embedding_provider | embedding_provider | Filter by provider |

---

### protocol_cache

TTL-based cache for protocol operation responses.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| cache_key | VARCHAR(255) | NO | -- | Primary key (cache identifier) |
| cache_data | JSONB | NO | -- | Cached response data |
| expires_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Cache expiration time |
| created_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last modification timestamp |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_protocol_cache_expires_at | expires_at | Expiration cleanup |
| idx_cache_expiration | expires_at | Partial: `expires_at < NOW()` |

---

### protocol_metrics

Operational metrics for all protocol operations (MCP, LSP, ACP, Embedding).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | SERIAL | NO | auto-increment | Primary key |
| protocol_type | VARCHAR(20) | NO | -- | `mcp`, `lsp`, `acp`, `embedding` (CHECK constraint) |
| server_id | VARCHAR(255) | YES | -- | Logical reference to *_servers.id |
| operation | VARCHAR(100) | NO | -- | Operation name |
| status | VARCHAR(20) | NO | -- | `success`, `error`, `timeout` (CHECK constraint) |
| duration_ms | INTEGER | YES | -- | Operation duration in milliseconds |
| error_message | TEXT | YES | -- | Error details if failed |
| metadata | JSONB | YES | `'{}'` | Additional metadata |
| created_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `CURRENT_TIMESTAMP` | Last modification timestamp |

**Indexes:**
| Index | Column(s) | Type | Notes |
|-------|-----------|------|-------|
| idx_protocol_metrics_protocol_type | protocol_type | btree | Filter by protocol |
| idx_protocol_metrics_created_at | created_at | btree | Time-based queries |
| idx_protocol_metrics_timeseries | protocol_type, created_at DESC | btree | INCLUDE (status, duration_ms) |
| idx_protocol_metrics_by_server | server_id, operation, status | btree | Partial: `server_id IS NOT NULL` |
| idx_protocol_metrics_brin | created_at | BRIN | Space-efficient time index |

---

## 5. Memory

### cognee_memories

Cognee RAG memory entries. Each entry belongs to a session and dataset, with optional vector and knowledge graph references.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| session_id | UUID | YES | -- | FK to `user_sessions(id)` |
| dataset_name | VARCHAR(255) | NO | -- | Dataset/collection name |
| content_type | VARCHAR(50) | YES | `'text'` | `text`, `code`, `structured` |
| content | TEXT | NO | -- | Memory content |
| vector_id | VARCHAR(255) | YES | -- | External vector store reference |
| graph_nodes | JSONB | YES | `'{}'` | Knowledge graph node references |
| search_key | VARCHAR(255) | YES | -- | Search key for retrieval |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| session_id | user_sessions(id) | CASCADE |

**Indexes:**
| Index | Column(s) | Type | Notes |
|-------|-----------|------|-------|
| idx_cognee_memories_session_id | session_id | btree | Memories by session |
| idx_cognee_memories_dataset_name | dataset_name | btree | Filter by dataset |
| idx_cognee_memories_search_key | search_key | btree | Retrieval by search key |
| idx_cognee_content_fts | to_tsvector('english', content) | GIN | Full-text search on content |
| idx_cognee_dataset_recent | dataset_name, created_at DESC | btree | Recent entries by dataset |

---

## 6. Background Tasks

### background_tasks

PostgreSQL-backed task queue with priority scheduling, retry logic, checkpoint/resume, resource requirements, and soft delete.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| task_type | VARCHAR(100) | NO | -- | Task type classifier |
| task_name | VARCHAR(255) | NO | -- | Human-readable task name |
| correlation_id | VARCHAR(255) | YES | -- | Request correlation identifier |
| parent_task_id | UUID | YES | -- | Self-referencing FK for task hierarchies |
| payload | JSONB | NO | `'{}'` | Task input data |
| config | JSONB | NO | `'{}'` | Task configuration |
| priority | task_priority | NO | `'normal'` | Priority level (ENUM) |
| status | task_status | NO | `'pending'` | Current lifecycle state (ENUM) |
| progress | DECIMAL(5,2) | YES | `0.0` | Completion percentage |
| progress_message | TEXT | YES | -- | Progress status message |
| checkpoint | JSONB | YES | -- | Checkpoint for resume |
| max_retries | INTEGER | YES | `3` | Max retry attempts |
| retry_count | INTEGER | YES | `0` | Current retry count |
| retry_delay_seconds | INTEGER | YES | `60` | Delay between retries |
| last_error | TEXT | YES | -- | Most recent error message |
| error_history | JSONB | YES | `'[]'` | Array of historical errors |
| worker_id | VARCHAR(100) | YES | -- | Assigned worker identifier |
| process_pid | INTEGER | YES | -- | OS process ID |
| started_at | TIMESTAMPTZ | YES | -- | Execution start time |
| completed_at | TIMESTAMPTZ | YES | -- | Execution completion time |
| last_heartbeat | TIMESTAMPTZ | YES | -- | Last worker heartbeat |
| deadline | TIMESTAMPTZ | YES | -- | Hard deadline for completion |
| required_cpu_cores | INTEGER | YES | `1` | CPU cores requirement |
| required_memory_mb | INTEGER | YES | `512` | Memory requirement (MB) |
| estimated_duration_seconds | INTEGER | YES | -- | Estimated duration |
| actual_duration_seconds | INTEGER | YES | -- | Actual duration |
| notification_config | JSONB | YES | `'{}'` | Notification settings |
| user_id | UUID | YES | -- | Logical user reference (no FK) |
| session_id | UUID | YES | -- | Logical session reference (no FK) |
| tags | JSONB | YES | `'[]'` | Tag array |
| metadata | JSONB | YES | `'{}'` | Additional metadata |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification (auto-trigger) |
| scheduled_at | TIMESTAMPTZ | YES | `NOW()` | Scheduled execution time |
| deleted_at | TIMESTAMPTZ | YES | -- | Soft delete timestamp |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| parent_task_id | background_tasks(id) | SET NULL |

**Triggers:** `trg_background_task_updated_at` -- auto-sets `updated_at` on UPDATE.

**Stored functions:**
- `dequeue_background_task(worker_id, max_cpu, max_memory)` -- atomic task dequeue with priority ordering, resource filtering, and `SKIP LOCKED`.
- `get_stale_tasks(heartbeat_threshold)` -- detects stuck tasks with stale heartbeats (default: 5 minutes).

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_tasks_status | status | Filter by state |
| idx_tasks_priority_status | priority, status, scheduled_at | Dequeue ordering |
| idx_tasks_worker | worker_id | Partial: `status = 'running'` |
| idx_tasks_user | user_id | Partial: `user_id IS NOT NULL` |
| idx_tasks_correlation | correlation_id | Partial: `correlation_id IS NOT NULL` |
| idx_tasks_scheduled | scheduled_at | Partial: `status = 'pending'` |
| idx_tasks_heartbeat | last_heartbeat | Partial: `status = 'running'` |
| idx_tasks_deadline | deadline | Partial: `deadline IS NOT NULL` |
| idx_tasks_type | task_type | Filter by type |
| idx_tasks_created | created_at DESC | Chronological listing |
| idx_tasks_not_deleted | id | Partial: `deleted_at IS NULL` |
| idx_tasks_queue_order | priority, scheduled_at ASC, created_at ASC | Partial: `status = 'pending' AND deleted_at IS NULL` |
| idx_tasks_running_heartbeat | last_heartbeat, started_at | Partial: `status = 'running'` |
| idx_tasks_completion | completed_at DESC, status | Partial: `status IN ('completed', 'failed')` |

---

### background_tasks_dead_letter

Dead-letter queue for tasks that exhausted all retry attempts. Task data is copied (no FK to background_tasks).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| original_task_id | UUID | NO | -- | Original task ID (no FK) |
| task_data | JSONB | NO | -- | Full task data snapshot |
| failure_reason | TEXT | NO | -- | Final failure reason |
| failure_count | INTEGER | YES | `1` | Total failure count |
| moved_at | TIMESTAMPTZ | YES | `NOW()` | Time moved to dead letter |
| reprocess_after | TIMESTAMPTZ | YES | -- | Eligible for reprocessing after |
| reprocessed | BOOLEAN | YES | `FALSE` | Whether reprocessed |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_dead_letter_reprocess | reprocess_after | Partial: `NOT reprocessed` |
| idx_dead_letter_original | original_task_id | Lookup by original task |
| idx_dead_letter_ready | reprocess_after ASC | Partial: `NOT reprocessed AND reprocess_after IS NOT NULL` |

---

### task_execution_history

Immutable audit trail of task state transitions.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| task_id | UUID | NO | -- | FK to `background_tasks(id)` |
| event_type | VARCHAR(50) | NO | -- | Event type identifier |
| event_data | JSONB | YES | `'{}'` | Event payload |
| worker_id | VARCHAR(100) | YES | -- | Worker that triggered event |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Event timestamp |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| task_id | background_tasks(id) | CASCADE |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_task_history_task_id | task_id | History by task |
| idx_task_history_event_type | event_type | Filter by event |
| idx_task_history_created | created_at DESC | Chronological |

---

### task_resource_snapshots

Time-series resource usage data for running tasks (CPU, memory, I/O, network).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| task_id | UUID | NO | -- | FK to `background_tasks(id)` |
| cpu_percent | DECIMAL(5,2) | YES | -- | CPU usage percentage |
| cpu_user_time | DECIMAL(12,4) | YES | -- | CPU user time (seconds) |
| cpu_system_time | DECIMAL(12,4) | YES | -- | CPU system time (seconds) |
| memory_rss_bytes | BIGINT | YES | -- | Resident set size |
| memory_vms_bytes | BIGINT | YES | -- | Virtual memory size |
| memory_percent | DECIMAL(5,2) | YES | -- | Memory usage percentage |
| io_read_bytes | BIGINT | YES | -- | Bytes read |
| io_write_bytes | BIGINT | YES | -- | Bytes written |
| io_read_count | BIGINT | YES | -- | Read operations count |
| io_write_count | BIGINT | YES | -- | Write operations count |
| net_bytes_sent | BIGINT | YES | -- | Network bytes sent |
| net_bytes_recv | BIGINT | YES | -- | Network bytes received |
| net_connections | INTEGER | YES | -- | Active network connections |
| open_files | INTEGER | YES | -- | Open file handles |
| open_fds | INTEGER | YES | -- | Open file descriptors |
| process_state | VARCHAR(20) | YES | -- | OS process state |
| thread_count | INTEGER | YES | -- | Active thread count |
| sampled_at | TIMESTAMPTZ | YES | `NOW()` | Sample timestamp |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| task_id | background_tasks(id) | CASCADE |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_resource_snapshots_task | task_id, sampled_at DESC | Snapshots by task |
| idx_resource_snapshots_recent | sampled_at DESC | Recent snapshots |

---

### webhook_deliveries

Webhook notification delivery tracking with retry support.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `uuid_generate_v4()` | Primary key |
| task_id | UUID | YES | -- | FK to `background_tasks(id)` |
| webhook_url | TEXT | NO | -- | Target webhook URL |
| event_type | VARCHAR(50) | NO | -- | Event type identifier |
| payload | JSONB | NO | -- | Delivery payload |
| status | VARCHAR(20) | YES | `'pending'` | `pending`, `delivered`, `failed` |
| attempts | INTEGER | YES | `0` | Delivery attempt count |
| last_attempt_at | TIMESTAMPTZ | YES | -- | Last attempt timestamp |
| last_error | TEXT | YES | -- | Last delivery error |
| response_code | INTEGER | YES | -- | HTTP response code |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| delivered_at | TIMESTAMPTZ | YES | -- | Successful delivery timestamp |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| task_id | background_tasks(id) | SET NULL |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_webhook_deliveries_task | task_id | Deliveries by task |
| idx_webhook_deliveries_status | status | Partial: `status != 'delivered'` |
| idx_webhooks_pending_retry | created_at, attempts | Partial: `status = 'pending' OR status = 'failed'` |

---

## 7. Debate System

### debate_sessions

Tracks the lifecycle of each debate session with full metadata for replay and recovery. Supports pause/resume via status transitions for approval gates.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| debate_id | VARCHAR(255) | NO | -- | Links to `debate_logs.debate_id` |
| topic | TEXT | NO | -- | Debate topic or task description |
| status | VARCHAR(50) | NO | `'pending'` | `pending`, `running`, `paused`, `completed`, `failed`, `cancelled` (CHECK) |
| topology_type | VARCHAR(50) | YES | -- | `graph_mesh`, `star`, `chain`, `tree` |
| coordination_protocol | VARCHAR(50) | YES | -- | `cpde`, `dpde`, `adaptive` |
| config | JSONB | YES | `'{}'` | Max rounds, timeout, consensus threshold, gates config |
| initiated_by | VARCHAR(255) | YES | -- | Requester/initiator identifier |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification (auto-trigger) |
| completed_at | TIMESTAMPTZ | YES | -- | Completion/failure/cancel timestamp |
| total_rounds | INTEGER | YES | `0` | Total rounds completed |
| final_consensus_score | DECIMAL(5,4) | YES | -- | Final consensus (0.0000-1.0000) |
| outcome | JSONB | YES | `'{}'` | Winner, voting method, confidence, summary |
| metadata | JSONB | YES | `'{}'` | Audit trail, provenance data |

**Triggers:** `trg_debate_sessions_updated_at` -- auto-sets `updated_at` on UPDATE.

**Indexes:**
| Index | Column(s) | Type | Notes |
|-------|-----------|------|-------|
| idx_debate_sessions_debate_id | debate_id | btree | Debate lookup |
| idx_debate_sessions_status | status | btree | Status filtering |
| idx_debate_sessions_created_at | created_at | btree | Chronological |
| idx_debate_sessions_topology | topology_type | btree | Topology analytics |
| idx_debate_sessions_active | status | btree | Partial: `status IN ('pending', 'running', 'paused')` |
| idx_debate_sessions_metadata | metadata | GIN | JSONB queries |
| idx_debate_sessions_config | config | GIN | JSONB queries |
| idx_debate_sessions_debate_status | debate_id, status | btree | Composite lookup |

---

### debate_turns

Granular turn-level state for debate replay and recovery. Each turn captures one agent's action in one phase of one round, including Reflexion episodic memory.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| session_id | UUID | NO | -- | FK to `debate_sessions(id)` |
| round | INTEGER | NO | -- | Round number (1-based) |
| phase | VARCHAR(50) | NO | -- | Protocol phase (CHECK: 8 valid phases) |
| agent_id | VARCHAR(255) | NO | -- | Agent identifier |
| agent_role | VARCHAR(100) | YES | -- | Agent role in this turn |
| provider | VARCHAR(100) | YES | -- | LLM provider used |
| model | VARCHAR(255) | YES | -- | Specific model used |
| content | TEXT | YES | -- | Response content |
| confidence | DECIMAL(5,4) | YES | -- | Agent confidence (0.0000-1.0000) |
| tool_calls | JSONB | YES | `'[]'` | Tool invocations and results |
| test_results | JSONB | YES | `'{}'` | Test execution results |
| reflections | JSONB | YES | `'[]'` | Reflexion episodic memory entries |
| metadata | JSONB | YES | `'{}'` | Additional structured data |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| response_time_ms | INTEGER | YES | -- | Response latency in milliseconds |

**CHECK constraint on `phase`:** `dehallucination`, `self_evolvement`, `proposal`, `critique`, `review`, `optimization`, `adversarial`, `convergence`

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| session_id | debate_sessions(id) | CASCADE |

**Indexes:**
| Index | Column(s) | Type | Notes |
|-------|-----------|------|-------|
| idx_debate_turns_session_id | session_id | btree | Turns by session |
| idx_debate_turns_session_round | session_id, round | btree | Round-level queries |
| idx_debate_turns_phase | phase | btree | Phase filtering |
| idx_debate_turns_agent | agent_id | btree | Agent tracking |
| idx_debate_turns_session_round_phase | session_id, round, phase | btree | Most specific query |
| idx_debate_turns_created_at | created_at | btree | Chronological |
| idx_debate_turns_reflections | reflections | GIN | Partial: `reflections != '[]'` |
| idx_debate_turns_tool_calls | tool_calls | GIN | Partial: `tool_calls != '[]'` |
| idx_debate_turns_metadata | metadata | GIN | JSONB queries |

---

### code_versions

Code snapshots at debate milestones for version tracking and quality trend analysis.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| session_id | UUID | NO | -- | FK to `debate_sessions(id)` |
| turn_id | UUID | YES | -- | FK to `debate_turns(id)` |
| language | VARCHAR(50) | YES | -- | Programming language |
| code | TEXT | NO | -- | Full code snapshot |
| version_number | INTEGER | NO | -- | Sequential version within session |
| quality_score | DECIMAL(5,4) | YES | -- | Overall quality (0.0000-1.0000) |
| test_pass_rate | DECIMAL(5,4) | YES | -- | Test pass rate (0.0000-1.0000) |
| metrics | JSONB | YES | `'{}'` | Maintainability, complexity, security scores |
| diff_from_previous | TEXT | YES | -- | Unified diff from prior version |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| session_id | debate_sessions(id) | CASCADE |
| turn_id | debate_turns(id) | SET NULL |

**Unique constraints:** `(session_id, version_number)`

**Indexes:**
| Index | Column(s) | Type | Notes |
|-------|-----------|------|-------|
| idx_code_versions_session_id | session_id | btree | Versions by session |
| idx_code_versions_turn_id | turn_id | btree | Versions by turn |
| idx_code_versions_session_version | session_id, version_number | btree | Ordered versions |
| idx_code_versions_language | language | btree | Language analytics |
| idx_code_versions_quality | quality_score | btree | Partial: `quality_score IS NOT NULL` |
| idx_code_versions_test_pass_rate | test_pass_rate | btree | Partial: `test_pass_rate IS NOT NULL` |
| idx_code_versions_metrics | metrics | GIN | JSONB queries |

---

### debate_logs

Append-only log of every participant action in every debate round. Uses string-based identifiers (no foreign keys). Supports time-based retention via `expires_at`.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | SERIAL | NO | auto-increment | Primary key |
| debate_id | VARCHAR(255) | NO | -- | Debate identifier |
| session_id | VARCHAR(255) | NO | -- | Session identifier (string, not FK) |
| participant_id | INTEGER | YES | -- | Numeric participant identifier |
| participant_identifier | VARCHAR(255) | YES | -- | String participant identifier |
| participant_name | VARCHAR(255) | YES | -- | Participant display name |
| role | VARCHAR(100) | YES | -- | `proponent`, `opponent`, `moderator`, `synthesizer` |
| provider | VARCHAR(100) | YES | -- | LLM provider name |
| model | VARCHAR(255) | YES | -- | Specific model used |
| round | INTEGER | YES | -- | Round number (1-based) |
| action | VARCHAR(100) | YES | -- | `response`, `rebuttal`, `summary`, `vote`, `synthesis` |
| response_time_ms | BIGINT | YES | -- | Response latency in milliseconds |
| quality_score | DECIMAL(5,4) | YES | -- | Quality score (0.0000-1.0000) |
| tokens_used | INTEGER | YES | -- | Tokens consumed |
| content_length | INTEGER | YES | -- | Response content length |
| error_message | TEXT | YES | -- | Error details if failed |
| metadata | JSONB | YES | `'{}'` | Additional metadata |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| expires_at | TIMESTAMPTZ | YES | -- | NULL = no expiration |

**Stored functions:** `cleanup_expired_debate_logs()` -- deletes rows where `expires_at < NOW()`.

**Indexes:**
| Index | Column(s) | Type | Notes |
|-------|-----------|------|-------|
| idx_debate_logs_debate_id | debate_id | btree | Logs by debate |
| idx_debate_logs_session_id | session_id | btree | Logs by session |
| idx_debate_logs_provider | provider | btree | Filter by provider |
| idx_debate_logs_model | model | btree | Filter by model |
| idx_debate_logs_created_at | created_at | btree | Chronological |
| idx_debate_logs_expires_at | expires_at | btree | Partial: `expires_at IS NOT NULL` |
| idx_debate_logs_debate_round | debate_id, round | btree | Round-level queries |
| idx_debate_logs_active | debate_id | btree | Partial: `expires_at IS NULL OR expires_at > NOW()` |
| idx_debate_logs_provider_model | provider, model | btree | Provider+model combo |
| idx_debate_logs_metadata | metadata | GIN | JSONB queries |

---

## 8. Analytics (ClickHouse)

The following tables are defined in `sql/schema/clickhouse_analytics.sql` and run on ClickHouse (not PostgreSQL). All use MergeTree family engines with monthly partitioning.

### debate_metrics

Time-series metrics for individual debate round performance.

| Column | Type | Description |
|--------|------|-------------|
| debate_id | String | Debate identifier |
| round | UInt8 | Round number |
| timestamp | DateTime | Event time |
| provider | String | LLM provider |
| model | String | Model used |
| position | String | Debate position |
| response_time_ms | Float32 | Response latency (ms) |
| tokens_used | UInt32 | Tokens consumed |
| confidence_score | Float32 | Confidence score |
| error_count | UInt8 | Errors in round |
| was_winner | UInt8 | Boolean (1=winner) |

**Engine:** MergeTree, partitioned by `toYYYYMM(timestamp)`, ordered by `(timestamp, debate_id, round)`.

**Materialized view:** `debate_metrics_hourly` (SummingMergeTree) -- hourly aggregation by provider.

---

### conversation_metrics

Conversation-level statistics including messages, entities, and tokens.

| Column | Type | Description |
|--------|------|-------------|
| conversation_id | String | Conversation identifier |
| user_id | String | User identifier |
| timestamp | DateTime | Event time |
| message_count | UInt32 | Total messages |
| entity_count | UInt32 | Entities extracted |
| total_tokens | UInt64 | Total tokens used |
| duration_ms | UInt64 | Duration (ms) |
| debate_rounds | UInt8 | Number of debate rounds |
| llms_used | Array(String) | LLMs involved |

**Engine:** MergeTree, partitioned by `toYYYYMM(timestamp)`, ordered by `(timestamp, user_id, conversation_id)`.

**Materialized view:** `conversation_metrics_daily` (SummingMergeTree) -- daily aggregation by user.

---

### provider_performance

Aggregated provider performance metrics.

| Column | Type | Description |
|--------|------|-------------|
| timestamp | DateTime | Measurement time |
| provider | String | Provider name |
| model | String | Model name |
| total_requests | UInt64 | Total requests |
| successful_requests | UInt64 | Successful requests |
| failed_requests | UInt64 | Failed requests |
| avg_response_time | Float32 | Average latency (ms) |
| p50_response_time | Float32 | Median latency |
| p95_response_time | Float32 | 95th percentile latency |
| p99_response_time | Float32 | 99th percentile latency |
| total_tokens | UInt64 | Total tokens |
| avg_tokens_per_req | Float32 | Average tokens per request |
| total_cost | Float64 | Total cost |
| avg_cost_per_req | Float64 | Average cost per request |

**Engine:** MergeTree, partitioned by `toYYYYMM(timestamp)`, ordered by `(timestamp, provider, model)`.

---

### llm_response_latency

LLM API response time tracking for performance analysis.

| Column | Type | Description |
|--------|------|-------------|
| timestamp | DateTime | Event time |
| provider | String | Provider name |
| model | String | Model name |
| operation | String | `complete`, `stream`, `embed` |
| latency_ms | Float32 | Latency (ms) |
| tokens_input | UInt32 | Input tokens |
| tokens_output | UInt32 | Output tokens |
| cache_hit | UInt8 | Boolean (1=cache hit) |

**Engine:** MergeTree, partitioned by `toYYYYMM(timestamp)`, ordered by `(timestamp, provider, operation)`.

---

### entity_extraction_metrics

Entity extraction performance and confidence tracking.

| Column | Type | Description |
|--------|------|-------------|
| timestamp | DateTime | Event time |
| conversation_id | String | Conversation identifier |
| entity_id | String | Entity identifier |
| entity_type | String | Type of entity |
| extraction_method | String | `llm`, `rule-based`, `hybrid` |
| confidence | Float32 | Extraction confidence |
| processing_time_ms | Float32 | Processing time (ms) |

**Engine:** MergeTree, partitioned by `toYYYYMM(timestamp)`, ordered by `(timestamp, conversation_id, entity_id)`.

---

### memory_operations

Memory system operation logs and performance.

| Column | Type | Description |
|--------|------|-------------|
| timestamp | DateTime | Event time |
| user_id | String | User identifier |
| operation | String | `add`, `update`, `search`, `delete` |
| memory_id | String | Memory entry identifier |
| duration_ms | Float32 | Operation duration (ms) |
| success | UInt8 | Boolean (1=success) |
| error_message | String | Error details |

**Engine:** MergeTree, partitioned by `toYYYYMM(timestamp)`, ordered by `(timestamp, user_id, operation)`.

---

### debate_winners

Debate winner tracking for analysis.

| Column | Type | Description |
|--------|------|-------------|
| debate_id | String | Debate identifier |
| timestamp | DateTime | Debate completion time |
| winner_provider | String | Winning provider |
| winner_model | String | Winning model |
| winner_position | String | Winning debate position |
| total_rounds | UInt8 | Total rounds |
| final_confidence | Float32 | Winner confidence |
| debate_duration_ms | UInt64 | Total debate duration (ms) |

**Engine:** MergeTree, partitioned by `toYYYYMM(timestamp)`, ordered by `(timestamp, debate_id)`.

---

### system_health

System component health monitoring.

| Column | Type | Description |
|--------|------|-------------|
| timestamp | DateTime | Measurement time |
| component | String | `api`, `kafka`, `redis`, `neo4j`, `clickhouse`, etc. |
| metric_name | String | Metric identifier |
| metric_value | Float64 | Metric value |
| unit | String | Unit of measurement |
| status | String | `healthy`, `degraded`, `unhealthy` |

**Engine:** MergeTree, partitioned by `toYYYYMM(timestamp)`, ordered by `(timestamp, component, metric_name)`.

---

### api_requests

REST API request logs and performance.

| Column | Type | Description |
|--------|------|-------------|
| timestamp | DateTime | Request time |
| endpoint | String | API endpoint path |
| method | String | HTTP method (GET, POST, PUT, DELETE) |
| status_code | UInt16 | HTTP response status code |
| response_time_ms | Float32 | Response time (ms) |
| user_id | String | User identifier |
| session_id | String | Session identifier |
| error_message | String | Error details |

**Engine:** MergeTree, partitioned by `toYYYYMM(timestamp)`, ordered by `(timestamp, endpoint, method)`.

**Materialized view:** `api_requests_minutely` (SummingMergeTree) -- per-minute aggregation by endpoint and method.

---

## Materialized Views (PostgreSQL)

Seven materialized views are defined in migration 013 for pre-computed analytics:

| View | Source Tables | Window | Refresh Interval |
|------|--------------|--------|-------------------|
| mv_provider_performance | llm_providers, llm_responses, llm_requests | 24h | 5 min |
| mv_mcp_server_health | mcp_servers, protocol_metrics | 1h | 1 min |
| mv_request_analytics_hourly | llm_requests | 7d | 15 min |
| mv_session_stats_daily | user_sessions | 30d | 1h |
| mv_task_statistics | background_tasks | 24h | 5 min |
| mv_model_capabilities | models_metadata | all | 1h |
| mv_protocol_metrics_agg | protocol_metrics | 24h | 5 min |

Refresh functions: `refresh_all_materialized_views()` (all views with status reporting), `refresh_critical_views()` (provider_performance, mcp_server_health, task_statistics only).

---

## Relationships

All foreign key relationships across the PostgreSQL schema:

| Source Table | Column | Target Table | Target Column | On Delete |
|-------------|--------|-------------|---------------|-----------|
| user_sessions | user_id | users | id | CASCADE |
| llm_requests | session_id | user_sessions | id | CASCADE |
| llm_requests | user_id | users | id | CASCADE |
| llm_responses | request_id | llm_requests | id | CASCADE |
| llm_responses | provider_id | llm_providers | id | SET NULL |
| cognee_memories | session_id | user_sessions | id | CASCADE |
| models_metadata | provider_id | llm_providers | id | CASCADE |
| model_benchmarks | model_id | models_metadata | model_id | CASCADE |
| background_tasks | parent_task_id | background_tasks | id | SET NULL |
| task_execution_history | task_id | background_tasks | id | CASCADE |
| task_resource_snapshots | task_id | background_tasks | id | CASCADE |
| webhook_deliveries | task_id | background_tasks | id | SET NULL |
| debate_turns | session_id | debate_sessions | id | CASCADE |
| code_versions | session_id | debate_sessions | id | CASCADE |
| code_versions | turn_id | debate_turns | id | SET NULL |

**Logical references (no FK constraint):**
- `user_sessions.memory_id` -- external Cognee memory reference
- `debate_sessions.debate_id` -- string-based link to `debate_logs.debate_id`
- `debate_logs.session_id` -- string-based session reference
- `protocol_metrics.server_id` -- logical reference to `mcp_servers.id`, `lsp_servers.id`, or `acp_servers.id`
- `background_tasks.user_id` / `background_tasks.session_id` -- logical user/session references

---

## 9. Agentic Workflows

### agentic_workflows

Graph-based workflow definitions with execution state and results. Defined in `sql/schema/agentic_workflows.sql`.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(64) | NO | -- | Primary key (workflow identifier) |
| name | VARCHAR(255) | NO | -- | Workflow name |
| status | VARCHAR(32) | NO | `'pending'` | Lifecycle state |
| entry_point | VARCHAR(64) | NO | -- | Entry node identifier |
| config | JSONB | YES | -- | Workflow configuration |
| input | JSONB | YES | -- | Workflow input data |
| result | JSONB | YES | -- | Workflow output/result |
| error | TEXT | YES | -- | Error details if failed |
| nodes_executed | INTEGER | YES | `0` | Count of executed nodes |
| execution_time_ms | BIGINT | YES | `0` | Total execution time (ms) |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification timestamp |
| completed_at | TIMESTAMPTZ | YES | -- | Completion timestamp |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_workflows_status | status | Filter by state |
| idx_workflows_created | created_at DESC | Chronological listing |

---

### agentic_workflow_nodes

Individual nodes within a workflow graph.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(64) | NO | -- | Primary key (node identifier) |
| workflow_id | VARCHAR(64) | NO | -- | FK to `agentic_workflows(id)` |
| name | VARCHAR(255) | NO | -- | Node name |
| type | VARCHAR(32) | NO | -- | Node type |
| config | JSONB | YES | -- | Node configuration |
| status | VARCHAR(32) | NO | `'pending'` | Node execution status |
| result | JSONB | YES | -- | Node output/result |
| error | TEXT | YES | -- | Error details if failed |
| started_at | TIMESTAMPTZ | YES | -- | Execution start time |
| completed_at | TIMESTAMPTZ | YES | -- | Execution completion time |
| execution_time_ms | BIGINT | YES | `0` | Node execution time (ms) |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| workflow_id | agentic_workflows(id) | CASCADE |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_workflow_nodes_wf | workflow_id | Nodes by workflow |

---

### agentic_workflow_edges

Edges (connections) between workflow nodes with optional conditions.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | SERIAL | NO | auto-increment | Primary key |
| workflow_id | VARCHAR(64) | NO | -- | FK to `agentic_workflows(id)` |
| from_node | VARCHAR(64) | NO | -- | Source node identifier |
| to_node | VARCHAR(64) | NO | -- | Target node identifier |
| condition | JSONB | YES | -- | Edge transition condition |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| workflow_id | agentic_workflows(id) | CASCADE |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_workflow_edges_wf | workflow_id | Edges by workflow |

---

## 10. LLMOps

### llmops_experiments

A/B experiments for LLM provider comparison. Defined in `sql/schema/llmops_experiments.sql`.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(64) | NO | -- | Primary key |
| name | VARCHAR(255) | NO | -- | Experiment name |
| description | TEXT | YES | -- | Experiment description |
| status | VARCHAR(32) | NO | `'created'` | `created`, `running`, `completed`, `failed` |
| variants | JSONB | NO | -- | Experiment variant definitions |
| metrics | JSONB | YES | -- | Collected metrics |
| config | JSONB | YES | -- | Experiment configuration |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification timestamp |
| started_at | TIMESTAMPTZ | YES | -- | Experiment start time |
| completed_at | TIMESTAMPTZ | YES | -- | Experiment completion time |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_experiments_status | status | Filter by state |
| idx_experiments_name | name | Lookup by name |

---

### llmops_evaluations

LLM evaluation runs against datasets.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(64) | NO | -- | Primary key |
| name | VARCHAR(255) | NO | -- | Evaluation name |
| dataset | VARCHAR(255) | NO | -- | Dataset identifier |
| status | VARCHAR(32) | NO | `'pending'` | Evaluation status |
| metrics | JSONB | YES | -- | Evaluation metrics |
| results | JSONB | YES | -- | Evaluation results |
| config | JSONB | YES | -- | Evaluation configuration |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| completed_at | TIMESTAMPTZ | YES | -- | Evaluation completion time |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_evaluations_dataset | dataset | Filter by dataset |

---

### llmops_prompt_versions

Versioned prompt templates.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | SERIAL | NO | auto-increment | Primary key |
| name | VARCHAR(255) | NO | -- | Prompt name |
| version | VARCHAR(32) | NO | -- | Version identifier |
| content | TEXT | NO | -- | Prompt template content |
| metadata | JSONB | YES | -- | Additional metadata |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |

**Unique constraints:** `(name, version)`

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_prompts_name | name | Lookup by name |

---

## 11. Planning

### planning_sessions

HiPlan, MCTS, and Tree of Thoughts planning results. Defined in `sql/schema/planning_sessions.sql`.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(64) | NO | -- | Primary key |
| algorithm | VARCHAR(32) | NO | -- | `hiplan`, `mcts`, `tot` |
| status | VARCHAR(32) | NO | `'pending'` | Execution status |
| input | JSONB | NO | -- | Planning input data |
| config | JSONB | YES | -- | Algorithm configuration |
| result | JSONB | YES | -- | Planning output/result |
| error | TEXT | YES | -- | Error details if failed |
| execution_time_ms | BIGINT | YES | `0` | Execution time (ms) |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| completed_at | TIMESTAMPTZ | YES | -- | Completion timestamp |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_planning_algorithm | algorithm | Filter by algorithm |
| idx_planning_status | status | Filter by state |
| idx_planning_created | created_at DESC | Chronological listing |

---

### planning_hiplan_milestones

Milestones within a HiPlan hierarchical planning session.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | SERIAL | NO | auto-increment | Primary key |
| session_id | VARCHAR(64) | NO | -- | FK to `planning_sessions(id)` |
| name | VARCHAR(255) | NO | -- | Milestone name |
| description | TEXT | YES | -- | Milestone description |
| order_index | INTEGER | NO | -- | Execution order |
| status | VARCHAR(32) | NO | `'pending'` | Milestone status |
| steps | JSONB | YES | -- | Steps within this milestone |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| session_id | planning_sessions(id) | CASCADE |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_milestones_session | session_id | Milestones by session |

---

### planning_mcts_nodes

Monte Carlo Tree Search nodes for MCTS planning sessions.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | SERIAL | NO | auto-increment | Primary key |
| session_id | VARCHAR(64) | NO | -- | FK to `planning_sessions(id)` |
| parent_id | INTEGER | YES | -- | Self-referencing FK (tree structure) |
| action | VARCHAR(255) | YES | -- | Action taken to reach this node |
| state | JSONB | YES | -- | Node state |
| visits | INTEGER | YES | `0` | Visit count (UCT exploration) |
| reward | DOUBLE PRECISION | YES | `0` | Accumulated reward |
| depth | INTEGER | YES | `0` | Tree depth |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| session_id | planning_sessions(id) | CASCADE |
| parent_id | planning_mcts_nodes(id) | -- |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_mcts_nodes_session | session_id | Nodes by session |

---

## 12. Performance & Security

### feature_flags

Feature flag management for runtime feature toggling. Defined in `sql/002_performance_and_security.sql`.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | SERIAL | NO | auto-increment | Primary key |
| name | VARCHAR(255) | NO | -- | Unique flag name |
| enabled | BOOLEAN | NO | `false` | Whether feature is enabled |
| description | TEXT | YES | -- | Flag description |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification timestamp |

**Unique constraints:** `name`

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_feature_flags_name | name | Lookup by name |

---

### performance_baselines

Performance benchmark baselines for regression detection.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | SERIAL | NO | auto-increment | Primary key |
| metric_name | VARCHAR(255) | NO | -- | Benchmark metric name |
| package_name | VARCHAR(255) | NO | -- | Go package path |
| baseline_ns | BIGINT | NO | -- | Baseline time in nanoseconds |
| baseline_allocs | BIGINT | YES | -- | Baseline allocation count |
| baseline_bytes | BIGINT | YES | -- | Baseline bytes allocated |
| captured_at | TIMESTAMPTZ | YES | `NOW()` | When baseline was captured |

**Unique constraints:** `(metric_name, package_name)`

---

### security_scan_history

Security scan result history for Snyk/SonarQube.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | SERIAL | NO | auto-increment | Primary key |
| tool_name | VARCHAR(100) | NO | -- | Scanner tool (`snyk`, `sonarqube`, `gosec`) |
| scan_type | VARCHAR(50) | NO | -- | Scan type identifier |
| findings_critical | INTEGER | YES | `0` | Critical severity findings |
| findings_high | INTEGER | YES | `0` | High severity findings |
| findings_medium | INTEGER | YES | `0` | Medium severity findings |
| findings_low | INTEGER | YES | `0` | Low severity findings |
| findings_info | INTEGER | YES | `0` | Informational findings |
| scan_duration_ms | BIGINT | YES | -- | Scan duration in milliseconds |
| report_path | TEXT | YES | -- | Path to full report |
| scanned_at | TIMESTAMPTZ | YES | `NOW()` | Scan execution timestamp |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_security_scans_tool | tool_name | Filter by tool |

---

### benchmark_runs

Benchmark run results for provider comparison (SWE-bench, HumanEval, MMLU).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(100) | NO | -- | Primary key |
| benchmark_type | VARCHAR(50) | NO | -- | `swe_bench`, `human_eval`, `mmlu`, `custom` |
| provider_name | VARCHAR(100) | NO | -- | LLM provider name |
| model_name | VARCHAR(100) | YES | -- | Specific model used |
| status | VARCHAR(20) | NO | `'pending'` | `pending`, `running`, `completed`, `failed` |
| pass_rate | REAL | YES | -- | Overall pass rate |
| average_score | REAL | YES | -- | Average score across tasks |
| average_latency_ns | BIGINT | YES | -- | Average latency in nanoseconds |
| total_tasks | INTEGER | YES | -- | Total tasks in benchmark |
| passed_tasks | INTEGER | YES | -- | Number of tasks passed |
| failed_tasks | INTEGER | YES | -- | Number of tasks failed |
| config | JSONB | YES | -- | Run configuration |
| summary | JSONB | YES | -- | Result summary |
| started_at | TIMESTAMPTZ | YES | -- | Run start time |
| ended_at | TIMESTAMPTZ | YES | -- | Run end time |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_benchmark_runs_type | benchmark_type | Filter by type |
| idx_benchmark_runs_provider | provider_name | Filter by provider |

---

## 13. CLI Agent Fusion

Tables defined in `sql/001_cli_agents_fusion.sql` for managing CLI agent instances, tasks, repository data, and tool use.

### cli_agent_instances

CLI agent instance registry. Each instance represents a running CLI agent (aider, claude_code, codex, cline, etc.).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| type | VARCHAR(50) | NO | -- | Agent type (aider, claude_code, codex, cline, openhands, kiro, continue) |
| status | VARCHAR(20) | NO | `'idle'` | Instance status |
| config | JSONB | NO | `'{}'` | Instance configuration |
| session_id | UUID | YES | -- | FK to `sessions(id)` |
| user_id | UUID | YES | -- | FK to `users(id)` |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification (auto-trigger) |
| last_heartbeat | TIMESTAMPTZ | YES | -- | Last heartbeat timestamp |
| metadata | JSONB | YES | `'{}'` | Additional metadata |

**Indexes:**
| Index | Column(s) | Notes |
|-------|-----------|-------|
| idx_cli_agent_instances_session | session_id | Instances by session |
| idx_cli_agent_instances_user | user_id | Instances by user |
| idx_cli_agent_instances_type | type | Filter by agent type |
| idx_cli_agent_instances_status | status | Filter by status |

---

### cli_agent_tasks

Tasks assigned to CLI agent instances.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| instance_id | UUID | YES | -- | FK to `cli_agent_instances(id)` |
| parent_task_id | UUID | YES | -- | Self-referencing FK for task hierarchies |
| type | VARCHAR(50) | NO | -- | Task type (repo_map, diff_apply, git_commit, tool_use, browser_action) |
| status | VARCHAR(20) | NO | `'pending'` | Task status |
| priority | INTEGER | YES | `5` | Priority (1-10, lower is higher) |
| input | JSONB | NO | `'{}'` | Task input data |
| output | JSONB | YES | -- | Task output data |
| error | TEXT | YES | -- | Error details |
| started_at | TIMESTAMPTZ | YES | -- | Execution start time |
| completed_at | TIMESTAMPTZ | YES | -- | Execution completion time |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification (auto-trigger) |
| duration_ms | INTEGER | YES | -- | Execution duration (ms) |
| retry_count | INTEGER | YES | `0` | Current retry count |
| max_retries | INTEGER | YES | `3` | Maximum retry attempts |

**Foreign keys:**
| Column | References | On Delete |
|--------|-----------|-----------|
| instance_id | cli_agent_instances(id) | CASCADE |
| parent_task_id | cli_agent_tasks(id) | SET NULL |

---

### repo_maps

Repository structure maps with symbol data for CLI agents.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| instance_id | UUID | YES | -- | FK to `cli_agent_instances(id)` |
| repository_path | TEXT | NO | -- | Repository filesystem path |
| git_commit_hash | VARCHAR(40) | YES | -- | Git commit at map creation |
| symbols | JSONB | NO | `'[]'` | Array of symbols with metadata |
| file_structure | JSONB | NO | `'{}'` | Repository file structure |
| map_tokens | INTEGER | YES | `1024` | Token budget for map |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| updated_at | TIMESTAMPTZ | YES | `NOW()` | Last modification timestamp |
| last_accessed_at | TIMESTAMPTZ | YES | -- | Last access timestamp |
| access_count | INTEGER | YES | `0` | Access count |

---

### repo_symbols

Individual repository symbols for efficient semantic search via pgvector.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| repo_map_id | UUID | YES | -- | FK to `repo_maps(id)` |
| name | VARCHAR(255) | NO | -- | Symbol name |
| type | VARCHAR(50) | NO | -- | Symbol type (function, class, variable, method) |
| file_path | TEXT | NO | -- | File path containing the symbol |
| line_start | INTEGER | YES | -- | Start line number |
| line_end | INTEGER | YES | -- | End line number |
| signature | TEXT | YES | -- | Symbol signature |
| documentation | TEXT | YES | -- | Symbol documentation |
| embedding | VECTOR(1536) | YES | -- | Semantic embedding for search |
| relevance_score | FLOAT | YES | -- | Relevance score |
| references_count | INTEGER | YES | `0` | Reference count |
| metadata | JSONB | YES | `'{}'` | Additional metadata |

---

### git_operations

Git operation log for CLI agent actions.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| instance_id | UUID | YES | -- | FK to `cli_agent_instances(id)` |
| operation | VARCHAR(50) | NO | -- | Operation type (commit, diff, merge, branch) |
| repository_path | TEXT | NO | -- | Repository filesystem path |
| commit_hash | VARCHAR(40) | YES | -- | Commit hash |
| parent_hash | VARCHAR(40) | YES | -- | Parent commit hash |
| message | TEXT | YES | -- | Commit message |
| author_name | VARCHAR(255) | YES | -- | Author name |
| author_email | VARCHAR(255) | YES | -- | Author email |
| files_changed | JSONB | YES | `'[]'` | Changed files array |
| diff_content | TEXT | YES | -- | Diff content |
| attribution | VARCHAR(255) | YES | -- | Attribution (HelixAgent or user) |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |

---

### tool_use_log

Tool invocation log for CLI agent actions with optional approval workflow.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| instance_id | UUID | YES | -- | FK to `cli_agent_instances(id)` |
| task_id | UUID | YES | -- | FK to `cli_agent_tasks(id)` |
| tool_name | VARCHAR(100) | NO | -- | Tool name |
| arguments | JSONB | NO | -- | Tool arguments |
| result | JSONB | YES | -- | Tool result |
| error | TEXT | YES | -- | Error details |
| status | VARCHAR(20) | NO | `'running'` | Execution status |
| started_at | TIMESTAMPTZ | YES | `NOW()` | Start time |
| completed_at | TIMESTAMPTZ | YES | -- | Completion time |
| duration_ms | INTEGER | YES | -- | Duration (ms) |
| required_approval | BOOLEAN | YES | `false` | Whether approval was required |
| approved_by | UUID | YES | -- | FK to `users(id)` |
| approved_at | TIMESTAMPTZ | YES | -- | Approval timestamp |

---

### project_memory

Per-project memory entries with semantic embeddings for retrieval.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | `gen_random_uuid()` | Primary key |
| project_id | UUID | NO | -- | Project identifier |
| instance_id | UUID | YES | -- | FK to `cli_agent_instances(id)` |
| entry_type | VARCHAR(50) | NO | -- | Entry type (code, conversation, decision, error, insight) |
| content | TEXT | NO | -- | Memory content |
| embedding | VECTOR(1536) | YES | -- | Semantic embedding |
| metadata | JSONB | YES | `'{}'` | Additional metadata |
| importance_score | FLOAT | YES | `0.5` | Importance score for retrieval |
| created_at | TIMESTAMPTZ | YES | `NOW()` | Row creation timestamp |
| accessed_at | TIMESTAMPTZ | YES | -- | Last access timestamp |
| access_count | INTEGER | YES | `0` | Access count |
| expires_at | TIMESTAMPTZ | YES | -- | Optional expiration |

---

Additional CLI agent fusion tables: `diff_applications`, `terminal_sessions`, `browser_sessions`, `sandbox_environments`, `task_plans`. See `sql/001_cli_agents_fusion.sql` for full definitions.
