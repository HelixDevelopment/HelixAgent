# SuperAgent Python SDK

A Python client for the SuperAgent AI orchestration platform. Provides OpenAI-compatible API access with support for ensemble LLM strategies.

## Installation

```bash
pip install superagent-sdk
```

Or install from source:

```bash
cd sdk/python
pip install -e .
```

## Quick Start

```python
from superagent import SuperAgent

# Initialize client
client = SuperAgent(
    api_key="your-api-key",
    base_url="http://localhost:8080"  # Or your SuperAgent instance
)

# Chat completion
response = client.chat.completions.create(
    model="superagent-ensemble",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "What is the capital of France?"}
    ]
)

print(response.choices[0].message.content)
```

## Streaming

```python
# Stream responses
for chunk in client.chat.completions.create(
    model="superagent-ensemble",
    messages=[{"role": "user", "content": "Tell me a story"}],
    stream=True
):
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
```

## Ensemble Mode

```python
from superagent import SuperAgent
from superagent.types import EnsembleConfig

client = SuperAgent(api_key="your-key")

# Configure ensemble
ensemble = EnsembleConfig(
    strategy="confidence_weighted",
    min_providers=2,
    confidence_threshold=0.8,
    preferred_providers=["openai", "anthropic"]
)

response = client.chat.completions.create(
    model="superagent-ensemble",
    messages=[{"role": "user", "content": "Complex question"}],
    ensemble_config=ensemble
)
```

## Configuration

The SDK can be configured via constructor or environment variables:

```python
# Via constructor
client = SuperAgent(
    api_key="your-key",
    base_url="http://localhost:8080",
    timeout=60,
    default_headers={"X-Custom-Header": "value"}
)

# Via environment variables
# SUPERAGENT_API_KEY=your-key
client = SuperAgent()  # Uses env vars
```

## API Reference

### Chat Completions

```python
response = client.chat.completions.create(
    model="model-name",
    messages=[...],
    temperature=0.7,
    max_tokens=1000,
    top_p=1.0,
    stop=["STOP"],
    stream=False
)
```

### Models

```python
# List models
models = client.models.list()
for model in models:
    print(f"{model.id} - {model.owned_by}")

# Get specific model
model = client.models.retrieve("gpt-4")
```

### Providers

```python
# List available providers
providers = client.providers()
```

### Health Check

```python
health = client.health()
print(health["status"])
```

## Error Handling

```python
from superagent.exceptions import (
    SuperAgentError,
    AuthenticationError,
    RateLimitError,
    APIError,
)

try:
    response = client.chat.completions.create(...)
except AuthenticationError as e:
    print(f"Auth failed: {e.message}")
except RateLimitError as e:
    print(f"Rate limited. Retry after: {e.retry_after}s")
except APIError as e:
    print(f"API error [{e.status_code}]: {e.message}")
except SuperAgentError as e:
    print(f"Error: {e}")
```

## OpenAI Compatibility

SuperAgent is fully compatible with the OpenAI API format. You can also use the official OpenAI Python client:

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-superagent-key",
    base_url="http://localhost:8080/v1"
)

response = client.chat.completions.create(
    model="superagent-ensemble",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

## Development

```bash
# Install dev dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Type checking
mypy superagent

# Formatting
black superagent tests
isort superagent tests
```

## License

MIT License
