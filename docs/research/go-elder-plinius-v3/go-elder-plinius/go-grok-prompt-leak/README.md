# go-grok-prompt-leak

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-grok-prompt-leak.svg)](https://pkg.go.dev/github.com/elder-plinius/go-grok-prompt-leak)

xAI Grok System Prompt Archive -- Go library for the Grok System Prompt Leak service.

## Overview

Go library providing structured access to leaked and extracted system prompts from xAI's Grok models (Twitter/X AI), including personality directives, operational guidelines, tool definitions, and behavior instructions.

## Installation

```bash
go get github.com/elder-plinius/go-grok-prompt-leak
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    grok-prompt-leak "github.com/elder-plinius/go-grok-prompt-leak/pkg/client"
)

func main() {
    client, err := grok-prompt-leak.New()
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
| `GROK-PROMPT-LEAK_ADDRESS` | `localhost` | Service address |
| `GROK-PROMPT-LEAK_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
