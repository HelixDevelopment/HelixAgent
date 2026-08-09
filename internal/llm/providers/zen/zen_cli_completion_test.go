package zen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/models"
)

// Guards for two defects in ZenCLIProvider.Complete (§11.4.115).
//
// BOTH are reproduced here against a FAKE CLI rather than the real `opencode`
// binary, deliberately: the fake reproduces the exact behaviour MEASURED from
// the real one on 2026-08-09, and does it deterministically, offline, and in
// seconds instead of minutes (§11.4.50 — a guard that needs a live LLM is not
// a guard, it is a coin flip).
//
// WHAT WAS MEASURED (real `opencode run … --format json --model
// opencode/big-pickle`, captured 2026-08-09):
//
//	FIRST_STDOUT_BYTE_AT   = 15.98 s
//	answer text "4" emitted at ~19.1 s   ({"type":"text",...,"text":"4"})
//	turn closed ~1.3 s later             ({"type":"step_finish",…"reason":"stop"})
//	then 6 further tool_use steps + 8 more text parts
//	VERDICT=STILL_RUNNING_AT_DEADLINE elapsed=150.3 s  → had to be SIGKILLed
//
// `opencode run` is an AGENTIC LOOP, not a one-shot completion: it answers and
// then keeps working. It did not exit within 150 s.
//
// DEFECT A — Complete() blocked on process exit.
//
//	It buffered stdout into a bytes.Buffer and called cmd.Run(), which returns
//	only when the child exits. With the answer available at ~19 s and the child
//	still alive at 150 s, Complete() could not return the answer: it waited out
//	the full 120 s p.timeout, the context then killed the child, and the
//	buffered answer was DISCARDED in favour of a timeout error. A correct
//	answer the user could have had in 19 s became a 2-minute failure.
//
// DEFECT B — the timeout message reported the wrong budget.
//
//	cmdCtx is context.WithTimeout(ctx, p.timeout), so its deadline is
//	min(parent deadline, now+p.timeout). When the PARENT deadline was the
//	binding one, the error still printed p.timeout — a caller who allowed 60 s
//	was told "opencode CLI timed out after 2m0s".
//
// POLARITY: RED_MODE=1 asserts each defect IS present (PASSes on the pre-fix
// tree — the proof these tests can see the bugs). RED_MODE=0, the committed
// default, is the standing regression guard.

func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// fakeCLI writes an executable shell script and returns its path.
func fakeCLI(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write fake CLI: %v", err)
	}
	return path
}

// providerWithCLI builds a provider bound to a fake CLI binary, with the
// availability check pre-satisfied so Complete() proceeds to exec.
func providerWithCLI(cliPath string, timeout time.Duration) *ZenCLIProvider {
	p := &ZenCLIProvider{
		model:           "test-model",
		cliPath:         cliPath,
		cliAvailable:    true,
		timeout:         timeout,
		maxOutputTokens: 4096,
		failedAPIModels: safe.NewStore[string, bool](),
	}
	p.cliCheckOnce.Do(func() {})
	return p
}

func userRequest(prompt string) *models.LLMRequest {
	return &models.LLMRequest{Messages: []models.Message{{Role: "user", Content: prompt}}}
}

// TestZenCLICompleteReturnsAnswerWithoutWaitingForExit covers DEFECT A.
//
// The fake CLI emits the same NDJSON turn shape the real binary emits, then
// sleeps WITHOUT exiting — exactly the measured real behaviour, compressed in
// time. A provider that waits for process exit cannot return before its
// timeout; one that reads the turn returns as soon as the turn closes.
func TestZenCLICompleteReturnsAnswerWithoutWaitingForExit(t *testing.T) {
	const providerTimeout = 6 * time.Second

	cli := fakeCLI(t, "opencode-fake", `
echo '{"type":"step_start","part":{"type":"step-start"}}'
echo '{"type":"text","part":{"type":"text","text":"4"}}'
echo '{"type":"step_finish","part":{"type":"step-finish","reason":"stop"}}'
# The real binary keeps running its agentic loop here and does NOT exit.
sleep 60
`)

	p := providerWithCLI(cli, providerTimeout)

	start := time.Now()
	resp, err := p.Complete(context.Background(), userRequest("What is 2+2?"))
	elapsed := time.Since(start)

	if redMode() {
		if err == nil {
			t.Fatalf("RED_MODE=1: expected Complete to fail by waiting out the %s "+
				"timeout, but it returned a response after %s — this test cannot "+
				"see the defect", providerTimeout, elapsed.Round(time.Millisecond))
		}
		if elapsed < providerTimeout-time.Second {
			t.Fatalf("RED_MODE=1: Complete failed after only %s, which is not the "+
				"blocked-until-timeout defect under test: %v",
				elapsed.Round(time.Millisecond), err)
		}
		t.Logf("RED_MODE=1: defect reproduced — the answer was on stdout within "+
			"milliseconds, yet Complete blocked %s waiting for process exit and "+
			"then discarded it: %v", elapsed.Round(time.Millisecond), err)
		return
	}

	if err != nil {
		t.Fatalf("Complete returned an error after %s: %v", elapsed.Round(time.Millisecond), err)
	}
	if resp == nil {
		t.Fatal("Complete returned a nil response and a nil error")
	}
	if got := strings.TrimSpace(resp.Content); got != "4" {
		t.Errorf("Content = %q, want %q (the assistant text from the turn, not the raw event log)", got, "4")
	}
	// The whole point: return when the ANSWER is done, not when the PROCESS is.
	if elapsed >= providerTimeout {
		t.Errorf("Complete took %s — it waited for the child to exit instead of "+
			"returning when the turn closed", elapsed.Round(time.Millisecond))
	}
	t.Logf("answer returned in %s (provider timeout %s, child still running)",
		elapsed.Round(time.Millisecond), providerTimeout)
}

// TestZenCLICompleteTimeoutReportsRealBudget covers DEFECT B: when the CALLER's
// deadline is shorter than p.timeout, the reported budget must be the caller's.
func TestZenCLICompleteTimeoutReportsRealBudget(t *testing.T) {
	const providerTimeout = 2 * time.Minute // the default; formats as "2m0s"
	const callerBudget = 2 * time.Second

	// Produces nothing and never exits: the only way out is a deadline.
	cli := fakeCLI(t, "opencode-silent", "sleep 60\n")
	p := providerWithCLI(cli, providerTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), callerBudget)
	defer cancel()

	_, err := p.Complete(ctx, userRequest("What is 2+2?"))
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	msg := err.Error()

	wrong := providerTimeout.String() // "2m0s"
	right := callerBudget.String()    // "2s"

	if redMode() {
		if !strings.Contains(msg, wrong) {
			t.Fatalf("RED_MODE=1: expected the message to misreport the budget as %q, "+
				"got %q — this test cannot see the defect", wrong, msg)
		}
		t.Logf("RED_MODE=1: defect reproduced — caller allowed %s but the error says "+
			"%q", right, msg)
		return
	}

	if strings.Contains(msg, wrong) {
		t.Errorf("timeout message reports the provider timeout %q, but the caller's "+
			"%s deadline was the binding one: %q", wrong, right, msg)
	}
	if !strings.Contains(msg, right) {
		t.Errorf("timeout message does not state the real %s budget: %q", right, msg)
	}
	t.Logf("timeout message states the real budget: %q", msg)
}

var _ = fmt.Sprintf // keep fmt imported for future message assertions
