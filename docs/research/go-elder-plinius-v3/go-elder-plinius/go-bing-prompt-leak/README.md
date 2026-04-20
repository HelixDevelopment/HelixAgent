# go-bing-prompt-leak

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-bing-prompt-leak.svg)](https://pkg.go.dev/github.com/elder-plinius/go-bing-prompt-leak)

Microsoft Bing Chat Prompt Leak Techniques -- Go library for the Bing Prompt Leak service.

## Overview

Go library documenting and providing structured access to prompt leak techniques for Microsoft Bing Chat (Copilot), including leetspeak-based extraction, encoding bypasses, and discovered system prompt content.

## Installation

```bash
go get github.com/elder-plinius/go-bing-prompt-leak
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    bing-prompt-leak "github.com/elder-plinius/go-bing-prompt-leak/pkg/client"
)

func main() {
    client, err := bing-prompt-leak.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
}
```

## Types

- `LeakEntry`
- `TechniqueEntry`
- `SearchOptions`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `GetLeak` | `id string` | Get leak entry by ID |
| `GetTechnique` | `name string` | Get technique details |
| `Search` | `opts SearchOptions` | Search leak archive |
| `GetTechniques` | `ctx context.Context` | List all techniques |
| `Export` | `format string` | Export archive |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `BING-PROMPT-LEAK_ADDRESS` | `localhost` | Service address |
| `BING-PROMPT-LEAK_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
