// Package adapters provides MCP server adapters.
package adapters

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAsanaClient implements AsanaClient for testing.
type MockAsanaClient struct {
	Tasks       []AsanaTask
	Projects    []AsanaProject
	Sections    []AsanaSection
	Workspaces  []AsanaWorkspace
	Users       []AsanaUser
	Tags        []AsanaTag
	Stories     []AsanaStory
	CurrentUser *AsanaUser
	GetTaskErr  error
}

func (m *MockAsanaClient) GetTask(ctx context.Context, taskID string) (*AsanaTask, error) {
	if m.GetTaskErr != nil {
		return nil, m.GetTaskErr
	}
	for _, task := range m.Tasks {
		if task.GID == taskID {
			return &task, nil
		}
	}
	return nil, errors.New("task not found")
}

func (m *MockAsanaClient) ListTasks(ctx context.Context, filter AsanaTaskFilter) ([]AsanaTask, error) {
	return m.Tasks, nil
}

func (m *MockAsanaClient) CreateTask(ctx context.Context, input CreateAsanaTaskInput) (*AsanaTask, error) {
	return &AsanaTask{
		GID:          "new-task-gid",
		Name:         input.Name,
		PermalinkURL: "https://app.asana.com/0/0/new-task-gid",
	}, nil
}

func (m *MockAsanaClient) UpdateTask(ctx context.Context, taskID string, input UpdateAsanaTaskInput) (*AsanaTask, error) {
	name := "Updated Task"
	if input.Name != nil {
		name = *input.Name
	}
	return &AsanaTask{GID: taskID, Name: name}, nil
}

func (m *MockAsanaClient) DeleteTask(ctx context.Context, taskID string) error {
	return nil
}

func (m *MockAsanaClient) CompleteTask(ctx context.Context, taskID string) error {
	return nil
}

func (m *MockAsanaClient) ListProjects(ctx context.Context, workspaceID string) ([]AsanaProject, error) {
	return m.Projects, nil
}

func (m *MockAsanaClient) GetProject(ctx context.Context, projectID string) (*AsanaProject, error) {
	for _, project := range m.Projects {
		if project.GID == projectID {
			return &project, nil
		}
	}
	return nil, errors.New("project not found")
}

func (m *MockAsanaClient) CreateProject(ctx context.Context, input CreateAsanaProjectInput) (*AsanaProject, error) {
	return &AsanaProject{
		GID:          "new-project-gid",
		Name:         input.Name,
		PermalinkURL: "https://app.asana.com/0/new-project-gid",
	}, nil
}

func (m *MockAsanaClient) ListProjectTasks(ctx context.Context, projectID string) ([]AsanaTask, error) {
	return m.Tasks, nil
}

func (m *MockAsanaClient) ListSections(ctx context.Context, projectID string) ([]AsanaSection, error) {
	return m.Sections, nil
}

func (m *MockAsanaClient) CreateSection(ctx context.Context, projectID, name string) (*AsanaSection, error) {
	return &AsanaSection{GID: "new-section-gid", Name: name}, nil
}

func (m *MockAsanaClient) AddTaskToSection(ctx context.Context, sectionID, taskID string) error {
	return nil
}

func (m *MockAsanaClient) ListWorkspaces(ctx context.Context) ([]AsanaWorkspace, error) {
	return m.Workspaces, nil
}

func (m *MockAsanaClient) ListUsers(ctx context.Context, workspaceID string) ([]AsanaUser, error) {
	return m.Users, nil
}

func (m *MockAsanaClient) GetMe(ctx context.Context) (*AsanaUser, error) {
	if m.CurrentUser != nil {
		return m.CurrentUser, nil
	}
	return &AsanaUser{
		GID:   "me-gid",
		Name:  "Test User",
		Email: "test@example.com",
	}, nil
}

func (m *MockAsanaClient) ListTags(ctx context.Context, workspaceID string) ([]AsanaTag, error) {
	return m.Tags, nil
}

func (m *MockAsanaClient) AddTagToTask(ctx context.Context, taskID, tagID string) error {
	return nil
}

func (m *MockAsanaClient) RemoveTagFromTask(ctx context.Context, taskID, tagID string) error {
	return nil
}

func (m *MockAsanaClient) ListSubtasks(ctx context.Context, taskID string) ([]AsanaTask, error) {
	return m.Tasks, nil
}

func (m *MockAsanaClient) CreateSubtask(ctx context.Context, parentTaskID string, input CreateAsanaTaskInput) (*AsanaTask, error) {
	return &AsanaTask{GID: "subtask-gid", Name: input.Name}, nil
}

func (m *MockAsanaClient) ListStories(ctx context.Context, taskID string) ([]AsanaStory, error) {
	return m.Stories, nil
}

func (m *MockAsanaClient) AddComment(ctx context.Context, taskID, text string) (*AsanaStory, error) {
	return &AsanaStory{GID: "comment-gid", Text: text, Type: "comment"}, nil
}

func (m *MockAsanaClient) SearchTasks(ctx context.Context, workspaceID, query string) ([]AsanaTask, error) {
	return m.Tasks, nil
}

func TestAsanaAdapter_GetServerInfo(t *testing.T) {
	t.Parallel()
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), &MockAsanaClient{})
	info := adapter.GetServerInfo()

	assert.Equal(t, "asana", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.NotEmpty(t, info.Description)
	assert.Contains(t, info.Capabilities, "tasks")
	assert.Contains(t, info.Capabilities, "projects")
}

func TestAsanaAdapter_ListTools(t *testing.T) {
	t.Parallel()
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), &MockAsanaClient{})
	tools := adapter.ListTools()

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["asana_get_task"])
	assert.True(t, toolNames["asana_list_tasks"])
	assert.True(t, toolNames["asana_create_task"])
	assert.True(t, toolNames["asana_update_task"])
	assert.True(t, toolNames["asana_complete_task"])
	assert.True(t, toolNames["asana_list_projects"])
	assert.True(t, toolNames["asana_list_workspaces"])
	assert.True(t, toolNames["asana_add_comment"])
}

func TestAsanaAdapter_GetTask(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		client := &MockAsanaClient{
			Tasks: []AsanaTask{
				{
					GID:          "task-1",
					Name:         "Test Task",
					Completed:    false,
					PermalinkURL: "https://app.asana.com/0/0/task-1",
				},
			},
		}
		adapter := NewAsanaAdapter(DefaultAsanaConfig(), client)

		result, err := adapter.CallTool(context.Background(), "asana_get_task", map[string]interface{}{
			"task_id": "task-1",
		})

		require.NoError(t, err)
		assert.False(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "Test Task")
	})

	t.Run("not found", func(t *testing.T) {
			t.Parallel()
		client := &MockAsanaClient{Tasks: []AsanaTask{}}
		adapter := NewAsanaAdapter(DefaultAsanaConfig(), client)

		result, err := adapter.CallTool(context.Background(), "asana_get_task", map[string]interface{}{
			"task_id": "notfound",
		})

		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestAsanaAdapter_ListTasks(t *testing.T) {
	t.Parallel()
	client := &MockAsanaClient{
		Tasks: []AsanaTask{
			{GID: "1", Name: "Task 1", Completed: false},
			{GID: "2", Name: "Task 2", Completed: true},
		},
	}
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), client)

	result, err := adapter.CallTool(context.Background(), "asana_list_tasks", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 tasks")
}

func TestAsanaAdapter_CreateTask(t *testing.T) {
	t.Parallel()
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), &MockAsanaClient{})

	result, err := adapter.CallTool(context.Background(), "asana_create_task", map[string]interface{}{
		"name": "New Task",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Created task")
}

func TestAsanaAdapter_UpdateTask(t *testing.T) {
	t.Parallel()
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), &MockAsanaClient{})

	result, err := adapter.CallTool(context.Background(), "asana_update_task", map[string]interface{}{
		"task_id": "task-1",
		"name":    "Updated Name",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Updated task")
}

func TestAsanaAdapter_CompleteTask(t *testing.T) {
	t.Parallel()
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), &MockAsanaClient{})

	result, err := adapter.CallTool(context.Background(), "asana_complete_task", map[string]interface{}{
		"task_id": "task-1",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "complete")
}

func TestAsanaAdapter_ListProjects(t *testing.T) {
	t.Parallel()
	client := &MockAsanaClient{
		Projects: []AsanaProject{
			{GID: "proj-1", Name: "Project Alpha", Archived: false},
			{GID: "proj-2", Name: "Project Beta", Archived: true},
		},
	}
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), client)

	result, err := adapter.CallTool(context.Background(), "asana_list_projects", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 projects")
}

func TestAsanaAdapter_ListWorkspaces(t *testing.T) {
	t.Parallel()
	client := &MockAsanaClient{
		Workspaces: []AsanaWorkspace{
			{GID: "ws-1", Name: "My Workspace", IsOrganization: false},
			{GID: "ws-2", Name: "Company Org", IsOrganization: true},
		},
	}
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), client)

	result, err := adapter.CallTool(context.Background(), "asana_list_workspaces", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 workspaces")
}

func TestAsanaAdapter_ListSections(t *testing.T) {
	t.Parallel()
	client := &MockAsanaClient{
		Sections: []AsanaSection{
			{GID: "sec-1", Name: "To Do"},
			{GID: "sec-2", Name: "In Progress"},
			{GID: "sec-3", Name: "Done"},
		},
	}
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), client)

	result, err := adapter.CallTool(context.Background(), "asana_list_sections", map[string]interface{}{
		"project_id": "proj-1",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 3 sections")
}

func TestAsanaAdapter_AddComment(t *testing.T) {
	t.Parallel()
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), &MockAsanaClient{})

	result, err := adapter.CallTool(context.Background(), "asana_add_comment", map[string]interface{}{
		"task_id": "task-1",
		"text":    "This is a comment",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Added comment")
}

func TestAsanaAdapter_GetMe(t *testing.T) {
	t.Parallel()
	client := &MockAsanaClient{
		CurrentUser: &AsanaUser{
			GID:   "user-1",
			Name:  "John Doe",
			Email: "john@example.com",
		},
	}
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), client)

	result, err := adapter.CallTool(context.Background(), "asana_get_me", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "John Doe")
}

func TestAsanaAdapter_SearchTasks(t *testing.T) {
	t.Parallel()
	client := &MockAsanaClient{
		Tasks: []AsanaTask{
			{GID: "1", Name: "Bug fix", Completed: false},
		},
	}
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), client)

	result, err := adapter.CallTool(context.Background(), "asana_search_tasks", map[string]interface{}{
		"query": "bug",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 1 tasks")
}

func TestAsanaAdapter_CreateSubtask(t *testing.T) {
	t.Parallel()
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), &MockAsanaClient{})

	result, err := adapter.CallTool(context.Background(), "asana_create_subtask", map[string]interface{}{
		"parent_task_id": "task-1",
		"name":           "Subtask 1",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Created subtask")
}

func TestAsanaAdapter_UnknownTool(t *testing.T) {
	t.Parallel()
	adapter := NewAsanaAdapter(DefaultAsanaConfig(), &MockAsanaClient{})

	_, err := adapter.CallTool(context.Background(), "unknown_tool", map[string]interface{}{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestDefaultAsanaConfig(t *testing.T) {
	t.Parallel()
	config := DefaultAsanaConfig()
	assert.Equal(t, 30*time.Second, config.Timeout)
}
