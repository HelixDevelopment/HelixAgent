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
	"time"

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

// Service is the KAIROS always-on assistant
type Service struct {
	config         ServiceConfig
	logger         *logrus.Logger
	observations   []Observation
	actions        []Action
	currentContext ObservedContext
	ticker         *time.Ticker
	stopCh         chan struct{}
	mu             sync.RWMutex
	running        bool

	// Callbacks
	onObservation func(Observation)
	onAction      func(Action)
	onDecision    func(TickPrompt) (Action, error)
}

// NewService creates a new KAIROS service
func NewService(config ServiceConfig, logger *logrus.Logger) *Service {
	return &Service{
		config:        config,
		logger:        logger,
		observations:  make([]Observation, 0),
		actions:       make([]Action, 0),
		stopCh:        make(chan struct{}),
		onObservation: func(o Observation) {},
		onAction:      func(a Action) {},
		onDecision:    func(t TickPrompt) (Action, error) { return Action{}, nil },
	}
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("KAIROS service already running")
	}

	if !s.config.Enabled {
		s.logger.Info("KAIROS service is disabled")
		return nil
	}

	// Ensure directories exist
	if err := os.MkdirAll(s.config.LogPath, 0750); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Load previous observations
	s.loadObservations()

	// Start ticker
	s.ticker = time.NewTicker(s.config.TickInterval)
	s.running = true

	go s.run(ctx)

	s.logger.Info("KAIROS service started")
	return nil
}

// Stop stops the KAIROS service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.ticker.Stop()
	close(s.stopCh)
	s.running = false

	// Save observations
	s.saveObservations()

	s.logger.Info("KAIROS service stopped")
	return nil
}

// IsRunning returns true if the service is running
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// run is the main service loop
func (s *Service) run(ctx context.Context) {
	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			s.Stop()
			return
		case <-s.ticker.C:
			s.tick(ctx)
		}
	}
}

// tick processes a tick event
func (s *Service) tick(ctx context.Context) {
	// Update context
	s.updateContext()

	// Create tick prompt
	prompt := TickPrompt{
		Type:       "tick",
		Timestamp:  time.Now(),
		Context:    s.currentContext,
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

	// Execute with timeout
	actionCtx, cancel := context.WithTimeout(ctx, s.config.BlockingBudget)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.onAction(action)
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
	// Get working directory
	wd, _ := os.Getwd()

	// Get git info
	branch := s.getGitBranch(wd)
	modifiedFiles := s.getModifiedFiles(wd)

	s.mu.Lock()
	s.currentContext = ObservedContext{
		WorkingDirectory: wd,
		GitBranch:        branch,
		ModifiedFiles:    modifiedFiles,
		RecentCommands:   s.currentContext.RecentCommands,
	}
	s.mu.Unlock()
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

	s.mu.Lock()
	s.observations = append(s.observations, obs)
	// Keep only last 1000 observations
	if len(s.observations) > 1000 {
		s.observations = s.observations[len(s.observations)-1000:]
	}
	s.mu.Unlock()

	s.onObservation(obs)

	// Write to daily log
	s.writeToDailyLog(obs)
}

// recordAction records an action
func (s *Service) recordAction(action Action) {
	s.mu.Lock()
	s.actions = append(s.actions, action)
	s.mu.Unlock()
}

// updateAction updates an existing action
func (s *Service) updateAction(action Action) {
	s.mu.Lock()
	for i, a := range s.actions {
		if a.ID == action.ID {
			s.actions[i] = action
			break
		}
	}
	s.mu.Unlock()
}

// GetObservations returns all observations
func (s *Service) GetObservations() []Observation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Observation, len(s.observations))
	copy(result, s.observations)
	return result
}

// GetActions returns all actions
func (s *Service) GetActions() []Action {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Action, len(s.actions))
	copy(result, s.actions)
	return result
}

// getRecentObservations returns recent observations
func (s *Service) getRecentObservations(count int) []Observation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.observations) <= count {
		result := make([]Observation, len(s.observations))
		copy(result, s.observations)
		return result
	}

	result := make([]Observation, count)
	copy(result, s.observations[len(s.observations)-count:])
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	var dayObservations []Observation
	var dayActions []Action

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	for _, obs := range s.observations {
		if obs.Timestamp.After(startOfDay) && obs.Timestamp.Before(endOfDay) {
			dayObservations = append(dayObservations, obs)
		}
	}

	for _, action := range s.actions {
		if action.Timestamp.After(startOfDay) && action.Timestamp.Before(endOfDay) {
			dayActions = append(dayActions, action)
		}
	}

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
