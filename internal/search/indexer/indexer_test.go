package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/search/types"
)

// --- mock helpers ---

type mockEmbedder struct {
	embedResult [][]float32
	embedErr    error
	dims        int
}

func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if m.embedErr != nil {
		return nil, m.embedErr
	}
	if m.embedResult != nil {
		return m.embedResult, nil
	}
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = []float32{0.1, 0.2, 0.3}
	}
	return res, nil
}

func (m *mockEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockEmbedder) Dimensions() int {
	if m.dims > 0 {
		return m.dims
	}
	return 3
}

type mockVectorStore struct {
	createErr     error
	upsertErr     error
	deleteErr     error
	searchResults []types.SearchResult
	upsertedDocs  []types.Document
	deletedIDs    []string
	createdCol    string
}

func (m *mockVectorStore) CreateCollection(_ context.Context, name string, _ int) error {
	m.createdCol = name
	return m.createErr
}

func (m *mockVectorStore) DeleteCollection(_ context.Context, _ string) error {
	return m.deleteErr
}

func (m *mockVectorStore) Upsert(_ context.Context, _ string, docs []types.Document) error {
	m.upsertedDocs = append(m.upsertedDocs, docs...)
	return m.upsertErr
}

func (m *mockVectorStore) Delete(_ context.Context, _ string, ids []string) error {
	m.deletedIDs = append(m.deletedIDs, ids...)
	return nil
}

func (m *mockVectorStore) Search(_ context.Context, _ string, _ []float32, _ types.SearchOptions) ([]types.SearchResult, error) {
	return m.searchResults, nil
}

func (m *mockVectorStore) SearchByText(_ context.Context, _ string, _ string, _ types.SearchOptions) ([]types.SearchResult, error) {
	return nil, nil
}

func (m *mockVectorStore) GetCollectionStats(_ context.Context, _ string) (*types.CollectionStats, error) {
	return &types.CollectionStats{}, nil
}

// --- tests ---

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	assert.Equal(t, ".", cfg.RootPath)
	assert.NotEmpty(t, cfg.IncludePatterns)
	assert.NotEmpty(t, cfg.ExcludePatterns)
	assert.Equal(t, 50, cfg.ChunkSize)
	assert.Equal(t, 10, cfg.ChunkOverlap)
	assert.Equal(t, int64(1024*1024), cfg.MaxFileSize)
	assert.True(t, cfg.IndexOnStartup)
	assert.False(t, cfg.WatchFiles)
	assert.Equal(t, "code_embeddings", cfg.CollectionName)

	// Verify default include patterns
	assert.Contains(t, cfg.IncludePatterns, "*.go")
	assert.Contains(t, cfg.IncludePatterns, "*.py")
	assert.Contains(t, cfg.IncludePatterns, "*.js")
	assert.Contains(t, cfg.IncludePatterns, "*.ts")

	// Verify default exclude patterns
	assert.Contains(t, cfg.ExcludePatterns, "vendor/")
	assert.Contains(t, cfg.ExcludePatterns, "node_modules/")
	assert.Contains(t, cfg.ExcludePatterns, ".git/")
}

func TestNewCodeIndexer(t *testing.T) {
	t.Parallel()

	emb := &mockEmbedder{dims: 128}
	store := &mockVectorStore{}
	cfg := DefaultConfig()

	idx := NewCodeIndexer(emb, store, cfg)
	require.NotNil(t, idx)
	assert.NotNil(t, idx.embedder)
	assert.NotNil(t, idx.vectorStore)
	assert.NotNil(t, idx.chunker)
	assert.NotNil(t, idx.indexedFiles)
}

func TestCodeIndexer_DetectLanguage(t *testing.T) {
	t.Parallel()

	idx := &CodeIndexer{}

	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"component.ts", "typescript"},
		{"lib.rs", "rust"},
		{"Main.java", "java"},
		{"program.cpp", "cpp"},
		{"program.cc", "cpp"},
		{"program.cxx", "cpp"},
		{"main.c", "c"},
		{"header.h", "cpp"},
		{"header.hpp", "cpp"},
		{"script.rb", "ruby"},
		{"index.php", "php"},
		{"app.swift", "swift"},
		{"Main.kt", "kotlin"},
		{"Main.scala", "scala"},
		{"analysis.r", "r"},
		{"controller.m", "objective-c"},
		{"README.md", "unknown"},
		{"Makefile", "unknown"},
		{"data.json", "unknown"},
		{"style.css", "unknown"},
		{"/path/to/file.go", "go"},
		{"deep/nested/path/file.py", "python"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, idx.detectLanguage(tc.path))
		})
	}
}

func TestCodeIndexer_ShouldIndexFile(t *testing.T) {
	t.Parallel()

	idx := &CodeIndexer{
		config: Config{
			IncludePatterns: []string{"*.go", "*.py", "*.js"},
			ExcludePatterns: []string{"vendor/", "*_test.go", "*.pb.go"},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"go file", "main.go", true},
		{"python file", "script.py", true},
		{"js file", "app.js", true},
		{"test file excluded", "main_test.go", false},
		{"protobuf excluded", "api.pb.go", false},
		{"vendor path excluded", "vendor/lib/file.go", false},
		{"markdown not included", "README.md", false},
		{"json not included", "config.json", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, idx.shouldIndexFile(tc.path))
		})
	}
}

func TestCodeIndexer_ShouldExcludeDir(t *testing.T) {
	t.Parallel()

	idx := &CodeIndexer{
		config: Config{
			ExcludePatterns: []string{"vendor/", "node_modules/", ".git/"},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"vendor dir", "vendor", true},
		{"nested vendor", "path/to/vendor", true},
		{"node_modules", "node_modules", true},
		{"git dir", ".git", true},
		{"source dir", "src", false},
		{"internal dir", "internal", false},
		{"cmd dir", "cmd", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, idx.shouldExcludeDir(tc.path))
		})
	}
}

func TestCodeIndexer_IndexFile(t *testing.T) {
	t.Parallel()

	// Create a temp file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	emb := &mockEmbedder{dims: 3}
	store := &mockVectorStore{}
	cfg := Config{
		ChunkSize:      50,
		ChunkOverlap:   10,
		CollectionName: "test_col",
	}

	idx := NewCodeIndexer(emb, store, cfg)

	err = idx.IndexFile(context.Background(), filePath)
	require.NoError(t, err)

	// Should have upserted documents
	assert.NotEmpty(t, store.upsertedDocs)

	// Should track indexed file
	idx.mu.RLock()
	_, tracked := idx.indexedFiles[filePath]
	idx.mu.RUnlock()
	assert.True(t, tracked)
}

func TestCodeIndexer_IndexFile_NonexistentFile(t *testing.T) {
	t.Parallel()

	emb := &mockEmbedder{}
	store := &mockVectorStore{}
	cfg := Config{ChunkSize: 10, ChunkOverlap: 2, CollectionName: "col"}
	idx := NewCodeIndexer(emb, store, cfg)

	err := idx.IndexFile(context.Background(), "/nonexistent/file.go")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestCodeIndexer_IndexFile_EmbedError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte("package main\nfunc main() {}\n"), 0644)
	require.NoError(t, err)

	emb := &mockEmbedder{embedErr: assert.AnError}
	store := &mockVectorStore{}
	cfg := Config{ChunkSize: 50, ChunkOverlap: 10, CollectionName: "col"}
	idx := NewCodeIndexer(emb, store, cfg)

	err = idx.IndexFile(context.Background(), filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate embeddings")
}

func TestCodeIndexer_IndexFile_UpsertError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte("package main\nfunc main() {}\n"), 0644)
	require.NoError(t, err)

	emb := &mockEmbedder{}
	store := &mockVectorStore{upsertErr: assert.AnError}
	cfg := Config{ChunkSize: 50, ChunkOverlap: 10, CollectionName: "col"}
	idx := NewCodeIndexer(emb, store, cfg)

	err = idx.IndexFile(context.Background(), filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upsert documents")
}

func TestCodeIndexer_IndexFile_DocumentMetadata(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "handler.go")
	content := "package handlers\n\nfunc HandleRequest() {}\n"
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	emb := &mockEmbedder{}
	store := &mockVectorStore{}
	cfg := Config{ChunkSize: 50, ChunkOverlap: 10, CollectionName: "col"}
	idx := NewCodeIndexer(emb, store, cfg)

	err = idx.IndexFile(context.Background(), filePath)
	require.NoError(t, err)
	require.NotEmpty(t, store.upsertedDocs)

	doc := store.upsertedDocs[0]
	assert.Equal(t, filePath, doc.Metadata["file_path"])
	assert.Equal(t, "go", doc.Metadata["language"])
	assert.NotNil(t, doc.Metadata["start_line"])
	assert.NotNil(t, doc.Metadata["end_line"])
	assert.Equal(t, "general", doc.Metadata["type"])
	assert.NotEmpty(t, doc.Vector)
	assert.NotEmpty(t, doc.Content)
}

func TestCodeIndexer_DeleteFile(t *testing.T) {
	t.Parallel()

	emb := &mockEmbedder{}
	store := &mockVectorStore{}
	cfg := Config{ChunkSize: 10, ChunkOverlap: 2, CollectionName: "col"}
	idx := NewCodeIndexer(emb, store, cfg)

	// Add a file to the tracked map
	idx.mu.Lock()
	idx.indexedFiles["test.go"] = time.Now()
	idx.mu.Unlock()

	err := idx.DeleteFile(context.Background(), "test.go")
	require.NoError(t, err)

	idx.mu.RLock()
	_, found := idx.indexedFiles["test.go"]
	idx.mu.RUnlock()
	assert.False(t, found, "file should be removed from indexed files")
}

func TestCodeIndexer_DeleteFile_NotTracked(t *testing.T) {
	t.Parallel()

	emb := &mockEmbedder{}
	store := &mockVectorStore{}
	cfg := Config{ChunkSize: 10, ChunkOverlap: 2, CollectionName: "col"}
	idx := NewCodeIndexer(emb, store, cfg)

	// Deleting a file not in the map should not error
	err := idx.DeleteFile(context.Background(), "nonexistent.go")
	assert.NoError(t, err)
}

func TestCodeIndexer_Index(t *testing.T) {
	t.Parallel()

	// Create temp directory with Go files
	tmpDir := t.TempDir()

	// Create a Go file
	goFile := filepath.Join(tmpDir, "main.go")
	err := os.WriteFile(goFile, []byte("package main\nfunc main() {}\n"), 0644)
	require.NoError(t, err)

	// Create a file that should be excluded
	mdFile := filepath.Join(tmpDir, "README.md")
	err = os.WriteFile(mdFile, []byte("# README\n"), 0644)
	require.NoError(t, err)

	emb := &mockEmbedder{}
	store := &mockVectorStore{}
	cfg := Config{
		RootPath:        tmpDir,
		IncludePatterns: []string{"*.go"},
		ExcludePatterns: []string{"vendor/", "*_test.go"},
		ChunkSize:       50,
		ChunkOverlap:    10,
		MaxFileSize:     1024 * 1024,
		CollectionName:  "test_col",
	}

	idx := NewCodeIndexer(emb, store, cfg)
	result, err := idx.Index(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.FilesIndexed)
	assert.True(t, result.Duration > 0)
	assert.Empty(t, result.Errors)

	// The collection should have been created
	assert.Equal(t, "test_col", store.createdCol)
}

func TestCodeIndexer_Index_ExcludesDirectories(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create vendor directory with a Go file
	vendorDir := filepath.Join(tmpDir, "vendor")
	err := os.MkdirAll(vendorDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte("package lib\n"), 0644)
	require.NoError(t, err)

	// Create source file
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644)
	require.NoError(t, err)

	emb := &mockEmbedder{}
	store := &mockVectorStore{}
	cfg := Config{
		RootPath:        tmpDir,
		IncludePatterns: []string{"*.go"},
		ExcludePatterns: []string{"vendor/"},
		ChunkSize:       50,
		ChunkOverlap:    10,
		MaxFileSize:     1024 * 1024,
		CollectionName:  "col",
	}

	idx := NewCodeIndexer(emb, store, cfg)
	result, err := idx.Index(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, result.FilesIndexed) // Only main.go, not vendor/lib.go
}

func TestCodeIndexer_Index_SkipsLargeFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a small file
	err := os.WriteFile(filepath.Join(tmpDir, "small.go"), []byte("package small\n"), 0644)
	require.NoError(t, err)

	// Create a large file exceeding MaxFileSize
	largeContent := make([]byte, 2048)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	err = os.WriteFile(filepath.Join(tmpDir, "large.go"), largeContent, 0644)
	require.NoError(t, err)

	emb := &mockEmbedder{}
	store := &mockVectorStore{}
	cfg := Config{
		RootPath:        tmpDir,
		IncludePatterns: []string{"*.go"},
		ExcludePatterns: []string{},
		ChunkSize:       50,
		ChunkOverlap:    10,
		MaxFileSize:     1024, // 1KB limit
		CollectionName:  "col",
	}

	idx := NewCodeIndexer(emb, store, cfg)
	result, err := idx.Index(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, result.FilesIndexed) // Only small.go
}

func TestCodeIndexer_Index_EmptyDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	emb := &mockEmbedder{}
	store := &mockVectorStore{}
	cfg := Config{
		RootPath:        tmpDir,
		IncludePatterns: []string{"*.go"},
		ExcludePatterns: []string{},
		ChunkSize:       50,
		ChunkOverlap:    10,
		MaxFileSize:     1024 * 1024,
		CollectionName:  "col",
	}

	idx := NewCodeIndexer(emb, store, cfg)
	result, err := idx.Index(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, result.FilesIndexed)
}

func TestCodeIndexer_ImplementsIndexer(t *testing.T) {
	t.Parallel()

	var _ types.Indexer = (*CodeIndexer)(nil)
}
