# go-theseus

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-theseus.svg)](https://pkg.go.dev/github.com/elder-plinius/go-theseus)

Autonomous GPT-4 Agent Framework -- Go library for the Theseus service.

## Overview

Go library for the Theseus autonomous agent framework (based on AutoGPT). Provides agent creation, task planning, tool integration, benchmark evaluation, and multi-agent arena competition. Experimental open-source attempt to make GPT-4 fully autonomous.

## Installation

```bash
go get github.com/elder-plinius/go-theseus
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    theseus "github.com/elder-plinius/go-theseus/pkg/client"
)

func main() {
    client, err := theseus.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
}
```

## Types

- `AgentConfig`
- `Agent`
- `TaskEntry`
- `BenchmarkConfig`
- `BenchmarkResult`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `CreateAgent` | `cfg AgentConfig` | Create autonomous agent |
| `RunAgent` | `agentID string, task string` | Run agent on task |
| `GetAgent` | `agentID string` | Get agent status |
| `RunBenchmark` | `cfg BenchmarkConfig` | Run benchmark |
| `ListAgents` | `ctx context.Context` | List all agents |
| `ArenaCompete` | `agentIDs []string, challenge string` | Run arena competition |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `THESEUS_ADDRESS` | `localhost` | Service address |
| `THESEUS_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
