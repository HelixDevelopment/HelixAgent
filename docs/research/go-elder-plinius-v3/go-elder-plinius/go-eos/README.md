# go-eos

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-eos.svg)](https://pkg.go.dev/github.com/elder-plinius/go-eos)

Discord Bot Developer Orchestration -- Go library for the Eos service.

## Overview

Go client for Eos -- a Discord bot that orchestrates open-source developers across multiple servers, facilitating recruitment, skill matching, project discovery, and notifications.

## Installation

```bash
go get github.com/elder-plinius/go-eos
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    eos "github.com/elder-plinius/go-eos/pkg/client"
)

func main() {
    client, err := eos.New()
    if err != nil { log.Fatal(err) }
    defer client.Close()

    // Use the client
    // ...
}
```

## Types

- `UserProfile` -- UserID
- `Project` -- ProjectID
- `ProjectMatch` -- Project
- `AuthenticateOptions` -- DiscordToken
- `AuthenticateResult` -- SessionToken
- `MatchSkillsOptions` -- UserID
- `MatchSkillsResponse` -- Matches
- `DiscoverOptions` -- Skills
- `JoinProjectOptions` -- UserID
- `JoinResult` -- Success
- `OnboardOptions` -- UserID
- `OnboardResult` -- Success

## API Reference

| Method | Parameters | Description |
|--------|------------|-------------|
| `Authenticate` | `opts AuthenticateOptions` | Authenticate user via Discord |
| `MatchSkills` | `opts MatchSkillsOptions` | Match user skills to projects |
| `DiscoverProjects` | `opts DiscoverOptions` | Discover projects by criteria |
| `JoinProject` | `opts JoinProjectOptions` | Join a project |
| `GetProject` | `projectID string` | Get project details |
| `OnboardUser` | `opts OnboardOptions` | Complete user onboarding |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `EOS_ADDRESS` | `localhost` | Service address |
| `EOS_TIMEOUT` | `30s` | RPC timeout |

## Testing

```bash
cd go-eos
go test ./... -v
```

## License

Apache-2.0
