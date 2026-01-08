# HelixAgent Common Use Cases

## 1. Multi-Provider Chat Completion

Use multiple LLM providers to get the best response:

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-ensemble",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain quantum computing in simple terms."}
    ],
    "ensemble_config": {
      "strategy": "confidence_weighted",
      "min_providers": 2,
      "preferred_providers": ["openai", "anthropic"]
    }
  }'
```

**Response includes:**
- Best response from ensemble
- Individual provider scores
- Selection reasoning

## 2. AI Debate for Decision Making

Have AI models debate a topic to explore all perspectives:

```bash
# Start debate
curl -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "Should autonomous vehicles be fully legalized?",
    "participants": [
      {"provider": "openai", "model": "gpt-4", "stance": "pro"},
      {"provider": "anthropic", "model": "claude-3-opus", "stance": "con"}
    ],
    "max_rounds": 3,
    "timeout": 300
  }'

# Get debate results
curl http://localhost:7061/v1/debates/{debate_id}/results
```

**Results include:**
- Arguments from each side
- Consensus points
- Key disagreements
- Quality scores

## 3. Model Verification Before Use

Verify a model works correctly before using it:

```bash
# Full verification
curl -X POST http://localhost:7061/api/v1/verifier/verify \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "gpt-4",
    "provider": "openai",
    "tests": ["code_visibility", "existence", "responsiveness", "streaming"]
  }'

# Code visibility test only
curl -X POST http://localhost:7061/api/v1/verifier/code-visibility \
  -H "Content-Type: application/json" \
  -d '{
    "code": "def calculate_sum(a, b): return a + b",
    "language": "python",
    "model_id": "gpt-4",
    "provider": "openai"
  }'
```

## 4. Compare Models by Score

Find the best model for your use case:

```bash
# Get top 5 models
curl "http://localhost:7061/api/v1/verifier/scores/top?limit=5"

# Compare specific models
curl -X POST http://localhost:7061/api/v1/verifier/scores/compare \
  -H "Content-Type: application/json" \
  -d '{
    "model_ids": ["gpt-4", "claude-3-opus", "gemini-pro"]
  }'
```

## 5. Provider Health Monitoring

Check provider status before routing traffic:

```bash
# All providers health
curl http://localhost:7061/api/v1/verifier/health/providers

# Specific provider
curl http://localhost:7061/api/v1/verifier/health/providers/openai

# Get healthy providers only
curl http://localhost:7061/api/v1/verifier/health/providers/healthy

# Get fastest available provider
curl -X POST http://localhost:7061/api/v1/verifier/health/fastest \
  -H "Content-Type: application/json" \
  -d '{"providers": ["openai", "anthropic", "google"]}'
```

## 6. Streaming Responses

Get responses as they're generated:

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Write a short story"}],
    "stream": true
  }'
```

## 7. Batch Processing

Process multiple requests efficiently:

```bash
# Batch model verification
curl -X POST http://localhost:7061/api/v1/verifier/batch-verify \
  -H "Content-Type: application/json" \
  -d '{
    "models": [
      {"model_id": "gpt-4", "provider": "openai"},
      {"model_id": "claude-3-opus", "provider": "anthropic"},
      {"model_id": "gemini-pro", "provider": "google"}
    ]
  }'

# Batch scoring
curl -X POST http://localhost:7061/api/v1/verifier/scores/batch \
  -H "Content-Type: application/json" \
  -d '{"model_ids": ["gpt-4", "claude-3-opus", "gemini-pro"]}'
```

## 8. Custom Scoring Weights

Adjust scoring to prioritize what matters to you:

```bash
# View current weights
curl http://localhost:7061/api/v1/verifier/scores/weights

# Update weights (must sum to 1.0)
curl -X PUT http://localhost:7061/api/v1/verifier/scores/weights \
  -H "Content-Type: application/json" \
  -d '{
    "response_speed": 0.30,
    "model_efficiency": 0.20,
    "cost_effectiveness": 0.25,
    "capability": 0.15,
    "recency": 0.10
  }'
```

## 9. Failover Configuration

Configure automatic failover between providers:

```yaml
# In configs/production.yaml
providers:
  failover:
    enabled: true
    order:
      - openai
      - anthropic
      - google
    retry_count: 3
    fallback_model: "gpt-3.5-turbo"
```

## 10. Using with SDK

### Python Example

```python
from helixagent import HelixAgent

client = HelixAgent(
    api_key="your-key",
    base_url="http://localhost:7061"
)

# Ensemble chat
response = client.chat.create(
    model="helixagent-ensemble",
    messages=[{"role": "user", "content": "Hello!"}],
    ensemble_config={
        "strategy": "confidence_weighted",
        "min_providers": 2
    }
)

# Start debate
debate = client.debates.create(
    topic="Is remote work better than office work?",
    participants=[
        {"provider": "openai", "model": "gpt-4"},
        {"provider": "anthropic", "model": "claude-3-opus"}
    ]
)

# Wait for completion
results = client.debates.wait_for_completion(debate.debate_id)
print(f"Consensus: {results.consensus}")
```

### Go Example

```go
client := helixagent.NewClient(&helixagent.Config{
    APIKey:  "your-key",
    BaseURL: "http://localhost:7061",
})

// Ensemble chat
resp, _ := client.Chat.Completions.Create(ctx, &helixagent.ChatCompletionRequest{
    Model: "helixagent-ensemble",
    Messages: []helixagent.ChatMessage{
        {Role: "user", Content: "Hello!"},
    },
    EnsembleConfig: &helixagent.EnsembleConfig{
        Strategy:     "confidence_weighted",
        MinProviders: 2,
    },
})

// Start debate
debate, _ := client.Debates.Create(ctx, &helixagent.DebateRequest{
    Topic: "Is AI beneficial for society?",
    Participants: []helixagent.Participant{
        {Provider: "openai", Model: "gpt-4"},
        {Provider: "anthropic", Model: "claude-3-opus"},
    },
})
```

## Best Practices

1. **Always verify models** before production use
2. **Use ensemble mode** for important decisions
3. **Monitor provider health** and configure failover
4. **Set appropriate timeouts** for long-running operations
5. **Use batch operations** when processing multiple items
6. **Cache responses** for frequently asked questions
7. **Log debate results** for analysis and improvement
