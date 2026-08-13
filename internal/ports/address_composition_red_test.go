package ports

import (
	"net"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// HXC-286 — GROUP F1: internal/ports's generic host+port composition
// helpers (HostPort, HTTPURL, HTTPSURL).
//
// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on the pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// THE DEFECT (empirically verified against this host's Go toolchain,
// 2026-08-12 — §11.4.6). HostPort/HTTPURL/HTTPSURL each composed
// fmt.Sprintf("...%s:%d", host, Get(svc)) around a CALLER-SUPPLIED host —
// this package is the project's own doc comment "single source of truth
// for TCP port assignments," a GENERIC utility usable for ANY service
// (Get(svc) resolves the PORT half only; the host half is always the
// caller's). Because it is generic, it cannot know whether the caller means
// HelixAgent's own address or a foreign one — the fix therefore uses
// netaddr's no-default-substitution builders here too (never
// helixendpoint's default-substituting ones, which would be wrong for
// whichever caller means a foreign service).
//
//	fmt.Sprintf("%s:%d", "2001:db8::1", 8100) -> "2001:db8::1:8100"
//	net.SplitHostPort: address 2001:db8::1:8100: too many colons in address
const redModeGroupF1Host = "2001:db8::1"

func redModeGroupF1Enabled() bool {
	return os.Getenv("RED_MODE") == "1"
}

func TestHostPort_UnbracketedIPv6(t *testing.T) {
	got := HostPort(HelixAgentHTTP, redModeGroupF1Host)
	_, _, err := net.SplitHostPort(got)

	if redModeGroupF1Enabled() {
		require.Error(t, err, "RED_MODE=1: %q must be UNSPLITTABLE — that is the defect", got)
		return
	}
	require.NoError(t, err, "%q must be dialable (net.SplitHostPort-parseable)", got)
	h, _, _ := net.SplitHostPort(got)
	require.Equal(t, redModeGroupF1Host, h)
}

func TestHTTPURL_UnbracketedIPv6(t *testing.T) {
	got := HTTPURL(HelixAgentHTTP, redModeGroupF1Host)
	_, err := url.Parse(got)

	if redModeGroupF1Enabled() {
		require.Error(t, err, "RED_MODE=1: %q must be UNPARSEABLE — that is the defect", got)
		return
	}
	require.NoError(t, err, "%q must be parseable by any HTTP client", got)
	u, _ := url.Parse(got)
	require.Equal(t, redModeGroupF1Host, u.Hostname())
}

func TestHTTPSURL_UnbracketedIPv6(t *testing.T) {
	got := HTTPSURL(HelixAgentHTTP, redModeGroupF1Host)
	_, err := url.Parse(got)

	if redModeGroupF1Enabled() {
		require.Error(t, err, "RED_MODE=1: %q must be UNPARSEABLE — that is the defect", got)
		return
	}
	require.NoError(t, err, "%q must be parseable by any HTTP client", got)
	u, _ := url.Parse(got)
	require.Equal(t, redModeGroupF1Host, u.Hostname())
}

// STEP 3 (§11.4.146): enumerate the address forms callers can hand to
// HostPort/HTTPURL/HTTPSURL, with per-case outcomes. GREEN-only.
func TestPortsHelpers_AddressFormEnumeration(t *testing.T) {
	if redModeGroupF1Enabled() {
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
		{"hostname", "helixagent.internal.example", "helixagent.internal.example"},
		{"zone_qualified_ipv6", "fe80::1%eth0", "fe80::1%eth0"},
	}

	for _, tc := range cases {
		t.Run("HostPort/"+tc.name, func(t *testing.T) {
			got := HostPort(HelixAgentHTTP, tc.host)
			h, _, err := net.SplitHostPort(got)
			require.NoError(t, err, "case %q: %q must split", tc.name, got)
			require.Equal(t, tc.wantHost, h, "case %q", tc.name)
		})
		t.Run("HTTPURL/"+tc.name, func(t *testing.T) {
			got := HTTPURL(HelixAgentHTTP, tc.host)
			u, err := url.Parse(got)
			require.NoError(t, err, "case %q: %q must parse", tc.name, got)
			require.Equal(t, tc.wantHost, u.Hostname(), "case %q", tc.name)
		})
		t.Run("HTTPSURL/"+tc.name, func(t *testing.T) {
			got := HTTPSURL(HelixAgentHTTP, tc.host)
			u, err := url.Parse(got)
			require.NoError(t, err, "case %q: %q must parse", tc.name, got)
			require.Equal(t, tc.wantHost, u.Hostname(), "case %q", tc.name)
		})
	}
}
