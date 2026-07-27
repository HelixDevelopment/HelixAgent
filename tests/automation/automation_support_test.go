package automation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Working-directory resolution
// ---------------------------------------------------------------------------
//
// `go test` runs a package's test binary with the working directory set to that
// package's source directory — here tests/automation. Every command in this
// suite (`go build ./cmd/helixagent`, `go vet ./...`, `make fmt`, `gofmt -l .`,
// `git diff go.mod`, `docker build -f docker/…`) is written against the
// repository root, so every one of them was resolving against
// tests/automation/… and failing with "directory not found" / "No rule to make
// target" / "unknown revision or path not in the working tree". The suite was
// therefore reporting on a directory that contains nothing but these four test
// files. repoCommand pins every command to the module root so the assertions
// describe the repository they claim to describe.

var (
	rootOnce sync.Once
	rootDir  string
	rootErr  error
)

// repoRoot returns the directory holding the `module dev.helix.agent` go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	rootOnce.Do(func() {
		dir, err := os.Getwd()
		if err != nil {
			rootErr = err
			return
		}
		for {
			if isModuleRoot(filepath.Join(dir, "go.mod"), "dev.helix.agent") {
				rootDir = dir
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				rootErr = fmt.Errorf("no go.mod declaring `module dev.helix.agent` above the working directory")
				return
			}
			dir = parent
		}
	})
	if rootErr != nil {
		t.Fatalf("cannot locate repository root: %v", rootErr)
	}
	return rootDir
}

func isModuleRoot(goModPath, module string) bool {
	raw, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "module "+module {
			return true
		}
	}
	return false
}

// repoPath joins path elements onto the repository root.
func repoPath(t *testing.T, elem ...string) string {
	t.Helper()
	return filepath.Join(append([]string{repoRoot(t)}, elem...)...)
}

// repoCommand builds a command whose working directory is the repository root.
func repoCommand(t *testing.T, name string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = repoRoot(t)
	return cmd
}

// repoCommandContext is repoCommand with a context attached.
func repoCommandContext(t *testing.T, ctx context.Context, name string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = repoRoot(t)
	return cmd
}
