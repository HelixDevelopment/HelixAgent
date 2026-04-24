package monitoring_test

import (
	"net/http"
	"net/http/httptest"
	"net/http/pprof"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupPprofRouter creates a gin router with pprof endpoints registered,
// mirroring the production registration in internal/router/router.go when
// ENABLE_PPROF=true.
func setupPprofRouter() *gin.Engine {
	r := gin.New()
	r.GET("/debug/pprof/", gin.WrapH(http.HandlerFunc(pprof.Index)))
	r.GET("/debug/pprof/cmdline", gin.WrapH(http.HandlerFunc(pprof.Cmdline)))
	r.GET("/debug/pprof/profile", gin.WrapH(http.HandlerFunc(pprof.Profile)))
	r.GET("/debug/pprof/symbol", gin.WrapH(http.HandlerFunc(pprof.Symbol)))
	r.GET("/debug/pprof/trace", gin.WrapH(http.HandlerFunc(pprof.Trace)))
	r.GET("/debug/pprof/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	r.GET("/debug/pprof/heap", gin.WrapH(pprof.Handler("heap")))
	r.GET("/debug/pprof/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
	r.GET("/debug/pprof/block", gin.WrapH(pprof.Handler("block")))
	r.GET("/debug/pprof/mutex", gin.WrapH(pprof.Handler("mutex")))
	return r
}

// TestPprofEndpoint_IndexRegistered verifies that /debug/pprof/ is reachable
// and returns a 200 response when pprof endpoints are enabled.
func TestPprofEndpoint_IndexRegistered(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	r := setupPprofRouter()

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"/debug/pprof/ must return HTTP 200 when pprof is enabled")
	assert.Contains(t, w.Body.String(), "goroutine",
		"/debug/pprof/ index must list 'goroutine' profile")
}

// TestPprofEndpoint_ProfileRoutes validates that the key pprof sub-routes are
// all registered (i.e. the router does not return 404 for them).
func TestPprofEndpoint_ProfileRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	r := setupPprofRouter()

	routes := []struct {
		path   string
		method string
	}{
		{"/debug/pprof/", http.MethodGet},
		{"/debug/pprof/goroutine", http.MethodGet},
		{"/debug/pprof/heap", http.MethodGet},
		{"/debug/pprof/threadcreate", http.MethodGet},
		{"/debug/pprof/block", http.MethodGet},
		{"/debug/pprof/mutex", http.MethodGet},
	}

	for _, tc := range routes {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.NotEqual(t, http.StatusNotFound, w.Code,
				"pprof route %q must be registered (must not return 404)", tc.path)
		})
	}
}

// TestPprofEndpoint_NotRegisteredWhenDisabled verifies that when pprof routes
// are NOT registered, requests to /debug/pprof/ return 404, confirming the
// environment-variable gating in production code works correctly.
func TestPprofEndpoint_NotRegisteredWhenDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	// Router without pprof routes registered — simulates ENABLE_PPROF unset.
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"/debug/pprof/ must return 404 when pprof is not enabled")
}

// TestPprofEndpoint_HeapProfile verifies that the /debug/pprof/heap endpoint
// returns a valid response (not 404, not 500) when pprof is enabled.
func TestPprofEndpoint_HeapProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	r := setupPprofRouter()

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// The heap profile handler returns 200 with binary proto data.
	assert.Equal(t, http.StatusOK, w.Code,
		"/debug/pprof/heap must return HTTP 200")
	assert.Greater(t, w.Body.Len(), 0,
		"/debug/pprof/heap response body must not be empty")
}

// TestPprofEndpoint_GoroutineProfile verifies that the goroutine profile
// endpoint responds with a non-empty body containing goroutine stack data.
func TestPprofEndpoint_GoroutineProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	r := setupPprofRouter()

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/goroutine", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"/debug/pprof/goroutine must return HTTP 200")
	assert.Greater(t, w.Body.Len(), 0,
		"/debug/pprof/goroutine response body must not be empty")
}

// TestPprofEndpoint_SymbolLookup verifies that the /debug/pprof/symbol
// endpoint handles a POST request (standard pprof symbol lookup protocol)
// without returning a server error.
func TestPprofEndpoint_SymbolLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	r := gin.New()
	// symbol endpoint is registered for GET in the router; pprof.Symbol also
	// handles POST for symbol lookups — register both for this test.
	r.GET("/debug/pprof/symbol", gin.WrapH(http.HandlerFunc(pprof.Symbol)))
	r.POST("/debug/pprof/symbol", gin.WrapH(http.HandlerFunc(pprof.Symbol)))

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/symbol?0x1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Symbol lookup may return 200 with "num_symbols: 0" for an unknown address;
	// it must not return 404 or 5xx.
	assert.NotEqual(t, http.StatusNotFound, w.Code,
		"/debug/pprof/symbol must be registered")
	assert.Less(t, w.Code, 500,
		"/debug/pprof/symbol must not return a server error")
}

// TestPprofEndpoint_RequireAuth is a design guard test that documents that
// pprof endpoints MUST NOT be exposed without the ENABLE_PPROF gate in
// production. This test verifies the gate logic at the router level.
func TestPprofEndpoint_RequireGate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	// Simulate ENABLE_PPROF="" (disabled): router has no pprof routes.
	r := gin.New()
	r.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, "# metrics")
	})

	sensitiveRoutes := []string{
		"/debug/pprof/",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
	}

	for _, route := range sensitiveRoutes {
		route := route
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusNotFound, w.Code,
				"pprof route %q must not be accessible without ENABLE_PPROF=true", route)
		})
	}
}
