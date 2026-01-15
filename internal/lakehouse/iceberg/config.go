package iceberg

import (
	"fmt"
	"time"
)

// Config holds Apache Iceberg catalog configuration
type Config struct {
	// Catalog settings
	CatalogType string `json:"catalog_type" yaml:"catalog_type"` // rest, hive, glue, jdbc
	CatalogURI  string `json:"catalog_uri" yaml:"catalog_uri"`
	Warehouse   string `json:"warehouse" yaml:"warehouse"`

	// S3 settings (for MinIO or AWS S3)
	S3Endpoint        string `json:"s3_endpoint" yaml:"s3_endpoint"`
	S3AccessKey       string `json:"s3_access_key" yaml:"s3_access_key"`
	S3SecretKey       string `json:"s3_secret_key" yaml:"s3_secret_key"`
	S3PathStyleAccess bool   `json:"s3_path_style_access" yaml:"s3_path_style_access"`
	S3Region          string `json:"s3_region" yaml:"s3_region"`

	// Connection options
	Timeout    time.Duration `json:"timeout" yaml:"timeout"`
	MaxRetries int           `json:"max_retries" yaml:"max_retries"`

	// Table defaults
	DefaultWriteFormat  string `json:"default_write_format" yaml:"default_write_format"`   // parquet, avro, orc
	DefaultCompression  string `json:"default_compression" yaml:"default_compression"`     // zstd, gzip, snappy, lz4
	TargetFileSizeBytes int64  `json:"target_file_size_bytes" yaml:"target_file_size_bytes"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		CatalogType:         "rest",
		CatalogURI:          "http://localhost:8181",
		Warehouse:           "s3://helixagent-iceberg/warehouse",
		S3Endpoint:          "http://localhost:9000",
		S3PathStyleAccess:   true,
		S3Region:            "us-east-1",
		Timeout:             30 * time.Second,
		MaxRetries:          3,
		DefaultWriteFormat:  "parquet",
		DefaultCompression:  "zstd",
		TargetFileSizeBytes: 134217728, // 128MB
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	validCatalogTypes := map[string]bool{"rest": true, "hive": true, "glue": true, "jdbc": true}
	if !validCatalogTypes[c.CatalogType] {
		return fmt.Errorf("invalid catalog_type: %s (must be rest, hive, glue, or jdbc)", c.CatalogType)
	}
	if c.CatalogURI == "" {
		return fmt.Errorf("catalog_uri is required")
	}
	if c.Warehouse == "" {
		return fmt.Errorf("warehouse is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	validFormats := map[string]bool{"parquet": true, "avro": true, "orc": true}
	if c.DefaultWriteFormat != "" && !validFormats[c.DefaultWriteFormat] {
		return fmt.Errorf("invalid default_write_format: %s", c.DefaultWriteFormat)
	}
	validCompressions := map[string]bool{"zstd": true, "gzip": true, "snappy": true, "lz4": true, "none": true}
	if c.DefaultCompression != "" && !validCompressions[c.DefaultCompression] {
		return fmt.Errorf("invalid default_compression: %s", c.DefaultCompression)
	}
	return nil
}

// NamespaceConfig represents an Iceberg namespace
type NamespaceConfig struct {
	Name       string            `json:"name" yaml:"name"`
	Properties map[string]string `json:"properties" yaml:"properties"`
}

// TableConfig holds configuration for an Iceberg table
type TableConfig struct {
	Namespace         string            `json:"namespace" yaml:"namespace"`
	Name              string            `json:"name" yaml:"name"`
	Schema            *Schema           `json:"schema" yaml:"schema"`
	PartitionSpec     []PartitionField  `json:"partition_spec" yaml:"partition_spec"`
	SortOrder         []SortField       `json:"sort_order" yaml:"sort_order"`
	WriteFormat       string            `json:"write_format" yaml:"write_format"`
	Compression       string            `json:"compression" yaml:"compression"`
	Properties        map[string]string `json:"properties" yaml:"properties"`
}

// DefaultTableConfig returns a TableConfig with defaults
func DefaultTableConfig(namespace, name string) *TableConfig {
	return &TableConfig{
		Namespace:     namespace,
		Name:          name,
		WriteFormat:   "parquet",
		Compression:   "zstd",
		Properties:    make(map[string]string),
	}
}

// FullName returns the full table identifier
func (tc *TableConfig) FullName() string {
	return fmt.Sprintf("%s.%s", tc.Namespace, tc.Name)
}

// WithSchema sets the table schema
func (tc *TableConfig) WithSchema(schema *Schema) *TableConfig {
	tc.Schema = schema
	return tc
}

// WithPartition adds a partition field
func (tc *TableConfig) WithPartition(field PartitionField) *TableConfig {
	tc.PartitionSpec = append(tc.PartitionSpec, field)
	return tc
}

// WithSortOrder adds a sort field
func (tc *TableConfig) WithSortOrder(field SortField) *TableConfig {
	tc.SortOrder = append(tc.SortOrder, field)
	return tc
}

// WithProperty sets a table property
func (tc *TableConfig) WithProperty(key, value string) *TableConfig {
	if tc.Properties == nil {
		tc.Properties = make(map[string]string)
	}
	tc.Properties[key] = value
	return tc
}

// Schema represents an Iceberg table schema
type Schema struct {
	SchemaID int     `json:"schema-id"`
	Fields   []Field `json:"fields"`
}

// Field represents a schema field
type Field struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // boolean, int, long, float, double, string, timestamp, date, binary, etc.
	Required bool   `json:"required"`
	Doc      string `json:"doc,omitempty"`
}

// NewSchema creates a new schema
func NewSchema() *Schema {
	return &Schema{
		SchemaID: 0,
		Fields:   []Field{},
	}
}

// AddField adds a field to the schema
func (s *Schema) AddField(id int, name, fieldType string, required bool) *Schema {
	s.Fields = append(s.Fields, Field{
		ID:       id,
		Name:     name,
		Type:     fieldType,
		Required: required,
	})
	return s
}

// AddFieldWithDoc adds a field with documentation
func (s *Schema) AddFieldWithDoc(id int, name, fieldType string, required bool, doc string) *Schema {
	s.Fields = append(s.Fields, Field{
		ID:       id,
		Name:     name,
		Type:     fieldType,
		Required: required,
		Doc:      doc,
	})
	return s
}

// PartitionTransform represents a partition transform type
type PartitionTransform string

const (
	TransformIdentity  PartitionTransform = "identity"
	TransformYear      PartitionTransform = "year"
	TransformMonth     PartitionTransform = "month"
	TransformDay       PartitionTransform = "day"
	TransformHour      PartitionTransform = "hour"
	TransformBucket    PartitionTransform = "bucket"
	TransformTruncate  PartitionTransform = "truncate"
)

// PartitionField represents a partition field
type PartitionField struct {
	SourceID  int                `json:"source-id"`
	FieldID   int                `json:"field-id"`
	Name      string             `json:"name"`
	Transform PartitionTransform `json:"transform"`
	Width     int                `json:"width,omitempty"` // for bucket and truncate
}

// NewPartitionField creates a new partition field
func NewPartitionField(sourceID int, name string, transform PartitionTransform) PartitionField {
	return PartitionField{
		SourceID:  sourceID,
		Name:      name,
		Transform: transform,
	}
}

// WithWidth sets the width for bucket/truncate transforms
func (pf PartitionField) WithWidth(width int) PartitionField {
	pf.Width = width
	return pf
}

// SortDirection represents sort direction
type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

// NullOrder represents null ordering
type NullOrder string

const (
	NullsFirst NullOrder = "nulls-first"
	NullsLast  NullOrder = "nulls-last"
)

// SortField represents a sort field
type SortField struct {
	SourceID   int           `json:"source-id"`
	Transform  string        `json:"transform"` // identity or other transforms
	Direction  SortDirection `json:"direction"`
	NullOrder  NullOrder     `json:"null-order"`
}

// NewSortField creates a new sort field
func NewSortField(sourceID int, direction SortDirection) SortField {
	nullOrder := NullsLast
	if direction == SortDesc {
		nullOrder = NullsFirst
	}
	return SortField{
		SourceID:  sourceID,
		Transform: "identity",
		Direction: direction,
		NullOrder: nullOrder,
	}
}

// WithNullOrder sets the null ordering
func (sf SortField) WithNullOrder(order NullOrder) SortField {
	sf.NullOrder = order
	return sf
}

// SnapshotConfig holds configuration for snapshot operations
type SnapshotConfig struct {
	OlderThanDays     int  `json:"older_than_days" yaml:"older_than_days"`
	RetainLast        int  `json:"retain_last" yaml:"retain_last"`
	MaxConcurrentOps  int  `json:"max_concurrent_ops" yaml:"max_concurrent_ops"`
	DryRun            bool `json:"dry_run" yaml:"dry_run"`
}

// DefaultSnapshotConfig returns default snapshot configuration
func DefaultSnapshotConfig() *SnapshotConfig {
	return &SnapshotConfig{
		OlderThanDays:    7,
		RetainLast:       10,
		MaxConcurrentOps: 4,
		DryRun:           false,
	}
}

// CompactionConfig holds configuration for data compaction
type CompactionConfig struct {
	TargetFileSizeBytes  int64  `json:"target_file_size_bytes" yaml:"target_file_size_bytes"`
	MinInputFiles        int    `json:"min_input_files" yaml:"min_input_files"`
	MaxConcurrentOps     int    `json:"max_concurrent_ops" yaml:"max_concurrent_ops"`
	PartialProgressEnabled bool `json:"partial_progress_enabled" yaml:"partial_progress_enabled"`
}

// DefaultCompactionConfig returns default compaction configuration
func DefaultCompactionConfig() *CompactionConfig {
	return &CompactionConfig{
		TargetFileSizeBytes:    134217728, // 128MB
		MinInputFiles:          5,
		MaxConcurrentOps:       4,
		PartialProgressEnabled: true,
	}
}
