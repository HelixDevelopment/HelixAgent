# go-p4rs3lt0ngv3

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-p4rs3lt0ngv3.svg)](https://pkg.go.dev/github.com/elder-plinius/go-p4rs3lt0ngv3)

Universal Text Transformation Engine -- Go library for the P4RS3LT0NGV3 service.

## Overview

Go implementation of P4RS3LT0NGV3 providing 159+ text transforms including encodings (Base64, ROT13, URL encoding), classical/modern ciphers (Caesar, Vigenere, Atbash), Unicode styles (bold, italic, strikethrough, script), formatting, and niche alphabets.

## Installation

```bash
go get github.com/elder-plinius/go-p4rs3lt0ngv3
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    p4rs3lt0ngv3 "github.com/elder-plinius/go-p4rs3lt0ngv3/pkg/client"
)

func main() {
    client, err := p4rs3lt0ngv3.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `TransformResult`
- `TransformConfig`
- `MultiTransformOptions`
- `EncodeDecodeOptions`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `ListTransforms` | `ctx context.Context` | List all available transforms |
| `GetByCategory` | `category string` | Get transforms by category |
| `Encode` | `opts EncodeDecodeOptions` | Encode/transform text |
| `Decode` | `opts EncodeDecodeOptions` | Decode/reverse transform |
| `MultiTransform` | `opts MultiTransformOptions` | Apply multiple transforms |
| `DetectEncoding` | `text string` | Detect possible encodings |
| `ChainTransform` | `text string, chain []string` | Chain multiple transforms |
| `GetCategories` | `ctx context.Context` | Get transform categories |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `P4RS3LT0NGV3_ADDRESS` | `localhost` | Service address |
| `P4RS3LT0NGV3_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
