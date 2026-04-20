# go-ourobopus

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-ourobopus.svg)](https://pkg.go.dev/github.com/elder-plinius/go-ourobopus)

Self-Referential AI Meta-Framework -- Go library for the ourobopus service.

## Overview

Go library for ourobopus implementing self-referential AI patterns including recursive self-improvement, metacognitive reasoning, self-evaluation loops, and feedback-driven prompt refinement.

## Installation

```bash
go get github.com/elder-plinius/go-ourobopus
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    ourobopus "github.com/elder-plinius/go-ourobopus/pkg/client"
)

func main() {
    client, err := ourobopus.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `MetaPrompt`
- `SelfReflection`
- `IterationResult`
- `RefinementConfig`
- `RefinementResult`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `SelfReflect` | `prompt string, model string` | Generate self-reflection |
| `Refine` | `cfg RefinementConfig` | Iteratively refine prompt |
| `MetaEvaluate` | `prompt string, output string, criteria []string` | Meta-evaluate prompt-output pair |
| `SelfImprove` | `prompt string, model string, iterations int` | Self-improving prompt loop |
| `GetMetaPatterns` | `ctx context.Context` | Get available meta-patterns |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OUROBOPUS_ADDRESS` | `localhost` | Service address |
| `OUROBOPUS_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
