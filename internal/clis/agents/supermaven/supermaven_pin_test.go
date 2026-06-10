package supermaven

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 PIN GUARD — supermaven honest-error (§11.4.115 / §11.4.135)
//
// HISTORY: Supermaven.complete USED to return a templated completion from a
// hardcoded generateCompletion() switch ("// Supermaven completion", "params)
// {…}", etc.) WITHOUT any real engine call — a BLUFF-001 / CONST-035
// false-success.
//
// FIX: Supermaven ships ONLY as an IDE/editor plugin (language server) with no
// headless CLI, so complete now returns the honest ErrNoHeadlessCLI instead of
// fabricating a completion. accept/reject/status are honest local bookkeeping
// and remain.
//
// §1.1 PAIRED MUTATION: re-introducing a fabricated completion (making complete
// return err==nil with a "completion" payload) makes this guard FAIL.
// ---------------------------------------------------------------------------

func TestD17_Supermaven_CompleteIsHonestError(t *testing.T) {
	s := New()
	ctx := context.Background()

	res, err := s.complete(ctx, map[string]interface{}{
		"prefix":   "func main() {",
		"language": "go",
	})
	if err == nil {
		t.Fatalf("D-17 BLUFF: complete returned success %v — must return an honest error (no headless CLI), never a fabricated completion (BLUFF-001).", res)
	}
	if res != nil {
		t.Fatalf("D-17 BLUFF: complete returned a result payload %v — must be nil.", res)
	}
	// The error must explain the real reason (IDE-plugin, no headless CLI), not
	// be a generic failure that could mask a future re-bluff.
	if !strings.Contains(err.Error(), "headless") {
		t.Fatalf("D-17: complete error %q should cite the no-headless-CLI reason.", err)
	}
}

// TestD17_Supermaven_NoFabricatedCompletionLiteral pins that the old templated
// literals are never emitted via any language path.
func TestD17_Supermaven_NoFabricatedCompletionLiteral(t *testing.T) {
	s := New()
	ctx := context.Background()
	for _, lang := range []string{"go", "python", "typescript", "javascript", "unknown"} {
		res, err := s.complete(ctx, map[string]interface{}{"prefix": "x", "language": lang})
		if err == nil {
			t.Fatalf("D-17 BLUFF: complete(lang=%s) succeeded with %v — must be an honest error.", lang, res)
		}
	}
}
