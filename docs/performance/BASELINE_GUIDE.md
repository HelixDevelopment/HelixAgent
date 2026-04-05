# Performance Baseline Management Guide

**Date:** 2026-03-30
**Status:** Active

## Overview

HelixAgent tracks performance baselines to detect regressions before they reach production. Baselines are golden files containing benchmark results stored in `benchmarks/baselines/`. A 15% regression threshold triggers a failure.

---

## How It Works

1. **Capture:** Run benchmarks and save results as the new baseline
2. **Check:** Run benchmarks again and compare against the stored baseline
3. **Gate:** If any benchmark regresses by more than 15%, the check fails

The system uses Go's standard `testing.B` benchmarks with `benchstat` for statistical comparison.

---

## Capturing a Baseline

```bash
make benchmark-baseline
```

This runs:
- `scripts/benchmark-baseline.sh`
- Executes benchmarks with `GOMAXPROCS=2 nice -n 19 -p 1`
- Saves results to `benchmarks/baselines/` with timestamped filenames
- Uses `-count=3` for statistical significance

**Manual capture for a specific package:**

```bash
GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=3 \
  -run='^$' ./internal/handlers/ -p 1 -timeout=300s \
  > benchmarks/baselines/handlers.txt
```

**When to capture a new baseline:**
- After intentional performance improvements
- After major refactoring that changes critical paths
- After upgrading Go versions or key dependencies
- At the start of each release cycle

---

## Checking Against Baseline

```bash
make benchmark-check
```

This runs current benchmarks and compares them against the stored baselines using `benchstat`. The comparison covers:

- **ns/op** -- nanoseconds per operation (latency)
- **B/op** -- bytes allocated per operation (memory)
- **allocs/op** -- heap allocations per operation (GC pressure)

**Manual check for a specific package:**

```bash
GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=3 \
  -run='^$' ./internal/handlers/ -p 1 -timeout=300s \
  > /tmp/bench-current-handlers.txt

benchstat benchmarks/baselines/handlers.txt /tmp/bench-current-handlers.txt
```

---

## Golden Files

**Location:** `benchmarks/baselines/`

**Format:** Standard Go benchmark output (`go test -bench` text format), compatible with `benchstat`.

**Example content:**
```
goos: linux
goarch: amd64
pkg: dev.helix.agent/internal/handlers
BenchmarkCompletionHandler-2      10000    115234 ns/op    8192 B/op    64 allocs/op
BenchmarkEnsembleHandler-2         5000    234567 ns/op   16384 B/op   128 allocs/op
BenchmarkHealthCheck-2           100000     12345 ns/op    1024 B/op     8 allocs/op
PASS
```

---

## Regression Threshold

The default regression threshold is **15%**. This means:

- A benchmark that was 100 ns/op at baseline can regress to at most 115 ns/op
- A benchmark that was 8192 B/op at baseline can regress to at most 9420 B/op
- Regressions beyond 15% fail the `benchmark-check` target

**Why 15%:** This balances sensitivity against noise. Benchmark results naturally vary by 5-10% between runs due to OS scheduling, thermal throttling, and background processes. A 15% threshold catches real regressions while avoiding false alarms.

---

## Benchmark Test Locations

Performance benchmarks are located in `tests/performance/`:

| File | Coverage |
|------|----------|
| `benchmark_test.go` | Core handler benchmarks |
| `comprehensive_benchmark_test.go` | Full request pipeline benchmarks |
| `core_benchmarks_test.go` | Low-level function benchmarks |
| `ensemble_benchmark_test.go` | Ensemble orchestration benchmarks |
| `debate_benchmark_test.go` | Debate system benchmarks |
| `formatters_benchmark_test.go` | Formatter execution benchmarks |
| `lazy_loading_benchmark_test.go` | Lazy initialization overhead |
| `lazy_loading_comprehensive_test.go` | Comprehensive lazy loading paths |
| `semaphore_benchmark_test.go` | Semaphore contention benchmarks |
| `skills_benchmark_test.go` | Skill registry lookup benchmarks |
| `agentic_ensemble_benchmark_test.go` | Agentic workflow benchmarks |
| `baseline_regression_test.go` | Automated regression detection |
| `monitoring_metrics_test.go` | Metrics collection overhead |
| `pprof_leak_detection_test.go` | pprof-based memory leak detection |

---

## Resource Limits for Benchmarks

Benchmarks MUST run with constrained resources to produce reproducible results:

```bash
GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=3 \
  -run='^$' -p 1 -timeout=300s ./tests/performance/
```

**Key flags:**
- `GOMAXPROCS=2` -- consistent parallelism across machines
- `-count=3` -- minimum 3 iterations for `benchstat` statistical analysis
- `-run='^$'` -- skip non-benchmark tests
- `-p 1` -- single package at a time to avoid interference

---

## CI Integration

The benchmark check can be integrated into the validation pipeline:

```bash
make ci-validate-all   # Includes benchmark-check
```

If `benchmarks/baselines/` is empty, `benchmark-check` will print a warning and skip comparison rather than failing.

---

## Cross-References

- Test strategy: `docs/testing/TEST_STRATEGY.md`
- Stress testing: `docs/testing/STRESS_TESTING_GUIDE.md`
- Performance architecture: `docs/performance/ARCHITECTURE.md`
- Performance troubleshooting: `docs/performance/TROUBLESHOOTING.md`
