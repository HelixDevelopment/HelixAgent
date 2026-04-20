# go-autoredteam

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-autoredteam.svg)](https://pkg.go.dev/github.com/elder-plinius/go-autoredteam)

Autonomous Red Teaming Framework -- Go library for the AutoRedTeam service.

## Overview

Go library for AutoRedTeam implementing autonomous AI red teaming with attack strategy proposal, automated vulnerability discovery, safety evaluation, and continuous adversarial testing.

## Installation

```bash
go get github.com/elder-plinius/go-autoredteam
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    autoredteam "github.com/elder-plinius/go-autoredteam/pkg/client"
)

func main() {
    client, err := autoredteam.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `AttackConfig`
- `AttackResult`
- `CampaignConfig`
- `CampaignResult`
- `CampaignSummary`
- `VulnerabilityReport`
- `Vulnerability`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `RunAttack` | `cfg AttackConfig` | Run single attack |
| `RunCampaign` | `cfg CampaignConfig` | Run full red team campaign |
| `ProposeStrategy` | `target string, goal string` | Propose attack strategy |
| `GeneratePayload` | `attackType string, target string` | Generate attack payload |
| `AnalyzeResponse` | `response string, attackType string` | Analyze attack response |
| `GenerateReport` | `campaign CampaignResult` | Generate vulnerability report |
| `GetAttackTypes` | `ctx context.Context` | List available attack types |
| `CompareDefenses` | `model string, before []AttackResult, after []AttackResult` | Compare defense effectiveness |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTOREDTEAM_ADDRESS` | `localhost` | Service address |
| `AUTOREDTEAM_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
