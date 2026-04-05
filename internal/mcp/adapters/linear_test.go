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

// MockLinearClient implements LinearClient for testing.
type MockLinearClient struct {
	Issues         []LinearIssue
	Teams          []LinearTeam
	Projects       []LinearProject
	Cycles         []LinearCycle
	Labels         []LinearLabel
	Users          []LinearUser
	Comments       []LinearComment
	States         []LinearWorkflowState
	CurrentUser    *LinearUser
	CreatedIssue   *LinearIssue
	UpdatedIssue   *LinearIssue
	CreatedProject *LinearProject
	CreatedComment *LinearComment
	GetIssueErr    error
	ListIssuesErr  error
	CreateIssueErr error
}

func (m *MockLinearClient) GetIssue(ctx context.Context, id string) (*LinearIssue, error) {
	if m.GetIssueErr != nil {
		return nil, m.GetIssueErr
	}
	for _, issue := range m.Issues {
		if issue.ID == id || issue.Identifier == id {
			return &issue, nil
		}
	}
	return nil, errors.New("issue not found")
}

func (m *MockLinearClient) ListIssues(ctx context.Context, filter IssueFilter) ([]LinearIssue, error) {
	if m.ListIssuesErr != nil {
		return nil, m.ListIssuesErr
	}
	return m.Issues, nil
}

func (m *MockLinearClient) CreateIssue(ctx context.Context, input CreateIssueInput) (*LinearIssue, error) {
	if m.CreateIssueErr != nil {
		return nil, m.CreateIssueErr
	}
	if m.CreatedIssue != nil {
		return m.CreatedIssue, nil
	}
	return &LinearIssue{
		ID:         "new-id",
		Identifier: "ENG-123",
		Title:      input.Title,
		URL:        "https://linear.app/issue/ENG-123",
	}, nil
}

func (m *MockLinearClient) UpdateIssue(ctx context.Context, id string, input UpdateIssueInput) (*LinearIssue, error) {
	if m.UpdatedIssue != nil {
		return m.UpdatedIssue, nil
	}
	return &LinearIssue{ID: id, Identifier: "ENG-123"}, nil
}

func (m *MockLinearClient) DeleteIssue(ctx context.Context, id string) error {
	return nil
}

func (m *MockLinearClient) ListTeams(ctx context.Context) ([]LinearTeam, error) {
	return m.Teams, nil
}

func (m *MockLinearClient) GetTeam(ctx context.Context, id string) (*LinearTeam, error) {
	for _, team := range m.Teams {
		if team.ID == id {
			return &team, nil
		}
	}
	return nil, errors.New("team not found")
}

func (m *MockLinearClient) ListProjects(ctx context.Context, teamID string) ([]LinearProject, error) {
	return m.Projects, nil
}

func (m *MockLinearClient) GetProject(ctx context.Context, id string) (*LinearProject, error) {
	for _, project := range m.Projects {
		if project.ID == id {
			return &project, nil
		}
	}
	return nil, errors.New("project not found")
}

func (m *MockLinearClient) CreateProject(ctx context.Context, input CreateProjectInput) (*LinearProject, error) {
	if m.CreatedProject != nil {
		return m.CreatedProject, nil
	}
	return &LinearProject{
		ID:   "new-project",
		Name: input.Name,
		URL:  "https://linear.app/project/new",
	}, nil
}

func (m *MockLinearClient) ListCycles(ctx context.Context, teamID string) ([]LinearCycle, error) {
	return m.Cycles, nil
}

func (m *MockLinearClient) GetActiveCycle(ctx context.Context, teamID string) (*LinearCycle, error) {
	for _, cycle := range m.Cycles {
		if cycle.Name == "Active" {
			return &cycle, nil
		}
	}
	return nil, errors.New("no active cycle")
}

func (m *MockLinearClient) ListLabels(ctx context.Context, teamID string) ([]LinearLabel, error) {
	return m.Labels, nil
}

func (m *MockLinearClient) ListUsers(ctx context.Context) ([]LinearUser, error) {
	return m.Users, nil
}

func (m *MockLinearClient) GetMe(ctx context.Context) (*LinearUser, error) {
	if m.CurrentUser != nil {
		return m.CurrentUser, nil
	}
	return &LinearUser{
		ID:          "me-id",
		Name:        "Test User",
		DisplayName: "Test User",
		Email:       "test@example.com",
	}, nil
}

func (m *MockLinearClient) ListComments(ctx context.Context, issueID string) ([]LinearComment, error) {
	return m.Comments, nil
}

func (m *MockLinearClient) CreateComment(ctx context.Context, issueID, body string) (*LinearComment, error) {
	if m.CreatedComment != nil {
		return m.CreatedComment, nil
	}
	return &LinearComment{
		ID:   "comment-id",
		Body: body,
	}, nil
}

func (m *MockLinearClient) ListWorkflowStates(ctx context.Context, teamID string) ([]LinearWorkflowState, error) {
	return m.States, nil
}

func (m *MockLinearClient) SearchIssues(ctx context.Context, query string, limit int) ([]LinearIssue, error) {
	return m.Issues, nil
}

func TestLinearAdapter_GetServerInfo(t *testing.T) {
	t.Parallel()
	adapter := NewLinearAdapter(DefaultLinearConfig(), &MockLinearClient{})
	info := adapter.GetServerInfo()

	assert.Equal(t, "linear", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.NotEmpty(t, info.Description)
	assert.Contains(t, info.Capabilities, "issues")
	assert.Contains(t, info.Capabilities, "projects")
}

func TestLinearAdapter_ListTools(t *testing.T) {
	t.Parallel()
	adapter := NewLinearAdapter(DefaultLinearConfig(), &MockLinearClient{})
	tools := adapter.ListTools()

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["linear_get_issue"])
	assert.True(t, toolNames["linear_list_issues"])
	assert.True(t, toolNames["linear_create_issue"])
	assert.True(t, toolNames["linear_update_issue"])
	assert.True(t, toolNames["linear_list_teams"])
	assert.True(t, toolNames["linear_list_projects"])
	assert.True(t, toolNames["linear_add_comment"])
	assert.True(t, toolNames["linear_search_issues"])
}

func TestLinearAdapter_GetIssue(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		client := &MockLinearClient{
			Issues: []LinearIssue{
				{
					ID:         "issue-1",
					Identifier: "ENG-123",
					Title:      "Test Issue",
					URL:        "https://linear.app/issue/ENG-123",
					State:      &LinearWorkflowState{Name: "In Progress"},
				},
			},
		}
		adapter := NewLinearAdapter(DefaultLinearConfig(), client)

		result, err := adapter.CallTool(context.Background(), "linear_get_issue", map[string]interface{}{
			"id": "ENG-123",
		})

		require.NoError(t, err)
		assert.False(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "ENG-123")
		assert.Contains(t, result.Content[0].Text, "Test Issue")
	})

	t.Run("not found", func(t *testing.T) {
			t.Parallel()
		client := &MockLinearClient{Issues: []LinearIssue{}}
		adapter := NewLinearAdapter(DefaultLinearConfig(), client)

		result, err := adapter.CallTool(context.Background(), "linear_get_issue", map[string]interface{}{
			"id": "NOTFOUND",
		})

		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestLinearAdapter_ListIssues(t *testing.T) {
	t.Parallel()
	client := &MockLinearClient{
		Issues: []LinearIssue{
			{ID: "1", Identifier: "ENG-1", Title: "Issue 1", State: &LinearWorkflowState{Name: "Open"}},
			{ID: "2", Identifier: "ENG-2", Title: "Issue 2", State: &LinearWorkflowState{Name: "Done"}},
		},
	}
	adapter := NewLinearAdapter(DefaultLinearConfig(), client)

	result, err := adapter.CallTool(context.Background(), "linear_list_issues", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 issues")
	assert.Contains(t, result.Content[0].Text, "ENG-1")
	assert.Contains(t, result.Content[0].Text, "ENG-2")
}

func TestLinearAdapter_CreateIssue(t *testing.T) {
	t.Parallel()
	client := &MockLinearClient{}
	adapter := NewLinearAdapter(DefaultLinearConfig(), client)

	result, err := adapter.CallTool(context.Background(), "linear_create_issue", map[string]interface{}{
		"title":   "New Issue",
		"team_id": "team-1",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Created issue ENG-123")
}

func TestLinearAdapter_UpdateIssue(t *testing.T) {
	t.Parallel()
	client := &MockLinearClient{}
	adapter := NewLinearAdapter(DefaultLinearConfig(), client)

	result, err := adapter.CallTool(context.Background(), "linear_update_issue", map[string]interface{}{
		"id":    "issue-1",
		"title": "Updated Title",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Updated issue")
}

func TestLinearAdapter_ListTeams(t *testing.T) {
	t.Parallel()
	client := &MockLinearClient{
		Teams: []LinearTeam{
			{ID: "team-1", Name: "Engineering", Key: "ENG", IssueCount: 50},
			{ID: "team-2", Name: "Design", Key: "DES", IssueCount: 25},
		},
	}
	adapter := NewLinearAdapter(DefaultLinearConfig(), client)

	result, err := adapter.CallTool(context.Background(), "linear_list_teams", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 teams")
	assert.Contains(t, result.Content[0].Text, "Engineering")
	assert.Contains(t, result.Content[0].Text, "Design")
}

func TestLinearAdapter_ListProjects(t *testing.T) {
	t.Parallel()
	client := &MockLinearClient{
		Projects: []LinearProject{
			{ID: "proj-1", Name: "Project Alpha", State: "started", Progress: 0.5},
			{ID: "proj-2", Name: "Project Beta", State: "planned", Progress: 0.0},
		},
	}
	adapter := NewLinearAdapter(DefaultLinearConfig(), client)

	result, err := adapter.CallTool(context.Background(), "linear_list_projects", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 projects")
}

func TestLinearAdapter_ListCycles(t *testing.T) {
	t.Parallel()
	client := &MockLinearClient{
		Cycles: []LinearCycle{
			{ID: "cycle-1", Number: 1, Name: "Sprint 1", StartsAt: "2024-01-01T00:00:00Z", EndsAt: "2024-01-14T00:00:00Z", Progress: 0.8},
			{ID: "cycle-2", Number: 2, Name: "Sprint 2", StartsAt: "2024-01-15T00:00:00Z", EndsAt: "2024-01-28T00:00:00Z", Progress: 0.2},
		},
	}
	adapter := NewLinearAdapter(DefaultLinearConfig(), client)

	result, err := adapter.CallTool(context.Background(), "linear_list_cycles", map[string]interface{}{
		"team_id": "team-1",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 2 cycles")
}

func TestLinearAdapter_AddComment(t *testing.T) {
	t.Parallel()
	client := &MockLinearClient{}
	adapter := NewLinearAdapter(DefaultLinearConfig(), client)

	result, err := adapter.CallTool(context.Background(), "linear_add_comment", map[string]interface{}{
		"issue_id": "issue-1",
		"body":     "This is a comment",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Added comment")
}

func TestLinearAdapter_GetMe(t *testing.T) {
	t.Parallel()
	client := &MockLinearClient{
		CurrentUser: &LinearUser{
			ID:          "user-1",
			DisplayName: "John Doe",
			Email:       "john@example.com",
		},
	}
	adapter := NewLinearAdapter(DefaultLinearConfig(), client)

	result, err := adapter.CallTool(context.Background(), "linear_get_me", map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "John Doe")
	assert.Contains(t, result.Content[0].Text, "john@example.com")
}

func TestLinearAdapter_SearchIssues(t *testing.T) {
	t.Parallel()
	client := &MockLinearClient{
		Issues: []LinearIssue{
			{ID: "1", Identifier: "ENG-1", Title: "Bug fix", State: &LinearWorkflowState{Name: "Open"}},
		},
	}
	adapter := NewLinearAdapter(DefaultLinearConfig(), client)

	result, err := adapter.CallTool(context.Background(), "linear_search_issues", map[string]interface{}{
		"query": "bug",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Found 1 issues")
}

func TestLinearAdapter_UnknownTool(t *testing.T) {
	t.Parallel()
	adapter := NewLinearAdapter(DefaultLinearConfig(), &MockLinearClient{})

	_, err := adapter.CallTool(context.Background(), "unknown_tool", map[string]interface{}{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestDefaultLinearConfig(t *testing.T) {
	t.Parallel()
	config := DefaultLinearConfig()
	assert.Equal(t, 30*time.Second, config.Timeout)
}
