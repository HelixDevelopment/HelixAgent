// Package servers provides MCP server adapters for various services.
package servers

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
	tests := []struct {
		name          string
		config        FigmaAdapterConfig
		expectedToken string
	}{
		{
			name: "with custom config",
			config: FigmaAdapterConfig{
				APIToken: "test-token",
				Timeout:  60 * time.Second,
			},
			expectedToken: "test-token",
		},
		{
			name: "with default timeout",
			config: FigmaAdapterConfig{
				APIToken: "default-token",
			},
			expectedToken: "default-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			adapter := NewFigmaAdapter(tt.config)
			assert.NotNil(t, adapter)
			assert.Equal(t, tt.expectedToken, adapter.apiToken)
			assert.NotNil(t, adapter.httpClient)
			assert.Equal(t, "https://api.figma.com/v1", adapter.baseURL)
		})
	}
}

func TestFigmaAdapter_Connect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name: "successful connection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/me", r.URL.Path)
				assert.Equal(t, "test-token", r.Header.Get("X-Figma-Token"))
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"id":     "12345",
					"handle": "testuser",
					"email":  "test@example.com",
				})
			},
			expectError: false,
		},
		{
			name: "authentication failure",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"status":403,"err":"Invalid token"}`))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			err := adapter.Connect(context.Background())
			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, adapter.IsConnected())
			} else {
				assert.NoError(t, err)
				assert.True(t, adapter.IsConnected())
			}
		})
	}
}

func TestFigmaAdapter_Health(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name: "healthy",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{"id": "12345"})
			},
			expectError: false,
		},
		{
			name: "unhealthy",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			err := adapter.Health(context.Background())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFigmaAdapter_GetFile(t *testing.T) {
	t.Parallel()
	expectedFile := FigmaFile{
		Name:         "Test File",
		LastModified: "2024-01-01T00:00:00Z",
		Version:      "123456789",
		Document: &FigmaDocument{
			ID:   "0:0",
			Name: "Document",
			Type: "DOCUMENT",
		},
	}

	tests := []struct {
		name          string
		fileKey       string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:    "successful get",
			fileKey: "abc123",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/files/abc123", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedFile)
			},
			expectError: false,
		},
		{
			name:    "file not found",
			fileKey: "nonexistent",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"status":404,"err":"File not found"}`))
			},
			expectError: true,
		},
		{
			name:    "decode error",
			fileKey: "invalid",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid json"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			file, err := adapter.GetFile(context.Background(), tt.fileKey)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, file)
				assert.Equal(t, expectedFile.Name, file.Name)
			}
		})
	}
}

func TestFigmaAdapter_GetFileNodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		fileKey       string
		nodeIDs       []string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:    "successful get",
			fileKey: "abc123",
			nodeIDs: []string{"1:1", "1:2"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, "/files/abc123/nodes")
				assert.Contains(t, r.URL.RawQuery, "ids=1:1,1:2")
				response := map[string]interface{}{
					"nodes": map[string]interface{}{
						"1:1": map[string]interface{}{
							"document": map[string]interface{}{
								"id":   "1:1",
								"name": "Node 1",
								"type": "FRAME",
							},
						},
						"1:2": map[string]interface{}{
							"document": map[string]interface{}{
								"id":   "1:2",
								"name": "Node 2",
								"type": "TEXT",
							},
						},
					},
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(response)
			},
			expectError: false,
		},
		{
			name:    "get failure",
			fileKey: "invalid",
			nodeIDs: []string{"1:1"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			nodes, err := adapter.GetFileNodes(context.Background(), tt.fileKey, tt.nodeIDs)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, nodes)
				assert.Len(t, nodes, 2)
			}
		})
	}
}

func TestFigmaAdapter_GetImages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		fileKey       string
		nodeIDs       []string
		format        string
		scale         float64
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:    "successful export with defaults",
			fileKey: "abc123",
			nodeIDs: []string{"1:1"},
			format:  "",
			scale:   0,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, "/images/abc123")
				assert.Contains(t, r.URL.RawQuery, "format=png")
				response := map[string]interface{}{
					"images": map[string]string{
						"1:1": "https://example.com/image.png",
					},
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(response)
			},
			expectError: false,
		},
		{
			name:    "export with custom format and scale",
			fileKey: "abc123",
			nodeIDs: []string{"1:1", "1:2"},
			format:  "svg",
			scale:   2.0,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.RawQuery, "format=svg")
				assert.Contains(t, r.URL.RawQuery, "scale=2")
				response := map[string]interface{}{
					"images": map[string]string{
						"1:1": "https://example.com/image1.svg",
						"1:2": "https://example.com/image2.svg",
					},
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(response)
			},
			expectError: false,
		},
		{
			name:    "export with API error",
			fileKey: "abc123",
			nodeIDs: []string{"invalid"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				response := map[string]interface{}{
					"images": nil,
					"err":    "Invalid node IDs",
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(response)
			},
			expectError: true,
		},
		{
			name:    "export failure",
			fileKey: "invalid",
			nodeIDs: []string{"1:1"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			images, err := adapter.GetImages(context.Background(), tt.fileKey, tt.nodeIDs, tt.format, tt.scale)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, images)
			}
		})
	}
}

func TestFigmaAdapter_GetComments(t *testing.T) {
	t.Parallel()
	expectedComments := []FigmaComment{
		{
			ID:        "comment1",
			Message:   "Test comment",
			FileKey:   "abc123",
			CreatedAt: "2024-01-01T00:00:00Z",
			User: FigmaUser{
				ID:     "user1",
				Handle: "testuser",
			},
		},
	}

	tests := []struct {
		name          string
		fileKey       string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:    "successful get",
			fileKey: "abc123",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/files/abc123/comments", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"comments": expectedComments,
				})
			},
			expectError: false,
		},
		{
			name:    "get failure",
			fileKey: "invalid",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			comments, err := adapter.GetComments(context.Background(), tt.fileKey)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, comments, 1)
			}
		})
	}
}

func TestFigmaAdapter_PostComment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		fileKey       string
		message       string
		position      *FigmaRect
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:     "post without position",
			fileKey:  "abc123",
			message:  "Test comment",
			position: nil,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, "Test comment", body["message"])
				assert.Nil(t, body["client_meta"])

				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(FigmaComment{
					ID:      "new-comment",
					Message: "Test comment",
				})
			},
			expectError: false,
		},
		{
			name:     "post with position",
			fileKey:  "abc123",
			message:  "Comment at position",
			position: &FigmaRect{X: 100, Y: 200},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.NotNil(t, body["client_meta"])

				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(FigmaComment{
					ID:      "positioned-comment",
					Message: "Comment at position",
				})
			},
			expectError: false,
		},
		{
			name:    "post failure",
			fileKey: "invalid",
			message: "Will fail",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"err":"Access denied"}`))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			comment, err := adapter.PostComment(context.Background(), tt.fileKey, tt.message, tt.position)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, comment)
			}
		})
	}
}

func TestFigmaAdapter_GetTeamProjects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		teamID        string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:   "successful get",
			teamID: "team123",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/teams/team123/projects", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(FigmaTeamProject{
					Name: "Test Team",
					Projects: []FigmaProject{
						{ID: "proj1", Name: "Project 1"},
						{ID: "proj2", Name: "Project 2"},
					},
				})
			},
			expectError: false,
		},
		{
			name:   "get failure",
			teamID: "invalid",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			projects, err := adapter.GetTeamProjects(context.Background(), tt.teamID)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, projects)
				assert.Len(t, projects.Projects, 2)
			}
		})
	}
}

func TestFigmaAdapter_GetProjectFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		projectID     string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:      "successful get",
			projectID: "proj123",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/projects/proj123/files", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"files": []FigmaFile{
						{Name: "File 1"},
						{Name: "File 2"},
					},
				})
			},
			expectError: false,
		},
		{
			name:      "get failure",
			projectID: "invalid",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			files, err := adapter.GetProjectFiles(context.Background(), tt.projectID)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, files, 2)
			}
		})
	}
}

func TestFigmaAdapter_GetFileComponents(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		fileKey       string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:    "successful get",
			fileKey: "abc123",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/files/abc123/components", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"meta": map[string]interface{}{
						"components": map[string]FigmaComponent{
							"comp1": {Key: "comp1", Name: "Button"},
							"comp2": {Key: "comp2", Name: "Card"},
						},
					},
				})
			},
			expectError: false,
		},
		{
			name:    "get failure",
			fileKey: "invalid",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			components, err := adapter.GetFileComponents(context.Background(), tt.fileKey)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, components, 2)
			}
		})
	}
}

func TestFigmaAdapter_GetFileStyles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		fileKey       string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:    "successful get",
			fileKey: "abc123",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/files/abc123/styles", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"meta": map[string]interface{}{
						"styles": map[string]interface{}{
							"style1": map[string]interface{}{"name": "Primary Color"},
							"style2": map[string]interface{}{"name": "Heading"},
						},
					},
				})
			},
			expectError: false,
		},
		{
			name:    "get failure",
			fileKey: "invalid",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
			adapter.baseURL = server.URL

			styles, err := adapter.GetFileStyles(context.Background(), tt.fileKey)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, styles, 2)
			}
		})
	}
}

func TestFigmaAdapter_Close(t *testing.T) {
	t.Parallel()
	adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
	adapter.connected = true

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.IsConnected())
}

func TestFigmaAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
	tools := adapter.GetMCPTools()

	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	expectedTools := []string{
		"figma_get_file",
		"figma_get_file_nodes",
		"figma_export_images",
		"figma_get_comments",
		"figma_post_comment",
		"figma_get_components",
		"figma_get_styles",
		"figma_get_team_projects",
		"figma_get_project_files",
	}

	for _, expected := range expectedTools {
		assert.True(t, toolNames[expected], "Expected tool %s not found", expected)
	}
}

func TestFigmaAdapter_doRequest_WithAuth(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Figma-Token")
		assert.Equal(t, "test-token", token)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "12345"})
	}))
	defer server.Close()

	adapter := NewFigmaAdapter(FigmaAdapterConfig{APIToken: "test-token"})
	adapter.baseURL = server.URL

	err := adapter.Health(context.Background())
	require.NoError(t, err)
}

func TestJoinIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ids      []string
		expected string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"1:1"}, "1:1"},
		{"multiple", []string{"1:1", "1:2", "1:3"}, "1:1,1:2,1:3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := joinIDs(tt.ids)
			assert.Equal(t, tt.expected, result)
		})
	}
}
