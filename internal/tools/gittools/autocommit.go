// Package gittools provides enhanced git operations
// Including auto-commit workflow inspired by Aider and Codex
package gittools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// AutoCommitConfig configures the auto-commit behavior
type AutoCommitConfig struct {
	Enabled             bool
	GenerateMessage     bool
	ConventionalCommits bool
	RequireClean        bool
	Timeout             time.Duration
}

// DefaultAutoCommitConfig returns default configuration
func DefaultAutoCommitConfig() AutoCommitConfig {
	return AutoCommitConfig{
		Enabled:             true,
		GenerateMessage:     true,
		ConventionalCommits: true,
		RequireClean:        false,
		Timeout:             30 * time.Second,
	}
}

// Change represents a file change
type Change struct {
	Path      string
	Operation string // added, modified, deleted, renamed
	Diff      string
}

// Commit represents a git commit
type Commit struct {
	Hash        string
	Message     string
	Timestamp   time.Time
	Files       []string
	Author      string
	Description string // Generated description of changes
}

// AutoCommit handles automatic commit operations
type AutoCommit struct {
	config   AutoCommitConfig
	logger   *logrus.Logger
	basePath string
}

// NewAutoCommit creates a new auto-commit handler
func NewAutoCommit(basePath string, config AutoCommitConfig, logger *logrus.Logger) *AutoCommit {
	if logger == nil {
		logger = logrus.New()
	}
	return &AutoCommit{
		config:   config,
		logger:   logger,
		basePath: basePath,
	}
}

// CommitChanges automatically commits changes with generated message
func (a *AutoCommit) CommitChanges(ctx context.Context, message string) (*Commit, error) {
	if !a.config.Enabled {
		return nil, fmt.Errorf("auto-commit is disabled")
	}

	// Apply timeout
	if a.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.config.Timeout)
		defer cancel()
	}

	// Check if we're in a git repo
	if !a.IsGitRepo() {
		return nil, fmt.Errorf("not a git repository")
	}

	// Get changes
	changes, err := a.GetChanges()
	if err != nil {
		return nil, fmt.Errorf("get changes: %w", err)
	}

	if len(changes) == 0 {
		return nil, fmt.Errorf("no changes to commit")
	}

	// Stage all changes
	if err := a.StageAll(ctx); err != nil {
		return nil, fmt.Errorf("stage changes: %w", err)
	}

	// Generate commit message if not provided
	commitMsg := message
	if commitMsg == "" && a.config.GenerateMessage {
		commitMsg = a.GenerateCommitMessage(changes)
	}
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("chore: update %d file(s)", len(changes))
	}

	// Commit
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", commitMsg)
	cmd.Dir = a.basePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("commit failed: %w\n%s", err, output)
	}

	// Get commit hash
	hash, err := a.GetLastCommitHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("get commit hash: %w", err)
	}

	commit := &Commit{
		Hash:        hash,
		Message:     commitMsg,
		Timestamp:   time.Now(),
		Files:       a.changesToPaths(changes),
		Author:      a.getAuthor(ctx),
		Description: a.generateDescription(changes),
	}

	a.logger.WithFields(logrus.Fields{
		"hash":    hash,
		"files":   len(changes),
		"message": truncate(commitMsg, 50),
	}).Info("Auto-commit completed")

	return commit, nil
}

// GetChanges returns all uncommitted changes
func (a *AutoCommit) GetChanges() ([]Change, error) {
	cmd := exec.Command("git", "status", "--porcelain", "-u")
	cmd.Dir = a.basePath
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var changes []Change
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		status := strings.TrimSpace(line[:2])
		path := strings.TrimSpace(line[3:])

		var operation string
		switch status {
		case "A", "??":
			operation = "added"
		case "M", " M":
			operation = "modified"
		case "D", " D":
			operation = "deleted"
		case "R":
			operation = "renamed"
		default:
			operation = "modified"
		}

		changes = append(changes, Change{
			Path:      path,
			Operation: operation,
		})
	}

	return changes, nil
}

// GetDiff returns the diff for a file
func (a *AutoCommit) GetDiff(path string) (string, error) {
	cmd := exec.Command("git", "diff", path)
	cmd.Dir = a.basePath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// StageAll stages all changes
func (a *AutoCommit) StageAll(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "add", "-A")
	cmd.Dir = a.basePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %w\n%s", err, output)
	}
	return nil
}

// Stage stages specific files
func (a *AutoCommit) Stage(ctx context.Context, paths []string) error {
	args := append([]string{"add"}, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = a.basePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %w\n%s", err, output)
	}
	return nil
}

// GenerateCommitMessage generates a commit message from changes
func (a *AutoCommit) GenerateCommitMessage(changes []Change) string {
	if len(changes) == 0 {
		return ""
	}

	// Analyze changes
	typeCount := make(map[string]int)
	extCount := make(map[string]int)

	for _, change := range changes {
		typeCount[change.Operation]++
		ext := filepath.Ext(change.Path)
		if ext != "" {
			extCount[ext]++
		}
	}

	// Determine scope
	scope := a.determineScope(changes)

	// Determine type
	changeType := a.determineChangeType(typeCount)

	// Build message
	var msg strings.Builder

	if a.config.ConventionalCommits {
		msg.WriteString(changeType)
		if scope != "" {
			msg.WriteString("(")
			msg.WriteString(scope)
			msg.WriteString(")")
		}
		msg.WriteString(": ")
	}

	// Generate description
	msg.WriteString(a.generateDescription(changes))

	return msg.String()
}

// determineScope analyzes changes to determine the scope
func (a *AutoCommit) determineScope(changes []Change) string {
	// Find common directory
	if len(changes) == 0 {
		return ""
	}

	dirs := make(map[string]int)
	for _, c := range changes {
		dir := filepath.Dir(c.Path)
		if dir == "." {
			dir = "root"
		}
		dirs[dir]++
	}

	// Find most common directory
	var maxCount int
	var commonDir string
	for dir, count := range dirs {
		if count > maxCount {
			maxCount = count
			commonDir = dir
		}
	}

	// Return first component of common directory
	if commonDir != "" && commonDir != "root" {
		parts := strings.Split(commonDir, string(filepath.Separator))
		if len(parts) > 0 {
			return parts[0]
		}
	}

	return ""
}

// determineChangeType determines the type of change based on operations
func (a *AutoCommit) determineChangeType(typeCount map[string]int) string {
	// Check for dominant operation
	if typeCount["added"] > 0 && typeCount["modified"] == 0 && typeCount["deleted"] == 0 {
		return "feat"
	}
	if typeCount["deleted"] > 0 && typeCount["modified"] == 0 && typeCount["added"] == 0 {
		return "remove"
	}

	// Default to chore for mixed changes
	if typeCount["modified"] > typeCount["added"] {
		return "fix"
	}

	return "chore"
}

// generateDescription generates a human-readable description of changes
func (a *AutoCommit) generateDescription(changes []Change) string {
	if len(changes) == 1 {
		change := changes[0]
		filename := filepath.Base(change.Path)

		switch change.Operation {
		case "added":
			return fmt.Sprintf("add %s", filename)
		case "deleted":
			return fmt.Sprintf("remove %s", filename)
		case "modified":
			return fmt.Sprintf("update %s", filename)
		default:
			return fmt.Sprintf("change %s", filename)
		}
	}

	// Multiple files
	var ops []string
	typeCount := make(map[string]int)

	for _, change := range changes {
		typeCount[change.Operation]++
	}

	if typeCount["added"] > 0 {
		ops = append(ops, fmt.Sprintf("add %d file(s)", typeCount["added"]))
	}
	if typeCount["modified"] > 0 {
		ops = append(ops, fmt.Sprintf("update %d file(s)", typeCount["modified"]))
	}
	if typeCount["deleted"] > 0 {
		ops = append(ops, fmt.Sprintf("remove %d file(s)", typeCount["deleted"]))
	}

	return strings.Join(ops, ", ")
}

// IsGitRepo checks if the base path is a git repository
func (a *AutoCommit) IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = a.basePath
	err := cmd.Run()
	return err == nil
}

// GetLastCommitHash returns the hash of the last commit
func (a *AutoCommit) GetLastCommitHash(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = a.basePath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getAuthor returns the git author
func (a *AutoCommit) getAuthor(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "git", "config", "user.name")
	cmd.Dir = a.basePath
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// changesToPaths extracts paths from changes
func (a *AutoCommit) changesToPaths(changes []Change) []string {
	paths := make([]string, len(changes))
	for i, c := range changes {
		paths[i] = c.Path
	}
	return paths
}

// Push pushes commits to remote
func (a *AutoCommit) Push(ctx context.Context, remote, branch string) error {
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		branch = "main"
	}

	cmd := exec.CommandContext(ctx, "git", "push", remote, branch)
	cmd.Dir = a.basePath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("push failed: %w\n%s", err, stderr.String())
	}

	a.logger.WithFields(logrus.Fields{
		"remote": remote,
		"branch": branch,
	}).Info("Push completed")

	return nil
}

// Pull pulls changes from remote
func (a *AutoCommit) Pull(ctx context.Context, remote, branch string) error {
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		branch = "main"
	}

	cmd := exec.CommandContext(ctx, "git", "pull", remote, branch)
	cmd.Dir = a.basePath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pull failed: %w\n%s", err, stderr.String())
	}

	return nil
}

// CreateBranch creates a new branch
func (a *AutoCommit) CreateBranch(ctx context.Context, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", branchName)
	cmd.Dir = a.basePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create branch failed: %w\n%s", err, output)
	}

	return nil
}

// SwitchBranch switches to a branch
func (a *AutoCommit) SwitchBranch(ctx context.Context, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", branchName)
	cmd.Dir = a.basePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("switch branch failed: %w\n%s", err, output)
	}

	return nil
}

// GetCurrentBranch returns the current branch name
func (a *AutoCommit) GetCurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	cmd.Dir = a.basePath

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// HasUncommittedChanges checks if there are uncommitted changes
func (a *AutoCommit) HasUncommittedChanges() bool {
	changes, err := a.GetChanges()
	if err != nil {
		return false
	}
	return len(changes) > 0
}

// Stash stashes current changes
func (a *AutoCommit) Stash(ctx context.Context, message string) error {
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = a.basePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stash failed: %w\n%s", err, output)
	}

	return nil
}

// PopStash pops the latest stash
func (a *AutoCommit) PopStash(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "stash", "pop")
	cmd.Dir = a.basePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stash pop failed: %w\n%s", err, output)
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
