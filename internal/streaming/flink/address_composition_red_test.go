package flink

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// HXC-286 — GROUP D: Apache Flink REST API endpoint (Config.GetRESTURL).
//
// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on the pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// THE DEFECT (empirically verified against this host's Go toolchain,
// 2026-08-12 — §11.4.6). GetRESTURL composed
// fmt.Sprintf("http://%s:%d", c.JobManagerHost, c.WebUIPort) whenever the
// operator did not set RESTURL directly. JobManagerHost is an operator-
// supplied config field with no bracketing or validation — for an
// unbracketed IPv6 host the resulting URL is unparseable:
//
//	fmt.Sprintf("http://%s:%d", "2001:db8::1", 8081) -> "http://2001:db8::1:8081"
//	net/url.Parse: parse "http://2001:db8::1:8081": invalid port "db8::1:8081" after host
//
// Flink is a stream processor HelixAgent does not own (named verbatim in
// HXC-280's own description: "a stream processor"), so the fix uses
// netaddr.BaseURL — no default-substitution.
const redModeGroupDHost = "2001:db8::1"

func redModeGroupDEnabled() bool {
	return os.Getenv("RED_MODE") == "1"
}

func TestConfig_GetRESTURL_UnbracketedIPv6(t *testing.T) {
	c := &Config{JobManagerHost: redModeGroupDHost, WebUIPort: 8081}
	got := c.GetRESTURL()

	_, err := url.Parse(got)
	if redModeGroupDEnabled() {
		require.Error(t, err, "RED_MODE=1: %q must be UNPARSEABLE pre-fix — that is the defect", got)
		return
	}

	require.NoError(t, err, "%q must be parseable by any HTTP client", got)
	u, _ := url.Parse(got)
	require.Equal(t, redModeGroupDHost, u.Hostname())
	require.Equal(t, "8081", u.Port())
}

// STEP 3 (§11.4.146): enumerate the address forms Config.JobManagerHost can
// carry, with per-case outcomes. GREEN-only.
func TestConfig_GetRESTURL_AddressFormEnumeration(t *testing.T) {
	if redModeGroupDEnabled() {
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
		{"hostname", "flink-jobmanager.internal", "flink-jobmanager.internal"},
		{"zone_qualified_ipv6", "fe80::1%eth0", "fe80::1%eth0"},
		{"empty_host", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{JobManagerHost: tc.host, WebUIPort: 8081}
			raw := c.GetRESTURL()
			u, err := url.Parse(raw)
			require.NoError(t, err, "case %q: %q must parse", tc.name, raw)
			require.Equal(t, tc.wantHost, u.Hostname(), "case %q", tc.name)
			require.Equal(t, "8081", u.Port(), "case %q", tc.name)
		})
	}
}

// RESTURL override (explicit operator URL) must be returned verbatim,
// bypassing composition entirely — a pre-existing, unaffected path, asserted
// here so the fix is proven NOT to have touched it.
func TestConfig_GetRESTURL_ExplicitOverrideUntouched(t *testing.T) {
	c := &Config{RESTURL: "http://explicit.example:1234", JobManagerHost: "ignored", WebUIPort: 1}
	require.Equal(t, "http://explicit.example:1234", c.GetRESTURL())
}
