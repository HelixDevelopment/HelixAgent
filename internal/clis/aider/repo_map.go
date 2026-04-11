// Package aider provides Aider CLI agent integration for HelixAgent.
package aider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/go-enry/go-enry/v2"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// RepoMap provides AST-based repository understanding.
// Ported from Aider's repo_map.py
type RepoMap struct {
	rootDir   string
	mapTokens int

	// Parsers by language
	parsers map[string]*sitter.Parser

	// Query strings by language
	queries map[string]string

	// Cache
	symbolCache *SymbolCache
	fileCache   *FileCache

	// File filtering
	ignorePatterns []string
}

// Symbol represents a code symbol (function, class, etc.).
type Symbol struct {
	Name       string
	Type       string // "function", "class", "method", "variable", etc.
	File       string
	Line       uint32
	Column     uint32
	Language   string
	Signature  string // Function signature or class definition
	Docstring  string
	References []string // Files that reference this symbol
}

// RankedSymbol is a symbol with a relevance score.
type RankedSymbol struct {
	*Symbol
	Score float64
}

// RepoContext contains the repository context for LLM prompting.
type RepoContext struct {
	Symbols    []*RankedSymbol
	Files      []string
	TokenCount int
	Content    string // Formatted content within token budget
}

// SymbolCache caches symbol information.
type SymbolCache struct {
	data map[string][]*Symbol
	mu   sync.RWMutex
}

// FileCache caches file content.
type FileCache struct {
	data map[string]*FileInfo
	mu   sync.RWMutex
}

// FileInfo contains cached file information.
type FileInfo struct {
	Content  []byte
	Modified int64
	Language string
}

// NewRepoMap creates a new RepoMap.
func NewRepoMap(rootDir string, mapTokens int) *RepoMap {
	rm := &RepoMap{
		rootDir:     rootDir,
		mapTokens:   mapTokens,
		parsers:     make(map[string]*sitter.Parser),
		queries:     make(map[string]string),
		symbolCache: &SymbolCache{data: make(map[string][]*Symbol)},
		fileCache:   &FileCache{data: make(map[string]*FileInfo)},
		ignorePatterns: []string{
			".git", "node_modules", "vendor", "__pycache__",
			".venv", "venv", "*.min.js", "*.min.css",
		},
	}

	// Initialize parsers
	rm.initParsers()

	return rm
}

// GetRankedTags returns symbols ranked by relevance to the query.
func (rm *RepoMap) GetRankedTags(
	ctx context.Context,
	query string,
	mentionedFiles []string,
) (*RepoContext, error) {
	// 1. Find all relevant files
	files, err := rm.findMatchingFiles(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find matching files: %w", err)
	}

	// 2. Extract symbols from files
	allSymbols := make([]*Symbol, 0)
	for _, file := range files {
		symbols, err := rm.extractSymbols(ctx, file)
		if err != nil {
			continue
		}
		allSymbols = append(allSymbols, symbols...)
	}

	// 3. Build reference graph
	graph := rm.buildReferenceGraph(allSymbols)

	// 4. Rank symbols
	ranked := rm.rankSymbols(allSymbols, graph, query, mentionedFiles)

	// 5. Format for LLM within token budget
	content := rm.formatForLLM(ranked, rm.mapTokens)

	return &RepoContext{
		Symbols:    ranked,
		Files:      files,
		TokenCount: rm.estimateTokens(content),
		Content:    content,
	}, nil
}

// findMatchingFiles finds files relevant to the query.
func (rm *RepoMap) findMatchingFiles(ctx context.Context, query string) ([]string, error) {
	var matches []string

	// Strategy 1: Direct file mention in query
	allFiles := rm.listAllFiles()
	for _, file := range allFiles {
		base := filepath.Base(file)
		if strings.Contains(query, base) {
			matches = append(matches, file)
		}
	}

	// Strategy 2: Fuzzy filename matching
	queryLower := strings.ToLower(query)
	for _, file := range allFiles {
		base := strings.ToLower(filepath.Base(file))
		// Simple fuzzy match - check if query words appear in filename
		score := fuzzyScore(base, queryLower)
		if score > 0.5 {
			matches = append(matches, file)
		}
	}

	// Strategy 3: Recent git changes (if available)
	recentFiles := rm.getRecentlyChangedFiles()
	matches = append(matches, recentFiles...)

	// Deduplicate and limit
	matches = uniqueStrings(matches)
	if len(matches) > 100 {
		matches = matches[:100]
	}

	return matches, nil
}

// extractSymbols extracts symbols from a file.
func (rm *RepoMap) extractSymbols(ctx context.Context, file string) ([]*Symbol, error) {
	// Check cache
	if cached := rm.symbolCache.Get(file); cached != nil {
		return cached, nil
	}

	// Read file
	content, err := rm.readFile(file)
	if err != nil {
		return nil, err
	}

	// Detect language
	lang := rm.detectLanguage(file, content)
	if lang == "" {
		return nil, fmt.Errorf("unsupported language")
	}

	// Get parser
	parser, ok := rm.parsers[lang]
	if !ok {
		return nil, fmt.Errorf("no parser for language: %s", lang)
	}

	// Parse
	tree := parser.Parse(nil, content)
	if tree == nil {
		return nil, fmt.Errorf("parse failed")
	}
	defer tree.Close()

	// Query for definitions
	queryStr := rm.queries[lang]
	if queryStr == "" {
		return nil, fmt.Errorf("no query for language: %s", lang)
	}

	query, err := sitter.NewQuery([]byte(queryStr), rm.getLanguage(lang))
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer query.Close()

	// Execute query
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, tree.RootNode())

	symbols := make([]*Symbol, 0)

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		for _, capture := range match.Captures {
			node := capture.Node
			captureName := query.CaptureNameForId(capture.Index)

			symbol := &Symbol{
				Name:     node.Content(content),
				Type:     rm.mapCaptureType(captureName),
				File:     file,
				Line:     node.StartPoint().Row + 1,
				Column:   node.StartPoint().Column,
				Language: lang,
			}

			// Extract signature/docstring
			symbol.Signature = rm.extractSignature(node, content)
			symbol.Docstring = rm.extractDocstring(node, content)

			symbols = append(symbols, symbol)
		}
	}

	// Cache results
	rm.symbolCache.Set(file, symbols)

	return symbols, nil
}

// rankSymbols ranks symbols by relevance.
func (rm *RepoMap) rankSymbols(
	symbols []*Symbol,
	graph *ReferenceGraph,
	query string,
	mentionedFiles []string,
) []*RankedSymbol {
	ranked := make([]*RankedSymbol, 0, len(symbols))

	queryLower := strings.ToLower(query)

	for _, sym := range symbols {
		score := 0.0

		// Factor 1: Distance from mentioned files
		if len(mentionedFiles) > 0 {
			minDist := graph.MinDistance(sym.File, mentionedFiles)
			score += 1.0 / (1.0 + float64(minDist))
		}

		// Factor 2: Symbol type weight
		typeWeights := map[string]float64{
			"class":     1.0,
			"interface": 0.95,
			"function":  0.9,
			"method":    0.85,
			"struct":    0.9,
			"enum":      0.8,
			"variable":  0.5,
			"const":     0.6,
		}
		if weight, ok := typeWeights[sym.Type]; ok {
			score += weight
		}

		// Factor 3: Name similarity to query
		nameLower := strings.ToLower(sym.Name)
		nameScore := fuzzyScore(nameLower, queryLower)
		score += nameScore * 0.5

		// Factor 4: Reference count (popularity)
		refCount := graph.ReferenceCount(sym)
		score += minFloat(float64(refCount)/10.0, 0.5)

		// Factor 5: Definition vs reference
		if sym.File == "" {
			score *= 0.5 // Penalize unresolved symbols
		}

		ranked = append(ranked, &RankedSymbol{
			Symbol: sym,
			Score:  score,
		})
	}

	// Sort by score descending
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked
}

// formatForLLM formats symbols for LLM prompt within token budget.
func (rm *RepoMap) formatForLLM(ranked []*RankedSymbol, maxTokens int) string {
	var builder strings.Builder
	usedTokens := 0

	for _, rs := range ranked {
		sym := rs.Symbol

		// Format symbol
		line := fmt.Sprintf("%s:%d: %s %s",
			sym.File, sym.Line, sym.Type, sym.Name)

		if sym.Signature != "" {
			line += " " + sym.Signature
		}

		// Estimate tokens (rough approximation: 1 token ≈ 4 chars)
		tokens := len(line) / 4

		if usedTokens+tokens > maxTokens {
			break
		}

		builder.WriteString(line)
		builder.WriteString("\n")
		usedTokens += tokens
	}

	return builder.String()
}

// ReferenceGraph tracks symbol references.
type ReferenceGraph struct {
	// File -> referenced files
	edges map[string]map[string]int

	// Symbol -> referencing files
	refs map[string][]string
}

// buildReferenceGraph builds a graph of file references.
func (rm *RepoMap) buildReferenceGraph(symbols []*Symbol) *ReferenceGraph {
	graph := &ReferenceGraph{
		edges: make(map[string]map[string]int),
		refs:  make(map[string][]string),
	}

	// Build symbol location index
	symbolLocs := make(map[string]string) // name -> file
	for _, sym := range symbols {
		symbolLocs[sym.Name] = sym.File
	}

	// Find references (simplified - would need import analysis)
	for _, sym := range symbols {
		// Track references
		key := fmt.Sprintf("%s:%s", sym.File, sym.Name)
		graph.refs[key] = sym.References
	}

	return graph
}

// MinDistance returns minimum distance between files.
func (g *ReferenceGraph) MinDistance(file string, targets []string) int {
	minDist := -1

	for _, target := range targets {
		if file == target {
			return 0
		}

		// Check direct edges
		if edges, ok := g.edges[file]; ok {
			if count, ok := edges[target]; ok && count > 0 {
				if minDist == -1 || 1 < minDist {
					minDist = 1
				}
			}
		}
	}

	if minDist == -1 {
		return 10 // Large default distance
	}
	return minDist
}

// ReferenceCount returns number of references to a symbol.
func (g *ReferenceGraph) ReferenceCount(sym *Symbol) int {
	key := fmt.Sprintf("%s:%s", sym.File, sym.Name)
	return len(g.refs[key])
}

// Helper methods

func (rm *RepoMap) initParsers() {
	// Go
	rm.parsers["go"] = sitter.NewParser()
	rm.parsers["go"].SetLanguage(golang.GetLanguage())
	rm.queries["go"] = `
		(function_declaration name: (identifier) @function)
		(method_declaration name: (field_identifier) @method)
		(type_declaration (type_spec name: (type_identifier) @type))
		(var_declaration (var_spec name: (identifier) @variable))
		(const_declaration (const_spec name: (identifier) @const))
	`

	// Python
	rm.parsers["python"] = sitter.NewParser()
	rm.parsers["python"].SetLanguage(python.GetLanguage())
	rm.queries["python"] = `
		(function_definition name: (identifier) @function)
		(class_definition name: (identifier) @class)
		(assignment left: (identifier) @variable)
	`

	// JavaScript - temporarily disabled due to import conflict
	// rm.parsers["javascript"] = sitter.NewParser()
	// rm.parsers["javascript"].SetLanguage(javascript.GetLanguage())
	rm.queries["javascript"] = `
		(function_declaration name: (identifier) @function)
		(method_definition name: (property_identifier) @method)
		(class_declaration name: (identifier) @class)
		(lexical_declaration (variable_declarator name: (identifier) @variable))
	`

	// TypeScript
	rm.parsers["typescript"] = sitter.NewParser()
	rm.parsers["typescript"].SetLanguage(typescript.GetLanguage())
	rm.queries["typescript"] = rm.queries["javascript"]

	// TSX
	rm.parsers["tsx"] = sitter.NewParser()
	rm.parsers["tsx"].SetLanguage(tsx.GetLanguage())
	rm.queries["tsx"] = rm.queries["javascript"]
}

func (rm *RepoMap) getLanguage(name string) *sitter.Language {
	switch name {
	case "go":
		return golang.GetLanguage()
	case "python":
		return python.GetLanguage()
	case "javascript":
		return nil // javascript.GetLanguage() - temporarily disabled
	case "typescript":
		return typescript.GetLanguage()
	case "tsx":
		return tsx.GetLanguage()
	default:
		return nil
	}
}

func (rm *RepoMap) mapCaptureType(captureName string) string {
	// Map tree-sitter capture names to symbol types
	mapping := map[string]string{
		"function": "function",
		"method":   "method",
		"class":    "class",
		"type":     "type",
		"variable": "variable",
		"const":    "const",
	}

	if t, ok := mapping[captureName]; ok {
		return t
	}
	return "unknown"
}

func (rm *RepoMap) extractSignature(node *sitter.Node, content []byte) string {
	// Extract function signature
	parent := node.Parent()
	if parent == nil {
		return ""
	}

	// Get the full declaration
	return parent.Content(content)
}

func (rm *RepoMap) extractDocstring(node *sitter.Node, content []byte) string {
	// This would extract Go doc comments, Python docstrings, etc.
	// Simplified implementation
	return ""
}

func (rm *RepoMap) detectLanguage(file string, content []byte) string {
	// Use go-enry for language detection
	lang, _ := enry.GetLanguageByFilename(file)
	if lang == "" {
		lang, _ = enry.GetLanguageByContent(file, content)
	}

	// Normalize language names
	lang = strings.ToLower(lang)
	switch lang {
	case "golang":
		return "go"
	case "python":
		return "python"
	case "javascript":
		return "javascript"
	case "typescript":
		return "typescript"
	case "tsx":
		return "tsx"
	default:
		return lang
	}
}

func (rm *RepoMap) listAllFiles() []string {
	var files []string

	filepath.Walk(rm.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Check ignore patterns
		for _, pattern := range rm.ignorePatterns {
			if matched, _ := filepath.Match(pattern, info.Name()); matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is relevant (has extension)
		if filepath.Ext(path) == "" {
			return nil
		}

		relPath, _ := filepath.Rel(rm.rootDir, path)
		files = append(files, relPath)

		return nil
	})

	return files
}

func (rm *RepoMap) readFile(file string) ([]byte, error) {
	// Check cache
	if cached := rm.fileCache.Get(file); cached != nil {
		return cached.Content, nil
	}

	// Read from disk
	path := filepath.Join(rm.rootDir, file)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Cache
	info, _ := os.Stat(path)
	var modTime int64
	if info != nil {
		modTime = info.ModTime().Unix()
	}

	rm.fileCache.Set(file, &FileInfo{
		Content:  content,
		Modified: modTime,
		Language: rm.detectLanguage(file, content),
	})

	return content, nil
}

func (rm *RepoMap) getRecentlyChangedFiles() []string {
	// This would integrate with git to get recently modified files
	// Simplified - return empty for now
	return nil
}

func (rm *RepoMap) estimateTokens(content string) int {
	// Rough estimation: 1 token ≈ 4 characters
	return len(content) / 4
}

// SymbolCache methods

func (c *SymbolCache) Get(file string) []*Symbol {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[file]
}

func (c *SymbolCache) Set(file string, symbols []*Symbol) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[file] = symbols
}

// FileCache methods

func (c *FileCache) Get(file string) *FileInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[file]
}

func (c *FileCache) Set(file string, info *FileInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[file] = info
}

// Helper functions

func fuzzyScore(s1, s2 string) float64 {
	// Simple fuzzy matching score
	words := strings.Fields(s2)
	if len(words) == 0 {
		return 0
	}

	matches := 0
	for _, word := range words {
		if strings.Contains(s1, word) {
			matches++
		}
	}

	return float64(matches) / float64(len(words))
}

func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
