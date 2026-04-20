// Package types defines Go types for the BasiliskToken library.
// Go library for BasiliskToken implementing genetic algorithm-based prompt evolution for AI red teaming. Creates, mutates, and breeds prompt tokens to find adversarial inputs that bypass safety mechanisms.
package types

import (
	"fmt"
	"strings"
)

// TokenGenome represents tokengenome data.
type TokenGenome struct {
	ParentIDs []string
	ID string
	Generation int
	Tokens []Token
	Fitness float64
}

// Validate checks that the TokenGenome is valid.
func (o *TokenGenome) Validate() error {
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

// Token represents token data.
type Token struct {
	Position int
	Value string
	Mutable bool
	Type string
	Weight float64
}

// EvolutionConfig represents evolutionconfig data.
type EvolutionConfig struct {
	TargetModel string
	SelectionMethod string
	FitnessFunction string
	CrossoverRate float64
	EliteCount int
	MutationRate float64
	Generations int
	PopulationSize int
}

// Validate checks that the EvolutionConfig is valid.
func (o *EvolutionConfig) Validate() error {
	if strings.TrimSpace(o.TargetModel) == "" {
		return fmt.Errorf("targetmodel is required")
	}
	return nil
}

// EvolutionResult represents evolutionresult data.
type EvolutionResult struct {
	Generations int
	FinalPopulation []TokenGenome
	FitnessHistory []float64
	BestGenome TokenGenome
	TimeMs int64
}

// FitnessTest represents fitnesstest data.
type FitnessTest struct {
	Model string
	Response string
	Score float64
	Genome TokenGenome
	TestType string
}

// Validate checks that the FitnessTest is valid.
func (o *FitnessTest) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

// PopulationStats represents populationstats data.
type PopulationStats struct {
	Generation int
	AvgFitness float64
	BestFitness float64
	PopulationSize int
	Diversity float64
}

