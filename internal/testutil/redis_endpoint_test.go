package testutil

import (
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"dev.helix.agent/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
// These tests reproduce the Redis port-coupling defect on the CURRENT,
// pre-fix artifact and, with the polarity flipped, become the standing
// GREEN regression guard (§11.4.135) that the defect stays absent.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on a pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS NOW THE DEFAULT.
//
// The default was RED while the defect was live and flipped to GREEN in the
// same commit as the fix, per §11.4.115: one source, two roles — the
// bug-catcher IS the regression guard. To re-run the RED role against the
// pre-fix artifact: `git stash && RED_MODE=1 go test ./internal/testutil/`.
//
// CAPTURED RED BASELINE (2026-08-09, on the pre-fix tree):
//
//	RED_MODE=1 -> PASS  (defect reproduced)
//	RED_MODE=0 -> FAIL:
//	  TestRedisGate_ResolvesCanonicalRedisService
//	      expected: "8102"   actual: "8110"
//	  TestRedisGate_RejectsNonRedisListener
//	      Should be false  (bare-TCP gate accepted a non-Redis listener)
//
// Both assertions FAILED before the fix and PASS after it, so neither is a
// blind test that merely agrees with whatever the code does.
//
// Both polarities are infra-free and deterministic (§11.4.50): they bind
// only ephemeral loopback ports they allocate themselves, and never depend
// on a live Redis being up or down.
//
// THE DEFECT (measured 2026-08-08, suite log
// qa-results/helixagent_suite_20260808T171748Z.log):
//
//	--- FAIL: TestCheckRedisHealth (2.31s)
//	    main_test.go:1146: failed to connect to Redis:
//	                       dial tcp 127.0.0.1:6379: connect: connection refused
//
// That test DOES call testutil.RequireRedis(t) first — the guard is present
// and it did not skip. It did not skip because the guard and the code under
// test resolve DIFFERENT endpoints from the SAME env var:
//
//	guard   (DefaultInfraConfig) REDIS_PORT ?: 8110  = ports.RedisMCP
//	subject (checkRedisHealth)   REDIS_PORT ?: 6379  = pre-CONST-027 legacy
//
// With REDIS_PORT unset — the state of this host, which has no .env — the
// guard probed the password-protected MCP-backend Redis on 8110 (up), so it
// reported "available", and the subject then dialled 6379 (down) and failed.
// A guard that does not assert the condition its test depends on is a
// §11.4.201(1) false-negative gate: it let the test proceed into a
// guaranteed failure, which was then reported as a product defect.
//
// The canonical value is neither of those two. internal/ports (CONST-027)
// registers RedisDefault at offset 102 -> 8102, .env.example:276 documents
// REDIS_PORT=8102, and helix_agent's own docker-compose.yml:53 publishes
// "${REDIS_PORT:-8102}:6379". 8110 is RedisMCP — a DIFFERENT service.

// redModeEnabled reports whether the RED polarity is active. Opt-in, because
// the fix has landed and the GREEN guard is the role that ships.
func redModeEnabled() bool {
	return os.Getenv("RED_MODE") == "1"
}

// TestRedisGate_ResolvesCanonicalRedisService asserts that the availability
// gate points at the Redis the product actually uses (ports.RedisDefault),
// rather than at the password-protected MCP-backend Redis (ports.RedisMCP).
//
// This is a §11.4.111 resolve-by-stable-name defect: the gate was migrated
// off the legacy 6379 literal onto an 81xx registry port, but onto the entry
// for the WRONG service. infra_test.go:25 records the mis-binding in its own
// comment — `assert.Equal(t, "8110", cfg.RedisPort) // HELIXAGENT_PORT_REDIS_MCP`.
func TestRedisGate_ResolvesCanonicalRedisService(t *testing.T) {
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")

	canonical := strconv.Itoa(ports.Get(ports.RedisDefault))
	mcpBackend := strconv.Itoa(ports.Get(ports.RedisMCP))
	require.NotEqual(t, canonical, mcpBackend,
		"registry sanity: RedisDefault and RedisMCP must be distinct services")

	got := DefaultInfraConfig().RedisPort

	if redModeEnabled() {
		assert.Equal(t, mcpBackend, got,
			"RED baseline: the gate is expected to still be mis-bound to "+
				"ports.RedisMCP (the password-protected MCP-backend Redis). "+
				"If this assertion fails, the defect is already fixed — "+
				"re-run with RED_MODE=0.")
		assert.NotEqual(t, canonical, got,
			"RED baseline: the gate must NOT yet resolve the canonical Redis")
		return
	}

	assert.Equal(t, canonical, got,
		"the availability gate MUST resolve the same Redis service the "+
			"product dials (ports.RedisDefault, CONST-027), so a passing "+
			"guard actually implies the subject can connect")
}

// TestRedisGate_RejectsNonRedisListener is the golden-bad fixture that
// self-validates the gate itself (§11.4.107(10)).
//
// RedisAvailable() claims "Redis is reachable". Pre-fix it performed a bare
// TCP dial, so ANY process holding the port satisfied it. That is not a
// theoretical hole on this host: ports.RedisMCP (8110) is served by
// helixagent-mcp-redis-backend, which is password-protected and answers a
// bare PING with "-NOAUTH Authentication required." — measured 2026-08-08.
// A TCP-only probe reports that endpoint as available even though every
// command against it fails, so the gate cannot distinguish "Redis I can
// use" from "a socket that happens to be open".
//
// A guard must assert the condition it CLAIMS (§11.4.201). The fixture below
// is deliberately NOT Redis — it accepts the connection and says nothing —
// so a correct gate must reject it.
func TestRedisGate_RejectsNonRedisListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "golden-bad fixture: could not bind loopback listener")
	t.Cleanup(func() { _ = ln.Close() })

	// Accept and hold connections without ever speaking the Redis protocol.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			// Hold briefly, answer nothing, then drop. This is what a
			// non-Redis occupant of the port looks like on the wire.
			go func(c net.Conn) {
				time.Sleep(200 * time.Millisecond)
				_ = c.Close()
			}(conn)
		}
	}()

	host, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	t.Setenv("REDIS_HOST", host)
	t.Setenv("REDIS_PORT", port)
	t.Setenv("REDIS_PASSWORD", "")
	resetInfraCacheForTest()

	got := RedisAvailable()

	if redModeEnabled() {
		assert.True(t, got,
			"RED baseline: the bare-TCP gate is expected to accept a "+
				"non-Redis listener. If this fails, the gate already "+
				"verifies the Redis protocol — re-run with RED_MODE=0.")
		return
	}

	assert.False(t, got,
		"RedisAvailable() must reject an endpoint that is reachable but "+
			"does not answer the Redis protocol; a TCP-open check cannot "+
			"tell a usable Redis from a NOAUTH-rejecting or foreign occupant")
}

// resetInfraCacheForTest clears the once-per-run availability cache so a
// test that rewrites REDIS_HOST/REDIS_PORT observes its own endpoint rather
// than a verdict cached from an earlier probe. Mirrors the inline reset
// already used by TestDefaultInfraConfig_EnvOverride.
func resetInfraCacheForTest() {
	infraMu.Lock()
	infraResults = make(map[string]bool)
	infraMu.Unlock()
}
