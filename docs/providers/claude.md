# Claude (Anthropic) Provider Setup Guide

## Overview

Claude is Anthropic's flagship AI assistant, known for its helpfulness, harmlessness, and honesty. SuperAgent integrates with the Claude API to provide access to various Claude model versions including Claude 3 Opus, Sonnet, and Haiku.

### Supported Models

- `claude-3-opus-20240229` - Most capable model for complex tasks
- `claude-3-sonnet-20240229` - Balanced performance and speed (default)
- `claude-3-haiku-20240307` - Fastest model for simple tasks
- `claude-2.1` - Previous generation
- `claude-2.0` - Previous generation

### Key Features

- Text completion and chat
- Function calling
- Streaming responses
- Vision capabilities (image analysis)
- Tool use
- Reasoning and analysis
- Code completion and analysis
- Refactoring support

## API Key Setup

### Step 1: Create an Anthropic Account

1. Visit [console.anthropic.com](https://console.anthropic.com)
2. Sign up for an account or log in
3. Complete any required verification steps

### Step 2: Generate an API Key

1. Navigate to **API Keys** in the console
2. Click **Create Key**
3. Give your key a descriptive name (e.g., "SuperAgent Production")
4. Copy the API key immediately - it will only be shown once

### Step 3: Store Your API Key Securely

Never commit your API key to version control. Use environment variables or a secrets manager.

## Environment Variable Configuration

Add the following to your `.env` file or environment:

```bash
# Required
CLAUDE_API_KEY=sk-ant-api03-xxxxxxxxxxxxxxxxxxxxx

# Optional - Override default settings
CLAUDE_BASE_URL=https://api.anthropic.com/v1/messages
CLAUDE_MODEL=claude-3-sonnet-20240229
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CLAUDE_API_KEY` | Yes | - | Your Anthropic API key |
| `CLAUDE_BASE_URL` | No | `https://api.anthropic.com/v1/messages` | API endpoint URL |
| `CLAUDE_MODEL` | No | `claude-3-sonnet-20240229` | Default model to use |

## Basic Usage Example

### Go Code Example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/superagent/superagent/internal/llm/providers/claude"
    "github.com/superagent/superagent/internal/models"
)

func main() {
    // Create provider
    provider := claude.NewClaudeProvider(
        os.Getenv("CLAUDE_API_KEY"),
        "", // Use default base URL
        "", // Use default model
    )

    // Create request
    req := &models.LLMRequest{
        ID: "request-1",
        Messages: []models.Message{
            {Role: "user", Content: "What is the capital of France?"},
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

### With System Prompt

```go
req := &models.LLMRequest{
    ID: "request-1",
    Messages: []models.Message{
        {Role: "system", Content: "You are a helpful coding assistant."},
        {Role: "user", Content: "Write a function to calculate factorial in Go."},
    },
    ModelParams: models.ModelParams{
        MaxTokens:   2048,
        Temperature: 0.5,
    },
}
```

## Rate Limits and Quotas

### Default Rate Limits

| Tier | Requests/Minute | Tokens/Minute | Tokens/Day |
|------|-----------------|---------------|------------|
| Free | 5 | 20,000 | 100,000 |
| Build | 50 | 100,000 | 500,000 |
| Scale | 1,000 | 400,000 | Unlimited |

### Model Context Limits

| Model | Max Input Tokens | Max Output Tokens |
|-------|-----------------|-------------------|
| Claude 3 Opus | 200,000 | 4,096 |
| Claude 3 Sonnet | 200,000 | 4,096 |
| Claude 3 Haiku | 200,000 | 4,096 |

### Best Practices for Rate Limits

1. **Implement exponential backoff** - SuperAgent automatically retries with backoff on 429 errors
2. **Monitor usage** - Track token consumption to avoid hitting limits
3. **Use appropriate models** - Use Haiku for simple tasks to save quota
4. **Batch requests** - Combine related queries when possible

## Troubleshooting

### Common Errors

#### Authentication Error (401)

```
Claude API error: 401 - {"error": {"type": "authentication_error", "message": "Invalid API key"}}
```

**Solution:**
- Verify your API key is correct
- Check that the key hasn't been revoked
- Ensure no extra whitespace in the environment variable

#### Rate Limit Error (429)

```
Claude API error: 429 - {"error": {"type": "rate_limit_error", "message": "Rate limit exceeded"}}
```

**Solution:**
- Wait and retry (SuperAgent handles this automatically)
- Upgrade your usage tier if hitting limits frequently
- Implement request queuing for high-volume applications

#### Overloaded Error (529)

```
Claude API error: 529 - {"error": {"type": "overloaded_error", "message": "API is temporarily overloaded"}}
```

**Solution:**
- Wait and retry (automatically handled)
- Consider using a different model if one is consistently overloaded

#### Invalid Request (400)

```
Claude API error: 400 - {"error": {"type": "invalid_request_error", "message": "..."}}
```

**Solution:**
- Check that your request format is correct
- Verify message roles are valid ("user", "assistant", "system")
- Ensure max_tokens is within limits

### Health Check

SuperAgent provides a health check endpoint for Claude:

```go
err := provider.HealthCheck()
if err != nil {
    fmt.Printf("Claude provider unhealthy: %v\n", err)
}
```

### Debug Logging

Enable debug logging to see request/response details:

```bash
export GIN_MODE=debug
export LOG_LEVEL=debug
```

### Connection Issues

If experiencing connection timeouts:

1. Check network connectivity to `api.anthropic.com`
2. Verify firewall rules allow outbound HTTPS
3. Consider increasing timeout (default is 60 seconds)

```go
// Custom retry configuration
retryConfig := claude.RetryConfig{
    MaxRetries:   5,
    InitialDelay: 2 * time.Second,
    MaxDelay:     60 * time.Second,
    Multiplier:   2.0,
}

provider := claude.NewClaudeProviderWithRetry(
    apiKey,
    baseURL,
    model,
    retryConfig,
)
```

## Additional Resources

- [Anthropic API Documentation](https://docs.anthropic.com/)
- [Claude Model Card](https://www.anthropic.com/claude)
- [API Reference](https://docs.anthropic.com/claude/reference)
- [Prompt Engineering Guide](https://docs.anthropic.com/claude/docs/prompt-engineering)
