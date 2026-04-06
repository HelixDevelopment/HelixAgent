# HelixAgent API Documentation

**Version:** 1.3.0  
**Base URL:** `http://localhost:7061`  
**Protocol:** HTTP/3 (QUIC) with Brotli compression

---

## Table of Contents

1. [Authentication](#authentication)
2. [LLM Endpoints](#llm-endpoints)
3. [Debate Endpoints](#debate-endpoints)
4. [Provider Endpoints](#provider-endpoints)
5. [MCP Endpoints](#mcp-endpoints)
6. [Monitoring Endpoints](#monitoring-endpoints)
7. [Embeddings Endpoints](#embeddings-endpoints)
8. [Memory Endpoints](#memory-endpoints)
9. [RAG Endpoints](#rag-endpoints)
10. [LSP Endpoints](#lsp-endpoints)
11. [ACP Endpoints](#acp-endpoints)
12. [Vision Endpoints](#vision-endpoints)
13. [Formatters Endpoints](#formatters-endpoints)
14. [Ensemble Endpoints](#ensemble-endpoints)
15. [Completion Endpoints](#completion-endpoints)
16. [Agentic Workflow Endpoints](#agentic-workflow-endpoints)
17. [Planning Endpoints](#planning-endpoints)
18. [LLMOps Endpoints](#llmops-endpoints)
19. [Benchmark Endpoints](#benchmark-endpoints)
20. [Discovery Endpoints](#discovery-endpoints)
21. [Scoring Endpoints](#scoring-endpoints)
22. [Verification Endpoints](#verification-endpoints)
23. [Health Monitoring Endpoints](#health-monitoring-endpoints)
24. [Cognee Endpoints](#cognee-endpoints)
25. [Search Endpoints](#search-endpoints)
26. [Templates Endpoints](#templates-endpoints)
27. [Checkpoints Endpoints](#checkpoints-endpoints)
28. [Browser Automation Endpoints](#browser-automation-endpoints)
29. [Skills Endpoints](#skills-endpoints)
30. [QA Endpoints](#qa-endpoints)
31. [Background Tasks Endpoints](#background-tasks-endpoints)
32. [Sessions Endpoints](#sessions-endpoints)
33. [Features Endpoints](#features-endpoints)
34. [GraphQL Endpoint](#graphql-endpoint)
35. [Error Handling](#error-handling)

---

## Authentication

All API requests require authentication via Bearer token or API key.

### Headers

```http
Authorization: Bearer <your-api-key>
Content-Type: application/json
```

### API Key Management

```http
POST /v1/api-keys
GET /v1/api-keys
DELETE /v1/api-keys/{key_id}
```

---

## LLM Endpoints

### Chat Completion

**POST** `/v1/chat/completions`

OpenAI-compatible chat completion endpoint.

**Request Body:**

```json
{
  "model": "string (required)",
  "messages": [
    {
      "role": "system|user|assistant",
      "content": "string"
    }
  ],
  "temperature": "number (0-2, default: 1)",
  "max_tokens": "integer (default: 4096)",
  "stream": "boolean (default: false)",
  "provider": "string (optional)",
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "string",
        "description": "string",
        "parameters": {}
      }
    }
  ]
}
```

**Response:**

```json
{
  "id": "string",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "string",
  "provider": "string",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "string"
      },
      "finish_reason": "stop|length|tool_calls"
    }
  ],
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 50,
    "total_tokens": 150
  }
}
```

### Stream Chat Completion

**POST** `/v1/chat/completions` with `stream: true`

Returns Server-Sent Events (SSE):

```
data: {"id":"chunk-1","choices":[{"delta":{"content":"Hello"}}]}

data: {"id":"chunk-2","choices":[{"delta":{"content":" world"}}]}

data: [DONE]
```

### List Models

**GET** `/v1/models`

**Response:**

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4",
      "object": "model",
      "created": 1234567890,
      "owned_by": "openai",
      "provider": "openai"
    }
  ]
}
```

---

## Debate Endpoints

### Conduct Debate

**POST** `/v1/debate/conduct`

**Request Body:**

```json
{
  "topic": "string (required)",
  "context": "string (optional)",
  "config": {
    "max_rounds": "integer (default: 3)",
    "participants": ["string"],
    "voting_method": "weighted|majority|borda|condorcet",
    "model": "string (default: provider default)"
  }
}
```

**Response:**

```json
{
  "debate_id": "string",
  "status": "completed|in_progress|failed",
  "result": {
    "winner": "string",
    "consensus": "string",
    "turns": [
      {
        "round": 1,
        "participant": "string",
        "content": "string"
      }
    ]
  },
  "scores": {
    "participant_name": 0.95
  }
}
```

### Get Debate Status

**GET** `/v1/debate/{debate_id}/status`

### Get Debate Result

**GET** `/v1/debate/{debate_id}/result`

### List Debates

**GET** `/v1/debates`

---

## Provider Endpoints

### List Providers

**GET** `/v1/providers`

**Response:**

```json
{
  "providers": [
    {
      "id": "openai",
      "name": "OpenAI",
      "enabled": true,
      "models": ["gpt-4", "gpt-3.5-turbo"],
      "score": 95.5,
      "status": "healthy"
    }
  ]
}
```

### Get Provider

**GET** `/v1/providers/{provider_id}`

### Provider Health Check

**GET** `/v1/providers/{provider_id}/health`

---

## MCP Endpoints

### Execute MCP Tool

**POST** `/v1/mcp`

**Request Body:**

```json
{
  "server": "string (required)",
  "tool": "string (required)",
  "arguments": {}
}
```

### List MCP Servers

**GET** `/v1/mcp/servers`

### List MCP Tools

**GET** `/v1/mcp/tools`

---

## Monitoring Endpoints

### System Status

**GET** `/v1/monitoring/status`

**Response:**

```json
{
  "status": "healthy",
  "version": "1.3.0",
  "uptime_seconds": 3600,
  "providers": {
    "healthy": 20,
    "total": 22
  },
  "memory": {
    "alloc_mb": 100,
    "sys_mb": 200
  },
  "goroutines": 50
}
```

### Circuit Breaker Status

**GET** `/v1/monitoring/circuit-breakers`

### Provider Health

**GET** `/v1/monitoring/provider-health`

### Metrics (Prometheus)

**GET** `/metrics`

---

## Embeddings Endpoints

### Generate Embeddings

**POST** `/v1/embeddings`

**Request Body:**

```json
{
  "model": "text-embedding-3-small",
  "input": "string or string[]",
  "encoding_format": "float|base64"
}
```

**Response:**

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.1, 0.2, 0.3]
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 10,
    "total_tokens": 10
  }
}
```

---

## Memory Endpoints

### Add Memory

**POST** `/v1/memory`

**Request Body:**

```json
{
  "content": "string",
  "metadata": {},
  "scope": "user|session|global"
}
```

### Search Memory

**GET** `/v1/memory/search?q={query}`

### Get Memory

**GET** `/v1/memory/{memory_id}`

### Delete Memory

**DELETE** `/v1/memory/{memory_id}`

---

## RAG Endpoints

### Health

**GET** `/v1/rag/health`

### Stats

**GET** `/v1/rag/stats`

### Ingest Document

**POST** `/v1/rag/documents`

**Request Body:**

```json
{
  "content": "string",
  "metadata": {},
  "chunk_strategy": "semantic"
}
```

### Batch Ingest

**POST** `/v1/rag/documents/batch`

### Delete Document

**DELETE** `/v1/rag/documents/{id}`

### Search Documents

**POST** `/v1/rag/search`

**Request Body:**

```json
{
  "query": "string",
  "limit": 10,
  "threshold": 0.7,
  "filters": {}
}
```

### Hybrid Search

**POST** `/v1/rag/search/hybrid`

**Request Body:**

```json
{
  "query": "string",
  "dense_weight": 0.7,
  "sparse_weight": 0.3,
  "limit": 10
}
```

### Search with Query Expansion

**POST** `/v1/rag/search/expanded`

### ReRank Results

**POST** `/v1/rag/rerank`

### Compress Context

**POST** `/v1/rag/compress`

### Expand Query

**POST** `/v1/rag/expand`

### Chunk Document

**POST** `/v1/rag/chunk`

---

## LSP Endpoints

### Get Diagnostics

**GET** `/v1/lsp/diagnostics?file_path={path}`

### Go to Definition

**GET** `/v1/lsp/definition?file_path={path}&line={line}&character={char}`

### Find References

**GET** `/v1/lsp/references?file_path={path}&line={line}&character={char}`

---

## ACP Endpoints

### Send Message to Agent

**POST** `/v1/acp`

**Request Body:**

```json
{
  "agent_id": "string",
  "message": "string"
}
```

### List Agents

**GET** `/v1/acp/agents`

---

## Vision Endpoints

### Analyze Image

**POST** `/v1/vision/analyze`

**Request Body:**

```json
{
  "image_url": "string",
  "prompt": "string (optional)"
}
```

### OCR

**POST** `/v1/vision/ocr`

**Request Body:**

```json
{
  "image_url": "string"
}
```

---

## Formatters Endpoints

### Format Code

**POST** `/v1/format`

**Request Body:**

```json
{
  "code": "string",
  "language": "go|python|javascript|...",
  "formatter": "gofmt|black|prettier|..."
}
```

### List Formatters

**GET** `/v1/formatters`

### Batch Format

**POST** `/v1/format/batch`

### Check Code Style

**POST** `/v1/format/check`

### Detect Formatter

**GET** `/v1/formatters/detect`

### Get Formatter Details

**GET** `/v1/formatters/{name}`

### Formatter Health

**GET** `/v1/formatters/{name}/health`

### Validate Config

**POST** `/v1/formatters/{name}/validate-config`

---

## Ensemble Endpoints

### Ensemble Completions

**POST** `/v1/ensemble/completions`

Forces multi-provider ensemble mode with voting.

### Create Session

**POST** `/v1/ensemble/sessions`

### List Sessions

**GET** `/v1/ensemble/sessions`

### Get Session

**GET** `/v1/ensemble/sessions/{id}`

### Execute Session

**POST** `/v1/ensemble/sessions/{id}/execute`

### Cancel Session

**POST** `/v1/ensemble/sessions/{id}/cancel`

### Team CRUD

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/ensemble/teams` | Create team |
| `GET` | `/v1/ensemble/teams` | List teams |
| `GET` | `/v1/ensemble/teams/{id}` | Get team |
| `PUT` | `/v1/ensemble/teams/{id}` | Update team |
| `DELETE` | `/v1/ensemble/teams/{id}` | Delete team |
| `POST` | `/v1/ensemble/teams/{id}/agents` | Add agent |
| `DELETE` | `/v1/ensemble/teams/{id}/agents/{agentId}` | Remove agent |
| `POST` | `/v1/ensemble/teams/{id}/execute` | Execute team |

---

## Completion Endpoints

Skills-enhanced completions with intent-based routing.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/completion` | Single completion |
| `POST` | `/v1/completion/stream` | Streaming completion |
| `POST` | `/v1/completion/chat` | Chat completion |
| `POST` | `/v1/completion/chat/stream` | Streaming chat |
| `GET` | `/v1/completion/models` | List models |

---

## Agentic Workflow Endpoints

Graph-based workflow orchestration.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/agentic/workflows` | Create and execute workflow |
| `GET` | `/v1/agentic/workflows/{id}` | Get workflow status |

---

## Planning Endpoints

AI planning algorithms: HiPlan, MCTS, Tree of Thoughts.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/planning/hiplan` | Hierarchical planning |
| `POST` | `/v1/planning/mcts` | Monte Carlo Tree Search |
| `POST` | `/v1/planning/tot` | Tree of Thoughts |
| `POST` | `/v1/planning/plan-mode/enter` | Enter plan mode |
| `POST` | `/v1/planning/plan-mode/{id}/exit` | Exit plan mode |
| `GET` | `/v1/planning/plan-mode/{id}/status` | Plan mode status |
| `POST` | `/v1/planning/plan-mode/{id}/verify` | Verify plan |
| `POST` | `/v1/planning/plan-mode/{id}/execute` | Execute plan |
| `PUT` | `/v1/planning/plan-mode/{id}/tasks/{taskId}` | Update task |

---

## LLMOps Endpoints

A/B experiments, continuous evaluation, prompt versioning.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/llmops/experiments` | Create experiment |
| `GET` | `/v1/llmops/experiments` | List experiments |
| `GET` | `/v1/llmops/experiments/{id}` | Get experiment |
| `POST` | `/v1/llmops/evaluate` | Run evaluation |
| `GET` | `/v1/llmops/prompts` | List prompt versions |
| `POST` | `/v1/llmops/prompts` | Create prompt version |

---

## Benchmark Endpoints

LLM benchmarking: SWE-bench, HumanEval, MMLU.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/benchmark/run` | Start benchmark |
| `GET` | `/v1/benchmark/results` | List results |
| `GET` | `/v1/benchmark/results/{id}` | Get result |

---

## Discovery Endpoints

Dynamic 3-tier model discovery.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/discovery/models` | Discovered models |
| `GET` | `/v1/discovery/models/selected` | Selected models |
| `GET` | `/v1/discovery/stats` | Discovery statistics |
| `POST` | `/v1/discovery/trigger` | Trigger discovery |
| `GET` | `/v1/discovery/ensemble` | Ensemble models |
| `GET` | `/v1/discovery/debate-model` | Best debate model |

---

## Scoring Endpoints

5-component weighted scoring pipeline.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/scoring/model/{model_id}` | Get model score |
| `POST` | `/v1/scoring/batch` | Batch scoring |
| `GET` | `/v1/scoring/top` | Top models |
| `GET` | `/v1/scoring/range` | Models by score range |
| `GET` | `/v1/scoring/weights` | Get scoring weights |
| `PUT` | `/v1/scoring/weights` | Update weights |
| `GET` | `/v1/scoring/model/{model_id}/detail` | Detailed score |
| `POST` | `/v1/scoring/cache/invalidate` | Invalidate cache |
| `POST` | `/v1/scoring/compare` | Compare models |

---

## Verification Endpoints

8-test verification pipeline.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/verification/model` | Verify model |
| `POST` | `/v1/verification/batch` | Batch verify |
| `GET` | `/v1/verification/status` | Verification status |
| `GET` | `/v1/verification/models` | Verified models |
| `POST` | `/v1/verification/model/{model_id}/reverify` | Re-verify |
| `GET` | `/v1/verification/tests` | Available tests |
| `GET` | `/v1/verification/health` | System health |
| `POST` | `/v1/verification/code-visibility` | Code visibility test |

---

## Health Monitoring Endpoints

Extended provider health, latency tracking, circuit breakers.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/health/providers` | All providers health |
| `GET` | `/v1/health/providers/healthy` | Healthy providers |
| `GET` | `/v1/health/providers/fastest` | Fastest provider |
| `GET` | `/v1/health/provider/{id}` | Provider health |
| `GET` | `/v1/health/provider/{id}/latency` | Provider latency |
| `GET` | `/v1/health/provider/{id}/available` | Provider availability |
| `GET` | `/v1/health/circuit-breakers` | Circuit breaker status |
| `POST` | `/v1/health/provider/{id}/success` | Record success |
| `POST` | `/v1/health/provider/{id}/failure` | Record failure |
| `POST` | `/v1/health/provider` | Add provider |
| `DELETE` | `/v1/health/provider/{id}` | Remove provider |
| `GET` | `/v1/health/status` | Service status |

---

## Cognee Endpoints

Knowledge graph with memory, cognification, insights, and datasets.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/cognee/health` | Service health |
| `GET` | `/v1/cognee/stats` | Usage statistics |
| `GET` | `/v1/cognee/config` | Configuration |
| `POST` | `/v1/cognee/start` | Ensure running |
| `POST` | `/v1/cognee/memory` | Add memory |
| `POST` | `/v1/cognee/search` | Search knowledge |
| `POST` | `/v1/cognee/cognify` | Cognify content |
| `POST` | `/v1/cognee/insights` | Get insights |
| `POST` | `/v1/cognee/graph/complete` | Graph completion |
| `GET` | `/v1/cognee/visualize` | Visualize graph |
| `POST` | `/v1/cognee/code` | Code intelligence |
| `POST` | `/v1/cognee/datasets` | Create dataset |
| `GET` | `/v1/cognee/datasets` | List datasets |
| `DELETE` | `/v1/cognee/datasets/{name}` | Delete dataset |
| `POST` | `/v1/cognee/feedback` | Provide feedback |

---

## Search Endpoints

Vector-based semantic code search.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/search/semantic` | Semantic search |
| `POST` | `/v1/search/index` | Trigger indexing |

---

## Templates Endpoints

Reusable prompt templates.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/templates` | List templates |
| `GET` | `/v1/templates/{id}` | Get template |
| `POST` | `/v1/templates/apply` | Apply template |

---

## Checkpoints Endpoints

Workspace snapshots with Git state capture.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/checkpoints` | List checkpoints |
| `POST` | `/v1/checkpoints` | Create checkpoint |
| `POST` | `/v1/checkpoints/{id}/restore` | Restore checkpoint |
| `DELETE` | `/v1/checkpoints/{id}` | Delete checkpoint |

---

## Browser Automation Endpoints

Playwright-based web automation.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/browser/navigate` | Navigate to URL |
| `POST` | `/v1/browser/click` | Click element |
| `POST` | `/v1/browser/type` | Type text |
| `POST` | `/v1/browser/screenshot` | Take screenshot |
| `POST` | `/v1/browser/extract` | Extract content |
| `POST` | `/v1/browser/evaluate` | Evaluate JavaScript |

---

## Skills Endpoints

Skill registry for enhanced completions.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/skills` | List skills |
| `GET` | `/v1/skills/categories` | List categories |
| `GET` | `/v1/skills/{category}` | Skills by category |
| `POST` | `/v1/skills/match` | Match skills to query |

---

## QA Endpoints

HelixQA autonomous QA pipeline.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/qa/sessions` | Start QA session |
| `GET` | `/v1/qa/findings` | List findings |
| `GET` | `/v1/qa/findings/{id}` | Get finding |
| `PUT` | `/v1/qa/findings/{id}` | Update finding |
| `GET` | `/v1/qa/platforms` | List platforms |
| `POST` | `/v1/qa/discover` | Discover knowledge |

---

## Background Tasks Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/tasks` | Create task |
| `GET` | `/v1/tasks` | List tasks |
| `GET` | `/v1/tasks/queue/stats` | Queue statistics |
| `GET` | `/v1/tasks/events` | Poll events |
| `GET` | `/v1/tasks/{id}` | Get task |
| `GET` | `/v1/tasks/{id}/status` | Task status |
| `GET` | `/v1/tasks/{id}/logs` | Task logs |
| `GET` | `/v1/tasks/{id}/resources` | Resource usage |
| `GET` | `/v1/tasks/{id}/events` | Task events (SSE) |
| `GET` | `/v1/tasks/{id}/analyze` | Stuck detection |
| `POST` | `/v1/tasks/{id}/pause` | Pause task |
| `POST` | `/v1/tasks/{id}/resume` | Resume task |
| `POST` | `/v1/tasks/{id}/cancel` | Cancel task |
| `DELETE` | `/v1/tasks/{id}` | Delete task |
| `POST` | `/v1/webhooks` | Register webhook |
| `GET` | `/v1/webhooks` | List webhooks |
| `DELETE` | `/v1/webhooks/{id}` | Delete webhook |

---

## Sessions Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/sessions` | Create session |
| `GET` | `/v1/sessions` | List sessions |
| `GET` | `/v1/sessions/{id}` | Get session |
| `DELETE` | `/v1/sessions/{id}` | Terminate session |

---

## Features Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/features` | Enabled features |
| `GET` | `/v1/features/available` | All features |
| `GET` | `/v1/features/agents` | Agent capabilities |

---

## GraphQL Endpoint

Feature-flagged (requires `GRAPHQL_ENABLED=true`).

**POST** `/v1/graphql`

**Request Body:**

```json
{
  "query": "{ providers { name status score } }",
  "variables": {}
}
```

---

## Error Handling

### Error Response Format

```json
{
  "error": {
    "type": "invalid_request_error|authentication_error|rate_limit_error|provider_error",
    "message": "string",
    "code": "ERROR_CODE",
    "param": "string (optional)",
    "details": {}
  }
}
```

### HTTP Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 429 | Rate Limit Exceeded |
| 500 | Internal Server Error |
| 502 | Provider Error |
| 503 | Service Unavailable |

### Error Types

- `invalid_request_error` - Malformed request
- `authentication_error` - Invalid or missing API key
- `rate_limit_error` - Too many requests
- `provider_error` - Upstream provider error
- `model_not_found` - Requested model doesn't exist
- `context_length_exceeded` - Input too long

---

## Rate Limiting

Default rate limits:

- **Authenticated:** 1000 requests/hour
- **Anonymous:** 100 requests/hour

Rate limit headers:

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1234567890
```

---

## Streaming

All streaming endpoints use Server-Sent Events (SSE):

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

---

## WebSocket

WebSocket endpoint for real-time updates:

```
ws://localhost:7061/ws
```

### Message Format

```json
{
  "type": "chat|debate|monitoring",
  "action": "subscribe|unsubscribe|message",
  "data": {}
}
```

---

## OpenAPI Specification

Full OpenAPI 3.0 specification available at:

```
GET /v1/openapi.json
GET /v1/openapi.yaml
```

---

## SDK Examples

### Go

```go
import "github.com/vasic-digital/helixagent-sdk"

client := helixagent.NewClient("your-api-key")
response, err := client.Chat.Completions.Create(ctx, &helixagent.ChatRequest{
    Model: "gpt-4",
    Messages: []helixagent.Message{
        {Role: "user", Content: "Hello!"},
    },
})
```

### Python

```python
import helixagent

client = helixagent.Client(api_key="your-api-key")
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### cURL

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

---

## Versioning

API version is included in the URL path: `/v1/...`

Current version: **v1**

---

## Changelog

### v1.3.0 (2026-02-25)
- Added Kimi Code provider
- Added Qwen Code provider
- Added profiling tools
- Enhanced security scanning
- Memory leak detection
- Deadlock prevention

### v1.2.0 (2026-02-20)
- Added 14 new LLM providers
- Enhanced debate orchestration
- Improved streaming support

### v1.1.0 (2026-02-15)
- Added MCP support
- Added RAG pipeline
- Added memory system

### v1.0.0 (2026-01-01)
- Initial release
- Core LLM integration
- Basic API endpoints
