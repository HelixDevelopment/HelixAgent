// Package regression holds standing regression guards for helix_agent.
//
// cli_agents_isolation_test.go is the §11.4.115 RED-polarity guard for HXC-139:
// a vendored Continue-submodule Go test fixture with a domain-less import
// (cli_agents/continue/core/autocomplete/context/root-path-context/test/files/file1.go)
// was swept into the dev.helix.agent module build, breaking `go build ./...`
// and `go vet ./...` for the whole module. The fix is a nested-module marker
// cli_agents/go.mod that excludes the whole vendored-reference tree from
// dev.helix.agent. This test is BOTH the bug-catcher and the standing guard,
// with a single RED_MODE polarity switch (§11.4.115):
//
//	RED_MODE=1 : reproduce the defect on the pre-fix artifact (move the marker
//	             aside, prove the module build breaks with the exact HXC-139
//	             signature). Proves the guard genuinely catches the defect.
//	RED_MODE=0 : (default) standing GREEN guard — the marker is present and no
//	             cli_agents package is compiled into dev.helix.agent.
//
// §1.1 anti-bluff pairing: if cli_agents/go.mod were reverted, the GREEN guard
// FAILs (go list ./... would surface a cli_agents package / error on the
// broken fixture). RED mode exercises exactly that reverted state.
package regression

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	brokenFixturePkg = "./cli_agents/continue/core/autocomplete/context/root-path-context/test/files/"
	markerRel        = "cli_agents/go.mod"
	markerModuleLine = "module dev.helix.agent.vendored/cli_agents"
)

// moduleRoot walks up from the test's working directory to the directory whose
// go.mod declares `module dev.helix.agent` (the helix_agent module root).
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	for {
		gomod := filepath.Join(dir, "go.mod")
		if modulePathOf(gomod) == "dev.helix.agent" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate the dev.helix.agent module root walking up from the test dir")
		}
		dir = parent
	}
}

// modulePathOf returns the module path declared in a go.mod file, or "" if the
// file is absent/unreadable or declares no module line.
func modulePathOf(gomod string) string {
	f, err := os.Open(gomod)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// runGo runs `go <args...>` in dir with a bounded timeout and returns combined
// output + error.
func runGo(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("`go %s` timed out after 120s in %s", strings.Join(args, " "), dir)
	}
	return string(out), err
}

func TestCliAgentsIsolation(t *testing.T) {
	root := moduleRoot(t)
	marker := filepath.Join(root, markerRel)

	if os.Getenv("RED_MODE") == "1" {
		// RED: reproduce HXC-139 on the pre-fix artifact by moving the marker
		// aside so the vendored tree is compiled into dev.helix.agent again.
		aside := marker + ".reddisabled"
		data, err := os.ReadFile(marker)
		if err != nil {
			t.Fatalf("RED: cannot read %s (fix must be present to reproduce its removal): %v", markerRel, err)
		}
		if err := os.Rename(marker, aside); err != nil {
			t.Fatalf("RED: cannot move marker aside: %v", err)
		}
		// Restore NO MATTER WHAT — including on panic/failure.
		t.Cleanup(func() {
			if _, statErr := os.Stat(marker); os.IsNotExist(statErr) {
				if err := os.WriteFile(marker, data, 0o644); err != nil {
					t.Errorf("RED cleanup: failed to restore %s: %v", markerRel, err)
				}
			}
			_ = os.Remove(aside)
		})

		out, err := runGo(t, root, "build", brokenFixturePkg)
		if err == nil {
			t.Fatalf("RED: expected `go build %s` to FAIL on the pre-fix artifact, but it succeeded.\nOutput:\n%s", brokenFixturePkg, out)
		}
		if !strings.Contains(out, "file1.go") || !strings.Contains(out, "is not in std") {
			t.Fatalf("RED: build failed but not with the HXC-139 signature (want 'file1.go' + 'is not in std').\nOutput:\n%s", out)
		}
		// Restore + confirm before returning.
		if err := os.WriteFile(marker, data, 0o644); err != nil {
			t.Fatalf("RED: failed to restore marker: %v", err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("RED: marker not restored: %v", err)
		}
		t.Logf("RED reproduced HXC-139: module build breaks with the file1.go 'is not in std' signature when %s is absent", markerRel)
		return
	}

	// GREEN (default): the standing regression guard.
	// (1) the isolation marker exists and declares the nested module.
	got := modulePathOf(marker)
	if got == "" {
		t.Fatalf("GREEN: %s missing or has no module line — HXC-139 fix absent", markerRel)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("GREEN: cannot read %s: %v", markerRel, err)
	}
	if !strings.Contains(string(data), markerModuleLine) {
		t.Fatalf("GREEN: %s does not declare %q (got module %q)", markerRel, markerModuleLine, got)
	}

	// (2) no cli_agents package is part of dev.helix.agent.
	out, err := runGo(t, root, "list", "./...")
	if err != nil {
		t.Fatalf("GREEN: `go list ./...` failed (the module does not cleanly enumerate — vendored fixtures may be leaking in):\n%s", out)
	}
	var pkgs, leaked int
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pkgs++
		if strings.Contains(line, "cli_agents") {
			leaked++
			t.Errorf("GREEN: cli_agents package leaked into dev.helix.agent: %s", line)
		}
	}
	if pkgs == 0 {
		t.Fatalf("GREEN: `go list ./...` returned zero packages — unexpected")
	}
	if leaked != 0 {
		t.Fatalf("GREEN: %d cli_agents package(s) compiled into dev.helix.agent — HXC-139 isolation broken", leaked)
	}
	t.Logf("GREEN: %d dev.helix.agent packages, 0 under cli_agents — HXC-139 isolation intact", pkgs)
}
