package stress

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/memory"
)

// TestMemoryStore_HighWriteVolume writes 1000 memories to InMemoryStore
// and verifies they are all stored correctly without data loss or corruption.
func TestMemoryStore_HighWriteVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	store := memory.NewInMemoryStore()
	ctx := context.Background()

	const totalMemories = 1000
	var writeErrors int64

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	start := time.Now()

	for i := 0; i < totalMemories; i++ {
		mem := &memory.Memory{
			ID:         fmt.Sprintf("mem-%d", i),
			UserID:     fmt.Sprintf("user-%d", i%10),
			SessionID:  fmt.Sprintf("session-%d", i%20),
			Content:    fmt.Sprintf("Memory content for item %d with some additional text to simulate real data", i),
			Type:       memory.MemoryTypeEpisodic,
			Category:   fmt.Sprintf("category-%d", i%5),
			Importance: float64(i%100) / 100.0,
			Metadata: map[string]interface{}{
				"index": i,
				"batch": i / 100,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := store.Add(ctx, mem); err != nil {
			atomic.AddInt64(&writeErrors, 1)
		}
	}

	elapsed := time.Since(start)

	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	liveHeapMB := float64(memAfter.HeapInuse) / 1024 / 1024

	t.Logf("High write volume: %d writes in %v (%.0f writes/sec), "+
		"errors=%d, live heap=%.2f MB",
		totalMemories, elapsed,
		float64(totalMemories)/elapsed.Seconds(),
		writeErrors, liveHeapMB)

	assert.Zero(t, writeErrors, "all writes should succeed")

	// Verify all memories are retrievable
	for i := 0; i < totalMemories; i++ {
		mem, err := store.Get(ctx, fmt.Sprintf("mem-%d", i))
		require.NoError(t, err, "memory %d should be retrievable", i)
		assert.Equal(t, fmt.Sprintf("user-%d", i%10), mem.UserID)
	}

	// Verify user index works correctly
	userMemories, err := store.GetByUser(ctx, "user-0", nil)
	require.NoError(t, err)
	assert.Equal(t, totalMemories/10, len(userMemories),
		"user-0 should have exactly %d memories", totalMemories/10)
}

// TestMemoryStore_ConcurrentReadWrite tests concurrent reads and writes
// from 100 goroutines against the InMemoryStore, verifying no panics,
// data races, or deadlocks occur.
func TestMemoryStore_ConcurrentReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	store := memory.NewInMemoryStore()
	ctx := context.Background()

	// Pre-populate with some data
	for i := 0; i < 100; i++ {
		_ = store.Add(ctx, &memory.Memory{
			ID:         fmt.Sprintf("pre-mem-%d", i),
			UserID:     fmt.Sprintf("user-%d", i%5),
			Content:    fmt.Sprintf("Pre-populated memory %d", i),
			Type:       memory.MemoryTypeEpisodic,
			Importance: 0.5,
			Metadata:   map[string]interface{}{},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})
	}

	const goroutineCount = 100
	var wg sync.WaitGroup
	var panics int64
	var reads int64
	var writes int64
	var updates int64
	var deletes int64

	startSignal := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-startSignal

			for j := 0; j < 50; j++ {
				switch j % 4 {
				case 0: // Write
					mem := &memory.Memory{
						ID:         fmt.Sprintf("conc-mem-%d-%d", id, j),
						UserID:     fmt.Sprintf("user-%d", id%5),
						Content:    fmt.Sprintf("Concurrent memory %d-%d", id, j),
						Type:       memory.MemoryTypeSemantic,
						Importance: float64(j) / 50.0,
						Metadata:   map[string]interface{}{},
						CreatedAt:  time.Now(),
						UpdatedAt:  time.Now(),
					}
					_ = store.Add(ctx, mem)
					atomic.AddInt64(&writes, 1)

				case 1: // Read
					_, _ = store.Get(ctx, fmt.Sprintf("pre-mem-%d", id%100))
					atomic.AddInt64(&reads, 1)

				case 2: // Update
					mem := &memory.Memory{
						ID:         fmt.Sprintf("pre-mem-%d", id%100),
						UserID:     fmt.Sprintf("user-%d", id%5),
						Content:    fmt.Sprintf("Updated content %d-%d", id, j),
						Type:       memory.MemoryTypeEpisodic,
						Importance: float64(j) / 50.0,
						Metadata:   map[string]interface{}{},
						UpdatedAt:  time.Now(),
					}
					_ = store.Update(ctx, mem)
					atomic.AddInt64(&updates, 1)

				case 3: // Search
					_, _ = store.Search(ctx, "memory", &memory.SearchOptions{
						TopK:     5,
						MinScore: 0.1,
					})
					atomic.AddInt64(&reads, 1)
				}
			}
		}(i)
	}

	close(startSignal)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: concurrent read/write timed out after 30s")
	}

	assert.Zero(t, panics,
		"no goroutine should panic during concurrent read/write")

	totalOps := reads + writes + updates + deletes
	t.Logf("Concurrent read/write: reads=%d, writes=%d, updates=%d, "+
		"deletes=%d, total=%d, panics=%d",
		reads, writes, updates, deletes, totalOps, panics)

	assert.Greater(t, totalOps, int64(0),
		"at least some operations should have completed")
}

// TestMemoryStore_SearchUnderLoad tests that search remains functional and
// does not degrade severely when 50 goroutines are concurrently searching
// while writes are happening.
func TestMemoryStore_SearchUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	store := memory.NewInMemoryStore()
	ctx := context.Background()

	// Pre-populate with searchable data
	keywords := []string{"golang", "python", "javascript", "rust", "java",
		"kubernetes", "docker", "database", "cache", "memory"}
	for i := 0; i < 500; i++ {
		keyword := keywords[i%len(keywords)]
		_ = store.Add(ctx, &memory.Memory{
			ID:         fmt.Sprintf("search-mem-%d", i),
			UserID:     fmt.Sprintf("user-%d", i%10),
			Content:    fmt.Sprintf("This memory is about %s programming and related topics %d", keyword, i),
			Type:       memory.MemoryTypeEpisodic,
			Importance: float64(i%100) / 100.0,
			Metadata:   map[string]interface{}{},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})
	}

	const searchGoroutines = 50
	const writeGoroutines = 10
	var wg sync.WaitGroup
	var searchOps int64
	var searchErrors int64
	var writeOps int64

	startSignal := make(chan struct{})

	// Searchers
	for i := 0; i < searchGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-startSignal

			for j := 0; j < 20; j++ {
				keyword := keywords[(id+j)%len(keywords)]
				results, err := store.Search(ctx, keyword, &memory.SearchOptions{
					TopK:     10,
					MinScore: 0.1,
				})
				if err != nil {
					atomic.AddInt64(&searchErrors, 1)
				} else {
					_ = results // Use results to prevent compiler optimization
					atomic.AddInt64(&searchOps, 1)
				}
			}
		}(i)
	}

	// Concurrent writers
	for i := 0; i < writeGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-startSignal

			for j := 0; j < 50; j++ {
				keyword := keywords[(id+j)%len(keywords)]
				_ = store.Add(ctx, &memory.Memory{
					ID:         fmt.Sprintf("live-mem-%d-%d", id, j),
					UserID:     fmt.Sprintf("user-%d", id),
					Content:    fmt.Sprintf("Live write about %s topic %d", keyword, j),
					Type:       memory.MemoryTypeSemantic,
					Importance: 0.5,
					Metadata:   map[string]interface{}{},
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				})
				atomic.AddInt64(&writeOps, 1)
			}
		}(i)
	}

	close(startSignal)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: search under load timed out after 30s")
	}

	t.Logf("Search under load: searches=%d, search_errors=%d, writes=%d",
		searchOps, searchErrors, writeOps)

	assert.Zero(t, searchErrors, "searches should not error under load")
	assert.Equal(t, int64(searchGoroutines*20), searchOps,
		"all search operations should complete")
}

// TestMemoryStore_MemoryNotGrowingUnbounded verifies that InMemoryStore
// memory usage stays within reasonable bounds after many write-delete
// cycles, detecting potential memory leaks.
func TestMemoryStore_MemoryNotGrowingUnbounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	store := memory.NewInMemoryStore()
	ctx := context.Background()

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	// Write and delete in cycles to test for memory leaks
	const cycles = 10
	const memoriesPerCycle = 200

	for cycle := 0; cycle < cycles; cycle++ {
		// Write phase
		for i := 0; i < memoriesPerCycle; i++ {
			_ = store.Add(ctx, &memory.Memory{
				ID:         fmt.Sprintf("cycle-%d-mem-%d", cycle, i),
				UserID:     fmt.Sprintf("user-%d", i%5),
				Content:    fmt.Sprintf("Cycle %d memory %d with some content for testing growth patterns", cycle, i),
				Type:       memory.MemoryTypeWorking,
				Importance: 0.5,
				Metadata: map[string]interface{}{
					"cycle": cycle,
					"index": i,
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
		}

		// Delete phase (delete previous cycle's memories)
		if cycle > 0 {
			for i := 0; i < memoriesPerCycle; i++ {
				_ = store.Delete(ctx, fmt.Sprintf("cycle-%d-mem-%d", cycle-1, i))
			}
		}
	}

	// Final cleanup
	for i := 0; i < memoriesPerCycle; i++ {
		_ = store.Delete(ctx, fmt.Sprintf("cycle-%d-mem-%d", cycles-1, i))
	}

	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	liveHeapMB := float64(memAfter.HeapInuse) / 1024 / 1024

	t.Logf("Memory growth check: %d cycles x %d memories, "+
		"live heap=%.2f MB",
		cycles, memoriesPerCycle, liveHeapMB)

	// After writing and deleting 2000 memories, live heap should be bounded
	assert.Less(t, liveHeapMB, 200.0,
		"memory should not grow unbounded after write-delete cycles")
}

// TestMemoryStore_EntityGraphUnderLoad tests entity and relationship
// operations under concurrent access to verify graph operations are
// thread-safe.
func TestMemoryStore_EntityGraphUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	store := memory.NewInMemoryStore()
	ctx := context.Background()

	const goroutineCount = 50
	var wg sync.WaitGroup
	var panics int64
	var entityAdds int64
	var relAdds int64

	startSignal := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-startSignal

			// Add entities
			for j := 0; j < 10; j++ {
				entity := &memory.Entity{
					ID:   fmt.Sprintf("entity-%d-%d", id, j),
					Name: fmt.Sprintf("Entity %d-%d", id, j),
					Type: "concept",
					Properties: map[string]interface{}{
						"worker": id,
					},
				}
				if err := store.AddEntity(ctx, entity); err == nil {
					atomic.AddInt64(&entityAdds, 1)
				}
			}

			// Add relationships
			for j := 0; j < 5; j++ {
				rel := &memory.Relationship{
					ID:         fmt.Sprintf("rel-%d-%d", id, j),
					SourceID:   fmt.Sprintf("entity-%d-%d", id, j),
					TargetID:   fmt.Sprintf("entity-%d-%d", id, (j+1)%10),
					Type:       "related_to",
					Strength:   0.8,
					Properties: map[string]interface{}{},
				}
				if err := store.AddRelationship(ctx, rel); err == nil {
					atomic.AddInt64(&relAdds, 1)
				}
			}

			// Read entities and relationships
			for j := 0; j < 10; j++ {
				_, _ = store.GetEntity(ctx, fmt.Sprintf("entity-%d-%d", id, j))
				_, _ = store.GetRelationships(ctx, fmt.Sprintf("entity-%d-%d", id, j))
			}

			// Search entities
			_, _ = store.SearchEntities(ctx, "Entity", 10)
		}(i)
	}

	close(startSignal)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: entity graph under load timed out")
	}

	assert.Zero(t, panics,
		"no goroutine should panic during entity graph operations")

	t.Logf("Entity graph under load: entities=%d, relationships=%d, panics=%d",
		entityAdds, relAdds, panics)

	assert.Equal(t, int64(goroutineCount*10), entityAdds,
		"all entity adds should succeed")
	assert.Equal(t, int64(goroutineCount*5), relAdds,
		"all relationship adds should succeed")
}
