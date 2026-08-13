package ports

import (
	"os"
	"strconv"
	"strings"

	"dev.helix.agent/internal/netaddr"
)

// Redis endpoint resolution — the single source of truth for "where is Redis".
//
// WHY THIS EXISTS (§11.4.111 / §11.4.201, added 2026-08-09)
//
// Before this file, seven places answered "which port is Redis on?" and gave
// five different answers, all keyed off the SAME env var (REDIS_PORT) and
// differing only in their fallback:
//
//	internal/ports (CONST-027 registry)        8102   canonical
//	.env.example:276  REDIS_PORT=8102          8102   documented contract
//	docker-compose.yml:53 ${REDIS_PORT:-8102}  8102   our own deployment
//	cmd/helixagent   checkRedisHealth()        6379   pre-CONST-027 legacy
//	internal/config  config.go:341, :508       6379   pre-CONST-027 legacy
//	internal/testutil  DefaultInfraConfig()    8110   WRONG SERVICE (RedisMCP)
//	tests/testutils  SkipIfNoRedis()          16379   pre-CONST-027 legacy
//
// The divergence was not cosmetic. It produced a live false-negative gate:
// TestCheckRedisHealth called testutil.RequireRedis(t), the guard probed 8110
// (the password-protected MCP-backend Redis, which is up on the dev host), did
// not skip, and the subject then dialled 6379 (down) and failed —
//
//	dial tcp 127.0.0.1:6379: connect: connection refused
//
// — so an absent-infrastructure condition was reported as a product failure.
// Measured 2026-08-08 in qa-results/helixagent_suite_20260808T171748Z.log.
//
// Resolving through one function makes guard and subject agree BY CONSTRUCTION
// rather than by two authors independently remembering the same number.
//
// PRECEDENCE (highest first) — chosen so no existing deployment changes
// behaviour; only the terminal fallback moves off the superseded literal:
//
//	1. REDIS_PORT / REDIS_HOST / REDIS_PASSWORD — the historical contract that
//	   .env.example, every compose file and every existing operator env sets.
//	2. HELIXAGENT_PORT_REDIS — the CONST-027 registry override (via Get).
//	3. The registry default, Prefix()*1000 + 102 = 8102.
//
// NOTE ON SCOPE. This resolves the CANONICAL application Redis
// (RedisDefault). It is deliberately NOT used for RedisMCP (8110), which is a
// separate, password-protected MCP backend, nor for the standalone Redis MCP
// adapter in internal/mcp/servers, whose 6379 default is the upstream Redis
// convention for a reusable adapter rather than a reference to a port in this
// deployment (§11.4.28).

// RedisHostEnv, RedisPortEnv and RedisPasswordEnv are the operator-facing
// env vars this resolver honours, named here so callers and docs cannot
// drift from the implementation.
const (
	RedisHostEnv     = "REDIS_HOST"
	RedisPortEnv     = "REDIS_PORT"
	RedisPasswordEnv = "REDIS_PASSWORD"
)

// DefaultRedisHost is the host assumed when REDIS_HOST is unset.
const DefaultRedisHost = "localhost"

// RedisHost returns the Redis host: REDIS_HOST when set, else localhost.
func RedisHost() string {
	if v := strings.TrimSpace(os.Getenv(RedisHostEnv)); v != "" {
		return v
	}
	return DefaultRedisHost
}

// RedisPort returns the Redis TCP port following the documented precedence.
//
// A REDIS_PORT that is absent, blank, or not a valid TCP port falls through
// to the registry rather than being propagated — an unparseable override is
// operator error, and silently dialling port 0 or a negative port would turn
// that into a confusing connection failure somewhere far away (§11.4.6: a
// value we cannot interpret is not a value we may guess at).
func RedisPort() int {
	if raw := strings.TrimSpace(os.Getenv(RedisPortEnv)); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n < 65536 {
			return n
		}
	}
	return Get(RedisDefault)
}

// RedisPortString returns RedisPort as a string, for the many call sites that
// carry ports as strings (config structs, DSNs, env export).
func RedisPortString() string {
	return strconv.Itoa(RedisPort())
}

// RedisAddr returns the dialable "host:port" for the canonical Redis.
//
// HXC-286 / F2 (2026-08-13): this composes through netaddr rather than
// net.JoinHostPort directly, because JoinHostPort is NOT safe on an
// ALREADY-BRACKETED host — it brackets a second time. Measured against this
// repository's Go toolchain:
//
//	net.JoinHostPort("[::1]", "6379") = "[[::1]]:6379"
//	  net.SplitHostPort -> address [[::1]]:6379: missing port in address
//	  url.Parse         -> invalid IP-literal
//
// RedisHost reads REDIS_HOST with no bracket handling (it trims, guards
// non-empty, and otherwise falls back to DefaultRedisHost — see its own doc),
// so `REDIS_HOST=[::1]` — the form an operator copies straight out of a URL —
// reached JoinHostPort already bracketed and produced that unsplittable
// double-bracket address. netaddr.DialAddress strips one layer of brackets
// first and only then joins, so both spellings of an IPv6 literal compose to
// the same dialable authority.
//
// WHY DialAddress AND NOT DialAddressString (round-8 review finding 4). An
// earlier revision of this fix routed through netaddr.DialAddressString with
// RedisPortString(). Both spellings are provably identical in behaviour —
// netaddr.stripBracket and helixendpoint.unbracket are byte-identical bodies,
// so the two entry points differ only in where strconv.Itoa runs — but
// DialAddressString was the WRONG entry point on its OWN documented contract:
// it exists for a caller "whose port is already a string (a config field typed
// `Port string`)", because forcing such a caller through Atoi-then-DialAddress
// is LOSSY on a malformed value. RedisAddr is the exact opposite case. Its
// port originates as a VALIDATED int from RedisPort() (bounds-checked
// 0 < n < 65536, otherwise the registry default), and RedisPortString() is
// literally strconv.Itoa(RedisPort()) — so there is no malformed string to
// preserve and the Itoa round-trip bought nothing. Routing through DialAddress
// also puts this call on the SAME underlying bracket primitive as its sibling
// HostPort (which reaches helixendpoint.unbracket via netaddr.DialAddress),
// giving this package one primitive on this path rather than two — the
// "a future divergence has a single place to appear" rationale HostPort's own
// doc in ports.go states.
//
// This is on the LIVE dial path, not a synthetic one: cmd/helixagent/main.go
// passes RedisAddr() as the go-redis client's Addr (:842) and as the "redis"
// entry of the dependency health-probe map (:630) — worktree lines.
//
// It also removes a disagreement INSIDE this package: HostPort (ports.go)
// already routed through netaddr, so ports.HostPort and ports.RedisAddr
// answered differently for the identical bracketed input — the "two builders
// privately disagreeing about the same logical endpoint" defect netaddr's own
// doc names as this family's core.
func RedisAddr() string {
	return netaddr.DialAddress(RedisHost(), RedisPort())
}

// RedisPassword returns the configured Redis password, or "" when unset.
//
// Empty is a meaningful value, not merely a missing one: helix_agent's own
// docker-compose.yml starts Redis with --requirepass, so an empty password
// against that deployment yields "-NOAUTH Authentication required." on every
// command. Callers that probe availability MUST therefore complete a real
// AUTH+PING handshake rather than treating an open socket as success.
func RedisPassword() string {
	return os.Getenv(RedisPasswordEnv)
}
