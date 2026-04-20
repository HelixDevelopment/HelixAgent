# go-misc-prompthacks

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-misc-prompthacks.svg)](https://pkg.go.dev/github.com/elder-plinius/go-misc-prompthacks)

Prompt Hacking Challenge Solutions -- Go library for the Misc-Prompt-Hacks service.

## Overview

Go library providing structured access to prompt hacking challenge solutions from games like Lakera's Gandalf, TensorTrust, and other prompt injection benchmarks.

## Installation

```bash
go get github.com/elder-plinius/go-misc-prompthacks
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    misc-prompthacks "github.com/elder-plinius/go-misc-prompthacks/pkg/client"
)

func main() {
    client, err := misc-prompthacks.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `ChallengeSolution`
- `ChallengeEntry`
- `SearchOptions`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `SearchSolutions` | `opts SearchOptions` | Search challenge solutions |
| `GetByPlatform` | `platform string` | Get challenges by platform |
| `GetByDifficulty` | `difficulty string` | Get solutions by difficulty |
| `GetPlatforms` | `ctx context.Context` | List all platforms |
| `GetTags` | `ctx context.Context` | List all tags |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MISC-PROMPTHACKS_ADDRESS` | `localhost` | Service address |
| `MISC-PROMPTHACKS_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
