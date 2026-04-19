package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"digital.vasic.concurrency/pkg/safe"
)

// MonitoringConfig holds configuration for monitoring
type MonitoringConfig struct {
	CheckInterval     time.Duration
	AlertThreshold    int
	HealthCheckPeriod time.Duration
}

// ExtendedDebateStatus extends DebateStatus with monitoring-specific fields
type ExtendedDebateStatus struct {
	DebateStatus
	LastUpdateTime time.Time `json:"last_update_time"`
	HealthScore    float64   `json:"health_score"`
	ErrorCount     int       `json:"error_count"`
	WarningCount   int       `json:"warning_count"`
}

// MonitoringSession represents an active monitoring session
type MonitoringSession struct {
	ID            string
	DebateID      string
	Config        *DebateConfig
	Status        *ExtendedDebateStatus
	StartTime     time.Time
	LastCheck     time.Time
	Active        bool
	Alerts        []MonitoringAlert
	CancelFunc    context.CancelFunc
	monitoringCtx context.Context
}

// MonitoringAlert represents an alert generated during monitoring
type MonitoringAlert struct {
	ID         string    `json:"id"`
	DebateID   string    `json:"debate_id"`
	Level      string    `json:"level"` // info, warning, error, critical
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Resolved   bool      `json:"resolved"`
	ResolvedAt time.Time `json:"resolved_at,omitempty"`
}

// DebateMonitoringService provides monitoring capabilities.
// Concurrent-safe by construction (CONST-029): sessions is a safe.Store.
// All session-field mutations go through safe.Store.Update callbacks,
// preserving the pre-migration lock-discipline for session-pointer writes.
type DebateMonitoringService struct {
	logger   *logrus.Logger
	sessions *safe.Store[string, *MonitoringSession]
	config   *MonitoringConfig
}

// NewDebateMonitoringService creates a new monitoring service
func NewDebateMonitoringService(logger *logrus.Logger) *DebateMonitoringService {
	return &DebateMonitoringService{
		logger:   logger,
		sessions: safe.NewStore[string, *MonitoringSession](),
		config: &MonitoringConfig{
			CheckInterval:     time.Second * 5,
			AlertThreshold:    3,
			HealthCheckPeriod: time.Second * 30,
		},
	}
}

// NewDebateMonitoringServiceWithConfig creates a monitoring service with custom config
func NewDebateMonitoringServiceWithConfig(logger *logrus.Logger, config *MonitoringConfig) *DebateMonitoringService {
	if config == nil {
		config = &MonitoringConfig{
			CheckInterval:     time.Second * 5,
			AlertThreshold:    3,
			HealthCheckPeriod: time.Second * 30,
		}
	}
	return &DebateMonitoringService{
		logger:   logger,
		sessions: safe.NewStore[string, *MonitoringSession](),
		config:   config,
	}
}

// StartMonitoring starts monitoring for a debate
func (dms *DebateMonitoringService) StartMonitoring(ctx context.Context, config *DebateConfig) (string, error) {
	if config == nil {
		return "", fmt.Errorf("debate config is required")
	}

	monitoringID := "mon-" + uuid.New().String()[:8]

	// Create cancellable context for this monitoring session
	monitoringCtx, cancelFunc := context.WithCancel(ctx)

	session := &MonitoringSession{
		ID:       monitoringID,
		DebateID: config.DebateID,
		Config:   config,
		Status: &ExtendedDebateStatus{
			DebateStatus: DebateStatus{
				DebateID:     config.DebateID,
				Status:       "pending",
				CurrentRound: 0,
				TotalRounds:  config.MaxRounds,
				StartTime:    time.Now(),
				Participants: make([]ParticipantStatus, 0),
			},
			HealthScore: 100.0,
		},
		StartTime:     time.Now(),
		LastCheck:     time.Now(),
		Active:        true,
		Alerts:        make([]MonitoringAlert, 0),
		CancelFunc:    cancelFunc,
		monitoringCtx: monitoringCtx,
	}

	// Initialize participant status
	for _, participant := range config.Participants {
		session.Status.Participants = append(session.Status.Participants, ParticipantStatus{
			ParticipantID:   participant.ParticipantID,
			ParticipantName: participant.Name,
			Status:          "pending",
		})
	}

	dms.sessions.Put(monitoringID, session)

	// Start background monitoring goroutine
	go dms.runMonitoringLoop(session)

	dms.logger.WithFields(logrus.Fields{
		"monitoring_id": monitoringID,
		"debate_id":     config.DebateID,
	}).Info("Started monitoring for debate")

	return monitoringID, nil
}

// runMonitoringLoop runs the background monitoring loop
func (dms *DebateMonitoringService) runMonitoringLoop(session *MonitoringSession) {
	ticker := time.NewTicker(dms.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-session.monitoringCtx.Done():
			dms.logger.Infof("Monitoring loop stopped for %s", session.ID)
			return
		case <-ticker.C:
			dms.performHealthCheck(session)
		}
	}
}

// performHealthCheck performs a health check on the monitoring session.
// Runs inside a Store.Update callback to serialize with other mutators.
func (dms *DebateMonitoringService) performHealthCheck(session *MonitoringSession) {
	dms.sessions.Update(session.ID, func(s *MonitoringSession, ok bool) (*MonitoringSession, bool) {
		if !ok {
			return nil, false
		}
		dms.doPerformHealthCheck(s)
		return s, true
	})
}

func (dms *DebateMonitoringService) doPerformHealthCheck(session *MonitoringSession) {
	if !session.Active {
		return
	}

	session.LastCheck = time.Now()

	// Update health score based on status
	healthScore := 100.0

	// Reduce health for errors
	if session.Status.ErrorCount > 0 {
		healthScore -= float64(session.Status.ErrorCount) * 10
	}

	// Reduce health for warnings
	if session.Status.WarningCount > 0 {
		healthScore -= float64(session.Status.WarningCount) * 5
	}

	// Check for stale participants (based on status)
	for i := range session.Status.Participants {
		p := &session.Status.Participants[i]
		if p.Status == "active" && p.ResponseTime == 0 {
			// Mark as potentially stale if no response recorded
			healthScore -= 5
		}
	}

	// Ensure health score is within bounds
	if healthScore < 0 {
		healthScore = 0
	}
	session.Status.HealthScore = healthScore

	// Generate critical alert if health is too low
	if healthScore < 50 && session.Status.Status != "failed" {
		dms.addAlert(session, "critical", "Debate health is critically low")
	}
}

// addAlert adds an alert to the monitoring session
func (dms *DebateMonitoringService) addAlert(session *MonitoringSession, level, message string) {
	alert := MonitoringAlert{
		ID:        "alert-" + uuid.New().String()[:8],
		DebateID:  session.DebateID,
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
	}

	session.Alerts = append(session.Alerts, alert)

	dms.logger.WithFields(logrus.Fields{
		"alert_id":  alert.ID,
		"debate_id": alert.DebateID,
		"level":     level,
		"message":   message,
	}).Warn("Monitoring alert generated")
}

// StopMonitoring stops monitoring for a debate
func (dms *DebateMonitoringService) StopMonitoring(ctx context.Context, monitoringID string) error {
	var notFound bool
	var debateID string
	dms.sessions.Update(monitoringID, func(session *MonitoringSession, ok bool) (*MonitoringSession, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		session.Active = false
		if session.CancelFunc != nil {
			session.CancelFunc()
		}
		debateID = session.DebateID
		return session, true
	})
	if notFound {
		return fmt.Errorf("monitoring session not found: %s", monitoringID)
	}
	dms.logger.Infof("Stopped monitoring %s for debate %s", monitoringID, debateID)
	return nil
}

// GetStatus retrieves the current status of a debate. Scans by debateID.
// Find and copy under a single Update callback to prevent reader/writer
// races on session.Status.* fields.
func (dms *DebateMonitoringService) GetStatus(ctx context.Context, debateID string) (*DebateStatus, error) {
	var result *DebateStatus
	for _, session := range dms.sessions.Snapshot() {
		if session.DebateID != debateID {
			continue
		}
		dms.sessions.Update(session.ID, func(s *MonitoringSession, ok bool) (*MonitoringSession, bool) {
			if !ok {
				return nil, false
			}
			statusCopy := s.Status.DebateStatus
			result = &statusCopy
			return s, true
		})
		if result != nil {
			return result, nil
		}
	}
	return nil, fmt.Errorf("no monitoring session found for debate: %s", debateID)
}

// GetStatusByMonitoringID retrieves status by monitoring ID
func (dms *DebateMonitoringService) GetStatusByMonitoringID(ctx context.Context, monitoringID string) (*DebateStatus, error) {
	var result *DebateStatus
	var notFound bool
	dms.sessions.Update(monitoringID, func(session *MonitoringSession, ok bool) (*MonitoringSession, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		statusCopy := session.Status.DebateStatus
		result = &statusCopy
		return session, true
	})
	if notFound {
		return nil, fmt.Errorf("monitoring session not found: %s", monitoringID)
	}
	return result, nil
}

// GetExtendedStatus retrieves the full extended status including health metrics
func (dms *DebateMonitoringService) GetExtendedStatus(ctx context.Context, monitoringID string) (*ExtendedDebateStatus, error) {
	var result *ExtendedDebateStatus
	var notFound bool
	dms.sessions.Update(monitoringID, func(session *MonitoringSession, ok bool) (*MonitoringSession, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		statusCopy := *session.Status
		result = &statusCopy
		return session, true
	})
	if notFound {
		return nil, fmt.Errorf("monitoring session not found: %s", monitoringID)
	}
	return result, nil
}

// UpdateParticipantStatus updates the status of a participant
func (dms *DebateMonitoringService) UpdateParticipantStatus(
	ctx context.Context,
	monitoringID string,
	participantID string,
	status string,
	responseTime time.Duration,
) error {
	var notFound, participantMissing bool
	dms.sessions.Update(monitoringID, func(session *MonitoringSession, ok bool) (*MonitoringSession, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		found := false
		for i := range session.Status.Participants {
			p := &session.Status.Participants[i]
			if p.ParticipantID == participantID {
				p.Status = status
				p.ResponseTime = responseTime
				session.Status.LastUpdateTime = time.Now()
				found = true
				break
			}
		}
		if !found {
			participantMissing = true
		}
		return session, true
	})
	if notFound {
		return fmt.Errorf("monitoring session not found: %s", monitoringID)
	}
	if participantMissing {
		return fmt.Errorf("participant not found: %s", participantID)
	}
	return nil
}

// UpdateRound updates the current round of a debate
func (dms *DebateMonitoringService) UpdateRound(ctx context.Context, monitoringID string, round int) error {
	var notFound bool
	dms.sessions.Update(monitoringID, func(session *MonitoringSession, ok bool) (*MonitoringSession, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		session.Status.CurrentRound = round
		session.Status.LastUpdateTime = time.Now()
		if round > 0 && session.Status.Status == "pending" {
			session.Status.Status = "active"
		}
		if round >= session.Status.TotalRounds {
			session.Status.Status = "completed"
		}
		return session, true
	})
	if notFound {
		return fmt.Errorf("monitoring session not found: %s", monitoringID)
	}
	return nil
}

// RecordError records an error during debate
func (dms *DebateMonitoringService) RecordError(ctx context.Context, monitoringID string, errMsg string) error {
	var notFound bool
	dms.sessions.Update(monitoringID, func(session *MonitoringSession, ok bool) (*MonitoringSession, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		session.Status.ErrorCount++
		dms.addAlert(session, "error", errMsg)
		if session.Status.ErrorCount >= dms.config.AlertThreshold {
			session.Status.Status = "failed"
			dms.addAlert(session, "critical", "Debate failed due to too many errors")
		}
		return session, true
	})
	if notFound {
		return fmt.Errorf("monitoring session not found: %s", monitoringID)
	}
	return nil
}

// GetAlerts retrieves alerts for a monitoring session. Copy under Update
// lock to avoid racing with RecordError appending to session.Alerts.
func (dms *DebateMonitoringService) GetAlerts(ctx context.Context, monitoringID string) ([]MonitoringAlert, error) {
	var alerts []MonitoringAlert
	var notFound bool
	dms.sessions.Update(monitoringID, func(session *MonitoringSession, ok bool) (*MonitoringSession, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		alerts = make([]MonitoringAlert, len(session.Alerts))
		copy(alerts, session.Alerts)
		return session, true
	})
	if notFound {
		return nil, fmt.Errorf("monitoring session not found: %s", monitoringID)
	}
	return alerts, nil
}

// ListActiveSessions returns all active monitoring session IDs
func (dms *DebateMonitoringService) ListActiveSessions() []string {
	ids := make([]string, 0)
	for id, session := range dms.sessions.Snapshot() {
		if session.Active {
			ids = append(ids, id)
		}
	}
	return ids
}

// CleanupInactiveSessions removes inactive sessions older than maxAge
func (dms *DebateMonitoringService) CleanupInactiveSessions(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, session := range dms.sessions.Snapshot() {
		if !session.Active && session.LastCheck.Before(cutoff) {
			if _, ok := dms.sessions.Delete(id); ok {
				removed++
			}
		}
	}

	if removed > 0 {
		dms.logger.Infof("Cleaned up %d inactive monitoring sessions", removed)
	}

	return removed
}

// GetStats returns monitoring service statistics
func (dms *DebateMonitoringService) GetStats() map[string]interface{} {
	snap := dms.sessions.Snapshot()
	stats := map[string]interface{}{
		"total_sessions":  len(snap),
		"active_sessions": 0,
		"total_alerts":    0,
		"critical_alerts": 0,
		"error_alerts":    0,
		"warning_alerts":  0,
	}

	for _, session := range snap {
		if session.Active {
			stats["active_sessions"] = stats["active_sessions"].(int) + 1 //nolint:errcheck
		}
		for _, alert := range session.Alerts {
			stats["total_alerts"] = stats["total_alerts"].(int) + 1 //nolint:errcheck
			switch alert.Level {
			case "critical":
				stats["critical_alerts"] = stats["critical_alerts"].(int) + 1 //nolint:errcheck
			case "error":
				stats["error_alerts"] = stats["error_alerts"].(int) + 1 //nolint:errcheck
			case "warning":
				stats["warning_alerts"] = stats["warning_alerts"].(int) + 1 //nolint:errcheck
			}
		}
	}

	return stats
}
