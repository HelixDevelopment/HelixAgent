# LLMsVerifier Integration Guide

## Overview

LLMsVerifier is HelixAgent's verification and scoring system for LLM providers. It provides:

- **Provider Verification**: 8-test verification pipeline
- **Dynamic Scoring**: 5-component weighted scoring
- **Provider Discovery**: Automatic detection of available providers
- **Debate Team Selection**: Score-based team composition

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    HelixAgent Startup                        │
├─────────────────────────────────────────────────────────────┤
│  1. Load Config & Environment                                │
│  2. Initialize StartupVerifier (Scoring + Verification)     │
│  3. Discover ALL Providers (API Key + OAuth + Free)         │
│  4. Verify ALL Providers in Parallel (8-test pipeline)      │
│  5. Score ALL Verified Providers (5-component weighted)     │
│  6. Rank by Score (OAuth priority when scores close)        │
│  7. Select AI Debate Team (15 LLMs: 5 primary + 10 fallback)│
│  8. Start Server with Verified Configuration                 │
└─────────────────────────────────────────────────────────────┘
```

## Verification Pipeline

### 8-Test Verification Suite

| Test | Description | Weight |
|------|-------------|--------|
| 1. Health Check | Basic API connectivity | Required |
| 2. Model List | Can list available models | Required |
| 3. Simple Completion | Basic chat completion | Required |
| 4. Streaming | SSE streaming support | Optional |
| 5. Tool Calling | Function/tool support | Optional |
| 6. Context Length | Token limit validation | Optional |
| 7. Response Time | Latency measurement | Scoring |
| 8. Error Handling | Graceful error responses | Scoring |

### Verification Scores

- **Fully Verified**: Passes all 8 tests (score 8.0-10.0)
- **Partially Verified**: Passes required tests (score 6.0-8.0)
- **Minimally Verified**: Passes health check only (score 5.0-6.0)
- **Failed**: Does not pass health check (excluded)

## Scoring Algorithm

### 5-Component Weighted Scoring

| Component | Weight | Description |
|-----------|--------|-------------|
| Response Speed | 25% | API response latency |
| Model Efficiency | 20% | Token efficiency |
| Cost Effectiveness | 25% | Cost per token |
| Capability | 20% | Model capability score |
| Recency | 10% | Model release date |

### Score Calculation

```go
func CalculateScore(provider *Provider) float64 {
    return (
        provider.ResponseSpeed    * 0.25 +
        provider.ModelEfficiency  * 0.20 +
        provider.CostEffectiveness * 0.25 +
        provider.Capability       * 0.20 +
        provider.Recency          * 0.10
    )
}
```

### Bonuses

- **OAuth Verified**: +0.5 bonus when verified
- **Free Providers**: Score capped at 6.0-7.0

## Provider Types

### API Key Providers

```bash
# Standard API key configuration
export DEEPSEEK_API_KEY=your-key
export GEMINI_API_KEY=your-key
export OPENROUTER_API_KEY=your-key
export MISTRAL_API_KEY=your-key
export ZAI_API_KEY=your-key
export CEREBRAS_API_KEY=your-key
```

### OAuth Providers

```bash
# Enable OAuth from CLI tools
export CLAUDE_CODE_USE_OAUTH_CREDENTIALS=true
export QWEN_CODE_USE_OAUTH_CREDENTIALS=true
```

**Important**: OAuth tokens from CLI tools are product-restricted:
- Claude OAuth: Only works with Claude Code
- Qwen OAuth: Only works with Qwen Portal

**Solution**: Use standard API keys for general API access.

### Free Providers

```bash
# Free providers (no API key needed)
export ZEN_ENABLED=true  # OpenCode Zen
# OpenRouter :free models auto-discovered
```

## CLI Usage

### Verify All Providers

```bash
./helixagent verify --all
```

Output:
```
Verifying 10 providers...

Provider         Score   Status      Tests
─────────────────────────────────────────────
Claude           9.2     verified    8/8
DeepSeek         8.8     verified    8/8
Gemini           8.5     verified    8/8
OpenRouter       8.3     verified    8/8
Mistral          7.9     verified    7/8
Qwen             7.5     verified    6/8
ZAI              7.2     verified    6/8
Cerebras         6.8     verified    5/8
Zen              6.5     partial     4/8
Ollama           5.0     deprecated  3/8

Total: 9 verified, 1 deprecated
```

### Verify Specific Provider

```bash
./helixagent verify --provider deepseek
```

### View Scores

```bash
./helixagent scores
```

### Refresh Verification

```bash
./helixagent verify --refresh
```

## API Endpoints

### Get Verification Status

```bash
curl http://localhost:8080/v1/providers/verification
```

Response:
```json
{
  "total_providers": 10,
  "verified_providers": 9,
  "last_verified": "2026-01-16T10:00:00Z",
  "providers": [
    {
      "provider_id": "deepseek",
      "score": 8.8,
      "status": "verified",
      "tests_passed": 8,
      "components": {
        "response_speed": 9.2,
        "model_efficiency": 8.5,
        "cost_effectiveness": 9.0,
        "capability": 8.8,
        "recency": 8.0
      }
    }
  ]
}
```

### Trigger Verification

```bash
curl -X POST http://localhost:8080/v1/providers/verify
```

### Get Best Providers

```bash
curl http://localhost:8080/v1/providers/best?count=5
```

## Integration with Debate Team

LLMsVerifier dynamically selects the AI Debate team:

```
Debate Team Selection (15 LLMs)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Position 1 (Lead Pro)
  Primary:  Claude Opus (9.2)
  Fallback: DeepSeek (8.8)
  Fallback: Gemini (8.5)

Position 2 (Lead Con)
  Primary:  Qwen (7.5)
  Fallback: OpenRouter (8.3)
  Fallback: Mistral (7.9)

Position 3 (Analyst)
  Primary:  DeepSeek (8.8)
  Fallback: Gemini (8.5)
  Fallback: ZAI (7.2)

Position 4 (Synthesizer)
  Primary:  Gemini (8.5)
  Fallback: OpenRouter (8.3)
  Fallback: Cerebras (6.8)

Position 5 (Moderator)
  Primary:  OpenRouter (8.3)
  Fallback: Mistral (7.9)
  Fallback: Zen (6.5)
```

## Configuration

### Environment Variables

```bash
# Verification settings
export VERIFIER_ENABLED=true
export VERIFIER_INTERVAL=1h          # Re-verify interval
export VERIFIER_TIMEOUT=30s          # Per-test timeout
export VERIFIER_PARALLEL=true        # Parallel verification
export VERIFIER_MIN_SCORE=5.0        # Minimum score to include

# Scoring weights (customize)
export SCORE_WEIGHT_SPEED=0.25
export SCORE_WEIGHT_EFFICIENCY=0.20
export SCORE_WEIGHT_COST=0.25
export SCORE_WEIGHT_CAPABILITY=0.20
export SCORE_WEIGHT_RECENCY=0.10
```

### Configuration File

```yaml
# configs/verifier.yaml
verifier:
  enabled: true
  interval: 1h
  timeout: 30s
  parallel: true
  min_score: 5.0

scoring:
  weights:
    response_speed: 0.25
    model_efficiency: 0.20
    cost_effectiveness: 0.25
    capability: 0.20
    recency: 0.10
```

## Troubleshooting

### Provider Not Verified

1. Check API key is set:
   ```bash
   env | grep API_KEY
   ```

2. Check connectivity:
   ```bash
   curl https://api.deepseek.com/v1/models \
     -H "Authorization: Bearer $DEEPSEEK_API_KEY"
   ```

3. Review verification logs:
   ```bash
   ./helixagent verify --provider deepseek --verbose
   ```

### Low Scores

- Check response latency (speed component)
- Verify model capabilities (capability component)
- Review cost settings (cost component)

### OAuth Failures

OAuth tokens are product-restricted. Use standard API keys:
- Claude: https://console.anthropic.com/
- Qwen: https://dashscope.aliyuncs.com/

## Related Documentation

- [Provider Configuration](/docs/guides/configuration-guide.md)
- [AI Debate System](/docs/ai-debate.html)
- [Startup Verification](/docs/architecture/startup-verification.md)
