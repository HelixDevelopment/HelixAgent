// Package types defines Go types for the V3R1T4S library.
// Go library for V3R1T4S providing AI truthfulness verification, fact-checking, hallucination detection, and response consistency analysis across multiple AI models.
package types

import (
	"fmt"
	"strings"
)

// VerifyRequest represents verifyrequest data.
type VerifyRequest struct {
	ReferenceSources []string
	Claim string
	Context string
	CheckType string
	SourceModel string
}

// Validate checks that the VerifyRequest is valid.
func (o *VerifyRequest) Validate() error {
	if strings.TrimSpace(o.Claim) == "" {
		return fmt.Errorf("claim is required")
	}
	return nil
}

// VerifyResult represents verifyresult data.
type VerifyResult struct {
	Evidence []Evidence
	Claim string
	Verdict string
	Contradictions []Contradiction
	Confidence float64
	SuggestedCorrections []string
}

// Validate checks that the VerifyResult is valid.
func (o *VerifyResult) Validate() error {
	if strings.TrimSpace(o.Claim) == "" {
		return fmt.Errorf("claim is required")
	}
	return nil
}

// Evidence represents evidence data.
type Evidence struct {
	Excerpt string
	Source string
	Relevance float64
	URL string
}

// Contradiction represents contradiction data.
type Contradiction struct {
	StatementA string
	StatementB string
	Severity float64
	Models []string
}

// ConsistencyCheck represents consistencycheck data.
type ConsistencyCheck struct {
	Responses []string
	ConsistencyScore float64
	Models []string
	DivergentPoints []string
}

// HallucinationResult represents hallucinationresult data.
type HallucinationResult struct {
	SuspiciousSegments []string
	Confidence float64
	FactualChecks []FactCheck
	Hallucinated bool
}

// FactCheck represents factcheck data.
type FactCheck struct {
	Statement string
	Verified bool
	Confidence float64
	Correction string
}

// ModelComparison represents the truthfulness comparison result across models.
type ModelComparison struct {
	Claim            string
	ModelResults     map[string]VerifyResult
	AgreementScore   float64
	MostAccurate     string
	LeastAccurate    string
}

