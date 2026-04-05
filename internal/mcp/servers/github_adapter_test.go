package servers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewGitHubAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, nil)

	assert.NotNil(t, adapter)
	assert.False(t, adapter.initialized)
	assert.Equal(t, "https://api.github.com", adapter.config.BaseURL)
}

func TestDefaultGitHubAdapterConfig(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()

	assert.Equal(t, "https://api.github.com", config.BaseURL)
	assert.Equal(t, 30*1000000000, int(config.Timeout)) // 30 seconds in nanoseconds
	assert.True(t, config.RateLimit)
	assert.Equal(t, 3, config.MaxRetries)
}

func TestNewGitHubAdapter_DefaultConfig(t *testing.T) {
	t.Parallel()
	config := GitHubAdapterConfig{}
	adapter := NewGitHubAdapter(config, logrus.New())

	assert.Equal(t, "https://api.github.com", adapter.config.BaseURL)
	assert.Equal(t, 3, adapter.config.MaxRetries)
}

func TestGitHubAdapter_Health_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	err := adapter.Health(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_Close(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestGitHubAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	tools := adapter.GetMCPTools()
	assert.Len(t, tools, 11)

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "github_get_user")
	assert.Contains(t, toolNames, "github_list_repos")
	assert.Contains(t, toolNames, "github_get_repo")
	assert.Contains(t, toolNames, "github_list_issues")
	assert.Contains(t, toolNames, "github_get_issue")
	assert.Contains(t, toolNames, "github_list_prs")
	assert.Contains(t, toolNames, "github_get_pr")
	assert.Contains(t, toolNames, "github_get_content")
	assert.Contains(t, toolNames, "github_search_repos")
	assert.Contains(t, toolNames, "github_search_code")
	assert.Contains(t, toolNames, "github_create_issue")
}

func TestGitHubAdapter_ExecuteTool_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.ExecuteTool(context.Background(), "github_get_user", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_ExecuteTool_UnknownTool(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	_, err := adapter.ExecuteTool(context.Background(), "unknown_tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestGitHubAdapter_GetCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	config.Token = "test-token"
	adapter := NewGitHubAdapter(config, logrus.New())

	caps := adapter.GetCapabilities()
	assert.Equal(t, "github", caps["name"])
	assert.Equal(t, "https://api.github.com", caps["base_url"])
	assert.Equal(t, true, caps["authenticated"])
	assert.Equal(t, 11, caps["tools"])
	assert.Equal(t, false, caps["initialized"])
}

func TestGitHubAdapter_GetCapabilities_NoToken(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	caps := adapter.GetCapabilities()
	assert.Equal(t, false, caps["authenticated"])
}

func TestGitHubAdapter_GetUser_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.GetUser(context.Background(), "testuser")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_ListRepositories_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.ListRepositories(context.Background(), "testuser", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_GetRepository_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.GetRepository(context.Background(), "owner", "repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_ListIssues_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.ListIssues(context.Background(), "owner", "repo", "open")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_GetIssue_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.GetIssue(context.Background(), "owner", "repo", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_ListPullRequests_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.ListPullRequests(context.Background(), "owner", "repo", "open")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_GetPullRequest_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.GetPullRequest(context.Background(), "owner", "repo", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_GetContent_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.GetContent(context.Background(), "owner", "repo", "path", "main")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_SearchRepositories_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.SearchRepositories(context.Background(), "test", "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_SearchCode_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.SearchCode(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitHubAdapter_CreateIssue_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	_, err := adapter.CreateIssue(context.Background(), "owner", "repo", "title", "body", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

// Mock server tests
func TestGitHubAdapter_Initialize_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"login": "testuser",
				"id":    123,
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	adapter := NewGitHubAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	assert.True(t, adapter.initialized)
}

func TestGitHubAdapter_Initialize_InvalidToken(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
		Token:   "invalid-token",
	}
	adapter := NewGitHubAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid GitHub token")
}

func TestGitHubAdapter_Health_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rate_limit" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"rate": map[string]interface{}{
					"limit":     5000,
					"remaining": 4999,
				},
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	err := adapter.Health(context.Background())
	assert.NoError(t, err)
}

func TestGitHubAdapter_GetUser_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/testuser" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"login":        "testuser",
				"id":           123,
				"name":         "Test User",
				"email":        "test@example.com",
				"public_repos": 10,
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	user, err := adapter.GetUser(context.Background(), "testuser")
	assert.NoError(t, err)
	assert.Equal(t, "testuser", user.Login)
	assert.Equal(t, int64(123), user.ID)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, 10, user.PublicRepos)
}

func TestGitHubAdapter_GetUser_NotFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	_, err := adapter.GetUser(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestGitHubAdapter_ListRepositories_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/testuser/repos" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":        1,
					"name":      "repo1",
					"full_name": "testuser/repo1",
				},
				{
					"id":        2,
					"name":      "repo2",
					"full_name": "testuser/repo2",
				},
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	repos, err := adapter.ListRepositories(context.Background(), "testuser", "")
	assert.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "repo1", repos[0].Name)
	assert.Equal(t, "repo2", repos[1].Name)
}

func TestGitHubAdapter_GetRepository_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/testuser/testrepo" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":               123,
				"name":             "testrepo",
				"full_name":        "testuser/testrepo",
				"description":      "A test repository",
				"stargazers_count": 100,
				"forks_count":      10,
				"default_branch":   "main",
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	repo, err := adapter.GetRepository(context.Background(), "testuser", "testrepo")
	assert.NoError(t, err)
	assert.Equal(t, "testrepo", repo.Name)
	assert.Equal(t, "A test repository", repo.Description)
	assert.Equal(t, 100, repo.StargazersCount)
	assert.Equal(t, "main", repo.DefaultBranch)
}

func TestGitHubAdapter_GetRepository_MissingParams(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	_, err := adapter.GetRepository(context.Background(), "", "repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "owner and repo are required")
}

func TestGitHubAdapter_ListIssues_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/testuser/testrepo/issues" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":     1,
					"number": 1,
					"title":  "Test Issue",
					"state":  "open",
				},
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	issues, err := adapter.ListIssues(context.Background(), "testuser", "testrepo", "open")
	assert.NoError(t, err)
	assert.Len(t, issues, 1)
	assert.Equal(t, "Test Issue", issues[0].Title)
}

func TestGitHubAdapter_GetIssue_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/testuser/testrepo/issues/1" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     1,
				"number": 1,
				"title":  "Test Issue",
				"state":  "open",
				"body":   "This is a test issue",
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	issue, err := adapter.GetIssue(context.Background(), "testuser", "testrepo", 1)
	assert.NoError(t, err)
	assert.Equal(t, "Test Issue", issue.Title)
	assert.Equal(t, "This is a test issue", issue.Body)
}

func TestGitHubAdapter_ListPullRequests_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/testuser/testrepo/pulls" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":     1,
					"number": 1,
					"title":  "Test PR",
					"state":  "open",
				},
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	prs, err := adapter.ListPullRequests(context.Background(), "testuser", "testrepo", "open")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	assert.Equal(t, "Test PR", prs[0].Title)
}

func TestGitHubAdapter_GetPullRequest_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/testuser/testrepo/pulls/1" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     1,
				"number": 1,
				"title":  "Test PR",
				"state":  "open",
				"merged": false,
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	pr, err := adapter.GetPullRequest(context.Background(), "testuser", "testrepo", 1)
	assert.NoError(t, err)
	assert.Equal(t, "Test PR", pr.Title)
	assert.False(t, pr.Merged)
}

func TestGitHubAdapter_GetContent_File_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/testuser/testrepo/contents/README.md" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"type":    "file",
				"name":    "README.md",
				"path":    "README.md",
				"sha":     "abc123",
				"size":    100,
				"content": "SGVsbG8gV29ybGQ=", // Base64 encoded "Hello World"
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	content, err := adapter.GetContent(context.Background(), "testuser", "testrepo", "README.md", "")
	assert.NoError(t, err)
	assert.Len(t, content, 1)
	assert.Equal(t, "file", content[0].Type)
	assert.Equal(t, "README.md", content[0].Name)
}

func TestGitHubAdapter_GetContent_Directory_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/testuser/testrepo/contents/src" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"type": "file",
					"name": "main.go",
					"path": "src/main.go",
				},
				{
					"type": "dir",
					"name": "utils",
					"path": "src/utils",
				},
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	content, err := adapter.GetContent(context.Background(), "testuser", "testrepo", "src", "")
	assert.NoError(t, err)
	assert.Len(t, content, 2)
}

func TestGitHubAdapter_SearchRepositories_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search/repositories" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 1,
				"items": []map[string]interface{}{
					{
						"id":        1,
						"name":      "test-repo",
						"full_name": "user/test-repo",
					},
				},
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	result, err := adapter.SearchRepositories(context.Background(), "test", "stars", "desc")
	assert.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Len(t, result.Items, 1)
}

func TestGitHubAdapter_SearchRepositories_EmptyQuery(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	_, err := adapter.SearchRepositories(context.Background(), "", "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")
}

func TestGitHubAdapter_SearchCode_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search/code" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total_count": 1,
				"items": []map[string]interface{}{
					{
						"name": "main.go",
						"path": "src/main.go",
						"sha":  "abc123",
					},
				},
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	result, err := adapter.SearchCode(context.Background(), "func main repo:test/test")
	assert.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
}

func TestGitHubAdapter_CreateIssue_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/testuser/testrepo/issues" && r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     1,
				"number": 1,
				"title":  "New Issue",
				"state":  "open",
			})
		}
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	issue, err := adapter.CreateIssue(context.Background(), "testuser", "testrepo", "New Issue", "Issue body", []string{"bug"})
	assert.NoError(t, err)
	assert.Equal(t, "New Issue", issue.Title)
	assert.Equal(t, 1, issue.Number)
}

func TestGitHubAdapter_CreateIssue_MissingParams(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	_, err := adapter.CreateIssue(context.Background(), "", "repo", "title", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "owner, repo, and title are required")
}

func TestGitHubAdapter_ExecuteTool_GetUser(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"login": "testuser",
			"id":    123,
		})
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	result, err := adapter.ExecuteTool(context.Background(), "github_get_user", map[string]interface{}{
		"username": "testuser",
	})
	assert.NoError(t, err)
	user := result.(*GitHubUser)
	assert.Equal(t, "testuser", user.Login)
}

func TestGitHubAdapter_ExecuteTool_ListRepos(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "repo1"},
		})
	}))
	defer server.Close()

	config := GitHubAdapterConfig{
		BaseURL: server.URL,
	}
	adapter := NewGitHubAdapter(config, logrus.New())
	adapter.initialized = true
	adapter.client = http.DefaultClient

	result, err := adapter.ExecuteTool(context.Background(), "github_list_repos", map[string]interface{}{
		"owner": "testuser",
	})
	assert.NoError(t, err)
	repos := result.([]GitHubRepository)
	assert.Len(t, repos, 1)
}

func TestGitHubAdapter_MarshalJSON(t *testing.T) {
	t.Parallel()
	config := DefaultGitHubAdapterConfig()
	adapter := NewGitHubAdapter(config, logrus.New())

	data, err := adapter.MarshalJSON()
	assert.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Contains(t, result, "initialized")
	assert.Contains(t, result, "capabilities")
}
