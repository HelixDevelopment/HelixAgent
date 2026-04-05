package formatters

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	assert.True(t, cfg.Enabled)
	assert.True(t, cfg.AutoFormat)
	assert.True(t, cfg.FormatOnSave)
	assert.True(t, cfg.FormatOnDebate)
	assert.Equal(t, "./formatters", cfg.SubmodulesPath)
	assert.Equal(t, "./bin/formatters", cfg.BinariesPath)
	assert.Equal(t, "./configs/formatters", cfg.ConfigsPath)
	assert.True(t, cfg.ServicesEnabled)
	assert.True(t, cfg.CacheEnabled)
	assert.Equal(t, 3600*time.Second, cfg.CacheTTL)
	assert.Equal(t, 30*time.Second, cfg.DefaultTimeout)
	assert.Equal(t, 10, cfg.MaxConcurrent)
	assert.True(t, cfg.HotReload)
	assert.True(t, cfg.Metrics)
	assert.True(t, cfg.Tracing)
	assert.Equal(t, 88, cfg.DefaultLineLength)
	assert.Equal(t, 4, cfg.DefaultIndentSize)
	assert.False(t, cfg.UseTabs)
}

func TestDefaultConfig_Preferences(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	tests := []struct {
		language  string
		formatter string
	}{
		{"python", "ruff"},
		{"javascript", "biome"},
		{"typescript", "biome"},
		{"rust", "rustfmt"},
		{"go", "gofmt"},
		{"c", "clang-format"},
		{"java", "google-java-format"},
		{"ruby", "rubocop"},
		{"bash", "shfmt"},
	}

	for _, tc := range tests {
		t.Run(tc.language, func(t *testing.T) {
				t.Parallel()
			pref, ok := cfg.Preferences[tc.language]
			assert.True(t, ok, "no preference for %s", tc.language)
			assert.Equal(t, tc.formatter, pref)
		})
	}
}

func TestDefaultConfig_Fallback(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	pythonFallback := cfg.Fallback["python"]
	assert.Contains(t, pythonFallback, "black")
	assert.Contains(t, pythonFallback, "autopep8")

	jsFallback := cfg.Fallback["javascript"]
	assert.Contains(t, jsFallback, "prettier")
}

func TestConfig_Validate_Valid(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_InvalidTimeout(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.DefaultTimeout = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default_timeout must be positive")

	cfg.DefaultTimeout = -1 * time.Second
	err = cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_Validate_InvalidMaxConcurrent(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.MaxConcurrent = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_concurrent must be positive")
}

func TestConfig_Validate_InvalidLineLength(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.DefaultLineLength = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default_line_length must be positive")
}

func TestConfig_Validate_InvalidIndentSize(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.DefaultIndentSize = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default_indent_size must be positive")
}

func TestConfig_GetPreferredFormatter(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	formatter, ok := cfg.GetPreferredFormatter("python")
	assert.True(t, ok)
	assert.Equal(t, "ruff", formatter)

	formatter, ok = cfg.GetPreferredFormatter("unknown_lang")
	assert.False(t, ok)
	assert.Empty(t, formatter)
}

func TestConfig_GetFallbackChain(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	chain := cfg.GetFallbackChain("python")
	assert.NotEmpty(t, chain)
	assert.Contains(t, chain, "black")

	chain = cfg.GetFallbackChain("unknown_lang")
	assert.Nil(t, chain)
}

func TestConfig_GetLanguageConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.LanguageConfigs["python"] = LanguageConfig{
		LineLength: 100,
		IndentSize: 4,
		UseTabs:    false,
	}

	lc, ok := cfg.GetLanguageConfig("python")
	assert.True(t, ok)
	assert.Equal(t, 100, lc.LineLength)
	assert.Equal(t, 4, lc.IndentSize)

	_, ok = cfg.GetLanguageConfig("unknown")
	assert.False(t, ok)
}

func TestConfig_ApplyOverrides(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.Overrides = []OverrideConfig{
		{
			Pattern:    "*.py",
			LineLength: 120,
			Config: map[string]interface{}{
				"extra_key": "extra_value",
			},
		},
	}

	req := &FormatRequest{
		FilePath:   "/path/to/test.py",
		LineLength: 88,
	}

	cfg.ApplyOverrides(req)
	assert.Equal(t, 120, req.LineLength)
	assert.Equal(t, "extra_value", req.Config["extra_key"])
}

func TestConfig_ApplyOverrides_NoMatch(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.Overrides = []OverrideConfig{
		{
			Pattern:    "*.py",
			LineLength: 120,
		},
	}

	req := &FormatRequest{
		FilePath:   "/path/to/test.js",
		LineLength: 88,
	}

	cfg.ApplyOverrides(req)
	assert.Equal(t, 88, req.LineLength)
}

func TestConfig_ApplyOverrides_NilConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.Overrides = []OverrideConfig{
		{
			Pattern: "*.py",
			Config:  map[string]interface{}{"key": "val"},
		},
	}

	req := &FormatRequest{
		FilePath: "/path/to/test.py",
		Config:   nil,
	}

	cfg.ApplyOverrides(req)
	assert.NotNil(t, req.Config)
	assert.Equal(t, "val", req.Config["key"])
}

func TestConfig_ApplyOverrides_ZeroLineLength(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.Overrides = []OverrideConfig{
		{
			Pattern:    "*.py",
			LineLength: 0, // zero means don't override
		},
	}

	req := &FormatRequest{
		FilePath:   "/path/to/test.py",
		LineLength: 88,
	}

	cfg.ApplyOverrides(req)
	assert.Equal(t, 88, req.LineLength)
}

func TestMatchesPattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		filePath string
		pattern  string
		expected bool
	}{
		{"py match", "/foo/test.py", "*.py", true},
		{"py no match", "/foo/test.js", "*.py", false},
		{"wildcard", "/foo/test.go", "test.*", true},
		{"exact", "/foo/Makefile", "Makefile", true},
		{"invalid pattern", "/foo/test.py", "[invalid", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			assert.Equal(t, tc.expected, matchesPattern(tc.filePath, tc.pattern))
		})
	}
}

func TestConfig_ToRegistryConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	rc := cfg.ToRegistryConfig()

	assert.Equal(t, cfg.SubmodulesPath, rc.SubmodulesPath)
	assert.Equal(t, cfg.BinariesPath, rc.BinariesPath)
	assert.Equal(t, cfg.ConfigsPath, rc.ConfigsPath)
	assert.Equal(t, cfg.ServicesEnabled, rc.ServicesEnabled)
	assert.Equal(t, cfg.CacheEnabled, rc.EnableCaching)
	assert.Equal(t, cfg.CacheTTL, rc.CacheTTL)
	assert.Equal(t, cfg.DefaultTimeout, rc.DefaultTimeout)
	assert.Equal(t, cfg.MaxConcurrent, rc.MaxConcurrent)
	assert.Equal(t, cfg.HotReload, rc.EnableHotReload)
	assert.Equal(t, cfg.Metrics, rc.EnableMetrics)
	assert.Equal(t, cfg.Tracing, rc.EnableTracing)
}

func TestConfig_ToExecutorConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	ec := cfg.ToExecutorConfig()

	assert.Equal(t, cfg.DefaultTimeout, ec.DefaultTimeout)
	assert.Equal(t, 3, ec.MaxRetries)
	assert.Equal(t, cfg.CacheEnabled, ec.EnableCache)
	assert.Equal(t, cfg.Metrics, ec.EnableMetrics)
	assert.Equal(t, cfg.Tracing, ec.EnableTracing)
}

func TestConfig_ToCacheConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cc := cfg.ToCacheConfig()

	assert.Equal(t, cfg.CacheTTL, cc.TTL)
	assert.Equal(t, 10000, cc.MaxSize)
	assert.Equal(t, 300*time.Second, cc.CleanupFreq)
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	// Create a temp YAML config
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "formatters.yaml")

	yamlContent := `enabled: true
auto_format: false
default_timeout: 60s
max_concurrent: 5
default_line_length: 100
default_indent_size: 2
use_tabs: true
preferences:
  python: black
  go: gofumpt
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(cfgPath)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.False(t, cfg.AutoFormat)
	assert.Equal(t, 60*time.Second, cfg.DefaultTimeout)
	assert.Equal(t, 5, cfg.MaxConcurrent)
	assert.Equal(t, 100, cfg.DefaultLineLength)
	assert.Equal(t, 2, cfg.DefaultIndentSize)
	assert.True(t, cfg.UseTabs)
	assert.Equal(t, "black", cfg.Preferences["python"])
	assert.Equal(t, "gofumpt", cfg.Preferences["go"])
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "bad.yaml")

	err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(cfgPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestLoadConfig_ResolvesRelativePaths(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `submodules_path: ./formatters
binaries_path: ./bin/formatters
configs_path: ./configs/formatters
services_compose_file: ./docker/compose.yml
default_timeout: 30s
max_concurrent: 10
default_line_length: 88
default_indent_size: 4
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(cfgPath)
	require.NoError(t, err)

	// Paths should be resolved relative to config file directory
	assert.True(t, filepath.IsAbs(cfg.SubmodulesPath))
	assert.True(t, filepath.IsAbs(cfg.BinariesPath))
	assert.True(t, filepath.IsAbs(cfg.ConfigsPath))
	assert.True(t, filepath.IsAbs(cfg.ServicesComposeFile))
}

func TestSaveConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "output.yaml")

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.AutoFormat = false

	err := SaveConfig(cfg, cfgPath)
	require.NoError(t, err)

	// Read back and verify
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "enabled: true")
	assert.Contains(t, string(data), "auto_format: false")
}

func TestSaveConfig_InvalidPath(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	err := SaveConfig(cfg, "/nonexistent/dir/config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write config file")
}

func TestConfig_ResolvePaths_AbsolutePaths(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.SubmodulesPath = "/absolute/path/formatters"
	cfg.BinariesPath = "/absolute/path/bin"
	cfg.ConfigsPath = "/absolute/path/configs"
	cfg.ServicesComposeFile = "/absolute/path/compose.yml"

	err := cfg.resolvePaths("/some/base")
	assert.NoError(t, err)

	// Absolute paths should remain unchanged
	assert.Equal(t, "/absolute/path/formatters", cfg.SubmodulesPath)
	assert.Equal(t, "/absolute/path/bin", cfg.BinariesPath)
	assert.Equal(t, "/absolute/path/configs", cfg.ConfigsPath)
	assert.Equal(t, "/absolute/path/compose.yml", cfg.ServicesComposeFile)
}
