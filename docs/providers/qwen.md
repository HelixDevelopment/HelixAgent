# Qwen (Alibaba) Provider Setup Guide

## Overview

Qwen (Tongyi Qianwen) is Alibaba Cloud's large language model family, offering multilingual capabilities with strong performance in Chinese and English. SuperAgent integrates with the DashScope API to provide access to Qwen models.

### Supported Models

- `qwen-turbo` - Fast, cost-effective model (default)
- `qwen-plus` - Balanced performance
- `qwen-max` - Most capable model
- `qwen-max-longcontext` - Extended context window support

### Key Features

- Text completion and chat
- Function calling
- Streaming responses (SSE)
- Tool use support
- Reasoning capabilities
- Code completion and analysis
- Strong multilingual support (Chinese/English)

## API Key Setup

### Step 1: Create an Alibaba Cloud Account

1. Visit [aliyun.com](https://www.aliyun.com) or [alibabacloud.com](https://www.alibabacloud.com)
2. Sign up for an account
3. Complete identity verification

### Step 2: Enable DashScope Service

1. Go to the [DashScope Console](https://dashscope.console.aliyun.com)
2. Activate the DashScope service
3. Accept the terms of service

### Step 3: Generate an API Key

1. In the DashScope console, go to **API-KEY Management**
2. Click **Create new API-KEY**
3. Name your key and confirm
4. Copy the API key immediately

### Step 4: Store Your API Key Securely

```bash
# Add to your environment or .env file
export QWEN_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxx
```

## Environment Variable Configuration

Add the following to your `.env` file or environment:

```bash
# Required
QWEN_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxx

# Optional - Override default settings
QWEN_BASE_URL=https://dashscope.aliyuncs.com/api/v1
QWEN_MODEL=qwen-turbo
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `QWEN_API_KEY` | Yes | - | Your DashScope API key |
| `QWEN_BASE_URL` | No | `https://dashscope.aliyuncs.com/api/v1` | API endpoint URL |
| `QWEN_MODEL` | No | `qwen-turbo` | Default model to use |

## Basic Usage Example

### Go Code Example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/superagent/superagent/internal/llm/providers/qwen"
    "github.com/superagent/superagent/internal/models"
)

func main() {
    // Create provider
    provider := qwen.NewQwenProvider(
        os.Getenv("QWEN_API_KEY"),
        "", // Use default base URL
        "", // Use default model
    )

    // Create request
    req := &models.LLMRequest{
        ID:     "request-1",
        Prompt: "You are a helpful assistant.",
        Messages: []models.Message{
            {Role: "user", Content: "Explain machine learning in simple terms."},
        },
        ModelParams: models.ModelParams{
            MaxTokens:   2000,
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

### Chinese Language Example

```go
req := &models.LLMRequest{
    ID:     "request-1",
    Prompt: "You are a helpful assistant fluent in Chinese.",
    Messages: []models.Message{
        {Role: "user", Content: "What is the weather like today?"},
    },
    ModelParams: models.ModelParams{
        MaxTokens:   1024,
        Temperature: 0.7,
    },
}
```

## Rate Limits and Quotas

### Default Rate Limits

| Model | Requests/Second | Tokens/Second |
|-------|-----------------|---------------|
| qwen-turbo | 300 | 100,000 |
| qwen-plus | 300 | 100,000 |
| qwen-max | 100 | 50,000 |

### Model Context Limits

| Model | Max Input Tokens | Max Output Tokens |
|-------|-----------------|-------------------|
| qwen-turbo | 6,000 | 2,000 |
| qwen-plus | 30,000 | 2,000 |
| qwen-max | 30,000 | 2,000 |
| qwen-max-longcontext | 28,000 | 2,000 |

### Concurrent Request Limits

SuperAgent configures a default limit of 50 concurrent requests per provider instance.

### Best Practices for Rate Limits

1. **Use exponential backoff** - SuperAgent automatically implements retry with backoff
2. **Monitor usage** - Track token consumption through the DashScope console
3. **Choose appropriate models** - Use `qwen-turbo` for cost-effective operations
4. **Batch requests** - Combine related queries when possible

## Troubleshooting

### Common Errors

#### Authentication Error (401)

```
Qwen API error: Unauthorized (invalid_api_key)
```

**Solution:**
- Verify your API key is correct
- Check that the key is active in the DashScope console
- Ensure no extra whitespace in the environment variable

#### Rate Limit Error (429)

```
Qwen API error: Too Many Requests (rate_limit_exceeded)
```

**Solution:**
- Wait for the rate limit window to reset
- SuperAgent automatically retries with exponential backoff
- Consider using request queuing for high-volume applications

#### Quota Exceeded (403)

```
Qwen API error: Forbidden (quota_exceeded)
```

**Solution:**
- Check your account quota in the DashScope console
- Purchase additional quota if needed
- Switch to a lower-tier model temporarily

#### Invalid Request (400)

```
Qwen API error: Bad Request (invalid_request)
```

**Solution:**
- Verify the request format is correct
- Check that message roles are valid
- Ensure max_tokens is within model limits

### Health Check

SuperAgent provides a health check endpoint for Qwen:

```go
err := provider.HealthCheck()
if err != nil {
    fmt.Printf("Qwen provider unhealthy: %v\n", err)
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

1. Check network connectivity to `dashscope.aliyuncs.com`
2. Verify the API is accessible from your region
3. Consider using Alibaba Cloud's international endpoint if outside China
4. Consider increasing timeout (default is 60 seconds)

```go
// Custom retry configuration
retryConfig := qwen.RetryConfig{
    MaxRetries:   5,
    InitialDelay: 2 * time.Second,
    MaxDelay:     60 * time.Second,
    Multiplier:   2.0,
}

provider := qwen.NewQwenProviderWithRetry(
    apiKey,
    baseURL,
    model,
    retryConfig,
)
```

### Streaming Issues

For SSE streaming problems:

1. Ensure proper headers are set:
   - `Accept: text/event-stream`
   - `Cache-Control: no-cache`
   - `Connection: keep-alive`

2. Check for proxy or firewall interference with SSE connections

3. Verify the response is being parsed correctly for `data:` prefixed lines

### Regional Considerations

DashScope endpoints may vary by region:

| Region | Endpoint |
|--------|----------|
| China | `https://dashscope.aliyuncs.com/api/v1` |
| International | `https://dashscope-intl.aliyuncs.com/api/v1` |

Configure the appropriate endpoint based on your deployment location.

## Model Selection Guide

| Use Case | Recommended Model | Reason |
|----------|-------------------|--------|
| Quick responses | qwen-turbo | Fastest, most cost-effective |
| Complex analysis | qwen-max | Best reasoning capabilities |
| Long documents | qwen-max-longcontext | Extended context window |
| General use | qwen-plus | Good balance of speed and quality |

## Additional Resources

- [DashScope Console](https://dashscope.console.aliyun.com)
- [Qwen API Documentation](https://help.aliyun.com/zh/dashscope/developer-reference/api-details)
- [Alibaba Cloud AI Documentation](https://www.alibabacloud.com/help/en/dashscope)
- [Qwen Model Card](https://github.com/QwenLM/Qwen)
