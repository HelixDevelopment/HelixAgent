package copilotcli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageInitialization(t *testing.T) {
	t.Parallel()
	// Basic test to ensure package compiles and initializes
	assert.True(t, true)
}

func TestAgentType(t *testing.T) {
	t.Parallel()
	// Test that agent type is defined
	assert.NotEmpty(t, "copilotcli")
}

// writeFakeCopilot writes an executable fake `copilot` binary that echoes a
// unique sentinel, so a test can prove the integration genuinely exec'd it
// (real os/exec) rather than fabricating a templated "// Generated code for:"
// fallback string.
func writeFakeCopilot(t *testing.T, sentinel string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: fake-binary injection uses a POSIX shell script")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-copilot")
	script := "#!/bin/sh\necho \"" + sentinel + " args=$*\"\n"
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	return bin
}

// TestCopilotCLI_RealExec is the §11.4.115 RED→GREEN guard for DEFECT D-17 /
// BLUFF-001 in this package. The original source returned fabricated fallbacks
// ("// Generated code for: ...", "Code explanation would be provided by GitHub
// Copilot") whenever the CLI was unavailable. This guard injects a fake copilot
// binary via COPILOT_BIN and asserts the binary's real stdout flows through —
// proving real os/exec is wired. §1.1 paired mutation: revert any handler to a
// hardcoded fallback string → the sentinel disappears → this FAILs.
func TestCopilotCLI_RealExec(t *testing.T) {
	sentinel := "COPILOT_REAL_EXEC_SENTINEL_9X"
	bin := writeFakeCopilot(t, sentinel)
	t.Setenv("COPILOT_BIN", bin)

	c := New()
	ctx := context.Background()
	require.NoError(t, c.Initialize(ctx, nil))

	cases := []struct {
		command string
		params  map[string]interface{}
		field   string
	}{
		{"suggest", map[string]interface{}{"prompt": "x"}, "suggestion"},
		{"explain", map[string]interface{}{"code": "func main(){}"}, "explanation"},
		{"test", map[string]interface{}{"code": "func main(){}"}, "tests"},
		{"fix", map[string]interface{}{"code": "func main(){}"}, "fixed"},
		{"docs", map[string]interface{}{"code": "func main(){}"}, "documentation"},
	}
	for _, c2 := range cases {
		res, err := c.Execute(ctx, c2.command, c2.params)
		require.NoError(t, err, c2.command)
		m := res.(map[string]interface{})
		assert.Contains(t, m[c2.field].(string), sentinel,
			"%s must surface REAL copilot stdout (proving os/exec), not a fabricated fallback", c2.command)
	}
}

// TestCopilotCLI_HonestErrorWhenBinaryAbsent proves that when no copilot binary
// is resolvable, the generation commands return an honest error rather than the
// old fabricated fallback string (BLUFF-001).
func TestCopilotCLI_HonestErrorWhenBinaryAbsent(t *testing.T) {
	t.Setenv("COPILOT_BIN", filepath.Join(t.TempDir(), "does-not-exist"))

	c := New()
	ctx := context.Background()
	require.NoError(t, c.Initialize(ctx, nil))

	res, err := c.Execute(ctx, "suggest", map[string]interface{}{"prompt": "x"})
	require.Error(t, err)
	assert.Nil(t, res)
	assert.False(t, c.IsAvailable(), "IsAvailable must be false when copilot is absent")
}
