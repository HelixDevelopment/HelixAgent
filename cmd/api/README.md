# HelixAgent Protocol Enhancement API Server

This package contains the standalone Protocol Enhancement REST API server
entry point -- a lightweight server exposing MCP, LSP, and ACP protocol
endpoints under `/api/v1/*`.

## Overview

This is **not** the OpenAI-compatible completions server. There is no
`/v1/completions`, `/v1/chat/completions`, or `/v1/models` endpoint here, no
`API_KEY`/`PROVIDER` configuration, and no `setup.go` file. For that surface,
see `cmd/helixagent/main.go` (the main production entry point) and
`internal/router/router.go` (the production API router).

This package serves the **protocol enhancement** API: MCP tool-calling, LSP
code-navigation, ACP agent execution, plus analytics, plugin, and template
endpoints, all under `/api/v1/*`.

### ACP endpoints are real, MCP/LSP endpoints are demo

- **ACP (`/api/v1/acp/*`)** delegates to the real ACP dispatcher --
  `internal/handlers.ACPHandler`, the same handler
  `internal/router/router.go` wires up at the production `/v1/acp/*`
  routes. Agent existence is genuinely validated against the ACP agent
  registry (`internal/handlers/acp_handler.go`), tasks are genuinely
  executed, and broadcast delivery counts reflect real per-target outcomes.
- **MCP (`/api/v1/mcp/*`)** and **LSP (`/api/v1/lsp/*`)** in this file
  remain DEMONSTRATION handlers that return HARDCODED/MOCK responses and do
  NOT connect to real backends. They are useful for API structure
  exploration, client development, and documentation examples, but MUST NOT
  be relied on for production MCP/LSP behaviour. For real MCP/LSP handling,
  use `internal/router/router.go`.

## Files

- `main.go` - API server entry point (`APIServer`, route wiring, all handlers)
- `main_test.go` - handler tests

## Usage

### Build and Run

```bash
# Build
go build -o bin/api-server ./cmd/api

# Run (defaults to port 8080)
./bin/api-server

# Run with a custom port
PORT=9090 ./bin/api-server
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |

## Endpoints

| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/api/v1/mcp/tools/call` | POST | Call an MCP tool | Demo (mock) |
| `/api/v1/mcp/tools/list` | GET | List MCP tools | Demo (mock) |
| `/api/v1/mcp/servers` | GET | List MCP servers | Demo (mock) |
| `/api/v1/lsp/completion` | POST | Code completion | Demo (mock) |
| `/api/v1/lsp/hover` | POST | Hover info | Demo (mock) |
| `/api/v1/lsp/definition` | POST | Go-to-definition | Demo (mock) |
| `/api/v1/lsp/diagnostics` | POST | Diagnostics | Demo (mock) |
| `/api/v1/acp/execute` | POST | Execute a task on a real registered ACP agent | Real |
| `/api/v1/acp/broadcast` | POST | Broadcast a task to a list of target agent ids | Real |
| `/api/v1/acp/status` | GET | Look up a real agent's status by `?agent_id=` | Real |
| `/api/v1/analytics/metrics` | GET | Aggregate request metrics | Real (in-memory) |
| `/api/v1/analytics/metrics/:protocol` | GET | Per-protocol metrics | Real (in-memory) |
| `/api/v1/analytics/health` | GET | Analytics health | Real |
| `/api/v1/analytics/record` | POST | Record a request metric | Real (in-memory) |
| `/api/v1/plugins/*` | various | Plugin registry endpoints | Demo (no real plugin loader) |
| `/api/v1/templates/*` | various | Integration templates | Real (static built-in set) |
| `/api/v1/health` | GET | Health check | Real |
| `/api/v1/status` | GET | Server status | Real |
| `/api/v1/metrics` | GET | Prometheus metrics | Real (in-memory) |

## Real ACP agents

The ACP endpoints dispatch against a fixed, built-in set of agents
registered by `internal/handlers.ACPHandler` (see
`internal/handlers/acp_handler.go:initializeAgents`):

- `code-reviewer`
- `bug-finder`
- `refactor-assistant`
- `documentation-generator`
- `test-generator`
- `security-scanner`

Executing or broadcasting to any other agent id genuinely fails (`404 agent
not found`) rather than reporting a fabricated success.

## Example

```bash
curl -X POST http://localhost:8080/api/v1/acp/execute \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "code-reviewer",
    "task": "Review this function for bugs"
  }'

curl "http://localhost:8080/api/v1/acp/status?agent_id=code-reviewer"
```

## Testing

```bash
go test -v ./cmd/api/...
```
