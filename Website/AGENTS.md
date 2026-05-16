# AGENTS.md - Website Module

## Module Purpose

The `Website/` module contains all user-facing documentation for HelixAgent: step-by-step
user manuals, structured video course content, and supplementary website assets. It has no
Go code — it is a pure content module.

## Development Standards

1. **Complete Documentation** — Every HelixAgent feature MUST have a corresponding entry in
   at least one user manual. New providers, protocols, modules, and endpoints require updates.

2. **Video Course Coverage** — Every major feature or module should have at least one video
   course lesson. Significant new capabilities (e.g., new extracted module, new debate
   topology) warrant a dedicated course file.

3. **No CI/CD** — All validation and publishing is done manually. No automated pipelines
   may be configured.

4. **Sequential Numbering** — Manuals are numbered `01`–`NN`; courses follow their own
   sequential series. Never reuse or skip numbers.

5. **Preserve Existing Content** — Never delete or overwrite existing manual or course
   content. Always append or insert; never remove.

6. **Real Examples** — All API examples must use actual HelixAgent endpoints and realistic
   request/response bodies. No fabricated or placeholder outputs.

## Module Dependencies

This module has no Go module dependencies. It contains only Markdown content and shell
scripts. Scripts under `scripts/` may call standard Unix tools and `curl`.

## File Inventory

### User Manuals (`user-manuals/`)

| File | Topic |
|------|-------|
| `01-getting-started.md` | Installation, setup, first API requests |
| `02-provider-configuration.md` | Claude, Gemini, DeepSeek, Qwen, ZAI, Ollama, OpenRouter |
| `03-ai-debate-system.md` | Debate participants, roles, consensus, Cognee, memory |
| `04-api-reference.md` | Endpoints, request/response formats, auth, rate limiting |
| `05-deployment-guide.md` | Production, Docker, Kubernetes, monitoring, scaling |
| `06-administration-guide.md` | User/provider management, security, backup |
| `07-protocols.md` | MCP, LSP, ACP, MCP Tool Search |
| `08-troubleshooting.md` | Common issues, auth errors, timeouts, rate limits |
| `09-mcp-integration.md` | MCP adapters, built-in adapters, tool chaining |
| `10-security-hardening.md` | JWT, API Key, OAuth, RBAC, TLS, audit logging |
| `11-performance-tuning.md` | Caching, connection pooling, batch processing |
| `12-plugin-development.md` | Plugin architecture, types, lifecycle, testing |
| `13-bigdata-integration.md` | BigData components, Neo4j, ClickHouse, Kafka |
| `14-grpc-api.md` | gRPC server, protobuf, streaming, authentication |
| `15-memory-system.md` | Mem0-style memory, entity graphs, semantic search |
| `16-code-formatters.md` | 32+ formatters, REST API, service formatters |
| `17-security-scanning-guide.md` | Snyk, SonarQube, containerized scanning |
| `18-performance-monitoring.md` | Prometheus, OpenTelemetry, Grafana dashboards |
| `19-concurrency-patterns.md` | Worker pools, rate limiters, circuit breakers |
| `20-testing-strategies.md` | Unit, integration, E2E, security, benchmark tests |
| `21-challenge-development.md` | Challenge scripts, assertion engine, runner |
| `22-custom-provider-guide.md` | Implementing LLMProvider, registering providers |
| `23-observability-setup.md` | Jaeger, Zipkin, Langfuse, structured logging |
| `24-backup-recovery.md` | PostgreSQL backup, Redis persistence, recovery |
| `25-multi-region-deployment.md` | Multi-region architecture, traffic routing |
| `26-compliance-guide.md` | GDPR, SOC2, audit trails, data residency |
| `27-api-rate-limiting.md` | Rate limit configuration, headers, strategies |
| `28-custom-middleware.md` | Gin middleware, auth, CORS, request logging |
| `29-disaster-recovery.md` | RTO/RPO, failover procedures, runbooks |
| `30-enterprise-architecture.md` | HA design, service mesh, secrets management |
| `31-fuzz-testing-guide.md` | Corpus management, fuzz targets, CI integration |
| `32-automated-security-scanning.md` | Snyk + SonarQube pipeline, findings remediation |
| `33-performance-optimization-guide.md` | Profiling, pprof, bottleneck identification |
| `34-agentic-workflows-guide.md` | Graph-based workflows, multi-step execution |
| `35-llmops-experimentation-guide.md` | A/B experiments, prompt versioning, evaluation |
| `36-planning-algorithms-guide.md` | HiPlan, MCTS, Tree of Thoughts |
| `37-benchmark-guide.md` | SWE-bench, HumanEval, MMLU, leaderboard |
| `38-docprocessor-guide.md` | Documentation processing, feature map extraction |
| `39-helixqa-guide.md` | QA orchestration, crash detection, ticket generation |
| `40-llmorchestrator-guide.md` | CLI agent management, lifecycle, circuit breakers |
| `41-visionengine-guide.md` | Computer vision, UI analysis, LLM vision providers |
| `42-module-integration-guide.md` | Extracted modules, replace directives, adapters |
| `43-agentic-ensemble-guide.md` | Agentic + ensemble integration, multi-agent coordination |

Next manual number: **44**

### Video Courses (`video-courses/`)

Primary series (`course-NN-<topic>.md`): 01–18, 66–76 (next: **77**)
Extended series (`video-course-NN-<topic>.md`): 53–65
Batch subdirectories: `courses-19-24/`, `courses-21-30/`, `courses-31-40/`, `courses-41-50/`

## Adding New Content

### Adding a User Manual

1. Create `user-manuals/NN-<topic-slug>.md` with the next sequential number.
2. Add an entry to `user-manuals/README.md` under the appropriate section.
3. Update this file's inventory table above.
4. Cross-reference from related existing manuals where relevant.

### Adding a Video Course

1. Create `video-courses/course-NN-<topic>.md` with the next sequential number.
2. Update `video-courses/VIDEO_METADATA.md` if it tracks course metadata.
3. Cross-reference from the related user manual.

### Updating an Existing Manual

1. Read the full file before editing.
2. Append new sections at the end, or insert at a logical point.
3. Never remove or overwrite existing headings or content.
4. Verify all curl examples still use `localhost:7061` and correct `/v1/` paths.

## Testing

Validate documentation integrity manually:

```bash
# Check for broken internal links (requires markdown-link-check or similar)
find /run/media/milosvasic/DATA4TB/Projects/helix_agent/Website/user-manuals -name "*.md" \
  | xargs grep -l '\[.*\](\.\./' | head -20

# Verify sequential numbering has no gaps
ls Website/user-manuals/*.md | grep -oP '^\d+' | sort -n

# Confirm all API examples use correct port
grep -rn 'localhost:' Website/user-manuals/ | grep -v ':7061' || echo "All ports correct"
```

## Coordination with Other Modules

- When a new extracted module is added to the project, create a corresponding user manual
  and update the video course catalog.
- When `CLAUDE.md` or `AGENTS.md` in the root project is updated with new features, verify
  the relevant user manual reflects those changes.
- The `DocProcessor` module (`DocProcessor/`) can be used to extract feature maps from
  this content for coverage tracking.
