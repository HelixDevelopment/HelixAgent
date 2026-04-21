// Package types defines Go types for the LEAKHUB library.
// Go library for LEAKHUB providing prompt leak detection, extraction, and archival. Identifies potential system prompt leaks in AI model responses and maintains a searchable archive of known leaks.
package types

import (
	"fmt"
	"strings"
)

// LeakEntry represents leakentry data.
type LeakEntry struct {
	Model string
	ID string
	Confidence float64
	Date string
	LeakedContent string
	LeakType string
	Source string
	Tags []string
}

// Validate checks that the LeakEntry is valid.
func (o *LeakEntry) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

// DetectionOptions represents detectionoptions data.
type DetectionOptions struct {
	Response string
	Model string
	Sensitivity float64
	KnownSignatures []string
}

// Validate checks that the DetectionOptions is valid.
func (o *DetectionOptions) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *DetectionOptions) Defaults() {
	if o.Sensitivity == 0 {
		o.Sensitivity = 0.5
	}
}

// DetectionResult represents detectionresult data.
type DetectionResult struct {
	Leaked bool
	Confidence float64
	Matches []LeakMatch
	SuggestedMitigation string
}

// LeakMatch represents leakmatch data.
type LeakMatch struct {
	Pattern string
	Position int
	Confidence float64
	MatchedText string
}

// ArchiveStats represents archivestats data.
type ArchiveStats struct {
	ByType map[string]int
	ByModel map[string]int
	TotalLeaks int
	AvgConfidence float64
}

