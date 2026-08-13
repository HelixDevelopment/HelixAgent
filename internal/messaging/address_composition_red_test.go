package messaging

import (
	"net"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// HXC-286 — GROUP E: AMQP broker connection string (BrokerConfig.ConnectionString).
//
// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on the pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// THE DEFECT AND THE READER-CHOICE FINDING (empirically verified against
// this host's Go toolchain and github.com/rabbitmq/amqp091-go v1.10.0,
// 2026-08-12 — §11.4.6). ConnectionString composed the authority by hand:
// `... + c.Host + ":" + strconv.Itoa(c.Port) + c.VirtualHost`.
//
// The obvious reader — net/url.Parse — turns out NOT to be the right proxy
// here: Go's net/url only runs its strict "invalid port after host" check
// for the "http"/"https" schemes. For "amqp" (and "redis", "postgres",
// "mongodb", "ws", and any unrecognised scheme, all measured directly) it
// is LENIENT — Parse succeeds, storing the whole malformed authority
// verbatim in u.Host, and its Hostname()/Port() accessors then apply their
// OWN last-colon split on THAT string, which happens to recover the right
// answer for a plain unbracketed IPv6 literal by coincidence of where the
// last colon falls. Verified further one level down: amqp091-go's own
// ParseURI (the library internal/messaging/rabbitmq/connection.go dials
// through) calls exactly url.Parse + Hostname()/Port(), so it ALSO
// tolerates a plain unbracketed IPv6 host — but FAILS for a zone-qualified
// one ("fe80::1%eth0" -> "invalid URL escape \"%et\"", the exact raw-'%'
// class already fixed in netaddr.BaseURL for Group B/D).
//
// So the RED oracle here is net.SplitHostPort applied to url.Parse's own
// u.Host (the raw authority, always populated regardless of scheme
// leniency) — scheme-agnostic, and the same question ANY consumer needing
// host and port SEPARATELY must answer (a health-check dialer, a metrics
// tag, a connection-pool key), not just amqp091-go's own tolerant parse:
//
//	url.Parse("amqp://...@2001:db8::1:5672/").Host -> "2001:db8::1:5672"
//	net.SplitHostPort("2001:db8::1:5672"): too many colons in address
//
// RabbitMQ is a message broker HelixAgent does not own (named verbatim in
// HXC-280's own description: "a message broker"). This fix uses
// netaddr.DialAddressForURL for the authority's host:port component — no
// default-substitution — and deliberately does NOT touch userinfo
// escaping (c.Username/c.Password are not percent-encoded here either):
// that is a distinct defect class (URL userinfo escaping, not host:port
// composition), already tracked once upstream (HXC-280,
// submodules/llms_verifier/internal/messaging/rabbitmq/config.go, out of
// scope for this HXC-286 batch) and reported rather than silently fixed
// here (see the task report).
const redModeGroupEHost = "2001:db8::1"

func redModeGroupEEnabled() bool {
	return os.Getenv("RED_MODE") == "1"
}

// hostPortSplittable is the scheme-agnostic reader oracle described above:
// can ANY consumer needing (host, port) separately recover them from the
// composed connection string's authority component.
func hostPortSplittable(t *testing.T, raw string) error {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err, "url.Parse itself must not fail for %q (that would be a different, more severe defect)", raw)
	_, _, splitErr := net.SplitHostPort(u.Host)
	return splitErr
}

func TestBrokerConfig_ConnectionString_UnbracketedIPv6_NoAuth(t *testing.T) {
	c := &BrokerConfig{Host: redModeGroupEHost, Port: 5672, VirtualHost: "/"}
	got := c.ConnectionString()
	splitErr := hostPortSplittable(t, got)

	if redModeGroupEEnabled() {
		require.Error(t, splitErr, "RED_MODE=1: %q's authority must NOT be splittable into (host, port) — that is the defect", got)
		return
	}
	require.NoError(t, splitErr, "%q's authority must be splittable into (host, port) by any consumer that needs them separately", got)

	u, _ := url.Parse(got)
	h, p, _ := net.SplitHostPort(u.Host)
	require.Equal(t, redModeGroupEHost, h)
	require.Equal(t, "5672", p)
}

func TestBrokerConfig_ConnectionString_UnbracketedIPv6_WithAuth(t *testing.T) {
	c := &BrokerConfig{
		Host: redModeGroupEHost, Port: 5672, VirtualHost: "/",
		Username: "app", Password: "s3cr3t",
	}
	got := c.ConnectionString()
	splitErr := hostPortSplittable(t, got)

	if redModeGroupEEnabled() {
		require.Error(t, splitErr, "RED_MODE=1: %q's authority must NOT be splittable into (host, port) — that is the defect", got)
		return
	}
	require.NoError(t, splitErr, "%q's authority must be splittable into (host, port)", got)

	u, _ := url.Parse(got)
	require.Equal(t, "app", u.User.Username())
}

// STEP 3 (§11.4.146): enumerate the address forms BrokerConfig.Host can
// carry, with per-case outcomes, both with and without credentials present
// in the URI. GREEN-only.
func TestBrokerConfig_ConnectionString_AddressFormEnumeration(t *testing.T) {
	if redModeGroupEEnabled() {
		t.Skip("STEP 3 fan-out runs in GREEN mode only (SKIP-OK: #red-mode-not-applicable)") // SKIP-OK: #red-mode-not-applicable
	}

	cases := []struct {
		name     string
		host     string
		wantHost string
	}{
		{"bracketed_ipv6", "[::1]", "::1"},
		{"unbracketed_ipv6", "::1", "::1"},
		{"unbracketed_ipv6_full_form", "2001:db8::1", "2001:db8::1"},
		{"ipv4_literal", "127.0.0.1", "127.0.0.1"},
		{"hostname", "rabbitmq.internal.example", "rabbitmq.internal.example"},
		{"zone_qualified_ipv6", "fe80::1%eth0", "fe80::1%eth0"},
	}

	for _, tc := range cases {
		for _, withAuth := range []bool{false, true} {
			name := tc.name
			if withAuth {
				name += "/with_auth"
			} else {
				name += "/no_auth"
			}
			t.Run(name, func(t *testing.T) {
				c := &BrokerConfig{Host: tc.host, Port: 5672, VirtualHost: "/"}
				if withAuth {
					c.Username = "app"
					c.Password = "s3cr3t"
				}
				raw := c.ConnectionString()
				require.NoError(t, hostPortSplittable(t, raw), "case %q: %q must be splittable", name, raw)

				u, _ := url.Parse(raw)
				h, _, _ := net.SplitHostPort(u.Host)
				require.Equal(t, tc.wantHost, h, "case %q", name)
				if withAuth {
					require.Equal(t, "app", u.User.Username(), "case %q", name)
				}
			})
		}
	}
}

// TestBrokerConfig_ConnectionString_TLSScheme confirms the amqps scheme path
// (c.TLS = true) is unaffected by the fix — a pre-existing, unrelated
// branch.
func TestBrokerConfig_ConnectionString_TLSScheme(t *testing.T) {
	c := &BrokerConfig{Host: "rabbitmq.internal", Port: 5671, VirtualHost: "/", TLS: true}
	got := c.ConnectionString()
	require.Contains(t, got, "amqps://")
	require.NoError(t, hostPortSplittable(t, got))
}
