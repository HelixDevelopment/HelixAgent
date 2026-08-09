package integration

// Service-identity preconditions for tests that assert against a live
// HelixAgent over HTTP.
//
// # Why this exists (§11.4.201, §11.4.111)
//
// Integration tests here resolve HelixAgent from HELIXAGENT_URL, defaulting
// to the port-registry value for HelixAgentHTTP (internal/ports => :8100).
// Their liveness guards were written as `if err != nil { t.Skip(...) }`, i.e.
// they only recognise ONE failure mode: nothing is listening (connection
// refused). That is a FALSE-NULL detector — it cannot distinguish
//
//	(1) HelixAgent is absent           => the test has nothing to assert on
//	(2) HelixAgent answered            => the test's assertions are meaningful
//	(3) a DIFFERENT service answered   => the test's assertions are meaningless
//
// Case (3) is not hypothetical. Measured on this host 2026-08-08: the
// LLMsVerifier binary was bound to :8100 (`llm-verifier ... server --port
// 8100`) — the port the registry reserves for HelixAgentHTTP — while the
// deployed HelixAgent was listening on its pre-migration :7061. Every request
// the tests made was answered by the wrong process with Go's default-mux
// `404 page not found`. Because a live socket accepted the connection, `err`
// was nil, the skip guard never fired, and ~74 assertions reported
// `expected: 200, actual: 404` — reading exactly like a broken HelixAgent.
// It was not broken: pointed at the instance that was actually running, the
// same assertions passed.
//
// A test that FAILS when its target is absent or impersonated is a defect in
// the test, not a finding about the product. These helpers convert that class
// into an honest SKIP-with-reason (§11.4.3) carrying the evidence needed to
// fix the environment, while leaving genuine product failures to FAIL loudly.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// helixAgentServiceName is the value HelixAgent reports in the "service"
// field of its /health payload:
//
//	{"service":"helixagent","status":"healthy"}
//
// It is the identity signal — a port number is not one. Binding a test to
// "whatever answers :8100" is the by-index coupling §11.4.111 forbids;
// binding it to "the process that identifies as helixagent" is the by-name
// resolution it mandates.
const helixAgentServiceName = "helixagent"

// helixAgentRequiredEnv, when set to "1", declares that a real HelixAgent IS
// deployed for this run. See requireHelixAgent for the semantics.
const helixAgentRequiredEnv = "HELIXAGENT_REQUIRED"

// helixAgentOutcome is the closed classification of one probe.
//
// WHY A CLASSIFICATION AND NOT JUST A BOOLEAN (§11.4.6, §11.4.201)
// An earlier revision collapsed every non-identifying answer into one skip
// message that asserted a single cause — "another process is bound to this
// port". That message is a GUESS: the wire cannot distinguish
//
//	(a) a foreign Go service answering its default-mux `404 page not found`
//	(b) a REAL HelixAgent whose /health route was dropped — byte-identical 404
//
// and it is simply WRONG for two further cases it also swallowed
//
//	(c) a REAL HelixAgent whose handler panicked — HTTP 500, empty body
//	(d) a REAL HelixAgent that stopped emitting its identity field — HTTP 200
//
// (b), (c) and (d) are genuine HelixAgent regressions that were being
// reported to the operator as "another process is bound to this port" and
// skipped. The probe now names what it actually observed, states plainly
// where the observation is AMBIGUOUS instead of picking a cause, and leaves
// the fail-vs-skip decision to requireHelixAgent + the operator's
// HELIXAGENT_REQUIRED declaration.
type helixAgentOutcome string

const (
	// outcomeUnreachable: nothing accepted the connection.
	outcomeUnreachable helixAgentOutcome = "unreachable"
	// outcomeIdentified: the responder positively identified as HelixAgent.
	outcomeIdentified helixAgentOutcome = "identified"
	// outcomeForeignService: valid JSON naming a DIFFERENT service. This is
	// the only wrong-service verdict the probe can make as a FACT.
	outcomeForeignService helixAgentOutcome = "foreign-service"
	// outcomeUnidentifiedJSON: valid JSON with no "service" field — case (d),
	// or any other JSON-speaking occupant.
	outcomeUnidentifiedJSON helixAgentOutcome = "unidentified-json"
	// outcomeNonJSONBody: the body is not JSON (the `404 page not found`
	// class) — AMBIGUOUS between (a) and (b).
	outcomeNonJSONBody helixAgentOutcome = "non-json-body"
	// outcomeServerError: HTTP 5xx with a non-JSON/empty body — case (c),
	// or a foreign service that is itself failing. AMBIGUOUS, but far more
	// likely a failing server than a port squatter.
	outcomeServerError helixAgentOutcome = "server-error"
	// outcomeUnreadableBody: the response headers arrived but the body could
	// not be read.
	outcomeUnreadableBody helixAgentOutcome = "unreadable-body"
)

// helixAgentProbe is the outcome of one identity probe.
type helixAgentProbe struct {
	// Reachable is true when a TCP connection was accepted and an HTTP
	// response was read, regardless of who sent it.
	Reachable bool
	// IsHelixAgent is true only when the responder positively identified
	// itself as HelixAgent.
	IsHelixAgent bool
	// StatusCode of the /health response (0 when unreachable).
	StatusCode int
	// Outcome is the closed classification of what answered.
	Outcome helixAgentOutcome
	// Detail is a human-readable description of what answered, for use in
	// skip and failure messages. Never empty when IsHelixAgent is false.
	Detail string
}

// probeHelixAgent asks baseURL+"/health" who it is.
//
// It never fails a test; it reports what it found so callers can decide.
// The distinction it draws is the one the old `err != nil` guard could not:
// "nobody answered" vs "somebody else answered".
func probeHelixAgent(baseURL string) helixAgentProbe {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(strings.TrimSuffix(baseURL, "/") + "/health")
	if err != nil {
		return helixAgentProbe{
			Outcome: outcomeUnreachable,
			Detail:  fmt.Sprintf("no service is reachable at %s (%v)", baseURL, err),
		}
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if readErr != nil {
		return helixAgentProbe{
			Reachable:  true,
			StatusCode: resp.StatusCode,
			Outcome:    outcomeUnreadableBody,
			Detail: fmt.Sprintf("a service at %s answered HTTP %d but its body could not be read (%v)",
				baseURL, resp.StatusCode, readErr),
		}
	}

	probe := helixAgentProbe{Reachable: true, StatusCode: resp.StatusCode}

	// The identity claim lives in the JSON body.
	var payload struct {
		Service string `json:"service"`
	}
	parseErr := json.Unmarshal(body, &payload)

	switch {
	case parseErr == nil && payload.Service == helixAgentServiceName:
		// The responder named itself. A non-200 from here on is a genuine
		// product signal and MUST reach the caller's own assertions.
		probe.IsHelixAgent = true
		probe.Outcome = outcomeIdentified
		return probe

	case parseErr == nil && payload.Service != "":
		// FACT, not inference: something else said who it is.
		probe.Outcome = outcomeForeignService
		probe.Detail = fmt.Sprintf(
			"the service at %s identifies as %q, not %q — a different service "+
				"is bound to this port; point HELIXAGENT_URL at the real HelixAgent",
			baseURL, payload.Service, helixAgentServiceName)
		return probe

	case parseErr == nil:
		// Valid JSON, no identity field. Case (d): a HelixAgent that stopped
		// emitting "service" looks exactly like any other JSON responder, so
		// the cause is named as a possibility, never asserted.
		probe.Outcome = outcomeUnidentifiedJSON
		probe.Detail = fmt.Sprintf(
			"the service at %s answered HTTP %d with JSON carrying no %q field "+
				"(body: %s) — this is EITHER a foreign JSON service OR a "+
				"HelixAgent that has stopped reporting its identity, which "+
				"would itself be a regression; the wire cannot tell them apart",
			baseURL, resp.StatusCode, "service", truncateForMessage(body))
		return probe

	case resp.StatusCode >= 500:
		// Case (c): a panicking handler answers 5xx with an empty body. Far
		// more likely a failing server than a port squatter, so the message
		// must not blame the port.
		probe.Outcome = outcomeServerError
		probe.Detail = fmt.Sprintf(
			"the service at %s answered HTTP %d with a non-JSON body "+
				"(body: %s) — a server error, not a wrong-service verdict; "+
				"if HelixAgent is deployed here this is a HelixAgent failure "+
				"to investigate, not an environment gap to skip",
			baseURL, resp.StatusCode, truncateForMessage(body))
		return probe

	default:
		// Cases (a) and (b) are byte-identical on the wire — Go's default mux
		// answers a missing route with `404 page not found`, and so does any
		// other Go service that does not serve /health. State the ambiguity;
		// do not pick a cause (§11.4.6).
		probe.Outcome = outcomeNonJSONBody
		probe.Detail = fmt.Sprintf(
			"the service at %s answered HTTP %d with a non-JSON body "+
				"(body: %s) — AMBIGUOUS: this is EITHER a foreign service "+
				"bound to this port OR a HelixAgent whose /health route is "+
				"missing (both emit an identical response). Set %s=1 to treat "+
				"this as a failure in environments where HelixAgent is known "+
				"to be deployed",
			baseURL, resp.StatusCode, truncateForMessage(body), helixAgentRequiredEnv)
		return probe
	}
}

// helixAgentRequired reports whether the operator has declared that a real
// HelixAgent IS deployed for this run.
//
// Only the literal "1" enables strict mode. A loose truthiness test would
// read `HELIXAGENT_REQUIRED=0` as "required", flipping whole suites from skip
// to fail on a value every operator reads as "off".
func helixAgentRequired() bool {
	return strings.TrimSpace(os.Getenv(helixAgentRequiredEnv)) == "1"
}

// helixAgentGuardT is the subset of *testing.T that requireHelixAgent uses.
// See requireHelixAgent's doc comment for why this is an interface.
type helixAgentGuardT interface {
	Helper()
	Skipf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// compile-time proof that the real *testing.T still satisfies the guard
// interface, so a future signature drift is caught here rather than at ~30
// call sites.
var _ helixAgentGuardT = (*testing.T)(nil)

// requireHelixAgent asserts that baseURL is served by a real HelixAgent
// before the caller asserts anything about its behaviour.
//
// DEFAULT (HELIXAGENT_REQUIRED unset): SKIP, never fail, when HelixAgent is
// absent or unidentified — in neither case has the caller observed HelixAgent
// at all, so failing would report an environment gap as a product defect.
// Once this returns, a non-200 from HelixAgent is a genuine product signal
// and MUST be allowed to fail the test.
//
// STRICT MODE (HELIXAGENT_REQUIRED=1): FAIL instead of skipping.
//
// The default is a deliberate fail-open, and a fail-open guard is only safe
// while nobody mistakes its silence for success. Three of the four
// non-identifying outcomes — a dropped /health route, a panicking handler, a
// lost identity field — are HelixAgent REGRESSIONS, and the wire cannot
// separate them from a foreign occupant (see helixAgentOutcome). So an
// environment that KNOWS HelixAgent is deployed gets no useful signal from a
// skip: the suite goes green while the product is broken.
//
// HELIXAGENT_REQUIRED=1 is the operator's declaration that HelixAgent IS
// deployed at baseURL. Under it every non-identified outcome is a FAILURE,
// including "unreachable" — the flag asserts the service is there, and
// connection-refused contradicts that just as loudly as a 404 does.
// Default behaviour is unchanged for every caller that does not set it.
//
// t is an interface rather than *testing.T for one reason: a guard whose only
// observable effects are t.Skipf and t.Fatalf cannot be tested through
// *testing.T, because a failing subtest propagates its failure to the parent
// (common.Fail walks up the chain), so "assert that this guard fails" would
// fail the asserting test too. Taking the two-method subset lets the tests
// record the verdict instead of suffering it. Every call site passes a real
// *testing.T, which satisfies this unchanged.
func requireHelixAgent(t helixAgentGuardT, baseURL string) {
	t.Helper()

	probe := probeHelixAgent(baseURL)
	if probe.IsHelixAgent {
		return
	}

	if helixAgentRequired() {
		t.Fatalf("%s=1 declares a HelixAgent is deployed at %s, but the probe "+
			"did not find one (outcome=%s): %s",
			helixAgentRequiredEnv, baseURL, probe.Outcome, probe.Detail)
	}

	if !probe.Reachable {
		t.Skipf("HelixAgent not available: %s (SKIP-OK: #infra-unavailable)", probe.Detail)
	}

	// Reachable but not identified as HelixAgent. Set HELIXAGENT_REQUIRED=1
	// where the service is known-deployed so this becomes a failure instead.
	t.Skipf("HelixAgent not addressable (outcome=%s): %s (SKIP-OK: #infra-wrong-service-on-port)",
		probe.Outcome, probe.Detail)
}

// truncateForMessage renders a response body for a skip message without
// dumping an unbounded payload into test output.
func truncateForMessage(body []byte) string {
	const max = 120
	s := strings.TrimSpace(string(body))
	if s == "" {
		return "<empty>"
	}
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
