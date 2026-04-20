# go-st3gg

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-st3gg.svg)](https://pkg.go.dev/github.com/elder-plinius/go-st3gg)

All-in-One Steganography Suite -- Go library for the ST3GG service.

## Overview

Go implementation of ST3GG providing 100+ steganography techniques for hiding data in images (LSB, DCT, DWT), audio (echo hiding, phase coding, spread spectrum), documents (whitespace, metadata), and network packets (TCP/IP headers, DNS tunnels).

## Installation

```bash
go get github.com/elder-plinius/go-st3gg
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    st3gg "github.com/elder-plinius/go-st3gg/pkg/client"
)

func main() {
    client, err := st3gg.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `EmbedOptions`
- `ExtractOptions`
- `StegoMethod`
- `EmbedResult`
- `ExtractResult`
- `AnalyzeResult`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `ListMethods` | `ctx context.Context` | List all steganography methods |
| `Embed` | `opts EmbedOptions` | Embed secret data in carrier |
| `Extract` | `opts ExtractOptions` | Extract secret from carrier |
| `Analyze` | `carrier []byte` | Analyze carrier for hidden data |
| `GetCapacity` | `carrier []byte, method string` | Get embedding capacity |
| `BatchEmbed` | `carriers [][]byte, secret []byte, method string` | Embed across multiple carriers |
| `CompareMethods` | `carrier []byte, secret []byte` | Compare methods for given carrier |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ST3GG_ADDRESS` | `localhost` | Service address |
| `ST3GG_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
