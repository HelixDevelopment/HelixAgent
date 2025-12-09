# SuperAgent LLM Facade API Documentation

## Overview

SuperAgent is a production-ready LLM facade system that provides unified access to multiple LLM providers through intelligent ensemble voting, streaming support, and comprehensive API compatibility.

## Base URL
```
https://api.superagent.ai/v1
```

## Authentication

SuperAgent uses JWT-based authentication with Bearer tokens.

### Login
```http
POST /v1/auth/login
Content-Type: application/json

{
  "username": "string",
  "password": "string"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 86400,
  "user": {
    "id": "user123",
    "username": "john_doe",
    "role": "user"
  }
}
```

### Refresh Token
```http
POST /v1/auth/refresh
Authorization: Bearer <token>
```

### Logout
```http
POST /v1/auth/logout
Authorization: Bearer <token>
```

### Get User Info
```http
GET /v1/auth/me
Authorization: Bearer <token>
```

## Completions API

### Standard Completion
Generate text completions using ensemble voting across multiple providers.

```http
POST /v1/completions
Authorization: Bearer <token>
Content-Type: application/json

{
  "prompt": "Write a Python function to calculate fibonacci numbers",
  "model": "claude-3-sonnet-20240229",
  "temperature": 0.7,
  "max_tokens": 1000,
  "top_p": 1.0,
  "stop": ["\n\n", "###"],
  "ensemble_config": {
    "strategy": "confidence_weighted",
    "min_providers": 2,
    "confidence_threshold": 0.8,
    "fallback_to_best": true,
    "preferred_providers": ["claude", "deepseek"]
  }
}
```

**Response:**
```json
{
  "id": "cmpl-1234567890",
  "object": "text_completion",
  "created": 1677652288,
  "model": "claude-3-sonnet-20240229",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "def fibonacci(n):\n    if n <= 1:\n        return n\n    return fibonacci(n-1) + fibonacci(n-2)"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 35,
    "total_tokens": 50
  }
}
```

### Streaming Completion
```http
POST /v1/completions/stream
Authorization: Bearer <token>
Content-Type: application/json

{
  "prompt": "Write a comprehensive guide to Go concurrency",
  "model": "deepseek-coder",
  "stream": true,
  "temperature": 0.8,
  "max_tokens": 2000
}
```

**Streaming Response:**
```json
data: {"id": "cmpl-123", "object": "text_completion", "choices": [{"index": 0, "delta": {"content": "Go"}}]}

data: {"id": "cmpl-123", "object": "text_completion", "choices": [{"index": 0, "delta": {"content": " concurrency"}}]}

data: [DONE]
```

## Chat API

### Chat Completion
```http
POST /v1/chat/completions
Authorization: Bearer <token>
Content-Type: application/json

{
  "model": "gpt-3.5-turbo",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful coding assistant."
    },
    {
      "role": "user",
      "content": "Explain goroutines in Go"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 1000,
  "memory_enhanced": true
}
```

**Response:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "claude-3-sonnet-20240229",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Goroutines are lightweight threads managed by the Go runtime..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 150,
    "total_tokens": 175
  }
}
```

### Streaming Chat
```http
POST /v1/chat/completions/stream
Authorization: Bearer <token>
Content-Type: application/json

{
  "model": "claude-3-sonnet-20240229",
  "messages": [
    {"role": "user", "content": "Write a React component"}
  ],
  "stream": true,
  "temperature": 0.7
}
```

## Ensemble API

### Ensemble Completion
Use multiple providers with intelligent voting.

```http
POST /v1/ensemble/completions
Authorization: Bearer <token>
Content-Type: application/json

{
  "prompt": "Design a microservices architecture",
  "ensemble_config": {
    "strategy": "confidence_weighted",
    "min_providers": 3,
    "confidence_threshold": 0.85,
    "preferred_providers": ["claude", "deepseek", "gemini"]
  }
}
```

**Response:**
```json
{
  "id": "ensemble-123",
  "object": "ensemble.completion",
  "created": 1677652288,
  "model": "claude-3-sonnet-20240229",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Microservices architecture involves..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 20,
    "completion_tokens": 200,
    "total_tokens": 220
  },
  "ensemble": {
    "voting_method": "confidence_weighted",
    "responses_count": 3,
    "scores": {
      "claude": 0.92,
      "deepseek": 0.88,
      "gemini": 0.85
    },
    "metadata": {
      "total_providers": 3,
      "successful_providers": 3,
      "failed_providers": 0,
      "execution_time": 1250
    },
    "selected_provider": "claude",
    "selection_score": 0.92
  }
}
```

## Models API

### List Available Models
```http
GET /v1/models
```

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "deepseek-coder",
      "object": "model",
      "created": 1677652288,
      "owned_by": "deepseek",
      "permission": "code_generation",
      "root": "deepseek",
      "parent": null
    },
    {
      "id": "claude-3-sonnet-20240229",
      "object": "model",
      "created": 1677652288,
      "owned_by": "anthropic",
      "permission": "reasoning",
      "root": "claude",
      "parent": null
    }
  ]
}
```

## Provider Management API

### List Providers
```http
GET /v1/providers
```

**Response:**
```json
{
  "providers": [
    {
      "name": "claude",
      "supported_models": ["claude-3-sonnet-20240229", "claude-3-opus-20240229"],
      "supported_features": ["streaming", "function_calling"],
      "supports_streaming": true,
      "supports_function_calling": true,
      "supports_vision": false,
      "metadata": {
        "version": "1.0.0",
        "region": "us-west-2"
      }
    }
  ],
  "count": 3
}
```

### Provider Health
```http
GET /v1/providers/{name}/health
```

**Response:**
```json
{
  "provider": "claude",
  "healthy": true
}
```

## Health & Monitoring

### System Health
```http
GET /v1/health
```

**Response:**
```json
{
  "status": "healthy",
  "providers": {
    "total": 3,
    "healthy": 3,
    "unhealthy": 0
  },
  "timestamp": 1677652288
}
```

### Metrics
```http
GET /metrics
```

Returns Prometheus metrics for monitoring.

## Error Handling

All APIs return standard HTTP status codes and error responses:

```json
{
  "error": {
    "message": "Invalid request format",
    "type": "invalid_request",
    "code": "400"
  }
}
```

## Rate Limiting

- **Authenticated requests**: 1000 requests/hour per user
- **Anonymous requests**: 100 requests/hour per IP
- **Streaming requests**: 10 concurrent streams per user

Rate limit headers are included in responses:
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1677652288
```

## Memory Enhancement

Enable memory-enhanced responses by setting `memory_enhanced: true`:

```json
{
  "prompt": "Continue the story about the dragon",
  "memory_enhanced": true,
  "messages": [
    {"role": "user", "content": "Once upon a time there was a dragon..."}
  ]
}
```

This integrates with Cognee for context-aware responses.

## Streaming Protocol

Streaming responses use Server-Sent Events format:

```
data: {"id": "cmpl-123", "object": "text_completion", "choices": [{"index": 0, "delta": {"content": "Hello"}}]}

data: {"id": "cmpl-123", "object": "text_completion", "choices": [{"index": 0, "delta": {"content": " world"}}]}

data: [DONE]
```

## SDK Examples

### Go Client
```go
client := superagent.NewClient("your-api-key")

response, err := client.Completions(context.Background(), superagent.CompletionRequest{
    Prompt: "Write a Go function",
    Model:  "claude-3-sonnet-20240229",
    Temperature: 0.7,
})
```

### Python Client
```python
import superagent

client = superagent.Client("your-api-key")
response = client.completions.create(
    prompt="Write a Python function",
    model="deepseek-coder",
    temperature=0.7
)
```

## WebSocket Support

For real-time applications, WebSocket connections are available:

```javascript
const ws = new WebSocket('wss://api.superagent.ai/v1/ws/completions');

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    if (data.choices[0].delta.content) {
        console.log(data.choices[0].delta.content);
    }
};

ws.send(JSON.stringify({
    prompt: "Tell me a story",
    stream: true,
    model: "claude-3-sonnet-20240229"
}));
```

## Configuration

### Environment Variables
- `SUPERAGENT_API_KEY`: Your API key
- `JWT_SECRET`: JWT signing secret
- `DB_HOST`: PostgreSQL host
- `REDIS_HOST`: Redis host
- `COGNEE_API_KEY`: Cognee API key

### Provider Configuration
Configure provider API keys and settings through the admin interface or environment variables.

## Security

- All API calls require authentication
- TLS 1.3 encryption for all connections
- Input validation and sanitization
- Rate limiting and abuse prevention
- Audit logging for all requests
- Secure credential storage

## Support

For support and documentation:
- API Documentation: https://docs.superagent.ai
- Community Forum: https://community.superagent.ai
- GitHub Issues: https://github.com/superagent/superagent/issues
- Email Support: support@superagent.ai