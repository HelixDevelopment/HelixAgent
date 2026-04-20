// Package types defines Go types for the OBLITERATUS library.
// Go client for OBLITERATUS -- an advanced toolkit for understanding and removing refusal behaviors from large language models through abliteration (surgically removing refusal representations).
package types

import (
	"fmt"
	"strings"
)

// ObliterateOptions represents obliterateoptions data.
type ObliterateOptions struct {
	ModelName string
	OutputDir string
	Method string
	Device string
	DataType string
	TrustRemoteCode bool
	HarmfulPrompts []string
	HarmlessPrompts []string
	MaxSeqLength int
}

// Validate checks that the ObliterateOptions is valid.
func (o *ObliterateOptions) Validate() error {
	if strings.TrimSpace(o.ModelName) == "" {
		return fmt.Errorf("modelname is required")
	}
	return nil
}

// MethodInfo represents methodinfo data.
type MethodInfo struct {
	Name string
	Label string
	Description string
	NDirections int
	NormPreserve bool
	RefinementPasses int
	Regularization float64
	UseWhitenedSVD bool
}

// Validate checks that the MethodInfo is valid.
func (o *MethodInfo) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// AnalysisResult represents analysisresult data.
type AnalysisResult struct {
	ModelName string
	Architecture string
	NumLayers int
	NumHeads int
	HiddenSize int
	TotalParams int64
	Perplexity float64
	RefusalRate float64
	DetectedAlignment string
}

// Validate checks that the AnalysisResult is valid.
func (o *AnalysisResult) Validate() error {
	if strings.TrimSpace(o.ModelName) == "" {
		return fmt.Errorf("modelname is required")
	}
	return nil
}

// RefusalDirection represents refusaldirection data.
type RefusalDirection struct {
	LayerIndex int
	Norm float64
	Direction []float64
}

// CancelResult represents cancelresult data.
type CancelResult struct {
	Success bool
	Message string
}

