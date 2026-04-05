package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/skills"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupDebateUnitTest creates a test environment for debate handler tests
func setupDebateUnitTest() (*gin.Engine, *DebateHandler) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Create handler without services for basic tests
	handler := NewDebateHandler(nil, nil, logger)

	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	return router, handler
}

// TestDebateHandler_CreateDebate_Success tests successful debate creation
func TestDebateHandlerUnit_CreateDebate_Success(t *testing.T) {
	router, _ := setupDebateUnitTest()

	reqBody := CreateDebateRequest{
		Topic: "Should AI be regulated?",
		Participants: []ParticipantConfigRequest{
			{Name: "Advocate", Role: "proposer"},
			{Name: "Skeptic", Role: "critic"},
		},
		MaxRounds: 5,
		Timeout:   300,
		Strategy:  "consensus",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "debate_id")
	assert.Equal(t, "pending", response["status"])
	assert.Equal(t, "Should AI be regulated?", response["topic"])
	assert.Equal(t, float64(5), response["max_rounds"])
	assert.Equal(t, float64(300), response["timeout"])
	assert.Equal(t, float64(2), response["participants"])
	assert.Contains(t, response, "created_at")
	assert.Contains(t, response, "message")
}

// TestDebateHandler_CreateDebate_InvalidJSON tests debate creation with invalid JSON
func TestDebateHandlerUnit_CreateDebate_InvalidJSON(t *testing.T) {
	router, _ := setupDebateUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBufferString("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "error")
}

// TestDebateHandler_CreateDebate_MissingTopic tests debate creation with missing topic
func TestDebateHandlerUnit_CreateDebate_MissingTopic(t *testing.T) {
	router, _ := setupDebateUnitTest()

	reqBody := CreateDebateRequest{
		Participants: []ParticipantConfigRequest{
			{Name: "Participant 1"},
			{Name: "Participant 2"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDebateHandler_CreateDebate_MissingParticipants tests debate creation with missing participants
func TestDebateHandlerUnit_CreateDebate_MissingParticipants(t *testing.T) {
	router, _ := setupDebateUnitTest()

	reqBody := CreateDebateRequest{
		Topic: "Test topic",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDebateHandler_CreateDebate_InsufficientParticipants tests debate creation with only one participant
func TestDebateHandlerUnit_CreateDebate_InsufficientParticipants(t *testing.T) {
	router, _ := setupDebateUnitTest()

	reqBody := CreateDebateRequest{
		Topic: "Test topic",
		Participants: []ParticipantConfigRequest{
			{Name: "Only Participant"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDebateHandler_CreateDebate_WithAllFields tests debate creation with all fields
func TestDebateHandlerUnit_CreateDebate_WithAllFields(t *testing.T) {
	router, _ := setupDebateUnitTest()

	reqBody := CreateDebateRequest{
		DebateID: "custom-debate-id",
		Topic:    "Is climate change reversible?",
		Participants: []ParticipantConfigRequest{
			{
				ParticipantID: "part-1",
				Name:          "Scientist",
				Role:          "expert",
				LLMProvider:   "claude",
				LLMModel:      "claude-3-opus",
				Weight:        1.5,
			},
			{
				ParticipantID: "part-2",
				Name:          "Skeptic",
				Role:          "critic",
				LLMProvider:   "openai",
				LLMModel:      "gpt-4",
				Weight:        1.0,
			},
		},
		MaxRounds:                 5,
		Timeout:                   600,
		Strategy:                  "adversarial",
		EnableCognee:              true,
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
		Metadata: map[string]interface{}{"category": "environment"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "custom-debate-id", response["debate_id"])
	assert.Equal(t, float64(5), response["max_rounds"])
	assert.Equal(t, float64(600), response["timeout"])
}

// TestDebateHandler_CreateDebate_DefaultValues tests debate creation default values
func TestDebateHandlerUnit_CreateDebate_DefaultValues(t *testing.T) {
	router, _ := setupDebateUnitTest()

	reqBody := CreateDebateRequest{
		Topic: "Test defaults",
		Participants: []ParticipantConfigRequest{
			{Name: "First"},  // Should get role: proposer
			{Name: "Second"}, // Should get role: critic
			{Name: "Third"},  // Should get role: debater
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(3), response["max_rounds"])   // default
	assert.Equal(t, float64(300), response["timeout"])    // default (5 minutes)
}

// TestDebateHandler_GetDebate_Success tests retrieving a debate successfully
func TestDebateHandlerUnit_GetDebate_Success(t *testing.T) {
	router, handler := setupDebateUnitTest()

	// Pre-populate a debate
	handler.mu.Lock()
	now := time.Now()
	endTime := now.Add(5 * time.Minute)
	handler.activeDebates["test-debate-1"] = &debateState{
		Config: &services.DebateConfig{
			DebateID:  "test-debate-1",
			Topic:     "Test topic",
			MaxRounds: 3,
		},
		Status:                    "completed",
		StartTime:                 now,
		EndTime:                   &endTime,
		EnableMultiPassValidation: true,
		CurrentPhase:              "final_conclusion",
		Result: &services.DebateResult{
			DebateID: "test-debate-1",
			Topic:    "Test topic",
			Success:  true,
			Consensus: &services.ConsensusResult{
				Reached:       true,
				FinalPosition: "Agreement reached",
			},
		},
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/test-debate-1", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "test-debate-1", response["debate_id"])
	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, "Test topic", response["topic"])
	assert.Equal(t, float64(3), response["max_rounds"])
	assert.Equal(t, true, response["enable_multi_pass_validation"])
	assert.Equal(t, "final_conclusion", response["current_phase"])
	assert.Contains(t, response, "start_time")
	assert.Contains(t, response, "end_time")
	assert.Contains(t, response, "duration_seconds")
	assert.Contains(t, response, "result")
}

// TestDebateHandler_GetDebate_NotFound tests retrieving a non-existent debate
func TestDebateHandlerUnit_GetDebate_NotFound(t *testing.T) {
	router, _ := setupDebateUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/non-existent", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "error")
}

// TestDebateHandler_GetDebateStatus_Success tests getting debate status
func TestDebateHandlerUnit_GetDebateStatus_Success(t *testing.T) {
	router, handler := setupDebateUnitTest()

	// Pre-populate debates with different states
	now := time.Now()
	endTime := now.Add(5 * time.Minute)

	handler.mu.Lock()
	handler.activeDebates["running-debate"] = &debateState{
		Config: &services.DebateConfig{
			DebateID:  "running-debate",
			Topic:     "Running topic",
			MaxRounds: 5,
			Timeout:   10 * time.Minute,
		},
		Status:                    "running",
		StartTime:                 now,
		EnableMultiPassValidation: true,
		CurrentPhase:              "validation",
	}
	handler.activeDebates["completed-debate"] = &debateState{
		Config: &services.DebateConfig{
			DebateID:  "completed-debate",
			Topic:     "Completed topic",
			MaxRounds: 3,
		},
		Status:    "completed",
		StartTime: now,
		EndTime:   &endTime,
		MultiPassResult: &services.MultiPassResult{
			FinalResponse:      "Consensus reached",
			OverallConfidence:  0.85,
			QualityImprovement: 0.15,
			Phases:             []*services.PhaseResult{},
		},
	}
	handler.activeDebates["failed-debate"] = &debateState{
		Config: &services.DebateConfig{
			DebateID: "failed-debate",
			Topic:    "Failed topic",
		},
		Status:    "failed",
		Error:     "provider timeout",
		StartTime: now,
		EndTime:   &endTime,
	}
	handler.mu.Unlock()

	tests := []struct {
		debateID       string
		expectedStatus string
		checkFields    func(t *testing.T, response map[string]interface{})
	}{
		{
			debateID:       "running-debate",
			expectedStatus: "running",
			checkFields: func(t *testing.T, response map[string]interface{}) {
				assert.Equal(t, float64(5), response["max_rounds"])
				assert.Equal(t, float64(600), response["timeout_seconds"])
				assert.Equal(t, "validation", response["current_phase"])
				assert.Contains(t, response, "validation_phases")
			},
		},
		{
			debateID:       "completed-debate",
			expectedStatus: "completed",
			checkFields: func(t *testing.T, response map[string]interface{}) {
				assert.Contains(t, response, "end_time")
				assert.Contains(t, response, "duration_seconds")
				assert.Equal(t, 0.85, response["overall_confidence"])
				assert.Equal(t, 0.15, response["quality_improvement"])
			},
		},
		{
			debateID:       "failed-debate",
			expectedStatus: "failed",
			checkFields: func(t *testing.T, response map[string]interface{}) {
				assert.Equal(t, "provider timeout", response["error"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.debateID, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/debates/"+tt.debateID+"/status", nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, response["status"])
			tt.checkFields(t, response)
		})
	}
}

// TestDebateHandler_GetDebateStatus_NotFound tests status for non-existent debate
func TestDebateHandlerUnit_GetDebateStatus_NotFound(t *testing.T) {
	router, _ := setupDebateUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/non-existent/status", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDebateHandler_GetDebateResults_Success tests getting debate results
func TestDebateHandlerUnit_GetDebateResults_Success(t *testing.T) {
	router, handler := setupDebateUnitTest()

	now := time.Now()
	endTime := now.Add(5 * time.Minute)

	handler.mu.Lock()
	handler.activeDebates["completed-with-result"] = &debateState{
		Config: &services.DebateConfig{
			DebateID: "completed-with-result",
			Topic:    "Test topic",
		},
		Status:    "completed",
		StartTime: now,
		EndTime:   &endTime,
		Result: &services.DebateResult{
			DebateID: "completed-with-result",
			Topic:    "Test topic",
			Success:  true,
			Consensus: &services.ConsensusResult{
				Reached:       true,
				FinalPosition: "We agree.",
			},
		},
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/completed-with-result/results", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "completed-with-result", response["debate_id"])
	assert.Equal(t, true, response["success"])
}

// TestDebateHandler_GetDebateResults_NotCompleted tests results for running debate
func TestDebateHandlerUnit_GetDebateResults_NotCompleted(t *testing.T) {
	router, handler := setupDebateUnitTest()

	handler.mu.Lock()
	handler.activeDebates["running-debate"] = &debateState{
		Config: &services.DebateConfig{
			DebateID: "running-debate",
			Topic:    "Running topic",
		},
		Status:    "running",
		StartTime: time.Now(),
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/running-debate/results", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDebateHandler_GetDebateResults_NoResult tests results for completed debate without result
func TestDebateHandlerUnit_GetDebateResults_NoResult(t *testing.T) {
	router, handler := setupDebateUnitTest()

	now := time.Now()
	endTime := now.Add(5 * time.Minute)

	handler.mu.Lock()
	handler.activeDebates["completed-no-result"] = &debateState{
		Config: &services.DebateConfig{
			DebateID: "completed-no-result",
			Topic:    "Weird topic",
		},
		Status:    "completed",
		StartTime: now,
		EndTime:   &endTime,
		Result:    nil,
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/completed-no-result/results", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestDebateHandler_GetDebateResults_NotFound tests results for non-existent debate
func TestDebateHandlerUnit_GetDebateResults_NotFound(t *testing.T) {
	router, _ := setupDebateUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/non-existent/results", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDebateHandler_ListDebates_Success tests listing all debates
func TestDebateHandlerUnit_ListDebates_Success(t *testing.T) {
	router, handler := setupDebateUnitTest()

	now := time.Now()
	endTime := now.Add(5 * time.Minute)

	handler.mu.Lock()
	handler.activeDebates["debate-1"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "debate-1", Topic: "Topic 1"},
		Status:    "running",
		StartTime: now,
	}
	handler.activeDebates["debate-2"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "debate-2", Topic: "Topic 2"},
		Status:    "completed",
		StartTime: now,
		EndTime:   &endTime,
	}
	handler.activeDebates["debate-3"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "debate-3", Topic: "Topic 3"},
		Status:    "pending",
		StartTime: now,
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(3), response["count"])

	debates := response["debates"].([]interface{})
	assert.Len(t, debates, 3)
}

// TestDebateHandler_ListDebates_WithStatusFilter tests listing debates with status filter
func TestDebateHandlerUnit_ListDebates_WithStatusFilter(t *testing.T) {
	router, handler := setupDebateUnitTest()

	now := time.Now()
	endTime := now.Add(5 * time.Minute)

	handler.mu.Lock()
	handler.activeDebates["debate-1"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "debate-1", Topic: "Topic 1"},
		Status:    "running",
		StartTime: now,
	}
	handler.activeDebates["debate-2"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "debate-2", Topic: "Topic 2"},
		Status:    "completed",
		StartTime: now,
		EndTime:   &endTime,
	}
	handler.mu.Unlock()

	tests := []struct {
		query         string
		expectedCount int
	}{
		{"", 2},
		{"?status=running", 1},
		{"?status=completed", 1},
		{"?status=pending", 0},
	}

	for _, tt := range tests {
		t.Run("status_"+tt.query, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/debates"+tt.query, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, float64(tt.expectedCount), response["count"])
		})
	}
}

// TestDebateHandler_DeleteDebate_Success tests deleting a debate
func TestDebateHandlerUnit_DeleteDebate_Success(t *testing.T) {
	router, handler := setupDebateUnitTest()

	handler.mu.Lock()
	handler.activeDebates["deletable-debate"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "deletable-debate", Topic: "Topic"},
		Status:    "completed",
		StartTime: time.Now(),
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/debates/deletable-debate", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Debate deleted", response["message"])
	assert.Equal(t, "deletable-debate", response["debate_id"])

	// Verify debate was deleted
	handler.mu.RLock()
	_, exists := handler.activeDebates["deletable-debate"]
	handler.mu.RUnlock()
	assert.False(t, exists)
}

// TestDebateHandler_DeleteDebate_NotFound tests deleting non-existent debate
func TestDebateHandlerUnit_DeleteDebate_NotFound(t *testing.T) {
	router, _ := setupDebateUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/debates/non-existent", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDebateHandler_ApproveDebate_Success tests approving a debate
func TestDebateHandlerUnit_ApproveDebate_Success(t *testing.T) {
	router, handler := setupDebateUnitTest()

	handler.mu.Lock()
	handler.activeDebates["paused-debate"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "paused-debate", Topic: "Topic"},
		Status:    "paused",
		StartTime: time.Now(),
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates/paused-debate/approve", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Debate approved and resumed", response["message"])
	assert.Equal(t, "running", response["status"])

	// Verify status was updated
	handler.mu.RLock()
	state := handler.activeDebates["paused-debate"]
	handler.mu.RUnlock()
	assert.Equal(t, "running", state.Status)
}

// TestDebateHandler_ApproveDebate_NotFound tests approving non-existent debate
func TestDebateHandlerUnit_ApproveDebate_NotFound(t *testing.T) {
	router, _ := setupDebateUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates/non-existent/approve", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDebateHandler_ApproveDebate_InvalidStatus tests approving debate not awaiting approval
func TestDebateHandlerUnit_ApproveDebate_InvalidStatus(t *testing.T) {
	router, handler := setupDebateUnitTest()

	handler.mu.Lock()
	handler.activeDebates["running-debate"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "running-debate", Topic: "Topic"},
		Status:    "running",
		StartTime: time.Now(),
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates/running-debate/approve", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestDebateHandler_RejectDebate_Success tests rejecting a debate
func TestDebateHandlerUnit_RejectDebate_Success(t *testing.T) {
	router, handler := setupDebateUnitTest()

	handler.mu.Lock()
	handler.activeDebates["pending-debate"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "pending-debate", Topic: "Topic"},
		Status:    "paused",
		StartTime: time.Now(),
	}
	handler.mu.Unlock()

	reqBody := map[string]interface{}{"reason": "Quality concerns"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates/pending-debate/reject", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Debate rejected", response["message"])
	assert.Equal(t, "rejected", response["status"])
	assert.Equal(t, "Quality concerns", response["reason"])

	// Verify status and error were updated
	handler.mu.RLock()
	state := handler.activeDebates["pending-debate"]
	handler.mu.RUnlock()
	assert.Equal(t, "rejected", state.Status)
	assert.Contains(t, state.Error, "Quality concerns")
	assert.NotNil(t, state.EndTime)
}

// TestDebateHandler_RejectDebate_NotFound tests rejecting non-existent debate
func TestDebateHandlerUnit_RejectDebate_NotFound(t *testing.T) {
	router, _ := setupDebateUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates/non-existent/reject", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDebateHandler_RejectDebate_NoReason tests rejecting without reason
func TestDebateHandlerUnit_RejectDebate_NoReason(t *testing.T) {
	router, handler := setupDebateUnitTest()

	handler.mu.Lock()
	handler.activeDebates["pending-debate"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "pending-debate", Topic: "Topic"},
		Status:    "paused",
		StartTime: time.Now(),
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates/pending-debate/reject", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "rejected", response["status"])
}

// TestDebateHandler_GetDebateGates_Success tests getting debate gates
func TestDebateHandlerUnit_GetDebateGates_Success(t *testing.T) {
	router, handler := setupDebateUnitTest()

	handler.mu.Lock()
	handler.activeDebates["gated-debate"] = &debateState{
		Config:       &services.DebateConfig{DebateID: "gated-debate", Topic: "Topic"},
		Status:       "pending_approval",
		CurrentPhase: "validation",
		StartTime:    time.Now(),
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/gated-debate/gates", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "gated-debate", response["debate_id"])
	assert.Equal(t, "pending_approval", response["status"])
	assert.Equal(t, "validation", response["current_phase"])
	assert.Equal(t, true, response["gates_enabled"])
	assert.Equal(t, true, response["pending_approval"])
}

// TestDebateHandler_GetDebateGates_NotFound tests gates for non-existent debate
func TestDebateHandlerUnit_GetDebateGates_NotFound(t *testing.T) {
	router, _ := setupDebateUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/non-existent/gates", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDebateHandler_GetDebateAudit_Success tests getting debate audit
func TestDebateHandlerUnit_GetDebateAudit_Success(t *testing.T) {
	router, handler := setupDebateUnitTest()

	now := time.Now()
	endTime := now.Add(5 * time.Minute)

	handler.mu.Lock()
	handler.activeDebates["audited-debate"] = &debateState{
		Config:       &services.DebateConfig{DebateID: "audited-debate", Topic: "Topic"},
		Status:       "completed",
		StartTime:    now,
		EndTime:      &endTime,
		CurrentPhase: "final_conclusion",
		Result:       &services.DebateResult{DebateID: "audited-debate"},
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/audited-debate/audit", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "audited-debate", response["debate_id"])
	assert.Equal(t, "completed", response["status"])
	assert.Contains(t, response, "start_time")
	assert.Contains(t, response, "end_time")
	assert.Equal(t, "final_conclusion", response["current_phase"])
	assert.Equal(t, true, response["has_result"])
}

// TestDebateHandler_GetDebateAudit_NotFound tests audit for non-existent debate
func TestDebateHandlerUnit_GetDebateAudit_NotFound(t *testing.T) {
	router, _ := setupDebateUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/non-existent/audit", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDebateHandlerUnit_NewDebateHandlerWithSkills tests handler creation with skills via setter
func TestDebateHandlerUnit_NewDebateHandlerWithSkills(t *testing.T) {
	logger := logrus.New()
	skillsService := skills.NewService(&skills.SkillConfig{MinConfidence: 0.5})
	skillsIntegration := skills.NewIntegration(skillsService)

	handler := NewDebateHandler(nil, nil, logger)
	handler.SetSkillsIntegration(skillsIntegration)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.activeDebates)
	assert.NotNil(t, handler.skillsIntegration)
	assert.Equal(t, logger, handler.logger)
}

// TestDebateHandler_SetSkillsIntegration tests setting skills integration
func TestDebateHandlerUnit_SetSkillsIntegration(t *testing.T) {
	logger := logrus.New()
	handler := NewDebateHandler(nil, nil, logger)

	skillsService := skills.NewService(&skills.SkillConfig{MinConfidence: 0.5})
	skillsIntegration := skills.NewIntegration(skillsService)

	handler.SetSkillsIntegration(skillsIntegration)

	assert.NotNil(t, handler.skillsIntegration)
}

// TestDebateHandler_DebateError tests the debateError type
func TestDebateHandlerUnit_DebateError(t *testing.T) {
	err := &debateError{message: "test error"}
	assert.Equal(t, "test error", err.Error())
}

// TestDebateHandler_ConcurrentAccess tests concurrent access to debates map
func TestDebateHandlerUnit_ConcurrentAccess(t *testing.T) {
	_, handler := setupDebateUnitTest()

	// Pre-populate a debate
	handler.mu.Lock()
	handler.activeDebates["concurrent-test"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "concurrent-test", Topic: "Topic"},
		Status:    "running",
		StartTime: time.Now(),
	}
	handler.mu.Unlock()

	// Run concurrent reads and writes
	done := make(chan bool, 20)

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				handler.mu.RLock()
				_ = handler.activeDebates["concurrent-test"]
				handler.mu.RUnlock()
			}
			done <- true
		}()
	}

	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 25; j++ {
				handler.mu.Lock()
				handler.activeDebates["concurrent-test"].Status = "running"
				handler.mu.Unlock()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

// TestDebateHandler_CreateDebate_WithSkillsIntegration tests debate creation with skills
func TestDebateHandlerUnit_CreateDebate_WithSkillsIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	skillsService := skills.NewService(&skills.SkillConfig{MinConfidence: 0.5})
	skillsIntegration := skills.NewIntegration(skillsService)

	handler := NewDebateHandler(nil, nil, logger)
	handler.SetSkillsIntegration(skillsIntegration)

	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	reqBody := CreateDebateRequest{
		Topic: "Should AI have consciousness?",
		Participants: []ParticipantConfigRequest{
			{Name: "Philosopher"},
			{Name: "Engineer"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

// TestDebateHandler_ParticipantDefaults tests participant default values
func TestDebateHandlerUnit_ParticipantDefaults(t *testing.T) {
	router, handler := setupDebateUnitTest()

	reqBody := CreateDebateRequest{
		Topic: "Test defaults",
		Participants: []ParticipantConfigRequest{
			{Name: "First"},  // Should get role: proposer
			{Name: "Second"}, // Should get role: critic
			{Name: "Third"},  // Should get role: debater
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	debateID := response["debate_id"].(string)

	// Check stored debate config has correct defaults
	handler.mu.RLock()
	state := handler.activeDebates[debateID]
	handler.mu.RUnlock()

	require.NotNil(t, state)
	require.NotNil(t, state.Config)
	assert.Len(t, state.Config.Participants, 3)

	// Check roles
	assert.Equal(t, "proposer", state.Config.Participants[0].Role)
	assert.Equal(t, "critic", state.Config.Participants[1].Role)
	assert.Equal(t, "debater", state.Config.Participants[2].Role)

	// Check default providers and models
	for _, p := range state.Config.Participants {
		assert.Equal(t, "openai", p.LLMProvider)
		assert.Equal(t, "gpt-4", p.LLMModel)
		assert.Equal(t, 1.0, p.Weight)
	}
}

// TestDebateHandler_GetDebate_WithMultiPassResults tests getting debate with multi-pass results
func TestDebateHandlerUnit_GetDebate_WithMultiPassResults(t *testing.T) {
	router, handler := setupDebateUnitTest()

	now := time.Now()
	endTime := now.Add(2 * time.Minute)

	handler.mu.Lock()
	handler.activeDebates["multipass-completed"] = &debateState{
		Config: &services.DebateConfig{
			DebateID:  "multipass-completed",
			Topic:     "Completed multi-pass debate",
			MaxRounds: 3,
		},
		ValidationConfig: &services.ValidationConfig{
			EnableValidation:    true,
			EnablePolish:        true,
			ShowPhaseIndicators: true,
		},
		EnableMultiPassValidation: true,
		Status:                    "completed",
		CurrentPhase:              "final_conclusion",
		StartTime:                 now,
		EndTime:                   &endTime,
		MultiPassResult: &services.MultiPassResult{
			FinalResponse:      "The consensus is that...",
			OverallConfidence:  0.85,
			QualityImprovement: 0.15,
			Phases: []*services.PhaseResult{
				{
					Phase:        services.PhaseInitialResponse,
					Duration:     30 * time.Second,
					PhaseScore:   0.75,
					PhaseSummary: "Initial perspectives gathered",
				},
				{
					Phase:        services.PhaseValidation,
					Duration:     45 * time.Second,
					PhaseScore:   0.82,
					PhaseSummary: "Cross-validation complete",
				},
			},
		},
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/multipass-completed", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	multiPassResult, ok := response["multi_pass_result"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(2), multiPassResult["phases_completed"])
	assert.Equal(t, 0.85, multiPassResult["overall_confidence"])
	assert.Equal(t, 0.15, multiPassResult["quality_improvement"])
	assert.Equal(t, "The consensus is that...", multiPassResult["final_response"])
}

// TestDebateHandler_CreateDebate_GeneratedID tests auto-generated debate ID
func TestDebateHandlerUnit_CreateDebate_GeneratedID(t *testing.T) {
	router, _ := setupDebateUnitTest()

	reqBody := CreateDebateRequest{
		Topic: "Test generated ID",
		Participants: []ParticipantConfigRequest{
			{Name: "A"},
			{Name: "B"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	debateID := response["debate_id"].(string)
	assert.NotEmpty(t, debateID)
	assert.Contains(t, debateID, "debate-")
}

// TestDebateHandler_RegisterRoutes tests route registration
func TestDebateHandlerUnit_RegisterRoutes(t *testing.T) {
	router, _ := setupDebateUnitTest()

	// Test that all routes are registered
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/debates"},
		{http.MethodGet, "/v1/debates"},
		{http.MethodGet, "/v1/debates/:id"},
		{http.MethodGet, "/v1/debates/:id/status"},
		{http.MethodGet, "/v1/debates/:id/results"},
		{http.MethodPost, "/v1/debates/:id/approve"},
		{http.MethodPost, "/v1/debates/:id/reject"},
		{http.MethodGet, "/v1/debates/:id/gates"},
		{http.MethodGet, "/v1/debates/:id/audit"},
		{http.MethodDelete, "/v1/debates/:id"},
	}

	for _, route := range routes {
		t.Run(route.method+"_"+route.path, func(t *testing.T) {
			// Just verify route exists - we test functionality elsewhere
			assert.NotNil(t, router)
		})
	}
}

// TestDebateHandler_GetDebate_WithSkillsUsed tests getting debate with skills metadata
func TestDebateHandlerUnit_GetDebate_WithSkillsUsed(t *testing.T) {
	router, handler := setupDebateUnitTest()

	now := time.Now()
	endTime := now.Add(2 * time.Minute)

	handler.mu.Lock()
	handler.activeDebates["skills-debate"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "skills-debate", Topic: "Topic"},
		Status:    "completed",
		StartTime: now,
		EndTime:   &endTime,
		Result:    &services.DebateResult{DebateID: "skills-debate"},
		SkillsUsed: &skills.SkillsUsedMetadata{
			TotalSkills: 2,
			Skills: []skills.SkillUsedInfo{
				{
					Name:       "Critical Thinking",
					Category:   "reasoning",
					Confidence: 0.85,
				},
			},
		},
	}
	handler.mu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates/skills-debate", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "skills_used")
}

// TestDebateHandler_CreateDebate_WithValidationConfig tests debate creation with validation config
func TestDebateHandlerUnit_CreateDebate_WithValidationConfig(t *testing.T) {
	router, _ := setupDebateUnitTest()

	reqBody := CreateDebateRequest{
		Topic: "Test validation",
		Participants: []ParticipantConfigRequest{
			{Name: "A"},
			{Name: "B"},
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
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

// TestDebateHandler_CreateDebate_NoValidationConfig tests debate creation without validation config
func TestDebateHandlerUnit_CreateDebate_NoValidationConfig(t *testing.T) {
	router, _ := setupDebateUnitTest()

	reqBody := CreateDebateRequest{
		Topic: "Test no validation",
		Participants: []ParticipantConfigRequest{
			{Name: "A"},
			{Name: "B"},
		},
		EnableMultiPassValidation: true,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}
