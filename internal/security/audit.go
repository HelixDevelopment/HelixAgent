package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// InMemoryAuditLogger provides an in-memory audit log implementation.
// mu is Pattern Zeta: it serialises the compound "cap-check + trim +
// append" in Log so the maxEvents invariant holds under concurrent
// writers. Events themselves live in a safe.Slice; *AuditEvent values
// are immutable post-append (Pattern Alpha).
type InMemoryAuditLogger struct {
	events    *safe.Slice[*AuditEvent]
	maxEvents int
	logger    *logrus.Logger
	mu        sync.Mutex
}

// NewInMemoryAuditLogger creates a new in-memory audit logger
func NewInMemoryAuditLogger(maxEvents int, logger *logrus.Logger) *InMemoryAuditLogger {
	if maxEvents <= 0 {
		maxEvents = 10000
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &InMemoryAuditLogger{
		events:    safe.NewSlice[*AuditEvent](),
		maxEvents: maxEvents,
		logger:    logger,
	}
}

// Log logs an audit event
func (l *InMemoryAuditLogger) Log(ctx context.Context, event *AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Enforce max events limit: trim the oldest 10% if we hit the cap.
	if l.events.Len() >= l.maxEvents {
		snap := l.events.Snapshot()
		removeCount := l.maxEvents / 10
		if removeCount > len(snap) {
			removeCount = len(snap)
		}
		l.events.Replace(snap[removeCount:])
	}

	l.events.Append(event)

	// Also log to structured logger
	l.logger.WithFields(logrus.Fields{
		"audit_id":   event.ID,
		"event_type": event.EventType,
		"action":     event.Action,
		"result":     event.Result,
		"risk":       event.Risk,
		"user_id":    event.UserID,
	}).Info("Audit event logged")

	return nil
}

// Query queries audit events with filtering
func (l *InMemoryAuditLogger) Query(ctx context.Context, filter *AuditFilter) ([]*AuditEvent, error) {
	events := l.events.Snapshot()

	var results []*AuditEvent

	for _, event := range events {
		// Apply filters
		if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
			continue
		}
		if filter.UserID != "" && event.UserID != filter.UserID {
			continue
		}
		if len(filter.EventTypes) > 0 {
			found := false
			for _, t := range filter.EventTypes {
				if event.EventType == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if filter.MinRisk != "" {
			if !l.isRiskAtLeast(event.Risk, filter.MinRisk) {
				continue
			}
		}

		results = append(results, event)
	}

	// Sort by timestamp (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// Apply limit
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

// GetStats returns audit statistics
func (l *InMemoryAuditLogger) GetStats(ctx context.Context, since time.Time) (*AuditStats, error) {
	events := l.events.Snapshot()

	stats := &AuditStats{
		EventsByType: make(map[AuditEventType]int64),
		EventsByRisk: make(map[Severity]int64),
		TopUsers:     make([]UserAuditStat, 0),
	}

	userStats := make(map[string]*UserAuditStat)

	for _, event := range events {
		if event.Timestamp.Before(since) {
			continue
		}

		stats.TotalEvents++
		stats.EventsByType[event.EventType]++
		stats.EventsByRisk[event.Risk]++

		// Track per-user stats
		if event.UserID != "" {
			if _, exists := userStats[event.UserID]; !exists {
				userStats[event.UserID] = &UserAuditStat{
					UserID: event.UserID,
				}
			}
			userStats[event.UserID].Events++
			if event.EventType == AuditEventGuardrailBlock || event.EventType == AuditEventPermissionDeny {
				userStats[event.UserID].Blocks++
			}
		}
	}

	// Convert user stats to slice and sort by events
	for _, us := range userStats {
		if us.Events > 0 {
			us.RiskScore = float64(us.Blocks) / float64(us.Events)
		}
		stats.TopUsers = append(stats.TopUsers, *us)
	}
	sort.Slice(stats.TopUsers, func(i, j int) bool {
		return stats.TopUsers[i].Events > stats.TopUsers[j].Events
	})
	if len(stats.TopUsers) > 10 {
		stats.TopUsers = stats.TopUsers[:10]
	}

	return stats, nil
}

func (l *InMemoryAuditLogger) isRiskAtLeast(actual, minimum Severity) bool {
	riskOrder := map[Severity]int{
		SeverityInfo:     0,
		SeverityLow:      1,
		SeverityMedium:   2,
		SeverityHigh:     3,
		SeverityCritical: 4,
	}
	return riskOrder[actual] >= riskOrder[minimum]
}

// FileAuditLogger logs audit events to a file
type FileAuditLogger struct {
	file   *os.File
	logger *logrus.Logger
	mu     sync.Mutex
}

// NewFileAuditLogger creates a new file-based audit logger
func NewFileAuditLogger(filename string, logger *logrus.Logger) (*FileAuditLogger, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &FileAuditLogger{
		file:   file,
		logger: logger,
	}, nil
}

// Log logs an audit event to the file
func (l *FileAuditLogger) Log(ctx context.Context, event *AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = l.file.WriteString(string(data) + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to audit log: %w", err)
	}

	return nil
}

// Query is not efficiently supported for file-based logger
func (l *FileAuditLogger) Query(ctx context.Context, filter *AuditFilter) ([]*AuditEvent, error) {
	return nil, fmt.Errorf("query not supported for file-based audit logger")
}

// GetStats is not efficiently supported for file-based logger
func (l *FileAuditLogger) GetStats(ctx context.Context, since time.Time) (*AuditStats, error) {
	return nil, fmt.Errorf("stats not supported for file-based audit logger")
}

// Close closes the file
func (l *FileAuditLogger) Close() error {
	return l.file.Close()
}

// CompositeAuditLogger combines multiple audit loggers
type CompositeAuditLogger struct {
	loggers []AuditLogger
}

// NewCompositeAuditLogger creates a composite logger
func NewCompositeAuditLogger(loggers ...AuditLogger) *CompositeAuditLogger {
	return &CompositeAuditLogger{
		loggers: loggers,
	}
}

// AddLogger adds a logger to the composite
func (l *CompositeAuditLogger) AddLogger(logger AuditLogger) {
	l.loggers = append(l.loggers, logger)
}

// Log logs to all underlying loggers
func (l *CompositeAuditLogger) Log(ctx context.Context, event *AuditEvent) error {
	var lastErr error
	for _, logger := range l.loggers {
		if err := logger.Log(ctx, event); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Query queries the first logger that supports it
func (l *CompositeAuditLogger) Query(ctx context.Context, filter *AuditFilter) ([]*AuditEvent, error) {
	for _, logger := range l.loggers {
		events, err := logger.Query(ctx, filter)
		if err == nil {
			return events, nil
		}
	}
	return nil, fmt.Errorf("no logger supports query")
}

// GetStats gets stats from the first logger that supports it
func (l *CompositeAuditLogger) GetStats(ctx context.Context, since time.Time) (*AuditStats, error) {
	for _, logger := range l.loggers {
		stats, err := logger.GetStats(ctx, since)
		if err == nil {
			return stats, nil
		}
	}
	return nil, fmt.Errorf("no logger supports stats")
}
