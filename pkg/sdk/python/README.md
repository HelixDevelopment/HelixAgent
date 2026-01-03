# SuperAgent Verifier Python SDK

Python SDK for interacting with the SuperAgent LLMsVerifier API. This SDK provides model verification, scoring, and health monitoring capabilities.

## Installation

```bash
pip install superagent-verifier
```

Or install from source:

```bash
cd pkg/sdk/python
pip install -e .
```

## Quick Start

```python
from superagent_verifier import VerifierClient

# Initialize the client
client = VerifierClient(
    base_url="http://localhost:8081",
    api_key="your-api-key"  # Optional
)

# Verify a model
result = client.verify_model("gpt-4", "openai")
print(f"Verified: {result.verified}")
print(f"Score: {result.overall_score}")
print(f"Code Visible: {result.code_visible}")

# Get model score
score = client.get_model_score("gpt-4")
print(f"Overall: {score.overall_score}")
print(f"Speed: {score.components.speed_score}")
print(f"Efficiency: {score.components.efficiency_score}")

# Get top models
top_models = client.get_top_models(limit=10)
for model in top_models:
    print(f"{model.rank}. {model.name}: {model.overall_score}")
```

## Features

### Model Verification

```python
# Single model verification
result = client.verify_model("gpt-4", "openai")

# Batch verification
results = client.batch_verify([
    {"model_id": "gpt-4", "provider": "openai"},
    {"model_id": "claude-3-5-sonnet", "provider": "anthropic"},
])

# Test code visibility ("Do you see my code?")
visibility = client.test_code_visibility("gpt-4", "openai", language="python")
print(f"Code visible: {visibility.code_visible}")

# Re-verify a model
result = client.reverify_model("gpt-4", "openai", force=True)
```

### Scoring

```python
# Get model score
score = client.get_model_score("gpt-4")

# Get top models
top = client.get_top_models(limit=10)

# Get models by score range
models = client.get_models_by_score_range(min_score=8.0, max_score=10.0)

# Compare models
comparison = client.compare_models(["gpt-4", "claude-3-5-sonnet", "gemini-1.5-pro"])
print(f"Winner: {comparison['winner']}")

# Get model name with score suffix
name = client.get_model_name_with_score("gpt-4")
# Returns: "GPT-4 (SC:9.2)"

# Update scoring weights
from superagent_verifier import ScoringWeights

weights = ScoringWeights(
    response_speed=0.30,
    model_efficiency=0.20,
    cost_effectiveness=0.20,
    capability=0.20,
    recency=0.10,
)
client.update_scoring_weights(weights)
```

### Health Monitoring

```python
# Get provider health
health = client.get_provider_health("openai")
print(f"Healthy: {health.healthy}")
print(f"Uptime: {health.uptime_percent}%")

# Get all providers health
providers = client.get_all_providers_health()

# Get healthy providers
healthy = client.get_healthy_providers()

# Get fastest provider
fastest = client.get_fastest_provider(["openai", "anthropic", "google"])
print(f"Fastest: {fastest['provider_id']}")

# Check provider availability
available = client.is_provider_available("openai")
```

## Error Handling

```python
from superagent_verifier import (
    VerifierError,
    APIError,
    ValidationError,
    AuthenticationError,
    NotFoundError,
)

try:
    result = client.verify_model("unknown-model", "unknown-provider")
except NotFoundError:
    print("Model or provider not found")
except AuthenticationError:
    print("Authentication failed")
except ValidationError as e:
    print(f"Validation error: {e}")
except APIError as e:
    print(f"API error (status {e.status_code}): {e}")
except VerifierError as e:
    print(f"General error: {e}")
```

## Configuration

```python
client = VerifierClient(
    base_url="http://localhost:8081",  # API base URL
    api_key="your-api-key",            # Optional API key
    timeout=30,                         # Request timeout in seconds
)
```

## Development

```bash
# Install dev dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Run with coverage
pytest --cov=superagent_verifier

# Type checking
mypy superagent_verifier

# Format code
black superagent_verifier
```

## License

MIT License
