package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/adapters/memory"
	helixmem "dev.helix.agent/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_HelixMemoryFullFlow tests the complete memory flow
func TestE2E_HelixMemoryFullFlow(t *testing.T) {
	if os.Getenv("HELIX_MEMORY_E2E") != "true" {
		t.Skip("Set HELIX_MEMORY_E2E=true to run E2E tests")  // SKIP-OK: #legacy-untriaged
	}

	ctx := context.Background()
	adapter := memory.NewOptimalStoreAdapter()
	require.NotNil(t, adapter, "HelixMemory adapter should be initialized")

	userID := fmt.Sprintf("e2e-user-%d", time.Now().Unix())
	sessionID := fmt.Sprintf("e2e-session-%d", time.Now().Unix())

	t.Run("Store Memories", func(t *testing.T) {
		memories := []*helixmem.Memory{
			{
				Content:    "User prefers dark mode interfaces",
				Type:       helixmem.MemoryTypeSemantic,
				Category:   "preference",
				Importance: 0.8,
				UserID:     userID,
				SessionID:  sessionID,
			},
			{
				Content:    "User is working on a Go project",
				Type:       helixmem.MemoryTypeEpisodic,
				Category:   "context",
				Importance: 0.7,
				UserID:     userID,
				SessionID:  sessionID,
			},
		}

		for _, mem := range memories {
			err := adapter.Add(ctx, mem)
			assert.NoError(t, err, "Should store memory")
		}
	})

	t.Run("Search Memories", func(t *testing.T) {
		opts := &helixmem.SearchOptions{
			UserID:    userID,
			SessionID: sessionID,
			TopK:      10,
		}

		results, err := adapter.Search(ctx, "user preferences", opts)
		require.NoError(t, err, "Search should succeed")
		assert.GreaterOrEqual(t, len(results), 1, "Should find at least one memory")
	})

	t.Run("Retrieve by User", func(t *testing.T) {
		opts := &helixmem.ListOptions{Limit: 10}
		memories, err := adapter.GetByUser(ctx, userID, opts)
		require.NoError(t, err, "Should retrieve user memories")
		assert.GreaterOrEqual(t, len(memories), 2, "Should have stored memories")
	})
}

// TestE2E_DebateWithMemory tests debate with memory integration
func TestE2E_DebateWithMemory(t *testing.T) {
	if os.Getenv("HELIX_MEMORY_E2E") != "true" {
		t.Skip("Set HELIX_MEMORY_E2E=true to run E2E tests")  // SKIP-OK: #legacy-untriaged
	}

	// This would test a full debate flow with memory
	// Requires running HelixAgent server
	t.Skip("Requires running HelixAgent server - manual test only")  // SKIP-OK: #legacy-untriaged
}

// TestE2E_ServiceHealth checks all memory services are healthy
func TestE2E_ServiceHealth(t *testing.T) {
	if os.Getenv("HELIX_MEMORY_E2E") != "true" {
		t.Skip("Set HELIX_MEMORY_E2E=true to run E2E tests")  // SKIP-OK: #legacy-untriaged
	}

	adapter := memory.NewOptimalStoreAdapter()
	require.NotNil(t, adapter, "Adapter should be initialized")

	// Check if we can perform operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := adapter.Search(ctx, "health check", &helixmem.SearchOptions{TopK: 1})
	// Search might return empty results but should not error if services are up
	assert.NoError(t, err, "Services should be accessible")
}
