// Package templates provides template resolution
package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Resolver expands templates into actual context
type Resolver struct {
	rootPath string
}

// ResolvedContext contains the expanded context
type ResolvedContext struct {
	Files        []ContextFile
	GitInfo      *GitContext
	Instructions string
	Variables    map[string]string
	TotalTokens  int
}

// ContextFile represents a file in the context
type ContextFile struct {
	Path       string
	Content    string
	TokenCount int
}

// GitContext contains git information
type GitContext struct {
	Branch        string
	RecentCommits []string
	ChangedFiles  []string
}

// NewResolver creates a new resolver
func NewResolver(rootPath string) *Resolver {
	return &Resolver{rootPath: rootPath}
}

// Resolve expands a template with variables
func (r *Resolver) Resolve(template *ContextTemplate, vars map[string]string) (*ResolvedContext, error) {
	// Validate required variables
	for _, v := range template.Spec.Variables {
		if v.Required {
			if _, ok := vars[v.Name]; !ok || vars[v.Name] == "" {
				return nil, fmt.Errorf("required variable %s not provided", v.Name)
			}
		}
	}

	// Set defaults for missing optional variables
	for _, v := range template.Spec.Variables {
		if _, ok := vars[v.Name]; !ok {
			vars[v.Name] = v.Default
		}
	}

	result := &ResolvedContext{
		Variables: vars,
	}

	// Resolve files
	files, err := r.resolveFiles(template.Spec.Files)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve files: %w", err)
	}
	result.Files = files

	// Resolve git context
	if gitCtx, err := r.resolveGitContext(template.Spec.GitContext); err == nil {
		result.GitInfo = gitCtx
	}

	// Substitute variables in instructions
	result.Instructions = r.substituteVars(template.Spec.Instructions, vars)

	return result, nil
}

// resolveFiles resolves file patterns
func (r *Resolver) resolveFiles(spec FileSpec) ([]ContextFile, error) {
	var files []ContextFile
	seen := make(map[string]bool)
	cleanRoot := filepath.Clean(r.rootPath)

	isWithinRoot := func(path string) bool {
		cleanPath := filepath.Clean(path)
		rel, err := filepath.Rel(cleanRoot, cleanPath)
		if err != nil {
			return false
		}
		return !strings.HasPrefix(rel, "..") && rel != ".."
	}

	for _, pattern := range spec.Include {
		var matches []string

		if strings.HasPrefix(pattern, "**/") {
			suffix := strings.TrimPrefix(pattern, "**/")
			filepath.WalkDir(r.rootPath, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				matched, _ := filepath.Match(suffix, filepath.Base(path))
				if matched {
					matches = append(matches, path)
				}
				return nil
			})
		} else if strings.Contains(pattern, "**") {
			parts := strings.SplitN(pattern, "**", 2)
			suffix := strings.TrimPrefix(parts[1], "/")
			filepath.WalkDir(r.rootPath, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if suffix == "" || suffix == "*" {
					matches = append(matches, path)
				} else {
					matched, _ := filepath.Match(suffix, filepath.Base(path))
					if matched {
						matches = append(matches, path)
					}
				}
				return nil
			})
		} else {
			var err error
			matches, err = filepath.Glob(filepath.Join(r.rootPath, pattern))
			if err != nil {
				continue
			}
		}

		for _, match := range matches {
			if !isWithinRoot(match) {
				continue
			}

			if r.isExcluded(match, spec.Exclude) {
				continue
			}

			relPath, _ := filepath.Rel(r.rootPath, match)
			if seen[relPath] {
				continue
			}
			seen[relPath] = true

			content, err := os.ReadFile(match)
			if err != nil {
				continue
			}

			files = append(files, ContextFile{
				Path:    relPath,
				Content: string(content),
			})
		}
	}

	return files, nil
}

// isExcluded checks if a file matches exclude patterns
func (r *Resolver) isExcluded(path string, excludes []string) bool {
	for _, pattern := range excludes {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// resolveGitContext resolves git context
func (r *Resolver) resolveGitContext(spec GitContextSpec) (*GitContext, error) {
	repo, err := git.PlainOpen(r.rootPath)
	if err != nil {
		return nil, err
	}

	ctx := &GitContext{}

	// Get current branch
	head, err := repo.Head()
	if err == nil {
		ctx.Branch = head.Name().Short()
	}

	// Get recent commits
	if spec.RecentCommits.Enabled {
		log, _ := repo.Log(&git.LogOptions{})
		if log != nil {
			count := 0
			log.ForEach(func(c *object.Commit) error {
				if count >= spec.RecentCommits.Count {
					return fmt.Errorf("done")
				}
				ctx.RecentCommits = append(ctx.RecentCommits, c.Message)
				count++
				return nil
			})
		}
	}

	return ctx, nil
}

// substituteVars replaces variables in text
func (r *Resolver) substituteVars(text string, vars map[string]string) string {
	result := text
	for name, value := range vars {
		placeholder := "{{" + name + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// FormatContext formats resolved context for LLM consumption
func (r *ResolvedContext) FormatContext() string {
	var sb strings.Builder

	// Add instructions
	if r.Instructions != "" {
		sb.WriteString("## Instructions\n\n")
		sb.WriteString(r.Instructions)
		sb.WriteString("\n\n")
	}

	// Add files
	if len(r.Files) > 0 {
		sb.WriteString("## Files\n\n")
		for _, f := range r.Files {
			sb.WriteString(fmt.Sprintf("### %s\n\n```\n%s\n```\n\n", f.Path, f.Content))
		}
	}

	// Add git context
	if r.GitInfo != nil {
		sb.WriteString("## Git Context\n\n")
		sb.WriteString(fmt.Sprintf("Branch: %s\n\n", r.GitInfo.Branch))

		if len(r.GitInfo.RecentCommits) > 0 {
			sb.WriteString("Recent commits:\n")
			for _, msg := range r.GitInfo.RecentCommits {
				sb.WriteString(fmt.Sprintf("- %s\n", msg))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
