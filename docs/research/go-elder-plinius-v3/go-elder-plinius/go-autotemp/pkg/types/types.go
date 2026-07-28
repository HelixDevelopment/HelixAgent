// Package types defines Go types for the AutoTemp library.
// Go client for the AutoTemp service -- intelligent temperature optimization for LLM interactions. Runs prompts at multiple temperatures and selects the best output using multi-judge structured scoring.
package types

import (
	"fmt"
	"strings"
)

// RunOptions represents runoptions data.
type RunOptions struct {
	Prompt       string
	Temperatures []float64
	TopP         float64
	AutoSelect   bool
	Judges       int
	ModelVersion string
}

// Validate checks that the RunOptions is valid.
func (o *RunOptions) Validate() error {
	if strings.TrimSpace(o.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *RunOptions) Defaults() {
	if o.TopP == 0 {
		o.TopP = 1.0
	}
}

// RunResult represents runresult data.
type RunResult struct {
	BestOutput       string
	BestTemperature  float64
	BestOverallScore float64
	Summary          string
	Usage            TokenUsage
}

// TokenUsage represents tokenusage data.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// AdvancedOptions represents advancedoptions data.
type AdvancedOptions struct {
	RunOptions
	Rounds       int
	ExplorationC float64
}

// EvaluateOptions represents evaluateoptions data.
type EvaluateOptions struct {
	Prompt       string
	Output       string
	Temperature  float64
	TopP         float64
	Judges       int
	ModelVersion string
}

// Validate checks that the EvaluateOptions is valid.
func (o *EvaluateOptions) Validate() error {
	if strings.TrimSpace(o.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *EvaluateOptions) Defaults() {
	if o.Temperature == 0 {
		o.Temperature = 0.7
	}
	if o.TopP == 0 {
		o.TopP = 1.0
	}
}

// EvaluateResult represents evaluateresult data.
type EvaluateResult struct {
	OverallScore float64
	Scores       ScoreBreakdown
	Usage        TokenUsage
}

// ScoreBreakdown represents scorebreakdown data.
type ScoreBreakdown struct {
	Relevance  float64
	Clarity    float64
	Utility    float64
	Creativity float64
	Coherence  float64
	Safety     float64
	Overall    float64
}

// BenchmarkOptions represents benchmarkoptions data.
type BenchmarkOptions struct {
	Dataset      []BenchmarkItem
	Temperatures []float64
	TopP         float64
	Advanced     bool
	Rounds       int
	Judges       int
	Models       []string
}

// Validate checks that the BenchmarkOptions is valid.
func (o *BenchmarkOptions) Validate() error {
	return nil
}

// Defaults applies default values for unset fields.
func (o *BenchmarkOptions) Defaults() {
	if o.TopP == 0 {
		o.TopP = 1.0
	}
}

// BenchmarkItem represents benchmarkitem data.
type BenchmarkItem struct {
	Prompt    string
	Reference string
}

// Validate checks that the BenchmarkItem is valid.
func (o *BenchmarkItem) Validate() error {
	if strings.TrimSpace(o.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	return nil
}

// BenchmarkResult represents benchmarkresult data.
type BenchmarkResult struct {
	ModelResults []ModelBenchmark
	Summary      string
}

// ModelBenchmark represents modelbenchmark data.
type ModelBenchmark struct {
	ModelName   string
	MeanOverall float64
	NumItems    int
	Tokens      TokenUsage
}

// Validate checks that the ModelBenchmark is valid.
func (o *ModelBenchmark) Validate() error {
	if strings.TrimSpace(o.ModelName) == "" {
		return fmt.Errorf("modelname is required")
	}
	return nil
}
