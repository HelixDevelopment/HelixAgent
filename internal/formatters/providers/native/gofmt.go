package native

import (
	"dev.helix.agent/internal/formatters"
	"github.com/sirupsen/logrus"
)

// NewGofmtFormatter creates a gofmt Go formatter
func NewGofmtFormatter(logger *logrus.Logger) *NativeFormatter {
	metadata := &formatters.FormatterMetadata{
		Name:            "gofmt",
		Type:            formatters.FormatterTypeBuiltin,
		Architecture:    "binary",
		GitHubURL:       "https://github.com/golang/go",
		Version:         "go1.24.11",
		Languages:       []string{"go"},
		License:         "BSD-3-Clause",
		InstallMethod:   "builtin",
		BinaryPath:      "gofmt",
		ConfigFormat:    "none",
		Performance:     "fast",
		Complexity:      "easy",
		SupportsStdin:   true,
		SupportsInPlace: true,
		SupportsCheck:   false,
		SupportsConfig:  false,
	}

	// gofmt reads stdin when given NO filename arguments. It treats `-`
	// as a literal filename (and lstats it, failing with exit 2). Use
	// NewNativeFormatterStdinNoDash instead of NewNativeFormatter so
	// the executor wires stdin without appending `-` to args.
	// CONST-035 §c regression-fix (2026-04-30): every /v1/format Go
	// request returned 200 with success:false because of this.
	return NewNativeFormatterStdinNoDash(
		metadata,
		"gofmt",
		[]string{}, // no args = read stdin
		logger,
	)
}
