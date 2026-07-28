package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/require"
)

// redModeGoroutineLeak reports whether the §11.4.115 RED polarity switch is
// engaged for this file's regression guard.
//
// RED_MODE=1 : reproduce-and-assert the goroutine leak IS PRESENT on the
//
//	pre-fix artifact.
//
// RED_MODE=0 (default, unset) : the standing GREEN regression-guard —
//
//	asserts the leak is ABSENT on the fixed artifact.
func redModeGoroutineLeak() bool {
	return os.Getenv("RED_MODE") == "1"
}

// leakSymbol is the fully-qualified symbol name that appears in a runtime
// goroutine-stack dump for the anonymous goroutine CompleteStream spawns.
// Module path per go.mod: "dev.helix.agent".
const leakSymbol = "dev.helix.agent/internal/llm/providers/ollama.(*OllamaProvider).CompleteStream.func1"

// goroutineParkedInCompleteStream dumps every live goroutine's stack via
// runtime.Stack(buf, all=true) and reports whether any goroutine is currently
// parked (blocked) inside CompleteStream's background closure. This is a
// deterministic, unforgeable liveness probe: it does not rely on counting
// runtime.NumGoroutine() (which would be polluted by unrelated goroutines
// from the test binary, the HTTP server, GC workers, etc.) — it looks for
// THIS SPECIFIC symbol, blocked in a channel-send state, on the live stack.
func goroutineParkedInCompleteStream() bool {
	buf := make([]byte, 1<<16)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	dump := string(buf)
	for _, goroutineBlock := range strings.Split(dump, "\n\n") {
		if strings.Contains(goroutineBlock, leakSymbol) && strings.Contains(goroutineBlock, "chan send") {
			return true
		}
	}
	return false
}

// waitUntilLeaked polls goroutineParkedInCompleteStream until it reports
// true or the deadline elapses. Early-exit-on-true is SOUND here: once the
// unguarded send blocks the goroutine forever, the leaked state never
// reverts to false on its own, so observing `true` once is conclusive.
func waitUntilLeaked(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if goroutineParkedInCompleteStream() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// notLeakedAfterSettling sleeps the FULL settle window (deliberately no
// early exit) and then takes a single snapshot. Early-exiting on an initial
// "not leaked" reading would be UNSOUND for the not-leaked assertion: a
// momentary false reading taken before the background goroutine has even
// attempted its channel send would falsely pass on genuinely broken
// (pre-fix) code. Waiting out the full window first gives the goroutine
// time to reach its steady state — either parked forever (pre-fix) or
// already exited via ctx.Done() (fixed) — before the single decisive read.
func notLeakedAfterSettling(settle time.Duration) bool {
	time.Sleep(settle)
	return goroutineParkedInCompleteStream()
}

// TestOllamaProvider_CompleteStream_GoroutineLeak_OnHTTPDoFailure is the
// §11.4.115 RED-on-broken-artifact + polarity-switch regression guard for a
// goroutine leak in CompleteStream.
//
// DEFECT (pre-fix, ollama.go CompleteStream ~lines 134-267): the background
// goroutine sends *models.LLMResponse values on an UNBUFFERED channel. Of its
// six send sites, four are unguarded blocking sends with NO ctx.Done() escape
// hatch — including the send that fires after httpClient.Do fails
// (~lines 183-195). The two correct sites (mid-stream decode error,
// ~lines 208-219; per-chunk send, ~lines 239-243) both use
// `select { case ch <- resp: case <-ctx.Done(): return }`.
//
// CONSEQUENCE: if the caller cancels ctx and stops draining the channel
// (exactly what a real caller does on cancellation/timeout — and exactly the
// scenario TestOllamaProvider_CompleteStream_ContextCancellation models, but
// that test happens to read the channel once, which masks this defect), the
// goroutine blocks FOREVER on the unguarded send: a goroutine leak holding
// its stack and captured references (httpReq, req, ollamaReq, o) alive
// indefinitely.
//
// REPRODUCTION STRATEGY: point the provider at an httptest server whose
// handler blocks (never responds) until the test's deferred cleanup runs.
// Cancel ctx shortly after starting CompleteStream so the in-flight
// httpClient.Do fails with a context error BEFORE any response arrives —
// driving the goroutine down the httpClient.Do-failure send site. The test
// then deliberately NEVER reads from the returned channel.
//
// RED_MODE=1: asserts a goroutine IS found parked in CompleteStream's
// closure, blocked in "chan send" state, after ctx cancellation — proving
// the defect is genuinely present on this (pre-fix) artifact.
// RED_MODE=0 (default): the standing GREEN guard — asserts NO such goroutine
// remains, because the fix wraps every send in
// `select { case ch <- resp: case <-ctx.Done(): return }`.
func TestOllamaProvider_CompleteStream_GoroutineLeak_OnHTTPDoFailure(t *testing.T) {
	// Baseline: confirm no stray goroutine from a prior test is already
	// sitting in CompleteStream's closure, so a positive result below is
	// unambiguously caused by THIS test's invocation.
	require.False(t, goroutineParkedInCompleteStream(),
		"precondition failed: a goroutine is already parked in CompleteStream's "+
			"closure before this test ran — cannot attribute a subsequent leak to "+
			"this test's invocation")

	// NOTE on defer order (deliberate): httptest.Server.Close() blocks until
	// every outstanding request's handler has returned. The handler below
	// blocks on <-unblock, so `unblock` MUST be closed BEFORE server.Close()
	// runs, or the test itself deadlocks. Since defers run LIFO, `defer
	// server.Close()` is registered FIRST (so it runs LAST) and `defer
	// close(unblock)` is registered SECOND (so it runs FIRST).
	unblock := make(chan struct{})
	requestReceived := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestReceived)
		<-unblock // never respond until the test's cleanup unblocks it
	}))
	defer server.Close()
	defer close(unblock)

	provider := NewOllamaProvider(server.URL, "llama2")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &models.LLMRequest{
		ID:     "goroutine-leak-test",
		Prompt: "test prompt",
	}

	ch, err := provider.CompleteStream(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Wait for the goroutine to actually reach the server (i.e. httpClient.Do
	// is genuinely in flight) before cancelling, so cancellation reliably
	// drives the httpClient.Do-failure send site rather than an earlier one.
	select {
	case <-requestReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("CompleteStream's goroutine never reached the test server — cannot drive the httpClient.Do-failure path")
	}

	cancel()

	// Deliberately DO NOT read from ch — this models exactly what a real
	// caller does on cancellation/timeout: cancel and walk away.
	const settle = 2 * time.Second

	if redModeGoroutineLeak() {
		leaked := waitUntilLeaked(settle)
		require.True(t, leaked,
			"RED expectation failed: no goroutine found parked in CompleteStream's "+
				"closure (blocked in chan-send state) within %s of ctx cancellation "+
				"on the pre-fix artifact — the leak should be reproducible here. "+
				"If this fails, the defect is already fixed; flip RED_MODE=0.", settle)
		return
	}

	leaked := notLeakedAfterSettling(settle)
	require.False(t, leaked,
		"GREEN failed: a goroutine is still parked in CompleteStream's closure "+
			"(blocked in chan-send state) %s after ctx cancellation with the "+
			"channel never drained — an unguarded blocking send in CompleteStream "+
			"leaks a goroutine forever. Every send site must use "+
			"`select { case ch <- resp: case <-ctx.Done(): return }`.", settle)
}
