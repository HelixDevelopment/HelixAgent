package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResolver(t *testing.T) {
	resolver := NewResolver("/test/path")
	require.NotNil(t, resolver)
	assert.Equal(t, "/test/path", resolver.rootPath)
}

func TestResolver_Resolve(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"README.md":   "# Test Project",
		"main.go":     "package main",
		"config.yaml": "key: value",
	}

	for name, content := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	resolver := NewResolver(tempDir)

	tests := []struct {
		name        string
		template    ContextTemplate
		vars        map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid resolve with no variables",
			template: ContextTemplate{
				Metadata: TemplateMetadata{
					ID:   "test",
					Name: "Test",
				},
				Spec: TemplateSpec{
					Files: FileSpec{
						Include: []string{"*.md", "*.go"},
					},
					Instructions: "Test instructions",
				},
			},
			vars:    map[string]string{},
			wantErr: false,
		},
		{
			name: "missing required variable",
			template: ContextTemplate{
				Metadata: TemplateMetadata{
					ID:   "test",
					Name: "Test",
				},
				Spec: TemplateSpec{
					Variables: []VariableDef{
						{Name: "required_var", Required: true},
					},
				},
			},
			vars:        map[string]string{},
			wantErr:     true,
			errContains: "required variable",
		},
		{
			name: "variable substitution",
			template: ContextTemplate{
				Metadata: TemplateMetadata{
					ID:   "test",
					Name: "Test",
				},
				Spec: TemplateSpec{
					Instructions: "Hello {{name}}!",
					Variables: []VariableDef{
						{Name: "name", Required: true},
					},
				},
			},
			vars:    map[string]string{"name": "World"},
			wantErr: false,
		},
		{
			name: "default variable value",
			template: ContextTemplate{
				Metadata: TemplateMetadata{
					ID:   "test",
					Name: "Test",
				},
				Spec: TemplateSpec{
					Instructions: "Hello {{name}}!",
					Variables: []VariableDef{
						{Name: "name", Required: false, Default: "Default"},
					},
				},
			},
			vars:    map[string]string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := resolver.Resolve(&tt.template, tt.vars)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, ctx)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ctx)
				assert.NotNil(t, ctx.Variables)

				if tt.name == "variable substitution" {
					assert.Equal(t, "Hello World!", ctx.Instructions)
				}
				if tt.name == "default variable value" {
					assert.Equal(t, "Hello Default!", ctx.Instructions)
				}
			}
		})
	}
}

func TestResolver_ResolveFiles(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"README.md":        "# Test Project",
		"main.go":          "package main",
		"utils.go":         "package utils",
		"config.yaml":      "key: value",
		"docs/guide.md":    "# Guide",
		"vendor/vendor.go": "package vendor",
	}

	for name, content := range testFiles {
		path := filepath.Join(tempDir, name)
		err := os.MkdirAll(filepath.Dir(path), 0755)
		require.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	resolver := NewResolver(tempDir)

	tests := []struct {
		name          string
		include       []string
		exclude       []string
		wantFileCount int
		wantFiles     []string
	}{
		{
			name:          "include all go files",
			include:       []string{"*.go"},
			exclude:       []string{},
			wantFileCount: 2,
			wantFiles:     []string{"main.go", "utils.go"},
		},
		{
			name:          "include markdown files",
			include:       []string{"*.md"},
			exclude:       []string{},
			wantFileCount: 1,
			wantFiles:     []string{"README.md"},
		},
		{
			name:          "include with exclude pattern",
			include:       []string{"*.go"},
			exclude:       []string{"utils*"},
			wantFileCount: 1,
			wantFiles:     []string{"main.go"},
		},
		{
			name:          "non-existent pattern",
			include:       []string{"*.nonexistent"},
			exclude:       []string{},
			wantFileCount: 0,
		},
		{
			name:          "recursive pattern",
			include:       []string{"**/*.md"},
			exclude:       []string{},
			wantFileCount: 2,
			wantFiles:     []string{"README.md", "docs/guide.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := resolver.resolveFiles(FileSpec{
				Include: tt.include,
				Exclude: tt.exclude,
			})
			assert.NoError(t, err)
			assert.Len(t, files, tt.wantFileCount)

			for _, wantFile := range tt.wantFiles {
				found := false
				for _, file := range files {
					if file.Path == wantFile {
						found = true
						break
					}
				}
				assert.True(t, found, "expected file %s not found", wantFile)
			}
		})
	}
}

func TestResolver_IsExcluded(t *testing.T) {
	resolver := NewResolver("/test")

	tests := []struct {
		name     string
		path     string
		excludes []string
		want     bool
	}{
		{
			name:     "basename match",
			path:     "/test/vendor.go",
			excludes: []string{"vendor*"},
			want:     true,
		},
		{
			name:     "path contains pattern",
			path:     "/test/node_modules/package/file.go",
			excludes: []string{"node_modules"},
			want:     true,
		},
		{
			name:     "no match",
			path:     "/test/main.go",
			excludes: []string{"vendor*", "node_modules"},
			want:     false,
		},
		{
			name:     "empty excludes",
			path:     "/test/main.go",
			excludes: []string{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolver.isExcluded(tt.path, tt.excludes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolver_SubstituteVars(t *testing.T) {
	resolver := NewResolver("/test")

	tests := []struct {
		name string
		text string
		vars map[string]string
		want string
	}{
		{
			name: "single variable",
			text: "Hello {{name}}!",
			vars: map[string]string{"name": "World"},
			want: "Hello World!",
		},
		{
			name: "multiple variables",
			text: "{{greeting}} {{name}}!",
			vars: map[string]string{"greeting": "Hello", "name": "World"},
			want: "Hello World!",
		},
		{
			name: "no variables in text",
			text: "Hello World!",
			vars: map[string]string{"name": "Test"},
			want: "Hello World!",
		},
		{
			name: "variable not in vars",
			text: "Hello {{name}}!",
			vars: map[string]string{"other": "value"},
			want: "Hello {{name}}!",
		},
		{
			name: "empty text",
			text: "",
			vars: map[string]string{"name": "World"},
			want: "",
		},
		{
			name: "empty vars",
			text: "Hello {{name}}!",
			vars: map[string]string{},
			want: "Hello {{name}}!",
		},
		{
			name: "multiple occurrences",
			text: "{{name}} says hello to {{name}}",
			vars: map[string]string{"name": "Alice"},
			want: "Alice says hello to Alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolver.substituteVars(tt.text, tt.vars)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolvedContext_FormatContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  ResolvedContext
		want string
	}{
		{
			name: "with instructions only",
			ctx: ResolvedContext{
				Instructions: "Test instructions",
			},
			want: "## Instructions\n\nTest instructions\n\n",
		},
		{
			name: "with files",
			ctx: ResolvedContext{
				Instructions: "Test",
				Files: []ContextFile{
					{Path: "test.go", Content: "package main"},
				},
			},
			want: "## Instructions\n\nTest\n\n## Files\n\n### test.go\n\n```\npackage main\n```\n\n",
		},
		{
			name: "with git context",
			ctx: ResolvedContext{
				Instructions: "Test",
				GitInfo: &GitContext{
					Branch:        "main",
					RecentCommits: []string{"commit1", "commit2"},
				},
			},
			want: "## Instructions\n\nTest\n\n## Git Context\n\nBranch: main\n\nRecent commits:\n- commit1\n- commit2\n\n",
		},
		{
			name: "empty context",
			ctx:  ResolvedContext{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.FormatContext()
			assert.Equal(t, tt.want, got)
		})
	}
}

// ContextFile Tests

func TestContextFile_Struct(t *testing.T) {
	file := ContextFile{
		Path:       "test.go",
		Content:    "package main",
		TokenCount: 100,
	}

	assert.Equal(t, "test.go", file.Path)
	assert.Equal(t, "package main", file.Content)
	assert.Equal(t, 100, file.TokenCount)
}

// GitContext Tests

func TestGitContext_Struct(t *testing.T) {
	git := GitContext{
		Branch:        "main",
		RecentCommits: []string{"commit1", "commit2"},
		ChangedFiles:  []string{"file1.go", "file2.go"},
	}

	assert.Equal(t, "main", git.Branch)
	assert.Len(t, git.RecentCommits, 2)
	assert.Len(t, git.ChangedFiles, 2)
}

// TemplateSpec Tests

func TestTemplateSpec_Struct(t *testing.T) {
	spec := TemplateSpec{
		Files: FileSpec{
			Include: []string{"*.go"},
			Exclude: []string{"*_test.go"},
		},
		GitContext: GitContextSpec{
			BranchDiff: BranchDiffSpec{
				Enabled:  true,
				Base:     "main",
				MaxFiles: 50,
			},
		},
		Documentation: DocumentationSpec{
			Enabled: true,
			Sources: []DocumentationSource{
				{Type: "mcp", Server: "docs", Query: "test"},
			},
		},
		Instructions: "Test instructions",
		Variables: []VariableDef{
			{Name: "var1", Required: true},
		},
		Prompts: []PromptDef{
			{Name: "prompt1", Template: "Template 1"},
		},
	}

	assert.NotNil(t, spec.Files.Include)
	assert.NotNil(t, spec.Files.Exclude)
	assert.True(t, spec.GitContext.BranchDiff.Enabled)
	assert.True(t, spec.Documentation.Enabled)
	assert.NotEmpty(t, spec.Instructions)
	assert.Len(t, spec.Variables, 1)
	assert.Len(t, spec.Prompts, 1)
}

// VariableDef Tests

func TestVariableDef_Struct(t *testing.T) {
	v := VariableDef{
		Name:        "test-var",
		Description: "Test variable",
		Required:    true,
		Default:     "default-value",
		Options:     []string{"option1", "option2"},
	}

	assert.Equal(t, "test-var", v.Name)
	assert.Equal(t, "Test variable", v.Description)
	assert.True(t, v.Required)
	assert.Equal(t, "default-value", v.Default)
	assert.Len(t, v.Options, 2)
}

// PromptDef Tests

func TestPromptDef_Struct(t *testing.T) {
	p := PromptDef{
		Name:        "test-prompt",
		Description: "Test prompt",
		Template:    "This is a {{type}} template",
	}

	assert.Equal(t, "test-prompt", p.Name)
	assert.Equal(t, "Test prompt", p.Description)
	assert.Equal(t, "This is a {{type}} template", p.Template)
}

// FileSpec Tests

func TestFileSpec_Struct(t *testing.T) {
	fs := FileSpec{
		Include: []string{"*.go", "*.md"},
		Exclude: []string{"vendor/**", "*_test.go"},
	}

	assert.Len(t, fs.Include, 2)
	assert.Len(t, fs.Exclude, 2)
}

// GitContextSpec Tests

func TestGitContextSpec_Struct(t *testing.T) {
	gcs := GitContextSpec{
		BranchDiff: BranchDiffSpec{
			Enabled:  true,
			Base:     "main",
			MaxFiles: 50,
		},
		RecentCommits: RecentCommitsSpec{
			Enabled: true,
			Count:   10,
		},
		RelatedFiles: RelatedFilesSpec{
			Enabled:  true,
			MaxFiles: 20,
		},
	}

	assert.True(t, gcs.BranchDiff.Enabled)
	assert.Equal(t, "main", gcs.BranchDiff.Base)
	assert.Equal(t, 50, gcs.BranchDiff.MaxFiles)
	assert.Equal(t, 10, gcs.RecentCommits.Count)
	assert.Equal(t, 20, gcs.RelatedFiles.MaxFiles)
}

// DocumentationSpec Tests

func TestDocumentationSpec_Struct(t *testing.T) {
	ds := DocumentationSpec{
		Enabled: true,
		Sources: []DocumentationSource{
			{Type: "mcp", Server: "docs", Query: "api"},
			{Type: "url", Server: "https://example.com/docs", Query: ""},
		},
	}

	assert.True(t, ds.Enabled)
	assert.Len(t, ds.Sources, 2)
}

// DocumentationSource Tests

func TestDocumentationSource_Struct(t *testing.T) {
	ds := DocumentationSource{
		Type:   "mcp",
		Server: "docs-server",
		Query:  "search query",
	}

	assert.Equal(t, "mcp", ds.Type)
	assert.Equal(t, "docs-server", ds.Server)
	assert.Equal(t, "search query", ds.Query)
}

// TemplateMetadata Tests

func TestTemplateMetadata_Struct(t *testing.T) {
	tm := TemplateMetadata{
		ID:          "test-id",
		Name:        "Test Template",
		Description: "A test template",
		Author:      "test-author",
		Version:     "1.0.0",
		Tags:        []string{"go", "test"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	assert.Equal(t, "test-id", tm.ID)
	assert.Equal(t, "Test Template", tm.Name)
	assert.Equal(t, "A test template", tm.Description)
	assert.Equal(t, "test-author", tm.Author)
	assert.Equal(t, "1.0.0", tm.Version)
	assert.Len(t, tm.Tags, 2)
}

// ResolvedContext Tests

func TestResolvedContext_Struct(t *testing.T) {
	rc := ResolvedContext{
		Files: []ContextFile{
			{Path: "file1.go", Content: "content1"},
		},
		GitInfo: &GitContext{
			Branch: "main",
		},
		Instructions: "Test instructions",
		Variables:    map[string]string{"var1": "value1"},
		TotalTokens:  1000,
	}

	assert.Len(t, rc.Files, 1)
	assert.NotNil(t, rc.GitInfo)
	assert.Equal(t, "Test instructions", rc.Instructions)
	assert.Equal(t, "value1", rc.Variables["var1"])
	assert.Equal(t, 1000, rc.TotalTokens)
}

// Helper function for time

// Additional tests for FormatContext with different combinations

func TestResolvedContext_FormatContext_Combinations(t *testing.T) {
	tests := []struct {
		name string
		ctx  ResolvedContext
	}{
		{
			name: "files and git",
			ctx: ResolvedContext{
				Files: []ContextFile{
					{Path: "a.go", Content: "package a"},
					{Path: "b.go", Content: "package b"},
				},
				GitInfo: &GitContext{
					Branch:        "feature-branch",
					RecentCommits: []string{"feat: add feature"},
				},
			},
		},
		{
			name: "empty git info commits",
			ctx: ResolvedContext{
				GitInfo: &GitContext{
					Branch:        "main",
					RecentCommits: []string{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ctx.FormatContext()
			assert.NotNil(t, result)
			// Just verify it doesn't panic
		})
	}
}

func TestResolvedContext_FormatContext_EmptyFiles(t *testing.T) {
	ctx := ResolvedContext{
		Files: []ContextFile{},
	}

	result := ctx.FormatContext()
	assert.NotContains(t, result, "## Files")
}

func TestResolvedContext_FormatContext_NilGitInfo(t *testing.T) {
	ctx := ResolvedContext{
		GitInfo: nil,
	}

	result := ctx.FormatContext()
	assert.NotContains(t, result, "## Git Context")
}

func TestResolver_resolveFiles_WithExcludeDir(t *testing.T) {
	// Create a temporary directory with files in subdirectories
	tempDir := t.TempDir()

	// Create files in regular and vendor directories
	files := map[string]string{
		"src/main.go":   "package main",
		"vendor/lib.go": "package lib",
		"README.md":     "# Test",
	}

	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	resolver := NewResolver(tempDir)

	// Test with exclusion of vendor directory
	resolvedFiles, err := resolver.resolveFiles(FileSpec{
		Include: []string{"**/*.go"},
		Exclude: []string{"vendor"},
	})

	assert.NoError(t, err)
	assert.Len(t, resolvedFiles, 1)
	assert.Equal(t, "src/main.go", resolvedFiles[0].Path)
}

func TestResolver_resolveFiles_NoMatches(t *testing.T) {
	tempDir := t.TempDir()

	// Create only .txt files
	err := os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("text"), 0644)
	require.NoError(t, err)

	resolver := NewResolver(tempDir)

	// Try to match .go files
	files, err := resolver.resolveFiles(FileSpec{
		Include: []string{"*.go"},
	})

	assert.NoError(t, err)
	assert.Empty(t, files)
}

func TestResolver_substituteVars_Complex(t *testing.T) {
	resolver := NewResolver("/test")

	// Test with special characters
	text := "Path: {{path}}, Name: {{name}}"
	vars := map[string]string{
		"path": "/home/user",
		"name": "test-file",
	}

	result := resolver.substituteVars(text, vars)
	assert.Equal(t, "Path: /home/user, Name: test-file", result)
}

func TestResolver_substituteVars_Overlapping(t *testing.T) {
	resolver := NewResolver("/test")

	// Test with overlapping variable names
	text := "{{a}} {{ab}} {{abc}}"
	vars := map[string]string{
		"a":   "1",
		"ab":  "2",
		"abc": "3",
	}

	result := resolver.substituteVars(text, vars)
	// Order of replacement may vary, but all should be replaced
	assert.NotContains(t, result, "{{a}}")
	assert.NotContains(t, result, "{{ab}}")
	assert.NotContains(t, result, "{{abc}}")
}

func TestResolver_resolveGitContext_InvalidPath(t *testing.T) {
	resolver := NewResolver("/non/existent/path")

	gitCtx, err := resolver.resolveGitContext(GitContextSpec{
		RecentCommits: RecentCommitsSpec{Enabled: true, Count: 5},
	})

	assert.Error(t, err)
	assert.Nil(t, gitCtx)
}

func TestResolvedContext_FormatContext_FileEscaping(t *testing.T) {
	ctx := ResolvedContext{
		Files: []ContextFile{
			{Path: "test.go", Content: "```code```"},
		},
	}

	result := ctx.FormatContext()
	// The backticks in content should be handled
	assert.Contains(t, result, "```")
	assert.Contains(t, result, "code")
}

// BranchDiffSpec Tests
func TestBranchDiffSpec_Struct(t *testing.T) {
	spec := BranchDiffSpec{
		Enabled:  true,
		Base:     "main",
		MaxFiles: 100,
	}

	assert.True(t, spec.Enabled)
	assert.Equal(t, "main", spec.Base)
	assert.Equal(t, 100, spec.MaxFiles)
}

// RecentCommitsSpec Tests
func TestRecentCommitsSpec_Struct(t *testing.T) {
	spec := RecentCommitsSpec{
		Enabled: true,
		Count:   20,
	}

	assert.True(t, spec.Enabled)
	assert.Equal(t, 20, spec.Count)
}

// RelatedFilesSpec Tests
func TestRelatedFilesSpec_Struct(t *testing.T) {
	spec := RelatedFilesSpec{
		Enabled:  true,
		MaxFiles: 10,
	}

	assert.True(t, spec.Enabled)
	assert.Equal(t, 10, spec.MaxFiles)
}

// Additional FormatContext tests
func TestResolvedContext_FormatContext_MultipleFiles(t *testing.T) {
	ctx := ResolvedContext{
		Instructions: "Test",
		Files: []ContextFile{
			{Path: "a.go", Content: "package a"},
			{Path: "b.go", Content: "package b"},
			{Path: "c.go", Content: "package c"},
		},
	}

	result := ctx.FormatContext()
	assert.Contains(t, result, "### a.go")
	assert.Contains(t, result, "### b.go")
	assert.Contains(t, result, "### c.go")
	assert.Contains(t, result, "package a")
	assert.Contains(t, result, "package b")
	assert.Contains(t, result, "package c")
}

func TestResolvedContext_FormatContext_NoInstructions(t *testing.T) {
	ctx := ResolvedContext{
		Files: []ContextFile{
			{Path: "test.go", Content: "code"},
		},
	}

	result := ctx.FormatContext()
	assert.NotContains(t, result, "## Instructions")
	assert.Contains(t, result, "## Files")
}

// Test resolver with real git repo (if available)
func TestResolver_resolveGitContext_WithRealRepo(t *testing.T) {
	// Use the current project directory which should have git
	resolver := NewResolver("/run/media/milosvasic/DATA4TB/Projects/HelixAgent")

	gitCtx, err := resolver.resolveGitContext(GitContextSpec{
		RecentCommits: RecentCommitsSpec{Enabled: true, Count: 3},
	})

	// May or may not succeed depending on git availability
	if err == nil {
		assert.NotEmpty(t, gitCtx.Branch)
	}
}

// ContextTemplate tests
func TestContextTemplate_EmptyVariables(t *testing.T) {
	template := ContextTemplate{
		Metadata: TemplateMetadata{
			ID:   "test",
			Name: "Test",
		},
		Spec: TemplateSpec{
			Variables: []VariableDef{},
		},
	}

	v := template.GetVariable("any")
	assert.Nil(t, v)
}

func TestContextTemplate_EmptyPrompts(t *testing.T) {
	template := ContextTemplate{
		Metadata: TemplateMetadata{
			ID:   "test",
			Name: "Test",
		},
		Spec: TemplateSpec{
			Prompts: []PromptDef{},
		},
	}

	p := template.GetPrompt("any")
	assert.Nil(t, p)
}

// Error handling tests
func TestResolver_Resolve_InvalidVariableName(t *testing.T) {
	resolver := NewResolver("/test")

	template := ContextTemplate{
		Metadata: TemplateMetadata{
			ID:   "test",
			Name: "Test",
		},
		Spec: TemplateSpec{
			Variables: []VariableDef{
				{Name: "valid-var", Required: false, Default: "default"},
			},
		},
	}

	// Should not error for optional variable
	ctx, err := resolver.Resolve(&template, map[string]string{})
	assert.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.Equal(t, "default", ctx.Variables["valid-var"])
}

// Test resolveFiles edge cases
func TestResolver_resolveFiles_InvalidPattern(t *testing.T) {
	tempDir := t.TempDir()
	resolver := NewResolver(tempDir)

	// Invalid glob pattern should be handled gracefully
	files, err := resolver.resolveFiles(FileSpec{
		Include: []string{"[invalid"},
	})

	// Should return empty without error
	assert.NoError(t, err)
	assert.Empty(t, files)
}

func TestResolver_resolveFiles_ReadError(t *testing.T) {
	tempDir := t.TempDir()

	// Create a directory that matches but can't be read as file
	err := os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)
	require.NoError(t, err)

	resolver := NewResolver(tempDir)

	// Try to read directory as file (will skip due to read error)
	files, err := resolver.resolveFiles(FileSpec{
		Include: []string{"*"},
	})

	// Should not error, but will have no files (subdirs are skipped)
	assert.NoError(t, err)
	assert.Empty(t, files)
}

// Large content test
func TestContextFile_LargeContent(t *testing.T) {
	// Create a file with large content
	largeContent := make([]byte, 10000)
	for i := range largeContent {
		largeContent[i] = byte('a' + (i % 26))
	}

	file := ContextFile{
		Path:    "large.txt",
		Content: string(largeContent),
	}

	assert.Equal(t, 10000, len(file.Content))
}

// FormatContext performance check
func TestResolvedContext_FormatContext_Large(t *testing.T) {
	// Create context with many files
	files := make([]ContextFile, 100)
	for i := range files {
		files[i] = ContextFile{
			Path:    fmt.Sprintf("file%d.go", i),
			Content: fmt.Sprintf("package file%d", i),
		}
	}

	ctx := ResolvedContext{
		Instructions: "Test with many files",
		Files:        files,
	}

	result := ctx.FormatContext()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "file0.go")
	assert.Contains(t, result, "file99.go")
}

// Test file path edge cases
func TestResolver_resolveFiles_PathTraversal(t *testing.T) {
	tempDir := t.TempDir()

	// Create files
	err := os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("content"), 0644)
	require.NoError(t, err)

	// Try path traversal patterns
	resolver := NewResolver(tempDir)

	files, err := resolver.resolveFiles(FileSpec{
		Include: []string{"../../*"},
	})

	// Should handle gracefully
	assert.NoError(t, err)
	// No files should match as they go outside root
	assert.Empty(t, files)
}
