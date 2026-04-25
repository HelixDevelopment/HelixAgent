//go:build integration
// +build integration

// Package integration provides ensemble handler integration tests.
//
// These tests exercise the LIVE /v1/ensemble/* endpoints on a running
// HelixAgent binary (./bin/helixagent), per CONST-030 — non-unit tests
// must hit the real running system, not an in-process router. They skip
// gracefully via testutil.RequireServer when the binary is not reachable.
//
// Converted from the in-process httptest.NewRecorder pattern on 2026-04-25
// as the proof-of-concept for the no-mocks-above-unit drainage workflow
// (see scripts/no-mocks-above-unit-allowlist.txt and docs/issues/MOCK_CATEGORIES.md).
// The previous version asserted handler behaviour against a fake gin.Engine
// — passing tests proved the handler returned the right struct, not that
// the running binary served the documented endpoints.
package integration

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

	"dev.helix.agent/internal/testutil"
)

// ensembleBaseURL returns the /v1 base URL for the running HelixAgent
// binary, derived from testutil.DefaultInfraConfig (HELIXAGENT_HOST/PORT
// env vars, defaulting to localhost:8100 per the CONST-027 port registry).
func ensembleBaseURL(t *testing.T) string {
	t.Helper()
	cfg := testutil.DefaultInfraConfig()
	return fmt.Sprintf("http://%s:%s/v1", cfg.ServerHost, cfg.ServerPort)
}

// ensembleHTTPClient returns a short-timeout client for the test
// roundtrips. The /v1/ensemble routes are in the auth-skip list
// (internal/router/router.go:387) so no Authorization header is needed.
func ensembleHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// TestEnsembleHandler_CreateTeam verifies that POST /v1/ensemble/teams on
// the real running binary creates a team and returns 201 with a valid id.
func TestEnsembleHandler_CreateTeam(t *testing.T) {
	testutil.RequireServer(t)

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
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, ensembleBaseURL(t)+"/ensemble/teams", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ensembleHTTPClient().Do(req)
	require.NoError(t, err, "POST /v1/ensemble/teams should reach the binary")
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode,
		"should return 201 Created; body=%s", string(bodyBytes))

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(bodyBytes, &result))
	assert.NotEmpty(t, result["id"], "response should contain a team id")
}

// TestEnsembleHandler_ListTeams verifies that GET /v1/ensemble/teams on
// the real binary returns 200 with a JSON array body.
func TestEnsembleHandler_ListTeams(t *testing.T) {
	testutil.RequireServer(t)

	req, err := http.NewRequest(http.MethodGet, ensembleBaseURL(t)+"/ensemble/teams", nil)
	require.NoError(t, err)

	resp, err := ensembleHTTPClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result),
		"response should be a JSON array")
}

// TestEnsembleHandler_ListSessions verifies that GET /v1/ensemble/sessions
// on the real binary returns 200.
func TestEnsembleHandler_ListSessions(t *testing.T) {
	testutil.RequireServer(t)

	req, err := http.NewRequest(http.MethodGet, ensembleBaseURL(t)+"/ensemble/sessions", nil)
	require.NoError(t, err)

	resp, err := ensembleHTTPClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestEnsembleHandler_GetTeam_NotFound verifies that GET on a non-existent
// team id against the real binary returns 404.
func TestEnsembleHandler_GetTeam_NotFound(t *testing.T) {
	testutil.RequireServer(t)

	req, err := http.NewRequest(http.MethodGet, ensembleBaseURL(t)+"/ensemble/teams/nonexistent-id", nil)
	require.NoError(t, err)

	resp, err := ensembleHTTPClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
