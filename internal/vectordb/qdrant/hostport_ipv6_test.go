package qdrant

import (
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file is the regression guard for the IPv6 host:port defect.
//
// It is fully deterministic and requires NO network: every input is a literal
// string, no listener is started, no port is bound, and nothing depends on
// whether the host can bind IPv4 or IPv6 loopback. That matters because the
// defect it guards only manifested when net/http/httptest.newLocalListener
// fell back to the IPv6 loopback under port pressure — i.e. exactly the
// condition an ordinary test run does not reproduce on demand.
//
// Defect recap:
//   - client_mock_test.go split the authority with strings.Split(_, ":"), so
//     the bracketed IPv6 authority "[::1]:39421" yielded host "[", and a
//     DISCARDED fmt.Sscanf error left the port silently defaulting to 80.
//   - config.go joined with fmt.Sprintf("%s:%d", ...), so a bare IPv6 host
//     "::1" produced the unparseable "http://::1:6333".
//
// The two are coupled: net.SplitHostPort returns the host UNBRACKETED, so
// fixing only the split still fed "::1" into the broken join.

// bracketed returns the bracketed spelling of an IPv6 literal, and returns
// hostnames and IPv4 literals unchanged (they are never bracketed).
func bracketed(host string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

// parseHostPortCase drives parseServerURL, the pure splitting half.
type parseHostPortCase struct {
	name     string
	rawURL   string
	wantHost string
	wantPort int
}

func TestParseServerURL_SplitsEveryAuthorityForm(t *testing.T) {
	tests := []parseHostPortCase{
		{
			name:     "ipv4 loopback",
			rawURL:   "http://127.0.0.1:39421",
			wantHost: "127.0.0.1",
			wantPort: 39421,
		},
		{
			name:     "hostname",
			rawURL:   "http://localhost:6333",
			wantHost: "localhost",
			wantPort: 6333,
		},
		{
			// THE defect input. httptest emits this form when it cannot bind
			// IPv4 loopback. Pre-fix this produced host "[" and port 80.
			name:     "ipv6 loopback bracketed by httptest",
			rawURL:   "http://[::1]:39421",
			wantHost: "::1",
			wantPort: 39421,
		},
		{
			name:     "ipv6 full address bracketed",
			rawURL:   "http://[2001:db8::1]:6333",
			wantHost: "2001:db8::1",
			wantPort: 6333,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := parseServerURL(tt.rawURL)

			require.NoError(t, err, "parseServerURL must not fail on %q", tt.rawURL)
			assert.Equal(t, tt.wantHost, host,
				"host parsed from %q (pre-fix strings.Split yielded %q for bracketed IPv6)",
				tt.rawURL, "[")
			assert.Equal(t, tt.wantPort, port,
				"port parsed from %q (pre-fix a discarded Sscanf error left this at 80)",
				tt.rawURL)
		})
	}
}

// TestParseServerURL_ReturnsErrorNeverSilentDefault proves the second half of
// the defect is closed: a malformed authority must surface an error rather
// than silently defaulting the port to 80.
func TestParseServerURL_ReturnsErrorNeverSilentDefault(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
	}{
		{name: "no port at all", rawURL: "http://127.0.0.1"},
		{name: "unterminated ipv6 bracket", rawURL: "http://[::1:39421"},
		{name: "non-numeric port", rawURL: "http://localhost:not-a-port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := parseServerURL(tt.rawURL)

			require.Error(t, err,
				"%q must surface an error, not silently default the port", tt.rawURL)
			assert.Empty(t, host, "no host may be returned alongside an error")
			assert.Zero(t, port,
				"port must be zero on error; the pre-fix code defaulted it to 80")
		})
	}
}

// TestGetHTTPURL_IsAlwaysAParseableURL is the joining half of the guard. Every
// spelling of a host must produce a URL that net/url can actually parse and
// whose Hostname()/Port() round-trip to the values that went in.
func TestGetHTTPURL_IsAlwaysAParseableURL(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		port         int
		wantURL      string
		wantHostname string
	}{
		{
			name:         "hostname is byte-identical to previous behaviour",
			host:         "localhost",
			port:         6333,
			wantURL:      "http://localhost:6333",
			wantHostname: "localhost",
		},
		{
			name:         "ipv4 is byte-identical to previous behaviour",
			host:         "192.168.1.100",
			port:         6333,
			wantURL:      "http://192.168.1.100:6333",
			wantHostname: "192.168.1.100",
		},
		{
			// RED pre-fix: fmt.Sprintf produced "http://::1:6333", which
			// url.Parse rejects with `invalid port "::1:6333" after host`,
			// so require.NoError below fails at runtime.
			name:         "bare ipv6 must be bracketed exactly once",
			host:         "::1",
			port:         6333,
			wantURL:      "http://[::1]:6333",
			wantHostname: "::1",
		},
		{
			// Anti-regression rather than pre-fix RED: the old fmt.Sprintf
			// happened to be correct here. A BARE net.JoinHostPort would
			// break it, emitting "http://[[::1]]:6333", because
			// JoinHostPort brackets any host containing ":". This case
			// pins the bracket-stripping step in hostPort.
			name:         "already-bracketed ipv6 must not be double-bracketed",
			host:         "[::1]",
			port:         6333,
			wantURL:      "http://[::1]:6333",
			wantHostname: "::1",
		},
		{
			name:         "full ipv6 address",
			host:         "2001:db8::1",
			port:         6333,
			wantURL:      "http://[2001:db8::1]:6333",
			wantHostname: "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{Host: tt.host, HTTPPort: tt.port}
			got := config.GetHTTPURL()

			assert.Equal(t, tt.wantURL, got)

			parsed, err := url.Parse(got)
			require.NoError(t, err,
				"GetHTTPURL returned %q, which net/url cannot parse", got)
			assert.Equal(t, tt.wantHostname, parsed.Hostname())
			assert.Equal(t, "6333", parsed.Port())
		})
	}
}

// TestGetGRPCAddress_IsAlwaysASplittableAuthority mirrors the HTTP guard for
// the gRPC authority, which had the identical fmt.Sprintf join.
func TestGetGRPCAddress_IsAlwaysASplittableAuthority(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		wantAddr string
		wantHost string
	}{
		{
			name:     "hostname is byte-identical to previous behaviour",
			host:     "localhost",
			wantAddr: "localhost:6334",
			wantHost: "localhost",
		},
		{
			name:     "bare ipv6 must be bracketed exactly once",
			host:     "::1",
			wantAddr: "[::1]:6334",
			wantHost: "::1",
		},
		{
			name:     "already-bracketed ipv6 must not be double-bracketed",
			host:     "[::1]",
			wantAddr: "[::1]:6334",
			wantHost: "::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{Host: tt.host, GRPCPort: 6334}
			got := config.GetGRPCAddress()

			assert.Equal(t, tt.wantAddr, got)

			// The emitted authority must be re-splittable, which is what a
			// gRPC dialer does with it.
			host, port, err := net.SplitHostPort(got)
			require.NoError(t, err,
				"GetGRPCAddress returned %q, which net.SplitHostPort rejects", got)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, "6334", port)
		})
	}
}

// TestHostPortRoundTrip proves join and split are exact inverses, so a host
// that survives one hop through Config cannot drift on the next.
func TestHostPortRoundTrip(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "::1", "2001:db8::1"} {
		t.Run(host, func(t *testing.T) {
			joined := hostPort(host, 6333)

			gotHost, gotPort, err := net.SplitHostPort(joined)
			require.NoError(t, err, "hostPort produced unsplittable authority %q", joined)
			assert.Equal(t, host, gotHost, "host drifted across a join/split round trip")
			assert.Equal(t, "6333", gotPort)

			// Re-joining the bracketed spelling must be idempotent: this is
			// what fails if the bracket-stripping step is ever removed.
			assert.Equal(t, joined, hostPort(bracketed(host), 6333),
				"hostPort is not idempotent on the bracketed spelling of %q", host)
		})
	}
}
