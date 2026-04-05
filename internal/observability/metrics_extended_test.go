package observability

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// MCPMetrics Tests
// =============================================================================

func TestNewMCPMetrics(t *testing.T) {
	t.Parallel()
	m, err := NewMCPMetrics("test-service")
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.NotNil(t, m.ToolCallsTotal)
	assert.NotNil(t, m.ToolDuration)
	assert.NotNil(t, m.ToolErrorsTotal)
	assert.NotNil(t, m.ToolsInFlight)
	assert.NotNil(t, m.ToolResultSize)
	assert.NotNil(t, m.ServerConnections)
}

func TestMCPMetrics_RecordToolCall_Success(t *testing.T) {
	t.Parallel()
	m, err := NewMCPMetrics("test-mcp")
	require.NoError(t, err)

	ctx := context.Background()
	// Should not panic
	m.RecordToolCall(ctx, "read_file", "filesystem", 0.5, 1024, nil)
}

func TestMCPMetrics_RecordToolCall_WithError(t *testing.T) {
	t.Parallel()
	m, err := NewMCPMetrics("test-mcp")
	require.NoError(t, err)

	ctx := context.Background()
	testErr := errors.New("tool execution failed")
	// Should not panic and should record error metrics
	m.RecordToolCall(ctx, "write_file", "filesystem", 1.2, 0, testErr)
}

func TestMCPMetrics_RecordToolCall_ZeroResultSize(t *testing.T) {
	t.Parallel()
	m, err := NewMCPMetrics("test-mcp")
	require.NoError(t, err)

	ctx := context.Background()
	// Zero result size should skip recording result size metric
	m.RecordToolCall(ctx, "delete_file", "filesystem", 0.1, 0, nil)
}

func TestMCPMetrics_StartToolCall(t *testing.T) {
	t.Parallel()
	m, err := NewMCPMetrics("test-mcp")
	require.NoError(t, err)

	ctx := context.Background()
	endFunc := m.StartToolCall(ctx, "search", "provider1")
	assert.NotNil(t, endFunc)

	// Call end function
	endFunc(ctx, 512, nil)
}

func TestMCPMetrics_RecordToolCall_Concurrent(t *testing.T) {
	t.Parallel()
	m, err := NewMCPMetrics("test-mcp-concurrent")
	require.NoError(t, err)

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var toolErr error
			if idx%5 == 0 {
				toolErr = errors.New("intermittent error")
			}
			m.RecordToolCall(ctx, "tool", "provider", float64(idx)*0.1, int64(idx*100), toolErr)
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// EmbeddingMetrics Tests
// =============================================================================

func TestNewEmbeddingMetrics(t *testing.T) {
	t.Parallel()
	m, err := NewEmbeddingMetrics("test-service")
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.NotNil(t, m.RequestsTotal)
	assert.NotNil(t, m.LatencySeconds)
	assert.NotNil(t, m.TokensTotal)
	assert.NotNil(t, m.EmbeddingSize)
	assert.NotNil(t, m.RequestsInFlight)
	assert.NotNil(t, m.ErrorsTotal)
	assert.NotNil(t, m.BatchSize)
}

func TestEmbeddingMetrics_RecordEmbedding_Success(t *testing.T) {
	t.Parallel()
	m, err := NewEmbeddingMetrics("test-embedding")
	require.NoError(t, err)

	ctx := context.Background()
	m.RecordEmbedding(ctx, "openai", 150, 1536, 0.3, 5, nil)
}

func TestEmbeddingMetrics_RecordEmbedding_WithError(t *testing.T) {
	t.Parallel()
	m, err := NewEmbeddingMetrics("test-embedding")
	require.NoError(t, err)

	ctx := context.Background()
	testErr := errors.New("rate limit exceeded")
	m.RecordEmbedding(ctx, "cohere", 0, 0, 1.5, 0, testErr)
}

func TestEmbeddingMetrics_RecordEmbedding_ZeroBatchSize(t *testing.T) {
	t.Parallel()
	m, err := NewEmbeddingMetrics("test-embedding")
	require.NoError(t, err)

	ctx := context.Background()
	// Zero batch size should skip recording batch size
	m.RecordEmbedding(ctx, "voyage", 100, 1024, 0.2, 0, nil)
}

func TestEmbeddingMetrics_RecordEmbedding_Concurrent(t *testing.T) {
	t.Parallel()
	m, err := NewEmbeddingMetrics("test-embedding-concurrent")
	require.NoError(t, err)

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m.RecordEmbedding(ctx, "openai", idx*10, 1536, float64(idx)*0.01, idx, nil)
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// VectorDBMetrics Tests
// =============================================================================

func TestNewVectorDBMetrics(t *testing.T) {
	t.Parallel()
	m, err := NewVectorDBMetrics("test-service")
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.NotNil(t, m.OperationsTotal)
	assert.NotNil(t, m.LatencySeconds)
	assert.NotNil(t, m.VectorsTotal)
	assert.NotNil(t, m.CollectionSize)
	assert.NotNil(t, m.OperationsInFlight)
	assert.NotNil(t, m.ErrorsTotal)
	assert.NotNil(t, m.QueryResultCount)
}

func TestVectorDBMetrics_RecordOperation_Success(t *testing.T) {
	t.Parallel()
	m, err := NewVectorDBMetrics("test-vectordb")
	require.NoError(t, err)

	ctx := context.Background()
	m.RecordOperation(ctx, "qdrant", "search", 0.05, 10, nil)
}

func TestVectorDBMetrics_RecordOperation_WithError(t *testing.T) {
	t.Parallel()
	m, err := NewVectorDBMetrics("test-vectordb")
	require.NoError(t, err)

	ctx := context.Background()
	testErr := errors.New("connection refused")
	m.RecordOperation(ctx, "pinecone", "upsert", 2.0, 0, testErr)
}

func TestVectorDBMetrics_RecordOperation_ZeroResults(t *testing.T) {
	t.Parallel()
	m, err := NewVectorDBMetrics("test-vectordb")
	require.NoError(t, err)

	ctx := context.Background()
	// Zero result count should skip recording query result count
	m.RecordOperation(ctx, "milvus", "delete", 0.1, 0, nil)
}

func TestVectorDBMetrics_RecordCollectionStats(t *testing.T) {
	t.Parallel()
	m, err := NewVectorDBMetrics("test-vectordb")
	require.NoError(t, err)

	ctx := context.Background()
	m.RecordCollectionStats(ctx, "qdrant", "documents", 10000, 52428800)
}

func TestVectorDBMetrics_Concurrent(t *testing.T) {
	t.Parallel()
	m, err := NewVectorDBMetrics("test-vectordb-concurrent")
	require.NoError(t, err)

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m.RecordOperation(ctx, "qdrant", "search", float64(idx)*0.01, idx, nil)
			m.RecordCollectionStats(ctx, "qdrant", "test", int64(idx*100), int64(idx*1024))
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// MemoryMetrics Tests
// =============================================================================

func TestNewMemoryMetrics(t *testing.T) {
	t.Parallel()
	m, err := NewMemoryMetrics("test-service")
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.NotNil(t, m.OperationsTotal)
	assert.NotNil(t, m.SearchLatency)
	assert.NotNil(t, m.EntriesTotal)
	assert.NotNil(t, m.OperationsInFlight)
	assert.NotNil(t, m.ErrorsTotal)
	assert.NotNil(t, m.CacheHits)
	assert.NotNil(t, m.CacheMisses)
}

func TestMemoryMetrics_RecordOperation_Success(t *testing.T) {
	t.Parallel()
	m, err := NewMemoryMetrics("test-memory")
	require.NoError(t, err)

	ctx := context.Background()
	m.RecordOperation(ctx, "search", "semantic", 0.15, nil)
}

func TestMemoryMetrics_RecordOperation_WithError(t *testing.T) {
	t.Parallel()
	m, err := NewMemoryMetrics("test-memory")
	require.NoError(t, err)

	ctx := context.Background()
	testErr := errors.New("memory store unavailable")
	m.RecordOperation(ctx, "store", "entity", 0.5, testErr)
}

func TestMemoryMetrics_RecordEntries(t *testing.T) {
	t.Parallel()
	m, err := NewMemoryMetrics("test-memory")
	require.NoError(t, err)

	ctx := context.Background()
	m.RecordEntries(ctx, "semantic", 500)
}

func TestMemoryMetrics_RecordCacheHit(t *testing.T) {
	t.Parallel()
	m, err := NewMemoryMetrics("test-memory")
	require.NoError(t, err)

	ctx := context.Background()
	m.RecordCacheHit(ctx)
}

func TestMemoryMetrics_RecordCacheMiss(t *testing.T) {
	t.Parallel()
	m, err := NewMemoryMetrics("test-memory")
	require.NoError(t, err)

	ctx := context.Background()
	m.RecordCacheMiss(ctx)
}

func TestMemoryMetrics_Concurrent(t *testing.T) {
	t.Parallel()
	m, err := NewMemoryMetrics("test-memory-concurrent")
	require.NoError(t, err)

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m.RecordOperation(ctx, "search", "semantic", float64(idx)*0.01, nil)
			if idx%2 == 0 {
				m.RecordCacheHit(ctx)
			} else {
				m.RecordCacheMiss(ctx)
			}
			m.RecordEntries(ctx, "semantic", int64(idx))
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// StreamingMetrics Tests
// =============================================================================

func TestNewStreamingMetrics(t *testing.T) {
	t.Parallel()
	m, err := NewStreamingMetrics("test-service")
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.NotNil(t, m.ChunksTotal)
	assert.NotNil(t, m.ErrorsTotal)
	assert.NotNil(t, m.Duration)
	assert.NotNil(t, m.StreamsInFlight)
	assert.NotNil(t, m.ChunkSize)
	assert.NotNil(t, m.ThroughputBytes)
}

func TestStreamingMetrics_RecordChunk(t *testing.T) {
	t.Parallel()
	m, err := NewStreamingMetrics("test-streaming")
	require.NoError(t, err)

	ctx := context.Background()
	m.RecordChunk(ctx, "openai", 256)
}

func TestStreamingMetrics_RecordStream_Success(t *testing.T) {
	t.Parallel()
	m, err := NewStreamingMetrics("test-streaming")
	require.NoError(t, err)

	ctx := context.Background()
	m.RecordStream(ctx, "anthropic", 5.2, nil)
}

func TestStreamingMetrics_RecordStream_WithError(t *testing.T) {
	t.Parallel()
	m, err := NewStreamingMetrics("test-streaming")
	require.NoError(t, err)

	ctx := context.Background()
	testErr := errors.New("stream interrupted")
	m.RecordStream(ctx, "openai", 1.5, testErr)
}

func TestStreamingMetrics_StartAndEndStream(t *testing.T) {
	t.Parallel()
	m, err := NewStreamingMetrics("test-streaming")
	require.NoError(t, err)

	ctx := context.Background()
	m.StartStream(ctx, "gemini")
	m.EndStream(ctx, "gemini")
}

func TestStreamingMetrics_Concurrent(t *testing.T) {
	t.Parallel()
	m, err := NewStreamingMetrics("test-streaming-concurrent")
	require.NoError(t, err)

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m.StartStream(ctx, "provider")
			m.RecordChunk(ctx, "provider", int64(idx*64))
			m.RecordStream(ctx, "provider", float64(idx)*0.1, nil)
			m.EndStream(ctx, "provider")
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// ProtocolMetrics Tests
// =============================================================================

func TestNewProtocolMetrics(t *testing.T) {
	t.Parallel()
	m, err := NewProtocolMetrics("test-service")
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.NotNil(t, m.RequestsTotal)
	assert.NotNil(t, m.ErrorsTotal)
	assert.NotNil(t, m.LatencySeconds)
	assert.NotNil(t, m.RequestsInFlight)
}

func TestProtocolMetrics_RecordRequest_Success(t *testing.T) {
	t.Parallel()
	m, err := NewProtocolMetrics("test-protocol")
	require.NoError(t, err)

	ctx := context.Background()
	m.RecordRequest(ctx, "mcp", "tool_call", 0.05, nil)
}

func TestProtocolMetrics_RecordRequest_WithError(t *testing.T) {
	t.Parallel()
	m, err := NewProtocolMetrics("test-protocol")
	require.NoError(t, err)

	ctx := context.Background()
	testErr := errors.New("protocol version mismatch")
	m.RecordRequest(ctx, "lsp", "diagnostics", 1.0, testErr)
}

func TestProtocolMetrics_RecordRequest_Concurrent(t *testing.T) {
	t.Parallel()
	m, err := NewProtocolMetrics("test-protocol-concurrent")
	require.NoError(t, err)

	ctx := context.Background()
	var wg sync.WaitGroup

	protocols := []string{"mcp", "lsp", "acp", "grpc"}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			proto := protocols[idx%len(protocols)]
			m.RecordRequest(ctx, proto, "operation", float64(idx)*0.01, nil)
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// Global Getter Tests
// =============================================================================

func TestGetMCPMetrics(t *testing.T) {
	t.Parallel()
	m := GetMCPMetrics()
	assert.NotNil(t, m)
}

func TestGetEmbeddingMetrics(t *testing.T) {
	t.Parallel()
	m := GetEmbeddingMetrics()
	assert.NotNil(t, m)
}

func TestGetVectorDBMetrics(t *testing.T) {
	t.Parallel()
	m := GetVectorDBMetrics()
	assert.NotNil(t, m)
}

func TestGetMemoryMetrics(t *testing.T) {
	t.Parallel()
	m := GetMemoryMetrics()
	assert.NotNil(t, m)
}

func TestGetStreamingMetrics(t *testing.T) {
	t.Parallel()
	m := GetStreamingMetrics()
	assert.NotNil(t, m)
}

func TestGetProtocolMetrics(t *testing.T) {
	t.Parallel()
	m := GetProtocolMetrics()
	assert.NotNil(t, m)
}

// =============================================================================
// errorType Tests
// =============================================================================

func TestErrorType_Nil(t *testing.T) {
	t.Parallel()
	result := errorType(nil)
	assert.Equal(t, "", result)
}

func TestErrorType_WithError(t *testing.T) {
	t.Parallel()
	err := errors.New("test error message")
	result := errorType(err)
	assert.Equal(t, "test error message", result)
}

// =============================================================================
// EndToolCallFunc Type Tests
// =============================================================================

func TestEndToolCallFunc_Type(t *testing.T) {
	t.Parallel()
	m, err := NewMCPMetrics("test-end-func")
	require.NoError(t, err)

	ctx := context.Background()
	endFunc := m.StartToolCall(ctx, "tool", "provider")

	// endFunc should satisfy the EndToolCallFunc type signature
	var typedFunc EndToolCallFunc = endFunc
	assert.NotNil(t, typedFunc)

	// Call it
	typedFunc(ctx, 100, nil)
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkMCPMetrics_RecordToolCall(b *testing.B) {
	m, _ := NewMCPMetrics("bench-mcp")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordToolCall(ctx, "tool", "provider", 0.1, 100, nil)
	}
}

func BenchmarkStreamingMetrics_RecordChunk(b *testing.B) {
	m, _ := NewStreamingMetrics("bench-streaming")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordChunk(ctx, "provider", 256)
	}
}

func BenchmarkProtocolMetrics_RecordRequest(b *testing.B) {
	m, _ := NewProtocolMetrics("bench-protocol")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordRequest(ctx, "mcp", "tool_call", 0.05, nil)
	}
}
