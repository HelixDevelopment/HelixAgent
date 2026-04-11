# Phase-3 Memory-Safety Observability

> Added 2026-04-11 during the memory-safety remediation pass. This doc
> covers the five Prometheus gauges that expose the Phase-3 hot-path
> counters, the decoupled wiring pattern that keeps domain packages
> free of any observability imports, the Grafana dashboard that
> visualises them, and the env-gated SLI test that smokes them end-to-end.

## Why this exists

The 2026-04-11 memory-safety audit found four CRITICAL issues on HelixAgent's
hottest paths: unbounded growth in the ensemble worker pool's pending-results
map, unbounded growth in the security guardrail pipeline's per-guardrail stats
map, a data race on the stats counters, and two cache mutexes without `defer`.
Fixing the code was necessary but not sufficient — without observability you
cannot prove the fix is holding under production load. This document describes
the observability layer that was added on top of the fixes.

## The five Phase-3 metrics

All metrics are registered against the default Prometheus global registry at
router boot via `internal/observability/metrics/phase3_gauges.go`. They are
served at `/metrics` through the standard `promhttp.Handler()` that already
powers the rest of HelixAgent's Prometheus surface.

| Metric | Type | Source | What it means |
|---|---|---|---|
| `helixagent_ensemble_pending_results` | Gauge | `WorkerPool.PendingCount()` | Current number of in-flight `SubmitAsync` tasks holding a per-task delivery channel. |
| `helixagent_ensemble_pending_results_cap` | Gauge | `background.DefaultMaxPendingResults` (or `WorkerPool.SetMaxPendingResults` override) | Configured hard cap on pending tasks. Rejection fires at this value. |
| `helixagent_ensemble_tasks_rejected_total` | Counter | `WorkerPool.GetStats()["tasks_rejected"]` | Cumulative count of `SubmitAsync` calls rejected because the cap was hit. |
| `helixagent_guardrails_stats_keys` | Gauge | `StandardGuardrailPipeline.StatsKeyCount()` | Current number of distinct guardrail names in the pipeline stats map. |
| `helixagent_guardrails_stats_dropped_total` | Counter | `StandardGuardrailPipeline.StatsKeysDropped()` | Cumulative count of stats updates dropped because the `MaxGuardrailStatsKeys=1024` cap was hit. |

All five are defined in `internal/observability/metrics/phase3_gauges.go` as
`prometheus.NewGaugeFunc` / `NewCounterFunc` — they pull from their contributor
singletons at scrape time, so mutations in the domain layer are visible without
any push step.

## Decoupled wiring pattern

A naive integration would import the Prometheus client library directly from
`internal/ensemble/background/worker_pool.go` and `internal/security/guardrails.go`.
That creates a cycle the moment those packages want to read back their own
metric handles, and it hard-couples domain code to observability. The Phase-3
wiring inverts the dependency via a small singleton in the observability
package itself:

```
┌─────────────────────────────────────────────────────────────┐
│ internal/observability/metrics/phase3_source.go             │
│                                                             │
│   defaultPhase3Source {                                     │
│     worker    WorkerPoolContributor                         │
│     guardrail GuardrailContributor                          │
│   }                                                         │
│                                                             │
│   type WorkerPoolContributor struct {                       │
│     PendingCount  func() int64                              │
│     PendingCap    func() int64                              │
│     TasksRejected func() uint64                             │
│   }                                                         │
│                                                             │
│   type GuardrailContributor struct {                        │
│     KeyCount    func() int64                                │
│     KeysDropped func() int64                                │
│   }                                                         │
└─────────────────────────────────────────────────────────────┘
         ▲                                   ▲
         │ SetEnsembleWorkerPoolContributor   │ SetGuardrailPipelineContributor
         │                                   │
┌────────┴──────────┐              ┌─────────┴──────────┐
│ internal/ensemble │              │ internal/security  │
│ multi_instance/   │              │ integration.go     │
│ coordinator.go    │              │ (CreateDefault     │
│ (creates pool)    │              │  Pipeline path)    │
└───────────────────┘              └────────────────────┘
```

Every contributor field is a zero-arg accessor, so the singleton never holds
a reference to any domain type — it just calls the func on each Prometheus
scrape (typically every 15 s). Missing accessors return zero, so
`RegisterDefaultPhase3Metrics` can run once at router boot regardless of
initialization order; the gauges light up as soon as the live instances
register their contributors.

## Installation sequence

Wiring order is driven by the router; any service that has not yet
registered its contributor reports zero until it does.

```go
// 1. Router boot (internal/router/router.go, after the HTTP metrics init)
if _, err := httpmetrics.RegisterDefaultPhase3Metrics(); err != nil {
    log.Printf("Warning: failed to register Phase-3 metrics: %v", err)
}

// 2. Ensemble worker pool construction
//    (internal/ensemble/multi_instance/coordinator.go, after NewWorkerPool)
obsmetrics.SetEnsembleWorkerPoolContributor(obsmetrics.WorkerPoolContributor{
    PendingCount: c.workerPool.PendingCount,
    PendingCap:   func() int64 { return background.DefaultMaxPendingResults },
    TasksRejected: func() uint64 {
        if v, ok := c.workerPool.GetStats()["tasks_rejected"].(uint64); ok {
            return v
        }
        return 0
    },
})

// 3. Guardrail pipeline construction
//    (internal/security/integration.go, when CreateDefaultPipeline returns)
obsmetrics.SetGuardrailPipelineContributor(obsmetrics.GuardrailContributor{
    KeyCount:    pipeline.StatsKeyCount,
    KeysDropped: pipeline.StatsKeysDropped,
})
```

## Grafana dashboard

**Location:** `docker/monitoring/grafana/dashboards/phase3-memory-safety.json`
**UID:** `phase3-memory-safety`
**Refresh:** 30 s

Eight panels:

1. **Ensemble Worker Pool — Pending Depth** (timeseries) — overlay of
   `pending_results` and `pending_results_cap`. A healthy pool oscillates
   near zero; sustained climbing toward the cap indicates backpressure.
2. **Ensemble Worker Pool — Utilization** (timeseries) — `pending / cap`
   with green / yellow (0.6) / red (0.8) thresholds.
3. **Ensemble — Tasks Rejected (rate/min)** (stat) — background colour
   flips red on any non-zero rate.
4. **Ensemble — Tasks Rejected (cumulative)** (timeseries) — monotonic
   counter. Flat = healthy.
5. **Guardrail Stats — Distinct Keys Tracked** (timeseries) — thresholds
   at 512 (yellow) and 900 (red) against the 1024 cap.
6. **Guardrail Stats — Drops (rate/min)** (stat) — red on any drop.
7. **Context** (row separator).
8. **Reading this dashboard** (markdown text panel) — describes four
   named operator states: healthy / backpressure warning / cap-hit
   incident / guardrail namespace pollution, with the specific counter
   signals that distinguish them and where to look in the code.

Grafana's file provider (see `docker/monitoring/grafana/datasources/`) picks
this up automatically on the next monitoring-stack boot.

## Env-gated SLI smoke test

**Location:** `tests/monitoring/phase3_sli_live_test.go`
**Activation:** `HELIX_MONITOR_URL=http://localhost:7061/metrics go test -v ./tests/monitoring/...`

`TestPhase3_SLI_Live` scrapes the live `/metrics` endpoint when
`HELIX_MONITOR_URL` is set and asserts:

- all five Phase-3 metrics are present
- idle utilization (`pending/cap`) ≤ 20%
- `tasks_rejected_total` = 0 at boot
- `stats_keys` ≤ 128 (healthy is ≤ 10; this is a loose ceiling to catch
  namespace pollution, not a precise SLO)
- `stats_dropped_total` = 0 at boot

Without the env var the test skips cleanly — the default
`go test ./tests/monitoring/` stays hermetic. Three helper tests
(`..._ScrapeHelper`, `..._ScrapeHelper_HTTPError`,
`..._ScrapeHelper_MalformedLines`) run unconditionally against an
`httptest.Server` so the text-exposition parser stays correct even
when the live path is skipped.

## Operator playbook

### Healthy state
- `pending_results` sawtooth between 0 and a few dozen
- `pending_results_cap` a horizontal line at 10_000 (or overridden value)
- `utilization` < 0.2
- `tasks_rejected_total` flat at 0
- `guardrail_stats_keys` around 5–10 (matches `CreateDefaultPipeline`'s count)
- `guardrail_stats_dropped_total` flat at 0

### Backpressure warning
- `pending_results` climbs toward the cap
- `utilization` stays above 0.6 for > 1 minute
- **Action:** investigate consumer-side throughput. The worker pool
  has `size * 10` queue slots; if they fill, `SubmitAsync` callers
  block on the per-task channel. Look at downstream LLM provider
  latency (`helixagent_provider_latency_seconds`) and any stuck
  goroutines in the ensemble pipeline.

### Cap-hit incident
- `tasks_rejected_total` slope > 0
- **Action:** the cap has actually fired. Either the pool is
  undersized for current load (raise `DefaultMaxPendingResults` or
  the pool's own `size`), or a bug is leaking per-task channels
  (check `storePending`/`deletePending` pairing and the
  `SubmitAsync` defer chain in
  `internal/ensemble/background/worker_pool.go`).

### Guardrail namespace pollution
- `guardrail_stats_keys` approaches 1024, or `stats_dropped_total` slope > 0
- **Action:** something is producing guardrail names as unique strings
  instead of the fixed allow-list from `CreateDefaultPipeline`. Search
  logs for recent `Guardrail triggered` lines and look at the
  `guardrail` field — if it contains user input or request IDs, fix
  the caller, not the cap.

## Related code

- **Domain counters:** `internal/ensemble/background/worker_pool.go`
  (`PendingCount`, `DefaultMaxPendingResults`, `tasksRejected`),
  `internal/security/guardrails.go` (`StatsKeyCount`, `StatsKeysDropped`,
  `MaxGuardrailStatsKeys`)
- **Prometheus collectors:** `internal/observability/metrics/phase3_gauges.go`
- **Contributor singleton:** `internal/observability/metrics/phase3_source.go`
- **Wiring points:** `internal/router/router.go`,
  `internal/ensemble/multi_instance/coordinator.go`,
  `internal/security/integration.go`
- **Dashboard:** `docker/monitoring/grafana/dashboards/phase3-memory-safety.json`
- **SLI test:** `tests/monitoring/phase3_sli_live_test.go`
- **Regression tests:** `internal/observability/metrics/phase3_gauges_test.go`,
  `phase3_source_test.go`, plus the per-package Phase-3 tests
  (`worker_pool_phase3_test.go`, `guardrails_phase3_test.go`,
  `provider_cache_phase3_test.go`)

## Non-negotiables

- Phase-3 metrics **never** fail the boot path. Registration errors are
  logged as warnings (see `router.go`) — metrics are observability, not
  a boot gate.
- Domain packages **never** import `prometheus/client_golang`. The
  contributor pattern in `phase3_source.go` is the only allowed bridge.
- Raising `DefaultMaxPendingResults` or `MaxGuardrailStatsKeys` is a
  **security-sensitive change**. Either must be justified in the PR
  description and validated against a real memory profile, not just
  bumped to make a dashboard alert go away.
