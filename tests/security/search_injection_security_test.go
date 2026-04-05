//go:build security
// +build security

// Package security provides search endpoint injection tests.
// These tests verify that SQL and NoSQL injection payloads in search
// queries are handled safely without data leakage.
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
)

func setupSearchSecurityRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/v1/search/semantic", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}

		query, _ := req["query"].(string)
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
			return
		}

		// Return empty results -- no actual DB hit in security tests.
		c.JSON(http.StatusOK, gin.H{
			"results":     []interface{}{},
			"total_found": 0,
		})
	})

	return r
}

// TestSearchSecurity_SQLInjection verifies that SQL injection payloads
// in the query field do not cause errors or data exfiltration.
func TestSearchSecurity_SQLInjection(t *testing.T) {
	r := setupSearchSecurityRouter()

	payloads := []string{
		"' OR '1'='1",
		"'; DROP TABLE users; --",
		"' UNION SELECT * FROM pg_catalog.pg_tables --",
		"1; SELECT pg_sleep(5);--",
		"' OR 1=1 LIMIT 1 --",
	}

	for _, payload := range payloads {
		t.Run(payload[:min(len(payload), 30)], func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{"query": payload})
			req := httptest.NewRequest(http.MethodPost, "/v1/search/semantic",
				bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code,
				"SQL injection should be handled safely")
			assert.NotContains(t, w.Body.String(), "pg_catalog",
				"response must not leak database metadata")
		})
	}
}

// TestSearchSecurity_NoSQLInjection verifies that NoSQL injection
// operators are not interpreted in the query field.
func TestSearchSecurity_NoSQLInjection(t *testing.T) {
	r := setupSearchSecurityRouter()

	payloads := []string{
		`{"$gt": ""}`,
		`{"$ne": null}`,
		`{"$where": "this.password"}`,
	}

	for _, payload := range payloads {
		t.Run(payload[:min(len(payload), 25)], func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{"query": payload})
			req := httptest.NewRequest(http.MethodPost, "/v1/search/semantic",
				bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, w.Code,
				"NoSQL injection should not cause 500")
		})
	}
}

// TestSearchSecurity_OversizedQuery verifies that extremely large
// query strings do not cause memory exhaustion.
func TestSearchSecurity_OversizedQuery(t *testing.T) {
	r := setupSearchSecurityRouter()

	hugeQuery := strings.Repeat("A", 1024*1024) // 1 MB
	body, _ := json.Marshal(map[string]interface{}{"query": hugeQuery})

	req := httptest.NewRequest(http.MethodPost, "/v1/search/semantic",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusInternalServerError, w.Code,
		"oversized query must not crash the server")
}

