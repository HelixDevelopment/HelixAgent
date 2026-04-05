package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Tests

func TestDefaultGitLabConfig(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()

	assert.Equal(t, "https://gitlab.com", config.BaseURL)
	assert.Equal(t, 60*time.Second, config.Timeout)
}

func TestNewGitLabAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	assert.NotNil(t, adapter)

	info := adapter.GetServerInfo()
	assert.Equal(t, "gitlab", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
}

func TestGitLabAdapter_ListTools(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "gitlab_list_projects")
	assert.Contains(t, toolNames, "gitlab_get_project")
	assert.Contains(t, toolNames, "gitlab_list_issues")
	assert.Contains(t, toolNames, "gitlab_create_issue")
	assert.Contains(t, toolNames, "gitlab_list_merge_requests")
	assert.Contains(t, toolNames, "gitlab_create_merge_request")
	assert.Contains(t, toolNames, "gitlab_get_file")
	assert.Contains(t, toolNames, "gitlab_list_pipelines")
	assert.Contains(t, toolNames, "gitlab_trigger_pipeline")
	assert.Contains(t, toolNames, "gitlab_list_branches")
}

func TestGitLabAdapter_ListProjects(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "gitlab_list_projects", map[string]interface{}{
		"owned":    true,
		"search":   "test",
		"per_page": 20,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestGitLabAdapter_GetProject(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "gitlab_get_project", map[string]interface{}{
		"project_id": "group/project",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGitLabAdapter_ListIssues(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "gitlab_list_issues", map[string]interface{}{
		"project_id": "123",
		"state":      "opened",
		"labels":     "bug,urgent",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGitLabAdapter_CreateIssue(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "gitlab_create_issue", map[string]interface{}{
		"project_id":  "123",
		"title":       "Test Issue",
		"description": "This is a test issue",
		"labels":      "test",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGitLabAdapter_ListMergeRequests(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "gitlab_list_merge_requests", map[string]interface{}{
		"project_id": "123",
		"state":      "opened",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGitLabAdapter_CreateMergeRequest(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "gitlab_create_merge_request", map[string]interface{}{
		"project_id":    "123",
		"source_branch": "feature-branch",
		"target_branch": "main",
		"title":         "Add new feature",
		"description":   "This MR adds a new feature",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGitLabAdapter_GetFile(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "gitlab_get_file", map[string]interface{}{
		"project_id": "123",
		"file_path":  "README.md",
		"ref":        "main",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGitLabAdapter_ListPipelines(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "gitlab_list_pipelines", map[string]interface{}{
		"project_id": "123",
		"status":     "success",
		"ref":        "main",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGitLabAdapter_TriggerPipeline(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "gitlab_trigger_pipeline", map[string]interface{}{
		"project_id": "123",
		"ref":        "main",
		"variables":  map[string]interface{}{"DEPLOY_ENV": "staging"},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGitLabAdapter_ListBranches(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "gitlab_list_branches", map[string]interface{}{
		"project_id": "123",
		"search":     "feature",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGitLabAdapter_InvalidTool(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	ctx := context.Background()
	_, err := adapter.CallTool(ctx, "invalid_tool", map[string]interface{}{})

	assert.Error(t, err)
}

// Type tests

func TestGitLabProjectTypes(t *testing.T) {
	t.Parallel()
	project := GitLabProject{
		ID:                1,
		Name:              "my-project",
		PathWithNamespace: "group/my-project",
		Description:       "A test project",
		DefaultBranch:     "main",
		WebURL:            "https://gitlab.com/group/my-project",
		CreatedAt:         time.Now().Add(-30 * 24 * time.Hour),
		LastActivityAt:    time.Now(),
	}

	assert.Equal(t, 1, project.ID)
	assert.Equal(t, "my-project", project.Name)
	assert.Equal(t, "group/my-project", project.PathWithNamespace)
	assert.Equal(t, "main", project.DefaultBranch)
}

func TestGitLabIssueTypes(t *testing.T) {
	t.Parallel()
	issue := GitLabIssue{
		ID:          1,
		IID:         42,
		Title:       "Bug report",
		Description: "Something is broken",
		State:       "opened",
		Labels:      []string{"bug", "high-priority"},
		WebURL:      "https://gitlab.com/group/project/-/issues/42",
		CreatedAt:   time.Now(),
	}

	assert.Equal(t, 42, issue.IID)
	assert.Equal(t, "Bug report", issue.Title)
	assert.Equal(t, "opened", issue.State)
	assert.Len(t, issue.Labels, 2)
}

func TestGitLabMergeRequestTypes(t *testing.T) {
	t.Parallel()
	mr := GitLabMergeRequest{
		ID:           1,
		IID:          10,
		Title:        "Add feature X",
		Description:  "This MR adds feature X",
		State:        "opened",
		SourceBranch: "feature-x",
		TargetBranch: "main",
		WebURL:       "https://gitlab.com/group/project/-/merge_requests/10",
		CreatedAt:    time.Now(),
	}

	assert.Equal(t, 10, mr.IID)
	assert.Equal(t, "Add feature X", mr.Title)
	assert.Equal(t, "feature-x", mr.SourceBranch)
	assert.Equal(t, "main", mr.TargetBranch)
}

func TestGitLabPipelineTypes(t *testing.T) {
	t.Parallel()
	pipeline := GitLabPipeline{
		ID:        100,
		Status:    "success",
		Ref:       "main",
		SHA:       "abc123def456",
		WebURL:    "https://gitlab.com/group/project/-/pipelines/100",
		CreatedAt: time.Now(),
	}

	assert.Equal(t, 100, pipeline.ID)
	assert.Equal(t, "success", pipeline.Status)
	assert.Equal(t, "main", pipeline.Ref)
}

func TestGitLabConfigTypes(t *testing.T) {
	t.Parallel()
	config := GitLabConfig{
		BaseURL: "https://gitlab.example.com",
		Token:   "glpat-xxxxxxxxxxxx",
		Timeout: 120 * time.Second,
	}

	assert.Equal(t, "https://gitlab.example.com", config.BaseURL)
	assert.NotEmpty(t, config.Token)
	assert.Equal(t, 120*time.Second, config.Timeout)
}

func TestGitLabProjectSerialize(t *testing.T) {
	t.Parallel()
	project := GitLabProject{
		ID:                1,
		Name:              "test-project",
		PathWithNamespace: "group/test-project",
	}

	data, err := project.Serialize()
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "test-project")
}

func TestFormatGitLabProjects(t *testing.T) {
	t.Parallel()
	projects := []GitLabProject{
		{Name: "project1", PathWithNamespace: "group/project1", DefaultBranch: "main", WebURL: "https://gitlab.com/group/project1"},
		{Name: "project2", PathWithNamespace: "group/project2", DefaultBranch: "develop", WebURL: "https://gitlab.com/group/project2"},
	}

	result := formatGitLabProjects(projects)
	assert.Contains(t, result, "Found 2 projects")
	assert.Contains(t, result, "project1")
	assert.Contains(t, result, "project2")
}

func TestGitLabAdapter_GetServerInfoCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultGitLabConfig()
	adapter := NewGitLabAdapter(config)

	info := adapter.GetServerInfo()
	assert.Contains(t, info.Capabilities, "repository_management")
	assert.Contains(t, info.Capabilities, "issue_tracking")
	assert.Contains(t, info.Capabilities, "merge_requests")
	assert.Contains(t, info.Capabilities, "ci_cd")
	assert.Contains(t, info.Capabilities, "user_management")
}
