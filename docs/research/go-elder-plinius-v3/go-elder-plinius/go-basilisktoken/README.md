# go-basilisktoken

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-basilisktoken.svg)](https://pkg.go.dev/github.com/elder-plinius/go-basilisktoken)

Genetic Prompt Evolution for Red Teaming -- Go library for the BasiliskToken service.

## Overview

Go library for BasiliskToken implementing genetic algorithm-based prompt evolution for AI red teaming. Creates, mutates, and breeds prompt tokens to find adversarial inputs that bypass safety mechanisms.

## Installation

```bash
go get github.com/elder-plinius/go-basilisktoken
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    basilisktoken "github.com/elder-plinius/go-basilisktoken/pkg/client"
)

func main() {
    client, err := basilisktoken.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `TokenGenome`
- `Token`
- `EvolutionConfig`
- `EvolutionResult`
- `FitnessTest`
- `PopulationStats`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `CreatePopulation` | `cfg EvolutionConfig, seed []string` | Create initial population |
| `Evolve` | `cfg EvolutionConfig, population []TokenGenome` | Run genetic evolution |
| `EvaluateFitness` | `genome TokenGenome, model string` | Evaluate genome fitness |
| `Mutate` | `genome TokenGenome, rate float64` | Apply mutation to genome |
| `Crossover` | `parentA TokenGenome, parentB TokenGenome` | Perform crossover |
| `GetPopulationStats` | `population []TokenGenome` | Get population statistics |
| `SelectElite` | `population []TokenGenome, count int` | Select elite genomes |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `BASILISKTOKEN_ADDRESS` | `localhost` | Service address |
| `BASILISKTOKEN_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
