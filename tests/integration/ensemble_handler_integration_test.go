//go:build integration
// +build integration

// Package integration provides ensemble handler integration tests.
// These tests verify the ensemble HTTP endpoints using httptest against
// a real Gin router with the EnsembleHandler wired in.
package integration

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

	"dev.helix.agent/internal/handlers"
)

func setupEnsembleRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	h := handlers.NewEnsembleHandler(nil, logger)

	r := gin.New()
	r.Use(gin.Recovery())
	api := r.Group("/v1")
	h.RegisterRoutes(api)
	return r
}

// TestEnsembleHandler_CreateTeam verifies that POST /v1/ensemble/teams
// creates a new team and returns 201 with a valid team ID.
func TestEnsembleHandler_CreateTeam(t *testing.T) {
	r := setupEnsembleRouter()

	body := map[string]interface{}{
		"name":        "integration-test-team",
		"description": "Team created during integration test",
		"agents":      []map[string]interface{}{},
		"config": map[string]interface{}{
			"max_parallel":        2,
			"consensus_threshold": 0.7,
			"timeout_seconds":     30,
			"enable_voting":       true,
		},
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ensemble/teams", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "should return 201 Created")

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp["id"], "response should contain a team id")
}

// TestEnsembleHandler_ListTeams verifies that GET /v1/ensemble/teams
// returns 200 with an array body.
func TestEnsembleHandler_ListTeams(t *testing.T) {
	r := setupEnsembleRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/ensemble/teams", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "response should be a JSON array")
}

// TestEnsembleHandler_ListSessions verifies that GET /v1/ensemble/sessions
// returns 200 and a valid JSON response.
func TestEnsembleHandler_ListSessions(t *testing.T) {
	r := setupEnsembleRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/ensemble/sessions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestEnsembleHandler_GetTeam_NotFound verifies that requesting a
// non-existent team returns 404.
func TestEnsembleHandler_GetTeam_NotFound(t *testing.T) {
	r := setupEnsembleRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/ensemble/teams/nonexistent-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
