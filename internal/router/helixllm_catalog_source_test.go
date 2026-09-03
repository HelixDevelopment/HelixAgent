package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.agent/internal/catalog"
	"github.com/sirupsen/logrus"
)

// These tests pin the failure policy of the catalog's live HelixLLM source.
// The property under test is NOT "the source works when HelixLLM is up" — it is
// that every way HelixLLM can be absent, slow, or broken produces ZERO options
// rather than a fabricated or remembered one (FR-019, CONST-036).

func TestNewHelixLLMCatalogSource_DisabledYieldsNoSource(t *testing.T) {
	// Off means off: nil, so catalog.Options emits no HelixLLM model entries at
	// all rather than a placeholder id.
	if src := newHelixLLMCatalogSource(false, nil); src != nil {
		t.Fatal("HelixLLM is disabled but a catalog source was wired; the catalog " +
			"must list no HelixLLM models rather than assume any")
	}
}

// TestNewHelixLLMCatalogSource_UnreachableYieldsNoOptions is the anti-bluff
// core of the wiring: an endpoint nothing is listening on must produce an empty
// option set, and must do so within the request budget.
func TestNewHelixLLMCatalogSource_UnreachableYieldsNoOptions(t *testing.T) {
	// A closed listener: the address is valid, and nothing answers on it.
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := closed.URL
	closed.Close()

	t.Setenv("HELIX_LLM_ENDPOINT", endpoint)

	src := newHelixLLMCatalogSource(true, logrus.New())
	if src == nil {
		t.Fatal("enabled HelixLLM produced no source at all; an unreachable serving " +
			"layer is a runtime condition, not a wiring failure")
	}

	start := time.Now()
	opts := src.HelixLLMOptions()
	elapsed := time.Since(start)

	if len(opts) != 0 {
		t.Fatalf("an unreachable serving layer produced %d options (%+v): nothing "+
			"confirmed those models are running", len(opts), opts)
	}
	// The handler must not inherit the serving layer's latency.
	if elapsed > helixLLMListTimeout+2*time.Second {
		t.Errorf("listing an unreachable serving layer took %s, well past the %s "+
			"budget: a catalog request must not hang on HelixLLM", elapsed, helixLLMListTimeout)
	}
}

// TestNewHelixLLMCatalogSource_SlowServingLayerIsBounded proves the timeout is
// real: a serving layer that never answers costs the budget, not forever.
func TestNewHelixLLMCatalogSource_SlowServingLayerIsBounded(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-blocked:
		case <-r.Context().Done():
		}
	}))
	defer func() {
		close(blocked)
		srv.Close()
	}()

	t.Setenv("HELIX_LLM_ENDPOINT", srv.URL)

	src := newHelixLLMCatalogSource(true, logrus.New())
	if src == nil {
		t.Fatal("enabled HelixLLM produced no source")
	}
	// The bound must come from the wiring, not from a caller: the source is
	// built on a background context, so if the listing closure did not impose
	// its own deadline, nothing would.
	if _, ok := context.Background().Deadline(); ok {
		t.Fatal("precondition: context.Background() must carry no deadline")
	}

	start := time.Now()
	opts := src.HelixLLMOptions()
	elapsed := time.Since(start)

	if len(opts) != 0 {
		t.Fatalf("a serving layer that never answered produced %d options: a "+
			"listing that did not complete is not evidence of anything", len(opts))
	}
	if elapsed > helixLLMListTimeout+2*time.Second {
		t.Fatalf("a non-answering serving layer held the catalog for %s; the "+
			"budget is %s", elapsed, helixLLMListTimeout)
	}
}

// TestNewHelixLLMCatalogSource_ServesTheServingLayersRealListing proves the
// wiring actually carries what /v1/models publishes — identity, host and
// availability included — rather than anything this repository invented.
func TestNewHelixLLMCatalogSource_ServesTheServingLayersRealListing(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[
			{"id":"qwen3-coder","object":"model","owned_by":"helixllm",
			 "model_identity":"helixllm/workstation/qwen3-coder",
			 "host":"workstation","availability":"serving"}
		]}`))
	}))
	defer srv.Close()

	t.Setenv("HELIX_LLM_ENDPOINT", srv.URL)

	src := newHelixLLMCatalogSource(true, logrus.New())
	if src == nil {
		t.Fatal("enabled HelixLLM produced no source")
	}

	opts := src.HelixLLMOptions()
	if len(opts) != 1 {
		t.Fatalf("options = %+v, want the one model the serving layer published", opts)
	}
	got := opts[0]
	if got.ID != "qwen3-coder" {
		t.Errorf("ID = %q, want the id the serving layer published verbatim", got.ID)
	}
	if got.ModelIdentity != "helixllm/workstation/qwen3-coder" {
		t.Errorf("ModelIdentity = %q, want the published identity", got.ModelIdentity)
	}
	if got.Host != "workstation" {
		t.Errorf("Host = %q, want the published serving host", got.Host)
	}
	if !got.Availability.Usable() {
		t.Errorf("Availability = %q; the serving layer reported it serving", got.Availability)
	}

	// A second listing inside the freshness window must be answered from the
	// cache, not from a second call to the serving layer.
	_ = src.HelixLLMOptions()
	if hits != 1 {
		t.Errorf("serving-layer calls = %d across two catalog requests, want 1", hits)
	}
}

// TestNewHelixLLMCatalogSource_LegacyPayloadIsListedButNotUsable pins the
// behaviour the model-listing contract requires of an OLDER serving layer: it
// publishes only the OpenAI shape, so its models decode cleanly, carry no
// serving claim, and are therefore NOT usable. Nothing is invented to fill the
// gap, and nothing is dropped either.
func TestNewHelixLLMCatalogSource_LegacyPayloadIsListedButNotUsable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[
			{"id":"legacy-model","object":"model","created":0,"owned_by":"helixllm"}
		]}`))
	}))
	defer srv.Close()

	t.Setenv("HELIX_LLM_ENDPOINT", srv.URL)

	src := newHelixLLMCatalogSource(true, logrus.New())
	opts := src.HelixLLMOptions()
	if len(opts) != 1 {
		t.Fatalf("options = %+v, want the one legacy model", opts)
	}
	if opts[0].Availability != catalog.AvailabilityUnreported {
		t.Errorf("Availability = %q, want unreported: a payload that says nothing "+
			"about serving state made no serving claim", opts[0].Availability)
	}
	if opts[0].Availability.Usable() {
		t.Error("a model whose serving state was never reported was treated as " +
			"usable; \"it said nothing\" is not \"it said yes\"")
	}
	if opts[0].ModelIdentity != "" || opts[0].Host != "" {
		t.Errorf("identity/host were invented for a legacy payload: %+v", opts[0])
	}
}

// TestHelixLLMListBudgets pins the two budgets to the reasons they were chosen
// for, so a later widening has to argue with the reason rather than the number.
func TestHelixLLMListBudgets(t *testing.T) {
	if helixLLMListTimeout <= 0 || helixLLMListTimeout > 10*time.Second {
		t.Errorf("helixLLMListTimeout = %s; a user waits on /v1/catalog, so one "+
			"listing must be bounded well inside a request", helixLLMListTimeout)
	}
	// CONST-038: model status must reflect the serving state within 60s.
	if helixLLMListTTL <= 0 || helixLLMListTTL > 60*time.Second {
		t.Errorf("helixLLMListTTL = %s; a listing older than the CONST-038 60s "+
			"accuracy window may advertise a model that stopped being served",
			helixLLMListTTL)
	}
}

// TestCatalogOptions_FeedsNoHardcodedHelixLLMModelList is the standing guard on
// the composition the live /v1/catalog route uses.
//
// It exercises the exact options newCatalogOptions builds, with HelixLLM ENABLED
// and its serving layer unreachable, and asserts the catalog advertises NO
// HelixLLM model. If a fixed id list is ever fed back into the catalog, this
// test fails naming the entry a user would have been offered for a model
// nothing confirmed is running (BLUFF-002, FR-019, CONST-036).
func TestCatalogOptions_FeedsNoHardcodedHelixLLMModelList(t *testing.T) {
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := closed.URL
	closed.Close()

	t.Setenv("USE_HELIX_LLM", "true")
	t.Setenv("HELIX_LLM_ENDPOINT", endpoint)

	opts := newCatalogOptions(nil, logrus.New())

	// Errorf, not Fatalf: the user-visible consequence below must be reported
	// too, so a regression shows BOTH the wiring that caused it and the entry a
	// user would have been offered.
	if len(opts.HelixLLMModels) != 0 {
		t.Errorf("the live catalog was fed a fixed HelixLLM model list %v; those "+
			"ids are advertised whether or not anything is serving them",
			opts.HelixLLMModels)
	}
	if !opts.HelixLLMEnabled {
		t.Fatal("precondition: USE_HELIX_LLM=true must enable the HelixLLM section")
	}

	entries := catalog.New(opts).Build()

	var helixRoot bool
	var models []string
	for _, e := range entries {
		if e.Provider != catalog.NameHelixLLM {
			continue
		}
		if e.Kind == catalog.KindProvider {
			helixRoot = true
			continue
		}
		if e.Kind == catalog.KindModel {
			models = append(models, e.Name)
		}
	}

	// The root is a promotion of the integration itself and stays (§11.4.122).
	if !helixRoot {
		t.Error("the helixllm root entry disappeared; enabling the integration " +
			"must still promote it")
	}
	if len(models) != 0 {
		t.Errorf("the catalog offered HelixLLM model(s) %v while the serving layer "+
			"was unreachable: every one of them is a target a user could select "+
			"for a request that can never be answered", models)
	}
}

// TestCatalogOptions_ServesTheServingLayersModels is the positive half: when
// the serving layer DOES answer, its models reach the catalog as selectable,
// correctly-labelled entries. Without this, "no models" would be trivially
// satisfiable by wiring nothing at all.
func TestCatalogOptions_ServesTheServingLayersModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[
			{"id":"qwen3-coder","object":"model","owned_by":"helixllm",
			 "model_identity":"helixllm/workstation/qwen3-coder",
			 "host":"workstation","availability":"serving"},
			{"id":"big-model","object":"model","owned_by":"helixllm",
			 "model_identity":"helixllm/workstation/big-model",
			 "host":"workstation","availability":"withheld",
			 "withheld_reason":"insufficient_resources"}
		]}`))
	}))
	defer srv.Close()

	t.Setenv("USE_HELIX_LLM", "true")
	t.Setenv("HELIX_LLM_ENDPOINT", srv.URL)

	entries := catalog.New(newCatalogOptions(nil, logrus.New())).Build()

	byName := map[string]catalog.Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}

	served, ok := byName["helixllm/qwen3-coder"]
	if !ok {
		t.Fatalf("the served model never reached the catalog; entries: %v", entries)
	}
	if !served.Enabled {
		t.Error("a model the serving layer reports SERVING was not marked enabled")
	}
	if served.Host != "workstation" || served.ModelIdentity != "helixllm/workstation/qwen3-coder" {
		t.Errorf("host/identity were not carried through: %+v", served)
	}

	withheld, ok := byName["helixllm/big-model"]
	if !ok {
		t.Fatalf("the withheld model was dropped instead of being reported as "+
			"withheld; entries: %v", entries)
	}
	if withheld.Enabled {
		t.Error("a WITHHELD model was offered as enabled: it is not being served")
	}
	if withheld.WithheldReason != catalog.ReasonInsufficientResources {
		t.Errorf("WithheldReason = %q, want the reason the serving layer gave — "+
			"it is the part the user can act on", withheld.WithheldReason)
	}
}
