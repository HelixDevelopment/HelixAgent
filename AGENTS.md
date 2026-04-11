# AGENTS.md

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- **NO Git hooks (pre-commit, pre-push, post-commit, etc.)** may be installed or configured
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

---

# HelixAgent: AI-Powered Ensemble LLM Service

## Project Overview

HelixAgent is a production-ready, AI-powered ensemble LLM service written in Go (1.25+) that aggregates responses from multiple language models to provide the most accurate and reliable outputs. It provides OpenAI-compatible APIs with support for 47+ LLM providers, debate orchestration, MCP adapters, and containerized infrastructure. (Authoritative count: `ls internal/llm/providers/ | grep -v common`.)

**Module**: `dev.helix.agent`

**Main Binary**: `helixagent` (built from `cmd/helixagent/`)

**Additional Applications**:
- `api` - Standalone API server
- `grpc-server` - gRPC service endpoint
- `cognee-mock` - Mock Cognee service for testing
- `sanity-check` - System validation tool
- `mcp-bridge` - MCP protocol bridge
- `generate-constitution` - Constitution file generator

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              HelixAgent                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │   Web API    │  │  AI Debate   │  │  LLM Verifier│  │   MCP Servers   │  │
│  │    (Gin)     │  │ Orchestrator │  │   (Scoring)  │  │   (45+ Adapters)│  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └────────┬────────┘  │
│         │                 │                 │                   │           │
│         └─────────────────┴─────────────────┴───────────────────┘           │
│                                    │                                        │
└────────────────────────────────────┼────────────────────────────────────────┘
                                     │
         ┌───────────────────────────┼───────────────────────────┐
         ▼                           ▼                           ▼
┌─────────────────┐    ┌─────────────────────┐    ┌─────────────────────┐
│   PostgreSQL    │    │       Redis         │    │   47+ LLM Providers │
│                 │    │                     │    │  ┌───────────────┐  │
│   - Sessions    │    │   - Caching         │    │  │ Claude        │  │
│   - Analytics   │    │   - Rate Limiting   │    │  │ DeepSeek      │  │
│   - Debates     │    │   - Pub/Sub         │    │  │ Gemini        │  │
│   - Memory      │    │   - Sessions        │    │  │ Mistral       │  │
└─────────────────┘    └─────────────────────┘    │  │ Groq          │  │
                                                  │  │ Qwen          │  │
                                                  │  │ xAI/Grok      │  │
                                                  │  │ Cerebras      │  │
                                                  │  │ + 15 more     │  │
                                                  │  └───────────────┘  │
                                                  └─────────────────────┘
```

### Extracted Modules (41 Total)

The project uses Go workspace-style modules for modularity. All modules are in the repository root:

| Module | Path | Purpose |
|--------|------|---------|
| Agentic | `./Agentic` | Agent workflow orchestration |
| Auth | `./Auth` | Authentication and authorization |
| BackgroundTasks | `./BackgroundTasks` | Background job processing |
| Benchmark | `./Benchmark` | Performance benchmarking |
| Cache | `./Cache` | Distributed caching |
| Challenges | `./Challenges` | Challenge framework |
| Concurrency | `./Concurrency` | Concurrency utilities |
| Containers | `./Containers` | Docker/Podman container management |
| Database | `./Database` | Database abstraction layer |
| DebateOrchestrator | `./DebateOrchestrator` | AI debate coordination |
| DocProcessor | `./DocProcessor` | Document processing |
| Embeddings | `./Embeddings` | Embedding generation |
| EventBus | `./EventBus` | Event streaming |
| Formatters | `./Formatters` | Code formatting (32+ formatters) |
| HelixMemory | `./HelixMemory` | Memory management |
| HelixQA | `./HelixQA` | Quality assurance |
| HelixSpecifier | `./HelixSpecifier` | Project specification |
| LLMOps | `./LLMOps` | LLM operations |
| LLMOrchestrator | `./LLMOrchestrator` | LLM orchestration |
| LLMProvider | `./LLMProvider` | Provider abstractions |
| LLMsVerifier | `./LLMsVerifier` | Provider verification |
| MCP_Module | `./MCP_Module` | MCP protocol implementation |
| Memory | `./Memory` | Session memory |
| Messaging | `./Messaging` | Message broker abstractions |
| Models | `./Models` | Model definitions |
| Observability | `./Observability` | Metrics and tracing |
| Optimization | `./Optimization` | LLM optimization |
| Planning | `./Planning` | Planning algorithms (HiPlan, MCTS, ToT) |
| Plugins | `./Plugins` | Plugin system |
| RAG | `./RAG` | Retrieval-Augmented Generation |
| Security | `./Security` | Security services |
| SelfImprove | `./SelfImprove` | Self-improvement framework |
| SkillRegistry | `./SkillRegistry` | Skill management |
| Storage | `./Storage` | Object storage |
| Streaming | `./Streaming` | Response streaming |
| ToolSchema | `./ToolSchema` | Tool parameter schemas |
| VectorDB | `./VectorDB` | Vector database |
| VisionEngine | `./VisionEngine` | Vision processing |

### Internal Package Structure

```
internal/
├── adapters/           # Module adapters (bridge to extracted modules)
│   ├── containers/     # Container runtime adapter
│   ├── database/       # Database adapter
│   ├── messaging/      # Messaging adapter
│   └── ...
├── agents/             # Agent registry and management
├── analytics/          # Analytics and metrics
├── auth/               # Authentication utilities
├── background/         # Background task processing
├── benchmark/          # Benchmark runner
├── bigdata/            # Big data integration (Kafka, ClickHouse, Neo4j)
├── cache/              # Caching layer
├── challenges/         # Challenge framework
├── concurrency/        # Concurrency utilities
├── config/             # Configuration management
├── conversation/       # Conversation context
├── database/           # Database repositories
├── debate/             # Debate orchestration
│   ├── agents/         # Debate agent implementations
│   ├── orchestrator/   # Debate coordination
│   ├── topology/       # Debate topologies (mesh, star, chain)
│   └── ...
├── embeddings/         # Embedding management
├── events/             # Event bus
├── features/           # Feature flags
├── formatters/         # Code formatting (native + service)
├── graphql/            # GraphQL resolvers
├── handlers/           # HTTP handlers
│   ├── completion.go   # LLM completion endpoints
│   ├── debate_handler.go # Debate endpoints
│   ├── mcp.go          # MCP endpoints
│   └── ...
├── http/               # HTTP utilities
├── knowledge/          # Knowledge graph
├── learning/           # Cross-session learning
├── llm/                # LLM provider implementations
│   └── providers/      # Individual provider implementations
│       ├── claude/
│       ├── deepseek/
│       ├── gemini/
│       └── ... (22+ providers)
├── llmops/             # LLM operations
├── lsp/                # Language Server Protocol
├── mcp/                # Model Context Protocol
│   ├── adapters/       # MCP adapters (45+ implementations)
│   ├── servers/        # MCP server implementations
│   └── bridge/         # MCP bridge
├── memory/             # Memory management
├── messaging/          # Message broker (Kafka, RabbitMQ)
├── middleware/         # HTTP middleware
├── models/             # Data models
├── modelsdev/          # Models.dev integration
├── notifications/      # Notification system
├── observability/      # Metrics and tracing
├── optimization/       # LLM optimization (semantic cache, streaming)
├── planning/           # Planning algorithms
├── plugins/            # Plugin system
├── rag/                # RAG pipeline
├── repository/         # Repository pattern
├── router/             # HTTP routing
├── routing/            # Request routing
├── security/           # Security utilities
├── services/           # Business logic
│   ├── debate_service.go
│   ├── ensemble.go
│   ├── provider_registry.go
│   └── ...
├── tools/              # Tool system
├── utils/              # Utilities
└── verifier/           # Provider verification
```

## Technology Stack

### Core
- **Language**: Go 1.25.3+
- **Web Framework**: Gin v1.12.0
- **Module System**: Go modules with local replace directives

### Databases
- **Primary**: PostgreSQL 15+ (with pgvector)
- **Cache**: Redis 7+
- **Vector**: ChromaDB, Qdrant
- **Graph**: Neo4j, Memgraph (optional)
- **Analytics**: ClickHouse (optional)

### Messaging
- **Apache Kafka** - Event streaming
- **RabbitMQ** - Message queuing
- **Redis Pub/Sub** - In-memory messaging

### Infrastructure
- **Container Runtimes**: Docker, Podman
- **Orchestration**: Docker Compose / Podman Compose
- **Monitoring**: Prometheus + Grafana
- **Tracing**: OpenTelemetry

### LLM Providers (22+ Supported)
| Provider | Auth Type | Notes |
|----------|-----------|-------|
| Claude | API Key, OAuth | Primary recommendation |
| DeepSeek | API Key | High performance |
| Gemini | API Key, OAuth | Google's models |
| Mistral | API Key | European provider |
| Groq | API Key | Fast inference |
| Qwen | API Key, OAuth | Alibaba models |
| xAI/Grok | API Key | xAI models |
| Cerebras | API Key | Cerebras hardware |
| OpenRouter | API Key | Model aggregator |
| Perplexity | API Key | Search+LLM |
| Together | API Key | Model aggregator |
| Fireworks | API Key | Fast inference |
| Cloudflare | API Key | Workers AI |
| Ollama | Local | **DEPRECATED for production** |
| Zen | Local | Free local models |
| + 8 more | Various | See internal/llm/providers/ |

## Build Commands

```bash
# Core build commands
make build              # Build helixagent binary (output: bin/helixagent)
make build-debug        # Build with debug symbols
make build-all          # Multi-architecture build

# Release builds
make release            # Build helixagent for all platforms
make release-all        # Build ALL 7 apps for all platforms
make release-<app>      # Build specific app (helixagent, api, grpc-server, etc.)

# Run commands
make run                # Run locally
make run-dev            # Development mode (GIN_MODE=debug)
./bin/helixagent        # Run built binary (auto-starts containers)
```

## Testing Commands

### Resource Limits (CRITICAL)
All tests MUST respect host resource limits (30-40%):
```bash
export GOMAXPROCS=2
# Tests run with: nice -n 19 ionice -c 3 go test -p 1 ...
```

### Test Categories
```bash
# Main test commands
make test               # All tests (auto-detects infrastructure)
make test-unit          # Unit tests only (./internal/... -short)
make test-integration   # Integration tests with Docker deps
make test-e2e           # End-to-end tests
make test-security      # Security tests
make test-stress        # Stress tests
make test-chaos         # Challenge/chaos tests
make test-bench         # Benchmark tests
make test-race          # Race condition detection
make test-coverage      # Coverage with HTML report
make test-coverage-100  # Enforce 100% coverage

# Full test suites
make test-all           # ALL tests with full infrastructure
make test-complete      # Complete test suite (6 types) with coverage

# Challenge tests
make test-challenges    # Run performance challenges
```

### Test Infrastructure
```bash
# Automatic (Recommended)
./bin/helixagent        # Auto-starts all required containers

# Manual (Deprecated - use binary auto-start instead)
make test-infra-start   # Start PostgreSQL, Redis, Mock LLM
make test-infra-stop    # Stop test infrastructure
make test-with-infra    # Run tests with Docker infra
```

### Single Test Execution
```bash
go test -v -run TestFunctionName ./path/to/package
go test -v ./internal/llm
go test -v -run "Test.*Integration" ./...

# With resource limits
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -v -p 1 -run TestName ./path/to/package
```

## Code Quality Commands

```bash
# Formatting and linting
make fmt                # Format with go fmt
make vet                # Run go vet
make lint               # Run golangci-lint
make ci-pre-commit      # Pre-commit checks (fmt, vet)
make ci-pre-push        # Pre-push checks (includes unit tests)

# Security scanning
make security-scan      # Comprehensive security scan
make security-scan-gosec    # Gosec security checker
make security-scan-trivy    # Trivy vulnerability scanner
make security-scan-snyk     # Snyk vulnerability scanner
make security-scan-sonarqube # SonarQube analysis
make security-scan-semgrep  # Semgrep pattern scanner
make security-scan-kics     # KICS IaC scanner
make security-scan-grype    # Grype vulnerability scanner
```

## Docker Commands

```bash
# Main docker commands
make docker-build       # Build Docker image
make docker-build-prod  # Production image build
make docker-run         # Start with docker-compose
make docker-stop        # Stop services
make docker-logs        # View logs
make docker-clean       # Clean containers

# Profile-specific
make docker-full        # Full stack (all services)
make docker-ai          # AI services only
make docker-monitoring  # Monitoring stack only
make docker-dev         # Development profile
make docker-prod        # Production profile

# Podman support
make podman-build       # Build with Podman
make podman-run         # Run with Podman Compose
make podman-full        # Full Podman environment
```

## Code Style Guidelines

### Go Conventions
- **Formatting**: Use `gofmt` / `goimports`. Run `make fmt` before committing.
- **Imports**: Grouped as: stdlib, third-party, internal (blank line separated)
- **Line Length**: ≤ 100 characters
- **Naming**:
  - `camelCase` for private identifiers
  - `PascalCase` for exported identifiers
  - `UPPER_SNAKE_CASE` for constants
  - Acronyms: all caps (`HTTP`, `URL`, `ID`, `JSON`)
  - Receiver names: 1-2 letters (`s` for service, `c` for client)

### Error Handling
```go
// Always check errors, wrap with context
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Use defer for cleanup
file, err := os.Open(path)
if err != nil {
    return err
}
defer file.Close()
```

### Types and Interfaces
- Use `interface` to define behavior, not data
- Prefer small, focused interfaces
- Avoid `any`/`interface{}`; use generics where appropriate

### Concurrency
- Always use `context.Context` for cancellation
- Protect shared data with `sync.Mutex`/`sync.RWMutex`
- Use `sync.WaitGroup` for goroutine coordination
- Never leak goroutines

### Tool Parameters
All tool parameters use **snake_case**. See `internal/tools/schema.go`.

### Comments
**DO NOT ADD COMMENTS** unless explicitly requested. Self-documenting code preferred.

## Testing Guidelines

### Test Structure
```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {name: "valid case", input: "test", want: "result"},
        {name: "error case", input: "", wantErr: true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Feature(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Mock Usage
- **Unit Tests**: Mocks/stubs allowed
- **Integration/E2E Tests**: Use real services, NO mocks
- **Production Code**: NO mocks, stubs, placeholder classes, or TODO implementations allowed

### Test Coverage Requirements
- **Core Logic**: 95%+ coverage
- **Handlers**: 90%+ coverage
- **Integration**: All critical paths tested

## Configuration

### Environment Variables
Key configuration via environment (see `.env.example`):

```bash
# Server
PORT=7061
GIN_MODE=release
JWT_SECRET=your-secret

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=helixagent
DB_PASSWORD=helixagent123
DB_NAME=helixagent_db

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=helixagent123

# LLM Providers (set at least one)
CLAUDE_API_KEY=sk-...
DEEPSEEK_API_KEY=sk-...
GEMINI_API_KEY=...
# ... see .env.example for all providers

# Feature Flags
BIGDATA_ENABLE_INFINITE_CONTEXT=true
BIGDATA_ENABLE_KNOWLEDGE_GRAPH=false
CONSTITUTION_WATCHER_ENABLED=false
```

### Configuration Files
- `.env` - Local environment configuration
- `.env.example` - Template with all options
- `docker-compose.yml` - Service orchestration
- `configs/` - YAML configuration files

## API Endpoints

### Core Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health status |
| `/v1/health` | GET | Detailed health with provider status |
| `/v1/models` | GET | Available LLM models |
| `/v1/providers` | GET | Configured providers |
| `/metrics` | GET | Prometheus metrics |

### Completion Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/completions` | POST | Single completion |
| `/v1/chat/completions` | POST | Chat completions (OpenAI-compatible) |
| `/v1/completions/stream` | POST | Streaming completions |
| `/v1/completion/*` | POST | Completion pipeline endpoints |
| `/v1/ensemble/completions` | POST | Ensemble completions |
| `/v1/ensemble/sessions` | GET/POST | Ensemble session management |
| `/v1/ensemble/teams` | GET/POST | Ensemble team management |

### Protocol Endpoints
| Endpoint | Description |
|----------|-------------|
| `/v1/mcp` | Model Context Protocol |
| `/v1/acp` | Agent Communication Protocol |
| `/v1/lsp` | Language Server Protocol |
| `/v1/embeddings` | Embedding generation |
| `/v1/vision` | Vision processing |
| `/v1/cognee` | Cognee knowledge graph |
| `/v1/bigdata/health` | BigData services health |
| `/v1/agentic/workflows` | Agentic workflows |
| `/v1/planning/{hiplan,mcts,tot}` | Planning algorithms |
| `/v1/llmops/{experiments,evaluate,prompts}` | LLMOps |
| `/v1/benchmark/{run,results}` | Benchmarking |
| `/v1/qa/{sessions,findings,platforms,discover}` | QA automation |
| `/v1/ensemble/{sessions,teams}` | Ensemble session and team management |
| `/v1/completion/*` | Completion pipeline endpoints |

## Challenge Scripts

```bash
./challenges/scripts/run_all_challenges.sh                              # All challenges
# Phase 1-3 new challenges:
./challenges/scripts/output_pipeline_challenge.sh                       # 11 tests
./challenges/scripts/container_lazy_loading_challenge.sh                # 13 tests
./challenges/scripts/ensemble_handler_challenge.sh                      # 16 tests
./challenges/scripts/browser_handler_challenge.sh                       # 11 tests
./challenges/scripts/search_handler_challenge.sh                        # 13 tests
./challenges/scripts/fuzz_test_validation_challenge.sh                  # 16 tests
./challenges/scripts/feature_flag_challenge.sh                          # 13 tests
./challenges/scripts/env_var_completeness_challenge.sh                  # 22 tests
./challenges/scripts/safety_regression_challenge.sh                     # 16 tests
./challenges/scripts/test_type_completeness_challenge.sh                # 21 tests
./challenges/scripts/performance_baseline_challenge.sh                  # 16 tests
./challenges/scripts/stress_resilience_challenge.sh                     # 21 tests
./challenges/scripts/security_scan_results_challenge.sh                 # 17 tests
./challenges/scripts/grafana_dashboard_content_challenge.sh             # 7 tests
./challenges/scripts/lazy_loading_comprehensive_challenge.sh            # 16 tests
./challenges/scripts/brotli_compression_challenge.sh                    # 11 tests
```

## Adding New Components

### Adding a New LLM Provider
1. Create package: `internal/llm/providers/<name>/<name>.go`
2. Implement `LLMProvider` interface
3. Add tests: `internal/llm/providers/<name>/<name>_test.go`
4. Register in `internal/services/provider_registry.go`
5. Add env vars to `.env.example`

### Adding a New MCP Adapter
1. Create file: `internal/mcp/adapters/<service>.go`
2. Implement adapter interface
3. Register in `internal/mcp/adapters/registry.go`
4. Add tests: `internal/mcp/adapters/<service>_test.go`

### Adding a New Handler
1. Create file: `internal/handlers/<name>_handler.go`
2. Implement handler functions
3. Add routes in `internal/router/gin_router.go`
4. Add tests: `internal/handlers/<name>_handler_test.go`
5. Update API documentation

## Git Workflow

### Branch Naming
- `feat/description` - New features
- `fix/description` - Bug fixes
- `chore/description` - Maintenance
- `docs/description` - Documentation
- `refactor/description` - Code refactoring
- `test/description` - Test additions

### Commit Format (Conventional Commits)
```
type(scope): description

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Examples:
```
feat(llm): add support for new provider XYZ
fix(handlers): resolve race condition in debate handler
docs(readme): update installation instructions
```

### Pre-commit Requirements
```bash
make fmt vet lint  # MUST run before committing
```

## Development Standards (NON-NEGOTIABLE)

1. **100% Test Coverage** - Unit, integration, E2E, security, stress, chaos, benchmark tests. Mocks ONLY in unit tests.
2. **Challenge Coverage** - Every component MUST have Challenge scripts (`./challenges/scripts/`).
3. **Containerization** - All services in containers. Auto boot-up via HelixAgent binary.
4. **Configuration via HelixAgent Only** - CLI agent configs generated ONLY by `./bin/helixagent --generate-agent-config=<name>`.
5. **Real Data** - Beyond unit tests, use actual API calls, real databases, live services.
6. **No Mocks in Production** - Mocks, stubs, placeholder classes, TODO implementations FORBIDDEN in production.
7. **Resource Limits** - ALL tests limited to 30-40% host resources: `GOMAXPROCS=2 nice -n 19 ionice -c 3`.
8. **No CI/CD** - No automated pipelines, no Git hooks. All manual or Makefile-driven.

## Key Conventions

### Container Operations
**ALL container operations MUST go through the Containers module adapter:**
```go
import containeradapter "dev.helix.agent/internal/adapters/containers"

adapter := containeradapter.NewAdapter(logger)
adapter.ComposeUp(ctx, composeFile, profile)
```

Forbidden: Direct `docker`/`podman` exec commands.

### Service Startup
The HelixAgent binary automatically manages container lifecycle:
```bash
./bin/helixagent  # Reads Containers/.env, orchestrates everything
```

### CLI Agent Configuration
Generate configs ONLY through the binary:
```bash
./bin/helixagent --generate-agent-config=<name>
./bin/helixagent --generate-all-agents --all-agents-output-dir=./configs
```

## Troubleshooting

### Common Issues

**Provider Authentication Failures:**
```bash
# Check provider configuration
curl http://localhost:7061/v1/providers

# Test specific provider
curl http://localhost:7061/v1/providers/claude/health
```

**Database Connection Issues:**
```bash
# Check PostgreSQL
docker compose exec postgres pg_isready -U helixagent -d helixagent_db

# Verify connection from app
curl http://localhost:7061/v1/health
```

**Container Runtime Issues:**
```bash
# Detect runtime
make container-detect

# Check container status
make container-status
```

### Debug Mode
```bash
export LOG_LEVEL=debug
export GIN_MODE=debug
export DEBUG_ENABLED=true
make run-dev
```

## Quick Reference

| Task | Command |
|------|---------|
| Build | `make build` |
| Run | `./bin/helixagent` |
| Test | `make test` |
| Unit tests only | `make test-unit` |
| Format | `make fmt` |
| Lint | `make lint` |
| Security scan | `make security-scan` |
| Docker full | `make docker-full` |
| Docker stop | `make docker-stop` |
| Generate agent config | `./bin/helixagent --generate-agent-config=<name>` |
| List all agents | `./bin/helixagent --list-agents` |

---

**Last Updated**: 2026-04-02
**Version**: 1.0.0
**Go Version**: 1.25.3+

---

## CLI Agents Documentation

HelixAgent supports **47 CLI agents** with unified configuration generation, validation, and plugin integration.

### Documentation Structure

Each CLI agent has comprehensive documentation in `docs/cli-agents/`:

| File | Purpose |
|------|---------|
| README.md | Overview, features, installation |
| ARCHITECTURE.md | System design, components |
| API.md | CLI reference, endpoints |
| USAGE.md | Workflows, examples |
| REFERENCES.md | External resources |
| USER-GUIDE.md | Complete user manual (600-900+ lines) |
| DIAGRAMS.md | Visual diagrams (Tier 1 agents) |
| GAP_ANALYSIS.md | Improvement opportunities (Tier 1) |

### Integration Documentation

Integration docs in `docs/cli-agents-integration/`:

| File | Purpose |
|------|---------|
| ARCHITECTURE.md | System architecture, data flows |
| MCP_SERVERS.md | 45+ MCP servers reference |
| HTTP_ENDPOINTS.md | All API endpoints documented |
| README.md | Integration overview |

### Configuration Files

All 47 agents have JSON configs in `cli_agents_configs/`:
- Provider settings (HelixAgent endpoint)
- MCP server configurations
- Model capabilities
- Formatter preferences

### Key Integration Points

1. **Provider**: `http://localhost:7061/v1` (OpenAI-compatible)
2. **Model**: `helixagent-debate` (AI Debate Ensemble)
3. **MCP Servers**: 45+ available
4. **Formatters**: 32+ programming languages

### Generate Configurations

```bash
# List all 47 agents
./bin/helixagent --list-agents

# Generate for specific agent
./bin/helixagent --generate-agent-config=codex --agent-config-output=codex.json

# Generate all 47 at once
./bin/helixagent --generate-all-agents --all-agents-output-dir=~/agent-configs/
```

### Total Documentation

- **47 CLI Agents** fully documented
- **850 Documentation Files**
- **325,013 Lines** of documentation
- **Complete User Guides** for all agents

