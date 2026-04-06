# HelixAgent Developer Guide

This guide provides comprehensive information for developers working on HelixAgent.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Development Environment](#development-environment)
3. [Core Components](#core-components)
4. [Adding New Features](#adding-new-features)
5. [Testing Strategy](#testing-strategy)
6. [Debugging](#debugging)
7. [Performance Optimization](#performance-optimization)
8. [Best Practices](#best-practices)

---

## Architecture Overview

### System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        API Gateway                               │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌───────┐ │
│  │  REST   │  │  gRPC   │  │   MCP   │  │   LSP   │  │  ACP  │ │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘  └───┬───┘ │
└───────┼────────────┼────────────┼────────────┼───────────┼─────┘
        │            │            │            │           │
┌───────▼────────────▼────────────▼────────────▼───────────▼─────┐
│                       Service Layer                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │  Debate  │  │ Ensemble │  │  Intent  │  │ Context  │       │
│  │ Service  │  │ Service  │  │Classifier│  │ Manager  │       │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘       │
└───────┼─────────────┼─────────────┼─────────────┼──────────────┘
        │             │             │             │
┌───────▼─────────────▼─────────────▼─────────────▼──────────────┐
│                      Provider Layer                             │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐       │
│  │ Claude │ │DeepSeek│ │ Gemini │ │Mistral │ │  ...   │       │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘       │
└─────────────────────────────────────────────────────────────────┘
        │             │             │             │
┌───────▼─────────────▼─────────────▼─────────────▼──────────────┐
│                    Infrastructure Layer                         │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐       │
│  │  DB    │ │ Redis  │ │ Qdrant │ │ Kafka  │ │Prometheus│     │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘       │
└─────────────────────────────────────────────────────────────────┘
```

### Request Flow

1. Request arrives at API Gateway
2. Authentication/Authorization middleware validates
3. Request routed to appropriate handler
4. Service layer processes business logic
5. Provider layer communicates with LLMs
6. Response aggregated and returned

---

## Development Environment

### Required Tools

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25.3+ | Primary language |
| Docker | 24+ | Containerization |
| Make | Any | Build automation |
| golangci-lint | 1.55+ | Code linting |
| govulncheck | Latest | Security scanning |

### Environment Setup

```bash
# 1. Clone repository (SSH only - HTTPS is forbidden)
git clone --recurse-submodules git@github.com:vasic-digital/HelixAgent.git
cd HelixAgent

# 2. Install Go dependencies
go mod download

# 3. Install dev tools
make install-deps

# 4. Copy environment file
cp .env.example .env

# 5. Configure API keys in .env
# DEEPSEEK_API_KEY=your-key
# GEMINI_API_KEY=your-key
# etc.

# 6. Verify setup
make build
make test-unit
```

### IDE Configuration

#### VS Code

Recommended extensions:
- Go (by Google)
- GitLens
- Error Lens

Settings (`settings.json`):
```json
{
    "go.useLanguageServer": true,
    "go.lintTool": "golangci-lint",
    "go.lintOnSave": "workspace",
    "go.testOnSave": true,
    "editor.formatOnSave": true
}
```

#### GoLand

1. Enable "Go Modules" integration
2. Set "File Watchers" for `gofmt`
3. Configure golangci-lint as external tool

---

## Core Components

### LLM Provider System

Location: `internal/llm/providers/`

Each provider implements the `LLMProvider` interface:

```go
type LLMProvider interface {
    Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
    CompleteStream(ctx context.Context, req *CompletionRequest) (<-chan *StreamChunk, error)
    HealthCheck(ctx context.Context) error
    GetCapabilities() *ProviderCapabilities
    ValidateConfig() error
}
```

### AI Debate System

Location: `internal/services/debate_service.go`

The debate system orchestrates multi-LLM consensus:

1. **Topic Introduction**: Present debate topic
2. **Position Gathering**: Each LLM provides position
3. **Cross-Examination**: LLMs critique each other
4. **Synthesis**: Generate consensus response

### Ensemble Voting

Location: `internal/llm/ensemble.go`

Voting strategies:
- **Majority Vote**: Most common response wins
- **Weighted Vote**: Provider scores influence votes
- **Confidence Weighted**: Response confidence affects weight

### Intent Classification

Location: `internal/services/llm_intent_classifier.go`

Uses LLM-based semantic understanding (not hardcoded patterns):

```go
type IntentClassifier interface {
    Classify(ctx context.Context, message string) (*IntentResult, error)
}
```

---

## Adding New Features

### Adding a New LLM Provider

1. **Create provider directory**:
```bash
mkdir -p internal/llm/providers/newprovider
```

2. **Implement provider**:
```go
// internal/llm/providers/newprovider/provider.go
package newprovider

type Provider struct {
    config  *Config
    client  *http.Client
    logger  *logrus.Logger
}

func NewProvider(config *Config, logger *logrus.Logger) *Provider {
    return &Provider{
        config: config,
        client: &http.Client{Timeout: config.Timeout},
        logger: logger,
    }
}

func (p *Provider) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
    // Implementation
}

// Implement remaining interface methods...
```

3. **Add tests**:
```go
// internal/llm/providers/newprovider/provider_test.go
func TestProvider_Complete(t *testing.T) {
    // Test implementation
}
```

4. **Register provider**:
```go
// internal/services/provider_registry.go
registry.Register("newprovider", func(config *Config) LLMProvider {
    return newprovider.NewProvider(config, logger)
})
```

5. **Update documentation**:
- Add to `CLAUDE.md`
- Add to provider documentation
- Update `.env.example`

### Adding a New API Endpoint

1. **Create handler**:
```go
// internal/handlers/new_handler.go
func (h *Handler) HandleNewEndpoint(c *gin.Context) {
    // Implementation
}
```

2. **Add route**:
```go
// internal/router/router.go
router.POST("/v1/new-endpoint", handler.HandleNewEndpoint)
```

3. **Add tests**:
```go
// internal/handlers/new_handler_test.go
func TestHandler_HandleNewEndpoint(t *testing.T) {
    // Test with httptest
}
```

### Adding a New Tool

1. **Define tool schema**:
```go
// internal/tools/schema.go
var NewTool = &Tool{
    Name:        "new_tool",
    Description: "Description of the tool",
    Parameters: Parameters{
        Type: "object",
        Properties: map[string]Property{
            "param1": {Type: "string", Description: "..."},
        },
        Required: []string{"param1"},
    },
}
```

2. **Implement handler**:
```go
// internal/tools/handler.go
func (h *ToolHandler) HandleNewTool(ctx context.Context, params map[string]interface{}) (interface{}, error) {
    // Implementation
}
```

---

## Testing Strategy

### Test Pyramid

```
        /\
       /  \      E2E Tests (few)
      /----\
     /      \    Integration Tests (moderate)
    /--------\
   /          \  Unit Tests (many)
  /------------\
        +
   Challenge Tests (70+ challenges, 100% pass rate)
```

### Test Commands

```bash
# Standard test commands
make test                  # Run all tests
make test-coverage         # Tests with coverage report
make test-unit             # Unit tests only
make test-integration      # Integration tests
make test-e2e              # End-to-end tests
make test-security         # Security penetration tests
make test-stress           # Stress/load tests
make test-chaos            # Chaos/challenge tests

# Challenge validation (70+ challenges)
./challenges/scripts/run_all_challenges.sh
```

### Unit Tests

```go
func TestFunction(t *testing.T) {
    // Arrange
    sut := NewSystemUnderTest()
    input := "test input"
    expected := "expected output"

    // Act
    result, err := sut.Function(input)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Integration Tests

```go
//go:build integration

func TestDatabaseIntegration(t *testing.T) {
    db := setupTestDB(t)
    defer teardownTestDB(t, db)

    // Test with real database
}
```

### Challenge Tests

Challenge tests are bash scripts that validate system behavior:

```bash
#!/bin/bash
# Example challenge test structure

# Test with strict result validation
result=$(curl -s http://localhost:7061/v1/endpoint)

# Validate non-empty response (strict validation)
if [[ -z "$result" ]]; then
    echo "FAIL: Empty response (FALSE SUCCESS detected)"
    exit 1
fi

# Validate expected content
if [[ "$result" != *"expected_field"* ]]; then
    echo "FAIL: Missing expected field"
    exit 1
fi

echo "PASS: Test completed"
```

### Test Infrastructure

**Important:** Infrastructure containers must be running before executing integration, E2E, or challenge tests.

```bash
# Start test containers (PostgreSQL:15432, Redis:16379, Mock LLM:18081)
make test-infra-start
# Or for Podman rootless fallback:
make test-infra-direct-start

# Run tests with infrastructure
DB_HOST=localhost DB_PORT=15432 DB_USER=helixagent DB_PASSWORD=helixagent123 DB_NAME=helixagent_db \
REDIS_HOST=localhost REDIS_PORT=16379 REDIS_PASSWORD=helixagent123 \
go test -v ./path/to/package

# All tests with infrastructure managed automatically
make test-with-infra

# Stop and clean up
make test-infra-stop
```

### Mocking

Use interfaces for mockability:

```go
// Define interface
type LLMClient interface {
    Complete(ctx context.Context, req *Request) (*Response, error)
}

// Mock implementation
type MockLLMClient struct {
    mock.Mock
}

func (m *MockLLMClient) Complete(ctx context.Context, req *Request) (*Response, error) {
    args := m.Called(ctx, req)
    return args.Get(0).(*Response), args.Error(1)
}
```

### Test Coverage Targets

| Priority | Coverage Target | Packages |
|----------|----------------|----------|
| Critical | 80%+ | core, handlers, services |
| High | 75%+ | providers, middleware |
| Normal | 70%+ | utilities, tools |

Current status: 50 packages at 80%+, 49 packages below target.

---

## Debugging

### Logging

```go
import "github.com/sirupsen/logrus"

logger.WithFields(logrus.Fields{
    "provider": "claude",
    "request_id": requestID,
}).Info("Processing request")
```

### Tracing

```go
import "dev.helix.agent/internal/observability"

ctx, span := tracer.Start(ctx, "operation-name")
defer span.End()

span.SetAttributes(attribute.String("key", "value"))
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Context deadline exceeded | Timeout too short | Increase timeout |
| Connection refused | Service not running | Start dependencies |
| Rate limited | Too many requests | Add backoff/retry |
| JSON unmarshal error | Schema mismatch | Check API response format |

### Debug Mode

```bash
# Run with debug logging
GIN_MODE=debug LOG_LEVEL=debug make run-dev

# Run specific test with verbose output
go test -v -run TestName ./path/to/package
```

---

## Performance Optimization

### Caching

```go
// Use Redis for distributed caching
cache := redis.NewCache(config)
result, err := cache.GetOrSet(ctx, key, func() (interface{}, error) {
    return expensiveOperation()
}, ttl)
```

### Connection Pooling

```go
// Database connection pool
db, err := pgx.NewPool(ctx, &pgx.PoolConfig{
    MaxConns: 100,
    MinConns: 10,
})
```

### Parallel Processing

```go
// Use errgroup for parallel operations
g, ctx := errgroup.WithContext(ctx)
for _, provider := range providers {
    p := provider
    g.Go(func() error {
        return p.Process(ctx)
    })
}
err := g.Wait()
```

---

## Best Practices

### Error Handling

```go
// Wrap errors with context
if err != nil {
    return nil, fmt.Errorf("failed to process request: %w", err)
}

// Use custom error types for specific cases
type ValidationError struct {
    Field   string
    Message string
}
```

### Context Usage

```go
// Always pass context
func Process(ctx context.Context, data *Data) error {
    // Check for cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    
    // Continue processing
}
```

### Resource Cleanup

```go
// Use defer for cleanup
func ProcessFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()
    
    // Process file
}
```

### Configuration

```go
// Use environment variables with defaults
func LoadConfig() *Config {
    return &Config{
        Port:    getEnvOrDefault("PORT", "8080"),
        Timeout: getDurationOrDefault("TIMEOUT", 30*time.Second),
    }
}
```

---

## Quick Reference

### Common Commands

```bash
# Build
make build

# Test
make test
make test-coverage

# Quality
make lint
make fmt

# Run
make run-dev

# Infrastructure
make test-infra-start
make test-infra-stop
```

### Package Documentation

```bash
# Generate godoc
godoc -http=:6060
# Visit http://localhost:6060/pkg/dev.helix.agent/
```

### API Documentation

Swagger UI available at `/swagger/index.html` when running in development mode.

---

## Challenge System

HelixAgent includes a comprehensive challenge validation system with **70+ challenges** achieving **100% pass rate**.

### Running Challenges

```bash
# Run all 70+ challenges
./challenges/scripts/run_all_challenges.sh

# Run specific challenge categories
./challenges/scripts/semantic_intent_challenge.sh                # Intent detection (19 tests)
./challenges/scripts/unified_verification_challenge.sh           # Verification (15 tests)
./challenges/scripts/debate_team_dynamic_selection_challenge.sh  # Debate team (12 tests)
./challenges/scripts/helixmemory_challenge.sh                    # Memory system (80+ tests)
./challenges/scripts/helixspecifier_challenge.sh                 # SpecKit (138 tests)
./challenges/scripts/full_system_boot_challenge.sh               # Full boot (53 tests)
```

### Challenge Categories

| Category | Tests | Description |
|----------|-------|-------------|
| Semantic Intent | 19 | Zero-hardcoding intent detection |
| AI Debate | 200+ | Debate orchestrator, reflexion, adversarial, voting, persistence |
| HelixMemory | 80+ | Unified cognitive memory engine |
| HelixSpecifier | 138 | Spec-driven development workflow |
| Provider | 40+ | Provider verification, URL consistency |
| CLI Agents | 60+ | Config generation, MCP server validation |
| Security | 55+ | Snyk, SonarQube, security scanning |
| System Boot | 53 | Full system boot and container orchestration |
| Release Build | 25 | Container-based build system |
| Userflow | 30+ | Browser, API, gRPC userflow testing |

### Writing New Challenges

1. Create challenge script in `challenges/scripts/`:

```bash
#!/bin/bash
# challenges/scripts/my_challenge.sh

set -e

echo "Running My Challenge..."

# Test 1
result=$(curl -s http://localhost:7061/v1/my-endpoint)
if [[ "$result" != *"expected"* ]]; then
    echo "FAIL: Test 1"
    exit 1
fi
echo "PASS: Test 1"

# Continue with more tests...

echo "All tests passed!"
```

2. Add to `run_all_challenges.sh`

3. Document in `challenges/README.md`

### Challenge Best Practices

- Use strict result validation (no empty responses)
- Set appropriate timeouts (60s for complex operations)
- Include error messages for debugging
- Test both success and failure cases
- Validate response schemas

---

## Troubleshooting

### Common Development Issues

#### ProviderHealthMonitor Mutex Deadlock

**Symptom**: Application hangs when checking provider health.

**Cause**: Lock held during external HTTP calls.

**Solution**: The fix has been applied. If you see similar patterns:
- Reduce lock scope
- Copy data before releasing lock
- Use RWMutex for read-heavy operations

#### CogneeService JSON Parsing

**Symptom**: `json: cannot unmarshal object into Go value of type []Dataset`

**Cause**: API response format varies between Cognee versions.

**Solution**: The fix has been applied. Use flexible JSON parsing:
```go
var result interface{}
json.Unmarshal(body, &result)
// Handle both array and object responses
```

#### Challenge Timeout

**Symptom**: `context deadline exceeded` during challenge execution.

**Solution**: Increase timeout:
```bash
# For RAGS challenge (now defaults to 60s)
RAGS_TIMEOUT=120 ./challenges/scripts/rags_challenge.sh
```

#### Test Infrastructure Not Running

**Symptom**: Database or Redis connection failures in tests.

**Solution**:
```bash
# Start test infrastructure
make test-infra-start

# Run tests with infrastructure
DB_HOST=localhost DB_PORT=15432 go test ./...
```

### Debug Logging

```bash
# Enable debug logging
GIN_MODE=debug LOG_LEVEL=debug make run-dev

# Enable provider debug logging
PROVIDER_DEBUG=true make run-dev

# Enable challenge verbose mode
VERBOSE=true ./challenges/scripts/run_all_challenges.sh
```

---

## Resources

- [Go Documentation](https://golang.org/doc/)
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [HelixAgent API Reference](./api/)
- [Comprehensive Completion Report](./reports/COMPREHENSIVE_AUDIT_REPORT.md)
- [Challenge Scripts](../challenges/scripts/)
