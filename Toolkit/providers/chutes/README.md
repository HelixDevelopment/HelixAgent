# Chutes AI Provider

This directory contains the complete implementation of the Chutes AI provider for the AI Toolkit, following the same pattern as SiliconFlow and other providers.

## Overview

The Chutes provider implements the full `Provider` interface with support for:
- **Chat Completions**: Text generation with various parameters
- **Embeddings**: Text-to-vector conversion
- **Reranking**: Document relevance scoring
- **Model Discovery**: Automatic model detection and capability inference

## Architecture

The implementation consists of four main components:

### 1. Provider Core (`chutes.go`)
- Implements the `Provider` interface
- Handles configuration management
- Provides factory functions for provider creation
- **Auto-registration**: Includes `init()` function for automatic registration

### 2. Configuration Builder (`builder.go`)
- Implements the `ConfigBuilder` interface
- Handles configuration validation and merging
- Supports all standard provider configuration options
- Type-safe configuration extraction

### 3. HTTP Client (`client.go`)
- Complete Chutes API client implementation
- Supports all three API endpoints: `/chat/completions`, `/embeddings`, `/rerank`
- Proper error handling and request/response management
- Configurable base URL support

### 4. Model Discovery (`discovery.go`)
- Comprehensive capability inference for Chutes models
- Model-specific logic for Qwen, DeepSeek, GLM, Kimi models
- Context window and max tokens inference
- Human-readable model name formatting

## Configuration

### Environment Variables
```bash
export CHUTES_API_KEY="your-chutes-api-key-here"
```

### Configuration File
```json
{
  "name": "chutes",
  "api_key": "your-chutes-api-key-here",
  "base_url": "https://api.chutes.ai/v1",
  "timeout": 30000,
  "retries": 3,
  "rate_limit": 60
}
```

### Configuration Options
- `api_key`: Your Chutes API key (required)
- `base_url`: Chutes API base URL (default: `https://api.chutes.ai/v1`)
- `timeout`: Request timeout in milliseconds (default: 30000)
- `retries`: Number of retry attempts (default: 3)
- `rate_limit`: Rate limit in requests per minute (default: 60)

## Usage

### CLI Usage

```bash
# List providers (Chutes should appear)
./toolkit list providers

# Generate Chutes configuration
./toolkit config generate provider chutes

# Validate Chutes configuration
./toolkit validate provider chutes provider-chutes-config.json

# Discover models from Chutes
./toolkit discover chutes

# Use Chutes with an agent
./toolkit execute generic "Hello world" --provider chutes --model qwen2.5-7b-instruct
```

### Programmatic Usage

```go
import (
    "github.com/superagent/toolkit/pkg/toolkit"
    _ "github.com/superagent/toolkit/providers/chutes" // Auto-registration
)

// Create toolkit instance
tk := toolkit.NewToolkit()

// Create Chutes provider
config := map[string]interface{}{
    "api_key": "your-api-key",
    "base_url": "https://api.chutes.ai/v1",
}

provider, err := tk.CreateProvider("chutes", config)
if err != nil {
    log.Fatal(err)
}

// Use the provider
ctx := context.Background()
response, err := provider.Chat(ctx, toolkit.ChatRequest{
    Model: "qwen2.5-7b-instruct",
    Messages: []toolkit.Message{
        {Role: "user", Content: "Hello, world!"},
    },
})
```

## Supported Models

The provider supports automatic capability inference for Chutes-hosted models:

### Chat Models
- Qwen series (Qwen2.5, Qwen3)
- DeepSeek series (DeepSeek-V3, DeepSeek-R1)
- GLM series (GLM-4)
- Kimi models

### Embedding Models
- Various embedding models hosted on Chutes

### Rerank Models
- Various rerank models hosted on Chutes

### Specialized Models
- Vision models (with VL suffix)
- Audio models (TTS, speech)
- Video models (T2V, I2V)

## Model Capabilities

The discovery system automatically infers model capabilities:

- **Chat Support**: Based on model type and ID patterns
- **Embedding Support**: Models with "embedding" in type
- **Rerank Support**: Models with "rerank" in type
- **Vision Support**: Models with vision/multimodal keywords
- **Audio Support**: Models with TTS/audio/speech keywords
- **Video Support**: Models with video/T2V/I2V keywords
- **Function Calling**: Supported for Qwen, DeepSeek, GLM, Kimi models

## Context Windows

Automatic context window inference:
- DeepSeek models: 131,072 tokens
- Qwen models: 32,768 tokens (some variants: 131,072)
- GLM models: 32,768 tokens (GLM-4.6: 131,072)
- Kimi models: 131,072 tokens
- Default: 4,096 tokens

## Error Handling

The provider implements comprehensive error handling:
- Configuration validation errors
- API request/response errors
- Network timeout handling
- Rate limit handling

## Testing

Run the provider tests:
```bash
go test ./providers/chutes/...
```

## Integration

The provider integrates seamlessly with the toolkit:
- Auto-registration via `init()` function
- Environment variable support (`CHUTES_API_KEY`)
- Configuration file support
- CLI integration
- Agent compatibility

## Comparison with SiliconFlow

The Chutes implementation follows the exact same patterns as SiliconFlow:
- Same interface implementations
- Same configuration structure
- Same factory and registration patterns
- Same CLI integration approach
- Same testing methodology

This ensures consistency across all providers in the toolkit.