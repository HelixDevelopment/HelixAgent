package helixllm

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// HXC-286 — GROUP F2: HelixLLM client-target composition
// (resolveEndpoint's HELIX_LLM_HOST/HELIX_LLM_PORT branch).
//
// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1 — assert the DEFECT IS PRESENT (reproduce on the pre-fix tree).
//	RED_MODE=0 — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// THE DEFECT AND THE OWNERSHIP FINDING (§11.4.6). resolveEndpoint composed
// normalizeBase("http://" + host + ":" + port) when HELIX_LLM_HOST/PORT are
// set. For an unbracketed IPv6 host the result is unparseable:
//
//	"http://" + "2001:db8::1" + ":" + "18434" -> "http://2001:db8::1:18434"
//	net/url.Parse: parse "http://2001:db8::1:18434": invalid port "db8::1:18434" after host
//
// OWNERSHIP: HelixLLM is a DIFFERENT service/submodule from HelixAgent
// (this package's own doc comment: "integrates the HelixLLM submodule as a
// first-class LLM provider"). helixendpoint.BaseURL's default-substitution
// falls back to a placeholder documented specifically for HelixAgent's OWN
// endpoint — using it here on a malformed HELIX_LLM_HOST would silently
// redirect to a machine meant for a DIFFERENT service than the one
// misconfigured, which is worse than failing loudly. This fix therefore
// uses netaddr.BaseURLString (no default-substitution), exactly as for
// every other non-HelixAgent-owned site in this family.
const redModeGroupF2Host = "2001:db8::1"

func redModeGroupF2Enabled() bool {
	return os.Getenv("RED_MODE") == "1"
}

func withHelixLLMHostPort(t *testing.T, host, port string) {
	t.Helper()
	prevHost, hadHost := os.LookupEnv(EnvHost)
	prevPort, hadPort := os.LookupEnv(EnvPort)
	prevEndpoint, hadEndpoint := os.LookupEnv(EnvEndpoint)
	prevLocal, hadLocal := os.LookupEnv(EnvLocalOpenAIEndpoint)

	require.NoError(t, os.Setenv(EnvHost, host))
	require.NoError(t, os.Setenv(EnvPort, port))
	// The host/port composition is only engaged when neither higher-precedence
	// endpoint env var is set (see resolveEndpoint's precedence doc comment).
	require.NoError(t, os.Unsetenv(EnvEndpoint))
	require.NoError(t, os.Unsetenv(EnvLocalOpenAIEndpoint))

	t.Cleanup(func() {
		restore := func(key, prev string, had bool) {
			if had {
				_ = os.Setenv(key, prev)
			} else {
				_ = os.Unsetenv(key)
			}
		}
		restore(EnvHost, prevHost, hadHost)
		restore(EnvPort, prevPort, hadPort)
		restore(EnvEndpoint, prevEndpoint, hadEndpoint)
		restore(EnvLocalOpenAIEndpoint, prevLocal, hadLocal)
	})
}

func TestResolveEndpoint_UnbracketedIPv6(t *testing.T) {
	withHelixLLMHostPort(t, redModeGroupF2Host, "18434")

	got := resolveEndpoint("")
	_, err := url.Parse(got)

	if redModeGroupF2Enabled() {
		require.Error(t, err, "RED_MODE=1: %q must be UNPARSEABLE pre-fix — that is the defect", got)
		return
	}
	require.NoError(t, err, "%q must be parseable by any HTTP client", got)
	u, _ := url.Parse(got)
	require.Equal(t, redModeGroupF2Host, u.Hostname())
	require.Equal(t, "18434", u.Port())
}

// STEP 3 (§11.4.146): enumerate the address forms HELIX_LLM_HOST can carry,
// with per-case outcomes. GREEN-only.
func TestResolveEndpoint_AddressFormEnumeration(t *testing.T) {
	if redModeGroupF2Enabled() {
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
		{"hostname", "helixllm.internal.example", "helixllm.internal.example"},
		{"zone_qualified_ipv6", "fe80::1%eth0", "fe80::1%eth0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withHelixLLMHostPort(t, tc.host, "18434")
			raw := resolveEndpoint("")
			u, err := url.Parse(raw)
			require.NoError(t, err, "case %q: %q must parse", tc.name, raw)
			require.Equal(t, tc.wantHost, u.Hostname(), "case %q", tc.name)
			require.Equal(t, "18434", u.Port(), "case %q", tc.name)
		})
	}
}

// TestResolveEndpoint_BindWildcardStillMapsToLocalhost confirms the
// pre-existing 0.0.0.0/::/[::] -> localhost normalisation (unrelated to
// this fix) survives untouched.
func TestResolveEndpoint_BindWildcardStillMapsToLocalhost(t *testing.T) {
	withHelixLLMHostPort(t, "0.0.0.0", "18434")
	got := resolveEndpoint("")
	require.Equal(t, "http://localhost:18434", got)
}
