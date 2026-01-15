# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HelixAgent is an AI-powered ensemble LLM service written in Go that combines responses from multiple language models using intelligent aggregation strategies. It provides OpenAI-compatible APIs and supports 10 LLM providers (Claude, DeepSeek, Gemini, Mistral, OpenRouter, Qwen, ZAI, Zen, Cerebras, Ollama) with **dynamic provider selection** based on LLMsVerifier verification scores.

**Module**: `dev.helix.agent` (Go 1.24+, toolchain go1.24.11)

The project includes:
- **Toolkit** (`Toolkit/`): Standalone Go library for building AI applications
- **LLMsVerifier** (`LLMsVerifier/`): Verification system for LLM provider accuracy

## Build Commands

```bash
make build              # Build HelixAgent binary
make build-debug        # Build with debug symbols
make run                # Run locally
make run-dev            # Run in development mode (GIN_MODE=debug)
make docker-build       # Build Docker image
docker-compose up -d    # Start full stack
```

## Testing

```bash
make test                  # Run all tests (auto-detects infrastructure)
make test-coverage         # Tests with HTML coverage report
make test-unit             # Unit tests only (./internal/... -short)
make test-integration      # Integration tests (./tests/integration)
make test-e2e              # End-to-end tests (./tests/e2e)
make test-security         # Security tests (./tests/security)
make test-stress           # Stress tests (./tests/stress)
make test-chaos            # Chaos/challenge tests (./tests/challenge)
make test-bench            # Benchmark tests
make test-race             # Race condition detection
```

Run a single test:
```bash
go test -v -run TestName ./path/to/package
```

Run a single test with infrastructure (PostgreSQL/Redis):
```bash
make test-infra-start

DB_HOST=localhost DB_PORT=15432 DB_USER=helixagent DB_PASSWORD=helixagent123 DB_NAME=helixagent_db \
REDIS_HOST=localhost REDIS_PORT=16379 REDIS_PASSWORD=helixagent123 \
go test -v -run TestName ./path/to/package
```

### Test Infrastructure
```bash
make test-infra-start   # Start PostgreSQL, Redis, Mock LLM containers
make test-infra-stop    # Stop test containers
make test-infra-clean   # Stop and remove volumes
make test-with-infra    # Run all tests with Docker infrastructure
```

## Code Quality

```bash
make fmt              # Format code (go fmt)
make vet              # Static analysis (go vet)
make lint             # Run golangci-lint
make security-scan    # Security scanning (gosec)
make install-deps     # Install dev dependencies
```

## Architecture

### Entry Points
- `cmd/helixagent/` - Main application
- `cmd/api/` - API server
- `cmd/grpc-server/` - gRPC server

### Core Packages (`internal/`)
- `llm/` - LLM provider abstractions and ensemble orchestration
  - `providers/` - Individual implementations (claude, deepseek, gemini, ollama, qwen, zai, openrouter, zen, mistral, cerebras)
  - `ensemble.go` - Ensemble orchestration logic
- `services/` - Business logic
  - Core: provider_registry, ensemble, context_manager, mcp_client, lsp_manager, plugin_system
  - Debate: debate_service, debate_team_config, debate_dialogue, debate_support_types
  - Intent: llm_intent_classifier (LLM-based), intent_classifier (fallback)
- `handlers/` - HTTP handlers & API endpoints
- `background/` - Background command execution (task queue, worker pool, resource monitor, stuck detector)
- `notifications/` - Real-time notifications (SSE, WebSocket, Webhooks, Polling)
- `middleware/` - Auth, rate limiting, CORS, validation
- `cache/` - Caching layer (Redis, in-memory)
- `database/` - PostgreSQL connections and repositories
- `models/` - Data models, enums, protocol types
- `plugins/` - Hot-reloadable plugin system
- `tools/` - Tool schema registry (21 tools)
- `agents/` - CLI agent registry (18 agents)
- `optimization/` - LLM optimization (gptcache, outlines, streaming, sglang, llamaindex, langchain, guidance, lmql)

### Key Interfaces
- `LLMProvider` - Provider implementation contract
- `VotingStrategy` - Ensemble voting strategies
- `PluginRegistry` / `PluginLoader` - Plugin system
- `CacheInterface` - Caching abstraction
- `TaskExecutor` / `TaskQueue` - Background task execution

### Architectural Patterns
- **Provider Registry**: Unified interface for multiple LLM providers with credential management
- **Ensemble Strategy**: Confidence-weighted voting, majority vote, parallel execution
- **AI Debate System**: Multi-round debate between providers for consensus (5 positions x 3 LLMs = 15 total)
- **Plugin System**: Hot-reloadable plugins with dependency resolution
- **Circuit Breaker**: Fault tolerance for provider failures
- **Protocol Managers**: Unified MCP/LSP/ACP protocol handling

## Unified Startup Verification Pipeline

HelixAgent uses LLMsVerifier as the **single source of truth** for all LLM verification and scoring. On startup:

```
1. Load Config & Environment
2. Initialize StartupVerifier (Scoring + Verification + Health)
3. Discover ALL Providers (API Key + OAuth + Free)
4. Verify ALL Providers in Parallel (8-test pipeline)
5. Score ALL Verified Providers (5-component weighted)
6. Rank by Score (OAuth priority when scores close)
7. Select AI Debate Team (15 LLMs: 5 primary + 10 fallback)
8. Start Server with Verified Configuration
```

### Provider Types

| Type | Providers | Auth | Description |
|------|-----------|------|-------------|
| **API Key** | DeepSeek, Gemini, Mistral, OpenRouter, ZAI, Cerebras | Bearer token | Full 8-test verification |
| **OAuth** | Claude, Qwen | OAuth2 tokens from CLI | Trust on API failure option |
| **Free** | Zen (OpenCode), OpenRouter :free models | Anonymous/X-Device-ID | Reduced verification, 6.0-7.0 score range |

### Key Files
- `internal/verifier/startup.go` - Startup verification orchestrator
- `internal/verifier/provider_types.go` - UnifiedProvider, UnifiedModel types
- `internal/verifier/adapters/oauth_adapter.go` - OAuth provider verification (Claude, Qwen)
- `internal/verifier/adapters/free_adapter.go` - Free provider verification (Zen)

### Scoring Algorithm (5 Weighted Components)
| Component | Weight | Description |
|-----------|--------|-------------|
| ResponseSpeed | 25% | API response latency |
| ModelEfficiency | 20% | Token efficiency |
| CostEffectiveness | 25% | Cost per token |
| Capability | 20% | Model capability score |
| Recency | 10% | Model release date |

OAuth providers get +0.5 bonus when verified. Free providers score 6.0-7.0. Minimum score to be selected: 5.0.

## AI Debate Team

The debate system uses dynamic selection via StartupVerifier:
1. OAuth2 providers first (Claude, Qwen) if verified
2. Then LLMsVerifier-scored providers by score
3. 5 positions × 3 LLMs (1 primary + 2 fallbacks) = **15 LLMs**

OAuth primaries get non-OAuth fallbacks to ensure redundancy.

Key files:
- `internal/services/debate_team_config.go` - Team configuration
- `internal/services/debate_dialogue.go` - Dialogue formatter

### Multi-Pass Validation System

The AI Debate system includes a **multi-pass validation** mechanism that re-evaluates, polishes, and improves debate responses before delivering the final consensus:

**Validation Phases:**
| Phase | Icon | Description |
|-------|------|-------------|
| 1. INITIAL RESPONSE | 🔍 | Each AI participant provides their initial perspective |
| 2. VALIDATION | ✓ | Cross-validation of responses for accuracy and completeness |
| 3. POLISH & IMPROVE | ✨ | Refinement and improvement based on validation feedback |
| 4. FINAL CONCLUSION | 📜 | Synthesized consensus with confidence scores |

**API Integration:**
```json
POST /v1/debates
{
  "topic": "Should AI have consciousness?",
  "participants": [...],
  "enable_multi_pass_validation": true,
  "validation_config": {
    "enable_validation": true,
    "enable_polish": true,
    "validation_timeout": 120,
    "polish_timeout": 60,
    "min_confidence_to_skip": 0.9,
    "max_validation_rounds": 3,
    "show_phase_indicators": true
  }
}
```

**Response includes:**
- `current_phase` - Current validation phase (when running)
- `multi_pass_result` - Detailed results including:
  - `phases_completed` - Number of completed phases
  - `overall_confidence` - Final confidence score (0-1)
  - `quality_improvement` - Quality improvement from initial to final
  - `final_response` - The polished, validated consensus

**Key Files:**
- `internal/services/debate_multipass_validation.go` - Core multi-pass validation system
- `internal/services/debate_multipass_validation_test.go` - Unit tests
- `internal/handlers/debate_handler.go` - API integration
- `challenges/scripts/multipass_validation_challenge.sh` - 66-test validation

### Semantic Intent Detection (ZERO Hardcoding)

HelixAgent uses **LLM-based semantic intent classification** to understand user messages. When a user confirms, refuses, or asks questions, the system uses AI to understand the semantic meaning - not pattern matching.

**Architecture:**
1. **Primary**: LLM-based classification (`llm_intent_classifier.go`) - Uses AI debate team members to semantically understand user intent
2. **Fallback**: Pattern-based classifier (`intent_classifier.go`) - Only used when LLM unavailable

**Intent Types:**
| Intent | Description | Examples |
|--------|-------------|----------|
| `confirmation` | User approves/confirms action | "Yes", "Go ahead", "Let's do all points!" |
| `refusal` | User declines/refuses action | "No", "Stop", "Cancel that" |
| `question` | User asks for information | "What do you mean?", "How does this work?" |
| `request` | User makes a new request | "Help me with X" |
| `clarification` | User needs more info | "I'm confused about this" |
| `unclear` | Cannot determine intent | Ambiguous messages |

**Key Principles (NO HARDCODING):**
- User intent detected by semantic meaning, not exact string matching
- Short positive responses with context = likely confirmation
- LLM classifies with JSON structured output (intent, confidence, is_actionable, should_proceed)
- Fallback uses semantic roots and word stems, not exact patterns

**Key Files:**
- `internal/services/llm_intent_classifier.go` - LLM-based classification (primary)
- `internal/services/intent_classifier.go` - Pattern-based fallback
- `internal/services/intent_classifier_test.go` - Comprehensive test suite (100+ test cases)
- `internal/services/debate_service.go` - Integration with `classifyUserIntent()`

**Challenge Validation:**
```bash
./challenges/scripts/semantic_intent_challenge.sh  # 19 tests - validates zero hardcoding
```

## Configuration

Environment variables in `.env.example`:
- Server: `PORT`, `GIN_MODE`, `JWT_SECRET`
- Database: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
- Redis: `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`
- LLM providers: `CLAUDE_API_KEY`, `DEEPSEEK_API_KEY`, `GEMINI_API_KEY`, `OPENCODE_API_KEY` (Zen), etc.
- OAuth2: `CLAUDE_CODE_USE_OAUTH_CREDENTIALS`, `QWEN_CODE_USE_OAUTH_CREDENTIALS`
- Cognee: `COGNEE_AUTH_EMAIL`, `COGNEE_AUTH_PASSWORD` (form-encoded OAuth2 auth)

### OAuth2 Authentication (Limitations)

**IMPORTANT: OAuth tokens from CLI tools are product-restricted and cannot be used for general API calls.**

| Provider | Token Source | API Access |
|----------|--------------|------------|
| **Claude** | `~/.claude/.credentials.json` (from `claude auth login`) | ❌ **Restricted to Claude Code only** - cannot use for general API |
| **Qwen** | `~/.qwen/oauth_creds.json` (from Qwen CLI login) | ❌ **For Qwen Portal only** - DashScope API requires separate API key |

**What works:**
- HelixAgent successfully reads OAuth tokens from both credential files
- Tokens are valid and non-expired

**What doesn't work:**
- Using Claude OAuth tokens for general API requests returns: _"This credential is only authorized for use with Claude Code and cannot be used for other API requests."_
- Using Qwen OAuth tokens for DashScope API returns: _"invalid_api_key"_ (tokens are for `portal.qwen.ai`)

**Solution:**
- **Claude**: Get an API key from https://console.anthropic.com/
- **Qwen**: Get a DashScope API key from https://dashscope.aliyuncs.com/

Key files: `internal/auth/oauth_credentials/`

### Cognee Authentication

Default credentials (development only):
```
Email:    admin@helixagent.ai
Password: HelixAgentPass123
```

Cognee uses form-encoded OAuth2-style login (NOT JSON).

## Adding a New LLM Provider

1. Create provider package: `internal/llm/providers/<name>/<name>.go`
2. Implement `LLMProvider` interface (Complete, CompleteStream, HealthCheck, GetCapabilities, ValidateConfig)
3. Add tool support if provider API supports it:
   - Define `<Provider>Tool`, `<Provider>ToolCall` types
   - Add `Tools` field to request, `ToolCalls` to response
   - Set `SupportsTools: true` in GetCapabilities
4. Register in `internal/services/provider_registry.go`
5. Add environment variables to `.env.example`
6. Add tests in `internal/llm/providers/<name>/<name>_test.go`

## Tool Schema (21 Tools)

All tool parameters use **snake_case** (e.g., `file_path`, `old_string`). Key files:
- `internal/tools/schema.go` - Tool schema registry
- `internal/tools/handler.go` - Tool handlers

Required fields per tool:
- Bash: `command`, `description`
- Read/Write/Edit: `file_path` (+ `content` for Write, + `old_string`/`new_string` for Edit)
- Glob/Grep: `pattern`
- WebFetch: `url`, `prompt`
- WebSearch: `query`
- Git: `operation`, `description`

## CLI Agent Registry (18 Agents)

Registry in `internal/agents/registry.go` supports: OpenCode, Crush, HelixCode, Kiro, Aider, ClaudeCode, Cline, CodenameGoose, DeepSeekCLI, Forge, GeminiCLI, GPTEngineer, KiloCode, MistralCode, OllamaCode, Plandex, QwenCode, AmazonQ.

## Challenges System

```bash
./challenges/scripts/run_all_challenges.sh                       # Run all challenges
./challenges/scripts/main_challenge.sh                           # Main challenge (generates OpenCode config)
./challenges/scripts/unified_verification_challenge.sh           # 15 tests - startup pipeline
./challenges/scripts/debate_team_dynamic_selection_challenge.sh  # 12 tests - team selection
./challenges/scripts/free_provider_fallback_challenge.sh         # 8 tests - Zen/free models
./challenges/scripts/semantic_intent_challenge.sh                # 19 tests - intent detection (ZERO hardcoding)
./challenges/scripts/fallback_mechanism_challenge.sh             # 17 tests - fallback chain for empty responses
```

Key concepts:
- HelixAgent presents as a single LLM provider with one virtual model (AI Debate Ensemble)
- ALL verification data comes from REAL API calls (no stubs)
- Infrastructure auto-starts when needed

## LLMsVerifier Integration

```bash
make verifier-init        # Initialize submodule
make verifier-build       # Build verifier CLI
make verifier-test        # Run verifier tests
make verifier-verify MODEL=gpt-4 PROVIDER=openai  # Verify a model
```

Dynamic provider selection based on real-time verification scores. Ollama is DEPRECATED (score: 5.0) - only used as fallback.

## Protocol Support

| Protocol | Endpoint | Description |
|----------|----------|-------------|
| MCP | `/v1/mcp` | Model Context Protocol |
| ACP | `/v1/acp` | Agent Communication Protocol |
| LSP | `/v1/lsp` | Language Server Protocol |
| Embeddings | `/v1/embeddings` | Vector embeddings |
| Vision | `/v1/vision` | Image analysis, OCR |
| Cognee | `/v1/cognee` | Knowledge graph & RAG |

Fallback mechanism: Routes to strongest LLM by LLMsVerifier score, falls back to next on failure.

## Background Task System

API endpoints: `POST /v1/tasks`, `GET /v1/tasks/:id/status`, `GET /v1/tasks/:id/events` (SSE), `GET /v1/ws/tasks/:id` (WebSocket)

Task states: `pending -> queued -> running -> completed/failed/stuck/cancelled`

Key files: `internal/background/`, `internal/notifications/`

## Container Runtime

Supports both Docker and Podman:
```bash
./scripts/container-runtime.sh build   # Auto-detects runtime
./scripts/container-runtime.sh start
```

## Technology Stack

- **Framework**: Gin (v1.11.0)
- **Database**: PostgreSQL 15 with pgx/v5
- **Cache**: Redis 7
- **Protocols**: OpenAI-compatible REST, gRPC, MCP, LSP, ACP
- **Testing**: testify (v1.11.1)
- **Monitoring**: Prometheus, Grafana

Configuration files in `/configs`: `development.yaml`, `production.yaml`, `multi-provider.yaml`
