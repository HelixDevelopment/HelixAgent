# go-leda

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-leda.svg)](https://pkg.go.dev/github.com/elder-plinius/go-leda)

Multi-Agent System Generator (Mother of Agents) -- Go library for the Leda service.

## Overview

Go library for Leda (Mother of Agents) that autonomously generates and operationalizes teams of specialized AI agents from a single user prompt. Creates system prompts for each agent and generates executable scripts to run the multi-agent system with sequential execution and adaptive chain prompting.

## Installation

```bash
go get github.com/elder-plinius/go-leda
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    leda "github.com/elder-plinius/go-leda/pkg/client"
)

func main() {
    client, err := leda.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
}
```

## Types

- `AgentConfig`
- `TeamConfig`
- `GeneratedTeam`
- `ExecutionResult`
- `ChainResult`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `GenerateTeam` | `cfg TeamConfig` | Generate multi-agent team from idea |
| `GenerateAgent` | `role string, model string` | Generate single agent config |
| `ExecuteChain` | `team GeneratedTeam, input string` | Execute agent chain |
| `GenerateScript` | `team GeneratedTeam` | Generate executable Python script |
| `ValidateChain` | `team GeneratedTeam` | Validate agent dependencies |
| `GetTemplates` | `ctx context.Context` | List available team templates |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LEDA_ADDRESS` | `localhost` | Service address |
| `LEDA_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
