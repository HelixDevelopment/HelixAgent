package servers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewFetchAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, nil)

	assert.NotNil(t, adapter)
	assert.False(t, adapter.initialized)
	assert.Equal(t, "HelixAgent-Fetch/1.0", adapter.config.UserAgent)
}

func TestDefaultFetchAdapterConfig(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()

	assert.Equal(t, "HelixAgent-Fetch/1.0", config.UserAgent)
	assert.Equal(t, 30*1000000000, int(config.Timeout))
	assert.Equal(t, 10, config.MaxRedirects)
	assert.Equal(t, int64(10*1024*1024), config.MaxResponseSize)
	assert.False(t, config.IgnoreRobotsTxt)
}

func TestNewFetchAdapter_DefaultConfig(t *testing.T) {
	t.Parallel()
	config := FetchAdapterConfig{}
	adapter := NewFetchAdapter(config, logrus.New())

	assert.Equal(t, "HelixAgent-Fetch/1.0", adapter.config.UserAgent)
	assert.Equal(t, 30*1000000000, int(adapter.config.Timeout))
	assert.Equal(t, 10, adapter.config.MaxRedirects)
}

func TestFetchAdapter_Health_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Health(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestFetchAdapter_Fetch_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	_, err := adapter.Fetch(context.Background(), "https://example.com", "GET", nil, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestFetchAdapter_Close(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())
	adapter.initialized = true

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestFetchAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	tools := adapter.GetMCPTools()
	assert.Len(t, tools, 4)

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "fetch_url")
	assert.Contains(t, toolNames, "fetch_json")
	assert.Contains(t, toolNames, "fetch_extract_links")
	assert.Contains(t, toolNames, "fetch_extract_text")
}

func TestFetchAdapter_ExecuteTool_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	_, err := adapter.ExecuteTool(context.Background(), "fetch_url", map[string]interface{}{
		"url": "https://example.com",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestFetchAdapter_ExecuteTool_UnknownTool(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	_, err := adapter.ExecuteTool(context.Background(), "unknown_tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestFetchAdapter_GetCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	caps := adapter.GetCapabilities()
	assert.Equal(t, "fetch", caps["name"])
	assert.Equal(t, "HelixAgent-Fetch/1.0", caps["user_agent"])
	assert.Equal(t, 10, caps["max_redirects"])
	assert.Equal(t, int64(10*1024*1024), caps["max_response_size"])
	assert.Equal(t, 4, caps["tools"])
	assert.Equal(t, false, caps["initialized"])
}

func TestFetchAdapter_Initialize(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	assert.True(t, adapter.initialized)

	err = adapter.Close()
	assert.NoError(t, err)
}

func TestFetchAdapter_Health(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	err = adapter.Health(context.Background())
	assert.NoError(t, err)
}

func TestFetchAdapter_isDomainAllowed(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	// No restrictions - all allowed
	assert.True(t, adapter.isDomainAllowed("example.com"))
	assert.True(t, adapter.isDomainAllowed("google.com"))

	// With blocked domains
	adapter.config.BlockedDomains = []string{"blocked.com"}
	assert.False(t, adapter.isDomainAllowed("blocked.com"))
	assert.False(t, adapter.isDomainAllowed("sub.blocked.com"))
	assert.True(t, adapter.isDomainAllowed("example.com"))

	// With allowed domains
	adapter.config.AllowedDomains = []string{"allowed.com"}
	adapter.config.BlockedDomains = []string{}
	assert.True(t, adapter.isDomainAllowed("allowed.com"))
	assert.True(t, adapter.isDomainAllowed("sub.allowed.com"))
	assert.False(t, adapter.isDomainAllowed("example.com"))

	// Blocked takes precedence
	adapter.config.BlockedDomains = []string{"allowed.com"}
	assert.False(t, adapter.isDomainAllowed("allowed.com"))
}

func TestFetchAdapter_Fetch_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>Hello World</body></html>"))
	}))
	defer server.Close()

	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.Fetch(context.Background(), server.URL, "GET", nil, "")
	assert.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)
	assert.Contains(t, result.Content, "Hello World")
	assert.Contains(t, result.ContentType, "text/html")
}

func TestFetchAdapter_Fetch_WithHeaders(t *testing.T) {
	t.Parallel()
	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom-Header")
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	headers := map[string]string{"X-Custom-Header": "test-value"}
	_, err = adapter.Fetch(context.Background(), server.URL, "GET", headers, "")
	assert.NoError(t, err)
	assert.Equal(t, "test-value", receivedHeader)
}

func TestFetchAdapter_Fetch_POST(t *testing.T) {
	t.Parallel()
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		body, _ := json.Marshal(map[string]string{"status": "ok"})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.Fetch(context.Background(), server.URL, "POST", nil, `{"test":"data"}`)
	assert.NoError(t, err)
	assert.Equal(t, "POST", receivedMethod)
	assert.Equal(t, 200, result.StatusCode)
}

func TestFetchAdapter_Fetch_InvalidURL(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Fetch(context.Background(), "://invalid", "GET", nil, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestFetchAdapter_Fetch_BlockedDomain(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	config.BlockedDomains = []string{"blocked.com"}
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Fetch(context.Background(), "https://blocked.com/page", "GET", nil, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "domain not allowed")
}

func TestFetchAdapter_Fetch_404(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.Fetch(context.Background(), server.URL, "GET", nil, "")
	assert.NoError(t, err)
	assert.Equal(t, 404, result.StatusCode)
}

func TestFetchAdapter_FetchJSON_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name":  "test",
			"value": 123,
		})
	}))
	defer server.Close()

	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.FetchJSON(context.Background(), server.URL, nil)
	assert.NoError(t, err)

	data, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "test", data["name"])
	assert.Equal(t, float64(123), data["value"])
}

func TestFetchAdapter_FetchJSON_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	_, err := adapter.FetchJSON(context.Background(), "https://example.com", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestFetchAdapter_FetchJSON_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.FetchJSON(context.Background(), server.URL, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JSON")
}

func TestFetchAdapter_ExtractLinks(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	html := `
		<html>
		<body>
			<a href="https://example.com/page1">Page 1</a>
			<a href="/page2">Page 2</a>
			<a href="page3.html">Page 3</a>
			<a href="javascript:void(0)">Skip</a>
			<a href="#section">Skip Anchor</a>
		</body>
		</html>
	`

	links, err := adapter.ExtractLinks(html, "https://base.com/dir/")
	assert.NoError(t, err)
	assert.Len(t, links, 3)
	assert.Contains(t, links, "https://example.com/page1")
	assert.Contains(t, links, "https://base.com/page2")
	assert.Contains(t, links, "https://base.com/dir/page3.html")
}

func TestFetchAdapter_ExtractText(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	html := `
		<html>
		<head><script>var x = 1;</script></head>
		<body>
			<style>.hidden{display:none}</style>
			<h1>Title</h1>
			<p>Hello &amp; World</p>
			<div>Some    spaced    text</div>
		</body>
		</html>
	`

	text := adapter.ExtractText(html)
	assert.Contains(t, text, "Title")
	assert.Contains(t, text, "Hello & World")
	assert.Contains(t, text, "Some spaced text")
	assert.NotContains(t, text, "var x = 1")
	assert.NotContains(t, text, ".hidden")
}

func TestFetchAdapter_ExecuteTool_FetchURL(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Test Content"))
	}))
	defer server.Close()

	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.ExecuteTool(context.Background(), "fetch_url", map[string]interface{}{
		"url": server.URL,
	})
	assert.NoError(t, err)
	fetchResult := result.(*FetchResult)
	assert.Equal(t, 200, fetchResult.StatusCode)
}

func TestFetchAdapter_ExecuteTool_FetchJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"key": "value"})
	}))
	defer server.Close()

	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.ExecuteTool(context.Background(), "fetch_json", map[string]interface{}{
		"url": server.URL,
	})
	assert.NoError(t, err)
	data := result.(map[string]interface{})
	assert.Equal(t, "value", data["key"])
}

func TestFetchAdapter_ExecuteTool_ExtractLinks(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<a href="https://example.com">Link</a>`))
	}))
	defer server.Close()

	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.ExecuteTool(context.Background(), "fetch_extract_links", map[string]interface{}{
		"url": server.URL,
	})
	assert.NoError(t, err)
	links := result.([]string)
	assert.Contains(t, links, "https://example.com")
}

func TestFetchAdapter_ExecuteTool_ExtractText(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><p>Hello World</p></body></html>`))
	}))
	defer server.Close()

	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.ExecuteTool(context.Background(), "fetch_extract_text", map[string]interface{}{
		"url": server.URL,
	})
	assert.NoError(t, err)
	text := result.(string)
	assert.Contains(t, text, "Hello World")
}

func TestFetchAdapter_MarshalJSON(t *testing.T) {
	t.Parallel()
	config := DefaultFetchAdapterConfig()
	adapter := NewFetchAdapter(config, logrus.New())

	data, err := adapter.MarshalJSON()
	assert.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Contains(t, result, "initialized")
	assert.Contains(t, result, "capabilities")
}
