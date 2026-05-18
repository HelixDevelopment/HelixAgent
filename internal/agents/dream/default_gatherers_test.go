// Round-80 §11.4 anti-bluff: tests for FilesystemRecentEditsGatherer
// + ProcessMemoryUsageGatherer + end-to-end composite dispatch.
//
// Anti-bluff posture: every test exercises REAL filesystem mtimes
// and REAL runtime.MemStats samples. Zero hardcoded byte counts,
// zero synthetic mtimes. A passing test proves the gatherers
// observe the live system as their data source.

package dream

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// FilesystemRecentEditsGatherer
// ---------------------------------------------------------------------------

func TestFilesystemRecentEdits_EmptyPaths_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	_, err := NewFilesystemRecentEditsGatherer(FilesystemRecentEditsConfig{})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrFilesystemPathsEmpty)
}

func TestFilesystemRecentEdits_PathNotExist_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	bogus := filepath.Join(t.TempDir(), "does-not-exist-"+t.Name())
	g, err := NewFilesystemRecentEditsGatherer(FilesystemRecentEditsConfig{
		WatchPaths: []string{bogus},
	})
	require.NoError(t, err, "construction succeeds; the missing path surfaces at Gather() time")

	err = g.Gather(context.Background(), &DreamSession{Metadata: map[string]interface{}{}})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrFilesystemPathInvalid,
		"non-existent watch path must surface ErrFilesystemPathInvalid — silently producing zero signals would be a §11.4 PASS-bluff")
}

func TestFilesystemRecentEdits_PathIsFile_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "regular-file.txt")
	require.NoError(t, os.WriteFile(file, []byte("hi"), 0o644))

	g, err := NewFilesystemRecentEditsGatherer(FilesystemRecentEditsConfig{
		WatchPaths: []string{file},
	})
	require.NoError(t, err)

	err = g.Gather(context.Background(), &DreamSession{Metadata: map[string]interface{}{}})
	require.ErrorIs(t, err, ErrFilesystemPathInvalid)
}

func TestFilesystemRecentEdits_GathersRecentlyModified(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create 3 real files with current mtime.
	for _, name := range []string{"a.txt", "b.md", "c.go"} {
		full := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(full, []byte("real content for "+name), 0o644))
	}

	g, err := NewFilesystemRecentEditsGatherer(FilesystemRecentEditsConfig{
		WatchPaths: []string{dir},
		MaxAge:     1 * time.Hour,
	})
	require.NoError(t, err)

	session := &DreamSession{Metadata: map[string]interface{}{}}
	require.NoError(t, g.Gather(context.Background(), session))

	signals := extractSignalsSlice(session.Metadata)
	require.Len(t, signals, 3, "every recently-written file must yield a Signal — anti-bluff: mtime is read from the live filesystem, not faked")

	// Verify Signal fields point to real filesystem evidence.
	sources := map[string]bool{}
	for _, s := range signals {
		require.Equal(t, "file_edit", s.Type)
		require.False(t, s.Timestamp.IsZero(), "Signal.Timestamp MUST be the file's real mtime, not zero — a zero timestamp would be the synthesis bluff")
		require.WithinDuration(t, time.Now(), s.Timestamp, 30*time.Second,
			"Signal.Timestamp must be close to now for a just-written file — wide drift means we're not reading the real mtime")
		// Source must point at one of the real created files.
		sources[filepath.Base(s.Source)] = true
		// Value contains real size_bytes from os.FileInfo.
		valueMap, ok := s.Value.(map[string]interface{})
		require.True(t, ok, "Value must be a map keyed by metadata field")
		size, ok := valueMap["size_bytes"].(int64)
		require.True(t, ok)
		require.Greater(t, size, int64(0), "size_bytes must be the real file size — a zero would mean we're not stat-ing the live file")
	}
	require.True(t, sources["a.txt"])
	require.True(t, sources["b.md"])
	require.True(t, sources["c.go"])
}

func TestFilesystemRecentEdits_RespectsMaxAge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	recent := filepath.Join(dir, "recent.txt")
	old := filepath.Join(dir, "old.txt")
	require.NoError(t, os.WriteFile(recent, []byte("recent"), 0o644))
	require.NoError(t, os.WriteFile(old, []byte("old"), 0o644))

	// Force "old" mtime to past via os.Chtimes (real syscall —
	// not a synthetic shortcut).
	pastTime := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(old, pastTime, pastTime))

	g, err := NewFilesystemRecentEditsGatherer(FilesystemRecentEditsConfig{
		WatchPaths: []string{dir},
		MaxAge:     1 * time.Hour,
	})
	require.NoError(t, err)

	session := &DreamSession{Metadata: map[string]interface{}{}}
	require.NoError(t, g.Gather(context.Background(), session))

	signals := extractSignalsSlice(session.Metadata)
	require.Len(t, signals, 1, "MaxAge filter must exclude old.txt — including it would mean the gatherer fails to honour its own configured bound")
	require.Equal(t, "recent.txt", filepath.Base(signals[0].Source))
}

func TestFilesystemRecentEdits_MaxResultsTruncatesByMtimeDesc(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create 5 files; the gatherer is configured to keep only 2.
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, "f"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(name, []byte("x"), 0o644))
		// Stagger mtimes deterministically so we can assert the
		// truncation picked the freshest two.
		mt := time.Now().Add(-time.Duration(i) * time.Minute)
		require.NoError(t, os.Chtimes(name, mt, mt))
	}

	g, err := NewFilesystemRecentEditsGatherer(FilesystemRecentEditsConfig{
		WatchPaths: []string{dir},
		MaxAge:     1 * time.Hour,
		MaxResults: 2,
	})
	require.NoError(t, err)

	session := &DreamSession{Metadata: map[string]interface{}{}}
	require.NoError(t, g.Gather(context.Background(), session))

	signals := extractSignalsSlice(session.Metadata)
	require.Len(t, signals, 2)
	// Freshest first.
	require.Equal(t, "f0.txt", filepath.Base(signals[0].Source))
	require.Equal(t, "f1.txt", filepath.Base(signals[1].Source))
}

func TestFilesystemRecentEdits_HonoursContextCancel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "f"+string(rune('0'+i))+".txt"), []byte("x"), 0o644))
	}
	g, err := NewFilesystemRecentEditsGatherer(FilesystemRecentEditsConfig{
		WatchPaths: []string{dir},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = g.Gather(ctx, &DreamSession{Metadata: map[string]interface{}{}})
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

// ---------------------------------------------------------------------------
// ProcessMemoryUsageGatherer
// ---------------------------------------------------------------------------

func TestProcessMemoryUsage_Gather_ReturnsRealBytes(t *testing.T) {
	t.Parallel()
	g := NewProcessMemoryUsageGatherer(ProcessMemoryUsageConfig{})

	session := &DreamSession{Metadata: map[string]interface{}{}}
	require.NoError(t, g.Gather(context.Background(), session))

	signals := extractSignalsSlice(session.Metadata)
	require.NotEmpty(t, signals, "runtime.MemStats sample must always succeed → ≥1 Signal expected")

	// Anti-bluff: at least one Signal must have a non-zero positive
	// Value sourced from runtime — a zero everywhere would mean the
	// gatherer is hardcoding sentinels instead of sampling reality.
	foundReal := false
	for _, s := range signals {
		require.Equal(t, "memory_usage", s.Type)
		require.False(t, s.Timestamp.IsZero())
		v, ok := s.Value.(int64)
		require.True(t, ok, "memory Value must be int64; got %T", s.Value)
		if v > 0 {
			foundReal = true
		}
	}
	require.True(t, foundReal, "at least one Signal.Value must be > 0 — every-Value-zero would mean we're not sampling the real Go runtime memstats (§11.4 anti-bluff invariant)")
}

func TestProcessMemoryUsage_Gather_ProcStatusAugmentation_Linux(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("SKIP-OK: /proc/self/status is Linux-only — skipping on " + runtime.GOOS)
	}
	g := NewProcessMemoryUsageGatherer(ProcessMemoryUsageConfig{IncludeProcStatus: true})
	session := &DreamSession{Metadata: map[string]interface{}{}}
	require.NoError(t, g.Gather(context.Background(), session))

	signals := extractSignalsSlice(session.Metadata)
	// At least one signal must come from /proc/self/status when on
	// Linux + IncludeProcStatus=true.
	foundProc := false
	for _, s := range signals {
		if len(s.Source) > 0 && s.Source[:1] == "/" {
			foundProc = true
		}
	}
	require.True(t, foundProc, "Linux + IncludeProcStatus=true must yield ≥1 Signal sourced from /proc/self/status")
}

func TestProcessMemoryUsage_Gather_HonoursContextCancel(t *testing.T) {
	t.Parallel()
	g := NewProcessMemoryUsageGatherer(ProcessMemoryUsageConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := g.Gather(ctx, &DreamSession{Metadata: map[string]interface{}{}})
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

// ---------------------------------------------------------------------------
// End-to-end: Composite + 2 defaults wired into a real Dreamer
// ---------------------------------------------------------------------------

// TestDreamGatherPhase_WithCompositeGatherer_AggregatesFromAllSources
// is the round-80 end-to-end proof: a CompositeSignalGatherer
// containing the 2 default concrete gatherers, wired into a real
// Dreamer, produces a non-empty []Signal during gatherPhase. This
// is the bluff-killing test — it would fail under any implementation
// that no-ops or hardcodes the gather output.
func TestDreamGatherPhase_WithCompositeGatherer_AggregatesFromAllSources(t *testing.T) {
	t.Parallel()

	// Real filesystem: create a temp dir with a real file.
	dir := t.TempDir()
	realFile := filepath.Join(dir, "evidence.txt")
	require.NoError(t, os.WriteFile(realFile, []byte("end-to-end real content"), 0o644))

	// Construct the 2 default gatherers — real concrete impls.
	fsGatherer, err := NewFilesystemRecentEditsGatherer(FilesystemRecentEditsConfig{
		WatchPaths: []string{dir},
		MaxAge:     1 * time.Hour,
	})
	require.NoError(t, err)
	memGatherer := NewProcessMemoryUsageGatherer(ProcessMemoryUsageConfig{})

	// Compose + wire into a real Dreamer.
	composite := NewCompositeSignalGatherer(fsGatherer, memGatherer)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	dreamer := NewDreamer(DefaultConfig(), logger)
	dreamer.AddGatherer(composite)

	// Invoke gatherPhase directly — round-31 contract.
	session := &DreamSession{
		ID:       "e2e-round-80",
		Metadata: map[string]interface{}{},
	}
	require.NoError(t, dreamer.gatherPhase(context.Background(), session))

	// Anti-bluff: assert the gathered Signals contain real evidence
	// from BOTH sources — proves the composite fan-out actually
	// reached both children.
	signals := extractSignalsSlice(session.Metadata)
	require.NotEmpty(t, signals, "gatherPhase wired with composite + 2 defaults MUST produce signals — empty would mean the composite ran but emitted nothing real")

	var sawFileEdit, sawMemoryUsage bool
	for _, s := range signals {
		switch s.Type {
		case "file_edit":
			sawFileEdit = true
			// Must point at the REAL file we created.
			require.Equal(t, realFile, s.Source, "file_edit Signal.Source must be the literal path of the real file we created — any other value means the gatherer fabricated the source")
		case "memory_usage":
			sawMemoryUsage = true
			v, ok := s.Value.(int64)
			require.True(t, ok)
			require.GreaterOrEqual(t, v, int64(0))
		}
	}
	require.True(t, sawFileEdit, "composite must surface filesystem-edits Signal — absent means the FilesystemRecentEditsGatherer never ran (§11.4 PASS-bluff)")
	require.True(t, sawMemoryUsage, "composite must surface memory-usage Signal — absent means the ProcessMemoryUsageGatherer never ran (§11.4 PASS-bluff)")
}

// TestDreamGatherPhase_WithEmptyComposite_SurfacesCompositeSentinel
// asserts the composition: an empty composite wired into a Dreamer
// surfaces ErrCompositeGathererEmpty (NOT round-31
// ErrGatherersNotWired, because the composite IS wired even if it
// has no children).
func TestDreamGatherPhase_WithEmptyComposite_SurfacesCompositeSentinel(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	dreamer := NewDreamer(DefaultConfig(), logger)
	dreamer.AddGatherer(NewCompositeSignalGatherer()) // empty composite

	session := &DreamSession{ID: "empty-composite", Metadata: map[string]interface{}{}}
	err := dreamer.gatherPhase(context.Background(), session)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrCompositeGathererEmpty,
		"empty composite must surface ErrCompositeGathererEmpty (round-80) — NOT collapse to ErrGatherersNotWired (round-31). Conflating them would mean operators can't distinguish 'forgot to wire a gatherer' from 'wired an empty composite'.")

	// Crucially: must NOT also wrap round-31 sentinel.
	require.False(t, errors.Is(err, ErrGatherersNotWired),
		"empty-composite case must NOT wrap round-31 ErrGatherersNotWired — distinct failure modes")
}
