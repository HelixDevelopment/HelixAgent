# HelixAgent Internal Architecture

## Overview

HelixAgent is an AI-powered ensemble LLM service that combines responses from multiple language models using intelligent aggregation strategies. This document describes the internal architecture, component interactions, and data flow.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Client Layer                                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐│
│  │   REST API  │  │    gRPC     │  │  WebSocket  │  │   SSE Streaming     ││
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘│
└─────────┼────────────────┼────────────────┼─────────────────────┼───────────┘
          │                │                │                     │
          ▼                ▼                ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            Gateway Layer                                     │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                        Gin HTTP Router                                   ││
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐││
│  │  │  Auth   │  │  CORS   │  │ RateLimit│  │ Logging │  │    Timeout      │││
│  │  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘  └────────┬────────┘││
│  └───────┼────────────┼────────────┼────────────┼────────────────┼─────────┘│
└──────────┼────────────┼────────────┼────────────┼────────────────┼──────────┘
           │            │            │            │                │
           ▼            ▼            ▼            ▼                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Handler Layer                                      │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐            │
│  │ Completion │  │   Debate   │  │   Protocol │  │   Health   │            │
│  │  Handler   │  │  Handler   │  │  Handlers  │  │  Handler   │            │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘            │
└────────┼───────────────┼───────────────┼───────────────┼────────────────────┘
         │               │               │               │
         ▼               ▼               ▼               ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Service Layer                                      │
│  ┌───────────────────┐  ┌───────────────────┐  ┌───────────────────────────┐│
│  │  Provider Registry│  │  Ensemble Service │  │    AI Debate Service      ││
│  │  ├─ Claude        │  │  ├─ Voting        │  │    ├─ Multi-Pass Valid.   ││
│  │  ├─ DeepSeek      │  │  ├─ Confidence    │  │    ├─ Team Selection      ││
│  │  ├─ Gemini        │  │  ├─ Parallel      │  │    ├─ Dialogue Format     ││
│  │  ├─ Mistral       │  │  └─ Aggregation   │  │    └─ Consensus Build     ││
│  │  ├─ OpenRouter    │  └───────────────────┘  └───────────────────────────┘│
│  │  ├─ Qwen          │                                                      │
│  │  ├─ ZAI           │  ┌───────────────────┐  ┌───────────────────────────┐│
│  │  ├─ Zen           │  │  Intent Classifier│  │    Plugin System          ││
│  │  ├─ Cerebras      │  │  (LLM-based)      │  │    ├─ Hot Reload          ││
│  │  └─ Ollama        │  └───────────────────┘  │    └─ Dependency Mgmt     ││
│  └───────────────────┘                         └───────────────────────────┘│
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Infrastructure Layer                                │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐            │
│  │ PostgreSQL │  │   Redis    │  │  Background│  │ Observability│           │
│  │ Database   │  │   Cache    │  │  Task Queue│  │  (OTel)    │            │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘            │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Provider Registry (`internal/services/provider_registry.go`)

Central registry for all LLM providers. Manages:
- Provider initialization and lifecycle
- Credential management (API keys, OAuth tokens)
- Health monitoring and circuit breaker state
- Dynamic provider selection based on verification scores

```go
type ProviderRegistry interface {
    Register(name string, provider LLMProvider) error
    Get(name string) (LLMProvider, error)
    List() []ProviderInfo
    HealthCheck(ctx context.Context) map[string]bool
}
```

### 2. Ensemble Service (`internal/services/ensemble.go`)

Orchestrates multi-provider responses:
- Parallel request execution
- Response aggregation strategies
- Confidence-weighted voting
- Fallback handling

**Voting Strategies:**
- `MajorityVote`: Simple majority for consistent answers
- `ConfidenceWeighted`: Weighted by provider confidence scores
- `ConsensusBuilding`: Iterative refinement until consensus

### 3. AI Debate Service (`internal/services/debate_service.go`)

Implements multi-agent debate for complex queries:
- 5 positions × 5 LLMs (1 primary + 4 fallbacks) = 25 participants
- Multi-pass validation (Initial → Validate → Polish → Conclude)
- Dynamic team selection via LLMsVerifier scores

**Debate Flow:**
```
1. Topic Analysis → Extract key aspects
2. Position Assignment → Assign perspectives to LLMs
3. Argument Generation → Each LLM presents arguments
4. Cross-Examination → LLMs critique each other
5. Validation Phase → Verify accuracy
6. Polish Phase → Improve clarity
7. Consensus Building → Synthesize final answer
```

### 4. Startup Verifier (`internal/verifier/`)

Unified verification pipeline run at startup:
1. Discover all providers (API Key + OAuth + Free)
2. Verify providers in parallel (8-test pipeline)
3. Score providers (5-component weighted algorithm)
4. Rank and select debate team

**Scoring Components:**
| Component | Weight | Description |
|-----------|--------|-------------|
| ResponseSpeed | 25% | API latency |
| ModelEfficiency | 20% | Token efficiency |
| CostEffectiveness | 25% | Cost per token |
| Capability | 20% | Model capability |
| Recency | 10% | Model release date |

### 5. Intent Classifier (`internal/services/llm_intent_classifier.go`)

LLM-based semantic intent detection:
- Zero hardcoding - uses AI to understand meaning
- Supports: confirmation, refusal, question, request, clarification
- Falls back to pattern-based classifier when LLM unavailable

## Data Flow

### Completion Request Flow

```
Client Request
    │
    ▼
┌─────────────────┐
│  Auth Middleware│ ─── Validate JWT/API Key
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Rate Limiter    │ ─── Check rate limits by key
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Completion      │
│ Handler         │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Provider        │ ─── Get verified provider by score
│ Registry        │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Cache Layer     │ ─── Check L1 (memory) then L2 (Redis)
└────────┬────────┘
         │
    ┌────┴────┐
    │ Hit?    │
    └────┬────┘
         │
    Yes ─┼─ No
         │    │
         │    ▼
         │ ┌─────────────────┐
         │ │ LLM Provider    │ ─── Execute completion
         │ └────────┬────────┘
         │          │
         │          ▼
         │ ┌─────────────────┐
         │ │ Cache Store     │ ─── Store in L1 + L2
         │ └────────┬────────┘
         │          │
         └────┬─────┘
              │
              ▼
┌─────────────────┐
│ Response        │
│ Formatting      │
└────────┬────────┘
         │
         ▼
    Client Response
```

### AI Debate Flow

```
Debate Request
    │
    ▼
┌─────────────────┐
│ Debate Handler  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Team Selection  │ ─── Select 25 LLMs from verified pool
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Topic Analysis  │ ─── Extract debate aspects
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Position        │ ─── Assign 5 positions to participants
│ Assignment      │
└────────┬────────┘
         │
         ▼
    ┌────────────────────────────────────┐
    │    For each position (parallel)    │
    │ ┌─────────────────────────────────┐│
    │ │ Primary LLM generates argument ││
    │ │         ↓                       ││
    │ │ Fallback 1 if primary fails    ││
    │ │         ↓                       ││
    │ │ Fallback 2 if fallback 1 fails ││
    │ │         ↓                       ││
    │ │ Fallback 3 if fallback 2 fails ││
    │ │         ↓                       ││
    │ │ Fallback 4 if fallback 3 fails ││
    │ └─────────────────────────────────┘│
    └────────────────┬───────────────────┘
                     │
                     ▼
    ┌─────────────────┐
    │ Multi-Pass      │
    │ Validation      │
    │ ┌─────────────┐ │
    │ │ 1. Initial  │ │
    │ │ 2. Validate │ │
    │ │ 3. Polish   │ │
    │ │ 4. Conclude │ │
    │ └─────────────┘ │
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │ Consensus       │ ─── Build final answer
    │ Building        │
    └────────┬────────┘
             │
             ▼
    Debate Response
```

## Package Dependencies

```
cmd/helixagent/
    └─▶ internal/router/
            └─▶ internal/handlers/
                    ├─▶ internal/services/
                    │       ├─▶ internal/llm/providers/
                    │       │       ├─▶ claude/
                    │       │       ├─▶ deepseek/
                    │       │       ├─▶ gemini/
                    │       │       └─▶ ...
                    │       ├─▶ internal/cache/
                    │       └─▶ internal/database/
                    ├─▶ internal/middleware/
                    └─▶ internal/verifier/
                            └─▶ LLMsVerifier/
```

## Protocol Support

### MCP (Model Context Protocol)
- Endpoint: `/v1/mcp`
- 45+ adapters for external services
- Key files: `internal/mcp/adapters/`

### ACP (Agent Communication Protocol)
- Endpoint: `/v1/acp`
- Inter-agent communication
- Key files: `internal/acp/`

### LSP (Language Server Protocol)
- Endpoint: `/v1/lsp`
- Code intelligence features
- Key files: `internal/lsp/`

## Background Task System

### Task Queue (`internal/background/task_queue.go`)

PostgreSQL-backed persistent queue:
- Task states: pending → queued → running → completed/failed/stuck/cancelled
- Priority levels: Low(0), Normal(5), High(10), Critical(15)
- Stuck task detection and recovery

### Worker Pool (`internal/background/worker_pool.go`)

Concurrent task execution:
- Configurable worker count
- Resource monitoring
- Graceful shutdown

### Real-time Notifications (`internal/notifications/`)

- SSE: `GET /v1/tasks/:id/events`
- WebSocket: `GET /v1/ws/tasks/:id`
- Webhooks: Configurable callback URLs

## Caching Architecture

### Two-Tier Cache

```
┌─────────────────────────────────────────────────────┐
│                   Application                        │
│                        │                             │
│                        ▼                             │
│  ┌────────────────────────────────────────────────┐ │
│  │              TieredCache                        │ │
│  │  ┌────────────────┐    ┌────────────────────┐  │ │
│  │  │ L1: MemoryCache│───▶│    L2: RedisCache  │  │ │
│  │  │ (fast, limited)│    │ (shared, larger)   │  │ │
│  │  └────────────────┘    └────────────────────┘  │ │
│  └────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

### Cache Keys

```go
// LLM completion cache
cache.CompletionKey(provider, model, prompt) // "completion:claude:claude-3:hash(prompt)"

// Embedding cache
cache.EmbeddingKey(provider, model, text) // "embedding:openai:text-embedding-3-small:hash(text)"

// Debate result cache
cache.DebateKey(topic, participants) // "debate:hash(topic):hash(participants)"
```

## Security Architecture

### Authentication Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│  Auth MW    │────▶│  Handler    │
└─────────────┘     └──────┬──────┘     └─────────────┘
                           │
            ┌──────────────┼──────────────┐
            ▼              ▼              ▼
    ┌───────────┐  ┌───────────┐  ┌───────────┐
    │  JWT Auth │  │ API Key   │  │  OAuth2   │
    └───────────┘  └───────────┘  └───────────┘
```

### Security Framework (`internal/security/`)

- Red team simulation (40+ attack types)
- Guardrails for input/output
- PII detection and masking
- Audit logging

## Observability

### OpenTelemetry Integration (`internal/observability/`)

- Distributed tracing (Jaeger, Zipkin)
- Metrics collection (Prometheus)
- Log correlation
- Langfuse integration for LLM observability

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `helixagent_request_duration` | Histogram | Request latency |
| `helixagent_provider_calls` | Counter | Calls per provider |
| `helixagent_cache_hits` | Counter | Cache hit rate |
| `helixagent_debate_duration` | Histogram | Debate processing time |
| `helixagent_provider_errors` | Counter | Provider error rate |

## Configuration

### Environment Variables

| Category | Variables |
|----------|-----------|
| Server | `PORT`, `GIN_MODE`, `JWT_SECRET` |
| Database | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` |
| Redis | `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` |
| Providers | `CLAUDE_API_KEY`, `DEEPSEEK_API_KEY`, `GEMINI_API_KEY`, etc. |

### Configuration Files

```
configs/
├── development.yaml   # Development settings
├── production.yaml    # Production settings
└── multi-provider.yaml # Multi-provider setup
```

## File Structure

```
helix_agent/
├── cmd/
│   ├── helixagent/     # Main application entry
│   ├── api/            # API server
│   └── grpc-server/    # gRPC server
├── internal/
│   ├── llm/            # LLM provider abstractions
│   │   └── providers/  # Individual provider implementations
│   ├── services/       # Business logic
│   ├── handlers/       # HTTP handlers
│   ├── middleware/     # HTTP middleware
│   ├── database/       # Data access layer
│   ├── cache/          # Caching layer
│   ├── background/     # Background tasks
│   ├── verifier/       # Startup verification
│   ├── debate/         # Debate orchestrator
│   ├── security/       # Security framework
│   ├── plugins/        # Plugin system
│   └── ...
├── pkg/
│   └── api/            # Generated gRPC/protobuf
├── LLMsVerifier/       # Verification submodule
├── Toolkit/            # Standalone Go library
├── Website/            # Documentation website
└── configs/            # Configuration files
```

---

**Document Version**: 1.0
**Last Updated**: January 23, 2026
**Author**: Generated by Claude Code
