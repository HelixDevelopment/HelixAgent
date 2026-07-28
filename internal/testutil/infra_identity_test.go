package testutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Server-identity regression guard (§11.4.111 / §11.4.194 / §11.4.201 / §11.4.135).
//
// THE DEFECT THIS GUARDS (captured 2026-07-28)
// ServerAvailable used checkHTTP, which accepts ANY status < 500. So a
// DIFFERENT service occupying the configured port satisfied it. On the
// development host llmsverifier legitimately owns :8100 (helix_agent's tests
// default HELIXAGENT_PORT to the same 8100) and answers `404 page not found`.
// 404 < 500, so ServerAvailable returned true, RequireServer did NOT skip, and
// roughly a dozen packages then fired real requests at the wrong service and
// failed on the wrong-shaped replies — reported as product failures with
// nothing wrong with the product.
//
// A guard must assert the condition it CLAIMS ("the HelixAgent server is
// reachable"), not a proxy another process can satisfy. These cases are
// hermetic — they stand up their own servers, so the guard keeps working on
// any host regardless of what happens to hold port 8100.
//
// CORRECTION (2026-07-28, §11.4.194 code review): a same-day earlier revision
// of this guard pinned a bare {"status":"healthy"} payload — the exact shape
// this repo's own mock LLM server (challenges/codebase/mock_server/main.go
// healthHandler) returns on /health — as an ACCEPTED ("real HelixAgent")
// fixture. Its commit message claimed that adding "service":"helixagent" to
// router.go's /health response "closes that false-GREEN vector at the
// source"; that claim was wrong, because checkHelixAgentHealth still fell
// back to accepting ANY {"status":"healthy"} body when no "service" field
// was present — which is exactly what the mock returns. checkHelixAgentHealth
// now probes /v1/health and requires the "providers" key (present on every
// real HelixAgent /v1/health response since the endpoint's original
// introduction, commit 4c968f22, 2025-12-10 — long before "service" existed),
// which the mock cannot produce since it has no /v1/health handler at all.
// The fixtures below are updated accordingly.
func TestCheckHelixAgentHealth_RejectsForeignOccupant(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		ctype   string
		wantOK  bool
		because string
	}{
		{
			name: "foreign_service_404_plaintext", status: http.StatusNotFound,
			body: "404 page not found", ctype: "text/plain",
			wantOK:  false,
			because: "the exact shape llmsverifier returns on :8100 — the original false positive",
		},
		{
			name: "foreign_service_200_but_not_helixagent", status: http.StatusOK,
			body: `{"service":"llmsverifier","status":"healthy"}`, ctype: "application/json",
			wantOK:  false,
			because: "answers 200 with a healthy payload but explicitly identifies as another service",
		},
		{
			name: "unauthorized", status: http.StatusUnauthorized,
			body: `{"error":"unauthorized"}`, ctype: "application/json",
			wantOK:  false,
			because: "401 is < 500 and would have passed the old check",
		},
		{
			name: "server_error", status: http.StatusInternalServerError,
			body: "boom", ctype: "text/plain",
			wantOK: false, because: "a broken server is not an available one",
		},
		{
			// This is the mock LLM server's OWN /health payload, byte-for-byte
			// (challenges/codebase/mock_server/main.go healthHandler). An
			// earlier revision of this fixture (named "real_helixagent")
			// wrongly asserted this exact payload as ACCEPTED — that was
			// precisely the false-GREEN vector the §11.4.194 review flagged:
			// checkHelixAgentHealth's old bare-/health "status"=="healthy"
			// fallback could not tell this apart from a real server. Now that
			// the check requires "providers" (which the mock's /health
			// response never carries), this payload is correctly rejected.
			name: "status_only_payload_missing_providers_now_rejected", status: http.StatusOK,
			body: `{"status":"healthy"}`, ctype: "application/json",
			wantOK:  false,
			because: "the mock LLM server's exact /health payload shape — no \"providers\" key, so it must be rejected regardless of status; a prior revision of this fixture wrongly accepted it",
		},
		{
			name: "providers_present_with_explicit_service_field", status: http.StatusOK,
			body: `{"service":"helixagent","status":"healthy","providers":{"total":1,"healthy":1,"unhealthy":0}}`, ctype: "application/json",
			wantOK:  true,
			because: "the mandatory \"providers\" key is present, plus the preferred stronger \"service\" identity field — honoured when the server provides it",
		},
		{
			name: "providers_present_without_service_field_accepted_via_status_fallback", status: http.StatusOK,
			body: `{"status":"healthy","providers":{"total":1,"healthy":1,"unhealthy":0}}`, ctype: "application/json",
			wantOK:  true,
			because: "the mandatory \"providers\" key is present but no \"service\" field is — must still be accepted via the status==\"healthy\" fallback, since \"providers\" alone is the identity gate now, \"service\" is only an optional stronger signal when present",
		},
		{
			// Locks in the exact payload router.go's GET /v1/health emits
			// (§11.4.111 identity-binding fix, §11.4.194 corrected 2026-07-28).
			// This repo's OWN mock LLM server (challenges/codebase/mock_server/
			// main.go healthHandler) answers bare {"status":"healthy"} on
			// /health and has NO /v1/health handler at all, so it cannot
			// produce this shape. This case pins router.go's full /v1/health
			// shape (service field alongside providers/timestamp) so a future
			// edit that drops any of those fields is caught here, not
			// discovered live against the mock.
			name: "real_helixagent_v1_health_payload_with_providers_and_timestamp", status: http.StatusOK,
			body: `{"service":"helixagent","status":"healthy","providers":{"total":2,"healthy":2,"unhealthy":0},"timestamp":1793270400}`, ctype: "application/json",
			wantOK:  true,
			because: "the exact /v1/health shape router.go emits: the service identity field alongside its pre-existing provider-stats/timestamp fields must still identify as helixagent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.ctype)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()

			got := checkHelixAgentHealth(srv.URL+"/v1/health", 3*time.Second)
			if got != tc.wantOK {
				t.Errorf("checkHelixAgentHealth = %v, want %v — %s", got, tc.wantOK, tc.because)
			}
		})
	}
}

// TestCheckHelixAgentHealth_UnreachableIsNotAvailable pins the trivial case:
// nothing listening must not read as available.
func TestCheckHelixAgentHealth_UnreachableIsNotAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL + "/v1/health"
	srv.Close() // closed before the probe — connection must be refused

	if checkHelixAgentHealth(url, 2*time.Second) {
		t.Error("a closed server was reported as an available HelixAgent")
	}
}
