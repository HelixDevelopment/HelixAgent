package handlers

import (
	"os"
	"strings"
	"testing"

	"dev.helix.agent/internal/models"
)

// TestUsageTokenSplit_NotFabricated is the §11.4.115 polarity test for the
// fabricated-token-usage defect found on 2026-09-03.
//
// THE DEFECT. Every OpenAI-compatible response shaper in this package
// synthesised the prompt/completion split by halving one number:
//
//	PromptTokens:     resp.TokensUsed / 2
//	CompletionTokens: resp.TokensUsed / 2
//	TotalTokens:      resp.TokensUsed
//
// That is invented telemetry. It is not an approximation of the real
// split — it discards a real split the provider already handed us.
// Providers under internal/llm/providers record the per-direction
// counts they receive in LLMResponse.Metadata — most under
// "prompt_tokens" + "completion_tokens", the anthropic family under
// "input_tokens" + "output_tokens" — so the true numbers were present
// in memory and were thrown away. Where a provider receives no usage
// object from its upstream it records no split, and the honest output
// is zeros, never a half.
//
// Two observable consequences, both captured live against :7061:
//
//  1. prompt_tokens ALWAYS equals completion_tokens, for every request,
//     which is never true of a real completion.
//  2. The arithmetic does not even close on odd totals. Measured
//     response, 2026-09-03 18:09 (provider=mistral, via
//     /v1/chat/completions model=helixagent-llm):
//     "usage":{"prompt_tokens":20,"completion_tokens":20,"total_tokens":41}
//     20 + 20 = 40 ≠ 41. A client reconciling the fields sees a
//     self-contradicting envelope.
//
// POLARITY. RED_MODE=1 reproduces the defect on a pre-fix build: it
// asserts the fabricated 50/50 split IS emitted. RED_MODE unset/"0"
// (the default, and the standing GREEN regression guard per §11.4.135)
// asserts the real provider-reported split is emitted instead.
//
// The fixture deliberately uses an ODD total with an ASYMMETRIC real
// split (7 prompt + 34 completion = 41) so that the fabricated
// behaviour and the correct behaviour cannot coincide.
func TestUsageTokenSplit_NotFabricated(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	const (
		wantPrompt     = 7
		wantCompletion = 34
		wantTotal      = 41
	)

	resp := &models.LLMResponse{
		ID:           "chatcmpl-token-split-fixture",
		ProviderName: "deepseek",
		Content:      "391",
		TokensUsed:   wantTotal,
		FinishReason: "stop",
		Metadata: map[string]interface{}{
			// Exactly the shape internal/llm/providers/*/*.go writes.
			"prompt_tokens":     wantPrompt,
			"completion_tokens": wantCompletion,
			"total_tokens":      wantTotal,
		},
	}

	h := &UnifiedHandler{}
	got := h.convertSingleResponseToOpenAI(resp, "helixagent-llm")

	if got.Usage == nil {
		t.Fatal("response carried no usage envelope at all")
	}

	if redMode {
		// Reproduce-and-assert-defect-present. A pre-fix build halves
		// the total, so both directions read 20 and the fields do not
		// sum to the total.
		if got.Usage.PromptTokens != got.Usage.CompletionTokens {
			t.Fatalf(
				"RED_MODE=1: expected the fabricated 50/50 split (prompt == completion), "+
					"but got prompt=%d completion=%d total=%d — the defect did NOT reproduce. "+
					"This is a FINDING, not evidence of a fix: either the fabrication is already "+
					"repaired in this build (run without RED_MODE for the GREEN guard) or the "+
					"fixture no longer reaches the halving code path.",
				got.Usage.PromptTokens, got.Usage.CompletionTokens, got.Usage.TotalTokens,
			)
		}
		if got.Usage.PromptTokens+got.Usage.CompletionTokens == got.Usage.TotalTokens {
			t.Fatalf(
				"RED_MODE=1: expected the halved split to NOT sum to the total on an odd "+
					"total, but %d + %d == %d — defect did not reproduce.",
				got.Usage.PromptTokens, got.Usage.CompletionTokens, got.Usage.TotalTokens,
			)
		}
		t.Logf(
			"RED_MODE=1: reproduced the fabricated-usage defect — provider reported "+
				"prompt=%d completion=%d total=%d, wire envelope claimed prompt=%d "+
				"completion=%d total=%d (%d+%d=%d ≠ %d).",
			wantPrompt, wantCompletion, wantTotal,
			got.Usage.PromptTokens, got.Usage.CompletionTokens, got.Usage.TotalTokens,
			got.Usage.PromptTokens, got.Usage.CompletionTokens,
			got.Usage.PromptTokens+got.Usage.CompletionTokens, got.Usage.TotalTokens,
		)
		return
	}

	// GREEN guard: the real, provider-reported split must survive to the wire.
	if got.Usage.PromptTokens != wantPrompt {
		t.Errorf("prompt_tokens = %d, want %d (provider-reported, from Metadata)",
			got.Usage.PromptTokens, wantPrompt)
	}
	if got.Usage.CompletionTokens != wantCompletion {
		t.Errorf("completion_tokens = %d, want %d (provider-reported, from Metadata)",
			got.Usage.CompletionTokens, wantCompletion)
	}
	if got.Usage.TotalTokens != wantTotal {
		t.Errorf("total_tokens = %d, want %d", got.Usage.TotalTokens, wantTotal)
	}
	if got.Usage.PromptTokens+got.Usage.CompletionTokens != got.Usage.TotalTokens {
		t.Errorf("usage envelope is self-contradicting: %d + %d != %d",
			got.Usage.PromptTokens, got.Usage.CompletionTokens, got.Usage.TotalTokens)
	}
}

// TestUsageFromResponse is the §1.1 guard for the TOOL-RESULT turn's
// usage envelope (the `usageFromResponse` seam in ChatCompletions).
//
// That path had its own, distinct fabrication: it estimated the whole
// envelope from the reply's STRING LENGTH —
// `PromptTokens: len(reply)/4, CompletionTokens: len(reply)/4,
// TotalTokens: len(reply)/2` — charging the PROMPT by the RESPONSE
// size. An independent review found it after the first five sites were
// fixed, so it gets its own falsifiable guard rather than relying on
// the shared helper's coverage.
//
// Fixtures use an ODD total with an ASYMMETRIC split so neither the
// halving nor the string-length estimate can satisfy them.
func TestUsageFromResponse(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		t.Skip("SKIP-OK: RED_MODE reproduces the 50/50 halving only; " +
			"this guard covers the separate tool-result string-length fabrication")
	}

	t.Run("real_provider_split_reaches_the_envelope", func(t *testing.T) {
		usage := usageFromResponse(&models.LLMResponse{
			Content:    "some synthesised tool-result summary text",
			TokensUsed: 41,
			Metadata: map[string]interface{}{
				"prompt_tokens":     7,
				"completion_tokens": 34,
				"total_tokens":      41,
			},
		})

		if usage == nil {
			t.Fatal("no usage envelope produced")
		}
		if usage.PromptTokens != 7 {
			t.Errorf("prompt_tokens = %d, want 7 (provider-reported)", usage.PromptTokens)
		}
		if usage.CompletionTokens != 34 {
			t.Errorf("completion_tokens = %d, want 34 (provider-reported)", usage.CompletionTokens)
		}
		if usage.TotalTokens != 41 {
			t.Errorf("total_tokens = %d, want 41", usage.TotalTokens)
		}
		if usage.PromptTokens == usage.CompletionTokens {
			t.Error("equal directions on an asymmetric fixture — fabricated-split signature")
		}
		if usage.PromptTokens+usage.CompletionTokens != usage.TotalTokens {
			t.Errorf("self-contradicting envelope: %d + %d != %d",
				usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
		}
	})

	t.Run("content_length_must_not_influence_the_numbers", func(t *testing.T) {
		// Same reported usage, wildly different content lengths. A
		// string-length estimate would move with the content; real
		// reported counts cannot.
		short := usageFromResponse(&models.LLMResponse{
			Content:    "ok",
			TokensUsed: 41,
			Metadata: map[string]interface{}{
				"prompt_tokens":     7,
				"completion_tokens": 34,
			},
		})
		long := usageFromResponse(&models.LLMResponse{
			Content:    strings.Repeat("a very long synthesised answer. ", 200),
			TokensUsed: 41,
			Metadata: map[string]interface{}{
				"prompt_tokens":     7,
				"completion_tokens": 34,
			},
		})

		if *short != *long {
			t.Errorf(
				"usage changed with content length (%+v vs %+v) — the envelope is being "+
					"derived from the reply string instead of the provider's counts",
				*short, *long,
			)
		}
	})

	t.Run("nil_response_yields_zeros_not_a_panic", func(t *testing.T) {
		usage := usageFromResponse(nil)
		if usage == nil {
			t.Fatal("no usage envelope produced")
		}
		if usage.PromptTokens != 0 || usage.CompletionTokens != 0 || usage.TotalTokens != 0 {
			t.Errorf("nil response must yield an all-zero envelope, got %+v", *usage)
		}
	})
}

// TestUsageTokenSplit_NoMetadata_ReportsZerosNotInvention covers the
// honest-fallback half of the contract (§11.4.6): when a provider does
// NOT report a split, the shaper must report zeros for the unknown
// directions rather than inventing halves. Zero is OpenAI-legal and is
// read by clients as "not reported"; a fabricated half is read as a
// measurement that never happened.
func TestUsageTokenSplit_NoMetadata_ReportsZerosNotInvention(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		t.Skip("SKIP-OK: RED_MODE covers the fabricated-split reproduction only; " +
			"this case asserts post-fix honest-zero behaviour")
	}

	resp := &models.LLMResponse{
		ID:         "chatcmpl-no-metadata-fixture",
		Content:    "hello",
		TokensUsed: 41,
		// No Metadata at all — provider gave us only a total.
	}

	h := &UnifiedHandler{}
	got := h.convertSingleResponseToOpenAI(resp, "helixagent-llm")

	if got.Usage == nil {
		t.Fatal("response carried no usage envelope at all")
	}
	if got.Usage.TotalTokens != 41 {
		t.Errorf("total_tokens = %d, want 41 (the one number we genuinely have)",
			got.Usage.TotalTokens)
	}
	if got.Usage.PromptTokens != 0 || got.Usage.CompletionTokens != 0 {
		t.Errorf(
			"unreported directions must be 0, not invented: got prompt=%d completion=%d",
			got.Usage.PromptTokens, got.Usage.CompletionTokens,
		)
	}
}
