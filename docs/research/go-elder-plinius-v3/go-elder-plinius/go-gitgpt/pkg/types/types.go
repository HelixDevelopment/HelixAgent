// Package types defines Go types for the GitGPT library.
// Go library for GitGPT providing AI-powered Git workflow assistance including commit message generation, code review, branch naming, repository analysis, and changelog generation using LLM intelligence.
package types

import (
	"fmt"
	"strings"
)

// CommitOptions represents commitoptions data.
type CommitOptions struct {
	Files []string
	Language string
	Context string
	MaxLength int
	Style string
	Diff string
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
	Subject string
	Body string
	Breaking bool
	Scope string
	Type string
}

// ReviewOptions represents reviewoptions data.
type ReviewOptions struct {
	FocusAreas []string
	Language string
	Diff string
	Files []string
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
	Issues []Issue
	RiskLevel string
	Suggestions []string
	Summary string
}

// Issue represents issue data.
type Issue struct {
	Line int
	Message string
	Severity string
	File string
	Suggestion string
}

