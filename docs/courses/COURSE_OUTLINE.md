# HelixAgent Video Course: Complete Training Program

## Course Overview

**Title**: "Mastering HelixAgent: Multi-Provider AI Orchestration"
**Duration**: 20+ hours across 20 comprehensive modules
**Target Audience**: Developers, DevOps engineers, AI engineers, and technical decision-makers
**Prerequisites**: Basic programming knowledge, familiarity with REST APIs, Go 1.25+ for development modules
**Skill Level**: Beginner to Advanced

---

## Course Objectives

Upon completion of this course, participants will be able to:

1. Understand the architecture and design principles of HelixAgent
2. Install, configure, and deploy HelixAgent in various environments
3. Integrate 43 LLM providers (Claude, DeepSeek, Gemini, Mistral, OpenRouter, Qwen, ZAI, Zen, Cerebras, Ollama, AI21, Anthropic, Cohere, Fireworks, GitHub Models, Groq, HuggingFace, OpenAI, Perplexity, Replicate, Together, Venice, xAI, Junie, Cloudflare, Codestral, Hyperbolic, Kilo, Kimi, KimiCode, Modal, Nia, NLPCloud, Novita, Nvidia, PublicAI, SambaNova, Sarvam, SiliconFlow, Upstage, VulaVula, Zhipu, HelixLLM)
4. Implement ensemble voting strategies for improved AI responses
5. Configure and utilize the AI Debate System with 25 LLMs for complex problem-solving
6. Develop custom plugins for extended functionality
7. Integrate MCP/LSP/ACP protocols for enhanced capabilities
8. Apply LLM optimization techniques for better performance
9. Implement security best practices for production deployments
10. Set up comprehensive testing and quality validation
11. Run and interpret RAGS, MCPS, and SKILLS challenges
12. Implement MCP Tool Search and discovery
13. Configure multi-pass validation for AI debates
14. Integrate with 48 CLI agents
15. Configure HelixLLM as a provider and AgenticEnsemble dual-mode operation
16. Deploy containers to remote hosts via automatic container distribution
17. Configure HTTP/3 (QUIC) transport with Brotli compression
18. Leverage HelixMemory unified cognitive memory engine
19. Apply HelixSpecifier spec-driven development workflows

---

## Module 1: Introduction to HelixAgent (45 minutes)

### Learning Objectives
- Understand what HelixAgent is and its core value proposition
- Learn the benefits of multi-provider AI orchestration
- Familiarize with the system architecture and components

### Videos

#### 1.1 Course Welcome and Learning Path (10 min)
- Welcome and instructor introduction
- Course structure and navigation
- Prerequisites review
- Learning outcomes overview
- How to get the most from this course

#### 1.2 What is HelixAgent? (15 min)
- Multi-provider AI orchestration explained
- The problem of vendor lock-in
- Benefits of intelligent routing
- Real-world use cases and applications
- Comparison with single-provider approaches

#### 1.3 Architecture Overview (20 min)
- System architecture diagram walkthrough
- Core components and their responsibilities:
  - LLM Provider Abstraction Layer
  - Ensemble Orchestration Engine
  - AI Debate Service
  - Plugin System
  - Protocol Managers (MCP, LSP, ACP)
  - Caching Layer
  - Monitoring and Metrics
- Data flow and processing pipeline
- Scalability and performance design

### Hands-On Lab
- Explore the HelixAgent repository structure
- Review key configuration files
- Examine API documentation

---

## Module 2: Installation and Setup (60 minutes)

### Learning Objectives
- Set up development environment
- Install HelixAgent using different methods
- Verify successful installation

### Videos

#### 2.1 Environment Prerequisites (15 min)
- System requirements overview
- Go 1.25+ installation and verification
- Docker and Docker Compose setup
- Git configuration
- IDE recommendations (VS Code, GoLand)

#### 2.2 Quick Start with Docker (20 min)
- Docker-based installation walkthrough
- Docker Compose configuration
- Starting core services (PostgreSQL, Redis, Cognee)
- Starting with different profiles:
  - Core services
  - AI services (Ollama)
  - Monitoring (Prometheus, Grafana)
  - Full stack deployment
- Container health verification

#### 2.3 Manual Installation from Source (20 min)
- Repository cloning
- Dependency installation
- Building from source:
  ```bash
  make build
  make build-debug
  ```
- Running locally:
  ```bash
  make run
  make run-dev
  ```
- Troubleshooting common installation issues

#### 2.4 Podman Alternative Setup (5 min)
- Podman compatibility overview
- Podman-specific configuration
- Using the container-runtime script

### Hands-On Lab
- Complete installation using preferred method
- Start all services
- Verify API endpoints are accessible
- Run initial health checks

---

## Module 3: Configuration (60 minutes)

### Learning Objectives
- Master HelixAgent configuration options
- Configure environment variables and YAML files
- Set up multi-environment configurations

### Videos

#### 3.1 Configuration Architecture (15 min)
- Configuration file structure
- Environment variable hierarchy
- Configuration loading precedence
- Secrets management best practices

#### 3.2 Core Configuration Options (20 min)
- Server configuration (PORT, GIN_MODE, JWT_SECRET)
- Database settings (PostgreSQL configuration)
- Redis cache configuration
- Logging and debugging options
- Configuration files walkthrough:
  - `configs/development.yaml`
  - `configs/production.yaml`
  - `configs/multi-provider.yaml`

#### 3.3 Provider Configuration (15 min)
- API key configuration for each provider:
  - CLAUDE_API_KEY
  - DEEPSEEK_API_KEY
  - GEMINI_API_KEY
  - QWEN_API_KEY
  - ZAI_API_KEY
  - OPENROUTER_API_KEY
- Ollama local model configuration
- Provider-specific settings and models
- Rate limiting and timeout configuration

#### 3.4 Advanced Configuration (10 min)
- AI Debate configuration
- Cognee integration settings
- Optimization feature configuration
- Cloud provider integration (AWS Bedrock, GCP Vertex AI, Azure OpenAI)

### Hands-On Lab
- Create custom configuration for your use case
- Configure multiple providers
- Set up environment-specific configurations
- Test configuration validation

---

## Module 4: LLM Provider Integration (75 minutes)

### Learning Objectives
- Integrate all supported LLM providers
- Understand provider capabilities and limitations
- Implement fallback chains for reliability

### Videos

#### 4.1 Provider Interface and Architecture (15 min)
- `LLMProvider` interface explained
- Provider lifecycle management
- Health checking and monitoring
- Error handling patterns

#### 4.2 Claude Integration (10 min)
- Claude API configuration
- Model selection (Claude 3 Opus, Sonnet, Haiku)
- Claude-specific features
- Best use cases for Claude

#### 4.3 DeepSeek Integration (10 min)
- DeepSeek API setup
- DeepSeek-Coder and DeepSeek-Chat models
- Technical content optimization
- Cost-effective processing strategies

#### 4.4 Gemini Integration (10 min)
- Google Gemini API configuration
- Gemini Pro and multimodal capabilities
- Scientific and analytical use cases
- GCP Vertex AI integration

#### 4.5 Qwen, Ollama, and Other Providers (15 min)
- Qwen API configuration
- Ollama local model setup and advantages
- OpenRouter as a meta-provider
- ZAI integration
- HelixLLM as a local AI ensemble provider
- Provider selection guidelines

#### 4.7 43 Provider Ecosystem Overview (15 min)
- Complete provider catalog: AI21, Anthropic, Cerebras, Chutes, Claude, Cloudflare, Codestral, Cohere, DeepSeek, Fireworks, Gemini, GitHub Models, Groq, HelixLLM, HuggingFace, Hyperbolic, Junie, Kilo, Kimi, KimiCode, LMStudio, Mistral, Modal, Nia, NLPCloud, Novita, Nvidia, Ollama, OpenAI, OpenRouter, Perplexity, PublicAI, Qwen, Replicate, SambaNova, Sarvam, SiliconFlow, Together, Upstage, Venice, VulaVula, xAI, ZAI, Zen, Zhipu
- Generic OpenAI-compatible provider for unlisted providers
- Dynamic provider selection via LLMsVerifier verification scores
- 3-tier dynamic model discovery (Provider API, models.dev, hardcoded fallback)
- OAuth/CLI proxy providers (Claude, Qwen, Zen, Gemini, Junie)

#### 4.6 Building Fallback Chains (15 min)
- Designing reliable fallback strategies
- Primary and secondary provider configuration
- Health-based routing
- Cost-optimized fallback ordering
- Testing fallback scenarios

### Hands-On Lab
- Configure at least 3 different providers
- Implement a fallback chain
- Test provider health checks
- Compare response quality across providers

---

## Module 5: Ensemble Strategies (60 minutes)

### Learning Objectives
- Understand ensemble voting mechanisms
- Implement different voting strategies
- Optimize ensemble performance

### Videos

#### 5.1 Introduction to Ensemble AI (15 min)
- What is ensemble learning in AI?
- Benefits of multi-model consensus
- Accuracy improvements through diversity
- Real-world ensemble applications

#### 5.2 Voting Strategies (20 min)
- Available voting strategies:
  - Majority voting
  - Weighted voting
  - Consensus voting
  - Confidence-weighted voting
  - Quality-weighted voting
- Selecting the right strategy for your use case
- Configuration examples

#### 5.3 Implementing Custom Strategies (15 min)
- VotingStrategy interface
- Creating custom voting algorithms
- Testing and validating strategies
- Performance considerations

#### 5.4 Performance Optimization (10 min)
- Parallel execution strategies
- Caching for ensemble responses
- Reducing latency in multi-provider calls
- Cost optimization techniques

### Hands-On Lab
- Configure different voting strategies
- Compare results across strategies
- Implement a simple custom strategy
- Benchmark ensemble performance

---

## Module 6: AI Debate System (90 minutes)

### Learning Objectives
- Master the AI Debate configuration system
- Configure participants and LLM chains
- Implement advanced debate strategies

### Videos

#### 6.1 AI Debate Concepts (20 min)
- What is AI debate?
- Benefits of multi-agent discussions
- Consensus building through debate
- Use cases: complex reasoning, decision making, analysis

#### 6.2 Participant Configuration (20 min)
- Participant structure and roles
- Configuring participant properties:
  - Name and role
  - Debate style (analytical, creative, balanced, etc.)
  - Argumentation style (logical, evidence-based, etc.)
  - Weight and priority
  - Quality thresholds
- LLM fallback chains per participant

#### 6.3 Debate Strategies (20 min)
- Available strategies:
  - Round-robin
  - Free-form
  - Structured
  - Adversarial
  - Collaborative
- Consensus threshold configuration
- Maximal repeat rounds
- Timeout management

#### 6.4 Cognee AI Integration (15 min)
- Enhanced responses with Cognee
- Semantic analysis and insights
- Dataset configuration
- Memory integration

#### 6.5 Programmatic Debate Execution (15 min)
- Using the Debate API
- Handling debate responses
- Interpreting consensus results
- Error handling and retries

### Hands-On Lab
- Configure a multi-participant debate
- Implement different debate strategies
- Test consensus building
- Analyze debate results and quality scores

---

## Module 7: Plugin Development (75 minutes)

### Learning Objectives
- Understand the plugin architecture
- Develop custom plugins
- Implement hot-reloadable extensions

### Videos

#### 7.1 Plugin Architecture Overview (15 min)
- Plugin system design
- Hot-reloading mechanism
- Plugin lifecycle management
- Discovery and registration

#### 7.2 Plugin Interfaces (20 min)
- PluginRegistry interface
- PluginLoader interface
- Plugin metadata and configuration
- Health and metrics integration

#### 7.3 Developing Your First Plugin (25 min)
- Plugin project structure
- Implementing required interfaces
- Plugin configuration
- Building and packaging
- Testing plugins locally

#### 7.4 Advanced Plugin Topics (15 min)
- Dependency resolution
- Plugin communication
- Error handling best practices
- Performance considerations
- Deployment strategies

### Hands-On Lab
- Create a simple plugin
- Implement plugin configuration
- Test hot-reloading
- Deploy and verify in running system

---

## Module 8: MCP/LSP Integration (60 minutes)

### Learning Objectives
- Understand protocol support in HelixAgent
- Configure MCP, LSP, and ACP servers
- Implement protocol-based workflows

### Videos

#### 8.1 Protocol Support Overview (15 min)
- Unified Protocol Manager architecture
- Supported protocols:
  - MCP (Model Context Protocol)
  - LSP (Language Server Protocol)
  - ACP (Agent Client Protocol)
  - Embeddings
- Use cases for each protocol

#### 8.2 MCP Integration (15 min)
- MCP server configuration
- Tool execution and management
- MCP API endpoints
- Building MCP-enabled workflows

#### 8.3 LSP Integration (15 min)
- Language Server Protocol basics
- LSP server configuration
- Code intelligence features
- IDE integration possibilities

#### 8.4 ACP and Embeddings (15 min)
- Agent Client Protocol configuration
- Agent communication patterns
- Embedding generation and comparison
- Vector operations and semantic search

### Hands-On Lab
- Configure an MCP server
- Execute MCP tools via API
- Set up embedding generation
- Test protocol metrics and health

---

## Module 9: Optimization Features (75 minutes)

### Learning Objectives
- Implement LLM optimization techniques
- Configure semantic caching
- Use structured output generation

### Videos

#### 9.1 Optimization Framework Overview (15 min)
- 8 optimization tools integrated
- Native Go vs HTTP client implementations
- Docker services for optimization
- Configuration overview

#### 9.2 Semantic Caching with GPTCache (15 min)
- Vector similarity caching
- LRU eviction strategies
- TTL configuration
- Cache hit optimization

#### 9.3 Structured Output with Outlines (15 min)
- JSON schema validation
- Regex pattern constraints
- Choice constraints
- Ensuring output format compliance

#### 9.4 Enhanced Streaming (10 min)
- Word and sentence buffering
- Progress tracking
- Rate limiting
- Real-time response handling

#### 9.5 Advanced Optimization (SGLang, LlamaIndex) (20 min)
- RadixAttention prefix caching
- Document retrieval with HyDE
- Reranking strategies
- Cognee integration for RAG

### Hands-On Lab
- Enable semantic caching
- Configure structured output schemas
- Test enhanced streaming
- Measure optimization improvements

---

## Module 10: Security Best Practices (60 minutes)

### Learning Objectives
- Implement security best practices
- Configure authentication and authorization
- Secure production deployments

### Videos

#### 10.1 Security Architecture (15 min)
- Security-first design principles
- Authentication mechanisms
- Authorization and RBAC
- Threat model overview

#### 10.2 API Security (15 min)
- JWT token configuration
- API key management
- Rate limiting implementation
- Input validation
- Request size limits

#### 10.3 Secrets Management (15 min)
- Environment variable best practices
- API key rotation
- Secure configuration storage
- Secrets in containers

#### 10.4 Production Security Hardening (15 min)
- Network security
- Container security
- Database security
- Logging and audit trails
- Security testing with `make test-security`

### Hands-On Lab
- Configure JWT authentication
- Implement rate limiting
- Set up secure secrets management
- Run security scan with gosec

---

## Module 11: Testing and CI/CD (75 minutes)

### Learning Objectives
- Master HelixAgent testing strategies
- Set up comprehensive CI/CD pipelines
- Implement quality gates

### Videos

#### 11.1 Testing Strategy Overview (15 min)
- Test pyramid for HelixAgent
- Test types:
  - Unit tests
  - Integration tests
  - E2E tests
  - Security tests
  - Stress tests
  - Chaos tests
- Coverage targets and metrics

#### 11.2 Running Tests (20 min)
- Make commands:
  ```bash
  make test
  make test-coverage
  make test-unit
  make test-integration
  make test-e2e
  make test-security
  make test-stress
  make test-chaos
  make test-bench
  make test-race
  ```
- Test infrastructure setup
- Using Docker for test dependencies

#### 11.3 Writing Effective Tests (20 min)
- Test patterns for LLM providers
- Mocking external services
- Integration test best practices
- Performance testing guidelines

#### 11.4 Manual CI/CD and Quality Validation (20 min)
- Manual CI/CD via Makefile targets (NO automated pipelines per project constitution)
- Five-phase container-based CI system:
  - Phase 1: Go builds + all tests + integration services
  - Phase 2: Mobile (Flutter/RN + Android)
  - Phase 3: Web (Angular + Playwright + Lighthouse)
  - Phase 4: Desktop (Electron/Tauri)
  - Phase 5: Full-stack integration
- Quality gates: `make ci-validate-all`
- Container-based builds for reproducibility: `make release-all`
- Resource limits: `CI_RESOURCE_LIMIT` (low/medium/high)

### Hands-On Lab
- Run all test suites
- Analyze coverage reports
- Write a custom integration test
- Review CI/CD pipeline configuration

---

## Module 12: Challenge System and Validation (90 minutes)

### Learning Objectives
- Master the HelixAgent Challenge System
- Run RAGS, MCPS, and SKILLS challenges
- Understand strict real-result validation
- Validate system integration across 48 CLI agents

### Videos

#### 12.1 Challenge System Architecture (20 min)
- What is the Challenge System?
- Challenge types:
  - RAGS Challenge (RAG Integration)
  - MCPS Challenge (MCP Server Integration)
  - SKILLS Challenge (Skills Integration)
- Challenge execution flow
- Results directory structure
- 100% test pass rate methodology

#### 12.2 RAGS Challenge - RAG Integration (20 min)
- RAG systems tested:
  - Cognee (Knowledge Graph + Memory)
  - Qdrant (Vector Database)
  - RAG Pipeline (Hybrid Search, Reranking, HyDE)
  - Embeddings Service
- 6 test sections:
  - RAG Endpoint Availability
  - CLI Agents RAG Access
  - RAG Trigger via AI Debate
  - Cognee Integration Depth
  - Qdrant/Vector DB Integration
  - RAG Pipeline Advanced Features
- Running the RAGS challenge:
  ```bash
  ./challenges/scripts/rags_challenge.sh
  ```

#### 12.3 MCPS Challenge - MCP Server Integration (20 min)
- 22 MCP servers tested:
  - Core: filesystem, memory, fetch, git, github, gitlab
  - Database: postgres, sqlite, redis, mongodb
  - Cloud: docker, kubernetes, aws-s3, google-drive
  - Communication: slack, notion
  - Search: brave-search
  - Vector: chroma, qdrant, weaviate
- MCP Tool Search integration
- Protocol endpoints (MCP, LSP, ACP)
- Running the MCPS challenge:
  ```bash
  ./challenges/scripts/mcps_challenge.sh
  ```

#### 12.4 SKILLS Challenge - Skills Integration (15 min)
- 21 skills across 8 categories:
  - Code (generate, refactor, optimize)
  - Debug (trace, profile, analyze)
  - Search (find, grep, semantic-search)
  - Git (commit, branch, merge)
  - Deploy (build, deploy)
  - Docs (document, explain, readme)
  - Test (unit-test, integration-test)
  - Review (lint, security-scan)
- Running the SKILLS challenge:
  ```bash
  ./challenges/scripts/skills_challenge.sh
  ```

#### 12.5 Strict Real-Result Validation (15 min)
- What is strict validation?
- FALSE SUCCESS detection:
  - HTTP 200 with no real content
  - Empty choices array
  - Error messages in responses
- Content length validation
- RAG evidence detection
- Real vs mock response differentiation
- Validation code walkthrough

### Hands-On Lab
- Run all three challenges (RAGS, MCPS, SKILLS)
- Analyze challenge reports
- Review test_results.csv outputs
- Interpret pass rates and failures
- Debug a failing challenge test

---

## Module 13: MCP Tool Search and Discovery (60 minutes)

### Learning Objectives
- Master MCP Tool Search functionality
- Implement semantic tool discovery
- Configure tool suggestions for prompts

### Videos

#### 13.1 MCP Tool Search Overview (15 min)
- What is MCP Tool Search?
- Search endpoints:
  - `/v1/mcp/tools/search` - Tool search by query
  - `/v1/mcp/tools/suggestions` - AI-powered suggestions
  - `/v1/mcp/adapters/search` - Adapter search
  - `/v1/mcp/categories` - Tool categories
  - `/v1/mcp/stats` - Usage statistics
- Search result structure

#### 13.2 Tool Search Implementation (20 min)
- GET and POST search methods
- Query parameters:
  - `q` - Search query
  - `limit` - Result limit
  - `category` - Filter by category
- Search result validation
- Real-time tool discovery
- Example searches:
  ```bash
  # Search for file tools
  curl "${HELIXAGENT_URL}/v1/mcp/tools/search?q=file"

  # Search for git tools
  curl "${HELIXAGENT_URL}/v1/mcp/tools/search?q=git"

  # POST search with options
  curl -X POST "${HELIXAGENT_URL}/v1/mcp/tools/search" \
    -d '{"query": "file operations", "limit": 10}'
  ```

#### 13.3 Tool Suggestions (15 min)
- AI-powered tool suggestions
- Prompt-based recommendation
- Suggestion endpoints:
  ```bash
  curl "${HELIXAGENT_URL}/v1/mcp/tools/suggestions?prompt=list%20files"
  ```
- Integration with chat completions
- Automatic tool selection

#### 13.4 Adapter Search (10 min)
- MCP adapter discovery
- Pre-built adapters:
  - GitHub, GitLab
  - PostgreSQL, MongoDB
  - Slack, Notion
  - Filesystem, Git
- Finding adapters for your use case

### Hands-On Lab
- Search for tools using different queries
- Test tool suggestions with various prompts
- Explore adapter search functionality
- Build a custom tool discovery workflow

---

## Module 14: AI Debate System Advanced (90 minutes)

### Learning Objectives
- Configure the 25 LLM AI Debate Ensemble
- Implement multi-pass validation
- Integrate with LLMsVerifier scoring

### Videos

#### 14.1 AI Debate with 25 LLMs (25 min)
- Debate team configuration:
  - 5 positions (Analyst, Proposer, Critic, Synthesizer, Mediator)
  - 5 LLMs per position (1 primary + 4 fallbacks)
  - Total: 25 LLMs in the ensemble
- Dynamic selection via LLMsVerifier scores
- OAuth providers priority (Claude, Qwen)
- Scoring algorithm (5 weighted components):
  - ResponseSpeed (25%)
  - ModelEfficiency (20%)
  - CostEffectiveness (25%)
  - Capability (20%)
  - Recency (10%)

#### 14.2 Multi-Pass Validation System (25 min)
- Validation phases:
  1. INITIAL RESPONSE - Initial perspectives
  2. VALIDATION - Cross-validation for accuracy
  3. POLISH & IMPROVE - Refinement based on feedback
  4. FINAL CONCLUSION - Synthesized consensus
- Configuration:
  ```json
  {
    "enable_multi_pass_validation": true,
    "validation_config": {
      "enable_validation": true,
      "enable_polish": true,
      "validation_timeout": 120,
      "polish_timeout": 60,
      "min_confidence_to_skip": 0.9,
      "max_validation_rounds": 3
    }
  }
  ```
- Quality improvement metrics
- Confidence scoring

#### 14.3 Debate Orchestrator Framework (25 min)
- New framework architecture:
  - Agent Pool
  - Team Building
  - Protocol Manager
  - Knowledge Repository
- Topologies:
  - Mesh (parallel)
  - Star (hub-spoke)
  - Chain (sequential)
  - Tree (hierarchical)
- 8-phase protocol:
  - Dehallucination, SelfEvolvement, Proposal, Critique, Review, Optimization, Adversarial, Convergence
- 6 voting methods: Weighted (MiniMax), Majority, Borda Count, Condorcet, Plurality, Unanimous
- Reflexion framework: episodic memory, verbal reflection, retry-and-learn loop
- Cross-debate learning and knowledge repository

#### 14.4 Integration with 48 CLI Agents (15 min)
- CLI agent registry (48 agents):
  - 4 custom: OpenCode, Crush, KiloCode, HelixCode
  - 44 generic agents including: Aider, ClaudeCode, Cline, CodenameGoose, DeepSeekCLI, Forge, GeminiCLI, GPTEngineer, MistralCode, OllamaCode, Plandex, QwenCode, AmazonQ, CursorAI, Windsurf, and more
- Agent-specific configurations generated via HelixAgent binary
- X-CLI-Agent header support and User-Agent pattern matching
- 15+ MCP servers per agent, 10+ plugins, 8+ skills

### Hands-On Lab
- Configure a 25 LLM debate team
- Enable multi-pass validation
- Test different topologies
- Verify agent integration

---

## Bonus Content

### Appendix A: Cloud Integrations
- AWS Bedrock integration
- GCP Vertex AI integration
- Azure OpenAI integration

### Appendix B: Kubernetes Deployment
- Kubernetes manifests
- Helm charts
- Scaling configurations
- Monitoring in K8s

### Appendix C: Troubleshooting Guide
- Common issues and solutions
- Debug mode configuration
- Log analysis
- Performance diagnostics

### Appendix D: API Reference Quick Guide
- OpenAI-compatible endpoints
- Protocol endpoints
- Debate API
- Monitoring endpoints

---

## Module S7.1: Advanced AI/ML Modules — Part 1 (150 minutes)

### Prerequisites

- Modules 1-3 completed (HelixAgent basics, configuration)
- Go 1.25+ development environment
- Familiarity with Go interfaces and context.Context
- HelixAgent running locally with at least one LLM provider configured
- Understanding of Module 7 (Plugin Architecture) recommended

### Learning Objectives

Upon completion of this section, participants will be able to:

1. Build autonomous AI agents using graph-based workflows with the Agentic module
2. Implement continuous evaluation pipelines and A/B experiments with the LLMOps module
3. Collect feedback, train reward models, and run self-refinement loops with the SelfImprove module
4. Integrate all three modules via HelixAgent adapters
5. Write tests and challenges following the 100% coverage standard for each module

### Estimated Duration: 150 minutes (3 × 30-minute videos)

---

#### S7.1.1: Agentic Module — Graph-Based Workflow Orchestration (30 min)

**Module**: `digital.vasic.agentic` | **Source**: `Agentic/` | **Adapter**: `internal/adapters/agentic/`

**Topics**:
- Graph-based workflow definition: Workflow, WorkflowGraph, Node, Edge
- NodeHandler signature: `func(ctx, state, input) (*NodeOutput, error)`
- WorkflowState: mutable context threaded through all nodes
- Dynamic routing via `NodeOutput.NextNode`
- Self-correction patterns: error edges, retry nodes
- HelixAgent integration: SpecKit orchestrator uses Agentic for the 7-phase development flow

**Live Demo**: Three-node code-review agent (gather context → LLM review → generate report)

**Challenge**: `./challenges/scripts/agentic_challenge.sh`

**Key Types**:
```go
type Workflow struct{ ... }
type WorkflowGraph struct {
    Nodes map[string]*Node
    Edges []Edge
}
type Node struct {
    ID       string
    Handler  NodeHandler
    Metadata map[string]interface{}
}
type NodeHandler func(ctx context.Context, state *WorkflowState,
    input interface{}) (*NodeOutput, error)
type WorkflowState struct{ ... } // thread-safe get/set
type NodeOutput struct {
    NextNode string
    Data     interface{}
    Done     bool
}
```

---

#### S7.1.2: LLMOps Module — Evaluation, Experiments, and Prompt Versioning (30 min)

**Module**: `digital.vasic.llmops` | **Source**: `LLMOps/` | **Adapter**: `internal/adapters/llmops/`

**Topics**:
- Continuous evaluation: InMemoryContinuousEvaluator, Dataset (golden/synthetic/production)
- EvaluationRun: per-example results, AggregateMetrics (MeanScore)
- A/B experiment management: InMemoryExperimentManager, Experiment, TrafficSplit
- Statistical significance: p-values, confidence intervals on experiment outcomes
- Prompt versioning: register, compare, promote, rollback
- HelixAgent integration: LLMsVerifier startup pipeline uses evaluation under the hood; provider
  promotions gated by benchmark results

**Live Demo**: A/B experiment — DeepSeek-Coder vs Claude-3.5-Sonnet on code generation dataset

**Challenge**: `./challenges/scripts/llmops_challenge.sh`

**Key Types**:
```go
type InMemoryContinuousEvaluator struct{ ... }
type InMemoryExperimentManager struct{ ... }
type Dataset struct {
    ID       string
    Type     DatasetType // golden | synthetic | production
    Examples []Example
}
type EvaluationRun struct {
    Score      float64
    AggMetrics AggregateMetrics
    Results    []ExampleResult
}
```

---

#### S7.1.3: SelfImprove Module — RLHF, Reward Modeling, and Preference Optimization (30 min)

**Module**: `digital.vasic.selfimprove` | **Source**: `SelfImprove/` | **Adapter**: `internal/adapters/selfimprove/`

**Topics**:
- Feedback types: ExplicitFeedback (thumbs up/down), ImplicitFeedback (behavioral), PreferencePair
- RewardModel interface: Score, Train, Evaluate
- FeedbackCollector: buffer, deduplicate, batch export for training
- SelfRefinementLoop: initial response → critique → refine → score → iterate
- Direct Preference Optimization (DPO) training data generation
- PII considerations: anonymize feedback data before training
- HelixAgent integration: FeedbackCollector wired into streaming response handler

**Live Demo**: SelfRefinementLoop improving a code explanation through 3 iterations, showing score
progression from 0.62 to 0.89

**Challenge**: `./challenges/scripts/selfimprove_challenge.sh`

**Key Types**:
```go
type RewardModel interface {
    Score(ctx context.Context, prompt, response string) (float64, error)
    Train(ctx context.Context, pairs []PreferencePair) error
}
type SelfRefinementLoop struct{ ... }
type SelfRefinementConfig struct {
    MaxIterations  int
    ScoreThreshold float64
    CritiquePrompt string
    RefinePrompt   string
}
```

---

### Hands-On Lab: AI Agent Self-Improvement Pipeline

**Duration**: 45 minutes | **Difficulty**: Advanced

**Objective**: Build a complete pipeline that:
1. Defines a three-node Agentic workflow for code review
2. Runs LLMOps evaluation to measure baseline code review quality
3. Applies SelfImprove to refine low-scoring code reviews
4. Re-evaluates to confirm quality improvement

**Assessment**: `docs/courses/assessments/QUIZ_MODULE_S71_S72.md`

---

## Module S7.2: Advanced AI/ML Modules — Part 2 (60 minutes)

### Prerequisites

- Module S7.1 completed (Agentic, LLMOps, SelfImprove)
- Module 9 (Optimization Features) recommended
- Module 14 (LLMsVerifier Integration) recommended for Benchmark integration

### Learning Objectives

Upon completion of this section, participants will be able to:

1. Select the appropriate planning algorithm (HiPlan, MCTS, Tree of Thoughts) for a given task
2. Implement custom MilestoneGenerator, StepExecutor, action generators, and reward functions
3. Run standardized benchmarks (MMLU, HumanEval, GSM8K) against multiple LLM providers
4. Define custom domain benchmark datasets and interpret ComparisonReports
5. Gate provider promotions using benchmark results integrated with LLMsVerifier

### Estimated Duration: 60 minutes (2 × 30-minute videos)

---

#### S7.2.1: Planning Module — HiPlan, MCTS, and Tree of Thoughts (30 min)

**Module**: `digital.vasic.planning` | **Source**: `Planning/` | **Adapter**: `internal/adapters/planning/`

**Topics**:
- HiPlan: hierarchical decomposition for structured tasks with known milestone topology
  - `HiPlan`, `HiPlanConfig`, `MilestoneGenerator`, `StepExecutor`
  - `HierarchicalPlan`, `Milestone`, `PlanStep`, `PlanResult`
  - `LLMMilestoneGenerator` — LLM-backed milestone generation
- MCTS: Monte Carlo Tree Search for exploratory planning with uncertainty
  - `MCTS`, `MCTSConfig` (MaxIterations, ExplorationC, MaxDepth)
  - `MCTSActionGenerator`, `MCTSRewardFunction`, `MCTSRolloutPolicy`
  - `MCTSNode`, `MCTSResult`
  - `CodeActionGenerator`, `CodeRewardFunction`, `DefaultRolloutPolicy`
- Tree of Thoughts: breadth-first thought exploration for open-ended reasoning
  - `TreeOfThoughts`, `TreeOfThoughtsConfig` (BranchingFactor, MaxDepth, BeamWidth)
  - `ThoughtGenerator`, `ThoughtEvaluator`
  - `Thought`, `ThoughtNode`, `ToTResult`
  - `LLMThoughtGenerator`, `LLMThoughtEvaluator`
- Algorithm selection guide:
  - HiPlan: multi-phase software projects, research pipelines (known topology)
  - MCTS: code generation with test feedback, game-like tasks (simulatable outcomes)
  - ToT: math proofs, strategic decisions, open-ended reasoning (unknown right path)
- Cost control: MaxDepth, MaxIterations, BranchingFactor vs quality tradeoffs
- HelixAgent integration: SpecKit uses HiPlan; Debate system optionally uses ToT

**Live Demo**: HiPlan decomposing "Implement JWT authentication" → milestones + steps

**Challenge**: `./challenges/scripts/planning_challenge.sh`

**Key Configuration**:
```go
// HiPlan defaults
planning.DefaultHiPlanConfig() // MaxMilestones=5, MaxStepsPerMile=10

// MCTS defaults (UCB constant sqrt(2))
planning.DefaultMCTSConfig() // MaxIterations=100, ExplorationC=1.414, MaxDepth=10

// ToT defaults
planning.DefaultTreeOfThoughtsConfig() // BranchingFactor=3, MaxDepth=4, BeamWidth=2
```

---

#### S7.2.2: Benchmark Module — Standardized LLM Evaluation (30 min)

**Module**: `digital.vasic.benchmark` | **Source**: `Benchmark/` | **Adapter**: `internal/adapters/benchmark/`

**Topics**:
- Supported standardized benchmarks:
  - MMLU (57-subject multiple-choice, accuracy metric)
  - HumanEval (164 Python problems, pass@k metric)
  - GSM8K (8500 math word problems, exact-match accuracy)
  - SWE-Bench (real GitHub issues, resolution rate)
  - MBPP, LMSYS, HellaSwag, MATH, Custom
- BenchmarkRunner interface: Run, RunSuite, Compare
- RunConfig: BenchmarkID, Provider, Model, MaxExamples, Temperature, Timeout, CustomDataset
- BenchmarkResult: Score, SubScores (per-category), Latency (p50/p95/p99), Cost (tokens, USD)
- ComparisonReport: ranking, statistical significance (bootstrapped CI, p-values), recommendation
- Custom benchmark workflow: define Dataset → RegisterCustomDataset → Run → Compare
- PostgreSQL persistence: store results for trend analysis over weeks/months
- Resource management: GOMAXPROCS=2, MaxExamples for CI runs; full suites in nightly jobs
- HelixAgent integration: LLMsVerifier startup pipeline; provider promotion gates

**Live Demo**: HumanEval benchmark on DeepSeek-Coder vs Claude-3.5-Sonnet (50 examples each),
with ComparisonReport showing winner, p-value, and latency comparison

**Challenge**: `./challenges/scripts/benchmark_challenge.sh`

**Key API**:
```bash
# Run benchmark via HelixAgent
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Content-Type: application/json" \
  -d '{"benchmark":"humaneval","providers":["deepseek","claude"],"max_examples":50}'

# Get comparison report
curl http://localhost:7061/v1/benchmark/reports/latest | jq '.ranking'
```

---

### Hands-On Lab: Provider Benchmark and Promotion Pipeline

**Duration**: 30 minutes | **Difficulty**: Advanced

**Objective**: Build a provider promotion pipeline that:
1. Runs HumanEval and GSM8K on two providers
2. Generates a ComparisonReport
3. Promotes the winning provider if p-value < 0.05
4. Persists results to PostgreSQL for trend tracking

**Assessment**: `docs/courses/assessments/QUIZ_MODULE_S71_S72.md`

---

## Module 15: HelixLLM and AgenticEnsemble (90 minutes)

### Prerequisites

- Modules 1-6 completed
- Understanding of ensemble strategies and AI debate
- HelixAgent running with at least one LLM provider configured

### Learning Objectives

Upon completion of this section, participants will be able to:

1. Configure HelixLLM as a first-class LLM provider within HelixAgent
2. Understand the AgenticEnsemble dual-mode architecture (Reason vs Execute)
3. Implement tool-augmented reasoning through iterative tool resolution
4. Configure autonomous execution with task decomposition and verification
5. Integrate HelixLLM's RAG capabilities with HelixAgent's ensemble

### Estimated Duration: 90 minutes (3 x 30-minute videos)

---

#### 15.1 HelixLLM Provider Integration (30 min)

**Module**: `internal/llm/providers/helixllm/` | **Adapter**: `internal/adapters/helixllm/`

**Topics**:
- HelixLLM as a local AI ensemble provider with OpenAI-compatible API
- Configuration: `HELIX_LLM_ENDPOINT`, `HELIX_LLM_API_KEY`, `HELIX_LLM_MODEL`
- TLS configuration and health checks
- Embedding generation via HelixLLM
- RAG-augmented completions
- HelixLLM submodule architecture: agents, shared config, analytics, events

**Live Demo**: Configure HelixLLM and make a completion request through HelixAgent

---

#### 15.2 AgenticEnsemble Architecture (30 min)

**Source**: `internal/services/agentic_ensemble.go`

**Topics**:
- Dual-mode operation: Reason mode vs Execute mode
- Reason mode: debate service augmented with iterative tool resolution
- Execute mode: task decomposition, agent worker pool, result verification
- Intent classification for automatic mode selection
- IterativeToolExecutor: multi-step tool calls within a single request
- ExecutionPlanner: task decomposition and parallel dispatch
- VerificationDebate: multi-LLM verification of execution results
- Graceful degradation when subsystems are unavailable

**Live Demo**: Submit a complex coding task and observe dual-mode routing

---

#### 15.3 AgenticEnsemble in Practice (30 min)

**Topics**:
- Configuring AgenticEnsemble via API
- Tool registry integration for agentic tool loops
- Autonomous task execution pipeline
- Verification and quality assurance through debate
- Performance optimization: parallel execution, caching, early termination
- Integration with HelixLLM as the backend provider

**Challenge**: `./challenges/scripts/agentic_ensemble_challenge.sh`

### Hands-On Lab: AgenticEnsemble Pipeline

**Duration**: 45 minutes | **Difficulty**: Advanced

**Objective**: Build a pipeline that:
1. Configures HelixLLM as a provider
2. Submits a task requiring tool use (Reason mode)
3. Submits a multi-step task (Execute mode)
4. Verifies results through the verification debate
5. Compares output quality between modes

---

## Module 16: HTTP/3 (QUIC) and Brotli Compression (60 minutes)

### Learning Objectives

Upon completion of this section, participants will be able to:

1. Understand HTTP/3 (QUIC) transport advantages over HTTP/2
2. Configure HelixAgent's HTTP/3 client and server
3. Implement Brotli compression with gzip fallback
4. Verify QUIC transport and compression in production

### Estimated Duration: 60 minutes (2 x 30-minute videos)

---

#### 16.1 HTTP/3 (QUIC) Transport (30 min)

**Topics**:
- Why HTTP/3: reduced latency, connection migration, multiplexing without head-of-line blocking
- `quic-go/quic-go` library integration
- HTTP/3 as primary transport, HTTP/2 as fallback
- TLS 1.3 requirement for QUIC
- Client pool adapter: `internal/adapters/http/`
- Server-side QUIC listener configuration
- Health check verification over QUIC

---

#### 16.2 Brotli Compression (30 min)

**Topics**:
- Brotli vs gzip: compression ratio and speed tradeoffs
- `andybalholm/brotli` library integration
- Compression priority: Brotli (primary) then gzip (fallback)
- Content-Encoding negotiation
- Middleware configuration for response compression
- Measuring compression savings in production

**Challenge**: `./challenges/scripts/brotli_compression_challenge.sh`

### Hands-On Lab: HTTP/3 and Compression Verification

**Duration**: 30 minutes | **Difficulty**: Intermediate

**Objective**:
1. Verify QUIC transport is active via connection metadata
2. Test Brotli compression on API responses
3. Measure latency improvement vs HTTP/2
4. Verify fallback behavior when QUIC is unavailable

---

## Module 17: Remote Container Distribution (60 minutes)

### Learning Objectives

Upon completion of this section, participants will be able to:

1. Configure remote container distribution via `containers/.env`
2. Understand the mandatory container orchestration flow
3. Deploy all services to remote hosts automatically
4. Verify health checks against remote endpoints

### Estimated Duration: 60 minutes (2 x 30-minute videos)

---

#### 17.1 Container Orchestration Flow (30 min)

**Topics**:
- Centralized container management via `digital.vasic.containers` module
- Container adapter: `internal/adapters/containers/adapter.go`
- BootManager service lifecycle: compose up, health check, strict mode
- `containers/.env` configuration (NOT project root `.env`)
- `CONTAINERS_REMOTE_ENABLED=true` for remote distribution
- `CONTAINERS_REMOTE_HOST_*` variables for multi-host deployment
- Runtime auto-detection: Docker vs Podman
- No manual container starts (constitution requirement)

---

#### 17.2 Remote Distribution in Practice (30 min)

**Topics**:
- SSH-based remote deployment (key-based auth, no interactive prompts)
- Health checks against remote endpoints (TCP/HTTP)
- Required vs optional services: boot failure in strict mode
- Service overrides: `SVC_<SERVICE>_<FIELD>` environment variables
- Mandatory container rebuild after code changes
- Monitoring distributed containers

**Key Files**: `containers/.env`, `internal/config/config.go:isContainersRemoteEnabled()`, `internal/services/boot_manager.go`

**Challenge**: `./challenges/scripts/container_lazy_loading_challenge.sh`

### Hands-On Lab: Remote Container Deployment

**Duration**: 30 minutes | **Difficulty**: Advanced

**Objective**:
1. Configure `containers/.env` for remote distribution
2. Boot HelixAgent and observe automatic container deployment
3. Verify health checks against remote endpoints
4. Test failover behavior when a remote service is unavailable

---

## Module 18: HelixMemory Cognitive Engine (75 minutes)

### Learning Objectives

Upon completion of this section, participants will be able to:

1. Understand the HelixMemory 3-stage fusion pipeline
2. Configure Mem0, Cognee, Letta, and Graphiti memory backends
3. Integrate HelixMemory with the AI debate ensemble
4. Use circuit breakers and Prometheus metrics for memory operations

### Estimated Duration: 75 minutes (3 x 25-minute videos)

---

#### 18.1 HelixMemory Architecture (25 min)

**Module**: `digital.vasic.helixmemory` | **Source**: `HelixMemory/`

**Topics**:
- Unified cognitive memory engine for HelixAgent and AI debate
- 4 memory backends orchestrated through fusion:
  - Mem0: fact-based memory
  - Cognee: knowledge graphs
  - Letta: stateful agent runtime
  - Graphiti: temporal knowledge graph
- 3-stage fusion pipeline
- 12 power features
- Active by default; opt out with `-tags nohelixmemory`
- Infrastructure bridge for backend connectivity

---

#### 18.2 Memory Configuration and Integration (25 min)

**Topics**:
- Configuring memory backends (endpoints, credentials)
- Circuit breakers for memory backend failures
- Prometheus metrics for memory operations
- Memory scopes: session, user, global
- Entity graph and semantic search
- Memory consolidation and retention policies

---

#### 18.3 HelixMemory in AI Debate (25 min)

**Topics**:
- Cross-debate learning through memory
- Episodic memory for debate reflexion
- Knowledge graph enrichment from debate outcomes
- Temporal context for improved reasoning
- Memory-augmented consensus building

**Challenge**: `./challenges/scripts/helixmemory_challenge.sh` (80+ tests)

### Hands-On Lab: Cognitive Memory Pipeline

**Duration**: 30 minutes | **Difficulty**: Advanced

**Objective**:
1. Configure HelixMemory with at least 2 backends
2. Store and retrieve facts through the memory API
3. Observe memory enrichment during an AI debate
4. Query the knowledge graph for cross-session insights

---

## Module 19: HelixSpecifier Spec-Driven Development (75 minutes)

### Learning Objectives

Upon completion of this section, participants will be able to:

1. Understand the 3-pillar architecture (SpecKit + Superpowers + GSD)
2. Configure auto-activation based on work granularity detection
3. Execute the 7-phase SpecKit development flow
4. Integrate HelixSpecifier with AI debate via DebateFunc injection

### Estimated Duration: 75 minutes (3 x 25-minute videos)

---

#### 19.1 HelixSpecifier Architecture (25 min)

**Module**: `digital.vasic.helixspecifier` | **Source**: `HelixSpecifier/`

**Topics**:
- Spec-Driven Development Fusion Engine
- 3-pillar architecture:
  - SpecKit: 7-phase SDD (Constitution, Specify, Clarify, Plan, Tasks, Analyze, Implement)
  - Superpowers: TDD and subagent coordination
  - GSD: milestone-driven execution
- Adaptive ceremony scaling based on effort classification
- 5 granularity levels: single action, small creation, big creation, whole functionality, refactoring
- Auto-activation for large changes based on WorkGranularity detection
- Phase caching for resumption (`.speckit/cache/`)
- 27 packages, 835+ tests

---

#### 19.2 SpecKit 7-Phase Flow (25 min)

**Topics**:
- Phase 1: Constitution alignment
- Phase 2: Specification authoring
- Phase 3: Clarification and disambiguation
- Phase 4: Planning and decomposition
- Phase 5: Task generation
- Phase 6: Analysis and risk assessment
- Phase 7: Implementation guidance
- Spec memory for cross-session context
- Intent classifier for automatic phase selection

---

#### 19.3 DebateFunc Integration and 3-Pillar Fusion (25 min)

**Topics**:
- DebateFunc injection: real multi-LLM debate within spec workflow
- 3-pillar fusion: combining SpecKit + Superpowers + GSD outputs
- CLI agent adapters for agent-specific spec generation
- 10 power features overview
- Configuration via SpecKit orchestrator

**Challenge**: `./challenges/scripts/helixspecifier_challenge.sh` (138 tests)

### Hands-On Lab: Spec-Driven Development Workflow

**Duration**: 30 minutes | **Difficulty**: Advanced

**Objective**:
1. Trigger HelixSpecifier auto-activation with a refactoring task
2. Walk through the 7-phase flow
3. Observe DebateFunc integration with live multi-LLM debate
4. Review generated specification and implementation plan

---

## Certification Path

### Level 1: HelixAgent Fundamentals
- Modules 1-3
- Basic installation and configuration
- Assessment: Written quiz + Lab exercise

### Level 2: Provider Expert
- Modules 4-6
- Multi-provider and ensemble mastery
- Assessment: Implementation project

### Level 3: Advanced Practitioner
- Modules 7-9
- Plugin development and optimization
- Assessment: Custom plugin submission

### Level 4: Master Engineer
- Modules 10-11
- Security and CI/CD mastery
- Assessment: Full production deployment review

### Level 5: Challenge Expert
- Modules 12-14
- Challenge system mastery and advanced AI debate
- Assessment: 100% pass rate on all challenge scripts
- Requirements:
  - Run RAGS, MCPS, and SKILLS challenges successfully
  - Configure 25 LLM debate team
  - Demonstrate MCP Tool Search integration
  - Document strict validation methodology

### Level 6: AI/ML Systems Architect
- Modules S7.1-S7.2
- Advanced AI/ML extracted module mastery
- Assessment: End-to-end AI agent system implementation
- Requirements:
  - Implement a complete Agentic workflow with at least 4 nodes and dynamic routing
  - Run LLMOps A/B experiment between two providers and document results
  - Demonstrate SelfRefinementLoop improving response quality by >20% over 3 iterations
  - Run HiPlan or MCTS on a real planning task and verify output
  - Complete Benchmark comparison of at least 2 providers on HumanEval with p-value analysis
  - Pass all 5 module challenge scripts with 100% test pass rate

### Level 7: Platform Architect
- Modules 15-19
- Full platform mastery: HelixLLM, AgenticEnsemble, HTTP/3, remote distribution, cognitive memory, spec-driven development
- Assessment: Complete platform deployment with all advanced features
- Requirements:
  - Configure HelixLLM provider and demonstrate AgenticEnsemble dual-mode operation
  - Verify HTTP/3 (QUIC) transport with Brotli compression
  - Deploy services to remote hosts via container distribution
  - Integrate HelixMemory with at least 2 memory backends
  - Execute a HelixSpecifier 7-phase workflow with DebateFunc integration
  - Pass all related challenge scripts with 100% test pass rate

---

## Course Resources

### Downloadable Materials
- Complete configuration examples
- API reference sheets
- Best practices checklists
- Troubleshooting guides
- Code templates and snippets

### Support Channels
- Course discussion forums
- GitHub Issues
- Community Discord
- Office hours sessions

---

*Course Version: 5.0.0*
*Last Updated: April 2026*
*Total Duration: 20+ hours*
*Modules: 19 + 2 advanced AI/ML sections (S7.1, S7.2) = 7 certification levels*
