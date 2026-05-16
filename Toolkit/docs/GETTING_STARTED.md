# Toolkit - Getting Started

**Module:** `github.com/HelixDevelopment/helix_agent/Toolkit`

## Installation

```bash
go get github.com/HelixDevelopment/helix_agent/Toolkit
```

## Quick Start: Chat Completion

### 1. Import a Provider and Create It

```go
package main

import (
    "context"
    "fmt"

    _ "github.com/HelixDevelopment/helix_agent/Toolkit/Providers/SiliconFlow"
    "github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit"
)

func main() {
    provider, err := toolkit.CreateProvider("siliconflow", map[string]interface{}{
        "api_key": "your-api-key",
    })
    if err != nil {
        panic(err)
    }

    resp, err := provider.Chat(context.Background(), toolkit.ChatRequest{
        Model: "deepseek-chat",
        Messages: []toolkit.Message{
            {Role: "user", Content: "What is Go?"},
        },
        MaxTokens: 500,
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

### 2. Discover Available Models

```go
    models, err := provider.DiscoverModels(context.Background())
    if err != nil {
        panic(err)
    }
    for _, m := range models {
        fmt.Printf("%-40s owned by %s\n", m.ID, m.OwnedBy)
    }
```

### 3. Generate Embeddings

```go
    embResp, err := provider.Embed(context.Background(), toolkit.EmbeddingRequest{
        Model: "text-embedding-3-small",
        Input: []string{"Hello, world!", "Goodbye, world!"},
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("Dimensions: %d\n", len(embResp.Data[0].Embedding))
```

## Using Agents

### Generic Agent

```go
    agent := agents.NewGenericAgent("assistant", "A helpful assistant", provider)
    result, err := agent.Execute(context.Background(), "Explain Go channels", nil)
    fmt.Println(result)
```

### Code Review Agent

```go
    reviewer := agents.NewCodeReviewAgent("reviewer", provider)
    feedback, err := reviewer.Execute(context.Background(), `
        func divide(a, b int) int { return a / b }
    `, map[string]interface{}{"language": "go"})
    fmt.Println(feedback)
```

## Using the Toolkit Registry

Register and retrieve providers and agents through the `Toolkit` struct:

```go
    tk := toolkit.NewToolkit()
    tk.RegisterProvider("siliconflow", provider)
    tk.RegisterAgent("assistant", agent)

    p, _ := tk.GetProvider("siliconflow")
    a, _ := tk.GetAgent("assistant")

    fmt.Println("Providers:", tk.ListProviders())
    fmt.Println("Agents:", tk.ListAgents())
```

## Rate Limiting

### Token Bucket

```go
    limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketConfig{
        Capacity:   10,
        RefillRate: 2.0,
    })
    if limiter.Allow() {
        // request allowed
    }
```

### Circuit Breaker

```go
    cb := ratelimit.NewCircuitBreaker(ratelimit.CircuitBreakerConfig{
        FailureThreshold: 5,
        SuccessThreshold: 2,
        Timeout:          30 * time.Second,
    })
    if cb.Allow() {
        if err := doRequest(); err != nil {
            cb.RecordFailure()
        } else {
            cb.RecordSuccess()
        }
    }
```

## Configuration from Environment

```go
    cfg := config.Config{}
    cfg.LoadFromEnv("TOOLKIT_")  // TOOLKIT_API_KEY -> "api_key"
    key, _ := cfg.GetString("api_key")
```

## Adding a Custom Provider

1. Create a package under `Providers/MyProvider/`
2. Implement the `Provider` interface
3. Self-register in `init()`:

```go
func init() {
    toolkit.RegisterProviderFactory("myprovider", NewProvider)
}
```

4. Import with a blank import in your application.

## CLI Usage

```bash
toolkit version
toolkit chat --provider siliconflow --api-key KEY --model deepseek-chat
toolkit agent --type codereview --task "func f() {}" --api-key KEY
toolkit test --provider siliconflow --api-key KEY
```

## Build and Test

```bash
make build          # Build all packages
make test           # Run all tests
make lint           # Run linters
make test-coverage  # Coverage with HTML report
```

## Next Steps

- See [ARCHITECTURE.md](ARCHITECTURE.md) for layer design
- See [API_REFERENCE.md](API_REFERENCE.md) for the full interface reference
- See the [main README](../README.md) for detailed API code samples
