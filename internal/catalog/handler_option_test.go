package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dev.helix.agent/internal/adapters/helixllm"
	"github.com/gin-gonic/gin"
)

// serveCatalog drives the real gin handler and returns the decoded entries,
// so these assertions are made against the JSON a final consumer actually
// receives rather than the Go struct behind it (§11.4.108 — a field that never
// reaches the artifact has not been propagated).
func serveCatalog(t *testing.T, svc *CatalogService) ([]map[string]any, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/catalog", NewHandler(svc).List)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/catalog", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /catalog = %d, want 200", rec.Code)
	}

	var body struct {
		Catalog []map[string]any `json:"catalog"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the catalog response failed: %v\nbody: %s", err, rec.Body.String())
	}
	return body.Catalog, rec.Body.String()
}

func entryByName(entries []map[string]any, name string) (map[string]any, bool) {
	for _, e := range entries {
		if e["name"] == name {
			return e, true
		}
	}
	return nil, false
}

// TestCatalogEndpoint_PublishesOptionFields proves the propagated fields cross
// the HTTP boundary to a final consumer: identity, host and availability all
// appear in the served JSON.
func TestCatalogEndpoint_PublishesOptionFields(t *testing.T) {
	svc := New(Options{
		Providers:       &fakeProviderSource{},
		HelixLLMEnabled: true,
		HelixLLM: &fakeHelixLLMSource{options: []HelixLLMOption{
			servingOption(),
			withheldOption("helixllm-gpu01-huge-b2c3d4e5f6a1", ReasonInsufficientResources),
		}},
	})

	entries, raw := serveCatalog(t, svc)

	served, ok := entryByName(entries, "helixllm/"+servingOption().ID)
	if !ok {
		t.Fatalf("the served option is absent from the published catalog:\n%s", raw)
	}
	if served["model_identity"] != "helixllm/gpu-01/llama3:8b" {
		t.Errorf("model_identity = %v, want the identity value", served["model_identity"])
	}
	if served["host"] != "gpu-01" {
		t.Errorf("host = %v, want gpu-01 (FR-023)", served["host"])
	}
	if served["availability"] != string(AvailabilityServing) {
		t.Errorf("availability = %v, want %q", served["availability"], AvailabilityServing)
	}
	if served["enabled"] != true {
		t.Errorf("enabled = %v, want true for a served option", served["enabled"])
	}
	if _, present := served["withheld_reason"]; present {
		t.Errorf("a served option published a withheld_reason: %v", served["withheld_reason"])
	}
}

// TestCatalogEndpoint_WithheldOptionIsNotPublishedAsAvailable is the consumer's
// view of contract invariant 5: the entry is published so the tool can explain
// itself, and every field that could read as "usable" says otherwise.
func TestCatalogEndpoint_WithheldOptionIsNotPublishedAsAvailable(t *testing.T) {
	opt := withheldOption("helixllm-gpu01-huge-b2c3d4e5f6a1", ReasonExcludedByUsageTerms)
	svc := New(Options{
		Providers:       &fakeProviderSource{},
		HelixLLMEnabled: true,
		HelixLLM:        &fakeHelixLLMSource{options: []HelixLLMOption{opt}},
	})

	entries, raw := serveCatalog(t, svc)

	e, ok := entryByName(entries, "helixllm/"+opt.ID)
	if !ok {
		t.Fatalf("the withheld option is absent, so a consumer cannot report why it is unusable:\n%s", raw)
	}
	if e["availability"] != string(AvailabilityWithheld) {
		t.Fatalf("availability = %v, want %q", e["availability"], AvailabilityWithheld)
	}
	if e["enabled"] != false {
		t.Fatalf("a withheld option published enabled=%v — presented as available", e["enabled"])
	}
	if e["verified"] != false {
		t.Fatalf("a withheld option published verified=%v", e["verified"])
	}
	if e["withheld_reason"] != string(ReasonExcludedByUsageTerms) {
		t.Fatalf("withheld_reason = %v, want %q — the remedy depends on which of the three it is",
			e["withheld_reason"], ReasonExcludedByUsageTerms)
	}
}

// TestCatalogEndpoint_LegacyServingLayerPublishesNothingUsable is the honest
// end-to-end state of a deployment whose serving layer still publishes only the
// OpenAI-compatible model shape: the models are listed, and not one of them
// claims to be running.
func TestCatalogEndpoint_LegacyServingLayerPublishesNothingUsable(t *testing.T) {
	resp := &helixllm.ModelsResponse{
		Object: "list",
		Data: []helixllm.ModelInfo{
			{ID: "helixllm-default", Object: "model", OwnedBy: "helixllm"},
		},
	}
	svc := New(Options{
		Providers:       &fakeProviderSource{},
		HelixLLMEnabled: true,
		HelixLLM: NewHelixLLMSource(context.Background(),
			func(context.Context) (*helixllm.ModelsResponse, error) { return resp, nil }),
	})

	entries, raw := serveCatalog(t, svc)

	e, ok := entryByName(entries, "helixllm/helixllm-default")
	if !ok {
		t.Fatalf("the listed model is absent:\n%s", raw)
	}
	if e["enabled"] != false {
		t.Fatalf("a serving layer that reported no availability produced enabled=%v", e["enabled"])
	}
	if _, present := e["availability"]; present {
		t.Fatalf("an unreported availability was published as %v instead of being omitted", e["availability"])
	}
	if strings.Contains(raw, "model_identity") {
		t.Fatalf("an identity was published for a model that stated none:\n%s", raw)
	}
}
