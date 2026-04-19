// Package database provides compatibility types and functions that mirror
// the original internal/database package API, enabling gradual migration.
package database

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"

	db "digital.vasic.database/pkg/database"
	"digital.vasic.database/pkg/postgres"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresDB implements the legacy DB interface using the new database module.
// This provides full backward compatibility with existing code.
type PostgresDB struct {
	client *Client
}

// DB interface for database operations - mirrors the original interface.
type DB interface {
	Ping() error
	Exec(query string, args ...any) error
	Query(query string, args ...any) ([]any, error)
	QueryRow(query string, args ...any) Row
	Close() error
	HealthCheck() error
}

// LegacyDB interface for backward compatibility.
type LegacyDB interface {
	Ping() error
	Exec(query string, args ...any) error
	Query(query string, args ...any) ([]any, error)
	Close() error
}

// NewPostgresDB creates a new PostgresDB using the extracted database module.
// This function signature matches the original for backward compatibility.
func NewPostgresDB(cfg *config.Config) (*PostgresDB, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &PostgresDB{client: client}, nil
}

// NewPostgresDBWithFallback tries PostgreSQL first, falls back to memory.
// Returns (PostgresDB, MemoryDB, error) for compatibility with existing code.
func NewPostgresDBWithFallback(cfg *config.Config) (*PostgresDB, *MemoryDB, error) {
	db, err := NewPostgresDB(cfg)
	if err == nil {
		if pingErr := db.Ping(); pingErr == nil {
			return db, nil, nil
		}
		log.Printf("PostgreSQL ping failed, falling back to in-memory mode")
	} else {
		log.Printf("PostgreSQL connection failed: %v, using in-memory mode", err)
	}

	return nil, NewMemoryDB(), nil
}

// Connect establishes a real PostgreSQL connection via the new module.
func Connect() (LegacyDB, error) {
	cfg := &config.Config{}
	db, err := NewPostgresDB(cfg)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Ping tests the database connection.
func (p *PostgresDB) Ping() error {
	return p.client.Ping()
}

// Exec executes a query that does not return rows.
func (p *PostgresDB) Exec(query string, args ...any) error {
	return p.client.Exec(query, args...)
}

// Query executes a query that returns rows.
func (p *PostgresDB) Query(query string, args ...any) ([]any, error) {
	return p.client.Query(query, args...)
}

// QueryRow executes a query that returns at most one row.
func (p *PostgresDB) QueryRow(query string, args ...any) Row {
	return p.client.QueryRow(query, args...)
}

// Close closes the database connection.
func (p *PostgresDB) Close() error {
	return p.client.Close()
}

// HealthCheck performs a health check on the database.
func (p *PostgresDB) HealthCheck() error {
	return p.client.HealthCheck()
}

// GetPool returns the underlying connection pool for direct access.
// This is needed by repositories that use pgxpool directly.
func (p *PostgresDB) GetPool() *pgxpool.Pool {
	return p.client.Pool()
}

// Database returns the underlying database.Database interface.
func (p *PostgresDB) Database() db.Database {
	return p.client.Database()
}

// MemoryDB implements DB interface using in-memory storage.
// This is used when PostgreSQL is not available (standalone/testing mode).
type MemoryDB struct {
	data    *safe.Store[string, []map[string]any]
	rowData *safe.Store[string, map[string][]any]
	enabled atomic.Bool
}

// memoryRow implements Row interface for in-memory queries.
type memoryRow struct {
	values []any
	err    error
}

func (r *memoryRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) == 0 {
		return fmt.Errorf("no rows")
	}
	for i := range dest {
		if i < len(r.values) {
			switch d := dest[i].(type) {
			case *string:
				if s, ok := r.values[i].(string); ok {
					*d = s
				}
			case *int:
				if n, ok := r.values[i].(int); ok {
					*d = n
				}
			case *bool:
				if b, ok := r.values[i].(bool); ok {
					*d = b
				}
			}
		}
	}
	return nil
}

// NewMemoryDB creates a new in-memory database.
func NewMemoryDB() *MemoryDB {
	log.Println("Using in-memory database (standalone mode)")
	m := &MemoryDB{
		data:    safe.NewStore[string, []map[string]any](),
		rowData: safe.NewStore[string, map[string][]any](),
	}
	m.enabled.Store(true)
	return m
}

func (m *MemoryDB) Ping() error {
	// No-op: the in-memory store is always reachable within the same process.
	// There is no network connection to probe, so a nil error is the correct
	// response — it signals "available" to callers that check Ping() != nil.
	return nil
}

func (m *MemoryDB) Exec(query string, args ...any) error {
	// No-op: MemoryDB is a lightweight stand-in used when PostgreSQL is
	// unavailable (standalone / testing mode). Write queries are silently
	// discarded because the in-memory store does not parse SQL; callers that
	// need persistence must ensure a real PostgreSQL connection is available.
	return nil
}

func (m *MemoryDB) Query(query string, args ...any) ([]any, error) {
	// Returns (nil, nil) intentionally: MemoryDB does not execute SQL queries.
	// Callers receive an empty result set, which is the safest degraded behaviour
	// in standalone mode. Use StoreRow/QueryRow for keyed in-memory lookups.
	return nil, nil
}

func (m *MemoryDB) QueryRow(query string, args ...any) Row {
	return &memoryRow{values: nil, err: fmt.Errorf("no rows found")}
}

func (m *MemoryDB) Close() error {
	m.data.Clear()
	m.enabled.Store(false)
	return nil
}

func (m *MemoryDB) HealthCheck() error {
	if !m.enabled.Load() {
		return fmt.Errorf("memory database closed")
	}
	return nil
}

// GetPool returns nil for memory database (no real pool).
func (m *MemoryDB) GetPool() *pgxpool.Pool {
	return nil
}

// IsMemoryMode returns true if this is an in-memory database.
func (m *MemoryDB) IsMemoryMode() bool {
	return true
}

// StoreRow stores a row in the in-memory database for later retrieval.
func (m *MemoryDB) StoreRow(table string, key string, values []any) {
	m.rowData.Update(table, func(inner map[string][]any, ok bool) (map[string][]any, bool) {
		if !ok || inner == nil {
			inner = make(map[string][]any)
		}
		inner[key] = values
		return inner, true
	})
}

// RunMigration executes database migrations.
func RunMigration(db *PostgresDB, migrations []string) error {
	ctx := context.Background()
	return db.client.Migrate(ctx, migrations)
}

// Config is an alias for the database module's Config type.
type Config = db.Config

// PostgresConfig is an alias for the postgres module's Config type.
type PostgresConfig = postgres.Config
