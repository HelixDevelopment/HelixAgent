package unit

import (
	"testing"

	"github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
)

func TestRunEnsembleBasic(t *testing.T) {
	// Build a minimal request and ensure ensemble runs without error
	req := &models.LLMRequest{ID: "req-1"}
	responses, selected, err := llm.RunEnsemble(req)
	if err != nil {
		t.Fatalf("ensemble error: %v", err)
	}
	if len(responses) == 0 {
		t.Fatalf("expected at least one response from ensemble, got %d", len(responses))
	}
	if selected == nil {
		t.Fatalf("expected a selected response from ensemble")
	}
}
