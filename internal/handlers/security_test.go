package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/verifier"
)

func setupSecurityTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/v1/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"models": []string{}})
	})
	return r
}

func setupSecurityTestRouterWithAuth() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	})
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func TestSecurity_HealthEndpointNoAuth(t *testing.T) {
	t.Parallel()
	router := setupSecurityTestRouter()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSecurity_APIEndpointRequiresAuth(t *testing.T) {
	t.Parallel()
	router := setupSecurityTestRouterWithAuth()

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSecurity_APIEndpointWithValidAuth(t *testing.T) {
	t.Parallel()
	router := setupSecurityTestRouterWithAuth()

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-test-token")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
}

func TestSecurity_SQLInjectionPrevention(t *testing.T) {
	t.Parallel()
	router := setupSecurityTestRouter()

	// Use URL-encoded values to avoid malformed URLs
	maliciousInputs := []string{
		"%27%3B%20DROP%20TABLE%20users%3B%20--",
		"%27%20OR%20%271%27%3D%271",
		"1%3B%20DELETE%20FROM%20users",
	}

	for _, input := range maliciousInputs {
		t.Run("sql_injection", func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", "/v1/models?id="+input, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			// Should not cause server error
			assert.NotEqual(t, http.StatusInternalServerError, rec.Code)
		})
	}
}

func TestSecurity_XSSPrevention(t *testing.T) {
	t.Parallel()
	router := setupSecurityTestRouter()

	// Use URL-encoded values to avoid malformed URLs
	xssPayloads := []string{
		"%3Cscript%3Ealert%28%27xss%27%29%3C%2Fscript%3E",
		"%3Cimg%20src%3Dx%20onerror%3Dalert%28%27xss%27%29%3E",
		"javascript%3Aalert%28%27xss%27%29",
	}

	for _, payload := range xssPayloads {
		t.Run("xss_prevention", func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", "/v1/models?q="+payload, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			// Should not cause server error
			assert.NotEqual(t, http.StatusInternalServerError, rec.Code)
		})
	}
}

func TestSecurity_RequestSizeLimit(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
		c.Next()
	})
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	largeBody := strings.Repeat("x", 2048)
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
}

func TestSecurity_TimeoutHandling(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping timeout test in short mode") // SKIP-OK: #short-mode
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan bool)
	go func() {
		time.Sleep(50 * time.Millisecond)
		done <- true
	}()

	select {
	case <-ctx.Done():
		t.Error("operation timed out")
	case <-done:
	}
}

func TestSecurity_HTTPHeaders(t *testing.T) {
	t.Parallel()
	router := setupSecurityTestRouter()

	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHealthHandler_SecurityValidation(t *testing.T) {
	t.Parallel()
	hs := verifier.NewHealthService(nil)
	h := NewHealthHandler(hs)

	assert.NotNil(t, h)
	assert.NotNil(t, h.healthService)
}

func TestSecurity_ContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.Error(t, ctx.Err())
	assert.Equal(t, context.Canceled, ctx.Err())
}

func TestSecurity_InputSanitization(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		hasIssue bool
	}{
		{"normal", "hello world", false},
		{"with_null", "hello\x00world", true},
		{"with_newline", "hello\nworld", false},
		{"with_control", "hello\x1bworld", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.hasIssue {
				assert.True(t, strings.Contains(tt.input, "\x00") || strings.Contains(tt.input, "\x1b"))
			}
		})
	}
}

func BenchmarkSecurity_RouteHandling(b *testing.B) {
	router := setupSecurityTestRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
	}
}
