package catalog

import (
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/verifier"
)

// --- StartupVerifier-backed VerifiedModelSource adapter (CONST-036/037) ---
//
// These guards cover the real wiring the catalog needs: the already-populated
// StartupVerifier (GetVerifiedProviders → UnifiedProvider.Models) feeding the
// catalog's VerifiedModelSource so GET /v1/catalog surfaces REAL Verified
// models instead of honest-empty.
//
// The staleness/filter logic lives in the pure helper
// verifiedModelsFromProviders(providers, now, ttl) so it is unit-testable with
// constructed *verifier.UnifiedProvider fixtures (no booted verifier / no real
// endpoints — that "real models flow from real providers" claim is
// integration-only, see TestStartupVerifierSource_IntegrationBoundary).

// fresh builds a VerifiedAt timestamp `age` before now.
func ago(now time.Time, age time.Duration) time.Time { return now.Add(-age) }

// TestStartupVerifierSource_NilIsHonestEmpty proves the adapter declines to
// fabricate a source when the verifier is absent: nil verifier → nil source →
// the catalog stays honestly empty of model entries (CONST-036/037, §11.4).
func TestStartupVerifierSource_NilIsHonestEmpty(t *testing.T) {
	if src := NewStartupVerifierSource(nil); src != nil {
		t.Fatalf("nil StartupVerifier must yield a nil VerifiedModelSource (honest-empty), got %T", src)
	}

	// Wired into the catalog with a nil source, NO model entry may appear.
	svc := New(Options{
		Providers: &fakeProviderSource{infos: []ProviderInfo{
			{Name: "anthropic", Enabled: true},
		}},
		Verified: NewStartupVerifierSource(nil), // nil verifier → nil source
	})
	for _, e := range svc.Build() {
		if e.Kind == KindModel {
			t.Fatalf("nil verifier but a model entry was fabricated: %+v", e)
		}
	}

	// An empty (un-run) real verifier likewise yields zero verified models —
	// never a fabricated working list.
	sv := verifier.NewStartupVerifier(verifier.DefaultStartupConfig(), nil)
	emptySrc := NewStartupVerifierSource(sv)
	if emptySrc == nil {
		t.Fatalf("a non-nil StartupVerifier must yield a non-nil source")
	}
	if got := emptySrc.VerifiedModels(); len(got) != 0 {
		t.Fatalf("un-run verifier must report zero verified models, got %d: %+v", len(got), got)
	}
}

// TestStartupVerifierSource_StalenessAndVerifiedGate is the core RED→GREEN +
// §1.1-paired guard for the adapter. It exercises the real
// verifiedModelsFromProviders staleness + Verified filter with a mix of:
//
//	verified + fresh   → INCLUDED  (within the CONST-037 24h window)
//	verified + stale   → EXCLUDED  (VerifiedAt older than 24h)
//	unverified         → EXCLUDED  (Verified==false, regardless of recency)
//
// Paired §1.1 mutation (constitution §1.1): dropping the 24h staleness gate
// leaks the stale model; dropping the Verified filter leaks the unverified
// model — either mutation makes THIS guard FAIL.
func TestStartupVerifierSource_StalenessAndVerifiedGate(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"
	now := time.Now()

	providers := []*verifier.UnifiedProvider{
		{
			Name:     "anthropic",
			Verified: true,
			Models: []verifier.UnifiedModel{
				// verified + fresh → must be included
				{ID: "claude-3-sonnet-20240229", Provider: "anthropic", Verified: true, VerifiedAt: ago(now, 1*time.Hour), Score: 9.1},
				// verified + STALE (>24h) → must be excluded (CONST-037)
				{ID: "claude-2-stale", Provider: "anthropic", Verified: true, VerifiedAt: ago(now, 30*time.Hour), Score: 8.0},
			},
		},
		{
			Name:     "openrouter",
			Verified: true,
			Models: []verifier.UnifiedModel{
				// verified + fresh, provider-namespaced id preserved verbatim
				{ID: "x-ai/grok-4", Provider: "openrouter", Verified: true, VerifiedAt: ago(now, 2*time.Hour), Score: 8.7},
				// UNVERIFIED (even though fresh) → must be excluded (CONST-036)
				{ID: "unverified-model", Provider: "openrouter", Verified: false, VerifiedAt: ago(now, 1*time.Minute), Score: 0},
			},
		},
	}

	got := verifiedModelsFromProviders(providers, now, verifiedModelTTL)

	byKey := func(provider, id string) (VerifiedModel, bool) {
		for _, m := range got {
			if m.Provider == provider && m.ModelID == id {
				return m, true
			}
		}
		return VerifiedModel{}, false
	}

	_, freshOK := byKey("anthropic", "claude-3-sonnet-20240229")
	_, grokOK := byKey("openrouter", "x-ai/grok-4")
	_, staleLeaked := byKey("anthropic", "claude-2-stale")
	_, unverifiedLeaked := byKey("openrouter", "unverified-model")

	if redMode {
		// Pre-fix artifact: no StartupVerifier-backed adapter existed, so the
		// fresh verified models never reached the catalog (honest-empty).
		// The RED assertion is that the wiring is ABSENT.
		if freshOK && grokOK {
			t.Fatalf("RED_MODE=1: expected fresh verified models to be ABSENT from the catalog on the pre-fix artifact, but they resolved: %+v", got)
		}
		t.Logf("RED_MODE=1 reproduced: fresh verified models not surfaced (fresh=%v grok=%v)", freshOK, grokOK)
		return
	}

	// GREEN guard.
	if !freshOK {
		t.Fatalf("verified+fresh anthropic/claude-3-sonnet-20240229 must be included, got: %+v", got)
	}
	if !grokOK {
		t.Fatalf("verified+fresh openrouter/x-ai/grok-4 must be included, got: %+v", got)
	}
	if staleLeaked {
		t.Fatalf("stale (>24h) verified model leaked past the CONST-037 staleness gate: %+v", got)
	}
	if unverifiedLeaked {
		t.Fatalf("unverified model leaked past the CONST-036 Verified filter: %+v", got)
	}
	if len(got) != 2 {
		t.Fatalf("expected exactly 2 surfaced models (the 2 verified+fresh), got %d: %+v", len(got), got)
	}

	// End-to-end through the catalog: the surfaced verified models become
	// <provider>/<model_id> entries; stale/unverified do NOT.
	svc := New(Options{
		Providers: &fakeProviderSource{infos: []ProviderInfo{
			{Name: "anthropic", Enabled: true},
			{Name: "openrouter", Enabled: true},
		}},
		Verified: &fakeVerifiedSource{models: got},
	})
	entries := svc.Build()
	if _, ok := has(entries, "anthropic/claude-3-sonnet-20240229"); !ok {
		t.Fatalf("fresh verified model did not surface as a catalog entry")
	}
	if _, ok := has(entries, "openrouter/x-ai/grok-4"); !ok {
		t.Fatalf("provider-namespaced verified id not surfaced as a catalog entry")
	}
	if _, ok := has(entries, "anthropic/claude-2-stale"); ok {
		t.Fatalf("stale model leaked into the catalog (anti-bluff violation)")
	}
	if _, ok := has(entries, "openrouter/unverified-model"); ok {
		t.Fatalf("unverified model leaked into the catalog (anti-bluff violation)")
	}
}

// TestStartupVerifierSource_ZeroVerifiedAtIsStale proves a verified model with
// a zero VerifiedAt (never stamped) is treated as STALE — it cannot prove it
// was verified within the CONST-037 24h window, so it is excluded rather than
// trusted by default (§11.4.6 no-guessing).
func TestStartupVerifierSource_ZeroVerifiedAtIsStale(t *testing.T) {
	now := time.Now()
	providers := []*verifier.UnifiedProvider{
		{
			Name:     "anthropic",
			Verified: true,
			Models: []verifier.UnifiedModel{
				{ID: "no-timestamp", Provider: "anthropic", Verified: true, VerifiedAt: time.Time{}, Score: 9.0},
			},
		},
	}
	if got := verifiedModelsFromProviders(providers, now, verifiedModelTTL); len(got) != 0 {
		t.Fatalf("verified model with zero VerifiedAt must be excluded (cannot prove 24h freshness), got: %+v", got)
	}
}
