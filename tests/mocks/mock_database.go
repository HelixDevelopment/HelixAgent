package mocks

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/mock"
)

// MockPool implements a mock database pool for testing
type MockPool struct {
	mock.Mock
}

func NewMockPool() *MockPool {
	return &MockPool{}
}

func (m *MockPool) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pgxpool.Conn), args.Error(1)
}

func (m *MockPool) AcquireFunc(ctx context.Context, f func(*pgxpool.Conn) error) error {
	args := m.Called(ctx, f)
	return args.Error(0)
}

func (m *MockPool) AcquireAllIdle(ctx context.Context) []*pgxpool.Conn {
	args := m.Called(ctx)
	return args.Get(0).([]*pgxpool.Conn)
}

func (m *MockPool) Begin(ctx context.Context) (pgx.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func (m *MockPool) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	args := m.Called(ctx, txOptions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func (m *MockPool) Close() {
	m.Called()
}

func (m *MockPool) Config() *pgxpool.Config {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*pgxpool.Config)
}

func (m *MockPool) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	args := m.Called(ctx, tableName, columnNames, rowSrc)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPool) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *MockPool) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockPool) Query(ctx context.Context, sql string, arguments ...interface{}) (pgx.Rows, error) {
	args := m.Called(ctx, sql, arguments)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Rows), args.Error(1)
}

func (m *MockPool) QueryRow(ctx context.Context, sql string, arguments ...interface{}) pgx.Row {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgx.Row)
}

func (m *MockPool) Reset() {
	m.Called()
}

func (m *MockPool) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	args := m.Called(ctx, b)
	return args.Get(0).(pgx.BatchResults)
}

func (m *MockPool) Stat() *pgxpool.Stat {
	args := m.Called()
	return args.Get(0).(*pgxpool.Stat)
}

// MockRow implements pgx.Row for testing
type MockRow struct {
	mock.Mock
	values []interface{}
	err    error
}

func NewMockRow(values ...interface{}) *MockRow {
	return &MockRow{values: values}
}

func NewMockRowWithError(err error) *MockRow {
	return &MockRow{err: err}
}

func (m *MockRow) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	for i, v := range m.values {
		if i < len(dest) {
			switch d := dest[i].(type) {
			case *string:
				if s, ok := v.(string); ok {
					*d = s
				}
			case *int:
				if n, ok := v.(int); ok {
					*d = n
				}
			case *int64:
				if n, ok := v.(int64); ok {
					*d = n
				}
			case *float64:
				if f, ok := v.(float64); ok {
					*d = f
				}
			case *bool:
				if b, ok := v.(bool); ok {
					*d = b
				}
			case *time.Time:
				if t, ok := v.(time.Time); ok {
					*d = t
				}
			case *[]byte:
				if b, ok := v.([]byte); ok {
					*d = b
				}
			}
		}
	}
	return nil
}

// MockRows implements pgx.Rows for testing
type MockRows struct {
	mock.Mock
	rows    [][]interface{}
	current int
	closed  bool
}

func NewMockRows(rows ...[]interface{}) *MockRows {
	return &MockRows{rows: rows, current: -1}
}

func (m *MockRows) Close() {
	m.closed = true
}

func (m *MockRows) Err() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRows) CommandTag() pgconn.CommandTag {
	args := m.Called()
	return args.Get(0).(pgconn.CommandTag)
}

func (m *MockRows) FieldDescriptions() []pgconn.FieldDescription {
	args := m.Called()
	return args.Get(0).([]pgconn.FieldDescription)
}

func (m *MockRows) Next() bool {
	m.current++
	return m.current < len(m.rows)
}

func (m *MockRows) Scan(dest ...interface{}) error {
	if m.current < 0 || m.current >= len(m.rows) {
		return pgx.ErrNoRows
	}
	row := m.rows[m.current]
	for i, v := range row {
		if i < len(dest) {
			switch d := dest[i].(type) {
			case *string:
				if s, ok := v.(string); ok {
					*d = s
				}
			case *int:
				if n, ok := v.(int); ok {
					*d = n
				}
			case *int64:
				if n, ok := v.(int64); ok {
					*d = n
				}
			case *float64:
				if f, ok := v.(float64); ok {
					*d = f
				}
			case *bool:
				if b, ok := v.(bool); ok {
					*d = b
				}
			case *time.Time:
				if t, ok := v.(time.Time); ok {
					*d = t
				}
			}
		}
	}
	return nil
}

func (m *MockRows) Values() ([]interface{}, error) {
	if m.current < 0 || m.current >= len(m.rows) {
		return nil, pgx.ErrNoRows
	}
	return m.rows[m.current], nil
}

func (m *MockRows) RawValues() [][]byte {
	args := m.Called()
	return args.Get(0).([][]byte)
}

func (m *MockRows) Conn() *pgx.Conn {
	return nil
}

// MockTx implements pgx.Tx for testing
type MockTx struct {
	mock.Mock
}

func NewMockTx() *MockTx {
	return &MockTx{}
}

func (m *MockTx) Begin(ctx context.Context) (pgx.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func (m *MockTx) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) Rollback(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	args := m.Called(ctx, tableName, columnNames, rowSrc)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	args := m.Called(ctx, b)
	return args.Get(0).(pgx.BatchResults)
}

func (m *MockTx) LargeObjects() pgx.LargeObjects {
	args := m.Called()
	return args.Get(0).(pgx.LargeObjects)
}

func (m *MockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	args := m.Called(ctx, name, sql)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pgconn.StatementDescription), args.Error(1)
}

func (m *MockTx) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *MockTx) Query(ctx context.Context, sql string, arguments ...interface{}) (pgx.Rows, error) {
	args := m.Called(ctx, sql, arguments)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Rows), args.Error(1)
}

func (m *MockTx) QueryRow(ctx context.Context, sql string, arguments ...interface{}) pgx.Row {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgx.Row)
}

func (m *MockTx) Conn() *pgx.Conn {
	return nil
}
