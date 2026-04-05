package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.agent/internal/formatters"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServiceFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:            "test-service",
		Type:            formatters.FormatterTypeService,
		Version:         "1.0.0",
		Languages:       []string{"testlang"},
		SupportsStdin:   true,
		SupportsInPlace: false,
		SupportsCheck:   true,
		SupportsConfig:  true,
	}

	formatter := NewServiceFormatter(metadata, "http://localhost:9999", 10*time.Second, logger)
	assert.NotNil(t, formatter)
	assert.Equal(t, "test-service", formatter.Name())
	assert.Equal(t, "1.0.0", formatter.Version())
	assert.Contains(t, formatter.Languages(), "testlang")
	assert.True(t, formatter.SupportsStdin())
	assert.False(t, formatter.SupportsInPlace())
	assert.True(t, formatter.SupportsCheck())
	assert.True(t, formatter.SupportsConfig())
}

func TestServiceFormatter_Format_Success(t *testing.T) {
	t.Parallel()
	// Create a test server that mimics a formatter service
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/format", r.URL.Path)
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Return a successful response
		response := `{
			"success": true,
			"content": "formatted content",
			"changed": true,
			"formatter": "test-formatter"
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:      "test-formatter",
		Type:      formatters.FormatterTypeService,
		Languages: []string{"test"},
	}
	formatter := NewServiceFormatter(metadata, server.URL, 5*time.Second, logger)

	ctx := context.Background()
	req := &formatters.FormatRequest{
		Content:  "original content",
		Language: "test",
	}

	result, err := formatter.Format(ctx, req)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "formatted content", result.Content)
	assert.True(t, result.Changed)
	assert.Equal(t, "test-formatter", result.FormatterName)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestServiceFormatter_Format_ServiceError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"success": false,
			"error": "Invalid syntax",
			"formatter": "test"
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:      "test",
		Type:      formatters.FormatterTypeService,
		Languages: []string{"test"},
	}
	formatter := NewServiceFormatter(metadata, server.URL, 5*time.Second, logger)

	ctx := context.Background()
	req := &formatters.FormatRequest{Content: "test", Language: "test"}

	result, err := formatter.Format(ctx, req)
	// Format returns error when service returns success: false
	require.Error(t, err)
	assert.Contains(t, err.Error(), "formatter service error")
	assert.False(t, result.Success)
}

func TestServiceFormatter_Format_HTTPError(t *testing.T) {
	t.Parallel()
	// Server that returns 500 Internal Server Error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:      "test",
		Type:      formatters.FormatterTypeService,
		Languages: []string{"test"},
	}
	formatter := NewServiceFormatter(metadata, server.URL, 5*time.Second, logger)

	ctx := context.Background()
	req := &formatters.FormatRequest{Content: "test", Language: "test"}

	result, err := formatter.Format(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.False(t, result.Success)
}

func TestServiceFormatter_HealthCheck_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/health", r.URL.Path)
		response := `{
			"status": "healthy",
			"formatter": "test",
			"version": "1.0"
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:      "test",
		Type:      formatters.FormatterTypeService,
		Languages: []string{"test"},
	}
	formatter := NewServiceFormatter(metadata, server.URL, 5*time.Second, logger)

	ctx := context.Background()
	err := formatter.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestServiceFormatter_HealthCheck_Unhealthy(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:      "test",
		Type:      formatters.FormatterTypeService,
		Languages: []string{"test"},
	}
	formatter := NewServiceFormatter(metadata, server.URL, 5*time.Second, logger)

	ctx := context.Background()
	err := formatter.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unhealthy status code")
}

func TestServiceFormatter_FormatBatch(t *testing.T) {
	t.Parallel()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		response := `{
			"success": true,
			"content": "formatted",
			"changed": true,
			"formatter": "test"
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:      "test",
		Type:      formatters.FormatterTypeService,
		Languages: []string{"test"},
	}
	formatter := NewServiceFormatter(metadata, server.URL, 5*time.Second, logger)

	ctx := context.Background()
	reqs := []*formatters.FormatRequest{
		{Content: "content1", Language: "test"},
		{Content: "content2", Language: "test"},
		{Content: "content3", Language: "test"},
	}

	results, err := formatter.FormatBatch(ctx, reqs)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, 3, requestCount, "Should have made 3 HTTP requests")
	for _, result := range results {
		assert.True(t, result.Success)
		assert.Equal(t, "formatted", result.Content)
	}
}

func TestSpecificServiceFormatters(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	testCases := []struct {
		name     string
		function func(string, *logrus.Logger) *ServiceFormatter
		baseURL  string
	}{
		{"autopep8", NewAutopep8Formatter, "http://localhost"},
		{"yapf", NewYapfFormatter, "http://localhost"},
		{"sqlfluff", NewSQLFluffFormatter, "http://localhost"},
		{"rubocop", NewRubocopFormatter, "http://localhost"},
		{"standardrb", NewStandardRBFormatter, "http://localhost"},
		{"php-cs-fixer", NewPHPCSFixerFormatter, "http://localhost"},
		{"laravel-pint", NewLaravelPintFormatter, "http://localhost"},
		{"perltidy", NewPerltidyFormatter, "http://localhost"},
		{"cljfmt", NewCljfmtFormatter, "http://localhost"},
		{"spotless", NewSpotlessFormatter, "http://localhost"},
		{"npm-groovy-lint", NewGroovyLintFormatter, "http://localhost"},
		{"styler", NewStylerFormatter, "http://localhost"},
		{"air", NewAirFormatter, "http://localhost"},
		{"psscriptanalyzer", NewPSScriptAnalyzerFormatter, "http://localhost"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			formatter := tc.function(tc.baseURL, logger)
			assert.NotNil(t, formatter)
			assert.Equal(t, tc.name, formatter.Name())
			// Should have at least one language
			assert.Greater(t, len(formatter.Languages()), 0)
		})
	}
}
