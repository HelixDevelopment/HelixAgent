package main

import (
	"strings"
	"testing"

	"dev.helix.agent/internal/ports"
	"github.com/stretchr/testify/require"
)

// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on a pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// Polarity helper redModeEnabled and the legacyGRPCAddr constant are shared
// with listen_addr_test.go, matching the convention that file established.
//
// CAPTURED RED BASELINE (2026-08-10, this host):
//
//	RED_MODE=1 -> PASS  (defect reproduced)
//	  startupBanner(":8112") == "HelixAgent gRPC server listening on :50051"
//	  the banner ignores its argument and prints the legacy literal verbatim
//	RED_MODE=0 -> FAIL:
//	  "the startup banner must name the address the server actually bound"
//	  expected substring ":8112"  actual ":50051"
//
// The baseline was captured against a FAITHFUL EXTRACTION of the original
// statement — startupBanner returning the hardcoded string verbatim,
// behaviour byte-identical to the pre-extraction `log.Println("...:50051")`
// — and both the extraction and the fix land in this one commit. The
// RED_MODE=0 assertions failed against that behaviour and pass after the
// fix, so this is not a blind test written to agree with the new code.
//
// THE DEFECT THIS GUARDS. Commit d2d70206 moved the listener off the
// contended literal but left the banner behind, twenty-five lines away in
// the same main():
//
//	addr := grpcListenAddr()                                  // ":8112"
//	lis, err := net.Listen("tcp", addr)                       // binds :8112
//	...
//	log.Println("HelixAgent gRPC server listening on :50051")  // UNTOUCHED
//
// The banner is not decoration — it is the address operators and downstream
// docs copy. A server that binds :8112 while instructing every reader to
// dial :50051 sends them to the helixcode-infra-weaviate container that
// publishes that port. The dial then APPEARS to succeed: a live peer
// completes a real HTTP/2 handshake and answers `Unimplemented` for every
// method, which is precisely the false-signal loop d2d70206 exists to end.
// A wrong banner is therefore worse than no banner — it manufactures the
// original defect's symptom out of the fix itself.
//
// WHY THIS TEST EXISTS AT ALL (§11.4.138). The banner survived the fix, its
// review, and the full suite because NO test asserted on it: the defect was
// invisible to the regime, not merely unnoticed. Registering this guard is
// the remediation for that coverage escape, not an optional extra — without
// it the next edit to main() can silently reintroduce the same divergence.

// TestStartupBanner_NamesTheBoundAddress asserts the banner reports the
// address the server actually binds, for whatever address it is given.
//
// Table-driven over several addresses because the defect's signature is
// precisely that the banner IGNORES its argument: a single-address check
// against the live registry value would still pass if someone re-hardcoded
// that value as a literal.
func TestStartupBanner_NamesTheBoundAddress(t *testing.T) {
	for _, addr := range []string{":8112", ":9112", "127.0.0.1:8112", ":36999"} {
		got := startupBanner(addr)

		if redModeEnabled(t) {
			require.Contains(t, got, legacyGRPCAddr,
				"RED_MODE=1: the pre-fix banner prints %s regardless of the "+
					"address bound (given %s)", legacyGRPCAddr, addr)
			continue
		}

		require.Contains(t, got, addr,
			"the startup banner must name the address the server actually "+
				"bound; operators and docs copy this line verbatim")
		require.NotContains(t, got, legacyGRPCAddr,
			"the banner must not direct operators to %s — this project's own "+
				"Weaviate container publishes it and answers Unimplemented, so "+
				"the mis-dial looks like a live HelixAgent with missing methods",
			legacyGRPCAddr)
	}
}

// TestStartupBanner_AgreesWithTheListener asserts the banner and the
// listener cannot disagree, which is the exact divergence that occurred.
//
// This is the invariant, stated once: whatever grpcListenAddr() resolves —
// registry default today, an operator's HELIXAGENT_PORT_GRPC override
// tomorrow — is what the banner must print. It needs no infrastructure, so
// it guards every run.
func TestStartupBanner_AgreesWithTheListener(t *testing.T) {
	addr := grpcListenAddr()
	got := startupBanner(addr)

	if redModeEnabled(t) {
		require.NotContains(t, got, ports.Addr(ports.HelixAgentGRPC),
			"RED_MODE=1: the pre-fix banner cannot name the registry address")
		return
	}

	require.Contains(t, got, addr,
		"banner and listener must never diverge: the server binds %s, so the "+
			"banner must say %s", addr, addr)
	require.Contains(t, got, ports.Addr(ports.HelixAgentGRPC),
		"the banner must publish the registry address, the one clients resolve")
}

// TestStartupBanner_IsNotDegenerate rejects the trivial ways this guard
// could be satisfied without the banner remaining useful.
//
// §1.1: a banner reduced to the bare address, or to an empty string, would
// pass the "contains addr" assertions above while destroying the line's
// purpose. Assert the identifying prose survives alongside the address.
func TestStartupBanner_IsNotDegenerate(t *testing.T) {
	if redModeEnabled(t) {
		t.Skip("shape assertions describe the fixed banner only") // SKIP-OK: #red-polarity
	}

	got := startupBanner(":8112")

	require.Contains(t, got, "HelixAgent",
		"the banner must still identify the service; operators read it to tell "+
			"our server apart from whatever else logs on this host")
	require.Contains(t, strings.ToLower(got), "grpc",
		"the banner must still identify the protocol")
	require.NotEqual(t, ":8112", strings.TrimSpace(got),
		"the banner must not collapse to a bare address")
}
