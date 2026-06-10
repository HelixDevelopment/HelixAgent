package catalog

import (
	"os"
	"sort"
	"testing"
)

// --- test doubles (unit-test-only fakes, permitted by CONST-050(A)) ---

type fakeProviderSource struct{ infos []ProviderInfo }

func (f *fakeProviderSource) Providers() []ProviderInfo { return f.infos }

type fakeVerifiedSource struct{ models []VerifiedModel }

func (f *fakeVerifiedSource) VerifiedModels() []VerifiedModel { return f.models }

func names(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

func has(entries []Entry, name string) (Entry, bool) {
	for _, e := range entries {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// fullCatalogFixture returns a CatalogService wired with ensemble presets,
// HelixLLM enabled, three providers, and a mix of verified / unverified
// models — the realistic join the SP2 catalog must produce.
func fullCatalogFixture() *CatalogService {
	return New(Options{
		Providers: &fakeProviderSource{infos: []ProviderInfo{
			{Name: "anthropic", Enabled: true, SupportedModels: []string{"claude-3-sonnet-20240229"}},
			{Name: "openrouter", Enabled: true},
			{Name: "deepseek", Enabled: false},
		}},
		Verified: &fakeVerifiedSource{models: []VerifiedModel{
			{Provider: "anthropic", ModelID: "claude-3-sonnet-20240229", Verified: true, OverallScore: 9.1},
			{Provider: "openrouter", ModelID: "x-ai/grok-4", Verified: true, OverallScore: 8.7},
			{Provider: "deepseek", ModelID: "deepseek-coder", Verified: false, OverallScore: 0}, // NOT working → excluded
		}},
		EnsemblePresets: []string{"confidence_weighted", "majority_vote", "quality_weighted"},
		HelixLLMEnabled: true,
		HelixLLMModels:  []string{"helixllm-default"},
	})
}

// TestCatalog_UnifiedList is the core RED→GREEN guard for SP2 P2.1/P2.2.
//
// It asserts the CatalogService returns ensemble + helixllm + per-provider +
// per-VERIFIED-model entries as ONE uniformly-named list. With RED_MODE=1 it
// proves the historical defect (no catalog layer existed) WOULD fail; with
// RED_MODE=0 (default) it is the standing regression guard (§11.4.135).
func TestCatalog_UnifiedList(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	entries := fullCatalogFixture().Build()

	// The four item classes must ALL be present as uniformly-named targets.
	want := []string{
		// (a) ensemble aggregate + presets
		"ensemble",
		"ensemble/confidence_weighted",
		"ensemble/majority_vote",
		"ensemble/quality_weighted",
		// (b) helixllm promoted to first-class root + model
		"helixllm",
		"helixllm/helixllm-default",
		// (c) per-provider
		"anthropic",
		"openrouter",
		"deepseek",
		// (d) per-VERIFIED-model (note: provider-namespaced ids preserved)
		"anthropic/claude-3-sonnet-20240229",
		"openrouter/x-ai/grok-4",
	}

	missing := make([]string, 0)
	for _, w := range want {
		if _, ok := has(entries, w); !ok {
			missing = append(missing, w)
		}
	}

	if redMode {
		// On the pre-catalog artifact NONE of these targets exist as a single
		// joined list; the RED assertion is that the catalog is missing them.
		if len(missing) == 0 {
			t.Fatalf("RED_MODE=1: expected the unified catalog to be ABSENT/incomplete on the pre-fix artifact, but every target resolved: %v", names(entries))
		}
		t.Logf("RED_MODE=1 reproduced: %d/%d unified targets missing on pre-fix artifact: %v", len(missing), len(want), missing)
		return
	}

	// GREEN guard.
	if len(missing) > 0 {
		t.Fatalf("unified catalog missing required targets %v\nfull catalog: %v", missing, names(entries))
	}

	// Unverified model must NOT appear (no fabricated "working" entry).
	if _, ok := has(entries, "deepseek/deepseek-coder"); ok {
		t.Fatalf("unverified model deepseek/deepseek-coder leaked into the catalog (anti-bluff violation)")
	}

	// The "ensemble" aggregate must sort first (deterministic ordering).
	if entries[0].Name != "ensemble" {
		t.Fatalf("expected aggregate 'ensemble' to sort first, got %q", entries[0].Name)
	}

	// Kind/Verified correctness on representative entries.
	if e, _ := has(entries, "anthropic/claude-3-sonnet-20240229"); !e.Verified || e.Kind != KindModel || e.Provider != "anthropic" || e.OverallScore != 9.1 {
		t.Fatalf("verified model entry wrong: %+v", e)
	}
	if e, _ := has(entries, "anthropic"); e.Kind != KindProvider || !e.Enabled {
		t.Fatalf("provider entry wrong: %+v", e)
	}
	if e, _ := has(entries, "deepseek"); e.Enabled {
		t.Fatalf("disabled provider deepseek must report Enabled=false: %+v", e)
	}
	if e, _ := has(entries, "ensemble/majority_vote"); e.Kind != KindEnsemble {
		t.Fatalf("ensemble preset entry wrong: %+v", e)
	}
}

// TestCatalog_HonestEmptyWhenVerifierDisabled proves the catalog never
// fabricates a "working" model list: with a nil verifier source, NO model
// entries are emitted (only ensemble + helixllm[/model] + providers).
func TestCatalog_HonestEmptyWhenVerifierDisabled(t *testing.T) {
	svc := New(Options{
		Providers: &fakeProviderSource{infos: []ProviderInfo{
			{Name: "anthropic", Enabled: true, SupportedModels: []string{"claude-3-sonnet-20240229"}},
		}},
		Verified:        nil, // verifier disabled / not wired
		EnsemblePresets: []string{"confidence_weighted"},
		HelixLLMEnabled: false,
	})
	entries := svc.Build()

	for _, e := range entries {
		if e.Kind == KindModel {
			t.Fatalf("verifier disabled but a model entry was fabricated: %+v", e)
		}
	}
	// helixllm must be absent when disabled.
	if _, ok := has(entries, "helixllm"); ok {
		t.Fatalf("helixllm must be absent when HelixLLMEnabled=false")
	}
	// provider + ensemble must still be present.
	if _, ok := has(entries, "anthropic"); !ok {
		t.Fatalf("provider entry missing in honest-empty mode")
	}
	if _, ok := has(entries, "ensemble"); !ok {
		t.Fatalf("ensemble entry missing in honest-empty mode")
	}
}

// TestCatalog_NamingGrammar is the §11.4.137-style truth-table for the
// uniform naming scheme. The paired §1.1 mutation: if Build() ever drops the
// aggregate ensemble entry (or stops verifying models), this guard FAILs.
func TestCatalog_NamingGrammar(t *testing.T) {
	entries := fullCatalogFixture().Build()
	got := names(entries)
	sort.Strings(got)

	// Every name must be uniformly lowercase (the existing convention).
	for _, n := range got {
		if n != normalize(n) {
			t.Fatalf("catalog name %q is not uniformly lowercase (naming-grammar violation)", n)
		}
	}

	// Provider-namespaced ids must be preserved verbatim (not flattened).
	if _, ok := has(entries, "openrouter/x-ai/grok-4"); !ok {
		t.Fatalf("provider-namespaced verified id openrouter/x-ai/grok-4 was not preserved")
	}
}
