# HelixAgent gRPC API Documentation

## Introduction

HelixAgent provides a gRPC interface alongside its REST API, enabling high-performance, strongly-typed communication for LLM operations. The gRPC server implements two services: `LLMFacade` for ensemble-level operations with session and provider management, and `LLMProvider` for direct single-provider access. Both services support unary and server-streaming RPCs.

---

## Table of Contents

1. [Connection Setup](#connection-setup)
2. [Services Overview](#services-overview)
3. [LLMFacade Service](#llmfacade-service)
4. [LLMProvider Service](#llmprovider-service)
5. [Protobuf Definitions](#protobuf-definitions)
6. [Examples](#examples)
7. [Error Handling](#error-handling)
8. [Configuration](#configuration)

---

## Connection Setup

The gRPC server runs on a separate port from the HTTP API (default: `8112`). Connect using any gRPC client library:

```bash
# Using grpcurl for testing
grpcurl -plaintext localhost:8112 list

# Using grpcurl to call a method
grpcurl -plaintext -d '{"prompt": "Hello"}' \
  localhost:8112 helixagent.LLMFacade/Complete
```

For Go clients:

```go
conn, err := grpc.Dial("localhost:8112", grpc.WithTransportCredentials(insecure.NewCredentials()))
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

client := pb.NewLLMFacadeClient(conn)
```

---

## Services Overview

| Service | Purpose | Methods |
|---------|---------|---------|
| `LLMFacade` | Ensemble operations, sessions, providers, metrics | 10 methods |
| `LLMProvider` | Direct single-provider access | 5 methods |

---

## LLMFacade Service

### Complete (Unary)

Sends a prompt to the ensemble and returns a single aggregated response.

```protobuf
rpc Complete(CompletionRequest) returns (CompletionResponse)
```

The server selects the best available provider based on LLMsVerifier scores and executes the completion.

### CompleteStream (Server Streaming)

Streams completion tokens as they are generated.

```protobuf
rpc CompleteStream(CompletionRequest) returns (stream CompletionResponse)
```

Each streamed message contains a partial response. The final message includes complete token usage metadata.

### Chat (Server Streaming)

Multi-turn chat with conversation history, streamed as server-sent chunks.

```protobuf
rpc Chat(ChatRequest) returns (stream ChatResponse)
```

### Provider Management

| Method | Type | Description |
|--------|------|-------------|
| `ListProviders` | Unary | List all registered providers with health status |
| `AddProvider` | Unary | Register a new LLM provider |
| `UpdateProvider` | Unary | Modify provider configuration or weight |
| `RemoveProvider` | Unary | Unregister a provider |

Provider registration includes fields for name, type, model, base URL, weight, and custom configuration passed as a `google.protobuf.Struct`.

### Session Management

| Method | Type | Description |
|--------|------|-------------|
| `CreateSession` | Unary | Create a new conversation session |
| `GetSession` | Unary | Retrieve session details and status |
| `TerminateSession` | Unary | End a session and release resources |

Sessions track user ID, request count, memory enablement, and expiration time.

### HealthCheck (Unary)

Returns server health including uptime, active sessions, active providers, and per-component status.

```protobuf
rpc HealthCheck(HealthRequest) returns (HealthResponse)
```

### GetMetrics (Unary)

Returns server metrics: total requests, success/failure counts, average latency, and active session/provider counts.

```protobuf
rpc GetMetrics(MetricsRequest) returns (MetricsResponse)
```

---

## LLMProvider Service

The `LLMProvider` service provides direct access to individual providers through the ProviderRegistry, bypassing ensemble logic.

| Method | Type | Description |
|--------|------|-------------|
| `Complete` | Unary | Single-provider completion |
| `CompleteStream` | Server Streaming | Single-provider streaming completion |
| `HealthCheck` | Unary | Provider-specific health check |
| `GetCapabilities` | Unary | Query provider capabilities (tools, streaming, vision) |
| `ValidateConfig` | Unary | Validate provider configuration |

Specify the target provider using the `provider` field in the request. The server looks up the provider in the registry and routes accordingly.

---

## Protobuf Definitions

The protobuf definitions are located in `pkg/api/`. Key message types:

```protobuf
message CompletionRequest {
  string prompt = 1;
  string model = 2;
  string provider = 3;
  int32 max_tokens = 4;
  double temperature = 5;
  string session_id = 6;
  bool stream = 7;
}

message CompletionResponse {
  string id = 1;
  string content = 2;
  string model = 3;
  string provider = 4;
  int32 tokens_used = 5;
  int64 latency_ms = 6;
  bool is_final = 7;
}
```

---

## Examples

### Unary Completion (Go)

```go
resp, err := client.Complete(ctx, &pb.CompletionRequest{
    Prompt:    "Explain the CAP theorem in distributed systems.",
    MaxTokens: 500,
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Response: %s\nTokens: %d\n", resp.Content, resp.TokensUsed)
```

### Streaming Completion (Go)

```go
stream, err := client.CompleteStream(ctx, &pb.CompletionRequest{
    Prompt: "Write a short story about a robot.",
    Stream: true,
})
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
    fmt.Print(resp.Content)
}
```

### Session Workflow (Go)

```go
session, _ := client.CreateSession(ctx, &pb.CreateSessionRequest{
    UserId: "user-123", MemoryEnabled: true,
})
resp, _ := client.Complete(ctx, &pb.CompletionRequest{
    Prompt: "Remember my name is Alice.", SessionId: session.Id,
})
client.TerminateSession(ctx, &pb.TerminateSessionRequest{SessionId: session.Id})
```

---

## Error Handling

The gRPC server uses standard gRPC status codes:

| Code | Condition |
|------|-----------|
| `InvalidArgument` | Missing or invalid request fields |
| `NotFound` | Provider or session not found |
| `Internal` | Provider failure or server error |
| `AlreadyExists` | Duplicate provider registration |
| `Unavailable` | Service not ready |

Errors include descriptive messages. For streaming RPCs, errors are delivered through the stream error mechanism.

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HELIXAGENT_PORT_GRPC` | `8112` | gRPC server listen port |
| `GRPC_MAX_RECV_MSG_SIZE` | `4MB` | Maximum inbound message size |
| `GRPC_MAX_SEND_MSG_SIZE` | `4MB` | Maximum outbound message size |

Start the gRPC server:

```bash
./bin/helixagent --grpc
# or
go run cmd/grpc-server/main.go
```
