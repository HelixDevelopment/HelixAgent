package verifier

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// VerificationResult represents a verification result
type VerificationResult struct {
	ID                     int64     `json:"id"`
	ModelID                string    `json:"model_id"`
	ProviderName           string    `json:"provider_name"`
	VerificationType       string    `json:"verification_type"`
	Status                 string    `json:"status"`
	OverallScore           float64   `json:"overall_score"`
	CodeCapabilityScore    float64   `json:"code_capability_score"`
	ResponsivenessScore    float64   `json:"responsiveness_score"`
	ReliabilityScore       float64   `json:"reliability_score"`
	FeatureRichnessScore   float64   `json:"feature_richness_score"`
	ValuePropositionScore  float64   `json:"value_proposition_score"`
	SupportsCodeGeneration bool      `json:"supports_code_generation"`
	SupportsCodeCompletion bool      `json:"supports_code_completion"`
	SupportsCodeReview     bool      `json:"supports_code_review"`
	SupportsStreaming      bool      `json:"supports_streaming"`
	SupportsReasoning      bool      `json:"supports_reasoning"`
	AvgLatencyMs           int       `json:"avg_latency_ms"`
	P95LatencyMs           int       `json:"p95_latency_ms"`
	ThroughputRPS          float64   `json:"throughput_rps"`
	VerifiedAt             time.Time `json:"verified_at"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// VerificationScore represents a model score
type VerificationScore struct {
	ID              int64     `json:"id"`
	ModelID         string    `json:"model_id"`
	OverallScore    float64   `json:"overall_score"`
	SpeedScore      float64   `json:"speed_score"`
	EfficiencyScore float64   `json:"efficiency_score"`
	CostScore       float64   `json:"cost_score"`
	CapabilityScore float64   `json:"capability_score"`
	RecencyScore    float64   `json:"recency_score"`
	ScoreSuffix     string    `json:"score_suffix"`
	DataSource      string    `json:"data_source"`
	CalculatedAt    time.Time `json:"calculated_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// DatabaseBridge provides database operations for verifier
type DatabaseBridge struct {
	pool *pgxpool.Pool
	mu   sync.RWMutex
}

// PostgresConfig represents PostgreSQL configuration
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// NewDatabaseBridge creates a new database bridge
func NewDatabaseBridge(pgConfig *PostgresConfig) (*DatabaseBridge, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		pgConfig.Host, pgConfig.Port, pgConfig.User,
		pgConfig.Password, pgConfig.Database, pgConfig.SSLMode,
	)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	return &DatabaseBridge{
		pool: pool,
	}, nil
}

// NewDatabaseBridgeWithPool creates a bridge with an existing pool
func NewDatabaseBridgeWithPool(pool *pgxpool.Pool) *DatabaseBridge {
	return &DatabaseBridge{
		pool: pool,
	}
}

// Close closes the database connection
func (db *DatabaseBridge) Close() error {
	if db.pool != nil {
		db.pool.Close()
	}
	return nil
}

// GetPool returns the underlying connection pool
func (db *DatabaseBridge) GetPool() *pgxpool.Pool {
	return db.pool
}

// SaveVerificationResult saves a verification result
func (db *DatabaseBridge) SaveVerificationResult(ctx context.Context, result *VerificationResult) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
		INSERT INTO llmsverifier_results (
			model_id, provider_name, verification_type, status,
			overall_score, code_capability_score, responsiveness_score,
			reliability_score, feature_richness_score, value_proposition_score,
			supports_code_generation, supports_code_completion, supports_code_review,
			supports_streaming, supports_reasoning, avg_latency_ms, p95_latency_ms,
			throughput_rps, verified_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (model_id, verification_type) DO UPDATE SET
			status = EXCLUDED.status,
			overall_score = EXCLUDED.overall_score,
			code_capability_score = EXCLUDED.code_capability_score,
			responsiveness_score = EXCLUDED.responsiveness_score,
			reliability_score = EXCLUDED.reliability_score,
			feature_richness_score = EXCLUDED.feature_richness_score,
			value_proposition_score = EXCLUDED.value_proposition_score,
			supports_code_generation = EXCLUDED.supports_code_generation,
			supports_code_completion = EXCLUDED.supports_code_completion,
			supports_code_review = EXCLUDED.supports_code_review,
			supports_streaming = EXCLUDED.supports_streaming,
			supports_reasoning = EXCLUDED.supports_reasoning,
			avg_latency_ms = EXCLUDED.avg_latency_ms,
			p95_latency_ms = EXCLUDED.p95_latency_ms,
			throughput_rps = EXCLUDED.throughput_rps,
			verified_at = EXCLUDED.verified_at,
			updated_at = NOW()
	`

	_, err := db.pool.Exec(ctx, query,
		result.ModelID,
		result.ProviderName,
		result.VerificationType,
		result.Status,
		result.OverallScore,
		result.CodeCapabilityScore,
		result.ResponsivenessScore,
		result.ReliabilityScore,
		result.FeatureRichnessScore,
		result.ValuePropositionScore,
		result.SupportsCodeGeneration,
		result.SupportsCodeCompletion,
		result.SupportsCodeReview,
		result.SupportsStreaming,
		result.SupportsReasoning,
		result.AvgLatencyMs,
		result.P95LatencyMs,
		result.ThroughputRPS,
		result.VerifiedAt,
	)

	return err
}

// GetVerificationResults retrieves verification results
func (db *DatabaseBridge) GetVerificationResults(ctx context.Context, limit int) ([]*VerificationResult, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT id, model_id, provider_name, verification_type, status,
			overall_score, code_capability_score, responsiveness_score,
			reliability_score, feature_richness_score, value_proposition_score,
			supports_code_generation, supports_code_completion, supports_code_review,
			supports_streaming, supports_reasoning, avg_latency_ms, p95_latency_ms,
			throughput_rps, verified_at, created_at, updated_at
		FROM llmsverifier_results
		ORDER BY verified_at DESC
		LIMIT $1
	`

	rows, err := db.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*VerificationResult
	for rows.Next() {
		r := &VerificationResult{}
		if err := rows.Scan(
			&r.ID, &r.ModelID, &r.ProviderName, &r.VerificationType, &r.Status,
			&r.OverallScore, &r.CodeCapabilityScore, &r.ResponsivenessScore,
			&r.ReliabilityScore, &r.FeatureRichnessScore, &r.ValuePropositionScore,
			&r.SupportsCodeGeneration, &r.SupportsCodeCompletion, &r.SupportsCodeReview,
			&r.SupportsStreaming, &r.SupportsReasoning, &r.AvgLatencyMs, &r.P95LatencyMs,
			&r.ThroughputRPS, &r.VerifiedAt, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return results, nil
}

// SaveScore saves a verification score
func (db *DatabaseBridge) SaveScore(ctx context.Context, score *VerificationScore) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
		INSERT INTO llmsverifier_scores (
			model_id, overall_score, speed_score, efficiency_score,
			cost_score, capability_score, recency_score, score_suffix,
			data_source, calculated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (model_id) DO UPDATE SET
			overall_score = EXCLUDED.overall_score,
			speed_score = EXCLUDED.speed_score,
			efficiency_score = EXCLUDED.efficiency_score,
			cost_score = EXCLUDED.cost_score,
			capability_score = EXCLUDED.capability_score,
			recency_score = EXCLUDED.recency_score,
			score_suffix = EXCLUDED.score_suffix,
			data_source = EXCLUDED.data_source,
			calculated_at = EXCLUDED.calculated_at
	`

	_, err := db.pool.Exec(ctx, query,
		score.ModelID,
		score.OverallScore,
		score.SpeedScore,
		score.EfficiencyScore,
		score.CostScore,
		score.CapabilityScore,
		score.RecencyScore,
		score.ScoreSuffix,
		score.DataSource,
		score.CalculatedAt,
	)

	return err
}

// GetScores retrieves scores with minimum threshold
func (db *DatabaseBridge) GetScores(ctx context.Context, minScore float64, limit int) ([]*VerificationScore, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT id, model_id, overall_score, speed_score, efficiency_score,
			cost_score, capability_score, recency_score, score_suffix,
			data_source, calculated_at, created_at
		FROM llmsverifier_scores
		WHERE overall_score >= $1
		ORDER BY overall_score DESC
		LIMIT $2
	`

	rows, err := db.pool.Query(ctx, query, minScore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []*VerificationScore
	for rows.Next() {
		s := &VerificationScore{}
		if err := rows.Scan(
			&s.ID, &s.ModelID, &s.OverallScore, &s.SpeedScore, &s.EfficiencyScore,
			&s.CostScore, &s.CapabilityScore, &s.RecencyScore, &s.ScoreSuffix,
			&s.DataSource, &s.CalculatedAt, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		scores = append(scores, s)
	}

	return scores, nil
}

// GetScoreByModelID retrieves a score by model ID
func (db *DatabaseBridge) GetScoreByModelID(ctx context.Context, modelID string) (*VerificationScore, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT id, model_id, overall_score, speed_score, efficiency_score,
			cost_score, capability_score, recency_score, score_suffix,
			data_source, calculated_at, created_at
		FROM llmsverifier_scores
		WHERE model_id = $1
	`

	s := &VerificationScore{}
	err := db.pool.QueryRow(ctx, query, modelID).Scan(
		&s.ID, &s.ModelID, &s.OverallScore, &s.SpeedScore, &s.EfficiencyScore,
		&s.CostScore, &s.CapabilityScore, &s.RecencyScore, &s.ScoreSuffix,
		&s.DataSource, &s.CalculatedAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// SaveProviderHealth saves provider health data
func (db *DatabaseBridge) SaveProviderHealth(ctx context.Context, health *ProviderHealth) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
		INSERT INTO llmsverifier_provider_health (
			provider_id, provider_name, status, circuit_breaker_state,
			failure_count, success_count, last_success_at, last_failure_at,
			avg_response_time_ms, uptime_percentage, checked_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (provider_id) DO UPDATE SET
			status = EXCLUDED.status,
			circuit_breaker_state = EXCLUDED.circuit_breaker_state,
			failure_count = EXCLUDED.failure_count,
			success_count = EXCLUDED.success_count,
			last_success_at = EXCLUDED.last_success_at,
			last_failure_at = EXCLUDED.last_failure_at,
			avg_response_time_ms = EXCLUDED.avg_response_time_ms,
			uptime_percentage = EXCLUDED.uptime_percentage,
			checked_at = EXCLUDED.checked_at,
			updated_at = NOW()
	`

	status := "unhealthy"
	if health.Healthy {
		status = "healthy"
	}

	var lastSuccessAt, lastFailureAt sql.NullTime
	if !health.LastSuccessAt.IsZero() {
		lastSuccessAt = sql.NullTime{Time: health.LastSuccessAt, Valid: true}
	}
	if !health.LastFailureAt.IsZero() {
		lastFailureAt = sql.NullTime{Time: health.LastFailureAt, Valid: true}
	}

	_, err := db.pool.Exec(ctx, query,
		health.ProviderID,
		health.ProviderName,
		status,
		health.CircuitState,
		health.FailureCount,
		health.SuccessCount,
		lastSuccessAt,
		lastFailureAt,
		health.AvgResponseMs,
		health.UptimePercent,
		health.LastCheckedAt,
	)

	return err
}

// LogEvent logs a verification event
func (db *DatabaseBridge) LogEvent(ctx context.Context, eventType, severity, modelID, providerID, message string, metadata map[string]interface{}) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
		INSERT INTO llmsverifier_events (
			event_type, severity, model_id, provider_id, message, metadata
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := db.pool.Exec(ctx, query,
		eventType, severity, modelID, providerID, message, metadata,
	)

	return err
}

// RunMigrations runs database migrations for verifier tables
func (db *DatabaseBridge) RunMigrations(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS llmsverifier_results (
			id BIGSERIAL PRIMARY KEY,
			model_id VARCHAR(255) NOT NULL,
			provider_name VARCHAR(100) NOT NULL DEFAULT '',
			verification_type VARCHAR(50) NOT NULL,
			status VARCHAR(50) NOT NULL,
			overall_score DECIMAL(5,2) DEFAULT 0,
			code_capability_score DECIMAL(5,2) DEFAULT 0,
			responsiveness_score DECIMAL(5,2) DEFAULT 0,
			reliability_score DECIMAL(5,2) DEFAULT 0,
			feature_richness_score DECIMAL(5,2) DEFAULT 0,
			value_proposition_score DECIMAL(5,2) DEFAULT 0,
			supports_code_generation BOOLEAN DEFAULT FALSE,
			supports_code_completion BOOLEAN DEFAULT FALSE,
			supports_code_review BOOLEAN DEFAULT FALSE,
			supports_streaming BOOLEAN DEFAULT FALSE,
			supports_reasoning BOOLEAN DEFAULT FALSE,
			avg_latency_ms INTEGER DEFAULT 0,
			p95_latency_ms INTEGER DEFAULT 0,
			throughput_rps DECIMAL(10,2) DEFAULT 0,
			verified_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(model_id, verification_type)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_llmsverifier_results_model_id ON llmsverifier_results(model_id)`,
		`CREATE INDEX IF NOT EXISTS idx_llmsverifier_results_provider ON llmsverifier_results(provider_name)`,
		`CREATE INDEX IF NOT EXISTS idx_llmsverifier_results_score ON llmsverifier_results(overall_score DESC)`,

		`CREATE TABLE IF NOT EXISTS llmsverifier_scores (
			id BIGSERIAL PRIMARY KEY,
			model_id VARCHAR(255) NOT NULL UNIQUE,
			overall_score DECIMAL(5,2) NOT NULL DEFAULT 0,
			speed_score DECIMAL(5,2) DEFAULT 0,
			efficiency_score DECIMAL(5,2) DEFAULT 0,
			cost_score DECIMAL(5,2) DEFAULT 0,
			capability_score DECIMAL(5,2) DEFAULT 0,
			recency_score DECIMAL(5,2) DEFAULT 0,
			score_suffix VARCHAR(20) DEFAULT '',
			data_source VARCHAR(50) DEFAULT '',
			calculated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_llmsverifier_scores_model_id ON llmsverifier_scores(model_id)`,
		`CREATE INDEX IF NOT EXISTS idx_llmsverifier_scores_overall ON llmsverifier_scores(overall_score DESC)`,

		`CREATE TABLE IF NOT EXISTS llmsverifier_provider_health (
			id BIGSERIAL PRIMARY KEY,
			provider_id VARCHAR(100) NOT NULL UNIQUE,
			provider_name VARCHAR(100) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'unknown',
			circuit_breaker_state VARCHAR(50) DEFAULT 'closed',
			failure_count INTEGER DEFAULT 0,
			success_count INTEGER DEFAULT 0,
			last_success_at TIMESTAMP WITH TIME ZONE,
			last_failure_at TIMESTAMP WITH TIME ZONE,
			avg_response_time_ms INTEGER DEFAULT 0,
			uptime_percentage DECIMAL(5,2) DEFAULT 0,
			checked_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_llmsverifier_provider_health_provider ON llmsverifier_provider_health(provider_id)`,
		`CREATE INDEX IF NOT EXISTS idx_llmsverifier_provider_health_status ON llmsverifier_provider_health(status)`,

		`CREATE TABLE IF NOT EXISTS llmsverifier_events (
			id BIGSERIAL PRIMARY KEY,
			event_type VARCHAR(100) NOT NULL,
			severity VARCHAR(20) NOT NULL DEFAULT 'info',
			model_id VARCHAR(255),
			provider_id VARCHAR(100),
			message TEXT,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_llmsverifier_events_type ON llmsverifier_events(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_llmsverifier_events_created ON llmsverifier_events(created_at DESC)`,
	}

	for _, migration := range migrations {
		if _, err := db.pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// GetTopModels returns the top scoring models
func (db *DatabaseBridge) GetTopModels(ctx context.Context, limit int) ([]*VerificationScore, error) {
	return db.GetScores(ctx, 0, limit)
}

// GetVerifiedModelsCount returns count of verified models
func (db *DatabaseBridge) GetVerifiedModelsCount(ctx context.Context) (int, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var count int
	err := db.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT model_id) FROM llmsverifier_results WHERE status = 'verified'`,
	).Scan(&count)

	return count, err
}

// GetProviderHealthStats returns health statistics by provider
func (db *DatabaseBridge) GetProviderHealthStats(ctx context.Context) (map[string]map[string]interface{}, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT provider_name,
			COUNT(*) as total_models,
			AVG(overall_score) as avg_score,
			SUM(CASE WHEN status = 'verified' THEN 1 ELSE 0 END) as verified_count
		FROM llmsverifier_results
		GROUP BY provider_name
	`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]map[string]interface{})
	for rows.Next() {
		var providerName string
		var totalModels int
		var avgScore float64
		var verifiedCount int

		if err := rows.Scan(&providerName, &totalModels, &avgScore, &verifiedCount); err != nil {
			return nil, err
		}

		stats[providerName] = map[string]interface{}{
			"total_models":   totalModels,
			"avg_score":      avgScore,
			"verified_count": verifiedCount,
		}
	}

	return stats, nil
}
