// Package types defines Go types for the CL4R1T4S library.
// Go library for accessing and searching the CL4R1T4S database of leaked and extracted AI system prompts from major AI companies including OpenAI, Google, Anthropic, xAI, Perplexity, Cursor, and Devin.
package types

import (
	"fmt"
	"strings"
)

// SystemPrompt represents systemprompt data.
type SystemPrompt struct {
	Model string
	ExtractedAt string
	Verified bool
	Category string
	Company string
	ID string
	Confidence float64
	PromptText string
	Product string
	Source string
	Tags []string
}

// Validate checks that the SystemPrompt is valid.
func (o *SystemPrompt) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(o.Company) == "" {
		return fmt.Errorf("company is required")
	}
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(o.Product) == "" {
		return fmt.Errorf("product is required")
	}
	return nil
}

// PromptEntry represents promptentry data.
type PromptEntry struct {
	Category string
	Company string
	ID string
	Prompt string
	Confidence float64
	Product string
	Date string
	Source string
	Tags []string
}

// Validate checks that the PromptEntry is valid.
func (o *PromptEntry) Validate() error {
	if strings.TrimSpace(o.Company) == "" {
		return fmt.Errorf("company is required")
	}
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(o.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	if strings.TrimSpace(o.Product) == "" {
		return fmt.Errorf("product is required")
	}
	return nil
}

// SearchOptions represents searchoptions data.
type SearchOptions struct {
	Limit int
	Offset int
	Categories []string
	Query string
	Tags []string
	VerifiedOnly bool
	Companies []string
	MinConfidence float64
}

// Validate checks that the SearchOptions is valid.
func (o *SearchOptions) Validate() error {
	if o.Limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}
	if strings.TrimSpace(o.Query) == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *SearchOptions) Defaults() {
	if o.Limit == 0 { o.Limit = 50 }
}

// ArchiveStats represents archivestats data.
type ArchiveStats struct {
	TotalPrompts int
	Companies []string
	Categories []string
	LastUpdated string
	AvgConfidence float64
}

// ComparisonResult represents a side-by-side comparison across prompts.
type ComparisonResult struct {
	Prompts       []PromptEntry
	Similarities  []string
	Differences   []string
	OverallScore  float64
}

// ExportOptions represents options for exporting archive data.
type ExportOptions struct {
	Companies    []string
	Categories   []string
	IncludeStats bool
	PrettyPrint  bool
}

// TrendOptions represents options for trend analysis.
type TrendOptions struct {
	StartDate  string
	EndDate    string
	Granularity string
	Companies  []string
}

// Validate checks that the TrendOptions is valid.
func (o *TrendOptions) Validate() error {
	return nil
}

// Defaults applies default values for unset fields.
func (o *TrendOptions) Defaults() {
	if strings.TrimSpace(o.Granularity) == "" {
		o.Granularity = "monthly"
	}
}

// TrendAnalysis represents the result of trend analysis.
type TrendAnalysis struct {
	Points       []TrendPoint
	Summary      string
	TotalChanges int
}

// TrendPoint represents a single trend data point.
type TrendPoint struct {
	Date        string
	Count       int
	TopCategory string
}


