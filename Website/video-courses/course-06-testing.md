# Video Course 06: Testing Strategies for HelixAgent

## Course Overview

**Duration:** 3.5 hours
**Level:** Intermediate
**Prerequisites:** Course 01 (Fundamentals)

Master testing strategies for HelixAgent applications, from unit tests to chaos engineering.

---

## Module 1: Testing Fundamentals

### Video 1.1: Test Types Overview (15 min)

**Topics:**
- Unit tests - isolated component testing
- Integration tests - component interaction
- E2E tests - full system validation
- Security tests - vulnerability detection
- Stress tests - performance limits
- Chaos tests - failure resilience

**Test Pyramid:**
```
         ┌──────────┐
         │   E2E    │  Slow, High Coverage
         ├──────────┤
         │Integration│
         ├──────────┤
         │   Unit   │  Fast, Targeted
         └──────────┘
```

### Video 1.2: Setting Up the Test Environment (20 min)

**Topics:**
- Test infrastructure setup
- Docker Compose for testing
- Mock servers
- Test data fixtures

**Commands:**
```bash
# Start test infrastructure
make test-infra-start

# Run all tests
make test

# Run with coverage
make test-coverage
```

---

## Module 2: Unit Testing

### Video 2.1: Writing Effective Unit Tests (25 min)

**Topics:**
- Table-driven tests
- Mock generation
- Test helpers
- Coverage targets

**Code Example:**
```go
func TestEnsembleSelectHighestConfidence(t *testing.T) {
    tests := []struct {
        name      string
        responses []*models.LLMResponse
        expected  string
    }{
        {
            name: "selects highest confidence",
            responses: []*models.LLMResponse{
                {Content: "Low", Confidence: 0.3},
                {Content: "High", Confidence: 0.9},
                {Content: "Medium", Confidence: 0.6},
            },
            expected: "High",
        },
        {
            name: "handles empty responses",
            responses: []*models.LLMResponse{},
            expected: "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := selectHighestConfidence(tt.responses)
            if tt.expected == "" {
                assert.Nil(t, result)
            } else {
                assert.Equal(t, tt.expected, result.Content)
            }
        })
    }
}
```

### Video 2.2: Mocking LLM Providers (20 min)

**Topics:**
- Creating mock providers
- Response simulation
- Error injection
- Delay simulation

**Code Example:**
```go
type mockProvider struct {
    name       string
    response   *models.LLMResponse
    err        error
    delay      time.Duration
    callCount  int32
}

func (m *mockProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
    atomic.AddInt32(&m.callCount, 1)

    if m.delay > 0 {
        select {
        case <-time.After(m.delay):
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }

    if m.err != nil {
        return nil, m.err
    }
    return m.response, nil
}
```

### Video 2.3: Testing Handlers (25 min)

**Topics:**
- HTTP handler testing
- Request/response validation
- Middleware testing
- Error scenarios

**Code Example:**
```go
func TestCompletionHandler(t *testing.T) {
    router := setupTestRouter()

    t.Run("successful completion", func(t *testing.T) {
        req := httptest.NewRequest("POST", "/v1/chat/completions",
            strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"hi"}]}`))
        req.Header.Set("Content-Type", "application/json")

        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)

        assert.Equal(t, http.StatusOK, w.Code)

        var response map[string]interface{}
        json.Unmarshal(w.Body.Bytes(), &response)
        assert.NotEmpty(t, response["choices"])
    })
}
```

---

## Module 3: Integration Testing

### Video 3.1: Database Integration Tests (25 min)

**Topics:**
- Test database setup
- Transaction isolation
- Data seeding
- Cleanup strategies

**Code Example:**
```go
func TestTaskRepository_CreateAndGet(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    repo := NewTaskRepository(db)

    task := &models.BackgroundTask{
        ID:   uuid.New().String(),
        Type: "test-task",
        Status: models.TaskStatusPending,
    }

    err := repo.Create(context.Background(), task)
    require.NoError(t, err)

    retrieved, err := repo.GetByID(context.Background(), task.ID)
    require.NoError(t, err)
    assert.Equal(t, task.Type, retrieved.Type)
}
```

### Video 3.2: Redis Integration Tests (20 min)

**Topics:**
- Miniredis for testing
- Cache behavior validation
- TTL testing
- Pipeline operations

**Code Example:**
```go
func TestRedisCache(t *testing.T) {
    mr, err := miniredis.Run()
    require.NoError(t, err)
    defer mr.Close()

    client := redis.NewClient(&redis.Options{
        Addr: mr.Addr(),
    })
    cache := NewRedisCache(client)

    t.Run("set and get", func(t *testing.T) {
        err := cache.Set(context.Background(), "key", "value", time.Minute)
        require.NoError(t, err)

        val, err := cache.Get(context.Background(), "key")
        require.NoError(t, err)
        assert.Equal(t, "value", val)
    })
}
```

### Video 3.3: Provider Integration Tests (25 min)

**Topics:**
- Testing with real providers (optional)
- Mock server integration
- Network failure simulation
- Rate limit testing

**Demo:**
```bash
# Start mock LLM server
docker run -d --name mock-llm -p 8081:8080 helix/mock-llm-server

# Run integration tests
MOCK_LLM_URL=http://localhost:8081 go test ./tests/integration/...
```

---

## Module 4: End-to-End Testing

### Video 4.1: E2E Test Design (20 min)

**Topics:**
- User journey mapping
- Test scenario design
- Environment preparation
- Assertion strategies

### Video 4.2: Testing AI Debate Flows (25 min)

**Topics:**
- Debate creation and lifecycle
- Multi-pass validation testing
- SSE event verification
- Consensus validation

**Code Example:**
```go
func TestDebateE2E(t *testing.T) {
    client := NewTestClient(baseURL)

    // Create debate
    debate, err := client.CreateDebate(&DebateRequest{
        Topic: "Test topic",
        Participants: []string{"provider1", "provider2"},
        MaxRounds: 2,
    })
    require.NoError(t, err)

    // Wait for completion
    result, err := client.WaitForDebate(debate.ID, 60*time.Second)
    require.NoError(t, err)

    assert.Equal(t, "completed", result.Status)
    assert.NotEmpty(t, result.Consensus)
}
```

### Video 4.3: Protocol E2E Tests (20 min)

**Topics:**
- MCP tool execution flows
- LSP request/response cycles
- ACP agent workflows
- Combined protocol scenarios

---

## Module 5: Security Testing

### Video 5.1: Security Test Patterns (20 min)

**Topics:**
- Input validation testing
- Authentication bypass attempts
- Authorization boundary testing
- Injection prevention

**Code Example:**
```go
func TestAPIKeyValidation(t *testing.T) {
    tests := []struct {
        name     string
        apiKey   string
        expected int
    }{
        {"valid key", "sk-valid-key-123", http.StatusOK},
        {"empty key", "", http.StatusUnauthorized},
        {"invalid format", "invalid", http.StatusUnauthorized},
        {"sql injection", "'; DROP TABLE users;--", http.StatusUnauthorized},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest("GET", "/v1/models", nil)
            req.Header.Set("Authorization", "Bearer "+tt.apiKey)

            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expected, w.Code)
        })
    }
}
```

### Video 5.2: Dependency Scanning (15 min)

**Topics:**
- Go vulnerability scanning
- Docker image scanning
- Secret detection
- License compliance

**Commands:**
```bash
# Run security scan
make security-scan

# Scan for vulnerabilities
govulncheck ./...

# Check for secrets
gitleaks detect
```

---

## Module 6: Stress Testing

### Video 6.1: Load Testing Setup (20 min)

**Topics:**
- Load test tools (k6, vegeta)
- Scenario design
- Metric collection
- Baseline establishment

**Example k6 Script:**
```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    vus: 100,
    duration: '5m',
    thresholds: {
        http_req_duration: ['p(95)<500'],
        http_req_failed: ['rate<0.01'],
    },
};

export default function() {
    const payload = JSON.stringify({
        model: 'helix-ensemble',
        messages: [{ role: 'user', content: 'Hello' }],
    });

    const res = http.post('http://localhost:8080/v1/chat/completions', payload, {
        headers: { 'Content-Type': 'application/json' },
    });

    check(res, {
        'status is 200': (r) => r.status === 200,
        'response time < 500ms': (r) => r.timings.duration < 500,
    });

    sleep(1);
}
```

### Video 6.2: Performance Profiling (20 min)

**Topics:**
- pprof integration
- Memory profiling
- CPU profiling
- Goroutine analysis

**Commands:**
```bash
# CPU profile
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# Memory profile
go tool pprof http://localhost:8080/debug/pprof/heap

# Goroutine analysis
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

---

## Module 7: Chaos Testing

### Video 7.1: Chaos Engineering Principles (15 min)

**Topics:**
- Chaos engineering philosophy
- Failure injection strategies
- Steady state hypothesis
- Blast radius control

### Video 7.2: Provider Failure Scenarios (25 min)

**Topics:**
- Provider outage simulation
- Network partition testing
- Rate limit exhaustion
- Timeout scenarios

**Code Example:**
```go
func TestProviderFailover(t *testing.T) {
    // Primary provider fails
    primary := &mockProvider{err: errors.New("service unavailable")}
    fallback := &mockProvider{response: &models.LLMResponse{Content: "Fallback response"}}

    registry := NewProviderRegistry()
    registry.Register("primary", primary)
    registry.Register("fallback", fallback)

    ensemble := NewEnsemble(registry)

    response, err := ensemble.Execute(context.Background(), request)
    require.NoError(t, err)
    assert.Equal(t, "Fallback response", response.Content)
    assert.Equal(t, int32(1), primary.callCount)
    assert.Equal(t, int32(1), fallback.callCount)
}
```

### Video 7.3: Database and Cache Failures (20 min)

**Topics:**
- Database connection loss
- Cache invalidation
- Partial data corruption
- Recovery testing

---

## Module 8: CI/CD Integration

### Video 8.1: Test Automation Pipeline (20 min)

**Topics:**
- GitHub Actions setup
- Test parallelization
- Coverage reporting
- Quality gates

**Example Workflow:**
```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: test
      redis:
        image: redis:7

    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Run tests
        run: make test-coverage

      - name: Upload coverage
        uses: codecov/codecov-action@v4
```

### Video 8.2: Test Data Management (15 min)

**Topics:**
- Fixture management
- Test data generation
- Environment isolation
- Cleanup automation

---

## Hands-on Labs

### Lab 1: Unit Test Suite
Write comprehensive unit tests for a service module.

### Lab 2: Integration Test Database
Set up and test database repository operations.

### Lab 3: E2E Debate Flow
Create end-to-end tests for the AI debate system.

### Lab 4: Chaos Scenarios
Implement provider failure and recovery tests.

---

## Resources

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Testify Library](https://github.com/stretchr/testify)
- [K6 Load Testing](https://k6.io/docs/)
- [HelixAgent Test Examples](https://github.com/helix-agent/test-examples)

---

## Course Completion

Congratulations! You've completed the HelixAgent Testing Strategies course. You should now be able to:

- Write effective unit and integration tests
- Design E2E test scenarios
- Implement security and stress tests
- Apply chaos engineering principles
- Set up CI/CD test automation
