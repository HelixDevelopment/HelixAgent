# HelixCode — Distributed AI Development Platform

**Module:** `dev.helix.code` | **Language:** Go 1.25.2 | **License:** MIT

## Overview

HelixCode is an enterprise-grade distributed AI development platform with SSH-based worker pools, 15+ LLM providers, 40+ development tools, and MCP protocol support.

## Architecture

- **40+ internal packages** — auth, config, database, llm, mcp, tools, worker, task, etc.
- **15+ LLM providers** — Ollama, Llama.cpp, vLLM, OpenAI, Claude, Gemini, xAI, Groq, OpenRouter
- **40+ tools** — filesystem, shell, web, browser, git, codebase mapping, multi-edit
- **9 memory systems** — Mem0, Zep, ChromaDB, FAISS, Pinecone, Qdrant, Weaviate
- **Multi-client** — REST API, CLI, TUI, Desktop, WebSocket, MCP

## Entry Points

| Binary | Path | Purpose |
|--------|------|---------|
| helixcode-server | `cmd/server/main.go` | HTTP server (port 8080) |
| helix-cli | `cmd/cli/main.go` | CLI client |
| helix-config | `cmd/helix-config/` | Config management |

## API Endpoints

Base: `http://localhost:8080/api/v1`

- `POST /auth/register`, `/auth/login`, `/auth/refresh`
- `GET/POST /workers`, `/workers/:id/heartbeat`, `/workers/:id/metrics`
- `GET/POST /tasks`, `/tasks/:id/start`, `/tasks/:id/checkpoint`
- `GET/POST /projects`, `/projects/:id/workflows/{planning,building,testing}`
- `GET/POST /sessions`
- `GET /llm/providers`, `/llm/models`
- `GET /memory/systems`, `/memory/stats`
- `GET /ws` — WebSocket + MCP

## Configuration

Primary: `config/config.yaml` (Viper-based, env var overrides)

Key env vars: `HELIX_AUTH_JWT_SECRET`, `HELIX_DATABASE_PASSWORD`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`

## Build & Test

```bash
cd HelixCode
make build          # Build server
make test           # Run all tests
make prod           # Cross-platform builds
go test -cover ./... # Coverage
```

## HelixAgent Integration

- Submodule at `cli_agents/HelixCode`
- Config generation via `helixagent --generate-agent-config=helixcode`
- Protocol integration via REST API + MCP
