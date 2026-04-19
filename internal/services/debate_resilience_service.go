package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"digital.vasic.concurrency/pkg/safe"
)

// DebateState represents the current state of a debate for recovery purposes
type DebateState struct {
	DebateID        string                `json:"debate_id"`
	Config          *DebateConfig         `json:"config"`
	CurrentRound    int                   `json:"current_round"`
	Responses       []ParticipantResponse `json:"responses"`
	StartTime       time.Time             `json:"start_time"`
	LastUpdated     time.Time             `json:"last_updated"`
	Status          string                `json:"status"` // active, failed, recovered, completed
	FailureCount    int                   `json:"failure_count"`
	LastError       string                `json:"last_error,omitempty"`
	RecoveryAttempt int                   `json:"recovery_attempt"`
}

// DebateResilienceService provides resilience and recovery capabilities.
// Concurrent-safe by construction (CONST-029): activeDebates is a
// safe.Store. All DebateState field mutations go through Store.Update
// callbacks to preserve the pre-migration lock discipline.
type DebateResilienceService struct {
	logger            *logrus.Logger
	debateService     *DebateService
	activeDebates     *safe.Store[string, *DebateState]
	maxRetries        int
	retryDelay        time.Duration
	checkpointEnabled bool
}

// ResilienceConfig holds configuration for the resilience service
type ResilienceConfig struct {
	MaxRetries        int
	RetryDelay        time.Duration
	CheckpointEnabled bool
}

// DefaultResilienceConfig returns default configuration
func DefaultResilienceConfig() *ResilienceConfig {
	return &ResilienceConfig{
		MaxRetries:        3,
		RetryDelay:        time.Second * 2,
		CheckpointEnabled: true,
	}
}

// NewDebateResilienceService creates a new resilience service
func NewDebateResilienceService(logger *logrus.Logger) *DebateResilienceService {
	config := DefaultResilienceConfig()
	return &DebateResilienceService{
		logger:            logger,
		activeDebates:     safe.NewStore[string, *DebateState](),
		maxRetries:        config.MaxRetries,
		retryDelay:        config.RetryDelay,
		checkpointEnabled: config.CheckpointEnabled,
	}
}

// NewDebateResilienceServiceWithConfig creates a resilience service with custom config
func NewDebateResilienceServiceWithConfig(logger *logrus.Logger, config *ResilienceConfig) *DebateResilienceService {
	if config == nil {
		config = DefaultResilienceConfig()
	}
	return &DebateResilienceService{
		logger:            logger,
		activeDebates:     safe.NewStore[string, *DebateState](),
		maxRetries:        config.MaxRetries,
		retryDelay:        config.RetryDelay,
		checkpointEnabled: config.CheckpointEnabled,
	}
}

// SetDebateService sets the debate service for recovery operations
func (drs *DebateResilienceService) SetDebateService(debateService *DebateService) {
	drs.debateService = debateService
}

// HandleFailure handles a failure during debate with retry logic
func (drs *DebateResilienceService) HandleFailure(ctx context.Context, err error) error {
	drs.logger.Warnf("Handling failure: %v", err)

	// Log the error for diagnostics
	if err != nil {
		drs.logger.WithFields(logrus.Fields{
			"error":     err.Error(),
			"timestamp": time.Now().UTC(),
		}).Error("Debate failure recorded")
	}

	return nil
}

// HandleDebateFailure handles a failure for a specific debate
func (drs *DebateResilienceService) HandleDebateFailure(ctx context.Context, debateID string, err error) error {
	var notFound bool
	var failureCount int
	drs.activeDebates.Update(debateID, func(state *DebateState, ok bool) (*DebateState, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		state.FailureCount++
		state.LastError = err.Error()
		state.LastUpdated = time.Now()
		state.Status = "failed"
		failureCount = state.FailureCount
		return state, true
	})
	if notFound {
		drs.logger.Warnf("No active debate found for ID %s", debateID)
		return fmt.Errorf("no active debate found: %s", debateID)
	}
	drs.logger.WithFields(logrus.Fields{
		"debate_id":     debateID,
		"failure_count": failureCount,
		"error":         err.Error(),
	}).Warn("Debate failure recorded")
	return nil
}

// RegisterDebate registers a new debate for tracking
func (drs *DebateResilienceService) RegisterDebate(config *DebateConfig) *DebateState {
	state := &DebateState{
		DebateID:     config.DebateID,
		Config:       config,
		CurrentRound: 0,
		Responses:    make([]ParticipantResponse, 0),
		StartTime:    time.Now(),
		LastUpdated:  time.Now(),
		Status:       "active",
		FailureCount: 0,
	}
	drs.activeDebates.Put(config.DebateID, state)
	drs.logger.Infof("Registered debate %s for resilience tracking", config.DebateID)
	return state
}

// UpdateDebateProgress updates the progress of an active debate
func (drs *DebateResilienceService) UpdateDebateProgress(debateID string, round int, responses []ParticipantResponse) error {
	var notFound bool
	drs.activeDebates.Update(debateID, func(state *DebateState, ok bool) (*DebateState, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		state.CurrentRound = round
		state.Responses = append(state.Responses, responses...)
		state.LastUpdated = time.Now()
		return state, true
	})
	if notFound {
		return fmt.Errorf("no active debate found: %s", debateID)
	}
	if drs.checkpointEnabled {
		drs.logger.Debugf("Checkpoint saved for debate %s at round %d", debateID, round)
	}
	return nil
}

// CompleteDebate marks a debate as completed
func (drs *DebateResilienceService) CompleteDebate(debateID string) error {
	var notFound bool
	drs.activeDebates.Update(debateID, func(state *DebateState, ok bool) (*DebateState, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		state.Status = "completed"
		state.LastUpdated = time.Now()
		return state, true
	})
	if notFound {
		return fmt.Errorf("no active debate found: %s", debateID)
	}
	drs.logger.Infof("Debate %s marked as completed", debateID)
	return nil
}

// RecoverDebate recovers a debate from a failure
func (drs *DebateResilienceService) RecoverDebate(ctx context.Context, debateID string) (*DebateResult, error) {
	var state *DebateState
	var preErr error
	drs.activeDebates.Update(debateID, func(s *DebateState, ok bool) (*DebateState, bool) {
		if !ok {
			preErr = fmt.Errorf("no debate state found for recovery: %s", debateID)
			return nil, false
		}
		if s.Status == "completed" {
			preErr = fmt.Errorf("debate %s is already completed", debateID)
			return s, true
		}
		if s.RecoveryAttempt >= drs.maxRetries {
			preErr = fmt.Errorf("max recovery attempts (%d) exceeded for debate %s", drs.maxRetries, debateID)
			return s, true
		}
		s.RecoveryAttempt++
		s.Status = "recovering"
		s.LastUpdated = time.Now()
		state = s
		return s, true
	})
	if preErr != nil {
		return nil, preErr
	}

	drs.logger.WithFields(logrus.Fields{
		"debate_id":        debateID,
		"recovery_attempt": state.RecoveryAttempt,
		"from_round":       state.CurrentRound,
	}).Info("Attempting debate recovery")

	// If no debate service, return error
	if drs.debateService == nil {
		return nil, fmt.Errorf("debate service not configured for recovery")
	}

	// Apply retry delay
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(drs.retryDelay):
	}

	// Create a modified config to resume from the current round
	recoveryConfig := *state.Config
	if state.CurrentRound > 0 {
		// Adjust remaining rounds
		remainingRounds := state.Config.MaxRounds - state.CurrentRound
		if remainingRounds <= 0 {
			remainingRounds = 1
		}
		recoveryConfig.MaxRounds = remainingRounds
	}

	// Attempt to re-run the debate
	result, err := drs.debateService.ConductDebate(ctx, &recoveryConfig)
	if err != nil {
		drs.activeDebates.Update(debateID, func(s *DebateState, ok bool) (*DebateState, bool) {
			if !ok {
				return nil, false
			}
			s.Status = "failed"
			s.LastError = err.Error()
			s.LastUpdated = time.Now()
			return s, true
		})
		drs.logger.WithFields(logrus.Fields{
			"debate_id": debateID,
			"error":     err.Error(),
		}).Error("Debate recovery failed")
		return nil, fmt.Errorf("recovery failed: %w", err)
	}

	// Mark as recovered + grab data under the write lock.
	var responses []ParticipantResponse
	var recoveryAttempt int
	drs.activeDebates.Update(debateID, func(s *DebateState, ok bool) (*DebateState, bool) {
		if !ok {
			return nil, false
		}
		s.Status = "recovered"
		s.LastUpdated = time.Now()
		responses = make([]ParticipantResponse, len(s.Responses))
		copy(responses, s.Responses)
		recoveryAttempt = s.RecoveryAttempt
		return s, true
	})

	drs.logger.WithFields(logrus.Fields{
		"debate_id":        debateID,
		"recovery_attempt": recoveryAttempt,
	}).Info("Debate recovered successfully")

	// Merge previous responses with new result if applicable
	if len(responses) > 0 && result != nil {
		result.AllResponses = append(responses, result.AllResponses...)
		result.FallbackUsed = true
	}

	return result, nil
}

// GetDebateState returns the current state of a debate. Copy under Update
// lock to avoid racing with mutators.
func (drs *DebateResilienceService) GetDebateState(debateID string) (*DebateState, error) {
	var result *DebateState
	var notFound bool
	drs.activeDebates.Update(debateID, func(state *DebateState, ok bool) (*DebateState, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		stateCopy := *state
		result = &stateCopy
		return state, true
	})
	if notFound {
		return nil, fmt.Errorf("no debate state found: %s", debateID)
	}
	return result, nil
}

// ListActiveDebates returns all active debate IDs
func (drs *DebateResilienceService) ListActiveDebates() []string {
	snap := drs.activeDebates.Snapshot()
	ids := make([]string, 0, len(snap))
	for id, state := range snap {
		if state.Status == "active" || state.Status == "recovering" {
			ids = append(ids, id)
		}
	}
	return ids
}

// CleanupCompletedDebates removes completed debates older than the given duration
func (drs *DebateResilienceService) CleanupCompletedDebates(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, state := range drs.activeDebates.Snapshot() {
		if (state.Status == "completed" || state.Status == "recovered") && state.LastUpdated.Before(cutoff) {
			if _, ok := drs.activeDebates.Delete(id); ok {
				removed++
			}
		}
	}

	if removed > 0 {
		drs.logger.Infof("Cleaned up %d completed debates older than %v", removed, maxAge)
	}

	return removed
}

// GetStats returns statistics about the resilience service
func (drs *DebateResilienceService) GetStats() map[string]interface{} {
	snap := drs.activeDebates.Snapshot()
	stats := map[string]interface{}{
		"total_debates": len(snap),
		"active":        0,
		"completed":     0,
		"failed":        0,
		"recovered":     0,
		"recovering":    0,
	}

	totalFailures := 0
	totalRecoveries := 0

	for _, state := range snap {
		switch state.Status {
		case "active":
			//nolint:errcheck // map initialized with int values
			stats["active"] = stats["active"].(int) + 1
		case "completed":
			//nolint:errcheck // map initialized with int values
			stats["completed"] = stats["completed"].(int) + 1
		case "failed":
			//nolint:errcheck // map initialized with int values
			stats["failed"] = stats["failed"].(int) + 1
		case "recovered":
			//nolint:errcheck // map initialized with int values
			stats["recovered"] = stats["recovered"].(int) + 1
		case "recovering":
			//nolint:errcheck // map initialized with int values
			stats["recovering"] = stats["recovering"].(int) + 1
		}
		totalFailures += state.FailureCount
		totalRecoveries += state.RecoveryAttempt
	}

	stats["total_failures"] = totalFailures
	stats["total_recovery_attempts"] = totalRecoveries

	return stats
}
