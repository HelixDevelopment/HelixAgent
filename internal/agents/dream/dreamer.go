// Package dream provides the memory consolidation system
// Inspired by Claude Code's Dream system for self-healing memory
package dream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
)

// ErrGatherersNotWired is returned by gatherPhase (phase 2 of the
// dream cycle) when no SignalGatherer implementations have been
// injected into the Dreamer. Forensic anchor (round-29 §11.4
// audit): prior to the fix, gatherPhase emitted a single
// "Gathering fresh signals" debug log and returned nil with the
// comment "For now, this is a placeholder that would integrate
// with other systems". Every dream session reported phase 2 as
// successful while no signals were ever gathered — downstream
// consolidation operated on stale data and the dreamer pretended
// the cycle was complete. §11.4 PASS-bluff at the dream-phase
// layer. Wire one or more SignalGatherers via
// (*Dreamer).AddGatherer before calling Dream() in production.
var ErrGatherersNotWired = errors.New("dream: no SignalGatherer implementations have been wired into the Dreamer — gatherPhase (phase 2 of the dream cycle) previously emitted a 'Gathering fresh signals' debug log and returned nil with the comment 'For now, this is a placeholder that would integrate with other systems', causing every dream session to report phase 2 as successful while no signals were actually gathered (§11.4 PASS-bluff at the dream-phase layer). Wire one or more SignalGatherers via (*Dreamer).AddGatherer before calling Dream()")

// DreamerConfig configures the Dream system
type DreamerConfig struct {
	Enabled               bool          `json:"enabled"`
	MemoryDir             string        `json:"memory_dir"`
	TimeThreshold         time.Duration `json:"time_threshold"` // 24 hours
	MinSessions           int           `json:"min_sessions"`   // 5 sessions
	ConsolidationInterval time.Duration `json:"consolidation_interval"`
}

// DefaultConfig returns default configuration
func DefaultConfig() DreamerConfig {
	homeDir, _ := os.UserHomeDir()
	return DreamerConfig{
		Enabled:               true,
		MemoryDir:             filepath.Join(homeDir, ".helixagent", "memory"),
		TimeThreshold:         24 * time.Hour,
		MinSessions:           5,
		ConsolidationInterval: 1 * time.Hour,
	}
}

// DreamPhase represents a phase of the dream process
type DreamPhase string

const (
	// PhaseOrientation - Read MEMORY.md, list directory
	PhaseOrientation DreamPhase = "ORIENTATION"
	// PhaseGather - Search for new information
	PhaseGather DreamPhase = "GATHER_SIGNALS"
	// PhaseConsolidate - Write/update memory files
	PhaseConsolidate DreamPhase = "CONSOLIDATION"
	// PhaseCleanup - Maintain MEMORY.md size
	PhaseCleanup DreamPhase = "CLEANUP_INDEXING"
)

// DreamState represents the current state of a dream
type DreamState string

const (
	DreamStatePending   DreamState = "pending"
	DreamStateRunning   DreamState = "running"
	DreamStateCompleted DreamState = "completed"
	DreamStateFailed    DreamState = "failed"
	DreamStateCancelled DreamState = "cancelled"
)

// DreamTrigger represents the three-gate trigger system
type DreamTrigger struct {
	LastDreamTime      time.Time     `json:"last_dream_time"`
	SessionCount       int           `json:"session_count"`
	SessionsSinceDream int           `json:"sessions_since_dream"`
	TimeThreshold      time.Duration `json:"time_threshold"`
	MinSessions        int           `json:"min_sessions"`
	Locked             bool          `json:"locked"`
}

// DreamSession represents a complete dream session
type DreamSession struct {
	ID              string                 `json:"id"`
	StartedAt       time.Time              `json:"started_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	State           DreamState             `json:"state"`
	CurrentPhase    DreamPhase             `json:"current_phase"`
	Phases          []PhaseResult          `json:"phases"`
	NewMemories     []MemoryEntry          `json:"new_memories"`
	UpdatedMemories []MemoryEntry          `json:"updated_memories"`
	RemovedMemories []string               `json:"removed_memories"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// PhaseResult represents the result of a dream phase
type PhaseResult struct {
	Phase     DreamPhase `json:"phase"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Success   bool       `json:"success"`
	Details   string     `json:"details,omitempty"`
}

// MemoryEntry represents a consolidated memory
type MemoryEntry struct {
	ID          string                 `json:"id"`
	Category    string                 `json:"category"` // pattern, fact, preference, project
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Confidence  float64                `json:"confidence"` // 0.0 - 1.0
	Source      string                 `json:"source"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	AccessCount int                    `json:"access_count"`
	LastAccess  time.Time              `json:"last_access"`
	Tags        []string               `json:"tags"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// MEMORY.md structure
type MemoryIndex struct {
	Version     string          `json:"version"`
	LastUpdated time.Time       `json:"last_updated"`
	Categories  []string        `json:"categories"`
	Entries     []MemorySummary `json:"entries"`
}

type MemorySummary struct {
	ID       string   `json:"id"`
	Category string   `json:"category"`
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
}

// SignalGatherer is the wiring contract for phase 2 of the dream
// cycle (gatherPhase). Implementations pull fresh signals from
// upstream subsystems — KAIROS daily logs, drifting memory stores,
// transcript indexes, telemetry buses — and stage them on the
// supplied DreamSession's Metadata so the consolidation phase can
// process them. Production wires real gatherers; unit tests under
// CONST-050(A) MAY supply deterministic stubs.
//
// Round-29 anti-bluff anchor: when no SignalGatherer is registered,
// gatherPhase returns ErrGatherersNotWired instead of the prior
// silent "Gathering fresh signals" debug log + nil return.
type SignalGatherer interface {
	// Gather pulls fresh signals from upstream subsystems and
	// records them on the session's Metadata. MUST return a non-nil
	// error on failure; MUST NOT silently no-op while reporting
	// success.
	Gather(ctx context.Context, session *DreamSession) error
}

// Dreamer is the memory consolidation engine.
//
// Concurrency model (CONST-029):
//   - sessions → *safe.Slice, memories → *safe.Store (lock-free reads).
//   - running → atomic.Bool for Start/Stop idempotency via CAS.
//   - mu (RWMutex) survives Pattern-Zeta: guards the DreamTrigger
//     compound (LastDreamTime/SessionCount/Locked), the *d.current
//     pointer, and per-session Phases appends during executePhase.
//   - memoryMu (Mutex) serializes every write to the on-disk memory
//     store (the per-entry JSON files AND MEMORY.md) so saveMemories()
//     can never be invoked by two goroutines concurrently — closing a
//     data-corruption race in which run()'s select (see run()'s doc
//     comment) could randomly pick a pending tick and launch a full
//     Dream() session — which itself calls saveMemories() on
//     completion — at the same moment an external Stop() call is also
//     directly invoking saveMemories(). See saveMemories()'s doc
//     comment for the exact invariant this delivers and its honestly
//     -scoped limits.
type Dreamer struct {
	config   DreamerConfig
	logger   *logrus.Logger
	trigger  DreamTrigger
	sessions *safe.Slice[DreamSession]
	memories *safe.Store[string, MemoryEntry]
	current  *DreamSession
	mu       sync.RWMutex
	memoryMu sync.Mutex
	running  atomic.Bool
	stopCh   chan struct{}

	// memoryWritersInFlight / memoryWritersPeak are the §11.4.108
	// runtime signature for the Defect 1 (data corruption) invariant:
	// they directly OBSERVE how many goroutines are simultaneously
	// inside saveMemories()'s file-writing critical section, rather
	// than inferring safety from the absence of an observed file tear
	// (which is filesystem- and timing-dependent and unreliable to
	// force on demand). memoryWritersInFlight is incremented on entry
	// to that critical section and decremented on exit; a
	// compare-and-swap loop tracks the highest value it has ever
	// reached in memoryWritersPeak. With memoryMu held around the same
	// section, the peak is provably never > 1 — a direct,
	// deterministic proof of "no concurrent writers, ever" — see
	// TestDreamer_SaveMemories_ConcurrentWriteCorruption.
	memoryWritersInFlight atomic.Int32
	memoryWritersPeak     atomic.Int32

	// gatherers is the round-29 anti-bluff injection point for
	// phase 2. Empty / nil = gatherPhase returns
	// ErrGatherersNotWired. Guarded by mu when mutated alongside
	// dream-cycle state.
	gatherers []SignalGatherer

	// Callbacks
	onPhaseStart    func(DreamPhase)
	onPhaseEnd      func(DreamPhase, bool, string)
	onMemoryAdded   func(MemoryEntry)
	onMemoryUpdated func(MemoryEntry)
}

// AddGatherer registers a SignalGatherer for phase 2 of the dream
// cycle. Round-29 anti-bluff fix: production MUST register at least
// one gatherer before calling Dream(); otherwise gatherPhase
// surfaces ErrGatherersNotWired and the phase is marked
// unsuccessful (instead of the pre-round-29 silent no-op that
// reported phase 2 as successful).
func (d *Dreamer) AddGatherer(g SignalGatherer) {
	if g == nil {
		return
	}
	d.mu.Lock()
	d.gatherers = append(d.gatherers, g)
	d.mu.Unlock()
}

// NewDreamer creates a new Dream system
func NewDreamer(config DreamerConfig, logger *logrus.Logger) *Dreamer {
	return &Dreamer{
		config: config,
		logger: logger,
		trigger: DreamTrigger{
			TimeThreshold: config.TimeThreshold,
			MinSessions:   config.MinSessions,
		},
		sessions:        safe.NewSlice[DreamSession](),
		memories:        safe.NewStore[string, MemoryEntry](),
		stopCh:          make(chan struct{}),
		onPhaseStart:    func(p DreamPhase) {},
		onPhaseEnd:      func(p DreamPhase, success bool, details string) {},
		onMemoryAdded:   func(m MemoryEntry) {},
		onMemoryUpdated: func(m MemoryEntry) {},
	}
}

// SetCallbacks sets the dreamer callbacks
func (d *Dreamer) SetCallbacks(
	onPhaseStart func(DreamPhase),
	onPhaseEnd func(DreamPhase, bool, string),
	onMemoryAdded func(MemoryEntry),
	onMemoryUpdated func(MemoryEntry),
) {
	d.onPhaseStart = onPhaseStart
	d.onPhaseEnd = onPhaseEnd
	d.onMemoryAdded = onMemoryAdded
	d.onMemoryUpdated = onMemoryUpdated
}

// Start starts the Dream system
func (d *Dreamer) Start(ctx context.Context) error {
	if !d.config.Enabled {
		d.logger.Info("Dream system is disabled")
		return nil
	}

	if !d.running.CompareAndSwap(false, true) {
		return fmt.Errorf("Dreamer already running")
	}

	// Ensure memory directory exists
	if err := os.MkdirAll(d.config.MemoryDir, 0750); err != nil {
		d.running.Store(false)
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Load existing memories
	d.loadMemories()

	go d.run(ctx)

	d.logger.Info("Dream system started")
	return nil
}

// Stop stops the Dream system
func (d *Dreamer) Stop() error {
	if !d.running.CompareAndSwap(true, false) {
		return nil
	}

	close(d.stopCh)

	// Save memories
	d.saveMemories()

	d.logger.Info("Dream system stopped")
	return nil
}

// IsRunning returns true if the dreamer is running
func (d *Dreamer) IsRunning() bool {
	return d.running.Load()
}

// checkStop performs a non-blocking check for a requested shutdown. It
// is called both at the top of run()'s loop (priority pre-check) and
// immediately before launching a Dream() session (re-check) so that a
// pending stop always wins over starting a new dream cycle — see
// run()'s doc comment for why this narrows, but per Go's async
// goroutine preemption (>=1.14) cannot provably close to zero-width,
// the window in which Dream() is launched after shutdown was
// requested. Reports whether run() should return immediately.
// Cancellation via ctx (as opposed to an explicit Stop() call, which
// already performs its own shutdown+save) additionally invokes
// d.Stop() synchronously — safe here because Dreamer.Stop() does not
// join against run()'s own goroutine (see memoryMu's doc comment for
// why a mutex, not a join, was chosen to close Defect 1).
func (d *Dreamer) checkStop(ctx context.Context) bool {
	select {
	case <-d.stopCh:
		return true
	case <-ctx.Done():
		d.Stop()
		return true
	default:
		return false
	}
}

// runOnce executes a single iteration of the dream loop's select
// statement. It is extracted from run() and parameterized by the tick
// channel — rather than reading a *time.Ticker directly — specifically
// so tests can force the exact race this fix addresses
// deterministically: a freshly-created time.Ticker's channel is
// guaranteed EMPTY until its interval has genuinely elapsed, so there
// is no way to make a real ticker.C provably ready at the SAME instant
// d.stopCh closes without depending on real wall-clock scheduling and
// accepting non-determinism. Passing a synthetic, pre-populated `tick`
// channel (a buffered chan time.Time with one value already sent)
// alongside an already-closed d.stopCh lets a test observe Go's real
// select semantics acting on this REAL production select statement —
// not a replica of it — with both cases provably ready before the
// select is evaluated. Reports whether run() should return.
//
// Forensic anchor (Defect 1 — data corruption): Go's select chooses
// UNIFORMLY AT RANDOM among all ready cases. If a tick is ALSO pending
// at the instant Stop() closes d.stopCh (or the caller cancels ctx),
// the naked select below could pick the tick branch and launch a full
// Dream() session — which itself calls saveMemories() on completion —
// even though shutdown has already been requested, racing Stop()'s own
// direct saveMemories() call. The priority pre-check + re-check below
// (both via checkStop) narrow that window so a pending stop always
// wins wherever it is observed; they do NOT by themselves prevent the
// corruption, because a stop landing WHILE either select is parked can
// still let the random pick land on the tick case. What actually
// closes the corruption is memoryMu inside saveMemories() — see its
// doc comment for the exact, honestly-scoped guarantee.
func (d *Dreamer) runOnce(ctx context.Context, tick <-chan time.Time) bool {
	// Priority pre-check: ensures a pending stop always wins once it
	// has fired, so Dream() is never launched after shutdown was
	// observed here.
	if d.checkStop(ctx) {
		return true
	}

	select {
	case <-d.stopCh:
		return true
	case <-ctx.Done():
		d.Stop()
		return true
	case <-tick:
		// Second, immediate re-check: the blocking select above is
		// itself a scheduling/yield point (it can park waiting on any
		// of the three channels), so a stop can be requested WHILE it
		// is parked and a tick can arrive at the same instant, letting
		// the random pick land on the tick case anyway. Re-checking
		// here narrows, but cannot provably close to zero-width, the
		// window in which a Dream() session is launched after
		// shutdown was requested. What this re-check DOES deliver: a
		// stop observed AT THIS CHECK always wins (Dream() is never
		// called).
		if d.checkStop(ctx) {
			return true
		}

		if d.ShouldDream() {
			d.logger.Info("Triggering dream session")
			if _, err := d.Dream(ctx); err != nil {
				d.logger.WithError(err).Error("Dream session failed")
			}
		}
	}
	return false
}

// run is the main dream loop. See runOnce for the per-iteration logic
// and the Defect 1 forensic anchor.
func (d *Dreamer) run(ctx context.Context) {
	ticker := time.NewTicker(d.config.ConsolidationInterval)
	defer ticker.Stop()

	for {
		if d.runOnce(ctx, ticker.C) {
			return
		}
	}
}

// ShouldDream checks if all three gates are open
func (d *Dreamer) ShouldDream() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Time Gate: 24 hours since last dream
	if time.Since(d.trigger.LastDreamTime) < d.trigger.TimeThreshold {
		return false
	}

	// Session Gate: Minimum 5 sessions since last dream
	if d.trigger.SessionsSinceDream < d.trigger.MinSessions {
		return false
	}

	// Lock Gate: Not already dreaming
	if d.trigger.Locked {
		return false
	}

	return true
}

// RecordSession records that a session occurred
func (d *Dreamer) RecordSession() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.trigger.SessionCount++
	d.trigger.SessionsSinceDream++
}

// Dream initiates a dream session
func (d *Dreamer) Dream(ctx context.Context) (*DreamSession, error) {
	d.mu.Lock()

	// Acquire lock
	if d.trigger.Locked {
		d.mu.Unlock()
		return nil, fmt.Errorf("dream already in progress")
	}
	d.trigger.Locked = true

	// Create session
	session := &DreamSession{
		ID:              generateDreamID(),
		StartedAt:       time.Now(),
		State:           DreamStateRunning,
		CurrentPhase:    PhaseOrientation,
		Phases:          make([]PhaseResult, 0),
		NewMemories:     make([]MemoryEntry, 0),
		UpdatedMemories: make([]MemoryEntry, 0),
		RemovedMemories: make([]string, 0),
		Metadata:        make(map[string]interface{}),
	}
	d.current = session
	d.mu.Unlock()

	d.logger.Infof("Starting dream session %s", session.ID)

	// Execute phases
	d.executePhase(ctx, session, PhaseOrientation, d.orientationPhase)
	d.executePhase(ctx, session, PhaseGather, d.gatherPhase)
	d.executePhase(ctx, session, PhaseConsolidate, d.consolidationPhase)
	d.executePhase(ctx, session, PhaseCleanup, d.cleanupPhase)

	// Complete session
	d.mu.Lock()
	session.State = DreamStateCompleted
	now := time.Now()
	session.CompletedAt = &now

	// Update trigger
	d.trigger.LastDreamTime = now
	d.trigger.SessionsSinceDream = 0
	d.trigger.Locked = false

	d.current = nil
	completed := *session
	d.mu.Unlock()

	d.sessions.Append(completed)

	d.logger.Infof("Dream session %s completed", session.ID)

	// Save memories
	d.saveMemories()

	return session, nil
}

// executePhase executes a single dream phase
func (d *Dreamer) executePhase(ctx context.Context, session *DreamSession, phase DreamPhase, fn func(context.Context, *DreamSession) error) {
	d.mu.Lock()
	session.CurrentPhase = phase
	phaseResult := PhaseResult{
		Phase:     phase,
		StartedAt: time.Now(),
	}
	d.mu.Unlock()

	d.onPhaseStart(phase)
	d.logger.Debugf("Dream phase: %s", phase)

	err := fn(ctx, session)

	now := time.Now()
	phaseResult.EndedAt = &now
	phaseResult.Success = err == nil
	if err != nil {
		phaseResult.Details = err.Error()
	}

	d.mu.Lock()
	session.Phases = append(session.Phases, phaseResult)
	d.mu.Unlock()

	d.onPhaseEnd(phase, phaseResult.Success, phaseResult.Details)
}

// orientationPhase: Phase 1 - Read MEMORY.md, list directory
func (d *Dreamer) orientationPhase(ctx context.Context, session *DreamSession) error {
	// Read MEMORY.md
	memoryMdPath := filepath.Join(d.config.MemoryDir, "MEMORY.md")
	content, err := os.ReadFile(memoryMdPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	session.Metadata["memory_md_size"] = len(content)

	// List memory directory
	entries, err := os.ReadDir(d.config.MemoryDir)
	if err != nil {
		return err
	}

	var memoryFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			memoryFiles = append(memoryFiles, entry.Name())
		}
	}

	session.Metadata["memory_file_count"] = len(memoryFiles)
	d.logger.Debugf("Found %d memory files", len(memoryFiles))

	return nil
}

// gatherPhase: Phase 2 - Search for new information.
//
// Round-29 anti-bluff fix: previously emitted a single "Gathering
// fresh signals" debug log and returned nil regardless of state —
// every dream session reported phase 2 as successful while no
// signals were ever gathered. Now: if no SignalGatherer has been
// registered (via AddGatherer), the phase surfaces
// ErrGatherersNotWired and is marked unsuccessful in the
// PhaseResult; if gatherers ARE registered, each is invoked in
// turn and the first error short-circuits the phase. Wire one or
// more gatherers before calling Dream() in production.
func (d *Dreamer) gatherPhase(ctx context.Context, session *DreamSession) error {
	d.mu.RLock()
	gatherers := append([]SignalGatherer(nil), d.gatherers...)
	d.mu.RUnlock()

	if len(gatherers) == 0 {
		return fmt.Errorf("gatherPhase: %w", ErrGatherersNotWired)
	}

	d.logger.Debugf("Gathering fresh signals from %d gatherers", len(gatherers))
	for i, g := range gatherers {
		if err := g.Gather(ctx, session); err != nil {
			return fmt.Errorf("gatherPhase: gatherer #%d (%T) failed: %w", i, g, err)
		}
	}
	return nil
}

// consolidationPhase: Phase 3 - Write/update memory files
func (d *Dreamer) consolidationPhase(ctx context.Context, session *DreamSession) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Process gathered information and create/update memories
	// This would:
	// - Extract patterns from sessions
	// - Identify facts worth preserving
	// - Update existing memories with new information
	// - Translate relative dates to absolute
	// - Remove disproven facts

	d.logger.Debug("Consolidating memories")

	return nil
}

// cleanupPhase: Phase 4 - Maintain MEMORY.md size
func (d *Dreamer) cleanupPhase(ctx context.Context, session *DreamSession) error {
	// Keep MEMORY.md within 200 lines (~25KB)
	memoryMdPath := filepath.Join(d.config.MemoryDir, "MEMORY.md")

	content, err := os.ReadFile(memoryMdPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) > 200 {
		// Remove stale pointers and resolve contradictions
		// For now, just trim to last 200 lines
		lines = lines[len(lines)-200:]
		newContent := strings.Join(lines, "\n")

		// #nosec G703 -- memoryMdPath is the dreamer's own internal state
		// file, derived from a dreamer-controlled directory, never from
		// user or LLM input.
		if err := os.WriteFile(memoryMdPath, []byte(newContent), 0644); err != nil {
			return err
		}

		d.logger.Debug("Trimmed MEMORY.md to 200 lines")
	}

	return nil
}

// AddMemory adds a new memory
func (d *Dreamer) AddMemory(entry MemoryEntry) error {
	if entry.ID == "" {
		entry.ID = generateMemoryID()
	}

	entry.CreatedAt = time.Now()
	entry.UpdatedAt = entry.CreatedAt

	d.memories.Put(entry.ID, entry)
	d.onMemoryAdded(entry)

	return nil
}

// UpdateMemory updates an existing memory
func (d *Dreamer) UpdateMemory(id string, updates map[string]interface{}) error {
	var (
		notFound bool
		updated  MemoryEntry
	)
	d.memories.Update(id, func(cur MemoryEntry, present bool) (MemoryEntry, bool) {
		if !present {
			notFound = true
			return cur, false
		}
		if content, ok := updates["content"].(string); ok {
			cur.Content = content
		}
		if confidence, ok := updates["confidence"].(float64); ok {
			cur.Confidence = confidence
		}
		if tags, ok := updates["tags"].([]string); ok {
			cur.Tags = tags
		}
		cur.UpdatedAt = time.Now()
		updated = cur
		return cur, true
	})
	if notFound {
		return fmt.Errorf("memory not found: %s", id)
	}

	d.onMemoryUpdated(updated)
	return nil
}

// GetMemory retrieves a memory by ID
func (d *Dreamer) GetMemory(id string) (MemoryEntry, bool) {
	return d.memories.Get(id)
}

// GetMemoriesByCategory returns memories in a category
func (d *Dreamer) GetMemoriesByCategory(category string) []MemoryEntry {
	var result []MemoryEntry
	d.memories.Range(func(_ string, memory MemoryEntry) bool {
		if memory.Category == category {
			result = append(result, memory)
		}
		return true
	})
	return result
}

// GetAllMemories returns all memories
func (d *Dreamer) GetAllMemories() []MemoryEntry {
	snap := d.memories.Snapshot()
	result := make([]MemoryEntry, 0, len(snap))
	for _, memory := range snap {
		result = append(result, memory)
	}
	return result
}

// GetCurrentSession returns the current dream session
func (d *Dreamer) GetCurrentSession() *DreamSession {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.current == nil {
		return nil
	}

	session := *d.current
	return &session
}

// GetSessions returns all dream sessions
func (d *Dreamer) GetSessions() []DreamSession {
	return d.sessions.Snapshot()
}

// loadMemories loads memories from disk
func (d *Dreamer) loadMemories() {
	entries, err := os.ReadDir(d.config.MemoryDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(d.config.MemoryDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var memory MemoryEntry
		if err := json.Unmarshal(data, &memory); err != nil {
			continue
		}

		d.memories.Put(memory.ID, memory)
	}

	d.logger.Infof("Loaded %d memories", d.memories.Len())
}

// saveMemories saves memories to disk.
//
// Defect 1 (data corruption) fix: memoryMu serializes every write this
// function performs (the per-entry JSON files below AND, transitively,
// MEMORY.md via updateMemoryIndex()) so that a Dream() session's own
// completion save (see Dream()) and Stop()'s directly-invoked save can
// never write those files concurrently. The mutex is the chosen
// mechanism — deliberately, not a join on run()'s goroutine — because
// Dream() is a publicly exported method any caller may invoke
// directly, independent of run()'s background loop; a
// sync.WaitGroup/done-channel join scoped only to run()'s goroutine
// lifetime would synchronize Stop() against dream sessions the
// background loop itself launched, but would do nothing to prevent
// Stop()'s saveMemories() from racing a Dream() call made by some
// OTHER caller — a gap that would leave two goroutines writing
// MEMORY.md concurrently. A mutex around the actual write path closes
// that unconditionally, regardless of who called Dream().
//
// Honest scope of the guarantee actually delivered: memoryMu
// guarantees the files are NEVER written by two goroutines at once
// (no torn/interleaved write is possible) — it does NOT guarantee that
// no write happens strictly AFTER Stop() has already returned to its
// caller. If a Dream() session was already racing in via run()'s
// ticker branch when Stop() was called, Stop() may win memoryMu first
// and return while that Dream() session is still mid-flight; it will
// subsequently acquire memoryMu and write safely (serialized, never
// interleaved with Stop()'s write) — just after Stop() already
// returned. Closing that residual fully would require Stop() to block
// (via a join) until run()'s goroutine has provably exited, which
// risks turning Stop() into a call whose latency is bounded only by
// however long an entire in-flight Dream() cycle (all four phases,
// including consolidation) takes. That cost was judged disproportionate
// to the corruption defect this fix actually closes: corruption
// (concurrent/torn writes) is what was reported and is what memoryMu
// eliminates; "a write can still land a moment after Stop() returns"
// is a lesser, non-corrupting, and explicitly accepted trade-off.
func (d *Dreamer) saveMemories() {
	d.memoryMu.Lock()
	defer d.memoryMu.Unlock()

	// §11.4.108 runtime signature: see memoryWritersInFlight's doc
	// comment on the Dreamer struct. Bracketing this INSIDE the
	// memoryMu-protected section (rather than at function entry, which
	// would race the counter itself before either caller acquires the
	// lock) is deliberate: it observes exactly the invariant under
	// test — concurrency inside the file-writing critical section
	// itself — not mere concurrent entry into saveMemories().
	inFlight := d.memoryWritersInFlight.Add(1)
	defer d.memoryWritersInFlight.Add(-1)
	for {
		peak := d.memoryWritersPeak.Load()
		if inFlight <= peak || d.memoryWritersPeak.CompareAndSwap(peak, inFlight) {
			break
		}
	}

	for id, memory := range d.memories.Snapshot() {
		path := filepath.Join(d.config.MemoryDir, fmt.Sprintf("%s.json", id))

		data, err := json.MarshalIndent(memory, "", "  ")
		if err != nil {
			continue
		}

		os.WriteFile(path, data, 0644)
	}

	// Update MEMORY.md index
	d.updateMemoryIndex()
}

// updateMemoryIndex updates the MEMORY.md file
func (d *Dreamer) updateMemoryIndex() {
	snap := d.memories.Snapshot()
	index := MemoryIndex{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Categories:  []string{"pattern", "fact", "preference", "project"},
		Entries:     make([]MemorySummary, 0, len(snap)),
	}

	for _, memory := range snap {
		index.Entries = append(index.Entries, MemorySummary{
			ID:       memory.ID,
			Category: memory.Category,
			Title:    memory.Title,
			Tags:     memory.Tags,
		})
	}

	// Generate markdown content
	var content strings.Builder
	content.WriteString("# HelixAgent Memory\n\n")
	content.WriteString(fmt.Sprintf("Last Updated: %s\n\n", index.LastUpdated.Format(time.RFC3339)))

	for _, category := range index.Categories {
		content.WriteString(fmt.Sprintf("## %s\n\n", strings.Title(category)))

		for _, entry := range index.Entries {
			if entry.Category == category {
				content.WriteString(fmt.Sprintf("- **%s** (%s)\n", entry.Title, strings.Join(entry.Tags, ", ")))
			}
		}

		content.WriteString("\n")
	}

	memoryMdPath := filepath.Join(d.config.MemoryDir, "MEMORY.md")
	os.WriteFile(memoryMdPath, []byte(content.String()), 0644)
}

// dreamIDCounter and memoryIDCounter guarantee ID uniqueness even when the
// generators are called multiple times within the same nanosecond tick
// (UnixNano resolution is not fine enough on fast hosts to distinguish
// back-to-back calls, which would otherwise overwrite map entries and silently
// lose dreams/memories).
var (
	dreamIDCounter  uint64
	memoryIDCounter uint64
)

// generateDreamID generates a unique dream ID
func generateDreamID() string {
	return fmt.Sprintf("dream_%d_%d", time.Now().UnixNano(), atomic.AddUint64(&dreamIDCounter, 1))
}

// generateMemoryID generates a unique memory ID
func generateMemoryID() string {
	return fmt.Sprintf("mem_%d_%d", time.Now().UnixNano(), atomic.AddUint64(&memoryIDCounter, 1))
}
