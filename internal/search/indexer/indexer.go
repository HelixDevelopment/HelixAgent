// Package indexer provides code indexing capabilities
package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dev.helix.agent/internal/search/chunker"
	"dev.helix.agent/internal/search/types"
)

// Config holds indexer configuration
type Config struct {
	RootPath        string
	IncludePatterns []string
	ExcludePatterns []string
	ChunkSize       int
	ChunkOverlap    int
	MaxFileSize     int64
	IndexOnStartup  bool
	WatchFiles      bool
	CollectionName  string
}

// DefaultConfig returns default indexer configuration
func DefaultConfig() Config {
	return Config{
		RootPath:        ".",
		IncludePatterns: []string{"*.go", "*.py", "*.js", "*.ts", "*.rs", "*.java", "*.cpp", "*.c", "*.h"},
		ExcludePatterns: []string{"vendor/", "node_modules/", ".git/", "*.pb.go", "*_test.go", "dist/", "build/"},
		ChunkSize:       50,
		ChunkOverlap:    10,
		MaxFileSize:     1024 * 1024, // 1MB
		IndexOnStartup:  true,
		WatchFiles:      false,
		CollectionName:  "code_embeddings",
	}
}

// CodeIndexer manages the indexing pipeline
type CodeIndexer struct {
	embedder     types.Embedder
	vectorStore  types.VectorStore
	chunker      types.Chunker
	config       Config
	mu           sync.RWMutex
	indexedFiles map[string]time.Time
}

// NewCodeIndexer creates a new code indexer
func NewCodeIndexer(embedder types.Embedder, store types.VectorStore, config Config) *CodeIndexer {
	return &CodeIndexer{
		embedder:     embedder,
		vectorStore:  store,
		chunker:      chunker.NewSimpleChunker(config.ChunkSize, config.ChunkOverlap),
		config:       config,
		indexedFiles: make(map[string]time.Time),
	}
}

// Index performs full indexing of the codebase
func (i *CodeIndexer) Index(ctx context.Context) (*types.IndexResult, error) {
	start := time.Now()
	result := &types.IndexResult{}

	// Create collection if it doesn't exist. A pre-existing collection
	// is the expected common case ("already exists" from the vector
	// store), so the error is intentionally ignored. Any other error
	// class would re-surface on the first Upsert below, so we don't
	// need to distinguish classes here.
	_ = i.vectorStore.CreateCollection(ctx, i.config.CollectionName, i.embedder.Dimensions()) // nolint:errcheck // intentional, see comment above

	// Walk the directory tree
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // Limit concurrent goroutines

	err := filepath.Walk(i.config.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, err)
			return nil
		}

		// Skip directories
		if info.IsDir() {
			if i.shouldExcludeDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be indexed
		if !i.shouldIndexFile(path) {
			return nil
		}

		// Check file size
		if info.Size() > i.config.MaxFileSize {
			return nil
		}

		// Index file concurrently
		wg.Add(1)
		semaphore <- struct{}{}

		go func(filePath string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			if err := i.IndexFile(ctx, filePath); err != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Errorf("%s: %w", filePath, err))
				mu.Unlock()
			} else {
				mu.Lock()
				result.FilesIndexed++
				mu.Unlock()
			}
		}(path)

		return nil
	})

	wg.Wait()

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// IndexFile indexes a single file
func (i *CodeIndexer) IndexFile(ctx context.Context, path string) error {
	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect language
	language := i.detectLanguage(path)

	// Chunk file
	chunks, err := chunker.ChunkFile(path, string(content), language, i.chunker)
	if err != nil {
		return fmt.Errorf("failed to chunk file: %w", err)
	}

	if len(chunks) == 0 {
		return nil
	}

	// Generate embeddings
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	embeddings, err := i.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Create documents
	docs := make([]types.Document, len(chunks))
	for i, chunk := range chunks {
		chunk.Embeddings = embeddings[i]
		docs[i] = types.Document{
			ID:      chunk.ID,
			Vector:  embeddings[i],
			Content: chunk.Content,
			Metadata: map[string]interface{}{
				"file_path":  chunk.FilePath,
				"start_line": chunk.StartLine,
				"end_line":   chunk.EndLine,
				"language":   chunk.Language,
				"type":       string(chunk.Type),
			},
		}
	}

	// Upsert to vector store
	if err := i.vectorStore.Upsert(ctx, i.config.CollectionName, docs); err != nil {
		return fmt.Errorf("failed to upsert documents: %w", err)
	}

	// Update indexed files
	i.mu.Lock()
	i.indexedFiles[path] = time.Now()
	i.mu.Unlock()

	return nil
}

// DeleteFile removes a file from the index
// Note: This removes the file from the indexed files map. Full vector store deletion
// would require tracking all chunk IDs per file, which is not currently implemented.
func (i *CodeIndexer) DeleteFile(ctx context.Context, path string) error {
	i.mu.Lock()
	delete(i.indexedFiles, path)
	i.mu.Unlock()

	return nil
}

// Watch starts watching for file changes
func (i *CodeIndexer) Watch(ctx context.Context) error {
	watcher, err := NewFileWatcher(i, i.config)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	return watcher.Start(ctx)
}

// shouldIndexFile checks if a file should be indexed
func (i *CodeIndexer) shouldIndexFile(path string) bool {
	// Check exclude patterns first
	for _, pattern := range i.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return false
		}
		if strings.Contains(path, pattern) {
			return false
		}
	}

	// Check include patterns
	for _, pattern := range i.config.IncludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}

	return false
}

// shouldExcludeDir checks if a directory should be excluded
func (i *CodeIndexer) shouldExcludeDir(path string) bool {
	for _, pattern := range i.config.ExcludePatterns {
		if strings.HasSuffix(pattern, "/") {
			if strings.Contains(path, strings.TrimSuffix(pattern, "/")) {
				return true
			}
		}
	}
	return false
}

// detectLanguage detects the programming language from file extension
func (i *CodeIndexer) detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".h", ".hpp":
		return "cpp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".r":
		return "r"
	case ".m":
		return "objective-c"
	default:
		return "unknown"
	}
}

// Ensure interface is implemented
var _ types.Indexer = (*CodeIndexer)(nil)
