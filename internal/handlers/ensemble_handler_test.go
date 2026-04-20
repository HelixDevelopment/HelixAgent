package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupEnsembleSessionRouter creates an EnsembleHandler without a real
// Coordinator. Session endpoints that call Coordinator methods will
// return 500, which lets us test request-validation and error-path
// coverage. Team endpoints work fully (in-memory).
func setupEnsembleSessionRouter(t *testing.T) (*EnsembleHandler, *gin.Engine) {
	t.Helper()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := &EnsembleHandler{
		logger: logger,
		teams:  safe.NewStore[string, *Team](),
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Session routes
	sessions := router.Group("/ensemble/sessions")
	{
		sessions.POST("", handler.CreateSession)
		sessions.GET("", handler.ListSessions)
		sessions.GET("/:id", handler.GetSession)
		sessions.POST("/:id/execute", handler.ExecuteSession)
		sessions.POST("/:id/cancel", handler.CancelSession)
	}

	return handler, router
}

// --- Constructor -------------------------------------------------------

func TestNewEnsembleHandler(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	h := NewEnsembleHandler(nil, logger)

	assert.NotNil(t, h)
	assert.NotNil(t, h.teams)
	assert.Equal(t, logger, h.logger)
}

func TestNewEnsembleHandler_NilLogger(t *testing.T) {
	t.Parallel()
	h := NewEnsembleHandler(nil, nil)
	assert.NotNil(t, h)
	assert.Nil(t, h.coordinator)
	assert.Nil(t, h.logger)
}

// --- CreateSession request validation ----------------------------------

func TestEnsembleHandler_CreateSession_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleSessionRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/ensemble/sessions", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEnsembleHandler_CreateSession_MissingStrategy(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleSessionRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"participants": map[string]interface{}{},
	})
	req := httptest.NewRequest(http.MethodPost, "/ensemble/sessions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- ExecuteSession request validation ---------------------------------

func TestEnsembleHandler_ExecuteSession_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleSessionRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/ensemble/sessions/some-id/execute", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEnsembleHandler_ExecuteSession_MissingContent(t *testing.T) {
	t.Parallel()
	_, router := setupEnsembleSessionRouter(t)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/ensemble/sessions/some-id/execute", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Request/Response type JSON round-trips ----------------------------

func TestCreateSessionRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := CreateSessionRequest{
		Strategy: "voting",
		Participants: ParticipantRequest{
			Primary: &InstanceRequest{
				Type:   "claude",
				Config: map[string]interface{}{"model": "claude-3"},
			},
			Critiques: []InstanceRequest{
				{Type: "openai", Provider: map[string]interface{}{"name": "openai"}},
			},
		},
	}

	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded CreateSessionRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig.Strategy, decoded.Strategy)
	assert.Equal(t, orig.Participants.Primary.Type, decoded.Participants.Primary.Type)
	assert.Len(t, decoded.Participants.Critiques, 1)
}

func TestExecuteRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := ExecuteRequest{Content: "solve this problem", Timeout: 120}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded ExecuteRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

func TestTeamExecutionRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := TeamExecutionRequest{
		Task:             "Analyze this code",
		Context:          map[string]interface{}{"language": "go"},
		Timeout:          30,
		RequireConsensus: true,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded TeamExecutionRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig.Task, decoded.Task)
	assert.Equal(t, orig.RequireConsensus, decoded.RequireConsensus)
}

func TestTeamExecutionResult_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := TeamExecutionResult{
		TeamID:           "team_123",
		Task:             "test",
		Consensus:        "agreed",
		Confidence:       0.85,
		IndividualVotes:  map[string]string{"a1": "yes", "a2": "yes"},
		AgentResults:     []AgentResult{{AgentID: "a1", Output: "yes", Confidence: 0.9}},
		ExecutionTime:    1500,
		ConsensusReached: true,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded TeamExecutionResult
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig.TeamID, decoded.TeamID)
	assert.Equal(t, orig.Consensus, decoded.Consensus)
	assert.True(t, decoded.ConsensusReached)
	assert.Len(t, decoded.AgentResults, 1)
}

// --- AgentType constants -----------------------------------------------

func TestAgentType_Values(t *testing.T) {
	t.Parallel()
	assert.Equal(t, AgentType("primary"), AgentTypePrimary)
	assert.Equal(t, AgentType("critic"), AgentTypeCritic)
	assert.Equal(t, AgentType("verifier"), AgentTypeVerifier)
	assert.Equal(t, AgentType("researcher"), AgentTypeResearcher)
	assert.Equal(t, AgentType("coder"), AgentTypeCoder)
	assert.Equal(t, AgentType("tester"), AgentTypeTester)
}

// --- AgentDefinition / ProviderConfig JSON -----------------------------

func TestAgentDefinition_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := AgentDefinition{
		ID:   "agent_1",
		Name: "Primary Agent",
		Type: AgentTypePrimary,
		Config: map[string]interface{}{
			"temperature": 0.7,
		},
		Provider: ProviderConfig{
			Name:  "openai",
			Model: "gpt-4",
		},
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded AgentDefinition
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig.ID, decoded.ID)
	assert.Equal(t, orig.Name, decoded.Name)
	assert.Equal(t, orig.Type, decoded.Type)
	assert.Equal(t, orig.Provider.Name, decoded.Provider.Name)
	assert.Equal(t, orig.Provider.Model, decoded.Provider.Model)
}

func TestProviderConfig_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := ProviderConfig{
		Name:   "anthropic",
		Model:  "claude-3-opus",
		Config: map[string]interface{}{"max_tokens": float64(4096)},
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded ProviderConfig
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

// --- Team JSON ---------------------------------------------------------

func TestTeam_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Now().Truncate(time.Second)
	orig := Team{
		ID:          "team_1",
		Name:        "Alpha Team",
		Description: "First team",
		Agents: []AgentDefinition{
			{ID: "a1", Name: "Agent 1", Type: AgentTypePrimary},
		},
		Config: TeamConfig{
			MaxParallel:        4,
			ConsensusThreshold: 0.5,
			Timeout:            300,
			EnableVoting:       true,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded Team
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig.ID, decoded.ID)
	assert.Equal(t, orig.Name, decoded.Name)
	assert.Len(t, decoded.Agents, 1)
	assert.Equal(t, orig.Config.MaxParallel, decoded.Config.MaxParallel)
}

// --- TeamConfig --------------------------------------------------------

func TestTeamConfig_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := TeamConfig{
		MaxParallel:        8,
		ConsensusThreshold: 0.75,
		Timeout:            120,
		EnableVoting:       false,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded TeamConfig
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

// --- CreateTeamRequest validation via handler --------------------------

func TestEnsembleHandler_CreateTeam_MissingName(t *testing.T) {
	t.Parallel()
	handler := &EnsembleHandler{
		logger: logrus.New(),
		teams:  safe.NewStore[string, *Team](),
	}
	teamRouter := gin.New()
	teamRouter.POST("/teams", handler.CreateTeam)

	body, _ := json.Marshal(map[string]interface{}{
		"description": "no name provided",
	})
	req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	teamRouter.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- executeAgent coverage (all agent types) ---------------------------

func TestEnsembleHandler_ExecuteAgent_AllTypes(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := &EnsembleHandler{
		logger: logger,
		teams:  safe.NewStore[string, *Team](),
	}

	agentTypes := []AgentType{
		AgentTypePrimary,
		AgentTypeCritic,
		AgentTypeVerifier,
		AgentTypeResearcher,
		AgentTypeCoder,
		AgentTypeTester,
		AgentType("unknown"),
	}

	for _, at := range agentTypes {
		t.Run(string(at), func(t *testing.T) {
			t.Parallel()
			agent := AgentDefinition{
				ID:   "test-agent",
				Name: "Test",
				Type: at,
			}
			req := TeamExecutionRequest{Task: "test"}

			result := handler.executeAgent(
				t.Context(),
				agent,
				req,
			)

			assert.Equal(t, "test-agent", result.AgentID)
			assert.Equal(t, "Test", result.AgentName)
			assert.Equal(t, at, result.AgentType)
			assert.NotEmpty(t, result.Output)
			assert.Greater(t, result.Confidence, 0.0)
			assert.GreaterOrEqual(t, result.ExecutionTime, int64(0))
		})
	}
}

// --- calculateConsensus edge cases -------------------------------------

func TestEnsembleHandler_CalculateConsensus_ZeroThreshold(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := &EnsembleHandler{
		logger: logger,
		teams:  safe.NewStore[string, *Team](),
	}

	results := []AgentResult{
		{Output: "A", Confidence: 0.8},
	}

	consensus, confidence, reached := handler.calculateConsensus(results, 0)
	assert.Equal(t, "A", consensus)
	assert.Greater(t, confidence, 0.0)
	assert.True(t, reached)
}

func TestEnsembleHandler_CalculateConsensus_Unanimous(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := &EnsembleHandler{
		logger: logger,
		teams:  safe.NewStore[string, *Team](),
	}

	results := []AgentResult{
		{Output: "X", Confidence: 0.9},
		{Output: "X", Confidence: 0.85},
		{Output: "X", Confidence: 0.95},
	}

	consensus, confidence, reached := handler.calculateConsensus(results, 0.5)
	assert.Equal(t, "X", consensus)
	assert.Greater(t, confidence, 0.5)
	assert.True(t, reached)
}

// --- AgentResult JSON --------------------------------------------------

func TestAgentResult_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := AgentResult{
		AgentID:       "a1",
		AgentName:     "Primary",
		AgentType:     AgentTypePrimary,
		Output:        "result text",
		Confidence:    0.88,
		ExecutionTime: 250,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded AgentResult
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig, decoded)
}

// --- RegisterRoutes verification ---------------------------------------

func TestEnsembleHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	handler := NewEnsembleHandler(nil, logger)

	router := gin.New()
	group := router.Group("/v1")
	handler.RegisterRoutes(group)

	routes := router.Routes()
	expected := map[string]bool{
		"POST:/v1/ensemble/sessions":                    true,
		"GET:/v1/ensemble/sessions":                     true,
		"GET:/v1/ensemble/sessions/:id":                 true,
		"POST:/v1/ensemble/sessions/:id/execute":        true,
		"POST:/v1/ensemble/sessions/:id/cancel":         true,
		"POST:/v1/ensemble/teams":                       true,
		"GET:/v1/ensemble/teams":                        true,
		"GET:/v1/ensemble/teams/:id":                    true,
		"PUT:/v1/ensemble/teams/:id":                    true,
		"DELETE:/v1/ensemble/teams/:id":                 true,
		"POST:/v1/ensemble/teams/:id/agents":            true,
		"DELETE:/v1/ensemble/teams/:id/agents/:agentId": true,
		"POST:/v1/ensemble/teams/:id/execute":           true,
	}

	for _, r := range routes {
		key := r.Method + ":" + r.Path
		delete(expected, key)
	}

	assert.Empty(t, expected, "some expected routes were not registered: %v", expected)
}
