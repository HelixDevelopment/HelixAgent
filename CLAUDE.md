# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SuperAgent is an AI-powered ensemble LLM service written in Go (1.24+) that combines responses from multiple language models using intelligent aggregation strategies. It provides OpenAI-compatible APIs and supports 7 LLM providers (Claude, DeepSeek, Gemini, Qwen, ZAI, Ollama, OpenRouter).

The project also includes:
- **Toolkit** (`Toolkit/`): A standalone Go library for building AI applications with multi-provider support
- **LLMsVerifier** (`LLMsVerifier/`): A verification system for LLM provider accuracy and reliability

## Build Commands

```bash
make build              # Build SuperAgent binary
make build-debug        # Build with debug symbols
make run                # Run locally
make run-dev            # Run in development mode (GIN_MODE=debug)
make docker-build       # Build Docker image
docker-compose up -d    # Start full stack
```

## Testing

```bash
make test                  # Run all tests
make test-coverage         # Tests with HTML coverage report
make test-unit             # Unit tests only (./internal/... -short)
make test-integration      # Integration tests (./tests/integration)
make test-e2e              # End-to-end tests (./tests/e2e)
make test-security         # Security tests (./tests/security)
make test-stress           # Stress tests (./tests/stress)
make test-chaos            # Chaos/challenge tests (./tests/challenge)
make test-bench            # Benchmark tests
make test-race             # Race condition detection
```

Run a single test:
```bash
go test -v -run TestName ./path/to/package
```

### Test Infrastructure (Docker-based)
```bash
make test-infra-start   # Start PostgreSQL, Redis, Mock LLM containers
make test-infra-stop    # Stop test containers
make test-infra-clean   # Stop and remove volumes
make test-with-infra    # Run all tests with Docker infrastructure
```

## Code Quality

```bash
make fmt              # Format code (go fmt)
make vet              # Static analysis (go vet)
make lint             # Run golangci-lint
make security-scan    # Security scanning (gosec)
make install-deps     # Install dev dependencies (golangci-lint, gosec)
```

## Architecture

### Entry Points
- `cmd/superagent/` - Main SuperAgent application
- `cmd/api/` - API server
- `cmd/grpc-server/` - gRPC server

### Core Packages (`internal/`)
- `llm/` - LLM provider abstractions and ensemble orchestration
  - `providers/` - Individual implementations (claude, deepseek, gemini, ollama, qwen, zai, openrouter)
  - `ensemble.go` - Ensemble orchestration logic
- `services/` - Business logic
  - `provider_registry.go` - Provider management
  - `ensemble.go` - Ensemble service
  - `context_manager.go` - Multi-source context aggregation
  - `mcp_client.go` - Model Context Protocol client
  - `lsp_manager.go` - Language Server Protocol manager
  - `plugin_system.go` - Hot-reloadable plugin architecture
- `handlers/` - HTTP handlers & API endpoints (OpenAI-compatible, MCP, LSP, Cognee, AI Debate)
- `middleware/` - Auth, rate limiting, CORS, validation
- `cache/` - Caching layer (Redis, in-memory)
- `database/` - PostgreSQL connections and repositories
  - `protocol_repository.go` - MCP/LSP/ACP server configs, protocol cache, and metrics
- `models/` - Data models, enums, and protocol types
- `plugins/` - Hot-reloadable plugin system with discovery, health, metrics
- `modelsdev/` - Models.dev API client for model metadata

### Key Interfaces (Extensibility Points)
- `LLMProvider` - Provider implementation contract
- `VotingStrategy` - Ensemble voting strategies
- `PluginRegistry` / `PluginLoader` - Plugin system
- `CacheInterface` - Caching abstraction
- `CloudProvider` - Cloud integration

### Architectural Patterns
- **Provider Registry**: Unified interface for multiple LLM providers with credential management
- **Ensemble Strategy**: Confidence-weighted voting, majority vote, parallel execution
- **AI Debate System**: Multi-round debate between providers for consensus (see `internal/services/debate_*.go`)
  - API: POST `/v1/debates`, GET `/v1/debates`, GET `/v1/debates/:id`, DELETE `/v1/debates/:id`
  - Supports async execution with status polling via `/v1/debates/:id/status`
- **Plugin System**: Hot-reloadable plugins with dependency resolution
- **Circuit Breaker**: Fault tolerance for provider failures with health monitoring
- **Protocol Managers**: Unified MCP/LSP/ACP protocol handling
- **Cognee Integration**: Knowledge graph and RAG capabilities
- **Middleware Chain**: Auth, rate limiting, validation pipeline
- **LLM Optimization**: Semantic caching, structured output, enhanced streaming (see below)

### LLM Optimization (`internal/optimization/`)

SuperAgent integrates 8 LLM optimization tools for performance and quality:

| Package | Purpose | Key Features |
|---------|---------|--------------|
| `gptcache/` | Semantic caching | Vector similarity, LRU eviction, TTL |
| `outlines/` | Structured output | JSON schema validation, regex patterns, choice constraints |
| `streaming/` | Enhanced streaming | Word/sentence buffering, progress tracking, rate limiting |
| `sglang/` | Prefix caching | RadixAttention, session management (GPU required) |
| `llamaindex/` | Document retrieval | HyDE, reranking, Cognee integration |
| `langchain/` | Task decomposition | Chain execution, ReAct agents |
| `guidance/` | Grammar constraints | CFG-based generation, templates |
| `lmql/` | Query language | Declarative constraints, decoding strategies |

**Start optimization services:**
```bash
docker-compose --profile optimization up -d     # CPU-only optimization
docker-compose --profile optimization-gpu up -d # With GPU support (SGLang)
```

**Configuration**: See `configs/production.yaml` under `optimization:` section.

**Documentation**: See `docs/optimization/` and `docs/guides/LLM_OPTIMIZATION_USER_GUIDE.md`.

## Technology Stack

- **Framework**: Gin (v1.11.0)
- **Database**: PostgreSQL 15 with pgx/v5 driver
- **Cache**: Redis 7
- **Protocols**: OpenAI-compatible REST, gRPC, MCP, LSP
- **Testing**: testify (v1.11.1)
- **Monitoring**: Prometheus, Grafana

## Configuration

Environment variables defined in `.env.example`. Key categories:
- Server: `PORT`, `GIN_MODE`, `JWT_SECRET`
- Database: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
- Redis: `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`
- LLM providers: `CLAUDE_API_KEY`, `DEEPSEEK_API_KEY`, `GEMINI_API_KEY`, etc.
- Ollama: `OLLAMA_ENABLED`, `OLLAMA_BASE_URL`, `OLLAMA_MODEL`

Configuration files in `/configs`: `development.yaml`, `production.yaml`, `multi-provider.yaml`

## Container Runtime Support (Docker/Podman)

SuperAgent supports both Docker and Podman as container runtimes. Use the wrapper script for automatic detection:

```bash
# Source the container runtime script
source scripts/container-runtime.sh

# Use the wrapper commands
./scripts/container-runtime.sh build      # Build container image
./scripts/container-runtime.sh start      # Start services
./scripts/container-runtime.sh stop       # Stop services
./scripts/container-runtime.sh logs       # View logs
./scripts/container-runtime.sh status     # Check service status
```

### Docker Usage

```bash
docker-compose up -d                    # Core services (postgres, redis, cognee, chromadb)
docker-compose --profile ai up -d       # Add AI services (ollama)
docker-compose --profile monitoring up -d  # Add monitoring (prometheus, grafana)
docker-compose --profile full up -d     # Everything
```

### Podman Usage

```bash
# Enable Podman socket for Docker compatibility
systemctl --user enable --now podman.socket

# Use podman-compose (install: pip install podman-compose)
podman-compose up -d                    # Core services
podman-compose --profile ai up -d       # Add AI services
podman-compose --profile full up -d     # Everything

# Or use Podman directly
podman build -t superagent:latest .
podman run -d --name superagent -p 8080:8080 superagent:latest
```

### Container Compatibility Tests

```bash
# Run container runtime compatibility tests
./tests/container/container_runtime_test.sh
```

## Adding a New LLM Provider

1. Create provider package: `internal/llm/providers/<name>/<name>.go`
2. Implement `LLMProvider` interface (Complete, CompleteStream, HealthCheck, GetCapabilities, ValidateConfig)
3. Register in `internal/services/provider_registry.go`
4. Add environment variables to `.env.example`
5. Add tests in `internal/llm/providers/<name>/<name>_test.go`

## Cloud Integration

SuperAgent supports integration with major cloud AI providers:

### AWS Bedrock
- Models: Claude, Titan, Llama, Cohere
- Implements AWS Signature V4 authentication
- Configuration via `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`

### GCP Vertex AI
- Models: PaLM, Gemini
- OAuth2 bearer token authentication
- Configuration via `GCP_PROJECT_ID`, `GCP_LOCATION`, `GOOGLE_ACCESS_TOKEN`

### Azure OpenAI
- Models: GPT-4, GPT-3.5
- API key authentication
- Configuration via `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_API_VERSION`

## LLMsVerifier Integration

The project includes LLMsVerifier for validating LLM provider accuracy:

```bash
make verifier-init        # Initialize the LLMsVerifier submodule
make verifier-build       # Build verifier CLI
make verifier-test        # Run verifier tests
make verifier-run         # Run SuperAgent with verifier enabled
make verifier-verify MODEL=gpt-4 PROVIDER=openai  # Verify a model
```

## Test Coverage Summary

The project maintains comprehensive test coverage across 50+ test packages:

| Package | Coverage | Notes |
|---------|----------|-------|
| internal/testing | 91.9% | Test framework utilities |
| internal/plugins | 71.4% | Plugin system |
| internal/services | 67.5% | Business logic |
| internal/handlers | 55.9% | HTTP handlers |
| internal/cloud | 42.8% | Cloud integrations (requires API credentials) |
| internal/cache | 42.4% | Caching (requires Redis) |
| internal/router | 23.8% | Router (requires database) |

### Test Types
- **Unit tests**: `./internal/...` - Core business logic
- **Integration tests**: `./tests/integration/...` - Service interactions, cloud providers, plugins
- **E2E tests**: `./tests/e2e/...` - Full workflow tests
- **Security tests**: `./tests/security/...` - Authentication, authorization, input validation
- **Stress tests**: `./tests/stress/...` - Load and performance testing
- **Chaos tests**: `./tests/challenge/...` - Resilience testing
