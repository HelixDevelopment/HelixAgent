// Round-29 §11.4 anti-bluff regression tests for the rag-bridge service.
//
// Fix 1 + Fix 2 forensic anchor: prior to round-29, retrieveDocuments
// returned []Source{} unconditionally with the comment "Simplified
// retrieval - in production this would query Milvus" and rerankSources
// returned the input sources unchanged while pretending rerank had
// occurred. Both were CRITICAL §11.4 contract-bluffs at the RAG
// bridge layer (caller could not distinguish "no matches" from
// "Milvus never asked", and "reranked output" from "unmodified
// input"). The fixes replace the bluffs with named sentinel errors
// (ErrMilvusRetrievalNotWired, ErrRerankerNotWired). These tests
// pin that contract so any future revert is caught by CI.
//
// Round-77 §11.4 anti-bluff extension tests follow the round-67
// Harmony OS template: the round-29 sentinels are PRESERVED as the
// no-backend default; new VectorBackend / RerankerBackend
// injection-point tests prove (a) delegation when injected, (b)
// sentinel still fires when no backend, (c) Set...Backend(nil)
// reverts to no-injection state, (d) re-injection wins.
package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

// TestRetrieveDocuments_ReturnsMilvusNotWiredSentinel asserts that
// retrieveDocuments surfaces ErrMilvusRetrievalNotWired rather than
// the pre-round-29 silent []Source{} no-op when NO VectorBackend has
// been injected (round-77: preserves the round-29 canary).
func TestRetrieveDocuments_ReturnsMilvusNotWiredSentinel(t *testing.T) {
	t.Parallel()

	cfg := &Config{MilvusHost: "milvus", MilvusPort: 19530}
	srv := NewServer(cfg)

	got, err := srv.retrieveDocuments(context.Background(), "what is 2+2", []float32{0.1, 0.2}, "docs", 5)
	if err == nil {
		t.Fatalf("retrieveDocuments returned (sources=%v, err=nil) — the round-29 sentinel was lost; the pre-round-29 bluff (silent empty []Source{}) has been reintroduced", got)
	}
	if !errors.Is(err, ErrMilvusRetrievalNotWired) {
		t.Fatalf("retrieveDocuments error did not wrap ErrMilvusRetrievalNotWired: got %v", err)
	}
	if got != nil {
		t.Fatalf("retrieveDocuments must return nil sources alongside the sentinel; got %v", got)
	}
	if !strings.Contains(err.Error(), "collection=\"docs\"") {
		t.Fatalf("error message must include caller context (collection name); got %q", err.Error())
	}
}

// TestRerankSources_ReturnsRerankerNotWiredSentinel asserts that
// rerankSources surfaces ErrRerankerNotWired when the reranker IS
// configured (URL set, type != none) but the client wiring is
// missing — the pre-round-29 bluff was a silent pass-through.
func TestRerankSources_ReturnsRerankerNotWiredSentinel(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		RerankerURL:  "http://reranker:80",
		RerankerType: "nemotron",
	}
	srv := NewServer(cfg)

	input := []Source{{ID: "a", Content: "first"}, {ID: "b", Content: "second"}}
	got, err := srv.rerankSources(context.Background(), "q", input, 1)
	if err == nil {
		t.Fatalf("rerankSources returned (sources=%v, err=nil) — the round-29 sentinel was lost; the pre-round-29 bluff (silent pass-through pretending rerank had occurred) has been reintroduced", got)
	}
	if !errors.Is(err, ErrRerankerNotWired) {
		t.Fatalf("rerankSources error did not wrap ErrRerankerNotWired: got %v", err)
	}
	if got != nil {
		t.Fatalf("rerankSources must return nil sources alongside the sentinel; got %v", got)
	}
}

// TestRerankSources_DisabledPassesThroughIntentionally asserts that
// the legitimate "rerank disabled by config" path (RerankerURL == ""
// OR RerankerType == "none") is NOT broken by the sentinel fix —
// the caller's contract for disabled rerank is "return input
// unchanged, no error".
func TestRerankSources_DisabledPassesThroughIntentionally(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cfg  *Config
	}{
		{"empty_url", &Config{RerankerURL: "", RerankerType: "nemotron"}},
		{"type_none", &Config{RerankerURL: "http://r", RerankerType: "none"}},
		{"both", &Config{RerankerURL: "", RerankerType: "none"}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := NewServer(tc.cfg)
			input := []Source{{ID: "a"}, {ID: "b"}}
			got, err := srv.rerankSources(context.Background(), "q", input, 5)
			if err != nil {
				t.Fatalf("rerankSources with reranker disabled must return (input, nil); got err=%v", err)
			}
			if len(got) != len(input) {
				t.Fatalf("rerankSources with reranker disabled must pass input through; got len=%d, want %d", len(got), len(input))
			}
		})
	}
}

// -- Round-77 §11.4 anti-bluff extension tests --------------------------
//
// These tests exercise the VectorBackend / RerankerBackend injection
// points added in round-77. They cover:
//
//   - Injected backend → delegate (Retrieve / Rerank)
//   - No injected backend → round-29 sentinel fires (canary above)
//   - SetXxxBackend(nil) reverts to no-injection state
//   - Re-injection: second call wins
//   - Backend error propagation (wrapped, not swallowed)
//   - End-to-end RAG flow with both backends injected (in-process)
//
// All mocks live exclusively in this *_test.go file per CONST-050(A).

// fakeVectorBackend is a unit-test-only VectorBackend stub. Mocks are
// permitted ONLY in unit-test files per CONST-050(A) — this file is
// a unit test (no -tags=integration), so the stub is compliant.
type fakeVectorBackend struct {
	calls   atomic.Int32
	gotQ    atomic.Value // string
	gotK    atomic.Int32
	respond []Source
	err     error
}

func (f *fakeVectorBackend) Retrieve(ctx context.Context, query string, k int) ([]Source, error) {
	f.calls.Add(1)
	f.gotQ.Store(query)
	f.gotK.Store(int32(k))
	if f.err != nil {
		return nil, f.err
	}
	out := make([]Source, len(f.respond))
	copy(out, f.respond)
	return out, nil
}

// fakeRerankerBackend is a unit-test-only RerankerBackend stub.
type fakeRerankerBackend struct {
	calls   atomic.Int32
	gotQ    atomic.Value // string
	respond []Source
	err     error
}

func (f *fakeRerankerBackend) Rerank(ctx context.Context, query string, candidates []Source) ([]Source, error) {
	f.calls.Add(1)
	f.gotQ.Store(query)
	if f.err != nil {
		return nil, f.err
	}
	out := make([]Source, len(f.respond))
	copy(out, f.respond)
	return out, nil
}

// TestVectorBackend_Injected_DelegatesRetrieve asserts that when a
// VectorBackend is injected, retrieveDocuments delegates to it and
// the round-29 sentinel does NOT fire.
func TestVectorBackend_Injected_DelegatesRetrieve(t *testing.T) {
	t.Parallel()

	srv := NewServer(&Config{})
	want := []Source{
		{ID: "doc-1", Document: "a.pdf", Content: "alpha content", Score: 0.91, Page: 1},
		{ID: "doc-2", Document: "b.pdf", Content: "beta content", Score: 0.77, Page: 3},
	}
	be := &fakeVectorBackend{respond: want}
	srv.SetVectorBackend(be)

	got, err := srv.retrieveDocuments(context.Background(), "what is alpha", []float32{0.1, 0.2}, "docs", 7)
	if err != nil {
		t.Fatalf("retrieveDocuments with injected backend must succeed; got err=%v", err)
	}
	if be.calls.Load() != 1 {
		t.Fatalf("backend Retrieve must be called exactly once; got %d", be.calls.Load())
	}
	if q := be.gotQ.Load().(string); q != "what is alpha" {
		t.Fatalf("backend received wrong query: got %q want %q", q, "what is alpha")
	}
	if k := be.gotK.Load(); k != 7 {
		t.Fatalf("backend received wrong topK: got %d want 7", k)
	}
	if len(got) != 2 {
		t.Fatalf("retrieveDocuments must return backend output; got len=%d want 2", len(got))
	}
	if got[0].ID != "doc-1" || got[1].ID != "doc-2" {
		t.Fatalf("retrieveDocuments preserved backend order: got [%s, %s]", got[0].ID, got[1].ID)
	}
}

// TestVectorBackend_Injected_ErrorPropagates asserts that a non-nil
// error from the injected VectorBackend is wrapped (not swallowed,
// not converted to the round-29 sentinel).
func TestVectorBackend_Injected_ErrorPropagates(t *testing.T) {
	t.Parallel()

	srv := NewServer(&Config{})
	sentinel := errors.New("milvus: connection refused")
	srv.SetVectorBackend(&fakeVectorBackend{err: sentinel})

	got, err := srv.retrieveDocuments(context.Background(), "q", nil, "docs", 5)
	if err == nil {
		t.Fatalf("retrieveDocuments with backend-error must propagate error; got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("retrieveDocuments must wrap backend error; got %v", err)
	}
	if errors.Is(err, ErrMilvusRetrievalNotWired) {
		t.Fatalf("retrieveDocuments must NOT mistake backend-error for not-wired sentinel; got %v", err)
	}
	if got != nil {
		t.Fatalf("retrieveDocuments must return nil sources on error; got %v", got)
	}
}

// TestSetVectorBackend_NilImplRevertsToSentinel asserts that
// installing nil deliberately UN-installs a previously-injected
// backend and restores the round-29 sentinel contract.
func TestSetVectorBackend_NilImplRevertsToSentinel(t *testing.T) {
	t.Parallel()

	srv := NewServer(&Config{})
	srv.SetVectorBackend(&fakeVectorBackend{respond: []Source{{ID: "x"}}})

	// First call: backend is installed, no sentinel.
	if _, err := srv.retrieveDocuments(context.Background(), "q", nil, "docs", 1); err != nil {
		t.Fatalf("with backend installed: expected nil err, got %v", err)
	}

	// Un-install.
	srv.SetVectorBackend(nil)

	// Second call: sentinel must fire again.
	_, err := srv.retrieveDocuments(context.Background(), "q", nil, "docs", 1)
	if !errors.Is(err, ErrMilvusRetrievalNotWired) {
		t.Fatalf("after SetVectorBackend(nil), sentinel must re-fire; got %v", err)
	}
}

// TestSetVectorBackend_PreservesPriorInjection asserts that re-
// injecting a different backend swaps the implementation (second
// wins, not first).
func TestSetVectorBackend_PreservesPriorInjection(t *testing.T) {
	t.Parallel()

	srv := NewServer(&Config{})
	first := &fakeVectorBackend{respond: []Source{{ID: "first"}}}
	second := &fakeVectorBackend{respond: []Source{{ID: "second"}}}

	srv.SetVectorBackend(first)
	srv.SetVectorBackend(second)

	got, err := srv.retrieveDocuments(context.Background(), "q", nil, "docs", 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if first.calls.Load() != 0 {
		t.Fatalf("first backend MUST NOT be called after swap; got calls=%d", first.calls.Load())
	}
	if second.calls.Load() != 1 {
		t.Fatalf("second backend MUST be called after swap; got calls=%d", second.calls.Load())
	}
	if len(got) != 1 || got[0].ID != "second" {
		t.Fatalf("response came from wrong backend; got %v", got)
	}
}

// TestRerankerBackend_Injected_DelegatesRerank asserts that when a
// RerankerBackend is injected and the reranker is configured (URL
// set, type != none), rerankSources delegates to it.
func TestRerankerBackend_Injected_DelegatesRerank(t *testing.T) {
	t.Parallel()

	cfg := &Config{RerankerURL: "http://reranker:80", RerankerType: "nemotron"}
	srv := NewServer(cfg)
	want := []Source{
		{ID: "best", Content: "best match", Score: 0.99},
		{ID: "ok", Content: "ok match", Score: 0.42},
	}
	be := &fakeRerankerBackend{respond: want}
	srv.SetRerankerBackend(be)

	in := []Source{
		{ID: "ok", Content: "ok match", Score: 0.7},
		{ID: "best", Content: "best match", Score: 0.6},
	}
	got, err := srv.rerankSources(context.Background(), "best?", in, 2)
	if err != nil {
		t.Fatalf("rerankSources with injected backend must succeed; got err=%v", err)
	}
	if be.calls.Load() != 1 {
		t.Fatalf("backend Rerank must be called exactly once; got %d", be.calls.Load())
	}
	if q := be.gotQ.Load().(string); q != "best?" {
		t.Fatalf("backend received wrong query: got %q want %q", q, "best?")
	}
	if len(got) != 2 || got[0].ID != "best" {
		t.Fatalf("rerankSources must return backend order; got %v", got)
	}
}

// TestRerankerBackend_Injected_TrimsToTopK asserts that even when
// the backend returns more than topK candidates, rerankSources trims
// to caller's topK.
func TestRerankerBackend_Injected_TrimsToTopK(t *testing.T) {
	t.Parallel()

	cfg := &Config{RerankerURL: "http://reranker:80", RerankerType: "nemotron"}
	srv := NewServer(cfg)
	srv.SetRerankerBackend(&fakeRerankerBackend{
		respond: []Source{
			{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"},
		},
	})

	got, err := srv.rerankSources(context.Background(), "q", []Source{{ID: "in"}}, 3)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("rerankSources must trim to topK=3; got len=%d", len(got))
	}
	if got[0].ID != "a" || got[2].ID != "c" {
		t.Fatalf("rerankSources must preserve backend order pre-trim; got %v", got)
	}
}

// TestRerankerBackend_Injected_ErrorPropagates asserts that a
// non-nil error from the injected RerankerBackend is wrapped (not
// swallowed, not converted to the round-29 sentinel).
func TestRerankerBackend_Injected_ErrorPropagates(t *testing.T) {
	t.Parallel()

	cfg := &Config{RerankerURL: "http://reranker:80", RerankerType: "nemotron"}
	srv := NewServer(cfg)
	sentinel := errors.New("cohere: 429 rate limited")
	srv.SetRerankerBackend(&fakeRerankerBackend{err: sentinel})

	got, err := srv.rerankSources(context.Background(), "q", []Source{{ID: "x"}}, 1)
	if err == nil {
		t.Fatalf("rerankSources with backend-error must propagate; got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("rerankSources must wrap backend error; got %v", err)
	}
	if errors.Is(err, ErrRerankerNotWired) {
		t.Fatalf("rerankSources must NOT mistake backend-error for not-wired sentinel; got %v", err)
	}
	if got != nil {
		t.Fatalf("rerankSources must return nil sources on error; got %v", got)
	}
}

// TestRerankerBackend_NotInjected_ReturnsRound29Sentinel preserves
// the round-29 canary: with reranker configured but NO backend
// injected, ErrRerankerNotWired must still fire.
func TestRerankerBackend_NotInjected_ReturnsRound29Sentinel(t *testing.T) {
	t.Parallel()

	cfg := &Config{RerankerURL: "http://reranker:80", RerankerType: "nemotron"}
	srv := NewServer(cfg)

	got, err := srv.rerankSources(context.Background(), "q", []Source{{ID: "x"}}, 1)
	if err == nil {
		t.Fatalf("with no RerankerBackend injected: round-29 sentinel was lost (got nil err, sources=%v)", got)
	}
	if !errors.Is(err, ErrRerankerNotWired) {
		t.Fatalf("rerankSources error did not wrap ErrRerankerNotWired: got %v", err)
	}
	if got != nil {
		t.Fatalf("rerankSources must return nil sources alongside the sentinel; got %v", got)
	}
}

// TestSetRerankerBackend_NilImplRevertsToSentinel asserts that
// installing nil deliberately UN-installs a previously-injected
// reranker backend.
func TestSetRerankerBackend_NilImplRevertsToSentinel(t *testing.T) {
	t.Parallel()

	cfg := &Config{RerankerURL: "http://reranker:80", RerankerType: "nemotron"}
	srv := NewServer(cfg)
	srv.SetRerankerBackend(&fakeRerankerBackend{respond: []Source{{ID: "x"}}})

	// With backend: success.
	if _, err := srv.rerankSources(context.Background(), "q", []Source{{ID: "in"}}, 1); err != nil {
		t.Fatalf("with backend installed: expected nil err, got %v", err)
	}

	// Un-install.
	srv.SetRerankerBackend(nil)

	// Sentinel re-fires.
	_, err := srv.rerankSources(context.Background(), "q", []Source{{ID: "in"}}, 1)
	if !errors.Is(err, ErrRerankerNotWired) {
		t.Fatalf("after SetRerankerBackend(nil), sentinel must re-fire; got %v", err)
	}
}

// TestEndToEndRAGFlow_BothBackendsInjected exercises the full
// retrieve -> rerank pipeline with both backends injected. This is
// the round-77 §11.4 anti-bluff positive runtime evidence:
// retrieveDocuments hits the injected VectorBackend, rerankSources
// hits the injected RerankerBackend, and the caller observes
// post-rerank Sources (NOT the pre-round-29 silent pass-through).
//
// This is an in-process test (no real Milvus / no real Cohere) — it
// proves the wiring contract, not the providers. Round 78+ ships
// challenges that exercise real backends end-to-end per CONST-050(B).
func TestEndToEndRAGFlow_BothBackendsInjected(t *testing.T) {
	t.Parallel()

	cfg := &Config{RerankerURL: "http://reranker:80", RerankerType: "nemotron"}
	srv := NewServer(cfg)

	// Vector backend returns 4 candidates.
	srv.SetVectorBackend(&fakeVectorBackend{respond: []Source{
		{ID: "v1", Content: "weak1", Score: 0.3},
		{ID: "v2", Content: "weak2", Score: 0.4},
		{ID: "v3", Content: "weak3", Score: 0.5},
		{ID: "v4", Content: "weak4", Score: 0.6},
	}})
	// Reranker backend re-orders and assigns rerank scores.
	srv.SetRerankerBackend(&fakeRerankerBackend{respond: []Source{
		{ID: "v3", Content: "weak3", Score: 0.99},
		{ID: "v1", Content: "weak1", Score: 0.85},
	}})

	ctx := context.Background()

	// Retrieve.
	cand, err := srv.retrieveDocuments(ctx, "find me weak3", nil, "docs", 4)
	if err != nil {
		t.Fatalf("retrieveDocuments unexpectedly failed: %v", err)
	}
	if len(cand) != 4 {
		t.Fatalf("expected 4 candidates from VectorBackend; got %d", len(cand))
	}

	// Rerank.
	reranked, err := srv.rerankSources(ctx, "find me weak3", cand, 2)
	if err != nil {
		t.Fatalf("rerankSources unexpectedly failed: %v", err)
	}
	if len(reranked) != 2 {
		t.Fatalf("expected 2 reranked sources; got %d", len(reranked))
	}
	if reranked[0].ID != "v3" || reranked[0].Score < 0.9 {
		t.Fatalf("rerank winner must be v3 with high score; got id=%s score=%f", reranked[0].ID, reranked[0].Score)
	}
	if reranked[1].ID != "v1" {
		t.Fatalf("rerank runner-up must be v1; got id=%s", reranked[1].ID)
	}
}

// TestVectorBackend_DefensiveNilCoercion asserts that a backend
// returning (nil, nil) — forbidden by the interface contract but
// possible from buggy implementations — is coerced to (empty, nil)
// at the retrieveDocuments boundary so callers reliably distinguish
// "no matches" from a nil-deref panic.
func TestVectorBackend_DefensiveNilCoercion(t *testing.T) {
	t.Parallel()

	srv := NewServer(&Config{})
	srv.SetVectorBackend(&fakeVectorBackend{respond: nil}) // returns nil slice

	got, err := srv.retrieveDocuments(context.Background(), "q", nil, "docs", 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil {
		t.Fatalf("nil-slice from backend must be coerced to empty-non-nil; got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice; got len=%d", len(got))
	}
}
