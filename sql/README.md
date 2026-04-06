# HelixAgent SQL Schema Index

This directory contains all PostgreSQL schema definitions for HelixAgent.

## Directory Structure

```
sql/
├── 001_cli_agents_fusion.sql        # CLI agent instances, tasks, repo maps, git ops, tools, memory
├── 002_performance_and_security.sql  # Feature flags, performance baselines, security scans, benchmarks
├── schema/                           # Individual schema files (apply in numbered order)
├── README.md                         # This file
└── SCHEMA_GUIDE.md                   # ER diagram, migration order, naming conventions
```

## Migration Files (top-level)

| File | Description |
|------|-------------|
| `001_cli_agents_fusion.sql` | CLI agent instances, tasks, repo maps, symbols, git operations, diffs, terminal sessions, tool use log, project memory, browser sessions, sandbox environments, task plans |
| `002_performance_and_security.sql` | Feature flags, performance baselines, security scan history, benchmark runs |

## Schema Files (schema/)

| File | Description |
|------|-------------|
| `agentic_workflows.sql` | Graph-based agentic workflow definitions, node state, and execution results |
| `background_tasks.sql` | Durable async task queue with priority scheduling, retry logic, and dead-letter queue |
| `clickhouse_analytics.sql` | ClickHouse time-series tables for real-time debate metrics and provider performance |
| `code_versions.sql` | Code snapshots captured at debate milestones for version tracking and rollback |
| `cognee_memories.sql` | Cognee RAG memory entries injected into LLM prompts for contextual knowledge |
| `complete_schema.sql` | Consolidated single-file reference merging all migrations (001–014) |
| `conversation_context.sql` | Infinite conversation context engine with Kafka-backed event sourcing and LLM compression |
| `cross_session_learning.sql` | Learned patterns, insights, and accumulated knowledge across debate sessions |
| `debate_sessions.sql` | Debate lifecycle tracking (status, topology, rounds, consensus scores, approval gates) |
| `debate_system.sql` | Append-only debate round log for multi-agent actions, participant tracking, and analytics |
| `debate_turns.sql` | Granular turn-level records with confidence scores, tool calls, and Reflexion episodic memory |
| `distributed_memory.sql` | CRDT-based multi-node memory synchronization via event sourcing |
| `indexes_views.sql` | Concurrent performance indexes and materialized views for analytics dashboards |
| `llmops_experiments.sql` | LLMOps A/B experiments, continuous evaluations, and prompt version management |
| `llm_providers.sql` | LLM provider registry, model catalog, and benchmark tracking with Models.dev integration |
| `planning_sessions.sql` | AI planning algorithm results (HiPlan milestones, MCTS nodes, Tree of Thoughts) |
| `protocol_support.sql` | MCP, LSP, ACP server configurations plus vector embeddings, cache, and metrics |
| `relationships.sql` | Documentation-only file listing all foreign key and logical entity relationships |
| `requests_responses.sql` | LLM request lifecycle tracking and full provider response storage |
| `streaming_analytics.sql` | Kafka Streams real-time conversation state snapshots and stream entity extraction |
| `users_sessions.sql` | User accounts, API keys, roles, and session management |

## Usage

For a fresh database, apply `complete_schema.sql` which includes everything.
For incremental migrations, apply individual files in the order described in
[SCHEMA_GUIDE.md](SCHEMA_GUIDE.md).

## See Also

- [SCHEMA_GUIDE.md](SCHEMA_GUIDE.md) — ER diagram, migration order, naming conventions
- `sql/` directory is referenced in `internal/services/boot_manager.go` for automated schema setup
