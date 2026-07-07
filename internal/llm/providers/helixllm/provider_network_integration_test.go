package helixllm

// Integration tests for the LAN/VPN provider: they stand up a REAL
// OpenAI-compatible HTTP server (httptest, real sockets) that mimics the
// llama.cpp-server Bearer-key contract (byte-for-byte compare → 401), and drive
// the REAL HelixAgent provider.Complete against it — no provider mock, no stub
// (§11.4.5 / §11.4.50 / §11.4.98, CONST-050). Deterministic + re-runnable at
// -count=N with no external infrastructure.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"dev.helix.agent/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockOpenAIServer returns an httptest.Server exposing exactly
// /v1/chat/completions guarded by a Bearer API key (empty wantKey ⇒ open). It
// records the request path + Authorization header it observed so tests can prove
// (a) the 401/200 auth matrix and (b) no double-/v1 (a /v1/v1 request 404s
// because only the single-/v1 route is registered).
func newMockOpenAIServer(t *testing.T, wantKey string, gotPath, gotAuth *string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(chatEndpoint, func(w http.ResponseWriter, r *http.Request) {
		if gotPath != nil {
			*gotPath = r.URL.Path
		}
		if gotAuth != nil {
			*gotAuth = r.Header.Get("Authorization")
		}
		if wantKey != "" && r.Header.Get("Authorization") != "Bearer "+wantKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid API Key","type":"authentication_error"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ChatCompletionResponse{
			ID:     "chatcmpl-mock",
			Object: "chat.completion",
			Model:  "mock-coder",
			Choices: []Choice{{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "func Add(a, b int) int { return a + b }"},
				FinishReason: "stop",
			}},
			Usage: Usage{PromptTokens: 12, CompletionTokens: 14, TotalTokens: 26},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func codeRequest() *models.LLMRequest {
	return &models.LLMRequest{
		Messages:    []models.Message{{Role: "user", Content: "Write a Go function Add(a,b int) int."}},
		ModelParams: models.ModelParameters{Model: "default", Temperature: 0.1, MaxTokens: 64},
	}
}

// TestIntegration_HelixLLM_AuthMatrix proves the 401/200 matrix end-to-end
// through the real provider: no key → 401, wrong key → 401, correct key → 200
// with a real completion body + the exact single-/v1 request path.
func TestIntegration_HelixLLM_AuthMatrix(t *testing.T) {
	const key = "s3cr3t-lan-key"
	var gotPath, gotAuth string
	srv := newMockOpenAIServer(t, key, &gotPath, &gotAuth)
	base := srv.URL // http://127.0.0.1:PORT — bare base, no /v1
	ctx := context.Background()

	// (1) NO key -> 401
	pNoKey := NewProvider(Config{Endpoint: base, Timeout: 10 * time.Second})
	_, err := pNoKey.Complete(ctx, codeRequest())
	require.Error(t, err, "keyed server must reject a request that carries no API key")
	assert.Contains(t, err.Error(), "401", "no-key request must surface HTTP 401; got: %v", err)

	// (2) WRONG key -> 401
	pWrong := NewProvider(Config{Endpoint: base, APIKey: "wrong-key", Timeout: 10 * time.Second})
	_, err = pWrong.Complete(ctx, codeRequest())
	require.Error(t, err, "keyed server must reject a request with the wrong API key")
	assert.Contains(t, err.Error(), "401", "wrong-key request must surface HTTP 401; got: %v", err)
	assert.Equal(t, "Bearer wrong-key", gotAuth, "provider must transmit the configured (wrong) bearer token")

	// (3) CORRECT key -> 200 + real completion
	pOK := NewProvider(Config{Endpoint: base, APIKey: key, Timeout: 10 * time.Second})
	resp, err := pOK.Complete(ctx, codeRequest())
	require.NoError(t, err, "correct API key must be accepted (HTTP 200)")
	require.NotNil(t, resp)
	assert.Contains(t, resp.Content, "func Add", "must return the real completion body")
	assert.Greater(t, resp.TokensUsed, 0, "usage must report real tokens")
	assert.Equal(t, "Bearer "+key, gotAuth, "provider must transmit the correct bearer token")
	assert.Equal(t, chatEndpoint, gotPath, "request path must be exactly /v1/chat/completions (no double /v1)")
}

// TestIntegration_HelixLLM_NoDoubleV1_BaseWithV1Suffix is the standing GREEN
// regression guard for the base-URL gotcha (§11.4.115): an operator who writes
// the /v1 suffix still lands on a single /v1/chat/completions. Without
// normalizeBase the request would hit /v1/v1/chat/completions → 404 → error.
func TestIntegration_HelixLLM_NoDoubleV1_BaseWithV1Suffix(t *testing.T) {
	var gotPath string
	srv := newMockOpenAIServer(t, "", &gotPath, nil)
	base := srv.URL + "/v1" // the gotcha input

	p := NewProvider(Config{Endpoint: base, Timeout: 10 * time.Second})
	require.Equal(t, srv.URL, p.Endpoint(), "a /v1-suffixed base must normalize to the bare host:port")

	resp, err := p.Complete(context.Background(), codeRequest())
	require.NoError(t, err, "single-/v1 request must succeed; a double /v1/v1 would 404")
	require.NotNil(t, resp)
	assert.Equal(t, chatEndpoint, gotPath, "server must observe exactly /v1/chat/completions")
}

// TestIntegration_HelixLLM_HostPortEnvDrivesRequest proves the LAN/VPN
// HELIX_LLM_HOST + HELIX_LLM_PORT composition drives a real HTTP request
// end-to-end through the real provider (the mechanism the live-LAN proof
// exercises against the actual coder).
func TestIntegration_HelixLLM_HostPortEnvDrivesRequest(t *testing.T) {
	clearHelixLLMEndpointEnv(t)
	const key = "hostport-key"
	var gotPath, gotAuth string
	srv := newMockOpenAIServer(t, key, &gotPath, &gotAuth)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	t.Setenv(EnvHost, u.Hostname())
	t.Setenv(EnvPort, u.Port())

	p := NewProvider(Config{APIKey: key, Timeout: 10 * time.Second})
	require.Equal(t, "http://"+u.Host, p.Endpoint(), "HOST/PORT env must compose the base client endpoint")

	resp, err := p.Complete(context.Background(), codeRequest())
	require.NoError(t, err, "HOST/PORT-composed request with correct key must succeed")
	require.NotNil(t, resp)
	assert.Contains(t, resp.Content, "func Add")
	assert.Equal(t, "Bearer "+key, gotAuth)
	assert.Equal(t, chatEndpoint, gotPath)
}
