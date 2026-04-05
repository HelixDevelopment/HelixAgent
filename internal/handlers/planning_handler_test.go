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

func setupPlanningHandler() (*PlanningHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	h := NewPlanningHandler(logger)
	r := gin.New()

	api := r.Group("/api/v1")
	RegisterPlanningRoutes(api, h)

	return h, r
}

func TestNewPlanningHandler(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	h := NewPlanningHandler(logger)

	assert.NotNil(t, h)
	assert.NotNil(t, h.logger)
}

func TestNewPlanningHandler_NilLogger(t *testing.T) {
	t.Parallel()
	h := NewPlanningHandler(nil)

	assert.NotNil(t, h)
	assert.NotNil(t, h.logger, "should create default logger")
}

// --- HiPlan Tests ---

func TestPlanningHandler_HiPlan_Success(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	reqBody := HiPlanRequest{
		Goal: "Build a REST API service",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/hiplan", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp HiPlanResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.PlanID)
	assert.Equal(t, "Build a REST API service", resp.Goal)
	assert.Equal(t, "created", resp.State)
	assert.NotEmpty(t, resp.Milestones)
	assert.NotEmpty(t, resp.CreatedAt)
	assert.Empty(t, resp.Error)
}

func TestPlanningHandler_HiPlan_WithConfig(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	falseVal := false
	reqBody := HiPlanRequest{
		Goal: "Deploy microservices",
		Config: &HiPlanConfigRequest{
			MaxMilestones:            5,
			MaxStepsPerMilestone:     10,
			EnableParallelMilestones: &falseVal,
			MaxRetries:               2,
			TimeoutSeconds:           30,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/hiplan", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp HiPlanResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "created", resp.State)
	assert.NotEmpty(t, resp.Milestones)
}

func TestPlanningHandler_HiPlan_MilestoneStructure(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	reqBody := HiPlanRequest{
		Goal: "Create a machine learning pipeline",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/hiplan", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp HiPlanResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Static generator produces 3 milestones
	assert.Equal(t, 3, len(resp.Milestones))

	for _, m := range resp.Milestones {
		assert.NotEmpty(t, m.ID)
		assert.NotEmpty(t, m.Name)
		assert.Equal(t, "pending", m.State)
		assert.GreaterOrEqual(t, m.StepsCount, 0)
	}
}

func TestPlanningHandler_HiPlan_BadRequest_EmptyBody(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/hiplan", bytes.NewBuffer([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanningHandler_HiPlan_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/hiplan",
		bytes.NewBuffer([]byte(`{invalid}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanningHandler_HiPlan_BadRequest_MissingGoal(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	body := []byte(`{"config": {"max_milestones": 5}}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/hiplan", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanningHandler_HiPlan_ContentType(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	reqBody := HiPlanRequest{Goal: "test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/hiplan", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

// --- MCTS Tests ---

func TestPlanningHandler_MCTS_Success(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	reqBody := MCTSRequest{
		InitialState: "function fibonacci(n) { }",
		Config: &MCTSConfigRequest{
			MaxIterations: 10,
			MaxDepth:      3,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/mcts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp MCTSResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, resp.TotalIterations, 1)
	assert.GreaterOrEqual(t, resp.TreeSize, 1)
	assert.GreaterOrEqual(t, resp.DurationMs, int64(0))
	assert.Empty(t, resp.Error)
}

func TestPlanningHandler_MCTS_DefaultConfig(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	// Use very low iterations to keep test fast
	reqBody := MCTSRequest{
		InitialState: "initial state",
		Config: &MCTSConfigRequest{
			MaxIterations:  5,
			MaxDepth:       2,
			TimeoutSeconds: 5,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/mcts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp MCTSResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Error)
}

func TestPlanningHandler_MCTS_WithFullConfig(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	falseVal := false
	reqBody := MCTSRequest{
		InitialState: "code state",
		Config: &MCTSConfigRequest{
			ExplorationConstant: 2.0,
			MaxDepth:            2,
			MaxIterations:       5,
			RolloutDepth:        3,
			SimulationCount:     2,
			DiscountFactor:      0.95,
			TimeoutSeconds:      5,
			UseUCTDP:            &falseVal,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/mcts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp MCTSResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Error)
}

func TestPlanningHandler_MCTS_BadRequest_EmptyBody(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/mcts", bytes.NewBuffer([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanningHandler_MCTS_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/mcts",
		bytes.NewBuffer([]byte(`not json`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanningHandler_MCTS_BadRequest_MissingInitialState(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	body := []byte(`{"config": {"max_iterations": 10}}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/mcts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanningHandler_MCTS_ContentType(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	reqBody := MCTSRequest{
		InitialState: "test",
		Config:       &MCTSConfigRequest{MaxIterations: 5, MaxDepth: 2},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/mcts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

// --- Tree of Thoughts Tests ---

func TestPlanningHandler_ToT_Success(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	reqBody := ToTRequest{
		Problem: "How to optimize database queries for a large dataset?",
		Config: &ToTConfigRequest{
			MaxDepth:      3,
			MaxBranches:   3,
			MaxIterations: 20,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/tot", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ToTResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "How to optimize database queries for a large dataset?", resp.Problem)
	assert.GreaterOrEqual(t, resp.Iterations, 1)
	assert.GreaterOrEqual(t, resp.NodesExplored, 1)
	assert.GreaterOrEqual(t, resp.DurationMs, int64(0))
	assert.Empty(t, resp.Error)
}

func TestPlanningHandler_ToT_WithStrategy(t *testing.T) {
	t.Parallel()
	strategies := []string{"bfs", "dfs", "beam"}

	for _, strategy := range strategies {
		t.Run("strategy_"+strategy, func(t *testing.T) {
				t.Parallel()
			_, r := setupPlanningHandler()

			reqBody := ToTRequest{
				Problem: "Solve the problem",
				Config: &ToTConfigRequest{
					SearchStrategy: strategy,
					MaxDepth:       3,
					MaxBranches:    2,
					MaxIterations:  15,
				},
			}
			body, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/planning/tot", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp ToTResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			assert.Equal(t, strategy, resp.Strategy)
			assert.Empty(t, resp.Error)
		})
	}
}

func TestPlanningHandler_ToT_WithFullConfig(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	trueVal := true
	reqBody := ToTRequest{
		Problem: "Design a caching layer",
		Config: &ToTConfigRequest{
			MaxDepth:           4,
			MaxBranches:        3,
			MinScore:           0.2,
			PruneThreshold:     0.1,
			SearchStrategy:     "beam",
			BeamWidth:          2,
			Temperature:        0.8,
			EnableBacktracking: &trueVal,
			MaxIterations:      20,
			TimeoutSeconds:     10,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/tot", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ToTResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "beam", resp.Strategy)
	assert.Empty(t, resp.Error)
}

func TestPlanningHandler_ToT_SolutionStructure(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	reqBody := ToTRequest{
		Problem: "Write a sorting algorithm",
		Config: &ToTConfigRequest{
			MaxDepth:      3,
			MaxBranches:   3,
			MaxIterations: 30,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/tot", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ToTResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	if len(resp.Solution) > 0 {
		for _, thought := range resp.Solution {
			assert.NotEmpty(t, thought.ID)
			assert.NotEmpty(t, thought.Content)
		}
	}
}

func TestPlanningHandler_ToT_BadRequest_EmptyBody(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/tot", bytes.NewBuffer([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanningHandler_ToT_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/tot",
		bytes.NewBuffer([]byte(`{broken`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanningHandler_ToT_BadRequest_MissingProblem(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	body := []byte(`{"config": {"max_depth": 5}}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/tot", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPlanningHandler_ToT_ContentType(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	reqBody := ToTRequest{
		Problem: "test",
		Config:  &ToTConfigRequest{MaxIterations: 5, MaxDepth: 2, MaxBranches: 2},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/planning/tot", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

// --- Route Registration and Method Tests ---

func TestRegisterPlanningRoutes(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	h := NewPlanningHandler(logger)
	r := gin.New()
	api := r.Group("/api/v1")

	// Should not panic
	RegisterPlanningRoutes(api, h)

	// Verify unknown route returns 404
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/planning/unknown", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPlanningHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	_, r := setupPlanningHandler()

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/planning/hiplan"},
		{"PUT", "/api/v1/planning/hiplan"},
		{"DELETE", "/api/v1/planning/hiplan"},
		{"GET", "/api/v1/planning/mcts"},
		{"PUT", "/api/v1/planning/mcts"},
		{"DELETE", "/api/v1/planning/mcts"},
		{"GET", "/api/v1/planning/tot"},
		{"PUT", "/api/v1/planning/tot"},
		{"DELETE", "/api/v1/planning/tot"},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(tt.method, tt.path, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code,
			"Expected 404 for %s %s, got %d", tt.method, tt.path, w.Code)
	}
}

// --- Request Type Field Tests ---

func TestHiPlanRequest_Fields(t *testing.T) {
	t.Parallel()
	trueVal := true
	req := HiPlanRequest{
		Goal: "Build something",
		Config: &HiPlanConfigRequest{
			MaxMilestones:            10,
			MaxStepsPerMilestone:     20,
			EnableParallelMilestones: &trueVal,
			MaxParallelMilestones:    4,
			MaxRetries:               3,
			TimeoutSeconds:           120,
		},
	}

	assert.Equal(t, "Build something", req.Goal)
	assert.NotNil(t, req.Config)
	assert.Equal(t, 10, req.Config.MaxMilestones)
	assert.Equal(t, 20, req.Config.MaxStepsPerMilestone)
	assert.True(t, *req.Config.EnableParallelMilestones)
}

func TestMCTSRequest_Fields(t *testing.T) {
	t.Parallel()
	trueVal := true
	req := MCTSRequest{
		InitialState: "start state",
		Config: &MCTSConfigRequest{
			ExplorationConstant: 1.5,
			MaxDepth:            20,
			MaxIterations:       500,
			UseUCTDP:            &trueVal,
		},
	}

	assert.Equal(t, "start state", req.InitialState)
	assert.NotNil(t, req.Config)
	assert.Equal(t, 1.5, req.Config.ExplorationConstant)
	assert.True(t, *req.Config.UseUCTDP)
}

func TestToTRequest_Fields(t *testing.T) {
	t.Parallel()
	falseVal := false
	req := ToTRequest{
		Problem: "solve this",
		Config: &ToTConfigRequest{
			MaxDepth:           8,
			MaxBranches:        4,
			SearchStrategy:     "dfs",
			EnableBacktracking: &falseVal,
		},
	}

	assert.Equal(t, "solve this", req.Problem)
	assert.NotNil(t, req.Config)
	assert.Equal(t, "dfs", req.Config.SearchStrategy)
	assert.False(t, *req.Config.EnableBacktracking)
}

// --- Helper Function Tests ---

func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exact len", 9, "exact len"},
		{"this is a long string", 10, "this is..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"", 5, ""},
		{"hello world", 5, "he..."},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		assert.Equal(t, tt.expected, result,
			"truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
	}
}
