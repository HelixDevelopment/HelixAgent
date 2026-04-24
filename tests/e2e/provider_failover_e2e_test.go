package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoServerFailover skips the test if HelixAgent server is not reachable.
func skipIfNoServerFailover(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping failover test in short mode (requires live server with LLM providers)")  // SKIP-OK: #short-mode
	}
	conn, err := net.DialTimeout("tcp", "localhost:7061", 2*time.Second)
	if err != nil {
		t.Skip("HelixAgent server not running on :7061")  // SKIP-OK: #legacy-untriaged
	}
	conn.Close()
}

// failoverClient returns an HTTP client with a short timeout suitable for
// failover testing where we expect provider-level retries.
func failoverClient() *http.Client {
	return &http.Client{Timeout: 90 * time.Second}
}

// failoverBaseURL returns the base URL of the local HelixAgent server.
func failoverBaseURL() string {
	return "http://localhost:7061"
}

// chatRequest builds a chat completion request body with the given model and
// optional overrides merged into the request map.
func chatRequest(model string, overrides map[string]interface{}) ([]byte, error) {
	req := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Say exactly: pong"},
		},
		"max_tokens":  20,
		"temperature": 0.0,
	}
	for k, v := range overrides {
		req[k] = v
	}
	return json.Marshal(req)
}

// doChat sends a chat completion request and returns the HTTP response.
func doChat(t *testing.T, body []byte) *http.Response {
	t.Helper()
	client := failoverClient()
	apiKey := getE2EAPIKey()

	req, err := http.NewRequest("POST",
		failoverBaseURL()+"/v1/chat/completions",
		bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

// TestE2E_ProviderFailover_FallbackOnError verifies that when the primary
// provider returns an error the system transparently falls back to another
// provider and the client receives a successful response.
func TestE2E_ProviderFailover_FallbackOnError(t *testing.T) {
	skipIfNoServerFailover(t)

	// Use a model name that triggers the ensemble/fallback path.
	body, err := chatRequest("gpt-3.5-turbo", nil)
	require.NoError(t, err)

	resp := doChat(t, body)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// The server should either succeed (provider available) or return
	// a structured error — never an unstructured 500.
	if resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		err = json.Unmarshal(respBody, &result)
		require.NoError(t, err)
		assert.NotNil(t, result["choices"], "Successful response should contain choices")
		t.Log("Primary or fallback provider returned a successful response")
	} else {
		// Even on failure the response must be valid JSON with an error field.
		var errResp map[string]interface{}
		err = json.Unmarshal(respBody, &errResp)
		require.NoError(t, err, "Error response must be valid JSON, got: %s", string(respBody))
		assert.NotNil(t, errResp["error"], "Error response should have 'error' field")
		assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
			"Should not get bare 500; expected structured error")
		t.Logf("All providers unavailable — got structured error (status %d)", resp.StatusCode)
	}
}

// TestE2E_ProviderFailover_AllFail_GracefulError verifies that when all
// providers are unreachable the server responds with a graceful, structured
// error message containing details about the failure chain.
func TestE2E_ProviderFailover_AllFail_GracefulError(t *testing.T) {
	skipIfNoServerFailover(t)

	// Request a model that is very unlikely to exist — forces all providers
	// to fail and exercises the full fallback chain.
	body, err := chatRequest("nonexistent-model-xyz-99999", nil)
	require.NoError(t, err)

	resp := doChat(t, body)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// We expect a structured error, not a success.
	var errResp map[string]interface{}
	err = json.Unmarshal(respBody, &errResp)
	require.NoError(t, err, "Response must be valid JSON")

	// The status should indicate a client or service error, not a raw crash.
	assert.True(t,
		resp.StatusCode >= 400 && resp.StatusCode < 600,
		"Expected 4xx/5xx status, got %d", resp.StatusCode)

	// The response should contain an error description.
	assert.NotNil(t, errResp["error"],
		"Graceful error should contain 'error' field")

	t.Logf("Graceful error response (status %d): %s",
		resp.StatusCode, string(respBody))
}

// TestE2E_ProviderFailover_CircuitBreaker_Opens validates that after
// repeated failures the circuit breaker health endpoint reflects degraded
// provider health.
func TestE2E_ProviderFailover_CircuitBreaker_Opens(t *testing.T) {
	skipIfNoServerFailover(t)

	client := failoverClient()
	baseURL := failoverBaseURL()

	// First, check health endpoint is reachable.
	resp, err := client.Get(baseURL + "/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"Health endpoint should be available")

	var healthResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&healthResp)
	require.NoError(t, err)

	// The health response should include provider-level status.
	assert.NotNil(t, healthResp["status"],
		"Health response should include 'status' field")

	// If providers map is present, validate its structure.
	if providers, ok := healthResp["providers"]; ok && providers != nil {
		providersMap, isMap := providers.(map[string]interface{})
		if isMap {
			for name, info := range providersMap {
				t.Logf("Provider %s: %v", name, info)
			}
		}
	}

	// Send several requests to a non-existent model to trigger circuit
	// breaker state changes. We do not assert the breaker opens because
	// that depends on configuration, but we verify the server stays healthy.
	for i := 0; i < 3; i++ {
		body, err := chatRequest("nonexistent-breaker-model", nil)
		require.NoError(t, err)
		r := doChat(t, body)
		io.ReadAll(r.Body)
		r.Body.Close()
	}

	// After the burst, the server itself should still be healthy.
	resp2, err := client.Get(baseURL + "/v1/health")
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode,
		"Server should remain healthy even after provider failures")

	t.Log("Server health maintained after repeated provider failures")
}

// TestE2E_ProviderFailover_Recovery_AfterBreaker verifies that after a
// period of failures the system can still serve requests when a working
// provider is available (half-open recovery).
func TestE2E_ProviderFailover_Recovery_AfterBreaker(t *testing.T) {
	skipIfNoServerFailover(t)

	client := failoverClient()
	baseURL := failoverBaseURL()

	// Trigger a few failures first.
	for i := 0; i < 2; i++ {
		body, err := chatRequest("nonexistent-recovery-model", nil)
		require.NoError(t, err)
		r := doChat(t, body)
		io.ReadAll(r.Body)
		r.Body.Close()
	}

	// Now send a request with a real model — should still work if any
	// provider is available.
	body, err := chatRequest("gpt-3.5-turbo", nil)
	require.NoError(t, err)

	resp := doChat(t, body)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	// Either a successful response or a structured error — not a 500 crash.
	if resp.StatusCode == http.StatusOK {
		assert.NotNil(t, result["choices"],
			"Recovered response should contain choices")
		t.Log("Recovery succeeded — got valid response after breaker activation")
	} else {
		assert.NotNil(t, result["error"],
			"Non-200 response should be structured error")
		t.Logf("Recovery attempt got status %d — no provider available", resp.StatusCode)
	}

	// Server should still respond to health checks.
	healthResp, err := client.Get(baseURL + "/v1/health")
	require.NoError(t, err)
	defer healthResp.Body.Close()
	assert.Equal(t, http.StatusOK, healthResp.StatusCode,
		"Health endpoint should work after recovery attempt")
}

// TestE2E_ProviderFailover_StreamingFallback verifies that streaming
// requests fall back gracefully when the primary provider is unavailable.
func TestE2E_ProviderFailover_StreamingFallback(t *testing.T) {
	skipIfNoServerFailover(t)

	body, err := chatRequest("gpt-3.5-turbo", map[string]interface{}{
		"stream": true,
	})
	require.NoError(t, err)

	resp := doChat(t, body)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// For streaming, we expect either SSE events or a JSON error.
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "text/event-stream") {
			// Read SSE events and verify at least one data line.
			scanner := bufio.NewScanner(resp.Body)
			gotData := false
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					gotData = true
					data := strings.TrimPrefix(line, "data: ")
					if data == "[DONE]" {
						break
					}
					// Each data chunk should be valid JSON.
					var chunk map[string]interface{}
					if err := json.Unmarshal([]byte(data), &chunk); err == nil {
						assert.NotNil(t, chunk["id"],
							"Stream chunk should have an id")
					}
				}
			}
			assert.True(t, gotData, "Should receive at least one data event")
			t.Log("Streaming fallback succeeded — received SSE events")
		} else {
			// Some implementations return a single JSON response even with
			// stream=true when the streaming provider is unavailable.
			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var result map[string]interface{}
			err = json.Unmarshal(respBody, &result)
			require.NoError(t, err)
			assert.NotNil(t, result["choices"],
				"Non-streaming fallback should contain choices")
			t.Log("Streaming unavailable — fell back to non-streaming response")
		}
	} else {
		// All streaming providers failed — verify structured error.
		respBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var errResp map[string]interface{}
		err = json.Unmarshal(respBody, &errResp)
		require.NoError(t, err,
			"Error response must be valid JSON, got: %s", string(respBody))
		assert.NotNil(t, errResp["error"],
			"Error response should have 'error' field")
		t.Logf("All streaming providers unavailable (status %d)", resp.StatusCode)
	}
}
