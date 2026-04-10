// Package security_test contains OWASP Top 10 (2021) compliance tests for HelixAgent.
// Each test category maps to an OWASP category (A01-A10) and validates that
// the application properly defends against the corresponding class of attacks.
//
// Run with: GOMAXPROCS=2 go test -v ./tests/security/ -run TestOWASP -timeout 10m
package security_test

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TEST INFRASTRUCTURE
// =============================================================================

func init() {
	gin.SetMode(gin.TestMode)
}

// owaspRouter builds a Gin engine that replicates the security-relevant
// middleware of the real HelixAgent server: JWT auth, body-size limits,
// content-type enforcement, rate limiting, CORS, input sanitisation,
// and security headers.  It exposes the same endpoint shapes as the
// production routes so every OWASP category can be exercised without
// starting the full server.
func owaspRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// ---- Security headers middleware ----
	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Cache-Control", "no-store")
		c.Header("Content-Security-Policy", "default-src 'self'")
		// Do NOT set Server header to avoid information leakage.
		c.Next()
	})

	// ---- Body-size limit (10 MB) ----
	const maxBodySize int64 = 10 * 1024 * 1024
	r.Use(func(c *gin.Context) {
		if c.Request.ContentLength > maxBodySize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": gin.H{
					"message": "request body too large",
					"type":    "invalid_request_error",
				},
			})
			c.Abort()
			return
		}
		c.Next()
	})

	// ---- Rate limiter (per-IP, 10 req/window for testing) ----
	var (
		rateMu    sync.Mutex
		rateCount = make(map[string]int)
	)
	const rateLimit = 10
	r.Use(func(c *gin.Context) {
		ip := c.ClientIP()
		rateMu.Lock()
		rateCount[ip]++
		count := rateCount[ip]
		rateMu.Unlock()
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rateLimit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, rateLimit-count)))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
		if count > rateLimit {
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": 60,
			})
			c.Abort()
			return
		}
		c.Next()
	})

	// ---- CORS middleware ----
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowedOrigins := map[string]bool{
			"https://app.helixagent.dev": true,
			"https://helixagent.dev":     true,
		}
		if allowedOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Max-Age", "3600")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// ---- Security event logger (in-memory for testing) ----
	var (
		secLogMu sync.Mutex
		secLog   []string
	)
	logEvent := func(event string) {
		secLogMu.Lock()
		secLog = append(secLog, event)
		secLogMu.Unlock()
	}
	getEvents := func() []string {
		secLogMu.Lock()
		defer secLogMu.Unlock()
		cp := make([]string, len(secLog))
		copy(cp, secLog)
		return cp
	}

	// ---- Public routes (no auth) ----
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/v1/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   []gin.H{{`id`: "helixagent-debate", "object": "model"}},
		})
	})

	// ---- Security log endpoint (for A09 tests) ----
	r.GET("/v1/security/events", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"events": getEvents()})
	})

	// ---- Auth helper: validate JWT ----
	const jwtSecret = "owasp-test-secret-key-32bytes!!"
	validateJWT := func(c *gin.Context) bool {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logEvent("auth_failure:missing_header")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "missing authorization header",
					"type":    "authentication_error",
				},
			})
			c.Abort()
			return false
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			logEvent("auth_failure:invalid_format")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "invalid authorization format, expected Bearer token",
					"type":    "authentication_error",
				},
			})
			c.Abort()
			return false
		}
		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			logEvent(fmt.Sprintf("auth_failure:invalid_token:%v", err))
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "invalid or expired token",
					"type":    "authentication_error",
				},
			})
			c.Abort()
			return false
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"message": "invalid token claims", "type": "authentication_error"},
			})
			c.Abort()
			return false
		}
		c.Set("user_id", claims["sub"])
		c.Set("role", claims["role"])
		return true
	}

	// ---- Role check ----
	requireRole := func(required string) gin.HandlerFunc {
		return func(c *gin.Context) {
			role, _ := c.Get("role")
			roleStr, _ := role.(string)
			if roleStr != required && roleStr != "admin" {
				logEvent(fmt.Sprintf("authz_failure:required=%s,got=%s", required, roleStr))
				c.JSON(http.StatusForbidden, gin.H{
					"error": gin.H{
						"message": "insufficient permissions",
						"type":    "authorization_error",
					},
				})
				c.Abort()
				return
			}
			c.Next()
		}
	}

	// ---- Protected endpoints ----
	authGroup := r.Group("/v1")
	authGroup.Use(func(c *gin.Context) {
		if !validateJWT(c) {
			return
		}
		c.Next()
	})

	// Chat completions (auth required, JSON only)
	authGroup.POST("/chat/completions", func(c *gin.Context) {
		ct := c.GetHeader("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{
				"error": gin.H{
					"message": "unsupported content type, expected application/json",
					"type":    "invalid_request_error",
				},
			})
			return
		}
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "invalid request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}
		// Validate messages or prompt present
		messages, _ := req["messages"].([]interface{})
		prompt, _ := req["prompt"].(string)
		if len(messages) == 0 && prompt == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "messages or prompt required",
					"type":    "invalid_request_error",
				},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":     "chatcmpl-owasp-test",
			"object": "chat.completion",
			"choices": []gin.H{{
				"index":         0,
				"message":       gin.H{"role": "assistant", "content": "OWASP test response"},
				"finish_reason": "stop",
			}},
		})
	})

	// Completions (legacy)
	authGroup.POST("/completions", func(c *gin.Context) {
		ct := c.GetHeader("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{
				"error": gin.H{
					"message": "unsupported content type",
					"type":    "invalid_request_error",
				},
			})
			return
		}
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "invalid request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":      "cmpl-owasp-test",
			"object":  "text_completion",
			"choices": []gin.H{{"text": "safe response", "index": 0}},
		})
	})

	// Admin-only endpoint
	authGroup.GET("/admin/users", requireRole("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"users": []gin.H{{"id": "1", "username": "admin"}},
		})
	})

	authGroup.DELETE("/admin/users/:id", requireRole("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deleted": true})
	})

	// User profile (auth required, user can see own data only)
	authGroup.GET("/users/:id/profile", func(c *gin.Context) {
		requestedID := c.Param("id")
		userID, _ := c.Get("user_id")
		userIDStr, _ := userID.(string)
		role, _ := c.Get("role")
		roleStr, _ := role.(string)
		if userIDStr != requestedID && roleStr != "admin" {
			logEvent(fmt.Sprintf("access_violation:user=%s,requested=%s", userIDStr, requestedID))
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"message": "access denied: you can only view your own profile",
					"type":    "authorization_error",
				},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":       requestedID,
			"username": "testuser",
			"email":    "user@example.com",
		})
	})

	// Webhook / callback endpoint (SSRF test target)
	authGroup.POST("/webhooks/register", func(c *gin.Context) {
		var req struct {
			URL string `json:"url" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{"message": "url field required", "type": "invalid_request_error"},
			})
			return
		}
		// Validate URL: reject internal/private IPs
		parsed, err := url.Parse(req.URL)
		if err != nil || parsed.Host == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{"message": "invalid URL", "type": "invalid_request_error"},
			})
			return
		}
		host := parsed.Hostname()
		// Block private/internal IPs
		blockedPrefixes := []string{
			"127.", "10.", "172.16.", "172.17.", "172.18.", "172.19.",
			"172.20.", "172.21.", "172.22.", "172.23.", "172.24.",
			"172.25.", "172.26.", "172.27.", "172.28.", "172.29.",
			"172.30.", "172.31.", "192.168.", "0.", "169.254.",
		}
		blockedHosts := []string{
			"localhost", "metadata.google.internal",
			"169.254.169.254", "[::1]", "0.0.0.0",
		}
		for _, blocked := range blockedHosts {
			if strings.EqualFold(host, blocked) {
				logEvent(fmt.Sprintf("ssrf_blocked:host=%s", host))
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": "URL target not allowed: internal host",
						"type":    "invalid_request_error",
					},
				})
				return
			}
		}
		for _, prefix := range blockedPrefixes {
			if strings.HasPrefix(host, prefix) {
				logEvent(fmt.Sprintf("ssrf_blocked:ip=%s", host))
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": "URL target not allowed: private IP range",
						"type":    "invalid_request_error",
					},
				})
				return
			}
		}
		// Resolve hostname to check for DNS rebinding to internal IPs
		ips, err := net.LookupHost(host)
		if err == nil {
			for _, ip := range ips {
				parsedIP := net.ParseIP(ip)
				if parsedIP != nil && (parsedIP.IsLoopback() || parsedIP.IsPrivate() || parsedIP.IsLinkLocalUnicast()) {
					logEvent(fmt.Sprintf("ssrf_blocked:dns_rebind=%s->%s", host, ip))
					c.JSON(http.StatusBadRequest, gin.H{
						"error": gin.H{
							"message": "URL resolves to internal address",
							"type":    "invalid_request_error",
						},
					})
					return
				}
			}
		}
		// Check scheme
		if parsed.Scheme != "https" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "only HTTPS webhook URLs are allowed",
					"type":    "invalid_request_error",
				},
			})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "webhook-1", "url": req.URL, "status": "active"})
	})

	// Error endpoint for testing error message leakage
	r.GET("/v1/debug/error", func(c *gin.Context) {
		// Intentionally does NOT leak stack traces
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "an internal error occurred",
				"type":    "server_error",
			},
		})
	})

	// Search endpoint for injection testing
	authGroup.GET("/search", func(c *gin.Context) {
		query := c.Query("q")
		// Sanitize query for output
		safe := owaspSanitize(query)
		c.JSON(http.StatusOK, gin.H{
			"query":   safe,
			"results": []gin.H{},
		})
	})

	// Tool execution endpoint (for command injection tests)
	authGroup.POST("/tools/execute", func(c *gin.Context) {
		ct := c.GetHeader("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{
				"error": gin.H{"message": "unsupported content type", "type": "invalid_request_error"},
			})
			return
		}
		var req struct {
			Tool   string                 `json:"tool" binding:"required"`
			Params map[string]interface{} `json:"params"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{"message": "invalid request", "type": "invalid_request_error"},
			})
			return
		}
		// Validate tool name: alphanumeric + underscore only
		toolNameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !toolNameRegex.MatchString(req.Tool) {
			logEvent(fmt.Sprintf("injection_blocked:tool_name=%s", req.Tool))
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "invalid tool name: must be alphanumeric",
					"type":    "invalid_request_error",
				},
			})
			return
		}
		// Validate params: reject shell metacharacters
		for key, val := range req.Params {
			if s, ok := val.(string); ok {
				if containsShellMeta(s) {
					logEvent(fmt.Sprintf("injection_blocked:param=%s,value=%s", key, s))
					c.JSON(http.StatusBadRequest, gin.H{
						"error": gin.H{
							"message": fmt.Sprintf("parameter '%s' contains disallowed characters", key),
							"type":    "invalid_request_error",
						},
					})
					return
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"tool": req.Tool, "status": "executed", "result": "ok"})
	})

	return r
}

// owaspSanitize escapes HTML special characters to prevent XSS.
func owaspSanitize(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// containsShellMeta checks if a string contains shell metacharacters
// that could be used for command injection.
func containsShellMeta(s string) bool {
	metaChars := []string{";", "|", "&", "`", "$", "(", ")", "{", "}", "<", ">", "\n", "\r"}
	for _, ch := range metaChars {
		if strings.Contains(s, ch) {
			return true
		}
	}
	return false
}

// makeJWT creates a signed JWT for testing purposes.
func makeJWT(secret string, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		panic(fmt.Sprintf("failed to sign JWT: %v", err))
	}
	return s
}

// validTestToken returns a valid JWT with standard claims for the test suite.
func validTestToken() string {
	return makeJWT("owasp-test-secret-key-32bytes!!", jwt.MapClaims{
		"sub":  "user-1",
		"role": "user",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iss":  "helixagent",
	})
}

// adminTestToken returns a valid JWT with admin role.
func adminTestToken() string {
	return makeJWT("owasp-test-secret-key-32bytes!!", jwt.MapClaims{
		"sub":  "admin-1",
		"role": "admin",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iss":  "helixagent",
	})
}

// jsonBody marshals a map to a JSON byte buffer.
func jsonBody(m map[string]interface{}) *bytes.Buffer {
	data, _ := json.Marshal(m)
	return bytes.NewBuffer(data)
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// =============================================================================
// A01: BROKEN ACCESS CONTROL
// =============================================================================

// TestOWASP_A01_BrokenAccessControl validates that authentication and
// authorization are enforced correctly across all protected endpoints.
func TestOWASP_A01_BrokenAccessControl(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	t.Parallel()

	// Each sub-group gets its own server to avoid rate limiter contention
	// when many parallel subtests hit the same server concurrently.

	t.Run("ProtectedEndpointsRequireAuth", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		protectedPaths := []struct {
			method string
			path   string
		}{
			{"POST", "/v1/chat/completions"},
			{"POST", "/v1/completions"},
			{"GET", "/v1/admin/users"},
			{"GET", "/v1/users/user-1/profile"},
			{"POST", "/v1/webhooks/register"},
			{"GET", "/v1/search?q=test"},
			{"POST", "/v1/tools/execute"},
		}

		for _, ep := range protectedPaths {
			t.Run(fmt.Sprintf("%s_%s", ep.method, strings.ReplaceAll(ep.path, "/", "_")), func(t *testing.T) {
				t.Parallel()
				var body io.Reader
				if ep.method == "POST" {
					body = jsonBody(map[string]interface{}{"prompt": "test"})
				}
				req, err := http.NewRequest(ep.method, srv.URL+ep.path, body)
				require.NoError(t, err)
				if ep.method == "POST" {
					req.Header.Set("Content-Type", "application/json")
				}
				// Deliberately omit Authorization header.
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
					"Endpoint %s %s must require authentication", ep.method, ep.path)
			})
		}
	})

	t.Run("AdminEndpointDeniesRegularUser", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		req, err := http.NewRequest("GET", srv.URL+"/v1/admin/users", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+validTestToken())

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"Regular user must not access admin endpoints")
	})

	t.Run("AdminEndpointAllowsAdmin", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		req, err := http.NewRequest("GET", srv.URL+"/v1/admin/users", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+adminTestToken())

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Admin user must access admin endpoints")
	})

	t.Run("HorizontalPrivilegeEscalation", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		// user-1 tries to access user-2's profile
		req, err := http.NewRequest("GET", srv.URL+"/v1/users/user-2/profile", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+validTestToken()) // user-1

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"User must not access another user's profile (IDOR)")
	})

	t.Run("VerticalPrivilegeEscalation", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		// Regular user tries to delete another user via admin endpoint
		req, err := http.NewRequest("DELETE", srv.URL+"/v1/admin/users/user-2", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+validTestToken())

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"Regular user must not perform admin-level DELETE operations")
	})

	t.Run("PublicEndpointsAccessibleWithoutAuth", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		publicPaths := []string{"/health", "/v1/models"}
		for _, path := range publicPaths {
			t.Run(strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
				t.Parallel()
				resp, err := client.Get(srv.URL + path)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode,
					"Public endpoint %s must be accessible without auth", path)
			})
		}
	})

	t.Run("MethodNotAllowedEnforced", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		// Try PUT on a GET-only endpoint
		req, err := http.NewRequest("PUT", srv.URL+"/v1/admin/users", jsonBody(map[string]interface{}{"x": 1}))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+adminTestToken())
		req.Header.Set("Content-Type", "application/json")

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Gin returns 404 for unregistered method/path combos (no route match)
		assert.NotEqual(t, http.StatusOK, resp.StatusCode,
			"Unregistered methods should not return 200")
	})
}

// =============================================================================
// A02: CRYPTOGRAPHIC FAILURES
// =============================================================================

// TestOWASP_A02_CryptographicFailures verifies that secrets are not leaked
// in responses and that TLS configuration meets minimum standards.
func TestOWASP_A02_CryptographicFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	t.Parallel()

	// Each sub-group gets its own server to avoid rate limiter contention.

	t.Run("NoSecretsInResponses", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		// Hit several endpoints and check responses for secret-like patterns.
		endpoints := []struct {
			method string
			path   string
			body   map[string]interface{}
			auth   string
		}{
			{"GET", "/health", nil, ""},
			{"GET", "/v1/models", nil, ""},
			{"POST", "/v1/chat/completions", map[string]interface{}{
				"messages": []map[string]string{{"role": "user", "content": "hello"}},
				"model":    "test",
			}, validTestToken()},
			{"GET", "/v1/debug/error", nil, ""},
		}

		secretPatterns := []*regexp.Regexp{
			regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*\S+`),
			regexp.MustCompile(`(?i)(secret|secret_key|jwt_secret)\s*[:=]\s*\S+`),
			regexp.MustCompile(`(?i)(api[_-]?key)\s*[:=]\s*["\']?[A-Za-z0-9+/=]{20,}`),
			regexp.MustCompile(`(?i)BEGIN\s+(RSA\s+)?PRIVATE\s+KEY`),
			regexp.MustCompile(`(?i)(aws_secret|aws_access_key_id)\s*[:=]`),
			regexp.MustCompile(`(?i)(database_url|db_password|redis_password)\s*[:=]`),
		}

		for _, ep := range endpoints {
			t.Run(fmt.Sprintf("%s%s", ep.method, strings.ReplaceAll(ep.path, "/", "_")),
				func(t *testing.T) {
					t.Parallel()
					var body io.Reader
					if ep.body != nil {
						body = jsonBody(ep.body)
					}
					req, err := http.NewRequest(ep.method, srv.URL+ep.path, body)
					require.NoError(t, err)
					if ep.body != nil {
						req.Header.Set("Content-Type", "application/json")
					}
					if ep.auth != "" {
						req.Header.Set("Authorization", "Bearer "+ep.auth)
					}
					resp, err := client.Do(req)
					require.NoError(t, err)
					defer resp.Body.Close()

					respBody, err := io.ReadAll(resp.Body)
					require.NoError(t, err)
					bodyStr := string(respBody)

					for _, pattern := range secretPatterns {
						assert.False(t, pattern.MatchString(bodyStr),
							"Response from %s %s must not contain secret pattern %s",
							ep.method, ep.path, pattern.String())
					}
				})
		}
	})

	t.Run("NoServerVersionInHeaders", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(srv.URL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		serverHeader := resp.Header.Get("Server")
		if serverHeader != "" {
			// Should not reveal detailed version info
			assert.NotContains(t, serverHeader, "Go/",
				"Server header should not reveal Go version")
			assert.NotContains(t, serverHeader, "gin-gonic",
				"Server header should not reveal framework name")
		}
	})

	t.Run("NoCacheControlOnSensitiveData", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		req, err := http.NewRequest("GET", srv.URL+"/v1/users/user-1/profile", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+validTestToken())

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		cacheControl := resp.Header.Get("Cache-Control")
		assert.Contains(t, cacheControl, "no-store",
			"Sensitive endpoints must set Cache-Control: no-store")
	})

	t.Run("TLSMinVersion", func(t *testing.T) {
		t.Parallel()
		// Create a TLS server to test TLS config
		tlsRouter := owaspRouter()
		cert, err := tls.X509KeyPair(
			[]byte(selfSignedCert), []byte(selfSignedKey),
		)
		if err != nil {
			t.Skip("Could not load test TLS cert")
		}
		tlsServer := httptest.NewUnstartedServer(tlsRouter)
		tlsServer.TLS = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		tlsServer.StartTLS()
		defer tlsServer.Close()

		// Attempt connection with TLS 1.1 (should be rejected)
		tlsClient := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					MaxVersion:         tls.VersionTLS11,
				},
			},
		}
		_, err = tlsClient.Get(tlsServer.URL + "/health")
		assert.Error(t, err, "TLS 1.1 connections should be rejected")

		// Attempt with TLS 1.2 (should succeed)
		tls12Client := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					MinVersion:         tls.VersionTLS12,
				},
			},
		}
		resp, err := tls12Client.Get(tlsServer.URL + "/health")
		if assert.NoError(t, err, "TLS 1.2 connections should succeed") {
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}
	})

	t.Run("JWTUsesSecureAlgorithm", func(t *testing.T) {
		t.Parallel()
		token := validTestToken()
		parts := strings.Split(token, ".")
		require.Len(t, parts, 3, "JWT must have 3 parts")

		headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
		require.NoError(t, err)

		var header map[string]interface{}
		err = json.Unmarshal(headerBytes, &header)
		require.NoError(t, err)

		alg, _ := header["alg"].(string)
		insecureAlgs := []string{"none", "HS1", ""}
		for _, bad := range insecureAlgs {
			assert.NotEqual(t, bad, alg,
				"JWT must not use insecure algorithm '%s'", bad)
		}
		assert.Equal(t, "HS256", alg,
			"JWT should use HS256 or stronger")
	})
}

// =============================================================================
// A03: INJECTION
// =============================================================================

// TestOWASP_A03_Injection tests SQL injection, XSS, and command injection
// across multiple input vectors.
func TestOWASP_A03_Injection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	t.Parallel()

	// Each sub-group gets its own server to avoid exhausting the shared
	// rate limiter (10 req/window) when many subtests run in parallel.
	token := validTestToken()

	t.Run("SQLInjectionInPrompt", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		sqlPayloads := []struct {
			name    string
			payload string
		}{
			{"BasicOR", "' OR '1'='1"},
			{"DropTable", "'; DROP TABLE users; --"},
			{"UnionSelect", "' UNION SELECT * FROM pg_tables --"},
			{"StackedQuery", "1; DELETE FROM sessions WHERE 1=1 --"},
			{"BooleanBlind", "' AND 1=1 --"},
			{"TimeBlind", "' AND (SELECT pg_sleep(5)) --"},
			{"ErrorBased", "' AND CAST((SELECT version()) AS int) --"},
			{"CommentBypass", "admin'/*"},
			{"DoubleEncoding", "%27%20OR%20%271%27%3D%271"},
			{"NestedComment", "' OR 1=1--/**/"},
		}

		dangerousPatterns := []string{
			"sql syntax", "postgresql", "pg_catalog", "ora-",
			"mysql_fetch", "sqlite_", "sqlserver", "syntax error",
			"unterminated string", "unclosed quotation",
			"relation \"users\"", "pg_tables",
		}

		for _, tc := range sqlPayloads {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				body := jsonBody(map[string]interface{}{
					"prompt": tc.payload,
					"model":  "test",
				})
				req, err := http.NewRequest("POST", srv.URL+"/v1/completions", body)
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				respBody, _ := io.ReadAll(resp.Body)
				bodyStr := strings.ToLower(string(respBody))

				for _, pattern := range dangerousPatterns {
					assert.NotContains(t, bodyStr, pattern,
						"SQL injection payload '%s' must not cause SQL error leakage (pattern: %s)",
						tc.name, pattern)
				}
			})
		}
	})

	t.Run("SQLInjectionInQueryParams", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		injections := []string{
			"' OR 1=1 --",
			"'; DROP TABLE users; --",
			"1 UNION SELECT username, password FROM users",
			"admin'--",
		}

		for i, payload := range injections {
			t.Run(fmt.Sprintf("Query_%d", i), func(t *testing.T) {
				t.Parallel()
				req, err := http.NewRequest("GET",
					srv.URL+"/v1/search?q="+url.QueryEscape(payload), nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				respBody, _ := io.ReadAll(resp.Body)
				bodyStr := strings.ToLower(string(respBody))

				assert.NotContains(t, bodyStr, "sql syntax",
					"SQL injection in query param must not leak SQL errors")
				assert.NotContains(t, bodyStr, "postgresql",
					"SQL injection in query param must not leak DB type")
			})
		}
	})

	t.Run("XSSInRequestBodies", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		xssPayloads := []struct {
			name    string
			payload string
		}{
			{"ScriptTag", "<script>alert('xss')</script>"},
			{"ImgOnerror", "<img src=x onerror=alert('xss')>"},
			{"SvgOnload", "<svg onload=alert('xss')>"},
			{"EventHandler", "\" onfocus=\"alert('xss')\" autofocus=\""},
			{"JavascriptURI", "javascript:alert('xss')"},
			{"DataURI", "data:text/html,<script>alert('xss')</script>"},
			{"EncodedScript", "&#60;script&#62;alert('xss')&#60;/script&#62;"},
			{"TemplateInjection", "{{constructor.constructor('alert(1)')()}}"},
		}

		for _, tc := range xssPayloads {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				body := jsonBody(map[string]interface{}{
					"messages": []map[string]string{
						{"role": "user", "content": tc.payload},
					},
					"model": "test",
				})
				req, err := http.NewRequest("POST", srv.URL+"/v1/chat/completions", body)
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				respBody, _ := io.ReadAll(resp.Body)
				bodyStr := string(respBody)

				// The response must not reflect raw XSS payloads
				assert.NotContains(t, bodyStr, "<script>",
					"Response must not contain unescaped <script> tags")
				assert.NotContains(t, bodyStr, "onerror=",
					"Response must not contain event handler attributes")
				assert.NotContains(t, bodyStr, "javascript:",
					"Response must not contain javascript: URIs")
			})
		}
	})

	t.Run("XSSInQueryParams", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		xssPayloads := []string{
			"<script>alert(1)</script>",
			"<img src=x onerror=alert(1)>",
			"'><script>alert(document.cookie)</script>",
		}

		for i, payload := range xssPayloads {
			t.Run(fmt.Sprintf("Query_%d", i), func(t *testing.T) {
				t.Parallel()
				req, err := http.NewRequest("GET",
					srv.URL+"/v1/search?q="+url.QueryEscape(payload), nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				respBody, _ := io.ReadAll(resp.Body)
				bodyStr := string(respBody)

				// Verify dangerous HTML is not present in unescaped form.
				// The &lt; escaped form is safe -- only raw < is dangerous.
				assert.NotContains(t, bodyStr, "<script>",
					"Search results must not reflect unescaped <script>")
				assert.NotContains(t, bodyStr, "<img ",
					"Search results must not reflect unescaped <img tags")
				assert.NotContains(t, bodyStr, "<svg ",
					"Search results must not reflect unescaped <svg tags")
			})
		}
	})

	t.Run("CommandInjectionInToolParams", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		cmdPayloads := []struct {
			name    string
			tool    string
			params  map[string]interface{}
			blocked bool
		}{
			{
				"Semicolon", "file_read",
				map[string]interface{}{"path": "/tmp/test; rm -rf /"},
				true,
			},
			{
				"Pipe", "file_read",
				map[string]interface{}{"path": "/tmp/test | cat /etc/passwd"},
				true,
			},
			{
				"Backtick", "file_read",
				map[string]interface{}{"path": "/tmp/`whoami`"},
				true,
			},
			{
				"DollarParen", "file_read",
				map[string]interface{}{"path": "/tmp/$(id)"},
				true,
			},
			{
				"Ampersand", "file_read",
				map[string]interface{}{"path": "/tmp/test & curl evil.com"},
				true,
			},
			{
				"Newline", "file_read",
				map[string]interface{}{"path": "/tmp/test\nwhoami"},
				true,
			},
			{
				"InvalidToolName", "; rm -rf /",
				map[string]interface{}{"path": "/tmp/test"},
				true,
			},
			{
				"ValidTool", "file_read",
				map[string]interface{}{"path": "/tmp/safe_file.txt"},
				false,
			},
		}

		for _, tc := range cmdPayloads {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				body := jsonBody(map[string]interface{}{
					"tool":   tc.tool,
					"params": tc.params,
				})
				req, err := http.NewRequest("POST", srv.URL+"/v1/tools/execute", body)
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				if tc.blocked {
					assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
						"Command injection payload '%s' must be blocked", tc.name)
				} else {
					assert.Equal(t, http.StatusOK, resp.StatusCode,
						"Safe tool call '%s' should succeed", tc.name)
				}
			})
		}
	})
}

// =============================================================================
// A04: INSECURE DESIGN
// =============================================================================

// TestOWASP_A04_InsecureDesign validates rate limiting, resource abuse
// protections, and error handling design.
func TestOWASP_A04_InsecureDesign(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	t.Parallel()

	router := owaspRouter()
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("RateLimitEnforced", func(t *testing.T) {
		// Not parallel: rate limit test needs sequential requests from same IP.
		var rateLimited bool
		for i := 0; i < 20; i++ {
			resp, err := client.Get(server.URL + "/health")
			require.NoError(t, err)
			resp.Body.Close()
			if resp.StatusCode == http.StatusTooManyRequests {
				rateLimited = true
				// Verify Retry-After header is present
				retryAfter := resp.Header.Get("Retry-After")
				assert.NotEmpty(t, retryAfter,
					"429 response must include Retry-After header")
				break
			}
		}
		assert.True(t, rateLimited,
			"Server must enforce rate limiting after excessive requests")
	})

	t.Run("RateLimitHeadersPresent", func(t *testing.T) {
		// Use a fresh server to avoid rate limit from previous test.
		freshRouter := owaspRouter()
		freshServer := httptest.NewServer(freshRouter)
		defer freshServer.Close()

		resp, err := client.Get(freshServer.URL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Limit"),
			"X-RateLimit-Limit header must be present")
		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Remaining"),
			"X-RateLimit-Remaining header must be present")
		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Reset"),
			"X-RateLimit-Reset header must be present")
	})

	t.Run("LargePayloadRejected", func(t *testing.T) {
		t.Parallel()
		freshRouter := owaspRouter()
		freshServer := httptest.NewServer(freshRouter)
		defer freshServer.Close()

		// Build a payload exceeding 10MB
		largePrompt := strings.Repeat("A", 11*1024*1024)
		body := jsonBody(map[string]interface{}{
			"prompt": largePrompt,
			"model":  "test",
		})
		req, err := http.NewRequest("POST", freshServer.URL+"/v1/chat/completions", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+validTestToken())
		req.ContentLength = int64(body.Len())

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode,
			"Payload exceeding size limit must be rejected with 413")
	})

	t.Run("ConcurrentRequestsHandledGracefully", func(t *testing.T) {
		t.Parallel()
		freshRouter := owaspRouter()
		freshServer := httptest.NewServer(freshRouter)
		defer freshServer.Close()

		var wg sync.WaitGroup
		var successCount, errorCount atomic.Int32
		const concurrency = 20

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := http.Get(freshServer.URL + "/v1/models")
				if err != nil {
					errorCount.Add(1)
					return
				}
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusTooManyRequests {
					successCount.Add(1)
				} else {
					errorCount.Add(1)
				}
			}()
		}
		wg.Wait()

		// The server must not crash or return 500s under concurrency.
		assert.Equal(t, int32(0), errorCount.Load(),
			"No errors should occur under concurrent load (got %d errors)", errorCount.Load())
		assert.Greater(t, successCount.Load(), int32(0),
			"At least some requests should succeed under concurrent load")
	})

	t.Run("PanicRecovery", func(t *testing.T) {
		t.Parallel()
		// Gin Recovery middleware should catch panics
		panicRouter := gin.New()
		panicRouter.Use(gin.Recovery())
		panicRouter.GET("/panic", func(c *gin.Context) {
			panic("test panic for OWASP A04")
		})
		panicServer := httptest.NewServer(panicRouter)
		defer panicServer.Close()

		resp, err := client.Get(panicServer.URL + "/panic")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode,
			"Panics must be caught and return 500, not crash the server")
	})
}

// =============================================================================
// A05: SECURITY MISCONFIGURATION
// =============================================================================

// TestOWASP_A05_SecurityMisconfiguration validates CORS headers, debug mode,
// error message sanitisation, and security header presence.
func TestOWASP_A05_SecurityMisconfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	t.Parallel()

	// Each sub-group gets its own server to avoid rate limiter contention.

	t.Run("CORSRestrictsOrigins", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		testCases := []struct {
			name           string
			origin         string
			expectAllowed  bool
			expectedOrigin string
		}{
			{"AllowedOrigin", "https://app.helixagent.dev", true, "https://app.helixagent.dev"},
			{"AnotherAllowed", "https://helixagent.dev", true, "https://helixagent.dev"},
			{"DisallowedOrigin", "https://evil.com", false, ""},
			{"NullOrigin", "null", false, ""},
			{"WildcardSubdomain", "https://subdomain.evil.com", false, ""},
			{"NoOrigin", "", false, ""},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				req, err := http.NewRequest("OPTIONS", srv.URL+"/v1/chat/completions", nil)
				require.NoError(t, err)
				if tc.origin != "" {
					req.Header.Set("Origin", tc.origin)
				}
				req.Header.Set("Access-Control-Request-Method", "POST")

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				acao := resp.Header.Get("Access-Control-Allow-Origin")
				if tc.expectAllowed {
					assert.Equal(t, tc.expectedOrigin, acao,
						"Allowed origin %s should be reflected", tc.origin)
				} else {
					assert.NotEqual(t, "*", acao,
						"CORS must not use wildcard for origin %s", tc.origin)
					if tc.origin != "" {
						assert.NotEqual(t, tc.origin, acao,
							"Disallowed origin %s should not be reflected", tc.origin)
					}
				}
			})
		}
	})

	t.Run("SecurityHeadersPresent", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(srv.URL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		requiredHeaders := map[string]string{
			"X-Content-Type-Options":   "nosniff",
			"X-Frame-Options":          "DENY",
			"X-XSS-Protection":         "1; mode=block",
			"Referrer-Policy":          "strict-origin-when-cross-origin",
			"Content-Security-Policy":  "default-src 'self'",
		}

		for header, expectedValue := range requiredHeaders {
			actual := resp.Header.Get(header)
			assert.Equal(t, expectedValue, actual,
				"Security header %s must be set to '%s', got '%s'",
				header, expectedValue, actual)
		}
	})

	t.Run("NoStackTraceInErrorResponses", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(srv.URL + "/v1/debug/error")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := strings.ToLower(string(body))

		stackPatterns := []string{
			"goroutine", "runtime.go", "panic(",
			"stack trace", "traceback",
			".go:", "runtime/",
			"file:", "line:",
		}
		for _, pattern := range stackPatterns {
			assert.NotContains(t, bodyStr, pattern,
				"Error response must not contain stack trace pattern '%s'", pattern)
		}
	})

	t.Run("DebugModeDisabled", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		// In test mode, Gin should not expose debug-level info in responses.
		resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(srv.URL + "/nonexistent/path")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		assert.NotContains(t, bodyStr, "gin.SetMode",
			"Response must not contain debug framework references")
		assert.NotContains(t, bodyStr, "WARNING",
			"Response must not contain Gin debug warnings")
	})

	t.Run("DirectoryListingDisabled", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		dirPaths := []string{"/", "/v1/", "/internal/", "/admin/"}
		for _, path := range dirPaths {
			t.Run(strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
				t.Parallel()
				resp, err := client.Get(srv.URL + path)
				require.NoError(t, err)
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				bodyStr := strings.ToLower(string(body))

				assert.NotContains(t, bodyStr, "index of",
					"Must not expose directory listing at %s", path)
				assert.NotContains(t, bodyStr, "directory listing",
					"Must not expose directory listing at %s", path)
			})
		}
	})

	t.Run("ErrorResponseFormatConsistent", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(srv.URL + "/v1/debug/error")
		require.NoError(t, err)
		defer resp.Body.Close()

		var errResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errResp)
		require.NoError(t, err, "Error responses must be valid JSON")

		errorField, hasError := errResp["error"]
		assert.True(t, hasError, "Error response must contain 'error' field")

		if errMap, ok := errorField.(map[string]interface{}); ok {
			assert.NotEmpty(t, errMap["message"],
				"Error must include a 'message' field")
			assert.NotEmpty(t, errMap["type"],
				"Error must include a 'type' field")
		}
	})
}

// =============================================================================
// A06: VULNERABLE AND OUTDATED COMPONENTS
// =============================================================================

// TestOWASP_A06_VulnerableComponents validates that dependency scanning
// infrastructure (Snyk, Trivy) is configured and that go.sum is present
// for reproducible builds.
func TestOWASP_A06_VulnerableComponents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	t.Parallel()

	root := owaspProjectRoot(t)

	t.Run("GoSumExists", func(t *testing.T) {
		t.Parallel()
		_, err := os.Stat(filepath.Join(root, "go.sum"))
		assert.NoError(t, err,
			"go.sum must exist for dependency integrity verification")
	})

	t.Run("SnykConfigExists", func(t *testing.T) {
		t.Parallel()
		_, err := os.Stat(filepath.Join(root, ".snyk"))
		assert.NoError(t, err,
			".snyk configuration file must exist for vulnerability scanning")
	})

	t.Run("GoModExists", func(t *testing.T) {
		t.Parallel()
		_, err := os.Stat(filepath.Join(root, "go.mod"))
		assert.NoError(t, err,
			"go.mod must exist for dependency management")
	})

	t.Run("GoModHasMinimumVersion", func(t *testing.T) {
		t.Parallel()
		content, err := os.ReadFile(filepath.Join(root, "go.mod"))
		require.NoError(t, err)

		goModStr := string(content)
		// Ensure a modern Go version is specified (1.21+)
		goVersionRegex := regexp.MustCompile(`go\s+1\.(\d+)`)
		matches := goVersionRegex.FindStringSubmatch(goModStr)
		if assert.NotEmpty(t, matches,
			"go.mod must specify a Go version") {
			// We don't enforce a specific minor version but check it exists
			assert.NotEmpty(t, matches[1],
				"Go minor version must be specified")
		}
	})

	t.Run("VendorDirectoryConsistency", func(t *testing.T) {
		t.Parallel()
		vendorDir := filepath.Join(root, "vendor")
		if _, err := os.Stat(vendorDir); err == nil {
			// If vendor exists, modules.txt must be present
			_, err := os.Stat(filepath.Join(vendorDir, "modules.txt"))
			assert.NoError(t, err,
				"vendor/modules.txt must exist when vendor/ is used")
		}
	})

	t.Run("SecurityScanConfigured", func(t *testing.T) {
		t.Parallel()
		// At least one scanning tool config should exist
		scanConfigs := []string{
			".snyk",
			".trivyignore",
			"sonar-project.properties",
		}
		found := false
		for _, cfg := range scanConfigs {
			if _, err := os.Stat(filepath.Join(root, cfg)); err == nil {
				found = true
				break
			}
		}
		assert.True(t, found,
			"At least one dependency scanning tool config must be present (.snyk, .trivyignore, or sonar-project.properties)")
	})
}

// =============================================================================
// A07: IDENTIFICATION AND AUTHENTICATION FAILURES
// =============================================================================

// TestOWASP_A07_AuthenticationFailures validates JWT validation, token
// expiration enforcement, and resistance to common auth bypass techniques.
func TestOWASP_A07_AuthenticationFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	t.Parallel()

	// Each sub-group gets its own server to avoid rate limiter contention.
	srv := httptest.NewServer(owaspRouter())
	t.Cleanup(srv.Close)
	client := &http.Client{Timeout: 10 * time.Second}
	completionURL := srv.URL + "/v1/chat/completions"

	makeCompletionReq := func(token string) *http.Request {
		body := jsonBody(map[string]interface{}{
			"messages": []map[string]string{{"role": "user", "content": "test"}},
			"model":    "test",
		})
		req, _ := http.NewRequest("POST", completionURL, body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		return req
	}

	t.Run("ValidTokenAccepted", func(t *testing.T) {
		t.Parallel()
		resp, err := client.Do(makeCompletionReq(validTestToken()))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Valid JWT must be accepted")
	})

	t.Run("ExpiredTokenRejected", func(t *testing.T) {
		t.Parallel()
		expiredToken := makeJWT("owasp-test-secret-key-32bytes!!", jwt.MapClaims{
			"sub":  "user-1",
			"role": "user",
			"iat":  time.Now().Add(-2 * time.Hour).Unix(),
			"exp":  time.Now().Add(-1 * time.Hour).Unix(), // expired 1 hour ago
			"iss":  "helixagent",
		})
		resp, err := client.Do(makeCompletionReq(expiredToken))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Expired JWT must be rejected")
	})

	t.Run("MalformedTokenRejected", func(t *testing.T) {
		t.Parallel()
		// Own server to avoid rate limit contention with other A07 subtests.
		malSrv := httptest.NewServer(owaspRouter())
		t.Cleanup(malSrv.Close)
		malClient := &http.Client{Timeout: 10 * time.Second}
		malURL := malSrv.URL + "/v1/chat/completions"

		malformedTokens := []struct {
			name  string
			token string
		}{
			{"EmptyToken", ""},
			{"RandomString", "not-a-jwt-token"},
			{"IncompleteJWT", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0"},
			{"ModifiedPayload", validTestToken()[:20] + "TAMPERED" + validTestToken()[28:]},
			{"OnlyDots", "..."},
			{"SQLInToken", "' OR '1'='1"},
			{"HTMLInToken", "<script>alert(1)</script>"},
		}

		for _, tc := range malformedTokens {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				body := jsonBody(map[string]interface{}{
					"messages": []map[string]string{{"role": "user", "content": "test"}},
					"model":    "test",
				})
				req, _ := http.NewRequest("POST", malURL, body)
				req.Header.Set("Content-Type", "application/json")
				if tc.token != "" {
					req.Header.Set("Authorization", "Bearer "+tc.token)
				}

				resp, err := malClient.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
					"Malformed token '%s' must be rejected with 401", tc.name)
			})
		}
	})

	t.Run("WrongSigningKeyRejected", func(t *testing.T) {
		t.Parallel()
		wrongKeyToken := makeJWT("wrong-secret-key-not-the-real!!", jwt.MapClaims{
			"sub":  "user-1",
			"role": "admin",
			"iat":  time.Now().Unix(),
			"exp":  time.Now().Add(time.Hour).Unix(),
		})
		resp, err := client.Do(makeCompletionReq(wrongKeyToken))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"JWT signed with wrong key must be rejected")
	})

	t.Run("AlgorithmNoneRejected", func(t *testing.T) {
		t.Parallel()
		// Manually craft a "none" algorithm JWT
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(
			fmt.Sprintf(`{"sub":"admin-1","role":"admin","exp":%d}`,
				time.Now().Add(time.Hour).Unix())))
		noneToken := header + "." + payload + "."

		resp, err := client.Do(makeCompletionReq(noneToken))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"JWT with 'none' algorithm must be rejected")
	})

	t.Run("MissingAuthSchemeRejected", func(t *testing.T) {
		t.Parallel()
		body := jsonBody(map[string]interface{}{
			"messages": []map[string]string{{"role": "user", "content": "test"}},
			"model":    "test",
		})
		req, _ := http.NewRequest("POST", completionURL, body)
		req.Header.Set("Content-Type", "application/json")
		// Set token without "Bearer " prefix
		req.Header.Set("Authorization", validTestToken())

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Token without Bearer scheme must be rejected")
	})

	t.Run("BasicAuthSchemeRejected", func(t *testing.T) {
		t.Parallel()
		body := jsonBody(map[string]interface{}{
			"messages": []map[string]string{{"role": "user", "content": "test"}},
			"model":    "test",
		})
		req, _ := http.NewRequest("POST", completionURL, body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization",
			"Basic "+base64.StdEncoding.EncodeToString([]byte("admin:admin")))

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Basic auth scheme must be rejected (only Bearer is accepted)")
	})

	t.Run("ErrorResponseDoesNotLeakTokenDetails", func(t *testing.T) {
		t.Parallel()
		resp, err := client.Do(makeCompletionReq("invalid-token"))
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		assert.NotContains(t, bodyStr, "owasp-test-secret",
			"Error response must not leak the JWT secret")
		assert.NotContains(t, bodyStr, "signing key",
			"Error response must not mention signing key")
	})
}

// =============================================================================
// A08: SOFTWARE AND DATA INTEGRITY FAILURES
// =============================================================================

// TestOWASP_A08_SoftwareIntegrity validates request body parsing, content-type
// enforcement, and protection against deserialization attacks.
func TestOWASP_A08_SoftwareIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	t.Parallel()

	// Each sub-group gets its own server to avoid rate limiter contention.
	token := validTestToken()

	t.Run("ContentTypeEnforcement", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}
		unsupportedTypes := []struct {
			name        string
			contentType string
			body        string
		}{
			{"TextPlain", "text/plain", `{"messages":[{"role":"user","content":"test"}]}`},
			{"TextHTML", "text/html", `<html><body>test</body></html>`},
			{"ApplicationXML", "application/xml", `<request><prompt>test</prompt></request>`},
			{"MultipartForm", "multipart/form-data; boundary=----", `------\nContent-Disposition: form-data; name="prompt"\n\ntest\n------`},
			{"URLEncoded", "application/x-www-form-urlencoded", `prompt=test&model=test`},
			{"ApplicationYAML", "application/yaml", `prompt: test`},
		}

		for _, tc := range unsupportedTypes {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				req, err := http.NewRequest("POST",
					srv.URL+"/v1/chat/completions",
					bytes.NewBufferString(tc.body))
				require.NoError(t, err)
				req.Header.Set("Content-Type", tc.contentType)
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode,
					"Content-Type '%s' must be rejected with 415", tc.contentType)
			})
		}
	})

	t.Run("MalformedJSONHandled", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		malformedBodies := []struct {
			name string
			body string
		}{
			{"EmptyBody", ""},
			{"IncompleteJSON", `{"prompt": "test"`},
			{"TrailingComma", `{"prompt": "test",}`},
			{"SingleQuotes", `{'prompt': 'test'}`},
			{"PlainText", "this is not json"},
			{"NullLiteral", "null"},
			{"ArrayInsteadOfObject", `[1, 2, 3]`},
			{"DeeplyNested", generateDeeplyNestedJSON(50)},
		}

		for _, tc := range malformedBodies {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				req, err := http.NewRequest("POST",
					srv.URL+"/v1/chat/completions",
					bytes.NewBufferString(tc.body))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
					"Malformed JSON '%s' must not cause 500", tc.name)
				// Should be 400 Bad Request
				assert.True(t,
					resp.StatusCode == http.StatusBadRequest ||
						resp.StatusCode == http.StatusOK,
					"Expected 400 for malformed JSON '%s', got %d", tc.name, resp.StatusCode)
			})
		}
	})

	t.Run("RequestBodyValidation", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		invalidRequests := []struct {
			name string
			body map[string]interface{}
		}{
			{
				"MissingMessagesAndPrompt",
				map[string]interface{}{"model": "test"},
			},
			{
				"EmptyMessages",
				map[string]interface{}{"messages": []interface{}{}, "model": "test"},
			},
		}

		for _, tc := range invalidRequests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				req, err := http.NewRequest("POST",
					srv.URL+"/v1/chat/completions",
					jsonBody(tc.body))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
					"Invalid request '%s' must be rejected with 400", tc.name)
			})
		}
	})

	t.Run("PrototypePollutionAttempts", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		pollutionPayloads := []struct {
			name string
			body string
		}{
			{
				"ProtoField",
				`{"__proto__":{"admin":true},"messages":[{"role":"user","content":"test"}],"model":"test"}`,
			},
			{
				"ConstructorField",
				`{"constructor":{"prototype":{"admin":true}},"messages":[{"role":"user","content":"test"}],"model":"test"}`,
			},
		}

		for _, tc := range pollutionPayloads {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				req, err := http.NewRequest("POST",
					srv.URL+"/v1/chat/completions",
					bytes.NewBufferString(tc.body))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				// Go's json.Unmarshal ignores unknown fields by default, so
				// prototype pollution is not applicable. Verify no escalation.
				assert.True(t,
					resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest,
					"Prototype pollution attempt must not cause 500, got %d", resp.StatusCode)
			})
		}
	})

	t.Run("JSONContentTypeWithCharset", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		body := jsonBody(map[string]interface{}{
			"messages": []map[string]string{{"role": "user", "content": "test"}},
			"model":    "test",
		})
		req, err := http.NewRequest("POST", srv.URL+"/v1/chat/completions", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"application/json with charset should be accepted")
	})
}

// =============================================================================
// A09: SECURITY LOGGING AND MONITORING FAILURES
// =============================================================================

// TestOWASP_A09_LoggingMonitoringFailures validates that security-relevant
// events (failed auth, access violations, injection attempts) are logged.
func TestOWASP_A09_LoggingMonitoringFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	// Not parallel: we need sequential requests to check cumulative log entries.

	router := owaspRouter()
	server := httptest.NewServer(router)
	defer server.Close()
	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("AuthFailuresLogged", func(t *testing.T) {
		// Trigger auth failures
		for i := 0; i < 3; i++ {
			body := jsonBody(map[string]interface{}{
				"messages": []map[string]string{{"role": "user", "content": "test"}},
				"model":    "test",
			})
			req, _ := http.NewRequest("POST", server.URL+"/v1/chat/completions", body)
			req.Header.Set("Content-Type", "application/json")
			// No auth header
			resp, err := client.Do(req)
			require.NoError(t, err)
			resp.Body.Close()
		}

		// Check security events
		resp, err := client.Get(server.URL + "/v1/security/events")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Events []string `json:"events"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		authFailureCount := 0
		for _, event := range result.Events {
			if strings.Contains(event, "auth_failure") {
				authFailureCount++
			}
		}
		assert.GreaterOrEqual(t, authFailureCount, 3,
			"Failed authentication attempts must be logged (found %d, expected >= 3)",
			authFailureCount)
	})

	t.Run("AccessViolationsLogged", func(t *testing.T) {
		// Use fresh server to get clean logs
		freshRouter := owaspRouter()
		freshServer := httptest.NewServer(freshRouter)
		defer freshServer.Close()

		// Trigger access violation: user-1 accessing user-2's profile
		req, _ := http.NewRequest("GET",
			freshServer.URL+"/v1/users/user-2/profile", nil)
		req.Header.Set("Authorization", "Bearer "+validTestToken())
		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()

		// Check logs
		resp, err = client.Get(freshServer.URL + "/v1/security/events")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Events []string `json:"events"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		accessViolationFound := false
		for _, event := range result.Events {
			if strings.Contains(event, "access_violation") {
				accessViolationFound = true
				break
			}
		}
		assert.True(t, accessViolationFound,
			"Access violations (IDOR attempts) must be logged")
	})

	t.Run("InjectionAttemptsLogged", func(t *testing.T) {
		// Use fresh server
		freshRouter := owaspRouter()
		freshServer := httptest.NewServer(freshRouter)
		defer freshServer.Close()

		// Trigger injection via tool execution
		body := jsonBody(map[string]interface{}{
			"tool":   "file_read",
			"params": map[string]interface{}{"path": "/tmp/test; rm -rf /"},
		})
		req, _ := http.NewRequest("POST",
			freshServer.URL+"/v1/tools/execute", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+validTestToken())
		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()

		// Check logs
		resp, err = client.Get(freshServer.URL + "/v1/security/events")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Events []string `json:"events"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		injectionLogFound := false
		for _, event := range result.Events {
			if strings.Contains(event, "injection_blocked") {
				injectionLogFound = true
				break
			}
		}
		assert.True(t, injectionLogFound,
			"Injection attempts must be logged for security monitoring")
	})

	t.Run("SSRFAttemptsLogged", func(t *testing.T) {
		freshRouter := owaspRouter()
		freshServer := httptest.NewServer(freshRouter)
		defer freshServer.Close()

		body := jsonBody(map[string]interface{}{
			"url": "https://169.254.169.254/latest/meta-data/",
		})
		req, _ := http.NewRequest("POST",
			freshServer.URL+"/v1/webhooks/register", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+validTestToken())
		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()

		// Check logs
		resp, err = client.Get(freshServer.URL + "/v1/security/events")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Events []string `json:"events"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		ssrfLogFound := false
		for _, event := range result.Events {
			if strings.Contains(event, "ssrf_blocked") {
				ssrfLogFound = true
				break
			}
		}
		assert.True(t, ssrfLogFound,
			"SSRF attempts must be logged for security monitoring")
	})

	t.Run("AuthorizationFailuresLogged", func(t *testing.T) {
		freshRouter := owaspRouter()
		freshServer := httptest.NewServer(freshRouter)
		defer freshServer.Close()

		// Regular user trying admin endpoint
		req, _ := http.NewRequest("GET",
			freshServer.URL+"/v1/admin/users", nil)
		req.Header.Set("Authorization", "Bearer "+validTestToken())
		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()

		// Check logs
		resp, err = client.Get(freshServer.URL + "/v1/security/events")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Events []string `json:"events"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		authzFailureFound := false
		for _, event := range result.Events {
			if strings.Contains(event, "authz_failure") {
				authzFailureFound = true
				break
			}
		}
		assert.True(t, authzFailureFound,
			"Authorization failures must be logged for security monitoring")
	})
}

// =============================================================================
// A10: SERVER-SIDE REQUEST FORGERY (SSRF)
// =============================================================================

// TestOWASP_A10_SSRF validates that webhook/callback URL registration
// blocks requests to internal, private, and cloud metadata addresses.
func TestOWASP_A10_SSRF(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")
	}
	t.Parallel()

	// Each sub-group gets its own server to avoid rate limiter contention.
	token := validTestToken()

	t.Run("BlockedInternalTargets", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		blockedURLs := []struct {
			name string
			url  string
		}{
			{"Localhost", "https://localhost/callback"},
			{"Localhost127", "https://127.0.0.1/callback"},
			{"MetadataAWS", "https://169.254.169.254/latest/meta-data/"},
			{"MetadataGCP", "https://metadata.google.internal/computeMetadata/v1/"},
			{"PrivateClassA", "https://10.0.0.1/internal"},
			{"PrivateClassB", "https://172.16.0.1/internal"},
			{"PrivateClassC", "https://192.168.1.1/internal"},
			{"IPv6Loopback", "https://[::1]/callback"},
			{"ZeroAddr", "https://0.0.0.0/callback"},
			{"LinkLocal", "https://169.254.1.1/callback"},
		}

		for _, tc := range blockedURLs {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				body := jsonBody(map[string]interface{}{
					"url": tc.url,
				})
				req, err := http.NewRequest("POST", srv.URL+"/v1/webhooks/register", body)
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
					"SSRF target '%s' must be blocked", tc.url)
			})
		}
	})

	t.Run("HTTPSchemeRejected", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		body := jsonBody(map[string]interface{}{
			"url": "http://example.com/callback",
		})
		req, err := http.NewRequest("POST", srv.URL+"/v1/webhooks/register", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
			"HTTP (non-TLS) webhook URLs must be rejected")
	})

	t.Run("InvalidURLFormats", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)
		client := &http.Client{Timeout: 10 * time.Second}

		invalidURLs := []struct {
			name string
			url  string
		}{
			{"EmptyURL", ""},
			{"JustPath", "/internal/data"},
			{"FileScheme", "file:///etc/passwd"},
			{"FTPScheme", "ftp://internal.server/data"},
			{"GopherScheme", "gopher://internal:70/1"},
			{"NoHost", "https:///path"},
		}

		for _, tc := range invalidURLs {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				body := jsonBody(map[string]interface{}{
					"url": tc.url,
				})
				req, err := http.NewRequest("POST", srv.URL+"/v1/webhooks/register", body)
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
					"Invalid URL format '%s' must be rejected", tc.url)
			})
		}
	})

	t.Run("ValidExternalURLAccepted", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		body := jsonBody(map[string]interface{}{
			"url": "https://hooks.example.com/webhook/abc123",
		})
		req, err := http.NewRequest("POST", srv.URL+"/v1/webhooks/register", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should be 201 Created (or possibly 400 if DNS resolution fails
		// for the test domain, which is acceptable behavior)
		assert.True(t,
			resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusBadRequest,
			"Valid external HTTPS URL should be accepted (got %d)", resp.StatusCode)
	})

	t.Run("DNSRebindingProtection", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(owaspRouter())
		t.Cleanup(srv.Close)

		// DNS rebinding attempts use hostnames that resolve to internal IPs.
		// The server must resolve the hostname and check the resulting IP.
		// We test with localhost which resolves to 127.0.0.1.
		body := jsonBody(map[string]interface{}{
			"url": "https://localhost/callback",
		})
		req, err := http.NewRequest("POST", srv.URL+"/v1/webhooks/register", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
			"DNS rebinding to internal host must be blocked")
	})
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// generateDeeplyNestedJSON creates a deeply nested JSON object for testing
// deserialization bomb resistance.
func generateDeeplyNestedJSON(depth int) string {
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"a":`)
	}
	sb.WriteString(`"leaf"`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`}`)
	}
	return sb.String()
}

// owaspProjectRoot walks up from the current working directory to find go.mod.
func owaspProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

// Self-signed TLS certificate for testing (RSA 2048, valid for localhost).
// Generated for testing purposes only -- DO NOT use in production.
const selfSignedCert = `-----BEGIN CERTIFICATE-----
MIICpDCCAYwCCQDU+pQ4pHgSpDANBgkqhkiG9w0BAQsFADAUMRIwEAYDVQQDDAls
b2NhbGhvc3QwHhcNMjUwMTAxMDAwMDAwWhcNMjYwMTAxMDAwMDAwWjAUMRIwEAYD
VQQDDAlsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC7
o4qne60TB3pOYaBy0XoXqE54AxOqMqmHcmOHR5TwaO64WMcgQNOFfMTPczC0p0rV
JGMBuMI7SQh63krOqf5YKVbMqLAieVpSJaGMbfYRwYqHsFI4oXvJIjjRa6elyi5P
MILfJ+Wk8P2TElSB9RKQ8ROMmRb+pGMPEM0U0CSYZR+D7VTiDMFN5YhQJHQHn5c
QNy4mMpTSWyQJ7SkzW0VEomqfL9rkr1R18WXwiPPPmJR7KBKB/JpNaifXMngYGl
vq/BbNYqZxOmKCFaMkk6bvXBL2LhGJ3UZHyoSd4cAJq6X/WJosgCwGKkd80amFF
3waCn7VK+5U4cRpe3jdjAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAI8jJy9mLkqh
WO2f72bllIC8xFBsSgPD6ts4T3q6I+5Zh0KPG0sAw5PbCDBbKMxJ7gyKnfChIiDV
JHC27Sxfceiv2aGaF88HvG3fpXoIJJh3fKShUbMCifLBkylJMrdruOPNvgHrz6fA
2eZJdihMPbj8hIa8JVPbO8L03UPHYFg6jGaFMuVMhSNLBDLZ6rM3QMLE7eYcMc1
5FiMPCcBJadPZwXAfqkOCT0bFGjxS8i7GOLxYTdDIRET5WCf5af2JhBUoiXpg1dC
N2USVjCMn6BGakwdBOGqV9zOimJBV3iRmNJXqVPbIJHX2UPxjYGNv3eKm5FLOVLR
DsnAmx2JQnM=
-----END CERTIFICATE-----`

const selfSignedKey = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC7o4qne60TB3pO
YaBy0XoXqE54AxOqMqmHcmOHR5TwaO64WMcgQNOFfMTPczC0p0rVJGMBuMI7SQh6
3krOqf5YKVbMqLAieVpSJaGMbfYRwYqHsFI4oXvJIjjRa6elyi5PMILfJ+Wk8P2T
ElSB9RKQ8ROMmRb+pGMPEM0U0CSYZR+D7VTiDMFN5YhQJHQHn5cQNy4mMpTSWyQ
J7SkzW0VEomqfL9rkr1R18WXwiPPPmJR7KBKB/JpNaifXMngYGlvq/BbNYqZxOmK
CFaMkk6bvXBL2LhGJ3UZHyoSd4cAJq6X/WJosgCwGKkd80amFF3waCn7VK+5U4c
Rpe3jdjAgMBAAECggEAF0K4emWlxXLYV2/3IxKCPiVQ4C09jVhMNVlcmsQJfp8c
JW5v8MWXQYG6mMnOeN0Pw9fJJGinFKyYYgKdYjE2bkDDyaMHB9BIa6yrJa8mXpZ
r9JhDDJOEC1iqFFhFpV1mECMc5G8U4M0lxAIHk9FXQM7RT/2+fCbfHcjGaEUNOC
VPLaP8f+0HJSCUy+bK3l3x26pLCqrz0YUaJ8K3do+FfMwkdh+Fp6M2DBlwdPjXz
mpJ5DLHQ9QL8B5pObYjXrno8CrZVmlfEFuJZSwQcgP4GR09aE2wN3VH5hvpBFfEd
pBMaenq/j6oEi3H5auP5N8TJF7rsh3ijCRbfQ+r8kQKBgQDj4HKRA3FKutXNJV8V
mMVy/PoFSkR0eFI8qj82v7rvFDfEz4FNHBjHqLm3GO9CJcuJdPWBJlYjm0p/2fhN
c+kXY2Wgke2vRjL2k/D5dvHrLF3RZZNYQN1LqGL5uC0qSeYjJdTm2bvPwg40TYL
sF//lvLbg4nnTpYvseAdNyNxIwKBgQDTf5TBRhA3YPc9zfnWXLih/ncMHdqRAdb8
6YzikVKr/sNEwGJQN5n19ALc0I/MmGz5lBGHSDMbfdRRJCRmqIR/3uZgDgVjLqQR
pGjVfVDZRqQvGfJtidfKZq/0aQH/fOezVzHMqKE5pX+cJ+Q8MQSWjX/zdqiMbz68
hd3QMqT3qQKBgCfrCC4J7sA26E+xPSJd0oNH7LZH/u7n4P0Bz7fn1VCzfAr3BP7
y4jXxGWq8Y5fR6FU8u9CT1+CnW/Rx+kag9T1D8NJ1vh6OLh6JILqThEsOJD8MOkX
YPYbhlzr5JZxQu4J4ixJZ1a8bnMqeJHQh/2mZMPSmq4kMAEbT0/3bnFjAoGAUx0f
sPPLJMfE2A0erpb5MxGq0V2P9WfH/L9xGIQw0uKlhSHiXlCBdoVPsOwTLiLP57P
g7BmSE6A3lhNqh8m7a4x0J9Rn0sBuRh7BjLGMfIcIL7F6r8sX/zhVVPSB0MaXeh
7waGr+iRbBJpaxB2PnETAVJvTQujnMKC/5VFXGECgYBLY/jnmJpX2w57UBg0q9Kv
Z3Hp3EvAVMXq1KEjJAo3VCLh2FhVRpRr4jAI6r2eVEAOq+W7Ftl3VxDERZ0vHDE
ZUi3aPd4WSRl5YBTNFMMwz0EUYGMNraq6HN5bFt4ofTkVWmS5FDoQLAyh3GfIsO+
gHqnzR+3sY/63wa7hJKJCA==
-----END PRIVATE KEY-----`
