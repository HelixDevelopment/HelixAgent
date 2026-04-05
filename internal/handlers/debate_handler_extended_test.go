package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestNewDebateHandler_Extended tests handler creation with various configs
func TestNewDebateHandler_Extended(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewDebateHandler(nil, nil, logger)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.logger)
	assert.NotNil(t, handler.activeDebates)
	assert.Nil(t, handler.debateService)
	assert.Nil(t, handler.advancedDebate)
	assert.Nil(t, handler.skillsIntegration)
}

// TestNewDebateHandlerWithSkills tests handler creation with skills set via setter
func TestNewDebateHandlerWithSkills(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewDebateHandler(nil, nil, logger)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.activeDebates)
	assert.Nil(t, handler.skillsIntegration)
}

// TestDebateHandler_SetSkillsIntegration tests setting skills integration
func TestDebateHandler_SetSkillsIntegration(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	handler := NewDebateHandler(nil, nil, logger)

	handler.SetSkillsIntegration(nil)
	assert.Nil(t, handler.skillsIntegration)
}

// TestDebateHandler_SetOrchestratorIntegration tests setting orchestrator integration
func TestDebateHandler_SetOrchestratorIntegration(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	handler := NewDebateHandler(nil, nil, logger)

	handler.SetOrchestratorIntegration(nil)
	assert.Nil(t, handler.orchestratorIntegration)
}

// TestDebateHandler_CreateDebate_InvalidJSON tests create with invalid JSON
func TestDebateHandler_CreateDebate_InvalidJSON(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader([]byte("invalid json")))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateDebate(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "error")
}

// TestDebateHandler_CreateDebate_MissingTopic tests create without topic
func TestDebateHandler_CreateDebate_MissingTopic(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	reqBody := map[string]interface{}{
		"participants": []map[string]string{
			{"name": "AI1"},
			{"name": "AI2"},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateDebate(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDebateHandler_CreateDebate_InsufficientParticipants tests create with <2 participants
func TestDebateHandler_CreateDebate_InsufficientParticipants(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	reqBody := map[string]interface{}{
		"topic": "Test topic",
		"participants": []map[string]string{
			{"name": "AI1"},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateDebate(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDebateHandler_CreateDebate_Success tests successful creation
func TestDebateHandler_CreateDebate_Success(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	reqBody := CreateDebateRequest{
		Topic: "Should AI be regulated?",
		Participants: []ParticipantConfigRequest{
			{Name: "Pro AI"},
			{Name: "Against AI"},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateDebate(c)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "debate_id")
	assert.Equal(t, "pending", response["status"])
	assert.Equal(t, "Should AI be regulated?", response["topic"])
}

// TestDebateHandler_CreateDebate_WithDebateID tests creation with provided ID
func TestDebateHandler_CreateDebate_WithDebateID(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	reqBody := CreateDebateRequest{
		DebateID: "custom-debate-id",
		Topic:    "Test topic",
		Participants: []ParticipantConfigRequest{
			{Name: "AI1"},
			{Name: "AI2"},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateDebate(c)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "custom-debate-id", response["debate_id"])
}

// TestDebateHandler_CreateDebate_WithAllOptions tests creation with all options
func TestDebateHandler_CreateDebate_WithAllOptions(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	reqBody := CreateDebateRequest{
		Topic:     "Complex topic",
		MaxRounds: 5,
		Timeout:   600,
		Strategy:  "majority_vote",
		Participants: []ParticipantConfigRequest{
			{
				Name:        "Expert 1",
				Role:        "proposer",
				LLMProvider: "deepseek",
				LLMModel:    "deepseek-chat",
				Weight:      1.5,
			},
			{
				Name:        "Expert 2",
				Role:        "critic",
				LLMProvider: "claude",
				LLMModel:    "claude-3",
				Weight:      1.2,
			},
		},
		EnableMultiPassValidation: true,
		ValidationConfig: &ValidationConfigRequest{
			EnableValidation:    true,
			EnablePolish:        true,
			ValidationTimeout:   120,
			PolishTimeout:       60,
			MinConfidenceToSkip: 0.9,
			MaxValidationRounds: 3,
			ShowPhaseIndicators: true,
		},
		Metadata: map[string]any{
			"category": "tech",
			"priority": "high",
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateDebate(c)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(5), response["max_rounds"])
	assert.Equal(t, float64(600), response["timeout"])
}

// TestDebateHandler_GetDebate_NotFound tests get non-existent debate
func TestDebateHandler_GetDebate_NotFound(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	c.Request = httptest.NewRequest("GET", "/v1/debates/non-existent", nil)

	handler.GetDebate(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "error")
}

// TestDebateHandler_GetDebate_Success tests successful get
func TestDebateHandler_GetDebate_Success(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	// First create a debate
	reqBody := CreateDebateRequest{
		DebateID: "test-debate-get",
		Topic:    "Test topic",
		Participants: []ParticipantConfigRequest{
			{Name: "AI1"},
			{Name: "AI2"},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader(jsonBody))
	c1.Request.Header.Set("Content-Type", "application/json")
	handler.CreateDebate(c1)

	// Give the goroutine time to update state
	time.Sleep(50 * time.Millisecond)

	// Now get the debate
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Params = gin.Params{{Key: "id", Value: "test-debate-get"}}
	c2.Request = httptest.NewRequest("GET", "/v1/debates/test-debate-get", nil)

	handler.GetDebate(c2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w2.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "test-debate-get", response["debate_id"])
	assert.Equal(t, "Test topic", response["topic"])
}

// TestDebateHandler_GetDebateStatus_NotFound tests status of non-existent debate
func TestDebateHandler_GetDebateStatus_NotFound(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	c.Request = httptest.NewRequest("GET", "/v1/debates/non-existent/status", nil)

	handler.GetDebateStatus(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDebateHandler_GetDebateResults_NotFound tests results of non-existent debate
func TestDebateHandler_GetDebateResults_NotFound(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	c.Request = httptest.NewRequest("GET", "/v1/debates/non-existent/results", nil)

	handler.GetDebateResults(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDebateHandler_GetDebateResults_NotCompleted tests results of incomplete debate
func TestDebateHandler_GetDebateResults_NotCompleted(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	// First create a debate
	reqBody := CreateDebateRequest{
		DebateID: "test-debate-pending",
		Topic:    "Test topic",
		Participants: []ParticipantConfigRequest{
			{Name: "AI1"},
			{Name: "AI2"},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader(jsonBody))
	c1.Request.Header.Set("Content-Type", "application/json")
	handler.CreateDebate(c1)

	// Give a tiny bit of time for state update
	time.Sleep(10 * time.Millisecond)

	// Now try to get results before completion
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Params = gin.Params{{Key: "id", Value: "test-debate-pending"}}
	c2.Request = httptest.NewRequest("GET", "/v1/debates/test-debate-pending/results", nil)

	handler.GetDebateResults(c2)

	assert.Equal(t, http.StatusBadRequest, w2.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w2.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "error")
}

// TestDebateHandler_ListDebates_Empty tests listing with no debates
func TestDebateHandler_ListDebates_Empty(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/debates", nil)

	handler.ListDebates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "debates")
	assert.Contains(t, response, "count")
	assert.Equal(t, float64(0), response["count"])
}

// TestDebateHandler_ListDebates_WithDebates tests listing with debates
func TestDebateHandler_ListDebates_WithDebates(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	// Create a few debates
	for i := 0; i < 3; i++ {
		reqBody := CreateDebateRequest{
			Topic: "Test topic " + string(rune('A'+i)),
			Participants: []ParticipantConfigRequest{
				{Name: "AI1"},
				{Name: "AI2"},
			},
		}
		jsonBody, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.CreateDebate(c)
	}

	time.Sleep(10 * time.Millisecond)

	// List debates
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/debates", nil)

	handler.ListDebates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(3), response["count"])
}

// TestDebateHandler_ListDebates_WithStatusFilter tests listing with status filter
func TestDebateHandler_ListDebates_WithStatusFilter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	// Create a debate
	reqBody := CreateDebateRequest{
		Topic: "Test topic",
		Participants: []ParticipantConfigRequest{
			{Name: "AI1"},
			{Name: "AI2"},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader(jsonBody))
	c1.Request.Header.Set("Content-Type", "application/json")
	handler.CreateDebate(c1)

	time.Sleep(10 * time.Millisecond)

	// List with status filter
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("GET", "/v1/debates?status=running", nil)

	handler.ListDebates(c2)

	assert.Equal(t, http.StatusOK, w2.Code)
}

// TestDebateHandler_DeleteDebate_NotFound tests deleting non-existent debate
func TestDebateHandler_DeleteDebate_NotFound(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	c.Request = httptest.NewRequest("DELETE", "/v1/debates/non-existent", nil)

	handler.DeleteDebate(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDebateHandler_DeleteDebate_Success tests successful deletion
func TestDebateHandler_DeleteDebate_Success(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	// First create a debate
	reqBody := CreateDebateRequest{
		DebateID: "test-debate-delete",
		Topic:    "Test topic",
		Participants: []ParticipantConfigRequest{
			{Name: "AI1"},
			{Name: "AI2"},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest("POST", "/v1/debates", bytes.NewReader(jsonBody))
	c1.Request.Header.Set("Content-Type", "application/json")
	handler.CreateDebate(c1)

	time.Sleep(10 * time.Millisecond)

	// Delete the debate
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Params = gin.Params{{Key: "id", Value: "test-debate-delete"}}
	c2.Request = httptest.NewRequest("DELETE", "/v1/debates/test-debate-delete", nil)

	handler.DeleteDebate(c2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w2.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "test-debate-delete", response["debate_id"])
	assert.Contains(t, response["message"], "deleted")
}

// TestDebateHandler_RegisterRoutes tests route registration
func TestDebateHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewDebateHandler(nil, nil, logger)

	router := gin.New()
	group := router.Group("/v1")
	handler.RegisterRoutes(group)

	routes := router.Routes()

	expectedRoutes := []string{
		"/v1/debates",
		"/v1/debates/:id",
		"/v1/debates/:id/status",
		"/v1/debates/:id/results",
	}

	routePaths := make(map[string]bool)
	for _, route := range routes {
		routePaths[route.Path] = true
	}

	for _, expected := range expectedRoutes {
		assert.True(t, routePaths[expected], "Route %s should be registered", expected)
	}
}

// TestCreateDebateRequest_Struct tests request struct fields
func TestCreateDebateRequest_Struct(t *testing.T) {
	t.Parallel()
	req := CreateDebateRequest{
		DebateID:                  "debate-123",
		Topic:                     "Test topic",
		MaxRounds:                 5,
		Timeout:                   300,
		Strategy:                  "consensus",
		EnableCognee:              true,
		EnableMultiPassValidation: true,
		Metadata: map[string]any{
			"key": "value",
		},
	}

	assert.Equal(t, "debate-123", req.DebateID)
	assert.Equal(t, "Test topic", req.Topic)
	assert.Equal(t, 5, req.MaxRounds)
	assert.Equal(t, 300, req.Timeout)
	assert.Equal(t, "consensus", req.Strategy)
	assert.True(t, req.EnableCognee)
	assert.True(t, req.EnableMultiPassValidation)
	assert.NotNil(t, req.Metadata)
}

// TestParticipantConfigRequest_Struct tests participant struct fields
func TestParticipantConfigRequest_Struct(t *testing.T) {
	t.Parallel()
	participant := ParticipantConfigRequest{
		ParticipantID: "participant-1",
		Name:          "AI Expert",
		Role:          "proposer",
		LLMProvider:   "deepseek",
		LLMModel:      "deepseek-chat",
		MaxRounds:     3,
		Timeout:       60,
		Weight:        1.5,
	}

	assert.Equal(t, "participant-1", participant.ParticipantID)
	assert.Equal(t, "AI Expert", participant.Name)
	assert.Equal(t, "proposer", participant.Role)
	assert.Equal(t, "deepseek", participant.LLMProvider)
	assert.Equal(t, "deepseek-chat", participant.LLMModel)
	assert.Equal(t, 3, participant.MaxRounds)
	assert.Equal(t, 60, participant.Timeout)
	assert.Equal(t, 1.5, participant.Weight)
}

// TestValidationConfigRequest_Struct tests validation config struct fields
func TestValidationConfigRequest_Struct(t *testing.T) {
	t.Parallel()
	config := ValidationConfigRequest{
		EnableValidation:    true,
		EnablePolish:        true,
		ValidationTimeout:   120,
		PolishTimeout:       60,
		MinConfidenceToSkip: 0.9,
		MaxValidationRounds: 3,
		ShowPhaseIndicators: true,
	}

	assert.True(t, config.EnableValidation)
	assert.True(t, config.EnablePolish)
	assert.Equal(t, 120, config.ValidationTimeout)
	assert.Equal(t, 60, config.PolishTimeout)
	assert.Equal(t, 0.9, config.MinConfidenceToSkip)
	assert.Equal(t, 3, config.MaxValidationRounds)
	assert.True(t, config.ShowPhaseIndicators)
}

// TestDebateError_Extended tests debateError implementation with various messages
func TestDebateError_Extended(t *testing.T) {
	t.Parallel()
	err := &debateError{message: "test error message"}

	assert.Equal(t, "test error message", err.Error())
}
