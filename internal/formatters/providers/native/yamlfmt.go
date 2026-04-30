package native

import (
	"dev.helix.agent/internal/formatters"
	"github.com/sirupsen/logrus"
)

// NewYamlfmtFormatter creates a yamlfmt YAML formatter
func NewYamlfmtFormatter(logger *logrus.Logger) *NativeFormatter {
	metadata := &formatters.FormatterMetadata{
		Name:            "yamlfmt",
		Type:            formatters.FormatterTypeNative,
		Architecture:    "binary",
		GitHubURL:       "https://github.com/google/yamlfmt",
		Version:         "0.14.0",
		Languages:       []string{"yaml", "yml"},
		License:         "Apache 2.0",
		InstallMethod:   "binary",
		BinaryPath:      "yamlfmt",
		ConfigFormat:    "yaml",
		Performance:     "fast",
		Complexity:      "easy",
		SupportsStdin:   true,
		SupportsInPlace: true,
		SupportsCheck:   false,
		SupportsConfig:  true,
	}

	// yamlfmt's `-in` flag activates stdin mode. Adding the conventional
	// trailing `-` (via stdinFlag=true) makes yamlfmt try to read a file
	// named `-` AND produce noisy "Failed reading file: stat -" output
	// alongside legitimate stdin processing. Use the no-dash variant so
	// the invocation is just `yamlfmt -in`. CONST-035 §c regression-fix
	// (2026-04-30) — same pattern as gofmt/rustfmt fixes.
	return NewNativeFormatterStdinNoDash(metadata, "yamlfmt", []string{"-in"}, logger)
}
