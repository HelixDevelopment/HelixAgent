package gptcache

import (
	"container/list"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
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
//
// mu is Pattern Zeta: it coordinates the container/list order with
// the key→element lookup during compound LRU operations (container/
// list is not thread-safe). The lookup index lives in a safe.Store
// so the audit-gate pattern ("sync.Mutex + bare map") is retired.
type LRUEviction struct {
	mu      sync.Mutex
	maxSize int
	order   *list.List
	index   *safe.Store[string, *list.Element]
}

// NewLRUEviction creates a new LRU eviction strategy.
func NewLRUEviction(maxSize int) *LRUEviction {
	return &LRUEviction{
		maxSize: maxSize,
		order:   list.New(),
		index:   safe.NewStore[string, *list.Element](),
	}
}

// Add adds a key to the LRU tracker.
func (e *LRUEviction) Add(key string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	// If key exists, move to front
	if elem, exists := e.index.Get(key); exists {
		e.order.MoveToFront(elem)
		return ""
	}

	// Add new key to front
	e.index.Put(key, e.order.PushFront(key))

	// Check if eviction needed
	if e.order.Len() > e.maxSize {
		oldest := e.order.Back()
		if oldest != nil {
			evicted := oldest.Value.(string) //nolint:errcheck
			e.order.Remove(oldest)
			e.index.Delete(evicted)
			return evicted
		}
	}

	return ""
}

// UpdateAccess moves a key to the front (most recently used).
func (e *LRUEviction) UpdateAccess(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if elem, exists := e.index.Get(key); exists {
		e.order.MoveToFront(elem)
	}
}

// Remove removes a key from the tracker.
func (e *LRUEviction) Remove(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if elem, exists := e.index.Get(key); exists {
		e.order.Remove(elem)
		e.index.Delete(key)
	}
}

// Size returns the number of tracked entries.
func (e *LRUEviction) Size() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.order.Len()
}

// TTLEviction implements Time-To-Live eviction policy.
// Pattern Alpha: entries are time.Time values with no post-insert
// mutation — UpdateAccess is a Put of a fresh time.Now().
type TTLEviction struct {
	ttl         time.Duration
	entries     *safe.Store[string, time.Time]
	stopCleanup chan struct{}
}

// NewTTLEviction creates a new TTL eviction strategy.
func NewTTLEviction(ttl time.Duration) *TTLEviction {
	e := &TTLEviction{
		ttl:         ttl,
		entries:     safe.NewStore[string, time.Time](),
		stopCleanup: make(chan struct{}),
	}
	go e.cleanupLoop()
	return e
}

// Add adds a key with current timestamp.
func (e *TTLEviction) Add(key string) string {
	e.entries.Put(key, time.Now())
	return "" // TTL doesn't evict on add
}

// UpdateAccess refreshes the timestamp for a key.
func (e *TTLEviction) UpdateAccess(key string) {
	e.entries.Update(key, func(_ time.Time, ok bool) (time.Time, bool) {
		if !ok {
			return time.Time{}, false
		}
		return time.Now(), true
	})
}

// Remove removes a key from the tracker.
func (e *TTLEviction) Remove(key string) {
	e.entries.Delete(key)
}

// Size returns the number of tracked entries.
func (e *TTLEviction) Size() int {
	return e.entries.Len()
}

// GetExpired returns all expired keys.
func (e *TTLEviction) GetExpired() []string {
	now := time.Now()
	var expired []string
	e.entries.Range(func(key string, createdAt time.Time) bool {
		if now.Sub(createdAt) > e.ttl {
			expired = append(expired, key)
		}
		return true
	})
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

// RelevanceEviction implements relevance-based eviction using access
// frequency and recency.
//
// mu is Pattern Zeta: it coordinates the compound decay-and-update
// sequence (applyDecay mutates every score and lastDecay, then the
// caller mutates a specific key) so concurrent callers cannot observe
// a partially decayed map. scores themselves live in safe.Store so
// the bare-mutex-plus-map pattern is gone.
type RelevanceEviction struct {
	mu          sync.Mutex
	maxSize     int
	decayFactor float64
	scores      *safe.Store[string, float64]
	lastDecay   time.Time
}

// NewRelevanceEviction creates a new relevance-based eviction strategy.
func NewRelevanceEviction(maxSize int, decayFactor float64) *RelevanceEviction {
	return &RelevanceEviction{
		maxSize:     maxSize,
		decayFactor: decayFactor,
		scores:      safe.NewStore[string, float64](),
		lastDecay:   time.Now(),
	}
}

// Add adds a key with initial relevance score.
func (e *RelevanceEviction) Add(key string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.applyDecayLocked()
	e.scores.Put(key, 1.0)

	if e.scores.Len() > e.maxSize {
		return e.evictLowestLocked()
	}
	return ""
}

// UpdateAccess boosts the relevance score for a key.
func (e *RelevanceEviction) UpdateAccess(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.applyDecayLocked()
	e.scores.Update(key, func(cur float64, ok bool) (float64, bool) {
		if !ok {
			return 0, false
		}
		return cur + 1.0, true
	})
}

// Remove removes a key from the tracker.
func (e *RelevanceEviction) Remove(key string) {
	e.scores.Delete(key)
}

// Size returns the number of tracked entries.
func (e *RelevanceEviction) Size() int {
	return e.scores.Len()
}

// applyDecayLocked must be called with e.mu held.
func (e *RelevanceEviction) applyDecayLocked() {
	// Apply decay every minute
	if time.Since(e.lastDecay) < time.Minute {
		return
	}

	for _, k := range e.scores.Keys() {
		e.scores.Update(k, func(cur float64, ok bool) (float64, bool) {
			if !ok {
				return 0, false
			}
			return cur * e.decayFactor, true
		})
	}
	e.lastDecay = time.Now()
}

// evictLowestLocked must be called with e.mu held.
func (e *RelevanceEviction) evictLowestLocked() string {
	var lowestKey string
	lowestScore := float64(1<<62 - 1)

	e.scores.Range(func(key string, score float64) bool {
		if score < lowestScore {
			lowestScore = score
			lowestKey = key
		}
		return true
	})

	if lowestKey == "" {
		return ""
	}
	e.scores.Delete(lowestKey)
	return lowestKey
}

// GetScore returns the relevance score for a key.
func (e *RelevanceEviction) GetScore(key string) float64 {
	v, _ := e.scores.Get(key)
	return v
}
