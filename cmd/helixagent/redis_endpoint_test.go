package main

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"dev.helix.agent/internal/ports"
	"github.com/stretchr/testify/assert"
)

// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on a pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS NOW THE DEFAULT.
//
// The default flipped to GREEN in the same commit as the fix, per §11.4.115.
//
// CAPTURED RED BASELINE (2026-08-09, pre-fix tree):
//
//	RED_MODE=1 -> PASS  (defect reproduced: error named :6379)
//	RED_MODE=0 -> FAIL:
//	  "failed to connect to Redis: dial tcp 127.0.0.1:6379: connect:
//	   connection refused" should not contain ":6379"
//	  ... does not contain ":8102"
//
// The RED_MODE=0 assertion failed before the fix and passes after it, so it
// is not a blind test written to agree with the new code.
//
// THE PRODUCT DEFECT. checkRedisHealth() resolves its port as
//
//	redisPort := os.Getenv("REDIS_PORT"); if redisPort == "" { redisPort = "6379" }
//
// 6379 is the pre-CONST-027 literal that internal/ports explicitly records as
// superseded — `RedisDefault — no-password Redis (was 6379)` — and re-registers
// at offset 102, i.e. 8102. helix_agent's own docker-compose.yml:53 publishes
// "${REDIS_PORT:-8102}:6379" and .env.example:276 documents REDIS_PORT=8102.
// So the shipped default names a port that this project's own deployment never
// publishes, and nothing on this host listens on 6379 (measured 2026-08-08).
//
// This is the same defect class as the precedent already recorded in
// scripts/systemd/helixllm-gateway.service, where HELIX_REDIS_PORT fell back to
// 6379, nothing listened there, and the gateway logged
// "dial tcp 127.0.0.1:6379: connect: connection refused" on every start.
//
// The assertion is written on the DIALLED ADDRESS rather than on success, so it
// stays valid whether or not a Redis is running: the point is not "Redis is up",
// it is "the binary aims at the port this project actually publishes".

// legacyRedisPort is the pre-CONST-027 literal the product must no longer
// fall back to.
const legacyRedisPort = "6379"

// TestCheckRedisHealth_DialsCanonicalRegistryPort asserts that, with no
// REDIS_PORT override, checkRedisHealth() aims at ports.RedisDefault.
func TestCheckRedisHealth_DialsCanonicalRegistryPort(t *testing.T) {
	t.Setenv("REDIS_HOST", "127.0.0.1")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("REDIS_PASSWORD", "")

	canonical := strconv.Itoa(ports.Get(ports.RedisDefault))

	err := checkRedisHealth()

	// A dial failure is the expected shape while no Redis is bound to the
	// canonical port; what matters is WHICH port it names. If Redis is in
	// fact up, err is nil (or an auth error) and no port is named — that
	// cannot be evidence of the wrong port, so the wrong-port assertions
	// only apply when a dial error was actually produced.
	dialed := ""
	if err != nil {
		dialed = err.Error()
	}
	isDialErr := strings.Contains(dialed, "dial tcp")

	if os.Getenv("RED_MODE") == "1" {
		if !isDialErr {
			t.Skipf("RED baseline needs an observable dial error to read the "+
				"port off; got err=%v. Bring the endpoint down, or run with "+
				"RED_MODE=0 to assert the fixed behaviour.", err) // SKIP-OK: #red-baseline-needs-dial-error
		}
		assert.Contains(t, dialed, ":"+legacyRedisPort,
			"RED baseline: checkRedisHealth is expected to still fall back to "+
				"the pre-CONST-027 literal 6379. If this fails, the defect is "+
				"already fixed — re-run with RED_MODE=0.")
		return
	}

	assert.NotContains(t, dialed, ":"+legacyRedisPort,
		"checkRedisHealth must not fall back to the superseded 6379 literal; "+
			"internal/ports registers RedisDefault at %s and docker-compose.yml "+
			"publishes it there", canonical)

	if isDialErr {
		assert.Contains(t, dialed, ":"+canonical,
			"checkRedisHealth must dial the canonical registry port for "+
				"ports.RedisDefault so the endpoint it aims at is the one this "+
				"project's own compose publishes")
	}
}

// TestCheckRedisHealth_HonoursExplicitOverride pins the precedence contract:
// an explicit REDIS_PORT still wins over the registry default, so every
// existing deployment that sets it keeps its exact current behaviour.
func TestCheckRedisHealth_HonoursExplicitOverride(t *testing.T) {
	// Port 1 is reserved and not listening, so the dial is guaranteed to
	// fail fast and name the port back to us.
	t.Setenv("REDIS_HOST", "127.0.0.1")
	t.Setenv("REDIS_PORT", "1")
	t.Setenv("REDIS_PASSWORD", "")

	err := checkRedisHealth()
	if assert.Error(t, err, "dialing a closed reserved port must fail") {
		assert.Contains(t, err.Error(), ":1:",
			"an explicit REDIS_PORT must take precedence over the registry default")
	}
}
