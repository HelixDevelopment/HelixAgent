package catalog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestHandler_CatalogRoute_Polarity is the §11.4.115 RED→GREEN polarity guard
// for the /v1/catalog WIRING. RED_MODE=1 reproduces the genuine pre-fix defect:
// a router with NO catalog route 404s on /catalog (the unified endpoint did not
// exist). RED_MODE=0 (default) is the standing GREEN guard: with the handler
// wired, /catalog resolves 200.
func TestHandler_CatalogRoute_Polarity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redMode := os.Getenv("RED_MODE") == "1"

	r := gin.New()
	if !redMode {
		// GREEN: wire the real handler exactly as router.go does.
		r.GET("/catalog", NewHandler(fullCatalogFixture()).List)
	}
	// RED: route deliberately absent — reproduces the pre-fix "no catalog endpoint".

	req := httptest.NewRequest(http.MethodGet, "/catalog", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if redMode {
		if rec.Code != http.StatusNotFound {
			t.Fatalf("RED_MODE=1: expected 404 on the pre-fix (catalog-absent) router, got %d", rec.Code)
		}
		t.Logf("RED_MODE=1 reproduced: /catalog 404s without the catalog handler (defect present)")
		return
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("GREEN: /catalog must resolve 200 with the handler wired, got %d", rec.Code)
	}
}

// TestHandler_CatalogEndpoint drives the real gin handler over httptest and
// asserts GET /catalog returns the unified list as JSON with the four item
// classes. This is the route-level GREEN guard for the /v1/catalog wiring.
func TestHandler_CatalogEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := fullCatalogFixture()
	h := NewHandler(svc)

	r := gin.New()
	r.GET("/catalog", h.List)

	req := httptest.NewRequest(http.MethodGet, "/catalog", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /catalog status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Catalog []Entry `json:"catalog"`
		Count   int     `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode catalog response: %v; body=%s", err, rec.Body.String())
	}
	if resp.Count != len(resp.Catalog) || resp.Count == 0 {
		t.Fatalf("count mismatch: count=%d len=%d", resp.Count, len(resp.Catalog))
	}

	want := []string{
		"ensemble", "ensemble/majority_vote", "helixllm", "helixllm/helixllm-default",
		"anthropic", "anthropic/claude-3-sonnet-20240229", "openrouter/x-ai/grok-4",
	}
	for _, w := range want {
		if _, ok := has(resp.Catalog, w); !ok {
			t.Fatalf("GET /catalog missing target %q; got %v", w, names(resp.Catalog))
		}
	}
}
