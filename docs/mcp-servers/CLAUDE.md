# CLAUDE.md - MCP-Servers Integration


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

```bash
# Register one MCP adapter (filesystem) against the running HelixAgent + exercise a tool call
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
GOMAXPROCS=2 nice -n 19 go test -count=1 -v \
  -run 'TestMCPAdapterFilesystem' ./internal/mcp/adapters/...
```
Expect: PASS; the filesystem adapter registers on the MCP bridge; a `tools/call` for `read_file` returns the expected content.


## Overview

MCP-Servers is a **read-only third-party submodule** containing 60+ containerized MCP (Model Context Protocol) server implementations. This document describes how HelixAgent integrates with and uses these servers. **Do not modify files in this submodule.**

## Port Assignments

All MCP servers run as containers with ports assigned in the 9101-9999 range. Zero npx dependencies — every server is containerized.

| Range       | Category              | Examples                                      |
|-------------|-----------------------|-----------------------------------------------|
| 9101-9120   | Core infrastructure   | filesystem, memory, sequential-thinking       |
| 9121-9150   | Database adapters     | postgres, sqlite, redis, mongodb, qdrant      |
| 9151-9180   | Search and retrieval  | brave-search, everything, fetch               |
| 9181-9210   | Collaboration tools   | slack, notion, jira, linear, asana, miro      |
| 9211-9250   | Cloud and DevOps      | docker, kubernetes, aws-s3, datadog, sentry   |
| 9251-9280   | AI and ML             | stable-diffusion, replicate, chroma, weaviate |
| 9281-9300   | Creative and media    | figma, svgmaker, puppeteer                    |
| 9301-9999   | Extended and custom   | vision, embeddings, RAG, formatters           |

## Container Configuration

MCP server containers are defined in `docker/mcp/docker-compose.mcp-full.yml`. The container config generator at `internal/mcp/config/generator_container.go` produces compose configurations for each server.

Key environment variables per server:
- `MCP_SERVER_PORT` — the assigned port for the server
- `MCP_SERVER_NAME` — human-readable server name
- `MCP_TRANSPORT` — transport protocol (stdio, sse, streamable-http)

## Adapter Registry

HelixAgent adapters for MCP servers live in `internal/mcp/adapters/` (45+ adapters) and `internal/mcp/servers/` (server-specific adapters). Each adapter implements the standard MCP adapter interface for:

- Tool listing and execution
- Health checking
- Configuration validation
- Connection lifecycle management

The adapter registry at `internal/mcp/server_registry.go` tracks all available MCP servers and their connection status.

## Health Checks

Every MCP server exposes a health endpoint. HelixAgent performs periodic health checks through the adapter layer:

1. **TCP connectivity** — verify the container port is reachable
2. **MCP protocol handshake** — send an `initialize` JSON-RPC request
3. **Tool listing** — confirm the server reports its expected tools

Health status is exposed at `/v1/mcp` and integrated into the monitoring dashboard at `/v1/monitoring/status`.

## Integration Flow

1. HelixAgent boots and the `BootManager` starts MCP server containers via compose
2. Health checks run against each configured MCP server
3. Healthy servers are registered in the adapter registry
4. CLI agents receive MCP server endpoints in their generated configs
5. Requests to `/v1/mcp` are routed to the appropriate adapter

## Adding a New MCP Server

When a new server is added upstream:

1. Note the server name and capabilities from `MCP-Servers/src/`
2. Create an adapter in `internal/mcp/adapters/<name>_test.go` and `<name>.go`
3. Register in `internal/mcp/server_registry.go`
4. Add container config in `internal/mcp/config/generator_container.go`
5. Add port assignment in the compose file
6. Update CLI agent MCP filter in `cmd/helixagent/main.go` (`filterWorkingMCPs`)

## Key Files

- `internal/mcp/adapters/` — 45+ MCP adapter implementations
- `internal/mcp/servers/` — Server-specific adapter implementations with tool execution
- `internal/mcp/server_registry.go` — Central registry for MCP server tracking
- `internal/mcp/config/generator_container.go` — Compose configuration generator
- `internal/mcp/bridge/` — SSE bridge for MCP protocol transport
- `docker/mcp/docker-compose.mcp-full.yml` — Full MCP server compose file

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none (integration docs for third-party MCP-Servers submodule) |
| Downstream (these import this module) | root HelixAgent |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
