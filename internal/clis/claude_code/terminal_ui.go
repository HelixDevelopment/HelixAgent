// Package claude_code provides Claude Code CLI agent integration for HelixAgent.
package claude_code

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/fatih/color"
)

// TerminalUI provides rich terminal output formatting.
// Ported from Claude Code's terminal rendering
type TerminalUI struct {
	formatter chroma.Formatter
	style     *chroma.Style

	// Color functions
	primaryColor *color.Color
	successColor *color.Color
	errorColor   *color.Color
	warningColor *color.Color
	infoColor    *color.Color
	dimColor     *color.Color

	// Width
	terminalWidth int
}

// NewTerminalUI creates a new TerminalUI.
func NewTerminalUI() *TerminalUI {
	return &TerminalUI{
		formatter: formatters.Get("terminal256"),
		style:     styles.Get("dracula"),

		primaryColor: color.New(color.FgCyan, color.Bold),
		successColor: color.New(color.FgGreen),
		errorColor:   color.New(color.FgRed),
		warningColor: color.New(color.FgYellow),
		infoColor:    color.New(color.FgBlue),
		dimColor:     color.New(color.Faint),

		terminalWidth: 80,
	}
}

// RenderCodeBlock renders a code block with syntax highlighting.
func (ui *TerminalUI) RenderCodeBlock(code, language string, lineNumbers bool) string {
	// Get lexer for language
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}

	// Tokenize
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	// Format with syntax highlighting
	var buf strings.Builder
	err = ui.formatter.Format(&buf, ui.style, iterator)
	if err != nil {
		return code
	}

	highlighted := buf.String()

	// Add line numbers if requested
	if lineNumbers {
		highlighted = ui.addLineNumbers(highlighted)
	}

	// Add border
	highlighted = ui.addBorder(highlighted, language)

	return highlighted
}

// RenderDiff renders a diff with color coding.
func (ui *TerminalUI) RenderDiff(oldCode, newCode string) string {
	// Generate unified diff
	diff := ui.generateUnifiedDiff(oldCode, newCode)

	// Colorize
	var result strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			result.WriteString(ui.successColor.Sprintf("%s\n", line))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			result.WriteString(ui.errorColor.Sprintf("%s\n", line))
		case strings.HasPrefix(line, "@@"):
			result.WriteString(ui.infoColor.Sprintf("%s\n", line))
		case strings.HasPrefix(line, "diff "):
			result.WriteString(ui.primaryColor.Sprintf("%s\n", line))
		default:
			result.WriteString(ui.dimColor.Sprintf("%s\n", line))
		}
	}

	return result.String()
}

// RenderProgress renders a progress bar.
func (ui *TerminalUI) RenderProgress(percent int, message string) string {
	width := 40
	filled := int(float64(width) * float64(percent) / 100.0)

	bar := ui.successColor.Sprint(strings.Repeat("█", filled)) +
		ui.dimColor.Sprint(strings.Repeat("░", width-filled))

	return fmt.Sprintf("\r[%s] %3d%% %s", bar, percent, message)
}

// RenderSpinner renders a spinner animation frame.
func (ui *TerminalUI) RenderSpinner(frame int) string {
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return ui.primaryColor.Sprintf("%s ", spinner[frame%len(spinner)])
}

// RenderBox renders content in a box.
func (ui *TerminalUI) RenderBox(content, title string) string {
	lines := strings.Split(content, "\n")
	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	// Ensure minimum width
	if maxWidth < len(title)+4 {
		maxWidth = len(title) + 4
	}

	var result strings.Builder

	// Top border
	if title != "" {
		padding := (maxWidth - len(title)) / 2
		result.WriteString("╔")
		result.WriteString(strings.Repeat("═", padding))
		result.WriteString(ui.primaryColor.Sprintf(" %s ", title))
		result.WriteString(strings.Repeat("═", maxWidth-len(title)-padding-2))
		result.WriteString("╗\n")
	} else {
		result.WriteString("╔")
		result.WriteString(strings.Repeat("═", maxWidth+2))
		result.WriteString("╗\n")
	}

	// Content
	for _, line := range lines {
		padded := fmt.Sprintf("%-*s", maxWidth, line)
		result.WriteString("║ ")
		result.WriteString(padded)
		result.WriteString(" ║\n")
	}

	// Bottom border
	result.WriteString("╚")
	result.WriteString(strings.Repeat("═", maxWidth+2))
	result.WriteString("╝")

	return result.String()
}

// RenderTable renders a simple table.
func (ui *TerminalUI) RenderTable(headers []string, rows [][]string) string {
	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var result strings.Builder

	// Header
	result.WriteString(ui.primaryColor.Sprint("┌"))
	for i, w := range widths {
		result.WriteString(strings.Repeat("─", w+2))
		if i < len(widths)-1 {
			result.WriteString("┬")
		}
	}
	result.WriteString("┐\n")

	// Header row
	result.WriteString("│ ")
	for i, h := range headers {
		result.WriteString(ui.primaryColor.Sprintf("%-*s", widths[i], h))
		result.WriteString(" │ ")
	}
	result.WriteString("\n")

	// Separator
	result.WriteString("├")
	for i, w := range widths {
		result.WriteString(strings.Repeat("─", w+2))
		if i < len(widths)-1 {
			result.WriteString("┼")
		}
	}
	result.WriteString("┤\n")

	// Rows
	for _, row := range rows {
		result.WriteString("│ ")
		for i, cell := range row {
			if i < len(widths) {
				result.WriteString(fmt.Sprintf("%-*s", widths[i], cell))
				result.WriteString(" │ ")
			}
		}
		result.WriteString("\n")
	}

	// Footer
	result.WriteString("└")
	for i, w := range widths {
		result.WriteString(strings.Repeat("─", w+2))
		if i < len(widths)-1 {
			result.WriteString("┴")
		}
	}
	result.WriteString("┘")

	return result.String()
}

// RenderStatus renders a status message.
func (ui *TerminalUI) RenderStatus(status, message string) string {
	switch status {
	case "success":
		return ui.successColor.Sprintf("✓ %s", message)
	case "error":
		return ui.errorColor.Sprintf("✗ %s", message)
	case "warning":
		return ui.warningColor.Sprintf("⚠ %s", message)
	case "info":
		return ui.infoColor.Sprintf("ℹ %s", message)
	default:
		return message
	}
}

// RenderTree renders a tree structure.
func (ui *TerminalUI) RenderTree(items []TreeItem, prefix string) string {
	var result strings.Builder

	for i, item := range items {
		isLast := i == len(items)-1

		// Determine connectors
		var connector string
		var childPrefix string
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		} else {
			connector = "├── "
			childPrefix = prefix + "│   "
		}

		// Render item
		result.WriteString(prefix)
		result.WriteString(connector)

		if item.IsDir {
			result.WriteString(ui.primaryColor.Sprintf("📁 %s", item.Name))
		} else {
			result.WriteString(fmt.Sprintf("📄 %s", item.Name))
		}
		result.WriteString("\n")

		// Render children
		if len(item.Children) > 0 {
			result.WriteString(ui.RenderTree(item.Children, childPrefix))
		}
	}

	return result.String()
}

// TreeItem represents an item in a tree.
type TreeItem struct {
	Name     string
	IsDir    bool
	Children []TreeItem
}

// RenderMarkdown renders markdown content.
func (ui *TerminalUI) RenderMarkdown(content string) string {
	// Simple markdown rendering
	var result strings.Builder
	lines := strings.Split(content, "\n")

	inCodeBlock := false
	codeBlockLang := ""
	var codeBlockLines []string

	for _, line := range lines {
		// Code blocks
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End code block
				code := strings.Join(codeBlockLines, "\n")
				result.WriteString(ui.RenderCodeBlock(code, codeBlockLang, true))
				result.WriteString("\n")
				inCodeBlock = false
				codeBlockLines = nil
			} else {
				// Start code block
				inCodeBlock = true
				codeBlockLang = strings.TrimPrefix(line, "```")
			}
			continue
		}

		if inCodeBlock {
			codeBlockLines = append(codeBlockLines, line)
			continue
		}

		// Headers
		if strings.HasPrefix(line, "# ") {
			result.WriteString(ui.primaryColor.Sprintf("\n%s\n", strings.TrimPrefix(line, "# ")))
			result.WriteString(strings.Repeat("=", ui.terminalWidth) + "\n")
		} else if strings.HasPrefix(line, "## ") {
			result.WriteString(ui.primaryColor.Sprintf("\n%s\n", strings.TrimPrefix(line, "## ")))
			result.WriteString(strings.Repeat("-", ui.terminalWidth) + "\n")
		} else if strings.HasPrefix(line, "### ") {
			result.WriteString(ui.infoColor.Sprintf("\n%s\n", strings.TrimPrefix(line, "### ")))
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			// Bullet points
			result.WriteString(fmt.Sprintf("  • %s\n", strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")))
		} else if strings.HasPrefix(line, "> ") {
			// Blockquote
			result.WriteString(ui.dimColor.Sprintf("  │ %s\n", strings.TrimPrefix(line, "> ")))
		} else {
			// Regular text
			result.WriteString(line + "\n")
		}
	}

	return result.String()
}

// Helper methods

func (ui *TerminalUI) addLineNumbers(code string) string {
	lines := strings.Split(code, "\n")
	var result strings.Builder

	lineNumWidth := len(fmt.Sprintf("%d", len(lines)))

	for i, line := range lines {
		lineNum := ui.dimColor.Sprintf("%*d │ ", lineNumWidth, i+1)
		result.WriteString(lineNum + line + "\n")
	}

	return result.String()
}

func (ui *TerminalUI) addBorder(content, language string) string {
	lines := strings.Split(content, "\n")
	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	var result strings.Builder

	// Header with language
	if language != "" {
		result.WriteString(ui.dimColor.Sprintf("┌─ %s ", language))
		result.WriteString(ui.dimColor.Sprint(strings.Repeat("─", maxWidth-len(language)-4)))
		result.WriteString("┐\n")
	} else {
		result.WriteString(ui.dimColor.Sprintf("┌%s┐\n", strings.Repeat("─", maxWidth+2)))
	}

	// Content
	for _, line := range lines {
		padded := fmt.Sprintf("%-*s", maxWidth, line)
		result.WriteString(ui.dimColor.Sprintf("│ %s │\n", padded))
	}

	// Footer
	result.WriteString(ui.dimColor.Sprintf("└%s┘", strings.Repeat("─", maxWidth+2)))

	return result.String()
}

func (ui *TerminalUI) generateUnifiedDiff(oldCode, newCode string) string {
	// Simplified unified diff generation
	oldLines := strings.Split(oldCode, "\n")
	newLines := strings.Split(newCode, "\n")

	var result strings.Builder
	result.WriteString("--- old\n")
	result.WriteString("+++ new\n")

	// Simple line-by-line comparison
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		if i < len(oldLines) && i < len(newLines) {
			if oldLines[i] != newLines[i] {
				result.WriteString(fmt.Sprintf("-%s\n", oldLines[i]))
				result.WriteString(fmt.Sprintf("+%s\n", newLines[i]))
			}
		} else if i < len(oldLines) {
			result.WriteString(fmt.Sprintf("-%s\n", oldLines[i]))
		} else {
			result.WriteString(fmt.Sprintf("+%s\n", newLines[i]))
		}
	}

	return result.String()
}

// SetWidth sets the terminal width.
func (ui *TerminalUI) SetWidth(width int) {
	ui.terminalWidth = width
}

// Color methods for direct use

func (ui *TerminalUI) Primary(text string) string {
	return ui.primaryColor.Sprint(text)
}

func (ui *TerminalUI) Success(text string) string {
	return ui.successColor.Sprint(text)
}

func (ui *TerminalUI) Error(text string) string {
	return ui.errorColor.Sprint(text)
}

func (ui *TerminalUI) Warning(text string) string {
	return ui.warningColor.Sprint(text)
}

func (ui *TerminalUI) Info(text string) string {
	return ui.infoColor.Sprint(text)
}

func (ui *TerminalUI) Dim(text string) string {
	return ui.dimColor.Sprint(text)
}
