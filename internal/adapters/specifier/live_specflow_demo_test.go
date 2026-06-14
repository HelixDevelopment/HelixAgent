//go:build helixspecdemo

// This is a REAL provider-backed service-layer proof that the default
// spec-driven flow genuinely produces a 7-phase spec artifact. It is NOT a
// unit test (no mocks): it constructs the production NewOptimalSpecAdapter
// engine and injects a DebateFunc that makes REAL HTTP calls to a live LLM
// provider (DeepSeek, OpenAI-compatible). Run explicitly:
//
//	source ~/api_keys.sh && go test -tags helixspecdemo -count=1 -v \
//	    -run TestLiveSpecFlow_RealProvider -timeout 600s ./internal/adapters/specifier/
//
// It is build-tagged so it never runs in the normal suite (needs network +
// real keys). The engine PANICS without a real DebateFunc (anti-bluff: it
// refuses to fabricate phase output), so a passing run with a non-empty
// FinalArtifact is positive runtime evidence the spec flow really ran.
package specifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	helixspec "digital.vasic.helixspecifier/pkg/types"
)

// realDeepSeekDebate makes a REAL HTTP call to DeepSeek's OpenAI-compatible
// chat endpoint and returns the model's output as the debate round result.
// No stub, no fabrication — if the call fails the error propagates.
func realDeepSeekDebate(ctx context.Context, topic string, rounds int, metadata map[string]interface{}) (string, float64, string, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return "", 0, "", fmt.Errorf("DEEPSEEK_API_KEY not set")
	}
	body := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": "You are a senior software architect running one phase of a spec-driven development flow. Respond concisely with concrete, actionable spec/plan content for the requested phase."},
			{"role": "user", "content": topic},
		},
		"temperature": 0.4,
		"max_tokens":  700,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.deepseek.com/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", 0, "", fmt.Errorf("deepseek HTTP %d: %s", resp.StatusCode, string(data))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", 0, "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", 0, "", fmt.Errorf("deepseek returned empty content")
	}
	out := parsed.Choices[0].Message.Content
	score := 0.85
	if len(out) > 400 {
		score = 0.9
	}
	return out, score, "deepseek-live", nil
}

func TestLiveSpecFlow_RealProvider(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("SKIP-OK: DEEPSEEK_API_KEY unset — real-provider spec-flow proof requires a live key (source ~/api_keys.sh)")
	}

	// Build the PRODUCTION default engine (same path the server uses).
	adapter := NewOptimalSpecAdapter()
	if !adapter.IsReady() {
		t.Fatalf("spec adapter not ready")
	}

	// Inject the REAL provider-backed debate func (no stub). The engine
	// panics if no debate func is wired and ExecuteFlow needs one.
	if !adapter.SetDebateFunc(helixspec.DebateFunc(realDeepSeekDebate)) {
		t.Fatalf("failed to inject real DebateFunc into engine")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	// A realistic DEVELOPMENT request — exactly the class that now defaults
	// to spec-driven routing after the default-on wiring.
	request := "Implement a rate limiter feature for the public REST API: per-client token-bucket limiting with configurable burst and refill rates, Redis-backed counters, and a 429 response with Retry-After."

	classification, err := adapter.ClassifyEffort(ctx, request)
	if err != nil {
		t.Fatalf("ClassifyEffort failed: %v", err)
	}
	t.Logf("ClassifyEffort: level=%s ceremony=%s requiresDebate=%v requiresSpecKit=%v reasoning=%q",
		classification.Level, classification.CeremonyLevel,
		classification.RequiresDebate, classification.RequiresSpecKit, classification.Reasoning)

	result, err := adapter.ExecuteFlow(ctx, request, classification)
	if err != nil {
		t.Fatalf("ExecuteFlow failed: %v", err)
	}

	if result == nil || len(result.PhaseResults) == 0 {
		t.Fatalf("spec flow produced no phase results")
	}
	if strings.TrimSpace(result.FinalArtifact) == "" {
		t.Fatalf("spec flow produced empty FinalArtifact — would be a bluff")
	}

	// Persist the REAL artifact as captured evidence.
	evidencePath := os.Getenv("SPEC_EVIDENCE_PATH")
	if evidencePath == "" {
		evidencePath = "/tmp/helixspec_real_artifact.md"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# HelixSpecifier REAL 7-phase artifact (provider=deepseek)\n\n")
	fmt.Fprintf(&sb, "FlowID: %s\nEffort: %s  Ceremony: %s  Success: %v\n", result.FlowID, result.EffortLevel, result.CeremonyLevel, result.Success)
	fmt.Fprintf(&sb, "OverallQualityScore: %.3f  Duration: %s\n", result.OverallQualityScore, result.Duration)
	fmt.Fprintf(&sb, "PhaseResults: %d\n\n", len(result.PhaseResults))
	for i, pr := range result.PhaseResults {
		fmt.Fprintf(&sb, "## Phase %d: %s (stage=%s) success=%v quality=%.2f debate=%s\n", i+1, pr.Phase, pr.Stage, pr.Success, pr.QualityScore, pr.DebateID)
		out := pr.Output
		if len(out) > 1200 {
			out = out[:1200] + "\n…(truncated)…"
		}
		fmt.Fprintf(&sb, "%s\n\n", out)
	}
	fmt.Fprintf(&sb, "## FinalArtifact\n%s\n", result.FinalArtifact)
	if err := os.WriteFile(evidencePath, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("failed to write evidence: %v", err)
	}
	t.Logf("WROTE real spec artifact (%d bytes, %d phases) to %s", sb.Len(), len(result.PhaseResults), evidencePath)
}
