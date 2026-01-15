# HelixAgent Feature Flags System

The Feature Flags system provides a flexible way to toggle features like GraphQL, TOON encoding, streaming types, compression algorithms, and transport protocols based on CLI agent capabilities and user preferences.

## Overview

HelixAgent supports 18+ CLI agents with varying capabilities. The feature flags system ensures:

1. **Backward Compatibility**: All existing CLI agents continue to work with default settings
2. **Flexibility**: Users can enable/disable features via headers or query parameters
3. **Automatic Detection**: Agent capabilities are detected from User-Agent headers
4. **Validation**: Feature combinations are validated to prevent conflicts

## Available Features

### Transport Features

| Feature | Default | Description |
|---------|---------|-------------|
| `graphql` | OFF | GraphQL API endpoint for flexible data fetching |
| `toon` | OFF | Token-Optimized Object Notation for efficient AI consumption |
| `http2` | ON | HTTP/2 multiplexing and server push support |
| `http3` | OFF | HTTP/3 with QUIC transport (limited client support) |
| `websocket` | ON | WebSocket-based bidirectional streaming |
| `sse` | ON | Server-Sent Events for real-time updates |
| `jsonl` | ON | JSON Lines format for streaming responses |

### Compression Features

| Feature | Default | Description |
|---------|---------|-------------|
| `brotli` | OFF | Brotli compression (better ratio, limited support) |
| `gzip` | ON | Gzip compression (universally supported) |
| `zstd` | OFF | Zstandard compression (high ratio, limited support) |

### Protocol Features

| Feature | Default | Description |
|---------|---------|-------------|
| `mcp` | ON | Model Context Protocol for tool/context sharing |
| `acp` | ON | Agent Communication Protocol |
| `lsp` | ON | Language Server Protocol for IDE integration |
| `grpc` | OFF | gRPC for high-performance RPC |

### API Features

| Feature | Default | Description |
|---------|---------|-------------|
| `embeddings` | ON | Vector embeddings generation API |
| `vision` | ON | Image analysis and OCR capabilities |
| `cognee` | OFF | Knowledge graph and RAG integration |
| `debate` | ON | Multi-model AI Debate system |
| `batch` | ON | Batch request support |
| `tool_calling` | ON | Function/tool calling support |

### Advanced Features

| Feature | Default | Description |
|---------|---------|-------------|
| `multipass` | OFF | Multi-pass response validation (requires `debate`) |
| `caching` | ON | Response caching for repeated queries |
| `rate_limiting` | ON | Request rate limiting per client |
| `metrics` | ON | Prometheus metrics endpoint |
| `tracing` | OFF | Distributed tracing with OpenTelemetry |

## Enabling Features

### Via HTTP Headers

Individual feature headers:
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Feature-GraphQL: true" \
  -H "X-Feature-TOON: true" \
  -d '{"messages": [{"role": "user", "content": "Hello"}]}'
```

Compact feature header:
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Features: graphql,toon,-sse,brotli=true" \
  -d '{"messages": [{"role": "user", "content": "Hello"}]}'
```

### Via Query Parameters

Individual query parameters:
```bash
curl "http://localhost:8080/v1/chat/completions?graphql=true&toon=true"
```

Compact features parameter:
```bash
curl "http://localhost:8080/v1/chat/completions?features=graphql,toon,!sse"
```

### Feature Values

Features accept various boolean representations:
- Enable: `true`, `1`, `yes`, `on`, `enabled`
- Disable: `false`, `0`, `no`, `off`, `disabled`
- Compact disable: `!feature`, `-feature`

## CLI Agent Capabilities

### Full-Feature Agents (All Features Supported)

**HelixCode** is the only agent that supports all advanced features:
- GraphQL + TOON encoding
- HTTP/3 QUIC transport
- Brotli + Zstd compression
- All protocols (MCP, ACP, LSP, gRPC)
- Multi-pass validation
- Distributed tracing

### Standard Agents (HTTP/2 + SSE + Gzip)

Most CLI agents support:
- HTTP/2 transport
- SSE and JSONL streaming
- Gzip compression
- Basic API features

Agents in this category:
- OpenCode
- ClaudeCode
- Aider
- Kiro
- Crush
- DeepSeekCLI
- GeminiCLI
- MistralCode
- OllamaCode
- Plandex
- QwenCode
- GPTEngineer
- CodenameGoose

### Enhanced Agents (Additional Features)

**KiloCode**, **Forge**, **AmazonQ**, **Cline**:
- WebSocket streaming
- Brotli compression
- MCP/gRPC protocols (varies)
- Batch requests

## Agent Detection

HelixAgent automatically detects CLI agents from the User-Agent header:

```bash
# OpenCode
curl -H "User-Agent: OpenCode/1.0" ...

# HelixCode
curl -H "User-Agent: HelixCode/1.0" ...

# ClaudeCode
curl -H "User-Agent: claude-code/1.0" ...
```

When an agent is detected, its default features are automatically applied.

## Response Headers

HelixAgent adds informative headers to responses:

```
X-Features-Enabled: http2,sse,gzip,mcp,embeddings
X-Transport-Protocol: h2
X-Compression-Available: gzip
X-Streaming-Method: sse
X-Agent-Detected: opencode
```

## Configuration

### Server Configuration

Configure feature defaults in your server initialization:

```go
import "dev.helix.agent/internal/features"

// Create custom configuration
config := features.DefaultFeatureConfig()
config.OpenAIEndpointGraphQL = true  // Enable GraphQL for OpenAI endpoints
config.AllowFeatureHeaders = true     // Allow header-based overrides
config.StrictValidation = false       // Be lenient with feature combinations

// Create middleware
middleware := features.Middleware(&features.MiddlewareConfig{
    Config:               config,
    EnableAgentDetection: true,
    StrictMode:           false,
    TrackUsage:           true,
})

router.Use(middleware)
```

### Endpoint-Specific Defaults

Set different defaults for specific endpoints:

```go
registry := features.GetRegistry()
registry.SetEndpointDefaults("/v1/graphql", map[features.Feature]bool{
    features.FeatureGraphQL: true,
    features.FeatureTOON:    true,
})
```

## Feature Validation

Some features have dependencies or conflicts:

### Dependencies
- `multipass` requires `debate` to be enabled

### Conflicts
- `http2` and `http3` cannot be enabled simultaneously

When strict validation is enabled, requests with invalid feature combinations are rejected:

```json
{
  "error": "Invalid feature combination: feature http3: conflicts with feature: http2"
}
```

## Programmatic Access

### Check Feature in Handler

```go
func MyHandler(c *gin.Context) {
    fc := features.GetFeatureContextFromGin(c)

    if fc.IsEnabled(features.FeatureGraphQL) {
        // Handle GraphQL request
    }

    // Or use the convenience function
    if features.IsFeatureEnabled(c, features.FeatureTOON) {
        // Encode response with TOON
    }
}
```

### Require Feature

```go
// Require a specific feature
router.GET("/graphql", features.RequireFeature(features.FeatureGraphQL), graphqlHandler)

// Require any of the listed features
router.GET("/stream", features.RequireAnyFeature(features.FeatureSSE, features.FeatureWebSocket), streamHandler)

// Conditional middleware
router.Use(features.ConditionalMiddleware(features.FeatureBrotli, brotliMiddleware))
```

## Usage Tracking

Feature usage is tracked automatically:

```go
tracker := features.GetUsageTracker()
stats := tracker.GetStats()

for _, stat := range stats {
    fmt.Printf("Feature %s: %d enabled, %d disabled, %d total\n",
        stat.Feature, stat.EnabledCount, stat.DisabledCount, stat.TotalRequests)
}
```

## Best Practices

1. **Default to Backward Compatibility**: Keep advanced features disabled by default
2. **Use Agent Detection**: Let the system auto-detect capabilities
3. **Validate Combinations**: Enable strict mode for production
4. **Monitor Usage**: Track feature usage for optimization decisions
5. **Document API Changes**: When enabling new features, update your API documentation

## Migration Guide

### Enabling GraphQL for All Requests

```go
config := features.DefaultFeatureConfig()
config.GlobalDefaults[features.FeatureGraphQL] = true
config.GlobalDefaults[features.FeatureTOON] = true
```

### Enabling HTTP/3 for Specific Clients

HTTP/3 requires client support. HelixCode is currently the only supported agent:

```go
// Server-side: HTTP/3 is auto-enabled for HelixCode
// Client-side: Use HTTP/3-capable client with proper User-Agent

curl --http3 -H "User-Agent: HelixCode/1.0" ...
```

## Troubleshooting

### Feature Not Working

1. Check if the feature is enabled: `X-Features-Enabled` header
2. Verify User-Agent is being detected: `X-Agent-Detected` header
3. Check for validation errors in response

### Agent Not Detected

Ensure your User-Agent contains a recognizable pattern:
- `HelixCode`, `helix-code`, `helix_code`
- `OpenCode`, `open-code`
- `claude-code`, `Claude Code`
- etc.

### Compression Not Applied

Compression requires:
1. Feature enabled (`brotli`, `gzip`, or `zstd`)
2. `Accept-Encoding` header from client
3. Response size > 1024 bytes (configurable)
4. Compressible content type
