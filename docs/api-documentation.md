# SuperAgent API Documentation

## Overview

SuperAgent provides a unified OpenAI-compatible API that aggregates responses from multiple LLM providers (DeepSeek, Qwen, OpenRouter, Claude, Gemini) and offers intelligent ensemble capabilities. The system includes advanced features like Model Context Protocol (MCP) support, Language Server Protocol (LSP) integration, intelligent tool orchestration, context management, and security sandboxing.

## Base URL

```
Development: http://localhost:8080
Production: https://api.yourdomain.com
```

## Authentication

SuperAgent uses JWT-based authentication for secure access:

```bash
# Get JWT token
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "your-username",
    "password": "your-password"
  }'

# Use token in requests
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/v1/models
```

## Available Models

SuperAgent supports 22+ models from multiple providers:

### Ensemble Models
- `superagent-ensemble` - Intelligent multi-provider aggregation with confidence-weighted voting

### DeepSeek Models
- `deepseek-chat` - General purpose chat model
- `deepseek-coder` - Code generation and analysis model

### Qwen Models  
- `qwen-turbo` - Fast, cost-effective model
- `qwen-plus` - Balanced performance and cost
- `qwen-max` - Highest quality responses

### OpenRouter Models
- `openrouter/grok-4` - Latest Grok model
- `openrouter/gemini-2.5` - Google's Gemini 2.5
- `openrouter/anthropic/claude-3.5-sonnet` - Advanced reasoning
- `openrouter/openai/gpt-4o` - OpenAI's GPT-4 Omni
- `openrouter/meta-llama/llama-3.1-405b` - Meta's LLaMA 3.1
- `openrouter/mistralai/mistral-large` - Mistral's large model
- `openrouter/meta-llama/llama-3.1-70b` - LLaMA 3.1 70B
- `openrouter/google/gemma-2-27b` - Google's Gemma 2
- `openrouter/openai/gpt-4-turbo` - GPT-4 Turbo
- `openrouter/microsoft/wizardlm-2-8x22b` - Microsoft's WizardLM
- `openrouter/anthropic/claude-3.5-haiku` - Claude 3.5 Haiku
- `openrouter/meta-llama/llama-3.1-8b` - LLaMA 3.1 8B
- `openrouter/microsoft/wizardlm-2-7b` - WizardLM 7B
- `openrouter/qwen/qwen-2-72b` - Qwen 2 72B
- `openrouter/openai/gpt-4o-mini` - GPT-4 Mini
- `openrouter/google/gemini-flash-1.5` - Gemini Flash

## API Endpoints

### 1. Health Check

#### GET /health
Basic health check endpoint.

**Response:**
```json
{
  "status": "ok",
  "timestamp": 1703123456,
  "version": "1.0.0"
}
```

#### GET /v1/health  
Enhanced health check with provider status.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": 1703123456,
  "providers": {
    "deepseek": {"status": "healthy", "response_time": 0.8},
    "qwen": {"status": "healthy", "response_time": 0.6},
    "openrouter": {"status": "healthy", "response_time": 1.2}
  },
  "ensemble": {"available": true, "providers": 3}
}
```

### 2. Available Models

#### GET /v1/models
List all available models.

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "superagent-ensemble",
      "object": "model",
      "created": 1703123456,
      "owned_by": "superagent"
    },
    {
      "id": "deepseek-chat",
      "object": "model", 
      "created": 1703123456,
      "owned_by": "deepseek"
    }
  ]
}
```

### 3. Chat Completions

#### POST /v1/chat/completions
Create chat completion with streaming support.

**Request Body:**
```json
{
  "model": "superagent-ensemble",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful AI assistant."
    },
    {
      "role": "user", 
      "content": "Explain quantum computing in simple terms."
    }
  ],
  "max_tokens": 1000,
  "temperature": 0.7,
  "stream": false
}
```

**Response:**
```json
{
  "id": "chatcmpl-superagent-123",
  "object": "chat.completion",
  "created": 1703123456,
  "model": "superagent-ensemble",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Quantum computing is like..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 75,
    "total_tokens": 100
  },
  "ensemble": {
    "providers_used": ["deepseek", "qwen", "openrouter"],
    "confidence_score": 0.92,
    "voting_strategy": "confidence_weighted"
  }
}
```

**Streaming Response:**
```json
data: {"id": "chatcmpl-123", "object": "chat.completion.chunk", ...}
data: {"id": "chatcmpl-123", "object": "chat.completion.chunk", ...}
data: [DONE]
```

### 4. Text Completions

#### POST /v1/completions
Legacy text completion endpoint.

**Request Body:**
```json
{
  "model": "deepseek-chat",
  "prompt": "The future of artificial intelligence is",
  "max_tokens": 100,
  "temperature": 0.5,
  "stream": false
}
```

**Response:**
```json
{
  "id": "cmpl-deepseek-456",
  "object": "text_completion",
  "created": 1703123456,
  "model": "deepseek-chat",
  "choices": [
    {
      "text": "...full of exciting possibilities.",
      "index": 0,
      "logprobs": null,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 15,
    "total_tokens": 25
  }
}
```

### 5. Text Embeddings

#### POST /v1/embeddings
Generate text embeddings for semantic search.

**Request Body:**
```json
{
  "model": "deepseek-chat",
  "input": "The quick brown fox jumps over the lazy dog",
  "encoding_format": "float"
}
```

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "embedding": [0.1, 0.2, 0.3, ...],
      "index": 0
    }
  ],
  "model": "deepseek-chat",
  "usage": {
    "prompt_tokens": 9,
    "total_tokens": 9
  }
}
```

### 6. Provider Information

#### GET /v1/providers
List all configured LLM providers.

**Response:**
```json
{
  "providers": [
    {
      "id": "deepseek",
      "name": "DeepSeek",
      "type": "openai_compatible",
      "status": "healthy",
      "models": ["deepseek-chat", "deepseek-coder"],
      "capabilities": ["text_completion", "chat", "embeddings"],
      "supports_streaming": true
    },
    {
      "id": "qwen", 
      "name": "Qwen",
      "type": "openai_compatible",
      "status": "healthy",
      "models": ["qwen-turbo", "qwen-plus", "qwen-max"],
      "capabilities": ["text_completion", "chat", "embeddings"],
      "supports_streaming": true
    }
  ]
}
```

#### GET /v1/providers/health
Check health status of all providers.

**Response:**
```json
{
  "status": "healthy",
  "providers": {
    "deepseek": {
      "status": "healthy",
      "response_time": 0.8,
      "last_check": "2024-01-15T10:30:00Z",
      "error_rate": 0.01
    },
    "qwen": {
      "status": "healthy", 
      "response_time": 0.6,
      "last_check": "2024-01-15T10:30:00Z",
      "error_rate": 0.02
    }
  }
}
```

### 7. Ensemble Configuration

#### POST /v1/ensemble/configure
Configure ensemble behavior for intelligent response aggregation.

**Request Body:**
```json
{
  "strategy": "confidence_weighted",
  "min_providers": 2,
  "confidence_threshold": 0.7,
  "timeout": 30,
  "providers": ["deepseek", "qwen", "openrouter"],
  "weights": {
    "deepseek": 0.4,
    "qwen": 0.3, 
    "openrouter": 0.3
  }
}
```

**Response:**
```json
{
  "status": "configured",
  "ensemble_id": "ensemble-123",
  "configuration": {
    "strategy": "confidence_weighted",
    "min_providers": 2,
    "confidence_threshold": 0.7,
    "providers": ["deepseek", "qwen", "openrouter"]
  }
}
```

## Error Handling

### Error Response Format

```json
{
  "error": {
    "message": "Invalid model specified",
    "type": "invalid_request_error", 
    "code": "invalid_model",
    "param": "model",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

### Common Error Codes

| Code | Description |
|------|-------------|
| `invalid_model` | Model not found or not supported |
| `invalid_request` | Request validation failed |
| `missing_api_key` | API key required but not provided |
| `invalid_api_key` | API key is invalid or expired |
| `rate_limit_exceeded` | Rate limit exceeded |
| `insufficient_quota` | API quota exceeded |
| `model_overloaded` | Model is currently overloaded |
| `provider_error` | LLM provider returned an error |
| `ensemble_failed` | Ensemble processing failed |

## Rate Limiting

SuperAgent implements intelligent rate limiting:

- **Anonymous requests**: 100 requests/minute
- **Authenticated users**: 1000 requests/minute  
- **Premium users**: 10000 requests/minute

Rate limit headers are included in responses:

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1703123456
```

## Streaming

All chat and text completion endpoints support streaming:

**Request:**
```json
{
  "model": "superagent-ensemble",
  "messages": [{"role": "user", "content": "Hello"}],
  "stream": true
}
```

**Response Format:**
```
data: {"id": "chatcmpl-123", "object": "chat.completion.chunk", "choices": [{"delta": {"role": "assistant"}}]}

data: {"id": "chatcmpl-123", "object": "chat.completion.chunk", "choices": [{"delta": {"content": "Hello"}}]}

data: {"id": "chatcmpl-123", "object": "chat.completion.chunk", "choices": [{"delta": {}, "finish_reason": "stop"}]}

data: [DONE]
```

## SDK Examples

### Go SDK
```go
package main

import (
    "context"
    "fmt"
    "github.com/superagent/superagent-go"
)

func main() {
    client := superagent.NewClient("your-api-key")
    
    resp, err := client.CreateChatCompletion(context.Background(), &superagent.ChatCompletionRequest{
        Model: "superagent-ensemble",
        Messages: []superagent.Message{
            {Role: "user", Content: "Hello, how are you?"},
        },
    })
    
    if err != nil {
        panic(err)
    }
    
    fmt.Println(resp.Choices[0].Message.Content)
}
```

### Python SDK
```python
from superagent import SuperAgentClient

client = SuperAgentClient(api_key="your-api-key")

response = client.chat.completions.create(
    model="superagent-ensemble",
    messages=[
        {"role": "user", "content": "Hello, how are you?"}
    ]
)

print(response.choices[0].message.content)
```

### JavaScript SDK
```javascript
import { SuperAgentClient } from '@superagent/client';

const client = new SuperAgentClient({ apiKey: 'your-api-key' });

const response = await client.chat.completions.create({
    model: 'superagent-ensemble',
    messages: [
        { role: 'user', content: 'Hello, how are you?' }
    ]
});

console.log(response.choices[0].message.content);
```

## Monitoring & Metrics

### Prometheus Metrics

SuperAgent exposes comprehensive metrics at `/metrics`:

- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request duration histogram
- `llm_requests_total` - LLM provider requests
- `llm_response_time_seconds` - Provider response times
- `llm_error_rate_total` - Error rates by provider
- `ensemble_requests_total` - Ensemble processing requests

### Grafana Dashboard

Import the pre-configured dashboard:

```bash
curl -X POST http://admin:admin@localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @monitoring/dashboards/superagent-dashboard.json
```

## Advanced Services

SuperAgent includes several advanced services that enhance LLM interactions and provide enterprise-grade capabilities. These services are currently available as internal APIs and will be exposed via REST endpoints in future releases.

### 1. Model Context Protocol (MCP)

The MCP service enables seamless integration with external tools and services through a standardized protocol.

**Features:**
- Auto-discovery of MCP servers
- Tool registration and management
- Health monitoring and failover
- Secure tool execution

**Current Status:** Internal service, REST API integration planned

### 2. Language Server Protocol (LSP)

LSP integration provides advanced code intelligence capabilities.

**Features:**
- Workspace symbols and references
- Code completion with context awareness
- Refactoring support
- Multi-language support (Go, Python, JavaScript, etc.)

**Current Status:** Internal service, REST API integration planned

### 3. Tool Registry & Orchestration

Intelligent tool management and execution orchestration.

**Features:**
- Dynamic tool discovery and registration
- Tool validation and security checks
- Parallel tool execution
- Dependency management and cycle detection

**Current Status:** Internal service, REST API integration planned

### 4. Context Management

Advanced context handling with ML-based relevance scoring.

**Features:**
- Multi-source context aggregation
- Relevance scoring and ranking
- Context compression and optimization
- Conflict detection and resolution

**Current Status:** Internal service, REST API integration planned

### 5. Security Sandbox

Secure execution environment for tool operations.

**Features:**
- Docker containerization
- Resource limits and isolation
- Command validation and sanitization
- Audit logging and monitoring

**Current Status:** Internal service, REST API integration planned

### 6. Integration Orchestrator

Workflow orchestration for complex multi-step operations.

**Features:**
- Code analysis workflows
- Tool chain execution
- Parallel processing
- Error handling and recovery

**Current Status:** Internal service, REST API integration planned

## Best Practices

### 1. Model Selection
- Use `superagent-ensemble` for most reliable responses
- Choose specific models for cost optimization
- Consider response time requirements

### 2. Error Handling
- Always handle `model_overloaded` errors with retry
- Implement exponential backoff for rate limits
- Monitor ensemble confidence scores

### 3. Performance Optimization
- Enable streaming for long responses
- Use appropriate `max_tokens` limits
- Batch requests when possible

### 4. Security
- Use JWT tokens for authentication
- Validate input parameters
- Implement rate limiting client-side

---

For more detailed information, see the [SuperAgent GitHub repository](https://github.com/superagent/superagent).