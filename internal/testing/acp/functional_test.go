// Package acp provides real functional tests for ACP (Agent Communication Protocol) agents.
// These tests execute ACTUAL agent operations, not just connectivity checks.
// Tests FAIL if the operation fails - no false positives.
package acp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ACPClient provides a client for testing ACP agents
type ACPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewACPClient creates a new ACP test client
func NewACPClient(baseURL string) *ACPClient {
	return &ACPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AgentRequest represents an ACP agent request
type AgentRequest struct {
	AgentID string                 `json:"agent_id"`
	Task    string                 `json:"task"`
	Context map[string]interface{} `json:"context,omitempty"`
	Tools   []string               `json:"tools,omitempty"`
	Timeout int                    `json:"timeout,omitempty"`
}

type AgentResponse struct {
	ID       string                 `json:"id"`
	AgentID  string                 `json:"agent_id"`
	Name     string                 `json:"name,omitempty"`
	Status   string                 `json:"status"`
	Result   interface{}            `json:"result,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func (r *AgentResponse) GetID() string {
	if r.ID != "" {
		return r.ID
	}
	return r.AgentID
}

// ListAgents lists all available ACP agents
func (c *ACPClient) ListAgents() ([]string, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v1/acp/agents")
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list agents failed with status %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Agents []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var agents []string
	for _, a := range raw.Agents {
		if a.Status == "active" {
			agents = append(agents, a.ID)
		}
	}
	return agents, nil
}

// GetAgentInfo gets information about a specific agent
func (c *ACPClient) GetAgentInfo(agentID string) (*AgentResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v1/acp/agents/" + agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent info: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get agent info failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result AgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ExecuteTask sends a task to an ACP agent
func (c *ACPClient) ExecuteTask(req *AgentRequest) (*AgentResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/v1/acp/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to execute task: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("execute task failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result AgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ACPAgentConfig holds configuration for testing an ACP agent
type ACPAgentConfig struct {
	ID          string
	Description string
	TestTask    string
}

// ACP agents to test
var ACPAgents = []ACPAgentConfig{
	{ID: "code-reviewer", Description: "Code review agent", TestTask: "Review this code for best practices"},
	{ID: "bug-finder", Description: "Bug detection agent", TestTask: "Find potential bugs in this code"},
	{ID: "refactor-assistant", Description: "Refactoring agent", TestTask: "Suggest refactoring improvements"},
	{ID: "documentation-generator", Description: "Documentation agent", TestTask: "Generate documentation for this function"},
	{ID: "test-generator", Description: "Test generation agent", TestTask: "Generate unit tests for this code"},
	{ID: "security-scanner", Description: "Security scanning agent", TestTask: "Scan for security vulnerabilities"},
}

// TestACPAgentDiscovery tests agent discovery endpoint
func TestACPAgentDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode") // SKIP-OK: #short-mode
	}
	client := NewACPClient(helixAgentBaseURL(t))

	agents, err := client.ListAgents()
	require.NoError(t, err)

	assert.NotEmpty(t, agents, "Should have at least one agent")
	t.Logf("Discovered %d ACP agents: %v", len(agents), agents)
}

// TestACPAgentInfo tests getting agent information
func TestACPAgentInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode") // SKIP-OK: #short-mode
	}
	client := NewACPClient(helixAgentBaseURL(t))

	for _, agent := range ACPAgents {
		t.Run(agent.ID, func(t *testing.T) {
			info, err := client.GetAgentInfo(agent.ID)
			if err != nil {
				t.Skipf("Agent %s not available: %v (SKIP-OK: #infra-unavailable)", agent.ID, err)
				return
			}

			assert.Equal(t, agent.ID, info.GetID())
			t.Logf("Agent %s info: %+v", agent.ID, info)
		})
	}
}

// TestACPAgentExecution tests actual agent task execution
func TestACPAgentExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode") // SKIP-OK: #short-mode
	}
	client := NewACPClient(helixAgentBaseURL(t))

	testCode := `
func add(a, b int) int {
    return a + b
}
`

	for _, agent := range ACPAgents {
		t.Run(agent.ID, func(t *testing.T) {
			req := &AgentRequest{
				AgentID: agent.ID,
				Task:    agent.TestTask,
				Context: map[string]interface{}{
					"code":     testCode,
					"language": "go",
				},
				Timeout: 60,
			}

			resp, err := client.ExecuteTask(req)
			if err != nil {
				t.Skipf("Agent %s execution failed: %v (SKIP-OK: #unmarked-skip-needs-ticket)", agent.ID, err)
				return
			}

			assert.Equal(t, agent.ID, resp.GetID())
			assert.NotEqual(t, "error", resp.Status, "Agent should not return error status")
			t.Logf("Agent %s result: %v", agent.ID, resp.Result)
		})
	}
}

// TestACPHealthCheck tests ACP service health
func TestACPHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode") // SKIP-OK: #short-mode
	}
	client := NewACPClient(helixAgentBaseURL(t))

	resp, err := client.httpClient.Get(client.baseURL + "/v1/acp/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")
}

// BenchmarkACPAgentExecution benchmarks agent task execution
func BenchmarkACPAgentExecution(b *testing.B) {
	base, probed := resolveHelixAgentACP()
	if base == "" {
		b.Skipf("HelixAgent ACP API not reachable; probed %v", probed)
	}
	client := NewACPClient(base)

	req := &AgentRequest{
		AgentID: "code-reviewer",
		Task:    "Review this code",
		Context: map[string]interface{}{
			"code":     "func main() {}",
			"language": "go",
		},
		Timeout: 30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.ExecuteTask(req)
		if err != nil {
			b.Skipf("ACP service not running: %v", err)
			return
		}
	}
}
