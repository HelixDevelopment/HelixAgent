// Package cache provides caching functionality for HelixAgent.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SemanticCache provides semantic similarity-based caching.
type SemanticCache struct {
	db        *pgxpool.Pool
	embedder  EmbeddingGenerator
	threshold float64
	ttl       time.Duration
}

// EmbeddingGenerator generates embeddings for text.
type EmbeddingGenerator interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// SemanticCacheEntry represents a cached response for semantic cache.
type SemanticCacheEntry struct {
	Key       string
	Query     string
	Embedding []float32
	Response  interface{}
	Metadata  map[string]interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
	HitCount  int
}

// NewSemanticCache creates a new semantic cache.
func NewSemanticCache(db *pgxpool.Pool, embedder EmbeddingGenerator, threshold float64, ttl time.Duration) *SemanticCache {
	if threshold == 0 {
		threshold = 0.85 // Default similarity threshold
	}
	if ttl == 0 {
		ttl = 1 * time.Hour
	}

	return &SemanticCache{
		db:        db,
		embedder:  embedder,
		threshold: threshold,
		ttl:       ttl,
	}
}

// Get retrieves a cached response by semantic similarity.
func (c *SemanticCache) Get(ctx context.Context, query string) (*SemanticCacheEntry, error) {
	// Generate embedding for query
	queryEmb, err := c.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Search for similar cached queries
	rows, err := c.db.Query(ctx, `
		SELECT key, query, embedding, response, metadata, created_at, expires_at, hit_count,
		       1 - (embedding <=> $1) as similarity
		FROM semantic_cache
		WHERE expires_at > NOW()
		ORDER BY embedding <=> $1
		LIMIT 5
	`, queryEmb)
	if err != nil {
		return nil, fmt.Errorf("query cache: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry SemanticCacheEntry
		var responseJSON []byte
		var metadataJSON []byte
		var similarity float64

		err := rows.Scan(
			&entry.Key, &entry.Query, &entry.Embedding,
			&responseJSON, &metadataJSON,
			&entry.CreatedAt, &entry.ExpiresAt, &entry.HitCount,
			&similarity,
		)
		if err != nil {
			continue
		}

		// Check similarity threshold
		if similarity < c.threshold {
			continue
		}

		// Parse response
		if err := json.Unmarshal(responseJSON, &entry.Response); err != nil {
			continue
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &entry.Metadata)
		}

		// Update hit count
		c.incrementHitCount(ctx, entry.Key)

		return &entry, nil
	}

	return nil, nil
}

// Set stores a response in the cache.
func (c *SemanticCache) Set(ctx context.Context, key, query string, response interface{}, metadata map[string]interface{}) error {
	// Generate embedding
	embedding, err := c.embedder.Embed(ctx, query)
	if err != nil {
		return fmt.Errorf("embed query: %w", err)
	}

	// Serialize response
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	// Serialize metadata
	var metadataJSON []byte
	if metadata != nil {
		metadataJSON, _ = json.Marshal(metadata)
	}

	// Store in database
	_, err = c.db.Exec(ctx, `
		INSERT INTO semantic_cache (
			key, query, embedding, response, metadata,
			created_at, expires_at, hit_count
		) VALUES ($1, $2, $3, $4, $5, NOW(), $6, 0)
		ON CONFLICT (key) DO UPDATE
		SET query = EXCLUDED.query,
		    embedding = EXCLUDED.embedding,
		    response = EXCLUDED.response,
		    metadata = EXCLUDED.metadata,
		    created_at = EXCLUDED.created_at,
		    expires_at = EXCLUDED.expires_at,
		    hit_count = 0
	`, key, query, embedding, responseJSON, metadataJSON, time.Now().Add(c.ttl))

	if err != nil {
		return fmt.Errorf("store cache: %w", err)
	}

	return nil
}

// Delete removes an entry from the cache.
func (c *SemanticCache) Delete(ctx context.Context, key string) error {
	_, err := c.db.Exec(ctx, "DELETE FROM semantic_cache WHERE key = $1", key)
	return err
}

// Clear removes all entries from the cache.
func (c *SemanticCache) Clear(ctx context.Context) error {
	_, err := c.db.Exec(ctx, "TRUNCATE semantic_cache")
	return err
}

// GetStats returns cache statistics.
func (c *SemanticCache) GetStats(ctx context.Context) (*CacheStats, error) {
	var stats CacheStats

	err := c.db.QueryRow(ctx, `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE expires_at > NOW()),
			COUNT(*) FILTER (WHERE expires_at <= NOW()),
			SUM(hit_count),
			AVG(hit_count)
		FROM semantic_cache
	`).Scan(
		&stats.TotalEntries,
		&stats.ActiveEntries,
		&stats.ExpiredEntries,
		&stats.TotalHits,
		&stats.AvgHits,
	)

	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// Cleanup removes expired entries.
func (c *SemanticCache) Cleanup(ctx context.Context) (int64, error) {
	result, err := c.db.Exec(ctx, "DELETE FROM semantic_cache WHERE expires_at <= NOW()")
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// incrementHitCount increments the hit count for a key.
func (c *SemanticCache) incrementHitCount(ctx context.Context, key string) {
	c.db.Exec(ctx, `
		UPDATE semantic_cache 
		SET hit_count = hit_count + 1 
		WHERE key = $1
	`, key)
}

// CacheStats represents cache statistics.
type CacheStats struct {
	TotalEntries   int64
	ActiveEntries  int64
	ExpiredEntries int64
	TotalHits      int64
	AvgHits        float64
}

// CosineSimilarity calculates cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct float64
	var normA float64
	var normB float64

	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// SimpleEmbedder is a simple embedding generator for testing.
type SimpleEmbedder struct{}

// Embed generates a simple embedding.
func (e *SimpleEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Simple bag-of-words embedding
	words := make(map[string]int)
	for _, word := range splitWords(text) {
		words[word]++
	}

	// Create a simple vector
	embedding := make([]float32, 100)
	for word, count := range words {
		hash := hashString(word) % 100
		embedding[hash] = float32(count)
	}

	// Normalize
	var sum float32
	for _, v := range embedding {
		sum += v * v
	}
	if sum > 0 {
		norm := float32(1.0 / math.Sqrt(float64(sum)))
		for i := range embedding {
			embedding[i] *= norm
		}
	}

	return embedding, nil
}

func splitWords(text string) []string {
	var words []string
	var current []rune

	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			current = append(current, r)
		} else if len(current) > 0 {
			words = append(words, string(current))
			current = nil
		}
	}

	if len(current) > 0 {
		words = append(words, string(current))
	}

	return words
}

func hashString(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}
