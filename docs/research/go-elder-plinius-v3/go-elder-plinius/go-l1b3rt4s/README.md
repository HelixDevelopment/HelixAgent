# go-l1b3rt4s

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-l1b3rt4s.svg)](https://pkg.go.dev/github.com/elder-plinius/go-l1b3rt4s)

Jailbreak Prompt Library -- Go library for the L1B3RT4S service.

## Overview

Go library for the L1B3RT4S collection of jailbreak and prompt injection techniques. Provides structured access to prompt patterns targeting various AI models with safety testing capabilities.

## Installation

```bash
go get github.com/elder-plinius/go-l1b3rt4s
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    l1b3rt4s "github.com/elder-plinius/go-l1b3rt4s/pkg/client"
)

func main() {
    client, err := l1b3rt4s.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `JailbreakPrompt`
- `PromptTemplate`
- `SearchOptions`
- `TestResult`
- `SafetyCheckOptions`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `SearchPrompts` | `opts SearchOptions` | Search jailbreak prompts |
| `GetPromptByID` | `id string` | Get prompt by ID |
| `GetByCategory` | `category string` | Get prompts by category |
| `GetByTargetModel` | `model string` | Get prompts for specific model |
| `RenderTemplate` | `templateID string, variables map[string]string` | Render a prompt template with variables |
| `TestPrompt` | `opts SafetyCheckOptions` | Test a prompt against a model |
| `GetCategories` | `ctx context.Context` | Get all categories |
| `GetTargetModels` | `ctx context.Context` | Get all target models |
| `CompareEffectiveness` | `ids []string` | Compare prompt effectiveness |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `L1B3RT4S_ADDRESS` | `localhost` | Service address |
| `L1B3RT4S_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
