package handlers

// D-20 — ID-generator collision regression guard (§11.4.115 RED-on-broken-artifact
// + polarity switch; §11.4.135 standing regression guard).
//
// Forensic root cause (D-10 → D-20): the ID generators in this package built IDs
// as fmt.Sprintf("..._%d", time.Now().UnixNano()). The host nanosecond clock is
// coarser than the call rate, so two creations within the same coarse clock tick
// produce IDENTICAL IDs (team / session / request / plan / task / tool-call). A
// tight same-tick loop reproduces tens-of-thousands of collisions in 200k calls.
//
// Polarity switch (RED_MODE env var):
//   RED_MODE=1 (default in this file's dedicated RED runner) → assert the defect
//             is PRESENT on the UnixNano-only artifact (collisions > 0). On the
//             FIXED artifact RED_MODE=1 fails to reproduce → that is itself a
//             reported finding (the fix took), so RED_MODE=1 is only meaningful
//             against the pre-fix code and is NOT run in the standing suite.
//   RED_MODE=0 (default everywhere) → the standing GREEN regression guard:
//             generate N IDs in a tight same-tick loop and assert ZERO duplicates.
//
// The same source is BOTH the bug-catcher and the standing guard (§11.4.115).

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redMode reports whether the test is running in defect-reproduction mode.
// Default 0 (standing GREEN guard). Set RED_MODE=1 to reproduce the historical
// collision on the pre-fix UnixNano-only artifact.
func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// genCollisionCount runs gen N times in the tightest possible loop (no clock
// wait, no sleep — the same-tick adversarial case) and returns how many of the
// produced IDs collided with an already-seen ID.
func genCollisionCount(gen func() string, n int) (collisions int, total int) {
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := gen()
		if _, dup := seen[id]; dup {
			collisions++
		}
		seen[id] = struct{}{}
	}
	return collisions, n
}

// idGenerators is the closed set of production ID generators under D-20.
// Each entry's func MUST be a single ID-producing call (the unit of collision).
func idGenerators(t *testing.T) map[string]func() string {
	t.Helper()

	// convertToInternalRequest produces BOTH the request ID and the session ID;
	// expose each via a fresh handler + fresh gin context per call so we measure
	// the generator, not context reuse.
	completionReqID := func() string {
		h := &CompletionHandler{}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		return h.convertToInternalRequest(&CompletionRequest{Prompt: "x"}, c).ID
	}
	completionSessionID := func() string {
		h := &CompletionHandler{}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		return h.convertToInternalRequest(&CompletionRequest{Prompt: "x"}, c).SessionID
	}

	return map[string]func() string{
		"generateTeamID":       generateTeamID,
		"generatePlanModeID":   generatePlanModeID,
		"generateTaskID":       generateTaskID,
		"generateToolCallID":   generateToolCallID,
		"completion.requestID": completionReqID,
		"completion.sessionID": completionSessionID,
	}
}

// historicallyDefective is the subset of generators that built IDs as
// fmt.Sprintf("..._%d", UnixNano()) and therefore collided under same-tick load.
// generateToolCallID is intentionally EXCLUDED — it already used a crypto-random
// suffix (utils.SecureRandomID) and never collided, so it has no defect to
// reproduce in RED_MODE=1. It is still covered by the GREEN guard below.
var historicallyDefective = map[string]bool{
	"generateTeamID":       true,
	"generatePlanModeID":   true,
	"generateTaskID":       true,
	"completion.requestID": true,
	"completion.sessionID": true,
}

// TestIDGenerators_NoCollisionUnderSameTickLoad is the standing GREEN regression
// guard (RED_MODE=0) and the defect reproducer (RED_MODE=1).
func TestIDGenerators_NoCollisionUnderSameTickLoad(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// N large enough that the coarse host clock cannot advance every iteration —
	// the exact condition that made the UnixNano-only generators collide.
	const n = 200_000

	for name, gen := range idGenerators(t) {
		name, gen := name, gen
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if redMode() && !historicallyDefective[name] {
				t.Skipf("SKIP-OK: RED_MODE=1: %q was never UnixNano-only (already crypto-random) "+
					"— no defect to reproduce; covered by the GREEN guard", name)
			}

			collisions, total := genCollisionCount(gen, n)

			if redMode() {
				// RED: prove the collision is genuinely present on the broken
				// artifact. A RED run that finds zero collisions is a blind test
				// (the fix already took) — reported via require so the operator
				// sees it, never silently passes.
				require.Positive(t, collisions,
					"RED_MODE=1: expected the UnixNano-only generator %q to COLLIDE "+
						"under a %d-call same-tick loop, but found zero collisions "+
						"(either the fix already landed, or the clock is fine-grained "+
						"on this host)", name, total)
				t.Logf("RED reproduced: %q produced %d collisions in %d same-tick calls",
					name, collisions, total)
				return
			}

			// GREEN guard: unique-by-construction means ZERO collisions even under
			// the tightest same-tick load.
			assert.Equal(t, 0, collisions,
				"generator %q produced %d duplicate IDs in %d same-tick calls — "+
					"IDs MUST be unique-by-construction (D-20)", name, collisions, total)
		})
	}
}

// TestIDGenerators_PrefixPreserved guards the human-readable / sortable prefixes
// the callers + sibling tests depend on (anthropic msg_, completion session_,
// ensemble team_) so the collision fix does not silently change wire formats.
func TestIDGenerators_PrefixPreserved(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	assert.Contains(t, generateTeamID(), "team_", "team ID must keep team_ prefix")
	assert.Contains(t, generatePlanModeID(), "plan_", "plan ID must keep plan_ prefix")
	assert.Contains(t, generateTaskID(), "task_", "task ID must keep task_ prefix")

	h := &CompletionHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	internal := h.convertToInternalRequest(&CompletionRequest{Prompt: "x"}, c)
	assert.Contains(t, internal.ID, "req_", "request ID must keep req_ prefix")
	assert.Contains(t, internal.SessionID, "session_", "session ID must keep session_ prefix")
}
