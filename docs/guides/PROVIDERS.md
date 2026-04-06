# Provider Configuration Guide

HelixAgent supports 43 LLM providers with dynamic selection based on real-time verification scores. This guide explains how to configure each.

## Quick Reference

| Provider | Environment Variable | Models | Features |
|----------|---------------------|--------|----------|
| OpenAI | `OPENAI_API_KEY` | GPT-4o, GPT-4o-mini | Tools, Vision, Streaming |
| Anthropic | `ANTHROPIC_API_KEY` | Claude 3.5 Sonnet, Haiku | Tools, Vision, 200K context |
| Claude (OAuth) | `CLAUDE_USE_OAUTH_CREDENTIALS` | Claude CLI proxy | Session continuity |
| DeepSeek | `DEEPSEEK_API_KEY` | DeepSeek-V3, R1 | Tools, Reasoning, 64K context |
| Groq | `GROQ_API_KEY` | Llama 3.1/3.2, Mixtral | Tools, Vision, 800+ tok/s |
| Mistral | `MISTRAL_API_KEY` | Mistral Small/Large | Tools, Agents |
| Gemini | `GEMINI_API_KEY` | Gemini 2.0 Flash, 1.5 Pro | Tools, Vision, 1M context |
| Cohere | `COHERE_API_KEY` | Command R, R+ | Tools, RAG |
| Perplexity | `PERPLEXITY_API_KEY` | Sonar, Sonar Pro | Search-enhanced |
| Together AI | `TOGETHER_API_KEY` | 100+ open source | Various |
| Fireworks | `FIREWORKS_API_KEY` | Llama, Mixtral | Fast inference |
| Cerebras | `CEREBRAS_API_KEY` | Llama 3.1 | Wafer-scale speed |
| xAI | `XAI_API_KEY` | Grok 2, Grok 3 | Real-time |
| OpenRouter | `OPENROUTER_API_KEY` | Multi-provider routing | Aggregated access |
| Qwen | `QWEN_API_KEY` | Qwen models | ACP/CLI proxy |
| ZAI (Zhipu) | `ZAI_API_KEY` | GLM-4 | International API |
| Zen | `OPENCODE_API_KEY` | OpenCode models | HTTP server mode |
| Ollama | `OLLAMA_HOST` | Local models | Local inference |
| AI21 | `AI21_API_KEY` | Jamba | Mamba architecture |
| Chutes | `CHUTES_API_KEY` | Various | GPU inference |
| Cloudflare | `CLOUDFLARE_API_KEY` | Workers AI | Edge inference |
| Codestral | `CODESTRAL_API_KEY` | Codestral | Code completion |
| GitHub Models | `GITHUB_TOKEN` | Various | GitHub-hosted |
| HuggingFace | `HUGGINGFACE_API_KEY` | Various | Open source models |
| Hyperbolic | `HYPERBOLIC_API_KEY` | Various | Efficient inference |
| Junie | `JUNIE_API_KEY` | Junie | CLI/ACP proxy |
| Kilo | `KILO_API_KEY` | Various | Fast inference |
| Kimi | `KIMI_API_KEY` | Moonshot | Chinese/English |
| KimiCode | `KIMICODE_API_KEY` | Kimi Code | Code-focused |
| Modal | `MODAL_API_KEY` | Various | Serverless |
| Nia | `NIA_API_KEY` | Various | Efficient inference |
| NLPCloud | `NLPCLOUD_API_KEY` | Various | NLP-optimized |
| Novita | `NOVITA_API_KEY` | Various | AI infrastructure |
| Nvidia | `NVIDIA_API_KEY` | Various | GPU-optimized |
| PublicAI | `PUBLICAI_API_KEY` | Various | Public API |
| Replicate | `REPLICATE_API_KEY` | Various | Model hosting |
| SambaNova | `SAMBANOVA_API_KEY` | Various | Chip-optimized |
| Sarvam | `SARVAM_API_KEY` | Various | Indic languages |
| SiliconFlow | `SILICONFLOW_API_KEY` | Various | Efficient inference |
| Upstage | `UPSTAGE_API_KEY` | Solar | Document AI |
| Venice | `VENICE_API_KEY` | Various | Privacy-focused |
| VulaVula | `VULAVULA_API_KEY` | Various | African languages |
| Zhipu | `ZHIPU_API_KEY` | GLM | Chinese AI |

## Configuration

### OpenAI

```bash
export OPENAI_API_KEY="sk-..."
```

Recommended models:
- `gpt-4o` - Best quality, vision
- `gpt-4o-mini` - Cost-effective

### Anthropic

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

Recommended models:
- `claude-3-5-sonnet-20241022` - Best overall
- `claude-3-5-haiku-20241022` - Fast, cheap

Features:
- 200K context window
- Excellent tool use
- Prompt caching

### DeepSeek

```bash
export DEEPSEEK_API_KEY="sk-..."
```

Recommended models:
- `deepseek-chat` - General purpose
- `deepseek-reasoner` - Chain-of-thought reasoning

### Groq

```bash
export GROQ_API_KEY="gsk_..."
```

Recommended models:
- `llama-3.1-8b-instant` - Fastest
- `llama-3.1-70b-versatile` - Best quality
- `llama-3.2-11b-vision-preview` - Vision

Features:
- 800+ tokens/second
- Very low latency

### Mistral

```bash
export MISTRAL_API_KEY="..."
```

Recommended models:
- `mistral-small-latest` - Fast
- `mistral-large-latest` - Best quality
- `codestral-latest` - Code

### Gemini

```bash
export GEMINI_API_KEY="..."
```

Recommended models:
- `gemini-2.0-flash-exp` - Fast, capable
- `gemini-1.5-pro` - 1M context

## Provider Selection

HelixAgent uses LLMsVerifier to dynamically select the best providers on startup. The 8-test verification pipeline scores each provider across 5 weighted dimensions:

- **ResponseSpeed** (25%) - How fast the provider responds
- **CostEffectiveness** (25%) - Cost per token value
- **ModelEfficiency** (20%) - Output quality per resource
- **Capability** (20%) - Feature support (tools, vision, etc.)
- **Recency** (10%) - Model freshness

Providers scoring below 5.0 are excluded. The top-scoring providers form the debate team. OAuth-based providers receive a +0.5 bonus. Free-tier providers score 6.0-7.0.

```bash
# Check startup verification results
curl http://localhost:7061/v1/startup/verification

# Run verification for a specific provider
make verifier-verify MODEL=gpt-4 PROVIDER=openai
```

## OAuth/CLI Proxy Providers

Some providers use CLI proxies when direct API access is restricted:

- **Claude**: `claude -p --output-format json` (set `CLAUDE_USE_OAUTH_CREDENTIALS=true`)
- **Qwen**: ACP via `qwen --acp` (set `QWEN_USE_OAUTH_CREDENTIALS=true`)
- **Zen**: HTTP server `opencode serve :4096` (uses `OPENCODE_API_KEY`)
- **Gemini**: `gemini -p --output-format json` (CLI mode when no `GEMINI_API_KEY`)
- **Junie**: `junie --acp` (uses `JUNIE_API_KEY`)

## Testing Provider Setup

```bash
# Check provider health
curl http://localhost:7061/v1/monitoring/status

# Verify specific provider
make verifier-verify MODEL=gpt-4 PROVIDER=openai
```

## Troubleshooting

### Rate Limits

If you hit rate limits:

```bash
# Enable circuit breaker
export CIRCUIT_BREAKER_ENABLED=true

# Use multiple providers
export FALLBACK_PROVIDERS="groq,mistral"
```

### Authentication Errors

Check API key format:

```bash
# OpenAI: sk-...
# Anthropic: sk-ant-...
# Groq: gsk_...
# DeepSeek: sk-...
```

### Timeout Issues

```bash
# Increase timeout
export LLM_TIMEOUT=60s
```
