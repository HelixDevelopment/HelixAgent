# HelixAgent Go SDK

A comprehensive Go SDK for the HelixAgent AI orchestration platform, providing idiomatic Go access to multi-provider LLM capabilities, AI debates, and advanced features.

## Installation

```bash
go get dev.helix.agent-go
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "dev.helix.agent-go"
)

func main() {
    client := helixagent.NewClient(&helixagent.Config{
        APIKey: "your-api-key",
        BaseURL: "https://api.helixagent.ai",
    })

    // Simple chat completion
    resp, err := client.Chat.Completions.Create(context.Background(), &helixagent.ChatCompletionRequest{
        Model: "helixagent-ensemble",
        Messages: []helixagent.ChatMessage{
            {Role: "user", Content: "Explain quantum computing"},
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

## Configuration

```go
config := &helixagent.Config{
    APIKey:      "your-api-key",
    BaseURL:     "https://api.helixagent.ai",
    Timeout:     30 * time.Second,
    MaxRetries:  3,
    UserAgent:   "MyApp/1.0",
    Debug:       false,
}

client := helixagent.NewClient(config)
```

## Authentication

The SDK supports multiple authentication methods:

```go
// API Key authentication (default)
client := helixagent.NewClient(&helixagent.Config{
    APIKey: "your-api-key",
})

// JWT Token authentication
client := helixagent.NewClient(&helixagent.Config{
    Token: "your-jwt-token",
})

// Custom HTTP client
httpClient := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    },
}

client := helixagent.NewClientWithHTTP(&helixagent.Config{
    APIKey: "your-api-key",
}, httpClient)
```

## Chat Completions

### Basic Chat Completion

```go
resp, err := client.Chat.Completions.Create(context.Background(), &helixagent.ChatCompletionRequest{
    Model: "helixagent-ensemble",
    Messages: []helixagent.ChatMessage{
        {Role: "system", Content: "You are a helpful assistant."},
        {Role: "user", Content: "What is machine learning?"},
    },
    MaxTokens:   500,
    Temperature: 0.7,
})
if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.Choices[0].Message.Content)
fmt.Printf("Usage: %d tokens\n", resp.Usage.TotalTokens)
```

### Streaming Chat Completion

```go
stream, err := client.Chat.Completions.CreateStream(context.Background(), &helixagent.ChatCompletionRequest{
    Model: "deepseek-chat",
    Messages: []helixagent.ChatMessage{
        {Role: "user", Content: "Tell me a story"},
    },
    Stream: true,
})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

for stream.Next() {
    chunk := stream.Current()
    if chunk.Choices[0].Delta.Content != "" {
        fmt.Print(chunk.Choices[0].Delta.Content)
    }
}

if err := stream.Err(); err != nil {
    log.Fatal(err)
}
```

### Ensemble Completion

```go
resp, err := client.Ensemble.Completions.Create(context.Background(), &helixagent.EnsembleCompletionRequest{
    Messages: []helixagent.ChatMessage{
        {Role: "user", Content: "What is the future of AI?"},
    },
    EnsembleConfig: &helixagent.EnsembleConfig{
        Strategy:            helixagent.StrategyConfidenceWeighted,
        MinProviders:        3,
        ConfidenceThreshold: 0.8,
        FallbackToBest:      true,
    },
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Ensemble result: %s\n", resp.Choices[0].Message.Content)
fmt.Printf("Providers used: %v\n", resp.Ensemble.ProvidersUsed)
fmt.Printf("Confidence: %.2f\n", resp.Ensemble.ConfidenceScore)
```

## Text Completions

### Basic Text Completion

```go
resp, err := client.Completions.Create(context.Background(), &helixagent.CompletionRequest{
    Model:       "qwen-max",
    Prompt:      "The future of technology is",
    MaxTokens:   100,
    Temperature: 0.8,
    Stop:        []string{"\n", "."},
})
if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.Choices[0].Text)
```

### Streaming Text Completion

```go
stream, err := client.Completions.CreateStream(context.Background(), &helixagent.CompletionRequest{
    Model:  "openrouter/grok-4",
    Prompt: "Write a haiku about programming:",
    Stream: true,
})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

for stream.Next() {
    chunk := stream.Current()
    fmt.Print(chunk.Choices[0].Text)
}

if err := stream.Err(); err != nil {
    log.Fatal(err)
}
```

## AI Debate System

### Creating a Basic Debate

```go
debateConfig := &helixagent.DebateConfig{
    DebateID:            "ai-ethics-debate-001",
    Topic:               "Should AI systems have ethical constraints built into their core architecture?",
    MaximalRepeatRounds: 5,
    ConsensusThreshold:  0.75,
    Participants: []helixagent.DebateParticipant{
        {
            Name: "EthicsExpert",
            Role: "AI Ethics Specialist",
            LLMs: []helixagent.LLMConfig{
                {
                    Provider: "claude",
                    Model:    "claude-3-5-sonnet-20241022",
                    APIKey:   os.Getenv("CLAUDE_API_KEY"),
                },
            },
        },
        {
            Name: "AIScientist",
            Role: "AI Research Scientist",
            LLMs: []helixagent.LLMConfig{
                {
                    Provider: "deepseek",
                    Model:    "deepseek-coder",
                },
            },
        },
    },
}

debate, err := client.Debates.Create(context.Background(), debateConfig)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Debate created: %s\n", debate.DebateID)
```

### Monitoring Debate Progress

```go
// Get debate status
status, err := client.Debates.GetStatus(context.Background(), "ai-ethics-debate-001")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Status: %s\n", status.Status)
fmt.Printf("Progress: Round %d/%d\n", status.CurrentRound, status.TotalRounds)

// Wait for completion
for status.Status != "completed" && status.Status != "failed" {
    time.Sleep(5 * time.Second)
    status, err = client.Debates.GetStatus(context.Background(), "ai-ethics-debate-001")
    if err != nil {
        log.Fatal(err)
    }
}
```

### Getting Debate Results

```go
results, err := client.Debates.GetResults(context.Background(), "ai-ethics-debate-001")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Topic: %s\n", results.Topic)
fmt.Printf("Consensus achieved: %t\n", results.Consensus.Achieved)
fmt.Printf("Final position: %s\n", results.Consensus.FinalPosition)

for _, participant := range results.Participants {
    fmt.Printf("%s: %d responses, avg quality: %.2f\n",
        participant.Name, participant.TotalResponses, participant.AvgQualityScore)
}
```

### Advanced Debate with Cognee Enhancement

```go
debateConfig := &helixagent.DebateConfig{
    DebateID:            "enhanced-debate-001",
    Topic:               "How should society regulate artificial general intelligence?",
    EnableCognee:        true,
    CogneeConfig: &helixagent.CogneeConfig{
        DatasetName:         "agi_regulation_debate",
        EnhancementStrategy: "hybrid",
        MaxEnhancementTime:  30000,
    },
    Participants: []helixagent.DebateParticipant{
        {
            Name:          "PolicyMaker",
            Role:          "Government Policy Advisor",
            EnableCognee:  true,
            CogneeSettings: &helixagent.CogneeParticipantConfig{
                EnhanceResponses: true,
                AnalyzeSentiment: true,
                DatasetName:      "policy_debate_data",
            },
        },
        {
            Name:         "AIRiskExpert",
            Role:         "AI Safety Researcher",
            EnableCognee: true,
        },
    },
}

debate, err := client.Debates.Create(context.Background(), debateConfig)
if err != nil {
    log.Fatal(err)
}
```

## Model Context Protocol (MCP)

### Getting MCP Capabilities

```go
capabilities, err := client.MCP.Capabilities(context.Background())
if err != nil {
    log.Fatal(err)
}

fmt.Printf("MCP Version: %s\n", capabilities.Version)
fmt.Printf("Available providers: %v\n", capabilities.Providers)
```

### Listing MCP Tools

```go
tools, err := client.MCP.Tools(context.Background())
if err != nil {
    log.Fatal(err)
}

for _, tool := range tools.Tools {
    fmt.Printf("Tool: %s - %s\n", tool.Name, tool.Description)
}
```

### Executing MCP Tools

```go
result, err := client.MCP.Tools.Call(context.Background(), &helixagent.MCPToolCall{
    Name: "read_file",
    Arguments: map[string]interface{}{
        "path": "/etc/hostname",
    },
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Result: %s\n", result.Result)
```

### MCP Prompts

```go
prompts, err := client.MCP.Prompts(context.Background())
if err != nil {
    log.Fatal(err)
}

for _, prompt := range prompts.Prompts {
    fmt.Printf("Prompt: %s - %s\n", prompt.Name, prompt.Description)
}
```

### MCP Resources

```go
resources, err := client.MCP.Resources(context.Background())
if err != nil {
    log.Fatal(err)
}

for _, resource := range resources.Resources {
    fmt.Printf("Resource: %s - %s\n", resource.Name, resource.Description)
}
```

## Provider Management

### Listing Available Providers

```go
providers, err := client.Providers.List(context.Background())
if err != nil {
    log.Fatal(err)
}

for _, provider := range providers.Providers {
    fmt.Printf("%s: %s - %d models\n", provider.Name, provider.Status, len(provider.Models))
}
```

### Provider Health Check

```go
health, err := client.Providers.Health(context.Background())
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Overall status: %s\n", health.Status)
for name, status := range health.Providers {
    fmt.Printf("%s: %s (response time: %.2fs)\n", name, status.Status, status.ResponseTime)
}
```

## Error Handling

The SDK provides structured error handling:

```go
import "dev.helix.agent-go/errors"

resp, err := client.Chat.Completions.Create(context.Background(), &helixagent.ChatCompletionRequest{
    Model: "invalid-model",
    Messages: []helixagent.ChatMessage{
        {Role: "user", Content: "Hello"},
    },
})
if err != nil {
    switch e := err.(type) {
    case *errors.AuthenticationError:
        log.Printf("Authentication failed: %s", e.Message)
    case *errors.RateLimitError:
        log.Printf("Rate limit exceeded: %s", e.Message)
    case *errors.ProviderError:
        log.Printf("Provider error: %s", e.Message)
    case *errors.ValidationError:
        log.Printf("Validation error: %s", e.Message)
    default:
        log.Printf("Unknown error: %v", err)
    }
    return
}
```

## Advanced Configuration

### Custom HTTP Client

```go
transport := &http.Transport{
    Proxy: http.ProxyURL(proxyURL),
    TLSClientConfig: &tls.Config{
        InsecureSkipVerify: false,
    },
}

httpClient := &http.Client{
    Timeout:   30 * time.Second,
    Transport: transport,
}

client := helixagent.NewClientWithHTTP(&helixagent.Config{
    APIKey: "your-api-key",
}, httpClient)
```

### Context and Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := client.Chat.Completions.Create(ctx, &helixagent.ChatCompletionRequest{
    Model: "helixagent-ensemble",
    Messages: []helixagent.ChatMessage{
        {Role: "user", Content: "Write a long response"},
    },
    MaxTokens: 2000,
})

// Context will automatically cancel the request after 30 seconds
```

### Logging and Debugging

```go
config := &helixagent.Config{
    APIKey: "your-api-key",
    Debug:  true, // Enable debug logging
    Logger: myCustomLogger,
}

client := helixagent.NewClient(config)
```

## Best Practices

### 1. Error Handling

```go
func safeCompletion(ctx context.Context, model string, messages []helixagent.ChatMessage) (string, error) {
    resp, err := client.Chat.Completions.Create(ctx, &helixagent.ChatCompletionRequest{
        Model:    model,
        Messages: messages,
        MaxTokens: 1000,
    })
    if err != nil {
        var rateLimitErr *errors.RateLimitError
        if errors.As(err, &rateLimitErr) {
            // Wait and retry
            time.Sleep(time.Minute)
            return safeCompletion(ctx, model, messages)
        }

        var providerErr *errors.ProviderError
        if errors.As(err, &providerErr) && strings.HasPrefix(model, "claude") {
            // Fallback to different provider
            return safeCompletion(ctx, "deepseek-chat", messages)
        }

        return "", err
    }

    return resp.Choices[0].Message.Content, nil
}
```

### 2. Resource Management

```go
func processBatch(ctx context.Context, prompts []string) ([]string, error) {
    results := make([]string, len(prompts))
    sem := make(chan struct{}, 5) // Limit to 5 concurrent requests

    var wg sync.WaitGroup
    var mu sync.Mutex
    var errs []error

    for i, prompt := range prompts {
        wg.Add(1)
        go func(idx int, p string) {
            defer wg.Done()

            sem <- struct{}{}        // Acquire
            defer func() { <-sem }() // Release

            content, err := safeCompletion(ctx, "helixagent-ensemble", []helixagent.ChatMessage{
                {Role: "user", Content: p},
            })

            mu.Lock()
            if err != nil {
                errs = append(errs, err)
            } else {
                results[idx] = content
            }
            mu.Unlock()
        }(i, prompt)
    }

    wg.Wait()

    if len(errs) > 0 {
        return nil, fmt.Errorf("batch processing failed: %v", errs)
    }

    return results, nil
}
```

### 3. Debate Orchestration

```go
type DebateOrchestrator struct {
    client     *helixagent.Client
    activeDebates map[string]*helixagent.Debate
    mu         sync.RWMutex
}

func NewDebateOrchestrator(client *helixagent.Client) *DebateOrchestrator {
    return &DebateOrchestrator{
        client:         client,
        activeDebates:  make(map[string]*helixagent.Debate),
    }
}

func (d *DebateOrchestrator) CreateDebate(ctx context.Context, config *helixagent.DebateConfig) (*helixagent.Debate, error) {
    debate, err := d.client.Debates.Create(ctx, config)
    if err != nil {
        return nil, err
    }

    d.mu.Lock()
    d.activeDebates[debate.DebateID] = debate
    d.mu.Unlock()

    return debate, nil
}

func (d *DebateOrchestrator) MonitorDebate(ctx context.Context, debateID string) (*helixagent.DebateStatus, error) {
    status, err := d.client.Debates.GetStatus(ctx, debateID)
    if err != nil {
        return nil, err
    }

    if status.Status == "completed" || status.Status == "failed" {
        d.mu.Lock()
        delete(d.activeDebates, debateID)
        d.mu.Unlock()
    }

    return status, nil
}

func (d *DebateOrchestrator) GetActiveDebates() []*helixagent.Debate {
    d.mu.RLock()
    debates := make([]*helixagent.Debate, 0, len(d.activeDebates))
    for _, debate := range d.activeDebates {
        debates = append(debates, debate)
    }
    d.mu.RUnlock()
    return debates
}
```

### 4. Connection Pooling

```go
config := &helixagent.Config{
    APIKey: "your-api-key",
    HTTPClient: &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:        100,
            MaxConnsPerHost:     10,
            IdleConnTimeout:     90 * time.Second,
            DisableCompression:  false,
        },
    },
}

client := helixagent.NewClient(config)
```

## API Reference

### Types

```go
type Config struct {
    APIKey      string
    Token       string
    BaseURL     string
    Timeout     time.Duration
    MaxRetries  int
    UserAgent   string
    Debug       bool
    Logger      Logger
    HTTPClient  *http.Client
}

type ChatCompletionRequest struct {
    Model       string
    Messages    []ChatMessage
    MaxTokens   int
    Temperature float64
    Stream      bool
}

type DebateConfig struct {
    DebateID            string
    Topic               string
    MaximalRepeatRounds int
    ConsensusThreshold  float64
    Participants        []DebateParticipant
    EnableCognee        bool
    CogneeConfig        *CogneeConfig
}
```

### Errors

- `AuthenticationError`: Authentication failures
- `RateLimitError`: Rate limit exceeded
- `ProviderError`: Provider-specific errors
- `ValidationError`: Input validation errors
- `NetworkError`: Network connectivity issues

## Requirements

- Go 1.19+
- Standard library only (no external dependencies for core functionality)

## Testing

```go
func TestChatCompletion(t *testing.T) {
    client := helixagent.NewClient(&helixagent.Config{
        APIKey: "test-key",
        BaseURL: "http://localhost:7061", // Use test server
    })

    resp, err := client.Chat.Completions.Create(context.Background(), &helixagent.ChatCompletionRequest{
        Model: "test-model",
        Messages: []helixagent.ChatMessage{
            {Role: "user", Content: "Hello"},
        },
    })

    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Greater(t, len(resp.Choices), 0)
}
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for new functionality
4. Ensure all tests pass (`go test ./...`)
5. Run linting (`golangci-lint run`)
6. Commit changes (`git commit -am 'Add amazing feature'`)
7. Push to branch (`git push origin feature/amazing-feature`)
8. Create a Pull Request

## License

MIT License - see LICENSE file for details.