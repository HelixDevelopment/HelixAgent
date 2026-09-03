package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"dev.helix.agent/internal/adapters/helixllm"
)

// decodeModels parses a real /v1/models wire payload the way the adapter does,
// so these tests exercise the actual decoding path rather than a hand-built
// struct that could drift from the JSON tags (§11.4.108 — the wire is the
// layer that matters, not the source shape).
func decodeModels(t *testing.T, payload string) *helixllm.ModelsResponse {
	t.Helper()
	var resp helixllm.ModelsResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("decoding the /v1/models payload failed: %v", err)
	}
	return &resp
}

// TestOptionsFromModels_CarriesEveryContractField proves each field of the
// model-listing contract survives the wire → option translation.
func TestOptionsFromModels_CarriesEveryContractField(t *testing.T) {
	resp := decodeModels(t, `{
	  "object": "list",
	  "data": [{
	    "id": "helixllm-gpu01-llama3-8b-a1b2c3d4e5f6",
	    "object": "model",
	    "created": 1700000000,
	    "owned_by": "helixllm",
	    "model_identity": "helixllm/gpu-01/llama3:8b",
	    "host": "gpu-01",
	    "availability": "serving"
	  }]
	}`)

	opts := OptionsFromModels(resp)
	if len(opts) != 1 {
		t.Fatalf("got %d options, want 1", len(opts))
	}
	got := opts[0]
	want := HelixLLMOption{
		ID:            "helixllm-gpu01-llama3-8b-a1b2c3d4e5f6",
		ModelIdentity: "helixllm/gpu-01/llama3:8b",
		Host:          "gpu-01",
		OwnedBy:       "helixllm",
		Availability:  AvailabilityServing,
	}
	if got != want {
		t.Fatalf("option = %+v, want %+v", got, want)
	}
}

// TestOptionsFromModels_CarriesWithheldReason proves every recorded reason
// survives the wire, still distinguishable from the others.
//
// The set was three when this test was written and is five now: the serving
// layer began publishing withheld options and needed to say WHY it was not
// serving them, which the original three could not express — they are about
// what a host can RUN, not about whether the serving layer is UP. This case is
// the reason the two ends have to move together: an unrecognised reason is
// discarded by reasonFromWire, so had this side not been extended, the producer
// would have emitted `provider_unavailable` and the consumer would have carried
// a withheld option with no reason at all — the failure mode is silent.
func TestOptionsFromModels_CarriesWithheldReason(t *testing.T) {
	resp := decodeModels(t, `{
	  "object": "list",
	  "data": [
	    {"id":"helixllm-a-000000000001","host":"h1","availability":"withheld","withheld_reason":"insufficient_resources"},
	    {"id":"helixllm-b-000000000002","host":"h1","availability":"withheld","withheld_reason":"unsupported_configuration"},
	    {"id":"helixllm-c-000000000003","host":"h1","availability":"withheld","withheld_reason":"excluded_by_usage_terms"},
	    {"id":"helixllm-d-000000000004","host":"h1","availability":"withheld","withheld_reason":"provider_unavailable"},
	    {"id":"helixllm-e-000000000005","host":"h1","availability":"withheld","withheld_reason":"identifier_conflict"}
	  ]
	}`)

	opts := OptionsFromModels(resp)
	if len(opts) != 5 {
		t.Fatalf("got %d options, want 5", len(opts))
	}
	seen := map[WithheldReason]bool{}
	for _, o := range opts {
		if o.Availability != AvailabilityWithheld {
			t.Fatalf("option %q: Availability = %q, want withheld", o.ID, o.Availability)
		}
		if !o.WithheldReason.Known() {
			t.Fatalf("option %q: reason %q was not carried as one of the recorded reasons; "+
				"an unrecognised reason is dropped, leaving the user a withheld option with "+
				"nothing to act on", o.ID, o.WithheldReason)
		}
		if o.Availability.Usable() {
			t.Fatalf("option %q is withheld but reports itself usable", o.ID)
		}
		seen[o.WithheldReason] = true
	}
	if len(seen) != 5 {
		t.Fatalf("the five reasons collapsed to %d on the way in: %v", len(seen), seen)
	}
}

// TestWithheldReason_ClosedSetMatchesTheServingLayers is the cross-repo contract
// check.
//
// The producer and this validator are in different repositories, so nothing
// mechanical stops one from moving without the other — and the failure is
// asymmetric and silent in the direction that matters: a producer emitting a
// key this side does not admit loses the reason entirely, with the option still
// arriving as withheld, so no error is raised anywhere. This pins the set as a
// literal so that widening it on one side without the other is a visible edit
// here rather than a field that quietly stops arriving.
//
// The mirror of this list lives in the serving layer's own wire contract; the
// two are the same five keys, spelled identically.
func TestWithheldReason_ClosedSetMatchesTheServingLayers(t *testing.T) {
	// Exactly what the serving layer's /v1/models may put in withheld_reason.
	servingLayerKeys := []string{
		"insufficient_resources",
		"unsupported_configuration",
		"excluded_by_usage_terms",
		"provider_unavailable",
		"identifier_conflict",
	}
	for _, key := range servingLayerKeys {
		if !WithheldReason(key).Known() {
			t.Errorf("the serving layer may publish withheld_reason %q, and this consumer "+
				"does not admit it: reasonFromWire will discard it and the option arrives "+
				"withheld with no actionable reason", key)
		}
	}

	// And the set is genuinely closed the other way: an invented reason is not
	// admitted, or the validation would be decorative.
	for _, bogus := range []string{"", "unavailable", "provider-unavailable", "PROVIDER_UNAVAILABLE"} {
		if WithheldReason(bogus).Known() {
			t.Errorf("%q was admitted as a recorded reason; the set must stay closed, and "+
				"note that the hyphenated spelling is the serving layer's INTERNAL "+
				"vocabulary for a different field, not a wire key", bogus)
		}
	}
}

// TestOptionsFromModels_LegacyPayloadIsNotAvailable is the compatibility guard
// that matters most. Today's serving layer publishes the plain OpenAI model
// shape with no availability field at all. Decoding that MUST NOT produce an
// option that reads as served — an absent claim is not a claim (§11.4.6).
func TestOptionsFromModels_LegacyPayloadIsNotAvailable(t *testing.T) {
	resp := decodeModels(t, `{
	  "object": "list",
	  "data": [{"id":"helixllm-default","object":"model","created":0,"owned_by":"helixllm"}]
	}`)

	opts := OptionsFromModels(resp)
	if len(opts) != 1 {
		t.Fatalf("got %d options, want 1", len(opts))
	}
	if opts[0].Availability.Usable() {
		t.Fatalf("a payload with no availability field produced a usable option: %+v", opts[0])
	}
	if opts[0].ModelIdentity != "" || opts[0].Host != "" {
		t.Fatalf("a payload stating no identity and no host had them invented: %+v", opts[0])
	}
}

// TestOptionsFromModels_UnrecordedReasonIsNotCarried proves the closed set is
// enforced at the boundary: a reason the contract does not record is dropped
// rather than passed on as though it were actionable.
func TestOptionsFromModels_UnrecordedReasonIsNotCarried(t *testing.T) {
	resp := decodeModels(t, `{
	  "object":"list",
	  "data":[{"id":"helixllm-x-000000000001","availability":"withheld","withheld_reason":"unavailable"}]
	}`)

	opts := OptionsFromModels(resp)
	if len(opts) != 1 {
		t.Fatalf("got %d options, want 1", len(opts))
	}
	if opts[0].WithheldReason != "" {
		t.Fatalf("an unrecorded reason %q was carried through as if actionable", opts[0].WithheldReason)
	}
	if opts[0].Availability.Usable() {
		t.Fatalf("a withheld option became usable: %+v", opts[0])
	}
}

// TestOptionsFromModels_UnrecordedAvailabilityIsNotUsable proves an
// availability value outside the recorded set never reads as serving.
func TestOptionsFromModels_UnrecordedAvailabilityIsNotUsable(t *testing.T) {
	resp := decodeModels(t, `{
	  "object":"list",
	  "data":[{"id":"helixllm-y-000000000002","availability":"probably_fine"}]
	}`)

	opts := OptionsFromModels(resp)
	if len(opts) != 1 {
		t.Fatalf("got %d options, want 1", len(opts))
	}
	if opts[0].Availability.Usable() {
		t.Fatalf("an unrecorded availability value read as usable: %+v", opts[0])
	}
}

// TestOptionsFromModels_NilResponse proves the absent case is empty, not a panic.
func TestOptionsFromModels_NilResponse(t *testing.T) {
	if opts := OptionsFromModels(nil); len(opts) != 0 {
		t.Fatalf("nil response produced %d options", len(opts))
	}
}

// --- the live source ---------------------------------------------------------

// TestListerSource_FetchFailureIsHonestlyEmpty proves an unreachable serving
// layer yields NO options rather than a stale or invented set: a listing that
// could not be obtained is not evidence that anything is running.
func TestListerSource_FetchFailureIsHonestlyEmpty(t *testing.T) {
	src := NewHelixLLMSource(context.Background(),
		func(context.Context) (*helixllm.ModelsResponse, error) {
			return nil, errors.New("connection refused")
		})

	if opts := src.HelixLLMOptions(); len(opts) != 0 {
		t.Fatalf("an unreachable serving layer produced %d options: %+v", len(opts), opts)
	}
}

// TestListerSource_NilListerIsNilSource proves that wiring nothing yields a nil
// source, so the catalog takes its honest-empty path rather than emitting an
// empty option set that would supersede the legacy list.
func TestListerSource_NilListerIsNilSource(t *testing.T) {
	if src := NewHelixLLMSource(context.Background(), nil); src != nil {
		t.Fatalf("a nil lister produced a non-nil source: %#v", src)
	}
}

// TestListerSource_CarriesOptionsThrough proves the end-to-end path: a wire
// payload reaches the catalog as entries carrying identity, host and
// availability.
func TestListerSource_CarriesOptionsThrough(t *testing.T) {
	resp := decodeModels(t, `{
	  "object":"list",
	  "data":[{
	    "id":"helixllm-gpu01-llama3-8b-a1b2c3d4e5f6",
	    "owned_by":"helixllm",
	    "model_identity":"helixllm/gpu-01/llama3:8b",
	    "host":"gpu-01",
	    "availability":"serving"
	  }]
	}`)

	src := NewHelixLLMSource(context.Background(),
		func(context.Context) (*helixllm.ModelsResponse, error) { return resp, nil })

	entries := New(Options{
		Providers:       &fakeProviderSource{},
		HelixLLMEnabled: true,
		HelixLLM:        src,
	}).Build()

	e, ok := has(entries, NameHelixLLM+"/helixllm-gpu01-llama3-8b-a1b2c3d4e5f6")
	if !ok {
		t.Fatalf("the wire option did not reach the catalog; entries: %v", names(entries))
	}
	if e.ModelIdentity != "helixllm/gpu-01/llama3:8b" || e.Host != "gpu-01" || !e.Availability.Usable() {
		t.Fatalf("entry lost a contract field in transit: %+v", e)
	}
}
