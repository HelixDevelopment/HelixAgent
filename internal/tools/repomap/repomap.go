// Package repomap provides repository mapping capabilities
// Inspired by Aider's repository mapping system
package repomap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// RepoMap represents a mapped repository.
//
// Concurrency model (CONST-029):
//
// RepoMap follows a single-writer, read-after-complete pattern. Map()
// owns the struct for the duration of a mapping call; readers
// (GetFiles, GetSymbols, GetLanguageStats, GetSummary, FindSymbol,
// etc.) must only run after Map() has returned, or concurrently with
// **no other** Map() call. The previous defensive sync.RWMutex was
// flagged by the CONST-029 audit (mutex + multiple bare maps/slices
// in the same struct) but never guarded anything the single-writer
// invariant didn't already cover.
//
// If a caller needs to read state from another goroutine while Map()
// may be running, they should Snapshot() it first (deep-copy) and
// work against the snapshot.
//
// JSON field layout is unchanged — the previous exported fields
// remain in place, so external JSON consumers see the same wire
// format.
type RepoMap struct {
	RootPath     string                  `json:"root_path"`
	Languages    map[string]LanguageInfo `json:"languages"`
	Files        []FileInfo              `json:"files"`
	Symbols      []Symbol                `json:"symbols"`
	Dependencies []Dependency            `json:"dependencies"`

	// Internal
	logger *logrus.Logger
}

// Snapshot returns a deep copy of the RepoMap's mapped data. Call
// Snapshot from the same goroutine that drove Map() (or after Map()
// has returned) to get a value safe to hand to another goroutine.
func (r *RepoMap) Snapshot() *RepoMap {
	if r == nil {
		return nil
	}
	out := &RepoMap{
		RootPath: r.RootPath,
		logger:   r.logger,
	}
	if r.Languages != nil {
		out.Languages = make(map[string]LanguageInfo, len(r.Languages))
		for k, v := range r.Languages {
			out.Languages[k] = v
		}
	}
	if r.Files != nil {
		out.Files = append([]FileInfo(nil), r.Files...)
	}
	if r.Symbols != nil {
		out.Symbols = append([]Symbol(nil), r.Symbols...)
	}
	if r.Dependencies != nil {
		out.Dependencies = append([]Dependency(nil), r.Dependencies...)
	}
	return out
}

// LanguageInfo contains information about a language in the repo
type LanguageInfo struct {
	Name       string  `json:"name"`
	FileCount  int     `json:"file_count"`
	LineCount  int     `json:"line_count"`
	Percentage float64 `json:"percentage"`
}

// FileInfo represents a file in the repository
type FileInfo struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Extension string `json:"extension"`
	Language  string `json:"language"`
	Size      int64  `json:"size"`
	LineCount int    `json:"line_count"`
	IsBinary  bool   `json:"is_binary"`
	IsTest    bool   `json:"is_test"`
}

// Symbol represents a code symbol (function, class, variable, etc.)
type Symbol struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // function, class, type, const, var, interface
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Language   string `json:"language"`
	Signature  string `json:"signature,omitempty"`
	Docstring  string `json:"docstring,omitempty"`
	IsExported bool   `json:"is_exported"`
}

// Dependency represents a dependency relationship
type Dependency struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // import, call, extend, implement
}

// Config configures the repository mapper
type Config struct {
	MaxFileSize    int64    // Max file size to parse (bytes)
	MaxDepth       int      // Max directory depth
	IgnorePatterns []string // Patterns to ignore
	Languages      []string // Languages to include (empty = all)
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		MaxFileSize: 1024 * 1024, // 1MB
		MaxDepth:    20,
		IgnorePatterns: []string{
			"node_modules", "vendor", ".git", "__pycache__",
			".venv", "venv", "target", "build", "dist",
			"*.min.js", "*.min.css", "*.map",
		},
		Languages: []string{}, // All languages
	}
}

// NewRepoMap creates a new repository mapper
func NewRepoMap(rootPath string, logger *logrus.Logger) *RepoMap {
	return &RepoMap{
		RootPath:     rootPath,
		Languages:    make(map[string]LanguageInfo),
		Files:        make([]FileInfo, 0),
		Symbols:      make([]Symbol, 0),
		Dependencies: make([]Dependency, 0),
		logger:       logger,
	}
}

// Map analyzes and maps the repository.
// Must not run concurrently with other Map() calls on the same instance
// or with the Get*/FindSymbol readers.
func (r *RepoMap) Map(ctx context.Context, config Config) error {
	r.logger.Infof("Mapping repository: %s", r.RootPath)

	// Reset state
	r.Files = make([]FileInfo, 0)
	r.Symbols = make([]Symbol, 0)
	r.Dependencies = make([]Dependency, 0)
	r.Languages = make(map[string]LanguageInfo)

	// Walk directory
	err := r.walkDirectory(ctx, r.RootPath, 0, config)
	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	// Calculate language percentages
	r.calculateLanguageStats()

	r.logger.Infof("Mapped %d files, %d symbols", len(r.Files), len(r.Symbols))

	return nil
}

// walkDirectory recursively walks the directory tree
func (r *RepoMap) walkDirectory(ctx context.Context, dir string, depth int, config Config) error {
	if depth > config.MaxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		path := filepath.Join(dir, entry.Name())
		relPath, _ := filepath.Rel(r.RootPath, path)

		// Skip ignored patterns
		if r.shouldIgnore(relPath, config.IgnorePatterns) {
			continue
		}

		if entry.IsDir() {
			// Recurse into subdirectory
			if err := r.walkDirectory(ctx, path, depth+1, config); err != nil {
				r.logger.Warnf("Error walking %s: %v", path, err)
			}
		} else {
			// Process file
			if err := r.processFile(path, config); err != nil {
				r.logger.Warnf("Error processing %s: %v", path, err)
			}
		}
	}

	return nil
}

// shouldIgnore checks if a path should be ignored
func (r *RepoMap) shouldIgnore(path string, patterns []string) bool {
	for _, pattern := range patterns {
		// Simple pattern matching
		if strings.Contains(path, pattern) {
			return true
		}
		// Glob matching
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

// processFile processes a single file
func (r *RepoMap) processFile(path string, config Config) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Skip large files
	if info.Size() > config.MaxFileSize {
		return nil
	}

	// Skip binary files (basic check)
	if isBinary(path) {
		r.Files = append(r.Files, FileInfo{
			Path:     path,
			Name:     filepath.Base(path),
			Size:     info.Size(),
			IsBinary: true,
		})
		return nil
	}

	// Detect language
	ext := filepath.Ext(path)
	language := detectLanguage(ext, path)

	// Filter by language if specified
	if len(config.Languages) > 0 {
		found := false
		for _, lang := range config.Languages {
			if strings.EqualFold(lang, language) {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	// Count lines
	lineCount, err := countLines(path)
	if err != nil {
		lineCount = 0
	}

	// Check if test file
	isTest := isTestFile(path, language)

	fileInfo := FileInfo{
		Path:      path,
		Name:      filepath.Base(path),
		Extension: ext,
		Language:  language,
		Size:      info.Size(),
		LineCount: lineCount,
		IsTest:    isTest,
	}

	r.Files = append(r.Files, fileInfo)

	// Update language stats
	langInfo := r.Languages[language]
	langInfo.Name = language
	langInfo.FileCount++
	langInfo.LineCount += lineCount
	r.Languages[language] = langInfo

	// Extract symbols (basic implementation)
	if language != "" && language != "Unknown" {
		symbols, err := r.extractSymbols(path, language)
		if err == nil {
			r.Symbols = append(r.Symbols, symbols...)
		}
	}

	return nil
}

// calculateLanguageStats calculates language percentages
func (r *RepoMap) calculateLanguageStats() {
	totalLines := 0
	for _, lang := range r.Languages {
		totalLines += lang.LineCount
	}

	for name, lang := range r.Languages {
		if totalLines > 0 {
			lang.Percentage = float64(lang.LineCount) / float64(totalLines) * 100
		}
		r.Languages[name] = lang
	}
}

// extractSymbols extracts symbols from a file (basic regex-based)
func (r *RepoMap) extractSymbols(path, language string) ([]Symbol, error) {
	// This is a basic implementation
	// For production, use tree-sitter or LSP
	switch language {
	case "Go":
		return r.extractGoSymbols(path)
	case "Python":
		return r.extractPythonSymbols(path)
	case "JavaScript", "TypeScript":
		return r.extractJSSymbols(path)
	default:
		return nil, nil
	}
}

// GetFiles returns all mapped files. See the struct docstring for the
// single-writer invariant: call only after Map() has returned, or use
// Snapshot() for cross-goroutine reads.
func (r *RepoMap) GetFiles() []FileInfo {
	return r.Files
}

// GetSymbols returns all extracted symbols. See GetFiles for concurrency.
func (r *RepoMap) GetSymbols() []Symbol {
	return r.Symbols
}

// GetSymbolsByType returns symbols of a specific type. See GetFiles for concurrency.
func (r *RepoMap) GetSymbolsByType(symbolType string) []Symbol {
	var result []Symbol
	for _, sym := range r.Symbols {
		if sym.Type == symbolType {
			result = append(result, sym)
		}
	}
	return result
}

// GetSymbolsInFile returns symbols in a specific file. See GetFiles for concurrency.
func (r *RepoMap) GetSymbolsInFile(file string) []Symbol {
	var result []Symbol
	for _, sym := range r.Symbols {
		if sym.File == file {
			result = append(result, sym)
		}
	}
	return result
}

// FindSymbol searches for a symbol by name. See GetFiles for concurrency.
func (r *RepoMap) FindSymbol(name string) []Symbol {
	var result []Symbol
	for _, sym := range r.Symbols {
		if sym.Name == name {
			result = append(result, sym)
		}
	}
	return result
}

// GetLanguageStats returns language statistics. See GetFiles for concurrency.
func (r *RepoMap) GetLanguageStats() map[string]LanguageInfo {
	return r.Languages
}

// GetSummary returns a summary of the repository. See GetFiles for concurrency.
func (r *RepoMap) GetSummary() map[string]interface{} {
	totalLines := 0
	for _, lang := range r.Languages {
		totalLines += lang.LineCount
	}

	return map[string]interface{}{
		"root_path":      r.RootPath,
		"total_files":    len(r.Files),
		"total_symbols":  len(r.Symbols),
		"total_lines":    totalLines,
		"languages":      len(r.Languages),
		"language_stats": r.Languages,
	}
}

// isBinary checks if a file is binary (basic check)
func isBinary(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := []string{
		".exe", ".dll", ".so", ".dylib", ".bin",
		".png", ".jpg", ".jpeg", ".gif", ".bmp", ".svg",
		".mp3", ".mp4", ".avi", ".mov", ".wav",
		".zip", ".tar", ".gz", ".bz2", ".7z", ".rar",
		".pdf", ".doc", ".docx", ".xls", ".xlsx",
		".db", ".sqlite", ".sqlite3",
	}
	for _, be := range binaryExts {
		if ext == be {
			return true
		}
	}
	return false
}

// detectLanguage detects language from file extension
func detectLanguage(ext, path string) string {
	// Remove leading dot
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	languageMap := map[string]string{
		"go":    "Go",
		"py":    "Python",
		"js":    "JavaScript",
		"ts":    "TypeScript",
		"jsx":   "JavaScript",
		"tsx":   "TypeScript",
		"java":  "Java",
		"kt":    "Kotlin",
		"rs":    "Rust",
		"cpp":   "C++",
		"cc":    "C++",
		"c":     "C",
		"h":     "C/C++ Header",
		"hpp":   "C++",
		"rb":    "Ruby",
		"php":   "PHP",
		"swift": "Swift",
		"scala": "Scala",
		"r":     "R",
		"m":     "Objective-C",
		"cs":    "C#",
		"fs":    "F#",
		"hs":    "Haskell",
		"lua":   "Lua",
		"sh":    "Shell",
		"bash":  "Bash",
		"zsh":   "Zsh",
		"ps1":   "PowerShell",
		"sql":   "SQL",
		"html":  "HTML",
		"css":   "CSS",
		"scss":  "SCSS",
		"less":  "LESS",
		"json":  "JSON",
		"xml":   "XML",
		"yaml":  "YAML",
		"yml":   "YAML",
		"toml":  "TOML",
		"md":    "Markdown",
		"rst":   "reStructuredText",
		"tex":   "LaTeX",
	}

	if lang, ok := languageMap[ext]; ok {
		return lang
	}

	// Check for specific filenames
	base := filepath.Base(path)
	specialFiles := map[string]string{
		"Dockerfile":       "Dockerfile",
		"Makefile":         "Makefile",
		"CMakeLists.txt":   "CMake",
		"Cargo.toml":       "Rust",
		"go.mod":           "Go",
		"package.json":     "JavaScript",
		"requirements.txt": "Python",
		"setup.py":         "Python",
		"Gemfile":          "Ruby",
	}

	if lang, ok := specialFiles[base]; ok {
		return lang
	}

	return "Unknown"
}

// isTestFile checks if a file is a test file
func isTestFile(path, language string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	switch language {
	case "Go":
		return strings.HasSuffix(nameWithoutExt, "_test")
	case "Python":
		return strings.HasPrefix(nameWithoutExt, "test_") ||
			strings.HasSuffix(nameWithoutExt, "_test")
	case "JavaScript", "TypeScript":
		return strings.Contains(nameWithoutExt, ".test") ||
			strings.Contains(nameWithoutExt, ".spec") ||
			strings.HasSuffix(nameWithoutExt, "_test") ||
			strings.HasSuffix(nameWithoutExt, ".spec")
	case "Java", "Kotlin":
		return strings.HasSuffix(nameWithoutExt, "Test") ||
			strings.HasPrefix(nameWithoutExt, "Test")
	case "Rust":
		return strings.HasSuffix(nameWithoutExt, "_test") ||
			strings.HasSuffix(nameWithoutExt, "_tests")
	}

	return false
}

// countLines counts lines in a file
func countLines(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, b := range content {
		if b == '\n' {
			count++
		}
	}
	if len(content) > 0 && content[len(content)-1] != '\n' {
		count++
	}

	return count, nil
}
