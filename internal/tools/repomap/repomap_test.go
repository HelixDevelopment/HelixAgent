package repomap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRepoMap(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	rm := NewRepoMap("/test/path", logger)

	assert.NotNil(t, rm)
	assert.Equal(t, "/test/path", rm.RootPath)
	assert.NotNil(t, rm.Languages)
	assert.NotNil(t, rm.Files)
	assert.NotNil(t, rm.Symbols)
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()

	assert.Equal(t, int64(1024*1024), config.MaxFileSize)
	assert.Equal(t, 20, config.MaxDepth)
	assert.NotEmpty(t, config.IgnorePatterns)
	assert.Contains(t, config.IgnorePatterns, "node_modules")
	assert.Contains(t, config.IgnorePatterns, ".git")
}

func TestDetectLanguage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ext      string
		path     string
		expected string
	}{
		{".go", "test.go", "Go"},
		{".py", "test.py", "Python"},
		{".js", "test.js", "JavaScript"},
		{".ts", "test.ts", "TypeScript"},
		{".jsx", "test.jsx", "JavaScript"},
		{".tsx", "test.tsx", "TypeScript"},
		{".java", "test.java", "Java"},
		{".rs", "test.rs", "Rust"},
		{".cpp", "test.cpp", "C++"},
		{".c", "test.c", "C"},
		{".rb", "test.rb", "Ruby"},
		{".swift", "test.swift", "Swift"},
		{".md", "test.md", "Markdown"},
		{"", "Dockerfile", "Dockerfile"},
		{"", "go.mod", "Go"},
		{"", "package.json", "JavaScript"},
		{"", "unknown.xyz", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			result := detectLanguage(tt.ext, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTestFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     string
		language string
		expected bool
	}{
		{"test_foo.py", "Python", true},
		{"foo_test.py", "Python", true},
		{"main.py", "Python", false},
		{"foo_test.go", "Go", true},
		{"main.go", "Go", false},
		{"foo.test.js", "JavaScript", true},
		{"foo.spec.ts", "TypeScript", true},
		{"main.js", "JavaScript", false},
		{"FooTest.java", "Java", true},
		{"Main.java", "Java", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			result := isTestFile(tt.path, tt.language)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsBinary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     string
		expected bool
	}{
		{"test.png", true},
		{"test.jpg", true},
		{"test.exe", true},
		{"test.zip", true},
		{"test.pdf", true},
		{"test.go", false},
		{"test.py", false},
		{"test.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			result := isBinary(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldIgnore(t *testing.T) {
	t.Parallel()
	rm := NewRepoMap("/test", logrus.New())
	patterns := []string{"node_modules", ".git", "*.tmp"}

	tests := []struct {
		path     string
		expected bool
	}{
		{"src/node_modules/package.json", true},
		{".git/config", true},
		{"temp.tmp", true},
		{"src/main.go", false},
		{"README.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			result := rm.shouldIgnore(tt.path, patterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRepoMap_Map(t *testing.T) {
	t.Parallel()
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`
package main

func main() {
	println("Hello")
}

func helper() int {
	return 42
}
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "utils.py"), []byte(`
def helper():
    return 42

class MyClass:
    def method(self):
        pass
`), 0644)

	os.MkdirAll(filepath.Join(tmpDir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "node_modules", "package.json"), []byte(`{}`), 0644)

	// Map repository
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	rm := NewRepoMap(tmpDir, logger)

	config := DefaultConfig()
	ctx := context.Background()

	err := rm.Map(ctx, config)
	require.NoError(t, err)

	// Verify results
	assert.Len(t, rm.Files, 2)                   // Should ignore node_modules
	assert.GreaterOrEqual(t, len(rm.Symbols), 4) // 2 Go funcs + 1 Python func + 1 class

	// Check languages
	assert.Contains(t, rm.Languages, "Go")
	assert.Contains(t, rm.Languages, "Python")

	// Check Go language stats
	goStats := rm.Languages["Go"]
	assert.Equal(t, 1, goStats.FileCount)
	assert.Greater(t, goStats.LineCount, 0)
}

func TestRepoMap_GetFiles(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	rm := NewRepoMap("/test", logger)
	rm.Files = []FileInfo{
		{Path: "/test/main.go", Language: "Go"},
		{Path: "/test/utils.py", Language: "Python"},
	}

	files := rm.GetFiles()
	assert.Len(t, files, 2)
}

func TestRepoMap_GetSymbols(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	rm := NewRepoMap("/test", logger)
	rm.Symbols = []Symbol{
		{Name: "main", Type: "function", File: "/test/main.go"},
		{Name: "helper", Type: "function", File: "/test/main.go"},
		{Name: "MyClass", Type: "class", File: "/test/utils.py"},
	}

	symbols := rm.GetSymbols()
	assert.Len(t, symbols, 3)
}

func TestRepoMap_GetSymbolsByType(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	rm := NewRepoMap("/test", logger)
	rm.Symbols = []Symbol{
		{Name: "main", Type: "function", File: "/test/main.go"},
		{Name: "helper", Type: "function", File: "/test/main.go"},
		{Name: "MyClass", Type: "class", File: "/test/utils.py"},
	}

	funcs := rm.GetSymbolsByType("function")
	assert.Len(t, funcs, 2)

	classes := rm.GetSymbolsByType("class")
	assert.Len(t, classes, 1)
}

func TestRepoMap_GetSymbolsInFile(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	rm := NewRepoMap("/test", logger)
	rm.Symbols = []Symbol{
		{Name: "main", Type: "function", File: "/test/main.go"},
		{Name: "helper", Type: "function", File: "/test/main.go"},
		{Name: "MyClass", Type: "class", File: "/test/utils.py"},
	}

	goSymbols := rm.GetSymbolsInFile("/test/main.go")
	assert.Len(t, goSymbols, 2)

	pySymbols := rm.GetSymbolsInFile("/test/utils.py")
	assert.Len(t, pySymbols, 1)
}

func TestRepoMap_FindSymbol(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	rm := NewRepoMap("/test", logger)
	rm.Symbols = []Symbol{
		{Name: "main", Type: "function", File: "/test/main.go"},
		{Name: "main", Type: "function", File: "/test/other.go"},
		{Name: "helper", Type: "function", File: "/test/main.go"},
	}

	results := rm.FindSymbol("main")
	assert.Len(t, results, 2)

	results = rm.FindSymbol("helper")
	assert.Len(t, results, 1)

	results = rm.FindSymbol("nonexistent")
	assert.Len(t, results, 0)
}

func TestRepoMap_GetSummary(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	rm := NewRepoMap("/test", logger)
	rm.Files = []FileInfo{
		{Path: "/test/main.go", Language: "Go", LineCount: 50},
		{Path: "/test/utils.py", Language: "Python", LineCount: 30},
	}
	rm.Symbols = []Symbol{
		{Name: "main", Type: "function"},
		{Name: "helper", Type: "function"},
	}
	rm.Languages = map[string]LanguageInfo{
		"Go":     {Name: "Go", FileCount: 1, LineCount: 50},
		"Python": {Name: "Python", FileCount: 1, LineCount: 30},
	}

	summary := rm.GetSummary()

	assert.Equal(t, "/test", summary["root_path"])
	assert.Equal(t, 2, summary["total_files"])
	assert.Equal(t, 2, summary["total_symbols"])
	assert.Equal(t, 80, summary["total_lines"])
	assert.Equal(t, 2, summary["languages"])
}

// TestRepoMap_Snapshot is the CONST-029 verification test for the
// single-writer model. Snapshot must return a deep copy so a second
// goroutine reading the snapshot cannot race with the Map()-owner.
func TestRepoMap_Snapshot(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	orig := NewRepoMap("/test", logger)
	orig.Files = []FileInfo{{Path: "/test/a.go", Language: "Go", LineCount: 10}}
	orig.Symbols = []Symbol{{Name: "a", Type: "function"}}
	orig.Languages = map[string]LanguageInfo{"Go": {Name: "Go", FileCount: 1, LineCount: 10}}
	orig.Dependencies = []Dependency{{Source: "a", Target: "b", Type: "import"}}

	snap := orig.Snapshot()
	require.NotNil(t, snap)

	// Mutate the snapshot — must not leak into the original.
	snap.Files = append(snap.Files, FileInfo{Path: "/test/b.go"})
	snap.Symbols = append(snap.Symbols, Symbol{Name: "extra"})
	snap.Languages["Python"] = LanguageInfo{Name: "Python"}
	snap.Dependencies = append(snap.Dependencies, Dependency{Source: "c", Target: "d"})

	assert.Len(t, orig.Files, 1, "snapshot mutation leaked into orig.Files")
	assert.Len(t, orig.Symbols, 1, "snapshot mutation leaked into orig.Symbols")
	assert.NotContains(t, orig.Languages, "Python", "snapshot mutation leaked into orig.Languages")
	assert.Len(t, orig.Dependencies, 1, "snapshot mutation leaked into orig.Dependencies")
}

// TestRepoMap_NilSnapshot documents the nil-receiver contract.
func TestRepoMap_NilSnapshot(t *testing.T) {
	t.Parallel()
	var rm *RepoMap
	assert.Nil(t, rm.Snapshot())
}

func TestExtractGoSymbols(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package main

import "fmt"

// MyStruct is a test struct
type MyStruct struct {
	Field string
}

// MyInterface is a test interface
type MyInterface interface {
	Method()
}

// main is the entry point
func main() {
	fmt.Println("Hello")
}

// Helper is an exported function
func Helper() int {
	return 42
}

// unexported is private
func unexported() {}

const MaxSize = 100

var GlobalVar = "test"

type MyType string
`
	os.WriteFile(testFile, []byte(content), 0644)

	rm := NewRepoMap(tmpDir, logrus.New())
	symbols, err := rm.extractGoSymbols(testFile)
	require.NoError(t, err)

	// Check that we found symbols
	assert.GreaterOrEqual(t, len(symbols), 6)

	// Check for specific symbols
	symbolNames := make(map[string]string)
	for _, sym := range symbols {
		symbolNames[sym.Name] = sym.Type
	}

	assert.Equal(t, "struct", symbolNames["MyStruct"])
	assert.Equal(t, "interface", symbolNames["MyInterface"])
	assert.Equal(t, "function", symbolNames["main"])
	assert.Equal(t, "function", symbolNames["Helper"])
	assert.Equal(t, "const", symbolNames["MaxSize"])
	assert.Equal(t, "var", symbolNames["GlobalVar"])
	assert.Equal(t, "type", symbolNames["MyType"])
}

func TestExtractPythonSymbols(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.py")

	content := `
def helper():
    return 42

class MyClass:
    def method1(self):
        pass

    def method2(self):
        pass

class AnotherClass:
    pass

MAX_SIZE = 100
`
	os.WriteFile(testFile, []byte(content), 0644)

	rm := NewRepoMap(tmpDir, logrus.New())
	symbols, err := rm.extractPythonSymbols(testFile)
	require.NoError(t, err)

	symbolNames := make(map[string]string)
	for _, sym := range symbols {
		symbolNames[sym.Name] = sym.Type
	}

	assert.Equal(t, "function", symbolNames["helper"])
	assert.Equal(t, "class", symbolNames["MyClass"])
	assert.Equal(t, "class", symbolNames["AnotherClass"])
	// Methods are inside classes - basic detection may not find them
	assert.Equal(t, "const", symbolNames["MAX_SIZE"])
}

func TestExtractJSSymbols(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.js")

	content := `
function helper() {
    return 42;
}

const arrow = () => 42;

class MyClass {
    constructor() {}

    method1() {}

    method2() {}
}

const constant = 42;
`
	os.WriteFile(testFile, []byte(content), 0644)

	rm := NewRepoMap(tmpDir, logrus.New())
	symbols, err := rm.extractJSSymbols(testFile)
	require.NoError(t, err)

	symbolNames := make(map[string]string)
	for _, sym := range symbols {
		symbolNames[sym.Name] = sym.Type
	}

	assert.Equal(t, "function", symbolNames["helper"])
	assert.Equal(t, "function", symbolNames["arrow"])
	assert.Equal(t, "class", symbolNames["MyClass"])
	// Methods are inside classes - basic detection may not find them
	assert.Equal(t, "const", symbolNames["constant"])
}

func TestIsExported(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected bool
	}{
		{"Exported", true},
		{"exported", false},
		{"", false},
		{"_private", false},
		{"Public", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isExported(tt.name)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkRepoMap_Map(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test files
	for i := 0; i < 100; i++ {
		content := `package main
func main() {}
func helper() int { return 42 }
`
		os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i)), []byte(content), 0644)
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	rm := NewRepoMap(tmpDir, logger)
	config := DefaultConfig()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.Map(ctx, config)
	}
}
