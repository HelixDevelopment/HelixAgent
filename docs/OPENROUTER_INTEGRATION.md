# OpenRouter AI Provider Integration

This document provides comprehensive guidance for integrating OpenRouter AI with SuperAgent.

## Overview

OpenRouter is an AI model routing service that provides access to multiple AI models through a single API key. SuperAgent now supports OpenRouter with:

- **Multi-tenancy**: Multiple configurations with the same OpenRouter API key
- **Model selection**: Choose between 50+ AI models
- **Cost optimization**: Automatic routing to most cost-effective models
- **Performance tracking**: Route to best performing providers

## Key Features

### 🔑 **Multi-Tenancy Support**
Configure multiple OpenRouter configurations using the same API key but different models and routing strategies:

```bash
# Example: Multiple configurations with same API key
OPENROUTER_API_KEY="sk-or-v1-your-openrouter-key"

# Environment variables for different configurations
OPENROUTER_DEFAULT_MODEL="openrouter/anthropic/claude-3.5-sonnet"
OPENROUTER_COST_OPTIMIZED="true"
OPENROUTER_ROUTE_STRATEGY="cost_optimized"
```

### 🎯 **Advanced Routing Strategies**

- **Basic**: Simple round-robin selection
- **Cost-Optimized**: Routes to lowest-cost models
- **Performance-Optimized**: Routes to best-performing providers
- **Multi-Model**: Intelligent model-based routing
- **Round Robin**: Weighted load balancing
- **Weighted**: Custom weighted routing

### 🏢 **Extensive Model Support**

Access to 50+ models including:

- **Anthropic**: Claude 3.5 Sonnet, Opus
- **OpenAI**: GPT-4o, GPT-4 Turbo
- **Google**: Gemini Pro 1.5
- **Meta**: Llama 3.1, Llama 2
- **Mistral**: Large
- **Perplexity**: 70B, 8x7B
- **OpenRouter**: Custom models
- And many more...

## 📊 **Usage Monitoring**

- **Real-time statistics**: Requests, success rates, response times
- **Cost tracking**: Per-model usage and costs
- **Performance metrics**: Provider performance comparison
- **Health monitoring**: Automatic health checks

## 🚀 **Enterprise Features**

- **Rate limiting**: Configurable per provider
- **API key management**: Secure API key handling
- **Request queuing**: Intelligent request management
- **Failover handling**: Automatic fallback mechanisms

## 🔧 **Configuration**

### Environment Variables

```bash
# OpenRouter Configuration
OPENROUTER_API_KEY=your_openrouter_api_key
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
OPENROUTER_DEFAULT_MODEL=openrouter/anthropic/claude-3.5-sonnet
OPENROUTER_ENABLED=true
OPENROUTER_MAX_RETRIES=3
OPENROUTER_TIMEOUT=60s

# Multi-tenancy
OPENROUTER_TENANT_1_API_KEY=sk-or-v1-openrouter-key
OPENROUTER_TENANT_1_DEFAULT_MODEL=openrouter/google/gemini-pro
OPENROUTER_TENANT_2_API_KEY=sk-or-v1-openrouter-key

# Routing Strategies
OPENROUTER_ROUTE_STRATEGY=cost_optimized
OPENROUTER_FALLBACK_STRATEGY=basic
OPENROUTER_PERFORMANCE_CACHE=true
```

## 🚀 **Getting Started**

1. **Add API Key**: Get your OpenRouter API key from [OpenRouter.ai](https://openrouter.ai/)

2. **Configure Environment**: Add the OpenRouter environment variables to your `.env` file

3. **Start SuperAgent**: `make docker-full` will include OpenRouter

4. **Test Integration**: Use `OPENROUTER_API_KEY` to test the integration

## 📖 **API Reference**

### Basic Request

```bash
curl -X POST https://openrouter.ai/api/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openrouter/anthropic/claude-3.5-sonnet",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### Advanced Request with Model Selection

```bash
curl -X POST https://openrouter.ai/api/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "HTTP-Referer: superagent" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openrouter/google/gemini-pro",
    "route": {
      "strategy": "cost_optimized",
      "fallback": "basic"
    },
    "messages": [
      {"role": "user", "content": "Write a Python function"}
    ]
  }'
```

## 🎯 **Integration with SuperAgent**

SuperAgent automatically detects and integrates with OpenRouter when `OPENROUTER_API_KEY` is configured.

### Configuration Examples

```yaml
# docker-compose.yml (partial)
version: '3.8'
services:
  superagent:
    environment:
      - OPENROUTER_API_KEY: ${OPENROUTER_API_KEY}
      - OPENROUTER_ENABLED: "true"
      - OPENROUTER_DEFAULT_MODEL: "openrouter/anthropic/claude-3.5-sonnet"
      - OPENROUTER_ROUTE_STRATEGY: "cost_optimized"
```

## 🔧 **Troubleshooting**

### Common Issues

**API Key Not Found**:
```bash
curl -H https://openrouter.ai/api/v1/models \
  -H "Authorization: Bearer test_key" \
  -H "HTTP-Referer: superagent"
# Expected: 401 Unauthorized
# Solution: Set OPENROUTER_API_KEY correctly
```

**Model Not Available**:
```bash
curl -H https://openrouter.ai/api/v1/models \
  -H "Authorization: Bearer valid_key"
# Check the model list for available models
```

**Rate Limiting**:
```bash
# Response: 429 Too Many Requests
# Solution: Wait or implement rate limiting
```

## 📊 **Best Practices**

### Cost Optimization

- Use `OPENROUTER_ROUTE_STRATEGY=cost_optimized` for production
- Monitor model costs via OpenRouter dashboard
- Set appropriate spending limits
- Use cheaper models for development/testing

### Performance Optimization

- Use `OPENROUTER_PERFORMANCE_CACHE=true` to enable performance-based routing
- Monitor response times
- Set appropriate timeouts

### Multi-Tenancy

- Use different API keys for different projects/environments
- Configure model preferences per tenant
- Monitor costs per tenant
- Implement usage quotas

### Security

- Never commit API keys to version control
- Use environment variables or secure secret management
- Implement request logging and audit trails

### Testing

- Test with different models before production
- Use smaller models for development
- Mock expensive providers during unit testing

## 🎯 **Support**

For OpenRouter support issues or questions:

1. Check the [OpenRouter Documentation](https://openrouter.ai/docs)
2. Review SuperAgent logs for OpenRouter integration issues
3. Check the SuperAgent dashboard for provider statistics

4. Join the [OpenRouter Community](https://community.openrouter.ai) for community support

## 📚 **Resource Links**

- [OpenRouter Official Docs](https://openrouter.ai/docs)
- [OpenRouter Pricing](https://openrouter.ai/pricing)
- [OpenRouter Community Discord](https://discord.gg/openrouter)
- [SuperAgent Dashboard](https://dashboard.openrouter.ai)

---

*Note: This documentation covers the integration. For the most up-to-date information, always refer to the official OpenRouter documentation.*