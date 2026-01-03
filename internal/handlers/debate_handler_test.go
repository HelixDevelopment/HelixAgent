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

	"github.com/superagent/superagent/internal/services"
)

func setupDebateTestRouter() (*gin.Engine, *DebateHandler) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Create handler without services (will store debates but won't run them)
	handler := NewDebateHandler(nil, nil, logger)

	router := gin.New()
	v1 := router.Group("/v1")
	handler.RegisterRoutes(v1)

	return router, handler
}

func TestNewDebateHandler(t *testing.T) {
	logger := logrus.New()
	handler := NewDebateHandler(nil, nil, logger)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.activeDebates)
	assert.Equal(t, logger, handler.logger)
}

func TestCreateDebate(t *testing.T) {
	router, _ := setupDebateTestRouter()

	tests := []struct {
		name           string
		request        CreateDebateRequest
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "valid debate request with minimal fields",
			request: CreateDebateRequest{
				Topic: "Should AI have rights?",
				Participants: []ParticipantConfigRequest{
					{Name: "Alice"},
					{Name: "Bob"},
				},
			},
			expectedStatus: http.StatusAccepted,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Contains(t, body, "debate_id")
				assert.Equal(t, "pending", body["status"])
				assert.Equal(t, "Should AI have rights?", body["topic"])
				assert.Equal(t, float64(3), body["max_rounds"]) // default
				assert.Equal(t, float64(2), body["participants"])
			},
		},
		{
			name: "valid debate request with all fields",
			request: CreateDebateRequest{
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
				MaxRounds:    5,
				Timeout:      600,
				Strategy:     "adversarial",
				EnableCognee: true,
				Metadata:     map[string]any{"category": "environment"},
			},
			expectedStatus: http.StatusAccepted,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "custom-debate-id", body["debate_id"])
				assert.Equal(t, float64(5), body["max_rounds"])
				assert.Equal(t, float64(600), body["timeout"])
			},
		},
		{
			name: "missing topic",
			request: CreateDebateRequest{
				Participants: []ParticipantConfigRequest{
					{Name: "Alice"},
					{Name: "Bob"},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing participants",
			request: CreateDebateRequest{
				Topic: "Test topic",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "only one participant",
			request: CreateDebateRequest{
				Topic: "Test topic",
				Participants: []ParticipantConfigRequest{
					{Name: "Alice"},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}

func TestGetDebate(t *testing.T) {
	router, handler := setupDebateTestRouter()

	// Pre-populate a debate
	handler.mu.Lock()
	handler.activeDebates["test-debate-1"] = &debateState{
		Config: &services.DebateConfig{
			DebateID:  "test-debate-1",
			Topic:     "Test topic",
			MaxRounds: 3,
		},
		Status:    "running",
		StartTime: time.Now(),
	}
	handler.mu.Unlock()

	tests := []struct {
		name           string
		debateID       string
		expectedStatus int
	}{
		{
			name:           "existing debate",
			debateID:       "test-debate-1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-existing debate",
			debateID:       "non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/debates/"+tt.debateID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, tt.debateID, response["debate_id"])
				assert.Equal(t, "running", response["status"])
			}
		})
	}
}

func TestGetDebateStatus(t *testing.T) {
	router, handler := setupDebateTestRouter()

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
		Status:    "running",
		StartTime: now,
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
		name           string
		debateID       string
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "running debate with progress info",
			debateID:       "running-debate",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "running", body["status"])
				assert.Equal(t, float64(5), body["max_rounds"])
				assert.Equal(t, float64(600), body["timeout_seconds"])
			},
		},
		{
			name:           "completed debate with duration",
			debateID:       "completed-debate",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "completed", body["status"])
				assert.Contains(t, body, "end_time")
				assert.Contains(t, body, "duration_seconds")
			},
		},
		{
			name:           "failed debate with error",
			debateID:       "failed-debate",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "failed", body["status"])
				assert.Equal(t, "provider timeout", body["error"])
			},
		},
		{
			name:           "non-existing debate",
			debateID:       "non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/debates/"+tt.debateID+"/status", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}

func TestGetDebateResults(t *testing.T) {
	router, handler := setupDebateTestRouter()

	// Pre-populate debates
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
				FinalPosition: "We agree on this.",
			},
		},
	}
	handler.activeDebates["running-no-result"] = &debateState{
		Config: &services.DebateConfig{
			DebateID: "running-no-result",
			Topic:    "Running topic",
		},
		Status:    "running",
		StartTime: now,
	}
	handler.activeDebates["completed-no-result"] = &debateState{
		Config: &services.DebateConfig{
			DebateID: "completed-no-result",
			Topic:    "Weird topic",
		},
		Status:    "completed",
		StartTime: now,
		EndTime:   &endTime,
		Result:    nil, // No result despite completion
	}
	handler.mu.Unlock()

	tests := []struct {
		name           string
		debateID       string
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "completed debate with results",
			debateID:       "completed-with-result",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "completed-with-result", body["debate_id"])
				assert.Equal(t, true, body["success"])
			},
		},
		{
			name:           "running debate - no results yet",
			debateID:       "running-no-result",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "completed without result - internal error",
			debateID:       "completed-no-result",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "non-existing debate",
			debateID:       "non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/debates/"+tt.debateID+"/results", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}

func TestListDebates(t *testing.T) {
	router, handler := setupDebateTestRouter()

	// Pre-populate debates
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

	tests := []struct {
		name           string
		query          string
		expectedCount  int
		expectedStatus int
	}{
		{
			name:           "list all debates",
			query:          "",
			expectedCount:  3,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by running status",
			query:          "?status=running",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by completed status",
			query:          "?status=completed",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by pending status",
			query:          "?status=pending",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by non-existing status",
			query:          "?status=cancelled",
			expectedCount:  0,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/debates"+tt.query, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, float64(tt.expectedCount), response["count"])
			debates := response["debates"].([]interface{})
			assert.Len(t, debates, tt.expectedCount)
		})
	}
}

func TestDeleteDebate(t *testing.T) {
	router, handler := setupDebateTestRouter()

	// Pre-populate a debate
	handler.mu.Lock()
	handler.activeDebates["deletable-debate"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "deletable-debate", Topic: "Topic"},
		Status:    "completed",
		StartTime: time.Now(),
	}
	handler.mu.Unlock()

	tests := []struct {
		name           string
		debateID       string
		expectedStatus int
	}{
		{
			name:           "delete existing debate",
			debateID:       "deletable-debate",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "delete non-existing debate",
			debateID:       "non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/v1/debates/"+tt.debateID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "Debate deleted", response["message"])
				assert.Equal(t, tt.debateID, response["debate_id"])

				// Verify debate was actually deleted
				handler.mu.RLock()
				_, exists := handler.activeDebates[tt.debateID]
				handler.mu.RUnlock()
				assert.False(t, exists)
			}
		})
	}
}

func TestDebateRouteRegistration(t *testing.T) {
	router, handler := setupDebateTestRouter()

	// Pre-populate a debate for routes that need it
	now := time.Now()
	endTime := now.Add(5 * time.Minute)
	handler.mu.Lock()
	handler.activeDebates["test-id"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "test-id", Topic: "Test topic"},
		Status:    "completed",
		StartTime: now,
		EndTime:   &endTime,
		Result: &services.DebateResult{
			DebateID: "test-id",
			Topic:    "Test topic",
			Success:  true,
		},
	}
	handler.mu.Unlock()

	// Test all route patterns exist
	routes := []struct {
		method         string
		path           string
		expectedStatus int // Expected status that proves route exists
	}{
		{http.MethodPost, "/v1/debates", http.StatusAccepted},
		{http.MethodGet, "/v1/debates", http.StatusOK},
		{http.MethodGet, "/v1/debates/test-id", http.StatusOK},
		{http.MethodGet, "/v1/debates/test-id/status", http.StatusOK},
		{http.MethodGet, "/v1/debates/test-id/results", http.StatusOK},
		{http.MethodDelete, "/v1/debates/test-id", http.StatusOK},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// Re-create the debate for DELETE test (since it gets deleted)
			if route.method == http.MethodDelete {
				handler.mu.Lock()
				handler.activeDebates["test-id"] = &debateState{
					Config:    &services.DebateConfig{DebateID: "test-id", Topic: "Test topic"},
					Status:    "completed",
					StartTime: now,
				}
				handler.mu.Unlock()
			}

			var req *http.Request
			if route.method == http.MethodPost {
				body := `{"topic": "test", "participants": [{"name": "A"}, {"name": "B"}]}`
				req = httptest.NewRequest(route.method, route.path, bytes.NewBufferString(body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(route.method, route.path, nil)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should return expected status (route exists and works)
			assert.Equal(t, route.expectedStatus, w.Code, "Route %s %s should return %d", route.method, route.path, route.expectedStatus)
		})
	}
}

func TestParticipantDefaults(t *testing.T) {
	router, handler := setupDebateTestRouter()

	request := CreateDebateRequest{
		Topic: "Test defaults",
		Participants: []ParticipantConfigRequest{
			{Name: "First"},      // Should get role: proposer
			{Name: "Second"},     // Should get role: critic
			{Name: "Third"},      // Should get role: debater
		},
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

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

	// Check default providers
	for _, p := range state.Config.Participants {
		assert.Equal(t, "openai", p.LLMProvider)
		assert.Equal(t, "gpt-4", p.LLMModel)
		assert.Equal(t, 1.0, p.Weight)
	}
}

func TestDebateError(t *testing.T) {
	err := &debateError{message: "test error"}
	assert.Equal(t, "test error", err.Error())
}

func TestConcurrentDebateAccess(t *testing.T) {
	_, handler := setupDebateTestRouter()

	// Pre-populate a debate
	handler.mu.Lock()
	handler.activeDebates["concurrent-test"] = &debateState{
		Config:    &services.DebateConfig{DebateID: "concurrent-test", Topic: "Topic"},
		Status:    "running",
		StartTime: time.Now(),
	}
	handler.mu.Unlock()

	// Concurrent reads and writes
	done := make(chan bool)

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				handler.mu.RLock()
				_ = handler.activeDebates["concurrent-test"]
				handler.mu.RUnlock()
			}
			done <- true
		}()
	}

	// Writer goroutines
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				handler.mu.Lock()
				handler.activeDebates["concurrent-test"].Status = "running"
				handler.mu.Unlock()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}
}
