// Package types defines Go types for the ourobopus library.
// Go library for ourobopus implementing self-referential AI patterns including recursive self-improvement, metacognitive reasoning, self-evaluation loops, and feedback-driven prompt refinement.
package types

import (
	"fmt"
	"strings"
)

// MetaPrompt represents metaprompt data.
type MetaPrompt struct {
	ReflectionQuestions []string
	Description string
	Category string
	IterationCount int
	ID string
	BasePrompt string
	Name string
}

// Validate checks that the MetaPrompt is valid.
func (o *MetaPrompt) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// SelfReflection represents selfreflection data.
type SelfReflection struct {
	OriginalPrompt string
	SuggestedImprovements []string
	IdentifiedWeaknesses []string
	Confidence float64
	SelfAssessment string
}

// IterationResult represents iterationresult data.
type IterationResult struct {
	Output string
	SelfScore float64
	Improvements []string
	Prompt string
	Iteration int
}

// Validate checks that the IterationResult is valid.
func (o *IterationResult) Validate() error {
	if strings.TrimSpace(o.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	return nil
}

// RefinementConfig represents refinementconfig data.
type RefinementConfig struct {
	Model string
	EarlyStop bool
	EvaluationCriteria []string
	TargetScore float64
	Iterations int
	InitialPrompt string
}

// Validate checks that the RefinementConfig is valid.
func (o *RefinementConfig) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(o.InitialPrompt) == "" {
		return fmt.Errorf("initialprompt is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *RefinementConfig) Defaults() {
	if o.Iterations == 0 {
		o.Iterations = 3
	}
	if o.TargetScore == 0 {
		o.TargetScore = 0.8
	}
}

// RefinementResult represents refinementresult data.
type RefinementResult struct {
	FinalOutput string
	FinalScore float64
	ImprovementHistory []float64
	Iterations []IterationResult
	FinalPrompt string
}

// MetaEvaluation represents the result of a meta-evaluation on a prompt/output pair.
type MetaEvaluation struct {
	Prompt      string
	Output      string
	Criteria    []string
	Scores      map[string]float64
	OverallScore float64
	Analysis    string
}

