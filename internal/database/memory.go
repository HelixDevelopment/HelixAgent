package database

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MemoryDB implements DB interface using in-memory storage
// This is used when PostgreSQL is not available (standalone/testing mode)
type MemoryDB struct {
	data    *safe.Store[string, []map[string]any]
	rowData *safe.Store[string, map[string][]any]
	enabled atomic.Bool
}

// memoryRow implements Row interface for in-memory queries
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
			// Simple type assertion - in real use, would need proper type conversion
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

// NewMemoryDB creates a new in-memory database
func NewMemoryDB() *MemoryDB {
	log.Println("Using in-memory database (standalone mode)")
	db := &MemoryDB{
		data:    safe.NewStore[string, []map[string]any](),
		rowData: safe.NewStore[string, map[string][]any](),
	}
	db.enabled.Store(true)
	return db
}

// NewPostgresDBWithFallback tries PostgreSQL first, falls back to memory
func NewPostgresDBWithFallback(cfg *config.Config) (*PostgresDB, *MemoryDB, error) {
	// Try PostgreSQL first
	db, err := NewPostgresDB(cfg)
	if err == nil {
		// Test the connection
		if pingErr := db.Ping(); pingErr == nil {
			return db, nil, nil
		}
		log.Printf("PostgreSQL ping failed, falling back to in-memory mode")
	} else {
		log.Printf("PostgreSQL connection failed: %v, using in-memory mode", err)
	}

	// Fall back to memory
	return nil, NewMemoryDB(), nil
}

func (m *MemoryDB) Ping() error {
	return nil // Always healthy
}

func (m *MemoryDB) Exec(query string, args ...any) error {
	// No-op for in-memory mode
	return nil
}

func (m *MemoryDB) Query(query string, args ...any) ([]any, error) {
	return nil, nil
}

func (m *MemoryDB) QueryRow(query string, args ...any) Row {
	// Parse simple queries to extract table and key
	// Supports: SELECT ... FROM table WHERE id = ?
	// This is a simplified implementation for standalone mode
	table := extractTableFromQuery(query)
	if table == "" {
		return &memoryRow{values: nil, err: fmt.Errorf("unable to parse query: %s", query)}
	}

	var result Row = &memoryRow{values: nil, err: fmt.Errorf("no rows found")}

	// Update-as-read so the inner map is inspected under the Store's
	// write lock, serialising with concurrent StoreRow callers that
	// mutate the same inner map in place.
	m.rowData.Update(table, func(inner map[string][]any, ok bool) (map[string][]any, bool) {
		if !ok {
			return inner, false
		}
		if len(args) > 0 {
			key := fmt.Sprintf("%v", args[0])
			if row, found := inner[key]; found {
				result = &memoryRow{values: row, err: nil}
				return inner, true
			}
		}
		for _, row := range inner {
			result = &memoryRow{values: row, err: nil}
			return inner, true
		}
		return inner, true
	})

	return result
}

// StoreRow stores a row in the in-memory database for later retrieval
func (m *MemoryDB) StoreRow(table string, key string, values []any) {
	m.rowData.Update(table, func(inner map[string][]any, ok bool) (map[string][]any, bool) {
		if !ok || inner == nil {
			inner = make(map[string][]any)
		}
		inner[key] = values
		return inner, true
	})
}

// extractTableFromQuery extracts the table name from a SQL query
func extractTableFromQuery(query string) string {
	// Simple extraction - look for "FROM table" pattern
	query = strings.ToLower(query)
	if idx := strings.Index(query, "from "); idx >= 0 {
		rest := query[idx+5:]
		// Get the table name (first word after FROM)
		parts := strings.Fields(rest)
		if len(parts) > 0 {
			// Remove any trailing WHERE, ORDER, etc.
			table := strings.TrimSuffix(parts[0], ",")
			return table
		}
	}
	return ""
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

// GetPool returns nil for memory database (no real pool)
func (m *MemoryDB) GetPool() *pgxpool.Pool {
	return nil
}

// IsMemoryMode returns true if this is an in-memory database
func (m *MemoryDB) IsMemoryMode() bool {
	return true
}
