# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HelixAgent is an AI-powered ensemble LLM service written in Go (1.24+) that combines responses from multiple language models using intelligent aggregation strategies. It provides OpenAI-compatible APIs and supports 18+ LLM providers with **dynamic provider selection** based on LLMsVerifier verification scores. Main providers: Claude, DeepSeek, Gemini, Qwen, ZAI, OpenRouter, Mistral, Cerebras, and more.

The project also includes:
- **Toolkit** (`Toolkit/`): A standalone Go library for building AI applications with multi-provider support
- **LLMsVerifier** (`LLMsVerifier/`): A verification system for LLM provider accuracy and reliability

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
make test                  # Run all tests
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

### Test Infrastructure (Docker-based)
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
make install-deps     # Install dev dependencies (golangci-lint, gosec)
```

## Architecture

### Entry Points
- `cmd/helixagent/` - Main HelixAgent application
- `cmd/api/` - API server
- `cmd/grpc-server/` - gRPC server

### Core Packages (`internal/`)
- `llm/` - LLM provider abstractions and ensemble orchestration
  - `providers/` - Individual implementations (claude, deepseek, gemini, ollama, qwen, zai, openrouter)
  - `ensemble.go` - Ensemble orchestration logic
- `services/` - Business logic
  - `provider_registry.go` - Provider management
  - `ensemble.go` - Ensemble service
  - `context_manager.go` - Multi-source context aggregation
  - `mcp_client.go` - Model Context Protocol client
  - `lsp_manager.go` - Language Server Protocol manager
  - `plugin_system.go` - Hot-reloadable plugin architecture
- `handlers/` - HTTP handlers & API endpoints (OpenAI-compatible, MCP, LSP, Cognee, AI Debate, Background Tasks)
- `background/` - Background command execution engine (task queue, worker pool, resource monitor, stuck detector)
- `notifications/` - Real-time notifications (SSE, WebSocket, Webhooks, Polling, CLI rendering)
- `middleware/` - Auth, rate limiting, CORS, validation
- `cache/` - Caching layer (Redis, in-memory)
- `database/` - PostgreSQL connections and repositories
  - `protocol_repository.go` - MCP/LSP/ACP server configs, protocol cache, and metrics
- `models/` - Data models, enums, and protocol types
- `plugins/` - Hot-reloadable plugin system with discovery, health, metrics
- `modelsdev/` - Models.dev API client for model metadata

### Key Interfaces (Extensibility Points)
- `LLMProvider` - Provider implementation contract
- `VotingStrategy` - Ensemble voting strategies
- `PluginRegistry` / `PluginLoader` - Plugin system
- `CacheInterface` - Caching abstraction
- `CloudProvider` - Cloud integration
- `TaskExecutor` / `TaskQueue` - Background task execution
- `ResourceMonitor` / `StuckDetector` - Task monitoring
- `NotificationHub` - Real-time event distribution

### Architectural Patterns
- **Provider Registry**: Unified interface for multiple LLM providers with credential management
- **Ensemble Strategy**: Confidence-weighted voting, majority vote, parallel execution
- **AI Debate System**: Multi-round debate between providers for consensus (see `internal/services/debate_*.go`)
  - API: POST `/v1/debates`, GET `/v1/debates`, GET `/v1/debates/:id`, DELETE `/v1/debates/:id`
  - Supports async execution with status polling via `/v1/debates/:id/status`
  - **Team Configuration**: See `internal/services/debate_team_config.go` for team composition
  - **Dialogue Rendering**: Theatrical dialogue presentation in streaming responses (see below)

### AI Debate Team Composition (15 LLMs Total)

The AI Debate Team uses a dynamic selection algorithm:

1. **OAuth2 Providers First**: Claude and Qwen (if verified by LLMsVerifier)
2. **LLMsVerifier Scored**: Remaining positions filled with best-scored verified providers
3. **Reuse Allowed**: Same LLM can be used in multiple instances if needed
4. **Total**: 5 positions × 3 LLMs each (1 primary + 2 fallbacks) = **15 LLMs**

**Selection Algorithm:**
```
1. Verify all providers via LLMsVerifier
2. Collect OAuth2 models (Claude, Qwen) if verified
3. Collect LLMsVerifier-scored providers (sorted by score)
4. Sort all: OAuth first, then by score (highest first)
5. Assign top 5 to primary positions
6. Assign next best as fallbacks (2 per position)
```

**Available Model Pools:**

| Provider Type | Models | Count |
|---------------|--------|-------|
| **Claude (OAuth2)** | Sonnet Latest, Opus, Haiku | 3 |
| **Qwen (OAuth2)** | Max, Plus, Turbo, Coder, Long | 5 |
| **LLMsVerifier** | DeepSeek, Gemini, Mistral, Groq, Cerebras | 5 |
| **Total Available** | | **13** |

**Model Definitions:**

```go
// OAuth2 Providers (prioritized if verified)
ClaudeModels.SonnetLatest = "claude-3-5-sonnet-20241022"  // Score: 9.5
ClaudeModels.Opus         = "claude-3-opus-20240229"      // Score: 9.5
ClaudeModels.Haiku        = "claude-3-haiku-20240307"     // Score: 8.5

QwenModels.Max   = "qwen-max"          // Score: 8.0
QwenModels.Plus  = "qwen-plus"         // Score: 7.8
QwenModels.Turbo = "qwen-turbo"        // Score: 7.5
QwenModels.Coder = "qwen-coder-turbo"  // Score: 7.5
QwenModels.Long  = "qwen-long"         // Score: 7.5

// LLMsVerifier Scored Providers (fill remaining positions)
LLMsVerifierModels.DeepSeek = "deepseek-chat"           // Score: 8.5
LLMsVerifierModels.Gemini   = "gemini-2.0-flash"        // Score: 9.0
LLMsVerifierModels.Mistral  = "mistral-large-latest"    // Score: 8.5
LLMsVerifierModels.Groq     = "llama-3.1-70b-versatile" // Score: 8.0
LLMsVerifierModels.Cerebras = "llama3.1-70b"            // Score: 7.5
```

**Key Constants:**
```go
TotalDebatePositions = 5   // 5 debate positions
FallbacksPerPosition = 2   // 2 fallbacks per position
TotalDebateLLMs      = 15  // 5 × (1 + 2) = 15 LLMs total
```

**Key Files:**
- `internal/services/debate_team_config.go` - Team configuration and dynamic selection
- `internal/services/debate_team_config_test.go` - Comprehensive unit tests

**Provider Verification:**
- All providers verified on startup via LLMsVerifier
- OAuth2 tokens validated (expired/invalid credentials handled)
- Invalid providers trigger automatic fallback activation
- Same LLM can fill multiple slots if insufficient verified providers

### AI Debate Dialogue Rendering

HelixAgent provides theatrical dialogue presentation for AI debate ensemble responses. Every streaming response includes:

**Response Structure:**
```
╔══════════════════════════════════════════════════════════════════╗
║           🎭 HELIXAGENT AI DEBATE ENSEMBLE 🎭                    ║
╠══════════════════════════════════════════════════════════════════╣
║  Five AI minds deliberate to synthesize the optimal response.    ║
╚══════════════════════════════════════════════════════════════════╝

📋 TOPIC: [User's query]

═══════════════════════════════════════════════════════════════════
                         DRAMATIS PERSONAE
═══════════════════════════════════════════════════════════════════
  [A]  THE ANALYST      │ [model] ([provider])
  [P]  THE PROPOSER     │ [model] ([provider])
  [C]  THE CRITIC       │ [model] ([provider])
  [S]  THE SYNTHESIZER  │ [model] ([provider])
  [M]  THE MEDIATOR     │ [model] ([provider])

═══════════════════════════════════════════════════════════════════
                        THE DELIBERATION
═══════════════════════════════════════════════════════════════════

[A] THE ANALYST: "Let me analyze this systematically..."
[P] THE PROPOSER: "I propose we approach this..."
[C] THE CRITIC: "I must challenge some assumptions..."
[S] THE SYNTHESIZER: "Combining these perspectives..."
[M] THE MEDIATOR: "After weighing all arguments..."

═══════════════════════════════════════════════════════════════════
                      📜 CONSENSUS REACHED 📜
═══════════════════════════════════════════════════════════════════

[Final synthesized response from the AI debate ensemble]

───────────────────────────────────────────────────────────────────
         ✨ Powered by HelixAgent AI Debate Ensemble ✨
───────────────────────────────────────────────────────────────────
```

**Dialogue Styles:**
| Style | Description |
|-------|-------------|
| `theater` | Default theatrical presentation (shown above) |
| `novel` | Novel-style prose narration |
| `screenplay` | Screenplay/script format |
| `minimal` | Minimal formatting |

**Key Files:**
- `internal/services/debate_dialogue.go` - Dialogue formatter
- `internal/services/debate_dialogue_test.go` - Comprehensive tests
- `internal/handlers/openai_compatible.go` - Streaming integration

- **Plugin System**: Hot-reloadable plugins with dependency resolution
- **Circuit Breaker**: Fault tolerance for provider failures with health monitoring
- **Protocol Managers**: Unified MCP/LSP/ACP protocol handling
- **Cognee Integration**: Knowledge graph and RAG capabilities
- **Middleware Chain**: Auth, rate limiting, validation pipeline
- **LLM Optimization**: Semantic caching, structured output, enhanced streaming (see below)
- **Background Execution**: Parallel task execution with adaptive worker pool (see below)

### Protocol Support and Capabilities

HelixAgent exposes comprehensive protocol support with automatic fallback mechanism:

**Supported Protocols:**
| Protocol | Endpoint | Description |
|----------|----------|-------------|
| MCP | `/v1/mcp` | Model Context Protocol |
| ACP | `/v1/acp` | Agent Communication Protocol |
| LSP | `/v1/lsp` | Language Server Protocol |
| Embeddings | `/v1/embeddings` | Vector embeddings generation |
| Vision | `/v1/vision` | Image analysis, OCR |
| Cognee | `/v1/cognee` | Knowledge graph & RAG |

**Full Model Capabilities:**
| Capability | Description |
|------------|-------------|
| `attachments` | File attachments support |
| `reasoning` | Extended reasoning/thinking |
| `vision` | Image input processing |
| `imageInput/Output` | Image generation and analysis |
| `ocr` | Optical character recognition |
| `pdf` | PDF document processing |
| `audio` | Audio input/output |
| `video` | Video processing |
| `streaming` | Real-time streaming responses |
| `functionCalls` | Function/tool calling |
| `embeddings` | Vector embeddings |
| `codeExecution` | Code execution sandbox |
| `webBrowsing` | Web browsing capability |
| `fileUpload` | File upload (no limit) |

**Protocol Fallback Mechanism:**
```
1. Request comes to protocol endpoint (e.g., /v1/embeddings)
2. Route to strongest LLM supporting the protocol (by LLMsVerifier score)
3. If primary fails → fallback to next strongest LLM
4. Continue fallback chain until success or all LLMs exhausted
5. Fallback order: OAuth providers first, then by score
```

**OpenCode Configuration:**
The generated configuration at `/home/milosvasic/.config/opencode/opencode.json` includes:
- 12 MCP server endpoints (6 HelixAgent + 6 standard)
- 4 specialized agents (default, code-reviewer, embeddings, vision)
- All protocol tools enabled
- Full capability declaration

**Key Files:**
- `challenges/codebase/go_files/opencode_generator/` - Configuration generator
- `internal/handlers/` - Protocol endpoint handlers
- `internal/services/protocol_*.go` - Protocol implementations

### LLM Optimization (`internal/optimization/`)

HelixAgent integrates 8 LLM optimization tools for performance and quality:

| Package | Purpose | Key Features |
|---------|---------|--------------|
| `gptcache/` | Semantic caching | Vector similarity, LRU eviction, TTL |
| `outlines/` | Structured output | JSON schema validation, regex patterns, choice constraints |
| `streaming/` | Enhanced streaming | Word/sentence buffering, progress tracking, rate limiting |
| `sglang/` | Prefix caching | RadixAttention, session management (GPU required) |
| `llamaindex/` | Document retrieval | HyDE, reranking, Cognee integration |
| `langchain/` | Task decomposition | Chain execution, ReAct agents |
| `guidance/` | Grammar constraints | CFG-based generation, templates |
| `lmql/` | Query language | Declarative constraints, decoding strategies |

**Start optimization services:**
```bash
docker-compose --profile optimization up -d     # CPU-only optimization
docker-compose --profile optimization-gpu up -d # With GPU support (SGLang)
```

**Configuration**: See `configs/production.yaml` under `optimization:` section.

**Documentation**: See `docs/optimization/` and `docs/guides/LLM_OPTIMIZATION_USER_GUIDE.md`.

### Background Command Execution System (`internal/background/`)

HelixAgent provides a comprehensive background command execution system for parallel task execution triggered by Tooling and AI Debate Team:

**Core Components:**
| Package | Purpose |
|---------|---------|
| `internal/background/interfaces.go` | Core interfaces (TaskExecutor, TaskQueue, ResourceMonitor) |
| `internal/background/task_queue.go` | PostgreSQL-backed priority queue |
| `internal/background/worker_pool.go` | Adaptive worker pool with CPU/memory scaling |
| `internal/background/resource_monitor.go` | Per-process resource tracking (gopsutil) |
| `internal/background/stuck_detector.go` | Stuck detection algorithms |
| `internal/background/metrics.go` | Prometheus metrics |

**Notification System:**
| Package | Purpose |
|---------|---------|
| `internal/notifications/hub.go` | Central event distribution |
| `internal/notifications/sse_manager.go` | Server-Sent Events streaming |
| `internal/notifications/websocket_server.go` | WebSocket real-time updates |
| `internal/notifications/webhook_dispatcher.go` | Webhook delivery with retry |
| `internal/notifications/polling_store.go` | Polling event buffer |

**CLI Rendering** (`internal/notifications/cli/`):
| File | Purpose |
|------|---------|
| `types.go` | Progress bar, status table, resource gauge types |
| `renderer.go` | CLI rendering with ANSI colors and Unicode |
| `detection.go` | Client detection (18 CLI agents including OpenCode, ClaudeCode, KiloCode, etc.) |

**API Endpoints:**
```
POST   /v1/tasks                 # Create background task
GET    /v1/tasks                 # List tasks
GET    /v1/tasks/:id/status      # Get task status
GET    /v1/tasks/:id/logs        # Get task logs
GET    /v1/tasks/:id/resources   # Get resource snapshots
GET    /v1/tasks/:id/events      # SSE event stream
GET    /v1/tasks/:id/analyze     # Stuck detection analysis
POST   /v1/tasks/:id/pause       # Pause task
POST   /v1/tasks/:id/resume      # Resume task
POST   /v1/tasks/:id/cancel      # Cancel task
GET    /v1/tasks/queue/stats     # Queue statistics
POST   /v1/webhooks              # Register webhook
GET    /v1/ws/tasks/:id          # WebSocket connection
```

**Task States:**
```
pending → queued → running → completed/failed/stuck/cancelled/dead_letter
                          ↓
                       paused → running (resume)
```

**Task Priority:** critical > high > normal > low > background

**Key Features:**
- Adaptive worker scaling based on CPU/memory availability
- Per-process resource monitoring (CPU, memory, I/O, network, FDs)
- Stuck detection (heartbeat timeout, CPU freeze, memory leak, I/O starvation)
- Endless process support with `"endless": true` config
- All notification methods: SSE, WebSocket, Webhooks, Polling
- CLI progress rendering for AI coding agents

**Challenges:**
```bash
./challenges/scripts/background_task_queue_challenge.sh
./challenges/scripts/background_worker_pool_challenge.sh
./challenges/scripts/background_resource_monitor_challenge.sh
./challenges/scripts/background_stuck_detection_challenge.sh
./challenges/scripts/background_notifications_challenge.sh
./challenges/scripts/background_endless_process_challenge.sh
./challenges/scripts/background_cli_rendering_challenge.sh
./challenges/scripts/background_full_integration_challenge.sh
```

**Documentation**: See `docs/background-execution/README.md` for full API reference and configuration.

## CLI Agent Registry (18 Agents)

HelixAgent includes a comprehensive CLI agent registry supporting 18 different AI coding agents. Each agent has unique characteristics, tool support, and protocol compatibility.

### Supported CLI Agents

| Agent | Language | Config | API Pattern | Key Features |
|-------|----------|--------|-------------|--------------|
| **OpenCode** | Go | JSON | OpenAI | MCP support, code completion |
| **Crush** | TypeScript | JSON | OpenAI | Terminal integration, streaming |
| **HelixCode** | Go | JSON | OpenAI | Distributed AI, ensemble, ACP |
| **Kiro** | Python | YAML | OpenAI | 3-phase methodology, steering files |
| **Aider** | Python | TOML | Multi | Git integration, auto-commits, voice |
| **ClaudeCode** | TypeScript | JSON | Anthropic | Codebase understanding, MCP |
| **Cline** | TypeScript | Proto | OpenAI | VS Code, browser, autonomous |
| **CodenameGoose** | Rust | YAML | Multi | Profile system, ripgrep |
| **DeepSeekCLI** | TypeScript | ENV | DeepSeek/Ollama | Local model support |
| **Forge** | Rust | YAML | Multi | Workflow orchestration, multi-agent |
| **GeminiCLI** | TypeScript | JSON | Google | GCP integration, Docker |
| **GPTEngineer** | Python | YAML | OpenAI | Project scaffolding, full generation |
| **KiloCode** | TypeScript | JSON | Multi (50+) | All 21 tools, VS Code, JetBrains |
| **MistralCode** | TypeScript | JSON | Mistral | Code generation, explanation |
| **OllamaCode** | TypeScript | JSON | Ollama | Local models, privacy-focused |
| **Plandex** | Go | JSON | OpenAI | Plan-based development |
| **QwenCode** | TypeScript | JSON | Qwen | Alibaba AI, localization |
| **AmazonQ** | Rust | JSON | AWS | MCP servers, AWS integration |

### Agent API Endpoints

```
GET    /v1/agents                       # List all 18 agents
GET    /v1/agents/:name                 # Get specific agent details
GET    /v1/agents/protocol/:protocol    # Filter by protocol (OpenAI, Anthropic, MCP, etc.)
GET    /v1/agents/tool/:tool            # Filter by tool support (Bash, Read, Git, etc.)
```

### Tool Support by Agent

| Tool Category | Agents Supporting | Tools |
|---------------|-------------------|-------|
| **Core** (all) | 18 agents | Bash, Read, Write |
| **Filesystem** | 17 agents | Edit, Glob, Grep |
| **Version Control** | 14 agents | Git, Diff |
| **Testing** | 6 agents | Test, Lint |
| **Code Intelligence** | 2 agents | Symbols, References, Definition |
| **Workflow** | 2 agents | PR, Issue, Workflow |
| **All 21 Tools** | KiloCode | Full toolset |

### Protocol Support

| Protocol | Agents | Description |
|----------|--------|-------------|
| **OpenAI** | 10 | Standard OpenAI-compatible API |
| **Anthropic** | 5 | Claude API pattern |
| **MCP** | 6 | Model Context Protocol |
| **AWS** | 2 | Amazon Bedrock/Q |
| **Ollama** | 2 | Local model inference |
| **Multi-provider** | 6 | Support multiple backends |

### Key Files

- `internal/agents/registry.go` - Agent registry definitions
- `internal/agents/registry_test.go` - Comprehensive unit tests
- `internal/handlers/agent_handler.go` - API handlers
- `tests/integration/cli_agents_integration_test.go` - Integration tests
- `challenges/scripts/cli_agents_challenge.sh` - Challenge validation

### Usage Example

```go
import "dev.helix.agent/internal/agents"

// Get specific agent
agent, found := agents.GetAgent("ClaudeCode")

// Get all agents supporting MCP
mcpAgents := agents.GetAgentsByProtocol("MCP")

// Get all agents supporting Git tool
gitAgents := agents.GetAgentsByTool("Git")
```

## Technology Stack

- **Framework**: Gin (v1.11.0)
- **Database**: PostgreSQL 15 with pgx/v5 driver
- **Cache**: Redis 7
- **Protocols**: OpenAI-compatible REST, gRPC, MCP, LSP
- **Testing**: testify (v1.11.1)
- **Monitoring**: Prometheus, Grafana

## Configuration

Environment variables defined in `.env.example`. Key categories:
- Server: `PORT`, `GIN_MODE`, `JWT_SECRET`
- Database: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
- Redis: `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`
- LLM providers: `CLAUDE_API_KEY`, `DEEPSEEK_API_KEY`, `GEMINI_API_KEY`, etc.
- Cognee: `COGNEE_AUTH_EMAIL`, `COGNEE_AUTH_PASSWORD` (form-encoded OAuth2 auth)

### Dynamic LLM Provider Selection (LLMsVerifier Integration)

HelixAgent uses **DYNAMIC provider selection** based on real-time LLMsVerifier verification scores. The system automatically selects the best-performing LLM provider based on actual benchmarks.

**How it works:**
1. LLMsVerifier runs verification tests on all available providers
2. Scores are calculated based on: response speed (25%), model efficiency (20%), cost effectiveness (25%), capability (20%), recency (10%)
3. `ProviderDiscovery.calculateProviderScore()` uses these dynamic scores
4. The highest-scoring verified provider is automatically preferred

**Key files:**
- `internal/services/provider_discovery.go` - Provider scoring logic
- `internal/services/llmsverifier_score_adapter.go` - LLMsVerifier integration
- `internal/verifier/scoring.go` - Score calculation

**Current verified scores (from LLMsVerifier):**

| Provider | Model | Score | Notes |
|----------|-------|-------|-------|
| **Dynamic** | Auto-selected | **Highest** | System selects best |
| Gemini | gemini-2.0-flash | ~8.5 | High baseline score |
| DeepSeek | deepseek-coder | ~8.1 | Code-focused |
| Claude | claude-3.5-sonnet | ~9.5 | Premium tier |

Cognee uses the highest-scoring available provider for AI operations.

**Ollama is DEPRECATED** - Lowest priority (score: 5.0). Only used as fallback when no other providers are available. The system dynamically prefers higher-scoring providers.

Configuration files in `/configs`: `development.yaml`, `production.yaml`, `multi-provider.yaml`

## Cognee Authentication (IMPORTANT)

Cognee 0.5.0+ requires authentication. HelixAgent handles this automatically.

### Default Credentials
```
Email:    admin@helixagent.ai
Password: HelixAgentPass123
```

These are configured in `.env`:
```bash
COGNEE_AUTH_EMAIL=admin@helixagent.ai
COGNEE_AUTH_PASSWORD=HelixAgentPass123
```

**IMPORTANT**: Cognee uses form-encoded OAuth2-style login (NOT JSON). The HelixAgent CogneeService handles this automatically.

### Changing Credentials

**Option 1: Update .env file**
```bash
# Edit .env and change:
COGNEE_AUTH_EMAIL=your-email@example.com
COGNEE_AUTH_PASSWORD=YourSecurePassword123

# Restart HelixAgent - new user will be auto-registered
```

**Option 2: Create additional users via API**
```bash
# Register a new user
curl -X POST http://localhost:8000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "newuser@example.com", "password": "SecurePass123"}'

# Login to get access token
curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=newuser@example.com&password=SecurePass123"
```

**Option 3: Change password for existing user**
```bash
# First login to get token
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=helixagent@localhost.com&password=HelixAgent123" | jq -r '.access_token')

# Then use the forgot-password flow or update via API
curl -X POST http://localhost:8000/api/v1/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{"email": "helixagent@localhost.com"}'
```

### Credential Security Notes
- Default credentials are for development only
- For production, change credentials immediately after deployment
- Cognee tokens expire after 1 hour by default
- HelixAgent auto-refreshes tokens as needed

## Container Runtime Support (Docker/Podman)

HelixAgent supports both Docker and Podman as container runtimes. Use the wrapper script for automatic detection:

```bash
# Source the container runtime script
source scripts/container-runtime.sh

# Use the wrapper commands
./scripts/container-runtime.sh build      # Build container image
./scripts/container-runtime.sh start      # Start services
./scripts/container-runtime.sh stop       # Stop services
./scripts/container-runtime.sh logs       # View logs
./scripts/container-runtime.sh status     # Check service status
```

### Docker Usage

```bash
docker-compose up -d                    # Core services (postgres, redis, cognee, chromadb)
docker-compose --profile ai up -d       # Add AI services (ollama)
docker-compose --profile monitoring up -d  # Add monitoring (prometheus, grafana)
docker-compose --profile full up -d     # Everything
```

### Podman Usage

```bash
# Enable Podman socket for Docker compatibility
systemctl --user enable --now podman.socket

# Use podman-compose (install: pip install podman-compose)
podman-compose up -d                    # Core services
podman-compose --profile ai up -d       # Add AI services
podman-compose --profile full up -d     # Everything

# Or use Podman directly
podman build -t helixagent:latest .
podman run -d --name helixagent -p 8080:7061 helixagent:latest
```

### Container Compatibility Tests

```bash
# Run container runtime compatibility tests
./tests/container/container_runtime_test.sh
```

## Adding a New LLM Provider

1. Create provider package: `internal/llm/providers/<name>/<name>.go`
2. Implement `LLMProvider` interface (Complete, CompleteStream, HealthCheck, GetCapabilities, ValidateConfig)
3. Register in `internal/services/provider_registry.go`
4. Add environment variables to `.env.example`
5. Add tests in `internal/llm/providers/<name>/<name>_test.go`

## Cloud Integration

HelixAgent supports integration with major cloud AI providers:

### AWS Bedrock
- Models: Claude, Titan, Llama, Cohere
- Implements AWS Signature V4 authentication
- Configuration via `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`

### GCP Vertex AI
- Models: PaLM, Gemini
- OAuth2 bearer token authentication
- Configuration via `GCP_PROJECT_ID`, `GCP_LOCATION`, `GOOGLE_ACCESS_TOKEN`

### Azure OpenAI
- Models: GPT-4, GPT-3.5
- API key authentication
- Configuration via `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_API_VERSION`

## LLMsVerifier Integration

The project includes LLMsVerifier for validating LLM provider accuracy:

```bash
make verifier-init        # Initialize the LLMsVerifier submodule
make verifier-build       # Build verifier CLI
make verifier-test        # Run verifier tests
make verifier-run         # Run HelixAgent with verifier enabled
make verifier-verify MODEL=gpt-4 PROVIDER=openai  # Verify a model
```

## Test Coverage Summary

The project maintains comprehensive test coverage across 50+ test packages:

| Package | Coverage | Notes |
|---------|----------|-------|
| internal/testing | 91.9% | Test framework utilities |
| internal/plugins | 71.4% | Plugin system |
| internal/services | 67.5% | Business logic |
| internal/handlers | 55.9% | HTTP handlers |
| internal/cloud | 42.8% | Cloud integrations (requires API credentials) |
| internal/cache | 42.4% | Caching (requires Redis) |
| internal/router | 23.8% | Router (requires database) |

### Test Types
- **Unit tests**: `./internal/...` - Core business logic
- **Integration tests**: `./tests/integration/...` - Service interactions, cloud providers, plugins
- **E2E tests**: `./tests/e2e/...` - Full workflow tests
- **Security tests**: `./tests/security/...` - Authentication, authorization, input validation
- **Stress tests**: `./tests/stress/...` - Load and performance testing
- **Chaos tests**: `./tests/challenge/...` - Resilience testing

## Challenges System

The `challenges/` directory contains a comprehensive challenge framework for testing, verifying, and validating LLM providers, AI debate groups, and API quality.

### Key Concepts

**HelixAgent as Virtual LLM Provider**: HelixAgent presents itself as a **single LLM provider** with **ONE virtual model** - the AI Debate Ensemble. The underlying implementation leverages multiple top-performing LLMs through consensus-driven voting.

**Real Data Only - No Stubs**: ALL verification data comes from REAL API calls. No hardcoded scores, no sample data, no cached demonstrations.

**Auto-Start Infrastructure**: ALL infrastructure starts automatically when needed - HelixAgent binary is built if not present, server auto-starts if not running, containers start automatically.

### Running Challenges

```bash
# Run all 44 challenges (auto-starts everything)
./challenges/scripts/run_all_challenges.sh

# Run the main challenge (provider verification + debate group formation + OpenCode config)
./challenges/scripts/main_challenge.sh

# Run specific challenge
./challenges/scripts/run_challenges.sh provider_verification

# Run new everyday use-case challenges
./challenges/scripts/protocol_challenge.sh        # MCP/ACP/LSP/Embeddings/Vision
./challenges/scripts/curl_api_challenge.sh        # Comprehensive curl API testing
./challenges/scripts/cli_agents_challenge.sh      # All 18 CLI agents integration (OpenCode, ClaudeCode, KiloCode, AmazonQ, etc.)
./challenges/scripts/content_generation_challenge.sh  # Content generation & web search
```

### Challenge Categories

| Category | Count | Description |
|----------|-------|-------------|
| Infrastructure | 7 | Health, caching, database, config, plugins, sessions, shutdown |
| Providers | 7 | Claude, DeepSeek, Gemini, Ollama, OpenRouter, Qwen, ZAI |
| Protocols | 3 | MCP, LSP, ACP |
| Security | 3 | Authentication, rate limiting, input validation |
| Core | 8 | Provider verification, ensemble, debate, embeddings, streaming, metadata, quality |
| Cloud | 3 | AWS Bedrock, GCP Vertex, Azure OpenAI |
| Optimization | 2 | Semantic cache, structured output |
| Integration | 1 | Cognee |
| Resilience | 3 | Circuit breaker, error handling, concurrent access |
| API | 2 | OpenAI compatibility, gRPC |
| Everyday Use | 4 | Protocol support, curl API, CLI agents (18 agents), content generation |
| Tool Validation | 1 | Tool call argument validation (required fields, snake_case naming) |

### Tool Call Validation (21 Tools)

HelixAgent validates all tool call arguments to ensure compatibility with AI coding assistants (OpenCode, Claude Code, Qwen Code, Kiro). The system supports **21 tools** across 6 categories:

**Tool Categories:**
| Category | Tools | Count |
|----------|-------|-------|
| Core | Bash, Task, Test, Lint | 4 |
| Filesystem | Read, Write, Edit, Glob, Grep, TreeView, FileInfo | 7 |
| Version Control | Git, Diff | 2 |
| Code Intelligence | Symbols, References, Definition | 3 |
| Workflow | PR, Issue, Workflow | 3 |
| Web | WebFetch, WebSearch | 2 |
| **Total** | | **21** |

**Required Fields per Tool:**
| Tool | Required Fields | Notes |
|------|----------------|-------|
| Bash/shell | `command`, `description` | description is MANDATORY |
| Read | `file_path` | snake_case naming |
| Write | `file_path`, `content` | snake_case naming |
| Edit | `file_path`, `old_string`, `new_string` | snake_case naming |
| Glob | `pattern` | |
| Grep | `pattern` | |
| WebFetch | `url`, `prompt` | |
| WebSearch | `query` | |
| Task | `prompt`, `description`, `subagent_type` | |
| Git | `operation`, `description` | Operations: status, log, diff, commit, branch |
| Diff | `description` | Show file differences |
| Test | `description` | Run project tests |
| Lint | `description` | Run linter checks |
| TreeView | `description` | Show directory tree |
| FileInfo | `file_path`, `description` | Get file metadata |
| Symbols | `description` | List code symbols |
| References | `symbol`, `description` | Find symbol references |
| Definition | `symbol`, `description` | Go to definition |
| PR | `action`, `description` | Actions: list, create, merge, review |
| Issue | `action`, `description` | Actions: list, create, close, comment |
| Workflow | `action`, `description` | Actions: status, trigger, list, logs |

**Parameter Naming Convention:**
- ALL parameters use **snake_case** (e.g., `file_path`, `old_string`, `new_string`)
- NEVER use camelCase (e.g., ~~`filePath`~~, ~~`oldString`~~)
- This ensures compatibility with external tool schemas

**Key Files:**
- `internal/tools/schema.go` - Tool schema registry (21 tools)
- `internal/tools/handler.go` - Tool handlers implementation
- `internal/tools/schema_test.go` - Comprehensive unit tests
- `internal/handlers/openai_compatible.go` - `generateActionToolCalls()` and `extractToolArguments()`
- `tests/integration/tool_call_api_validation_test.go` - Integration tests

**Run Tool Validation Challenges:**
```bash
./challenges/scripts/tool_call_validation_challenge.sh
./challenges/scripts/all_tools_validation_challenge.sh  # NEW: Validates all 21 tools
```

### Main Challenge Output

The Main Challenge generates an OpenCode-compatible configuration:

```bash
./challenges/scripts/main_challenge.sh
# Output: ~/Downloads/opencode-helix-agent.json
```

The generated configuration is validated using LLMsVerifier's OpenCode validator implementation.

## OpenCode Configuration

HelixAgent generates OpenCode configurations following the official schema (`https://opencode.ai/config.json`).

### Valid Top-Level Keys (per LLMsVerifier)

The following keys are valid at the top level (from `LLMsVerifier/scripts/validate_opencode_config.py`):

```
$schema, plugin, enterprise, instructions, provider, mcp, tools, agent,
command, keybinds, username, share, permission, compaction, sse, mode, autoshare
```

### OpenCode Configuration Validation

Configurations are validated using LLMsVerifier's validator which checks:
- Only valid top-level keys are present
- `provider` section is present with `options`
- MCP servers have valid `type` (local/remote) with required fields
- Agents have `model` or `prompt`

### Go-based Generator

A Go-based generator is available at `challenges/codebase/go_files/opencode_generator/`:

```bash
cd challenges/codebase/go_files/opencode_generator
go build -o opencode_generator opencode_generator.go
./opencode_generator --host localhost --port 7061 --output config.json
./opencode_generator --validate config.json  # Validate existing config
```

### Protocol Support and Capabilities

The OpenCode configuration exposes comprehensive protocol support:

| Protocol | Endpoint | Description |
|----------|----------|-------------|
| MCP | `/v1/mcp` | Model Context Protocol for tools and resources |
| ACP | `/v1/acp` | Agent Communication Protocol |
| LSP | `/v1/lsp` | Language Server Protocol for code intelligence |
| Embeddings | `/v1/embeddings` | Text embeddings generation |
| Vision | `/v1/vision` | Image analysis and OCR |
| Cognee | `/v1/cognee` | Knowledge graph and RAG capabilities |

**Model Capabilities** (helixagent-debate):
- `maxTokens`: 128000
- `vision`, `imageInput`, `imageOutput`, `ocr`, `pdf`: true
- `streaming`, `functionCalls`, `toolUse`, `embeddings`: true
- `fileUpload`, `noFileLimit`: true
- `mcp`, `acp`, `lsp`: true

**Fallback Mechanism**: All protocol requests automatically fall back to the next highest-scoring LLM if the primary fails, using LLMsVerifier scores for prioritization.
