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

// MockJiraClient implements JiraClient for testing.
type MockJiraClient struct {
	Issues      []JiraIssue
	Projects    []JiraProject
	Sprints     []JiraSprint
	Boards      []JiraBoard
	Transitions []JiraTransition
	Comments    []JiraComment
	Users       []JiraUser
	IssueTypes  []JiraIssueType
	Priorities  []JiraPriority
	Statuses    []JiraStatus
	CurrentUser *JiraUser
	GetIssueErr error
}

func (m *MockJiraClient) GetIssue(ctx context.Context, issueKey string) (*JiraIssue, error) {
	if m.GetIssueErr != nil {
		return nil, m.GetIssueErr
	}
	for _, issue := range m.Issues {
		if issue.Key == issueKey {
			return &issue, nil
		}
	}
	return nil, errors.New("issue not found")
}

func (m *MockJiraClient) SearchIssues(ctx context.Context, jql string, maxResults int) ([]JiraIssue, error) {
	return m.Issues, nil
}

func (m *MockJiraClient) CreateIssue(ctx context.Context, input CreateJiraIssueInput) (*JiraIssue, error) {
	return &JiraIssue{
		ID:  "10001",
		Key: input.ProjectKey + "-123",
		Fields: JiraIssueFields{
			Summary: input.Summary,
		},
	}, nil
}

func (m *MockJiraClient) UpdateIssue(ctx context.Context, issueKey string, input UpdateJiraIssueInput) (*JiraIssue, error) {
	return &JiraIssue{Key: issueKey}, nil
}

func (m *MockJiraClient) DeleteIssue(ctx context.Context, issueKey string) error {
	return nil
}

func (m *MockJiraClient) AssignIssue(ctx context.Context, issueKey, accountID string) error {
	return nil
}

func (m *MockJiraClient) GetTransitions(ctx context.Context, issueKey string) ([]JiraTransition, error) {
	return m.Transitions, nil
}

func (m *MockJiraClient) TransitionIssue(ctx context.Context, issueKey, transitionID string) error {
	return nil
}

func (m *MockJiraClient) ListComments(ctx context.Context, issueKey string) ([]JiraComment, error) {
	return m.Comments, nil
}

func (m *MockJiraClient) AddComment(ctx context.Context, issueKey, body string) (*JiraComment, error) {
	return &JiraComment{ID: "comment-1", Body: body}, nil
}

func (m *MockJiraClient) ListProjects(ctx context.Context) ([]JiraProject, error) {
	return m.Projects, nil
}

func (m *MockJiraClient) GetProject(ctx context.Context, projectKey string) (*JiraProject, error) {
	for _, project := range m.Projects {
		if project.Key == projectKey {
			return &project, nil
		}
	}
	return nil, errors.New("project not found")
}

func (m *MockJiraClient) ListSprints(ctx context.Context, boardID int) ([]JiraSprint, error) {
	return m.Sprints, nil
}

func (m *MockJiraClient) GetActiveSprint(ctx context.Context, boardID int) (*JiraSprint, error) {
	for _, sprint := range m.Sprints {
		if sprint.State == "active" {
			return &sprint, nil
		}
	}
	return nil, errors.New("no active sprint")
}

func (m *MockJiraClient) ListBoards(ctx context.Context, projectKey string) ([]JiraBoard, error) {
	return m.Boards, nil
}

func (m *MockJiraClient) SearchUsers(ctx context.Context, query string) ([]JiraUser, error) {
	return m.Users, nil
}

func (m *MockJiraClient) GetCurrentUser(ctx context.Context) (*JiraUser, error) {
	if m.CurrentUser != nil {
		return m.CurrentUser, nil
	}
	return &JiraUser{
		AccountID:    "me-id",
		DisplayName:  "Test User",
		EmailAddress: "test@example.com",
	}, nil
}

func (m *MockJiraClient) ListIssueTypes(ctx context.Context, projectKey string) ([]JiraIssueType, error) {
	return m.IssueTypes, nil
}

func (m *MockJiraClient) ListPriorities(ctx context.Context) ([]JiraPriority, error) {
	return m.Priorities, nil
}

func (m *MockJiraClient) ListStatuses(ctx context.Context, projectKey string) ([]JiraStatus, error) {
	return m.Statuses, nil
}

func (m *MockJiraClient) ListComponents(ctx context.Context, projectKey string) ([]JiraComponent, error) {
	return nil, nil
}

func (m *MockJiraClient) ListVersions(ctx context.Context, projectKey string) ([]JiraVersion, error) {
	return nil, nil
}

func (m *MockJiraClient) ListWorklogs(ctx context.Context, issueKey string) ([]JiraWorklog, error) {
	return nil, nil
}

func (m *MockJiraClient) AddWorklog(ctx context.Context, issueKey string, input AddWorklogInput) (*JiraWorklog, error) {
	return &JiraWorklog{ID: "worklog-1", TimeSpent: input.TimeSpent}, nil
}

func (m *MockJiraClient) ListAttachments(ctx context.Context, issueKey string) ([]JiraAttachment, error) {
	return nil, nil
}

func (m *MockJiraClient) AddWatcher(ctx context.Context, issueKey, accountID string) error {
	return nil
}

func (m *MockJiraClient) RemoveWatcher(ctx context.Context, issueKey, accountID string) error {
	return nil
}

func TestJiraAdapter_GetServerInfo(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, &MockJiraClient{})
	info := adapter.GetServerInfo()

	assert.Equal(t, "jira", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.NotEmpty(t, info.Description)
	assert.Contains(t, info.Capabilities, "issues")
	assert.Contains(t, info.Capabilities, "projects")
	assert.Contains(t, info.Capabilities, "sprints")
}

func TestJiraAdapter_ListTools(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, &MockJiraClient{})
	tools := adapter.ListTools()

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["jira_get_issue"])
	assert.True(t, toolNames["jira_search_issues"])
	assert.True(t, toolNames["jira_create_issue"])
	assert.True(t, toolNames["jira_update_issue"])
	assert.True(t, toolNames["jira_transition_issue"])
	assert.True(t, toolNames["jira_add_comment"])
	assert.True(t, toolNames["jira_list_projects"])
	assert.True(t, toolNames["jira_list_sprints"])
}

func TestJiraAdapter_GetIssue(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		client := &MockJiraClient{
			Issues: []JiraIssue{
				{
					ID:  "10001",
					Key: "PROJ-123",
					Fields: JiraIssueFields{
						Summary: "Test Issue",
						Status:  &JiraStatus{Name: "Open"},
					},
				},
			},
		}
		config := DefaultJiraConfig()
		config.BaseURL = "https://test.atlassian.net"
		adapter := NewJiraAdapter(config, client)

		result, err := adapter.CallTool(context.Background(), "jira_get_issue", map[string]interface{}{
			"issue_key": "PROJ-123",
		})

		require.NoError(t, err)
		assert.False(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "PROJ-123")
		assert.Contains(t, result.Content[0].Text, "Test Issue")
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		client := &MockJiraClient{Issues: []JiraIssue{}}
		config := DefaultJiraConfig()
		config.BaseURL = "https://test.atlassian.net"
		adapter := NewJiraAdapter(config, client)

		result, err := adapter.CallTool(context.Background(), "jira_get_issue", map[string]interface{}{
			"issue_key": "NOTFOUND",
		})

		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestJiraAdapter_SearchIssues(t *testing.T) {
	t.Parallel()
	client := &MockJiraClient{
		Issues: []JiraIssue{
			{Key: "PROJ-1", Fields: JiraIssueFields{Summary: "Issue 1", Status: &JiraStatus{Name: "Open"}}},
			{Key: "PROJ-2", Fields: JiraIssueFields{Summary: "Issue 2", Status: &JiraStatus{Name: "Done"}}},
		},
	}
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, client)

	result, err := adapter.CallTool(context.Background(), "jira_search_issues", map[string]interface{}{
		"jql": "project = PROJ",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 issues")
}

func TestJiraAdapter_CreateIssue(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, &MockJiraClient{})

	result, err := adapter.CallTool(context.Background(), "jira_create_issue", map[string]interface{}{
		"project_key": "PROJ",
		"summary":     "New Issue",
		"issue_type":  "Bug",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Created issue PROJ-123")
}

func TestJiraAdapter_UpdateIssue(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, &MockJiraClient{})

	result, err := adapter.CallTool(context.Background(), "jira_update_issue", map[string]interface{}{
		"issue_key": "PROJ-123",
		"summary":   "Updated Summary",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Updated issue")
}

func TestJiraAdapter_TransitionIssue(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, &MockJiraClient{})

	result, err := adapter.CallTool(context.Background(), "jira_transition_issue", map[string]interface{}{
		"issue_key":     "PROJ-123",
		"transition_id": "21",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "transitioned successfully")
}

func TestJiraAdapter_GetTransitions(t *testing.T) {
	t.Parallel()
	client := &MockJiraClient{
		Transitions: []JiraTransition{
			{ID: "11", Name: "To Do", To: &JiraStatus{Name: "To Do"}},
			{ID: "21", Name: "In Progress", To: &JiraStatus{Name: "In Progress"}},
			{ID: "31", Name: "Done", To: &JiraStatus{Name: "Done"}},
		},
	}
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, client)

	result, err := adapter.CallTool(context.Background(), "jira_get_transitions", map[string]interface{}{
		"issue_key": "PROJ-123",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Available transitions")
	assert.Contains(t, result.Content[0].Text, "To Do")
	assert.Contains(t, result.Content[0].Text, "In Progress")
}

func TestJiraAdapter_AssignIssue(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, &MockJiraClient{})

	result, err := adapter.CallTool(context.Background(), "jira_assign_issue", map[string]interface{}{
		"issue_key":  "PROJ-123",
		"account_id": "user-id",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "assigned")
}

func TestJiraAdapter_AddComment(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, &MockJiraClient{})

	result, err := adapter.CallTool(context.Background(), "jira_add_comment", map[string]interface{}{
		"issue_key": "PROJ-123",
		"body":      "This is a comment",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Added comment")
}

func TestJiraAdapter_ListProjects(t *testing.T) {
	t.Parallel()
	client := &MockJiraClient{
		Projects: []JiraProject{
			{Key: "PROJ1", Name: "Project One", ProjectType: "software"},
			{Key: "PROJ2", Name: "Project Two", ProjectType: "business"},
		},
	}
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, client)

	result, err := adapter.CallTool(context.Background(), "jira_list_projects", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 projects")
}

func TestJiraAdapter_ListSprints(t *testing.T) {
	t.Parallel()
	client := &MockJiraClient{
		Sprints: []JiraSprint{
			{ID: 1, Name: "Sprint 1", State: "closed", StartDate: "2024-01-01T00:00:00Z", EndDate: "2024-01-14T00:00:00Z"},
			{ID: 2, Name: "Sprint 2", State: "active", StartDate: "2024-01-15T00:00:00Z", EndDate: "2024-01-28T00:00:00Z"},
		},
	}
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, client)

	result, err := adapter.CallTool(context.Background(), "jira_list_sprints", map[string]interface{}{
		"board_id": 1,
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 sprints")
}

func TestJiraAdapter_ListBoards(t *testing.T) {
	t.Parallel()
	client := &MockJiraClient{
		Boards: []JiraBoard{
			{ID: 1, Name: "Scrum Board", Type: "scrum"},
			{ID: 2, Name: "Kanban Board", Type: "kanban"},
		},
	}
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, client)

	result, err := adapter.CallTool(context.Background(), "jira_list_boards", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 boards")
}

func TestJiraAdapter_GetMe(t *testing.T) {
	t.Parallel()
	client := &MockJiraClient{
		CurrentUser: &JiraUser{
			AccountID:    "user-1",
			DisplayName:  "John Doe",
			EmailAddress: "john@example.com",
		},
	}
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, client)

	result, err := adapter.CallTool(context.Background(), "jira_get_me", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "John Doe")
}

func TestJiraAdapter_ListIssueTypes(t *testing.T) {
	t.Parallel()
	client := &MockJiraClient{
		IssueTypes: []JiraIssueType{
			{ID: "1", Name: "Bug", Subtask: false},
			{ID: "2", Name: "Story", Subtask: false},
			{ID: "3", Name: "Sub-task", Subtask: true},
		},
	}
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, client)

	result, err := adapter.CallTool(context.Background(), "jira_list_issue_types", map[string]interface{}{
		"project_key": "PROJ",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 3 issue types")
}

func TestJiraAdapter_ListPriorities(t *testing.T) {
	t.Parallel()
	client := &MockJiraClient{
		Priorities: []JiraPriority{
			{ID: "1", Name: "Highest"},
			{ID: "2", Name: "High"},
			{ID: "3", Name: "Medium"},
			{ID: "4", Name: "Low"},
		},
	}
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, client)

	result, err := adapter.CallTool(context.Background(), "jira_list_priorities", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 4 priorities")
}

func TestJiraAdapter_AddWorklog(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, &MockJiraClient{})

	result, err := adapter.CallTool(context.Background(), "jira_add_worklog", map[string]interface{}{
		"issue_key":  "PROJ-123",
		"time_spent": "2h 30m",
		"comment":    "Worked on feature",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Logged")
}

func TestJiraAdapter_AddWatcher(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, &MockJiraClient{})

	result, err := adapter.CallTool(context.Background(), "jira_add_watcher", map[string]interface{}{
		"issue_key":  "PROJ-123",
		"account_id": "user-id",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Added watcher")
}

func TestJiraAdapter_UnknownTool(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	config.BaseURL = "https://test.atlassian.net"
	adapter := NewJiraAdapter(config, &MockJiraClient{})

	_, err := adapter.CallTool(context.Background(), "unknown_tool", map[string]interface{}{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestDefaultJiraConfig(t *testing.T) {
	t.Parallel()
	config := DefaultJiraConfig()
	assert.Equal(t, 30*time.Second, config.Timeout)
}
