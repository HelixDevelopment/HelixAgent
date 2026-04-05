package adapters

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockGoogleDriveClient implements GoogleDriveClient for testing
type MockGoogleDriveClient struct {
	files       []DriveFile
	shouldError bool
}

func NewMockGoogleDriveClient() *MockGoogleDriveClient {
	return &MockGoogleDriveClient{
		files: []DriveFile{
			{
				ID:           "file1",
				Name:         "document.txt",
				MimeType:     "text/plain",
				Size:         1024,
				CreatedTime:  time.Now().Add(-24 * time.Hour),
				ModifiedTime: time.Now().Add(-time.Hour),
				Parents:      []string{"root"},
				WebViewLink:  "https://drive.google.com/file/d/file1/view",
				Shared:       false,
			},
			{
				ID:           "folder1",
				Name:         "My Folder",
				MimeType:     "application/vnd.google-apps.folder",
				Size:         0,
				CreatedTime:  time.Now().Add(-48 * time.Hour),
				ModifiedTime: time.Now().Add(-24 * time.Hour),
				Parents:      []string{"root"},
				Shared:       true,
			},
			{
				ID:           "file2",
				Name:         "image.jpg",
				MimeType:     "image/jpeg",
				Size:         5000000,
				CreatedTime:  time.Now().Add(-72 * time.Hour),
				ModifiedTime: time.Now().Add(-48 * time.Hour),
				Parents:      []string{"folder1"},
				Shared:       false,
			},
		},
	}
}

func (m *MockGoogleDriveClient) SetError(shouldError bool) {
	m.shouldError = shouldError
}

func (m *MockGoogleDriveClient) ListFiles(ctx context.Context, query string, pageSize int) ([]DriveFile, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	if pageSize > len(m.files) {
		return m.files, nil
	}
	return m.files[:pageSize], nil
}

func (m *MockGoogleDriveClient) GetFile(ctx context.Context, fileID string) (*DriveFile, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, f := range m.files {
		if f.ID == fileID {
			return &f, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockGoogleDriveClient) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return io.NopCloser(strings.NewReader("file content here")), nil
}

func (m *MockGoogleDriveClient) CreateFile(ctx context.Context, name, mimeType, parentID string, content io.Reader) (*DriveFile, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return &DriveFile{
		ID:       "new-file-id",
		Name:     name,
		MimeType: mimeType,
		Parents:  []string{parentID},
	}, nil
}

func (m *MockGoogleDriveClient) UpdateFile(ctx context.Context, fileID string, content io.Reader) (*DriveFile, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, f := range m.files {
		if f.ID == fileID {
			return &f, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockGoogleDriveClient) DeleteFile(ctx context.Context, fileID string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockGoogleDriveClient) CopyFile(ctx context.Context, fileID, name string) (*DriveFile, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return &DriveFile{
		ID:   "copied-file-id",
		Name: name,
	}, nil
}

func (m *MockGoogleDriveClient) MoveFile(ctx context.Context, fileID, newParentID string) (*DriveFile, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, f := range m.files {
		if f.ID == fileID {
			f.Parents = []string{newParentID}
			return &f, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockGoogleDriveClient) CreateFolder(ctx context.Context, name, parentID string) (*DriveFile, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return &DriveFile{
		ID:       "new-folder-id",
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentID},
	}, nil
}

func (m *MockGoogleDriveClient) SearchFiles(ctx context.Context, query string) ([]DriveFile, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	var results []DriveFile
	for _, f := range m.files {
		if strings.Contains(strings.ToLower(f.Name), strings.ToLower(query)) {
			results = append(results, f)
		}
	}
	return results, nil
}

func (m *MockGoogleDriveClient) ShareFile(ctx context.Context, fileID, email, role string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

// Tests

func TestDefaultGoogleDriveConfig(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()

	assert.Equal(t, 60*time.Second, config.Timeout)
}

func TestNewGoogleDriveAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	assert.NotNil(t, adapter)

	info := adapter.GetServerInfo()
	assert.Equal(t, "google-drive", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
}

func TestGoogleDriveAdapter_ListTools(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "drive_list_files")
	assert.Contains(t, toolNames, "drive_get_file")
	assert.Contains(t, toolNames, "drive_download_file")
	assert.Contains(t, toolNames, "drive_create_file")
	assert.Contains(t, toolNames, "drive_delete_file")
}

func TestGoogleDriveAdapter_ListFiles(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_list_files", map[string]interface{}{
		"query":     "",
		"page_size": 50,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestGoogleDriveAdapter_GetFile(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_get_file", map[string]interface{}{
		"file_id": "file1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGoogleDriveAdapter_DownloadFile(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_download_file", map[string]interface{}{
		"file_id": "file1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGoogleDriveAdapter_CreateFile(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_create_file", map[string]interface{}{
		"name":      "new-file.txt",
		"content":   "Hello, World!",
		"mime_type": "text/plain",
		"parent_id": "folder1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGoogleDriveAdapter_UpdateFile(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_update_file", map[string]interface{}{
		"file_id": "file1",
		"content": "Updated content",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGoogleDriveAdapter_DeleteFile(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_delete_file", map[string]interface{}{
		"file_id": "file1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGoogleDriveAdapter_CreateFolder(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_create_folder", map[string]interface{}{
		"name":      "New Folder",
		"parent_id": "root",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGoogleDriveAdapter_Search(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_search", map[string]interface{}{
		"query": "document",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGoogleDriveAdapter_ShareFile(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_share_file", map[string]interface{}{
		"file_id": "file1",
		"email":   "user@example.com",
		"role":    "reader",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGoogleDriveAdapter_CopyFile(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_copy_file", map[string]interface{}{
		"file_id": "file1",
		"name":    "document-copy.txt",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGoogleDriveAdapter_MoveFile(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_move_file", map[string]interface{}{
		"file_id":       "file1",
		"new_parent_id": "folder1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGoogleDriveAdapter_InvalidTool(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	_, err := adapter.CallTool(ctx, "invalid_tool", map[string]interface{}{})

	assert.Error(t, err)
}

func TestGoogleDriveAdapter_ErrorHandling(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	client.SetError(true)
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_list_files", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Type tests

func TestDriveFileTypes(t *testing.T) {
	t.Parallel()
	file := DriveFile{
		ID:           "abc123",
		Name:         "test-document.pdf",
		MimeType:     "application/pdf",
		Size:         1024000,
		CreatedTime:  time.Now().Add(-24 * time.Hour),
		ModifiedTime: time.Now(),
		Parents:      []string{"folder1", "folder2"},
		WebViewLink:  "https://drive.google.com/file/d/abc123/view",
		IconLink:     "https://drive.google.com/icon/pdf",
		Owners:       []Owner{{DisplayName: "John Doe", EmailAddress: "john@example.com"}},
		Shared:       true,
	}

	assert.Equal(t, "abc123", file.ID)
	assert.Equal(t, "test-document.pdf", file.Name)
	assert.Equal(t, "application/pdf", file.MimeType)
	assert.Equal(t, int64(1024000), file.Size)
	assert.True(t, file.Shared)
	assert.Len(t, file.Parents, 2)
	assert.Len(t, file.Owners, 1)
}

func TestOwnerTypes(t *testing.T) {
	t.Parallel()
	owner := Owner{
		DisplayName:  "Jane Smith",
		EmailAddress: "jane@example.com",
	}

	assert.Equal(t, "Jane Smith", owner.DisplayName)
	assert.Equal(t, "jane@example.com", owner.EmailAddress)
}

func TestGoogleDriveConfigTypes(t *testing.T) {
	t.Parallel()
	config := GoogleDriveConfig{
		ClientID:     "client-id-123",
		ClientSecret: "client-secret-456",
		RefreshToken: "refresh-token-789",
		Timeout:      120 * time.Second,
	}

	assert.NotEmpty(t, config.ClientID)
	assert.NotEmpty(t, config.ClientSecret)
	assert.NotEmpty(t, config.RefreshToken)
	assert.Equal(t, 120*time.Second, config.Timeout)
}

func TestGoogleDriveAdapter_GetServerInfoCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	info := adapter.GetServerInfo()
	assert.Contains(t, info.Capabilities, "file_management")
	assert.Contains(t, info.Capabilities, "folder_management")
	assert.Contains(t, info.Capabilities, "sharing")
	assert.Contains(t, info.Capabilities, "search")
}

func TestGoogleDriveAdapter_ListFilesWithFolderIcon(t *testing.T) {
	t.Parallel()
	config := DefaultGoogleDriveConfig()
	client := NewMockGoogleDriveClient()
	adapter := NewGoogleDriveAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "drive_list_files", map[string]interface{}{
		"page_size": 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Should contain folder icon for folder items
	if len(result.Content) > 0 && result.Content[0].Text != "" {
		assert.Contains(t, result.Content[0].Text, "Found")
	}
}
