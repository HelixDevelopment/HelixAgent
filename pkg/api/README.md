# pkg/api - HelixAgent Public API

**Module:** `dev.helix.agent/pkg/api`  
**Language:** Go 1.24+  
**Protocol:** gRPC with Protocol Buffers

## Overview

The `pkg/api` module provides the public gRPC API for HelixAgent, enabling external services and clients to interact with the ensemble LLM system programmatically. This package contains Protocol Buffer definitions and generated Go code for type-safe API communication.

## Features

- **gRPC API**: High-performance, streaming-capable RPC interface
- **Protocol Buffers**: Efficient binary serialization
- **Streaming Support**: Real-time response streaming for LLM completions
- **Ensemble Configuration**: Configure multi-provider ensemble parameters
- **Memory Integration**: Access to HelixAgent's memory system
- **Type Safety**: Generated Go structs from proto definitions

## Installation

```bash
go get dev.helix.agent/pkg/api
```

## API Overview

### Core Services

#### LLMFacade Service

The main service for LLM operations:

- `Complete`: Single-shot completion request
- `CompleteStream`: Streaming completion with real-time responses
- `GetCapabilities`: Retrieve provider capabilities
- `HealthCheck`: Service health verification

### Message Types

#### CompletionRequest

```protobuf
message CompletionRequest {
  string request_id = 1;
  string session_id = 2;
  string prompt = 3;
  repeated Message messages = 4;
  ModelParameters model_params = 5;
  EnsembleConfig ensemble_config = 6;
  bool memory_enhanced = 7;
  map<string, string> metadata = 8;
  string request_type = 9;  // code_generation, reasoning, tool_use
  int32 priority = 10;
  google.protobuf.Timestamp created_at = 11;
}
```

#### CompletionResponse

```protobuf
message CompletionResponse {
  string response_id = 1;
  string request_id = 2;
  string content = 3;
  repeated ProviderResponse provider_responses = 4;
  EnsembleResult ensemble_result = 5;
  UsageStats usage = 6;
  google.protobuf.Timestamp created_at = 7;
  google.protobuf.Duration latency = 8;
}
```

#### StreamingResponse

```protobuf
message StreamingResponse {
  oneof response {
    ContentChunk chunk = 1;
    CompletionResponse final = 2;
    ErrorResponse error = 3;
  }
}
```

## Usage Examples

### Basic Completion

```go
package main

import (
    "context"
    "log"
    
    api "dev.helix.agent/pkg/api"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    // Connect to HelixAgent
    conn, err := grpc.Dial("localhost:8112", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    
    client := api.NewLLMFacadeClient(conn)
    
    // Create completion request
    req := &api.CompletionRequest{
        RequestId: "req-001",
        Prompt:    "Explain the benefits of ensemble LLM systems",
        ModelParams: &api.ModelParameters{
            ModelId:    "gpt-4",
            MaxTokens:  500,
            Temperature: 0.7,
        },
        RequestType: "reasoning",
    }
    
    // Execute completion
    resp, err := client.Complete(context.Background(), req)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Response: %s", resp.Content)
}
```

### Streaming Completion

```go
func streamCompletion(client api.LLMFacadeClient) {
    req := &api.CompletionRequest{
        RequestId: "stream-001",
        Prompt:    "Write a Python function to calculate fibonacci numbers",
        RequestType: "code_generation",
    }
    
    stream, err := client.CompleteStream(context.Background(), req)
    if err != nil {
        log.Fatal(err)
    }
    
    for {
        resp, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Fatal(err)
        }
        
        switch r := resp.Response.(type) {
        case *api.StreamingResponse_Chunk:
            fmt.Print(r.Chunk.Content)
        case *api.StreamingResponse_Final:
            fmt.Printf("\n\nCompleted in %v\n", r.Final.Latency)
        case *api.StreamingResponse_Error:
            log.Printf("Error: %s", r.Error.Message)
        }
    }
}
```

### Ensemble Configuration

```go
func ensembleCompletion(client api.LLMFacadeClient) {
    req := &api.CompletionRequest{
        Prompt: "Analyze the trade-offs between microservices and monoliths",
        EnsembleConfig: &api.EnsembleConfig{
            Strategy:      api.EnsembleStrategy_CONFIDENCE_WEIGHTED,
            MinProviders:  3,
            MaxProviders:  5,
            Providers: []string{
                "openai/gpt-4",
                "anthropic/claude-3",
                "gemini/pro",
            },
            VotingMethod: api.VotingMethod_MAJORITY,
        },
    }
    
    resp, err := client.Complete(context.Background(), req)
    if err != nil {
        log.Fatal(err)
    }
    
    // Access individual provider responses
    for _, providerResp := range resp.ProviderResponses {
        log.Printf("Provider %s: confidence=%.2f", 
            providerResp.ProviderId, 
            providerResp.Confidence)
    }
    
    // Access ensemble aggregation
    log.Printf("Ensemble method: %s", resp.EnsembleResult.Method)
    log.Printf("Consensus level: %.2f", resp.EnsembleResult.ConsensusLevel)
}
```

### Memory-Enhanced Completion

```go
func memoryEnhancedCompletion(client api.LLMFacadeClient, sessionID string) {
    req := &api.CompletionRequest{
        SessionId:      sessionID,  // Links to existing conversation context
        Prompt:         "What were the key points from our previous discussion?",
        MemoryEnhanced: true,        // Enable memory retrieval
        RequestType:    "reasoning",
    }
    
    resp, err := client.Complete(context.Background(), req)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Response with memory context: %s", resp.Content)
}
```

## Configuration

### Client Options

```go
// With authentication
creds, err := credentials.NewClientTLSFromFile("server.crt", "")
if err != nil {
    log.Fatal(err)
}

conn, err := grpc.Dial("localhost:8112", grpc.WithTransportCredentials(creds))

// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := client.Complete(ctx, req)

// With retry policy
retryOpts := []grpc.DialOption{
    grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(
        grpc_retry.WithMax(3),
        grpc_retry.WithBackoff(grpc_retry.BackoffLinear(100*time.Millisecond)),
    )),
}
```

## API Reference

### Request Types

| Type | Description | Use Case |
|------|-------------|----------|
| `code_generation` | Generate code snippets | IDE integration, code assistants |
| `reasoning` | Complex reasoning tasks | Analysis, explanations |
| `tool_use` | Tool invocation requests | Agent systems, automation |
| `chat` | Conversational responses | Chatbots, assistants |

### Ensemble Strategies

| Strategy | Description |
|----------|-------------|
| `SINGLE_BEST` | Use highest-confidence provider |
| `CONFIDENCE_WEIGHTED` | Weight responses by confidence |
| `VOTING` | Simple majority voting |
| `RANK_AGGREGATION` | Combine rankings from providers |

### Voting Methods

| Method | Description |
|--------|-------------|
| `MAJORITY` | Most common response wins |
| `WEIGHTED` | Weighted by provider confidence |
| `BORDA_COUNT` | Rank-based scoring |
| `CONDORCET` | Pairwise comparison |

## Error Handling

```go
resp, err := client.Complete(ctx, req)
if err != nil {
    st, ok := status.FromError(err)
    if ok {
        switch st.Code() {
        case codes.InvalidArgument:
            log.Printf("Invalid request: %v", st.Message())
        case codes.Unavailable:
            log.Printf("Service unavailable: %v", st.Message())
        case codes.ResourceExhausted:
            log.Printf("Rate limit exceeded: %v", st.Message())
        default:
            log.Printf("Error: %v", st.Message())
        }
    }
}
```

## Performance Considerations

- **Connection Pooling**: Reuse gRPC connections
- **Streaming**: Use `CompleteStream` for long responses
- **Compression**: Enable gzip compression for large payloads
- **Timeouts**: Set appropriate context timeouts
- **Circuit Breaking**: Implement client-side circuit breakers

## Environment Variables

```bash
HELIXAGENT_API_HOST=localhost:8112
HELIXAGENT_API_TLS_CERT=/path/to/cert.pem
HELIXAGENT_API_TLS_KEY=/path/to/key.pem
HELIXAGENT_API_AUTH_TOKEN=your-api-token
```

## Development

### Regenerate Protobuf

```bash
# Install protoc and protoc-gen-go
# Regenerate Go code from .proto files
protoc --go_out=. --go-grpc_out=. llm-facade.proto
```

### Testing

```bash
go test ./...
```

## Integration

### With OpenAI-Compatible Clients

HelixAgent provides an OpenAI-compatible HTTP API alongside the gRPC API. Use the HTTP API for simpler integrations:

```bash
curl http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "helixagent-ensemble",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for contribution guidelines.

## License

MIT License - See [LICENSE](../../LICENSE) for details.

## Support

- Documentation: https://docs.helix.agent
- Issues: https://github.com/helixagent/helixagent/issues
- Discussions: https://github.com/helixagent/helixagent/discussions
