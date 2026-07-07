//go:build helixllm_e2e

// Package helixllm — LAN/VPN END-TO-END live proof via the HELIX_LLM_HOST /
// HELIX_LLM_PORT client-target composition.
//
// Guarded by the `helixllm_e2e` build tag (excluded from the unit suite). Run
// against the LIVE coder reachable over the LAN interface (NOT localhost):
//
//	HELIX_LLM_HOST=10.6.100.221 HELIX_LLM_PORT=18434 \
//	  go test -tags=helixllm_e2e -run TestE2E_HelixAgent_To_LiveHelixLLM_ViaHostPort -v \
//	  ./internal/llm/providers/helixllm/
//
// It drives the REAL provider.Complete — no stub, no mock (§11.4.5 / §11.4.50 /
// §11.4.69). If HELIX_LLM_HOST is set to a NON-localhost address (a real LAN
// IP), a PASS proves the provider works as a provider alias over the LAN.
package helixllm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_HelixAgent_To_LiveHelixLLM_ViaHostPort(t *testing.T) {
	host := os.Getenv(EnvHost)
	if host == "" {
		// SKIP-OK: no LAN host pinned — honest §11.4.3 skip, never a faked PASS.
		t.Skipf("SKIP-OK: %s not set — set it to the live coder's LAN IP (e.g. 10.6.100.221) to run the live LAN proof", EnvHost)
	}
	// Clear the higher-precedence seams so the HOST/PORT composition is the path
	// under test (proving the new parameterization, not an endpoint override).
	t.Setenv(EnvLocalOpenAIEndpoint, "")
	t.Setenv(EnvEndpoint, "")

	port := os.Getenv(EnvPort)
	if port == "" {
		port = defaultPort
	}
	wantEndpoint := "http://" + host + ":" + port

	// HELIX_LLM_API_KEY (optional) lets the same live proof target a
	// Bearer-key-protected endpoint over the LAN (e.g. an ephemeral --api-key
	// server); empty ⇒ no Authorization header (unkeyed coder).
	p := NewProvider(Config{APIKey: os.Getenv("HELIX_LLM_API_KEY"), Timeout: 90 * time.Second})
	require.Equal(t, wantEndpoint, p.Endpoint(),
		"provider must compose the LAN base endpoint from HELIX_LLM_HOST/HELIX_LLM_PORT")

	req := &models.LLMRequest{
		Messages: []models.Message{{Role: "user", Content: "Write a Go function named Add that takes two ints a and b and returns their sum. Output only the function."}},
		ModelParams: models.ModelParameters{
			Model:       "default",
			Temperature: 0.1,
			MaxTokens:   256,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := p.Complete(ctx, req)
	elapsed := time.Since(start)
	require.NoError(t, err, "live HelixAgent->HelixLLM Complete over the LAN must succeed against %s", wantEndpoint)
	require.NotNil(t, resp)

	content := resp.Content
	t.Logf("LAN endpoint=%s elapsed=%s tokens_used=%d model=%v", wantEndpoint, elapsed, resp.TokensUsed, resp.Metadata["model"])
	t.Logf("=== REAL MODEL RESPONSE OVER LAN (%d bytes) ===\n%s", len(content), content)

	require.NotEmpty(t, strings.TrimSpace(content), "model response must be non-empty")
	require.Greater(t, resp.TokensUsed, 0, "usage must report real tokens consumed")

	low := strings.ToLower(content)
	for _, m := range bluffMarkers {
		assert.NotContains(t, low, strings.ToLower(m),
			"response must not contain simulation marker %q — that would be a bluff", m)
	}
	assert.Contains(t, content, "func Add", "genuine coder output must contain the requested Go function 'func Add'")
}
