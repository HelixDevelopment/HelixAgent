# HelixAgent Hello World Tutorial

This tutorial will guide you through setting up and using HelixAgent, a production-ready LLM facade system that intelligently routes requests across multiple LLM providers.

## Prerequisites

- Docker and Docker Compose
- curl (for API testing)
- Git

## Step 1: Clone and Setup HelixAgent

```bash
# Clone the repository
git clone https://github.com/vasic-digital/HelixAgent.git
cd HelixAgent

# The project is already built and ready to run
```

## Step 2: Configure API Keys

Create a `.env` file with your LLM provider API keys:

```bash
# HelixAgent Configuration
PORT=8080
HELIXAGENT_API_KEY=your-super-secret-api-key-here

# JWT Secret (generate a secure random string)
JWT_SECRET=your-secure-jwt-secret-here

# LLM Provider API Keys (get these from each provider)
CLAUDE_API_KEY=sk-ant-api03-your-claude-key-here
DEEPSEEK_API_KEY=sk-your-deepseek-key-here
GEMINI_API_KEY=your-gemini-api-key-here

# Optional: Qwen and Z.AI if you have them
QWEN_API_KEY=your-qwen-key-here
ZAI_API_KEY=your-zai-key-here

# Database (will use SQLite for this tutorial)
DB_HOST=localhost
DB_PORT=5432
DB_USER=helixagent
DB_PASSWORD=password
DB_NAME=helixagent_db

# Redis (optional for this tutorial)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
```

### Getting API Keys

1. **Claude (Anthropic)**: https://console.anthropic.com/
2. **DeepSeek**: https://platform.deepseek.com/
3. **Gemini (Google)**: https://makersuite.google.com/app/apikey

## Step 3: Start HelixAgent with Docker

```bash
# Build and start the services
docker-compose up --build -d

# Check that services are running
docker-compose ps

# View logs
docker-compose logs -f helixagent
```

## Step 4: Verify System Health

```bash
# Check if HelixAgent is running
curl http://localhost:7061/health

# Expected response:
# {"status":"healthy"}
```

## Step 5: Test Individual LLM Providers

### Test Claude

```bash
curl -X POST http://localhost:7061/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Hello, can you tell me about yourself in one sentence?",
    "model": "claude-3-sonnet-20240229",
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

### Test DeepSeek

```bash
curl -X POST http://localhost:7061/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Hello, can you tell me about yourself in one sentence?",
    "model": "deepseek-coder",
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

### Test Gemini

```bash
curl -X POST http://localhost:7061/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Hello, can you tell me about yourself in one sentence?",
    "model": "gemini-pro",
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

## Step 6: Experience Ensemble Intelligence

HelixAgent's magic happens with ensemble voting - it routes your request to multiple providers and returns the best response:

```bash
curl -X POST http://localhost:7061/v1/ensemble/completions \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Explain quantum computing in simple terms",
    "ensemble_config": {
      "strategy": "confidence_weighted",
      "min_providers": 3,
      "confidence_threshold": 0.8,
      "preferred_providers": ["claude", "deepseek", "gemini"]
    }
  }'
```

**Expected Response:**
```json
{
  "id": "ensemble-123",
  "object": "ensemble.completion",
  "created": 1677652288,
  "model": "claude-3-sonnet-20240229",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Quantum computing uses quantum bits (qubits) that can exist in multiple states simultaneously, allowing them to solve certain complex problems much faster than classical computers..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 20,
    "completion_tokens": 150,
    "total_tokens": 170
  },
  "ensemble": {
    "voting_method": "confidence_weighted",
    "responses_count": 3,
    "scores": {
      "claude": 0.92,
      "deepseek": 0.88,
      "gemini": 0.85
    },
    "selected_provider": "claude",
    "selection_score": 0.92
  }
}
```

## Step 7: Try Chat Completions

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful coding assistant."
      },
      {
        "role": "user",
        "content": "Write a simple Python function to calculate fibonacci numbers"
      }
    ],
    "temperature": 0.7,
    "max_tokens": 200
  }'
```

## Step 8: Explore Provider Information

```bash
# List all available providers
curl http://localhost:7061/v1/providers

# Check provider health
curl http://localhost:7061/v1/providers/claude/health

# Get available models
curl http://localhost:7061/v1/models
```

## Step 9: Monitor System Performance

```bash
# Check enhanced health with provider status
curl http://localhost:7061/v1/health

# View Prometheus metrics
curl http://localhost:7061/metrics
```

## Step 10: Clean Up

```bash
# Stop and remove containers
docker-compose down

# Remove volumes (optional - this will delete data)
docker-compose down -v
```

## What You Learned

1. **Multi-Provider Intelligence**: HelixAgent routes requests across Claude, DeepSeek, and Gemini
2. **Ensemble Voting**: The system intelligently selects the best response using confidence scoring
3. **OpenAI Compatibility**: Use familiar API patterns with enhanced capabilities
4. **Health Monitoring**: Built-in health checks and metrics collection
5. **Easy Deployment**: Docker-based setup for quick testing

## Next Steps

- **Add Authentication**: Set up user accounts and JWT tokens
- **Configure Memory**: Enable Cognee for context-aware responses
- **Set up Monitoring**: Configure Grafana dashboards for visualization
- **Scale Up**: Deploy with load balancing for production use
- **Add More Providers**: Integrate additional LLM providers as needed

## Troubleshooting

### Common Issues

1. **API Key Errors**: Double-check your API keys in `.env`
2. **Port Conflicts**: Ensure port 8080 is available
3. **Database Issues**: Check PostgreSQL container logs
4. **Rate Limits**: Some providers have rate limits - wait and retry

### Debug Commands

```bash
# Check container logs
docker-compose logs helixagent

# Restart services
docker-compose restart

# Rebuild from scratch
docker-compose down
docker-compose up --build --force-recreate
```

## Advanced Features to Explore

1. **Streaming**: Add `"stream": true` for real-time responses
2. **Memory Enhancement**: Add `"memory_enhanced": true` for context awareness
3. **Custom Routing**: Configure provider preferences and weights
4. **Rate Limiting**: Implement per-user rate limits
5. **Plugin System**: Extend functionality with custom plugins

---

**Congratulations!** You've successfully set up and used HelixAgent, an enterprise-grade LLM facade system. The system is now intelligently routing your requests across multiple providers and delivering the best possible responses through ensemble voting. 🚀