package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockMongoDBClient implements MongoDBClient for testing
type MockMongoDBClient struct {
	databases   []string
	collections []string
	documents   []map[string]interface{}
	shouldError bool
}

func NewMockMongoDBClient() *MockMongoDBClient {
	return &MockMongoDBClient{
		databases:   []string{"admin", "local", "test", "myapp"},
		collections: []string{"users", "products", "orders"},
		documents: []map[string]interface{}{
			{"_id": "1", "name": "Alice", "email": "alice@example.com"},
			{"_id": "2", "name": "Bob", "email": "bob@example.com"},
		},
	}
}

func (m *MockMongoDBClient) SetError(shouldError bool) {
	m.shouldError = shouldError
}

func (m *MockMongoDBClient) ListDatabases(ctx context.Context) ([]string, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.databases, nil
}

func (m *MockMongoDBClient) ListCollections(ctx context.Context, database string) ([]string, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.collections, nil
}

func (m *MockMongoDBClient) Find(ctx context.Context, database, collection string, filter map[string]interface{}, options FindOptions) ([]map[string]interface{}, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.documents, nil
}

func (m *MockMongoDBClient) FindOne(ctx context.Context, database, collection string, filter map[string]interface{}) (map[string]interface{}, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	if len(m.documents) > 0 {
		return m.documents[0], nil
	}
	return nil, assert.AnError
}

func (m *MockMongoDBClient) InsertOne(ctx context.Context, database, collection string, document map[string]interface{}) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return "inserted-id-123", nil
}

func (m *MockMongoDBClient) InsertMany(ctx context.Context, database, collection string, documents []map[string]interface{}) ([]string, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	ids := make([]string, len(documents))
	for i := range documents {
		ids[i] = "id-" + string(rune('a'+i))
	}
	return ids, nil
}

func (m *MockMongoDBClient) UpdateOne(ctx context.Context, database, collection string, filter, update map[string]interface{}) (int64, error) {
	if m.shouldError {
		return 0, assert.AnError
	}
	return 1, nil
}

func (m *MockMongoDBClient) UpdateMany(ctx context.Context, database, collection string, filter, update map[string]interface{}) (int64, error) {
	if m.shouldError {
		return 0, assert.AnError
	}
	return 5, nil
}

func (m *MockMongoDBClient) DeleteOne(ctx context.Context, database, collection string, filter map[string]interface{}) (int64, error) {
	if m.shouldError {
		return 0, assert.AnError
	}
	return 1, nil
}

func (m *MockMongoDBClient) DeleteMany(ctx context.Context, database, collection string, filter map[string]interface{}) (int64, error) {
	if m.shouldError {
		return 0, assert.AnError
	}
	return 10, nil
}

func (m *MockMongoDBClient) Aggregate(ctx context.Context, database, collection string, pipeline []map[string]interface{}) ([]map[string]interface{}, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return []map[string]interface{}{
		{"_id": "group1", "count": 10},
		{"_id": "group2", "count": 20},
	}, nil
}

func (m *MockMongoDBClient) CreateIndex(ctx context.Context, database, collection string, keys map[string]interface{}, options IndexOptions) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return "index_name_1", nil
}

func (m *MockMongoDBClient) DropIndex(ctx context.Context, database, collection, indexName string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockMongoDBClient) Count(ctx context.Context, database, collection string, filter map[string]interface{}) (int64, error) {
	if m.shouldError {
		return 0, assert.AnError
	}
	return 100, nil
}

// Tests

func TestDefaultMongoDBConfig(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()

	assert.Equal(t, "mongodb://localhost:27017", config.URI)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 10, config.MaxPoolSize)
	assert.False(t, config.EnableTLS)
}

func TestNewMongoDBAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	assert.NotNil(t, adapter)

	info := adapter.GetServerInfo()
	assert.Equal(t, "mongodb", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
}

func TestMongoDBAdapter_ListTools(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "mongodb_list_databases")
	assert.Contains(t, toolNames, "mongodb_find")
	assert.Contains(t, toolNames, "mongodb_insert_one")
}

func TestMongoDBAdapter_ListDatabases(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_list_databases", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestMongoDBAdapter_ListCollections(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_list_collections", map[string]interface{}{
		"database": "myapp",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMongoDBAdapter_Find(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_find", map[string]interface{}{
		"database":   "myapp",
		"collection": "users",
		"filter":     map[string]interface{}{"status": "active"},
		"limit":      10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMongoDBAdapter_FindOne(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_find_one", map[string]interface{}{
		"database":   "myapp",
		"collection": "users",
		"filter":     map[string]interface{}{"_id": "1"},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMongoDBAdapter_InsertOne(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_insert_one", map[string]interface{}{
		"database":   "myapp",
		"collection": "users",
		"document":   map[string]interface{}{"name": "Charlie", "email": "charlie@example.com"},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMongoDBAdapter_InsertMany(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_insert_many", map[string]interface{}{
		"database":   "myapp",
		"collection": "users",
		"documents": []interface{}{
			map[string]interface{}{"name": "Dave"},
			map[string]interface{}{"name": "Eve"},
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMongoDBAdapter_UpdateOne(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_update_one", map[string]interface{}{
		"database":   "myapp",
		"collection": "users",
		"filter":     map[string]interface{}{"_id": "1"},
		"update":     map[string]interface{}{"$set": map[string]interface{}{"status": "inactive"}},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMongoDBAdapter_DeleteOne(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_delete_one", map[string]interface{}{
		"database":   "myapp",
		"collection": "users",
		"filter":     map[string]interface{}{"_id": "1"},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMongoDBAdapter_Aggregate(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_aggregate", map[string]interface{}{
		"database":   "myapp",
		"collection": "orders",
		"pipeline": []interface{}{
			map[string]interface{}{"$match": map[string]interface{}{"status": "completed"}},
			map[string]interface{}{"$group": map[string]interface{}{"_id": "$customer", "total": map[string]interface{}{"$sum": "$amount"}}},
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMongoDBAdapter_Count(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_count", map[string]interface{}{
		"database":   "myapp",
		"collection": "users",
		"filter":     map[string]interface{}{},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMongoDBAdapter_CreateIndex(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_create_index", map[string]interface{}{
		"database":   "myapp",
		"collection": "users",
		"keys":       map[string]interface{}{"email": 1},
		"unique":     true,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMongoDBAdapter_InvalidTool(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	_, err := adapter.CallTool(ctx, "invalid_tool", map[string]interface{}{})

	assert.Error(t, err)
}

func TestMongoDBAdapter_ErrorHandling(t *testing.T) {
	t.Parallel()
	config := DefaultMongoDBConfig()
	client := NewMockMongoDBClient()
	client.SetError(true)
	adapter := NewMongoDBAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "mongodb_list_databases", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Type tests

func TestFindOptionsTypes(t *testing.T) {
	t.Parallel()
	options := FindOptions{
		Limit:      100,
		Skip:       10,
		Sort:       map[string]interface{}{"created_at": -1},
		Projection: map[string]interface{}{"password": 0},
	}

	assert.Equal(t, int64(100), options.Limit)
	assert.Equal(t, int64(10), options.Skip)
	assert.NotNil(t, options.Sort)
	assert.NotNil(t, options.Projection)
}

func TestIndexOptionsTypes(t *testing.T) {
	t.Parallel()
	options := IndexOptions{
		Unique:     true,
		Background: true,
		Name:       "email_unique",
	}

	assert.True(t, options.Unique)
	assert.True(t, options.Background)
	assert.Equal(t, "email_unique", options.Name)
}

func TestMongoDBConfigTypes(t *testing.T) {
	t.Parallel()
	config := MongoDBConfig{
		URI:         "mongodb://user:pass@localhost:27017",
		Database:    "production",
		Timeout:     60 * time.Second,
		MaxPoolSize: 50,
		EnableTLS:   true,
	}

	assert.Contains(t, config.URI, "mongodb://")
	assert.Equal(t, "production", config.Database)
	assert.Equal(t, 50, config.MaxPoolSize)
	assert.True(t, config.EnableTLS)
}
