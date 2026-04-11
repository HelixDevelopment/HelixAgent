// Package indexer provides file watching capabilities
package indexer

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches for file changes and triggers reindexing
type FileWatcher struct {
	indexer  *CodeIndexer
	watcher  *fsnotify.Watcher
	config   Config
	stopChan chan struct{}
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(indexer *CodeIndexer, config Config) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &FileWatcher{
		indexer:  indexer,
		watcher:  watcher,
		config:   config,
		stopChan: make(chan struct{}),
	}, nil
}

// Start begins watching for file changes
func (w *FileWatcher) Start(ctx context.Context) error {
	// Add root path to watcher
	if err := w.addWatchPaths(w.config.RootPath); err != nil {
		return err
	}

	go w.watchLoop(ctx)
	return nil
}

// Stop stops the file watcher
func (w *FileWatcher) Stop() error {
	close(w.stopChan)
	return w.watcher.Close()
}

// addWatchPaths recursively adds directories to watch
func (w *FileWatcher) addWatchPaths(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() {
			return nil
		}

		// Skip excluded directories
		if w.indexer.shouldExcludeDir(path) {
			return filepath.SkipDir
		}

		return w.watcher.Add(path)
	})
}

// watchLoop processes file system events
func (w *FileWatcher) watchLoop(ctx context.Context) {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(ctx, event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)

		case <-w.stopChan:
			return

		case <-ctx.Done():
			return
		}
	}
}

// handleEvent processes a file system event
func (w *FileWatcher) handleEvent(ctx context.Context, event fsnotify.Event) {
	// Check if file should be indexed
	if !w.indexer.shouldIndexFile(event.Name) {
		return
	}

	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		log.Printf("Modified file: %s", event.Name)
		if err := w.indexer.IndexFile(ctx, event.Name); err != nil {
			log.Printf("Failed to reindex %s: %v", event.Name, err)
		}

	case event.Op&fsnotify.Create == fsnotify.Create:
		log.Printf("Created file: %s", event.Name)
		if err := w.indexer.IndexFile(ctx, event.Name); err != nil {
			log.Printf("Failed to index %s: %v", event.Name, err)
		}

		// If it's a directory, add it to watcher
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			w.watcher.Add(event.Name)
		}

	case event.Op&fsnotify.Remove == fsnotify.Remove:
		log.Printf("Removed file: %s", event.Name)
		if err := w.indexer.DeleteFile(ctx, event.Name); err != nil {
			log.Printf("Failed to delete %s from index: %v", event.Name, err)
		}

	case event.Op&fsnotify.Rename == fsnotify.Rename:
		log.Printf("Renamed file: %s", event.Name)
		// Treat as delete - new file will be handled by Create event
		if err := w.indexer.DeleteFile(ctx, event.Name); err != nil {
			log.Printf("Failed to delete %s from index: %v", event.Name, err)
		}
	}
}
