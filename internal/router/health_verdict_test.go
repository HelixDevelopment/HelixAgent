package router

import (
	"errors"
	"os"
	"testing"
)

// TestHealthVerdict is the permanent regression guard for HXC-244: the
// /v1/health endpoint computed the per-provider healthy/unhealthy counts and
// then DISCARDED them, hard-coding "status":"healthy". An agent with every
// provider down — i.e. one that cannot serve a single completion, its entire
// reason to exist — still reported itself healthy.
//
// §11.4.115 polarity switch, one source, two roles:
//
//	RED_MODE=1  reproduce-and-assert-the-defect-is-PRESENT. Passes against the
//	            pre-fix artifact (verdict is the constant "healthy"), fails
//	            once the fix lands. This is the proof the guard is real and
//	            not merely in agreement with whatever the code happens to do.
//	RED_MODE=0  (default) the standing GREEN guard: the verdict must reflect
//	            the actual provider state.
//
// Note the pre-existing /health tests (router_test.go:26,
// router_integration_test.go:27) register their OWN inline stub handlers and
// assert a literal they wrote two lines earlier — they never execute the
// shipping handler, so they could not have caught this and cannot catch its
// recurrence. This test exercises the real shipping verdict function.
func TestHealthVerdict(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	down := errors.New("connection refused")

	cases := []struct {
		name string
		in   map[string]error
		// want is the verdict the FIXED code must produce.
		want string
		// wantHealthy / wantUnhealthy are counts, asserted in both modes:
		// the counts were always computed correctly, only the verdict lied.
		wantHealthy   int
		wantUnhealthy int
		// servable records whether the agent can serve any completion at all.
		// Every non-servable case is one where the pre-fix code claimed
		// "healthy" — those are the cases RED_MODE=1 pins down.
		servable bool
	}{
		{
			name:     "no providers registered at all",
			in:       map[string]error{},
			want:     "unhealthy",
			servable: false,
		},
		{
			name:     "nil map (boundary)",
			in:       nil,
			want:     "unhealthy",
			servable: false,
		},
		{
			name:          "every provider down",
			in:            map[string]error{"openai": down, "ollama": down},
			want:          "unhealthy",
			wantUnhealthy: 2,
			servable:      false,
		},
		{
			name:          "sole provider down",
			in:            map[string]error{"ollama": down},
			want:          "unhealthy",
			wantUnhealthy: 1,
			servable:      false,
		},
		{
			name:          "partially degraded, still servable",
			in:            map[string]error{"openai": nil, "ollama": down},
			want:          "degraded",
			wantHealthy:   1,
			wantUnhealthy: 1,
			servable:      true,
		},
		{
			name:        "sole provider healthy",
			in:          map[string]error{"ollama": nil},
			want:        "healthy",
			wantHealthy: 1,
			servable:    true,
		},
		{
			name:        "every provider healthy",
			in:          map[string]error{"openai": nil, "ollama": nil},
			want:        "healthy",
			wantHealthy: 2,
			servable:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, healthy, unhealthy := healthVerdict(tc.in)

			if healthy != tc.wantHealthy {
				t.Errorf("healthy count = %d, want %d", healthy, tc.wantHealthy)
			}
			if unhealthy != tc.wantUnhealthy {
				t.Errorf("unhealthy count = %d, want %d", unhealthy, tc.wantUnhealthy)
			}

			if redMode {
				// Reproduce the defect: for every non-servable case the
				// pre-fix verdict was the constant "healthy".
				if !tc.servable && got != "healthy" {
					t.Errorf("RED_MODE: expected the historical defect "+
						"(verdict %q for an unservable agent), got %q — "+
						"the defect is absent, so the fix is in place",
						"healthy", got)
				}
				return
			}

			if got != tc.want {
				t.Errorf("healthVerdict() = %q, want %q "+
					"(healthy=%d unhealthy=%d)", got, tc.want, healthy, unhealthy)
			}

			// The load-bearing invariant, stated independently of the
			// specific vocabulary: an agent that cannot serve a single
			// completion must never report itself healthy.
			if !tc.servable && got == "healthy" {
				t.Errorf("verdict %q for an agent with zero healthy providers: "+
					"a health check that cannot fail is not a health check", got)
			}
		})
	}
}
