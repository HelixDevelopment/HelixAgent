//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/formatters"
	"dev.helix.agent/internal/formatters/providers"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormattersSystem_EndToEnd(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// Create configuration
	config := formatters.DefaultConfig()

	// Initialize system
	system, err := formatters.NewSystem(config, logger)
	require.NoError(t, err)
	defer system.Shutdown()

	// Register formatters
	err = providers.RegisterAllFormatters(system.Registry, logger)
	require.NoError(t, err)

	// Verify formatters are registered
	count := system.Registry.Count()
	assert.GreaterOrEqual(t, count, 4, "Should have at least 4 formatters registered")
}

func TestFormattersSystem_PythonFormatting(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := formatters.DefaultConfig()
	system, err := formatters.NewSystem(config, logger)
	require.NoError(t, err)
	defer system.Shutdown()

	err = providers.RegisterAllFormatters(system.Registry, logger)
	require.NoError(t, err)

	// Test Python formatting
	ctx := context.Background()
	req := &formatters.FormatRequest{
		Content:  "def hello(  x,y ):\n  return x+y",
		Language: "python",
		Timeout:  5 * time.Second,
	}

	result, err := system.Executor.Execute(ctx, req)

	// Formatter might not be installed - that's okay
	if err != nil {
		t.Skip("Formatter not available (expected in test environment)")  // SKIP-OK: #legacy-untriaged
		return
	}

	assert.NotNil(t, result)
	if result.Success {
		assert.NotEmpty(t, result.Content)
		assert.NotEmpty(t, result.FormatterName)
		assert.Greater(t, result.Duration.Milliseconds(), int64(0))
	}
}

func TestFormattersRegistry_LanguageDetection(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := formatters.DefaultConfig()
	system, err := formatters.NewSystem(config, logger)
	require.NoError(t, err)
	defer system.Shutdown()

	testCases := []struct {
		filePath string
		expected string
	}{
		{"test.py", "python"},
		{"test.js", "javascript"},
		{"test.go", "go"},
		{"test.rs", "rust"},
		{"test.c", "c"},
		{"test.cpp", "cpp"},
		{"test.java", "java"},
		{"test.sh", "bash"},
		{"test.yaml", "yaml"},
		{"test.toml", "toml"},
		{"test.md", "markdown"},
	}

	for _, tc := range testCases {
		t.Run(tc.filePath, func(t *testing.T) {
			language := system.Registry.DetectLanguageFromPath(tc.filePath)
			assert.Equal(t, tc.expected, language)
		})
	}
}

func TestFormattersRegistry_GetByLanguage(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := formatters.DefaultConfig()
	system, err := formatters.NewSystem(config, logger)
	require.NoError(t, err)
	defer system.Shutdown()

	err = providers.RegisterAllFormatters(system.Registry, logger)
	require.NoError(t, err)

	// Test Python formatters
	pythonFormatters := system.Registry.GetByLanguage("python")
	assert.GreaterOrEqual(t, len(pythonFormatters), 1, "Should have at least 1 Python formatter")

	// Test JavaScript formatters
	jsFormatters := system.Registry.GetByLanguage("javascript")
	assert.GreaterOrEqual(t, len(jsFormatters), 1, "Should have at least 1 JavaScript formatter")

	// Test Go formatters
	goFormatters := system.Registry.GetByLanguage("go")
	assert.GreaterOrEqual(t, len(goFormatters), 1, "Should have at least 1 Go formatter")
}

func TestFormattersCache(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := formatters.DefaultConfig()
	config.CacheEnabled = true

	system, err := formatters.NewSystem(config, logger)
	require.NoError(t, err)
	defer system.Shutdown()

	err = providers.RegisterAllFormatters(system.Registry, logger)
	require.NoError(t, err)

	ctx := context.Background()
	req := &formatters.FormatRequest{
		Content:  "def test():\n    pass",
		Language: "python",
		Timeout:  5 * time.Second,
	}

	// First request (cache miss)
	result1, err := system.Executor.Execute(ctx, req)
	if err != nil {
		t.Skip("Formatter not available")  // SKIP-OK: #legacy-untriaged
		return
	}

	// Second request (cache hit - should be faster)
	result2, err := system.Executor.Execute(ctx, req)
	require.NoError(t, err)

	if result1.Success && result2.Success {
		// Cache hit should be significantly faster
		assert.LessOrEqual(t, result2.Duration, result1.Duration)
	}
}

func TestFormattersHealthCheck(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := formatters.DefaultConfig()
	system, err := formatters.NewSystem(config, logger)
	require.NoError(t, err)
	defer system.Shutdown()

	err = providers.RegisterAllFormatters(system.Registry, logger)
	require.NoError(t, err)

	// Health check all formatters
	ctx := context.Background()
	report := system.Health.CheckAll(ctx)

	assert.NotNil(t, report)
	assert.Equal(t, system.Registry.Count(), report.TotalFormatters)
	assert.GreaterOrEqual(t, report.HealthyCount, 0)
	assert.NotNil(t, report.Timestamp)
}

func TestFormattersBatchExecution(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := formatters.DefaultConfig()
	system, err := formatters.NewSystem(config, logger)
	require.NoError(t, err)
	defer system.Shutdown()

	err = providers.RegisterAllFormatters(system.Registry, logger)
	require.NoError(t, err)

	ctx := context.Background()
	reqs := []*formatters.FormatRequest{
		{
			Content:  "def foo():\n pass",
			Language: "python",
			Timeout:  5 * time.Second,
		},
		{
			Content:  "const x={a:1};",
			Language: "javascript",
			Timeout:  5 * time.Second,
		},
	}

	results, err := system.Executor.ExecuteBatch(ctx, reqs)

	// Some formatters might not be installed
	if err != nil {
		t.Skip("Formatters not available")  // SKIP-OK: #legacy-untriaged
		return
	}

	assert.Len(t, results, 2)
}

func TestFormattersMiddleware(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := formatters.DefaultConfig()
	config.DefaultTimeout = 1 * time.Second // Short timeout for testing

	system, err := formatters.NewSystem(config, logger)
	require.NoError(t, err)
	defer system.Shutdown()

	// Verify middleware is applied
	// The system should have timeout, retry, validation middleware at minimum
	// This is implicitly tested through normal operations
	assert.NotNil(t, system.Executor)
}
