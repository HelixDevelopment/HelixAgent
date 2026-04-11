# User Manual 18: Performance Monitoring

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Key Metrics](#key-metrics)
4. [Infrastructure Setup](#infrastructure-setup)
5. [Prometheus Configuration](#prometheus-configuration)
6. [Grafana Dashboards](#grafana-dashboards)
7. [Custom Metrics in HelixAgent](#custom-metrics-in-helixagent)
8. [Provider Latency Tracking](#provider-latency-tracking)
9. [Performance Baselines](#performance-baselines)
10. [Alerting](#alerting)
11. [Monitoring Architecture](#monitoring-architecture)
12. [CLI and API Access](#cli-and-api-access)
13. [Troubleshooting](#troubleshooting)
14. [Related Resources](#related-resources)

## Overview

This manual covers the full observability and performance monitoring stack for HelixAgent in production environments. HelixAgent exposes Prometheus-compatible metrics on every running instance and integrates with OpenTelemetry for distributed tracing. The monitoring system tracks request latency, LLM provider response times, ensemble debate performance, circuit breaker states, cache hit rates, and infrastructure resource usage.

All metrics are accessible via the `/v1/monitoring/status` endpoint and the Prometheus scrape target at `/metrics`. Grafana dashboards provide real-time visualization and historical analysis.

## Prerequisites

- HelixAgent running on port 7061 (default)
- Docker or Podman for running the monitoring stack
- Network access from Prometheus to the HelixAgent instance(s)
- Minimum 2 GB additional RAM for the monitoring stack
- Familiarity with PromQL for custom queries

## Key Metrics

HelixAgent exposes metrics across several categories:

| Metric Name | Type | Description |
|---|---|---|
| `helixagent_request_duration_seconds` | Histogram | HTTP request latency (p50, p95, p99) |
| `helixagent_requests_total` | Counter | Total requests by method, path, status |
| `helixagent_active_requests` | Gauge | Currently in-flight requests |
| `helixagent_provider_latency_seconds` | Histogram | Per-provider LLM response time |
| `helixagent_provider_errors_total` | Counter | Provider errors by type and provider |
| `helixagent_circuit_breaker_state` | Gauge | Circuit breaker state (0=closed, 1=half-open, 2=open) |
| `helixagent_cache_hits_total` | Counter | Cache hit count (Redis + in-memory) |
| `helixagent_cache_misses_total` | Counter | Cache miss count |
| `helixagent_debate_rounds_total` | Counter | Debate rounds executed |
| `helixagent_debate_duration_seconds` | Histogram | End-to-end debate duration |
| `helixagent_ensemble_votes_total` | Counter | Ensemble voting events |
| `helixagent_token_usage_total` | Counter | Token consumption by provider |
| `helixagent_goroutines` | Gauge | Active goroutine count |
| `helixagent_memory_alloc_bytes` | Gauge | Heap memory allocation |

Additional standard Go runtime metrics are exposed automatically via the Prometheus client library (GC pauses, memory stats, goroutine counts).

## Infrastructure Setup

### Docker Compose Monitoring Stack

```yaml
# docker-compose.monitoring.yml
version: '3.8'
services:
  prometheus:
    image: prom/prometheus:v2.51.0
    ports:
      - "9090:9090"
    volumes:
      - ./configs/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.retention.time=30d'
      - '--web.enable-lifecycle'
    restart: unless-stopped

  grafana:
    image: grafana/grafana:11.0.0
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./configs/grafana/provisioning:/etc/grafana/provisioning
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=helixagent
      - GF_USERS_ALLOW_SIGN_UP=false
    depends_on:
      - prometheus
    restart: unless-stopped

  alertmanager:
    image: prom/alertmanager:v0.27.0
    ports:
      - "9093:9093"
    volumes:
      - ./configs/alertmanager/alertmanager.yml:/etc/alertmanager/alertmanager.yml
    restart: unless-stopped

volumes:
  prometheus_data:
  grafana_data:
```

Start the stack:

```bash
docker-compose -f docker-compose.monitoring.yml up -d
```

### Verify Services

```bash
# Check Prometheus targets
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[].health'

# Check Grafana health
curl -s http://localhost:3000/api/health

# Check HelixAgent metrics endpoint
curl -s http://localhost:7061/metrics | head -20
```

## Prometheus Configuration

### Scrape Configuration

```yaml
# configs/prometheus/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'helixagent'
    scrape_interval: 10s
    metrics_path: /metrics
    static_configs:
      - targets: ['host.docker.internal:7061']
        labels:
          environment: 'production'
          service: 'helixagent'

  - job_name: 'helixagent-infra'
    static_configs:
      - targets: ['host.docker.internal:15432']
        labels:
          component: 'postgresql'

rule_files:
  - 'alerts/*.yml'
```

### Useful PromQL Queries

```promql
# Request rate (last 5 minutes)
rate(helixagent_requests_total[5m])

# P95 latency
histogram_quantile(0.95, rate(helixagent_request_duration_seconds_bucket[5m]))

# Error rate percentage
100 * rate(helixagent_requests_total{status=~"5.."}[5m])
  / rate(helixagent_requests_total[5m])

# Provider latency comparison
histogram_quantile(0.95, rate(helixagent_provider_latency_seconds_bucket[5m]))

# Cache hit ratio
rate(helixagent_cache_hits_total[5m])
  / (rate(helixagent_cache_hits_total[5m]) + rate(helixagent_cache_misses_total[5m]))

# Circuit breaker open providers
helixagent_circuit_breaker_state == 2
```

## Grafana Dashboards

### Importing the HelixAgent Dashboard

1. Open Grafana at `http://localhost:3000` (default credentials: admin / helixagent)
2. Navigate to Dashboards > Import
3. Import the dashboard JSON from `configs/grafana/dashboards/helixagent-overview.json`

### Recommended Dashboard Panels

| Panel | Visualization | Query |
|---|---|---|
| Request Rate | Time Series | `rate(helixagent_requests_total[5m])` |
| P95 Latency | Time Series | `histogram_quantile(0.95, ...)` |
| Error Rate | Stat | `100 * rate(...{status=~"5.."}[5m]) / rate(...[5m])` |
| Active Requests | Gauge | `helixagent_active_requests` |
| Provider Latency Heatmap | Heatmap | `helixagent_provider_latency_seconds_bucket` |
| Circuit Breaker States | State Timeline | `helixagent_circuit_breaker_state` |
| Token Usage | Bar Chart | `increase(helixagent_token_usage_total[1h])` |
| Memory Usage | Time Series | `helixagent_memory_alloc_bytes` |
| Goroutine Count | Time Series | `helixagent_goroutines` |
| Debate Duration | Histogram | `helixagent_debate_duration_seconds_bucket` |

### Provider Comparison Dashboard

Create a dedicated dashboard that compares all 47+ LLM providers side by side:

- Response latency per provider (grouped bar chart)
- Error rate per provider (stacked area)
- Token consumption per provider (pie chart)
- Circuit breaker state per provider (status map)

## Custom Metrics in HelixAgent

### Registering Custom Metrics

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    DebateConvergenceTime = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "helixagent_debate_convergence_seconds",
            Help:    "Time to reach consensus in debate rounds",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
        },
        []string{"topology", "voting_method"},
    )

    ProviderFallbackCount = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "helixagent_provider_fallback_total",
            Help: "Number of provider fallback events",
        },
        []string{"from_provider", "to_provider", "reason"},
    )
)

func init() {
    prometheus.MustRegister(DebateConvergenceTime)
    prometheus.MustRegister(ProviderFallbackCount)
}
```

### Instrumenting Handler Code

```go
func (h *Handler) HandleChat(c *gin.Context) {
    timer := prometheus.NewTimer(requestDuration.WithLabelValues("chat"))
    defer timer.ObserveDuration()

    activeRequests.Inc()
    defer activeRequests.Dec()

    // ... handler logic ...
}
```

## Provider Latency Tracking

Each of the 47+ LLM providers is tracked independently. The verification pipeline at startup establishes baseline latency scores.

### Viewing Provider Performance

```bash
# Check provider health and latency via API
curl -s http://localhost:7061/v1/monitoring/provider-health | jq .

# View circuit breaker states
curl -s http://localhost:7061/v1/monitoring/circuit-breakers | jq .

# Force a health check refresh
curl -s -X POST http://localhost:7061/v1/monitoring/force-health-check
```

### Provider Scoring Weights

Performance scoring during verification uses these weights:

| Component | Weight |
|---|---|
| ResponseSpeed | 25% |
| CostEffectiveness | 25% |
| ModelEfficiency | 20% |
| Capability | 20% |
| Recency | 10% |

Providers with a composite score below 5.0 are excluded from the active debate team.

## Performance Baselines

Establish baselines after initial deployment to detect regressions:

| Metric | Acceptable Range | Warning | Critical |
|---|---|---|---|
| P50 Latency | < 200ms | 200-500ms | > 500ms |
| P95 Latency | < 500ms | 500ms-1s | > 1s |
| P99 Latency | < 1s | 1-2s | > 2s |
| Error Rate | < 0.1% | 0.1-1% | > 1% |
| Cache Hit Ratio | > 80% | 60-80% | < 60% |
| Goroutine Count | < 1000 | 1000-5000 | > 5000 |
| Memory Usage | < 60% | 60-80% | > 80% |
| Provider Response | < 2s | 2-5s | > 5s |

Run the benchmark suite to capture baselines:

```bash
make test-bench
```

## Alerting

### Alert Rules Configuration

```yaml
# configs/prometheus/alerts/helixagent.yml
groups:
  - name: helixagent-alerts
    rules:
      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(helixagent_request_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High P95 latency detected"
          description: "P95 latency is {{ $value }}s (threshold: 0.5s)"

      - alert: HighErrorRate
        expr: |
          100 * rate(helixagent_requests_total{status=~"5.."}[5m])
            / rate(helixagent_requests_total[5m]) > 1
        for: 3m
        labels:
          severity: critical
        annotations:
          summary: "Error rate above 1%"

      - alert: CircuitBreakerOpen
        expr: helixagent_circuit_breaker_state == 2
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Circuit breaker open for {{ $labels.provider }}"

      - alert: HighMemoryUsage
        expr: helixagent_memory_alloc_bytes / 1024 / 1024 > 800
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Memory usage above 800MB"

      - alert: AllProvidersDown
        expr: count(helixagent_circuit_breaker_state == 0) == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "No healthy LLM providers available"
```

### AlertManager Routing

```yaml
# configs/alertmanager/alertmanager.yml
route:
  receiver: 'default'
  group_by: ['alertname', 'severity']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  routes:
    - match:
        severity: critical
      receiver: 'pagerduty'

receivers:
  - name: 'default'
    webhook_configs:
      - url: 'http://localhost:7061/v1/notifications/webhook'
  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: '<PAGERDUTY_KEY>'
```

## Monitoring Architecture

```
                          +-------------------+
                          |   Grafana :3000    |
                          |  (Visualization)  |
                          +--------+----------+
                                   |
                          +--------v----------+
                          | Prometheus :9090   |
                          |  (Metrics Store)  |
                          +--------+----------+
                                   | scrape /metrics
                    +--------------+--------------+
                    |              |               |
           +-------v---+  +------v----+  +-------v-------+
           | HelixAgent |  | HelixAgent |  | Infrastructure |
           |   :7061    |  |   :7062   |  |  (PG, Redis)  |
           +------------+  +-----------+  +---------------+
                    |
           +--------v----------+
           | AlertManager :9093 |
           |   (Routing)       |
           +--------+----------+
                    |
           +--------v----------+
           |  Notification     |
           |  (Webhook/Email)  |
           +-------------------+
```

## CLI and API Access

### Monitoring Endpoints

```bash
# Full system status
curl -s http://localhost:7061/v1/monitoring/status | jq .

# Provider health summary
curl -s http://localhost:7061/v1/monitoring/provider-health | jq .

# Circuit breaker states
curl -s http://localhost:7061/v1/monitoring/circuit-breakers | jq .

# Fallback chain status
curl -s http://localhost:7061/v1/monitoring/fallback-chain | jq .

# Reset circuit breakers (force closed)
curl -s -X POST http://localhost:7061/v1/monitoring/reset-circuits

# Force immediate health check
curl -s -X POST http://localhost:7061/v1/monitoring/force-health-check
```

### Make Targets

```bash
make monitoring-status         # Show system status
make circuit-breakers          # Show circuit breaker states
make provider-health           # Show provider health
make fallback-chain            # Show fallback chain
make monitoring-reset-circuits # Reset all circuit breakers
make force-health-check        # Trigger health check cycle
```

## Troubleshooting

### Prometheus Cannot Scrape HelixAgent

**Symptom:** Target shows as DOWN in Prometheus.

**Solutions:**
1. Verify HelixAgent is running: `curl http://localhost:7061/metrics`
2. Check network connectivity from the Prometheus container: `docker exec prometheus wget -qO- http://host.docker.internal:7061/metrics`
3. If using Docker, ensure `host.docker.internal` resolves or use the host IP directly
4. Verify the metrics endpoint is not behind authentication middleware

### Missing Custom Metrics

**Symptom:** Custom metrics do not appear in Prometheus.

**Solutions:**
1. Confirm metrics are registered with `prometheus.MustRegister(...)` in an `init()` function
2. Ensure the metric has been observed at least once (counters start at 0, histograms appear after first observation)
3. Check for label cardinality issues (too many unique label values)

### High Memory Usage in Prometheus

**Symptom:** Prometheus uses excessive memory.

**Solutions:**
1. Reduce retention: `--storage.tsdb.retention.time=15d`
2. Increase scrape interval from 10s to 30s
3. Drop high-cardinality metrics with `metric_relabel_configs`
4. Use recording rules to pre-aggregate expensive queries

### Grafana Dashboard Shows No Data

**Symptom:** Panels show "No data" despite metrics existing.

**Solutions:**
1. Verify the Prometheus data source is configured correctly in Grafana
2. Check the time range selector (default may be too narrow)
3. Test the PromQL query directly in the Prometheus UI at `http://localhost:9090/graph`
4. Ensure the metric names match exactly (check for typos in label filters)

## Related Resources

- [User Manual 23: Observability Setup](23-observability-setup.md) -- OpenTelemetry and tracing configuration
- [User Manual 29: Disaster Recovery](29-disaster-recovery.md) -- Monitoring during failover
- [User Manual 30: Enterprise Architecture](30-enterprise-architecture.md) -- Production monitoring at scale
- HelixAgent source: `internal/observability/` -- metrics and tracing implementation
- Prometheus documentation: https://prometheus.io/docs/
- Grafana documentation: https://grafana.com/docs/
