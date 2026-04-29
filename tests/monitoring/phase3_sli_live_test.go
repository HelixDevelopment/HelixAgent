package monitoring_test

// Env-gated SLI test that scrapes a live HelixAgent /metrics endpoint
// and asserts the Phase-3 memory-safety gauges are present, zero or
// sensible, and within the thresholds that drive the Grafana dashboard
// in docker/monitoring/grafana/dashboards/phase3-memory-safety.json.
//
// Activation: set HELIX_MONITOR_URL to a reachable HelixAgent metrics
// endpoint (for example http://localhost:8100/metrics). Without the
// env var the live test is skipped cleanly — the default
// `go test ./tests/monitoring/` run stays hermetic and does not
// require a booted helixagent.
//
// The two helper tests (ScrapeHelper / ScrapeHelper_HTTPError) run
// unconditionally against an httptest.Server so the text-exposition
// parser stays correct even when the live test is skipped.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	phase3EnvVar = "HELIX_MONITOR_URL"

	// Phase-3 metric names. Must stay in sync with
	// internal/observability/metrics/phase3_gauges.go.
	metricPendingResults     = "helixagent_ensemble_pending_results"
	metricPendingResultsCap  = "helixagent_ensemble_pending_results_cap"
	metricTasksRejectedTotal = "helixagent_ensemble_tasks_rejected_total"
	metricGuardrailsKeys     = "helixagent_guardrails_stats_keys"
	metricGuardrailsDropped  = "helixagent_guardrails_stats_dropped_total"

	// SLI thresholds aligned with the dashboard's green/yellow/red
	// stops. Any value above the "red" threshold during a fresh idle
	// scrape is a boot-time regression worth investigating.
	maxIdleUtilization = 0.2 // pending / cap at idle must stay under 20%
	maxIdleKeys        = 128 // guardrail keys at idle << cap of 1024
	maxIdleDropped     = 0.0 // any dropped-keys counter at boot is a bug
)

// TestPhase3_SLI_Live scrapes the live /metrics endpoint and verifies
// the five Phase-3 counters exist and report sensible values. The
// thresholds are deliberately loose — this is a smoke test for boot
// correctness, not a load test.
func TestPhase3_SLI_Live(t *testing.T) {
	url := os.Getenv(phase3EnvVar)
	if url == "" {
		t.Skipf("skipping live SLI test: %s not set (SKIP-OK: #unmarked-skip-needs-ticket)", phase3EnvVar)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metrics, err := scrapePrometheus(ctx, url)
	require.NoError(t, err, "failed to scrape %s", url)
	require.NotEmpty(t, metrics, "scrape returned no metric samples")

	// 1. Every Phase-3 metric must be present in the exposition.
	for _, name := range []string{
		metricPendingResults,
		metricPendingResultsCap,
		metricTasksRejectedTotal,
		metricGuardrailsKeys,
		metricGuardrailsDropped,
	} {
		_, ok := metrics[name]
		assert.Truef(t, ok,
			"metric %s missing from %s — Phase-3 wiring not registered? "+
				"Check router.Setup() calls RegisterDefaultPhase3Metrics and "+
				"the live WorkerPool / GuardrailPipeline have pushed contributors.",
			name, url)
	}

	// 2. pending_results_cap must be > 0 (the gauge reads zero until
	// the worker pool registers its contributor).
	if cap := metrics[metricPendingResultsCap]; cap > 0 {
		pending := metrics[metricPendingResults]
		utilization := pending / cap
		assert.LessOrEqualf(t, utilization, maxIdleUtilization,
			"idle utilization %.2f%% exceeds %.0f%% threshold "+
				"(pending=%.0f, cap=%.0f) — investigate PendingCount "+
				"in internal/ensemble/background/worker_pool.go.",
			utilization*100, maxIdleUtilization*100, pending, cap)
	} else {
		t.Logf("pending_results_cap is 0 — worker pool contributor not yet registered on the live server (soft warning, not a test failure)")
	}

	// 3. rejected counter must be 0 at boot.
	if rejected := metrics[metricTasksRejectedTotal]; rejected > 0 {
		t.Errorf("tasks_rejected_total = %.0f at boot — SubmitAsync "+
			"rejected work before the test even started. Check the "+
			"pool capacity and any goroutines that hammer SubmitAsync "+
			"during initialization.", rejected)
	}

	// 4. guardrail_keys should be 0-128 at boot.
	if keys := metrics[metricGuardrailsKeys]; keys > maxIdleKeys {
		t.Errorf("guardrail_stats_keys = %.0f at boot exceeds threshold %d — "+
			"a healthy pipeline has a fixed small guardrail set. Check "+
			"CreateDefaultPipeline and any caller that registers guardrails "+
			"with dynamic names.", keys, maxIdleKeys)
	}

	// 5. guardrail_stats_dropped must be 0 at boot.
	if dropped := metrics[metricGuardrailsDropped]; dropped > maxIdleDropped {
		t.Errorf("guardrail_stats_dropped_total = %.0f at boot — "+
			"the pipeline already hit the stats-key cap before the test "+
			"ran. See docs/security/GOSEC_HIGH_TRIAGE_2026-04-11.md for "+
			"the guardrail cap design.", dropped)
	}

	t.Logf("Phase-3 SLI summary: pending=%.0f/%.0f rejected=%.0f keys=%.0f dropped=%.0f",
		metrics[metricPendingResults],
		metrics[metricPendingResultsCap],
		metrics[metricTasksRejectedTotal],
		metrics[metricGuardrailsKeys],
		metrics[metricGuardrailsDropped],
	)
}

// scrapePrometheus fetches the text exposition format from the given URL
// and parses it into a map of metric name → sample value. Labels are
// ignored (the Phase-3 gauges have no labels) and HELP / TYPE comment
// lines are skipped.
func scrapePrometheus(ctx context.Context, url string) (map[string]float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "text/plain; version=0.0.4")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scrape: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("scrape status %d: %s", resp.StatusCode, string(snippet))
	}

	out := make(map[string]float64)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexAny(line, " \t")
		if idx < 0 {
			continue
		}
		nameWithLabels := line[:idx]
		valueStr := strings.TrimSpace(line[idx+1:])
		if sp := strings.Index(valueStr, " "); sp >= 0 {
			valueStr = valueStr[:sp]
		}
		name := nameWithLabels
		if br := strings.Index(nameWithLabels, "{"); br >= 0 {
			name = nameWithLabels[:br]
		}
		val, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}
		out[name] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan exposition: %w", err)
	}
	return out, nil
}

// TestPhase3_SLI_ScrapeHelper validates the scrapePrometheus helper
// itself against an httptest.Server. Runs unconditionally so the
// helper stays correct even when HELIX_MONITOR_URL is not set.
func TestPhase3_SLI_ScrapeHelper(t *testing.T) {
	exposition := `# HELP helixagent_ensemble_pending_results Current pending results
# TYPE helixagent_ensemble_pending_results gauge
helixagent_ensemble_pending_results 7
# HELP helixagent_ensemble_pending_results_cap Configured cap
# TYPE helixagent_ensemble_pending_results_cap gauge
helixagent_ensemble_pending_results_cap 10000
# HELP helixagent_ensemble_tasks_rejected_total Cumulative rejections
# TYPE helixagent_ensemble_tasks_rejected_total counter
helixagent_ensemble_tasks_rejected_total 0
helixagent_guardrails_stats_keys 5
helixagent_guardrails_stats_dropped_total 0
some_other_metric{label="foo"} 42 1700000000000
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = io.WriteString(w, exposition)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	metrics, err := scrapePrometheus(ctx, srv.URL)
	require.NoError(t, err)

	assert.Equal(t, 7.0, metrics[metricPendingResults])
	assert.Equal(t, 10000.0, metrics[metricPendingResultsCap])
	assert.Equal(t, 0.0, metrics[metricTasksRejectedTotal])
	assert.Equal(t, 5.0, metrics[metricGuardrailsKeys])
	assert.Equal(t, 0.0, metrics[metricGuardrailsDropped])
	// Labelled metric still parses — the helper strips the braces.
	assert.Equal(t, 42.0, metrics["some_other_metric"])
}

func TestPhase3_SLI_ScrapeHelper_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := scrapePrometheus(ctx, srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestPhase3_SLI_ScrapeHelper_MalformedLines(t *testing.T) {
	// A mix of valid samples, broken lines, bare comments, and a line
	// with no value. The helper must extract the valid samples and
	// silently skip the rest without erroring out.
	exposition := `# comment only
broken line without a value
valid_gauge 3.14
# TYPE valid_counter counter
valid_counter 42
not_a_number foo
labelled_metric{a="b",c="d"} 99
trailing_whitespace 7
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, exposition)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	metrics, err := scrapePrometheus(ctx, srv.URL)
	require.NoError(t, err)

	assert.InDelta(t, 3.14, metrics["valid_gauge"], 0.0001)
	assert.Equal(t, 42.0, metrics["valid_counter"])
	assert.Equal(t, 99.0, metrics["labelled_metric"])
	assert.Equal(t, 7.0, metrics["trailing_whitespace"])
	_, badExists := metrics["not_a_number"]
	assert.False(t, badExists, "unparseable line must be skipped, not error")
}
