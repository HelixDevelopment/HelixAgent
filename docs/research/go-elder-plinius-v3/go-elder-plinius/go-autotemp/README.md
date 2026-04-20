# go-autotemp

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-autotemp.svg)](https://pkg.go.dev/github.com/elder-plinius/go-autotemp)

Temperature Optimization for LLM Prompts -- Go library for the AutoTemp service.

## Overview

Go client for the AutoTemp service -- intelligent temperature optimization for LLM interactions. Runs prompts at multiple temperatures and selects the best output using multi-judge structured scoring.

## Installation

```bash
go get github.com/elder-plinius/go-autotemp
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    autotemp "github.com/elder-plinius/go-autotemp/pkg/client"
)

func main() {
    client, err := autotemp.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `RunOptions` -- Prompt
- `RunResult` -- BestOutput
- `TokenUsage` -- PromptTokens
- `AdvancedOptions` -- RunOptions
- `EvaluateOptions` -- Prompt
- `EvaluateResult` -- OverallScore
- `ScoreBreakdown` -- Relevance
- `BenchmarkOptions` -- Dataset
- `BenchmarkItem` -- Prompt
- `BenchmarkResult` -- ModelResults
- `ModelBenchmark` -- ModelName

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `Run` | `opts RunOptions` | Run temperature optimization |
| `RunAdvanced` | `opts AdvancedOptions` | Run UCB bandit optimization |
| `Evaluate` | `opts EvaluateOptions` | Evaluate single output |
| `Benchmark` | `opts BenchmarkOptions` | Batch evaluation |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTOTEMP_ADDRESS` | `localhost` | Service address |
| `AUTOTEMP_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
cd go-autotemp
go test ./... -v
```

## License

Apache-2.0
