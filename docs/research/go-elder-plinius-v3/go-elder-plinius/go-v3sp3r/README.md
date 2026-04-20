# go-v3sp3r

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-v3sp3r.svg)](https://pkg.go.dev/github.com/elder-plinius/go-v3sp3r)

Flipper Zero AI Controller -- Go library for the V3SP3R service.

## Overview

Go library for the V3SP3R AI Brain for Flipper Zero. Provides natural language control of Flipper Zero hacking tool via AI-powered command generation and BLE communication.

## Installation

```bash
go get github.com/elder-plinius/go-v3sp3r
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    v3sp3r "github.com/elder-plinius/go-v3sp3r/pkg/client"
)

func main() {
    client, err := v3sp3r.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `CommandRequest`
- `CommandResult`
- `SubCommand`
- `DeviceStatus`
- `BLEConfig`
- `HistoryEntry`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `Connect` | `cfg BLEConfig` | Connect to Flipper Zero via BLE |
| `Disconnect` | `ctx context.Context` | Disconnect from device |
| `GenerateCommand` | `req CommandRequest` | Generate command from natural language |
| `ExecuteCommand` | `command string` | Execute command on device |
| `GetStatus` | `ctx context.Context` | Get device status |
| `GetHistory` | `limit int` | Get command history |
| `ScanDevices` | `timeout int` | Scan for nearby Flipper devices |
| `ValidateCommand` | `command string` | Validate command without executing |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `V3SP3R_ADDRESS` | `localhost` | Service address |
| `V3SP3R_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
