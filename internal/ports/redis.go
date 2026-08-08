package ports

import (
	"net"
	"os"
	"strconv"
	"strings"
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
func RedisAddr() string {
	return net.JoinHostPort(RedisHost(), RedisPortString())
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
