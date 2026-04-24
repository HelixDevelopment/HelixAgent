package qdrant_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	adapter "dev.helix.agent/internal/adapters/vectordb/qdrant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isQdrantAvailable checks whether a Qdrant server is reachable on the default port.
func isQdrantAvailable() bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", 6333), 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ============================================================================
// DefaultConfig Tests
// ============================================================================

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := adapter.DefaultConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 6333, cfg.Port)
	assert.False(t, cfg.UseTLS)
	assert.Greater(t, cfg.Timeout.Milliseconds(), int64(0))
}

func TestConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := &adapter.Config{
		Host:   "qdrant-server",
		Port:   6333,
		APIKey: "test-key",
		UseTLS: true,
	}
	assert.Equal(t, "qdrant-server", cfg.Host)
	assert.Equal(t, 6333, cfg.Port)
	assert.Equal(t, "test-key", cfg.APIKey)
	assert.True(t, cfg.UseTLS)
}

// ============================================================================
// DefaultSearchOptions Tests
// ============================================================================

func TestDefaultSearchOptions(t *testing.T) {
	t.Parallel()
	opts := adapter.DefaultSearchOptions()
	require.NotNil(t, opts)
	assert.Equal(t, 10, opts.Limit)
	assert.True(t, opts.WithPayload)
	assert.False(t, opts.WithVectors)
	assert.Equal(t, float32(0), opts.ScoreThreshold)
}

func TestSearchOptions_WithLimit(t *testing.T) {
	t.Parallel()
	opts := adapter.DefaultSearchOptions()
	result := opts.WithLimit(25)
	assert.Same(t, opts, result) // returns self for chaining
	assert.Equal(t, 25, opts.Limit)
}

func TestSearchOptions_Fields(t *testing.T) {
	t.Parallel()
	opts := &adapter.SearchOptions{
		Limit:          50,
		ScoreThreshold: 0.8,
		WithPayload:    true,
		WithVectors:    true,
		Filter:         map[string]interface{}{"type": "document"},
	}
	assert.Equal(t, 50, opts.Limit)
	assert.Equal(t, float32(0.8), opts.ScoreThreshold)
	assert.True(t, opts.WithPayload)
	assert.True(t, opts.WithVectors)
	assert.Equal(t, "document", opts.Filter["type"])
}

// ============================================================================
// DefaultCollectionConfig Tests
// ============================================================================

func TestDefaultCollectionConfig(t *testing.T) {
	t.Parallel()
	cfg := adapter.DefaultCollectionConfig("my-collection", 384)
	require.NotNil(t, cfg)
	assert.Equal(t, "my-collection", cfg.Name)
	assert.Equal(t, 384, cfg.VectorSize)
	assert.Equal(t, adapter.DistanceCosine, cfg.Distance)
}

func TestCollectionConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := &adapter.CollectionConfig{
		Name:       "test-col",
		VectorSize: 1536,
		Distance:   adapter.DistanceDot,
	}
	assert.Equal(t, "test-col", cfg.Name)
	assert.Equal(t, 1536, cfg.VectorSize)
	assert.Equal(t, adapter.DistanceDot, cfg.Distance)
}

// ============================================================================
// Distance Metric Constants
// ============================================================================

func TestDistanceMetricConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, adapter.DistanceMetric("Cosine"), adapter.DistanceCosine)
	assert.Equal(t, adapter.DistanceMetric("Dot"), adapter.DistanceDot)
	assert.Equal(t, adapter.DistanceMetric("Euclid"), adapter.DistanceEuclidean)

	// All distinct
	assert.NotEqual(t, adapter.DistanceCosine, adapter.DistanceDot)
	assert.NotEqual(t, adapter.DistanceDot, adapter.DistanceEuclidean)
	assert.NotEqual(t, adapter.DistanceCosine, adapter.DistanceEuclidean)
}

// ============================================================================
// CollectionInfo Tests
// ============================================================================

func TestCollectionInfo_Fields(t *testing.T) {
	t.Parallel()
	info := adapter.CollectionInfo{
		Name:         "test-collection",
		VectorSize:   768,
		PointsCount:  1000,
		Distance:     "Cosine",
		OptimizersOk: true,
	}
	assert.Equal(t, "test-collection", info.Name)
	assert.Equal(t, 768, info.VectorSize)
	assert.Equal(t, int64(1000), info.PointsCount)
	assert.Equal(t, "Cosine", info.Distance)
	assert.True(t, info.OptimizersOk)
}

// ============================================================================
// Point and ScoredPoint Tests
// ============================================================================

func TestPoint_Fields(t *testing.T) {
	t.Parallel()
	p := adapter.Point{
		ID:      "point-001",
		Vector:  []float32{0.1, 0.2, 0.3},
		Payload: map[string]interface{}{"text": "hello"},
	}
	assert.Equal(t, "point-001", p.ID)
	assert.Len(t, p.Vector, 3)
	assert.Equal(t, "hello", p.Payload["text"])
}

func TestScoredPoint_Fields(t *testing.T) {
	t.Parallel()
	sp := adapter.ScoredPoint{
		ID:      "scored-001",
		Score:   0.95,
		Payload: map[string]interface{}{"text": "result"},
		Vector:  []float32{0.1, 0.2},
	}
	assert.Equal(t, "scored-001", sp.ID)
	assert.Equal(t, float32(0.95), sp.Score)
	assert.Equal(t, "result", sp.Payload["text"])
	assert.Len(t, sp.Vector, 2)
}

// ============================================================================
// NewClient - creates client (Connect/HealthCheck fail without Qdrant)
// ============================================================================

func TestNewClient_DefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := adapter.DefaultConfig()
	client, err := adapter.NewClient(cfg, nil)
	// NewClient should succeed (lazy connection)
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestNewClient_NilConfig_UsesDefault(t *testing.T) {
	t.Parallel()
	// nil config should use DefaultConfig
	client, err := adapter.NewClient(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, client)
	client.Close()
}

func TestNewClient_Connect_FailsWithoutQdrant(t *testing.T) {
	t.Parallel()
	if isQdrantAvailable() {
		t.Skip("Skipping negative-path test: Qdrant is running on localhost:6333")  // SKIP-OK: #legacy-untriaged
	}

	cfg := adapter.DefaultConfig()
	client, err := adapter.NewClient(cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close()

	// Connect should fail without a running Qdrant server
	ctx := context.Background()
	err = client.Connect(ctx)
	assert.Error(t, err)
}

func TestNewClient_Close(t *testing.T) {
	t.Parallel()
	cfg := adapter.DefaultConfig()
	client, err := adapter.NewClient(cfg, nil)
	require.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)
}

func TestNewClient_HealthCheck_FailsWithoutQdrant(t *testing.T) {
	t.Parallel()
	if isQdrantAvailable() {
		t.Skip("Skipping negative-path test: Qdrant is running on localhost:6333")  // SKIP-OK: #legacy-untriaged
	}

	cfg := adapter.DefaultConfig()
	client, err := adapter.NewClient(cfg, nil)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	err = client.HealthCheck(ctx)
	// HealthCheck calls Connect which fails without server
	assert.Error(t, err)
}

func TestNewClient_CountPoints(t *testing.T) {
	t.Parallel()
	cfg := adapter.DefaultConfig()
	client, err := adapter.NewClient(cfg, nil)
	require.NoError(t, err)
	defer client.Close()

	// CountPoints is implemented as a no-op returning 0
	ctx := context.Background()
	count, err := client.CountPoints(ctx, "any-collection", nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}
