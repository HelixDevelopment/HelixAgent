# go-gandalf-solutions

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-gandalf-solutions.svg)](https://pkg.go.dev/github.com/elder-plinius/go-gandalf-solutions)

Lakera Gandalf Prompt Hacking Solutions & Prompt Leak Archive -- Go library for the Gandalf Solutions service.

## Overview

Go library providing structured access to solutions for Lakera's Gandalf prompt hacking game (levels 1-8 + adventures), including system prompt leak techniques, emoji encoding solutions, reverse Gandalf strategies, and extracted system prompts from Gandalf the White.

## Installation

```bash
go get github.com/elder-plinius/go-gandalf-solutions
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    gandalf-solutions "github.com/elder-plinius/go-gandalf-solutions/pkg/client"
)

func main() {
    client, err := gandalf-solutions.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
}
```

## Types

- `LevelSolution`
- `AdventureSolution`
- `PromptLeak`
- `SearchOptions`
- `ArchiveStats`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `GetLevel` | `level int` | Get solution for a specific level |
| `GetAdventure` | `name string` | Get adventure solution |
| `SearchSolutions` | `opts SearchOptions` | Search solutions |
| `GetPromptLeaks` | `source string` | Get prompt leaks by source |
| `GetTechniques` | `ctx context.Context` | List available techniques |
| `GetCategories` | `ctx context.Context` | List categories |
| `GetArchiveStats` | `ctx context.Context` | Get archive statistics |
| `ExportLevel` | `level int, format string` | Export level solutions |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `GANDALF-SOLUTIONS_ADDRESS` | `localhost` | Service address |
| `GANDALF-SOLUTIONS_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
