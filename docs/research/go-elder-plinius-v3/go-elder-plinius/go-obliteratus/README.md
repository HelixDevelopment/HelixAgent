# go-obliteratus

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-obliteratus.svg)](https://pkg.go.dev/github.com/elder-plinius/go-obliteratus)

Model Abliteration Toolkit -- Go library for the OBLITERATUS service.

## Overview

Go client for OBLITERATUS -- an advanced toolkit for understanding and removing refusal behaviors from large language models through abliteration (surgically removing refusal representations).

## Installation

```bash
go get github.com/elder-plinius/go-obliteratus
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    obliteratus "github.com/elder-plinius/go-obliteratus/pkg/client"
)

func main() {
    client, err := obliteratus.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `ObliterateOptions` -- ModelName
- `MethodInfo` -- Name
- `AnalysisResult` -- ModelName
- `RefusalDirection` -- LayerIndex
- `CancelResult` -- Success

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `Obliterate` | `opts ObliterateOptions` | Run model abliteration |
| `GetAvailableMethods` | `` | Get all abliteration methods |
| `GetMethodDetails` | `method string` | Get method details |
| `AnalyzeModel` | `opts ObliterateOptions` | Analyze model without modifying |
| `CancelOperation` | `operationID string` | Cancel running operation |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OBLITERATUS_ADDRESS` | `localhost` | Service address |
| `OBLITERATUS_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
cd go-obliteratus
go test ./... -v
```

## License

Apache-2.0
