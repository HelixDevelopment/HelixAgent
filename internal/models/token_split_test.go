package models

import (
	"encoding/json"
	"testing"
)

// TestLLMResponse_TokenSplit covers the honest-usage accessor added
// 2026-09-03 to replace the fabricated `TokensUsed / 2` split that every
// OpenAI-compatible response shaper used to emit.
//
// The contract under test: report the provider's REAL per-direction
// counts when it supplied them (they live in Metadata, written by 30+
// providers under internal/llm/providers), and report honest zeros when
// it did not — never an invented half.
func TestLLMResponse_TokenSplit(t *testing.T) {
	tests := []struct {
		name           string
		resp           *LLMResponse
		wantPrompt     int
		wantCompletion int
		wantTotal      int
	}{
		{
			// The shape the live defect produced: a real asymmetric
			// split available in Metadata, discarded in favour of
			// 20/20/41. 7+34=41 so the correct answer is unambiguous.
			name: "real_split_from_metadata_survives",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"prompt_tokens":     7,
					"completion_tokens": 34,
					"total_tokens":      41,
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			// float64 is what int values become after a JSON round-trip
			// through map[string]interface{} — a real path whenever a
			// response is serialised and re-read.
			name: "float64_after_json_roundtrip",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"prompt_tokens":     float64(7),
					"completion_tokens": float64(34),
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			name: "json_number_from_decoder_usenumber",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"prompt_tokens":     json.Number("7"),
					"completion_tokens": json.Number("34"),
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			name: "int64_values",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"prompt_tokens":     int64(7),
					"completion_tokens": int64(34),
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			// No split reported: zeros, NOT 20/20. Zero is
			// OpenAI-legal and reads as "not reported"; a fabricated
			// half claims a measurement that never happened.
			name:           "no_metadata_reports_honest_zeros",
			resp:           &LLMResponse{TokensUsed: 41},
			wantPrompt:     0,
			wantCompletion: 0,
			wantTotal:      41,
		},
		{
			name: "empty_metadata_reports_honest_zeros",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata:   map[string]interface{}{},
			},
			wantPrompt: 0, wantCompletion: 0, wantTotal: 41,
		},
		{
			// A garbage value is treated as absent, never guessed at
			// (§11.4.6). Both directions unparseable ⇒ honest zeros.
			name: "unparseable_values_treated_as_absent",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"prompt_tokens":     "not-a-number",
					"completion_tokens": []int{1, 2},
				},
			},
			wantPrompt: 0, wantCompletion: 0, wantTotal: 41,
		},
		{
			// Negative counts are impossible; treat as unreported.
			name: "negative_values_treated_as_absent",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"prompt_tokens":     -5,
					"completion_tokens": -9,
				},
			},
			wantPrompt: 0, wantCompletion: 0, wantTotal: 41,
		},
		{
			// Partial report WITH a usable aggregate: the missing
			// direction is DERIVED as total − known. That is
			// arithmetic, not invention — the OpenAI usage schema
			// defines total = prompt + completion, so two knowns
			// determine the third. 41 − 7 = 34.
			name: "only_prompt_reported_derives_completion_from_total",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"prompt_tokens": 7,
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			// Same rule in the other direction.
			name: "only_completion_reported_derives_prompt_from_total",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"completion_tokens": 34,
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			// The aggregate cannot be smaller than one of its parts, so
			// it is not a true total and the relationship between these
			// numbers is unknown. Report neither direction rather than
			// deriving a negative or otherwise unjustifiable figure.
			name: "aggregate_smaller_than_known_part_yields_zeros",
			resp: &LLMResponse{
				TokensUsed: 3,
				Metadata: map[string]interface{}{
					"prompt_tokens": 7,
				},
			},
			wantPrompt: 0, wantCompletion: 0, wantTotal: 3,
		},
		{
			// Partial report with no aggregate at all: the one known
			// part is all we have, so it becomes the total.
			name: "only_prompt_reported_no_aggregate",
			resp: &LLMResponse{
				Metadata: map[string]interface{}{
					"prompt_tokens": 7,
				},
			},
			wantPrompt: 7, wantCompletion: 0, wantTotal: 7,
		},
		{
			// Anthropic-shaped naming. internal/llm/providers/cohere
			// writes input_tokens + output_tokens; reading only the
			// OpenAI-shaped keys would return honest-looking zeros
			// while a REAL split sat available in the map — a subtler
			// form of the fabrication defect this accessor exists to
			// end.
			name: "anthropic_shaped_input_output_keys",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"input_tokens":  7,
					"output_tokens": 34,
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			// The claude provider now writes BOTH input_tokens and
			// output_tokens with TokensUsed as the true total (it used
			// to write input only, with TokensUsed holding the OUTPUT
			// count — so a Claude response silently under-reported
			// usage by the entire input side).
			name: "claude_shape_both_directions",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"model":         "claude-x",
					"input_tokens":  7,
					"output_tokens": 34,
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			// A legacy/stored response predating that provider fix
			// carries input_tokens only. The derive rule recovers the
			// output side from the aggregate instead of reporting 0.
			name: "legacy_input_only_derives_output_from_aggregate",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"model":        "claude-x",
					"input_tokens": 7,
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			// OpenAI-shaped keys win over Anthropic aliases when both
			// somehow appear, so precedence is deterministic and never
			// depends on map iteration order (§11.4.6).
			name: "openai_keys_take_precedence_over_aliases",
			resp: &LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"prompt_tokens":     7,
					"completion_tokens": 34,
					"input_tokens":      1000,
					"output_tokens":     2000,
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			// Provider's own total disagrees with its own parts — trust
			// the parts so prompt+completion==total always holds.
			name: "inconsistent_provider_total_is_derived_from_parts",
			resp: &LLMResponse{
				TokensUsed: 999,
				Metadata: map[string]interface{}{
					"prompt_tokens":     7,
					"completion_tokens": 34,
					"total_tokens":      12345,
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			name:       "nil_receiver_is_all_zeros",
			resp:       nil,
			wantPrompt: 0, wantCompletion: 0, wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, completion, total := tt.resp.TokenSplit()

			if prompt != tt.wantPrompt {
				t.Errorf("prompt = %d, want %d", prompt, tt.wantPrompt)
			}
			if completion != tt.wantCompletion {
				t.Errorf("completion = %d, want %d", completion, tt.wantCompletion)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}

			// The fabrication signature: both directions equal AND
			// non-zero. Never legal output for a real split unless the
			// provider genuinely reported equal counts (no fixture here
			// does), so this catches a reinstated halving directly.
			if prompt != 0 && prompt == completion && tt.wantPrompt != tt.wantCompletion {
				t.Errorf(
					"prompt == completion == %d looks like a fabricated 50/50 split", prompt)
			}
			// When BOTH directions are known the envelope must add up —
			// the exact invariant the fabricated 50/50 split violated
			// on odd totals (20 + 20 != 41). A partial report is
			// exempt: one direction is genuinely unknown there, and the
			// provider's aggregate is kept rather than understated.
			if prompt != 0 && completion != 0 && prompt+completion != total {
				t.Errorf("inconsistent envelope: %d + %d != %d", prompt, completion, total)
			}
		})
	}
}
