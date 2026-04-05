# HelixAgent: AI-Powered Ensemble LLM Service

## Project Overview

HelixAgent is a production-ready, AI-powered ensemble LLM orchestration platform that intelligently combines responses from multiple language models to provide the most accurate and reliable outputs. Built in Go, it provides a comprehensive platform for managing multiple LLM providers with advanced features including AI debate systems, Cognee knowledge graph integration, multi-protocol support (MCP, LSP, ACP), and enterprise-grade monitoring.

### Key Technologies
- **Backend**: Go 1.24+ with Gin web framework
- **Database**: PostgreSQL 15 with pgx driver
- **Cache**: Redis 7 for high-performance caching
- **Containerization**: Docker and Docker Compose (also supports Podman)
- **Monitoring**: Prometheus metrics + Grafana dashboards + OpenTelemetry tracing
- **Knowledge Graph**: Cognee for advanced AI memory
- **API Gateway**: OpenAI-compatible REST API + gRPC support

### Architecture
The system follows a microservices architecture with:
- Main HelixAgent service handling API requests (entry point: `cmd/helixagent/main.go`)
- PostgreSQL database for persistent storage
- Redis for caching and session management
- Cognee knowledge graph for advanced AI memory
- 21 LLM providers (Claude, DeepSeek, Gemini, Mistral, OpenRouter, Qwen, xAI/Grok, Cohere, Perplexity, Groq, and more)
- 13 embedding providers with 40+ models
- 35 MCP implementations (19 adapters + 16 servers)
- 10 LSP language servers
- Monitoring stack with Prometheus and Grafana

## Building and Running

### Prerequisites
- Docker & Docker Compose (or Podman)
- Go 1.24+ (for local development)
- Git
- Make (build automation)

### Using Docker (Recommended)
```bash
# Clone the repository
git clone https://dev.helix.agent.git
cd helixagent

# Copy environment configuration
cp .env.example .env

# Start all services
make docker-full

# Or start specific profiles
make docker-ai          # AI services only
make docker-monitoring   # Monitoring stack only
```

### Local Development
```bash
# Install dependencies
make install-deps

# Setup development environment
make setup-dev

# Run locally
make run-dev
```

### Build Commands
```bash
# Standard build
make build

# Debug build
make build-debug

# Multi-architecture build
make build-all

# Run locally
make run

# Run in development mode
make run-dev
```

## Development Conventions

### Code Quality
- **Code formatting**: `make fmt` (uses go fmt)
- **Static analysis**: `make vet` and `make lint`
- **Security scanning**: `make security-scan` (includes gosec, Snyk, SonarQube, Trivy)

### Testing Strategy
The project has comprehensive test coverage with 6 test types:
- **Unit tests**: `make test-unit` - Core logic testing (./internal/... -short)
- **Integration tests**: `make test-integration` - End-to-end API testing
- **E2E tests**: `make test-e2e` - Full system testing
- **Security tests**: `make test-security` - LLM penetration testing
- **Stress tests**: `make test-stress` - Load and performance testing
- **Challenge tests**: `make test-chaos` - AI debate maximal challenge validation

```bash
# Run all tests (auto-detects infrastructure)
make test

# Run with coverage
make test-coverage

# Run with full infrastructure
make test-with-infra

# Run complete test suite (all 6 types)
make test-complete

# Benchmark tests
make test-bench

# Race condition tests
make test-race
```

### Test Infrastructure
```bash
# Start test infrastructure (PostgreSQL, Redis, Mock LLM)
make test-infra-start

# Start full infrastructure (includes Kafka, RabbitMQ, MinIO, Iceberg, Qdrant)
make test-infra-full-start

# Stop infrastructure
make test-infra-stop

# View infrastructure logs
make test-infra-logs
```

### Project Structure
```
HelixAgent/
├── cmd/                    # Application entry points
│   ├── helixagent/         # Main production server
│   ├── api/                # Demo API server (mock responses)
│   ├── grpc-server/        # gRPC API server
│   ├── cognee-mock/        # Cognee mock service
│   ├── mcp-bridge/         # MCP bridge service
│   └── sanity-check/       # Sanity check utility
├── internal/               # Private application code (50+ modules)
│   ├── llm/                # LLM provider implementations (21 providers)
│   ├── services/           # Business logic
│   ├── handlers/           # HTTP handlers
│   ├── config/             # Configuration management
│   ├── router/             # HTTP routing and middleware
│   ├── middleware/         # Request middleware
│   ├── models/             # Data models
│   ├── database/           # Database operations
│   ├── cache/              # Caching mechanisms
│   ├── auth/               # Authentication and authorization
│   ├── security/           # Security framework
│   ├── mcp/                # Model Context Protocol
│   ├── lsp/                # Language Server Protocol
│   ├── embedding/          # Embedding providers
│   ├── vectordb/           # Vector database integrations
│   ├── rag/                # Retrieval Augmented Generation
│   ├── memory/             # Memory management
│   ├── optimization/       # LLM optimization
│   ├── observability/      # Metrics and tracing
│   ├── messaging/          # Message queues (Kafka, RabbitMQ)
│   ├── streaming/          # Streaming support
│   ├── plugins/            # Plugin system
│   └── ...                 # 30+ more modules
├── pkg/                    # Public packages
├── tests/                  # Test suites
│   ├── unit/               # Unit tests
│   ├── integration/        # Integration tests
│   ├── e2e/                # End-to-end tests
│   ├── security/           # Security tests
│   ├── challenge/          # Challenge tests
│   ├── stress/             # Stress tests
│   └── performance/        # Performance benchmarks
├── docs/                   # Documentation
├── scripts/                # Build and deployment scripts
├── docker/                 # Docker configurations
├── configs/                # Configuration files
└── deployments/            # Deployment configurations
```

### Configuration
The application uses comprehensive environment-based configuration. Key variables:

```bash
# Server Configuration
PORT=7061
HELIXAGENT_API_KEY=your-api-key
GIN_MODE=release

# JWT Configuration
JWT_SECRET=your-jwt-secret
TOKEN_EXPIRY=24h

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=helixagent
DB_PASSWORD=your-password
DB_NAME=helixagent_db
DB_SSLMODE=disable

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your-password

# LLM Provider API Keys
CLAUDE_API_KEY=sk-your-claude-key
DEEPSEEK_API_KEY=sk-your-deepseek-key
GEMINI_API_KEY=your-gemini-key
QWEN_API_KEY=your-qwen-key
ZAI_API_KEY=your-zai-key

# Ollama Configuration (DEPRECATED - use as fallback only)
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama2
OLLAMA_ENABLED=false

# Cognee Configuration
COGNEE_BASE_URL=http://localhost:8000
COGNEE_API_KEY=your-cognee-key

# Rate Limiting
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=1m
```

### API Endpoints
The service provides an OpenAI-compatible API with the following key endpoints:

#### Core API
- `GET /health` - Service health status
- `GET /v1/health` - Detailed health with provider status
- `GET /v1/models` - Available LLM models
- `GET /v1/providers` - Configured providers
- `GET /metrics` - Prometheus metrics

#### Completions
- `POST /v1/completions` - Single completion request
- `POST /v1/chat/completions` - Chat-style completions
- `POST /v1/completions/stream` - Streaming completions
- `POST /v1/ensemble/completions` - Ensemble completions

#### Protocol Endpoints
- `POST /v1/mcp/tools` - MCP tool execution
- `POST /v1/lsp/completion` - LSP code completion
- `POST /v1/embeddings` - Embedding generation

### Request Examples

#### Basic Completion
```bash
curl -X POST http://localhost:7061/v1/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "prompt": "Explain quantum computing in simple terms",
    "model": "gemini-2.0-flash",
    "max_tokens": 500,
    "temperature": 0.7
  }'
```

#### Ensemble Request
```bash
curl -X POST http://localhost:7061/v1/ensemble/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "prompt": "What is the meaning of life?",
    "ensemble_config": {
      "strategy": "confidence_weighted",
      "min_providers": 2,
      "confidence_threshold": 0.8,
      "fallback_to_best": true
    }
  }'
```

### Security Features
- **JWT token authentication**: Secure session management
- **API key authentication**: Request validation
- **Rate limiting**: Configurable per-user limits
- **CORS support**: Cross-origin request handling
- **Input sanitization**: Request validation and sanitization
- **Secure error handling**: No information leakage
- **Security scanning**: Automated security analysis (gosec, Snyk, SonarQube, Trivy)

### Monitoring and Observability
- **Prometheus metrics**: Available at `/metrics` endpoint
- **Grafana dashboards**: Access at http://localhost:3000 (admin/admin123)
- **OpenTelemetry tracing**: Distributed tracing support
- **Comprehensive health checks**: Service and provider health monitoring
- **Structured logging**: Configurable log levels with logrus/zap

### Plugin System
The application supports a hot-reloading plugin system with standardized interfaces:

```go
package main

import (
    "context"
    "dev.helix.agent/internal/plugins"
)

type MyPlugin struct {
    name string
}

func (p *MyPlugin) Name() string { return p.name }
func (p *MyPlugin) Version() string { return "1.0.0" }
func (p *MyPlugin) Init(config map[string]any) error { /* init logic */ }
func (p *MyPlugin) HealthCheck(ctx context.Context) error { /* health check */ }
func (p *MyPlugin) Shutdown(ctx context.Context) error { /* cleanup */ }
```

### Key Features

#### AI Ensemble System
- **Multi-Provider Support**: 21 LLM providers with automatic discovery
- **Dynamic Provider Selection**: Real-time verification scores via LLMsVerifier
- **SpecKit Auto-Activation**: Intelligent 7-phase development flow
- **Constitution Watcher**: Auto-update Constitution on project changes
- **AI Debate System**: Multi-round debate (5 positions x 5 LLMs = 25 total)
- **Debate Orchestrator**: Multi-topology (mesh/star/chain), phase protocol
- **Intelligent Routing**: Confidence-weighted, majority vote, custom strategies
- **Graceful Fallbacks**: Automatic fallback to best performing provider

#### Production Features
- **High Availability**: PostgreSQL + Redis with automated failover
- **Modular Architecture**: 20 extracted modules
- **Scalability**: Horizontal scaling, load balancing, distributed memory
- **Caching**: Redis-based response caching + semantic cache

#### Developer Tools
- **Challenge Framework**: 193+ validation scripts with 1500+ tests
- **Hot Reloading**: Automatic plugin system updates
- **Health Checks**: Comprehensive service health monitoring
- **48 CLI Agents**: Full agent registry with auto-generated configs
- **32+ Code Formatters**: For 19 languages
- **45+ MCP Adapters**: Model Context Protocol adapters

### Troubleshooting

#### Common Issues

**Provider Authentication**
```bash
# Check provider configuration
curl http://localhost:7061/v1/providers

# Test provider health
curl http://localhost:7061/v1/providers/ollama/health

# View logs
docker-compose logs helixagent
```

**Database Connection**
```bash
# Check database connectivity
docker-compose exec postgres pg_isready -U helixagent -d helixagent_db

# Test from application container
docker-compose exec helixagent ./helixagent check-db
```

**Performance Issues**
```bash
# Monitor response times
curl -w "@{time_total}\n" -o /dev/null -s http://localhost:7061/health

# Check resource usage
docker stats helixagent

# View metrics
curl http://localhost:9090/metrics
```

### Debug Mode
```bash
# Enable debug logging
export LOG_LEVEL=debug
export GIN_MODE=debug
make run-dev
```

## Mandatory Constraints

### HTTP/3 (QUIC) with Brotli Compression (MANDATORY)
- **ALL HTTP communication MUST use HTTP/3 (QUIC) as the primary transport protocol** with Brotli compression
- **HTTP/2 is ONLY allowed as a fallback** when HTTP/3 is not supported by the remote endpoint
- **Compression priority**: Brotli (primary) → gzip (fallback only when Brotli is unavailable)
- **All HTTP clients and servers** MUST prefer HTTP/3 and advertise it via `Alt-Svc` headers
- **Go implementation**: Use `quic-go/quic-go` for HTTP/3 transport and `andybalholm/brotli` for compression

### Resource Limits for Tests & Challenges (CRITICAL)
- **ALL test and challenge execution MUST be strictly limited to 30-40% of host system resources**
- **CPU limiting**: Use `GOMAXPROCS=2`, `nice -n 19` for all test processes
- **I/O limiting**: Use `ionice -c 3` for all test and challenge processes
- **Sequential execution**: Use `-p 1` flag for `go test`
- **Rationale**: The host machine runs mission-critical processes; exceeding limits has caused system crashes

### Centralized Container Management (MANDATORY)
- **ALL container operations** (runtime detection, compose up/down, health checks, remote distribution) MUST go through the Containers module (`digital.vasic.containers`) via `internal/adapters/containers/adapter.go`
- **No direct `exec.Command`** to `docker`/`podman` in production code — use adapter methods instead
- **Adapter methods**: `DetectRuntime`, `ComposeUp`, `ComposeDown`, `HealthCheck`, `HealthCheckHTTP`, `HealthCheckTCP`, `Distribute`, `Shutdown`
- **Remote distribution**: Enabled via `CONTAINERS_REMOTE_*` env vars with SSH-based execution, resource-aware scheduling, SSH tunnels, and volume management

## Additional Resources

### Documentation
- **[Full Documentation](./docs/README.md)**: Complete documentation index
- **[Features Reference](./docs/FEATURES.md)**: All providers, protocols, and capabilities
- **[API Reference](./docs/api/API_REFERENCE.md)**: Complete REST API documentation
- **[Architecture Guide](./docs/architecture/architecture.md)**: System architecture
- **[Deployment Guide](./docs/deployment/DEPLOYMENT_GUIDE.md)**: Production deployment
- **[Contributing Guide](./CONTRIBUTING.md)**: Contribution guidelines

### Support
- **GitHub Issues**: Bug reports and feature requests
- **Documentation**: https://docs.helixagent.ai
- **Website**: https://helixagent.ai

---

**Last Updated**: February 17, 2026
**Version**: 1.0.0
**Go Version**: 1.24+
