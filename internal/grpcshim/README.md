# gRPC Shim Package

The grpcshim package provides a compatibility layer for gRPC services in SuperAgent.

## Overview

This package enables:
- gRPC service definitions alongside REST APIs
- Protocol buffer message handling
- Streaming RPC support
- Cross-service communication

## Components

### Server

The gRPC server implementation:

```go
server := grpcshim.NewServer(cfg)
server.RegisterServices(services...)
server.Start(":50051")
```

### Services

Available gRPC services:

- **LLMService** - LLM completion requests
- **EnsembleService** - Ensemble orchestration
- **DebateService** - AI debate sessions
- **VerificationService** - Model verification

## Configuration

```yaml
grpc:
  port: 50051
  max_message_size: 4194304  # 4MB
  keep_alive_time: 30s
  keep_alive_timeout: 10s
```

## Usage

### Client Connection

```go
conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

client := pb.NewLLMServiceClient(conn)
```

### Streaming Example

```go
stream, err := client.CompleteStream(ctx, &pb.CompletionRequest{...})
for {
    chunk, err := stream.Recv()
    if err == io.EOF {
        break
    }
    fmt.Print(chunk.Content)
}
```

## Protocol Buffers

Proto files are located in `proto/` directory. Regenerate with:

```bash
protoc --go_out=. --go-grpc_out=. proto/*.proto
```

## Integration with HTTP

The package supports gRPC-Gateway for REST/gRPC interoperability:

```go
mux := runtime.NewServeMux()
pb.RegisterLLMServiceHandlerFromEndpoint(ctx, mux, grpcAddr, opts)
```
