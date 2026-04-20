// Package types defines Go types for the G0DM0D3 library.
// Go implementation of the G0DM0D3 liberated AI chat framework. Provides parallel model racing (GODMODE CLASSIC), multi-model evaluation (ULTRAPLINIAN), input perturbation (Parseltongue), context-adaptive sampling (AutoTune), and semantic transformation modules (STM).
package types

import (
	"fmt"
	"strings"
)

// ChatRequest represents chatrequest data.
type ChatRequest struct {
	Models []string
	Mode string
	MaxTokens int
	Prompt string
	Temperature float64
	SystemPrompt string
}

// Validate checks that the ChatRequest is valid.
func (o *ChatRequest) Validate() error {
	if strings.TrimSpace(o.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *ChatRequest) Defaults() {
	if o.MaxTokens == 0 { o.MaxTokens = 2048 }
	if o.Temperature == 0 { o.Temperature = 0.7 }
}

// ChatResponse represents chatresponse data.
type ChatResponse struct {
	Responses []ModelResponse
	Evaluation *EvaluationResult
	Consensus string
	BestResponse string
}

// ModelResponse represents modelresponse data.
type ModelResponse struct {
	Model string
	Response string
	TokensUsed int
	LatencyMs int64
	FinishReason string
}

// Validate checks that the ModelResponse is valid.
func (o *ModelResponse) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

// EvaluationResult represents evaluationresult data.
type EvaluationResult struct {
	Analysis string
	Rankings []string
	Scores map[string]float64
	Disagreements []string
}

// ParseltongueOptions represents parseltongueoptions data.
type ParseltongueOptions struct {
	Techniques []string
	Text string
	Intensity float64
}

// Validate checks that the ParseltongueOptions is valid.
func (o *ParseltongueOptions) Validate() error {
	if strings.TrimSpace(o.Text) == "" {
		return fmt.Errorf("text is required")
	}
	return nil
}

// AutoTuneOptions represents autotuneoptions data.
type AutoTuneOptions struct {
	Model string
	Context string
	Prompt string
	TargetCoherence float64
	TargetCreativity float64
}

// Validate checks that the AutoTuneOptions is valid.
func (o *AutoTuneOptions) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(o.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	return nil
}

// RaceResult represents raceresult data.
type RaceResult struct {
	Responses []ModelResponse
	Winner string
	VoteDistribution map[string]int
	TimeMs int64
}

