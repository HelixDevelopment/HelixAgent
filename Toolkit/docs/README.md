# Toolkit Documentation

## Architecture Overview

The Toolkit module follows a layered architecture with clean separation between interfaces,
implementations, and shared utilities.

### Layer Diagram

```
+--------------------------------------------------+
|                  CLI (cmd/toolkit)                |
|  version | test | chat | agent                   |
+--------------------------------------------------+
        |                       |
        v                       v
+------------------+   +------------------+
|   pkg/toolkit    |   | pkg/toolkit/     |
|   Toolkit struct |   | agents           |
|   Provider iface |   | GenericAgent     |
|   Agent iface    |   | CodeReviewAgent  |
|   Factory Reg.   |   |                  |
+------------------+   +------------------+
        |                       |
        v                       v
+--------------------------------------------------+
|           pkg/toolkit/common                      |
|   discovery | http/client | ratelimit             |
|   (caching, retry, token bucket, circuit breaker) |
+--------------------------------------------------+
        |
        v
+--------------------------------------------------+
|                    Commons                        |
|   auth | config | discovery | errors | http |     |
|   ratelimit | response | testing                  |
+--------------------------------------------------+
        |
        v
+--------------------------------------------------+
|                   Providers                       |
|          Chutes  |  SiliconFlow                   |
+--------------------------------------------------+
```

### Core Design Decisions

1. **Interface-driven**: `Provider` and `Agent` are the two core interfaces. All provider
   implementations conform to `Provider`; all agent implementations conform to `Agent`.

2. **Factory pattern**: Providers self-register via `init()` functions using
   `toolkit.RegisterProviderFactory()`. This allows adding new providers by simply
   importing the package.

3. **Two-tier common packages**: There are two sets of shared utilities:
   - `pkg/toolkit/common/` -- utilities that depend on the `toolkit` package types
   - `Commons/` -- standalone utilities with no dependency on `toolkit` types

4. **Capability inference**: The discovery framework infers model capabilities (chat,
   embedding, rerank, vision, audio, video, function calling) from model IDs and type
   strings using keyword matching rather than requiring explicit metadata from providers.

## API Reference

### Provider Interface

Every AI provider must implement the `Provider` interface:

```go
type Provider interface {
    // Name returns the provider identifier (e.g., "siliconflow", "chutes")
    Name() string

    // Chat performs a chat completion request
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)

    // Embed generates embeddings for input texts
    Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)

    // Rerank reranks documents by relevance to a query
    Rerank(ctx context.Context, req RerankRequest) (RerankResponse, error)

    // DiscoverModels lists available models from the provider
    DiscoverModels(ctx context.Context) ([]ModelInfo, error)

    // ValidateConfig validates a configuration map
    ValidateConfig(config map[string]interface{}) error
}
```

### Agent Interface

Agents provide specialized AI capabilities on top of providers:

```go
type Agent interface {
    // Name returns the agent identifier
    Name() string

    // Execute performs a task and returns the result text
    Execute(ctx context.Context, task string, config interface{}) (string, error)

    // ValidateConfig validates agent-specific configuration
    ValidateConfig(config interface{}) error

    // Capabilities returns a list of capability strings
    Capabilities() []string
}
```

### Toolkit Struct

The `Toolkit` struct manages provider and agent registries:

```go
tk := toolkit.NewToolkit()
tk.RegisterProvider("siliconflow", provider)
tk.RegisterAgent("assistant", agent)

provider, err := tk.GetProvider("siliconflow")
agent, err := tk.GetAgent("assistant")

providerNames := tk.ListProviders()
agentNames := tk.ListAgents()
```

### Global Provider Factory

Providers register factories globally for creation by name:

```go
// In provider init():
toolkit.RegisterProviderFactory("siliconflow", NewProvider)

// Usage:
provider, err := toolkit.CreateProvider("siliconflow", map[string]interface{}{
    "api_key": "your-api-key",
})
```

### Rate Limiting

Token bucket rate limiter:

```go
limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketConfig{
    Capacity:   10,    // max burst
    RefillRate: 2.0,   // tokens per second
})

if limiter.Allow() {
    // request allowed
}

// Or block until allowed:
err := limiter.Wait(ctx)
```

Sliding window rate limiter:

```go
limiter := ratelimit.NewSlidingWindowLimiter(time.Minute, 60) // 60 req/min
```

Per-key rate limiting (e.g., per user):

```go
perKey := ratelimit.NewPerKeyLimiter(ratelimit.TokenBucketConfig{
    Capacity:   5,
    RefillRate: 1.0,
})

if perKey.Allow("user-123") {
    // allowed for this user
}
```

Circuit breaker:

```go
cb := ratelimit.NewCircuitBreaker(ratelimit.CircuitBreakerConfig{
    FailureThreshold: 5,
    SuccessThreshold: 2,
    Timeout:          30 * time.Second,
})

if cb.Allow() {
    err := doRequest()
    if err != nil {
        cb.RecordFailure()
    } else {
        cb.RecordSuccess()
    }
}
```

### Authentication

API key authentication:

```go
auth := auth.NewAPIKeyAuth("your-api-key")
header, err := auth.GetAuthHeader(ctx) // "Bearer your-api-key"
```

OAuth2 token refresh:

```go
refresher := auth.NewOAuth2Refresher(
    "client-id", "client-secret", "https://auth.example.com/token",
    auth.WithRefreshToken("refresh-token"),
    auth.WithScopes([]string{"read", "write"}),
)

manager := auth.NewAuthManager("", refresher)
header, err := manager.GetAuthHeader(ctx) // auto-refreshes expired tokens
```

HTTP client with auth middleware:

```go
middleware := auth.NewMiddleware(manager)
authedClient := middleware.WrapClient(http.DefaultClient)
```

### Configuration

Typed configuration map:

```go
cfg := config.Config{
    "api_key":     "key-123",
    "timeout":     30,
    "debug":       true,
    "temperature": 0.7,
}

key, ok := cfg.GetString("api_key")
timeout := cfg.GetIntWithDefault("timeout", 60)
debug := cfg.GetBoolWithDefault("debug", false)
temp := cfg.GetFloatWithDefault("temperature", 1.0)
```

Validation:

```go
validator := config.NewValidator()
validator.AddRule("api_key", config.Required("api_key"))
validator.AddRule("api_key", config.MinLength("api_key", 10))
validator.AddRule("mode", config.OneOf("mode", "development", "production"))

err := validator.Validate(cfg)
```

Environment variable loading:

```go
cfg := config.Config{}
cfg.LoadFromEnv("TOOLKIT_") // loads TOOLKIT_API_KEY -> "api_key", etc.
```

### Error Handling

Structured error types with classification:

```go
handler := errors.NewErrorHandler("siliconflow")
err := handler.HandleHTTPError(resp, body)

if errors.IsRetryable(err) {
    // retry the request
}
if errors.IsRateLimit(err) {
    retryAfter := errors.GetRetryAfter(err)
    time.Sleep(time.Duration(retryAfter) * time.Second)
}
if errors.IsAuth(err) {
    // refresh credentials
}
```

### Response Parsing

JSON parsing:

```go
parser := &response.JSONParser{}
err := parser.ParseJSON(httpResp, &result)
```

SSE streaming:

```go
streamParser := response.NewStreamingParser(
    func(data []byte) error {
        // handle each SSE data chunk
        return nil
    },
    func(err error) {
        // handle stream errors
    },
)
err := streamParser.ParseStream(httpResp)
```

Paginated responses:

```go
pagParser := response.NewPaginationParser(
    func(resp map[string]interface{}) bool { return resp["has_more"] == true },
    func(resp map[string]interface{}) string { return resp["next_url"].(string) },
)
hasNext, nextURL, err := pagParser.ParsePaginated(httpResp, &results)
```

## Usage Examples

### Creating a Provider and Running a Chat

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

### Using the Code Review Agent

```go
package main

import (
    "context"
    "fmt"

    _ "github.com/HelixDevelopment/helix_agent/Toolkit/Providers/SiliconFlow"
    "github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit"
    "github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit/agents"
)

func main() {
    provider, _ := toolkit.CreateProvider("siliconflow", map[string]interface{}{
        "api_key": "your-api-key",
    })

    reviewer := agents.NewCodeReviewAgent("Reviewer", provider)

    code := `func divide(a, b int) int { return a / b }`

    feedback, err := reviewer.Execute(context.Background(), code, map[string]interface{}{
        "language": "go",
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(feedback)
}
```

### Discovering Models

```go
provider, _ := toolkit.CreateProvider("siliconflow", map[string]interface{}{
    "api_key": "your-api-key",
})

models, err := provider.DiscoverModels(context.Background())
if err != nil {
    panic(err)
}

for _, model := range models {
    fmt.Printf("Model: %s (owned by %s)\n", model.ID, model.OwnedBy)
}
```

### Adding a Custom Provider

Create a new package under `Providers/` implementing the `Provider` interface:

```go
package myprovider

import "github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit"

func init() {
    toolkit.RegisterProviderFactory("myprovider", NewProvider)
}

func NewProvider(config map[string]interface{}) (toolkit.Provider, error) {
    // Build and return your provider
}
```

Then import it with a blank import:

```go
import _ "github.com/HelixDevelopment/helix_agent/Toolkit/Providers/MyProvider"
```

## Testing Utilities

The `Commons/testing` package provides mock implementations for unit testing:

```go
import ttesting "github.com/HelixDevelopment/helix_agent/Toolkit/Commons/testing"

func TestMyFeature(t *testing.T) {
    // Create mock provider
    mock := ttesting.NewMockProvider("test")
    fixtures := ttesting.NewTestFixtures()

    // Set expected response
    mock.SetChatResponse(fixtures.ChatResponse())

    // Use mock provider in your code
    agent := agents.NewGenericAgent("test", "desc", mock)
    result, err := agent.Execute(context.Background(), "hello", nil)
    assert.NoError(t, err)
    assert.NotEmpty(t, result)
}
```
