package verifier

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"llm-verifier/database"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseBridge bridges LLMsVerifier SQLite and SuperAgent PostgreSQL
type DatabaseBridge struct {
	verifierDB   *database.Database
	superagentDB *pgxpool.Pool
	mu           sync.RWMutex
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
func NewDatabaseBridge(verifierDBPath string, pgConfig *PostgresConfig) (*DatabaseBridge, error) {
	// Initialize LLMsVerifier SQLite database
	verifierDB, err := database.New(verifierDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize verifier database: %w", err)
	}

	// Build PostgreSQL connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		pgConfig.Host, pgConfig.Port, pgConfig.User,
		pgConfig.Password, pgConfig.Database, pgConfig.SSLMode,
	)

	// Connect to SuperAgent PostgreSQL
	pgPool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		verifierDB.Close()
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	return &DatabaseBridge{
		verifierDB:   verifierDB,
		superagentDB: pgPool,
	}, nil
}

// NewDatabaseBridgeWithPool creates a bridge with an existing PostgreSQL pool
func NewDatabaseBridgeWithPool(verifierDBPath string, pgPool *pgxpool.Pool) (*DatabaseBridge, error) {
	verifierDB, err := database.New(verifierDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize verifier database: %w", err)
	}

	return &DatabaseBridge{
		verifierDB:   verifierDB,
		superagentDB: pgPool,
	}, nil
}

// Close closes both database connections
func (db *DatabaseBridge) Close() error {
	var errs []error

	if db.verifierDB != nil {
		if err := db.verifierDB.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if db.superagentDB != nil {
		db.superagentDB.Close()
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing databases: %v", errs)
	}
	return nil
}

// GetVerifierDB returns the LLMsVerifier database
func (db *DatabaseBridge) GetVerifierDB() *database.Database {
	return db.verifierDB
}

// GetSuperAgentDB returns the SuperAgent PostgreSQL pool
func (db *DatabaseBridge) GetSuperAgentDB() *pgxpool.Pool {
	return db.superagentDB
}

// SyncVerificationResults syncs verification results from SQLite to PostgreSQL
func (db *DatabaseBridge) SyncVerificationResults(ctx context.Context) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Get all verification results from LLMsVerifier
	results, err := db.verifierDB.GetAllVerificationResults()
	if err != nil {
		return fmt.Errorf("failed to get verification results: %w", err)
	}

	// Insert/update into PostgreSQL
	for _, result := range results {
		if err := db.upsertVerificationResult(ctx, result); err != nil {
			return fmt.Errorf("failed to upsert verification result: %w", err)
		}
	}

	return nil
}

// upsertVerificationResult inserts or updates a verification result in PostgreSQL
func (db *DatabaseBridge) upsertVerificationResult(ctx context.Context, result *database.VerificationResult) error {
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

	_, err := db.superagentDB.Exec(ctx, query,
		fmt.Sprintf("%d", result.ModelID),
		"", // provider_name would need to be fetched
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
		result.StartedAt,
	)

	return err
}

// SyncScores syncs verification scores from SQLite to PostgreSQL
func (db *DatabaseBridge) SyncScores(ctx context.Context) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Get all scores from LLMsVerifier
	scores, err := db.verifierDB.GetAllVerificationScores()
	if err != nil {
		return fmt.Errorf("failed to get verification scores: %w", err)
	}

	for _, score := range scores {
		if err := db.upsertScore(ctx, score); err != nil {
			return fmt.Errorf("failed to upsert score: %w", err)
		}
	}

	return nil
}

// upsertScore inserts or updates a score in PostgreSQL
func (db *DatabaseBridge) upsertScore(ctx context.Context, score *database.VerificationScore) error {
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

	_, err := db.superagentDB.Exec(ctx, query,
		fmt.Sprintf("%d", score.ModelID),
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

// SyncProviderHealth syncs provider health from SQLite to PostgreSQL
func (db *DatabaseBridge) SyncProviderHealth(ctx context.Context, healthData []*ProviderHealth) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	for _, health := range healthData {
		if err := db.upsertProviderHealth(ctx, health); err != nil {
			return fmt.Errorf("failed to upsert provider health: %w", err)
		}
	}

	return nil
}

// upsertProviderHealth inserts or updates provider health in PostgreSQL
func (db *DatabaseBridge) upsertProviderHealth(ctx context.Context, health *ProviderHealth) error {
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

	_, err := db.superagentDB.Exec(ctx, query,
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

// LogEvent logs a verification event to PostgreSQL
func (db *DatabaseBridge) LogEvent(ctx context.Context, eventType, severity, modelID, providerID, message string, metadata map[string]interface{}) error {
	query := `
		INSERT INTO llmsverifier_events (
			event_type, severity, model_id, provider_id, message, metadata
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := db.superagentDB.Exec(ctx, query,
		eventType, severity, modelID, providerID, message, metadata,
	)

	return err
}

// GetVerificationResultsFromPG retrieves verification results from PostgreSQL
func (db *DatabaseBridge) GetVerificationResultsFromPG(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT model_id, provider_name, verification_type, status,
			overall_score, code_capability_score, supports_code_generation,
			supports_streaming, avg_latency_ms, verified_at
		FROM llmsverifier_results
		ORDER BY verified_at DESC
		LIMIT $1
	`

	rows, err := db.superagentDB.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var modelID, providerName, verificationType, status string
		var overallScore, codeCapabilityScore float64
		var supportsCodeGen, supportsStreaming bool
		var avgLatency int
		var verifiedAt time.Time

		if err := rows.Scan(
			&modelID, &providerName, &verificationType, &status,
			&overallScore, &codeCapabilityScore, &supportsCodeGen,
			&supportsStreaming, &avgLatency, &verifiedAt,
		); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"model_id":                modelID,
			"provider_name":           providerName,
			"verification_type":       verificationType,
			"status":                  status,
			"overall_score":           overallScore,
			"code_capability_score":   codeCapabilityScore,
			"supports_code_generation": supportsCodeGen,
			"supports_streaming":      supportsStreaming,
			"avg_latency_ms":          avgLatency,
			"verified_at":             verifiedAt,
		})
	}

	return results, nil
}

// GetScoresFromPG retrieves scores from PostgreSQL
func (db *DatabaseBridge) GetScoresFromPG(ctx context.Context, minScore float64, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT model_id, overall_score, speed_score, efficiency_score,
			cost_score, capability_score, recency_score, score_suffix,
			data_source, calculated_at
		FROM llmsverifier_scores
		WHERE overall_score >= $1
		ORDER BY overall_score DESC
		LIMIT $2
	`

	rows, err := db.superagentDB.Query(ctx, query, minScore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var modelID, scoreSuffix, dataSource string
		var overallScore, speedScore, efficiencyScore, costScore, capabilityScore, recencyScore float64
		var calculatedAt time.Time

		if err := rows.Scan(
			&modelID, &overallScore, &speedScore, &efficiencyScore,
			&costScore, &capabilityScore, &recencyScore, &scoreSuffix,
			&dataSource, &calculatedAt,
		); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"model_id":         modelID,
			"overall_score":    overallScore,
			"speed_score":      speedScore,
			"efficiency_score": efficiencyScore,
			"cost_score":       costScore,
			"capability_score": capabilityScore,
			"recency_score":    recencyScore,
			"score_suffix":     scoreSuffix,
			"data_source":      dataSource,
			"calculated_at":    calculatedAt,
		})
	}

	return results, nil
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
		if _, err := db.superagentDB.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}
