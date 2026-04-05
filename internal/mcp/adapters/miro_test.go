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

func TestNewMiroAdapter(t *testing.T) {
	t.Parallel()
	config := MiroConfig{
		AccessToken: "test-token",
	}

	adapter := NewMiroAdapter(config)

	assert.NotNil(t, adapter)
	assert.Equal(t, "test-token", adapter.config.AccessToken)
	assert.Equal(t, DefaultMiroConfig().BaseURL, adapter.config.BaseURL)
}

func TestMiroAdapter_GetServerInfo(t *testing.T) {
	t.Parallel()
	adapter := NewMiroAdapter(MiroConfig{})

	info := adapter.GetServerInfo()

	assert.Equal(t, "miro", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "Miro")
	assert.NotEmpty(t, info.Capabilities)
	assert.Contains(t, info.Capabilities, "list_boards")
	assert.Contains(t, info.Capabilities, "create_sticky_note")
}

func TestMiroAdapter_ListTools(t *testing.T) {
	t.Parallel()
	adapter := NewMiroAdapter(MiroConfig{})

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	assert.True(t, toolNames["miro_list_boards"])
	assert.True(t, toolNames["miro_create_board"])
	assert.True(t, toolNames["miro_get_board"])
	assert.True(t, toolNames["miro_create_sticky_note"])
	assert.True(t, toolNames["miro_create_shape"])
	assert.True(t, toolNames["miro_create_text"])
	assert.True(t, toolNames["miro_create_connector"])
	assert.True(t, toolNames["miro_list_items"])
	assert.True(t, toolNames["miro_get_item"])
	assert.True(t, toolNames["miro_delete_item"])
	assert.True(t, toolNames["miro_create_frame"])
	assert.True(t, toolNames["miro_export_board"])
}

func TestMiroAdapter_CallTool_UnknownTool(t *testing.T) {
	t.Parallel()
	adapter := NewMiroAdapter(MiroConfig{})

	result, err := adapter.CallTool(context.Background(), "unknown_tool", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestMiroAdapter_ListBoards(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/boards", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer")

		response := MiroBoardsResponse{
			Data: []MiroBoard{
				{ID: "board-1", Name: "Sprint Planning", CreatedAt: "2024-01-01T00:00:00Z", ViewLink: "https://miro.com/board-1"},
				{ID: "board-2", Name: "Design Review", CreatedAt: "2024-01-02T00:00:00Z", ViewLink: "https://miro.com/board-2"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_list_boards", map[string]interface{}{
		"limit": 50,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Sprint Planning")
	assert.Contains(t, result.Content[0].Text, "Design Review")
}

func TestMiroAdapter_CreateBoard(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/boards", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "New Board", body["name"])

		response := MiroBoard{
			ID:       "new-board-id",
			Name:     "New Board",
			ViewLink: "https://miro.com/new-board-id",
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_create_board", map[string]interface{}{
		"name":        "New Board",
		"description": "A test board",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "New Board")
	assert.Contains(t, result.Content[0].Text, "new-board-id")
}

func TestMiroAdapter_GetBoard(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/boards/board-1", r.URL.Path)

		response := MiroBoard{
			ID:          "board-1",
			Name:        "Sprint Planning",
			Description: "Q1 Sprint Planning Board",
			CreatedAt:   "2024-01-01T00:00:00Z",
			ModifiedAt:  "2024-01-02T00:00:00Z",
			ViewLink:    "https://miro.com/board-1",
			Owner:       &MiroUser{ID: "user-1", Name: "John Doe"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_get_board", map[string]interface{}{
		"board_id": "board-1",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Sprint Planning")
	assert.Contains(t, result.Content[0].Text, "John Doe")
}

func TestMiroAdapter_CreateStickyNote(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/boards/board-1/sticky_notes", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		data := body["data"].(map[string]interface{})
		assert.Equal(t, "Todo: Fix bug", data["content"])

		response := MiroItem{
			ID:   "sticky-1",
			Type: "sticky_note",
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_create_sticky_note", map[string]interface{}{
		"board_id": "board-1",
		"content":  "Todo: Fix bug",
		"x":        100.0,
		"y":        200.0,
		"color":    "yellow",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "sticky-1")
}

func TestMiroAdapter_CreateShape(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/boards/board-1/shapes", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		data := body["data"].(map[string]interface{})
		assert.Equal(t, "rectangle", data["shape"])

		response := MiroItem{
			ID:   "shape-1",
			Type: "shape",
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_create_shape", map[string]interface{}{
		"board_id":     "board-1",
		"shape":        "rectangle",
		"content":      "Process Step",
		"x":            0.0,
		"y":            0.0,
		"width":        200.0,
		"height":       100.0,
		"fill_color":   "#ffffff",
		"border_color": "#000000",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "shape-1")
}

func TestMiroAdapter_CreateText(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/boards/board-1/texts", r.URL.Path)

		response := MiroItem{
			ID:   "text-1",
			Type: "text",
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_create_text", map[string]interface{}{
		"board_id":  "board-1",
		"content":   "Title: Project Overview",
		"x":         0.0,
		"y":         0.0,
		"font_size": 24,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "text-1")
}

func TestMiroAdapter_CreateConnector(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/boards/board-1/connectors", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		startItem := body["startItem"].(map[string]interface{})
		endItem := body["endItem"].(map[string]interface{})
		assert.Equal(t, "item-1", startItem["id"])
		assert.Equal(t, "item-2", endItem["id"])

		response := MiroItem{
			ID:   "connector-1",
			Type: "connector",
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_create_connector", map[string]interface{}{
		"board_id":      "board-1",
		"start_item_id": "item-1",
		"end_item_id":   "item-2",
		"style":         "elbowed",
		"start_cap":     "none",
		"end_cap":       "stealth",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "connector-1")
}

func TestMiroAdapter_ListItems(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/boards/board-1/items", r.URL.Path)

		response := MiroItemsResponse{
			Data: []MiroItem{
				{ID: "item-1", Type: "sticky_note", Position: &MiroPosition{X: 100, Y: 200}},
				{ID: "item-2", Type: "shape", Position: &MiroPosition{X: 300, Y: 400}},
				{ID: "item-3", Type: "text", Position: &MiroPosition{X: 500, Y: 600}},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_list_items", map[string]interface{}{
		"board_id": "board-1",
		"limit":    50,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "sticky_note")
	assert.Contains(t, result.Content[0].Text, "shape")
}

func TestMiroAdapter_GetItem(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/boards/board-1/items/item-1", r.URL.Path)

		response := MiroItem{
			ID:         "item-1",
			Type:       "sticky_note",
			CreatedAt:  "2024-01-01T00:00:00Z",
			ModifiedAt: "2024-01-02T00:00:00Z",
			Position:   &MiroPosition{X: 100, Y: 200},
			Geometry:   &MiroGeometry{Width: 228, Height: 228},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_get_item", map[string]interface{}{
		"board_id": "board-1",
		"item_id":  "item-1",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "item-1")
	assert.Contains(t, result.Content[0].Text, "sticky_note")
}

func TestMiroAdapter_DeleteItem(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/boards/board-1/items/item-1", r.URL.Path)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_delete_item", map[string]interface{}{
		"board_id": "board-1",
		"item_id":  "item-1",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "deleted")
}

func TestMiroAdapter_CreateFrame(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/boards/board-1/frames", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		data := body["data"].(map[string]interface{})
		assert.Equal(t, "Sprint 1", data["title"])

		response := MiroItem{
			ID:   "frame-1",
			Type: "frame",
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_create_frame", map[string]interface{}{
		"board_id": "board-1",
		"title":    "Sprint 1",
		"x":        0.0,
		"y":        0.0,
		"width":    800.0,
		"height":   600.0,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Sprint 1")
	assert.Contains(t, result.Content[0].Text, "frame-1")
}

func TestMiroAdapter_ExportBoard(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/boards/board-1", r.URL.Path)

		response := MiroBoard{
			ID:       "board-1",
			Name:     "Sprint Planning",
			ViewLink: "https://miro.com/board-1",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "test-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_export_board", map[string]interface{}{
		"board_id": "board-1",
		"format":   "png",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "https://miro.com/board-1")
}

func TestMiroAdapter_APIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message": "Invalid token"}`))
	}))
	defer server.Close()

	adapter := NewMiroAdapter(MiroConfig{
		AccessToken: "invalid-token",
		BaseURL:     server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "miro_list_boards", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "API error")
}

func TestDefaultMiroConfig(t *testing.T) {
	t.Parallel()
	config := DefaultMiroConfig()

	assert.Equal(t, "https://api.miro.com/v2", config.BaseURL)
	assert.Equal(t, 30*time.Second, config.Timeout)
}
