package gitmcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.agent/internal/clis/agents/base"
)

// ---------------------------------------------------------------------------
// D-17 STUB-BLUFF PIN GUARD — RED-on-broken-artifact + GREEN regression guard
// (§11.4.115 polarity switch / §11.4.135 standing guard)
//
// HISTORY: GitMCP.commit USED to return a FABRICATED constant commit hash
// ("commit":"abc123","status":"committed") WITHOUT running git at all (zero
// os/exec in gitmcp.go) — a stub bluff per BLUFF-001/003 / CONST-035: the agent
// claimed to commit while running nothing.
//
// FIX (D-17): commit/branch/status now exec the REAL git CLI in the repo dir;
// commit returns the REAL `git rev-parse HEAD` SHA. These guards run against a
// REAL temporary git repository (real infrastructure, no mocks per CONST-050).
// ---------------------------------------------------------------------------

// initRealRepo creates a real, fully-initialized git repository in a temp dir
// with one initial commit, and returns the dir. Skips if git is unavailable.
func initRealRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("SKIP-OK: D-17 — git not installed on PATH; the real-exec code path is identical regardless.")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v (%s)", strings.Join(args, " "), err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@helix.local")
	run("config", "user.name", "Helix Test")
	run("config", "commit.gpgsign", "false")
	// Seed a tracked file + initial commit so HEAD exists.
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "seed")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seed commit: %v (%s)", err, out)
	}
	return dir
}

func newGitMCP(t *testing.T, repo string) *GitMCP {
	t.Helper()
	g := New()
	if err := g.Initialize(context.Background(), &Config{
		BaseConfig: base.BaseConfig{WorkDir: repo},
		Repository: repo,
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	return g
}

// realHEAD returns the real current HEAD SHA of the repo.
func realHEAD(t *testing.T, repo string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse: %v (%s)", err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestD17_GitMCP_CommitRunsRealGit — standing GREEN regression guard.
func TestD17_GitMCP_CommitRunsRealGit(t *testing.T) {
	repo := initRealRepo(t)
	g := newGitMCP(t, repo)
	ctx := context.Background()

	// Stage a real change so `git commit` has something to commit (real git behaviour).
	if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("real\n"), 0o644); err != nil {
		t.Fatalf("write feature.txt: %v", err)
	}
	stage := exec.Command("git", "add", "feature.txt")
	stage.Dir = repo
	if out, err := stage.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v (%s)", err, out)
	}

	res, err := g.commit(ctx, map[string]interface{}{"message": "D17 real commit"})
	if err != nil {
		t.Fatalf("commit returned error against a real repo: %v", err)
	}
	m, _ := res.(map[string]interface{})
	sha, _ := m["commit"].(string)

	if sha == "abc123" {
		t.Fatalf("D17 REGRESSION: GitMCP.commit returned the fabricated literal 'abc123' (BLUFF-001 reintroduced).")
	}
	// The returned SHA MUST equal the REAL current HEAD — proof a real commit ran.
	if want := realHEAD(t, repo); sha != want {
		t.Fatalf("D17 REGRESSION: GitMCP.commit returned %q but real HEAD is %q — the commit was not real.", sha, want)
	}
	if len(sha) < 7 {
		t.Fatalf("D17 REGRESSION: returned commit SHA %q is not a real git SHA.", sha)
	}
}

// TestD17_GitMCP_StatusReadsRealRepo — status reflects real working-tree state.
func TestD17_GitMCP_StatusReadsRealRepo(t *testing.T) {
	repo := initRealRepo(t)
	g := newGitMCP(t, repo)
	ctx := context.Background()

	res, err := g.status(ctx)
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}
	m, _ := res.(map[string]interface{})
	if clean, _ := m["clean"].(bool); !clean {
		t.Fatalf("D17: freshly-seeded repo should be clean, porcelain=%q", m["porcelain"])
	}

	// Create an untracked file → real porcelain must change.
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("create dirty file: %v", err)
	}
	res2, err := g.status(ctx)
	if err != nil {
		t.Fatalf("status (dirty) returned error: %v", err)
	}
	m2, _ := res2.(map[string]interface{})
	if clean, _ := m2["clean"].(bool); clean {
		t.Fatalf("D17 REGRESSION: GitMCP.status reported clean against a dirty repo — it is not reading real git state.")
	}
	if porc, _ := m2["porcelain"].(string); !strings.Contains(porc, "dirty.txt") {
		t.Fatalf("D17 REGRESSION: porcelain %q does not reflect the real untracked file.", porc)
	}
}

// TestD17_GitMCP_AbsentGitIsHonestError — with no git resolvable, honest error.
func TestD17_GitMCP_AbsentGitIsHonestError(t *testing.T) {
	t.Setenv("GITMCP_GIT_BIN", t.TempDir()+"/does-not-exist-git")
	g := New()
	if err := g.Initialize(context.Background(), &Config{Repository: t.TempDir()}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := g.commit(context.Background(), map[string]interface{}{"message": "x"}); err == nil {
		t.Fatal("D17 BLUFF: commit returned success with NO git available — must be an honest error.")
	}
}

// TestD17_GitMCP_CommitIsStubBluff — §11.4.115 RED-on-broken-artifact, RED_MODE=1.
func TestD17_GitMCP_CommitIsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"The standing GREEN guard is TestD17_GitMCP_CommitRunsRealGit.")
	}
	repo := initRealRepo(t)
	g := newGitMCP(t, repo)
	res, err := g.commit(context.Background(), map[string]interface{}{"message": "x"})
	if err != nil {
		return
	}
	m, _ := res.(map[string]interface{})
	if sha, _ := m["commit"].(string); sha == "abc123" {
		t.Fatalf("D17 BLUFF PINNED: GitMCP.commit returned the fabricated literal 'abc123' without running git (BLUFF-001).")
	}
}
