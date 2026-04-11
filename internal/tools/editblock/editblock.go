// Package editblock provides Aider-style edit block functionality
// Edit blocks are a reliable way to apply code changes using search/replace
package editblock

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditBlock represents a single search/replace operation
type EditBlock struct {
	FilePath string `json:"file_path"`
	Search   string `json:"search"`
	Replace  string `json:"replace"`
	LineNum  int    `json:"line_num,omitempty"` // Optional hint for where to search
}

// Result represents the result of applying an edit block
type Result struct {
	Success     bool   `json:"success"`
	FilePath    string `json:"file_path"`
	LineStart   int    `json:"line_start,omitempty"`
	LineEnd     int    `json:"line_end,omitempty"`
	Original    string `json:"original,omitempty"`
	New         string `json:"new,omitempty"`
	Error       string `json:"error,omitempty"`
	ChangesMade int    `json:"changes_made"`
}

// Apply applies an edit block to a file
func (e *EditBlock) Apply(basePath string) (*Result, error) {
	if e.FilePath == "" {
		return &Result{Success: false, Error: "file path is required"}, nil
	}

	fullPath := filepath.Join(basePath, e.FilePath)
	fullPath = filepath.Clean(fullPath)

	// Security: ensure path is within basePath
	if !strings.HasPrefix(fullPath, filepath.Clean(basePath)) {
		return &Result{Success: false, Error: "path escapes base directory"}, nil
	}

	// Read file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Result{Success: false, Error: fmt.Sprintf("file not found: %s", e.FilePath)}, nil
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	original := string(content)

	// Find and replace
	newContent, lineStart, lineEnd, found := findAndReplace(original, e.Search, e.Replace, e.LineNum)
	if !found {
		return &Result{
			Success:  false,
			FilePath: e.FilePath,
			Error:    "search pattern not found",
		}, nil
	}

	// Write back.
	// #nosec G703 -- fullPath has been validated above (lines 42-45) to
	// live under basePath via filepath.Clean + prefix check; any escape
	// returns early before reaching here.
	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	return &Result{
		Success:     true,
		FilePath:    e.FilePath,
		LineStart:   lineStart,
		LineEnd:     lineEnd,
		Original:    truncate(e.Search, 100),
		New:         truncate(e.Replace, 100),
		ChangesMade: 1,
	}, nil
}

// ApplyMultiple applies multiple edit blocks
func ApplyMultiple(basePath string, blocks []EditBlock) ([]Result, error) {
	results := make([]Result, 0, len(blocks))

	for _, block := range blocks {
		result, err := block.Apply(basePath)
		if err != nil {
			return results, err
		}
		results = append(results, *result)
	}

	return results, nil
}

// findAndReplace searches for the pattern and replaces it
func findAndReplace(content, search, replace string, hintLine int) (string, int, int, bool) {
	content = normalizeLineEndings(content)
	search = normalizeLineEndings(search)
	replace = normalizeLineEndings(replace)

	// Try exact match first
	if idx := strings.Index(content, search); idx != -1 {
		lineStart := strings.Count(content[:idx], "\n") + 1
		lineEnd := lineStart + strings.Count(search, "\n")
		newContent := content[:idx] + replace + content[idx+len(search):]
		return newContent, lineStart, lineEnd, true
	}

	// Try with flexible whitespace (leading/trailing whitespace per line)
	lines := strings.Split(content, "\n")
	searchLines := strings.Split(search, "\n")

	if len(searchLines) == 0 {
		return content, 0, 0, false
	}

	// Search for matching block
	startIdx := 0
	if hintLine > 0 && hintLine <= len(lines) {
		startIdx = hintLine - 1
	}

	for i := startIdx; i <= len(lines)-len(searchLines); i++ {
		if matchBlock(lines[i:], searchLines) {
			endIdx := i + len(searchLines)
			before := strings.Join(lines[:i], "\n")
			after := strings.Join(lines[endIdx:], "\n")

			var newContent string
			if i > 0 {
				newContent = before + "\n" + replace
			} else {
				newContent = replace
			}
			if endIdx < len(lines) {
				if len(replace) > 0 && !strings.HasSuffix(replace, "\n") {
					newContent += "\n"
				}
				newContent += after
			}

			return newContent, i + 1, endIdx, true
		}
	}

	return content, 0, 0, false
}

func matchBlock(lines, searchLines []string) bool {
	if len(lines) < len(searchLines) {
		return false
	}

	for i, searchLine := range searchLines {
		line := lines[i]
		if normalizeWhitespace(line) != normalizeWhitespace(searchLine) {
			return false
		}
	}

	return true
}

func normalizeWhitespace(s string) string {
	return strings.TrimSpace(s)
}

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FormatEditBlock formats an edit block as Aider-style text
func FormatEditBlock(filePath, search, replace string) string {
	return fmt.Sprintf(`<<<<<<< SEARCH
%s
=======
%s
>>>>>>> REPLACE
`, search, replace)
}

// ParseEditBlocks parses Aider-style edit blocks from text
func ParseEditBlocks(text string) []EditBlock {
	var blocks []EditBlock

	lines := strings.Split(text, "\n")
	i := 0

	for i < len(lines) {
		if !strings.Contains(lines[i], "<<<<<<< SEARCH") {
			i++
			continue
		}

		i++
		var searchLines []string
		for i < len(lines) && !strings.Contains(lines[i], "=======") {
			searchLines = append(searchLines, lines[i])
			i++
		}

		if i >= len(lines) {
			break
		}

		i++

		var replaceLines []string
		for i < len(lines) && !strings.Contains(lines[i], ">>>>>>> REPLACE") {
			replaceLines = append(replaceLines, lines[i])
			i++
		}

		if i < len(lines) {
			search := strings.Join(searchLines, "\n")
			replace := strings.Join(replaceLines, "\n")
			search = strings.TrimSuffix(search, "\n")
			replace = strings.TrimSuffix(replace, "\n")

			blocks = append(blocks, EditBlock{
				Search:  search,
				Replace: replace,
			})
		}

		i++
	}

	return blocks
}

// Validate checks if an edit block is valid
func (e *EditBlock) Validate() error {
	if e.FilePath == "" {
		return fmt.Errorf("file path is required")
	}
	if e.Search == "" && e.Replace == "" {
		return fmt.Errorf("search and replace cannot both be empty")
	}
	return nil
}

// Diff generates a unified diff preview of the edit
func (e *EditBlock) Diff(basePath string) (string, error) {
	fullPath := filepath.Join(basePath, e.FilePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	original := string(content)
	_, lineStart, lineEnd, found := findAndReplace(original, e.Search, e.Replace, e.LineNum)
	if !found {
		return "", fmt.Errorf("search pattern not found")
	}

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- %s\n", e.FilePath))
	diff.WriteString(fmt.Sprintf("+++ %s\n", e.FilePath))
	diff.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
		lineStart, lineEnd-lineStart+1,
		lineStart, strings.Count(e.Replace, "\n")+1))

	originalLines := strings.Split(original, "\n")
	for i := lineStart - 1; i < lineEnd && i < len(originalLines); i++ {
		diff.WriteString(fmt.Sprintf("-%s\n", originalLines[i]))
	}

	replaceLines := strings.Split(e.Replace, "\n")
	for _, line := range replaceLines {
		diff.WriteString(fmt.Sprintf("+%s\n", line))
	}

	return diff.String(), nil
}

// ReadFileLines reads a file and returns its lines
func ReadFileLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

// WriteFileLines writes lines to a file
func WriteFileLines(filePath string, lines []string) error {
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}
	return os.WriteFile(filePath, []byte(content), 0644)
}
