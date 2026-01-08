# HelixAgent CLI Reference

This document provides a comprehensive reference for the HelixAgent command-line interface (CLI), including all available commands, flags, configuration options, and usage examples.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Global Options](#global-options)
- [Commands](#commands)
- [Configuration](#configuration)
- [Environment Variables](#environment-variables)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

---

## Overview

HelixAgent is an AI-powered ensemble LLM service that combines responses from multiple language models using intelligent aggregation strategies. The CLI provides a simple interface to start and configure the HelixAgent server.

### Key Features

- Cognee knowledge graph integration for advanced AI memory
- Graph-powered reasoning beyond traditional RAG
- Multi-modal processing (text, code, images, audio)
- Auto-containerization for seamless deployment
- Automatic startup of required Docker containers
- Models.dev integration for comprehensive model metadata
- Multi-layer caching with Redis and in-memory
- Circuit breaker for API resilience
- Support for 7 LLM providers (Claude, DeepSeek, Gemini, Qwen, ZAI, Ollama, OpenRouter)

---

## Installation

### Building from Source

```bash
# Clone the repository
git clone https://dev.helix.agent.git
cd helixagent

# Build the binary
make build

# The binary will be available at ./bin/helixagent
```

### Build Variants

```bash
# Standard build (optimized)
make build

# Debug build (with debug symbols)
make build-debug

# Build for all architectures
make build-all
```

### Installing System-Wide

```bash
# Build and install to /usr/local/bin
make build
sudo make install

# Uninstall
sudo make uninstall
```

### Using Docker

```bash
# Build Docker image
make docker-build

# Or build directly
docker build -t helixagent:latest .
```

---

## Global Options

The HelixAgent CLI accepts the following global options:

### `--config`

**Type:** `string`
**Default:** `""` (empty, uses environment variables and defaults)
**Environment Variable:** N/A

Specifies the path to a YAML configuration file.

```bash
helixagent --config /path/to/config.yaml
```

### `--auto-start-docker`

**Type:** `boolean`
**Default:** `true`

When enabled, HelixAgent automatically starts required Docker containers (PostgreSQL, Redis, Cognee, ChromaDB) before starting the server.

```bash
# Start with auto-container management (default)
helixagent

# Disable auto-container management
helixagent --auto-start-docker=false
```

### `--version`

**Type:** `boolean`
**Default:** `false`

Displays version information and exits.

```bash
helixagent --version
# Output: HelixAgent v1.0.0 - Models.dev Enhanced Edition
```

### `--help`

**Type:** `boolean`
**Default:** `false`

Displays help message with all available options.

```bash
helixagent --help
```

---

## Commands

### Start Server (Default)

The default command starts the HelixAgent HTTP server.

```bash
# Start with defaults
helixagent

# Start with custom config
helixagent --config configs/production.yaml

# Start without auto-container management
helixagent --auto-start-docker=false
```

### Version

Display version information:

```bash
helixagent --version
```

### Help

Display help information:

```bash
helixagent --help
```

---

## Configuration

HelixAgent can be configured through:

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Configuration files (YAML)**
4. **Default values** (lowest priority)

### Configuration File Format

Configuration files use YAML format. Example configuration files are provided in the `configs/` directory:

- `configs/development.yaml` - Development environment settings
- `configs/production.yaml` - Production environment settings
- `configs/multi-provider.yaml` - Multi-provider configuration examples

### Example Configuration File

```yaml
server:
  port: 8080
  host: "0.0.0.0"
  environment: "development"
  log_level: "info"

  debug:
    enabled: false
    pprof_enabled: false
    verbose_logging: false

database:
  host: "localhost"
  port: 5432
  user: "helixagent"
  password: "secret"
  name: "helixagent_db"
  sslmode: "disable"
  max_open_connections: 20
  max_idle_connections: 5
  connection_max_lifetime: "1h"

redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  pool_size: 10

llm_providers:
  openai:
    enabled: true
    api_key: "${OPENAI_API_KEY}"
    model: "gpt-4"
    base_url: "https://api.openai.com/v1"
    temperature: 0.7
    max_tokens: 2048
    timeout: "30s"
    weight: 1.0

  anthropic:
    enabled: true
    api_key: "${ANTHROPIC_API_KEY}"
    model: "claude-3-sonnet-20240229"
    base_url: "https://api.anthropic.com"
    temperature: 0.7
    max_tokens: 2048
    timeout: "30s"
    weight: 0.8

  ollama:
    enabled: true
    base_url: "http://localhost:11434"
    model: "llama2"
    temperature: 0.7
    max_tokens: 2048
    timeout: "60s"
    weight: 0.5

security:
  jwt:
    secret: "${JWT_SECRET}"
    expiration: "24h"
  rate_limit:
    requests_per_minute: 60
    burst: 10

monitoring:
  metrics:
    enabled: true
    port: 9090
    path: "/metrics"
  health_check:
    enabled: true
    path: "/health"
    interval: "30s"

cache:
  default_ttl: 3600
  max_memory_size: "500MB"
  max_entries: 50000

optimization:
  enabled: true
  semantic_cache:
    enabled: true
    similarity_threshold: 0.85
    max_entries: 10000
    ttl: "24h"
  streaming:
    enabled: true
    buffer_type: "word"
```

---

## Environment Variables

HelixAgent supports extensive configuration through environment variables. These override defaults but are overridden by configuration file values.

### Server Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `SERVER_HOST` | Server bind address | `0.0.0.0` |
| `GIN_MODE` | Gin framework mode (`debug`, `release`) | `release` |
| `JWT_SECRET` | JWT signing secret (**required**) | - |
| `HELIXAGENT_API_KEY` | API key for authentication | - |
| `READ_TIMEOUT` | HTTP read timeout | `30s` |
| `WRITE_TIMEOUT` | HTTP write timeout | `30s` |
| `TOKEN_EXPIRY` | JWT token expiration | `24h` |
| `CORS_ENABLED` | Enable CORS | `true` |
| `CORS_ORIGINS` | Allowed CORS origins (comma-separated) | `*` |
| `REQUEST_LOGGING` | Enable request logging | `true` |
| `DEBUG_ENABLED` | Enable debug mode | `false` |

### Database Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | PostgreSQL username | `helixagent` |
| `DB_PASSWORD` | PostgreSQL password | `secret` |
| `DB_NAME` | Database name | `helixagent_db` |
| `DB_SSLMODE` | SSL mode (`disable`, `require`) | `disable` |
| `DB_MAX_CONNECTIONS` | Maximum connections | `20` |
| `DB_CONN_TIMEOUT` | Connection timeout | `10s` |
| `DB_POOL_SIZE` | Connection pool size | `10` |

### Redis Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `REDIS_HOST` | Redis host | `localhost` |
| `REDIS_PORT` | Redis port | `6379` |
| `REDIS_PASSWORD` | Redis password | - |
| `REDIS_DB` | Redis database number | `0` |
| `REDIS_POOL_SIZE` | Connection pool size | `10` |
| `REDIS_TIMEOUT` | Connection timeout | `5s` |

### LLM Provider Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CLAUDE_API_KEY` | Anthropic Claude API key | - |
| `DEEPSEEK_API_KEY` | DeepSeek API key | - |
| `GEMINI_API_KEY` | Google Gemini API key | - |
| `QWEN_API_KEY` | Alibaba Qwen API key | - |
| `ZAI_API_KEY` | ZAI API key | - |
| `OLLAMA_ENABLED` | Enable Ollama provider | `true` |
| `OLLAMA_BASE_URL` | Ollama server URL | `http://localhost:11434` |
| `OLLAMA_MODEL` | Default Ollama model | `llama2` |
| `LLM_TIMEOUT` | Default LLM request timeout | `60s` |
| `LLM_MAX_RETRIES` | Maximum retry attempts | `3` |

### Ensemble Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `ENSEMBLE_STRATEGY` | Voting strategy | `confidence_weighted` |
| `ENSEMBLE_MIN_PROVIDERS` | Minimum providers | `2` |
| `ENSEMBLE_MAX_PROVIDERS` | Maximum providers | `5` |
| `ENSEMBLE_CONFIDENCE_THRESHOLD` | Confidence threshold | `0.8` |
| `ENSEMBLE_FALLBACK_BEST` | Fallback to best response | `true` |
| `ENSEMBLE_TIMEOUT` | Ensemble timeout | `30s` |

### Cognee Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `COGNEE_ENABLED` | Enable Cognee integration | `true` |
| `COGNEE_BASE_URL` | Cognee service URL | `http://cognee:8000` |
| `COGNEE_API_KEY` | Cognee API key | - |
| `COGNEE_AUTO_COGNIFY` | Auto-cognify new data | `true` |
| `COGNEE_TIMEOUT` | Request timeout | `30s` |

### Models.dev Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `MODELSDEV_ENABLED` | Enable Models.dev | `false` |
| `MODELSDEV_API_KEY` | Models.dev API key | - |
| `MODELSDEV_BASE_URL` | API base URL | `https://api.models.dev/v1` |
| `MODELSDEV_REFRESH_INTERVAL` | Data refresh interval | `24h` |
| `MODELSDEV_CACHE_TTL` | Cache TTL | `1h` |
| `MODELSDEV_BATCH_SIZE` | Batch size | `100` |
| `MODELSDEV_MAX_RETRIES` | Max retries | `3` |
| `MODELSDEV_AUTO_REFRESH` | Enable auto-refresh | `true` |

### Monitoring Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `METRICS_ENABLED` | Enable Prometheus metrics | `true` |
| `METRICS_PATH` | Metrics endpoint path | `/metrics` |
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `TRACING_ENABLED` | Enable distributed tracing | `false` |
| `JAEGER_ENDPOINT` | Jaeger endpoint URL | - |
| `PROMETHEUS_ENABLED` | Enable Prometheus | `true` |
| `PROMETHEUS_PORT` | Prometheus port | `9090` |

### Security Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `SESSION_TIMEOUT` | Session timeout | `24h` |
| `MAX_LOGIN_ATTEMPTS` | Max login attempts | `5` |
| `LOCKOUT_DURATION` | Account lockout duration | `15m` |
| `RATE_LIMITING_ENABLED` | Enable rate limiting | `true` |
| `RATE_LIMIT_REQUESTS` | Requests per window | `100` |
| `RATE_LIMIT_WINDOW` | Rate limit window | `1m` |
| `RATE_LIMIT_STRATEGY` | Rate limit strategy | `sliding_window` |

### Performance Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `MAX_CONCURRENT_REQUESTS` | Max concurrent requests | `10` |
| `REQUEST_TIMEOUT` | Request timeout | `60s` |
| `IDLE_TIMEOUT` | Idle timeout | `120s` |
| `READ_BUFFER_SIZE` | Read buffer size | `4096` |
| `WRITE_BUFFER_SIZE` | Write buffer size | `4096` |
| `ENABLE_COMPRESSION` | Enable response compression | `true` |

### Protocol Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `MCP_ENABLED` | Enable MCP protocol | `true` |
| `MCP_EXPOSE_ALL_TOOLS` | Expose all MCP tools | `true` |
| `MCP_UNIFIED_NAMESPACE` | Use unified namespace | `true` |
| `ACP_ENABLED` | Enable ACP protocol | `true` |
| `ACP_DEFAULT_TIMEOUT` | ACP request timeout | `30s` |
| `ACP_MAX_RETRIES` | ACP max retries | `3` |

### Streaming Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `STREAMING_ENABLED` | Enable streaming | `true` |
| `STREAMING_BUFFER_SIZE` | Buffer size | `1024` |
| `STREAMING_KEEP_ALIVE` | Keep-alive interval | `30s` |
| `STREAMING_HEARTBEAT` | Enable heartbeat | `true` |

### Plugin Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `PLUGIN_WATCH_PATHS` | Plugin watch paths | `./plugins` |
| `PLUGIN_AUTO_RELOAD` | Auto-reload plugins | `false` |
| `PLUGIN_ENABLED_PLUGINS` | Enabled plugins (comma-separated) | - |
| `PLUGIN_DIRS` | Plugin directories | `./plugins` |
| `PLUGIN_HOT_RELOAD` | Enable hot reload | `false` |

---

## Examples

### Starting the Server

#### Basic Start

```bash
# Start with all defaults
helixagent
```

#### Development Mode

```bash
# Start in debug mode with verbose logging
GIN_MODE=debug LOG_LEVEL=debug helixagent
```

#### Production Mode

```bash
# Start with production configuration
helixagent --config configs/production.yaml
```

#### Without Auto-Container Management

```bash
# Start without automatically starting Docker containers
helixagent --auto-start-docker=false
```

### Running with Different Providers

#### Using Ollama (Local, Free)

```bash
# Start with Ollama only
OLLAMA_ENABLED=true \
OLLAMA_BASE_URL=http://localhost:11434 \
OLLAMA_MODEL=llama2 \
helixagent
```

#### Using Claude API

```bash
# Start with Claude provider
CLAUDE_API_KEY=your-api-key \
helixagent
```

#### Using Multiple Providers

```bash
# Start with multiple providers for ensemble
CLAUDE_API_KEY=your-claude-key \
DEEPSEEK_API_KEY=your-deepseek-key \
GEMINI_API_KEY=your-gemini-key \
OLLAMA_ENABLED=true \
ENSEMBLE_STRATEGY=confidence_weighted \
ENSEMBLE_MIN_PROVIDERS=2 \
helixagent
```

### Using Docker

#### Start Core Services

```bash
# Start PostgreSQL, Redis, Cognee, ChromaDB
docker compose up -d
```

#### Start with AI Services

```bash
# Include Ollama
docker compose --profile ai up -d
```

#### Start with Monitoring

```bash
# Include Prometheus and Grafana
docker compose --profile monitoring up -d
```

#### Start Full Stack

```bash
# All services including optimization tools
docker compose --profile full up -d
```

#### Start with Optimization Services

```bash
# CPU-only optimization services
docker compose --profile optimization up -d

# GPU optimization (SGLang)
docker compose --profile optimization-gpu up -d
```

### Using Podman

```bash
# Enable Podman socket
systemctl --user enable --now podman.socket

# Start with podman-compose
podman-compose up -d

# Or use the wrapper script
./scripts/container-runtime.sh start
```

### Health Check Commands

#### Check Server Health

```bash
curl http://localhost:7061/health
```

#### Check Metrics

```bash
curl http://localhost:7061/metrics
```

#### Check Provider Status

```bash
curl http://localhost:7061/api/v1/providers
```

#### Check Cognee Health

```bash
curl http://localhost:8000/health
```

---

## Troubleshooting

### Common Errors and Solutions

#### 1. Docker Containers Not Starting

**Error:** `failed to start containers: docker not found in PATH`

**Solution:**
```bash
# Ensure Docker is installed and in PATH
docker --version

# Start containers manually
docker compose up -d postgres redis cognee chromadb
```

#### 2. Database Connection Failed

**Error:** `cannot connect to postgres`

**Solution:**
```bash
# Check PostgreSQL is running
docker compose ps postgres

# Verify connection settings
psql -h localhost -U helixagent -d helixagent_db

# Check logs
docker compose logs postgres
```

#### 3. Redis Connection Failed

**Error:** `cannot connect to redis`

**Solution:**
```bash
# Check Redis is running
docker compose ps redis

# Test connection
redis-cli -h localhost -p 6379 ping

# Check if password is required
REDIS_PASSWORD=your-password redis-cli -h localhost -p 6379 -a $REDIS_PASSWORD ping
```

#### 4. LLM Provider Errors

**Error:** `provider X returned error: unauthorized`

**Solution:**
```bash
# Verify API keys are set correctly
echo $CLAUDE_API_KEY
echo $DEEPSEEK_API_KEY

# Test provider directly
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $CLAUDE_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model": "claude-3-sonnet-20240229", "max_tokens": 100, "messages": [{"role": "user", "content": "Hello"}]}'
```

#### 5. Port Already in Use

**Error:** `bind: address already in use`

**Solution:**
```bash
# Find process using the port
lsof -i :7061

# Kill the process or use a different port
PORT=8081 helixagent
```

#### 6. Memory Issues

**Error:** `out of memory` or high memory usage

**Solution:**
```bash
# Reduce cache size
CACHE_MAX_ENTRIES=5000 helixagent

# Limit concurrent requests
MAX_CONCURRENT_REQUESTS=5 helixagent

# Check container memory limits
docker stats
```

### Debug Mode Usage

Enable debug mode to get detailed logging:

```bash
# Enable all debug options
GIN_MODE=debug \
LOG_LEVEL=debug \
DEBUG_ENABLED=true \
REQUEST_LOGGING=true \
helixagent
```

#### Debug Logging Levels

- `debug` - All messages including detailed request/response logs
- `info` - Informational messages, startup, shutdown
- `warn` - Warning messages
- `error` - Error messages only

### Viewing Logs

```bash
# View Docker logs
docker compose logs -f helixagent

# View specific service logs
docker compose logs -f postgres
docker compose logs -f redis
docker compose logs -f cognee

# View all logs
docker compose logs -f
```

### Checking Service Status

```bash
# Check all container status
docker compose ps

# Check specific container health
docker inspect --format='{{.State.Health.Status}}' helixagent-app

# Check infrastructure status
make test-infra-status
```

### Resetting the Environment

```bash
# Stop all containers
docker compose down

# Clean all data volumes
docker compose down -v --remove-orphans

# Full cleanup
make docker-clean-all
```

---

## Additional Resources

- **API Documentation:** `http://localhost:7061/docs` (when running)
- **Prometheus Metrics:** `http://localhost:9090` (when monitoring enabled)
- **Grafana Dashboard:** `http://localhost:3000` (when monitoring enabled)
- **Cognee API:** `http://localhost:8000` (when Cognee enabled)

For more information, visit the project repository or consult the documentation in the `docs/` directory.
