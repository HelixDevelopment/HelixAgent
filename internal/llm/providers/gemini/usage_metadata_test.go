package gemini

import (
	"testing"
	"time"

	"dev.helix.agent/internal/models"
)

// TestGeminiAPIProvider_ConvertResponse_RecordsRealTokenSplit guards the
// 2026-09-03 change that stopped this provider keeping only
// totalTokenCount from usageMetadata and discarding the per-direction
// counts it had already parsed.
//
// Gemini names them promptTokenCount / candidatesTokenCount; they are
// mapped onto the canonical prompt_tokens / completion_tokens keys that
// models.LLMResponse.TokenSplit reads, so the OpenAI-compatible usage
// envelope reports what Gemini measured instead of 0 for both
// directions.
func TestGeminiAPIProvider_ConvertResponse_RecordsRealTokenSplit(t *testing.T) {
	p := NewGeminiAPIProvider("test-key", "", "gemini-1.5-pro")

	newResp := func(usage *GeminiUsageMetadata) *GeminiResponse {
		return &GeminiResponse{
			Candidates: []GeminiCandidate{{
				Content:      GeminiContent{Parts: []GeminiPart{{Text: "391"}}, Role: "model"},
				FinishReason: "STOP",
			}},
			UsageMetadata: usage,
		}
	}

	t.Run("split_recorded_when_gemini_reports_usage", func(t *testing.T) {
		// Asymmetric, odd total — a 50/50 halving cannot match.
		got := p.convertResponse(&models.LLMRequest{ID: "req-1"}, newResp(&GeminiUsageMetadata{
			PromptTokenCount:     7,
			CandidatesTokenCount: 34,
			TotalTokenCount:      41,
		}), time.Now())

		if got.TokensUsed != 41 {
			t.Errorf("TokensUsed = %d, want 41", got.TokensUsed)
		}
		if v := got.Metadata["prompt_tokens"]; v != 7 {
			t.Errorf("Metadata[prompt_tokens] = %v, want 7 (from promptTokenCount)", v)
		}
		if v := got.Metadata["completion_tokens"]; v != 34 {
			t.Errorf("Metadata[completion_tokens] = %v, want 34 (from candidatesTokenCount)", v)
		}
		if v := got.Metadata["total_tokens"]; v != 41 {
			t.Errorf("Metadata[total_tokens] = %v, want 41", v)
		}
	})

	t.Run("no_usage_metadata_claims_no_split", func(t *testing.T) {
		got := p.convertResponse(&models.LLMRequest{ID: "req-2"}, newResp(nil), time.Now())
		if _, present := got.Metadata["prompt_tokens"]; present {
			t.Error("prompt_tokens must be absent when Gemini sent no usageMetadata")
		}
	})
}
