package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryDB(t *testing.T) {
	db := NewMemoryDB()
	require.NotNil(t, db)
	assert.True(t, db.enabled.Load())
	assert.NotNil(t, db.data)
}

func TestMemoryDB_Ping(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	err := db.Ping()
	assert.NoError(t, err)
}

func TestMemoryDB_Exec(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	err := db.Exec("INSERT INTO test VALUES ($1)", "value")
	assert.NoError(t, err)
}

func TestMemoryDB_Query(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	result, err := db.Query("SELECT * FROM test")
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestMemoryDB_QueryRow(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	row := db.QueryRow("SELECT * FROM test")
	assert.NotNil(t, row)

	var value string
	err := row.Scan(&value)
	assert.Error(t, err)
}

func TestMemoryDB_Close(t *testing.T) {
	db := NewMemoryDB()

	err := db.Close()
	assert.NoError(t, err)
	assert.False(t, db.enabled.Load())
	assert.Equal(t, 0, db.data.Len())
}

func TestMemoryDB_HealthCheck(t *testing.T) {
	db := NewMemoryDB()

	err := db.HealthCheck()
	assert.NoError(t, err)

	_ = db.Close()

	err = db.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory database closed")
}

func TestMemoryDB_GetPool(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	pool := db.GetPool()
	assert.Nil(t, pool)
}

func TestMemoryDB_IsMemoryMode(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	assert.True(t, db.IsMemoryMode())
}

func TestMemoryRow_Scan_StringValue(t *testing.T) {
	row := &memoryRow{
		values: []any{"test-string"},
		err:    nil,
	}

	var s string
	err := row.Scan(&s)
	assert.NoError(t, err)
	assert.Equal(t, "test-string", s)
}

func TestMemoryRow_Scan_IntValue(t *testing.T) {
	row := &memoryRow{
		values: []any{42},
		err:    nil,
	}

	var n int
	err := row.Scan(&n)
	assert.NoError(t, err)
	assert.Equal(t, 42, n)
}

func TestMemoryRow_Scan_BoolValue(t *testing.T) {
	row := &memoryRow{
		values: []any{true},
		err:    nil,
	}

	var b bool
	err := row.Scan(&b)
	assert.NoError(t, err)
	assert.True(t, b)
}

func TestMemoryRow_Scan_MultipleValues(t *testing.T) {
	row := &memoryRow{
		values: []any{"hello", 123, true},
		err:    nil,
	}

	var s string
	var n int
	var b bool
	err := row.Scan(&s, &n, &b)
	assert.NoError(t, err)
	assert.Equal(t, "hello", s)
	assert.Equal(t, 123, n)
	assert.True(t, b)
}

func TestMemoryRow_Scan_WithError(t *testing.T) {
	row := &memoryRow{
		values: nil,
		err:    assert.AnError,
	}

	var s string
	err := row.Scan(&s)
	assert.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

func TestMemoryRow_Scan_NoRows(t *testing.T) {
	row := &memoryRow{
		values: []any{},
		err:    nil,
	}

	var s string
	err := row.Scan(&s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}

func TestMemoryRow_Scan_TypeMismatch(t *testing.T) {
	row := &memoryRow{
		values: []any{"string-value"},
		err:    nil,
	}

	var n int
	// Scan won't error but won't update the value either
	err := row.Scan(&n)
	assert.NoError(t, err)
	assert.Equal(t, 0, n) // Default int value, not updated
}

func TestMemoryRow_Scan_MoreDestsThanValues(t *testing.T) {
	row := &memoryRow{
		values: []any{"value1"},
		err:    nil,
	}

	var s1, s2 string
	err := row.Scan(&s1, &s2)
	assert.NoError(t, err)
	assert.Equal(t, "value1", s1)
	assert.Empty(t, s2) // Not filled
}

// Tests for QueryRow implementation (CRIT-001 fix)

func TestMemoryDB_QueryRow_WithStoredData(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	// Store data first
	db.StoreRow("users", "user-123", []any{"user-123", "john@example.com", "John Doe"})

	// Query for the stored data
	row := db.QueryRow("SELECT id, email, name FROM users WHERE id = $1", "user-123")
	assert.NotNil(t, row)

	var id, email, name string
	err := row.Scan(&id, &email, &name)
	assert.NoError(t, err)
	assert.Equal(t, "user-123", id)
	assert.Equal(t, "john@example.com", email)
	assert.Equal(t, "John Doe", name)
}

func TestMemoryDB_QueryRow_NotFound(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	// Query without storing data first
	row := db.QueryRow("SELECT * FROM users WHERE id = $1", "nonexistent")
	assert.NotNil(t, row)

	var value string
	err := row.Scan(&value)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}

func TestMemoryDB_QueryRow_InvalidQuery(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	// Query with invalid SQL (no FROM clause)
	row := db.QueryRow("SELECT * WHERE id = $1", "test")
	assert.NotNil(t, row)

	var value string
	err := row.Scan(&value)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to parse query")
}

func TestMemoryDB_StoreRow_MultipleRows(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	// Store multiple rows in the same table
	db.StoreRow("sessions", "sess-1", []any{"sess-1", "active", 100})
	db.StoreRow("sessions", "sess-2", []any{"sess-2", "expired", 50})
	db.StoreRow("sessions", "sess-3", []any{"sess-3", "active", 200})

	// Query for specific session
	row := db.QueryRow("SELECT id, status, requests FROM sessions WHERE id = $1", "sess-2")
	var id, status string
	var requests int
	err := row.Scan(&id, &status, &requests)
	assert.NoError(t, err)
	assert.Equal(t, "sess-2", id)
	assert.Equal(t, "expired", status)
	assert.Equal(t, 50, requests)
}

func TestMemoryDB_StoreRow_Overwrite(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	// Store initial data
	db.StoreRow("config", "setting-1", []any{"old-value"})

	// Overwrite with new data
	db.StoreRow("config", "setting-1", []any{"new-value"})

	// Verify new data
	row := db.QueryRow("SELECT value FROM config WHERE key = $1", "setting-1")
	var value string
	err := row.Scan(&value)
	assert.NoError(t, err)
	assert.Equal(t, "new-value", value)
}

func TestMemoryDB_QueryRow_FirstRowWithoutArgs(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	// Store some data
	db.StoreRow("providers", "p1", []any{"claude", "active"})

	// Query without specific key - should return first row
	row := db.QueryRow("SELECT name, status FROM providers")
	var name, status string
	err := row.Scan(&name, &status)
	assert.NoError(t, err)
	assert.Equal(t, "claude", name)
}

func TestMemoryDB_QueryRow_ConcurrentAccess(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	// Store initial data
	db.StoreRow("concurrent", "key-1", []any{"value-1"})

	// Run concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			row := db.QueryRow("SELECT v FROM concurrent WHERE k = $1", "key-1")
			var v string
			_ = row.Scan(&v)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestExtractTableFromQuery(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"SELECT * FROM users WHERE id = $1", "users"},
		{"SELECT id, name FROM providers", "providers"},
		{"select * from sessions where status = 'active'", "sessions"},
		{"SELECT COUNT(*) FROM requests WHERE user_id = $1", "requests"},
		{"SELECT * FROM user_sessions WHERE id = $1 ORDER BY created_at", "user_sessions"},
		{"INVALID QUERY", ""},
		{"SELECT * WHERE id = $1", ""},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := extractTableFromQuery(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMemoryDB_MultipleTables(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	// Store data in different tables
	db.StoreRow("users", "u1", []any{"u1", "alice"})
	db.StoreRow("sessions", "s1", []any{"s1", "active"})
	db.StoreRow("requests", "r1", []any{"r1", "pending"})

	// Query each table
	row1 := db.QueryRow("SELECT id, name FROM users WHERE id = $1", "u1")
	var uid, uname string
	err := row1.Scan(&uid, &uname)
	assert.NoError(t, err)
	assert.Equal(t, "alice", uname)

	row2 := db.QueryRow("SELECT id, status FROM sessions WHERE id = $1", "s1")
	var sid, sstatus string
	err = row2.Scan(&sid, &sstatus)
	assert.NoError(t, err)
	assert.Equal(t, "active", sstatus)

	row3 := db.QueryRow("SELECT id, status FROM requests WHERE id = $1", "r1")
	var rid, rstatus string
	err = row3.Scan(&rid, &rstatus)
	assert.NoError(t, err)
	assert.Equal(t, "pending", rstatus)
}
