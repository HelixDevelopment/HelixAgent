// Package types defines Go types for the Dioscuri library.
// Go library for Dioscuri implementing dual-model AI interaction patterns inspired by the mythological twins. Enables collaborative reasoning, debate-based analysis, and consensus building between two AI models.
package types

import (
	"fmt"
	"strings"
)

// DebateConfig represents debateconfig data.
type DebateConfig struct {
	Format string
	SystemPromptB string
	Topic string
	ModelB string
	JudgeModel string
	SystemPromptA string
	Rounds int
	ModelA string
}

// Validate checks that the DebateConfig is valid.
func (o *DebateConfig) Validate() error {
	if strings.TrimSpace(o.Topic) == "" {
		return fmt.Errorf("topic is required")
	}
	return nil
}

// DebateRound represents debateround data.
type DebateRound struct {
	ArgumentA string
	RebuttalA string
	Round int
	JudgeComment string
	RebuttalB string
	ArgumentB string
}

// DebateResult represents debateresult data.
type DebateResult struct {
	KeyPoints []string
	Consensus string
	AgreementAreas []string
	Rounds []DebateRound
	Winner string
	DisagreementAreas []string
}

// CollaborationConfig represents collaborationconfig data.
type CollaborationConfig struct {
	Mode string
	ModelA string
	Iterations int
	ModelB string
	Task string
}

// Validate checks that the CollaborationConfig is valid.
func (o *CollaborationConfig) Validate() error {
	if strings.TrimSpace(o.Task) == "" {
		return fmt.Errorf("task is required")
	}
	return nil
}

// CollaborationResult represents collaborationresult data.
type CollaborationResult struct {
	Iterations []Iteration
	FinalOutput string
	QualityScore float64
	Contributions map[string]float64
}

// Iteration represents iteration data.
type Iteration struct {
	OutputB string
	Improvement float64
	Num int
	OutputA string
	Merged string
}

