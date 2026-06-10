package honeycomb

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 STUB-BLUFF PIN GUARD — RED-on-broken-artifact + GREEN regression guard
// (§11.4.115 polarity switch / §11.4.135 standing guard)
//
// HISTORY: Honeycomb.query/analyze/trace/alert USED to return FABRICATED data
// WITHOUT any real Honeycomb API call (zero os/exec, zero HTTP): query → an
// empty result set, analyze → "AI analysis of <metric>" + invented insights,
// trace → hardcoded spans, alert → an "active" alert. Stub bluffs per BLUFF-001
// / CONST-035.
//
// FIX (D-17): Honeycomb data lives only behind its hosted HTTP API; with no real
// client wired these return an HONEST error (ErrAPINotWired) rather than
// fabricating results.
// ---------------------------------------------------------------------------

func TestD17_Honeycomb_NoFabricatedData(t *testing.T) {
	h := New()
	ctx := context.Background()

	cases := []struct {
		name   string
		invoke func() (interface{}, error)
	}{
		{"query", func() (interface{}, error) {
			return h.query(ctx, map[string]interface{}{"query": "SELECT count()"})
		}},
		{"analyze", func() (interface{}, error) {
			return h.analyze(ctx, map[string]interface{}{"metric": "latency"})
		}},
		{"trace", func() (interface{}, error) {
			return h.trace(ctx, map[string]interface{}{"trace_id": "abc123"})
		}},
		{"alert", func() (interface{}, error) {
			return h.alert(ctx, map[string]interface{}{"condition": "latency > 1000"})
		}},
	}
	for _, c := range cases {
		res, err := c.invoke()
		if err == nil {
			t.Fatalf("D17 REGRESSION: Honeycomb.%s returned success %v with no real API — must return an honest error (BLUFF-001 reintroduced?).", c.name, res)
		}
		if !errors.Is(err, ErrAPINotWired) {
			t.Fatalf("D17: Honeycomb.%s error should wrap ErrAPINotWired, got: %v", c.name, err)
		}
	}
}

// TestD17_Honeycomb_AnalyzeIsStubBluff — §11.4.115 RED-on-broken-artifact, RED_MODE=1.
func TestD17_Honeycomb_AnalyzeIsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"The standing GREEN guard is TestD17_Honeycomb_NoFabricatedData.")
	}
	h := New()
	res, err := h.analyze(context.Background(), map[string]interface{}{"metric": "latency"})
	if err != nil {
		return
	}
	m, _ := res.(map[string]interface{})
	if a, _ := m["analysis"].(string); strings.HasPrefix(a, "AI analysis of ") {
		t.Fatalf("D17 BLUFF PINNED: Honeycomb.analyze returned the fabricated literal %q without any real API call (BLUFF-001).", a)
	}
}
