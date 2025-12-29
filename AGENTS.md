# AGENTS.md - Updated with Models.dev Integration

## Build/Lint/Test Commands
```bash
# Build
make build                 # Build SuperAgent binary
make build-debug           # Build with debug symbols
make build-all            # Build for all architectures (linux/darwin/windows, amd64/arm64)

# Run
make run                  # Run SuperAgent locally
make run-dev              # Run in development mode

# Test (6-tier testing strategy)
make test                 # Run all tests
make test-coverage        # Run tests with coverage report (generates coverage.html)
make test-coverage-100    # Run tests with 100% coverage requirement
make test-unit            # Unit tests only: go test -v ./internal/... -short
make test-integration     # Integration tests only: go test -v ./tests/integration
make test-e2e             # E2E tests only: go test -v ./tests/e2e
make test-security        # Security tests only: go test -v ./tests/security
make test-stress          # Stress tests only: go test -v ./tests/stress
make test-chaos           # Chaos tests only: go test -v ./tests/challenge
make test-all-types       # Run all 6 test types sequentially
make test-bench           # Run benchmark tests
make test-race            # Run tests with race detection

# Run single test
go test -v ./internal/modelsdev
go test -v ./internal/handlers -run TestModelMetadataHandler
go test -v ./internal/services -run TestModelMetadataService

# Code Quality
make fmt                  # Format Go code (gofmt)
make vet                  # Run go vet
make lint                 # Run golangci-lint
make security-scan        # Run gosec security scan
make all                  # fmt + vet + lint + test + build
```

## Code Style Guidelines

### General
- **Go version**: Go 1.23+ required (toolchain go1.24.11)
- **Formatting**: Always run `make fmt` before committing (uses gofmt)
- **Vetting**: Always run `make vet` to catch potential issues
- **Linting**: Use golangci-lint for comprehensive linting

### Import Organization
```go
import (
    // 1. Standard library
    "context"
    "fmt"
    "time"

    // 2. Third-party packages
    "github.com/gin-gonic/gin"
    "github.com/sirupsen/logrus"

    // 3. Internal packages
    "github.com/superagent/superagent/internal/models"
    "github.com/superagent/superagent/internal/services"
    "github.com/superagent/superagent/internal/modelsdev"
)
```

### Naming Conventions
- **Exports**: CamelCase (e.g., `CompletionHandler`, `ProcessRequest`)
- **Private**: camelCase (e.g., `requestService`, `convertToAPIResponse`)
- **Constants**: UPPER_SNAKE_CASE (e.g., `MAX_RETRIES`, `DEFAULT_TIMEOUT`)
- **Interfaces**: Simple, focused names ending in interface type behavior (e.g., `LLMProvider`)
- **Test Functions**: `Test<Struct>_<Method>_<Scenario>` (e.g., `TestCompletionHandler_Complete_Success`)

### Error Handling
```go
// Use structured error wrapping with context
if err != nil {
    return fmt.Errorf("failed to process request: %w", err)
}

// Use AppError for HTTP handler errors
appErr := utils.NewAppError("INVALID_REQUEST", "Missing required field", http.StatusBadRequest, err)
utils.HandleError(c, appErr)

// Always handle errors, never ignore them
result, err := someFunction()
if err != nil {
    logger.WithFields(logrus.Fields{
        "error": err,
        "context": "processing_request",
    }).Error("Failed to process")
    return err
}
```

### Structured Logging
```go
// Always include context in logs
logger.WithFields(logrus.Fields{
    "request_id": req.ID,
    "user_id": userID,
    "provider": providerName,
}).Info("Processing request")

// Use appropriate log levels
logger.Debug("Detailed debugging info")
logger.Info("Normal operation")
logger.Warn("Warning condition")
logger.Error("Error occurred")
```

### Testing
```go
// Use testify for assertions
import "github.com/stretchr/testify/assert"

func TestSomething_SpecificScenario(t *testing.T) {
    // Arrange
    handler := NewCompletionHandler(nil)
    req := &CompletionRequest{Prompt: "test"}

    // Act
    result, err := handler.Process(req)

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, "expected", result.Value)
}

// Test naming: Test<Struct>_<Method>_<Scenario>
// Minimum 95% coverage required
// Use httptest for HTTP testing
```

### Interface Design
- Keep interfaces small and focused
- Design for testability (dependency injection)
- Accept interfaces, return structs
- Example: `LLMProvider` interface with `Complete()`, `CompleteStream()`, `HealthCheck()`

### Type Definitions
```go
// Struct fields should be documented
type CompletionHandler struct {
    requestService *services.RequestService
}

// JSON tags for API types
type CompletionRequest struct {
    Prompt      string `json:"prompt" binding:"required"`
    Model       string `json:"model,omitempty"`
    Temperature float64 `json:"temperature,omitempty"`
}

// Use pointer types for optional fields in structs
```

### Context Usage
- Always pass `context.Context` as first parameter in functions that do I/O
- Use context for request-scoped values, timeouts, cancellation
- Example: `func (h *Handler) Process(ctx context.Context, req *Request) error`

### HTTP Handler Pattern
```go
func (h *Handler) Handle(c *gin.Context) {
    // 1. Parse request
    var req RequestType
    if err := c.ShouldBindJSON(&req); err != nil {
        utils.HandleError(c, err)
        return
    }

    // 2. Process (pass context)
    result, err := h.service.Process(c.Request.Context(), req)
    if err != nil {
        utils.HandleError(c, err)
        return
    }

    // 3. Return response
    c.JSON(http.StatusOK, result)
}
```

### Dependencies
- **Web Framework**: github.com/gin-gonic/gin
- **Logging**: github.com/sirupsen/logrus
- **Testing**: github.com/stretchr/testify
- **Database**: github.com/jackc/pgx/v5 (PostgreSQL)
- **Caching**: github.com/redis/go-redis/v9
- **Metrics**: github.com/prometheus/client_golang
- **Models.dev**: Custom client in internal/modelsdev

### Models.dev Integration Guidelines

#### Client Usage
```go
import "github.com/superagent/superagent/internal/modelsdev"

// Create client with configuration
client := modelsdev.NewClient(&modelsdev.ClientConfig{
    APIKey:    os.Getenv("MODELSDEV_API_KEY"),
    BaseURL:   "https://api.models.dev/v1",
    Timeout:   30 * time.Second,
    UserAgent: "SuperAgent/1.0",
})

// List models
models, err := client.ListModels(ctx, &modelsdev.ListModelsOptions{
    Provider: "anthropic",
    Limit:    100,
    Page:     1,
})

// Get model details
model, err := client.GetModel(ctx, "claude-3-sonnet-20240229")

// Search models
results, err := client.SearchModels(ctx, "code", nil)
```

#### Service Layer Integration
```go
import "github.com/superagent/superagent/internal/services"

// Initialize model metadata service
modelMetadataService := services.NewModelMetadataService(
    modelsdevClient,
    dbRepository,
    services.NewModelMetadataCache(1*time.Hour),
    &services.ModelMetadataConfig{
        RefreshInterval: 24*time.Hour,
        CacheTTL:        1*time.Hour,
        DefaultBatchSize: 100,
        MaxRetries:      3,
        EnableAutoRefresh: true,
    },
    logger,
)

// Get model information (with caching)
metadata, err := modelMetadataService.GetModel(ctx, "model-id")

// List models
models, total, err := modelMetadataService.ListModels(ctx, "anthropic", "", 1, 20)

// Compare models
models, err := modelMetadataService.CompareModels(ctx, []string{"model-1", "model-2"})

// Get models by capability
visionModels, err := modelMetadataService.GetModelsByCapability(ctx, "vision")
```

#### Database Repository Pattern
```go
import "github.com/superagent/superagent/internal/database"

// Repository interface for testability
type ModelMetadataRepository interface {
    CreateModelMetadata(ctx context.Context, metadata *ModelMetadata) error
    GetModelMetadata(ctx context.Context, modelID string) (*ModelMetadata, error)
    ListModels(ctx context.Context, providerID string, modelType string, limit int, offset int) ([]*ModelMetadata, int, error)
    SearchModels(ctx context.Context, searchTerm string, limit int, offset int) ([]*ModelMetadata, int, error)
    CreateBenchmark(ctx context.Context, benchmark *ModelBenchmark) error
    GetBenchmarks(ctx context.Context, modelID string) ([]*ModelBenchmark, error)
    CreateRefreshHistory(ctx context.Context, history *ModelsRefreshHistory) error
    GetLatestRefreshHistory(ctx context.Context, limit int) ([]*ModelsRefreshHistory, error)
    UpdateProviderSyncInfo(ctx context.Context, providerID string, totalModels int, enabledModels int) error
}
```

### Caching Strategy
```go
// Multi-layer cache with TTL
cache := services.NewModelMetadataCache(1*time.Hour)

// Cache operations
cache.Set(modelID, metadata)
metadata, exists := cache.Get(modelID)
cache.Delete(modelID)
cache.Clear()

// Cache size management
size := cache.Size()
if size > maxCacheSize {
    cache.Clear()
}
```

### API Handler Pattern
```go
import "github.com/superagent/superagent/internal/handlers"

type ModelMetadataHandler struct {
    service *services.ModelMetadataService
}

func NewModelMetadataHandler(service *services.ModelMetadataService) *ModelMetadataHandler {
    return &ModelMetadataHandler{
        service: service,
    }
}

// Example: List models
func (h *ModelMetadataHandler) ListModels(c *gin.Context) {
    var req ListModelsRequest
    if err := c.ShouldBindQuery(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    models, total, err := h.service.ListModels(c.Request.Context(), 
        req.Provider, req.ModelType, req.Page, req.Limit)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list models"})
        return
    }

    c.JSON(http.StatusOK, ListModelsResponse{
        Models:     models,
        Total:      total,
        Page:       req.Page,
        Limit:      req.Limit,
        TotalPages: (total + req.Limit - 1) / req.Limit,
    })
}
```

### Documentation
- Document all exported functions/types with examples
- Use godoc comments: `// FunctionName does something...`
- Run `make docs` to serve documentation at http://localhost:6060

### Security
- Never commit secrets or credentials
- Use environment variables for configuration
- Run `make security-scan` to check for security issues
- Validate all user inputs
- Use parameterized queries for database operations
- Rate limit API calls to Models.dev
- Implement proper authentication for admin endpoints

### Performance
- Use connection pooling for database
- Implement multi-layer caching (in-memory, database, API)
- Use proper indexing for database queries
- Implement pagination for large datasets
- Use streaming for large responses
- Monitor and optimize slow queries
- Use proper timeouts for all external API calls

### Monitoring
- Track cache hit/miss ratios
- Monitor API response times
- Log refresh operations with timing
- Track error rates and types
- Monitor database query performance
- Set up alerts for critical failures

### Testing Models.dev Integration
```go
// Mock Models.dev client for tests
type MockModelsDevClient struct {
    mock.Mock
}

// Test repository with in-memory database
func TestModelMetadataRepository_Create(t *testing.T) {
    // Use test database or mock
    // Test CRUD operations
    // Verify error handling
}

// Test service with mocks
func TestModelMetadataService_GetModel(t *testing.T) {
    // Mock repository and client
    // Test cache behavior
    // Test error scenarios
}

// Test handlers with httptest
func TestModelMetadataHandler_ListModels(t *testing.T) {
    // Use httptest for HTTP requests
    // Verify response format
    // Test error cases
}
```

### Environment Configuration
```bash
# Models.dev configuration
MODELSDEV_ENABLED=true
MODELSDEV_API_KEY=your-api-key-here
MODELSDEV_BASE_URL=https://api.models.dev/v1
MODELSDEV_REFRESH_INTERVAL=24h
MODELSDEV_CACHE_TTL=1h
MODELSDEV_BATCH_SIZE=100
MODELSDEV_MAX_RETRIES=3
MODELSDEV_AUTO_REFRESH=true
```

### Database Schema Notes
- `models_metadata`: Stores complete model information from Models.dev
- `model_benchmarks`: Stores benchmark results with upsert support
- `models_refresh_history`: Tracks all refresh operations for auditing
- Proper indexing on provider_id, model_type, tags, benchmarks
- JSONB fields for flexible data storage

### Refresh Mechanism
- Automatic refresh runs every 24 hours (configurable)
- Manual refresh available via POST /api/v1/models/refresh
- Provider-specific refresh available via POST /api/v1/models/refresh?provider=xxx
- Refresh history tracked in models_refresh_history table
- Graceful degradation if Models.dev API is unavailable

### Error Recovery
- Implement retry logic with exponential backoff
- Fall back to cached data on API failures
- Log all errors with context for debugging
- Continue operation with partial data if possible
- Alert on critical failures

### Development Workflow
```bash
# 1. Make code changes
# 2. Run tests
make test-unit -run ModelMetadataService

# 3. Run linter
make lint

# 4. Build
make build

# 5. Test manually
make run-dev
```

### Production Deployment
1. Set MODELSDEV_ENABLED=true
2. Configure MODELSDEV_API_KEY
3. Run database migration: `psql -f scripts/migrations/002_modelsdev_integration.sql`
4. Set appropriate refresh interval based on your needs
5. Monitor refresh history and error rates
6. Set up alerts for failed refreshes

### Common Patterns

#### Repository Pattern
- Interface-first design for testability
- Context passed to all methods
- Returns upsert-friendly errors
- Proper transaction handling

#### Service Pattern
- Dependency injection
- Cache-first approach
- Business logic separation
- Error wrapping with context

#### Handler Pattern
- Request validation
- Response format standardization
- HTTP status code correctness
- Error message consistency
