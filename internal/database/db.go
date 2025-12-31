package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/superagent/superagent/internal/config"
)

// Row interface for row scanning
type Row interface {
	Scan(dest ...any) error
}

// DB interface for database operations
type DB interface {
	Ping() error
	Exec(query string, args ...any) error
	Query(query string, args ...any) ([]any, error)
	QueryRow(query string, args ...any) Row
	Close() error
	HealthCheck() error
}

// PostgresDB implements DB using PostgreSQL with pgxpool
type PostgresDB struct {
	pool *pgxpool.Pool
}

// pgxRow wraps pgx.Row to implement the Row interface
type pgxRow struct {
	row pgx.Row
}

// Scan implements the Row interface
func (r *pgxRow) Scan(dest ...any) error {
	return r.row.Scan(dest...)
}

func NewPostgresDB(cfg *config.Config) (*PostgresDB, error) {
	// Use config values if provided, otherwise fall back to environment variables with defaults
	dbHost := cfg.Database.Host
	if dbHost == "" {
		dbHost = getEnv("DB_HOST", "localhost")
	}

	dbPort := cfg.Database.Port
	if dbPort == "" {
		dbPort = getEnv("DB_PORT", "5432")
	}

	dbUser := cfg.Database.User
	if dbUser == "" {
		dbUser = getEnv("DB_USER", "superagent")
	}

	dbPassword := cfg.Database.Password
	if dbPassword == "" {
		dbPassword = getEnv("DB_PASSWORD", "secret")
	}

	dbName := cfg.Database.Name
	if dbName == "" {
		dbName = getEnv("DB_NAME", "superagent_db")
	}

	sslMode := cfg.Database.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbUser, dbPassword, dbHost, dbPort, dbName, sslMode)

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		log.Printf("Warning: Database connection test failed: %v", err)
	}

	log.Printf("Connected to PostgreSQL database: %s", dbName)
	return &PostgresDB{pool: pool}, nil
}

func (p *PostgresDB) Ping() error {
	return p.pool.Ping(context.Background())
}

func (p *PostgresDB) Exec(query string, args ...any) error {
	_, err := p.pool.Exec(context.Background(), query, args...)
	return err
}

func (p *PostgresDB) Query(query string, args ...any) ([]any, error) {
	rows, err := p.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		results = append(results, values)
	}
	return results, nil
}

func (p *PostgresDB) QueryRow(query string, args ...any) Row {
	row := p.pool.QueryRow(context.Background(), query, args...)
	return &pgxRow{row: row}
}

func (p *PostgresDB) Close() error {
	p.pool.Close()
	return nil
}

// GetPool returns the underlying connection pool
func (p *PostgresDB) GetPool() *pgxpool.Pool {
	return p.pool
}

// HealthCheck performs a health check on the database.
func (p *PostgresDB) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return p.pool.Ping(ctx)
}

// getEnv gets environment variable or returns default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// RunMigration executes database migrations
func RunMigration(db *PostgresDB, migrations []string) error {
	for _, migration := range migrations {
		log.Printf("Running migration: %s", migration)
		if err := db.Exec(migration); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", migration, err)
		}
	}

	log.Printf("All migrations completed successfully")
	return nil
}

// Migrations for the LLM facade
var migrations = []string{
	`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,

	`CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		username VARCHAR(255) UNIQUE NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		api_key VARCHAR(255) UNIQUE NOT NULL,
		role VARCHAR(50) DEFAULT 'user',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS user_sessions (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		user_id UUID REFERENCES users(id) ON DELETE CASCADE,
		session_token VARCHAR(255) UNIQUE NOT NULL,
		context JSONB DEFAULT '{}',
		memory_id UUID,
		status VARCHAR(50) DEFAULT 'active',
		request_count INTEGER DEFAULT 0,
		last_activity TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS llm_providers (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		name VARCHAR(255) UNIQUE NOT NULL,
		type VARCHAR(100) NOT NULL,
		api_key VARCHAR(255),
		base_url VARCHAR(500),
		model VARCHAR(255),
		weight DECIMAL(5,2) DEFAULT 1.0,
		enabled BOOLEAN DEFAULT TRUE,
		config JSONB DEFAULT '{}',
		health_status VARCHAR(50) DEFAULT 'unknown',
		response_time BIGINT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS llm_requests (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		session_id UUID REFERENCES user_sessions(id) ON DELETE CASCADE,
		user_id UUID REFERENCES users(id) ON DELETE CASCADE,
		prompt TEXT NOT NULL,
		messages JSONB NOT NULL DEFAULT '[]',
		model_params JSONB NOT NULL DEFAULT '{}',
		ensemble_config JSONB DEFAULT NULL,
		memory_enhanced BOOLEAN DEFAULT FALSE,
		memory JSONB DEFAULT '{}',
		status VARCHAR(50) DEFAULT 'pending',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		started_at TIMESTAMP WITH TIME ZONE,
		completed_at TIMESTAMP WITH TIME ZONE,
		request_type VARCHAR(50) DEFAULT 'completion'
	)`,

	`CREATE TABLE IF NOT EXISTS llm_responses (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		request_id UUID REFERENCES llm_requests(id) ON DELETE CASCADE,
		provider_id UUID REFERENCES llm_providers(id) ON DELETE SET NULL,
		provider_name VARCHAR(100) NOT NULL,
		content TEXT NOT NULL,
		confidence DECIMAL(3,2) NOT NULL DEFAULT 0.0,
		tokens_used INTEGER DEFAULT 0,
		response_time BIGINT DEFAULT 0,
		finish_reason VARCHAR(50) DEFAULT 'stop',
		metadata JSONB DEFAULT '{}',
		selected BOOLEAN DEFAULT FALSE,
		selection_score DECIMAL(5,2) DEFAULT 0.0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS cognee_memories (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		session_id UUID REFERENCES user_sessions(id) ON DELETE CASCADE,
		dataset_name VARCHAR(255) NOT NULL,
		content_type VARCHAR(50) DEFAULT 'text',
		content TEXT NOT NULL,
		vector_id VARCHAR(255),
		graph_nodes JSONB DEFAULT '{}',
		search_key VARCHAR(255),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
	`CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key)`,
	`CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at)`,
	`CREATE INDEX IF NOT EXISTS idx_user_sessions_session_token ON user_sessions(session_token)`,
	`CREATE INDEX IF NOT EXISTS idx_llm_providers_name ON llm_providers(name)`,
	`CREATE INDEX IF NOT EXISTS idx_llm_providers_enabled ON llm_providers(enabled)`,
	`CREATE INDEX IF NOT EXISTS idx_llm_requests_session_id ON llm_requests(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_llm_requests_user_id ON llm_requests(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_llm_requests_status ON llm_requests(status)`,
	`CREATE INDEX IF NOT EXISTS idx_llm_responses_request_id ON llm_responses(request_id)`,
	`CREATE INDEX IF NOT EXISTS idx_llm_responses_provider_id ON llm_responses(provider_id)`,
	`CREATE INDEX IF NOT EXISTS idx_llm_responses_selected ON llm_responses(selected)`,
	`CREATE INDEX IF NOT EXISTS idx_cognee_memories_session_id ON cognee_memories(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_cognee_memories_dataset_name ON cognee_memories(dataset_name)`,
	`CREATE INDEX IF NOT EXISTS idx_cognee_memories_search_key ON cognee_memories(search_key)`,
}

// Legacy interface for backward compatibility
type LegacyDB interface {
	Ping() error
	Exec(query string, args ...any) error
	Query(query string, args ...any) ([]any, error)
	Close() error
}

// Connect establishes a real PostgreSQL connection via pgx.
func Connect() (LegacyDB, error) {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "superagent")
	dbPassword := getEnv("DB_PASSWORD", "secret")
	dbName := getEnv("DB_NAME", "superagent_db")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &PostgresDB{pool: pool}, nil
}
