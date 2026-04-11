package database

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mock implementations for testing
// ============================================================================

type mockDatabase struct {
	connectCalled     bool
	closeCalled       bool
	execCalled        bool
	queryCalled       bool
	queryRowCalled    bool
	beginCalled       bool
	healthCheckCalled bool
	poolReturned      bool

	connectErr     error
	closeErr       error
	execResult     mockResult
	execErr        error
	queryRows      *mockRows
	queryErr       error
	queryRowResult mockRow
	beginTx        *mockTx
	beginErr       error
	healthCheckErr error
}

type mockResult struct {
	affected int64
	err      error
}

func (m mockResult) RowsAffected() (int64, error) {
	return m.affected, m.err
}

type mockRow struct {
	scanErr error
	values  []any
}

func (m mockRow) Scan(dest ...any) error {
	return m.scanErr
}

type mockRows struct {
	nextReturns []bool
	nextIndex   int
	scanErr     error
	closeErr    error
	errVal      error
}

func (m *mockRows) Next() bool {
	if m.nextIndex < len(m.nextReturns) {
		result := m.nextReturns[m.nextIndex]
		m.nextIndex++
		return result
	}
	return false
}

func (m *mockRows) Scan(dest ...any) error {
	return m.scanErr
}

func (m *mockRows) Close() error {
	return m.closeErr
}

func (m *mockRows) Err() error {
	return m.errVal
}

type mockTx struct {
	commitCalled   bool
	rollbackCalled bool
	commitErr      error
	rollbackErr    error
	execResult     mockResult
	execErr        error
	queryRows      *mockRows
	queryErr       error
	queryRowResult mockRow
}

func (m *mockTx) Commit(ctx context.Context) error {
	m.commitCalled = true
	return m.commitErr
}

func (m *mockTx) Rollback(ctx context.Context) error {
	m.rollbackCalled = true
	return m.rollbackErr
}

func (m *mockTx) Exec(ctx context.Context, query string, args ...any) (Result, error) {
	return m.execResult, m.execErr
}

func (m *mockTx) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	return m.queryRows, m.queryErr
}

func (m *mockTx) QueryRow(ctx context.Context, query string, args ...any) Row {
	return m.queryRowResult
}

// Implement db.Database interface methods for mockDatabase
func (m *mockDatabase) Connect(ctx context.Context) error {
	m.connectCalled = true
	return m.connectErr
}

func (m *mockDatabase) Close() error {
	m.closeCalled = true
	return m.closeErr
}

func (m *mockDatabase) Exec(ctx context.Context, query string, args ...any) (Result, error) {
	m.execCalled = true
	return m.execResult, m.execErr
}

func (m *mockDatabase) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	m.queryCalled = true
	return m.queryRows, m.queryErr
}

func (m *mockDatabase) QueryRow(ctx context.Context, query string, args ...any) Row {
	m.queryRowCalled = true
	return m.queryRowResult
}

func (m *mockDatabase) Begin(ctx context.Context) (Tx, error) {
	m.beginCalled = true
	return m.beginTx, m.beginErr
}

func (m *mockDatabase) HealthCheck(ctx context.Context) error {
	m.healthCheckCalled = true
	return m.healthCheckErr
}

// Migrate is not part of db.Database but we need it for the interface
func (m *mockDatabase) Migrate(ctx context.Context, migrations []string) error {
	return nil
}

// ============================================================================
// ErrorRow Tests
// ============================================================================

func TestErrorRow_ScanReturnsError(t *testing.T) {
	t.Parallel()
	testErr := errors.New("test error")
	row := &ErrorRow{err: testErr}

	var dest string
	err := row.Scan(&dest)

	assert.Error(t, err)
	assert.Equal(t, testErr, err)
}

func TestErrorRow_ScanWithMultipleDestinations(t *testing.T) {
	t.Parallel()
	testErr := errors.New("connection failed")
	row := &ErrorRow{err: testErr}

	var a, b, c string
	err := row.Scan(&a, &b, &c)

	assert.Error(t, err)
	assert.Equal(t, testErr, err)
}

// ============================================================================
// Client with Mock Tests
// ============================================================================

func newTestClient(mockDB *mockDatabase) *Client {
	return &Client{
		pg:     nil,
		pool:   nil,
		testPG: mockDB,
	}
}

func TestClient_Pool_WithMock(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	pool := client.Pool()

	// Pool returns nil when using mock (no real pool)
	assert.Nil(t, pool)
}

func TestClient_Database_ReturnsUnderlyingDB(t *testing.T) {
	t.Parallel()
	// Create a real client to test Database() method
	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)

	db := client.Database()
	assert.NotNil(t, db)
}

func TestClient_Close_WithMock(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	err := client.Close()

	assert.NoError(t, err)
	assert.True(t, mock.closeCalled)
}

func TestClient_Close_MockError(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{closeErr: errors.New("close failed")}
	client := newTestClient(mock)

	err := client.Close()

	assert.Error(t, err)
	assert.Equal(t, "close failed", err.Error())
}

func TestClient_Ping_WithMock(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	err := client.Ping()

	assert.NoError(t, err)
	assert.True(t, mock.healthCheckCalled)
}

func TestClient_Ping_MockError(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{healthCheckErr: errors.New("ping failed")}
	client := newTestClient(mock)

	err := client.Ping()

	assert.Error(t, err)
	assert.Equal(t, "ping failed", err.Error())
}

func TestClient_HealthCheck_WithMock(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	err := client.HealthCheck()

	assert.NoError(t, err)
	assert.True(t, mock.healthCheckCalled)
}

func TestClient_HealthCheck_MockError(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{healthCheckErr: errors.New("health check failed")}
	client := newTestClient(mock)

	err := client.HealthCheck()

	assert.Error(t, err)
	assert.Equal(t, "health check failed", err.Error())
}

func TestClient_Exec_WithMock(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	err := client.Exec("INSERT INTO test VALUES ($1)", "value")

	assert.NoError(t, err)
	assert.True(t, mock.execCalled)
}

func TestClient_Exec_MockError(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{execErr: errors.New("exec failed")}
	client := newTestClient(mock)

	err := client.Exec("INSERT INTO test VALUES ($1)", "value")

	assert.Error(t, err)
	assert.Equal(t, "exec failed", err.Error())
}

func TestClient_Query_WithMock(t *testing.T) {
	t.Parallel()
	mockRows := &mockRows{nextReturns: []bool{true, true, false}}
	mock := &mockDatabase{queryRows: mockRows}
	client := newTestClient(mock)

	results, err := client.Query("SELECT * FROM test")

	assert.NoError(t, err)
	assert.True(t, mock.queryCalled)
	assert.Len(t, results, 2) // Two rows
}

func TestClient_Query_MockError(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{queryErr: errors.New("query failed")}
	client := newTestClient(mock)

	results, err := client.Query("SELECT * FROM test")

	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Equal(t, "query failed", err.Error())
}

func TestClient_Query_RowsError(t *testing.T) {
	t.Parallel()
	mockRows := &mockRows{
		nextReturns: []bool{true, false},
		errVal:      errors.New("rows iteration error"),
	}
	mock := &mockDatabase{queryRows: mockRows}
	client := newTestClient(mock)

	results, err := client.Query("SELECT * FROM test")

	assert.Error(t, err)
	assert.Equal(t, "rows iteration error", err.Error())
	assert.Len(t, results, 1)
}

func TestClient_QueryRow_WithMock(t *testing.T) {
	t.Parallel()
	mockRow := mockRow{}
	mock := &mockDatabase{queryRowResult: mockRow}
	client := newTestClient(mock)

	row := client.QueryRow("SELECT * FROM test WHERE id = $1", 1)

	assert.NotNil(t, row)
	assert.True(t, mock.queryRowCalled)
}

func TestClient_Begin_WithMock(t *testing.T) {
	t.Parallel()
	mockTx := &mockTx{}
	mock := &mockDatabase{beginTx: mockTx}
	client := newTestClient(mock)

	tx, err := client.Begin(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, tx)
	assert.True(t, mock.beginCalled)
}

func TestClient_Begin_MockError(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{beginErr: errors.New("begin failed")}
	client := newTestClient(mock)

	tx, err := client.Begin(context.Background())

	assert.Error(t, err)
	assert.Nil(t, tx)
	assert.Equal(t, "begin failed", err.Error())
}

// ============================================================================
// Connection Failure Tests
// ============================================================================

func TestClient_Pool_ConnectionFailure(t *testing.T) {
	t.Parallel()
	// Create client with invalid config to trigger connection failure
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	pool := client.Pool()
	assert.Nil(t, pool)
}

func TestClient_Ping_ConnectionFailure(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	err = client.Ping()
	assert.Error(t, err)
}

func TestClient_HealthCheck_ConnectionFailure(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	err = client.HealthCheck()
	assert.Error(t, err)
}

func TestClient_Exec_ConnectionFailure(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	err = client.Exec("SELECT 1")
	assert.Error(t, err)
}

func TestClient_Query_ConnectionFailure(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	results, err := client.Query("SELECT 1")
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestClient_QueryRow_ConnectionFailure(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	row := client.QueryRow("SELECT 1")
	assert.NotNil(t, row)

	var dest int
	err = row.Scan(&dest)
	assert.Error(t, err)
}

func TestClient_Begin_ConnectionFailure(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	tx, err := client.Begin(context.Background())
	assert.Error(t, err)
	assert.Nil(t, tx)
}

func TestClient_Migrate_ConnectionFailure(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	err = client.Migrate(context.Background(), []string{"CREATE TABLE test (id INT)"})
	assert.Error(t, err)
}

// ============================================================================
// Context Deadline Tests for initConnection
// ============================================================================

func TestClient_initConnection_WithExistingDeadline(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	// Create context with deadline
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.initConnection(ctx)

	assert.NoError(t, err)
}

func TestClient_initConnection_WithoutDeadline(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	// Use context without deadline
	ctx := context.Background()

	err := client.initConnection(ctx)

	assert.NoError(t, err)
}

func TestClient_initConnection_AlreadyConnected(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	// First connection
	err := client.initConnection(context.Background())
	assert.NoError(t, err)

	// Second connection attempt - should use sync.Once, no error
	err = client.initConnection(context.Background())
	assert.NoError(t, err)
	// connectCalled should still be false because we use the test hook
	assert.False(t, mock.connectCalled)
}

// ============================================================================
// NewClient and NewClientWithFallback Tests
// ============================================================================

func TestNewClient_Success(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "testuser",
			Password: "testpass",
			Name:     "testdb",
			SSLMode:  "disable",
		},
	}

	client, err := NewClient(cfg)

	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_WithEmptyConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	client, err := NewClient(cfg)

	assert.NoError(t, err)
	assert.NotNil(t, client)
}

// ============================================================================
// Type Alias Tests
// ============================================================================

func TestTypeAliases(t *testing.T) {
	t.Parallel()
	// These tests ensure type aliases compile correctly

	// Test Row alias
	var _ Row = (mockRow{})

	// Test Rows alias
	var _ Rows = (&mockRows{})

	// Test Tx alias
	var _ Tx = (&mockTx{})

	// Test Result alias
	var _ Result = (mockResult{})
}
