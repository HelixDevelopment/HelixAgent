# Getting Started with HelixAgent

## Quick Start

### 1. Prerequisites

- Go 1.25.3+
- Docker or Podman (for automatic container orchestration)
- At least one LLM provider API key
- 4 GB RAM minimum (8 GB recommended for full stack)

### 2. Installation

```bash
# Clone the repository (SSH only - HTTPS is forbidden)
git clone --recurse-submodules git@github.com:vasic-digital/HelixAgent.git
cd HelixAgent

# Install development tools
make install-deps

# Build the main binary
make build
```

### 3. Configuration

Copy the example environment file and configure your API keys:

```bash
cp .env.example .env
```

Edit `.env` and add at least one provider API key:

```bash
# Required: At least one provider
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
DEEPSEEK_API_KEY=sk-...
GROQ_API_KEY=gsk_...

# Optional: Additional providers (43 total supported)
MISTRAL_API_KEY=...
COHERE_API_KEY=...
PERPLEXITY_API_KEY=...
GEMINI_API_KEY=...
```

### 4. Run HelixAgent

```bash
# Build and run - containers start automatically
make build
./bin/helixagent

# The service will be available at http://localhost:7061
```

**Important:** HelixAgent automatically orchestrates all required containers (PostgreSQL, Redis, ChromaDB, MCP servers, etc.) on startup based on `Containers/.env`. Do **not** start containers manually with `docker-compose` or `podman-compose`. The binary is the sole orchestrator.

## First API Call

Test the installation (wait for startup verification to complete, typically 1-2 minutes):

```bash
# Health check
curl http://localhost:7061/v1/health

# Monitoring status
curl http://localhost:7061/v1/monitoring/status

# List available models
curl http://localhost:7061/v1/models

# Simple completion
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-debate",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Startup verification results
curl http://localhost:7061/v1/startup/verification
```

## Running Tests

**Important:** Infrastructure containers must be running before executing tests. HelixAgent manages these automatically during normal operation, but for isolated test runs use `make test-infra-start` (or `make test-infra-direct-start` for Podman rootless fallback).

```bash
# Unit tests only (fast, no infrastructure needed)
make test-unit

# Start test infrastructure (PostgreSQL:15432, Redis:16379, Mock LLM:18081)
make test-infra-start

# Integration tests (requires running infrastructure)
make test-integration

# All tests with infrastructure
make test-with-infra

# Single test
go test -v -run TestName ./path/to/package

# Stop test infrastructure
make test-infra-stop
```

## Next Steps

- [Configuration Guide](configuration-guide.md) - All environment variables and settings
- [Provider Configuration](PROVIDERS.md) - Configure 43 LLM providers
- [Feature Flags](feature-flags.md) - Toggle features like GraphQL, pprof, etc.
- [Deployment Guide](deployment-guide.md) - Production deployment and remote containers
- [Developer Guide](../DEVELOPER_GUIDE.md) - Contributing and architecture
- [FAQ](../FAQ.md) - Common questions and troubleshooting
