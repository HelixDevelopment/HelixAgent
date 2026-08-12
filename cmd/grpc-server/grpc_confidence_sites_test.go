package main

// Standing regression guards for HXC-281 (§11.4.135) — every site in
// cmd/grpc-server/main.go that writes a Confidence into a response a caller
// receives must be behaviourally guarded, and the SET of those sites must be
// pinned so the next one added cannot slip in unguarded.
//
// THE FINDING (HXC-281). HXC-281 replaced a vacuous round-29 guard with two
// real behavioural ones, correctly reasoning that the old mis-named test hid a
// coverage gap. But it fanned out to two methods because TWO was the number the
// DEFECT had, not the number the CODE has. main.go holds THREE copies of the
// round-29 streaming code — LLMFacadeServer.CompleteStream,
// LLMFacadeServer.Chat, and LLMProviderServer.CompleteStream, the last being a
// separate gRPC service with a byte-identical fabrication surface — plus two
// unary Complete siblings that also hand a Confidence to a caller.
//
// The gap was measured, not inferred. On fa09060f, with HXC-281's two guards in
// place, `Confidence: 0.7` written into the chunk literal of
// LLMProviderServer.CompleteStream (main.go:961) COMPILED and SURVIVED the
// entire module:
//
//	go build ./cmd/grpc-server/   -> exit 0
//	go test ./cmd/grpc-server/    -> ok   (both HXC-281 guards green)
//	go test ./internal/challenges/-> ok   (HXC-271 wiring tests green)
//
// It passed the structural scan too, because 0.7 is not in that scan's banned
// literal list — the exact lesson TestRound29_NoFabricated085FallbackInStreamingPaths
// already records about what a source scan cannot see. The same mutation at the
// two unary sites (main.go:206+:211 and :884+:889) also compiled and survived:
// the HXC-271 wiring tests drive both unary Completes but assert Content and
// ProviderName, never Confidence, and main_test.go's only Confidence assertion
// (TestGrpcServer_Complete) checks the ERROR-path `Confidence: 0` literal, which
// no provider value ever reaches.
//
// WHY THE CENSUS EXISTS. Fixing the third copy closes today's gap; it does not
// stop the fourth. The root cause of HXC-281 was that nobody counted the sites —
// and the information was already written down (the old structural test's
// comment names "three sites"), just never checked against the code. So
// TestHXC281_ConfidenceWriteSiteCensus parses main.go and pins the exact set of
// Confidence-write sites (composite-literal keys and plain assignments) per
// method IN THAT FILE. Add such a site to an existing or new method in main.go
// and it fails until a behavioural guard is added and the census updated
// deliberately. See HONEST BOUNDARY below for what this does NOT catch.
//
// HONEST BOUNDARY (§11.4.6). The census is STRUCTURAL. It proves nothing about
// whether any site behaves correctly — that is entirely the job of the
// behavioural guards below and in facade_stream_confidence_test.go. It is a
// tripwire against silent fan-out gaps, not evidence of coverage, and it must
// never be read as the latter. It is also deliberately blind to a fabrication
// written into an EXISTING site (`confidence = 0.7` at main.go:264 would not
// move the count); that case is exactly what the behavioural guards catch, which
// is why the two are kept as a pair rather than either standing alone.
//
// Two further limits, both measured rather than assumed, neither narrowed by
// this file's earlier claim that "add a site ... and it fails":
//
//   - FILE SCOPE. The census parses ONLY main.go (see mainPath below). A
//     Confidence-write site added to any OTHER file in this package — a new
//     non-test file containing `&pb.CompletionResponse{Confidence: 0.5}`, for
//     instance — builds and passes the ENTIRE suite green, because the census
//     never opens that file to look. Every server method lives in main.go today,
//     so this is a prospective gap, not a live one, but the protection this file
//     provides stops at main.go's boundary and does not extend package-wide.
//
//   - WRITE SHAPE. The AST walk below recognises exactly two node shapes: a
//     `Confidence:` key inside a composite literal (ast.KeyValueExpr), and a
//     plain `x.Confidence = v` assignment (ast.AssignStmt). It does NOT
//     recognise `x.Confidence++` / `x.Confidence--` (ast.IncDecStmt), a
//     whole-struct copy, or a `proto.Merge` call — each mutates a Confidence
//     field by the same definition this file otherwise uses, yet leaves the
//     pinned count unchanged. Measured on fa09060f: inserting `out.Confidence++`
//     immediately after the assignment at main.go:211 left this census GREEN
//     while the unary behavioural guard for LLMFacadeServer.Complete
//     (TestHXC281_UnaryCompleteCarriesOnlyProviderSuppliedConfidence/
//     LLMFacadeServer, in this file) FAILed — so an increment is a
//     Confidence-write by this file's own stated definition, yet goes
//     uncounted. The behavioural guards are what catch it; the census does not.
//
// POLARITY (§11.4.115). The behavioural guards follow the RED_MODE discipline of
// facade_stream_confidence_test.go and reuse its provider, registry, and
// assertion helper verbatim, so no site is checked more loosely than another.

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	pb "dev.helix.agent/pkg/api"
	"github.com/stretchr/testify/require"
)

// hxc281ConfidenceWriteCensus is the pinned set: enclosing method -> number of
// statements in main.go that write a Confidence field into a response value.
//
// Reads of a Confidence (`confidence = selected.Confidence`) are NOT sites —
// they feed a local that reaches one of the sites below, and the behavioural
// guards cover that path. Only writes into a response reach a caller.
var hxc281ConfidenceWriteCensus = map[string]int{
	// :198 error-path literal, :206 and :211 provider-sourced.
	"LLMFacadeServer.Complete": 3,
	// :287 chunk literal.
	"LLMFacadeServer.CompleteStream": 1,
	// :380 chunk literal (pb.ChatResponse).
	"LLMFacadeServer.Chat": 1,
	// :876 error-path literal, :884 and :889 provider-sourced.
	"LLMProviderServer.Complete": 3,
	// :961 chunk literal — the copy HXC-281's fan-out missed.
	"LLMProviderServer.CompleteStream": 1,
}

// hxc281GuardedBy names the behavioural guard for each censused method, so a
// census failure tells the next reader where to extend rather than leaving them
// to rediscover the map.
var hxc281GuardedBy = map[string]string{
	"LLMFacadeServer.Complete":         "TestHXC281_UnaryCompleteCarriesOnlyProviderSuppliedConfidence/LLMFacadeServer",
	"LLMFacadeServer.CompleteStream":   "TestHXC281_CompleteStreamChunksCarryOnlyProviderSuppliedConfidence",
	"LLMFacadeServer.Chat":             "TestHXC281_ChatChunksCarryOnlyProviderSuppliedConfidence",
	"LLMProviderServer.Complete":       "TestHXC281_UnaryCompleteCarriesOnlyProviderSuppliedConfidence/LLMProviderServer",
	"LLMProviderServer.CompleteStream": "TestHXC281_ProviderCompleteStreamChunksCarryOnlyProviderSuppliedConfidence",
}

// hxc281FuncLabel renders "Receiver.Method" for a method, or "Func" for a
// plain function.
func hxc281FuncLabel(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	expr := fn.Recv.List[0].Type
	if star, isStar := expr.(*ast.StarExpr); isStar {
		expr = star.X
	}
	if id, isIdent := expr.(*ast.Ident); isIdent {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// TestHXC281_ConfidenceWriteSiteCensus pins WHICH methods hand a Confidence to
// a caller, so the fan-out cannot silently stop short again.
//
// It walks the AST rather than grepping: a regex over source text would drift
// on reformatting (a false alarm) and, worse, could be satisfied by a comment
// mentioning Confidence (a false clear). Two node shapes count as a write:
// `Confidence: x` inside a composite literal, and `something.Confidence = x`.
func TestHXC281_ConfidenceWriteSiteCensus(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller(0) failed; cannot locate main.go to census")
	mainPath := filepath.Join(filepath.Dir(thisFile), "main.go")

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, mainPath, nil, parser.SkipObjectResolution)
	require.NoErrorf(t, err, "could not parse %s; the census cannot run", mainPath)

	lines := map[string][]int{}
	for _, decl := range parsed.Decls {
		fn, isFn := decl.(*ast.FuncDecl)
		if !isFn || fn.Body == nil {
			continue
		}
		label := hxc281FuncLabel(fn)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.KeyValueExpr:
				if key, isIdent := node.Key.(*ast.Ident); isIdent && key.Name == "Confidence" {
					lines[label] = append(lines[label], fset.Position(node.Pos()).Line)
				}
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					sel, isSel := lhs.(*ast.SelectorExpr)
					if isSel && sel.Sel.Name == "Confidence" {
						lines[label] = append(lines[label], fset.Position(lhs.Pos()).Line)
					}
				}
			}
			return true
		})
	}

	got := map[string]int{}
	for label, at := range lines {
		sort.Ints(at)
		got[label] = len(at)
	}

	require.Equalf(t, hxc281ConfidenceWriteCensus, got,
		"the set of Confidence-write sites in main.go no longer matches the pinned census.\n\n"+
			"OBSERVED (method -> source lines):\n%s\n"+
			"GUARDED BY:\n%s\n"+
			"This is a TRIPWIRE, not a coverage claim (§11.4.6): it says the site set moved, "+
			"not that anything is broken. Do NOT simply re-pin the numbers to make it green "+
			"(§11.4.120: reconcile a gate, never fake-pass it). A NEW site means a new place a "+
			"fabricated confidence can reach a caller — give it a behavioural guard in the "+
			"style of the tests in this file, add it to hxc281GuardedBy, and only then update "+
			"the census. A REMOVED site means a guard is now asserting against code that no "+
			"longer exists and must be retired with it. Forensic anchor (HXC-281): the last "+
			"time this set was assumed rather than measured, LLMProviderServer.CompleteStream "+
			"went unguarded and a fabricated 0.7 confidence survived the whole module.",
		hxc281FormatLines(lines), hxc281FormatGuards())
}

func hxc281FormatLines(lines map[string][]int) string {
	labels := make([]string, 0, len(lines))
	for label := range lines {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	var out strings.Builder
	for _, label := range labels {
		fmt.Fprintf(&out, "  %-34s %v\n", label, lines[label])
	}
	return out.String()
}

func hxc281FormatGuards() string {
	labels := make([]string, 0, len(hxc281GuardedBy))
	for label := range hxc281GuardedBy {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	var out strings.Builder
	for _, label := range labels {
		fmt.Fprintf(&out, "  %-34s %s\n", label, hxc281GuardedBy[label])
	}
	return out.String()
}

// TestHXC281_ProviderCompleteStreamChunksCarryOnlyProviderSuppliedConfidence is
// the third streaming guard: the copy of the round-29 code living in the OTHER
// gRPC service.
//
// It is not a duplicate of the facade guards. LLMProviderServer is a distinct
// service with its own registration, its own metrics recorder, and its own
// chunking loop; a fabrication there is invisible to every facade-side
// assertion, which is precisely what the HXC-281 mutation demonstrated.
func TestHXC281_ProviderCompleteStreamChunksCarryOnlyProviderSuppliedConfidence(t *testing.T) {
	req := &pb.CompletionRequest{SessionId: "hxc281", Prompt: "stream several chunks"}

	if hxc281RedMode() {
		// The registry-less construction, matching the RED arm of the two
		// facade guards so all three copies are reproduced the same way.
		server := NewLLMProviderServer(nil)
		stream := &captureCompletionStream{ctx: context.Background()}

		err := server.CompleteStream(req, stream)
		require.Error(t, err,
			"RED_MODE=1: a registry-less provider service must refuse before it chunks "+
				"anything; a success here means the reproduction no longer reproduces the "+
				"condition that makes a per-chunk assertion vacuous")
		require.Empty(t, stream.chunks,
			"RED_MODE=1: the registry-less provider path's chunk list MUST be empty — that "+
				"emptiness is the condition under which a per-chunk assertion silently "+
				"never executes, which is the HXC-281 defect this guard is built to avoid "+
				"repeating in the third copy")
		return
	}

	server := NewLLMProviderServer(hxc281Registry(t))
	stream := &captureCompletionStream{ctx: context.Background()}

	require.NoError(t, server.CompleteStream(req, stream),
		"LLMProviderServer.CompleteStream failed while the registry held a working provider")

	chunks := make([]hxc281Chunk, 0, len(stream.chunks))
	for _, c := range stream.chunks {
		chunks = append(chunks, hxc281Chunk{
			content:      c.Content,
			confidence:   c.Confidence,
			providerName: c.ProviderName,
		})
	}
	hxc281AssertStreamedChunks(t, "LLMProviderServer.CompleteStream", chunks)
}

// TestHXC281_UnaryCompleteCarriesOnlyProviderSuppliedConfidence guards the two
// unary siblings, which hand a Confidence straight back to the caller.
//
// The streaming paths got the attention because the round-29 fabrication lived
// there. But a unary Complete is the call an integrator makes FIRST, and its
// Confidence is read exactly the same way — as the provider's own self-reported
// confidence in the content returned. Both were provably unasserted: mutating
// them to a constant 0.7 compiled and passed the whole module.
func TestHXC281_UnaryCompleteCarriesOnlyProviderSuppliedConfidence(t *testing.T) {
	req := &pb.CompletionRequest{SessionId: "hxc281", Prompt: "What is 2+2?"}

	// Both services expose the same unary shape over the same registry, so one
	// table keeps them checked identically — the asymmetry that produced
	// HXC-281 was exactly one method being held to a weaker standard than its
	// sibling.
	for _, svc := range []struct {
		name     string
		complete func(*testing.T, bool) (*pb.CompletionResponse, error)
	}{
		{
			name: "LLMFacadeServer",
			complete: func(t *testing.T, wired bool) (*pb.CompletionResponse, error) {
				if !wired {
					return NewLLMFacadeServer(nil).Complete(context.Background(), req)
				}
				return NewLLMFacadeServer(hxc281Registry(t)).Complete(context.Background(), req)
			},
		},
		{
			name: "LLMProviderServer",
			complete: func(t *testing.T, wired bool) (*pb.CompletionResponse, error) {
				if !wired {
					return NewLLMProviderServer(nil).Complete(context.Background(), req)
				}
				return NewLLMProviderServer(hxc281Registry(t)).Complete(context.Background(), req)
			},
		},
	} {
		t.Run(svc.name, func(t *testing.T) {
			if hxc281RedMode() {
				resp, err := svc.complete(t, false)
				require.Error(t, err,
					"RED_MODE=1: a registry-less %s must refuse the completion", svc.name)
				require.NotNil(t, resp,
					"RED_MODE=1: %s returns a response value alongside the error; if that "+
						"stopped being true the assertions below no longer describe this "+
						"code path", svc.name)
				// The reproduction: on this path Confidence is the hardcoded
				// error-path 0, never a provider's value. Any equality check
				// against a provider-supplied confidence would therefore be
				// unfalsifiable here — which is why the pre-existing unary
				// tests, all of which build the server exactly like this, never
				// made one.
				require.Equal(t, float64(0), resp.Confidence,
					"RED_MODE=1: the registry-less error path must surface confidence 0, not "+
						"a fabricated score")
				require.Empty(t, resp.Content,
					"RED_MODE=1: the registry-less error path must surface no content, which "+
						"is what leaves a provider-supplied confidence unobservable")
				return
			}

			resp, err := svc.complete(t, true)
			require.NoErrorf(t, err,
				"%s.Complete failed while the registry held a working provider", svc.name)

			// Equality against the test-local constant, exactly as the streaming
			// guards do. Not "non-zero" and not "not 0.85": a fabricated 0.7
			// would satisfy both.
			require.Equalf(t, hxc281Confidence, resp.Confidence,
				"%s.Complete returned confidence %v, but the only provider serving this "+
					"request reported %v. A confidence the provider never supplied is "+
					"fabricated (§11.4 anti-bluff).", svc.name, resp.Confidence, hxc281Confidence)

			require.NotEqualf(t, hxc281HistoricalFabricatedConfidence, resp.Confidence,
				"%s.Complete returned the round-29 fabricated 0.85 confidence", svc.name)

			// Ties the confidence to a response that genuinely came from the
			// registered provider. Without this, a server that returned the
			// right number for the wrong reason would pass.
			require.Equalf(t, hxc281Content, resp.Content,
				"%s.Complete did not return the registered provider's output, so the "+
					"confidence above was not read off a genuinely served response", svc.name)
			require.Equalf(t, hxc281ProviderName, resp.ProviderName,
				"%s.Complete is not attributed to the registered provider", svc.name)
		})
	}
}
