package rabbitmq

import (
	"net"
	"net/url"
	"os"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

// HXC-286 — GROUP E (continued): the REAL, wired RabbitMQ dial path,
// Connection.buildURL — found independently (NOT in the task's given
// survey, which named only internal/messaging/broker.go). That sibling
// function (internal/messaging.BrokerConfig.ConnectionString) has ZERO
// callers anywhere in this tree (confirmed by grep across the module,
// 2026-08-12) — THIS function, one level down in the rabbitmq package, is
// what Connection.dial() actually feeds to amqp.DialConfig for every real
// connection this codebase makes.
//
// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on the pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// THE DEFECT AND THE REAL-READER FINDING (empirically verified against
// github.com/rabbitmq/amqp091-go v1.10.0, 2026-08-12 — §11.4.6). buildURL
// composed fmt.Sprintf("%s://%s:%s@%s:%d/%s", scheme, user, pass, host,
// port, vhost). Measured DIRECTLY against amqp.ParseURI — the function
// amqp.DialConfig calls internally — a plain unbracketed IPv6 host is
// tolerated (Go's net/url is lenient for the "amqp" scheme and
// Hostname()/Port() recover it via a last-colon split that happens to land
// correctly), but a ZONE-QUALIFIED one is not:
//
//	amqp.ParseURI("amqp://u:p@fe80::1%eth0:5672/")
//	  -> parse "amqp://u:p@fe80::1%eth0:5672/": invalid URL escape "%et"
//
// This RED test therefore uses the zone-qualified case as its killer input
// — the one that concretely breaks the REAL dial path — while the STEP-3
// enumeration below also asserts the scheme-agnostic host:port-splittable
// property (net.SplitHostPort on the parsed authority) for every other
// form, matching Group E's sibling test in the parent messaging package.
const redModeGroupEConnHost = "fe80::1%eth0"

func redModeGroupEConnEnabled() bool {
	return os.Getenv("RED_MODE") == "1"
}

func TestConnection_BuildURL_ZoneQualifiedIPv6(t *testing.T) {
	c := NewConnection(&Config{
		Host: redModeGroupEConnHost, Port: 5672, VHost: "/",
		Username: "app", Password: "s3cr3t",
	}, nil)
	got := c.buildURL()

	_, parseErr := amqp.ParseURI(got)
	if redModeGroupEConnEnabled() {
		require.Error(t, parseErr,
			"RED_MODE=1: amqp.ParseURI must REJECT %q — that is the defect on the real dial path", got)
		return
	}

	require.NoError(t, parseErr, "amqp.ParseURI must accept %q — this is what amqp.DialConfig parses on every real connection", got)
	uri, _ := amqp.ParseURI(got)
	require.Equal(t, redModeGroupEConnHost, uri.Host)
	require.Equal(t, 5672, uri.Port)
	require.Equal(t, "app", uri.Username)
}

// STEP 3 (§11.4.146): enumerate the address forms Config.Host can carry,
// with per-case outcomes against BOTH the real amqp091-go parser and the
// scheme-agnostic net.SplitHostPort oracle. GREEN-only.
func TestConnection_BuildURL_AddressFormEnumeration(t *testing.T) {
	if redModeGroupEConnEnabled() {
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
		t.Run(tc.name, func(t *testing.T) {
			c := NewConnection(&Config{Host: tc.host, Port: 5672, VHost: "/", Username: "app", Password: "s3cr3t"}, nil)
			raw := c.buildURL()

			uri, err := amqp.ParseURI(raw)
			require.NoError(t, err, "case %q: amqp.ParseURI must accept %q", tc.name, raw)
			require.Equal(t, tc.wantHost, uri.Host, "case %q", tc.name)
			require.Equal(t, 5672, uri.Port, "case %q", tc.name)

			u, uerr := url.Parse(raw)
			require.NoError(t, uerr, "case %q: url.Parse must accept %q", tc.name, raw)
			h, p, splitErr := net.SplitHostPort(u.Host)
			require.NoError(t, splitErr, "case %q: the authority %q must ALSO be splittable via the scheme-agnostic net.SplitHostPort oracle, for any consumer that isn't amqp091-go", tc.name, u.Host)
			require.Equal(t, tc.wantHost, h, "case %q", tc.name)
			require.Equal(t, "5672", p, "case %q", tc.name)
		})
	}
}
