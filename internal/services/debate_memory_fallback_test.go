package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	helixmem "dev.helix.agent/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDebateMemoryFallbackPath_ReturnsWritableDurablePath asserts the wiring-side
// path resolver hands back a non-empty, writable file path under a durable base
// dir (OS user-cache or temp), creating the parent dir. This path is what the
// DebateService uses to make memory recall survive a restart OUT OF THE BOX when
// the HelixMemory fusion engine is unavailable.
func TestDebateMemoryFallbackPath_ReturnsWritableDurablePath(t *testing.T) {
	path := debateMemoryFallbackPath()
	require.NotEmpty(t, path, "fallback path must resolve to a writable durable location out of the box")

	// Parent dir must exist (the resolver MkdirAll's it).
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	require.NoError(t, err, "fallback parent dir must exist after resolution")
	require.True(t, info.IsDir())

	// CONST-051 decoupling: the path is namespaced by the module's OWN name, never
	// a hardcoded consuming-project (HelixCode) path.
	assert.Contains(t, path, "helixagent")
	assert.True(t, strings.HasSuffix(path, ".db"), "fallback target is a sqlite db file")

	// Writability proof: a real file can be created at the resolved location.
	require.NoError(t, os.WriteFile(path+".probe", []byte("ok"), 0o600))
	_ = os.Remove(path + ".probe")
}

// TestDebateMemoryFallbackPath_DurableRecallSurvivesRestart is the load-bearing
// out-of-the-box durability proof for the DebateService fallback: it persists a
// memory through a DiskStore opened at the EXACT path production wiring chooses,
// closes it (simulating process exit), reopens a BRAND-NEW DiskStore on that same
// resolved path, and asserts the memory is RECALLED. The pre-fix in-memory
// fallback loses the record here; the disk-durable fallback survives.
func TestDebateMemoryFallbackPath_DurableRecallSurvivesRestart(t *testing.T) {
	// Redirect the OS cache dir to a temp location so the test never touches the
	// real per-user cache and self-cleans. os.UserCacheDir honors XDG_CACHE_HOME
	// on Linux and HOME on macOS; set both to be portable.
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("HOME", tmp)

	path := debateMemoryFallbackPath()
	require.NotEmpty(t, path)

	ctx := context.Background()

	// --- Session 1: persist through the production-chosen path, then "exit". ---
	store1, err := helixmem.NewDiskStore(path)
	require.NoError(t, err)
	require.NoError(t, store1.Add(ctx, &helixmem.Memory{
		ID:      "debate-durable-1",
		UserID:  "operator",
		Content: "the user prefers concise answers",
		Type:    helixmem.MemoryTypeSemantic,
	}))
	require.NoError(t, store1.Close())

	// --- Session 2: fresh store on the SAME resolved path (a "restart"). ---
	store2, err := helixmem.NewDiskStore(path)
	require.NoError(t, err)
	defer func() { _ = store2.Close() }()

	got, err := store2.Get(ctx, "debate-durable-1")
	require.NoError(t, err, "memory MUST be recalled after restart via the fallback path — out-of-the-box durability")
	assert.Equal(t, "the user prefers concise answers", got.Content)
	assert.Equal(t, "operator", got.UserID)
}
