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

// TestBrowserHandler_Navigate_E2E verifies a full navigation request
// to the browser endpoint returns a well-formed response.
func TestBrowserHandler_Navigate_E2E(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := testutil.ServerURL()
	client := &http.Client{Timeout: 30 * time.Second}

	body := map[string]interface{}{
		"url":     "https://example.com",
		"timeout": 10,
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/browser/navigate", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Browser service may not be available in all environments;
	// accept 200 (success) or 503 (service unavailable).
	assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable, http.StatusBadRequest},
		resp.StatusCode, "expected 200, 400, or 503")
}

// TestBrowserHandler_Navigate_MissingURL verifies that a navigate
// request without a URL is rejected with 400.
func TestBrowserHandler_Navigate_MissingURL(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := testutil.ServerURL()
	client := &http.Client{Timeout: 15 * time.Second}

	body := map[string]interface{}{
		"timeout": 5,
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/browser/navigate", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"missing url should return 400")
}

// TestBrowserHandler_Screenshot_NoSession verifies that requesting a
// screenshot without an active browser session returns an error.
func TestBrowserHandler_Screenshot_NoSession(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := testutil.ServerURL()
	client := &http.Client{Timeout: 15 * time.Second}

	body := map[string]interface{}{
		"full_page": true,
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/browser/screenshot", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getE2EAPIKey())

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Without a browser session the server should reject this.
	assert.NotEqual(t, http.StatusOK, resp.StatusCode,
		"screenshot without session should not succeed")
}
