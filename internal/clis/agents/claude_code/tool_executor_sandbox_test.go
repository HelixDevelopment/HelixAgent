package claude_code

// Sandbox regression tests for the claude_code tool executor.
// Parallels internal/tools/tool_executors_sandbox_test.go: verifies that
// every LLM-tool path goes through sanitizePath and that escape attempts
// fail with a clear "path escapes working directory" error rather than
// touching files outside workDir.

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSandboxedToolExecutor(t *testing.T) (*ToolExecutor, string) {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	te := &ToolExecutor{
		workDir:      resolved,
		allowedTools: map[string]bool{},
	}
	return te, resolved
}

func TestClaudeCode_SanitizePath_AcceptsRelative(t *testing.T) {
	te, base := newSandboxedToolExecutor(t)
	// Create a file so EvalSymlinks has a real target for the happy path.
	require.NoError(t, os.WriteFile(filepath.Join(base, "ok.txt"), []byte("x"), 0o600))
	got := te.sanitizePath("ok.txt")
	assert.Equal(t, filepath.Join(base, "ok.txt"), got)
}

func TestClaudeCode_SanitizePath_AcceptsAbsoluteInside(t *testing.T) {
	te, base := newSandboxedToolExecutor(t)
	inside := filepath.Join(base, "a", "b.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(inside), 0o755))
	require.NoError(t, os.WriteFile(inside, []byte("y"), 0o600))
	got := te.sanitizePath(inside)
	assert.Equal(t, inside, got)
}

func TestClaudeCode_SanitizePath_RejectsDotDotEscape(t *testing.T) {
	te, _ := newSandboxedToolExecutor(t)
	for _, p := range []string{
		"../escape.txt",
		"../../../../etc/passwd",
		"a/../../escape.txt",
	} {
		t.Run(p, func(t *testing.T) {
			assert.Equal(t, "", te.sanitizePath(p),
				"path %q must be rejected", p)
		})
	}
}

func TestClaudeCode_SanitizePath_RejectsAbsoluteOutside(t *testing.T) {
	te, _ := newSandboxedToolExecutor(t)
	outside := "/etc/passwd"
	if runtime.GOOS == "windows" {
		outside = `C:\Windows\System32\drivers\etc\hosts`
	}
	assert.Equal(t, "", te.sanitizePath(outside))
}

func TestClaudeCode_SanitizePath_NoWorkdirFailsClosed(t *testing.T) {
	te := &ToolExecutor{workDir: ""}
	assert.Equal(t, "", te.sanitizePath("anything.txt"))
	assert.Equal(t, "", te.sanitizePath(""))
}

func TestClaudeCode_ReadFile_RejectsEscape(t *testing.T) {
	te, _ := newSandboxedToolExecutor(t)
	result, err := te.readFile(context.Background(), map[string]interface{}{
		"file_path": "../../../etc/passwd",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "path escapes working directory", result.Error,
		"readFile must surface the sandbox error, not a generic filesystem error")
}

func TestClaudeCode_WriteFile_RejectsEscape(t *testing.T) {
	te, _ := newSandboxedToolExecutor(t)
	result, err := te.writeFile(context.Background(), map[string]interface{}{
		"file_path": "../../../tmp/pwned.txt",
		"content":   "should never be written",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "path escapes working directory", result.Error)
	// Double-check nothing was actually written.
	_, statErr := os.Stat("/tmp/pwned.txt")
	assert.True(t, os.IsNotExist(statErr) || statErr != nil,
		"writeFile with escape must produce no filesystem side effect")
}

func TestClaudeCode_EditFile_RejectsEscape(t *testing.T) {
	te, _ := newSandboxedToolExecutor(t)
	result, err := te.editFile(context.Background(), map[string]interface{}{
		"file_path":  "../escape.txt",
		"old_string": "foo",
		"new_string": "bar",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "path escapes working directory", result.Error)
}

func TestClaudeCode_WriteFile_AcceptsInsideSandbox(t *testing.T) {
	te, base := newSandboxedToolExecutor(t)
	result, err := te.writeFile(context.Background(), map[string]interface{}{
		"file_path": "nested/ok.txt",
		"content":   "hello",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Truef(t, result.Success, "write to relative path should succeed: %+v", result)

	// Verify the file exists under the sandbox and nowhere else.
	got, err := os.ReadFile(filepath.Join(base, "nested", "ok.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestClaudeCode_EditFile_ReplacesContent(t *testing.T) {
	te, base := newSandboxedToolExecutor(t)
	// Pre-populate a file.
	require.NoError(t, os.MkdirAll(base, 0o755))
	target := filepath.Join(base, "greet.txt")
	require.NoError(t, os.WriteFile(target, []byte("hello world"), 0o600))

	result, err := te.editFile(context.Background(), map[string]interface{}{
		"file_path":  "greet.txt",
		"old_string": "hello",
		"new_string": "goodbye",
	})
	require.NoError(t, err)
	require.True(t, result.Success, "editFile should succeed: %+v", result)

	updated, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "goodbye world", string(updated))
}
