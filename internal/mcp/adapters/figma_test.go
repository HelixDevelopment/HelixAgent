// Package adapters provides MCP server adapter tests.
package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFigmaAdapter(t *testing.T) {
	t.Parallel()
	config := FigmaConfig{
		AccessToken: "test-token",
	}

	adapter := NewFigmaAdapter(config)

	assert.NotNil(t, adapter)
	assert.Equal(t, "test-token", adapter.config.AccessToken)
	assert.Equal(t, DefaultFigmaConfig().BaseURL, adapter.config.BaseURL)
	assert.Equal(t, DefaultFigmaConfig().Timeout, adapter.config.Timeout)
}

func TestFigmaAdapter_GetServerInfo(t *testing.T) {
	t.Parallel()
	adapter := NewFigmaAdapter(FigmaConfig{})

	info := adapter.GetServerInfo()

	assert.Equal(t, "figma", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "Figma")
	assert.NotEmpty(t, info.Capabilities)
	assert.Contains(t, info.Capabilities, "get_file")
	assert.Contains(t, info.Capabilities, "get_components")
}

func TestFigmaAdapter_ListTools(t *testing.T) {
	t.Parallel()
	adapter := NewFigmaAdapter(FigmaConfig{})

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	assert.True(t, toolNames["figma_get_file"])
	assert.True(t, toolNames["figma_get_file_nodes"])
	assert.True(t, toolNames["figma_get_images"])
	assert.True(t, toolNames["figma_get_components"])
	assert.True(t, toolNames["figma_get_styles"])
	assert.True(t, toolNames["figma_get_comments"])
	assert.True(t, toolNames["figma_post_comment"])
	assert.True(t, toolNames["figma_get_team_projects"])
	assert.True(t, toolNames["figma_get_project_files"])
}

func TestFigmaAdapter_CallTool_UnknownTool(t *testing.T) {
	t.Parallel()
	adapter := NewFigmaAdapter(FigmaConfig{})

	result, err := adapter.CallTool(context.Background(), "unknown_tool", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestFigmaAdapter_GetFile(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/files/test-file-key", r.URL.Path)
		assert.Equal(t, "test-token", r.Header.Get("X-Figma-Token"))

		response := FigmaFileResponse{
			Name:         "Test File",
			LastModified: "2024-01-01T00:00:00Z",
			Version:      "1.0",
			ThumbnailURL: "https://example.com/thumb.png",
			Document: FigmaNode{
				ID:   "0:0",
				Name: "Document",
				Type: "DOCUMENT",
				Children: []FigmaNode{
					{ID: "1:1", Name: "Page 1", Type: "CANVAS"},
				},
			},
			Components: map[string]FigmaCompDef{
				"comp1": {Key: "comp1", Name: "Button", Description: "A button component"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewFigmaAdapter(FigmaConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL + "/v1",
	})

	result, err := adapter.CallTool(context.Background(), "figma_get_file", map[string]interface{}{
		"file_key": "test-file-key",
		"depth":    2,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.NotEmpty(t, result.Content)
	assert.Contains(t, result.Content[0].Text, "Test File")
}

func TestFigmaAdapter_GetFileNodes(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/files/test-file-key/nodes")
		assert.Contains(t, r.URL.RawQuery, "ids=1:1,2:2")

		response := FigmaNodesResponse{
			Nodes: map[string]*FigmaNodeData{
				"1:1": {Document: &FigmaNode{ID: "1:1", Name: "Node 1", Type: "FRAME"}},
				"2:2": {Document: &FigmaNode{ID: "2:2", Name: "Node 2", Type: "TEXT"}},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewFigmaAdapter(FigmaConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL + "/v1",
	})

	result, err := adapter.CallTool(context.Background(), "figma_get_file_nodes", map[string]interface{}{
		"file_key": "test-file-key",
		"node_ids": []interface{}{"1:1", "2:2"},
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Node 1")
}

func TestFigmaAdapter_GetImages(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/images/test-file-key")

		response := FigmaImagesResponse{
			Images: map[string]string{
				"1:1": "https://example.com/image1.png",
				"2:2": "https://example.com/image2.png",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewFigmaAdapter(FigmaConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL + "/v1",
	})

	result, err := adapter.CallTool(context.Background(), "figma_get_images", map[string]interface{}{
		"file_key": "test-file-key",
		"node_ids": []interface{}{"1:1", "2:2"},
		"format":   "png",
		"scale":    2.0,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Exported")
}

func TestFigmaAdapter_GetComments(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/files/test-file-key/comments")

		response := FigmaCommentsResponse{
			Comments: []FigmaComment{
				{
					ID:        "comment1",
					Message:   "Great design!",
					CreatedAt: "2024-01-01T00:00:00Z",
					User:      FigmaUser{Handle: "user1"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewFigmaAdapter(FigmaConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL + "/v1",
	})

	result, err := adapter.CallTool(context.Background(), "figma_get_comments", map[string]interface{}{
		"file_key": "test-file-key",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Great design!")
}

func TestFigmaAdapter_PostComment(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/files/test-file-key/comments")

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Test comment", body["message"])

		response := FigmaComment{
			ID:      "new-comment-id",
			Message: "Test comment",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewFigmaAdapter(FigmaConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL + "/v1",
	})

	result, err := adapter.CallTool(context.Background(), "figma_post_comment", map[string]interface{}{
		"file_key": "test-file-key",
		"message":  "Test comment",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "new-comment-id")
}

func TestFigmaAdapter_GetTeamProjects(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/teams/team-123/projects")

		response := FigmaProjectsResponse{
			Projects: []FigmaProject{
				{ID: "proj1", Name: "Project 1"},
				{ID: "proj2", Name: "Project 2"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewFigmaAdapter(FigmaConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL + "/v1",
	})

	result, err := adapter.CallTool(context.Background(), "figma_get_team_projects", map[string]interface{}{
		"team_id": "team-123",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Project 1")
}

func TestFigmaAdapter_GetProjectFiles(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/projects/proj-123/files")

		response := FigmaFilesResponse{
			Files: []FigmaFileMeta{
				{Key: "file1", Name: "Design File 1", LastModified: "2024-01-01"},
				{Key: "file2", Name: "Design File 2", LastModified: "2024-01-02"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewFigmaAdapter(FigmaConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL + "/v1",
	})

	result, err := adapter.CallTool(context.Background(), "figma_get_project_files", map[string]interface{}{
		"project_id": "proj-123",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Design File 1")
}

func TestFigmaAdapter_APIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Invalid token"}`))
	}))
	defer server.Close()

	adapter := NewFigmaAdapter(FigmaConfig{
		AccessToken: "invalid-token",
		BaseURL:     server.URL + "/v1",
	})

	result, err := adapter.CallTool(context.Background(), "figma_get_file", map[string]interface{}{
		"file_key": "test-file-key",
	})

	require.NoError(t, err) // Error is returned in ToolResult, not as error
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "API error")
}

func TestDefaultFigmaConfig(t *testing.T) {
	t.Parallel()
	config := DefaultFigmaConfig()

	assert.Equal(t, "https://api.figma.com/v1", config.BaseURL)
	assert.Equal(t, 60*time.Second, config.Timeout)
}
