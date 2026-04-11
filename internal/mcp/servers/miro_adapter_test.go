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

func TestNewMiroAdapter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		config MiroConfig
		want   struct {
			baseURL string
			timeout time.Duration
		}
	}{
		{
			name: "with_custom_config",
			config: MiroConfig{
				AccessToken: "test-token",
				BaseURL:     "https://custom.miro.com/v2",
				Timeout:     60 * time.Second,
			},
			want: struct {
				baseURL string
				timeout time.Duration
			}{
				baseURL: "https://custom.miro.com/v2",
				timeout: 60 * time.Second,
			},
		},
		{
			name: "with_default_values",
			config: MiroConfig{
				AccessToken: "test-token",
			},
			want: struct {
				baseURL string
				timeout time.Duration
			}{
				baseURL: "https://api.miro.com/v2",
				timeout: 30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			adapter := NewMiroAdapter(tt.config)
			assert.NotNil(t, adapter)
			assert.Equal(t, tt.want.baseURL, adapter.baseURL)
			assert.Equal(t, tt.want.timeout, adapter.client.Timeout)
		})
	}
}

func TestMiroAdapter_Connect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		statusCode  int
		response    interface{}
		wantErr     bool
		errContains string
	}{
		{
			name:       "successful_connection",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":   "user123",
				"name": "Test User",
			},
			wantErr: false,
		},
		{
			name:        "authentication_failure",
			statusCode:  http.StatusUnauthorized,
			response:    map[string]string{"error": "invalid_token"},
			wantErr:     true,
			errContains: "authentication failed",
		},
		{
			name:        "server_error",
			statusCode:  http.StatusInternalServerError,
			response:    map[string]string{"error": "internal error"},
			wantErr:     true,
			errContains: "failed to authenticate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/users/me", r.URL.Path)
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			err := adapter.Connect(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.True(t, adapter.connected)
			}
		})
	}
}

func TestMiroAdapter_Health(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		connected  bool
		statusCode int
		wantErr    bool
	}{
		{
			name:       "healthy",
			connected:  true,
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "not_connected",
			connected:  false,
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name:       "unhealthy",
			connected:  true,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})
			adapter.connected = tt.connected

			err := adapter.Health(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMiroAdapter_ListBoards(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		teamID     string
		limit      int
		cursor     string
		statusCode int
		response   interface{}
		wantBoards int
		wantCursor string
		wantErr    bool
	}{
		{
			name:       "successful_list",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "board1", "name": "Board 1"},
					{"id": "board2", "name": "Board 2"},
				},
				"cursor": "next_cursor",
			},
			wantBoards: 2,
			wantCursor: "next_cursor",
			wantErr:    false,
		},
		{
			name:       "with_team_filter",
			teamID:     "team123",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "board1", "name": "Team Board"},
				},
			},
			wantBoards: 1,
			wantErr:    false,
		},
		{
			name:       "list_failure",
			statusCode: http.StatusForbidden,
			response:   map[string]string{"error": "forbidden"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/boards")

				if tt.teamID != "" {
					assert.Equal(t, tt.teamID, r.URL.Query().Get("team_id"))
				}

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			boards, cursor, err := adapter.ListBoards(context.Background(), tt.teamID, tt.limit, tt.cursor)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, boards, tt.wantBoards)
				assert.Equal(t, tt.wantCursor, cursor)
			}
		})
	}
}

func TestMiroAdapter_GetBoard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		boardID     string
		statusCode  int
		response    interface{}
		wantErr     bool
		errContains string
	}{
		{
			name:       "successful_get",
			boardID:    "board123",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":          "board123",
				"name":        "Test Board",
				"description": "A test board",
			},
			wantErr: false,
		},
		{
			name:        "board_not_found",
			boardID:     "nonexistent",
			statusCode:  http.StatusNotFound,
			response:    map[string]string{"error": "not found"},
			wantErr:     true,
			errContains: "board not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/boards/"+tt.boardID)

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			board, err := adapter.GetBoard(context.Background(), tt.boardID)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.boardID, board.ID)
				assert.Equal(t, "Test Board", board.Name)
			}
		})
	}
}

func TestMiroAdapter_CreateBoard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		boardName   string
		description string
		teamID      string
		statusCode  int
		response    interface{}
		wantErr     bool
	}{
		{
			name:        "successful_create",
			boardName:   "New Board",
			description: "Description",
			statusCode:  http.StatusCreated,
			response: map[string]interface{}{
				"id":          "new_board_id",
				"name":        "New Board",
				"description": "Description",
			},
			wantErr: false,
		},
		{
			name:       "create_with_team",
			boardName:  "Team Board",
			teamID:     "team123",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":   "team_board_id",
				"name": "Team Board",
			},
			wantErr: false,
		},
		{
			name:       "create_failure",
			boardName:  "Failed Board",
			statusCode: http.StatusBadRequest,
			response:   map[string]string{"error": "invalid request"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/boards", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, tt.boardName, body["name"])

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			board, err := adapter.CreateBoard(context.Background(), tt.boardName, tt.description, tt.teamID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, board.ID)
				assert.Equal(t, tt.boardName, board.Name)
			}
		})
	}
}

func TestMiroAdapter_UpdateBoard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		boardID    string
		newName    string
		newDesc    string
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "successful_update",
			boardID:    "board123",
			newName:    "Updated Name",
			newDesc:    "Updated Description",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":          "board123",
				"name":        "Updated Name",
				"description": "Updated Description",
			},
			wantErr: false,
		},
		{
			name:       "update_failure",
			boardID:    "board123",
			newName:    "New Name",
			statusCode: http.StatusForbidden,
			response:   map[string]string{"error": "forbidden"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PATCH", r.Method)
				assert.Contains(t, r.URL.Path, "/boards/"+tt.boardID)

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			board, err := adapter.UpdateBoard(context.Background(), tt.boardID, tt.newName, tt.newDesc)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.newName, board.Name)
			}
		})
	}
}

func TestMiroAdapter_DeleteBoard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		boardID    string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful_delete",
			boardID:    "board123",
			statusCode: http.StatusNoContent,
			wantErr:    false,
		},
		{
			name:       "delete_with_ok_status",
			boardID:    "board456",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "delete_failure",
			boardID:    "board789",
			statusCode: http.StatusForbidden,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "DELETE", r.Method)
				assert.Contains(t, r.URL.Path, "/boards/"+tt.boardID)

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			err := adapter.DeleteBoard(context.Background(), tt.boardID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMiroAdapter_GetBoardItems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		boardID    string
		itemType   string
		limit      int
		statusCode int
		response   interface{}
		wantItems  int
		wantErr    bool
	}{
		{
			name:       "successful_get_all_items",
			boardID:    "board123",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "item1", "type": "sticky_note"},
					{"id": "item2", "type": "shape"},
					{"id": "item3", "type": "text"},
				},
			},
			wantItems: 3,
			wantErr:   false,
		},
		{
			name:       "get_items_with_type_filter",
			boardID:    "board123",
			itemType:   "sticky_note",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "item1", "type": "sticky_note"},
				},
			},
			wantItems: 1,
			wantErr:   false,
		},
		{
			name:       "get_items_failure",
			boardID:    "board123",
			statusCode: http.StatusNotFound,
			response:   map[string]string{"error": "not found"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/boards/"+tt.boardID+"/items")

				if tt.itemType != "" {
					assert.Equal(t, tt.itemType, r.URL.Query().Get("type"))
				}

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			items, _, err := adapter.GetBoardItems(context.Background(), tt.boardID, tt.itemType, tt.limit, "")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, items, tt.wantItems)
			}
		})
	}
}

func TestMiroAdapter_GetItem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		boardID     string
		itemID      string
		statusCode  int
		response    interface{}
		wantErr     bool
		errContains string
	}{
		{
			name:       "successful_get",
			boardID:    "board123",
			itemID:     "item456",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":   "item456",
				"type": "sticky_note",
			},
			wantErr: false,
		},
		{
			name:        "item_not_found",
			boardID:     "board123",
			itemID:      "nonexistent",
			statusCode:  http.StatusNotFound,
			response:    map[string]string{"error": "not found"},
			wantErr:     true,
			errContains: "item not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			item, err := adapter.GetItem(context.Background(), tt.boardID, tt.itemID)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.itemID, item.ID)
			}
		})
	}
}

func TestMiroAdapter_CreateStickyNote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		boardID    string
		content    string
		position   *MiroPosition
		style      *MiroStickyNoteStyle
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "successful_create",
			boardID:    "board123",
			content:    "Test sticky note",
			statusCode: http.StatusCreated,
			response: map[string]interface{}{
				"id":   "sticky1",
				"type": "sticky_note",
				"data": map[string]interface{}{
					"content": "Test sticky note",
				},
			},
			wantErr: false,
		},
		{
			name:     "create_with_position_and_style",
			boardID:  "board123",
			content:  "Styled note",
			position: &MiroPosition{X: 100, Y: 200},
			style: &MiroStickyNoteStyle{
				FillColor: "yellow",
				TextAlign: "center",
			},
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":   "sticky2",
				"type": "sticky_note",
			},
			wantErr: false,
		},
		{
			name:       "create_failure",
			boardID:    "board123",
			content:    "Failed note",
			statusCode: http.StatusBadRequest,
			response:   map[string]string{"error": "invalid request"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/sticky_notes")

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			note, err := adapter.CreateStickyNote(context.Background(), tt.boardID, tt.content, tt.position, tt.style)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, note.ID)
			}
		})
	}
}

func TestMiroAdapter_CreateShape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		boardID    string
		shapeType  string
		content    string
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "successful_create_rectangle",
			boardID:    "board123",
			shapeType:  "rectangle",
			content:    "Rectangle content",
			statusCode: http.StatusCreated,
			response: map[string]interface{}{
				"id":   "shape1",
				"type": "shape",
				"data": map[string]interface{}{
					"shape":   "rectangle",
					"content": "Rectangle content",
				},
			},
			wantErr: false,
		},
		{
			name:       "create_circle_without_content",
			boardID:    "board123",
			shapeType:  "circle",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":   "shape2",
				"type": "shape",
			},
			wantErr: false,
		},
		{
			name:       "create_failure",
			boardID:    "board123",
			shapeType:  "invalid",
			statusCode: http.StatusBadRequest,
			response:   map[string]string{"error": "invalid shape"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/shapes")

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			shape, err := adapter.CreateShape(context.Background(), tt.boardID, tt.shapeType, tt.content, nil, nil, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, shape.ID)
			}
		})
	}
}

func TestMiroAdapter_CreateConnector(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		boardID     string
		startItemID string
		endItemID   string
		statusCode  int
		response    interface{}
		wantErr     bool
	}{
		{
			name:        "successful_create",
			boardID:     "board123",
			startItemID: "item1",
			endItemID:   "item2",
			statusCode:  http.StatusCreated,
			response: map[string]interface{}{
				"id":   "connector1",
				"type": "connector",
				"startItem": map[string]interface{}{
					"id": "item1",
				},
				"endItem": map[string]interface{}{
					"id": "item2",
				},
			},
			wantErr: false,
		},
		{
			name:        "create_failure",
			boardID:     "board123",
			startItemID: "nonexistent",
			endItemID:   "item2",
			statusCode:  http.StatusBadRequest,
			response:    map[string]string{"error": "item not found"},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/connectors")

				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)

				startItem := body["startItem"].(map[string]interface{})
				endItem := body["endItem"].(map[string]interface{})
				assert.Equal(t, tt.startItemID, startItem["id"])
				assert.Equal(t, tt.endItemID, endItem["id"])

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			connector, err := adapter.CreateConnector(context.Background(), tt.boardID, tt.startItemID, tt.endItemID, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, connector.ID)
			}
		})
	}
}

func TestMiroAdapter_CreateFrame(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		boardID    string
		title      string
		position   *MiroPosition
		geometry   *MiroGeometry
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "successful_create",
			boardID:    "board123",
			title:      "Test Frame",
			statusCode: http.StatusCreated,
			response: map[string]interface{}{
				"id":   "frame1",
				"type": "frame",
				"data": map[string]interface{}{
					"title": "Test Frame",
				},
			},
			wantErr: false,
		},
		{
			name:       "create_with_geometry",
			boardID:    "board123",
			title:      "Sized Frame",
			position:   &MiroPosition{X: 0, Y: 0},
			geometry:   &MiroGeometry{Width: 1000, Height: 800},
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":   "frame2",
				"type": "frame",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/frames")

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			frame, err := adapter.CreateFrame(context.Background(), tt.boardID, tt.title, tt.position, tt.geometry, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, frame.ID)
			}
		})
	}
}

func TestMiroAdapter_CreateText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		boardID    string
		content    string
		position   *MiroPosition
		style      *MiroTextStyle
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "successful_create",
			boardID:    "board123",
			content:    "Hello World",
			statusCode: http.StatusCreated,
			response: map[string]interface{}{
				"id":   "text1",
				"type": "text",
				"data": map[string]interface{}{
					"content": "Hello World",
				},
			},
			wantErr: false,
		},
		{
			name:     "create_with_style",
			boardID:  "board123",
			content:  "Styled Text",
			position: &MiroPosition{X: 50, Y: 50},
			style: &MiroTextStyle{
				FontSize:  24,
				Color:     "#000000",
				TextAlign: "center",
			},
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":   "text2",
				"type": "text",
			},
			wantErr: false,
		},
		{
			name:       "create_failure",
			boardID:    "board123",
			content:    "Failed text",
			statusCode: http.StatusForbidden,
			response:   map[string]string{"error": "forbidden"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/texts")

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			text, err := adapter.CreateText(context.Background(), tt.boardID, tt.content, tt.position, tt.style)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, text.ID)
			}
		})
	}
}

func TestMiroAdapter_DeleteItem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		boardID    string
		itemID     string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful_delete",
			boardID:    "board123",
			itemID:     "item456",
			statusCode: http.StatusNoContent,
			wantErr:    false,
		},
		{
			name:       "delete_with_ok_status",
			boardID:    "board123",
			itemID:     "item789",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "delete_failure",
			boardID:    "board123",
			itemID:     "protected",
			statusCode: http.StatusForbidden,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "DELETE", r.Method)
				assert.Contains(t, r.URL.Path, "/items/"+tt.itemID)

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			adapter := NewMiroAdapter(MiroConfig{
				AccessToken: "test-token",
				BaseURL:     server.URL,
			})

			err := adapter.DeleteItem(context.Background(), tt.boardID, tt.itemID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMiroAdapter_Close(t *testing.T) {
	t.Parallel()
	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
	})
	adapter.connected = true

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.connected)
}

func TestMiroAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
	})

	tools := adapter.GetMCPTools()
	assert.NotEmpty(t, tools)
	assert.Equal(t, 11, len(tools))

	// Verify tool names
	expectedTools := []string{
		"miro_list_boards",
		"miro_get_board",
		"miro_create_board",
		"miro_delete_board",
		"miro_get_board_items",
		"miro_create_sticky_note",
		"miro_create_shape",
		"miro_create_connector",
		"miro_create_frame",
		"miro_create_text",
		"miro_delete_item",
	}

	for i, expected := range expectedTools {
		assert.Equal(t, expected, tools[i].Name)
	}
}

func TestMiroAdapter_AuthHeader(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Bearer token
		assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "user1"})
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-access-token",
		BaseURL:     server.URL,
	})

	err := adapter.Connect(context.Background())
	assert.NoError(t, err)
}
