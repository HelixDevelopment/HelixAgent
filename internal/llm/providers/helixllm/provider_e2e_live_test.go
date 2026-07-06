//go:build helixllm_e2e

// Package helixllm — Phase-2 END-TO-END live proof.
//
// This file is guarded by the `helixllm_e2e` build tag so it is EXCLUDED from
// the normal unit suite (`go test ./...`) and only runs when explicitly
// requested against a LIVE HelixLLM OpenAI-compatible endpoint:
//
//	HELIX_LLM_LOCAL_OPENAI_ENDPOINT=http://localhost:18434/v1 \
//	  go test -tags=helixllm_e2e -run TestE2E_HelixAgent_To_LiveHelixLLM -v \
//	  ./internal/llm/providers/helixllm/
//
// It drives the REAL HelixAgent HelixLLM provider (Provider.Complete) against
// the live local coder — no stub, no mock (§11.4.50 / §11.4.5 / §11.4.69).
//
// §11.4.115 RED-baseline polarity switch (RED_MODE env):
//   - RED_MODE=1  → reproduce the PRE-PIN broken artifact: construct the
//     provider with NO endpoint pin so resolveEndpoint() falls back to the
//     TLS :8443 gateway default (which is NOT running). Complete MUST FAIL
//     (connection refused / TLS) — this proves the OQ1 defect is genuinely
//     present without the endpoint pin. If it does NOT fail, that is a finding.
//   - RED_MODE=0 (default) → the GREEN proof: construct with the pinned
//     HELIX_LLM_LOCAL_OPENAI_ENDPOINT seam; Complete MUST return real,
//     non-empty genuine model output.
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

// bluffMarkers are substrings that would indicate a simulated / placeholder
// response rather than genuine model output (anti-bluff §11.4 / §11.4.2).
var bluffMarkers = []string{
	"simulated response",
	"this is a simulated",
	"in production this would",
	"placeholder response",
	"TODO implement",
}

func TestE2E_HelixAgent_To_LiveHelixLLM(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	// A real coding prompt — the response must be genuine, non-trivial code.
	prompt := "Write a Go function named Add that takes two ints a and b and returns their sum. Output only the function."
	req := &models.LLMRequest{
		Messages: []models.Message{{Role: "user", Content: prompt}},
		ModelParams: models.ModelParameters{
			// llama.cpp-server ignores the model string; "default" mirrors the
			// operator-verified curl. Kept explicit for reproducibility.
			Model:       "default",
			Temperature: 0.1,
			MaxTokens:   256,
		},
	}

	if redMode {
		// PRE-PIN broken artifact: clear BOTH endpoint seams so
		// resolveEndpoint() falls back to the TLS :8443 gateway default.
		t.Setenv(EnvLocalOpenAIEndpoint, "")
		t.Setenv(EnvEndpoint, "")
		p := NewProvider(Config{Timeout: 8 * time.Second})
		require.Equal(t, DefaultEndpoint, p.Endpoint(),
			"RED baseline must resolve to the (non-running) TLS default")

		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		resp, err := p.Complete(ctx, req)
		require.Error(t, err,
			"RED baseline: HelixAgent->HelixLLM MUST FAIL without the endpoint pin (OQ1 defect reproduced)")
		require.Nil(t, resp)
		t.Logf("RED PASS — defect reproduced: Complete() failed as expected: %v", err)
		return
	}

	// GREEN proof: require the pinned live endpoint seam.
	endpoint := os.Getenv(EnvLocalOpenAIEndpoint)
	if endpoint == "" {
		// SKIP-OK: live endpoint not provided — honest §11.4.3 skip, never a
		// faked PASS. Set HELIX_LLM_LOCAL_OPENAI_ENDPOINT to the live router.
		t.Skipf("SKIP-OK: %s not set — set it to the live HelixLLM router (e.g. http://localhost:18434/v1) to run the live E2E proof",
			EnvLocalOpenAIEndpoint)
	}

	p := NewProvider(Config{Timeout: 90 * time.Second})
	require.Equal(t, endpoint, p.Endpoint(),
		"provider must resolve to the pinned live endpoint via the highest-precedence local seam")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := p.Complete(ctx, req)
	elapsed := time.Since(start)
	require.NoError(t, err, "live HelixAgent->HelixLLM Complete must succeed against %s", endpoint)
	require.NotNil(t, resp)

	content := resp.Content
	t.Logf("endpoint=%s elapsed=%s tokens_used=%d model=%v",
		endpoint, elapsed, resp.TokensUsed, resp.Metadata["model"])
	t.Logf("=== REAL MODEL RESPONSE (%d bytes) ===\n%s", len(content), content)

	// Genuine, non-empty output.
	require.NotEmpty(t, strings.TrimSpace(content), "model response must be non-empty")
	require.Greater(t, resp.TokensUsed, 0, "usage must report real tokens consumed")

	// Anti-bluff: no simulation markers.
	low := strings.ToLower(content)
	for _, m := range bluffMarkers {
		assert.NotContains(t, low, strings.ToLower(m),
			"response must not contain simulation marker %q — that would be a bluff", m)
	}

	// It must actually be the code we asked for (genuine coder output).
	assert.Contains(t, content, "func Add",
		"genuine coder output must contain the requested Go function 'func Add'")
	assert.True(t, strings.Contains(content, "+") || strings.Contains(content, "a, b"),
		"function body should reference the summed parameters")
}
