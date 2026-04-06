# HelixAgent Deployment Guide

**Version:** 1.3.0  
**Last Updated:** 2026-04-06

## Prerequisites

- Go 1.25.3+
- Docker or Podman (auto-detected)
- At least one LLM provider API key
- 4 GB RAM minimum (8 GB recommended)

## Quick Start

### 1. Install Dependencies

```bash
make install-deps
```

### 2. Configure Environment

```bash
cp .env.example .env
# Edit .env with your provider API keys and settings
```

### 3. Build and Run

```bash
make build
./bin/helixagent
```

**Important:** HelixAgent automatically orchestrates all required containers (PostgreSQL, Redis, ChromaDB, MCP servers, etc.) on startup. Do **not** start containers manually. The binary reads `Containers/.env` and manages the entire container lifecycle.

## Container Orchestration

HelixAgent uses the Containers module (`digital.vasic.containers`) for all container management. The orchestration flow is:

1. HelixAgent boots and initializes the Containers module adapter
2. Adapter reads `Containers/.env` (not the project root `.env`)
3. Based on configuration, containers start locally or are distributed to remote hosts
4. Health checks run against all configured endpoints
5. Required services failing health check cause boot failure in strict mode

### Local Deployment (Default)

When `CONTAINERS_REMOTE_ENABLED=false` (or not set) in `Containers/.env`, all containers start on the local machine:

```bash
make build
./bin/helixagent
```

### Remote Container Distribution

HelixAgent can distribute all containers to remote hosts via SSH. Configure `Containers/.env`:

```bash
CONTAINERS_REMOTE_ENABLED=true
CONTAINERS_REMOTE_SCHEDULER=resource_aware

# Remote host configuration
CONTAINERS_REMOTE_HOST_1_NAME=thinker
CONTAINERS_REMOTE_HOST_1_ADDRESS=thinker.local
CONTAINERS_REMOTE_HOST_1_PORT=22
CONTAINERS_REMOTE_HOST_1_USER=milosvasic
CONTAINERS_REMOTE_HOST_1_RUNTIME=podman
CONTAINERS_REMOTE_HOST_1_LABELS=storage=fast,memory=high
```

When remote is enabled, ALL containers are distributed to the remote host(s). No containers run locally. No mixed mode is supported.

**Scheduling strategies:** `resource_aware`, `round_robin`, `affinity`, `spread`, `bin_pack`

**Requirements for remote distribution:**
- SSH key-based authentication (no interactive password prompts)
- Docker or Podman installed on remote host
- Network connectivity between hosts

### Release Builds

All release builds must be performed inside Docker/Podman containers for reproducibility:

```bash
make release              # Build helixagent for all platforms
make release-all          # Build ALL 7 apps for all platforms
make release-info         # Show version codes and source hashes
```

Output: `releases/<app>/<os>-<arch>/<version_code>/<binary>`

Supported platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64

## Configuration

### Core Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `7061` |
| `GIN_MODE` | Gin mode (`release`/`debug`) | `release` |
| `JWT_SECRET` | JWT signing key | Required |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | PostgreSQL user | `helixagent` |
| `DB_PASSWORD` | PostgreSQL password | Required |
| `DB_NAME` | PostgreSQL database | `helixagent_db` |
| `DB_SSL_MODE` | PostgreSQL SSL mode | `disable` |
| `REDIS_HOST` | Redis host | `localhost` |
| `REDIS_PORT` | Redis port | `6379` |
| `REDIS_PASSWORD` | Redis password | Required |
| `LOG_LEVEL` | Log level (`debug`/`info`/`warn`/`error`) | `info` |

### Feature Flags

| Variable | Description | Default |
|----------|-------------|---------|
| `GRAPHQL_ENABLED` | Enable GraphQL endpoint at `/v1/graphql` | `false` |
| `ENABLE_PPROF` | Enable pprof profiling endpoints | `false` |
| `COGNEE_ENABLED` | Enable Cognee knowledge graph (replaced by Mem0) | `false` |
| `CONSTITUTION_WATCHER_ENABLED` | Auto-update Constitution on project changes | `false` |
| `USE_HELIX_LLM` | Enable HelixLLM submodule integration | `true` |
| `LLM_VERIFIER_DISABLED` | Skip startup provider verification | `false` |

### Service Overrides

Use `SVC_<SERVICE>_<FIELD>` pattern to override service-level settings:

```bash
SVC_POSTGRESQL_HOST=remote-db.example.com
SVC_REDIS_REMOTE=true
SVC_REDIS_PORT=16379
```

### LLM Provider API Keys

| Provider | Environment Variable |
|----------|---------------------|
| OpenAI | `OPENAI_API_KEY` |
| Anthropic | `ANTHROPIC_API_KEY` |
| Claude (OAuth) | `CLAUDE_API_KEY` + `CLAUDE_USE_OAUTH_CREDENTIALS` |
| DeepSeek | `DEEPSEEK_API_KEY` |
| Gemini | `GEMINI_API_KEY` |
| Mistral | `MISTRAL_API_KEY` |
| OpenRouter | `OPENROUTER_API_KEY` |
| ZAI | `ZAI_API_KEY` |
| Cerebras | `CEREBRAS_API_KEY` |
| Groq | `GROQ_API_KEY` |
| Cohere | `COHERE_API_KEY` |
| Perplexity | `PERPLEXITY_API_KEY` |
| xAI | `XAI_API_KEY` |
| Together | `TOGETHER_API_KEY` |
| Fireworks | `FIREWORKS_API_KEY` |

See `.env.example` for the full list of all 43 providers.

## Health Checks

```bash
curl http://localhost:7061/v1/health
curl http://localhost:7061/v1/monitoring/status
curl http://localhost:7061/v1/startup/verification
```

## Monitoring

```bash
make monitoring-status
make monitoring-circuit-breakers
make monitoring-provider-health
make monitoring-fallback-chain
make force-health-check
```

Prometheus metrics are available at `/metrics` when `PROMETHEUS_ENABLED=true`.

## Scaling

### Horizontal Scaling

1. Use external PostgreSQL and Redis
2. Configure `SVC_*_REMOTE=true` in `.env`
3. Deploy multiple instances behind load balancer

### Resource Limits

All containers support resource limits via Docker Compose:

```yaml
services:
  helixagent:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G
```

**Test/challenge resource limits:** All test and challenge execution is limited to 30-40% of host resources (`GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`).

## Troubleshooting

### Common Issues

1. **Port already in use**: Change `PORT` in `.env`
2. **Database connection failed**: Check `DB_*` settings and container health
3. **Redis connection failed**: Check `REDIS_*` settings; test infra uses port 16379
4. **Provider unavailable**: Verify API keys, check `/v1/startup/verification`
5. **Slow startup**: Provider verification takes 1-2 minutes (normal)
6. **Container distribution failed**: Check SSH connectivity and `Containers/.env`

### Logs

```bash
# Server log
tail -f /tmp/helixagent-server.log

# Container logs (use podman or docker directly for inspection only)
podman logs <container-name>
```

## See Also

- [Getting Started](GETTING_STARTED.md) - First-time setup
- [Configuration Guide](configuration-guide.md) - All configuration options
- [Feature Flags](feature-flags.md) - Feature toggle system
- [FAQ](../FAQ.md) - Common questions
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Development guide
