package cache

import (
	"context"
	"sync"
	"sync/atomic"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/adapters"
)

// Type aliases for backward compatibility
type EventType = adapters.EventType
type Event = adapters.Event

// InvalidationStrategy defines how cache is invalidated
type InvalidationStrategy interface {
	// ShouldInvalidate returns keys that should be invalidated for an event
	ShouldInvalidate(event *Event) []string
	// Name returns the strategy name
	Name() string
}

// InvalidationRule defines a rule for event-driven invalidation
type InvalidationRule struct {
	// EventType to match
	EventType EventType
	// KeyPattern to invalidate (supports * wildcard)
	KeyPattern string
	// Tags to invalidate
	Tags []string
	// Custom handler for complex invalidation logic
	Handler func(event *Event) []string
}

// TagBasedInvalidation provides tag-based cache invalidation.
//
// Concurrent-safe by construction (CONST-029): the (tagIndex, keyTags)
// pair is held under a sentinel key in a single safe.Store so AddTag
// and RemoveKey can mutate both maps atomically inside one Update
// callback (Pattern Epsilon — joint atomicity via state-struct).
type TagBasedInvalidation struct {
	state   *safe.Store[string, *tagState]
	metrics *InvalidationMetrics
}

// tagState is the joint state held under tagStateKey.
type tagState struct {
	tagIndex map[string]map[string]struct{} // tag -> keys
	keyTags  map[string][]string            // key -> tags
}

const tagStateKey = "_"

func newTagState() *tagState {
	return &tagState{
		tagIndex: make(map[string]map[string]struct{}),
		keyTags:  make(map[string][]string),
	}
}

// InvalidationMetrics tracks invalidation statistics
type InvalidationMetrics struct {
	TotalInvalidations   int64
	TagInvalidations     int64
	PatternInvalidations int64
	EventInvalidations   int64
	KeysInvalidated      int64
}

// NewTagBasedInvalidation creates a new tag-based invalidation strategy
func NewTagBasedInvalidation() *TagBasedInvalidation {
	store := safe.NewStore[string, *tagState]()
	store.Put(tagStateKey, newTagState())
	return &TagBasedInvalidation{
		state:   store,
		metrics: &InvalidationMetrics{},
	}
}

// withState runs fn under the Store's write lock; fn may mutate the maps.
func (i *TagBasedInvalidation) withState(fn func(*tagState)) {
	i.state.Update(tagStateKey, func(s *tagState, _ bool) (*tagState, bool) {
		if s == nil {
			s = newTagState()
		}
		fn(s)
		return s, true
	})
}

// AddTag associates tags with a cache key
func (i *TagBasedInvalidation) AddTag(key string, tags ...string) {
	i.withState(func(s *tagState) {
		s.keyTags[key] = append(s.keyTags[key], tags...)
		for _, tag := range tags {
			if s.tagIndex[tag] == nil {
				s.tagIndex[tag] = make(map[string]struct{})
			}
			s.tagIndex[tag][key] = struct{}{}
		}
	})
}

// RemoveKey removes a key from all tag associations
func (i *TagBasedInvalidation) RemoveKey(key string) {
	i.withState(func(s *tagState) {
		tags := s.keyTags[key]
		delete(s.keyTags, key)
		for _, tag := range tags {
			if keys, exists := s.tagIndex[tag]; exists {
				delete(keys, key)
				if len(keys) == 0 {
					delete(s.tagIndex, tag)
				}
			}
		}
	})
}

// InvalidateByTag returns all keys with the given tag
func (i *TagBasedInvalidation) InvalidateByTag(tag string) []string {
	var result []string
	i.withState(func(s *tagState) {
		keys := s.tagIndex[tag]
		if keys == nil {
			return
		}
		result = make([]string, 0, len(keys))
		for key := range keys {
			result = append(result, key)
		}
	})
	if result == nil {
		return nil
	}
	atomic.AddInt64(&i.metrics.TagInvalidations, 1)
	atomic.AddInt64(&i.metrics.KeysInvalidated, int64(len(result)))
	return result
}

// InvalidateByTags returns all keys with any of the given tags
func (i *TagBasedInvalidation) InvalidateByTags(tags ...string) []string {
	keySet := make(map[string]struct{})

	for _, tag := range tags {
		keys := i.InvalidateByTag(tag)
		for _, key := range keys {
			keySet[key] = struct{}{}
		}
	}

	result := make([]string, 0, len(keySet))
	for key := range keySet {
		result = append(result, key)
	}

	return result
}

// GetTags returns all tags for a key
func (i *TagBasedInvalidation) GetTags(key string) []string {
	var result []string
	i.withState(func(s *tagState) {
		tags := s.keyTags[key]
		if tags == nil {
			return
		}
		result = make([]string, len(tags))
		copy(result, tags)
	})
	return result
}

// GetKeys returns all keys for a tag
func (i *TagBasedInvalidation) GetKeys(tag string) []string {
	var result []string
	i.withState(func(s *tagState) {
		keys := s.tagIndex[tag]
		if keys == nil {
			return
		}
		result = make([]string, 0, len(keys))
		for key := range keys {
			result = append(result, key)
		}
	})
	return result
}

// Metrics returns current metrics
func (i *TagBasedInvalidation) Metrics() *InvalidationMetrics {
	return &InvalidationMetrics{
		TotalInvalidations:   atomic.LoadInt64(&i.metrics.TotalInvalidations),
		TagInvalidations:     atomic.LoadInt64(&i.metrics.TagInvalidations),
		PatternInvalidations: atomic.LoadInt64(&i.metrics.PatternInvalidations),
		EventInvalidations:   atomic.LoadInt64(&i.metrics.EventInvalidations),
		KeysInvalidated:      atomic.LoadInt64(&i.metrics.KeysInvalidated),
	}
}

// ShouldInvalidate implements InvalidationStrategy
func (i *TagBasedInvalidation) ShouldInvalidate(event *Event) []string {
	return nil // Tag-based invalidation is explicit, not event-driven
}

// Name implements InvalidationStrategy
func (i *TagBasedInvalidation) Name() string {
	return "tag-based"
}

// EventBus alias for backward compatibility
type EventBus = adapters.EventBus

// EventDrivenInvalidation invalidates cache based on system events.
//
// Concurrent-safe by construction (CONST-029): rules is a safe.Store
// keyed by EventType; per-event rule slices are append-and-replace via
// Update (Pattern Beta).
type EventDrivenInvalidation struct {
	eventBus *EventBus
	cache    *TieredCache
	rules    *safe.Store[EventType, []InvalidationRule]
	metrics  *InvalidationMetrics
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewEventDrivenInvalidation creates a new event-driven invalidation strategy
func NewEventDrivenInvalidation(eventBus *EventBus, cache *TieredCache) *EventDrivenInvalidation {
	ctx, cancel := context.WithCancel(context.Background())

	edi := &EventDrivenInvalidation{
		eventBus: eventBus,
		cache:    cache,
		rules:    safe.NewStore[EventType, []InvalidationRule](),
		metrics:  &InvalidationMetrics{},
		ctx:      ctx,
		cancel:   cancel,
	}

	// Register default invalidation rules
	edi.registerDefaultRules()

	return edi
}

// Start starts listening for events
func (i *EventDrivenInvalidation) Start() {
	if i.eventBus == nil {
		return
	}

	// Subscribe to all events
	ch := i.eventBus.SubscribeAll()

	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		for {
			select {
			case <-i.ctx.Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				i.handleEvent(i.ctx, event)
			}
		}
	}()
}

// Stop stops the event listener and waits for the goroutine to exit
func (i *EventDrivenInvalidation) Stop() {
	i.cancel()
	i.wg.Wait()
}

// AddRule adds an invalidation rule
func (i *EventDrivenInvalidation) AddRule(rule InvalidationRule) {
	i.rules.Update(rule.EventType, func(curr []InvalidationRule, _ bool) ([]InvalidationRule, bool) {
		return append(curr, rule), true
	})
}

// RemoveRules removes all rules for an event type
func (i *EventDrivenInvalidation) RemoveRules(eventType EventType) {
	i.rules.Delete(eventType)
}

func (i *EventDrivenInvalidation) registerDefaultRules() {
	// Provider health change invalidates provider cache
	i.AddRule(InvalidationRule{
		EventType:  adapters.EventProviderHealthChanged,
		KeyPattern: "provider:*",
		Tags:       []string{"provider", "health"},
		Handler: func(event *Event) []string {
			if payload, ok := event.Payload.(map[string]interface{}); ok {
				if name, ok := payload["name"].(string); ok {
					return []string{"provider:" + name, "health:" + name}
				}
			}
			return nil
		},
	})

	// MCP server disconnect invalidates MCP cache
	i.AddRule(InvalidationRule{
		EventType:  adapters.EventMCPServerDisconnected,
		KeyPattern: "mcp:*",
		Tags:       []string{"mcp"},
		Handler: func(event *Event) []string {
			if payload, ok := event.Payload.(map[string]interface{}); ok {
				if server, ok := payload["server"].(string); ok {
					return []string{"mcp:" + server + ":*"}
				}
			}
			return nil
		},
	})

	// Cache invalidation event
	i.AddRule(InvalidationRule{
		EventType: adapters.EventCacheInvalidated,
		Handler: func(event *Event) []string {
			if payload, ok := event.Payload.(map[string]interface{}); ok {
				if keys, ok := payload["keys"].([]string); ok {
					return keys
				}
				if pattern, ok := payload["pattern"].(string); ok {
					return []string{pattern}
				}
			}
			return nil
		},
	})
}

func (i *EventDrivenInvalidation) handleEvent(ctx context.Context, event *Event) {
	rules, _ := i.rules.Get(event.Type)
	if len(rules) == 0 {
		return
	}

	keysToInvalidate := make(map[string]struct{})

	for _, rule := range rules {
		// Get keys from handler if present
		if rule.Handler != nil {
			keys := rule.Handler(event)
			for _, key := range keys {
				keysToInvalidate[key] = struct{}{}
			}
		}

		// Add pattern-based keys
		if rule.KeyPattern != "" {
			keysToInvalidate[rule.KeyPattern] = struct{}{}
		}
	}

	// Invalidate all collected keys
	for key := range keysToInvalidate {
		// Check if it's a pattern (contains *)
		if containsWildcard(key) {
			_, _ = i.cache.InvalidatePrefix(ctx, trimWildcard(key)) //nolint:errcheck
		} else {
			_ = i.cache.Delete(ctx, key) //nolint:errcheck
		}
	}

	atomic.AddInt64(&i.metrics.EventInvalidations, 1)
	atomic.AddInt64(&i.metrics.KeysInvalidated, int64(len(keysToInvalidate)))
}

// ShouldInvalidate implements InvalidationStrategy
func (i *EventDrivenInvalidation) ShouldInvalidate(event *Event) []string {
	rules, _ := i.rules.Get(event.Type)
	if len(rules) == 0 {
		return nil
	}

	var keys []string
	for _, rule := range rules {
		if rule.Handler != nil {
			keys = append(keys, rule.Handler(event)...)
		}
		if rule.KeyPattern != "" {
			keys = append(keys, rule.KeyPattern)
		}
	}

	return keys
}

// Name implements InvalidationStrategy
func (i *EventDrivenInvalidation) Name() string {
	return "event-driven"
}

// Metrics returns current metrics
func (i *EventDrivenInvalidation) Metrics() *InvalidationMetrics {
	return &InvalidationMetrics{
		TotalInvalidations:   atomic.LoadInt64(&i.metrics.TotalInvalidations),
		TagInvalidations:     atomic.LoadInt64(&i.metrics.TagInvalidations),
		PatternInvalidations: atomic.LoadInt64(&i.metrics.PatternInvalidations),
		EventInvalidations:   atomic.LoadInt64(&i.metrics.EventInvalidations),
		KeysInvalidated:      atomic.LoadInt64(&i.metrics.KeysInvalidated),
	}
}

// Helper functions

func containsWildcard(s string) bool {
	for _, c := range s {
		if c == '*' {
			return true
		}
	}
	return false
}

func trimWildcard(s string) string {
	for i, c := range s {
		if c == '*' {
			return s[:i]
		}
	}
	return s
}

// CompositeInvalidation combines multiple invalidation strategies
type CompositeInvalidation struct {
	strategies []InvalidationStrategy
	metrics    *InvalidationMetrics
}

// NewCompositeInvalidation creates a composite invalidation strategy
func NewCompositeInvalidation(strategies ...InvalidationStrategy) *CompositeInvalidation {
	return &CompositeInvalidation{
		strategies: strategies,
		metrics:    &InvalidationMetrics{},
	}
}

// ShouldInvalidate returns keys from all strategies
func (c *CompositeInvalidation) ShouldInvalidate(event *Event) []string {
	keySet := make(map[string]struct{})

	for _, strategy := range c.strategies {
		keys := strategy.ShouldInvalidate(event)
		for _, key := range keys {
			keySet[key] = struct{}{}
		}
	}

	result := make([]string, 0, len(keySet))
	for key := range keySet {
		result = append(result, key)
	}

	return result
}

// Name implements InvalidationStrategy
func (c *CompositeInvalidation) Name() string {
	return "composite"
}

// AddStrategy adds a strategy to the composite
func (c *CompositeInvalidation) AddStrategy(strategy InvalidationStrategy) {
	c.strategies = append(c.strategies, strategy)
}
