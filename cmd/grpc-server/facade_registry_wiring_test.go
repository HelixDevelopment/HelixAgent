package main

// Standing regression guard for HXC-271 (§11.4.135) — the llm.LLMFacade
// completion family must reach the live provider registry.
//
// THE DEFECT. cmd/grpc-server publishes two completion families on one
// grpc.Server. main() built one *services.ProviderRegistry and handed it to
// LLMProviderServer only; LLMFacadeServer had no registry field at all, so
// Complete / CompleteStream / Chat fell through to the package-level
// llm.RunEnsemble, which passes a hardcoded nil provider slice and refuses on
// an empty one. llm.LLMFacade is the service the published interface presents
// first, so it is what an integrator reaches for — and it could not succeed on
// any machine, however well configured.
//
// WHY IT SURVIVED. The refusal reads as the caller's mistake ("use
// services.ProviderRegistry for proper credential injection"), so a failure
// looked like missing credentials at the far end rather than a missing wire at
// this one. Captured pre-fix, over a real socket, on a host where the registry
// in the SAME process held a working provider:
//
//	llm.LLMFacade/Complete  -> Unknown "no providers configured - use services.ProviderRegistry ..."
//	llm.LLMProvider/Complete -> content:"SENTINEL..." provider_name:"..."   (same registry)
//
// HOW IT AROSE (git history, so the next reader does not re-derive it and does
// not mistake it for a deliberate design). 127d556e (2025-12-31) made the
// package-level llm.RunEnsemble refuse instead of silently constructing
// providers with empty credentials — a good anti-bluff fix. f45f38ba
// (2026-01-03) then wired LLMFacadeServer's three methods to that very
// function, three days AFTER it began refusing: the facade's completions were
// dead on arrival and never worked. 3d0ef6c1 (2026-01-04) introduced
// LLMProviderServer the next day WITH the registry-first pattern written
// correctly — but never backported it to the facade's three siblings. 93e63ada
// (round-29 anti-bluff sweep) later edited LLMFacadeServer.CompleteStream to
// remove a fabricated 0.85 confidence, one line from the unrouted call, without
// noticing it. No comment, README, or commit anywhere describes the facade as
// an intentional thin client of the provider service. This was a missed
// backport, not a split-brain design a wiring fix would violate.
//
// WHY THIS GUARD IS BEHAVIOURAL, NOT STRUCTURAL. The defect class is "a call
// that looks wired and is not". A guard that asserted the registry FIELD
// exists, or that main() mentions it, would certify exactly the bug it is
// meant to catch: a field can be present and point at an empty registry, or be
// read and then ignored. So every assertion below drives a real gRPC request
// over a real socket and requires the answer to carry a sentinel that ONLY the
// test's own provider could have produced. A registry that is merely non-nil
// cannot satisfy that; only one whose providers actually serve this server can.
//
// POLARITY (§11.4.115). One source, two roles, switched by RED_MODE:
//
//	RED_MODE=1 — reproduce: a registry-less facade refuses. This is not a
//	             synthetic failure. NewLLMFacadeServer(nil) is byte-for-byte
//	             the pre-fix code path (no registry -> llm.RunEnsemble -> the
//	             standalone refusal), which is what the pre-fix binary did on
//	             EVERY call because it was always registry-less.
//	RED_MODE=0 — guard: the facade main() actually builds serves providers.
//
// The RED baseline was taken on the genuinely broken artifact (b30cf1f7,
// before this fix) with a throwaway probe, not on this file — this file could
// not compile there, since the constructor took no registry to give it. The
// two arms therefore assert the same two runtime facts the probe recorded.

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	pb "dev.helix.agent/pkg/api"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Expected wordings are TEST-LOCAL COPIES, deliberately not imported from the
// code under test. A guard that asked the implementation what it says, then
// checked it said it, would pass through any rewording — including a rewording
// that changed which code path produced the message. These two literals are
// how the test tells the standalone refusal apart from the registry's own, so
// they must be pinned independently. If either genuinely changes, update this
// copy and say why (§11.4.120: reconcile a gate, never fake-pass it).
const (
	// From internal/llm/ensemble.go — reached ONLY when no registry routed
	// the call. Its presence in an answer is the defect's signature.
	wantStandaloneRefusal = "no providers configured - use services.ProviderRegistry"

	// From internal/services/ensemble.go — reached only THROUGH a registry.
	// Its presence proves the call was routed, even when it fails.
	wantRegistryRefusal = "no providers available"
)

// hxc271Sentinel is content no code path but the test's own provider can
// produce. Every positive assertion in this file requires it, which is what
// makes "the registry is wired" unforgeable by a non-nil-but-useless registry.
const hxc271Sentinel = "HXC271-SENTINEL-ONLY-A-REAL-PROVIDER-EMITS-THIS"

const hxc271ProviderName = "hxc271-guard-provider"

func hxc271RedMode() bool { return strings.TrimSpace(os.Getenv("RED_MODE")) == "1" }

// sentinelProvider is a unit-test provider (CONST-050(A): stubs are permitted
// in *_test.go only). It exists to be unmistakable — if a completion carries
// its sentinel, that completion genuinely traversed a registered provider.
type sentinelProvider struct{}

func (p *sentinelProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	return &models.LLMResponse{
		ID:           "hxc271",
		Content:      hxc271Sentinel,
		Confidence:   0.99,
		ProviderName: hxc271ProviderName,
	}, nil
}

func (p *sentinelProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
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

func (p *sentinelProvider) HealthCheck() error { return nil }

func (p *sentinelProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{}
}

func (p *sentinelProvider) ValidateConfig(map[string]interface{}) (bool, []string) {
	return true, nil
}

// hxc271Registry builds a registry with auto-discovery OFF, so the test never
// depends on host environment variables or reachable provider endpoints
// (§11.4.50 determinism). withProvider selects whether it can actually serve.
func hxc271Registry(t *testing.T, withProvider bool) *services.ProviderRegistry {
	t.Helper()
	reg := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	if withProvider {
		require.NoError(t, reg.RegisterProvider(hxc271ProviderName, &sentinelProvider{}),
			"could not register the guard's provider; the assertions below would be vacuous")
	}
	return reg
}

// serveViaMainWiring starts a real gRPC server over a real TCP socket using
// registerGRPCServices — the SAME function main() calls. The test therefore
// exercises the shipped wiring, not a re-creation of it that could agree with
// the test while main() disagrees with both.
//
// Port 0 lets the OS assign a free port. This is deliberate: this repo has
// already been bitten by assuming a fixed port was ours (:8100 belongs to
// llm-verifier, and a probe that finds *something* answering there learns
// nothing about HelixAgent). An OS-assigned port cannot be another service's.
func serveViaMainWiring(t *testing.T, registry *services.ProviderRegistry) *grpc.ClientConn {
	t.Helper()

	gs := grpc.NewServer()
	registerGRPCServices(gs, registry)
	return serveAndDial(t, gs)
}

// serveRegistrylessFacade starts a server whose facade was built with NO
// registry — the pre-fix wiring, replayed for the RED arm.
func serveRegistrylessFacade(t *testing.T) *grpc.ClientConn {
	t.Helper()

	gs := grpc.NewServer()
	pb.RegisterLLMFacadeServer(gs, NewLLMFacadeServer(nil))
	return serveAndDial(t, gs)
}

func serveAndDial(t *testing.T, gs *grpc.Server) *grpc.ClientConn {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "could not obtain a loopback port for the guard server")

	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "could not dial the guard server at %s", lis.Addr())
	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

func hxc271Ctx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestHXC271_FacadeCompleteReachesTheProviderRegistry is the core polarity
// test: the unary completion an integrator calls first.
func TestHXC271_FacadeCompleteReachesTheProviderRegistry(t *testing.T) {
	ctx := hxc271Ctx(t)

	if hxc271RedMode() {
		client := pb.NewLLMFacadeClient(serveRegistrylessFacade(t))
		_, err := client.Complete(ctx, &pb.CompletionRequest{
			SessionId: "hxc271-red", Prompt: "What is 2+2?",
		})
		require.Error(t, err,
			"RED_MODE=1: the pre-fix facade had no registry and MUST refuse; a success "+
				"here means the reproduction is no longer reproducing anything")
		require.Containsf(t, status.Convert(err).Message(), wantStandaloneRefusal,
			"RED_MODE=1: the pre-fix facade must fail via the standalone package-level "+
				"ensemble, which is the defect's signature; got %q", status.Convert(err).Message())
		return
	}

	client := pb.NewLLMFacadeClient(serveViaMainWiring(t, hxc271Registry(t, true)))
	resp, err := client.Complete(ctx, &pb.CompletionRequest{
		SessionId: "hxc271-green", Prompt: "What is 2+2?",
	})

	require.NoErrorf(t, err,
		"llm.LLMFacade/Complete failed while the registry main() wires held a working "+
			"provider. If the message mentions %q, the facade is once again answering "+
			"from the registry-less package-level ensemble (HXC-271 has regressed).",
		wantStandaloneRefusal)

	// The usefulness assertion. Not "no error" and not "some content" — the
	// sentinel, which only a genuinely-served provider can produce. This is
	// what a non-nil-but-empty registry, or a registry the facade holds but
	// never routes to, cannot fake.
	require.Equal(t, hxc271Sentinel, resp.Content,
		"the completion did not carry the registered provider's output, so the request "+
			"did not actually traverse the registry the server was wired with")
	require.Equal(t, hxc271ProviderName, resp.ProviderName,
		"the completion is not attributed to the registered provider")
}

// TestHXC271_FacadeStreamingPathsReachTheProviderRegistry extends the guard to
// the other two facade methods (§11.4.146 STEP 3). The defect was never
// Complete-only: CompleteStream and Chat each carried their own copy of the
// same unrouted call, so a fix verified on Complete alone would have left two
// thirds of the family broken with the guard still green.
func TestHXC271_FacadeStreamingPathsReachTheProviderRegistry(t *testing.T) {
	if hxc271RedMode() {
		ctx := hxc271Ctx(t)
		client := pb.NewLLMFacadeClient(serveRegistrylessFacade(t))

		cs, err := client.CompleteStream(ctx, &pb.CompletionRequest{
			SessionId: "hxc271-red", Prompt: "stream",
		})
		require.NoError(t, err, "opening the stream should not itself fail")
		_, err = cs.Recv()
		require.Error(t, err, "RED_MODE=1: the pre-fix CompleteStream must refuse")
		require.Contains(t, status.Convert(err).Message(), wantStandaloneRefusal,
			"RED_MODE=1: CompleteStream must fail via the standalone ensemble")

		ch, err := client.Chat(ctx, &pb.ChatRequest{
			SessionId: "hxc271-red",
			Messages:  []*pb.Message{{Role: "user", Content: "hello"}},
		})
		require.NoError(t, err, "opening the chat stream should not itself fail")
		_, err = ch.Recv()
		require.Error(t, err, "RED_MODE=1: the pre-fix Chat must refuse")
		require.Contains(t, status.Convert(err).Message(), wantStandaloneRefusal,
			"RED_MODE=1: Chat must fail via the standalone ensemble")
		return
	}

	conn := serveViaMainWiring(t, hxc271Registry(t, true))
	client := pb.NewLLMFacadeClient(conn)

	t.Run("CompleteStream", func(t *testing.T) {
		stream, err := client.CompleteStream(hxc271Ctx(t), &pb.CompletionRequest{
			SessionId: "hxc271-green", Prompt: "stream",
		})
		require.NoError(t, err)

		var got strings.Builder
		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoErrorf(t, err,
				"CompleteStream failed mid-stream; if it mentions %q the streaming path "+
					"is not routed through the registry", wantStandaloneRefusal)
			got.WriteString(chunk.Content)
		}
		require.Equal(t, hxc271Sentinel, got.String(),
			"the streamed completion did not reassemble to the registered provider's "+
				"output, so CompleteStream did not traverse the registry")
	})

	t.Run("Chat", func(t *testing.T) {
		stream, err := client.Chat(hxc271Ctx(t), &pb.ChatRequest{
			SessionId: "hxc271-green",
			Messages:  []*pb.Message{{Role: "user", Content: "hello"}},
		})
		require.NoError(t, err)

		var got strings.Builder
		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoErrorf(t, err,
				"Chat failed mid-stream; if it mentions %q the chat path is not routed "+
					"through the registry", wantStandaloneRefusal)
			got.WriteString(chunk.Content)
		}
		require.Equal(t, hxc271Sentinel, got.String(),
			"the chat response did not reassemble to the registered provider's output, "+
				"so Chat did not traverse the registry")
	})
}

// TestHXC271_FacadeFailsThroughTheRegistryNotAroundIt is the second half of
// the anti-bluff pair, and the one that catches a fix which only LOOKS wired.
//
// The positive tests prove the facade can answer when providers exist. They do
// not, by themselves, prove WHERE the answer came from when there are none —
// and a plausible bad fix (hold the registry, then call llm.RunEnsemble
// anyway; or wire a private registry nobody populates) would leave every
// no-provider failure looking exactly as it did before the fix.
//
// So: with a registry that is present but empty, the refusal must be the
// REGISTRY's, never the standalone package-level one. The two messages are
// different code paths, and which one answers is the whole question.
func TestHXC271_FacadeFailsThroughTheRegistryNotAroundIt(t *testing.T) {
	if hxc271RedMode() {
		t.Skip("RED_MODE=1 replays the registry-less facade, which by definition fails " +
			"through the standalone path; this arm's premise is a registry that IS " +
			"present, so there is no pre-fix baseline for it " +
			"(SKIP-OK: #hxc271-no-red-baseline-for-present-registry)")
	}

	ctx := hxc271Ctx(t)
	client := pb.NewLLMFacadeClient(serveViaMainWiring(t, hxc271Registry(t, false)))

	_, err := client.Complete(ctx, &pb.CompletionRequest{
		SessionId: "hxc271-empty", Prompt: "What is 2+2?",
	})
	require.Error(t, err, "a registry with no providers must still fail the completion")

	msg := status.Convert(err).Message()
	require.NotContainsf(t, msg, wantStandaloneRefusal,
		"the facade failed via the standalone package-level ensemble even though it "+
			"HOLDS a registry — the registry is decorative, not wired; got %q", msg)
	require.Containsf(t, msg, wantRegistryRefusal,
		"the failure did not come from the registry's own ensemble, so the call was not "+
			"routed through the registry; got %q", msg)
}

// TestHXC271_BothServicesShareTheOneRegistry pins the invariant whose absence
// WAS the defect: two services, one registry, both wired.
//
// The families were never meant to differ — LLMProvider worked and LLMFacade
// did not, for no reason a caller could see. Asserting both answer from the
// same registry in the same server is what makes "we wired the other one too"
// a fact rather than an intention.
func TestHXC271_BothServicesShareTheOneRegistry(t *testing.T) {
	if hxc271RedMode() {
		t.Skip("RED_MODE=1 replays the pre-fix facade in isolation; this arm asserts the " +
			"post-fix shared-registry invariant, which had no pre-fix form to baseline " +
			"(SKIP-OK: #hxc271-no-red-baseline-for-shared-registry)")
	}

	ctx := hxc271Ctx(t)
	conn := serveViaMainWiring(t, hxc271Registry(t, true))

	facadeResp, facadeErr := pb.NewLLMFacadeClient(conn).Complete(ctx,
		&pb.CompletionRequest{SessionId: "hxc271-both", Prompt: "What is 2+2?"})
	require.NoError(t, facadeErr, "llm.LLMFacade could not complete")

	providerResp, providerErr := pb.NewLLMProviderClient(conn).Complete(ctx,
		&pb.CompletionRequest{SessionId: "hxc271-both", Prompt: "What is 2+2?"})
	require.NoError(t, providerErr, "llm.LLMProvider could not complete")

	require.Equal(t, hxc271Sentinel, facadeResp.Content,
		"llm.LLMFacade did not answer from the shared registry")
	require.Equal(t, hxc271Sentinel, providerResp.Content,
		"llm.LLMProvider did not answer from the shared registry")
	require.Equal(t, providerResp.Content, facadeResp.Content,
		"the two services registered by registerGRPCServices disagree about the "+
			"providers they serve, so they are not backed by the same registry")
}
