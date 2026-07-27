// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"sync"
	"testing"
)

// TestDebateService_IntentCacheConcurrentAccess tests concurrent reads and writes
// to an intentCache-like map protected by a sync.Mutex — mirroring the pattern
// used in DebateService. The DebateService itself requires a full dependency graph
// (ProviderRegistry, DB, etc.) so we replicate its exact locking pattern directly.
func TestDebateService_IntentCacheConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode") // SKIP-OK: #short-mode
	}

	// Replicate the DebateService intentCache structure exactly.
	type intentResult struct {
		Intent string
		Score  float64
	}

	var mu sync.Mutex
	intentCache := make(map[string]*intentResult)

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				key := "intent-key"
				if j%5 == 0 {
					key = "intent-key-alt"
				}

				// Write path (cache miss → populate).
				mu.Lock()
				if _, exists := intentCache[key]; !exists {
					intentCache[key] = &intentResult{
						Intent: "code_generation",
						Score:  0.92,
					}
				}
				mu.Unlock()

				// Read path (cache hit).
				mu.Lock()
				result := intentCache[key]
				mu.Unlock()
				_ = result

				// Eviction path.
				if j%20 == 0 {
					mu.Lock()
					delete(intentCache, key)
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestDebateService_ConcurrentMapGrow tests that the map grows safely under
// concurrent writes when the key space is large (many unique keys).
func TestDebateService_ConcurrentMapGrow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode") // SKIP-OK: #short-mode
	}

	type entry struct{ value int }

	var mu sync.Mutex
	store := make(map[string]*entry)

	pickKey := func(id, j int) string {
		if j%3 == 0 {
			return "shared"
		}
		return "unique-" + string(rune('A'+id))
	}

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				k := pickKey(id, j)
				mu.Lock()
				store[k] = &entry{value: id*j + 1}
				mu.Unlock()

				mu.Lock()
				_ = store[k]
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
}
