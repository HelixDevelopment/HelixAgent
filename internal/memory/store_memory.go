package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/google/uuid"
)

// InMemoryStore provides an in-memory implementation of MemoryStore.
//
// Six safe.Stores replace the original sync.RWMutex + six bare maps.
// Compound operations (index updates that append an ID to the inner
// slice, Get's access-count mutation) run under Store.Update so both
// the outer map modification and the inner mutation share the same
// write lock. The original code's concurrent Get mutating
// memory.AccessCount under mu.RLock was a latent race; Update fixes
// it (Pattern Beta for the *Memory values).
type InMemoryStore struct {
	memories      *safe.Store[string, *Memory]
	entities      *safe.Store[string, *Entity]
	relationships *safe.Store[string, *Relationship]

	// Indexes for faster lookups
	userIndex    *safe.Store[string, []string] // userID -> memoryIDs
	sessionIndex *safe.Store[string, []string] // sessionID -> memoryIDs
	entityIndex  *safe.Store[string, []string] // entityID -> relationshipIDs
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		memories:      safe.NewStore[string, *Memory](),
		entities:      safe.NewStore[string, *Entity](),
		relationships: safe.NewStore[string, *Relationship](),
		userIndex:     safe.NewStore[string, []string](),
		sessionIndex:  safe.NewStore[string, []string](),
		entityIndex:   safe.NewStore[string, []string](),
	}
}

// Add adds a new memory
func (s *InMemoryStore) Add(ctx context.Context, memory *Memory) error {
	if memory.ID == "" {
		memory.ID = uuid.New().String()
	}

	s.memories.Put(memory.ID, memory)

	// Update indexes
	if memory.UserID != "" {
		s.userIndex.Update(memory.UserID, func(ids []string, _ bool) ([]string, bool) {
			return append(ids, memory.ID), true
		})
	}
	if memory.SessionID != "" {
		s.sessionIndex.Update(memory.SessionID, func(ids []string, _ bool) ([]string, bool) {
			return append(ids, memory.ID), true
		})
	}

	return nil
}

// Get retrieves a memory by ID
func (s *InMemoryStore) Get(ctx context.Context, id string) (*Memory, error) {
	var result *Memory
	var missing bool
	s.memories.Update(id, func(m *Memory, ok bool) (*Memory, bool) {
		if !ok {
			missing = true
			return nil, false
		}
		m.AccessCount++
		m.LastAccess = time.Now()
		result = m
		return m, true
	})

	if missing {
		return nil, fmt.Errorf("memory not found: %s", id)
	}
	return result, nil
}

// Update updates an existing memory
func (s *InMemoryStore) Update(ctx context.Context, memory *Memory) error {
	if !s.memories.Has(memory.ID) {
		return fmt.Errorf("memory not found: %s", memory.ID)
	}

	memory.UpdatedAt = time.Now()
	s.memories.Put(memory.ID, memory)

	return nil
}

// Delete removes a memory
func (s *InMemoryStore) Delete(ctx context.Context, id string) error {
	memory, existed := s.memories.Delete(id)
	if !existed {
		return fmt.Errorf("memory not found: %s", id)
	}

	// Remove from indexes
	if memory.UserID != "" {
		s.userIndex.Update(memory.UserID, func(ids []string, ok bool) ([]string, bool) {
			if !ok {
				return nil, false
			}
			next := removeFromSlice(ids, id)
			return next, len(next) > 0
		})
	}
	if memory.SessionID != "" {
		s.sessionIndex.Update(memory.SessionID, func(ids []string, ok bool) ([]string, bool) {
			if !ok {
				return nil, false
			}
			next := removeFromSlice(ids, id)
			return next, len(next) > 0
		})
	}

	return nil
}

// Search searches for relevant memories
func (s *InMemoryStore) Search(ctx context.Context, query string, opts *SearchOptions) ([]*Memory, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	var results []*Memory
	queryLower := strings.ToLower(query)

	s.memories.Range(func(_ string, memory *Memory) bool {
		// Filter by user
		if opts.UserID != "" && memory.UserID != opts.UserID {
			return true
		}

		// Filter by session
		if opts.SessionID != "" && memory.SessionID != opts.SessionID {
			return true
		}

		// Filter by type
		if opts.Type != "" && memory.Type != opts.Type {
			return true
		}

		// Filter by category
		if opts.Category != "" && memory.Category != opts.Category {
			return true
		}

		// Filter by time range
		if opts.TimeRange != nil {
			if memory.CreatedAt.Before(opts.TimeRange.Start) || memory.CreatedAt.After(opts.TimeRange.End) {
				return true
			}
		}

		// Simple text matching (in production, use vector similarity)
		score := s.calculateMatchScore(queryLower, memory)
		if score >= opts.MinScore {
			memoryCopy := *memory
			memoryCopy.Importance = score // Use importance as score for sorting
			results = append(results, &memoryCopy)
		}
		return true
	})

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Importance > results[j].Importance
	})

	// Limit results
	if opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	return results, nil
}

// GetByUser retrieves memories for a user
func (s *InMemoryStore) GetByUser(ctx context.Context, userID string, opts *ListOptions) ([]*Memory, error) {
	memoryIDs, _ := s.userIndex.Get(userID)
	if len(memoryIDs) == 0 {
		return []*Memory{}, nil
	}

	var results []*Memory
	for _, id := range memoryIDs {
		if memory, exists := s.memories.Get(id); exists {
			results = append(results, memory)
		}
	}

	// Sort
	if opts != nil && opts.SortBy != "" {
		s.sortMemories(results, opts.SortBy, opts.Order)
	}

	// Pagination
	if opts != nil {
		start := opts.Offset
		if start > len(results) {
			return []*Memory{}, nil
		}

		end := start + opts.Limit
		if end > len(results) || opts.Limit == 0 {
			end = len(results)
		}

		results = results[start:end]
	}

	return results, nil
}

// GetBySession retrieves memories for a session
func (s *InMemoryStore) GetBySession(ctx context.Context, sessionID string) ([]*Memory, error) {
	memoryIDs, _ := s.sessionIndex.Get(sessionID)
	if len(memoryIDs) == 0 {
		return []*Memory{}, nil
	}

	var results []*Memory
	for _, id := range memoryIDs {
		if memory, exists := s.memories.Get(id); exists {
			results = append(results, memory)
		}
	}

	// Sort by creation time
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})

	return results, nil
}

// AddEntity adds an entity
func (s *InMemoryStore) AddEntity(ctx context.Context, entity *Entity) error {
	if entity.ID == "" {
		entity.ID = uuid.New().String()
	}

	now := time.Now()
	entity.CreatedAt = now
	entity.UpdatedAt = now

	s.entities.Put(entity.ID, entity)
	return nil
}

// GetEntity retrieves an entity
func (s *InMemoryStore) GetEntity(ctx context.Context, id string) (*Entity, error) {
	entity, exists := s.entities.Get(id)
	if !exists {
		return nil, fmt.Errorf("entity not found: %s", id)
	}
	return entity, nil
}

// SearchEntities searches for entities
func (s *InMemoryStore) SearchEntities(ctx context.Context, query string, limit int) ([]*Entity, error) {
	queryLower := strings.ToLower(query)
	var results []*Entity

	s.entities.Range(func(_ string, entity *Entity) bool {
		if strings.Contains(strings.ToLower(entity.Name), queryLower) {
			results = append(results, entity)
		}
		return true
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// AddRelationship adds a relationship
func (s *InMemoryStore) AddRelationship(ctx context.Context, rel *Relationship) error {
	if rel.ID == "" {
		rel.ID = uuid.New().String()
	}

	now := time.Now()
	rel.CreatedAt = now
	rel.UpdatedAt = now

	s.relationships.Put(rel.ID, rel)

	// Update indexes
	appendID := func(ids []string, _ bool) ([]string, bool) {
		return append(ids, rel.ID), true
	}
	s.entityIndex.Update(rel.SourceID, appendID)
	s.entityIndex.Update(rel.TargetID, appendID)

	return nil
}

// GetRelationships gets relationships for an entity
func (s *InMemoryStore) GetRelationships(ctx context.Context, entityID string) ([]*Relationship, error) {
	relIDs, _ := s.entityIndex.Get(entityID)
	var results []*Relationship

	for _, id := range relIDs {
		if rel, exists := s.relationships.Get(id); exists {
			results = append(results, rel)
		}
	}

	return results, nil
}

// Helper functions

func (s *InMemoryStore) calculateMatchScore(query string, memory *Memory) float64 {
	contentLower := strings.ToLower(memory.Content)

	// Simple word overlap score
	queryWords := strings.Fields(query)
	if len(queryWords) == 0 {
		return 0
	}

	matches := 0
	for _, word := range queryWords {
		if strings.Contains(contentLower, word) {
			matches++
		}
	}

	return float64(matches) / float64(len(queryWords))
}

func (s *InMemoryStore) sortMemories(memories []*Memory, sortBy, order string) {
	sort.Slice(memories, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "created_at":
			less = memories[i].CreatedAt.Before(memories[j].CreatedAt)
		case "updated_at":
			less = memories[i].UpdatedAt.Before(memories[j].UpdatedAt)
		case "importance":
			less = memories[i].Importance < memories[j].Importance
		case "access_count":
			less = memories[i].AccessCount < memories[j].AccessCount
		default:
			less = memories[i].CreatedAt.Before(memories[j].CreatedAt)
		}

		if order == "desc" {
			return !less
		}
		return less
	})
}

func removeFromSlice(slice []string, item string) []string {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
