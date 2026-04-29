// Package pgvector provides a client for PostgreSQL with pgvector extension.
package pgvector

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/testutil"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "nil config uses defaults",
			config:  nil,
			wantErr: false,
		},
		{
			name: "valid config",
			config: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Database: "testdb",
			},
			wantErr: false,
		},
		{
			name: "empty host",
			config: &Config{
				Host: "",
				Port: 5432,
				User: "postgres",
			},
			wantErr:   true,
			errSubstr: "host is required",
		},
		{
			name: "invalid port",
			config: &Config{
				Host: "localhost",
				Port: 0,
				User: "postgres",
			},
			wantErr:   true,
			errSubstr: "invalid port",
		},
		{
			name: "empty user",
			config: &Config{
				Host: "localhost",
				Port: 5432,
				User: "",
			},
			wantErr:   true,
			errSubstr: "user is required",
		},
		{
			name: "empty database",
			config: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Database: "",
			},
			wantErr:   true,
			errSubstr: "database is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config, nil)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, client)
			assert.False(t, client.IsConnected())
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 5432, config.Port)
	assert.Equal(t, "postgres", config.User)
	assert.Equal(t, "postgres", config.Database)
	assert.Equal(t, "disable", config.SSLMode)
	assert.Equal(t, int32(10), config.MaxConns)
	assert.Equal(t, int32(2), config.MinConns)
	assert.Equal(t, time.Hour, config.MaxConnLifetime)
	assert.Equal(t, 30*time.Minute, config.MaxConnIdleTime)
	assert.Equal(t, 30*time.Second, config.ConnectTimeout)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid config",
			config: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Database: "testdb",
			},
			wantErr: false,
		},
		{
			name: "empty host",
			config: &Config{
				Host:     "",
				Port:     5432,
				User:     "postgres",
				Database: "testdb",
			},
			wantErr:   true,
			errSubstr: "host is required",
		},
		{
			name: "zero port",
			config: &Config{
				Host:     "localhost",
				Port:     0,
				User:     "postgres",
				Database: "testdb",
			},
			wantErr:   true,
			errSubstr: "invalid port",
		},
		{
			name: "negative port",
			config: &Config{
				Host:     "localhost",
				Port:     -1,
				User:     "postgres",
				Database: "testdb",
			},
			wantErr:   true,
			errSubstr: "invalid port",
		},
		{
			name: "empty user",
			config: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "",
				Database: "testdb",
			},
			wantErr:   true,
			errSubstr: "user is required",
		},
		{
			name: "empty database",
			config: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Database: "",
			},
			wantErr:   true,
			errSubstr: "database is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		contains []string
	}{
		{
			name: "basic config",
			config: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Database: "testdb",
			},
			contains: []string{"host=localhost", "port=5432", "user=postgres", "dbname=testdb"},
		},
		{
			name: "with password",
			config: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Password: "secret",
				Database: "testdb",
			},
			contains: []string{"password=secret"},
		},
		{
			name: "with ssl mode",
			config: &Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Database: "testdb",
				SSLMode:  "require",
			},
			contains: []string{"sslmode=require"},
		},
		{
			name: "with timeout",
			config: &Config{
				Host:           "localhost",
				Port:           5432,
				User:           "postgres",
				Database:       "testdb",
				ConnectTimeout: 10 * time.Second,
			},
			contains: []string{"connect_timeout=10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connStr := tt.config.ConnectionString()
			for _, s := range tt.contains {
				assert.Contains(t, connStr, s)
			}
		})
	}
}

func TestVectorToString(t *testing.T) {
	tests := []struct {
		name     string
		vector   []float32
		expected string
	}{
		{
			name:     "empty vector",
			vector:   []float32{},
			expected: "[]",
		},
		{
			name:     "single element",
			vector:   []float32{1.0},
			expected: "[1.000000]",
		},
		{
			name:     "multiple elements",
			vector:   []float32{0.1, 0.2, 0.3},
			expected: "[0.100000,0.200000,0.300000]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vectorToString(tt.vector)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDistanceMetricConstants(t *testing.T) {
	assert.Equal(t, DistanceMetric("l2"), DistanceL2)
	assert.Equal(t, DistanceMetric("ip"), DistanceIP)
	assert.Equal(t, DistanceMetric("cosine"), DistanceCosine)
}

func TestIndexTypeConstants(t *testing.T) {
	assert.Equal(t, IndexType("ivfflat"), IndexTypeIVFFlat)
	assert.Equal(t, IndexType("hnsw"), IndexTypeHNSW)
}

func TestClient_NotConnected(t *testing.T) {
	config := &Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Database: "testdb",
	}
	client, err := NewClient(config, logrus.New())
	require.NoError(t, err)

	t.Run("HealthCheck", func(t *testing.T) {
		err := client.HealthCheck(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("CreateTable", func(t *testing.T) {
		err := client.CreateTable(context.Background(), &TableSchema{
			TableName:    "test",
			VectorColumn: "embedding",
			Dimension:    768,
			IDColumn:     "id",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("DropTable", func(t *testing.T) {
		err := client.DropTable(context.Background(), "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("TableExists", func(t *testing.T) {
		_, err := client.TableExists(context.Background(), "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("CreateIndex", func(t *testing.T) {
		err := client.CreateIndex(context.Background(), &CreateIndexRequest{
			TableName:    "test",
			IndexName:    "test_idx",
			VectorColumn: "embedding",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("Upsert", func(t *testing.T) {
		_, err := client.Upsert(context.Background(), &UpsertRequest{
			TableName:    "test",
			VectorColumn: "embedding",
			IDColumn:     "id",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("Search", func(t *testing.T) {
		_, err := client.Search(context.Background(), &SearchRequest{
			TableName:    "test",
			VectorColumn: "embedding",
			IDColumn:     "id",
			QueryVector:  []float32{0.1, 0.2, 0.3},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("Delete", func(t *testing.T) {
		_, err := client.Delete(context.Background(), &DeleteRequest{
			TableName: "test",
			IDColumn:  "id",
			IDs:       []string{"id1"},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("Get", func(t *testing.T) {
		_, err := client.Get(context.Background(), "test", "id", []string{"id1"}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("Count", func(t *testing.T) {
		_, err := client.Count(context.Background(), "test", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})
}

func TestClient_Close(t *testing.T) {
	config := DefaultConfig()
	client, err := NewClient(config, logrus.New())
	require.NoError(t, err)

	// Close without connect should not error
	err = client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

func TestTableSchema(t *testing.T) {
	schema := &TableSchema{
		TableName:    "test_vectors",
		VectorColumn: "embedding",
		Dimension:    768,
		IDColumn:     "id",
		MetadataColumns: []ColumnDef{
			{Name: "content", Type: "TEXT", Nullable: true},
			{Name: "category", Type: "VARCHAR(100)", Nullable: false},
		},
	}

	assert.Equal(t, "test_vectors", schema.TableName)
	assert.Equal(t, "embedding", schema.VectorColumn)
	assert.Equal(t, 768, schema.Dimension)
	assert.Equal(t, "id", schema.IDColumn)
	assert.Len(t, schema.MetadataColumns, 2)
}

func TestVector(t *testing.T) {
	v := Vector{
		ID:     "test-id",
		Vector: []float32{0.1, 0.2, 0.3},
		Metadata: map[string]interface{}{
			"content": "test content",
			"count":   42,
		},
	}

	assert.Equal(t, "test-id", v.ID)
	assert.Len(t, v.Vector, 3)
	assert.Equal(t, "test content", v.Metadata["content"])
	assert.Equal(t, 42, v.Metadata["count"])
}

func TestSearchResult(t *testing.T) {
	result := SearchResult{
		ID:       "test-id",
		Distance: 0.15,
		Metadata: map[string]interface{}{
			"content": "test content",
		},
	}

	assert.Equal(t, "test-id", result.ID)
	assert.InDelta(t, 0.15, result.Distance, 0.001)
	assert.Equal(t, "test content", result.Metadata["content"])
}

func TestCreateIndexRequest(t *testing.T) {
	tests := []struct {
		name      string
		req       *CreateIndexRequest
		wantM     int
		wantLists int
	}{
		{
			name: "HNSW with defaults",
			req: &CreateIndexRequest{
				TableName:    "test",
				IndexName:    "test_idx",
				VectorColumn: "embedding",
				IndexType:    IndexTypeHNSW,
				Metric:       DistanceCosine,
			},
			wantM: 16,
		},
		{
			name: "HNSW with custom M",
			req: &CreateIndexRequest{
				TableName:    "test",
				IndexName:    "test_idx",
				VectorColumn: "embedding",
				IndexType:    IndexTypeHNSW,
				Metric:       DistanceCosine,
				M:            32,
			},
			wantM: 32,
		},
		{
			name: "IVFFlat with defaults",
			req: &CreateIndexRequest{
				TableName:    "test",
				IndexName:    "test_idx",
				VectorColumn: "embedding",
				IndexType:    IndexTypeIVFFlat,
				Metric:       DistanceL2,
			},
			wantLists: 100,
		},
		{
			name: "IVFFlat with custom lists",
			req: &CreateIndexRequest{
				TableName:    "test",
				IndexName:    "test_idx",
				VectorColumn: "embedding",
				IndexType:    IndexTypeIVFFlat,
				Metric:       DistanceL2,
				Lists:        200,
			},
			wantLists: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.req.IndexType == IndexTypeHNSW {
				m := tt.req.M
				if m == 0 {
					m = 16
				}
				assert.Equal(t, tt.wantM, m)
			} else {
				lists := tt.req.Lists
				if lists == 0 {
					lists = 100
				}
				assert.Equal(t, tt.wantLists, lists)
			}
		})
	}
}

func TestUpsertRequest_EmptyVectors(t *testing.T) {
	config := DefaultConfig()
	config.Host = "localhost" // Will fail to connect but that's OK for this test
	client, err := NewClient(config, logrus.New())
	require.NoError(t, err)

	// Manually set connected to test the empty vectors case
	client.connected = true
	client.pool = nil // Will cause panic if actually used

	// Empty vectors should return 0 without calling pool
	req := &UpsertRequest{
		TableName:    "test",
		VectorColumn: "embedding",
		IDColumn:     "id",
		Vectors:      []Vector{},
	}

	count, err := client.Upsert(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestDeleteRequest_Validation(t *testing.T) {
	config := DefaultConfig()
	client, err := NewClient(config, logrus.New())
	require.NoError(t, err)

	// Manually set connected
	client.connected = true
	client.pool = nil

	// Neither IDs nor filter - should error
	req := &DeleteRequest{
		TableName: "test",
		IDColumn:  "id",
		IDs:       nil,
		Filter:    "",
	}

	_, err = client.Delete(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either IDs or filter must be specified")
}

func TestGetEmptyIDs(t *testing.T) {
	config := DefaultConfig()
	client, err := NewClient(config, logrus.New())
	require.NoError(t, err)

	// Manually set connected
	client.connected = true
	client.pool = nil

	// Empty IDs should return empty result without calling pool
	results, err := client.Get(context.Background(), "test", "id", []string{}, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// Integration tests - only run when PostgreSQL with pgvector is available
func skipIfNoPostgres(t *testing.T) *Client {
	testutil.RequirePostgres(t)

	host := os.Getenv("PGVECTOR_HOST")
	if host == "" {
		host = os.Getenv("DB_HOST")
	}
	if host == "" {
		host = "localhost"
	}

	config := &Config{
		Host:           host,
		Port:           5432,
		User:           os.Getenv("PGVECTOR_USER"),
		Password:       os.Getenv("PGVECTOR_PASSWORD"),
		Database:       os.Getenv("PGVECTOR_DATABASE"),
		SSLMode:        "disable",
		ConnectTimeout: 10 * time.Second,
	}

	if config.User == "" {
		config.User = "postgres"
	}
	if config.Database == "" {
		config.Database = "postgres"
	}

	client, err := NewClient(config, logrus.New())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Skipf("Skipping integration test: cannot connect to PostgreSQL: %v (SKIP-OK: #integration-only)", err)
	}

	return client
}

func TestIntegration_CreateTableAndIndex(t *testing.T) {
	client := skipIfNoPostgres(t)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	tableName := "test_vectors_" + time.Now().Format("20060102150405")

	// Clean up at end
	defer func() { _ = client.DropTable(ctx, tableName) }()

	// Create table
	schema := &TableSchema{
		TableName:    tableName,
		VectorColumn: "embedding",
		Dimension:    384,
		IDColumn:     "id",
		MetadataColumns: []ColumnDef{
			{Name: "content", Type: "TEXT", Nullable: true},
			{Name: "category", Type: "VARCHAR(100)", Nullable: false},
		},
	}

	err := client.CreateTable(ctx, schema)
	require.NoError(t, err)

	// Check table exists
	exists, err := client.TableExists(ctx, tableName)
	require.NoError(t, err)
	assert.True(t, exists)

	// Create index
	err = client.CreateIndex(ctx, &CreateIndexRequest{
		TableName:      tableName,
		IndexName:      tableName + "_idx",
		VectorColumn:   "embedding",
		IndexType:      IndexTypeHNSW,
		Metric:         DistanceCosine,
		M:              16,
		EfConstruction: 64,
	})
	require.NoError(t, err)
}

func TestIntegration_UpsertSearchDelete(t *testing.T) {
	client := skipIfNoPostgres(t)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	tableName := "test_vectors_ops_" + time.Now().Format("20060102150405")

	// Create table
	schema := &TableSchema{
		TableName:    tableName,
		VectorColumn: "embedding",
		Dimension:    3,
		IDColumn:     "id",
		MetadataColumns: []ColumnDef{
			{Name: "content", Type: "TEXT", Nullable: true},
		},
	}

	err := client.CreateTable(ctx, schema)
	require.NoError(t, err)
	defer func() { _ = client.DropTable(ctx, tableName) }()

	// Upsert vectors
	vectors := []Vector{
		{ID: "id1", Vector: []float32{1.0, 0.0, 0.0}, Metadata: map[string]interface{}{"content": "first"}},
		{ID: "id2", Vector: []float32{0.0, 1.0, 0.0}, Metadata: map[string]interface{}{"content": "second"}},
		{ID: "id3", Vector: []float32{0.0, 0.0, 1.0}, Metadata: map[string]interface{}{"content": "third"}},
	}

	count, err := client.Upsert(ctx, &UpsertRequest{
		TableName:    tableName,
		VectorColumn: "embedding",
		IDColumn:     "id",
		Vectors:      vectors,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Check count
	totalCount, err := client.Count(ctx, tableName, "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), totalCount)

	// Get vectors
	results, err := client.Get(ctx, tableName, "id", []string{"id1", "id2"}, []string{"id", "content"})
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Search
	searchResults, err := client.Search(ctx, &SearchRequest{
		TableName:     tableName,
		VectorColumn:  "embedding",
		IDColumn:      "id",
		QueryVector:   []float32{1.0, 0.1, 0.0},
		Limit:         2,
		Metric:        DistanceCosine,
		OutputColumns: []string{"content"},
	})
	require.NoError(t, err)
	assert.Len(t, searchResults, 2)
	// First result should be id1 (closest to query)
	assert.Equal(t, "id1", searchResults[0].ID)

	// Delete
	deleted, err := client.Delete(ctx, &DeleteRequest{
		TableName: tableName,
		IDColumn:  "id",
		IDs:       []string{"id1"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Verify delete
	totalCount, err = client.Count(ctx, tableName, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), totalCount)
}
