# OpenRouter Provider Setup Guide

## Overview

OpenRouter is a unified API gateway that provides access to multiple AI models from various providers through a single API. This allows you to use models from OpenAI, Anthropic, Google, Meta, and many others without managing separate API keys for each provider.

### Supported Models

OpenRouter provides access to 100+ models including:

**Top Providers:**
- `openrouter/anthropic/claude-3.5-sonnet` - Anthropic Claude
- `openrouter/openai/gpt-4o` - OpenAI GPT-4o
- `openrouter/google/gemini-pro` - Google Gemini
- `openrouter/meta-llama/llama-3.1-405b` - Meta Llama 3.1
- `openrouter/mistralai/mistral-large` - Mistral AI
- `openrouter/deepseek-v2-lite` - DeepSeek

**Open Source Models:**
- `openrouter/meta-llama/llama-3.1-70b`
- `openrouter/perplexity-70b`
- Various other open source models

### Key Features

- Access to 100+ models through one API
- Multi-model routing
- Cost optimization
- Streaming responses
- Tool use support
- Search capabilities
- Code completion and analysis
- Unified billing across providers

## API Key Setup

### Step 1: Create an OpenRouter Account

1. Visit [openrouter.ai](https://openrouter.ai)
2. Click **Sign In** and create an account
3. Verify your email address

### Step 2: Add Credits

1. Navigate to **Credits** in your dashboard
2. Add funds to your account (minimum $5)
3. Your credits will be used across all model providers

### Step 3: Generate an API Key

1. Go to **API Keys** in your dashboard
2. Click **Create Key**
3. Name your key (e.g., "SuperAgent")
4. Copy the API key immediately

### Step 4: Store Your API Key Securely

```bash
# Add to your environment or .env file
export OPENROUTER_API_KEY=sk-or-v1-xxxxxxxxxxxxxxxxxxxxxxxx
```

## Environment Variable Configuration

Add the following to your `.env` file or environment:

```bash
# Required
OPENROUTER_API_KEY=sk-or-v1-xxxxxxxxxxxxxxxxxxxxxxxx

# Optional - Override default settings
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
OPENROUTER_DEFAULT_MODEL=openrouter/anthropic/claude-3.5-sonnet
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENROUTER_API_KEY` | Yes | - | Your OpenRouter API key |
| `OPENROUTER_BASE_URL` | No | `https://openrouter.ai/api/v1` | API endpoint URL |
| `OPENROUTER_DEFAULT_MODEL` | No | - | Default model to use |

## Basic Usage Example

### Go Code Example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/superagent/superagent/internal/llm/providers/openrouter"
    "github.com/superagent/superagent/internal/models"
)

func main() {
    // Create provider
    provider := openrouter.NewSimpleOpenRouterProvider(
        os.Getenv("OPENROUTER_API_KEY"),
    )

    // Create request
    req := &models.LLMRequest{
        ID: "request-1",
        Messages: []models.Message{
            {Role: "user", Content: "What is the capital of France?"},
        },
        ModelParams: models.ModelParams{
            Model:       "openrouter/anthropic/claude-3.5-sonnet",
            MaxTokens:   1024,
            Temperature: 0.7,
        },
    }

    // Make completion request
    ctx := context.Background()
    resp, err := provider.Complete(ctx, req)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Response: %s\n", resp.Content)
}
```

### Streaming Example

```go
// Enable streaming
streamChan, err := provider.CompleteStream(ctx, req)
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}

for chunk := range streamChan {
    fmt.Print(chunk.Content)
}
```

### Using Different Models

```go
// Use GPT-4
req := &models.LLMRequest{
    ID: "request-1",
    Messages: []models.Message{
        {Role: "user", Content: "Write a poem about coding."},
    },
    ModelParams: models.ModelParams{
        Model:       "openrouter/openai/gpt-4o",
        MaxTokens:   1024,
        Temperature: 0.8,
    },
}

// Use Llama 3.1
req := &models.LLMRequest{
    ID: "request-2",
    Messages: []models.Message{
        {Role: "user", Content: "Explain quantum computing."},
    },
    ModelParams: models.ModelParams{
        Model:       "openrouter/meta-llama/llama-3.1-70b",
        MaxTokens:   2048,
        Temperature: 0.7,
    },
}
```

## Rate Limits and Quotas

### Default Rate Limits

OpenRouter rate limits vary by model and account tier:

| Tier | Requests/Minute | Concurrent Requests |
|------|-----------------|---------------------|
| Free | 20 | 5 |
| Standard | 200 | 50 |
| Pro | 1000 | 100 |

### Model-Specific Limits

Different models have different rate limits based on the underlying provider. Check the [OpenRouter Models page](https://openrouter.ai/models) for specific limits.

### Context Limits

| Model Family | Max Context |
|--------------|-------------|
| Claude 3.5 | 200,000 tokens |
| GPT-4o | 128,000 tokens |
| Llama 3.1 | 128,000 tokens |
| Mistral Large | 32,000 tokens |

### Best Practices for Rate Limits

1. **Use exponential backoff** - SuperAgent automatically implements retry with backoff
2. **Monitor usage** - Check your OpenRouter dashboard for usage statistics
3. **Use appropriate models** - Choose cost-effective models for simple tasks
4. **Enable request queuing** - For high-volume applications

## Cost Management

OpenRouter provides transparent pricing per model:

### Checking Costs

1. Visit the [OpenRouter Models page](https://openrouter.ai/models)
2. View per-token pricing for each model
3. Monitor spending in your dashboard

### Cost Optimization Tips

1. **Use smaller models** when appropriate
2. **Set max_tokens limits** to prevent runaway costs
3. **Cache responses** for repeated queries
4. **Use streaming** for faster perceived response times

## Troubleshooting

### Common Errors

#### Authentication Error (401)

```
OpenRouter API error: Invalid API key
```

**Solution:**
- Verify your API key is correct
- Check that the key hasn't been revoked
- Ensure proper `Authorization: Bearer` header

#### Insufficient Credits (402)

```
OpenRouter API error: Insufficient credits
```

**Solution:**
- Add more credits to your OpenRouter account
- Check your balance in the dashboard
- Consider using a more cost-effective model

#### Rate Limit Error (429)

```
OpenRouter API error: Rate limit exceeded
```

**Solution:**
- Wait for the rate limit to reset
- SuperAgent automatically retries with exponential backoff
- Upgrade to a higher tier if needed

#### Model Not Found (404)

```
OpenRouter API error: Model not found
```

**Solution:**
- Verify the model ID is correct
- Check that the model is available on OpenRouter
- Use the model browser at openrouter.ai/models

#### Provider Unavailable (503)

```
OpenRouter API error: Provider temporarily unavailable
```

**Solution:**
- Wait and retry (automatic with SuperAgent)
- Try an alternative model from a different provider
- Check OpenRouter status page

### Health Check

SuperAgent provides a health check for OpenRouter:

```go
err := provider.HealthCheck()
if err != nil {
    fmt.Printf("OpenRouter provider unhealthy: %v\n", err)
}
```

Note: OpenRouter doesn't have a dedicated health check endpoint. The health check verifies that the API key is configured.

### Debug Logging

Enable debug logging to troubleshoot issues:

```bash
export GIN_MODE=debug
export LOG_LEVEL=debug
```

### Connection Issues

If experiencing connection timeouts:

1. Check network connectivity to `openrouter.ai`
2. Verify SSL/TLS is working correctly
3. Check for proxy interference
4. Consider increasing timeout (default is 60 seconds)

```go
// Custom retry configuration
retryConfig := openrouter.RetryConfig{
    MaxRetries:   5,
    InitialDelay: 2 * time.Second,
    MaxDelay:     60 * time.Second,
    Multiplier:   2.0,
}

provider := openrouter.NewSimpleOpenRouterProviderWithRetry(
    apiKey,
    baseURL,
    retryConfig,
)
```

## Headers and Metadata

OpenRouter supports additional headers for tracking:

```go
httpReq.Header.Set("HTTP-Referer", "superagent")  // Identify your app
httpReq.Header.Set("X-Title", "SuperAgent App")   // Your app name
```

These headers help OpenRouter track usage and may be required for some models.

## Model Selection Guide

| Use Case | Recommended Model | Cost |
|----------|-------------------|------|
| Complex reasoning | claude-3.5-sonnet | $$$ |
| General chat | gpt-4o | $$$ |
| Code generation | deepseek-coder | $$ |
| Fast responses | llama-3.1-70b | $ |
| Cost-effective | mistral-7b | $ |

## Additional Resources

- [OpenRouter Website](https://openrouter.ai)
- [OpenRouter API Documentation](https://openrouter.ai/docs)
- [Model Browser](https://openrouter.ai/models)
- [Pricing Information](https://openrouter.ai/docs#pricing)
- [OpenRouter Discord](https://discord.gg/openrouter)
