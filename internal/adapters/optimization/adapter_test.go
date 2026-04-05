package optimization_test

import (
	"testing"
	"time"

	adapter "dev.helix.agent/internal/adapters/optimization"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// StreamBufferAdapter Tests
// ============================================================================

func TestNewStreamBufferAdapter(t *testing.T) {
	t.Parallel()
	tests := []string{"word", "sentence", "line", "size", "unknown"}
	for _, strategy := range tests {
		t.Run(strategy, func(t *testing.T) {
				t.Parallel()
			buf := adapter.NewStreamBufferAdapter(strategy, 100)
			require.NotNil(t, buf)
		})
	}
}

func TestStreamBufferAdapter_Add_Word(t *testing.T) {
	t.Parallel()
	buf := adapter.NewStreamBufferAdapter("word", 0)
	flushed := buf.Add("hello ")
	assert.NotNil(t, flushed)
}

func TestStreamBufferAdapter_Add_Sentence(t *testing.T) {
	t.Parallel()
	buf := adapter.NewStreamBufferAdapter("sentence", 0)
	_ = buf.Add("hello world. ")
	_ = buf.Add("this is a test. ")
}

func TestStreamBufferAdapter_Flush(t *testing.T) {
	t.Parallel()
	buf := adapter.NewStreamBufferAdapter("word", 0)
	_ = buf.Add("hello")
	remaining := buf.Flush()
	assert.NotNil(t, remaining)
}

func TestStreamBufferAdapter_Reset(t *testing.T) {
	t.Parallel()
	buf := adapter.NewStreamBufferAdapter("word", 0)
	_ = buf.Add("some text")
	buf.Reset()
	// After reset, flush should return empty
	remaining := buf.Flush()
	assert.Empty(t, remaining)
}

// ============================================================================
// Schema adapter tests
// ============================================================================

func TestNewSchemaAdapter(t *testing.T) {
	t.Parallel()
	sa := adapter.NewSchemaAdapter()
	require.NotNil(t, sa)
}

func TestStringSchema(t *testing.T) {
	t.Parallel()
	schema := adapter.StringSchema()
	require.NotNil(t, schema)
	assert.Equal(t, "string", schema.Type)
}

func TestIntegerSchema(t *testing.T) {
	t.Parallel()
	schema := adapter.IntegerSchema()
	require.NotNil(t, schema)
	assert.Equal(t, "integer", schema.Type)
}

func TestNumberSchema(t *testing.T) {
	t.Parallel()
	schema := adapter.NumberSchema()
	require.NotNil(t, schema)
	assert.Equal(t, "number", schema.Type)
}

func TestBooleanSchema(t *testing.T) {
	t.Parallel()
	schema := adapter.BooleanSchema()
	require.NotNil(t, schema)
	assert.Equal(t, "boolean", schema.Type)
}

func TestArraySchema(t *testing.T) {
	t.Parallel()
	items := adapter.StringSchema()
	schema := adapter.ArraySchema(items)
	require.NotNil(t, schema)
	assert.Equal(t, "array", schema.Type)
}

func TestObjectSchema(t *testing.T) {
	t.Parallel()
	props := map[string]*adapter.Schema{
		"name": adapter.StringSchema(),
		"age":  adapter.IntegerSchema(),
	}
	schema := adapter.ObjectSchema(props, "name")
	require.NotNil(t, schema)
	assert.Equal(t, "object", schema.Type)
}

func TestNewSchemaBuilder(t *testing.T) {
	t.Parallel()
	builder := adapter.NewSchemaBuilder()
	require.NotNil(t, builder)
}

// ============================================================================
// ToHelixOptimizedRequest tests
// ============================================================================

func TestToHelixOptimizedRequest_NilCacheHit(t *testing.T) {
	t.Parallel()
	result := adapter.ToHelixOptimizedRequest(nil, "my prompt")
	require.NotNil(t, result)
	assert.Equal(t, "my prompt", result.OriginalPrompt)
	assert.Equal(t, "my prompt", result.OptimizedPrompt)
	assert.False(t, result.CacheHit)
	assert.Empty(t, result.CachedResponse)
}

func TestToHelixOptimizedRequest_WithCacheHit(t *testing.T) {
	t.Parallel()
	hit := &adapter.CacheHit{
		Response:   "cached response",
		Similarity: 0.95,
		Metadata:   map[string]interface{}{"source": "test"},
		CachedAt:   time.Now(),
	}

	result := adapter.ToHelixOptimizedRequest(hit, "my prompt")
	require.NotNil(t, result)
	assert.Equal(t, "my prompt", result.OriginalPrompt)
	assert.True(t, result.CacheHit)
	assert.Equal(t, "cached response", result.CachedResponse)
}

// ============================================================================
// CacheHit struct test
// ============================================================================

func TestCacheHit_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	hit := &adapter.CacheHit{
		Response:   "hello",
		Similarity: 0.9,
		Metadata:   map[string]interface{}{"key": "val"},
		CachedAt:   now,
	}

	assert.Equal(t, "hello", hit.Response)
	assert.Equal(t, 0.9, hit.Similarity)
	assert.Equal(t, "val", hit.Metadata["key"])
	assert.Equal(t, now, hit.CachedAt)
}
