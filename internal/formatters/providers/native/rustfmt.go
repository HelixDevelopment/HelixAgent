package native

import (
	"dev.helix.agent/internal/formatters"
	"github.com/sirupsen/logrus"
)

// NewRustfmtFormatter creates a rustfmt Rust formatter
func NewRustfmtFormatter(logger *logrus.Logger) *NativeFormatter {
	metadata := &formatters.FormatterMetadata{
		Name:            "rustfmt",
		Type:            formatters.FormatterTypeNative,
		Architecture:    "binary",
		GitHubURL:       "https://github.com/rust-lang/rustfmt",
		Version:         "1.8.1",
		Languages:       []string{"rust"},
		License:         "Apache 2.0",
		InstallMethod:   "cargo",
		BinaryPath:      "rustfmt",
		ConfigFormat:    "toml",
		Performance:     "fast",
		Complexity:      "easy",
		SupportsStdin:   true,
		SupportsInPlace: true,
		SupportsCheck:   true,
		SupportsConfig:  true,
	}

	// rustfmt has the same gofmt-style bug: it treats `-` as a literal
	// filename ("file `-` does not exist") rather than "read stdin".
	// rustfmt with NO filename arguments reads stdin; use the no-dash
	// constructor. CONST-035 §c regression-fix (2026-04-30) — same
	// pattern as gofmt fix in commit d82d631c.
	return NewNativeFormatterStdinNoDash(metadata, "rustfmt", []string{"--edition=2024"}, logger)
}
