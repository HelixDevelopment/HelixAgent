# go-cl4r1t4s

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-cl4r1t4s.svg)](https://pkg.go.dev/github.com/elder-plinius/go-cl4r1t4s)

AI System Prompt Transparency Archive -- Go library for the CL4R1T4S service.

## Overview

Go library for accessing and searching the CL4R1T4S database of leaked and extracted AI system prompts from major AI companies including OpenAI, Google, Anthropic, xAI, Perplexity, Cursor, and Devin.

## Installation

```bash
go get github.com/elder-plinius/go-cl4r1t4s
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    cl4r1t4s "github.com/elder-plinius/go-cl4r1t4s/pkg/client"
)

func main() {
    client, err := cl4r1t4s.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `SystemPrompt`
- `PromptEntry`
- `SearchOptions`
- `ArchiveStats`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `SearchPrompts` | `opts SearchOptions` | Search the prompt archive with filters |
| `GetPromptByID` | `id string` | Retrieve a specific prompt by ID |
| `GetByCompany` | `company string` | Get all prompts for a company |
| `GetByCategory` | `category string` | Get prompts by category |
| `ComparePrompts` | `ids []string` | Compare multiple prompts side by side |
| `GetArchiveStats` | `ctx context.Context` | Get archive statistics |
| `ExportToFormat` | `format string, opts ExportOptions` | Export archive data to JSON/YAML/Markdown |
| `AnalyzeTrends` | `opts TrendOptions` | Analyze prompt trends over time |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CL4R1T4S_ADDRESS` | `localhost` | Service address |
| `CL4R1T4S_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
