// Package api provides Files API implementation for Claude Code integration.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FilesAPI provides file operations via Anthropic's Files API
type FilesAPI struct {
	client *Client
}

// NewFilesAPI creates a new Files API client
func NewFilesAPI(client *Client) *FilesAPI {
	return &FilesAPI{client: client}
}

// DownloadFile downloads a file from the Files API
func (f *FilesAPI) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/v1/files/%s/content", fileID)

	resp, err := f.client.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		return nil, handleErrorResponse(resp)
	}

	return resp.Body, nil
}

// DownloadFileToPath downloads a file to a specific path
func (f *FilesAPI) DownloadFileToPath(ctx context.Context, fileID, destPath string) error {
	reader, err := f.DownloadFile(ctx, fileID)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Create destination directory if needed
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Create file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	// Copy content
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// DownloadFileWithRetry downloads a file with retry logic
func (f *FilesAPI) DownloadFileWithRetry(ctx context.Context, fileID string, maxRetries int) (io.ReadCloser, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 500ms, 1s, 2s
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(500*attempt) * time.Millisecond):
			}
		}

		reader, err := f.DownloadFile(ctx, fileID)
		if err == nil {
			return reader, nil
		}

		lastErr = err

		// Don't retry on auth errors
		if IsAuthenticationError(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("download failed after %d attempts: %w", maxRetries, lastErr)
}

// FileInfo represents file metadata
type FileInfo struct {
	ID            string    `json:"id"`
	Object        string    `json:"object"`
	Bytes         int64     `json:"bytes"`
	CreatedAt     time.Time `json:"created_at"`
	Filename      string    `json:"filename"`
	Purpose       string    `json:"purpose"`
	Status        string    `json:"status"` // "uploaded", "processed", "error"
	StatusDetails string    `json:"status_details,omitempty"`
}

// GetFileInfo retrieves file metadata
func (f *FilesAPI) GetFileInfo(ctx context.Context, fileID string) (*FileInfo, error) {
	path := fmt.Sprintf("/v1/files/%s", fileID)

	resp, err := f.client.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result FileInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ListFiles lists all files
func (f *FilesAPI) ListFiles(ctx context.Context, purpose string) ([]FileInfo, error) {
	path := "/v1/files"
	if purpose != "" {
		path = fmt.Sprintf("/v1/files?purpose=%s", purpose)
	}

	resp, err := f.client.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result struct {
		Data   []FileInfo `json:"data"`
		Object string     `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Data, nil
}

// DeleteFile deletes a file
func (f *FilesAPI) DeleteFile(ctx context.Context, fileID string) error {
	path := fmt.Sprintf("/v1/files/%s", fileID)

	resp, err := f.client.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return handleErrorResponse(resp)
	}

	return nil
}

// UploadFileRequest represents a file upload request
type UploadFileRequest struct {
	Filename string
	Content  io.Reader
	Purpose  string // e.g., "assistants", "batch", "fine-tune"
}

// UploadFile uploads a file to the Files API
func (f *FilesAPI) UploadFile(ctx context.Context, req *UploadFileRequest) (*FileInfo, error) {
	// Build multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add purpose field
	if err := writer.WriteField("purpose", req.Purpose); err != nil {
		return nil, fmt.Errorf("write purpose field: %w", err)
	}

	// Add file
	part, err := writer.CreateFormFile("file", req.Filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, req.Content); err != nil {
		return nil, fmt.Errorf("write file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", f.client.baseURL+"/v1/files", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	f.client.setAuthHeaders(&httpReq.Header)

	// Execute request
	resp, err := f.client.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result FileInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// UploadFileFromPath uploads a file from a local path
func (f *FilesAPI) UploadFileFromPath(ctx context.Context, filePath, purpose string) (*FileInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	filename := filepath.Base(filePath)

	return f.UploadFile(ctx, &UploadFileRequest{
		Filename: filename,
		Content:  file,
		Purpose:  purpose,
	})
}

// WaitForFileProcessing waits for a file to be processed
func (f *FilesAPI) WaitForFileProcessing(ctx context.Context, fileID string, timeout time.Duration) (*FileInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			info, err := f.GetFileInfo(ctx, fileID)
			if err != nil {
				return nil, err
			}

			switch info.Status {
			case "processed":
				return info, nil
			case "error":
				return nil, fmt.Errorf("file processing failed: %s", info.StatusDetails)
			}
		}
	}
}

// MaxFileSize is the maximum file size (500MB)
const MaxFileSize = 500 * 1024 * 1024

// ValidateFileSize validates that a file is within size limits
func ValidateFileSize(size int64) error {
	if size > MaxFileSize {
		return fmt.Errorf("file size %d exceeds maximum of %d bytes", size, MaxFileSize)
	}
	return nil
}

// FileCache provides local caching for downloaded files
type FileCache struct {
	baseDir string
	maxSize int64
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	path       string
	size       int64
	accessedAt time.Time
}

// NewFileCache creates a new file cache
func NewFileCache(baseDir string, maxSize int64) (*FileCache, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	return &FileCache{
		baseDir: baseDir,
		maxSize: maxSize,
		entries: make(map[string]cacheEntry),
	}, nil
}

// Get retrieves a file from cache or downloads it
func (c *FileCache) Get(ctx context.Context, api *FilesAPI, fileID string) (string, error) {
	c.mu.RLock()
	entry, exists := c.entries[fileID]
	c.mu.RUnlock()

	if exists {
		// Check if file still exists
		if _, err := os.Stat(entry.path); err == nil {
			c.mu.Lock()
			c.entries[fileID] = cacheEntry{
				path:       entry.path,
				size:       entry.size,
				accessedAt: time.Now(),
			}
			c.mu.Unlock()
			return entry.path, nil
		}
	}

	// Download file
	cachePath := filepath.Join(c.baseDir, fileID)
	if err := api.DownloadFileToPath(ctx, fileID, cachePath); err != nil {
		return "", err
	}

	// Get file size
	info, err := os.Stat(cachePath)
	if err != nil {
		return "", err
	}

	// Add to cache
	c.mu.Lock()
	c.entries[fileID] = cacheEntry{
		path:       cachePath,
		size:       info.Size(),
		accessedAt: time.Now(),
	}
	c.mu.Unlock()

	// Clean up if needed
	c.cleanup()

	return cachePath, nil
}

// cleanup removes old entries to stay under max size
func (c *FileCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var totalSize int64
	for _, entry := range c.entries {
		totalSize += entry.size
	}

	if totalSize <= c.maxSize {
		return
	}

	// Sort by access time (oldest first)
	type kv struct {
		key   string
		entry cacheEntry
	}

	var sorted []kv
	for k, v := range c.entries {
		sorted = append(sorted, kv{k, v})
	}

	// Simple bubble sort by accessedAt
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].entry.accessedAt.After(sorted[j].entry.accessedAt) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Remove oldest entries until under limit
	for _, kv := range sorted {
		if totalSize <= c.maxSize {
			break
		}

		os.Remove(kv.entry.path)
		delete(c.entries, kv.key)
		totalSize -= kv.entry.size
	}
}

// Clear clears the entire cache
func (c *FileCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entry := range c.entries {
		os.Remove(entry.path)
	}

	c.entries = make(map[string]cacheEntry)
	return nil
}
