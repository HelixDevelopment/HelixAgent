# Video Course 66: Load Testing Mastery

## Course Overview

**Duration:** 2 hours
**Level:** Intermediate to Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 06 (Testing), Course 57 (Stress Testing Guide)

Master load testing techniques for HelixAgent: sustained constant-rate testing, spike traffic simulation, soak testing for leak detection, and goroutine leak detection. Learn to use the built-in load testing framework and interpret results for production readiness.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Run sustained load tests with configurable concurrency and duration
2. Simulate traffic spikes and validate graceful degradation
3. Detect memory leaks and goroutine leaks via soak testing
4. Interpret load test metrics (latency, success rate, rejection rate)
5. Use the ConcurrencyLimiter middleware to provide hard backpressure
6. Apply resource limits to prevent test infrastructure overload

---

## Module 1: Load Testing Fundamentals (25 min)

### Video 1.1: Why Load Testing Matters (10 min)

**Topics:**
- Difference between unit tests, integration tests, stress tests, and load tests
- Sustained load vs spike vs soak testing
- Production readiness criteria: latency SLOs, error rate thresholds
- HelixAgent resource limit mandate: GOMAXPROCS=2, nice -n 19, ionice -c 3

### Video 1.2: The Load Testing Framework (15 min)

**Topics:**
- Location: `tests/load/load_test.go`
- `loadMetrics` struct: atomic counters for thread-safe metrics collection
- `newTestRouter()`: creating test routers with ConcurrencyLimiter
- Running load tests: `GOMAXPROCS=2 nice -n 19 go test -run TestLoad ./tests/load/ -count=1 -p 1`

**Code Example:**
```go
type loadMetrics struct {
    totalRequests   int64
    successCount    int64
    failureCount    int64
    rejectedCount   int64
    totalLatencyNs  int64
    maxLatencyNs    int64
}
```

---

## Module 2: Sustained Load Testing (25 min)

### Video 2.1: Constant-Rate Load Test Design (10 min)

**Topics:**
- Fixed concurrency with deadline-based loop
- Using `httptest.NewRecorder()` for in-process testing
- Measuring success rate, average latency, max latency
- Acceptance criteria: >95% success rate, <100ms avg latency

### Video 2.2: Running and Interpreting Results (15 min)

**Topics:**
- Running: `go test -v -run TestLoad_SustainedConstantRate ./tests/load/`
- Reading output: total requests, success/failure/rejected counts
- Goroutine peak tracking with atomic CompareAndSwap
- Tuning concurrency level vs handler latency

---

## Module 3: Spike Traffic Testing (25 min)

### Video 3.1: Two-Phase Spike Simulation (10 min)

**Topics:**
- Phase 1: Normal load baseline (5 concurrent, 2 seconds)
- Phase 2: 10x spike (50 concurrent, 2 seconds)
- Why ConcurrencyLimiter returns 503 instead of queuing
- Zero unexpected failures criterion

### Video 3.2: Analyzing Spike Results (15 min)

**Topics:**
- Expected: some 503 rejections, zero 5xx failures
- Success count during spike vs normal baseline
- Recovery after spike: does normal throughput resume?

---

## Module 4: Soak Testing for Leak Detection (25 min)

### Video 4.1: Memory and Goroutine Leak Detection (15 min)

**Topics:**
- `runtime.ReadMemStats()` before and after test
- `runtime.NumGoroutine()` baseline vs final
- `runtime.GC()` to force garbage collection before measurement
- Acceptance criteria: goroutine delta <= 5, memory growth reasonable

### Video 4.2: Dedicated Goroutine Leak Test (10 min)

**Topics:**
- Warmup phase to stabilize goroutine count
- Batch request processing (5 batches x 50 requests)
- Settlement period before final measurement
- `TestLoad_GoroutineLeakDetection` walkthrough

---

## Module 5: Integration with CI Validation (20 min)

### Video 5.1: Running Load Tests in CI (10 min)

**Topics:**
- `make test-stress` includes load tests
- Resource limits: `CI_RESOURCE_LIMIT=low` for CI environments
- Short-mode skipping: `testing.Short()` skips load tests
- Full run: requires explicit `-short=false` flag

### Video 5.2: Adding Custom Load Tests (10 min)

**Topics:**
- Adding new endpoints to test
- Creating custom metrics collectors
- Setting acceptance thresholds
- Integrating with Prometheus metrics comparison

---

## Exercises

1. Run the spike traffic test and verify the concurrency limiter works
2. Modify `TestLoad_SustainedConstantRate` to use 20 concurrent goroutines
3. Create a new load test for the `/v1/health` endpoint
4. Add a custom metric tracking P99 latency

---

## Summary

Load testing validates production readiness under realistic traffic patterns. The HelixAgent load testing framework provides four test types: sustained constant-rate, spike traffic, soak testing, and goroutine leak detection. All tests respect the project's resource limit mandate and integrate with the existing test infrastructure.
