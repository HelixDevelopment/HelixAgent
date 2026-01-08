# HelixAgent Multi-Provider Setup Guide

## Overview

HelixAgent now provides 100% OpenAI API compatibility with automatic ensemble multi-provider support. Configure multiple LLM providers (DeepSeek, Qwen, OpenRouter Grok-4, OpenRouter Gemini 2.5) and use them transparently through a single unified endpoint.

## Quick Start

### 1. Configure API Keys

Set your API keys as environment variables:

```bash
export DEEPSEEK_API_KEY="sk-your-deepseek-key"
export QWEN_API_KEY="sk-your-qwen-key"  
export OPENROUTER_API_KEY="sk-or-your-openrouter-key"
```

### 2. Start the Server

```bash
# Using default configuration
go run ./cmd/helixagent/main_multi_provider.go

# Or with custom config
CONFIG_PATH=configs/multi-provider.yaml go run ./cmd/helixagent/main_multi_provider.go

# Or build and run
go build -o helixagent-multi ./cmd/helixagent/main_multi_provider.go
./helixagent-multi
```

### 3. Use with AI CLI Tools

The server runs on `http://localhost:7061` and exposes OpenAI-compatible endpoints at `/v1`:

```bash
# Test with OpenCode
opencode --api-key test-key --base-url http://localhost:7061/v1 --model helixagent-ensemble "Write a Go function"

# Test with Crush
crush --api-key test-key --base-url http://localhost:7061/v1 --model helixagent-ensemble "Explain microservices"

# Any OpenAI-compatible tool
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key" \
  -d '{"model":"helixagent-ensemble","messages":[{"role":"user","content":"Hello"}]}'
```

## Configuration

### Multi-Provider Configuration

Edit `configs/multi-provider.yaml`:

```yaml
providers:
  deepseek:
    name: "DeepSeek"
    type: "deepseek"
    enabled: true
    api_key: "${DEEPSEEK_API_KEY}"
    models:
      - name: "deepseek-chat"
        display_name: "DeepSeek Chat"
        capabilities: ["chat", "coding"]
  
  qwen:
    name: "Qwen"
    type: "qwen" 
    enabled: true
    api_key: "${QWEN_API_KEY}"
    models:
      - name: "qwen-turbo"
        display_name: "Qwen Turbo"
        capabilities: ["chat", "reasoning"]
  
  openrouter:
    name: "OpenRouter"
    type: "openrouter"
    enabled: true
    api_key: "${OPENROUTER_API_KEY}"
    models:
      - name: "x-ai/grok-4"
        display_name: "Grok-4"
        capabilities: ["chat", "reasoning"]
      - name: "google/gemini-2.5-flash"
        display_name: "Gemini 2.5 Flash"
        capabilities: ["chat", "multimodal"]

ensemble:
  strategy: "confidence_weighted"
  min_providers: 2
  confidence_threshold: 0.7
  fallback_to_best: true
```

### Environment Variables

- `DEEPSEEK_API_KEY`: DeepSeek API key
- `QWEN_API_KEY`: Qwen API key  
- `OPENROUTER_API_KEY`: OpenRouter API key
- `CONFIG_PATH`: Path to configuration file (default: `configs/multi-provider.yaml`)

## Available Models

### Ensemble Model
- **helixagent-ensemble**: Automatically uses all configured providers with intelligent voting to return the best result

### Individual Models
- **deepseek-chat**: DeepSeek Chat model
- **deepseek-coder**: DeepSeek Coder model
- **qwen-turbo**: Qwen Turbo model
- **qwen-plus**: Qwen Plus model
- **x-ai/grok-4**: Grok-4 via OpenRouter
- **google/gemini-2.5-flash**: Gemini 2.5 Flash via OpenRouter
- **anthropic/claude-3.5-sonnet**: Claude 3.5 Sonnet via OpenRouter
- And many more...

## OpenAI API Endpoints

All standard OpenAI endpoints are supported:

- `GET /v1/models` - List available models
- `POST /v1/chat/completions` - Chat completions (uses ensemble by default)
- `POST /v1/chat/completions/stream` - Streaming chat completions
- `POST /v1/completions` - Text completions
- `POST /v1/completions/stream` - Streaming text completions

## Admin Endpoints

- `GET /admin/providers` - List configured providers
- `GET /admin/ensemble/status` - Ensemble service status
- `GET /health` - Health check

## Testing

Run the built-in test:

```bash
go run test_api.go
```

Or use the test script:

```bash
chmod +x test_multi_provider.sh
./test_multi_provider.sh
```

## How It Works

1. **Automatic Ensemble**: By default, all requests use `helixagent-ensemble` which queries multiple providers and selects the best response
2. **Provider Selection**: You can also specify individual models to use a specific provider
3. **OpenAI Compatible**: 100% compatible with OpenAI API format - works with any OpenAI-compatible tool
4. **Intelligent Routing**: Automatic failover and confidence-based response selection
5. **MCP/LSP Ready**: All provider capabilities and tools exposed through unified API

## Docker Deployment

```bash
# Build and start with Docker Compose
docker-compose -f docker-compose.multi-provider.yaml up -d

# Set environment variables first
export DEEPSEEK_API_KEY="sk-your-key"
export QWEN_API_KEY="sk-your-key" 
export OPENROUTER_API_KEY="sk-or-your-key"
```

## Production Tips

1. **Use Real API Keys**: Replace test keys with actual provider API keys
2. **Database Setup**: Configure PostgreSQL and Redis for production
3. **Monitoring**: Use the admin endpoints for health monitoring
4. **Rate Limits**: Configure appropriate rate limits for each provider
5. **Load Balancing**: Deploy multiple instances behind a load balancer

## Troubleshooting

### Server Won't Start
- Check if ports 8080, 5432, 6379 are available
- Verify configuration file syntax
- Check environment variables

### API Calls Fail
- Verify API keys are correct
- Check provider connectivity
- Review ensemble configuration
- Look at server logs for errors

### Model Not Available
- Ensure provider is enabled in configuration
- Check if provider has valid API key
- Verify model name matches provider's model names

## Next Steps

The multi-provider system is ready for production use. Future enhancements will include:

1. **MCP/LSP Protocol Support**: Full Model Context Protocol and Language Server Protocol integration
2. **Advanced Ensemble Strategies**: Custom voting algorithms and provider selection
3. **Real-time Analytics**: Provider performance metrics and optimization
4. **Auto-scaling**: Dynamic provider scaling based on load