# Diagram Catalog

**Date:** 2026-03-30
**Status:** Active

## Overview

HelixAgent maintains architectural and flow diagrams in two formats: source files (PlantUML `.puml` and Mermaid `.mmd`) in `docs/diagrams/src/`, and rendered SVG files in `docs/diagrams/rendered/`. This document catalogs every diagram with a one-line description and cross-references to relevant documentation.

---

## Source Files (`docs/diagrams/src/`)

### PlantUML (`.puml`)

| File | Description | Related Docs |
|------|-------------|-------------|
| `architecture.puml` | High-level system architecture: providers, ensemble, debate, handlers, storage | `docs/ARCHITECTURE.md` |
| `boot-sequence.puml` | Startup sequence: config load, container orchestration, provider verification, server start | `docs/deployment/` |
| `database-er.puml` | Full entity-relationship diagram: debate sessions, turns, code versions, users, tasks | `docs/SQL_SCHEMA.md` |
| `debate-orchestration-flow.puml` | Debate orchestrator 8-phase protocol flow | `docs/architecture/AGENTIC_ENSEMBLE.md` |
| `goroutine-lifecycle.puml` | Goroutine lifecycle pattern: WaitGroup tracking, context cancellation, graceful shutdown | `docs/development/SAFETY_FIXES.md` |
| `lazy-loading-architecture.puml` | Lazy loading patterns: sync.Once singletons, per-entity init, timeout handling | `docs/architecture/LAZY_LOADING_PATTERNS.md` |
| `module-dependency-graph.puml` | Dependency graph between all 41+ extracted modules | `docs/MODULES.md` |
| `security-scanning-pipeline.puml` | Security scanning workflow: gosec, Snyk, SonarQube, Trivy, Semgrep | `docs/security/SCANNING_GUIDE.md` |
| `test-pyramid.puml` | Test pyramid: unit, integration, E2E, security, stress, fuzz, pentest | `docs/testing/TEST_STRATEGY.md` |
| `adapter-helixqa.puml` | HelixQA adapter architecture: pipeline, evidence, ticket generation | `docs/testing/` |
| `qa-pipeline-flow.puml` | QA autonomous pipeline: session creation, test execution, finding collection | `docs/testing/` |
| `vision-pool-architecture.puml` | Remote vision pool: multi-instance Ollama/llama.cpp distribution | `docs/architecture/` |

### Mermaid (`.mmd`)

| File | Description | Related Docs |
|------|-------------|-------------|
| `architecture-overview.mmd` | Comprehensive architecture overview (Mermaid version) | `docs/ARCHITECTURE.md` |
| `architecture-mermaid.mmd` | Simplified architecture diagram for documentation embedding | `docs/ARCHITECTURE.md` |
| `boot-sequence.mmd` | Boot sequence flowchart (Mermaid version) | `docs/deployment/` |
| `database-er.mmd` | Database ER diagram (Mermaid version) | `docs/SQL_SCHEMA.md` |
| `data-flow.mmd` | Request data flow: client to provider to ensemble to response | `docs/architecture/` |
| `debate-system.mmd` | Debate system overview: agents, topology, voting, phases | `docs/architecture/AGENTIC_ENSEMBLE.md` |
| `debate-orchestration-flow-mermaid.mmd` | Debate orchestration flow (Mermaid version) | `docs/architecture/AGENTIC_ENSEMBLE.md` |
| `service-dependencies.mmd` | Service dependency graph: PostgreSQL, Redis, ChromaDB, Kafka | `docs/SERVICE_MANAGEMENT.md` |
| `shutdown-sequence.mmd` | Graceful shutdown sequence: drain connections, stop workers, close DB | `docs/deployment/` |
| `memory-system.mmd` | Memory system architecture: Mem0, entity graph, scopes | `docs/HELIXMEMORY_INTEGRATION.md` |
| `grpc-service.mmd` | gRPC service architecture and protobuf definitions | `docs/protocols/` |
| `bigdata_architecture.mmd` | BigData subsystem: infinite context, distributed memory, analytics | `docs/bigdata/` |
| `cross_session_learning.mmd` | Cross-session learning flow in debate system | `docs/architecture/AGENTIC_ENSEMBLE.md` |
| `data_lake_architecture.mmd` | Data lake architecture for analytics pipeline | `docs/bigdata/` |
| `distributed_memory_sync.mmd` | Distributed memory synchronization protocol | `docs/bigdata/` |
| `infinite_context_flow.mmd` | Infinite context window via event sourcing and Kafka replay | `docs/bigdata/` |
| `kafka_streams_topology.mmd` | Kafka Streams topology for real-time processing | `docs/architecture/kafka-integration.md` |
| `knowledge_graph_streaming.mmd` | Knowledge graph streaming pipeline | `docs/bigdata/` |

---

## Rendered Files (`docs/diagrams/rendered/`)

Pre-rendered SVG files for embedding in documentation and web views.

| File | Source |
|------|--------|
| `architecture-overview.svg` | `architecture-overview.mmd` |
| `architecture.svg` | `architecture-mermaid.mmd` |
| `bigdata_architecture.svg` | `bigdata_architecture.mmd` |
| `boot-sequence.svg` | `boot-sequence.mmd` |
| `cross_session_learning.svg` | `cross_session_learning.mmd` |
| `database-er.svg` | `database-er.mmd` |
| `data-flow.svg` | `data-flow.mmd` |
| `data_lake_architecture.svg` | `data_lake_architecture.mmd` |
| `debate-adversarial-round.svg` | (debate subsystem) |
| `debate-approval-gate-flow.svg` | (debate subsystem) |
| `debate-database-er.svg` | (debate subsystem) |
| `debate-full-protocol.svg` | (debate subsystem) |
| `debate-orchestration-flow.svg` | `debate-orchestration-flow-mermaid.mmd` |
| `debate-reflexion-loop.svg` | (debate subsystem) |
| `debate-system.svg` | `debate-system.mmd` |
| `debate-tree-topology.svg` | (debate subsystem) |
| `debate-voting-methods.svg` | (debate subsystem) |
| `distributed_memory_sync.svg` | `distributed_memory_sync.mmd` |
| `grpc-service.svg` | `grpc-service.mmd` |
| `infinite_context_flow.svg` | `infinite_context_flow.mmd` |
| `kafka_streams_topology.svg` | `kafka_streams_topology.mmd` |
| `knowledge_graph_streaming.svg` | `knowledge_graph_streaming.mmd` |
| `memory-system.svg` | `memory-system.mmd` |
| `service-dependencies.svg` | `service-dependencies.mmd` |
| `shutdown-sequence.svg` | `shutdown-sequence.mmd` |

---

## Rendering Diagrams

### PlantUML

```bash
# Requires plantuml.jar or plantuml CLI
plantuml docs/diagrams/src/*.puml -o ../rendered/ -tsvg
```

### Mermaid

```bash
# Requires @mermaid-js/mermaid-cli (mmdc)
npx @mermaid-js/mermaid-cli -i docs/diagrams/src/architecture-overview.mmd \
  -o docs/diagrams/rendered/architecture-overview.svg
```

### Batch Render

```bash
# PlantUML batch
for f in docs/diagrams/src/*.puml; do
  plantuml "$f" -o ../rendered/ -tsvg
done

# Mermaid batch
for f in docs/diagrams/src/*.mmd; do
  name=$(basename "$f" .mmd)
  npx @mermaid-js/mermaid-cli -i "$f" -o "docs/diagrams/rendered/${name}.svg"
done
```

---

## Adding New Diagrams

1. Create the source file in `docs/diagrams/src/` using PlantUML (`.puml`) or Mermaid (`.mmd`)
2. Render to SVG in `docs/diagrams/rendered/`
3. Add an entry to this catalog with a description and cross-reference
4. Reference the diagram from the relevant documentation file

**Naming convention:** `<subject>-<aspect>.<ext>` (e.g., `debate-orchestration-flow.mmd`)
