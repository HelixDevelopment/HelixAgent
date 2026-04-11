// Package aider provides Aider CLI agent integration for HelixAgent.
package aider

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DiffFormat implements SEARCH/REPLACE block editing.
// Ported from Aider's editblock_coder.py
type DiffFormat struct {
	// Configuration
	FENCE string // Fence marker for code blocks
}

// NewDiffFormat creates a new DiffFormat.
func NewDiffFormat() *DiffFormat {
	return &DiffFormat{
		FENCE: "```",
	}
}

// EditBlock represents a single SEARCH/REPLACE block.
type EditBlock struct {
	File    string
	Search  string
	Replace string
}

// ParseResult contains parsing results.
type ParseResult struct {
	Blocks []*EditBlock
	Errors []error
}

// ValidationResult contains validation results.
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// ValidationError represents a validation error.
type ValidationError struct {
	BlockIndex int
	File       string
	Message    string
}

// ParseEditBlocks parses SEARCH/REPLACE blocks from text.
func (df *DiffFormat) ParseEditBlocks(text string) (*ParseResult, error) {
	result := &ParseResult{
		Blocks: make([]*EditBlock, 0),
		Errors: make([]error, 0),
	}

	// Pattern for file path
	filePattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(df.FENCE) + `\s*(\S+)$`)

	// Pattern for SEARCH/REPLACE blocks
	searchReplacePattern := regexp.MustCompile(
		`(?s)` +
			`^<<<<<<< SEARCH\n` +
			`(.*?)\n` +
			`=======\n` +
			`(.*?)\n` +
			`>>>>>>> REPLACE$`,
	)

	// Find all file markers
	fileMatches := filePattern.FindAllStringSubmatchIndex(text, -1)

	if len(fileMatches) == 0 {
		// Try without file markers - assume single file context
		return df.parseBlocksWithoutFiles(text)
	}

	// Parse blocks for each file
	for i, match := range fileMatches {
		fileStart := match[2]
		fileEnd := match[3]
		file := strings.TrimSpace(text[fileStart:fileEnd])

		// Determine the section for this file
		sectionStart := match[1]
		var sectionEnd int
		if i < len(fileMatches)-1 {
			sectionEnd = fileMatches[i+1][0]
		} else {
			sectionEnd = len(text)
		}

		section := text[sectionStart:sectionEnd]

		// Find SEARCH/REPLACE blocks in this section
		blocks := searchReplacePattern.FindAllStringSubmatch(section, -1)
		for _, block := range blocks {
			if len(block) >= 3 {
				result.Blocks = append(result.Blocks, &EditBlock{
					File:    file,
					Search:  block[1],
					Replace: block[2],
				})
			}
		}
	}

	return result, nil
}

// parseBlocksWithoutFiles parses blocks without explicit file markers.
func (df *DiffFormat) parseBlocksWithoutFiles(text string) (*ParseResult, error) {
	result := &ParseResult{
		Blocks: make([]*EditBlock, 0),
		Errors: make([]error, 0),
	}

	// Pattern for SEARCH/REPLACE blocks
	searchReplacePattern := regexp.MustCompile(
		`(?s)` +
			`^<<<<<<< SEARCH\n` +
			`(.*?)\n` +
			`=======\n` +
			`(.*?)\n` +
			`>>>>>>> REPLACE$`,
	)

	blocks := searchReplacePattern.FindAllStringSubmatch(text, -1)
	for _, block := range blocks {
		if len(block) >= 3 {
			result.Blocks = append(result.Blocks, &EditBlock{
				File:    "", // Will be filled in later
				Search:  block[1],
				Replace: block[2],
			})
		}
	}

	return result, nil
}

// ValidateEditBlocks validates edit blocks against actual file content.
func (df *DiffFormat) ValidateEditBlocks(blocks []*EditBlock, baseDir string) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: make([]ValidationError, 0),
	}

	for i, block := range blocks {
		if block.File == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				BlockIndex: i,
				Message:    "missing file path",
			})
			continue
		}

		// G703 containment: validate the block path stays under baseDir
		// before any filesystem access.
		filePath := filepath.Clean(filepath.Join(baseDir, block.File))
		cleanedBase := filepath.Clean(baseDir)
		rel, relErr := filepath.Rel(cleanedBase, filePath)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				BlockIndex: i,
				File:       block.File,
				Message:    "path escapes base directory",
			})
			continue
		}

		// Check file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				BlockIndex: i,
				File:       block.File,
				Message:    "file does not exist",
			})
			continue
		}

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				BlockIndex: i,
				File:       block.File,
				Message:    fmt.Sprintf("cannot read file: %v", err),
			})
			continue
		}

		// Check search text exists
		if !strings.Contains(string(content), block.Search) {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				BlockIndex: i,
				File:       block.File,
				Message:    "search text not found in file",
			})
		}
	}

	return result
}

// ApplyEditBlocks applies edit blocks to files.
func (df *DiffFormat) ApplyEditBlocks(
	blocks []*EditBlock,
	baseDir string,
	createIfMissing bool,
) (*ApplyResult, error) {
	result := &ApplyResult{
		Applied:     make([]*AppliedBlock, 0),
		Failed:      make([]*FailedBlock, 0),
		FilesEdited: make(map[string]bool),
	}

	// Group blocks by file
	byFile := make(map[string][]*EditBlock)
	for _, block := range blocks {
		byFile[block.File] = append(byFile[block.File], block)
	}

	// Apply blocks file by file
	for file, fileBlocks := range byFile {
		// G703 containment: edit-block file paths come from LLM-generated
		// diffs. Clean and validate that the resolved target stays under
		// baseDir before any filesystem side effect.
		filePath := filepath.Clean(filepath.Join(baseDir, file))
		cleanedBase := filepath.Clean(baseDir)
		rel, relErr := filepath.Rel(cleanedBase, filePath)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			for _, block := range fileBlocks {
				result.Failed = append(result.Failed, &FailedBlock{
					Block: block,
					Error: fmt.Errorf("path %q escapes base directory", file),
				})
			}
			continue
		}

		// Read or create file
		var content []byte
		var err error

		if _, statErr := os.Stat(filePath); os.IsNotExist(statErr) {
			if createIfMissing {
				// Create parent directories
				dir := filepath.Dir(filePath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					for _, block := range fileBlocks {
						result.Failed = append(result.Failed, &FailedBlock{
							Block: block,
							Error: fmt.Errorf("create directory: %w", err),
						})
					}
					continue
				}
				content = []byte{}
			} else {
				for _, block := range fileBlocks {
					result.Failed = append(result.Failed, &FailedBlock{
						Block: block,
						Error: fmt.Errorf("file does not exist"),
					})
				}
				continue
			}
		} else {
			content, err = os.ReadFile(filePath)
			if err != nil {
				for _, block := range fileBlocks {
					result.Failed = append(result.Failed, &FailedBlock{
						Block: block,
						Error: fmt.Errorf("read file: %w", err),
					})
				}
				continue
			}
		}

		// Apply blocks sequentially
		contentStr := string(content)

		for _, block := range fileBlocks {
			if !strings.Contains(contentStr, block.Search) {
				result.Failed = append(result.Failed, &FailedBlock{
					Block: block,
					Error: fmt.Errorf("search text not found"),
				})
				continue
			}

			// Apply replacement (first occurrence only)
			newContent := strings.Replace(contentStr, block.Search, block.Replace, 1)
			if newContent == contentStr {
				result.Failed = append(result.Failed, &FailedBlock{
					Block: block,
					Error: fmt.Errorf("replacement did not change content"),
				})
				continue
			}

			contentStr = newContent
			result.Applied = append(result.Applied, &AppliedBlock{
				Block:    block,
				OldLines: strings.Count(block.Search, "\n"),
				NewLines: strings.Count(block.Replace, "\n"),
			})
		}

		// Write back if any blocks applied
		if len(result.Applied) > 0 {
			if err := os.WriteFile(filePath, []byte(contentStr), 0644); err != nil {
				for _, block := range fileBlocks {
					result.Failed = append(result.Failed, &FailedBlock{
						Block: block,
						Error: fmt.Errorf("write file: %w", err),
					})
				}
				continue
			}

			result.FilesEdited[file] = true
		}
	}

	return result, nil
}

// ApplyResult contains apply results.
type ApplyResult struct {
	Applied     []*AppliedBlock
	Failed      []*FailedBlock
	FilesEdited map[string]bool
}

// AppliedBlock represents a successfully applied block.
type AppliedBlock struct {
	Block    *EditBlock
	OldLines int
	NewLines int
}

// FailedBlock represents a failed block.
type FailedBlock struct {
	Block *EditBlock
	Error error
}

// FormatEditBlock formats an edit block as text.
func (df *DiffFormat) FormatEditBlock(file, search, replace string) string {
	var builder strings.Builder

	if file != "" {
		builder.WriteString(df.FENCE)
		builder.WriteString(file)
		builder.WriteString("\n")
	}

	builder.WriteString("<<<<<<< SEARCH\n")
	builder.WriteString(search)
	builder.WriteString("\n=======\n")
	builder.WriteString(replace)
	builder.WriteString("\n>>>>>>> REPLACE\n")

	if file != "" {
		builder.WriteString(df.FENCE)
		builder.WriteString("\n")
	}

	return builder.String()
}

// FindBestMatch finds the best fuzzy match for search text in content.
func (df *DiffFormat) FindBestMatch(content, search string) (string, float64) {
	// This implements fuzzy matching similar to Aider
	// Returns the best matching substring and similarity score

	// Normalize whitespace
	content = normalizeWhitespace(content)
	search = normalizeWhitespace(search)

	// If exact match exists, return it
	if strings.Contains(content, search) {
		return search, 1.0
	}

	// Try line-by-line fuzzy matching
	contentLines := strings.Split(content, "\n")
	searchLines := strings.Split(search, "\n")

	if len(searchLines) == 0 {
		return "", 0.0
	}

	bestMatch := ""
	bestScore := 0.0

	// Sliding window over content
	for i := 0; i <= len(contentLines)-len(searchLines); i++ {
		window := strings.Join(contentLines[i:i+len(searchLines)], "\n")
		score := similarityScore(window, search)

		if score > bestScore {
			bestScore = score
			bestMatch = window
		}
	}

	return bestMatch, bestScore
}

// similarityScore calculates similarity between two strings (0-1).
func similarityScore(s1, s2 string) float64 {
	// Levenshtein distance-based similarity
	distance := levenshteinDistance(s1, s2)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - float64(distance)/float64(maxLen)
}

// levenshteinDistance calculates edit distance.
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	// Initialize
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			deletion := matrix[i-1][j] + 1
			insertion := matrix[i][j-1] + 1
			substitution := matrix[i-1][j-1] + cost

			matrix[i][j] = min(deletion, min(insertion, substitution))
		}
	}

	return matrix[len(s1)][len(s2)]
}

func normalizeWhitespace(s string) string {
	// Normalize leading/trailing whitespace
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
