# SuperAgent Quick Start Guide

Get started with SuperAgent in 5 minutes.

## Prerequisites

- Go 1.23+ or Docker
- API keys for at least one LLM provider (OpenAI, Anthropic, etc.)

## Option 1: Docker (Recommended)

```bash
# Clone the repository
git clone https://github.com/superagent/superagent.git
cd superagent

# Copy environment file
cp .env.example .env

# Add your API keys to .env
# OPENAI_API_KEY=your-key
# ANTHROPIC_API_KEY=your-key

# Start all services
docker-compose up -d

# Check health
curl http://localhost:8080/health
```

## Option 2: Local Development

```bash
# Clone the repository
git clone https://github.com/superagent/superagent.git
cd superagent

# Install dependencies
go mod download

# Copy environment file
cp .env.example .env
# Edit .env to add your API keys

# Run in development mode
make run-dev

# Or build and run
make build
./bin/superagent
```

## Your First API Call

### Chat Completion

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello! How are you?"}
    ]
  }'
```

### Ensemble Chat (Multiple Providers)

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "superagent-ensemble",
    "messages": [
      {"role": "user", "content": "What is the best programming language?"}
    ],
    "ensemble_config": {
      "strategy": "confidence_weighted",
      "min_providers": 2
    }
  }'
```

### AI Debate

```bash
# Start a debate
curl -X POST http://localhost:8080/v1/debates \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "Is AI beneficial for society?",
    "participants": [
      {"provider": "openai", "model": "gpt-4"},
      {"provider": "anthropic", "model": "claude-3-opus"}
    ],
    "max_rounds": 3
  }'
```

## Available Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `GET /v1/models` | List available models |
| `POST /v1/chat/completions` | Chat completion |
| `POST /v1/completions` | Text completion |
| `POST /v1/debates` | Start AI debate |
| `GET /v1/providers` | List providers |

## Configuration

Edit `configs/development.yaml` for local development or `configs/production.yaml` for production.

Key configuration options:

```yaml
server:
  port: 8080
  debug: true

providers:
  openai:
    enabled: true
    api_key: ${OPENAI_API_KEY}
  anthropic:
    enabled: true
    api_key: ${ANTHROPIC_API_KEY}

ensemble:
  default_strategy: confidence_weighted
  min_providers: 2
```

## Using SDKs

### Python SDK

```bash
pip install superagent-py
```

```python
from superagent import SuperAgent

client = SuperAgent(api_key="your-key")
response = client.chat.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

### Go SDK

```go
import "github.com/superagent/superagent-go"

client := superagent.NewClient(&superagent.Config{
    APIKey: "your-key",
})
resp, _ := client.Chat.Completions.Create(ctx, &superagent.ChatCompletionRequest{
    Model: "gpt-4",
    Messages: []superagent.ChatMessage{
        {Role: "user", Content: "Hello!"},
    },
})
fmt.Println(resp.Choices[0].Message.Content)
```

### JavaScript SDK

```javascript
import { SuperAgent } from 'superagent-js';

const client = new SuperAgent({ apiKey: 'your-key' });
const response = await client.chat.create({
  model: 'gpt-4',
  messages: [{ role: 'user', content: 'Hello!' }],
});
console.log(response.choices[0].message.content);
```

## LLMsVerifier Integration

SuperAgent includes LLMsVerifier for model verification and scoring:

```bash
# Verify a model
curl -X POST http://localhost:8080/api/v1/verifier/verify \
  -H "Content-Type: application/json" \
  -d '{"model_id": "gpt-4", "provider": "openai"}'

# Get model score
curl http://localhost:8080/api/v1/verifier/scores/gpt-4

# Check code visibility
curl -X POST http://localhost:8080/api/v1/verifier/code-visibility \
  -H "Content-Type: application/json" \
  -d '{
    "code": "def hello(): print(\"world\")",
    "language": "python",
    "model_id": "gpt-4",
    "provider": "openai"
  }'
```

## Next Steps

- [API Documentation](../api/api-documentation.md)
- [Deployment Guide](../deployment/QUICK_DEPLOYMENT_GUIDE.md)
- [LLMsVerifier Guide](../verifier/USER_GUIDE.md)
- [Advanced Features](../features/ADVANCED_FEATURES_SUMMARY.md)

## Getting Help

- GitHub Issues: https://github.com/superagent/superagent/issues
- Documentation: https://superagent.ai/docs
