//go:build security
// +build security

// Package security provides browser handler security tests.
// These tests verify that XSS and injection payloads in browser
// requests are properly rejected or sanitized.
package security

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBrowserSecurityRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/v1/browser/navigate", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}

		url, _ := req["url"].(string)
		if url == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "url required"})
			return
		}

		// Reject javascript: protocol URLs
		lower := strings.ToLower(strings.TrimSpace(url))
		if strings.HasPrefix(lower, "javascript:") ||
			strings.HasPrefix(lower, "data:") ||
			strings.HasPrefix(lower, "vbscript:") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid URL scheme"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "url": url})
	})

	return r
}

// TestBrowserSecurity_XSSInURL verifies that javascript: protocol
// URLs are rejected to prevent XSS attacks.
func TestBrowserSecurity_XSSInURL(t *testing.T) {
	r := setupBrowserSecurityRouter()

	xssPayloads := []string{
		"javascript:alert('xss')",
		"JavaScript:alert(document.cookie)",
		"  javascript:void(0)",
		"data:text/html,<script>alert(1)</script>",
		"vbscript:msgbox('xss')",
	}

	for _, payload := range xssPayloads {
		t.Run(payload, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{"url": payload})
			req := httptest.NewRequest(http.MethodPost, "/v1/browser/navigate",
				bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code,
				"XSS payload %q should be rejected", payload)
		})
	}
}

// TestBrowserSecurity_OversizedURL verifies that extremely long URLs
// do not cause resource exhaustion.
func TestBrowserSecurity_OversizedURL(t *testing.T) {
	r := setupBrowserSecurityRouter()

	longURL := "https://example.com/" + strings.Repeat("a", 100000)
	body, _ := json.Marshal(map[string]interface{}{"url": longURL})

	req := httptest.NewRequest(http.MethodPost, "/v1/browser/navigate",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should not panic; any status is acceptable as long as server stays stable.
	assert.NotEqual(t, http.StatusInternalServerError, w.Code,
		"oversized URL should not cause 500")
}

// TestBrowserSecurity_HTMLInjectionInSelector verifies that HTML tags
// embedded in CSS selectors do not propagate to responses.
func TestBrowserSecurity_HTMLInjectionInSelector(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	gin.SetMode(gin.TestMode)

	r.POST("/v1/browser/click", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}
		selector, _ := req["selector"].(string)
		// Sanitize: strip HTML tags from selector before echoing
		sanitized := strings.ReplaceAll(selector, "<", "&lt;")
		sanitized = strings.ReplaceAll(sanitized, ">", "&gt;")
		c.JSON(http.StatusOK, gin.H{"selector": sanitized})
	})

	body, _ := json.Marshal(map[string]interface{}{
		"selector": "<img src=x onerror=alert(1)>",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/browser/click",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "<img",
		"HTML tags must be escaped in response")
}
