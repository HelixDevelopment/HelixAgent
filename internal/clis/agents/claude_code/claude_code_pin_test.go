package claude_code

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"dev.helix.agent/internal/clis/agents/base"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// D-16 (BLUFF-001) pin guards.
//
// These tests are RED-on-broken-artifact + polarity-switch guards per
// §11.4.115. When RED_MODE=1 (default) they REPRODUCE the historical bluff:
// handleChat / handleReview / handleEdit fabricated their textual output
// (echo / template strings) instead of exec-ing the real `claude` CLI. When
// RED_MODE=0 (the standing GREEN regression guard) they assert the bluff is
// ABSENT — the real `claude` exec path is wired and a fake binary injected on
// PATH flows its output through, which is impossible without genuine os/exec.
//
// The GREEN assertion uses a fake `claude` binary injected via the
// CLAUDE_BIN override env var. The fake echoes an unforgeable token in the
// JSON envelope; observing that token in the response proves the handler
// actually executed the binary and parsed its stdout — a value that cannot be
// produced by any in-process template.

const redMode = "RED_MODE"

// isRedMode reports whether the historical-defect REPRODUCTION branch should
// run. Default is GREEN (the standing §11.4.135 regression guard): RED_MODE=1
// opts into reproducing the pre-fix bluff on a pre-fix artifact, which is only
// meaningful before the fix lands.
func isRedMode() bool { return os.Getenv(redMode) == "1" }

// writeFakeClaude writes a fake `claude` executable into a temp dir and
// returns its absolute path. The fake ignores its args and emits a JSON
// envelope whose "result" field carries the marker, so extractClaudeText
// surfaces it. On non-unix CI this is skipped.
func writeFakeClaude(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: fake-binary injection unsupported on windows")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude")
	script := "#!/bin/sh\n" +
		"printf '{\"result\":\"" + marker + "\"}'\n"
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	return bin
}

func TestPin_HandleChat_NotFabricatedEcho(t *testing.T) {
	cc := New()
	ctx := context.Background()
	tempDir := t.TempDir()
	require.NoError(t, cc.Initialize(ctx, &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}))

	const marker = "PIN-CHAT-9f1c-REALEXEC"

	if isRedMode() {
		// Reproduce the defect on the (pre-fix) artifact: with NO real exec
		// wired, handleChat echoed "Claude Code received: <msg>". Assert that
		// fabricated shape is present — the bluff is genuinely reproducible.
		res, err := cc.Execute(ctx, "chat", map[string]interface{}{"message": "ping-" + marker})
		require.NoError(t, err)
		resp := res.(*Response)
		assert.Contains(t, resp.Content, "Claude Code received",
			"RED: pre-fix handleChat fabricates an echo response instead of exec-ing claude")
		return
	}

	// GREEN: inject a fake `claude` binary; its marker MUST flow through,
	// proving real os/exec + stdout parse (impossible via any template).
	bin := writeFakeClaude(t, marker)
	t.Setenv("CLAUDE_BIN", bin)

	res, err := cc.Execute(ctx, "chat", map[string]interface{}{"message": "hello"})
	require.NoError(t, err)
	resp := res.(*Response)
	assert.Equal(t, marker, resp.Content,
		"GREEN: handleChat must surface the real claude binary stdout, not a fabricated echo")
	assert.NotContains(t, resp.Content, "Claude Code received",
		"GREEN: the fabricated echo literal must be gone")
}

func TestPin_HandleReview_NotFabricated(t *testing.T) {
	cc := New()
	ctx := context.Background()
	tempDir := t.TempDir()
	require.NoError(t, cc.Initialize(ctx, &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}))

	const marker = "PIN-REVIEW-3a7b-REALEXEC"

	if isRedMode() {
		res, err := cc.Execute(ctx, "review", map[string]interface{}{"target": "."})
		require.NoError(t, err)
		resp := res.(*Response)
		assert.Contains(t, resp.Content, "code quality, security, and best practices",
			"RED: pre-fix handleReview fabricates a templated review string")
		return
	}

	bin := writeFakeClaude(t, marker)
	t.Setenv("CLAUDE_BIN", bin)

	res, err := cc.Execute(ctx, "review", map[string]interface{}{"target": "."})
	require.NoError(t, err)
	resp := res.(*Response)
	assert.Equal(t, marker, resp.Content,
		"GREEN: handleReview must surface the real claude binary stdout")
}

func TestPin_HandleEdit_NotFabricated(t *testing.T) {
	cc := New()
	ctx := context.Background()
	tempDir := t.TempDir()
	require.NoError(t, cc.Initialize(ctx, &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}))

	const marker = "PIN-EDIT-5d2e-REALEXEC"

	if isRedMode() {
		res, err := cc.Execute(ctx, "edit", map[string]interface{}{
			"file": "x.go", "instruction": "do it",
		})
		require.NoError(t, err)
		resp := res.(*Response)
		assert.Contains(t, resp.Content, "Applied edit to",
			"RED: pre-fix handleEdit fabricates an 'Applied edit' string without exec")
		return
	}

	bin := writeFakeClaude(t, marker)
	t.Setenv("CLAUDE_BIN", bin)

	res, err := cc.Execute(ctx, "edit", map[string]interface{}{
		"file": "x.go", "instruction": "do it",
	})
	require.NoError(t, err)
	resp := res.(*Response)
	assert.Equal(t, marker, resp.Content,
		"GREEN: handleEdit must surface the real claude binary stdout")
}

// TestPin_AbsentBinary_HonestError asserts that when the claude binary cannot
// be resolved, the handler returns an honest error — NEVER a fabricated
// success response (the §11.4 / BLUFF-001 contract).
func TestPin_AbsentBinary_HonestError(t *testing.T) {
	if isRedMode() {
		t.Skip("SKIP-OK: absent-binary honest-error behaviour only exists post-fix")
	}
	cc := New()
	ctx := context.Background()
	tempDir := t.TempDir()
	require.NoError(t, cc.Initialize(ctx, &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}))

	// Point the override at a path that does not exist.
	t.Setenv("CLAUDE_BIN", filepath.Join(tempDir, "definitely-not-a-binary"))

	_, err := cc.Execute(ctx, "chat", map[string]interface{}{"message": "hello"})
	require.Error(t, err, "absent claude binary must yield an honest error, never a fabricated response")
}
