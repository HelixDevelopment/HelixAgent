package integration

// Self-validation for the HelixAgent service-identity gate
// (helixagent_identity.go).
//
// WHY THIS FILE EXISTS (§11.4.107(10), §11.4.201)
//
// The gate shipped with ZERO tests of its own, and a gate that guards 13
// call sites while nothing guards IT is an unfalsifiable gate. The escape was
// demonstrated, not theorised: mutate helixAgentServiceName to "helixagentx"
// and point HELIXAGENT_URL at anything answering
// {"service":"helixagent","status":"healthy"} — every guarded test SKIPs and
// `go test ./tests/integration/` still prints `ok`. A defect in the gate
// silently converts the entire guarded suite into permanent green skips,
// which is the §11.4 PASS-bluff in its purest form: the suite reports success
// precisely because it verified nothing.
//
// These fixtures are the golden-good / golden-bad pair the gate lacked:
//
//	golden-good  TestProbeHelixAgent_IdentifiesHealthyHelixAgent
//	             — a correct gate MUST accept a real HelixAgent.
//	             Mutating helixAgentServiceName makes THIS test FAIL, so the
//	             gate can no longer be broken silently.
//	golden-bad   TestProbeHelixAgent_ForeignServiceWithJSONIdentity and
//	             siblings — a correct gate MUST reject each of them, so the
//	             gate cannot be "fixed" by making it accept everything.
//
// Both directions are required. A one-sided suite is defeated by the opposite
// mutation: golden-good alone passes a gate that accepts every responder;
// golden-bad alone passes a gate hard-wired to `return false`, which would
// skip the whole suite forever.
//
// Every fixture is an httptest server on an ephemeral loopback port, so these
// tests are infra-free and deterministic (§11.4.50) — they never depend on a
// live HelixAgent being up or down.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// guardRecorder records requireHelixAgent's verdict instead of suffering it.
//
// Skipf and Fatalf must call runtime.Goexit, exactly as *testing.T does, or
// the guard's callers would keep running past a verdict that is supposed to
// stop them — and the tests would then be asserting on a control flow the
// real guard never has.
type guardRecorder struct {
	skipped  bool
	failed   bool
	returned bool
	msg      string
}

func (r *guardRecorder) Helper() {}

func (r *guardRecorder) Skipf(format string, args ...interface{}) {
	r.skipped = true
	r.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

func (r *guardRecorder) Fatalf(format string, args ...interface{}) {
	r.failed = true
	r.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// runGuard invokes requireHelixAgent on a throwaway goroutine — required
// because Goexit terminates the calling goroutine — and returns what it did.
func runGuard(t *testing.T, baseURL string) *guardRecorder {
	t.Helper()
	rec := &guardRecorder{}
	done := make(chan struct{})
	go func() {
		// Deferred calls still run on Goexit, so this closes in every path.
		defer close(done)
		requireHelixAgent(rec, baseURL)
		rec.returned = true
	}()
	<-done
	return rec
}

// newHealthFixture starts a loopback HTTP server whose /health answers with
// the supplied status and body. Anything other than /health 404s, exactly as
// Go's default mux does.
func newHealthFixture(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// --- golden-good -----------------------------------------------------------

// TestProbeHelixAgent_IdentifiesHealthyHelixAgent is the golden-good fixture.
//
// This is the assertion the reviewer's mutation must break. With
// helixAgentServiceName mutated to "helixagentx" this test FAILS, so the gate
// is falsifiable: it can no longer be silently broken into a suite-wide skip.
func TestProbeHelixAgent_IdentifiesHealthyHelixAgent(t *testing.T) {
	srv := newHealthFixture(t, http.StatusOK,
		`{"service":"helixagent","status":"healthy"}`)

	probe := probeHelixAgent(srv.URL)

	assert.True(t, probe.Reachable, "the fixture accepted the connection")
	assert.True(t, probe.IsHelixAgent,
		"a responder naming itself %q MUST be accepted; if this fails, the "+
			"identity constant no longer matches what HelixAgent reports and "+
			"every guarded test in this package would silently skip",
		helixAgentServiceName)
	assert.Equal(t, outcomeIdentified, probe.Outcome)
	assert.Equal(t, http.StatusOK, probe.StatusCode)
}

// TestProbeHelixAgent_IdentifiedDespiteNon200 pins the deliberate asymmetry:
// identity is decided by the reported service name, NOT by the status code.
// A degraded-but-identified HelixAgent must reach the caller's assertions so
// its non-200 fails the test as the genuine product signal it is, rather than
// being converted into a skip.
func TestProbeHelixAgent_IdentifiedDespiteNon200(t *testing.T) {
	srv := newHealthFixture(t, http.StatusServiceUnavailable,
		`{"service":"helixagent","status":"degraded"}`)

	probe := probeHelixAgent(srv.URL)

	assert.True(t, probe.IsHelixAgent,
		"a degraded HelixAgent is still HelixAgent; converting its 503 into a "+
			"skip would hide a real outage behind a green suite")
	assert.Equal(t, outcomeIdentified, probe.Outcome)
	assert.Equal(t, http.StatusServiceUnavailable, probe.StatusCode)
}

// --- golden-bad: the four branches the old probe could not separate --------

// TestProbeHelixAgent_ForeignServiceWithJSONIdentity covers case (a) in its
// only unambiguous form: a JSON responder that names a DIFFERENT service.
// This is the one wrong-service verdict the probe may state as a fact.
func TestProbeHelixAgent_ForeignServiceWithJSONIdentity(t *testing.T) {
	srv := newHealthFixture(t, http.StatusOK,
		`{"service":"llmsverifier","status":"healthy"}`)

	probe := probeHelixAgent(srv.URL)

	assert.True(t, probe.Reachable)
	assert.False(t, probe.IsHelixAgent, "a different service must be rejected")
	assert.Equal(t, outcomeForeignService, probe.Outcome)
	assert.Contains(t, probe.Detail, "llmsverifier",
		"the message must name what actually answered, so the operator can "+
			"fix the environment instead of guessing")
}

// TestProbeHelixAgent_NonJSONBodyIsAmbiguous covers cases (a) and (b)
// together, because on the wire they are the same bytes.
//
// A foreign Go service and a real HelixAgent whose /health route was dropped
// BOTH answer `404 page not found`. The old probe asserted one cause —
// "another process is bound to this port" — for both, so a genuine HelixAgent
// regression was reported to the operator as someone else's port and skipped.
// The probe must now state the ambiguity rather than pick a side (§11.4.6).
func TestProbeHelixAgent_NonJSONBodyIsAmbiguous(t *testing.T) {
	// No /health route at all — the fixture's mux 404s exactly as Go's does.
	srv := httptest.NewServer(http.HandlerFunc(http.NotFound))
	t.Cleanup(srv.Close)

	probe := probeHelixAgent(srv.URL)

	assert.True(t, probe.Reachable)
	assert.False(t, probe.IsHelixAgent)
	assert.Equal(t, outcomeNonJSONBody, probe.Outcome)
	assert.Equal(t, http.StatusNotFound, probe.StatusCode)
	assert.Contains(t, probe.Detail, "AMBIGUOUS",
		"a 404 cannot distinguish a foreign service from a HelixAgent with a "+
			"missing /health route; asserting either cause is a guess")
	assert.Contains(t, probe.Detail, helixAgentRequiredEnv,
		"the message must tell the operator how to turn this into a failure")
	assert.Contains(t, probe.Detail, "/health route was dropped",
		"a 404 is the one status where the dropped-route reading applies, "+
			"and it is the reason this outcome is ambiguous at all")
}

// TestProbeHelixAgent_NonJSON200DoesNotBlameAMissingRoute is the sibling of
// the 404 case. A 200 with an HTML body — a proxy error page, a static site
// on the port — is classified the same way and behaves the same way, but the
// dropped-/health-route reading does NOT apply: the route plainly answered.
// Telling the operator it might be missing sends them hunting for a route
// that is right there.
func TestProbeHelixAgent_NonJSON200DoesNotBlameAMissingRoute(t *testing.T) {
	srv := newHealthFixture(t, http.StatusOK,
		"<html><body>Service Temporarily Unavailable</body></html>")

	probe := probeHelixAgent(srv.URL)

	assert.True(t, probe.Reachable)
	assert.False(t, probe.IsHelixAgent,
		"an HTML body identifies nobody and must not be accepted")
	assert.Equal(t, outcomeNonJSONBody, probe.Outcome,
		"classification is unchanged — only the explanation differs by status")
	assert.NotContains(t, probe.Detail, "route was dropped",
		"this endpoint ANSWERED /health with a 200; blaming a missing route "+
			"would misdirect the operator")
	assert.Contains(t, probe.Detail, "AMBIGUOUS")
	assert.Contains(t, probe.Detail, helixAgentRequiredEnv)
}

// TestProbeHelixAgent_ServerErrorIsNotAPortSquatter covers case (c): a real
// HelixAgent whose handler panicked answers 500 with an empty body.
//
// The old probe swallowed this into the same "another process is bound to
// this port" message — actively misleading, since a 5xx is overwhelmingly a
// failing server rather than a squatter.
func TestProbeHelixAgent_ServerErrorIsNotAPortSquatter(t *testing.T) {
	srv := newHealthFixture(t, http.StatusInternalServerError, "")

	probe := probeHelixAgent(srv.URL)

	assert.True(t, probe.Reachable)
	assert.False(t, probe.IsHelixAgent)
	assert.Equal(t, outcomeServerError, probe.Outcome)
	assert.Equal(t, http.StatusInternalServerError, probe.StatusCode)
	// The wrong-service branches tell the operator a different service "is
	// bound to this port". Saying that about a 5xx sends them to debug the
	// wrong machine while their own server is down.
	assert.NotContains(t, probe.Detail, "bound to this port",
		"a 5xx is a server failure, not evidence of a port squatter")
	assert.Contains(t, probe.Detail, "server error")
}

// TestProbeHelixAgent_LostIdentityFieldIsFlaggedAsPossibleRegression covers
// case (d): HTTP 200, valid JSON, but the "service" field is gone.
//
// This repo's OWN mock LLM server answers /health with exactly
// {"status":"healthy"} and no service field, so this shape is reachable
// today — but so is a HelixAgent that regressed its own health payload. The
// probe must name both possibilities instead of asserting one.
func TestProbeHelixAgent_LostIdentityFieldIsFlaggedAsPossibleRegression(t *testing.T) {
	srv := newHealthFixture(t, http.StatusOK, `{"status":"healthy"}`)

	probe := probeHelixAgent(srv.URL)

	assert.True(t, probe.Reachable)
	assert.False(t, probe.IsHelixAgent,
		"an unidentified 200 must NOT be accepted; accepting it is how a mock "+
			"server passes for the product")
	assert.Equal(t, outcomeUnidentifiedJSON, probe.Outcome)
	assert.Contains(t, probe.Detail, "regression",
		"a HelixAgent that stopped reporting its identity is itself a defect "+
			"and the message must say so")
}

// TestProbeHelixAgent_UnreachableEndpoint covers the fifth outcome: nothing
// is listening at all.
func TestProbeHelixAgent_UnreachableEndpoint(t *testing.T) {
	// Bind and immediately close, so the port is provably free rather than
	// a hardcoded guess that some other process might occupy (§11.4.6).
	srv := httptest.NewServer(http.HandlerFunc(http.NotFound))
	dead := srv.URL
	srv.Close()

	probe := probeHelixAgent(dead)

	assert.False(t, probe.Reachable)
	assert.False(t, probe.IsHelixAgent)
	assert.Equal(t, outcomeUnreachable, probe.Outcome)
	assert.Equal(t, 0, probe.StatusCode)
	assert.Contains(t, probe.Detail, "no service is reachable")
}

// --- the outcome set itself ------------------------------------------------

// TestProbeHelixAgent_EveryNonIdentifiedOutcomeCarriesDetail enforces the
// contract the skip/fail messages depend on: Detail is never empty when the
// responder was not identified. An empty detail would produce a skip message
// that tells the operator nothing — the §11.4.201 false-null failure at the
// reporting layer.
func TestProbeHelixAgent_EveryNonIdentifiedOutcomeCarriesDetail(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		outcome helixAgentOutcome
	}{
		{"foreign-service", http.StatusOK, `{"service":"other"}`, outcomeForeignService},
		{"unidentified-json", http.StatusOK, `{"status":"healthy"}`, outcomeUnidentifiedJSON},
		{"non-json-body", http.StatusNotFound, "404 page not found\n", outcomeNonJSONBody},
		{"server-error", http.StatusInternalServerError, "", outcomeServerError},
	}

	seen := make(map[helixAgentOutcome]bool, len(cases))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newHealthFixture(t, tc.status, tc.body)
			probe := probeHelixAgent(srv.URL)

			require.False(t, probe.IsHelixAgent)
			assert.Equal(t, tc.outcome, probe.Outcome)
			assert.NotEmpty(t, probe.Detail,
				"a non-identified probe MUST explain itself; an empty detail "+
					"produces a skip message the operator cannot act on")
		})
		seen[tc.outcome] = true
	}

	assert.Len(t, seen, len(cases),
		"each case must map to a DISTINCT outcome; collapsing two of them "+
			"re-creates the one-message-fits-all defect this classification "+
			"exists to remove")
}

// --- strict mode -----------------------------------------------------------

// nonIdentifiedFixtures is the shared case table for the guard tests: every
// outcome that is NOT "identified". Default mode must skip each; strict mode
// must fail each.
var nonIdentifiedFixtures = []struct {
	name   string
	status int
	body   string
}{
	{"foreign-service", http.StatusOK, `{"service":"llmsverifier"}`},
	{"unidentified-json", http.StatusOK, `{"status":"healthy"}`},
	{"non-json-body", http.StatusNotFound, "404 page not found\n"},
	{"server-error", http.StatusInternalServerError, ""},
}

// TestRequireHelixAgent_DefaultSkipsOnUnidentified pins the DEFAULT
// behaviour as UNCHANGED by the strict-mode addition: absent an explicit
// operator declaration, every unidentified endpoint is a skip, never a
// failure, so an absent environment is not reported as a product defect.
func TestRequireHelixAgent_DefaultSkipsOnUnidentified(t *testing.T) {
	t.Setenv(helixAgentRequiredEnv, "")

	for _, tc := range nonIdentifiedFixtures {
		t.Run(tc.name, func(t *testing.T) {
			srv := newHealthFixture(t, tc.status, tc.body)

			rec := runGuard(t, srv.URL)

			assert.True(t, rec.skipped, "default mode must SKIP %s", tc.name)
			assert.False(t, rec.failed, "default mode must not fail")
			assert.False(t, rec.returned,
				"the guard must stop the caller; returning would let the test "+
					"proceed to assert against a service that is not HelixAgent")
			assert.Contains(t, rec.msg, "SKIP-OK:",
				"the project's no-silent-skips gate requires a SKIP-OK marker")
		})
	}
}

// TestRequireHelixAgent_StrictModeFailsOnUnidentified is the point of the
// strict mode: where HelixAgent is known-deployed, a non-identifying answer
// is a real failure and must stop the suite instead of greening it.
//
// It covers ALL four non-identified outcomes, including the ambiguous 404
// that could be a dropped /health route — the case that motivated the flag.
func TestRequireHelixAgent_StrictModeFailsOnUnidentified(t *testing.T) {
	t.Setenv(helixAgentRequiredEnv, "1")

	for _, tc := range nonIdentifiedFixtures {
		t.Run(tc.name, func(t *testing.T) {
			srv := newHealthFixture(t, tc.status, tc.body)

			rec := runGuard(t, srv.URL)

			assert.True(t, rec.failed,
				"with %s=1 the operator has declared HelixAgent is deployed "+
					"here; %s must FAIL — skipping is how a broken deployment "+
					"reports a green suite",
				helixAgentRequiredEnv, tc.name)
			assert.False(t, rec.skipped, "strict mode must not skip")
			assert.Contains(t, rec.msg, helixAgentRequiredEnv,
				"the failure must name the declaration it is enforcing, so "+
					"the operator knows why this became fatal")
		})
	}
}

// TestRequireHelixAgent_StrictModeFailsOnUnreachable: the flag asserts the
// service IS there, so connection-refused contradicts it just as loudly as a
// 404 does.
func TestRequireHelixAgent_StrictModeFailsOnUnreachable(t *testing.T) {
	t.Setenv(helixAgentRequiredEnv, "1")

	srv := httptest.NewServer(http.HandlerFunc(http.NotFound))
	dead := srv.URL
	srv.Close()

	rec := runGuard(t, dead)

	assert.True(t, rec.failed,
		"%s=1 declares HelixAgent is deployed; an unreachable endpoint "+
			"refutes that declaration and must fail", helixAgentRequiredEnv)
	assert.False(t, rec.skipped)
}

// TestRequireHelixAgent_DefaultSkipsOnUnreachable pins the unchanged default
// for the same endpoint, so the pair proves the flag is what moved the
// behaviour — not some unrelated drift.
func TestRequireHelixAgent_DefaultSkipsOnUnreachable(t *testing.T) {
	t.Setenv(helixAgentRequiredEnv, "")

	srv := httptest.NewServer(http.HandlerFunc(http.NotFound))
	dead := srv.URL
	srv.Close()

	rec := runGuard(t, dead)

	assert.True(t, rec.skipped, "default mode must skip an absent HelixAgent")
	assert.False(t, rec.failed)
}

// TestRequireHelixAgent_StrictModeStillAdmitsRealHelixAgent guards the
// obvious over-correction: strict mode must not fail on a HEALTHY agent.
// Without this, "make strict mode fail" could be satisfied by failing always,
// which would be a different but equally useless gate.
func TestRequireHelixAgent_StrictModeStillAdmitsRealHelixAgent(t *testing.T) {
	t.Setenv(helixAgentRequiredEnv, "1")
	srv := newHealthFixture(t, http.StatusOK,
		`{"service":"helixagent","status":"healthy"}`)

	rec := runGuard(t, srv.URL)

	assert.True(t, rec.returned,
		"strict mode must admit a real HelixAgent and let the caller proceed; "+
			"a gate that fails on everything is as useless as one that passes "+
			"on everything")
	assert.False(t, rec.failed)
	assert.False(t, rec.skipped)
}

// TestHelixAgentRequiredEnv_IsTheDocumentedLiteral closes the second
// self-referential loop in this file.
//
// Every OTHER test here sets the variable through helixAgentRequiredEnv —
// the same constant requireHelixAgent reads. That is a closed loop: rename
// the constant's VALUE to "HELIXAGENT_REQUIRE" and the whole suite stays
// GREEN, including with the real HELIXAGENT_REQUIRED=1 exported into the
// run, because the tests and the code moved together. An environment wired
// with HELIXAGENT_REQUIRED=1 would then silently revert to fail-open
// masking — the precise failure the flag exists to prevent.
//
// The service name was already pinned this way (the golden-good fixture
// hardcodes {"service":"helixagent"}, which is what makes the helixagentx
// mutation fail). This does the same for the env var.
//
// WHICH CONSTANTS NEED THIS: any constant whose value is part of a contract
// with something OUTSIDE this process — an env var an operator exports, a
// field name a server emits. The outcome* constants below deliberately do
// NOT get this treatment: they never cross a process boundary, so their
// values are free to change and only their distinctness matters.
func TestHelixAgentRequiredEnv_IsTheDocumentedLiteral(t *testing.T) {
	// Written out longhand ON PURPOSE — do not replace with the constant.
	const documented = "HELIXAGENT_REQUIRED"

	assert.Equal(t, documented, helixAgentRequiredEnv,
		"the strict-mode env var name is an operator-facing contract; "+
			"changing it silently disables strict mode in every environment "+
			"already exporting the old name")

	// Compare-only is not enough: drive the real resolver through the
	// literal so the binding is proven end-to-end, not merely asserted.
	t.Setenv(documented, "1")
	assert.True(t, helixAgentRequired(),
		"exporting the documented name %q must enable strict mode; if this "+
			"fails, the code reads some other variable and every environment "+
			"that sets %s has silently fallen back to skip-on-unidentified",
		documented, documented)
}

// TestHelixAgentRequired_OnlyExactlyOneEnables pins the trigger value. A
// loose truthiness check ("any non-empty value") would flip whole suites from
// skip to fail on a stray `HELIXAGENT_REQUIRED=0`, which reads as "not
// required" to every operator.
func TestHelixAgentRequired_OnlyExactlyOneEnables(t *testing.T) {
	for _, val := range []string{"", "0", "false", "no", "2", "true", "yes"} {
		t.Run(fmt.Sprintf("value=%q", val), func(t *testing.T) {
			t.Setenv(helixAgentRequiredEnv, val)
			assert.False(t, helixAgentRequired(),
				"only the literal \"1\" may enable strict mode")
		})
	}

	t.Run("value=1", func(t *testing.T) {
		t.Setenv(helixAgentRequiredEnv, "1")
		assert.True(t, helixAgentRequired())
	})

	t.Run("value=1 with surrounding whitespace", func(t *testing.T) {
		t.Setenv(helixAgentRequiredEnv, " 1 ")
		assert.True(t, helixAgentRequired(),
			"shell exports routinely carry stray whitespace; refusing to "+
				"honour them would silently disable the strict mode an "+
				"operator believes they turned on")
	})
}
