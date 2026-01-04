# LLM Provider Setup Guides

SuperAgent supports 9 LLM providers, offering flexibility to choose the right model for your use case. This directory contains detailed setup guides for each provider.

## Provider Overview

| Provider | Type | API Key Required | Best For |
|----------|------|------------------|----------|
| [Claude (Anthropic)](./claude.md) | Cloud | Yes | Complex reasoning, code, analysis |
| [DeepSeek](./deepseek.md) | Cloud | Yes | Code generation, technical tasks |
| [Gemini (Google)](./gemini.md) | Cloud | Yes | Multimodal, general tasks |
| [Qwen (Alibaba)](./qwen.md) | Cloud | Yes | Multilingual, Chinese/English |
| [Z.AI](./zai.md) | Cloud | Yes | General purpose |
| [Ollama](./ollama.md) | Local | No | Privacy, no internet required |
| [OpenRouter](./openrouter.md) | Cloud | Yes | Multi-model access, cost optimization |
| [AWS Bedrock](./aws-bedrock.md) | Cloud | AWS Credentials | Enterprise, compliance |
| [Azure OpenAI](./azure-openai.md) | Cloud | Yes | Enterprise, Microsoft ecosystem |

## Quick Start

### 1. Choose Your Provider

Consider these factors when choosing a provider:

- **Privacy**: Use Ollama for fully local processing
- **Enterprise**: Use AWS Bedrock or Azure OpenAI for compliance
- **Cost**: Use OpenRouter for pay-per-use across multiple models
- **Performance**: Use Claude or Gemini for complex tasks
- **Code**: Use DeepSeek or Claude for code generation

### 2. Configure Environment Variables

Example `.env` configuration with multiple providers:

```bash
# Anthropic Claude
CLAUDE_API_KEY=sk-ant-api03-xxxxx

# Google Gemini
GEMINI_API_KEY=AIzaxxxxx

# DeepSeek
DEEPSEEK_API_KEY=sk-xxxxx

# Alibaba Qwen
QWEN_API_KEY=sk-xxxxx

# Z.AI
ZAI_API_KEY=xxxxx

# Ollama (local)
OLLAMA_ENABLED=true
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama2

# OpenRouter
OPENROUTER_API_KEY=sk-or-v1-xxxxx

# AWS Bedrock
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=xxxxx
AWS_REGION=us-east-1

# Azure OpenAI
AZURE_OPENAI_ENDPOINT=https://your-resource.openai.azure.com
AZURE_OPENAI_API_KEY=xxxxx
AZURE_OPENAI_API_VERSION=2024-02-01
```

### 3. Verify Setup

Run the health check for your configured providers:

```go
// Example: Check provider health
err := provider.HealthCheck()
if err != nil {
    log.Printf("Provider unhealthy: %v", err)
}
```

## Feature Comparison

| Feature | Claude | DeepSeek | Gemini | Qwen | Z.AI | Ollama | OpenRouter | Bedrock | Azure |
|---------|--------|----------|--------|------|------|--------|------------|---------|-------|
| Streaming | Yes | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes |
| Function Calling | Yes | Yes | Yes | Yes | No | No | No | Varies | Yes |
| Vision | Yes | No | Yes | No | No | No | Varies | Varies | Yes |
| Tool Use | Yes | Yes | Yes | Yes | No | No | Yes | Varies | Yes |
| Code Focus | Yes | Yes | Yes | Yes | No | Yes | Varies | Varies | Yes |

## Common Issues

### API Key Not Working
1. Verify the key is correct (no extra whitespace)
2. Check the key hasn't expired or been revoked
3. Ensure the API is enabled for your account

### Rate Limiting
1. SuperAgent automatically retries with exponential backoff
2. Consider upgrading your plan for higher limits
3. Use request queuing for high-volume applications

### Connection Timeouts
1. Check network connectivity to the provider
2. Verify firewall/proxy settings
3. Consider increasing the timeout setting

## Provider-Specific Guides

- [Claude (Anthropic)](./claude.md) - Anthropic's AI assistant
- [DeepSeek](./deepseek.md) - Code-focused LLM
- [Gemini (Google)](./gemini.md) - Google's multimodal AI
- [Qwen (Alibaba)](./qwen.md) - Alibaba Cloud's LLM
- [Z.AI](./zai.md) - Z.AI platform
- [Ollama](./ollama.md) - Local LLM runner
- [OpenRouter](./openrouter.md) - Multi-model gateway
- [AWS Bedrock](./aws-bedrock.md) - AWS managed AI service
- [Azure OpenAI](./azure-openai.md) - Azure-hosted OpenAI models

## Related Documentation

- [Multi-Provider Configuration](../guides/multi-provider-setup.md)
- [Ensemble Strategies](../guides/ensemble-strategies.md)
- [LLM Optimization](../guides/LLM_OPTIMIZATION_USER_GUIDE.md)
