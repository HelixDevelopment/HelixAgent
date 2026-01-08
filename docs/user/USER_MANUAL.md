# HelixAgent User Manual

Comprehensive guide for using HelixAgent - the intelligent ensemble LLM service.

## Table of Contents

1. [Introduction](#introduction)
2. [Installation](#installation)
3. [Configuration](#configuration)
4. [LLM Providers](#llm-providers)
5. [Chat Completions](#chat-completions)
6. [Ensemble Mode](#ensemble-mode)
7. [AI Debate System](#ai-debate-system)
8. [Model Verification](#model-verification)
9. [Streaming](#streaming)
10. [SDKs and Client Libraries](#sdks-and-client-libraries)
11. [Advanced Features](#advanced-features)
12. [Performance Optimization](#performance-optimization)
13. [Monitoring and Observability](#monitoring-and-observability)
14. [Troubleshooting](#troubleshooting)
15. [Glossary](#glossary)

---

## Introduction

HelixAgent is an AI-powered ensemble LLM service that combines responses from multiple language models using intelligent aggregation strategies. It provides OpenAI-compatible APIs and supports 7+ LLM providers.

### Key Features

- **Ensemble Orchestration**: Combine responses from multiple LLMs using confidence-weighted voting
- **AI Debate System**: Multi-round debates between AI models to reach consensus
- **Model Verification**: Test and score models before production use
- **Provider Failover**: Automatic failover between providers
- **OpenAI Compatibility**: Drop-in replacement for OpenAI API
- **Streaming Support**: Real-time response streaming with SSE

### System Requirements

- **Server**: Go 1.23+ or Docker
- **Database**: PostgreSQL 15+
- **Cache**: Redis 7+
- **Memory**: 2GB+ RAM recommended
- **Storage**: 1GB+ for logs and cache

---

## Installation

### Docker Installation (Recommended)

```bash
# Clone the repository
git clone https://dev.helix.agent.git
cd helixagent

# Copy and configure environment
cp .env.example .env
nano .env  # Add your API keys

# Start all services
docker-compose up -d

# Verify installation
curl http://localhost:7061/health
```

### Docker Profiles

HelixAgent uses Docker profiles to control which services start:

```bash
# Core services only (postgres, redis, cognee, chromadb)
docker-compose up -d

# Add AI services (ollama)
docker-compose --profile ai up -d

# Add monitoring (prometheus, grafana)
docker-compose --profile monitoring up -d

# Add optimization services
docker-compose --profile optimization up -d

# Everything
docker-compose --profile full up -d
```

### Manual Installation

```bash
# Install Go 1.23+
# https://golang.org/doc/install

# Clone repository
git clone https://dev.helix.agent.git
cd helixagent

# Install dependencies
go mod download

# Build
make build

# Configure
cp .env.example .env
nano .env

# Run
./bin/helixagent
```

### Podman Support

For rootless container environments:

```bash
# Enable Podman socket
systemctl --user enable --now podman.socket

# Use podman-compose
pip install podman-compose
podman-compose up -d
```

---

## Configuration

### Environment Variables

Create a `.env` file with your configuration:

```bash
# Server Configuration
PORT=8080
GIN_MODE=release
JWT_SECRET=your-secret-key

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=helixagent
DB_PASSWORD=your-db-password
DB_NAME=helixagent

# Redis Cache
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your-redis-password

# LLM Provider API Keys
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
DEEPSEEK_API_KEY=sk-...
GEMINI_API_KEY=...
QWEN_API_KEY=...
ZAI_API_KEY=...

# Ollama (Local LLM)
OLLAMA_ENABLED=true
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama2

# OpenRouter (Multi-provider Gateway)
OPENROUTER_API_KEY=sk-or-...
```

### Configuration Files

YAML configuration files are in `/configs`:

**development.yaml**
```yaml
server:
  port: 8080
  debug: true
  request_timeout: 30s

providers:
  openai:
    enabled: true
    api_key: ${OPENAI_API_KEY}
    models:
      - gpt-4
      - gpt-4-turbo
      - gpt-3.5-turbo
  anthropic:
    enabled: true
    api_key: ${ANTHROPIC_API_KEY}
    models:
      - claude-3-opus
      - claude-3-sonnet
      - claude-3-haiku

ensemble:
  default_strategy: confidence_weighted
  min_providers: 2
  timeout: 60s
  retry_count: 3

cache:
  enabled: true
  type: redis
  ttl: 1h

logging:
  level: debug
  format: json
```

**production.yaml**
```yaml
server:
  port: 8080
  debug: false
  request_timeout: 30s
  read_timeout: 10s
  write_timeout: 30s

security:
  jwt_enabled: true
  rate_limit:
    enabled: true
    requests_per_minute: 60
    burst: 10
  cors:
    allowed_origins: ["https://yourdomain.com"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE"]

database:
  host: ${DB_HOST}
  port: ${DB_PORT}
  user: ${DB_USER}
  password: ${DB_PASSWORD}
  name: ${DB_NAME}
  max_connections: 25
  ssl_mode: require
```

---

## LLM Providers

HelixAgent supports 7 LLM providers out of the box.

### OpenAI

```yaml
providers:
  openai:
    enabled: true
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1  # Optional, default
    models:
      - gpt-4
      - gpt-4-turbo
      - gpt-3.5-turbo
      - gpt-4o
```

### Anthropic (Claude)

```yaml
providers:
  anthropic:
    enabled: true
    api_key: ${ANTHROPIC_API_KEY}
    models:
      - claude-3-opus-20240229
      - claude-3-sonnet-20240229
      - claude-3-haiku-20240307
```

### Google Gemini

```yaml
providers:
  google:
    enabled: true
    api_key: ${GEMINI_API_KEY}
    models:
      - gemini-pro
      - gemini-pro-vision
```

### DeepSeek

```yaml
providers:
  deepseek:
    enabled: true
    api_key: ${DEEPSEEK_API_KEY}
    models:
      - deepseek-chat
      - deepseek-coder
```

### Qwen

```yaml
providers:
  qwen:
    enabled: true
    api_key: ${QWEN_API_KEY}
    models:
      - qwen-turbo
      - qwen-plus
```

### Ollama (Local)

```yaml
providers:
  ollama:
    enabled: true
    base_url: http://localhost:11434
    models:
      - llama2
      - codellama
      - mistral
```

### OpenRouter (Multi-Provider Gateway)

```yaml
providers:
  openrouter:
    enabled: true
    api_key: ${OPENROUTER_API_KEY}
    # Access 100+ models through single gateway
```

### Cloud Providers

#### AWS Bedrock

```yaml
providers:
  aws_bedrock:
    enabled: true
    region: us-east-1
    access_key_id: ${AWS_ACCESS_KEY_ID}
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}
    models:
      - anthropic.claude-3-opus-20240229-v1:0
      - amazon.titan-text-lite-v1
```

#### GCP Vertex AI

```yaml
providers:
  gcp_vertex:
    enabled: true
    project_id: ${GCP_PROJECT_ID}
    location: us-central1
    models:
      - gemini-pro
      - text-bison
```

#### Azure OpenAI

```yaml
providers:
  azure_openai:
    enabled: true
    endpoint: ${AZURE_OPENAI_ENDPOINT}
    api_key: ${AZURE_OPENAI_API_KEY}
    api_version: "2024-02-15-preview"
    deployments:
      - gpt-4
      - gpt-35-turbo
```

### Listing Available Providers

```bash
# List all configured providers
curl http://localhost:7061/v1/providers

# List available models
curl http://localhost:7061/v1/models
```

---

## Chat Completions

### Basic Request

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "What is the capital of France?"}
    ]
  }'
```

### Response Format

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1704067200,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The capital of France is Paris."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 23,
    "completion_tokens": 8,
    "total_tokens": 31
  }
}
```

### Request Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `model` | string | Model ID to use | Required |
| `messages` | array | Conversation messages | Required |
| `max_tokens` | integer | Maximum tokens to generate | Model default |
| `temperature` | float | Randomness (0-2) | 1.0 |
| `top_p` | float | Nucleus sampling | 1.0 |
| `stream` | boolean | Enable streaming | false |
| `stop` | array | Stop sequences | null |
| `presence_penalty` | float | Presence penalty (-2 to 2) | 0 |
| `frequency_penalty` | float | Frequency penalty (-2 to 2) | 0 |

### Message Roles

- `system`: Sets behavior and context for the assistant
- `user`: User messages and questions
- `assistant`: Previous assistant responses (for context)

---

## Ensemble Mode

Ensemble mode aggregates responses from multiple LLM providers to produce higher-quality outputs.

### Basic Ensemble Request

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-ensemble",
    "messages": [
      {"role": "user", "content": "Explain quantum entanglement."}
    ],
    "ensemble_config": {
      "strategy": "confidence_weighted",
      "min_providers": 2
    }
  }'
```

### Ensemble Strategies

#### Confidence-Weighted Voting

Weighs responses based on provider confidence scores:

```json
{
  "ensemble_config": {
    "strategy": "confidence_weighted",
    "min_providers": 2,
    "weights": {
      "openai": 1.0,
      "anthropic": 1.0,
      "google": 0.8
    }
  }
}
```

#### Majority Vote

Selects the most common response:

```json
{
  "ensemble_config": {
    "strategy": "majority_vote",
    "min_providers": 3
  }
}
```

#### Best-of-N

Returns the highest-scored response:

```json
{
  "ensemble_config": {
    "strategy": "best_of_n",
    "n": 3
  }
}
```

### Ensemble Response

```json
{
  "id": "ensemble-abc123",
  "model": "helixagent-ensemble",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Quantum entanglement is..."
      },
      "finish_reason": "stop"
    }
  ],
  "ensemble_metadata": {
    "strategy": "confidence_weighted",
    "providers_used": ["openai", "anthropic"],
    "scores": {
      "openai": 0.92,
      "anthropic": 0.89
    },
    "selected_provider": "openai"
  }
}
```

---

## AI Debate System

The AI Debate system enables multi-round debates between LLM models to explore different perspectives and reach consensus.

### Starting a Debate

```bash
curl -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "Is renewable energy more cost-effective than fossil fuels?",
    "participants": [
      {"provider": "openai", "model": "gpt-4", "stance": "pro"},
      {"provider": "anthropic", "model": "claude-3-opus", "stance": "con"}
    ],
    "max_rounds": 3,
    "timeout": 300
  }'
```

### Debate Response

```json
{
  "debate_id": "debate-abc123",
  "status": "in_progress",
  "topic": "Is renewable energy more cost-effective than fossil fuels?",
  "participants": [
    {"provider": "openai", "model": "gpt-4", "stance": "pro"},
    {"provider": "anthropic", "model": "claude-3-opus", "stance": "con"}
  ],
  "current_round": 1,
  "max_rounds": 3
}
```

### Checking Debate Status

```bash
curl http://localhost:7061/v1/debates/{debate_id}
```

### Getting Debate Results

```bash
curl http://localhost:7061/v1/debates/{debate_id}/results
```

### Results Format

```json
{
  "debate_id": "debate-abc123",
  "status": "completed",
  "topic": "Is renewable energy more cost-effective than fossil fuels?",
  "rounds": [
    {
      "round": 1,
      "arguments": [
        {
          "provider": "openai",
          "stance": "pro",
          "argument": "Initial capital costs for renewables have dropped...",
          "score": 0.85
        },
        {
          "provider": "anthropic",
          "stance": "con",
          "argument": "While costs have decreased, grid integration...",
          "score": 0.82
        }
      ]
    }
  ],
  "consensus": {
    "reached": true,
    "points": [
      "Both agree long-term costs favor renewables",
      "Infrastructure investment needed regardless of energy source"
    ]
  },
  "key_disagreements": [
    "Timeline for cost parity",
    "Role of government subsidies"
  ],
  "quality_scores": {
    "openai": 0.88,
    "anthropic": 0.85
  }
}
```

### Debate Configuration Options

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `topic` | string | Debate topic | Required |
| `participants` | array | AI participants | Required |
| `max_rounds` | integer | Maximum debate rounds | 3 |
| `timeout` | integer | Timeout in seconds | 300 |
| `judge_provider` | string | Provider to judge debate | null |
| `require_consensus` | boolean | Require consensus to end | false |

---

## Model Verification

LLMsVerifier integration allows you to test and score models before production use.

### Full Model Verification

```bash
curl -X POST http://localhost:7061/api/v1/verifier/verify \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "gpt-4",
    "provider": "openai",
    "tests": ["existence", "responsiveness", "streaming", "code_visibility"]
  }'
```

### Verification Tests

| Test | Description |
|------|-------------|
| `existence` | Verify model exists and is accessible |
| `responsiveness` | Test response time and reliability |
| `streaming` | Verify streaming capability |
| `code_visibility` | Test model's code understanding |
| `consistency` | Check response consistency |

### Code Visibility Check

Test if a model can understand specific code:

```bash
curl -X POST http://localhost:7061/api/v1/verifier/code-visibility \
  -H "Content-Type: application/json" \
  -d '{
    "code": "def fibonacci(n):\n    if n <= 1:\n        return n\n    return fibonacci(n-1) + fibonacci(n-2)",
    "language": "python",
    "model_id": "gpt-4",
    "provider": "openai"
  }'
```

### Model Scoring

```bash
# Get model score
curl http://localhost:7061/api/v1/verifier/scores/gpt-4

# Get top models
curl "http://localhost:7061/api/v1/verifier/scores/top?limit=5"

# Compare models
curl -X POST http://localhost:7061/api/v1/verifier/scores/compare \
  -H "Content-Type: application/json" \
  -d '{"model_ids": ["gpt-4", "claude-3-opus", "gemini-pro"]}'
```

### Custom Scoring Weights

```bash
curl -X PUT http://localhost:7061/api/v1/verifier/scores/weights \
  -H "Content-Type: application/json" \
  -d '{
    "response_speed": 0.30,
    "model_efficiency": 0.20,
    "cost_effectiveness": 0.25,
    "capability": 0.15,
    "recency": 0.10
  }'
```

### Provider Health Monitoring

```bash
# All providers
curl http://localhost:7061/api/v1/verifier/health/providers

# Healthy providers only
curl http://localhost:7061/api/v1/verifier/health/providers/healthy

# Fastest provider
curl -X POST http://localhost:7061/api/v1/verifier/health/fastest \
  -H "Content-Type: application/json" \
  -d '{"providers": ["openai", "anthropic", "google"]}'
```

---

## Streaming

HelixAgent supports Server-Sent Events (SSE) for real-time response streaming.

### Basic Streaming Request

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -N \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Write a poem about AI."}],
    "stream": true
  }'
```

### Stream Events

```
data: {"id":"chatcmpl-abc","choices":[{"delta":{"content":"In"}}]}

data: {"id":"chatcmpl-abc","choices":[{"delta":{"content":" circuits"}}]}

data: {"id":"chatcmpl-abc","choices":[{"delta":{"content":" deep"}}]}

data: [DONE]
```

### Streaming in SDKs

**Python**
```python
stream = client.chat.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Write a story"}],
    stream=True
)

for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

**JavaScript**
```javascript
const stream = await client.chat.create({
  model: 'gpt-4',
  messages: [{ role: 'user', content: 'Write a story' }],
  stream: true,
});

for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0].delta.content || '');
}
```

**Go**
```go
stream, _ := client.Chat.Completions.CreateStream(ctx, &ChatRequest{
    Model: "gpt-4",
    Messages: []ChatMessage{{Role: "user", Content: "Write a story"}},
    Stream: true,
})

for event := range stream.Events() {
    if event.Choices[0].Delta.Content != "" {
        fmt.Print(event.Choices[0].Delta.Content)
    }
}
```

---

## SDKs and Client Libraries

### Python SDK

```bash
pip install helixagent-py
```

```python
from helixagent import HelixAgent

# Initialize client
client = HelixAgent(
    api_key="your-api-key",
    base_url="http://localhost:7061"  # Optional
)

# Chat completion
response = client.chat.create(
    model="gpt-4",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello!"}
    ]
)
print(response.choices[0].message.content)

# Ensemble chat
response = client.chat.create(
    model="helixagent-ensemble",
    messages=[{"role": "user", "content": "What is AI?"}],
    ensemble_config={"strategy": "confidence_weighted", "min_providers": 2}
)

# Start debate
debate = client.debates.create(
    topic="Is AI beneficial for society?",
    participants=[
        {"provider": "openai", "model": "gpt-4"},
        {"provider": "anthropic", "model": "claude-3-opus"}
    ]
)

# Wait for results
results = client.debates.wait_for_completion(debate.debate_id)
```

### Go SDK

```bash
go get dev.helix.agent-go
```

```go
import "dev.helix.agent-go"

// Initialize client
client := helixagent.NewClient(&helixagent.Config{
    APIKey:  "your-api-key",
    BaseURL: "http://localhost:7061",
})

// Chat completion
resp, err := client.Chat.Completions.Create(ctx, &helixagent.ChatCompletionRequest{
    Model: "gpt-4",
    Messages: []helixagent.ChatMessage{
        {Role: "system", Content: "You are a helpful assistant."},
        {Role: "user", Content: "Hello!"},
    },
})
fmt.Println(resp.Choices[0].Message.Content)

// Ensemble chat
resp, _ := client.Chat.Completions.Create(ctx, &helixagent.ChatCompletionRequest{
    Model: "helixagent-ensemble",
    Messages: []helixagent.ChatMessage{
        {Role: "user", Content: "What is AI?"},
    },
    EnsembleConfig: &helixagent.EnsembleConfig{
        Strategy:     "confidence_weighted",
        MinProviders: 2,
    },
})
```

### JavaScript SDK

```bash
npm install helixagent-js
```

```javascript
import { HelixAgent } from 'helixagent-js';

// Initialize client
const client = new HelixAgent({
  apiKey: 'your-api-key',
  baseUrl: 'http://localhost:7061',
});

// Chat completion
const response = await client.chat.create({
  model: 'gpt-4',
  messages: [
    { role: 'system', content: 'You are a helpful assistant.' },
    { role: 'user', content: 'Hello!' },
  ],
});
console.log(response.choices[0].message.content);

// Ensemble chat
const ensembleResponse = await client.chat.create({
  model: 'helixagent-ensemble',
  messages: [{ role: 'user', content: 'What is AI?' }],
  ensembleConfig: { strategy: 'confidence_weighted', minProviders: 2 },
});
```

### Mobile SDKs

#### iOS (Swift)

```swift
import HelixAgentSDK

let client = HelixAgent(apiKey: "your-api-key")

let response = try await client.chat.create(
    model: "gpt-4",
    messages: [
        ChatMessage(role: .user, content: "Hello!")
    ]
)
print(response.choices[0].message.content)
```

#### Android (Kotlin)

```kotlin
import com.helixagent.sdk.HelixAgent

val client = HelixAgent.Builder()
    .apiKey("your-api-key")
    .build()

val response = client.chat.create(
    ChatCompletionRequest(
        model = "gpt-4",
        messages = listOf(
            ChatMessage(role = "user", content = "Hello!")
        )
    )
)
println(response.choices[0].message.content)
```

---

## Advanced Features

### Cognee Integration (Knowledge Graph)

HelixAgent integrates with Cognee for knowledge graph and RAG capabilities:

```bash
# Add document to knowledge graph
curl -X POST http://localhost:7061/api/v1/cognee/documents \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Your document content here...",
    "metadata": {"source": "manual", "category": "technical"}
  }'

# Query knowledge graph
curl -X POST http://localhost:7061/api/v1/cognee/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is the architecture of HelixAgent?"}'
```

### Plugin System

HelixAgent supports hot-reloadable plugins:

```bash
# List plugins
curl http://localhost:7061/api/v1/plugins

# Enable plugin
curl -X POST http://localhost:7061/api/v1/plugins/my-plugin/enable

# Plugin health
curl http://localhost:7061/api/v1/plugins/my-plugin/health
```

### MCP (Model Context Protocol)

```bash
curl -X POST http://localhost:7061/api/v1/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "method": "resources/list",
    "params": {}
  }'
```

### LSP (Language Server Protocol)

```bash
curl -X POST http://localhost:7061/api/v1/lsp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "textDocument/completion",
    "params": {"textDocument": {"uri": "file:///example.py"}}
  }'
```

### Circuit Breaker Configuration

```yaml
circuit_breaker:
  enabled: true
  threshold: 5              # Failures before opening
  timeout: 30s              # Time in open state
  half_open_max: 3          # Max requests in half-open state
```

### Rate Limiting

```yaml
rate_limit:
  enabled: true
  requests_per_minute: 60
  burst: 10
  by_user: true             # Per-user limits
  by_ip: false              # Or per-IP limits
```

---

## Performance Optimization

### Semantic Caching

Enable caching for frequently repeated queries:

```yaml
optimization:
  semantic_cache:
    enabled: true
    similarity_threshold: 0.85
    max_entries: 10000
    ttl: 24h
```

### Connection Pooling

```yaml
http_client:
  max_idle_conns: 100
  max_idle_conns_per_host: 10
  idle_conn_timeout: 90s
```

### Batch Processing

```bash
# Batch verification
curl -X POST http://localhost:7061/api/v1/verifier/batch-verify \
  -H "Content-Type: application/json" \
  -d '{
    "models": [
      {"model_id": "gpt-4", "provider": "openai"},
      {"model_id": "claude-3-opus", "provider": "anthropic"}
    ]
  }'
```

### Retry Configuration

```yaml
retry:
  count: 3
  wait_time: 100ms
  max_wait: 2s
  exponential_backoff: true
```

---

## Monitoring and Observability

### Health Checks

```bash
# Application health
curl http://localhost:7061/health

# Detailed health
curl http://localhost:7061/health/detailed

# Provider health
curl http://localhost:7061/api/v1/verifier/health/providers
```

### Prometheus Metrics

```bash
# Metrics endpoint
curl http://localhost:7061/metrics
```

Available metrics:
- `helixagent_requests_total` - Total requests by endpoint
- `helixagent_request_duration_seconds` - Request latency
- `helixagent_provider_requests_total` - Requests per provider
- `helixagent_provider_errors_total` - Errors per provider
- `helixagent_ensemble_votes_total` - Ensemble votes
- `helixagent_cache_hits_total` - Cache hit rate

### Grafana Dashboards

Access Grafana at `http://localhost:3000` (default credentials: admin/admin).

Pre-configured dashboards:
- HelixAgent Overview
- Provider Performance
- Ensemble Analytics
- Cache Performance

### Logging

```yaml
logging:
  level: info              # debug, info, warn, error
  format: json             # json or text
  output: stdout           # stdout, stderr, or file path
  request_logging: true    # Log all requests
```

---

## Troubleshooting

### Common Issues

#### Connection Refused

**Symptom**: `curl: (7) Failed to connect to localhost port 8080`

**Solutions**:
1. Check if HelixAgent is running: `docker-compose ps`
2. Verify port binding: `netstat -tlnp | grep 8080`
3. Check logs: `docker-compose logs helixagent`

#### Provider Authentication Errors

**Symptom**: `401 Unauthorized` from provider

**Solutions**:
1. Verify API key in `.env`
2. Check key hasn't expired
3. Ensure correct key format
4. Test directly: `curl https://api.openai.com/v1/models -H "Authorization: Bearer $OPENAI_API_KEY"`

#### Database Connection Errors

**Symptom**: `connection refused` or `database does not exist`

**Solutions**:
1. Start PostgreSQL: `docker-compose up -d postgres`
2. Check connection: `psql -h localhost -U helixagent -d helixagent`
3. Initialize database: `make db-migrate`

#### Redis Connection Errors

**Symptom**: `dial tcp: connection refused`

**Solutions**:
1. Start Redis: `docker-compose up -d redis`
2. Verify connection: `redis-cli ping`
3. Check Redis password in `.env`

#### High Latency

**Symptom**: Slow response times

**Solutions**:
1. Enable caching in configuration
2. Reduce `min_providers` for ensemble
3. Use faster models (e.g., gpt-3.5-turbo instead of gpt-4)
4. Check provider latency: `curl http://localhost:7061/api/v1/verifier/health/providers`

#### Memory Issues

**Symptom**: Out of memory errors

**Solutions**:
1. Reduce `max_entries` in cache config
2. Limit concurrent requests with rate limiting
3. Increase container memory limits

### Debug Mode

Enable debug mode for detailed logging:

```bash
# Environment variable
export GIN_MODE=debug

# Or in configuration
server:
  debug: true

logging:
  level: debug
```

### Getting Help

- GitHub Issues: https://dev.helix.agent/issues
- Documentation: https://helixagent.ai/docs
- Discord: https://discord.gg/helixagent

---

## Glossary

| Term | Definition |
|------|------------|
| **Ensemble** | Combining responses from multiple LLMs using voting strategies |
| **Provider** | An LLM service (OpenAI, Anthropic, etc.) |
| **Confidence-Weighted Voting** | Ensemble strategy that weighs responses by confidence scores |
| **Circuit Breaker** | Pattern to prevent cascading failures |
| **SSE** | Server-Sent Events for streaming responses |
| **MCP** | Model Context Protocol for AI tool integration |
| **LSP** | Language Server Protocol for IDE features |
| **RAG** | Retrieval-Augmented Generation |
| **TTL** | Time-To-Live for cache entries |
| **JWT** | JSON Web Token for authentication |
| **Cognee** | Knowledge graph integration for contextual responses |
| **LLMsVerifier** | Model verification and scoring system |

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | Initial Release | Core features, 7 providers |
| 1.1.0 | AI Debate system, LLMsVerifier integration |
| 1.2.0 | Optimization framework, Cognee integration |
| 1.3.0 | Cloud providers (AWS, GCP, Azure) |

---

*Last updated: January 2026*
