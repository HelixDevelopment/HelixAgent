---
# AGENTS.md - AI Toolkit Repository

## Project Overview

The AI Toolkit is a comprehensive, generic toolkit for building AI-powered applications with support for multiple providers and specialized agents. Built in Go 1.21+, it provides a unified interface for interacting with various AI providers and agents through a clean, extensible architecture.

## Technology Stack

- **Language**: Go 1.21+
- **Architecture**: Interface-driven design with factory and registry patterns
- **Configuration**: JSON-based with environment variable fallback
- **CLI**: Custom command-line interface with built-in commands
- **Testing**: Mock-based unit testing and integration test suites

## Key Configuration Files

### Go Module Configuration
- **`go.mod`**: Defines module as `github.com/superagent/toolkit` with Go 1.21 minimum version

### Configuration Files
- **`configs/providers.json`**: Provider configurations (currently empty with null providers)
- **`configs/agents.json`**: Agent configurations (currently empty with null agents)
- **`configs/deployment.json`**: Deployment configuration for multi-agent system with resource allocation
- **`provider-siliconflow-config.json`**: Sample SiliconFlow provider configuration

## Project Structure

### Core Components
- **`pkg/toolkit/`**: Core toolkit library with interfaces and implementation
  - `interfaces.go`: Core Provider and Agent interface definitions
  - `toolkit.go`: Main Toolkit struct with registry management
  - `toolkit_types.go`: Type definitions for models, requests, responses
  - `registry.go`: Provider and agent registry implementations
  - `builders.go`: Configuration builders for providers and agents
  - `testing.go`: Testing utilities and mock implementations

### Provider Implementations
- **`providers/`**: Individual provider implementations
  - `siliconflow/`: SiliconFlow provider (in `SiliconFlow/` subdirectory)
  - `openrouter/`: OpenRouter unified API provider
  - `nvidia/`: NVIDIA enterprise AI solutions
  - `claude/`: Anthropic's Claude models
  - `chutes/`: Custom AI deployment platform
  - `deepseek/`: DeepSeek models

### Agent Implementations
- **`agents/`**: Individual agent implementations
  - `generic/`: General-purpose AI assistant
  - `crush/`: Performance optimization specialist
  - `opencode/`: Open-source development assistant

### CLI and Examples
- **`cmd/toolkit/`**: CLI tool entry point (`main_multi_provider.go`)
- **`examples/`**: Example usage patterns and integration tests
  - `basic_usage/`: Basic toolkit usage demonstration
  - `integration_test/`: Comprehensive integration test suite
  - `config_generation/`: Configuration generation examples

### Common Utilities
- **`pkg/toolkit/common/`**: Shared utilities
  - `http/`: HTTP client utilities
  - `discovery/`: Model discovery services
  - `config/`: Configuration management
  - `auth/`: Authentication utilities
  - `errors/`: Error handling
  - `ratelimit/`: Rate limiting
  - `response/`: Response handling
  - `testing/`: Common testing utilities

## Core Interfaces

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

## Build and Test Commands

### Building
```bash
# Build the CLI tool
go build -o toolkit ./cmd/toolkit

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o toolkit-linux ./cmd/toolkit
GOOS=darwin GOARCH=amd64 go build -o toolkit-darwin ./cmd/toolkit
GOOS=windows GOARCH=amd64 go build -o toolkit-windows.exe ./cmd/toolkit
```

### Testing
```bash
# Run unit tests (currently no test files exist)
go test ./...

# Run integration tests
go run ./examples/integration_test/main.go

# Run specific integration test
go test -v ./examples/integration_test/
```

### CLI Usage
```bash
# List available providers and agents
./toolkit list all

# Execute a task with an agent
./toolkit execute generic "Explain quantum computing in simple terms"

# Discover models from providers
./toolkit discover siliconflow

# Generate configuration files
./toolkit config generate provider siliconflow
./toolkit config generate agent generic

# Validate configuration
./toolkit validate provider siliconflow config.json

# Run integration tests
./toolkit test
```

## Code Style Guidelines

### Go Standards
- **Formatting**: Use `gofmt` for consistent formatting
- **Imports**: Group imports (stdlib, third-party, internal)
- **Naming**: Use CamelCase for exports, camelCase for private
- **Error handling**: Always handle errors, use structured error wrapping with `fmt.Errorf`

### Project-Specific Conventions
- **Interface-first design**: All major components implement defined interfaces
- **Factory pattern**: Use factories for component creation
- **Registry pattern**: Use registries for component management
- **Configuration-driven**: Support both file and environment-based configuration
- **Context-aware**: All operations accept and respect context cancellation
- **Thread-safe**: All registries use mutex protection for concurrent access

### Registration Pattern
- **Providers register via init()**: Import package triggers registration
- **CLI must import all packages**: Add imports to `cmd/toolkit/main_multi_provider.go`
- **Factory pattern**: Use factories for dynamic creation, not direct instantiation

## Configuration Management

### Provider Configuration
```json
{
  "name": "providername",
  "api_key": "your-api-key-here",
  "base_url": "https://api.example.com",
  "timeout": 30000,
  "retries": 3,
  "rate_limit": 60
}
```

### Agent Configuration
```json
{
  "name": "agentname",
  "description": "Agent description",
  "provider": "providername",
  "model": "model-id",
  "max_tokens": 4096,
  "temperature": 0.7,
  "timeout": 30000,
  "retries": 3
}
```

### Environment Variables
- `SILICONFLOW_API_KEY`: SiliconFlow API key
- `OPENROUTER_API_KEY`: OpenRouter API key
- `NVIDIA_API_KEY`: NVIDIA API key
- `ANTHROPIC_API_KEY`: Claude API key

### Configuration Loading Order
1. Check `configs/providers.json` and `configs/agents.json`
2. If files don't exist, use environment variables
3. Fall back to default configurations
4. Command-line options override all other sources

## Development Patterns

### Adding a New Provider
1. Create package in `providers/providername/`
2. Implement the `Provider` interface
3. Create a configuration builder
4. Register with factory registry in `init()` function
5. Import the provider in the CLI (`cmd/toolkit/main_multi_provider.go`)

**Example Provider Structure**:
```
providers/myprovider/
  myprovider.go     # Provider implementation
  builder.go        # Configuration builder
  client.go         # HTTP client (if needed)
  discovery.go      # Model discovery (if needed)
```

### Adding a New Agent
1. Create package in `agents/agentname/`
2. Implement the `Agent` interface
3. Create a configuration builder
4. Register with factory registry in `init()` function
5. Import the agent in the CLI (`cmd/toolkit/main_multi_provider.go`)

**Example Agent Structure**:
```
agents/myagent/
  myagent.go        # Agent implementation
  config.go         # Configuration handling
```

## Testing Strategies

### Unit Testing
- Uses Go's standard `testing` package
- Mock implementations available in `pkg/toolkit/testing.go`
- `MockProvider` and `MockAgent` for isolated testing
- `TestRunner` and `TestSuite` for structured testing

### Integration Testing
- Located in `examples/integration_test/`
- Tests real provider/agent combinations
- Requires actual API keys for full testing
- `IntegrationTestHelper` provides utilities

### Test Patterns
```go
// Create mock provider
provider := NewMockProvider("test")
provider.SetChatResponse(expectedResponse)

// Create mock agent
agent := NewMockAgent("test")
agent.SetExecuteResult("expected result");

// Test with helper
helper := NewIntegrationTestHelper()
helper.SetupProvider("test", provider)
helper.SetupAgent("test", agent)
helper.TestProviderAgentCombination(t, "test", "test", "generic", config)
```

## Security Considerations

### API Key Management
- API keys are stored in environment variables or configuration files
- Never commit API keys to version control
- Use secure configuration management practices

### Network Security
- All API calls should use HTTPS
- Implement proper timeout handling
- Use context cancellation for request management

### Error Handling
- Sanitize error messages before logging
- Avoid exposing sensitive information in errors
- Implement proper input validation

## Important Gotchas

### Registration Dependencies
- Built-in providers/agents register via `init()` functions when packages are imported
- CLI must import all provider/agent packages to trigger registration
- Missing imports will result in "provider not found" errors

### Configuration Validation
- Always validate configurations before creating instances
- Use structured error messages with context
- Implement graceful degradation for optional features

### Concurrent Access
- All registry operations are thread-safe
- Provider/agent instances are shared across calls
- Use appropriate locking for custom implementations

### Context Handling
- All operations accept and respect context cancellation
- Implement proper timeout handling
- Use context values for request-scoped data

## Deployment Process

### Local Development
1. Clone the repository
2. Set up environment variables for API keys
3. Build the toolkit: `go build -o toolkit ./cmd/toolkit`
4. Test with `./toolkit list all`

### Production Deployment
1. Build for target platform
2. Configure providers via JSON files or environment variables
3. Deploy with appropriate security measures
4. Monitor logs and performance

### Docker Deployment
- No Dockerfile currently exists in the project
- Consider creating multi-stage builds for production
- Include configuration management in container setup

## Common Issues and Solutions

### Build Issues
- Ensure Go 1.21+ is installed
- Check module dependencies with `go mod tidy`
- Verify all imports are available

### Runtime Issues
- Check API key validity
- Verify network connectivity to providers
- Review configuration file syntax

### Registration Issues
- Ensure all provider/agent packages are imported in CLI
- Check for duplicate registrations
- Verify factory function signatures

---

This document serves as a comprehensive guide for AI coding agents working with the AI Toolkit project. It covers the essential architecture, development patterns, testing strategies, and operational considerations needed to effectively work with this codebase.