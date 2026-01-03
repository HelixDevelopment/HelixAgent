# LLMsVerifier User Guide

This guide covers how to use the LLMsVerifier integration in SuperAgent for verifying, scoring, and monitoring LLM providers.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Model Verification](#model-verification)
3. [Code Visibility Testing](#code-visibility-testing)
4. [Scoring System](#scoring-system)
5. [Health Monitoring](#health-monitoring)
6. [Provider Management](#provider-management)
7. [Failover Configuration](#failover-configuration)
8. [Best Practices](#best-practices)
9. [Troubleshooting](#troubleshooting)

## Getting Started

### Prerequisites

- SuperAgent installed and running
- API key for at least one LLM provider
- Optional: models.dev API access for enhanced scoring

### Initial Setup

1. **Initialize the verifier submodule:**
   ```bash
   make verifier-init
   ```

2. **Configure your providers in `configs/verifier.yaml`:**
   ```yaml
   providers:
     openai:
       enabled: true
       api_key: "${OPENAI_API_KEY}"
       models:
         - gpt-4
         - gpt-4-turbo
   ```

3. **Start the verifier:**
   ```bash
   make verifier-run
   ```

4. **Check health:**
   ```bash
   make verifier-health
   ```

## Model Verification

### Single Model Verification

```bash
# Using curl
curl -X POST http://localhost:8081/api/v1/verifier/verify \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "gpt-4",
    "provider": "openai"
  }'
```

Response:
```json
{
  "model_id": "gpt-4",
  "provider": "openai",
  "verified": true,
  "score": 8.5,
  "overall_score": 8.7,
  "score_suffix": "(SC:8.7)",
  "code_visible": true,
  "tests": {
    "existence": true,
    "responsiveness": true,
    "code_visibility": true,
    "streaming": true
  },
  "verification_time_ms": 2345
}
```

### Batch Verification

Verify multiple models at once:

```bash
curl -X POST http://localhost:8081/api/v1/verifier/verify/batch \
  -H "Content-Type: application/json" \
  -d '{
    "models": [
      {"model_id": "gpt-4", "provider": "openai"},
      {"model_id": "claude-3-5-sonnet", "provider": "anthropic"},
      {"model_id": "gemini-1.5-pro", "provider": "google"}
    ]
  }'
```

### Using the Python SDK

```python
from superagent_verifier import VerifierClient

client = VerifierClient(base_url="http://localhost:8081")

# Single verification
result = client.verify_model("gpt-4", "openai")
print(f"Verified: {result.verified}")
print(f"Score: {result.overall_score}")
print(f"Code visible: {result.code_visible}")

# Batch verification
results = client.batch_verify([
    {"model_id": "gpt-4", "provider": "openai"},
    {"model_id": "claude-3", "provider": "anthropic"},
])

for r in results.results:
    print(f"{r.model_id}: {r.verified}")
```

## Code Visibility Testing

The "Do you see my code?" test verifies that LLMs can actually see code injected into prompts.

### How It Works

1. Code is injected into the prompt in a specific language (Python, Go, JavaScript, etc.)
2. The model is asked "Do you see my code?"
3. The response is analyzed to determine if the model acknowledges seeing the code

### Running the Test

```bash
curl -X POST http://localhost:8081/api/v1/verifier/test/code-visibility \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "gpt-4",
    "provider": "openai",
    "language": "python"
  }'
```

Response:
```json
{
  "model_id": "gpt-4",
  "provider": "openai",
  "code_visible": true,
  "language": "python",
  "prompt": "Here is my code:\n```python\ndef calculate_sum(a, b):\n    return a + b\n```\n\nDo you see my code?",
  "response": "Yes, I can see your Python code. It defines a function called 'calculate_sum' that takes two parameters 'a' and 'b' and returns their sum.",
  "confidence": 0.95
}
```

### Supported Languages

- Python
- Go
- JavaScript
- Java
- C#

## Scoring System

### Understanding the Score

The overall score (0-10) is calculated from 5 weighted components:

| Component | Weight | Description |
|-----------|--------|-------------|
| **Response Speed** | 25% | Model response latency |
| **Model Efficiency** | 20% | Output quality per token |
| **Cost Effectiveness** | 25% | Value for money |
| **Capability** | 20% | Overall model capabilities |
| **Recency** | 10% | How recently updated |

### Getting Model Scores

```bash
# Get score for a specific model
curl http://localhost:8081/api/v1/verifier/scores/gpt-4

# Get top 10 models
curl "http://localhost:8081/api/v1/verifier/scores/top?limit=10"

# Get models in a score range
curl "http://localhost:8081/api/v1/verifier/scores/range?min_score=8&max_score=10"
```

### Comparing Models

```bash
curl -X POST http://localhost:8081/api/v1/verifier/scores/compare \
  -H "Content-Type: application/json" \
  -d '{
    "model_ids": ["gpt-4", "claude-3-5-sonnet", "gemini-1.5-pro"]
  }'
```

### Customizing Weights

```bash
curl -X PUT http://localhost:8081/api/v1/verifier/scores/weights \
  -H "Content-Type: application/json" \
  -d '{
    "response_speed": 0.30,
    "model_efficiency": 0.20,
    "cost_effectiveness": 0.20,
    "capability": 0.20,
    "recency": 0.10
  }'
```

**Note:** Weights must sum to 1.0.

### Score Suffix

Models display a score suffix like `(SC:8.7)` after their name:
- GPT-4 (SC:9.2)
- Claude 3.5 Sonnet (SC:8.8)
- Gemini 1.5 Pro (SC:8.5)

## Health Monitoring

### Checking Provider Health

```bash
# All providers
curl http://localhost:8081/api/v1/verifier/health/providers

# Specific provider
curl http://localhost:8081/api/v1/verifier/health/providers/openai-1

# Only healthy providers
curl http://localhost:8081/api/v1/verifier/health/healthy
```

### Circuit Breaker States

Each provider has a circuit breaker with 3 states:

| State | Description |
|-------|-------------|
| **closed** | Normal operation, requests allowed |
| **half-open** | Testing if provider recovered |
| **open** | Provider failing, requests blocked |

Check circuit breaker:
```bash
curl http://localhost:8081/api/v1/verifier/health/circuit/openai-1
```

### Finding the Fastest Provider

```bash
curl -X POST http://localhost:8081/api/v1/verifier/health/fastest \
  -H "Content-Type: application/json" \
  -d '{
    "providers": ["openai-1", "anthropic-1", "google-1"]
  }'
```

## Provider Management

### Adding a Provider

```bash
curl -X POST http://localhost:8081/api/v1/verifier/health/providers \
  -H "Content-Type: application/json" \
  -d '{
    "provider_id": "openai-2",
    "provider_name": "openai"
  }'
```

### Removing a Provider

```bash
curl -X DELETE http://localhost:8081/api/v1/verifier/health/providers/openai-2
```

### Recording Success/Failure

Manually record provider outcomes:

```bash
# Success
curl -X POST http://localhost:8081/api/v1/verifier/health/record/success \
  -H "Content-Type: application/json" \
  -d '{"provider_id": "openai-1"}'

# Failure
curl -X POST http://localhost:8081/api/v1/verifier/health/record/failure \
  -H "Content-Type: application/json" \
  -d '{"provider_id": "openai-1"}'
```

## Failover Configuration

### Enable Automatic Failover

In `configs/verifier.yaml`:

```yaml
health:
  circuit_breaker:
    enabled: true
    half_open_timeout: 60s
  failure_threshold: 5

advanced:
  failover_enabled: true
  max_failover_attempts: 3
```

### Failover Priority

Configure provider priority for failover:

```yaml
providers:
  primary:
    openai:
      priority: 1
  fallback:
    anthropic:
      priority: 2
    google:
      priority: 3
```

## Best Practices

### 1. Regular Re-verification

Schedule periodic re-verification to catch provider changes:

```yaml
scheduling:
  re_verification:
    enabled: true
    interval: 24h
```

### 2. Monitor Metrics

Set up alerts for:
- Circuit breaker openings
- High latency (>5s)
- Low uptime (<99%)
- Verification failures

### 3. Cache Configuration

Optimize caching for your use case:

```yaml
scoring:
  cache_ttl: 24h  # Longer for stable models
```

### 4. Use Mandatory Code Check

Enable for security-sensitive applications:

```yaml
verification:
  mandatory_code_check: true
```

## Troubleshooting

### Model Not Verified

**Symptoms:** `verified: false` in response

**Solutions:**
1. Check provider API key is valid
2. Verify model ID is correct
3. Check provider health status
4. Review verification logs

### High Latency

**Symptoms:** Slow verification times

**Solutions:**
1. Check network connectivity
2. Verify provider isn't rate limiting
3. Reduce verification timeout
4. Use caching

### Circuit Breaker Open

**Symptoms:** Provider requests blocked

**Solutions:**
1. Check provider status page
2. Wait for half-open timeout
3. Manually record success to reset
4. Check for rate limiting

### Code Not Visible

**Symptoms:** `code_visible: false` despite working model

**Solutions:**
1. Try different programming language
2. Check prompt format
3. Verify model supports code
4. Test with longer code sample

### Cache Issues

**Symptoms:** Stale scores or verification results

**Solutions:**
```bash
# Invalidate single model
curl -X POST http://localhost:8081/api/v1/verifier/scores/cache/invalidate \
  -H "Content-Type: application/json" \
  -d '{"model_id": "gpt-4"}'

# Invalidate all
curl -X POST http://localhost:8081/api/v1/verifier/scores/cache/invalidate \
  -H "Content-Type: application/json" \
  -d '{"all": true}'
```

## Getting Help

- **Documentation:** [docs/verifier/](.)
- **Issues:** [GitHub Issues](https://github.com/superagent/superagent/issues)
- **API Reference:** [docs/verifier/API.md](API.md)
