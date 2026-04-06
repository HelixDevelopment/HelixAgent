# HelixAgent Comprehensive Feature Reference

This document provides a complete reference of all features, providers, protocols, and capabilities supported by HelixAgent.

## Table of Contents

- [1. LLM Providers](#1-llm-providers)
  - [1.1 Premium Providers (Tier 1)](#11-premium-providers-tier-1)
  - [1.2 High-Quality Specialized (Tier 2)](#12-high-quality-specialized-tier-2)
  - [1.3 Fast Inference (Tier 3)](#13-fast-inference-tier-3)
  - [1.4 Alternative Providers (Tier 4)](#14-alternative-providers-tier-4)
  - [1.5 Cloud & Inference Providers (Tier 5)](#15-cloud--inference-providers-tier-5)
  - [1.6 Specialized Providers (Tier 6)](#16-specialized-providers-tier-6)
  - [1.7 Aggregators, Local & Self-Hosted (Tier 7-8)](#17-aggregators-local--self-hosted-tier-7-8)
- [2. Embedding Providers](#2-embedding-providers)
  - [2.1 Core Providers](#21-core-providers)
  - [2.2 Extended Providers](#22-extended-providers)
- [3. Protocol Implementations](#3-protocol-implementations)
  - [3.1 MCP (Model Context Protocol)](#31-mcp-model-context-protocol)
  - [3.2 LSP (Language Server Protocol)](#32-lsp-language-server-protocol)
  - [3.3 ACP (Agent Communication Protocol)](#33-acp-agent-communication-protocol)
- [4. Vector Databases](#4-vector-databases)
- [5. Power Features](#5-power-features)
  - [5.1 AI Debate System](#51-ai-debate-system)
  - [5.2 RAG System](#52-rag-system)
  - [5.3 Memory Management](#53-memory-management)
  - [5.4 Semantic Routing](#54-semantic-routing)
  - [5.5 Agentic Workflows](#55-agentic-workflows)
  - [5.6 Security Framework](#56-security-framework)
  - [5.7 Structured Output](#57-structured-output)
  - [5.8 LLM Testing Framework](#58-llm-testing-framework)
  - [5.9 Self-Improvement (RLAIF)](#59-self-improvement-rlaif)
  - [5.10 LLMOps](#510-llmops)
  - [5.11 Benchmark Runner](#511-benchmark-runner)
  - [5.12 Optimization Pipeline](#512-optimization-pipeline)
  - [5.13 Observability](#513-observability)
  - [5.14 Background Tasks](#514-background-tasks)
  - [5.15 Notifications](#515-notifications)
  - [5.16 Plugin System](#516-plugin-system)
  - [5.17 Skills Registry](#517-skills-registry)
  - [5.18 Tools Registry](#518-tools-registry)
  - [5.19 Agents Registry](#519-agents-registry)
  - [5.20 HelixLLM Integration](#520-helixllm-integration)
  - [5.21 AgenticEnsemble](#521-agenticensemble)
  - [5.22 HelixMemory](#522-helixmemory)
  - [5.23 HelixSpecifier](#523-helixspecifier)
  - [5.24 HTTP/3 (QUIC) with Brotli Compression](#524-http3-quic-with-brotli-compression)
- [6. Summary Statistics](#6-summary-statistics)

---

## 1. LLM Providers

HelixAgent supports **48 LLM providers** with automatic discovery and dynamic selection based on LLMsVerifier scores.

### 1.1 Premium Providers (Tier 1)

| Provider | Default Model | Capabilities | Authentication | Priority |
|----------|---------------|--------------|----------------|----------|
| **Claude (Anthropic)** | `claude-sonnet-4-5-20250929` | Streaming, Tools, System prompts | API Key / OAuth2 | 1 |
| **OpenAI** | `gpt-4o` | Streaming, Tools, Response formatting | API Key | 1 |
| **Google Gemini** | `gemini-2.0-flash` | Streaming, Tools, Safety settings, Images | API Key | 2 |

### 1.2 High-Quality Specialized (Tier 2)

| Provider | Default Model | Capabilities | Authentication | Priority |
|----------|---------------|--------------|----------------|----------|
| **DeepSeek** | `deepseek-chat` | Streaming, Tools | API Key | 3 |
| **Mistral** | `mistral-large-latest` | Streaming, Tools, Safe prompt | API Key | 3 |
| **xAI (Grok)** | `grok-3-beta` | Streaming, Tools, Regional support (US/EU) | API Key | 3 |
| **Qwen (Alibaba)** | `qwen-max` | Streaming, Tools | API Key / OAuth2 | 4 |
| **Cohere** | `command-r-plus` | Streaming, Tools, Citations, RAG | API Key | 4 |
| **Perplexity** | `llama-3.1-sonar-large-128k-online` | Streaming, Online search | API Key | 4 |
| **AI21 Labs** | `jamba-1.5-large` | Streaming, Tools | API Key | 5 |

### 1.3 Fast Inference (Tier 3)

| Provider | Default Model | Capabilities | Authentication | Priority |
|----------|---------------|--------------|----------------|----------|
| **Groq** | `llama-3.3-70b-versatile` | Streaming, Tools, Audio transcription | API Key | 5 |
| **Cerebras** | `llama-3.3-70b` | Streaming, Fast inference | API Key | 5 |

### 1.4 Alternative Providers (Tier 4)

| Provider | Default Model | Capabilities | Authentication | Priority |
|----------|---------------|--------------|----------------|----------|
| **Fireworks AI** | `llama-v3p1-70b-instruct` | Streaming, Tools | API Key | 6 |
| **Together AI** | `Llama-3.3-70B-Instruct-Turbo` | Streaming, Tools | API Key | 6 |
| **Replicate** | `meta/llama-2-70b-chat` | Async prediction, Webhooks | API Key | 7 |
| **Hugging Face** | `Meta-Llama-3-8B-Instruct` | Standard/Pro modes, Cache control | API Key | 8 |

### 1.5 Cloud & Inference Providers (Tier 5)

| Provider | Default Model | Capabilities | Authentication | Priority |
|----------|---------------|--------------|----------------|----------|
| **Cloudflare** | `@cf/meta/llama-3-8b-instruct` | Workers AI, Streaming | API Key | 6 |
| **Codestral** | `codestral-latest` | Code-specialized, Streaming | API Key | 6 |
| **NVIDIA** | `meta/llama3-70b-instruct` | NIM, Streaming, Tools | API Key | 6 |
| **Hyperbolic** | `meta-llama/Llama-3-70b-chat-hf` | Streaming | API Key | 7 |
| **SiliconFlow** | `deepseek-ai/DeepSeek-V2-Chat` | Streaming | API Key | 7 |
| **Novita** | `meta-llama/llama-3-70b-instruct` | Streaming | API Key | 7 |
| **SambaNova** | `Meta-Llama-3-70B-Instruct` | Streaming, Fast inference | API Key | 7 |
| **Upstage** | `solar-pro` | Streaming | API Key | 7 |
| **Sarvam** | `sarvam-2b-v0.5` | Streaming, Indian languages | API Key | 8 |
| **PublicAI** | `llama-3-70b` | Streaming | API Key | 8 |

### 1.6 Specialized Providers (Tier 6)

| Provider | Default Model | Capabilities | Authentication | Priority |
|----------|---------------|--------------|----------------|----------|
| **Kimi (Moonshot)** | `moonshot-v1-8k` | Streaming, Long context | API Key | 7 |
| **KimiCode** | `moonshot-v1-code` | Code-specialized, Streaming | API Key | 7 |
| **Kilo** | `kilo-v1` | Streaming | API Key | 7 |
| **Modal** | `meta-llama/Llama-3-70b-chat-hf` | Serverless, Streaming | API Key | 7 |
| **Nia** | `nia-v1` | Streaming | API Key | 8 |
| **NLPCloud** | `chatdolphin` | Streaming, Tools | API Key | 8 |
| **VulaVula** | `vulavula-chat` | Streaming, African languages | API Key | 8 |
| **Zhipu (GLM)** | `glm-4` | Streaming, Tools | API Key | 7 |
| **Venice** | `llama-3.1-405b` | Privacy-focused, Streaming | API Key | 7 |
| **Junie** | `junie-v1` | Streaming, CLI/ACP mode | API Key / OAuth2 | 6 |

### 1.7 Aggregators, Local & Self-Hosted (Tier 7-8)

| Provider | Default Model | Capabilities | Authentication | Priority |
|----------|---------------|--------------|----------------|----------|
| **OpenRouter** | `anthropic/claude-3.5-sonnet` | 150+ models, Streaming, Tools | API Key | 10 |
| **Zen (OpenCode)** | `grok-code` (free) | Anonymous access, Streaming, Tools | Optional API Key | 4 |
| **Ollama** | `llama3.2` | Local execution, Streaming | None (local) | 20 |
| **LM Studio** | `local-model` | Local execution, OpenAI-compatible | None (local) | 20 |
| **Azure OpenAI** | `gpt-4o` | Enterprise, Streaming, Tools | API Key | 5 |
| **Vertex AI** | `gemini-pro` | Google Cloud, Streaming | Service Account | 5 |
| **HelixLLM** | `helixllm-default` | Self-hosted, RAG, OpenAI-compatible | API Key | 3 |
| **Anthropic CU** | `claude-sonnet-4-5-20250929` | Computer Use integration | API Key | 5 |
| **GitHub Models** | `gpt-4o` | GitHub-hosted, Streaming, Tools | GitHub Token | 6 |

**Note**: Ollama and LM Studio are local-only providers (score: 5.0) - used as fallback.

### Provider Authentication Methods

| Method | Providers | Storage |
|--------|-----------|---------|
| **API Key** | Most providers (AI21, Anthropic, Cerebras, Chutes, Cloudflare, Codestral, Cohere, DeepSeek, Fireworks, Groq, HuggingFace, Hyperbolic, Kilo, Kimi, KimiCode, Mistral, Modal, Nia, NLPCloud, Novita, NVIDIA, OpenAI, OpenRouter, Perplexity, PublicAI, Replicate, SambaNova, Sarvam, SiliconFlow, Together, Upstage, Venice, VulaVula, xAI, ZAI, Zhipu) | Environment variables |
| **OAuth2** | Claude, Qwen, Junie | CLI credential files |
| **Service Account** | Vertex AI, AWS Bedrock | Service account JSON / SigV4 |
| **GitHub Token** | GitHub Models | GitHub PAT |
| **Azure AD** | Azure OpenAI | API Key or Azure AD |
| **Anonymous** | Zen (free models) | Device-ID header |
| **None** | Ollama, LM Studio | Local only |
| **Self-hosted** | HelixLLM | API Key (optional) |

---

## 2. Embedding Providers

HelixAgent supports **13 embedding providers** with 40+ models.

### 2.1 Core Providers

| Provider | Models | Dimensions | Authentication |
|----------|--------|------------|----------------|
| **OpenAI** | `text-embedding-3-small`, `text-embedding-3-large`, `text-embedding-ada-002` | 1536, 3072 | API Key |
| **Ollama** | `nomic-embed-text`, `mxbai-embed-large`, `all-minilm` | 384, 768, 1024 | None (local) |
| **BGE-M3** | `BAAI/bge-m3` | 1024 | HuggingFace API Key |
| **Nomic** | `nomic-ai/nomic-embed-text-v1.5` | 768 | HuggingFace API Key |
| **CodeBERT** | `microsoft/codebert-base` | 768 | HuggingFace API Key |
| **Qwen3** | `Qwen/Qwen3-Embedding-0.6B` | 768 | HuggingFace API Key |
| **GTE** | `thenlper/gte-large` | 1024 | HuggingFace API Key |
| **E5** | `intfloat/e5-large-v2` | 1024 | HuggingFace API Key |

### 2.2 Extended Providers

| Provider | Models | Dimensions | Authentication |
|----------|--------|------------|----------------|
| **Cohere** | `embed-english-v3.0`, `embed-multilingual-v3.0`, 4 more | 384-4096 | API Key |
| **Voyage AI** | `voyage-3`, `voyage-code-3`, `voyage-finance-2`, 5 more | 512-1536 | API Key |
| **Jina AI** | `jina-embeddings-v3`, `jina-clip-v1`, `jina-colbert-v2`, 6 more | 128-1024 | API Key |
| **Google Vertex AI** | `text-embedding-005`, `text-multilingual-embedding-002`, 3 more | 768 | Service Account |
| **AWS Bedrock** | `amazon.titan-embed-text-v1/v2`, `cohere.embed-english-v3` | 1024-1536 | AWS SigV4 |

---

## 3. Protocol Implementations

### 3.1 MCP (Model Context Protocol)

**Total: 79+ implementations (19 adapters + 60+ containerized servers)**

#### MCP Adapters (External Service Integrations)

| Category | Adapters | Key Tools |
|----------|----------|-----------|
| **Cloud Storage** | AWS S3, Google Drive | `s3_list_buckets`, `s3_get/put_object`, `gdrive_list/get/create_file` |
| **Project Management** | Jira, Linear, Asana | `jira_get/create_issue`, `linear_create_issue`, `asana_create_task` |
| **Version Control** | GitLab | `gitlab_list_projects`, `gitlab_create_merge_request` |
| **Communication** | Slack | `slack_post_message`, `slack_list_channels` |
| **Design Tools** | Figma, Miro, SVGMaker | `figma_get_file`, `miro_list_boards`, `svgmaker_create_svg` |
| **Infrastructure** | Docker, Kubernetes | `docker_list_containers`, `k8s_list_pods`, `k8s_list_deployments` |
| **Search** | Brave Search | `brave_web_search`, `brave_image_search`, `brave_news_search` |
| **Analytics** | Datadog, Sentry | `datadog_get_metrics`, `sentry_list_issues` |
| **Database** | MongoDB | `mongodb_query_collection`, `mongodb_insert_document` |
| **Knowledge** | Notion | `notion_query_database`, `notion_create_page` |
| **Automation** | Puppeteer | `puppeteer_take_screenshot`, `puppeteer_extract_text` |
| **AI Generation** | Stable Diffusion | `stable_diffusion_generate_image` |

#### MCP Servers (Backend Integrations)

| Category | Servers |
|----------|---------|
| **Vector Stores** | Chroma, Qdrant, Weaviate |
| **Databases** | PostgreSQL, SQLite, Redis |
| **Development** | Git, GitHub, Filesystem |
| **Content** | Fetch, Memory, Replicate |
| **Design** | Figma, Miro, SVGMaker, StableDiffusion |

### 3.2 LSP (Language Server Protocol)

**10 supported language servers**

| Language | Server | Priority | Full LSP Support |
|----------|--------|----------|------------------|
| **Go** | `gopls` | 100 | Yes |
| **Rust** | `rust-analyzer` | 100 | Yes |
| **TypeScript/JS** | `ts-server` | 95 | Yes |
| **Python** | `pylsp` | 90 | Yes |
| **Python** | `pyright` | 85 | Partial |
| **C/C++** | `clangd` | 90 | Yes |
| **Java** | `jdtls` | 80 | Core |
| **PHP** | `intelephense` | 80 | Yes |
| **Ruby** | `solargraph` | 75 | Partial |
| **Lua** | `lua-language-server` | 70 | Partial |

**LSP Capabilities**: Completion, Hover, Definition, References, Diagnostics, Rename, Code Actions, Formatting, Signature Help

### 3.3 ACP (Agent Communication Protocol)

| Component | Purpose |
|-----------|---------|
| **ACPManager** | Server discovery, capability enumeration, action execution |
| **ACPClient** | HTTP/WebSocket transport, JSON-RPC 2.0, retry logic |

**Features**: Multi-transport, exponential backoff, server synchronization, diagnostics

---

## 4. Vector Databases

| Database | Location | Capabilities |
|----------|----------|--------------|
| **Qdrant** | `internal/vectordb/qdrant/` | Dense/sparse vectors, filtering, namespaces |
| **Milvus** | `internal/vectordb/milvus/` | Scalable vector storage, batch operations |
| **Pinecone** | `internal/vectordb/pinecone/` | Cloud-native, metadata filtering |
| **PgVector** | `internal/vectordb/pgvector/` | PostgreSQL native vectors |

---

## 5. Power Features

### 5.1 AI Debate System

**Location**: `internal/debate/`, `internal/services/debate_*`

| Aspect | Description |
|--------|-------------|
| **Purpose** | Multi-round debate between LLM providers with consensus voting |
| **Participants** | 25 LLMs (5 positions x 5 per position) |
| **Topologies** | Mesh, Star, Chain, Tree |
| **Phases** | 8-phase protocol: Dehallucination → SelfEvolvement → Proposal → Critique → Review → Optimization → Adversarial → Convergence |
| **Voting** | 6 methods: Weighted (MiniMax), Majority, Borda Count, Condorcet, Plurality, Unanimous |
| **Reflexion** | Episodic memory, verbal reflection, retry-and-learn loop, cross-session wisdom |
| **Adversarial** | Red/Blue team multi-round attack-defend cycles |
| **Approval Gates** | Configurable human-in-the-loop with REST API (approve/reject/gates endpoints) |
| **Performance** | Parallel execution, response caching with TTL, early termination on consensus |
| **Persistence** | PostgreSQL tables (debate_sessions, debate_turns, code_versions) |
| **Provenance** | Full reproducibility tracking with 14 event types, JSON export |
| **Learning** | Cross-debate lesson extraction and application |
| **Activation** | `POST /v1/debates` |

### 5.2 RAG System

**Location**: `internal/rag/`

| Component | Purpose |
|-----------|---------|
| **Pipeline** | Orchestrates retrieval workflow |
| **Hybrid Search** | Dense + sparse retrieval fusion |
| **HyDE** | Hypothetical document embeddings for query expansion |
| **Reranker** | Multi-stage relevance scoring |
| **Qdrant Integration** | Vector storage and retrieval |

### 5.3 Memory Management

**Location**: `internal/memory/`

| Feature | Description |
|---------|-------------|
| **Memory Types** | Episodic, semantic, procedural, working |
| **Entity Graph** | Entity and relationship storage |
| **Decay** | Automatic memory decay over time |
| **Session Scope** | Cross-session recall |

### 5.4 Semantic Routing

**Location**: `internal/routing/semantic/`

| Feature | Description |
|---------|-------------|
| **Purpose** | Embedding-based request routing |
| **Coverage** | 96.2% test coverage |
| **Matching** | Threshold-based with top-K retrieval |

### 5.5 Agentic Workflows

**Location**: `internal/agentic/`

| Feature | Description |
|---------|-------------|
| **Style** | LangGraph-style DAG execution |
| **Node Types** | Agent, Tool, Condition, Parallel, Human-in-loop, Subgraph |
| **Checkpointing** | Fault-tolerant state saving |
| **Coverage** | 96.5% test coverage |

### 5.6 Security Framework

**Location**: `internal/security/`

| Component | Purpose |
|-----------|---------|
| **Red Team** | 40+ attack patterns |
| **Guardrails** | Output safety constraints |
| **PII Detection** | Sensitive data identification |
| **Secure Fix Agent** | AI-powered vulnerability remediation |
| **Audit Logging** | Security event tracking |
| **OWASP Coverage** | Top 10 vulnerabilities |

### 5.7 Structured Output

**Location**: `internal/structured/`

| Schema Type | Description |
|-------------|-------------|
| **JSON Schema** | Strict JSON output enforcement |
| **Regex** | Pattern-based constraints |
| **Grammar** | Context-free grammar validation |
| **Enum** | Enumeration constraints |

### 5.8 LLM Testing Framework

**Location**: `internal/testing/llm/`

| Feature | Description |
|---------|-------------|
| **Style** | DeepEval-style evaluation |
| **Metrics** | Relevance, faithfulness, hallucination |
| **RAGAS** | RAG evaluation metrics |
| **Coverage** | 96.2% test coverage |

### 5.9 Self-Improvement (RLAIF)

**Location**: `internal/selfimprove/`

| Component | Purpose |
|-----------|---------|
| **AI Reward Model** | LLM-as-judge scoring |
| **Feedback Collector** | Human/AI/debate feedback |
| **Policy Optimizer** | Policy update generation |
| **Constitutional AI** | Principle enforcement |

### 5.10 LLMOps

**Location**: `internal/llmops/`

| Feature | Description |
|---------|-------------|
| **Prompt Registry** | Semantic versioning for prompts |
| **A/B Testing** | Statistical experiment framework |
| **Continuous Eval** | Automated quality monitoring |
| **Alerting** | Regression detection |

### 5.11 Benchmark Runner

**Location**: `internal/benchmark/`

| Benchmark | Type |
|-----------|------|
| **SWE-Bench** | Software engineering |
| **HumanEval** | Code generation |
| **MBPP** | Python programming |
| **MMLU** | Multi-task knowledge |
| **GSM8K** | Math reasoning |
| **MATH** | Advanced math |
| **HellaSwag** | Commonsense reasoning |

### 5.12 Optimization Pipeline

**Location**: `internal/optimization/`

| Component | Purpose |
|-----------|---------|
| **GPTCache** | Semantic caching (90%+ latency reduction) |
| **Outlines** | Structured output constraints |
| **Streaming** | Multi-level buffering and rate limiting |
| **LangChain** | Chain optimization |
| **LlamaIndex** | RAG optimization |
| **SGLang** | Structured generation |
| **Guidance** | Template optimization |
| **LMQL** | Query language optimization |

### 5.13 Observability

**Location**: `internal/observability/`

| Exporter | Type |
|----------|------|
| **Jaeger** | Distributed tracing |
| **Zipkin** | Trace collection |
| **Langfuse** | LLM-specific observability |
| **Prometheus** | Metrics export |

### 5.14 Background Tasks

**Location**: `internal/background/`

| Feature | Description |
|---------|-------------|
| **Task Queue** | PostgreSQL/In-memory queueing |
| **Worker Pool** | Auto-scaling (2-10 workers) |
| **Resource Monitor** | CPU, memory, disk tracking |
| **Stuck Detector** | Timeout and progress detection |
| **States** | pending, queued, running, completed, failed, stuck, cancelled |

### 5.15 Notifications

**Location**: `internal/notifications/`

| Channel | Description |
|---------|-------------|
| **SSE** | Server-Sent Events |
| **WebSocket** | Bidirectional real-time |
| **Webhooks** | HTTP callbacks with HMAC |
| **Polling** | Event storage for polling clients |

### 5.16 Plugin System

**Location**: `internal/plugins/`

| Feature | Description |
|---------|-------------|
| **Discovery** | Automatic plugin scanning |
| **Hot Reload** | File change detection |
| **Dependencies** | Dependency resolution |
| **Security** | Sandboxing and permissions |
| **Versioning** | Semantic versioning |
| **Health** | Plugin health monitoring |

### 5.17 Skills Registry

**Location**: `internal/skills/`

| Feature | Description |
|---------|-------------|
| **Categories** | code, debug, search, git, deploy, docs, test, review |
| **Matching** | Trigger phrase, fuzzy, semantic |
| **Tracking** | Usage analytics |
| **Hot Reload** | YAML configuration reload |

### 5.18 Tools Registry

**Location**: `internal/tools/`

**21 Tools**: Bash, Read, Write, Edit, Glob, Grep, WebFetch, WebSearch, Git, Task, TodoWrite, AskUserQuestion, EnterPlanMode, ExitPlanMode, Skill, NotebookEdit, KillShell, TaskOutput, and more.

### 5.19 Agents Registry

**Location**: `internal/agents/`

**48 Agents**: OpenCode, Crush, HelixCode, Kiro, Aider, ClaudeCode, Cline, CodenameGoose, DeepSeekCLI, Forge, GeminiCLI, GPTEngineer, KiloCode, MistralCode, OllamaCode, Plandex, QwenCode, AmazonQ, AgentDeck, Bridle, CheshireCat, ClaudePlugins, ClaudeSquad, Codai, Codex, CodexSkills, Conduit, Emdash, FauxPilot, GetShitDone, GitHubCopilotCLI, GitHubSpecKit, GitMCP, GPTME, MobileAgent, MultiagentCoding, Nanocoder, Noi, Octogen, OpenHands, PostgresMCP, Shai, SnowCLI, TaskWeaver, UIUXProMax, VTCode, Warp, Continue

### 5.20 HelixLLM Integration

**Location**: `internal/llm/providers/helixllm/`, `HelixLLM/` (submodule)

| Feature | Description |
|---------|-------------|
| **Self-hosted LLM** | OpenAI-compatible API with RAG capabilities |
| **First-class provider** | Registered in provider registry, participates in ensemble and debate |
| **TLS** | HTTPS endpoint with configurable TLS verification |
| **Endpoints** | Chat completions, embeddings, models, health check |

### 5.21 AgenticEnsemble

**Location**: `internal/services/agentic_ensemble.go`

| Feature | Description |
|---------|-------------|
| **Mode Classification** | Automatic routing: single-provider, ensemble, tool-augmented debate, agentic loop |
| **Tool-Augmented Debate** | Combines debate orchestration with tool calling for grounded reasoning |
| **Agentic Execution Loop** | Plan-execute-verify cycle: task decomposition → layered execution → result verification → synthesis |
| **Task Planning** | Decomposes complex queries into parallel/sequential task layers |
| **Result Verification** | LLM-based verification of task execution results |

### 5.22 HelixMemory

**Location**: `HelixMemory/` (submodule), `internal/adapters/memory/factory_helixmemory.go`

| Feature | Description |
|---------|-------------|
| **Unified Memory** | Fuses Mem0, Cognee, Letta, and Graphiti into single engine |
| **3-Stage Fusion** | Collect → Dedup → Rerank pipeline with weighted scoring |
| **12 Power Features** | Codebase DNA, procedural memory, mesh, temporal, debate, context window, cross-project, MCP bridge, code gen, confidence, quality loop, snapshots |
| **Circuit Breakers** | Fault tolerance for each memory backend |
| **Active by default** | Opt out with `-tags nohelixmemory` |

### 5.23 HelixSpecifier

**Location**: `HelixSpecifier/` (submodule), `internal/adapters/specifier/adapter.go`

| Feature | Description |
|---------|-------------|
| **Spec-Driven Development** | 3-pillar architecture: SpecKit + Superpowers + GSD |
| **7-Phase SDD** | Constitution → Specify → Clarify → Plan → Tasks → Analyze → Implement |
| **Adaptive Ceremony** | Scales ceremony based on work granularity (5 levels) |
| **Intent Classification** | Signal-based request analysis for effort classification |
| **Active by default** | Opt out with `-tags nohelixspecifier` |

### 5.24 HTTP/3 (QUIC) with Brotli Compression

| Feature | Description |
|---------|-------------|
| **Primary Transport** | HTTP/3 (QUIC) via `quic-go/quic-go` |
| **Fallback** | HTTP/2 when HTTP/3 is unavailable |
| **Compression** | Brotli (primary, via `andybalholm/brotli`) → gzip (fallback) |
| **Scope** | All HTTP clients and servers prefer HTTP/3 |

---

## 6. Summary Statistics

| Category | Count |
|----------|-------|
| **LLM Providers** | 48 |
| **Embedding Providers** | 13 (6 core + 7 extended) |
| **MCP Implementations** | 79+ (19 adapters + 60+ containerized servers) |
| **LSP Language Servers** | 10 |
| **ACP Components** | 2 |
| **Vector Databases** | 4 |
| **Tools** | 21 |
| **CLI Agents** | 48 |
| **Extracted Modules** | 41 |
| **Power Features** | 24+ major systems |
| **Security Attack Patterns** | 40+ |
| **Debate Participants** | 25 LLMs |
| **Debate Voting Methods** | 6 |
| **Benchmarks Supported** | 7 |
| **Code Formatters** | 32+ |

---

## Quick Reference: API Endpoints

| Endpoint | Protocol | Description |
|----------|----------|-------------|
| `/v1/chat/completions` | OpenAI | Chat completions (ensemble) |
| `/v1/completions` | OpenAI | Text completions |
| `/v1/embeddings` | OpenAI | Vector embeddings |
| `/v1/debates` | HelixAgent | AI debate system |
| `/v1/mcp` | MCP | Model Context Protocol |
| `/v1/lsp` | LSP | Language Server Protocol |
| `/v1/lsp/ws` | LSP | LSP WebSocket |
| `/v1/acp` | ACP | Agent Communication Protocol |
| `/v1/rag/*` | HelixAgent | RAG operations |
| `/v1/cognee` | HelixAgent | Knowledge graph (optional) |
| `/v1/vision` | HelixAgent | Image analysis |
| `/v1/tasks` | HelixAgent | Background tasks |
| `/v1/monitoring/*` | HelixAgent | Monitoring endpoints |
| `/v1/startup/verification` | HelixAgent | Startup verification status |
| `/v1/bigdata/health` | HelixAgent | BigData health |
| `/v1/discovery` | HelixAgent | Dynamic model discovery |
| `/v1/scoring` | HelixAgent | Provider scoring |
| `/v1/verification` | HelixAgent | Provider verification |
| `/v1/health` | HelixAgent | Health check |
| `/v1/agentic/workflows` | HelixAgent | Agentic workflow orchestration |
| `/v1/planning/{hiplan,mcts,tot}` | HelixAgent | AI planning algorithms |
| `/v1/llmops/{experiments,evaluate,prompts}` | HelixAgent | LLM operations |
| `/v1/benchmark/{run,results}` | HelixAgent | Benchmarking |
| `/v1/qa/{sessions,findings,platforms,discover}` | HelixAgent | QA orchestration |
| `/v1/ensemble/{sessions,teams}` | HelixAgent | Ensemble management |
| `/v1/completion/*` | HelixAgent | Completion endpoints |
| `/v1/format` | HelixAgent | Code formatting |
| `/v1/formatters` | HelixAgent | Formatter registry |
| `/v1/graphql` | HelixAgent | GraphQL (feature-flagged, `GRAPHQL_ENABLED=true`) |

---

*Last updated: 2026-04-06*
