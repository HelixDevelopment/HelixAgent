# Video Course 84: Monitoring, Dashboards & Alerting

## Course Overview

**Duration:** 2.5 hours
**Level:** Intermediate to Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 09 (Production Operations), Course 75 (Performance Tuning)

Build a complete observability stack for HelixAgent: instrument services with Prometheus
metrics, create Grafana dashboards that surface the signals that matter, write
AlertManager rules that page on real problems, and use pprof for live memory and CPU
analysis. By the end of this course you will have a production-ready monitoring setup
connected to HelixAgent's built-in metrics endpoints.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Query and interpret all built-in Prometheus metrics exposed by HelixAgent
2. Build Grafana dashboards covering LLM provider health, debate quality, and system resources
3. Write AlertManager rules with correct severity, routing, and inhibition
4. Create custom Prometheus metrics in Go using the `prometheus/client_golang` library
5. Use the pprof HTTP endpoint for live profiling without restarting the service
6. Validate the monitoring stack using the `monitoring_dashboard_challenge.sh`

---

## Module 1: Prometheus Metrics (30 min)

### Video 1.1: Built-In HelixAgent Metrics (15 min)

**Topics:**
- The `/metrics` endpoint: exposed by HelixAgent at `http://localhost:7061/metrics`
- Metric families and their naming conventions: `helix_*` prefix
- Counter metrics: monotonically increasing totals (requests, errors, retries)
- Gauge metrics: current values (active goroutines, cache size, circuit breaker state)
- Histogram metrics: latency distributions with configurable buckets
- Summary metrics: provider scoring percentiles
- Key file: `internal/observability/` (OpenTelemetry + Prometheus integration)

**Key Metrics Reference:**
```
# LLM Provider Metrics
helix_provider_requests_total{provider,model,status}       Counter
helix_provider_latency_seconds{provider,model}             Histogram
helix_provider_errors_total{provider,model,error_type}     Counter
helix_provider_score{provider}                             Gauge
helix_circuit_breaker_state{provider}                      Gauge (0=closed,1=open,2=half-open)

# Ensemble Metrics
helix_ensemble_requests_total{strategy,status}             Counter
helix_ensemble_latency_seconds{strategy}                   Histogram
helix_debate_rounds_total{topology}                        Counter
helix_debate_consensus_latency_seconds{topology}           Histogram

# System Metrics
helix_active_goroutines                                    Gauge
helix_background_tasks_active                              Gauge
helix_cache_hits_total{cache_type}                         Counter
helix_cache_misses_total{cache_type}                       Counter
helix_http_request_duration_seconds{handler,method,code}  Histogram
```

### Video 1.2: Querying Metrics with PromQL (15 min)

**Topics:**
- Basic PromQL: selectors, labels, time ranges
- `rate()`: requests per second over a rolling window
- `histogram_quantile()`: p95 and p99 latency from histogram metrics
- `increase()`: total increase over a time window (for counters)
- Label filtering: restricting to a single provider or strategy
- Useful PromQL patterns for HelixAgent

**Essential PromQL Queries:**
```promql
# Request rate per provider (last 5m)
rate(helix_provider_requests_total[5m])

# p95 provider latency
histogram_quantile(0.95,
  rate(helix_provider_latency_seconds_bucket[5m])
)

# Error rate per provider
rate(helix_provider_errors_total[5m])
  / rate(helix_provider_requests_total[5m])

# Circuit breakers currently open
helix_circuit_breaker_state == 1

# Cache hit ratio
rate(helix_cache_hits_total[5m])
  / (rate(helix_cache_hits_total[5m]) + rate(helix_cache_misses_total[5m]))

# Active goroutines trend
helix_active_goroutines
```

---

## Module 2: Grafana Dashboards (40 min)

### Video 2.1: Setting Up Grafana (10 min)

**Topics:**
- Starting the monitoring stack: `make infra-start` starts Prometheus and Grafana
- Default ports: Prometheus at `http://localhost:9090`, Grafana at `http://localhost:3000`
- Adding Prometheus as a Grafana data source
- Importing HelixAgent's pre-built dashboard JSON from `docker/monitoring/grafana/dashboards/`
- Dashboard organisation: one dashboard per domain (Providers, Debate, System, LLMOps)

**Add Prometheus Data Source:**
```bash
curl -X POST http://admin:admin@localhost:3000/api/datasources \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Prometheus",
    "type": "prometheus",
    "url": "http://prometheus:9090",
    "access": "proxy",
    "isDefault": true
  }'
```

### Video 2.2: Building the Provider Health Dashboard (15 min)

**Topics:**
- Stat panels: current provider score, circuit breaker state
- Time-series panels: request rate, error rate, p95 latency per provider
- Bar gauge: provider score ranking (leaderboard)
- Table panel: provider details with last-check timestamp
- Template variables: `$provider` dropdown to filter all panels simultaneously

**Provider Score Panel (JSON excerpt):**
```json
{
  "type": "gauge",
  "title": "Provider Score",
  "targets": [
    {
      "expr": "helix_provider_score",
      "legendFormat": "{{provider}}"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "min": 0, "max": 10,
      "thresholds": {
        "steps": [
          {"value": 0,   "color": "red"},
          {"value": 5,   "color": "yellow"},
          {"value": 7.5, "color": "green"}
        ]
      }
    }
  }
}
```

### Video 2.3: Building the Debate and System Dashboards (15 min)

**Topics:**
- Debate dashboard: consensus latency histogram, rounds per topology, phase durations
- System dashboard: goroutine count trend, HTTP request duration heatmap,
  cache hit/miss ratio, background task queue depth
- LLMOps dashboard: experiment traffic split, metric delta per variant, promotion events
- Row collapsing: group related panels to keep dashboards scannable
- Annotations: marking deployments and benchmark runs on time-series charts

**Debate Consensus Latency Panel:**
```json
{
  "type": "timeseries",
  "title": "Debate Consensus Latency p95 (by topology)",
  "targets": [
    {
      "expr": "histogram_quantile(0.95, rate(helix_debate_consensus_latency_seconds_bucket[5m]))",
      "legendFormat": "{{topology}}"
    }
  ]
}
```

---

## Module 3: AlertManager Rules (30 min)

### Video 3.1: Writing Alert Rules (15 min)

**Topics:**
- Prometheus alert rule structure: `groups`, `rules`, `alert`, `expr`, `for`, `labels`, `annotations`
- Severity labels: `critical` (page immediately), `warning` (ticket), `info` (dashboard only)
- `for` duration: avoid flapping by requiring condition to hold for N minutes
- Annotation templates: `{{ $labels.provider }}`, `{{ $value | humanizeDuration }}`
- Alert inhibition: suppress `warning` when `critical` is already firing for the same target
- Key file: `docker/monitoring/prometheus/alerts.yml`

**Alert Rules:**
```yaml
groups:
  - name: helixagent.providers
    rules:
      - alert: ProviderCircuitBreakerOpen
        expr: helix_circuit_breaker_state == 1
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Circuit breaker open for {{ $labels.provider }}"
          description: >
            Provider {{ $labels.provider }} has had its circuit breaker
            open for more than 1 minute. Ensemble quality is degraded.

      - alert: ProviderHighErrorRate
        expr: >
          rate(helix_provider_errors_total[5m])
            / rate(helix_provider_requests_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate for {{ $labels.provider }}"
          description: >
            Provider {{ $labels.provider }} error rate is
            {{ $value | humanizePercentage }} over the last 5 minutes.

      - alert: ProviderScoreBelowThreshold
        expr: helix_provider_score < 5.0
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Provider {{ $labels.provider }} score below minimum"
          description: >
            Provider score {{ $value | printf "%.2f" }} is below the
            minimum threshold of 5.0 for ensemble participation.
```

### Video 3.2: AlertManager Routing and Inhibition (15 min)

**Topics:**
- AlertManager `route` tree: match labels to route to the correct receiver
- Receivers: PagerDuty for `critical`, Slack for `warning`, email for `info`
- Grouping: batch related alerts into one notification to reduce noise
- Inhibition rules: if `critical` is firing, suppress `warning` for the same provider
- Silences: temporary suppression during planned maintenance windows

**AlertManager Config:**
```yaml
global:
  resolve_timeout: 5m

route:
  group_by: ['alertname', 'provider']
  group_wait:      30s
  group_interval:  5m
  repeat_interval: 4h
  receiver: slack-warnings
  routes:
    - match:
        severity: critical
      receiver: pagerduty-critical
      continue: false

inhibit_rules:
  - source_match:
      severity: critical
    target_match:
      severity: warning
    equal: ['provider']

receivers:
  - name: pagerduty-critical
    pagerduty_configs:
      - routing_key: "${PAGERDUTY_KEY}"
        description: '{{ template "pagerduty.description" . }}'

  - name: slack-warnings
    slack_configs:
      - api_url: "${SLACK_WEBHOOK_URL}"
        channel: '#helixagent-alerts'
        text: '{{ range .Alerts }}{{ .Annotations.summary }}\n{{ end }}'
```

---

## Module 4: Custom Prometheus Metrics (20 min)

### Video 4.1: Adding Custom Metrics in Go (10 min)

**Topics:**
- Registering a custom counter: `prometheus.NewCounterVec`
- Registering a custom histogram: `prometheus.NewHistogramVec` with bucket configuration
- Registering a custom gauge: `prometheus.NewGaugeVec`
- Best practice: register metrics at package init time, not per request
- Naming conventions: `helix_<subsystem>_<metric>_<unit>` (e.g. `helix_rag_chunks_indexed_total`)

**Custom Metric Registration:**
```go
var (
    ragChunksIndexed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "helix_rag_chunks_indexed_total",
            Help: "Total number of document chunks indexed into the vector store.",
        },
        []string{"collection"},
    )

    ragSearchLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "helix_rag_search_latency_seconds",
            Help:    "RAG hybrid search latency in seconds.",
            Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
        },
        []string{"collection"},
    )
)

func init() {
    prometheus.MustRegister(ragChunksIndexed, ragSearchLatency)
}
```

### Video 4.2: Instrumenting Hot Paths (10 min)

**Topics:**
- Recording observations: `counter.WithLabelValues("my_collection").Inc()`
- Timing with `prometheus.NewTimer`: automatically records duration on `ObserveDuration()`
- Gauge set/increment/decrement for queue depths and active connection counts
- Avoiding cardinality explosion: never use user-provided strings as label values directly

**Timer Pattern:**
```go
func (s *RAGService) Search(ctx context.Context, query string, collection string) ([]Chunk, error) {
    timer := prometheus.NewTimer(
        ragSearchLatency.WithLabelValues(collection),
    )
    defer timer.ObserveDuration()

    return s.vectorDB.Search(ctx, query, collection)
}
```

---

## Module 5: Live pprof Profiling (20 min)

### Video 5.1: The pprof HTTP Endpoint (10 min)

**Topics:**
- Available pprof profiles: `heap`, `goroutine`, `threadcreate`, `block`, `mutex`, `profile`
- Enabling pprof in development: `GIN_MODE=debug` registers `net/http/pprof` routes
- Security: pprof is never exposed in `GIN_MODE=release`
- Capturing profiles without restarting: all profiles available via HTTP GET
- The `monitoring_dashboard_challenge.sh` validates Prometheus metrics and pprof availability

**All Available Profiles:**
```bash
# Live goroutine count and stacks
curl http://localhost:7061/debug/pprof/goroutine?debug=1

# Heap allocation snapshot
curl http://localhost:7061/debug/pprof/heap > heap.prof

# 30-second CPU profile
curl "http://localhost:7061/debug/pprof/profile?seconds=30" > cpu.prof

# Mutex contention profile (requires runtime.SetMutexProfileFraction > 0)
curl http://localhost:7061/debug/pprof/mutex > mutex.prof

# Block profile (goroutines blocked on channels/mutexes)
curl http://localhost:7061/debug/pprof/block > block.prof
```

### Video 5.2: Goroutine and Mutex Profiles (10 min)

**Topics:**
- Goroutine profile: counting goroutines, identifying leaked or stuck goroutines
- `debug=1` output: text summary with goroutine count by state
- `debug=2` output: full stacks for every goroutine (use sparingly under load)
- Mutex profile: which mutexes are most contended; reducing lock granularity
- Block profile: which channel operations are blocking goroutines the longest
- Connecting pprof findings to the WaitGroup patterns from Course 81

**Goroutine Count Check:**
```bash
# Quick goroutine count
curl -s http://localhost:7061/debug/pprof/goroutine?debug=1 | head -5
# Output: goroutine profile: total 47

# Alert if goroutine count grows unexpectedly
# Prometheus: helix_active_goroutines > 500 for 5m
```

---

## Code Demo: Full Monitoring Stack Setup

This demo builds the complete monitoring stack from scratch:

1. Start Prometheus and Grafana: `make infra-start`
2. Verify HelixAgent metrics at `http://localhost:7061/metrics`
3. Import provider health dashboard JSON into Grafana
4. Write and load the circuit breaker alert rule
5. Trigger a provider failure and watch the alert fire in AlertManager
6. Capture a heap profile and identify an allocation hotspot

**Validation:**
```bash
# Run the monitoring dashboard challenge (validates Prometheus + Grafana + pprof)
./challenges/scripts/monitoring_dashboard_challenge.sh
```

---

## Key Takeaways

- HelixAgent exposes a rich set of built-in Prometheus metrics under the `helix_*`
  namespace; learn their names and they become your primary debugging tool in production.
- Grafana dashboards should be organised by domain: one for provider health, one for
  debate quality, one for system resources — avoid mixing concerns in a single dashboard.
- AlertManager routing and inhibition rules prevent alert fatigue; only page on
  `critical` severity and group related alerts to reduce noise.
- Custom metrics should be registered at `init()` time and use low-cardinality label
  values to prevent the cardinality explosion that degrades Prometheus performance.
- The pprof HTTP endpoint gives live memory and CPU profiles without restarting the
  service — invaluable for diagnosing production issues.

---

## Related Courses

- **Course 09: Production Operations** — Operations runbook that this monitoring setup supports
- **Course 75: Performance Tuning** — Profiling methodology that complements the pprof section
- **Course 82: Performance Tuning and Baselines** — Benchmark baselines surfaced in dashboards
- **Course 83: Security Scanning and Vulnerability Management** — Security metrics in Grafana
