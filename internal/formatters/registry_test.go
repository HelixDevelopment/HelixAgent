package formatters

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFormatter is a mock formatter for testing
type mockFormatter struct {
	BaseFormatter
	formatFunc func(ctx context.Context, req *FormatRequest) (*FormatResult, error)
	healthFunc func(ctx context.Context) error
}

func newMockFormatter(name string, version string, languages []string) *mockFormatter {
	metadata := &FormatterMetadata{
		Name:            name,
		Version:         version,
		Languages:       languages,
		Type:            FormatterTypeNative,
		SupportsStdin:   true,
		SupportsInPlace: true,
		SupportsCheck:   true,
		SupportsConfig:  true,
	}

	return &mockFormatter{
		BaseFormatter: *NewBaseFormatter(metadata),
	}
}

func (m *mockFormatter) Format(ctx context.Context, req *FormatRequest) (*FormatResult, error) {
	if m.formatFunc != nil {
		return m.formatFunc(ctx, req)
	}

	return &FormatResult{
		Content:          req.Content + " formatted",
		Changed:          true,
		FormatterName:    m.Name(),
		FormatterVersion: m.Version(),
		Success:          true,
		Duration:         10 * time.Millisecond,
	}, nil
}

func (m *mockFormatter) FormatBatch(ctx context.Context, reqs []*FormatRequest) ([]*FormatResult, error) {
	results := make([]*FormatResult, len(reqs))
	for i, req := range reqs {
		result, err := m.Format(ctx, req)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	return results, nil
}

func (m *mockFormatter) HealthCheck(ctx context.Context) error {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return nil
}

func TestFormatterRegistry_Register(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrent:  10,
	}

	registry := NewFormatterRegistry(config, logger)

	formatter := newMockFormatter("black", "26.1a1", []string{"python"})
	metadata := &FormatterMetadata{
		Name:      "black",
		Version:   "26.1a1",
		Languages: []string{"python"},
		Type:      FormatterTypeNative,
	}

	err := registry.Register(formatter, metadata)
	require.NoError(t, err)

	// Verify registration
	assert.Equal(t, 1, registry.Count())

	// Verify can retrieve by name
	retrieved, err := registry.Get("black")
	require.NoError(t, err)
	assert.Equal(t, "black", retrieved.Name())

	// Verify can retrieve by language
	pythonFormatters := registry.GetByLanguage("python")
	assert.Len(t, pythonFormatters, 1)
	assert.Equal(t, "black", pythonFormatters[0].Name())
}

func TestFormatterRegistry_Register_Duplicate(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrent:  10,
	}

	registry := NewFormatterRegistry(config, logger)

	formatter := newMockFormatter("black", "26.1a1", []string{"python"})
	metadata := &FormatterMetadata{
		Name:      "black",
		Version:   "26.1a1",
		Languages: []string{"python"},
		Type:      FormatterTypeNative,
	}

	// First registration should succeed
	err := registry.Register(formatter, metadata)
	require.NoError(t, err)

	// Second registration should fail
	err = registry.Register(formatter, metadata)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestFormatterRegistry_Unregister(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrent:  10,
	}

	registry := NewFormatterRegistry(config, logger)

	formatter := newMockFormatter("black", "26.1a1", []string{"python"})
	metadata := &FormatterMetadata{
		Name:      "black",
		Version:   "26.1a1",
		Languages: []string{"python"},
		Type:      FormatterTypeNative,
	}

	err := registry.Register(formatter, metadata)
	require.NoError(t, err)

	// Unregister
	err = registry.Unregister("black")
	require.NoError(t, err)

	// Verify removed
	assert.Equal(t, 0, registry.Count())

	// Verify not retrievable
	_, err = registry.Get("black")
	assert.Error(t, err)
}

func TestFormatterRegistry_GetByLanguage(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrent:  10,
	}

	registry := NewFormatterRegistry(config, logger)

	// Register multiple Python formatters
	formatters := []struct {
		name    string
		version string
	}{
		{"black", "26.1a1"},
		{"ruff", "0.9.6"},
		{"autopep8", "2.0.4"},
	}

	for _, f := range formatters {
		formatter := newMockFormatter(f.name, f.version, []string{"python"})
		metadata := &FormatterMetadata{
			Name:      f.name,
			Version:   f.version,
			Languages: []string{"python"},
			Type:      FormatterTypeNative,
		}

		err := registry.Register(formatter, metadata)
		require.NoError(t, err)
	}

	// Get all Python formatters
	pythonFormatters := registry.GetByLanguage("python")
	assert.Len(t, pythonFormatters, 3)

	// Verify names
	names := make([]string, len(pythonFormatters))
	for i, f := range pythonFormatters {
		names[i] = f.Name()
	}
	assert.Contains(t, names, "black")
	assert.Contains(t, names, "ruff")
	assert.Contains(t, names, "autopep8")
}

func TestFormatterRegistry_DetectLanguageFromPath(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrent:  10,
	}

	registry := NewFormatterRegistry(config, logger)

	testCases := []struct {
		path     string
		expected string
	}{
		{"main.py", "python"},
		{"script.js", "javascript"},
		{"component.tsx", "typescript"},
		{"main.rs", "rust"},
		{"main.go", "go"},
		{"main.c", "c"},
		{"main.cpp", "cpp"},
		{"Main.java", "java"},
		{"Main.kt", "kotlin"},
		{"Main.scala", "scala"},
		{"script.sh", "bash"},
		{"config.yaml", "yaml"},
		{"data.json", "json"},
		{"config.toml", "toml"},
		{"readme.md", "markdown"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			language := registry.DetectLanguageFromPath(tc.path)
			assert.Equal(t, tc.expected, language, "Failed for path: %s", tc.path)
		})
	}
}

func TestFormatterRegistry_HealthCheckAll(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrent:  10,
	}

	registry := NewFormatterRegistry(config, logger)

	// Register healthy formatter
	healthyFormatter := newMockFormatter("black", "26.1a1", []string{"python"})
	healthyFormatter.healthFunc = func(ctx context.Context) error {
		return nil
	}

	err := registry.Register(healthyFormatter, &FormatterMetadata{
		Name:      "black",
		Version:   "26.1a1",
		Languages: []string{"python"},
		Type:      FormatterTypeNative,
	})
	require.NoError(t, err)

	// Register unhealthy formatter
	unhealthyFormatter := newMockFormatter("ruff", "0.9.6", []string{"python"})
	unhealthyFormatter.healthFunc = func(ctx context.Context) error {
		return assert.AnError
	}

	err = registry.Register(unhealthyFormatter, &FormatterMetadata{
		Name:      "ruff",
		Version:   "0.9.6",
		Languages: []string{"python"},
		Type:      FormatterTypeNative,
	})
	require.NoError(t, err)

	// Run health checks
	ctx := context.Background()
	results := registry.HealthCheckAll(ctx)

	assert.Len(t, results, 2)
	assert.NoError(t, results["black"])
	assert.Error(t, results["ruff"])
}

func TestFormatterRegistry_ListByType(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrent:  10,
	}

	registry := NewFormatterRegistry(config, logger)

	// Register native formatters
	nativeFormatter := newMockFormatter("black", "26.1a1", []string{"python"})
	err := registry.Register(nativeFormatter, &FormatterMetadata{
		Name:      "black",
		Version:   "26.1a1",
		Languages: []string{"python"},
		Type:      FormatterTypeNative,
	})
	require.NoError(t, err)

	// Register service formatter
	serviceFormatter := newMockFormatter("sqlfluff", "3.4.1", []string{"sql"})
	err = registry.Register(serviceFormatter, &FormatterMetadata{
		Name:      "sqlfluff",
		Version:   "3.4.1",
		Languages: []string{"sql"},
		Type:      FormatterTypeService,
	})
	require.NoError(t, err)

	// List native formatters
	nativeNames := registry.ListByType(FormatterTypeNative)
	assert.Len(t, nativeNames, 1)
	assert.Contains(t, nativeNames, "black")

	// List service formatters
	serviceNames := registry.ListByType(FormatterTypeService)
	assert.Len(t, serviceNames, 1)
	assert.Contains(t, serviceNames, "sqlfluff")
}

func TestFormatterRegistry_GetPreferredFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrent:  10,
	}

	registry := NewFormatterRegistry(config, logger)

	// Register multiple Python formatters
	formatters := []string{"black", "ruff", "autopep8"}
	for _, name := range formatters {
		formatter := newMockFormatter(name, "1.0.0", []string{"python"})
		metadata := &FormatterMetadata{
			Name:      name,
			Version:   "1.0.0",
			Languages: []string{"python"},
			Type:      FormatterTypeNative,
		}

		err := registry.Register(formatter, metadata)
		require.NoError(t, err)
	}

	// Test with preferences
	preferences := map[string]string{
		"python": "ruff",
	}

	formatter, err := registry.GetPreferredFormatter("python", preferences)
	require.NoError(t, err)
	assert.Equal(t, "ruff", formatter.Name())

	// Test without preferences (should return first)
	formatter, err = registry.GetPreferredFormatter("python", nil)
	require.NoError(t, err)
	assert.Contains(t, formatters, formatter.Name())
}
