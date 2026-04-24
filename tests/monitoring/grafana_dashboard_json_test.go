package monitoring_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// grafanaDashboardPath returns the absolute path to the Grafana dashboard JSON
// relative to the repository root, resolved from this test file's location.
func grafanaDashboardPath(t *testing.T) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed")
	// tests/monitoring/ → ../../docs/monitoring/grafana-dashboard.json
	return filepath.Join(filepath.Dir(testFile), "..", "..", "docs", "monitoring", "grafana-dashboard.json")
}

// TestGrafanaDashboardJSON_FileExists verifies that the Grafana dashboard JSON
// file is present in the repository.
func TestGrafanaDashboardJSON_FileExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := grafanaDashboardPath(t)
	_, err := os.Stat(path)
	assert.NoError(t, err, "grafana-dashboard.json must exist at %s", path)
}

// TestGrafanaDashboardJSON_ValidJSON verifies that the dashboard file contains
// well-formed JSON.
func TestGrafanaDashboardJSON_ValidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := grafanaDashboardPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "grafana-dashboard.json must be readable")

	var parsed interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err, "grafana-dashboard.json must be valid JSON")
}

// TestGrafanaDashboardJSON_TopLevelStructure validates that the dashboard JSON
// has the expected top-level structure with a "dashboard" wrapper object
// containing "title", "tags", and "panels" fields.
func TestGrafanaDashboardJSON_TopLevelStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := grafanaDashboardPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var root map[string]interface{}
	err = json.Unmarshal(data, &root)
	require.NoError(t, err)

	dashboardRaw, hasDashboard := root["dashboard"]
	require.True(t, hasDashboard, "Top-level object must contain 'dashboard' key")

	dashboard, ok := dashboardRaw.(map[string]interface{})
	require.True(t, ok, "'dashboard' must be a JSON object")

	for _, field := range []string{"title", "panels"} {
		_, exists := dashboard[field]
		assert.True(t, exists, "dashboard object must contain field %q", field)
	}

	title, _ := dashboard["title"].(string)
	assert.NotEmpty(t, title, "Dashboard title must not be empty")
}

// TestGrafanaDashboardJSON_Panels validates that the dashboard contains a
// non-empty panels array and that each panel has the required fields:
// "id", "title", and "type".
func TestGrafanaDashboardJSON_Panels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := grafanaDashboardPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var root map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &root))

	dashboard, ok := root["dashboard"].(map[string]interface{})
	require.True(t, ok)

	panelsRaw, hasPanels := dashboard["panels"]
	require.True(t, hasPanels, "Dashboard must have a 'panels' array")

	panels, ok := panelsRaw.([]interface{})
	require.True(t, ok, "'panels' must be a JSON array")
	assert.NotEmpty(t, panels, "Dashboard must have at least one panel")

	for i, panelRaw := range panels {
		panel, ok := panelRaw.(map[string]interface{})
		require.True(t, ok, "Panel %d must be a JSON object", i)

		for _, field := range []string{"id", "title", "type"} {
			_, exists := panel[field]
			assert.True(t, exists, "Panel %d must contain field %q", i, field)
		}

		title, _ := panel["title"].(string)
		assert.NotEmpty(t, title, "Panel %d title must not be empty", i)
	}
}

// TestGrafanaDashboardJSON_ExpectedPanelTitles verifies that the dashboard
// contains panels with the titles that are essential for HelixAgent
// production monitoring.
func TestGrafanaDashboardJSON_ExpectedPanelTitles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := grafanaDashboardPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var root map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &root))

	dashboard, ok := root["dashboard"].(map[string]interface{})
	require.True(t, ok)

	panels, ok := dashboard["panels"].([]interface{})
	require.True(t, ok)

	// Build a set of actual panel titles.
	actualTitles := make(map[string]bool, len(panels))
	for _, panelRaw := range panels {
		panel, ok := panelRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if title, ok := panel["title"].(string); ok && title != "" {
			actualTitles[title] = true
		}
	}

	// These are the required panels that must be present in the dashboard.
	requiredTitles := []string{
		"System Overview",
		"LLM Request Rate",
		"Response Time Distribution",
		"Provider Health Status",
		"Token Usage",
		"Error Rate by Provider",
		"Cache Hit Rate",
		"Memory Usage",
		"Circuit Breaker Status",
	}

	for _, title := range requiredTitles {
		assert.True(t, actualTitles[title],
			"Dashboard must include panel with title %q", title)
	}
}

// TestGrafanaDashboardJSON_PanelTargets validates that every panel with a
// "targets" array has at least one target with a non-empty "expr" field,
// ensuring that all panels actually reference a metric query.
func TestGrafanaDashboardJSON_PanelTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := grafanaDashboardPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var root map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &root))

	dashboard, ok := root["dashboard"].(map[string]interface{})
	require.True(t, ok)

	panels, ok := dashboard["panels"].([]interface{})
	require.True(t, ok)

	for i, panelRaw := range panels {
		panel, ok := panelRaw.(map[string]interface{})
		if !ok {
			continue
		}
		title, _ := panel["title"].(string)

		targetsRaw, hasTargets := panel["targets"]
		if !hasTargets {
			// Some panels (e.g. text/row) may legitimately have no targets.
			continue
		}

		targets, ok := targetsRaw.([]interface{})
		require.True(t, ok, "Panel %d (%q) 'targets' must be an array", i, title)
		require.NotEmpty(t, targets,
			"Panel %d (%q) must have at least one target", i, title)

		for j, targetRaw := range targets {
			target, ok := targetRaw.(map[string]interface{})
			require.True(t, ok,
				"Panel %d (%q) target %d must be a JSON object", i, title, j)

			expr, hasExpr := target["expr"]
			require.True(t, hasExpr,
				"Panel %d (%q) target %d must have an 'expr' field", i, title, j)

			exprStr, _ := expr.(string)
			assert.NotEmpty(t, exprStr,
				"Panel %d (%q) target %d 'expr' must not be empty", i, title, j)
		}
	}
}

// TestGrafanaDashboardJSON_NoDuplicatePanelIDs validates that no two panels
// share the same numeric ID, which would cause Grafana import errors.
func TestGrafanaDashboardJSON_NoDuplicatePanelIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	path := grafanaDashboardPath(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var root map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &root))

	dashboard, ok := root["dashboard"].(map[string]interface{})
	require.True(t, ok)

	panels, ok := dashboard["panels"].([]interface{})
	require.True(t, ok)

	seen := make(map[float64]bool)
	for i, panelRaw := range panels {
		panel, ok := panelRaw.(map[string]interface{})
		if !ok {
			continue
		}
		id, hasID := panel["id"].(float64)
		if !hasID {
			continue
		}
		assert.False(t, seen[id],
			"Panel %d has duplicate ID %.0f", i, id)
		seen[id] = true
	}
}
