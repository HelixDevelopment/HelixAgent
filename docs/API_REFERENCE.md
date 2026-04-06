# HelixAgent API Reference

> **Full API documentation:** See [docs/api/API_REFERENCE.md](api/API_REFERENCE.md) for the
> comprehensive reference with all endpoints, request/response examples, and detailed descriptions.

## Base URL

```
http://localhost:7061
```

## Authentication

Most endpoints require Bearer token authentication:

```
Authorization: Bearer <your-jwt-token>
```

## Endpoints Overview

### Health

#### GET /health
Basic health check.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:30:00Z"
}
```

#### GET /v1/health
Detailed health with provider status.

**Response:**
```json
{
  "status": "healthy",
  "providers": {
    "openai": "available",
    "anthropic": "available",
    "deepseek": "unavailable"
  },
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### Models

#### GET /v1/models
List all available models.

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "owned_by": "openai"
    },
    {
      "id": "claude-3-5-sonnet-20241022",
      "object": "model", 
      "owned_by": "anthropic"
    }
  ]
}
```

#### GET /v1/providers
List configured providers.

**Response:**
```json
{
  "providers": [
    {
      "name": "openai",
      "models": ["gpt-4o", "gpt-4o-mini"],
      "status": "active"
    },
    {
      "name": "anthropic",
      "models": ["claude-3-5-sonnet-20241022"],
      "status": "active"
    }
  ]
}
```

### Chat Completions

#### POST /v1/chat/completions
Create a chat completion (OpenAI-compatible).

**Request:**
```json
{
  "model": "gpt-4o-mini",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "max_tokens": 100,
  "temperature": 0.7,
  "stream": false
}
```

**Response:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-4o-mini",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "Hello! How can I help you today?"
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 12,
    "total_tokens": 21
  }
}
```

#### POST /v1/chat/completions (Streaming)

**Request:**
```json
{
  "model": "gpt-4o-mini",
  "messages": [{"role": "user", "content": "Count to 5"}],
  "stream": true
}
```

**Response:** (SSE stream)
```
data: {"id":"...","choices":[{"delta":{"content":"1"}}]}

data: {"id":"...","choices":[{"delta":{"content":", 2"}}]}

data: [DONE]
```

### Tool Calling

#### POST /v1/chat/completions with Tools

**Request:**
```json
{
  "model": "gpt-4o",
  "messages": [{"role": "user", "content": "What's the weather in Paris?"}],
  "tools": [{
    "type": "function",
    "function": {
      "name": "get_weather",
      "description": "Get weather for a location",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {"type": "string"}
        },
        "required": ["location"]
      }
    }
  }]
}
```

**Response:**
```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [{
        "id": "call_123",
        "type": "function",
        "function": {
          "name": "get_weather",
          "arguments": "{\"location\":\"Paris\"}"
        }
      }]
    }
  }]
}
```

### Ensemble

#### POST /v1/ensemble/completions
Aggregate responses from multiple providers.

**Request:**
```json
{
  "messages": [{"role": "user", "content": "What is 2+2?"}],
  "providers": ["openai", "anthropic", "deepseek"],
  "strategy": "majority_vote"
}
```

**Response:**
```json
{
  "ensemble_id": "ens_123",
  "consensus": "4",
  "responses": [
    {"provider": "openai", "response": "4", "confidence": 0.95},
    {"provider": "anthropic", "response": "4", "confidence": 0.98},
    {"provider": "deepseek", "response": "4", "confidence": 0.97}
  ],
  "agreement_score": 1.0
}
```

### Embeddings

#### POST /v1/embeddings

**Request:**
```json
{
  "input": "The quick brown fox",
  "model": "text-embedding-3-small"
}
```

**Response:**
```json
{
  "object": "list",
  "data": [{
    "object": "embedding",
    "embedding": [0.0023, -0.0091, ...],
    "index": 0
  }]
}
```

### Vision

#### POST /v1/vision
Analyze images.

**Request:**
```json
{
  "model": "gpt-4o",
  "messages": [{
    "role": "user",
    "content": [
      {"type": "text", "text": "What's in this image?"},
      {"type": "image_url", "image_url": {"url": "data:image/png;base64,..."}}
    ]
  }]
}
```

### Debate

#### POST /v1/debate
Start a debate between models.

**Request:**
```json
{
  "topic": "Should AI be regulated?",
  "participants": ["claude", "gpt-4", "deepseek"],
  "rounds": 3,
  "topology": "mesh"
}
```

### MCP

#### GET /v1/mcp/servers
List MCP servers.

#### POST /v1/mcp/tools/call
Call an MCP tool.

**Request:**
```json
{
  "server": "filesystem",
  "tool": "read_file",
  "arguments": {"path": "/tmp/test.txt"}
}
```

### Additional Endpoint Groups

The following endpoint groups are fully documented in [docs/api/API_REFERENCE.md](api/API_REFERENCE.md):

| Group | Base Path | Description |
|-------|-----------|-------------|
| Authentication | `/v1/auth/*` | Register, login, refresh, logout |
| Sessions | `/v1/sessions/*` | Conversation session management |
| Features | `/v1/features/*` | Feature flags and agent capabilities |
| Agents | `/v1/agents/*` | CLI agent registry (48 agents) |
| Ensemble | `/v1/ensemble/*` | Multi-provider ensemble sessions and teams |
| Completion | `/v1/completion/*` | Skills-enhanced completion with intent routing |
| Agentic | `/v1/agentic/workflows/*` | Graph-based workflow orchestration |
| Planning | `/v1/planning/*` | HiPlan, MCTS, Tree of Thoughts |
| LLMOps | `/v1/llmops/*` | A/B experiments, evaluation, prompts |
| Benchmark | `/v1/benchmark/*` | SWE-bench, HumanEval, MMLU |
| Discovery | `/v1/discovery/*` | 3-tier dynamic model discovery |
| Scoring | `/v1/scoring/*` | 5-component weighted scoring |
| Verification | `/v1/verification/*` | 8-test verification pipeline |
| Health Monitoring | `/v1/health/*` | Provider health, circuit breakers, latency |
| Monitoring | `/v1/monitoring/*` | OAuth, concurrency, fallback chain |
| Cognee | `/v1/cognee/*` | Knowledge graph, memory, cognification |
| RAG | `/v1/rag/*` | Document ingestion, hybrid search, reranking |
| Embeddings | `/v1/embeddings/*` | Generate, search, batch index |
| MCP | `/v1/mcp/*` | MCP tools, prompts, resources, search |
| ACP | `/v1/acp/*` | Agent Communication Protocol |
| LSP | `/v1/lsp/*` | Language Server Protocol integration |
| Vision | `/v1/vision/*` | Image analysis, OCR, detection |
| Protocols | `/v1/protocols/*` | Unified protocol management |
| Formatters | `/v1/format*`, `/v1/formatters*` | Code formatting (32+ languages) |
| Search | `/v1/search/*` | Semantic code search |
| Templates | `/v1/templates/*` | Reusable prompt templates |
| Checkpoints | `/v1/checkpoints/*` | Workspace snapshots |
| Browser | `/v1/browser/*` | Playwright web automation |
| Skills | `/v1/skills/*` | Skill registry |
| Tasks | `/v1/tasks/*` | Background task management |
| Webhooks | `/v1/webhooks/*` | Webhook management |
| QA | `/v1/qa/*` | HelixQA autonomous testing |
| Debates | `/v1/debates/*` | AI debate ensemble |
| GraphQL | `/v1/graphql` | GraphQL (feature-flagged) |
| Model Metadata | `/v1/models/metadata/*` | Models.dev integration |
| Startup | `/v1/startup/verification` | Startup verification status |
| BigData | `/v1/bigdata/components` | BigData component status |
| Metrics | `/metrics` | Prometheus metrics |
| Debug | `/debug/pprof/*` | pprof profiling (when enabled) |

---

## Error Responses

All errors follow this format:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "Invalid API key",
    "type": "authentication_error"
  }
}
```

Common HTTP status codes:
- `200` - Success
- `400` - Bad Request
- `401` - Unauthorized
- `429` - Rate Limited
- `500` - Internal Server Error

## Rate Limits

Default rate limits per provider:
- OpenAI: 60 RPM
- Anthropic: 50 RPM
- Groq: 100 RPM
- Others: 30 RPM

Headers returned:
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 59
X-RateLimit-Reset: 1677652345
```
