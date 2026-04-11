package gptcache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// LRUEviction Tests
// =============================================================================

func TestNewLRUEviction(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(10)
	assert.NotNil(t, e)
	assert.Equal(t, 0, e.Size())
}

func TestLRUEviction_Add_NoEviction(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(3)

	evicted := e.Add("key1")
	assert.Equal(t, "", evicted)
	assert.Equal(t, 1, e.Size())

	evicted = e.Add("key2")
	assert.Equal(t, "", evicted)
	assert.Equal(t, 2, e.Size())

	evicted = e.Add("key3")
	assert.Equal(t, "", evicted)
	assert.Equal(t, 3, e.Size())
}

func TestLRUEviction_Add_WithEviction(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(2)

	e.Add("key1")
	e.Add("key2")

	// This should evict key1 (oldest)
	evicted := e.Add("key3")
	assert.Equal(t, "key1", evicted)
	assert.Equal(t, 2, e.Size())
}

func TestLRUEviction_Add_ExistingKey(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(3)

	e.Add("key1")
	e.Add("key2")

	// Adding existing key should move it to front, not increase size
	evicted := e.Add("key1")
	assert.Equal(t, "", evicted)
	assert.Equal(t, 2, e.Size())
}

func TestLRUEviction_Add_ExistingKey_Prevents_Eviction(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(2)

	e.Add("key1")
	e.Add("key2")

	// Re-add key1 (moves to front)
	e.Add("key1")

	// Now key2 is oldest, so it should be evicted
	evicted := e.Add("key3")
	assert.Equal(t, "key2", evicted)
}

func TestLRUEviction_UpdateAccess(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(2)

	e.Add("key1")
	e.Add("key2")

	// Access key1 — makes it most recently used
	e.UpdateAccess("key1")

	// Now key2 is the LRU, should be evicted
	evicted := e.Add("key3")
	assert.Equal(t, "key2", evicted)
}

func TestLRUEviction_UpdateAccess_NonExistent(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(5)
	// Should not panic
	e.UpdateAccess("nonexistent")
}

func TestLRUEviction_Remove_Dedicated(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(5)

	e.Add("key1")
	e.Add("key2")
	assert.Equal(t, 2, e.Size())

	e.Remove("key1")
	assert.Equal(t, 1, e.Size())
}

func TestLRUEviction_Remove_NonExistent(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(5)
	e.Add("key1")

	// Should not panic
	e.Remove("nonexistent")
	assert.Equal(t, 1, e.Size())
}

func TestLRUEviction_Size(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(10)
	assert.Equal(t, 0, e.Size())

	e.Add("a")
	e.Add("b")
	e.Add("c")
	assert.Equal(t, 3, e.Size())

	e.Remove("b")
	assert.Equal(t, 2, e.Size())
}

func TestLRUEviction_ImplementsEvictionStrategy(t *testing.T) {
	t.Parallel()
	var _ EvictionStrategy = (*LRUEviction)(nil)
}

func TestLRUEviction_Concurrent(t *testing.T) {
	t.Parallel()
	e := NewLRUEviction(100)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "key"
			e.Add(key)
			e.UpdateAccess(key)
			e.Size()
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// TTLEviction Tests
// =============================================================================

func TestNewTTLEviction(t *testing.T) {
	t.Parallel()
	e := NewTTLEviction(5 * time.Minute)
	defer e.Stop()

	assert.NotNil(t, e)
	assert.Equal(t, 0, e.Size())
}

func TestTTLEviction_Add(t *testing.T) {
	t.Parallel()
	e := NewTTLEviction(5 * time.Minute)
	defer e.Stop()

	evicted := e.Add("key1")
	assert.Equal(t, "", evicted) // TTL never evicts on add
	assert.Equal(t, 1, e.Size())
}

func TestTTLEviction_UpdateAccess_Dedicated(t *testing.T) {
	t.Parallel()
	e := NewTTLEviction(5 * time.Minute)
	defer e.Stop()

	e.Add("key1")
	e.UpdateAccess("key1")
	assert.Equal(t, 1, e.Size())
}

func TestTTLEviction_UpdateAccess_NonExistent(t *testing.T) {
	t.Parallel()
	e := NewTTLEviction(5 * time.Minute)
	defer e.Stop()

	// Should not panic or add the key
	e.UpdateAccess("nonexistent")
	assert.Equal(t, 0, e.Size())
}

func TestTTLEviction_Remove_Dedicated(t *testing.T) {
	t.Parallel()
	e := NewTTLEviction(5 * time.Minute)
	defer e.Stop()

	e.Add("key1")
	e.Add("key2")
	assert.Equal(t, 2, e.Size())

	e.Remove("key1")
	assert.Equal(t, 1, e.Size())
}

func TestTTLEviction_GetExpired(t *testing.T) {
	t.Parallel()
	e := NewTTLEviction(10 * time.Millisecond)
	defer e.Stop()

	e.Add("key1")
	e.Add("key2")

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	expired := e.GetExpired()
	assert.Len(t, expired, 2)
}

func TestTTLEviction_GetExpired_NoneExpired(t *testing.T) {
	t.Parallel()
	e := NewTTLEviction(10 * time.Minute)
	defer e.Stop()

	e.Add("key1")
	e.Add("key2")

	expired := e.GetExpired()
	assert.Empty(t, expired)
}

func TestTTLEviction_GetExpired_PartiallyExpired(t *testing.T) {
	t.Parallel()
	e := NewTTLEviction(30 * time.Millisecond)
	defer e.Stop()

	e.Add("old")
	time.Sleep(40 * time.Millisecond) // old expires
	e.Add("new")

	expired := e.GetExpired()
	assert.Len(t, expired, 1)
	assert.Equal(t, "old", expired[0])
}

func TestTTLEviction_Stop(t *testing.T) {
	t.Parallel()
	e := NewTTLEviction(time.Minute)
	e.Stop() // Should not panic or block
}

func TestTTLEviction_ImplementsEvictionStrategy(t *testing.T) {
	t.Parallel()
	var _ EvictionStrategy = (*TTLEviction)(nil)
}

// =============================================================================
// LRUWithTTLEviction Tests
// =============================================================================

func TestNewLRUWithTTLEviction(t *testing.T) {
	t.Parallel()
	var evictedKeys []string
	e := NewLRUWithTTLEviction(10, 5*time.Minute, func(key string) {
		evictedKeys = append(evictedKeys, key)
	})
	defer e.Stop()

	assert.NotNil(t, e)
	assert.Equal(t, 0, e.Size())
}

func TestLRUWithTTLEviction_Add_NoEviction(t *testing.T) {
	t.Parallel()
	e := NewLRUWithTTLEviction(5, 5*time.Minute, nil)
	defer e.Stop()

	evicted := e.Add("key1")
	assert.Equal(t, "", evicted)
	assert.Equal(t, 1, e.Size())
}

func TestLRUWithTTLEviction_Add_LRUEviction(t *testing.T) {
	t.Parallel()
	e := NewLRUWithTTLEviction(2, 5*time.Minute, nil)
	defer e.Stop()

	e.Add("key1")
	e.Add("key2")

	evicted := e.Add("key3")
	assert.Equal(t, "key1", evicted)
	assert.Equal(t, 2, e.Size())
}

func TestLRUWithTTLEviction_UpdateAccess(t *testing.T) {
	t.Parallel()
	e := NewLRUWithTTLEviction(2, 5*time.Minute, nil)
	defer e.Stop()

	e.Add("key1")
	e.Add("key2")
	e.UpdateAccess("key1")

	// key2 should be evicted since key1 was accessed more recently
	evicted := e.Add("key3")
	assert.Equal(t, "key2", evicted)
}

func TestLRUWithTTLEviction_Remove(t *testing.T) {
	t.Parallel()
	e := NewLRUWithTTLEviction(10, 5*time.Minute, nil)
	defer e.Stop()

	e.Add("key1")
	e.Add("key2")
	assert.Equal(t, 2, e.Size())

	e.Remove("key1")
	assert.Equal(t, 1, e.Size())
}

func TestLRUWithTTLEviction_Stop(t *testing.T) {
	t.Parallel()
	e := NewLRUWithTTLEviction(10, time.Minute, nil)
	e.Stop() // Should not panic
}

func TestLRUWithTTLEviction_ImplementsEvictionStrategy(t *testing.T) {
	t.Parallel()
	var _ EvictionStrategy = (*LRUWithTTLEviction)(nil)
}

// =============================================================================
// RelevanceEviction Tests
// =============================================================================

func TestNewRelevanceEviction(t *testing.T) {
	t.Parallel()
	e := NewRelevanceEviction(10, 0.9)
	assert.NotNil(t, e)
	assert.Equal(t, 0, e.Size())
}

func TestRelevanceEviction_Add_NoEviction(t *testing.T) {
	t.Parallel()
	e := NewRelevanceEviction(5, 0.9)

	evicted := e.Add("key1")
	assert.Equal(t, "", evicted)
	assert.Equal(t, 1, e.Size())
}

func TestRelevanceEviction_Add_WithEviction(t *testing.T) {
	t.Parallel()
	e := NewRelevanceEviction(2, 0.9)

	e.Add("key1")
	e.Add("key2")

	// Third key should evict the lowest-scored key
	evicted := e.Add("key3")
	assert.NotEmpty(t, evicted)
	assert.Equal(t, 2, e.Size())
}

func TestRelevanceEviction_UpdateAccess(t *testing.T) {
	t.Parallel()
	e := NewRelevanceEviction(3, 0.9)

	e.Add("key1")          // score = 1.0
	e.Add("key2")          // score = 1.0
	e.UpdateAccess("key1") // score = 2.0

	score := e.GetScore("key1")
	assert.Greater(t, score, e.GetScore("key2"))
}

func TestRelevanceEviction_UpdateAccess_NonExistent(t *testing.T) {
	t.Parallel()
	e := NewRelevanceEviction(5, 0.9)
	// Should not panic or add key
	e.UpdateAccess("nonexistent")
	assert.Equal(t, 0, e.Size())
}

func TestRelevanceEviction_Remove_Dedicated(t *testing.T) {
	t.Parallel()
	e := NewRelevanceEviction(5, 0.9)

	e.Add("key1")
	e.Add("key2")
	assert.Equal(t, 2, e.Size())

	e.Remove("key1")
	assert.Equal(t, 1, e.Size())
}

func TestRelevanceEviction_GetScore_Existing(t *testing.T) {
	t.Parallel()
	e := NewRelevanceEviction(5, 0.9)
	e.Add("key1")

	score := e.GetScore("key1")
	assert.Equal(t, 1.0, score)
}

func TestRelevanceEviction_GetScore_NonExistent(t *testing.T) {
	t.Parallel()
	e := NewRelevanceEviction(5, 0.9)
	score := e.GetScore("nonexistent")
	assert.Equal(t, float64(0), score)
}

func TestRelevanceEviction_EvictsLowestScore(t *testing.T) {
	t.Parallel()
	e := NewRelevanceEviction(2, 0.9)

	e.Add("frequent")
	e.Add("rare")

	// Boost "frequent" score significantly
	e.UpdateAccess("frequent")
	e.UpdateAccess("frequent")
	e.UpdateAccess("frequent")

	// Boost "rare" once so it's at 2.0 while "new" will be at 1.0
	e.UpdateAccess("rare")

	// Adding a new key should evict "new" (lowest score at 1.0)
	evicted := e.Add("new")
	assert.Equal(t, "new", evicted)
}

func TestRelevanceEviction_ImplementsEvictionStrategy(t *testing.T) {
	t.Parallel()
	var _ EvictionStrategy = (*RelevanceEviction)(nil)
}

func TestRelevanceEviction_Concurrent(t *testing.T) {
	t.Parallel()
	e := NewRelevanceEviction(100, 0.9)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "key"
			e.Add(key)
			e.UpdateAccess(key)
			e.GetScore(key)
			e.Size()
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// EvictionStrategy Interface Compliance Tests
// =============================================================================

func TestEvictionStrategy_AllImplementations(t *testing.T) {
	t.Parallel()
	strategies := []struct {
		name     string
		strategy EvictionStrategy
		cleanup  func()
	}{
		{
			name:     "LRU",
			strategy: NewLRUEviction(5),
			cleanup:  func() {},
		},
		{
			name: "TTL",
			strategy: func() EvictionStrategy {
				e := NewTTLEviction(time.Minute)
				return e
			}(),
			cleanup: func() {},
		},
		{
			name: "LRUWithTTL",
			strategy: func() EvictionStrategy {
				e := NewLRUWithTTLEviction(5, time.Minute, nil)
				return e
			}(),
			cleanup: func() {},
		},
		{
			name:     "Relevance",
			strategy: NewRelevanceEviction(5, 0.9),
			cleanup:  func() {},
		},
	}

	for _, tt := range strategies {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := tt.strategy

			// Add
			evicted := s.Add("a")
			require.Equal(t, "", evicted)
			assert.Equal(t, 1, s.Size())

			// UpdateAccess
			s.UpdateAccess("a")
			assert.Equal(t, 1, s.Size())

			// Remove
			s.Remove("a")
			assert.Equal(t, 0, s.Size())

			tt.cleanup()
		})
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkLRUEviction_Add(b *testing.B) {
	e := NewLRUEviction(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Add("key")
	}
}

func BenchmarkLRUEviction_AddEvict(b *testing.B) {
	e := NewLRUEviction(100)
	// Pre-fill
	for i := 0; i < 100; i++ {
		e.Add(string(rune(i)))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Add("new-key")
	}
}

func BenchmarkRelevanceEviction_Add(b *testing.B) {
	e := NewRelevanceEviction(1000, 0.9)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Add("key")
	}
}

func BenchmarkRelevanceEviction_UpdateAccess(b *testing.B) {
	e := NewRelevanceEviction(100, 0.9)
	e.Add("key")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.UpdateAccess("key")
	}
}
