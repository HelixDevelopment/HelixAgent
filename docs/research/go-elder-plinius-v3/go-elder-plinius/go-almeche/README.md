# go-almeche

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-almeche.svg)](https://pkg.go.dev/github.com/elder-plinius/go-almeche)

Speech-to-CAD Generation -- Go library for the AlmechE service.

## Overview

Go client for AlmechE -- the Idea-to-Object speech-to-CAD generation service. Transforms spoken descriptions into physical 3D models ready for 3D printing.

## Installation

```bash
go get github.com/elder-plinius/go-almeche
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    almeche "github.com/elder-plinius/go-almeche/pkg/client"
)

func main() {
    client, err := almeche.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `ProcessSpeechOptions` -- AudioData
- `GenerateCADOptions` -- Description
- `GenerateCADResult` -- CADPrompt
- `CostEstimate` -- MaterialWeightG
- `ExportOptions` -- ModelData
- `ExportResult` -- ExportedData
- `TextToSpeechOptions` -- Text
- `TextToSpeechResult` -- AudioData
- `Material` -- Name
- `EstimateOptions` -- VolumeCM3
- `EstimateResult` -- MaterialWeightG

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `ProcessSpeech` | `opts ProcessSpeechOptions` | Speech-to-CAD pipeline |
| `GenerateCAD` | `opts GenerateCADOptions` | Generate CAD from text |
| `ExportModel` | `opts ExportOptions` | Export model to format |
| `TextToSpeech` | `opts TextToSpeechOptions` | Convert text to speech |
| `GetAvailableMaterials` | `` | Get 3D printing materials |
| `EstimateCost` | `opts EstimateOptions` | Estimate print cost |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ALMECHE_ADDRESS` | `localhost` | Service address |
| `ALMECHE_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
cd go-almeche
go test ./... -v
```

## License

Apache-2.0
