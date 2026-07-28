package testutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Server-identity regression guard (§11.4.111 / §11.4.201 / §11.4.135).
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
			name: "real_helixagent", status: http.StatusOK,
			body: `{"status":"healthy"}`, ctype: "application/json",
			wantOK:  true,
			because: "the payload a real HelixAgent /health returns",
		},
		{
			name: "real_helixagent_with_explicit_service_field", status: http.StatusOK,
			body: `{"service":"helixagent","status":"healthy"}`, ctype: "application/json",
			wantOK:  true,
			because: "preferred stronger identity, honoured when the server provides it",
		},
		{
			// Locks in the exact payload router.go's GET /health and
			// GET /v1/health now emit (§11.4.111 identity-binding fix). Before
			// this fix, router.go's /health answered bare {"status":"healthy"}
			// — byte-identical to this repo's OWN mock LLM server
			// (challenges/codebase/mock_server/main.go healthHandler), which
			// also implements /v1/chat/completions with fabricated replies.
			// A HELIXAGENT_HOST/PORT pointed at the mock would have opened
			// this gate and let downstream tests PASS AGAINST THE MOCK — a
			// false-GREEN strictly worse than the wrong-service false-RED this
			// batch fixed elsewhere. This case pins router.go's richer
			// /v1/health shape (service field alongside the pre-existing
			// providers/timestamp fields) so a future edit that drops the
			// field is caught here, not discovered live against a mock.
			name: "real_helixagent_v1_health_payload_with_providers_and_timestamp", status: http.StatusOK,
			body: `{"service":"helixagent","status":"healthy","providers":{"total":2,"healthy":2,"unhealthy":0},"timestamp":1793270400}`, ctype: "application/json",
			wantOK:  true,
			because: "the exact /v1/health shape router.go emits post-fix: the service identity field alongside its pre-existing provider-stats/timestamp fields must still identify as helixagent",
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

			got := checkHelixAgentHealth(srv.URL+"/health", 3*time.Second)
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
	url := srv.URL + "/health"
	srv.Close() // closed before the probe — connection must be refused

	if checkHelixAgentHealth(url, 2*time.Second) {
		t.Error("a closed server was reported as an available HelixAgent")
	}
}
