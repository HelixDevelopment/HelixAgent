// Package messaging provides migration support for transitioning from
// PostgreSQL-backed task queues to RabbitMQ/Kafka messaging.
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// MigrationMode represents the current migration state.
type MigrationMode int

const (
	// ModeLegacy uses only the legacy PostgreSQL task queue.
	ModeLegacy MigrationMode = iota
	// ModeDualWrite writes to both PostgreSQL and RabbitMQ.
	ModeDualWrite
	// ModeMessaging uses only the new messaging system (RabbitMQ/Kafka).
	ModeMessaging
	// ModeRollback emergency rollback to legacy system.
	ModeRollback
)

// String returns the string representation of MigrationMode.
func (m MigrationMode) String() string {
	switch m {
	case ModeLegacy:
		return "legacy"
	case ModeDualWrite:
		return "dual_write"
	case ModeMessaging:
		return "messaging"
	case ModeRollback:
		return "rollback"
	default:
		return "unknown"
	}
}

// ParseMigrationMode parses a string into a MigrationMode.
func ParseMigrationMode(s string) MigrationMode {
	switch s {
	case "legacy":
		return ModeLegacy
	case "dual_write":
		return ModeDualWrite
	case "messaging":
		return ModeMessaging
	case "rollback":
		return ModeRollback
	default:
		return ModeLegacy
	}
}

// MigrationConfig holds configuration for the migration manager.
type MigrationConfig struct {
	// Mode is the current migration mode.
	Mode MigrationMode `json:"mode" yaml:"mode"`
	// VerifyConsistency enables consistency checks in dual-write mode.
	VerifyConsistency bool `json:"verify_consistency" yaml:"verify_consistency"`
	// LogDiscrepancies logs any discrepancies found during dual-write.
	LogDiscrepancies bool `json:"log_discrepancies" yaml:"log_discrepancies"`
	// ConsumerTrafficSplit defines traffic split percentage (0-100) for RabbitMQ.
	ConsumerTrafficSplit int `json:"consumer_traffic_split" yaml:"consumer_traffic_split"`
	// AutoRollback enables automatic rollback on errors.
	AutoRollback bool `json:"auto_rollback" yaml:"auto_rollback"`
	// ErrorThreshold is errors per minute before triggering rollback.
	ErrorThreshold int `json:"error_threshold" yaml:"error_threshold"`
	// LatencyThreshold is max latency (ms) before triggering rollback.
	LatencyThreshold int `json:"latency_threshold" yaml:"latency_threshold"`
	// DualWriteTimeout is timeout for dual-write operations.
	DualWriteTimeout time.Duration `json:"dual_write_timeout" yaml:"dual_write_timeout"`
}

// DefaultMigrationConfig returns default migration configuration.
func DefaultMigrationConfig() *MigrationConfig {
	return &MigrationConfig{
		Mode:                 ModeLegacy,
		VerifyConsistency:    true,
		LogDiscrepancies:     true,
		ConsumerTrafficSplit: 0,
		AutoRollback:         false,
		ErrorThreshold:       10,
		LatencyThreshold:     5000,
		DualWriteTimeout:     10 * time.Second,
	}
}

// LegacyTaskQueue defines the interface for the legacy PostgreSQL task queue.
type LegacyTaskQueue interface {
	// Enqueue adds a task to the queue.
	Enqueue(ctx context.Context, taskType string, payload []byte, priority int) (string, error)
	// Dequeue retrieves and locks a task for processing.
	Dequeue(ctx context.Context, workerID string) (*LegacyTask, error)
	// Complete marks a task as completed.
	Complete(ctx context.Context, taskID string, result []byte) error
	// Fail marks a task as failed.
	Fail(ctx context.Context, taskID string, err error) error
	// GetPendingTasks returns all pending tasks.
	GetPendingTasks(ctx context.Context) ([]*LegacyTask, error)
	// GetTask returns a task by ID.
	GetTask(ctx context.Context, taskID string) (*LegacyTask, error)
	// MarkMigrated marks a task as migrated to the new system.
	MarkMigrated(ctx context.Context, taskID string) error
}

// LegacyTask represents a task in the legacy system.
type LegacyTask struct {
	ID          string
	Type        string
	Payload     []byte
	Priority    int
	Status      string
	WorkerID    string
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Result      []byte
	Error       string
	RetryCount  int
	Migrated    bool
}

// MigrationMetrics tracks migration statistics.
type MigrationMetrics struct {
	// Tasks migrated from legacy to new system.
	TasksMigrated atomic.Int64
	// Tasks written to both systems in dual-write mode.
	DualWriteCount atomic.Int64
	// Discrepancies found during verification.
	DiscrepanciesFound atomic.Int64
	// Errors encountered during migration.
	ErrorCount atomic.Int64
	// Rollbacks triggered.
	RollbackCount atomic.Int64
	// Current mode.
	CurrentMode atomic.Int32
	// Last error timestamp.
	LastErrorTime atomic.Int64
	// Error rate (errors per minute).
	ErrorsPerMinute atomic.Int64
}

// MigrationManager handles the transition between task queue systems.
type MigrationManager struct {
	config       *MigrationConfig
	legacyQueue  LegacyTaskQueue
	rabbitBroker MessageBroker
	kafkaBroker  MessageBroker
	metrics      *MigrationMetrics
	logger       *zap.Logger
	mu           sync.RWMutex
	errorWindow  []time.Time
	windowMu     sync.Mutex
}

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(cfg *MigrationConfig, logger *zap.Logger) *MigrationManager {
	if cfg == nil {
		cfg = DefaultMigrationConfig()
	}
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	m := &MigrationManager{
		config:      cfg,
		metrics:     &MigrationMetrics{},
		logger:      logger,
		errorWindow: make([]time.Time, 0),
	}
	m.metrics.CurrentMode.Store(int32(cfg.Mode))
	return m
}

// SetLegacyQueue sets the legacy PostgreSQL task queue.
func (m *MigrationManager) SetLegacyQueue(queue LegacyTaskQueue) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.legacyQueue = queue
}

// SetRabbitMQBroker sets the RabbitMQ broker.
func (m *MigrationManager) SetRabbitMQBroker(broker MessageBroker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rabbitBroker = broker
}

// SetKafkaBroker sets the Kafka broker.
func (m *MigrationManager) SetKafkaBroker(broker MessageBroker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kafkaBroker = broker
}

// Mode returns the current migration mode.
func (m *MigrationManager) Mode() MigrationMode {
	return MigrationMode(m.metrics.CurrentMode.Load())
}

// SetMode changes the migration mode.
func (m *MigrationManager) SetMode(mode MigrationMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldMode := MigrationMode(m.metrics.CurrentMode.Load())
	if oldMode == mode {
		return nil
	}

	// Validate mode transition
	if err := m.validateModeTransition(oldMode, mode); err != nil {
		return err
	}

	m.metrics.CurrentMode.Store(int32(mode))
	m.config.Mode = mode

	m.logger.Info("migration mode changed",
		zap.String("old_mode", oldMode.String()),
		zap.String("new_mode", mode.String()))

	return nil
}

// validateModeTransition checks if the mode transition is valid.
func (m *MigrationManager) validateModeTransition(from, to MigrationMode) error {
	// Allow any transition to rollback
	if to == ModeRollback {
		return nil
	}

	// Validate normal progression
	switch from {
	case ModeLegacy:
		if to != ModeDualWrite {
			return fmt.Errorf("can only transition from legacy to dual_write")
		}
	case ModeDualWrite:
		if to != ModeMessaging && to != ModeLegacy {
			return fmt.Errorf("can only transition from dual_write to messaging or legacy")
		}
	case ModeMessaging:
		if to != ModeDualWrite {
			return fmt.Errorf("can only transition from messaging to dual_write")
		}
	case ModeRollback:
		if to != ModeLegacy {
			return fmt.Errorf("can only transition from rollback to legacy")
		}
	}

	return nil
}

// EnqueueTask enqueues a task using the appropriate system based on mode.
func (m *MigrationManager) EnqueueTask(ctx context.Context, taskType string, payload []byte, priority int) (string, error) {
	mode := m.Mode()

	switch mode {
	case ModeLegacy, ModeRollback:
		return m.enqueueLegacy(ctx, taskType, payload, priority)
	case ModeDualWrite:
		return m.enqueueDualWrite(ctx, taskType, payload, priority)
	case ModeMessaging:
		return m.enqueueMessaging(ctx, taskType, payload, priority)
	default:
		return "", fmt.Errorf("unknown migration mode: %d", mode)
	}
}

// enqueueLegacy enqueues using the legacy PostgreSQL queue.
func (m *MigrationManager) enqueueLegacy(ctx context.Context, taskType string, payload []byte, priority int) (string, error) {
	m.mu.RLock()
	queue := m.legacyQueue
	m.mu.RUnlock()

	if queue == nil {
		return "", fmt.Errorf("legacy queue not configured")
	}

	return queue.Enqueue(ctx, taskType, payload, priority)
}

// enqueueMessaging enqueues using RabbitMQ.
func (m *MigrationManager) enqueueMessaging(ctx context.Context, taskType string, payload []byte, priority int) (string, error) {
	m.mu.RLock()
	broker := m.rabbitBroker
	m.mu.RUnlock()

	if broker == nil {
		return "", fmt.Errorf("RabbitMQ broker not configured")
	}

	msg := NewMessage(taskType, payload)
	msg.Priority = MessagePriority(priority)

	queue := m.taskTypeToQueue(taskType)
	if err := broker.Publish(ctx, queue, msg); err != nil {
		m.recordError()
		return "", err
	}

	return msg.ID, nil
}

// enqueueDualWrite writes to both systems.
func (m *MigrationManager) enqueueDualWrite(ctx context.Context, taskType string, payload []byte, priority int) (string, error) {
	m.mu.RLock()
	legacyQueue := m.legacyQueue
	rabbitBroker := m.rabbitBroker
	m.mu.RUnlock()

	if legacyQueue == nil {
		return "", fmt.Errorf("legacy queue not configured")
	}

	// Create context with timeout for dual-write
	dualCtx, cancel := context.WithTimeout(ctx, m.config.DualWriteTimeout)
	defer cancel()

	// Write to legacy system first (primary)
	taskID, err := legacyQueue.Enqueue(dualCtx, taskType, payload, priority)
	if err != nil {
		m.recordError()
		return "", fmt.Errorf("legacy enqueue failed: %w", err)
	}

	// Write to RabbitMQ (secondary) - log but don't fail on error
	if rabbitBroker != nil {
		msg := NewMessageWithID(taskID, taskType, payload)
		msg.Priority = MessagePriority(priority)

		queue := m.taskTypeToQueue(taskType)
		if err := rabbitBroker.Publish(dualCtx, queue, msg); err != nil {
			m.recordError()
			if m.config.LogDiscrepancies {
				m.logger.Warn("dual-write RabbitMQ publish failed",
					zap.String("task_id", taskID),
					zap.String("task_type", taskType),
					zap.Error(err))
			}
			m.metrics.DiscrepanciesFound.Add(1)
		}
	}

	m.metrics.DualWriteCount.Add(1)
	return taskID, nil
}

// taskTypeToQueue maps task types to queue names.
func (m *MigrationManager) taskTypeToQueue(taskType string) string {
	switch taskType {
	case "llm_request", "llm.request":
		return "helixagent.tasks.llm"
	case "debate", "debate.round":
		return "helixagent.tasks.debate"
	case "verification", "verify":
		return "helixagent.tasks.verification"
	case "notification", "notify":
		return "helixagent.tasks.notifications"
	default:
		return "helixagent.tasks.background"
	}
}

// MigratePendingTasks migrates all pending tasks from legacy to new system.
func (m *MigrationManager) MigratePendingTasks(ctx context.Context) error {
	m.mu.RLock()
	legacyQueue := m.legacyQueue
	rabbitBroker := m.rabbitBroker
	m.mu.RUnlock()

	if legacyQueue == nil {
		return fmt.Errorf("legacy queue not configured")
	}
	if rabbitBroker == nil {
		return fmt.Errorf("RabbitMQ broker not configured")
	}

	// Get all pending tasks
	tasks, err := legacyQueue.GetPendingTasks(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending tasks: %w", err)
	}

	m.logger.Info("starting task migration", zap.Int("task_count", len(tasks)))

	var migrated, failed int
	for _, task := range tasks {
		if task.Migrated {
			continue
		}

		// Create message from legacy task
		msg := NewMessageWithID(task.ID, task.Type, task.Payload)
		msg.Priority = MessagePriority(task.Priority)
		msg.RetryCount = task.RetryCount

		// Publish to RabbitMQ
		queue := m.taskTypeToQueue(task.Type)
		if err := rabbitBroker.Publish(ctx, queue, msg); err != nil {
			m.logger.Error("failed to migrate task",
				zap.String("task_id", task.ID),
				zap.Error(err))
			failed++
			continue
		}

		// Mark as migrated in legacy system
		if err := legacyQueue.MarkMigrated(ctx, task.ID); err != nil {
			m.logger.Error("failed to mark task as migrated",
				zap.String("task_id", task.ID),
				zap.Error(err))
		}

		migrated++
		m.metrics.TasksMigrated.Add(1)
	}

	m.logger.Info("task migration complete",
		zap.Int("migrated", migrated),
		zap.Int("failed", failed))

	return nil
}

// Rollback triggers an emergency rollback to legacy mode.
func (m *MigrationManager) Rollback() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldMode := MigrationMode(m.metrics.CurrentMode.Load())
	m.metrics.CurrentMode.Store(int32(ModeRollback))
	m.config.Mode = ModeRollback
	m.metrics.RollbackCount.Add(1)

	m.logger.Warn("emergency rollback triggered",
		zap.String("from_mode", oldMode.String()))

	return nil
}

// recordError records an error and checks rollback threshold.
func (m *MigrationManager) recordError() {
	m.metrics.ErrorCount.Add(1)
	m.metrics.LastErrorTime.Store(time.Now().UnixNano())

	if !m.config.AutoRollback {
		return
	}

	m.windowMu.Lock()
	defer m.windowMu.Unlock()

	now := time.Now()
	windowStart := now.Add(-time.Minute)

	// Clean old entries
	cleaned := make([]time.Time, 0)
	for _, t := range m.errorWindow {
		if t.After(windowStart) {
			cleaned = append(cleaned, t)
		}
	}
	cleaned = append(cleaned, now)
	m.errorWindow = cleaned

	m.metrics.ErrorsPerMinute.Store(int64(len(cleaned)))

	// Check threshold
	if len(cleaned) >= m.config.ErrorThreshold {
		m.logger.Warn("error threshold exceeded, triggering rollback",
			zap.Int("errors_per_minute", len(cleaned)),
			zap.Int("threshold", m.config.ErrorThreshold))
		// Don't call Rollback() directly to avoid deadlock, schedule it
		go func() {
			_ = m.Rollback()
		}()
	}
}

// GetMetrics returns migration metrics.
func (m *MigrationManager) GetMetrics() *MigrationMetrics {
	return m.metrics
}

// GetStatus returns the current migration status.
func (m *MigrationManager) GetStatus() *MigrationStatus {
	return &MigrationStatus{
		Mode:               MigrationMode(m.metrics.CurrentMode.Load()),
		TasksMigrated:      m.metrics.TasksMigrated.Load(),
		DualWriteCount:     m.metrics.DualWriteCount.Load(),
		DiscrepanciesFound: m.metrics.DiscrepanciesFound.Load(),
		ErrorCount:         m.metrics.ErrorCount.Load(),
		RollbackCount:      m.metrics.RollbackCount.Load(),
		ErrorsPerMinute:    m.metrics.ErrorsPerMinute.Load(),
		LastErrorTime:      time.Unix(0, m.metrics.LastErrorTime.Load()),
	}
}

// MigrationStatus represents the current migration status.
type MigrationStatus struct {
	Mode               MigrationMode `json:"mode"`
	TasksMigrated      int64         `json:"tasks_migrated"`
	DualWriteCount     int64         `json:"dual_write_count"`
	DiscrepanciesFound int64         `json:"discrepancies_found"`
	ErrorCount         int64         `json:"error_count"`
	RollbackCount      int64         `json:"rollback_count"`
	ErrorsPerMinute    int64         `json:"errors_per_minute"`
	LastErrorTime      time.Time     `json:"last_error_time"`
}

// MarshalJSON implements json.Marshaler for MigrationStatus.
func (s *MigrationStatus) MarshalJSON() ([]byte, error) {
	type Alias MigrationStatus
	return json.Marshal(&struct {
		*Alias
		Mode string `json:"mode"`
	}{
		Alias: (*Alias)(s),
		Mode:  s.Mode.String(),
	})
}

// ShouldUseMessaging determines if a request should use messaging based on traffic split.
func (m *MigrationManager) ShouldUseMessaging() bool {
	if m.Mode() != ModeDualWrite {
		return m.Mode() == ModeMessaging
	}

	// Use simple modulo for traffic split
	split := m.config.ConsumerTrafficSplit
	if split <= 0 {
		return false
	}
	if split >= 100 {
		return true
	}

	// Use current nanosecond modulo for randomness
	return time.Now().UnixNano()%100 < int64(split)
}
