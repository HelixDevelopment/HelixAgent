# Video Course 82: Performance Tuning & Baselines

## Course Overview

**Duration:** 2.5 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 75 (Performance Tuning), Course 81 (Safety & Concurrency Patterns)

Learn to capture performance baselines, detect regressions automatically, and drive
targeted optimisations using data from Go benchmarks, pprof profiles, and Prometheus
metrics. This course covers the complete baseline framework built into HelixAgent, how
to read and act on pprof CPU and memory output, and how to connect profiling findings
to code-level improvements.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Write reliable Go benchmark functions and avoid common pitfalls
2. Capture and store baseline benchmark results for regression detection
3. Use `go tool pprof` to identify CPU hotspots and memory allocation paths
4. Interpret heap profiles to find memory leaks and excessive allocations
5. Apply targeted optimisations informed by profiling data
6. Integrate benchmark regression detection into the CI validation workflow

---

## Module 1: Benchmark Methodology (30 min)

### Video 1.1: Writing Correct Go Benchmarks (15 min)

**Topics:**
- Benchmark function signature: `func BenchmarkFoo(b *testing.B)`
- The `b.N` loop: why you must loop exactly `b.N` times and nothing else
- `b.ResetTimer()`: exclude expensive setup from measurement
- `b.ReportAllocs()`: count heap allocations per operation
- `b.SetBytes(n)`: report throughput in bytes/second
- Sub-benchmarks with `b.Run`: compare variants in one benchmark file
- Avoiding compiler optimisation: use `testing.B` sink via `_ = result`
- Resource limit rule: always run benchmarks with `GOMAXPROCS=2` and `nice -n 19`

**Correct Benchmark:**
```go
func BenchmarkEnsembleComplete(b *testing.B) {
    // Setup (not measured)
    svc := newTestEnsembleService(b)
    req := &CompletionRequest{Model: "helixagent-debate", Messages: testMessages}
    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        resp, err := svc.Complete(context.Background(), req)
        if err != nil {
            b.Fatal(err)
        }
        _ = resp // prevent compiler elimination
    }
}
```

**Run with resource limits:**
```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -bench=BenchmarkEnsembleComplete -benchmem -benchtime=10s \
  -p 1 ./internal/llm/...
```

### Video 1.2: Sub-Benchmarks and Table-Driven Benchmarks (15 min)

**Topics:**
- Table-driven benchmarks: iterate over a slice of test cases with `b.Run`
- Comparing providers: one sub-benchmark per provider
- Comparing strategies: one sub-benchmark per ensemble strategy
- `benchstat`: the standard tool for comparing two benchmark result files
- Interpreting `benchstat` output: delta, p-value, confidence

**Table-Driven Benchmark:**
```go
func BenchmarkProviders(b *testing.B) {
    providers := []string{"claude", "gemini", "deepseek"}
    for _, p := range providers {
        p := p
        b.Run(p, func(b *testing.B) {
            client := newProviderClient(b, p)
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                _, _ = client.Complete(context.Background(), testReq)
            }
        })
    }
}
```

**Compare with benchstat:**
```bash
go test -bench=BenchmarkProviders -count=5 ./... > baseline.txt
# make changes
go test -bench=BenchmarkProviders -count=5 ./... > after.txt
benchstat baseline.txt after.txt
```

---

## Module 2: Baseline Framework (30 min)

### Video 2.1: Capturing and Storing Baselines (15 min)

**Topics:**
- What is a performance baseline? A known-good benchmark result to compare against
- HelixAgent's baseline framework: stores results in `benchmarks/baselines/` as JSON
- Baseline metadata: Git commit hash, Go version, GOMAXPROCS, timestamp
- Baseline capture script: `make test-bench` outputs to `benchmarks/baselines/<date>.json`
- Querying baselines: `GET /v1/benchmark/results` shows historical runs

**Baseline JSON Structure:**
```json
{
  "commit": "32869789",
  "go_version": "go1.25.3",
  "gomaxprocs": 2,
  "timestamp": "2026-04-05T10:00:00Z",
  "benchmarks": [
    {
      "name": "BenchmarkEnsembleComplete",
      "ns_per_op": 1842000,
      "allocs_per_op": 234,
      "bytes_per_op": 48920
    }
  ]
}
```

### Video 2.2: Regression Detection (15 min)

**Topics:**
- Regression threshold: flag if `ns_per_op` increases more than 10% vs. baseline
- The `coverage_gate_challenge.sh` pattern applied to performance gates
- Automated regression check: `make test-bench` compares against last committed baseline
- What to do when a regression is detected: profile first, optimise second
- False positives: system load variance; always run benchmarks on a quiet machine

**Regression Check Script:**
```bash
#!/usr/bin/env bash
# compare current benchmark against stored baseline
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -bench=. -count=3 -p 1 ./... > /tmp/current.txt

benchstat benchmarks/baselines/latest.txt /tmp/current.txt | \
  awk '/\+[0-9]+\.[0-9]+%/ && $NF > 10 {
    print "REGRESSION:", $0; exit 1
  }'
```

---

## Module 3: pprof CPU Profiling (30 min)

### Video 3.1: Capturing a CPU Profile (15 min)

**Topics:**
- Two ways to capture: benchmark flag (`-cpuprofile`) and live HTTP endpoint
- Live profiling via `net/http/pprof`: `GET /debug/pprof/profile?seconds=30`
- Enabling the pprof endpoint in development mode: `GIN_MODE=debug`
- Security: pprof endpoint is disabled in production (`GIN_MODE=release`)
- Saving profiles: `curl ... > cpu.prof`

**Capture via HTTP:**
```bash
# Capture 30-second CPU profile from running HelixAgent
curl -s "http://localhost:7061/debug/pprof/profile?seconds=30" > cpu.prof

# Open interactive pprof UI
go tool pprof -http=:6060 cpu.prof
```

**Capture via Benchmark:**
```bash
GOMAXPROCS=2 nice -n 19 \
  go test -bench=BenchmarkEnsembleComplete -cpuprofile=cpu.prof \
  -p 1 ./internal/llm/...
go tool pprof cpu.prof
```

### Video 3.2: Reading pprof Output (15 min)

**Topics:**
- `top10`: shows the 10 functions consuming the most CPU time
- `flat` vs. `cum`: flat = time in function body; cum = time in function + callees
- `list FunctionName`: shows annotated source with per-line CPU cost
- `web`: renders a call graph in the browser (requires Graphviz)
- Common hotspots in HelixAgent: JSON marshaling, HTTP connection setup, regex compilation
- Interpreting flame graphs: width = time, depth = call stack depth

**pprof Interactive Session:**
```
(pprof) top10
Showing nodes accounting for 8.2s, 82% of 10s total
      flat  flat%   sum%        cum   cum%
     3.1s 31.00% 31.00%      3.1s 31.00%  encoding/json.Marshal
     1.4s 14.00% 45.00%      1.4s 14.00%  crypto/tls.(*Conn).Write
     0.9s  9.00% 54.00%      0.9s  9.00%  regexp.(*Regexp).Find

(pprof) list json.Marshal
Total: 10s
ROUTINE ======================== encoding/json.Marshal
     3.1s      3.1s (flat, cum) 31.00% of Total
...
```

---

## Module 4: Memory Profiling (30 min)

### Video 4.1: Heap Profiles (15 min)

**Topics:**
- Heap profile: snapshot of live allocations at a point in time
- Capturing heap profile: `GET /debug/pprof/heap`
- `alloc_objects` vs. `inuse_objects`: total allocated vs. currently live
- `alloc_space` vs. `inuse_space`: total bytes allocated vs. currently live
- Identifying memory leaks: `inuse_objects` grows without bound under steady load
- The `pprof_memory_profiling_challenge.sh` challenge script

**Capture Heap Profile:**
```bash
# Capture heap profile
curl -s "http://localhost:7061/debug/pprof/heap" > heap.prof

# Compare two profiles to see growth
go tool pprof -http=:6061 -diff_base=heap_before.prof heap_after.prof
```

### Video 4.2: Allocation Reduction Techniques (15 min)

**Topics:**
- Pre-allocating slices: `make([]T, 0, expectedLen)` avoids repeated growth copies
- Reusing buffers: `sync.Pool` for frequently allocated byte slices
- Avoiding interface boxing: use concrete types on hot paths
- String vs. `[]byte` conversions: minimise copies in HTTP response writing
- Escape analysis: `go build -gcflags='-m' ./...` shows what escapes to the heap

**sync.Pool for Buffer Reuse:**
```go
var bufPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func encodeResponse(resp *CompletionResponse) ([]byte, error) {
    buf := bufPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufPool.Put(buf)
    }()
    if err := json.NewEncoder(buf).Encode(resp); err != nil {
        return nil, err
    }
    out := make([]byte, buf.Len())
    copy(out, buf.Bytes())
    return out, nil
}
```

---

## Module 5: Connecting Profiling to Optimisation (20 min)

### Video 5.1: Optimisation Workflow (10 min)

**Topics:**
- The correct workflow: measure → profile → identify hotspot → optimise → measure again
- Never optimise without data: profiling prevents wasted effort on non-bottlenecks
- Setting an optimisation target: reduce p95 latency by 20%, reduce allocs by 30%
- Tracking optimisation impact: benchmark before and after every change
- Documenting optimisations: commit message format, benchmark delta in PR description

**Optimisation Workflow:**
```
1. Run benchmark → establish baseline
2. Capture CPU + heap profile under load
3. Identify top-3 hotspots in pprof
4. Implement fix for #1 hotspot
5. Run benchmark again → measure delta
6. If target met: commit; if not: profile again
7. Repeat for remaining hotspots
```

### Video 5.2: Performance Regression Prevention (10 min)

**Topics:**
- Committing baselines to the repository: `benchmarks/baselines/` in version control
- Pre-push benchmark gate: `make ci-pre-push` runs benchmarks and checks regression
- Alerting on production latency increase: Prometheus alert rule on p95 endpoint latency
- The `resource_limits_challenge.sh` ensures tests never consume more than 40% of host

---

## Key Takeaways

- Always capture a baseline before optimising; without a baseline you cannot measure
  whether your change actually improved anything.
- CPU profiles reveal algorithmic hotspots (JSON marshaling, regex, TLS); heap profiles
  reveal memory leaks and excessive allocation patterns.
- `sync.Pool` and pre-allocated slices are the two most impactful allocation reduction
  techniques in high-throughput Go services.
- The optimisation loop — measure, profile, fix, measure — must be run with consistent
  resource constraints (`GOMAXPROCS=2`, `nice -n 19`) to produce reproducible results.
- Regression detection in CI prevents performance regressions from reaching production.

---

## Related Courses

- **Course 75: Performance Tuning** — Lazy loading, semaphores, and HTTP/3 tuning
- **Course 81: Safety and Concurrency Patterns** — Concurrency fixes that affect performance
- **Course 83: Security Scanning and Vulnerability Management** — Scanning tools alongside profiling
- **Course 84: Monitoring, Dashboards and Alerting** — Prometheus metrics for ongoing performance tracking
