// Package types defines Go types for the L1B3RT4S library.
// Go library for the L1B3RT4S collection of jailbreak and prompt injection techniques. Provides structured access to prompt patterns targeting various AI models with safety testing capabilities.
package types

import (
	"fmt"
	"strings"
)

// JailbreakPrompt represents jailbreakprompt data.
type JailbreakPrompt struct {
	DateAdded string
	Category string
	ID string
	TargetModels []string
	Description string
	Effectiveness float64
	PromptTemplate string
	Source string
	Tags []string
	Name string
}

// Validate checks that the JailbreakPrompt is valid.
func (o *JailbreakPrompt) Validate() error {
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// PromptTemplate represents prompttemplate data.
type PromptTemplate struct {
	SuccessRate float64
	Category string
	Template string
	ID string
	Variables []string
	TargetModel string
	Name string
}

// Validate checks that the PromptTemplate is valid.
func (o *PromptTemplate) Validate() error {
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(o.TargetModel) == "" {
		return fmt.Errorf("targetmodel is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// SearchOptions represents searchoptions data.
type SearchOptions struct {
	Limit int
	Query string
	Categories []string
	MinEffectiveness float64
	Tags []string
	TargetModels []string
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

// TestResult represents testresult data.
type TestResult struct {
	Model string
	Response string
	TestedAt string
	PromptID string
	DurationMs int64
	Success bool
}

// Validate checks that the TestResult is valid.
func (o *TestResult) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

// SafetyCheckOptions represents safetycheckoptions data.
type SafetyCheckOptions struct {
	Model string
	Prompt string
	CheckTypes []string
}

// Validate checks that the SafetyCheckOptions is valid.
func (o *SafetyCheckOptions) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(o.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	return nil
}

