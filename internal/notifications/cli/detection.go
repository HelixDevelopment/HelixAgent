package cli

import (
	"fmt"
	"os"
	"strings"
)

// DetectCLIClient attempts to detect the CLI client from environment
func DetectCLIClient() CLIClient {
	// Check for specific environment variables set by CLI tools

	// OpenCode detection
	if os.Getenv("OPENCODE") != "" || os.Getenv("OPENCODE_VERSION") != "" {
		return CLIClientOpenCode
	}

	// Crush detection
	if os.Getenv("CRUSH_CLI") != "" || os.Getenv("CRUSH_VERSION") != "" {
		return CLIClientCrush
	}

	// HelixCode detection
	if os.Getenv("HELIXCODE") != "" || os.Getenv("HELIXCODE_VERSION") != "" {
		return CLIClientHelixCode
	}

	// KiloCode detection
	if os.Getenv("KILOCODE") != "" || os.Getenv("KILOCODE_VERSION") != "" {
		return CLIClientKiloCode
	}

	// Check for Claude Code / Anthropic CLI
	if os.Getenv("CLAUDE_CODE") != "" || os.Getenv("ANTHROPIC_CLI") != "" {
		return CLIClientOpenCode // Treat Claude Code like OpenCode
	}

	// Check terminal type
	term := os.Getenv("TERM")
	if strings.Contains(term, "xterm") || strings.Contains(term, "256color") {
		// Likely a modern terminal, default to OpenCode-compatible output
		return CLIClientOpenCode
	}

	// Check user agent from request headers (if available via context)
	userAgent := os.Getenv("HTTP_USER_AGENT")
	if userAgent != "" {
		return detectFromUserAgent(userAgent)
	}

	return CLIClientUnknown
}

// detectFromUserAgent parses the user agent string
func detectFromUserAgent(userAgent string) CLIClient {
	ua := strings.ToLower(userAgent)

	if strings.Contains(ua, "opencode") {
		return CLIClientOpenCode
	}
	if strings.Contains(ua, "crush") {
		return CLIClientCrush
	}
	if strings.Contains(ua, "helixcode") {
		return CLIClientHelixCode
	}
	if strings.Contains(ua, "kilocode") {
		return CLIClientKiloCode
	}

	return CLIClientUnknown
}

// CLIClientInfo holds information about a CLI client
type CLIClientInfo struct {
	Client         CLIClient
	Version        string
	SupportsColor  bool
	SupportsUnicode bool
	TerminalWidth  int
	TerminalHeight int
}

// DetectCLIClientInfo detects detailed client information
func DetectCLIClientInfo() *CLIClientInfo {
	info := &CLIClientInfo{
		Client:          DetectCLIClient(),
		SupportsColor:   detectColorSupport(),
		SupportsUnicode: detectUnicodeSupport(),
	}

	// Get terminal size
	info.TerminalWidth, info.TerminalHeight = getTerminalSize()

	// Get version from environment
	switch info.Client {
	case CLIClientOpenCode:
		info.Version = os.Getenv("OPENCODE_VERSION")
	case CLIClientCrush:
		info.Version = os.Getenv("CRUSH_VERSION")
	case CLIClientHelixCode:
		info.Version = os.Getenv("HELIXCODE_VERSION")
	case CLIClientKiloCode:
		info.Version = os.Getenv("KILOCODE_VERSION")
	}

	return info
}

// detectColorSupport checks if terminal supports colors
func detectColorSupport() bool {
	// Check NO_COLOR environment variable (standard)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check FORCE_COLOR
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}

	// Check TERM
	term := os.Getenv("TERM")
	if term == "dumb" {
		return false
	}

	// Check for color support in TERM
	if strings.Contains(term, "color") || strings.Contains(term, "256") || strings.Contains(term, "truecolor") {
		return true
	}

	// Check COLORTERM
	colorTerm := os.Getenv("COLORTERM")
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		return true
	}

	// Default: check if stdout is a TTY
	return isTTY()
}

// detectUnicodeSupport checks if terminal supports Unicode
func detectUnicodeSupport() bool {
	// Check LANG/LC_ALL for UTF-8
	for _, envVar := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		value := os.Getenv(envVar)
		if strings.Contains(strings.ToLower(value), "utf-8") || strings.Contains(strings.ToLower(value), "utf8") {
			return true
		}
	}

	// Check TERM for xterm which usually supports Unicode
	term := os.Getenv("TERM")
	if strings.HasPrefix(term, "xterm") || strings.Contains(term, "256color") {
		return true
	}

	// Check for Windows Terminal (supports Unicode)
	if os.Getenv("WT_SESSION") != "" {
		return true
	}

	return false
}

// getTerminalSize returns the terminal dimensions
func getTerminalSize() (width, height int) {
	// Default values
	width = 80
	height = 24

	// Try to get from environment
	if cols := os.Getenv("COLUMNS"); cols != "" {
		var w int
		if _, err := fmt.Sscanf(cols, "%d", &w); err == nil && w > 0 {
			width = w
		}
	}
	if lines := os.Getenv("LINES"); lines != "" {
		var h int
		if _, err := fmt.Sscanf(lines, "%d", &h); err == nil && h > 0 {
			height = h
		}
	}

	return width, height
}

// GetRenderConfigForClient returns optimal render config for a client
func GetRenderConfigForClient(client CLIClient) *RenderConfig {
	info := DetectCLIClientInfo()
	config := DefaultRenderConfig()

	// Adjust width
	if info.TerminalWidth > 0 {
		config.Width = info.TerminalWidth
	}

	// Adjust color scheme
	if !info.SupportsColor {
		config.ColorScheme = ColorSchemeNone
		config.Style = RenderStylePlain
	}

	// Adjust progress style based on Unicode support
	if !info.SupportsUnicode {
		config.ProgressStyle = ProgressBarStyleASCII
	}

	// Client-specific adjustments
	switch client {
	case CLIClientOpenCode:
		// OpenCode supports rich formatting
		config.Style = RenderStyleTheater
		config.ProgressStyle = ProgressBarStyleUnicode
		config.ShowResources = true
		config.ShowLogs = true

	case CLIClientCrush:
		// Crush prefers compact output
		config.Style = RenderStyleMinimal
		config.ShowResources = false
		config.LogLines = 3

	case CLIClientHelixCode:
		// HelixCode supports full features
		config.Style = RenderStyleTheater
		config.ProgressStyle = ProgressBarStyleUnicode
		config.ShowResources = true
		config.ShowLogs = true

	case CLIClientKiloCode:
		// KiloCode prefers screenplay style
		config.Style = RenderStyleScreenplay
		config.ProgressStyle = ProgressBarStyleBlock

	default:
		// Unknown client - use safe defaults
		if !info.SupportsUnicode {
			config.ProgressStyle = ProgressBarStyleASCII
		}
		if !info.SupportsColor {
			config.Style = RenderStylePlain
		}
	}

	return config
}

// FormatForClient formats content for a specific client
func FormatForClient(client CLIClient, content string) string {
	info := DetectCLIClientInfo()

	// Strip ANSI codes if no color support
	if !info.SupportsColor {
		return stripANSI(content)
	}

	// Convert Unicode to ASCII if needed
	if !info.SupportsUnicode {
		return convertToASCII(content)
	}

	return content
}

// stripANSI removes ANSI escape codes from a string
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false

	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}

	return result.String()
}

// convertToASCII converts Unicode box-drawing characters to ASCII
func convertToASCII(s string) string {
	replacements := map[string]string{
		BoxHorizontal:        "-",
		BoxVertical:          "|",
		BoxTopLeft:           "+",
		BoxTopRight:          "+",
		BoxBottomLeft:        "+",
		BoxBottomRight:       "+",
		BoxTeeLeft:           "+",
		BoxTeeRight:          "+",
		BoxTeeTop:            "+",
		BoxTeeBottom:         "+",
		BoxCross:             "+",
		BoxDoubleHorizontal:  "=",
		BoxDoubleVertical:    "|",
		BoxDoubleTopLeft:     "+",
		BoxDoubleTopRight:    "+",
		BoxDoubleBottomLeft:  "+",
		BoxDoubleBottomRight: "+",
		BoxDoubleTeeLeft:     "+",
		BoxDoubleTeeRight:    "+",
		ProgressFilled:       "#",
		ProgressEmpty:        ".",
		IconPending:          "o",
		IconRunning:          "*",
		IconCompleted:        "+",
		IconFailed:           "x",
		IconStuck:            "!",
		IconCancelled:        "-",
		IconPaused:           "=",
	}

	result := s
	for unicode, ascii := range replacements {
		result = strings.ReplaceAll(result, unicode, ascii)
	}

	return result
}
