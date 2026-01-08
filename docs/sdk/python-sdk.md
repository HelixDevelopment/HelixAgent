# HelixAgent Python SDK

> **Status: Available**
>
> The Python SDK is implemented and available at `/sdk/python/`.
> Install from source or wait for PyPI publication.

A comprehensive Python SDK for interacting with the HelixAgent AI orchestration platform, providing easy access to multi-provider LLM capabilities and OpenAI-compatible API.

## Installation

```bash
# Install from source
cd sdk/python
pip install -e .

# Or when published to PyPI:
# pip install helixagent-sdk
```

## Quick Start

```python
from helixagent import HelixAgent

# Initialize client
client = HelixAgent(
    api_key="your-api-key",
    base_url="https://api.helixagent.ai"
)

# Simple chat completion
response = client.chat.completions.create(
    model="helixagent-ensemble",
    messages=[
        {"role": "user", "content": "Explain quantum computing"}
    ]
)

print(response.choices[0].message.content)
```

## Authentication

The SDK supports multiple authentication methods:

```python
# API Key authentication
client = HelixAgent(api_key="your-api-key")

# JWT Token authentication
client = HelixAgent(token="your-jwt-token")

# Custom base URL
client = HelixAgent(
    api_key="your-api-key",
    base_url="http://localhost:7061"
)
```

## Chat Completions

### Basic Chat Completion

```python
from helixagent import HelixAgent

client = HelixAgent(api_key="your-api-key")

response = client.chat.completions.create(
    model="helixagent-ensemble",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "What is machine learning?"}
    ],
    max_tokens=500,
    temperature=0.7
)

print(response.choices[0].message.content)
print(f"Usage: {response.usage.total_tokens} tokens")
```

### Streaming Chat Completion

```python
response = client.chat.completions.create(
    model="deepseek-chat",
    messages=[{"role": "user", "content": "Tell me a story"}],
    stream=True
)

for chunk in response:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### Ensemble Completion

```python
# Advanced ensemble with custom configuration
response = client.ensemble.completions.create(
    messages=[{"role": "user", "content": "What is the future of AI?"}],
    ensemble_config={
        "strategy": "confidence_weighted",
        "min_providers": 3,
        "confidence_threshold": 0.8,
        "fallback_to_best": True
    }
)

print(f"Ensemble result: {response.choices[0].message.content}")
print(f"Providers used: {response.ensemble.providers_used}")
print(f"Confidence: {response.ensemble.confidence_score}")
```

## Text Completions

### Basic Text Completion

```python
response = client.completions.create(
    model="qwen-max",
    prompt="The future of technology is",
    max_tokens=100,
    temperature=0.8,
    stop=["\n", "."]
)

print(response.choices[0].text)
```

### Streaming Text Completion

```python
response = client.completions.create(
    model="openrouter/grok-4",
    prompt="Write a haiku about programming:",
    stream=True
)

for chunk in response:
    print(chunk.choices[0].text, end="")
```

## AI Debate System

### Creating a Basic Debate

```python
debate_config = {
    "debateId": "ai-ethics-debate-001",
    "topic": "Should AI systems have ethical constraints built into their core architecture?",
    "maximal_repeat_rounds": 3,
    "consensus_threshold": 0.75,
    "participants": [
        {
            "name": "EthicsExpert",
            "role": "AI Ethics Specialist",
            "llms": [{
                "provider": "claude",
                "model": "claude-3-5-sonnet-20241022"
            }]
        },
        {
            "name": "AIScientist",
            "role": "AI Research Scientist",
            "llms": [{
                "provider": "deepseek",
                "model": "deepseek-coder"
            }]
        }
    ]
}

debate = client.debates.create(debate_config)
print(f"Debate created: {debate.debateId}")
```

### Monitoring Debate Progress

```python
# Get debate status
status = client.debates.get_status("ai-ethics-debate-001")
print(f"Status: {status.status}")
print(f"Progress: Round {status.current_round}/{status.total_rounds}")

# Wait for completion
while status.status not in ["completed", "failed"]:
    time.sleep(5)
    status = client.debates.get_status("ai-ethics-debate-001")
```

### Getting Debate Results

```python
results = client.debates.get_results("ai-ethics-debate-001")

print(f"Topic: {results.topic}")
print(f"Consensus achieved: {results.consensus.achieved}")
print(f"Final position: {results.consensus.final_position}")

for participant in results.participants:
    print(f"{participant.name}: {participant.total_responses} responses, "
          f"avg quality: {participant.avg_quality_score}")
```

### Advanced Debate with Cognee Enhancement

```python
debate_config = {
    "debateId": "enhanced-debate-001",
    "topic": "How should society regulate artificial general intelligence?",
    "enable_cognee": True,
    "cognee_config": {
        "dataset_name": "agi_regulation_debate",
        "enhancement_strategy": "hybrid",
        "max_enhancement_time": 30000
    },
    "participants": [
        {
            "name": "PolicyMaker",
            "role": "Government Policy Advisor",
            "enable_cognee": True,
            "cognee_settings": {
                "enhance_responses": True,
                "analyze_sentiment": True,
                "dataset_name": "policy_debate_data"
            }
        },
        {
            "name": "AIRiskExpert",
            "role": "AI Safety Researcher",
            "enable_cognee": True
        }
    ]
}

debate = client.debates.create(debate_config)
```

## Model Context Protocol (MCP)

### Getting MCP Capabilities

```python
capabilities = client.mcp.capabilities()
print(f"MCP Version: {capabilities.version}")
print(f"Available providers: {capabilities.providers}")
```

### Listing MCP Tools

```python
tools = client.mcp.tools()
for tool in tools.tools:
    print(f"Tool: {tool.name} - {tool.description}")
```

### Executing MCP Tools

```python
result = client.mcp.tools.call(
    name="read_file",
    arguments={"path": "/etc/hostname"}
)
print(f"Result: {result.result}")
```

### MCP Prompts

```python
prompts = client.mcp.prompts()
for prompt in prompts.prompts:
    print(f"Prompt: {prompt.name} - {prompt.description}")
```

### MCP Resources

```python
resources = client.mcp.resources()
for resource in resources.resources:
    print(f"Resource: {resource.name} - {resource.description}")
```

## Provider Management

### Listing Available Providers

```python
providers = client.providers.list()
for provider in providers.providers:
    print(f"{provider.name}: {provider.status} - {len(provider.models)} models")
```

### Provider Health Check

```python
health = client.providers.health()
print(f"Overall status: {health.status}")
for name, status in health.providers.items():
    print(f"{name}: {status.status} (response time: {status.response_time}s)")
```

## Error Handling

The SDK provides comprehensive error handling:

```python
from helixagent.exceptions import (
    AuthenticationError,
    RateLimitError,
    ProviderError,
    ValidationError
)

try:
    response = client.chat.completions.create(
        model="invalid-model",
        messages=[{"role": "user", "content": "Hello"}]
    )
except ValidationError as e:
    print(f"Validation error: {e}")
except ProviderError as e:
    print(f"Provider error: {e}")
except RateLimitError as e:
    print(f"Rate limit exceeded: {e}")
except AuthenticationError as e:
    print(f"Authentication failed: {e}")
```

## Advanced Configuration

### Custom HTTP Client

```python
import requests
from helixagent.client import HelixAgentClient

# Custom session with proxy
session = requests.Session()
session.proxies = {"https": "https://proxy.company.com:7061"}

client = HelixAgentClient(
    api_key="your-api-key",
    session=session
)
```

### Timeout Configuration

```python
client = HelixAgent(
    api_key="your-api-key",
    timeout=30.0,  # 30 seconds
    max_retries=3
)
```

### Logging

```python
import logging

logging.basicConfig(level=logging.DEBUG)

# SDK will use the standard Python logging
client = HelixAgent(api_key="your-api-key")
```

## Best Practices

### 1. Error Handling

Always implement proper error handling:

```python
def safe_completion(model, messages):
    try:
        response = client.chat.completions.create(
            model=model,
            messages=messages,
            max_tokens=1000
        )
        return response.choices[0].message.content
    except RateLimitError:
        time.sleep(60)  # Wait before retry
        return safe_completion(model, messages)
    except ProviderError as e:
        # Fallback to different provider
        if model.startswith("claude"):
            return safe_completion("deepseek-chat", messages)
        raise e
```

### 2. Resource Management

Use context managers for long-running operations:

```python
with client as session:
    # All operations within this block use the same session
    response1 = session.chat.completions.create(...)
    response2 = session.debates.create(...)
```

### 3. Batch Operations

For multiple requests, consider batching:

```python
# Instead of multiple individual requests
responses = []
for prompt in prompts:
    response = client.completions.create(
        model="helixagent-ensemble",
        prompt=prompt
    )
    responses.append(response)

# Consider using async version if available
import asyncio

async def batch_completions(prompts):
    tasks = [
        client.completions.acreate(
            model="helixagent-ensemble",
            prompt=prompt
        )
        for prompt in prompts
    ]
    return await asyncio.gather(*tasks)
```

### 4. Monitoring and Metrics

```python
# Enable detailed logging
client.enable_logging()

# Get usage statistics
usage = client.get_usage_stats()
print(f"Total requests: {usage.total_requests}")
print(f"Total tokens: {usage.total_tokens}")
print(f"Success rate: {usage.success_rate}%")
```

## API Reference

### Classes

- `HelixAgent`: Main client class
- `ChatCompletions`: Chat completion operations
- `Completions`: Text completion operations
- `Ensemble`: Ensemble operations
- `Debates`: AI debate operations
- `MCP`: Model Context Protocol operations
- `Providers`: Provider management

### Exceptions

- `HelixAgentError`: Base exception
- `AuthenticationError`: Authentication failures
- `RateLimitError`: Rate limit exceeded
- `ProviderError`: Provider-specific errors
- `ValidationError`: Input validation errors
- `NetworkError`: Network connectivity issues

## Requirements

- Python 3.8+
- requests
- pydantic
- aiohttp (for async operations)

## License

MIT License - see LICENSE file for details.