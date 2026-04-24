// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	cachepkg "dev.helix.agent/internal/adapters/cache"
)

// TestMemoryCacheAdapter_ConcurrentReadWrite tests concurrent Set/Get/Delete
// on MemoryCacheAdapter. The adapter wraps an extracted Cache module that uses
// internal locking — this test verifies that protection is effective.
func TestMemoryCacheAdapter_ConcurrentReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	adapter := cachepkg.NewMemoryCacheAdapter(1000, 5*time.Minute)
	ctx := context.Background()

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("race-key-%d", id%5) // Shared keys to force contention.
			for j := 0; j < 50; j++ {
				_ = adapter.Set(ctx, key, fmt.Sprintf("value-%d-%d", id, j), time.Minute)
				var dest string
				_ = adapter.Get(ctx, key, &dest)
				if j%10 == 0 {
					_ = adapter.Delete(ctx, key)
				}
				_, _ = adapter.Exists(ctx, key)
			}
		}(i)
	}

	wg.Wait()
}

// TestMemoryCacheAdapter_ConcurrentMixedKeys tests concurrent access with both
// shared and unique keys to exercise both hot-path and cold-path code.
func TestMemoryCacheAdapter_ConcurrentMixedKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	adapter := cachepkg.NewMemoryCacheAdapter(500, time.Minute)
	ctx := context.Background()

	var wg sync.WaitGroup
	const goroutines = 15

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Unique key per goroutine — no contention, but tests independent paths.
			uniqueKey := fmt.Sprintf("unique-%d", id)
			// Shared key — intentional contention.
			sharedKey := "shared-key"

			for j := 0; j < 30; j++ {
				_ = adapter.Set(ctx, uniqueKey, id*j, time.Minute)
				_ = adapter.Set(ctx, sharedKey, id*j, time.Second)
				var v int
				_ = adapter.Get(ctx, uniqueKey, &v)
				_ = adapter.Get(ctx, sharedKey, &v)
			}
			_ = adapter.Delete(ctx, uniqueKey)
		}(i)
	}

	wg.Wait()
}
