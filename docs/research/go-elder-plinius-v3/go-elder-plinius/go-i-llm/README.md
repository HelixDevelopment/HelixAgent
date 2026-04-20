# go-i-llm

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-i-llm.svg)](https://pkg.go.dev/github.com/elder-plinius/go-i-llm)

Interactive LLM Pattern Library -- Go library for the I-LLM service.

## Overview

Go library for I-LLM providing interactive LLM conversation patterns, chain-of-thought templates, ReAct agent implementations, and structured reasoning frameworks.

## Installation

```bash
go get github.com/elder-plinius/go-i-llm
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    i-llm "github.com/elder-plinius/go-i-llm/pkg/client"
)

func main() {
    client, err := i-llm.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `ConversationPattern`
- `ReActStep`
- `AgentConfig`
- `Tool`
- `ChainResult`
- `PromptChain`
- `ChainStep`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `GetPattern` | `id string` | Get conversation pattern by ID |
| `ListPatterns` | `category string` | List patterns by category |
| `RenderPattern` | `pattern ConversationPattern, vars map[string]string` | Render pattern with variables |
| `CreateAgent` | `cfg AgentConfig` | Create ReAct agent |
| `RunChain` | `chain PromptChain, inputs map[string]string` | Run prompt chain |
| `ChainOfThought` | `problem string, model string` | Generate chain-of-thought reasoning |
| `TreeOfThought` | `problem string, model string, breadth int` | Generate tree-of-thought exploration |
| `GetCategories` | `ctx context.Context` | List pattern categories |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `I-LLM_ADDRESS` | `localhost` | Service address |
| `I-LLM_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
