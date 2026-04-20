# go-leakhub

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-leakhub.svg)](https://pkg.go.dev/github.com/elder-plinius/go-leakhub)

Prompt Leak Detection and Archive -- Go library for the LEAKHUB service.

## Overview

Go library for LEAKHUB providing prompt leak detection, extraction, and archival. Identifies potential system prompt leaks in AI model responses and maintains a searchable archive of known leaks.

## Installation

```bash
go get github.com/elder-plinius/go-leakhub
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    leakhub "github.com/elder-plinius/go-leakhub/pkg/client"
)

func main() {
    client, err := leakhub.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `LeakEntry`
- `DetectionOptions`
- `DetectionResult`
- `LeakMatch`
- `ArchiveStats`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `DetectLeak` | `opts DetectionOptions` | Detect prompt leaks in response |
| `SearchArchive` | `query string, limit int` | Search leak archive |
| `AddToArchive` | `entry LeakEntry` | Add leak entry to archive |
| `GetByModel` | `model string` | Get leaks for specific model |
| `GetStats` | `ctx context.Context` | Get archive statistics |
| `ExportArchive` | `format string` | Export archive to format |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LEAKHUB_ADDRESS` | `localhost` | Service address |
| `LEAKHUB_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
