package cache

import (
	"net"
	"os"
	"testing"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/require"
)

// HXC-286 — GROUP C: Redis (third-party cache).
//
// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on the pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// THE DEFECT (empirically verified against this host's Go toolchain,
// 2026-08-12 — §11.4.6). NewRedisClient composed
// `cfg.Redis.Host + ":" + cfg.Redis.Port` — raw string concatenation, the
// same shape as fmt.Sprintf("%s:%d", ...). RedisConfig.Host/Port are BOTH
// operator-supplied config fields (env-injected), and for an unbracketed
// IPv6 host the resulting Addr is unusable by go-redis's own dialer, which
// (like net.Dial generally) requires net.SplitHostPort-parseable input:
//
//	"2001:db8::1" + ":" + "6379" -> "2001:db8::1:6379"
//	net.SplitHostPort: address 2001:db8::1:6379: too many colons in address
//
// Redis is a service HelixAgent does not own (named explicitly in HXC-280's
// own description as one of "two database systems"), so the fix uses
// netaddr.DialAddressString — no default-substitution, string-typed port to
// match RedisConfig.Port's own type.
const redModeGroupCHost = "2001:db8::1"

func redModeGroupCEnabled() bool {
	return os.Getenv("RED_MODE") == "1"
}

// redisClientDialAddr extracts the resolved dial address WITHOUT letting the
// underlying go-redis connection pool leak background dialConn/idle-conn
// goroutines that keep retrying against a (possibly malformed, in the RED
// case) address forever — this package's TestMain runs goleak.VerifyTestMain,
// so the client is closed immediately after the address is read, mirroring
// NewRedisClient's own nil-cfg branch, which does exactly this for the same
// reason (see its doc comment).
func redisClientDialAddr(t *testing.T, host, port string) string {
	t.Helper()
	c := NewRedisClient(&config.Config{
		Redis: config.RedisConfig{
			Host: host,
			Port: port,
		},
	})
	require.NotNil(t, c)
	require.NotNil(t, c.client, "NewRedisClient must populate the underlying go-redis client")
	addr := c.client.Options().Addr
	require.NoError(t, c.client.Close())
	return addr
}

func TestNewRedisClient_UnbracketedIPv6(t *testing.T) {
	addr := redisClientDialAddr(t, redModeGroupCHost, "6379")

	_, _, err := net.SplitHostPort(addr)

	if redModeGroupCEnabled() {
		require.Error(t, err,
			"RED_MODE=1: the pre-fix Redis Addr %q must be UNUSABLE by net.SplitHostPort — that is the defect", addr)
		return
	}

	require.NoError(t, err, "the Redis Addr %q must be dialable (net.SplitHostPort-parseable)", addr)
	h, p, _ := net.SplitHostPort(addr)
	require.Equal(t, redModeGroupCHost, h, "the parsed host must be the exact configured host")
	require.Equal(t, "6379", p)
}

// STEP 3 (§11.4.146): enumerate the address forms RedisConfig.Host/Port can
// carry, with per-case outcomes.
//
// The enumeration lives ONCE, at the primitive
// (internal/netaddr/netaddr_test.go's
// TestDialAddressString_AddressFormEnumeration — bracketed/unbracketed/
// zone-qualified/hostname IPv6, IPv4, hostname), not repeated here via N
// real *RedisClient instantiations: NewRedisClient's underlying go-redis
// pool starts MinIdleConns=2 background maintenance goroutines that
// ACTIVELY DIAL the composed address, and for the non-loopback hosts a form
// enumeration needs (a documentation-only IPv6 prefix, an unresolvable
// hostname) those dial attempts were measured (2026-08-12, this host) to
// outlive Close() long enough for this package's
// goleak.VerifyTestMain (main_test.go) to report them as leaked — an
// artefact of exercising the connection pool, not of the address-
// composition fix. redis.go's ONE call site is a direct, untransformed
// pass-through of (cfg.Redis.Host, cfg.Redis.Port) into
// netaddr.DialAddressString, so TestNewRedisClient_UnbracketedIPv6 above
// (the one case exercised through a REAL client, confirmed leak-free) is
// the wiring proof; the per-form fan-out is the primitive's job.
