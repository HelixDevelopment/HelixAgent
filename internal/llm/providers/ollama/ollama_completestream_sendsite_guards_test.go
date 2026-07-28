package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/require"
)

// This file closes the §11.4.135 regression-guard gap left by commit 69a5ab8d.
//
// 69a5ab8d wrapped FOUR previously-unguarded blocking channel sends in
// CompleteStream with `select { case ch <- resp: case <-ctx.Done(): ... }`:
//
//	(1) after http.NewRequestWithContext failure   (ollama.go ~152-167)
//	(2) after json.Marshal failure                 (ollama.go ~169-184)
//	(3) after o.httpClient.Do failure              (ollama.go ~186-204)
//	(4) the final "stop" response                  (ollama.go ~256-274)
//
// Its companion guard (ollama_completestream_leak_test.go) drives ONLY site
// (3). Sites (1), (2) and (4) were reached by NO test at all: deleting their
// guards failed zero tests, which made them unverified claims (§11.4.125(6) /
// §1.1) — precisely the "reconciliation applied in one place but not its
// siblings" pattern the meta-repo already closed for vertexai/groq/bedrock/
// ensemble in commit 97d5ad2b. This file adds the three missing guards using
// the SAME proven oracle, reusing goroutineParkedInCompleteStream /
// waitUntilLeaked / notLeakedAfterSettling / redModeGoroutineLeak from
// ollama_completestream_leak_test.go unchanged (same package).
//
// SHARED PROTOCOL (every subtest below):
//
//	1. BASELINE     — assert no goroutine is already parked in CompleteStream.
//	2. DRIVE        — steer the goroutine to exactly one target send site.
//	3. SITE-REACHED — assert a goroutine IS parked in "chan send" state BEFORE
//	                  ctx is cancelled. This is positive, non-vacuous evidence
//	                  that the intended send site was genuinely executed and is
//	                  genuinely blocking. Without this step a subtest that never
//	                  reached its site would pass the GREEN assertion trivially
//	                  (absence-of-error is not evidence, §11.4/§11.4.1).
//	4. CANCEL       — cancel ctx while the channel is deliberately NEVER drained,
//	                  modelling exactly what a real caller does on
//	                  cancellation/timeout: cancel and walk away.
//	5. VERDICT      — RED_MODE=1 asserts the goroutine is STILL parked after the
//	                  settle window (the leak reproduces on a pre-fix/mutated
//	                  artifact); RED_MODE=0 (default) is the standing GREEN
//	                  guard asserting it is GONE because the ctx.Done() case
//	                  fired.
//
// Both polarities in step 5 wait out the FULL settle window before their single
// decisive read (notLeakedAfterSettling), so neither can pass on a transient
// reading taken before the goroutine reached its steady state.

// sendSiteSettle is the steady-state window used for both the site-reached
// wait and the post-cancellation verdict. It matches the 2s window the
// existing httpClient.Do guard uses.
const sendSiteSettle = 2 * time.Second

// goroutineBlockedAtCompleteStreamSend reports whether any goroutine is
// currently parked at a send site inside CompleteStream's background closure.
//
// WHY THIS EXISTS ALONGSIDE goroutineParkedInCompleteStream (do not collapse
// the two): the sibling helper in ollama_completestream_leak_test.go matches
// the "chan send" wait state ONLY, which is the state of an UNGUARDED bare
// `ch <- resp`. A GUARDED send — `select { case ch <- resp: case <-ctx.Done(): }`
// — has two cases, so the Go runtime parks the goroutine in the "select" wait
// state instead. Captured evidence from a live runtime.Stack(all=true) dump
// while a guarded send was blocking:
//
//	goroutine 22 [select]:
//	dev.helix.agent/internal/llm/providers/ollama.(*OllamaProvider).CompleteStream.func1()
//
// That narrowness is CORRECT for the sibling helper's own use (it asserts a
// leak, and only the unguarded shape leaks). But it makes the sibling helper
// blind to a guarded-yet-reached send site, so it cannot serve as this file's
// step-3 "the drive genuinely reached the site" precondition on FIXED code.
//
// Matching either state is also STRICTER for the step-5 verdict: on fixed code
// the ctx.Done() case fires and the goroutine RETURNS, so neither state is
// present — this helper therefore demands the goroutine be entirely gone,
// rather than merely no longer in "chan send".
//
// Matching "select" cannot over-match here: every select in CompleteStream's
// closure is one of its six channel-send selects, so a goroutine parked in a
// select inside that closure is by construction parked at a send site.
//
// The bracketed prefixes ("[chan send", "[select") match the goroutine header
// line and deliberately omit the closing bracket, because the runtime appends
// a duration for long-blocked goroutines (e.g. "[chan send, 5 minutes]:").
func goroutineBlockedAtCompleteStreamSend() bool {
	buf := make([]byte, 1<<16)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	for _, block := range strings.Split(string(buf), "\n\n") {
		if !strings.Contains(block, leakSymbol) {
			continue
		}
		if strings.Contains(block, "[chan send") || strings.Contains(block, "[select") {
			return true
		}
	}
	return false
}

// waitUntilBlockedAtSendSite polls until a goroutine is parked at a send site
// or the deadline elapses. Early-exit-on-true is sound: the observation is
// only used to prove the site was reached.
func waitUntilBlockedAtSendSite(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if goroutineBlockedAtCompleteStreamSend() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// stillBlockedAfterSettling sleeps the FULL settle window with no early exit
// and then takes a single decisive snapshot. Early-exiting on an initial
// "not blocked" reading would be UNSOUND for the GREEN assertion: a reading
// taken before the goroutine has even attempted its send would falsely pass
// on genuinely broken (unguarded) code.
func stillBlockedAfterSettling(settle time.Duration) bool {
	time.Sleep(settle)
	return goroutineBlockedAtCompleteStreamSend()
}

// assertSendSiteReachedAndBlocking performs step 3 of the shared protocol: it
// waits until a goroutine is genuinely parked at a send site inside
// CompleteStream's closure, and fails with a site-specific diagnostic if that
// never happens.
func assertSendSiteReachedAndBlocking(t *testing.T, siteDesc string) {
	t.Helper()
	require.True(t, waitUntilBlockedAtSendSite(sendSiteSettle),
		"drive failed: no goroutine became parked at a send site in "+
			"CompleteStream's closure within %s while driving the %s send site. "+
			"The test never actually reached the site under guard, so any "+
			"subsequent GREEN result would be vacuous.", sendSiteSettle, siteDesc)
}

// assertSendSiteVerdict performs step 5 of the shared protocol for both
// polarities.
func assertSendSiteVerdict(t *testing.T, siteDesc string) {
	t.Helper()
	stillParked := stillBlockedAfterSettling(sendSiteSettle)

	if redModeGoroutineLeak() {
		require.True(t, stillParked,
			"RED expectation failed: the goroutine driven to the %s send site is "+
				"NOT parked %s after ctx cancellation — the unguarded-send leak "+
				"should be reproducible on a pre-fix artifact. If this fails, the "+
				"guard is already in place; flip RED_MODE=0.", siteDesc, sendSiteSettle)
		return
	}

	require.False(t, stillParked,
		"GREEN failed: a goroutine is still parked at a send site in "+
			"CompleteStream's closure %s after ctx cancellation, with the channel "+
			"never drained, while driving the %s send site — that send does not "+
			"honour cancellation and leaks the goroutine forever. It must use "+
			"`select { case ch <- resp: case <-ctx.Done(): }`.", sendSiteSettle, siteDesc)
}

// requireNoPreexistingPark is the shared baseline precondition (step 1).
func requireNoPreexistingPark(t *testing.T) {
	t.Helper()
	require.False(t, goroutineBlockedAtCompleteStreamSend(),
		"precondition failed: a goroutine is already parked at a send site in "+
			"CompleteStream's closure before this subtest ran — a subsequent "+
			"observation could not be attributed to this subtest's invocation")
}

// TestOllamaProvider_CompleteStream_SendSiteGuards covers the three send sites
// that commit 69a5ab8d guarded but left untested. Each subtest drives exactly
// ONE site; the subtests are deliberately NOT parallel because the
// runtime.Stack(all=true) oracle observes the whole process, so a concurrent
// subtest parked in CompleteStream would cross-contaminate every other
// subtest's reading (§11.4.119 single-resource-owner applied to the
// process-wide goroutine dump).
func TestOllamaProvider_CompleteStream_SendSiteGuards(t *testing.T) {
	// SITE (1): the send that fires after http.NewRequestWithContext fails.
	//
	// DRIVE: a base URL containing an ASCII control character (0x7f) makes
	// net/url reject the URL, so http.NewRequestWithContext returns an error
	// before any network I/O is attempted. Verified driver output:
	//   parse "http://ollama\x7finvalid.example/api/generate":
	//   net/url: invalid control character in URL
	t.Run("request_construction_failure", func(t *testing.T) {
		requireNoPreexistingPark(t)

		// No server is contacted on this path: construction fails first.
		provider := NewOllamaProvider("http://ollama\x7finvalid.example", "llama2")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch, err := provider.CompleteStream(ctx, &models.LLMRequest{
			ID:     "sendsite-request-construction",
			Prompt: "test prompt",
		})
		require.NoError(t, err)
		require.NotNil(t, ch)

		// Deliberately never read from ch.
		assertSendSiteReachedAndBlocking(t, "http.NewRequestWithContext-failure")
		cancel()
		assertSendSiteVerdict(t, "http.NewRequestWithContext-failure")
	})

	// SITE (2): the send that fires after json.Marshal fails.
	//
	// DRIVE: OllamaOptions.Temperature is a float64 populated straight from
	// req.ModelParams.Temperature, and encoding/json refuses to marshal NaN.
	// Verified driver output: json: unsupported value: NaN
	//
	// The base URL must be well-formed here so that site (1) succeeds and
	// execution actually reaches the marshal; it is never dialled because
	// the marshal fails before o.httpClient.Do is called.
	t.Run("json_marshal_failure", func(t *testing.T) {
		requireNoPreexistingPark(t)

		// Sanity-check the driver itself rather than assuming it (§11.4.6):
		// if encoding/json ever started accepting NaN, this subtest would
		// silently stop exercising the marshal-failure site.
		_, marshalErr := json.Marshal(OllamaRequest{
			Model:   "llama2",
			Options: OllamaOptions{Temperature: math.NaN()},
		})
		require.Error(t, marshalErr,
			"driver precondition failed: json.Marshal accepted a NaN Temperature, "+
				"so this subtest can no longer reach the json.Marshal-failure send site")

		provider := NewOllamaProvider("http://127.0.0.1:1", "llama2")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch, err := provider.CompleteStream(ctx, &models.LLMRequest{
			ID:     "sendsite-json-marshal",
			Prompt: "test prompt",
			ModelParams: models.ModelParameters{
				Temperature: math.NaN(),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, ch)

		// Deliberately never read from ch.
		assertSendSiteReachedAndBlocking(t, "json.Marshal-failure")
		cancel()
		assertSendSiteVerdict(t, "json.Marshal-failure")
	})

	// SITE (4): the final "stop" response send, after a completed stream.
	//
	// DRIVE: the server returns a single well-formed streaming object with
	// "done":true. The goroutine decodes it, blocks on the per-chunk send
	// (site 5 — already guarded pre-69a5ab8d), and the test reads EXACTLY ONE
	// value to release it. The goroutine then sees streamResp.Done, builds
	// finalResp, and blocks on the final send — the site under guard — because
	// the test reads nothing further. No further chunk send can occur, since
	// Done breaks the decode loop, so the only send a goroutine can be parked
	// on from here on is the final one.
	t.Run("final_stop_response", func(t *testing.T) {
		requireNoPreexistingPark(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = fmt.Fprint(w, `{"model":"llama2","response":"hello","done":true}`)
		}))
		defer server.Close()

		provider := NewOllamaProvider(server.URL, "llama2")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch, err := provider.CompleteStream(ctx, &models.LLMRequest{
			ID:     "sendsite-final-stop",
			Prompt: "test prompt",
		})
		require.NoError(t, err)
		require.NotNil(t, ch)

		// Read EXACTLY ONE value: the per-chunk response. This releases the
		// already-guarded per-chunk send so the goroutine can advance to the
		// final-response send, which is the site under test.
		select {
		case chunk, ok := <-ch:
			require.True(t, ok, "channel closed before the per-chunk response arrived — "+
				"the stream never reached the final-response send site")
			require.NotNil(t, chunk)
			require.Equal(t, "hello", chunk.Content,
				"unexpected first response: the decode loop did not deliver the "+
					"streamed chunk, so the final-response send site was not reached")
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for the per-chunk response — cannot drive the final-response send site")
		}

		// Deliberately read nothing further: the goroutine now blocks on the
		// final "stop" send.
		assertSendSiteReachedAndBlocking(t, "final-stop-response")
		cancel()
		assertSendSiteVerdict(t, "final-stop-response")
	})
}
