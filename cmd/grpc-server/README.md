# HelixAgent gRPC Server

This package contains the gRPC server entry point for high-performance RPC communication.

## Overview

The gRPC server provides a high-performance alternative to the REST API for scenarios requiring:
- Lower latency
- Bi-directional streaming
- Strong typing with Protocol Buffers
- Efficient binary serialization

## Files

- `main.go` - gRPC server entry point
- `server.go` - gRPC service implementations

## Features

- Completion service (unary and streaming)
- Debate service with progress streaming
- Model information service
- Health check service (gRPC health protocol)

## Proto Definitions

Proto files are located in `proto/`:
- `completion.proto` - Completion service
- `debate.proto` - Debate service
- `models.proto` - Model service
- `health.proto` - Health check service

## Usage

### Build and Run

```bash
# Build
go build -o bin/grpc-server ./cmd/grpc-server

# Run
./bin/grpc-server

# Run with custom port (the default is 8112; HELIXAGENT_PORT_PREFIX=9 shifts
# the whole band to 91xx instead of overriding this one service)
HELIXAGENT_PORT_GRPC=8500 ./bin/grpc-server
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HELIXAGENT_PORT_GRPC` | gRPC server port | `8112` |
| `GRPC_MAX_MESSAGE_SIZE` | Max message size | `4MB` |
| `GRPC_KEEPALIVE_TIME` | Keepalive interval | `30s` |

## Client Example

```go
import (
    "google.golang.org/grpc"
    pb "dev.helix.agent/proto"
)

conn, err := grpc.Dial("localhost:8112", grpc.WithInsecure())
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

client := pb.NewCompletionServiceClient(conn)

// Unary call
response, err := client.CreateCompletion(ctx, &pb.CompletionRequest{
    Model: "claude-sonnet-4-20250514",
    Messages: []*pb.Message{
        {Role: "user", Content: "Hello!"},
    },
})

// Streaming call
stream, err := client.CreateCompletionStream(ctx, &pb.CompletionRequest{...})
for {
    chunk, err := stream.Recv()
    if err == io.EOF {
        break
    }
    fmt.Print(chunk.Content)
}
```

## Health Checking

```bash
# Using grpcurl
grpcurl -plaintext localhost:8112 grpc.health.v1.Health/Check

# Using grpc-health-probe
grpc-health-probe -addr=localhost:8112
```

## Testing

```bash
go test -v ./cmd/grpc-server/...
```
