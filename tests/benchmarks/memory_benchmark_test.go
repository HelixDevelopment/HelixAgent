package benchmarks

import (
	"context"
	"fmt"
	"testing"

	"dev.helix.agent/internal/adapters/memory"
	helixmem "dev.helix.agent/internal/memory"
)

// BenchmarkHelixMemoryAdd benchmarks adding memories
func BenchmarkHelixMemoryAdd(b *testing.B) {
	adapter := memory.NewOptimalStoreAdapter()
	if adapter == nil {
		b.Skip("HelixMemory not available")
	}

	ctx := context.Background()
	mem := &helixmem.Memory{
		Content:    "Benchmark memory content",
		Type:       helixmem.MemoryTypeSemantic,
		Category:   "benchmark",
		Importance: 0.5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mem.ID = fmt.Sprintf("bench-%d", i)
		_ = adapter.Add(ctx, mem)
	}
}

// BenchmarkHelixMemorySearch benchmarks searching memories
func BenchmarkHelixMemorySearch(b *testing.B) {
	adapter := memory.NewOptimalStoreAdapter()
	if adapter == nil {
		b.Skip("HelixMemory not available")
	}

	ctx := context.Background()
	opts := &helixmem.SearchOptions{
		TopK: 10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.Search(ctx, "benchmark query", opts)
	}
}

// BenchmarkHelixMemoryParallel benchmarks concurrent operations
func BenchmarkHelixMemoryParallel(b *testing.B) {
	adapter := memory.NewOptimalStoreAdapter()
	if adapter == nil {
		b.Skip("HelixMemory not available")
	}

	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			mem := &helixmem.Memory{
				ID:         fmt.Sprintf("parallel-%d", i),
				Content:    "Parallel benchmark content",
				Type:       helixmem.MemoryTypeSemantic,
				Category:   "benchmark",
				Importance: 0.5,
			}
			_ = adapter.Add(ctx, mem)
			_, _ = adapter.Search(ctx, "parallel", &helixmem.SearchOptions{TopK: 5})
			i++
		}
	})
}

// BenchmarkMemoryLatency measures operation latency
func BenchmarkMemoryLatency(b *testing.B) {
	adapter := memory.NewOptimalStoreAdapter()
	if adapter == nil {
		b.Skip("HelixMemory not available")
	}

	ctx := context.Background()

	b.Run("Add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mem := &helixmem.Memory{
				ID:         fmt.Sprintf("latency-%d", i),
				Content:    "Latency test content",
				Type:       helixmem.MemoryTypeSemantic,
				Category:   "latency",
				Importance: 0.5,
			}
			_ = adapter.Add(ctx, mem)
		}
	})

	b.Run("Search", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = adapter.Search(ctx, "latency", &helixmem.SearchOptions{TopK: 5})
		}
	})
}
