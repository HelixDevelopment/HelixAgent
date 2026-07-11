package helixllm

// Unit tests for the LAN/VPN HELIX_LLM_HOST / HELIX_LLM_PORT client-target
// parameterization + the no-double-/v1 base-normalization invariant
// (§11.4.5 / §11.4.6 / §11.4.50). These run in the normal `go test ./...`
// suite — no build tag, no external infrastructure, fully deterministic.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearHelixLLMEndpointEnv unsets every env var that influences endpoint
// resolution so a composition/precedence case starts from a known-clean slate,
// restoring the unset state after the test (t.Cleanup). Shared with the
// integration tests in this package.
func clearHelixLLMEndpointEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{EnvEndpoint, EnvLocalOpenAIEndpoint, EnvHost, EnvPort} {
		k := k
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}
}

// TestNormalizeBase_NoDoubleV1 proves the anti-gotcha guard: any base — with or
// without a trailing /v1 or slash — becomes the bare host:port that is safe to
// concatenate with the hardcoded /v1/... paths.
func TestNormalizeBase_NoDoubleV1(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://10.6.100.221:18434", "http://10.6.100.221:18434"},
		{"http://10.6.100.221:18434/v1", "http://10.6.100.221:18434"},
		{"http://10.6.100.221:18434/v1/", "http://10.6.100.221:18434"},
		{"http://10.6.100.221:18434/", "http://10.6.100.221:18434"},
		{"https://localhost:8443", "https://localhost:8443"},
		{"  http://h:1/v1  ", "http://h:1"},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeBase(c.in)
		assert.Equalf(t, c.want, got, "normalizeBase(%q)", c.in)
		// The core invariant: base + chatEndpoint never yields /v1/v1.
		if got != "" {
			full := got + chatEndpoint
			assert.NotContainsf(t, full, "/v1/v1", "double /v1 leaked for input %q -> %q", c.in, full)
			assert.Truef(t, strings.HasSuffix(full, "/v1/chat/completions"),
				"composed URL must end in a single /v1/chat/completions, got %q", full)
		}
	}
}

// TestResolveEndpoint_HostPort_LANOverride: HELIX_LLM_HOST alone pins the LAN
// host; the port defaults to 18434. This is the operator's headline LAN case.
func TestResolveEndpoint_HostPort_LANOverride(t *testing.T) {
	clearHelixLLMEndpointEnv(t)
	t.Setenv(EnvHost, "10.6.100.221")
	assert.Equal(t, "http://10.6.100.221:18434", resolveEndpoint(""))
}

// TestResolveEndpoint_HostPort_LocalhostDefault: setting only the port engages
// the composition with the localhost host default.
func TestResolveEndpoint_HostPort_LocalhostDefault(t *testing.T) {
	clearHelixLLMEndpointEnv(t)
	t.Setenv(EnvPort, "18434")
	assert.Equal(t, "http://localhost:18434", resolveEndpoint(""))
}

// TestResolveEndpoint_HostPort_Both: both halves honoured verbatim.
func TestResolveEndpoint_HostPort_Both(t *testing.T) {
	clearHelixLLMEndpointEnv(t)
	t.Setenv(EnvHost, "10.6.100.221")
	t.Setenv(EnvPort, "19999")
	assert.Equal(t, "http://10.6.100.221:19999", resolveEndpoint(""))
}

// TestResolveEndpoint_HostPort_BindAllMappedToLocalhost: a server-bind 0.0.0.0
// (or ::) leaking into the client process maps to localhost — never an
// unreachable connect target (§11.4.6).
func TestResolveEndpoint_HostPort_BindAllMappedToLocalhost(t *testing.T) {
	for _, bindAll := range []string{"0.0.0.0", "::", "[::]"} {
		clearHelixLLMEndpointEnv(t)
		t.Setenv(EnvHost, bindAll)
		assert.Equalf(t, "http://localhost:18434", resolveEndpoint(""),
			"bind-all host %q must map to localhost", bindAll)
	}
}

// TestResolveEndpoint_ExplicitOverridesHostPort: an explicit cfg.Endpoint wins
// over the HOST/PORT composition.
func TestResolveEndpoint_ExplicitOverridesHostPort(t *testing.T) {
	clearHelixLLMEndpointEnv(t)
	t.Setenv(EnvHost, "10.6.100.221")
	assert.Equal(t, "https://custom:9443", resolveEndpoint("https://custom:9443"))
}

// TestResolveEndpoint_ExplicitEndpointSeamsWinOverHostPort: the two explicit
// endpoint env seams both outrank the HOST/PORT composition, so existing
// deployments that set HELIX_LLM_ENDPOINT / HELIX_LLM_LOCAL_OPENAI_ENDPOINT are
// unaffected by the new composition.
func TestResolveEndpoint_ExplicitEndpointSeamsWinOverHostPort(t *testing.T) {
	t.Run("local_openai_seam_wins", func(t *testing.T) {
		clearHelixLLMEndpointEnv(t)
		t.Setenv(EnvHost, "10.6.100.221")
		t.Setenv(EnvLocalOpenAIEndpoint, "http://seam:8080")
		assert.Equal(t, "http://seam:8080", resolveEndpoint(""))
	})
	t.Run("general_endpoint_wins", func(t *testing.T) {
		clearHelixLLMEndpointEnv(t)
		t.Setenv(EnvHost, "10.6.100.221")
		t.Setenv(EnvEndpoint, "https://gen:8443")
		assert.Equal(t, "https://gen:8443", resolveEndpoint(""))
	})
}

// TestResolveEndpoint_NoDoubleV1Invariant proves that no matter how the endpoint
// is supplied — a /v1-suffixed seam OR the HOST/PORT composition — the composed
// chat URL is exactly one /v1/chat/completions.
func TestResolveEndpoint_NoDoubleV1Invariant(t *testing.T) {
	t.Run("local_openai_seam_with_v1_suffix", func(t *testing.T) {
		clearHelixLLMEndpointEnv(t)
		t.Setenv(EnvLocalOpenAIEndpoint, "http://10.6.100.221:18434/v1")
		base := resolveEndpoint("")
		assert.Equal(t, "http://10.6.100.221:18434", base)
		full := base + chatEndpoint
		assert.Equal(t, "http://10.6.100.221:18434/v1/chat/completions", full)
		assert.NotContains(t, full, "/v1/v1")
	})
	t.Run("host_port_composition", func(t *testing.T) {
		clearHelixLLMEndpointEnv(t)
		t.Setenv(EnvHost, "10.6.100.221")
		full := resolveEndpoint("") + chatEndpoint
		assert.Equal(t, "http://10.6.100.221:18434/v1/chat/completions", full)
		assert.NotContains(t, full, "/v1/v1")
	})
}

// TestResolveEndpoint_NothingSetIsLegacyDefault: with every seam cleared the
// legacy TLS :8443 default is preserved (no behaviour change for existing
// gateway deployments).
func TestResolveEndpoint_NothingSetIsLegacyDefault(t *testing.T) {
	clearHelixLLMEndpointEnv(t)
	assert.Equal(t, DefaultEndpoint, resolveEndpoint(""))
}

// TestNewProvider_HostPortComposition: the composition flows through the public
// constructor so a caller passing an empty Config picks up the LAN target.
func TestNewProvider_HostPortComposition(t *testing.T) {
	clearHelixLLMEndpointEnv(t)
	t.Setenv(EnvHost, "10.6.100.221")
	t.Setenv(EnvPort, "18434")
	p := NewProvider(Config{})
	require.NotNil(t, p)
	assert.Equal(t, "http://10.6.100.221:18434", p.Endpoint())
}

// TestNewProvider_APIKeyWired: the API key from Config is retained so the
// Authorization: Bearer header is emitted (proven end-to-end in the auth-matrix
// integration test).
func TestNewProvider_APIKeyWired(t *testing.T) {
	p := NewProvider(Config{Endpoint: "http://localhost:18434", APIKey: "lan-key-123"})
	assert.Equal(t, "lan-key-123", p.apiKey)
}
