package stress

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/adapters/memory"
	helixmem "dev.helix.agent/internal/memory"
)

// TestStress_ConcurrentUsers simulates multiple concurrent users
func TestStress_ConcurrentUsers(t *testing.T) {
	if os.Getenv("STRESS_TEST") != "true" {
		t.Skip("Set STRESS_TEST=true to run stress tests") // SKIP-OK: #legacy-untriaged
	}

	adapter := memory.NewOptimalStoreAdapter()
	if adapter == nil {
		t.Skip("HelixMemory not available") // SKIP-OK: #legacy-untriaged
	}

	ctx := context.Background()
	numUsers := 10
	operationsPerUser := 100

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			for j := 0; j < operationsPerUser; j++ {
				mem := &helixmem.Memory{
					ID:         fmt.Sprintf("user%d-mem%d", userID, j),
					Content:    fmt.Sprintf("User %d memory %d", userID, j),
					Type:       helixmem.MemoryTypeSemantic,
					Category:   "stress",
					Importance: 0.5,
				}
				_ = adapter.Add(ctx, mem)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)
	totalOps := numUsers * operationsPerUser
	opsPerSecond := float64(totalOps) / duration.Seconds()

	t.Logf("Completed %d operations in %v (%.2f ops/sec)", totalOps, duration, opsPerSecond)
}

// TestStress_MemoryVolume tests with large memory volumes
func TestStress_MemoryVolume(t *testing.T) {
	if os.Getenv("STRESS_TEST") != "true" {
		t.Skip("Set STRESS_TEST=true to run stress tests") // SKIP-OK: #legacy-untriaged
	}

	adapter := memory.NewOptimalStoreAdapter()
	if adapter == nil {
		t.Skip("HelixMemory not available") // SKIP-OK: #legacy-untriaged
	}

	ctx := context.Background()
	numMemories := 1000

	start := time.Now()
	for i := 0; i < numMemories; i++ {
		mem := &helixmem.Memory{
			ID:         fmt.Sprintf("volume-%d", i),
			Content:    fmt.Sprintf("Volume test memory number %d with some content", i),
			Type:       helixmem.MemoryTypeSemantic,
			Category:   "volume",
			Importance: float64(i%10) / 10.0,
		}
		if err := adapter.Add(ctx, mem); err != nil {
			t.Logf("Failed to add memory %d: %v", i, err)
		}
	}

	duration := time.Since(start)
	t.Logf("Inserted %d memories in %v", numMemories, duration)
}
