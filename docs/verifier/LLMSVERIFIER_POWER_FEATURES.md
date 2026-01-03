# LLMsVerifier Power Features in SuperAgent

This document explains how SuperAgent leverages LLMsVerifier's power features to automatically discover, verify, score, and select the best LLM models for AI debate ensemble communication.

## Table of Contents

1. [Overview](#overview)
2. [Automatic Model Discovery](#automatic-model-discovery)
3. [Model Verification Pipeline](#model-verification-pipeline)
4. [Scoring System](#scoring-system)
5. [Automatic Best Model Selection](#automatic-best-model-selection)
6. [AI Debate Integration](#ai-debate-integration)
7. [Configuration](#configuration)
8. [Power Features Summary](#power-features-summary)

---

## Overview

### The Problem

Traditional LLM integration requires users to:
- Manually specify each model by name
- Research which models are available
- Test each model individually
- Make educated guesses about which models perform best
- Update configurations when new models are released

### The LLMsVerifier Solution

With LLMsVerifier integration, SuperAgent users simply provide:
- **API keys** for their LLM providers
- **Optionally**: Custom API base URLs

LLMsVerifier then automatically:
1. **Discovers** all available models from each provider
2. **Verifies** each model works correctly (including code visibility)
3. **Scores** each model using 5 weighted components
4. **Selects** the top-scoring models
5. **Provides** them to SuperAgent for AI debate ensemble

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         USER INPUT (Minimal)                            │
│  ┌──────────────────────┐  ┌──────────────────────┐                    │
│  │ OPENAI_API_KEY=sk-...│  │ ANTHROPIC_API_KEY=...│  (Just API keys!) │
│  └──────────────────────┘  └──────────────────────┘                    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    LLMSVERIFIER AUTOMATIC PIPELINE                      │
│                                                                         │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐ │
│  │  DISCOVER   │──▶│   VERIFY    │──▶│   SCORE     │──▶│   SELECT    │ │
│  │  All Models │   │  Each Model │   │  All Models │   │  Top Models │ │
│  └─────────────┘   └─────────────┘   └─────────────┘   └─────────────┘ │
│                                                                         │
│  Found: 47 models   Verified: 42     Scored: 42      Selected: Top 5   │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    SUPERAGENT AI DEBATE ENSEMBLE                        │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  GPT-4o (SC:9.4) ◄──► Claude 3.5 (SC:9.2) ◄──► Gemini 1.5 (SC:8.9)│ │
│  │         ▲                     ▲                      ▲           │   │
│  │         └─────────────────────┼──────────────────────┘           │   │
│  │                    AI DEBATE (Consensus Building)                │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  Output: Best consensus answer from top-performing verified models      │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Automatic Model Discovery

### How It Works

When you provide only API keys, LLMsVerifier's **Provider Discovery** feature automatically queries each provider's API to discover all available models.

### Supported Provider APIs

| Provider | Discovery Endpoint | Example Models Discovered |
|----------|-------------------|---------------------------|
| OpenAI | `GET /v1/models` | gpt-4o, gpt-4-turbo, gpt-3.5-turbo, o1-preview |
| Anthropic | `GET /v1/models` | claude-3-5-sonnet, claude-3-opus, claude-3-haiku |
| Google | `GET /v1/models` | gemini-1.5-pro, gemini-1.5-flash, gemini-pro |
| Groq | `GET /v1/models` | llama-3.3-70b, mixtral-8x7b |
| Together | `GET /v1/models` | llama-3-70b, mistral-7b, qwen-72b |
| Mistral | `GET /v1/models` | mistral-large, mistral-medium, codestral |
| DeepSeek | `GET /v1/models` | deepseek-chat, deepseek-coder |
| Ollama | `GET /api/tags` | llama3.2, codellama, mistral (local) |
| OpenRouter | `GET /v1/models` | All 100+ models from multiple providers |

### Configuration for Auto-Discovery

```yaml
# configs/verifier.yaml

verifier:
  challenges:
    enabled: true
    provider_discovery: true    # Enable automatic model discovery
    model_verification: true    # Verify all discovered models
    config_generation: true     # Auto-generate optimal config

providers:
  # MINIMAL CONFIGURATION - Just API keys!
  openai:
    enabled: true
    api_key: "${OPENAI_API_KEY}"
    # No models list needed - auto-discovered!

  anthropic:
    enabled: true
    api_key: "${ANTHROPIC_API_KEY}"
    # No models list needed - auto-discovered!

  google:
    enabled: true
    api_key: "${GOOGLE_API_KEY}"
    # No models list needed - auto-discovered!

  # Custom base URL example (for proxies, enterprise endpoints)
  openai_enterprise:
    enabled: true
    api_key: "${OPENAI_ENTERPRISE_KEY}"
    base_url: "https://openai.mycompany.com/v1"
    # Models auto-discovered from custom endpoint!
```

### Environment Variables Only (Simplest Setup)

```bash
# .env - That's ALL you need!
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GOOGLE_API_KEY=AIza...
GROQ_API_KEY=gsk_...

# Optional: Custom base URLs
# OPENAI_BASE_URL=https://proxy.mycompany.com/v1
```

---

## Model Verification Pipeline

### The "Do You See My Code?" Test

LLMsVerifier's signature verification test ensures models can actually see context:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    CODE VISIBILITY VERIFICATION                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  STEP 1: Inject Code Sample                                         │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  ```python                                                    │   │
│  │  def calculate_fibonacci(n):                                  │   │
│  │      if n <= 1:                                               │   │
│  │          return n                                             │   │
│  │      return calculate_fibonacci(n-1) + calculate_fibonacci(n-2)│  │
│  │  ```                                                          │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  STEP 2: Ask "Do you see my code?"                                  │
│                                                                      │
│  STEP 3: Analyze Response                                            │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  ✓ "Yes, I can see your Python code"                        │   │
│  │  ✓ "The function calculate_fibonacci..."                     │   │
│  │  ✓ "It implements recursive Fibonacci..."                    │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  RESULT: code_visible = true, confidence = 0.98                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Complete Verification Test Suite

Each discovered model undergoes 8 verification tests:

| Test | Description | Why It Matters |
|------|-------------|----------------|
| **code_visibility** | "Do you see my code?" | Ensures context awareness |
| **existence** | Model responds to basic prompt | Confirms model is accessible |
| **responsiveness** | Measures response quality | Ensures coherent outputs |
| **latency** | Measures response time | Performance baseline |
| **streaming** | Tests streaming capability | Real-time response support |
| **function_calling** | Tests tool use capability | API integration support |
| **coding_capability** | Tests code generation | Developer use cases |
| **error_detection** | Tests bug finding ability | Code review capability |

### Verification Results

```json
{
  "model_id": "gpt-4o",
  "provider": "openai",
  "verified": true,
  "tests": {
    "code_visibility": true,
    "existence": true,
    "responsiveness": true,
    "latency": true,
    "streaming": true,
    "function_calling": true,
    "coding_capability": true,
    "error_detection": true
  },
  "verification_time_ms": 3456,
  "code_visible": true
}
```

---

## Scoring System

### 5-Component Weighted Scoring

Every verified model receives a comprehensive score (0-10):

```
┌─────────────────────────────────────────────────────────────────────┐
│                    SCORING FORMULA                                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Overall Score = (Speed × 0.25) + (Efficiency × 0.20) +             │
│                  (Cost × 0.25) + (Capability × 0.20) +              │
│                  (Recency × 0.10)                                   │
│                                                                      │
│  ┌──────────────────┬────────┬──────────────────────────────────┐  │
│  │ Component        │ Weight │ Measures                          │  │
│  ├──────────────────┼────────┼──────────────────────────────────┤  │
│  │ Response Speed   │  25%   │ Latency, tokens/second            │  │
│  │ Model Efficiency │  20%   │ Quality per token, output quality │  │
│  │ Cost Effective   │  25%   │ Price/performance ratio           │  │
│  │ Capability       │  20%   │ Features, context window, tools   │  │
│  │ Recency          │  10%   │ How recently updated              │  │
│  └──────────────────┴────────┴──────────────────────────────────┘  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Score Suffix Display

Models display their scores inline:
- `GPT-4o (SC:9.4)`
- `Claude 3.5 Sonnet (SC:9.2)`
- `Gemini 1.5 Pro (SC:8.9)`

### models.dev Integration

LLMsVerifier optionally integrates with [models.dev](https://models.dev) for enhanced scoring data:

```yaml
scoring:
  models_dev_enabled: true
  models_dev_endpoint: "https://api.models.dev"
```

This provides:
- Real-world benchmark data
- Community ratings
- Pricing information
- Capability matrices

---

## Automatic Best Model Selection

### Selection Algorithm

After discovery, verification, and scoring, LLMsVerifier automatically selects the best models:

```python
# Pseudo-code for selection algorithm

def select_best_models(all_models, config):
    # Step 1: Filter by verification
    verified = [m for m in all_models if m.verified and m.code_visible]

    # Step 2: Sort by score
    sorted_models = sorted(verified, key=lambda m: m.overall_score, reverse=True)

    # Step 3: Apply diversity (different providers)
    selected = []
    providers_used = set()

    for model in sorted_models:
        if len(selected) >= config.max_models:
            break
        if config.require_diversity and model.provider in providers_used:
            continue
        selected.append(model)
        providers_used.add(model.provider)

    return selected
```

### Selection Configuration

```yaml
# configs/verifier.yaml

model_selection:
  enabled: true
  max_models: 5                    # Top N models for ensemble
  min_score: 7.0                   # Minimum acceptable score
  require_verification: true       # Must pass all tests
  require_code_visibility: true    # Must see code
  require_diversity: true          # Different providers preferred

  # Provider priority (for tie-breaking)
  provider_priority:
    - openai
    - anthropic
    - google
    - groq
    - together
```

### Example Selection Output

```
┌─────────────────────────────────────────────────────────────────────┐
│              AUTOMATIC MODEL SELECTION RESULTS                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Discovered: 47 models from 8 providers                             │
│  Verified:   42 models (5 failed verification)                      │
│  Scored:     42 models                                              │
│  Selected:   5 models for AI Debate Ensemble                        │
│                                                                      │
│  ┌────┬─────────────────────────┬────────────┬───────┬───────────┐ │
│  │ #  │ Model                   │ Provider   │ Score │ Code Viz  │ │
│  ├────┼─────────────────────────┼────────────┼───────┼───────────┤ │
│  │ 1  │ gpt-4o                  │ openai     │ 9.4   │ ✓         │ │
│  │ 2  │ claude-3-5-sonnet       │ anthropic  │ 9.2   │ ✓         │ │
│  │ 3  │ gemini-1.5-pro          │ google     │ 8.9   │ ✓         │ │
│  │ 4  │ llama-3.3-70b           │ groq       │ 8.7   │ ✓         │ │
│  │ 5  │ deepseek-chat           │ deepseek   │ 8.5   │ ✓         │ │
│  └────┴─────────────────────────┴────────────┴───────┴───────────┘ │
│                                                                      │
│  These 5 models will participate in AI Debate Ensemble              │
└─────────────────────────────────────────────────────────────────────┘
```

---

## AI Debate Integration

### How Selected Models Power AI Debate

The top-scoring verified models are automatically used by SuperAgent's AI Debate system:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    AI DEBATE ENSEMBLE FLOW                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  USER QUERY: "What's the best approach to implement a rate limiter?" │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                      ROUND 1: INITIAL RESPONSES                │  │
│  │                                                                │  │
│  │  GPT-4o (SC:9.4)     → Token bucket algorithm recommended      │  │
│  │  Claude 3.5 (SC:9.2) → Sliding window approach suggested       │  │
│  │  Gemini 1.5 (SC:8.9) → Leaky bucket with Redis backend         │  │
│  │  Llama 3.3 (SC:8.7)  → Fixed window with distributed locks     │  │
│  │  DeepSeek (SC:8.5)   → Token bucket with burst handling        │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                │                                     │
│                                ▼                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                      ROUND 2: DEBATE & CRITIQUE                │  │
│  │                                                                │  │
│  │  Each model reviews others' responses:                         │  │
│  │  • GPT-4o critiques sliding window memory usage                │  │
│  │  • Claude 3.5 notes token bucket burst handling gap            │  │
│  │  • Gemini 1.5 points out distributed system considerations     │  │
│  │  • Models update positions based on valid critiques            │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                │                                     │
│                                ▼                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                      ROUND 3: CONSENSUS                        │  │
│  │                                                                │  │
│  │  Voting (weighted by score):                                   │  │
│  │  • Token bucket with sliding window hybrid: 3 votes (78%)      │  │
│  │  • Pure sliding window: 1 vote (12%)                           │  │
│  │  • Leaky bucket: 1 vote (10%)                                  │  │
│  │                                                                │  │
│  │  CONSENSUS: Token bucket + sliding window hybrid approach      │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                │                                     │
│                                ▼                                     │
│  FINAL OUTPUT: Comprehensive rate limiter implementation guide      │
│  with token bucket base, sliding window for smoothing, and          │
│  Redis backend for distributed deployments.                         │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Score-Weighted Voting

In AI Debate, model votes are weighted by their verification scores:

```go
// Vote weight calculation
func calculateVoteWeight(model *VerifiedModel) float64 {
    // Normalize score to 0-1 range
    normalizedScore := model.OverallScore / 10.0

    // Apply code visibility bonus
    if model.CodeVisible {
        normalizedScore *= 1.1
    }

    return normalizedScore
}
```

### Automatic Failover During Debate

If a model fails during debate, LLMsVerifier's health monitoring kicks in:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    FAILOVER DURING AI DEBATE                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  SCENARIO: GPT-4o becomes unavailable during Round 2                 │
│                                                                      │
│  1. Circuit breaker opens for GPT-4o                                │
│  2. LLMsVerifier selects next best model: Mistral Large (SC:8.4)    │
│  3. Debate continues with replacement model                          │
│  4. No user intervention required                                    │
│                                                                      │
│  BEFORE:  GPT-4o ◄──► Claude 3.5 ◄──► Gemini 1.5                    │
│                         ▼                                            │
│  AFTER:   Mistral Large ◄──► Claude 3.5 ◄──► Gemini 1.5             │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Configuration

### Minimal Configuration (API Keys Only)

```bash
# .env - Simplest possible setup
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
```

```yaml
# configs/verifier.yaml
verifier:
  enabled: true
  challenges:
    provider_discovery: true
    model_verification: true

providers:
  openai:
    enabled: true
    api_key: "${OPENAI_API_KEY}"
  anthropic:
    enabled: true
    api_key: "${ANTHROPIC_API_KEY}"
```

### With Custom Base URLs

```yaml
# For enterprise/proxy setups
providers:
  openai_enterprise:
    enabled: true
    api_key: "${OPENAI_ENTERPRISE_KEY}"
    base_url: "https://openai-proxy.mycompany.com/v1"

  azure_openai:
    enabled: true
    api_key: "${AZURE_OPENAI_KEY}"
    base_url: "https://mycompany.openai.azure.com"
```

### Full Configuration

```yaml
verifier:
  enabled: true

  verification:
    mandatory_code_check: true
    verification_timeout: 60s
    tests:
      - code_visibility
      - existence
      - responsiveness
      - latency
      - streaming
      - function_calling
      - coding_capability
      - error_detection

  scoring:
    weights:
      response_speed: 0.25
      model_efficiency: 0.20
      cost_effectiveness: 0.25
      capability: 0.20
      recency: 0.10
    models_dev_enabled: true
    cache_ttl: 24h

  challenges:
    enabled: true
    provider_discovery: true
    model_verification: true
    config_generation: true

  model_selection:
    max_models: 5
    min_score: 7.0
    require_verification: true
    require_code_visibility: true
    require_diversity: true

  health:
    check_interval: 30s
    circuit_breaker:
      enabled: true
      half_open_timeout: 60s

  scheduling:
    re_verification:
      enabled: true
      interval: 24h
    score_recalculation:
      enabled: true
      interval: 12h

providers:
  openai:
    enabled: true
    api_key: "${OPENAI_API_KEY}"
  anthropic:
    enabled: true
    api_key: "${ANTHROPIC_API_KEY}"
  google:
    enabled: true
    api_key: "${GOOGLE_API_KEY}"
  groq:
    enabled: true
    api_key: "${GROQ_API_KEY}"
```

---

## Power Features Summary

### What LLMsVerifier Provides

| Feature | Description | Benefit |
|---------|-------------|---------|
| **Automatic Discovery** | Finds all models from API keys | No manual model configuration |
| **Code Visibility Test** | "Do you see my code?" verification | Ensures context awareness |
| **8 Verification Tests** | Comprehensive model testing | Reliability assurance |
| **5-Component Scoring** | Weighted performance scoring | Objective model ranking |
| **Score Suffix (SC:X.X)** | Inline score display | Quick quality reference |
| **Auto Model Selection** | Top N model selection | Best models for ensemble |
| **Circuit Breakers** | Automatic failure handling | Resilient operation |
| **Automatic Failover** | Seamless provider switching | Zero downtime |
| **Health Monitoring** | Real-time provider health | Proactive issue detection |
| **Re-verification** | Scheduled model re-testing | Up-to-date reliability data |
| **models.dev Integration** | Enhanced scoring data | Community benchmarks |
| **12+ Providers** | Broad provider support | Maximum flexibility |

### The Complete Flow

```
USER PROVIDES:                    LLMSVERIFIER DOES:               SUPERAGENT USES:
┌──────────────┐                  ┌─────────────────┐              ┌─────────────────┐
│ API Keys     │  ──────────────▶ │ Discovery       │ ──────────▶ │ AI Debate with  │
│ (optionally  │                  │ Verification    │              │ Top 5 Verified  │
│ Base URLs)   │                  │ Scoring         │              │ Highest-Scoring │
│              │                  │ Selection       │              │ Models          │
└──────────────┘                  └─────────────────┘              └─────────────────┘
     │                                    │                               │
     │                                    │                               │
     ▼                                    ▼                               ▼
  MINIMAL                            AUTOMATIC                      INTELLIGENT
  CONFIGURATION                      PROCESSING                     ENSEMBLE
```

### Why This Matters

1. **Zero Configuration Burden**: Users don't need to know model names or capabilities
2. **Always Best Models**: Automatic selection ensures optimal performance
3. **Self-Healing**: Circuit breakers and failover handle issues automatically
4. **Future-Proof**: New models are automatically discovered and tested
5. **Quality Assured**: Only verified, code-aware models are used
6. **Objective Ranking**: Scores remove guesswork from model selection

---

## Getting Started

### Quickest Start

1. Set environment variables:
   ```bash
   export OPENAI_API_KEY="sk-..."
   export ANTHROPIC_API_KEY="sk-ant-..."
   ```

2. Enable verifier:
   ```bash
   make verifier-run
   ```

3. SuperAgent automatically discovers, verifies, scores, and selects the best models for AI debate!

### Verify It's Working

```bash
# Check discovered models
curl http://localhost:8081/api/v1/verifier/models

# Check top scoring models
curl http://localhost:8081/api/v1/verifier/scores/top?limit=10

# Check provider health
curl http://localhost:8081/api/v1/verifier/health/providers
```

---

## Conclusion

LLMsVerifier transforms SuperAgent from a manually-configured LLM ensemble into an **intelligent, self-optimizing AI system** that:

- Automatically finds the best available models
- Verifies they work correctly (especially code visibility)
- Scores them objectively
- Selects the top performers
- Uses them for high-quality AI debate

**Users just provide API keys. LLMsVerifier handles everything else.**
