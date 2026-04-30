package native

import (
	"dev.helix.agent/internal/formatters"
	"github.com/sirupsen/logrus"
)

// NewBiomeFormatter creates a Biome formatter (35x faster than Prettier)
func NewBiomeFormatter(logger *logrus.Logger) *NativeFormatter {
	metadata := &formatters.FormatterMetadata{
		Name:            "biome",
		Type:            formatters.FormatterTypeNative,
		Architecture:    "binary",
		GitHubURL:       "https://github.com/biomejs/biome",
		Version:         "1.9.4",
		Languages:       []string{"javascript", "typescript", "json", "jsx", "tsx"},
		License:         "MIT",
		InstallMethod:   "npm",
		BinaryPath:      "biome",
		ConfigFormat:    "json",
		Performance:     "very_fast",
		Complexity:      "easy",
		SupportsStdin:   true,
		SupportsInPlace: true,
		SupportsCheck:   true,
		SupportsConfig:  true,
	}

	// biome's `--stdin-file-path=temp.js` flag activates stdin mode and
	// supplies a virtual filename for syntax detection. Adding the
	// conventional trailing `-` (via stdinFlag=true) gives biome an
	// unrecognized positional arg. Use the no-dash variant. CONST-035
	// §c regression-fix (2026-04-30) — same pattern as gofmt/rustfmt/
	// yamlfmt fixes.
	return NewNativeFormatterStdinNoDash(metadata, "biome", []string{"format", "--stdin-file-path=temp.js"}, logger)
}
