package generic

import (
	"testing"
	"time"

	"dev.helix.agent/internal/models"
)

// TestProvider_ConvertResponse_RecordsRealTokenSplit guards the
// 2026-09-03 change that stopped this provider parsing the upstream
// per-direction token counts and then throwing them away, keeping only
// the total. Because models.LLMResponse.TokenSplit reads these Metadata
// keys to build the OpenAI-compatible usage envelope, discarding them
// made every response report prompt_tokens=0 / completion_tokens=0 for
// numbers the upstream had actually measured.
func TestProvider_ConvertResponse_RecordsRealTokenSplit(t *testing.T) {
	p := NewGenericProvider("testprovider", "key", "https://example.invalid/v1/chat/completions", "test-model")

	t.Run("split_recorded_when_upstream_reports_usage", func(t *testing.T) {
		// Asymmetric, odd total — a 50/50 halving cannot match.
		got := p.convertResponse(&models.LLMRequest{ID: "req-1"}, &Response{
			ID:      "cmpl-1",
			Choices: []Choice{{Message: Message{Role: "assistant", Content: "391"}, FinishReason: "stop"}},
			Usage:   &Usage{PromptTokens: 7, CompletionTokens: 34, TotalTokens: 41},
		}, time.Now())

		if got.TokensUsed != 41 {
			t.Errorf("TokensUsed = %d, want 41", got.TokensUsed)
		}
		if v := got.Metadata["prompt_tokens"]; v != 7 {
			t.Errorf("Metadata[prompt_tokens] = %v, want 7", v)
		}
		if v := got.Metadata["completion_tokens"]; v != 34 {
			t.Errorf("Metadata[completion_tokens] = %v, want 34", v)
		}
		if v := got.Metadata["total_tokens"]; v != 41 {
			t.Errorf("Metadata[total_tokens] = %v, want 41", v)
		}
	})

	t.Run("no_usage_object_claims_no_split", func(t *testing.T) {
		got := p.convertResponse(&models.LLMRequest{ID: "req-2"}, &Response{
			ID:      "cmpl-2",
			Choices: []Choice{{Message: Message{Role: "assistant", Content: "hi"}, FinishReason: "stop"}},
			Usage:   nil,
		}, time.Now())

		if _, present := got.Metadata["prompt_tokens"]; present {
			t.Error("prompt_tokens must be absent when the upstream sent no usage object")
		}
		if got.TokensUsed != 0 {
			t.Errorf("TokensUsed = %d, want 0 when no usage reported", got.TokensUsed)
		}
	})
}
