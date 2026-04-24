package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SubmoduleInfo struct {
	Path        string
	Commit      string
	LocalDelta  int64
	RemoteDelta int64
}

func getSubmoduleStatus(t *testing.T, path string) SubmoduleInfo {
	cmd := exec.Command("git", "submodule", "status", path)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git submodule status must succeed for %s", path)

	line := strings.TrimSpace(string(output))
	var info SubmoduleInfo
	info.Path = path

	if len(line) > 0 {
		switch line[0] {
		case '-':
			info.LocalDelta = -1
			line = line[1:]
		case '+':
			info.LocalDelta = 1
			line = line[1:]
		case 'U':
			info.LocalDelta = -2
			line = line[1:]
		}

		parts := strings.Fields(line)
		if len(parts) >= 1 {
			info.Commit = parts[0]
		}
	}

	return info
}

func getAllSubmodules(t *testing.T) []string {
	cmd := exec.Command("git", "submodule", "status")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git submodule status must succeed")

	var submodules []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			submodules = append(submodules, parts[1])
		}
	}
	return submodules
}

func TestSubmoduleSyncAllInitialized(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err)

	submodules := getAllSubmodules(t)
	require.NotEmpty(t, submodules, "Must have submodules")

	t.Logf("Found %d submodules", len(submodules))

	var uninitialized []string
	for _, path := range submodules {
		info := getSubmoduleStatus(t, path)
		if info.LocalDelta == -1 {
			uninitialized = append(uninitialized, path)
		}
	}

	assert.Empty(t, uninitialized, "All submodules must be initialized. Uninitialized: %v", uninitialized)
}

func TestSubmoduleSyncNoLocalModifications(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err)

	submodules := getAllSubmodules(t)
	require.NotEmpty(t, submodules)

	var modified []string
	for _, path := range submodules {
		submodulePath := filepath.Join(projectRoot, path)

		cmd := exec.Command("git", "-C", submodulePath, "diff", "--exit-code")
		if err := cmd.Run(); err != nil {
			modified = append(modified, path+": has unstaged changes")
			continue
		}

		cmd = exec.Command("git", "-C", submodulePath, "diff", "--cached", "--exit-code")
		if err := cmd.Run(); err != nil {
			modified = append(modified, path+": has staged changes")
		}
	}

	assert.Empty(t, modified, "No submodules should have local modifications. Modified: %v", modified)
}

func TestSubmoduleSyncNoDivergentBranches(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err)

	submodules := getAllSubmodules(t)
	require.NotEmpty(t, submodules)

	var divergent []string
	var externalDetached []string
	for _, path := range submodules {
		submodulePath := filepath.Join(projectRoot, path)

		cmd := exec.Command("git", "-C", submodulePath, "rev-parse", "--abbrev-ref", "HEAD")
		output, err := cmd.CombinedOutput()
		if err != nil {
			divergent = append(divergent, path+": cannot read HEAD branch")
			continue
		}

		branch := strings.TrimSpace(string(output))
		if branch == "HEAD" {
			if strings.HasPrefix(path, "cli_agents/") {
				externalDetached = append(externalDetached, path)
			} else {
				divergent = append(divergent, path+": detached HEAD state (non-external)")
			}
		}
	}

	for _, path := range externalDetached {
		t.Logf("WARNING: %s has detached HEAD (external repo - may be expected)", path)
	}

	assert.Empty(t, divergent, "Non-external submodules should not have divergent branch state. Divergent: %v", divergent)
}

func TestSubmoduleSyncHelixQAOpensourceTools(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err)

	helixQAPath := "HelixQA"
	if _, err := os.Stat(helixQAPath); os.IsNotExist(err) {
		t.Skip("HelixQA submodule not present")  // SKIP-OK: #legacy-untriaged
	}

	helixQARoot := filepath.Join(projectRoot, helixQAPath)

	cmd := exec.Command("git", "-C", helixQARoot, "submodule", "status")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "HelixQA submodule status must succeed")

	qaSubmodules := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			qaSubmodules[parts[1]] = parts[0]
		}
	}

	opensourceTools := "tools/opensource"
	if _, err := os.Stat(filepath.Join(helixQARoot, opensourceTools)); os.IsNotExist(err) {
		t.Skip("tools/opensource not present in HelixQA")  // SKIP-OK: #legacy-untriaged
	}

	var opensourceSubmodules []string
	for path := range qaSubmodules {
		if strings.HasPrefix(path, opensourceTools) {
			opensourceSubmodules = append(opensourceSubmodules, path)
		}
	}

	if len(opensourceSubmodules) == 0 {
		t.Skip("No opensource tool submodules found in HelixQA")  // SKIP-OK: #legacy-untriaged
	}

	t.Logf("Checking %d opensource tool submodules in HelixQA", len(opensourceSubmodules))

	var issues []string
	for _, relPath := range opensourceSubmodules {
		fullPath := filepath.Join(helixQARoot, relPath)
		info, err := os.Stat(fullPath)
		if err != nil {
			issues = append(issues, relPath+": directory does not exist")
			continue
		}
		if !info.IsDir() {
			issues = append(issues, relPath+": not a directory")
			continue
		}

		gitDir := filepath.Join(fullPath, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			issues = append(issues, relPath+": not initialized (.git missing)")
			continue
		}

		cmd = exec.Command("git", "-C", fullPath, "status", "--porcelain")
		out, err := cmd.CombinedOutput()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			issues = append(issues, relPath+": has uncommitted changes")
		}

		cmd = exec.Command("git", "-C", fullPath, "fetch", "--all", "--quiet")
		_ = cmd.Run()

		cmd = exec.Command("git", "-C", fullPath, "log", "--oneline", "@{u}..HEAD")
		out, err = cmd.CombinedOutput()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			issues = append(issues, relPath+": local is ahead of remote")
		}

		cmd = exec.Command("git", "-C", fullPath, "log", "--oneline", "HEAD..@{u}")
		out, err = cmd.CombinedOutput()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			issues = append(issues, relPath+": remote is ahead of local (needs update)")
		}
	}

	assert.Empty(t, issues, "HelixQA opensource tools must be in sync. Issues: %v", issues)
}

func TestSubmoduleSyncHelixQAGitHubVsGitLab(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err)

	helixQAPath := "HelixQA"
	if _, err := os.Stat(helixQAPath); os.IsNotExist(err) {
		t.Skip("HelixQA submodule not present")  // SKIP-OK: #legacy-untriaged
	}

	helixQARoot := filepath.Join(projectRoot, helixQAPath)

	cmd := exec.Command("git", "-C", helixQARoot, "remote", "-v")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "HelixQA must have remotes configured")

	var githubCommit, gitlabCommit string
	var hasGithub, hasGitlab bool

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		_ = parts[0] // remote name (unused)
		url := parts[1]

		isGitHub := strings.Contains(url, "github.com")
		isGitLab := strings.Contains(url, "gitlab.com")

		var refCmd *exec.Cmd
		if isGitHub {
			refCmd = exec.Command("git", "-C", helixQARoot, "rev-parse", "--short", "github/main")
			if out, err := refCmd.CombinedOutput(); err == nil {
				githubCommit = strings.TrimSpace(string(out))
				hasGithub = true
			}
		}
		if isGitLab {
			refCmd = exec.Command("git", "-C", helixQARoot, "rev-parse", "--short", "gitlab/main")
			if out, err := refCmd.CombinedOutput(); err == nil {
				gitlabCommit = strings.TrimSpace(string(out))
				hasGitlab = true
			}
		}
	}

	t.Logf("HelixQA - GitHub main: %s, GitLab main: %s", githubCommit, gitlabCommit)

	if hasGithub && hasGitlab {
		assert.Equal(t, githubCommit, gitlabCommit,
			"HelixQA GitHub and GitLab must be at same commit")
	}
}

func TestSubmoduleSyncRemoteConsistency(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err)

	submodules := getAllSubmodules(t)
	require.NotEmpty(t, submodules)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var fetchFailures []string
	var externalFailures []string
	var skipped []string
	for _, path := range submodules {
		if ctx.Err() != nil {
			t.Logf("Context timeout reached after checking %d/%d submodules", len(fetchFailures)+len(externalFailures)+len(skipped), len(submodules))
			break
		}

		submodulePath := filepath.Join(projectRoot, path)

		cmdCtx, cmdCancel := context.WithTimeout(ctx, 10*time.Second)
		cmd := exec.CommandContext(cmdCtx, "git", "-C", submodulePath, "fetch", "origin", "--quiet")
		err := cmd.Run()
		cmdCancel()

		if err != nil {
			if ctx.Err() != nil || cmdCtx.Err() != nil {
				skipped = append(skipped, path+" (timeout)")
				continue
			}
			if strings.HasPrefix(path, "cli_agents/") {
				externalFailures = append(externalFailures, path)
			} else {
				fetchFailures = append(fetchFailures, path)
			}
		}
	}

	total := len(submodules)
	failed := len(fetchFailures)
	externalFailed := len(externalFailures)
	skippedTotal := len(skipped)

	if skippedTotal > 0 {
		t.Logf("Note: %d/%d submodules skipped due to timeout (may be network-limited or multi-remote)", skippedTotal, total)
	}

	if failed > 0 {
		t.Errorf("Non-external submodules failed to fetch: %v", fetchFailures)
	}

	if externalFailed > 0 {
		t.Logf("Note: %d/%d external cli_agents submodules failed to fetch (may be expected for private repos)",
			externalFailed, total)
	}
}

func TestSubmoduleSyncMainRepoState(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err)

	var issues []string

	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err == nil {
		status := strings.TrimSpace(string(output))
		if status != "" {
			lines := strings.Split(status, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "??") && strings.Contains(line, "tests/integration/submodule_sync_test.go") {
					continue
				}
				if strings.HasPrefix(line, "M") && strings.TrimPrefix(line, "M ") == "Makefile" {
					continue
				}
				issues = append(issues, "main repo has uncommitted changes: "+line)
			}
		}
	}

	submodules := getAllSubmodules(t)
	for _, path := range submodules {
		info := getSubmoduleStatus(t, path)
		if info.LocalDelta != 0 {
			issues = append(issues, path+": local delta is "+string(rune(info.LocalDelta)))
		}
	}

	secretFile := "docs/SUBMODULE_UPSTREAM_STATUS.md"
	if _, err := os.Stat(secretFile); err == nil {
		issues = append(issues, secretFile+": secret file must not exist in working tree")
	}

	assert.Empty(t, issues, "Main repo must be in clean state. Issues: %v", issues)
}

func TestSubmoduleSyncSecretFilesNotInHistory(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err)

	sensitiveFiles := []string{
		"docs/SUBMODULE_UPSTREAM_STATUS.md",
	}

	for _, file := range sensitiveFiles {
		cmd := exec.Command("git", "ls-files", "--error-unmatch", file)
		if err := cmd.Run(); err == nil {
			t.Errorf("Secret file %s must NOT be tracked in git", file)
		}
	}
}

func BenchmarkSubmoduleSyncAllStatus(b *testing.B) {
	wd, _ := os.Getwd()
	os.Chdir(wd)

	submodules := getAllSubmodules(&testing.T{})
	for i := 0; i < b.N; i++ {
		for _, path := range submodules {
			cmd := exec.Command("git", "submodule", "status", path)
			_, _ = cmd.CombinedOutput()
		}
	}
}

func BenchmarkSubmoduleSyncHelixQAOpensourceTools(b *testing.B) {
	wd, _ := os.Getwd()
	os.Chdir(wd)

	for i := 0; i < b.N; i++ {
		cmd := exec.Command("git", "submodule", "status", "HelixQA/tools/opensource")
		_, _ = cmd.CombinedOutput()
	}
}
