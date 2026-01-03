# LLMsVerifier Integration

SuperAgent integrates [LLMsVerifier](https://github.com/vasic-digital/LLMsVerifier) as a Git submodule to provide comprehensive LLM verification, scoring, and health monitoring capabilities.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [SDKs](#sdks)
- [Monitoring](#monitoring)
- [Testing](#testing)

## Overview

The LLMsVerifier integration adds the following capabilities to SuperAgent:

1. **Model Verification** - Verify LLM models work correctly, including the unique "Do you see my code?" test
2. **Comprehensive Scoring** - 5-component weighted scoring system
3. **Health Monitoring** - Real-time provider health with circuit breakers
4. **Automatic Failover** - Intelligent failover between providers
5. **Extended Provider Support** - 12+ LLM providers

## Features

### Code Visibility Test ("Do you see my code?")

A unique verification test that injects code into prompts and asks the model "Do you see my code?" to verify the model can actually see context:

```go
result, err := verifier.TestCodeVisibility(ctx, "gpt-4", "openai", "python")
if result.CodeVisible {
    fmt.Println("Model can see injected code!")
}
```

### 5-Component Scoring System

Models are scored on 5 weighted components:

| Component | Weight | Description |
|-----------|--------|-------------|
| Response Speed | 25% | How fast the model responds |
| Model Efficiency | 20% | Token efficiency and output quality |
| Cost Effectiveness | 25% | Price per quality ratio |
| Capability | 20% | Overall model capabilities |
| Recency | 10% | How recently updated |

### Supported Providers

- OpenAI
- Anthropic
- Google (Gemini)
- Groq
- Together AI
- Mistral
- DeepSeek
- xAI (Grok)
- Cerebras
- Cloudflare Workers AI
- SiliconFlow
- Replicate
- Ollama (local)
- OpenRouter

## Quick Start

### 1. Initialize Submodule

```bash
make verifier-init
```

### 2. Configure

Create or edit `configs/verifier.yaml`:

```yaml
verifier:
  enabled: true
  verification:
    mandatory_code_check: true
  scoring:
    weights:
      response_speed: 0.25
      model_efficiency: 0.20
      cost_effectiveness: 0.25
      capability: 0.20
      recency: 0.10
```

### 3. Run

```bash
make verifier-run
```

### 4. Verify a Model

```bash
curl -X POST http://localhost:8081/api/v1/verifier/verify \
  -H "Content-Type: application/json" \
  -d '{"model_id": "gpt-4", "provider": "openai"}'
```

## Configuration

See [configs/verifier.yaml](../../configs/verifier.yaml) for full configuration options.

### Key Configuration Sections

#### Verification Settings
```yaml
verification:
  mandatory_code_check: true
  code_visibility_prompt: "Do you see my code?"
  verification_timeout: 60s
  retry_count: 3
  tests:
    - code_visibility
    - existence
    - responsiveness
    - latency
    - streaming
```

#### Scoring Weights
```yaml
scoring:
  weights:
    response_speed: 0.25
    model_efficiency: 0.20
    cost_effectiveness: 0.25
    capability: 0.20
    recency: 0.10
  models_dev_enabled: true
  cache_ttl: 24h
```

#### Health Monitoring
```yaml
health:
  check_interval: 30s
  timeout: 10s
  failure_threshold: 5
  circuit_breaker:
    enabled: true
    half_open_timeout: 60s
```

## API Reference

### Verification Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/verifier/verify` | Verify a model |
| POST | `/api/v1/verifier/verify/batch` | Batch verify models |
| GET | `/api/v1/verifier/status/{model_id}` | Get verification status |
| POST | `/api/v1/verifier/test/code-visibility` | Test code visibility |
| GET | `/api/v1/verifier/tests` | List available tests |
| POST | `/api/v1/verifier/reverify` | Re-verify a model |

### Scoring Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/verifier/scores/{model_id}` | Get model score |
| POST | `/api/v1/verifier/scores/batch` | Batch calculate scores |
| GET | `/api/v1/verifier/scores/top` | Get top models |
| GET | `/api/v1/verifier/scores/range` | Get models by score range |
| GET | `/api/v1/verifier/scores/weights` | Get scoring weights |
| PUT | `/api/v1/verifier/scores/weights` | Update scoring weights |
| POST | `/api/v1/verifier/scores/compare` | Compare models |

### Health Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/verifier/health` | Service health |
| GET | `/api/v1/verifier/health/providers` | All providers health |
| GET | `/api/v1/verifier/health/providers/{id}` | Provider health |
| POST | `/api/v1/verifier/health/fastest` | Get fastest provider |
| GET | `/api/v1/verifier/health/healthy` | List healthy providers |

## SDKs

### Go SDK

```go
import "github.com/superagent/superagent/pkg/sdk/go/verifier"

client := verifier.NewClient(verifier.ClientConfig{
    BaseURL: "http://localhost:8081",
    APIKey:  "your-api-key",
})

result, err := client.VerifyModel(ctx, verifier.VerificationRequest{
    ModelID:  "gpt-4",
    Provider: "openai",
})
```

### Python SDK

```python
from superagent_verifier import VerifierClient

client = VerifierClient(
    base_url="http://localhost:8081",
    api_key="your-api-key"
)

result = client.verify_model("gpt-4", "openai")
print(f"Verified: {result.verified}, Score: {result.overall_score}")
```

## Monitoring

### Prometheus Metrics

Metrics available at `/metrics/verifier`:

- `superagent_verifier_verifications_total` - Total verifications
- `superagent_verifier_verification_duration_seconds` - Verification duration
- `superagent_verifier_code_visibility_tests_total` - Code visibility tests
- `superagent_verifier_model_score` - Model scores
- `superagent_verifier_provider_healthy` - Provider health status
- `superagent_verifier_circuit_breaker_state` - Circuit breaker states
- `superagent_verifier_provider_latency_seconds` - Provider latency
- `superagent_verifier_failover_attempts_total` - Failover attempts

### Grafana Dashboard

Import the dashboard from `dashboards/verifier/verifier-dashboard.json`.

## Testing

### Run All Verifier Tests

```bash
make verifier-test-all
```

### Individual Test Types

```bash
make verifier-test-unit          # Unit tests
make verifier-test-integration   # Integration tests
make verifier-test-e2e           # End-to-end tests
make verifier-test-security      # Security tests
make verifier-test-stress        # Stress tests
make verifier-test-chaos         # Chaos tests
```

### Coverage

```bash
make verifier-test-coverage      # Generate coverage report
make verifier-test-coverage-100  # Require 100% coverage
```

## Architecture

```
internal/verifier/
├── adapters/
│   ├── provider_adapter.go    # LLMsVerifier provider adapters
│   └── extended_registry.go   # Extended provider registry
├── config.go                   # Configuration
├── database.go                 # Database bridge
├── health.go                   # Health monitoring
├── metrics.go                  # Prometheus metrics
├── scoring.go                  # Scoring engine
└── service.go                  # Verification service

internal/handlers/
├── verification_handler.go     # Verification HTTP handlers
├── scoring_handler.go          # Scoring HTTP handlers
└── health_handler.go           # Health HTTP handlers

pkg/sdk/
├── go/verifier/                # Go SDK
├── python/superagent_verifier/ # Python SDK
└── javascript/                 # JavaScript SDK
```

## License

MIT License - See LICENSE file for details.
