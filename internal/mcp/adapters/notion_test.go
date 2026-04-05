package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockNotionClient implements NotionClient for testing
type MockNotionClient struct {
	pages       []NotionPage
	databases   []NotionDatabase
	blocks      []NotionBlock
	comments    []NotionComment
	shouldError bool
}

func NewMockNotionClient() *MockNotionClient {
	return &MockNotionClient{
		pages: []NotionPage{
			{
				ID:             "page-1",
				Title:          "Getting Started",
				URL:            "https://notion.so/Getting-Started-page1",
				ParentID:       "workspace",
				ParentType:     "workspace",
				Properties:     map[string]interface{}{"status": "published"},
				CreatedTime:    time.Now().Add(-48 * time.Hour),
				LastEditedTime: time.Now().Add(-time.Hour),
				Archived:       false,
			},
			{
				ID:             "page-2",
				Title:          "Project Notes",
				URL:            "https://notion.so/Project-Notes-page2",
				ParentID:       "db-1",
				ParentType:     "database",
				Properties:     map[string]interface{}{"status": "draft"},
				CreatedTime:    time.Now().Add(-24 * time.Hour),
				LastEditedTime: time.Now(),
				Archived:       false,
			},
		},
		databases: []NotionDatabase{
			{
				ID:          "db-1",
				Title:       "Tasks",
				Description: "Project task tracker",
				Properties: map[string]NotionPropertyDef{
					"Name":   {ID: "title", Name: "Name", Type: "title"},
					"Status": {ID: "status", Name: "Status", Type: "select"},
					"Date":   {ID: "date", Name: "Date", Type: "date"},
				},
				URL: "https://notion.so/Tasks-db1",
			},
		},
		blocks: []NotionBlock{
			{ID: "block-1", Type: "paragraph", Content: "This is a paragraph"},
			{ID: "block-2", Type: "heading_1", Content: "Main Heading"},
			{ID: "block-3", Type: "bulleted_list_item", Content: "List item 1"},
		},
		comments: []NotionComment{
			{ID: "comment-1", Content: "Great work!", CreatedBy: "user-1", CreatedTime: time.Now()},
		},
	}
}

func (m *MockNotionClient) SetError(shouldError bool) {
	m.shouldError = shouldError
}

func (m *MockNotionClient) SearchPages(ctx context.Context, query string, filter NotionFilter) ([]NotionPage, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.pages, nil
}

func (m *MockNotionClient) GetPage(ctx context.Context, pageID string) (*NotionPage, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, p := range m.pages {
		if p.ID == pageID {
			return &p, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockNotionClient) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}, content []NotionBlock) (*NotionPage, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	title, _ := properties["title"].(string)
	return &NotionPage{
		ID:       "new-page-id",
		Title:    title,
		ParentID: parentID,
	}, nil
}

func (m *MockNotionClient) UpdatePage(ctx context.Context, pageID string, properties map[string]interface{}) (*NotionPage, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, p := range m.pages {
		if p.ID == pageID {
			return &p, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockNotionClient) DeletePage(ctx context.Context, pageID string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockNotionClient) GetDatabase(ctx context.Context, databaseID string) (*NotionDatabase, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, db := range m.databases {
		if db.ID == databaseID {
			return &db, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockNotionClient) QueryDatabase(ctx context.Context, databaseID string, filter, sorts interface{}) ([]NotionPage, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.pages, nil
}

func (m *MockNotionClient) AppendBlocks(ctx context.Context, pageID string, blocks []NotionBlock) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockNotionClient) GetBlocks(ctx context.Context, pageID string) ([]NotionBlock, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.blocks, nil
}

func (m *MockNotionClient) CreateComment(ctx context.Context, pageID, content string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockNotionClient) GetComments(ctx context.Context, pageID string) ([]NotionComment, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.comments, nil
}

// Tests

func TestNewNotionAdapter(t *testing.T) {
	t.Parallel()
	config := NotionConfig{
		APIKey:  "secret_xxxxx",
		Timeout: 30 * time.Second,
	}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	assert.NotNil(t, adapter)

	info := adapter.GetServerInfo()
	assert.Equal(t, "notion", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
}

func TestNotionAdapter_ListTools(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "notion_search")
	assert.Contains(t, toolNames, "notion_get_page")
	assert.Contains(t, toolNames, "notion_create_page")
	assert.Contains(t, toolNames, "notion_update_page")
	assert.Contains(t, toolNames, "notion_delete_page")
	assert.Contains(t, toolNames, "notion_get_database")
	assert.Contains(t, toolNames, "notion_query_database")
	assert.Contains(t, toolNames, "notion_get_blocks")
	assert.Contains(t, toolNames, "notion_append_blocks")
	assert.Contains(t, toolNames, "notion_add_comment")
}

func TestNotionAdapter_Search(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_search", map[string]interface{}{
		"query": "Getting Started",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestNotionAdapter_GetPage(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_get_page", map[string]interface{}{
		"page_id": "page-1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNotionAdapter_CreatePage(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_create_page", map[string]interface{}{
		"parent_id": "workspace",
		"title":     "New Page",
		"content":   "This is the page content",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNotionAdapter_UpdatePage(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_update_page", map[string]interface{}{
		"page_id":    "page-1",
		"properties": map[string]interface{}{"status": "published"},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNotionAdapter_DeletePage(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_delete_page", map[string]interface{}{
		"page_id": "page-1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNotionAdapter_GetDatabase(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_get_database", map[string]interface{}{
		"database_id": "db-1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNotionAdapter_QueryDatabase(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_query_database", map[string]interface{}{
		"database_id": "db-1",
		"filter":      map[string]interface{}{"property": "Status", "equals": "Done"},
		"sorts":       []interface{}{map[string]interface{}{"property": "Date", "direction": "descending"}},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNotionAdapter_GetBlocks(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_get_blocks", map[string]interface{}{
		"page_id": "page-1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNotionAdapter_AppendBlocks(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_append_blocks", map[string]interface{}{
		"page_id": "page-1",
		"content": "New paragraph content",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNotionAdapter_AddComment(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_add_comment", map[string]interface{}{
		"page_id": "page-1",
		"content": "This is a comment",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNotionAdapter_InvalidTool(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	_, err := adapter.CallTool(ctx, "invalid_tool", map[string]interface{}{})

	assert.Error(t, err)
}

func TestNotionAdapter_ErrorHandling(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	client.SetError(true)
	adapter := NewNotionAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "notion_search", map[string]interface{}{
		"query": "test",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Type tests

func TestNotionPageTypes(t *testing.T) {
	t.Parallel()
	page := NotionPage{
		ID:             "page-abc123",
		Title:          "My Page",
		URL:            "https://notion.so/My-Page-abc123",
		ParentID:       "workspace",
		ParentType:     "workspace",
		Properties:     map[string]interface{}{"status": "done", "priority": "high"},
		CreatedTime:    time.Now().Add(-24 * time.Hour),
		LastEditedTime: time.Now(),
		Archived:       false,
	}

	assert.Equal(t, "page-abc123", page.ID)
	assert.Equal(t, "My Page", page.Title)
	assert.Equal(t, "workspace", page.ParentType)
	assert.False(t, page.Archived)
}

func TestNotionDatabaseTypes(t *testing.T) {
	t.Parallel()
	database := NotionDatabase{
		ID:          "db-xyz789",
		Title:       "Task Database",
		Description: "Track all project tasks",
		Properties: map[string]NotionPropertyDef{
			"Name":     {ID: "title", Name: "Name", Type: "title"},
			"Status":   {ID: "status", Name: "Status", Type: "select"},
			"Assignee": {ID: "assignee", Name: "Assignee", Type: "people"},
		},
		URL: "https://notion.so/Task-Database-xyz789",
	}

	assert.Equal(t, "db-xyz789", database.ID)
	assert.Equal(t, "Task Database", database.Title)
	assert.Len(t, database.Properties, 3)
	assert.Equal(t, "select", database.Properties["Status"].Type)
}

func TestNotionPropertyDefTypes(t *testing.T) {
	t.Parallel()
	propDef := NotionPropertyDef{
		ID:   "prop-123",
		Name: "Due Date",
		Type: "date",
	}

	assert.Equal(t, "prop-123", propDef.ID)
	assert.Equal(t, "Due Date", propDef.Name)
	assert.Equal(t, "date", propDef.Type)
}

func TestNotionBlockTypes(t *testing.T) {
	t.Parallel()
	block := NotionBlock{
		ID:      "block-456",
		Type:    "paragraph",
		Content: "This is paragraph content",
		Children: []NotionBlock{
			{ID: "child-1", Type: "bulleted_list_item", Content: "Child item"},
		},
	}

	assert.Equal(t, "block-456", block.ID)
	assert.Equal(t, "paragraph", block.Type)
	assert.Len(t, block.Children, 1)
}

func TestNotionCommentTypes(t *testing.T) {
	t.Parallel()
	comment := NotionComment{
		ID:          "comment-789",
		Content:     "This is a great idea!",
		CreatedBy:   "user-123",
		CreatedTime: time.Now(),
	}

	assert.Equal(t, "comment-789", comment.ID)
	assert.Equal(t, "This is a great idea!", comment.Content)
	assert.Equal(t, "user-123", comment.CreatedBy)
}

func TestNotionFilterTypes(t *testing.T) {
	t.Parallel()
	filter := NotionFilter{
		Property: "Status",
		Value:    "In Progress",
	}

	assert.Equal(t, "Status", filter.Property)
	assert.Equal(t, "In Progress", filter.Value)
}

func TestNotionConfigTypes(t *testing.T) {
	t.Parallel()
	config := NotionConfig{
		APIKey:  "secret_xxxxxxxxxxxxxxxxxx",
		Timeout: 60 * time.Second,
	}

	assert.NotEmpty(t, config.APIKey)
	assert.Equal(t, 60*time.Second, config.Timeout)
}

func TestNotionAdapter_GetServerInfoCapabilities(t *testing.T) {
	t.Parallel()
	config := NotionConfig{}
	client := NewMockNotionClient()
	adapter := NewNotionAdapter(config, client)

	info := adapter.GetServerInfo()
	assert.Contains(t, info.Capabilities, "pages")
	assert.Contains(t, info.Capabilities, "databases")
	assert.Contains(t, info.Capabilities, "blocks")
	assert.Contains(t, info.Capabilities, "search")
	assert.Contains(t, info.Capabilities, "comments")
}
