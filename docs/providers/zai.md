# Z.AI Provider Setup Guide

## Overview

Z.AI is an AI platform providing access to various language models through a unified API. SuperAgent integrates with Z.AI to provide text completion and chat capabilities.

### Supported Models

- `z-ai-base` - Standard model for general tasks (default)
- `z-ai-pro` - Enhanced model with better performance
- `z-ai-enterprise` - Enterprise-grade model with additional features

### Key Features

- Text completion
- Chat conversations
- Streaming responses
- Both completion and chat API formats

## API Key Setup

### Step 1: Create a Z.AI Account

1. Visit [z.ai](https://z.ai) (or the relevant Z.AI platform)
2. Sign up for an account
3. Complete the registration process

### Step 2: Generate an API Key

1. Navigate to your account settings or API section
2. Generate a new API key
3. Copy the API key and store it securely

### Step 3: Store Your API Key Securely

```bash
# Add to your environment or .env file
export ZAI_API_KEY=your_api_key_here
```

## Environment Variable Configuration

Add the following to your `.env` file or environment:

```bash
# Required
ZAI_API_KEY=your_api_key_here

# Optional - Override default settings
ZAI_BASE_URL=https://api.z.ai/v1
ZAI_MODEL=z-ai-base
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ZAI_API_KEY` | Yes | - | Your Z.AI API key |
| `ZAI_BASE_URL` | No | `https://api.z.ai/v1` | API endpoint URL |
| `ZAI_MODEL` | No | `z-ai-base` | Default model to use |

## Basic Usage Example

### Go Code Example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/superagent/superagent/internal/llm/providers/zai"
    "github.com/superagent/superagent/internal/models"
)

func main() {
    // Create provider
    provider := zai.NewZAIProvider(
        os.Getenv("ZAI_API_KEY"),
        "", // Use default base URL
        "", // Use default model
    )

    // Create request with messages (chat format)
    req := &models.LLMRequest{
        ID: "request-1",
        Messages: []models.Message{
            {Role: "system", Content: "You are a helpful assistant."},
            {Role: "user", Content: "What is artificial intelligence?"},
        },
        ModelParams: models.ModelParams{
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

### Completion Format (Prompt-based)

```go
// Create request with prompt (completion format)
req := &models.LLMRequest{
    ID:     "request-1",
    Prompt: "Write a haiku about programming:",
    ModelParams: models.ModelParams{
        MaxTokens:   256,
        Temperature: 0.8,
    },
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
    if chunk.FinishReason == "error" {
        fmt.Printf("Error: %s\n", chunk.Content)
        break
    }
    fmt.Print(chunk.Content)
}
```

## Rate Limits and Quotas

### Default Rate Limits

| Model | Requests/Minute | Tokens/Minute |
|-------|-----------------|---------------|
| z-ai-base | 60 | 100,000 |
| z-ai-pro | 60 | 150,000 |
| z-ai-enterprise | Custom | Custom |

### Model Context Limits

| Model | Max Input Tokens | Max Output Tokens |
|-------|-----------------|-------------------|
| z-ai-base | 8,192 | 4,096 |
| z-ai-pro | 8,192 | 4,096 |
| z-ai-enterprise | 16,384 | 8,192 |

### Best Practices for Rate Limits

1. **Use exponential backoff** - SuperAgent automatically implements retry with backoff
2. **Monitor usage** - Track token consumption in your account dashboard
3. **Choose appropriate models** - Use base model for simple tasks
4. **Implement request queuing** - For high-volume applications

## Troubleshooting

### Common Errors

#### Authentication Error (401)

```
Z.AI API error: Unauthorized (unauthorized)
```

**Solution:**
- Verify your API key is correct
- Check that the key is active
- Ensure the `Authorization: Bearer` header is properly formatted

#### Rate Limit Error (429)

```
Z.AI API error: Too Many Requests (rate_limit_exceeded)
```

**Solution:**
- Wait for the rate limit window to reset
- SuperAgent automatically retries with exponential backoff
- Consider upgrading your plan for higher limits

#### Model Not Found (404)

```
Z.AI API error: Not Found (model_not_found)
```

**Solution:**
- Verify the model name is correct
- Check that the model is available for your account tier
- Use the default `z-ai-base` model as a fallback

#### Invalid Request (400)

```
Z.AI API error: Bad Request (invalid_request)
```

**Solution:**
- Verify the request format is correct
- Ensure either `prompt` or `messages` is provided (not both empty)
- Check that parameters are within valid ranges

### Health Check

SuperAgent provides a health check endpoint for Z.AI:

```go
err := provider.HealthCheck()
if err != nil {
    fmt.Printf("Z.AI provider unhealthy: %v\n", err)
}
```

### Debug Logging

Enable debug logging to troubleshoot issues:

```bash
export GIN_MODE=debug
export LOG_LEVEL=debug
```

### Connection Issues

If experiencing connection timeouts:

1. Check network connectivity to `api.z.ai`
2. Verify DNS resolution is working
3. Check if the service is accessible from your region
4. Consider increasing timeout (default is 60 seconds)

```go
// Custom retry configuration
retryConfig := zai.RetryConfig{
    MaxRetries:   5,
    InitialDelay: 2 * time.Second,
    MaxDelay:     60 * time.Second,
    Multiplier:   2.0,
}

provider := zai.NewZAIProviderWithRetry(
    apiKey,
    baseURL,
    model,
    retryConfig,
)
```

### Streaming Issues

If streaming responses are not working correctly:

1. Ensure the `Accept: text/event-stream` header is set
2. Check for proxy or firewall interference
3. Verify the SSE parsing handles `data:` prefixed lines correctly
4. Check for the `[DONE]` marker to properly end streams

## Request Formats

Z.AI supports two request formats:

### Chat Completion Format

Used when `messages` array is provided:

```json
{
    "model": "z-ai-base",
    "messages": [
        {"role": "system", "content": "You are helpful."},
        {"role": "user", "content": "Hello!"}
    ],
    "temperature": 0.7,
    "max_tokens": 1024
}
```

Endpoint: `POST /v1/chat/completions`

### Text Completion Format

Used when only `prompt` is provided:

```json
{
    "model": "z-ai-base",
    "prompt": "Complete this sentence:",
    "temperature": 0.7,
    "max_tokens": 256
}
```

Endpoint: `POST /v1/completions`

SuperAgent automatically selects the appropriate endpoint based on the request content.

## Model Selection Guide

| Use Case | Recommended Model | Reason |
|----------|-------------------|--------|
| Simple queries | z-ai-base | Cost-effective, fast |
| Complex analysis | z-ai-pro | Better reasoning |
| Enterprise apps | z-ai-enterprise | SLA, support, higher limits |

## Additional Resources

- [Z.AI Platform](https://z.ai)
- [Z.AI API Documentation](https://docs.z.ai)
- [Z.AI Developer Portal](https://developers.z.ai)
