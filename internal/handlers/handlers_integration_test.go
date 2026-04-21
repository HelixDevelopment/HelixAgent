package handlers

// CONST-030 compliance: this file used to wire an in-process gin router with
// a MockLLMProvider and httptest.NewRecorder to exercise handler code. That
// pattern violates CONST-030 ("mocks ONLY in unit tests"). The file has been
// rewritten to drive the LIVE HelixAgent HTTP surface on port 7061 via real
// net/http round-trips. Each test guards with a TCP dial pre-check and skips
// (per CONST-030's skip-not-fail clause) when HelixAgent is unreachable —
// `./bin/helixagent` is the supported way to bring it up.
//
// Pattern 1 from docs/development/CONST-030_COMPLIANCE_AUDIT_2026-04-21.md.

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	helixAgentHost = "localhost:7061"
	helixAgentBase = "http://localhost:7061"
)

// isHelixAgentAvailable returns true iff a TCP handshake to :7061 succeeds.
// Tests that need the live service MUST t.Skip when this returns false —
// never fail — per CONST-030's skip-not-fail clause.
func isHelixAgentAvailable(t *testing.T) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", helixAgentHost, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// liveClient returns an http.Client with a short timeout suitable for CI.
// Request bodies are small; responses from handler-layer endpoints are
// bounded by the 10 MiB middleware cap (see CLAUDE.md).
func liveClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// doJSON is a small helper: POST/GET with a JSON body, parse the response
// body into out (may be nil), return status code. Any transport error is
// surfaced; assertion on HTTP status is the caller's responsibility.
func doJSON(t *testing.T, method, path string, reqBody any, out any) (int, http.Header) {
	t.Helper()
	var body io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		require.NoError(t, err)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, helixAgentBase+path, body)
	require.NoError(t, err)
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := liveClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	if out != nil && len(raw) > 0 {
		// Non-JSON or empty bodies are legitimate for several endpoints
		// (e.g. SSE streams, 204s); don't require-fail the test.
		_ = json.Unmarshal(raw, out)
	}
	return resp.StatusCode, resp.Header
}

// TestIntegration_HealthEndpoint probes the live health endpoint.
func TestIntegration_HealthEndpoint(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	status, _ := doJSON(t, http.MethodGet, "/v1/health", nil, nil)
	assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status,
		"expected /v1/health to return 200 or 204, got %d", status)
}

// TestIntegration_CompleteFlow drives the live /v1/chat/completions endpoint.
// We use chat completions (OpenAI-compatible) because that is the contract
// CLI agents and tests exercise; plain /v1/completions remains available but
// requires a prompt and provider wiring that the default live instance may
// deprioritise.
func TestIntegration_CompleteFlow(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	reqBody := map[string]any{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"max_tokens": 8,
	}
	status, hdr := doJSON(t, http.MethodPost, "/v1/chat/completions", reqBody, nil)
	// Live HelixAgent may return 200 (success), 429 (upstream rate-limited),
	// 502/503 (upstream unavailable) depending on provider state. We assert
	// only that the handler accepted the request shape — i.e. NOT a 4xx
	// schema rejection from Gin.
	assert.NotEqual(t, http.StatusBadRequest, status,
		"/v1/chat/completions rejected a well-formed request")
	assert.NotEqual(t, http.StatusNotFound, status,
		"/v1/chat/completions route missing on live server")
	if status == http.StatusOK {
		assert.Contains(t, hdr.Get("Content-Type"), "application/json")
	}
}

// TestIntegration_ChatFlow is a second shape of the chat endpoint (multi-turn).
func TestIntegration_ChatFlow(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	reqBody := map[string]any{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "system", "content": "You are concise."},
			{"role": "user", "content": "Reply with OK."},
		},
		"max_tokens":  4,
		"temperature": 0.0,
	}
	status, _ := doJSON(t, http.MethodPost, "/v1/chat/completions", reqBody, nil)
	assert.NotEqual(t, http.StatusBadRequest, status)
	assert.NotEqual(t, http.StatusNotFound, status)
}

// TestIntegration_ModelsEndpoint asserts the live /v1/models contract.
func TestIntegration_ModelsEndpoint(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	var payload map[string]any
	status, _ := doJSON(t, http.MethodGet, "/v1/models", nil, &payload)
	require.Equal(t, http.StatusOK, status, "/v1/models should be 200 on a healthy instance")
	assert.Equal(t, "list", payload["object"])
	assert.Contains(t, payload, "data")
}

// TestIntegration_DebateCreateAndRetrieve tests live debate create/get cycle.
func TestIntegration_DebateCreateAndRetrieve(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	createBody := map[string]any{
		"topic": "Should AI be regulated?",
		"participants": []map[string]string{
			{"name": "Advocate", "role": "proposer"},
			{"name": "Skeptic", "role": "critic"},
		},
		"max_rounds": 3,
	}
	var createResp map[string]any
	status, _ := doJSON(t, http.MethodPost, "/v1/debates", createBody, &createResp)
	if status == http.StatusServiceUnavailable || status == http.StatusNotImplemented {
		t.Skipf("/v1/debates not available on this instance (status=%d)", status)
	}
	require.Equal(t, http.StatusAccepted, status,
		"/v1/debates POST expected 202, got %d body=%v", status, createResp)
	require.Contains(t, createResp, "debate_id")
	debateID, _ := createResp["debate_id"].(string)
	require.NotEmpty(t, debateID)

	var getResp map[string]any
	status, _ = doJSON(t, http.MethodGet, "/v1/debates/"+debateID, nil, &getResp)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, debateID, getResp["debate_id"])
}

// TestIntegration_DebateStatusFlow tests live debate status endpoint.
func TestIntegration_DebateStatusFlow(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	createBody := map[string]any{
		"topic": "Test debate",
		"participants": []map[string]string{
			{"name": "A"},
			{"name": "B"},
		},
	}
	var createResp map[string]any
	status, _ := doJSON(t, http.MethodPost, "/v1/debates", createBody, &createResp)
	if status == http.StatusServiceUnavailable || status == http.StatusNotImplemented {
		t.Skipf("/v1/debates not available on this instance (status=%d)", status)
	}
	require.Equal(t, http.StatusAccepted, status)
	debateID, _ := createResp["debate_id"].(string)
	require.NotEmpty(t, debateID)

	var statusResp map[string]any
	status, _ = doJSON(t, http.MethodGet, "/v1/debates/"+debateID+"/status", nil, &statusResp)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, debateID, statusResp["debate_id"])
	assert.Contains(t, statusResp, "status")
}

// TestIntegration_DebateListAndDelete tests live debate list endpoint.
func TestIntegration_DebateListAndDelete(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	// Create three debates.
	for i := 0; i < 3; i++ {
		createBody := map[string]any{
			"topic": "Test debate " + string(rune('A'+i)),
			"participants": []map[string]string{
				{"name": "Participant 1"},
				{"name": "Participant 2"},
			},
		}
		status, _ := doJSON(t, http.MethodPost, "/v1/debates", createBody, nil)
		if status == http.StatusServiceUnavailable || status == http.StatusNotImplemented {
			t.Skipf("/v1/debates not available on this instance (status=%d)", status)
		}
		require.Equal(t, http.StatusAccepted, status)
	}

	var listResp map[string]any
	status, _ := doJSON(t, http.MethodGet, "/v1/debates", nil, &listResp)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, listResp, "debates")
}

// TestIntegration_MCPFlow probes the live MCP HTTP surface.
func TestIntegration_MCPFlow(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	for _, path := range []string{
		"/v1/mcp/capabilities",
		"/v1/mcp/tools",
		"/v1/mcp/prompts",
		"/v1/mcp/resources",
	} {
		status, _ := doJSON(t, http.MethodGet, path, nil, nil)
		// MCP surface may be feature-gated; accept 200 or 404 (disabled),
		// but never a 5xx (which would mean a broken route).
		assert.Less(t, status, http.StatusInternalServerError,
			"%s returned 5xx %d (expected 2xx/4xx)", path, status)
	}
}

// TestIntegration_InvalidRoutes drives a handful of bad requests and asserts
// that the live server rejects them cleanly (4xx, never 200).
func TestIntegration_InvalidRoutes(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/v1/debates/non-existent", ""},
		{http.MethodGet, "/v1/debates/non-existent/status", ""},
		{http.MethodPost, "/v1/completions", "invalid json"},
		{http.MethodPost, "/v1/chat/completions", "invalid json"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.method+"_"+c.path, func(t *testing.T) {
			t.Parallel()
			var body io.Reader
			if c.body != "" {
				body = strings.NewReader(c.body)
			}
			req, err := http.NewRequest(c.method, helixAgentBase+c.path, body)
			require.NoError(t, err)
			if c.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := liveClient().Do(req)
			require.NoError(t, err)
			_ = resp.Body.Close()
			assert.NotEqual(t, http.StatusOK, resp.StatusCode,
				"%s %s should not succeed with invalid input", c.method, c.path)
		})
	}
}

// TestIntegration_MiddlewareNotFound verifies the live server's 404 posture.
func TestIntegration_MiddlewareNotFound(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	status, _ := doJSON(t, http.MethodGet, "/non-existent-route-const030", nil, nil)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestIntegration_ConcurrentRequests fans 10 parallel models requests and
// requires that the live server handles them without 5xx-ing. This replaces
// the previous in-process goroutine soak against a mock.
func TestIntegration_ConcurrentRequests(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	const n = 10
	var wg sync.WaitGroup
	results := make(chan int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := doJSON(t, http.MethodGet, "/v1/models", nil, nil)
			results <- status
		}()
	}
	wg.Wait()
	close(results)

	ok := 0
	for s := range results {
		if s == http.StatusOK {
			ok++
		}
		assert.Less(t, s, http.StatusInternalServerError,
			"/v1/models returned 5xx %d under parallel load", s)
	}
	assert.Equal(t, n, ok, "expected all %d /v1/models requests to succeed, got %d", n, ok)
}

// TestIntegration_ResponseHeaders asserts JSON content-type from live routes.
func TestIntegration_ResponseHeaders(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	_, hdr := doJSON(t, http.MethodGet, "/v1/models", nil, nil)
	assert.Contains(t, hdr.Get("Content-Type"), "application/json")
}

// TestIntegration_ErrorResponseFormat asserts error-body shape on live server.
func TestIntegration_ErrorResponseFormat(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	var resp map[string]any
	status, _ := doJSON(t, http.MethodPost, "/v1/completions",
		map[string]any{"invalid": "request"}, &resp)
	assert.GreaterOrEqual(t, status, 400)
	assert.Less(t, status, 500)
	assert.Contains(t, resp, "error")
}

// TestIntegration_URLRouting sanity-checks the standard live routes resolve.
func TestIntegration_URLRouting(t *testing.T) {
	t.Parallel()
	if !isHelixAgentAvailable(t) {
		t.Skip("HelixAgent unreachable on :7061 — start with `./bin/helixagent` (CONST-030)")
	}

	t.Run("GET_/v1/models", func(t *testing.T) {
		t.Parallel()
		status, _ := doJSON(t, http.MethodGet, "/v1/models", nil, nil)
		assert.Equal(t, http.StatusOK, status)
	})
	t.Run("GET_/v1/health", func(t *testing.T) {
		t.Parallel()
		status, _ := doJSON(t, http.MethodGet, "/v1/health", nil, nil)
		assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status)
	})
}
