// SPDX-License-Identifier: Apache-2.0
// Regression tests for BUGFIX #16 — helixLLMTLSConfig must be secure by
// default and only skip verification when explicitly opted in.
package verifier

import (
	"crypto/tls"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelixLLMTLSConfig_SecureByDefault asserts that calling
// helixLLMTLSConfig() without any env overrides returns a config with
// InsecureSkipVerify=false and MinVersion at or above TLS 1.2. The
// previous code unconditionally set InsecureSkipVerify=true which
// violated CLAUDE.md's TLS posture rule.
func TestHelixLLMTLSConfig_SecureByDefault(t *testing.T) {
	// Ensure no opt-in env is set.
	t.Setenv("HELIX_LLM_TLS_SKIP_VERIFY", "")
	t.Setenv("SSL_CERT_FILE", "")
	t.Setenv("HELIX_LLM_CERT_PATH", "")

	cfg := helixLLMTLSConfig()
	require.NotNil(t, cfg)
	assert.False(t, cfg.InsecureSkipVerify, "default must verify certs")
	assert.GreaterOrEqual(t, cfg.MinVersion, uint16(tls.VersionTLS12), "must pin MinVersion to TLS 1.2 or above")
	assert.NotNil(t, cfg.RootCAs, "RootCAs pool must be populated when not skipping verify")
}

// TestHelixLLMTLSConfig_OptInSkip asserts that HELIX_LLM_TLS_SKIP_VERIFY=true
// is honoured — this is the single sanctioned escape hatch for local
// development against an un-trusted self-signed cert.
func TestHelixLLMTLSConfig_OptInSkip(t *testing.T) {
	t.Setenv("HELIX_LLM_TLS_SKIP_VERIFY", "true")

	cfg := helixLLMTLSConfig()
	require.NotNil(t, cfg)
	assert.True(t, cfg.InsecureSkipVerify, "opt-in env must disable verification")
	// When skipping verify, we do not bother constructing a RootCAs pool.
}

// TestHelixLLMTLSConfig_LoadsHelixLLMCert covers the HELIX_LLM_CERT_PATH
// path: when set to a valid PEM file, the bytes are loaded into RootCAs.
func TestHelixLLMTLSConfig_LoadsHelixLLMCert(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "helixllm-cert-*.pem")
	require.NoError(t, err)
	defer func() { _ = tmp.Close() }()

	// A minimal (self-signed) PEM block — structurally valid; fine for the
	// AppendCertsFromPEM call even though we never use it for a real
	// handshake in this test.
	const dummyPEM = `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIUDJjOF0eoAaLiRPGx8c1CqYz0xvswDQYJKoZIhvcNAQELBQAwDTEL
MAkGA1UEBhMCTlowHhcNMjYwNDE5MDAwMDAwWhcNMjcwNDE5MDAwMDAwWjANMQsw
CQYDVQQGEwJOWjBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQCcPR/Ot/8G74vA5lAg
IiCJJ/fT5U4pXC0VccQygQjtr+vmeoYJ+qdlBOwhyrSCmxcvS+1K50LwDOXG+l8+
VWrJAgMBAAEwDQYJKoZIhvcNAQELBQADQQAwBhuGjMqW2qHhL4ZixP7VZPJSLnZv
efrLdA1HSflK9xE2QUvKBKrBrCnOTRV4PAC0dN4J5PnGx+6U4FIKmNmV
-----END CERTIFICATE-----
`
	_, err = tmp.WriteString(dummyPEM)
	require.NoError(t, err)

	t.Setenv("HELIX_LLM_TLS_SKIP_VERIFY", "")
	t.Setenv("SSL_CERT_FILE", "")
	t.Setenv("HELIX_LLM_CERT_PATH", tmp.Name())

	cfg := helixLLMTLSConfig()
	require.NotNil(t, cfg)
	assert.False(t, cfg.InsecureSkipVerify)
	assert.NotNil(t, cfg.RootCAs)
	// Can't easily count certs in a x509.CertPool from outside but we can
	// assert it is non-nil and the function did not panic — coverage
	// confirms AppendCertsFromPEM was taken.
}

// TestGetEnvBoolVerifier_BoundaryValues documents accepted env values.
func TestGetEnvBoolVerifier_BoundaryValues(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"", false},
		{"0", false}, {"f", false}, {"false", false}, {"False", false}, {"NO", false},
		{"1", true}, {"t", true}, {"true", true}, {"True", true}, {"YES", true},
		{"garbage", false /* returns default */},
	}
	for _, tc := range tests {
		t.Run("env="+tc.env, func(t *testing.T) {
			t.Setenv("TEST_BOOL", tc.env)
			assert.Equal(t, tc.want, getEnvBoolVerifier("TEST_BOOL", false))
		})
	}
}
