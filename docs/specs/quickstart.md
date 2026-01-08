# HelixAgent LLM Facade Quickstart Guide

## Overview

HelixAgent LLM Facade is a unified API service that abstracts multiple LLM providers (DeepSeek, Claude, Gemini, Qwen, Z.AI) into a single intelligent interface with ensemble voting, memory enhancement via Cognee, and comprehensive monitoring.

## Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- PostgreSQL 14+ (or use the provided Docker setup)
- Kubernetes cluster (for production deployment)

## Quick Start

### 1. Clone and Setup

```bash
git clone https://github.com/your-org/helixagent.git
cd helixagent
```

### 2. Configure Environment

Copy the example configuration:

```bash
cp config/config.example.yaml config/config.yaml
cp .env.example .env
```

Edit `.env` with your API keys:

```bash
# LLM Provider API Keys
DEEPSEEK_API_KEY=your_deepseek_api_key
CLAUDE_API_KEY=your_claude_api_key
GEMINI_API_KEY=your_gemini_api_key
QWEN_API_KEY=your_qwen_api_key
ZAI_API_KEY=your_zai_api_key

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=helixagent
DB_PASSWORD=your_secure_password
DB_NAME=helixagent_db

# Cognee Configuration
COGNEE_PASSWORD=your_cognee_password
NEO4J_PASSWORD=your_neo4j_password
```

### 3. Start Services

Using Docker Compose for development:

```bash
docker-compose up -d
```

This starts:
- PostgreSQL database with pgvector
- Cognee memory system with ChromaDB and Neo4j
- Redis for caching
- HelixAgent API service

### 4. Build and Run the Application

```bash
# Build the application
make build

# Run in development mode
make dev

# Or run directly
go run cmd/helixagent/main.go
```

The API will be available at `http://localhost:7061`

## Basic Usage

### 1. Health Check

```bash
curl http://localhost:7061/v1/health
```

Response:
```json
{
  "status": "healthy",
  "components": [
    {
      "name": "database",
      "status": "healthy",
      "response_time_ms": 12
    },
    {
      "name": "cognee",
      "status": "healthy",
      "response_time_ms": 45
    }
  ],
  "timestamp": "2025-12-08T10:30:00Z",
  "version": "1.0.0"
}
```

### 2. List Available Providers

```bash
curl -H "X-API-Key: your_api_key" http://localhost:7061/v1/providers
```

### 3. Simple Text Completion

```bash
curl -X POST http://localhost:7061/v1/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{
    "prompt": "Explain the concept of microservices architecture",
    "model_params": {
      "max_tokens": 500,
      "temperature": 0.7
    }
  }'
```

### 4. Memory-Enhanced Completion

```bash
curl -X POST http://localhost:7061/v1/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{
    "prompt": "What did we discuss about Go programming?",
    "memory_enhanced": true,
    "model_params": {
      "max_tokens": 300
    }
  }'
```

### 5. Ensemble Request

```bash
curl -X POST http://localhost:7061/v1/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{
    "prompt": "Write a Go function to sort an array of integers",
    "request_type": "code_generation",
    "ensemble_config": {
      "strategy": "confidence_weighted",
      "min_providers": 3,
      "confidence_threshold": 0.7
    },
    "model_params": {
      "max_tokens": 1000,
      "temperature": 0.2
    }
  }'
```

### 6. Chat with Session

Create a session:
```bash
curl -X POST http://localhost:7061/v1/sessions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{
    "user_id": "user123",
    "memory_enabled": true
  }'
```

Start a chat:
```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{
    "session_id": "sess_abc123",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful Go programming assistant."
      },
      {
        "role": "user",
        "content": "How do I create a REST API in Go?"
      }
    ],
    "memory_enhanced": true
  }'
```

## Configuration

### Provider Configuration

Add new providers via the API:

```bash
curl -X POST http://localhost:7061/v1/providers \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your_api_key" \
  -d '{
    "name": "deepseek-primary",
    "type": "deepseek",
    "api_key": "your_deepseek_key",
    "base_url": "https://api.deepseek.com",
    "model": "deepseek-chat",
    "weight": 1.5,
    "config": {
      "timeout": "25s",
      "max_retries": 3
    }
  }'
```

### Ensemble Configuration

Configure ensemble behavior in `config/config.yaml`:

```yaml
ensemble:
  voting_strategy: "confidence_weighted"
  min_providers: 2
  confidence_threshold: 0.7
  fallback_to_best: true
  preferred_providers:
    - "claude"
    - "deepseek"

load_balancing:
  strategy: "weighted_round_robin"
  health_check_interval: "30s"
  circuit_breaker:
    failure_threshold: 5
    recovery_timeout: "60s"
```

### Memory Configuration

Configure Cognee integration:

```yaml
cognee:
  base_url: "http://localhost:8000"
  api_key: "${COGNEE_API_KEY}"
  auto_cognify: true
  cache_ttl: "1h"
  max_memory_per_request: 2048
  background_processing: true
```

## Monitoring

### Prometheus Metrics

Access metrics at `http://localhost:9090/metrics`

Key metrics:
- `llm_requests_total` - Total number of requests
- `llm_response_time_ms` - Response time histogram
- `llm_tokens_used_total` - Token usage counter
- `llm_provider_success_rate` - Provider success rate
- `llm_ensemble_confidence_score` - Ensemble confidence distribution

### Grafana Dashboard

Access Grafana at `http://localhost:3000` (admin/admin)

Pre-configured dashboards:
- System Overview
- Provider Performance
- Request Analytics
- Memory Usage

## Development

### Running Tests

```bash
# Unit tests
make test

# Integration tests
make test-integration

# E2E tests
make test-e2e

# Performance tests
make test-performance

# Security tests
make test-security
```

### Code Quality

```bash
# Linting
make lint

# Formatting
make fmt

# Security scanning
make security-scan

# Code coverage
make coverage
```

### Adding New Providers

1. Create a new plugin in `plugins/{provider-name}/`
2. Implement the `LLMProvider` interface
3. Add provider configuration to the schema
4. Write comprehensive tests
5. Update documentation

Example plugin structure:
```
plugins/newprovider/
├── plugin.go
├── client.go
├── config.go
├── test/
│   ├── unit_test.go
│   └── integration_test.go
└── README.md
```

## Deployment

### Docker

```bash
# Build image
docker build -t helixagent:latest .

# Run with Docker Compose
docker-compose -f docker-compose.prod.yml up -d
```

### Kubernetes

```bash
# Deploy to Kubernetes
kubectl apply -f k8s/

# Check deployment
kubectl get pods -l app=helixagent
```

### Environment Variables

Production environment variables:
```bash
# Required
HELIXAGENT_API_KEY=your_production_api_key
DB_HOST=your_database_host
DB_PASSWORD=your_secure_db_password

# Optional
LOG_LEVEL=info
METRICS_ENABLED=true
TRACING_ENABLED=true
```

## Troubleshooting

### Common Issues

1. **Provider Connection Errors**
   - Check API keys in environment variables
   - Verify network connectivity to provider endpoints
   - Review provider configuration in config file

2. **Memory System Errors**
   - Ensure Cognee container is running
   - Check Redis connection
   - Verify database schema

3. **Performance Issues**
   - Monitor response times in Grafana
   - Check provider health status
   - Review ensemble configuration

### Logs

```bash
# View application logs
docker-compose logs -f helixagent

# View specific component logs
docker-compose logs -f cognee
docker-compose logs -f postgres
```

### Health Checks

```bash
# Detailed health check
curl "http://localhost:7061/v1/health?detailed=true&components=database,cognee,providers"

# Check specific provider
curl -H "X-API-Key: your_api_key" \
  http://localhost:7061/v1/providers/deepseek-primary
```

## API Documentation

- Interactive API docs: `http://localhost:7061/docs`
- OpenAPI spec: `http://localhost:7061/v1/openapi.json`
- gRPC documentation: See `/contracts/llm-facade.proto`

## Support

- Documentation: `/docs`
- Issue tracking: GitHub Issues
- Community: Discord/Slack channel
- Support email: support@helixagent.com

## License

MIT License - see LICENSE file for details.