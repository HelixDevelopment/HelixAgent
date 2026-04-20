# go-gitty

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-gitty.svg)](https://pkg.go.dev/github.com/elder-plinius/go-gitty)

Git AI Assistant -- Go library for the Gitty service.

## Overview

Go library for Gitty providing AI-powered Git assistance including commit message generation, code review, PR description writing, branch naming suggestions, and repository analysis.

## Installation

```bash
go get github.com/elder-plinius/go-gitty
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    gitty "github.com/elder-plinius/go-gitty/pkg/client"
)

func main() {
    client, err := gitty.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `CommitOptions`
- `CommitMessage`
- `ReviewOptions`
- `ReviewResult`
- `Issue`
- `Suggestion`
- `RepoStats`

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `GenerateCommitMessage` | `opts CommitOptions` | Generate commit message from diff |
| `ReviewCode` | `opts ReviewOptions` | Review code changes |
| `GeneratePRDescription` | `diff string, title string` | Generate PR description |
| `SuggestBranchName` | `description string` | Suggest branch names |
| `AnalyzeRepo` | `repoPath string` | Analyze repository |
| `GenerateChangelog` | `commits []string, style string` | Generate changelog |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `GITTY_ADDRESS` | `localhost` | Service address |
| `GITTY_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
go test ./... -v
```

## License

Apache-2.0
