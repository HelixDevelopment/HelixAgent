// Package types defines Go types for the Grok System Prompt Leak library.
// Go library providing structured access to leaked and extracted system prompts from xAI's Grok models (Twitter/X AI), including personality directives, operational guidelines, tool definitions, and behavior instructions.
package types

import (
	"fmt"
	"strings"
)

// PromptEntry represents promptentry data.
type PromptEntry struct {
	Model string
	ID string
	Confidence float64
	PromptText string
	Date string
	Version string
	Source string
	Tags []string
}

// Validate checks that the PromptEntry is valid.
func (o *PromptEntry) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

// SearchOptions represents searchoptions data.
type SearchOptions struct {
	Tags []string
	Models []string
	Limit int
	Query string
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

