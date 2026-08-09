package main

import (
	"net"
	"os"
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
// The default flipped to GREEN in the same commit as the fix, per §11.4.115,
// matching the convention established by cmd/helixagent/redis_endpoint_test.go.
//
// CAPTURED RED BASELINE (2026-08-09, this host):
//
//	RED_MODE=1 -> PASS  (defect reproduced)
//	  grpcListenAddr() == ":50051"
//	  :50051 is held by rootlessport pid 1828579 -> helixcode-infra-weaviate
//	  binding ":50051" fails: listen tcp :50051: bind: address already in use
//	  binding the registry address ":8112" succeeds
//	RED_MODE=0 -> FAIL:
//	  "the gRPC server must listen on the registry address"
//	  expected: ":8112"  actual: ":50051"
//
// The RED baseline was captured on the faithful extraction of the original
// literal — grpcListenAddr() returning ":50051" verbatim, behaviour
// byte-identical to the pre-extraction main() — and both the extraction and
// the fix land in this one commit. The RED_MODE=0 assertion failed against
// that behaviour and passes after the fix, so it is not a blind test written
// to agree with the new code.
//
// THE PRODUCT DEFECT. The server hardcoded the gRPC ecosystem's conventional
// port and died on collision:
//
//	lis, err := net.Listen("tcp", ":50051")
//	if err != nil { log.Fatalf("failed to listen: %v", err) }
//
// 50051 is the port EVERY gRPC service reaches for by default, so it is the
// one most likely to be taken — and on this project's own stack it IS taken:
// the helixcode-infra-weaviate container publishes it. The result is that
// HelixAgent's gRPC server cannot start while HelixAgent's own
// infrastructure is up.
//
// The collision is worse than a failed start, because the port was ALSO
// absent from internal/ports — the registry whose entire purpose is to make
// exactly this arbitrable ("Ports were previously scattered ... 1000+ file
// references to raw integer literals"). A service the registry does not know
// about cannot be checked against the services it does know about, so the
// collision was undetectable by construction. Registering the service is
// therefore part of the fix, not an optional tidy-up.
//
// The second-order damage is a false RED in the test suite: clients dialling
// the same literal reach WEAVIATE and receive `Unimplemented` for every
// method. That is a live TCP peer completing an HTTP/2 handshake, so a
// reachability guard sees a healthy connection and does not skip — the
// failures are then reported against HelixAgent although HelixAgent was
// never running. Measured with a control needle: a FABRICATED service name
// returns the same `Unimplemented`, while `weaviate.v1.Weaviate/Search`
// executes — proving the responder is Weaviate, not a HelixAgent with
// missing methods.
//
// WHY THE FIX IS NOT "PICK ANOTHER FREE PORT" (§11.4.111). Any second
// literal is the same by-ordinal binding, just luckier for now. The service
// is registered in internal/ports (HelixAgentGRPC, core band offset 112) and
// resolved by name, so it is arbitrated against every other service by
// TestOffsets_NoCollisions and overridable per-deployment through the
// registry's own HELIXAGENT_PORT_GRPC env var — which is what makes the
// remaining collision risk operable instead of fatal.
//
// FAIL-FAST IS PRESERVED, DELIBERATELY. The fix does NOT make the server
// fall back to another port when the bind fails. Silently serving somewhere
// other than the address the registry publishes would strand every client
// that resolved correctly — a worse failure than refusing to start. The
// error message now names the address and the override variable so the
// refusal is actionable.

// redModeEnabled reports whether the RED polarity is selected. Absent or "0"
// means GREEN (assert the defect is absent) — the standing default.
func redModeEnabled(t *testing.T) bool {
	t.Helper()
	return strings.TrimSpace(os.Getenv("RED_MODE")) == "1"
}

// legacyGRPCAddr is the conventional-but-contended literal the server used
// to bind. On this host it belongs to the Weaviate container.
const legacyGRPCAddr = ":50051"

// TestGRPCListenAddr_ResolvesFromPortRegistry asserts the server's listen
// address comes from the port registry rather than a bare literal.
//
// Deterministic and infrastructure-free, so it guards the binding on every
// run — including in a bare container where no collision can be staged.
func TestGRPCListenAddr_ResolvesFromPortRegistry(t *testing.T) {
	got := grpcListenAddr()

	if redModeEnabled(t) {
		require.Equal(t, legacyGRPCAddr, got,
			"RED_MODE=1: the pre-fix server binds the hardcoded %s", legacyGRPCAddr)
		return
	}

	require.Equal(t, ports.Addr(ports.HelixAgentGRPC), got,
		"the gRPC server must listen on the registry address; a bare literal "+
			"cannot be arbitrated against the other registered services")
	require.NotEqual(t, legacyGRPCAddr, got,
		"the gRPC server must not re-adopt %s — this project's own Weaviate "+
			"container publishes it", legacyGRPCAddr)
}

// TestGRPCListenAddr_RegistryPortIsUniqueAcrossServices asserts the newly
// registered service does not shadow another service's port.
//
// This is the invariant the missing registry entry made uncheckable: an
// unregistered service cannot collide-check against registered ones.
// TestOffsets_NoCollisions in internal/ports covers offset uniqueness across
// the whole registry; this asserts the property from the consumer's side, so
// the gRPC server's own suite fails if someone later points it at a port
// another service already owns.
func TestGRPCListenAddr_RegistryPortIsUniqueAcrossServices(t *testing.T) {
	mine := ports.Get(ports.HelixAgentGRPC)

	for _, svc := range ports.All() {
		if svc == ports.HelixAgentGRPC {
			continue
		}
		require.NotEqual(t, mine, ports.Get(svc),
			"gRPC port %d is also registered to %s", mine, svc)
	}
}

// TestGRPCListenAddr_CollisionEvidence captures the physical proof of the
// defect: with the Weaviate container up, the legacy literal is unbindable
// while the registry address binds.
//
// HONEST INSTRUMENT (§11.4.6, §11.4.3). The collision can only be
// demonstrated while something actually holds the legacy port. Three
// outcomes, never collapsed:
//   - legacy port free       -> SKIP (the collision cannot be staged here;
//     reporting a pass would claim evidence that was never gathered)
//   - legacy port held       -> assert the legacy literal is unbindable AND
//     the registry address is bindable — the defect
//     and the fix, both measured on the same run
//   - registry port held     -> reported, not silently tolerated
func TestGRPCListenAddr_CollisionEvidence(t *testing.T) {
	legacyListener, legacyErr := net.Listen("tcp", legacyGRPCAddr)
	if legacyErr == nil {
		_ = legacyListener.Close()
		t.Skipf("%s is free on this host, so the collision that motivated "+
			"this fix cannot be reproduced here (SKIP-OK: #infra-unavailable)",
			legacyGRPCAddr) // SKIP-OK: #infra-unavailable
	}

	t.Logf("legacy address %s is occupied, as expected: %v",
		legacyGRPCAddr, legacyErr)

	// The registry address must be bindable while the legacy one is not —
	// that difference IS the fix, measured rather than asserted.
	registryAddr := ports.Addr(ports.HelixAgentGRPC)

	registryListener, registryErr := net.Listen("tcp", registryAddr)
	require.NoError(t, registryErr,
		"the registry address %s must be bindable while %s is occupied; if it "+
			"is not, the registry port needs re-allocating, not the test relaxing",
		registryAddr, legacyGRPCAddr)
	require.NoError(t, registryListener.Close())

	if redModeEnabled(t) {
		require.Equal(t, legacyGRPCAddr, grpcListenAddr(),
			"RED_MODE=1: the pre-fix server binds the address just proven "+
				"unbindable — it cannot start while our own stack is up")
		return
	}

	require.Equal(t, registryAddr, grpcListenAddr(),
		"the server must bind the address proven bindable, not the occupied one")
}
