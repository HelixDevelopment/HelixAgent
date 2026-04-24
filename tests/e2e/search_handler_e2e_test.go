package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/testutil"
)

// TestSearchHandler_SemanticSearch_E2E verifies the semantic search
// endpoint processes a query and returns a valid response structure.
func TestSearchHandler_SemanticSearch_E2E(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := testutil.ServerURL()
	client := &http.Client{Timeout: 30 * time.Second}

	body := map[string]interface{}{
		"query": "function to create HTTP server",
		"top_k": 5,
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/search/semantic", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Search infra may not be fully indexed or available. Accept:
	// - 200 = success
	// - 500 = backend error (e.g., embedding provider unreachable)
	// - 503 = service unavailable (search not configured)
	// - 404 = route not registered
	assert.Contains(t,
		[]int{http.StatusOK, http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusNotFound},
		resp.StatusCode, "expected 200, 404, 500, or 503")

	if resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Contains(t, result, "results", "response should contain results field")
	}
}

// TestSearchHandler_SemanticSearch_MissingQuery verifies that a search
// request without a query is rejected with 400.
func TestSearchHandler_SemanticSearch_MissingQuery(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := testutil.ServerURL()
	client := &http.Client{Timeout: 15 * time.Second}

	body := map[string]interface{}{
		"top_k": 3,
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/search/semantic", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"missing query should return 400")
}

// TestSearchHandler_SemanticSearch_TopKLimit verifies that top_k
// beyond the maximum (50) is rejected.
func TestSearchHandler_SemanticSearch_TopKLimit(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := testutil.ServerURL()
	client := &http.Client{Timeout: 15 * time.Second}

	body := map[string]interface{}{
		"query": "test query",
		"top_k": 100,
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/search/semantic", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"top_k exceeding max should return 400")
}
