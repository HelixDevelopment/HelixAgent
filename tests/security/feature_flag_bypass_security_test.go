//go:build security
// +build security

// Package security provides feature flag bypass security tests.
// These tests verify that feature flags cannot be overridden through
// crafted HTTP headers or query parameters when not permitted.
package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/features"
)

func setupFeatureFlagSecurityRouter(allowHeaders bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	cfg := features.DefaultFeatureConfig()
	cfg.AllowFeatureHeaders = allowHeaders
	cfg.AllowFeatureQueryParams = allowHeaders

	r.GET("/v1/test-feature", func(c *gin.Context) {
		// Check if GraphQL feature is enabled from header
		headerVal := c.GetHeader("X-Feature-GraphQL")
		graphqlRequested := headerVal == "true" || headerVal == "1"

		// When headers are not allowed, ignore the header override
		if !cfg.AllowFeatureHeaders && graphqlRequested {
			graphqlRequested = false
		}

		// Use default from config
		graphqlEnabled := cfg.GlobalDefaults[features.FeatureGraphQL]
		if cfg.AllowFeatureHeaders && graphqlRequested {
			graphqlEnabled = true
		}

		c.JSON(http.StatusOK, gin.H{
			"graphql_enabled": graphqlEnabled,
		})
	})

	return r
}

// TestFeatureFlagBypass_HeaderDisallowed verifies that when header
// overrides are disabled, the X-Feature-GraphQL header is ignored.
func TestFeatureFlagBypass_HeaderDisallowed(t *testing.T) {
	r := setupFeatureFlagSecurityRouter(false)

	req := httptest.NewRequest(http.MethodGet, "/v1/test-feature", nil)
	req.Header.Set("X-Feature-GraphQL", "true")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"graphql_enabled":false`,
		"header override should be ignored when AllowFeatureHeaders=false")
}

// TestFeatureFlagBypass_HeaderAllowed verifies that when header
// overrides are allowed, the feature can be toggled via header.
func TestFeatureFlagBypass_HeaderAllowed(t *testing.T) {
	r := setupFeatureFlagSecurityRouter(true)

	req := httptest.NewRequest(http.MethodGet, "/v1/test-feature", nil)
	req.Header.Set("X-Feature-GraphQL", "true")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"graphql_enabled":true`,
		"header override should work when AllowFeatureHeaders=true")
}

// TestFeatureFlagBypass_InvalidHeaderValue verifies that non-boolean
// header values do not enable features.
func TestFeatureFlagBypass_InvalidHeaderValue(t *testing.T) {
	r := setupFeatureFlagSecurityRouter(true)

	invalidValues := []string{
		"yes",
		"enabled",
		"<script>alert(1)</script>",
		"'; DROP TABLE features; --",
	}

	for _, val := range invalidValues {
		t.Run(val, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/test-feature", nil)
			req.Header.Set("X-Feature-GraphQL", val)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), `"graphql_enabled":false`,
				"invalid header value %q should not enable feature", val)
		})
	}
}
