# HelixAgent Quick Reference Card

## Essential Commands

### Build & Run
```bash
make build              # Build binary
make run                # Run server
make run-dev            # Run in debug mode
./bin/helixagent        # Run directly (auto-starts all containers)
```

### Release Builds (inside containers)
```bash
make release            # Build helixagent for all platforms
make release-all        # Build ALL 7 apps for all platforms
make release-info       # Show version codes and source hashes
```

### Testing
```bash
make test               # All tests
make test-unit          # Unit tests only
make test-integration   # Integration tests
make test-e2e           # End-to-end tests
make test-security      # Security tests
make test-stress        # Stress tests
make test-bench         # Benchmarks
make test-race          # Race detection
make test-coverage      # With coverage report
```

### Code Quality
```bash
make fmt                # Format code
make lint               # Run linter
make vet                # Static analysis
make security-scan      # Security check (gosec)
make ci-validate-all    # All validation checks
```

### CI (Manual, Container-Based)
```bash
make ci-all             # All five CI phases
make ci-go              # Phase 1: Go builds + tests
make ci-report          # Aggregate reports
```

---

## API Endpoints

### Health & Status
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/v1/models` | GET | List models |
| `/v1/providers/status` | GET | Provider status |
| `/v1/providers/health` | GET | Provider health |
| `/v1/monitoring/status` | GET | Full system status |
| `/v1/startup/verification` | GET | Startup verification |

### Chat Completions
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Chat completion |
| `/v1/embeddings` | POST | Generate embeddings |
| `/v1/completion/*` | POST | Completion routes |

### AI Debate
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/debates` | POST | Create debate |
| `/v1/debates/:id` | GET | Get debate |
| `/v1/debates/:id/status` | GET | Debate status |
| `/v1/debates/:id` | DELETE | Cancel debate |
| `/v1/ensemble/sessions` | GET | Ensemble sessions |
| `/v1/ensemble/teams` | GET | Ensemble teams |

### Protocols
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/mcp` | * | MCP protocol |
| `/v1/mcp/tools` | GET | List MCP tools |
| `/v1/mcp/tools/search` | GET/POST | Search tools |
| `/v1/mcp/tools/suggestions` | GET | AI-powered suggestions |
| `/v1/mcp/categories` | GET | Tool categories |
| `/v1/acp` | * | Agent Client Protocol |
| `/v1/lsp` | * | Language Server Protocol |
| `/v1/embeddings` | POST | Embeddings |
| `/v1/vision` | * | Vision analysis |
| `/v1/cognee` | * | Cognee (optional) |

### AI/ML
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/agentic/workflows` | * | Agentic workflows |
| `/v1/planning/{hiplan,mcts,tot}` | POST | Planning algorithms |
| `/v1/llmops/{experiments,evaluate,prompts}` | * | LLM operations |
| `/v1/benchmark/{run,results}` | * | Benchmarking |
| `/v1/qa/{sessions,findings}` | * | QA orchestration |

### Infrastructure
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/discovery` | GET | Provider discovery |
| `/v1/scoring` | GET | Provider scoring |
| `/v1/verification` | GET | Verification results |
| `/v1/tasks` | * | Background tasks |
| `/v1/bigdata/health` | GET | BigData health |

---

## Chat Completion Request

```json
{
  "model": "helixagent-debate",
  "messages": [
    {"role": "system", "content": "System prompt"},
    {"role": "user", "content": "User message"}
  ],
  "temperature": 0.7,
  "max_tokens": 1000,
  "stream": false
}
```

---

## Environment Variables

### Required (at least one provider)
```bash
CLAUDE_API_KEY=your-key
DEEPSEEK_API_KEY=your-key
GEMINI_API_KEY=your-key
OPENROUTER_API_KEY=your-key
```

### Server Configuration
```bash
PORT=7061
GIN_MODE=release
JWT_SECRET=your-secret
```

### Database
```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=helixagent
DB_PASSWORD=password
DB_NAME=helixagent
```

### Cache
```bash
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
```

### HelixLLM Provider
```bash
HELIX_LLM_ENDPOINT=https://localhost:8443
HELIX_LLM_API_KEY=your-key
HELIX_LLM_MODEL=helixllm-default
```

### Container Orchestration (in Containers/.env)
```bash
CONTAINERS_REMOTE_ENABLED=false
CONTAINERS_REMOTE_HOST_1=user@remote-host
```

### Service Overrides
```bash
SVC_POSTGRESQL_HOST=localhost
SVC_REDIS_PORT=16379
SVC_REDIS_REMOTE=true
```

---

## Supported Providers (43)

| Provider | Key Variable | Notes |
|----------|--------------|-------|
| AI21 | `AI21_API_KEY` | Enterprise NLP |
| Anthropic | `ANTHROPIC_API_KEY` | Claude via API |
| Cerebras | `CEREBRAS_API_KEY` | Ultra-fast inference |
| Chutes | `CHUTES_API_KEY` | Endpoint: llm.chutes.ai |
| Claude | `CLAUDE_API_KEY` | OAuth/CLI proxy |
| Cloudflare | `CLOUDFLARE_API_KEY` | Workers AI |
| Codestral | `CODESTRAL_API_KEY` | Code generation |
| Cohere | `COHERE_API_KEY` | Endpoint: api.cohere.com/v2 |
| DeepSeek | `DEEPSEEK_API_KEY` | Code, technical |
| Fireworks | `FIREWORKS_API_KEY` | Fast inference |
| Gemini | `GEMINI_API_KEY` | Multimodal, scientific |
| GitHub Models | `GITHUB_MODELS_API_KEY` | GitHub-hosted |
| Groq | `GROQ_API_KEY` | Ultra-fast inference |
| HelixLLM | `HELIX_LLM_API_KEY` | Local ensemble + RAG |
| HuggingFace | `HUGGINGFACE_API_KEY` | Open models |
| Hyperbolic | `HYPERBOLIC_API_KEY` | Specialized |
| Junie | `JUNIE_API_KEY` | CLI/ACP mode |
| Kilo | `KILO_API_KEY` | Specialized |
| Kimi | `KIMI_API_KEY` | Moonshot (api.moonshot.cn) |
| KimiCode | `KIMICODE_API_KEY` | Code-focused Kimi |
| Mistral | `MISTRAL_API_KEY` | European AI |
| Modal | `MODAL_API_KEY` | Serverless inference |
| Nia | `NIA_API_KEY` | Specialized |
| NLPCloud | `NLPCLOUD_API_KEY` | NLP APIs |
| Novita | `NOVITA_API_KEY` | Cost-effective |
| Nvidia | `NVIDIA_API_KEY` | NIM inference |
| Ollama | (none) | Local, free |
| OpenAI | `OPENAI_API_KEY` | GPT models |
| OpenRouter | `OPENROUTER_API_KEY` | Meta-provider |
| Perplexity | `PERPLEXITY_API_KEY` | Search-augmented |
| PublicAI | `PUBLICAI_API_KEY` | Decentralized |
| Qwen | `QWEN_API_KEY` | Multilingual, ACP mode |
| Replicate | `REPLICATE_API_KEY` | Run open models |
| SambaNova | `SAMBANOVA_API_KEY` | Enterprise AI |
| Sarvam | `SARVAM_API_KEY` | Indian languages |
| SiliconFlow | `SILICONFLOW_API_KEY` | Chinese models |
| Together | `TOGETHER_API_KEY` | Open model hosting |
| Upstage | `UPSTAGE_API_KEY` | Korean AI |
| Venice | `VENICE_API_KEY` | Privacy-focused |
| VulaVula | `VULAVULA_API_KEY` | African languages |
| xAI | `XAI_API_KEY` | Grok models |
| ZAI | `ZAI_API_KEY` | Zhipu GLM (api.z.ai) |
| Zen | `OPENCODE_API_KEY` | OpenCode serve |
| Zhipu | `ZHIPU_API_KEY` | Chinese AI |

*Plus generic OpenAI-compatible provider for unlisted services.*

---

## AI Debate Roles

| Role | Character | Purpose |
|------|-----------|---------|
| [A] | THE ANALYST | Systematic analysis |
| [P] | THE PROPOSER | Solution proposals |
| [C] | THE CRITIC | Challenge assumptions |
| [S] | THE SYNTHESIZER | Combine perspectives |
| [M] | THE MEDIATOR | Reach consensus |

---

## Debate Strategies

| Strategy | Description |
|----------|-------------|
| `round_robin` | Fixed turn order |
| `free_form` | Dynamic order |
| `structured` | Organized rounds |
| `adversarial` | Opposing views |
| `collaborative` | Build together |

---

## Voting Strategies

| Strategy | Description |
|----------|-------------|
| `weighted` | MiniMax weighted voting |
| `majority` | Simple vote count |
| `borda_count` | Borda Count ranking |
| `condorcet` | Condorcet with cycle detection + Borda fallback |
| `plurality` | Plurality voting |
| `unanimous` | All must agree |
| `confidence_weighted` | By confidence scores |
| `quality_weighted` | By quality scores |

---

## Debate Topologies

| Topology | Description |
|----------|-------------|
| `mesh` | Parallel (all agents communicate) |
| `star` | Hub-spoke (mediator at center) |
| `chain` | Sequential pipeline |
| `tree` | Hierarchical decomposition |

---

## 8-Phase Debate Protocol

1. **Dehallucination** -- Fact verification
2. **SelfEvolvement** -- Self-improvement
3. **Proposal** -- Initial proposals
4. **Critique** -- Critical review
5. **Review** -- Cross-validation
6. **Optimization** -- Refinement
7. **Adversarial** -- Red/Blue team attack-defend
8. **Convergence** -- Consensus building

---

## Debate Styles

| Style | Format |
|-------|--------|
| `theater` | Theatrical dialogue (default) |
| `novel` | Prose narration |
| `screenplay` | Script format |
| `minimal` | Plain text |

---

## AgenticEnsemble Modes

| Mode | Trigger | Behavior |
|------|---------|----------|
| Reason | Complex reasoning tasks | Debate + iterative tool resolution |
| Execute | Multi-step execution tasks | Decompose + dispatch + verify |

---

## Key Configuration Files

| File | Purpose |
|------|---------|
| `.env` | API keys and server config |
| `Containers/.env` | Container orchestration (local/remote) |
| `configs/development.yaml` | Dev settings |
| `configs/production.yaml` | Prod settings |
| `.speckit/cache/` | HelixSpecifier phase cache |

---

## Useful Curl Commands

### Health Check
```bash
curl http://localhost:7061/health
```

### Chat Completion
```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Hello"}]}'
```

### Streaming
```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -N \
  -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Hello"}],"stream":true}'
```

### Create Debate
```bash
curl -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d '{"topic":"Your topic","rounds":3}'
```

### MCP Tool Search
```bash
curl "http://localhost:7061/v1/mcp/tools/search?q=file"
```

### Run Benchmark
```bash
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Content-Type: application/json" \
  -d '{"benchmark":"humaneval","providers":["deepseek","claude"],"max_examples":50}'
```

---

## Error Codes

| Code | Error | Resolution |
|------|-------|------------|
| 400 | Invalid request | Check JSON format |
| 401 | Auth failed | Verify API key |
| 403 | Access denied | Check permissions |
| 404 | Not found | Verify endpoint |
| 429 | Rate limited | Reduce requests |
| 500 | Server error | Check logs |

---

## Metrics Endpoints

```bash
# Prometheus metrics
curl http://localhost:7061/metrics

# Health checks
curl http://localhost:7061/healthz/live
curl http://localhost:7061/healthz/ready
```

---

## Challenge Scripts (Key)

```bash
./challenges/scripts/run_all_challenges.sh              # All challenges
./challenges/scripts/helixmemory_challenge.sh            # 80+ tests
./challenges/scripts/helixspecifier_challenge.sh          # 138 tests
./challenges/scripts/agentic_ensemble_challenge.sh        # AgenticEnsemble
./challenges/scripts/brotli_compression_challenge.sh      # 11 tests
./challenges/scripts/container_lazy_loading_challenge.sh  # 13 tests
./challenges/scripts/debate_orchestrator_challenge.sh     # 61 tests
./challenges/scripts/cli_agent_config_challenge.sh        # 60 tests
```

---

*Quick Reference v2.0.0 | April 2026*
