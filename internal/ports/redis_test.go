package ports

import (
	"net"
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

// TestRedisAddr_AlreadyBracketedIPv6HostIsNotDoubleBracketed is the HXC-286 /
// F2 regression guard (§11.4.135) for the defect the round-5 review measured:
// RedisAddr composed with net.JoinHostPort directly, and JoinHostPort brackets
// an ALREADY-BRACKETED host a SECOND time.
//
// RED baseline on the pre-fix source (measured 2026-08-13, this repository's
// Go toolchain, with RedisAddr's body reverted to
// `net.JoinHostPort(RedisHost(), RedisPortString())`):
//
//	REDIS_HOST=[::1]  ->  "[[::1]]:8102"
//	  net.SplitHostPort -> address [[::1]]:8102: missing port in address
//	  url.Parse         -> invalid IP-literal
//
// `[::1]` is not an exotic input: RedisHost applies no bracket handling (it
// trims, guards non-empty, and otherwise falls back to DefaultRedisHost — NOT
// a "bare TrimSpace", as an earlier revision of this comment said; round-8
// review finding 6), and the bracketed spelling is what an operator copies out
// of a URL. The address then reaches go-redis as the client Addr
// (cmd/helixagent/main.go:842) and the dependency health probe (:630).
//
// The guard asserts BOTH spellings compose to the SAME dialable authority,
// and that the result actually splits — a string comparison alone would not
// notice a differently-broken address.
func TestRedisAddr_AlreadyBracketedIPv6HostIsNotDoubleBracketed(t *testing.T) {
	t.Setenv(RedisPortEnv, "8102")

	for _, host := range []string{"::1", "[::1]", "  [::1]  "} {
		t.Setenv(RedisHostEnv, host)
		got := RedisAddr()
		if want := "[::1]:8102"; got != want {
			t.Fatalf("RedisAddr() with REDIS_HOST=%q = %q, want %q "+
				"(a second layer of brackets makes the address unsplittable)", host, got, want)
		}
		h, p, err := net.SplitHostPort(got)
		if err != nil {
			t.Fatalf("RedisAddr() = %q for REDIS_HOST=%q is not splittable: %v", got, host, err)
		}
		if h != "::1" || p != "8102" {
			t.Fatalf("RedisAddr() = %q split to host=%q port=%q, want host=%q port=%q",
				got, h, p, "::1", "8102")
		}
	}
}

// TestRedisAddr_AgreesWithHostPort pins the intra-package agreement the F2 fix
// restores. ports.HostPort already routed through netaddr; ports.RedisAddr did
// not, so the two builders in THIS package answered differently for the
// identical bracketed host — the "two builders privately disagreeing about the
// same logical endpoint" defect netaddr's package doc names as this family's
// core. They must not be able to drift apart again.
func TestRedisAddr_AgreesWithHostPort(t *testing.T) {
	t.Setenv(RedisPortEnv, strconv.Itoa(Get(RedisDefault)))

	for _, host := range []string{"::1", "[::1]", "127.0.0.1", "redis.internal", "  [fe80::1]  "} {
		t.Setenv(RedisHostEnv, host)
		got := RedisAddr()
		want := HostPort(RedisDefault, host)
		if got != want {
			t.Errorf("RedisAddr() = %q but HostPort(RedisDefault, %q) = %q — "+
				"two builders in one package must not disagree about one address", got, host, want)
		}
	}
}

// TestRedisAddr_HonoursRedisPortEnv is the round-9/F1 regression guard
// (§11.4.135) for a mutant that SURVIVED the entire shipped suite: replacing
// `RedisPort()` with `Get(RedisDefault)` in RedisAddr's body — RedisAddr
// silently ignoring REDIS_PORT and dialling the registry default instead.
//
// WHY IT SURVIVED — a fixture pattern, not a one-off. Every pre-existing
// RedisAddr fixture pinned REDIS_PORT to a value EQUAL to Get(RedisDefault)
// (8102): :99, :109, :159 and :186 — the last of which is FORCED to that
// value, because it compares against HostPort(RedisDefault, ...), which
// resolves its port through Get, not RedisPort. When the env value and the
// fallback coincide, the correct and the broken implementation emit the SAME
// string, so the wiring is invisible. Contrast the HOST axis, which IS
// covered precisely because its fixtures VARY the env (:91, :162, :189).
// TestRedisPort_ExplicitEnvWins (:52) covers RedisPort() in ISOLATION;
// nothing asserted that RedisAddr() actually USES it.
//
// No ambient REDIS_PORT can expose this from outside the suite: every fixture
// calls t.Setenv, which shadows the process environment. Only a fixture row
// carrying a non-default port can.
//
// Measured 2026-08-13, this repository's Go toolchain, with RedisAddr's body
// mutated to `netaddr.DialAddress(RedisHost(), Get(RedisDefault))`:
//
//	shipped suite, pre-guard -> ok  dev.helix.agent/internal/ports  (GREEN)
//	this guard               -> RedisAddr() = "127.0.0.1:8102", want "127.0.0.1:9999"
//
// It matters on the LIVE dial path, not a synthetic one: RedisAddr() is the
// go-redis client's Addr (cmd/helixagent/main.go:842) and the "redis" entry of
// the dependency health-probe map (:630). An operator setting REDIS_PORT=9999
// would have dialled 8102 with a fully green suite — the §11.4.108
// source-vs-wiring class, in the one function this round changed.
func TestRedisAddr_HonoursRedisPortEnv(t *testing.T) {
	// Registry override off, so Get(RedisDefault) yields the pure default —
	// the value RedisAddr would emit if the wiring were broken.
	t.Setenv(string(RedisDefault), "")

	const overridePort = "9999"
	if fallback := strconv.Itoa(Get(RedisDefault)); fallback == overridePort {
		t.Fatalf("degenerate fixture: REDIS_PORT=%s equals the fallback Get(RedisDefault)=%s, "+
			"so this test could not tell RedisPort() from the registry default — "+
			"choose an override port that differs", overridePort, fallback)
	}

	// Both composition spellings — the plain path and the bracket-stripping
	// path — so the port wiring cannot be correct on only one of them.
	for _, tc := range []struct{ host, want string }{
		{"127.0.0.1", "127.0.0.1:" + overridePort},
		{"[::1]", "[::1]:" + overridePort},
	} {
		t.Setenv(RedisHostEnv, tc.host)
		t.Setenv(RedisPortEnv, overridePort)
		if got := RedisAddr(); got != tc.want {
			t.Errorf("RedisAddr() with REDIS_HOST=%q REDIS_PORT=%s = %q, want %q — "+
				"RedisAddr must resolve its port through RedisPort(), which honours "+
				"REDIS_PORT, not through the registry default",
				tc.host, overridePort, got, tc.want)
		}
	}
}
