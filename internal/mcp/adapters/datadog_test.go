// Package adapters provides MCP server adapter tests.
package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDatadogAdapter(t *testing.T) {
	t.Parallel()
	config := DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		Site:           "datadoghq.com",
	}

	adapter := NewDatadogAdapter(config)

	assert.NotNil(t, adapter)
	assert.Equal(t, "api-key", adapter.config.APIKey)
	assert.Equal(t, "app-key", adapter.config.ApplicationKey)
	assert.Equal(t, "datadoghq.com", adapter.config.Site)
}

func TestDatadogAdapter_GetServerInfo(t *testing.T) {
	t.Parallel()
	adapter := NewDatadogAdapter(DatadogConfig{})

	info := adapter.GetServerInfo()

	assert.Equal(t, "datadog", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "Datadog")
	assert.NotEmpty(t, info.Capabilities)
	assert.Contains(t, info.Capabilities, "query_metrics")
	assert.Contains(t, info.Capabilities, "list_dashboards")
}

func TestDatadogAdapter_ListTools(t *testing.T) {
	t.Parallel()
	adapter := NewDatadogAdapter(DatadogConfig{})

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	assert.True(t, toolNames["datadog_query_metrics"])
	assert.True(t, toolNames["datadog_list_dashboards"])
	assert.True(t, toolNames["datadog_get_dashboard"])
	assert.True(t, toolNames["datadog_list_monitors"])
	assert.True(t, toolNames["datadog_get_monitor"])
	assert.True(t, toolNames["datadog_mute_monitor"])
	assert.True(t, toolNames["datadog_unmute_monitor"])
	assert.True(t, toolNames["datadog_query_logs"])
	assert.True(t, toolNames["datadog_list_hosts"])
	assert.True(t, toolNames["datadog_list_events"])
	assert.True(t, toolNames["datadog_create_event"])
}

func TestDatadogAdapter_CallTool_UnknownTool(t *testing.T) {
	t.Parallel()
	adapter := NewDatadogAdapter(DatadogConfig{})

	result, err := adapter.CallTool(context.Background(), "unknown_tool", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestDatadogAdapter_QueryMetrics(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check path - note: server.URL includes scheme and host, so path is relative
		assert.True(t, strings.HasSuffix(r.URL.Path, "/api/v1/query"))
		assert.Equal(t, "api-key", r.Header.Get("DD-API-KEY"))
		assert.Equal(t, "app-key", r.Header.Get("DD-APPLICATION-KEY"))

		response := DDMetricsResponse{
			Status: "ok",
			Series: []DDSeries{
				{
					DisplayName: "avg:system.cpu.user{*}",
					Scope:       "*",
					Pointlist:   [][]interface{}{{1704067200000.0, 25.5}, {1704070800000.0, 30.2}},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Parse server URL to get just host:port
	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_query_metrics", map[string]interface{}{
		"query": "avg:system.cpu.user{*}",
		"from":  float64(1704067200),
		"to":    float64(1704070800),
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "system.cpu.user")
}

func TestDatadogAdapter_ListDashboards(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/api/v1/dashboard"))

		response := DDDashboardsResponse{
			Dashboards: []DDDashboardSummary{
				{ID: "dash-1", Title: "System Overview", LayoutType: "ordered", URL: "/dashboard/dash-1"},
				{ID: "dash-2", Title: "Application Metrics", LayoutType: "free", URL: "/dashboard/dash-2"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_list_dashboards", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "System Overview")
}

func TestDatadogAdapter_GetDashboard(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v1/dashboard/dash-1")

		response := DDDashboard{
			ID:          "dash-1",
			Title:       "System Overview",
			Description: "System metrics dashboard",
			LayoutType:  "ordered",
			CreatedAt:   "2024-01-01T00:00:00Z",
			ModifiedAt:  "2024-01-02T00:00:00Z",
			Widgets:     []interface{}{map[string]interface{}{"type": "timeseries"}},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_get_dashboard", map[string]interface{}{
		"dashboard_id": "dash-1",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "System Overview")
}

func TestDatadogAdapter_ListMonitors(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/api/v1/monitor"))

		response := []DDMonitor{
			{
				ID:           123,
				Name:         "High CPU Alert",
				Type:         "metric alert",
				Query:        "avg(last_5m):avg:system.cpu.user{*} > 80",
				OverallState: "OK",
				Tags:         []string{"env:production"},
			},
			{
				ID:           124,
				Name:         "Low Memory Alert",
				Type:         "metric alert",
				Query:        "avg(last_5m):avg:system.mem.used{*} > 90",
				OverallState: "Alert",
				Tags:         []string{"env:production"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_list_monitors", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "High CPU Alert")
	assert.Contains(t, result.Content[0].Text, "[OK]")
	assert.Contains(t, result.Content[0].Text, "[ALERT]")
}

func TestDatadogAdapter_GetMonitor(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v1/monitor/123")

		response := DDMonitor{
			ID:           123,
			Name:         "High CPU Alert",
			Type:         "metric alert",
			Query:        "avg(last_5m):avg:system.cpu.user{*} > 80",
			Message:      "CPU usage is high!",
			OverallState: "OK",
			Tags:         []string{"env:production", "team:ops"},
			Created:      "2024-01-01T00:00:00Z",
			Modified:     "2024-01-02T00:00:00Z",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_get_monitor", map[string]interface{}{
		"monitor_id": float64(123),
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "High CPU Alert")
	assert.Contains(t, result.Content[0].Text, "env:production")
}

func TestDatadogAdapter_MuteMonitor(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/monitor/123/mute")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_mute_monitor", map[string]interface{}{
		"monitor_id": float64(123),
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "muted")
}

func TestDatadogAdapter_UnmuteMonitor(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/monitor/123/unmute")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_unmute_monitor", map[string]interface{}{
		"monitor_id": float64(123),
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "unmuted")
}

func TestDatadogAdapter_QueryLogs(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/api/v2/logs/events/search"))

		response := DDLogsResponse{
			Data: []DDLogEntry{
				{
					ID: "log-1",
					Attributes: DDLogAttributes{
						Timestamp: "2024-01-01T00:00:00Z",
						Service:   "web-app",
						Status:    "error",
						Message:   "Connection timeout",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_query_logs", map[string]interface{}{
		"query": "service:web-app status:error",
		"from":  "now-1h",
		"to":    "now",
		"limit": 50,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "web-app")
	assert.Contains(t, result.Content[0].Text, "Connection timeout")
}

func TestDatadogAdapter_ListHosts(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/api/v1/hosts"))

		response := DDHostsResponse{
			TotalReturned: 2,
			HostList: []DDHost{
				{ID: 1, Name: "web-server-1", IsUp: true, Apps: []string{"nginx", "node"}, Meta: &DDHostMeta{Platform: "linux"}},
				{ID: 2, Name: "web-server-2", IsUp: true, Apps: []string{"nginx"}, Meta: &DDHostMeta{Platform: "linux"}},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_list_hosts", map[string]interface{}{
		"count": 100,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "web-server-1")
	assert.Contains(t, result.Content[0].Text, "nginx")
}

func TestDatadogAdapter_ListEvents(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/api/v1/events"))

		response := DDEventsResponse{
			Events: []DDEvent{
				{ID: 1, Title: "Deployment completed", Text: "v1.2.3 deployed", DateHappened: 1704067200, AlertType: "info"},
				{ID: 2, Title: "High error rate", Text: "Error rate above threshold", DateHappened: 1704070800, AlertType: "warning"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_list_events", map[string]interface{}{
		"start": float64(1704067200),
		"end":   float64(1704153600),
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Deployment completed")
}

func TestDatadogAdapter_CreateEvent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/api/v1/events"))

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Test Event", body["title"])
		assert.Equal(t, "Test event body", body["text"])

		response := DDEventCreateResponse{
			Event: DDEvent{ID: 123, Title: "Test Event"},
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "api-key",
		ApplicationKey: "app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_create_event", map[string]interface{}{
		"title":      "Test Event",
		"text":       "Test event body",
		"alert_type": "info",
		"priority":   "normal",
		"tags":       []interface{}{"env:test"},
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Event created")
}

func TestDatadogAdapter_APIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors": ["Invalid API key"]}`))
	}))
	defer server.Close()

	serverURL := server.URL

	adapter := NewDatadogAdapter(DatadogConfig{
		APIKey:         "invalid-key",
		ApplicationKey: "invalid-app-key",
		BaseURL:        serverURL,
	})

	result, err := adapter.CallTool(context.Background(), "datadog_list_dashboards", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "API error")
}

func TestDefaultDatadogConfig(t *testing.T) {
	t.Parallel()
	config := DefaultDatadogConfig()

	assert.Equal(t, "datadoghq.com", config.Site)
	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestGetInt64Arg(t *testing.T) {
	t.Parallel()
	args := map[string]interface{}{
		"float64_val": float64(123),
		"int64_val":   int64(456),
		"int_val":     789,
		"string_val":  "not a number",
	}

	assert.Equal(t, int64(123), getInt64Arg(args, "float64_val", 0))
	assert.Equal(t, int64(456), getInt64Arg(args, "int64_val", 0))
	assert.Equal(t, int64(789), getInt64Arg(args, "int_val", 0))
	assert.Equal(t, int64(999), getInt64Arg(args, "missing", 999))
	assert.Equal(t, int64(0), getInt64Arg(args, "string_val", 0))
}
