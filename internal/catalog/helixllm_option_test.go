package catalog

import (
	"os"
	"testing"
)

// This file is the T045 propagation guard: the model option set HelixLLM
// serves must reach HelixAgent's upper layers and final consumers WITHOUT
// losing any of the four invariants that travel with the data.
//
// Polarity switch per §11.4.115: RED_MODE=1 asserts the defect is present on
// the pre-fix artifact (no propagation layer existed — a hardcoded
// DefaultHelixLLMModels() string list with no identity, no host and no
// availability); RED_MODE=0 (the default) is the standing regression guard.

// fakeHelixLLMSource is a unit-test-only double (CONST-050(A)).
type fakeHelixLLMSource struct{ options []HelixLLMOption }

func (f *fakeHelixLLMSource) HelixLLMOptions() []HelixLLMOption { return f.options }

// servingOption is a well-formed, actually-served option as the serving layer
// reports it: a derived charset-safe identifier, the human-readable identity
// VALUE, and the host serving it (FR-023).
func servingOption() HelixLLMOption {
	return HelixLLMOption{
		ID:            "helixllm-gpu01-llama3-8b-a1b2c3d4e5f6",
		ModelIdentity: "helixllm/gpu-01/llama3:8b",
		Host:          "gpu-01",
		OwnedBy:       "helixllm",
		Availability:  AvailabilityServing,
	}
}

func withheldOption(id string, reason WithheldReason) HelixLLMOption {
	return HelixLLMOption{
		ID:             id,
		ModelIdentity:  "helixllm/gpu-01/" + id,
		Host:           "gpu-01",
		OwnedBy:        "helixllm",
		Availability:   AvailabilityWithheld,
		WithheldReason: reason,
	}
}

func withOptions(opts ...HelixLLMOption) *CatalogService {
	return New(Options{
		Providers:       &fakeProviderSource{},
		HelixLLMEnabled: true,
		HelixLLM:        &fakeHelixLLMSource{options: opts},
	})
}

// TestHelixLLMOptions_PropagateIdentityHostAndAvailability is the core T045
// guard: every field the option set carries must survive the join into the
// catalog a consumer reads. FR-016 (the active configuration reaches consuming
// layers), FR-023 (every model labelled with its serving host).
func TestHelixLLMOptions_PropagateIdentityHostAndAvailability(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	opt := servingOption()
	entries := withOptions(opt).Build()

	e, ok := has(entries, NameHelixLLM+"/"+opt.ID)
	if redMode {
		if ok {
			t.Fatalf("RED_MODE=1: expected NO propagated option entry on the pre-fix artifact (it published a hardcoded model-id string list), but found %+v", e)
		}
		t.Logf("RED_MODE=1 reproduced: the option set does not reach the catalog; entries are %v", names(entries))
		return
	}

	if !ok {
		t.Fatalf("propagated option is absent from the catalog; entries: %v", names(entries))
	}
	if e.Kind != KindModel {
		t.Errorf("Kind = %q, want %q", e.Kind, KindModel)
	}
	if e.Provider != NameHelixLLM {
		t.Errorf("Provider = %q, want %q", e.Provider, NameHelixLLM)
	}
	// The identifier a consumer uses is the one the serving layer derived.
	if e.Model != opt.ID {
		t.Errorf("Model (the consumer identifier) = %q, want the derived id %q", e.Model, opt.ID)
	}
	// The human-readable identity travels as a VALUE, in its own field.
	if e.ModelIdentity != opt.ModelIdentity {
		t.Errorf("ModelIdentity = %q, want %q", e.ModelIdentity, opt.ModelIdentity)
	}
	// FR-023: labelled with the host serving it.
	if e.Host != opt.Host {
		t.Errorf("Host = %q, want %q", e.Host, opt.Host)
	}
	if e.Availability != AvailabilityServing {
		t.Errorf("Availability = %q, want %q", e.Availability, AvailabilityServing)
	}
	if !e.Enabled {
		t.Errorf("a served option must report Enabled=true: %+v", e)
	}
	if e.WithheldReason != "" {
		t.Errorf("a served option must carry no withheld reason, got %q", e.WithheldReason)
	}
}

// TestHelixLLMOptions_UnavailableIsNeverPresentedAsAvailable is invariant 5 of
// contracts/model-listing.md and FR-019: a model that is not being served is
// listed (so the tool can show WHY) but never as usable.
func TestHelixLLMOptions_UnavailableIsNeverPresentedAsAvailable(t *testing.T) {
	opt := withheldOption("helixllm-gpu01-huge-b2c3d4e5f6a1", ReasonInsufficientResources)
	entries := withOptions(opt).Build()

	e, ok := has(entries, NameHelixLLM+"/"+opt.ID)
	if !ok {
		t.Fatalf("withheld option vanished from the catalog; a consumer cannot report why it is unusable. entries: %v", names(entries))
	}
	if e.Availability.Usable() {
		t.Fatalf("withheld option reports as usable: %+v", e)
	}
	if e.Enabled {
		t.Fatalf("withheld option reports Enabled=true — presented as available (anti-bluff violation): %+v", e)
	}
	if e.WithheldReason != ReasonInsufficientResources {
		t.Fatalf("WithheldReason = %q, want %q", e.WithheldReason, ReasonInsufficientResources)
	}
}

// TestHelixLLMOptions_WithheldReasonsStayDistinct is the load-bearing
// invariant. The three reasons have three different remedies; collapsing them
// into one generic unavailability destroys the only part of the answer the
// user can act on.
func TestHelixLLMOptions_WithheldReasonsStayDistinct(t *testing.T) {
	reasons := []WithheldReason{
		ReasonInsufficientResources,
		ReasonUnsupportedConfiguration,
		ReasonExcludedByUsageTerms,
	}

	opts := make([]HelixLLMOption, 0, len(reasons))
	for i, r := range reasons {
		opts = append(opts, withheldOption(
			"helixllm-gpu01-model"+string(rune('a'+i))+"-c3d4e5f6a1b2", r))
	}
	entries := withOptions(opts...).Build()

	seen := make(map[WithheldReason]int, len(reasons))
	for _, o := range opts {
		e, ok := has(entries, NameHelixLLM+"/"+o.ID)
		if !ok {
			t.Fatalf("withheld option %q missing from the catalog", o.ID)
		}
		if e.WithheldReason != o.WithheldReason {
			t.Fatalf("option %q: WithheldReason = %q, want %q — reasons were altered in transit",
				o.ID, e.WithheldReason, o.WithheldReason)
		}
		seen[e.WithheldReason]++
	}

	if len(seen) != len(reasons) {
		t.Fatalf("the three withheld reasons collapsed into %d distinct value(s): %v — each implies a different remedy and they are not interchangeable",
			len(seen), seen)
	}
	for _, r := range reasons {
		if !r.Known() {
			t.Fatalf("reason %q is not in the closed set", r)
		}
	}
}

// TestWithheldReason_ClosedSet proves the set is closed: a generic
// "unavailable" invented downstream is not a reason.
func TestWithheldReason_ClosedSet(t *testing.T) {
	for _, bad := range []WithheldReason{"", "unavailable", "unknown", "not_available", "error"} {
		if bad.Known() {
			t.Fatalf("%q must not be accepted as a withheld reason — the set is closed to exactly three", bad)
		}
	}
}

// TestHelixLLMOptions_UnreportedAvailabilityIsNotUsable covers the state the
// serving layer is in before it reports availability at all. Absence of a
// serving claim is not a serving claim (§11.4.6): it must never read as usable.
func TestHelixLLMOptions_UnreportedAvailabilityIsNotUsable(t *testing.T) {
	opt := servingOption()
	opt.Availability = AvailabilityUnreported

	entries := withOptions(opt).Build()
	e, ok := has(entries, NameHelixLLM+"/"+opt.ID)
	if !ok {
		t.Fatalf("option with unreported availability vanished; entries: %v", names(entries))
	}
	if e.Availability.Usable() {
		t.Fatalf("unreported availability read as usable: %+v", e)
	}
	if e.Enabled {
		t.Fatalf("unreported availability reported Enabled=true — a serving claim was fabricated: %+v", e)
	}
}

// TestHelixLLMOptions_UnsafeIdentifierIsDropped holds the shell-injection
// guard at the propagation boundary (FR-014a). The derived identifier is
// carried verbatim or the option is not carried at all — it is NEVER widened
// to fit, and the human-readable identity is NEVER substituted for it.
func TestHelixLLMOptions_UnsafeIdentifierIsDropped(t *testing.T) {
	unsafe := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"path separator", "helixllm/gpu-01/llama3"},
		{"variant separator", "helixllm-gpu01-llama3:8b"},
		{"command separator", "helixllm-gpu01;rm -rf /"},
		{"command substitution", "helixllm-$(id)"},
		{"whitespace", "helixllm gpu01"},
		{"leading digit", "0helixllm-gpu01-abc123def456"},
		{"dot", "helixllm.gpu01.llama3"},
	}

	for _, tc := range unsafe {
		t.Run(tc.name, func(t *testing.T) {
			opt := servingOption()
			opt.ID = tc.id

			entries := withOptions(opt).Build()
			for _, e := range entries {
				if e.Kind != KindModel {
					continue
				}
				t.Fatalf("option with unsafe identifier %q was carried into the catalog as %+v — a hostile id must be dropped, never carried and never replaced by the identity", tc.id, e)
			}
		})
	}
}

// TestHelixLLMOptions_IdentityIsNeverTheIdentifier proves the substitution the
// unsafe-id case must not silently perform: dropping an option is the correct
// outcome, using the `/`- and `:`-bearing identity as the identifier is not.
func TestHelixLLMOptions_IdentityIsNeverTheIdentifier(t *testing.T) {
	opt := servingOption()
	entries := withOptions(opt).Build()

	e, ok := has(entries, NameHelixLLM+"/"+opt.ID)
	if !ok {
		t.Fatalf("served option missing; entries: %v", names(entries))
	}
	if e.Model == e.ModelIdentity {
		t.Fatalf("the identity VALUE is being used as the consumer identifier: %+v", e)
	}
	if !identifierSafe(e.Model) {
		t.Fatalf("the carried identifier %q does not satisfy the consumer's charset as it stands", e.Model)
	}
}

// TestHelixLLMOptions_HonestEmptyWhenSourceAbsent proves the catalog never
// fabricates an option set: with no source wired, no option entries appear.
func TestHelixLLMOptions_HonestEmptyWhenSourceAbsent(t *testing.T) {
	svc := New(Options{
		Providers:       &fakeProviderSource{},
		HelixLLMEnabled: true,
		HelixLLM:        nil,
	})
	for _, e := range svc.Build() {
		if e.ModelIdentity != "" || e.Host != "" || e.Availability != AvailabilityUnreported {
			t.Fatalf("no option source was wired but an option-shaped entry was fabricated: %+v", e)
		}
	}
}

// TestHelixLLMOptions_SuppressedWhenHelixLLMDisabled proves options are not
// surfaced when HelixLLM itself is off (no source of truth to speak for them).
func TestHelixLLMOptions_SuppressedWhenHelixLLMDisabled(t *testing.T) {
	svc := New(Options{
		Providers:       &fakeProviderSource{},
		HelixLLMEnabled: false,
		HelixLLM:        &fakeHelixLLMSource{options: []HelixLLMOption{servingOption()}},
	})
	for _, e := range svc.Build() {
		if e.Provider == NameHelixLLM {
			t.Fatalf("HelixLLM is disabled but an option entry was surfaced: %+v", e)
		}
	}
}

// TestHelixLLMOptions_LegacyModelListStillWorks proves the pre-existing
// HelixLLMModels path is not removed (§11.4.122): when no option source is
// wired it behaves exactly as before, and it makes no availability claim it
// cannot support.
func TestHelixLLMOptions_LegacyModelListStillWorks(t *testing.T) {
	svc := New(Options{
		Providers:       &fakeProviderSource{},
		HelixLLMEnabled: true,
		HelixLLMModels:  DefaultHelixLLMModels(),
	})
	entries := svc.Build()

	e, ok := has(entries, NameHelixLLM+"/helixllm-default")
	if !ok {
		t.Fatalf("legacy HelixLLMModels entry disappeared; entries: %v", names(entries))
	}
	if e.Availability != AvailabilityUnreported {
		t.Fatalf("a bare model-id string carries no serving report, so Availability must be unreported, got %q", e.Availability)
	}
	if e.ModelIdentity != "" || e.Host != "" {
		t.Fatalf("a bare model-id string carries no identity and no host; they must not be invented: %+v", e)
	}
}

// TestHelixLLMOptions_TakePrecedenceOverLegacyList proves that once the real
// option set is available it replaces the hardcoded list rather than being
// merged with it — otherwise the fabricated entry would sit beside the real
// ones and be indistinguishable (CONST-036).
func TestHelixLLMOptions_TakePrecedenceOverLegacyList(t *testing.T) {
	opt := servingOption()
	svc := New(Options{
		Providers:       &fakeProviderSource{},
		HelixLLMEnabled: true,
		HelixLLMModels:  DefaultHelixLLMModels(),
		HelixLLM:        &fakeHelixLLMSource{options: []HelixLLMOption{opt}},
	})
	entries := svc.Build()

	if _, ok := has(entries, NameHelixLLM+"/helixllm-default"); ok {
		t.Fatalf("the hardcoded model list survived alongside the real option set; entries: %v", names(entries))
	}
	if _, ok := has(entries, NameHelixLLM+"/"+opt.ID); !ok {
		t.Fatalf("the real option is missing; entries: %v", names(entries))
	}
}
