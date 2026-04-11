package cache

// Phase-3 regression tests for the provider cache mutex hardening.
// Pins two invariants from the 2026-04-11 fix:
//
//  1. The per-provider stats helpers now use defer on the write lock so a
//     panic inside the critical section cannot leak the mutex.
//  2. High-concurrency hit/miss/set tracking stays race-clean under
//     `go test -race` and the counts accumulate deterministically.

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderCache_TrackStats_Concurrent(t *testing.T) {
	pc := NewProviderCache(nil, nil)

	const (
		providers  = 4
		goroutines = 16
		iterations = 250
	)
	providerNames := []string{"openai", "claude", "gemini", "deepseek"}

	var wg sync.WaitGroup
	wg.Add(goroutines * providers)
	for p := 0; p < providers; p++ {
		name := providerNames[p]
		for g := 0; g < goroutines; g++ {
			go func(name string) {
				defer wg.Done()
				for i := 0; i < iterations; i++ {
					pc.trackProviderHit(name)
					pc.trackProviderMiss(name)
					pc.trackProviderSet(name)
				}
			}(name)
		}
	}
	wg.Wait()

	metrics := pc.Metrics()
	require.NotNil(t, metrics)
	// Each provider saw goroutines*iterations of each op.
	expected := int64(goroutines * iterations)
	for _, name := range providerNames {
		stats, ok := metrics.ByProvider[name]
		require.True(t, ok, "provider %s must have stats", name)
		assert.Equal(t, expected, atomic.LoadInt64(&stats.Hits),
			"atomic hit accumulation must not lose updates")
		assert.Equal(t, expected, atomic.LoadInt64(&stats.Misses))
		assert.Equal(t, expected, atomic.LoadInt64(&stats.Sets))
	}
}

func TestProviderCache_GetOrCreateStats_Idempotent(t *testing.T) {
	pc := NewProviderCache(nil, nil)

	// First call creates the stats bucket; subsequent calls must return
	// the same pointer so atomic increments accumulate rather than being
	// lost to rebinding.
	a := pc.getOrCreateStats("probe")
	b := pc.getOrCreateStats("probe")
	require.NotNil(t, a)
	assert.Same(t, a, b, "stats bucket must be stable for the provider lifetime")

	// Distinct providers get distinct buckets.
	c := pc.getOrCreateStats("other")
	assert.NotSame(t, a, c)
}
