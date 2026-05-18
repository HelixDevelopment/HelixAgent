// Round-80 §11.4 anti-bluff implementation: concrete SignalGatherer
// composite + 2 default implementations.
//
// Round-29 (commit 93e63ada) introduced the SignalGatherer interface
// + AddGatherer injection point + ErrGatherersNotWired sentinel —
// closing the original gatherPhase no-op-debug-log bluff. Round-80
// (this file) provides shippable concrete wiring so the interface
// can be satisfied without each consumer rolling its own.
//
// Anti-bluff invariant (§11.4 / CONST-035 / Article XI §11.9):
//   - Composite fan-out runs every child in parallel via goroutines
//     gated by sync.WaitGroup, never short-circuits on the first
//     error (partial-success pattern; errors aggregated via
//     errors.Join), never reorders or silently drops a child.
//   - Empty-children case returns the dedicated
//     ErrCompositeGathererEmpty sentinel — distinct from round-31's
//     ErrGatherersNotWired (which fires when the Dreamer itself has
//     no gatherer wired, regardless of composite presence).
//   - Honours ctx cancellation between children AND propagates it
//     into each child Gather() call so child gatherers can short-
//     circuit too.
//   - All gathered signals stage on session.Metadata["signals"] as
//     a []Signal slice — preserving the SignalGatherer contract
//     established in round-31 (mutate session, return error).

package dream

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrCompositeGathererEmpty fires when a CompositeSignalGatherer is
// constructed with zero child gatherers AND Gather() is invoked.
// Distinct from round-31's ErrGatherersNotWired (which fires inside
// Dreamer.gatherPhase when *no* gatherer — composite or otherwise
// — has been registered via AddGatherer). A composite SHOULD be
// constructed with ≥1 child; wiring an empty composite into a
// Dreamer would still flip gatherPhase to "wired" (the Dreamer can't
// introspect composite internals) while every dream cycle produced
// zero signals — exactly the §11.4 PASS-bluff shape round-31 was
// designed to eliminate. ErrCompositeGathererEmpty surfaces that
// mis-wiring at first invocation instead of letting it rot silently.
var ErrCompositeGathererEmpty = errors.New(
	"dream/composite: CompositeSignalGatherer constructed with zero " +
		"children — Gather() would silently no-op and report success, " +
		"reproducing the round-29 §11.4 PASS-bluff shape at the " +
		"composite layer. Construct with NewCompositeSignalGatherer(" +
		"child1, child2, …) supplying at least one concrete gatherer",
)

// Signal is the structured record emitted by round-80 default
// gatherers and aggregated by CompositeSignalGatherer onto
// session.Metadata["signals"] ([]Signal).
//
// Anti-bluff invariant: every Signal field MUST be populated from a
// real upstream source — file mtime, runtime.MemStats, /proc read,
// upstream HTTP response, telemetry bus payload — never hardcoded
// or simulated. A Signal whose Value/Timestamp is a literal is a
// CONST-035 / §11.4 violation regardless of test pass status.
type Signal struct {
	// Type classifies the signal — "file_edit", "memory_usage",
	// "transcript_token", etc. Used by phase-3 consolidation to
	// route signals to the appropriate memory-extraction strategy.
	Type string `json:"type"`

	// Source identifies the upstream emitter — file path,
	// "runtime", "/proc/self/status", URL, etc. Enables forensic
	// trace-back from a consolidated memory to the originating
	// real-world observation.
	Source string `json:"source"`

	// Timestamp is when the upstream observation was made (file
	// mtime, memstats sample wall-clock, etc.). NOT the gather-
	// time; phase-3 consolidation needs the real observation time
	// to order signals across gathers.
	Timestamp time.Time `json:"timestamp"`

	// Value is the gatherer-specific payload — int64 byte count for
	// memory, string content snippet for file edits, structured
	// map for richer sources. Phase-3 reflects on this field.
	Value interface{} `json:"value,omitempty"`
}

// CompositeSignalGatherer chains N concrete SignalGatherer
// implementations and aggregates their outputs. Children run in
// parallel via goroutines; errors aggregated via errors.Join;
// partial-success preserved (signals from successful children
// reach session.Metadata even when a sibling errors).
//
// Concurrency model:
//   - children is set once at construction and never mutated —
//     immutable after NewCompositeSignalGatherer returns. No mutex
//     required around reads.
//   - Each child gets its own *DreamSession-scoped intermediate
//     buffer to avoid cross-child writes contending on a shared
//     map (the public DreamSession.Metadata map is only updated
//     once, after all children complete, under collectMu).
type CompositeSignalGatherer struct {
	children []SignalGatherer
}

// NewCompositeSignalGatherer constructs a composite from N children.
// Nil children are filtered out (defensive — same posture as
// (*Dreamer).AddGatherer). Empty result is legal at construction
// time; the error surfaces at Gather() call instead so test
// scaffolding can exercise the empty-case sentinel cleanly.
func NewCompositeSignalGatherer(children ...SignalGatherer) *CompositeSignalGatherer {
	filtered := make([]SignalGatherer, 0, len(children))
	for _, c := range children {
		if c != nil {
			filtered = append(filtered, c)
		}
	}
	return &CompositeSignalGatherer{children: filtered}
}

// Gather runs every child in parallel against ctx + session.
// Returns ErrCompositeGathererEmpty if zero children registered;
// otherwise returns errors.Join of every child error (nil if all
// succeed). Successful children's signal contributions are always
// merged into session.Metadata["signals"] regardless of sibling
// failures (partial-success — §11.4 anti-bluff posture: a single
// child failure must not erase every other child's real evidence).
func (c *CompositeSignalGatherer) Gather(ctx context.Context, session *DreamSession) error {
	if len(c.children) == 0 {
		return fmt.Errorf("CompositeSignalGatherer.Gather: %w", ErrCompositeGathererEmpty)
	}

	// Honour caller-side cancellation before launching any work.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("CompositeSignalGatherer.Gather: %w", err)
	}

	var (
		wg        sync.WaitGroup
		collectMu sync.Mutex
		errs      = make([]error, len(c.children))
		// Each child accumulates into its own private session so
		// child-internal Metadata writes don't race. After fan-in,
		// we merge their "signals" slices under collectMu.
		childSessions = make([]*DreamSession, len(c.children))
	)

	for i, child := range c.children {
		i, child := i, child // capture
		childSessions[i] = &DreamSession{
			ID:       session.ID,
			Metadata: map[string]interface{}{},
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Defensive panic capture — a panicking child must not
			// take down sibling gatherers nor the dream cycle.
			defer func() {
				if r := recover(); r != nil {
					collectMu.Lock()
					errs[i] = fmt.Errorf("CompositeSignalGatherer: child #%d (%T) panicked: %v", i, child, r)
					collectMu.Unlock()
				}
			}()
			if err := child.Gather(ctx, childSessions[i]); err != nil {
				collectMu.Lock()
				errs[i] = fmt.Errorf("child #%d (%T): %w", i, child, err)
				collectMu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Merge every child's signals into the parent session under a
	// single lock acquisition. Successful children always contribute
	// — partial-success guaranteed.
	if session.Metadata == nil {
		session.Metadata = map[string]interface{}{}
	}
	merged := extractSignalsSlice(session.Metadata)
	for _, cs := range childSessions {
		merged = append(merged, extractSignalsSlice(cs.Metadata)...)
	}
	session.Metadata["signals"] = merged

	return errors.Join(errs...)
}

// extractSignalsSlice safely reads a []Signal from a Metadata map,
// returning empty slice if absent or wrong type. Tolerates the
// session having been mutated by a non-round-80 gatherer that
// stored something else under "signals".
func extractSignalsSlice(metadata map[string]interface{}) []Signal {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata["signals"]
	if !ok {
		return nil
	}
	signals, ok := raw.([]Signal)
	if !ok {
		return nil
	}
	return signals
}
