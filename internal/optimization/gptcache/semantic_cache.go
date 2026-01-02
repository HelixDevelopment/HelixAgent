package gptcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrCacheMiss indicates no matching entry was found.
	ErrCacheMiss = errors.New("cache miss")
	// ErrInvalidEmbedding indicates the embedding is invalid.
	ErrInvalidEmbedding = errors.New("invalid embedding")
)

// CacheEntry represents a cached query-response pair.
type CacheEntry struct {
	ID          string                 `json:"id"`
	Query       string                 `json:"query"`
	QueryHash   string                 `json:"query_hash"`
	Response    string                 `json:"response"`
	Embedding   []float64              `json:"embedding"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	AccessedAt  time.Time              `json:"accessed_at"`
	AccessCount int                    `json:"access_count"`
}

// CacheHit represents a successful cache lookup.
type CacheHit struct {
	Entry      *CacheEntry `json:"entry"`
	Similarity float64     `json:"similarity"`
}

// CacheStats contains cache statistics.
type CacheStats struct {
	TotalEntries int     `json:"total_entries"`
	Hits         int64   `json:"hits"`
	Misses       int64   `json:"misses"`
	HitRate      float64 `json:"hit_rate"`
	AvgSimilarity float64 `json:"avg_similarity"`
}

// SemanticCache provides semantic similarity-based caching for LLM queries.
type SemanticCache struct {
	mu sync.RWMutex

	// Storage
	entries    map[string]*CacheEntry // ID -> Entry
	embeddings [][]float64            // Ordered embeddings for similarity search
	entryIDs   []string               // Ordered entry IDs matching embeddings

	// Eviction
	eviction EvictionStrategy

	// Configuration
	config *Config

	// Statistics
	hits          int64
	misses        int64
	totalSimilarity float64
	hitCount      int64

	// Callbacks
	onEvict func(entry *CacheEntry)
}

// NewSemanticCache creates a new semantic cache with the given options.
func NewSemanticCache(opts ...ConfigOption) *SemanticCache {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(config)
	}
	config.Validate()

	cache := &SemanticCache{
		entries:    make(map[string]*CacheEntry),
		embeddings: make([][]float64, 0),
		entryIDs:   make([]string, 0),
		config:     config,
	}

	// Initialize eviction strategy
	cache.initEviction()

	return cache
}

// NewSemanticCacheWithConfig creates a new semantic cache with explicit config.
func NewSemanticCacheWithConfig(config *Config) *SemanticCache {
	if config == nil {
		config = DefaultConfig()
	}
	config.Validate()

	cache := &SemanticCache{
		entries:    make(map[string]*CacheEntry),
		embeddings: make([][]float64, 0),
		entryIDs:   make([]string, 0),
		config:     config,
	}

	cache.initEviction()

	return cache
}

func (c *SemanticCache) initEviction() {
	switch c.config.EvictionPolicy {
	case EvictionLRU:
		c.eviction = NewLRUEviction(c.config.MaxEntries)
	case EvictionTTL:
		c.eviction = NewTTLEviction(c.config.TTL)
	case EvictionLRUWithTTL:
		c.eviction = NewLRUWithTTLEviction(c.config.MaxEntries, c.config.TTL, func(key string) {
			c.removeByID(key)
		})
	case EvictionRelevance:
		c.eviction = NewRelevanceEviction(c.config.MaxEntries, c.config.DecayFactor)
	default:
		c.eviction = NewLRUEviction(c.config.MaxEntries)
	}
}

// Get searches for a semantically similar cached entry.
// Returns the cache hit if found, or ErrCacheMiss if not found.
func (c *SemanticCache) Get(ctx context.Context, embedding []float64) (*CacheHit, error) {
	return c.GetWithThreshold(ctx, embedding, c.config.SimilarityThreshold)
}

// GetWithThreshold searches with a custom similarity threshold.
func (c *SemanticCache) GetWithThreshold(ctx context.Context, embedding []float64, threshold float64) (*CacheHit, error) {
	if len(embedding) == 0 {
		return nil, ErrInvalidEmbedding
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.embeddings) == 0 {
		c.misses++
		return nil, ErrCacheMiss
	}

	// Normalize if configured
	searchEmbedding := embedding
	if c.config.NormalizeEmbeddings {
		searchEmbedding = NormalizeL2(embedding)
	}

	// Find most similar
	bestIdx, bestScore := FindMostSimilar(searchEmbedding, c.embeddings, c.config.SimilarityMetric)

	if bestIdx < 0 || bestScore < threshold {
		c.misses++
		return nil, ErrCacheMiss
	}

	// Get entry
	entryID := c.entryIDs[bestIdx]
	entry, ok := c.entries[entryID]
	if !ok {
		c.misses++
		return nil, ErrCacheMiss
	}

	// Update access metadata
	entry.AccessedAt = time.Now()
	entry.AccessCount++
	c.eviction.UpdateAccess(entryID)

	// Update stats
	c.hits++
	c.totalSimilarity += bestScore
	c.hitCount++

	return &CacheHit{
		Entry:      entry,
		Similarity: bestScore,
	}, nil
}

// Set stores a query-response pair in the cache.
func (c *SemanticCache) Set(ctx context.Context, query, response string, embedding []float64, metadata map[string]interface{}) (*CacheEntry, error) {
	if len(embedding) == 0 {
		return nil, ErrInvalidEmbedding
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Normalize if configured
	storeEmbedding := embedding
	if c.config.NormalizeEmbeddings {
		storeEmbedding = NormalizeL2(embedding)
	}

	// Create entry
	entry := &CacheEntry{
		ID:          uuid.New().String(),
		Query:       query,
		QueryHash:   hashQuery(query),
		Response:    response,
		Embedding:   storeEmbedding,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 1,
	}

	// Store entry
	c.entries[entry.ID] = entry
	c.embeddings = append(c.embeddings, storeEmbedding)
	c.entryIDs = append(c.entryIDs, entry.ID)

	// Check for eviction
	if evicted := c.eviction.Add(entry.ID); evicted != "" {
		c.removeByIDLocked(evicted)
	}

	return entry, nil
}

// SetWithID stores an entry with a specific ID (useful for persistence).
func (c *SemanticCache) SetWithID(ctx context.Context, id, query, response string, embedding []float64, metadata map[string]interface{}) (*CacheEntry, error) {
	if len(embedding) == 0 {
		return nil, ErrInvalidEmbedding
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	storeEmbedding := embedding
	if c.config.NormalizeEmbeddings {
		storeEmbedding = NormalizeL2(embedding)
	}

	entry := &CacheEntry{
		ID:          id,
		Query:       query,
		QueryHash:   hashQuery(query),
		Response:    response,
		Embedding:   storeEmbedding,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 1,
	}

	c.entries[entry.ID] = entry
	c.embeddings = append(c.embeddings, storeEmbedding)
	c.entryIDs = append(c.entryIDs, entry.ID)

	if evicted := c.eviction.Add(entry.ID); evicted != "" {
		c.removeByIDLocked(evicted)
	}

	return entry, nil
}

// GetByID retrieves an entry by its ID.
func (c *SemanticCache) GetByID(ctx context.Context, id string) (*CacheEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[id]
	if !ok {
		return nil, ErrCacheMiss
	}

	return entry, nil
}

// GetByQueryHash retrieves an entry by exact query hash match.
func (c *SemanticCache) GetByQueryHash(ctx context.Context, query string) (*CacheEntry, error) {
	hash := hashQuery(query)

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, entry := range c.entries {
		if entry.QueryHash == hash {
			entry.AccessedAt = time.Now()
			entry.AccessCount++
			c.eviction.UpdateAccess(entry.ID)
			c.hits++
			return entry, nil
		}
	}

	c.misses++
	return nil, ErrCacheMiss
}

// Remove removes an entry by ID.
func (c *SemanticCache) Remove(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.removeByIDLocked(id)
}

func (c *SemanticCache) removeByID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeByIDLocked(id)
}

func (c *SemanticCache) removeByIDLocked(id string) error {
	entry, ok := c.entries[id]
	if !ok {
		return ErrCacheMiss
	}

	// Call eviction callback
	if c.onEvict != nil {
		c.onEvict(entry)
	}

	// Remove from entries
	delete(c.entries, id)

	// Remove from embeddings and entryIDs
	for i, eid := range c.entryIDs {
		if eid == id {
			c.embeddings = append(c.embeddings[:i], c.embeddings[i+1:]...)
			c.entryIDs = append(c.entryIDs[:i], c.entryIDs[i+1:]...)
			break
		}
	}

	c.eviction.Remove(id)

	return nil
}

// Clear removes all entries from the cache.
func (c *SemanticCache) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.embeddings = make([][]float64, 0)
	c.entryIDs = make([]string, 0)

	// Reinitialize eviction
	c.initEviction()
}

// Stats returns cache statistics.
func (c *SemanticCache) Stats(ctx context.Context) *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	var avgSimilarity float64
	if c.hitCount > 0 {
		avgSimilarity = c.totalSimilarity / float64(c.hitCount)
	}

	return &CacheStats{
		TotalEntries:  len(c.entries),
		Hits:          c.hits,
		Misses:        c.misses,
		HitRate:       hitRate,
		AvgSimilarity: avgSimilarity,
	}
}

// Size returns the current number of entries.
func (c *SemanticCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// GetTopK finds the top K most similar entries.
func (c *SemanticCache) GetTopK(ctx context.Context, embedding []float64, k int) ([]*CacheHit, error) {
	if len(embedding) == 0 {
		return nil, ErrInvalidEmbedding
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.embeddings) == 0 {
		return nil, nil
	}

	searchEmbedding := embedding
	if c.config.NormalizeEmbeddings {
		searchEmbedding = NormalizeL2(embedding)
	}

	indices, scores := FindTopK(searchEmbedding, c.embeddings, c.config.SimilarityMetric, k)

	hits := make([]*CacheHit, 0, len(indices))
	for i, idx := range indices {
		entryID := c.entryIDs[idx]
		entry, ok := c.entries[entryID]
		if ok {
			hits = append(hits, &CacheHit{
				Entry:      entry,
				Similarity: scores[i],
			})
		}
	}

	return hits, nil
}

// SetOnEvict sets a callback function to be called when an entry is evicted.
func (c *SemanticCache) SetOnEvict(fn func(entry *CacheEntry)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvict = fn
}

// GetAllEntries returns all cached entries (for persistence).
func (c *SemanticCache) GetAllEntries(ctx context.Context) []*CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]*CacheEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, entry)
	}
	return entries
}

// Config returns the cache configuration.
func (c *SemanticCache) Config() *Config {
	return c.config
}

func hashQuery(query string) string {
	h := sha256.Sum256([]byte(query))
	return hex.EncodeToString(h[:])
}

// InvalidationCriteria defines criteria for bulk invalidation.
type InvalidationCriteria struct {
	// OlderThan invalidates entries older than this duration.
	OlderThan time.Duration
	// MatchMetadata invalidates entries with matching metadata.
	MatchMetadata map[string]interface{}
	// SimilarTo invalidates entries similar to this embedding.
	SimilarTo []float64
	// SimilarityThreshold is the threshold for similarity-based invalidation.
	SimilarityThreshold float64
}

// Invalidate removes entries matching the given criteria.
// Returns the number of entries invalidated.
func (c *SemanticCache) Invalidate(ctx context.Context, criteria InvalidationCriteria) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	toRemove := make([]string, 0)

	for id, entry := range c.entries {
		shouldRemove := false

		// Check age
		if criteria.OlderThan > 0 && time.Since(entry.CreatedAt) > criteria.OlderThan {
			shouldRemove = true
		}

		// Check metadata
		if len(criteria.MatchMetadata) > 0 && entry.Metadata != nil {
			for k, v := range criteria.MatchMetadata {
				if entry.Metadata[k] == v {
					shouldRemove = true
					break
				}
			}
		}

		// Check similarity
		if len(criteria.SimilarTo) > 0 {
			similarity := ComputeSimilarity(criteria.SimilarTo, entry.Embedding, c.config.SimilarityMetric)
			if similarity >= criteria.SimilarityThreshold {
				shouldRemove = true
			}
		}

		if shouldRemove {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		c.removeByIDLocked(id)
	}

	return len(toRemove), nil
}
