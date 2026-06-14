// Package memory provides adapters between HelixAgent's internal/memory types
// and the extracted digital.vasic.helixmemory module.
//
// This is the DEFAULT memory implementation using the HelixMemory unified
// cognitive engine (Cognee + Mem0 + Letta fusion). It is active unless you
// explicitly opt out with:
//
//	go build -tags nohelixmemory
//
//go:build !nohelixmemory

package memory

import (
	"log"

	"dev.helix.agent/internal/config"
)

// NewOptimalStoreAdapter creates a StoreAdapter backed by the HelixMemory
// unified cognitive memory engine. It initializes the FusionEngine that
// combines Cognee (knowledge graphs), Mem0 (semantic memory), and Letta
// (agent memory) into a single powerful memory system.
//
// Configuration is read from HELIX_MEMORY_* environment variables.
// See docs/HELIXMEMORY_SETUP.md for detailed setup instructions.
//
// Default Behavior:
//   - Local mode (containers): All services run in Docker
//   - Cognee defaults to local (requires paid subscription for cloud)
//   - Automatic fallback between systems via circuit breaker
//   - Intelligent routing based on memory type
func NewOptimalStoreAdapter() *HelixMemoryFusionAdapter {
	cfg := config.Load()
	adapter, err := NewHelixMemoryFusionAdapter(cfg)
	if err != nil {
		log.Printf("[HelixMemory] Warning: Failed to initialize fusion engine: %v", err)
		log.Printf("[HelixMemory] Returning nil; caller substitutes the local in-memory store fallback")
		return nil
	}

	// Log active configuration
	stats := adapter.GetStats()
	log.Printf("[HelixMemory] Fusion engine initialized:")
	log.Printf("  - Cognee: %v", stats.CogneeHealthy)
	log.Printf("  - Mem0: %v", stats.Mem0Healthy)
	log.Printf("  - Letta: %v", stats.LettaHealthy)

	return adapter
}

// IsHelixMemoryEnabled returns whether HelixMemory is compiled into the binary.
func IsHelixMemoryEnabled() bool {
	return true
}

// MemoryBackendName returns the name of the active memory backend.
func MemoryBackendName() string {
	return "digital.vasic.helixmemory (Fusion: Cognee+Mem0+Letta)"
}

// NewHelixMemoryProvider creates a standalone HelixMemoryFusionAdapter for
// direct access to HelixMemory features (temporal queries, core memory,
// consolidation, power features). Use this when you need more than
// the basic MemoryStore interface.
func NewHelixMemoryProvider() (*HelixMemoryFusionAdapter, error) {
	cfg := config.Load()
	return NewHelixMemoryFusionAdapter(cfg)
}
