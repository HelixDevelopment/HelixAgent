# Gemini (Google) Provider Setup Guide

## Overview

Gemini is Google's multimodal AI model family, capable of understanding and generating text, code, images, and more. SuperAgent integrates with the Google AI Studio API to provide access to Gemini models for text and vision tasks.

### Supported Models

- `gemini-pro` - Text-only model for general tasks (default)
- `gemini-pro-vision` - Multimodal model supporting text and images
- `gemini-1.5-pro` - Latest generation with extended context
- `gemini-1.5-flash` - Faster, more efficient model

### Key Features

- Text completion and chat
- Function calling
- Streaming responses
- Vision capabilities (image understanding)
- Tool use support
- Reasoning capabilities
- Code completion and analysis

## API Key Setup

### Step 1: Create a Google Cloud Account

1. Visit [console.cloud.google.com](https://console.cloud.google.com)
2. Sign in with your Google account or create a new one
3. Set up billing (required for API access)

### Step 2: Enable the Generative AI API

1. Go to the [Google AI Studio](https://makersuite.google.com)
2. Accept the terms of service
3. Navigate to **Get API key**
4. Click **Create API key** or use an existing project

### Step 3: Generate an API Key

1. In Google AI Studio, click **Get API key**
2. Select **Create API key in new project** or choose existing project
3. Copy the generated API key

### Step 4: Store Your API Key Securely

```bash
# Add to your environment or .env file
export GEMINI_API_KEY=AIzaxxxxxxxxxxxxxxxxxxxxxxxxx
```

## Environment Variable Configuration

Add the following to your `.env` file or environment:

```bash
# Required
GEMINI_API_KEY=AIzaxxxxxxxxxxxxxxxxxxxxxxxxx

# Optional - Override default settings
GEMINI_BASE_URL=https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent
GEMINI_MODEL=gemini-pro
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GEMINI_API_KEY` | Yes | - | Your Google AI API key |
| `GEMINI_BASE_URL` | No | `https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent` | API endpoint URL template |
| `GEMINI_MODEL` | No | `gemini-pro` | Default model to use |

## Basic Usage Example

### Go Code Example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/superagent/superagent/internal/llm/providers/gemini"
    "github.com/superagent/superagent/internal/models"
)

func main() {
    // Create provider
    provider := gemini.NewGeminiProvider(
        os.Getenv("GEMINI_API_KEY"),
        "", // Use default base URL
        "", // Use default model
    )

    // Create request
    req := &models.LLMRequest{
        ID:     "request-1",
        Prompt: "You are a helpful assistant.",
        Messages: []models.Message{
            {Role: "user", Content: "Explain quantum computing in simple terms."},
        },
        ModelParams: models.ModelParams{
            MaxTokens:   2048,
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

### Multi-turn Conversation

```go
req := &models.LLMRequest{
    ID: "request-1",
    Messages: []models.Message{
        {Role: "user", Content: "What is the capital of Japan?"},
        {Role: "assistant", Content: "The capital of Japan is Tokyo."},
        {Role: "user", Content: "What is the population?"},
    },
    ModelParams: models.ModelParams{
        MaxTokens:   1024,
        Temperature: 0.7,
    },
}
```

## Rate Limits and Quotas

### Default Rate Limits (Google AI Studio)

| Plan | Requests/Minute | Tokens/Minute |
|------|-----------------|---------------|
| Free | 60 | 60,000 |
| Pay-as-you-go | 360 | 120,000 |
| Enterprise | Custom | Custom |

### Model Context Limits

| Model | Max Input Tokens | Max Output Tokens |
|-------|-----------------|-------------------|
| Gemini Pro | 32,768 | 8,192 |
| Gemini Pro Vision | 16,384 | 4,096 |
| Gemini 1.5 Pro | 1,000,000 | 8,192 |
| Gemini 1.5 Flash | 1,000,000 | 8,192 |

### Best Practices for Rate Limits

1. **Use exponential backoff** - SuperAgent automatically implements retry with backoff
2. **Monitor quota usage** - Check the Google Cloud Console for usage statistics
3. **Use appropriate models** - Use Flash for speed-critical applications
4. **Implement caching** - Cache responses for repeated queries

## Safety Settings

Gemini includes built-in safety filters. SuperAgent configures these settings by default:

```go
SafetySettings: []GeminiSafetySetting{
    {Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
    {Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
    {Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
    {Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
}
```

Available thresholds:
- `BLOCK_NONE` - Don't block any content
- `BLOCK_LOW_AND_ABOVE` - Block low probability and higher
- `BLOCK_MEDIUM_AND_ABOVE` - Block medium probability and higher
- `BLOCK_HIGH_AND_ABOVE` - Only block high probability content

## Troubleshooting

### Common Errors

#### Invalid API Key (400)

```
Gemini API error: 400 - {"error": {"message": "API key not valid"}}
```

**Solution:**
- Verify your API key is correct
- Ensure the key was created in Google AI Studio
- Check that the Generative AI API is enabled

#### Quota Exceeded (429)

```
Gemini API error: 429 - {"error": {"message": "Resource exhausted"}}
```

**Solution:**
- Wait for the rate limit to reset (usually 1 minute)
- SuperAgent automatically retries with exponential backoff
- Consider upgrading to a paid plan

#### Safety Filter Triggered (400)

```
Gemini API error: 400 - {"error": {"message": "Response blocked due to safety"}}
```

**Solution:**
- Review your prompt for potentially sensitive content
- Adjust safety settings if appropriate for your use case
- Rephrase the request

#### Model Not Found (404)

```
Gemini API error: 404 - {"error": {"message": "Model not found"}}
```

**Solution:**
- Verify the model name is correct
- Check that the model is available in your region
- Use `gemini-pro` as a fallback

### Health Check

SuperAgent provides a health check endpoint for Gemini:

```go
err := provider.HealthCheck()
if err != nil {
    fmt.Printf("Gemini provider unhealthy: %v\n", err)
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

1. Check network connectivity to `generativelanguage.googleapis.com`
2. Verify the API is accessible from your region
3. Check Google Cloud status page for outages
4. Consider increasing timeout (default is 60 seconds)

```go
// Custom retry configuration
retryConfig := gemini.RetryConfig{
    MaxRetries:   5,
    InitialDelay: 2 * time.Second,
    MaxDelay:     60 * time.Second,
    Multiplier:   2.0,
}

provider := gemini.NewGeminiProviderWithRetry(
    apiKey,
    baseURL,
    model,
    retryConfig,
)
```

### Region Availability

Gemini API may not be available in all regions. If you encounter access issues:

1. Check [Google AI availability](https://ai.google.dev/available_regions)
2. Use a VPN to test from a supported region
3. Consider using Vertex AI for enterprise deployments in restricted regions

## Vertex AI Alternative

For enterprise deployments, consider using Gemini through Google Cloud Vertex AI:

```bash
# Vertex AI configuration
export GCP_PROJECT_ID=your-project-id
export GCP_LOCATION=us-central1
export GOOGLE_ACCESS_TOKEN=your-access-token
```

See the [GCP Vertex AI guide](./gcp-vertex-ai.md) for more details.

## Additional Resources

- [Google AI Studio](https://makersuite.google.com)
- [Gemini API Documentation](https://ai.google.dev/docs)
- [Gemini Model Information](https://deepmind.google/technologies/gemini/)
- [API Reference](https://ai.google.dev/api/rest/v1beta/models)
- [Prompt Design Guide](https://ai.google.dev/docs/prompt_best_practices)
