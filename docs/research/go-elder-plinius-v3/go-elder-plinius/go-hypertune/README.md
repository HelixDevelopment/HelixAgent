# go-hypertune

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-hypertune.svg)](https://pkg.go.dev/github.com/elder-plinius/go-hypertune)

LLM Hyperparameter Optimization -- Go library for the HyperTune service.

## Overview

Go library for HyperTune providing automated hyperparameter optimization for LLM inference including temperature, top_p, top_k, repetition penalty, and context window tuning via Bayesian optimization and grid search.

## Installation

```bash
go get github.com/elder-plinius/go-hypertune
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    hypertune "github.com/elder-plinius/go-hypertune/pkg/client"
)

func main() {
    client, err := hypertune.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `ParameterSpace`
- `OptimizationConfig`
- `OptimizationResult`
- `TrialResult`
- `EvaluationMetric`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `Optimize` | `space ParameterSpace, cfg OptimizationConfig` | Run hyperparameter optimization |
| `GridSearch` | `space ParameterSpace, cfg OptimizationConfig` | Run grid search |
| `BayesianOptimize` | `space ParameterSpace, cfg OptimizationConfig` | Run Bayesian optimization |
| `Evaluate` | `params map[string]float64, prompt string, model string` | Evaluate parameter set |
| `GetMetrics` | `ctx context.Context` | List available metrics |
| `SuggestParameters` | `space ParameterSpace, history []TrialResult` | Suggest next parameters to try |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HYPERTUNE_ADDRESS` | `localhost` | Service address |
| `HYPERTUNE_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
