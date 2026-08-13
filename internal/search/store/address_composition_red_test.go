package store

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// HXC-286 — GROUP B: third-party vector-database HTTP endpoints
// (QdrantStore.baseURL / ChromaStore.baseURL).
//
// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on the pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// THE DEFECT (empirically verified against this host's Go toolchain,
// 2026-08-12 — §11.4.6). Both baseURL() methods composed
// fmt.Sprintf("http://%s:%d", s.host, s.port). NewQdrantStore/NewChromaStore
// accept host as a caller-supplied CONSTRUCTOR PARAMETER — configuration-
// reachable exactly like the sites HXC-268 already fixed — defaulting only
// when the caller passes an EMPTY string, never validating or bracketing a
// non-empty value. An operator pointing HelixAgent at a real, routable
// Qdrant/Chroma deployment addressed by IPv6 (unbracketed — the natural way
// to write a bare IP, since brackets are URL-authority syntax) gets a
// baseURL no HTTP client can parse:
//
//	fmt.Sprintf("http://%s:%d", "2001:db8::1", 6333) -> "http://2001:db8::1:6333"
//	net/url.Parse: parse "http://2001:db8::1:6333": invalid port "db8::1:6333" after host
//
// Qdrant and Chroma are services HelixAgent does NOT own (§4.2 of
// docs/research/address_composition_family_20260812/ANALYSIS.md), so the
// fix uses netaddr.BaseURL — no default-substitution — never
// helixendpoint.BaseURL, whose fallback host is meant specifically for
// HelixAgent's own endpoint and would silently redirect a misconfigured
// Qdrant/Chroma address to a completely unrelated machine.
const redModeGroupBHost = "2001:db8::1"

func redModeGroupBEnabled() bool {
	return os.Getenv("RED_MODE") == "1"
}

func assertAddressParsesOrFails(t *testing.T, label, raw string) {
	t.Helper()
	_, err := url.Parse(raw)

	if redModeGroupBEnabled() {
		require.Error(t, err,
			"RED_MODE=1: %s baseURL %q must be UNPARSEABLE pre-fix — that is the defect", label, raw)
		return
	}

	require.NoError(t, err, "%s baseURL %q must be parseable by any HTTP client", label, raw)
	u, _ := url.Parse(raw)
	require.Equal(t, redModeGroupBHost, u.Hostname(),
		"%s baseURL %q must name the exact configured host", label, raw)
	require.NotEmpty(t, u.Port(), "%s baseURL %q must carry an explicit port", label, raw)
}

func TestQdrantStore_BaseURL_UnbracketedIPv6(t *testing.T) {
	s, err := NewQdrantStore(redModeGroupBHost, 6333, "test-collection")
	require.NoError(t, err)
	assertAddressParsesOrFails(t, "Qdrant", s.baseURL())
}

func TestChromaStore_BaseURL_UnbracketedIPv6(t *testing.T) {
	s, err := NewChromaStore(redModeGroupBHost, 8000, "test-collection")
	require.NoError(t, err)
	assertAddressParsesOrFails(t, "Chroma", s.baseURL())
}

// STEP 3 (§11.4.146): enumerate the address forms QdrantStore/ChromaStore
// callers can hand in — bracketed, unbracketed, hostname, IPv4, empty,
// zone-qualified — with per-case outcomes. GREEN-only (this is the
// exhaustive fan-out, not the RED/GREEN polarity check above).
func TestQdrantChromaStore_BaseURL_AddressFormEnumeration(t *testing.T) {
	if redModeGroupBEnabled() {
		t.Skip("STEP 3 fan-out runs in GREEN mode only (SKIP-OK: #red-mode-not-applicable)") // SKIP-OK: #red-mode-not-applicable
	}

	cases := []struct {
		name       string
		host       string
		port       int
		wantHost   string
		wantParses bool
	}{
		{"bracketed_ipv6", "[::1]", 6333, "::1", true},
		{"unbracketed_ipv6", "::1", 6333, "::1", true},
		{"unbracketed_ipv6_full_form", "2001:db8::1", 6333, "2001:db8::1", true},
		{"ipv4_literal", "127.0.0.1", 6333, "127.0.0.1", true},
		{"hostname", "qdrant.internal.example", 6333, "qdrant.internal.example", true},
		{"zone_qualified_ipv6", "fe80::1%eth0", 6333, "fe80::1%eth0", true},
		{"empty_host_defaults_to_localhost", "", 6333, "localhost", true},
	}

	for _, tc := range cases {
		t.Run("qdrant/"+tc.name, func(t *testing.T) {
			s, err := NewQdrantStore(tc.host, tc.port, "c")
			require.NoError(t, err)
			raw := s.baseURL()
			u, parseErr := url.Parse(raw)
			if !tc.wantParses {
				require.Error(t, parseErr, "case %q: %q must be unparseable", tc.name, raw)
				return
			}
			require.NoError(t, parseErr, "case %q: %q must parse", tc.name, raw)
			require.Equal(t, tc.wantHost, u.Hostname(), "case %q", tc.name)
		})
		t.Run("chroma/"+tc.name, func(t *testing.T) {
			s, err := NewChromaStore(tc.host, tc.port, "c")
			require.NoError(t, err)
			raw := s.baseURL()
			u, parseErr := url.Parse(raw)
			if !tc.wantParses {
				require.Error(t, parseErr, "case %q: %q must be unparseable", tc.name, raw)
				return
			}
			require.NoError(t, parseErr, "case %q: %q must parse", tc.name, raw)
			require.Equal(t, tc.wantHost, u.Hostname(), "case %q", tc.name)
		})
	}
}
