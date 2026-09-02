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

// TestOptionsFromModels_CarriesWithheldReason proves the reason survives the
// wire, still distinguishable from the other two.
func TestOptionsFromModels_CarriesWithheldReason(t *testing.T) {
	resp := decodeModels(t, `{
	  "object": "list",
	  "data": [
	    {"id":"helixllm-a-000000000001","host":"h1","availability":"withheld","withheld_reason":"insufficient_resources"},
	    {"id":"helixllm-b-000000000002","host":"h1","availability":"withheld","withheld_reason":"unsupported_configuration"},
	    {"id":"helixllm-c-000000000003","host":"h1","availability":"withheld","withheld_reason":"excluded_by_usage_terms"}
	  ]
	}`)

	opts := OptionsFromModels(resp)
	if len(opts) != 3 {
		t.Fatalf("got %d options, want 3", len(opts))
	}
	seen := map[WithheldReason]bool{}
	for _, o := range opts {
		if o.Availability != AvailabilityWithheld {
			t.Fatalf("option %q: Availability = %q, want withheld", o.ID, o.Availability)
		}
		if !o.WithheldReason.Known() {
			t.Fatalf("option %q: reason %q was not carried as one of the three recorded reasons", o.ID, o.WithheldReason)
		}
		seen[o.WithheldReason] = true
	}
	if len(seen) != 3 {
		t.Fatalf("the three reasons collapsed to %d on the way in: %v", len(seen), seen)
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
