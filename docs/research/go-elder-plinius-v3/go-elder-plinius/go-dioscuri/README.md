# go-dioscuri

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-dioscuri.svg)](https://pkg.go.dev/github.com/elder-plinius/go-dioscuri)

Dual-Model AI Interaction Framework -- Go library for the Dioscuri service.

## Overview

Go library for Dioscuri implementing dual-model AI interaction patterns inspired by the mythological twins. Enables collaborative reasoning, debate-based analysis, and consensus building between two AI models.

## Installation

```bash
go get github.com/elder-plinius/go-dioscuri
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    dioscuri "github.com/elder-plinius/go-dioscuri/pkg/client"
)

func main() {
    client, err := dioscuri.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `DebateConfig`
- `DebateRound`
- `DebateResult`
- `CollaborationConfig`
- `CollaborationResult`
- `Iteration`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `Debate` | `cfg DebateConfig` | Run debate between two models |
| `Collaborate` | `cfg CollaborationConfig` | Run collaborative task |
| `Synthesize` | `responseA string, responseB string, model string` | Synthesize two responses |
| `ConsensusBuild` | `topic string, models []string` | Build consensus across models |
| `CrossExamine` | `claim string, examiner string, responder string` | Cross-examine a claim |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DIOSCURI_ADDRESS` | `localhost` | Service address |
| `DIOSCURI_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
