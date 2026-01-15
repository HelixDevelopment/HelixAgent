package iceberg

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "rest", config.CatalogType)
	assert.Equal(t, "http://localhost:8181", config.CatalogURI)
	assert.Equal(t, "s3://helixagent-iceberg/warehouse", config.Warehouse)
	assert.Equal(t, "http://localhost:9000", config.S3Endpoint)
	assert.True(t, config.S3PathStyleAccess)
	assert.Equal(t, "us-east-1", config.S3Region)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, "parquet", config.DefaultWriteFormat)
	assert.Equal(t, "zstd", config.DefaultCompression)
	assert.Equal(t, int64(134217728), config.TargetFileSizeBytes) // 128MB
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default config",
			modify:      func(c *Config) {},
			expectError: false,
		},
		{
			name: "invalid catalog type",
			modify: func(c *Config) {
				c.CatalogType = "invalid"
			},
			expectError: true,
			errorMsg:    "invalid catalog_type",
		},
		{
			name: "empty catalog URI",
			modify: func(c *Config) {
				c.CatalogURI = ""
			},
			expectError: true,
			errorMsg:    "catalog_uri is required",
		},
		{
			name: "empty warehouse",
			modify: func(c *Config) {
				c.Warehouse = ""
			},
			expectError: true,
			errorMsg:    "warehouse is required",
		},
		{
			name: "invalid timeout",
			modify: func(c *Config) {
				c.Timeout = 0
			},
			expectError: true,
			errorMsg:    "timeout must be positive",
		},
		{
			name: "invalid write format",
			modify: func(c *Config) {
				c.DefaultWriteFormat = "invalid"
			},
			expectError: true,
			errorMsg:    "invalid default_write_format",
		},
		{
			name: "invalid compression",
			modify: func(c *Config) {
				c.DefaultCompression = "invalid"
			},
			expectError: true,
			errorMsg:    "invalid default_compression",
		},
		{
			name: "valid hive catalog",
			modify: func(c *Config) {
				c.CatalogType = "hive"
			},
			expectError: false,
		},
		{
			name: "valid glue catalog",
			modify: func(c *Config) {
				c.CatalogType = "glue"
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.modify(config)

			err := config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultTableConfig(t *testing.T) {
	config := DefaultTableConfig("helixagent", "debates")

	assert.Equal(t, "helixagent", config.Namespace)
	assert.Equal(t, "debates", config.Name)
	assert.Equal(t, "parquet", config.WriteFormat)
	assert.Equal(t, "zstd", config.Compression)
	assert.NotNil(t, config.Properties)
}

func TestTableConfigFullName(t *testing.T) {
	config := DefaultTableConfig("helixagent", "debates")
	assert.Equal(t, "helixagent.debates", config.FullName())
}

func TestTableConfigChaining(t *testing.T) {
	schema := NewSchema().
		AddField(1, "id", "string", true).
		AddField(2, "name", "string", true).
		AddField(3, "created_at", "timestamp", true)

	config := DefaultTableConfig("helixagent", "debates").
		WithSchema(schema).
		WithPartition(NewPartitionField(3, "created_at_day", TransformDay)).
		WithSortOrder(NewSortField(1, SortAsc)).
		WithProperty("write.format.default", "parquet")

	assert.Equal(t, "helixagent", config.Namespace)
	assert.Equal(t, "debates", config.Name)
	assert.NotNil(t, config.Schema)
	assert.Len(t, config.Schema.Fields, 3)
	assert.Len(t, config.PartitionSpec, 1)
	assert.Len(t, config.SortOrder, 1)
	assert.Equal(t, "parquet", config.Properties["write.format.default"])
}

func TestNewSchema(t *testing.T) {
	schema := NewSchema()

	assert.NotNil(t, schema)
	assert.Equal(t, 0, schema.SchemaID)
	assert.Empty(t, schema.Fields)
}

func TestSchemaAddField(t *testing.T) {
	schema := NewSchema().
		AddField(1, "id", "string", true).
		AddField(2, "value", "double", false).
		AddFieldWithDoc(3, "timestamp", "timestamp", true, "Event timestamp")

	assert.Len(t, schema.Fields, 3)

	assert.Equal(t, 1, schema.Fields[0].ID)
	assert.Equal(t, "id", schema.Fields[0].Name)
	assert.Equal(t, "string", schema.Fields[0].Type)
	assert.True(t, schema.Fields[0].Required)

	assert.Equal(t, 2, schema.Fields[1].ID)
	assert.Equal(t, "value", schema.Fields[1].Name)
	assert.Equal(t, "double", schema.Fields[1].Type)
	assert.False(t, schema.Fields[1].Required)

	assert.Equal(t, 3, schema.Fields[2].ID)
	assert.Equal(t, "timestamp", schema.Fields[2].Name)
	assert.Equal(t, "Event timestamp", schema.Fields[2].Doc)
}

func TestNewPartitionField(t *testing.T) {
	field := NewPartitionField(3, "created_at_day", TransformDay)

	assert.Equal(t, 3, field.SourceID)
	assert.Equal(t, "created_at_day", field.Name)
	assert.Equal(t, TransformDay, field.Transform)
	assert.Equal(t, 0, field.Width)
}

func TestPartitionFieldWithWidth(t *testing.T) {
	field := NewPartitionField(1, "bucket_field", TransformBucket).WithWidth(16)

	assert.Equal(t, TransformBucket, field.Transform)
	assert.Equal(t, 16, field.Width)
}

func TestNewSortField(t *testing.T) {
	ascField := NewSortField(1, SortAsc)
	assert.Equal(t, 1, ascField.SourceID)
	assert.Equal(t, SortAsc, ascField.Direction)
	assert.Equal(t, NullsLast, ascField.NullOrder)
	assert.Equal(t, "identity", ascField.Transform)

	descField := NewSortField(2, SortDesc)
	assert.Equal(t, SortDesc, descField.Direction)
	assert.Equal(t, NullsFirst, descField.NullOrder)
}

func TestSortFieldWithNullOrder(t *testing.T) {
	field := NewSortField(1, SortAsc).WithNullOrder(NullsFirst)
	assert.Equal(t, NullsFirst, field.NullOrder)
}

func TestDefaultSnapshotConfig(t *testing.T) {
	config := DefaultSnapshotConfig()

	assert.Equal(t, 7, config.OlderThanDays)
	assert.Equal(t, 10, config.RetainLast)
	assert.Equal(t, 4, config.MaxConcurrentOps)
	assert.False(t, config.DryRun)
}

func TestDefaultCompactionConfig(t *testing.T) {
	config := DefaultCompactionConfig()

	assert.Equal(t, int64(134217728), config.TargetFileSizeBytes) // 128MB
	assert.Equal(t, 5, config.MinInputFiles)
	assert.Equal(t, 4, config.MaxConcurrentOps)
	assert.True(t, config.PartialProgressEnabled)
}
