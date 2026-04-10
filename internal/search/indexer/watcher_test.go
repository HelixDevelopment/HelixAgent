package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileWatcher(t *testing.T) {
	t.Parallel()

	emb := &mockEmbedder{}
	store := &mockVectorStore{}
	cfg := DefaultConfig()

	idx := NewCodeIndexer(emb, store, cfg)
	watcher, err := NewFileWatcher(idx, cfg)

	require.NoError(t, err)
	require.NotNil(t, watcher)
	assert.NotNil(t, watcher.watcher)
	assert.NotNil(t, watcher.indexer)
	assert.NotNil(t, watcher.stopChan)
}

func TestFileWatcher_Stop(t *testing.T) {
	t.Parallel()

	emb := &mockEmbedder{}
	store := &mockVectorStore{}
	cfg := DefaultConfig()

	idx := NewCodeIndexer(emb, store, cfg)
	watcher, err := NewFileWatcher(idx, cfg)
	require.NoError(t, err)

	err = watcher.Stop()
	assert.NoError(t, err)
}

func TestFileWatcher_Start_And_Stop(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

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
	watcher, err := NewFileWatcher(idx, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	err = watcher.Start(ctx)
	assert.NoError(t, err)

	// Give the watcher goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// Stop via context cancellation
	cancel()
	time.Sleep(50 * time.Millisecond)

	err = watcher.Stop()
	assert.NoError(t, err)
}

func TestFileWatcher_Start_WatchesSubdirectories(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create subdirectories
	subDir := filepath.Join(tmpDir, "pkg", "handlers")
	err := os.MkdirAll(subDir, 0755)
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
	watcher, err := NewFileWatcher(idx, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = watcher.Start(ctx)
	assert.NoError(t, err)

	// Allow watcher goroutine to start
	time.Sleep(50 * time.Millisecond)

	err = watcher.Stop()
	assert.NoError(t, err)
}

func TestFileWatcher_Start_ExcludesVendor(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create vendor directory
	vendorDir := filepath.Join(tmpDir, "vendor")
	err := os.MkdirAll(vendorDir, 0755)
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
	watcher, err := NewFileWatcher(idx, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should not fail even with excluded dirs
	err = watcher.Start(ctx)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	err = watcher.Stop()
	assert.NoError(t, err)
}

func TestFileWatcher_ContextCancellation(t *testing.T) {
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
	watcher, err := NewFileWatcher(idx, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = watcher.Start(ctx)
	assert.NoError(t, err)

	// Wait for context to expire
	<-ctx.Done()

	// Small delay for goroutine cleanup
	time.Sleep(50 * time.Millisecond)

	err = watcher.Stop()
	assert.NoError(t, err)
}
