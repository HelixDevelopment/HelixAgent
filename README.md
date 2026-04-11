# HelixAgent: AI-Powered Ensemble LLM Service

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD1E?style=flat-square&logo=go)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue?style=flat-square&logo=docker)](https://www.docker.com)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-Passing-brightgreen?style=flat-square)](https://dev.helix.agent/actions/workflows/tests)

HelixAgent is a production-ready, AI-powered ensemble LLM service that intelligently combines responses from multiple language models to provide the most accurate and reliable outputs.

## 🚀 Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.24+ (for local development)
- Git

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

## 📋 Features

> **Comprehensive Reference**: See **[docs/FEATURES.md](docs/FEATURES.md)** for complete documentation of all 47+ LLM providers, 13 embedding providers, 35 MCP implementations, 10 LSP servers, and 24+ power features. (Authoritative count: `ls internal/llm/providers/ | grep -v common`.)

### 🧠 AI Ensemble System
- **Multi-Provider Support**: 47+ LLM providers including Claude, DeepSeek, Gemini, Mistral, OpenRouter, Qwen, xAI/Grok, Cohere, Perplexity, Groq, and many more — see `internal/llm/providers/` for the full list
- **Dynamic Provider Selection**: Real-time verification scores via LLMsVerifier integration
- **SpecKit Auto-Activation**: Intelligent 7-phase development flow (Constitution → Specify → Clarify → Plan → Tasks → Analyze → Implement) triggered automatically for large changes based on work granularity detection
- **Constitution Watcher**: Auto-update Constitution on project changes (new modules, documentation changes, structure changes, test coverage drops) with background filesystem monitoring

> \* **Note**: Ollama is deprecated for production use (verification score: 5.0) and only serves as a fallback for local development/testing. Recommended production providers: Claude, DeepSeek, Gemini.
- **AI Debate System**: Multi-round debate between providers for consensus (5 positions x 5 LLMs = 25 total)
- **Debate Orchestrator**: Multi-topology (mesh/star/chain), phase protocol (Proposal → Critique → Review → Synthesis), cross-debate learning, auto-fallback to legacy
- **Intelligent Routing**: Confidence-weighted, majority vote, semantic intent detection, custom strategies
- **Graceful Fallbacks**: Automatic fallback to best performing provider based on verification scores with categorized error reporting
- **Streaming Support**: Real-time streaming responses with progress tracking

### 🔧 Production Features
- **High Availability**: PostgreSQL + Redis clustering with automated failover
- **Modular Architecture**: 20 extracted modules (EventBus, Concurrency, Observability, Auth, Storage, Streaming, Security, VectorDB, Embeddings, Database, Cache, Messaging, Formatters, MCP, RAG, Memory, Optimization, Plugins, Containers, Challenges)
- **Monitoring**: Prometheus metrics + Grafana dashboards + OpenTelemetry tracing
- **Security**: JWT authentication, rate limiting, CORS, guardrails engine, PII detection
- **Scalability**: Horizontal scaling, load balancing, distributed memory
- **Caching**: Redis-based response caching + semantic cache (GPTCache-inspired)
- **BigData**: Infinite context, distributed memory, knowledge graph streaming (Neo4j, ClickHouse, Kafka)

### 🛠 Developer Tools
- **Comprehensive Testing**: Unit, integration, E2E, security, stress, chaos, benchmark tests (65.6% coverage)
- **Challenge Framework**: 193+ validation scripts with 1500+ tests
- **Hot Reloading**: Automatic plugin system updates
- **Health Checks**: Comprehensive service health monitoring with circuit breakers
- **API Documentation**: Complete REST API reference + gRPC support
- **48 CLI Agents**: Full agent registry with auto-generated configs
- **32+ Code Formatters**: 11 native, 14 service, 7 built-in formatters for 19 languages
- **45+ MCP Adapters**: Model Context Protocol adapters for extended functionality

## 🏗 Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         HelixAgent                               │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────┐ │
│  │   Web API   │  │  AI Debate   │  │   LLMsVerifier        │ │
│  │    (Gin)    │  │  Orchestrator │  │   (Dynamic Scoring)   │ │
│  └──────┬───────┘  └───────┬──────┘  └──────────┬─────────────┘ │
│         │                  │                     │               │
│         └──────────────────┼─────────────────────┘               │
└───────────────────────────┬┬─────────────────────────────────────┘
                            ││
         ┌──────────────────┼┼──────────────────┐
         ▼                  ▼▼                  ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   PostgreSQL   │    │     Redis      │    │  21 LLM Providers│
│                │    │                │    │  ┌─────────────┐ │
│   - Sessions   │    │   - Caching   │    │  │Claude│DeepSeek│
│   - Analytics  │    │   - Queues    │    │  │Gemini│Mistral │
│   - Debates    │    │   - Tasks     │    │  │Qwen  │xAI/Grok│
└─────────────────┘    └─────────────────┘    │  │Zen  │Cerebras│
                                              │  │OpenRouter   │
                                              │  │+ 13 more    │
                                              │  └─────────────┘ │
                                              └─────────────────┘
```

## 📊 Monitoring Stack

### Grafana Dashboard
- **URL**: http://localhost:3000
- **Credentials**: admin/admin123
- **Features**: 
  - Response time metrics
  - Error rate monitoring
  - Provider performance comparison
  - Request throughput tracking

### Prometheus Metrics
- **URL**: http://localhost:9090
- **Metrics Available**:
  - `helixagent_requests_total`
  - `helixagent_response_time_seconds`
  - `helixagent_errors_total`
  - `helixagent_provider_health`

## 🔌 Configuration

### Environment Variables
HelixAgent uses comprehensive environment-based configuration. Key variables:

```bash
# Server Configuration
PORT=8080
HELIXAGENT_API_KEY=your-api-key
GIN_MODE=release

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=helixagent
DB_PASSWORD=your-password
DB_NAME=helixagent_db

# LLM Providers (Ollama is deprecated - use as fallback only)
# See docs/providers/ollama.md for deprecation notice
OLLAMA_ENABLED=true
OLLAMA_BASE_URL=http://ollama:11434
OLLAMA_MODEL=llama2

# Recommended Production Providers
CLAUDE_API_KEY=sk-your-claude-key
DEEPSEEK_API_KEY=sk-your-deepseek-key
GEMINI_API_KEY=your-gemini-key
```

### Free Testing with Ollama (Development Only)
> ⚠️ **Ollama is deprecated for production** - use it only for local development and testing. For production, use API key-based providers like Claude, DeepSeek, or Gemini.

```bash
# Ollama requires no API keys and works locally
docker run -p 11434:11434 ollama/ollama

# Pull a model (first time only)
docker exec -it ollama ollama pull llama2

# Test the model
curl -X POST http://localhost:11434/api/generate \
  -H "Content-Type: application/json" \
  -d '{"model": "llama2", "prompt": "Hello!"}'
```

## 🔧 Development

### Building
```bash
# Standard build
make build

# Multi-architecture build
make build-all

# Production build
make docker-build-prod
```

### Testing
```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific test suites
make test-unit
make test-integration

# Run benchmarks
make test-bench
```

### Code Quality
```bash
# Format code
make fmt

# Run static analysis
make vet
make lint

# Security scanning
make security-scan
```

## 🐳 Docker Deployment

### Production Profiles
```bash
# Full stack (recommended)
make docker-full

# AI services only
make docker-ai

# Monitoring stack only
make docker-monitoring

# Custom configuration
docker-compose --profile custom up -d
```

### Container Health
All containers include comprehensive health checks:
- **Application**: `/health` endpoint monitoring
- **Database**: PostgreSQL connection validation
- **Cache**: Redis ping verification
- **LLM Providers**: API endpoint health monitoring

## 🌐 Remote Deployment & Service Discovery

HelixAgent supports **distributed deployment** across multiple hosts with **automatic service discovery**. Deploy services to remote machines, discover existing services in your network, and manage hybrid local/remote environments.

### Service Discovery
```bash
# Discover services in your network
./scripts/discover-services.sh

# Discover with specific strategy (TCP, HTTP, DNS, mDNS)
./scripts/discover-services.sh --strategy=tcp
./scripts/discover-services.sh --strategy=http

# Test discovery methods
./scripts/discover-services.sh --test
```

### Remote Deployment to Single Host
```bash
# Deploy services to a remote host
./scripts/deploy-remote.sh thinker.local

# Deploy specific services
./scripts/deploy-remote.sh thinker.local --services=postgresql,redis

# Deploy with custom compose file
./scripts/deploy-remote.sh thinker.local --compose-file=docker-compose.prod.yml
```

### Parallel Multi-Host Deployment
```bash
# Deploy to multiple hosts in parallel
./scripts/deploy-all-remote.sh

# Custom host configuration
./scripts/deploy-all-remote.sh --config=configs/remote-hosts.yaml
```

### Configuration
Create `configs/remote-hosts.yaml`:
```yaml
remote_deployment:
  enabled: true
  hosts:
    thinker.local:
      services:
        - postgresql
        - redis
        - mem0
    raspberrypi.local:
      services:
        - ollama
        - zen
```

### BootManager Integration
```go
// Deploy services to remote hosts
deployer := services.NewSSHRemoteDeployer()
err := deployer.DeployServices(ctx, config)

// Health check remote services
status := bootManager.HealthCheckRemoteServices(ctx)
```

### Hybrid Environments
- **Local services**: PostgreSQL, Redis, ChromaDB
- **Remote services**: LLM providers, Mem0 (memory), monitoring
- **Discovered services**: Auto-detected in network

**Full Documentation**: [docs/REMOTE_DEPLOYMENT.md](docs/REMOTE_DEPLOYMENT.md)

## 📚 API Documentation

**Full API Reference**: [docs/api/API_REFERENCE.md](docs/api/API_REFERENCE.md) - Complete REST API documentation with examples

**Capability Detection**: [LLMsVerifier/docs/CAPABILITY_DETECTION.md](LLMsVerifier/docs/CAPABILITY_DETECTION.md) - Dynamic capability detection for 18+ CLI agents

### Endpoints

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

### Request Examples

#### Basic Completion
```bash
curl -X POST http://localhost:7061/v1/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "prompt": "Explain quantum computing in simple terms",
    "model": "llama2",
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

#### Streaming Request
```bash
curl -X POST http://localhost:7061/v1/completions/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "prompt": "Tell me a story",
    "stream": true,
    "model": "llama2"
  }'
```

## 🔌 Security Features

### Authentication
- **JWT Tokens**: Secure session management
- **API Key Authentication**: Request validation
- **Rate Limiting**: Configurable per-user limits
- **CORS Support**: Cross-origin request handling

### Data Protection
- **Input Sanitization**: Request validation and sanitization
- **Error Handling**: Secure error responses without information leakage
- **Logging**: Structured logging with security events
- **Environment Variables**: No hardcoded secrets

## 🔌 Plugin System

### Architecture
- **Hot Reloading**: Automatic plugin detection and loading
- **Interface-Based**: Standardized plugin interfaces
- **Configuration**: Plugin-specific config management
- **Health Monitoring**: Plugin health status tracking

### Example Plugin
```go
package main

import (
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

## 🚀 Performance

### Benchmarks
- **Request Throughput**: 1000+ requests/second
- **Response Time**: <500ms for cached responses
- **Memory Usage**: <512MB for typical workloads
- **CPU Usage**: <50% on 4-core instances

### Optimization Features
- **Connection Pooling**: Database connection reuse
- **Response Caching**: Redis-based intelligent caching
- **Async Processing**: Non-blocking I/O operations
- **Resource Limits**: Configurable timeouts and pool sizes

## 🔬 LLM Optimization Framework

HelixAgent includes a comprehensive LLM optimization framework for improving performance:

### Native Go Optimizations
- **Semantic Cache**: Vector similarity-based caching (GPTCache-inspired)
- **Structured Output**: JSON schema validation and generation (Outlines-inspired)
- **Enhanced Streaming**: Word/sentence buffering, progress tracking, rate limiting

### External Service Integrations
- **SGLang**: RadixAttention prefix caching for multi-turn conversations
- **LlamaIndex**: Advanced document retrieval with Mem0 memory sync
- **LangChain**: Task decomposition and ReAct agents
- **Guidance**: CFG/regex constrained generation
- **LMQL**: Query language for LLM constraints

### Quick Start with Optimization Services
```bash
# Start optimization services
docker-compose --profile optimization up -d

# Services available:
# - langchain-server (port 8011)
# - llamaindex-server (port 8012)
# - guidance-server (port 8013)
# - lmql-server (port 8014)
# - sglang (port 30000, requires GPU)
```

### Usage Example
```go
import "dev.helix.agent/internal/optimization"

// Create and use optimization service
config := optimization.DefaultConfig()
svc, _ := optimization.NewService(config)

// Check cache, retrieve context, decompose complex tasks
optimized, _ := svc.OptimizeRequest(ctx, prompt, embedding)
```

## 🧪 Testing Strategy

### Test Coverage
- **Unit Tests**: 95%+ coverage for core logic
- **Integration Tests**: End-to-end API testing
- **Security Tests**: LLM penetration testing (prompt injection, jailbreaking, data exfiltration)
- **Challenge Tests**: AI debate maximal challenge validation
- **Benchmark Tests**: Performance regression detection
- **Race Tests**: Concurrency safety validation
- **Chaos Tests**: Resilience and fault tolerance testing

### Test Categories
```bash
make test                  # Run all tests (auto-detects infrastructure)
make test-unit             # Unit tests only (./internal/... -short)
make test-integration      # Integration tests (./tests/integration)
make test-e2e              # End-to-end tests (./tests/e2e)
make test-security         # Security tests (./tests/security)
make test-stress           # Stress tests (./tests/stress)
make test-chaos            # Chaos/challenge tests (./tests/challenge)
make test-bench            # Benchmark tests
make test-race             # Race condition detection
```

### Test Environments
- **Mock Providers**: Isolated unit testing with HTTP mock servers
- **Docker Compose**: Full integration testing
- **Free LLM Testing**: Ollama/Zen-based testing without API keys
- **CI/CD Pipeline**: Automated testing on every push

### Security Testing
The security test suite validates LLM security including:
- **Prompt Injection**: System prompt extraction, role manipulation
- **Jailbreaking**: Multi-language attacks, hypothetical scenarios
- **Data Exfiltration**: PII extraction, credential probing
- **Indirect Injection**: Markdown/HTML injection, encoded payloads

## 📈 Monitoring & Observability

### Metrics Collection
```go
// Request metrics
requestCounter := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "helixagent_requests_total",
        Help: "Total number of requests processed",
    },
    []string{"method", "endpoint", "provider"},
)

// Response time metrics
responseTime := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "helixagent_response_time_seconds",
        Help: "Request response time in seconds",
    },
    []string{"method", "endpoint"},
)
```

### Health Status
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T00:00:00Z",
  "version": "1.0.0",
  "uptime": "72h30m15s",
  "providers": {
    "total": 6,
    "healthy": 4,
    "unhealthy": 2,
    "details": {
      "ollama": {"status": "healthy", "response_time": 150},
      "claude": {"status": "unhealthy", "error": "authentication_failed"},
      "deepseek": {"status": "healthy", "response_time": 300}
    }
  },
  "database": {"status": "healthy", "connections": 15/20},
  "cache": {"status": "healthy", "hit_rate": 0.85}
}
```

## 🔄 CI/CD Pipeline

### GitHub Actions
- **Multi-Architecture Builds**: Linux (amd64/arm64), macOS, Windows
- **Docker Image Building**: Automated image creation and publishing
- **Security Scanning**: CodeQL and dependency scanning
- **Test Execution**: Unit, integration, and end-to-end tests
- **Release Automation**: Semantic versioning and release notes

### Deployment Targets
- **Docker Hub**: Production image repository
- **Kubernetes**: Production K8s manifests
- **Cloud Providers**: AWS, GCP, Azure deployment guides
- **Self-Hosted**: On-premise deployment documentation

## 🛠 Troubleshooting

### Common Issues

#### Provider Authentication
```bash
# Check provider configuration
curl http://localhost:7061/v1/providers

# Test provider health
curl http://localhost:7061/v1/providers/ollama/health

# View logs
docker-compose logs helixagent
```

#### Database Connection
```bash
# Check database connectivity
docker-compose exec postgres pg_isready -U helixagent -d helixagent_db

# Test from application container
docker-compose exec helixagent ./helixagent check-db
```

#### Performance Issues
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

# Enable detailed error responses
export DEBUG_ENABLED=true
export REQUEST_LOGGING=true
```

## 📚 Additional Resources

### Documentation
- **[Full Documentation](./docs/README.md)**: Complete documentation index
- **[Features Reference](./docs/FEATURES.md)**: Comprehensive list of all providers, protocols, and features
- **API Reference**: http://localhost:7061/docs
- **[Architecture Guide](./docs/architecture/architecture.md)**: System architecture
- **[Deployment Guide](./docs/deployment/DEPLOYMENT_GUIDE.md)**: Production deployment
- **[Quick Start](./docs/guides/quick-start-guide.md)**: Getting started guide
- **[NVIDIA Nemotron RAG](./docs/NVIDIA_NEMOTRON_RAG_INTEGRATION.md)**: Document processing with Nemotron models

### 🔬 Research Documentation (328KB)
Comprehensive analysis of 47 CLI agents and protocol specifications:
- **[Research Master Index](./docs/research/MASTER_INDEX.md)**: Navigation guide for all research docs
- **[Comparative Analysis](./docs/research/against_cli_agents/)**: 47 CLI agents vs HelixAgent comparison
  - Tier 1 detailed comparisons (Claude Code, Aider, Codex, Cline, OpenHands, Kiro)
  - Feature matrices and strategic analysis
- **[API Documentation](./docs/research/api_documentation/)**: Complete API specs with cross-references
  - REST APIs, WebSocket protocols, authentication
  - Provider compatibility matrices
- **[Protocol Documentation](./docs/research/protocol_documentation/)**: Full protocol specifications
  - MCP (Model Context Protocol)
  - ACP (Agent Communication Protocol)
  - LSP (Language Server Protocol)
  - Embeddings and RAG protocols

### Community
- **GitHub Discussions**: [Community Support](https://dev.helix.agent/discussions)
- **Issues**: [Bug Reports & Feature Requests](https://dev.helix.agent/issues)
- **Contributing**: [Contribution Guidelines](CONTRIBUTING.md)

### Support
- **Documentation**: [HelixAgent Docs](https://docs.helixagent.ai)
- **Website**: [HelixAgent.ai](https://helixagent.ai)
- **Email**: [support@helixagent.ai](mailto:support@helixagent.ai)

---

## 🎯 Getting Help

### Quick Commands
```bash
# Show all available commands
make help

# Setup development environment
make setup-dev

# Start with monitoring
make docker-full

# View logs
make docker-logs

# Stop all services
make docker-stop
```

### License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**HelixAgent** - Intelligent ensemble LLM service for production workloads. 🚀

**Last Updated**: February 10, 2026
**Version**: 1.0.0
**Documentation Status**: ✅ Complete and Synchronized