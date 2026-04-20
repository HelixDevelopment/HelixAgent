// Package types defines Go types for the Bing Prompt Leak library.
// Go library documenting and providing structured access to prompt leak techniques for Microsoft Bing Chat (Copilot), including leetspeak-based extraction, encoding bypasses, and discovered system prompt content.
package types

import (
	"fmt"
	"strings"
)

// LeakEntry represents leakentry data.
type LeakEntry struct {
	ID string
	Confidence float64
	Date string
	Version string
	LeakedContent string
	ExtractionMethod string
	Source string
	Tags []string
}

// Validate checks that the LeakEntry is valid.
func (o *LeakEntry) Validate() error {
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

// TechniqueEntry represents techniqueentry data.
type TechniqueEntry struct {
	ModelTarget string
	Steps []string
	Effectiveness float64
	Description string
	Category string
	Name string
}

// Validate checks that the TechniqueEntry is valid.
func (o *TechniqueEntry) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// SearchOptions represents searchoptions data.
type SearchOptions struct {
	Versions []string
	Limit int
	Query string
	Methods []string
	Tags []string
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

