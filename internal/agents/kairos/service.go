// Package kairos provides the always-on AI assistant service
// Inspired by Claude Code's KAIROS internal feature
package kairos

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// ServiceConfig configures the KAIROS service
type ServiceConfig struct {
	Enabled             bool          `json:"enabled"`
	TickInterval        time.Duration `json:"tick_interval"`
	BlockingBudget      time.Duration `json:"blocking_budget"`
	LogRetentionDays    int           `json:"log_retention_days"`
	WorkspacePath       string        `json:"workspace_path"`
	LogPath             string        `json:"log_path"`
	EnableNotifications bool          `json:"enable_notifications"`
}

// DefaultConfig returns default configuration
func DefaultConfig() ServiceConfig {
	homeDir, _ := os.UserHomeDir()
	return ServiceConfig{
		Enabled:             true,
		TickInterval:        5 * time.Minute,
		BlockingBudget:      15 * time.Second,
		LogRetentionDays:    30,
		WorkspacePath:       filepath.Join(homeDir, ".helixagent", "kairos"),
		LogPath:             filepath.Join(homeDir, ".helixagent", "kairos", "logs"),
		EnableNotifications: true,
	}
}

// Observation represents something KAIROS observed
type Observation struct {
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`
	Content     string                 `json:"content"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Processed   bool                   `json:"processed"`
	ActionTaken string                 `json:"action_taken,omitempty"`
}

// Action represents a proactive action taken by KAIROS
type Action struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"`
	Trigger     string                 `json:"trigger"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"` // pending, executing, completed, failed, deferred
	Result      string                 `json:"result,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TickPrompt represents a tick event for decision making
type TickPrompt struct {
	Type       string          `json:"type"`
	Timestamp  time.Time       `json:"timestamp"`
	Context    ObservedContext `json:"context"`
	RecentLogs []Observation   `json:"recent_logs"`
}

// ObservedContext represents the current context
type ObservedContext struct {
	WorkingDirectory string                 `json:"working_directory"`
	GitBranch        string                 `json:"git_branch,omitempty"`
	ModifiedFiles    []string               `json:"modified_files,omitempty"`
	RecentCommands   []string               `json:"recent_commands,omitempty"`
	SystemState      map[string]interface{} `json:"system_state,omitempty"`
}

// Service is the KAIROS always-on assistant.
//
// Concurrent-safe by construction (CONST-029):
//   - observations: single-key Store holding the full slice for atomic
//     append+bounded-trim (Pattern Epsilon).
//   - actions: safe.Slice with UpdateAt for find-and-mutate (Pattern Beta).
//   - currentContext: atomic.Pointer[ObservedContext].
//   - running: atomic.Bool.
//   - startMu: sync.Mutex retained for Start/Stop validate-then-commit
//     (no collection adjacency — audit-clean).
//   - runWg: sync.WaitGroup joined by Stop() (Defect 2 fix — see run()'s
//     and Stop()'s doc comments) so Stop() does not return to its
//     caller until the background run() goroutine, and any tick() it
//     had already started, has fully exited.
type Service struct {
	config         ServiceConfig
	logger         *logrus.Logger
	observations   *safe.Store[string, []Observation]
	actions        *safe.Slice[Action]
	currentContext atomic.Pointer[ObservedContext]
	ticker         *time.Ticker
	stopCh         chan struct{}
	startMu        sync.Mutex // serialises Start/Stop only
	runWg          sync.WaitGroup
	running        atomic.Bool

	// Callbacks
	onObservation func(Observation)
	onAction      func(Action)
	onDecision    func(TickPrompt) (Action, error)
}

const kairosObsKey = "_"

// NewService creates a new KAIROS service
func NewService(config ServiceConfig, logger *logrus.Logger) *Service {
	obsStore := safe.NewStore[string, []Observation]()
	obsStore.Put(kairosObsKey, make([]Observation, 0))
	s := &Service{
		config:        config,
		logger:        logger,
		observations:  obsStore,
		actions:       safe.NewSlice[Action](),
		stopCh:        make(chan struct{}),
		onObservation: func(o Observation) {},
		onAction:      func(a Action) {},
		onDecision:    func(t TickPrompt) (Action, error) { return Action{}, nil },
	}
	s.currentContext.Store(&ObservedContext{})
	return s
}

// SetCallbacks sets the service callbacks
func (s *Service) SetCallbacks(
	onObservation func(Observation),
	onAction func(Action),
	onDecision func(TickPrompt) (Action, error),
) {
	s.onObservation = onObservation
	s.onAction = onAction
	s.onDecision = onDecision
}

// Start starts the KAIROS service
func (s *Service) Start(ctx context.Context) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()

	if s.running.Load() {
		return fmt.Errorf("KAIROS service already running")
	}

	if !s.config.Enabled {
		s.logger.Info("KAIROS service is disabled")
		return nil
	}

	if err := os.MkdirAll(s.config.LogPath, 0750); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	s.loadObservations()

	s.ticker = time.NewTicker(s.config.TickInterval)
	s.running.Store(true)

	s.runWg.Add(1)
	go s.run(ctx)

	s.logger.Info("KAIROS service started")
	return nil
}

// Stop stops the KAIROS service.
//
// Defect 2 (action executed after Stop() returned) fix: after closing
// s.stopCh, Stop() joins on s.runWg — waiting for the background
// run() goroutine, and any tick() it had already started before
// observing the stop, to fully exit — BEFORE Stop() itself returns to
// its caller. Without this join, run()'s select could have randomly
// picked s.ticker.C over s.stopCh (Go's select chooses uniformly at
// random among ready cases) and launched a real tick() — which can
// fire onAction and execute a genuine, potentially long-running
// action — moments before Stop() was called; Stop() would then have
// returned to its caller while that action was still executing,
// exactly the defect reported. The wait is bounded: tick() itself
// never blocks longer than s.config.BlockingBudget (it races its own
// onAction goroutine against an actionCtx timeout of that duration),
// so s.runWg.Wait() below is bounded by, at most, one BlockingBudget
// plus whatever remains of the current dispatch loop iteration.
//
// Honest scope: this closes the gap for the common case — an action
// that completes (or times out) within tick()'s own actionCtx budget.
// It does NOT reach the pre-existing, independent limitation that
// tick()'s onAction runs in a detached goroutine with no context
// propagated into the callback itself (see tick()'s doc comment): if
// onAction hangs past BlockingBudget, tick() returns via its timeout
// branch and this join is satisfied, while that detached goroutine
// keeps running in the background after Stop() has returned — the
// same "cannot forcibly kill a goroutine in Go" limitation documented
// elsewhere in this codebase (e.g. llm.LazyProvider.
// createProviderWithContext). Fixing that would require changing
// onAction's callback contract to accept and honour a context, which
// is out of scope here.
func (s *Service) Stop() error {
	s.startMu.Lock()
	defer s.startMu.Unlock()

	if !s.running.Load() {
		return nil
	}

	s.ticker.Stop()
	close(s.stopCh)
	s.running.Store(false)

	s.runWg.Wait()

	s.saveObservations()

	s.logger.Info("KAIROS service stopped")
	return nil
}

// IsRunning returns true if the service is running
func (s *Service) IsRunning() bool {
	return s.running.Load()
}

// checkStop performs a non-blocking check for a requested shutdown. It
// is called both at the top of run()'s loop (priority pre-check) and
// immediately before invoking tick() (re-check) so that a pending stop
// always wins over launching a new tick() — see run()'s doc comment
// for why this narrows, but per Go's async goroutine preemption
// (>=1.14) cannot provably close to zero-width, the window in which a
// tick() is launched after shutdown was requested. Reports whether
// run() should return immediately.
//
// Cancellation via ctx (as opposed to an explicit Stop() call, which
// already performs its own shutdown) additionally dispatches Stop()
// in its OWN goroutine — never synchronously. A synchronous s.Stop()
// call from within run() would deadlock: Stop() (see its doc comment)
// now joins on s.runWg, which only reaches zero once THIS goroutine
// returns; calling Stop() synchronously from here would have run()
// blocked inside Stop() waiting for run() itself to return, which it
// never can while blocked inside Stop(). Dispatching via `go s.Stop()`
// decouples the two goroutines so the join can actually complete once
// this goroutine's deferred s.runWg.Done() fires.
func (s *Service) checkStop(ctx context.Context) bool {
	select {
	case <-s.stopCh:
		return true
	case <-ctx.Done():
		go s.Stop()
		return true
	default:
		return false
	}
}

// runOnce executes a single iteration of the service loop's select
// statement. It is extracted from run() and parameterized by the tick
// channel — rather than reading s.ticker directly — specifically so
// tests can force the exact race this fix addresses deterministically:
// a freshly-created time.Ticker's channel is guaranteed EMPTY until
// its interval has genuinely elapsed, so there is no way to make a
// real s.ticker.C provably ready at the SAME instant s.stopCh closes
// without depending on real wall-clock scheduling and accepting
// non-determinism. Passing a synthetic, pre-populated `tick` channel (a
// buffered chan time.Time with one value already sent) alongside an
// already-closed s.stopCh lets a test observe Go's real select
// semantics acting on this REAL production select statement — not a
// replica of it — with both cases provably ready before the select is
// evaluated. Reports whether run() should return.
//
// Forensic anchor (Defect 2 — action executed after Stop() returned):
// Go's select chooses UNIFORMLY AT RANDOM among all ready cases. If a
// tick is ALSO pending at the instant Stop() closes s.stopCh (or the
// caller cancels ctx), the naked select below could pick the tick case
// and run a real tick() — which can fire onAction and execute a
// genuine, potentially long-running action — even though shutdown has
// already been requested. The priority pre-check + re-check below
// (both via checkStop) narrow that window so a pending stop always
// wins wherever it is observed; they do NOT by themselves guarantee no
// action starts after a stop was requested, because a stop landing
// WHILE either select is parked can still let the random pick land on
// the tick case. What actually closes the "returned-before-action
// -finished" gap is Stop()'s s.runWg.Wait() join — see Stop()'s doc
// comment for the exact, honestly-scoped guarantee it delivers.
func (s *Service) runOnce(ctx context.Context, tick <-chan time.Time) bool {
	// Priority pre-check: ensures a pending stop always wins once it
	// has fired, so a NEW tick() is never launched after shutdown was
	// observed here.
	if s.checkStop(ctx) {
		return true
	}

	select {
	case <-s.stopCh:
		return true
	case <-ctx.Done():
		go s.Stop()
		return true
	case <-tick:
		// Second, immediate re-check: the blocking select above is
		// itself a scheduling/yield point (it can park waiting on any
		// of the three channels), so a stop can be requested WHILE it
		// is parked and a tick can arrive at the same instant, letting
		// the random pick land on the tick case anyway. Re-checking
		// here narrows, but cannot provably close to zero-width, the
		// window in which tick() is launched after shutdown was
		// requested. What this re-check DOES deliver: a stop observed
		// AT THIS CHECK always wins (tick() is never called).
		if s.checkStop(ctx) {
			return true
		}

		s.tick(ctx)
	}
	return false
}

// run is the main service loop. See runOnce for the per-iteration
// logic and the Defect 2 forensic anchor.
func (s *Service) run(ctx context.Context) {
	defer s.runWg.Done()

	for {
		if s.runOnce(ctx, s.ticker.C) {
			return
		}
	}
}

// tick processes a tick event
func (s *Service) tick(ctx context.Context) {
	// Update context
	s.updateContext()

	// Create tick prompt
	currentCtx := s.currentContext.Load()
	if currentCtx == nil {
		currentCtx = &ObservedContext{}
	}
	prompt := TickPrompt{
		Type:       "tick",
		Timestamp:  time.Now(),
		Context:    *currentCtx,
		RecentLogs: s.getRecentObservations(10),
	}

	s.logger.Debug("KAIROS tick: processing context")

	// Make decision
	action, err := s.onDecision(prompt)
	if err != nil {
		s.logger.WithError(err).Error("KAIROS decision error")
		return
	}

	// If no action needed, return
	if action.Type == "" {
		return
	}

	// Check blocking budget
	if action.Duration > s.config.BlockingBudget {
		s.logger.Debugf("Action %s exceeds blocking budget, deferring", action.Type)
		action.Status = "deferred"
		s.recordAction(action)
		return
	}

	// Execute action
	action.ID = generateActionID()
	action.Timestamp = time.Now()
	action.Status = "executing"
	s.recordAction(action)

	s.logger.Infof("KAIROS executing action: %s", action.Description)

	// Execute with timeout. The onAction callback is a caller-supplied
	// function value and may read/mutate its argument arbitrarily, so
	// we pass it a defensive COPY of the Action. The outer function
	// then mutates only the original (status, result) and the callback
	// runs in its own isolated goroutine against its own copy. On
	// completion we merge any metadata the callback might have set
	// back (shallow merge — Metadata map). Race-debt BUGFIX #30.
	actionCtx, cancel := context.WithTimeout(ctx, s.config.BlockingBudget)
	defer cancel()

	done := make(chan struct{})
	// `action` is Action (value) here. We pass a COPY to the callback
	// so the callback cannot race with our post-select mutations of
	// the original. Metadata map is still shared — callers should
	// treat Metadata as append-only or synchronise externally.
	actionForCallback := action
	go func() {
		s.onAction(actionForCallback)
		close(done)
	}()

	select {
	case <-done:
		action.Status = "completed"
	case <-actionCtx.Done():
		action.Status = "failed"
		action.Result = "timeout"
	}

	s.updateAction(action)
}

// updateContext updates the current observed context
func (s *Service) updateContext() {
	wd, _ := os.Getwd()
	branch := s.getGitBranch(wd)
	modifiedFiles := s.getModifiedFiles(wd)

	prev := s.currentContext.Load()
	var recentCmds []string
	if prev != nil {
		recentCmds = prev.RecentCommands
	}
	next := &ObservedContext{
		WorkingDirectory: wd,
		GitBranch:        branch,
		ModifiedFiles:    modifiedFiles,
		RecentCommands:   recentCmds,
	}
	s.currentContext.Store(next)
}

// getGitBranch gets the current git branch
func (s *Service) getGitBranch(workingDir string) string {
	// This would use git commands or the git tool
	// Placeholder implementation
	return ""
}

// getModifiedFiles gets modified files in git
func (s *Service) getModifiedFiles(workingDir string) []string {
	// This would use git commands
	// Placeholder implementation
	return nil
}

// Observe records an observation
func (s *Service) Observe(obsType, source, content string, metadata map[string]interface{}) {
	obs := Observation{
		Timestamp: time.Now(),
		Type:      obsType,
		Source:    source,
		Content:   content,
		Metadata:  metadata,
		Processed: false,
	}

	s.observations.Update(kairosObsKey, func(events []Observation, _ bool) ([]Observation, bool) {
		events = append(events, obs)
		if len(events) > 1000 {
			events = events[len(events)-1000:]
		}
		return events, true
	})

	s.onObservation(obs)

	// Write to daily log
	s.writeToDailyLog(obs)
}

// recordAction records an action
func (s *Service) recordAction(action Action) {
	s.actions.Append(action)
}

// updateAction updates an existing action
func (s *Service) updateAction(action Action) {
	s.actions.UpdateAt(
		func(a Action) bool { return a.ID == action.ID },
		func(_ Action) Action { return action },
	)
}

// GetObservations returns all observations
func (s *Service) GetObservations() []Observation {
	obs, _ := s.observations.Get(kairosObsKey)
	result := make([]Observation, len(obs))
	copy(result, obs)
	return result
}

// GetActions returns all actions
func (s *Service) GetActions() []Action {
	return s.actions.Snapshot()
}

// getRecentObservations returns recent observations
func (s *Service) getRecentObservations(count int) []Observation {
	obs, _ := s.observations.Get(kairosObsKey)
	if len(obs) <= count {
		result := make([]Observation, len(obs))
		copy(result, obs)
		return result
	}
	result := make([]Observation, count)
	copy(result, obs[len(obs)-count:])
	return result
}

// writeToDailyLog writes observation to daily log file
func (s *Service) writeToDailyLog(obs Observation) {
	date := obs.Timestamp.Format("2006-01-02")
	logFile := filepath.Join(s.config.LogPath, fmt.Sprintf("kairos-%s.log", date))

	data, _ := json.Marshal(obs)
	data = append(data, '\n')

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(data)
}

// loadObservations loads observations from log files
func (s *Service) loadObservations() {
	// This would load from the last few log files
	// Placeholder implementation
}

// saveObservations saves observations to disk
func (s *Service) saveObservations() {
	// Observations are already written to daily logs
	// This could archive old logs
}

// generateActionID generates a unique action ID
func generateActionID() string {
	return fmt.Sprintf("action_%d", time.Now().UnixNano())
}

// CleanupOldLogs removes log files older than retention period
func (s *Service) CleanupOldLogs() error {
	cutoff := time.Now().AddDate(0, 0, -s.config.LogRetentionDays)

	entries, err := os.ReadDir(s.config.LogPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(s.config.LogPath, entry.Name()))
		}
	}

	return nil
}

// GetDailySummary returns a summary of the day's activities
func (s *Service) GetDailySummary(date time.Time) map[string]interface{} {
	var dayObservations []Observation
	var dayActions []Action

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	allObs, _ := s.observations.Get(kairosObsKey)
	for _, obs := range allObs {
		if obs.Timestamp.After(startOfDay) && obs.Timestamp.Before(endOfDay) {
			dayObservations = append(dayObservations, obs)
		}
	}

	s.actions.Range(func(_ int, action Action) bool {
		if action.Timestamp.After(startOfDay) && action.Timestamp.Before(endOfDay) {
			dayActions = append(dayActions, action)
		}
		return true
	})

	return map[string]interface{}{
		"date":              date.Format("2006-01-02"),
		"observations":      len(dayObservations),
		"actions":           len(dayActions),
		"observation_types": countByType(dayObservations),
		"action_statuses":   countActionStatuses(dayActions),
	}
}

// countByType counts observations by type
func countByType(observations []Observation) map[string]int {
	counts := make(map[string]int)
	for _, obs := range observations {
		counts[obs.Type]++
	}
	return counts
}

// countActionStatuses counts actions by status
func countActionStatuses(actions []Action) map[string]int {
	counts := make(map[string]int)
	for _, action := range actions {
		counts[action.Status]++
	}
	return counts
}
