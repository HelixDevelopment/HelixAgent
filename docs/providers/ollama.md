# Ollama Provider Setup Guide

## Overview

Ollama is an open-source platform for running large language models locally on your own hardware. SuperAgent integrates with Ollama to provide access to a wide variety of open-source models without requiring cloud API keys.

### Supported Models

Ollama supports many open-source models including:

- `llama2` - Meta's Llama 2 base model (default)
- `llama2:13b` - Llama 2 13B parameter version
- `llama2:70b` - Llama 2 70B parameter version
- `codellama` - Code-specialized Llama model
- `mistral` - Mistral 7B model
- `vicuna` - Fine-tuned Llama for chat
- `orca-mini` - Smaller, efficient model

You can also use any model available in the [Ollama Library](https://ollama.ai/library).

### Key Features

- Run models locally with no API keys required
- Text completion and chat
- Streaming responses
- Privacy - data never leaves your machine
- Support for custom models and fine-tunes
- GPU acceleration support

## Installation

### Step 1: Install Ollama

#### Linux

```bash
curl -fsSL https://ollama.ai/install.sh | sh
```

#### macOS

```bash
brew install ollama
```

Or download from [ollama.ai/download](https://ollama.ai/download)

#### Windows

Download the installer from [ollama.ai/download](https://ollama.ai/download)

### Step 2: Start the Ollama Service

```bash
# Start Ollama service
ollama serve
```

The service will start on `http://localhost:11434` by default.

### Step 3: Pull a Model

```bash
# Pull the default llama2 model
ollama pull llama2

# Or pull a specific model
ollama pull codellama
ollama pull mistral
```

### Step 4: Verify Installation

```bash
# List available models
ollama list

# Test a model
ollama run llama2 "Hello, how are you?"
```

## Environment Variable Configuration

Add the following to your `.env` file or environment:

```bash
# Enable Ollama provider
OLLAMA_ENABLED=true

# Optional - Override default settings
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama2
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OLLAMA_ENABLED` | No | `false` | Enable/disable Ollama provider |
| `OLLAMA_BASE_URL` | No | `http://localhost:11434` | Ollama API endpoint |
| `OLLAMA_MODEL` | No | `llama2` | Default model to use |

## Basic Usage Example

### Go Code Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/superagent/superagent/internal/llm/providers/ollama"
    "github.com/superagent/superagent/internal/models"
)

func main() {
    // Create provider (no API key needed for local Ollama)
    provider := ollama.NewOllamaProvider(
        "", // Use default base URL (localhost:11434)
        "", // Use default model (llama2)
    )

    // Create request
    req := &models.LLMRequest{
        ID:     "request-1",
        Prompt: "Explain what a neural network is in simple terms.",
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

### Using a Specific Model

```go
provider := ollama.NewOllamaProvider(
    "http://localhost:11434",
    "codellama", // Use CodeLlama for code tasks
)

req := &models.LLMRequest{
    ID:     "request-1",
    Prompt: "Write a Python function to calculate Fibonacci numbers.",
    ModelParams: models.ModelParams{
        MaxTokens:   2048,
        Temperature: 0.2, // Lower temperature for code
    },
}
```

## Docker Deployment

### Running Ollama in Docker

```bash
# Run Ollama with GPU support (NVIDIA)
docker run -d \
    --gpus all \
    -v ollama:/root/.ollama \
    -p 11434:11434 \
    --name ollama \
    ollama/ollama

# Run without GPU (CPU only)
docker run -d \
    -v ollama:/root/.ollama \
    -p 11434:11434 \
    --name ollama \
    ollama/ollama
```

### Docker Compose

Add to your `docker-compose.yml`:

```yaml
services:
  ollama:
    image: ollama/ollama
    ports:
      - "11434:11434"
    volumes:
      - ollama:/root/.ollama
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]

volumes:
  ollama:
```

### Pull Models in Docker

```bash
# Execute in the running container
docker exec -it ollama ollama pull llama2
docker exec -it ollama ollama pull codellama
```

## Rate Limits and Quotas

Since Ollama runs locally, there are no API rate limits. However, performance depends on your hardware:

### Hardware Requirements

| Model Size | RAM Required | GPU VRAM | Performance |
|------------|-------------|----------|-------------|
| 7B params | 8GB | 6GB | Fast |
| 13B params | 16GB | 12GB | Medium |
| 70B params | 64GB | 48GB+ | Slow |

### Concurrent Request Limits

By default, Ollama processes one request at a time. SuperAgent sets `MaxConcurrentRequests: 1` for the Ollama provider.

### Best Practices for Performance

1. **Use appropriate model sizes** - Smaller models are faster
2. **Enable GPU acceleration** - Much faster than CPU-only
3. **Allocate sufficient RAM** - Models are loaded into memory
4. **Use SSDs** - Faster model loading
5. **Consider quantized models** - Lower precision for faster inference

## Troubleshooting

### Common Errors

#### Connection Refused

```
Ollama API returned status 0: dial tcp 127.0.0.1:11434: connect: connection refused
```

**Solution:**
- Start the Ollama service: `ollama serve`
- Check if Ollama is running: `curl http://localhost:11434/api/tags`
- Verify the base URL configuration

#### Model Not Found

```
Ollama API returned status 404: model 'mistral' not found
```

**Solution:**
- Pull the model first: `ollama pull mistral`
- List available models: `ollama list`
- Check model name spelling

#### Out of Memory

```
error: out of memory
```

**Solution:**
- Use a smaller model (e.g., `llama2:7b` instead of `llama2:70b`)
- Close other applications to free RAM
- Use a quantized model version (e.g., `llama2:7b-q4_0`)
- Upgrade hardware (add more RAM or GPU VRAM)

#### Slow Response Times

```
context deadline exceeded
```

**Solution:**
- Increase timeout (default is 120 seconds for Ollama)
- Use a smaller/faster model
- Enable GPU acceleration
- Consider using a more powerful machine

### Health Check

SuperAgent provides a health check endpoint for Ollama:

```go
err := provider.HealthCheck()
if err != nil {
    fmt.Printf("Ollama provider unhealthy: %v\n", err)
}
```

The health check queries the `/api/tags` endpoint to verify connectivity.

### Debug Logging

Enable debug logging to troubleshoot issues:

```bash
export GIN_MODE=debug
export LOG_LEVEL=debug
```

### Network Configuration

If running Ollama on a different host:

```bash
# On the Ollama host, bind to all interfaces
OLLAMA_HOST=0.0.0.0 ollama serve

# Configure SuperAgent to connect to remote Ollama
export OLLAMA_BASE_URL=http://remote-host:11434
```

### Custom Retry Configuration

```go
retryConfig := ollama.RetryConfig{
    MaxRetries:   3,
    InitialDelay: 2 * time.Second,
    MaxDelay:     60 * time.Second,
    Multiplier:   2.0,
}

provider := ollama.NewOllamaProviderWithRetry(
    baseURL,
    model,
    retryConfig,
)
```

## Model Management

### Listing Models

```bash
ollama list
```

### Pulling Models

```bash
# Pull from Ollama library
ollama pull llama2

# Pull specific version
ollama pull llama2:13b

# Pull quantized version (smaller, faster)
ollama pull llama2:7b-q4_0
```

### Removing Models

```bash
ollama rm llama2
```

### Creating Custom Models

Create a Modelfile:

```
FROM llama2

PARAMETER temperature 0.7
PARAMETER num_ctx 4096

SYSTEM You are a helpful coding assistant.
```

Build the model:

```bash
ollama create my-assistant -f Modelfile
```

## Popular Models

| Model | Size | Best For |
|-------|------|----------|
| llama2 | 7B | General chat, reasoning |
| codellama | 7B | Code generation |
| mistral | 7B | Fast, high-quality responses |
| mixtral | 47B | Complex tasks (requires high RAM) |
| phi | 2.7B | Fast, efficient responses |
| neural-chat | 7B | Conversational AI |

## Additional Resources

- [Ollama Documentation](https://github.com/ollama/ollama)
- [Ollama Model Library](https://ollama.ai/library)
- [Ollama Blog](https://ollama.ai/blog)
- [Ollama Discord Community](https://discord.gg/ollama)
