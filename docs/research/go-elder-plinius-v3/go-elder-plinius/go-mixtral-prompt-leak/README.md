# go-mixtral-prompt-leak

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-mixtral-prompt-leak.svg)](https://pkg.go.dev/github.com/elder-plinius/go-mixtral-prompt-leak)

Mixtral/Mistral System Prompt Archive -- Go library for the Mixtral System Prompt Leak service.

## Overview

Go library providing structured access to leaked and extracted system prompts from Mistral and Mixtral AI models, including system instructions, safety guidelines, tool use definitions, and model behavior directives.

## Installation

```bash
go get github.com/elder-plinius/go-mixtral-prompt-leak
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    mixtral-prompt-leak "github.com/elder-plinius/go-mixtral-prompt-leak/pkg/client"
)

func main() {
    client, err := mixtral-prompt-leak.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
}
```

## Types

- `PromptEntry`
- `SearchOptions`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `GetPrompt` | `id string` | Get prompt by ID |
| `GetByModel` | `model string` | Get prompts for model |
| `Search` | `opts SearchOptions` | Search prompt archive |
| `GetModels` | `ctx context.Context` | List available models |
| `Export` | `format string` | Export archive |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MIXTRAL-PROMPT-LEAK_ADDRESS` | `localhost` | Service address |
| `MIXTRAL-PROMPT-LEAK_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
