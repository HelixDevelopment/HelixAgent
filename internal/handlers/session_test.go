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

func newTestSessionLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	return logger
}

func TestNewSessionHandler(t *testing.T) {
	logger := newTestSessionLogger()
	handler := NewSessionHandler(logger)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.sessions)
	assert.NotNil(t, handler.log)
	assert.Empty(t, handler.sessions)
}

func TestSessionHandler_CreateSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestSessionLogger()
	handler := NewSessionHandler(logger)

	t.Run("creates session successfully", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := CreateSessionRequest{
			UserID:        "test-user-123",
			MemoryEnabled: false,
			TTLHours:      24,
		}
		jsonBody, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.CreateSession(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response SessionResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)
		assert.NotEmpty(t, response.SessionID)
		assert.Equal(t, "test-user-123", response.UserID)
		assert.Equal(t, "active", response.Status)
		assert.Equal(t, 0, response.RequestCount)
	})

	t.Run("creates session with memory enabled", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := CreateSessionRequest{
			UserID:        "test-user-memory",
			MemoryEnabled: true,
			TTLHours:      12,
		}
		jsonBody, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.CreateSession(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response SessionResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)
	})

	t.Run("creates session with initial context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := CreateSessionRequest{
			UserID: "test-user-context",
			InitialContext: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
		}
		jsonBody, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.CreateSession(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response SessionResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotNil(t, response.Context)
	})

	t.Run("uses default TTL when not provided", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := CreateSessionRequest{
			UserID:   "test-user-default-ttl",
			TTLHours: 0, // Should default to 24
		}
		jsonBody, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.CreateSession(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response SessionResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		// Should expire in ~24 hours
		assert.True(t, response.ExpiresAt.After(time.Now().Add(23*time.Hour)))
	})

	t.Run("caps TTL at 7 days", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := CreateSessionRequest{
			UserID:   "test-user-max-ttl",
			TTLHours: 1000, // Should cap at 168 (7 days)
		}
		jsonBody, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.CreateSession(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response SessionResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		// Should not expire more than 7 days from now
		assert.True(t, response.ExpiresAt.Before(time.Now().Add(169*time.Hour)))
	})

	t.Run("returns error for invalid request", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Missing required user_id
		c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader([]byte(`{}`)))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.CreateSession(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader([]byte(`{invalid json}`)))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.CreateSession(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestSessionHandler_GetSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestSessionLogger()
	handler := NewSessionHandler(logger)

	// First create a session
	createW := httptest.NewRecorder()
	createC, _ := gin.CreateTestContext(createW)
	reqBody := CreateSessionRequest{
		UserID: "test-user-get",
		InitialContext: map[string]interface{}{
			"test": "context",
		},
	}
	jsonBody, _ := json.Marshal(reqBody)
	createC.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
	createC.Request.Header.Set("Content-Type", "application/json")
	handler.CreateSession(createC)

	var createResp SessionResponse
	json.Unmarshal(createW.Body.Bytes(), &createResp)
	sessionID := createResp.SessionID

	t.Run("gets session successfully", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: sessionID}}
		c.Request = httptest.NewRequest("GET", "/v1/sessions/"+sessionID, nil)

		handler.GetSession(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response SessionResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, sessionID, response.SessionID)
		assert.Equal(t, "test-user-get", response.UserID)
	})

	t.Run("gets session with context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: sessionID}}
		c.Request = httptest.NewRequest("GET", "/v1/sessions/"+sessionID+"?includeContext=true", nil)

		handler.GetSession(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response SessionResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotNil(t, response.Context)
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "non-existent-session"}}
		c.Request = httptest.NewRequest("GET", "/v1/sessions/non-existent-session", nil)

		handler.GetSession(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestSessionHandler_TerminateSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestSessionLogger()
	handler := NewSessionHandler(logger)

	t.Run("terminates session gracefully", func(t *testing.T) {
		// Create a session first
		createW := httptest.NewRecorder()
		createC, _ := gin.CreateTestContext(createW)
		reqBody := CreateSessionRequest{UserID: "test-user-terminate"}
		jsonBody, _ := json.Marshal(reqBody)
		createC.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
		createC.Request.Header.Set("Content-Type", "application/json")
		handler.CreateSession(createC)

		var createResp SessionResponse
		json.Unmarshal(createW.Body.Bytes(), &createResp)
		sessionID := createResp.SessionID

		// Terminate gracefully
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: sessionID}}
		c.Request = httptest.NewRequest("DELETE", "/v1/sessions/"+sessionID, nil)

		handler.TerminateSession(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response SessionResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, "terminated", response.Status)

		// Session should still exist (graceful termination)
		assert.NotNil(t, handler.GetSessionByID(sessionID))
	})

	t.Run("terminates session immediately", func(t *testing.T) {
		// Create a session first
		createW := httptest.NewRecorder()
		createC, _ := gin.CreateTestContext(createW)
		reqBody := CreateSessionRequest{UserID: "test-user-immediate"}
		jsonBody, _ := json.Marshal(reqBody)
		createC.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
		createC.Request.Header.Set("Content-Type", "application/json")
		handler.CreateSession(createC)

		var createResp SessionResponse
		json.Unmarshal(createW.Body.Bytes(), &createResp)
		sessionID := createResp.SessionID

		// Terminate immediately
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: sessionID}}
		c.Request = httptest.NewRequest("DELETE", "/v1/sessions/"+sessionID+"?graceful=false", nil)

		handler.TerminateSession(c)

		assert.Equal(t, http.StatusOK, w.Code)

		// Session should be deleted (immediate termination)
		assert.Nil(t, handler.GetSessionByID(sessionID))
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "non-existent-session"}}
		c.Request = httptest.NewRequest("DELETE", "/v1/sessions/non-existent-session", nil)

		handler.TerminateSession(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestSessionHandler_ListSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestSessionLogger()
	handler := NewSessionHandler(logger)

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		reqBody := CreateSessionRequest{
			UserID: "test-user-list",
		}
		jsonBody, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.CreateSession(c)
	}

	// Create a session for a different user
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reqBody := CreateSessionRequest{
		UserID: "different-user",
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	handler.CreateSession(c)

	t.Run("lists all sessions", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/v1/sessions", nil)

		handler.ListSessions(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(4), response["count"])
	})

	t.Run("filters by user_id", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/v1/sessions?user_id=test-user-list", nil)

		handler.ListSessions(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(3), response["count"])
	})

	t.Run("filters by status", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/v1/sessions?status=active", nil)

		handler.ListSessions(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		// All sessions should be active
		assert.GreaterOrEqual(t, response["count"].(float64), float64(4))
	})
}

func TestSessionHandler_UpdateSessionContext(t *testing.T) {
	logger := newTestSessionLogger()
	handler := NewSessionHandler(logger)

	// Create a session first
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reqBody := CreateSessionRequest{
		UserID: "test-user-update",
	}
	jsonBody, _ := json.Marshal(reqBody)
	c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	handler.CreateSession(c)

	var createResp SessionResponse
	json.Unmarshal(w.Body.Bytes(), &createResp)
	sessionID := createResp.SessionID

	t.Run("updates context successfully", func(t *testing.T) {
		err := handler.UpdateSessionContext(sessionID, map[string]interface{}{
			"new_key": "new_value",
		})
		assert.NoError(t, err)

		session := handler.GetSessionByID(sessionID)
		require.NotNil(t, session)
		assert.Equal(t, "new_value", session.Context["new_key"])
		assert.Equal(t, 1, session.RequestCount)
	})

	t.Run("updates context on existing context", func(t *testing.T) {
		err := handler.UpdateSessionContext(sessionID, map[string]interface{}{
			"another_key": 42,
		})
		assert.NoError(t, err)

		session := handler.GetSessionByID(sessionID)
		require.NotNil(t, session)
		assert.Equal(t, "new_value", session.Context["new_key"]) // Previous value preserved
		assert.Equal(t, 42, session.Context["another_key"])
		assert.Equal(t, 2, session.RequestCount)
	})

	t.Run("returns nil for non-existent session", func(t *testing.T) {
		err := handler.UpdateSessionContext("non-existent", map[string]interface{}{
			"key": "value",
		})
		assert.NoError(t, err) // Returns nil for non-existent sessions
	})
}

func TestSessionHandler_GetSessionByID(t *testing.T) {
	logger := newTestSessionLogger()
	handler := NewSessionHandler(logger)

	t.Run("returns nil for non-existent session", func(t *testing.T) {
		session := handler.GetSessionByID("non-existent")
		assert.Nil(t, session)
	})

	t.Run("returns session when exists", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		reqBody := CreateSessionRequest{
			UserID: "test-user-getbyid",
		}
		jsonBody, _ := json.Marshal(reqBody)
		c.Request = httptest.NewRequest("POST", "/v1/sessions", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.CreateSession(c)

		var createResp SessionResponse
		json.Unmarshal(w.Body.Bytes(), &createResp)
		sessionID := createResp.SessionID

		session := handler.GetSessionByID(sessionID)
		require.NotNil(t, session)
		assert.Equal(t, sessionID, session.ID)
		assert.Equal(t, "test-user-getbyid", session.UserID)
	})
}
