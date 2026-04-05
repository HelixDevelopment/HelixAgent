# Grafana Dashboard Guide

**Date:** 2026-03-30
**Status:** Active

## Overview

HelixAgent ships with pre-configured Grafana dashboards and Prometheus alert rules for monitoring system health, provider performance, and resource utilization. The monitoring stack is containerized via `docker/monitoring/`.

---

## Dashboard Inventory

### 1. HelixAgent Overview (`helixagent-dashboard.json`)

**Location:** `monitoring/dashboards/helixagent-dashboard.json`
**Purpose:** Top-level system overview showing request rates, error rates, response times, and active connections.
**Key panels:**
- Request rate (requests/sec by endpoint)
- Error rate (5xx/4xx breakdown)
- Response time histogram (p50, p95, p99)
- Active goroutine count
- Memory usage (heap, stack, GC)
- Uptime and health status

### 2. Messaging Dashboard (`messaging-dashboard.json`)

**Location:** `monitoring/dashboards/messaging-dashboard.json`
**Purpose:** Kafka and RabbitMQ monitoring: message throughput, consumer lag, dead letter queue depth.
**Key panels:**
- Messages produced/consumed per second
- Consumer group lag
- Dead letter queue size
- Broker connection status
- Message processing latency

### 3. Provider Health (API overview panels)

**Embedded in:** HelixAgent dashboard, provider-specific panels
**Purpose:** Per-provider health status, circuit breaker states, response times, and error rates.
**Key metrics:**
- `helixagent_provider_health_status` -- 1 (healthy) or 0 (unhealthy) per provider
- `helixagent_provider_response_time_seconds` -- histogram by provider
- `helixagent_circuit_breaker_state` -- 0 (closed), 1 (half-open), 2 (open)
- `helixagent_provider_errors_total` -- counter by provider and error type

### 4. Ensemble Performance (API overview panels)

**Embedded in:** HelixAgent dashboard
**Purpose:** Ensemble voting latency, strategy distribution, provider participation rates.
**Key metrics:**
- `helixagent_ensemble_voting_duration_seconds`
- `helixagent_ensemble_strategy_used_total` -- counter by strategy
- `helixagent_ensemble_providers_participating` -- gauge

### 5. Resource Utilization (system panels)

**Embedded in:** HelixAgent dashboard
**Purpose:** Go runtime metrics: goroutines, heap, GC pauses, CPU usage.
**Key metrics:**
- `go_goroutines` -- current goroutine count
- `go_memstats_heap_alloc_bytes` -- heap allocation
- `go_gc_duration_seconds` -- GC pause duration
- `process_cpu_seconds_total` -- CPU time consumed

### 6. Cache Performance (API overview panels)

**Embedded in:** HelixAgent dashboard
**Purpose:** Cache hit/miss rates, eviction counts, memory usage.
**Key metrics:**
- `helixagent_cache_hits_total` / `helixagent_cache_misses_total`
- `helixagent_cache_evictions_total`
- `helixagent_cache_size_bytes`

### 7. MCP Adapter Status (API overview panels)

**Embedded in:** HelixAgent dashboard
**Purpose:** MCP adapter health, request counts, and error rates per adapter.
**Key metrics:**
- `helixagent_mcp_adapter_status` -- per adapter
- `helixagent_mcp_request_duration_seconds` -- histogram
- `helixagent_mcp_errors_total` -- counter by adapter

---

## Prometheus Configuration

**Config file:** `monitoring/prometheus.yml`
**Scrape targets:** HelixAgent exposes metrics at `/metrics` (Prometheus format).

**Key scrape config:**
```yaml
scrape_configs:
  - job_name: 'helixagent'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:7061']
```

Additional monitoring components:
- `monitoring/alertmanager.yml` -- AlertManager configuration
- `monitoring/blackbox.yml` -- Blackbox exporter for endpoint probing
- `monitoring/helixagent-exporter.py` -- Custom Python exporter for additional metrics
- `monitoring/loki-config.yml` -- Loki log aggregation config
- `monitoring/promtail-config.yml` -- Promtail log shipping config
- `monitoring/grafana-datasources.yml` -- Grafana datasource provisioning
- `monitoring/grafana-dashboards.yml` -- Grafana dashboard provisioning

---

## Alert Rules

**File:** `monitoring/alert-rules.yml`

### Pre-Configured Alerts

| Alert Name | Condition | Severity | For Duration |
|------------|-----------|----------|-------------|
| `HelixAgentDown` | `up{job="helixagent"} == 0` | critical | 1m |
| `HealthCheckFailing` | `helixagent_health_status == 0` | critical | 2m |
| `HelixAgentHighResponseTime` | Response time > 5000ms | warning | 2m |
| `HighLatency` | p99 latency > 30s | warning | 5m |
| `HighErrorRate` | Error rate > 10% | warning | 5m |

### Alert Groups

- **helixagent_availability** -- Service up/down and health checks (30s interval)
- **helixagent_performance** -- Latency and error rate thresholds (30s interval)

---

## Starting the Monitoring Stack

```bash
make infra-start          # Starts all infrastructure including monitoring
# Or specifically:
cd docker/monitoring && docker compose up -d
```

**Default ports:**
- Prometheus: 9090
- Grafana: 3000 (default credentials: admin/admin)
- AlertManager: 9093
- Loki: 3100

---

## Adding New Dashboards

1. Create the dashboard in Grafana UI (easiest approach)
2. Export as JSON via Grafana's Share > Export > Save to file
3. Save to `monitoring/dashboards/<name>.json`
4. Reference it in `monitoring/grafana-dashboards.yml` for auto-provisioning

**Naming convention:** `<subsystem>-dashboard.json`

**Adding new metrics to expose:**
1. Add the metric in Go code using `prometheus.NewCounterVec`, `prometheus.NewHistogramVec`, etc.
2. Register it in the metrics registry (`internal/observability/`)
3. Add a panel in the relevant dashboard JSON
4. Test with `./challenges/scripts/monitoring_dashboard_challenge.sh`

---

## Monitoring Makefile Targets

```bash
make monitoring-status          # Show status of all monitoring components
make circuit-breakers           # Show circuit breaker states
make provider-health            # Show provider health status
make fallback-chain             # Show fallback chain status
make monitoring-reset-circuits  # Reset all circuit breakers
make force-health-check         # Force immediate health check
```

---

## Cross-References

- Monitoring system docs: `docs/monitoring/MONITORING_SYSTEM.md`
- Prometheus monitoring: `docs/monitoring/PROMETHEUS_MONITORING.md`
- Observability architecture: `docs/observability/`
- Dashboard challenge: `./challenges/scripts/monitoring_dashboard_challenge.sh`
- Grafana dashboard JSON: `docs/monitoring/grafana-dashboard.json`
