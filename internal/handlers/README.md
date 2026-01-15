# Handlers Package

The `handlers` package contains HTTP handlers for HelixAgent's REST API, providing OpenAI-compatible endpoints, protocol handlers, and administrative interfaces.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP Router (Gin)                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              OpenAI-Compatible Endpoints                 │    │
│  │  /v1/chat/completions  /v1/completions  /v1/models      │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │   Protocol   │  │    Task      │  │      Admin           │  │
│  │   Handlers   │  │   Handlers   │  │      Handlers        │  │
│  │              │  │              │  │                      │  │
│  │  MCP, LSP    │  │  Background  │  │  Health, Scoring     │  │
│  │  Cognee      │  │  SSE, WS     │  │  Verification        │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                  Debate Handlers                          │   │
│  │  /v1/debates  /v1/debates/:id/events  visualization      │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## OpenAI-Compatible Endpoints

### Completion Handler (`openai_compatible.go`)

Full OpenAI API compatibility:

```go
handler := handlers.NewOpenAICompatibleHandler(registry, config, logger)

// Routes:
// POST /v1/chat/completions - Chat completions (streaming supported)
// POST /v1/completions - Text completions
// GET  /v1/models - List available models
// GET  /v1/models/:id - Get model details
```

Request format:
```json
{
    "model": "helix-ensemble",
    "messages": [
        {"role": "user", "content": "Hello!"}
    ],
    "stream": true,
    "max_tokens": 1000,
    "temperature": 0.7
}
```

### Embeddings Handler (`embeddings.go`)

Vector embedding generation:

```go
handler := handlers.NewEmbeddingsHandler(embeddingManager, logger)

// POST /v1/embeddings
```

## Protocol Handlers

### MCP Handler (`mcp.go`)

Model Context Protocol:

```go
handler := handlers.NewMCPHandler(mcpClient, logger)

// POST /v1/mcp/tools/:name - Execute MCP tool
// GET  /v1/mcp/tools - List available tools
// GET  /v1/mcp/resources - List resources
```

### LSP Handler (`lsp.go`)

Language Server Protocol:

```go
handler := handlers.NewLSPHandler(lspManager, logger)

// POST /v1/lsp/completions - Get completions
// POST /v1/lsp/hover - Get hover info
// POST /v1/lsp/definition - Go to definition
// POST /v1/lsp/references - Find references
```

### Cognee Handler (`cognee_handler.go`)

Knowledge graph and RAG:

```go
handler := handlers.NewCogneeHandler(cogneeService, logger)

// POST /v1/cognee/add - Add knowledge
// POST /v1/cognee/search - Search knowledge graph
// POST /v1/cognee/query - Query with RAG
```

## Task Handlers

### Background Task Handler (`background_task_handler.go`)

Async task management:

```go
handler := handlers.NewBackgroundTaskHandler(taskService, notificationHub, logger)

// POST   /v1/tasks - Create task
// GET    /v1/tasks/:id - Get task status
// PUT    /v1/tasks/:id/pause - Pause task
// PUT    /v1/tasks/:id/resume - Resume task
// DELETE /v1/tasks/:id - Cancel task
// GET    /v1/tasks/:id/events - SSE event stream
// GET    /v1/ws/tasks/:id - WebSocket connection
```

### Protocol SSE Handler (`protocol_sse.go`)

Server-Sent Events for real-time updates:

```go
handler := handlers.NewProtocolSSEHandler(hub, logger)

// GET /v1/events/:channel - SSE stream
```

## Debate Handlers

### Debate Handler (`debate_handler.go`)

AI debate management:

```go
handler := handlers.NewDebateHandler(debateService, logger)

// POST /v1/debates - Start debate
// GET  /v1/debates/:id - Get debate status
// GET  /v1/debates/:id/events - SSE for debate events
// POST /v1/debates/:id/vote - Vote on response
```

### Debate Visualization (`debate_visualization.go`)

Formats debate output for display:

```go
visualizer := handlers.NewDebateVisualizer(config)

// Renders multi-pass validation phases
// Shows request/response timing
// Displays fallback chains
// ANSI color support for CLI
```

## Admin Handlers

### Health Handler (`health_handler.go`)

System health checks:

```go
handler := handlers.NewHealthHandler(services, logger)

// GET /health - Basic health check
// GET /health/ready - Readiness probe
// GET /health/live - Liveness probe
// GET /health/detailed - Detailed system status
```

### Verification Handler (`verification_handler.go`)

Provider verification:

```go
handler := handlers.NewVerificationHandler(verifier, logger)

// POST /v1/verify/:provider - Verify provider
// GET  /v1/verify/status - Verification status
```

### Scoring Handler (`scoring_handler.go`)

Provider scoring:

```go
handler := handlers.NewScoringHandler(scorer, logger)

// GET  /v1/scores - Get all provider scores
// GET  /v1/scores/:provider - Get specific score
// POST /v1/scores/recalculate - Force recalculation
```

### Discovery Handler (`discovery_handler.go`)

Provider discovery:

```go
handler := handlers.NewDiscoveryHandler(discoveryService, logger)

// GET /v1/discover - Discover available providers
// GET /v1/discover/:provider - Provider details
```

### Provider Management (`provider_management.go`)

Provider lifecycle:

```go
handler := handlers.NewProviderManagementHandler(registry, logger)

// GET  /v1/providers - List providers
// POST /v1/providers/:name/enable - Enable provider
// POST /v1/providers/:name/disable - Disable provider
```

## Files

| File | Description |
|------|-------------|
| `openai_compatible.go` | OpenAI API compatibility layer |
| `completion.go` | Legacy completion handler |
| `embeddings.go` | Embedding generation |
| `mcp.go` | MCP protocol handler |
| `lsp.go` | LSP protocol handler |
| `cognee_handler.go` | Cognee integration |
| `background_task_handler.go` | Async task management |
| `protocol_sse.go` | SSE streaming |
| `debate_handler.go` | AI debate API |
| `debate_visualization.go` | Debate output formatting |
| `health_handler.go` | Health endpoints |
| `verification_handler.go` | Provider verification |
| `scoring_handler.go` | Provider scoring |
| `discovery_handler.go` | Provider discovery |
| `provider_management.go` | Provider lifecycle |
| `monitoring_handler.go` | System monitoring |
| `session.go` | Session management |
| `model_metadata.go` | Model information |
| `openrouter_models.go` | OpenRouter model list |

## Testing

```bash
go test -v ./internal/handlers/...
```

Tests cover:
- OpenAI API compatibility
- Request validation
- Response formatting
- SSE streaming
- WebSocket connections
- Error handling
- Authentication and authorization
