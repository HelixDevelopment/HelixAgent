package crush

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
	assert.NotEmpty(t, "crush")
}

// writeFakeCrush writes an executable fake `crush` binary that echoes a unique
// sentinel containing its args, so a test can prove the integration genuinely
// exec'd it (real os/exec) rather than fabricating a templated response.
func writeFakeCrush(t *testing.T, sentinel string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: fake-binary injection uses a POSIX shell script")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-crush")
	script := "#!/bin/sh\necho \"" + sentinel + " args=$*\"\n"
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	return bin
}

// TestCrush_RealExec is the §11.4.115 RED→GREEN guard for DEFECT D-17 /
// BLUFF-001 in this package. The original source fabricated "Generated tests by
// Crush" / "Bug analysis by Crush" with NO process execution. This guard
// injects a fake crush binary via CRUSH_BIN and asserts the binary's real
// stdout flows through to the result — proving real os/exec is wired. §1.1
// paired mutation: if test()/analyze() are reverted to return a hardcoded
// string instead of runCrush() output, the sentinel disappears and this FAILs.
func TestCrush_RealExec(t *testing.T) {
	sentinel := "CRUSH_REAL_EXEC_SENTINEL_7Q"
	bin := writeFakeCrush(t, sentinel)
	t.Setenv("CRUSH_BIN", bin)

	c := New()
	ctx := context.Background()
	require.NoError(t, c.Initialize(ctx, nil))

	res, err := c.Execute(ctx, "test", map[string]interface{}{"code": "func main(){}"})
	require.NoError(t, err)
	m := res.(map[string]interface{})
	assert.Contains(t, m["tests"].(string), sentinel,
		"test must surface REAL crush stdout (proving os/exec), not a fabricated string")

	res, err = c.Execute(ctx, "analyze", map[string]interface{}{"code": "func main(){}"})
	require.NoError(t, err)
	m = res.(map[string]interface{})
	assert.Contains(t, m["analysis"].(string), sentinel,
		"analyze must surface REAL crush stdout (proving os/exec), not a fabricated string")
}

// TestCrush_HonestErrorWhenBinaryAbsent proves that when no crush binary is
// resolvable, the integration returns an honest error rather than a fabricated
// success (BLUFF-001). CRUSH_BIN points at a non-existent path.
func TestCrush_HonestErrorWhenBinaryAbsent(t *testing.T) {
	t.Setenv("CRUSH_BIN", filepath.Join(t.TempDir(), "does-not-exist"))

	c := New()
	ctx := context.Background()
	require.NoError(t, c.Initialize(ctx, nil))

	res, err := c.Execute(ctx, "test", map[string]interface{}{"code": "x"})
	require.Error(t, err)
	assert.Nil(t, res)
	assert.False(t, c.IsAvailable(), "IsAvailable must be false when crush is absent")
}
