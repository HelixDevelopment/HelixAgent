package openrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"dev.helix.agent/internal/models"
)

// TestSimpleOpenRouterProvider_Complete_RecordsRealTokenSplit guards the
// 2026-09-03 change that stopped this provider parsing the upstream
// per-direction token counts and then copying only the total onto the
// response.
//
// models.LLMResponse.TokenSplit reads these Metadata keys to build the
// OpenAI-compatible usage envelope, so discarding them made every
// OpenRouter response publish prompt_tokens=0 / completion_tokens=0 for
// values OpenRouter had measured and returned.
func TestSimpleOpenRouterProvider_Complete_RecordsRealTokenSplit(t *testing.T) {
	// Asymmetric split with an ODD total — halving 41 gives 20/20, so a
	// reinstated fabrication cannot satisfy these assertions.
	const body = `{
	  "id":"gen-1",
	  "model":"test/model",
	  "choices":[{"message":{"role":"assistant","content":"391"},"finish_reason":"stop"}],
	  "usage":{"prompt_tokens":7,"completion_tokens":34,"total_tokens":41}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	p := NewSimpleOpenRouterProviderWithBaseURL("test-key", srv.URL)

	resp, err := p.Complete(context.Background(), &models.LLMRequest{
		ID:     "req-1",
		Prompt: "What is 17 multiplied by 23?",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.TokensUsed != 41 {
		t.Errorf("TokensUsed = %d, want 41", resp.TokensUsed)
	}
	if resp.Metadata == nil {
		t.Fatal("Metadata is nil — the real token split was discarded")
	}
	if v := resp.Metadata["prompt_tokens"]; v != 7 {
		t.Errorf("Metadata[prompt_tokens] = %v, want 7", v)
	}
	if v := resp.Metadata["completion_tokens"]; v != 34 {
		t.Errorf("Metadata[completion_tokens] = %v, want 34", v)
	}
	if v := resp.Metadata["total_tokens"]; v != 41 {
		t.Errorf("Metadata[total_tokens] = %v, want 41", v)
	}
}
