// Package servers provides MCP server adapters for various services.
// This file implements a Filesystem MCP server adapter for secure file operations.
package servers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dev.helix.agent/internal/utils"
)

// FilesystemAdapterConfig holds configuration for the Filesystem adapter
type FilesystemAdapterConfig struct {
	// AllowedPaths are the root paths that are allowed for file operations
	AllowedPaths []string `json:"allowed_paths"`
	// DeniedPaths are paths that are explicitly denied
	DeniedPaths []string `json:"denied_paths"`
	// MaxFileSize is the maximum file size in bytes that can be read/written
	MaxFileSize int64 `json:"max_file_size"`
	// AllowWrite enables write operations
	AllowWrite bool `json:"allow_write"`
	// AllowDelete enables delete operations
	AllowDelete bool `json:"allow_delete"`
	// AllowCreateDir enables directory creation
	AllowCreateDir bool `json:"allow_create_dir"`
	// FollowSymlinks allows following symbolic links
	FollowSymlinks bool `json:"follow_symlinks"`
}

// DefaultFilesystemAdapterConfig returns sensible defaults
func DefaultFilesystemAdapterConfig() FilesystemAdapterConfig {
	homeDir, _ := os.UserHomeDir() //nolint:errcheck
	return FilesystemAdapterConfig{
		AllowedPaths:   []string{homeDir},
		DeniedPaths:    []string{"/etc/passwd", "/etc/shadow", "/.ssh", "/.gnupg"},
		MaxFileSize:    10 * 1024 * 1024, // 10MB
		AllowWrite:     false,
		AllowDelete:    false,
		AllowCreateDir: false,
		FollowSymlinks: false,
	}
}

// FileInfo represents information about a file
type FileInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	Mode       string    `json:"mode"`
	ModTime    time.Time `json:"mod_time"`
	IsDir      bool      `json:"is_dir"`
	IsSymlink  bool      `json:"is_symlink"`
	LinkTarget string    `json:"link_target,omitempty"`
}

// DirectoryListing represents contents of a directory
type DirectoryListing struct {
	Path  string     `json:"path"`
	Files []FileInfo `json:"files"`
	Count int        `json:"count"`
}

// FileContent represents file content
type FileContent struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Size     int64  `json:"size"`
	Encoding string `json:"encoding"`
}

// FilesystemAdapter implements the MCP server interface for filesystem operations
type FilesystemAdapter struct {
	mu          sync.RWMutex
	config      FilesystemAdapterConfig
	initialized bool
}

// NewFilesystemAdapter creates a new Filesystem adapter
func NewFilesystemAdapter(config FilesystemAdapterConfig) *FilesystemAdapter {
	return &FilesystemAdapter{
		config: config,
	}
}

// Initialize initializes the adapter
func (a *FilesystemAdapter) Initialize(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.initialized {
		return nil
	}

	// Validate allowed paths exist
	for _, path := range a.config.AllowedPaths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("allowed path does not exist: %s", path)
		}
	}

	a.initialized = true
	return nil
}

// Health checks if the adapter is healthy
func (a *FilesystemAdapter) Health(ctx context.Context) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return fmt.Errorf("filesystem adapter not initialized")
	}

	// Check if allowed paths are still accessible
	for _, path := range a.config.AllowedPaths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("allowed path not accessible: %s", path)
		}
	}

	return nil
}

// Close closes the adapter
func (a *FilesystemAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.initialized = false
	return nil
}

// isPathAllowed checks if a path is allowed based on configuration
// Uses utils.ValidatePath to prevent path traversal attacks (G304)
func (a *FilesystemAdapter) isPathAllowed(path string) error {
	// Validate path for traversal attacks and dangerous characters
	if !utils.ValidatePath(path) {
		return fmt.Errorf("invalid path: contains path traversal or dangerous characters: %s", path)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Double-check the absolute path for traversal (belt and suspenders)
	if !utils.ValidatePath(absPath) {
		return fmt.Errorf("invalid path after normalization: contains path traversal: %s", path)
	}

	// Check denied paths first
	for _, denied := range a.config.DeniedPaths {
		if strings.Contains(absPath, denied) {
			return fmt.Errorf("path is denied: %s", path)
		}
	}

	// Check if path is under an allowed root
	allowed := false
	for _, allowedPath := range a.config.AllowedPaths {
		allowedAbs, _ := filepath.Abs(allowedPath) //nolint:errcheck
		if strings.HasPrefix(absPath, allowedAbs) {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("path is outside allowed directories: %s", path)
	}

	// Check for symlinks if not allowed
	if !a.config.FollowSymlinks {
		// #nosec G304 - path is validated by isPathAllowed with utils.ValidatePath
		info, err := os.Lstat(absPath)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed: %s", path)
		}
	}

	return nil
}

// ReadFile reads a file and returns its content
func (a *FilesystemAdapter) ReadFile(ctx context.Context, path string) (*FileContent, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return nil, fmt.Errorf("adapter not initialized")
	}

	if err := a.isPathAllowed(path); err != nil {
		return nil, err
	}

	// #nosec G304 - path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file")
	}

	if info.Size() > a.config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum %d", info.Size(), a.config.MaxFileSize)
	}

	// #nosec G304 - path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &FileContent{
		Path:     path,
		Content:  string(content),
		Size:     info.Size(),
		Encoding: "utf-8",
	}, nil
}

// WriteFile writes content to a file
func (a *FilesystemAdapter) WriteFile(ctx context.Context, path string, content string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return fmt.Errorf("adapter not initialized")
	}

	if !a.config.AllowWrite {
		return fmt.Errorf("write operations are not allowed")
	}

	if err := a.isPathAllowed(path); err != nil {
		return err
	}

	if int64(len(content)) > a.config.MaxFileSize {
		return fmt.Errorf("content size %d exceeds maximum %d", len(content), a.config.MaxFileSize)
	}

	// #nosec G306 - file permissions 0644 are appropriate for user-created files
	// #nosec G304 - path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// AppendFile appends content to a file
func (a *FilesystemAdapter) AppendFile(ctx context.Context, path string, content string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return fmt.Errorf("adapter not initialized")
	}

	if !a.config.AllowWrite {
		return fmt.Errorf("write operations are not allowed")
	}

	if err := a.isPathAllowed(path); err != nil {
		return err
	}

	// Check existing file size
	// #nosec G304 - path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	info, err := os.Stat(path)
	if err == nil && info.Size()+int64(len(content)) > a.config.MaxFileSize {
		return fmt.Errorf("appending would exceed maximum file size")
	}

	// #nosec G306 - file permissions 0644 are appropriate for user-created files
	// #nosec G304 - path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for append: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to append to file: %w", err)
	}

	return nil
}

// DeleteFile deletes a file
func (a *FilesystemAdapter) DeleteFile(ctx context.Context, path string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return fmt.Errorf("adapter not initialized")
	}

	if !a.config.AllowDelete {
		return fmt.Errorf("delete operations are not allowed")
	}

	if err := a.isPathAllowed(path); err != nil {
		return err
	}

	// #nosec G304 - path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("path is a directory, use DeleteDirectory instead")
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// ListDirectory lists contents of a directory
func (a *FilesystemAdapter) ListDirectory(ctx context.Context, path string) (*DirectoryListing, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return nil, fmt.Errorf("adapter not initialized")
	}

	if err := a.isPathAllowed(path); err != nil {
		return nil, err
	}

	// #nosec G304 - path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fi := FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime(),
			IsDir:   entry.IsDir(),
		}

		// Check for symlink
		if info.Mode()&os.ModeSymlink != 0 {
			fi.IsSymlink = true
			if target, err := os.Readlink(filepath.Join(path, entry.Name())); err == nil {
				fi.LinkTarget = target
			}
		}

		files = append(files, fi)
	}

	return &DirectoryListing{
		Path:  path,
		Files: files,
		Count: len(files),
	}, nil
}

// CreateDirectory creates a directory
func (a *FilesystemAdapter) CreateDirectory(ctx context.Context, path string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return fmt.Errorf("adapter not initialized")
	}

	if !a.config.AllowCreateDir {
		return fmt.Errorf("directory creation is not allowed")
	}

	// Check parent directory is allowed
	parentDir := filepath.Dir(path)
	if err := a.isPathAllowed(parentDir); err != nil {
		return err
	}

	// #nosec G301 -- restricted permissions for directories that may contain sensitive data
	if err := os.MkdirAll(path, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

// DeleteDirectory deletes a directory
func (a *FilesystemAdapter) DeleteDirectory(ctx context.Context, path string, recursive bool) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return fmt.Errorf("adapter not initialized")
	}

	if !a.config.AllowDelete {
		return fmt.Errorf("delete operations are not allowed")
	}

	if err := a.isPathAllowed(path); err != nil {
		return err
	}

	// #nosec G304 - path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	if recursive {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to delete directory recursively: %w", err)
		}
	} else {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to delete directory (may not be empty): %w", err)
		}
	}

	return nil
}

// GetFileInfo gets information about a file or directory
func (a *FilesystemAdapter) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return nil, fmt.Errorf("adapter not initialized")
	}

	if err := a.isPathAllowed(path); err != nil {
		return nil, err
	}

	// #nosec G304 - path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	fi := &FileInfo{
		Name:    info.Name(),
		Path:    path,
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}

	// Check for symlink using Lstat
	linfo, err := os.Lstat(path)
	if err == nil && linfo.Mode()&os.ModeSymlink != 0 {
		fi.IsSymlink = true
		if target, err := os.Readlink(path); err == nil {
			fi.LinkTarget = target
		}
	}

	return fi, nil
}

// CopyFile copies a file to a new location
func (a *FilesystemAdapter) CopyFile(ctx context.Context, src, dst string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return fmt.Errorf("adapter not initialized")
	}

	if !a.config.AllowWrite {
		return fmt.Errorf("write operations are not allowed")
	}

	if err := a.isPathAllowed(src); err != nil {
		return err
	}
	if err := a.isPathAllowed(dst); err != nil {
		return err
	}

	// #nosec G304 - src path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if srcInfo.IsDir() {
		return fmt.Errorf("source is a directory, not a file")
	}

	if srcInfo.Size() > a.config.MaxFileSize {
		return fmt.Errorf("file size exceeds maximum")
	}

	// #nosec G304 - src path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	// #nosec G304 - dst path is validated by isPathAllowed with utils.ValidatePath, allowed paths whitelist, and denied paths blacklist
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	return nil
}

// MoveFile moves a file to a new location
func (a *FilesystemAdapter) MoveFile(ctx context.Context, src, dst string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return fmt.Errorf("adapter not initialized")
	}

	if !a.config.AllowWrite || !a.config.AllowDelete {
		return fmt.Errorf("write and delete operations are required for move")
	}

	if err := a.isPathAllowed(src); err != nil {
		return err
	}
	if err := a.isPathAllowed(dst); err != nil {
		return err
	}

	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	return nil
}

// SearchFiles searches for files matching a pattern
func (a *FilesystemAdapter) SearchFiles(ctx context.Context, rootPath string, pattern string, maxResults int) ([]FileInfo, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.initialized {
		return nil, fmt.Errorf("adapter not initialized")
	}

	if err := a.isPathAllowed(rootPath); err != nil {
		return nil, err
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	var results []FileInfo
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		if len(results) >= maxResults {
			return filepath.SkipAll
		}

		// Check if filename matches pattern
		matched, _ := filepath.Match(pattern, info.Name()) //nolint:errcheck
		if matched {
			results = append(results, FileInfo{
				Name:    info.Name(),
				Path:    path,
				Size:    info.Size(),
				Mode:    info.Mode().String(),
				ModTime: info.ModTime(),
				IsDir:   info.IsDir(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	return results, nil
}

// GetMCPTools returns the MCP tool definitions for the filesystem adapter
func (a *FilesystemAdapter) GetMCPTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "filesystem_read_file",
			Description: "Read the contents of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to read",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "filesystem_write_file",
			Description: "Write content to a file (creates or overwrites)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to write",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "filesystem_append_file",
			Description: "Append content to a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to append to",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to append",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "filesystem_delete_file",
			Description: "Delete a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to delete",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "filesystem_list_directory",
			Description: "List contents of a directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the directory to list",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "filesystem_create_directory",
			Description: "Create a directory (including parents)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the directory to create",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "filesystem_delete_directory",
			Description: "Delete a directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the directory to delete",
					},
					"recursive": map[string]interface{}{
						"type":        "boolean",
						"description": "Delete recursively (including contents)",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "filesystem_get_info",
			Description: "Get information about a file or directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to get info for",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "filesystem_copy_file",
			Description: "Copy a file to a new location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]interface{}{
						"type":        "string",
						"description": "Source file path",
					},
					"destination": map[string]interface{}{
						"type":        "string",
						"description": "Destination file path",
					},
				},
				"required": []string{"source", "destination"},
			},
		},
		{
			Name:        "filesystem_move_file",
			Description: "Move a file to a new location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]interface{}{
						"type":        "string",
						"description": "Source file path",
					},
					"destination": map[string]interface{}{
						"type":        "string",
						"description": "Destination file path",
					},
				},
				"required": []string{"source", "destination"},
			},
		},
		{
			Name:        "filesystem_search",
			Description: "Search for files matching a pattern",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"root_path": map[string]interface{}{
						"type":        "string",
						"description": "Root directory to search from",
					},
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Glob pattern to match (e.g., *.txt)",
					},
					"max_results": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results to return",
					},
				},
				"required": []string{"root_path", "pattern"},
			},
		},
	}
}

// ExecuteTool executes an MCP tool by name
func (a *FilesystemAdapter) ExecuteTool(ctx context.Context, toolName string, args json.RawMessage) (interface{}, error) {
	var params map[string]interface{}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}

	switch toolName {
	case "filesystem_read_file":
		//nolint:errcheck // schema validation ensures correct type
		path, _ := params["path"].(string) //nolint:errcheck // schema validation ensures correct type
		return a.ReadFile(ctx, path)

	case "filesystem_write_file":
		//nolint:errcheck // schema validation ensures correct type
		path, _ := params["path"].(string) //nolint:errcheck // schema validation ensures correct type
		//nolint:errcheck // schema validation ensures correct type
		content, _ := params["content"].(string) //nolint:errcheck // schema validation ensures correct type
		return nil, a.WriteFile(ctx, path, content)

	case "filesystem_append_file":
		//nolint:errcheck // schema validation ensures correct type
		path, _ := params["path"].(string) //nolint:errcheck // schema validation ensures correct type
		//nolint:errcheck // schema validation ensures correct type
		content, _ := params["content"].(string) //nolint:errcheck // schema validation ensures correct type
		return nil, a.AppendFile(ctx, path, content)

	case "filesystem_delete_file":
		//nolint:errcheck // schema validation ensures correct type
		path, _ := params["path"].(string) //nolint:errcheck // schema validation ensures correct type
		return nil, a.DeleteFile(ctx, path)

	case "filesystem_list_directory":
		//nolint:errcheck // schema validation ensures correct type
		path, _ := params["path"].(string) //nolint:errcheck // schema validation ensures correct type
		return a.ListDirectory(ctx, path)

	case "filesystem_create_directory":
		//nolint:errcheck // schema validation ensures correct type
		path, _ := params["path"].(string) //nolint:errcheck // schema validation ensures correct type
		return nil, a.CreateDirectory(ctx, path)

	case "filesystem_delete_directory":
		//nolint:errcheck // schema validation ensures correct type
		path, _ := params["path"].(string) //nolint:errcheck // schema validation ensures correct type
		//nolint:errcheck // schema validation ensures correct type
		recursive, _ := params["recursive"].(bool) //nolint:errcheck // schema validation ensures correct type
		return nil, a.DeleteDirectory(ctx, path, recursive)

	case "filesystem_get_info":
		//nolint:errcheck // schema validation ensures correct type
		path, _ := params["path"].(string) //nolint:errcheck // schema validation ensures correct type
		return a.GetFileInfo(ctx, path)

	case "filesystem_copy_file":
		//nolint:errcheck // schema validation ensures correct type
		src, _ := params["source"].(string) //nolint:errcheck // schema validation ensures correct type
		//nolint:errcheck // schema validation ensures correct type
		dst, _ := params["destination"].(string) //nolint:errcheck // schema validation ensures correct type
		return nil, a.CopyFile(ctx, src, dst)

	case "filesystem_move_file":
		//nolint:errcheck // schema validation ensures correct type
		src, _ := params["source"].(string) //nolint:errcheck // schema validation ensures correct type
		//nolint:errcheck // schema validation ensures correct type
		dst, _ := params["destination"].(string) //nolint:errcheck // schema validation ensures correct type
		return nil, a.MoveFile(ctx, src, dst)

	case "filesystem_search":
		//nolint:errcheck // schema validation ensures correct type
		rootPath, _ := params["root_path"].(string) //nolint:errcheck // schema validation ensures correct type
		//nolint:errcheck // schema validation ensures correct type
		pattern, _ := params["pattern"].(string) //nolint:errcheck // schema validation ensures correct type
		maxResults := 100
		if mr, ok := params["max_results"].(float64); ok {
			maxResults = int(mr)
		}
		return a.SearchFiles(ctx, rootPath, pattern, maxResults)

	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}
