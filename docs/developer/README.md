# SuperAgent Developer Documentation

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Development Setup](#development-setup)
3. [Core Components](#core-components)
4. [Provider Integration](#provider-integration)
5. [Plugin Development](#plugin-development)
6. [Testing Guidelines](#testing-guidelines)
7. [Deployment](#deployment)
8. [Contributing](#contributing)

---

## Architecture Overview

### System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Client Applications                      │
├─────────────────────────────────────────────────────────────┤
│                    API Gateway                              │
├─────────────────────────────────────────────────────────────┤
│                  SuperAgent Core                            │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐ │
│  │   Debate    │  Provider   │   Cognee    │ Monitoring  │ │
│  │   Service   │  Manager    │  Service    │  Service    │ │
│  └─────────────┴─────────────┴─────────────┴─────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                  LLM Providers                              │
│  ┌─────────┬───────────┬─────────┬─────────┬─────────┬────┐ │
│  │ Claude  │ DeepSeek  │ Gemini  │  Qwen   │   Zai   │Olla│ │
│  └─────────┴───────────┴─────────┴─────────┴─────────┴────┘ │
├─────────────────────────────────────────────────────────────┤
│               Infrastructure Layer                          │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐ │
│  │ PostgreSQL  │    Redis    │  Prometheus │   Grafana   │ │
│  └─────────────┴─────────────┴─────────────┴─────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Core Services

1. **DebateService**: Orchestrates AI debates with multiple participants
2. **ProviderManager**: Manages LLM provider integrations
3. **CogneeService**: Provides AI enhancement and semantic analysis
4. **MonitoringService**: Tracks performance and system health
5. **HistoryService**: Persists debate history and analytics
6. **SecurityService**: Handles authentication and authorization

### Data Flow

```
Client Request → API Gateway → Debate Service → Provider Manager
                                                      ↓
Cognee Service ← Monitoring Service ← LLM Providers
     ↓                 ↓
History Service ← Security Service
```

---

## Development Setup

### Prerequisites

```bash
# Install Go 1.23+
go version  # Should be ≥ 1.23

# Install development tools
make install-tools

# Install database dependencies
make install-db-deps
```

### Local Development Environment

```bash
# Clone repository
git clone https://github.com/superagent/superagent.git
cd superagent

# Install dependencies
go mod download

# Start development services
docker-compose -f docker-compose.dev.yml up -d

# Run database migrations
make migrate-up

# Start development server
make dev
```

### Development Configuration

Create `config.dev.yaml`:

```yaml
server:
  host: "localhost"
  port: 8080
  debug: true

database:
  host: "localhost"
  port: 5432
  name: "superagent_dev"
  user: "superagent"
  password: "dev_password"

providers:
  claude:
    api_key: "dev-claude-key"
    model: "claude-3-sonnet-20240229"
    enabled: true
    timeout: 30000
  
  deepseek:
    api_key: "dev-deepseek-key"
    model: "deepseek-chat"
    enabled: true
    timeout: 30000

logging:
  level: "debug"
  format: "json"
```

---

## Core Components

### Debate Service

**File**: `internal/services/debate_service.go`

```go
type DebateService struct {
    logger *logrus.Logger
}

func (ds *DebateService) ConductDebate(
    ctx context.Context,
    config *DebateConfig,
) (*DebateResult, error) {
    // Implementation details
}
```

**Key Methods:**
- `ConductDebate()`: Main debate orchestration
- `ValidateConfig()`: Configuration validation
- `BuildConsensus()`: Consensus building algorithm

### Provider Registry

**File**: `internal/services/provider_registry.go`

```go
type ProviderRegistry struct {
     providers       map[string]llm.LLMProvider
     circuitBreakers map[string]*CircuitBreaker
     config          *RegistryConfig
     ensemble        *EnsembleService
     requestService  *RequestService
     memory          *MemoryService
     mu              sync.RWMutex
}
```

**Key Features:**
- **Circuit Breaker Protection**: Automatic failure detection and recovery for LLM providers
- **Provider Management**: Registration, health monitoring, and capability tracking
- **Ensemble Support**: Multi-provider voting and consensus building
- **Caching Integration**: Request caching with Redis backend

**Circuit Breaker Configuration:**
```go
type CircuitBreakerConfig struct {
    Enabled          bool          `json:"enabled"`
    FailureThreshold int           `json:"failure_threshold"`
    RecoveryTimeout  time.Duration `json:"recovery_timeout"`
    SuccessThreshold int           `json:"success_threshold"`
}
```

**Key Methods:**
- `RegisterProvider()`: Register new LLM providers with circuit breaker protection
- `GetProvider()`: Retrieve providers with automatic failover
- `HealthCheck()`: Monitor provider health and circuit breaker status

### Cognee Integration

**File**: `internal/services/cognee_service.go`

```go
type CogneeService struct {
     client *cognee.Client
     config *CogneeConfig
}

func (cs *CogneeService) EnhanceResponse(
     ctx context.Context,
     response string,
     dataset string,
 ) (*EnhancedResponse, error) {
     // Cognee AI enhancement
 }
```

**Key Methods:**
- `EnhanceResponse()`: Response enhancement
- `AnalyzeConsensus()`: Consensus analysis
- `GenerateInsights()`: Insight generation

### Circuit Breaker System

**File**: `internal/services/plugin_system.go`

```go
type CircuitBreaker struct {
    state                CircuitState
    failureThreshold     int
    successThreshold     int
    timeout              time.Duration
    consecutiveFailures  int
    consecutiveSuccesses int
    lastFailure          time.Time
    mu                   sync.Mutex
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    // Circuit breaker protection with automatic recovery
}
```

**States:**
- `StateClosed`: Normal operation
- `StateOpen`: Circuit breaker tripped, requests fail fast
- `StateHalfOpen`: Testing recovery with limited requests

**Benefits:**
- Automatic failure detection and recovery
- Prevents cascade failures in distributed systems
- Configurable thresholds and timeouts
- Thread-safe implementation

---

## Provider Integration

### Adding a New Provider

#### Step 1: Create Provider Implementation

**File**: `internal/llm/providers/new_provider.go`

```go
package providers

import (
    "context"
    "github.com/superagent/superagent/internal/models"
)

type NewProvider struct {
    apiKey  string
    baseURL string
    model   string
    logger  *logrus.Logger
}

func NewNewProvider(
    apiKey string,
    baseURL string,
    model string,
    logger *logrus.Logger,
) (*NewProvider, error) {
    return &NewProvider{
        apiKey:  apiKey,
        baseURL: baseURL,
        model:   model,
        logger:  logger,
    }, nil
}

func (np *NewProvider) Complete(
    ctx context.Context,
    req *models.LLMRequest,
) (*models.LLMResponse, error) {
    // Implement provider-specific API calls
}

func (np *NewProvider) GetCapabilities() *models.ProviderCapabilities {
    return &models.ProviderCapabilities{
        SupportedModels:         []string{np.model},
        SupportedFeatures:       []string{"completion", "streaming"},
        SupportsStreaming:       true,
        SupportsFunctionCalling: false,
        Limits: models.ModelLimits{
            MaxTokens:             32000,
            MaxInputLength:        32000,
            MaxOutputLength:       4096,
            MaxConcurrentRequests: 50,
        },
    }
}
```

#### Step 2: Register Provider

**File**: `internal/llm/provider_registry.go`

```go
func (r *ProviderRegistry) RegisterDefaultProviders() error {
    // Add new provider
    newProvider, err := providers.NewNewProvider(
        r.config.Providers["new_provider"].APIKey,
        r.config.Providers["new_provider"].BaseURL,
        r.config.Providers["new_provider"].Model,
        r.logger,
    )
    if err != nil {
        return fmt.Errorf("failed to create new provider: %w", err)
    }
    
    return r.RegisterProvider("new_provider", newProvider)
}
```

#### Step 3: Configuration Schema

**File**: `internal/config/providers.go`

```go
type NewProviderConfig struct {
    APIKey    string  `yaml:"api_key" json:"api_key"`
    BaseURL   string  `yaml:"base_url" json:"base_url"`
    Model     string  `yaml:"model" json:"model"`
    Enabled   bool    `yaml:"enabled" json:"enabled"`
    Weight    float64 `yaml:"weight" json:"weight"`
    Timeout   int     `yaml:"timeout" json:"timeout"`
}
```

### Provider Capabilities

Each provider must implement:

```go
type LLMProvider interface {
    Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
    CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error)
    HealthCheck() error
    GetCapabilities() *models.ProviderCapabilities
    ValidateConfig(config map[string]interface{}) (bool, []string)
}
```

---

## Plugin Development

### Plugin Architecture

```
┌─────────────────┐
│   Plugin API    │
├─────────────────┤
│ Plugin Manager  │
├─────────────────┤
│ Plugin Loader   │
├─────────────────┤
│ Loaded Plugins  │
└─────────────────┘
```

### Creating a Plugin

#### Step 1: Define Plugin Interface

**File**: `plugins/example/plugin.go`

```go
package main

import (
    "context"
    "github.com/superagent/superagent/plugins"
)

type ExamplePlugin struct {
    name    string
    version string
}

func (p *ExamplePlugin) Name() string {
    return p.name
}

func (p *ExamplePlugin) Version() string {
    return p.version
}

func (p *ExamplePlugin) Capabilities() []string {
    return []string{"pre_debate", "post_debate", "response_filter"}
}

func (p *ExamplePlugin) Execute(
    ctx context.Context,
    capability string,
    data interface{},
) (interface{}, error) {
    switch capability {
    case "pre_debate":
        return p.handlePreDebate(ctx, data)
    case "post_debate":
        return p.handlePostDebate(ctx, data)
    case "response_filter":
        return p.filterResponse(ctx, data)
    default:
        return data, nil
    }
}

func (p *ExamplePlugin) handlePreDebate(
    ctx context.Context,
    data interface{},
) (interface{}, error) {
    // Pre-debate processing
    return data, nil
}

// Plugin entry point
func NewPlugin() plugins.Plugin {
    return &ExamplePlugin{
        name:    "example-plugin",
        version: "1.0.0",
    }
}
```

#### Step 2: Build Plugin

```bash
go build -buildmode=plugin -o example.so plugins/example/plugin.go
```

#### Step 3: Load Plugin

```go
plugin, err := pluginManager.LoadPlugin(ctx, "example", "./plugins/example.so")
if err != nil {
    return fmt.Errorf("failed to load plugin: %w", err)
}
```

### Plugin Hooks

Available hooks:

- `pre_debate`: Before debate starts
- `post_debate`: After debate completes
- `pre_round`: Before each round
- `post_round`: After each round
- `response_filter`: Filter participant responses
- `consensus_check`: Custom consensus logic
- `report_generation`: Custom report formatting

---

## Testing Guidelines

### Unit Testing

```go
func TestDebateService_ConductDebate(t *testing.T) {
    logger := logrus.New()
    service := NewDebateService(logger)
    
    config := &DebateConfig{
        DebateID: "test-debate-001",
        Topic:    "Test topic",
        Participants: []ParticipantConfig{
            {
                ParticipantID: "test-1",
                Name:          "Test Participant 1",
                Role:          "proponent",
                LLMProvider:   "mock",
                LLMModel:      "mock-model",
            },
            {
                ParticipantID: "test-2",
                Name:          "Test Participant 2",
                Role:          "opponent",
                LLMProvider:   "mock",
                LLMModel:      "mock-model",
            },
        },
        MaxRounds: 3,
        Timeout:   60 * time.Second,
    }
    
    result, err := service.ConductDebate(context.Background(), config)
    
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, config.DebateID, result.DebateID)
    assert.True(t, result.Success)
}
```

### Integration Testing

```go
func TestIntegration_CompleteDebateWorkflow(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Setup test environment
    ctx := context.Background()
    config := setupIntegrationTestConfig(t)
    
    // Create all services
    debateService := services.NewDebateService(testLogger)
    monitoringService := services.NewDebateMonitoringService(testLogger)
    // ... other services
    
    // Execute complete workflow
    debateConfig := createTestDebateConfig()
    result, err := debateService.ConductDebate(ctx, debateConfig)
    
    // Assert results
    require.NoError(t, err)
    assert.True(t, result.Success)
    assert.NotNil(t, result.Consensus)
}
```

### Mock Testing

```go
type MockLLMProvider struct {
    mock.Mock
}

func (m *MockLLMProvider) Complete(
    ctx context.Context,
    req *models.LLMRequest,
) (*models.LLMResponse, error) {
    args := m.Called(ctx, req)
    return args.Get(0).(*models.LLMResponse), args.Error(1)
}

func TestDebateService_WithMockProvider(t *testing.T) {
    mockProvider := &MockLLMProvider{}
    mockProvider.On("Complete", mock.Anything, mock.Anything).
        Return(&models.LLMResponse{
            Content: "Mock response",
            Confidence: 0.9,
        }, nil)
    
    // Use mock provider in tests
}
```

### Performance Testing

```go
func BenchmarkDebateService_ConductDebate(b *testing.B) {
    service := NewDebateService(testLogger)
    config := createBenchmarkConfig()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := service.ConductDebate(context.Background(), config)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

---

## Deployment

### Docker Deployment

```dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/superagent

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/configs ./configs

EXPOSE 8080 9090
CMD ["./main"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: superagent
spec:
  replicas: 3
  selector:
    matchLabels:
      app: superagent
  template:
    metadata:
      labels:
        app: superagent
    spec:
      containers:
      - name: superagent
        image: superagent/superagent:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        env:
        - name: SUPERAGENT_API_KEY
          valueFrom:
            secretKeyRef:
              name: superagent-secrets
              key: api-key
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: superagent-secrets
              key: database-url
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### Environment Configuration

```bash
# Production environment variables
export SUPERAGENT_API_KEY=your-production-api-key
export JWT_SECRET=your-jwt-secret
export DATABASE_URL=postgres://user:pass@prod-db:5432/superagent
export REDIS_URL=redis://prod-redis:6379
export COGNEE_API_KEY=your-cognee-api-key

# Provider API keys (production)
export CLAUDE_API_KEY=your-production-claude-key
export DEEPSEEK_API_KEY=your-production-deepseek-key
export GEMINI_API_KEY=your-production-gemini-key
export QWEN_API_KEY=your-production-qwen-key
export ZAI_API_KEY=your-production-zai-key
```

---

## Contributing

### Development Workflow

1. **Fork Repository**: Create your fork
2. **Create Feature Branch**: `git checkout -b feature/amazing-feature`
3. **Make Changes**: Implement your feature
4. **Write Tests**: Add comprehensive tests
5. **Run Tests**: `make test`
6. **Lint Code**: `make lint`
7. **Commit Changes**: `git commit -m "Add amazing feature"`
8. **Push Branch**: `git push origin feature/amazing-feature`
9. **Create PR**: Submit pull request

### Code Style

```go
// Follow standard Go conventions
package services

import (
    "context"
    "github.com/sirupsen/logrus"
)

// Use meaningful names
type DebateService struct {
    logger *logrus.Logger
}

// Add comprehensive comments
// ConductDebate orchestrates a debate between multiple AI participants
func (ds *DebateService) ConductDebate(
    ctx context.Context,
    config *DebateConfig,
) (*DebateResult, error) {
    // Implementation with clear logic flow
}
```

### Commit Message Guidelines

```
type(scope): description

Types:
- feat: New feature
- fix: Bug fix
- docs: Documentation changes
- style: Code style changes
- refactor: Code refactoring
- test: Test changes
- chore: Build/tooling changes

Examples:
feat(debate): add consensus building algorithm
fix(provider): handle rate limit errors correctly
docs(api): update OpenAPI specification
```

### Testing Requirements

- **Unit Tests**: Minimum 90% coverage for core services
- **Integration Tests**: All major workflows tested
- **Performance Tests**: Benchmarks for critical paths
- **Security Tests**: Authentication and authorization
- **End-to-End Tests**: Complete user workflows

---

## Advanced Topics

### Custom Consensus Algorithms

```go
type ConsensusAlgorithm interface {
    CalculateConsensus(responses []ParticipantResponse) (*ConsensusResult, error)
    GetAlgorithmName() string
    GetParameters() map[string]interface{}
}

type WeightedVotingConsensus struct {
    weights map[string]float64
}

func (w *WeightedVotingConsensus) CalculateConsensus(
    responses []ParticipantResponse,
) (*ConsensusResult, error) {
    // Implement weighted voting logic
}
```

### Advanced Provider Selection

```go
type ProviderSelector interface {
    SelectProvider(
        ctx context.Context,
        request *models.LLMRequest,
        availableProviders []string,
    ) (string, error)
}

type IntelligentSelector struct {
    predictor *ml.Predictor
}

func (s *IntelligentSelector) SelectProvider(
    ctx context.Context,
    request *models.LLMRequest,
    availableProviders []string,
) (string, error) {
    // Use ML to predict best provider for request
}
```

### Distributed Deployment

```yaml
# Kubernetes StatefulSet for distributed deployment
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: superagent-cluster
spec:
  serviceName: superagent
  replicas: 5
  template:
    spec:
      containers:
      - name: superagent
        image: superagent/superagent:latest
        env:
        - name: NODE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: CLUSTER_MODE
          value: "distributed"
```

---

## Resources

### Documentation

- [API Documentation](../api/README.md)
- [User Guide](../user/README.md)
- [Architecture Diagrams](../architecture/)

### Tools

- **API Testing**: Postman, Insomnia
- **Performance**: k6, wrk
- **Monitoring**: Prometheus, Grafana
- **Debugging**: Delve, pprof

### Community

- **GitHub**: https://github.com/superagent/superagent
- **Discord**: https://discord.gg/superagent
- **Forum**: https://community.superagent.ai
- **Blog**: https://blog.superagent.ai

---

*Last Updated: 2024-12-27*
*Version: 1.0.0*