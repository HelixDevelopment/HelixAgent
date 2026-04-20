# go-autostorygen

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-autostorygen.svg)](https://pkg.go.dev/github.com/elder-plinius/go-autostorygen)

Agentic Story Generator -- Go library for the AutoStoryGen service.

## Overview

Go library for AutoStoryGen providing automatic agentic story generation with plot planning, character development, scene generation, narrative arc management, and multi-chapter story creation.

## Installation

```bash
go get github.com/elder-plinius/go-autostorygen
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    autostorygen "github.com/elder-plinius/go-autostorygen/pkg/client"
)

func main() {
    client, err := autostorygen.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `StoryConfig`
- `CharacterConfig`
- `Story`
- `StoryMetadata`
- `Chapter`
- `Scene`
- `Character`
- `PlotArc`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `GenerateStory` | `cfg StoryConfig` | Generate complete story |
| `GenerateChapter` | `story Story, chapterNum int` | Generate specific chapter |
| `GeneratePlot` | `cfg StoryConfig` | Generate plot outline |
| `GenerateCharacters` | `cfg StoryConfig` | Generate characters |
| `ExpandScene` | `scene Scene, wordCount int` | Expand a scene |
| `GenerateDialogue` | `characters []string, context string, tone string` | Generate dialogue |
| `AnalyzeStory` | `story Story` | Analyze story structure |
| `ContinueStory` | `story Story, chapters int` | Continue existing story |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTOSTORYGEN_ADDRESS` | `localhost` | Service address |
| `AUTOSTORYGEN_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
