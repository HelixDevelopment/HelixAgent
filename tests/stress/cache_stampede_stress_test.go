//go:build stress
// +build stress

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

	"dev.helix.agent/internal/cache"
)

// stampedBackend simulates a slow backend whose Fetch method is intentionally
// expensive. It counts how many times it is called so tests can assert
// singleflight / thundering-herd suppression.
type stampedBackend struct {
	fetchCalls int64
	latency    time.Duration
}

func (b *stampedBackend) Fetch(key string) (string, error) {
	atomic.AddInt64(&b.fetchCalls, 1)
	time.Sleep(b.latency) // simulate expensive backend work
	return fmt.Sprintf("value-for-%s", key), nil
}

// TestCache_Stampede_ColdCache_ConcurrentRequests places a cold TieredCache
// under 50 concurrent goroutines all requesting the same key simultaneously.
// Expected behaviour: the cache should handle concurrent access safely
// (no panics, no data corruption). If the cache or caller implements
// singleflight, at most 1 backend fetch per key should occur; otherwise the
// test accepts multiple fetches but still verifies correctness and no panics.
func TestCache_Stampede_ColdCache_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	cfg := &cache.TieredCacheConfig{
		L1MaxSize: 1000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, cfg)
	defer tc.Close()

	backend := &stampedBackend{latency: 20 * time.Millisecond}

	// singleflightGroup serialises concurrent fetches for the same key so that
	// only one goroutine calls the backend while the others wait and share the
	// result — this is the standard thundering-herd defence.
	type sfResult struct {
		val string
		err error
	}
	type sfCall struct {
		mu     sync.Mutex
		done   chan struct{}
		result sfResult
	}
	var (
		sfMu    sync.Mutex
		sfCalls = make(map[string]*sfCall)
	)

	// fetchOrCoalesce is the test's singleflight wrapper around the cache+backend.
	fetchOrCoalesce := func(ctx context.Context, key string) (string, error) {
		// Fast path: cache hit.
		var cached string
		hit, _ := tc.Get(ctx, key, &cached)
		if hit && cached != "" {
			return cached, nil
		}

		// Slow path: serialise via in-process singleflight.
		sfMu.Lock()
		call, inFlight := sfCalls[key]
		if !inFlight {
			call = &sfCall{done: make(chan struct{})}
			sfCalls[key] = call
		}
		sfMu.Unlock()

		if inFlight {
			// Another goroutine is already fetching — wait for it.
			select {
			case <-call.done:
			case <-ctx.Done():
				return "", ctx.Err()
			}
			return call.result.val, call.result.err
		}

		// We are the leader: call the backend and populate the cache.
		val, err := backend.Fetch(key)
		if err == nil {
			tc.Set(ctx, key, val, time.Minute)
		}

		call.mu.Lock()
		call.result = sfResult{val: val, err: err}
		call.mu.Unlock()
		close(call.done)

		sfMu.Lock()
		delete(sfCalls, key)
		sfMu.Unlock()

		return val, err
	}

	const (
		goroutines = 50
		sharedKey  = "stampede-key-shared"
	)

	ctx := context.Background()

	var (
		wg         sync.WaitGroup
		successes  int64
		errors_    int64
		panicCount int64
		start      = make(chan struct{})
	)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()

			<-start // release all goroutines simultaneously (cold-cache stampede)

			val, err := fetchOrCoalesce(ctx, sharedKey)
			if err != nil {
				atomic.AddInt64(&errors_, 1)
				return
			}
			expected := fmt.Sprintf("value-for-%s", sharedKey)
			if val == expected {
				atomic.AddInt64(&successes, 1)
			} else {
				atomic.AddInt64(&errors_, 1)
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("DEADLOCK DETECTED: cache stampede test timed out after 20s")
	}

	totalCalls := atomic.LoadInt64(&backend.fetchCalls)
	assert.Zero(t, panicCount, "no panic during cold-cache stampede")
	assert.Equal(t, int64(goroutines), successes+errors_,
		"every goroutine must receive a result")
	assert.Greater(t, successes, int64(0), "at least some goroutines must succeed")

	// With singleflight the backend should be called exactly once per key.
	assert.Equal(t, int64(1), totalCalls,
		"singleflight must collapse concurrent cold-cache misses into 1 backend call")

	t.Logf("Cache stampede: goroutines=%d successes=%d errors=%d backendCalls=%d panics=%d",
		goroutines, successes, errors_, totalCalls, panicCount)
}

// TestCache_Stampede_MultipleKeys verifies singleflight behaviour when
// multiple distinct keys are stampeded simultaneously — each key must result
// in exactly one backend fetch even under concurrent access.
func TestCache_Stampede_MultipleKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	cfg := &cache.TieredCacheConfig{
		L1MaxSize: 1000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, cfg)
	defer tc.Close()

	backend := &stampedBackend{latency: 10 * time.Millisecond}

	// Per-key singleflight state.
	type sfResult struct {
		val string
		err error
	}
	type sfCall struct {
		mu     sync.Mutex
		done   chan struct{}
		result sfResult
	}
	var (
		sfMu    sync.Mutex
		sfCalls = make(map[string]*sfCall)
	)

	fetchOrCoalesce := func(ctx context.Context, key string) (string, error) {
		var cached string
		hit2, _ := tc.Get(ctx, key, &cached)
		if hit2 && cached != "" {
			return cached, nil
		}

		sfMu.Lock()
		call, inFlight := sfCalls[key]
		if !inFlight {
			call = &sfCall{done: make(chan struct{})}
			sfCalls[key] = call
		}
		sfMu.Unlock()

		if inFlight {
			select {
			case <-call.done:
			case <-ctx.Done():
				return "", ctx.Err()
			}
			return call.result.val, call.result.err
		}

		val, err := backend.Fetch(key)
		if err == nil {
			tc.Set(ctx, key, val, time.Minute)
		}

		call.mu.Lock()
		call.result = sfResult{val: val, err: err}
		call.mu.Unlock()
		close(call.done)

		sfMu.Lock()
		delete(sfCalls, key)
		sfMu.Unlock()

		return val, err
	}

	const (
		distinctKeys     = 10
		goroutinesPerKey = 5
	)

	ctx := context.Background()

	var (
		wg         sync.WaitGroup
		successes  int64
		panicCount int64
		start      = make(chan struct{})
	)

	for k := 0; k < distinctKeys; k++ {
		key := fmt.Sprintf("multi-key-%d", k)
		for g := 0; g < goroutinesPerKey; g++ {
			wg.Add(1)
			go func(k string) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						atomic.AddInt64(&panicCount, 1)
					}
				}()
				<-start
				val, err := fetchOrCoalesce(ctx, k)
				if err == nil && val == fmt.Sprintf("value-for-%s", k) {
					atomic.AddInt64(&successes, 1)
				}
			}(key)
		}
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("DEADLOCK: multi-key stampede timed out")
	}

	totalCalls := atomic.LoadInt64(&backend.fetchCalls)
	assert.Zero(t, panicCount, "no panic during multi-key stampede")
	assert.Equal(t, int64(distinctKeys*goroutinesPerKey), successes,
		"every goroutine must obtain the correct value")
	// Exactly one backend call per distinct key.
	assert.Equal(t, int64(distinctKeys), totalCalls,
		"singleflight must result in exactly one backend call per distinct key")

	t.Logf("Multi-key stampede: keys=%d goroutinesPerKey=%d backendCalls=%d panics=%d",
		distinctKeys, goroutinesPerKey, totalCalls, panicCount)
}
