// Package types defines Go types for the Gandalf Solutions library.
// Go library providing structured access to solutions for Lakera's Gandalf prompt hacking game (levels 1-8 + adventures), including system prompt leak techniques, emoji encoding solutions, reverse Gandalf strategies, and extracted system prompts from Gandalf the White.
package types

import (
	"fmt"
	"strings"
)

// LevelSolution represents levelsolution data.
type LevelSolution struct {
	Level int
	Techniques []string
	Category string
	Description string
	Difficulty string
	SystemPromptLeak string
	Solutions []string
	Name string
}

// Validate checks that the LevelSolution is valid.
func (o *LevelSolution) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// AdventureSolution represents adventuresolution data.
type AdventureSolution struct {
	Description string
	Adventure string
	Difficulty string
	Solutions []string
	Name string
}

// Validate checks that the AdventureSolution is valid.
func (o *AdventureSolution) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// PromptLeak represents promptleak data.
type PromptLeak struct {
	Model string
	Date string
	LeakedContent string
	ID string
	ExtractionMethod string
	Source string
	Confidence float64
}

// Validate checks that the PromptLeak is valid.
func (o *PromptLeak) Validate() error {
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
	Query string
	Techniques []string
	Difficulty string
	Levels []int
	Limit int
	Categories []string
}

// Validate checks that the SearchOptions is valid.
func (o *SearchOptions) Validate() error {
	if strings.TrimSpace(o.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if o.Limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *SearchOptions) Defaults() {
	if o.Limit == 0 { o.Limit = 50 }
}

// ArchiveStats represents archivestats data.
type ArchiveStats struct {
	Techniques []string
	TotalAdventures int
	TotalLeaks int
	Categories []string
	TotalLevels int
}

