package router

import (
	"testing"

	"dev.helix.agent/internal/models"
)

// TestEnsembleUsage is the §1.1 guard for the ensemble endpoint's
// `usage` envelope.
//
// It exists because the envelope's only previous coverage was a
// presence check (`assert.Contains(response, "usage")`) in
// router_test.go, which passes no matter WHAT the three numbers are. An
// independent review proved that reinstating the removed fabricated
// `TokensUsed / 2` split at this site survived the whole suite
// undetected. The fixtures below use an ODD total with an ASYMMETRIC
// split so the fabricated behaviour (20/20/41) and the correct
// behaviour (7/34/41) cannot coincide.
func TestEnsembleUsage(t *testing.T) {
	tests := []struct {
		name           string
		selected       *models.LLMResponse
		wantPrompt     int
		wantCompletion int
		wantTotal      int
	}{
		{
			// The shape the live defect mis-reported as 20/20/41.
			name: "real_provider_split_reaches_the_envelope",
			selected: &models.LLMResponse{
				ID:         "ens-1",
				Content:    "391",
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
			// Anthropic-shaped keys must be honoured too, or a real
			// split is silently reported as zeros.
			name: "anthropic_shaped_split_reaches_the_envelope",
			selected: &models.LLMResponse{
				TokensUsed: 41,
				Metadata: map[string]interface{}{
					"input_tokens":  7,
					"output_tokens": 34,
				},
			},
			wantPrompt: 7, wantCompletion: 34, wantTotal: 41,
		},
		{
			// No split reported ⇒ honest zeros plus the real total.
			// NOTE: an odd total is used deliberately — with an even
			// one, halving would coincidentally satisfy the sum and
			// weaken this case as a mutation guard.
			name: "no_split_reported_yields_honest_zeros",
			selected: &models.LLMResponse{
				TokensUsed: 41,
			},
			wantPrompt: 0, wantCompletion: 0, wantTotal: 41,
		},
		{
			// A nil selection must not panic inside the handler.
			name:       "nil_selection_does_not_panic",
			selected:   nil,
			wantPrompt: 0, wantCompletion: 0, wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := ensembleUsage(tt.selected)

			got := func(key string) int {
				raw, present := usage[key]
				if !present {
					t.Fatalf("usage envelope is missing %q", key)
				}
				n, ok := raw.(int)
				if !ok {
					t.Fatalf("usage[%q] = %T, want int", key, raw)
				}
				return n
			}

			prompt := got("prompt_tokens")
			completion := got("completion_tokens")
			total := got("total_tokens")

			if prompt != tt.wantPrompt {
				t.Errorf("prompt_tokens = %d, want %d", prompt, tt.wantPrompt)
			}
			if completion != tt.wantCompletion {
				t.Errorf("completion_tokens = %d, want %d", completion, tt.wantCompletion)
			}
			if total != tt.wantTotal {
				t.Errorf("total_tokens = %d, want %d", total, tt.wantTotal)
			}

			// The fabricated-split signature: both directions equal and
			// non-zero on a fixture whose real split is asymmetric.
			if prompt != 0 && prompt == completion && tt.wantPrompt != tt.wantCompletion {
				t.Errorf(
					"prompt_tokens == completion_tokens == %d on an asymmetric fixture — "+
						"this is the fabricated 50/50 split signature", prompt)
			}
			// Whenever both directions are known the envelope must add up.
			if prompt != 0 && completion != 0 && prompt+completion != total {
				t.Errorf("self-contradicting envelope: %d + %d != %d", prompt, completion, total)
			}
		})
	}
}
