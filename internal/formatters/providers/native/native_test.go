package native

import (
	"context"
	"testing"

	"dev.helix.agent/internal/formatters"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNativeFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:            "test-formatter",
		Type:            formatters.FormatterTypeNative,
		Version:         "1.0.0",
		Languages:       []string{"testlang"},
		SupportsStdin:   true,
		SupportsInPlace: true,
		SupportsCheck:   true,
		SupportsConfig:  true,
	}

	formatter := NewNativeFormatter(metadata, "fake-binary", []string{"--quiet"}, true, logger)
	assert.NotNil(t, formatter)
	assert.Equal(t, "test-formatter", formatter.Name())
	assert.Equal(t, "1.0.0", formatter.Version())
	assert.Contains(t, formatter.Languages(), "testlang")
	assert.True(t, formatter.SupportsStdin())
	assert.True(t, formatter.SupportsInPlace())
	assert.True(t, formatter.SupportsCheck())
	assert.True(t, formatter.SupportsConfig())
}

func TestNativeFormatter_buildArgs(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:      "test",
		Type:      formatters.FormatterTypeNative,
		Languages: []string{"test"},
	}

	testCases := []struct {
		name          string
		args          []string
		stdinFlag     bool
		checkOnly     bool
		supportsCheck bool
		expected      []string
	}{
		{
			name:      "basic args",
			args:      []string{"--quiet"},
			stdinFlag: false,
			expected:  []string{"--quiet"},
		},
		{
			name:      "with stdin flag",
			args:      []string{"--quiet"},
			stdinFlag: true,
			expected:  []string{"--quiet", "-"},
		},
		{
			name:          "check only with support",
			args:          []string{"--quiet"},
			stdinFlag:     false,
			checkOnly:     true,
			supportsCheck: true,
			expected:      []string{"--quiet", "--check"},
		},
		{
			name:          "check only without support",
			args:          []string{"--quiet"},
			stdinFlag:     false,
			checkOnly:     true,
			supportsCheck: false,
			expected:      []string{"--quiet"},
		},
		{
			name:          "stdin and check",
			args:          []string{"--quiet"},
			stdinFlag:     true,
			checkOnly:     true,
			supportsCheck: true,
			expected:      []string{"--quiet", "-", "--check"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			metadata.SupportsCheck = tc.supportsCheck
			formatter := &NativeFormatter{
				BaseFormatter: formatters.NewBaseFormatter(metadata),
				binaryPath:    "fake",
				args:          tc.args,
				stdinFlag:     tc.stdinFlag,
				logger:        logger,
			}

			req := &formatters.FormatRequest{
				CheckOnly: tc.checkOnly,
			}
			args := formatter.buildArgs(req)
			assert.Equal(t, tc.expected, args)
		})
	}
}

func TestNativeFormatter_HealthCheck_BinaryMissing(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:      "test",
		Type:      formatters.FormatterTypeNative,
		Languages: []string{"test"},
	}

	// Use a binary that definitely doesn't exist
	formatter := NewNativeFormatter(metadata, "/nonexistent/binary/that/does/not/exist", nil, false, logger)
	ctx := context.Background()
	err := formatter.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "formatter binary not available")
}

func TestNativeFormatter_FormatBatch(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:      "test",
		Type:      formatters.FormatterTypeNative,
		Languages: []string{"test"},
	}

	formatter := NewNativeFormatter(metadata, "fake-binary", []string{"--quiet"}, true, logger)
	ctx := context.Background()
	reqs := []*formatters.FormatRequest{
		{Content: "content1", Language: "test"},
		{Content: "content2", Language: "test"},
	}

	// Since the binary doesn't exist, Format will fail but return a result with error
	results, err := formatter.FormatBatch(ctx, reqs)
	require.NoError(t, err) // FormatBatch returns nil error, errors are in results
	require.Len(t, results, 2)
	for _, result := range results {
		assert.False(t, result.Success)
		assert.Contains(t, result.Error.Error(), "formatter execution failed")
	}
}

func TestNativeFormatter_Format_MissingBinary(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	metadata := &formatters.FormatterMetadata{
		Name:      "test",
		Type:      formatters.FormatterTypeNative,
		Languages: []string{"test"},
	}

	formatter := NewNativeFormatter(metadata, "/nonexistent/binary/that/does/not/exist", []string{"--quiet"}, true, logger)
	ctx := context.Background()
	req := &formatters.FormatRequest{Content: "test", Language: "test"}
	result, err := formatter.Format(ctx, req)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error.Error(), "formatter execution failed")
}

func TestComputeLineChanges(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		original  string
		formatted string
		expected  int
	}{
		{
			name:      "no changes",
			original:  "line1\nline2\nline3",
			formatted: "line1\nline2\nline3",
			expected:  0,
		},
		{
			name:      "one line changed",
			original:  "line1\nline2\nline3",
			formatted: "line1\nCHANGED\nline3",
			expected:  1,
		},
		{
			name:      "all lines changed",
			original:  "a\nb\nc",
			formatted: "x\ny\nz",
			expected:  3,
		},
		{
			name:      "added line",
			original:  "line1\nline2",
			formatted: "line1\nline2\nline3",
			expected:  1,
		},
		{
			name:      "removed line",
			original:  "line1\nline2\nline3",
			formatted: "line1\nline2",
			expected:  1,
		},
		{
			name:      "empty strings",
			original:  "",
			formatted: "",
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := computeLineChanges(tt.original, tt.formatted)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewGofmtFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewGofmtFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "gofmt", formatter.Name())
	assert.Equal(t, "go1.24.11", formatter.Version())
	assert.Contains(t, formatter.Languages(), "go")
}

func TestNewBlackFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewBlackFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "black", formatter.Name())
	assert.Contains(t, formatter.Languages(), "python")
}

func TestNewRuffFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewRuffFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "ruff", formatter.Name())
	assert.Contains(t, formatter.Languages(), "python")
}

func TestNewPrettierFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewPrettierFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "prettier", formatter.Name())
}

func TestNewRustfmtFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewRustfmtFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "rustfmt", formatter.Name())
	assert.Contains(t, formatter.Languages(), "rust")
}

func TestNewClangFormatFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewClangFormatFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "clang-format", formatter.Name())
}

func TestNewBiomeFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewBiomeFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "biome", formatter.Name())
}

func TestNewShfmtFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewShfmtFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "shfmt", formatter.Name())
	assert.Contains(t, formatter.Languages(), "shell")
}

func TestNewYamlfmtFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewYamlfmtFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "yamlfmt", formatter.Name())
	assert.Contains(t, formatter.Languages(), "yaml")
}

func TestNewTaploFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewTaploFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "taplo", formatter.Name())
	assert.Contains(t, formatter.Languages(), "toml")
}

func TestNewStyluaFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatter := NewStyluaFormatter(logger)

	require.NotNil(t, formatter)
	assert.Equal(t, "stylua", formatter.Name())
	assert.Contains(t, formatter.Languages(), "lua")
}

func TestAllFormatters_UniqueNames(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	formatters := map[string]*NativeFormatter{
		"gofmt":        NewGofmtFormatter(logger),
		"black":        NewBlackFormatter(logger),
		"ruff":         NewRuffFormatter(logger),
		"prettier":     NewPrettierFormatter(logger),
		"rustfmt":      NewRustfmtFormatter(logger),
		"clang-format": NewClangFormatFormatter(logger),
		"biome":        NewBiomeFormatter(logger),
		"shfmt":        NewShfmtFormatter(logger),
		"yamlfmt":      NewYamlfmtFormatter(logger),
		"taplo":        NewTaploFormatter(logger),
		"stylua":       NewStyluaFormatter(logger),
	}

	names := make(map[string]bool)
	for name, f := range formatters {
		assert.False(t, names[f.Name()], "Duplicate formatter name: %s", f.Name())
		names[f.Name()] = true
		assert.Equal(t, name, f.Name())
	}
}

func BenchmarkComputeLineChanges(b *testing.B) {
	original := "line1\nline2\nline3\nline4\nline5"
	formatted := "line1\nchanged\nline3\nline4\nline5"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = computeLineChanges(original, formatted)
	}
}
