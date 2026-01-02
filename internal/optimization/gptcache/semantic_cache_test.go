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

func TestDotProduct(t *testing.T) {
	tests := []struct {
		name     string
		vec1     []float64
		vec2     []float64
		expected float64
	}{
		{
			name:     "orthogonal vectors",
			vec1:     []float64{1, 0, 0},
			vec2:     []float64{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "parallel vectors",
			vec1:     []float64{1, 0, 0},
			vec2:     []float64{2, 0, 0},
			expected: 2.0,
		},
		{
			name:     "mixed vectors",
			vec1:     []float64{1, 2, 3},
			vec2:     []float64{4, 5, 6},
			expected: 32.0, // 1*4 + 2*5 + 3*6 = 32
		},
		{
			name:     "different lengths",
			vec1:     []float64{1, 2},
			vec2:     []float64{1, 2, 3},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DotProduct(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestManhattanDistance(t *testing.T) {
	tests := []struct {
		name     string
		vec1     []float64
		vec2     []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			vec1:     []float64{1, 2, 3},
			vec2:     []float64{1, 2, 3},
			expected: 0.0,
		},
		{
			name:     "unit distance",
			vec1:     []float64{0, 0, 0},
			vec2:     []float64{1, 1, 1},
			expected: 3.0,
		},
		{
			name:     "mixed distances",
			vec1:     []float64{1, 2, 3},
			vec2:     []float64{4, 6, 9},
			expected: 13.0, // |4-1| + |6-2| + |9-3| = 3 + 4 + 6 = 13
		},
		{
			name:     "different lengths",
			vec1:     []float64{1, 2},
			vec2:     []float64{1, 2, 3},
			expected: math.MaxFloat64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ManhattanDistance(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestComputeSimilarity(t *testing.T) {
	vec1 := []float64{1, 0, 0}
	vec2 := []float64{0.9, 0.1, 0}

	// Test all metrics
	cosineSim := ComputeSimilarity(vec1, vec2, MetricCosine)
	assert.Greater(t, cosineSim, 0.9)

	euclideanSim := ComputeSimilarity(vec1, vec2, MetricEuclidean)
	assert.Greater(t, euclideanSim, 0.0)
	assert.Less(t, euclideanSim, 1.0)

	dotSim := ComputeSimilarity(vec1, vec2, MetricDotProduct)
	assert.Greater(t, dotSim, 0.0)

	manhattanSim := ComputeSimilarity(vec1, vec2, MetricManhattan)
	assert.Greater(t, manhattanSim, 0.0)
	assert.Less(t, manhattanSim, 1.0)

	// Test default metric (invalid)
	defaultSim := ComputeSimilarity(vec1, vec2, "invalid")
	assert.InDelta(t, CosineSimilarity(vec1, vec2), defaultSim, 0.0001)
}

func TestFindMostSimilar_EmptyCollection(t *testing.T) {
	query := []float64{1, 0, 0}
	idx, score := FindMostSimilar(query, [][]float64{}, MetricCosine)

	assert.Equal(t, -1, idx)
	assert.Equal(t, 0.0, score)
}

func TestFindTopK_EdgeCases(t *testing.T) {
	query := []float64{1, 0, 0}

	// Empty collection
	indices, scores := FindTopK(query, [][]float64{}, MetricCosine, 5)
	assert.Nil(t, indices)
	assert.Nil(t, scores)

	// K = 0
	collection := [][]float64{{1, 0, 0}, {0, 1, 0}}
	indices, scores = FindTopK(query, collection, MetricCosine, 0)
	assert.Nil(t, indices)
	assert.Nil(t, scores)

	// K > collection size
	indices, scores = FindTopK(query, collection, MetricCosine, 10)
	assert.Len(t, indices, 2) // Should return only what's available
	assert.Len(t, scores, 2)
}

func TestLRUEviction_Remove(t *testing.T) {
	eviction := NewLRUEviction(5)

	eviction.Add("a")
	eviction.Add("b")
	eviction.Add("c")

	assert.Equal(t, 3, eviction.Size())

	eviction.Remove("b")
	assert.Equal(t, 2, eviction.Size())

	// Remove non-existent key (should not panic)
	eviction.Remove("nonexistent")
	assert.Equal(t, 2, eviction.Size())
}

func TestLRUEviction_UpdateNonExistent(t *testing.T) {
	eviction := NewLRUEviction(5)

	// UpdateAccess on non-existent key should not panic
	eviction.UpdateAccess("nonexistent")
	assert.Equal(t, 0, eviction.Size())
}

func TestLRUEviction_AddExisting(t *testing.T) {
	eviction := NewLRUEviction(3)

	eviction.Add("a")
	eviction.Add("b")

	// Adding existing key should move to front, not create duplicate
	evicted := eviction.Add("a")
	assert.Empty(t, evicted)
	assert.Equal(t, 2, eviction.Size())
}

func TestTTLEviction_Remove(t *testing.T) {
	eviction := NewTTLEviction(time.Hour)
	defer eviction.Stop()

	eviction.Add("a")
	eviction.Add("b")
	assert.Equal(t, 2, eviction.Size())

	eviction.Remove("a")
	assert.Equal(t, 1, eviction.Size())
}

func TestTTLEviction_UpdateAccess(t *testing.T) {
	eviction := NewTTLEviction(100 * time.Millisecond)
	defer eviction.Stop()

	eviction.Add("a")
	time.Sleep(50 * time.Millisecond)

	// Update access should refresh timestamp
	eviction.UpdateAccess("a")

	time.Sleep(60 * time.Millisecond) // 110ms total, but only 60ms since update

	expired := eviction.GetExpired()
	assert.Empty(t, expired) // Should not be expired due to refresh

	// UpdateAccess on non-existent should be no-op
	eviction.UpdateAccess("nonexistent")
}

func TestLRUWithTTLEviction(t *testing.T) {
	var evictedKeys []string
	onEvict := func(key string) {
		evictedKeys = append(evictedKeys, key)
	}

	eviction := NewLRUWithTTLEviction(5, time.Hour, onEvict)
	defer eviction.Stop()

	eviction.Add("a")
	eviction.Add("b")
	eviction.Add("c")

	assert.Equal(t, 3, eviction.Size())

	eviction.UpdateAccess("a")
	eviction.Remove("b")

	assert.Equal(t, 2, eviction.Size())
}

func TestLRUWithTTLEviction_Eviction(t *testing.T) {
	var evictedKeys []string
	onEvict := func(key string) {
		evictedKeys = append(evictedKeys, key)
	}

	eviction := NewLRUWithTTLEviction(3, time.Hour, onEvict)
	defer eviction.Stop()

	eviction.Add("a")
	eviction.Add("b")
	eviction.Add("c")

	// Adding 4th should evict oldest (LRU)
	evicted := eviction.Add("d")
	assert.Equal(t, "a", evicted)
	assert.Equal(t, 3, eviction.Size())
}

func TestRelevanceEviction_Remove(t *testing.T) {
	eviction := NewRelevanceEviction(5, 0.9)

	eviction.Add("a")
	eviction.Add("b")
	assert.Equal(t, 2, eviction.Size())

	eviction.Remove("a")
	assert.Equal(t, 1, eviction.Size())
}

func TestRelevanceEviction_GetScore(t *testing.T) {
	eviction := NewRelevanceEviction(5, 0.9)

	eviction.Add("a")
	score := eviction.GetScore("a")
	assert.Equal(t, 1.0, score)

	eviction.UpdateAccess("a")
	score = eviction.GetScore("a")
	assert.Equal(t, 2.0, score) // Initial 1.0 + 1.0 from UpdateAccess

	// Non-existent key
	score = eviction.GetScore("nonexistent")
	assert.Equal(t, 0.0, score)
}

func TestRelevanceEviction_UpdateNonExistent(t *testing.T) {
	eviction := NewRelevanceEviction(5, 0.9)

	// Should not panic or add the key
	eviction.UpdateAccess("nonexistent")
	assert.Equal(t, 0, eviction.Size())
}

func TestSemanticCache_Eviction_TTL(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache(
		WithMaxEntries(100),
		WithEvictionPolicy(EvictionTTL),
		WithTTL(50*time.Millisecond),
	)

	embedding := []float64{1, 0, 0}
	cache.Set(ctx, "query", "response", embedding, nil)

	assert.Equal(t, 1, cache.Size())

	// Entry should be retrievable immediately
	hit, err := cache.Get(ctx, embedding)
	require.NoError(t, err)
	assert.Equal(t, "response", hit.Entry.Response)
}

func TestSemanticCache_WithOptions(t *testing.T) {
	cache := NewSemanticCache(
		WithMaxEntries(500),
		WithSimilarityThreshold(0.9),
		WithTTL(time.Hour),
		WithSimilarityMetric(MetricEuclidean),
		WithEvictionPolicy(EvictionRelevance),
	)

	assert.NotNil(t, cache)
	stats := cache.Stats(context.Background())
	assert.Equal(t, 0, stats.TotalEntries)
}

func TestSemanticCache_InvalidateOlderThan(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	embedding := []float64{1, 0, 0}
	cache.Set(ctx, "query1", "response1", embedding, nil)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Invalidate entries older than 5ms (should invalidate our entry)
	count, err := cache.Invalidate(ctx, InvalidationCriteria{
		OlderThan: 5 * time.Millisecond,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, 0, cache.Size())
}

func TestSemanticCache_InvalidateBySimilarity(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	cache.Set(ctx, "query1", "response1", []float64{1, 0, 0}, nil)
	cache.Set(ctx, "query2", "response2", []float64{0.95, 0.05, 0}, nil) // Similar to query1
	cache.Set(ctx, "query3", "response3", []float64{0, 0, 1}, nil)       // Not similar

	// Invalidate entries similar to [1, 0, 0]
	count, err := cache.Invalidate(ctx, InvalidationCriteria{
		SimilarTo:           []float64{1, 0, 0},
		SimilarityThreshold: 0.9,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, count)        // query1 and query2 are similar
	assert.Equal(t, 1, cache.Size()) // Only query3 remains
}

func TestWithEmbeddingDimension(t *testing.T) {
	cache := NewSemanticCache(
		WithEmbeddingDimension(768),
	)
	assert.Equal(t, 768, cache.Config().EmbeddingDimension)
}

func TestWithNormalizeEmbeddings(t *testing.T) {
	cache := NewSemanticCache(
		WithNormalizeEmbeddings(false),
	)
	assert.False(t, cache.Config().NormalizeEmbeddings)

	cache2 := NewSemanticCache(
		WithNormalizeEmbeddings(true),
	)
	assert.True(t, cache2.Config().NormalizeEmbeddings)
}

func TestWithDecayFactor(t *testing.T) {
	cache := NewSemanticCache(
		WithDecayFactor(0.8),
		WithEvictionPolicy(EvictionRelevance),
	)
	assert.Equal(t, 0.8, cache.Config().DecayFactor)
}

func TestNewSemanticCacheWithConfig(t *testing.T) {
	config := &Config{
		MaxEntries:          500,
		SimilarityThreshold: 0.9,
		TTL:                 time.Hour,
	}

	cache := NewSemanticCacheWithConfig(config)
	assert.NotNil(t, cache)
	assert.Equal(t, 500, cache.Config().MaxEntries)
	assert.Equal(t, 0.9, cache.Config().SimilarityThreshold)

	// Test with nil config
	cacheDefault := NewSemanticCacheWithConfig(nil)
	assert.NotNil(t, cacheDefault)
	assert.Equal(t, 10000, cacheDefault.Config().MaxEntries) // Default value
}

func TestSemanticCache_SetWithID(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	customID := "custom-entry-id-123"
	embedding := []float64{1, 0, 0}
	entry, err := cache.SetWithID(ctx, customID, "query", "response", embedding, nil)
	require.NoError(t, err)
	assert.Equal(t, customID, entry.ID)

	// Verify we can retrieve by ID
	retrieved, err := cache.GetByID(ctx, customID)
	require.NoError(t, err)
	assert.Equal(t, "query", retrieved.Query)
	assert.Equal(t, "response", retrieved.Response)
}

func TestSemanticCache_GetByID(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	embedding := []float64{1, 0, 0}
	entry, err := cache.Set(ctx, "query", "response", embedding, nil)
	require.NoError(t, err)

	// Get by ID
	retrieved, err := cache.GetByID(ctx, entry.ID)
	require.NoError(t, err)
	assert.Equal(t, entry.ID, retrieved.ID)
	assert.Equal(t, "query", retrieved.Query)

	// Get non-existent ID
	_, err = cache.GetByID(ctx, "non-existent-id")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestSemanticCache_SetOnEvict(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache(
		WithMaxEntries(2),
		WithEvictionPolicy(EvictionLRU),
	)

	var evictedEntry *CacheEntry
	cache.SetOnEvict(func(entry *CacheEntry) {
		evictedEntry = entry
	})

	// Add 3 entries to trigger eviction
	cache.Set(ctx, "query1", "response1", []float64{1, 0, 0}, nil)
	cache.Set(ctx, "query2", "response2", []float64{0, 1, 0}, nil)
	cache.Set(ctx, "query3", "response3", []float64{0, 0, 1}, nil)

	assert.NotNil(t, evictedEntry)
	assert.Equal(t, "query1", evictedEntry.Query)
}

func TestSemanticCache_GetAllEntries(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	cache.Set(ctx, "query1", "response1", []float64{1, 0, 0}, nil)
	cache.Set(ctx, "query2", "response2", []float64{0, 1, 0}, nil)
	cache.Set(ctx, "query3", "response3", []float64{0, 0, 1}, nil)

	entries := cache.GetAllEntries(ctx)
	assert.Len(t, entries, 3)

	queries := make(map[string]bool)
	for _, e := range entries {
		queries[e.Query] = true
	}
	assert.True(t, queries["query1"])
	assert.True(t, queries["query2"])
	assert.True(t, queries["query3"])
}

func TestSemanticCache_Config(t *testing.T) {
	cache := NewSemanticCache(
		WithMaxEntries(500),
		WithSimilarityThreshold(0.9),
	)

	config := cache.Config()
	assert.NotNil(t, config)
	assert.Equal(t, 500, config.MaxEntries)
	assert.Equal(t, 0.9, config.SimilarityThreshold)
}

func TestSemanticCache_GetWithThreshold(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache(
		WithSimilarityThreshold(0.5), // Low default threshold
	)

	embedding := []float64{1, 0, 0}
	cache.Set(ctx, "query", "response", embedding, nil)

	// Get with higher threshold - should hit
	hit, err := cache.GetWithThreshold(ctx, embedding, 0.99)
	require.NoError(t, err)
	assert.Equal(t, "response", hit.Entry.Response)

	// Get with similar embedding but high threshold - should miss
	similarEmb := []float64{0.5, 0.5, 0.5}
	_, err = cache.GetWithThreshold(ctx, similarEmb, 0.99)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestSemanticCache_EmptyEmbedding(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	// Set with empty embedding should fail
	_, err := cache.Set(ctx, "query", "response", []float64{}, nil)
	assert.ErrorIs(t, err, ErrInvalidEmbedding)

	// Get with empty embedding should fail
	_, err = cache.Get(ctx, []float64{})
	assert.ErrorIs(t, err, ErrInvalidEmbedding)

	// SetWithID with empty embedding should fail
	_, err = cache.SetWithID(ctx, "id", "query", "response", []float64{}, nil)
	assert.ErrorIs(t, err, ErrInvalidEmbedding)

	// GetTopK with empty embedding should fail
	_, err = cache.GetTopK(ctx, []float64{}, 5)
	assert.ErrorIs(t, err, ErrInvalidEmbedding)
}

func TestSemanticCache_GetTopK_Empty(t *testing.T) {
	ctx := context.Background()
	cache := NewSemanticCache()

	// No entries - should return nil
	hits, err := cache.GetTopK(ctx, []float64{1, 0, 0}, 5)
	require.NoError(t, err)
	assert.Nil(t, hits)
}
