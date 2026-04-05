// Package adapters provides MCP server adapter tests.
package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSentryAdapter(t *testing.T) {
	t.Parallel()
	config := SentryConfig{
		AuthToken:    "test-token",
		Organization: "test-org",
	}

	adapter := NewSentryAdapter(config)

	assert.NotNil(t, adapter)
	assert.Equal(t, "test-token", adapter.config.AuthToken)
	assert.Equal(t, "test-org", adapter.config.Organization)
	assert.Equal(t, DefaultSentryConfig().BaseURL, adapter.config.BaseURL)
}

func TestSentryAdapter_GetServerInfo(t *testing.T) {
	t.Parallel()
	adapter := NewSentryAdapter(SentryConfig{})

	info := adapter.GetServerInfo()

	assert.Equal(t, "sentry", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "Sentry")
	assert.NotEmpty(t, info.Capabilities)
	assert.Contains(t, info.Capabilities, "list_projects")
	assert.Contains(t, info.Capabilities, "list_issues")
}

func TestSentryAdapter_ListTools(t *testing.T) {
	t.Parallel()
	adapter := NewSentryAdapter(SentryConfig{})

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	assert.True(t, toolNames["sentry_list_projects"])
	assert.True(t, toolNames["sentry_list_issues"])
	assert.True(t, toolNames["sentry_get_issue"])
	assert.True(t, toolNames["sentry_resolve_issue"])
	assert.True(t, toolNames["sentry_list_events"])
	assert.True(t, toolNames["sentry_get_event"])
	assert.True(t, toolNames["sentry_list_alerts"])
	assert.True(t, toolNames["sentry_query_stats"])
	assert.True(t, toolNames["sentry_search_issues"])
}

func TestSentryAdapter_CallTool_UnknownTool(t *testing.T) {
	t.Parallel()
	adapter := NewSentryAdapter(SentryConfig{})

	result, err := adapter.CallTool(context.Background(), "unknown_tool", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestSentryAdapter_ListProjects(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/organizations/test-org/projects/")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		response := []SentryProject{
			{ID: "1", Name: "Frontend", Slug: "frontend", Platform: "javascript", Status: "active"},
			{ID: "2", Name: "Backend", Slug: "backend", Platform: "python", Status: "active"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSentryAdapter(SentryConfig{
		AuthToken:    "test-token",
		Organization: "test-org",
		BaseURL:      server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sentry_list_projects", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Frontend")
	assert.Contains(t, result.Content[0].Text, "Backend")
}

func TestSentryAdapter_ListIssues(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/projects/test-org/frontend/issues/")
		// Note: query is URL-encoded (is:unresolved becomes is%3Aunresolved)
		assert.Contains(t, r.URL.RawQuery, "query=is%3Aunresolved")

		response := []SentryIssue{
			{
				ID:        "123",
				ShortID:   "FRONT-123",
				Title:     "TypeError: Cannot read property",
				Level:     "error",
				Status:    "unresolved",
				Count:     "42",
				UserCount: "10",
				FirstSeen: "2024-01-01T00:00:00Z",
				LastSeen:  "2024-01-02T00:00:00Z",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSentryAdapter(SentryConfig{
		AuthToken:    "test-token",
		Organization: "test-org",
		BaseURL:      server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sentry_list_issues", map[string]interface{}{
		"project": "frontend",
		"query":   "is:unresolved",
		"sort":    "date",
		"limit":   25,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "FRONT-123")
	assert.Contains(t, result.Content[0].Text, "TypeError")
}

func TestSentryAdapter_GetIssue(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/issues/123/")

		response := SentryIssueDetail{
			SentryIssue: SentryIssue{
				ID:        "123",
				ShortID:   "FRONT-123",
				Title:     "TypeError: Cannot read property",
				Level:     "error",
				Status:    "unresolved",
				Count:     "42",
				UserCount: "10",
				FirstSeen: "2024-01-01T00:00:00Z",
				LastSeen:  "2024-01-02T00:00:00Z",
				Culprit:   "main.js in handleClick",
			},
			Platform: "javascript",
			Metadata: &SentryIssueMetadata{
				Type:     "TypeError",
				Value:    "Cannot read property 'x' of undefined",
				Filename: "main.js",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSentryAdapter(SentryConfig{
		AuthToken:    "test-token",
		Organization: "test-org",
		BaseURL:      server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sentry_get_issue", map[string]interface{}{
		"issue_id": "123",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "FRONT-123")
	assert.Contains(t, result.Content[0].Text, "TypeError")
	assert.Contains(t, result.Content[0].Text, "main.js")
}

func TestSentryAdapter_ResolveIssue(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.URL.Path, "/issues/123/")

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "resolved", body["status"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := NewSentryAdapter(SentryConfig{
		AuthToken:    "test-token",
		Organization: "test-org",
		BaseURL:      server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sentry_resolve_issue", map[string]interface{}{
		"issue_id": "123",
		"status":   "resolved",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "resolved")
}

func TestSentryAdapter_ListEvents(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/issues/123/events/")

		response := []SentryEvent{
			{
				EventID:  "event-1",
				Title:    "TypeError: Cannot read property",
				DateTime: "2024-01-01T00:00:00Z",
				User:     &SentryUser{Email: "user@example.com"},
			},
			{
				EventID:  "event-2",
				Title:    "TypeError: Cannot read property",
				DateTime: "2024-01-01T01:00:00Z",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSentryAdapter(SentryConfig{
		AuthToken:    "test-token",
		Organization: "test-org",
		BaseURL:      server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sentry_list_events", map[string]interface{}{
		"issue_id": "123",
		"limit":    25,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "event-1")
	assert.Contains(t, result.Content[0].Text, "user@example.com")
}

func TestSentryAdapter_GetEvent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/projects/test-org/frontend/events/event-1/")

		response := SentryEventDetail{
			SentryEvent: SentryEvent{
				EventID:  "event-1",
				Title:    "TypeError",
				DateTime: "2024-01-01T00:00:00Z",
				User: &SentryUser{
					ID:        "user-1",
					Email:     "user@example.com",
					IPAddress: "192.168.1.1",
				},
			},
			Platform: "javascript",
			Contexts: map[string]interface{}{
				"browser": map[string]interface{}{"name": "Chrome", "version": "120"},
			},
			Tags: []SentryTag{
				{Key: "environment", Value: "production"},
				{Key: "release", Value: "1.0.0"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSentryAdapter(SentryConfig{
		AuthToken:    "test-token",
		Organization: "test-org",
		BaseURL:      server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sentry_get_event", map[string]interface{}{
		"project":  "frontend",
		"event_id": "event-1",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "event-1")
	assert.Contains(t, result.Content[0].Text, "user@example.com")
}

func TestSentryAdapter_ListAlerts(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/projects/test-org/frontend/rules/")

		response := []SentryAlertRule{
			{
				ID:        "rule-1",
				Name:      "High Error Rate",
				Status:    "active",
				Frequency: 60,
				Conditions: []SentryCondition{
					{ID: "cond-1", Name: "event.type:error"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSentryAdapter(SentryConfig{
		AuthToken:    "test-token",
		Organization: "test-org",
		BaseURL:      server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sentry_list_alerts", map[string]interface{}{
		"project": "frontend",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "High Error Rate")
}

func TestSentryAdapter_QueryStats(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/projects/test-org/frontend/stats/")

		response := [][]interface{}{
			{1704067200000.0, 100.0},
			{1704070800000.0, 150.0},
			{1704074400000.0, 75.0},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSentryAdapter(SentryConfig{
		AuthToken:    "test-token",
		Organization: "test-org",
		BaseURL:      server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sentry_query_stats", map[string]interface{}{
		"project":    "frontend",
		"stat":       "received",
		"resolution": "1h",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "received")
}

func TestSentryAdapter_SearchIssues(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/organizations/test-org/issues/")
		assert.Contains(t, r.URL.RawQuery, "query=TypeError")

		response := []SentryIssue{
			{
				ID:      "123",
				ShortID: "FRONT-123",
				Title:   "TypeError in frontend",
				Status:  "unresolved",
				Count:   "10",
				Project: &SentryProject{Slug: "frontend"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSentryAdapter(SentryConfig{
		AuthToken:    "test-token",
		Organization: "test-org",
		BaseURL:      server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sentry_search_issues", map[string]interface{}{
		"query": "TypeError",
		"limit": 25,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "FRONT-123")
}

func TestSentryAdapter_APIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail": "Invalid token"}`))
	}))
	defer server.Close()

	adapter := NewSentryAdapter(SentryConfig{
		AuthToken:    "invalid-token",
		Organization: "test-org",
		BaseURL:      server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sentry_list_projects", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "API error")
}

func TestDefaultSentryConfig(t *testing.T) {
	t.Parallel()
	config := DefaultSentryConfig()

	assert.Equal(t, "https://sentry.io/api/0", config.BaseURL)
	assert.Equal(t, 30*time.Second, config.Timeout)
}
