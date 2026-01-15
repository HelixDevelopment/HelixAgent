# LLM Package

The `llm` package provides a unified interface for interacting with multiple Large Language Model providers. It handles provider abstraction, health monitoring, circuit breaking, and ensemble orchestration.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      LLMProvider Interface                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────┐  │
│  │   Claude   │  │  DeepSeek  │  │   Gemini   │  │  Ollama  │  │
│  └────────────┘  └────────────┘  └────────────┘  └──────────┘  │
│                                                                  │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────┐  │
│  │    Qwen    │  │    ZAI     │  │ OpenRouter │  │   Zen    │  │
│  └────────────┘  └────────────┘  └────────────┘  └──────────┘  │
│                                                                  │
│  ┌────────────┐  ┌────────────┐                                 │
│  │  Mistral   │  │  Cerebras  │                                 │
│  └────────────┘  └────────────┘                                 │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│  Circuit Breaker │ Health Monitor │ Lazy Provider │ Retry Logic │
└─────────────────────────────────────────────────────────────────┘
```

## LLMProvider Interface

All providers implement this core interface:

```go
type LLMProvider interface {
    // Complete sends a request and returns a response
    Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)

    // CompleteStream returns a channel of streaming responses
    CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error)

    // HealthCheck verifies provider connectivity
    HealthCheck() error

    // GetCapabilities returns provider feature support
    GetCapabilities() *models.ProviderCapabilities

    // ValidateConfig validates provider configuration
    ValidateConfig(config map[string]interface{}) (bool, []string)
}
```

## Components

### Provider Interface (`provider.go`)

The core LLMProvider interface that all providers implement.

### Circuit Breaker (`circuit_breaker.go`)

Prevents cascading failures when providers become unhealthy:

```go
breaker := llm.NewCircuitBreaker(llm.CircuitBreakerConfig{
    FailureThreshold:   5,
    ResetTimeout:       30 * time.Second,
    HalfOpenMaxCalls:   3,
})

response, err := breaker.Execute(func() (*models.LLMResponse, error) {
    return provider.Complete(ctx, request)
})
```

States:
- **Closed**: Normal operation, requests pass through
- **Open**: Provider failed, requests blocked
- **Half-Open**: Testing if provider recovered

### Health Monitor (`health_monitor.go`)

Continuous health checking for all providers:

```go
monitor := llm.NewHealthMonitor(providers, llm.HealthMonitorConfig{
    CheckInterval:  30 * time.Second,
    Timeout:        10 * time.Second,
    UnhealthyAfter: 3,
})

monitor.Start(ctx)
defer monitor.Stop()

// Get healthy providers
healthy := monitor.GetHealthyProviders()
```

### Lazy Provider (`lazy_provider.go`)

Deferred initialization for providers:

```go
provider := llm.NewLazyProvider(func() (llm.LLMProvider, error) {
    return providers.NewClaudeProvider(apiKey)
})

// Provider initializes on first use
response, err := provider.Complete(ctx, request)
```

### Retry Logic (`retry.go`)

Configurable retry with exponential backoff:

```go
retryer := llm.NewRetryer(llm.RetryConfig{
    MaxRetries:     3,
    InitialBackoff: 100 * time.Millisecond,
    MaxBackoff:     10 * time.Second,
    Multiplier:     2.0,
})

response, err := retryer.Do(ctx, func() (*models.LLMResponse, error) {
    return provider.Complete(ctx, request)
})
```

### Ensemble Orchestration (`ensemble.go`)

Run requests against multiple providers in parallel:

```go
responses, selected, err := llm.RunEnsembleWithProviders(request, providers)
// `selected` is the response with highest confidence
```

## Providers Subpackage

Individual provider implementations in `providers/`:

| Provider | Description |
|----------|-------------|
| `claude` | Anthropic Claude API |
| `deepseek` | DeepSeek API |
| `gemini` | Google Gemini API |
| `ollama` | Local Ollama models |
| `qwen` | Alibaba Qwen/DashScope |
| `zai` | ZAI API |
| `openrouter` | OpenRouter aggregator |
| `zen` | Zen (OpenCode) free API |
| `mistral` | Mistral AI API |
| `cerebras` | Cerebras API |

### Cognee Subpackage

The `cognee/` subpackage provides knowledge graph integration:

```go
provider := cognee.NewCogneeProvider(config)
// Adds RAG capabilities to LLM responses
```

## Files

| File | Description |
|------|-------------|
| `provider.go` | Core LLMProvider interface |
| `types.go` | Common type definitions |
| `circuit_breaker.go` | Circuit breaker pattern implementation |
| `health_monitor.go` | Provider health monitoring |
| `lazy_provider.go` | Lazy initialization wrapper |
| `retry.go` | Retry logic with backoff |
| `ensemble.go` | Multi-provider ensemble execution |
| `providers/` | Individual provider implementations |
| `cognee/` | Knowledge graph integration |

## Usage

### Basic Provider Usage

```go
provider := providers.NewClaudeProvider(apiKey, "claude-3-opus-20240229")

request := &models.LLMRequest{
    Prompt:      "Explain quantum computing",
    MaxTokens:   1000,
    Temperature: 0.7,
}

response, err := provider.Complete(ctx, request)
if err != nil {
    log.Fatal(err)
}

fmt.Println(response.Content)
```

### Streaming Usage

```go
stream, err := provider.CompleteStream(ctx, request)
if err != nil {
    log.Fatal(err)
}

for chunk := range stream {
    fmt.Print(chunk.Content)
}
```

### With Circuit Breaker

```go
safeProvider := llm.WrapWithCircuitBreaker(provider, breakerConfig)
response, err := safeProvider.Complete(ctx, request)
```

## Testing

```bash
go test -v ./internal/llm/...
```

Tests cover:
- Provider interface compliance
- Circuit breaker state transitions
- Health monitoring lifecycle
- Retry logic and backoff
- Ensemble orchestration
- Concurrent access patterns
