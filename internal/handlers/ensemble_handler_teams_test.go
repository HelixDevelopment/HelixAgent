package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupEnsembleHandlerTest(t *testing.T) (*EnsembleHandler, *gin.Engine) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Create handler without coordinator for team tests
	handler := &EnsembleHandler{
		logger: logger,
		teams:  make(map[string]*Team),
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Register team routes
	teams := router.Group("/ensemble/teams")
	{
		teams.POST("", handler.CreateTeam)
		teams.GET("", handler.ListTeams)
		teams.GET("/:id", handler.GetTeam)
		teams.PUT("/:id", handler.UpdateTeam)
		teams.DELETE("/:id", handler.DeleteTeam)
		teams.POST("/:id/agents", handler.AddAgentToTeam)
		teams.DELETE("/:id/agents/:agentId", handler.RemoveAgentFromTeam)
		teams.POST("/:id/execute", handler.ExecuteTeam)
	}

	return handler, router
}

func TestCreateTeam(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleHandlerTest(t)

	reqBody := CreateTeamRequest{
		Name:        "Test Team",
		Description: "A test team",
		Agents: []AgentDefinition{
			{
				Name: "Agent 1",
				Type: AgentTypePrimary,
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/ensemble/teams", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response["id"])
	assert.Equal(t, "Test Team", response["name"])
	assert.Equal(t, "A test team", response["description"])
	assert.Equal(t, float64(1), response["agents"])
	assert.NotNil(t, response["created_at"])
}

func TestCreateTeam_InvalidRequest(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleHandlerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/ensemble/teams", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListTeams(t *testing.T) {
	t.Parallel()
	handler, router := setupEnsembleHandlerTest(t)

	// Create some teams
	handler.teams["team1"] = &Team{
		ID:          "team1",
		Name:        "Team 1",
		Description: "First team",
		Agents:      []AgentDefinition{{ID: "a1", Name: "Agent 1", Type: AgentTypePrimary}},
	}
	handler.teams["team2"] = &Team{
		ID:          "team2",
		Name:        "Team 2",
		Description: "Second team",
		Agents:      []AgentDefinition{},
	}

	req := httptest.NewRequest(http.MethodGet, "/ensemble/teams", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response, 2)
}

func TestGetTeam(t *testing.T) {
	t.Parallel()
	handler, router := setupEnsembleHandlerTest(t)

	// Create a team
	handler.teams["team1"] = &Team{
		ID:          "team1",
		Name:        "Team 1",
		Description: "First team",
		Agents:      []AgentDefinition{{ID: "a1", Name: "Agent 1", Type: AgentTypePrimary}},
	}

	req := httptest.NewRequest(http.MethodGet, "/ensemble/teams/team1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response Team
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "team1", response.ID)
	assert.Equal(t, "Team 1", response.Name)
}

func TestGetTeam_NotFound(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/ensemble/teams/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateTeam(t *testing.T) {
	t.Parallel()
	handler, router := setupEnsembleHandlerTest(t)

	// Create a team
	handler.teams["team1"] = &Team{
		ID:          "team1",
		Name:        "Team 1",
		Description: "First team",
	}

	reqBody := UpdateTeamRequest{
		Name:        "Updated Team",
		Description: "Updated description",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/ensemble/teams/team1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response Team
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Updated Team", response.Name)
	assert.Equal(t, "Updated description", response.Description)
}

func TestUpdateTeam_NotFound(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleHandlerTest(t)

	reqBody := UpdateTeamRequest{Name: "Updated"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/ensemble/teams/nonexistent", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteTeam(t *testing.T) {
	t.Parallel()
	handler, router := setupEnsembleHandlerTest(t)

	// Create a team
	handler.teams["team1"] = &Team{
		ID:   "team1",
		Name: "Team 1",
	}

	req := httptest.NewRequest(http.MethodDelete, "/ensemble/teams/team1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify team was deleted
	handler.teamsMu.RLock()
	_, exists := handler.teams["team1"]
	handler.teamsMu.RUnlock()
	assert.False(t, exists)
}

func TestDeleteTeam_NotFound(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleHandlerTest(t)

	req := httptest.NewRequest(http.MethodDelete, "/ensemble/teams/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAddAgentToTeam(t *testing.T) {
	t.Parallel()
	handler, router := setupEnsembleHandlerTest(t)

	// Create a team
	handler.teams["team1"] = &Team{
		ID:     "team1",
		Name:   "Team 1",
		Agents: []AgentDefinition{},
	}

	agent := AgentDefinition{
		Name: "New Agent",
		Type: AgentTypeCoder,
	}

	body, _ := json.Marshal(agent)
	req := httptest.NewRequest(http.MethodPost, "/ensemble/teams/team1/agents", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response AgentDefinition
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "New Agent", response.Name)
	assert.NotEmpty(t, response.ID)
}

func TestAddAgentToTeam_NotFound(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleHandlerTest(t)

	agent := AgentDefinition{Name: "New Agent"}
	body, _ := json.Marshal(agent)
	req := httptest.NewRequest(http.MethodPost, "/ensemble/teams/nonexistent/agents", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRemoveAgentFromTeam(t *testing.T) {
	t.Parallel()
	handler, router := setupEnsembleHandlerTest(t)

	// Create a team with agents
	handler.teams["team1"] = &Team{
		ID:   "team1",
		Name: "Team 1",
		Agents: []AgentDefinition{
			{ID: "agent1", Name: "Agent 1"},
			{ID: "agent2", Name: "Agent 2"},
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/ensemble/teams/team1/agents/agent1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify agent was removed
	handler.teamsMu.RLock()
	team := handler.teams["team1"]
	handler.teamsMu.RUnlock()
	assert.Len(t, team.Agents, 1)
}

func TestRemoveAgentFromTeam_TeamNotFound(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleHandlerTest(t)

	req := httptest.NewRequest(http.MethodDelete, "/ensemble/teams/nonexistent/agents/agent1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRemoveAgentFromTeam_AgentNotFound(t *testing.T) {
	t.Parallel()
	handler, router := setupEnsembleHandlerTest(t)

	// Create a team
	handler.teams["team1"] = &Team{
		ID:     "team1",
		Name:   "Team 1",
		Agents: []AgentDefinition{{ID: "agent1", Name: "Agent 1"}},
	}

	req := httptest.NewRequest(http.MethodDelete, "/ensemble/teams/team1/agents/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExecuteTeam(t *testing.T) {
	t.Parallel()
	handler, router := setupEnsembleHandlerTest(t)

	// Create a team with agents
	handler.teams["team1"] = &Team{
		ID:   "team1",
		Name: "Team 1",
		Agents: []AgentDefinition{
			{ID: "agent1", Name: "Primary", Type: AgentTypePrimary},
			{ID: "agent2", Name: "Critic", Type: AgentTypeCritic},
		},
		Config: TeamConfig{
			MaxParallel:        4,
			ConsensusThreshold: 0.5,
			Timeout:            60,
			EnableVoting:       true,
		},
	}

	reqBody := TeamExecutionRequest{
		Task: "Test task",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/ensemble/teams/team1/execute", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response TeamExecutionResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "team1", response.TeamID)
	assert.Equal(t, "Test task", response.Task)
	assert.True(t, response.ConsensusReached)
	assert.NotEmpty(t, response.Consensus)
	assert.Len(t, response.AgentResults, 2)
}

func TestExecuteTeam_NotFound(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleHandlerTest(t)

	reqBody := TeamExecutionRequest{Task: "Test task"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/ensemble/teams/nonexistent/execute", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExecuteTeam_NoAgents(t *testing.T) {
	t.Parallel()
	handler, router := setupEnsembleHandlerTest(t)

	// Create a team with no agents
	handler.teams["team1"] = &Team{
		ID:     "team1",
		Name:   "Team 1",
		Agents: []AgentDefinition{},
	}

	reqBody := TeamExecutionRequest{Task: "Test task"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/ensemble/teams/team1/execute", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDefaultTeamConfig(t *testing.T) {
	t.Parallel()
	config := DefaultTeamConfig()

	assert.Equal(t, 4, config.MaxParallel)
	assert.Equal(t, 0.5, config.ConsensusThreshold)
	assert.Equal(t, 300, config.Timeout)
	assert.True(t, config.EnableVoting)
}

func TestCalculateConsensus(t *testing.T) {
	t.Parallel()
	handler, _ := setupEnsembleHandlerTest(t)

	tests := []struct {
		name      string
		results   []AgentResult
		threshold float64
		wantReach bool
	}{
		{
			name: "simple_majority",
			results: []AgentResult{
				{Output: "A", Confidence: 0.8},
				{Output: "A", Confidence: 0.9},
				{Output: "B", Confidence: 0.7},
			},
			threshold: 0.5,
			wantReach: true,
		},
		{
			name: "no_consensus",
			results: []AgentResult{
				{Output: "A", Confidence: 0.8},
				{Output: "B", Confidence: 0.9},
				{Output: "C", Confidence: 0.7},
			},
			threshold: 0.5,
			wantReach: false,
		},
		{
			name:      "empty",
			results:   []AgentResult{},
			threshold: 0.5,
			wantReach: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			consensus, confidence, reached := handler.calculateConsensus(tt.results, tt.threshold)
			assert.Equal(t, tt.wantReach, reached)
			if tt.wantReach {
				assert.NotEmpty(t, consensus)
				assert.Greater(t, confidence, 0.0)
			}
		})
	}
}

func TestGenerateTeamID(t *testing.T) {
	t.Parallel()
	id1 := generateTeamID()
	id2 := generateTeamID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "team_")
}

func BenchmarkCreateTeam(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := &EnsembleHandler{
		logger: logger,
		teams:  make(map[string]*Team),
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/ensemble/teams", handler.CreateTeam)

	reqBody := CreateTeamRequest{
		Name:        "Benchmark Team",
		Description: "Benchmark description",
	}
	body, _ := json.Marshal(reqBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/ensemble/teams", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkListTeams(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := &EnsembleHandler{
		logger: logger,
		teams:  make(map[string]*Team),
	}

	// Create 100 teams
	for i := 0; i < 100; i++ {
		handler.teams[fmt.Sprintf("team%d", i)] = &Team{
			ID:   fmt.Sprintf("team%d", i),
			Name: fmt.Sprintf("Team %d", i),
		}
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ensemble/teams", handler.ListTeams)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ensemble/teams", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
