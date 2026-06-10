package kodu

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.agent/internal/clis/agents/base"
)

// ---------------------------------------------------------------------------
// D-17 STUB-BLUFF PIN GUARD — RED-on-broken-artifact + GREEN regression guard
// (§11.4.115 polarity switch / §11.4.135 standing guard)
//
// HISTORY: Kodu.ask/explain/refactor USED to return FABRICATED natural-language
// output ("Based on the codebase: <q>", "This file <f> contains...", a templated
// change list) WITHOUT any LLM call — a stub bluff per BLUFF-001 / CONST-035.
//
// FIX (D-17): Kodu's semantic INDEX is real; ask/explain/refactor now return an
// HONEST error (ErrNoLLMBackend) because no LLM backend is wired. The real index
// commands (index/search/navigate) keep working against a real on-disk tree.
// ---------------------------------------------------------------------------

func TestD17_Kodu_NoFabricatedNaturalLanguage(t *testing.T) {
	k := New()
	ctx := context.Background()

	ares, aerr := k.ask(ctx, map[string]interface{}{"question": "What does this do?"})
	if aerr == nil {
		t.Fatalf("D17 REGRESSION: Kodu.ask returned success %v with no LLM backend — must return an honest error (BLUFF-001 reintroduced?).", ares)
	}
	if !errors.Is(aerr, ErrNoLLMBackend) {
		t.Fatalf("D17: Kodu.ask error should wrap ErrNoLLMBackend, got: %v", aerr)
	}

	rres, rerr := k.refactor(ctx, map[string]interface{}{"file": "main.go", "instruction": "extract"})
	if rerr == nil {
		t.Fatalf("D17 REGRESSION: Kodu.refactor returned success %v with no LLM backend — must return an honest error.", rres)
	}
	if !errors.Is(rerr, ErrNoLLMBackend) {
		t.Fatalf("D17: Kodu.refactor error should wrap ErrNoLLMBackend, got: %v", rerr)
	}
}

// TestD17_Kodu_RealIndexStillWorks proves the de-bluff did NOT break the genuine
// semantic-index commands: indexing a REAL on-disk Go tree must populate real
// files/symbols and search/navigate must read them back.
func TestD17_Kodu_RealIndexStillWorks(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "sample.go")
	const code = "package sample\n\nfunc UniqueKoduSymbol() int { return 7 }\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatalf("write sample.go: %v", err)
	}

	k := New()
	ctx := context.Background()
	if err := k.Initialize(ctx, &Config{BaseConfig: base.BaseConfig{WorkDir: work}}); err != nil {
		t.Fatalf("init: %v", err)
	}

	idx, err := k.index(ctx, map[string]interface{}{"directory": work})
	if err != nil {
		t.Fatalf("index returned error: %v", err)
	}
	im, _ := idx.(map[string]interface{})
	if files, _ := im["files"].(int); files < 1 {
		t.Fatalf("D17 REGRESSION: index found %v files in a real tree containing sample.go — real indexing broke.", im["files"])
	}

	// navigate must find the REAL symbol extracted from the real file. The
	// (simplistic) extractor stores the token after `func`, i.e. the name WITH
	// its parens ("UniqueKoduSymbol()") — we assert against the REAL stored name,
	// not an assumed one (anti-bluff).
	nav, err := k.navigate(ctx, map[string]interface{}{"symbol": "UniqueKoduSymbol()"})
	if err != nil {
		t.Fatalf("navigate returned error: %v", err)
	}
	nm, _ := nav.(map[string]interface{})
	if found, _ := nm["found"].(bool); !found {
		t.Fatalf("D17 REGRESSION: navigate did not find the real indexed symbol 'UniqueKoduSymbol()' — real navigation broke.")
	}

	// search must find the real content on disk.
	sr, err := k.search(ctx, map[string]interface{}{"query": "UniqueKoduSymbol"})
	if err != nil {
		t.Fatalf("search returned error: %v", err)
	}
	sm, _ := sr.(map[string]interface{})
	if cnt, _ := sm["count"].(int); cnt < 1 {
		t.Fatalf("D17 REGRESSION: search found %v matches for real on-disk content — real search broke.", sm["count"])
	}
}

// TestD17_Kodu_AskIsStubBluff — §11.4.115 RED-on-broken-artifact, RED_MODE=1.
func TestD17_Kodu_AskIsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"The standing GREEN guard is TestD17_Kodu_NoFabricatedNaturalLanguage.")
	}
	k := New()
	res, err := k.ask(context.Background(), map[string]interface{}{"question": "What does this do?"})
	if err != nil {
		return
	}
	m, _ := res.(map[string]interface{})
	if a, _ := m["answer"].(string); strings.HasPrefix(a, "Based on the codebase: ") {
		t.Fatalf("D17 BLUFF PINNED: Kodu.ask returned the fabricated literal %q without any LLM call (BLUFF-001).", a)
	}
}
