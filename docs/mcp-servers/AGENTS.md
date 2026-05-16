# AGENTS.md - MCP-Servers Agent Integration

## Overview

This document describes how HelixAgent's 48 CLI agents consume MCP servers from the MCP-Servers submodule. Each CLI agent ships with a minimum of 15 MCP server configurations, drawn from three categories: HelixAgent remote servers, extended servers, and local/free remote servers.

## MCP Server Categories for CLI Agents

### HelixAgent Remote Servers (6 required)

These run as part of the HelixAgent infrastructure and are accessed via the HelixAgent base URL:

| Server         | Endpoint                           | Purpose                          |
|----------------|------------------------------------|----------------------------------|
| mcp            | `http://localhost:7061/v1/mcp`     | Model Context Protocol gateway   |
| acp            | `http://localhost:7061/v1/acp`     | Agent Communication Protocol     |
| lsp            | `http://localhost:7061/v1/lsp`     | Language Server Protocol         |
| embeddings     | `http://localhost:7061/v1/embeddings` | Text embedding generation     |
| vision         | `http://localhost:7061/v1/vision`  | Computer vision analysis         |
| cognee         | `http://localhost:7061/v1/cognee`  | Knowledge graph (optional)       |

### Extended Servers (3 required)

Additional HelixAgent capabilities exposed as MCP endpoints:

| Server         | Endpoint                                | Purpose                     |
|----------------|-----------------------------------------|-----------------------------|
| rag            | `http://localhost:7061/v1/rag`          | Retrieval-Augmented Generation |
| formatters     | `http://localhost:7061/v1/format`       | Code formatting (32+ langs)  |
| monitoring     | `http://localhost:7061/v1/monitoring`   | Health and metrics           |

### Local and Free Remote Servers (6+ required)

Containerized MCP servers from this submodule, plus free remote services:

| Server               | Type         | Source                     |
|----------------------|--------------|----------------------------|
| filesystem           | Container    | mcp_servers/src/filesystem |
| memory               | Container    | mcp_servers/src/memory     |
| sequential-thinking  | Container    | mcp_servers/src/sequentialthinking |
| everything           | Container    | mcp_servers/src/everything |
| puppeteer            | Container    | mcp_servers/src/puppeteer  |
| sqlite               | Container    | mcp_servers/src/sqlite     |
| context7             | Free remote  | context7.com               |
| deepwiki             | Free remote  | deepwiki.com               |
| cloudflare-docs      | Free remote  | Cloudflare                 |

## Agent Dependency Graph

All 48 CLI agents depend on MCP servers in the same hierarchy:

```
CLI Agent Config
  |
  +-- HelixAgent Remote (6)
  |     +-- mcp, acp, lsp, embeddings, vision, cognee
  |
  +-- Extended (3)
  |     +-- rag, formatters, monitoring
  |
  +-- Local Containers (6)
  |     +-- filesystem, memory, sequential-thinking,
  |         everything, puppeteer, sqlite
  |
  +-- Free Remote (3)
        +-- context7, deepwiki, cloudflare-docs
```

### Custom Agents with Extra MCP Servers

Four agents have custom handlers with additional MCP server support:

- **OpenCode** — special handler in `cmd/helixagent/main.go` (`handleGenerateOpenCode`), uses `filterWorkingMCPs` whitelist
- **Crush** — unified generator path with full MCP set
- **KiloCode** — unified generator path with full MCP set
- **HelixCode** — unified generator path with full MCP set

The remaining 44 agents use the generic generator from `LLMsVerifier/llm-verifier/pkg/cliagents/`.

## Adding MCP Server Support to CLI Agents

1. **Register the server** in `internal/mcp/server_registry.go`
2. **Add container config** via `internal/mcp/config/generator_container.go`
3. **Update the unified generator** in `LLMsVerifier/llm-verifier/pkg/cliagents/` to include the new server in the default MCP list
4. **For OpenCode specifically**, add the server name to the `filterWorkingMCPs` whitelist in `cmd/helixagent/main.go`
5. **Regenerate configs** with `./bin/helixagent --generate-agent-config=<agent>`
6. **Validate** with `./challenges/scripts/cli_agent_config_challenge.sh`

## Config Generation Flow

```
./bin/helixagent --generate-agent-config=<agent>
  |
  +-- Is agent "opencode"?
  |     Yes --> handleGenerateOpenCode (special path)
  |     No  --> LLMsVerifier unified generator
  |
  +-- Generator reads .env for API keys
  +-- Generator builds MCP server list (15+ servers)
  +-- Generator outputs JSON config to stdout
  +-- Config includes all MCP server endpoints with transport settings
```

## Key Files

- `LLMsVerifier/llm-verifier/pkg/cliagents/` — Unified config generator for all 48 agents
- `cmd/helixagent/main.go` — Entry point with OpenCode special handler
- `internal/mcp/config/generator_container.go` — Container compose config generator
- `internal/mcp/server_registry.go` — MCP server registry
- `configs/cli-agents/` — Repository example configs (placeholder API keys)
- `challenges/scripts/cli_agent_config_challenge.sh` — Validation (60 tests)
