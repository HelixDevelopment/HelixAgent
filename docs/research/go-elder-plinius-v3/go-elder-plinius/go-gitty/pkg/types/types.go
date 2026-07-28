// Package types defines Go types for the Gitty library.
// Go library for Gitty providing AI-powered Git assistance including commit message generation, code review, PR description writing, branch naming suggestions, and repository analysis.
package types

import (
	"fmt"
	"strings"
)

// CommitOptions represents commitoptions data.
type CommitOptions struct {
	Files     []string
	Language  string
	Context   string
	MaxLength int
	Style     string
	Diff      string
}

// Validate checks that the CommitOptions is valid.
func (o *CommitOptions) Validate() error {
	if strings.TrimSpace(o.Diff) == "" {
		return fmt.Errorf("diff is required")
	}
	return nil
}

// CommitMessage represents commitmessage data.
type CommitMessage struct {
	Subject  string
	Emoji    string
	Body     string
	Breaking bool
	Scope    string
	Type     string
}

// ReviewOptions represents reviewoptions data.
type ReviewOptions struct {
	Files      []string
	Language   string
	FocusAreas []string
	Severity   string
	Diff       string
}

// Validate checks that the ReviewOptions is valid.
func (o *ReviewOptions) Validate() error {
	if strings.TrimSpace(o.Diff) == "" {
		return fmt.Errorf("diff is required")
	}
	return nil
}

// ReviewResult represents reviewresult data.
type ReviewResult struct {
	Summary     string
	Issues      []Issue
	Praises     []string
	Suggestions []Suggestion
	RiskLevel   string
}

// Issue represents issue data.
type Issue struct {
	Line       int
	Message    string
	Severity   string
	File       string
	Suggestion string
}

// Suggestion represents suggestion data.
type Suggestion struct {
	CodeExample string
	Confidence  float64
	Description string
	Category    string
}

// Validate checks that the Suggestion is valid.
func (o *Suggestion) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// RepoStats represents repostats data.
type RepoStats struct {
	HealthScore    float64
	Languages      map[string]float64
	TotalCommits   int
	Contributors   int
	RecentActivity string
}

// Defaults applies default values for unset fields.
func (o *CommitOptions) Defaults() {}

// Defaults applies default values for unset fields.
func (o *ReviewOptions) Defaults() {}
