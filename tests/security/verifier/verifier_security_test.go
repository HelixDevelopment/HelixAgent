package verifier

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// SecurityTestConfig holds configuration for security tests
type SecurityTestConfig struct {
	BaseURL string
	Timeout time.Duration
}

// VulnerabilityResult holds vulnerability scan results
type VulnerabilityResult struct {
	VulnerabilityType string
	Description       string
	URL               string
	StatusCode        int
	Response          string
	Severity          string
}

// checkServerAvailable checks if the test server is reachable
func checkServerAvailable(baseURL string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// TestVerifierInputValidation tests for input validation vulnerabilities
func TestVerifierInputValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping security test - server not available at " + config.BaseURL)
	}

	client := &http.Client{Timeout: config.Timeout}

	t.Run("SQLInjectionInVerify", func(t *testing.T) {
		sqlInjectionPayloads := []string{
			"' OR '1'='1",
			"'; DROP TABLE models; --",
			"' UNION SELECT * FROM users --",
			"1' OR '1'='1' --",
			"admin'--",
		}

		for _, payload := range sqlInjectionPayloads {
			requestBody := map[string]interface{}{
				"model_id": payload,
				"provider": "openai",
			}

			jsonData, _ := json.Marshal(requestBody)
			resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			bodyStr := strings.ToLower(string(body))

			// Check for SQL error messages
			sqlErrorPatterns := []string{
				"sql syntax", "mysql", "postgresql", "sqlite",
				"ora-", "syntax error", "unclosed quotation",
			}

			for _, pattern := range sqlErrorPatterns {
				if strings.Contains(bodyStr, pattern) {
					t.Errorf("Potential SQL injection vulnerability with payload '%s': found '%s' in response", payload, pattern)
				}
			}
		}
	})

	t.Run("XSSInVerify", func(t *testing.T) {
		xssPayloads := []string{
			"<script>alert('xss')</script>",
			"<img src=x onerror=alert('xss')>",
			"javascript:alert('xss')",
			"<svg onload=alert('xss')>",
			"<body onload=alert('xss')>",
		}

		for _, payload := range xssPayloads {
			requestBody := map[string]interface{}{
				"model_id": payload,
				"provider": "openai",
			}

			jsonData, _ := json.Marshal(requestBody)
			resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			// Response should not reflect unescaped XSS payload
			if strings.Contains(string(body), payload) {
				t.Errorf("Potential XSS vulnerability: payload '%s' reflected in response", payload)
			}
		}
	})

	t.Run("CommandInjectionInCodeVisibility", func(t *testing.T) {
		cmdInjectionPayloads := []string{
			"; cat /etc/passwd",
			"| ls -la",
			"$(whoami)",
			"`id`",
			"&& rm -rf /",
		}

		for _, payload := range cmdInjectionPayloads {
			requestBody := map[string]interface{}{
				"code":     payload,
				"language": "go",
				"model_id": "gpt-4",
				"provider": "openai",
			}

			jsonData, _ := json.Marshal(requestBody)
			resp, err := client.Post(config.BaseURL+"/api/v1/verifier/code-visibility", "application/json", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Check for command execution evidence
			cmdOutputPatterns := []string{
				"root:", "uid=", "gid=", "drwxr", "-rw-r--",
			}

			for _, pattern := range cmdOutputPatterns {
				if strings.Contains(bodyStr, pattern) {
					t.Errorf("Potential command injection vulnerability with payload '%s': found '%s' in response", payload, pattern)
				}
			}
		}
	})

	t.Run("PathTraversalInStatus", func(t *testing.T) {
		pathTraversalPayloads := []string{
			"../../../etc/passwd",
			"..%2F..%2F..%2Fetc%2Fpasswd",
			"....//....//....//etc/passwd",
			"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc/passwd",
		}

		for _, payload := range pathTraversalPayloads {
			resp, err := client.Get(config.BaseURL + "/api/v1/verifier/status/" + payload)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			// Check for file content leakage
			if strings.Contains(string(body), "root:") || strings.Contains(string(body), "/bin/bash") {
				t.Errorf("Potential path traversal vulnerability with payload '%s'", payload)
			}
		}
	})

	t.Run("LargeInputHandling", func(t *testing.T) {
		// Test with extremely large inputs
		largeString := strings.Repeat("A", 1000000) // 1MB

		requestBody := map[string]interface{}{
			"model_id": largeString,
			"provider": "openai",
		}

		jsonData, _ := json.Marshal(requestBody)
		resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))

		if err == nil {
			resp.Body.Close()
			// Server should handle large inputs gracefully
			if resp.StatusCode == http.StatusOK {
				t.Log("Warning: Server accepted very large input without error")
			}
		}
	})

	t.Run("MalformedJSONHandling", func(t *testing.T) {
		malformedPayloads := []string{
			`{"model_id": "test"`, // Unclosed brace
			`{model_id: "test"}`,  // Missing quotes
			`{"model_id": }`,      // Missing value
			`null`,
			`[]`,
		}

		for _, payload := range malformedPayloads {
			resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer([]byte(payload)))
			require.NoError(t, err)
			defer resp.Body.Close()

			// Server should return 400 for malformed JSON, not 500
			if resp.StatusCode == http.StatusInternalServerError {
				t.Errorf("Server returned 500 for malformed JSON '%s', expected 400", payload)
			}
		}
	})
}

// TestVerifierAuthentication tests authentication security
func TestVerifierAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping security test - server not available at " + config.BaseURL)
	}

	client := &http.Client{Timeout: config.Timeout}

	t.Run("MissingAuthentication", func(t *testing.T) {
		// Protected endpoints should require authentication
		protectedEndpoints := []string{
			"/api/v1/verifier/verify",
			"/api/v1/verifier/batch-verify",
			"/api/v1/verifier/code-visibility",
		}

		for _, endpoint := range protectedEndpoints {
			req, _ := http.NewRequest("POST", config.BaseURL+endpoint, bytes.NewBuffer([]byte(`{}`)))
			// Explicitly no Authorization header

			resp, err := client.Do(req)
			require.NoError(t, err)
			resp.Body.Close()

			// Log the behavior (may or may not require auth depending on config)
			t.Logf("Endpoint %s without auth returned status %d", endpoint, resp.StatusCode)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		invalidTokens := []string{
			"invalid-token",
			"Bearer invalid",
			"Bearer " + strings.Repeat("x", 1000),
			"",
			"null",
		}

		for _, token := range invalidTokens {
			req, _ := http.NewRequest("POST", config.BaseURL+"/api/v1/verifier/verify", bytes.NewBuffer([]byte(`{"model_id":"test","provider":"openai"}`)))
			req.Header.Set("Authorization", token)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			require.NoError(t, err)
			resp.Body.Close()

			t.Logf("Invalid token '%s' returned status %d", token[:min(20, len(token))], resp.StatusCode)
		}
	})

	t.Run("TokenInURL", func(t *testing.T) {
		// Tokens should not be accepted in URL parameters
		resp, err := client.Get(config.BaseURL + "/api/v1/verifier/health?token=secret-token")
		require.NoError(t, err)
		resp.Body.Close()

		// Should not process token from URL
		t.Logf("Token in URL returned status %d", resp.StatusCode)
	})
}

// TestVerifierRateLimiting tests rate limiting behavior
func TestVerifierRateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping security test - server not available at " + config.BaseURL)
	}

	client := &http.Client{Timeout: config.Timeout}

	t.Run("RapidRequests", func(t *testing.T) {
		rateLimitHit := false
		requestBody := []byte(`{"model_id":"test","provider":"openai"}`)

		for i := 0; i < 100; i++ {
			resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(requestBody))
			if err != nil {
				continue
			}

			if resp.StatusCode == http.StatusTooManyRequests {
				rateLimitHit = true
				resp.Body.Close()
				t.Logf("Rate limit hit after %d requests", i+1)
				break
			}
			resp.Body.Close()
		}

		if !rateLimitHit {
			t.Log("Warning: Rate limiting may not be configured or threshold not reached")
		}
	})
}

// TestVerifierSecurityHeaders tests security headers
func TestVerifierSecurityHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping security test - server not available at " + config.BaseURL)
	}

	client := &http.Client{Timeout: config.Timeout}

	t.Run("SecurityHeaders", func(t *testing.T) {
		resp, err := client.Get(config.BaseURL + "/api/v1/verifier/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		expectedHeaders := map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"X-XSS-Protection":       "1; mode=block",
		}

		for header, expectedValue := range expectedHeaders {
			actualValue := resp.Header.Get(header)
			if actualValue == "" {
				t.Logf("Warning: Missing security header %s", header)
			} else if actualValue != expectedValue {
				t.Logf("Security header %s has value '%s', expected '%s'", header, actualValue, expectedValue)
			}
		}
	})

	t.Run("CORSHeaders", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", config.BaseURL+"/api/v1/verifier/verify", nil)
		req.Header.Set("Origin", "http://malicious-site.com")
		req.Header.Set("Access-Control-Request-Method", "POST")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
		if allowOrigin == "*" {
			t.Log("Warning: CORS allows all origins (Access-Control-Allow-Origin: *)")
		} else if allowOrigin == "http://malicious-site.com" {
			t.Error("CORS allows requests from malicious origin")
		}
	})

	t.Run("ContentTypeEnforcement", func(t *testing.T) {
		// Send request with wrong content type
		req, _ := http.NewRequest("POST", config.BaseURL+"/api/v1/verifier/verify", bytes.NewBuffer([]byte(`{"model_id":"test"}`)))
		req.Header.Set("Content-Type", "text/plain")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Server should ideally reject non-JSON content types for JSON endpoints
		if resp.StatusCode == http.StatusOK {
			t.Log("Warning: Server accepted non-JSON content type for JSON endpoint")
		}
	})
}

// TestVerifierDataLeakage tests for data leakage vulnerabilities
func TestVerifierDataLeakage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping security test - server not available at " + config.BaseURL)
	}

	client := &http.Client{Timeout: config.Timeout}

	t.Run("ErrorMessageLeakage", func(t *testing.T) {
		// Force errors and check for sensitive information leakage
		requestBody := map[string]interface{}{
			"model_id": nil, // Invalid value
			"provider": "invalid-provider",
		}

		jsonData, _ := json.Marshal(requestBody)
		resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := strings.ToLower(string(body))

		// Check for sensitive information in error messages
		sensitivePatterns := []string{
			"password", "secret", "api_key", "api-key",
			"stack trace", "panic", "goroutine",
			"/home/", "/var/", "/etc/",
		}

		for _, pattern := range sensitivePatterns {
			if strings.Contains(bodyStr, pattern) {
				t.Errorf("Potential data leakage: found '%s' in error response", pattern)
			}
		}
	})

	t.Run("DebugModeLeakage", func(t *testing.T) {
		resp, err := client.Get(config.BaseURL + "/debug/pprof/")
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Log("Warning: Debug endpoints are exposed")
			}
		}

		resp, err = client.Get(config.BaseURL + "/debug/vars")
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Log("Warning: Debug vars endpoint is exposed")
			}
		}
	})

	t.Run("ServerVersionLeakage", func(t *testing.T) {
		resp, err := client.Get(config.BaseURL + "/api/v1/verifier/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		serverHeader := resp.Header.Get("Server")
		if serverHeader != "" {
			t.Logf("Server header exposed: %s", serverHeader)
		}

		xPoweredBy := resp.Header.Get("X-Powered-By")
		if xPoweredBy != "" {
			t.Logf("Warning: X-Powered-By header exposed: %s", xPoweredBy)
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
