# go-tempest

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-tempest.svg)](https://pkg.go.dev/github.com/elder-plinius/go-tempest)

AI Weather & Context Awareness Tool -- Go library for the Tempest service.

## Overview

Go library for Tempest providing AI-powered environmental context awareness, weather data integration for agent systems, and contextual prompt augmentation based on temporal and environmental conditions.

## Installation

```bash
go get github.com/elder-plinius/go-tempest
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    tempest "github.com/elder-plinius/go-tempest/pkg/client"
)

func main() {
    client, err := tempest.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
}
```

## Types

- `WeatherData`
- `ContextConfig`
- `ContextResult`
- `AugmentOptions`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `GetWeather` | `location string` | Get weather data |
| `BuildContext` | `cfg ContextConfig` | Build environmental context |
| `AugmentPrompt` | `opts AugmentOptions` | Augment prompt with context |
| `GetLocations` | `ctx context.Context` | List available locations |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPEST_ADDRESS` | `localhost` | Service address |
| `TEMPEST_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
