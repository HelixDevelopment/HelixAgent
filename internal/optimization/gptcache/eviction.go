package gptcache

import (
	"container/list"
	"sync"
	"time"
)

// EvictionStrategy defines the interface for cache eviction policies.
type EvictionStrategy interface {
	// Add adds a key to the eviction tracker.
	// Returns the key to evict if capacity is exceeded, or empty string if no eviction needed.
	Add(key string) string
	// UpdateAccess updates access metadata for a key (e.g., for LRU).
	UpdateAccess(key string)
	// Remove removes a key from the eviction tracker.
	Remove(key string)
	// Size returns the current number of tracked entries.
	Size() int
}

// LRUEviction implements Least Recently Used eviction policy.
type LRUEviction struct {
	mu      sync.Mutex
	maxSize int
	order   *list.List
	index   map[string]*list.Element
}

// NewLRUEviction creates a new LRU eviction strategy.
func NewLRUEviction(maxSize int) *LRUEviction {
	return &LRUEviction{
		maxSize: maxSize,
		order:   list.New(),
		index:   make(map[string]*list.Element),
	}
}

// Add adds a key to the LRU tracker.
func (e *LRUEviction) Add(key string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	// If key exists, move to front
	if elem, exists := e.index[key]; exists {
		e.order.MoveToFront(elem)
		return ""
	}

	// Add new key to front
	e.index[key] = e.order.PushFront(key)

	// Check if eviction needed
	if e.order.Len() > e.maxSize {
		oldest := e.order.Back()
		if oldest != nil {
			evicted := oldest.Value.(string)
			e.order.Remove(oldest)
			delete(e.index, evicted)
			return evicted
		}
	}

	return ""
}

// UpdateAccess moves a key to the front (most recently used).
func (e *LRUEviction) UpdateAccess(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if elem, exists := e.index[key]; exists {
		e.order.MoveToFront(elem)
	}
}

// Remove removes a key from the tracker.
func (e *LRUEviction) Remove(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if elem, exists := e.index[key]; exists {
		e.order.Remove(elem)
		delete(e.index, key)
	}
}

// Size returns the number of tracked entries.
func (e *LRUEviction) Size() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.order.Len()
}

// TTLEviction implements Time-To-Live eviction policy.
type TTLEviction struct {
	mu          sync.Mutex
	ttl         time.Duration
	entries     map[string]time.Time
	stopCleanup chan struct{}
}

// NewTTLEviction creates a new TTL eviction strategy.
func NewTTLEviction(ttl time.Duration) *TTLEviction {
	e := &TTLEviction{
		ttl:         ttl,
		entries:     make(map[string]time.Time),
		stopCleanup: make(chan struct{}),
	}
	go e.cleanupLoop()
	return e
}

// Add adds a key with current timestamp.
func (e *TTLEviction) Add(key string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.entries[key] = time.Now()
	return "" // TTL doesn't evict on add
}

// UpdateAccess refreshes the timestamp for a key.
func (e *TTLEviction) UpdateAccess(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.entries[key]; exists {
		e.entries[key] = time.Now()
	}
}

// Remove removes a key from the tracker.
func (e *TTLEviction) Remove(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.entries, key)
}

// Size returns the number of tracked entries.
func (e *TTLEviction) Size() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.entries)
}

// GetExpired returns all expired keys.
func (e *TTLEviction) GetExpired() []string {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	var expired []string
	for key, createdAt := range e.entries {
		if now.Sub(createdAt) > e.ttl {
			expired = append(expired, key)
		}
	}
	return expired
}

func (e *TTLEviction) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCleanup:
			return
		case <-ticker.C:
			expired := e.GetExpired()
			for _, key := range expired {
				e.Remove(key)
			}
		}
	}
}

// Stop stops the cleanup goroutine.
func (e *TTLEviction) Stop() {
	close(e.stopCleanup)
}

// LRUWithTTLEviction combines LRU and TTL eviction.
type LRUWithTTLEviction struct {
	lru         *LRUEviction
	ttl         *TTLEviction
	onEvict     func(key string)
	stopCleanup chan struct{}
}

// NewLRUWithTTLEviction creates a combined LRU+TTL eviction strategy.
func NewLRUWithTTLEviction(maxSize int, ttl time.Duration, onEvict func(key string)) *LRUWithTTLEviction {
	e := &LRUWithTTLEviction{
		lru:         NewLRUEviction(maxSize),
		ttl:         NewTTLEviction(ttl),
		onEvict:     onEvict,
		stopCleanup: make(chan struct{}),
	}
	e.ttl.Stop() // Stop the internal TTL cleanup
	go e.cleanupLoop()
	return e
}

// Add adds a key to both trackers.
func (e *LRUWithTTLEviction) Add(key string) string {
	e.ttl.Add(key)
	return e.lru.Add(key)
}

// UpdateAccess updates both trackers.
func (e *LRUWithTTLEviction) UpdateAccess(key string) {
	e.lru.UpdateAccess(key)
	e.ttl.UpdateAccess(key)
}

// Remove removes from both trackers.
func (e *LRUWithTTLEviction) Remove(key string) {
	e.lru.Remove(key)
	e.ttl.Remove(key)
}

// Size returns the number of tracked entries.
func (e *LRUWithTTLEviction) Size() int {
	return e.lru.Size()
}

func (e *LRUWithTTLEviction) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCleanup:
			return
		case <-ticker.C:
			expired := e.ttl.GetExpired()
			for _, key := range expired {
				e.Remove(key)
				if e.onEvict != nil {
					e.onEvict(key)
				}
			}
		}
	}
}

// Stop stops the cleanup goroutine.
func (e *LRUWithTTLEviction) Stop() {
	close(e.stopCleanup)
}

// RelevanceEviction implements relevance-based eviction using access frequency and recency.
type RelevanceEviction struct {
	mu          sync.Mutex
	maxSize     int
	decayFactor float64
	scores      map[string]float64
	lastDecay   time.Time
}

// NewRelevanceEviction creates a new relevance-based eviction strategy.
func NewRelevanceEviction(maxSize int, decayFactor float64) *RelevanceEviction {
	return &RelevanceEviction{
		maxSize:     maxSize,
		decayFactor: decayFactor,
		scores:      make(map[string]float64),
		lastDecay:   time.Now(),
	}
}

// Add adds a key with initial relevance score.
func (e *RelevanceEviction) Add(key string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.applyDecay()
	e.scores[key] = 1.0

	if len(e.scores) > e.maxSize {
		return e.evictLowest()
	}
	return ""
}

// UpdateAccess boosts the relevance score for a key.
func (e *RelevanceEviction) UpdateAccess(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.applyDecay()
	if _, exists := e.scores[key]; exists {
		e.scores[key] += 1.0
	}
}

// Remove removes a key from the tracker.
func (e *RelevanceEviction) Remove(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.scores, key)
}

// Size returns the number of tracked entries.
func (e *RelevanceEviction) Size() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.scores)
}

func (e *RelevanceEviction) applyDecay() {
	// Apply decay every minute
	if time.Since(e.lastDecay) < time.Minute {
		return
	}

	for k := range e.scores {
		e.scores[k] *= e.decayFactor
	}
	e.lastDecay = time.Now()
}

func (e *RelevanceEviction) evictLowest() string {
	if len(e.scores) == 0 {
		return ""
	}

	var lowestKey string
	lowestScore := float64(1<<62 - 1)

	for key, score := range e.scores {
		if score < lowestScore {
			lowestScore = score
			lowestKey = key
		}
	}

	delete(e.scores, lowestKey)
	return lowestKey
}

// GetScore returns the relevance score for a key.
func (e *RelevanceEviction) GetScore(key string) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.scores[key]
}
