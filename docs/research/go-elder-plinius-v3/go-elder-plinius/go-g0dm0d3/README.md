# go-g0dm0d3

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-g0dm0d3.svg)](https://pkg.go.dev/github.com/elder-plinius/go-g0dm0d3)

Multi-Model AI Chat Framework -- Go library for the G0DM0D3 service.

## Overview

Go implementation of the G0DM0D3 liberated AI chat framework. Provides parallel model racing (GODMODE CLASSIC), multi-model evaluation (ULTRAPLINIAN), input perturbation (Parseltongue), context-adaptive sampling (AutoTune), and semantic transformation modules (STM).

## Installation

```bash
go get github.com/elder-plinius/go-g0dm0d3
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    g0dm0d3 "github.com/elder-plinius/go-g0dm0d3/pkg/client"
)

func main() {
    client, err := g0dm0d3.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `ChatRequest`
- `ChatResponse`
- `ModelResponse`
- `EvaluationResult`
- `ParseltongueOptions`
- `AutoTuneOptions`
- `RaceResult`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `Chat` | `req ChatRequest` | Send chat request to models |
| `Race` | `req ChatRequest` | Race multiple models for fastest best response |
| `Evaluate` | `responses []ModelResponse` | Evaluate and rank responses |
| `Parseltongue` | `opts ParseltongueOptions` | Apply input perturbation techniques |
| `AutoTune` | `opts AutoTuneOptions` | Auto-tune parameters for prompt |
| `GetAvailableModels` | `ctx context.Context` | List available models |
| `GetModes` | `ctx context.Context` | List available modes |
| `STMTransform` | `text string, module string` | Apply semantic transformation |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `G0DM0D3_ADDRESS` | `localhost` | Service address |
| `G0DM0D3_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
