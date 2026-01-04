# DeepSeek Provider Setup Guide

## Overview

DeepSeek is a Chinese AI company that develops advanced large language models, particularly known for their code-focused models. SuperAgent integrates with DeepSeek's API to provide access to their chat and code completion models.

### Supported Models

- `deepseek-coder` - Optimized for code generation and analysis (default)
- `deepseek-chat` - General-purpose conversational model

### Key Features

- Text completion and chat
- Function calling
- Streaming responses
- Tool use support
- Reasoning capabilities
- Code completion and analysis
- Refactoring support

## API Key Setup

### Step 1: Create a DeepSeek Account

1. Visit [platform.deepseek.com](https://platform.deepseek.com)
2. Sign up for an account
3. Complete the verification process

### Step 2: Generate an API Key

1. Navigate to **API Keys** in your dashboard
2. Click **Create new API key**
3. Name your key and set permissions
4. Copy the API key immediately - store it securely

### Step 3: Store Your API Key Securely

```bash
# Add to your environment or .env file
export DEEPSEEK_API_KEY=your_api_key_here
```

## Environment Variable Configuration

Add the following to your `.env` file or environment:

```bash
# Required
DEEPSEEK_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxx

# Optional - Override default settings
DEEPSEEK_BASE_URL=https://api.deepseek.com/v1/chat/completions
DEEPSEEK_MODEL=deepseek-coder
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DEEPSEEK_API_KEY` | Yes | - | Your DeepSeek API key |
| `DEEPSEEK_BASE_URL` | No | `https://api.deepseek.com/v1/chat/completions` | API endpoint URL |
| `DEEPSEEK_MODEL` | No | `deepseek-coder` | Default model to use |

## Basic Usage Example

### Go Code Example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/superagent/superagent/internal/llm/providers/deepseek"
    "github.com/superagent/superagent/internal/models"
)

func main() {
    // Create provider
    provider := deepseek.NewDeepSeekProvider(
        os.Getenv("DEEPSEEK_API_KEY"),
        "", // Use default base URL
        "", // Use default model
    )

    // Create request
    req := &models.LLMRequest{
        ID:     "request-1",
        Prompt: "You are a helpful coding assistant.",
        Messages: []models.Message{
            {Role: "user", Content: "Write a Python function to check if a number is prime."},
        },
        ModelParams: models.ModelParams{
            MaxTokens:   1024,
            Temperature: 0.3,
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

### Code-Focused Request

```go
req := &models.LLMRequest{
    ID:     "request-1",
    Prompt: "You are an expert Go developer.",
    Messages: []models.Message{
        {Role: "user", Content: "Implement a concurrent-safe cache with TTL in Go."},
    },
    ModelParams: models.ModelParams{
        MaxTokens:   2048,
        Temperature: 0.2, // Lower temperature for more deterministic code
    },
}
```

## Rate Limits and Quotas

### Default Rate Limits

| Plan | Requests/Minute | Tokens/Minute |
|------|-----------------|---------------|
| Free | 60 | 60,000 |
| Standard | 600 | 300,000 |
| Enterprise | Custom | Custom |

### Model Context Limits

| Model | Max Input Tokens | Max Output Tokens |
|-------|-----------------|-------------------|
| DeepSeek Coder | 4,096 | 4,096 |
| DeepSeek Chat | 4,096 | 4,096 |

### Best Practices for Rate Limits

1. **Use exponential backoff** - SuperAgent automatically implements retry with backoff
2. **Monitor token usage** - Track consumption through response metadata
3. **Batch code reviews** - Combine related code snippets when possible
4. **Choose appropriate temperature** - Use lower values (0.1-0.3) for code generation

## Troubleshooting

### Common Errors

#### Authentication Error (401)

```
DeepSeek API error: 401 - {"error": {"message": "Invalid API key"}}
```

**Solution:**
- Verify your API key is correct
- Check that the key hasn't expired or been revoked
- Ensure the `Authorization: Bearer` header is properly formatted

#### Rate Limit Error (429)

```
DeepSeek API error: 429 - {"error": {"message": "Rate limit exceeded"}}
```

**Solution:**
- Wait for the rate limit window to reset
- SuperAgent automatically retries with exponential backoff
- Consider upgrading your plan for higher limits

#### Model Not Found (404)

```
DeepSeek API error: 404 - {"error": {"message": "Model not found"}}
```

**Solution:**
- Verify the model name is correct (`deepseek-coder` or `deepseek-chat`)
- Check DeepSeek documentation for currently available models

#### Content Filter (400)

```
DeepSeek API error: 400 - {"error": {"message": "Content filtered"}}
```

**Solution:**
- Review your prompt for potentially problematic content
- Rephrase the request to be more specific and technical

### Health Check

SuperAgent provides a health check endpoint for DeepSeek:

```go
err := provider.HealthCheck()
if err != nil {
    fmt.Printf("DeepSeek provider unhealthy: %v\n", err)
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

1. Check network connectivity to `api.deepseek.com`
2. Verify DNS resolution is working
3. Check if the service is accessible from your region
4. Consider increasing timeout (default is 60 seconds)

```go
// Custom retry configuration
retryConfig := deepseek.RetryConfig{
    MaxRetries:   5,
    InitialDelay: 2 * time.Second,
    MaxDelay:     60 * time.Second,
    Multiplier:   2.0,
}

provider := deepseek.NewDeepSeekProviderWithRetry(
    apiKey,
    baseURL,
    model,
    retryConfig,
)
```

### Response Quality Issues

If code generation quality is poor:

1. **Lower temperature** - Use 0.1-0.3 for deterministic code
2. **Be specific** - Provide detailed requirements and context
3. **Use system prompts** - Set clear expectations for the assistant
4. **Include examples** - Show expected input/output formats

Example improved prompt:

```go
req := &models.LLMRequest{
    ID:     "request-1",
    Prompt: `You are an expert Go developer. Write clean, idiomatic Go code.
Follow these guidelines:
- Use proper error handling
- Add meaningful comments
- Follow Go naming conventions
- Include type definitions where appropriate`,
    Messages: []models.Message{
        {Role: "user", Content: "Create a REST API endpoint handler for user registration."},
    },
    ModelParams: models.ModelParams{
        MaxTokens:   2048,
        Temperature: 0.2,
    },
}
```

## Additional Resources

- [DeepSeek API Documentation](https://platform.deepseek.com/docs)
- [DeepSeek Coder Model Card](https://github.com/deepseek-ai/DeepSeek-Coder)
- [DeepSeek Platform](https://platform.deepseek.com)
