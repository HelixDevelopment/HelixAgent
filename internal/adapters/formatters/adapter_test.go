package formatters_test

import (
	"context"
	"testing"
	"time"

	adapter "dev.helix.agent/internal/adapters/formatters"
	"dev.helix.agent/internal/formatters"
	genericfmt "digital.vasic.formatters/pkg/formatter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToGenericRequest(t *testing.T) {
	t.Parallel()
	req := &formatters.FormatRequest{
		Content:    "test content",
		FilePath:   "/test/path.go",
		Language:   "go",
		LineLength: 100,
		IndentSize: 4,
		UseTabs:    false,
		CheckOnly:  true,
		Timeout:    30 * time.Second,
		AgentName:  "test-agent",
		SessionID:  "session-123",
		RequestID:  "req-456",
	}

	generic := adapter.ToGenericRequest(req)

	assert.Equal(t, req.Content, generic.Content)
	assert.Equal(t, req.FilePath, generic.FilePath)
	assert.Equal(t, req.Language, generic.Language)
	assert.Equal(t, req.LineLength, generic.LineLength)
	assert.Equal(t, req.IndentSize, generic.IndentSize)
	assert.Equal(t, req.UseTabs, generic.UseTabs)
	assert.Equal(t, req.CheckOnly, generic.CheckOnly)
	assert.Equal(t, req.Timeout, generic.Timeout)
	assert.Equal(t, req.RequestID, generic.RequestID)
}

func TestFromGenericResult(t *testing.T) {
	t.Parallel()
	generic := &genericfmt.FormatResult{
		Content:          "formatted content",
		Changed:          true,
		FormatterName:    "gofmt",
		FormatterVersion: "1.24",
		Duration:         100 * time.Millisecond,
		Success:          true,
		Warnings:         []string{"warning1"},
		Stats: &genericfmt.FormatStats{
			LinesTotal:   10,
			LinesChanged: 2,
			BytesTotal:   100,
			BytesChanged: 20,
			Violations:   1,
		},
	}

	result := adapter.FromGenericResult(generic)

	assert.Equal(t, generic.Content, result.Content)
	assert.Equal(t, generic.Changed, result.Changed)
	assert.Equal(t, generic.FormatterName, result.FormatterName)
	assert.Equal(t, generic.FormatterVersion, result.FormatterVersion)
	assert.Equal(t, generic.Duration, result.Duration)
	assert.Equal(t, generic.Success, result.Success)
	assert.Equal(t, generic.Warnings, result.Warnings)
	require.NotNil(t, result.Stats)
	assert.Equal(t, generic.Stats.LinesTotal, result.Stats.LinesTotal)
	assert.Equal(t, generic.Stats.LinesChanged, result.Stats.LinesChanged)
}

func TestToGenericMetadata(t *testing.T) {
	t.Parallel()
	metadata := &formatters.FormatterMetadata{
		Name:            "gofmt",
		Type:            formatters.FormatterTypeNative,
		Architecture:    "binary",
		Version:         "1.24",
		Languages:       []string{"go"},
		SupportsStdin:   true,
		SupportsInPlace: true,
	}

	generic := adapter.ToGenericMetadata(metadata)

	assert.Equal(t, metadata.Name, generic.Name)
	assert.Equal(t, genericfmt.FormatterTypeNative, generic.Type)
	assert.Equal(t, metadata.Architecture, generic.Architecture)
	assert.Equal(t, metadata.Version, generic.Version)
	assert.Equal(t, metadata.Languages, generic.Languages)
	assert.Equal(t, metadata.SupportsStdin, generic.SupportsStdin)
	assert.Equal(t, metadata.SupportsInPlace, generic.SupportsInPlace)
}

func TestFromGenericMetadata(t *testing.T) {
	t.Parallel()
	generic := &genericfmt.FormatterMetadata{
		Name:          "black",
		Type:          genericfmt.FormatterTypeNative,
		Architecture:  "python",
		Version:       "26.1",
		Languages:     []string{"python"},
		SupportsStdin: true,
		SupportsCheck: true,
	}

	metadata := adapter.FromGenericMetadata(generic)

	assert.Equal(t, generic.Name, metadata.Name)
	assert.Equal(t, formatters.FormatterTypeNative, metadata.Type)
	assert.Equal(t, generic.Architecture, metadata.Architecture)
	assert.Equal(t, generic.Version, metadata.Version)
	assert.Equal(t, generic.Languages, metadata.Languages)
	assert.Equal(t, generic.SupportsStdin, metadata.SupportsStdin)
	assert.Equal(t, generic.SupportsCheck, metadata.SupportsCheck)
}

func TestNilConversions(t *testing.T) {
	t.Parallel()
	assert.Nil(t, adapter.ToGenericRequest(nil))
	assert.Nil(t, adapter.FromGenericResult(nil))
	assert.Nil(t, adapter.ToGenericMetadata(nil))
	assert.Nil(t, adapter.FromGenericMetadata(nil))
}

func TestDetectLanguageFromPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     string
		expected string
	}{
		{"test.go", "go"},
		{"test.py", "python"},
		{"test.js", "javascript"},
		{"test.ts", "typescript"},
		{"test.rs", "rust"},
		{"test.java", "java"},
		{"test.sql", "sql"},
		{"test.yaml", "yaml"},
		{"test.yml", "yaml"},
		{"test.json", "json"},
		{"test.toml", "toml"},
		{"test.md", "markdown"},
		{"test", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
				t.Parallel()
			result := adapter.DetectLanguageFromPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockFormatter implements the Formatter interface for testing.
type mockFormatter struct {
	name      string
	version   string
	languages []string
}

func (m *mockFormatter) Name() string                                { return m.name }
func (m *mockFormatter) Version() string                             { return m.version }
func (m *mockFormatter) Languages() []string                         { return m.languages }
func (m *mockFormatter) SupportsStdin() bool                         { return true }
func (m *mockFormatter) SupportsInPlace() bool                       { return true }
func (m *mockFormatter) SupportsCheck() bool                         { return true }
func (m *mockFormatter) SupportsConfig() bool                        { return true }
func (m *mockFormatter) HealthCheck(ctx context.Context) error       { return nil }
func (m *mockFormatter) ValidateConfig(map[string]interface{}) error { return nil }
func (m *mockFormatter) DefaultConfig() map[string]interface{}       { return nil }

func (m *mockFormatter) Format(
	ctx context.Context,
	req *genericfmt.FormatRequest,
) (*genericfmt.FormatResult, error) {
	return &genericfmt.FormatResult{
		Content:       req.Content + " // formatted",
		Changed:       true,
		FormatterName: m.name,
		Success:       true,
	}, nil
}

func (m *mockFormatter) FormatBatch(
	ctx context.Context,
	reqs []*genericfmt.FormatRequest,
) ([]*genericfmt.FormatResult, error) {
	results := make([]*genericfmt.FormatResult, len(reqs))
	for i, req := range reqs {
		result, _ := m.Format(ctx, req)
		results[i] = result
	}
	return results, nil
}

func TestFormatterAdapter(t *testing.T) {
	t.Parallel()
	mock := &mockFormatter{
		name:      "test-formatter",
		version:   "1.0.0",
		languages: []string{"test"},
	}

	adapted := adapter.NewFormatterAdapter(mock)

	assert.Equal(t, "test-formatter", adapted.Name())
	assert.Equal(t, "1.0.0", adapted.Version())
	assert.Equal(t, []string{"test"}, adapted.Languages())
	assert.True(t, adapted.SupportsStdin())
	assert.True(t, adapted.SupportsInPlace())
	assert.True(t, adapted.SupportsCheck())
	assert.True(t, adapted.SupportsConfig())

	// Test Format
	req := &formatters.FormatRequest{
		Content:  "test",
		Language: "test",
	}

	result, err := adapted.Format(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "test // formatted", result.Content)
}

func TestNativeFormatterFactory(t *testing.T) {
	t.Parallel()
	factory := adapter.NewNativeFormatterFactory()

	// These will fail health checks if the binaries aren't installed,
	// but the factory method should work
	goFmt := factory.CreateGoFormatter()
	assert.NotNil(t, goFmt)
	assert.Equal(t, "gofmt", goFmt.Name())

	pyFmt := factory.CreatePythonFormatter()
	assert.NotNil(t, pyFmt)
	assert.Equal(t, "black", pyFmt.Name())

	jsFmt := factory.CreateJSFormatter()
	assert.NotNil(t, jsFmt)
	assert.Equal(t, "prettier", jsFmt.Name())

	rustFmt := factory.CreateRustFormatter()
	assert.NotNil(t, rustFmt)
	assert.Equal(t, "rustfmt", rustFmt.Name())

	sqlFmt := factory.CreateSQLFormatter()
	assert.NotNil(t, sqlFmt)
	assert.Equal(t, "sqlformat", sqlFmt.Name())
}
