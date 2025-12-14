# AI Toolkit

A comprehensive, generic toolkit for building AI-powered applications with support for multiple providers and specialized agents.

## Features

- **Multi-Provider Support**: SiliconFlow, OpenRouter, NVIDIA, Claude, and extensible provider system
- **Specialized Agents**: Generic assistant, code review, and custom agent support with extensible architecture
- **Configuration Management**: Flexible configuration builders and validation
- **CLI Tool**: Command-line interface for easy management and testing
- **Advanced Testing**: Comprehensive test suite with fuzzing, benchmarking, and 85.8% coverage
- **Extensible Architecture**: Easy to add new providers and agents

## Installation

### Prerequisites

- Go 1.21 or later
- API keys for your chosen providers

### Install from Source

```bash
git clone https://github.com/superagent/toolkit.git
cd toolkit
go mod download
make build
```

### Build Commands

```bash
# Build the toolkit binary
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run linting
make lint

# Run security scan
make security-scan
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/superagent/toolkit/pkg/toolkit"
)

func main() {
    // Initialize the toolkit
    tk := toolkit.NewToolkit()

    // Configure a provider
    providerConfig := map[string]interface{}{
        "name":     "siliconflow",
        "api_key":  "your-api-key-here",
        "base_url": "https://api.siliconflow.com",
    }

    // Create and register provider
    provider, err := tk.CreateProvider("siliconflow", providerConfig)
    if err != nil {
        log.Fatal(err)
    }
    tk.RegisterProvider("siliconflow", provider)

    // Configure an agent
    agentConfig := map[string]interface{}{
        "name":        "my-assistant",
        "provider":    "siliconflow",
        "model":       "deepseek-chat",
        "max_tokens":  1000,
        "temperature": 0.7,
    }

    // Create and register agent
    agent, err := tk.CreateAgent("generic", agentConfig)
    if err != nil {
        log.Fatal(err)
    }
    tk.RegisterAgent("my-assistant", agent)

    // Execute a task
    ctx := context.Background()
    result, err := tk.ExecuteTask(ctx, "my-assistant", "Hello, how are you?", agentConfig)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result)
}
```

### Using the CLI

```bash
# Show version
toolkit version

# Run integration tests
toolkit test

# Start interactive chat session
toolkit chat --provider siliconflow --api-key YOUR_API_KEY

# Execute tasks with AI agents
toolkit agent --type generic --task "Explain quantum computing" --api-key YOUR_API_KEY

# With custom model and base URL
toolkit chat --provider siliconflow --api-key YOUR_API_KEY --model deepseek-chat --base-url https://api.siliconflow.com
```

## Architecture

### Core Components

#### Providers

Providers implement the `Provider` interface and handle communication with AI APIs:

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
    Rerank(ctx context.Context, req RerankRequest) (RerankResponse, error)
    DiscoverModels(ctx context.Context) ([]ModelInfo, error)
    ValidateConfig(config map[string]interface{}) error
}
```

#### Agents

Agents implement the `Agent` interface and provide specialized AI capabilities:

```go
type Agent interface {
    Name() string
    Execute(ctx context.Context, task string, config interface{}) (string, error)
    ValidateConfig(config interface{}) error
    Capabilities() []string
}
```

#### Configuration Builders

Configuration builders handle the creation and validation of provider and agent configurations:

```go
type ConfigBuilder interface {
    Build(config map[string]interface{}) (interface{}, error)
    Validate(config interface{}) error
    Merge(base, override interface{}) (interface{}, error)
}
```

### Built-in Providers

- **SiliconFlow**: High-performance AI inference platform
- **OpenRouter**: Unified API for multiple AI providers
- **NVIDIA**: Enterprise-grade AI solutions
- **Claude**: Anthropic's Claude models
- **Chutes**: Custom AI deployment platform

### Built-in Agents

- **Generic Agent**: General-purpose AI assistant
- **Code Review Agent**: Specialized in code analysis and review
- **OpenCode Agent**: Open-source development assistant
- **Crush Agent**: Performance optimization specialist

## Configuration

### Provider Configuration

```json
{
  "name": "siliconflow",
  "api_key": "your-api-key-here",
  "base_url": "https://api.siliconflow.com",
  "timeout": 30000,
  "retries": 3,
  "rate_limit": 60
}
```

### Agent Configuration

```json
{
  "name": "code-review-agent",
  "description": "Specialized code review assistant",
  "provider": "siliconflow",
  "model": "deepseek-coder",
  "max_tokens": 4096,
  "temperature": 0.3,
  "timeout": 60000,
  "retries": 3,
  "focus_areas": ["security", "performance", "maintainability"],
  "language": "go"
}
```

## Examples

### Basic Usage

See `examples/basic_usage/main.go` for a complete example of:
- Toolkit initialization
- Provider and agent registration
- Basic task execution
- Configuration validation

### Configuration Generation

See `examples/config_generation/main.go` for examples of:
- Generating configurations for multiple agents
- Environment variable integration
- JSON configuration file creation
- Configuration validation

### Integration Testing

See `examples/integration_test/main.go` for comprehensive testing:
- Toolkit initialization tests
- Provider and agent registry tests
- Configuration building tests
- Model discovery tests
- Concurrent operation tests

### Custom Provider

See `examples/custom_provider/main.go` for creating custom providers:
- Implementing the Provider interface
- Mock API responses for testing
- Configuration validation
- Error handling

### Custom Agent

See `examples/custom_agent/main.go` for creating custom agents:
- Implementing the Agent interface
- Specialized task execution
- Extended configuration options
- Capability reporting

## CLI Reference

### Commands

#### `toolkit version`
Show version information.

```bash
toolkit version
# Output: HelixAgent Toolkit v1.0.0
```

#### `toolkit test`
Run integration tests to validate provider connections.

```bash
toolkit test
# Runs basic provider creation and model discovery tests
```

#### `toolkit chat`
Start an interactive chat session with an AI provider.

```bash
# Basic chat
toolkit chat --provider siliconflow --api-key YOUR_API_KEY

# With custom model
toolkit chat --provider siliconflow --api-key YOUR_API_KEY --model deepseek-chat

# With custom base URL
toolkit chat --provider siliconflow --api-key YOUR_API_KEY --base-url https://api.siliconflow.com
```

#### `toolkit agent`
Execute tasks using specialized AI agents.

```bash
# Generic agent
toolkit agent --type generic --task "Explain machine learning" --api-key YOUR_API_KEY

# Code review agent
toolkit agent --type codereview --task "Review this Go function" --api-key YOUR_API_KEY

# With custom model
toolkit agent --type generic --task "Hello" --api-key YOUR_API_KEY --model deepseek-chat
```

### Flags

- `--provider, -p`: Provider name (siliconflow, chutes)
- `--api-key, -k`: API key for authentication
- `--base-url, -u`: Custom base URL for the provider
- `--model, -m`: Model name to use
- `--type, -t`: Agent type (generic, codereview)
- `--task`: Task description for agent execution

## Development

### Adding a New Provider

1. Create a new package in `providers/`
2. Implement the `Provider` interface
3. Add configuration builder
4. Register with the toolkit in `init()`

Example structure:
```
providers/
  myprovider/
    myprovider.go     # Provider implementation
    builder.go        # Configuration builder
    client.go         # API client
    discovery.go      # Model discovery
```

### Adding a New Agent

1. Create a new package in `agents/`
2. Implement the `Agent` interface
3. Add configuration builder
4. Register with the toolkit in `init()`

Example structure:
```
agents/
  myagent/
    myagent.go        # Agent implementation
    config.go         # Configuration handling
```

### Agents

The toolkit provides specialized AI agents for different use cases:

#### Generic Agent

A versatile AI assistant for general tasks:

```go
agent := agents.NewGenericAgent("Assistant", "A helpful AI assistant", provider)
result, err := agent.Execute(ctx, "Explain machine learning", nil)
```

#### Code Review Agent

Specialized agent for code analysis and review:

```go
agent := agents.NewCodeReviewAgent("CodeReviewer", provider)
feedback, err := agent.Execute(ctx, "func add(a, b int) int { return a + b }", map[string]interface{}{
    "language": "go",
})
```

### Testing

Run the test suite:

```bash
# Unit tests
make test-unit

# Integration tests
make test-integration

# All tests
make test

# With coverage
make test-coverage

# Fuzz tests
make test-fuzz
```

#### Test Coverage

The toolkit maintains high test coverage across all modules:

- **Overall Coverage**: 85.8%
- **Commons Modules**: 87.7% - 100%
  - `auth`: 87.9%
  - `config`: 97.1%
  - `discovery`: 99.0%
  - `errors`: 100.0%
  - `http`: 89.2%
  - `ratelimit`: 95.2%
  - `response`: 89.6%
  - `testing`: 87.7%
- **Provider Modules**: 72.5% - 92.1%
  - `Chutes`: 72.5%
  - `SiliconFlow`: 92.1%
- **pkg/toolkit**: 80.5% - 100%
  - `agents`: 100.0%
  - `common/*`: 80.5% - 100%
  - `interfaces`: 100.0%

Coverage reports are generated with `make test-coverage` and include detailed breakdowns by package.

#### Advanced Testing Features

- **Fuzz Testing**: Implemented fuzz tests for critical functions to ensure robustness against edge cases
- **Property-Based Testing**: Table-driven tests and invariant checks throughout the codebase
- **Concurrent Testing**: Comprehensive testing of concurrent operations in rate limiting and discovery
- **Integration Testing**: End-to-end tests for provider interactions with proper mocking
- **Performance Benchmarking**: Benchmarks for HTTP clients, rate limiters, and core operations

### Code Quality

```bash
# Format code
make fmt

# Lint code
make lint

# Security scan
make security-scan

# Vet code
make vet
```

## API Reference

### Provider Interface

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
    Rerank(ctx context.Context, req RerankRequest) (RerankResponse, error)
    DiscoverModels(ctx context.Context) ([]ModelInfo, error)
    ValidateConfig(config map[string]interface{}) error
}
```

### Agent Interface

```go
type Agent interface {
    Name() string
    Execute(ctx context.Context, task string, config interface{}) (string, error)
    ValidateConfig(config interface{}) error
    Capabilities() []string
}
```

### Request/Response Types

#### ChatRequest
```go
type ChatRequest struct {
    Model            string         `json:"model"`
    Messages         []ChatMessage  `json:"messages"`
    MaxTokens        int            `json:"max_tokens,omitempty"`
    Temperature      float64        `json:"temperature,omitempty"`
    TopP             float64        `json:"top_p,omitempty"`
    Stream           bool           `json:"stream,omitempty"`
    Stop             []string       `json:"stop,omitempty"`
    PresencePenalty  float64        `json:"presence_penalty,omitempty"`
    FrequencyPenalty float64        `json:"frequency_penalty,omitempty"`
}
```

#### ChatResponse
```go
type ChatResponse struct {
    ID      string       `json:"id"`
    Object  string       `json:"object"`
    Created int64        `json:"created"`
    Model   string       `json:"model"`
    Choices []ChatChoice `json:"choices"`
    Usage   Usage        `json:"usage"`
}
```

#### EmbeddingRequest/Response
```go
type EmbeddingRequest struct {
    Model          string   `json:"model"`
    Input          []string `json:"input"`
    User           string   `json:"user,omitempty"`
    EncodingFormat string   `json:"encoding_format,omitempty"`
    Dimensions     int      `json:"dimensions,omitempty"`
}

type EmbeddingResponse struct {
    Object string          `json:"object"`
    Data   []EmbeddingData `json:"data"`
    Model  string          `json:"model"`
    Usage  Usage           `json:"usage"`
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run the test suite
6. Submit a pull request

### Guidelines

- Follow Go conventions and best practices
- Add comprehensive tests for new features
- Update documentation for API changes
- Use structured logging
- Handle errors appropriately
- Maintain backward compatibility

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/superagent/toolkit/issues)
- **Discussions**: [GitHub Discussions](https://github.com/superagent/toolkit/discussions)
- **Documentation**: [API Reference](API_REFERENCE.md)

## Changelog

### v1.0.0
- Initial release
- Multi-provider support (SiliconFlow, OpenRouter, NVIDIA, Claude, Chutes)
- Built-in agents (Generic, Code Review, OpenCode, Crush)
- CLI tool
- Configuration management
- Integration testing suite
- Comprehensive documentation