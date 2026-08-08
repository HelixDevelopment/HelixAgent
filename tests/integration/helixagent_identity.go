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
	// Detail is a human-readable description of what answered, for use in
	// skip messages. Never empty when IsHelixAgent is false.
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
			Detail: fmt.Sprintf("no service is reachable at %s (%v)", baseURL, err),
		}
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if readErr != nil {
		return helixAgentProbe{
			Reachable:  true,
			StatusCode: resp.StatusCode,
			Detail: fmt.Sprintf("a service at %s answered HTTP %d but its body could not be read (%v)",
				baseURL, resp.StatusCode, readErr),
		}
	}

	probe := helixAgentProbe{Reachable: true, StatusCode: resp.StatusCode}

	// The identity claim lives in the JSON body. A non-JSON body (the
	// classic "404 page not found" of a foreign Go service) can never
	// satisfy it, which is precisely the case this guard exists to catch.
	var payload struct {
		Service string `json:"service"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Service == "" {
		probe.Detail = fmt.Sprintf(
			"a service at %s answered HTTP %d but did not identify as %q "+
				"(body: %s) — another process is bound to this port; "+
				"point HELIXAGENT_URL at the real HelixAgent",
			baseURL, resp.StatusCode, helixAgentServiceName, truncateForMessage(body))
		return probe
	}

	if payload.Service != helixAgentServiceName {
		probe.Detail = fmt.Sprintf(
			"the service at %s identifies as %q, not %q — another process is "+
				"bound to this port; point HELIXAGENT_URL at the real HelixAgent",
			baseURL, payload.Service, helixAgentServiceName)
		return probe
	}

	probe.IsHelixAgent = true
	return probe
}

// requireHelixAgent asserts that baseURL is served by a real HelixAgent
// before the caller asserts anything about its behaviour.
//
// It SKIPs (never fails) when HelixAgent is absent or when a different
// service holds the port, because in neither case has the caller observed
// HelixAgent at all. Once it returns, a non-200 from HelixAgent is a genuine
// product signal and MUST be allowed to fail the test.
func requireHelixAgent(t *testing.T, baseURL string) {
	t.Helper()

	probe := probeHelixAgent(baseURL)
	if probe.IsHelixAgent {
		return
	}

	if !probe.Reachable {
		t.Skipf("HelixAgent not available: %s (SKIP-OK: #infra-unavailable)", probe.Detail)
	}

	// Reachable but not HelixAgent: the port is occupied by something else.
	// This is an environment defect with a precise, actionable cause, and it
	// must not be reported as a HelixAgent failure.
	t.Skipf("HelixAgent not addressable: %s (SKIP-OK: #infra-wrong-service-on-port)", probe.Detail)
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
