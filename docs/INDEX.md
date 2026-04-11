# HelixAgent Documentation Index

Generated 2026-04-11 during Phase 7 remediation. This is a **hand-written
index** of the top 60 markdown files and 55 subdirectories in `docs/`. It is
not auto-generated (yet) — refresh it whenever a major section is added.

See also:
- `../README.md` — project overview, quick start
- `../CLAUDE.md` — Claude Code operating guide
- `../AGENTS.md` — agent-facing operating guide
- `../CONSTITUTION.json` / `CONSTITUTION.md` — 26 mandatory rules

## Core references (start here)

| File | Purpose |
|---|---|
| `ARCHITECTURE.md` | High-level architecture overview |
| `ARCHITECTURE_DIAGRAMS.md` | Rendered and source diagrams |
| `DEVELOPER_GUIDE.md` | Day-to-day developer workflow |
| `FEATURES.md` | Feature catalog (source of truth for provider counts) |
| `MODULES.md` | 41-module catalog with ownership |
| `API_REFERENCE.md` / `api/openapi.yaml` | REST / OpenAPI surface |
| `FAQ.md` | Frequently asked questions |
| `CONTRIBUTING.md` | Contribution guidelines |

## Getting started & operations

| File | Purpose |
|---|---|
| `guides/quickstart.md` | Quickest path to a running instance |
| `installation/` | Install guides |
| `deployment/` | Prod-grade deployment notes |
| `operations/` | Runbooks, on-call procedures |
| `runbooks/` | Named incident runbooks |
| `monitoring/` | Metrics, dashboards, alert wiring |
| `observability/` | OpenTelemetry, tracing, logging |

## Feature-specific docs

| File | Purpose |
|---|---|
| `HELIXLLM_USER_MANUAL.md` | HelixLLM local inference manual |
| `HELIXLLM_TESTING_GUIDE.md` | HelixLLM test surface |
| `HELIXMEMORY_SETUP.md` | HelixMemory installation |
| `HELIXMEMORY_INTEGRATION.md` | HelixMemory integration guide |
| `CODE_FORMATTERS_CATALOG.md` | 32+ formatters catalog |
| `MCP_COMPLETE_INTEGRATION.md` | MCP adapter integration |
| `ADVANCED_AI_FEATURES.md` | Advanced capabilities reference |
| `CLI_AGENTS_RESEARCH_2025.md` | 48 CLI agents research / design |

## Subsystem deep-dives

| Directory | Covers |
|---|---|
| `api/` | REST + gRPC + OpenAPI + event types |
| `architecture/` | Architecture deep-dives per subsystem |
| `bigdata/` | BigData subsystem (infinite context, knowledge graphs) |
| `cli-agents/` | 48 CLI agent configuration & docs |
| `database/` | Schema guides, migration notes |
| `features/` | Feature-specific documents |
| `mcp/` | MCP adapter catalog |
| `mcp-servers/` | Containerized MCP server specs |
| `protocols/` | MCP / ACP / LSP / SSE protocol references |
| `providers/` | LLM provider per-provider docs |
| `rag/` (via `features`) | RAG subsystem |
| `verifier/` | LLMsVerifier integration |

## Learning materials

| Directory | Content |
|---|---|
| `courses/` | Full video courses (80+ files, 20 slide decks, 15 labs) |
| `tutorials/` | Stand-alone tutorials |
| `user-guides/` | Step-by-step user guides |
| `manuals/` | Long-form manuals |
| `superpowers/` | Advanced usage patterns |
| `testing/` | Test strategy + how to run every test type |

## Security & compliance

| File / Dir | Purpose |
|---|---|
| `security/` | Security docs (Phase 4 findings live here: `SECURITY_FINDINGS_2026-04-11.md`) |
| `memory_safety/` | Memory-safety audits |
| `validation/` | Validation & acceptance checklists |

## Release & planning

| File / Dir | Purpose |
|---|---|
| `plans/` | Active / historical implementation plans |
| `development/` | Developer-facing dev notes (contains `BASELINE_2026-04-11.md`) |
| `archive/phase-summaries/` | Archived phase completion reports (13 files, moved 2026-04-11) |
| `reports/` | Project reports and audits |
| `implementation_specs/` | Formal implementation specifications |
| `migration/` | Migration guides |

## Historical / background

| Directory | Notes |
|---|---|
| `research/` | Original research notes for CLI agent porting, etc. |
| `fixes/` | Historical fix write-ups |
| `COMPREHENSIVE_PROJECT_REPORT.md` | Historical comprehensive report |
| `FINAL_COMPLETION_REPORT.md` | Earlier milestone final report |
| `IMPLEMENTATION_FINAL_SUMMARY.md` | Earlier implementation summary |

## What this index is NOT

- **Not** a replacement for file search — use `grep` / `rg` for text queries.
- **Not** auto-updated. Refresh this file when you add a major directory or
  rename something high-traffic.
- **Not** the ground truth for counts — code is. Provider count: run
  `ls internal/llm/providers/ | grep -v common`. Module count: run
  `find . -maxdepth 2 -name go.mod | wc -l`.
