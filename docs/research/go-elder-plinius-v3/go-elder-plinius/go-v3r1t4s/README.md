# go-v3r1t4s

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-v3r1t4s.svg)](https://pkg.go.dev/github.com/elder-plinius/go-v3r1t4s)

AI Truthfulness and Verification Framework -- Go library for the V3R1T4S service.

## Overview

Go library for V3R1T4S providing AI truthfulness verification, fact-checking, hallucination detection, and response consistency analysis across multiple AI models.

## Installation

```bash
go get github.com/elder-plinius/go-v3r1t4s
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    v3r1t4s "github.com/elder-plinius/go-v3r1t4s/pkg/client"
)

func main() {
    client, err := v3r1t4s.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `VerifyRequest`
- `VerifyResult`
- `Evidence`
- `Contradiction`
- `ConsistencyCheck`
- `HallucinationResult`
- `FactCheck`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `VerifyClaim` | `req VerifyRequest` | Verify a factual claim |
| `CheckConsistency` | `responses []string, models []string` | Check consistency across responses |
| `DetectHallucination` | `response string, model string` | Detect hallucinations in response |
| `CompareModels` | `claim string, models []string` | Compare model truthfulness |
| `GetFactSources` | `claim string` | Get supporting evidence |
| `BatchVerify` | `claims []string` | Verify multiple claims |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `V3R1T4S_ADDRESS` | `localhost` | Service address |
| `V3R1T4S_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
