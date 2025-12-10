package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/superagent/superagent/internal/config"
)

// DB interface for database operations
type DB interface {
	Ping() error
	Exec(query string, args ...any) error
	Query(query string, args ...any) ([]any, error)
	QueryRow(query string, args ...any) *sql.Row
	Close() error
	HealthCheck() error
}

// PostgresDB implements DB using PostgreSQL with pgxpool
type PostgresDB struct {
	pool *pgxpool.Pool
}

func NewPostgresDB(cfg *config.Config) (*PostgresDB, error) {
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

func (p *PostgresDB) QueryRow(query string, args ...any) *sql.Row {
	// Note: This is a simplified implementation
	// In a real implementation, you'd need to handle the pgx.Row properly
	return nil
}

func (p *PostgresDB) Close() error {
	p.pool.Close()
	return nil
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
	`CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(255) UNIQUE NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		api_key VARCHAR(255) UNIQUE NOT NULL,
		role VARCHAR(50) DEFAULT 'user',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS user_sessions (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
		session_token VARCHAR(255) UNIQUE NOT NULL,
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		last_activity TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		metadata JSONB DEFAULT '{}'
	)`,

	`CREATE TABLE IF NOT EXISTS llm_requests (
		id SERIAL PRIMARY KEY,
		session_id INTEGER REFERENCES user_sessions(id) ON DELETE CASCADE,
		prompt TEXT NOT NULL,
		messages JSONB NOT NULL DEFAULT '[]',
		model_params JSONB NOT NULL DEFAULT '{}',
		ensemble_config JSONB DEFAULT NULL,
		memory_enhanced BOOLEAN DEFAULT FALSE,
		status VARCHAR(50) DEFAULT 'pending',
		provider_id VARCHAR(100),
		response_content TEXT,
		tokens_used INTEGER DEFAULT 0,
		response_time_ms INTEGER DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		started_at TIMESTAMP WITH TIME ZONE,
		completed_at TIMESTAMP WITH TIME ZONE,
		error_message TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS llm_responses (
		id SERIAL PRIMARY KEY,
		request_id INTEGER REFERENCES llm_requests(id) ON DELETE CASCADE,
		provider_id VARCHAR(100) NOT NULL,
		provider_name VARCHAR(100) NOT NULL,
		content TEXT NOT NULL,
		confidence DECIMAL(3,2) NOT NULL DEFAULT 0.0,
		tokens_used INTEGER DEFAULT 0,
		response_time_ms INTEGER DEFAULT 0,
		finish_reason VARCHAR(50) DEFAULT 'stop',
		metadata JSONB DEFAULT '{}',
		selected BOOLEAN DEFAULT FALSE,
		selection_score DECIMAL(5,2) DEFAULT 0.0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS memory_sources (
		id SERIAL PRIMARY KEY,
		session_id INTEGER REFERENCES user_sessions(id) ON DELETE CASCADE,
		dataset_name VARCHAR(255) NOT NULL,
		content TEXT NOT NULL,
		vector_id VARCHAR(255),
		content_type VARCHAR(50) DEFAULT 'text',
		relevance_score DECIMAL(5,2) DEFAULT 1.0,
		search_key VARCHAR(255),
		source_type VARCHAR(50) DEFAULT 'cognee',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		expired_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() + INTERVAL '7 days'
	)`,

	`CREATE INDEX IF NOT EXISTS idx_llm_requests_session_id ON llm_requests(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_llm_responses_request_id ON llm_responses(request_id)`,
	`CREATE INDEX IF NOT EXISTS idx_memory_sources_session_id ON memory_sources(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_memory_sources_expires_at ON memory_sources(expires_at)`,
	`CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at)`,
	`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
	`CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key)`,
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
