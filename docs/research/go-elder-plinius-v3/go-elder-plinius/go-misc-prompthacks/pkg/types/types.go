// Package types defines Go types for the Misc-Prompt-Hacks library.
// Go library providing structured access to prompt hacking challenge solutions from games like Lakera's Gandalf, TensorTrust, and other prompt injection benchmarks.
package types

import (
	"fmt"
	"strings"
)

// ChallengeSolution represents challengesolution data.
type ChallengeSolution struct {
	ID string
	SuccessRate float64
	Difficulty string
	Solution string
	Explanation string
	Level string
	Challenge string
	Tags []string
	Platform string
}

// Validate checks that the ChallengeSolution is valid.
func (o *ChallengeSolution) Validate() error {
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

// ChallengeEntry represents challengeentry data.
type ChallengeEntry struct {
	Description string
	Name string
	Solutions []ChallengeSolution
	ID string
	Difficulty string
	Platform string
}

// Validate checks that the ChallengeEntry is valid.
func (o *ChallengeEntry) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

// SearchOptions represents searchoptions data.
type SearchOptions struct {
	Difficulties []string
	Limit int
	Query string
	Platforms []string
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

