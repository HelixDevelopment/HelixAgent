# HelixLLM Integration Documentation

## Overview

HelixLLM is a fully integrated submodule within the HelixAgent ecosystem, providing enterprise-grade distributed LLM capabilities. This document describes the comprehensive integration of HelixLLM with HelixAgent.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           HelixAgent                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     HelixLLM Integration                             │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐  │   │
│  │  │   Gateway   │  │    Brain    │  │  Knowledge  │  │  Agents   │  │   │
│  │  │   Layer     │  │    Layer    │  │   Layer     │  │  Layer    │  │   │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └─────┬─────┘  │   │
│  │         │                │                │               │        │   │
│  │         └────────────────┴────────────────┴───────────────┘        │   │
│  │                              │                                      │   │
│  │                    ┌─────────┴──────────┐                          │   │
│  │                    │   Shared Foundation  │                         │   │
│  │                    │   (Config, Events,   │                         │   │
│  │                    │    Health, Logging)  │                         │   │
│  │                    └─────────────────────┘                          │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
         ┌────────────────────────────┼────────────────────────────┐
         ▼                            ▼                            ▼
┌─────────────────┐      ┌─────────────────────┐      ┌─────────────────────┐
│  MCP Servers    │      │   LSP Adapters      │      │   ACP Protocol      │
│  (45+ adapters) │      │   (32+ languages)   │      │   (Multi-agent)     │
└─────────────────┘      └─────────────────────┘      └─────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                         HelixLLM Infrastructure                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐  │
│  │PostgreSQL│  │  Redis   │  │ Qdrant   │  │  Kafka   │  │  Prometheus  │  │
│  │  (DB)    │  │ (Cache)  │  │(VectorDB)│  │(Messaging│  │ (Monitoring) │  │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Features

### Core Capabilities

- **OpenAI-compatible API** - Drop-in replacement for OpenAI API
- **Anthropic-compatible API** - Native support for Anthropic format
- **HTTP/3 (QUIC) Server** - High-performance HTTP/3 with HTTP/2 fallback
- **Local LLM Inference** - llama.cpp integration for on-premise deployment
- **RAG Pipeline** - Document ingestion, chunking, embedding, and retrieval
- **ReAct Agent System** - Tool calling with conversation context
- **Multi-mode Deployment** - Full, Gateway, Brain, Knowledge, Agents, Control

### Integration Features

| Feature | Module | Status | Description |
|---------|--------|--------|-------------|
| MCP | MCP_Module | ✅ | 45+ MCP server adapters |
| LSP | LSP | ✅ | 32+ language formatters |
| ACP | ACP | ✅ | Agent Communication Protocol |
| Embeddings | Embeddings | ✅ | Multi-provider embedding generation |
| RAG | RAG | ✅ | Retrieval-Augmented Generation |
| Memory | Memory | ✅ | Session and persistent memory |
| VectorDB | VectorDB | ✅ | Qdrant, pgvector, Milvus support |

## Configuration

### Environment Variables

```bash
# Enable HelixLLM integration
USE_HELIX_LLM=true

# Connection Settings
HELIX_LLM_ENDPOINT=https://localhost:8443
HELIX_LLM_API_KEY=
HELIX_LLM_TLS_SKIP_VERIFY=true

# Mode Configuration
HELIX_LLM_MODE=full
# Options: full | gateway | brain | knowledge | agents | control

# Database Configuration
HELIX_LLM_DB_HOST=helixllm-postgres
HELIX_LLM_DB_PORT=5432
HELIX_LLM_DB_NAME=helixllm
HELIX_LLM_DB_USER=helix
HELIX_LLM_DB_PASSWORD=helixllm

# Redis Configuration
HELIX_LLM_REDIS_HOST=helixllm-redis
HELIX_LLM_REDIS_PORT=6379
HELIX_LLM_REDIS_PASSWORD=helixllm123

# Feature Integrations
HELIX_LLM_USE_HELIXAGENT_MCP=true
HELIX_LLM_USE_HELIXAGENT_LSP=true
HELIX_LLM_USE_HELIXAGENT_ACP=true
HELIX_LLM_USE_HELIXAGENT_EMBEDDINGS=true
HELIX_LLM_USE_HELIXAGENT_RAG=true
HELIX_LLM_USE_HELIXAGENT_MEMORY=true
```

## Container Orchestration

HelixLLM uses the Containers submodule for all container operations:

```go
// Using the Containers adapter
import containeradapter "dev.helix.agent/internal/adapters/containers"

// Create adapter
containers := containeradapter.NewAdapterFromConfig(cfg)

// Start HelixLLM
err := containers.ComposeUp(ctx, "docker-compose.helixllm.yml", "")

// Check status
status, err := containers.ComposeStatus(ctx, "docker-compose.helixllm.yml")
```

## API Endpoints

### OpenAI-Compatible Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/chat/completions` | Chat completions (streaming supported) |
| POST | `/v1/completions` | Text completions |
| GET | `/v1/models` | List available models |
| POST | `/v1/embeddings` | Generate embeddings |

### Knowledge Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/internal/knowledge/ingest` | Ingest documents |
| POST | `/internal/knowledge/query` | Query knowledge base |
| GET | `/internal/knowledge/stats` | Knowledge base statistics |

### Agent Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/agents/chat` | Run agent with ReAct loop |
| GET | `/v1/agents/tools` | List available tools |

## Testing

### LLMsVerifier Integration

```bash
# Run comprehensive LLMsVerifier validation
./tests/helixllm/test_helixllm_integration.sh
```

This script performs:
- Submodule verification
- Infrastructure startup
- LLMsVerifier validation and scoring
- Performance benchmarks
- Unit and integration tests
- Comprehensive report generation

### HelixQA Test Bank

```bash
# Run HelixQA test bank
./helixqa run --banks ./HelixQA/banks/helixllm.yaml --platform api
```

### Challenge Script

```bash
# Run integration challenge
./challenges/scripts/helixllm_integration_challenge.sh
```

## Provider Integration

HelixLLM is registered as a first-class LLM provider:

```go
// Register HelixLLM provider
import "dev.helix.agent/internal/llm/providers/helixllm"

provider := helixllm.NewProvider(helixllm.Config{
    Endpoint: "https://localhost:8443",
    Model:    "helixllm-default",
})

// Use like any other provider
response, err := provider.Complete(ctx, &models.LLMRequest{
    Messages: []models.Message{
        {Role: "user", Content: "Hello!"},
    },
})
```

## Deployment Modes

### Full Mode (Default)
```bash
HELIX_MODE=full
```
All layers run in a single process. Best for development and single-host production.

### Gateway Mode
```bash
HELIX_MODE=gateway
```
API surface only. Routes requests to other layers.

### Brain Mode
```bash
HELIX_MODE=brain
```
LLM inference and routing only.

### Knowledge Mode
```bash
HELIX_MODE=knowledge
```
RAG pipeline only.

### Agents Mode
```bash
HELIX_MODE=agents
```
Agent workers only.

### Control Mode
```bash
HELIX_MODE=control
```
Cluster management only.

## Monitoring

HelixLLM exports Prometheus metrics:

- `helixllm_requests_total` - Total requests
- `helixllm_request_duration_seconds` - Request latency
- `helixllm_tokens_total` - Token usage
- `helixllm_active_connections` - Active connections

Access Grafana at `http://localhost:3001` (default credentials: admin/admin)

## Troubleshooting

### Check Service Health
```bash
curl -k https://localhost:8443/internal/health
```

### View Container Logs
```bash
docker logs helixagent-helixllm
docker logs helixagent-helixllm-postgres
docker logs helixagent-helixllm-redis
```

### Verify Submodule
```bash
git submodule update --init --recursive HelixLLM
```

## Documentation

- **HelixLLM Manual:** [HelixLLM/docs/manual/architecture.md](../HelixLLM/docs/manual/architecture.md)
- **API Reference:** [HelixLLM/docs/user-guide/api-reference.md](../HelixLLM/docs/user-guide/api-reference.md)
- **Configuration:** [HelixLLM/docs/user-guide/configuration.md](../HelixLLM/docs/user-guide/configuration.md)

## References

- HelixLLM Repository: `https://github.com/HelixDevelopment/HelixLLM`
- Containers Module: `digital.vasic.containers`
- MCP Module: `digital.vasic.mcp`
- Embeddings Module: `digital.vasic.embeddings`
- RAG Module: `digital.vasic.rag`

---

*Last Updated: April 2026*
