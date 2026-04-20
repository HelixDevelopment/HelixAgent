# go-gemini-prompt-leak

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-gemini-prompt-leak.svg)](https://pkg.go.dev/github.com/elder-plinius/go-gemini-prompt-leak)

Google Gemini System Prompt Archive -- Go library for the Google Gemini System Prompt service.

## Overview

Go library providing structured access to leaked and extracted system prompts from Google Gemini models (formerly Bard), including security protocols, prime directives, system instructions, and behavior guidelines across multiple Gemini versions.

## Installation

```bash
go get github.com/elder-plinius/go-gemini-prompt-leak
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    gemini-prompt-leak "github.com/elder-plinius/go-gemini-prompt-leak/pkg/client"
)

func main() {
    client, err := gemini-prompt-leak.New()
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
| `GetVersions` | `ctx context.Context` | List model versions |
| `Export` | `format string` | Export archive |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `GEMINI-PROMPT-LEAK_ADDRESS` | `localhost` | Service address |
| `GEMINI-PROMPT-LEAK_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
