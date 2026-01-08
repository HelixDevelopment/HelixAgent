# Module 11: Testing and CI/CD

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 11: Testing and CI/CD
- Duration: 75 minutes
- Quality Assurance and Automation

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Master HelixAgent testing strategies
- Write effective unit and integration tests
- Set up comprehensive CI/CD pipelines
- Implement quality gates

---

## Slide 3: Testing Strategy

**Test Pyramid for HelixAgent:**

```
         /\
        /  \
       / E2E\        <- Few, expensive
      /------\
     /  Integ \      <- Some, moderate
    /----------\
   /    Unit    \    <- Many, fast
  /--------------\
```

---

## Slide 4: Test Types Overview

**Available Test Commands:**

| Command | Purpose | Duration |
|---------|---------|----------|
| `make test` | All tests | ~5 min |
| `make test-unit` | Unit tests | ~1 min |
| `make test-integration` | Integration | ~3 min |
| `make test-e2e` | End-to-end | ~10 min |
| `make test-security` | Security | ~2 min |
| `make test-stress` | Load testing | ~15 min |

---

## Slide 5: Running Tests

**Basic Test Commands:**

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific test
go test -v -run TestName ./path/to/package

# Run with race detection
make test-race

# Run benchmarks
make test-bench
```

---

## Slide 6: Unit Testing

**Unit Test Characteristics:**

- Test single function/method
- Mock external dependencies
- Fast execution (<100ms each)
- No external services required

```bash
make test-unit
# Runs: go test -v -short ./internal/...
```

---

## Slide 7: Unit Test Example

**Testing a Provider:**

```go
func TestClaudeProvider_Complete(t *testing.T) {
    // Arrange
    cfg := &Config{
        APIKey: "test-key",
        Model:  "claude-3-sonnet",
    }
    provider := NewClaudeProvider(cfg)

    // Mock HTTP client
    mockClient := &MockHTTPClient{}
    mockClient.On("Post", mock.Anything).Return(
        &http.Response{
            StatusCode: 200,
            Body:       mockBody,
        },
        nil,
    )
    provider.client = mockClient

    // Act
    resp, err := provider.Complete(ctx, request)

    // Assert
    assert.NoError(t, err)
    assert.NotEmpty(t, resp.Content)
    mockClient.AssertExpectations(t)
}
```

---

## Slide 8: Mocking External Services

**Using testify/mock:**

```go
// Mock interface
type MockLLMProvider struct {
    mock.Mock
}

func (m *MockLLMProvider) Complete(
    ctx context.Context,
    req *Request,
) (*Response, error) {
    args := m.Called(ctx, req)
    return args.Get(0).(*Response), args.Error(1)
}

// Setup expectations
mockProvider := &MockLLMProvider{}
mockProvider.On("Complete", mock.Anything, mock.Anything).
    Return(&Response{Content: "test"}, nil)
```

---

## Slide 9: Integration Testing

**Integration Test Characteristics:**

- Test multiple components together
- May use real external services
- Longer execution time
- Verify component interactions

```bash
make test-integration
# Runs: go test -v ./tests/integration/...
```

---

## Slide 10: Test Infrastructure

**Docker-Based Test Environment:**

```bash
# Start test infrastructure
make test-infra-start
# Starts: PostgreSQL, Redis, Mock LLM containers

# Run tests with infrastructure
make test-with-infra

# Stop and cleanup
make test-infra-stop
make test-infra-clean
```

---

## Slide 11: Integration Test Example

**Testing Debate Service:**

```go
func TestDebateService_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup
    cfg := testutil.LoadTestConfig(t)
    db := testutil.SetupTestDB(t)
    cache := testutil.SetupTestCache(t)

    service := services.NewDebateService(cfg, db, cache)

    // Test
    result, err := service.ConductDebate(
        ctx,
        "Test topic",
        "Test context",
    )

    // Verify
    require.NoError(t, err)
    assert.NotNil(t, result.Consensus)
    assert.True(t, result.Duration > 0)
}
```

---

## Slide 12: E2E Testing

**End-to-End Test Characteristics:**

- Test complete user workflows
- Use real API endpoints
- Verify entire system
- Longest execution time

```bash
make test-e2e
# Runs: go test -v ./tests/e2e/...
```

---

## Slide 13: E2E Test Example

**Testing Complete API Flow:**

```go
func TestE2E_CompletionFlow(t *testing.T) {
    // Start server
    app := testutil.StartTestServer(t)
    defer app.Stop()

    // Make real API call
    resp, err := http.Post(
        "http://localhost:7061/v1/completion",
        "application/json",
        bytes.NewReader([]byte(`{
            "prompt": "Hello, world!",
            "providers": ["claude"]
        }`)),
    )

    require.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)

    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    assert.NotEmpty(t, result["content"])
}
```

---

## Slide 14: Security Testing

**Security Test Focus:**

- Authentication bypass
- Authorization flaws
- Input validation
- SQL injection
- XSS vulnerabilities

```bash
make test-security
# Runs: go test -v ./tests/security/...
```

---

## Slide 15: Stress Testing

**Load Testing:**

```bash
make test-stress
# Runs: go test -v ./tests/stress/...

# Or with custom parameters
go test -v -run TestStress \
  -concurrent=100 \
  -duration=5m \
  ./tests/stress/...
```

---

## Slide 16: Chaos Testing

**Resilience Testing:**

```bash
make test-chaos
# Runs: go test -v ./tests/challenge/...

# Tests:
# - Provider failures
# - Network partitions
# - Resource exhaustion
# - Configuration errors
```

---

## Slide 17: Test Coverage

**Measuring Coverage:**

```bash
make test-coverage

# Output:
# coverage: 67.5% of statements
# HTML report: coverage.html

# View report
open coverage.html
```

---

## Slide 18: Coverage Targets

**Package Coverage Goals:**

| Package | Current | Target |
|---------|---------|--------|
| internal/testing | 91.9% | 90% |
| internal/plugins | 71.4% | 70% |
| internal/services | 67.5% | 70% |
| internal/handlers | 55.9% | 60% |
| internal/cache | 42.4% | 50% |

---

## Slide 19: Code Quality

**Quality Commands:**

```bash
# Format code
make fmt

# Static analysis
make vet

# Run linter
make lint

# Security scan
make security-scan

# All quality checks
make fmt && make vet && make lint
```

---

## Slide 20: CI/CD Pipeline Overview

**Pipeline Stages:**

```
+--------+   +--------+   +---------+
|  Lint  |-->| Build  |-->|  Test   |
+--------+   +--------+   +---------+
                              |
+--------+   +--------+   +---v-----+
| Deploy |<--| Push   |<--| Security|
+--------+   +--------+   +---------+
```

---

## Slide 21: GitHub Actions Workflow

**Basic CI Configuration:**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: make lint

  test:
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: make test-coverage
```

---

## Slide 22: Docker Build in CI

**Building and Pushing Images:**

```yaml
  build:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v4

      - name: Build Docker image
        run: make docker-build

      - name: Push to registry
        run: |
          docker login -u ${{ secrets.DOCKER_USER }} \
            -p ${{ secrets.DOCKER_PASS }}
          docker push helixagent/helixagent:latest
```

---

## Slide 23: Quality Gates

**Enforcing Quality:**

```yaml
  quality-gate:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - name: Check coverage
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | \
            grep total | awk '{print $3}')
          if [ ${COVERAGE%\%} -lt 60 ]; then
            echo "Coverage below 60%"
            exit 1
          fi

      - name: Check lint
        run: |
          ISSUES=$(make lint 2>&1 | grep -c "^")
          if [ $ISSUES -gt 0 ]; then
            echo "Lint issues found"
            exit 1
          fi
```

---

## Slide 24: Deployment Automation

**Automated Deployment:**

```yaml
  deploy:
    runs-on: ubuntu-latest
    needs: [build, quality-gate]
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Deploy to staging
        run: |
          kubectl set image deployment/helixagent \
            helixagent=helixagent/helixagent:${{ github.sha }}

      - name: Run smoke tests
        run: |
          ./scripts/smoke-test.sh

      - name: Deploy to production
        if: success()
        run: |
          kubectl --context=production set image \
            deployment/helixagent \
            helixagent=helixagent/helixagent:${{ github.sha }}
```

---

## Slide 25: Test Best Practices

**Testing Guidelines:**

| Practice | Description |
|----------|-------------|
| AAA Pattern | Arrange, Act, Assert |
| One Assert | Focus each test |
| Descriptive Names | TestSubject_Condition_Expected |
| Test Edge Cases | Boundaries, empty, nil |
| Mock External | Isolate unit under test |
| Parallel Tests | Use t.Parallel() |

---

## Slide 26: Writing Effective Tests

**Test Quality Checklist:**

- [ ] Tests are independent
- [ ] Tests are repeatable
- [ ] Tests are fast
- [ ] Tests are clear
- [ ] Edge cases covered
- [ ] Error paths tested
- [ ] Mocks verified

---

## Slide 27: Hands-On Lab

**Lab Exercise 11.1: Testing and CI/CD**

Tasks:
1. Run all test suites
2. Analyze coverage reports
3. Write a custom integration test
4. Review CI/CD pipeline configuration
5. Set up a quality gate

Time: 35 minutes

---

## Slide 28: Module Summary

**Key Takeaways:**

- Test pyramid: Unit > Integration > E2E
- Multiple test types available
- Docker infrastructure for integration tests
- Coverage targets per package
- CI/CD with quality gates
- Automated deployment pipelines

**Congratulations! Course Complete!**

---

## Speaker Notes

### Slide 3 Notes
Explain the test pyramid concept. Most tests should be unit tests because they're fast and cheap. E2E tests are expensive but provide the most confidence.

### Slide 10 Notes
Demonstrate starting the test infrastructure. Show how Docker containers simulate external dependencies.

### Slide 21 Notes
Walk through each step of the GitHub Actions workflow. Explain how stages depend on each other.

### Slide 28 Notes
Celebrate course completion! Remind participants about certification options and next steps for continued learning.
