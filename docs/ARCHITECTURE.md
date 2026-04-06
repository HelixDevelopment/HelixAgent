# HelixAgent Architecture

**Version:** 1.3.0  
**Last Updated:** 2026-04-06

## Overview

HelixAgent is an AI-powered ensemble LLM service written in Go that combines responses from multiple language models using intelligent aggregation strategies.

## Core Architecture

### Entry Points

| Component | Location | Description |
|-----------|----------|-------------|
| Main Application | `cmd/helixagent/` | Primary server binary |
| API Server | `cmd/api/` | REST API server |
| gRPC Server | `cmd/grpc-server/` | gRPC protocol server |
| Cognee Mock | `cmd/cognee-mock/` | Mock Cognee service |
| Sanity Check | `cmd/sanity-check/` | System validation tool |
| MCP Bridge | `cmd/mcp-bridge/` | MCP protocol bridge |
| Constitution Generator | `cmd/generate-constitution/` | Constitution file generator |

### Internal Packages

```
internal/
├── adapters/          # Bridge layer to extracted modules
├── agents/            # CLI agent registry (48 agents)
├── background/        # Task queue, worker pool, resource monitor
├── bigdata/           # Infinite context, distributed memory
├── cache/             # Redis + in-memory caching
├── concurrency/       # Worker pools, semaphores, rate limiters
├── database/          # PostgreSQL/pgx repositories
├── debate/            # AI debate orchestrator framework
├── embedding/         # 6 embedding providers
├── formatters/        # 32+ code formatters
├── handlers/          # HTTP handlers
├── challenges/        # HelixAgent-specific challenge implementations
├── llm/               # 48 LLM providers + discovery + ensemble
├── mcp/               # MCP adapters (19+) + config generator
├── memory/            # Mem0-style with entity graphs
├── messaging/         # Kafka + RabbitMQ abstraction
├── middleware/        # Auth, rate limiting, CORS
├── models/            # Data models and enums
├── notifications/     # SSE, WebSocket, Webhooks
├── observability/     # OpenTelemetry, Jaeger, Zipkin
├── optimization/      # GPT-Cache, Outlines, SGLang
├── plugins/           # Hot-reloadable plugin system
├── rag/               # Hybrid retrieval
├── routing/           # Semantic routing
├── security/          # Red team framework, guardrails
├── services/          # Business logic
├── streaming/         # SSE, WebSocket, gRPC streaming
├── tools/             # Tool schema registry (21 tools)
├── vectordb/          # Qdrant, Pinecone, Milvus, pgvector
├── verifier/          # Startup verification orchestrator
├── agentic/           # Graph-based workflow orchestration
├── llmops/            # LLM operations (eval, A/B testing)
├── selfimprove/       # RLHF-style self-improvement
├── planning/          # HiPlan, MCTS, Tree of Thoughts
├── benchmark/         # SWE-bench, HumanEval, MMLU
└── structured/        # Structured output (JSON Schema, regex, grammar)
```

## Key Components

### LLM Provider Registry

- **48 dedicated providers**: AI21, Anthropic, Anthropic CU, Azure OpenAI, Cerebras, Chutes, Claude, Cloudflare, Codestral, Cohere, DeepSeek, Fireworks, Gemini (unified: API+CLI+ACP), GitHub Models, Groq, HelixLLM, HuggingFace, Hyperbolic, Junie, Kilo, Kimi, KimiCode, LM Studio, Mistral, Modal, Nia, NLPCloud, Novita, NVIDIA, Ollama, OpenAI, OpenRouter, Perplexity, PublicAI, Qwen, Replicate, SambaNova, Sarvam, SiliconFlow, Together, Upstage, Venice, Vertex AI, VulaVula, xAI, ZAI, Zen, Zhipu
- **Generic OpenAI-compatible**: Provider for verification of providers without dedicated implementations
- **Dynamic model discovery**: 3-tier (Provider API → models.dev → hardcoded fallback)
- **HelixLLM integration**: First-class provider wrapping the HelixLLM submodule's OpenAI-compatible API and RAG capabilities

### AI Debate System

- **5 positions × 5 LLMs** = 25 total LLMs
- **Multi-pass validation**: Initial → Validation → Polish → Final
- **Orchestrator**: Multi-topology (mesh/star/chain/tree), 8-phase protocol (Dehallucination → SelfEvolvement → Proposal → Critique → Review → Optimization → Adversarial → Convergence)
- **6 voting methods**: Weighted (MiniMax), Majority, Borda Count, Condorcet, Plurality, Unanimous
- **Reflexion framework**: Episodic memory, verbal reflection, retry-and-learn loop
- **Adversarial dynamics**: Red/Blue team attack-defend cycles
- **Approval gates**: Configurable human-in-the-loop with REST API
- **Performance optimizer**: Parallel LLM execution, response caching, early termination on consensus

### AgenticEnsemble

- **Intelligent mode classification**: Automatically routes requests to single-provider, ensemble, tool-augmented debate, or full agentic execution loop
- **Tool-augmented debate**: Combines debate orchestration with tool calling for grounded reasoning
- **Agentic execution loop**: Plan-execute-verify cycle with task decomposition, layered execution, and result synthesis

### Extracted Modules

41 independent modules with zero shared dependencies:

| Phase | Modules |
|-------|---------|
| Foundation | EventBus, Concurrency, Observability, Auth, Storage, Streaming, ToolSchema, SkillRegistry, Models |
| Infrastructure | Security, VectorDB, Embeddings, Database, Cache, LLMProvider |
| Services | Messaging, Formatters, MCP, BackgroundTasks |
| Integration | RAG, ConversationContext, Memory, Optimization, Plugins |
| AI/ML | Agentic, LLMOps, SelfImprove, Planning, Benchmark, DebateOrchestrator |
| Cognitive | HelixMemory |
| Specification | HelixSpecifier |
| Pre-existing | Containers, Challenges, BuildCheck, DocProcessor, HelixQA, LLMOrchestrator, VisionEngine, LLMsVerifier, MCP-Servers |

## Data Flow

```
Request → Handler → Middleware (Brotli/gzip) → Service Layer
    ↓
AgenticEnsemble → Mode Classification → Route Decision
    ↓                                       ↓
Single Provider    Ensemble (3-5)    Debate (25 LLMs)    Agentic Loop
    ↓                   ↓                  ↓                   ↓
Provider Registry → LLM Provider (48) → Response
    ↓
Cache → Database → Response
```

## Networking

- **Primary transport**: HTTP/3 (QUIC) via `quic-go/quic-go`
- **Fallback**: HTTP/2 when HTTP/3 is unavailable
- **Compression**: Brotli (primary, via `andybalholm/brotli`) → gzip (fallback)
- **All HTTP clients and servers prefer HTTP/3**

## Container Orchestration

- **Container Runtime**: Docker / Podman / Kubernetes (auto-detected)
- **Centralized management**: All container operations through the Containers module (`digital.vasic.containers`) via `internal/adapters/containers/adapter.go`
- **Remote distribution**: `CONTAINERS_REMOTE_ENABLED=true` in `Containers/.env` distributes all containers to remote hosts via SSH; `false` runs everything locally
- **Health monitoring**: TCP/HTTP checks with circuit breakers; required services fail boot on health failure in strict mode

## Deployment

- **Container Runtime**: Docker / Podman / Kubernetes
- **Build**: All release builds in containers for reproducibility
- **Configuration**: YAML files + environment variables
- **7 Apps**: helixagent, api, grpc-server, cognee-mock, sanity-check, mcp-bridge, generate-constitution
- **5 Platforms**: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64

## See Also

- [MODULES.md](MODULES.md) - Detailed module documentation
- [CLAUDE.md](../CLAUDE.md) - Development guidelines
- [AGENTS.md](../AGENTS.md) - Agent configuration
