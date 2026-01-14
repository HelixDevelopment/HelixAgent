# HelixAgent API Reference

Complete API documentation for HelixAgent and LLMsVerifier.

## Table of Contents

1. [HelixAgent REST API](#helixagent-rest-api)
2. [AI Debate Ensemble API](#ai-debate-ensemble-api)
3. [Protocol APIs](#protocol-apis)
4. [Background Tasks API](#background-tasks-api)
5. [CLI Agent Configuration API](#cli-agent-configuration-api)
6. [LLMsVerifier Capability Detection API](#llmsverifier-capability-detection-api)

---

## HelixAgent REST API

Base URL: `http://localhost:7061`

### Authentication

Most endpoints require authentication via Bearer token:

```bash
Authorization: Bearer YOUR_API_KEY
```

### OpenAI-Compatible Endpoints

#### POST /v1/chat/completions

Create a chat completion using the AI Debate Ensemble.

**Request:**
```json
{
  "model": "helixagent-debate",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Explain quantum computing."}
  ],
  "stream": true,
  "temperature": 0.7,
  "max_tokens": 4096,
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "Glob",
        "description": "Find files matching a pattern",
        "parameters": {
          "type": "object",
          "properties": {"pattern": {"type": "string"}},
          "required": ["pattern"]
        }
      }
    }
  ]
}
```

**Response (Streaming):**
```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"Quantum"},"index":0}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":" computing"},"index":0}]}

data: [DONE]
```

**Response (Non-Streaming):**
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1705555200,
  "model": "helixagent-debate",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Quantum computing is..."
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

#### GET /v1/models

List available models.

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "helixagent-debate",
      "object": "model",
      "created": 1705555200,
      "owned_by": "helixagent",
      "capabilities": {
        "vision": true,
        "streaming": true,
        "function_calling": true,
        "embeddings": true,
        "mcp": true,
        "acp": true,
        "lsp": true
      }
    }
  ]
}
```

#### POST /v1/embeddings

Generate embeddings for text.

**Request:**
```json
{
  "model": "helixagent-debate",
  "input": "The quick brown fox jumps over the lazy dog."
}
```

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "embedding": [0.0023, -0.0012, ...],
      "index": 0
    }
  ],
  "model": "helixagent-debate",
  "usage": {
    "prompt_tokens": 10,
    "total_tokens": 10
  }
}
```

---

## AI Debate Ensemble API

### POST /v1/debates

Create a new AI debate.

**Request:**
```json
{
  "topic": "Should AI systems be open source?",
  "participants": [
    {"role": "analyst", "provider": "anthropic", "model": "claude-3-opus"},
    {"role": "proposer", "provider": "openai", "model": "gpt-4"},
    {"role": "critic", "provider": "deepseek", "model": "deepseek-chat"},
    {"role": "synthesizer", "provider": "gemini", "model": "gemini-pro"},
    {"role": "mediator", "provider": "qwen", "model": "qwen-max"}
  ],
  "rounds": 3,
  "dialogue_style": "theater"
}
```

**Response:**
```json
{
  "id": "debate-abc123",
  "status": "created",
  "topic": "Should AI systems be open source?",
  "participants": [...],
  "created_at": "2025-01-14T10:30:00Z"
}
```

### GET /v1/debates/:id

Get debate details and status.

**Response:**
```json
{
  "id": "debate-abc123",
  "status": "completed",
  "topic": "Should AI systems be open source?",
  "rounds": [
    {
      "number": 1,
      "responses": [
        {"role": "analyst", "content": "Let me analyze..."},
        {"role": "proposer", "content": "I propose..."},
        {"role": "critic", "content": "I challenge..."},
        {"role": "synthesizer", "content": "Combining perspectives..."},
        {"role": "mediator", "content": "After weighing..."}
      ]
    }
  ],
  "consensus": "The debate concluded with...",
  "completed_at": "2025-01-14T10:35:00Z"
}
```

### GET /v1/debates/:id/status

Get debate execution status (for async debates).

**Response:**
```json
{
  "id": "debate-abc123",
  "status": "running",
  "current_round": 2,
  "total_rounds": 3,
  "progress": 66.7
}
```

### GET /v1/debates

List all debates.

**Response:**
```json
{
  "debates": [
    {"id": "debate-abc123", "topic": "...", "status": "completed"},
    {"id": "debate-def456", "topic": "...", "status": "running"}
  ],
  "total": 2
}
```

### DELETE /v1/debates/:id

Delete a debate.

**Response:**
```json
{
  "id": "debate-abc123",
  "deleted": true
}
```

---

## Protocol APIs

### MCP (Model Context Protocol)

#### GET /v1/mcp

SSE endpoint for MCP connection.

**Response (SSE):**
```
event: endpoint
data: {"uri": "http://localhost:7061/v1/mcp/message"}

event: heartbeat
data: {"timestamp": "2025-01-14T10:30:00Z"}
```

#### POST /v1/mcp/message

Send MCP message.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "tools": [
      {"name": "Bash", "description": "Execute shell commands"},
      {"name": "Read", "description": "Read file contents"},
      {"name": "Write", "description": "Write file contents"}
    ]
  },
  "id": 1
}
```

### ACP (Agent Communication Protocol)

#### POST /v1/acp

Send ACP message.

**Request:**
```json
{
  "type": "request",
  "agent_id": "agent-123",
  "action": "execute_task",
  "payload": {
    "task": "analyze_code",
    "target": "/path/to/file.go"
  }
}
```

### LSP (Language Server Protocol)

#### POST /v1/lsp

Send LSP request.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "textDocument/definition",
  "params": {
    "textDocument": {"uri": "file:///path/to/file.go"},
    "position": {"line": 10, "character": 5}
  },
  "id": 1
}
```

### Vision

#### POST /v1/vision

Analyze images.

**Request:**
```json
{
  "model": "helixagent-debate",
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "Describe this image"},
        {"type": "image_url", "image_url": {"url": "data:image/png;base64,..."}}
      ]
    }
  ]
}
```

### Cognee

#### POST /v1/cognee/add

Add content to knowledge graph.

**Request:**
```json
{
  "content": "Important information about our project architecture...",
  "metadata": {
    "source": "architecture.md",
    "tags": ["architecture", "design"]
  }
}
```

#### POST /v1/cognee/search

Search knowledge graph.

**Request:**
```json
{
  "query": "What is the project architecture?",
  "limit": 10
}
```

---

## Background Tasks API

### POST /v1/tasks

Create a background task.

**Request:**
```json
{
  "type": "command",
  "command": "npm run build",
  "working_dir": "/path/to/project",
  "priority": "high",
  "endless": false
}
```

**Response:**
```json
{
  "id": "task-xyz789",
  "status": "pending",
  "created_at": "2025-01-14T10:30:00Z"
}
```

### GET /v1/tasks/:id/status

Get task status.

**Response:**
```json
{
  "id": "task-xyz789",
  "status": "running",
  "progress": 45.5,
  "started_at": "2025-01-14T10:30:05Z",
  "resources": {
    "cpu_percent": 25.3,
    "memory_mb": 512,
    "io_read_bytes": 1048576,
    "io_write_bytes": 524288
  }
}
```

### GET /v1/tasks/:id/events

SSE stream for task events.

**Response (SSE):**
```
event: progress
data: {"progress": 50.0, "message": "Compiling..."}

event: output
data: {"stream": "stdout", "content": "Building module 5/10..."}

event: complete
data: {"status": "completed", "exit_code": 0}
```

### POST /v1/tasks/:id/cancel

Cancel a running task.

**Response:**
```json
{
  "id": "task-xyz789",
  "status": "cancelled"
}
```

### GET /v1/tasks/:id/analyze

Analyze task for stuck detection.

**Response:**
```json
{
  "id": "task-xyz789",
  "stuck_analysis": {
    "is_stuck": false,
    "checks": {
      "heartbeat": {"passed": true, "last_heartbeat": "2025-01-14T10:35:00Z"},
      "cpu_freeze": {"passed": true, "cpu_usage": 25.3},
      "memory_leak": {"passed": true, "memory_growth_rate": 0.01},
      "io_starvation": {"passed": true, "io_activity": true}
    }
  }
}
```

---

## CLI Agent Configuration API

### GET /v1/agents

List all supported CLI agents.

**Response:**
```json
{
  "agents": [
    {
      "name": "opencode",
      "language": "Go",
      "config_format": "json",
      "streaming": true,
      "mcp_support": true,
      "provider_count": 15
    },
    {
      "name": "claudecode",
      "language": "TypeScript",
      "config_format": "json",
      "streaming": true,
      "mcp_support": true,
      "provider_count": 1
    }
  ],
  "total": 18
}
```

### GET /v1/agents/:name

Get specific agent details.

**Response:**
```json
{
  "name": "kilocode",
  "language": "TypeScript",
  "config_format": "json",
  "config_path": "~/.kilocode/settings.json",
  "streaming": {
    "supported": true,
    "types": ["async_generator"],
    "chunk_types": ["text", "reasoning", "tool_call"]
  },
  "network": {
    "http_versions": ["http/1.1", "http/2"],
    "http3_supported": false,
    "proxy_supported": true
  },
  "compression": {
    "supported": false
  },
  "caching": {
    "supported": true,
    "types": ["prompt_caching"]
  },
  "protocols": ["openai", "anthropic", "mcp"],
  "provider_count": 43,
  "tool_count": 28,
  "extended": {
    "plan_act_modes": true,
    "checkpointing": true,
    "auto_approval": true
  }
}
```

### GET /v1/agents/protocol/:protocol

Get agents supporting a specific protocol.

**Response:**
```json
{
  "protocol": "mcp",
  "agents": ["opencode", "claudecode", "amazonq", "helixcode"]
}
```

### GET /v1/agents/tool/:tool

Get agents supporting a specific tool.

**Response:**
```json
{
  "tool": "Git",
  "agents": ["opencode", "claudecode", "kilocode", "aider", "plandex"]
}
```

---

## LLMsVerifier Capability Detection API

### Go Package API

```go
import "llm-verifier/capabilities"

// Create detector
detector := capabilities.NewDetector()

// Detect provider capabilities dynamically
caps, err := detector.DetectProviderCapabilities(ctx, "openai", apiKey)

// Query specific capabilities
sseType := capabilities.StreamingTypeSSE
query := &capabilities.CapabilityQuery{
    Provider:         "openai",
    RequireStreaming: &sseType,
    RequireVision:    true,
}
result, err := detector.Query(ctx, query)

// Get full capability matrix
matrix := detector.GetCapabilityMatrix()
sseProviders := matrix.ByStreaming[capabilities.StreamingTypeSSE]

// Generate CLI agent configuration
generator := capabilities.NewConfigGenerator("localhost", 7061)
config, err := generator.GenerateForAgent("opencode", nil)
```

### Capability Types

#### StreamingType
```go
StreamingTypeSSE           // Server-Sent Events
StreamingTypeWebSocket     // WebSocket
StreamingTypeAsyncGen      // AsyncGenerator/yield
StreamingTypeJSONL         // JSON Lines streaming
StreamingTypeMpscStream    // Rust MPSC channel
StreamingTypeEventStream   // AWS EventStream
StreamingTypeStdout        // Standard output
StreamingTypeNone          // No streaming
```

#### CompressionType
```go
CompressionGzip     // Gzip compression
CompressionBrotli   // Brotli compression
CompressionSemantic // Semantic context compression
CompressionChat     // Chat history compression
```

#### CachingType
```go
CachingAnthropic    // Anthropic cache_control
CachingDashScope    // DashScope X-DashScope-CacheControl
CachingPrompt       // Generic prompt caching
CachingSemantic     // Semantic similarity caching
CachingLLMOps       // LangChain/SQLite cache
```

#### ProtocolType
```go
ProtocolMCP         // Model Context Protocol
ProtocolACP         // Agent Communication Protocol
ProtocolLSP         // Language Server Protocol
ProtocolGRPC        // gRPC
ProtocolOpenAI      // OpenAI-compatible API
ProtocolAnthropic   // Anthropic API
ProtocolOllama      // Ollama local API
```

### Key Functions

```go
// Get provider capabilities
caps := capabilities.GetProviderBaseCapabilities("openai")

// Get CLI agent capabilities
agentCaps := capabilities.GetCLIAgentCapabilities("kilocode")

// List all providers
providers := capabilities.GetAllProviders()

// List all CLI agents
agents := capabilities.GetAllCLIAgents()

// Find providers with specific capability
streamingProviders := capabilities.GetProvidersWithCapability("streaming", nil)
oauthProviders := capabilities.GetProvidersWithCapability("oauth", nil)

// Find CLI agents with specific capability
mcpAgents := capabilities.GetCLIAgentsWithCapability("mcp")
checkpointAgents := capabilities.GetCLIAgentsWithCapability("checkpointing")
```

---

## Error Responses

All endpoints return standard error responses:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "The request body is malformed.",
    "details": {
      "field": "messages",
      "issue": "required field missing"
    }
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `invalid_request` | 400 | Malformed request |
| `authentication_error` | 401 | Invalid or missing API key |
| `permission_denied` | 403 | Insufficient permissions |
| `not_found` | 404 | Resource not found |
| `rate_limited` | 429 | Too many requests |
| `internal_error` | 500 | Server error |
| `service_unavailable` | 503 | Service temporarily unavailable |

---

## Rate Limits

| Endpoint | Limit | Window |
|----------|-------|--------|
| `/v1/chat/completions` | 60 | 1 minute |
| `/v1/debates` | 10 | 1 minute |
| `/v1/embeddings` | 100 | 1 minute |
| `/v1/tasks` | 30 | 1 minute |

Rate limit headers:
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1705555260
```

---

## WebSocket Endpoints

### WS /v1/ws/tasks/:id

Real-time task updates via WebSocket.

**Messages:**
```json
// Progress update
{"type": "progress", "data": {"progress": 50.0, "message": "Building..."}}

// Output
{"type": "output", "data": {"stream": "stdout", "content": "Compiling module..."}}

// Complete
{"type": "complete", "data": {"status": "completed", "exit_code": 0}}

// Error
{"type": "error", "data": {"code": "task_failed", "message": "Build failed"}}
```

---

## SDK Examples

### Python

```python
import requests

response = requests.post(
    "http://localhost:7061/v1/chat/completions",
    headers={"Authorization": "Bearer YOUR_API_KEY"},
    json={
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Hello!"}],
        "stream": False
    }
)
print(response.json()["choices"][0]["message"]["content"])
```

### TypeScript

```typescript
const response = await fetch("http://localhost:7061/v1/chat/completions", {
  method: "POST",
  headers: {
    "Authorization": "Bearer YOUR_API_KEY",
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    model: "helixagent-debate",
    messages: [{ role: "user", content: "Hello!" }],
    stream: false
  })
});
const data = await response.json();
console.log(data.choices[0].message.content);
```

### Go

```go
import "dev.helix.agent/client"

client := client.New("http://localhost:7061", "YOUR_API_KEY")
response, err := client.ChatCompletion(ctx, &client.ChatRequest{
    Model: "helixagent-debate",
    Messages: []client.Message{
        {Role: "user", Content: "Hello!"},
    },
})
fmt.Println(response.Choices[0].Message.Content)
```

---

## Related Documentation

- [Capability Detection](../LLMsVerifier/docs/CAPABILITY_DETECTION.md) - Full capability detection documentation
- [CLI Agent Registry](./CLAUDE.md#cli-agent-registry) - Detailed CLI agent information
- [AI Debate Team](./CLAUDE.md#ai-debate-team-composition) - Debate team configuration
- [Background Execution](./docs/background-execution/README.md) - Background task system
- [Challenge Scripts](./challenges/scripts/) - Validation challenges
