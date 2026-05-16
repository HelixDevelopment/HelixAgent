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

## Slide 21: Manual CI via Makefile

**All CI/CD runs manually via Makefile (NO automated pipelines per constitution):**

```bash
# Five-phase container-based CI system
make ci-all              # All five phases + report aggregation
make ci-go               # Phase 1: Go builds + all tests
make ci-mobile           # Phase 2: Flutter/RN + Android
make ci-web              # Phase 3: Angular + Playwright
make ci-desktop          # Phase 4: Electron/Tauri
make ci-integration      # Phase 5: Full-stack integration

# Pre-commit/pre-push validation (manual, no git hooks)
make ci-pre-commit       # fmt, vet, fallback lint
make ci-pre-push         # includes unit tests
make ci-validate-all     # All validation checks

# Resource control
CI_RESOURCE_LIMIT=low make ci-all   # 30% host resources (default)
CI_RESOURCE_LIMIT=medium make ci-all # 50% host resources
```

---

## Slide 22: Container-Based Release Builds

**All release builds run inside containers for reproducibility:**

```bash
# Build release binaries for all 7 apps, all 5 platforms
make release-all

# Build specific app
make release              # helixagent only
make release-api          # API server only

# Build Docker/Podman images
make docker-build         # Build Docker image
make container-build      # Auto-detect runtime

# Release info and change detection
make release-info         # Show version codes and source hashes
make release-force        # Force rebuild (ignore change detection)
make release-clean        # Clean release artifacts

# 7 Apps x 5 Platforms = 35 binaries
# Apps: helixagent, api, grpc-server, cognee-mock,
#       sanity-check, mcp-bridge, generate-constitution
# Platforms: linux/amd64, linux/arm64, darwin/amd64,
#            darwin/arm64, windows/amd64
```

---

## Slide 23: Quality Gates

**Enforcing Quality via Makefile:**

```bash
# All-in-one validation (recommended before any release)
make ci-validate-all

# Individual quality checks
make fmt                  # Format code
make vet                  # Static analysis
make lint                 # golangci-lint
make security-scan        # gosec

# Coverage gate
make test-coverage
# Thresholds defined in ci/thresholds.json

# Challenge validation (real-world use case tests)
./challenges/scripts/run_all_challenges.sh

# 6-layer false positive prevention:
# 1. Exit codes  2. Test counts  3. Coverage gates
# 4. Artifact integrity  5. Integration liveness
# 6. Report cross-validation
```

---

## Slide 24: Deployment via Container Orchestration

**HelixAgent automatic container deployment:**

```bash
# Build the binary
make build

# Run HelixAgent - ALL container orchestration is automatic
./bin/helixagent
# Step 1: Reads containers/.env for configuration
# Step 2: Auto-detects Docker/Podman runtime
# Step 3: Starts all required containers (local or remote)
# Step 4: Health checks all services
# Step 5: Fails boot if required services are unhealthy

# Remote distribution (configured in containers/.env)
# CONTAINERS_REMOTE_ENABLED=true
# CONTAINERS_REMOTE_HOST_1=user@remote-host
# All containers deployed to remote hosts via SSH

# After code changes: rebuild and redeploy
make docker-build         # Rebuild affected images
./bin/helixagent          # Automatic restart with new images
```

**IMPORTANT: Never start containers manually. HelixAgent handles all orchestration.**

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
Walk through the five-phase CI system and Makefile targets. Explain how phases depend on each other and how resource limits protect the host system.

### Slide 28 Notes
Celebrate course completion! Remind participants about certification options and next steps for continued learning.
