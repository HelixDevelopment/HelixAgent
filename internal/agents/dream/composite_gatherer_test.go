// Round-80 §11.4 anti-bluff: tests for CompositeSignalGatherer.
//
// Coverage scope (per round-80 brief):
//   - No-children construction → sentinel.
//   - All-children-succeed → aggregated signals.
//   - Mixed children (some err) → partial-success + errors.Join.
//   - Context cancellation honoured (pre- and intra-walk).
//   - Concurrency-safe under -race.
//   - Round-80 sentinels distinct from round-31 ErrGatherersNotWired.

package dream

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFixedSignalsGatherer is a CONST-050(A)-compliant unit-test
// stub that emits a configurable []Signal slice and optional error.
// Production code uses FilesystemRecentEditsGatherer +
// ProcessMemoryUsageGatherer; tests use this stub to assert
// composite-layer fan-in mechanics in isolation from real I/O.
type fakeFixedSignalsGatherer struct {
	id        string
	signals   []Signal
	err       error
	gatherCnt atomic.Int32
	sleep     time.Duration
}

func (f *fakeFixedSignalsGatherer) Gather(ctx context.Context, session *DreamSession) error {
	f.gatherCnt.Add(1)
	if f.sleep > 0 {
		select {
		case <-time.After(f.sleep):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if session.Metadata == nil {
		session.Metadata = map[string]interface{}{}
	}
	existing := extractSignalsSlice(session.Metadata)
	session.Metadata["signals"] = append(existing, f.signals...)
	return f.err
}

func TestCompositeSignalGatherer_NoChildren_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	c := NewCompositeSignalGatherer()
	err := c.Gather(context.Background(), &DreamSession{Metadata: map[string]interface{}{}})
	require.Error(t, err, "empty composite must surface ErrCompositeGathererEmpty — silent success would reproduce the round-29 PASS-bluff at the composite layer")
	require.ErrorIs(t, err, ErrCompositeGathererEmpty)
}

func TestCompositeSignalGatherer_NilChildrenFiltered(t *testing.T) {
	t.Parallel()
	c := NewCompositeSignalGatherer(nil, nil, nil)
	err := c.Gather(context.Background(), &DreamSession{Metadata: map[string]interface{}{}})
	require.ErrorIs(t, err, ErrCompositeGathererEmpty,
		"nil-only children must be filtered, collapsing to the empty-composite case")
}

func TestCompositeSignalGatherer_AllChildrenSucceed_AggregatesSignals(t *testing.T) {
	t.Parallel()
	now := time.Now()
	g1 := &fakeFixedSignalsGatherer{
		id:      "g1",
		signals: []Signal{{Type: "file_edit", Source: "/tmp/a", Timestamp: now, Value: int64(100)}},
	}
	g2 := &fakeFixedSignalsGatherer{
		id: "g2",
		signals: []Signal{
			{Type: "memory_usage", Source: "runtime.Alloc", Timestamp: now, Value: int64(2048)},
			{Type: "memory_usage", Source: "runtime.Sys", Timestamp: now, Value: int64(8192)},
		},
	}
	c := NewCompositeSignalGatherer(g1, g2)

	session := &DreamSession{Metadata: map[string]interface{}{}}
	err := c.Gather(context.Background(), session)
	require.NoError(t, err)

	require.Equal(t, int32(1), g1.gatherCnt.Load())
	require.Equal(t, int32(1), g2.gatherCnt.Load())

	signals, ok := session.Metadata["signals"].([]Signal)
	require.True(t, ok, "session.Metadata[signals] must be []Signal — wrong type means the composite's contract is broken")
	require.Len(t, signals, 3, "composite must aggregate every successful child's signals")

	// Anti-bluff: assert signal content matches what children emitted
	// (not just length — length alone could be a counter bluff).
	sources := make(map[string]bool)
	for _, s := range signals {
		sources[s.Source] = true
	}
	require.True(t, sources["/tmp/a"])
	require.True(t, sources["runtime.Alloc"])
	require.True(t, sources["runtime.Sys"])
}

func TestCompositeSignalGatherer_SomeChildrenError_PartialSuccess(t *testing.T) {
	t.Parallel()
	now := time.Now()
	upstream := errors.New("upstream 503")
	g1 := &fakeFixedSignalsGatherer{
		id:      "g1",
		signals: []Signal{{Type: "file_edit", Source: "/tmp/a", Timestamp: now, Value: int64(100)}},
	}
	g2 := &fakeFixedSignalsGatherer{
		id:  "g2-erroring",
		err: upstream,
	}
	g3 := &fakeFixedSignalsGatherer{
		id:      "g3",
		signals: []Signal{{Type: "memory_usage", Source: "runtime.Alloc", Timestamp: now, Value: int64(2048)}},
	}
	c := NewCompositeSignalGatherer(g1, g2, g3)

	session := &DreamSession{Metadata: map[string]interface{}{}}
	err := c.Gather(context.Background(), session)
	require.Error(t, err, "errored child must propagate")
	require.ErrorIs(t, err, upstream, "errors.Join must preserve the upstream error so callers can ErrorIs-check it")

	// Anti-bluff: successful children's signals MUST still reach
	// session — partial-success is the whole point.
	signals, ok := session.Metadata["signals"].([]Signal)
	require.True(t, ok)
	require.Len(t, signals, 2, "g1 + g3 signals must still aggregate despite g2 erroring; a single child failure must not erase real evidence from siblings — that would be a §11.4 PASS-bluff inversion")
	sources := map[string]bool{}
	for _, s := range signals {
		sources[s.Source] = true
	}
	require.True(t, sources["/tmp/a"])
	require.True(t, sources["runtime.Alloc"])
}

func TestCompositeSignalGatherer_HonoursContextCancel_PreLaunch(t *testing.T) {
	t.Parallel()
	g := &fakeFixedSignalsGatherer{id: "g", signals: []Signal{{Type: "x", Source: "y", Timestamp: time.Now()}}}
	c := NewCompositeSignalGatherer(g)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE invoking

	session := &DreamSession{Metadata: map[string]interface{}{}}
	err := c.Gather(ctx, session)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, int32(0), g.gatherCnt.Load(),
		"pre-cancelled context must short-circuit before any child runs")
}

func TestCompositeSignalGatherer_HonoursContextCancel_DuringChildren(t *testing.T) {
	t.Parallel()
	slow := &fakeFixedSignalsGatherer{
		id:      "slow",
		signals: []Signal{{Type: "x", Source: "y", Timestamp: time.Now()}},
		sleep:   500 * time.Millisecond,
	}
	c := NewCompositeSignalGatherer(slow)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	session := &DreamSession{Metadata: map[string]interface{}{}}
	start := time.Now()
	err := c.Gather(ctx, session)
	elapsed := time.Since(start)

	require.Error(t, err, "in-flight child must observe ctx cancellation and propagate the error")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, elapsed, 400*time.Millisecond,
		"composite must not block past child sleep — child's select{} must unblock on ctx.Done")
}

func TestCompositeSignalGatherer_Concurrent_NoRaces(t *testing.T) {
	t.Parallel()
	now := time.Now()
	children := []SignalGatherer{}
	for i := 0; i < 16; i++ {
		i := i
		children = append(children, &fakeFixedSignalsGatherer{
			id: fmt.Sprintf("g%d", i),
			signals: []Signal{
				{Type: "memory_usage", Source: fmt.Sprintf("child-%d", i), Timestamp: now, Value: int64(i)},
			},
		})
	}
	c := NewCompositeSignalGatherer(children...)

	// Hammer the composite from N goroutines simultaneously with
	// distinct sessions. -race must report no data races.
	var wg sync.WaitGroup
	const goroutines = 8
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < 4; r++ {
				session := &DreamSession{Metadata: map[string]interface{}{}}
				err := c.Gather(context.Background(), session)
				assert.NoError(t, err)
				signals := extractSignalsSlice(session.Metadata)
				assert.Len(t, signals, 16, "every concurrent invocation must receive every child's signal")
			}
		}()
	}
	wg.Wait()
}

func TestCompositeSignalGatherer_ChildPanic_RecoveredAndReported(t *testing.T) {
	t.Parallel()
	now := time.Now()
	good := &fakeFixedSignalsGatherer{
		id:      "good",
		signals: []Signal{{Type: "memory_usage", Source: "ok", Timestamp: now, Value: int64(42)}},
	}
	panicker := &panickingGatherer{}
	c := NewCompositeSignalGatherer(good, panicker)

	session := &DreamSession{Metadata: map[string]interface{}{}}
	err := c.Gather(context.Background(), session)
	require.Error(t, err, "a panicking child must be recovered and surfaced as an error — silent swallow would be a §11.4 contract bluff")
	require.Contains(t, err.Error(), "panicked")

	// Anti-bluff: the good child's signal must still be visible —
	// a sibling's panic must not erase real evidence.
	signals := extractSignalsSlice(session.Metadata)
	require.Len(t, signals, 1)
	require.Equal(t, "ok", signals[0].Source)
}

type panickingGatherer struct{}

func (p *panickingGatherer) Gather(ctx context.Context, session *DreamSession) error {
	panic("deliberate test panic")
}

// TestRound80Sentinels_DistinctFromRound31 asserts the paired-
// mutation distinctness invariant: the new sentinels MUST NOT
// collapse to ErrGatherersNotWired (round-31), otherwise an
// ErrorIs check intended to detect "no SignalGatherer wired" would
// false-fire on "empty composite" / "filesystem mis-config".
func TestRound80Sentinels_DistinctFromRound31(t *testing.T) {
	t.Parallel()
	require.False(t, errors.Is(ErrCompositeGathererEmpty, ErrGatherersNotWired),
		"ErrCompositeGathererEmpty must not be Is-equal to ErrGatherersNotWired — distinct failure modes")
	require.False(t, errors.Is(ErrFilesystemPathsEmpty, ErrGatherersNotWired),
		"ErrFilesystemPathsEmpty must not be Is-equal to ErrGatherersNotWired")
	require.False(t, errors.Is(ErrFilesystemPathInvalid, ErrGatherersNotWired),
		"ErrFilesystemPathInvalid must not be Is-equal to ErrGatherersNotWired")
	require.False(t, errors.Is(ErrProcessMemoryReadFailed, ErrGatherersNotWired),
		"ErrProcessMemoryReadFailed must not be Is-equal to ErrGatherersNotWired")

	// Also assert pairwise distinctness among the new sentinels.
	pairs := []struct {
		a, b error
		name string
	}{
		{ErrCompositeGathererEmpty, ErrFilesystemPathsEmpty, "composite-empty vs fs-empty"},
		{ErrCompositeGathererEmpty, ErrFilesystemPathInvalid, "composite-empty vs fs-invalid"},
		{ErrCompositeGathererEmpty, ErrProcessMemoryReadFailed, "composite-empty vs proc-mem-failed"},
		{ErrFilesystemPathsEmpty, ErrFilesystemPathInvalid, "fs-empty vs fs-invalid"},
		{ErrFilesystemPathsEmpty, ErrProcessMemoryReadFailed, "fs-empty vs proc-mem-failed"},
		{ErrFilesystemPathInvalid, ErrProcessMemoryReadFailed, "fs-invalid vs proc-mem-failed"},
	}
	for _, p := range pairs {
		require.False(t, errors.Is(p.a, p.b), "round-80 sentinels must be pairwise distinct: %s", p.name)
	}
}
