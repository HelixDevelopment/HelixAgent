package unit

import (
	"testing"

	"github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
)

func TestRunEnsembleBasic(t *testing.T) {
	// Build a minimal request and ensure ensemble runs without error
	req := &models.LLMRequest{
		ID: "req-1",
		ModelParams: models.ModelParameters{
			Model: "llama2",
		},
	}

	responses, selected, err := llm.RunEnsemble(req)
	if err != nil {
		t.Fatalf("ensemble error: %v", err)
	}

	// If no providers are available (common in test environment), that's acceptable
	if len(responses) == 0 {
		t.Skip("No LLM providers available for testing - skipping ensemble test")
		return
	}

	if selected == nil {
		t.Fatalf("expected a selected response from ensemble")
	}
}
