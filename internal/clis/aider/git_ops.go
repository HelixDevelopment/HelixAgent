// Package aider provides Aider CLI agent integration for HelixAgent.
package aider

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitOps provides git operations for the repository.
// Ported from Aider's git integration
type GitOps struct {
	repoPath string
}

// NewGitOps creates a new GitOps for the repository.
func NewGitOps(repoPath string) *GitOps {
	return &GitOps{repoPath: repoPath}
}

// Status represents git status.
type Status struct {
	Branch         string
	IsDirty        bool
	UntrackedFiles []string
	ModifiedFiles  []string
	StagedFiles    []string
	Ahead          int
	Behind         int
}

// GetStatus gets the current git status.
func (g *GitOps) GetStatus(ctx context.Context) (*Status, error) {
	status := &Status{}

	// Get branch
	branch, err := g.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("get branch: %w", err)
	}
	status.Branch = strings.TrimSpace(branch)

	// Check if dirty
	porcelain, err := g.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("get status: %w", err)
	}

	lines := strings.Split(porcelain, "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		statusCode := line[:2]
		filename := strings.TrimSpace(line[2:])

		if statusCode == "??" {
			status.UntrackedFiles = append(status.UntrackedFiles, filename)
		} else if statusCode[0] != ' ' {
			status.StagedFiles = append(status.StagedFiles, filename)
		} else if statusCode[1] != ' ' {
			status.ModifiedFiles = append(status.ModifiedFiles, filename)
		}
	}

	status.IsDirty = len(status.UntrackedFiles) > 0 ||
		len(status.ModifiedFiles) > 0 ||
		len(status.StagedFiles) > 0

	// Get ahead/behind
	aheadBehind, err := g.runGit(ctx, "rev-list", "--left-right", "--count",
		fmt.Sprintf("HEAD...origin/%s", status.Branch))
	if err == nil {
		parts := strings.Fields(aheadBehind)
		if len(parts) == 2 {
			fmt.Sscanf(parts[0], "%d", &status.Ahead)
			fmt.Sscanf(parts[1], "%d", &status.Behind)
		}
	}

	return status, nil
}

// Diff represents a file diff.
type Diff struct {
	File     string
	OldLines []string
	NewLines []string
	Added    int
	Removed  int
}

// GetDiff gets the diff for modified files.
func (g *GitOps) GetDiff(ctx context.Context, files []string) ([]*Diff, error) {
	args := append([]string{"diff", "--no-color"}, files...)
	diffOutput, err := g.runGit(ctx, args...)
	if err != nil {
		return nil, err
	}

	return g.parseDiff(diffOutput), nil
}

// GetStagedDiff gets the diff for staged files.
func (g *GitOps) GetStagedDiff(ctx context.Context) ([]*Diff, error) {
	diffOutput, err := g.runGit(ctx, "diff", "--cached", "--no-color")
	if err != nil {
		return nil, err
	}

	return g.parseDiff(diffOutput), nil
}

// Commit commits the staged changes.
func (g *GitOps) Commit(ctx context.Context, message string, files []string) error {
	// Add files
	if len(files) > 0 {
		args := append([]string{"add"}, files...)
		if _, err := g.runGit(ctx, args...); err != nil {
			return fmt.Errorf("stage files: %w", err)
		}
	}

	// Commit
	_, err := g.runGit(ctx, "commit", "-m", message)
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// CommitAll stages and commits all changes.
func (g *GitOps) CommitAll(ctx context.Context, message string) error {
	_, err := g.runGit(ctx, "add", "-A")
	if err != nil {
		return fmt.Errorf("stage all: %w", err)
	}

	_, err = g.runGit(ctx, "commit", "-m", message)
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// CreateBranch creates and switches to a new branch.
func (g *GitOps) CreateBranch(ctx context.Context, branchName string) error {
	_, err := g.runGit(ctx, "checkout", "-b", branchName)
	return err
}

// SwitchBranch switches to an existing branch.
func (g *GitOps) SwitchBranch(ctx context.Context, branchName string) error {
	_, err := g.runGit(ctx, "checkout", branchName)
	return err
}

// GetCurrentBranch returns the current branch name.
func (g *GitOps) GetCurrentBranch(ctx context.Context) (string, error) {
	branch, err := g.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(branch), nil
}

// GetRecentCommits returns recent commit history.
func (g *GitOps) GetRecentCommits(ctx context.Context, n int) ([]*Commit, error) {
	format := "%H|%an|%ae|%ad|%s"
	output, err := g.runGit(ctx, "log", fmt.Sprintf("-%d", n),
		"--format="+format, "--date=short")
	if err != nil {
		return nil, err
	}

	var commits []*Commit
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}

		date, _ := time.Parse("2006-01-02", parts[3])
		commits = append(commits, &Commit{
			Hash:    parts[0],
			Author:  parts[1],
			Email:   parts[2],
			Date:    date,
			Message: parts[4],
		})
	}

	return commits, nil
}

// Commit represents a git commit.
type Commit struct {
	Hash    string
	Author  string
	Email   string
	Date    time.Time
	Message string
}

// Push pushes the current branch to origin.
func (g *GitOps) Push(ctx context.Context, force bool) error {
	args := []string{"push"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, "origin", "HEAD")

	_, err := g.runGit(ctx, args...)
	return err
}

// Pull pulls from origin.
func (g *GitOps) Pull(ctx context.Context) error {
	_, err := g.runGit(ctx, "pull", "origin")
	return err
}

// Fetch fetches from origin.
func (g *GitOps) Fetch(ctx context.Context) error {
	_, err := g.runGit(ctx, "fetch", "origin")
	return err
}

// GetLastCommitMessage returns the last commit message.
func (g *GitOps) GetLastCommitMessage(ctx context.Context) (string, error) {
	msg, err := g.runGit(ctx, "log", "-1", "--pretty=%B")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(msg), nil
}

// GetFileContentAtCommit returns file content at a specific commit.
func (g *GitOps) GetFileContentAtCommit(ctx context.Context, file, commit string) (string, error) {
	return g.runGit(ctx, "show", fmt.Sprintf("%s:%s", commit, file))
}

// GetFilesChangedInCommit returns files changed in a commit.
func (g *GitOps) GetFilesChangedInCommit(ctx context.Context, commit string) ([]string, error) {
	output, err := g.runGit(ctx, "diff-tree", "--no-commit-id", "--name-only", "-r", commit)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(output), "\n"), nil
}

// IsGitRepo checks if the path is a git repository.
func (g *GitOps) IsGitRepo(ctx context.Context) bool {
	_, err := g.runGit(ctx, "rev-parse", "--git-dir")
	return err == nil
}

// HasRemote checks if origin remote exists.
func (g *GitOps) HasRemote(ctx context.Context) bool {
	_, err := g.runGit(ctx, "remote", "get-url", "origin")
	return err == nil
}

// GetRemoteURL returns the origin remote URL.
func (g *GitOps) GetRemoteURL(ctx context.Context) (string, error) {
	url, err := g.runGit(ctx, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(url), nil
}

// StageFiles stages files for commit.
func (g *GitOps) StageFiles(ctx context.Context, files []string) error {
	args := append([]string{"add"}, files...)
	_, err := g.runGit(ctx, args...)
	return err
}

// UnstageFiles unstages files.
func (g *GitOps) UnstageFiles(ctx context.Context, files []string) error {
	args := append([]string{"reset", "HEAD"}, files...)
	_, err := g.runGit(ctx, args...)
	return err
}

// DiscardChanges discards local changes.
func (g *GitOps) DiscardChanges(ctx context.Context, files []string) error {
	args := append([]string{"checkout", "--"}, files...)
	_, err := g.runGit(ctx, args...)
	return err
}

// Stash stashes current changes.
func (g *GitOps) Stash(ctx context.Context, message string) error {
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := g.runGit(ctx, args...)
	return err
}

// StashPop pops the latest stash.
func (g *GitOps) StashPop(ctx context.Context) error {
	_, err := g.runGit(ctx, "stash", "pop")
	return err
}

// GetStashList returns the stash list.
func (g *GitOps) GetStashList(ctx context.Context) ([]string, error) {
	output, err := g.runGit(ctx, "stash", "list")
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(output), "\n"), nil
}

// CreateCommitMessage generates a commit message based on changes.
func (g *GitOps) CreateCommitMessage(ctx context.Context, context string) (string, error) {
	diffs, err := g.GetStagedDiff(ctx)
	if err != nil {
		return "", err
	}

	if len(diffs) == 0 {
		return "", fmt.Errorf("no staged changes")
	}

	// Simple message generation based on changes
	var parts []string
	for _, diff := range diffs {
		file := filepath.Base(diff.File)
		if diff.Added > 0 && diff.Removed == 0 {
			parts = append(parts, fmt.Sprintf("add %s", file))
		} else if diff.Removed > 0 && diff.Added == 0 {
			parts = append(parts, fmt.Sprintf("remove %s", file))
		} else {
			parts = append(parts, fmt.Sprintf("update %s", file))
		}
	}

	if context != "" {
		return fmt.Sprintf("%s: %s", context, strings.Join(parts, ", ")), nil
	}

	return strings.Join(parts, ", "), nil
}

// parseDiff parses git diff output.
func (g *GitOps) parseDiff(diffOutput string) []*Diff {
	var diffs []*Diff
	var currentDiff *Diff

	lines := strings.Split(diffOutput, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			// Start of new diff
			if currentDiff != nil {
				diffs = append(diffs, currentDiff)
			}

			// Extract filename
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				currentDiff = &Diff{
					File: strings.TrimPrefix(parts[2], "a/"),
				}
			}
			continue
		}

		if currentDiff == nil {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			// Hunk header
			continue
		}

		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			currentDiff.OldLines = append(currentDiff.OldLines, line[1:])
			currentDiff.Removed++
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			currentDiff.NewLines = append(currentDiff.NewLines, line[1:])
			currentDiff.Added++
		}
	}

	if currentDiff != nil {
		diffs = append(diffs, currentDiff)
	}

	return diffs
}

// runGit runs a git command.
func (g *GitOps) runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v (stderr: %s)",
			strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}
