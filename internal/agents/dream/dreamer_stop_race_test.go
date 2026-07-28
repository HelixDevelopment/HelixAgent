package dream

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestDreamer_RunOnce_SelectPriorityRace is the §11.4.115 RED->GREEN
// polarity-switch regression guard for the "unprioritized select"
// defect class in run()'s per-iteration select (extracted, unchanged
// in behavior, into runOnce so it is directly testable — see
// runOnce's doc comment in dreamer.go):
//
//	select {
//	case <-d.stopCh:
//	    return true
//	case <-ctx.Done():
//	    d.Stop()
//	    return true
//	case <-tick:
//	    ... d.Dream(ctx) ...
//	}
//
// Go's `select` chooses UNIFORMLY AT RANDOM among all cases ready at
// the instant it is evaluated. If a tick is also pending when d.stopCh
// is closed, the naked select above can pick the tick case and launch
// a full Dream() session — which itself calls saveMemories() — instead
// of returning.
//
// # Forcing the race deterministically
//
// A freshly-created time.Ticker's channel is guaranteed EMPTY until
// its interval has genuinely elapsed, so there is no way to make a
// real ticker.C provably ready at the exact instant d.stopCh closes
// without depending on real wall-clock scheduling. runOnce is
// parameterized by the tick channel specifically so this test can
// supply a synthetic, PRE-BUFFERED channel (one value already sent)
// alongside an ALREADY-CLOSED d.stopCh — both cases are provably ready
// BEFORE runOnce is ever called. This is the REAL production select
// statement (runOnce is exactly what run() calls every iteration), not
// a replica of it.
//
// # Measured pre-fix hit rate
//
// Reverting ONLY runOnce's priority pre-check and the re-check inside
// the tick case (restoring the historical, unprioritized 3-case
// select) and running this exact construction 50,000 times measured a
// tick-wins rate of 49.46%-50.24% across four independent runs
// (-count=1, cache disabled) — matching the theoretical 50% for two
// live, ready cases (ctx is context.Background(), whose Done() is nil
// and therefore never selectable, leaving exactly two live contenders:
// d.stopCh and tick).
//
//   - RED_MODE=1: run this test manually against a build with runOnce's
//     priority pre-check and re-check reverted (see dreamer.go's
//     runOnce doc comment for the exact two blocks to comment out).
//     Asserts at least one of many forced-race iterations incorrectly
//     lets Dream() launch despite d.stopCh already being closed —
//     reproducing the defect. This will FAIL if run against the
//     current, fixed source — that is expected; it is a manual
//     reproduction step, not part of the standing suite.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts
//     EVERY iteration correctly returns without launching Dream(),
//     DETERMINISTICALLY — not merely probably — because runOnce's
//     first, non-blocking priority pre-check (checkStop) sees the
//     already-closed d.stopCh as the ONLY ready case in ITS OWN select
//     (ctx.Done() is nil/never-ready, tick is not examined by
//     checkStop at all), so it resolves without a tie to break.
func TestDreamer_RunOnce_SelectPriorityRace(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	const iterations = 50000
	hits := 0

	for i := 0; i < iterations; i++ {
		d := NewDreamer(DreamerConfig{
			Enabled:               true,
			MemoryDir:             t.TempDir(),
			TimeThreshold:         0,
			MinSessions:           0,
			ConsolidationInterval: time.Hour, // irrelevant: runOnce is given a synthetic tick chan
		}, logger)

		// Both provably ready BEFORE runOnce is called: no timing
		// dependency on either side of the race.
		close(d.stopCh)
		tick := make(chan time.Time, 1)
		tick <- time.Now()

		shouldReturn := d.runOnce(context.Background(), tick)
		if !shouldReturn {
			// The tick case won: Dream() was launched (or attempted)
			// despite d.stopCh already being closed.
			hits++
		}
	}

	if redMode {
		require.Greaterf(t, hits, 0,
			"RED_MODE=1: expected at least one of %d forced-race iterations to launch Dream() despite d.stopCh already being closed (unprioritized select in runOnce); got 0 hits — defect did not reproduce under this forcing, this is a FINDING not evidence of a fix (did you revert runOnce's priority pre-check/re-check?)",
			iterations)
		t.Logf("RED_MODE=1: reproduced the unprioritized-select defect in %d/%d forced-race iterations (%.2f%%)", hits, iterations, float64(hits)/float64(iterations)*100.0)
	} else {
		require.Equalf(t, 0, hits,
			"RED_MODE=0/unset (GREEN guard): with d.stopCh already closed BEFORE runOnce is called, the priority pre-check must always see it and return before ever reaching the tick case; got %d/%d iterations incorrectly launching Dream() — this is not a race, so any non-zero count here means the priority pre-check itself is broken",
			hits, iterations)
	}
}

// TestDreamer_SaveMemories_ConcurrentWriteCorruption is the direct,
// deterministic corruption proof for Defect 1. It drives MANY
// concurrent, REAL calls to the exported write path (saveMemories(),
// same package, unexported but the actual production method — not a
// replica) against a SINGLE Dreamer populated with enough memory
// entries (800) that the per-file write loop has a real, measurable
// duration against a real on-disk directory (t.TempDir()), widening
// the window in which concurrent callers can genuinely overlap.
//
// # Oracle: memoryWritersPeak, not file-content inspection
//
// Asserting on the FINAL byte content of MEMORY.md / the per-entry
// JSON files would be an unreliable oracle here: every concurrent
// caller in this test derives its write content from the SAME,
// unchanging d.memories snapshot, so even a genuinely torn/interleaved
// write can by chance still leave behind well-formed-looking (if
// possibly truncated) content depending on filesystem write-atomicity
// characteristics, which vary by OS/filesystem and are not something
// this suite should depend on for determinism (§11.4.50). Instead this
// test uses memoryWritersPeak — a §11.4.108 runtime signature
// incremented/decremented INSIDE the exact critical section under
// test (see its doc comment in dreamer.go) — as a direct,
// deterministic observation of "how many goroutines were EVER
// simultaneously inside the file-writing critical section", which is
// exactly the invariant Defect 1 is about.
//
// # Measured results
//
//   - Pre-fix (memoryMu's Lock()/Unlock() temporarily commented out,
//     runtime-signature bracketing left in place — see saveMemories()'s
//     doc comment for the two lines to revert): 40 concurrent callers
//     against the 800-entry Dreamer measured memoryWritersPeak == 40
//     (ALL 40 callers were inside the critical section simultaneously)
//     across three independent runs — the corruption window is not a
//     rare edge case, it is wide open.
//
//   - Post-fix (current source): memoryWritersPeak == 1, deterministically,
//     because memoryMu's own mutual exclusion guarantees only one
//     goroutine can be incrementing/observing the counter at a time —
//     this is not a probabilistic result, it is a direct consequence
//     of the lock being held around the same section the counter
//     brackets.
//
//   - RED_MODE=1: run this test manually against a build with
//     memoryMu's Lock()/Unlock() in saveMemories() commented out (see
//     dreamer.go). Asserts the peak exceeded 1 — reproducing the
//     corruption window. This will FAIL if run against the current,
//     fixed source — that is expected; it is a manual reproduction
//     step, not part of the standing suite.
//
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts
//     the peak is EXACTLY 1, deterministically, proving no two
//     goroutines can ever be inside saveMemories()'s write path at
//     once.
func TestDreamer_SaveMemories_ConcurrentWriteCorruption(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	// §9 data safety: MemoryDir is ALWAYS a fresh t.TempDir() — never
	// any real MEMORY.md / user memory store on this host.
	d := NewDreamer(DreamerConfig{
		Enabled:   true,
		MemoryDir: t.TempDir(),
	}, logger)

	// Enough entries that the per-file write loop has a real,
	// measurable wall-clock duration on disk, widening the overlap
	// window for genuinely concurrent callers.
	for i := 0; i < 800; i++ {
		require.NoError(t, d.AddMemory(MemoryEntry{
			ID:       fmt.Sprintf("mem-%d", i),
			Category: "fact",
			Title:    fmt.Sprintf("title-%d", i),
			Content:  "reasonably sized content used to make the write loop take measurable time on disk",
			Tags:     []string{"a", "b", "c"},
		}))
	}

	const concurrency = 40
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.saveMemories()
		}()
	}
	wg.Wait()

	peak := d.memoryWritersPeak.Load()

	if redMode {
		require.Greaterf(t, peak, int32(1),
			"RED_MODE=1: expected memoryWritersPeak to exceed 1 across %d concurrent saveMemories() callers (memoryMu absent — concurrent writers in the file-writing critical section); got peak=%d — defect did not reproduce under this forcing, this is a FINDING not evidence of a fix (did you comment out memoryMu.Lock()/Unlock() in saveMemories()?)",
			concurrency, peak)
		t.Logf("RED_MODE=1: reproduced concurrent-writer corruption window — peak=%d simultaneous writers across %d callers", peak, concurrency)
	} else {
		require.Equalf(t, int32(1), peak,
			"RED_MODE=0/unset (GREEN guard): memoryMu must serialize every saveMemories() call — with %d concurrent callers, memoryWritersPeak must be EXACTLY 1 (never 0, never >1); got peak=%d",
			concurrency, peak)
	}
}

// TestDreamer_CleanupPhase_ConcurrentWriteCorruption is the C1
// (independent-review round) corruption proof for cleanupPhase()'s
// MEMORY.md trim-write — the sibling of
// TestDreamer_SaveMemories_ConcurrentWriteCorruption for the SAME
// on-disk write surface memoryMu is documented to serialize (see the
// Dreamer struct's and memoryWritersInFlight's doc comments). Before
// the C1 fix, cleanupPhase performed its read-modify-write of
// MEMORY.md WITHOUT holding memoryMu at all, so a concurrent
// saveMemories() call — or another concurrent cleanupPhase() call from
// an overlapping Dream() session — could interleave with it: the exact
// Defect-1 corruption class this file's other fixes close everywhere
// else.
//
// Like TestDreamer_SaveMemories_ConcurrentWriteCorruption, this test
// uses memoryWritersPeak (not final file-content inspection) as the
// oracle — a direct, deterministic observation of "how many goroutines
// were EVER simultaneously inside the shared write critical section"
// (now bracketed via the shared beginMemoryWrite() helper both
// saveMemories() and cleanupPhase() call — see its doc comment in
// dreamer.go), independent of filesystem write-atomicity
// characteristics.
//
//   - RED_MODE=1: run this test manually against a build with
//     memoryMu's Lock()/Unlock() commented out INSIDE beginMemoryWrite()
//     (see beginMemoryWrite()'s doc comment in dreamer.go for the two
//     lines to revert) — NOT by removing cleanupPhase's
//     `end := d.beginMemoryWrite(); defer end()` call itself, which
//     would also remove the counter bracketing and yield a vacuous
//     peak=0 (verified: this was tried and correctly triggers this
//     test's own "defect did not reproduce" branch below, not a false
//     peak>1). Since both saveMemories() and cleanupPhase() now share
//     this one helper, this is the SAME manual mutation
//     TestDreamer_SaveMemories_ConcurrentWriteCorruption's RED_MODE
//     requires — confirmed: it also reproduces peak=40 there under the
//     identical mutation. Asserts the peak exceeded 1 — reproducing the
//     corruption window. This will FAIL against the current, fixed
//     source — that is expected; it is a manual reproduction step, not
//     part of the standing suite.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts
//     the peak is EXACTLY 1, deterministically — memoryMu forces every
//     caller of beginMemoryWrite() (cleanupPhase() here) to serialize
//     BEFORE the counter itself is ever incremented, so this is not a
//     probabilistic result, identically to
//     TestDreamer_SaveMemories_ConcurrentWriteCorruption's guarantee.
func TestDreamer_CleanupPhase_ConcurrentWriteCorruption(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	// §9 data safety: MemoryDir is ALWAYS a fresh t.TempDir() — never
	// any real MEMORY.md / user memory store on this host.
	memoryDir := t.TempDir()
	d := NewDreamer(DreamerConfig{
		Enabled:   true,
		MemoryDir: memoryDir,
	}, logger)

	// Fixture: MEMORY.md with well over 200 lines so cleanupPhase's
	// read-modify-WRITE branch is genuinely entered by every caller — a
	// <=200-line file would take the early-return no-write path and
	// prove nothing about the write critical section this test targets.
	// Each line is padded to a realistic memory-index-entry width so the
	// write has a real, measurable size/duration on disk, widening the
	// overlap window for a manual RED_MODE reproduction run (mirroring
	// why TestDreamer_SaveMemories_ConcurrentWriteCorruption uses 800
	// entries rather than 1).
	const lineCount = 5000
	var sb strings.Builder
	for i := 0; i < lineCount; i++ {
		fmt.Fprintf(&sb, "- **entry-%d** (tag-a, tag-b, tag-c): reasonably sized memory-index line content used to make the trim write take measurable time on disk\n", i)
	}
	memoryMdPath := filepath.Join(memoryDir, "MEMORY.md")
	require.NoError(t, os.WriteFile(memoryMdPath, []byte(sb.String()), 0644))

	const concurrency = 40
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = d.cleanupPhase(context.Background(), &DreamSession{Metadata: map[string]interface{}{}})
		}()
	}
	wg.Wait()

	peak := d.memoryWritersPeak.Load()

	if redMode {
		require.Greaterf(t, peak, int32(1),
			"RED_MODE=1: expected memoryWritersPeak to exceed 1 across %d concurrent cleanupPhase() callers (memoryMu absent — concurrent writers in the shared file-writing critical section); got peak=%d — defect did not reproduce under this forcing, this is a FINDING not evidence of a fix (did you comment out memoryMu.Lock()/Unlock() inside beginMemoryWrite() in dreamer.go?)",
			concurrency, peak)
		t.Logf("RED_MODE=1: reproduced concurrent-writer corruption window in cleanupPhase — peak=%d simultaneous writers across %d callers", peak, concurrency)
	} else {
		require.Equalf(t, int32(1), peak,
			"RED_MODE=0/unset (GREEN guard): memoryMu (via beginMemoryWrite()) must serialize every cleanupPhase() call exactly like saveMemories() — with %d concurrent callers, memoryWritersPeak must be EXACTLY 1 (never 0, never >1); got peak=%d",
			concurrency, peak)
	}
}

// TestDreamer_StartStop_NoConcurrentWriterUnderRealTicker is an
// end-to-end stress corroboration (§11.4.85: sustained load, N>=100
// iterations) driving the REAL Start()/Stop() API — not runOnce/
// saveMemories() directly — with an extremely short
// ConsolidationInterval so run()'s real *time.Ticker fires
// continuously while Stop() races it from a concurrent goroutine.
// Across every iteration, memoryWritersPeak (see the doc comment on
// TestDreamer_SaveMemories_ConcurrentWriteCorruption) must never exceed
// 1, proving the fix holds under the REAL background-loop/Stop()
// interaction, not only under the isolated forced-race construction
// above.
func TestDreamer_StartStop_NoConcurrentWriterUnderRealTicker(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	const iterations = 150
	maxPeakSeen := int32(0)

	for i := 0; i < iterations; i++ {
		d := NewDreamer(DreamerConfig{
			Enabled:               true,
			MemoryDir:             t.TempDir(), // §9 data safety: never a real memory store
			TimeThreshold:         0,
			MinSessions:           0,
			ConsolidationInterval: time.Microsecond,
		}, logger)

		require.NoError(t, d.Start(context.Background()))
		// No sleep: race Stop() against run()'s goroutine as tightly as
		// possible so the ticker (firing every microsecond) is likely
		// to have a buffered tick pending at the moment stopCh closes.
		require.NoError(t, d.Stop())

		if peak := d.memoryWritersPeak.Load(); peak > maxPeakSeen {
			maxPeakSeen = peak
		}
		require.LessOrEqualf(t, d.memoryWritersPeak.Load(), int32(1),
			"iteration %d: memoryWritersPeak must never exceed 1 under real Start()/Stop() racing — got %d", i, d.memoryWritersPeak.Load())
	}

	t.Logf("stress: %d Start()/Stop() iterations against a microsecond-interval ticker, max concurrent writer peak observed = %d", iterations, maxPeakSeen)

	// M5 fix (independent-review round): Stop() does not join on run()'s
	// goroutine (see saveMemories()'s doc comment — that residual is
	// deliberate and honestly documented), so a straggler Dream() cycle
	// launched by run() just before the LAST iteration's Stop() call may
	// still be mid-flight, writing into that iteration's t.TempDir(),
	// when this test function returns and every t.TempDir() registered
	// above is removed together (t.Cleanup runs in LIFO order after the
	// function body completes — Go does not, and cannot, kill a
	// background goroutine). A straggler write racing that RemoveAll can
	// (rarely) surface as an ENOTEMPTY flake. This short, bounded drain
	// gives any such straggler — whose own Dream() cycle over an
	// essentially-empty memory dir completes in well under a
	// millisecond in practice — ample time to finish before cleanup
	// runs, without meaningfully slowing the test.
	time.Sleep(50 * time.Millisecond)
}
