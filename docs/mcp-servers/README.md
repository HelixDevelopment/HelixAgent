# MCP-Servers Documentation

> **Location note**: relocated 2026-04-11 from `MCP-Servers/docs/README.md`
> (inside the third-party submodule) to `docs/mcp-servers/README.md` in
> the parent HelixAgent repo. The submodule is a read-only clone of
> `modelcontextprotocol/servers` at commit `f4244583` and must stay free
> of HelixAgent-specific files.

## Table of Contents

- [Server Index](#server-index)
- [Architecture Overview](#architecture-overview)
- [Configuration Guide](#configuration-guide)
- [Deployment Instructions](#deployment-instructions)
- [HelixAgent Integration](#helixagent-integration)

## Server Index

The MCP-Servers submodule contains 7 reference MCP server implementations. Each server demonstrates specific Model Context Protocol capabilities.

### TypeScript Servers (Node.js 22)

| Server | Package | Port | Features |
|--------|---------|------|----------|
| Everything | `@modelcontextprotocol/server-everything` | 3001 | 13 tools, resources (files, subscriptions, templates), 5 prompt types, sampling, logging, roots. Supports stdio, SSE, StreamableHTTP transports. |
| Filesystem | `@modelcontextprotocol/server-filesystem` | 3002 | File read/write/create/move/search. Path validation with configurable allowed directories. Root-based access control. |
| Memory | `@modelcontextprotocol/server-memory` | 3003 | Knowledge graph with entities, relations, observations. JSONL persistence. Tools: create_entities, create_relations, add_observations, delete_entities, delete_observations, delete_relations, read_graph, search_nodes, open_nodes. |
| Sequential Thinking | `@modelcontextprotocol/server-sequential-thinking` | 3004 | Structured thought sequences with branching, revision, and hypothesis verification. Dynamic thought count adjustment. |

### Python Servers (Python 3.12)

| Server | Package | Port | Features |
|--------|---------|------|----------|
| Fetch | `mcp-server-fetch` | 3005 | HTTP web fetching with configurable user-agent, robots.txt handling, proxy support. HTML-to-text conversion for LLM consumption. |
| Git | `mcp-server-git` | 3006 | Git repository operations: file reading, content search, history, diffs, branches, tags. Includes git-lfs support. |
| Time | `mcp-server-time` | 3007 | Time queries and timezone conversions. Configurable local timezone via `LOCAL_TIMEZONE` environment variable. |

## Architecture Overview

### System Context

```
+------------------+      +-------------------+      +------------------+
|   CLI Agents     |      |    HelixAgent     |      |   MCP-Servers    |
|  (48 agents)     |----->|   MCP Adapters    |----->|  (7 reference    |
|  OpenCode, etc.  |      |   (45+ adapters)  |      |   servers)       |
+------------------+      +-------------------+      +------------------+
                                    |
                                    v
                          +-------------------+
                          |   MCP Directory   |
                          |  docker-compose   |
                          |  (48 total MCPs)  |
                          +-------------------+
```

MCP-Servers provides the core 7 reference implementations. The broader `MCP/` directory adds 41 additional third-party MCP server submodules, bringing the total to 48 containerized MCP servers.

### Container Architecture

All servers use multi-stage Docker builds optimized for minimal image size:

```
TypeScript Build Pipeline:
  node:22.12-alpine (builder)
    -> npm install + TypeScript compile
    -> Copy dist/ + package.json
  node:22-alpine (release)
    -> npm ci --omit-dev
    -> CMD/ENTRYPOINT node dist/index.js

Python Build Pipeline:
  ghcr.io/astral-sh/uv:python3.12-bookworm-slim (builder)
    -> uv sync --locked (dependency install)
    -> uv sync (project install)
  python:3.12-slim-bookworm (release)
    -> Copy .venv from builder
    -> ENTRYPOINT mcp-server-<name>
```

### Transport Modes

The MCP specification defines three transport mechanisms:

| Transport | Protocol | Status | Servers |
|-----------|----------|--------|---------|
| stdio | Standard I/O | Active (default) | All 7 servers |
| SSE | HTTP Server-Sent Events | Deprecated (spec 2025-03-26) | everything |
| StreamableHTTP | HTTP streaming | Active (spec 2025-03-26) | everything |

In the HelixAgent deployment, all servers run in containers and communicate via stdio transport through `docker exec -i`.

### Everything Server Internal Architecture

The `everything` server is the most complex, demonstrating all MCP features:

```
src/everything/
├── index.ts                 # Entry point: transport mode selection
├── server/
│   ├── index.ts             # Server factory: creates and configures MCP server
│   ├── logging.ts           # Logging helpers
│   └── roots.ts             # Root tracking
├── tools/
│   ├── index.ts             # Tool registration aggregator
│   ├── echo.ts              # Echo tool
│   ├── get-sum.ts           # Arithmetic tool
│   ├── get-tiny-image.ts    # Image generation tool
│   ├── get-env.ts           # Environment variable tool
│   ├── get-roots-list.ts    # Root listing tool
│   ├── get-annotated-message.ts
│   ├── get-resource-links.ts
│   ├── get-resource-reference.ts
│   ├── get-structured-content.ts
│   ├── gzip-file-as-resource.ts
│   ├── simulate-research-query.ts
│   ├── toggle-simulated-logging.ts
│   ├── toggle-subscriber-updates.ts
│   ├── trigger-elicitation-request.ts
│   ├── trigger-elicitation-request-async.ts
│   ├── trigger-long-running-operation.ts
│   ├── trigger-sampling-request.ts
│   └── trigger-sampling-request-async.ts
├── resources/
│   ├── index.ts             # Resource registration
│   ├── files.ts             # File-based resources
│   ├── templates.ts         # URI template resources
│   └── subscriptions.ts     # Resource subscription management
├── prompts/
│   ├── index.ts             # Prompt registration
│   ├── simple.ts            # Simple text prompts
│   ├── args.ts              # Prompts with arguments
│   ├── completions.ts       # Completion prompts
│   └── resource.ts          # Resource-embedded prompts
└── transports/
    ├── stdio.ts             # Standard I/O transport
    ├── sse.ts               # Server-Sent Events transport
    └── streamableHttp.ts    # Streamable HTTP transport
```

## Configuration Guide

### Environment Variables

| Variable | Server | Default | Description |
|----------|--------|---------|-------------|
| `MEMORY_FILE_PATH` | memory | `memory.jsonl` | Path to knowledge graph storage file |
| `LOCAL_TIMEZONE` | time | `UTC` | Local timezone for time server |
| `DISABLE_THOUGHT_LOGGING` | sequentialthinking | `false` | Disable thought step logging |

### Docker Volume Mounts

| Server | Mount | Purpose |
|--------|-------|---------|
| filesystem | Host directories -> `/mnt/*` | Directories the server can access (read-only or read-write) |
| memory | Named volume -> `/data` | Persistent knowledge graph storage |
| git | Repository path -> `/repo` | Git repository to operate on |

### MCP Client Configuration

To use these servers with MCP-compatible clients, configure them in the client's MCP settings:

**stdio transport (recommended):**

```json
{
  "mcpServers": {
    "memory": {
      "command": "docker",
      "args": ["run", "-i", "-v", "mcp-memory-data:/data", "--rm", "mcp/memory"]
    },
    "sequentialthinking": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "mcp/sequentialthinking"]
    },
    "everything": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "mcp/everything"]
    }
  }
}
```

**npx (without Docker):**

```json
{
  "mcpServers": {
    "memory": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-memory"]
    },
    "sequential-thinking": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sequential-thinking"]
    }
  }
}
```

## Deployment Instructions

### Building All Server Images

From the `MCP-Servers/` directory:

```bash
# Build TypeScript servers
docker build -t mcp/everything -f src/everything/Dockerfile .
docker build -t mcp/filesystem -f src/filesystem/Dockerfile .
docker build -t mcp/memory -f src/memory/Dockerfile .
docker build -t mcp/sequentialthinking -f src/sequentialthinking/Dockerfile .

# Build Python servers (from individual server directories)
docker build -t mcp/fetch -f src/fetch/Dockerfile src/fetch/
docker build -t mcp/git -f src/git/Dockerfile src/git/
docker build -t mcp/time -f src/time/Dockerfile src/time/
```

### Running with Docker Compose

The primary deployment method is through the HelixAgent MCP Docker Compose configuration:

```bash
# Start all MCP servers
docker-compose -f MCP/docker-compose.yml up -d

# Start only reference servers (core)
docker-compose -f MCP/docker-compose.yml up -d \
  mcp-everything mcp-filesystem mcp-memory \
  mcp-sequential-thinking mcp-fetch mcp-git mcp-time

# View logs
docker-compose -f MCP/docker-compose.yml logs -f mcp-memory

# Stop all servers
docker-compose -f MCP/docker-compose.yml down
```

### Running with Podman

```bash
# Podman Compose equivalent
podman-compose -f MCP/docker-compose.yml up -d

# Individual server
podman run -i --rm mcp/memory
```

### Health Verification

After deployment, verify servers are healthy:

```bash
# Check container status
docker ps --filter "name=mcp-"

# Test stdio communication (memory server example)
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}' | \
  docker exec -i mcp-memory node dist/index.js
```

## HelixAgent Integration

### Container Orchestration

In production, MCP-Servers containers are managed by HelixAgent's boot manager:

1. HelixAgent binary starts and reads `Containers/.env`
2. Boot manager brings up MCP server containers via Docker Compose
3. Health checks verify all MCP servers are responsive
4. CLI agent configurations reference the running containers

Manual container manipulation is forbidden per the project Constitution. All orchestration goes through `./bin/helixagent`.

### Port Assignments

| Server | Container Name | Host Port | Internal Port |
|--------|---------------|-----------|---------------|
| Everything | mcp-everything | 3001 | 3000 |
| Filesystem | mcp-filesystem | 3002 | 3000 |
| Memory | mcp-memory | 3003 | 3000 |
| Sequential Thinking | mcp-sequential-thinking | 3004 | 3000 |
| Fetch | mcp-fetch | 3005 | 3000 |
| Git | mcp-git | 3006 | 3000 |
| Time | mcp-time | 3007 | 3000 |

### Related Files in HelixAgent

| File | Purpose |
|------|---------|
| `MCP/docker-compose.yml` | Docker Compose for all 48 MCP servers |
| `MCP/dockerfiles/Dockerfile.*` | HelixAgent-specific Dockerfile templates |
| `internal/mcp/adapters/` | 45+ MCP adapter implementations |
| `internal/mcp/config/generator_container.go` | Container configuration generator |
| `docker/mcp/docker-compose.mcp-full.yml` | Full MCP fleet compose file |
| `challenges/scripts/cli_agent_mcp_challenge.sh` | MCP integration challenge |

### Upstream Source

This submodule tracks: https://github.com/modelcontextprotocol/servers

Update to latest:

```bash
cd MCP-Servers
git fetch origin
git checkout main
git pull
cd ..
git add MCP-Servers
git commit -m "chore(mcp-servers): update to latest upstream"
```

---

**Last Updated**: April 10, 2026
