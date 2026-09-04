package ollama

import "testing"

// TestOllamaProvider_RealTokenCountsPreferredOverEstimate guards the
// 2026-09-03 change that stopped this provider guessing token usage
// from the reply's character length.
//
// Ollama's final (done) response reports prompt_eval_count (input) and
// eval_count (generated) — field names verified against the current
// official API reference (github.com/ollama/ollama docs/api.md, POST
// /api/generate, accessed 2026-09-03). Those fields were not parsed at
// all, so TokensUsed held len(response)/4: an estimate of the OUTPUT
// side presented as a total, and no per-direction split at all, which
// left the OpenAI-compatible usage envelope reporting 0/0.
func TestOllamaProvider_RealTokenCountsPreferredOverEstimate(t *testing.T) {
	provider := NewOllamaProvider("http://localhost:11434", "llama3.2")

	t.Run("real_counts_are_used_and_split_is_recorded", func(t *testing.T) {
		// Deliberately asymmetric (26 != 259) and summing to an ODD
		// total, so neither a 50/50 halving nor a character-length
		// estimate can produce these numbers by accident. Values are
		// the ones in the official docs' sample response.
		resp, err := provider.convertResponse(&OllamaResponse{
			Model:           "llama3.2",
			Response:        "short",
			Done:            true,
			PromptEvalCount: 26,
			EvalCount:       259,
		}, "req-1")
		if err != nil {
			t.Fatalf("convertResponse: %v", err)
		}

		if resp.TokensUsed != 285 {
			t.Errorf("TokensUsed = %d, want 285 (26 input + 259 generated)", resp.TokensUsed)
		}
		if got := resp.Metadata["prompt_tokens"]; got != 26 {
			t.Errorf("Metadata[prompt_tokens] = %v, want 26", got)
		}
		if got := resp.Metadata["completion_tokens"]; got != 259 {
			t.Errorf("Metadata[completion_tokens] = %v, want 259", got)
		}
		if got := resp.Metadata["total_tokens"]; got != 285 {
			t.Errorf("Metadata[total_tokens] = %v, want 285", got)
		}
	})

	t.Run("counts_do_not_track_content_length", func(t *testing.T) {
		// Same reported counts, very different reply lengths. A
		// character-length estimate would move; measured counts cannot.
		short, err := provider.convertResponse(&OllamaResponse{
			Response: "hi", Done: true, PromptEvalCount: 26, EvalCount: 259,
		}, "req-2")
		if err != nil {
			t.Fatalf("convertResponse: %v", err)
		}
		long, err := provider.convertResponse(&OllamaResponse{
			Response: string(make([]byte, 8192)), Done: true, PromptEvalCount: 26, EvalCount: 259,
		}, "req-3")
		if err != nil {
			t.Fatalf("convertResponse: %v", err)
		}

		if short.TokensUsed != long.TokensUsed {
			t.Errorf(
				"TokensUsed changed with content length (%d vs %d) — usage is being "+
					"estimated from the reply string instead of Ollama's counts",
				short.TokensUsed, long.TokensUsed)
		}
	})

	t.Run("no_counts_reported_claims_no_split", func(t *testing.T) {
		// Older Ollama, or a non-final chunk: the estimate remains the
		// only figure available, and NO split is claimed (claiming one
		// would be fabrication).
		resp, err := provider.convertResponse(&OllamaResponse{
			Response: "abcdefgh", Done: true,
		}, "req-4")
		if err != nil {
			t.Fatalf("convertResponse: %v", err)
		}
		if _, present := resp.Metadata["prompt_tokens"]; present {
			t.Error("prompt_tokens must be absent when Ollama reported no counts")
		}
		if _, present := resp.Metadata["completion_tokens"]; present {
			t.Error("completion_tokens must be absent when Ollama reported no counts")
		}
	})
}
