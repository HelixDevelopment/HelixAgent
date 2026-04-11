package tools

// Sandbox regression tests for resolveInWorkingDir — the G703 fix added
// in the 2026-04-11 gosec triage. Each case pins one invariant that, if
// it regressed, would re-open the LLM tool-call path traversal exposure.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSandboxedExecutor(t *testing.T) (*DefaultToolExecutor, string) {
	t.Helper()
	dir := t.TempDir()
	// Resolve symlinks up front so we compare like-for-like on macOS,
	// where /tmp is a symlink to /private/tmp.
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	e := NewDefaultToolExecutor(nil)
	e.workingDir = resolved
	return e, resolved
}

func TestResolveInWorkingDir_EmptyPathRejected(t *testing.T) {
	e, _ := newSandboxedExecutor(t)
	_, err := e.resolveInWorkingDir("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestResolveInWorkingDir_NoWorkingDirRejected(t *testing.T) {
	// A brand-new executor created without a working dir must refuse to
	// touch the filesystem — failing closed is a safety property.
	e := NewDefaultToolExecutor(nil)
	e.workingDir = ""
	_, err := e.resolveInWorkingDir("probe.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "working directory")
}

func TestResolveInWorkingDir_RelativeWithin(t *testing.T) {
	e, base := newSandboxedExecutor(t)
	// Create a file inside the sandbox so EvalSymlinks has a target.
	inside := filepath.Join(base, "ok.txt")
	require.NoError(t, os.WriteFile(inside, []byte("x"), 0o600))

	got, err := e.resolveInWorkingDir("ok.txt")
	require.NoError(t, err)
	assert.Equal(t, inside, got)
}

func TestResolveInWorkingDir_RelativeNewFile(t *testing.T) {
	// A write path to a new file (parent exists, leaf does not) must
	// resolve to an absolute path inside the sandbox — the WriteFile
	// caller will create it. This is the critical "create new file"
	// branch that must not be rejected.
	e, base := newSandboxedExecutor(t)
	got, err := e.resolveInWorkingDir("new/leaf.txt")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(got, base),
		"expected %q under %q", got, base)
}

func TestResolveInWorkingDir_NestedSubdir(t *testing.T) {
	e, base := newSandboxedExecutor(t)
	sub := filepath.Join(base, "a", "b", "c")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "d.txt"), []byte("y"), 0o600))

	got, err := e.resolveInWorkingDir("a/b/c/d.txt")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(sub, "d.txt"), got)
}

func TestResolveInWorkingDir_DotDotEscapeRejected(t *testing.T) {
	e, _ := newSandboxedExecutor(t)
	cases := []string{
		"../escape.txt",
		"../../../../etc/passwd",
		"a/../../escape.txt",
		"sub/../../out.txt",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			_, err := e.resolveInWorkingDir(p)
			require.Error(t, err, "path %q must be rejected", p)
			assert.Contains(t, err.Error(), "escapes working directory")
		})
	}
}

func TestResolveInWorkingDir_AbsolutePathOutsideRejected(t *testing.T) {
	e, _ := newSandboxedExecutor(t)
	outside := "/etc/passwd"
	if runtime.GOOS == "windows" {
		outside = `C:\Windows\System32\drivers\etc\hosts`
	}
	_, err := e.resolveInWorkingDir(outside)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes working directory")
}

func TestResolveInWorkingDir_AbsolutePathInsideAccepted(t *testing.T) {
	e, base := newSandboxedExecutor(t)
	inside := filepath.Join(base, "inside.txt")
	require.NoError(t, os.WriteFile(inside, []byte("z"), 0o600))

	got, err := e.resolveInWorkingDir(inside)
	require.NoError(t, err)
	assert.Equal(t, inside, got)
}

func TestResolveInWorkingDir_SymlinkEscapeRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation on windows test runners is flaky; unix-only")
	}
	e, base := newSandboxedExecutor(t)

	// Create an outside target, then a symlink inside the sandbox
	// pointing at it. The sandbox must refuse operations on the
	// symlinked path — EvalSymlinks resolves to the outside target
	// and the containment check catches the escape.
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))

	link := filepath.Join(base, "link.txt")
	require.NoError(t, os.Symlink(outside, link))

	_, err := e.resolveInWorkingDir("link.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes working directory")
}

func TestResolveInWorkingDir_PrefixCollisionRejected(t *testing.T) {
	// If the working directory is /tmp/abc and the attacker tries
	// /tmp/abc-attacker/file, the fix must reject it — prefix
	// comparison without a separator would let it through.
	base := t.TempDir()
	resolved, err := filepath.EvalSymlinks(base)
	require.NoError(t, err)

	// Manufacture a sibling dir that shares the prefix.
	attackerDir := resolved + "-attacker"
	require.NoError(t, os.MkdirAll(attackerDir, 0o755))
	attacker := filepath.Join(attackerDir, "secret.txt")
	require.NoError(t, os.WriteFile(attacker, []byte("nope"), 0o600))

	e := NewDefaultToolExecutor(nil)
	e.workingDir = resolved

	_, err = e.resolveInWorkingDir(attacker)
	require.Error(t, err, "prefix collision must be rejected")
	assert.Contains(t, err.Error(), "escapes working directory")
}
