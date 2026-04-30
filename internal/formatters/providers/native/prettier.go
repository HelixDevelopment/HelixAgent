package native

import (
	"dev.helix.agent/internal/formatters"
	"github.com/sirupsen/logrus"
)

// NewPrettierFormatter creates a Prettier formatter (JS/TS/HTML/CSS/etc.)
func NewPrettierFormatter(logger *logrus.Logger) *NativeFormatter {
	metadata := &formatters.FormatterMetadata{
		Name:            "prettier",
		Type:            formatters.FormatterTypeUnified,
		Architecture:    "node",
		GitHubURL:       "https://github.com/prettier/prettier",
		Version:         "3.4.2",
		Languages:       []string{"javascript", "typescript", "json", "html", "css", "scss", "markdown", "yaml", "graphql"},
		License:         "MIT",
		InstallMethod:   "npm",
		BinaryPath:      "prettier",
		ConfigFormat:    "json",
		Performance:     "medium",
		Complexity:      "easy",
		SupportsStdin:   true,
		SupportsInPlace: true,
		SupportsCheck:   true,
		SupportsConfig:  true,
	}

	// prettier's `--stdin-filepath temp.js` flag activates stdin mode
	// and supplies a virtual filename for parser inference. Adding the
	// conventional trailing `-` (via stdinFlag=true) gives prettier an
	// extra positional that it interprets as a filename to format
	// alongside stdin. Use the no-dash variant. CONST-035 §c
	// regression-fix (2026-04-30) — same pattern as gofmt/rustfmt/
	// yamlfmt/biome fixes.
	return NewNativeFormatterStdinNoDash(
		metadata,
		"prettier",
		[]string{"--stdin-filepath", "temp.js"},
		logger,
	)
}
