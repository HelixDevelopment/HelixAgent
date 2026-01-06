package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// LogRetentionPolicy defines how long logs should be kept
type LogRetentionPolicy struct {
	RetentionDays int           // Number of days to keep logs (0 = no expiration)
	RetentionTime time.Duration // Alternative: specify as duration
	NoExpiration  bool          // If true, logs are never cleaned up
}

// DefaultRetentionPolicy returns the default 5-day retention policy
func DefaultRetentionPolicy() LogRetentionPolicy {
	return LogRetentionPolicy{
		RetentionDays: 5,
		NoExpiration:  false,
	}
}

// NoExpirationPolicy returns a policy that never expires logs
func NoExpirationPolicy() LogRetentionPolicy {
	return LogRetentionPolicy{
		NoExpiration: true,
	}
}

// DebateLogEntry represents a log entry for AI debate activities
type DebateLogEntry struct {
	ID                    int64      `json:"id" db:"id"`
	DebateID              string     `json:"debate_id" db:"debate_id"`
	SessionID             string     `json:"session_id" db:"session_id"`
	ParticipantID         string     `json:"participant_id" db:"participant_id"`
	ParticipantIdentifier string     `json:"participant_identifier" db:"participant_identifier"` // e.g., "DeepSeek-1"
	ParticipantName       string     `json:"participant_name" db:"participant_name"`
	Role                  string     `json:"role" db:"role"`
	Provider              string     `json:"provider" db:"provider"`
	Model                 string     `json:"model" db:"model"`
	Round                 int        `json:"round" db:"round"`
	Action                string     `json:"action" db:"action"` // "start", "complete", "error"
	ResponseTimeMs        int64      `json:"response_time_ms" db:"response_time_ms"`
	QualityScore          float64    `json:"quality_score" db:"quality_score"`
	TokensUsed            int        `json:"tokens_used" db:"tokens_used"`
	ContentLength         int        `json:"content_length" db:"content_length"`
	ErrorMessage          string     `json:"error_message,omitempty" db:"error_message"`
	Metadata              string     `json:"metadata,omitempty" db:"metadata"` // JSON metadata
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt             *time.Time `json:"expires_at,omitempty" db:"expires_at"` // nil = no expiration
}

// DebateLogRepository manages debate log storage with retention
type DebateLogRepository struct {
	pool            *pgxpool.Pool
	log             *logrus.Logger
	retentionPolicy LogRetentionPolicy
}

// NewDebateLogRepository creates a new debate log repository
func NewDebateLogRepository(pool *pgxpool.Pool, log *logrus.Logger, policy LogRetentionPolicy) *DebateLogRepository {
	if log == nil {
		log = logrus.New()
	}
	return &DebateLogRepository{
		pool:            pool,
		log:             log,
		retentionPolicy: policy,
	}
}

// CreateTable creates the debate_logs table if it doesn't exist
func (r *DebateLogRepository) CreateTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS debate_logs (
			id SERIAL PRIMARY KEY,
			debate_id VARCHAR(255) NOT NULL,
			session_id VARCHAR(255),
			participant_id VARCHAR(255) NOT NULL,
			participant_identifier VARCHAR(255) NOT NULL,
			participant_name VARCHAR(255),
			role VARCHAR(50),
			provider VARCHAR(100) NOT NULL,
			model VARCHAR(100),
			round INT DEFAULT 1,
			action VARCHAR(50) NOT NULL,
			response_time_ms BIGINT DEFAULT 0,
			quality_score DECIMAL(5,4) DEFAULT 0,
			tokens_used INT DEFAULT 0,
			content_length INT DEFAULT 0,
			error_message TEXT,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			expires_at TIMESTAMP WITH TIME ZONE
		);

		CREATE INDEX IF NOT EXISTS idx_debate_logs_debate_id ON debate_logs(debate_id);
		CREATE INDEX IF NOT EXISTS idx_debate_logs_participant_identifier ON debate_logs(participant_identifier);
		CREATE INDEX IF NOT EXISTS idx_debate_logs_created_at ON debate_logs(created_at);
		CREATE INDEX IF NOT EXISTS idx_debate_logs_expires_at ON debate_logs(expires_at);
	`

	_, err := r.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create debate_logs table: %w", err)
	}

	r.log.Info("Debate logs table created/verified")
	return nil
}

// Insert adds a new log entry
func (r *DebateLogRepository) Insert(ctx context.Context, entry *DebateLogEntry) error {
	// Calculate expiration time based on policy
	entry.CreatedAt = time.Now()
	entry.ExpiresAt = r.calculateExpiration()

	query := `
		INSERT INTO debate_logs (
			debate_id, session_id, participant_id, participant_identifier,
			participant_name, role, provider, model, round, action,
			response_time_ms, quality_score, tokens_used, content_length,
			error_message, metadata, created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id
	`

	err := r.pool.QueryRow(ctx, query,
		entry.DebateID, entry.SessionID, entry.ParticipantID, entry.ParticipantIdentifier,
		entry.ParticipantName, entry.Role, entry.Provider, entry.Model, entry.Round, entry.Action,
		entry.ResponseTimeMs, entry.QualityScore, entry.TokensUsed, entry.ContentLength,
		entry.ErrorMessage, entry.Metadata, entry.CreatedAt, entry.ExpiresAt,
	).Scan(&entry.ID)

	if err != nil {
		return fmt.Errorf("failed to insert debate log: %w", err)
	}

	r.log.WithFields(logrus.Fields{
		"id":          entry.ID,
		"debate_id":   entry.DebateID,
		"participant": entry.ParticipantIdentifier,
		"action":      entry.Action,
		"expires_at":  entry.ExpiresAt,
	}).Debug("Debate log entry inserted")

	return nil
}

// calculateExpiration returns the expiration time based on the retention policy
func (r *DebateLogRepository) calculateExpiration() *time.Time {
	if r.retentionPolicy.NoExpiration {
		return nil // Never expires
	}

	var expiration time.Time
	if r.retentionPolicy.RetentionTime > 0 {
		expiration = time.Now().Add(r.retentionPolicy.RetentionTime)
	} else if r.retentionPolicy.RetentionDays > 0 {
		expiration = time.Now().AddDate(0, 0, r.retentionPolicy.RetentionDays)
	} else {
		// Default to 5 days
		expiration = time.Now().AddDate(0, 0, 5)
	}
	return &expiration
}

// GetByDebateID retrieves all logs for a specific debate
func (r *DebateLogRepository) GetByDebateID(ctx context.Context, debateID string) ([]DebateLogEntry, error) {
	query := `
		SELECT id, debate_id, session_id, participant_id, participant_identifier,
			   participant_name, role, provider, model, round, action,
			   response_time_ms, quality_score, tokens_used, content_length,
			   error_message, metadata, created_at, expires_at
		FROM debate_logs
		WHERE debate_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, debateID)
	if err != nil {
		return nil, fmt.Errorf("failed to query debate logs: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetByParticipantIdentifier retrieves logs for a specific participant identifier (e.g., "DeepSeek-1")
func (r *DebateLogRepository) GetByParticipantIdentifier(ctx context.Context, identifier string) ([]DebateLogEntry, error) {
	query := `
		SELECT id, debate_id, session_id, participant_id, participant_identifier,
			   participant_name, role, provider, model, round, action,
			   response_time_ms, quality_score, tokens_used, content_length,
			   error_message, metadata, created_at, expires_at
		FROM debate_logs
		WHERE participant_identifier = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to query debate logs by participant: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetExpiredLogs retrieves logs that have expired
func (r *DebateLogRepository) GetExpiredLogs(ctx context.Context) ([]DebateLogEntry, error) {
	query := `
		SELECT id, debate_id, session_id, participant_id, participant_identifier,
			   participant_name, role, provider, model, round, action,
			   response_time_ms, quality_score, tokens_used, content_length,
			   error_message, metadata, created_at, expires_at
		FROM debate_logs
		WHERE expires_at IS NOT NULL AND expires_at < NOW()
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query expired debate logs: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// CleanupExpiredLogs removes logs that have passed their expiration date
func (r *DebateLogRepository) CleanupExpiredLogs(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM debate_logs
		WHERE expires_at IS NOT NULL AND expires_at < NOW()
	`

	result, err := r.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired debate logs: %w", err)
	}

	deleted := result.RowsAffected()

	if deleted > 0 {
		r.log.WithFields(logrus.Fields{
			"deleted_count": deleted,
		}).Info("Cleaned up expired debate logs")
	}

	return deleted, nil
}

// GetLogsOlderThan retrieves logs older than a specific duration
func (r *DebateLogRepository) GetLogsOlderThan(ctx context.Context, duration time.Duration) ([]DebateLogEntry, error) {
	cutoff := time.Now().Add(-duration)

	query := `
		SELECT id, debate_id, session_id, participant_id, participant_identifier,
			   participant_name, role, provider, model, round, action,
			   response_time_ms, quality_score, tokens_used, content_length,
			   error_message, metadata, created_at, expires_at
		FROM debate_logs
		WHERE created_at < $1
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to query old debate logs: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// DeleteLogsOlderThan removes logs older than a specific duration (manual cleanup)
func (r *DebateLogRepository) DeleteLogsOlderThan(ctx context.Context, duration time.Duration) (int64, error) {
	cutoff := time.Now().Add(-duration)

	query := `
		DELETE FROM debate_logs
		WHERE created_at < $1
	`

	result, err := r.pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old debate logs: %w", err)
	}

	deleted := result.RowsAffected()

	r.log.WithFields(logrus.Fields{
		"deleted_count": deleted,
		"cutoff":        cutoff,
	}).Info("Deleted old debate logs")

	return deleted, nil
}

// SetRetentionPolicy updates the retention policy
func (r *DebateLogRepository) SetRetentionPolicy(policy LogRetentionPolicy) {
	r.retentionPolicy = policy
	r.log.WithFields(logrus.Fields{
		"retention_days": policy.RetentionDays,
		"no_expiration":  policy.NoExpiration,
	}).Info("Debate log retention policy updated")
}

// GetRetentionPolicy returns the current retention policy
func (r *DebateLogRepository) GetRetentionPolicy() LogRetentionPolicy {
	return r.retentionPolicy
}

// GetLogCount returns the total number of logs
func (r *DebateLogRepository) GetLogCount(ctx context.Context) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM debate_logs`
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count debate logs: %w", err)
	}
	return count, nil
}

// GetLogStats returns statistics about the logs
func (r *DebateLogRepository) GetLogStats(ctx context.Context) (*LogStats, error) {
	query := `
		SELECT
			COUNT(*) as total_logs,
			COUNT(CASE WHEN expires_at IS NULL THEN 1 END) as permanent_logs,
			COUNT(CASE WHEN expires_at IS NOT NULL AND expires_at < NOW() THEN 1 END) as expired_logs,
			COUNT(DISTINCT debate_id) as unique_debates,
			COUNT(DISTINCT participant_identifier) as unique_participants,
			MIN(created_at) as oldest_log,
			MAX(created_at) as newest_log
		FROM debate_logs
	`

	var stats LogStats
	var oldestLog, newestLog *time.Time

	err := r.pool.QueryRow(ctx, query).Scan(
		&stats.TotalLogs,
		&stats.PermanentLogs,
		&stats.ExpiredLogs,
		&stats.UniqueDebates,
		&stats.UniqueParticipants,
		&oldestLog,
		&newestLog,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get log stats: %w", err)
	}

	stats.OldestLog = oldestLog
	stats.NewestLog = newestLog

	return &stats, nil
}

// LogStats holds statistics about debate logs
type LogStats struct {
	TotalLogs          int64      `json:"total_logs"`
	PermanentLogs      int64      `json:"permanent_logs"`
	ExpiredLogs        int64      `json:"expired_logs"`
	UniqueDebates      int64      `json:"unique_debates"`
	UniqueParticipants int64      `json:"unique_participants"`
	OldestLog          *time.Time `json:"oldest_log,omitempty"`
	NewestLog          *time.Time `json:"newest_log,omitempty"`
}

// scanRows scans database rows into DebateLogEntry slice
func (r *DebateLogRepository) scanRows(rows pgx.Rows) ([]DebateLogEntry, error) {
	var entries []DebateLogEntry

	for rows.Next() {
		var entry DebateLogEntry
		var errorMsg, metadata *string
		var expiresAt *time.Time

		err := rows.Scan(
			&entry.ID, &entry.DebateID, &entry.SessionID, &entry.ParticipantID,
			&entry.ParticipantIdentifier, &entry.ParticipantName, &entry.Role,
			&entry.Provider, &entry.Model, &entry.Round, &entry.Action,
			&entry.ResponseTimeMs, &entry.QualityScore, &entry.TokensUsed,
			&entry.ContentLength, &errorMsg, &metadata, &entry.CreatedAt, &expiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan debate log row: %w", err)
		}

		if errorMsg != nil {
			entry.ErrorMessage = *errorMsg
		}
		if metadata != nil {
			entry.Metadata = *metadata
		}
		if expiresAt != nil {
			entry.ExpiresAt = expiresAt
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating debate log rows: %w", err)
	}

	return entries, nil
}

// StartCleanupWorker starts a background worker that periodically cleans up expired logs
func (r *DebateLogRepository) StartCleanupWorker(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		r.log.WithFields(logrus.Fields{
			"interval": interval,
		}).Info("Starting debate log cleanup worker")

		for {
			select {
			case <-ctx.Done():
				r.log.Info("Stopping debate log cleanup worker")
				return
			case <-ticker.C:
				deleted, err := r.CleanupExpiredLogs(ctx)
				if err != nil {
					r.log.WithError(err).Error("Failed to cleanup expired debate logs")
				} else if deleted > 0 {
					r.log.WithFields(logrus.Fields{
						"deleted_count": deleted,
					}).Info("Debate log cleanup completed")
				}
			}
		}
	}()
}
