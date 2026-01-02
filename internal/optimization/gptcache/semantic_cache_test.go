package gptcache

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		vec1     []float64
		vec2     []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			vec1:     []float64{1, 0, 0},
			vec2:     []float64{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			vec1:     []float64{1, 0, 0},
			vec2:     []float64{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			vec1:     []float64{1, 0, 0},
			vec2:     []float64{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "zero vector",
			vec1:     []float64{0, 0, 0},
			vec2:     []float64{1, 0, 0},
			expected: 0.0,
		},
		{
			name:     "different lengths",
			vec1:     []float64{1, 0},
			vec2:     []float64{1, 0, 0},
			expected: 0.0,
		},
		{
			name:     "empty vectors",
			vec1:     []float64{},
			vec2:     []float64{},
			expected: 0.0,
		},
		{
			name:     "similar vectors",
			vec1:     []float64{1, 1, 0},
			vec2:     []float64{1, 0.9, 0.1},
			expected: 0.99, // approximately
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CosineSimilarity(tt.vec1, tt.vec2)
			if tt.name == "similar vectors" {
				assert.InDelta(t, tt.expected, result, 0.02)
			} else {
				assert.InDelta(t, tt.expected, result, 0.0001)
			}
		})
	}
}

func TestEuclideanDistance(t *testing.T) {
	tests := []struct {
		name     string
		vec1     []float64
		vec2     []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			vec1:     []float64{1, 0, 0},
			vec2:     []float64{1, 0, 0},
			expected: 0.0,
		},
		{
			name:     "unit distance",
			vec1:     []float64{0, 0, 0},
			vec2:     []float64{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "diagonal",
			vec1:     []float64{0, 0, 0},
			vec2:     []float64{1, 1, 1},
			expected: math.Sqrt(3),
		},
		{
			name:     "different lengths",
			vec1:     []float64{1, 0},
			vec2:     []float64{1, 0, 0},
			expected: math.MaxFloat64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EuclideanDistance(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestNormalizeL2(t *testing.T) {
	tests := []struct {
		name     string
		vec      []float64
		expected []float64
	}{
		{
			name:     "unit vector",
			vec:      []float64{1, 0, 0},
			expected: []float64{1, 0, 0},
		},
		{
			name:     "scale down",
			vec:      []float64{3, 4, 0},
			expected: []float64{0.6, 0.8, 0},
		},
		{
			name:     "zero vector",
			vec:      []float64{0, 0, 0},
			expected: []float64{0, 0, 0},
		},
		{
			name:     "empty vector",
			vec:      []float64{},
			expected: []float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeL2(tt.vec)
			assert.Equal(t, len(tt.expected), len(result))
			for i := range result {
				assert.InDelta(t, tt.expected[i], result[i], 0.0001)
			}
		})
	}
}

func TestFindMostSimilar(t *testing.T) {
	collection := [][]float64{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
		{0.9, 0.1, 0},
	}

	query := []float64{0.95, 0.05, 0}

	idx, score := FindMostSimilar(query, collection, MetricCosine)

	// {1, 0, 0} is actually most similar to {0.95, 0.05, 0} with cosine ~0.999
	assert.Equal(t, 0, idx)
	assert.Greater(t, score, 0.99)
}

func TestFindTopK(t *testing.T) {
	collection := [][]float64{
		{1, 0, 0},     // idx 0
		{0, 1, 0},     // idx 1
		{0.9, 0.1, 0}, // idx 2
		{0.8, 0.2, 0}, // idx 3
	}

	query := []float64{1, 0, 0}

	indices, scores := FindTopK(query, collection, MetricCosine, 2)

	assert.Len(t, indices, 2)
	assert.Equal(t, 0, indices[0]) // Exact match first
	assert.Equal(t, 2, indices[1]) // Second closest
	assert.Greater(t, scores[0], scores[1])
}

func TestSemanticCache_SetAndGet(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache(
		WithMaxEntries(100),
		WithSimilarityThreshold(0.8),
	)

	// Set an entry
	embedding := []float64{1, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	entry, err := cache.Set(ctx, "What is 2+2?", "4", embedding, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, entry.ID)

	// Get with same embedding (exact match)
	hit, err := cache.Get(ctx, embedding)
	require.NoError(t, err)
	assert.Equal(t, "What is 2+2?", hit.Entry.Query)
	assert.Equal(t, "4", hit.Entry.Response)
	assert.InDelta(t, 1.0, hit.Similarity, 0.01)

	// Get with similar embedding
	similarEmbedding := []float64{0.99, 0.01, 0, 0, 0, 0, 0, 0, 0, 0}
	hit, err = cache.Get(ctx, similarEmbedding)
	require.NoError(t, err)
	assert.Equal(t, "4", hit.Entry.Response)

	// Get with different embedding (should miss)
	differentEmbedding := []float64{0, 1, 0, 0, 0, 0, 0, 0, 0, 0}
	_, err = cache.Get(ctx, differentEmbedding)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestSemanticCache_GetByQueryHash(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	embedding := []float64{1, 0, 0}
	_, err := cache.Set(ctx, "test query", "test response", embedding, nil)
	require.NoError(t, err)

	// Exact query match
	entry, err := cache.GetByQueryHash(ctx, "test query")
	require.NoError(t, err)
	assert.Equal(t, "test response", entry.Response)

	// Different query
	_, err = cache.GetByQueryHash(ctx, "different query")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestSemanticCache_Remove(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	embedding := []float64{1, 0, 0}
	entry, err := cache.Set(ctx, "test", "response", embedding, nil)
	require.NoError(t, err)

	assert.Equal(t, 1, cache.Size())

	err = cache.Remove(ctx, entry.ID)
	require.NoError(t, err)

	assert.Equal(t, 0, cache.Size())

	// Remove non-existent
	err = cache.Remove(ctx, "non-existent")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestSemanticCache_Clear(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	for i := 0; i < 10; i++ {
		embedding := make([]float64, 10)
		embedding[i] = 1
		cache.Set(ctx, "query", "response", embedding, nil)
	}

	assert.Equal(t, 10, cache.Size())

	cache.Clear(ctx)

	assert.Equal(t, 0, cache.Size())
}

func TestSemanticCache_Stats(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	embedding := []float64{1, 0, 0}
	cache.Set(ctx, "test", "response", embedding, nil)

	// Cache hit
	cache.Get(ctx, embedding)
	cache.Get(ctx, embedding)

	// Cache miss
	differentEmbedding := []float64{0, 1, 0}
	cache.Get(ctx, differentEmbedding)

	stats := cache.Stats(ctx)

	assert.Equal(t, 1, stats.TotalEntries)
	assert.Equal(t, int64(2), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.InDelta(t, 0.666, stats.HitRate, 0.01)
}

func TestSemanticCache_Eviction_LRU(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache(
		WithMaxEntries(3),
		WithEvictionPolicy(EvictionLRU),
	)

	// Add 4 entries (should evict first one)
	for i := 0; i < 4; i++ {
		embedding := make([]float64, 10)
		embedding[i] = 1
		cache.Set(ctx, "query"+string(rune('0'+i)), "response", embedding, nil)
	}

	assert.Equal(t, 3, cache.Size())

	// First entry should be evicted
	embedding0 := make([]float64, 10)
	embedding0[0] = 1
	_, err := cache.Get(ctx, embedding0)
	assert.ErrorIs(t, err, ErrCacheMiss)

	// Last entry should exist
	embedding3 := make([]float64, 10)
	embedding3[3] = 1
	hit, err := cache.Get(ctx, embedding3)
	require.NoError(t, err)
	assert.Equal(t, "query3", hit.Entry.Query)
}

func TestSemanticCache_GetTopK(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	// Add multiple entries
	embeddings := [][]float64{
		{1, 0, 0},
		{0.9, 0.1, 0},
		{0.8, 0.2, 0},
		{0, 1, 0},
	}

	for i, emb := range embeddings {
		cache.Set(ctx, "query"+string(rune('0'+i)), "response", emb, nil)
	}

	// Query for top 2
	query := []float64{1, 0, 0}
	hits, err := cache.GetTopK(ctx, query, 2)
	require.NoError(t, err)

	assert.Len(t, hits, 2)
	assert.Greater(t, hits[0].Similarity, hits[1].Similarity)
}

func TestSemanticCache_Invalidate(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	// Add entries with metadata
	embedding := []float64{1, 0, 0}
	cache.Set(ctx, "query1", "response1", embedding, map[string]interface{}{"type": "test"})
	cache.Set(ctx, "query2", "response2", []float64{0, 1, 0}, map[string]interface{}{"type": "prod"})
	cache.Set(ctx, "query3", "response3", []float64{0, 0, 1}, map[string]interface{}{"type": "test"})

	assert.Equal(t, 3, cache.Size())

	// Invalidate by metadata
	count, err := cache.Invalidate(ctx, InvalidationCriteria{
		MatchMetadata: map[string]interface{}{"type": "test"},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, 1, cache.Size())
}

func TestSemanticCache_Metadata(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	metadata := map[string]interface{}{
		"model":     "gpt-4",
		"user_id":   "123",
		"timestamp": time.Now().Unix(),
	}

	embedding := []float64{1, 0, 0}
	cache.Set(ctx, "query", "response", embedding, metadata)

	hit, err := cache.Get(ctx, embedding)
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", hit.Entry.Metadata["model"])
	assert.Equal(t, "123", hit.Entry.Metadata["user_id"])
}

func TestLRUEviction(t *testing.T) {
	eviction := NewLRUEviction(3)

	// Add 3 keys
	eviction.Add("a")
	eviction.Add("b")
	eviction.Add("c")

	assert.Equal(t, 3, eviction.Size())

	// Access 'a' to make it recently used
	eviction.UpdateAccess("a")

	// Add 'd' - should evict 'b' (least recently used)
	evicted := eviction.Add("d")
	assert.Equal(t, "b", evicted)
	assert.Equal(t, 3, eviction.Size())
}

func TestTTLEviction(t *testing.T) {
	eviction := NewTTLEviction(100 * time.Millisecond)
	defer eviction.Stop()

	eviction.Add("a")
	eviction.Add("b")

	assert.Equal(t, 2, eviction.Size())

	// Wait for TTL
	time.Sleep(150 * time.Millisecond)

	expired := eviction.GetExpired()
	assert.Len(t, expired, 2)
}

func TestRelevanceEviction(t *testing.T) {
	eviction := NewRelevanceEviction(3, 0.9)

	eviction.Add("a")
	eviction.Add("b")
	eviction.Add("c")

	// Access 'a' multiple times to boost its score
	eviction.UpdateAccess("a")
	eviction.UpdateAccess("a")
	eviction.UpdateAccess("a")

	// Add 'd' - should evict 'b' or 'c' (lowest score)
	evicted := eviction.Add("d")
	assert.NotEqual(t, "a", evicted) // 'a' should not be evicted due to high score
}

func TestSemanticCache_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache(WithMaxEntries(100))

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			embedding := make([]float64, 10)
			embedding[idx%10] = 1
			cache.Set(ctx, "query", "response", embedding, nil)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(idx int) {
			embedding := make([]float64, 10)
			embedding[idx%10] = 1
			cache.Get(ctx, embedding)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.LessOrEqual(t, cache.Size(), 10)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 10000, config.MaxEntries)
	assert.Equal(t, 0.85, config.SimilarityThreshold)
	assert.Equal(t, MetricCosine, config.SimilarityMetric)
	assert.Equal(t, 24*time.Hour, config.TTL)
	assert.Equal(t, EvictionLRUWithTTL, config.EvictionPolicy)
}

func TestConfigValidation(t *testing.T) {
	config := &Config{
		MaxEntries:          -1,
		SimilarityThreshold: 2.0,
		TTL:                 -1,
	}

	config.Validate()

	assert.Equal(t, 10000, config.MaxEntries)
	assert.Equal(t, 0.85, config.SimilarityThreshold)
	assert.Equal(t, 24*time.Hour, config.TTL)
}

func BenchmarkCosineSimilarity(b *testing.B) {
	vec1 := make([]float64, 1536)
	vec2 := make([]float64, 1536)
	for i := range vec1 {
		vec1[i] = float64(i) / 1536
		vec2[i] = float64(1536-i) / 1536
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CosineSimilarity(vec1, vec2)
	}
}

func BenchmarkSemanticCacheGet(b *testing.B) {
	ctx := context.Background()
	cache := NewSemanticCache(WithMaxEntries(10000))

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		embedding := make([]float64, 128)
		embedding[i%128] = 1
		cache.Set(ctx, "query", "response", embedding, nil)
	}

	queryEmbedding := make([]float64, 128)
	queryEmbedding[50] = 1

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(ctx, queryEmbedding)
	}
}

func BenchmarkSemanticCacheSet(b *testing.B) {
	ctx := context.Background()
	cache := NewSemanticCache(WithMaxEntries(100000))

	embedding := make([]float64, 128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		embedding[i%128] = float64(i)
		cache.Set(ctx, "query", "response", embedding, nil)
	}
}
