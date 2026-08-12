package main

// Standing regression guard for HXC-281 (§11.4.135) — the LLMFacadeServer
// streaming paths must emit chunks whose Confidence is the value a provider
// actually supplied, and the guard that says so must be checking real chunks.
//
// THE DEFECT (HXC-281). The behavioural round-29 guard,
// TestRound29_ChatStreamDegeneratePath_NoFabricatedConfidence in main_test.go,
// asserted inside `for i, ch := range stream.chunks`. It built the server with
// NewLLMFacadeServer(nil) — no registry — so CompleteStream reached the
// package-level llm.RunEnsemble, took its refusal, and returned BEFORE the
// chunking loop. `stream.chunks` was therefore empty on every run since the
// test was written: the loop body never executed and the assertion never ran.
// A test that iterates an always-empty list is green for the same reason an
// empty `for` is green, which is no reason at all. Captured on fa09060f with a
// throwaway probe replaying the pre-fix construction exactly:
//
//	CompleteStream err = rpc error: code = Unknown desc = no providers
//	    configured - use services.ProviderRegistry for proper credential injection
//	len(stream.chunks) = 0
//
// It was also MIS-NAMED: it said ChatStream and drove CompleteStream. Those are
// two different methods with two separate copies of the round-29 code, so the
// name did not merely read wrong — it described coverage that did not exist.
//
// SCOPE OF THIS FILE, AND WHY IT IS NOT THE WHOLE FAN-OUT (HXC-281). This file
// guards the FACADE's two streaming copies. main.go carries a THIRD copy in
// LLMProviderServer.CompleteStream — a separate gRPC service with a
// byte-identical fabrication surface. The first pass at this fix stopped at two
// because two was the number the DEFECT had, not the number the CODE has. The
// third copy, the two unary Complete siblings, and a census that makes the next
// such omission mechanical rather than noticed, all live in
// grpc_confidence_sites_test.go.
//
// WHY IT IS FIXABLE NOW. HXC-271 (fa09060f) routed the facade's completion
// family through the live provider registry, so CompleteStream and Chat can
// finally produce chunks at all. Before that commit no registry-backed chunk
// could be obtained from this service on any machine, so there was nothing for
// this guard to inspect. See facade_registry_wiring_test.go for that fix's own
// guard; this file follows its sentinel discipline — every positive assertion
// requires a value ONLY this test's provider could have produced.
//
// WHAT "FABRICATED CONFIDENCE" MEANS AT THIS SEAM. A chunk's
// pb.CompletionResponse.Confidence / pb.ChatResponse.Confidence is read as the
// provider's own self-reported confidence in the content being streamed. It is
// FABRICATED when the server synthesises a number no provider supplied. The
// historical instance (round 29) was the literal 0.85 written straight into the
// chunk struct literal, so a degenerate ensemble that had returned nothing
// still produced a meaningful-looking score. Post-fix (main.go CompleteStream /
// Chat) the only legitimate sources are selected.Confidence, responses[0]
// .Confidence, or 0.0 on the degenerate path — and on that path content is ""
// so no chunk is emitted at all, which is why the property is only observable
// once a provider genuinely serves the request. The property is expressible
// against real chunks: every emitted chunk MUST carry exactly the confidence
// its provider supplied. It is asserted by EQUALITY, not by "not 0.85" alone —
// a fabricated 0.7 would sail past a not-0.85 check, and a provider that
// legitimately reported 0.85 would fail one.
//
// A note on the ensemble path, since it sits between the provider and the
// chunk: services.EnsembleService.RunEnsemble copies provider responses through
// voting, and ConfidenceWeightedStrategy.Vote writes Selected and
// SelectionScore but never Confidence. The provider's value therefore reaches
// the chunk unmodified, which is what makes exact equality the right assertion
// rather than a tolerance.
//
// POLARITY (§11.4.115). One source, two roles, switched by RED_MODE:
//
//	RED_MODE=1 — reproduce: replay the pre-fix construction
//	             (NewLLMFacadeServer(nil), exactly what the defective test
//	             built) and assert the captured chunk list is EMPTY. That
//	             emptiness IS the defect — it is the reason the old
//	             assertion loop never ran. Not a synthetic failure: this is
//	             the same code path, byte for byte, that the old test drove.
//	RED_MODE=0 — guard: a registry-backed facade streams multiple real
//	             chunks and every one of them carries only the provider's
//	             own confidence.
//
// The subject of this guard can disappear (a future change upstream stops
// producing chunks) exactly as it did before, so the chunk-count floor below is
// load-bearing: it fails when there is nothing left to check, instead of
// quietly passing again.

import (
	"context"
	"os"
	"strings"
	"testing"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	pb "dev.helix.agent/pkg/api"
	"github.com/stretchr/testify/require"
)

func hxc281RedMode() bool { return strings.TrimSpace(os.Getenv("RED_MODE")) == "1" }

// Expected values are TEST-LOCAL. The provider below returns them and the
// assertions compare against these same constants, so nothing is ever checked
// against a number the code under test computed and handed back — a guard that
// asked the server what confidence it emitted, then agreed, would pass through
// any fabrication.
const (
	hxc281ProviderName = "hxc281-guard-provider"

	// Deliberately not 0.85 (the historical fabrication), not 0.0 (the honest
	// degenerate-path value), and not a round number that could coincide with
	// a default somewhere.
	hxc281Confidence float64 = 0.4217

	// The round-29 bluff literal, pinned here so the assertion that names it
	// does not depend on the source still containing it.
	hxc281HistoricalFabricatedConfidence float64 = 0.85
)

// hxc281Content is long enough that the server must split it into several
// chunks, which is the point: the defective guard could only ever have seen an
// empty list, so this fixture is what gives it something real to inspect.
// Content no other code path can produce, per the HXC-271 sentinel discipline.
const hxc281Content = "HXC281-SENTINEL-STREAMED-BODY-SEGMENT-01-ONLY-THIS-PROVIDER-EMITS-IT|" +
	"HXC281-SENTINEL-STREAMED-BODY-SEGMENT-02-ONLY-THIS-PROVIDER-EMITS-IT|" +
	"HXC281-SENTINEL-STREAMED-BODY-SEGMENT-03-ONLY-THIS-PROVIDER-EMITS-IT"

// hxc281Provider is a unit-test provider (CONST-050(A): stubs live in
// *_test.go only). It never performs I/O — no real provider is contacted, no
// credential is read, nothing is spent.
type hxc281Provider struct{}

func (p *hxc281Provider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	return &models.LLMResponse{
		ID:           "hxc281",
		Content:      hxc281Content,
		Confidence:   hxc281Confidence,
		ProviderName: hxc281ProviderName,
	}, nil
}

func (p *hxc281Provider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	resp, err := p.Complete(ctx, req)
	if err != nil {
		close(ch)
		return ch, err
	}
	ch <- resp
	close(ch)
	return ch, nil
}

func (p *hxc281Provider) HealthCheck() error { return nil }

func (p *hxc281Provider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{}
}

func (p *hxc281Provider) ValidateConfig(map[string]interface{}) (bool, []string) {
	return true, nil
}

// hxc281Registry builds a registry with auto-discovery OFF so the test never
// depends on host environment variables or reachable provider endpoints
// (§11.4.50 determinism, and no real provider call is ever issued).
func hxc281Registry(t *testing.T) *services.ProviderRegistry {
	t.Helper()
	reg := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	require.NoError(t, reg.RegisterProvider(hxc281ProviderName, &hxc281Provider{}),
		"could not register the guard's provider; every assertion below would be vacuous — "+
			"which is the exact HXC-281 failure this file exists to prevent")
	return reg
}

// hxc281Chunk is the shape both streaming methods share, so one set of
// assertions covers CompleteStream (pb.CompletionResponse) and Chat
// (pb.ChatResponse) without either being checked more loosely than the other.
type hxc281Chunk struct {
	content      string
	confidence   float64
	providerName string
}

// hxc281AssertStreamedChunks holds the whole contract in one place.
//
// Order matters: the count floor comes FIRST because it is the assertion whose
// absence caused HXC-281. Everything after it is a per-chunk claim, and
// per-chunk claims about an empty list are unfalsifiable.
//
// `method` is the FULLY-QUALIFIED method label ("LLMFacadeServer.Chat"), not a
// bare method name. It used to be bare, with "LLMFacadeServer." spliced into
// the failure text — which was fine while the facade was the only caller and
// actively misleading the moment it was not. main.go carries a THIRD copy of
// this streaming code in LLMProviderServer.CompleteStream (see
// grpc_confidence_sites_test.go, HXC-281); that guard shares this helper, so a
// hardcoded service name here would have mis-attributed its failures to the
// facade.
func hxc281AssertStreamedChunks(t *testing.T, method string, chunks []hxc281Chunk) {
	t.Helper()

	require.GreaterOrEqualf(t, len(chunks), 2,
		"%s emitted %d chunk(s); this guard requires a genuinely multi-chunk stream, because "+
			"the HXC-281 defect was precisely a per-chunk assertion looping over an empty list. "+
			"If the server's chunk size legitimately grew past the fixture, LENGTHEN "+
			"hxc281Content — do NOT relax this floor (§11.4.120: reconcile a gate, never "+
			"fake-pass it).", method, len(chunks))

	var reassembled strings.Builder
	for i, ch := range chunks {
		// The load-bearing assertion. Equality against the test-local constant,
		// not a "not 0.85" check: a fabricated 0.7 would pass the latter.
		require.Equalf(t, hxc281Confidence, ch.confidence,
			"%s chunk %d carried confidence %v, but the only provider serving this request "+
				"reported %v. A chunk confidence the provider never supplied is fabricated "+
				"(round-29 §11.4 anti-bluff regression in the %s path).",
			method, i, ch.confidence, hxc281Confidence, method)

		// Names the historical bluff explicitly. Subsumed by the equality above
		// while that holds, and deliberately kept so the specific round-29
		// literal still has a guard of its own if the equality is ever loosened.
		require.NotEqualf(t, hxc281HistoricalFabricatedConfidence, ch.confidence,
			"%s chunk %d carried the round-29 fabricated 0.85 confidence fallback", method, i)

		require.Equalf(t, hxc281ProviderName, ch.providerName,
			"%s chunk %d is not attributed to the registered provider, so it did not come "+
				"from the registry this server was wired with", method, i)

		reassembled.WriteString(ch.content)
	}

	require.Equalf(t, hxc281Content, reassembled.String(),
		"%s chunks did not reassemble to the registered provider's output, so the stream "+
			"carried something other than the served content", method)
}

// TestHXC281_CompleteStreamChunksCarryOnlyProviderSuppliedConfidence guards the
// method the defective test actually drove (while claiming to drive Chat).
func TestHXC281_CompleteStreamChunksCarryOnlyProviderSuppliedConfidence(t *testing.T) {
	req := &pb.CompletionRequest{SessionId: "hxc281", Prompt: "stream several chunks"}

	if hxc281RedMode() {
		// The pre-fix construction, verbatim: no registry.
		server := NewLLMFacadeServer(nil)
		stream := &captureCompletionStream{ctx: context.Background()}

		err := server.CompleteStream(req, stream)
		require.Error(t, err,
			"RED_MODE=1: a registry-less facade must refuse before it chunks anything; "+
				"a success here means the reproduction no longer reproduces the condition "+
				"that made the old assertion vacuous")
		require.Empty(t, stream.chunks,
			"RED_MODE=1: the pre-fix guard's chunk list MUST be empty — that emptiness is "+
				"the defect, since it is why its per-chunk assertion never executed")
		return
	}

	server := NewLLMFacadeServer(hxc281Registry(t))
	stream := &captureCompletionStream{ctx: context.Background()}

	require.NoError(t, server.CompleteStream(req, stream),
		"CompleteStream failed while the registry held a working provider")

	chunks := make([]hxc281Chunk, 0, len(stream.chunks))
	for _, c := range stream.chunks {
		chunks = append(chunks, hxc281Chunk{
			content:      c.Content,
			confidence:   c.Confidence,
			providerName: c.ProviderName,
		})
	}
	hxc281AssertStreamedChunks(t, "LLMFacadeServer.CompleteStream", chunks)
}

// TestHXC281_ChatChunksCarryOnlyProviderSuppliedConfidence guards the method
// the defective test's NAME claimed. Chat carries its own copy of the round-29
// code (main.go), so it needed its own behavioural guard rather than inheriting
// confidence from a test that never touched it. This is why the mis-naming was
// a coverage gap and not just a typo.
func TestHXC281_ChatChunksCarryOnlyProviderSuppliedConfidence(t *testing.T) {
	req := &pb.ChatRequest{
		SessionId: "hxc281",
		Messages:  []*pb.Message{{Role: "user", Content: "stream several chunks"}},
	}

	if hxc281RedMode() {
		server := NewLLMFacadeServer(nil)
		stream := &captureChatStream{ctx: context.Background()}

		err := server.Chat(req, stream)
		require.Error(t, err,
			"RED_MODE=1: a registry-less facade must refuse before it chunks anything")
		require.Empty(t, stream.chunks,
			"RED_MODE=1: the pre-fix Chat path emits no chunks, so a per-chunk assertion "+
				"over it would be vacuous in exactly the HXC-281 way")
		return
	}

	server := NewLLMFacadeServer(hxc281Registry(t))
	stream := &captureChatStream{ctx: context.Background()}

	require.NoError(t, server.Chat(req, stream),
		"Chat failed while the registry held a working provider")

	chunks := make([]hxc281Chunk, 0, len(stream.chunks))
	for _, c := range stream.chunks {
		chunks = append(chunks, hxc281Chunk{
			content:      c.Content,
			confidence:   c.Confidence,
			providerName: c.ProviderName,
		})
	}
	hxc281AssertStreamedChunks(t, "LLMFacadeServer.Chat", chunks)
}
