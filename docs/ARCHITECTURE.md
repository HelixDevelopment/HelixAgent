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
- **HelixLLM integration**: First-class provider wrapping the HelixLLM submodule's OpenAI-compatible API, RAG capabilities, and multi-provider fallback chain

### HelixLLM Multi-Provider Fallback Chain

HelixAgent's smart routing layer (`internal/handlers/handler.go:processWithDirectProvider`)
dispatches tool-bearing and direct-provider requests to HelixLLM as the primary local
inference endpoint. Inside HelixLLM, the Gateway never calls a brain provider directly —
every completion is routed through a `FallbackChain` (`HelixLLM/internal/fallback/chain.go`)
that is fully transparent to HelixAgent.

#### Request Flow

```
HelixAgent smart routing
    └─► HelixLLM Gateway  (HTTPS / HTTP/3, port 8443)
            └─► FallbackChain
                    ├─► Cloud provider 1  (highest LLMsVerifier score) ──► success → return
                    ├─► Cloud provider 2  (429 or 5xx → skip, try next)
                    ├─► ...
                    ├─► Cloud provider 7  (lowest score)
                    └─► llama.cpp local   (IsLocalFallback=true, always last)
```

#### ScorerBridge — LLMsVerifier Integration

`ScorerBridge` (`HelixLLM/internal/fallback/scorer_bridge.go`) polls the LLMsVerifier
service at `{HELIX_LLM_VERIFIER_URL}/api/scores` every 5 minutes and rebuilds the chain
entry list sorted by composite quality score (descending). Local llama.cpp is pinned at the
end of the chain regardless of score. On any network or decode failure, the bridge falls
back to built-in static scores so the chain always has a usable ordering.

**Default chain ordering (static scores, highest → lowest):**

| Provider | Score | Key Env Var |
|----------|-------|-------------|
| OpenRouter | 90 | `HELIX_LLM_OPENROUTER_KEY` |
| Chutes | 85 | `HELIX_LLM_CHUTES_KEY` |
| HuggingFace Inference | 80 | `HELIX_LLM_HUGGINGFACE_KEY` |
| Nvidia NIM | 75 | `HELIX_LLM_NVIDIA_KEY` |
| Cerebras | 70 | `HELIX_LLM_CEREBRAS_KEY` |
| SambaNova | 65 | `HELIX_LLM_SAMBANOVA_KEY` |
| Together AI | 60 | `HELIX_LLM_TOGETHER_KEY` |
| llama.cpp (local) | 10 — pinned last | *(always available)* |

#### Rate Limit Handling — Transparent to HelixAgent

HelixAgent does not need to implement provider rotation for HelixLLM requests. Two
complementary mechanisms inside HelixLLM handle rate limits automatically:

1. **Reactive failover** (`chain.go`) — a `429 Too Many Requests` response marks the
   entry `Exhausted` with a cooldown derived from the `Retry-After` header, or an
   exponential backoff sequence (60s → 120s → 240s → 480s → capped at 15 minutes). The
   chain immediately advances to the next available entry.
2. **Proactive header parsing** (`RateLimitTracker`, `internal/fallback/rate_limit.go`) —
   parses `X-RateLimit-Remaining-Requests`, `X-RateLimit-Remaining-Tokens`, and
   `X-RateLimit-Reset` on every successful response. When remaining quota drops below
   configured minimum thresholds, the provider is skipped *before* a 429 is ever received.

#### Circuit Breaker

Each cloud chain entry carries an independent `CircuitBreaker`
(`internal/fallback/circuit_breaker.go`):

| State | Trigger | Behavior |
|-------|---------|----------|
| Closed (normal) | — | All requests pass through |
| Open | 3 consecutive failures | All requests skip this provider immediately |
| Half-open | 2 minutes after opening | One probe allowed; success → Closed, failure → Open |

Failure categories that trip the breaker: connection errors, 5xx responses, timeouts.
Rate-limit 429s do **not** count toward the failure threshold — they are handled by
`RateLimitTracker` separately.

#### Key Source Files

| File | Responsibility |
|------|---------------|
| `HelixLLM/internal/fallback/chain.go` | Ordered provider list, retry loop, error aggregation |
| `HelixLLM/internal/fallback/scorer_bridge.go` | LLMsVerifier integration, background score refresh |
| `HelixLLM/internal/fallback/rate_limit.go` | Proactive rate-limit header parsing and deprioritization |
| `HelixLLM/internal/fallback/circuit_breaker.go` | Per-provider circuit breaker (closed/open/half-open) |
| `HelixLLM/internal/fallback/memory_adapter.go` | High-importance memory sync to HelixMemory |
| `internal/llm/providers/helixllm/provider.go` | HelixAgent-side provider wrapping HelixLLM's gateway |

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
- **Remote distribution**: `CONTAINERS_REMOTE_ENABLED=true` in `containers/.env` distributes all containers to remote hosts via SSH; `false` runs everything locally
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
