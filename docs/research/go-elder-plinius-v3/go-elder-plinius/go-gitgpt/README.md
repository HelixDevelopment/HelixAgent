# go-gitgpt

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-gitgpt.svg)](https://pkg.go.dev/github.com/elder-plinius/go-gitgpt)

Git AI Assistant -- Go library for the GitGPT service.

## Overview

Go library for GitGPT providing AI-powered Git workflow assistance including commit message generation, code review, branch naming, repository analysis, and changelog generation using LLM intelligence.

## Installation

```bash
go get github.com/elder-plinius/go-gitgpt
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    gitgpt "github.com/elder-plinius/go-gitgpt/pkg/client"
)

func main() {
    client, err := gitgpt.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
}
```

## Types

- `CommitOptions`
- `CommitMessage`
- `ReviewOptions`
- `ReviewResult`
- `Issue`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `GenerateCommitMessage` | `opts CommitOptions` | Generate commit message |
| `ReviewCode` | `opts ReviewOptions` | Review code changes |
| `GeneratePRDescription` | `diff string, title string` | Generate PR description |
| `SuggestBranchName` | `description string` | Suggest branch names |
| `AnalyzeRepo` | `repoPath string` | Analyze repository |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `GITGPT_ADDRESS` | `localhost` | Service address |
| `GITGPT_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
