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
package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestRetrieveDocuments_ReturnsMilvusNotWiredSentinel asserts that
// retrieveDocuments surfaces ErrMilvusRetrievalNotWired rather than
// the pre-round-29 silent []Source{} no-op.
func TestRetrieveDocuments_ReturnsMilvusNotWiredSentinel(t *testing.T) {
	t.Parallel()

	cfg := &Config{MilvusHost: "milvus", MilvusPort: 19530}
	srv := NewServer(cfg)

	got, err := srv.retrieveDocuments(context.Background(), []float32{0.1, 0.2}, "docs", 5)
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
