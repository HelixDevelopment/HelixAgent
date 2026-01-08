# AI Coding Agent Compatibility Guide

HelixAgent provides full compatibility with OpenCode, Crush, and HelixCode AI coding assistants through its OpenAI-compatible API.

## Supported Agents

| Agent | Status | Configuration Type |
|-------|--------|-------------------|
| [OpenCode](https://github.com/anomalyco/opencode) | Fully Compatible | `@ai-sdk/openai-compatible` |
| [Crush](https://github.com/charmbracelet/crush) | Fully Compatible | `openai-compat` |
| [HelixCode](https://github.com/HelixDevelopment/HelixCode) | Fully Compatible | `openai-compatible` |

## Streaming Compatibility

HelixAgent implements the OpenAI streaming specification exactly as required by these agents:

### Chunk Format

```json
{
  "id": "chatcmpl-1234567890",
  "object": "chat.completion.chunk",
  "created": 1767779259,
  "model": "helixagent-ensemble",
  "system_fingerprint": "fp_helixagent_v1",
  "choices": [{
    "index": 0,
    "delta": {
      "role": "assistant",  // Only in first chunk
      "content": "Hello"
    },
    "logprobs": null,
    "finish_reason": null  // null for intermediate, "stop" for final
  }]
}
```

### Streaming Requirements

| Requirement | Implementation | Agent |
|------------|----------------|-------|
| First chunk has role | `delta.role: "assistant"` | OpenCode, Crush, HelixCode |
| Subsequent chunks: no role | `delta: {content: "..."}` | OpenCode |
| Consistent stream ID | Same `id` across all chunks | OpenCode, Crush |
| `finish_reason: null` for intermediate | Explicit null value | OpenCode |
| `finish_reason: "stop"` for final | Final chunk before [DONE] | All |
| `system_fingerprint` field | Present in all chunks | OpenCode |
| `logprobs: null` field | Present in all chunks | OpenCode |
| Skip empty content chunks | Don't send `content: ""` | OpenCode (Issue #2840) |
| No empty `tool_calls` array | Never include `tool_calls: []` | OpenCode (Issue #4255) |
| `[DONE]` marker | `data: [DONE]\n\n` at end | All |

### SSE Format

```
data: {"id":"...","choices":[...]}\n\n
data: {"id":"...","choices":[...]}\n\n
data: [DONE]\n\n
```

## Configuration Generation

HelixAgent provides a configuration generator for all three agents:

### Go API

```go
import "dev.helix.agent/internal/config"

// Create generator
gen := config.NewConfigGenerator(
    "http://localhost:7061/v1",
    "your-api-key",
    "helixagent-ensemble",
)
gen.SetTimeout(120).SetMaxTokens(8192)

// Generate for each agent type
openCodeConfig, _ := gen.GenerateOpenCodeConfig()
crushConfig, _ := gen.GenerateCrushConfig()
helixCodeConfig, _ := gen.GenerateHelixCodeConfig()

// Or generate JSON directly
jsonData, _ := gen.GenerateJSON(config.AgentTypeOpenCode)
```

### Configuration Validation

```go
validator := config.NewConfigValidator()

// Validate typed config
result := validator.ValidateOpenCodeConfig(config)
if !result.Valid {
    for _, err := range result.Errors {
        log.Printf("Error: %s", err)
    }
}

// Validate JSON
result, err := validator.ValidateJSON(config.AgentTypeOpenCode, jsonData)
```

## Agent Configuration Examples

### OpenCode Configuration

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "helixagent": {
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "baseURL": "http://localhost:7061/v1",
        "apiKey": "your-api-key",
        "timeout": 120000
      }
    }
  }
}
```

### Crush Configuration

```json
{
  "$schema": "https://charm.land/crush.json",
  "providers": {
    "helixagent": {
      "type": "openai-compat",
      "base_url": "http://localhost:7061/v1",
      "api_key": "your-api-key",
      "models": [{
        "id": "helixagent-ensemble",
        "name": "HelixAgent AI Debate Ensemble",
        "context_window": 128000,
        "default_max_tokens": 8192
      }]
    }
  }
}
```

### HelixCode Configuration

```json
{
  "providers": {
    "helixagent": {
      "type": "openai-compatible",
      "base_url": "http://localhost:7061/v1",
      "api_key": "your-api-key",
      "model": "helixagent-ensemble",
      "max_tokens": 8192,
      "timeout": 120
    }
  },
  "settings": {
    "default_provider": "helixagent",
    "streaming_enabled": true,
    "auto_save": true
  }
}
```

## MCP Server Configuration

### OpenCode MCP

```json
{
  "mcp": {
    "helixagent-mcp": {
      "type": "remote",
      "url": "http://localhost:7061/mcp",
      "headers": {
        "Authorization": "Bearer your-api-key"
      }
    }
  }
}
```

### Crush MCP

```json
{
  "mcp": {
    "helixagent-mcp": {
      "type": "http",
      "url": "http://localhost:7061/mcp",
      "timeout": 30,
      "headers": {
        "Authorization": "Bearer your-api-key"
      }
    }
  }
}
```

## Troubleshooting

### OpenCode Spinner Never Stops

**Cause**: Missing `finish_reason: "stop"` in final chunk.

**Fix**: HelixAgent now sends a proper final chunk with `finish_reason: "stop"` before the `[DONE]` marker.

### Content Repeating in Loop

**Cause**: Role included in every chunk instead of only the first.

**Fix**: HelixAgent now only includes `delta.role: "assistant"` in the first chunk.

### Connection Reset After 30 Seconds

**Cause**: No idle timeout handling.

**Fix**: HelixAgent implements a 30-second idle timeout that gracefully closes the stream with `[DONE]`.

### Empty Events Cause Issues (OpenCode Issue #2840)

**Cause**: Empty content chunks sent to client.

**Fix**: HelixAgent filters out chunks with empty content before sending.

### Empty tool_calls Array (OpenCode Issue #4255)

**Cause**: `tool_calls: []` sent in chunks.

**Fix**: HelixAgent never includes an empty `tool_calls` array in streaming responses.

## Testing Compatibility

### Manual Test

```bash
curl -s http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "helixagent-ensemble", "messages": [{"role": "user", "content": "Say hi"}], "stream": true}'
```

Expected output:
1. First chunk: `delta` with `role` and empty `content`
2. Content chunks: `delta` with `content` only
3. Final chunk: `finish_reason: "stop"`
4. Stream end: `data: [DONE]`

### Automated Tests

```bash
# Run all streaming compatibility tests
go test -v ./internal/handlers/... -run "Streaming"

# Run config generator tests
go test -v ./internal/config/...
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Chat completions (streaming supported) |
| `/v1/completions` | POST | Text completions (streaming supported) |
| `/v1/models` | GET | List available models |
| `/health` | GET | Health check |

## Model Names

HelixAgent exposes a single virtual model that combines multiple LLM providers:

- **Model ID**: `helixagent-ensemble`
- **Display Name**: HelixAgent AI Debate Ensemble
- **Context Window**: 128,000 tokens
- **Max Output Tokens**: 8,192 tokens (configurable)

## Environment Variables

```bash
# Server configuration
PORT=8080
GIN_MODE=release

# API authentication (optional)
API_KEY=your-secret-key

# Provider API keys (at least one required)
GEMINI_API_KEY=...
DEEPSEEK_API_KEY=...
CLAUDE_API_KEY=...
MISTRAL_API_KEY=...
```

## Version Compatibility

| HelixAgent | OpenCode | Crush | HelixCode |
|------------|----------|-------|-----------|
| 1.0.0+ | 0.1.0+ | 0.1.0+ | 0.1.0+ |

## References

- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat/create)
- [OpenAI Streaming Guide](https://platform.openai.com/docs/api-reference/streaming)
- [OpenCode GitHub](https://github.com/anomalyco/opencode)
- [Crush GitHub](https://github.com/charmbracelet/crush)
- [HelixCode GitHub](https://github.com/HelixDevelopment/HelixCode)
