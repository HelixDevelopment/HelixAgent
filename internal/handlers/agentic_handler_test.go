package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAgenticHandler() (*AgenticHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	h := NewAgenticHandler(logger)
	r := gin.New()

	api := r.Group("/api/v1")
	RegisterAgenticRoutes(api, h)

	return h, r
}

func TestNewAgenticHandler(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	h := NewAgenticHandler(logger)

	assert.NotNil(t, h)
	assert.NotNil(t, h.logger)
	assert.NotNil(t, h.workflows)
}

func TestNewAgenticHandler_NilLogger(t *testing.T) {
	t.Parallel()
	h := NewAgenticHandler(nil)

	assert.NotNil(t, h)
	assert.NotNil(t, h.logger, "should create default logger")
}

func TestAgenticHandler_CreateWorkflow_Success(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	reqBody := CreateWorkflowRequest{
		Name:        "test-workflow",
		Description: "A test workflow",
		Nodes: []WorkflowNodeRequest{
			{ID: "start", Name: "Start Node", Type: "agent"},
			{ID: "end", Name: "End Node", Type: "tool"},
		},
		Edges: []WorkflowEdgeRequest{
			{From: "start", To: "end", Label: "next"},
		},
		EntryPoint: "start",
		EndNodes:   []string{"end"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CreateWorkflowResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "test-workflow", resp.Name)
	assert.Equal(t, "completed", resp.Status)
	assert.Equal(t, 2, resp.NodesCount)
	assert.Equal(t, 1, resp.EdgesCount)
	assert.Equal(t, "start", resp.EntryPoint)
	assert.Contains(t, resp.EndNodes, "end")
	assert.NotEmpty(t, resp.CreatedAt)
	assert.Empty(t, resp.Error)
}

func TestAgenticHandler_CreateWorkflow_SingleNode(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	reqBody := CreateWorkflowRequest{
		Name: "single-node",
		Nodes: []WorkflowNodeRequest{
			{ID: "only", Name: "Only Node", Type: "agent"},
		},
		EntryPoint: "only",
		EndNodes:   []string{"only"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CreateWorkflowResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "completed", resp.Status)
	assert.Equal(t, 1, resp.NodesCount)
	assert.Equal(t, 0, resp.EdgesCount)
}

func TestAgenticHandler_CreateWorkflow_WithConfig(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	trueVal := true
	reqBody := CreateWorkflowRequest{
		Name: "configured-workflow",
		Nodes: []WorkflowNodeRequest{
			{ID: "a", Name: "Node A", Type: "agent"},
		},
		EntryPoint: "a",
		EndNodes:   []string{"a"},
		Config: &WorkflowConfigRequest{
			MaxIterations:        50,
			TimeoutSeconds:       10,
			EnableCheckpoints:    &trueVal,
			EnableSelfCorrection: &trueVal,
			MaxRetries:           5,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CreateWorkflowResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "completed", resp.Status)
}

func TestAgenticHandler_CreateWorkflow_WithInput(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	reqBody := CreateWorkflowRequest{
		Name: "input-workflow",
		Nodes: []WorkflowNodeRequest{
			{ID: "n1", Name: "Node 1", Type: "agent"},
		},
		EntryPoint: "n1",
		EndNodes:   []string{"n1"},
		Input: &WorkflowInputRequest{
			Query:   "test query",
			Context: map[string]interface{}{"key": "value"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CreateWorkflowResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "completed", resp.Status)
}

func TestAgenticHandler_CreateWorkflow_BadRequest_EmptyBody(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgenticHandler_CreateWorkflow_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows",
		bytes.NewBuffer([]byte(`{invalid}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgenticHandler_CreateWorkflow_BadRequest_MissingName(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	body := []byte(`{
		"nodes": [{"id": "a", "name": "A", "type": "agent"}],
		"entry_point": "a"
	}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgenticHandler_CreateWorkflow_BadRequest_InvalidEntryPoint(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	reqBody := CreateWorkflowRequest{
		Name: "bad-entry",
		Nodes: []WorkflowNodeRequest{
			{ID: "a", Name: "A", Type: "agent"},
		},
		EntryPoint: "nonexistent",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "invalid entry_point")
}

func TestAgenticHandler_CreateWorkflow_BadRequest_InvalidEndNode(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	reqBody := CreateWorkflowRequest{
		Name: "bad-end",
		Nodes: []WorkflowNodeRequest{
			{ID: "a", Name: "A", Type: "agent"},
		},
		EntryPoint: "a",
		EndNodes:   []string{"nonexistent"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "invalid end_node")
}

func TestAgenticHandler_CreateWorkflow_BadRequest_InvalidEdge(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	reqBody := CreateWorkflowRequest{
		Name: "bad-edge",
		Nodes: []WorkflowNodeRequest{
			{ID: "a", Name: "A", Type: "agent"},
		},
		Edges: []WorkflowEdgeRequest{
			{From: "a", To: "nonexistent"},
		},
		EntryPoint: "a",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "failed to add edge")
}

func TestAgenticHandler_GetWorkflow_Success(t *testing.T) {
	t.Parallel()
	h, r := setupAgenticHandler()

	// First create a workflow
	reqBody := CreateWorkflowRequest{
		Name:        "get-test",
		Description: "For GET test",
		Nodes: []WorkflowNodeRequest{
			{ID: "n1", Name: "N1", Type: "agent"},
		},
		EntryPoint: "n1",
		EndNodes:   []string{"n1"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var createResp CreateWorkflowResponse
	err := json.Unmarshal(w.Body.Bytes(), &createResp)
	require.NoError(t, err)

	// Verify the workflow was stored
	_, exists := h.workflows.Get(createResp.ID)
	require.True(t, exists, "workflow should be stored")

	// Now GET it
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/agentic/workflows/"+createResp.ID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var getResp GetWorkflowResponse
	err = json.Unmarshal(w.Body.Bytes(), &getResp)
	require.NoError(t, err)

	assert.Equal(t, createResp.ID, getResp.ID)
	assert.Equal(t, "get-test", getResp.Name)
	assert.Equal(t, "For GET test", getResp.Description)
	assert.Equal(t, "completed", getResp.Status)
	assert.NotEmpty(t, getResp.CreatedAt)
}

func TestAgenticHandler_GetWorkflow_NotFound(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/agentic/workflows/nonexistent-id", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp VerifierErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Error, "workflow not found")
}

func TestAgenticHandler_CreateWorkflow_ResponseContentType(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	reqBody := CreateWorkflowRequest{
		Name: "content-type-test",
		Nodes: []WorkflowNodeRequest{
			{ID: "a", Name: "A", Type: "agent"},
		},
		EntryPoint: "a",
		EndNodes:   []string{"a"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestAgenticHandler_GetWorkflow_ResponseContentType(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/agentic/workflows/any-id", nil)
	r.ServeHTTP(w, req)

	// Even for 404, should return JSON
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestAgenticHandler_CreateWorkflow_MultiNodeChain(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	reqBody := CreateWorkflowRequest{
		Name: "chain-workflow",
		Nodes: []WorkflowNodeRequest{
			{ID: "a", Name: "Step A", Type: "agent"},
			{ID: "b", Name: "Step B", Type: "tool"},
			{ID: "c", Name: "Step C", Type: "agent"},
		},
		Edges: []WorkflowEdgeRequest{
			{From: "a", To: "b", Label: "a-to-b"},
			{From: "b", To: "c", Label: "b-to-c"},
		},
		EntryPoint: "a",
		EndNodes:   []string{"c"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/agentic/workflows", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CreateWorkflowResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "completed", resp.Status)
	assert.Equal(t, 3, resp.NodesCount)
	assert.Equal(t, 2, resp.EdgesCount)
	// Verify execution history contains entries for traversed nodes
	assert.GreaterOrEqual(t, len(resp.History), 1)
}

func TestAgenticHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	_, r := setupAgenticHandler()

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/agentic/workflows"},
		{"PUT", "/api/v1/agentic/workflows"},
		{"DELETE", "/api/v1/agentic/workflows"},
		{"POST", "/api/v1/agentic/workflows/some-id"},
		{"PUT", "/api/v1/agentic/workflows/some-id"},
		{"DELETE", "/api/v1/agentic/workflows/some-id"},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(tt.method, tt.path, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code,
			"Expected 404 for %s %s, got %d", tt.method, tt.path, w.Code)
	}
}

func TestRegisterAgenticRoutes(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	h := NewAgenticHandler(logger)
	r := gin.New()
	api := r.Group("/api/v1")

	// Should not panic
	RegisterAgenticRoutes(api, h)

	// Verify unknown route returns 404
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/agentic/unknown", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateWorkflowRequest_Fields(t *testing.T) {
	t.Parallel()
	trueVal := true
	req := CreateWorkflowRequest{
		Name:        "test",
		Description: "desc",
		Nodes: []WorkflowNodeRequest{
			{ID: "n1", Name: "Node 1", Type: "agent"},
		},
		Edges: []WorkflowEdgeRequest{
			{From: "n1", To: "n2", Label: "link"},
		},
		EntryPoint: "n1",
		EndNodes:   []string{"n2"},
		Config: &WorkflowConfigRequest{
			MaxIterations:     100,
			TimeoutSeconds:    60,
			EnableCheckpoints: &trueVal,
			MaxRetries:        3,
		},
		Input: &WorkflowInputRequest{
			Query:   "hello",
			Context: map[string]interface{}{"a": "b"},
		},
	}

	assert.Equal(t, "test", req.Name)
	assert.Equal(t, "desc", req.Description)
	assert.Len(t, req.Nodes, 1)
	assert.Len(t, req.Edges, 1)
	assert.Equal(t, "n1", req.EntryPoint)
	assert.NotNil(t, req.Config)
	assert.Equal(t, 100, req.Config.MaxIterations)
	assert.NotNil(t, req.Input)
	assert.Equal(t, "hello", req.Input.Query)
}

func TestNodeExecutionSummary_Fields(t *testing.T) {
	t.Parallel()
	s := NodeExecutionSummary{
		NodeID:     "node-1",
		NodeName:   "Test Node",
		DurationMs: 42,
		Error:      "",
	}

	assert.Equal(t, "node-1", s.NodeID)
	assert.Equal(t, "Test Node", s.NodeName)
	assert.Equal(t, int64(42), s.DurationMs)
	assert.Empty(t, s.Error)
}
