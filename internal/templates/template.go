// Package templates provides context template management
package templates

import (
	"fmt"
	"time"
)

// ContextTemplate represents a context template
type ContextTemplate struct {
	APIVersion string           `json:"api_version" yaml:"api_version"`
	Kind       string           `json:"kind" yaml:"kind"`
	Metadata   TemplateMetadata `json:"metadata" yaml:"metadata"`
	Spec       TemplateSpec     `json:"spec" yaml:"spec"`
}

// TemplateMetadata contains template metadata
type TemplateMetadata struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description" yaml:"description"`
	Author      string    `json:"author" yaml:"author"`
	Version     string    `json:"version" yaml:"version"`
	Tags        []string  `json:"tags" yaml:"tags"`
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"updated_at"`
}

// TemplateSpec contains template specification
type TemplateSpec struct {
	Files         FileSpec          `json:"files" yaml:"files"`
	GitContext    GitContextSpec    `json:"git_context" yaml:"git_context"`
	Documentation DocumentationSpec `json:"documentation" yaml:"documentation"`
	Instructions  string            `json:"instructions" yaml:"instructions"`
	Variables     []VariableDef     `json:"variables" yaml:"variables"`
	Prompts       []PromptDef       `json:"prompts" yaml:"prompts"`
}

// FileSpec defines file patterns
type FileSpec struct {
	Include []string `json:"include" yaml:"include"`
	Exclude []string `json:"exclude" yaml:"exclude"`
}

// GitContextSpec defines git context options
type GitContextSpec struct {
	BranchDiff    BranchDiffSpec    `json:"branch_diff" yaml:"branch_diff"`
	RecentCommits RecentCommitsSpec `json:"recent_commits" yaml:"recent_commits"`
	RelatedFiles  RelatedFilesSpec  `json:"related_files" yaml:"related_files"`
}

// BranchDiffSpec defines branch diff options
type BranchDiffSpec struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Base     string `json:"base" yaml:"base"`
	MaxFiles int    `json:"max_files" yaml:"max_files"`
}

// RecentCommitsSpec defines recent commits options
type RecentCommitsSpec struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
	Count   int  `json:"count" yaml:"count"`
}

// RelatedFilesSpec defines related files options
type RelatedFilesSpec struct {
	Enabled  bool `json:"enabled" yaml:"enabled"`
	MaxFiles int  `json:"max_files" yaml:"max_files"`
}

// DocumentationSpec defines documentation options
type DocumentationSpec struct {
	Enabled bool                  `json:"enabled" yaml:"enabled"`
	Sources []DocumentationSource `json:"sources" yaml:"sources"`
}

// DocumentationSource defines a documentation source
type DocumentationSource struct {
	Type   string `json:"type" yaml:"type"`
	Server string `json:"server" yaml:"server"`
	Query  string `json:"query" yaml:"query"`
}

// VariableDef defines a template variable
type VariableDef struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Required    bool     `json:"required" yaml:"required"`
	Default     string   `json:"default" yaml:"default"`
	Options     []string `json:"options,omitempty" yaml:"options,omitempty"`
}

// PromptDef defines a template prompt
type PromptDef struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Template    string `json:"template" yaml:"template"`
}

// Validate checks if the template is valid
func (t *ContextTemplate) Validate() error {
	if t.APIVersion == "" {
		return fmt.Errorf("api_version is required")
	}
	if t.Metadata.ID == "" {
		return fmt.Errorf("metadata.id is required")
	}
	if t.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	return nil
}

// GetVariable returns a variable definition by name
func (t *ContextTemplate) GetVariable(name string) *VariableDef {
	for i := range t.Spec.Variables {
		if t.Spec.Variables[i].Name == name {
			return &t.Spec.Variables[i]
		}
	}
	return nil
}

// GetPrompt returns a prompt definition by name
func (t *ContextTemplate) GetPrompt(name string) *PromptDef {
	for i := range t.Spec.Prompts {
		if t.Spec.Prompts[i].Name == name {
			return &t.Spec.Prompts[i]
		}
	}
	return nil
}
