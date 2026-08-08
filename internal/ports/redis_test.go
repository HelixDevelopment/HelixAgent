package ports

import (
	"strconv"
	"testing"
)

// Precedence contract for the shared Redis resolver (see redis.go).
//
// These are the assertions that keep guard and subject from drifting apart
// again: every consumer — checkRedisHealth, internal/config, the testutil
// availability gate, tests/testutils — now resolves through these functions,
// so the precedence proven here is the precedence the whole module obeys.

func TestRedisPort_DefaultsToRegistry(t *testing.T) {
	t.Setenv(RedisPortEnv, "")
	t.Setenv(string(RedisDefault), "")

	want := Get(RedisDefault)
	if got := RedisPort(); got != want {
		t.Errorf("RedisPort() with no override = %d, want %d (ports.RedisDefault)", got, want)
	}
}

func TestRedisPort_DefaultIsNotTheSupersededLiteral(t *testing.T) {
	t.Setenv(RedisPortEnv, "")
	t.Setenv(string(RedisDefault), "")

	// 6379 is the pre-CONST-027 value internal/ports records as superseded.
	// Nothing in this deployment publishes it; docker-compose.yml:53 publishes
	// ${REDIS_PORT:-8102}. A regression back to it is the exact defect that
	// produced "dial tcp 127.0.0.1:6379: connect: connection refused".
	if got := RedisPort(); got == 6379 {
		t.Errorf("RedisPort() regressed to the superseded literal 6379")
	}
}

func TestRedisPort_DefaultIsNotTheMCPBackend(t *testing.T) {
	t.Setenv(RedisPortEnv, "")
	t.Setenv(string(RedisDefault), "")

	// RedisMCP is a DIFFERENT service — the password-protected MCP backend.
	// The testutil gate used to resolve it by mistake, which is why a guard
	// could pass while the product's Redis was down.
	if got, mcp := RedisPort(), Get(RedisMCP); got == mcp {
		t.Errorf("RedisPort() = %d, which is ports.RedisMCP; the canonical "+
			"application Redis must be distinct from the MCP backend", got)
	}
}

func TestRedisPort_ExplicitEnvWins(t *testing.T) {
	t.Setenv(RedisPortEnv, "16379")
	if got := RedisPort(); got != 16379 {
		t.Errorf("RedisPort() = %d, want 16379: an explicit REDIS_PORT must "+
			"take precedence so existing deployments keep their behaviour", got)
	}
}

func TestRedisPort_RegistryEnvWinsOverDefault(t *testing.T) {
	t.Setenv(RedisPortEnv, "")
	t.Setenv(string(RedisDefault), "9102")
	if got := RedisPort(); got != 9102 {
		t.Errorf("RedisPort() = %d, want 9102: HELIXAGENT_PORT_REDIS must "+
			"override the registry default", got)
	}
}

func TestRedisPort_UnparseableOverrideFallsThrough(t *testing.T) {
	// An override we cannot interpret is operator error. Propagating it would
	// turn a typo into a confusing dial failure far from its cause, so it
	// falls through to the registry rather than being guessed at (§11.4.6).
	for _, bad := range []string{"abc", "0", "-1", "70000", "   "} {
		t.Run(bad, func(t *testing.T) {
			t.Setenv(RedisPortEnv, bad)
			t.Setenv(string(RedisDefault), "")
			if got, want := RedisPort(), Get(RedisDefault); got != want {
				t.Errorf("RedisPort() with REDIS_PORT=%q = %d, want registry default %d",
					bad, got, want)
			}
		})
	}
}

func TestRedisHost_DefaultAndOverride(t *testing.T) {
	t.Setenv(RedisHostEnv, "")
	if got := RedisHost(); got != DefaultRedisHost {
		t.Errorf("RedisHost() = %q, want %q", got, DefaultRedisHost)
	}

	t.Setenv(RedisHostEnv, "redis.internal")
	if got := RedisHost(); got != "redis.internal" {
		t.Errorf("RedisHost() = %q, want %q", got, "redis.internal")
	}
}

func TestRedisAddr_JoinsHostAndPort(t *testing.T) {
	t.Setenv(RedisHostEnv, "127.0.0.1")
	t.Setenv(RedisPortEnv, "8102")
	if got, want := RedisAddr(), "127.0.0.1:8102"; got != want {
		t.Errorf("RedisAddr() = %q, want %q", got, want)
	}
}

func TestRedisAddr_BracketsIPv6Host(t *testing.T) {
	// net.JoinHostPort, not string concatenation: "::1:8102" is not a
	// dialable address and go-redis would reject it.
	t.Setenv(RedisHostEnv, "::1")
	t.Setenv(RedisPortEnv, "8102")
	if got, want := RedisAddr(), "[::1]:8102"; got != want {
		t.Errorf("RedisAddr() = %q, want %q", got, want)
	}
}

func TestRedisPortString_MatchesRedisPort(t *testing.T) {
	t.Setenv(RedisPortEnv, "")
	t.Setenv(string(RedisDefault), "")
	if got, want := RedisPortString(), strconv.Itoa(RedisPort()); got != want {
		t.Errorf("RedisPortString() = %q, want %q", got, want)
	}
}

func TestRedisPassword_PassesThroughEnv(t *testing.T) {
	t.Setenv(RedisPasswordEnv, "")
	if got := RedisPassword(); got != "" {
		t.Errorf("RedisPassword() = %q, want empty", got)
	}

	t.Setenv(RedisPasswordEnv, "helixagent123")
	if got := RedisPassword(); got != "helixagent123" {
		t.Errorf("RedisPassword() = %q, want %q", got, "helixagent123")
	}
}
